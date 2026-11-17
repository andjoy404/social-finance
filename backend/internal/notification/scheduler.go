package notification

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"social-finance/internal/database"
)

// schedulerTickInterval is how often the scheduler wakes up to check whether any
// active schedule is due for execution.
const schedulerTickInterval = 1 * time.Minute

// Scheduler runs active notification schedules automatically in the background.
// It is safe to run alongside multiple backend instances: each schedule
// execution is guarded by a Postgres advisory lock plus a UNIQUE dedup key on
// notification_logs, so at most one instance delivers a given schedule on a
// given execution date.
type Scheduler struct {
	pool *database.Pool
	svc  Service
	env  string

	mu      sync.Mutex
	running bool
	stop    chan struct{}
}

// NewScheduler creates a scheduler. It is disabled in the "test" environment
// (appEnv == "test") so that automated tests are not affected by background
// pushes.
func NewScheduler(pool *database.Pool, svc Service, appEnv string) *Scheduler {
	return &Scheduler{
		pool: pool,
		svc:  svc,
		env:  appEnv,
		stop: make(chan struct{}),
	}
}

// Start runs a catch-up pass on startup (schedules that should have fired today
// but were missed while the backend was down) and then ticks every minute,
// executing any due schedules. It is a no-op in the test environment.
func (sch *Scheduler) Start(ctx context.Context) {
	if sch.env == "test" {
		slog.Info("notification scheduler disabled in test environment")
		return
	}
	sch.mu.Lock()
	if sch.running {
		sch.mu.Unlock()
		return
	}
	sch.running = true
	sch.mu.Unlock()

	// Catch-up on startup: execute schedules that should have fired today but
	// were missed while the backend was unavailable.
	sch.runOnce(ctx, time.Now())

	go sch.loop(ctx)
}

// Stop halts the scheduler loop.
func (sch *Scheduler) Stop() {
	sch.mu.Lock()
	if !sch.running {
		sch.mu.Unlock()
		return
	}
	sch.running = false
	sch.mu.Unlock()
	close(sch.stop)
}

func (sch *Scheduler) loop(ctx context.Context) {
	ticker := time.NewTicker(schedulerTickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-sch.stop:
			return
		case now := <-ticker.C:
			sch.runOnce(ctx, now)
		}
	}
}

// runOnce fetches all active schedules whose send days/hour/minute match the
// given local time (computed in each schedule's own timezone) and executes each
// one. Per-schedule execution is guarded by an advisory lock + dedup key.
func (sch *Scheduler) runOnce(ctx context.Context, now time.Time) {
	// Age out activity-linked schedules whose activity has terminated, fire
	// lifecycle events for completed/cancelled activities.
	if err := sch.svc.RunActivityMaintenance(ctx); err != nil {
		slog.Error("failed to run activity maintenance", "error", err)
	}

	schedules, err := sch.svc.GetDueSchedules(ctx, now)
	if err != nil {
		slog.Error("failed to fetch due notification schedules", "error", err)
		return
	}
	for _, schedule := range schedules {
		sent, err := sch.svc.ExecuteSchedule(ctx, schedule, now)
		if err != nil {
			slog.Error("failed to execute notification schedule",
				"schedule_id", schedule.ID, "event_type", schedule.EventType, "rt_id", schedule.RTID, "error", err)
			continue
		}
		slog.Info("notification schedule executed",
			"schedule_id", schedule.ID, "event_type", schedule.EventType,
			"rt_id", schedule.RTID, "sent", sent)
	}
}
