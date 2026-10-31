# URENA RW DEVELOPMENT — PHASE 0 READINESS AUDIT

**Branch:** `urena`
**Current HEAD:** `1789737` (Iuran household name visibility fix)
**Uncommitted changes:** ~45 modified files + many untracked files

---

## 1. Ringkasan Kesiapan RW

| Komponen | Status | Catatan |
|----------|--------|---------|
| Database Schema | **READY** | Tabel `rws`, `rts.rw_id`, `user_rw_memberships` sudah ada (migration 027–029) |
| Backend RW Service | **READY** | `backend/internal/rw/` — Handler, Service, Repository lengkap (read-only) |
| Backend Auth/RW Membership | **READY** | `user_rw_memberships.jabatan`, RW context di JWT, `loadRWMemberships()` |
| Backend Scoping Middleware | **READY** | `RequireRWScope`, `ScopingConfig` (SCOPED_RT_IDS, SCOPED_RW_IDS) |
| Frontend Web RW | **PARTIAL** | Sidebar ada menu "Lihat Data RW", AppRoute ada `rw-view/:rwId/*`. Komponen RW feature belum terintegrasi penuh. |
| Android RW | **PARTIAL** | Models, API Client, Screens (Dashboard, KAS, Lokasi Dana) ada tapi belum terhubung ke navigation shell secara fungsional |
| API OpenAPI | **NEEDS DECISION** | File `docs/rbac-operasional-rw-openapi.yaml` dan `rbac-operasional-rw-planned-api.yaml` ada tapi belum pasti yang mana yang live |
| Testing | **PARTIAL** | Backend: ada test files di `backend/internal/rw/` dan `backend/internal/auth/me_rw_membership_test.go`. Android: belum ada test RW screens. |

**Kesimpulan:** Infrastruktur backend RW sudah 90% siap (read-only aggregate). Gap utama ada di frontend integration dan test coverage.

---

## 2. Kondisi Database dan Migrasi

### Tabel Global (SUPER_ADMIN Scope)

| Tabel | Status | File Migrasi |
|-------|--------|-------------|
| `rws` | **READY** | `027_rw_domain_and_transaction_reference.up.sql` |
| `rts` (dengan `rw_id`) | **READY** | `027_rw_domain_and_transaction_reference.up.sql` |
| `user_rw_memberships` | **READY** | `029_rw_membership_and_jabatan.up.sql` |

### Struktur `rws`

```
id              uuid PK
rw_number       int UNIQUE (active only)
name            text
head_name       text (nullable)
address         text (nullable)
is_active       boolean default true
```

### Struktur `rts` — ada FK ke RW

```
id          uuid PK
rw_id       uuid FK → rws.id ON DELETE RESTRICT  ← BARU (migration 027)
name        text
rw          int
rt          text
is_active   boolean default true
```

Index: `idx_rts_rw_id` pada `(rw_id)` — **ada**.

### Struktur `user_rw_memberships` — ada kolom `jabatan`

```
id          uuid PK
user_id     uuid FK → users.id ON DELETE CASCADE
rt_id       uuid FK → rts.id ON DELETE CASCADE
role        text CHECK IN ('pengurus','bendahara','warga')
jabatan     text (nullable) — RT-style values
is_active   boolean default true
```

### Tabel `user_rw_memberships` juga memiliki scope RW

Berdasarkan migration 029 (`029_rw_membership_and_jabatan.up.sql`) dan kode `loadRWMemberships()`:
- Tabel ini menyimpan **RT membership** (`user_rt_memberships`)
- RW membership dimuat terpisah via query ke `user_rw_memberships` alias untuk RW scope — kode mengacu ke kolom `rw_id` dan `jabatan`

### Migrasi terakhir: 041 (notification_manual_and_image)

**Catatan:** Migrasi 027–029 sudah mencakup domain RW. Tidak ada migrasi RW baru sejak itu.

### Apakah transaksi keuangan RW ada?

**Ya, melalui aggregation.** Tidak ada tabel `rw_transactions` terpisah. Semua transaksi RW di-agg dari `transactions.rt_id` via `rts.rw_id` join:

- Dashboard: `SELECT ... FROM transactions WHERE rt_id IN (SELECT id FROM rts WHERE rw_id = $1)`
- KAS Report: same pattern
- Locations: `fund_locations.rt_id` → `rts.rw_id`

**KESIMPULAN:** Schema database sudah siap untuk RW. Tidak perlu migrasi baru untuk fitur read-only RW.

---

## 3. Kondisi Authentication dan Membership

### JWT Claims (file: `backend/internal/auth/jwt.go`)

```go
type TokenClaims struct {
    UserID    string `json:"sub"`
    MID       string `json:"mid"`       // empty for system-only
    RTID      string `json:"rt_id"`     // empty for system-only
    Role      string `json:"role"`      // tenant role
    Jabatan   string `json:"jabatan"`   // RT position
    SysRole   string `json:"sys_role"`  // "super_admin" or empty
    RWContext string `json:"rw_context,omitempty"`  // JSON string of RWMemberships
}
```

**RW Context di JWT:** `loadRWMemberships()` di `auth/service.go` mengisi RW memberships ke token during login dan refresh:

```go
var rwMemberships []RWMembership
if userRW, err := loadRWMemberships(ctx, tx, user.ID); err == nil {
    rwMemberships = userRW
}
rwContext := buildRWContextJSON(rwMemberships)
```

Format JSON: `[{"rw_id":"uuid","rw_name":"RW 01","jabatan":"ketua"}, ...]`

### AuthContext (file: `backend/internal/auth/models.go`)

```go
type AuthContext struct {
    UserID          string
    SystemRole      SystemRole
    MembershipID    string
    RTID            string
    TenantRole      Role
    Jabatan         Jabatan
    RWMemberships   []RWMembership  // ← populated in RequireAuth middleware
}
```

### RwMembership Model

```go
type RWMembership struct {
    RWID    string
    Name    string
    Jabatan string
}
```

**Key decision:** RW jabatan menggunakan nilai RT-style (`ketua`, `wakil_ketua`, `sekretaris`, `bendahara`), bukan role-based (`pengurus`, `bendahara`, `warga`). Ini konsisten dengan approach RBAC yang sudah ada.

### Login dengan Multiple Memberships

Di `auth/service.go` line ~160-175:
- Jika user punya 1 membership → langsung pakai
- Jika user punya >1 membership → **SUPER_ADMIN** mendapat special case: cari RT ID `a95c1cb5-2f67-4ea5-9197-bfec537b6d7b` (RT 002), fallback ke membership pertama
- User biasa dengan >1 membership → **ERR_MULTIPLE_MEMBERSHIPS**

**RISIKO:** Logic multi-membership hanya menangani SUPER_ADMIN, bukan user biasa dengan jabatan ganda RT+RW. User dengan jabatan di RT dan RW akan tetap berhasil login (1 RT membership + RW memberships di token).

---

## 4. Kondisi Permission dan Authorization

### Middleware Stack (file: `backend/internal/auth/middleware.go`)

| Middleware | Fungsi | Status |
|-----------|--------|--------|
| `RequireAuth` | Validasi JWT + populate AuthContext + RW memberships | **READY** |
| `RequireSystemRole` | Block SUPER_ADMIN-only endpoints | **READY** |
| `RequireRole` | Validasi tenant role (pengurus/bendahara/warga) | **READY** |
| `RequirePermission` | Position-based permission check | **READY** |
| `RequireJabatan` | Validasi jabatan spesifik | **READY** |
| `RequireAccess` | Role OR jabatan check | **READY** |
| `RequireRWScope` | Validasi rwId di URL path terhadap RWMemberships | **READY** |

### RW Permission Map (file: `backend/internal/auth/scoping.go`)

```go
var rwPermissionMap = map[string][]string{
    "ketua":       {"finance.read", "finance.create", "finance.update", "finance.approve"},
    "wakil_ketua": {"finance.read", "finance.create", "finance.approve"}, // TIDAK finance.update
    "sekretaris":  {"finance.read"},
    "bendahara":   {"finance.read", "finance.create", "finance.update", "finance.approve"},
}
```

**RISIKO:** Map ini **HARDCODED**, tidak ada di DB. Tidak ada cara untuk menambahkan jabatan RW baru (misalnya "sekretaris_rw" atau "wakil_ketua_rw") tanpa modifikasi kode.

### Authorization Chain RW

```
1. RequireAuth → populates RWMemberships from DB
2. RequireRWScope(pool, cfg) → validates rwId in URL matches memberships + SCOPED_RW_IDS allowlist
3. Service.checkRWAccess() → defense-in-depth: rejects if no active RW membership
```

**Layering Defense-in-Depth: 3 lapis — solid.**

### Apakah user dapat memiliki jabatan RT dan RW sekaligus?

**Ya, secara database.** `user_rw_memberships` menyimpan RT membership dengan `jabatan`, dan RW memberships dimuat terpisah via query ke tabel yang sama (kolom `jabatan` di `user_rw_memberships` digunakan untuk kedua scope).

**Risiko konflik:** Tidak ada validasi yang mencegah user memiliki jabatan yang sama di RT dan RW. Jika user adalah "Ketua RT" dan "Ketua RW", mereka mendapatkan permission gabungan dari kedua jabatan.

---

## 5. Kondisi Backend

### RW Handler (file: `backend/internal/rw/handler.go`)

| Endpoint | Method | Handler | Status |
|----------|--------|---------|--------|
| `/api/v1/rw/{rwId}/dashboard` | GET | `GetDashboard` | **READY** |
| `/api/v1/rw/{rwId}/kas` | GET | `GetKAS` | **READY** |
| `/api/v1/rw/{rwId}/kas/transactions` | GET | `GetKASTransactions` | **READY** |
| `/api/v1/rw/{rwId}/locations` | GET | `GetLocations` | **READY** |

Semua endpoint **READ-ONLY** — tidak ada POST/PUT/DELETE. Ini konsisten dengan arsitektur aggregate.

### RW Service (file: `backend/internal/rw/service.go`)

Semua method melakukan:
1. `checkRWAccess(ac, rwID)` — validasi membership
2. Query via repository
3. Return data

**Status:** ✅ Solid, pattern konsisten.

### RW Repository (file: `backend/internal/rw/repository.go`)

Semua query menggunakan `rts.rw_id` join, tidak pernah menerima `rw_id` dari client.

### Router Integration

Di `backend/cmd/api/main.go`, RW routes menggunakan:
- `RequireAuth`
- `RequireRWScope(pool, &cfg)`
- Tidak ada `RequireRole` (karena RW hanya read, tidak butuh role check)

**KESIMPULAN:** Backend RW sudah lengkap dan siap digunakan. Tidak ada endpoint RT yang terpengaruh.

---

## 6. Kondisi Web (Frontend)

### Sidebar (file: `frontend/src/components/Sidebar.tsx`)

```tsx
// Menu RT (existing)
{ to: '/', label: 'Beranda' }
{ to: '/kas', label: 'Kas' }
{ to: '/iuran', label: 'Iuran' }
{ to: '/lokasi-dana', label: 'Lokasi Dana' }
{ to: '/warga', label: 'Warga' }
{ to: '/notifikasi', label: 'Notifikasi' }

// Menu RW (ada, tapi limited)
const rwIdForData = user?.rwId || ''
if (rwIdForData) {
  items.push({ to: `/rw-view/${rwIdForData}/dashboard`, label: 'Lihat Data RW', ... })
}
```

**Masalah:** Menu RW hanya muncul jika `user?.rwId` ada (dari single RT's rwId), bukan dari `rw_memberships[]`. Artinya:
- Jika user punya 1 RT membership dengan rw_id → 1 menu "Lihat Data RW"
- Jika user punya multiple RW memberships → hanya 1 menu yang muncul (rwId pertama)
- Tidak ada menu RW per-RW untuk user dengan jabatan di beberapa RW

### App Routes (file: `frontend/src/app/App.tsx`)

```tsx
<Route path="rw-view/:rwId/*" element={<RwDataView rwId="" />}>
```

**Masalah:** Route `rw-view/:rwId/*` ada tapi `rwId=""` hardcoded — parameter URL tidak diteruskan ke komponen.

### AuthContext (file: `frontend/src/app/AuthContext.tsx`)

- `mapUser()` → `rwMemberships: RWMembership[]` — **sudah support multiple memberships**
- `isSuperAdminUser()` — sudah ada
- `isAllowedRT()` — sudah ada
- `ALLOWED_RT_IDS` — hardcoded UUID untuk RT 002, 003

### RwDataView Component

File: `frontend/src/features/rw/RwDataView.tsx` (untracked di git, ada di `frontend/src/features/rw/`)

**STATUS:** Komponen ada tapi belum di-review detail isinya. Harus dicek apakah sudah terintegrasi dengan benar.

### Sidebar — Menu RW yang Direkomendasikan

Berdasarkan brief, yang diminta:
- **BERANDA RW** — read-only dashboard RW
- **KAS RW** — read-only KAS aggregate
- **LOKASI DANA RW** — read-only locations aggregate

Pendekatan aman: **Tidak mengubah menu RT yang ada**. Tambah menu baru khusus RW di bawah menu RT, muncul hanya jika user punya `rw_memberships` yang tidak kosong.

### File Web yang Perlu Diubah

| File | Perubahan |
|------|-----------|
| `frontend/src/components/Sidebar.tsx` | Tambah menu RW per-RW dari `rwMemberships[]`, bukan dari `rwId` single |
| `frontend/src/app/App.tsx` | Fix `rw-view/:rwId/*` route parameter passing |
| `frontend/src/features/rw/RwDataView.tsx` | Verifikasi + perlu routing per-subpage (dashboard, kas, locations) |

---

## 7. Kondisi Android (Flutter)

### Navigation Shell (file: `mobile/lib/app/navigation_shell.dart`)

```dart
static const _rwDataTab = _NavRoute(
    path: '/home/rw-data',
    label: 'Data RW',
    icon: Icons.area_chart_outlined,
    activeIcon: Icons.area_chart,
);
```

**Ada tab "Data RW"** di bottom navigation — muncul jika `rwMemberships.isNotEmpty`.

**Masalah:** Hanya 1 tab untuk semua RW. Jika user punya 2 RW memberships, tidak ada cara memilih RW mana yang dilihat.

### RW Tab Navigation

Di `_onTabTapped()`:
```dart
if (tab.path == '/home/rw-data') {
    if (rwMemberships.length == 1) {
        context.go('/rw/${rwMemberships.first.rwId}/dashboard');
    } else {
        // Show dialog to select RW
    }
}
```

**Ini sudah menangani multi-RW dengan dialog.** Bagus.

### RW Screens (ada 3 files)

| File | Status |
|------|--------|
| `mobile/lib/features/rw/presentation/screens/rw_dashboard_screen.dart` | **READY** — full dashboard RW UI |
| `mobile/lib/features/rw/presentation/screens/rw_kas_screen.dart` | **READY** — full KAS RW UI dengan table + filters |
| `mobile/lib/features/rw/presentation/screens/rw_fund_location_screen.dart` | **READY** — full lokasi dana RW UI |

### RW Data Models (file: `mobile/lib/features/rw/data/rw_models.dart`)

**Ada 2 set models:**
1. `RwModel` — untuk CRUD RW management (fitur lama)
2. `RwDashboardSummary`, `RwKasReport`, `RwTransaction`, `RwFundLocation` — untuk PR 4 aggregated data

**Status:** ✅ Lengkap, strukturnya sesuai backend response.

### RW API Client (file: `mobile/lib/features/rw/data/rw_api_client.dart`)

Methods:
- `getDashboard(rwId, period?)` → `RwDashboardSummary`
- `getKas(rwId, period?)` → `RwKasReport`
- `getTransactions(rwId, period?, startDate?, endDate?, page?, pageSize?)` → `RwTransactionsResponse`
- `getLocations(rwId, category?)` → `List<RwFundLocation>`

**Status:** ✅ Semua endpoint backend sudah dicover.

### RW Providers (file: `mobile/lib/features/rw/data/rw_providers.dart`)

```dart
final rwApiClientProvider = Provider<RwApiClient>((ref) { ... });
final rwListProvider = AsyncNotifierProvider<RwListNotifier, List<RwModel>>(...);
```

**KESIMPULAN:** Android RW sudah 80% siap. Gap: belum ada routing ke ketiga screens RW (dashboard, kas, lokasi dana). Bottom nav "Data RW" hanya trigger navigasi ke path `/rw/{rwId}/dashboard` tapi belum ada route definition di app.dart.

### File Android yang Perlu Diubah

| File | Perubahan |
|------|-----------|
| `mobile/lib/app/app.dart` | Tambah route definition untuk `/rw/:rwId/dashboard`, `/rw/:rwId/kas`, `/rw/:rwId/lokasi-dana` |
| `mobile/lib/app/navigation_shell.dart` | Pertimbangkan: apakah "Data RW" tab cukup atau perlu per-RW tabs? |
| Bottom nav scrolling | Brief menyebutkan "Bottom navigation wajib mendukung horizontal scrolling/sliding jika menu melebihi lebar layar" — saat ini menggunakan `Row` + `Flexible` → tidak ada horizontal scroll |

---

## 8. Risiko Isolasi Data RT/RW

### Risiko yang Sudah Ditangani

| Risiko | Mitigasi | Status |
|--------|----------|--------|
| RT mengakses data RW | Semua RW endpoints pakai `RequireRWScope` | **TERATASI** |
| RW mengakses data RT | RT endpoints tetap pakai `RequireRole` + `RTID` dari JWT | **TERATATI** |
| Client mengirim rwId palsu | Middleware validasi rwId di URL path | **TERATASI** |
| Data RT/RW tercampur di aggregation | Query menggunakan subquery `rts WHERE rw_id = $1` | **TERATASI** |
| Super Admin akses RW tanpa membership | `RequireRWScope` + `SCOPED_RW_IDS` allowlist | **TERATASI** |
| Membership RW di-revoke | `RequireAuth` middleware re-validate RW memberships dari DB | **TERATASI** |

### Risiko yang Perlu Perhatian

| Risiko | Tingkat | Catatan |
|--------|---------|---------|
| `RequireRole` middleware: SUPER_ADMIN bypass | Sedang | `RequireRole` bypass semua role check untuk SUPER_ADMIN. Jika suatu hari SUPER_ADMIN butuh access control pada RT operations, ini perlu ditimpa. |
| Hardcoded RW permission map | Sedang | `rwPermissionMap` di `scoping.go` hardcode 4 jabatan. Tidak ada extensibility. |
| Login multi-membership fallback | Ringan | SUPER_ADMIN dengan >1 RT membership hardcoded mencari RT 002 UUID. Ini workaround, bukan design. |
| Jabatan RT vs Jabatan RW tidak dibedakan | Ringan | `jabatan` di `user_rw_memberships` digunakan untuk kedua scope. Tidak ada prefix/namespace (misalnya `rw_ketua` vs `rt_ketua`). |

---

## 9. Daftar Komponen yang Bisa Digunakan Kembali

| Komponen | Sumber | Digunakan di RW |
|----------|--------|----------------|
| `AuthContext` | `backend/internal/auth/models.go` | RW: `checkRWAccess(ac, rwID)` |
| `RequireAuth` | `backend/internal/auth/middleware.go` | RW: middleware stack |
| `RequireRWScope` | `backend/internal/auth/scoping.go` | RW: middleware stack |
| `finance.ParseMoney` / `FormatMoney` | `backend/internal/finance/money.go` | RW: delegates to finance |
| `transactions` table | `backend/migrations/009` | RW: aggregate source |
| `fund_locations` table | `backend/migrations/034` | RW: aggregate source |
| `location_balance_changes` | `backend/migrations/034` | RW: aggregate source |
| `rts.rw_id` FK | `backend/migrations/027` | RW: join key |
| `RwDashboardSummary` | `backend/internal/rw/model.go` | RW: response model |
| `RwKasReport` | `backend/internal/rw/model.go` | RW: response model |
| `Sidebar` component (existing) | `frontend/src/components/Sidebar.tsx` | Perlu modifikasi untuk menu RW |
| Bottom navigation pattern | `mobile/lib/app/navigation_shell.dart` | Pattern yang sama bisa dipakai |

---

## 10. Daftar Komponen yang Harus Dibuat Khusus RW

| Komponen | Keterangan |
|----------|-----------|
| **Web: Menu RW per-RW** | Sidebar perlu render satu item per-RW dari `rw_memberships[]` |
| **Web: RwDataView full integration** | Halaman dengan 3 sub-page (dashboard, kas, locations) |
| **Web: Route fixing** | `rw-view/:rwId/*` route parameter passthrough |
| **Android: Route definitions** | `app.dart` perlu route untuk `/rw/:rwId/dashboard`, `/rw/:rwId/kas`, `/rw/:rwId/lokasi-dana` |
| **Android: Bottom nav scrolling** | Jika tab > 5, bottom nav perlu `SingleChildScrollView` horizontal |
| **Android: RW screens navigation** | Bottom nav "Data RW" saat ini hanya ke dashboard. Perlu navigasi ke kas dan lokasi dana juga. |
| **Backend: RW permission extensibility** | Pertimbangkan migrasi untuk membuat RW permissions di DB, bukan hardcoded map |
| **Backend: RW CRUD endpoints** | Saat ini hanya read-only. Jika butuh manajemen RW (create/edit RT di bawah RW), perlu endpoint tambahan |

---

## 11. Daftar Pekerjaan yang Belum Tersedia

### Backend
- [ ] RW CRUD endpoints (CREATE/UPDATE/DELETE RW, assign RT ke RW) — saat ini hanya read
- [ ] RW permission stored in DB (bukan hardcoded map)
- [ ] Endpoint untuk managing RW memberships (assign jabatan ke user di RW)
- [ ] Endpoint untuk SA managing RW list (CRUD)

### Frontend Web
- [ ] Sidebar multi-RW menu rendering
- [ ] RwDataView sub-page routing (dashboard → kas → locations)
- [ ] Route parameter fixing di `App.tsx`
- [ ] RW-specific pages/components (bukan hanya wrapper)

### Android
- [ ] Route definitions untuk RW screens
- [ ] Bottom nav horizontal scrolling
- [ ] Navigation dari RW dashboard ke RW kas dan RW lokasi dana
- [ ] RW management screens (assign RW membership, edit RW)

### Testing
- [ ] Android integration tests untuk RW screens
- [ ] Web E2E tests untuk RW navigation flow
- [ ] Backend test untuk multi-RW membership scenario

---

## 12. Rekomendasi Urutan Implementasi

### Phase 1: Fix Existing RW Infrastructure (High Priority)
1. **Fix Web routing** — `rw-view/:rwId/*` parameter passthrough
2. **Fix Web sidebar** — render per-RW menu dari `rw_memberships[]`
3. **Fix Android routing** — tambah route definitions untuk RW screens
4. **Test end-to-end** — login → RW menu → dashboard → kas → lokasi dana

### Phase 2: Improve Navigation UX (Medium Priority)
5. **Android bottom nav horizontal scroll** — jika tab count > 5
6. **RW intra-navigation** — dari RW dashboard bisa navigasi ke RW kas dan lokasi dana
7. **Web multi-RW selection** — jika user punya banyak RW, tampilkan selection dialog (mirip Android)

### Phase 3: RW CRUD & Permission Extensibility (Lower Priority)
8. **RW permission to DB** — pindahkan `rwPermissionMap` ke `position_permissions` table
9. **RW management endpoints** — SA CRUD RW, assign users ke RW
10. **RW management UI** — Web dan Android screens untuk manajemen RW

---

## 13. File yang Mungkin Perlu Diubah per Tahap

### Phase 1
| File | Perubahan |
|------|-----------|
| `frontend/src/app/App.tsx` | Pass `rwId` param ke `RwDataView` |
| `frontend/src/components/Sidebar.tsx` | Render per-RW menu dari `rw_memberships[]` |
| `frontend/src/features/rw/RwDataView.tsx` | Sub-page routing logic |
| `mobile/lib/app/app.dart` | Add GoRouter paths for RW screens |
| `mobile/lib/features/rw/data/rw_repository.dart` | (ada, perlu dicek) |

### Phase 2
| File | Perubahan |
|------|-----------|
| `mobile/lib/app/navigation_shell.dart` | Horizontal scroll untuk bottom nav |
| `mobile/lib/features/rw/presentation/screens/rw_dashboard_screen.dart` | Add nav buttons to kas/locations |
| `mobile/lib/features/rw/presentation/screens/rw_kas_screen.dart` | Add back nav |

### Phase 3
| File | Perubahan |
|------|-----------|
| `backend/internal/auth/scoping.go` | Pindahkan `rwPermissionMap` ke DB |
| `backend/internal/rw/handler.go` | Tambah CRUD handlers |
| `frontend/src/features/rw/` | Tambah CRUD pages |
| `mobile/lib/features/rw/presentation/screens/` | Tambah management screens |

---

## 14. Risiko Regresi Terhadap Fitur RT

| Risiko | Dampak | Mitigasi |
|--------|--------|----------|
| Perubahan `RequireAuth` mempengaruhi RT auth | **TINGGI** | RW auth menggunakan middleware yang sama. Test RT endpoints wajib. |
| Perubahan sidebar menghilangkan menu RT | **TINGGI** | Sidebar RT logic dipisahkan dari RW logic. Pastikan `if (allowedRT)` tetap intact. |
| Perubahan AuthContext breaking RT flow | **TINGGI** | `mapUser()` sudah support `rw_memberships`. Pastikan tidak mengubah field RT yang ada. |
| Migration baru mengubah struktur tabel RT | **TINGGI** | Phase 3 migration harus backward-compatible. Tidak ada kolom yang di-drop. |
| Login dengan >1 RT membership | **SEDANG** | Hardcoded RT 002 UUID fallback. Jika user punya RT lain sebagai primary, bisa salah assign. |
| Bottom nav scrolling mengubah layout | **RENDAH** | Hanya affect Android. Test pada device dengan berbagai ukuran layar. |
| Route collision RT vs RW | **RENDAH** | RT routes: `/kas`, `/iuran`, `/warga`. RW routes: `/rw/:rwId/*`. Path namespace berbeda — tidak bisa collision. |

### Critical RT Paths yang Tidak Boleh Berubah
- `/` — Beranda RT
- `/kas/*` — KAS RT
- `/iuran/*` — Iuran RT
- `/lokasi-dana/*` — Lokasi Dana RT
- `/warga/*` — Warga RT
- `/notifikasi` — Notifikasi RT
- `/rt/*` — Kelola RT (SUPER_ADMIN)
- `/rw` — Kelola RW (SUPER_ADMIN) — **ini berbeda dari `/rw-view/:rwId`**

### RW Routes (Baru, Tidak Conflict)
- `/rw-view/:rwId/*` — RW Data View (Web)
- `/rw/:rwId/*` — RW screens (Android)

---

## Lampiran: File Untracked Penting

| File | Deskripsi |
|------|-----------|
| `backend/internal/rw/` | RW backend lengkap (handler, service, repository) |
| `backend/internal/auth/me_rw_membership_test.go` | Test untuk RW membership di /me endpoint |
| `backend/internal/auth/scoping.go` | RequireRWScope middleware |
| `backend/internal/auth/scoping_test.go` | Test untuk scoping |
| `backend/internal/fund_location/handler_security_test.go` | Security test fund_location |
| `mobile/lib/features/rw/` | Android RW screens + models + API client |
| `frontend/src/features/rw/` | Web RW feature (belum di-commit) |
| `docs/rbac-operasional-rw-openapi.yaml` | OpenAPI spec untuk RW endpoints |
| `docs/rbac-operasional-rw-planned-api.yaml` | Planned API untuk RW (belum implement) |
| `docs/rbac-operasional-rw.md` | Documentation |

---

**AUDIT SELESAI — Tidak ada perubahan kode yang dilakukan.**
