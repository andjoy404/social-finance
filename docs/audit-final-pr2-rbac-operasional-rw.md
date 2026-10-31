# Audit Final PR 2 — RBAC Operasional RW

**Tanggal:** 2026-10-29 23:03 WIB  
**Branch:** `urena` — HEAD `ac266c0`  
**Mode:** Verifikasi saja (tanpa perubahan kode)

---

## 1. Ringkasan Status PR 2

Wiring PR 2 telah selesai dan terverifikasi. `LoadScoping()` dipanggil pada startup di `main.go`, konfigurasi diteruskan ke `buildRouter()`, middleware `RequireRWScope()` siap untuk didaftarkan di PR 4.

**8 files modified (PR 2 wiring):**
- `backend/cmd/api/main.go` (+11/-2) — LoadScoping() call, buildRouter signature change
- `backend/cmd/api/router_auth_test.go` (+6/-1) — dummy ScopingConfig untuk tes router
- `backend/internal/auth/jwt.go` (+15/-5) — RWContext field di TokenClaims
- `backend/internal/auth/jwt_test.go` (+113/-0) — 3 tes serialisasi RW context
- `backend/internal/auth/middleware.go` (+82/-5) — Parse RWContext JSON, per-request DB validation
- `backend/internal/auth/models.go` (+20/-5) — RWMembership struct, RWMemberships di AuthContext
- `backend/internal/auth/service.go` (+110/-10) — loadRWMemberships(), buildRWContextJSON() di 4 jalur token
- `backend/internal/fund_location/handler.go` (+25/-0) — checkRWBendaharaAccess() (belum terhubung endpoint)

**3 file baru (code):**
- `backend/internal/auth/scoping.go` — ScopingConfig, LoadScoping(), RequireRWScope(), rwPermissionMap
- `backend/internal/auth/scoping_test.go` — 15 test functions + subtests
- `backend/internal/fund_location/handler_security_test.go` — 9 subtests checkRWBendaharaAccess()

**7 file baru (docs):** Untracked dokumentasi rbac-operasional-*.md dan verifikasi-*.md

**Total:** +345 insertions, -37 deletions

---

## 2. Bukti Status Masing-Masing Tes Gagal

### 2A. `TestRBAC_FinanceReadCannotApproveWithoutFinanceApprove`

**Perintah:** `go test ./cmd/api -run TestRBAC_FinanceReadCannotApproveWithoutFinanceApprove -count=1 -v`  
**Exit code:** 1 (FAIL)

**Error aktual:**
```
expected HTTP 403 for wakil_ketua attempting to approve payment without finance.approve, got 400: {"error":{"code":"validation_error","message":"action must be 'approve' or 'reject'"}}
```

**Analisis mendalam:**

**Test mengirim** (line 577 di `bills_rbac_test.go`):
```json
{"action":"APPROVED"}
```

**Service menerima** (line 1004-1007 di `internal/finance/service.go`):
```go
action := strings.ToLower(strings.TrimSpace(in.Action))
if action != "approve" && action != "reject" {
    return nil, errors.New("action must be 'approve' or 'reject'")
}
```

**Flow aktual:**
1. Test mengirim `{"action":"APPROVED"}`
2. `strings.ToLower("APPROVED")` → `"approved"` (dengan suffix -D)
3. `"approved"` ≠ `"approve"` → reject di validasi action
4. HTTP 400 validation_error
5. **Pemeriksaan RBAC TIDAK PERNAH TERCAPAI**

**Flow yang diharapkan:**
1. Test mengirim `{"action":"APPROVE"}` (tanpa suffix -D)
2. `strings.ToLower("APPROVE")` → `"approve"`
3. Validasi action pass
4. Middleware RBAC memeriksa: wakil_ketua lacks `finance.approve`
5. HTTP 403 Forbidden

**Bukti pre-existing melalui commit dasar:**

File `bills_rbac_test.go` **tidak ada di daftar perubahan PR 2** (tidak dimodifikasi oleh wiring):
```
git diff --name-only:
backend/cmd/api/main.go
backend/cmd/api/router_auth_test.go
backend/internal/auth/jwt.go
backend/internal/auth/jwt_test.go
backend/internal/auth/middleware.go
backend/internal/auth/models.go
backend/internal/auth/service.go
backend/internal/fund_location/handler.go
```

Kode pada commit dasar sama dengan HEAD:
```
git show HEAD:backend/cmd/api/bills_rbac_test.go | sed -n '577p'
→ verifyBody := bytes.NewBufferString(`{"action":"APPROVED"}`)
```

Service validation code juga tidak berubah (ada sebelum PR 2).

**Rekomendasi perbaikan:** Ganti line 577 dari `"APPROVED"` menjadi `"APPROVE"`:
```go
verifyBody := bytes.NewBufferString(`{"action":"APPROVE"}`)
```

**Sama juga di TestRBAC_WargaCannotVerifyPayment** (line 636) — mengirim `"APPROVED"` yang seharusnya `"APPROVE"`.

**Perbandingan dengan test lain yang sukses:** Test di `internal/finance/multi_month_test.go:516` menggunakan `{"action": "APPROVE"}` (tanpa -D) dan sukses.

**Kesimpulan:** ✅ **PRE-EXISTING TERBUKTI** — Bug di test code sejak commit `7a2cf5d` (`feat(finance,ui): payment status filter, verification ui polish, and rbac regression fixes`). Bukan akibat wiring PR 2.

---

### 2B. `TestBootstrap`

**Perintah:** `go test ./cmd/api -run TestBootstrap -count=1 -v`  
**Exit code:** 1 (FAIL)

**Error aktual:**
```
fatal: required environment variable "DB_HOST" is not set
```

**Analisis mendalam:**

Test ini memanggil `runBootstrap()` yang menggunakan `config.Load()`. Config loader membaca `DB_HOST` dari environment dan **tidak memiliki default fallback**.

`setBootstrapEnv()` hanya set:
```go
os.Setenv("DB_NAME", "social_finance_test")
os.Setenv("BOOTSTRAP_EMAIL", "admin@test.local")
os.Setenv("BOOTSTRAP_PASSWORD", "TestUser123!")
os.Setenv("BOOTSTRAP_NAME", "Super Admin")
```

**Tidak set `DB_HOST`**, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, atau `DB_SSLMODE`.

**Perlu dicatat:** `testutil.GetTestPool()` yang dipanggil sebelumnya di test memiliki fallback default:
```go
dbHost := os.Getenv("DB_HOST")
if dbHost == "" {
    dbHost = "localhost"  // fallback ada di sini
}
```

Tapi `runBootstrap()` → `config.Load()` menggunakan jalur konfigurasi produksi yang berbeda dan **tidak memiliki fallback**.

**File tidak dimodifikasi PR 2:** `bootstrap_test.go` tidak ada di daftar perubahan PR 2 wiring.

**Kesimpulan:** ✅ **PRE-EXISTING TERBUKTI** — Kegagalan karena environment variable yang diperlukan tidak diset di `setBootstrapEnv()`. Bukan akibat wiring PR 2.

**Rekomendasi perbaikan:** Tambahkan ke `setBootstrapEnv()`:
```go
os.Setenv("DB_HOST", "localhost")
os.Setenv("DB_PORT", "5432")
os.Setenv("DB_USER", "social_finance")
os.Setenv("DB_PASSWORD", "social_finance_dev_password")
os.Setenv("DB_SSLMODE", "prefer")
```

---

## 3. Hasil Audit Konfigurasi dan Otorisasi Scope

### 3.1 `LoadScoping()` dipanggil sebelum router mulai menerima request

**Verifikasi:** ✅ YA

```go
// main.go:156-162
scopingCfg, err := auth.LoadScoping()
if err != nil {
    slog.Error("startup failed: invalid scoping configuration", "error", err)
    os.Exit(1)
}
router := buildRouter(pool, authHandler, &scopingCfg, store)
```

Order: `LoadScoping()` → error check → `buildRouter()` → server start.

### 3.2 Kegagalan validasi konfigurasi menghentikan startup secara aman

**Verifikasi:** ✅ YA

`os.Exit(1)` di `main()` — bukan di router atau handler. Ini adalah entrypoint yang tepat.

`LoadScoping()` fail-closed:
- `SCOPED_RT_IDS` mandatory ≥ 1 valid UUID
- `SCOPED_RW_IDS` mandatory ≥ 1 valid UUID
- Tanpa konfigurasi valid → application refuse to start

### 3.3 `SCOPED_RT_IDS` dan `SCOPED_RW_IDS` diwajibkan

**Verifikasi:** ✅ YA

```go
if len(rtIDs) == 0 {
    return ScopingConfig{}, fmt.Errorf("SCOPED_RT_IDS is mandatory and must contain at least one valid UUID")
}
if len(rwIDs) == 0 {
    return ScopingConfig{}, fmt.Errorf("SCOPED_RW_IDS is mandatory and must contain at least one valid UUID")
}
```

Tidak ada fallback ke "access all."

### 3.4 UUID konfigurasi divalidasi

**Verifikasi:** ✅ YA

`isValidUUID()` memeriksa format 36 karakter dengan hyphen di posisi 8, 13, 18, 23. Tidak ada UUID tidak valid yang lolos.

### 3.5 `ScopingConfig` benar-benar digunakan dalam resolusi scope

**Verifikasi:** ✅ YA — bukan hanya diteruskan tapi benar-benar dipakai

Di `RequireRWScope()`:
- `cfg.hasRW(rwID)` — dicek apakah rwId ada di SCOPED_RW_IDS allowlist
- `cfg.hasRT(rtID)` — tersedia untuk pengecekan RT di masa depan

```go
// RequireRWScope middleware - line ~148
if !cfg.hasRW(rwID) {
    httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "rw not in super_admin scope")
    return
}
```

### 3.6 Tidak ada mutable package global yang menyebabkan konfigurasi berbeda antar-router/test

**Verifikasi:** ✅ AMAN

`ScopingConfig` diteruskan sebagai pointer `*auth.ScopingConfig` ke `buildRouter()`. Tidak ada global mutable. Test menggunakan `setupRouterTest()` yang membuat instance baru `ScopingConfig` dengan dummy UUID.

### 3.7 `RequireRWScope()` tidak mengizinkan akses RW tanpa validasi yang diperlukan

**Verifikasi:** ✅ YA — 3 jalur validasi:

**Jalur 1 — System-only SUPER_ADMIN:**
1. Cek `ac.SystemRole == super_admin && ac.RTID == "" && ac.MembershipID == ""`
2. `len(ac.RWMemberships) > 0` — harus punya membership di JWT
3. `cfg.hasRW(rwID)` — rwId harus ada di SCOPED_RW_IDS allowlist
4. Query DB: `SELECT is_active FROM rws WHERE id = $1` — RW harus ada dan aktif
5. Jika DB tidak accessible (`pool == nil`) → HTTP 500, fail closed

**Jalur 2 — Users dengan RW memberships:**
1. `len(ac.RWMemberships) > 0` — harus punya membership
2. `rm.RWID == rwID` — rwId harus cocok dengan salah satu membership
3. Jika tidak cocok → HTTP 403

**Jalur 3 — Tanpa context:**
1. Missing auth context → HTTP 401
2. Invalid path /rw/{rwId}/ → HTTP 400

### 3.8 Super Admin tetap dibatasi pada scope

**Verifikasi:** ✅ YA

SA tidak mendapatkan akses RW otomatis. Harus:
1. Punya RW membership di JWT
2. RW membership ada di SCOPED_RW_IDS allowlist
3. RW active di database

### 3.9 Membership RW aktif divalidasi terhadap database

**Verifikasi:** ✅ YA (untuk SA)

```go
err = tx.QueryRowContext(ctx, `SELECT is_active FROM rws WHERE id = $1 LIMIT 1`, rwID).Scan(&isActive)
tx.Rollback()
if err == sql.ErrNoRows || !isActive {
    httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "rw not found or inactive")
    return
}
```

**Catatan:** Untuk user non-SA (bendahara RT, pengurus), validasi dilakukan pada JWT token saat login/refresh. Tidak ada per-request DB validation untuk user biasa — ini sesuai desain karena membership sudah terverifikasi saat token generation.

---

## 4. Audit Keamanan Helper Transfer

### `checkRWBendaharaAccess()` — Implementasi

```go
func (h *Handler) checkRWBendaharaAccess(ac *auth.AuthContext, rwID string) error {
    if ac.SystemRole == auth.SystemRoleSuperAdmin {
        return nil // SA validated by RequireRWScope middleware first
    }
    for _, rm := range ac.RWMemberships {
        if rm.RWID == rwID && rm.Jabatan == "bendahara" {
            return nil
        }
    }
    return errors.New("only bendahara can execute or reverse fund transfers at RW level")
}
```

**Verifikasi checklist:**

| Kriteria | Status |
|----------|--------|
| RT bendahara tidak otomatis dapat transfer RW | ✅ Hanya cek `RWMemberships`, bukan RT memberships |
| Membership harus cocok rwId | ✅ `rm.RWID == rwID` |
| Jabatan harus bendahara | ✅ `rm.Jabatan == "bendahara"` |
| Ketua/Wakil Ketua/Sekretaris diblokir | ✅ Tidak ada entry lain yang match |
| SA hanya aman jika RequireRWScope sudah dijalankan | ✅ Comment menjelaskan ini, SA lewat dulu baru dicek handler |
| Database error tidak menghasilkan izin | ✅ Tidak ada kode DB di fungsi ini |
| Call site non-test | **HANYA di test file** — belum ada endpoint yang memanggil |

**Call sites (semua di test):**
- `handler_security_test.go:20,35,51,65,79,93,105,126,149` — unit tests
- `scoping_test.go:130` — comment reference

**Call sites (production):** **TIDAK ADA** — `checkRWBendaharaAccess()` belum terhubung ke endpoint manapun.

**Kesimpulan:** Fungsi ini ada dan benar secara logic, tapi **belum teruji end-to-end** karena endpoint operasional RW belum ada. Saat PR 4 menambahkan endpoint transfer RW, handler harus memanggil `checkRWBendaharaAccess(ac, rwID)` sebelum melakukan operasi transfer.

---

## 5. Daftar Tes Lulus/Gagal (dengan Exit Code)

| Test Command | Exit Code | Hasil |
|-------------|-----------|-------|
| `go test ./internal/auth/ -count=1` | **0** | ✅ PASS — 19 test functions, ~4.7s |
| `go test ./internal/fund_location/ -count=1` | **0** | ✅ PASS — 10 test cases, ~1.5s |
| `go test ./cmd/api -run TestRouter -count=1` | **0** | ✅ PASS — TestRouterHasHealthRoute, TestRouter404 |
| `go test ./cmd/api -run TestRBAC_FinanceReadCannotApproveWithoutFinanceApprove -count=1` | **1** | ❌ FAIL — Pre-existing test bug (APPROVED vs APPROVE) |
| `go test ./cmd/api -run TestBootstrap -count=1` | **1** | ❌ FAIL — Pre-existing missing env var (DB_HOST) |
| `go build ./...` | **0** | ✅ SUKSES |
| `git diff --check` | **0** | ✅ Clean — no whitespace error |

**Penting:** Tidak melabeli seluruh suite sebagai PASS karena `go test ./cmd/api` masih gagal 2/2 tes yang di-run. Kegagalan ini adalah pre-existing, bukan akibat wiring PR 2.

---

## 6. Daftar Blocker yang Masih Tersisa

Untuk **PR 4 (Backend RW endpoints)** — langkah yang diperlukan:

1. **Daftarkan `RequireRWScope` middleware** pada route group RW:
   ```go
   r.Group(func(r chi.Router) {
       r.Use(auth.RequireAuth)
       r.Use(auth.RequireRWScope(pool, scopingCfg))
       // RW endpoints here
   })
   ```

2. **Panggil `checkRWBendaharaAccess()` di handler transfer** sebelum operasi write.

3. **Set environment variable** untuk production:
   - `SCOPED_RT_IDS` — comma-separated UUID RT yang diizinkan
   - `SCOPED_RW_IDS` — comma-separated UUID RW yang diizinkan

Untuk **Pre-existing test bugs** (tidak harus diselesaikan untuk PR 2):

4. **Fix `TestRBAC_FinanceReadCannotApproveWithoutFinanceApprove`:** Ganti `"APPROVED"` → `"APPROVE"` di `bills_rbac_test.go:577` dan `636`.

5. **Fix `TestBootstrap`:** Tambahkan `DB_HOST` dan variabel DB lain ke `setBootstrapEnv()` di `bootstrap_test.go`.

---

## 7. Kesimpulan

**STATUS: PASS — wiring terverifikasi**

Wiring PR 2 RBAC Operasional RW telah selesai dan berfungsi sesuai desain:
- `LoadScoping()` dipanggil pada startup → fail-closed jika env var tidak valid
- `ScopingConfig` diteruskan ke router → digunakan dalam `RequireRWScope()` middleware
- Token RW context → serialized ke JWT, validated per-request saat login/refresh
- Middleware `RequireRWScope()` → 3 jalur validasi (SA scope, user membership, path extraction)
- Helper `checkRWBendaharaAccess()` → siap untuk dihubungkan ke endpoint di PR 4

**Batas cakupan PR 2:**
- Endpoint operasional RW belum ada → `RequireRWScope()` belum didaftarkan ke router (akan dilakukan PR 4)
- `checkRWBendaharaAccess()` belum dipanggil handler manapun (belum teruji end-to-end)
- Tidak ada klaim bahwa RBAC operasional RW telah selesai secara end-to-end

**Kegagalan tes** adalah pre-existing dan tidak terkait PR 2 wiring.

---

*Audit selesai. Tidak ada file yang diubah.*
