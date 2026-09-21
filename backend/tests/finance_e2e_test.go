package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

func jsonReader(v any) io.Reader {
	b, _ := json.Marshal(v)
	return bytes.NewReader(b)
}

func TestFinance_E2E(t *testing.T) {
	assertTestBackendTarget(t)
	tokenPengurusA := login(t, emailPengurusA, testPassword)
	tokenBendaharaA := login(t, emailBendaharaA, testPassword)
	tokenWargaB := login(t, emailWargaB, testPassword)
	tokenPengurusC := login(t, emailPengurusC, testPassword)

	timestamp := time.Now().UnixNano()
	catName := fmt.Sprintf("Fin-Cat-%d", timestamp%100000)
	dueName := fmt.Sprintf("Fin-Due-%d", timestamp%100000)
	period := fmt.Sprintf("2026-%02d", (timestamp%12)+1)

	var catID string
	var dueID string
	var billID string
	var transID string

	// F2: Categories CRUD & Role Matrix
	t.Run("F2_Categories_CRUD_And_Roles", func(t *testing.T) {
		// Pengurus A creates category
		payload := map[string]any{
			"name": catName,
			"type": "income",
		}
		resp, body := doRequest(t, http.MethodPost, "/api/v1/categories", tokenPengurusA, payload)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("create category: %d: %s", resp.StatusCode, string(body))
		}
		var cat map[string]any
		json.Unmarshal(body, &cat)
		catID = cat["id"].(string)

		// Warga cannot create category (403)
		wResp, wBody := doRequest(t, http.MethodPost, "/api/v1/categories", tokenWargaB, payload)
		if wResp.StatusCode != http.StatusForbidden {
			t.Errorf("expected 403 for warga create category, got %d: %s", wResp.StatusCode, string(wBody))
		}

		// Bendahara A cannot delete category (403)
		bResp, bBody := doRequest(t, http.MethodDelete, "/api/v1/categories/"+catID, tokenBendaharaA, nil)
		if bResp.StatusCode != http.StatusForbidden {
			t.Errorf("expected 403 for bendahara delete category, got %d: %s", bResp.StatusCode, string(bBody))
		}
	})

	// F3: Dues CRUD & Validation
	t.Run("F3_Dues_CRUD_And_Validation", func(t *testing.T) {
		// Bendahara creates valid due
		payload := map[string]any{
			"name":        dueName,
			"amount":      "75000",
			"period_type": "monthly",
		}
		resp, body := doRequest(t, http.MethodPost, "/api/v1/dues", tokenBendaharaA, payload)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("create due: %d: %s", resp.StatusCode, string(body))
		}
		var due map[string]any
		json.Unmarshal(body, &due)
		dueID = due["id"].(string)
		if due["amount"] != "75000.00" {
			t.Errorf("expected amount 75000.00, got %v", due["amount"])
		}

		// Negative amount rejected
		negPayload := map[string]any{
			"name":        "Negative Due",
			"amount":      "-5000",
			"period_type": "monthly",
		}
		negResp, _ := doRequest(t, http.MethodPost, "/api/v1/dues", tokenBendaharaA, negPayload)
		if negResp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400 for negative due amount, got %d", negResp.StatusCode)
		}
	})

	// F4: Bills Generation
	t.Run("F4_Bills_Generation", func(t *testing.T) {
		payload := map[string]any{
			"due_id":   dueID,
			"period":   period,
			"due_date": "2026-10-15",
		}
		resp, body := doRequest(t, http.MethodPost, "/api/v1/bills/generate", tokenBendaharaA, payload)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("generate bills: %d: %s", resp.StatusCode, string(body))
		}

		// List bills
		listResp, listBody := doRequest(t, http.MethodGet, "/api/v1/bills?period="+period, tokenBendaharaA, nil)
		if listResp.StatusCode != http.StatusOK {
			t.Fatalf("list bills: %d: %s", listResp.StatusCode, string(listBody))
		}
		var bl struct {
			Data []map[string]any `json:"data"`
		}
		json.Unmarshal(listBody, &bl)
		if len(bl.Data) == 0 {
			t.Fatalf("expected at least 1 generated bill for period %s", period)
		}
		billID = bl.Data[0]["id"].(string)
	})

	// F5: Payment Creation & Verification
	t.Run("F5_Payment_Creation_And_Verification", func(t *testing.T) {
		// Staff recorded payment by Bendahara A
		payload := map[string]any{
			"bill_id": billID,
			"amount":  "75000",
			"method":  "CASH",
			"notes":   "Cash received by treasurer",
		}
		resp, body := doRequest(t, http.MethodPost, "/api/v1/payments", tokenBendaharaA, payload)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("staff payment create: %d: %s", resp.StatusCode, string(body))
		}
		var p map[string]any
		json.Unmarshal(body, &p)
		paymentID := p["id"].(string)
		_ = paymentID
		if p["status"] != "APPROVED" {
			t.Errorf("expected staff-recorded payment to be APPROVED, got %v", p["status"])
		}

		// Verify bill is now paid
		bResp, bBody := doRequest(t, http.MethodGet, "/api/v1/bills/"+billID, tokenBendaharaA, nil)
		if bResp.StatusCode != http.StatusOK {
			t.Fatalf("get bill: %d: %s", bResp.StatusCode, string(bBody))
		}
		var bill map[string]any
		json.Unmarshal(bBody, &bill)
		if bill["status"] != "paid" {
			t.Errorf("expected bill status paid, got %v", bill["status"])
		}
	})

	// F6 & F7: Ledger, Real-time Derived Balance, and Reversal
	t.Run("F6_F7_Ledger_Balance_And_Reversal", func(t *testing.T) {
		// Check ledger contains posted payment transaction
		lResp, lBody := doRequest(t, http.MethodGet, "/api/v1/transactions", tokenBendaharaA, nil)
		if lResp.StatusCode != http.StatusOK {
			t.Fatalf("list transactions: %d: %s", lResp.StatusCode, string(lBody))
		}
		var tl struct {
			Data []map[string]any `json:"data"`
		}
		json.Unmarshal(lBody, &tl)
		if len(tl.Data) == 0 {
			t.Fatalf("expected at least 1 ledger transaction")
		}
		transID = tl.Data[0]["id"].(string)

		// Check derived balance
		balResp, balBody := doRequest(t, http.MethodGet, "/api/v1/reports/balance", tokenBendaharaA, nil)
		if balResp.StatusCode != http.StatusOK {
			t.Fatalf("get balance: %d: %s", balResp.StatusCode, string(balBody))
		}
		var bal map[string]any
		json.Unmarshal(balBody, &bal)
		if bal["total_income"] == "0.00" {
			t.Errorf("expected positive total_income, got %v", bal["total_income"])
		}

		// Reversal of transaction
		revPayload := map[string]any{"reason": "E2E Reversal Test"}
		revResp, revBody := doRequest(t, http.MethodPost, "/api/v1/transactions/"+transID+"/reverse", tokenBendaharaA, revPayload)
		if revResp.StatusCode != http.StatusOK {
			t.Fatalf("reverse transaction: %d: %s", revResp.StatusCode, string(revBody))
		}
		var rev map[string]any
		json.Unmarshal(revBody, &rev)
		if rev["reverses_transaction_id"] != transID {
			t.Errorf("expected reverses_transaction_id %s, got %v", transID, rev["reverses_transaction_id"])
		}
	})

	// F8: Idempotency Key Replay
	t.Run("F8_Idempotency_Key_Replay", func(t *testing.T) {
		idemKey := fmt.Sprintf("e2e-idem-%d", time.Now().UnixNano())
		manualPayload := map[string]any{
			"category_id": catID,
			"amount":      "30000",
			"type":        "income",
			"description": "Donation test",
		}

		// First call with Idempotency-Key
		req1, _ := http.NewRequest(http.MethodPost, baseURL+"/api/v1/transactions", jsonReader(manualPayload))
		req1.Header.Set("Authorization", "Bearer "+tokenBendaharaA)
		req1.Header.Set("Content-Type", "application/json")
		req1.Header.Set("Idempotency-Key", idemKey)

		client := &http.Client{Timeout: 10 * time.Second}
		resp1, err := client.Do(req1)
		if err != nil || resp1.StatusCode != http.StatusCreated {
			t.Fatalf("first idempotent call failed: status %d, err %v", resp1.StatusCode, err)
		}

		// Second call with same Idempotency-Key
		req2, _ := http.NewRequest(http.MethodPost, baseURL+"/api/v1/transactions", jsonReader(manualPayload))
		req2.Header.Set("Authorization", "Bearer "+tokenBendaharaA)
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("Idempotency-Key", idemKey)

		resp2, err := client.Do(req2)
		if err != nil || resp2.StatusCode != http.StatusCreated {
			t.Fatalf("replayed idempotent call failed: status %d, err %v", resp2.StatusCode, err)
		}
		if resp2.Header.Get("X-Cache-Lookup") != "HIT" {
			t.Errorf("expected X-Cache-Lookup: HIT on replay, got %s", resp2.Header.Get("X-Cache-Lookup"))
		}
	})

	// F10: Tenant Isolation (Cross-RT)
	t.Run("F10_Cross_RT_Tenant_Isolation", func(t *testing.T) {
		// Pengurus C (in RT C) cannot read or update RT A's due
		resp, body := doRequest(t, http.MethodPatch, "/api/v1/dues/"+dueID, tokenPengurusC, map[string]any{"name": "Hacked Due"})
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected 404 for cross-RT due update, got %d: %s", resp.StatusCode, string(body))
		}
	})
}
