package finance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"social-finance/internal/auth"
)

func TestPaymentProofMandatory(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, tokenBendahara, _ := setupValidationRT(t, pool)
	defer cleanupTestRT(t, pool, rtID)

	ctx := context.Background()

	// Setup household and occupancy
	var hhID, houseID, occID string
	err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO households (rt_id, head_name) VALUES ($1, 'Keluarga Bukti') RETURNING id`, rtID).Scan(&hhID)
	require.NoError(t, err)
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO physical_houses (rt_id, house_number) VALUES ($1, 'B-01') RETURNING id`, rtID).Scan(&houseID)
	require.NoError(t, err)
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO household_occupancies (household_id, physical_house_id, occupancy_status, start_date)
		 VALUES ($1, $2, 'OWNER', '2025-01-01') RETURNING id`, hhID, houseID).Scan(&occID)
	require.NoError(t, err)

	// Setup Warga User & Link to Household
	phoneW := fmt.Sprintf("081%09d", time.Now().UnixNano()%1000000000)
	userIDW, midW := setupTestUserWithMembership(t, pool, "warga-bukti@test.com", "Warga Bukti", phoneW, rtID, "warga")
	tokenWarga := makeToken(t, auth.TokenClaims{UserID: userIDW, MID: midW, RTID: rtID, Role: string(auth.RoleWarga)})

	var resID string
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO residents (rt_id, user_id, full_name, phone, is_active) VALUES ($1, $2, 'Warga Bukti', $3, true) RETURNING id`,
		rtID, userIDW, phoneW).Scan(&resID)
	require.NoError(t, err)
	_, err = pool.Raw().ExecContext(ctx,
		`INSERT INTO residency_periods (resident_id, household_occupancy_id, start_date) VALUES ($1, $2, CURRENT_DATE)`,
		resID, occID)
	require.NoError(t, err)

	// Create a regular due & bill
	bodyDue := `{"name":"Iuran Wajib Bukti","amount":"50000","period_type":"monthly","due_type":"regular"}`
	reqDue := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(bodyDue))
	reqDue.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqDue.Header.Set("Content-Type", "application/json")
	recDue := httptest.NewRecorder()
	router.ServeHTTP(recDue, reqDue)
	require.Equal(t, http.StatusCreated, recDue.Code)
	var due Due
	require.NoError(t, json.Unmarshal(recDue.Body.Bytes(), &due))

	// Create bill directly
	tx, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	bill, err := BillCreate(ctx, tx, rtID, CreateBillInput{
		HouseholdOccupancyID: occID,
		DueID:                due.ID,
		Amount:               "50000",
		Period:               "2026-05",
		DueDate:              "2026-05-10",
	})
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	billID := bill.ID

	const expectedErrMsg = "Bukti pembayaran wajib dilampirkan. Silakan unggah foto bukti pembayaran sebelum melanjutkan."

	t.Run("JSON CreatePayment without proof_path is rejected", func(t *testing.T) {
		body := fmt.Sprintf(`{"bill_id":"%s","amount":"50000","method":"TRANSFER"}`, billID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Contains(t, rec.Body.String(), expectedErrMsg)
	})

	t.Run("JSON CreatePayment with empty proof_path is rejected", func(t *testing.T) {
		body := fmt.Sprintf(`{"bill_id":"%s","amount":"50000","method":"TRANSFER","proof_path":"   "}`, billID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Contains(t, rec.Body.String(), expectedErrMsg)
	})

	t.Run("Multipart CreatePayment without file is rejected", func(t *testing.T) {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		_ = mw.WriteField("bill_id", billID)
		_ = mw.WriteField("amount", "50000")
		_ = mw.WriteField("method", "TRANSFER")
		_ = mw.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", &buf)
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Contains(t, rec.Body.String(), expectedErrMsg)
	})

	t.Run("JSON CreatePayment with valid proof_path is accepted", func(t *testing.T) {
		body := fmt.Sprintf(`{"bill_id":"%s","amount":"50000","method":"TRANSFER","proof_path":"proofs/valid_proof.jpg"}`, billID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		var p Payment
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &p))
		require.Equal(t, PaymentStatusPending, p.Status)
		require.NotNil(t, p.ProofPath)
		require.Equal(t, "proofs/valid_proof.jpg", *p.ProofPath)
	})

	t.Run("JSON CreateMultiPeriodPayment without proof is rejected", func(t *testing.T) {
		body := fmt.Sprintf(`{"bill_ids":["%s"],"allocations":[{"bill_id":"%s","amount":"50000"}],"amount":"50000","method":"TRANSFER"}`, billID, billID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Contains(t, rec.Body.String(), expectedErrMsg)
	})
}
