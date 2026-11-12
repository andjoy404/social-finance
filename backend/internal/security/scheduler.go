package security

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"
)

const schedulerTickInterval = 1 * time.Minute

// Scheduler runs periodic background transitions for security attendance
// (marking missed check-ins as 'tidak_hadir' and missed check-outs as 'belum_absen_pulang').
// It follows the notification.Scheduler lifecycle and is disabled in APP_ENV=test.
type Scheduler struct {
	svc    *Service
	stopCh chan struct{}
	wg     sync.WaitGroup
}

func NewScheduler(svc *Service) *Scheduler {
	return &Scheduler{
		svc:    svc,
		stopCh: make(chan struct{}),
	}
}

func (s *Scheduler) Start() {
	if s.svc == nil {
		return
	}
	if os.Getenv("APP_ENV") == "test" {
		slog.Info("security attendance scheduler disabled in test mode")
		return
	}

	s.wg.Add(1)
	go s.runLoop()
}

func (s *Scheduler) Stop() {
	select {
	case <-s.stopCh:
		return
	default:
		close(s.stopCh)
	}
	s.wg.Wait()
}

func (s *Scheduler) runLoop() {
	defer s.wg.Done()

	// Catch-up tick on startup so any shifts missed while the backend was offline are reconciled safely.
	s.tick()

	ticker := time.NewTicker(schedulerTickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.tick()
		}
	}
}

func (s *Scheduler) tick() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	res, err := s.svc.RunSchedulerTick(ctx)
	if err != nil {
		slog.Error("security attendance scheduler tick failed", "error", err)
		return
	}
	if res != nil && (res.MarkedAbsentCount > 0 || res.MarkedMissingCheckout > 0) {
		slog.Info("security attendance scheduler processed transitions",
			"marked_absent", res.MarkedAbsentCount,
			"marked_missing_checkout", res.MarkedMissingCheckout,
		)
	}
}
