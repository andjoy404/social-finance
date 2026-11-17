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

func TestNormalizeScheduleDatesRequiresBothEndpoints(t *testing.T) {
	if _, _, err := normalizeScheduleDates("2026-11-12", "", ""); err == nil {
		t.Fatal("expected missing end_date rejection")
	}
	if _, _, err := normalizeScheduleDates("", "2026-11-12", ""); err == nil {
		t.Fatal("expected missing start_date rejection")
	}
}

func TestNormalizeScheduleDatesKeepsLegacyOneDateClient(t *testing.T) {
	start, end, err := normalizeScheduleDates("", "", "2026-11-12")
	if err != nil {
		t.Fatal(err)
	}
	if !start.Equal(end) {
		t.Fatalf("legacy date must expand to same day, got %s and %s", start, end)
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
