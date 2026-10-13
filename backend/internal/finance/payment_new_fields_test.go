package finance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"social-finance/internal/auth"
	"social-finance/internal/database"
)

// cleanupTestPayments removes all payments tied to the given RT so that
// the payment_no generator (PAY-YYYYMMDD-NNNNN) starts from 00001.
// This avoids collisions with seed/migration data that may already
// have payment_no values for payments created before migration 016 ran.
func cleanupTestPayments(t *testing.T, pool *database.Pool, rtID string) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Raw().ExecContext(ctx,
		`DELETE FROM payments WHERE rt_id = $1`, rtID,
	)
	if err != nil {
		t.Fatalf("cleanupTestPayments(rt=%s): %v", rtID, err)
	}
}

// TestPaymentNewFieldsOfficerNotesApprove verifies that officer_notes is
// persisted when approving a payment and returned in the response.
func TestPaymentNewFieldsOfficerNotesApprove(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID := setupTestRT(t, pool, "NewFields RT", 1, "NFIELD")
	defer cleanupTestRT(t, pool, rtID)

	cleanupTestPayments(t, pool, rtID)

	phoneW := fmt.Sprintf("0833%08d", time.Now().UnixNano()%100000000)
	userW, midW := setupTestUserWithMembership(t, pool, "w@nf.test", "Warga NF", phoneW, rtID, "warga")

	phoneB := fmt.Sprintf("0811%08d", time.Now().UnixNano()%100000000)
	userB, midB := setupTestUserWithMembership(t, pool, "b@nf.test", "Bendahara NF", phoneB, rtID, "bendahara")

	occID := setupTestOccupancyWithResident(t, pool, rtID, phoneW)
	tokenWarga := makeToken(t, auth.TokenClaims{UserID: userW, MID: midW, RTID: rtID, Role: string(auth.RoleWarga)})
	tokenBendahara := makeToken(t, auth.TokenClaims{UserID: userB, MID: midB, RTID: rtID, Role: string(auth.RoleBendahara)})

	_ = occID

	// Create due
	var dueID string
	{
		body := `{"name":"Iuran Testing","amount":"50000","period_type":"monthly"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create due: %d: %s", rec.Code, rec.Body.String())
		}
		var d Due
		json.Unmarshal(rec.Body.Bytes(), &d)
		dueID = d.ID
	}

	// Generate bill
	{
		body := fmt.Sprintf(`{"due_id":"%s","period":"2026-11-nf","due_date":"2026-11-25"}`, dueID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("generate bills: %d: %s", rec.Code, rec.Body.String())
		}
	}

	// Get bill ID
	var billID string
	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/bills?period=2026-11-nf", nil)
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		var listResp struct {
			Data []Bill `json:"data"`
		}
		json.Unmarshal(rec.Body.Bytes(), &listResp)
		if len(listResp.Data) == 0 {
			t.Fatal("expected at least 1 bill")
		}
		billID = listResp.Data[0].ID
	}

	// Warga submits payment (PENDING)
	var paymentID string
	{
		body := fmt.Sprintf(`{"bill_id":"%s","amount":"50000","method":"TRANSFER","notes":"Test transfer"}`, billID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("submit payment: %d: %s", rec.Code, rec.Body.String())
		}
		var p Payment
		json.Unmarshal(rec.Body.Bytes(), &p)
		if p.Status != PaymentStatusPending {
			t.Fatalf("expected PENDING, got %s", p.Status)
		}
		paymentID = p.ID
	}

	// Approve with officer_notes
	{
		officerNotes := "Pembayaran sesuai"
		body := fmt.Sprintf(`{"action":"approve","officer_notes":"%s"}`, officerNotes)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+paymentID+"/verify", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("verify payment: %d: %s", rec.Code, rec.Body.String())
		}
		var p Payment
		json.Unmarshal(rec.Body.Bytes(), &p)
		if p.Status != PaymentStatusApproved {
			t.Errorf("expected APPROVED, got %s", p.Status)
		}
		if p.OfficerNotes == nil {
			t.Fatal("expected officer_notes to be set on approve")
		}
		if *p.OfficerNotes != "Pembayaran sesuai" {
			t.Errorf("officer_notes = %q, want %q", *p.OfficerNotes, "Pembayaran sesuai")
		}
		if p.RejectionReason != nil {
			t.Errorf("expected rejection_reason to be nil, got %q", *p.RejectionReason)
		}
	}
}

// TestPaymentNewFieldsOfficerNotesReject verifies that officer_notes is
// persisted when rejecting a payment along with rejection_reason.
func TestPaymentNewFieldsOfficerNotesReject(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID := setupTestRT(t, pool, "NewFields Reject RT", 1, "NFRJ")
	defer cleanupTestRT(t, pool, rtID)

	cleanupTestPayments(t, pool, rtID)

	phoneW := fmt.Sprintf("0834%08d", time.Now().UnixNano()%100000000)
	userW, midW := setupTestUserWithMembership(t, pool, "w@nr.test", "Warga NR", phoneW, rtID, "warga")

	phoneB := fmt.Sprintf("0812%08d", time.Now().UnixNano()%100000000)
	userB, midB := setupTestUserWithMembership(t, pool, "b@nr.test", "Bendahara NR", phoneB, rtID, "bendahara")

	occID := setupTestOccupancyWithResident(t, pool, rtID, phoneW)
	tokenWarga := makeToken(t, auth.TokenClaims{UserID: userW, MID: midW, RTID: rtID, Role: string(auth.RoleWarga)})
	tokenBendahara := makeToken(t, auth.TokenClaims{UserID: userB, MID: midB, RTID: rtID, Role: string(auth.RoleBendahara)})

	_ = occID

	// Create due and bill
	var dueID string
	{
		body := `{"name":"Iuran Reject Test","amount":"50000","period_type":"monthly"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create due: %d: %s", rec.Code, rec.Body.String())
		}
		var d Due
		json.Unmarshal(rec.Body.Bytes(), &d)
		dueID = d.ID
	}

	{
		body := fmt.Sprintf(`{"due_id":"%s","period":"2026-11-nfrj","due_date":"2026-11-25"}`, dueID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("generate bills: %d: %s", rec.Code, rec.Body.String())
		}
	}

	var billID string
	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/bills?period=2026-11-nfrj", nil)
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		var listResp struct {
			Data []Bill `json:"data"`
		}
		json.Unmarshal(rec.Body.Bytes(), &listResp)
		if len(listResp.Data) == 0 {
			t.Fatal("expected at least 1 bill")
		}
		billID = listResp.Data[0].ID
	}

	// Submit payment
	var paymentID string
	{
		body := fmt.Sprintf(`{"bill_id":"%s","amount":"50000","method":"CASH"}`, billID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("submit payment: %d: %s", rec.Code, rec.Body.String())
		}
		var p Payment
		json.Unmarshal(rec.Body.Bytes(), &p)
		paymentID = p.ID
	}

	// Reject with officer_notes and rejection_reason
	{
		body := `{"action":"reject","officer_notes":"Bukti tidak jelas","rejection_reason":"Foto buram"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+paymentID+"/verify", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("reject payment: %d: %s", rec.Code, rec.Body.String())
		}
		var p Payment
		json.Unmarshal(rec.Body.Bytes(), &p)
		if p.Status != PaymentStatusRejected {
			t.Errorf("expected REJECTED, got %s", p.Status)
		}
		if p.OfficerNotes == nil {
			t.Fatal("expected officer_notes to be set on reject")
		}
		if *p.OfficerNotes != "Bukti tidak jelas" {
			t.Errorf("officer_notes = %q, want %q", *p.OfficerNotes, "Bukti tidak jelas")
		}
		if p.RejectionReason == nil {
			t.Fatal("expected rejection_reason to be set on reject")
		}
		if *p.RejectionReason != "Foto buram" {
			t.Errorf("rejection_reason = %q, want %q", *p.RejectionReason, "Foto buram")
		}
	}
}

// TestPaymentNewFieldsRejectionReasonOnly verifies that rejection_reason works
// correctly when officer_notes is NOT provided.
func TestPaymentNewFieldsRejectionReasonOnly(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID := setupTestRT(t, pool, "NewFields RJOnly RT", 1, "NFRO")
	defer cleanupTestRT(t, pool, rtID)

	cleanupTestPayments(t, pool, rtID)

	phoneW := fmt.Sprintf("0835%08d", time.Now().UnixNano()%100000000)
	userW, midW := setupTestUserWithMembership(t, pool, "w@nj.test", "Warga NJ", phoneW, rtID, "warga")

	phoneB := fmt.Sprintf("0813%08d", time.Now().UnixNano()%100000000)
	userB, midB := setupTestUserWithMembership(t, pool, "b@nj.test", "Bendahara NJ", phoneB, rtID, "bendahara")

	occID := setupTestOccupancyWithResident(t, pool, rtID, phoneW)
	tokenWarga := makeToken(t, auth.TokenClaims{UserID: userW, MID: midW, RTID: rtID, Role: string(auth.RoleWarga)})
	tokenBendahara := makeToken(t, auth.TokenClaims{UserID: userB, MID: midB, RTID: rtID, Role: string(auth.RoleBendahara)})

	_ = occID

	var dueID string
	{
		body := `{"name":"Iuran RJOnly","amount":"50000","period_type":"monthly"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create due: %d: %s", rec.Code, rec.Body.String())
		}
		var d Due
		json.Unmarshal(rec.Body.Bytes(), &d)
		dueID = d.ID
	}

	{
		body := fmt.Sprintf(`{"due_id":"%s","period":"2026-11-nfro","due_date":"2026-11-25"}`, dueID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("generate bills: %d: %s", rec.Code, rec.Body.String())
		}
	}

	var billID, paymentID string
	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/bills?period=2026-11-nfro", nil)
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		var listResp struct {
			Data []Bill `json:"data"`
		}
		json.Unmarshal(rec.Body.Bytes(), &listResp)
		if len(listResp.Data) == 0 {
			t.Fatal("expected at least 1 bill")
		}
		billID = listResp.Data[0].ID
	}
	{
		body := fmt.Sprintf(`{"bill_id":"%s","amount":"50000","method":"CASH"}`, billID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("submit payment: %d: %s", rec.Code, rec.Body.String())
		}
		var p Payment
		json.Unmarshal(rec.Body.Bytes(), &p)
		paymentID = p.ID
	}

	// Reject with rejection_reason ONLY — no officer_notes field at all
	{
		body := `{"action":"reject","rejection_reason":"Transfer ke rekening salah"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+paymentID+"/verify", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("reject payment: %d: %s", rec.Code, rec.Body.String())
		}
		var p Payment
		json.Unmarshal(rec.Body.Bytes(), &p)
		if p.Status != PaymentStatusRejected {
			t.Errorf("expected REJECTED, got %s", p.Status)
		}
		if p.RejectionReason == nil {
			t.Fatal("expected rejection_reason to be set")
		}
		if *p.RejectionReason != "Transfer ke rekening salah" {
			t.Errorf("rejection_reason = %q, want %q", *p.RejectionReason, "Transfer ke rekening salah")
		}
		// officer_notes should be nil when not provided
		if p.OfficerNotes != nil {
			t.Errorf("expected officer_notes to be nil, got %q", *p.OfficerNotes)
		}
	}
}

// TestPaymentNewFieldsVerifiedByName verifies that verified_by_name contains
// the user's full_name (not UUID) from the users table.
func TestPaymentNewFieldsVerifiedByName(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID := setupTestRT(t, pool, "NewFields VBN RT", 1, "NFVN")
	defer cleanupTestRT(t, pool, rtID)

	cleanupTestPayments(t, pool, rtID)

	// Create warga user to submit payments
	phoneW := fmt.Sprintf("0836%08d", time.Now().UnixNano()%100000000)
	userW, midW := setupTestUserWithMembership(t, pool, "w@nv.test", "Warga NV", phoneW, rtID, "warga")
	occID := setupTestOccupancyWithResident(t, pool, rtID, phoneW)
	tokenWarga := makeToken(t, auth.TokenClaims{UserID: userW, MID: midW, RTID: rtID, Role: string(auth.RoleWarga)})
	_ = occID

	// Use a distinctive full_name for the verifier
	verifierFullName := "Bendahara Satu"
	phoneB := fmt.Sprintf("0814%08d", time.Now().UnixNano()%100000000)
	_ = phoneB

	db := pool.Raw()
	ctx := context.Background()
	// Use unique email (with counter prefix) to avoid constraint conflicts
	c := atomic.AddInt64(&rtCounter, 1)
	uniqueEmail := fmt.Sprintf("%d_bendahara_one@test.com", c)
	var verifierUID, verifierMID string
	err := db.QueryRowContext(ctx,
		`INSERT INTO users (email, phone, password_hash, full_name, is_active) VALUES ($1, $2, 'dummyhash', $3, true) RETURNING id`,
		uniqueEmail, phoneB, verifierFullName,
	).Scan(&verifierUID)
	if err != nil {
		t.Fatalf("create bendahara user: %v", err)
	}
	err = db.QueryRowContext(ctx,
		`INSERT INTO user_rt_memberships (user_id, rt_id, role, is_active) VALUES ($1, $2, 'bendahara', true) RETURNING id`,
		verifierUID, rtID,
	).Scan(&verifierMID)
	if err != nil {
		t.Fatalf("create bendahara membership: %v", err)
	}

	tokenBendahara := makeToken(t, auth.TokenClaims{
		UserID: verifierUID,
		MID:    verifierMID,
		RTID:   rtID,
		Role:   string(auth.RoleBendahara),
	})

	var dueID string
	{
		body := `{"name":"Iuran VBN","amount":"50000","period_type":"monthly"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create due: %d: %s", rec.Code, rec.Body.String())
		}
		var d Due
		json.Unmarshal(rec.Body.Bytes(), &d)
		dueID = d.ID
	}

	{
		body := fmt.Sprintf(`{"due_id":"%s","period":"2026-11-nfyn","due_date":"2026-11-25"}`, dueID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("generate bills: %d: %s", rec.Code, rec.Body.String())
		}
	}

	var billID, paymentID string
	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/bills?period=2026-11-nfyn", nil)
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		var listResp struct {
			Data []Bill `json:"data"`
		}
		json.Unmarshal(rec.Body.Bytes(), &listResp)
		if len(listResp.Data) == 0 {
			t.Fatal("expected at least 1 bill")
		}
		billID = listResp.Data[0].ID
	}

	// Warga submits payment
	{
		body := fmt.Sprintf(`{"bill_id":"%s","amount":"50000","method":"TRANSFER"}`, billID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("submit payment: %d: %s", rec.Code, rec.Body.String())
		}
		var p Payment
		json.Unmarshal(rec.Body.Bytes(), &p)
		paymentID = p.ID
	}

	// Approve — verified_by_name should be bendahara's full_name
	{
		body := `{"action":"approve"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+paymentID+"/verify", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("approve payment: %d: %s", rec.Code, rec.Body.String())
		}
		var p Payment
		json.Unmarshal(rec.Body.Bytes(), &p)
		if p.Status != PaymentStatusApproved {
			t.Errorf("expected APPROVED, got %s", p.Status)
		}
		if p.VerifiedByName == nil {
			t.Fatal("expected verified_by_name to be set")
		}
		if *p.VerifiedByName != verifierFullName {
			t.Errorf("verified_by_name = %q, want %q", *p.VerifiedByName, verifierFullName)
		}
		// Also verify it is NOT a UUID
		if strings.HasPrefix(*p.VerifiedByName, "00000000-") || strings.HasPrefix(*p.VerifiedByName, "11111111-") {
			t.Errorf("verified_by_name appears to be a UUID: %q, expected full_name", *p.VerifiedByName)
		}
	}
}

// TestPaymentNewFieldsPaymentNoNew verifies that payment_no is generated for
// new payments in the expected format.
func TestPaymentNewFieldsPaymentNoNew(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID := setupTestRT(t, pool, "NewFields PN RT", 1, "NFPN")
	defer cleanupTestRT(t, pool, rtID)

	cleanupTestPayments(t, pool, rtID)

	phoneW := fmt.Sprintf("0837%08d", time.Now().UnixNano()%100000000)
	userW, midW := setupTestUserWithMembership(t, pool, "w@np.test", "Warga NP", phoneW, rtID, "warga")

	phoneB := fmt.Sprintf("0815%08d", time.Now().UnixNano()%100000000)
	userB, midB := setupTestUserWithMembership(t, pool, "b@np.test", "Bendahara NP", phoneB, rtID, "bendahara")

	occID := setupTestOccupancyWithResident(t, pool, rtID, phoneW)
	tokenWarga := makeToken(t, auth.TokenClaims{UserID: userW, MID: midW, RTID: rtID, Role: string(auth.RoleWarga)})
	tokenBendahara := makeToken(t, auth.TokenClaims{UserID: userB, MID: midB, RTID: rtID, Role: string(auth.RoleBendahara)})

	_ = occID

	var dueID string
	{
		body := `{"name":"Iuran PNo","amount":"50000","period_type":"monthly"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create due: %d: %s", rec.Code, rec.Body.String())
		}
		var d Due
		json.Unmarshal(rec.Body.Bytes(), &d)
		dueID = d.ID
	}

	{
		body := fmt.Sprintf(`{"due_id":"%s","period":"2026-11-nfnp","due_date":"2026-11-25"}`, dueID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("generate bills: %d: %s", rec.Code, rec.Body.String())
		}
	}

	var billID, paymentID string
	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/bills?period=2026-11-nfnp", nil)
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		var listResp struct {
			Data []Bill `json:"data"`
		}
		json.Unmarshal(rec.Body.Bytes(), &listResp)
		if len(listResp.Data) == 0 {
			t.Fatal("expected at least 1 bill")
		}
		billID = listResp.Data[0].ID
	}

	// Warga submits payment — payment_no should be nil before verify
	{
		body := fmt.Sprintf(`{"bill_id":"%s","amount":"50000","method":"TRANSFER"}`, billID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("submit payment: %d: %s", rec.Code, rec.Body.String())
		}
		var p Payment
		json.Unmarshal(rec.Body.Bytes(), &p)
		paymentID = p.ID
		// Before verify, payment_no may be nil (generated at verify time)
		_ = p.PaymentNo
	}

	// Approve — payment_no should now be set
	{
		body := `{"action":"approve"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+paymentID+"/verify", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("approve payment: %d: %s", rec.Code, rec.Body.String())
		}
		var p Payment
		json.Unmarshal(rec.Body.Bytes(), &p)
		if p.PaymentNo == nil {
			t.Fatal("expected payment_no to be set after verify")
		}
		// Format: PAY-YYYYMMDD-NNNNN
		pn := *p.PaymentNo
		if len(pn) < 15 {
			t.Errorf("payment_no too short: %q", pn)
		}
		if !strings.HasPrefix(pn, "PAY-") {
			t.Errorf("payment_no should start with PAY-, got %q", pn)
		}
		// Verify date part is today's date
		datePart := pn[4:12]
		expectedDate := time.Now().Format("20060102")
		if datePart != expectedDate {
			t.Errorf("payment_no date part = %q, want %q", datePart, expectedDate)
		}
	}
}

// TestPaymentNewFieldsPaymentNoStable verifies that payment_no remains the
// same before and after verification (if already generated) or is consistently
// generated at verify time.
func TestPaymentNewFieldsPaymentNoStable(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID := setupTestRT(t, pool, "NewFields PNS RT", 1, "NFPS")
	defer cleanupTestRT(t, pool, rtID)

	cleanupTestPayments(t, pool, rtID)

	phoneW := fmt.Sprintf("0838%08d", time.Now().UnixNano()%100000000)
	userW, midW := setupTestUserWithMembership(t, pool, "w@ns.test", "Warga NS", phoneW, rtID, "warga")

	phoneB := fmt.Sprintf("0816%08d", time.Now().UnixNano()%100000000)
	userB, midB := setupTestUserWithMembership(t, pool, "b@ns.test", "Bendahara NS", phoneB, rtID, "bendahara")

	occID := setupTestOccupancyWithResident(t, pool, rtID, phoneW)
	tokenWarga := makeToken(t, auth.TokenClaims{UserID: userW, MID: midW, RTID: rtID, Role: string(auth.RoleWarga)})
	tokenBendahara := makeToken(t, auth.TokenClaims{UserID: userB, MID: midB, RTID: rtID, Role: string(auth.RoleBendahara)})

	_ = occID

	var dueID string
	{
		body := `{"name":"Iuran PNS","amount":"50000","period_type":"monthly"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create due: %d: %s", rec.Code, rec.Body.String())
		}
		var d Due
		json.Unmarshal(rec.Body.Bytes(), &d)
		dueID = d.ID
	}

	{
		body := fmt.Sprintf(`{"due_id":"%s","period":"2026-11-nfps","due_date":"2026-11-25"}`, dueID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("generate bills: %d: %s", rec.Code, rec.Body.String())
		}
	}

	var billID, paymentID string
	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/bills?period=2026-11-nfps", nil)
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		var listResp struct {
			Data []Bill `json:"data"`
		}
		json.Unmarshal(rec.Body.Bytes(), &listResp)
		if len(listResp.Data) == 0 {
			t.Fatal("expected at least 1 bill")
		}
		billID = listResp.Data[0].ID
	}

	// Submit payment
	{
		body := fmt.Sprintf(`{"bill_id":"%s","amount":"50000","method":"CASH"}`, billID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("submit payment: %d: %s", rec.Code, rec.Body.String())
		}
		var p Payment
		json.Unmarshal(rec.Body.Bytes(), &p)
		paymentID = p.ID
	}

	// Verify (approve)
	{
		body := `{"action":"approve"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+paymentID+"/verify", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("approve payment: %d: %s", rec.Code, rec.Body.String())
		}
		var p Payment
		json.Unmarshal(rec.Body.Bytes(), &p)
		if p.PaymentNo == nil {
			t.Fatal("expected payment_no after verify")
		}
		firstPN := *p.PaymentNo
		firstID := p.ID

		// Get payment by ID — payment_no should be the same
		getReq := httptest.NewRequest(http.MethodGet, "/api/v1/payments/"+firstID, nil)
		getReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
		getRec := httptest.NewRecorder()
		router.ServeHTTP(getRec, getReq)
		if getRec.Code != http.StatusOK {
			t.Fatalf("get payment: %d: %s", getRec.Code, getRec.Body.String())
		}
		var p2 Payment
		json.Unmarshal(getRec.Body.Bytes(), &p2)
		if p2.PaymentNo == nil {
			t.Fatal("expected payment_no in get response")
		}
		secondPN := *p2.PaymentNo
		if firstPN != secondPN {
			t.Errorf("payment_no changed: after verify = %q, after get = %q", firstPN, secondPN)
		}
	}
}

// TestPaymentNewFieldsOldPaymentsSafe verifies that existing payments
// (where verified_by_name, officer_notes, payment_no may be NULL) do not
// cause GetPaymentByID to crash.
func TestPaymentNewFieldsOldPaymentsSafe(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID := setupTestRT(t, pool, "NewFields Old RT", 1, "NFOD")
	defer cleanupTestRT(t, pool, rtID)

	cleanupTestPayments(t, pool, rtID)

	phoneW := fmt.Sprintf("0839%08d", time.Now().UnixNano()%100000000)
	_ = phoneW

	phoneB := fmt.Sprintf("0817%08d", time.Now().UnixNano()%100000000)
	userB, midB := setupTestUserWithMembership(t, pool, "b@no.test", "Bendahara NO", phoneB, rtID, "bendahara")

	occID := setupTestOccupancyWithResident(t, pool, rtID, phoneW)
	tokenBendahara := makeToken(t, auth.TokenClaims{UserID: userB, MID: midB, RTID: rtID, Role: string(auth.RoleBendahara)})

	_ = occID

	// Create due and bill
	var dueID string
	{
		body := `{"name":"Iuran Old","amount":"50000","period_type":"monthly"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create due: %d: %s", rec.Code, rec.Body.String())
		}
		var d Due
		json.Unmarshal(rec.Body.Bytes(), &d)
		dueID = d.ID
	}

	{
		body := fmt.Sprintf(`{"due_id":"%s","period":"2026-11-nfod","due_date":"2026-11-25"}`, dueID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("generate bills: %d: %s", rec.Code, rec.Body.String())
		}
	}

	var billID string
	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/bills?period=2026-11-nfod", nil)
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		var listResp struct {
			Data []Bill `json:"data"`
		}
		json.Unmarshal(rec.Body.Bytes(), &listResp)
		if len(listResp.Data) == 0 {
			t.Fatal("expected at least 1 bill")
		}
		billID = listResp.Data[0].ID
	}

	// Insert an "old" payment directly into the DB with NULL for new fields.
	// This simulates a payment created before the officer_notes, verified_by_name,
	// and payment_no columns existed.
	db := pool.Raw()
	ctx := context.Background()

	var oldPaymentID string
	err := db.QueryRowContext(ctx,
		`INSERT INTO payments (rt_id, bill_id, amount, method, origin, status, notes, paid_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, CURRENT_TIMESTAMP)
		 RETURNING id`,
		rtID, billID, "50000", "CASH", PaymentOriginStaffRecorded, PaymentStatusPending, "Old payment note",
	).Scan(&oldPaymentID)
	if err != nil {
		t.Fatalf("insert old payment: %v", err)
	}

	// Verify no crash on GetPaymentByID
	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/payments/"+oldPaymentID, nil)
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("get old payment: %d: %s", rec.Code, rec.Body.String())
		}
		var p Payment
		json.Unmarshal(rec.Body.Bytes(), &p)
		// New fields should be nil for old payments
		if p.VerifiedByName != nil {
			t.Errorf("verified_by_name should be nil for old payment, got %q", *p.VerifiedByName)
		}
		if p.OfficerNotes != nil {
			t.Errorf("officer_notes should be nil for old payment, got %q", *p.OfficerNotes)
		}
		if p.PaymentNo != nil {
			t.Errorf("payment_no should be nil for old payment, got %q", *p.PaymentNo)
		}
	}
}
