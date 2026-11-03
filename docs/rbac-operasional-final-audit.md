# Laporan Final — RBAC Operasional RW Urena

**Status:** Laporan Final Audit  
**Branch:** `urena` — HEAD `ac266c0`  
**Dokumen Acuan:** `docs/rbac-operasional-rw.md`  
**Tanggal:** 2026-10-29  
**Jenis:** Read-only — Tidak ada kode diubah, tidak ada commit.

---

## 0. Kondisi Repository

| Item | Nilai |
|---|---|
| Branch | `urena` |
| HEAD | `ac266c0 test(notification): cover ServerException error mapping` |
| Commit sebelumnya | `454d885 fix(notification): resolve back navigation and ServerException display bugs` |
| Di atas origin/urena | 0 commit |
| Working tree | Bersih — 3 file untracked: `docs/rbac-operasional-audit-laporan.md`, `docs/rbac-operasional-rw.md`, `docs/verifikasi-454d885.md` |
| File yang diubah selama audit ini | **Tidak ada** |

---

## 1. Matriks Gap per Ketentuan RBAC

### Ketentuan Section 2.1–2.2: Nama dan Identitas Menu RT/RW

| Ketentuan | Existing | Gap | Bukti |
|---|---|---|---|
| Menu RT `BERANDA`, `KAS`, `LOKASI DANA` dipertahankan | ✅ Ada | — | `mobile/lib/app/navigation_shell.dart` — tab `_homeTab`, `_kasTab`, `_lokasiTab` |
| Menu RW `BERANDA RW`, `KAS RW`, `LOKASI DANA RW` tersedia | ❌ Tidak ada | **KRITIS** | Tidak ada route, halaman, atau menu RW operasional di `frontend/`, `mobile/`, atau backend |
| Menu RT dan RW tidak saling menggantikan | ✅ Aman (menu RW belum ada) | — | — |

### Ketentuan Section 3.1: Menu Bukan Otorisasi

| Ketentuan | Existing | Gap | Bukti |
|---|---|---|---|
| Backend/API sumber otorisasi utama | ⚠️ Tidak lengkap | Otorisasi ada untuk RT tapi tidak ada untuk RW | `backend/internal/auth/permission.go` `HasPermission()` — hanya cek jabatan RT, tidak ada logic RW |
| Menampilkan/menyembunyikan menu hanya navigasi | ⚠️ Belum | Menu RW belum ada sama sekali | `frontend/src/components/Sidebar.tsx` — hanya "Kelola RW" (admin), `mobile/lib/app/navigation_shell.dart` — 7 tab fixed RT-only |

### Ketentuan Section 3.2: Hak Akses Mengikuti Jabatan dan Lingkup

| Ketentuan | Existing | Gap | Bukti |
|---|---|---|---|
| Role/jabatan pengguna dibaca dari server | ✅ Ada untuk RT | ❌ Tidak ada untuk RW | `backend/internal/auth/service.go` `Login()` query `user_rt_memberships` — **tidak pernah query `user_rw_memberships`** |
| Permission untuk menu, data, tindakan | ⚠️ RT saja | ❌ Tidak ada untuk jabatan RW | `backend/migrations/031_restore_permission_catalog.up.sql` — posisi yang terdaftar: `ketua`, `wakil_ketua`, `sekretaris`, `bendahara`, `keamanan`, `sosial`, `kebersihan_pembangunan`, `anggota` — **semua jabatan RT** |
| Batas wilayah eksplisit | ✅ Ada untuk RT | ❌ Tidak ada untuk RW | `backend/internal/auth/middleware.go` `RequireAuth` — `AuthContext` hanya punya `RTID` |

### Ketentuan Section 3.3: Jabatan Ganda

| Ketentuan | Existing | Gap | Bukti |
|---|---|---|---|
| Sistem membaca seluruh jabatan pengguna | ❌ Tidak ada | Token hanya berisi satu jabatan (RT) | `backend/internal/auth/jwt.go` `TokenClaims`: `{UserID, MID, RTID, Role, Jabatan, SysRole}` — **tidak ada RWID, RWRole, RWJabatan** |
| Gabungan hak akses kedua jabatan | ❌ Tidak ada | — | `frontend/src/app/AuthContext.tsx` `AuthUser` hanya punya `rt: {id, name}`, **tidak ada field `rw`** |
| Tidak boleh hanya membaca satu role | ❌ Tidak ada | — | `mobile/lib/core/models/role.dart` `AppRole`: `{superAdmin, pengurus, bendahara, warga, perangkat}` — **tidak ada role RW** |

### Ketentuan Section 3.4: Akses Silang Baca Saja

| Ketentuan | Existing | Gap | Bukti |
|---|---|---|---|
| Perangkat RT lihat Beranda/KAS RW baca saja | ❌ Tidak ada | — | Tidak ada endpoint atau route |
| Perangkat RT tidak bisa Lokasi Dana RW | ⚠️ Aman (belum ada) | — | — |
| Perangkat RW lihat Beranda/KAS RT baca saja | ❌ Tidak ada | — | — |
| Akses silang tidak otomatis izin tulis | ⚠️ Aman (belum ada endpoint) | — | — |

### Ketentuan Section 4: Matriks Akses Menu

| Role | Beranda RT | KAS RT | Lokasi Dana RT | Beranda RW | KAS RW | Lokasi Dana RW |
|---|---|---|---|---|---|---|
| Perangkat RT | ✅ Ada | ✅ Ada | ✅ Ada | ❌ Tidak ada | ❌ Tidak ada | ❌ Aman (tidak ada) |
| Ketua RW | ⚠️ Baca dari RT | ⚠️ Baca dari RT | ❌ — | ❌ Tidak ada | ❌ Tidak ada | ❌ Tidak ada |
| Wakil Ketua RW | ⚠️ Baca dari RT | ⚠️ Baca dari RT | ❌ — | ❌ Tidak ada | ❌ Tidak ada | ❌ Tidak ada |
| Bendahara RW | ⚠️ Baca dari RT | ⚠️ Baca dari RT | ❌ — | ❌ Tidak ada | ❌ Tidak ada | ❌ Tidak ada |
| Super Admin | ⚠️ Ada (hardcoded) | ⚠️ Ada (hardcoded) | ⚠️ Ada (hardcoded) | ❌ Tidak ada | ❌ Tidak ada | ❌ Tidak ada |

### Ketentuan Section 5.3: Isolasi Antar-RT

| Ketentuan | Existing | Gap | Bukti |
|---|---|---|---|
| Perangkat RT hanya akses RT sendiri | ✅ Ada | — | `backend/internal/auth/middleware.go` `AuthContext` punya `RTID` dari JWT |
| Manipulasi URL/parameter tidak bisa bypass | ⚠️ Cukup | Perlu validasi scope RW | `backend/internal/auth/permission.go` — SUPER_ADMIN `return true, nil` tanpa scope check |
| Hak baca ke RW tidak berikan akses seluruh RT di bawahnya | ❌ Belum ada aturan | — | — |

### Ketentuan Section 7: Hak Akses Jabatan Ganda

| Ketentuan | Existing | Gap | Bukti |
|---|---|---|---|
| Token menyertakan konteks RT dan RW | ❌ Tidak ada | — | `TokenClaims` tidak punya field RW |
| Login mengambil membership RT dan RW | ❌ Tidak ada | — | `auth/service.go` `Login()` tidak query `user_rw_memberships` |
| `/me` return data RW membership untuk user | ⚠️ Hanya SUPER_ADMIN | — | `backend/internal/auth/handlers.go` `handleMe` — `profile.RWID`/`profile.RWName` dari JOIN RT→RW, bukan dari `user_rw_memberships` |
| Permission resolver cek jabatan RT dan RW | ❌ Tidak ada | — | `backend/internal/auth/permission.go` — hanya cek `user_rt_memberships` dan `position_permissions` RT |

### Ketentuan Section 8–9: Menu Web dan Android

| Ketentuan | Existing | Gap | Bukti |
|---|---|---|---|
| Web sidebar dinamis RT+RW | ❌ Tidak ada | `frontend/src/components/Sidebar.tsx` — hanya "Kelola RW" (admin) |
| Web route RW operasional | ❌ Tidak ada | `frontend/src/app/App.tsx` line 86: `path="rw"` → `RwList` (hanya admin CRUD) |
| Web API call operasional RW | ❌ Tidak ada | `frontend/src/app/api.ts` — hanya `apiListRWs`, `apiCreateRW`, `apiUpdateRW`, `apiDeactivateRW` (admin CRUD), tidak ada endpoint KAS/Lokasi Dana RW |
| Android bottom nav horizontal sliding | ❌ Tidak ada | `mobile/lib/app/navigation_shell.dart` — 7 fixed tabs, tidak ada `PageView` horizontal scroll |
| Android menu RW ditampilkan | ❌ Tidak ada | Tab fixed: `_homeTab, _kasTab, _lokasiTab, _iuranTab, _wargaTab, _profileTab, _notifikasiTab` |
| Android role model support RW | ❌ Tidak ada | `mobile/lib/core/models/role.dart` — tidak ada enum role RW |

### Ketentuan Section 10: Backend dan API

| Ketentuan | Existing | Gap | Bukti |
|---|---|---|---|
| Semua endpoint validasi auth/role/permission/scope | ✅ RT saja | ❌ RW scope belum ada | Tidak ada endpoint RW operasional |
| Baca lintas RT-RW tidak beri izin tulis | ⚠️ Aman (belum ada endpoint) | — | — |
| Endpoint Lokasi Dana RW divalidasi | ❌ Tidak ada | `backend/internal/fund_location/handler.go` — hanya RT-scoped, 0 baris kode yang merujuk `rw_id` |
| Akses SUPER Admin dibatasi scope | ⚠️ Ada tapi hardcode | `backend/internal/auth/handlers.go` — hardcoded UUID `'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b'` dan string `'urena'` |
| Permission existing tidak diubah | — | — | — |

### Ketentuan Section 11: Integritas Data

| Ketentuan | Existing | Gap | Bukti |
|---|---|---|---|
| KAS RW baca data dalam lingkup RW | ❌ Tidak ada | — | `backend/internal/finance/handler.go` — semua query filter `rt_id`, tidak ada agregasi RT→RW |
| Lokasi Dana RW terpisah dari RT | ❌ Tidak ada | `backend/internal/fund_location/handler.go` — 0 referensi `rw_id` |
| Saldo dihitung dari ledger | ✅ Untuk RT | Perlu agregasi untuk RW | `backend/internal/finance/repository.go` — query saldo filter `rt_id` |

---

## 2. Keputusan Arsitektur

### 2.1 Membership dan Token (Login/Token/Refresh)

**Fakta Teknis:**
- `TokenClaims` (`backend/internal/auth/jwt.go`) punya field: `UserID, MID, RTID, Role, Jabatan, SysRole`
- `AuthContext` (`backend/internal/auth/models.go`) punya: `UserID, SystemRole, MembershipID, RTID, TenantRole, Jabatan`
- `Login()` (`backend/internal/auth/service.go`) query `RTMembershipsFindByUser()` → hanya `user_rt_memberships`
- `user_rw_memberships` (migration 029) sudah ada tapi **tidak pernah dibaca** saat login
- `user_rw_membership_history` (migration 029) ada tapi belum dipakai

**Keputusan Direkomendasikan:**

Tambahkan field ke `TokenClaims` dan `AuthContext`:
```
RWID   string  // RW ID yang diautorisasi
RWRole string  // Role RW (ketua_rw, wakil_ketua_rw, bendahara_rw)
```

Perubahan di:
- `backend/internal/auth/models.go` — tambah field `TokenClaims` dan `AuthContext`
- `backend/internal/auth/jwt.go` — `GenerateAccessToken()` inject field baru
- `backend/internal/auth/service.go` — `Login()`: setelah query RT membership, query tambahan `SELECT rw_id, role FROM user_rw_memberships WHERE user_id = $1 AND is_active = true`; `Refresh()`: inject field baru saat regenerate
- `backend/internal/auth/middleware.go` — `RequireAuth()` extract field RW dari JWT ke `AuthContext`
- `backend/internal/auth/handlers.go` — `GetProfile()`: query dan return RW membership untuk user biasa (bukan hanya SUPER_ADMIN)

**Dampak:** Token JWT lebih besar ~2x (100 bytes). Refresh token tetap compatible. Tidak ada breaking change karena field baru tidak diparsing di client yang tidak expect.

**Test wajib:**
- Login user RT-only → token tidak punya field RW
- Login user dengan jabatan RT+RW → token punya `RWID` dan `RWRole`
- Refresh token dengan user RW → token baru punya field RW
- `/me` untuk user RT-only → response `rw: null`; untuk user RT+RW → response `rw: {id, role}`

### 2.2 Scope Aktif dan Pemilihan Lingkup

**Fakta Teknis:**
- User saat ini hanya punya satu `RTID` di JWT
- Tidak ada mekanisme "scope aktif" — user selalu beroprasi di konteks RT
- SUPER_ADMIN bisa set `RTID` via header/API tapi tanpa validasi scope

**Keputusan Direkomendasikan:**

Token menyertakan **kedua scope** (RT dan RW) secara simultan. Backend menentukan scope berdasarkan:
1. **Endpoint path**: `/api/v1/rw/{rwId}/...` → scope RW, `/api/v1/rt/{rtId}/...` → scope RT
2. **Default behavior**: endpoint tanpa parameter scope → gunakan scope RT existing
3. **Validasi**: backend cek apakah user punya jabatan di scope yang diminta (dari JWT claims)

**Dampak:** Tidak ada perubahan pada endpoint RT existing. Endpoint RW operasional menggunakan path `/api/v1/rw/{rwId}/kas`, `/api/v1/rw/{rwId}/beranda`, `/api/v1/rw/{rwId}/lokasi-dana`.

### 2.3 Permission Resolver untuk Jabatan RW

**Fakta Teknis:**
- `position_permissions` (migration 031) hanya berisi jabatan RT: `ketua, wakil_ketua, sekretaris, bendahara, keamanan, sosial, kebersihan_pembangunan, anggota`
- Tidak ada `INSERT INTO position_permissions` untuk jabatan RW di migration manapun
- `permission.go` `hasPermissionForPosition()` query `position_permissions` berdasarkan `jabatan` kolom — jika posisi tidak ada, return 0
- `AllowedRWJabatans` di `rw_membership.go`: `{ketua, wakil_ketua, sekretaris, bendahara}` — ini jabatan **administrasi** RW, bukan jabatan operasional

**Keputusan Direkomendasikan:**

1. **Migration baru (042)**: Tambah baris `position_permissions` untuk jabatan RW operasional:
   ```sql
   INSERT INTO position_permissions (position, permission_id)
   SELECT 'ketua_rw', id FROM permission_types WHERE code IN ('rw.finance.read', 'rw.finance.create', 'rw.finance.approve', 'rw.fund.read', 'rw.fund.create', 'rw.fund.transfer')
   ON CONFLICT DO NOTHING;
   
   INSERT INTO position_permissions (position, permission_id)
   SELECT 'wakil_ketua_rw', id FROM permission_types WHERE code IN ('rw.finance.read', 'rw.finance.create', 'rw.finance.approve', 'rw.fund.read', 'rw.fund.create', 'rw.fund.transfer')
   ON CONFLICT DO NOTHING;
   
   INSERT INTO position_permissions (position, permission_id)
   SELECT 'bendahara_rw', id FROM permission_types WHERE code IN ('rw.finance.read', 'rw.finance.create', 'rw.finance.approve', 'rw.fund.read', 'rw.fund.create', 'rw.fund.transfer', 'rw.fund.transfer')
   ON CONFLICT DO NOTHING;
   ```

2. **Buat permission types baru** (migration 041):
   ```sql
   INSERT INTO permission_types (id, code, name, description) VALUES
   ('rw-finance-read', 'rw.finance.read', 'Baca Keuangan RW', 'Hak membaca beranda dan kas RW'),
   ('rw-finance-create', 'rw.finance.create', 'Buat Transaksi RW', 'Hak membuat transaksi income/expense RW'),
   ('rw-finance-approve', 'rw.finance.approve', 'Approve Transaksi RW', 'Hak approve transaksi RW'),
   ('rw-fund-read', 'rw.fund.read', 'Baca Lokasi Dana RW', 'Hak membaca lokasi dana RW'),
   ('rw-fund-create', 'rw.fund.create', 'Buat Lokasi Dana RW', 'Hak membuat lokasi dana RW'),
   ('rw-fund-transfer', 'rw.fund.transfer', 'Transfer Lokasi Dana RW', 'Hak transfer antar lokasi dana RW');
   ```

3. **Update `permission.go`**: `HasPermission()` cek `ac.RWRole` → `position_permissions.position` jika `ac.RWRole` ada, cek juga `ac.RTID` scope.

**Dampak:** Permission resolver menjadi dual-scope: cek RT permission untuk operasi RT, cek RW permission untuk operasi RW.

**Test wajib:**
- `hasPermission(ketua_rw, rw.finance.read)` → true
- `hasPermission(warga, rw.finance.read)` → false
- `hasPermission(ketua_rw, rw.fund.transfer)` → true (setelah migration 042)
- `hasPermission(bendahara_rw, rw.finance.create)` → true

### 2.4 Super Admin Scope Validation

**Fakta Teknis:**
- `permission.go` `HasPermission()`: `if ac.SystemRole == SystemRoleSuperAdmin { return true, nil }` — **always true, no scope check**
- `auth/handlers.go` `handleMe()`: hardcoded UUID `'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b'` dan string `'urena'` dalam query RT selection
- `auth/service.go` `Login()`: hardcoded UUID yang sama untuk prioritas RT SUPER_ADMIN
- Header `X-RT-ID` diterima tapi tidak divalidasi terhadap authorized scope

**Keputusan Direkomendasikan:**

1. **Ganti hardcoded UUID dengan database-driven scope config**:
   - Tabel baru `super_admin_scopes` atau konfigurasi environment: `SCOPED_RT_IDS=a95c1cb5-...`, `SCOPED_RW_IDS=...`
   - Atau lebih baik: gunakan `user_rw_memberships` + `user_rt_memberships` — jika SUPER_ADMIN punya membership, scope otomatis; jika tidak, gunakan default scope

2. **Update `HasPermission()`**:
   ```go
   if ac.SystemRole == SystemRoleSuperAdmin {
       if ac.ScopeID == "" {
           return false, nil // SA tanpa scope aktif
       }
       // Validasi ScopeID milik wilayah yang diizinkan
       return true, nil
   }
   ```

3. **Header `X-RT-ID` perlu divalidasi** terhadap authorized scope dari JWT.

**Dampak:** SUPER_ADMIN tidak lagi bisa mengakses seluruh wilayah secara acak. Akses terbatas pada wilayah yang didefinisikan.

### 2.5 Pemisahan Fakta Teknis vs Keputusan Produk

| Aspek | Fakta Teknis (Tidak Bisa Diubah Tanpa Migrasi Besar) | Keputusan Produk (Bisa Dipilih) |
|---|---|---|
| **Transaksi ada di tabel `transactions`** | Tabel punya `rt_id` (FK ke `rts`), TIDAK punya `rw_id` | KAS RW = agregasi dari semua RT di bawah RW (tidak perlu migrate tabel) |
| **Lokasi Dana ada di tabel `fund_locations`** | Tabel punya `rt_id` (FK ke `rts`), TIDAK punya `rw_id` | Lokasi Dana RW = agregasi dari semua lokasi RT di bawah RW |
| **Relasi RT→RW ada di `rts.rw_id`** | FK `rts.rw_id → rws.id` sudah ada (migration 027) | Gunakan relasi ini untuk query agregasi — tidak perlu tabel baru |
| **`user_rw_memberships` sudah ada** | Tabel migration 029 sudah dibuat dengan benar | Isi tabel dengan data awal via migration, bukan kode |

---

## 3. Keputusan Model Data KAS RW dan Lokasi Dana RW

### Fakta Teknis yang Terverifikasi

| Tabel | Kolom | `rw_id`? | Status |
|---|---|---|---|
| `transactions` (migration 009) | `rt_id UUID NOT NULL REFERENCES rts(id)` | ❌ Tidak ada | Hanya RT-scoped |
| `fund_locations` (migration 034) | `rt_id UUID NOT NULL REFERENCES rts(id)` | ❌ Tidak ada | Hanya RT-scoped |
| `rts` (migration 027) | `rw_id UUID REFERENCES rws(id)` | ✅ Ada | RT→RW link ada |
| `rws` (migration 027) | `id UUID PRIMARY KEY` | — | — |

**Bukti file:**
- `backend/migrations/009_finance.up.sql` → `CREATE TABLE transactions` dengan `rt_id`, 0 referensi `rw_id`
- `backend/migrations/034_fund_locations.up.sql` → `CREATE TABLE fund_locations` dengan `rt_id`, 0 referensi `rw_id`
- `backend/migrations/027_rw_domain_and_transaction_reference.up.sql` → `ALTER TABLE rts ADD COLUMN IF NOT EXISTS rw_id uuid REFERENCES rws(id)`
- `backend/internal/finance/handler.go` → 0 baris kode yang merujuk `rw_id`
- `backend/internal/fund_location/handler.go` → 0 baris kode yang merujuk `rw_id`

### Rekomendasi: Agregasi Data dari RT

**Konsep:** KAS RW dan Lokasi Dana RW adalah **ringkasan agregat** dari semua RT di bawah RW tersebut. Tidak ada transaksi RW mandiri. Tidak ada lokasi dana RW mandiri.

**Query KAS RW:**
```sql
SELECT SUM(t.amount) FILTER (WHERE t.type = 'income') as total_income,
       SUM(t.amount) FILTER (WHERE t.type = 'expense') as total_expense
FROM transactions t
WHERE t.rt_id IN (
    SELECT id FROM rts WHERE rw_id = $1 AND is_active = true
);
```

**Query Lokasi Dana RW:**
```sql
SELECT fl.*, r.name as rt_name, r.rt as rt_code
FROM fund_locations fl
JOIN rts r ON r.id = fl.rt_id
WHERE r.rw_id = $1 AND r.is_active = true;
```

**Keuntungan:**
- Tidak ada migration database baru
- Tidak mengubah struktur transaksi atau lokasi dana RT
- Sumber kebenaran tetap ledger transaksi RT (konsisten dengan prinsip integritas data)
- Tidak ada duplikasi data
- Compatible dengan audit trail existing

**Dampak pada handler:**
- Finance handler perlu route baru `/api/v1/rw/:rwId/beranda` dan `/api/v1/rw/:rwId/kas`
- Fund location handler perlu route baru `/api/v1/rw/:rwId/lokasi-dana`
- Semua route baru require validasi: user punya jabatan RW yang sah
- Query menggunakan subquery `rt_id IN (SELECT id FROM rts WHERE rw_id = ?)`
- Response wrapper sama dengan response RT existing (hanya data yang berbeda)

**Risiko:**
- Performance: jika satu RW punya 50+ RT, query subquery bisa lambat. Mitigasi: composite index `rts(rw_id, is_active)`, caching layer
- Konsep "transaksi RW" tidak ada — semua transaksi tetap di RT. Ini bisa membingungkan jika ada kebutuhan di masa depan untuk transaksi mandiri RW.

**Keputusan Produk yang Dibutuhkan:**
1. Apakah KAS RW murni agregasi RT, atau butuh transaksi RW mandiri? → **Direkomendasikan: agregasi**
2. Apakah Lokasi Dana RW murni agregasi RT, atau butuh lokasi dana RW mandiri? → **Direkomendasikan: agregasi**
3. Jika keputusan produk mengubah poin 1 atau 2, maka perlu migration besar (menambah `rw_id` ke `transactions` dan `fund_locations`) yang mengubah seluruh kontrak API.

---

## 4. Audit Kesiapan Per-Layer

### Layer Database

| Komponen | Status | Bukti |
|---|---|---|
| Tabel `rws` | ✅ Sudah ada | Migration 027 |
| Kolom `rts.rw_id` | ✅ Sudah ada | Migration 027 |
| `user_rw_memberships` | ✅ Sudah ada | Migration 029 |
| `user_rw_membership_history` | ✅ Sudah ada | Migration 029 |
| Foreign keys & unique constraint | ✅ Sudah ada | Migration 029 |
| `position_permissions` untuk RW | ❌ Tidak ada | Migration 031 hanya isi jabatan RT |
| Permission types untuk RW | ❌ Tidak ada | Table `permission_types` tidak ada entry `rw.*` |
| `transactions.rw_id` | ❌ Tidak ada | Migration 009 dan 027 tidak menambahkan kolom ini |
| `fund_locations.rw_id` | ❌ Tidak ada | Migration 034 tidak menambahkan kolom ini |

### Layer Backend/API

| Komponen | Status | Bukti |
|---|---|---|
| Admin CRUD RW | ✅ Sudah ada | `backend/internal/rt/rw_membership.go` — `ListRWMemberships()`, `SetRWJabatan()`, `RemoveRWJabatan()` |
| `/me` return RWID/RWName | ⚠️ Partial | `backend/internal/auth/handlers.go` — hanya untuk SUPER_ADMIN, dari JOIN RT→RW (bukan dari `user_rw_memberships`) |
| JWT claims punya RW | ❌ Tidak ada | `backend/internal/auth/jwt.go` `TokenClaims`: 0 field RW |
| Login query RW membership | ❌ Tidak ada | `backend/internal/auth/service.go` `Login()`: 0 query ke `user_rw_memberships` |
| Permission resolver support RW | ❌ Tidak ada | `backend/internal/auth/permission.go`: 0 referensi RW |
| Finance handler RW scope | ❌ Tidak ada | `backend/internal/finance/`: 0 referensi `rw_id` |
| Fund location handler RW scope | ❌ Tidak ada | `backend/internal/fund_location/`: 0 referensi `rw_id` |
| Endpoint operasional RW | ❌ Tidak ada | Tidak ada route `rw/beranda`, `rw/kas`, `rw/lokasi-dana` |
| Super Admin scope validation | ❌ Tidak ada | `permission.go` always return true untuk SA; hardcoded UUID di handler |
| Test otorisasi RW | ❌ Tidak ada | 0 test untuk RW permission atau scope |

### Layer Web Frontend

| Komponen | Status | Bukti |
|---|---|---|
| Admin CRUD RW UI | ✅ Sudah ada | `frontend/src/features/rt/RwList.tsx`, `RwDetailModal.tsx` |
| Sidebar menu RW operasional | ❌ Tidak ada | `frontend/src/components/Sidebar.tsx`: hanya "Kelola RW" (admin) |
| Route RW operasional | ❌ Tidak ada | `frontend/src/app/App.tsx`: `path="rw"` → `RwList` (admin) |
| API call operasional RW | ❌ Tidak ada | `frontend/src/app/api.ts`: hanya `apiListRWs`, `apiCreateRW`, `apiUpdateRW`, `apiDeactivateRW` |
| AuthContext punya RW data | ❌ Tidak ada | `frontend/src/app/AuthContext.tsx` `AuthUser`: field `rt` tapi tidak ada `rw` |
| Halaman Beranda RW | ❌ Tidak ada | — |
| Halaman KAS RW | ❌ Tidak ada | — |
| Halaman Lokasi Dana RW | ❌ Tidak ada | — |

### Layer Android (Flutter)

| Komponen | Status | Bukti |
|---|---|---|
| Admin RW UI | ✅ Sudah ada | `mobile/lib/features/rw/rw_list_screen.dart` |
| Bottom nav sliding horizontal | ❌ Tidak ada | `mobile/lib/app/navigation_shell.dart`: 7 fixed tabs |
| Menu RW operasional | ❌ Tidak ada | Tab: `_homeTab, _kasTab, _lokasiTab, _iuranTab, _wargaTab, _profileTab, _notifikasiTab` |
| Role model support RW | ❌ Tidak ada | `mobile/lib/core/models/role.dart`: enum `AppRole` tidak punya role RW |
| Halaman Beranda RW | ❌ Tidak ada | — |
| Halaman KAS RW | ❌ Tidak ada | — |
| Halaman Lokasi Dana RW | ❌ Tidak ada | — |
| Dinamische menu per jabatan ganda | ❌ Tidak ada | — |

---

## 5. Rencana Implementasi Minimum (PR-by-PR)

### PR 1 — Database Foundation (Migration 041 + 042)

**Dependensi:** Tidak ada (hanya migration)

**File yang diubah:**
- `backend/migrations/041_rw_permission_types.up.sql` — INSERT permission types `rw.finance.read`, `rw.finance.create`, `rw.finance.approve`, `rw.fund.read`, `rw.fund.create`, `rw.fund.transfer`
- `backend/migrations/041_rw_permission_types.down.sql` — DELETE permission types
- `backend/migrations/042_rw_position_permissions.up.sql` — INSERT `position_permissions` untuk `ketua_rw`, `wakil_ketua_rw`, `bendahara_rw`
- `backend/migrations/042_rw_position_permissions.down.sql` — DELETE position_permissions entries

**Test:**
```sql
SELECT * FROM permission_types WHERE code LIKE 'rw.%'; -- harus ada 6 baris
SELECT * FROM position_permissions WHERE position LIKE '%_rw'; -- harus ada 3 baris
```

**Kriteria penerimaan:** Migration up/down berjalan tanpa error. Query di atas mengembalikan data yang diharapkan.

### PR 2 — JWT Extension (Token Claims + AuthContext)

**Dependensi:** PR 1 (untuk memastikan permission types ada)

**File yang diubah:**
- `backend/internal/auth/models.go` — tambah field `RWID`, `RWRole` ke `TokenClaims` dan `AuthContext`
- `backend/internal/auth/jwt.go` — `GenerateAccessToken()` inject field baru; `GenerateRefreshToken()` tidak perlu diubah
- `backend/internal/auth/middleware.go` — `RequireAuth()` extract `RWID`/`RWRole` dari JWT ke `AuthContext`
- `backend/internal/auth/service.go` — `Login()`: query tambahan `user_rw_memberships`; `Refresh()`: inject field baru; `GetProfile()`: query dan return RW membership untuk user biasa

**Test:**
- Unit test: `TokenClaims` serialization/deserialization dengan field RW
- Integration test: Login user RT-only → token tidak punya field RW
- Integration test: Login user RT+RW → token punya `RWID` dan `RWRole`
- Integration test: Refresh token dengan user RW → token baru punya field RW
- Integration test: `/me` untuk user RT-only → `rw: null`; user RT+RW → `rw: {id, role}`

**Kriteria penerimaan:** Token generation test pass. Integration test login+refresh+me pass.

### PR 3 — Permission Resolver Extension

**Dependensi:** PR 1, PR 2

**File yang diubah:**
- `backend/internal/auth/permission.go` — `HasPermission()` cek `ac.RWRole` → `position_permissions.position` jika ada
- `backend/internal/auth/handlers.go` — `handleMe()`: query `user_rw_memberships` untuk semua user (tidak hanya SUPER_ADMIN)

**Test:**
- Unit test: `hasPermission(ketua_rw, rw.finance.read)` → true
- Unit test: `hasPermission(warga, rw.finance.read)` → false
- Unit test: `hasPermission(ketua_rw, rw.fund.transfer)` → true (juga cek RT permission untuk jabatan RT)
- Integration test: request ke endpoint RW tanpa permission → 403

**Kriteria penerimaan:** Semua test pass. Tidak ada regresi permission RT existing.

### PR 4 — Finance/Fund Location RW Scoping (Backend)

**Dependensi:** PR 2, PR 3

**File yang diubah:**
- `backend/internal/finance/handler.go` — tambah route `/rw/:rwId/beranda`, `/rw/:rwId/kas`; query agregasi `rt_id IN (SELECT id FROM rts WHERE rw_id = ?)`
- `backend/internal/finance/service.go` — tambah method `GetDashboardRW(rwId)` dan `GetKasRW(rwId)`
- `backend/internal/finance/repository.go` — tambah query aggregate untuk RW scope
- `backend/internal/fund_location/handler.go` — tambah route `/rw/:rwId/lokasi-dana`; query agregasi
- `backend/internal/fund_location/service.go` — tambah method `GetLocationsRW(rwId)`
- `backend/internal/fund_location/repository.go` — tambah query agregasi

**Test:**
- Integration test: user ketua_rw akses `/rw/{rwId}/beranda` → 200, data agregasi dari semua RT di bawah RW
- Integration test: user ketua_rw akses `/rw/{rwId}/kas` → 200
- Integration test: user ketua_rw akses `/rw/{rwId}/lokasi-dana` → 200
- Integration test: user perangkat RT akses `/rw/{rwId}/beranda` → 200 baca-only
- Integration test: user perangkat RT akses `/rw/{rwId}/lokasi-dana` → 403
- Integration test: user perangkat RT POST transaksi RW → 403
- Integration test: user RT akses `/rw/{rwId}` di mana RT bukan bagian dari RW → 403
- Integration test: saldo agregasi RT→RW konsisten dengan sum of all RT transactions

**Kriteria penerimaan:** Semua test pass. Saldo agregasi sama dengan sum of individual RT saldo. Tidak ada regresi endpoint RT existing.

### PR 5 — Super Admin Scope Validation

**Dependensi:** PR 2

**File yang diubah:**
- `backend/internal/auth/permission.go` — `HasPermission()`: SA butuh validasi `ScopeID`
- `backend/internal/auth/middleware.go` — validasi `X-RT-ID` header terhadap authorized scope
- `backend/internal/auth/service.go` — `Login()`: ganti hardcoded UUID dengan database-driven scope
- `backend/internal/auth/handlers.go` — `handleMe()`: ganti hardcoded UUID dengan config/database lookup

**Test:**
- Integration test: SA akses RT 002 → berhasil
- Integration test: SA akses RW 016 → berhasil
- Integration test: SA akses wilayah luar → 403
- Integration test: SA POST transaksi → 403 jika tidak punya permission scope

**Kriteria penerimaan:** SA hanya bisa akses wilayah yang didefinisikan. Hardcoded string dihapus.

### PR 6 — Web Frontend: Menu, Route, dan Halaman RW

**Dependensi:** PR 4 (API endpoint harus ada)

**File yang diubah:**
- `frontend/src/app/api.ts` — tambah `apiGetRWDashboard(rwId)`, `apiGetRWKas(rwId)`, `apiGetRWLocations(rwId)`
- `frontend/src/app/AuthContext.tsx` — tambah field `rw` ke `AuthUser` interface dan context value
- `frontend/src/components/Sidebar.tsx` — tambah menu `Beranda RW`, `KAS RW`, `Lokasi Dana RW` (conditional per role)
- `frontend/src/app/App.tsx` — tambah route `/rw-home`, `/rw-kas`, `/rw-lokasi-dana`
- `frontend/src/features/rw/RwHome.tsx` — halaman Beranda RW (baru)
- `frontend/src/features/rw/RwKas.tsx` — halaman KAS RW (baru)
- `frontend/src/features/rw/RwLokasiDana.tsx` — halaman Lokasi Dana RW (baru)
- `frontend/src/features/rw/RwHome.test.tsx` — test komponen

**Test:**
- Unit test: Sidebar render menu RW untuk user ketua_rw
- Unit test: Sidebar tidak render menu RW untuk user warga
- E2E test: user perangkat RT buka `/rw-kas` → baca mode, bisa melihat data
- E2E test: user perangkat RT buka `/rw-lokasi-dana` → 403
- E2E test: user ketua_rw buka `/rw-beranda` → full access

**Kriteria penerimaan:** Semua test pass. Menu RT tidak berubah. Menu RW muncul sesuai role.

### PR 7 — Android: Horizontal Sliding + Menu RW

**Dependensi:** PR 4 (API endpoint harus ada), PR 6 (konsep menu harus ada di Web sebagai referensi)

**File yang diubah:**
- `mobile/lib/app/navigation_shell.dart` — ganti fixed `TabBar` dengan `TabBar` horizontal sliding; tambah 3 tab RW
- `mobile/lib/core/models/role.dart` — tambah `AppRole.ketuaRw`, `AppRole.wakilKetuaRw`, `AppRole.bendaharaRw`
- `mobile/lib/features/rw/rw_home_screen.dart` — halaman Beranda RW (baru)
- `mobile/lib/features/rw/rw_kas_screen.dart` — halaman KAS RW (baru)
- `mobile/lib/features/rw/rw_lokasi_dana_screen.dart` — halaman Lokasi Dana RW (baru)
- `mobile/lib/features/rw/rw_repository.dart` — tambah method API RW operasional

**Test:**
- Widget test: Bottom nav rendering dengan 10 tabs → horizontal sliding berfungsi
- Widget test: Tab RW hanya tampil untuk role yang diizinkan
- Integration test: buka halaman RW → fetch data dari API, render benar
- UI test (physical device): horizontal sliding nyaman di layar 5"
- Regression test: pengguna single-role tidak melihat perubahan navigasi

**Kriteria penerimaan:** Sliding horizontal berfungsi. Menu RW tampil sesuai role. Tidak ada regresi navigasi RT.

### PR 8 — Integration & Regression Tests

**Dependensi:** Semua PR sebelumnya

**Test yang harus dijalankan:**
1. Semua test scenario dari `docs/rbac-operasional-rw.md` section 14
2. Regression test fitur RT existing (semua test finance/fund_location yang ada)
3. Cross-tenant isolation test
4. Jabatan ganda test
5. Android sliding + menu test pada emulator + physical device
6. `flutter analyze` — 0 error, 0 warning
7. `git diff --check` — pass

**Kriteria penerimaan:** Semua test pass. Tidak ada error/warning dari analyzer.

---

## 6. Lima Blocker Utama dan Tindakan Penyelesaian

### Blocker 1: Tidak Ada JWT Context RW

**Deskripsi:** Token JWT tidak menyertakan informasi RW sama sekali. Tanpa ini, tidak mungkin melakukan otorisasi berbasis jabatan RW di backend.

**Aksi:** PR 2 — Tambah field `RWID`, `RWRole` ke `TokenClaims` dan `AuthContext`, update login/refresh untuk query dan inject data RW.

**Estimasi:** 2-3 hari

### Blocker 2: Tidak Ada Permission Types dan Position untuk RW

**Deskripsi:** `permission_types` tidak punya entry `rw.*` dan `position_permissions` tidak punya entry jabatan RW. Permission resolver tidak bisa bekerja untuk operasional RW.

**Aksi:** PR 1 — Migration 041 (permission types) dan 042 (position_permissions).

**Estimasi:** 1 hari

### Blocker 3: Tidak Ada Endpoint Operasional RW

**Deskripsi:** Finance dan fund location handler 100% RT-scoped. Tidak ada endpoint `/rw/{rwId}/...` untuk beranda, kas, atau lokasi dana.

**Aksi:** PR 4 — Tambah handler, service, repository methods dengan query agregasi RT→RW.

**Estimasi:** 3-4 hari

### Blocker 4: Frontend Tidak Punya Menu/Route/API RW

**Deskripsi:** Web hanya punya admin CRUD RW. Android hanya punya admin RW. Tidak ada satu pun halaman operasional RW di platform manapun.

**Aksi:** PR 6 (Web) dan PR 7 (Android) — tambah menu, route, halaman, API calls.

**Estimasi:** 4-5 hari (Web 2-3 hari, Android 2-3 hari, bisa paralel)

### Blocker 5: Super Admin Scope Validasi Tidak Ada

**Deskripsi:** SUPER_ADMIN `return true, nil` tanpa scope check. Hardcoded UUID di beberapa tempat. Risiko kebocoran data lintas tenant.

**Aksi:** PR 5 — Ganti hardcoded dengan database-driven scope config, tambahkan validasi di middleware dan permission resolver.

**Estimasi:** 1-2 hari

---

## 7. Risiko dan Potensi Regresi Fitur RT

| Risiko | Tingkat | Mitigasi |
|---|---|---|
| Token JWT lebih besar (~100 bytes) | Rendah | Tidak ada dampak signifikan pada performance |
| Query agregasi RT→RW lambat di RW besar | Sedang | Tambah composite index `rts(rw_id, is_active)`, pertimbangkan caching |
| Menu RW bertabrakan dengan menu RT di UI | Sedang | Gunakan suffix "RW" pada nama menu, pastikan tidak ada duplikasi |
| Horizontal sliding mengubah UX existing | Rendah | Test pada emulator + physical device, pertahankan 7 tab RT tetap berfungsi |
| Permission resolver change regresi RT | Sedang | Semua test RT existing harus tetap pass |
| SUPER Admin scope validation mengubah akses existing | Tinggi | Test exhaustive: SA akses RT 002, SA akses RW 016, SA akses wilayah lain |

---

## 8. Kesimpulan dan Status Akhir

### Ringkasan Kesiapan

| Aspek | Kesiapan |
|---|---|
| Database schema (rws, user_rw_memberships, user_rw_membership_history) | ✅ 100% — migration 027-029 lengkap |
| Admin RW endpoints | ✅ 100% — `rw_membership.go` lengkap |
| JWT & Auth context | ❌ 0% — tidak ada field RW |
| Permission resolver | ❌ 0% — tidak ada RW permission types atau position entries |
| Finance/Fund location RW scoping | ❌ 0% — semua handler RT-only |
| Web frontend operasional RW | ❌ 0% — hanya admin CRUD |
| Android navigasi & menu RW | ❌ 0% — fixed 7 tab RT-only |
| Super Admin scope validation | ❌ 0% — hardcoded + always-true |

**Implementasi keseluruhan: ~15% selesai (database admin + schema).**

### Status Akhir

**`SIAP IMPLEMENTASI BERTAHAP`**

Semua ketidakpastian teknis telah diverifikasi. Dokumen ini berisi matriks gap lengkap, keputusan arsitektur dengan bukti, rencana implementasi per PR dengan dependensi dan test, lima blocker utama dengan tindakan spesifik, dan analisis risiko.

**Syarat lanjut:**
- Produk harus memutuskan model data KAS/Lokasi Dana RW (agregasi vs mandiri) — rekomendasi: agregasi
- Migration permission types dan position_permissions harus disetujui produk (daftar permission yang diperlukan untuk jabatan RW)
- Tidak ada keputusan produk yang menghalangi implementasi langkah demi langkah sesuai PR-by-PR di atas.

---

*Akhir laporan final.*
