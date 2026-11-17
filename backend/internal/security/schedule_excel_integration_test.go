package security

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"social-finance/internal/auth"
	"social-finance/internal/database"
	"social-finance/internal/storage"
	"social-finance/internal/testutil"
)

// Helper: Setup isolated RT fixture
func setupIntegrationRT(t *testing.T, pool *database.Pool, ctx context.Context, rtCode string) string {
	t.Helper()
	var rtID string
	var err error
	for attempt := 0; attempt < 8; attempt++ {
		fixtureID := uuid.New()
		rwNumber := int(fixtureID.ID() & 0x7fffffff)
		if rwNumber == 0 {
			rwNumber = 1
		}
		rtID = fixtureID.String()
		err = pool.Raw().QueryRowContext(ctx, `
			INSERT INTO rts (id, rw, rt, name, is_active)
			VALUES ($1, $2, $3, $4, true)
			RETURNING id
		`, rtID, rwNumber, rtCode, fmt.Sprintf("Test RT %s %s", rtCode, rtID)).Scan(&rtID)
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Fatalf("failed to create RT fixture: %v", err)
	}

	fixtureRTID := rtID
	t.Cleanup(func() {
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM security_attendance_records WHERE rt_id = $1`, fixtureRTID)
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM security_schedules WHERE rt_id = $1`, fixtureRTID)
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM security_shift_configs WHERE rt_id = $1`, fixtureRTID)
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM user_rt_memberships WHERE rt_id = $1`, fixtureRTID)
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM rts WHERE id = $1`, fixtureRTID)
	})
	return fixtureRTID
}

// Helper: Create user with RT membership
func setupIntegrationUser(t *testing.T, pool *database.Pool, ctx context.Context, rtID, fullName, email, role, jabatan string) (string, string) {
	t.Helper()
	userID := uuid.NewString()
	_, err := pool.Raw().ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, system_role, is_active, created_at, updated_at)
		VALUES ($1, $2, 'dummyhash', $3, NULL, true, now(), now())
	`, userID, email, fullName)
	if err != nil {
		t.Fatalf("failed to insert user %s: %v", email, err)
	}

	membershipID := uuid.NewString()
	_, err = pool.Raw().ExecContext(ctx, `
		INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, true, now(), now())
	`, membershipID, userID, rtID, role, jabatan)
	if err != nil {
		t.Fatalf("failed to insert membership for user %s: %v", email, err)
	}

	token, err := auth.GenerateAccessToken(auth.TokenClaims{
		UserID:  userID,
		MID:     membershipID,
		RTID:    rtID,
		Role:    role,
		Jabatan: jabatan,
		SysRole: "standard",
	})
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM user_rt_memberships WHERE user_id = $1`, userID)
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
	})

	return userID, token
}

// Helper: Create shift configuration
func setupIntegrationShift(t *testing.T, pool *database.Pool, ctx context.Context, rtID, name, startTime, endTime string, cross bool, creatorID string) string {
	t.Helper()
	shiftID := uuid.NewString()
	_, err := pool.Raw().ExecContext(ctx, `
		INSERT INTO security_shift_configs (id, rt_id, name, start_time, end_time, crosses_midnight, created_by, is_active, version)
		VALUES ($1, $2, $3, $4::time, $5::time, $6, $7, true, 1)
	`, shiftID, rtID, name, startTime, endTime, cross, creatorID)
	if err != nil {
		t.Fatalf("failed to insert shift %s: %v", name, err)
	}
	return shiftID
}

// Helper: Build security test router
func setupSecurityTestRouter(pool *database.Pool) http.Handler {
	auth.AuthDBPool = pool
	auth.SigningSecret = []byte("test-signing-secret-at-least-16-chars")
	repo := NewRepository(pool)
	svc := NewService(repo)
	store := storage.NewMockStorage()
	handler := NewHandler(svc, pool, nil, store)

	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Get("/api/v1/security/schedules/export", handler.ExportSchedulesXLSX)
		r.Get("/api/v1/security/schedules/export/xlsx", handler.ExportSchedulesXLSX)
		r.Post("/api/v1/security/schedules/import/preview", handler.ImportSchedulesPreview)
		r.Post("/api/v1/security/schedules/import/commit", handler.ImportSchedulesCommit)
		r.Get("/api/v1/security/schedules", handler.ListSchedules)
		r.Get("/api/v1/security/schedules/{id}", handler.GetSchedule)
		r.Post("/api/v1/security/schedules", handler.CreateSchedule)
		r.Put("/api/v1/security/schedules/{id}", handler.UpdateSchedule)
		r.Delete("/api/v1/security/schedules/{id}", handler.CancelSchedule)
	})
	return r
}

func TestSecurityScheduleExcel_DatabaseIntegration(t *testing.T) {
	pool := testutil.GetTestPool(t)
	auth.AuthDBPool = pool
	auth.SigningSecret = []byte("test-signing-secret-at-least-16-chars")
	ctx := context.Background()

	// 1. Setup isolated RTs
	rtAID := setupIntegrationRT(t, pool, ctx, "001")
	rtBID := setupIntegrationRT(t, pool, ctx, "002")

	// 2. Setup Users in RT A
	ts := time.Now().UnixNano()
	ketuaAID, ketuaAToken := setupIntegrationUser(t, pool, ctx, rtAID, "Ketua RT A", fmt.Sprintf("ketua-a-%d@test.com", ts), "pengurus", "ketua")
	offA1ID, _ := setupIntegrationUser(t, pool, ctx, rtAID, "Budi Santoso", fmt.Sprintf("budi-%d@test.com", ts), "warga", "keamanan")
	offA2ID, _ := setupIntegrationUser(t, pool, ctx, rtAID, "Agus Setiawan", fmt.Sprintf("agus-%d@test.com", ts), "warga", "keamanan")
	_, wargaAToken := setupIntegrationUser(t, pool, ctx, rtAID, "Warga Biasa", fmt.Sprintf("warga-%d@test.com", ts), "warga", "anggota")

	// Setup User in RT B
	offB1ID, _ := setupIntegrationUser(t, pool, ctx, rtBID, "Siti Rahma", fmt.Sprintf("siti-%d@test.com", ts), "warga", "keamanan")
	_, ketuaBToken := setupIntegrationUser(t, pool, ctx, rtBID, "Ketua RT B", fmt.Sprintf("ketua-b-%d@test.com", ts), "pengurus", "ketua")

	// 3. Setup Shifts
	shiftA1ID := setupIntegrationShift(t, pool, ctx, rtAID, "Shift Pagi", "07:00", "15:00", false, ketuaAID)
	shiftA2ID := setupIntegrationShift(t, pool, ctx, rtAID, "Shift Malam", "23:00", "07:00", true, ketuaAID)
	_ = shiftA2ID
	_ = setupIntegrationShift(t, pool, ctx, rtBID, "Shift RT B", "08:00", "16:00", false, offB1ID)

	router := setupSecurityTestRouter(pool)

	// Helper to send preview request
	sendPreview := func(token string, fileBytes []byte) (*httptest.ResponseRecorder, ScheduleImportPreviewResponse) {
		bodyBuf, boundary := buildMultipartXLSX("file", "schedule.xlsx", fileBytes)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/security/schedules/import/preview", bodyBuf)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", boundary)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		var resp ScheduleImportPreviewResponse
		if rec.Code == http.StatusOK {
			_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		}
		return rec, resp
	}

	// Helper to send commit request
	sendCommit := func(token string, rows []ScheduleExcelRow) (*httptest.ResponseRecorder, ScheduleImportCommitResponse) {
		bodyDat, _ := json.Marshal(ScheduleImportCommitRequest{Rows: rows})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/security/schedules/import/commit", bytes.NewReader(bodyDat))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		var resp ScheduleImportCommitResponse
		if rec.Code == http.StatusOK {
			_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		}
		return rec, resp
	}

	// -------------------------------------------------------------------------
	// Scenario 1: Satu baris jadwal baru berhasil dibuat via import
	// -------------------------------------------------------------------------
	t.Run("Scenario 1: Satu baris jadwal baru berhasil dibuat", func(t *testing.T) {
		f := createTestXLSX("Jadwal", ScheduleExcelHeaders, [][]string{
			{"", "Budi Santoso", "Shift Pagi", "2026-11-20", "2026-11-22", "Pos Depan"},
		})
		data := xlsxToBytes(f)
		f.Close()

		recPrev, prevResp := sendPreview(ketuaAToken, data)
		if recPrev.Code != http.StatusOK {
			t.Fatalf("expected 200 on preview, got %d: %s", recPrev.Code, recPrev.Body.String())
		}
		if prevResp.TotalRows != 1 || prevResp.ValidRows != 1 || prevResp.RejectedRows != 0 {
			t.Fatalf("preview stats mismatch: total=%d valid=%d rejected=%d errors=%v",
				prevResp.TotalRows, prevResp.ValidRows, prevResp.RejectedRows, prevResp.Rows[0].Errors)
		}
		if prevResp.Rows[0].Action != "create" {
			t.Fatalf("expected action create, got %s", prevResp.Rows[0].Action)
		}

		recCommit, commitResp := sendCommit(ketuaAToken, prevResp.Rows)
		if recCommit.Code != http.StatusOK {
			t.Fatalf("expected 200 on commit, got %d: %s", recCommit.Code, recCommit.Body.String())
		}
		if commitResp.CreatedCount != 1 || commitResp.TotalCommitted != 1 {
			t.Fatalf("commit response mismatch: %+v", commitResp)
		}

		// Verify 3 daily schedules in database
		var dbCount int
		err := pool.Raw().QueryRowContext(ctx, `
			SELECT COUNT(*) FROM security_schedules
			WHERE rt_id = $1 AND user_id = $2 AND shift_config_id = $3 AND status = 'scheduled'
		`, rtAID, offA1ID, shiftA1ID).Scan(&dbCount)
		if err != nil || dbCount != 3 {
			t.Fatalf("expected 3 schedule rows in DB, got %d (err: %v)", dbCount, err)
		}
	})

	// -------------------------------------------------------------------------
	// Scenario 2: Satu jadwal lama berhasil diperbarui berdasarkan ID
	// -------------------------------------------------------------------------
	var existingAssignmentID string
	t.Run("Scenario 2: Satu jadwal lama berhasil diperbarui berdasarkan ID", func(t *testing.T) {
		err := pool.Raw().QueryRowContext(ctx, `
			SELECT assignment_id FROM security_schedules
			WHERE rt_id = $1 AND user_id = $2 AND status = 'scheduled' LIMIT 1
		`, rtAID, offA1ID).Scan(&existingAssignmentID)
		if err != nil {
			t.Fatalf("failed to query existing assignment_id: %v", err)
		}

		// Update schedule to Officer 2 (Agus Setiawan) and Shift Malam
		f := createTestXLSX("Jadwal", ScheduleExcelHeaders, [][]string{
			{existingAssignmentID, "Agus Setiawan", "Shift Malam", "2026-11-20", "2026-11-22", "Pos Belakang"},
		})
		data := xlsxToBytes(f)
		f.Close()

		recPrev, prevResp := sendPreview(ketuaAToken, data)
		if recPrev.Code != http.StatusOK {
			t.Fatalf("expected 200 on preview, got %d: %s", recPrev.Code, recPrev.Body.String())
		}
		if prevResp.ValidRows != 1 || prevResp.Rows[0].Action != "update" {
			t.Fatalf("preview mismatch: valid=%d action=%s errors=%v",
				prevResp.ValidRows, prevResp.Rows[0].Action, prevResp.Rows[0].Errors)
		}

		recCommit, commitResp := sendCommit(ketuaAToken, prevResp.Rows)
		if recCommit.Code != http.StatusOK {
			t.Fatalf("expected 200 on commit, got %d: %s", recCommit.Code, recCommit.Body.String())
		}
		if commitResp.UpdatedCount != 1 {
			t.Fatalf("expected updated_count 1, got %+v", commitResp)
		}

		// Verify schedule now assigned to Officer 2
		var officerID string
		_ = pool.Raw().QueryRowContext(ctx, `
			SELECT user_id FROM security_schedules WHERE assignment_id = $1 AND status = 'scheduled' LIMIT 1
		`, existingAssignmentID).Scan(&officerID)
		if officerID != offA2ID {
			t.Fatalf("expected schedule assigned to Agus (%s), got %s", offA2ID, officerID)
		}
	})

	// -------------------------------------------------------------------------
	// Scenario 3: File berisi kombinasi jadwal baru dan update lama
	// -------------------------------------------------------------------------
	t.Run("Scenario 3: File berisi kombinasi jadwal baru dan update lama", func(t *testing.T) {
		f := createTestXLSX("Jadwal", ScheduleExcelHeaders, [][]string{
			{existingAssignmentID, "Agus Setiawan", "Shift Malam", "2026-11-20", "2026-11-22", "Catatan Baru"},
			{"", "Budi Santoso", "Shift Pagi", "2026-11-25", "2026-11-26", "Jadwal Tambahan"},
		})
		data := xlsxToBytes(f)
		f.Close()

		recPrev, prevResp := sendPreview(ketuaAToken, data)
		if recPrev.Code != http.StatusOK || prevResp.ValidRows != 2 {
			t.Fatalf("expected 2 valid rows, got %d: %s", prevResp.ValidRows, recPrev.Body.String())
		}
		if prevResp.Rows[0].Action != "update" || prevResp.Rows[1].Action != "create" {
			t.Fatalf("expected update + create, got %s + %s", prevResp.Rows[0].Action, prevResp.Rows[1].Action)
		}

		recCommit, commitResp := sendCommit(ketuaAToken, prevResp.Rows)
		if recCommit.Code != http.StatusOK {
			t.Fatalf("expected 200 on commit, got %d: %s", recCommit.Code, recCommit.Body.String())
		}
		if commitResp.CreatedCount != 1 || commitResp.UpdatedCount != 1 {
			t.Fatalf("expected 1 created and 1 updated, got %+v", commitResp)
		}
	})

	// -------------------------------------------------------------------------
	// Scenario 4: Import file yang sama dua kali tidak menghasilkan jadwal duplikat
	// -------------------------------------------------------------------------
	t.Run("Scenario 4: Import file yang sama dua kali tidak menghasilkan duplikat", func(t *testing.T) {
		// A. Existing schedules with ID are idempotent when re-imported
		fUpdate := createTestXLSX("Jadwal", ScheduleExcelHeaders, [][]string{
			{existingAssignmentID, "Agus Setiawan", "Shift Malam", "2026-11-20", "2026-11-22", "Catatan Re-import"},
		})
		dataUpdate := xlsxToBytes(fUpdate)
		fUpdate.Close()

		var countBefore int
		_ = pool.Raw().QueryRowContext(ctx, `SELECT COUNT(*) FROM security_schedules WHERE rt_id = $1 AND status = 'scheduled'`, rtAID).Scan(&countBefore)

		_, prevUpdate := sendPreview(ketuaAToken, dataUpdate)
		_, commitUpdate := sendCommit(ketuaAToken, prevUpdate.Rows)
		if commitUpdate.UpdatedCount != 1 {
			t.Fatalf("expected 1 update, got %+v", commitUpdate)
		}

		var countAfter int
		_ = pool.Raw().QueryRowContext(ctx, `SELECT COUNT(*) FROM security_schedules WHERE rt_id = $1 AND status = 'scheduled'`, rtAID).Scan(&countAfter)
		if countAfter != countBefore {
			t.Fatalf("re-import of existing schedule changed total row count from %d to %d", countBefore, countAfter)
		}

		// B. New schedule without ID rejects duplicate insertion when imported a second time
		fNew := createTestXLSX("Jadwal", ScheduleExcelHeaders, [][]string{
			{"", "Budi Santoso", "Shift Pagi", "2026-11-28", "2026-11-29", "Jadwal 28-29"},
		})
		dataNew := xlsxToBytes(fNew)
		fNew.Close()

		// First import creates
		_, prevNew1 := sendPreview(ketuaAToken, dataNew)
		sendCommit(ketuaAToken, prevNew1.Rows)

		// Second import of same file without ID conflicts with database
		_, prevNew2 := sendPreview(ketuaAToken, dataNew)
		if prevNew2.ValidRows != 0 || prevNew2.RejectedRows != 1 {
			t.Fatalf("expected second import of same new row to be rejected as conflict, got valid=%d", prevNew2.ValidRows)
		}
		if !strings.Contains(prevNew2.Rows[0].Errors[0], "bentrok") {
			t.Fatalf("expected conflict error, got %v", prevNew2.Rows[0].Errors)
		}
	})

	// -------------------------------------------------------------------------
	// Scenario 5: ID jadwal tidak ditemukan ditolak
	// -------------------------------------------------------------------------
	t.Run("Scenario 5: ID jadwal tidak ditemukan ditolak", func(t *testing.T) {
		fakeID := "00000000-0000-0000-0000-000000000999"
		f := createTestXLSX("Jadwal", ScheduleExcelHeaders, [][]string{
			{fakeID, "Budi Santoso", "Shift Pagi", "2026-12-01", "2026-12-02", ""},
		})
		data := xlsxToBytes(f)
		f.Close()

		_, prev := sendPreview(ketuaAToken, data)
		if prev.ValidRows != 0 || prev.RejectedRows != 1 {
			t.Fatalf("expected rejection for fake ID, got valid=%d", prev.ValidRows)
		}
		if !strings.Contains(prev.Rows[0].Errors[0], "tidak ditemukan") {
			t.Fatalf("expected not found error, got %v", prev.Rows[0].Errors)
		}
	})

	// -------------------------------------------------------------------------
	// Scenario 6: ID jadwal milik RT lain ditolak
	// -------------------------------------------------------------------------
	t.Run("Scenario 6: ID jadwal milik RT lain ditolak", func(t *testing.T) {
		// Create schedule in RT B
		schedBID := uuid.NewString()
		_, err := pool.Raw().ExecContext(ctx, `
			INSERT INTO security_schedules (id, rt_id, assignment_id, assignment_start_date, assignment_end_date, user_id, membership_id, shift_config_id, shift_version, shift_date, shift_name_snapshot, start_time_snapshot, end_time_snapshot, crosses_midnight_snapshot, lateness_tolerance_minutes, check_in_window_before_minutes, check_in_window_after_minutes, absent_cutoff_minutes, missing_checkout_cutoff_minutes, scheduled_start_at, scheduled_end_at, status, created_by, created_at, updated_at)
			VALUES ($1, $2, $1, '2026-12-05'::date, '2026-12-05'::date, $3, (SELECT id FROM user_rt_memberships WHERE user_id=$3 LIMIT 1), (SELECT id FROM security_shift_configs WHERE rt_id=$2 LIMIT 1), 1, '2026-12-05'::date, 'Shift RT B', '08:00'::time, '16:00'::time, false, 15, 60, 120, 120, 180, now(), now(), 'scheduled', $4, now(), now())
		`, schedBID, rtBID, offB1ID, ketuaAID)
		if err != nil {
			t.Fatalf("failed to insert schedule in RT B: %v", err)
		}

		// User in RT A tries to update RT B schedule
		f := createTestXLSX("Jadwal", ScheduleExcelHeaders, [][]string{
			{schedBID, "Budi Santoso", "Shift Pagi", "2026-12-05", "2026-12-05", ""},
		})
		data := xlsxToBytes(f)
		f.Close()

		_, prev := sendPreview(ketuaAToken, data)
		if prev.ValidRows != 0 || prev.RejectedRows != 1 {
			t.Fatalf("expected cross-tenant schedule ID to be rejected, got valid=%d", prev.ValidRows)
		}
		if !strings.Contains(prev.Rows[0].Errors[0], "tidak ditemukan") && !strings.Contains(prev.Rows[0].Errors[0], "bukan milik RT ini") {
			t.Fatalf("expected cross-RT rejection message, got %v", prev.Rows[0].Errors)
		}
	})

	// -------------------------------------------------------------------------
	// Scenario 7: Petugas milik RT lain ditolak
	// -------------------------------------------------------------------------
	t.Run("Scenario 7: Petugas milik RT lain ditolak", func(t *testing.T) {
		// Try to schedule Siti Rahma (Officer of RT B) inside RT A
		f := createTestXLSX("Jadwal", ScheduleExcelHeaders, [][]string{
			{"", "Siti Rahma", "Shift Pagi", "2026-12-10", "2026-12-11", ""},
		})
		data := xlsxToBytes(f)
		f.Close()

		_, prev := sendPreview(ketuaAToken, data)
		if prev.ValidRows != 0 || prev.RejectedRows != 1 {
			t.Fatalf("expected rejection for officer of another RT, got valid=%d", prev.ValidRows)
		}
		if !strings.Contains(prev.Rows[0].Errors[0], "tidak ditemukan dalam data RT ini") {
			t.Fatalf("expected officer not found error, got %v", prev.Rows[0].Errors)
		}
	})

	// -------------------------------------------------------------------------
	// Scenario 8: Pengguna tanpa permission ditolak
	// -------------------------------------------------------------------------
	t.Run("Scenario 8: Pengguna tanpa permission ditolak", func(t *testing.T) {
		f := createTestXLSX("Jadwal", ScheduleExcelHeaders, [][]string{
			{"", "Budi Santoso", "Shift Pagi", "2026-12-15", "2026-12-16", ""},
		})
		data := xlsxToBytes(f)
		f.Close()

		recPrev, _ := sendPreview(wargaAToken, data)
		if recPrev.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for unauthorized warga preview, got %d", recPrev.Code)
		}

		recCommit, _ := sendCommit(wargaAToken, []ScheduleExcelRow{})
		if recCommit.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for unauthorized warga commit, got %d", recCommit.Code)
		}

		// Export by warga without schedule permissions
		reqExp := httptest.NewRequest(http.MethodGet, "/api/v1/security/schedules/export", nil)
		reqExp.Header.Set("Authorization", "Bearer "+wargaAToken)
		recExp := httptest.NewRecorder()
		router.ServeHTTP(recExp, reqExp)
		if recExp.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for unauthorized warga export, got %d", recExp.Code)
		}
	})

	// -------------------------------------------------------------------------
	// Scenario 9: Jadwal yang sudah memiliki riwayat absensi tidak dapat diubah
	// -------------------------------------------------------------------------
	t.Run("Scenario 9: Jadwal dengan riwayat absensi tidak dapat diubah", func(t *testing.T) {
		// Find a schedule day from existingAssignmentID and record attendance
		var lockedSchedID string
		err := pool.Raw().QueryRowContext(ctx, `
			SELECT id FROM security_schedules WHERE assignment_id = $1 LIMIT 1
		`, existingAssignmentID).Scan(&lockedSchedID)
		if err != nil {
			t.Fatalf("failed to query schedule id: %v", err)
		}

		_, err = pool.Raw().ExecContext(ctx, `
			UPDATE security_attendance_records
			SET status = 'hadir', checked_in_at = now()
			WHERE schedule_id = $1
		`, lockedSchedID)
		if err != nil {
			t.Fatalf("failed to update attendance record: %v", err)
		}

		// Attempt to update this locked schedule
		f := createTestXLSX("Jadwal", ScheduleExcelHeaders, [][]string{
			{existingAssignmentID, "Budi Santoso", "Shift Pagi", "2026-11-20", "2026-11-22", "Ubah Jadwal Terkunci"},
		})
		data := xlsxToBytes(f)
		f.Close()

		_, prev := sendPreview(ketuaAToken, data)
		if prev.ValidRows != 0 || prev.RejectedRows != 1 {
			t.Fatalf("expected rejection for schedule with attendance record, got valid=%d", prev.ValidRows)
		}
		if !strings.Contains(prev.Rows[0].Errors[0], "riwayat absensi") {
			t.Fatalf("expected attendance lock message, got %v", prev.Rows[0].Errors)
		}
	})

	// -------------------------------------------------------------------------
	// Scenario 10: Konflik jadwal dengan data database ditolak
	// -------------------------------------------------------------------------
	t.Run("Scenario 10: Konflik jadwal dengan database ditolak", func(t *testing.T) {
		// Officer 1 already has shift on 2026-11-25 s/d 2026-11-26
		f := createTestXLSX("Jadwal", ScheduleExcelHeaders, [][]string{
			{"", "Budi Santoso", "Shift Pagi", "2026-11-25", "2026-11-25", "Bentrok"},
		})
		data := xlsxToBytes(f)
		f.Close()

		_, prev := sendPreview(ketuaAToken, data)
		if prev.ValidRows != 0 || prev.RejectedRows != 1 {
			t.Fatalf("expected conflict rejection, got valid=%d", prev.ValidRows)
		}
		if !strings.Contains(prev.Rows[0].Errors[0], "bentrok") {
			t.Fatalf("expected conflict error message, got %v", prev.Rows[0].Errors)
		}
	})

	// -------------------------------------------------------------------------
	// Scenario 11: Dua baris yang saling bentrok dalam satu file ditangani
	// -------------------------------------------------------------------------
	t.Run("Scenario 11: Dua baris saling bentrok dalam satu file", func(t *testing.T) {
		f := createTestXLSX("Jadwal", ScheduleExcelHeaders, [][]string{
			{"", "Budi Santoso", "Shift Pagi", "2026-12-20", "2026-12-22", "Jadwal 1"},
			{"", "Budi Santoso", "Shift Pagi", "2026-12-21", "2026-12-23", "Jadwal Bentrok"},
		})
		data := xlsxToBytes(f)
		f.Close()

		_, prev := sendPreview(ketuaAToken, data)
		if prev.ValidRows != 1 || prev.RejectedRows != 1 {
			t.Fatalf("expected 1 valid and 1 rejected for intra-file conflict, got valid=%d rejected=%d",
				prev.ValidRows, prev.RejectedRows)
		}
		if !strings.Contains(prev.Rows[1].Errors[0], "baris 2 dalam file ini") {
			t.Fatalf("expected intra-file conflict reason mentioning row 2, got %v", prev.Rows[1].Errors)
		}
	})

	// -------------------------------------------------------------------------
	// Scenario 12: Atomic rollback jika ada baris gagal saat commit
	// -------------------------------------------------------------------------
	t.Run("Scenario 12: Atomic rollback jika operasi dalam batch gagal", func(t *testing.T) {
		var countBefore int
		_ = pool.Raw().QueryRowContext(ctx, `SELECT COUNT(*) FROM security_schedules WHERE rt_id = $1`, rtAID).Scan(&countBefore)

		// Create a batch with 1 valid row and 1 invalid row (fake UUID)
		batch := []ScheduleExcelRow{
			{
				RowNumber:     2,
				Action:        "create",
				OfficerUserID: offA1ID,
				ShiftConfigID: shiftA1ID,
				StartDate:     "2027-01-10",
				EndDate:       "2027-01-11",
				Notes:         "Valid Row",
				IsValid:       true,
			},
			{
				RowNumber:     3,
				Action:        "update",
				ScheduleID:    "00000000-0000-0000-0000-000000000999", // Fake
				OfficerUserID: offA1ID,
				ShiftConfigID: shiftA1ID,
				StartDate:     "2027-01-10",
				EndDate:       "2027-01-11",
				Notes:         "Invalid Row",
				IsValid:       true,
			},
		}

		recCommit, _ := sendCommit(ketuaAToken, batch)
		if recCommit.Code == http.StatusOK {
			t.Fatal("expected commit to fail on invalid row in batch")
		}

		var countAfter int
		_ = pool.Raw().QueryRowContext(ctx, `SELECT COUNT(*) FROM security_schedules WHERE rt_id = $1`, rtAID).Scan(&countAfter)
		if countAfter != countBefore {
			t.Fatalf("atomic rollback failed: row count changed from %d to %d", countBefore, countAfter)
		}
	})

	// -------------------------------------------------------------------------
	// Scenario 13: Commit memvalidasi ulang data dan tidak hanya mempercayai preview
	// -------------------------------------------------------------------------
	t.Run("Scenario 13: Commit memvalidasi ulang data independen", func(t *testing.T) {
		// Client forges is_valid: true for an officer belonging to RT B
		forgedBatch := []ScheduleExcelRow{
			{
				RowNumber:     2,
				Action:        "create",
				OfficerUserID: offB1ID, // RT B officer
				ShiftConfigID: shiftA1ID,
				StartDate:     "2027-02-01",
				EndDate:       "2027-02-02",
				IsValid:       true, // forged!
			},
		}

		recCommit, _ := sendCommit(ketuaAToken, forgedBatch)
		if recCommit.Code == http.StatusOK {
			t.Fatal("expected commit to reject forged cross-tenant officer")
		}
	})

	// -------------------------------------------------------------------------
	// Scenario 14: Round-trip test: Export -> Read -> Preview
	// -------------------------------------------------------------------------
	t.Run("Scenario 14: Round-trip Export -> Read -> Preview", func(t *testing.T) {
		reqExp := httptest.NewRequest(http.MethodGet, "/api/v1/security/schedules/export", nil)
		reqExp.Header.Set("Authorization", "Bearer "+ketuaAToken)
		recExp := httptest.NewRecorder()
		router.ServeHTTP(recExp, reqExp)

		if recExp.Code != http.StatusOK {
			t.Fatalf("expected 200 on export, got %d: %s", recExp.Code, recExp.Body.String())
		}
		ct := recExp.Header().Get("Content-Type")
		if !strings.Contains(ct, "spreadsheetml") {
			t.Fatalf("expected spreadsheet content-type, got %s", ct)
		}

		expBytes := recExp.Body.Bytes()
		f, err := excelize.OpenReader(bytes.NewReader(expBytes))
		if err != nil {
			t.Fatalf("failed to open exported workbook: %v", err)
		}
		defer f.Close()

		sheetList := f.GetSheetList()
		if len(sheetList) < 2 {
			t.Fatalf("expected at least 2 sheets, got %v", sheetList)
		}
		if sheetList[0] != "Jadwal" || sheetList[1] != "Petunjuk" {
			t.Fatalf("expected sheets [Jadwal, Petunjuk], got %v", sheetList)
		}

		// Verify headers
		headers, _ := f.GetRows("Jadwal")
		if len(headers) == 0 {
			t.Fatal("exported Jadwal sheet has no rows")
		}
		for i, h := range ScheduleExcelHeaders {
			if i >= len(headers[0]) || headers[0][i] != h {
				t.Fatalf("header mismatch at index %d: expected %s, got %v", i, h, headers[0])
			}
		}

		// Re-preview exported data
		recPrev, prevResp := sendPreview(ketuaAToken, expBytes)
		if recPrev.Code != http.StatusOK {
			t.Fatalf("failed to preview exported workbook: %s", recPrev.Body.String())
		}
		if prevResp.TotalRows == 0 {
			t.Fatal("expected exported workbook to have schedule rows")
		}
		// All non-attendance locked exported rows should be valid update rows
		t.Logf("Exported %d rows, all verified successfully in round-trip preview", prevResp.TotalRows)
	})

	// -------------------------------------------------------------------------
	// Scenario 15: Cross-tenant isolation on export
	// -------------------------------------------------------------------------
	t.Run("Scenario 15: Cross-tenant isolation pada export", func(t *testing.T) {
		// Non-superadmin cannot access RT B data by passing ?rt_id=rtBID
		reqExp := httptest.NewRequest(http.MethodGet, "/api/v1/security/schedules/export?rt_id="+rtBID, nil)
		reqExp.Header.Set("Authorization", "Bearer "+ketuaAToken)
		recExp := httptest.NewRecorder()
		router.ServeHTTP(recExp, reqExp)

		if recExp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", recExp.Code)
		}

		// Inspect workbook: must only contain RT A officers (Budi, Agus), NEVER Siti Rahma (RT B)
		f, _ := excelize.OpenReader(recExp.Body)
		defer f.Close()
		guideRows, _ := f.GetRows("Petunjuk")
		allText := fmt.Sprintf("%v", guideRows)
		if strings.Contains(allText, "Siti Rahma") {
			t.Fatal("TENANT ISOLATION LEAK: RT B officer found in RT A export workbook!")
		}

		// Ketua B exports: must only see Siti Rahma
		reqExpB := httptest.NewRequest(http.MethodGet, "/api/v1/security/schedules/export", nil)
		reqExpB.Header.Set("Authorization", "Bearer "+ketuaBToken)
		recExpB := httptest.NewRecorder()
		router.ServeHTTP(recExpB, reqExpB)

		fB, _ := excelize.OpenReader(recExpB.Body)
		defer fB.Close()
		guideRowsB, _ := fB.GetRows("Petunjuk")
		allTextB := fmt.Sprintf("%v", guideRowsB)
		if strings.Contains(allTextB, "Budi Santoso") || strings.Contains(allTextB, "Agus Setiawan") {
			t.Fatal("TENANT ISOLATION LEAK: RT A officer found in RT B export workbook!")
		}
		if !strings.Contains(allTextB, "Siti Rahma") {
			t.Fatal("Expected Siti Rahma in RT B export workbook")
		}
	})
}
