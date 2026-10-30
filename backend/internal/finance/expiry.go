package finance

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	_ "time/tzdata" // Embed IANA timezone data so Asia/Jakarta is available in scratch/alpine builds.

	"social-finance/internal/database"
)

// ErrDueExpired indicates that a voluntary due has passed its valid_until date.
var ErrDueExpired = errors.New("voluntary due has expired")

const (
	jakartaTimezone              = "Asia/Jakarta"
	voluntaryExpiryHour          = 0
	voluntaryExpiryMinute        = 5
	voluntaryExpiryRetryInterval = 5 * time.Minute
)

var (
	jakartaLocationOnce sync.Once
	jakartaLocation     *time.Location
	jakartaLocationErr  error
)

func loadJakartaLocation() (*time.Location, error) {
	jakartaLocationOnce.Do(func() {
		jakartaLocation, jakartaLocationErr = time.LoadLocation(jakartaTimezone)
	})
	return jakartaLocation, jakartaLocationErr
}

// JakartaDate returns the calendar date at now in Asia/Jakarta as YYYY-MM-DD.
func JakartaDate(now time.Time) (string, error) {
	loc, err := loadJakartaLocation()
	if err != nil {
		return "", fmt.Errorf("load %s timezone: %w", jakartaTimezone, err)
	}
	return now.In(loc).Format("2006-01-02"), nil
}

// ParseActualReceivedDate validates that dateStr is in YYYY-MM-DD format,
// represents a valid calendar day, and is not in the future relative to now in Asia/Jakarta.
// Returns the occurredAt timestamp set to 12:00:00 (noon) WIB, the canonical YYYY-MM-DD string, or an error.
func ParseActualReceivedDate(dateStr string, now time.Time) (time.Time, string, error) {
	trimmed := strings.TrimSpace(dateStr)
	if trimmed == "" {
		return time.Time{}, "", errors.New("tanggal penerimaan aktual (actual_received_date) wajib diisi")
	}
	if len(trimmed) != 10 || trimmed[4] != '-' || trimmed[7] != '-' {
		return time.Time{}, "", errors.New("format actual_received_date harus YYYY-MM-DD")
	}
	loc, err := loadJakartaLocation()
	if err != nil {
		return time.Time{}, "", fmt.Errorf("load timezone: %w", err)
	}
	parsed, err := time.ParseInLocation("2006-01-02", trimmed, loc)
	if err != nil {
		return time.Time{}, "", errors.New("tanggal penerimaan aktual tidak valid")
	}
	canon := parsed.Format("2006-01-02")
	if canon != trimmed {
		return time.Time{}, "", errors.New("tanggal penerimaan aktual tidak valid")
	}
	todayJakarta, err := JakartaDate(now)
	if err != nil {
		return time.Time{}, "", err
	}
	if canon > todayJakarta {
		return time.Time{}, "", errors.New("tanggal penerimaan aktual tidak boleh berada di masa depan")
	}
	occurredAt := time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 12, 0, 0, 0, loc)
	return occurredAt, canon, nil
}

func normalizeDateOnly(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	parsed, err := time.Parse("2006-01-02", trimmed)
	if err != nil {
		return "", errors.New("valid_until must use YYYY-MM-DD format")
	}
	return parsed.Format("2006-01-02"), nil
}

// voluntaryDueExpired reports whether a voluntary due is past its valid_until
// date. The deadline is inclusive in Asia/Jakarta: a due whose valid_until is
// 2026-10-31 remains active through 2026-10-31 23:59:59 WIB and expires at
// 2026-11-01 00:00:00 WIB.
func voluntaryDueExpired(due *Due, now time.Time) (bool, error) {
	if due == nil || !due.IsVoluntary || due.ValidUntil == nil {
		return false, nil
	}
	validUntil, err := normalizeDateOnly(*due.ValidUntil)
	if err != nil {
		return false, err
	}
	today, err := JakartaDate(now)
	if err != nil {
		return false, err
	}
	return today > validUntil, nil
}

func ensureVoluntaryDueNotExpired(due *Due, now time.Time) error {
	expired, err := voluntaryDueExpired(due, now)
	if err != nil {
		return err
	}
	if expired {
		return ErrDueExpired
	}
	return nil
}

// ExpireVoluntaryDues marks active voluntary dues whose valid_until is before
// the reference date (Asia/Jakarta) as inactive. It is idempotent and must run
// inside a caller-owned transaction so the status changes and audit rows commit
// atomically. The caller is responsible for commit/rollback.
func ExpireVoluntaryDues(ctx context.Context, tx *sql.Tx, now time.Time) (int64, error) {
	today, err := JakartaDate(now)
	if err != nil {
		return 0, err
	}

	type expiredDue struct {
		id         string
		rtID       string
		validUntil string
	}
	var expired []expiredDue

	rows, err := tx.QueryContext(ctx,
		`UPDATE dues
		 SET is_active = false, updated_at = CURRENT_TIMESTAMP
		 WHERE is_voluntary = true
		   AND is_active = true
		   AND valid_until IS NOT NULL
		   AND valid_until < $1::date
		 RETURNING id, rt_id, to_char(valid_until, 'YYYY-MM-DD')`,
		today,
	)
	if err != nil {
		return 0, fmt.Errorf("expire voluntary dues: %w", err)
	}
	for rows.Next() {
		var item expiredDue
		if err := rows.Scan(&item.id, &item.rtID, &item.validUntil); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan expired due: %w", err)
		}
		expired = append(expired, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("iterate expired dues: %w", err)
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("close expired dues: %w", err)
	}

	for _, item := range expired {
		payload, _ := json.Marshal(map[string]string{
			"reason":      "valid_until_expired",
			"valid_until": item.validUntil,
		})
		_ = AuditLogInsert(ctx, tx, AuditLog{
			RTID:       item.rtID,
			UserID:     nil,
			Action:     "expire_due",
			EntityType: "due",
			EntityID:   item.id,
			NewValues:  payload,
		})
	}

	return int64(len(expired)), nil
}

func runVoluntaryDueExpiry(ctx context.Context, pool *database.Pool, now time.Time) (int64, error) {
	tx, err := pool.BeginTx(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin expiry transaction: %w", err)
	}
	defer tx.Rollback()

	count, err := ExpireVoluntaryDues(ctx, tx, now)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit expiry transaction: %w", err)
	}
	return count, nil
}

// nextVoluntaryExpiryRun returns the next 00:05 Asia/Jakarta strictly after now.
func nextVoluntaryExpiryRun(now time.Time) (time.Time, error) {
	loc, err := loadJakartaLocation()
	if err != nil {
		return time.Time{}, fmt.Errorf("load %s timezone: %w", jakartaTimezone, err)
	}
	local := now.In(loc)
	next := time.Date(
		local.Year(), local.Month(), local.Day(),
		voluntaryExpiryHour, voluntaryExpiryMinute, 0, 0, loc,
	)
	if !next.After(local) {
		next = next.AddDate(0, 0, 1)
	}
	return next, nil
}

func logVoluntaryExpiry(ctx context.Context, pool *database.Pool, now time.Time) error {
	count, err := runVoluntaryDueExpiry(ctx, pool, now)
	if err != nil {
		slog.Error("voluntary due expiry run failed", "error", err)
		return err
	}
	if count > 0 {
		slog.Info("voluntary dues expired", "count", count, "reference_date", mustJakartaDate(now))
	}
	return nil
}

func mustJakartaDate(now time.Time) string {
	date, err := JakartaDate(now)
	if err != nil {
		return now.UTC().Format("2006-01-02")
	}
	return date
}

// StartVoluntaryDueExpiryScheduler starts the daily voluntary-due expiry job.
// It runs once on startup to recover from downtime, then waits until 00:05 WIB
// each day. A failed run is retried after a short delay. The job only changes
// dues.is_active; it never touches bills, payments, allocations, transactions,
// balances, or historical reports.
func StartVoluntaryDueExpiryScheduler(ctx context.Context, pool *database.Pool) {
	go func() {
		// Catch up immediately after restart, then align to the daily schedule.
		startupRunFailed := logVoluntaryExpiry(ctx, pool, time.Now()) != nil

		for {
			var wait time.Duration
			if startupRunFailed {
				wait = voluntaryExpiryRetryInterval
			} else {
				next, err := nextVoluntaryExpiryRun(time.Now())
				if err != nil {
					slog.Error("failed to schedule voluntary due expiry", "error", err)
					wait = voluntaryExpiryRetryInterval
				} else {
					wait = time.Until(next)
					if wait < 0 {
						wait = 0
					}
				}
			}

			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return
			case <-timer.C:
				if err := logVoluntaryExpiry(ctx, pool, time.Now()); err != nil {
					// Retry shortly instead of waiting a full day.
					startupRunFailed = true
				} else {
					startupRunFailed = false
				}
			}
		}
	}()
}
