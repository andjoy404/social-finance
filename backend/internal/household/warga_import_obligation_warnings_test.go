package household

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"social-finance/internal/auth"
)

// TestImportObligationWarnings_XLSX covers the obligation warnings returned by
// POST /api/v1/warga/import/commit/xlsx:
//   - A: XLSX moves Tanggal Mulai backwards; the commit response must carry the
//     actual activated periods, count, and outstanding total.
//   - X4: identical re-import reports NO new obligations (idempotent).
//   - D: newly obligatory but fully paid bills are not reported as new obligation.
//   - E: import affecting several households produces one warning per affected
//     occupancy, never mixed together.
//   - F: a failing import produces no warnings and leaves no partial warnings
//     or bills behind (transactional rollback).
func TestImportObligationWarnings_XLSX(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupWargaAndFinanceTestRouter(pool)
	ctx := context.Background()

	type obligationWarning struct {
		HouseholdOccupancyID string   `json:"household_occupancy_id"`
		HouseNumber          string   `json:"house_number"`
		ResidentNames        []string `json:"resident_names"`
		ActivatedPeriods     []string `json:"activated_periods"`
		ActivatedBillCount   int      `json:"activated_bill_count"`
		TotalActivatedAmount string   `json:"total_activated_amount"`
	}
	type commitResp struct {
		Status             string              `json:"status"`
		ImportedHouseholds int                 `json:"imported_households"`
		ImportedResidents  int                 `json:"imported_residents"`
		ObligationWarnings []obligationWarning `json:"obligation_warnings"`
	}

	commitAndDecode := func(t *testing.T, token string, xlsxData []byte) (int, commitResp) {
		t.Helper()
		buf, boundary := buildMultipartBody("file", "warga.xlsx", xlsxData)
		req := makeAuthRequest(pool, http.MethodPost, "/api/v1/warga/import/commit/xlsx", token, buf)
		req.Header.Set("Content-Type", boundary)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		var resp commitResp
		if rec.Code == http.StatusCreated {
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode commit response: %v :: %s", err, rec.Body.String())
			}
		}
		if rec.Code != http.StatusCreated && rec.Code != http.StatusBadRequest {
			t.Logf("commit XLSX body: %s", rec.Body.String())
		}
		return rec.Code, resp
	}

	// ── Test A — Mundurkan Tanggal Mulai lewat XLSX ──────────────────────────
	t.Run("A XLSX move backwards reports obligation warnings", func(t *testing.T) {
		rtID := setupTestRT(t, pool, "RT Warn A", 2, fmt.Sprintf("YNA%d", time.Now().UnixNano()%100000))
		pToken := makeTenantToken(t, "pengurus-ywa", "mid-ywa", rtID, auth.RolePengurus)

		dueID := createMultiMonthDueForTest(t, router, pToken, "Iuran Warga A", "100000", 2026)

		_ = uuid.New
		xlsxInitial := buildSingleWargaXLSX("AW-01", "Budi A", "OWNER", "Jl. A1", "Budi A",
			"3171010000007701", "085700000001", "budi.a.warn@test.com", "2026-08-01", "anggota", "Password123!")
		status, _ := commitAndDecode(t, pToken, xlsxInitial)
		if status != http.StatusCreated {
			t.Fatalf("initial import: expected 201, got %d", status)
		}

		xlsxBackwards := buildSingleWargaXLSX("AW-01", "Budi A", "OWNER", "Jl. A1", "Budi A",
			"3171010000007701", "085700000001", "budi.a.warn@test.com", "2026-01-01", "", "")
		status, resp := commitAndDecode(t, pToken, xlsxBackwards)
		if status != http.StatusCreated {
			t.Fatalf("backward import: expected 201, got %d", status)
		}

		if len(resp.ObligationWarnings) != 1 {
			t.Fatalf("expected exactly 1 obligation warning, got %d: %+v", len(resp.ObligationWarnings), resp.ObligationWarnings)
		}
		w := resp.ObligationWarnings[0]
		wantPeriods := []string{"2026-01", "2026-02", "2026-03", "2026-04", "2026-05", "2026-06", "2026-07"}
		if strings.Join(w.ActivatedPeriods, ",") != strings.Join(wantPeriods, ",") {
			t.Errorf("activated_periods = %v, want %v", w.ActivatedPeriods, wantPeriods)
		}
		if w.ActivatedBillCount != len(wantPeriods) {
			t.Errorf("activated_bill_count = %d, want %d", w.ActivatedBillCount, len(wantPeriods))
		}
		if w.TotalActivatedAmount != "700000.00" {
			t.Errorf("total_activated_amount = %q, want %q", w.TotalActivatedAmount, "700000.00")
		}
		if w.HouseNumber != "AW-01" {
			t.Errorf("house_number = %q, want AW-01", w.HouseNumber)
		}
		if len(w.ResidentNames) != 1 || w.ResidentNames[0] != "Budi A" {
			t.Errorf("resident_names = %v, want [Budi A]", w.ResidentNames)
		}

		// Billing state must also be correct: Jan–Jul obligatory Belum Bayar,
		// Aug–Dec untouched.
		var occID string
		if err := pool.Raw().QueryRowContext(ctx,
			`SELECT ho.id FROM household_occupancies ho JOIN physical_houses ph ON ph.id = ho.physical_house_id
			 JOIN households h ON h.id = ho.household_id
			 WHERE ph.house_number = $1 AND ph.rt_id = $2 LIMIT 1`,
			"AW-01", rtID,
		).Scan(&occID); err != nil {
			t.Fatalf("find occupancy: %v", err)
		}
		bills := queryOccupancyBillsByDue(t, pool, occID, dueID)
		if len(bills) != 12 {
			t.Fatalf("expected 12 bills, got %d", len(bills))
		}
		for _, p := range wantPeriods {
			b := bills[p]
			if !b.IsObligatory {
				t.Errorf("period %s: expected is_obligatory=true after backward import", p)
			}
			if b.Status != "unpaid" {
				t.Errorf("period %s: expected status unpaid, got %s", p, b.Status)
			}
		}
	})

	// ── Test X4 — Import ulang identik tidak memuat warning palsu ────────────
	t.Run("X4 identical re-import produces no obligation warnings", func(t *testing.T) {
		rtID := setupTestRT(t, pool, "RT Warn X4", 2, fmt.Sprintf("YNZ%d", time.Now().UnixNano()%100000))
		pToken := makeTenantToken(t, "pengurus-ynz", "mid-ynz", rtID, auth.RolePengurus)
		_ = createMultiMonthDueForTest(t, router, pToken, "Iuran Warga X4", "100000", 2026)

		row := buildSingleWargaXLSX("X4-01", "Wati X4", "OWNER", "Jl. X41", "Wati X4",
			"3171010000007702", "085700000002", "wati.x4@test.com", "2026-04-01", "anggota", "Password123!")

		status, resp := commitAndDecode(t, pToken, row)
		if status != http.StatusCreated {
			t.Fatalf("initial import: got %d", status)
		}
		// Non-obligatory periods Jan–Mar already flipped on first import if any
		// bills exist; on the CURRENT test the first import is identical to the second.
		status, resp = commitAndDecode(t, pToken, row)
		if status != http.StatusCreated {
			t.Fatalf("re-import: got %d", status)
		}
		if len(resp.ObligationWarnings) != 0 {
			t.Fatalf("expected no obligation warnings on identical re-import, got %+v", resp.ObligationWarnings)
		}
	})

	// ── Test D — Tagihan lunas dianggap lunas, warning hanya menghitung baru ──
	t.Run("D fully paid newly-obligatory bills are not reported", func(t *testing.T) {
		rtID := setupTestRT(t, pool, "RT Warn D", 2, fmt.Sprintf("YND%d", time.Now().UnixNano()%100000))
		pToken := makeTenantToken(t, "pengurus-ynd", "mid-ynd", rtID, auth.RolePengurus)

		dueID := createMultiMonthDueForTest(t, router, pToken, "Iuran Warga D", "100000", 2026)

		xlsxInitial := buildSingleWargaXLSX("D-01", "Dana D", "OWNER", "Jl. D1", "Dana D",
			"3171010000007703", "085700000003", "dana.d@test.com", "2026-08-01", "anggota", "Password123!")
		if status, _ := commitAndDecode(t, pToken, xlsxInitial); status != http.StatusCreated {
			t.Fatalf("initial import: got %d", status)
		}

		var occID string
		if err := pool.Raw().QueryRowContext(ctx,
			`SELECT ho.id FROM household_occupancies ho
			 JOIN physical_houses ph ON ph.id = ho.physical_house_id
			JOIN households h ON h.id = ho.household_id
			 WHERE ph.house_number = $1 AND ph.rt_id = $2 LIMIT 1`,
			"D-01", rtID,
		).Scan(&occID); err != nil {
			t.Fatalf("find occupancy: %v", err)
		}

		// Fully pay Jan–Jul (installments of 100000 each) while the bills are
		// not-obligatory / not yet enforced, with APPROVED payments + allocations.
		bills := queryOccupancyBillsByDue(t, pool, occID, dueID)
		paidByAllocation := []struct {
			period string
			amount int
		}{
			{"2026-01", 100000}, {"2026-02", 100000}, {"2026-03", 100000}, {"2026-04", 100000},
			{"2026-05", 100000}, {"2026-06", 100000}, {"2026-07", 100000},
		}
		if _, err := pool.Raw().ExecContext(ctx,
			`UPDATE bills SET status = 'paid' WHERE household_occupancy_id = $1 AND period = ANY($2)`,
			occID, []string{"2026-01", "2026-02", "2026-03", "2026-04", "2026-05", "2026-06", "2026-07"},
		); err != nil {
			t.Fatalf("mark bills paid: %v", err)
		}
		for _, item := range paidByAllocation {
			bill := bills[item.period]
			if bill.ID == "" {
				t.Fatalf("missing bill for period %s", item.period)
			}
			payID := uuid.NewString()
			if _, err := pool.Raw().ExecContext(ctx,
				`INSERT INTO payments (id, rt_id, bill_id, amount, method, origin, status, paid_at)
				 VALUES ($1, $2, $3, $4, 'CASH', 'SELF_SUBMITTED', 'APPROVED', now())`,
				payID, rtID, bill.ID, item.amount,
			); err != nil {
				t.Fatalf("insert payment for %s: %v", item.period, err)
			}
			if _, err := pool.Raw().ExecContext(ctx,
				`INSERT INTO payment_allocations (payment_id, bill_id, amount) VALUES ($1, $2, $3)`,
				payID, bill.ID, item.amount,
			); err != nil {
				t.Fatalf("insert allocation for %s: %v", item.period, err)
			}
		}

		// Mundur ke 1 Januari: bills 2026-01..07 flip ke is_obligatory=true but are fully paid.
		xlsxBackwards := buildSingleWargaXLSX("D-01", "Dana D", "OWNER", "Jl. D1", "Dana D",
			"3171010000007703", "085700000003", "dana.d@test.com", "2026-01-01", "", "")
		status, resp := commitAndDecode(t, pToken, xlsxBackwards)
		if status != http.StatusCreated {
			t.Fatalf("backward import: got %d", status)
		}

		billsAfter := queryOccupancyBillsByDue(t, pool, occID, dueID)
		for _, p := range []string{"2026-01", "2026-02", "2026-03", "2026-04", "2026-05", "2026-06", "2026-07"} {
			if !billsAfter[p].IsObligatory {
				t.Errorf("period %s: expected is_obligatory=true after backward import (paid bills must stay obligatory)", p)
			}
			if billsAfter[p].Status != "paid" {
				t.Errorf("period %s: expected paid status unchanged, got %s", p, billsAfter[p].Status)
			}
		}

		if len(resp.ObligationWarnings) != 0 {
			t.Fatalf("expected no obligation warnings for fully paid flipped bills, got %+v", resp.ObligationWarnings)
		}
	})

	// ── Test E — beberapa warga, warning per occupancy, tidak tertukar ────────
	t.Run("E multi-warga warnings are per-occupancy and not mixed", func(t *testing.T) {
		rtID := setupTestRT(t, pool, "RT Warn E", 2, fmt.Sprintf("YNE%d", time.Now().UnixNano()%100000))
		pToken := makeTenantToken(t, "pengurus-yne", "mid-yne", rtID, auth.RolePengurus)
		_ = createMultiMonthDueForTest(t, router, pToken, "Iuran Warga E", "100000", 2026)

		f := excelize.NewFile()
		defer f.Close()
		writeHeaderRow(f)
		rows := [][]string{
			{"E-01", "Ena E1", "OWNER", "Jl. E1", "Ena E1", "3171010000008801", "085700000011", "ena.e1@test.com", "2026-08-01", "anggota", "Password123!"},
			{"E-02", "Ena E2", "OWNER", "Jl. E2", "Ena E2", "3171010000008802", "085700000012", "ena.e2@test.com", "2026-08-01", "anggota", "Password123!"},
		}
		for i, row := range rows {
			writeRow(f, i, row)
		}
		if status, _ := commitAndDecode(t, pToken, xlsxBytes(f)); status != http.StatusCreated {
			t.Fatalf("initial import: got %d", status)
		}

		// Now move ONLY E-01 backwards, leaving E-02 unchanged.
		f2 := excelize.NewFile()
		defer f2.Close()
		writeHeaderRow(f2)
		rows = [][]string{
			{"E-01", "Ena E1", "OWNER", "Jl. E1", "Ena E1", "3171010000008801", "085700000011", "ena.e1@test.com", "2026-01-01", "", ""},
			{"E-02", "Ena E2", "OWNER", "Jl. E2", "Ena E2", "3171010000008802", "085700000012", "ena.e2@test.com", "2026-08-01", "anggota", "Password123!"},
		}
		for i, row := range rows {
			writeRow(f2, i, row)
		}
		status, resp := commitAndDecode(t, pToken, xlsxBytes(f2))
		if status != http.StatusCreated {
			t.Fatalf("backward import: got %d", status)
		}

		if len(resp.ObligationWarnings) != 1 {
			t.Fatalf("expected exactly 1 warning (only E-01 changed), got %+v", resp.ObligationWarnings)
		}
		w := resp.ObligationWarnings[0]
		if w.HouseNumber != "E-01" {
			t.Errorf("warning house_number = %q, want E-01 (mandiri warning tidak boleh tertukar)", w.HouseNumber)
		}
		if len(w.ActivatedPeriods) != 7 {
			t.Errorf("activated_bill_count = %d, want 7 (%v)", len(w.ActivatedPeriods), w.ActivatedPeriods)
		}
	})

	// ── Test F — import gagal: tidak ada warning sukses ──────────────────────
	t.Run("F failing import produces no warnings and full rollback", func(t *testing.T) {
		rtID := setupTestRT(t, pool, "RT Warn F", 2, fmt.Sprintf("YNF%d", time.Now().UnixNano()%100000))
		pToken := makeTenantToken(t, "pengurus-ynf", "mid-ynf", rtID, auth.RolePengurus)
		_ = createMultiMonthDueForTest(t, router, pToken, "Iuran Warga F", "100000", 2026)

		xlsxInitial := buildSingleWargaXLSX("F-01", "Fajri F", "OWNER", "Jl. F1", "Fajri F",
			"3171010000009901", "085700000021", "fajri.f@test.com", "2026-08-01", "anggota", "Password123!")
		if status, _ := commitAndDecode(t, pToken, xlsxInitial); status != http.StatusCreated {
			t.Fatalf("initial import: got %d", status)
		}

		// Force billing sync to fail during the backward import.
		original := SyncOccupancyBillsFn
		SyncOccupancyBillsFn = func(ctx context.Context, tx *sql.Tx, rtID, occID string) error {
			return fmt.Errorf("injected sync failure")
		}
		defer func() { SyncOccupancyBillsFn = original }()

		xlsxBackwards := buildSingleWargaXLSX("F-01", "Fajri F", "OWNER", "Jl. F1", "Fajri F",
			"3171010000009901", "085700000021", "fajri.f@test.com", "2026-01-01", "", "")
		buf, boundary := buildMultipartBody("file", "warga.xlsx", xlsxBackwards)
		req := makeAuthRequest(pool, http.MethodPost, "/api/v1/warga/import/commit/xlsx", pToken, buf)
		req.Header.Set("Content-Type", boundary)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code == http.StatusCreated {
			t.Fatalf("expected failing import to not return 201, body=%s", rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "obligation_warnings") {
			t.Fatalf("failing import must not leak obligation_warnings: %s", rec.Body.String())
		}

		SyncOccupancyBillsFn = original

		// Bills remain consistent: no partially applied obligation activation.
		var occID string
		if err := pool.Raw().QueryRowContext(ctx,
			`SELECT ho.id FROM household_occupancies ho
			 JOIN physical_houses ph ON ph.id = ho.physical_house_id
			 JOIN households h ON h.id = ho.household_id
			 WHERE ph.house_number = $1 AND ph.rt_id = $2 LIMIT 1`,
			"F-01", rtID,
		).Scan(&occID); err != nil {
			t.Fatalf("find occupancy: %v", err)
		}
		var notObligCount int
		if err := pool.Raw().QueryRowContext(ctx,
			`SELECT COUNT(*) FROM bills WHERE household_occupancy_id = $1 AND period < '2026-08' AND is_obligatory = true`,
			occID,
		).Scan(&notObligCount); err != nil {
			t.Fatalf("count obligatory bills: %v", err)
		}
		if notObligCount != 0 {
			t.Fatalf("expected rollback: 0 of Jan–Jul obligatory after failed import, got %d", notObligCount)
		}
	})
}
