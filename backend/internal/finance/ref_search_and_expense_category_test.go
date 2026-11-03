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

	"github.com/stretchr/testify/require"
	"social-finance/internal/auth"
)

func TestRefSearch_And_ExpenseCategory(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID := setupTestRT(t, pool, "RT Ref Search", 1, "RTRF")
	defer cleanupTestRT(t, pool, rtID)

	phoneB := fmt.Sprintf("0811%08d", time.Now().UnixNano()%100000000)
	userB, midB := setupTestUserWithMembership(t, pool, "benrf@test.com", "Bendahara RF", phoneB, rtID, "bendahara")
	tokenBendahara := makeToken(t, auth.TokenClaims{UserID: userB, MID: midB, RTID: rtID, Role: string(auth.RoleBendahara)})

	ctx := context.Background()
	locID := setupTestBankLocation(t, pool, rtID, "Kas Bank RF")

	var catID string
	err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO financial_categories (rt_id, name, type) VALUES ($1, 'Operasional Kantor', 'expense') RETURNING id`,
		rtID,
	).Scan(&catID)
	require.NoError(t, err)

	// 1. Test Expense Category in Create Manual Transaction
	bodyTx := fmt.Sprintf(`{
		"category_id": "%s",
		"amount": "150000.00",
		"type": "expense",
		"description": "Beli Kertas HVS dan Spidol",
		"payment_method": "CASH",
		"expense_category": "ATK"
	}`, catID)
	reqTx := httptest.NewRequest(http.MethodPost, "/api/v1/transactions", bytes.NewBufferString(bodyTx))
	reqTx.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqTx.Header.Set("Content-Type", "application/json")
	wTx := httptest.NewRecorder()
	router.ServeHTTP(wTx, reqTx)
	require.Equal(t, http.StatusCreated, wTx.Code, wTx.Body.String())

	var txResp struct {
		ID              string  `json:"id"`
		ExpenseCategory *string `json:"expense_category"`
		ReferenceNo     *string `json:"reference_no"`
	}
	err = json.Unmarshal(wTx.Body.Bytes(), &txResp)
	require.NoError(t, err)
	require.NotNil(t, txResp.ExpenseCategory)
	require.Equal(t, "ATK", *txResp.ExpenseCategory)
	require.NotNil(t, txResp.ReferenceNo)
	refNo := *txResp.ReferenceNo

	// 2. Test Transaction Search by Reference No
	reqSearchTx := httptest.NewRequest(http.MethodGet, "/api/v1/transactions?search="+refNo, nil)
	reqSearchTx.Header.Set("Authorization", "Bearer "+tokenBendahara)
	wSearchTx := httptest.NewRecorder()
	router.ServeHTTP(wSearchTx, reqSearchTx)
	require.Equal(t, http.StatusOK, wSearchTx.Code)

	var searchTxResp struct {
		Data []struct {
			ID              string  `json:"id"`
			ReferenceNo     *string `json:"reference_no"`
			ExpenseCategory *string `json:"expense_category"`
		} `json:"data"`
	}
	err = json.Unmarshal(wSearchTx.Body.Bytes(), &searchTxResp)
	require.NoError(t, err)
	require.NotEmpty(t, searchTxResp.Data)
	require.Equal(t, txResp.ID, searchTxResp.Data[0].ID)
	require.Equal(t, "ATK", *searchTxResp.Data[0].ExpenseCategory)

	// 3. Test Multi-Month Payment and Search Bill by Payment No
	bodyDue := `{"name":"IPL 2026 RF","amount":"100000","due_type":"multi_month","year":2026,"due_date":"2026-01-10"}`
	reqDue := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(bodyDue))
	reqDue.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqDue.Header.Set("Content-Type", "application/json")
	wDue := httptest.NewRecorder()
	router.ServeHTTP(wDue, reqDue)
	require.Equal(t, http.StatusCreated, wDue.Code)
	var dueResp struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(wDue.Body.Bytes(), &dueResp)

	phoneW := fmt.Sprintf("0812%08d", time.Now().UnixNano()%100000000)
	userW, midW := setupTestUserWithMembership(t, pool, "wargarf@test.com", "Warga RF", phoneW, rtID, "warga")
	tokenWarga := makeToken(t, auth.TokenClaims{UserID: userW, MID: midW, RTID: rtID, Role: string(auth.RoleWarga)})

	var hhID, houseID, occID string
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO households (rt_id, head_name) VALUES ($1, 'Warga RF Head') RETURNING id`,
		rtID).Scan(&hhID)
	require.NoError(t, err)

	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO physical_houses (rt_id, house_number) VALUES ($1, 'House RF-01') RETURNING id`,
		rtID).Scan(&houseID)
	require.NoError(t, err)

	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO household_occupancies (household_id, physical_house_id, occupancy_status, start_date)
		 VALUES ($1, $2, 'OWNER', '2026-01-01') RETURNING id`,
		hhID, houseID).Scan(&occID)
	require.NoError(t, err)

	var residentID string
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO residents (rt_id, full_name, phone, user_id, is_active)
		 VALUES ($1, 'Warga RF', $2, $3, true) RETURNING id`,
		rtID, phoneW, userW).Scan(&residentID)
	require.NoError(t, err)

	_, err = pool.Raw().ExecContext(ctx,
		`INSERT INTO residency_periods (resident_id, household_occupancy_id, start_date)
		 VALUES ($1, $2, '2026-01-01')`,
		residentID, occID)
	require.NoError(t, err)

	// Sync bills
	tx, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	_, err = SyncHouseholdOccupancyBills(ctx, tx, rtID, occID)
	require.NoError(t, err)
	err = tx.Commit()
	require.NoError(t, err)

	// Fetch bills for occupancy
	tx, err = pool.BeginTx(ctx)
	require.NoError(t, err)
	bills, err := BillList(ctx, tx, rtID, &occID, &dueResp.ID, nil, nil, nil, nil, nil, nil, nil, 0, 12, "period_asc")
	require.NoError(t, err)
	_ = tx.Commit()
	require.True(t, len(bills) >= 2)

	billJanID := bills[0].ID
	billFebID := bills[1].ID

	// Create multi-month payment covering Jan and Feb (200,000)
	bodyPay := fmt.Sprintf(`{
		"bill_ids": ["%s", "%s"],
		"allocations": [
			{"bill_id": "%s", "amount": "100000.00"},
			{"bill_id": "%s", "amount": "100000.00"}
		],
		"amount": "200000.00",
		"method": "TRANSFER",
		"proof_path": "/uploads/proof-rf.jpg",
		"proof_original_name": "proof.jpg",
		"proof_mime_type": "image/jpeg",
		"proof_size_bytes": 1024,
		"notes": "Bayar Jan dan Feb"
	}`, billJanID, billFebID, billJanID, billFebID)

	reqPay := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(bodyPay))
	reqPay.Header.Set("Authorization", "Bearer "+tokenWarga)
	reqPay.Header.Set("Content-Type", "application/json")
	wPay := httptest.NewRecorder()
	router.ServeHTTP(wPay, reqPay)
	require.Equal(t, http.StatusCreated, wPay.Code, wPay.Body.String())

	var payResp struct {
		Payment struct {
			ID string `json:"id"`
		} `json:"payment"`
	}
	err = json.Unmarshal(wPay.Body.Bytes(), &payResp)
	require.NoError(t, err)

	// Approve payment
	bodyApprove := fmt.Sprintf(`{"action":"approve","location_id":"%s","actual_received_date":"2026-01-15"}`, locID)
	reqApprove := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/payments/%s/verify", payResp.Payment.ID), bytes.NewBufferString(bodyApprove))
	reqApprove.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqApprove.Header.Set("Content-Type", "application/json")
	wApprove := httptest.NewRecorder()
	router.ServeHTTP(wApprove, reqApprove)
	require.Equal(t, http.StatusOK, wApprove.Code, wApprove.Body.String())

	var approvedPay Payment
	err = json.Unmarshal(wApprove.Body.Bytes(), &approvedPay)
	require.NoError(t, err)
	require.NotNil(t, approvedPay.PaymentNo)
	paymentNo := *approvedPay.PaymentNo

	// Verify PaymentList for February bill returns the master payment!
	tx, err = pool.BeginTx(ctx)
	require.NoError(t, err)
	febPayments, err := PaymentList(ctx, tx, rtID, &billFebID, nil, nil, nil, nil, 0, 10)
	require.NoError(t, err)
	_ = tx.Commit()
	require.NotEmpty(t, febPayments, "PaymentList for February bill must return the multi-period payment")
	require.Equal(t, payResp.Payment.ID, febPayments[0].ID)

	// Verify BillList search by paymentNo finds the bills!
	tx, err = pool.BeginTx(ctx)
	require.NoError(t, err)
	searchBills, err := BillList(ctx, tx, rtID, nil, nil, nil, nil, nil, &paymentNo, nil, nil, nil, 0, 10)
	require.NoError(t, err)
	_ = tx.Commit()
	require.True(t, len(searchBills) >= 2, "Searching by payment_no must return all bills settled by that payment")
}
