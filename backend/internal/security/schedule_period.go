package security

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

const maxSchedulePeriodDays = 366

type periodScheduleRow struct {
	id, userID, status, shiftConfigID string
	shiftDate, startAt, endAt         time.Time
	attendanceStatus                  sql.NullString
	checkedIn, checkedOut             sql.NullTime
}

func parseSchedulePeriod(startRaw, endRaw string) (time.Time, time.Time, error) {
	start, err := parseShiftDate(startRaw)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end, err := parseShiftDate(endRaw)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if end.Before(start) {
		return time.Time{}, time.Time{}, newAppError(400, "invalid_date_range", "Tanggal berakhir tugas tidak boleh sebelum tanggal mulai tugas", ErrValidation)
	}
	if int(end.Sub(start).Hours()/24)+1 > maxSchedulePeriodDays {
		return time.Time{}, time.Time{}, newAppError(400, "invalid_date_range", "Rentang jadwal maksimal 366 hari", ErrValidation)
	}
	return start, end, nil
}

func normalizeScheduleDates(startRaw, endRaw, legacy string) (time.Time, time.Time, error) {
	if startRaw == "" && endRaw == "" && legacy != "" {
		startRaw, endRaw = legacy, legacy
	}
	if startRaw == "" || endRaw == "" {
		return time.Time{}, time.Time{}, newAppError(400, "validation_error", "start_date dan end_date wajib diisi", ErrValidation)
	}
	return parseSchedulePeriod(startRaw, endRaw)
}

func lockOfficerSchedules(ctx context.Context, tx *sql.Tx, rtID, userID string) error {
	var locked any
	return tx.QueryRowContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, rtID+":"+userID).Scan(&locked)
}

func loadActiveShift(ctx context.Context, tx *sql.Tx, rtID, shiftID string) (ShiftConfig, error) {
	var c ShiftConfig
	err := tx.QueryRowContext(ctx, `SELECT id,rt_id,name,to_char(start_time,'HH24:MI:SS'),to_char(end_time,'HH24:MI:SS'),crosses_midnight,lateness_tolerance_minutes,check_in_window_before_minutes,check_in_window_after_minutes,absent_cutoff_minutes,missing_checkout_cutoff_minutes,version,is_active FROM security_shift_configs WHERE id=$1 AND rt_id=$2`, shiftID, rtID).Scan(&c.ID, &c.RTID, &c.Name, &c.StartTime, &c.EndTime, &c.CrossesMidnight, &c.LatenessToleranceMinutes, &c.CheckInWindowBeforeMinutes, &c.CheckInWindowAfterMinutes, &c.AbsentCutoffMinutes, &c.MissingCheckoutCutoffMinutes, &c.Version, &c.IsActive)
	if errors.Is(err, sql.ErrNoRows) {
		return c, newAppError(404, "shift_not_found", "shift configuration not found", ErrNotFound)
	}
	if err != nil {
		return c, err
	}
	if !c.IsActive {
		return c, newAppError(400, "shift_inactive", "cannot schedule an inactive shift configuration", ErrValidation)
	}
	return c, nil
}

func scheduleProjectionSQL() string {
	return `SELECT s.id,s.assignment_id,to_char(s.assignment_start_date,'YYYY-MM-DD'),to_char(s.assignment_end_date,'YYYY-MM-DD'),s.rt_id,s.user_id,u.full_name,s.membership_id,s.shift_config_id,s.shift_version,to_char(s.shift_date,'YYYY-MM-DD'),s.shift_name_snapshot,to_char(s.start_time_snapshot,'HH24:MI'),to_char(s.end_time_snapshot,'HH24:MI'),s.crosses_midnight_snapshot,s.lateness_tolerance_minutes,s.check_in_window_before_minutes,s.check_in_window_after_minutes,s.absent_cutoff_minutes,s.missing_checkout_cutoff_minutes,s.scheduled_start_at,s.scheduled_end_at,s.notes,s.status,COALESCE(ar.status,'belum_hadir'),ar.checked_in_at,ar.checked_out_at,s.created_at,s.updated_at FROM security_schedules s JOIN users u ON u.id=s.user_id LEFT JOIN security_attendance_records ar ON ar.schedule_id=s.id`
}

func scanSchedule(row interface{ Scan(...any) error }) (SecuritySchedule, error) {
	var s SecuritySchedule
	err := row.Scan(&s.ID, &s.AssignmentID, &s.AssignmentStartDate, &s.AssignmentEndDate, &s.RTID, &s.UserID, &s.OfficerName, &s.MembershipID, &s.ShiftConfigID, &s.ShiftVersion, &s.ShiftDate, &s.ShiftName, &s.StartTime, &s.EndTime, &s.CrossesMidnight, &s.LatenessToleranceMinutes, &s.CheckInWindowBeforeMinutes, &s.CheckInWindowAfterMinutes, &s.AbsentCutoffMinutes, &s.MissingCheckoutCutoffMinutes, &s.ScheduledStartAt, &s.ScheduledEndAt, &s.Notes, &s.Status, &s.AttendanceStatus, &s.CheckedInAt, &s.CheckedOutAt, &s.CreatedAt, &s.UpdatedAt)
	s.AttendanceStatusLabel = StatusLabel(s.AttendanceStatus)
	return s, err
}

func (r *Repository) CreateSchedulePeriod(ctx context.Context, rtID, actorID string, req CreateScheduleRequest, start, end, now time.Time) (*SecuritySchedule, error) {
	tx, err := r.pool.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	first, err := r.createSchedulePeriodTx(ctx, tx, rtID, actorID, req, start, end, now)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return first, nil
}

func (r *Repository) createSchedulePeriodTx(ctx context.Context, tx *sql.Tx, rtID, actorID string, req CreateScheduleRequest, start, end, now time.Time) (*SecuritySchedule, error) {
	if err := lockOfficerSchedules(ctx, tx, rtID, req.UserID); err != nil {
		return nil, err
	}
	membership, err := r.ValidateActiveOfficer(ctx, tx, rtID, req.UserID)
	if err != nil {
		return nil, err
	}
	cfg, err := loadActiveShift(ctx, tx, rtID, req.ShiftConfigID)
	if err != nil {
		return nil, err
	}
	assignmentID := uuid.NewString()
	var first SecuritySchedule
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		bs, be, cross, e := ComputeShiftBounds(day, cfg.StartTime, cfg.EndTime, cfg.CrossesMidnight)
		if e != nil {
			return nil, newAppError(400, "invalid_shift_time", e.Error(), ErrValidation)
		}
		var conflict string
		e = tx.QueryRowContext(ctx, `SELECT to_char(shift_date,'YYYY-MM-DD') FROM security_schedules WHERE rt_id=$1 AND user_id=$2 AND status='scheduled' AND scheduled_start_at<$3 AND scheduled_end_at>$4 ORDER BY shift_date LIMIT 1`, rtID, req.UserID, be, bs).Scan(&conflict)
		if e == nil {
			return nil, newAppError(409, "schedule_conflict", "Jadwal bentrok pada tanggal "+conflict, ErrConflict)
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return nil, e
		}
		date := day.Format("2006-01-02")
		res, e := tx.ExecContext(ctx, `INSERT INTO security_schedules(rt_id,assignment_id,assignment_start_date,assignment_end_date,user_id,membership_id,shift_config_id,shift_version,shift_date,shift_name_snapshot,start_time_snapshot,end_time_snapshot,crosses_midnight_snapshot,lateness_tolerance_minutes,check_in_window_before_minutes,check_in_window_after_minutes,absent_cutoff_minutes,missing_checkout_cutoff_minutes,scheduled_start_at,scheduled_end_at,notes,status,created_by,created_at,updated_at) VALUES($1,$2,$3::date,$4::date,$5,$6,$7,$8,$9::date,$10,$11::time,$12::time,$13,$14,$15,$16,$17,$18,$19,$20,$21,'scheduled',$22,$23,$23)`, rtID, assignmentID, start.Format("2006-01-02"), end.Format("2006-01-02"), req.UserID, membership, cfg.ID, cfg.Version, date, cfg.Name, cfg.StartTime, cfg.EndTime, cross, cfg.LatenessToleranceMinutes, cfg.CheckInWindowBeforeMinutes, cfg.CheckInWindowAfterMinutes, cfg.AbsentCutoffMinutes, cfg.MissingCheckoutCutoffMinutes, bs, be, req.Notes, actorID, now)
		if e != nil {
			return nil, e
		}
		_ = res
		var id string
		e = tx.QueryRowContext(ctx, `SELECT id FROM security_schedules WHERE rt_id=$1 AND assignment_id=$2 AND shift_date=$3::date`, rtID, assignmentID, date).Scan(&id)
		if e != nil {
			return nil, e
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO security_attendance_records(rt_id,schedule_id,user_id,shift_config_id,shift_version,shift_date,status,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6::date,'belum_hadir',$7,$7)`, rtID, id, req.UserID, cfg.ID, cfg.Version, date, now); e != nil {
			return nil, e
		}
		if day.Equal(start) {
			first, e = scanSchedule(tx.QueryRowContext(ctx, scheduleProjectionSQL()+` WHERE s.id=$1`, id))
			if e != nil {
				return nil, e
			}
		}
	}
	actor := actorID
	newVal := map[string]any{"assignment_id": assignmentID, "user_id": req.UserID, "shift_config_id": cfg.ID, "start_date": start.Format("2006-01-02"), "end_date": end.Format("2006-01-02")}
	if err := insertAuditLog(ctx, tx, rtID, &actor, "security.schedule.created", "security_schedule", assignmentID, nil, newVal, ""); err != nil {
		return nil, err
	}
	return &first, nil
}

func hasAttendanceCorrections(ctx context.Context, tx *sql.Tx, scheduleID string) bool {
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM security_attendance_corrections WHERE schedule_id=$1`, scheduleID).Scan(&count); err != nil {
		return true
	}
	return count > 0
}

func isRecordedAttendance(x periodScheduleRow) bool {
	return x.checkedIn.Valid || x.checkedOut.Valid || (x.attendanceStatus.Valid && x.attendanceStatus.String != StatusBelumHadir)
}
func attendanceLockedError(date time.Time) error {
	return newAppError(409, "attendance_locked", "Jadwal tanggal "+date.Format("2006-01-02")+" sudah memiliki riwayat absensi dan tidak dapat diubah", ErrConflict)
}

func (r *Repository) UpdateSchedulePeriod(ctx context.Context, rtID, scheduleID, actorID string, req UpdateScheduleRequest, start, end, now time.Time) (*SecuritySchedule, error) {
	tx, err := r.pool.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	out, err := r.updateSchedulePeriodTx(ctx, tx, rtID, scheduleID, actorID, req, start, end, now)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Repository) updateSchedulePeriodTx(ctx context.Context, tx *sql.Tx, rtID, scheduleID, actorID string, req UpdateScheduleRequest, start, end, now time.Time) (*SecuritySchedule, error) {
	var groupID, currentUser string
	var oldStart, oldEnd time.Time
	err := tx.QueryRowContext(ctx, `SELECT assignment_id,user_id,assignment_start_date,assignment_end_date FROM security_schedules WHERE (id=$1 OR assignment_id=$1) AND rt_id=$2 ORDER BY shift_date LIMIT 1`, scheduleID, rtID).Scan(&groupID, &currentUser, &oldStart, &oldEnd)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, newAppError(404, "not_found", "schedule not found", ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	users := []string{currentUser, req.UserID}
	if users[0] > users[1] {
		users[0], users[1] = users[1], users[0]
	}
	if err = lockOfficerSchedules(ctx, tx, rtID, users[0]); err != nil {
		return nil, err
	}
	if users[1] != users[0] {
		if err = lockOfficerSchedules(ctx, tx, rtID, users[1]); err != nil {
			return nil, err
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT s.id,s.user_id,s.status,s.shift_config_id,s.shift_date,s.scheduled_start_at,s.scheduled_end_at,ar.status,ar.checked_in_at,ar.checked_out_at FROM security_schedules s LEFT JOIN security_attendance_records ar ON ar.schedule_id=s.id WHERE s.rt_id=$1 AND s.assignment_id=$2 ORDER BY s.shift_date FOR UPDATE OF s`, rtID, groupID)
	if err != nil {
		return nil, err
	}
	var existing []periodScheduleRow
	for rows.Next() {
		var x periodScheduleRow
		if err = rows.Scan(&x.id, &x.userID, &x.status, &x.shiftConfigID, &x.shiftDate, &x.startAt, &x.endAt, &x.attendanceStatus, &x.checkedIn, &x.checkedOut); err != nil {
			rows.Close()
			return nil, err
		}
		existing = append(existing, x)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	if len(existing) == 0 {
		return nil, newAppError(404, "not_found", "schedule not found", ErrNotFound)
	}
	membership, err := r.ValidateActiveOfficer(ctx, tx, rtID, req.UserID)
	if err != nil {
		return nil, err
	}
	cfg, err := loadActiveShift(ctx, tx, rtID, req.ShiftConfigID)
	if err != nil {
		return nil, err
	}
	today := now.In(JakartaLoc)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, JakartaLoc)
	todayKey := today.Format("2006-01-02")
	byDate := map[string]periodScheduleRow{}
	for _, x := range existing {
		byDate[x.shiftDate.In(JakartaLoc).Format("2006-01-02")] = x
	}
	identical := req.UserID == currentUser && req.ShiftConfigID == existing[0].shiftConfigID && start.Format("2006-01-02") == oldStart.Format("2006-01-02") && end.Format("2006-01-02") == oldEnd.Format("2006-01-02")
	for _, x := range existing {
		if x.shiftDate.In(JakartaLoc).Format("2006-01-02") >= todayKey && x.status == ScheduleStatusScheduled && (x.startAt.In(JakartaLoc).Format("15:04:05") != cfg.StartTime || x.endAt.In(JakartaLoc).Format("15:04:05") != cfg.EndTime) {
			identical = false
		}
	}
	if identical {
		if _, err = tx.ExecContext(ctx, `UPDATE security_schedules SET notes=$1,updated_by=$2,updated_at=$3 WHERE rt_id=$4 AND assignment_id=$5`, req.Notes, actorID, now, rtID, groupID); err != nil {
			return nil, err
		}
		out, err := scanSchedule(tx.QueryRowContext(ctx, scheduleProjectionSQL()+` WHERE (s.id=$1 OR s.assignment_id=$1) AND s.rt_id=$2 ORDER BY s.shift_date LIMIT 1`, scheduleID, rtID))
		if err != nil {
			return nil, err
		}
		return &out, nil
	}
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		bs, be, _, e := ComputeShiftBounds(day, cfg.StartTime, cfg.EndTime, cfg.CrossesMidnight)
		if e != nil {
			return nil, e
		}
		var conflict string
		e = tx.QueryRowContext(ctx, `SELECT to_char(shift_date,'YYYY-MM-DD') FROM security_schedules WHERE rt_id=$1 AND user_id=$2 AND status='scheduled' AND assignment_id<>$3 AND scheduled_start_at<$4 AND scheduled_end_at>$5 ORDER BY shift_date LIMIT 1`, rtID, req.UserID, groupID, be, bs).Scan(&conflict)
		if e == nil {
			return nil, newAppError(409, "schedule_conflict", "Jadwal bentrok pada tanggal "+conflict, ErrConflict)
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return nil, e
		}
		if x, ok := byDate[day.Format("2006-01-02")]; ok && !day.Before(today) && isRecordedAttendance(x) {
			return nil, attendanceLockedError(day)
		}
	}
	for _, x := range existing {
		d := x.shiftDate.In(JakartaLoc)
		date := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, JakartaLoc)
		key := date.Format("2006-01-02")
		if key < todayKey {
			continue
		}
		inside := !date.Before(start) && !date.After(end)
		if !inside {
			if x.status == ScheduleStatusScheduled {
				if isRecordedAttendance(x) || hasAttendanceCorrections(ctx, tx, x.id) {
					return nil, attendanceLockedError(date)
				}
				if _, err = tx.ExecContext(ctx, `UPDATE security_schedules SET status='cancelled',updated_by=$2,updated_at=$3 WHERE id=$1 AND rt_id=$4`, x.id, actorID, now, rtID); err != nil {
					return nil, err
				}
			}
			continue
		}
		bs, be, cross, e := ComputeShiftBounds(date, cfg.StartTime, cfg.EndTime, cfg.CrossesMidnight)
		if e != nil {
			return nil, e
		}
		if x.status == ScheduleStatusScheduled && x.userID == req.UserID && x.shiftConfigID == cfg.ID && x.startAt.Equal(bs) && x.endAt.Equal(be) {
			continue
		}
		if isRecordedAttendance(x) || hasAttendanceCorrections(ctx, tx, x.id) {
			return nil, attendanceLockedError(date)
		}
		_, err = tx.ExecContext(ctx, `UPDATE security_schedules SET status='scheduled',user_id=$2,membership_id=$3,shift_config_id=$4,shift_version=$5,shift_name_snapshot=$6,start_time_snapshot=$7::time,end_time_snapshot=$8::time,crosses_midnight_snapshot=$9,lateness_tolerance_minutes=$10,check_in_window_before_minutes=$11,check_in_window_after_minutes=$12,absent_cutoff_minutes=$13,missing_checkout_cutoff_minutes=$14,scheduled_start_at=$15,scheduled_end_at=$16,notes=$17,updated_by=$18,updated_at=$19 WHERE id=$1 AND rt_id=$20`, x.id, req.UserID, membership, cfg.ID, cfg.Version, cfg.Name, cfg.StartTime, cfg.EndTime, cross, cfg.LatenessToleranceMinutes, cfg.CheckInWindowBeforeMinutes, cfg.CheckInWindowAfterMinutes, cfg.AbsentCutoffMinutes, cfg.MissingCheckoutCutoffMinutes, bs, be, req.Notes, actorID, now, rtID)
		if err != nil {
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE security_attendance_records SET user_id=$2,shift_config_id=$3,shift_version=$4 WHERE schedule_id=$1 AND rt_id=$5 AND status='belum_hadir' AND checked_in_at IS NULL AND checked_out_at IS NULL`, x.id, req.UserID, cfg.ID, cfg.Version, rtID); err != nil {
			return nil, err
		}
	}
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		if old, ok := byDate[key]; ok {
			if old.status == ScheduleStatusScheduled || old.status == ScheduleStatusCancelled {
				continue
			}
		}
		if key < todayKey {
			continue
		}
		bs, be, cross, e := ComputeShiftBounds(day, cfg.StartTime, cfg.EndTime, cfg.CrossesMidnight)
		if e != nil {
			return nil, e
		}
		var id string
		err = tx.QueryRowContext(ctx, `INSERT INTO security_schedules(rt_id,assignment_id,assignment_start_date,assignment_end_date,user_id,membership_id,shift_config_id,shift_version,shift_date,shift_name_snapshot,start_time_snapshot,end_time_snapshot,crosses_midnight_snapshot,lateness_tolerance_minutes,check_in_window_before_minutes,check_in_window_after_minutes,absent_cutoff_minutes,missing_checkout_cutoff_minutes,scheduled_start_at,scheduled_end_at,notes,status,created_by,updated_by,created_at,updated_at) VALUES($1,$2,$3::date,$4::date,$5,$6,$7,$8,$9::date,$10,$11::time,$12::time,$13,$14,$15,$16,$17,$18,$19,$20,$21,'scheduled',$22,$22,$23,$23) RETURNING id`, rtID, groupID, start.Format("2006-01-02"), end.Format("2006-01-02"), req.UserID, membership, cfg.ID, cfg.Version, key, cfg.Name, cfg.StartTime, cfg.EndTime, cross, cfg.LatenessToleranceMinutes, cfg.CheckInWindowBeforeMinutes, cfg.CheckInWindowAfterMinutes, cfg.AbsentCutoffMinutes, cfg.MissingCheckoutCutoffMinutes, bs, be, req.Notes, actorID, now).Scan(&id)
		if err != nil {
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO security_attendance_records(rt_id,schedule_id,user_id,shift_config_id,shift_version,shift_date,status,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6::date,'belum_hadir',$7,$7)`, rtID, id, req.UserID, cfg.ID, cfg.Version, key, now); err != nil {
			return nil, err
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE security_schedules SET assignment_start_date=$3::date,assignment_end_date=$4::date WHERE rt_id=$1 AND assignment_id=$2 AND status='scheduled' AND shift_date >= $5::date`, rtID, groupID, start.Format("2006-01-02"), end.Format("2006-01-02"), todayKey); err != nil {
		return nil, err
	}
	old := map[string]any{"user_id": currentUser, "start_date": oldStart.Format("2006-01-02"), "end_date": oldEnd.Format("2006-01-02")}
	newVal := map[string]any{"user_id": req.UserID, "shift_config_id": cfg.ID, "start_date": start.Format("2006-01-02"), "end_date": end.Format("2006-01-02")}
	if err = insertAuditLog(ctx, tx, rtID, &actorID, "security.schedule.updated", "security_schedule", groupID, old, newVal, ""); err != nil {
		return nil, err
	}
	out, err := scanSchedule(tx.QueryRowContext(ctx, scheduleProjectionSQL()+` WHERE (s.id=$1 OR s.assignment_id=$1) AND s.rt_id=$2 ORDER BY s.shift_date LIMIT 1`, scheduleID, rtID))
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *Repository) GetScheduleAssignment(ctx context.Context, rtID, scheduleID string) ([]SecuritySchedule, error) {
	var groupID string
	err := r.pool.Raw().QueryRowContext(ctx, `SELECT assignment_id FROM security_schedules WHERE id=$1 AND rt_id=$2`, scheduleID, rtID).Scan(&groupID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, newAppError(404, "not_found", "schedule not found", ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Raw().QueryContext(ctx, scheduleProjectionSQL()+` WHERE s.rt_id=$1 AND s.assignment_id=$2 ORDER BY s.shift_date`, rtID, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SecuritySchedule{}
	for rows.Next() {
		x, e := scanSchedule(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
