package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"social-finance/internal/auth"
	"social-finance/internal/security"
	"social-finance/internal/storage"
	"social-finance/internal/testutil"
)

func TestPaket_WargaRecipientDeepLinkAndPhotosRBAC(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	auth.SigningSecret = []byte("test-secret-key-32-chars-long-ok!!")
	auth.DefaultAccessTokenLifetime = 15 * time.Minute
	auth.AuthDBPool = pool

	rtA := uuid.New().String()
	rtB := uuid.New().String()
	baseRW := int(time.Now().UnixNano()%80000) + 1000
	for idx, rt := range []struct {
		id   string
		name string
	}{
		{rtA, "RT 001 Test A"},
		{rtB, "RT 002 Test B"},
	} {
		_, err := pool.Raw().ExecContext(ctx, `
			INSERT INTO rts (id, rw, rt, name, is_active)
			VALUES ($1, $2, $3, $4, true)
			ON CONFLICT (id) DO NOTHING
		`, rt.id, baseRW+idx, fmt.Sprintf("%03d-%s", idx+1, uuid.NewString()[:6]), rt.name)
		if err != nil {
			t.Fatalf("setup rt: %v", err)
		}
	}

	createUserWithMembership := func(uid, emailPrefix, role, jabatan, rtID string) string {
		email := fmt.Sprintf("%s-%s@test.com", uuid.New().String()[:8], emailPrefix)
		_, err := pool.Raw().ExecContext(ctx, `
			INSERT INTO users (id, email, password_hash, full_name, is_active)
			VALUES ($1, $2, 'hash', $3, true)
		`, uid, email, fmt.Sprintf("User %s", email))
		if err != nil {
			t.Fatalf("insert user: %v", err)
		}
		var jabVal any
		if jabatan != "" {
			jabVal = jabatan
		}
		mid := uuid.New().String()
		_, err = pool.Raw().ExecContext(ctx, `
			INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
			VALUES ($1, $2, $3, $4, $5, true)
		`, mid, uid, rtID, role, jabVal)
		if err != nil {
			t.Fatalf("insert membership: %v", err)
		}
		tok, err := auth.GenerateAccessToken(auth.TokenClaims{
			UserID:  uid,
			MID:     mid,
			RTID:    rtID,
			Role:    role,
			Jabatan: jabatan,
		})
		if err != nil {
			t.Fatalf("issue token: %v", err)
		}
		return tok
	}

	createResident := func(resID, rtID, userID, fullName string) {
		_, err := pool.Raw().ExecContext(ctx, `
			INSERT INTO residents (id, rt_id, user_id, full_name, is_active)
			VALUES ($1, $2, $3, $4, true)
		`, resID, rtID, userID, fullName)
		if err != nil {
			t.Fatalf("insert resident: %v", err)
		}
	}

	// 1. Setup users
	// RT A:
	// - Officer (keamanan)
	// - Recipient Warga
	// - Non-recipient Warga
	officerID := uuid.New().String()
	officerToken := createUserWithMembership(officerID, "officer@rta.com", "pengurus", "keamanan", rtA)

	wargaRecvID := uuid.New().String()
	wargaRecvToken := createUserWithMembership(wargaRecvID, "warga_recv@rta.com", "warga", "", rtA)
	residentRecvID := uuid.New().String()
	createResident(residentRecvID, rtA, wargaRecvID, "Warga Penerima A")

	wargaOtherID := uuid.New().String()
	wargaOtherToken := createUserWithMembership(wargaOtherID, "warga_other@rta.com", "warga", "", rtA)
	residentOtherID := uuid.New().String()
	createResident(residentOtherID, rtA, wargaOtherID, "Warga Lain A")

	// RT B:
	// - Cross-RT Warga
	// - Cross-RT Security Officer
	wargaCrossID := uuid.New().String()
	wargaCrossToken := createUserWithMembership(wargaCrossID, "warga_cross@rtb.com", "warga", "", rtB)
	residentCrossID := uuid.New().String()
	createResident(residentCrossID, rtB, wargaCrossID, "Warga RT B")

	officerBID := uuid.New().String()
	officerBToken := createUserWithMembership(officerBID, "officer_b@rtb.com", "pengurus", "keamanan", rtB)
	for _, perm := range []string{"paket.read", "paket.create"} {
		_, _ = pool.Raw().ExecContext(ctx, `
			INSERT INTO position_permissions (rt_id, position, permission)
			VALUES ($1, 'keamanan', $2)
			ON CONFLICT DO NOTHING
		`, rtB, perm)
	}

	// Setup duplicate contact resident & spoofed user (same phone & email, different user_id)
	sharedEmail := fmt.Sprintf("recv_%s@example.com", uuid.New().String()[:8])
	sharedPhone := fmt.Sprintf("0812%08d", time.Now().UnixNano()%100000000)
	_, _ = pool.Raw().ExecContext(ctx, `
		UPDATE residents SET email = $1, phone = $2 WHERE id = $3
	`, sharedEmail, sharedPhone, residentRecvID)
	_, _ = pool.Raw().ExecContext(ctx, `
		UPDATE users SET email = $1, phone = $2 WHERE id = $3
	`, sharedEmail, sharedPhone, wargaRecvID)

	wargaSpoofedID := uuid.New().String()
	wargaSpoofedToken := createUserWithMembership(wargaSpoofedID, "warga_spoof@rta.com", "warga", "", rtA)
	_, _ = pool.Raw().ExecContext(ctx, `
		UPDATE users SET phone = $1 WHERE id = $2
	`, sharedPhone, wargaSpoofedID)

	// 2. Grant position permissions for petugas keamanan in RT A
	for _, perm := range []string{"paket.read", "paket.create", "paket.status_change"} {
		_, _ = pool.Raw().ExecContext(ctx, `
			INSERT INTO position_permissions (rt_id, position, permission)
			VALUES ($1, 'keamanan', $2)
			ON CONFLICT DO NOTHING
		`, rtA, perm)
	}

	// 3. Create mock storage and seed an image
	mockStore := storage.NewMockStorage()
	jpgBytes := tinyJPEG()
	meta, err := mockStore.UploadCommunityImage(ctx, rtA, "paket", "sample_box.jpg", bytes.NewReader(jpgBytes), int64(len(jpgBytes)))
	if err != nil {
		t.Fatalf("upload mock image: %v", err)
	}
	imgKey := meta.Key

	// 4. Create a Paket Report directly using repository/service
	secRepo := security.NewRepository(pool)
	secSvc := security.NewService(secRepo)
	desc := "Paket Tokopedia Sepatu"
	report, err := secSvc.CreatePaketReport(ctx, rtA, officerID, residentRecvID, []security.CreatePaketItem{
		{
			Description: &desc,
			ImageKeys:   []string{imgKey},
		},
	})
	if err != nil {
		t.Fatalf("failed to create paket report: %v", err)
	}
	if len(report.Items) == 0 || len(report.Items[0].Photos) == 0 {
		t.Fatalf("expected created report to have items and photos")
	}
	reportID := report.ID
	photoID := report.Items[0].Photos[0].ID
	itemID := report.Items[0].ID

	// 5. Build full production router
	authSvc := auth.NewService(auth.ServiceOptions{
		RefreshLifetime: 7 * 24 * time.Hour,
	})
	authHandler := auth.NewHandler(pool, authSvc)
	cfg := auth.ScopingConfig{
		RTIDs: []string{rtA, rtB},
		RWIDs: []string{},
	}
	router := buildRouter(pool, authHandler, &cfg, mockStore)

	// Helper for HTTP requests
	doReq := func(method, path, token string, body []byte) *httptest.ResponseRecorder {
		var req *http.Request
		if body != nil {
			req = httptest.NewRequest(method, path, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req = httptest.NewRequest(method, path, nil)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		return rr
	}

	// ── TEST CASES ─────────────────────────────────────────────────────────────

	// 1. Recipient Warga opens Detail Paket (deep link) -> 200 OK
	t.Run("WargaRecipient_CanGetPaketDetail", func(t *testing.T) {
		rr := doReq("GET", fmt.Sprintf("/api/v1/security/paket/reports/%s", reportID), wargaRecvToken, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for recipient warga, got %d: %s", rr.Code, rr.Body.String())
		}

		var res struct {
			ID           string `json:"id"`
			ReceiverName string `json:"receiver_name"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if res.ID != reportID {
			t.Errorf("expected report ID %s, got %s", reportID, res.ID)
		}
		if res.ReceiverName != "Warga Penerima A" {
			t.Errorf("expected receiver name 'Warga Penerima A', got %s", res.ReceiverName)
		}
	})

	// 1b. Recipient Warga opens Detail Paket using Item ID (from pickup notification dedup_key) -> 200 OK
	t.Run("WargaRecipient_CanGetPaketDetail_UsingItemID", func(t *testing.T) {
		rr := doReq("GET", fmt.Sprintf("/api/v1/security/paket/reports/%s", itemID), wargaRecvToken, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for recipient warga using itemID, got %d: %s", rr.Code, rr.Body.String())
		}

		var res struct {
			ID           string `json:"id"`
			ReceiverName string `json:"receiver_name"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if res.ID != reportID {
			t.Errorf("expected resolved report ID %s, got %s", reportID, res.ID)
		}
		if res.ReceiverName != "Warga Penerima A" {
			t.Errorf("expected receiver name 'Warga Penerima A', got %s", res.ReceiverName)
		}
	})

	// 2. Recipient Warga opens attached photo -> 200 OK
	t.Run("WargaRecipient_CanGetPaketPhoto", func(t *testing.T) {
		rr := doReq("GET", fmt.Sprintf("/api/v1/security/paket/photos/%s", photoID), wargaRecvToken, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for photo fetch, got %d: %s", rr.Code, rr.Body.String())
		}
		if len(rr.Body.Bytes()) != len(jpgBytes) {
			t.Errorf("expected image content length %d, got %d", len(jpgBytes), len(rr.Body.Bytes()))
		}
	})

	// 3. Non-recipient Warga in same RT calling Detail Paket -> 403 Forbidden
	t.Run("WargaOtherSameRT_ForbiddenFromDetail", func(t *testing.T) {
		rr := doReq("GET", fmt.Sprintf("/api/v1/security/paket/reports/%s", reportID), wargaOtherToken, nil)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for non-recipient warga, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	// 3b. Non-recipient Warga in same RT calling Detail Paket using Item ID -> 403 Forbidden
	t.Run("WargaOtherSameRT_ForbiddenFromDetail_UsingItemID", func(t *testing.T) {
		rr := doReq("GET", fmt.Sprintf("/api/v1/security/paket/reports/%s", itemID), wargaOtherToken, nil)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for non-recipient warga using itemID, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	// 4. Non-recipient Warga in same RT calling photo -> 403 Forbidden
	t.Run("WargaOtherSameRT_ForbiddenFromPhoto", func(t *testing.T) {
		rr := doReq("GET", fmt.Sprintf("/api/v1/security/paket/photos/%s", photoID), wargaOtherToken, nil)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for non-recipient photo access, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	// 5. Recipient Warga cannot list all packages (no menu/tab access) -> 403 Forbidden
	t.Run("WargaRecipient_ForbiddenFromListingPackages", func(t *testing.T) {
		rr := doReq("GET", "/api/v1/security/paket/reports", wargaRecvToken, nil)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden when warga tries to list packages, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	// 6. Recipient Warga cannot create package reports -> 403 Forbidden
	t.Run("WargaRecipient_ForbiddenFromCreatingPackage", func(t *testing.T) {
		body := `{"receiver_warga_id":"` + residentRecvID + `","items":[{"description":"Illegal"}]}`
		rr := doReq("POST", "/api/v1/security/paket/reports", wargaRecvToken, []byte(body))
		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden when warga tries to create package, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	// 7. Recipient Warga cannot change package status -> 403 Forbidden
	t.Run("WargaRecipient_ForbiddenFromChangingStatus", func(t *testing.T) {
		body := `{"status":"sudah_diambil"}`
		rr := doReq("PATCH", fmt.Sprintf("/api/v1/security/paket/reports/%s/items/%s/status", reportID, itemID), wargaRecvToken, []byte(body))
		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden when warga tries to mutate status, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	// 8. Recipient Warga cannot upload package photos -> 403 Forbidden
	t.Run("WargaRecipient_ForbiddenFromUploadingPhoto", func(t *testing.T) {
		rr := doReq("POST", "/api/v1/security/paket/photos", wargaRecvToken, []byte("fake-photo"))
		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden when warga tries to upload photo, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	// 9. Warga with matching phone number but different user account -> 403 Forbidden
	t.Run("WargaDuplicateContact_DeniedAccess", func(t *testing.T) {
		// Detail access must fail
		rrDetail := doReq("GET", fmt.Sprintf("/api/v1/security/paket/reports/%s", reportID), wargaSpoofedToken, nil)
		if rrDetail.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for warga with duplicate phone, got %d: %s", rrDetail.Code, rrDetail.Body.String())
		}
		// Photo access must fail
		rrPhoto := doReq("GET", fmt.Sprintf("/api/v1/security/paket/photos/%s", photoID), wargaSpoofedToken, nil)
		if rrPhoto.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for photo access by duplicate contact warga, got %d: %s", rrPhoto.Code, rrPhoto.Body.String())
		}
	})

	// 10. Cross-RT Warga calling Detail Paket -> 403 / 404
	t.Run("WargaCrossRT_DeniedAccess", func(t *testing.T) {
		rr := doReq("GET", fmt.Sprintf("/api/v1/security/paket/reports/%s", reportID), wargaCrossToken, nil)
		if rr.Code != http.StatusForbidden && rr.Code != http.StatusNotFound {
			t.Fatalf("expected 403 or 404 for cross-RT warga access, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	// 11. Security Officer from another RT calling Detail or Photo of RT A -> 403 / 404
	t.Run("SecurityOfficerCrossRT_DeniedAccess", func(t *testing.T) {
		rrDetail := doReq("GET", fmt.Sprintf("/api/v1/security/paket/reports/%s", reportID), officerBToken, nil)
		if rrDetail.Code != http.StatusForbidden && rrDetail.Code != http.StatusNotFound {
			t.Fatalf("expected 403 or 404 for cross-RT officer detail access, got %d: %s", rrDetail.Code, rrDetail.Body.String())
		}
		rrPhoto := doReq("GET", fmt.Sprintf("/api/v1/security/paket/photos/%s", photoID), officerBToken, nil)
		if rrPhoto.Code != http.StatusForbidden && rrPhoto.Code != http.StatusNotFound {
			t.Fatalf("expected 403 or 404 for cross-RT officer photo access, got %d: %s", rrPhoto.Code, rrPhoto.Body.String())
		}
	})

	// 12. Unauthenticated caller calling Detail Paket -> 401 Unauthorized
	t.Run("Unauthenticated_DeniedAccess", func(t *testing.T) {
		rr := doReq("GET", fmt.Sprintf("/api/v1/security/paket/reports/%s", reportID), "", nil)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	// 13. Security Officer in same RT calling Detail, List, Photo -> 200 OK
	t.Run("SecurityOfficer_FullAccessAllowed", func(t *testing.T) {
		// List
		rrList := doReq("GET", "/api/v1/security/paket/reports", officerToken, nil)
		if rrList.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for officer list, got %d: %s", rrList.Code, rrList.Body.String())
		}
		// Detail
		rrDetail := doReq("GET", fmt.Sprintf("/api/v1/security/paket/reports/%s", reportID), officerToken, nil)
		if rrDetail.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for officer detail, got %d: %s", rrDetail.Code, rrDetail.Body.String())
		}
		// Photo
		rrPhoto := doReq("GET", fmt.Sprintf("/api/v1/security/paket/photos/%s", photoID), officerToken, nil)
		if rrPhoto.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for officer photo, got %d: %s", rrPhoto.Code, rrPhoto.Body.String())
		}
	})
}
