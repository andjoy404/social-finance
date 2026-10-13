package finance

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

	"social-finance/internal/auth"
)

// TestOverpaymentRejectedPersisted verifies that when a PENDING payment
// (with amount exceeding the remaining balance) is verified by a bendahara,
// the overpayment path rejects the payment atomically: payment row is
// updated to REJECTED, the transaction commits (not rolls back), and the
// HTTP 400 response includes "exceeds remaining balance".
func TestOverpaymentRejectedPersisted(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID := setupTestRT(t, pool, "Overpay Persist RT", 1, "OVP1")
	defer cleanupTestRT(t, pool, rtID)
	cleanupTestPayments(t, pool, rtID)

	phoneW := fmt.Sprintf("0840%08d", time.Now().UnixNano()%100000000)
	_ = fmt.Sprintf("Warga OP1 %s", phoneW)
	_ = setupTestOccupancyWithResident(t, pool, rtID, phoneW)

	phoneB := fmt.Sprintf("0817%08d", time.Now().UnixNano()%100000000)
	userB, midB := setupTestUserWithMembership(t, pool, "b@op1.test", "Bendahara OP1", phoneB, rtID, "bendahara")
	tokenBendahara := makeToken(t, auth.TokenClaims{UserID: userB, MID: midB, RTID: rtID, Role: string(auth.RoleBendahara)})

	// Create due with amount = 50000.
	var dueID string
	{
		body := `{"name":"Iuran OVP1","amount":"50000","period_type":"monthly"}`
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

	// Generate bill.
	{
		body := fmt.Sprintf(`{"due_id":"%s","period":"2026-12-ovp1","due_date":"2026-12-25"}`, dueID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("generate bills: %d: %s", rec.Code, rec.Body.String())
		}
	}

	// Get bill ID.
	var billID string
	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/bills?period=2026-12-ovp1", nil)
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

	// Insert a PENDING payment directly via SQL that exceeds the remaining
	// balance (bill amount 50000, payment 150000). This simulates a scenario
	// where CreatePayment's overpayment check was bypassed or a PENDING
	// payment is created by another mechanism (e.g. data import).
	db := pool.Raw()
	ctx := context.Background()
	var paymentID string
	err := db.QueryRowContext(ctx,
		`INSERT INTO payments (rt_id, bill_id, amount, method, origin, status, notes,
		                        verified_by, verified_at, created_at, updated_at)
		 VALUES ($1, $2, '150000.00', 'TRANSFER', 'SELF_SUBMITTED', 'PENDING',
		         'Overpayment for testing', NULL, NULL, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		 RETURNING id`,
		rtID, billID,
	).Scan(&paymentID)
	if err != nil {
		t.Fatalf("insert overpayment: %v", err)
	}

	// Bendahara verifies the payment. Expect HTTP 400 (overpayment).
	{
		body := `{"action":"approve"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+paymentID+"/verify", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected HTTP 400, got %d: %s", rec.Code, rec.Body.String())
		}
		bodyStr := rec.Body.String()
		if !strings.Contains(bodyStr, "exceeds remaining balance") {
			t.Errorf("response body should mention 'exceeds remaining balance', got: %s", bodyStr)
		}
	}

	// ── Critical: verify DB state post-transaction ──
	// The REJECTED state must be persisted despite HTTP 400.
	var status, amount, notesStr string
	var rejectionReason, officerNotes, verifierName, paymentNo *string
	err = db.QueryRowContext(ctx,
		`SELECT status, amount::text, notes, rejection_reason, officer_notes, verified_by_name, payment_no
		 FROM payments WHERE id = $1`,
		paymentID,
	).Scan(&status, &amount, &notesStr, &rejectionReason, &officerNotes, &verifierName, &paymentNo)
	if err != nil {
		t.Fatalf("query payment: %v", err)
	}
	if status != string(PaymentStatusRejected) {
		t.Errorf("expected payment status REJECTED, got %s", status)
	}
	if amount != "150000.00" {
		t.Errorf("expected amount 150000.00, got %s", amount)
	}
	if rejectionReason == nil {
		t.Fatal("expected rejection_reason to be set")
	} else if !strings.Contains(*rejectionReason, "exceeds remaining balance") {
		t.Errorf("expected rejection_reason to mention overpayment, got %q", *rejectionReason)
	}
	if verifierName == nil {
		t.Fatal("expected verified_by_name to be set")
	} else if *verifierName == "Unknown" {
		t.Error("expected bendahara's full_name, got 'Unknown'")
	} else if strings.HasPrefix(*verifierName, "00000000-") {
		t.Errorf("expected full_name (not UUID), got %q", *verifierName)
	}
	if paymentNo == nil {
		t.Error("expected payment_no to be set after verification")
	}

	// Verify audit log was created with correct EntityID.
	var auditCount int
	err = db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM audit_logs WHERE rt_id = $1 AND entity_type = 'payment' AND entity_id = $2 AND action = 'verify_payment'`,
		rtID, paymentID,
	).Scan(&auditCount)
	if err != nil {
		t.Fatalf("query audit logs: %v", err)
	}
	if auditCount != 1 {
		t.Errorf("expected 1 audit log for payment verification, got %d", auditCount)
	}

	// Verify bill status is STILL not paid (because payment was rejected).
	var billStatus string
	err = db.QueryRowContext(ctx,
		`SELECT status FROM bills WHERE id = $1`, billID,
	).Scan(&billStatus)
	if err != nil {
		t.Fatalf("query bill: %v", err)
	}
	if billStatus != "unpaid" {
		t.Errorf("expected bill status 'unpaid' (overpayment rejected), got %q", billStatus)
	}
}

// TestOverpaymentRejectedAuditLogHasValidEntityID verifies that when
// overpayment rejection is successful, the audit log entry contains a
// valid EntityID (the payment's actual UUID, not empty/garbage).
func TestOverpaymentRejectedAuditLogHasValidEntityID(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID := setupTestRT(t, pool, "Overpay Audit RT", 1, "OVP2")
	defer cleanupTestRT(t, pool, rtID)
	cleanupTestPayments(t, pool, rtID)

	phoneW := fmt.Sprintf("0841%08d", time.Now().UnixNano()%100000000)
	_ = setupTestOccupancyWithResident(t, pool, rtID, phoneW)

	phoneB := fmt.Sprintf("0818%08d", time.Now().UnixNano()%100000000)
	userB, midB := setupTestUserWithMembership(t, pool, "b@op2.test", "Bendahara OP2", phoneB, rtID, "bendahara")
	tokenBendahara := makeToken(t, auth.TokenClaims{UserID: userB, MID: midB, RTID: rtID, Role: string(auth.RoleBendahara)})

	// Create due, bill.
	var dueID, billID string
	{
		body := `{"name":"Iuran OVP2","amount":"100000","period_type":"monthly"}`
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
		body := fmt.Sprintf(`{"due_id":"%s","period":"2026-12-ovp2","due_date":"2026-12-25"}`, dueID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("generate bills: %d: %s", rec.Code, rec.Body.String())
		}
	}
	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/bills?period=2026-12-ovp2", nil)
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

	// Insert PENDING payment with amount = 500000 (5x bill) directly via SQL.
	db := pool.Raw()
	ctx := context.Background()
	var paymentID string
	err := db.QueryRowContext(ctx,
		`INSERT INTO payments (rt_id, bill_id, amount, method, origin, status, notes,
		                        created_at, updated_at)
		 VALUES ($1, $2, '500000.00', 'TRANSFER', 'SELF_SUBMITTED', 'PENDING',
		         'Large overpayment for audit test', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		 RETURNING id`,
		rtID, billID,
	).Scan(&paymentID)
	if err != nil {
		t.Fatalf("insert overpayment: %v", err)
	}

	// Reject overpayment.
	{
		body := `{"action":"approve"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+paymentID+"/verify", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected HTTP 400, got %d: %s", rec.Code, rec.Body.String())
		}
	}

	// Check audit log: entity_id must be valid UUID (the payment's actual ID).
	var entityID, newValues string
	err = db.QueryRowContext(ctx,
		`SELECT entity_id, new_values FROM audit_logs
		 WHERE rt_id = $1 AND entity_type = 'payment' AND action = 'verify_payment'
		 ORDER BY created_at DESC LIMIT 1`,
		rtID,
	).Scan(&entityID, &newValues)
	if err != nil {
		t.Fatalf("query audit log: %v", err)
	}
	if entityID == "" {
		t.Error("expected non-empty entity_id in audit log")
	}
	if !strings.Contains(entityID, "-") {
		t.Errorf("expected entity_id to look like UUID, got %q", entityID)
	}
	// Don't truncate before checking for REJECTED — the status field may be beyond 200 chars.
	if !strings.Contains(newValues, "REJECTED") {
		if len(newValues) > 200 {
			newValues = newValues[:200] + "..."
		}
		t.Errorf("expected audit log new_values to contain 'REJECTED', got: %s", newValues)
	}
	if !strings.Contains(newValues, paymentID) {
		t.Errorf("expected audit log new_values to contain payment ID %s", paymentID)
	}
}

// TestOverpaymentRejectPaymentVerifyErrorPropagated verifies that when
// PaymentVerify fails in the overpayment path (e.g., payment row deleted
// concurrently so the UPDATE finds zero rows → sql.ErrNoRows), the error
// is propagated — NOT silently discarded — and the transaction is rolled
// back (no fake audit log, payment stays PENDING or is deleted).
func TestOverpaymentRejectPaymentVerifyErrorPropagated(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID := setupTestRT(t, pool, "Overpay Error RT", 1, "OVP3")
	defer cleanupTestRT(t, pool, rtID)
	cleanupTestPayments(t, pool, rtID)

	phoneW := fmt.Sprintf("0842%08d", time.Now().UnixNano()%100000000)
	_ = setupTestOccupancyWithResident(t, pool, rtID, phoneW)

	phoneB := fmt.Sprintf("0819%08d", time.Now().UnixNano()%100000000)
	userB, midB := setupTestUserWithMembership(t, pool, "b@op3.test", "Bendahara OP3", phoneB, rtID, "bendahara")
	tokenBendahara := makeToken(t, auth.TokenClaims{UserID: userB, MID: midB, RTID: rtID, Role: string(auth.RoleBendahara)})

	// Create due, bill.
	var dueID, billID string
	{
		body := `{"name":"Iuran OVP3","amount":"50000","period_type":"monthly"}`
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
		body := fmt.Sprintf(`{"due_id":"%s","period":"2026-12-ovp3","due_date":"2026-12-25"}`, dueID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("generate bills: %d: %s", rec.Code, rec.Body.String())
		}
	}
	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/bills?period=2026-12-ovp3", nil)
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

	// Insert PENDING payment directly.
	db := pool.Raw()
	ctx := context.Background()
	var paymentID string
	err := db.QueryRowContext(ctx,
		`INSERT INTO payments (rt_id, bill_id, amount, method, origin, status, notes,
		                        created_at, updated_at)
		 VALUES ($1, $2, '50000.00', 'CASH', 'SELF_SUBMITTED', 'PENDING',
		         'Concurrent delete test', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		 RETURNING id`,
		rtID, billID,
	).Scan(&paymentID)
	if err != nil {
		t.Fatalf("insert payment: %v", err)
	}

	// Simulate concurrent deletion: delete payment row so PaymentVerify
	// returns sql.ErrNoRows (wrapped as ErrNotFound) during the verify.
	_, err = db.ExecContext(ctx, `DELETE FROM payments WHERE id = $1`, paymentID)
	if err != nil {
		t.Fatalf("delete payment for error injection: %v", err)
	}

	// Now try to verify — should get 404, NOT 400.
	{
		body := `{"action":"approve"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+paymentID+"/verify", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("expected HTTP 404 (payment not found), got %d: %s", rec.Code, rec.Body.String())
		}
	}

	// Verify: no audit log was created (transaction was rolled back).
	var auditCount int
	err = db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM audit_logs WHERE rt_id = $1 AND action = 'verify_payment'`,
		rtID,
	).Scan(&auditCount)
	if err != nil {
		t.Fatalf("query audit logs: %v", err)
	}
	if auditCount != 0 {
		t.Errorf("expected 0 audit logs (transaction rolled back), got %d", auditCount)
	}
}

// TestOverpaymentRejectWithOfficerNotes verifies that officer_notes provided
// in the reject request are persisted alongside the REJECTED status when
// rejecting an overpayment.
func TestOverpaymentRejectWithOfficerNotes(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID := setupTestRT(t, pool, "Overpay Notes RT", 1, "OVP4")
	defer cleanupTestRT(t, pool, rtID)
	cleanupTestPayments(t, pool, rtID)

	phoneW := fmt.Sprintf("0843%08d", time.Now().UnixNano()%100000000)
	_ = setupTestOccupancyWithResident(t, pool, rtID, phoneW)

	phoneB := fmt.Sprintf("0820%08d", time.Now().UnixNano()%100000000)
	userB, midB := setupTestUserWithMembership(t, pool, "b@op4.test", "Bendahara OP4", phoneB, rtID, "bendahara")
	tokenBendahara := makeToken(t, auth.TokenClaims{UserID: userB, MID: midB, RTID: rtID, Role: string(auth.RoleBendahara)})

	var dueID, billID string
	{
		body := `{"name":"Iuran OVP4","amount":"50000","period_type":"monthly"}`
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
		body := fmt.Sprintf(`{"due_id":"%s","period":"2026-12-ovp4","due_date":"2026-12-25"}`, dueID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("generate bills: %d: %s", rec.Code, rec.Body.String())
		}
	}
	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/bills?period=2026-12-ovp4", nil)
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

	// Insert overpayment via SQL.
	db := pool.Raw()
	ctx := context.Background()
	var paymentID string
	err := db.QueryRowContext(ctx,
		`INSERT INTO payments (rt_id, bill_id, amount, method, origin, status, notes,
		                        created_at, updated_at)
		 VALUES ($1, $2, '200000.00', 'TRANSFER', 'SELF_SUBMITTED', 'PENDING',
		         'Overpayment with notes', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		 RETURNING id`,
		rtID, billID,
	).Scan(&paymentID)
	if err != nil {
		t.Fatalf("insert overpayment: %v", err)
	}

	// Reject overpayment WITH custom rejection_reason and officer_notes.
	{
		body := `{"action":"approve","rejection_reason":"Total iuran sudah penuh","officer_notes":"Jumlah melebihi yang diperlukan"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+paymentID+"/verify", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected HTTP 400, got %d: %s", rec.Code, rec.Body.String())
		}
	}

	// Verify both rejection_reason and officer_notes were persisted.
	var rejectionReason, officerNotes *string
	err = db.QueryRowContext(ctx,
		`SELECT rejection_reason, officer_notes FROM payments WHERE id = $1`,
		paymentID,
	).Scan(&rejectionReason, &officerNotes)
	if err != nil {
		t.Fatalf("query payment fields: %v", err)
	}
	if rejectionReason == nil {
		t.Fatal("expected rejection_reason to be set")
	}
	if *rejectionReason != "Total iuran sudah penuh" {
		t.Errorf("rejection_reason = %q, want %q", *rejectionReason, "Total iuran sudah penuh")
	}
	if officerNotes == nil {
		t.Fatal("expected officer_notes to be set")
	}
	if *officerNotes != "Jumlah melebihi yang diperlukan" {
		t.Errorf("officer_notes = %q, want %q", *officerNotes, "Jumlah melebihi yang diperlukan")
	}
}

// TestOverpaymentRejectDefaultReason verifies that when no rejection_reason
// is provided, the default reason "Payment amount exceeds remaining balance"
// is used.
func TestOverpaymentRejectDefaultReason(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID := setupTestRT(t, pool, "Overpay Default RT", 1, "OVP5")
	defer cleanupTestRT(t, pool, rtID)
	cleanupTestPayments(t, pool, rtID)

	phoneW := fmt.Sprintf("0844%08d", time.Now().UnixNano()%100000000)
	_ = setupTestOccupancyWithResident(t, pool, rtID, phoneW)

	phoneB := fmt.Sprintf("0821%08d", time.Now().UnixNano()%100000000)
	userB, midB := setupTestUserWithMembership(t, pool, "b@op5.test", "Bendahara OP5", phoneB, rtID, "bendahara")
	tokenBendahara := makeToken(t, auth.TokenClaims{UserID: userB, MID: midB, RTID: rtID, Role: string(auth.RoleBendahara)})

	var dueID, billID, paymentID string
	{
		body := `{"name":"Iuran OVP5","amount":"30000","period_type":"monthly"}`
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
		body := fmt.Sprintf(`{"due_id":"%s","period":"2026-12-ovp5","due_date":"2026-12-25"}`, dueID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("generate bills: %d: %s", rec.Code, rec.Body.String())
		}
	}
	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/bills?period=2026-12-ovp5", nil)
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

	// Insert overpayment.
	db := pool.Raw()
	ctx := context.Background()
	err := db.QueryRowContext(ctx,
		`INSERT INTO payments (rt_id, bill_id, amount, method, origin, status,
		                        created_at, updated_at)
		 VALUES ($1, $2, '100000.00', 'CASH', 'SELF_SUBMITTED', 'PENDING',
		         CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		 RETURNING id`,
		rtID, billID,
	).Scan(&paymentID)
	if err != nil {
		t.Fatalf("insert overpayment: %v", err)
	}

	// Reject overpayment WITHOUT providing rejection_reason.
	{
		body := `{"action":"approve"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+paymentID+"/verify", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected HTTP 400, got %d: %s", rec.Code, rec.Body.String())
		}
	}

	// Verify default rejection_reason was used.
	var rejectionReason *string
	err = db.QueryRowContext(ctx,
		`SELECT rejection_reason FROM payments WHERE id = $1`,
		paymentID,
	).Scan(&rejectionReason)
	if err != nil {
		t.Fatalf("query payment: %v", err)
	}
	if rejectionReason == nil {
		t.Fatal("expected rejection_reason to be set")
	}
	if *rejectionReason != "Payment amount exceeds remaining balance" {
		t.Errorf("rejection_reason = %q, want %q", *rejectionReason, "Payment amount exceeds remaining balance")
	}
}

// TestOverpaymentRejectPaymentVerifyNotFound ensures that the error
// wrapping in the overpayment path correctly exposes ErrNotFound so
// the handler returns HTTP 404 instead of propagating a database error.
func TestOverpaymentRejectPaymentVerifyNotFound(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID := setupTestRT(t, pool, "Overpay NF RT", 1, "OVP6")
	defer cleanupTestRT(t, pool, rtID)
	cleanupTestPayments(t, pool, rtID)

	phoneW := fmt.Sprintf("0845%08d", time.Now().UnixNano()%100000000)
	_ = setupTestOccupancyWithResident(t, pool, rtID, phoneW)

	phoneB := fmt.Sprintf("0822%08d", time.Now().UnixNano()%100000000)
	userB, midB := setupTestUserWithMembership(t, pool, "b@op6.test", "Bendahara OP6", phoneB, rtID, "bendahara")
	tokenBendahara := makeToken(t, auth.TokenClaims{UserID: userB, MID: midB, RTID: rtID, Role: string(auth.RoleBendahara)})

	// Create due and bill.
	var dueID, billID string
	{
		body := `{"name":"Iuran OVP6","amount":"50000","period_type":"monthly"}`
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
		body := fmt.Sprintf(`{"due_id":"%s","period":"2026-12-ovp6","due_date":"2026-12-25"}`, dueID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("generate bills: %d: %s", rec.Code, rec.Body.String())
		}
	}
	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/bills?period=2026-12-ovp6", nil)
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

	// Insert and immediately delete a payment.
	db := pool.Raw()
	ctx := context.Background()
	var paymentID string
	err := db.QueryRowContext(ctx,
		`INSERT INTO payments (rt_id, bill_id, amount, method, origin, status,
		                        created_at, updated_at)
		 VALUES ($1, $2, '999999.00', 'CASH', 'SELF_SUBMITTED', 'PENDING',
		         CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		 RETURNING id`,
		rtID, billID,
	).Scan(&paymentID)
	if err != nil {
		t.Fatalf("insert payment: %v", err)
	}

	// Delete BEFORE verification so PaymentVerify finds no rows.
	_, err = db.ExecContext(ctx, `DELETE FROM payments WHERE id = $1`, paymentID)
	if err != nil {
		t.Fatalf("delete payment: %v", err)
	}

	// Verify should return 404, not 500 or 400.
	{
		body := `{"action":"approve"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+paymentID+"/verify", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("expected HTTP 404, got %d: %s", rec.Code, rec.Body.String())
		}
	}
}

// TestOverpaymentRejectPaymentVerifyRowLocked ensures that if PaymentVerify
// itself encounters a real database error (not sql.ErrNoRows), the error
// is wrapped with "reject overpayment:" prefix and propagated.
func TestOverpaymentRejectPaymentVerifyDBError(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID := setupTestRT(t, pool, "Overpay DBErr RT", 1, "OVP7")
	defer cleanupTestRT(t, pool, rtID)
	cleanupTestPayments(t, pool, rtID)

	phoneW := fmt.Sprintf("0846%08d", time.Now().UnixNano()%100000000)
	_ = setupTestOccupancyWithResident(t, pool, rtID, phoneW)

	phoneB := fmt.Sprintf("0823%08d", time.Now().UnixNano()%100000000)
	userB, midB := setupTestUserWithMembership(t, pool, "b@op7.test", "Bendahara OP7", phoneB, rtID, "bendahara")
	tokenBendahara := makeToken(t, auth.TokenClaims{UserID: userB, MID: midB, RTID: rtID, Role: string(auth.RoleBendahara)})

	var dueID, billID string
	{
		body := `{"name":"Iuran OVP7","amount":"50000","period_type":"monthly"}`
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
		body := fmt.Sprintf(`{"due_id":"%s","period":"2026-12-ovp7","due_date":"2026-12-25"}`, dueID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("generate bills: %d: %s", rec.Code, rec.Body.String())
		}
	}
	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/bills?period=2026-12-ovp7", nil)
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

	// Insert a valid overpayment via SQL.
	db := pool.Raw()
	ctx := context.Background()
	var paymentID string
	err := db.QueryRowContext(ctx,
		`INSERT INTO payments (rt_id, bill_id, amount, method, origin, status,
		                        created_at, updated_at)
		 VALUES ($1, $2, '999999.00', 'CASH', 'SELF_SUBMITTED', 'PENDING',
		         CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		 RETURNING id`,
		rtID, billID,
	).Scan(&paymentID)
	if err != nil {
		t.Fatalf("insert overpayment: %v", err)
	}

	// Verify should succeed in rejecting (returns 400 overpayment) and
	// the payment should be REJECTED.
	{
		body := `{"action":"approve"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+paymentID+"/verify", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected HTTP 400, got %d: %s", rec.Code, rec.Body.String())
		}
	}

	// Verify payment is REJECTED.
	var status string
	err = db.QueryRowContext(ctx,
		`SELECT status FROM payments WHERE id = $1`,
		paymentID,
	).Scan(&status)
	if err != nil {
		t.Fatalf("query payment: %v", err)
	}
	if status != string(PaymentStatusRejected) {
		t.Errorf("expected REJECTED, got %s", status)
	}

	// Verify that err wrapping with "reject overpayment:" works correctly
	// by checking that PaymentVerify error propagation uses fmt.Errorf
	// with %w (already verified by TestOverpaymentRejectPaymentVerifyErrorPropagated
	// returning 404 instead of 500 for a deleted payment).
}
