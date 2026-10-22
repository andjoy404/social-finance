package finance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-finance/internal/auth"
)

func doFinanceRequest(
	t *testing.T,
	router http.Handler,
	method, path, token, body string,
) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Buffer
	if body == "" {
		reader = bytes.NewBuffer(nil)
	} else {
		reader = bytes.NewBufferString(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Authorization", "Bearer "+token)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestDueActiveStatusLifecycle(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID := setupTestRT(t, pool, "Finance RT Due Status", 1, "FDSTAT")
	defer cleanupTestRT(t, pool, rtID)

	userB, midB := setupTestUserWithMembership(
		t, pool, "b.due.status@test.com", "Bendahara Due Status",
		fmt.Sprintf("0813%08d", time.Now().UnixNano()%100000000), rtID, "bendahara",
	)
	userW, midW := setupTestUserWithMembership(
		t, pool, "w.due.status@test.com", "Warga Due Status",
		fmt.Sprintf("0814%08d", time.Now().UnixNano()%100000000), rtID, "warga",
	)
	tokenBendahara := makeToken(t, auth.TokenClaims{
		UserID: userB, MID: midB, RTID: rtID, Role: string(auth.RoleBendahara),
	})
	tokenWarga := makeToken(t, auth.TokenClaims{
		UserID: userW, MID: midW, RTID: rtID, Role: string(auth.RoleWarga),
	})

	occID := setupTestOccupancyWithResident(
		t, pool, rtID, fmt.Sprintf("0815%08d", time.Now().UnixNano()%100000000),
	)

	// A regular due must default to active on creation.
	rec := doFinanceRequest(t, router, http.MethodPost, "/api/v1/dues", tokenBendahara,
		`{"name":"Iuran Keamanan Status","amount":"50000","period_type":"monthly"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create due: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var due Due
	if err := json.Unmarshal(rec.Body.Bytes(), &due); err != nil {
		t.Fatalf("unmarshal due: %v", err)
	}
	if !due.IsActive {
		t.Fatalf("new regular due must be active by default")
	}

	// Warga must not be able to change due status.
	rec = doFinanceRequest(t, router, http.MethodPatch, "/api/v1/dues/"+due.ID, tokenWarga,
		`{"is_active":false}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("warga update due: expected 403, got %d: %s", rec.Code, rec.Body.String())
	}

	// A manager from another RT must not be able to change this RT's due.
	otherRTID := setupTestRT(t, pool, "Finance RT Due Status Other", 2, "FDSTAT2")
	defer cleanupTestRT(t, pool, otherRTID)
	userBO, midBO := setupTestUserWithMembership(
		t, pool, "b.other.due.status@test.com", "Bendahara Other",
		fmt.Sprintf("0816%08d", time.Now().UnixNano()%100000000), otherRTID, "bendahara",
	)
	tokenOther := makeToken(t, auth.TokenClaims{
		UserID: userBO, MID: midBO, RTID: otherRTID, Role: string(auth.RoleBendahara),
	})
	rec = doFinanceRequest(t, router, http.MethodPatch, "/api/v1/dues/"+due.ID, tokenOther,
		`{"is_active":false}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-RT update due: expected 404, got %d: %s", rec.Code, rec.Body.String())
	}

	// Generate a bill while active and mark it paid to prove history survives.
	rec = doFinanceRequest(t, router, http.MethodPost, "/api/v1/bills/generate", tokenBendahara,
		fmt.Sprintf(`{"due_id":"%s","period":"2026-10","due_date":"2026-10-31"}`, due.ID))
	if rec.Code != http.StatusCreated {
		t.Fatalf("generate active due: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var generated struct {
		GeneratedCount int    `json:"generated_count"`
		Bills          []Bill `json:"bills"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &generated); err != nil {
		t.Fatalf("unmarshal generated bills: %v", err)
	}
	if generated.GeneratedCount != 1 || len(generated.Bills) != 1 {
		t.Fatalf("expected 1 generated bill, got %d", generated.GeneratedCount)
	}
	billID := generated.Bills[0].ID
	if _, err := pool.Raw().ExecContext(context.Background(),
		`UPDATE bills SET status = 'paid' WHERE id = $1`, billID,
	); err != nil {
		t.Fatalf("mark bill paid: %v", err)
	}

	// Generate an older unpaid bill so we can prove arrears remain payable
	// after the due is deactivated.
	rec = doFinanceRequest(t, router, http.MethodPost, "/api/v1/bills/generate", tokenBendahara,
		fmt.Sprintf(`{"due_id":"%s","period":"2026-09","due_date":"2026-09-30"}`, due.ID))
	if rec.Code != http.StatusCreated {
		t.Fatalf("generate arrears bill: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var arrearsGenerated struct {
		GeneratedCount int    `json:"generated_count"`
		Bills          []Bill `json:"bills"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &arrearsGenerated); err != nil {
		t.Fatalf("unmarshal arrears bills: %v", err)
	}
	if arrearsGenerated.GeneratedCount != 1 || len(arrearsGenerated.Bills) != 1 {
		t.Fatalf("expected 1 arrears bill, got %d", arrearsGenerated.GeneratedCount)
	}
	arrearsBillID := arrearsGenerated.Bills[0].ID

	// Manager deactivates the due.
	rec = doFinanceRequest(t, router, http.MethodPatch, "/api/v1/dues/"+due.ID, tokenBendahara,
		`{"is_active":false}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("deactivate due: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var deactivated Due
	if err := json.Unmarshal(rec.Body.Bytes(), &deactivated); err != nil {
		t.Fatalf("unmarshal deactivated due: %v", err)
	}
	if deactivated.IsActive {
		t.Fatalf("due must be inactive after update")
	}

	readDues := func(query string) []Due {
		rec := doFinanceRequest(t, router, http.MethodGet, "/api/v1/dues"+query, tokenBendahara, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("list dues %q: expected 200, got %d: %s", query, rec.Code, rec.Body.String())
		}
		var list []Due
		if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
			t.Fatalf("unmarshal dues: %v", err)
		}
		return list
	}
	containsDue := func(list []Due) bool {
		for _, item := range list {
			if item.ID == due.ID {
				return true
			}
		}
		return false
	}

	if !containsDue(readDues("")) {
		t.Fatalf("inactive due must remain visible when listing all dues")
	}
	if containsDue(readDues("?is_active=true")) {
		t.Fatalf("inactive due must not be returned by is_active=true")
	}
	if !containsDue(readDues("?is_active=false")) {
		t.Fatalf("inactive due must be returned by is_active=false")
	}

	// Inactive due must not produce new bills through either endpoint.
	rec = doFinanceRequest(t, router, http.MethodPost, "/api/v1/bills/generate", tokenBendahara,
		fmt.Sprintf(`{"due_id":"%s","period":"2026-11","due_date":"2026-11-30"}`, due.ID))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("generate inactive due: expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = doFinanceRequest(t, router, http.MethodPost, "/api/v1/bills", tokenBendahara,
		fmt.Sprintf(
			`{"household_occupancy_id":"%s","due_id":"%s","amount":"50000","period":"2026-12","due_date":"2026-12-31"}`,
			occID, due.ID,
		))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("create bill for inactive due: expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	// The historical bill remains readable and keeps its paid status.
	rec = doFinanceRequest(t, router, http.MethodGet, "/api/v1/bills?period=2026-10", tokenBendahara, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list historical bills: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var page struct {
		Data []Bill `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("unmarshal bills: %v", err)
	}
	var historical *Bill
	for i := range page.Data {
		if page.Data[i].ID == billID {
			historical = &page.Data[i]
			break
		}
	}
	if historical == nil {
		t.Fatalf("historical bill must remain visible after due deactivation")
	}
	if historical.Status != BillStatusPaid {
		t.Fatalf("historical bill status changed: got %s", historical.Status)
	}

	// Existing arrears remain payable even though the due is inactive.
	rec = doFinanceRequest(t, router, http.MethodPost, "/api/v1/payments", tokenBendahara,
		fmt.Sprintf(`{"bill_id":"%s","amount":"50000","method":"CASH"}`, arrearsBillID))
	if rec.Code != http.StatusCreated {
		t.Fatalf("pay arrears bill for inactive due: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = doFinanceRequest(t, router, http.MethodGet, "/api/v1/bills/"+arrearsBillID, tokenBendahara, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("read paid arrears bill: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var arrearsBill Bill
	if err := json.Unmarshal(rec.Body.Bytes(), &arrearsBill); err != nil {
		t.Fatalf("unmarshal arrears bill: %v", err)
	}
	if arrearsBill.Status != BillStatusPaid {
		t.Fatalf("arrears bill must become paid, got %s", arrearsBill.Status)
	}

	// Reactivation allows bill generation again.
	rec = doFinanceRequest(t, router, http.MethodPatch, "/api/v1/dues/"+due.ID, tokenBendahara,
		`{"is_active":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("reactivate due: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = doFinanceRequest(t, router, http.MethodPost, "/api/v1/bills/generate", tokenBendahara,
		fmt.Sprintf(`{"due_id":"%s","period":"2026-11","due_date":"2026-11-30"}`, due.ID))
	if rec.Code != http.StatusCreated {
		t.Fatalf("generate reactivated due: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestVoluntaryDueDeactivationBlocksNewDonationButKeepsHistory(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID := setupTestRT(t, pool, "Finance RT Voluntary Status", 3, "FVSTAT")
	defer cleanupTestRT(t, pool, rtID)

	userB, midB := setupTestUserWithMembership(
		t, pool, "b.vol.status@test.com", "Bendahara Voluntary Status",
		fmt.Sprintf("0821%08d", time.Now().UnixNano()%100000000), rtID, "bendahara",
	)
	userW, midW := setupTestUserWithMembership(
		t, pool, "w.vol.status@test.com", "Warga Voluntary Status",
		fmt.Sprintf("0822%08d", time.Now().UnixNano()%100000000), rtID, "warga",
	)
	tokenBendahara := makeToken(t, auth.TokenClaims{
		UserID: userB, MID: midB, RTID: rtID, Role: string(auth.RoleBendahara),
	})
	tokenWarga := makeToken(t, auth.TokenClaims{
		UserID: userW, MID: midW, RTID: rtID, Role: string(auth.RoleWarga),
	})
	setupOccupancyWithLinkedResident(t, pool, rtID, userW)

	rec := doFinanceRequest(t, router, http.MethodPost, "/api/v1/dues", tokenBendahara,
		`{"name":"Donasi Sukarela Status","amount":"0","period_type":"monthly","is_voluntary":true}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create voluntary due: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var due Due
	if err := json.Unmarshal(rec.Body.Bytes(), &due); err != nil {
		t.Fatalf("unmarshal voluntary due: %v", err)
	}
	if !due.IsActive {
		t.Fatalf("new voluntary due must default to active")
	}

	donationBody := fmt.Sprintf(`{"due_id":"%s","amount":"50000","method":"CASH"}`, due.ID)
	rec = doFinanceRequest(t, router, http.MethodPost, "/api/v1/payments", tokenWarga, donationBody)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create donation while active: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var donation Payment
	if err := json.Unmarshal(rec.Body.Bytes(), &donation); err != nil {
		t.Fatalf("unmarshal donation: %v", err)
	}

	rec = doFinanceRequest(t, router, http.MethodPatch, "/api/v1/dues/"+due.ID, tokenBendahara,
		`{"is_active":false}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("deactivate voluntary due: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// The historical donation remains readable and keeps its status.
	rec = doFinanceRequest(t, router, http.MethodGet, "/api/v1/payments/"+donation.ID, tokenBendahara, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("read historical donation: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var history Payment
	if err := json.Unmarshal(rec.Body.Bytes(), &history); err != nil {
		t.Fatalf("unmarshal historical donation: %v", err)
	}
	if history.Status != PaymentStatusPending {
		t.Fatalf("historical donation status changed: got %s", history.Status)
	}

	// New donations to the inactive voluntary due are rejected.
	rec = doFinanceRequest(t, router, http.MethodPost, "/api/v1/payments", tokenWarga, donationBody)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("donate to inactive due: expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}
