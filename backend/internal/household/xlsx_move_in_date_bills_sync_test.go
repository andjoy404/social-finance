package household

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/xuri/excelize/v2"

	"social-finance/internal/auth"
	"social-finance/internal/database"
	"social-finance/internal/finance"
	"social-finance/internal/storage"
)

// setupWargaAndFinanceTestRouter wires both household (Warga menu + XLSX import)
// and finance (Dues, Bills, Payments) endpoints onto a single router.
func setupWargaAndFinanceTestRouter(pool *database.Pool) http.Handler {
	auth.AuthDBPool = pool
	r := chi.NewRouter()

	hhHandler := NewHandler(pool)
	store := storage.NewMockStorage()
	finSvc := finance.NewService(pool, store)
	finH := finance.NewHandler(pool, finSvc, store)

	// Ensure hook points to finance.SyncHouseholdOccupancyBills
	SyncOccupancyBillsFn = func(ctx context.Context, tx *sql.Tx, rtID, occID string) error {
		_, err := finance.SyncHouseholdOccupancyBills(ctx, tx, rtID, occID)
		return err
	}

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireRole(auth.RoleWarga, auth.RoleBendahara, auth.RolePengurus))
		r.Get("/api/v1/households/{id}", hhHandler.GetHousehold)
		r.Get("/api/v1/residents/{id}", hhHandler.GetResident)
		r.Get("/api/v1/bills", finH.ListBills)
		r.Get("/api/v1/bills/{id}", finH.GetBill)
		r.Post("/api/v1/payments", finH.CreatePayment)
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireRole(auth.RoleBendahara, auth.RolePengurus))
		r.Post("/api/v1/dues", finH.CreateDue)
		r.Post("/api/v1/bills/generate", finH.GenerateBills)
		r.Post("/api/v1/payments/{id}/verify", finH.VerifyPayment)
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireRole(auth.RolePengurus))
		r.Post("/api/v1/households", hhHandler.CreateHousehold)
		r.Patch("/api/v1/households/{id}", hhHandler.UpdateHousehold)
		r.Patch("/api/v1/residents/{id}", hhHandler.UpdateResident)
		r.Post("/api/v1/warga/import/preview/xlsx", HandleWargaImportPreviewXLSX(pool))
		r.Post("/api/v1/warga/import/commit/xlsx", HandleWargaImportCommitXLSX(pool))
		r.Post("/api/v1/warga/import/commit", HandleWargaImportCommit(pool))
	})

	return r
}

func osTempDir() string {
	return "/tmp/social-finance-xlsx-sync-test"
}

func buildSingleWargaXLSX(houseNumber, headName, occStatus, address, resName, nik, phone, email, moveInDate, jabatan, password string) []byte {
	if jabatan == "" && email != "" {
		jabatan = "anggota"
	}
	f := excelize.NewFile()
	defer f.Close()
	writeHeaderRow(f)
	writeRow(f, 0, []string{
		houseNumber, headName, occStatus, address, resName, nik, phone, email, moveInDate, jabatan, password,
	})
	return xlsxBytes(f)
}

func commitXLSXImport(t *testing.T, router http.Handler, pool *database.Pool, token string, xlsxData []byte) {
	t.Helper()
	buf, boundary := buildMultipartBody("file", "warga.xlsx", xlsxData)
	req := makeAuthRequest(pool, http.MethodPost, "/api/v1/warga/import/commit/xlsx", token, buf)
	req.Header.Set("Content-Type", boundary)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created from XLSX import commit, got %d: %s", rec.Code, rec.Body.String())
	}
}

func createMultiMonthDueForTest(t *testing.T, router http.Handler, token, name, amount string, year int) string {
	t.Helper()
	body := fmt.Sprintf(`{"name":%q,"amount":%q,"due_type":"multi_month","year":%d,"period_type":"monthly","due_date":"%04d-01-10"}`, name, amount, year, year)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create due %s (%d): expected 201, got %d: %s", name, year, rec.Code, rec.Body.String())
	}
	var d struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatalf("unmarshal due: %v", err)
	}
	return d.ID
}

type billState struct {
	ID           string
	Period       string
	Status       string
	IsObligatory bool
	Amount       string
}

func queryOccupancyBillsByDue(t *testing.T, pool *database.Pool, occID, dueID string) map[string]billState {
	t.Helper()
	rows, err := pool.Raw().QueryContext(context.Background(), `
		SELECT id, period, status, is_obligatory, amount::text
		FROM bills
		WHERE household_occupancy_id = $1 AND due_id = $2
		ORDER BY period ASC
	`, occID, dueID)
	if err != nil {
		t.Fatalf("query bills: %v", err)
	}
	defer rows.Close()

	res := make(map[string]billState)
	for rows.Next() {
		var b billState
		if err := rows.Scan(&b.ID, &b.Period, &b.Status, &b.IsObligatory, &b.Amount); err != nil {
			t.Fatalf("scan bill: %v", err)
		}
		res[b.Period] = b
	}
	return res
}

// TestXLSXMoveInDateBillsSync_AllScenarios covers Test A through Test K from the
// bug fix specification for synchronizing Warga Tanggal Mulai via XLSX import.
func TestXLSXMoveInDateBillsSync_AllScenarios(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupWargaAndFinanceTestRouter(pool)
	ctx := context.Background()

	// -------------------------------------------------------------------------
	// Test A — Mundurkan Tanggal Mulai lewat XLSX (1 April 2026 -> 1 Januari 2026)
	// -------------------------------------------------------------------------
	t.Run("Test A - Mundurkan Tanggal Mulai lewat XLSX", func(t *testing.T) {
		rtID := setupTestRT(t, pool, "RT XLSX Sync A", 1, fmt.Sprintf("XA%d", time.Now().UnixNano()%100000))
		pToken := makeTenantToken(t, "pengurus-xa", "mid-xa", rtID, auth.RolePengurus)

		dueID := createMultiMonthDueForTest(t, router, pToken, "Iuran Bulanan 2026", "50000", 2026)

		nik := fmt.Sprintf("320101%010d", time.Now().UnixNano()%10000000000)
		phone := fmt.Sprintf("0812%08d", time.Now().UnixNano()%100000000)
		email := fmt.Sprintf("warga.a.%d@test.com", time.Now().UnixNano())

		// 1. Initial import with Tanggal Mulai = 2026-04-01
		xlsxInitial := buildSingleWargaXLSX("A-01", "Budi A", "OWNER", "Jl. A1", "Budi A", nik, phone, email, "2026-04-01", "", "")
		commitXLSXImport(t, router, pool, pToken, xlsxInitial)

		var occID, resID string
		err := pool.Raw().QueryRowContext(ctx, `
			SELECT ho.id, r.id
			FROM residents r
			JOIN residency_periods rp ON rp.resident_id = r.id AND rp.end_date IS NULL
			JOIN household_occupancies ho ON ho.id = rp.household_occupancy_id AND ho.end_date IS NULL
			WHERE r.rt_id = $1 AND r.nik = $2
		`, rtID, nik).Scan(&occID, &resID)
		if err != nil {
			t.Fatalf("lookup occupancy and resident: %v", err)
		}

		// Verify initial state: Jan-Mar Tidak Wajib (is_obligatory=false), Apr-Dec Belum Bayar (is_obligatory=true, status='unpaid')
		billsBefore := queryOccupancyBillsByDue(t, pool, occID, dueID)
		if len(billsBefore) != 12 {
			t.Fatalf("expected 12 bills initially, got %d", len(billsBefore))
		}
		for m := 1; m <= 12; m++ {
			period := fmt.Sprintf("2026-%02d", m)
			b := billsBefore[period]
			wantOblig := m >= 4
			if b.IsObligatory != wantOblig {
				t.Fatalf("before update period %s: expected is_obligatory=%v, got %v", period, wantOblig, b.IsObligatory)
			}
			if b.Status != "unpaid" {
				t.Fatalf("before update period %s: expected status=unpaid, got %s", period, b.Status)
			}
		}

		// 2. Import XLSX changing Tanggal Mulai backwards to 2026-01-01
		xlsxUpdated := buildSingleWargaXLSX("A-01", "Budi A", "OWNER", "Jl. A1", "Budi A", nik, phone, email, "2026-01-01", "", "")
		commitXLSXImport(t, router, pool, pToken, xlsxUpdated)

		// Verify both residency_periods.start_date and household_occupancies.start_date are 2026-01-01
		var rpStart, hoStart string
		_ = pool.Raw().QueryRowContext(ctx, `SELECT to_char(start_date, 'YYYY-MM-DD') FROM residency_periods WHERE resident_id = $1 AND end_date IS NULL`, resID).Scan(&rpStart)
		_ = pool.Raw().QueryRowContext(ctx, `SELECT to_char(start_date, 'YYYY-MM-DD') FROM household_occupancies WHERE id = $1`, occID).Scan(&hoStart)
		if rpStart != "2026-01-01" || hoStart != "2026-01-01" {
			t.Fatalf("expected rp.start_date and ho.start_date to be 2026-01-01, got rp=%s ho=%s", rpStart, hoStart)
		}

		// Verify Jan-Dec 2026 are all Belum Bayar (is_obligatory=true, status='unpaid')
		billsAfter := queryOccupancyBillsByDue(t, pool, occID, dueID)
		if len(billsAfter) != 12 {
			t.Fatalf("expected 12 bills after XLSX update, got %d", len(billsAfter))
		}
		for m := 1; m <= 12; m++ {
			period := fmt.Sprintf("2026-%02d", m)
			b := billsAfter[period]
			if !b.IsObligatory {
				t.Errorf("after XLSX update period %s: expected is_obligatory=true (Belum Bayar), got false (Tidak Wajib)", period)
			}
			if b.Status != "unpaid" {
				t.Errorf("after XLSX update period %s: expected status=unpaid, got %s", period, b.Status)
			}
		}
	})

	// -------------------------------------------------------------------------
	// Test B — Mundurkan Tanggal Mulai lewat menu Warga (Hasil identik dengan Test A)
	// -------------------------------------------------------------------------
	t.Run("Test B - Mundurkan Tanggal Mulai lewat menu Warga identik dengan Test A", func(t *testing.T) {
		rtID := setupTestRT(t, pool, "RT XLSX Sync B", 1, fmt.Sprintf("XB%d", time.Now().UnixNano()%100000))
		pToken := makeTenantToken(t, "pengurus-xb", "mid-xb", rtID, auth.RolePengurus)

		dueID := createMultiMonthDueForTest(t, router, pToken, "Iuran Bulanan 2026", "50000", 2026)

		nikA := fmt.Sprintf("320102%010d", time.Now().UnixNano()%10000000000)
		phoneA := fmt.Sprintf("0813%08d", time.Now().UnixNano()%100000000)
		emailA := fmt.Sprintf("warga.ba.%d@test.com", time.Now().UnixNano())

		nikB := fmt.Sprintf("320103%010d", time.Now().UnixNano()%10000000000)
		phoneB := fmt.Sprintf("0814%08d", time.Now().UnixNano()%100000000)
		emailB := fmt.Sprintf("warga.bb.%d@test.com", time.Now().UnixNano())

		// Household 1 updated via XLSX
		commitXLSXImport(t, router, pool, pToken, buildSingleWargaXLSX("B-01", "Warga Via XLSX", "OWNER", "Jl. B1", "Warga Via XLSX", nikA, phoneA, emailA, "2026-04-01", "", ""))
		commitXLSXImport(t, router, pool, pToken, buildSingleWargaXLSX("B-01", "Warga Via XLSX", "OWNER", "Jl. B1", "Warga Via XLSX", nikA, phoneA, emailA, "2026-01-01", "", ""))

		// Household 2 created with 2026-04-01 and updated via Warga menu (PATCH /api/v1/households/{id})
		commitXLSXImport(t, router, pool, pToken, buildSingleWargaXLSX("B-02", "Warga Via Menu", "OWNER", "Jl. B2", "Warga Via Menu", nikB, phoneB, emailB, "2026-04-01", "", ""))

		var hhID2, occID1, occID2 string
		_ = pool.Raw().QueryRowContext(ctx, `SELECT ho.id FROM residents r JOIN residency_periods rp ON rp.resident_id = r.id JOIN household_occupancies ho ON ho.id = rp.household_occupancy_id WHERE r.rt_id = $1 AND r.nik = $2`, rtID, nikA).Scan(&occID1)
		_ = pool.Raw().QueryRowContext(ctx, `SELECT ho.household_id, ho.id FROM residents r JOIN residency_periods rp ON rp.resident_id = r.id JOIN household_occupancies ho ON ho.id = rp.household_occupancy_id WHERE r.rt_id = $1 AND r.nik = $2`, rtID, nikB).Scan(&hhID2, &occID2)

		patchBody := `{"start_date":"2026-01-01"}`
		reqPatch := httptest.NewRequest(http.MethodPatch, "/api/v1/households/"+hhID2, bytes.NewBufferString(patchBody))
		reqPatch.Header.Set("Authorization", "Bearer "+pToken)
		reqPatch.Header.Set("Content-Type", "application/json")
		recPatch := httptest.NewRecorder()
		router.ServeHTTP(recPatch, reqPatch)
		if recPatch.Code != http.StatusOK {
			t.Fatalf("PATCH household expected 200, got %d: %s", recPatch.Code, recPatch.Body.String())
		}

		billsXLSX := queryOccupancyBillsByDue(t, pool, occID1, dueID)
		billsMenu := queryOccupancyBillsByDue(t, pool, occID2, dueID)

		if len(billsXLSX) != len(billsMenu) {
			t.Fatalf("bill count mismatch: xlsx=%d menu=%d", len(billsXLSX), len(billsMenu))
		}
		for m := 1; m <= 12; m++ {
			period := fmt.Sprintf("2026-%02d", m)
			bx := billsXLSX[period]
			bm := billsMenu[period]
			if bx.Status != bm.Status || bx.IsObligatory != bm.IsObligatory || bx.Amount != bm.Amount {
				t.Errorf("period %s mismatch between XLSX (%+v) and Menu Warga (%+v)", period, bx, bm)
			}
		}
	})

	// -------------------------------------------------------------------------
	// Test C — Tagihan sudah Lunas (tidak berubah menjadi Belum Bayar)
	// -------------------------------------------------------------------------
	t.Run("Test C - Tagihan sudah Lunas tetap Lunas", func(t *testing.T) {
		rtID := setupTestRT(t, pool, "RT XLSX Sync C", 1, fmt.Sprintf("XC%d", time.Now().UnixNano()%100000))
		pToken := makeTenantToken(t, "pengurus-xc", "mid-xc", rtID, auth.RolePengurus)

		dueID := createMultiMonthDueForTest(t, router, pToken, "Iuran Bulanan 2026", "50000", 2026)

		nik := fmt.Sprintf("320104%010d", time.Now().UnixNano()%10000000000)
		phone := fmt.Sprintf("0815%08d", time.Now().UnixNano()%100000000)
		email := fmt.Sprintf("warga.c.%d@test.com", time.Now().UnixNano())

		commitXLSXImport(t, router, pool, pToken, buildSingleWargaXLSX("C-01", "Warga C", "OWNER", "Jl. C1", "Warga C", nik, phone, email, "2026-04-01", "", ""))

		var occID string
		_ = pool.Raw().QueryRowContext(ctx, `SELECT ho.id FROM residents r JOIN residency_periods rp ON rp.resident_id = r.id JOIN household_occupancies ho ON ho.id = rp.household_occupancy_id WHERE r.rt_id = $1 AND r.nik = $2`, rtID, nik).Scan(&occID)

		// Mark Februari 2026 and April 2026 as Lunas ('paid')
		_, err := pool.Raw().ExecContext(ctx, `
			UPDATE bills SET status = 'paid', is_obligatory = true
			WHERE household_occupancy_id = $1 AND due_id = $2 AND period IN ('2026-02', '2026-04')
		`, occID, dueID)
		if err != nil {
			t.Fatalf("mark bills paid: %v", err)
		}

		// Import XLSX moving Tanggal Mulai to 2026-01-01
		commitXLSXImport(t, router, pool, pToken, buildSingleWargaXLSX("C-01", "Warga C", "OWNER", "Jl. C1", "Warga C", nik, phone, email, "2026-01-01", "", ""))

		bills := queryOccupancyBillsByDue(t, pool, occID, dueID)
		if bills["2026-02"].Status != "paid" || !bills["2026-02"].IsObligatory {
			t.Errorf("2026-02 should remain paid & obligatory, got %+v", bills["2026-02"])
		}
		if bills["2026-04"].Status != "paid" || !bills["2026-04"].IsObligatory {
			t.Errorf("2026-04 should remain paid & obligatory, got %+v", bills["2026-04"])
		}
		if bills["2026-01"].Status != "unpaid" || !bills["2026-01"].IsObligatory {
			t.Errorf("2026-01 should become unpaid & obligatory, got %+v", bills["2026-01"])
		}
		if bills["2026-03"].Status != "unpaid" || !bills["2026-03"].IsObligatory {
			t.Errorf("2026-03 should become unpaid & obligatory, got %+v", bills["2026-03"])
		}
	})

	// -------------------------------------------------------------------------
	// Test D — Record tagihan sudah ada (diupdate in-place tanpa duplikat)
	// -------------------------------------------------------------------------
	t.Run("Test D - Record tagihan sudah ada diupdate tanpa duplikat", func(t *testing.T) {
		rtID := setupTestRT(t, pool, "RT XLSX Sync D", 1, fmt.Sprintf("XD%d", time.Now().UnixNano()%100000))
		pToken := makeTenantToken(t, "pengurus-xd", "mid-xd", rtID, auth.RolePengurus)

		dueID := createMultiMonthDueForTest(t, router, pToken, "Iuran Bulanan 2026", "50000", 2026)

		nik := fmt.Sprintf("320105%010d", time.Now().UnixNano()%10000000000)
		phone := fmt.Sprintf("0816%08d", time.Now().UnixNano()%100000000)
		email := fmt.Sprintf("warga.d.%d@test.com", time.Now().UnixNano())

		commitXLSXImport(t, router, pool, pToken, buildSingleWargaXLSX("D-01", "Warga D", "OWNER", "Jl. D1", "Warga D", nik, phone, email, "2026-04-01", "", ""))

		var occID string
		_ = pool.Raw().QueryRowContext(ctx, `SELECT ho.id FROM residents r JOIN residency_periods rp ON rp.resident_id = r.id JOIN household_occupancies ho ON ho.id = rp.household_occupancy_id WHERE r.rt_id = $1 AND r.nik = $2`, rtID, nik).Scan(&occID)

		billsBefore := queryOccupancyBillsByDue(t, pool, occID, dueID)

		// Update Tanggal Mulai to 2026-01-01 via XLSX
		commitXLSXImport(t, router, pool, pToken, buildSingleWargaXLSX("D-01", "Warga D", "OWNER", "Jl. D1", "Warga D", nik, phone, email, "2026-01-01", "", ""))

		billsAfter := queryOccupancyBillsByDue(t, pool, occID, dueID)
		if len(billsAfter) != 12 {
			t.Fatalf("expected 12 bills, got %d", len(billsAfter))
		}
		for _, p := range []string{"2026-01", "2026-02", "2026-03"} {
			if billsBefore[p].ID != billsAfter[p].ID {
				t.Errorf("period %s bill ID changed (%s -> %s); expected in-place update", p, billsBefore[p].ID, billsAfter[p].ID)
			}
			if !billsAfter[p].IsObligatory || billsAfter[p].Status != "unpaid" {
				t.Errorf("period %s expected unpaid+obligatory, got %+v", p, billsAfter[p])
			}
		}
	})

	// -------------------------------------------------------------------------
	// Test E — Record tagihan belum ada (dibuat otomatis dengan status Belum Bayar)
	// -------------------------------------------------------------------------
	t.Run("Test E - Record tagihan belum ada dibuat otomatis", func(t *testing.T) {
		rtID := setupTestRT(t, pool, "RT XLSX Sync E", 1, fmt.Sprintf("XE%d", time.Now().UnixNano()%100000))
		pToken := makeTenantToken(t, "pengurus-xe", "mid-xe", rtID, auth.RolePengurus)

		dueID := createMultiMonthDueForTest(t, router, pToken, "Iuran Bulanan 2026", "50000", 2026)

		nik := fmt.Sprintf("320106%010d", time.Now().UnixNano()%10000000000)
		phone := fmt.Sprintf("0817%08d", time.Now().UnixNano()%100000000)
		email := fmt.Sprintf("warga.e.%d@test.com", time.Now().UnixNano())

		commitXLSXImport(t, router, pool, pToken, buildSingleWargaXLSX("E-01", "Warga E", "OWNER", "Jl. E1", "Warga E", nik, phone, email, "2026-04-01", "", ""))

		var occID string
		_ = pool.Raw().QueryRowContext(ctx, `SELECT ho.id FROM residents r JOIN residency_periods rp ON rp.resident_id = r.id JOIN household_occupancies ho ON ho.id = rp.household_occupancy_id WHERE r.rt_id = $1 AND r.nik = $2`, rtID, nik).Scan(&occID)

		// Simulate missing bill records for Jan-Mar 2026
		_, err := pool.Raw().ExecContext(ctx, `
			DELETE FROM bills
			WHERE household_occupancy_id = $1 AND due_id = $2 AND period IN ('2026-01', '2026-02', '2026-03')
		`, occID, dueID)
		if err != nil {
			t.Fatalf("delete Jan-Mar bills: %v", err)
		}

		// Import XLSX moving Tanggal Mulai backwards to 2026-01-01
		commitXLSXImport(t, router, pool, pToken, buildSingleWargaXLSX("E-01", "Warga E", "OWNER", "Jl. E1", "Warga E", nik, phone, email, "2026-01-01", "", ""))

		billsAfter := queryOccupancyBillsByDue(t, pool, occID, dueID)
		if len(billsAfter) != 12 {
			t.Fatalf("expected all 12 bills to exist after sync, got %d", len(billsAfter))
		}
		for _, p := range []string{"2026-01", "2026-02", "2026-03"} {
			b, ok := billsAfter[p]
			if !ok {
				t.Fatalf("missing bill for %s", p)
			}
			if !b.IsObligatory || b.Status != "unpaid" {
				t.Errorf("newly created bill %s expected status=unpaid is_obligatory=true, got %+v", p, b)
			}
		}
	})

	// -------------------------------------------------------------------------
	// Test F — Majukan Tanggal Mulai (1 Januari 2026 -> 1 April 2026)
	// -------------------------------------------------------------------------
	t.Run("Test F - Majukan Tanggal Mulai lewat XLSX", func(t *testing.T) {
		rtID := setupTestRT(t, pool, "RT XLSX Sync F", 1, fmt.Sprintf("XF%d", time.Now().UnixNano()%100000))
		pToken := makeTenantToken(t, "pengurus-xf", "mid-xf", rtID, auth.RolePengurus)

		dueID := createMultiMonthDueForTest(t, router, pToken, "Iuran Bulanan 2026", "50000", 2026)

		nik := fmt.Sprintf("320107%010d", time.Now().UnixNano()%10000000000)
		phone := fmt.Sprintf("0818%08d", time.Now().UnixNano()%100000000)
		email := fmt.Sprintf("warga.f.%d@test.com", time.Now().UnixNano())

		// 1. Initial Tanggal Mulai = 2026-01-01
		commitXLSXImport(t, router, pool, pToken, buildSingleWargaXLSX("F-01", "Warga F", "OWNER", "Jl. F1", "Warga F", nik, phone, email, "2026-01-01", "", ""))

		var occID string
		_ = pool.Raw().QueryRowContext(ctx, `SELECT ho.id FROM residents r JOIN residency_periods rp ON rp.resident_id = r.id JOIN household_occupancies ho ON ho.id = rp.household_occupancy_id WHERE r.rt_id = $1 AND r.nik = $2`, rtID, nik).Scan(&occID)

		// Move Tanggal Mulai forward to 2026-04-01 via XLSX
		commitXLSXImport(t, router, pool, pToken, buildSingleWargaXLSX("F-01", "Warga F", "OWNER", "Jl. F1", "Warga F", nik, phone, email, "2026-04-01", "", ""))

		billsAfter := queryOccupancyBillsByDue(t, pool, occID, dueID)
		for m := 1; m <= 12; m++ {
			period := fmt.Sprintf("2026-%02d", m)
			b := billsAfter[period]
			wantOblig := m >= 4
			if b.IsObligatory != wantOblig {
				t.Errorf("period %s: expected is_obligatory=%v after moving start_date forward to 2026-04-01, got %v", period, wantOblig, b.IsObligatory)
			}
		}
	})

	// -------------------------------------------------------------------------
	// Test G — Import tanpa perubahan tanggal (tanggal sama atau kosong)
	// -------------------------------------------------------------------------
	t.Run("Test G - Import tanpa perubahan tanggal atau tanggal kosong", func(t *testing.T) {
		rtID := setupTestRT(t, pool, "RT XLSX Sync G", 1, fmt.Sprintf("XG%d", time.Now().UnixNano()%100000))
		pToken := makeTenantToken(t, "pengurus-xg", "mid-xg", rtID, auth.RolePengurus)

		dueID := createMultiMonthDueForTest(t, router, pToken, "Iuran Bulanan 2026", "50000", 2026)

		nik := fmt.Sprintf("320108%010d", time.Now().UnixNano()%10000000000)
		phone := fmt.Sprintf("0819%08d", time.Now().UnixNano()%100000000)
		email := fmt.Sprintf("warga.g.%d@test.com", time.Now().UnixNano())

		commitXLSXImport(t, router, pool, pToken, buildSingleWargaXLSX("G-01", "Warga G", "OWNER", "Jl. G1", "Warga G", nik, phone, email, "2026-04-01", "", ""))

		var occID string
		_ = pool.Raw().QueryRowContext(ctx, `SELECT ho.id FROM residents r JOIN residency_periods rp ON rp.resident_id = r.id JOIN household_occupancies ho ON ho.id = rp.household_occupancy_id WHERE r.rt_id = $1 AND r.nik = $2`, rtID, nik).Scan(&occID)

		// Re-import with empty move_in_date ("")
		commitXLSXImport(t, router, pool, pToken, buildSingleWargaXLSX("G-01", "Warga G", "OWNER", "Jl. G1 Updated", "Warga G", nik, phone, email, "", "", ""))

		// Re-import with identical move_in_date ("2026-04-01")
		commitXLSXImport(t, router, pool, pToken, buildSingleWargaXLSX("G-01", "Warga G", "OWNER", "Jl. G1 Updated", "Warga G", nik, phone, email, "2026-04-01", "", ""))

		bills := queryOccupancyBillsByDue(t, pool, occID, dueID)
		for m := 1; m <= 12; m++ {
			period := fmt.Sprintf("2026-%02d", m)
			wantOblig := m >= 4
			if bills[period].IsObligatory != wantOblig {
				t.Errorf("period %s: expected is_obligatory=%v, got %v", period, wantOblig, bills[period].IsObligatory)
			}
		}
	})

	// -------------------------------------------------------------------------
	// Test H — Import ulang file yang sama (Idempoten, tidak ada duplikat)
	// -------------------------------------------------------------------------
	t.Run("Test H - Import ulang file XLSX yang sama dua kali idempoten", func(t *testing.T) {
		rtID := setupTestRT(t, pool, "RT XLSX Sync H", 1, fmt.Sprintf("XH%d", time.Now().UnixNano()%100000))
		pToken := makeTenantToken(t, "pengurus-xh", "mid-xh", rtID, auth.RolePengurus)

		dueID := createMultiMonthDueForTest(t, router, pToken, "Iuran Bulanan 2026", "50000", 2026)

		nik := fmt.Sprintf("320109%010d", time.Now().UnixNano()%10000000000)
		phone := fmt.Sprintf("0821%08d", time.Now().UnixNano()%100000000)
		email := fmt.Sprintf("warga.h.%d@test.com", time.Now().UnixNano())

		commitXLSXImport(t, router, pool, pToken, buildSingleWargaXLSX("H-01", "Warga H", "OWNER", "Jl. H1", "Warga H", nik, phone, email, "2026-04-01", "", ""))

		xlsxBackwards := buildSingleWargaXLSX("H-01", "Warga H", "OWNER", "Jl. H1", "Warga H", nik, phone, email, "2026-01-01", "", "")
		commitXLSXImport(t, router, pool, pToken, xlsxBackwards)
		commitXLSXImport(t, router, pool, pToken, xlsxBackwards)

		var occID string
		_ = pool.Raw().QueryRowContext(ctx, `SELECT ho.id FROM residents r JOIN residency_periods rp ON rp.resident_id = r.id JOIN household_occupancies ho ON ho.id = rp.household_occupancy_id WHERE r.rt_id = $1 AND r.nik = $2`, rtID, nik).Scan(&occID)

		var totalBills int
		_ = pool.Raw().QueryRowContext(ctx, `SELECT COUNT(*) FROM bills WHERE household_occupancy_id = $1 AND due_id = $2`, occID, dueID).Scan(&totalBills)
		if totalBills != 12 {
			t.Fatalf("expected exactly 12 bills after duplicate XLSX imports, got %d", totalBills)
		}
	})

	// -------------------------------------------------------------------------
	// Test I — FIFO pembayaran setelah perubahan Tanggal Mulai lewat XLSX
	// -------------------------------------------------------------------------
	t.Run("Test I - FIFO pembayaran kompatibel dengan perubahan Tanggal Mulai via XLSX", func(t *testing.T) {
		rtID := setupTestRT(t, pool, "RT XLSX Sync I", 1, fmt.Sprintf("XI%d", time.Now().UnixNano()%100000))
		pToken := makeTenantToken(t, "pengurus-xi", "mid-xi", rtID, auth.RolePengurus)

		dueID := createMultiMonthDueForTest(t, router, pToken, "Iuran FIFO 2026", "50000", 2026)

		nik := fmt.Sprintf("320110%010d", time.Now().UnixNano()%10000000000)
		phone := fmt.Sprintf("0822%08d", time.Now().UnixNano()%100000000)
		email := fmt.Sprintf("warga.i.%d@test.com", time.Now().UnixNano())

		// 1. Import resident with Tanggal Mulai = 2026-04-01 and create user account via password
		commitXLSXImport(t, router, pool, pToken, buildSingleWargaXLSX("I-01", "Warga I", "OWNER", "Jl. I1", "Warga I", nik, phone, email, "2026-04-01", "anggota", "Password123!"))

		var occID, userID, midID string
		_ = pool.Raw().QueryRowContext(ctx, `
			SELECT ho.id, r.user_id, m.id
			FROM residents r
			JOIN residency_periods rp ON rp.resident_id = r.id AND rp.end_date IS NULL
			JOIN household_occupancies ho ON ho.id = rp.household_occupancy_id AND ho.end_date IS NULL
			JOIN user_rt_memberships m ON m.user_id = r.user_id AND m.rt_id = r.rt_id AND m.is_active = true
			WHERE r.rt_id = $1 AND r.nik = $2
		`, rtID, nik).Scan(&occID, &userID, &midID)

		wargaToken := makeTenantToken(t, userID, midID, rtID, auth.RoleWarga)

		// Move Tanggal Mulai backwards to 2026-01-01 via XLSX
		commitXLSXImport(t, router, pool, pToken, buildSingleWargaXLSX("I-01", "Warga I", "OWNER", "Jl. I1", "Warga I", nik, phone, email, "2026-01-01", "", ""))

		bills := queryOccupancyBillsByDue(t, pool, occID, dueID)

		// Attempting to pay April 2026 before January 2026 must now be rejected by FIFO!
		payAprBody := fmt.Sprintf(`{"bill_id":%q,"amount":"50000","method":"TRANSFER","proof_path":"proofs/test.jpg"}`, bills["2026-04"].ID)
		reqApr := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(payAprBody))
		reqApr.Header.Set("Authorization", "Bearer "+wargaToken)
		reqApr.Header.Set("Content-Type", "application/json")
		recApr := httptest.NewRecorder()
		router.ServeHTTP(recApr, reqApr)
		if recApr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 FIFO rejection when paying 2026-04 before 2026-01, got %d: %s", recApr.Code, recApr.Body.String())
		}

		// Paying January 2026 (now the oldest obligatory unpaid bill) must succeed!
		payJanBody := fmt.Sprintf(`{"bill_id":%q,"amount":"50000","method":"TRANSFER","proof_path":"proofs/test.jpg"}`, bills["2026-01"].ID)
		reqJan := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(payJanBody))
		reqJan.Header.Set("Authorization", "Bearer "+wargaToken)
		reqJan.Header.Set("Content-Type", "application/json")
		recJan := httptest.NewRecorder()
		router.ServeHTTP(recJan, reqJan)
		if recJan.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created when paying 2026-01 (oldest obligatory bill), got %d: %s", recJan.Code, recJan.Body.String())
		}
	})

	// -------------------------------------------------------------------------
	// Test J — Perubahan Tanggal Mulai lintas tahun (2026-03-01 <-> 2025-10-01)
	// -------------------------------------------------------------------------
	t.Run("Test J - Perubahan Tanggal Mulai lintas tahun via XLSX", func(t *testing.T) {
		rtID := setupTestRT(t, pool, "RT XLSX Sync J", 1, fmt.Sprintf("XJ%d", time.Now().UnixNano()%100000))
		pToken := makeTenantToken(t, "pengurus-xj", "mid-xj", rtID, auth.RolePengurus)

		due2025 := createMultiMonthDueForTest(t, router, pToken, "Iuran 2025", "40000", 2025)
		due2026 := createMultiMonthDueForTest(t, router, pToken, "Iuran 2026", "50000", 2026)

		nik := fmt.Sprintf("320111%010d", time.Now().UnixNano()%10000000000)
		phone := fmt.Sprintf("0823%08d", time.Now().UnixNano()%100000000)
		email := fmt.Sprintf("warga.j.%d@test.com", time.Now().UnixNano())

		// 1. Initial import: 2026-03-01 (no 2025 obligation; 2026 Jan-Feb Tidak Wajib, Mar-Dec Belum Bayar)
		commitXLSXImport(t, router, pool, pToken, buildSingleWargaXLSX("J-01", "Warga J", "OWNER", "Jl. J1", "Warga J", nik, phone, email, "2026-03-01", "", ""))

		var occID string
		_ = pool.Raw().QueryRowContext(ctx, `SELECT ho.id FROM residents r JOIN residency_periods rp ON rp.resident_id = r.id JOIN household_occupancies ho ON ho.id = rp.household_occupancy_id WHERE r.rt_id = $1 AND r.nik = $2`, rtID, nik).Scan(&occID)

		// 2. Move backwards across year to 2025-10-01 via XLSX
		commitXLSXImport(t, router, pool, pToken, buildSingleWargaXLSX("J-01", "Warga J", "OWNER", "Jl. J1", "Warga J", nik, phone, email, "2025-10-01", "", ""))

		bills2025 := queryOccupancyBillsByDue(t, pool, occID, due2025)
		bills2026 := queryOccupancyBillsByDue(t, pool, occID, due2026)
		if len(bills2025) != 12 {
			t.Fatalf("expected 12 bills generated for 2025 after moving start_date to 2025-10-01, got %d", len(bills2025))
		}
		for m := 1; m <= 12; m++ {
			p25 := fmt.Sprintf("2025-%02d", m)
			want25 := m >= 10
			if bills2025[p25].IsObligatory != want25 {
				t.Errorf("period %s: expected is_obligatory=%v, got %v", p25, want25, bills2025[p25].IsObligatory)
			}
			p26 := fmt.Sprintf("2026-%02d", m)
			if !bills2026[p26].IsObligatory {
				t.Errorf("period %s: expected is_obligatory=true, got false", p26)
			}
		}

		// 3. Move forwards across year back to 2026-03-01 via XLSX
		commitXLSXImport(t, router, pool, pToken, buildSingleWargaXLSX("J-01", "Warga J", "OWNER", "Jl. J1", "Warga J", nik, phone, email, "2026-03-01", "", ""))

		bills2025After := queryOccupancyBillsByDue(t, pool, occID, due2025)
		bills2026After := queryOccupancyBillsByDue(t, pool, occID, due2026)
		for m := 1; m <= 12; m++ {
			p25 := fmt.Sprintf("2025-%02d", m)
			if bills2025After[p25].IsObligatory {
				t.Errorf("period %s: expected is_obligatory=false after moving start_date to 2026-03-01, got true", p25)
			}
			p26 := fmt.Sprintf("2026-%02d", m)
			want26 := m >= 3
			if bills2026After[p26].IsObligatory != want26 {
				t.Errorf("period %s: expected is_obligatory=%v, got %v", p26, want26, bills2026After[p26].IsObligatory)
			}
		}
	})

	// -------------------------------------------------------------------------
	// Test K — Pembayaran parsial atau status khusus tetap konsisten
	// -------------------------------------------------------------------------
	t.Run("Test K - Pembayaran parsial dan pembayaran aktif tetap konsisten", func(t *testing.T) {
		rtID := setupTestRT(t, pool, "RT XLSX Sync K", 1, fmt.Sprintf("XK%d", time.Now().UnixNano()%100000))
		pToken := makeTenantToken(t, "pengurus-xk", "mid-xk", rtID, auth.RolePengurus)

		dueID := createMultiMonthDueForTest(t, router, pToken, "Iuran Parsial 2026", "50000", 2026)

		nik := fmt.Sprintf("320112%010d", time.Now().UnixNano()%10000000000)
		phone := fmt.Sprintf("0824%08d", time.Now().UnixNano()%100000000)
		email := fmt.Sprintf("warga.k.%d@test.com", time.Now().UnixNano())

		commitXLSXImport(t, router, pool, pToken, buildSingleWargaXLSX("K-01", "Warga K", "OWNER", "Jl. K1", "Warga K", nik, phone, email, "2026-01-01", "", ""))

		var occID string
		_ = pool.Raw().QueryRowContext(ctx, `SELECT ho.id FROM residents r JOIN residency_periods rp ON rp.resident_id = r.id JOIN household_occupancies ho ON ho.id = rp.household_occupancy_id WHERE r.rt_id = $1 AND r.nik = $2`, rtID, nik).Scan(&occID)

		bills := queryOccupancyBillsByDue(t, pool, occID, dueID)

		// Simulate:
		// - 2026-01: unpaid bill with a REJECTED payment (should be allowed to become Tidak Wajib when start_date moves to 2026-04-01)
		// - 2026-02: partially paid bill (Rp 20.000 APPROVED) -> must stay partial + is_obligatory=true
		// - 2026-03: unpaid bill with a PENDING payment via payment_allocations -> must stay unpaid + is_obligatory=true
		_, err := pool.Raw().ExecContext(ctx, `
			INSERT INTO payments (rt_id, bill_id, amount, method, origin, status, rejection_reason)
			VALUES ($1, $2, 50000, 'TRANSFER', 'SELF_SUBMITTED', 'REJECTED', 'Bukti buram')
		`, rtID, bills["2026-01"].ID)
		if err != nil {
			t.Fatalf("insert rejected payment on 2026-01: %v", err)
		}

		_, err = pool.Raw().ExecContext(ctx, `
			UPDATE bills SET status = 'partial', is_obligatory = true WHERE id = $1
		`, bills["2026-02"].ID)
		if err != nil {
			t.Fatalf("set bill partial: %v", err)
		}
		_, err = pool.Raw().ExecContext(ctx, `
			INSERT INTO payments (rt_id, bill_id, amount, method, origin, status)
			VALUES ($1, $2, 20000, 'CASH', 'SELF_SUBMITTED', 'APPROVED')
		`, rtID, bills["2026-02"].ID)
		if err != nil {
			t.Fatalf("insert partial payment: %v", err)
		}

		var pendingPayID string
		err = pool.Raw().QueryRowContext(ctx, `
			INSERT INTO payments (rt_id, bill_id, amount, method, origin, status)
			VALUES ($1, $2, 50000, 'TRANSFER', 'SELF_SUBMITTED', 'PENDING')
			RETURNING id
		`, rtID, bills["2026-04"].ID).Scan(&pendingPayID)
		if err != nil {
			t.Fatalf("insert pending parent payment: %v", err)
		}
		_, err = pool.Raw().ExecContext(ctx, `
			INSERT INTO payment_allocations (payment_id, bill_id, amount)
			VALUES ($1, $2, 50000)
		`, pendingPayID, bills["2026-03"].ID)
		if err != nil {
			t.Fatalf("insert pending payment_allocation on 2026-03: %v", err)
		}

		// Move Tanggal Mulai forward to 2026-04-01 via XLSX:
		// - 2026-01 (unpaid with only REJECTED payment) becomes Tidak Wajib (is_obligatory = false)
		// - 2026-02 (partial with APPROVED payment) retains status='partial', is_obligatory=true, and its Rp 20.000 payment
		// - 2026-03 (unpaid with PENDING payment_allocation) retains status='unpaid', is_obligatory=true
		commitXLSXImport(t, router, pool, pToken, buildSingleWargaXLSX("K-01", "Warga K", "OWNER", "Jl. K1", "Warga K", nik, phone, email, "2026-04-01", "", ""))

		billsAfterForward := queryOccupancyBillsByDue(t, pool, occID, dueID)
		if billsAfterForward["2026-01"].IsObligatory {
			t.Errorf("2026-01 unpaid bill with only REJECTED payment should become is_obligatory=false")
		}
		if !billsAfterForward["2026-03"].IsObligatory || billsAfterForward["2026-03"].Status != "unpaid" {
			t.Errorf("2026-03 unpaid bill with PENDING payment_allocation must stay is_obligatory=true, got %+v", billsAfterForward["2026-03"])
		}
		if billsAfterForward["2026-02"].Status != "partial" || !billsAfterForward["2026-02"].IsObligatory {
			t.Errorf("2026-02 partially paid bill must retain status=partial and is_obligatory=true, got %+v", billsAfterForward["2026-02"])
		}

		// Move Tanggal Mulai backwards again to 2026-01-01 via XLSX:
		// - 2026-01 and 2026-03 become/remain Belum Bayar (status='unpaid', is_obligatory=true)
		// - 2026-02 remains status='partial', is_obligatory=true
		commitXLSXImport(t, router, pool, pToken, buildSingleWargaXLSX("K-01", "Warga K", "OWNER", "Jl. K1", "Warga K", nik, phone, email, "2026-01-01", "", ""))

		billsAfterBackward := queryOccupancyBillsByDue(t, pool, occID, dueID)
		if !billsAfterBackward["2026-01"].IsObligatory || billsAfterBackward["2026-01"].Status != "unpaid" {
			t.Errorf("2026-01 expected unpaid+obligatory, got %+v", billsAfterBackward["2026-01"])
		}
		if !billsAfterBackward["2026-02"].IsObligatory || billsAfterBackward["2026-02"].Status != "partial" {
			t.Errorf("2026-02 expected partial+obligatory, got %+v", billsAfterBackward["2026-02"])
		}
		if !billsAfterBackward["2026-03"].IsObligatory || billsAfterBackward["2026-03"].Status != "unpaid" {
			t.Errorf("2026-03 expected unpaid+obligatory, got %+v", billsAfterBackward["2026-03"])
		}
	})

	// -------------------------------------------------------------------------
	// Test L — Rollback transaksi penuh jika sinkronisasi tagihan gagal
	// -------------------------------------------------------------------------
	t.Run("Test L - Rollback transaksi penuh jika sinkronisasi tagihan gagal", func(t *testing.T) {
		rtID := setupTestRT(t, pool, "RT XLSX Sync L", 1, fmt.Sprintf("XL%d", time.Now().UnixNano()%100000))
		pToken := makeTenantToken(t, "pengurus-xl", "mid-xl", rtID, auth.RolePengurus)

		dueID := createMultiMonthDueForTest(t, router, pToken, "Iuran Rollback 2026", "50000", 2026)

		nik := fmt.Sprintf("320113%010d", time.Now().UnixNano()%10000000000)
		phone := fmt.Sprintf("0825%08d", time.Now().UnixNano()%100000000)
		email := fmt.Sprintf("warga.l.%d@test.com", time.Now().UnixNano())

		commitXLSXImport(t, router, pool, pToken, buildSingleWargaXLSX("L-01", "Warga L", "OWNER", "Jl. L1", "Warga L", nik, phone, email, "2026-04-01", "", ""))

		var hhID, occID, resID string
		_ = pool.Raw().QueryRowContext(ctx, `
			SELECT ho.household_id, ho.id, r.id
			FROM residents r
			JOIN residency_periods rp ON rp.resident_id = r.id AND rp.end_date IS NULL
			JOIN household_occupancies ho ON ho.id = rp.household_occupancy_id AND ho.end_date IS NULL
			WHERE r.rt_id = $1 AND r.nik = $2
		`, rtID, nik).Scan(&hhID, &occID, &resID)

		// Inject failure into SyncOccupancyBillsFn
		origHook := SyncOccupancyBillsFn
		SyncOccupancyBillsFn = func(ctx context.Context, tx *sql.Tx, rtID, occID string) error {
			return fmt.Errorf("simulated bill sync database failure")
		}
		defer func() { SyncOccupancyBillsFn = origHook }()

		// 1. Attempt XLSX import moving start_date to 2026-01-01 -> must fail and rollback!
		xlsxData := buildSingleWargaXLSX("L-01", "Warga L", "OWNER", "Jl. L1", "Warga L", nik, phone, email, "2026-01-01", "", "")
		buf, boundary := buildMultipartBody("file", "warga.xlsx", xlsxData)
		req := makeAuthRequest(pool, http.MethodPost, "/api/v1/warga/import/commit/xlsx", pToken, buf)
		req.Header.Set("Content-Type", boundary)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code == http.StatusCreated || rec.Code == http.StatusOK {
			t.Fatalf("expected error status when SyncOccupancyBillsFn fails during XLSX import, got %d", rec.Code)
		}

		// 2. Attempt PATCH /api/v1/households/{id} -> must fail and rollback!
		reqHH := httptest.NewRequest(http.MethodPatch, "/api/v1/households/"+hhID, bytes.NewBufferString(`{"start_date":"2026-01-01"}`))
		reqHH.Header.Set("Authorization", "Bearer "+pToken)
		reqHH.Header.Set("Content-Type", "application/json")
		recHH := httptest.NewRecorder()
		router.ServeHTTP(recHH, reqHH)
		if recHH.Code == http.StatusOK {
			t.Fatalf("expected error status when SyncOccupancyBillsFn fails during PATCH household, got %d", recHH.Code)
		}

		// 3. Attempt PATCH /api/v1/residents/{id} -> must fail and rollback!
		reqRes := httptest.NewRequest(http.MethodPatch, "/api/v1/residents/"+resID, bytes.NewBufferString(`{"start_date":"2026-01-01"}`))
		reqRes.Header.Set("Authorization", "Bearer "+pToken)
		reqRes.Header.Set("Content-Type", "application/json")
		recRes := httptest.NewRecorder()
		router.ServeHTTP(recRes, reqRes)
		if recRes.Code == http.StatusOK {
			t.Fatalf("expected error status when SyncOccupancyBillsFn fails during PATCH resident, got %d", recRes.Code)
		}

		// Verify neither residency_periods.start_date nor household_occupancies.start_date was committed!
		var rpStart, hoStart string
		_ = pool.Raw().QueryRowContext(ctx, `SELECT to_char(start_date, 'YYYY-MM-DD') FROM residency_periods WHERE resident_id = $1 AND end_date IS NULL`, resID).Scan(&rpStart)
		_ = pool.Raw().QueryRowContext(ctx, `SELECT to_char(start_date, 'YYYY-MM-DD') FROM household_occupancies WHERE id = $1`, occID).Scan(&hoStart)
		if rpStart != "2026-04-01" || hoStart != "2026-04-01" {
			t.Fatalf("expected rollback to preserve 2026-04-01, got rp=%s ho=%s", rpStart, hoStart)
		}

		bills := queryOccupancyBillsByDue(t, pool, occID, dueID)
		if bills["2026-01"].IsObligatory {
			t.Errorf("expected 2026-01 to remain is_obligatory=false after rollback")
		}
	})

	// -------------------------------------------------------------------------
	// Test M — Riwayat hunian historis & multi-penghuni dalam satu household
	// -------------------------------------------------------------------------
	t.Run("Test M - Riwayat hunian historis dan multi-penghuni tetap terjaga", func(t *testing.T) {
		rtID := setupTestRT(t, pool, "RT XLSX Sync M", 1, fmt.Sprintf("XM%d", time.Now().UnixNano()%100000))
		pToken := makeTenantToken(t, "pengurus-xm", "mid-xm", rtID, auth.RolePengurus)

		dueID := createMultiMonthDueForTest(t, router, pToken, "Iuran Multi Penghuni 2026", "50000", 2026)

		nikHead := fmt.Sprintf("320114%010d", time.Now().UnixNano()%10000000000)
		phoneHead := fmt.Sprintf("0826%08d", time.Now().UnixNano()%100000000)
		emailHead := fmt.Sprintf("head.m.%d@test.com", time.Now().UnixNano())

		nikMember := fmt.Sprintf("320115%010d", time.Now().UnixNano()%10000000000)
		phoneMember := fmt.Sprintf("0827%08d", time.Now().UnixNano()%100000000)
		emailMember := fmt.Sprintf("member.m.%d@test.com", time.Now().UnixNano())

		// Import household M-01 with Head (2026-04-01) and Member (2026-06-01)
		f := excelize.NewFile()
		writeHeaderRow(f)
		writeRow(f, 0, []string{"M-01", "Kepala M", "OWNER", "Jl. M1", "Kepala M", nikHead, phoneHead, emailHead, "2026-04-01", "anggota", ""})
		writeRow(f, 1, []string{"M-01", "Kepala M", "OWNER", "Jl. M1", "Anggota M", nikMember, phoneMember, emailMember, "2026-06-01", "anggota", ""})
		commitXLSXImport(t, router, pool, pToken, xlsxBytes(f))
		f.Close()

		var hhID, occID, headResID, memberResID string
		_ = pool.Raw().QueryRowContext(ctx, `SELECT ho.household_id, ho.id, r.id FROM residents r JOIN residency_periods rp ON rp.resident_id = r.id JOIN household_occupancies ho ON ho.id = rp.household_occupancy_id WHERE r.rt_id = $1 AND r.nik = $2`, rtID, nikHead).Scan(&hhID, &occID, &headResID)
		_ = pool.Raw().QueryRowContext(ctx, `SELECT r.id FROM residents r WHERE r.rt_id = $1 AND r.nik = $2`, rtID, nikMember).Scan(&memberResID)

		// Also insert a closed historical residency period in the same occupancy (2025-01-01 .. 2025-08-01)
		var histRPID string
		err := pool.Raw().QueryRowContext(ctx, `
			INSERT INTO residency_periods (resident_id, household_occupancy_id, start_date, end_date)
			VALUES ($1, $2, '2025-01-01', '2025-08-01')
			RETURNING id
		`, memberResID, occID).Scan(&histRPID)
		if err != nil {
			t.Fatalf("insert historical residency period: %v", err)
		}

		// 1. Update household via PATCH /api/v1/households/{id} with start_date=2026-01-01 and end_date=""
		reqHH := httptest.NewRequest(http.MethodPatch, "/api/v1/households/"+hhID, bytes.NewBufferString(`{"start_date":"2026-01-01","end_date":""}`))
		reqHH.Header.Set("Authorization", "Bearer "+pToken)
		reqHH.Header.Set("Content-Type", "application/json")
		recHH := httptest.NewRecorder()
		router.ServeHTTP(recHH, reqHH)
		if recHH.Code != http.StatusOK {
			t.Fatalf("PATCH household expected 200, got %d: %s", recHH.Code, recHH.Body.String())
		}

		// Verify:
		// - Head resident's active start_date is now 2026-01-01
		// - Member resident's active start_date is STILL 2026-06-01 (not overwritten!)
		// - Historical residency period is STILL 2025-01-01 .. 2025-08-01 (not overwritten or reopened!)
		var headStart, memberStart, histStart string
		var histEnd sql.NullString
		_ = pool.Raw().QueryRowContext(ctx, `SELECT to_char(start_date, 'YYYY-MM-DD') FROM residency_periods WHERE resident_id = $1 AND end_date IS NULL`, headResID).Scan(&headStart)
		_ = pool.Raw().QueryRowContext(ctx, `SELECT to_char(start_date, 'YYYY-MM-DD') FROM residency_periods WHERE resident_id = $1 AND end_date IS NULL`, memberResID).Scan(&memberStart)
		_ = pool.Raw().QueryRowContext(ctx, `SELECT to_char(start_date, 'YYYY-MM-DD'), to_char(end_date, 'YYYY-MM-DD') FROM residency_periods WHERE id = $1`, histRPID).Scan(&histStart, &histEnd)

		if headStart != "2026-01-01" {
			t.Errorf("expected head start_date 2026-01-01, got %s", headStart)
		}
		if memberStart != "2026-06-01" {
			t.Errorf("expected member start_date to remain 2026-06-01, got %s", memberStart)
		}
		if histStart != "2025-01-01" || !histEnd.Valid || histEnd.String != "2025-08-01" {
			t.Errorf("historical residency period was corrupted: start=%s end=%+v", histStart, histEnd)
		}

		// 2. Update Member's start_date to 2026-07-01 via XLSX (while Head's move_in_date is empty in XLSX)
		// Household occupancy start_date must remain 2026-01-01 (Head's start_date) and Jan-Dec bills must remain obligatory!
		f2 := excelize.NewFile()
		writeHeaderRow(f2)
		writeRow(f2, 0, []string{"M-01", "Kepala M", "OWNER", "Jl. M1", "Kepala M", nikHead, phoneHead, emailHead, "", "anggota", ""})
		writeRow(f2, 1, []string{"M-01", "Kepala M", "OWNER", "Jl. M1", "Anggota M", nikMember, phoneMember, emailMember, "2026-07-01", "anggota", ""})
		commitXLSXImport(t, router, pool, pToken, xlsxBytes(f2))
		f2.Close()

		var hoStart string
		_ = pool.Raw().QueryRowContext(ctx, `SELECT to_char(start_date, 'YYYY-MM-DD') FROM household_occupancies WHERE id = $1`, occID).Scan(&hoStart)
		_ = pool.Raw().QueryRowContext(ctx, `SELECT to_char(start_date, 'YYYY-MM-DD') FROM residency_periods WHERE resident_id = $1 AND end_date IS NULL`, memberResID).Scan(&memberStart)
		if memberStart != "2026-07-01" {
			t.Errorf("expected member start_date updated to 2026-07-01, got %s", memberStart)
		}
		if hoStart != "2026-01-01" {
			t.Errorf("expected household occupancy start_date to remain 2026-01-01 (Head's date), got %s", hoStart)
		}
		bills := queryOccupancyBillsByDue(t, pool, occID, dueID)
		for m := 1; m <= 12; m++ {
			p := fmt.Sprintf("2026-%02d", m)
			if !bills[p].IsObligatory {
				t.Errorf("period %s should remain obligatory (Head moved in 2026-01-01)", p)
			}
		}
	})
}
