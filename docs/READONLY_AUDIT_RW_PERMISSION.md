# AUDIT READ-ONLY — VALIDASI RW PERMISSION ENFORCEMENT
**Tanggal:** 2026-10-31
**Status:** READ-ONLY — Tidak ada file diubah.

---

## 1. Executive Verdict: PASS

Implementasi RW Permission Enforcement **memenuhi spesifikasi desain yang disetujui**. Permission map benar, enforcement di service layer, SA bypass terbatas, isolasi tenant terjamin, dan error handling sesuai kebijakan. Tidak ada jalur bypass yang ditemukan.

---

## 2. Bukti Implementasi Permission Map

**File:** `backend/internal/auth/scoping.go`

### rwPermissionMap — Baris 243–266

```go
var rwPermissionMap = map[string][]string{
	"ketua": {
		"finance.read",
		"finance.create",
		"finance.update",
		"finance.approve",
	},
	"wakil_ketua": {
		"finance.read",
		"finance.create",
		// TIDAK punya finance.update — tidak bisa transfer, tidak bisa update transaksi
		"finance.approve",
	},
	"sekretaris": {
		"finance.read",
	},
	"bendahara": {
		"finance.read",
		"finance.create",
		"finance.update",
		"finance.approve",
	},
}
```

**Verifikasi matriks:**

| Jabatan | read | create | update | approve | Sesuai Desain? |
|---|---|---|---|---|---|
| `ketua` | ✅ | ✅ | ✅ | ✅ | Ya |
| `wakil_ketua` | ✅ | ✅ | ❌ | ✅ | Ya |
| `sekretaris` | ✅ | ❌ | ❌ | ❌ | Ya |
| `bendahara` | ✅ | ✅ | ✅ | ✅ | Ya |
| unknown | ❌ | ❌ | ❌ | ❌ | Ya (deny by default) |

### hasRWScopePermission() — Baris 278–289

```go
func hasRWScopePermission(jabatan string, permissionCode string) bool {
	perms, ok := rwPermissionMap[jabatan]
	if !ok {
		return false
	}
	for _, p := range perms {
		if p == permissionCode {
			return true
		}
	}
	return false
}
```

**Analisis:**
- **Tidak ada default allow:** Jabatan yang tidak ada di map langsung return `false` (baris 281).
- **Tidak ada fallback:** Loop hanya mengecek kesetaraan eksak, tidak ada fallback ke jabatan lain.
- **Case-sensitive:** Map lookup `rwPermissionMap[jabatan]` menggunakan string eksak — client tidak bisa memanipulasi case untuk绕过.
- **Exported wrapper** `HasRWScopePermission()` (baris 272–274) hanya forward ke lowercase, tidak ada logik tambahan.

### Jabatan RT ≠ Jabatan RW

Jabatan RT disimpan di field `auth.Jabatan` dalam `AuthContext`. Jabatan RW disimpan di `auth.RWMembership.Jabatan`. Fungsi `hasRWScopePermission()` menerima string `jabatan`, dan pemanggilnya (`checkRWPermission`) secara eksplisit menggunakan `rm.Jabatan` dari `RWMembership` (baris service.go:70). Tidak ada jalur yang mengirim `ac.Jabatan` (RT) ke permission check.

---

## 3. Bukti Service-Layer Enforcement

**File:** `backend/internal/rw/service.go`

### checkRWAccess() — Baris 51–65

```go
func (s *Service) checkRWAccess(ac *auth.AuthContext, rwID string) error {
	if ac == nil {
		return fmt.Errorf("no authentication context")
	}
	if ac.SystemRole == auth.SystemRoleSuperAdmin {
		return nil
	}
	for _, rm := range ac.RWMemberships {
		if rm.RWID == rwID {
			return nil
		}
	}
	return fmt.Errorf("%w: rw %s", ErrRWAccessDenied, rwID)
}
```

**Verifikasi:**
- Membership user diverifikasi pada RW yang sedang diakses (baris 61–62: `rm.RWID == rwID`).
- Role berasal dari `AuthContext` yang diset middleware `RequireAuth` berdasarkan JWT server-validated.
- Role/permission tidak dapat dikirim client — `RWMemberships` diisi dari JWT claim yang di-hash & diverifikasi server.

### checkRWPermission() — Baris 67–88

```go
func (s *Service) checkRWPermission(ac *auth.AuthContext, rwID string, requiredPermission string) error {
	if ac == nil {
		return fmt.Errorf("no authentication context")
	}
	if ac.SystemRole == auth.SystemRoleSuperAdmin {
		return nil
	}
	for _, rm := range ac.RWMemberships {
		if rm.RWID == rwID {
			if auth.HasRWScopePermission(rm.Jabatan, requiredPermission) {
				return nil
			}
			return fmt.Errorf("%w: required %q for jabatan %q", ErrPermissionDenied, requiredPermission, rm.Jabatan)
		}
	}
	return fmt.Errorf("%w: no RW membership for %s", ErrRWAccessDenied, rwID)
}
```

**Verifikasi:**
- Permission diperiksa **sebelum** query repository dijalankan (dipanggil di `requirePermission()`).
- Jika jabatan tidak memiliki permission → `ErrPermissionDenied` (deny by default).
- Jika tidak ada membership matching → `ErrRWAccessDenied`.

### requirePermission() — Baris 91–96

```go
func (s *Service) requirePermission(ctx context.Context, ac *auth.AuthContext, rwID string, permissionCode string) error {
	if err := s.checkRWAccess(ac, rwID); err != nil {
		return err
	}
	return s.checkRWPermission(ac, rwID, permissionCode)
}
```

### Public Service Methods — Semua dilindungi

| Method | Baris | Operation | Permission | Dilindungi? |
|---|---|---|---|---|
| `GetDashboard()` | 98 | read | `finance.read` | ✅ Baris 100 |
| `GetKASReport()` | 120 | read | `finance.read` | ✅ Baris 122 |
| `GetTransactions()` | 140 | read | `finance.read` | ✅ Baris 142 |
| `GetLocations()` | 161 | read | `finance.read` | ✅ Baris 163 |

### Tidak ada jalur bypass

- Handler (`handler.go`) memanggil hanya 4 public service methods di atas.
- Tidak ada method publik lain di package `rw` yang tidak memanggil `requirePermission()`.
- Semua 4 methods memanggil `requirePermission()` di **baris pertama**, sebelum `pool.BeginTx()` atau query manapun.
- Tidak ada method internal publik yang bisa dipanggil langsung dari luar service.

---

## 4. Audit Super Admin

### Middleware — `backend/internal/auth/scoping.go` baris 158–178

SA bypass dilakukan di middleware `RequireRWScope`, bukan hanya di service layer:

```go
if ac.SystemRole == SystemRoleSuperAdmin {
	// ... query DB untuk cek active status
	var isActive bool
	err = tx.QueryRowContext(ctx, `SELECT is_active FROM rws WHERE id = $1 LIMIT 1`, rwID).Scan(&isActive)
	// ...
	if err == sql.ErrNoRows || !isActive {
		httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "rw not found or inactive")
		return
	}
	next.ServeHTTP(w, r)
	return
}
```

### SA di service layer — `service.go` baris 57–58 dan 73–74

```go
// Super Admin bypass: system-level authorization, no membership required.
if ac.SystemRole == auth.SystemRoleSuperAdmin {
	return nil
}
```

**Verifikasi SA:**
1. ✅ **Dapat mengakses RW terkonfigurasi dan aktif tanpa membership** — Bypass di middleware (scope + active DB check) dan service (checkRWAccess + checkRWPermission).
2. ✅ **Tidak dapat mengakses RW di luar `SCOPED_RW_IDS`** — `cfg.hasRW(rwID)` di middleware baris 147 mengecek SCOPED_RW_IDS sebelum SA bypass. Return 403.
3. ✅ **Tidak dapat mengakses RW yang tidak aktif** — Query `SELECT is_active FROM rws WHERE id = $1` di middleware baris 168–171. Return 403.
4. ✅ **Tidak mendapatkan membership/jabatan palsu** — SA bypass kedua check tanpa membuat record apa pun.
5. ✅ **Bypass permission, bukan bypass scope/isolasi** — SA tetap melewati `cfg.hasRW()` (scope) dan `is_active` check. Repository queries tetap parameterized dengan `rw_id` (isolasi data tetap ada, tapi SA bisa mengakses semua rw_id yang valid).

### Service-level `checkRWAccess()` untuk SA

Service `checkRWAccess()` memiliki SA bypass (`ac.SystemRole == auth.SystemRoleSuperAdmin`), tapi **deployment-level gate** (SCOPED_RW_IDS) dan **active DB check** hanya ada di middleware. Ini benar — service layer tidak perlu mengulangi pengecekan deployment config yang sudah ada di middleware.

---

## 5. Inventaris Seluruh Endpoint dan Public Service Method

### Semua Route RW — `backend/cmd/api/main.go` baris 748–760

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

### Endpoint Detail

| # | Method | Route | Handler | Service Method | Permission | Protected? |
|---|---|---|---|---|---|---|
| 1 | GET | `/api/v1/rw/{rwId}/dashboard` | `rwHandler.GetDashboard` | `svc.GetDashboard()` | `finance.read` | ✅ |
| 2 | GET | `/api/v1/rw/{rwId}/kas` | `rwHandler.GetKAS` | `svc.GetKASReport()` | `finance.read` | ✅ |
| 3 | GET | `/api/v1/rw/{rwId}/kas/transactions` | `rwHandler.GetKASTransactions` | `svc.GetTransactions()` | `finance.read` | ✅ |
| 4 | GET | `/api/v1/rw/{rwId}/locations` | `rwHandler.GetLocations` | `svc.GetLocations()` | `finance.read` | ✅ |

**Semua endpoint READ-ONLY.** Tidak ada operasi WRITE (POST/PUT/PATCH/DELETE) di module RW.

### Transfer Antar-Lokasi

**Belum diimplementasikan.** Tidak ada endpoint, handler, service method, atau repository method untuk transfer antar lokasi dalam codebase. Tidak dapat diuji karena belum ada.

### Public Methods di Package `rw`

Semua 4 public service method telah diinventarisasi di atas. Tidak ada method publik lain di package `rw` yang melakukan operasi data.

Public types: `Service`, `Handler`, `Repository` — semua interface implementation, tidak ada method standalone.

---

## 6. Audit HTTP Error Handling

**File:** `backend/internal/rw/handler.go`

### Error Handler — Baris 171–181

```go
func (h *Handler) handleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrRWAccessDenied):
		httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "rw access denied")
	case errors.Is(err, ErrPermissionDenied):
		httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "insufficient permissions")
	case errors.Is(err, ErrValidation):
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", err.Error())
	default:
		slog.Error("internal error", "error", err.Error())
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}
```

### Error Types — `backend/internal/rw/model.go`

```go
var ErrRWAccessDenied = errors.New("rw access denied")      // Baris 10
var ErrPermissionDenied = errors.New("permission denied")    // Baris 13
var ErrValidation = errors.New("validation error")           // Baris 16
```

### Pemetaan HTTP Status

| Kondisi | HTTP | File | Baris | Mekanisme |
|---|---|---|---|---|
| Tidak terautentikasi | **401** | `handler.go` | 38, 63, 91, 125 | `http.StatusUnauthorized` langsung di handler |
| Scope tidak valid (SA & non-SA) | **403** | `scoping.go` | 147 | `cfg.hasRW()` return false |
| RW tidak aktif | **403** | `scoping.go` | 171 | `!isActive` check |
| Membership tidak valid | **403** | `scoping.go` | 193 | `rw access denied` |
| Permission tidak cukup | **403** | `handler.go` | 174 | `ErrPermissionDenied` via `errors.Is` |
| Resource RW lain | **Tidak ada 404 khusus** | — | — | Repository parameterized query akan return empty results, bukan 404 |

### Catatan: Tidak Ada 404 untuk Cross-RW Resource

Ketika user mencoba mengakses resource yang sebenarnya milik RW lain, repository query parameterized dengan `rw_id` akan mengembalikan **empty results** (0 rows), bukan 404. Ini sesuai dengan kebijakan "Never leak information about resources in other RWs." Namun, tidak ada pengecekan eksplisit untuk mengembalikan 404.

### Error Wrapping

`ErrPermissionDenied` dan `ErrRWAccessDenied` menggunakan `errors.New()` sebagai sentinel errors. Service methods membungkusnya dengan `fmt.Errorf("%w: ...")`. Handler menggunakan `errors.Is()` yang benar-benar melakukan error unwrapping. **Tidak ada perbandingan error yang rapuh** (`err == ErrPermissionDenied`). Ini aman.

### Verifikasi 401 dari Middleware

Middleware `RequireAuth` (file terpisah) mengembalikan 401 untuk token tidak valid. Middleware `RequireRWScope` di `scoping.go` baris 127 juga mengembalikan 401 jika `AuthContext` tidak ada di request context.

---

## 7. Audit Isolasi Tenant

**File:** `backend/internal/rw/repository.go`

### GetDashboardSummary() — Baris 45–110

Semua query menggunakan `$1` parameterized dengan `rwID`:

| Query | rw_id Filter | Aman? |
|---|---|---|
| `SELECT COUNT(*) FROM rts WHERE rw_id = $1` | ✅ | Ya |
| `SELECT COUNT(*) FROM fund_locations fl JOIN rts r ON fl.rt_id = r.id WHERE r.rw_id = $1` | ✅ | Ya |
| `SELECT ... FROM rw_opening_balances WHERE rw_id = $1` | ✅ | Ya |
| `SELECT ... FROM rw_transactions WHERE rw_id = $1` | ✅ (x3 query) | Ya |

### GetKASReport() — Baris 112–185

| Query | rw_id Filter | Aman? |
|---|---|---|
| `SELECT ... FROM rw_opening_balances WHERE rw_id = $1` | ✅ | Ya |
| `SELECT ... FROM rw_transactions WHERE rw_id = $1` | ✅ (x2 query) | Ya |

### GetTransactions() — Baris 187–316

| Query | rw_id Filter | Aman? |
|---|---|---|
| `SELECT ... FROM rw_transactions t WHERE ... t.rw_id = $1` | ✅ | Ya |
| `SELECT COUNT(*) FROM rw_transactions t WHERE ... t.rw_id = $1` | ✅ | Ya |

### GetLocations() — Baris 318–405

| Query | rw_id Filter | Aman? |
|---|---|---|
| `SELECT ... FROM fund_locations fl JOIN rts rt ON fl.rt_id = rt.id WHERE rt.rw_id = $1` | ✅ | Ya |

### Balance Computation

- **RW balance TIDAK menggunakan `fund_locations` RT** — semua dari `rw_transactions` dan `rw_opening_balances`.
- **Fund locations balance** dihitung per-location via `location_balance_changes`, hanya untuk display, bukan agregasi KAS RW.
- **Tidak ada aggregasi RT** untuk menghitung saldo KAS RW.

### Parameterized Queries

Semua query menggunakan `$1`, `$2`, dst. **Tidak ada string concatenation** dari user input. Date/period filters menggunakan `fmt.Sprintf()` untuk posisi parameter, bukan untuk nilai — nilai tetap melalui `args` slice.

---

## 8. Jumlah Test Aktual dan Hasilnya

### Test Functions di `permission_enforcement_test.go` (15 test functions)

| # | Nama Test | Tipe | Baris |
|---|---|---|---|
| 1 | `TestPermissionEnforcement_JabatanKetua` | Top-level | 53 |
| 2 | `TestPermissionEnforcement_JabatanWakilKetua` | Top-level | 80 |
| 3 | `TestPermissionEnforcement_JabatanSekretaris` | Top-level | 103 |
| 4 | `TestPermissionEnforcement_JabatanBendahara` | Top-level | 133 |
| 5 | `TestPermissionEnforcement_UnknownJabatan` | Top-level | 164 |
| 6 | `TestPermissionEnforcement_MultiRole` | Top-level | 187 |
| 7 | `TestPermissionEnforcement_RTJabatanNoRW` | Top-level | 210 |
| 8 | `TestPermissionEnforcement_SuperAdmin` | Top-level | 232 |
| 9 | `TestPermissionEnforcement_NoMembership` | Top-level | 262 |
| 10 | `TestPermissionEnforcement_CrossRW` | Top-level | 279 |
| 11 | `TestServiceCheckRWPermission_Direct` | Top-level (dengan subtest) | 303 |
| 12 | `TestServiceCheckRWAccess_SAWithNoMembership` | Top-level | 369 |
| 13 | `TestServiceCheckRWAccess_NilContext` | Top-level | 384 |
| 14 | `TestPermissionEnforcement_Consistency` | Top-level | 400 |
| 15 | `TestErrPermissionDenied_Is` | Top-level | 430 |

### Subtest di `TestServiceCheckRWPermission_Direct` (4 subtest)

| # | Nama Subtest | Baris |
|---|---|---|
| 1 | `ketua has all permissions` | 310 |
| 2 | `wakil_ketua lacks update` | 325 |
| 3 | `sekretaris has only read` | 340 |
| 4 | `bendahara has all permissions` | 355 |

### Total Test Count

| Metric | Jumlah |
|---|---|
| Top-level test functions | **15** |
| Subtest functions | **4** |
| Total (top-level + subtest) | **19** |

### Versus Klaim Sebelumnya

| Klaim | Aktual | Perbedaan |
|---|---|---|
| "16 test functions" | **15** | Laporan menyebut 16, yang ada 15 top-level |
| "11 coverage tests + 5 direct/check" | **11 coverage + 4 direct** | `TestServiceCheckRWAccess_NilContext` dan `TestErrPermissionDenied_Is` tidak termasuk "direct check" |

### Semua Test di Package `rw`

```
go test -list . ./internal/rw/
```

Total test functions di package: **47** (dari output `go test -list`).

### Hasil Build & Test

```
cd backend && go build ./... → EXIT: 0 ✅
cd backend && go test ./internal/rw/ → ok social-finance/internal/rw ✅
```

### Apakah Test Memproduksi Produksi?

| Aspek | Ya/Tidak | Catatan |
|---|---|---|
| Memanggil production service method? | ✅ Ya | `svc.GetDashboard()`, `svc.GetKASReport()`, dll adalah method asli |
| Memanggil `checkRWPermission` langsung? | ✅ Ya | `TestServiceCheckRWPermission_Direct` memanggil `svc.checkRWPermission()` langsung |
| Memanggil `checkRWAccess` langsung? | ✅ Ya | `TestServiceCheckRWAccess_SAWithNoMembership` memanggil `svc.checkRWAccess()` langsung |
| Repository tidak dipanggil setelah permission ditolak? | ✅ Ya | `checkRWAccess` dan `checkRWPermission` return error sebelum `requirePermission` memanggil query |
| SA tetap dibatasi konfigurasi dan status RW? | ✅ Ya | Middleware `RequireRWScope` memeriksa `cfg.hasRW()` dan `is_active` sebelum SA bypass |
| Wakil Ketua tidak dapat update? | ✅ Ya | `TestPermissionEnforcement_JabatanWakilKetua` baris 93–96 |
| Sekretaris tidak dapat create/update/approve? | ✅ Ya | `TestPermissionEnforcement_JabatanSekretaris` baris 115–129 |
| Jabatan RT tidak memberikan akses RW? | ✅ Ya | `TestPermissionEnforcement_RTJabatanNoRW` baris 221–229 |
| Multi-role bekerja sesuai scope RW aktif? | ✅ Ya | `TestPermissionEnforcement_MultiRole` baris 190–208 |
| Resource lintas-RW ditolak? | ✅ Ya | `TestPermissionEnforcement_CrossRW` baris 282–296 |
| Error handler menghasilkan HTTP 403/404? | ⚠️ Sebagian | HTTP 403 ✅, tapi 404 untuk cross-RW resource belum ada (empty results) |

### HTTP Integration Test

Terdapat HTTP integration tests di `backend/internal/auth/rw_http_integration_test.go` yang menggunakan JWT nyata melalui `RequireAuth` dan `RequireRWScope` middleware. Ini lebih dari sekadar AuthContext injection — mereka menjalankan full middleware chain dengan database.

---

## 9. Hasil Build

```
cd backend && go build ./...
Exit code: 0 ✅
```

Build sukses. Tidak ada compilation error, warning, atau unused import.

---

## 10. Temuan Keamanan

### Critical: Tidak Ada

Tidak ditemukan celah kritis.

### High: Tidak Ada

Tidak ditemukan celah high severity.

### Medium: `requirePermission()` Menerima `ctx` Tapi Tidak Digunakan

**File:** `backend/internal/rw/service.go` baris 91

```go
func (s *Service) requirePermission(ctx context.Context, ac *auth.AuthContext, rwID string, permissionCode string) error {
```

Parameter `ctx` diterima tapi tidak digunakan di dalam fungsi. Ini adalah kode mati (dead code). Tidak mempengaruhi keamanan, tapi menunjukkan potensi masalah jika di masa depan `ctx` diperlukan untuk timeout/cancellation pada permission check.

**Risiko:** Low (tidak fungsional, tidak membahayakan).

### Low: Tidak Ada 404 Eksplisit untuk Cross-RW Resource

Ketika user dengan membership RW-A mencoba mengakses resource yang secara teknis ada tapi di RW-B, repository akan return empty results (0 rows) karena query parameterized. Ini mencegah data leakage sesuai kebijakan, tapi tidak memberikan feedback ke client bahwa resource tidak ditemukan vs. tidak ada data.

**Risiko:** Low (kebijakan keamanan, bukan celah).

### Low: Tidak Ada Rate Limiting di RW Endpoints

RW endpoints tidak memiliki rate limiting khusus selain yang mungkin sudah ada di middleware umum.

**Risiko:** Low (tergantung middleware global).

---

## 11. Daftar Pekerjaan yang Belum Tervalidasi

| Pekerjaan | Alasan |
|---|---|
| **Transfer Antar-Lokasi** | Belum diimplementasikan. Permission `finance.update` sudah tersedia di map, tapi tidak ada endpoint yang menggunakannya. |
| **POST/PUT/PATCH/DELETE RW endpoints** | Belum ada route RW selain GET. Tidak ada endpoint write untuk diuji. |
| **End-to-end HTTP integration test untuk permission denial** | `rw_http_integration_test.go` ada di package `auth` dan menguji middleware chain, tapi tidak ada test HTTP-level yang secara spesifik menguji error message untuk permission denied di response body. |
| **Performance/Load testing** | Tidak termasuk dalam scope audit ini. |
| **Audit test `TestServiceCheckRWPermission_Direct` subtest** | Subtest menggunakan `repo` yang nil (`NewService(pool, nil)`). Ini berarti test tidak benar-benar mengakses database — hanya menguji logic permission check. Untuk memastikan service method menolak operasi saat permission ditolak, test yang memanggil `svc.GetDashboard()` (yang memanggil `s.repo.GetDashboardSummary()`) akan panic jika repo nil. Perlu verifikasi bahwa test permission denial tidak mencoba memanggil query repo. |

### Detail: Test Service dengan Repo Nil

`TestServiceCheckRWPermission_Direct` baris 303–367 menggunakan:
```go
pool, _, rwID, _, _ := setupRWPermissionTest(t)
svc := NewService(pool, nil)  // repo = nil
```

Subtest ini memanggil `svc.checkRWPermission()` langsung, **tidak** memanggil `svc.GetDashboard()` atau method lain yang mengakses repo. Ini aman — permission check tidak mengakses database.

Tapi `TestPermissionEnforcement_JabatanKetua` dan test lainnya memanggil `svc.GetDashboard()` yang mengakses `s.repo.GetDashboardSummary()` — dan mereka menggunakan `setupRWPermissionTest()` yang mengembalikan repo yang valid (`repo := NewRepository(pool.Raw())`). Jadi test yang mengakses DB menggunakan repo yang benar.

---

## 12. Status Working Tree

**Konfirmasi tidak ada perubahan file selama audit.**

Dari `git status --short`:

### Modified Files (termasuk RW Permission):
- `backend/cmd/api/main.go` — RW endpoint registration
- `backend/internal/auth/scoping.go` — `HasRWScopePermission()`, `rwPermissionMap`
- `backend/internal/auth/scoping_test.go` — Test permission map
- `backend/internal/rw/service.go` — `checkRWAccess`, `checkRWPermission`, `requirePermission`
- `backend/internal/rw/handler.go` — `handleError()` dengan `ErrPermissionDenied`
- `docs/api.md`, `docs/database.md` — Documentation updates

### New Files:
- `RW_PERMISSION_ENFORCEMENT_DESIGN.md`
- `SECURITY_AUDIT_RW.md`
- `RW_PERMISSION_IMPLEMENTATION_REPORT.md`
- `backend/internal/rw/permission_enforcement_test.go`
- Migrations 042, 043
- Berbagai dokumen audit dan PR4

### Tidak Ada Perubahan Hilang atau Tertimpa

Semua file yang disebutkan dalam handoff briefing ada di working tree. Tidak ada file yang dihapus atau ditimpa.

---

## Ringkasan

| Aspek | Status |
|---|---|
| Permission map | ✅ Sesuai spesifikasi |
| Service-layer enforcement | ✅ Semua method dilindungi |
| SA bypass | ✅ Terbatas pada scope + active DB check |
| HTTP error handling | ✅ 401/403/400 benar, 404 belum ada (empty results) |
| Error wrapping | ✅ `errors.Is()` dengan `%w` |
| Tenant isolation | ✅ Semua query parameterized dengan `rw_id` |
| Balance computation | ✅ RW ledger independen, tidak dari RT |
| Test coverage | ✅ 15 top-level, 4 subtest, semua valid |
| Build | ✅ Clean build, exit 0 |
| Write endpoints | ⚠️ Belum diimplementasikan |

**Verdict Akhir: PASS — Implementasi aman dan sesuai desain yang disetujui.**
