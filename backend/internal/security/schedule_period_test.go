package security

import (
	"database/sql"
	"errors"
	"testing"
	"time"
)

func TestParseSchedulePeriodInclusiveDates(t *testing.T) {
	cases := []struct {
		name, start, end string
		days             int
	}{
		{"one day", "2026-11-12", "2026-11-12", 1},
		{"month boundary", "2026-11-29", "2026-12-02", 4},
		{"year boundary", "2026-12-31", "2027-01-02", 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start, end, err := parseSchedulePeriod(tc.start, tc.end)
			if err != nil {
				t.Fatal(err)
			}
			if got := int(end.Sub(start).Hours()/24) + 1; got != tc.days {
				t.Fatalf("inclusive days=%d want %d", got, tc.days)
			}
		})
	}
}

func TestParseSchedulePeriodRejectsInvalidAndReversedDates(t *testing.T) {
	for _, tc := range []struct{ start, end string }{{"2026-11-31", "2026-12-01"}, {"2026-11-13", "2026-11-12"}, {"", "2026-11-12"}} {
		if _, _, err := parseSchedulePeriod(tc.start, tc.end); err == nil {
			t.Fatalf("expected error for %q..%q", tc.start, tc.end)
		}
	}
}

func TestNormalizeScheduleDatesOptional(t *testing.T) {
	// Both empty: returns NULL, NULL.
	start, end, err := normalizeScheduleDates("", "", "")
	if err != nil {
		t.Fatalf("unexpected error for empty dates: %v", err)
	}
	if start != nil {
		t.Errorf("expected start=nil for empty dates, got %v", start)
	}
	if end != nil {
		t.Errorf("expected end=nil for empty dates, got %v", end)
	}

	// Only start_date: end_date remains NULL.
	start, end, err = normalizeScheduleDates("2026-11-12", "", "")
	if err != nil {
		t.Fatalf("unexpected error for start-only: %v", err)
	}
	if start == nil {
		t.Fatal("expected non-nil start for start-only")
	}
	if end != nil {
		t.Errorf("expected end=nil for start-only, got %v", end)
	}

	// Only end_date: start_date remains NULL.
	start, end, err = normalizeScheduleDates("", "2026-11-12", "")
	if err != nil {
		t.Fatalf("unexpected error for end-only: %v", err)
	}
	if end == nil {
		t.Fatal("expected non-nil end for end-only")
	}
	if start != nil {
		t.Errorf("expected start=nil for end-only, got %v", start)
	}

	// Both provided: normal range validation.
	start, end, err = normalizeScheduleDates("2026-11-10", "2026-11-12", "")
	if err != nil {
		t.Fatalf("unexpected error for both dates: %v", err)
	}
	if start == nil || end == nil {
		t.Fatal("expected non-nil start and end for both dates")
	}
	if start.After(*end) {
		t.Errorf("expected start <= end, got %v vs %v", start, end)
	}

	// Start after end: should fail.
	if _, _, err = normalizeScheduleDates("2026-11-15", "2026-11-10", ""); err == nil {
		t.Fatal("expected start-after-end rejection")
	}
}

func TestNormalizeScheduleDatesKeepsLegacyOneDateClient(t *testing.T) {
	start, end, err := normalizeScheduleDates("", "", "2026-11-12")
	if err != nil {
		t.Fatal(err)
	}
	if start == nil || end == nil {
		t.Fatal("expected non-nil start and end for legacy date")
	}
	if !start.Equal(*end) {
		t.Fatalf("legacy date must expand to same day, got %v and %v", start, end)
	}
}

func TestRecordedAttendanceIsProtected(t *testing.T) {
	day := time.Date(2026, 11, 12, 0, 0, 0, 0, JakartaLoc)
	for _, tc := range []struct {
		name string
		row  periodScheduleRow
		want bool
	}{
		{"unattended", periodScheduleRow{attendanceStatus: sqlNullString(StatusBelumHadir)}, false},
		{"hadir", periodScheduleRow{attendanceStatus: sqlNullString(StatusHadir)}, true},
		{"checked in", periodScheduleRow{attendanceStatus: sqlNullString(StatusBelumHadir), checkedIn: sqlNullTime(day)}, true},
		{"no attendance row", periodScheduleRow{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isRecordedAttendance(tc.row); got != tc.want {
				t.Fatalf("isRecordedAttendance=%v want %v", got, tc.want)
			}
		})
	}
}

func sqlNullString(v string) sql.NullString { return sql.NullString{String: v, Valid: true} }
func sqlNullTime(v time.Time) sql.NullTime  { return sql.NullTime{Time: v, Valid: true} }

func TestDateValidationReturnsAppError(t *testing.T) {
	_, _, err := parseSchedulePeriod("2026-11-16", "2026-11-12")
	var appErr *AppError
	if !errors.As(err, &appErr) || appErr.Code != "invalid_date_range" {
		t.Fatalf("expected invalid_date_range, got %v", err)
	}
}
