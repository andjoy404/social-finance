package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"social-finance/internal/auth"
	"social-finance/internal/database"
)

func createRekeningSeed(t *testing.T, pool *database.Pool, rtID, name, category string, isActive bool, bank, number, holder string) string {
	t.Helper()
	var id string
	if err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO fund_locations (rt_id, name, category, bank_name, account_number, account_holder, is_active)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		rtID, name, category, nullString(bank), nullString(number), nullString(holder), isActive,
	).Scan(&id); err != nil {
		t.Fatalf("seed fund location: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Raw().ExecContext(ctx,
			`DELETE FROM location_balance_changes WHERE location_id = $1`, id)
		_, _ = pool.Raw().ExecContext(ctx,
			`DELETE FROM fund_transfers WHERE from_location_id = $1 OR to_location_id = $1`, id)
		_, _ = pool.Raw().ExecContext(ctx,
			`DELETE FROM fund_locations WHERE id = $1`, id)
	})
	return id
}

func nullString(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

// TestPublicRekening_WargaReadBalanceFreeActiveRekening verifies:
// 1. Plain warga (no jabatan) can call the endpoint (not 403).
// 2. Response shape excludes current_balance, rt_id — only transfer info.
// 3. Only active rekening are listed; brankas/coh and inactive rows are hidden.
// 4. Tenant caller sees only its own RT's rekening (no cross-RT leak).
func TestPublicRekening_WargaReadBalanceFreeActiveRekeningOnly(t *testing.T) {
	pool, router := setupFLRouter(t, []string{flRTUrena})

	reqIDRT := flRTUrena
	seedID := createRekeningSeed(t, pool, reqIDRT, "BCA Uji Warga", "rekening", true, "BCA", "1112223334", "RT 002")
	_ = seedID
	createRekeningSeed(t, pool, reqIDRT, "BRI Uji Warga", "rekening", true, "BRI", "5554443322", "RT 002 Treasurer")
	createRekeningSeed(t, pool, reqIDRT, "BRI Nonaktif", "rekening", false, "BRI", "9998887776", "RT 002 Discontinued")
	createRekeningSeed(t, pool, reqIDRT, "Brankas Warga", "brankas", true, "", "", "")
	createRekeningSeed(t, pool, reqIDRT, "COH Warga", "coh", true, "", "", "")

	_, warToken := createRTUser(t, pool, reqIDRT, auth.RoleWarga, "")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/fund-locations/rekening/public", nil)
	req.Header.Set("Authorization", "Bearer "+warToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("warga list public rekening: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v :: %s", err, rec.Body.String())
	}
	names := make([]string, 0, len(body.Data))
	for _, row := range body.Data {
		names = append(names, fmt.Sprint(row["name"]))
		for _, forbidden := range []string{"current_balance", "rt_id", "created_at", "updated_at", "is_active", "category"} {
			if _, ok := row[forbidden]; ok {
				t.Errorf("public rekening payload must not include %q; got %v", forbidden, row)
			}
		}
		// Rows seeded by this test must carry the full transfer info (bank, number, holder).
		if strings.HasPrefix(fmt.Sprint(row["name"]), "BCA Uji Warga") || strings.HasPrefix(fmt.Sprint(row["name"]), "BRI Uji Warga") {
			if row["bank_name"] == nil || row["account_number"] == nil || row["account_holder"] == nil {
				t.Errorf("missing transfer fields in %q: %v", row["name"], row)
			}
		}
	}
	for _, want := range []string{"BCA Uji Warga", "BRI Uji Warga"} {
		found := false
		for _, n := range names {
			if n == want {
				found = true
			}
		}
		if !found {
			t.Errorf("expected active rekening %q in response, got %v", want, names)
		}
	}
	for _, notWant := range []string{"BRI Nonaktif", "Brankas Warga", "COH Warga"} {
		for _, n := range names {
			if n == notWant {
				t.Errorf("non-public entry %q must not appear in response (%v)", notWant, names)
			}
		}
	}
}

// TestPublicRekening_CrossRTIsolated verifies a caller can only see its own RT's rekening.
func TestPublicRekening_CrossRTIsolated(t *testing.T) {
	pool, router := setupFLRouter(t, []string{flRTUrena})

	// Find a second RT to cross-check (any other active RT in the dev seed).
	var otherRT string
	if err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT id FROM rts WHERE id <> $1 AND is_active = true LIMIT 1`, flRTUrena,
	).Scan(&otherRT); err != nil {
		t.Fatalf("resolve second RT: %v", err)
	}

	randomName := fmt.Sprintf("CrossRT-Rekening-%d", time.Now().UnixNano())
	createRekeningSeed(t, pool, flRTUrena, randomName, "rekening", true, "Mandiri", "0001112223", "RT002-Cross")

	_, wargaOther := createRTUser(t, pool, otherRT, auth.RoleWarga, "")
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/fund-locations/rekening/public?rt_id=%s", flRTUrena), nil)
	req.Header.Set("Authorization", "Bearer "+wargaOther)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK && rec.Code != http.StatusForbidden && rec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status for cross-RT probe: %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Code == http.StatusOK {
		var body struct {
			Data []map[string]any `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err == nil {
			for _, row := range body.Data {
				if fmt.Sprint(row["name"]) == randomName {
					t.Fatalf("cross-RT leak: warga from RT %s saw rekening of RT %s", otherRT, flRTUrena)
				}
			}
		}
	}
}
