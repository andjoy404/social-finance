package finance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"github.com/stretchr/testify/require"
)

// TestNewResidentBills12Scenarios validates all 12 required test scenarios
// for mid-year / mid-month resident joining and bill generation integrity.
func TestNewResidentBills12Scenarios(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, tokenBendahara, _ := setupValidationRT(t, pool)
	defer cleanupTestRT(t, pool, rtID)

	ctx := context.Background()

	// Create an active Multi-Month due for 2026
	bodyMM := `{"name":"IPL Multi-Bulan 2026","amount":"50000","due_type":"multi_month","year":2026,"period_type":"monthly"}`
	reqMM := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(bodyMM))
	reqMM.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqMM.Header.Set("Content-Type", "application/json")
	recMM := httptest.NewRecorder()
	router.ServeHTTP(recMM, reqMM)
	require.Equal(t, http.StatusCreated, recMM.Code, "failed to create multi-month due: %s", recMM.Body.String())

	var dueMM Due
	require.NoError(t, json.Unmarshal(recMM.Body.Bytes(), &dueMM))

	// Helper to create a new household with given startDate
	createNewHouseholdWithOccupancy := func(t *testing.T, houseNum, headName, startDate string) (string, string) {
		t.Helper()
		tx, err := pool.BeginTx(ctx)
		require.NoError(t, err)
		defer tx.Rollback()

		// Physical house
		var phID string
		err = tx.QueryRowContext(ctx, `
			INSERT INTO physical_houses (rt_id, house_number, is_active)
			VALUES ($1, $2, true) RETURNING id
		`, rtID, houseNum).Scan(&phID)
		require.NoError(t, err)

		// Household
		var hhID string
		err = tx.QueryRowContext(ctx, `
			INSERT INTO households (rt_id, head_name, is_active)
			VALUES ($1, $2, true) RETURNING id
		`, rtID, headName).Scan(&hhID)
		require.NoError(t, err)

		// Household Occupancy
		var occID string
		err = tx.QueryRowContext(ctx, `
			INSERT INTO household_occupancies (physical_house_id, household_id, occupancy_status, start_date)
			VALUES ($1, $2, 'OWNER', $3) RETURNING id
		`, phID, hhID, startDate).Scan(&occID)
		require.NoError(t, err)

		// Trigger sync bills hook
		n, err := SyncHouseholdOccupancyBills(ctx, tx, rtID, occID)
		require.NoError(t, err)
		_ = n

		require.NoError(t, tx.Commit())
		return hhID, occID
	}

	// Helper to count bills for an occupancy and check periods
	getBillsForOccupancy := func(t *testing.T, occID string) []Bill {
		t.Helper()
		tx, err := pool.BeginTx(ctx)
		require.NoError(t, err)
		defer tx.Rollback()

		rows, err := tx.QueryContext(ctx, `
			SELECT b.id, b.rt_id, b.household_occupancy_id, b.due_id, b.amount::text, b.period, to_char(b.due_date, 'YYYY-MM-DD'), b.status, b.is_active
			FROM bills b
			WHERE b.household_occupancy_id = $1
			ORDER BY b.period ASC
		`, occID)
		require.NoError(t, err)
		defer rows.Close()

		var res []Bill
		for rows.Next() {
			var b Bill
			var act bool
			err := rows.Scan(&b.ID, &b.RTID, &b.HouseholdOccupancyID, &b.DueID, &b.Amount, &b.Period, &b.DueDate, &b.Status, &act)
			require.NoError(t, err)
			b.IsActive = act
			res = append(res, b)
		}
		return res
	}

	// -------------------------------------------------------------
	// SCENARIO 1: Warga bergabung 1 Januari -> Mendapatkan 12 tagihan
	// -------------------------------------------------------------
	t.Run("Scenario 1: Warga bergabung 1 Januari mendapat 12 tagihan", func(t *testing.T) {
		_, occID := createNewHouseholdWithOccupancy(t, "A-01", "Warga Jan 01", "2026-01-01")
		bills := getBillsForOccupancy(t, occID)
		require.Len(t, bills, 12, "warga 1 Jan harus mendapatkan 12 tagihan")
		for i, b := range bills {
			expectedPeriod := fmt.Sprintf("2026-%02d", i+1)
			require.Equal(t, expectedPeriod, b.Period)
			require.Equal(t, BillStatusUnpaid, b.Status)
			require.True(t, b.IsActive)
		}
	})

	// -------------------------------------------------------------------------
	// SCENARIO 2: Warga bergabung 15 Maret -> Mendapatkan 12 tagihan tahun berjalan
	// -------------------------------------------------------------------------
	t.Run("Scenario 2: Warga bergabung 15 Maret mendapat 12 tagihan tahun berjalan", func(t *testing.T) {
		_, occID := createNewHouseholdWithOccupancy(t, "A-02", "Warga Mar 15", "2026-03-15")
		bills := getBillsForOccupancy(t, occID)
		require.Len(t, bills, 12, "warga 15 Mar harus mendapatkan 12 tagihan tahun berjalan")
		require.Equal(t, "2026-01", bills[0].Period)
		require.Equal(t, "2026-12", bills[11].Period)
	})

	// ---------------------------------------------------------------------------
	// SCENARIO 3: Warga bergabung 15 Oktober -> Mendapatkan 12 tagihan tahun berjalan
	// ---------------------------------------------------------------------------
	t.Run("Scenario 3: Warga bergabung 15 Oktober mendapat 12 tagihan tahun berjalan", func(t *testing.T) {
		_, occID := createNewHouseholdWithOccupancy(t, "A-03", "Warga Okt 15", "2026-10-15")
		bills := getBillsForOccupancy(t, occID)
		require.Len(t, bills, 12, "warga 15 Okt harus mendapatkan 12 tagihan tahun berjalan")
		require.Equal(t, "2026-01", bills[0].Period)
		require.Equal(t, "2026-10", bills[9].Period)
		require.Equal(t, "2026-12", bills[11].Period)
	})

	// ------------------------------------------------------------------------------
	// SCENARIO 4: Warga bergabung 31 Desember -> Mendapatkan 12 tagihan tahun berjalan
	// ------------------------------------------------------------------------------
	t.Run("Scenario 4: Warga bergabung 31 Desember mendapat 12 tagihan tahun berjalan", func(t *testing.T) {
		_, occID := createNewHouseholdWithOccupancy(t, "A-04", "Warga Des 31", "2026-12-31")
		bills := getBillsForOccupancy(t, occID)
		require.Len(t, bills, 12, "warga 31 Des harus mendapatkan 12 tagihan tahun berjalan")
		require.Equal(t, "2026-01", bills[0].Period)
		require.Equal(t, "2026-12", bills[11].Period)
	})

	// ---------------------------------------------------------------------------
	// SCENARIO 5: KK lama sudah memiliki 12 tagihan -> Tidak ada duplikasi
	// ---------------------------------------------------------------------------
	t.Run("Scenario 5: KK lama sudah memiliki 12 tagihan tidak ada duplikasi", func(t *testing.T) {
		_, occID := createNewHouseholdWithOccupancy(t, "A-05", "KK Lama", "2026-01-01")
		bills1 := getBillsForOccupancy(t, occID)
		require.Len(t, bills1, 12)

		// Sync again
		tx, err := pool.BeginTx(ctx)
		require.NoError(t, err)
		n, err := SyncHouseholdOccupancyBills(ctx, tx, rtID, occID)
		require.NoError(t, err)
		require.Equal(t, 0, n, "syncing already complete KK should create 0 new bills")
		require.NoError(t, tx.Commit())

		bills2 := getBillsForOccupancy(t, occID)
		require.Len(t, bills2, 12, "tagihan KK lama tetap 12 tanpa duplikasi")
	})

	// ----------------------------------------------------------------------------------
	// SCENARIO 6: KK baru sudah memiliki sebagian tagihan -> Hanya tagihan kurang dibuat
	// ----------------------------------------------------------------------------------
	t.Run("Scenario 6: KK memiliki sebagian tagihan hanya tagihan kurang yang dibuat", func(t *testing.T) {
		tx, err := pool.BeginTx(ctx)
		require.NoError(t, err)
		defer tx.Rollback()

		var phID, hhID, occID string
		_ = tx.QueryRowContext(ctx, `INSERT INTO physical_houses (rt_id, house_number, is_active) VALUES ($1, 'A-06', true) RETURNING id`, rtID).Scan(&phID)
		_ = tx.QueryRowContext(ctx, `INSERT INTO households (rt_id, head_name, is_active) VALUES ($1, 'KK Sebagian', true) RETURNING id`, rtID).Scan(&hhID)
		_ = tx.QueryRowContext(ctx, `INSERT INTO household_occupancies (physical_house_id, household_id, occupancy_status, start_date) VALUES ($1, $2, 'OWNER', '2026-07-01') RETURNING id`, phID, hhID).Scan(&occID)

		// Pre-insert Jan & Feb bills manually (e.g. Jan is already paid)
		_, err = tx.ExecContext(ctx, `
			INSERT INTO bills (rt_id, household_occupancy_id, due_id, amount, period, due_date, status, is_active)
			VALUES ($1, $2, $3, 50000, '2026-01', '2026-01-10', 'paid', true),
			       ($1, $2, $3, 50000, '2026-02', '2026-02-10', 'unpaid', true)
		`, rtID, occID, dueMM.ID)
		require.NoError(t, err)
		require.NoError(t, tx.Commit())

		// Now run Sync
		tx2, err := pool.BeginTx(ctx)
		require.NoError(t, err)
		n, err := SyncHouseholdOccupancyBills(ctx, tx2, rtID, occID)
		require.NoError(t, err)
		require.Equal(t, 10, n, "only 10 missing bills (Mar to Dec) should be created")
		require.NoError(t, tx2.Commit())

		bills := getBillsForOccupancy(t, occID)
		require.Len(t, bills, 12, "total must be 12 bills")
		// Jan must still be 'paid'
		require.Equal(t, "2026-01", bills[0].Period)
		require.Equal(t, BillStatusPaid, bills[0].Status, "status paid on pre-existing Jan bill must remain intact")
	})

	// ----------------------------------------------------------------------------------
	// SCENARIO 7: Proses generate dipanggil dua kali -> Tidak ada duplikasi
	// ----------------------------------------------------------------------------------
	t.Run("Scenario 7: Proses generate dipanggil dua kali tidak ada duplikasi", func(t *testing.T) {
		_, occID := createNewHouseholdWithOccupancy(t, "A-07", "KK Double Gen", "2026-05-01")

		// Call GenerateBills via API for the same due and year
		genBody := fmt.Sprintf(`{"due_id":"%s","period":"2026"}`, dueMM.ID)
		reqGen1 := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(genBody))
		reqGen1.Header.Set("Authorization", "Bearer "+tokenBendahara)
		reqGen1.Header.Set("Content-Type", "application/json")
		recGen1 := httptest.NewRecorder()
		router.ServeHTTP(recGen1, reqGen1)
		require.Equal(t, http.StatusCreated, recGen1.Code)

		reqGen2 := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(genBody))
		reqGen2.Header.Set("Authorization", "Bearer "+tokenBendahara)
		reqGen2.Header.Set("Content-Type", "application/json")
		recGen2 := httptest.NewRecorder()
		router.ServeHTTP(recGen2, reqGen2)
		require.Equal(t, http.StatusCreated, recGen2.Code)

		bills := getBillsForOccupancy(t, occID)
		require.Len(t, bills, 12, "tetap 12 tagihan tanpa duplikasi")
	})

	// ----------------------------------------------------------------------------------
	// SCENARIO 8: Dua request generate berjalan bersamaan -> Tidak ada duplikasi
	// ----------------------------------------------------------------------------------
	t.Run("Scenario 8: Dua request generate berjalan bersamaan aman dan bebas duplikasi", func(t *testing.T) {
		tx, err := pool.BeginTx(ctx)
		require.NoError(t, err)
		defer tx.Rollback()

		var phID, hhID, occID string
		_ = tx.QueryRowContext(ctx, `INSERT INTO physical_houses (rt_id, house_number, is_active) VALUES ($1, 'A-08', true) RETURNING id`, rtID).Scan(&phID)
		_ = tx.QueryRowContext(ctx, `INSERT INTO households (rt_id, head_name, is_active) VALUES ($1, 'KK Concurrency', true) RETURNING id`, rtID).Scan(&hhID)
		_ = tx.QueryRowContext(ctx, `INSERT INTO household_occupancies (physical_house_id, household_id, occupancy_status, start_date) VALUES ($1, $2, 'OWNER', '2026-08-15') RETURNING id`, phID, hhID).Scan(&occID)
		require.NoError(t, tx.Commit())

		// Launch 5 concurrent sync routines
		var wg sync.WaitGroup
		errs := make([]error, 5)
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				txi, err := pool.BeginTx(ctx)
				if err != nil {
					errs[idx] = err
					return
				}
				defer txi.Rollback()
				_, err = SyncHouseholdOccupancyBills(ctx, txi, rtID, occID)
				if err != nil {
					errs[idx] = err
					return
				}
				errs[idx] = txi.Commit()
			}(i)
		}
		wg.Wait()

		for _, e := range errs {
			require.NoError(t, e)
		}

		bills := getBillsForOccupancy(t, occID)
		require.Len(t, bills, 12, "hasil concurrent sync harus tepat 12 tagihan")
	})

	// ----------------------------------------------------------------------------------
	// SCENARIO 9: Warga baru dengan beberapa anggota keluarga -> Tagihan per KK bukan per warga
	// ----------------------------------------------------------------------------------
	t.Run("Scenario 9: Anggota keluarga ganda dalam satu KK tagihan tetap per KK", func(t *testing.T) {
		_, occID := createNewHouseholdWithOccupancy(t, "A-09", "Warga Utama 09", "2026-09-01")

		// Add 3 residents to this same occupancy
		tx, err := pool.BeginTx(ctx)
		require.NoError(t, err)
		defer tx.Rollback()

		for i := 1; i <= 3; i++ {
			var rID string
			err = tx.QueryRowContext(ctx, `
				INSERT INTO residents (rt_id, full_name, is_active)
				VALUES ($1, $2, true) RETURNING id
			`, rtID, fmt.Sprintf("Anggota %d", i)).Scan(&rID)
			require.NoError(t, err)

			_, err = tx.ExecContext(ctx, `
				INSERT INTO residency_periods (resident_id, household_occupancy_id, start_date)
				VALUES ($1, $2, '2026-09-01')
			`, rID, occID)
			require.NoError(t, err)
		}
		require.NoError(t, tx.Commit())

		bills := getBillsForOccupancy(t, occID)
		require.Len(t, bills, 12, "tagihan harus tepat 12 per KK, tidak dikali lipat jumlah anggota keluarga")
	})

	// ----------------------------------------------------------------------------------
	// SCENARIO 10: Warga baru pada tipe Sukarela -> Perilaku Sukarela tidak berubah
	// ----------------------------------------------------------------------------------
	t.Run("Scenario 10: Warga baru pada tipe Sukarela tidak di-generate otomatis", func(t *testing.T) {
		// Create voluntary due
		bodyVol := `{"name":"Sumbangan Sukarela Warga","amount":"0","due_type":"voluntary","period_type":"one_time"}`
		reqVol := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(bodyVol))
		reqVol.Header.Set("Authorization", "Bearer "+tokenBendahara)
		reqVol.Header.Set("Content-Type", "application/json")
		recVol := httptest.NewRecorder()
		router.ServeHTTP(recVol, reqVol)
		require.Equal(t, http.StatusCreated, recVol.Code)

		var dueVol Due
		require.NoError(t, json.Unmarshal(recVol.Body.Bytes(), &dueVol))

		_, occID := createNewHouseholdWithOccupancy(t, "A-10", "Warga Sukarela", "2026-10-01")

		// Query bills for this occupancy specifically for dueVol
		tx, err := pool.BeginTx(ctx)
		require.NoError(t, err)
		defer tx.Rollback()

		var volCount int
		err = tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM bills WHERE household_occupancy_id = $1 AND due_id = $2
		`, occID, dueVol.ID).Scan(&volCount)
		require.NoError(t, err)
		require.Equal(t, 0, volCount, "tagihan sukarela TIDAK boleh di-generate otomatis untuk warga baru")
	})

	// ----------------------------------------------------------------------------------
	// SCENARIO 11: Warga baru pada tipe Reguler -> Sesuai aturan Reguler yang berlaku
	// ----------------------------------------------------------------------------------
	t.Run("Scenario 11: Warga baru pada tipe Reguler mendapatkan periode yang sudah diterbitkan", func(t *testing.T) {
		// Create regular monthly due
		bodyReg := `{"name":"Iuran Sampah Reguler","amount":"15000","due_type":"regular","period_type":"monthly"}`
		reqReg := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(bodyReg))
		reqReg.Header.Set("Authorization", "Bearer "+tokenBendahara)
		reqReg.Header.Set("Content-Type", "application/json")
		recReg := httptest.NewRecorder()
		router.ServeHTTP(recReg, reqReg)
		require.Equal(t, http.StatusCreated, recReg.Code)

		var dueReg Due
		require.NoError(t, json.Unmarshal(recReg.Body.Bytes(), &dueReg))

		// Issue regular bills for period 2026-09 and 2026-10
		genPeriod1 := fmt.Sprintf(`{"due_id":"%s","period":"2026-09","due_date":"2026-09-25"}`, dueReg.ID)
		reqGP1 := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(genPeriod1))
		reqGP1.Header.Set("Authorization", "Bearer "+tokenBendahara)
		reqGP1.Header.Set("Content-Type", "application/json")
		recGP1 := httptest.NewRecorder()
		router.ServeHTTP(recGP1, reqGP1)
		require.Equal(t, http.StatusCreated, recGP1.Code)

		genPeriod2 := fmt.Sprintf(`{"due_id":"%s","period":"2026-10","due_date":"2026-10-25"}`, dueReg.ID)
		reqGP2 := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(genPeriod2))
		reqGP2.Header.Set("Authorization", "Bearer "+tokenBendahara)
		reqGP2.Header.Set("Content-Type", "application/json")
		recGP2 := httptest.NewRecorder()
		router.ServeHTTP(recGP2, reqGP2)
		require.Equal(t, http.StatusCreated, recGP2.Code)

		// Warga baru joins on 2026-10-15
		_, occID := createNewHouseholdWithOccupancy(t, "A-11", "Warga Reguler", "2026-10-15")

		// Query bills for dueReg
		tx, err := pool.BeginTx(ctx)
		require.NoError(t, err)
		defer tx.Rollback()

		rows, err := tx.QueryContext(ctx, `
			SELECT period, amount::text FROM bills WHERE household_occupancy_id = $1 AND due_id = $2 ORDER BY period ASC
		`, occID, dueReg.ID)
		require.NoError(t, err)
		defer rows.Close()

		var periods []string
		for rows.Next() {
			var p, a string
			require.NoError(t, rows.Scan(&p, &a))
			require.Equal(t, "15000.00", a)
			periods = append(periods, p)
		}
		require.Equal(t, []string{"2026-09", "2026-10"}, periods, "warga baru harus mendapatkan periode reguler yang sudah diterbitkan di RT")
	})

	// ----------------------------------------------------------------------------------
	// SCENARIO 12: Warga baru pada tipe Multi-Bulan -> Seluruh 12 periode Jan-Des tersedia
	// ----------------------------------------------------------------------------------
	t.Run("Scenario 12: Warga baru pada tipe Multi-Bulan seluruh 12 periode Jan-Des tersedia", func(t *testing.T) {
		_, occID := createNewHouseholdWithOccupancy(t, "A-12", "Warga Multi-Bulan Full", "2026-10-15")
		bills := getBillsForOccupancy(t, occID)

		var mmPeriods []string
		for _, b := range bills {
			if b.DueID == dueMM.ID {
				mmPeriods = append(mmPeriods, b.Period)
			}
		}
		expected12 := []string{
			"2026-01", "2026-02", "2026-03", "2026-04",
			"2026-05", "2026-06", "2026-07", "2026-08",
			"2026-09", "2026-10", "2026-11", "2026-12",
		}
		require.Equal(t, expected12, mmPeriods, "seluruh 12 periode Januari - Desember harus lengkap tersedia")
	})
}
