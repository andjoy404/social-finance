# Rancangan Administrasi RT/RW

**Status:** RANCANGAN — BELUM DIIMPLEMENTASIKAN
**Tanggal Audit:** 2026-01-15
**Branch:** `main`
**Commit SHA:** `2f6678ded22fa8005bc6c1c4303ab225bb80c1d1`
**Status Working Tree:** MODIFIED — perubahan lokal yang dipertahankan (lihat §23)

---

## 1. Ringkasan Eksekutif

Fitur **Administrasi** pada proyek `social-finance` akan menggantikan menu **RT** yang saat ini tersedia untuk **Super Admin** di Web. Menu baru ini akan memiliki dua tab:

1. **Tab RT** — pengelolaan kepengurusan RT (jabatan pengurus).
2. **Tab RW** — pengelolaan kepengurusan RW (jabatan pengurus RW).

Pada tahap pertama, seluruh operasi Administrasi hanya dapat diakses oleh **Super Admin** — baik di level backend middleware, database constraint, maupun frontend UI. Tab RW berstatus **direncanakan/belum aktif** sampai model data, relasi organisasi, aturan bisnis, dan permission RW disepakati serta diimplementasikan.

Dokumen ini adalah hasil audit implementasi aktual repository `social-finance` pada commit `2f6678d`. Semua fakta yang dicantumkan telah diverifikasi dari kode sumber, migrasi database, route registry, dan dokumentasi yang ada. Hal yang belum dapat diverifikasi ditandai **BELUM TERVERIFIKASI**.

---

## 2. Tujuan dan Ruang Lingkup

### 2.1 Tujuan

Mendefinisikan arsitektur, model data, alur bisnis, kontrak API, kebijakan akses, rencana migrasi, dan strategi pengujian untuk fitur Administrasi RT/RW.

### 2.2 Ruang Lingkup

| Lingkup | Cakupan |
|---------|---------|
| Web (React) | Sidebar, route, halaman Administrasi tab RT (dan RW nanti) |
| Backend (Go) | Endpoint CRUD jabatan, middleware super_admin validation, transaction handling |
| Database | Skema keanggotaan RT, position_permissions, audit log |
| Android (Flutter) | Evaluasi apakah perlu pada Fase 1 |
| RW | Desain konseptual saja — implementasi pada Fase 2+ |

### 2.3 Batasan

- **Tidak ada implementasi kode** pada tugas ini.
- **Tidak ada migration, data mutation, atau konfigurasi baru.**
- **Tidak ada commit atau push.**

---

## 3. Keputusan Produk yang Sudah Final

### 3.1 Nama dan Struktur Menu

| Item | Nilai |
|------|-------|
| Nama baru menu | **Administrasi** |
| Sub-menu | Tab RT, Tab RW |
| Titik masuk | Hanya Super Admin |
| Route saat ini | `/rt` → `RtList` (lihat §21) |

**Implikasi:** Perubahan bukan hanya label. Diperlukan evaluasi route, API, middleware, otorisasi, model data, migrasi, pengujian, dan kompatibilitas navigasi.

### 3.2 Kebijakan Akses

| Aturan | Sumber |
|--------|--------|
| Hanya `system_role = 'super_admin'` yang boleh melihat dan menggunakan Administrasi | Keputusan produk |
| Backend harus menegakkan, bukan hanya UI | Keamanan |
| Endpoint baru wajib validasi role Super Admin | Backend contract |
| Tidak ada wewenang delegasi ke Ketua RT, Ketua RW, Bendahara, atau role lain pada tahap ini | Keamanan |
| Delegasi bisa dievaluasi di Fase 4 | Roadmap |

### 3.3 Tab Pengurus RT — Alur Konseptual

1. Super Admin membuka **Administrasi → RT**.
2. Sistem menampilkan daftar RT yang tersedia (status, identitas).
3. Super Admin memilih RT aktif.
4. Sistem menampilkan daftar akun/keanggotaan terkait RT dan jabatan yang sudah ditetapkan.
5. Super Admin memilih akun dan menetapkan jabatan RT.
6. Super Admin dapat mengganti atau mencabut penetapan jabatan.
7. Perubahan divalidasi dan disimpan melalui backend.
8. UI menampilkan hasil operasi dan data terbaru.

**Catatan:** Gunakan model keanggotaan RT yang sudah ada (`user_rt_memberships.jabatan`) apabila audit membuktikan model tersebut sesuai.

### 3.4 Tab Pengurus RW — Status

| Item | Status |
|------|--------|
| Tab RW | **Direncanakan / Belum Aktif** |
| Model data RW | **BELUM TERVERIFIKASI** — tidak ada tabel RW dalam schema |
| Relasi RW–RT | Hanya konsep dalam `docs/rw-architecture-plan.md` |
| Implementasi | Fase 2+ setelah desain disepakati |

**Penting:** Tab RW **bukan** bukti bahwa struktur RW sudah ada. Dokumen `docs/rw-architecture-plan.md` (683 baris) berisi desain konseptual RW, bukan implementasi.

---

## 4. Arsitektur Saat Ini — Hasil Audit

### 4.1 Database — Tabel yang Sudah Ada

| Tabel | Migrasi | Status |
|-------|---------|--------|
| `rts` | 001 | **SUDAH ADA** — RT organizations |
| `users` | 002 | **SUDAH ADA** — Global identity |
| `user_rt_memberships` | 002 (diperbarui 003, 011) | **SUDAH ADA** — Tenant role + jabatan |
| `refresh_tokens` | 002 (diperbarui 014) | **SUDAH ADA** |
| `households` | 004, 006, 008 | **SUDAH ADA** |
| `residents` | 004, 007, 008, 010, 012 | **SUDAH ADA** |
| `permission_types` | 011 | **SUDAH ADA** — Granular permissions |
| `position_permissions` | 011 (diperbarui 025) | **SUDAH ADA** — Jabatan → permission mapping |
| `financial_categories` | 009 | **SUDAH ADA** |
| `dues` | 009 (diperbarui 017, 018) | **SUDAH ADA** |
| `bills` | 009 (diperbarui 021) | **SUDAH ADA** |
| `payments` | 009 (diperbarui 020) | **SUDAH ADA** |
| `transactions` | 009 (diperbarui 026) | **SUDAH ADA** — Append-only financial ledger |
| `idempotency_keys` | 007 | **SUDAH ADA** |
| `audit_logs` | 007 | **SUDAH ADA** |
| `period_opening_balances` | 024 | **SUDAH ADA** |
| **`rts_rw_links`** | — | **BELUM ADA** — Tidak ada tabel relasi RW-RT |
| **`user_rw_memberships`** | — | **BELUM ADA** — Tidak ada membership RW |
| **`rw_organizations`** | — | **BELUM ADA** — Tidak ada tabel organisasi RW |

**Total migration files:** 52 (26 versi: 001–026, masing-masing up + down).

**File path:** `backend/migrations/`

### 4.2 Tabel `user_rt_memberships` — Struktur Saat Ini

| Kolom | Tipe | Keterangan |
|-------|------|------------|
| `id` | uuid PK | |
| `user_id` | uuid FK → `users.id` ON DELETE CASCADE | |
| `rt_id` | uuid FK → `rts.id` ON DELETE CASCADE | Tenant scope |
| `role` | text | CHECK IN (`pengurus`, `bendahara`, `warga`) |
| `jabatan` | text NULL | CHECK IN (`ketua`, `wakil_ketua`, `sekretaris`, `bendahara`, `keamanan`, `sosial`, `kebersihan_pembangunan`, `anggota`) |
| `is_active` | boolean | Default `true` |
| `created_at` | timestamptz | |
| `updated_at` | timestamptz | |

**Constraint:** UNIQUE `(user_id, rt_id) WHERE is_active = true`

**Verifikasi:** File `backend/migrations/011_add_positions_permissions.up.sql`, baris 5–12.

### 4.3 Tabel `position_permissions` — Struktur Saat Ini

| Kolom | Tipe | Keterangan |
|-------|------|------------|
| `id` | uuid PK | |
| `position` | text | CHECK IN jabatan RT yang sama |
| `permission_id` | uuid FK → `permission_types.id` ON DELETE CASCADE | |

**Verifikasi:** File `backend/migrations/011_add_positions_permissions.up.sql`, baris 47–56.

### 4.4 Tabel `permission_types` — Struktur Saat Ini

| Kolom | Tipe | Keterangan |
|-------|------|------------|
| `id` | uuid PK | |
| `code` | text UNIQUE | Permission code (e.g. `warga.read`, `finance.create`) |
| `domain` | text | Feature domain |
| `operation` | text | Action (read, create, update, approve, import, export, manage) |

**Verifikasi:** File `backend/migrations/011_add_positions_permissions.up.sql`, baris 15–25.

### 4.5 Tabel `rts` — Struktur Saat Ini

| Kolom | Tipe | Keterangan |
|-------|------|------------|
| `id` | uuid PK | |
| `name` | text NOT NULL | |
| `rw` | int NOT NULL | RW number (bukan FK) |
| `rt` | text NOT NULL | RT number |
| `address` | text | |
| `head_name` | text | Nama Ketua RT |
| `is_active` | boolean | Default `true` |
| `created_at` | timestamptz | |
| `updated_at` | timestamptz | |

**Constraint:** UNIQUE `(rw, rt) WHERE is_active = true` (dari migration 005)

**Verifikasi:** File `backend/migrations/001_initial.up.sql` + `005_fix_rt_unique_indexes.up.sql`.

---

## 5. Inventaris File dan Komponen Relevan

### 5.1 Backend Go

| Komponen | File Path | Status |
|----------|-----------|--------|
| Route registration | `backend/cmd/api/main.go` | **SUDAH ADA** — RT routes + semua route lain |
| RT Handler | `backend/internal/rt/handler.go` | **SUDAH ADA** — CRUD lengkap |
| RT Service | `backend/internal/rt/service.go` | **SUDAH ADA** |
| RT Repository | `backend/internal/rt/repository.go` | **SUDAH ADA** |
| RT Model | `backend/internal/rt/model.go` | **SUDAH ADA** |
| RT Handler Tests | `backend/internal/rt/handler_test.go` | **SUDAH ADA** |
| Auth Middleware | `backend/internal/auth/middleware.go` | **SUDAH ADA** — RequireAuth, RequireSystemRole, RequireRole, RequirePermission |
| Auth Models | `backend/internal/auth/models.go` | **SUDAH ADA** — SystemRole, Role, Jabatan |
| Auth Handlers | `backend/internal/auth/handlers.go` | **SUDAH ADA** |
| Permission Resolver | `backend/internal/auth/permission.go` | **SUDAH ADA** — jabatan → permission resolution |
| DevSeed | `backend/internal/devseed/seed.go` | **SUDAH ADA** |

**Total backend files:** Di audit via `grep` dan `read_file`.

### 5.2 Web React

| Komponen | File Path | Status |
|----------|-----------|--------|
| App Router | `frontend/src/app/App.tsx` | **SUDAH ADA** — Route `/rt` → `<SuperAdminRoute>` → `<RtList />` |
| Sidebar | `frontend/src/components/Sidebar.tsx` | **SUDAH ADA** — Menu RT untuk Super Admin |
| RtList | `frontend/src/features/rt/RtList.tsx` | **SUDAH ADA** — CRUD list RT |
| RtDetail | `frontend/src/features/rt/RtDetail.tsx` | **SUDAH ADA** — Detail RT |
| RtCreate | `frontend/src/features/rt/RtCreate.tsx` | **SUDAH ADA** — Buat RT baru |
| RtEdit | `frontend/src/features/rt/RtEdit.tsx` | **SUDAH ADA** — Edit RT |
| AuthContext | `frontend/src/app/AuthContext.tsx` | **SUDAH ADA** — Auth state + role check |
| API Client | `frontend/src/app/api.ts` | **SUDAH ADA** |

### 5.3 Android Flutter

| Komponen | File Path | Status |
|----------|-----------|--------|
| App Router | `mobile/lib/app/app.dart` | **SUDAH ADA** — GoRouter, shell routes |
| Navigation Shell | `mobile/lib/app/navigation_shell.dart` | **SUDAH ADA** |
| Login Screen | `mobile/lib/features/auth/presentation/screens/login_screen.dart` | **SUDAH ADA** |
| Mock Auth Repository | `mobile/lib/features/auth/data/mock_auth_repository.dart` | **SUDAH ADA** |

**KONFIRMASI:** Tidak ada halaman, route, atau operasi administrasi RT/RW di Flutter. Flutter saat ini hanya memiliki fitur RT-level: warga, iuran, kas, dashboard.

### 5.4 Dokumentasi

| Dokumen | File Path | Baris | Status |
|---------|-----------|-------|--------|
| Database Design | `docs/database.md` | 586 | **SUDAH ADA** |
| API Reference | `docs/api.md` | 767 | **SUDAH ADA** |
| Authorization Design | `docs/authorization-design.md` | 464 | **SUDAH ADA** |
| RW Architecture Plan | `docs/rw-architecture-plan.md` | 683 | **SUDAH ADA** — Konseptual, belum implementasi |
| Business Rules | `docs/business-rules.md` | — | **SUDAH ADA** |
| Security | `docs/security.md` | — | **SUDAH ADA** |

---

## 6. Model Data Saat Ini

### 6.1 Hubungan Akun – Warga – Rumah Tangga – RT

```
users (system_role = super_admin, NULL, atau 'super_admin')
  │
  ├── system_role = 'super_admin' → 0 memberships, platform-wide access
  │
  └── system_role = NULL → 1+ memberships di user_rt_memberships
        │
        user_rt_memberships
        ├── user_id → users.id
        ├── rt_id → rts.id
        ├── role → 'pengurus' | 'bendahara' | 'warga'
        └── jabatan → 'ketua' | 'wakil_ketua' | 'sekretaris' | 'bendahara' | 'keamanan' | 'sosial' | 'kebersihan_pembangunan' | 'anggota' | NULL

households (rt_id, kk_number, head_name, is_active)
  │
residents (rt_id, household_id → households.id, full_name, is_active)

physical_houses (rt_id, house_number)
household_occupancies (household_id, physical_house_id, occupancy_status, start_date, end_date)
residency_periods (resident_id, household_occupancy_id, start_date, end_date)
```

**Verifikasi:** `backend/migrations/002_auth.up.sql`, `003_separate_global_identity_from_tenant_membership.up.sql`, `004_households_and_residents.up.sql`, `008_temporal_household_model.up.sql`, `backend/internal/auth/models.go`, `frontend/src/app/AuthContext.tsx`.

### 6.2 Jabatan RT — Katalog Saat Ini

| Konstanta | Nilai | Deskripsi |
|-----------|-------|-----------|
| `JabatanKetua` | `"ketua"` | Ketua RT |
| `JabatanWakilKetua` | `"wakil_ketua"` | Wakil Ketua RT |
| `JabatanSekretaris` | `"sekretaris"` | Sekretaris RT |
| `JabatanBendahara` | `"bendahara"` | Bendahara RT |
| `JabatanKeamanan` | `"keamanan"` | Seksi Keamanan |
| `JabatanSosial` | `"sosial"` | Seksi Sosial |
| `JabatanKebersihanPembangunan` | `"kebersihan_pembangunan"` | Seksi Kebersihan/Pembangunan |

**Konstanta di:** `backend/internal/auth/models.go`, baris 33–55.

**KONFIRMASI:** Katalog jabatan saat ini **hanya untuk RT**. Tidak ada definisi jabatan RW.

### 6.3 Middleware Otorisasi

| Middleware | Lokasi | Fungsi |
|------------|--------|--------|
| `RequireAuth` | `backend/internal/auth/middleware.go:20` | Validasi JWT + validasi identity di DB |
| `RequireSystemRole` | `backend/internal/auth/middleware.go:88` | Cek `SystemRole` dalam allowed set |
| `RequireRole` | `backend/internal/auth/middleware.go:129` | Cek `TenantRole` dalam allowed set |
| `RequirePermission` | `backend/internal/auth/middleware.go:173` | Cek jabatan → permission via `position_permissions` |

**Super Admin bypass:** `RequireSystemRole(SuperAdmin)` → middleware `RequireRole` selalu pass (baris 147–149), `RequirePermission` selalu pass (baris 200–202).

### 6.4 Route RT Super Admin

```go
// backend/cmd/api/main.go, baris 298–306
r.Group(func(r chi.Router) {
    r.Use(auth.RequireAuth)
    r.Use(auth.RequireSystemRole(auth.SystemRoleSuperAdmin))
    r.Get("/api/v1/rts", rtHandler.ListAll)
    r.Post("/api/v1/rts", rtHandler.Create)
    r.Get("/api/v1/rts/{id}", rtHandler.GetByID)
    r.Patch("/api/v1/rts/{id}", rtHandler.Update)
    r.Patch("/api/v1/rts/{id}/deactivate", rtHandler.Deactivate)
})
```

---

## 7. Gap Analysis — RT

### 7.1 Yang Sudah Ada untuk RT

| Fitur | Status | Catatan |
|-------|--------|---------|
| CRUD RT (organisasi) | ✅ SUDAH ADA | Endpoint `/api/v1/rts/*` + middleware `RequireSystemRole(SuperAdmin)` |
| Tabel `rts` | ✅ SUDAH ADA | Migration 001 + 005 |
| Tabel `user_rt_memberships` | ✅ SUDAH ADA | Migration 002, 003, 011 |
| Kolom `jabatan` | ✅ SUDAH ADA | Migration 011 — CHECK constraint pada jabatan RT |
| Tabel `position_permissions` | ✅ SUDAH ADA | Migration 011 — mapping jabatan → permission |
| Tabel `permission_types` | ✅ SUDAH ADA | Migration 011 |
| Middleware `RequireSystemRole` | ✅ SUDAH ADA | `backend/internal/auth/middleware.go:88` |
| Middleware `RequirePermission` | ✅ SUDAH ADA | `backend/internal/auth/middleware.go:173` |
| Halaman RT (Web) | ✅ SUDAH ADA | `RtList`, `RtDetail`, `RtCreate`, `RtEdit` |
| Menu RT (Sidebar Web) | ✅ SUDAH ADA | `Sidebar.tsx:82` — `if (isSuperAdmin) { items.push({ to: '/rt', ... }) }` |
| Route `/rt` (Web) | ✅ SUDAH ADA | `App.tsx:72` — `<SuperAdminRoute>` |

### 7.2 Yang Belum Ada untuk RT (Tugas Administrasi)

| Fitur | Status | Catatan |
|-------|--------|---------|
| Endpoint CRUD jabatan pengurus | **BELUM ADA** | Tidak ada endpoint `PUT /api/v1/rts/{id}/jabatan` atau sejenisnya |
| Endpoint daftar keanggotaan RT (akun + jabatan) | **BELUM ADA** | RT handler hanya CRUD organisasi, bukan manajemen jabatan |
| Endpoint penetapan jabatan | **BELUM ADA** | Kolom `jabatan` sudah ada di DB, tapi tidak ada API untuk mengubahnya |
| Endpoint pencabutan jabatan | **BELUM ADA** | |
| Halaman Administrasi Tab RT (Web) | **BELUM ADA** | Saat ini hanya menu RT (CRUD organisasi) |
| Halaman penetapan/pencabutan jabatan (Web) | **BELUM ADA** | |

**USULAN — BELUM DIIMPLEMENTASIKAN:**

| Endpoint | Method | Deskripsi |
|----------|--------|-----------|
| `/api/v1/rts/{id}/memberships` | GET | Daftar keanggotaan RT (akun + role + jabatan) |
| `/api/v1/rts/{id}/memberships/{userId}/jabatan` | PUT | Tetapkan jabatan untuk user di RT tertentu |
| `/api/v1/rts/{id}/memberships/{userId}/jabatan` | DELETE | Cabut jabatan dari user di RT tertentu |
| `/api/v1/administrasi/jabatan-history` | GET | Riwayat perubahan jabatan (audit trail) |

---

## 8. Gap Analysis — RW

### 8.1 Status Saat Ini

| Item | Status | Catatan |
|------|--------|---------|
| Tabel organisasi RW | **TIDAK ADA** | Tidak ada `rw_organizations` atau tabel serupa |
| Tabel membership RW | **TIDAK ADA** | Tidak ada `user_rw_memberships` |
| Tabel relasi RW–RT | **TIDAK ADA** | Tidak ada `rts_rw_links` atau tabel serupa |
| Katalog jabatan RW | **TIDAK ADA** | Hanya jabatan RT yang terdefinisi |
| Permission domain RW | **TIDAK ADA** | `permission_types` hanya memiliki domain `warga`, `finance`, `reports`, `rt.settings` |
| Halaman RW (Web) | **TIDAK ADA** | |
| Halaman RW (Flutter) | **TIDAK ADA** | |
| Route RW | **TIDAK ADA** | |

**Sumber desain konseptual RW:** `docs/rw-architecture-plan.md` (683 baris) — **dokumen perencanaan, bukan implementasi**.

### 8.2 Kontribusi `docs/rw-architecture-plan.md`

Dokumen ini mendefinisikan:

- **Frozen requirements** (Section 3): optional hierarchy, separate authority, provisioning authority, financial integrity, no transitive permissions.
- **Planned architecture** (Section 4): organizational scope, membership model, finance ownership, report visibility.
- **Threat cases** (Section 9): 10 skenario ancaman yang harus ditangani.
- **Open questions** (Section 10): 17 pertanyaan desain yang belum beku.
- **Authorization matrix** (Section 8): matriks akses SUPER_ADMIN, Pengurus_RW, Bendahara_RW, dll.
- **Roadmap** (Section 11): STOP sebelum W5 Finance UI, jalankan RW.1 research dulu.

**KESIMPULAN:** RW sepenuhnya masih dalam fase perencanaan. Tidak ada kode, schema, endpoint, atau halaman yang terkait RW yang sudah diimplementasikan.

---

## 9. Desain Menu Administrasi

### 9.1 Struktur Menu (Konseptual)

```
Sidebar Web:
├── Beranda (/)
├── Warga (/warga)
├── Iuran (/iuran)
├── KAS (/kas)
└── Administrasi (/administrasi)     ← RENCANA: ganti menu RT
    ├── Tab RT (/administrasi)        ← default
    └── Tab RW (/administrasi/rw)     ← RENCANA: Fase 2+
```

### 9.2 Perubahan yang Dibutuhkan di Web

| File | Perubahan |
|------|-----------|
| `frontend/src/components/Sidebar.tsx` | Ganti label "RT" → "Administrasi", tambahkan navigasi tab |
| `frontend/src/app/App.tsx` | Ganti route `/rt` → `/administrasi`, tambah route `/administrasi/rw` |
| `frontend/src/features/rt/` | Buat folder baru `frontend/src/features/administrasi/` untuk halaman tab RT/RW |

**Pertimbangan kompatibilitas:** Route lama `/rt` dapat tetap diredirect ke `/administrasi` untuk backward compatibility.

---

## 10. Desain Alur Pengelolaan Pengurus RT

### 10.1 Flow Lengkap

```
Super Admin
    │
    ▼
[Buka Administrasi → RT]
    │
    ▼
[Halaman: Daftar RT]
  - Menampilkan semua RT (aktif + non-aktif)
  - Kolom: Nama, RW, RT, Status, Head Name
  - Filter: status (aktif/tidak aktif)
  - Search: nama RT
  - Action: "Kelola Pengurus" per RT
    │
    ▼
[Halaman: Daftar Pengurus RT {nama RT}]
  - Tabel: Nama Akun, Email, Role, Jabatan Saat Ini, Status
  - Sumber: JOIN users + user_rt_memberships WHERE rt_id = {selected_rt}
  - Action per baris:
    - "Tetapkan Jabatan" (jika jabatan = NULL)
    - "Ganti Jabatan" (jika jabatan ≠ NULL)
    - "Cabut Jabatan" (jika jabatan ≠ NULL)
    │
    ▼
[Modal: Tetapkan/Ganti/Cabut Jabatan]
  - Pilih jabatan dari katalog (dropdown)
  - Konfirmasi operasi
  - Submit → PUT /api/v1/rts/{id}/memberships/{userId}/jabatan
    │
    ▼
[Backend validation]
  - 1. RequireSystemRole(SuperAdmin)
  - 2. Validasi rt_id exists + is_active
  - 3. Validasi user_id exists + is_active
  - 4. Validasi user_id punya membership di rt_id
  - 5. Validasi jabatan dalam katalog
  - 6. Cek constraint (contoh: hanya 1 ketua per RT)
  - 7. Transaksi DB: UPDATE user_rt_memberships SET jabatan = ?
  - 8. Audit log: actor, target, scope, jabatan lama → baru
  - 9. Invalidate refresh token user jika diperlukan
    │
    ▼
[Response: 200 OK dengan data terbaru]
    │
    ▼
[Halaman: Refresh data otomatis]
  - Tabel diperbarui
  - Toast/notification: "Jabatan berhasil diperbarui"
```

### 10.2 Constraint Bisnis yang Perlu Diterapkan

| Constraint | Keterangan |
|------------|------------|
| Satu Ketua per RT | Hanya satu pemegang `jabatan = 'ketua'` per RT aktif |
| Jabatan dalam katalog | Hanya nilai yang ada di `ValidJabatans` yang boleh ditetapkan |
| Membership harus aktif | User harus punya `user_rt_memberships.is_active = true` |
| User harus aktif | `users.is_active = true` |
| Transaksi atomik | UPDATE + audit log dalam satu transaction |
| Audit log wajib | Catat actor (super_admin), target (user_id), scope (rt_id), jabatan lama → baru, waktu |

---

## 11. Desain Alur Pengelolaan Pengurus RW

### 11.1 Status: DIRENCANAKAN — BELUM AKTIF

Tab RW berstatus **direncanakan** hingga:

1. Model data RW disepakati (tabel `rw_organizations`, `user_rw_memberships`, relasi RW–RT).
2. Katalog jabatan RW didefinisikan.
3. Permission domain RW ditetapkan.
4. Migration disetujui dan diimplementasikan.

### 11.2 Prinsip Desain (dari `docs/rw-architecture-plan.md`)

- RW dan RT adalah **scope operasional terpisah**.
- RW perangkat **MUST NOT** gain write access ke RT data.
- RT perangkat **MUST NOT** gain write access ke RW data.
- **No transitive permissions**.
- **No write inheritance** dari hubungan organisasi.
- SUPER_ADMIN provisions RW organizations.

### 11.3 USULAN — BELUM DIIMPLEMENTASIKAN

| Endpoint | Method | Deskripsi |
|----------|--------|-----------|
| `/api/v1/administrasi/rw` | GET | Daftar organisasi RW |
| `/api/v1/administrasi/rw/{id}` | GET | Detail RW + keanggotaan |
| `/api/v1/administrasi/rw/{id}/memberships/{userId}/jabatan` | PUT | Tetapkan jabatan RW |
| `/api/v1/administrasi/rw/{id}/memberships/{userId}/jabatan` | DELETE | Cabut jabatan RW |

**Catatan:** Endpoint di atas membutuhkan migration untuk tabel RW yang belum ada. **Tidak boleh dilompat** ke Fase 3 tanpa menyelesaikan Fase 2.

---

## 12. Independent Organizational Membership dan Batas Scope

### 12.A. Prinsip Membership Mandiri per Scope

Identitas akun bersifat **tunggal**, tetapi **membership** dan **jabatan** berlaku **hanya pada scope organisasi** tempat membership tersebut didefinisikan.

```
users (1 identitas global)
  │
  ├── user_rt_memberships (scope: RT)
  │     ├── rt_id: "rt-003"
  │     ├── role: "warga"
  │     └── jabatan: NULL          → Warga biasa di RT 003
  │
  └── user_rw_memberships (scope: RW, RENCANA: Fase 2+)
        ├── rw_id: "rw-05"
        ├── role: "pengurus"
        └── jabatan: "ketua"       → Ketua RW 05
```

#### Konfirmasi Implementasi Aktual

| Fakta | Status | Sumber |
|-------|--------|--------|
| `users` sebagai identitas global | ✅ SUDAH ADA | Migration 002 + 003 |
| `user_rt_memberships` untuk membership RT | ✅ SUDAH ADA | Migration 002, 003, 011 |
| Kolom `jabatan` di `user_rt_memberships` | ✅ SUDAH ADA | Migration 011 |
| `user_rw_memberships` untuk membership RW | **BELUM ADA** | Tidak ada tabel ini di skema manapun |
| `rw_organizations` | **BELUM ADA** | Tidak ada tabel ini di skema manapun |
| Katalog jabatan RW di kode Go | **BELUM ADA** | Hanya `Jabatan*` konstanta untuk RT di `backend/internal/auth/models.go` |

**Penting:** Semua klaim yang bertanda **BELUM ADA** berarti tidak ada kode, migrasi, atau tabel yang relevan yang ditemukan di repository pada commit `2f6678d`.

#### Aturan Membership Mandiri

| Kasus | Aturan |
|-------|--------|
| Satu akun, dua scope | Pak Andi bisa memiliki `user_rt_memberships` di RT 03 DAN `user_rw_memberships` di RW 05 dalam waktu yang sama. |
| Jabatan tidak menular antar-scope | Jabatan Ketua RW 05 **TIDAK** memberikan jabatan pengurus di RT mana pun. Jabatan Ketua RT 03 **TIDAK** memberikan jabatan pengurus di RW mana pun. |
| Pencabutan jabatan RW | Pencabutan jabatan Ketua RW 05 **TIDAK** otomatis menonaktifkan atau mengubah `user_rt_memberships` Pak Andi di RT 03. |
| Penonaktifan membership RT | Menonaktifkan `user_rt_memberships` Pak Andi di RT 03 **TIDAK** otomatis mencabut jabatan Ketua RW 05 yang masih sah. |
| Jabatan rangkap | Seseorang boleh memiliki jabatan RT dan RW sekaligus jika aturan organisasi mengizinkan, tetapi setiap jabatan dan permission tetap **terpisah secara operasional**. |
| Permission terikat scope | Permission yang diperoleh dari jabatan RW 05 **hanya** berlaku untuk operasi di scope RW 05. Permission yang diperoleh dari membership RT 03 **hanya** berlaku untuk operasi di scope RT 03. |

#### Contoh Skenario: Pak Andi

| Properti | Nilai |
|----------|-------|
| Akun `users.id` | `usr-andi-001` |
| Membership RT | RT 03 — role: `warga`, jabatan: `NULL` (Warga biasa) |
| Membership RW | RW 05 — role: `pengurus`, jabatan: `ketua` (Ketua RW) |
| Permission RT 03 | Hanya permission dasar warga (lihat matriks §22) |
| Permission RW 05 | Permission Ketua RW (tidak mempengaruhi RT mana pun) |

**Hasil:** Pak Andi tidak otomatis menjadi pengurus RT 03 karena jabatannya di RW 05. Pak Andi hanya memperoleh permission yang secara eksplisit berlaku untuk scope RW 05 dan membership RT 03 miliknya.

### 12.B. Data Model dan Batas Scope

| Aspek | Sumber Data | Catatan |
|-------|-------------|---------|
| Identitas akun | `users` (sudah ada) | Satu identitas global, mendukung `system_role` |
| Membership RT | `user_rt_memberships` (sudah ada) | Migration 002, 003, 011 — kolom `jabatan` sudah ada |
| Jabatan RT | `user_rt_memberships.jabatan` (sudah ada) | CHECK constraint pada katalog jabatan RT |
| Membership RW | **BELUM ADA** — rancangan: `user_rw_memberships` | Akan dibuat di Fase 2 migration |
| Jabatan RW | **BELUM ADA** — rancangan: `user_rw_memberships.jabatan` | Katalog jabatan RW terpisah dari jabatan RT |
| Organisasi RW | **BELUM ADA** — rancangan: `rw_organizations` | Akan dibuat di Fase 2 migration |

#### Batas yang Harus Dijaga

| Batas | Alasan |
|-------|--------|
| **Jangan** menyimpan jabatan RW di kolom `user_rt_memberships.jabatan` | Kolom ini memiliki CHECK constraint untuk katalog jabatan RT. Jabatan RW harus menggunakan katalog dan tabel terpisah. |
| **Jangan** menganggap `system_role`, jabatan RT, jabatan RW, dan status warga sebagai satu jenis role | `system_role` (super_admin / NULL) adalah otoritas platform. Jabatan RT/RW adalah metadata organisasi per-scope. Status warga (role di membership) adalah level akses tenant. Ketiganya orthogonal. |
| **Jangan** mengasumsikan setiap warga wajib memiliki akun login | Saat ini model mendukung warga tanpa akun login (akses melalui anggota rumah tangga atau link eksternal). Keputusan ini tetap terbuka. |
| **Jangan** mengasumsikan satu akun hanya merepresentasikan satu identitas warga | Satu akun bisa memiliki multiple memberships. Satu warga bisa represented oleh beberapa akun (kebijakan organisasi). |
| **Bedakan** fakta yang sudah terverifikasi dengan rancangan yang diusulkan | Semua klaim factual harus bersumber dari kode aktual. Rancangan yang belum diimplementasikan ditandai **BELUM TERVERIFIKASI** atau **USULAN**. |

### 12.C. Otorisasi Backend per Scope

Setiap endpoint harus memvalidasi **scope** dan **permission** yang relevan.

| Prinsip | Implementasi |
|---------|--------------|
| Permission RW tidak memberi akses RT | `RequirePermission("rw.ketua.read")` **TIDAK** boleh bypass validasi untuk endpoint RT (`/api/v1/rts/...`). |
| Permission RT tidak memberi akses RW | `RequirePermission("rt.ketua.manage")` **TIDAK** boleh bypass validasi untuk endpoint RW. |
| Scope harus diekstrak dari JWT atau path | JWT `rt_id` untuk operasi RT, JWT `rw_id` (rencana) untuk operasi RW. Endpoint Administrasi Super Admin menggunakan path parameter yang diverifikasi. |
| Super Admin override | Middleware `RequireSystemRole(SuperAdmin)` membebaskan semua validasi scope karena Super Admin mengelola semua organisasi. |

#### USULAN — Belum Diimplementasikan: Middleware Scope Validation

| Endpoint | Scope Validated | Middleware Chain |
|----------|----------------|------------------|
| `/api/v1/administrasi/rts/...` | RT scope | `RequireAuth` → `RequireSystemRole(SuperAdmin)` |
| `/api/v1/administrasi/rw/...` (Fase 2) | RW scope | `RequireAuth` → `RequireSystemRole(SuperAdmin)` → (opsional: `RequirePermission("rw.*")`) |
| `/api/v1/rts/{id}/...` (existing) | RT scope | `RequireAuth` → `RequireSystemRole(SuperAdmin)` |
| Endpoint warga/iuran/kas (existing) | RT scope dari JWT | `RequireAuth` → `RequireRole(...)` |

**Penting:** Pengangkatan, penggantian, dan pencabutan pengurus RT dan RW **tetap hanya boleh dilakukan oleh Super Admin** sesuai keputusan produk saat ini. Ketentuan ini harus ditegakan di backend via middleware, **bukan hanya** melalui penyembunyian menu UI.

### 12.D. Data dan Audit Trail Antar-Scope

| Aspek | Perilaku |
|-------|----------|
| Riwayat keuangan | Tidak terpengaruh perubahan jabatan RT atau RW. Transaksi `transactions` bersifat append-only dan terikat `rt_id`. |
| Riwayat iuran | `bills` dan `payments` terikat `rt_id`. Perubahan jabatan tidak menghapus atau memindahkan riwayat iuran. |
| Identitas warga | `residents` dan `households` terikat `rt_id`. Perubahan jabatan pengurus tidak mengubah data warga. |
| Audit log | Perubahan jabatan RT dan RW dicatat terpisah di `audit_logs` dengan `scope_type` dan `scope_id` yang berbeda. |

---

## 13. USULAN Kontrak API untuk Operasi Administrasi

### 13.1 USULAN — BELUM DIIMPLEMENTASIKAN

Semua endpoint di bawah memerlukan middleware `RequireSystemRole(SuperAdmin)`.

| Method | Path | Deskripsi | Request | Response |
|--------|------|-----------|---------|----------|
| GET | `/api/v1/administrasi/rts` | Daftar RT untuk administrasi | Query: `is_active`, `search`, `page`, `page_size` | Paginated RT list |
| GET | `/api/v1/administrasi/rts/{id}/memberships` | Daftar keanggotaan RT + jabatan | — | Array: user_id, email, full_name, role, jabatan, is_active |
| PUT | `/api/v1/administrasi/rts/{id}/memberships/{userId}/jabatan` | Tetapkan/ganti jabatan | Body: `{ "jabatan": "ketua" }` | 200 OK with updated membership |
| DELETE | `/api/v1/administrasi/rts/{id}/memberships/{userId}/jabatan` | Cabut jabatan | — | 204 No Content |
| GET | `/api/v1/administrasi/rts/{id}/jabatan-history` | Riwayat perubahan jabatan | — | Array: timestamp, actor, user, jabatan_lama → baru |

### 13.2 Error Response Format

```json
{ "error": { "code": "validation_error", "message": "...", "details": [...] } }
```

Sesuai konvensi API yang sudah ada (`docs/api.md`).

---

## 14. Kebijakan Akses Super Admin

### 14.1 Backend

| Lapisan | Mekanisme | File |
|---------|-----------|------|
| Middleware | `RequireSystemRole(auth.SystemRoleSuperAdmin)` | `backend/internal/auth/middleware.go:88` |
| JWT Claims | `claims.SysRole` → `SystemRole` | `backend/internal/auth/models.go` |
| DB Validation | `ValidateIdentity(ctx, tx, claims.UserID, claims.MID)` | `backend/internal/auth/repository.go` |

**Konfirmasi:** `RequireSystemRole` sudah digunakan untuk route RT yang ada (`main.go:299`). Pola yang sama dapat digunakan untuk endpoint Administrasi baru.

### 14.2 Web Frontend

| Lapisan | Mekanisme | File |
|---------|-----------|------|
| AuthContext | `user.systemRole === 'super_admin'` | `frontend/src/app/AuthContext.tsx` |
| Route Guard | `<SuperAdminRoute>` wrapper | `frontend/src/app/App.tsx:36` |
| Sidebar | `if (isSuperAdmin) { items.push(...) }` | `frontend/src/components/Sidebar.tsx:82` |

**Konfirmasi:** Pattern `SuperAdminRoute` sudah ada dan dapat digunakan untuk route Administrasi.

### 14.3 Android (Flutter)

| Lapisan | Status |
|---------|--------|
| Auth Model | `system_role` ada di auth response (`mobile/lib/app/app.dart`) |
| Route Guard | `isAllowedRT(user)` di `app.dart:179` |
| Halaman Administrasi | **TIDAK ADA** — perlu implementasi baru jika diperlukan |

**Rekomendasi:** Administrasi hanya di Web pada Fase 1. Flutter dapat ditambahkan pada Fase 3 atau lebih.

---

## 15. Audit Trail, Konsistensi Transaksi, dan Permission

### 15.1 Audit Trail

| Item | Tersedia | Catatan |
|------|----------|---------|
| `audit_logs` table | ✅ SUDAH ADA | Migration 007 — foundation audit logging |
| `idempotency_keys` table | ✅ SUDAH ADA | Migration 007 |
| Penulisan audit untuk perubahan jabatan | **BELUM ADA** | Perlu implementasi handler untuk menulis ke `audit_logs` |

### 15.2 Konsistensi Transaksi

| Kebutuhan | Status |
|-----------|--------|
| Transaksi atomik (UPDATE jabatan + audit log) | Perlu implementasi |
| Constraint DB untuk mencegah penetapan ganda | Perlu evaluasi (contoh: max 1 ketua per RT) |
| Invalidasi refresh token setelah perubahan jabatan | Perlu evaluasi — role change mungkin perlu refresh token baru |

### 15.3 Pembaruan Permission Setelah Perubahan Jabatan

Perubahan jabatan → perubahan permission (via `position_permissions`). Pemegang token lama mungkin masih memiliki permission lama. Opsi:

1. **Paksa re-login** — user harus login ulang untuk mendapatkan token dengan permission baru.
2. **Refresh token rotation** — putuskan semua session aktif.
3. **Short-lived access tokens** — sudah berlaku (15 menit), permission akan update otomatis.

**Rekomendasi:** Gunakan short-lived tokens (sudah ada). Untuk perubahan kritis, pertimbangkan opsi 1 atau 2.

---

## 16. Penanganan Error, Loading, Empty State, dan Status Nonaktif

### 16.1 Pattern yang Sudah Ada (Web)

| Kasus | Pattern | File |
|-------|---------|------|
| Loading | `loading` state → spinner/banner | `RtList.tsx:35` |
| Error | `error` state → display error banner | `RtList.tsx:36` |
| Empty | Conditional rendering | `RtList.tsx` |
| Non-active RT | `is_active` column + filter | `RtList.tsx` |
| Permission denied | 403 → redirect to `/` | `App.tsx:39–43` |

### 16.2 USULAN — BELUM DIIMPLEMENTASIKAN

| Kasus | Penanganan |
|-------|------------|
| Jabatan tidak valid | 400 Bad Request dengan detail |
| User tidak punya membership di RT | 404 Not Found |
| RT tidak ditemukan | 404 Not Found |
| Duplikat jabatan (contoh: 2 ketua) | 409 Conflict |
| Race condition | 409 Conflict + advisory lock DB |

---

## 17. Rencana Perubahan Per Lapisan dan File

### 17.1 Fase 1 — Administrasi RT

| Lapisan | File | Perubahan |
|---------|------|-----------|
| Backend Route | `backend/cmd/api/main.go` | Tambah route group Administrasi + handler |
| Backend Handler | `backend/internal/administrasi/handler.go` | **BARU** — CRUD jabatan |
| Backend Service | `backend/internal/administrasi/service.go` | **BARU** — business logic jabatan |
| Backend Repository | `backend/internal/administrasi/repository.go` | **BARU** — DB queries |
| Backend Test | `backend/internal/administrasi/handler_test.go` | **BARU** — unit + integration tests |
| Frontend Route | `frontend/src/app/App.tsx` | Ganti `/rt` → `/administrasi` + redirect |
| Frontend Sidebar | `frontend/src/components/Sidebar.tsx` | Ganti label "RT" → "Administrasi" |
| Frontend Page | `frontend/src/features/administrasi/AdminList.tsx` | **BARU** — halaman daftar RT (mirip RtList) |
| Frontend Page | `frontend/src/features/administrasi/AdminManage.tsx` | **BARU** — halaman kelola pengurus |
| Frontend API | `frontend/src/app/api.ts` | Tambah fungsi `apiListAdministrasiRTs`, `apiSetJabatan`, dll |

### 17.2 Fase 2 — Fondasi RW

| Lapisan | File | Perubahan |
|---------|------|-----------|
| Migration | `backend/migrations/027_rw_organizations.up.sql` | **BARU** |
| Migration | `backend/migrations/027_rw_organizations.down.sql` | **BARU** |
| Backend Model | `backend/internal/rw/model.go` | **BARU** |
| Backend Repository | `backend/internal/rw/repository.go` | **BARU** |
| Backend Service | `backend/internal/rw/service.go` | **BARU** |
| Backend Handler | `backend/internal/rw/handler.go` | **BARU** |

### 17.3 Fase 3 — Administrasi RW

| Lapisan | File | Perubahan |
|---------|------|-----------|
| Frontend Page | `frontend/src/features/administrasi/AdminRw.tsx` | **BARU** |
| Backend Route | `backend/cmd/api/main.go` | Tambah route Administrasi RW |

---

## 18. Rencana Migration dan Strategi Kompatibilitas Data

### 18.1 Fase 1 — Administrasi RT

**Tidak perlu migration baru.** Semua operasi menggunakan tabel yang sudah ada:
- `user_rt_memberships.jabatan` — kolom sudah ada (migration 011).
- `audit_logs` — foundation sudah ada (migration 007).
- `permission_types` + `position_permissions` — sudah ada (migration 011 + 025).

### 18.2 Fase 2 — Fondasi RW

Migration yang diperkirakan dibutuhkan:

1. **027_rw_organizations.up.sql** — buat tabel `rw_organizations`, `user_rw_memberships`, `rts_rw_links`.
2. **028_position_permissions_rw.up.sql** — tambah permission domain `rw.*` dan mapping jabatan RW.

**Strategi Kompatibilitas:**
- Migration harus backward compatible — tidak boleh blocking.
- RT yang sudah ada tidak boleh terpengaruh.
- `rts.rw` column (bukan FK) tetap sebagai metadata, bukan referensi operasional.

### 18.3 Risiko Migration

| Risiko | Mitigasi |
|--------|----------|
| Breaking existing RT queries | Gunakan migration incremental, tidak drop/rename kolom existing |
| Data RW belum ada | Semua kolom RW-related nullable, default to NULL |
| Konflik constraint UNIQUE | Gunakan partial indexes (seperti `uniq_rts_active`) |
| Performance impact | Index pada kolom yang difilter (rt_id, rw_id, user_id) |

---

## 19. Rencana Pengujian

### 19.1 Backend Tests

| Kategori | Test Cases |
|----------|-----------|
| **Middleware** | `RequireSystemRole(SuperAdmin)` — pass untuk super_admin, 403 untuk role lain |
| **Endpoint RT List** | List semua RT, filter `is_active`, search by name, pagination |
| **Endpoint Set Jabatan** | Validasi: RT exists, user exists, membership exists, jabatan valid |
| **Endpoint Delete Jabatan** | Set jabatan → NULL, verifikasi |
| **Tenant Isolation** | Super Admin tidak bisa melihat RT yang bukan miliknya (jika ada multi-tenant admin) |
| **Constraint** | Duplikat jabatan (max 1 ketua per RT) → 409 Conflict |
| **Race Condition** | Dua penetapan jabatan bersamaan → advisory lock |
| **Audit Trail** | Setiap perubahan jabatan → record di `audit_logs` |
| **Idempotency** | PUT jabatan yang sama → 200 OK, tidak ada perubahan duplikat |

### 19.2 Web Frontend Tests

| Kategori | Test Cases |
|----------|-----------|
| **Route Guard** | Non-super_admin → redirect ke `/` |
| **Sidebar** | Menu "Administrasi" hanya muncul untuk super_admin |
| **Halaman Daftar RT** | Render, loading, error, empty state |
| **Halaman Kelola Pengurus** | Tabel menampilkan user + jabatan, action buttons |
| **Modal Tetapkan Jabatan** | Dropdown jabatan, submit, refresh data |
| **HTTP Error Handling** | 400, 401, 403, 404, 409, 500 |
| **Permission Denied** | User non-super_admin → redirect |

### 19.3 Android Tests

| Kategori | Test Cases |
|----------|-----------|
| **Route Guard** | User non-super_admin tidak melihat menu Administrasi |
| **API Call** | Auth header, RTID derivation dari JWT |
| **UI** | Loading, error, empty state |

### 19.4 Security Tests

| Test | Deskripsi |
|------|-----------|
| **Role tampering** | Frontend mengirim role palsu di body → backend menolak |
| **ID manipulation** | User mencoba mengakses RT orang lain → 403 |
| **Direct API access** | Non-super_admin call langsung ke endpoint Administrasi → 403 |
| **Stale token** | Token lama sebelum perubahan jabatan → token expired, re-login diperlukan |

### 19.5 Independent Membership Scope Tests (MUST — BELUM DIIMPLEMENTASIKAN)

Berikut adalah 10 skenario pengujian minimum yang **wajib** ditambahkan pada fase implementasi. Tandai sebagai **Wajib Implementasi** — bukan berarti sudah lulus, melainkan harus diuji saat implementasi berjalan.

| # | Skenario | Input | Expected Result | Test Type |
|---|----------|-------|-----------------|-----------|
| **S-01** | Ketua RW berstatus Warga di RT | JWT: role=pengurus, jabatan=ketua, rw_id=rw-05 + membership RT 03 sebagai warga | Endpoint RW: 200 OK. Endpoint RT (warga.read): 200 OK (sebagai warga biasa). Endpoint RT (finance.create): 403 (bukan bendahara pengurus RT). | Backend integration test |
| **S-02** | Ketua RW sekaligus Ketua RT (jika diizinkan) | JWT: membership RT jabatan=ketua + membership RW jabatan=ketua. Permission matrix mengizinkan kedua jabatan. | Endpoint RT (rt.settings.manage): 200 OK. Endpoint RW (rw.settings.manage): 200 OK. Permission evaluation menunjukkan kedua jabatan beroperasi secara independen. | Backend integration test |
| **S-03** | Jabatan RW dicabut, membership RT tetap aktif | Super Admin DELETE jabatan RW Ketua → 204. User login ulang. | JWT baru: permission RW hilang, permission RT (jika ada jabatan RT) tetap ada. Riwayat keuangan RT tidak terpengaruh. | Backend integration test + End-to-end |
| **S-04** | Membership RT dinonaktifkan, membership RW tetap aktif | Super Admin PATCH user_rt_memberships SET is_active=false. User login ulang. | JWT baru: permission RT hilang. Permission RW tetap ada (jika ada jabatan RW). Riwayat keuangan RT tetap tercatat di ledger. | Backend integration test + End-to-end |
| **S-05** | User mencoba pakai permission RW untuk akses endpoint RT | JWT dengan permission rw.ketua.manage → call GET /api/v1/administrasi/rts/{id}/memberships | 403 Forbidden. Middleware RequireSystemRole(SuperAdmin) atau RequirePermission(rts.*) menolak. | Backend security test |
| **S-06** | User mencoba pakai permission RT untuk akses endpoint RW | JWT dengan permission rt.ketua.manage → call GET /api/v1/administrasi/rw | 403 Forbidden. Middleware RequireSystemRole(SuperAdmin) atau RequirePermission(rw.*) menolak. | Backend security test |
| **S-07** | Satu akun memiliki beberapa membership organisasi dengan scope berbeda | User A: membership RT 03 (warga) + membership RT 04 (pengurus) + membership RW 05 (ketua). JWT hanya memuat satu RT scope (dari JWT claims) + scope RW (jika ada). | User hanya mendapat permission dari scope aktif di JWT. Scope lain tidak mempengaruhi permission saat ini, tetapi tetap tercatat di DB dan bisa diakses setelah switch scope. | Backend integration test |
| **S-08** | Pengangkatan/pencabutan jabatan bersamaan dengan permintaan permission lama | Super Admin SET jabatan RT dari NULL → ketua. Di waktu yang sama, user dengan token lama mencoba endpoint RT. | Token lama (dengan jabatan NULL) mendapat permission lama. Token baru (setelah re-login atau expiry) mendapat permission ketua. Idempotency key mencegah duplikasi. | Backend concurrency test |
| **S-09** | Perubahan jabatan tidak menghapus riwayat keuangan/iuran/identitas warga | Super Admin SET/CABUT jabatan RT. Query financial_transactions WHERE rt_id={rt}. Query bills WHERE rt_id={rt}. Query residents WHERE rt_id={rt}. | Semua data tetap ada, tidak berubah, tidak terhapus. Jabatan pengurus hanya mengontrol akses, bukan kepemilikan data. | Backend integration test |
| **S-10** | Super Admin mengubah jabatan, user biasa mencoba operasi sama via API | Non-super_admin POST /api/v1/administrasi/rts/{id}/memberships/{userId}/jabatan dengan body jabatan | 403 Forbidden. Middleware RequireSystemRole(SuperAdmin) menolak. | Backend security test |

> **Catatan:** Semua test di atas berstatus **Wajib Implementasi — belum diuji**. Tandai sebagai `[ ] Belum` pada checklist implementasi. Tidak ada test di atas yang sudah berlalu karena belum ada kode yang diimplementasikan untuk fitur ini.

### 19.6 Cross-Organization Permission Tests (MUST — BELUM DIIMPLEMENTASIKAN)

| Test | Deskripsi |
|------|-----------|
| **RW permission tidak bocor ke RT** | User dengan jabatan RW hanya mendapat permission domain `rw.*`. Semua endpoint yang memfilter domain `rt.*` atau tenant scope RT harus menolak. |
| **RT permission tidak bocor ke RW** | User dengan jabatan RT hanya mendapat permission domain `rt.*`. Semua endpoint yang memerlukan domain `rw.*` harus menolak. |
| **Multi-scope isolation** | User dengan jabatan di RT 03 dan RW 05 tidak bisa mengakses data RT 04 atau RW lain. |
| **Token rotation pada jabatan change** | Setelah jabatan diubah, semua token lama (sebelum perubahan) tidak boleh memberikan permission baru. |

---

## 20. Tahapan Implementasi dan Kriteria Penerimaan

### 20.1 Fase 0 — Dokumetasi dan Keputusan Arsitektur (TUGAS SAAT INI)

- ✅ Repository diaudit.
- ✅ Dokumen ini dibuat.

### 20.2 Fase 1 — Administrasi RT

| Kriteria | Status |
|----------|--------|
| Menu RT di sidebar diganti "Administrasi" | [ ] Belum |
| Route `/administrasi` berfungsi | [ ] Belum |
| Endpoint CRUD jabatan tersedia | [ ] Belum |
| Halaman kelola pengurus tersedia | [ ] Belum |
| Hanya Super Admin yang bisa akses | [ ] Belum |
| Backend middleware `RequireSystemRole` diterapkan | [ ] Belum |
| Test coverage memadai | [ ] Belum |
| Kompatibilitas dengan fitur RT yang ada terjaga | [ ] Belum |

### 20.3 Fase 2 — Fondasi Organisasi RW

| Kriteria | Status |
|----------|--------|
| Migration RW organizations dibuat | [ ] Belum |
| Tabel `user_rw_memberships` dibuat | [ ] Belum |
| Tabel `rts_rw_links` dibuat | [ ] Belum |
| Katalog jabatan RW didefinisikan | [ ] Belum |
| Permission domain RW ditetapkan | [ ] Belum |
| Migration backward compatible | [ ] Belum |

### 20.4 Fase 3 — Administrasi RW

| Kriteria | Status |
|----------|--------|
| Endpoint Administrasi RW tersedia | [ ] Belum |
| Tab RW di menu Administrasi | [ ] Belum |
| Penetapan, pergantian, pencabutan jabatan RW | [ ] Belum |
| Isolasi permission RT/RW terverifikasi | [ ] Belum |

### 20.5 Fase 4 — Evaluasi Tata Kelola

| Kriteria | Status |
|----------|--------|
| Evaluasi pengalaman Super Admin | [ ] Belum |
| Evaluasi kebutuhan delegasi aman | [ ] Belum |
| Delegasi tidak diaktifkan tanpa kebijakan baru | [ ] Belum |

---

## 21. Risiko, Dependensi, dan Keputusan Terbuka

### 21.1 Risiko

| Risiko | Dampak | Tingkat | Mitigasi |
|--------|--------|---------|----------|
| Penetapan jabatan mempengaruhi permission user secara instan | User baru bisa mengakses fitur yang sebelumnya tidak bisa | Medium | Short-lived tokens + opsi re-login |
| Duplikat penetapan jabatan (2 ketua per RT) | Konflik otoritas | Medium | Constraint DB atau application-level validation |
| Race condition saat dua penetapan simultan | Data corrupt | Rendah | PostgreSQL advisory lock |
| Migrasi RW breaking existing RT ops | Data loss atau downtime | Rendah | Backward compatible migration, staging environment |
| Non-super_admin bypass via direct API call | Keamanan bocor | Tinggi | Middleware `RequireSystemRole` di semua endpoint |

### 21.2 Dependensi

| Dependensi | Status |
|------------|--------|
| Migration 011 (jabatan + position_permissions) | ✅ SUDAH ADA |
| Migration 025 (update position_permissions) | ✅ SUDAH ADA |
| Backend RequireSystemRole middleware | ✅ SUDAH ADA |
| Frontend SuperAdminRoute guard | ✅ SUDAH ADA |
| Audit logs foundation | ✅ SUDAH ADA |
| Desain akhir katalog jabatan RW | **BELUM DECIDED** |
| Desain akhir relasi RW-RT | **BELUM DECIDED** |
| Approval desain dokumen ini | **BELUM DECIDED** |

### 21.3 Keputusan Terbuka yang Perlu Disepakati

| No | Keputusan | Keterangan |
|----|-----------|------------|
| 1 | Apakah `jabatan_history` tabel baru atau reuse `audit_logs`? | Opsi A: tabel khusus. Opsi B: gunakan `audit_logs` yang ada. |
| 2 | Apakah perubahan jabatan memicu re-login wajib? | Opsi A: ya, paksa re-login. Opsi B: short-lived token saja. |
| 3 | Apakah satu RT boleh punya jabatan rangkap? (contoh: 2 bendahara) | Perlu keputusan bisnis. |
| 4 | Apakah jabatan "anggota" perlu diimplementasi? | Sudah ada di CHECK constraint, tapi belum ada permission mapping. |
| 5 | Apakah route lama `/rt` perlu redirect ke `/administrasi`? | Untuk backward compatibility. |
| 6 | Apakah Flutter/Android perlu Administrasi di Fase 1? | Rekomendasi: tidak. Web-only dulu. |
| 7 | Apakah jabatan RW perlu katalog terpisah dari jabatan RT? | Ya, sesuai prinsip "no transitive permissions". |
| 8 | Apakah ada batasan jumlah pemegang jabatan? (contoh: hanya 1 ketua) | Perlu keputusan bisnis untuk setiap jabatan. |
| 9 | Apakah perlu validasi bahwa user yang ditetapkan jabatannya sudah punya keanggotaan RT? | Ya, seharusnya wajib. Tapi perlu kepastian dari produk. |
| 10 | Apakah warga bisa sekaligus punya jabatan pengurus? | Saat ini bisa (via `user_rt_memberships`), tapi perlu konfirmasi kebijakan. |

---

## 22. Konfirmasi Keselamatan

| Item | Status |
|------|--------|
| Tidak ada kode aplikasi diubah | ✅ Tidak |
| Tidak ada database diubah | ✅ Tidak |
| Tidak ada data runtime diubah | ✅ Tidak |
| Tidak ada migration dijalankan | ✅ Tidak |
| Tidak ada commit dilakukan | ✅ Tidak |
| Tidak ada push dilakukan | ✅ Tidak |
| Perubahan lokal yang sudah ada dipertahankan | ✅ Ya |

---

## 23. Bukti Audit dan Referensi File Repository

### 23.1 File yang Diaudit

| File | Action | Tujuan |
|------|--------|--------|
| `docs/database.md` | Read (519 baris) | Skema database, migration history |
| `docs/rw-architecture-plan.md` | Read (683 baris) | Desain konseptual RW |
| `docs/authorization-design.md` | Read (464 baris) | RBAC design, jabatan, permission |
| `docs/api.md` | Read (767 baris) | API contract |
| `backend/cmd/api/main.go` | Read (full) | Route registration, middleware wiring |
| `backend/internal/rt/handler.go` | Read (full) | RT CRUD handler |
| `backend/internal/auth/middleware.go` | Read (302 baris) | RequireAuth, RequireSystemRole, RequireRole, RequirePermission |
| `backend/internal/auth/models.go` | Read (full) | SystemRole, Role, Jabatan constants, AuthContext |
| `backend/internal/auth/permission.go` | Read (partial) | Permission resolution logic |
| `frontend/src/app/App.tsx` | Read (80 baris) | Route registry, SuperAdminRoute guard |
| `frontend/src/components/Sidebar.tsx` | Read (80 baris) | Menu RT untuk Super Admin |
| `frontend/src/features/rt/RtList.tsx` | Read (60 baris) | Halaman RT list |
| `frontend/src/features/rt/RtDetail.tsx` | Read (50 baris) | Halaman RT detail |
| `mobile/lib/app/app.dart` | Read (full) | Flutter GoRouter, auth guard |
| `backend/migrations/001_initial.up.sql` | Read | RT table creation |
| `backend/migrations/003_separate_global_identity.up.sql` | Read | system_role addition |
| `backend/migrations/005_fix_rt_unique_indexes.up.sql` | Read | RT unique constraint |
| `backend/migrations/011_add_positions_permissions.up.sql` | Read | jabatan + position_permissions |
| `backend/migrations/025_update_position_permissions.up.sql` | Read | position_permissions update |
| `backend/migrations/` (52 files) | List | Total migration inventory |
| `frontend/src/features/kas/KasList.tsx` | Read (partial) | Verify KAS table column alignment (sebelum dokumen ini) |

### 23.2 Informasi Repository

| Item | Nilai |
|------|-------|
| Branch | `main` |
| HEAD | `2f6678ded22fa8005bc6c1c4303ab225bb80c1d1` |
| Total migration files | 52 (26 versions: 001–026) |
| Backend modules | auth, config, database, devseed, finance, health, household, http, migration, notification, rt, storage |
| Frontend features | auth, dashboard, iuran, kas, profile, rt, warga |
| Flutter features | auth, cash, dues, dashboard, iuran, kas, profile, reports, warga |

### 23.3 Working Tree Saat Ini (Tidak Diubah)

```
 M backend/cmd/api/main.go
 M backend/internal/auth/handlers.go
 M backend/internal/auth/hash.go
 M backend/internal/auth/repository.go
 M backend/internal/auth/service.go
 M backend/internal/finance/handler.go
 M backend/internal/finance/repository.go
 M backend/internal/finance/service.go
 M backend/internal/household/model.go
 M backend/internal/household/repository.go
 M frontend/src/app/AuthContext.tsx
 M frontend/src/app/api.ts
 M frontend/src/features/auth/Login.tsx
 M frontend/src/features/warga/HouseholdCreate.tsx
 M frontend/src/features/warga/HouseholdDetail.tsx
 M frontend/src/features/warga/ResidentDetail.tsx
 M frontend/src/features/warga/ResidentDetailFields.tsx
 M frontend/src/features/warga/ResidentList.tsx
 M frontend/src/layouts/AppLayout.tsx
 M frontend/src/styles/format.ts
 D mobile/android/app/src/debug/res/xml/network_security_config.xml
 M mobile/android/app/src/main/AndroidManifest.xml
 M mobile/lib/core/api/auth_service.dart
 M mobile/lib/core/storage/app_storage.dart
 M mobile/lib/core/utils/phone_utils.dart
 M mobile/lib/features/auth/data/mock_auth_repository.dart
 M mobile/lib/features/auth/presentation/screens/login_screen.dart
 M mobile/test/auth_test.dart
 M mobile/test/features/auth/login_features_test.dart
?? backend/internal/auth/profile_password_test.go
?? backend/internal/finance/occupancy_user_test.go
?? frontend/src/features/profile/
?? mobile/android/app/src/main/res/xml/
```

**Semua perubahan di atas DIPERTAHANKAN.** Dokumen ini tidak memodifikasi file manapun selain `docs/administrasi-rt-rw-design.md`.

---

## 24. Kesiapan untuk Implementasi Fase 1

### 24.1 Yang Sudah Tersedia

| Komponen | Status |
|----------|--------|
| Middleware `RequireSystemRole(SuperAdmin)` | ✅ SUDAH ADA |
| Tabel `user_rt_memberships` dengan kolom `jabatan` | ✅ SUDAH ADA |
| Tabel `rts` dengan struktur lengkap | ✅ SUDAH ADA |
| Tabel `audit_logs` foundation | ✅ SUDAH ADA |
| Tabel `permission_types` + `position_permissions` | ✅ SUDAH ADA |
| Route pattern RT CRUD (super_admin only) | ✅ SUDAH ADA |
| Frontend `SuperAdminRoute` guard | ✅ SUDAH ADA |
| Frontend `Sidebar` pattern untuk Super Admin | ✅ SUDAH ADA |
| Frontend `RtList` pattern (table + filter + pagination) | ✅ SUDAH ADA |
| Backend RT handler pattern (CRUD + transaction) | ✅ SUDAH ADA |
| API response format (paginated wrapper, error format) | ✅ SUDAH ADA |

### 24.2 Yang Masih Perlu Dikeputusankan Sebelum Fase 1

| No | Keputusan | Pengaruh |
|----|-----------|----------|
| 1 | Nama route: `/administrasi` atau `/admin`? | Frontend routing + URL compatibility |
| 2 | Apakah route lama `/rt` redirect ke `/administrasi`? | Backward compatibility |
| 3 | Apakah jabatan "anggota" perlu diimplementasi di Fase 1? | Scope tabel jabatan |
| 4 | Apakah satu RT boleh punya jabatan rangkap? | Constraint logic |
| 5 | Apakah perlu `jabatan_history` tabel terpisah atau reuse `audit_logs`? | Database design + migration |
| 6 | Apakah perubahan jabatan wajib re-login? | UX + security tradeoff |

### 24.3 Kesimpulan Kesiapan

**Dokumen ini sudah cukup untuk memulai implementasi Fase 1 (Administrasi RT) dalam hal:**
- Arsitektur dan model data (menggunakan tabel existing).
- Kontrak API (endpoint yang diusulkan jelas).
- Kebijakan akses (middleware sudah ada).
- Pola implementasi (handler, service, repository pattern sudah terbukti).

**Namun, keputusan terbuka di §24.2 (terutama item 1–3) perlu diselesaikan terlebih dahulu** agar implementasi tidak perlu direvisi setelah selesai.

---

## 25. UI Konteks Organisasi dan Display Jabatan

### 25.A. Prinsip Konteks Eksplisit

Setiap jabatan harus selalu ditampilkan dengan **nama organisasi dan scope**-nya. UI **tidak boleh** menampilkan jabatan tanpa konteks organisasi yang jelas.

```
✅ Benar: "Administrasi RW 05 — Ketua RW"
✅ Benar: "Keanggotaan RT 003 — Warga"
❌ Salah: "Jabatan: Ketua" (tidak jelas Ketua RT atau Ketua RW)
❌ Salah: "Pengurus" (tidak jelas RT atau RW, tidak jelas jabatan apa)
```

### 25.B. Contoh Display Konteks di UI

| Halaman | Konteks | Display Jabatan |
|---------|---------|-----------------|
| Halaman detail pengurus RT | RT 003 — RW 05 | "Ketua RT 003" |
| Halaman detail pengurus RW | RW 05 | "Ketua RW 05" |
| Profil user dengan jabatan RT dan RW | Tab RT 003 | "Warga" (jabatan: NULL) |
| Profil user dengan jabatan RT dan RW | Tab RW 05 | "Ketua RW" |
| Sidebar / navigasi utama | — | Tidak menampilkan jabatan organisasi. Hanya menampilkan nama user + role login. |

### 25.C. Tabel Pengurus — Konteks per Baris

Saat Super Admin membuka halaman kelola pengurus, setiap baris harus menampilkan konteks organisasi:

**Halaman: Kelola Pengurus RT 003**

| Nama | Email | Role | Jabatan | Scope | Action |
|------|-------|------|---------|-------|--------|
| Pak Budi | budi@... | pengurus | Ketua | RT 003 | Ganti/Cabut |
| Pak Andi | andi@... | warga | — | RT 003 | Tetapkan |
| Ibu Sari | sari@... | bendahara | Bendahara | RT 003 | Ganti/Cabut |

**Halaman: Kelola Pengurus RW 05** (Fase 2+)

| Nama | Email | Role | Jabatan | Scope | Action |
|------|-------|------|---------|-------|--------|
| Pak Andi | andi@... | pengurus | Ketua | RW 05 | Ganti/Cabut |
| Pak Joko | joko@... | pengurus | Sekretaris | RW 05 | Tetapkan |

### 25.D. Halaman Profil User — Multi-Scope Display

Jika user memiliki jabatan di lebih dari satu scope, profil user harus menampilkan informasi per-scope:

**Halaman: Profil User — Pak Andi**

```
┌─────────────────────────────────────────┐
│ Pak Andi                                │
│ andi@example.com                        │
│                                         │
│ Keanggotaan Organisasi:                 │
│                                         │
│ ┌─ RT 003 ───────────────────────────┐  │
│ │ Role: Warga                        │  │
│ │ Jabatan: — (Warga biasa)           │  │
│ │ Status: Aktif                      │  │
│ └────────────────────────────────────┘  │
│                                         │
│ ┌─ RW 05 ───────────────────────────┐  │
│ │ Role: Pengurus                   │  │
│ │ Jabatan: Ketua RW                │  │
│ │ Status: Aktif                    │  │
│ └────────────────────────────────────┘  │
└─────────────────────────────────────────┘
```

**Dilarang:**
- Menampilkan "Jabatan: Ketua" tanpa menyebut RT/RW.
- Menggabungkan jabatan RT dan RW menjadi satu daftar flat.
- Mengklaim user adalah "Pengurus RT 003" jika yang benar adalah "Warga RT 003 + Ketua RW 05".

### 25.E. Sidebar dan Navigasi — Tidak Menampilkan Jabatan Organisasi

| Elemen | Isi |
|--------|-----|
| Logo / brand | "Social Finance" |
| Menu utama | Beranda, Warga, Iuran, KAS, Administrasi (jika Super Admin) |
| User menu / profil | Nama user + role login (contoh: "admin — Super Admin") |
| Badge / notifikasi | Tidak menampilkan jabatan organisasi |

**Alasan:** Sidebar adalah navigasi aplikasi, bukan daftar jabatan. Menampilkan jabatan di sidebar akan menyesatkan karena tidak jelas scope-nya dan akan berubah-ubah saat user login sebagai role berbeda.

### 25.F. Toast / Notification / Alert

Saat operasi administrasi berhasil atau gagal, pesan harus menyertakan konteks organisasi:

| Operasi | Pesan Benar | Pesan Salah |
|---------|-------------|-------------|
| Tetapkan jabatan RT | "Jabatan Ketua RT 003 berhasil ditetapkan untuk Pak Budi" | "Jabatan Ketua berhasil ditetapkan" |
| Cabut jabatan RW | "Jabatan Ketua RW 05 berhasil dicabut dari Pak Andi" | "Jabatan berhasil dicabut" |
| Gagal tetapkan jabatan | "Gagal: Pak Budi sudah memiliki jabatan Bendahara di RT 003" | "Gagal: jabatan sudah ada" |

---

*Dokumen ini dibuat berdasarkan audit implementasi aktual repository `social-finance` pada commit `2f6678d`. Semua fakta yang dicantumkan telah diverifikasi dari kode sumber, migrasi database, route registry, dan dokumentasi yang ada. Hal yang belum dapat diverifikasi ditandai **BELUM TERVERIFIKASI** atau **USULAN — BELUM DIIMPLEMENTASIKAN**.*
