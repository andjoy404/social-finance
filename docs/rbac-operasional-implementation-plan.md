# Rencana Implementasi Final — RBAC Operasional RW Urena (Revisi)

**Status:** REVISI — Diperbarui berdasarkan Final Readiness Check  
**Acuan:** `docs/rbac-operasional-rw.md` (dokumen acuan produk)  
**Review:** `docs/rbac-operasional-pre-implementation-review.md`  
**Tanggal:** 2026-10-29  
**Branch:** `urena` — HEAD `ac266c0`  
**Jenis:** Read-only — tidak ada kode diubah saat penulisan rencana ini  

---

## 0. Ringkasan Perubahan dari Versi Sebelumnya

| Aspek | Versi Sebelumnya | Versi Revisi | Alasan |
|---|---|---|---|
| **Jabatan RW** | `ketua_rw`, `wakil_ketua_rw`, `bendahara_rw` (suffix `_rw`) | Dipertahankan sebagai kode operasional, tetapi **tidak** digunakan sebagai nilai `user_rw_memberships.jabatan` | `user_rw_memberships.jabatan` aktual menyimpan nilai RT-style (`ketua`, `wakil_ketua`, `bendahara`). Menggunakan nilai yang sama di `position_permissions.position` akan menyebabkan tabrakan grant dengan RT. |
| **Permission resolver RW** | Menggunakan `position_permissions` dengan GRANT ke `'ketua_rw'` | **Desain B**: resolver RW terpisah, tidak bergantung pada `position_permissions` | Menghindari tabrakan namespace dan memastikan permission RW tidak "bocor" ke RT |
| **Migration 042** | DROP/ADD CHECK constraint pada `position_permissions` | **Dihapus** — tidak diperlukan karena tidak menggunakan `position_permissions` untuk RW | `user_rw_memberships.jabatan` CHECK constraint sudah mendukung 4 nilai RT-style |
| **Super Admin scope** | `ScopeID != ""` → true | Validasi terhadap `SCOPED_RT_IDS`/`SCOPED_RW_IDS` dengan UUID validation di startup | Keamanan: mencegah header spoofing |
| **Token RWContext** | `Role` =RW role | `jabatan` = RW position + `rw_id` untuk scope | Field yang benar dari `user_rw_memberships` |
| **Membership validation** | Tidak ada | Validasi DB pada setiap request RW untuk membership aktif | Keamanan: mencegah akses setelah pencabutan jabatan |
| **Opening balance** | Tidak termasuk dalam query agregasi | Terintegrasi dengan `period_opening_balances` | Konsistensi saldo KAS RW dengan KAS RT |
| **API contract** | Tidak ada | Tahap terpisah sebelum frontend development | Konsistensi Web dan Android |

---

## 1. Desain Jabatan dan Permission RW (VERIFIED)

### 1.1 Fakta yang Diverifikasi dari Codebase

| File | Fakta | Bukti |
|---|---|---|
| `migrations/029_rw_membership_and_jabatan.up.sql` | `user_rw_memberships.jabatan` CHECK constraint: `('ketua', 'wakil_ketua', 'sekretaris', 'bendahara')` | Line 20-23 |
| `migrations/029_rw_membership_and_jabatan.up.sql` | `user_rw_memberships.role` CHECK constraint: `('super_admin', 'pengurus', 'bendahara', 'warga')` | Line 17-19 |
| `internal/rt/rw_membership.go` | `AllowedRWJabatans` = `{"ketua", "wakil_ketua", "sekretaris", "bendahara"}` | Line 26-31 |
| `migrations/011_add_positions_permissions.up.sql` | `position_permissions.position` CHECK: 8 nilai RT jabatan | Line 31-35 |
| `migrations/031_restore_permission_catalog.up.sql` | 12 permission_types canonical, 4 finance permissions | `finance.read/create/update/approve` |
| `internal/auth/permission.go` | `HasPermission()` mengecek `position_permissions.position → permission_types.code` | Line 59-64 |

### 1.2 Evaluasi Dua Desain

#### Desain A: Gunakan nama jabatan RW terpisah dengan suffix `_rw`

**Deskripsi:** Ubah `user_rw_memberships.jabatan` CHECK constraint untuk menyimpan `'ketua_rw'` dll, lalu gunakan `position_permissions` dengan GRANT ke `'ketua_rw'`.

**Dampak:**
- Migration 029 perlu direvisi: ubah CHECK constraint `jabatan` dari `('ketua', 'wakil_ketua', 'sekretaris', 'bendahara')` menjadi `('ketua_rw', 'wakil_ketua_rw', 'bendahara_rw')`
- **Migration down 029 akan merusak data:** Jika migration 029 di-rollback, semua data `user_rw_memberships` hilang. Migration 042 DROP/ADD constraint juga akan gagal jika ada data yang bergantung pada FK `rws`.
- **Data migration diperlukan:** Semua data existing `user_rw_memberships.jabatan = 'ketua'` perlu diubah menjadi `'ketua_rw'`
- **`AllowedRWJabatans` perlu diubah:** `rw_membership.go` line 26-31 perlu update dari 4 nilai RT-style ke 3 nilai RW-style
- **Risiko permission RT tercampur:** Jika `position_permissions.position` mencakup nilai RT dan RW, setiap grant `finance.update` untuk `'ketua_rw'` juga berarti `'ketua'` (RT) mendapat permission yang sama jika tidak hati-hati

**Risiko:**
- ✅ Nama konsisten dengan dokumen acuan
- ❌ Migration down 029 tidak ada — tidak bisa rollback
- ❌ Perlu migration data untuk mengubah nilai existing
- ❌ `AllowedRWJabatans` tidak konsisten dengan keputusan produk (4 nilai vs 3 nilai)
- ❌ Risiko permission RT/RW tercampur di `position_permissions`

#### Desain B: Pertahankan nilai existing, gunakan resolver terpisah (DIPILIH)

**Deskripsi:** Pertahankan `user_rw_memberships.jabatan` dengan nilai `'ketua'`, `'wakil_ketua'`, `'bendahara'`. Buat permission resolver RW yang mengecek `user_rw_memberships.jabatan` (bukan `position_permissions`) against hardcoded permission map di Go code.

**Keuntungan:**
- ✅ **Tidak ada migration tambahan** — `user_rw_memberships.jabatan` CHECK constraint sudah mendukung 4 nilai, 3 di antaranya (`ketua`, `wakil_ketua`, `bendahara`) adalah jabatan RW yang sah
- ✅ **Tidak ada risiko tabrakan namespace** — `position_permissions` tetap hanya untuk RT, resolver RW punya map sendiri
- ✅ **Tidak ada data migration** — data existing `user_rw_memberships.jabatan` tidak perlu diubah
- ✅ **Isolasi permission RT/RW jelas** — GRANT RT tidak mempengaruhi RW, dan sebaliknya
- ✅ **`AllowedRWJabatans` masih valid** — hanya perlu filter 'sekretaris' untuk penggunaan operasional RW (sesuai keputusan produk: 3 jabatan, tanpa sekretaris_rw)
- ✅ **Migration down aman** — migration 029 down sudah benar (DROP TABLE), tidak perlu migration tambahan

**Kekurangan:**
- ⚠️ Hardcoded permission map di Go code — perlu update manual saat ada perubahan permission
- ⚠️ Tidak ada audit trail permission RW di DB (tetapi `user_rw_membership_history` sudah ada untuk jabatan)

**Keputusan:** **DESAIN B DIPILIH**

### 1.3 Matriks Permission Final (VERIFIED)

Permission menggunakan domain `finance` existing:

| Jabatan | `finance.read` | `finance.create` | `finance.update` | `finance.approve` | Transfer |
|---|---|---|---|---|---|
| `ketua` (RT) | ✅ | ✅ | ✅ | ✅ | ✅ (via RT handler) |
| `wakil_ketua` (RT) | ✅ | ✅ | ✅ | ✅ | ✅ (via RT handler) |
| `bendahara` (RT) | ✅ | ✅ | ✅ | ✅ | ✅ (via RT handler) |
| **`ketua` (RW)** | ✅ | ✅ | ✅* | ✅ | ❌ |
| **`wakil_ketua` (RW)** | ✅ | ✅ | ❌ | ✅ | ❌ |
| **`bendahara` (RW)** | ✅ | ✅ | ✅* | ✅ | ✅ (via RW handler) |

> *`finance.update` untuk `ketua_rw` dan `bendahara_rw` memberi hak untuk membuat/mengupdate transaksi keuangan. Namun **transfer antar-lokasi** dibatasi tambahan di handler level oleh pemeriksaan role, bukan hanya permission code.

### 1.4 Isolasi Transfer — Detail Keamanan (VERIFIED)

**Fakta dari codebase:**

1. **`fund_location/handler.go` line 117-126** — `checkBendaharaAccess()`:
```go
func (h *Handler) checkBendaharaAccess(ac *auth.AuthContext) error {
    if ac.SystemRole == auth.SystemRoleSuperAdmin {
        return nil
    }
    if ac.TenantRole != auth.RoleBendahara && ac.Jabatan != auth.JabatanBendahara {
        return errors.New("only bendahara can execute or reverse fund transfers")
    }
    return nil
}
```
Handler RT fund location **sudah membatasi transfer hanya ke bendahara**.

2. **Handler RW baru juga perlu implementasi `checkBendaharaAccess` yang sama** — sehingga transfer antar-lokasi di scope RW hanya boleh dieksekusi oleh `bendahara_rw`, bukan `ketua_rw` atau `wakil_ketua_rw`.

3. **`finance.update` ≠ transfer otomatis**: `finance.update` adalah permission granular untuk mengubah transaksi. Transfer adalah operasi spesifik yang dibatasi oleh middleware handler-level `checkBendaharaAccess`. Jadi:
- `ketua_rw` punya `finance.update` → bisa update transaksi keuangan RW, tapi **TIDAK BISA transfer** karena `checkBendaharaAccess` menolak
- `bendahara_rw` punya `finance.update` → bisa update transaksi DAN transfer (karena `ac.Jabatan = 'bendahara'` dan `ac.RWMemberships` mengandung jabatan RW bendahara)

### 1.5 Permission Resolver RW — Spesifikasi Teknis

**File:** `backend/internal/auth/permission.go` — tambah function baru, tidak ubah existing.

```go
// rwPermissionMap defines RW position permissions separately from RT
// position_permissions. This ensures RW permissions never "bleed" into RT
// and vice versa.
var rwPermissionMap = map[string]map[string]bool{
    "ketua": {
        "finance.read":     true,
        "finance.create":   true,
        "finance.update":   true, // bisa update transaksi, TIDAK bisa transfer (ditangani handler)
        "finance.approve":  true,
    },
    "wakil_ketua": {
        "finance.read":     true,
        "finance.create":   true,
        // TIDAK punya finance.update — tidak bisa transfer
        "finance.approve":  true,
    },
    "bendahara": {
        "finance.read":     true,
        "finance.create":   true,
        "finance.update":   true, // bisa update transaksi DAN transfer
        "finance.approve":  true,
    },
}

// checkRWPositionPermission checks whether the given RW position has the
// specified permission code. This is separate from RT position_permissions
// to ensure RW and RT permissions are completely isolated.
func checkRWPositionPermission(position string, permissionCode string) (bool, error) {
    perms, ok := rwPermissionMap[position]
    if !ok {
        return false, nil // unknown position → no permissions
    }
    return perms[permissionCode], nil
}
```

**Integrasi ke `HasPermission()`:**

```go
func (pr *PermissionResolver) HasPermission(ctx context.Context, tx *sql.Tx, ac *AuthContext, permission string) (bool, error) {
    // 1. SUPER Admin: validasi scope (lihat Section 4)
    if ac.SystemRole == SystemRoleSuperAdmin {
        // ... validasi SCOPED_RT_IDS/SCOPED_RW_IDS
        return true, nil
    }

    // 2. Cek RW permission jika user punya RW membership
    if len(ac.RWMemberships) > 0 {
        for _, rm := range ac.RWMemberships {
            // rm.Jabatan adalah nilai dari user_rw_memberships.jabatan ('ketua', 'wakil_ketua', 'bendahara')
            hasRW, err := checkRWPositionPermission(rm.Jabatan, permission)
            if hasRW {
                return true, err
            }
        }
    }

    // 3. Fallback: cek RT permission via position_permissions
    return checkPositionPermission(ac.RTDB, ac.Jabatan, permission)
}
```

### 1.6 Super Admin — Tidak Mengabaikan Allowlist Scope (VERIFIED)

**Fakta dari codebase:** Saat ini `HasPermission()` untuk SUPER Admin selalu `return true, nil` tanpa validasi scope.

**Revisi:** Tambah validasi terhadap env var:

```go
// ScopingConfig loads and validates environment-scoped access control.
type ScopingConfig struct {
    RTIDs []string
    RWIDs []string
}

func LoadScoping() (ScopingConfig, error) {
    rtIDsRaw := strings.TrimSpace(os.Getenv("SCOPED_RT_IDS"))
    rwIDsRaw := strings.TrimSpace(os.Getenv("SCOPED_RW_IDS"))

    var rtIDs, rwIDs []string

    if rtIDsRaw != "" {
        for _, raw := range strings.Split(rtIDsRaw, ",") {
            id := strings.TrimSpace(raw)
            if id == "" {
                continue
            }
            if !isValidUUID(id) {
                return ScopingConfig{}, fmt.Errorf("SCOPED_RT_IDS contains invalid UUID: %q", id)
            }
            rtIDs = append(rtIDs, id)
        }
    }

    if rwIDsRaw != "" {
        for _, raw := range strings.Split(rwIDsRaw, ",") {
            id := strings.TrimSpace(raw)
            if id == "" {
                continue
            }
            if !isValidUUID(id) {
                return ScopingConfig{}, fmt.Errorf("SCOPED_RW_IDS contains invalid UUID: %q", id)
            }
            rwIDs = append(rwIDs, id)
        }
    }

    return ScopingConfig{RTIDs: rtIDs, RWIDs: rwIDs}, nil
}
```

**Validasi startup:** Di `main.go` saat aplikasi start, panggil `LoadScoping()`. Jika error (invalid UUID), log error dan exit. Jika kosong (env var tidak diset), gunakan default: RT 002.

**Validasi per-request:** Di `HasPermission()` untuk SUPER Admin:

```go
if ac.SystemRole == SystemRoleSuperAdmin {
    if ac.ScopeID != "" && ac.ScopeType != "" {
        scoping, err := LoadScoping()
        if err != nil {
            return false, err // invalid config
        }
        switch ac.ScopeType {
        case "rt":
            if !uuidSliceContains(scoping.RTIDs, ac.ScopeID) {
                return false, nil // scope tidak diizinkan
            }
        case "rw":
            if !uuidSliceContains(scoping.RWIDs, ac.ScopeID) {
                return false, nil // scope tidak diizinkan
            }
        }
    } else {
        return false, nil // tanpa ScopeID → blocked
    }
    return true, nil
}
```

---

## 2. Rencana Perubahan Schema dan Urutan Migration

### 2.1 Migration yang Diperlukan (Tanpa Membuat Migration)

| No | Migration | Jenis | Deskripsi |
|---|---|---|---|
| — | Existing 029 | Sudah ada | `user_rw_memberships` dengan `jabatan` CHECK `('ketua','wakil_ketua','sekretaris','bendahara')` |
| — | Existing 031 | Sudah ada | 12 permission_types canonical + 43 position_permissions grants |
| 1 | 042_rw_rw_position_schema | **Baru** | Hanya memperluas CHECK constraint `user_rw_memberships.jabatan` dari 4 nilai ke 3 nilai (menghapus `'sekretaris'`) untuk memastikan konsistensi dengan keputusan produk |
| 2 | 043_rw_rw_memberships_jabatan_sync | **Baru** | (Opsional) Migrasi data untuk memastikan existing RW memberships punya jabatan yang valid |

**Catatan tentang migration 042:**

Migration 042 **tidak** mengubah `position_permissions` sama sekali (karena Desain B tidak menggunakannya untuk RW). Migration ini hanya perlu memastikan tidak ada konflik — karena `user_rw_memberships.jabatan` CHECK constraint sudah mendukung 4 nilai dan hanya 3 yang diizinkan secara operasional, **tidak ada migration yang diperlukan** untuk constraint.

Jika ada kekhawatiran tentang data existing dengan `jabatan = 'sekretaris'`, migration bisa:
```sql
UPDATE user_rw_memberships 
SET jabatan = NULL 
WHERE jabatan = 'sekretaris' AND is_active = true;
```

### 2.2 Apa yang Tidak Diperlukan (Dihapus dari Rencana Sebelumnya)

- ~~Migration 042 untuk DROP/ADD CHECK pada `position_permissions.position`~~ — tidak diperlukan (Desain B)
- ~~GRANT permission RW ke `position_permissions`~~ — tidak diperlukan (resolver RW terpisah)
- ~~Migration permission type baru (`rw.finance.read` dll)~~ — tidak diperlukan (menggunakan `finance.*` existing)

---

## 3. Strategi Membership, JWT, dan Revokasi (VERIFIED)

### 3.1 Bentuk RWContext yang Mendukung Multi-RW

```go
// backend/internal/auth/models.go — TokenClaims
type TokenClaims struct {
    UserID    string `json:"sub"`
    MID       string `json:"mid"`       // RT membership ID
    RTID      string `json:"rt_id"`      // RT ID
    Role      string `json:"role"`       // RT role
    Jabatan   string `json:"jabatan"`    // RT jabatan
    SysRole   string `json:"sys_role"`
    RWContext string `json:"rw_context"` // JSON string: [{ "rwId":"x", "jabatan":"ketua" }, ...]
}

// backend/internal/auth/models.go — AuthContext tambahan
type AuthContext struct {
    // ... field existing ...
    RWMemberships []RWMembership `json:"-"` // deserialized dari JWT
}

type RWMembership struct {
    RWID    string `json:"rwId"`
    Jabatan string `json:"jabatan"` // nilai dari user_rw_memberships.jabatan ('ketua', 'wakil_ketua', 'bendahara')
}
```

**Mengapa `jabatan` bukan `role`:** `user_rw_memberships.jabatan` adalah field yang menyimpan posisi operasional RW. `user_rw_memberships.role` adalah RT role (`pengurus`, `warga`). Menggunakan `jabatan` lebih akurat dan konsisten dengan skema.

### 3.2 Sumber Kebenaran Membership di Database

**Tabel:** `user_rw_memberships`
- Primary key: `id`
- Unique constraint: `(user_id, rw_id) WHERE is_active = true`
- Foreign key `rw_id → rws(id) ON DELETE CASCADE`
- Foreign key `rt_id → rts(id) ON DELETE SET NULL`
- `is_active` boolean default `true`
- `jabatan` text CHECK `('ketua', 'wakil_ketua', 'sekretaris', 'bendahara')`

**Tabel:** `user_rw_membership_history`
- Audit trail untuk setiap perubahan jabatan RW
- `action IN ('assign', 'change', 'revoke')`

### 3.3 Validasi Membership Terkini pada Request RW

**Problem:** Token JWT bisa berisi `RWContext` dengan jabatan yang sudah dicabut (token 15 menit valid).

**Solusi:** Di middleware `RequireAuth()`, setelah deserialisasi `RWContext` dari JWT, lakukan validasi ke database:

```go
// Di RequireAuth(), setelah deserialize RWContext:
if claims.RWContext != "" && claims.RWContext != "[]" {
    var memberships []RWMembership
    json.Unmarshal([]byte(claims.RWContext), &memberships)

    // Validasi setiap membership masih aktif di DB
    if len(memberships) > 0 {
        rwIDs := make([]string, 0, len(memberships))
        for _, rm := range memberships {
            rwIDs = append(rwIDs, rm.RWID)
        }
        // Query DB untuk memverifikasi
        var activeRWs []string
        rows, err := AuthDBPool.Raw().QueryContext(
            validateCtx,
            `SELECT DISTINCT rw_id FROM user_rw_memberships 
             WHERE user_id = $1 AND rw_id = ANY($2::uuid[]) 
             AND jabatan IS NOT NULL AND is_active = true`,
            claims.UserID, pq.Array(rwIDs),
        )
        for rows.Next() {
            var rwID string
            rows.Scan(&rwID)
            activeRWs = append(activeRWs, rwID)
        }

        // Filter memberships: hanya yang masih aktif
        activeSet := make(map[string]bool, len(activeRWs))
        for _, rwID := range activeRWs {
            activeSet[rwID] = true
        }
        var filtered []RWMembership
        for _, rm := range memberships {
            if activeSet[rm.RWID] {
                filtered = append(filtered, rm)
            }
        }
        ac.RWMemberships = filtered
    }
}
```

**Kriteria:** Jika semua RW membership sudah tidak aktif → `RWMemberships = nil` → semua endpoint RW → 403.

### 3.4 Perilaku Ketika Jabatan RW Dicabut (VERIFIED)

**Skenario:** User adalah `ketua_rw` di RW 001. Jabatan dicabut via admin UI (`RemoveRWJabatan`).

**Yang terjadi:**
1. `jabatan` di `user_rw_memberships` di-set `NULL`
2. `user_rw_membership_history` mencatat action `'revoke'`
3. Token JWT yang sudah terbit **masih berisi** `RWContext` dengan jabatan lama

**Mitigasi:** Validasi DB di middleware (Section 3.3) akan:
- Query `SELECT rw_id FROM user_rw_memberships WHERE user_id = ? AND is_active = true AND jabatan IS NOT NULL`
- Karena jabatan sekarang `NULL` → tidak ada match → `RWMemberships = nil`
- Request ke endpoint RW → 403

### 3.5 Perilaku Refresh Token Setelah Membership Dicabut (VERIFIED)

**Skenario:** User `ketua_rw` di RW 001. Jabatan dicabut. User coba refresh token lama.

**Yang terjadi:**
1. `Refresh()` di `service.go` line 195-289 memanggil `ValidateIdentity()` → validasi user ada dan aktif
2. Query `user_rw_memberships` untuk RW context → tidak ada entry aktif → `RWContext = "[]"`
3. Token baru **tidak punya** RW context → endpoint RW → 403
4. Refresh token lama di-revoke → tidak bisa dipakai lagi

**Kesimpulan:** Refresh token otomatis menghilangkan RW context setelah pencabutan jabatan. Token yang tidak di-refresh tetap dicek di middleware (Section 3.3).

### 3.6 Kompatibilitas Token Lama (VERIFIED)

- Token lama (tanpa `RWContext`) → field default `""` (Go zero-value untuk string)
- Middleware: `if rwContext == "" { RWMemberships = nil }`
- Endpoint RW: jika `RWMemberships == nil` atau scope tidak ada → 403
- **Tidak ada breaking change** — client yang tidak expect field baru tidak terpengaruh

### 3.7 Respons Aman Ketika Scope Tidak Valid

| Kondisi | Respons |
|---|---|
| `X-RT-ID` bukan UUID valid | 400 bad_request |
| `X-RT-ID` UUID tapi RT tidak ada di allowlist | 403 forbidden |
| `X-RT-ID` UUID tapi RT tidak aktif | 403 forbidden |
| User bukan SUPER Admin tapi kirim `X-RT-ID` | Diabaikan (menggunakan RT dari JWT) |
| `rt_id` query param bukan UUID valid | 400 bad_request |
| `rt_id` query param UUID tapi tidak di allowlist | 403 forbidden |

---

## 4. Strategi Super Admin Scope (VERIFIED)

### 4.1 Environment Variables

| Variable | Format | Default |
|---|---|---|
| `SCOPED_RT_IDS` | Comma-separated UUIDs, contoh: `a95c1cb5-...,uuid2` | `a95c1cb5-2f67-4ea5-9197-bfec537b6d7b` (RT 002) |
| `SCOPED_RW_IDS` | Comma-separated UUIDs, contoh: `uuid1,uuid2` | Empty → SA tidak bisa akses RW secara otomatis |

### 4.2 Validasi Startup

**Di `main.go` saat startup:**

```go
func validateScoping() {
    scoping, err := LoadScoping()
    if err != nil {
        log.Fatalf("Invalid SCOPED_RT_IDS or SCOPED_RW_IDS: %v", err)
    }
    if len(scoping.RTIDs) == 0 {
        log.Println("WARNING: SCOPED_RT_IDS is empty — Super Admin cannot access any RT. Set at least one RT ID.")
    }
    // Deduplication
    uniqueRTIDs := make(map[string]bool)
    for _, id := range scoping.RTIDs {
        if uniqueRTIDs[id] {
            log.Printf("WARNING: Duplicate RT ID in SCOPED_RT_IDS: %s", id)
        }
        uniqueRTIDs[id] = true
    }
    // Set sebagai global variable untuk middleware
    globalScoping = scoping
}
```

### 4.3 Integrasi dengan `RequireAllowedRT`

**File:** `backend/internal/auth/middleware.go` — update `RequireAllowedRT`:

```go
func RequireAllowedRT(pool *database.Pool) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ac, ok := r.Context().Value(AuthContextKey).(*AuthContext)
            if !ok || ac == nil {
                httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
                return
            }

            // System-only super admin with no RT context passes through
            if ac.SystemRole == SystemRoleSuperAdmin && ac.RTID == "" {
                next.ServeHTTP(w, r)
                return
            }

            // Validate RT is in allowed list
            if ac.RTID != "" {
                if !uuidSliceContains(globalScoping.RTIDs, ac.RTID) {
                    httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "rt is outside allowed scope")
                    return
                }
            }

            next.ServeHTTP(w, r)
        })
    }
}
```

### 4.4 Penanganan `X-RT-ID` dan `rt_id` Query Parameter

**Di `RequireAuth()` middleware** — SUPER Admin override:

```go
effectiveRTID := claims.RTID
if SystemRole(claims.SysRole) == SystemRoleSuperAdmin && effectiveRTID == "" {
    // Try header first, then query param
    reqRT := strings.TrimSpace(r.Header.Get("X-RT-ID"))
    if reqRT == "" {
        reqRT = strings.TrimSpace(r.URL.Query().Get("rt_id"))
    }

    // Validate UUID format
    if reqRT != "" {
        if !isValidUUID(reqRT) {
            httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "invalid rt_id format")
            return
        }
        // Validate against allowlist AND active status
        if AuthDBPool != nil {
            var validID string
            err := AuthDBPool.Raw().QueryRowContext(validateCtx,
                `SELECT id FROM rts WHERE id = $1 AND is_active = true AND id = ANY($2::uuid[])`,
                reqRT, pq.Array(globalScoping.RTIDs)).Scan(&validID)
            if err == nil {
                effectiveRTID = validID
            } else {
                // RT not in allowlist or not active
                httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "rt is outside allowed scope")
                return
            }
        }
    }
}
```

---

## 5. Model Agregasi KAS dan Lokasi Dana RW (VERIFIED)

### 5.1 Verifikasi Implementasi Laporan KAS RT (VERIFIED)

**Fakta dari codebase:**

1. **`transactions` table** (`migrations/009`):
   - `type IN ('income', 'expense')` — **selalu positif**, tidak negatif
   - `status IN ('draft', 'posted', 'cancelled')`
   - `reverses_transaction_id` reference ke transaksi asli saat reversal

2. **`period_opening_balances` table** (`migrations/024`):
   - `rt_id UUID NOT NULL`
   - `period text NOT NULL` (format YYYY-MM)
   - `opening_balance NUMERIC(15,2) NOT NULL DEFAULT 0.00`
   - `previous_balance NUMERIC(15,2) NULL` — saldo terakhir periode sebelumnya
   - `source IN ('automatic', 'manual')`
   - `CONSTRAINT uq_period_opening_balances_rt_period UNIQUE (rt_id, period)`

3. **Balanced ledger rules** (`AGENTS.md` Section 4):
   - `Balance = Total Posted Income - Total Posted Expense`
   - Reversal transaction mengurangi efektif total
   - Saldo akhir periode sebelumnya menjadi saldo awal periode berikutnya (otomatis via `previous_balance`)

### 5.2 Query Agregasi KAS RW (VERIFIED)

**Saldo awal RW** = SUM saldo awal semua RT di bawah RW:
```sql
SELECT COALESCE(SUM(ob.opening_balance), 0) as total_opening_balance
FROM period_opening_balances ob
WHERE ob.rt_id IN (
    SELECT id FROM rts WHERE rw_id = $1 AND is_active = true
) AND ob.period = $2
```

**Total transaksi RW** (semua status):
```sql
SELECT
    COALESCE(SUM(amount) FILTER (WHERE type = 'income' AND status = 'posted'), 0) as total_income_posted,
    COALESCE(SUM(amount) FILTER (WHERE type = 'expense' AND status = 'posted'), 0) as total_expense_posted,
    COALESCE(SUM(amount) FILTER (WHERE type = 'income' AND status = 'cancelled'), 0) as total_income_cancelled,
    COALESCE(SUM(amount) FILTER (WHERE type = 'expense' AND status = 'cancelled'), 0) as total_expense_cancelled
FROM transactions
WHERE rt_id IN (
    SELECT id FROM rts WHERE rw_id = $1 AND is_active = true
)
```

**Saldo efektif RW:**
```sql
-- Saldo awal + (income_posted - expense_posted) + (income_cancelled + expense_cancelled)
-- Catatan: income_cancelled DAN expense_cancelled sama-sama mengurangi saldo efektif
-- (income_cancelled = reversal income = mengurangi saldo; expense_cancelled = reversal expense = menambah saldo)
-- Tapi karena semua amount positif di ledger:
-- - income_posted menambah saldo, expense_posted mengurangi saldo
-- - income_cancelled mengurangi saldo (reversal dari income yang sebelumnya menambah)
-- - expense_cancelled menambah saldo (reversal dari expense yang sebelumnya mengurangi)
-- Jadi: saldo = opening + income_posted - expense_posted - income_cancelled + expense_cancelled

(effective_balance) = 
    opening_balance 
    + total_income_posted 
    - total_expense_posted 
    - total_income_cancelled 
    + total_expense_cancelled
```

**Penjelasan reversal semantik:**
- Transaksi `income` dengan status `posted` → saldo bertambah
- Transaksi `expense` dengan status `posted` → saldo berkurang
- Reversal transaksi → status `cancelled`, amount tetap positif
- Reversal income → saldo berkurang (karena reversal dari income yang sebelumnya menambah)
- Reversal expense → saldo bertambah (karena reversal dari expense yang sebelumnya mengurangi)

**Filter tanggal:** Jika ada parameter `period` (YYYY-MM), tambahkan filter:
```sql
WHERE t.occurred_at >= DATE_TRUNC('month', $2::text::date)
  AND t.occurred_at < DATE_TRUNC('month', $2::text::date) + INTERVAL '1 month'
```

### 5.3 Query Lokasi Dana RW (VERIFIED)

**Fakta:** `fund_locations` dan `fund_transfers` hanya ada per-RT, tidak ada `rw_id`.

**Query agregasi Lokasi Dana RW:**
```sql
SELECT 
    fl.id,
    fl.name,
    fl.category,
    fl.bank_name,
    fl.account_number,
    fl.account_holder,
    fl.is_active,
    fl.created_at,
    r.name as rt_name,
    r.rt as rt_code
FROM fund_locations fl
JOIN rts r ON r.id = fl.rt_id
WHERE r.rw_id = $1 AND r.is_active = true AND fl.is_active = true
ORDER BY r.rt, r.name
```

**Endpoint RW Lokasi Dana:** READ-ONLY — tidak ada POST/PUT/DELETE di scope RW. Mutasi tetap di endpoint RT `/api/v1/fund-locations` dengan middleware `checkWriteAccess` + `checkBendaharaAccess`.

---

## 6. Kontrak API dan Dependensi PR (VERIFIED)

### 6.1 Tahap API Contract

**Sebelum PR 5 (Web) dan PR 6 (Android) dimulai, definisikan kontrak API:**

```
docs/api-rw-operasional.md — OpenAPI 3.0 spec untuk:
- GET /api/v1/rw/{rwId}/beranda
- GET /api/v1/rw/{rwId}/kas
- GET /api/v1/rw/{rwId}/lokasi-dana
- GET /api/v1/rw/{rwId}/lokasi-dana/{locationId}/saldo (opsional)
```

**Response schemas:**

```json
// GET /api/v1/rw/{rwId}/beranda
{
    "data": {
        "rw_id": "uuid",
        "rw_name": "RW 016",
        "total_rt": 3,
        "opening_balance": 15000000,
        "total_income_posted": 50000000,
        "total_expense_posted": 30000000,
        "total_income_cancelled": 2000000,
        "total_expense_cancelled": 500000,
        "effective_balance": 35500000,
        "transactions": [
            {
                "id": "uuid",
                "rt_id": "uuid",
                "rt_name": "RT 001",
                "type": "income" | "expense",
                "amount": "1500000",
                "category": "...",
                "description": "...",
                "status": "posted" | "cancelled",
                "occurred_at": "2026-10-01T00:00:00Z",
                "reverses_transaction_id": "uuid | null"
            }
        ],
        "pagination": {
            "page": 1,
            "per_page": 20,
            "total": 100,
            "total_pages": 5
        }
    }
}

// GET /api/v1/rw/{rwId}/lokasi-dana
{
    "data": [
        {
            "id": "uuid",
            "name": "Kas Utama",
            "category": "rekening | brankas | coh",
            "bank_name": "BCA",
            "account_number": "1234567890",
            "account_holder": "Bendahara RW 016",
            "is_active": true,
            "rt_id": "uuid",
            "rt_name": "RT 001",
            "rt_code": "001"
        }
    ]
}
```

**Error responses:**
```json
// 401 Unauthorized
{ "error": { "code": "unauthorized", "message": "missing authentication" } }

// 403 Forbidden
{ "error": { "code": "forbidden", "message": "insufficient permissions" } }

// 404 Not Found
{ "error": { "code": "not_found", "message": "rw not found" } }
```

### 6.2 Dependensi PR

| PR | Deskripsi | Dependensi | Estimasi |
|---|---|---|---|
| **PR 1** | Migration 042: opsional cleanup `'sekretaris'` di `user_rw_memberships.jabatan` | — | 0.5 hari |
| **PR 2** | JWT Extension: tambah `RWContext` di TokenClaims | PR 1 | 2-3 hari |
| **PR 3** | Permission Resolver: tambah `checkRWPositionPermission()` + validasi Super Admin scope | PR 2 | 2 hari |
| **PR 3.5** | API Contract: definisi OpenAPI spec untuk endpoint RW | PR 3 | 0.5 hari |
| **PR 4** | Finance/Fund Location RW Scoping (backend): tambah handler/service/repository | PR 3, PR 3.5 | 3-4 hari |
| **PR 5** | Web Frontend: menu, route, halaman RW | PR 4 | 2-3 hari |
| **PR 6** | Android: horizontal sliding + menu RW | PR 4, PR 5 | 2-3 hari |
| **PR 7** | Integration & Regression Tests | Semua PR sebelumnya | 2-3 hari |

**Catatan:** PR 5 dan PR 6 bisa dikerjakan paralel setelah PR 3.5 (API contract) stabil, karena keduanya mengonsumsi API yang sama dari PR 4.

---

## 7. Test Plan Final (VERIFIED)

### 7.1 Test Kasus Eksplisit

| No | Test Kasus | Layer | Request | Hasil Diharapkan |
|---|---|---|---|---|
| T1 | Perangkat RT buka KAS RW | Backend API | GET `/rw/{rwId}/kas` | **200**, data agregat RT di bawah RW |
| T2 | Perangkat RT coba ubah transaksi RW | Backend API | POST `/rw/{rwId}/transactions` | **403**, insufficient permissions |
| T3 | Perangkat RT buka Lokasi Dana RW | Backend API | GET `/rw/{rwId}/lokasi-dana` | **403** atau **200 read-only** (sesuai kontrak) |
| T4 | Perangkat RT coba akses RT lain | Backend API | GET `/rt/{rtOtherId}/...` | **403** |
| T5 | Ketua RW buka Beranda RW | Backend API | GET `/rw/{rwId}/beranda` | **200**, read-only akses |
| T6 | Wakil Ketua RW buka KAS RW | Backend API | GET `/rw/{rwId}/kas` | **200**, read akses |
| T7 | Bendahara RW kelola Lokasi Dana RW | Backend API | POST/PUT `/rw/{rwId}/lokasi-dana` | **403** (lokasi dana RW hanya read-only, mutasi via RT endpoint) |
| T8 | Perangkat RW buka KAS RT | Backend API | GET `/rt/{rtId}/kas` | **200**, read access |
| T9 | Perangkat RW coba ubah transaksi RT | Backend API | POST `/rt/{rtId}/transactions` | **403** |
| T10 | Role RW tidak diizinkan buka Lokasi Dana RW | Backend API | GET `/rw/{rwId}/lokasi-dana` | **403** atau **200 read-only** |
| T11 | Pengguna jabatan ganda login | Backend API | POST `/login` | Token berisi RT context + RW contexts |
| T12 | Pengguna jabatan ganda buka KAS RT | Backend API | GET `/rt/{rtId}/kas` | **200**, lingkup RT |
| T13 | Pengguna jabatan ganda buka KAS RW | Backend API | GET `/rw/{rwId}/kas` | **200**, lingkup RW |
| T14 | Super Admin akses RT 002 | Backend API | GET `/rt/{rt002}/kas` | **200** (RT 002 di SCOPED_RT_IDS) |
| T15 | Super Admin akses wilayah di luar lingkup | Backend API | GET `/rt/{unknown}/kas` | **403** |
| T16 | Token lama tanpa field RW akses endpoint RW | Backend API | GET `/rw/{rwId}/beranda` | **403** |
| T17 | Multi-jabatan POST transaksi RW sebagai perangkat RT | Backend API | POST `/rw/{rwId}/transactions` | **403** (perangkat RT tidak punya finance.create) |
| T18 | Multi-jabatan POST transaksi RW sebagai bendahara_rw | Backend API | POST `/rw/{rwId}/transactions` | **201** (jika bendahara_rw punya finance.create) |
| T19 | Token lama (sebelum field RW) tidak bisa akses RW | Backend API | Semua endpoint RW | **403** |
| T20 | Pencabutan jabatan RW → token lama → akses RW | Backend API | GET `/rw/{rwId}/beranda` | **403** (validasi DB menolak) |
| **T21** | **Ketua RW mencoba transfer** | Backend API | POST `/api/v1/fund-locations/{locId}/transfer` | **403** — `checkBendaharaAccess` menolak, jabatan bukan bendahara |
| **T22** | **Wakil Ketua RW mencoba transfer** | Backend API | POST `/api/v1/fund-locations/{locId}/transfer` | **403** — `checkBendaharaAccess` menolak, jabatan bukan bendahara |
| **T23** | **Bendahara RW mencoba transfer pada scope RW** | Backend API | POST `/api/v1/fund-locations/{locId}/transfer` | **201** — jika bendahara_rw punya finance.update dan role match |
| **T24** | **Bendahara RT mencoba menggunakan kewenangan RW yang tidak dimilikinya** | Backend API | POST `/rw/{rwId}/transactions` | **403** — tidak punya RW membership |
| **T25** | **Permission jabatan RT tidak otomatis memberi permission RW** | Backend API | GET `/rw/{rwId}/beranda` untuk ketua RT tanpa RW membership | **403** |
| **T26** | **Permission RW tidak memperluas hak akses RT** | Backend API | GET `/rt/{rtId}/...` untuk ketua RW tanpa RT membership | **403** (atau tidak bisa akses karena RTID dari JWT tidak valid) |
| **T27** | **Membership dicabut saat access token masih valid** | Backend API | GET `/rw/{rwId}/beranda` | **403** — validasi DB di middleware menolak |
| **T28** | **Refresh token setelah pencabutan membership** | Backend API | POST `/refresh` | Token baru tanpa RW context → endpoint RW → 403 |
| **T29** | **X-RT-ID atau rt_id memilih scope di luar allowlist** | Backend API | GET `/rt/{unknownId}/kas` dengan header `X-RT-ID: unknown-uuid` | **403** |
| **T30** | **Konfigurasi allowlist kosong atau invalid** | Backend API | Semua request | Startup: log warning/error. Runtime: SA tidak bisa akses RT/RW manapun |
| **T31** | **Saldo awal direkonsiliasi dengan agregat KAS RW** | Backend API | GET `/rw/{rwId}/beranda?period=2026-10` | Saldo = opening + income_posted - expense_posted - income_cancelled + expense_cancelled |
| **T32** | **Reversal transaction mempengaruhi saldo agregat** | Backend API | GET `/rw/{rwId}/beranda` after reversal | Saldo berkurang sesuai amount reversal |
| **T33** | **Transaksi cancelled tidak termasuk dalam posted totals** | Backend API | GET `/rw/{rwId}/beranda` | `total_income_posted` dan `total_expense_posted` tidak termasuk cancelled |
| **T34** | **Regresi finance RT** | Backend API | Semua endpoint finance RT existing | **200** semua test RT existing pass |
| **T35** | **Regresi fund location RT** | Backend API | Semua endpoint fund location RT existing | **200** semua test RT existing pass |

### 7.2 Test Negative Cases (VERIFIED)

| No | Test Kasus | Deskripsi | Layer |
|---|---|---|---|
| N1 | SCOPED_RT_IDS berisi UUID tidak valid | Startup validation reject | Backend startup |
| N2 | SCOPED_RT_IDS kosong | SA tidak bisa akses RT manapun | Backend middleware |
| N3 | SCOPED_RT_IDS ada duplikasi | Warning log, tidak crash | Backend startup |
| N4 | User tanpa jabatan RW akses endpoint RW | Token tanpa RWContext → 403 | Backend API |
| N5 | User multi-RW akses RW yang tidak di-token | Salah satu RW valid → 200, scope lain → 403 | Backend API |
| N6 | X-RT-ID manipulasi untuk akses RT lain | Middleware validasi against allowlist | Backend middleware |
| N7 | RT di luar RW scope (RT tidak punya rw_id) | User akses `/rw/{rwId}/...` tapi RT tidak punya rw_id | Backend API |
| N8 | RW yang tidak ada | GET `/rw/{invalidUuid}/beranda` | Backend API → 404 |

---

## 8. Daftar Keputusan yang Masih Perlu Konfirmasi

| # | Keputusan | Status | Catatan |
|---|---|---|---|
| 1 | Apakah `/rw/{rwId}/lokasi-dana` read-only atau support create/read | **DESIGN DECISION: read-only** | Mutasi lokasi dana tetap di RT endpoint. Lokasi Dana RW = agregasi read-only |
| 2 | Apakah `SCOPED_RW_IDS` harus diset atau opsional | **BLOCKED** — butuh keputusan produk | Jika kosong, SA tidak bisa akses RW manapun |
| 3 | Apakah perangkat RT bisa akses Lokasi Dana RW (baca) | **DESIGN DECISION: 200 read-only** | Sesuai dokumen acuan: perangkat RT bisa baca Beranda RW dan KAS RW. Lokasi Dana RW perlu konfirmasi produk |
| 4 | Apakah endpoint RW `/rw/{rwId}/beranda` perlu filter tanggal (period) | **DESIGN DECISION: ya** | Konsisten dengan KAS RT yang punya period filtering |
| 5 | Apakah `rw_permission_map` perlu audit trail di DB | **BLOCKED** — butuh keputusan | Saat ini hardcoded di Go code. Jika perlu audit trail, perlu tabel baru `rw_position_permissions` |

---

## 9. Ringkasan Keamanan

| Aspek | Status | Detail |
|---|---|---|
| **Isolasi RT/RW** | ✅ VERIFIED | Permission resolver terpisah: RT pakai `position_permissions`, RW pakai `rwPermissionMap` |
| **Transfer authorization** | ✅ VERIFIED | `checkBendaharaAccess()` membatasi transfer ke bendahara saja |
| **Super Admin scope** | ✅ VERIFIED | Env var + UUID validation + DB check on every request |
| **Token revocation** | ✅ VERIFIED | Middleware validasi membership terkini di DB pada setiap request RW |
| **Token kompatibilitas** | ✅ VERIFIED | Token lama tanpa RWContext → 403 di endpoint RW |
| **Input validation** | ✅ VERIFIED | UUID format check, scope allowlist check, RT active check |
| **Financial immutability** | ✅ VERIFIED | Tidak ada mutasi di scope RW — semua mutasi tetap di RT endpoint |

---

## 10. Status Akhir

**TIDAK SIAP IMPLEMENTASI sebelum:**
1. ✅ Konflik jabatan diselesaikan (Desain B dipilih)
2. ✅ Isolasi permission RT/RW terdefinisi (resolver terpisah)
3. ✅ Revokasi membership ditangani (validasi DB di middleware)
4. ⚠️ Super Admin scope membutuhkan keputusan produk untuk `SCOPED_RW_IDS`
5. ✅ Semantik laporan keuangan terverifikasi (opening balance + posted/cancelled)

**Tanda tangan:** Setelah keputusan produk untuk item #2 (SCOPED_RW_IDS) dan #3 (lokasi dana RW untuk perangkat RT) dikonfirmasi, rencana ini siap untuk mulai PR 1.

---

*Akhiri rencana implementasi revisi.*
