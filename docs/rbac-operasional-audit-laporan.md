# Laporan Akhir Audit RBAC Operasional RW

**Status:** FINAL — Hasil Audit Lengkap  
**Branch:** `urena` — HEAD `ac266c0`  
**Dokumen Acuan:** `docs/rbac-operasional-rw.md`  
**Jenis:** READ-ONLY — Tidak ada kode diubah  
**Tanggal:** 2026-10-29

---

## 1. Ringkasan Kesiapan Arsitektur

| Aspek | Status | Ringkasan |
|---|---|---|
| Database schema RW | ✅ LENGKAP | 3 migration (027, 028, 029) membuat tabel `rws`, `user_rw_memberships`, `user_rw_membership_history` dengan foreign key, unique constraint, dan projection trigger |
| Admin endpoints RW | ✅ LENGKAP | `rw_membership.go` implement CRUD jabatan RW (list, set, remove) |
| `/me` endpoint | ⚠️ PARSIAL | Mengembalikan `RWID`/`RWName` untuk SUPER_ADMIN dari JOIN RT→RW, tapi tidak mengembalikan membership RW untuk pengguna biasa |
| JWT token claims | ❌ BELUM ADA | `TokenClaims` tidak memiliki `RWID`, `RWRole`, atau `RWJabatan` |
| Auth service login | ❌ BELUM ADA | `Login()` hanya query `user_rt_memberships`, tidak query `user_rw_memberships` |
| Permission resolver | ❌ BELUM ADA | Tidak ada resolusi permission untuk jabatan RW (`ketua_rw`, `wakil_ketua_rw`, `bendahara_rw`) |
| `position_permissions` RW | ❌ BELUM ADA | Tidak ada migrasi atau baris tabel untuk jabatan RW |
| Finance/Fund location RW scope | ❌ BELUM ADA | Semua handler dan service murni RT-scoped, tidak ada `rw_id` |
| Web frontend menu RW | ❌ BELUM ADA | Hanya ada menu "Kelola RW" (admin), tidak ada menu operasional RW |
| Android navigation | ❌ BELUM ADA | Hardcoded 7 tab RT-only, tidak ada sliding horizontal, tidak ada menu RW |
| Transactions table RW scope | ⚠️ BELUM JELAS | Tabel `transactions` memiliki `rt_id` tapi tidak ada `rw_id` |

**Kesimpulan umum:** Lapisan database dan admin admin RW sudah ada. Namun untuk operasional RW (Beranda RW, KAS RW, Lokasi Dana RW), **tidak ada satu pun komponen operasional yang tersedia di backend, frontend, atau Android**. Semua harus dibangun dari awal, dengan fondasi autentikasi dan otorisasi yang perlu diperluas terlebih dahulu.

---

## 2. Temuan Skema RW pada Branch Utama

### Migration yang Terverifikasi Ada

| File | Deskripsi |
|---|---|
| `027_rw_domain_and_transaction_reference.up.sql` | Membuat tabel `rws`, tambahkan kolom `rw_id` pada `rts` dengan FK ke `rws(id)`, backfill RW dari RT existing |
| `028_communities_rw_scope.up.sql` | Membuat tabel `communities`, tambahkan `community_id` ke `rws` dan `rts`, composite FK constraint, trigger `enforce_rt_rw_projection()` |
| `029_rw_membership_and_jabatan.up.sql` | Membuat `user_rw_memberships` (user→RW role/jabatan), `user_rw_membership_history` (audit trail), unique constraint `(user_id, rw_id)`, FK cascade |

### Konsistensi Kode dan Skema

- `backend/internal/rt/rw_membership.go` menggunakan skema yang benar (query `user_rw_memberships`, `rws`, validasi `AllowedRWJabatans = {ketua, wakil_ketua, sekretaris, bendahara}`)
- Tidak ada dependensi pada worktree sekunder — seluruh kode dan migration berada di branch `urena`
- `backend/internal/auth/handlers.go` untuk `/me` endpoint menggunakan JOIN `rts r LEFT JOIN rws w ON w.id = r.rw_id` untuk menampilkan informasi RW yang dimiliki RT

### Hardcoded UUID Found

Di `auth/handlers.go` ditemukan hardcoded UUID `'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b'` dan hardcoded string `'urena'` dalam query RT selection untuk SUPER_ADMIN. Ini adalah implementasi bypass yang perlu dikaji ulang pada tahap implementasi.

---

## 3. Rekomendasi Kontrak Autentikasi dan Membership

### Masalah Utama

1. **JWT tidak menyertakan context RW** — `TokenClaims` di `auth/models.go` hanya punya `UserID`, `MID`, `RTID`, `Role`, `Jabatan`, `SysRole`. Tidak ada `RWID`, `RWRole`, atau `RWJabatan`.

2. **Login tidak query `user_rw_memberships`** — `auth/service.go` `Login()` hanya mengambil membership RT. Pengguna dengan jabatan RW tidak pernah mendapatkan context RW di token.

3. **Permission resolver tidak resolve RW** — `permission.go` `HasPermission()` hanya cek `user_rt_memberships` dan `position_permissions` berdasarkan jabatan RT.

### Desain Minimum yang Direkomendasikan

#### Opsi A: Satu Token dengan Ekstensi Claims (Disarankan)

Tambahkan field RW ke `TokenClaims`:
```go
type TokenClaims struct {
    UserID  string
    MID     string
    RTID    string
    Role    string
    Jabatan string
    SysRole string
    RWID    string  // NEW
    RWRole  string  // NEW
}
```

**Keuntungan:**
- Satu token, satu session
- Backend bisa langsung cek authorization tanpa query tambahan
- Kompatibel dengan refresh token existing (hanya perlu tambahkan field saat generate)
- Login flow minim perubahan: query tambahan `user_rw_memberships` + inject ke claims

**Dampak:**
- Login: tambahkan query `SELECT rw_id, role, jabatan FROM user_rw_memberships WHERE user_id = ? AND is_active = true`
- Generate token: tambahkan field `RWID` dan `RWRole` ke `TokenClaims`
- Refresh token: inject field baru saat regenerate
- Middleware: tambahkan field RW ke `AuthContext`
- Semua handler: validasi permission terhadap jabatan RW (ketua_rw, wakil_ketua_rw, bendahara_rw)

#### Opsi B: Token Pemisahan (Tidak Disarankan)

Pisahkan token untuk RT dan RW. Tidak direkomendasikan karena:
- Membutuhkan dua token per user
- Komplikasi pada refresh token
- Lebih banyak risiko keamanan (token hijacking, session management)

#### Opsi C: Resolusi Membership di Server (Tidak Disarankan)

Token hanya berisi userID, server query membership pada setiap request. Tidak direkomendasikan karena:
- Overhead database pada setiap request
- Tidak kompatibel dengan stateless JWT design existing

---

## 4. Rekomendasi Model Data KAS dan Lokasi Dana RW

### Temuan Faktual

- Tabel `transactions` (migration 009) memiliki kolom `rt_id` (FK ke `rts(id)`), **tidak ada `rw_id`**
- Tabel `fund_locations` (migration 034) memiliki kolom `rt_id`, **tidak ada `rw_id`**
- Migration 027 menambahkan kolom `transaction_reference` pada `transactions` tetapi **tidak** menambahkan `rw_id`
- `rws.rw_id` yang ditambahkan di 027 merujuk pada kolom dalam `rts` (relasi RT→RW), bukan kolom di `rws`

### Rekomendasi: Agregasi Data dari RT (Disarankan)

Karena semua transaksi dan lokasi dana saat ini tertaut ke RT, dan relasi RT→RW sudah ada di `rts.rw_id`, maka:

**KAS RW** = agregasi dari semua transaksi RT di bawah RW tersebut. Query:
```sql
SELECT SUM(amount) FROM transactions 
WHERE rt_id IN (SELECT id FROM rts WHERE rw_id = ? AND is_active = true)
```

**Lokasi Dana RW** = agregasi dari semua lokasi dana RT di bawah RW.

**Keuntungan:**
- Tidak perlu migration database besar
- Tidak mengubah struktur transaksi yang sudah mapan
- Data tidak diduplikasi (sumber kebenaran tetap transaksi RT)
- Kompatibel dengan audit trail existing
- Pemisahan konsep antara "data RW" (agregasi) dan "data RT" (individual)

**Dampak pada handler finance/fund_location:**
- Tambah parameter `rw_id` yang di-resolve dari auth context
- Query tambahkan `WHERE rt_id IN (SELECT id FROM rts WHERE rw_id = ?)`
- Backend perlu memvalidasi bahwa user memiliki jabatan RW yang sah sebelum mengembalikan data

### Model Alternatif: Transaksi RW Sendiri (Tidak Disarankan Saat Ini)

Membuat tabel `rw_transactions` atau menambahkan `rw_id` ke `transactions` akan:
- Membutuhkan migration besar
- Menambah kompleksitas audit trail
- Berpotensi menimbulkan duplikasi data
- Tidak disebutkan sebagai kebutuhan di `docs/rbac-operasional-rw.md`

---

## 5. Rekomendasi Pembatasan Super Admin

### Temuan Tembusan Keamanan

1. **Hardcoded UUID bypass** — `auth/handlers.go` menggunakan hardcoded `'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b'` untuk memilih RT SUPER_ADMIN. Tidak ada validasi terhadap database.

2. **Super Admin selalu return true** — `permission.go` `HasPermission()` return `true, nil` untuk semua permission jika `SystemRole == SuperAdmin`, tanpa memeriksa scope wilayah.

3. **RT ID hardcode di login** — `auth/service.go` `Login()` hardcode UUID yang sama untuk prioritas RT SUPER_ADMIN.

### Rekomendasi Minimum

1. **AuthContext perlu `ScopeID`** — tambahkan field `ScopeID` yang berisi RT atau RW ID yang diautorisasi, bukan hardcoded.

2. **Permission resolver perlu scope validation** — SUPER_ADMIN harus divalidasi terhadap daftar wilayah yang diizinkan (RT 002, RW 016) sebelum memberikan akses penuh.

3. **Header `X-RT-ID` perlu divalidasi** — verifikasi bahwa RT yang diminta milik wilayah yang otorisasi SUPER_ADMIN.

4. **Jangan hapus mekanisme existing** — SUPER_ADMIN mungkin butuh akses admin (kelola RT, pengguna, dll). Pembatasan wilayah hanya berlaku untuk operasional RW.

---

## 6. Kepastian Lokasi Proyek Web

**Web menggunakan React/Vite terpisah** di direktori `frontend/`, bukan Flutter Web dari `mobile/`.

Bukti:
- `frontend/package.json` ada (Vite React project)
- `frontend/src/app/App.tsx` — routing React
- `frontend/src/components/Sidebar.tsx` — sidebar Web
- Tidak ada file Flutter Web di `web/` atau `frontend/web/`
- Proyek `mobile/` adalah Flutter native Android (bukan Flutter Web)

**Kesimpulan:** Fitur operasional RW untuk Web harus diimplementasikan di `frontend/` (React), bukan di `mobile/` (Flutter Android).

---

## 7. Matriks Keputusan Arsitektur

| Pertanyaan | Bukti Aktual | Keputusan Direkomendasikan | Dampak pada RT Existing | File/Component Perlu Diubah | Test Wajib | Status |
|---|---|---|---|---|---|---|
| **JWT claims perlu ekstensi?** | `TokenClaims` tidak punya field RW | Ya, tambahkan `RWID`, `RWRole` | Tidak ada — hanya menambah field | `auth/models.go`, `auth/jwt.go`, `auth/service.go`, `auth/middleware.go`, `auth/permission.go` | Unit test token generation, integration test login + RW context | Terjawab |
| **Login perlu query RW membership?** | `Login()` tidak query `user_rw_memberships` | Ya, query setelah RT membership | Tidak ada — hanya menambah query | `auth/service.go` | Integration test: user RT+RW login → token punya kedua context | Terjawab |
| **Permission resolver perlu RW support?** | Tidak ada RW position di `position_permissions` | Ya, tambahkan RW position entries + logic resolusi | Tidak ada — hanya menambah kondisi | `auth/permission.go`, migration baru (042) | Unit test: hasPermission ketua_rw → true, warga → false | Terjawab |
| **Finance/Fund handler perlu RW scope?** | Semua handler RT-scoped | Ya, tambahkan parameter `rw_id` + scope validation | Tidak ada — hanya menambah path baru | `finance/handler.go`, `finance/service.go`, `fund_location/handler.go` | Integration test: akses RW endpoint → scope validasi | Terjawab |
| **Transactions table perlu rw_id?** | Hanya punya `rt_id` | Tidak — gunakan agregasi dari RT melalui relasi `rts.rw_id` | Tidak ada — tidak mengubah transaksi existing | Hanya handler query, bukan migration | Integration test: agregasi transaksi RT → KAS RW benar | Terjawab |
| **Web perlu menu RW?** | Sidebar hanya punya "Kelola RW" | Ya, tambah Beranda RW, KAS RW, Lokasi Dana RW | Tidak ada — hanya menambah menu | `frontend/src/components/Sidebar.tsx`, routing, halaman baru | E2E test: user RT membuka KAS RW → baca mode | Terjawab |
| **Android perlu sliding nav + menu RW?** | Hardcoded 7 tab | Ya, tambah horizontal sliding + 3 menu RW | Tidak ada — hanya menambah menu | `mobile/lib/app/navigation_shell.dart`, `mobile/lib/core/models/role.dart` | UI test: sliding horizontal, menu RW tampil untuk role sah | Terjawab |
| **Super Admin scope validasi?** | Hardcoded UUID + always-true permission | Ya, tambahkan scope validation | Tidak ada — hanya membatasi akses | `auth/middleware.go`, `auth/permission.go`, `auth/service.go` | Integration test: SA akses wilayah luar → ditolak | Terjawab |
| **Cross-scope read-only access?** | Tidak ada | Ya, validasi di handler bahwa read-only tidak diizinkan untuk write | Tidak ada — hanya menambah validasi | Semua handler finance + fund_location | Integration test: perangkat RT POST transaksi RW → ditolak | Terjawab |

---

## 8. Daftar Keputusan yang Butuh Persetujuan Pemilik Produk

| # | Keputusan | Pilihan | Rekomendasi | Risiko Jika Tidak Dikonfirmasi |
|---|---|---|---|---|
| 1 | **Model data KAS/Lokasi Dana RW** | Agregasi dari RT vs tabel mandiri | Agregasi dari RT (tidak ada migration baru) | Jika pemilik ingin transaksi RW mandiri, perlu migration besar yang mengubah kontrak API |
| 2 | **RW jabatan yang diizinkan** | `{ketua, wakil_ketua, sekretaris, bendahara}` vs subset | Gunakan semua 4 jabatan dari `AllowedRWJabatans` existing | Jika hanya {ketua, wakil_ketua} yang boleh akses operasional, bendahara_rw perlu dibatasi |
| 3 | **Cross-scope read-only: arah akses** | Perangkat RT→RW, Perangkat RW→RT, atau keduanya? | Keduanya (sesuai dokumen acuan) | Jika hanya satu arah, perlu penyesuaian matriks permission |
| 4 | **Scope Super Admin** | Tetap RT 002 + RW 016 vs dinamis dari config | Tetap RT 002 + RW 016, tapi tanpa hardcoded UUID — gunakan database lookup | Hardcoded UUID tidak scalable dan tidak aman |
| 5 | **JWT RW token lifetime** | Sama dengan RT token (15 menit) vs berbeda? | Sama — satu token untuk RT+RW | Jika lifetime berbeda, perlu logic refresh token yang lebih kompleks |

---

## 9. Matriks Gap: Existing vs RBAC Requirements

### Layer Database

| Ketentuan RBAC | Existing | Gap | Prioritas |
|---|---|---|---|
| Tabel `rws` | ✅ Ada (migration 027) | — | — |
| Relasi RT→RW | ✅ Ada (`rts.rw_id`) | — | — |
| Tabel `user_rw_memberships` | ✅ Ada (migration 029) | — | — |
| Tabel `user_rw_membership_history` | ✅ Ada (migration 029) | — | — |
| RW permission di `position_permissions` | ❌ Tidak ada | Butuh migration baru | **KRITIS** |
| `transactions.rw_id` | ❌ Tidak ada | Gunakan agregasi RT | — |
| `fund_locations.rw_id` | ❌ Tidak ada | Gunakan agregasi RT | — |

### Layer Backend/API

| Ketentuan RBAC | Existing | Gap | Prioritas |
|---|---|---|---|
| JWT punya RW claims | ❌ Tidak ada | Tambah `RWID`, `RWRole` | **KRITIS** |
| Login query RW membership | ❌ Tidak ada | Tambah query `user_rw_memberships` | **KRITIS** |
| Permission resolver support RW | ❌ Tidak ada | Tambah resolusi jabatan RW | **KRITIS** |
| Finance handler RW scope | ❌ Tidak ada | Tambah validasi scope RW | **KRITIS** |
| Fund location handler RW scope | ❌ Tidak ada | Tambah validasi scope RW | **KRITIS** |
| Admin CRUD RW | ✅ Ada (`rw_membership.go`) | — | — |
| `/me` return RW membership | ⚠️ Hanya display, bukan access control | Perlu juga untuk member users | **TINGGI** |
| Super Admin scope validation | ❌ Hardcoded UUID + always-true | Ganti dengan DB-validated scope | **KRITIS** |

### Layer Web Frontend

| Ketentuan RBAC | Existing | Gap | Prioritas |
|---|---|---|---|
| Menu Beranda RW | ❌ Tidak ada | Buat halaman baru | **TINGGI** |
| Menu KAS RW | ❌ Tidak ada | Buat halaman baru | **TINGGI** |
| Menu Lokasi Dana RW | ❌ Tidak ada | Buat halaman baru | **TINGGI** |
| Sidebar dinamis (gabungan RT+RW) | ❌ Tidak ada | Buat logic merge menu | **TINGGI** |
| AuthContext punya RW data | ❌ Tidak ada | Perlu `apiMe` response + context update | **TINGGI** |
| API calls untuk RW endpoints | ❌ Tidak ada (hanya admin CRUD) | Butuh API untuk aggregate RW data | **TINGGI** |

### Layer Android (Flutter)

| Ketentuan RBAC | Existing | Gap | Prioritas |
|---|---|---|---|
| Horizontal sliding nav | ❌ Hardcoded 7 tab | Modifikasi `navigation_shell.dart` | **TINGGI** |
| Menu Beranda RW | ❌ Tidak ada | Tambah halaman/tab baru | **TINGGI** |
| Menu KAS RW | ❌ Tidak ada | Tambah halaman/tab baru | **TINGGI** |
| Menu Lokasi Dana RW | ❌ Tidak ada | Tambah halaman/tab baru | **TINGGI** |
| Role model support RW | ❌ `AppRole` tidak punya RW | Tambah role RW di `role.dart` | **TINGGI** |
| Dinamische menu per user | ❌ Fixed 7 tab | Logic filter menu per role/jabatan | **TINGGI** |

---

## 10. Risk Assessment per Layer

### Backend/API — RISK: KRITIS

| Risiko | Deskripsi | Mitigasi |
|---|---|---|
| Token claims backward compat | Menambah field JWT bisa break client yang tidak expect field baru | Field baru optional di parser, existing client tidak terpengaruh |
| Permission resolver change | Jika SUPER_ADMIN always-true tidak diubah, semua endpoint bisa di-bypass | Wajib ganti logic `HasPermission()` — unit test wajib |
| Hardcoded UUID | Hardcoded RT UUID di handler dan service | Ganti dengan validasi database + config |
| Missing RW endpoints | Tidak ada endpoint RW sama sekali | Buat endpoint baru, jangan modifikasi endpoint RT existing |

### Database — RISK: RENDAH

| Risiko | Deskripsi | Mitigasi |
|---|---|---|
| Agregasi performa | Query agregasi RT→RW bisa lambat jika RT banyak | Tambah composite index pada `rts.rw_id`, caching layer |
| Migration baru position_permissions | Perlu insert RW jabatan ke `position_permissions` | Buat migration 042 yang idempotent |

### Web Frontend — RISK: SEDANG

| Risiko | Deskripsi | Mitigasi |
|---|---|---|
| Menu overlap | Menu RT dan RW bisa muncul ganda untuk user jabatan ganda | Logic filter perlu cermat: RT menu untuk jabatan RT, RW menu untuk jabatan RW |
| API contract change | `/me` response perlu tambah field RW membership | Update `ApiUser` interface, pastikan backward compat |

### Android — RISK: SEDANG

| Risiko | Deskripsi | Mitigasi |
|---|---|---|
| Regresi navigasi | Perubahan bottom nav bisa rusak navigasi existing | Test pada emulator + physical device, pastikan 7 tab lama tetap berfungsi |
| Menu overflow | Lebih banyak menu dari lebar layar | Horizontal sliding sebagai solusi, bukan menampilkan semua sekaligus |
| Role model breaking | Ubah `AppRole` enum bisa break existing code | Tambah role baru di enum end, tidak ubah yang existing |

---

## 11. Urutan Implementasi Minimum (PR-by-PR)

Setiap PR harus lulus review, test backend, dan `git diff --check` sebelum lanjut ke PR berikutnya.

### PR 1 — Database Foundation (No code change, migration only)
- Migration 042: Tambah RW position entries ke `position_permissions`
- Migration 042 down: Hapus entries yang ditambah
- Test: Query `position_permissions` verifikasi entry `{ketua_rw, wakil_ketua_rw, bendahara_rw}` ada

### PR 2 — JWT & Auth Extension
- Tambah `RWID`, `RWRole` ke `TokenClaims` (`auth/models.go`, `auth/jwt.go`)
- Tambah query `user_rw_memberships` di `Login()` (`auth/service.go`)
- Tambah field RW ke `AuthContext` (`auth/middleware.go`)
- Update `Refresh()` untuk inject field RW
- Update `GetProfile()` untuk query dan return RW membership ke client (`auth/service.go`)
- Test: Login user RT+RW → token punya `RWID` + `RWRole`; Login user RT-only → RW fields kosong/nested null

### PR 3 — Permission Resolver Extension
- Update `HasPermission()` untuk cek jabatan RW (`auth/permission.go`)
- Tambah logic: jabatan RW (ketua_rw, dll) → permission RW (rw.finance.read, rw.finance.create, dll)
- Test: `hasPermission(ketua_rw, rw.finance.read)` = true; `hasPermission(warga, rw.finance.read)` = false

### PR 4 — Finance/Fund Location RW Scoping
- Tambah parameter `rw_id` pada finance/fund location handlers
- Query tambahkan `WHERE rt_id IN (SELECT id FROM rts WHERE rw_id = ?)`
- Validasi: user harus punya jabatan RW yang sah untuk akses scope RW
- Validasi: role RT hanya bisa akses scope RT
- Test: User RT mengakses endpoint RW scope RW → ditolak; User ketua_rw mengakses endpoint RW scope RW → berhasil

### PR 5 — Web Frontend: Menu & Routing
- Tambah menu RW di Sidebar (`frontend/src/components/Sidebar.tsx`)
- Tambah route `/rw-home`, `/rw-kas`, `/rw-lokasi-dana`
- Tambah halaman: `RwHome`, `RwKas`, `RwLokasiDana`
- Update `AuthContext.tsx` → tambahkan `rw` field
- Test: E2E user perangkat RT → lihat Beranda RW, KAS RW; user perangkat RT → Lokasi Dana RW ditolak

### PR 6 — Android: Horizontal Sliding + Menu RW
- Modifikasi `navigation_shell.dart` → ganti `TabBar` fixed dengan `TabBar` horizontal sliding
- Tambah menu RW ke navigation
- Update `role.dart` → tambah role RW (ketua_rw, wakil_ketua_rw, bendahara_rw)
- Logic filter menu per role/jabatan
- Test: Sliding horizontal berfungsi, menu RW tampil untuk role sah, user single-role tidak regresi

### PR 7 — Super Admin Scope Validation
- Ganti hardcoded UUID dengan validasi database
- Update `HasPermission()` scope check untuk SUPER_ADMIN
- Validasi header `X-RT-ID` terhadap authorized scope
- Test: SA akses RT 002 + RW 016 → berhasil; SA akses wilayah lain → ditolak

### PR 8 — Integration & Regression Tests
- Semua test skenario dari `docs/rbac-operasional-rw.md` section 14
- Test cross-tenant isolation
- Test jabatan ganda
- Test Android sliding + menu pada emulator + physical device

---

## 12. Kesimpulan

**Kesiapan arsitektur saat ini:** 30% implementasi RW (database schema + admin endpoints). 70% yang tersisa (operational endpoints, JWT extension, permission resolution, frontend, Android) belum ada satu pun baris kode.

**Risiko tertinggi:**
1. JWT tidak menyertakan context RW → seluruh otorisasi downstream gagal
2. SUPER_ADMIN bypass hardcoded → risiko kebocoran data lintas tenant
3. Tidak ada permission resolver untuk jabatan RW → menu RW tampil tapi backend menolak semua request

**Rekomendasi urutan prioritas:**
1. JWT + Auth extension (PR 2) — fondasi untuk semua otorisasi
2. Permission resolver (PR 3) — tanpa ini tidak ada RBAC RW
3. Database position_permissions entries (PR 1) — support PR 2 & 3
4. Finance/Fund location scoping (PR 4) — operasional backend
5. Web frontend (PR 5) — UI layer
6. Android navigation (PR 6) — UI layer
7. Super Admin scope (PR 7) — keamanan
8. Tests (PR 8) — verifikasi

---

*Akhir laporan audit.*
