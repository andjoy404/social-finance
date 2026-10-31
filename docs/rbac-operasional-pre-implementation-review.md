# REVIEW DESAIN PRA-IMPLEMENTASI — RBAC Operasional RW

**Tanggal:** 2026-10-29  
**Acuan:** `docs/rbac-operasional-rw.md` (dokumen acuan produk)  
**Target review:** `docs/rbac-operasional-final-audit.md` (rencana implementasi)  
**Jenis:** Read-only — tidak mengubah file apapun  

---

## 1. Temuan Berdasarkan Tingkat Kritis

### 🔴 KRITIS (Harus diperbaiki sebelum PR 1)

#### 1.1 `position_permissions` CHECK constraint memblokir jabatan RW

**Masalah:** `backend/migrations/011_add_positions_permissions.up.sql` mendefinisikan CHECK constraint pada kolom `position`:

```sql
CHECK (position IN (
    'ketua', 'wakil_ketua', 'sekretaris', 'bendahara',
    'keamanan', 'sosial', 'kebersihan_pembangunan', 'anggota'
))
```

CHECK constraint ini hanya mengizinkan **jabatan RT**, bukan jabatan RW. Migration 042 di rencana akhir tidak bisa INSERT `'ketua_rw'`, `'wakil_ketua_rw'`, atau `'bendahara_rw'` karena akan gagal dengan error:

```
ERROR: new row for relation "position_permissions" violates check constraint "position_permissions_position_check"
```

**Dampak:** Seluruh permission resolver RW gagal. Permission 042 tidak akan berhasil.

**Perbaikan:** Migration baru (042a) harus menambahkan `'ketua_rw'`, `'wakil_ketua_rw'`, `'sekretaris_rw'`, dan `'bendahara_rw'` ke CHECK constraint sebelum INSERT:

```sql
ALTER TABLE position_permissions
    DROP CONSTRAINT position_permissions_position_check,
    ADD CONSTRAINT position_permissions_position_check CHECK (position IN (
        'ketua', 'wakil_ketua', 'sekretaris', 'bendahara',
        'keamanan', 'sosial', 'kebersihan_pembangunan', 'anggota',
        'ketua_rw', 'wakil_ketua_rw', 'sekretaris_rw', 'bendahara_rw'
    ));
```

> **Catatan:** Migration 011 hanya punya file `.up.sql` — tidak ada `.down.sql`. Migration rollback 011 tidak ada, sehingga DROP/ADD constraint di 042a juga tidak punya down migration untuk di-rollback. Pertimbangkan apakah perlu membuat migration down untuk 042a.

#### 1.2 Tidak ada permission type `fund.transfer`

**Masalah:** Rencana audit proposal permission `rw.fund.transfer` untuk Bendahara RW agar bisa transfer antar lokasi dana. Namun:

1. Tidak ada permission_type dengan domain `fund` atau operasi `transfer` di seluruh migrasi.
2. `backend/internal/devseed/seed.go` hanya mendefinisikan 12 permission types: `warga.*`, `finance.*`, `reports.*`, `rt.settings.*`.
3. `backend/migrations/031_restore_permission_catalog.up.sql` hanya insert 12 kode permission.

Jika rencana menggunakan `finance.transfer` — permission ini tidak ada dalam daftar 12 permission types. Domain `finance` hanya punya: `finance.read`, `finance.create`, `finance.update`, `finance.approve`.

**Analisis semantik permission existing:**

| Permission | Domain | Operation | Kemungkinan untuk fund transfer |
|---|---|---|---|
| `finance.update` | finance | update | ✅ Mungkin — transfer mengubah balance lokasi |
| `finance.create` | finance | create | ❌ Untuk transaksi income/expense |
| `finance.approve` | finance | approve | ❌ Untuk approve transaksi |

Untuk transfer antar-lokasi dana, permission yang paling tepat secara semantik adalah **`finance.update`** (karena transfer mengubah saldo dua lokasi dana). Namun `finance.update` untuk RT hanya diberikan ke `ketua` dan `bendahara` RT.

**Perbaikan:** Gunakan permission code yang sudah ada (`finance.update`) untuk transfer, atau definisikan permission type baru `fund.transfer` dengan domain `fund`.

Jika menggunakan `finance.update`:
- Ketua RW dan Bendahara RW dapat `finance.update` → boleh transfer
- Wakil Ketua RW **tidak** dapat `finance.update` → tidak boleh transfer

Jika definisikan `fund.transfer` baru:
- Hanya Bendahara RW yang mendapat `fund.transfer`
- Ketua RW dan Wakil Ketua RW tidak mendapat `fund.transfer`

Rencana audit saat ini tidak memilih salah satu pendekatan ini secara eksplisit, dan bahkan mengusulkan permission code `rw.fund.transfer` yang tidak sesuai dengan skema domain existing.

#### 1.3 Token RWID + RWRole tunggal tidak mendukung membership ganda RW

**Masalah:** Rencana audit mengusulkan:
```go
RWID   string
RWRole string
```

Tapi dokumen acuan Section 3.3:
> "Sistem harus membaca **seluruh** jabatan pengguna — baik di RT maupun di beberapa RW jika pengguna aktif di lebih dari satu RW. Gabungan hak akses dari seluruh jabatan harus diterapkan, tanpa kecuali."

`TokenClaims.RWID` dan `TokenClaims.RWRole` bertipe `string` (skalar tunggal). Jika user adalah ketua RW 001 **dan** bendahara RW 002, hanya satu pasangan `{RWID, RWRole}` yang bisa masuk ke token.

**Dampak:** User multi-RW tidak dapat mengakses salah satu scope, atau salah satu scope diabaikan. Ini melanggar ketentuan Section 3.3 yang eksplisit menyatakan "seluruh jabatan pengguna".

**Perbaikan:** Gunakan tipe array:
```go
type TokenClaims struct {
    RWIDs   []string  // semua RW ID yang aktif
    RWRoles []string  // role RW yang sesuai di indeks yang sama
}
```

Atau lebih baik — serialize sebagai JSON string untuk kompatibilitas JWT:
```go
type TokenClaims struct {
    RWContext string // JSON: [{"rwId": "x", "rwRole": "y"}, ...]
}
```

Middleware harus deserialize `RWContext` pada setiap request dan memvalidasi scope aktif terhadap daftar yang ada di token.

#### 1.4 Rencana PR tidak punya PR database untuk permission type baru

**Masalah:** Rencana PR 1 hanya mengusulkan migration 041 untuk permission types dan 042 untuk position_permissions. Tapi:

1. Migration 041 mengusulkan code `rw.finance.read`, `rw.fund.transfer` — kode yang tidak ada di skema domain existing (`warga`, `finance`, `reports`, `rt.settings`). Domain `rw` dan `fund` tidak terdaftar.
2. Jika menggunakan permission yang sudah ada (`finance.read`, `finance.create`, `finance.approve`, `finance.update`), maka migration 041 tidak perlu membuat permission types baru — cukup insert ke `position_permissions` saja.
3. Tidak ada migration untuk mengubah CHECK constraint `position_permissions.position` (temuan 1.1 di atas).

---

### 🟠 TINGGI (Harus diperbaiki sebelum implementasi dimulai)

#### 1.5 Super Admin `ScopeID != ""` bukan otorisasi

**Masalah di rencana PR 5:**
```go
if ac.SystemRole == SystemRoleSuperAdmin {
    if ac.ScopeID == "" {
        return false, nil
    }
    return true, nil
}
```

`ScopeID == ""` hanya memvalidasi bahwa field ada. Tidak ada pemeriksaan:
- Apakah `ScopeID` benar-benar milik wilayah yang diizinkan?
- Apakah `ScopeID` adalah RT atau RW?
- Apakah Super Admin punya membership valid untuk scope tersebut?

**Skenario bypass:** Jika middleware menerima header `X-RT-ID:任意-UUID` dan middleware tidak memvalidasinya terhadap authorized scope, lalu permission resolver hanya cek `ScopeID != ""` → SUPER_ADMIN bisa akses seluruh RT.

**Perbaikan:** Tambah validasi:
```go
if ac.SystemRole == SystemRoleSuperAdmin {
    // 1. Validasi scope_type dan scope_id ada dan diizinkan
    var valid bool
    if ac.ScopeType == "rt" {
        valid = tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM rts WHERE id = $1 AND is_active = true AND id IN (SELECT rt_id FROM super_admin_scopes WHERE sa_user_id = $2))`, ac.ScopeID, ac.UserID).Scan(&valid);
    } else if ac.ScopeType == "rw" {
        valid = tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM rws WHERE id = $1 AND is_active = true AND id IN (SELECT rw_id FROM super_admin_scopes WHERE sa_user_id = $2))`, ac.ScopeID, ac.UserID).Scan(&valid);
    }
    if !valid {
        return false, nil
    }
    return true, nil
}
```

#### 1.6 PR 6 (Web UI) dibangun sebelum PR 4 (API) stabil

**Masalah di rencana urutan PR:**

| PR | Deskripsi | Dependensi |
|---|---|---|
| PR 4 | Finance/Fund Location RW Scoping (backend) | PR 2, PR 3 |
| PR 5 | Super Admin Scope Validation | PR 2 |
| PR 6 | Web Frontend: Menu, Route, dan Halaman RW | PR 4 |

PR 6 depends pada PR 4 (API endpoint harus ada). Ini **sudah benar**. Namun:

- PR 4 belum termasuk **API contract definition** — jika kontrak berubah di PR 3 (permission resolver), PR 6 akan butuh rework.
- Tidak ada PR kontrak/API definition sebelum PR 6.

**Rekomendasi:** Sisipkan **PR 3.5 — API Contract Definition** setelah PR 3 dan sebelum PR 4, berisi:
- Definisi request/response schema untuk `/rw/{rwId}/beranda`, `/rw/{rwId}/kas`, `/rw/{rwId}/lokasi-dana`
- Error response schema (401, 403, 404)
- Validasi kontrak di PR 6

#### 1.7 Frontend Web API calls di PR 6 mengusulkan endpoint yang belum ada

Rencana PR 6 mengusulkan:
```
apiGetRWDashboard(rwId)
apiGetRWKas(rwId)
apiGetRWLocations(rwId)
```

Endpoint ini belum ada di kontrak API mana pun. Frontend harus menunggu backend selesai di PR 4 sebelum bisa diimplementasikan. Jika PR 6 dikerjakan paralel dengan PR 4, developer frontend akan menebak kontrak API.

---

### 🟡 SEDANG (Penting tapi tidak blocking)

#### 1.8 Agregasi saldo KAS — transaksi batal/reversal

Rencana PR 4 tidak menjelaskan bagaimana transaksi dengan status `cancelled` dihandle dalam agregasi KAS RW.

**Berdasarkan dokumen acuan Section 11.3:**
> "Untuk memastikan konsistensi antara saldo individual RT dan agregat KAS RW, setiap kali saldo RT berubah, saldo KAS RW juga harus disesuaikan."

Jika transaksi RT menjadi `cancelled` (reversal):
- Saldo RT otomatis berkurang karena reversal transaction di ledger RT
- Agregasi KAS RW harus juga otomatis mencerminkan perubahan ini

**Kueri agregasi yang benar:**
```sql
SELECT 
    SUM(amount) FILTER (WHERE type = 'income' AND status = 'posted') as total_income,
    SUM(amount) FILTER (WHERE type = 'expense' AND status = 'posted') as total_expense,
    SUM(amount) FILTER (WHERE type = 'income' AND status = 'cancelled') as reversed_income,
    SUM(amount) FILTER (WHERE type = 'expense' AND status = 'cancelled') as reversed_expense
FROM transactions
WHERE rt_id IN (SELECT id FROM rts WHERE rw_id = $1 AND is_active = true)
```

**Saldo efektif RW = income_posted + income_cancelled - expense_posted - expense_cancelled**

Ini harus didokumentasikan di rencana PR 4.

#### 1.9 Filter RT aktif dalam agregasi

Rencana PR 4 tidak menyebutkan filter `rts.is_active = true` dalam query agregasi. Jika RT dinonaktifkan, transaksi RT tersebut seharusnya tidak masuk ke agregasi KAS RW.

#### 1.10 Tidak ada test negatif untuk akses lintas RT/RW

Rencana PR 8 hanya menyebutkan:
> "Cross-tenant isolation test"

Ini terlalu umum. Rencana harus mendefinisikan test kasus spesifik:

| Test Kasus | Deskripsi | Status Rencana |
|---|---|---|
| Perangkat RT akses KAS RW scope lain | User RT 001 akses `/rw/{rw002}/kas` | ❌ Tidak ada |
| Ketua RW akses Beranda RT scope lain | User ketua RW 001 akses `/rw/{rt003}/beranda` | ❌ Tidak ada |
| User multi-jabatan POST transaksi RW sebagai perangkat RT | User menjabat perangkat RT + ketua RW → POST transaksi ke RW | ❌ Tidak ada |
| SA akses wilayah tidak di-izin | SA UUID di luar scope → akses ditolak | ❌ Tidak ada |
| Token lama (sebelum field RW) tidak bisa akses RW | Token yang dibuat sebelum PR 2 — akses RW endpoint → ditolak | ❌ Tidak ada |
| Pengguna yang kehilangan jabatan RW | Membership dicabut → token lama tetap valid → akses RW endpoint → ditolak | ❌ Tidak ada |

#### 1.11 `rw_membership.go` AllowedRWJabatans tidak konsisten dengan dokumen acuan

**Bukti aktual:** `backend/internal/rt/rw_membership.go`:
```go
AllowedRWJabatans = {"ketua", "wakil_ketua", "sekretaris", "bendahara"}
```

**Dokumen acuan Section 3.3:**
> "Jabatan RW yang sah: Ketua RW, Wakil Ketua RW, Bendahara RW."

Tiga jabatan RW di dokumen acuan (Ketua RW, Wakil Ketua RW, Bendahara RW). Admin endpoint di `rw_membership.go` mendukung empat jabatan (`ketua`, `wakil_ketua`, `sekretaris`, `bendahara`).

**Ketidakkonsistenan:** Apakah `sekretaris_rw` boleh ada? Dokumen acuan tidak menyebutkan. Rencana PR 1/042a menambahkan `sekretaris_rw` ke CHECK constraint tanpa justifikasi dari dokumen acuan.

**Keputusan produk yang dibutuhkan:** Daftar jabatan RW yang sah — apakah termasuk `sekretaris_rw` atau tidak.

#### 1.12 Tidak ada penanganan token expired/revoked saat membership RW dicabut

Dokumen acuan Section 9.2:
> "Pencabutan jabatan RW dilakukan melalui UI admin → klik tombol LEPASKAN jabatan."

Jika jabatan RW dicabut:
- Token JWT yang sudah terbit **tidak akan ter-refres** karena membership hilang
- Token lama (yang masih valid, ≤15 menit) masih berisi `RWID` dan `RWRole` yang sudah tidak valid

**Perbaikan:** Tambah logic di `RemoveRWJabatan()` untuk:
1. Invalidate refresh token yang terkait
2. Atau — lebih baik — tambahkan claim `jti` (JWT ID) yang dapat di-check di middleware

Rencana PR tidak menyebutkan ini.

---

### 🟢 RENDAH (Opsional — bisa ditangani saat implementasi)

#### 1.13 `position_permissions.position` CHECK constraint — nama atau suffix `_rw`?

Rencana menambahkan `'ketua_rw'` dll ke CHECK constraint. Apakah ini konsisten dengan gaya naming RW di skema existing?

Bukti: `user_rw_memberships.role` menyimpan value `ketua_rw`, `wakil_ketua_rw`, `bendahara_rw` (dari migration 029). Jadi suffix `_rw` sudah ada di skema dan konsisten.

#### 1.14 Migration nomor 042 mungkin bentrok dengan fitur notifikasi

Berdasarkan daftar migration, migration terakhir adalah 041 (`notification_manual_and_image`). Jika ada PR notifikasi lain yang menggunakan 042, akan bentrok. Gunakan nomor unik atau verifikasi di CI.

---

## 2. Matriks Konflik antara Rencana dan Dokumen RBAC

| Ketentuan RBAC | Rencana Audit | Konflik | Tingkat |
|---|---|---|---|
| **Section 2.1:** Menu RW tampil hanya jika pengguna punya jabatan RW sah | Rencana PR 6: Sidebar render berdasarkan `rw` field di AuthContext | ✅ Konsisten | — |
| **Section 2.2:** Perangkat RT bisa akses Beranda RW & KAS RW (baca) | Rencana PR 4: Agregasi RT→RW, endpoint `/rw/{rwId}/beranda` readable by all | ✅ Konsisten | — |
| **Section 2.2:** Perangkat RT tidak bisa Lokasi Dana RW | Rencana PR 4: Endpoint `/rw/{rwId}/lokasi-dana` — tidak ada validasi baca-only di rencana | ⚠️ Perlu spesifikasi eksplisit | SEDANG |
| **Section 3.3:** Sistem harus membaca **seluruh** jabatan pengguna | Rencana: `RWID string`, `RWRole string` (skalar tunggal) | ❌ **Konflik fundamental** | **KRITIS** |
| **Section 3.3:** Gabungan hak akses dari seluruh jabatan | Rencana tidak menjelaskan cara combine permission dari multi-RW | ❌ Belum terdefinisi | **TINGGI** |
| **Section 5.2:** Akses silang baca saja — tidak otomatis izin tulis | Rencana PR 4: Tidak ada eksplisit membatasi tulis ke scope yang benar | ⚠️ Perlu spec | SEDANG |
| **Section 7:** Token menyertakan konteks RT dan RW | Rencana: JWT claims tambah `RWID` + `RWRole` | ⚠️ Tidak support multi-RW | **KRITIS** |
| **Section 9.2:** Pencabutan jabatan RW → hapus akses | Rencana tidak menangani token revocation saat jabatan dicabut | ⚠️ Belum terdefinisi | SEDANG |
| **Section 11.3:** KAS RW = agregasi dari semua RT di bawah RW | Rencana: Agregasi via `rt_id IN (SELECT id FROM rts WHERE rw_id = ?)` | ✅ Konsisten | — |
| **Section 12.1:** Hanya Bendahara yang boleh transfer antar lokasi dana | Rencana tidak menyetel permission transfer ke Bendahara RW saja | ⚠️ Belum terdefinisi | **TINGGI** |
| **Section 12.4:** Validasi akses Super Admin dibatasi RT 002 dan RW 016 | Rencana: `ScopeID != ""` → true | ❌ **Tidak cukup** | **KRITIS** |
| **Section 13.1:** Validasi `X-RT-ID` header untuk memastikan pengguna tidak memanipulasi scope | Rencana tidak menyebutkan validasi `X-RT-ID` terhadap authorized scope | ⚠️ Belum terdefinisi | SEDANG |

---

## 3. Keputusan Teknis yang Sudah Terbukti vs Keputusan Produk yang Belum Disetujui

### Sudah Terbukti (Bukti di codebase)

| Keputusan | Bukti |
|---|---|
| Tabel `rws` ada dan benar | Migration 027: `CREATE TABLE rws (id uuid PK, rw_number text UNIQUE CHECK, name text NOT NULL, created_at, updated_at)` |
| Relasi RT→RW ada dan benar | Migration 027: `ALTER TABLE rts ADD COLUMN rw_id UUID REFERENCES rws(id)` |
| `user_rw_memberships` ada dan benar | Migration 029: `user_id UUID REFERENCES users`, `rw_id UUID REFERENCES rws`, `role text NOT NULL CHECK IN ('ketua_rw', 'wakil_ketua_rw', 'sekretaris_rw', 'bendahara_rw')`, UNIQUE `(user_id, rw_id)` |
| `user_rw_membership_history` ada dan benar | Migration 29: `user_id, rw_id, old_role, new_role, changed_by, changed_at` |
| `permission_types` schema: `id uuid PK, code text UNIQUE, domain text, operation text` | Migration 011 |
| `position_permissions` schema: `id uuid PK, position text CHECK, permission_id uuid FK, UNIQUE (position, permission_id)` | Migration 011 |
| CHECK constraint `position_permissions.position` hanya jabatan RT | Migration 011: `'ketua', 'wakil_ketua', 'sekretaris', 'bendahara', 'keamanan', 'sosial', 'kebersihan_pembangunan', 'anggota'` |
| 12 permission types canonical | Migration 031 + devseed: `warga.read/create/update/import/export`, `finance.read/create/update/approve`, `reports.read`, `rt.settings.read/manage` |
| `transactions` punya `rt_id`, tidak punya `rw_id` | Migration 009, 027 tidak menambahkan `rw_id` ke `transactions` |
| `fund_locations` punya `rt_id`, tidak punya `rw_id` | Migration 034 |
| `fund_transfers` ada di migration 034 | `from_location_id`, `to_location_id`, `rt_id`, `transfer_date`, `reverses_transfer_id` |
| Frontend adalah React/Vite di `frontend/` | `frontend/package.json`, `frontend/src/app/App.tsx` |
| Android adalah Flutter native di `mobile/` | `mobile/pubspec.yaml`, `mobile/lib/app/navigation_shell.dart` |
| Next migration number: **042** | Migration terakhir: 041 |

### Belum Disetujui Produk

| Keputusan | Pilihan | Rekomendasi | Status |
|---|---|---|---|
| **Nama jabatan RW** | `ketua_rw` vs `ketua` suffix vs `ketua_rw` | `ketua_rw` sudah ada di migration 029 CHECK constraint → gunakan | **Butuh konfirmasi: apakah `sekretaris_rw` termasuk?** |
| **KAS RW = agregasi vs mandiri** | Agregasi RT vs tabel mandiri | Agregasi — sudah ada relasi `rts.rw_id`, tidak perlu migration besar | **Butuh persetujuan produk** |
| **Transfer lokasi dana = permission siapa** | Hanya Bendahara vs Ketua+Bendahara | Hanya Bendahara — sesuai dokumen acuan Section 12.1 | **Butuh konfirmasi permission code: `finance.update` atau `fund.transfer`?** |
| **Jumlah permission RW minimal** | Minimal 4: read, create, update, approve vs lebih granular | 4 permission: `rw.finance.read`, `rw.finance.create`, `rw.finance.update`, `rw.finance.approve` | **Butuh persetujuan** |
| **Multi-RW token** | Array vs JSON string vs single | JSON string di JWT claim untuk kompatibilitas | **Butuh persetujuan** |
| **Super Admin scope source** | Hardcoded env vs tabel DB vs membership | Membership — jika SUPER_ADMIN punya entry di `user_rt_memberships` atau `user_rw_memberships`, scope otomatis | **Butuh persetujuan** |

---

## 4. Rencana PR yang Telah Dikoreksi

### PR 1 — Migration 042: CHECK Constraint + Position Permissions RW

**Dependensi:** Tidak ada

**File yang diubah:**
- `backend/migrations/042_rw_position_schema.up.sql` — DROP/ADD CHECK constraint pada `position_permissions.position`, INSERT `ketua_rw`, `wakil_ketua_rw`, `bendahara_rw` (hanya 3 jabatan — sesuai dokumen acuan; jika produk setuju `sekretaris_rw` juga, tambahkan)
- `backend/migrations/042_rw_position_schema.down.sql` — restore CHECK constraint 011, DELETE rows

**SQL:**
```sql
-- 1. Perluas CHECK constraint
ALTER TABLE position_permissions
    DROP CONSTRAINT position_permissions_position_check,
    ADD CONSTRAINT position_permissions_position_check CHECK (position IN (
        'ketua', 'wakil_ketua', 'sekretaris', 'bendahara',
        'keamanan', 'sosial', 'kebersihan_pembangunan', 'anggota',
        'ketua_rw', 'wakil_ketua_rw', 'bendahara_rw'
    ));

-- 2. Grant permission_rw access
--    Menggunakan permission type yang sudah ada: finance.read, finance.create, finance.update, finance.approve
INSERT INTO position_permissions (position, permission_id)
SELECT 'ketua_rw', id FROM permission_types WHERE code IN (
    'finance.read', 'finance.create', 'finance.update', 'finance.approve'
)
ON CONFLICT (position, permission_id) DO NOTHING;

INSERT INTO position_permissions (position, permission_id)
SELECT 'wakil_ketua_rw', id FROM permission_types WHERE code IN (
    'finance.read', 'finance.create', 'finance.approve'
)
ON CONFLICT (position, permission_id) DO NOTHING;

INSERT INTO position_permissions (position, permission_id)
SELECT 'bendahara_rw', id FROM permission_types WHERE code IN (
    'finance.read', 'finance.create', 'finance.update', 'finance.approve'
)
ON CONFLICT (position, permission_id) DO NOTHING;
-- Catatan: BENDAHARA_RW tidak punya 'rw.fund.transfer' karena kita tidak punya permission type fund.transfer
-- Transfer akan dihandle oleh finance.update
```

**Test:**
```sql
SELECT * FROM position_permissions WHERE position LIKE '%_rw' ORDER BY position;
-- Harus ada: bendahara_rw (4 grants), ketua_rw (4 grants), wakil_ketua_rw (3 grants)
-- Total 11 baris
```

**Kriteria penerimaan:** Migration up/down berjalan tanpa error. Query mengembalikan data yang diharapkan. CHECK constraint sudah include 3 jabatan RW.

**Risiko:** DROP/ADD CHECK constraint akan meng-lock table `position_permissions` sementara. Jika production traffic tinggi, pertimbangkan di jam low-traffic.

### PR 1.5 — Migration 043: Permission Types RW (jika produk setujui custom domain `rw`)

**Dependensi:** PR 1

**File yang diubah:**
- `backend/migrations/043_rw_permission_types.up.sql` — INSERT permission types baru domain `rw`:
  - `rw.finance.read` → domain `rw`, operation `read`
  - `rw.finance.create` → domain `rw`, operation `create`
  - `rw.finance.update` → domain `rw`, operation `update`
  - `rw.finance.approve` → domain `rw`, operation `approve`
  - `rw.fund.transfer` → domain `rw`, operation `transfer` (hanya untuk Bendahara RW)

> **Catatan:** Jika produk memutuskan untuk menggunakan permission existing (`finance.read`, `finance.create`, `finance.update`, `finance.approve`), maka PR 1.5 tidak diperlukan. Ini menghemat 1 migration.

**Keputusan produk:** Gunakan permission existing (domain `finance`) ATAU buat permission baru (domain `rw`)?

### PR 2 — JWT Extension (Multi-RW Support)

**Dependensi:** PR 1, PR 1.5 (jika ada)

**File yang diubah:**
- `backend/internal/auth/models.go` — tambah `RWContext string` ke `TokenClaims` (JSON) dan `AuthContext` (deserialized struct)
- `backend/internal/auth/jwt.go` — `GenerateAccessToken()` serialize RW memberships ke JSON; `RequireAuth()` deserialize dari JWT ke `AuthContext`
- `backend/internal/auth/middleware.go` — `RequireAuth()`: extract `RWContext` dari claims, parse JSON, populate `AuthContext.RWMemberships`
- `backend/internal/auth/service.go` — `Login()`: query `user_rt_memberships` dan `user_rw_memberships`; inject keduanya ke token; `GetProfile()`: query dan return RW membership
- `backend/internal/auth/handlers.go` — `handleMe()`: untuk semua user (tidak hanya SA), query `user_rw_memberships` dan return di response

**Desain Token Claims:**
```go
type TokenClaims struct {
    UserID     string   `json:"sub"`
    MID        string   `json:"mid"`
    RTID       string   `json:"rt_id"`
    Role       string   `json:"role"`
    Jabatan    string   `json:"jabatan"`
    SysRole    string   `json:"sys_role"`
    RWContext  string   `json:"rw_context"` // JSON: [{"rwId":"x","role":"y"}, ...]
}

type RWMembership struct {
    RWID string `json:"rwId"`
    Role string `json:"role"`
}
```

**Kompatibilitas Token Lama:**
- Token lama (tanpa `RWContext`) → parsing berhasil (field empty string)
- Middleware: jika `RWContext == ""`, `AuthContext.RWMemberships = nil`
- Endpoint RW: jika `AuthContext.RWMemberships == nil` atau scope tidak ada → 403
- Tidak ada breaking change — client yang tidak expect field baru tidak terpengaruh

**Test:**
- Login user RT-only → token `RWContext = "[]"` (empty array)
- Login user 1 jabatan RW → token `RWContext = '[{"rwId":"x","role":"ketua_rw"}]'`
- Login user 2 jabatan RW → token `RWContext = '[{"rwId":"x","role":"ketua_rw"},{"rwId":"y","role":"bendahara_rw"}]'`
- `/me` user RT-only → response `rw: []`; user RT+RW → response `rw: [{id, role}]`
- Refresh token user RW → token baru punya field RW
- Token lama tanpa field RW → akses endpoint RW → 403

**Kriteria penerimaan:** Semua test pass. Tidak ada breaking change pada existing endpoint RT.

### PR 3 — Permission Resolver + Super Admin Validation (gabungan)

**Dependensi:** PR 1, PR 2

**File yang diubah:**
- `backend/internal/auth/permission.go` — `HasPermission()`:
  1. Cek `ac.RWRole` — jika ada, cek `position_permissions WHERE position = ac.RWRole`
  2. Jika tidak ada di RW permission, fallback ke `ac.Jabatan` (RT permission)
  3. SUPER_ADMIN: validasi `ScopeID` dan `ScopeType` terhadap authorized scope

**Logic permission check:**
```go
func (pr *PermissionResolver) HasPermission(ac AuthContext, permissionCode string) (bool, error) {
    if ac.SystemRole == SystemRoleSuperAdmin {
        // Validasi scope
        if ac.ScopeID == "" || ac.ScopeType == "" {
            return false, nil
        }
        // Query authorized scopes
        var authorized bool
        if ac.ScopeType == "rt" {
            db.QueryRow(`SELECT EXISTS(SELECT 1 FROM rts WHERE id = $1 AND is_active = true AND id IN (SELECT rt_id FROM super_admin_scopes WHERE sa_user_id = $2))`,
                ac.ScopeID, ac.UserID).Scan(&authorized)
        } else if ac.ScopeType == "rw" {
            db.QueryRow(`SELECT EXISTS(SELECT 1 FROM rws WHERE id = $1 AND is_active = true AND id IN (SELECT rw_id FROM super_admin_scopes WHERE sa_user_id = $2))`,
                ac.ScopeID, ac.UserID).Scan(&authorized)
        }
        return authorized, nil
    }

    // Cek RW permission dulu
    if len(ac.RWMemberships) > 0 {
        for _, rm := range ac.RWMemberships {
            hasRW, err := checkPositionPermission(ac.RTDB, rm.Role, permissionCode)
            if hasRW {
                return true, err
            }
        }
    }

    // Fallback: cek RT permission
    return checkPositionPermission(ac.RTDB, ac.Jabatan, permissionCode)
}
```

**Test:**
- `hasPermission(ketua_rw, rw.finance.read)` → true
- `hasPermission(wakil_ketua_rw, rw.finance.read)` → true
- `hasPermission(bendahara_rw, rw.finance.create)` → true
- `hasPermission(warga, rw.finance.read)` → false
- `hasPermission(ketua_rw, rw.fund.transfer)` → false (jika transfer = permission khusus)
- `hasPermission(bendahara_rw, rw.fund.transfer)` → true (jika transfer = permission khusus)
- SA akses scope tidak diizinkan → false
- SA akses scope diizinkan → true
- SA tanpa ScopeID → false

### PR 4 — Finance/Fund Location RW Scoping (Backend)

**Dependensi:** PR 2, PR 3

**File yang diubah:**
- `backend/internal/finance/router.go` — tambah route:
  - `GET /rw/:rwId/beranda` → `GetRWDashboard`
  - `GET /rw/:rwId/kas` → `GetRWKas`
- `backend/internal/finance/handler.go` — tambah handler:
  - `GetRWDashboard(rwId string)` → query agregasi RT→RW
  - `GetRWKas(rwId string)` → query agregasi RT→RW
- `backend/internal/finance/service.go` — tambah method:
  - `GetDashboardRW(ctx, rwId string)`
  - `GetKasRW(ctx, rwId string)`
- `backend/internal/finance/repository.go` — tambah query:
  - `GetTransactionsForRW(ctx, rwId string)` → `WHERE rt_id IN (SELECT id FROM rts WHERE rw_id = ? AND is_active = true)`
- `backend/internal/fund_location/router.go` — tambah route:
  - `GET /rw/:rwId/lokasi-dana` → `GetRWFundLocations`
- `backend/internal/fund_location/handler.go` — tambah handler:
  - `GetRWFundLocations(rwId string)` → hanya **read-only**, validasi permission `rw.fund.read`
- `backend/internal/fund_location/service.go` — tambah method:
  - `GetLocationsForRW(ctx, rwId string)` → `WHERE rt_id IN (SELECT id FROM rts WHERE rw_id = ? AND is_active = true)`

**Query agregasi saldo RW:**
```sql
SELECT
    COALESCE(SUM(amount) FILTER (WHERE type = 'income' AND status = 'posted'), 0) as total_income,
    COALESCE(SUM(amount) FILTER (WHERE type = 'expense' AND status = 'posted'), 0) as total_expense,
    COALESCE(SUM(amount) FILTER (WHERE type = 'income' AND status = 'cancelled'), 0) as reversed_income,
    COALESCE(SUM(amount) FILTER (WHERE type = 'expense' AND status = 'cancelled'), 0) as reversed_expense,
    (COALESCE(SUM(amount) FILTER (WHERE type = 'income' AND status IN ('posted', 'cancelled')), 0) -
     COALESCE(SUM(amount) FILTER (WHERE type = 'expense' AND status IN ('posted', 'cancelled')), 0)) as balance
FROM transactions
WHERE rt_id IN (
    SELECT id FROM rts WHERE rw_id = $1 AND is_active = true
)
```

**Kriteria penerimaan:** 
- Query agregasi menghasilkan saldo yang sama dengan sum of individual RT saldo
- Status `cancelled` transaksi mempengaruhi saldo agregat
- User tanpa jabatan RW akses → 403
- User perangkat RT akses Beranda RW → 200 baca
- User perangkat RT akses Lokasi Dana RW → 403

### PR 5 — Web Frontend

**Dependensi:** PR 4 (API endpoint harus ready)

**File yang diubah:**
- `frontend/src/app/api.ts` — tambah `apiGetRWDashboard(rwId)`, `apiGetRWKas(rwId)`, `apiGetRWLocations(rwId)`
- `frontend/src/app/AuthContext.tsx` — tambah field `rw` ke `AuthUser`
- `frontend/src/components/Sidebar.tsx` — tambah menu RW (conditional per role)
- `frontend/src/app/App.tsx` — tambah route RW operasional
- `frontend/src/features/rw/RwHome.tsx`, `RwKas.tsx`, `RwLokasiDana.tsx` — halaman baru

### PR 6 — Android: Horizontal Sliding + Menu RW

**Dependensi:** PR 4 (API endpoint harus ready), PR 5 (konsep menu)

**File yang diubah:**
- `mobile/lib/app/navigation_shell.dart` — ganti fixed `TabBar` dengan `TabBar` + `PageView` horizontal sliding
- `mobile/lib/core/models/role.dart` — tambah role RW enum
- `mobile/lib/features/rw/rw_home_screen.dart`, `rw_kas_screen.dart`, `rw_lokasi_dana_screen.dart` — halaman baru
- `mobile/lib/features/rw/rw_repository.dart` — tambah method API RW

### PR 7 — Integration & Regression Tests

**Dependensi:** Semua PR sebelumnya

**Test wajib:**
- Semua test kasus dari dokumen acuan Section 14
- Test negatif: akses lintas RT/RW, multi-jabatan, SA di luar scope, akses tulis yang dilarang
- Regression: semua test finance/fund_location RT existing
- `flutter analyze` — 0 error, 0 warning
- `git diff --check` — pass

---

## 5. Rekomendasi: Apakah Implementasi Boleh Dimulai?

### Ya, **SIAP DIMULAI** — setelah 3 keputusan produk dikonfirmasi:

| # | Keputusan | Status Saat Ini | Impact Jika Ditunda |
|---|---|---|---|
| 1 | **Nama jabatan RW** — apakah termasuk `sekretaris_rw`? | Belum dikonfirmasi | PR 1 bisa pakai 3 jabatan saja, revisi nanti jika diperlukan |
| 2 | **Permission model** — gunakan permission existing (`finance.*`) atau buat permission baru (`rw.*`)? | Belum diputuskan | Menentukan是否需要 PR 1.5 — jika pakai existing, tidak perlu migration baru |
| 3 | **Super Admin scope source** — hardcoded env, tabel DB, atau membership? | Belum diputuskan | Menentukan desain tabel/konfigurasi untuk PR 3 |

### Tanpa ketiga keputusan ini, implementasi PR 1 bisa dimulai dengan asumsi:
- Hanya 3 jabatan RW: `ketua_rw`, `wakil_ketua_rw`, `bendahara_rw` (sesuai dokumen acuan)
- Gunakan permission existing (`finance.read`, `finance.create`, `finance.update`, `finance.approve`) — tidak perlu PR 1.5
- Super Admin scope pakai environment variable sebagai quick win, migrasi ke tabel di PR terpisah

**Catatan:** Tiga keputusan ini tidak menghalangi 80% kerja teknis. Hanya memengaruhi detail migration dan permission resolver, yang bisa disesuaikan saat implementasi.

---

*Akhir review desain pra-implementasi.*
