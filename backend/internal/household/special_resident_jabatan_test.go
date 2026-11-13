package household

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	chi "github.com/go-chi/chi/v5"
	"social-finance/internal/auth"
)

func TestSpecialResidentJabatanValidation(t *testing.T) {
	// 1. Verify validSpecialJabatans allows only keamanan, operasional, kebersihan
	expectedSpecial := []string{"keamanan", "operasional", "kebersihan"}
	for _, j := range expectedSpecial {
		if !validSpecialJabatans[j] {
			t.Errorf("expected validSpecialJabatans[%q] to be true", j)
		}
	}

	rejectedSpecial := []string{"kebersihan_pembangunan", "ketua", "bendahara", "anggota", "unknown", ""}
	for _, j := range rejectedSpecial {
		if validSpecialJabatans[j] {
			t.Errorf("expected validSpecialJabatans[%q] to be false", j)
		}
	}

	// 2. Verify auth.ValidJabatans includes operasional & kebersihan and excludes kebersihan_pembangunan
	if !auth.ValidJabatans[auth.JabatanKeamanan] {
		t.Errorf("expected auth.ValidJabatans to include %q", auth.JabatanKeamanan)
	}
	if !auth.ValidJabatans[auth.JabatanOperasional] {
		t.Errorf("expected auth.ValidJabatans to include %q", auth.JabatanOperasional)
	}
	if !auth.ValidJabatans[auth.JabatanKebersihan] {
		t.Errorf("expected auth.ValidJabatans to include %q", auth.JabatanKebersihan)
	}
	if auth.ValidJabatans[auth.Jabatan("kebersihan_pembangunan")] {
		t.Errorf("expected auth.ValidJabatans NOT to include obsolete 'kebersihan_pembangunan'")
	}
}

func TestMigration056AndSpecialResidentPermissions(t *testing.T) {
	pool := testPool(t)
	db := pool.Raw()
	ctx := context.Background()

	upPath := filepath.Join("..", "..", "migrations", "056_special_resident_operasional_and_kebersihan.up.sql")
	downPath := filepath.Join("..", "..", "migrations", "056_special_resident_operasional_and_kebersihan.down.sql")

	upSQLBytes, err := os.ReadFile(upPath)
	if err != nil {
		t.Fatalf("read migration 056 up: %v", err)
	}
	downSQLBytes, err := os.ReadFile(downPath)
	if err != nil {
		t.Fatalf("read migration 056 down: %v", err)
	}
	upSQL := string(upSQLBytes)
	downSQL := string(downSQLBytes)

	rtID := setupTestRT(t, pool, "RT Mig 056", 1, "056")
	keamananUserID := toUUID("test-user-keamanan-mig056")
	operasionalUserID := toUUID("test-user-operasional-mig056")
	kebersihanUserID := toUUID("test-user-kebersihan-mig056")

	keamananMemID := toUUID("test-mem-keamanan-mig056")
	operasionalMemID := toUUID("test-mem-operasional-mig056")
	kebersihanMemID := toUUID("test-mem-kebersihan-mig056")

	cleanup := func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM user_rt_memberships WHERE id IN ($1, $2, $3)`, keamananMemID, operasionalMemID, kebersihanMemID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id IN ($1, $2, $3)`, keamananUserID, operasionalUserID, kebersihanUserID)
	}
	cleanup()
	t.Cleanup(func() {
		cleanup()
		// Always leave test DB with migration 056 UP applied
		_, _ = db.ExecContext(ctx, upSQL)
	})

	for _, u := range []struct {
		id    string
		email string
		name  string
	}{
		{keamananUserID, "keamanan.mig056@test.com", "Petugas Keamanan 056"},
		{operasionalUserID, "operasional.mig056@test.com", "Petugas Operasional 056"},
		{kebersihanUserID, "kebersihan.mig056@test.com", "Petugas Kebersihan 056"},
	} {
		_, err = db.ExecContext(ctx, `
			INSERT INTO users (id, email, phone, password_hash, full_name, is_active)
			VALUES ($1, $2, '08123456056', 'dummyhash', $3, true)
			ON CONFLICT (id) DO NOTHING
		`, u.id, u.email, u.name)
		if err != nil {
			t.Fatalf("insert test user %s: %v", u.email, err)
		}
	}

	// Ensure migration 056 is first applied, then rolled back so we can test pre-056 data migration
	if _, err := db.ExecContext(ctx, upSQL); err != nil {
		t.Fatalf("initial apply migration 056 up: %v", err)
	}
	// Remove any existing 'kebersihan' memberships in test DB before running down
	_, _ = db.ExecContext(ctx, `DELETE FROM user_rt_memberships WHERE jabatan = 'kebersihan'`)
	if _, err := db.ExecContext(ctx, downSQL); err != nil {
		t.Fatalf("rollback migration 056 to pre-056 state: %v", err)
	}

	// Insert pre-056 memberships: keamanan and legacy kebersihan_pembangunan
	_, err = db.ExecContext(ctx, `
		INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		VALUES
			($1, $2, $3, 'warga', 'keamanan', true),
			($4, $5, $3, 'warga', 'kebersihan_pembangunan', true)
	`, keamananMemID, keamananUserID, rtID, operasionalMemID, operasionalUserID)
	if err != nil {
		t.Fatalf("insert pre-056 memberships: %v", err)
	}

	// Apply Migration 056 UP
	if _, err := db.ExecContext(ctx, upSQL); err != nil {
		t.Fatalf("apply migration 056 up failed: %v", err)
	}

	// 1. Verify legacy 'kebersihan_pembangunan' membership was migrated to 'operasional'
	var migratedJabatan string
	if err := db.QueryRowContext(ctx, `SELECT jabatan FROM user_rt_memberships WHERE id = $1`, operasionalMemID).Scan(&migratedJabatan); err != nil {
		t.Fatalf("query migrated membership: %v", err)
	}
	if migratedJabatan != "operasional" {
		t.Fatalf("expected legacy 'kebersihan_pembangunan' membership to migrate to 'operasional', got %q", migratedJabatan)
	}

	var legacyCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_rt_memberships WHERE jabatan = 'kebersihan_pembangunan'`).Scan(&legacyCount); err != nil {
		t.Fatalf("count legacy memberships: %v", err)
	}
	if legacyCount != 0 {
		t.Fatalf("expected 0 rows with jabatan = 'kebersihan_pembangunan', got %d", legacyCount)
	}

	// 2. Insert a new 'kebersihan' membership and verify constraint accepts it
	_, err = db.ExecContext(ctx, `
		INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		VALUES ($1, $2, $3, 'warga', 'kebersihan', true)
	`, kebersihanMemID, kebersihanUserID, rtID)
	if err != nil {
		t.Fatalf("insert 'kebersihan' membership failed: %v", err)
	}

	// Verify constraint rejects obsolete 'kebersihan_pembangunan'
	_, err = db.ExecContext(ctx, `
		UPDATE user_rt_memberships SET jabatan = 'kebersihan_pembangunan' WHERE id = $1
	`, operasionalMemID)
	if err == nil {
		t.Fatalf("expected CHECK constraint to reject 'kebersihan_pembangunan' after migration 056 UP")
	}

	// 3. Verify RBAC permissions for keamanan, operasional, and kebersihan independently
	resolver := auth.NewPermissionResolver(pool)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx for permission check: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, tc := range []struct {
		jabatan auth.Jabatan
		userID  string
		memID   string
	}{
		{auth.JabatanKeamanan, keamananUserID, keamananMemID},
		{auth.JabatanOperasional, operasionalUserID, operasionalMemID},
		{auth.JabatanKebersihan, kebersihanUserID, kebersihanMemID},
	} {
		ac := &auth.AuthContext{
			UserID:       tc.userID,
			MembershipID: tc.memID,
			RTID:         rtID,
			Jabatan:      tc.jabatan,
		}
		// Each position must have warga.read and reports.read
		for _, allowedPerm := range []string{"warga.read", "reports.read"} {
			has, err := resolver.HasPermission(ctx, tx, ac, allowedPerm)
			if err != nil {
				t.Fatalf("HasPermission(%s, %s): %v", tc.jabatan, allowedPerm, err)
			}
			if !has {
				t.Errorf("expected position %q to have permission %q", tc.jabatan, allowedPerm)
			}
		}

		// None of them may have finance, create/update, settings, or notification.manage permissions
		for _, deniedPerm := range []string{
			"finance.read",
			"finance.create",
			"finance.approve",
			"warga.create",
			"warga.update",
			"rt.settings.update",
			"notification.manage",
		} {
			has, err := resolver.HasPermission(ctx, tx, ac, deniedPerm)
			if err != nil {
				t.Fatalf("HasPermission(%s, %s): %v", tc.jabatan, deniedPerm, err)
			}
			if has {
				t.Errorf("expected position %q NOT to have permission %q", tc.jabatan, deniedPerm)
			}
		}
	}
	_ = tx.Rollback()

	// 4. Verify DOWN migration blocks rollback while a 'kebersihan' membership exists
	_, err = db.ExecContext(ctx, downSQL)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "cannot rollback migration 056") {
		t.Fatalf("expected migration 056 down to fail with conflict message while 'kebersihan' membership exists, got: %v", err)
	}

	// 5. After deleting the 'kebersihan' membership, DOWN migration succeeds and restores 'operasional' -> 'kebersihan_pembangunan'
	if _, err := db.ExecContext(ctx, `DELETE FROM user_rt_memberships WHERE id = $1`, kebersihanMemID); err != nil {
		t.Fatalf("delete kebersihan membership: %v", err)
	}
	if _, err := db.ExecContext(ctx, downSQL); err != nil {
		t.Fatalf("expected migration 056 down to succeed after removing 'kebersihan' membership, got: %v", err)
	}

	var restoredJabatan string
	if err := db.QueryRowContext(ctx, `SELECT jabatan FROM user_rt_memberships WHERE id = $1`, operasionalMemID).Scan(&restoredJabatan); err != nil {
		t.Fatalf("query restored membership: %v", err)
	}
	if restoredJabatan != "kebersihan_pembangunan" {
		t.Fatalf("expected DOWN migration to restore 'operasional' back to 'kebersihan_pembangunan', got %q", restoredJabatan)
	}

	// Re-apply UP so test DB stays at migration 056
	if _, err := db.ExecContext(ctx, upSQL); err != nil {
		t.Fatalf("re-apply migration 056 up: %v", err)
	}

	// 6. Verify SpecialResidentCreate and UpdateSpecialResident HTTP handlers
	h := NewHandler(pool)
	r := chi.NewRouter()
	r.Use(auth.RequireAuth)
	r.Post("/api/v1/residents/special", h.CreateSpecialResident)
	r.Patch("/api/v1/residents/special/{id}", h.UpdateSpecialResident)

	pengurusToken := makeTenantToken(t, "test-pengurus-mig056", "test-pengurus-mem-mig056", rtID, auth.RolePengurus)

	var createdID string
	runID := time.Now().UnixNano()
	for idx, jab := range []string{"keamanan", "operasional", "kebersihan"} {
		body := fmt.Sprintf(`{"full_name":"Petugas %s","nik":"317300000000005%d","phone":"0812%04d00%d","email":"petugas.%s.056.%d@test.com","jabatan":"%s"}`, jab, idx, runID%10000, idx, jab, runID, jab)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/residents/special", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+pengurusToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("CreateSpecialResident(%q) expected 201, got %d: %s", jab, rec.Code, rec.Body.String())
		}
		var created Resident
		if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
			t.Fatalf("unmarshal created special resident: %v", err)
		}
		createdID = created.ID
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `DELETE FROM residents WHERE id = $1`, created.ID)
			if created.UserID != nil {
				_, _ = db.ExecContext(ctx, `DELETE FROM user_rt_memberships WHERE user_id = $1`, *created.UserID)
				_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, *created.UserID)
			}
		})
	}

	// Reject obsolete 'kebersihan_pembangunan' and unknown jabatans on CreateSpecialResident
	for _, invalidJab := range []string{"kebersihan_pembangunan", "ketua", "unknown"} {
		body := fmt.Sprintf(`{"full_name":"Invalid Petugas","nik":"3173000000000099","phone":"08129999009","email":"invalid.%s@test.com","jabatan":"%s"}`, invalidJab, invalidJab)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/residents/special", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+pengurusToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("CreateSpecialResident(%q) expected 400, got %d: %s", invalidJab, rec.Code, rec.Body.String())
		}
	}

	// UpdateSpecialResident accepts switching between operasional, kebersihan, and keamanan, and rejects kebersihan_pembangunan
	for _, validJab := range []string{"operasional", "kebersihan", "keamanan"} {
		body := fmt.Sprintf(`{"jabatan":"%s"}`, validJab)
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/residents/special/"+createdID, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+pengurusToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("UpdateSpecialResident(%q) expected 200, got %d: %s", validJab, rec.Code, rec.Body.String())
		}
	}

	rejectUpdateReq := httptest.NewRequest(http.MethodPatch, "/api/v1/residents/special/"+createdID, bytes.NewBufferString(`{"jabatan":"kebersihan_pembangunan"}`))
	rejectUpdateReq.Header.Set("Content-Type", "application/json")
	rejectUpdateReq.Header.Set("Authorization", "Bearer "+pengurusToken)
	rejectUpdateRec := httptest.NewRecorder()
	r.ServeHTTP(rejectUpdateRec, rejectUpdateReq)
	if rejectUpdateRec.Code != http.StatusBadRequest {
		t.Fatalf("UpdateSpecialResident(kebersihan_pembangunan) expected 400, got %d: %s", rejectUpdateRec.Code, rejectUpdateRec.Body.String())
	}
}
