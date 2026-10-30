package finance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"social-finance/internal/auth"
)

func TestVoluntaryDonationLifecycleAndTenantScope(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID := setupTestRT(t, pool, "Donation lifecycle RT", 1, "FDON")
	defer cleanupTestRT(t, pool, rtID)

	otherRTID := setupTestRT(t, pool, "Other donation RT", 1, "FDONOTHER")
	defer cleanupTestRT(t, pool, otherRTID)

	phone := fmt.Sprintf("0812%08d", time.Now().UnixNano()%100000000)
	occupancyID := setupTestOccupancyWithResident(t, pool, rtID, phone)
	wargaID, wargaMembershipID := setupTestUserWithMembership(
		t, pool, "donor@test.com", "Donation Warga", phone, rtID, "warga",
	)
	staffID, staffMembershipID := setupTestUserWithMembership(
		t, pool, "donation.staff@test.com", "Donation Treasurer",
		fmt.Sprintf("0813%08d", time.Now().UnixNano()%100000000), rtID, "bendahara",
	)
	otherID, otherMembershipID := setupTestUserWithMembership(
		t, pool, "other.donor@test.com", "Other RT Warga",
		fmt.Sprintf("0814%08d", time.Now().UnixNano()%100000000), otherRTID, "warga",
	)
	otherStaffID, otherStaffMembershipID := setupTestUserWithMembership(
		t, pool, "other.staff@test.com", "Other RT Treasurer",
		fmt.Sprintf("0815%08d", time.Now().UnixNano()%100000000), otherRTID, "bendahara",
	)

	wargaToken := makeToken(t, auth.TokenClaims{
		UserID: wargaID, MID: wargaMembershipID, RTID: rtID, Role: string(auth.RoleWarga),
	})
	staffToken := makeToken(t, auth.TokenClaims{
		UserID: staffID, MID: staffMembershipID, RTID: rtID, Role: string(auth.RoleBendahara),
	})
	otherToken := makeToken(t, auth.TokenClaims{
		UserID: otherID, MID: otherMembershipID, RTID: otherRTID, Role: string(auth.RoleWarga),
	})
	otherStaffToken := makeToken(t, auth.TokenClaims{
		UserID: otherStaffID, MID: otherStaffMembershipID, RTID: otherRTID, Role: string(auth.RoleBendahara),
	})

	createDue := func(dueRTID, name, amount string, voluntary, active bool) string {
		t.Helper()
		var dueID string
		err := pool.Raw().QueryRowContext(context.Background(),
			`INSERT INTO dues (rt_id, name, amount, period_type, payment_rule, is_voluntary, is_active)
			 VALUES ($1, $2, $3, 'one_time', 'partial', $4, $5) RETURNING id`,
			dueRTID, name, amount, voluntary, active,
		).Scan(&dueID)
		if err != nil {
			t.Fatalf("create test due: %v", err)
		}
		return dueID
	}
	voluntaryDueID := createDue(rtID, "Voluntary Rp0", "0", true, true)
	mandatoryDueID := createDue(rtID, "Mandatory", "0", false, true)
	positiveVoluntaryDueID := createDue(rtID, "Voluntary with positive amount", "1000", true, true)
	inactiveVoluntaryDueID := createDue(rtID, "Inactive voluntary", "0", true, false)
	foreignDueID := createDue(otherRTID, "Other RT voluntary", "0", true, true)

	postPayment := func(token, dueID, amount, idemKey string) *httptest.ResponseRecorder {
		t.Helper()
		body, err := json.Marshal(map[string]string{
			"due_id": dueID, "amount": amount, "method": "TRANSFER",
		})
		if err != nil {
			t.Fatalf("marshal payment body: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		if idemKey != "" {
			req.Header.Set("Idempotency-Key", idemKey)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	postMultipartPayment := func(dueID, amount string) *httptest.ResponseRecorder {
		t.Helper()
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		for key, value := range map[string]string{
			"due_id": dueID, "amount": amount, "method": "TRANSFER",
		} {
			if err := form.WriteField(key, value); err != nil {
				t.Fatalf("write multipart field %s: %v", key, err)
			}
		}
		file, err := form.CreateFormFile("file", "donation.png")
		if err != nil {
			t.Fatalf("create multipart proof file: %v", err)
		}
		if _, err := file.Write([]byte{
			0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
			0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		}); err != nil {
			t.Fatalf("write multipart proof file: %v", err)
		}
		if err := form.Close(); err != nil {
			t.Fatalf("close multipart form: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", &body)
		req.Header.Set("Authorization", "Bearer "+wargaToken)
		req.Header.Set("Content-Type", form.FormDataContentType())
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	verifyPaymentAs := func(token, paymentID, action string) *httptest.ResponseRecorder {
		t.Helper()
		var body string
		if action == "approve" {
			body = fmt.Sprintf(`{"action":%q,"actual_received_date":"2026-10-20"}`, action)
		} else {
			body = fmt.Sprintf(`{"action":%q}`, action)
		}
		req := httptest.NewRequest(
			http.MethodPost, "/api/v1/payments/"+paymentID+"/verify", bytes.NewBufferString(body),
		)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	verifyPayment := func(paymentID, action string) *httptest.ResponseRecorder {
		t.Helper()
		return verifyPaymentAs(staffToken, paymentID, action)
	}
	ledgerCount := func(paymentID string) int {
		t.Helper()
		var count int
		err := pool.Raw().QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM transactions WHERE rt_id = $1 AND payment_id = $2`,
			rtID, paymentID,
		).Scan(&count)
		if err != nil {
			t.Fatalf("count payment ledger entries: %v", err)
		}
		return count
	}

	for _, tt := range []struct {
		name   string
		token  string
		dueID  string
		amount string
	}{
		{name: "zero amount", token: wargaToken, dueID: voluntaryDueID, amount: "0"},
		{name: "negative amount", token: wargaToken, dueID: voluntaryDueID, amount: "-1"},
		{name: "mandatory due", token: wargaToken, dueID: mandatoryDueID, amount: "1000"},
		{name: "positive voluntary due", token: wargaToken, dueID: positiveVoluntaryDueID, amount: "1000"},
		{name: "inactive voluntary due", token: wargaToken, dueID: inactiveVoluntaryDueID, amount: "1000"},
		{name: "foreign RT due", token: otherToken, dueID: voluntaryDueID, amount: "1000"},
		{name: "cross-tenant due", token: wargaToken, dueID: foreignDueID, amount: "1000"},
	} {
		t.Run("rejects "+tt.name, func(t *testing.T) {
			rec := postPayment(tt.token, tt.dueID, tt.amount, "")
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 for %s, got %d: %s", tt.name, rec.Code, rec.Body.String())
			}
		})
	}

	idempotencyKey := fmt.Sprintf("donation-retry-%d", time.Now().UnixNano())
	const concurrentRequests = 8
	start := make(chan struct{})
	responses := make([]*httptest.ResponseRecorder, concurrentRequests)
	var wg sync.WaitGroup
	for i := range responses {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			responses[index] = postPayment(wargaToken, voluntaryDueID, "12500", idempotencyKey)
		}(i)
	}
	close(start)
	wg.Wait()

	first := responses[0]
	if first.Code != http.StatusCreated {
		t.Fatalf("create donation: expected 201, got %d: %s", first.Code, first.Body.String())
	}
	var pending Payment
	if err := json.Unmarshal(first.Body.Bytes(), &pending); err != nil {
		t.Fatalf("decode pending donation: %v", err)
	}
	var pendingJSON map[string]json.RawMessage
	if err := json.Unmarshal(first.Body.Bytes(), &pendingJSON); err != nil {
		t.Fatalf("decode pending donation JSON: %v", err)
	}
	if string(pendingJSON["bill_id"]) != "null" {
		t.Fatalf("donation response bill_id should be null, got %s", pendingJSON["bill_id"])
	}
	if pending.Status != PaymentStatusPending || pending.BillID != nil || pending.DueID == nil || *pending.DueID != voluntaryDueID {
		t.Fatalf("unexpected pending donation target/status: %+v", pending)
	}
	if got := ledgerCount(pending.ID); got != 0 {
		t.Fatalf("pending donation created %d ledger entries, want 0", got)
	}
	if unauthorized := verifyPaymentAs(wargaToken, pending.ID, "approve"); unauthorized.Code != http.StatusForbidden {
		t.Fatalf("warga must not review donations, got %d: %s", unauthorized.Code, unauthorized.Body.String())
	}
	if crossTenant := verifyPaymentAs(otherStaffToken, pending.ID, "approve"); crossTenant.Code != http.StatusNotFound {
		t.Fatalf("other RT staff must not review donation, got %d: %s", crossTenant.Code, crossTenant.Body.String())
	}

	retry := postPayment(wargaToken, voluntaryDueID, "12500", idempotencyKey)
	if retry.Code != http.StatusCreated || retry.Header().Get("X-Cache-Lookup") != "HIT" {
		t.Fatalf("idempotent retry should replay original response, got %d (%s): %s",
			retry.Code, retry.Header().Get("X-Cache-Lookup"), retry.Body.String())
	}
	var retried Payment
	if err := json.Unmarshal(retry.Body.Bytes(), &retried); err != nil {
		t.Fatalf("decode replayed donation: %v", err)
	}
	if retried.ID != pending.ID {
		t.Fatalf("retry created a different payment: first %s, retry %s", pending.ID, retried.ID)
	}
	for i, response := range responses {
		var payment Payment
		if response.Code != http.StatusCreated {
			t.Fatalf("parallel request %d returned %d: %s", i, response.Code, response.Body.String())
		}
		if err := json.Unmarshal(response.Body.Bytes(), &payment); err != nil {
			t.Fatalf("decode parallel response %d: %v", i, err)
		}
		if payment.ID != pending.ID {
			t.Fatalf("parallel request %d returned payment %s, want %s", i, payment.ID, pending.ID)
		}
	}
	conflictingRetry := postPayment(wargaToken, voluntaryDueID, "12501", idempotencyKey)
	if conflictingRetry.Code != http.StatusConflict {
		t.Fatalf("same key with a different payload should return 409, got %d: %s",
			conflictingRetry.Code, conflictingRetry.Body.String())
	}
	var parallelPaymentCount int
	if err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM payments WHERE rt_id = $1 AND due_id = $2`,
		rtID, voluntaryDueID,
	).Scan(&parallelPaymentCount); err != nil {
		t.Fatalf("count payments after parallel retry: %v", err)
	}
	if parallelPaymentCount != 1 {
		t.Fatalf("parallel idempotent requests created %d payments, want exactly 1", parallelPaymentCount)
	}

	detailReq := httptest.NewRequest(http.MethodGet, "/api/v1/payments/"+pending.ID, nil)
	detailReq.Header.Set("Authorization", "Bearer "+wargaToken)
	detailRec := httptest.NewRecorder()
	router.ServeHTTP(detailRec, detailReq)
	if detailRec.Code != http.StatusOK {
		t.Fatalf("warga donation detail: expected 200, got %d: %s", detailRec.Code, detailRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/payments?due_id="+voluntaryDueID, nil)
	listReq.Header.Set("Authorization", "Bearer "+wargaToken)
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("warga donation list: expected 200, got %d: %s", listRec.Code, listRec.Body.String())
	}
	var listed struct {
		Data []Payment `json:"data"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode donation list: %v", err)
	}
	if len(listed.Data) != 1 || listed.Data[0].ID != pending.ID {
		t.Fatalf("warga donation list returned unexpected payments: %+v", listed.Data)
	}

	approved := verifyPayment(pending.ID, "approve")
	if approved.Code != http.StatusOK {
		t.Fatalf("approve donation: expected 200, got %d: %s", approved.Code, approved.Body.String())
	}
	if got := ledgerCount(pending.ID); got != 1 {
		t.Fatalf("approved donation created %d ledger entries, want exactly 1", got)
	}
	if repeatedApproval := verifyPayment(pending.ID, "approve"); repeatedApproval.Code == http.StatusOK {
		t.Fatalf("repeated approval unexpectedly succeeded: %s", repeatedApproval.Body.String())
	}
	if got := ledgerCount(pending.ID); got != 1 {
		t.Fatalf("repeated approval changed ledger entry count to %d, want exactly 1", got)
	}

	concurrentApprovalRec := postPayment(wargaToken, voluntaryDueID, "6000", "")
	if concurrentApprovalRec.Code != http.StatusCreated {
		t.Fatalf("create donation for concurrent approval: %d: %s", concurrentApprovalRec.Code, concurrentApprovalRec.Body.String())
	}
	var concurrentApprovalPayment Payment
	if err := json.Unmarshal(concurrentApprovalRec.Body.Bytes(), &concurrentApprovalPayment); err != nil {
		t.Fatalf("decode donation for concurrent approval: %v", err)
	}
	runConcurrentVerifications := func(paymentID string, actions ...string) []*httptest.ResponseRecorder {
		t.Helper()
		start := make(chan struct{})
		recs := make([]*httptest.ResponseRecorder, len(actions))
		var verifyWG sync.WaitGroup
		for i, action := range actions {
			verifyWG.Add(1)
			go func(index int, action string) {
				defer verifyWG.Done()
				<-start
				recs[index] = verifyPayment(paymentID, action)
			}(i, action)
		}
		close(start)
		verifyWG.Wait()
		return recs
	}
	approvalResults := runConcurrentVerifications(concurrentApprovalPayment.ID, "approve", "approve")
	approvalSuccesses := 0
	for _, result := range approvalResults {
		if result.Code == http.StatusOK {
			approvalSuccesses++
		}
	}
	if approvalSuccesses != 1 || ledgerCount(concurrentApprovalPayment.ID) != 1 {
		t.Fatalf("parallel approvals had %d successes and %d ledger entries; want 1 and 1",
			approvalSuccesses, ledgerCount(concurrentApprovalPayment.ID))
	}

	racingDecisionRec := postPayment(wargaToken, voluntaryDueID, "7000", "")
	if racingDecisionRec.Code != http.StatusCreated {
		t.Fatalf("create donation for concurrent approve/reject: %d: %s", racingDecisionRec.Code, racingDecisionRec.Body.String())
	}
	var racingDecisionPayment Payment
	if err := json.Unmarshal(racingDecisionRec.Body.Bytes(), &racingDecisionPayment); err != nil {
		t.Fatalf("decode donation for concurrent approve/reject: %v", err)
	}
	decisionResults := runConcurrentVerifications(racingDecisionPayment.ID, "approve", "reject")
	decisionSuccesses := 0
	for _, result := range decisionResults {
		if result.Code == http.StatusOK {
			decisionSuccesses++
		}
	}
	if decisionSuccesses != 1 {
		t.Fatalf("parallel approve/reject had %d successful decisions, want exactly 1", decisionSuccesses)
	}
	var decisionStatus PaymentStatus
	if err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT status FROM payments WHERE id = $1`, racingDecisionPayment.ID,
	).Scan(&decisionStatus); err != nil {
		t.Fatalf("read concurrent decision result: %v", err)
	}
	wantLedger := 0
	if decisionStatus == PaymentStatusApproved {
		wantLedger = 1
	} else if decisionStatus != PaymentStatusRejected {
		t.Fatalf("unexpected concurrent decision status: %s", decisionStatus)
	}
	if got := ledgerCount(racingDecisionPayment.ID); got != wantLedger {
		t.Fatalf("concurrent approve/reject status %s has %d ledger entries, want %d", decisionStatus, got, wantLedger)
	}

	rejectedRec := postMultipartPayment(voluntaryDueID, "3000")
	if rejectedRec.Code != http.StatusCreated {
		t.Fatalf("create donation for rejection: expected 201, got %d: %s", rejectedRec.Code, rejectedRec.Body.String())
	}
	var rejectedCandidate Payment
	if err := json.Unmarshal(rejectedRec.Body.Bytes(), &rejectedCandidate); err != nil {
		t.Fatalf("decode donation for rejection: %v", err)
	}
	if rejectedCandidate.DueID == nil || *rejectedCandidate.DueID != voluntaryDueID ||
		rejectedCandidate.BillID != nil || rejectedCandidate.ProofPath == nil {
		t.Fatalf("multipart donation was not linked correctly: %+v", rejectedCandidate)
	}
	rejected := verifyPayment(rejectedCandidate.ID, "reject")
	if rejected.Code != http.StatusOK {
		t.Fatalf("reject donation: expected 200, got %d: %s", rejected.Code, rejected.Body.String())
	}
	if got := ledgerCount(rejectedCandidate.ID); got != 0 {
		t.Fatalf("rejected donation created %d ledger entries, want 0", got)
	}

	var paymentCount int
	if err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM payments WHERE rt_id = $1 AND due_id = $2`,
		rtID, voluntaryDueID,
	).Scan(&paymentCount); err != nil {
		t.Fatalf("count donations after idempotent retry: %v", err)
	}
	if paymentCount != 4 {
		t.Fatalf("expected four donations after concurrent decisions, got %d", paymentCount)
	}

	billTx, err := pool.BeginTx(context.Background())
	if err != nil {
		t.Fatalf("begin ordinary bill transaction: %v", err)
	}
	ordinaryDueID := createDue(rtID, "Ordinary required due", "10000", false, true)
	ordinaryBill, err := NewService(pool).CreateBill(context.Background(), billTx, rtID, staffID, CreateBillInput{
		HouseholdOccupancyID: occupancyID,
		DueID:                ordinaryDueID,
		Amount:               "10000",
		Period:               "2026-10",
		DueDate:              "2026-10-31",
	})
	if err != nil {
		_ = billTx.Rollback()
		t.Fatalf("create ordinary bill on migration 020 schema: %v", err)
	}
	if err := billTx.Commit(); err != nil {
		t.Fatalf("commit ordinary bill: %v", err)
	}
	ordinaryPaymentRec := postOrdinaryBillPayment(t, router, wargaToken, ordinaryBill.ID, "4000")
	if ordinaryPaymentRec.Code != http.StatusCreated {
		t.Fatalf("ordinary payment after migration 020: expected 201, got %d: %s",
			ordinaryPaymentRec.Code, ordinaryPaymentRec.Body.String())
	}
	var ordinaryPayment Payment
	if err := json.Unmarshal(ordinaryPaymentRec.Body.Bytes(), &ordinaryPayment); err != nil {
		t.Fatalf("decode ordinary payment: %v", err)
	}
	if ordinaryPayment.BillID == nil || *ordinaryPayment.BillID != ordinaryBill.ID || ordinaryPayment.DueID != nil {
		t.Fatalf("ordinary bill payment has wrong target after migration 020: %+v", ordinaryPayment)
	}
	if rec := verifyPayment(ordinaryPayment.ID, "approve"); rec.Code != http.StatusOK {
		t.Fatalf("approve ordinary bill payment after migration 020: %d: %s", rec.Code, rec.Body.String())
	}

	reportReq := httptest.NewRequest(http.MethodGet, "/api/v1/reports/balance", nil)
	reportReq.Header.Set("Authorization", "Bearer "+staffToken)
	reportRec := httptest.NewRecorder()
	router.ServeHTTP(reportRec, reportReq)
	if reportRec.Code != http.StatusOK {
		t.Fatalf("balance report with NULL bill_id donations: %d: %s", reportRec.Code, reportRec.Body.String())
	}
	var report BalanceSummary
	if err := json.Unmarshal(reportRec.Body.Bytes(), &report); err != nil {
		t.Fatalf("decode balance report: %v", err)
	}
	var ledgerIncome string
	if err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT COALESCE(SUM(amount), 0)::text FROM transactions
		 WHERE rt_id = $1 AND type = 'income' AND status = 'posted'`,
		rtID,
	).Scan(&ledgerIncome); err != nil {
		t.Fatalf("sum posted income for balance report: %v", err)
	}
	if report.TotalIncome != ledgerIncome {
		t.Fatalf("balance report total income %s differs from ledger %s", report.TotalIncome, ledgerIncome)
	}

	downSQL, err := os.ReadFile(filepath.Join("..", "..", "migrations", "020_payments_donations.down.sql"))
	if err != nil {
		t.Fatalf("read migration 020 down script: %v", err)
	}
	sqlText := string(downSQL)
	if strings.Index(sqlText, "IF null_count > 0") > strings.Index(sqlText, "DROP INDEX IF EXISTS idx_payments_due_id") {
		t.Fatal("rollback must check for bill-less donation rows before destructive DDL")
	}
	rollbackTx, err := pool.Raw().BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin rollback simulation: %v", err)
	}
	if _, err := rollbackTx.ExecContext(context.Background(), sqlText); err == nil || !strings.Contains(err.Error(), "Cannot restore bill_id NOT NULL") {
		_ = rollbackTx.Rollback()
		t.Fatalf("rollback with donations should refuse before schema changes, got %v", err)
	}
	_ = rollbackTx.Rollback()
	var donationColumns int
	if err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM information_schema.columns
		 WHERE table_schema = current_schema() AND table_name = 'payments'
		   AND column_name IN ('due_id', 'submitted_by')`,
	).Scan(&donationColumns); err != nil {
		t.Fatalf("check donation columns after rollback simulation: %v", err)
	}
	if donationColumns != 2 {
		t.Fatalf("rollback simulation changed schema; found %d donation columns, want 2", donationColumns)
	}

	// ── Cross-Citizen Isolation Test ──
	// Create citizens B and C in the same RT to verify A's donation is not visible when they query by due_id.

	phoneB := fmt.Sprintf("0816%08d", time.Now().UnixNano()%100000000)
	_ = setupTestOccupancyWithResident(t, pool, rtID, phoneB)
	citizenBID, citizenBMID := setupTestUserWithMembership(
		t, pool, "citizen.b@test.com", "Citizen B", phoneB, rtID, "warga",
	)
	citizenBToken := makeToken(t, auth.TokenClaims{
		UserID: citizenBID, MID: citizenBMID, RTID: rtID, Role: string(auth.RoleWarga),
	})

	phoneC := fmt.Sprintf("0817%08d", time.Now().UnixNano()%100000000)
	_ = setupTestOccupancyWithResident(t, pool, rtID, phoneC)
	citizenCID, citizenCMID := setupTestUserWithMembership(
		t, pool, "citizen.c@test.com", "Citizen C", phoneC, rtID, "warga",
	)
	citizenCToken := makeToken(t, auth.TokenClaims{
		UserID: citizenCID, MID: citizenCMID, RTID: rtID, Role: string(auth.RoleWarga),
	})

	// B queries voluntary payments — must NOT see A's donation.
	bListReq := httptest.NewRequest(http.MethodGet, "/api/v1/payments?due_id="+voluntaryDueID, nil)
	bListReq.Header.Set("Authorization", "Bearer "+citizenBToken)
	bListRec := httptest.NewRecorder()
	router.ServeHTTP(bListRec, bListReq)
	if bListRec.Code != http.StatusOK {
		t.Fatalf("citizen B donation list: expected 200, got %d: %s", bListRec.Code, bListRec.Body.String())
	}
	var bListed struct {
		Data []Payment `json:"data"`
	}
	if err := json.Unmarshal(bListRec.Body.Bytes(), &bListed); err != nil {
		t.Fatalf("decode citizen B donation list: %v", err)
	}
	if len(bListed.Data) != 0 {
		t.Fatalf("citizen B donation list returned %d payments (should be 0 — A's donation not visible): %+v", len(bListed.Data), bListed.Data)
	}

	// C queries voluntary payments — must NOT see A's donation.
	cListReq := httptest.NewRequest(http.MethodGet, "/api/v1/payments?due_id="+voluntaryDueID, nil)
	cListReq.Header.Set("Authorization", "Bearer "+citizenCToken)
	cListRec := httptest.NewRecorder()
	router.ServeHTTP(cListRec, cListReq)
	if cListRec.Code != http.StatusOK {
		t.Fatalf("citizen C donation list: expected 200, got %d: %s", cListRec.Code, cListRec.Body.String())
	}
	var cListed struct {
		Data []Payment `json:"data"`
	}
	if err := json.Unmarshal(cListRec.Body.Bytes(), &cListed); err != nil {
		t.Fatalf("decode citizen C donation list: %v", err)
	}
	if len(cListed.Data) != 0 {
		t.Fatalf("citizen C donation list returned %d payments (should be 0 — A's donation not visible): %+v", len(cListed.Data), cListed.Data)
	}

	// Staff CAN see A's donation for verification.
	staffListReq := httptest.NewRequest(http.MethodGet, "/api/v1/payments?due_id="+voluntaryDueID, nil)
	staffListReq.Header.Set("Authorization", "Bearer "+staffToken)
	staffListRec := httptest.NewRecorder()
	router.ServeHTTP(staffListRec, staffListReq)
	if staffListRec.Code != http.StatusOK {
		t.Fatalf("staff donation list: expected 200, got %d: %s", staffListRec.Code, staffListRec.Body.String())
	}
	var staffListed struct {
		Data []Payment `json:"data"`
	}
	if err := json.Unmarshal(staffListRec.Body.Bytes(), &staffListed); err != nil {
		t.Fatalf("decode staff donation list: %v", err)
	}
	if len(staffListed.Data) == 0 {
		t.Fatalf("staff donation list returned 0 payments (should see A's pending donation)")
	}
	// Verify pending payment is in the staff list.
	found := false
	for _, p := range staffListed.Data {
		if p.ID == pending.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("staff donation list does not contain A's pending donation (ID=%s): %+v", pending.ID, staffListed.Data)
	}

	// ── End of Cross-Citizen Isolation Test ──
}

func postOrdinaryBillPayment(t *testing.T, router http.Handler, token, billID, amount string) *httptest.ResponseRecorder {
	t.Helper()
	body := fmt.Sprintf(`{"bill_id":%q,"amount":%q,"method":"TRANSFER"}`, billID, amount)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}
