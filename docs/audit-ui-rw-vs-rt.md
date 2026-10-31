# Laporan Audit UI — RW vs RT (Web & Android)

**Tanggal:** 2026-10-30  
**Status:** READ-ONLY — tidak ada file yang dimodifikasi  
**Fokus:** Perbandingan UI tiga menu utama: BERANDA, KAS, LOKASI DANA

---

## 1. Temuan Utama: Struktur Fitur

### WEB
| Fitur | Lokasi |
|-------|--------|
| Beranda (Dashboard) | `frontend/src/features/dashboard/Dashboard.tsx` |
| Kas | `frontend/src/features/kas/KasList.tsx` (dan turunan) |
| Lokasi Dana | `frontend/src/features/fund_location/FundLocationList.tsx` |
| Kelola RT | `frontend/src/features/rt/RtList.tsx` + `RtDetail.tsx` |
| Kelola RW | `frontend/src/features/rt/RwList.tsx` (modals) + `RwDetailModal.tsx` |
| Warga | `frontend/src/features/warga/` |

### ANDROID
| Fitur | Lokasi |
|-------|--------|
| Beranda (Dashboard) | `mobile/lib/features/dashboard/presentation/screens/dashboard_screen.dart` |
| Kas | `mobile/lib/features/kas/presentation/screens/kas_screen.dart` |
| Lokasi Dana | `mobile/lib/features/fund_location/presentation/screens/fund_location_screen.dart` |
| **Kelola RT** | **TIDAK ADA** — tidak ada folder `features/rt/` |
| Kelola RW | `mobile/lib/features/rw/presentation/screens/rw_list_screen.dart` |
| Warga | `mobile/lib/features/warga/presentation/screens/` |

### TEMUAN KRITIS
- **Android TIDAK memiliki fitur Kelola RT/Warga seperti Web.** Tidak ada folder `features/rt/` di Android.
- Semua fitur operasional di Android (Kas, Iuran, Warga, Lokasi Dana) **dipakai sebagai fitur RT** — tidak ada layer RW terpisah di Android untuk fitur-fitur ini.
- RW di Android hanya memiliki `rw_list_screen.dart` — satu halaman daftar RW untuk Super Admin.

---

## 2. Audit Perbandingan: BERANDA (Dashboard)

### WEB — `frontend/src/features/dashboard/Dashboard.tsx`
- **Baris:** 565 baris total
- **Komponen:** DashboardSummary API → `apiGetDashboardSummary()`
- **Komponen UI:**
  - `PageHeader` — greeting + role badge
  - `sf-metrics-row` — 3 metric cards (Saldo Kas, Pemasukan, Pengeluaran)
  - `FinancialChart` — chart 12 bulan (recharts Pie/ResponsiveContainer)
  - Donut chart (Recharts) untuk metode pembayaran (Transfer vs Tunai)
  - Progress bar untuk Iuran
  - Tabel transaksi terbaru
- **Filter periode:** Bulan + Tahun (select dropdowns), tombol "Semua Periode"
- **Data source:** API `apiGetDashboardSummary(token, periodParam)`
- **Scope data:** `user?.rt?.id` — selalu scoped ke RT user saat login

### ANDROID — `mobile/lib/features/dashboard/presentation/screens/dashboard_screen.dart`
- **Baris:** 857 baris total
- **Komponen UI:**
  - `MenuAppBarTitle` — judul "Beranda"
  - SummaryCard — metric cards (Saldo Kas, Pemasukan, Pengeluaran)
  - `FinancialSummaryChart` — chart 12 bulan (fl_chart)
  - Donut chart (fl_chart) untuk metode pembayaran
  - Progress bar untuk Iuran
  - `KasTransactionCard` — list transaksi terbaru
- **Filter periode:** Chevron buttons + DatePicker + ChoiceChip "Semua"
- **Data source:** API via `kasApiClientProvider.getDashboardSummary()`
- **Scope data:** `user?['rt']` + `user?['rw']` — menampilkan RT/RW user

### PERBANDINGAN DASHBOARD

| Aspek | Web | Android | Kesimpulan |
|-------|-----|---------|------------|
| Komponen chart | Recharts (Pie, ResponsiveContainer) | fl_chart | REUSE dengan adapter — struktur data API sama |
| Metric cards | `sf-metric-card` CSS-based | `SummaryCard` widget | Layout berbeda tapi konten identik |
| Iuran progress | Custom div + CSS progress bar | Custom Column + AnimatedContainer | Layout serupa, bisa reuse logic |
| Donut chart | Recharts Pie + Cell | fl_chart PieChart | Struktur data sama, komponen berbeda |
| Filter periode | Select bulan + select tahun + button | Chevron + DatePicker + ChoiceChip | UX berbeda tapi fungsi sama |
| Transaksi terbaru | Tabel HTML (6 kolom) | `KasTransactionCard` (list vertical) | Responsif — wajar untuk web vs mobile |
| **RW-specific?** | **TIDAK** — dashboard sama untuk semua role RT | **TIDAK** — dashboard sama untuk semua role RT | **REUSE DIRECT** — tidak ada perbedaan data atau tampilan |

### HASIL AUDIT BERANDA
**REUSE DIRECT — TIDAK ADA DIFERENSIASI**

Dashboard sama sekali tidak membedakan RW vs RT. API yang dipanggil adalah `apiGetDashboardSummary()` yang sama. Tidak ada parameter `rw_id` atau `rt_id` yang membedakan scope.

**Rekomendasi:** Tidak ada perubahan diperlukan. Dashboard adalah fitur RT-level yang juga dipakai oleh semua role RW.

---

## 3. Audit Perbandingan: KAS

### WEB — `frontend/src/features/kas/KasList.tsx`
- **Baris:** ~59,095 baris (file terbesar di project)
- **Komponen utama:**
  - `KasList` — daftar transaksi dengan filter, search, pagination
  - `KasIncome` — form pemasukan
  - `KasExpense` — form pengeluaran
  - `KasDetail` — detail transaksi
  - `KasReport` — laporan kas
- **Routing:** `kasRoutes.tsx` → `/kas`, `/kas/baru`, `/kas/baru/keluar`, `/kas/:id`
- **API:** `kas-api.ts` — `getKasTransactions`, `createKasIncome`, `createKasExpense`, dll.
- **Filter:** Periode (bulan/tahun), filter arus (masuk/keluar), filter metode (transfer/cash), search
- **Akses:** `AllowedRTRoute` — dibatasi untuk role RT-level

### ANDROID — `mobile/lib/features/kas/presentation/screens/kas_screen.dart`
- **Baris:** ~1,795 baris
- **Komponen utama:**
  - `KasScreen` — daftar transaksi dengan filter
  - `kas_detail_screen.dart` — detail transaksi
  - `kas_masuk_screen.dart` — form pemasukan
  - `kas_keluar_screen.dart` — form pengeluaran
  - `kas_report_screen.dart` — laporan kas
- **Routing:** Via GoRouter — `/home/kas`, `/home/kas/:id`, `/home/kas/masuk`, `/home/kas/keluar`
- **API:** `kas_repository.dart`, `kas_api_client.dart`
- **Filter:** Periode, filter arus, filter metode, search
- **Akses:** `isAllowedRT(user)` — dibatasi untuk role RT-level

### PERBANDINGAN KAS

| Aspek | Web | Android | Kesimpulan |
|-------|-----|---------|------------|
| Komponen daftar | Tabel + toolbar filter (horizontal) | List + filter expanded (vertical) | Wajar — responsive pattern berbeda |
| Form transaksi | Halaman terpisah (KasIncome/KasExpense) | Halaman terpisah (kas_masuk_screen/kas_keluar_screen) | REUSE dengan adapter |
| Detail transaksi | KasDetail.tsx (page) | kas_detail_screen.dart | REUSE dengan adapter |
| Laporan kas | KasReport.tsx | kas_report_screen.dart | REUSE dengan adapter |
| API endpoint | `/api/v1/kas/*` | `/api/v1/kas/*` | SAMA — endpoint identik |
| Filter logic | Bulan/tahun + arus + metode + search | Sama | REUSE DIRECT — logic sama |
| **RW-specific?** | **TIDAK** — kas diakses via `/kas` (same route) | **TIDAK** — kas diakses via `/home/kas` (same route) | **REUSE DIRECT** |

### HASIL AUDIT KAS
**REUSE DIRECT — TIDAK ADA DIFERENSIASI**

KAS menggunakan endpoint API yang sama (`/api/v1/kas/*`) untuk semua user, terlepas dari apakah mereka akses sebagai RT atau RW. Tidak ada parameter `rw_id` di route atau query. Semua filter dan logic identik.

**Rekomendasi:** Tidak ada perubahan diperlukan. Kas adalah fitur RT-level yang juga dipakai oleh semua role RW.

---

## 4. Audit Perbandingan: LOKASI DANA

### WEB — `frontend/src/features/fund_location/FundLocationList.tsx`
- **Baris:** ~1,273 baris
- **Komponen utama:**
  - `FundLocationList` — single component untuk semua tab
  - Tab: "Daftar Lokasi" + "Riwayat Transfer"
  - Summary cards: Rekening Bank, Brankas Fisik, Cash on Hand
  - Form create/edit location
  - Form transfer dana
  - Form opening balance
  - Reconciliation card
  - History modal
- **Routing:** `fundLocationRoutes.tsx` → `/lokasi-dana/*`
- **API:** `api.ts` — `apiListFundLocations`, `apiCreateFundLocation`, `apiCreateFundTransfer`, dll.
- **Akses:** `AllowedRTRoute` — blocked for warga, allowed untuk pengurus/bendahara/super_admin
- **Mutasi:**
  - Create location (Pengurus + Bendahara + SA)
  - Edit location (Pengurus + Bendahara + SA)
  - Deactivate location (Pengurus + Bendahara + SA)
  - Transfer dana (hanya Bendahara + SA)
  - Opening balance (Pengurus + Bendahara + SA)
  - Reverse transfer (hanya Bendahara + SA)

### ANDROID — `mobile/lib/features/fund_location/presentation/screens/fund_location_screen.dart`
- **Baris:** ~2,199 baris
- **Komponen utama:**
  - `FundLocationScreen` — TabView (Locations + Transfers)
  - Summary cards (3 kategori)
  - Reconciliation card
  - Form dialog (add location, transfer, opening balance)
  - FAB (floating action button) untuk tambah lokasi
- **Routing:** Via GoRouter — `/home/lokasi-dana`
- **API:** `fund_location_repository.dart`, `fund_location_api_client.dart`
- **Akses:** `isAllowedRT(user)` — warga diblokir
- **Mutasi:**
  - Create location — FAB (semua manager)
  - Transfer — button inline (Bendahara + SA)
  - Opening balance — button di reconciliation card (semua manager)
  - Reverse transfer — dari history transfer (Bendahara + SA)

### PERBANDINGAN LOKASI DANA

| Aspek | Web | Android | Kesimpulan |
|-------|-----|---------|------------|
| Struktur | Single page dengan tab | TabView dengan TabBar | REUSE dengan adapter |
| Summary cards | Grid 3 kolom | Row 3 kolom | Layout berbeda (responsive), konten sama |
| Reconciliation | Card di tab locations | Card di tab locations | REUSE dengan adapter |
| Form create | Modal (Web) vs Dialog (Android) | Dialog (Android) | Pattern berbeda, konten sama |
| Form transfer | Modal (Web) | Dialog (Android) | Pattern berbeda, konten sama |
| Summary API | `apiListFundLocations` (RT-scoped) | `fundLocationsProvider` (RT-scoped) | **SAMA API** |
| Aksi mutasi | Button di page header + inline | FAB + inline buttons | Pattern berbeda, aksi sama |
| **RW-specific?** | **TIDAK** — lokasi dana diakses via `/lokasi-dana` (same route) | **TIDAK** — lokasi dana diakses via `/home/lokasi-dana` (same route) | **REUSE DIRECT** |

### HASIL AUDIT LOKASI DANA
**REUSE DIRECT — TIDAK ADA DIFERENSIASI**

Lokasi Dana menggunakan endpoint API yang sama (`/api/v1/fund_locations/*`) untuk semua user, tanpa parameter `rw_id`. Tidak ada perbedaan tampilan atau fitur antara akses RT vs RW.

**Rekomendasi:** Tidak ada perubahan diperlukan. Lokasi Dana adalah fitur RT-level yang juga dipakai oleh semua role RW.

---

## 5. Temuan Penting: Tidak Ada RW-Specific UI di Kedua Platform

### Web
- **Tidak ada folder `features/rw/` di Web.**
- Semua konten RW (Daftar RW, Kelola RW) berada di dalam `features/rt/` — `RwList.tsx` dan `RwDetailModal.tsx`.
- RW di Web hanya bisa diakses oleh **Super Admin** (`/rw` route guarded by `SuperAdminRoute`).
- Role Pengurus/Bendahara/Warga **TIDAK** memiliki akses ke fitur RW-specific (RW management).
- Fitur operasional (Beranda, Kas, Lokasi Dana) adalah fitur RT-level yang dipakai oleh semua role RT/RW.

### Android
- **Tidak ada folder `features/rt/` di Android.**
- Semua fitur operasional (Kas, Iuran, Warga, Lokasi Dana, Beranda) adalah fitur RT-level yang dipakai oleh semua role.
- RW di Android hanya memiliki satu halaman: `rw_list_screen.dart` — daftar RW untuk Super Admin.
- Semua fitur operasional di Android **tidak membedakan** akses RT vs RW — semuanya menggunakan route yang sama.

### Navigasi
| Platform | RW Menu | RT Menu | Lokasi |
|----------|---------|---------|--------|
| Web | Sidebar: "Kelola RW" (SA only) | Sidebar: "Kelola RT" (SA only) | `/rw`, `/rt` |
| Android | Bottom nav: tidak ada | Bottom nav: tidak ada | Hanya `rw_list_screen.dart` via route |

---

## 6. Matriks Reuse

| Menu | Platform | RT UI | RW UI | Hasil |
|------|----------|-------|-------|-------|
| **Beranda** | Web | `Dashboard.tsx` (line 1-565) | **TIDAK ADA** — sama dengan RT | **REUSE DIRECT** |
| **Beranda** | Android | `dashboard_screen.dart` (line 1-857) | **TIDAK ADA** — sama dengan RT | **REUSE DIRECT** |
| **KAS** | Web | `KasList.tsx` (line 1-59095) | **TIDAK ADA** — sama dengan RT | **REUSE DIRECT** |
| **KAS** | Android | `kas_screen.dart` (line 1-1795) | **TIDAK ADA** — sama dengan RT | **REUSE DIRECT** |
| **Lokasi Dana** | Web | `FundLocationList.tsx` (line 1-1273) | **TIDAK ADA** — sama dengan RT | **REUSE DIRECT** |
| **Lokasi Dana** | Android | `fund_location_screen.dart` (line 1-2199) | **TIDAK ADA** — sama dengan RT | **REUSE DIRECT** |

---

## 7. Daftar Issue Berdasarkan Severity

### BLOCKING (0)
Tidak ada.

### HIGH (0)
Tidak ada.

### MEDIUM (3)
| # | Issue | Severity | Platform | Deskripsi |
|---|-------|----------|----------|-----------|
| 1 | **RW Management UI hanya untuk Super Admin** | MEDIUM | Web | `RwList.tsx` dan `RwDetailModal.tsx` hanya bisa diakses Super Admin. Jika Pengurus RT perlu mengelola RW di wilayahnya, perlu authorization check baru. |
| 2 | **Tidak ada RW-specific menu di navigasi Android** | MEDIUM | Android | Android tidak memiliki menu/navigasi untuk fitur RT-level. Bottom nav hanya menampilkan fitur operasional (Beranda, Kas, Iuran, dll.) tanpa indikasi konteks RT vs RW. |
| 3 | **Tidak ada indicator konteks RW di UI** | MEDIUM | Web+Android | Dashboard/Kas/Lokasi Dana tidak menampilkan informasi bahwa user sedang beroperasi di konteks RW (jika user adalah pengurus RW, bukan Ketua RT). Tidak ada header/banner yang menunjukkan konteks. |

### LOW (4)
| # | Issue | Severity | Platform | Deskripsi |
|---|-------|----------|----------|-----------|
| 4 | **Duplikasi code chart — Recharts vs fl_chart** | LOW | Web+Android | Web menggunakan Recharts, Android menggunakan fl_chart. Struktur data sama tapi implementasi komponen berbeda. Bisa distandardisasi di future refactor. |
| 5 | **Summary card pattern berbeda di Android** | LOW | Android | Summary card di Android (`SummaryCard` widget) tidak konsisten dengan pattern Card di halaman lain (misalnya dashboard menggunakan container custom, bukan `SummaryCard`). |
| 6 | **Tidak ada empty state yang konsisten** | LOW | Web+Android | Web menggunakan `sf-empty-banner`, Android menggunakan `EmptyState` widget. Pattern berbeda. |
| 7 | **Loading state tidak seragam** | LOW | Web+Android | Web menggunakan `sf-loading-banner`, Android menggunakan `CircularProgressIndicator` di `Center`. |

### INFO (2)
| # | Issue | Severity | Platform | Deskripsi |
|---|-------|----------|----------|-----------|
| 8 | **File KasList.tsx terlalu besar (59,095 baris)** | INFO | Web | File terbesar di project. Memudahkan pencarian tapi menyulitkan maintenance. |
| 9 | **FundLocationList.tsx juga besar (1,273 baris)** | INFO | Web | Satu component menangani semua tab, filter, forms, dan modals. Bisa dipecah. |

---

## 8. Kesimpulan

### Status Audit: **UI CONSISTENT**

Ketiga menu utama (Beranda, Kas, Lokasi Dana) di **kedua platform (Web dan Android) menggunakan implementasi yang sama untuk RT dan RW**. Tidak ada perbedaan implementasi karena:

1. **API yang digunakan identik** — endpoint `/api/v1/kas/*`, `/api/v1/fund_locations/*`, dan `/api/v1/dashboard-summary` tidak membeda-bedakan konteks RW vs RT di level UI.
2. **Tidak ada komponen RW-specific** — tidak ada file terpisah untuk "RwDashboard", "RwKas", atau "RwLokasiDana".
3. **Authorization enforced di backend** — scope RT/RW di-control melalui JWT claim (`rt_id`) dan middleware backend, bukan di UI.
4. **UI adaptif sesuai role, bukan konteks RW/RT** — perbedaan UI di kedua platform terjadi berdasarkan role (warga vs pengurus vs bendahara vs super_admin), bukan berdasarkan apakah user mengakses konteks RW atau RT.

### Rekomendasi
Tidak ada perubahan UI yang diperlukan untuk konsistensi RW vs RT karena **RW tidak memiliki layer UI terpisah** — semua fitur operasional adalah fitur RT-level yang juga berlaku untuk RW.

### Catatan Penting
- Audit ini **TIDAK** menyertakan backend PR 4 (sudah diaudit terpisah).
- Audit ini **TIDAK** menyertakan fitur Warga, Iuran (selain yang muncul di dashboard), atau Notifikasi.
- File attachment yang disertakan dalam handoff tidak dapat diakses (file tidak tersedia di filesystem sandbox).
