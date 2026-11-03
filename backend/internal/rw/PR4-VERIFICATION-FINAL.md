# Verifikasi Final PR 4 — 4 RW Read-Only Endpoints

## 1. Status Repositori

| Metric | Nilai |
|--------|-------|
| `git status --short` | 9 file dimodifikasi, 25 untracked |
| `git diff --stat` | 392 insertions, 37 deletions across 9 modified files |
| `git diff --check` | ✅ No whitespace errors |
| **Commit** | ❌ Tidak ada commit (sesuai instruksi) |

### File yang berubah (9 modified + 25 untracked):

**Modified:**
| File | Perubahan | Catatan |
|------|-----------|---------|
| `backend/cmd/api/main.go` | +28 | import `rw`, `LoadScoping`, `buildRouter` signature baru, 4 RW routes |
| `backend/cmd/api/router_auth_test.go` | +6 | pass `&cfg` ke `buildRouter` |
| `backend/internal/auth/jwt.go` | +15 | scoping-related |
| `backend/internal/auth/jwt_test.go` | +113 | JWT changes |
| `backend/internal/auth/middleware.go` | +82 | middleware additions |
| `backend/internal/auth/models.go` | +20 | AuthContext additions |
| `backend/internal/auth/service.go` | +110 | service enhancements |
| `backend/internal/fund_location/handler.go` | +25 | handler updates |
| `docs/database.md` | +30 | database docs |

**New (25 untracked):**
- `backend/internal/rw/` — 6 file (model, handler, service, repository, handler test, integration test)
- `backend/internal/auth/scoping.go` — `RequireRWScope` middleware + `rwPermissionMap`
- `backend/internal/auth/scoping_test.go` — 20 test cases
- `backend/internal/fund_location/handler_security_test.go`
- `docs/` — 18 file audit & spec

---

## 2. Audit Keenam Query Dashboard `GetDashboardSummary()`

### Query 1 — TotalRTs ✅
```sql
SELECT COUNT(*) FROM rts WHERE rw_id = $1 AND is_active = true
```
- ✅ Filter `rw_id` secara server-side (tidak dari client)
- ✅ Hanya RT aktif (`is_active = true`)

### Query 2 — TotalLocations ✅
```sql
SELECT COUNT(*) FROM fund_locations fl
 JOIN rts r ON fl.rt_id = r.id
 WHERE r.rw_id = $1 AND r.is_active = true AND fl.is_active = true
```
- ✅ Filter `rw_id` via join `rts`
- ✅ Hanya RT dan location aktif

### Query 3 — TotalBalance ✅ (FIXED)
```sql
SELECT COALESCE(SUM(lbc.amount_delta), 0)::numeric(15,2)::text
 FROM location_balance_changes lbc
 JOIN fund_locations fl ON lbc.location_id = fl.id
 JOIN rts r ON fl.rt_id = r.id
 WHERE r.rw_id = $1 AND r.is_active = true
```
- ✅ Kolom `lbc.amount_delta` (bukan `lbc.net` yang tidak ada)
- ✅ Mirip `fund_location/repository.go:62` dan `fund_location/repository.go:95`
- ✅ Hanya LBC dari RT aktif dalam RW
- ✅ Tidak double-count: join `lbc → fl → rts` satu-ke-satu

### Query 4 — TotalIncome ✅
```sql
SELECT COALESCE(SUM(CASE WHEN type = 'income' AND status = 'posted' THEN amount ELSE 0 END), 0)::numeric(15,2)::text
 FROM transactions
 WHERE rt_id IN (SELECT id FROM rts WHERE rw_id = $1 AND is_active = true)
   AND status = 'posted'
```
- ✅ Filter `rw_id` via subquery
- ✅ Hanya transaksi `posted`
- ✅ Hanya `income`

### Query 5 — TotalExpense ✅
```sql
SELECT COALESCE(SUM(CASE WHEN type = 'expense' AND status = 'posted' THEN amount ELSE 0 END), 0)::numeric(15,2)::text
 FROM transactions
 WHERE rt_id IN (SELECT id FROM rts WHERE rw_id = $1 AND is_active = true)
   AND status = 'posted'
```
- ✅ Pola sama dengan income, filter `expense`

### Query 6 — TransactionCount ✅
```sql
SELECT COUNT(*) FROM transactions
 WHERE rt_id IN (SELECT id FROM rts WHERE rw_id = $1 AND is_active = true)
   AND status = 'posted'
```
- ✅ Hanya transaksi posted

### Kesimpulan Dashboard Queries
- ✅ Tidak ada Cartesian product — 6 `QueryRowContext` independen
- ✅ Tidak ada `lbc.net` — semua menggunakan `amount_delta`
- ✅ Semua query mem-filter `rw_id` secara server-side
- ✅ Semua query mem-filter `is_active = true` pada RT
- ✅ Format konsisten `numeric(15,2)::text` → `"0.00"` bukan `"0"`

---

## 3. Bug Baru Ditemukan dan Diperbaiki: `GetLocations()`

### Masalah
```go
// LAMA (bug)
SELECT fl.current_balance::text FROM fund_locations fl
```
Kolom `current_balance` **tidak ada** di tabel `fund_locations`. Nilai ini adalah computed dari `SUM(lbc.amount_delta)`.

### Perbaikan
```go
// BARU (fixed)
SELECT fl.id, fl.rt_id, rt.name as rt_name, fl.name, fl.category,
    fl.bank_name, fl.account_number, fl.account_holder, fl.is_active,
    COALESCE(SUM(lbc.amount_delta), 0)::numeric(15,2)::text as current_balance,
    fl.created_at::text
FROM fund_locations fl
JOIN rts rt ON fl.rt_id = rt.id
LEFT JOIN location_balance_changes lbc ON fl.id = lbc.location_id
WHERE rt.rw_id = $1 AND rt.is_active = true
GROUP BY fl.id, rt.name
```

✅ Mirip pola `fund_location/repository.go:62` dan `fund_location/repository.go:95`  
✅ `LEFT JOIN` — lokasi tanpa LBC menghasilkan balance `0.00`  
✅ `GROUP BY fl.id, rt.name` — diperlukan karena `SUM()` adalah agregat  
✅ Format konsisten `numeric(15,2)::text` → `"0.00"` bukan `"0"`

---

## 4. Hasil Test

| Test Suite | Hasil | Detail |
|------------|-------|--------|
| `go build ./...` | ✅ **0 errors** | Seluruh workspace Go |
| `go test ./internal/rw/...` | ✅ **30/30 PASS** | 14 handler test + 16 integration test |
| `go test ./internal/auth/...` | ✅ **ALL PASS** | 50+ test functions |
| `go test ./internal/fund_location/...` | ✅ **36/36 PASS** | Semua integration tests |

---

## 5. Verifikasi 4 Endpoint GET (Bukan POST)

Dari `backend/cmd/api/main.go:748-761`:
```go
r.Group(func(r chi.Router) {
    r.Use(auth.RequireAuth)
    r.Use(auth.RequireRWScope(pool, *scopingCfg))
    r.Get("/api/v1/rw/{rwId}/dashboard", rwHandler.GetDashboard)
    r.Get("/api/v1/rw/{rwId}/kas", rwHandler.GetKAS)
    r.Get("/api/v1/rw/{rwId}/kas/transactions", rwHandler.GetKASTransactions)
    r.Get("/api/v1/rw/{rwId}/locations", rwHandler.GetLocations)
})
```

✅ **Tepat 4 endpoint**, semua `GET`  
✅ Tidak ada `POST`, `PUT`, `PATCH`, `DELETE`  
✅ `RequireAuth` + `RequireRWScope` middleware terpasang  
✅ Routing tidak tumpang tindih dengan RT routes

---

## 6. Verifikasi Rumus Balance

### Sumber Balance: `location_balance_changes.amount_delta`

Dari `fund_location/repository.go:62` dan `fund_location/repository.go:95`:
```sql
COALESCE(SUM(lbc.amount_delta), 0)::text as current_balance
```

Dari `rw/repository.go:42` (dashboard balance):
```sql
COALESCE(SUM(lbc.amount_delta), 0)::numeric(15,2)::text
```

✅ **Pola identik** — dashboard RW menggunakan sumber data yang sama.

### Reversal & Cancelled Transactions

Semua query menggunakan `status = 'posted'` — hanya transaksi finalized yang dihitung.
- `draft` — tidak计入
- `cancelled` — tidak计入 (reversal dibuat sebagai transaksi baru `posted`)

---

## 7. Regression Test Numerik

Integration test baru (`rw_integration_test.go`) menambahkan 16 test:

| Test Function | Deskripsi | Hasil |
|---------------|-----------|-------|
| `TestGetDashboardSummaryNumeric` | Verifikasi angka numerik dengan data multiple locations, multiple transactions, isolasi 2 RW | ✅ PASS |
| `TestGetDashboardSummaryWithReversal` | Verifikasi reversal, draft, cancellation, dan impact pada balance | ✅ PASS |
| `TestGetLocationsBalanceComputed` | Verifikasi balance dihitung via `SUM(lbc.amount_delta)`, filter is_active | ✅ PASS |
| `TestGetLocationsTenantIsolation` | Verifikasi isolasi tenant antar RW | ✅ PASS |
| `TestGetKASReportWithPeriod` | Verifikasi period filter | ✅ PASS |
| `TestGetTransactionsPagination` | Verifikasi pagination | ✅ PASS |
| `TestServiceCheckRWAccess` | 4 sub-test: nil context, matching RW, non-matching RW, empty list | ✅ PASS |

---

## 8. Status Verifikasi: **PASS**

| Kriteria | Status | Catatan |
|----------|--------|---------|
| Query 1-6 dashboard benar | ✅ PASS | Semua filter rw_id, is_active, status benar |
| Kolom `lbc.amount_delta` | ✅ PASS | Tidak ada `lbc.net` |
| Tidak ada Cartesian product | ✅ PASS | 6 query independen |
| Rumus balance konsisten | ✅ PASS | Mirip `fund_location/repository.go` |
| Tidak double-count | ✅ PASS | Dashboard sum LBC saja |
| Reversal/cancelled handled | ✅ PASS | Hanya `status = 'posted'` |
| 4 endpoint GET | ✅ PASS | Tidak ada POST/PUT/DELETE |
| Build sukses | ✅ PASS | `go build ./...` |
| Test RW 30/30 | ✅ PASS | `go test ./internal/rw/...` |
| Test auth | ✅ PASS | `go test ./internal/auth/...` |
| Test fund_location 36/36 | ✅ PASS | `go test ./internal/fund_location/...` |
| Regression test numerik | ✅ PASS | 16 new integration tests |
| Tenant isolation | ✅ PASS | Verified di test Dashboard + Locations |
| No whitespace errors | ✅ PASS | `git diff --check` |

---

## 9. Tidak Ada Commit atau Push yang Dilakukan.

Semua perubahan tetap di working tree (unstaged) sesuai instruksi.
