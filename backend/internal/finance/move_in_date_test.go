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
	"social-finance/internal/database"
)

func setupTestBankLocation(t *testing.T, pool *database.Pool, rtID, name string) string {
	t.Helper()
	var id string
	err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO fund_locations (rt_id, name, category, bank_name, account_number, account_holder, is_active)
		 VALUES ($1, $2, 'rekening', 'BCA', '1234567890', 'RT Treasurer', true) RETURNING id`,
		rtID, name,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// -----------------------------------------------------------------------------
// 9A. Skenario perhitungan kewajiban (8 skenario)
// -----------------------------------------------------------------------------
func TestMoveInDate_9A_CalculationScenarios(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID := setupTestRT(t, pool, "RT Move-In 9A", 1, "RTM9A")
	defer cleanupTestRT(t, pool, rtID)

	phoneB := fmt.Sprintf("0811%08d", time.Now().UnixNano()%100000000)
	userB, midB := setupTestUserWithMembership(t, pool, "ben9a@test.com", "Bendahara 9A", phoneB, rtID, "bendahara")
	tokenBendahara := makeToken(t, auth.TokenClaims{UserID: userB, MID: midB, RTID: rtID, Role: string(auth.RoleBendahara)})

	ctx := context.Background()

	// Create Multi-Month Due 2026
	bodyDue := `{"name":"IPL 2026 9A","amount":"100000","due_type":"multi_month","year":2026,"due_date":"2026-01-10"}`
	reqDue := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(bodyDue))
	reqDue.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqDue.Header.Set("Content-Type", "application/json")
	recDue := httptest.NewRecorder()
	router.ServeHTTP(recDue, reqDue)
	require.Equal(t, http.StatusCreated, recDue.Code)
	var due Due
	require.NoError(t, json.Unmarshal(recDue.Body.Bytes(), &due))

	scenarios := []struct {
		name          string
		moveInDate    string
		expectedTotal int
		expectedOblig int
		expectedFirst string
		expectedLast  string
	}{
		{
			name:          "1. 1 Januari 2026: 12 bulan wajib (Jan–Des)",
			moveInDate:    "2026-01-01",
			expectedTotal: 12,
			expectedOblig: 12,
			expectedFirst: "2026-01",
			expectedLast:  "2026-12",
		},
		{
			name:          "2. 15 Maret 2026: 10 bulan wajib (Mar–Des), Jan–Feb tidak wajib",
			moveInDate:    "2026-03-15",
			expectedTotal: 12,
			expectedOblig: 10,
			expectedFirst: "2026-03",
			expectedLast:  "2026-12",
		},
		{
			name:          "3. 15 Oktober 2026: 3 bulan wajib (Okt–Des), Jan–Sep tidak wajib",
			moveInDate:    "2026-10-15",
			expectedTotal: 12,
			expectedOblig: 3,
			expectedFirst: "2026-10",
			expectedLast:  "2026-12",
		},
		{
			name:          "4. 31 Oktober 2026: 3 bulan wajib (Okt–Des), Jan–Sep tidak wajib",
			moveInDate:    "2026-10-31",
			expectedTotal: 12,
			expectedOblig: 3,
			expectedFirst: "2026-10",
			expectedLast:  "2026-12",
		},
		{
			name:          "5. 1 November 2026: 2 bulan wajib (Nov–Des), Jan–Okt tidak wajib",
			moveInDate:    "2026-11-01",
			expectedTotal: 12,
			expectedOblig: 2,
			expectedFirst: "2026-11",
			expectedLast:  "2026-12",
		},
		{
			name:          "6. 15 Desember 2026: 1 bulan wajib (Des), Jan–Nov tidak wajib",
			moveInDate:    "2026-12-15",
			expectedTotal: 12,
			expectedOblig: 1,
			expectedFirst: "2026-12",
			expectedLast:  "2026-12",
		},
		{
			name:          "7. 15 Oktober 2025: 12 bulan wajib (Jan–Des 2026) move-in sebelum tahun tagihan",
			moveInDate:    "2025-10-15",
			expectedTotal: 12,
			expectedOblig: 12,
			expectedFirst: "2026-01",
			expectedLast:  "2026-12",
		},
		{
			name:          "8. 15 Januari 2027: 0 bulan wajib untuk tahun 2026",
			moveInDate:    "2027-01-15",
			expectedTotal: 0, // Does not overlap with 2026 calendar year, no bills generated
			expectedOblig: 0,
		},
	}

	for i, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			var hhID, houseID, occID string
			err := pool.Raw().QueryRowContext(ctx,
				`INSERT INTO households (rt_id, head_name) VALUES ($1, $2) RETURNING id`,
				rtID, fmt.Sprintf("Head 9A-%d", i)).Scan(&hhID)
			require.NoError(t, err)

			err = pool.Raw().QueryRowContext(ctx,
				`INSERT INTO physical_houses (rt_id, house_number) VALUES ($1, $2) RETURNING id`,
				rtID, fmt.Sprintf("H-9A-%d", i)).Scan(&houseID)
			require.NoError(t, err)

			err = pool.Raw().QueryRowContext(ctx,
				`INSERT INTO household_occupancies (household_id, physical_house_id, occupancy_status, start_date)
				 VALUES ($1, $2, 'OWNER', $3) RETURNING id`,
				hhID, houseID, sc.moveInDate).Scan(&occID)
			require.NoError(t, err)

			// Sync bills for this occupancy
			tx, err := pool.BeginTx(ctx)
			require.NoError(t, err)
			_, err = SyncHouseholdOccupancyBills(ctx, tx, rtID, occID)
			require.NoError(t, err)
			require.NoError(t, tx.Commit())

			// Query bills generated for this occupancy
			type billRow struct {
				Period       string
				IsObligatory bool
			}
			rows, err := pool.Raw().QueryContext(ctx,
				`SELECT period, is_obligatory FROM bills WHERE due_id = $1 AND household_occupancy_id = $2 ORDER BY period ASC`,
				due.ID, occID)
			require.NoError(t, err)
			defer rows.Close()

			var bills []billRow
			var obligPeriods []string
			for rows.Next() {
				var br billRow
				require.NoError(t, rows.Scan(&br.Period, &br.IsObligatory))
				bills = append(bills, br)
				if br.IsObligatory {
					obligPeriods = append(obligPeriods, br.Period)
				}
			}

			require.Len(t, bills, sc.expectedTotal, "Total bills count mismatch for %s", sc.moveInDate)
			require.Len(t, obligPeriods, sc.expectedOblig, "Obligatory bills count mismatch for %s", sc.moveInDate)

			if sc.expectedOblig > 0 {
				require.Equal(t, sc.expectedFirst, obligPeriods[0], "First obligatory month mismatch")
				require.Equal(t, sc.expectedLast, obligPeriods[len(obligPeriods)-1], "Last obligatory month mismatch")
			}
		})
	}
}

// -----------------------------------------------------------------------------
// 9B. Skenario FIFO enforcement
// -----------------------------------------------------------------------------
func TestMoveInDate_9B_FIFOEnforcement(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID := setupTestRT(t, pool, "RT Move-In 9B", 1, "RTM9B")
	defer cleanupTestRT(t, pool, rtID)

	phoneB := fmt.Sprintf("0811%08d", time.Now().UnixNano()%100000000)
	userB, midB := setupTestUserWithMembership(t, pool, "ben9b@test.com", "Bendahara 9B", phoneB, rtID, "bendahara")
	tokenBendahara := makeToken(t, auth.TokenClaims{UserID: userB, MID: midB, RTID: rtID, Role: string(auth.RoleBendahara)})

	ctx := context.Background()

	// Create Multi-Month Due 2026
	bodyDue := `{"name":"IPL 2026 9B","amount":"50000","due_type":"multi_month","year":2026,"due_date":"2026-01-10"}`
	reqDue := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(bodyDue))
	reqDue.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqDue.Header.Set("Content-Type", "application/json")
	recDue := httptest.NewRecorder()
	router.ServeHTTP(recDue, reqDue)
	require.Equal(t, http.StatusCreated, recDue.Code)
	var due Due
	require.NoError(t, json.Unmarshal(recDue.Body.Bytes(), &due))

	// Setup household with move-in date 2026-10-15
	var hhID, houseID, occID string
	err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO households (rt_id, head_name) VALUES ($1, 'KK Oct 15') RETURNING id`, rtID).Scan(&hhID)
	require.NoError(t, err)

	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO physical_houses (rt_id, house_number) VALUES ($1, 'B-9B') RETURNING id`, rtID).Scan(&houseID)
	require.NoError(t, err)

	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO household_occupancies (household_id, physical_house_id, occupancy_status, start_date)
		 VALUES ($1, $2, 'OWNER', '2026-10-15') RETURNING id`, hhID, houseID).Scan(&occID)
	require.NoError(t, err)

	phoneW := fmt.Sprintf("0877%08d", time.Now().UnixNano()%100000000)
	var resID string
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO residents (rt_id, full_name, phone, is_active) VALUES ($1, 'Warga Oct', $2, true) RETURNING id`, rtID, phoneW).Scan(&resID)
	require.NoError(t, err)
	_, err = pool.Raw().ExecContext(ctx,
		`INSERT INTO residency_periods (resident_id, household_occupancy_id, start_date) VALUES ($1, $2, '2026-10-15')`, resID, occID)
	require.NoError(t, err)

	userW, midW := setupTestUserWithMembership(t, pool, "warga9b@test.com", "Warga Oct", phoneW, rtID, "warga")
	tokenWarga := makeToken(t, auth.TokenClaims{UserID: userW, MID: midW, RTID: rtID, Role: string(auth.RoleWarga)})

	// Sync bills
	tx, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	_, err = SyncHouseholdOccupancyBills(ctx, tx, rtID, occID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	var billIDs = make(map[string]string)
	rows, err := pool.Raw().QueryContext(ctx,
		`SELECT period, id FROM bills WHERE due_id = $1 AND household_occupancy_id = $2`, due.ID, occID)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var p, id string
		require.NoError(t, rows.Scan(&p, &id))
		billIDs[p] = id
	}

	// Skenario 1: Warga mencoba membayar Januari 2026 (non-obligatory) -> DITOLAK
	t.Run("Reject payment on non-obligatory month Jan 2026", func(t *testing.T) {
		body := fmt.Sprintf(`{"bill_id":"%s","amount":"50000","method":"TRANSFER","proof_path":"proofs/test.jpg"}`, billIDs["2026-01"])
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Contains(t, rec.Body.String(), "tidak wajib dibayar")
	})

	// Skenario 2: Warga mencoba membayar November 2026 sebelum Oktober 2026 -> DITOLAK (FIFO)
	t.Run("Reject payment on Nov 2026 before Oct 2026 is paid (FIFO violation)", func(t *testing.T) {
		body := fmt.Sprintf(`{"bill_id":"%s","amount":"50000","method":"TRANSFER","proof_path":"proofs/test.jpg"}`, billIDs["2026-11"])
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Contains(t, rec.Body.String(), "FIFO")
	})

	// Skenario 3: Warga dapat LANGSUNG membayar Oktober 2026 (periode wajib pertama) -> SUKSES
	var octPaymentID string
	t.Run("Pay Oct 2026 directly (first obligatory month) succeeds", func(t *testing.T) {
		body := fmt.Sprintf(`{"bill_id":"%s","amount":"50000","method":"TRANSFER","proof_path":"proofs/test.jpg"}`, billIDs["2026-10"])
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code)

		var p Payment
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &p))
		octPaymentID = p.ID
	})

	// Bendahara approves Oct 2026 payment
	t.Run("Approve Oct 2026 payment", func(t *testing.T) {
		locID := setupTestBankLocation(t, pool, rtID, "Rekening BCA 9B")
		bodyVerify := fmt.Sprintf(`{"action":"approve","notes":"Approved Oct","actual_received_date":"2026-10-16","location_id":"%s"}`, locID)
		reqVerify := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/payments/%s/verify", octPaymentID), bytes.NewBufferString(bodyVerify))
		reqVerify.Header.Set("Authorization", "Bearer "+tokenBendahara)
		reqVerify.Header.Set("Content-Type", "application/json")
		recVerify := httptest.NewRecorder()
		router.ServeHTTP(recVerify, reqVerify)
		require.Equal(t, http.StatusOK, recVerify.Code, recVerify.Body.String())

		var status string
		err := pool.Raw().QueryRowContext(ctx, `SELECT status FROM bills WHERE id = $1`, billIDs["2026-10"]).Scan(&status)
		require.NoError(t, err)
		require.Equal(t, "paid", status)
	})

	// Skenario 4: Setelah Oktober lunas, warga sekarang BISA membayar November 2026 -> SUKSES
	t.Run("Pay Nov 2026 after Oct 2026 is paid succeeds", func(t *testing.T) {
		body := fmt.Sprintf(`{"bill_id":"%s","amount":"50000","method":"TRANSFER","proof_path":"proofs/test.jpg"}`, billIDs["2026-11"])
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code)
	})
}

// -----------------------------------------------------------------------------
// 9C. Skenario regresi
// -----------------------------------------------------------------------------
func TestMoveInDate_9C_RegressionScenarios(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID := setupTestRT(t, pool, "RT Move-In 9C", 1, "RTM9C")
	defer cleanupTestRT(t, pool, rtID)

	phoneB := fmt.Sprintf("0811%08d", time.Now().UnixNano()%100000000)
	userB, midB := setupTestUserWithMembership(t, pool, "ben9c@test.com", "Bendahara 9C", phoneB, rtID, "bendahara")
	tokenBendahara := makeToken(t, auth.TokenClaims{UserID: userB, MID: midB, RTID: rtID, Role: string(auth.RoleBendahara)})

	ctx := context.Background()

	// 1. Warga lama tanpa start_date (NULL): tetap memiliki 12 bulan wajib
	t.Run("Legacy resident with NULL start_date has 12 obligatory bills", func(t *testing.T) {
		var hhID, houseID, occID string
		err := pool.Raw().QueryRowContext(ctx,
			`INSERT INTO households (rt_id, head_name) VALUES ($1, 'Legacy Head') RETURNING id`, rtID).Scan(&hhID)
		require.NoError(t, err)

		err = pool.Raw().QueryRowContext(ctx,
			`INSERT INTO physical_houses (rt_id, house_number) VALUES ($1, 'H-LEGACY') RETURNING id`, rtID).Scan(&houseID)
		require.NoError(t, err)

		err = pool.Raw().QueryRowContext(ctx,
			`INSERT INTO household_occupancies (household_id, physical_house_id, occupancy_status, start_date)
			 VALUES ($1, $2, 'OWNER', NULL) RETURNING id`, hhID, houseID).Scan(&occID)
		require.NoError(t, err)

		bodyDue := `{"name":"IPL Legacy","amount":"60000","due_type":"multi_month","year":2026,"due_date":"2026-01-10"}`
		reqDue := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(bodyDue))
		reqDue.Header.Set("Authorization", "Bearer "+tokenBendahara)
		reqDue.Header.Set("Content-Type", "application/json")
		recDue := httptest.NewRecorder()
		router.ServeHTTP(recDue, reqDue)
		require.Equal(t, http.StatusCreated, recDue.Code)
		var due Due
		require.NoError(t, json.Unmarshal(recDue.Body.Bytes(), &due))

		tx, err := pool.BeginTx(ctx)
		require.NoError(t, err)
		_, err = SyncHouseholdOccupancyBills(ctx, tx, rtID, occID)
		require.NoError(t, err)
		require.NoError(t, tx.Commit())

		var countOblig int
		err = pool.Raw().QueryRowContext(ctx,
			`SELECT COUNT(*) FROM bills WHERE due_id = $1 AND household_occupancy_id = $2 AND is_obligatory = true`,
			due.ID, occID).Scan(&countOblig)
		require.NoError(t, err)
		require.Equal(t, 12, countOblig, "Legacy resident with NULL start_date must have all 12 bills marked obligatory")
	})

	// 2. Satu KK banyak anggota/akun warga: tagihan per KK tidak dobel
	t.Run("Multi-member household does not duplicate bills", func(t *testing.T) {
		rtIDMulti := setupTestRT(t, pool, "RT Move-In 9C Multi", 2, "RTM9CM")
		defer cleanupTestRT(t, pool, rtIDMulti)

		phoneBM := fmt.Sprintf("0811%08d", time.Now().UnixNano()%100000000)
		userBM, midBM := setupTestUserWithMembership(t, pool, "ben9cm@test.com", "Bendahara 9C Multi", phoneBM, rtIDMulti, "bendahara")
		tokenBM := makeToken(t, auth.TokenClaims{UserID: userBM, MID: midBM, RTID: rtIDMulti, Role: string(auth.RoleBendahara)})

		var hhID, houseID, occID string
		err := pool.Raw().QueryRowContext(ctx,
			`INSERT INTO households (rt_id, head_name) VALUES ($1, 'Multi Member Head') RETURNING id`, rtIDMulti).Scan(&hhID)
		require.NoError(t, err)

		err = pool.Raw().QueryRowContext(ctx,
			`INSERT INTO physical_houses (rt_id, house_number) VALUES ($1, 'H-MULTI') RETURNING id`, rtIDMulti).Scan(&houseID)
		require.NoError(t, err)

		err = pool.Raw().QueryRowContext(ctx,
			`INSERT INTO household_occupancies (household_id, physical_house_id, occupancy_status, start_date)
			 VALUES ($1, $2, 'OWNER', '2026-06-01') RETURNING id`, hhID, houseID).Scan(&occID)
		require.NoError(t, err)

		// Member 1
		p1 := fmt.Sprintf("081%08d", time.Now().UnixNano()%100000000)
		var r1 string
		err = pool.Raw().QueryRowContext(ctx,
			`INSERT INTO residents (rt_id, full_name, phone, is_active) VALUES ($1, 'Member 1', $2, true) RETURNING id`, rtIDMulti, p1).Scan(&r1)
		require.NoError(t, err)
		_, err = pool.Raw().ExecContext(ctx,
			`INSERT INTO residency_periods (resident_id, household_occupancy_id, start_date) VALUES ($1, $2, '2026-06-01')`, r1, occID)
		require.NoError(t, err)

		// Member 2
		p2 := fmt.Sprintf("082%08d", time.Now().UnixNano()%100000000)
		var r2 string
		err = pool.Raw().QueryRowContext(ctx,
			`INSERT INTO residents (rt_id, full_name, phone, is_active) VALUES ($1, 'Member 2', $2, true) RETURNING id`, rtIDMulti, p2).Scan(&r2)
		require.NoError(t, err)
		_, err = pool.Raw().ExecContext(ctx,
			`INSERT INTO residency_periods (resident_id, household_occupancy_id, start_date) VALUES ($1, $2, '2026-06-01')`, r2, occID)
		require.NoError(t, err)

		bodyDue := `{"name":"IPL Multi","amount":"70000","due_type":"multi_month","year":2026,"due_date":"2026-01-10","period_type":"monthly"}`
		reqDue := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(bodyDue))
		reqDue.Header.Set("Authorization", "Bearer "+tokenBM)
		reqDue.Header.Set("Content-Type", "application/json")
		recDue := httptest.NewRecorder()
		router.ServeHTTP(recDue, reqDue)
		require.Equal(t, http.StatusCreated, recDue.Code, recDue.Body.String())
		var due Due
		require.NoError(t, json.Unmarshal(recDue.Body.Bytes(), &due))

		tx, err := pool.BeginTx(ctx)
		require.NoError(t, err)
		_, err = SyncHouseholdOccupancyBills(ctx, tx, rtIDMulti, occID)
		require.NoError(t, err)
		require.NoError(t, tx.Commit())

		var countTotal int
		err = pool.Raw().QueryRowContext(ctx,
			`SELECT COUNT(*) FROM bills WHERE due_id = $1 AND household_occupancy_id = $2`,
			due.ID, occID).Scan(&countTotal)
		require.NoError(t, err)
		require.Equal(t, 12, countTotal, "Multi-member household must have exactly 12 bills (unit of billing is KK)")
	})

	// 3. Sukarela dues are NEVER touched or auto-generated
	t.Run("Voluntary dues are untouched and never auto-generated", func(t *testing.T) {
		bodyVol := `{"name":"Sumbangan 17an","amount":"0","due_type":"voluntary","is_voluntary":true,"period_type":"one_time"}`
		reqVol := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(bodyVol))
		reqVol.Header.Set("Authorization", "Bearer "+tokenBendahara)
		reqVol.Header.Set("Content-Type", "application/json")
		recVol := httptest.NewRecorder()
		router.ServeHTTP(recVol, reqVol)
		require.Equal(t, http.StatusCreated, recVol.Code, recVol.Body.String())
		var dueVol Due
		require.NoError(t, json.Unmarshal(recVol.Body.Bytes(), &dueVol))

		// Check count of bills for voluntary due
		var countVolBills int
		err := pool.Raw().QueryRowContext(ctx, `SELECT COUNT(*) FROM bills WHERE due_id = $1`, dueVol.ID).Scan(&countVolBills)
		require.NoError(t, err)
		require.Equal(t, 0, countVolBills, "Voluntary dues must have 0 auto-generated bills")
	})
}

// -----------------------------------------------------------------------------
// 9D. Skenario perubahan tanggal mulai menempati (move-in date updated)
// -----------------------------------------------------------------------------
func TestMoveInDate_9D_MoveInDateChange(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID := setupTestRT(t, pool, "RT Move-In 9D", 1, "RTM9D")
	defer cleanupTestRT(t, pool, rtID)

	phoneB := fmt.Sprintf("0811%08d", time.Now().UnixNano()%100000000)
	userB, midB := setupTestUserWithMembership(t, pool, "ben9d@test.com", "Bendahara 9D", phoneB, rtID, "bendahara")
	tokenBendahara := makeToken(t, auth.TokenClaims{UserID: userB, MID: midB, RTID: rtID, Role: string(auth.RoleBendahara)})

	ctx := context.Background()

	bodyDue := `{"name":"IPL 2026 9D","amount":"50000","due_type":"multi_month","year":2026,"due_date":"2026-01-10"}`
	reqDue := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(bodyDue))
	reqDue.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqDue.Header.Set("Content-Type", "application/json")
	recDue := httptest.NewRecorder()
	router.ServeHTTP(recDue, reqDue)
	require.Equal(t, http.StatusCreated, recDue.Code)
	var due Due
	require.NoError(t, json.Unmarshal(recDue.Body.Bytes(), &due))

	// Initial occupancy: move-in date 2026-10-15 (Oct, Nov, Dec obligatory = 3 months)
	var hhID, houseID, occID string
	err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO households (rt_id, head_name) VALUES ($1, 'KK Move Change') RETURNING id`, rtID).Scan(&hhID)
	require.NoError(t, err)

	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO physical_houses (rt_id, house_number) VALUES ($1, 'H-9D') RETURNING id`, rtID).Scan(&houseID)
	require.NoError(t, err)

	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO household_occupancies (household_id, physical_house_id, occupancy_status, start_date)
		 VALUES ($1, $2, 'OWNER', '2026-10-15') RETURNING id`, hhID, houseID).Scan(&occID)
	require.NoError(t, err)

	phoneW := fmt.Sprintf("0877%08d", time.Now().UnixNano()%100000000)
	var resID string
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO residents (rt_id, full_name, phone, is_active) VALUES ($1, 'Warga 9D', $2, true) RETURNING id`, rtID, phoneW).Scan(&resID)
	require.NoError(t, err)
	_, err = pool.Raw().ExecContext(ctx,
		`INSERT INTO residency_periods (resident_id, household_occupancy_id, start_date) VALUES ($1, $2, '2026-10-15')`, resID, occID)
	require.NoError(t, err)

	userW, midW := setupTestUserWithMembership(t, pool, "warga9d@test.com", "Warga 9D", phoneW, rtID, "warga")
	tokenWarga := makeToken(t, auth.TokenClaims{UserID: userW, MID: midW, RTID: rtID, Role: string(auth.RoleWarga)})

	tx, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	_, err = SyncHouseholdOccupancyBills(ctx, tx, rtID, occID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	var obligBefore int
	err = pool.Raw().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM bills WHERE due_id = $1 AND household_occupancy_id = $2 AND is_obligatory = true`,
		due.ID, occID).Scan(&obligBefore)
	require.NoError(t, err)
	require.Equal(t, 3, obligBefore, "Initially 3 months obligatory (Oct, Nov, Dec)")

	// KK pays Oct 2026
	var octBillID string
	err = pool.Raw().QueryRowContext(ctx,
		`SELECT id FROM bills WHERE due_id = $1 AND household_occupancy_id = $2 AND period = '2026-10'`,
		due.ID, occID).Scan(&octBillID)
	require.NoError(t, err)

	bodyPay := fmt.Sprintf(`{"bill_id":"%s","amount":"50000","method":"TRANSFER","proof_path":"proofs/test.jpg"}`, octBillID)
	reqPay := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(bodyPay))
	reqPay.Header.Set("Authorization", "Bearer "+tokenWarga)
	reqPay.Header.Set("Content-Type", "application/json")
	recPay := httptest.NewRecorder()
	router.ServeHTTP(recPay, reqPay)
	require.Equal(t, http.StatusCreated, recPay.Code)
	var pay Payment
	require.NoError(t, json.Unmarshal(recPay.Body.Bytes(), &pay))

	locID := setupTestBankLocation(t, pool, rtID, "BCA 9D")
	bodyApprove := fmt.Sprintf(`{"action":"approve","notes":"OK","actual_received_date":"2026-10-16","location_id":"%s"}`, locID)
	reqApprove := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/payments/%s/verify", pay.ID), bytes.NewBufferString(bodyApprove))
	reqApprove.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqApprove.Header.Set("Content-Type", "application/json")
	recApprove := httptest.NewRecorder()
	router.ServeHTTP(recApprove, reqApprove)
	require.Equal(t, http.StatusOK, recApprove.Code, recApprove.Body.String())

	// Now move-in date is changed backwards to 2026-08-01 (Aug, Sep now also become obligatory!)
	txUpdate, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	_, err = txUpdate.ExecContext(ctx,
		`UPDATE household_occupancies SET start_date = '2026-08-01' WHERE id = $1`, occID)
	require.NoError(t, err)
	// Sync reflects updated obligation
	_, err = SyncHouseholdOccupancyBills(ctx, txUpdate, rtID, occID)
	require.NoError(t, err)
	require.NoError(t, txUpdate.Commit())

	// Verify: August and September unpaid bills now become is_obligatory = true
	var obligAfter int
	err = pool.Raw().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM bills WHERE due_id = $1 AND household_occupancy_id = $2 AND is_obligatory = true`,
		due.ID, occID).Scan(&obligAfter)
	require.NoError(t, err)
	require.Equal(t, 5, obligAfter, "After moving move-in date to Aug 1, 5 months are obligatory (Aug-Dec)")

	// Verify October bill is STILL paid!
	var octStatus string
	err = pool.Raw().QueryRowContext(ctx, `SELECT status FROM bills WHERE id = $1`, octBillID).Scan(&octStatus)
	require.NoError(t, err)
	require.Equal(t, "paid", octStatus, "Already paid bill must remain paid and untouched after move-in date change")
}
