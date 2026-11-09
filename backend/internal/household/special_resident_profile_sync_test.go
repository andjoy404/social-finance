package household

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	chi "github.com/go-chi/chi/v5"
	"social-finance/internal/auth"
)

func TestUpdateSpecialResidentSynchronizesAccountProfile(t *testing.T) {
	pool := testPool(t)
	db := pool.Raw()
	ctx := context.Background()
	rtID := setupTestRT(t, pool, "RT Profile Sync", 2, "profile-sync")
	runID := time.Now().UnixNano()
	token := makeTenantToken(
		t,
		fmt.Sprintf("profile-sync-manager-%d", runID),
		fmt.Sprintf("profile-sync-membership-%d", runID),
		rtID,
		auth.RolePengurus,
	)
	residentID := ""
	t.Cleanup(func() {
		if residentID == "" {
			return
		}
		var userID string
		_ = db.QueryRowContext(ctx,
			`SELECT user_id FROM residents WHERE id = $1`,
			residentID,
		).Scan(&userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM residents WHERE id = $1`, residentID)
		if userID != "" {
			_, _ = db.ExecContext(ctx, `DELETE FROM user_rt_memberships WHERE user_id = $1`, userID)
			_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
		}
	})

	router := chi.NewRouter()
	router.Use(auth.RequireAuth)
	handler := NewHandler(pool)
	router.Post("/api/v1/residents/special", handler.CreateSpecialResident)
	router.Patch("/api/v1/residents/special/{id}", handler.UpdateSpecialResident)

	createBody := fmt.Sprintf(
		`{"jabatan":"keamanan","full_name":"Before Update","nik":"317%013d","phone":"0812%08d","email":"security.%d@example.test"}`,
		runID%10000000000000,
		runID%100000000,
		runID,
	)
	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/residents/special", bytes.NewBufferString(createBody))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.Header.Set("Authorization", "Bearer "+token)
	createResponse := httptest.NewRecorder()
	router.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create special resident: status=%d body=%s", createResponse.Code, createResponse.Body.String())
	}

	var resident Resident
	if err := json.Unmarshal(createResponse.Body.Bytes(), &resident); err != nil {
		t.Fatalf("decode created resident: %v", err)
	}
	residentID = resident.ID

	updatedEmail := fmt.Sprintf("security.updated.%d@example.test", runID)
	updateBody := fmt.Sprintf(
		`{"full_name":"Updated Security Officer","phone":"081277788899","email":%q}`,
		updatedEmail,
	)
	updateRequest := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/residents/special/"+resident.ID,
		bytes.NewBufferString(updateBody),
	)
	updateRequest.Header.Set("Content-Type", "application/json")
	updateRequest.Header.Set("Authorization", "Bearer "+token)
	updateResponse := httptest.NewRecorder()
	router.ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update special resident: status=%d body=%s", updateResponse.Code, updateResponse.Body.String())
	}

	var updatedUser struct {
		FullName string `json:"full_name"`
		Phone    string `json:"phone"`
		Email    string `json:"email"`
	}
	if err := db.QueryRowContext(ctx,
		`SELECT u.full_name, u.phone, u.email
		 FROM users u JOIN residents r ON r.user_id = u.id WHERE r.id = $1`,
		resident.ID,
	).Scan(&updatedUser.FullName, &updatedUser.Phone, &updatedUser.Email); err != nil {
		t.Fatalf("read updated account profile: %v", err)
	}
	if updatedUser.FullName != "Updated Security Officer" ||
		updatedUser.Phone != "081277788899" ||
		updatedUser.Email != updatedEmail {
		t.Fatalf("account profile was not synchronized: %+v", updatedUser)
	}

	var updatedResident Resident
	if err := db.QueryRowContext(ctx,
		`SELECT id, rt_id, full_name, phone, nik, email, is_active, created_at, updated_at
		 FROM residents WHERE id = $1`,
		resident.ID,
	).Scan(
		&updatedResident.ID, &updatedResident.RTID, &updatedResident.FullName,
		&updatedResident.Phone, &updatedResident.Nik, &updatedResident.Email,
		&updatedResident.IsActive, &updatedResident.CreatedAt, &updatedResident.UpdatedAt,
	); err != nil {
		t.Fatalf("read updated resident profile: %v", err)
	}
	if updatedResident.FullName != updatedUser.FullName ||
		updatedResident.Phone == nil || *updatedResident.Phone != updatedUser.Phone ||
		updatedResident.Email == nil || *updatedResident.Email != updatedUser.Email {
		t.Fatalf("resident and account profile differ: resident=%+v user=%+v", updatedResident, updatedUser)
	}
}

func TestUpdateSpecialResidentEmailConflictRollsBackBothProfiles(t *testing.T) {
	pool := testPool(t)
	db := pool.Raw()
	ctx := context.Background()
	rtID := setupTestRT(t, pool, "RT Profile Conflict", 2, "profile-conflict")
	runID := time.Now().UnixNano()
	token := makeTenantToken(
		t,
		fmt.Sprintf("profile-conflict-manager-%d", runID),
		fmt.Sprintf("profile-conflict-membership-%d", runID),
		rtID,
		auth.RolePengurus,
	)
	originalEmail := fmt.Sprintf("profile.conflict.%d@example.test", runID)
	conflictingEmail := fmt.Sprintf("registered.%d@example.test", runID)
	var residentID, userID, conflictingUserID string
	t.Cleanup(func() {
		if residentID != "" {
			_, _ = db.ExecContext(ctx, `DELETE FROM residents WHERE id = $1`, residentID)
		}
		if userID != "" {
			_, _ = db.ExecContext(ctx, `DELETE FROM user_rt_memberships WHERE user_id = $1`, userID)
			_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
		}
		if conflictingUserID != "" {
			_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, conflictingUserID)
		}
	})

	router := chi.NewRouter()
	router.Use(auth.RequireAuth)
	handler := NewHandler(pool)
	router.Post("/api/v1/residents/special", handler.CreateSpecialResident)
	router.Patch("/api/v1/residents/special/{id}", handler.UpdateSpecialResident)

	createBody := fmt.Sprintf(
		`{"jabatan":"keamanan","full_name":"Original Officer","nik":"317%013d","phone":"0812%08d","email":%q}`,
		runID%10000000000000,
		runID%100000000,
		originalEmail,
	)
	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/residents/special", bytes.NewBufferString(createBody))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.Header.Set("Authorization", "Bearer "+token)
	createResponse := httptest.NewRecorder()
	router.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create special resident: status=%d body=%s", createResponse.Code, createResponse.Body.String())
	}

	var resident Resident
	if err := json.Unmarshal(createResponse.Body.Bytes(), &resident); err != nil {
		t.Fatalf("decode created resident: %v", err)
	}
	residentID = resident.ID
	if err := db.QueryRowContext(ctx,
		`SELECT user_id FROM residents WHERE id = $1`, residentID,
	).Scan(&userID); err != nil {
		t.Fatalf("read linked user id: %v", err)
	}
	if err := db.QueryRowContext(ctx,
		`INSERT INTO users (email, phone, password_hash, full_name, is_active)
		 VALUES ($1, '081200000001', 'dummyhash', 'Existing User', true)
		 RETURNING id`,
		conflictingEmail,
	).Scan(&conflictingUserID); err != nil {
		t.Fatalf("create conflicting user: %v", err)
	}
	var originalResidentName, originalResidentPhone, originalResidentEmail string
	if err := db.QueryRowContext(ctx,
		`SELECT full_name, phone, email FROM residents WHERE id = $1`, residentID,
	).Scan(&originalResidentName, &originalResidentPhone, &originalResidentEmail); err != nil {
		t.Fatalf("read resident before rejected update: %v", err)
	}
	var originalUserName, originalUserPhone, originalUserEmail string
	if err := db.QueryRowContext(ctx,
		`SELECT full_name, phone, email FROM users WHERE id = $1`, userID,
	).Scan(&originalUserName, &originalUserPhone, &originalUserEmail); err != nil {
		t.Fatalf("read account before rejected update: %v", err)
	}

	updateBody := fmt.Sprintf(
		`{"full_name":"Must Roll Back","phone":"081277788899","email":%q}`,
		conflictingEmail,
	)
	updateRequest := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/residents/special/"+residentID,
		bytes.NewBufferString(updateBody),
	)
	updateRequest.Header.Set("Content-Type", "application/json")
	updateRequest.Header.Set("Authorization", "Bearer "+token)
	updateResponse := httptest.NewRecorder()
	router.ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusConflict {
		t.Fatalf("expected 409 for conflicting account email, got %d: %s", updateResponse.Code, updateResponse.Body.String())
	}

	var residentName, residentPhone, residentEmail string
	if err := db.QueryRowContext(ctx,
		`SELECT full_name, phone, email FROM residents WHERE id = $1`, residentID,
	).Scan(&residentName, &residentPhone, &residentEmail); err != nil {
		t.Fatalf("read resident after rejected update: %v", err)
	}
	var userName, userPhone, userEmail string
	if err := db.QueryRowContext(ctx,
		`SELECT full_name, phone, email FROM users WHERE id = $1`, userID,
	).Scan(&userName, &userPhone, &userEmail); err != nil {
		t.Fatalf("read account after rejected update: %v", err)
	}
	if residentName != originalResidentName || residentPhone != originalResidentPhone ||
		residentEmail != originalResidentEmail || userName != originalUserName ||
		userPhone != originalUserPhone || userEmail != originalUserEmail {
		t.Fatalf("email conflict partially changed profiles: resident=%q/%q/%q user=%q/%q/%q",
			residentName, residentPhone, residentEmail, userName, userPhone, userEmail)
	}
}
