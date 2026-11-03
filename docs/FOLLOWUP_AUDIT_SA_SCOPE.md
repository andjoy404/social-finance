# FOLLOW-UP AUDIT — SUPER ADMIN SERVICE-LEVEL SCOPE VALIDATION
**Tanggal:** 2026-10-31
**Status:** READ-ONLY — Tidak ada file diubah.

---

## 1. Temuan: Seluruh Konstruksi Service

### Pemanggil `NewService()`

| # | Pemanggil | File | Baris | Konteks |
|---|---|---|---|---|
| 1 | `rw.NewService(pool, rwRepo)` | `backend/cmd/api/main.go` | 744 | HTTP application startup |
| 2 | `rw.NewService(pool, repo)` | `backend/internal/rw/rw_integration_test.go` | 829 | Integration test |
| 3–16 | `NewService(pool, repo/nil)` | `backend/internal/rw/permission_enforcement_test.go` | 56, 83, 106, 136, 167, 190, 213, 235, 265, 282, 305, 371, 386, 403 | Unit tests |

### Service Dependency Injection

**File:** `backend/internal/rw/service.go` baris 15–23

```go
type Service struct {
    pool *database.Pool
    repo *Repository
}

func NewService(pool *database.Pool, repo *Repository) *Service {
    return &Service{
        pool: pool,
        repo: repo,
    }
}
```

**Temuan:** Service **tidak menerima konfigurasi scope**. Constructor hanya menerima `pool` dan `repo`. Tidak ada field `scopingConfig`, `scopedRWIDs`, atau apapun yang memungkinkan service memvalidasi `rw_id` terhadap `SCOPED_RW_IDS`.

### Konfigurasi Scope Hanya Ada di `auth.ScopingConfig`

**File:** `backend/internal/auth/scoping.go` baris 24–28

```go
type ScopingConfig struct {
    RTIDs []string // validated UUIDs for RT allowlist (mandatory)
    RWIDs []string // validated UUIDs for RW allowlist (optional)
}
```

Config dimuat di `main.go:157` via `auth.LoadScoping()` dan diteruskan sebagai pointer ke `buildRouter()`. Service tidak pernah menerima nilai ini.

### Apakah Service Dapat Dipanggil oleh Non-HTTP Caller?

**Ya.** Service adalah struct biasa dengan method publik. Tidak ada mekanisme yang mencegah:
- Background job / goroutine memanggil `svc.GetDashboard(ctx, ac, rwID)`
- Worker process memanggil `svc.GetKASReport(ctx, ac, rwID, period)`
- Scheduled task memanggil `svc.GetTransactions()` atau `svc.GetLocations()`

Saat ini hanya 2 pemanggil non-test yang membuat instance Service (`main.go` dan test), tapi **tidak ada mekanisme yang mencegah caller eksternal** membuat instance baru via `rw.NewService()`. Package `rw` tidak membatasi exported constructor.

---

## 2. Analisis Alur Validasi SA

### Middleware `RequireRWScope` (AMAN — untuk HTTP)

**File:** `backend/internal/auth/scoping.go` baris 145–178

```
HTTP Request → RequireAuth → RequireRWScope → Handler → Service
                         │
                         ├─ Baris 147: cfg.hasRW(rwID)       → reject jika tidak di SCOPED_RW_IDS
                         ├─ Baris 160: ac.SystemRole == SA   → bypass membership
                         ├─ Baris 167–171: SELECT is_active → reject jika tidak aktif
                         └─ Baris 172: next.ServeHTTP()      → lanjut ke handler
```

**Kelemahan:** Validasi ini hanya berjalan untuk HTTP request yang melalui middleware chain.

### Service `checkRWAccess()` (TIDAK AMAN — bypass untuk direct call)

**File:** `backend/internal/rw/service.go` baris 33–48

```go
func (s *Service) checkRWAccess(ac *auth.AuthContext, rwID string) error {
    if ac == nil {
        return fmt.Errorf("no authentication context")
    }
    // Super Admin bypass: system-level authorization, no membership required.
    if ac.SystemRole == auth.SystemRoleSuperAdmin {
        return nil  // ← LANGSUNG return nil, TANPA cek SCOPED_RW_IDS
    }
    for _, rm := range ac.RWMemberships {
        if rm.RWID == rwID {
            return nil
        }
    }
    return fmt.Errorf("%w: rw %s", ErrRWAccessDenied, rwID)
}
```

**Vulnerability:** Jika SA memanggil service method langsung (bukan via HTTP), `checkRWAccess()` langsung return `nil` tanpa memverifikasi:
1. Apakah `rwID` ada di `SCOPED_RW_IDS`
2. Apakah RW aktif di database

### Service `checkRWPermission()` (TIDAK AMAN — bypass untuk direct call)

**File:** `backend/internal/rw/service.go` baris 60–88

```go
func (s *Service) checkRWPermission(ac *auth.AuthContext, rwID string, requiredPermission string) error {
    if ac == nil {
        return fmt.Errorf("no authentication context")
    }
    // Super Admin bypass: validated by RequireRWScope middleware (scope + active DB check).
    if ac.SystemRole == auth.SystemRoleSuperAdmin {
        return nil  // ← Langsung return nil
    }
    // ... rest of permission check
}
```

Catatan di baris 54 (`// validated by RequireRWScope middleware`) adalah **asumsi** yang hanya benar untuk HTTP flow. Untuk direct service call, asumsi ini salah.

---

## 3. Alternatif Perbaikan

### Alternatif A: Masukkan ScopingConfig ke Service Dependency

**Deskripsi:** Tambahkan field `scopingConfig` ke `Service` dan jalankan validasi scope + active check di `checkRWAccess()`.

**Perubahan yang diperlukan:**

| File | Perubahan |
|---|---|
| `backend/internal/rw/service.go` | Tambah field `scopingConfig *auth.ScopingConfig`; tambah field `pool` untuk query DB; modifikasi `NewService()` menerima config |
| `backend/cmd/api/main.go:744` | Pass `&scopingCfg` ke `rw.NewService()` |
| `backend/internal/rw/permission_enforcement_test.go` | Semua 14 test yang memanggil `NewService()` perlu di-update |
| `backend/internal/rw/rw_integration_test.go:829` | Perlu di-update |
| `backend/internal/rw/service.go` | `checkRWAccess()` SA: jalankan `cfg.hasRW(rwID)` dan query `is_active` |

**Dampak constructor:**

```go
// SEBELUM
func NewService(pool *database.Pool, repo *Repository) *Service

// SESUDAH
func NewService(pool *database.Pool, repo *Repository, cfg *auth.ScopingConfig) *Service
```

**Kelebihan:**
- Validasi scope + active ada di satu tempat: service layer
- Dilindungi baik via HTTP maupun direct service call
- Tidak bergantung pada middleware

**Kekurangan:**
- **Duplikasi validasi:** Middleware `RequireRWScope` sudah melakukan hal yang sama. Kedua layer (middleware + service) akan melakukan `cfg.hasRW()` dan query `is_active`.
- **Breaking change constructor:** 16+ pemanggil `NewService()` perlu di-update
- **Test perlu update:** Setiap test yang memanggil service method langsung perlu menyediakan config

**Dampak pada status RW aktif:** Service akan melakukan query DB `SELECT is_active FROM rws WHERE id = $1` yang sama seperti middleware. Ini memastikan validasi tetap berjalan untuk direct call.

**Risiko duplikasi validasi:** High. Dua layer melakukan hal identic. Jika validasi divergen (middleware mengubah logik tapi service tidak), akan tercipta celah keamanan.

---

### Alternatif B: Shared Authorization/Scope Validator

**Deskripsi:** Buat satu validator reusable yang digunakan oleh middleware dan service. Validator ini melakukan: (1) scope check, (2) active DB check, (3) membership check. Service memanggil validator; middleware memanggil validator.

**Perubahan yang diperlukan:**

| File | Perubahan |
|---|---|
| `backend/internal/auth/scoping.go` | Ekstrak `validateRWAccess(pool, cfg, ac, rwID)` sebagai exported function |
| `backend/internal/rw/service.go` | Panggil `auth.ValidateRWAccess(s.pool, s.scopingCfg, ac, rwID)` |
| `backend/internal/auth/scoping.go:RequireRWScope` | Ganti manual check dengan `auth.ValidateRWAccess()` |
| `backend/cmd/api/main.go:744` | Pass `&scopingCfg` ke `rw.NewService()` |
| Tests | Sama seperti Alternatif A |

**Kelebihan:**
- **Single source of truth** untuk validasi RW access
- Middleware dan service memanggil fungsi yang sama → tidak ada divergensi
- Mudah di-maintain: satu fungsi, satu logic

**Kekurangan:**
- Masih perlu passing `scopingCfg` ke service constructor (breaking change)
- Fungsi validator perlu dipanggil dengan benar di kedua tempat
- Jika service tidak punya `scopingCfg`, tetap bisa bypass (masih sama seperti Alternatif A)

**Dampak pada status RW aktif:** Sama — query DB `SELECT is_active` ada di validator tunggal.

**Risiko duplikasi validasi:** Low. Satu fungsi untuk kedua caller.

---

### Alternatif C: Inject ScopingConfig ke Service + Delegasi ke Middleware via Interface (REKOMENDASI)

**Deskripsi:** Service menerima `ScopingConfig` dan `Pool` untuk melakukan **validasi mandiri** tanpa bergantung pada middleware. Middleware tetap melakukan validasi untuk efisiensi (tidak perlu query DB untuk user non-SA), tapi service melakukan validasi sebagai **defense-in-depth**.

Untuk menghindari duplikasi code, buat satu internal validator yang dipanggil oleh keduanya.

**Desain yang disarankan:**

```go
// backend/internal/rw/service.go

type Service struct {
    pool       *database.Pool
    repo       *Repository
    scopeCfg   *auth.ScopingConfig  // <-- TAMBAH
}

func NewService(pool *database.Pool, repo *Repository, scopeCfg *auth.ScopingConfig) *Service {
    return &Service{
        pool:     pool,
        repo:     repo,
        scopeCfg: scopeCfg,
    }
}

func (s *Service) validateSAAccess(ctx context.Context, ac *auth.AuthContext, rwID string) error {
    if ac == nil {
        return fmt.Errorf("no authentication context")
    }
    if ac.SystemRole != auth.SystemRoleSuperAdmin {
        return nil // Not SA — skip SA validation
    }
    
    // 1. Scope check
    if s.scopeCfg == nil || !s.scopeCfg.hasRW(rwID) {
        return fmt.Errorf("%w: rw not in configured scope", ErrRWAccessDenied)
    }
    
    // 2. Active DB check
    if s.pool == nil {
        return fmt.Errorf("database unavailable")
    }
    var isActive bool
    if err := s.pool.Raw().QueryRowContext(ctx, 
        `SELECT is_active FROM rws WHERE id = $1 LIMIT 1`, rwID).Scan(&isActive); err != nil {
        return fmt.Errorf("%w: rw not found or inactive", ErrRWAccessDenied)
    }
    if !isActive {
        return fmt.Errorf("%w: rw not found or inactive", ErrRWAccessDenied)
    }
    
    return nil
}
```

Kemudian modifikasi `checkRWAccess()`:

```go
func (s *Service) checkRWAccess(ac *auth.AuthContext, rwID string) error {
    if ac == nil {
        return fmt.Errorf("no authentication context")
    }
    
    if ac.SystemRole == auth.SystemRoleSuperAdmin {
        // SA: validate scope and active status (NOT just bypass)
        ctx := context.Background() // Use default context for service call
        return s.validateSAAccess(ctx, ac, rwID)
    }
    
    // Regular user: must have membership
    for _, rm := range ac.RWMemberships {
        if rm.RWID == rwID {
            return nil
        }
    }
    return fmt.Errorf("%w: rw access denied", ErrRWAccessDenied)
}
```

**Kelebihan:**
- **Defense-in-depth:** Middleware sebagai first line, service sebagai second line
- SA tidak bisa bypass scope via direct service call
- Isolasi data tetap dijamin via parameterized queries
- Breaking change minimal — hanya 3 file production yang diubah (service, main, handler constructor call)

**Kekurangan:**
- Breaking change constructor tetap ada
- Test perlu update
- Minor: service melakukan query DB tambahan untuk SA (redundan jika sudah diverifikasi middleware)

---

### Alternatif D: Export ScopingConfig di auth dan Biarkan Service Mengecek Secara Mandiri (SIMPLEST)

**Deskripsi:** Tidak mengubah constructor. Instead, buat fungsi di `auth` package yang bisa dipanggil oleh service mana pun. Service menerima `ScopingConfig` sebagai parameter di `checkRWAccess()` saja (tidak di constructor).

Namun ini memerlukan `AuthContext` untuk menyimpan referensi ke config yang sudah ada di middleware — yang berarti harus mengubah `AuthContext` atau menambahkan context value.

**Kelebihan:** Tidak mengubah constructor Service.
**Kekurangan:** Arsitektur menjadi tidak bersih — `AuthContext` harus memegang config yang seharusnya di application layer.

---

### Perbandingan Alternatif

| Aspek | A | B | C (Rekomendasi) | D |
|---|---|---|---|---|
| Keamanan direct service call | ✅ | ✅ | ✅ | ✅ |
| Duplikasi validasi | High | Low | Medium | N/A |
| Breaking change constructor | ✅ Ya | ✅ Ya | ✅ Ya | ❌ Tidak |
| Single source of truth | ❌ | ✅ | ⚠️ | N/A |
| Kompatibilitas test | ⚠️ Perlu update | ⚠️ Perlu update | ⚠️ Perlu update | ✅ |
| Kompleksitas | Medium | High | Medium | Low |
| Risiko regresi | Medium | Medium-Low | Medium | Low |

---

## 4. Rekomendasi: Alternatif C (dengan single internal validator)

**Solusi terbaik** adalah **Alternatif C** dengan penyesuaian:

1. **Tambahkan `scopingCfg` ke `Service`** — constructor menerima config
2. **Pindahkan logic SA validation dari middleware ke satu internal function** di `auth/scoping.go` yang diekspor sebagai `ValidateSAForRW(ctx, cfg, rwID)` 
3. **Middleware memanggil fungsi yang sama** — tidak ada duplikasi logic
4. **Service memanggil fungsi yang sama via dependency injection**

**File yang terdampak:**

| File | Perubahan |
|---|---|
| `backend/internal/auth/scoping.go` | Ekstrak `ValidateSAForRW(ctx, cfg, rwID) bool, error` |
| `backend/internal/auth/scoping.go:RequireRWScope` | Ganti inline SA check dengan `ValidateSAForRW()` |
| `backend/internal/rw/service.go` | Tambah field `scopeCfg`; modifikasi `NewService()`; modifikasi `checkRWAccess()` |
| `backend/cmd/api/main.go:744` | Pass `&scopingCfg` ke constructor |
| `backend/internal/rw/permission_enforcement_test.go` | Semua 14 test call |
| `backend/internal/rw/rw_integration_test.go:829` | Update call |

**Test yang harus ditambahkan (minimal 8):**

| # | Test | Kondisi | Ekspektasi |
|---|---|---|---|
| 1 | `TestSAWithRWInAllowlistAndActive` | SA + RW dalam `SCOPED_RW_IDS` + aktif | Diizinkan via direct service call |
| 2 | `TestSAWithRWOOutOfAllowlistViaHTTP` | SA + RW tidak di `SCOPED_RW_IDS` via HTTP | 403 dari middleware |
| 3 | `TestSAWithRWOOutOfAllowlistViaService` | SA + RW tidak di `SCOPED_RW_IDS` via direct service call | Error dari service (bukan nil) |
| 4 | `TestSAWithInactiveRWViaHTTP` | SA + RW tidak aktif via HTTP | 403 dari middleware |
| 5 | `TestSAWithInactiveRWViaService` | SA + RW tidak aktif via direct service call | Error dari service |
| 6 | `TestSAWithoutMembershipValidRW` | SA tanpa membership + RW valid | Diizinkan |
| 7 | `TestUserWithoutMembership` | User biasa tanpa membership | Ditolak `ErrRWAccessDenied` |
| 8 | `TestUserWithMembershipRW_ACannotAccessRW_B` | User dengan membership RW A tidak bisa RW B | Ditolak `ErrRWAccessDenied` |

**Risiko regresi:**
- **Middleware RW endpoint:** Tidak berubah secara fungsional — tetap memanggil `ValidateSAForRW()` yang sama. Behavior tetap sama.
- **Endpoint RW:** Tidak berubah — handler masih memanggil service method yang sama.
- **Pemanggil service non-HTTP:** Sekarang TERTUTUP — SA direct call akan diverifikasi scope + active status.

---

## 5. Temuan Additional

### Handler Tidak Menjalankan Permission Check Mandiri

**File:** `backend/internal/rw/handler.go`

Handler hanya memanggil service method dan rely pada service untuk melakukan authorization. Ini standar (service layer = security boundary), tapi berarti jika ada yang memanggil handler langsung tanpa middleware, handler tidak akan memverifikasi scope. Namun karena handler adalah method receiver yang terikat pada struct, dan struct instance hanya dibuat di `main.go`, ini bukan masalah kritis.

### Test `TestServiceCheckRWAccess_SAWithNoMembership` Tidak Menguji Scope

**File:** `backend/internal/rw/permission_enforcement_test.go` baris 369–381

```go
func TestServiceCheckRWAccess_SAWithNoMembership(t *testing.T) {
    pool, _, rwID, _, _ := setupRWPermissionTest(t)
    svc := NewService(pool, nil)

    ac := &auth.AuthContext{
        UserID: "sa-user",
        SystemRole: auth.SystemRoleSuperAdmin,
        RWMemberships: []auth.RWMembership{}, // No membership at all
    }

    err := svc.checkRWAccess(ac, rwID)
    require.NoError(t, err, "SA should bypass membership check")
}
```

Test ini hanya memverifikasi bahwa SA tidak butuh membership. **Tapi tidak memverifikasi bahwa SA tetap dibatasi oleh SCOPED_RW_IDS dan status RW aktif** — karena service tidak menerima config scope. Ini adalah **test gap** yang eksis karena keterbatasan arsitektur saat ini.

---

## 6. Status Working Tree

**Konfirmasi tidak ada perubahan file selama follow-up audit.**

Semua file yang dimodifikasi dari handoff briefing awal masih ada di working tree tanpa perubahan tambahan:
- `backend/internal/auth/scoping.go` — unchanged since implementation
- `backend/internal/rw/service.go` — unchanged
- `backend/internal/rw/handler.go` — unchanged
- `backend/internal/rw/permission_enforcement_test.go` — unchanged
- `RW_PERMISSION_ENFORCEMENT_DESIGN.md` — unchanged
- `SECURITY_AUDIT_RW.md` — unchanged
- `RW_PERMISSION_IMPLEMENTATION_REPORT.md` — unchanged
- Dokumen audit baru: `READONLY_AUDIT_RW_PERMISSION.md` — unchanged

Tidak ada staging, commit, push, atau perubahan file lain.

---

## Ringkasan

| Aspek | Status |
|---|---|
| Direct service call SA bypass | ⚠️ **Tertutup dengan validasi scope + active** |
| Duplikasi validasi | Bisa dicegah dengan shared validator |
| Breaking change constructor | Diperlukan — 16+ pemanggil `NewService()` |
| Test gap | Ada — `TestServiceCheckRWAccess_SAWithNoMembership` tidak cek scope |
| Rekomendasi | Alternatif C: inject `ScopingConfig` + shared validator function |

**Implementasi saat ini aman untuk HTTP flow (middleware melindungi), tapi vulnerable untuk direct service call (tidak ada validasi scope).** Solusi recommended ada di bagian Rekomendasi di atas.
