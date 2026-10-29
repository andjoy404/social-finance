# Laporan Deep-Dive Audit — Iuran Warga (Android + Backend + SQL)

Tanggal: 2026-10-28  
Penulis: Audit berbasis kode sumber  
Status: **Audit only — tidak ada perubahan file**

---

## 1. Executive Summary

### Kondisi Aktual

Sistem Iuran Warga saat ini menggunakan arsitektur **hybrid pagination**: backend (`GET /api/v1/bills`) **mendukung pagination server-side** dengan `LIMIT/OFFSET`, parameter filter lengkap (`search`, `period`, `status`, `payment_status`, `due_id`), dan perhitungan `paid_amount` langsung via subquery SQL. Namun, **Android tidak memanfaatkan kemampuan ini**.

Di sisi Android, `fetchAllBills()` di `iuran_repository.dart` melakukan loop halaman demi halaman (`pageSize: 100`) hingga seluruh dataset terkumpul, lalu melakukan **seluruh filtering dan sorting di Dart** (`_filterBills()` di `iuran_providers.dart`). Ini berarti:
- Semua tagihan untuk seluruh RT dimuat ke memori perangkat warga.
- 100+ rumah × 3 jenis iuran = 300-500+ baris tagihan untuk satu RT.
- Filtering periode, status, dan pencarian dilakukan setelah seluruh data ada di client.

### Masalah Paling Penting

| # | Temuan | Tingkat Dampak | Prioritas |
|---|--------|---------------|-----------|
| 1 | Client-side fetch-all + sorting/filtering membuang bandwidth dan memori | **Major** | P1 |
| 2 | N+1 query: `listPayments()` dipanggil per-bill setelah `fetchBills()` | **Medium** | P2 |
| 3 | `paid_amount` dihitung via correlated subquery di BE (`SELECT SUM(...) FROM payments WHERE bill_id = ?`) untuk setiap baris | **Medium** | P2 |
| 4 | Tidak ada unit test untuk filter/sort logic di Android | **Medium** | P3 |
| 5 | Pagination di UI Android tidak terimplementasi (single-screen ListView tanpa infinite scroll) | **Medium** | P3 |

### Rekomendasi Awal

**Langkah 1 (cepat):** Aktifkan server-side pagination di Android — ganti `fetchAllBills()` untuk `Iuran Warga` menjadi `fetchBills()` yang menggunakan pagination dengan `period` sebagai filter server-side.  
**Langkah 2:** Optimasi correlated subquery `paid_amount` di `BillList` dengan JOIN aggregation atau materialized approach.  
**Langkah 3:** Tambahkan unit test untuk `_filterBills()`.

---

## 2. Evidence Matrix

### Temuan 1 — Android Fetch-All vs Backend Pagination

| Temuan | Kepastian | Path | Dampak |
|--------|-----------|------|--------|
| `IuranRepository.fetchAllBills()` melakukan loop halaman hingga totalPages terkumpul, kemudian filter/sort di Dart | **Fakta** | `mobile/lib/features/iuran/data/iuran_repository.dart:248-274` | Bandwidth + memori terbuang untuk dataset yang tidak difilter |
| `BillsNotifier._fetch()` memanggil `fetchAllBills()` dengan `pageSize: 100` | **Fakta** | `mobile/lib/features/iuran/data/iuran_repository.dart:258` | Minimal 1 halaman data dimuat per request |
| Backend `GET /api/v1/bills` mendukung `page`, `page_size`, `period`, `search`, `status`, `payment_status` | **Fakta** | `backend/internal/finance/handler.go:600-630` | Backend siap, tapi tidak dipakai |
| `billsProvider` dan `myBillsProvider` menggunakan `BillsNotifier` yang memanggil `fetchAllBills()` | **Fakta** | `mobile/lib/features/iuran/data/iuran_providers.dart:98-104` | Filter/periode hanya di client |

### Temuan 2 — Billing Data Model dan UI List Item

| Temuan | Kepastian | Path | Dampak |
|--------|-----------|------|--------|
| Satu item `IuranBill` = satu tagihan individual (bukan ringkasan per rumah) | **Fakta** | `mobile/lib/features/iuran/data/iuran_models.dart`; `iuran_screen.dart:627` (_IuranCard menerima `IuranBill`) | Multi-Bill menghasilkan 12 item terpisah per rumah |
| `_IuranCard` menampilkan household name hanya jika `!isMyBill` | **Fakta** | `iuran_screen.dart:655-663` | Warga tidak melihat nama rumah orang lain |
| ListView tanpa pagination/infinite scroll | **Fakta** | `iuran_screen.dart:454-478` | Tidak ada "load more" saat scroll ke bawah |

### Temuan 3 — SQL Query dan Index

| Temuan | Kepastian | Path | Dampak |
|--------|-----------|------|--------|
| `BillList` menggunakan correlated subquery untuk `paid_amount` dan `pending_amount` | **Fakta** | `backend/internal/finance/repository.go:527-528` | Per-baris subquery = O(n) × cost per payment lookup |
| JOINs: `bills → dues → household_occupancies → households → physical_houses` | **Fakta** | `backend/internal/finance/repository.go:517-523` | 5-table join untuk setiap halaman |
| Index `idx_bills_rt_id`, `idx_bills_occupancy_id`, `idx_bills_due_id`, `idx_bills_status`, `idx_bills_period` | **Fakta** | `backend/migrations/009_finance.up.sql:45-50` | Index yang ada mendukung sebagian besar filter, tapi tidak ada composite index untuk `(rt_id, period, status)` |
| `BillCount` menggunakan query terpisah untuk COUNT total | **Fakta** | `backend/internal/finance/repository.go:590-638` | 2 query per halaman (COUNT + SELECT) — overhead tambahan |

### Temuan 4 — Volume Data

| Temuan | Kepastian | Path | Dampak |
|--------|-----------|------|--------|
| Regular: 1 bill per rumah per periode | **Fakta** | `service.go:219-253` (GenerateBills) | 150 rumah × 12 bulan = 1800 bills/tahun |
| Multi-Bulan: 12 bills per rumah per tahun (Jan-Des) | **Fakta** | `service.go:145-194` (generateMultiMonthBills) | 150 rumah × 12 = 1800 bills/tahun (satu periode per bulan) |
| Sukarela: 1 bill per transaksi/periode | **Fakta** | `service.go:219-253` | Tidak bisa diproyeksikan dengan rumus tetap — bergantung jumlah transaksi |

### Temuan 5 — Token Isolation & Security

| Temuan | Kepastian | Path | Dampak |
|--------|-----------|------|--------|
| `rt_id` selalu berasal dari JWT claim (`ac.RTID`), bukan dari parameter HTTP | **Fakta** | `handler.go:600-610` | Keamanan tenant isolation terjaga |
| Warga dibatasi ke occupancy sendiri via `GetOccupancyIDForUser()` | **Fakta** | `handler.go:698-709` | Warga tidak bisa melihat data rumah lain |
| `my_bills=true` memaksa filter ke occupancy pengguna | **Fakta** | `handler.go:698` | Konsisten di seluruh jalur |

---

## 3. Alur Data: Android → Database

```
Android UI (IuranScreen)
  └─> billsProvider / myBillsProvider (Riverpod)
       └─> BillsNotifier._fetch()
            └─> repository.fetchAllBills(myBills: bool)
                 └─> API Client listBills(page, pageSize=100)  ← Loop halaman
                      └─> HTTP GET /api/v1/bills?page=1&page_size=100&period=xxx&my_bills=true
                           └─> Handler.ListBills
                                └─> BillCount() + BillList()
                                     └─> SQL: 5-table JOIN + 2 correlated subqueries per row
                                          └─> PostgreSQL
                           └─> Response → Android
                      └─> HTTP GET /api/v1/bills?page=2&page_size=100&...
                           └─> (loop hingga totalPages)
                 └─> Client-side filter (_filterBills)
                 └─> Client-side sort
  └─> ListView.builder → _IuranCard
```

**Titik pagination diterapkan:**
- Backend: ✅ Benar (OFFSET + LIMIT)
- Android: ❌ Pagination dimuat tetapi semua halaman digabung sebelum dikirim ke UI

**Titik bottleneck:**
1. `fetchBills()` dipanggil beberapa kali (loop halaman) sebelum data sampai ke UI.
2. Setiap bill memanggil `listPayments()` secara terpisah untuk menghitung `paidAmount` di repository layer Android.

---

## 4. Endpoint dan SQL

### Endpoint Aktual

| Method | Path | Parameter | Pagination | Tenant Scope |
|--------|------|-----------|-----------|--------------|
| GET | `/api/v1/bills` | `page`, `page_size`, `due_id`, `status`, `payment_status`, `period`, `household_occupancy_id`, `my_bills`, `search`, `due_name`, `is_active` | ✅ `LIMIT/OFFSET`, max `page_size=100` | ✅ JWT `rt_id` |

### Query SQL Utama (BillList)

```sql
SELECT b.id, b.rt_id, rt.rt as rt_code,
       b.household_occupancy_id, b.due_id, b.amount::text, b.period, b.due_date,
       b.status, b.is_active, b.created_at, b.updated_at,
       d.name as due_name, d.is_active as due_is_active,
       ph.house_number, h.head_name, d.is_voluntary,
       COALESCE((SELECT SUM(p.amount) FROM payments p
                   WHERE p.bill_id = b.id AND p.status = 'APPROVED'), 0)::text as paid_amount,
       COALESCE((SELECT SUM(p.amount) FROM payments p
                   WHERE p.bill_id = b.id AND p.status = 'PENDING'), 0)::text as pending_amount,
       d.due_type, d.year
  FROM bills b
  JOIN dues d ON b.due_id = d.id
  JOIN household_occupancies ho ON b.household_occupancy_id = ho.id
  JOIN households h ON ho.household_id = h.id
  LEFT JOIN physical_houses ph ON ho.physical_house_id = ph.id
  LEFT JOIN rts rt ON b.rt_id = rt.id
 WHERE b.rt_id = $1
   [AND filters...]
 ORDER BY ... OFFSET $N LIMIT $M
```

### Subquery per Baris (Correlated)
Dua subquery per baris hasil:
1. `SELECT SUM(p.amount) FROM payments p WHERE p.bill_id = b.id AND p.status = 'APPROVED'`
2. `SELECT SUM(p.amount) FROM payments p WHERE p.bill_id = b.id AND p.status = 'PENDING'`

Ini berarti untuk 20 bill per halaman → **40 subquery tambahan**.

### Indeks yang Ditemukan

| Tabel | Kolom | Indeks | Bukti |
|-------|-------|--------|-------|
| `bills` | `rt_id` | `idx_bills_rt_id` | `009_finance.up.sql:46` |
| `bills` | `household_occupancy_id` | `idx_bills_occupancy_id` | `009_finance.up.sql:47` |
| `bills` | `due_id` | `idx_bills_due_id` | `009_finance.up.sql:48` |
| `bills` | `status` | `idx_bills_status` | `009_finance.up.sql:49` |
| `bills` | `period` | `idx_bills_period` | `009_finance.up.sql:50` |
| `bills` | `(household_occupancy_id, due_id, period)` | UNIQUE constraint | `009_finance.up.sql:43` |
| `payments` | `bill_id` | `idx_payments_bill_id` | `009_finance.up.sql:79` |
| `payments` | `status` | `idx_payments_status` | `009_finance.up.sql:80` |
| `dues` | `rt_id` | `idx_dues_rt_id` | `009_finance.up.sql:31` |
| `dues` | `(rt_id, is_active)` | `idx_dues_rt_active` | `009_finance.up.sql:32` |
| `financial_categories` | `rt_id`, `(rt_id, is_active)` | `idx_financial_categories_*` | `009_finance.up.sql:12-13` |

### Kekurangan Index

Tidak ada composite index yang mendukung filter gabungan yang umum:
- `(rt_id, period, status)` — untuk filter periode + status yang umum di `Iuran Warga`.
- `(bill_id, status)` — untuk correlated subquery `paid_amount`.

### Execution Plan

**Status: Belum diukur.**  
Tidak ada akses ke database live untuk menjalankan `EXPLAIN ANALYZE`. Rekomendasi: jalankan pada database test/staging dengan minimal 500 baris bills.

---

## 5. Estimasi Volume Data

### A. Regular (Reguler)

**Model:** Satu tagihan per rumah per periode.  
Jika `period_type = 'monthly'`, tagihan dibuat 1 kali per periode (manual generate).  
Tidak ada batasan jumlah periode — bisa beberapa periode dalam setahun jika pengurus membuat beberapa kali.

- Rumus: `jumlah_rumah × jumlah_periode_per_tahun`
- Asumsi: 12 periode/tahun (bulanan)
- 150 rumah: 150 × 12 = **1.800 bills**
- 300 rumah: 300 × 12 = **3.600 bills**
- 500 rumah: 500 × 12 = **6.000 bills**
- 1.000 rumah: 1.000 × 12 = **12.000 bills**

### B. Multi-Bulan

**Model:** 12 tagihan otomatis (Jan–Des) saat iuran dibuat dengan `year` tertentu.  
Semua 12 bulan dibuat sekaligus di `generateMultiMonthBills()`.

- Rumus: `jumlah_rumah × 12`
- 150 rumah: 150 × 12 = **1.800 bills**
- 300 rumah: 300 × 12 = **3.600 bills**
- 500 rumah: 500 × 12 = **6.000 bills**
- 1.000 rumah: 1.000 × 12 = **12.000 bills**

### C. Sukarela (Voluntary)

**Model:** Tagihan dibuat per transaksi — setiap pembayaran donasi membuat satu tagihan baru.  
Tidak bisa diproyeksikan dengan rumus tetap karena bergantung pada frekuensi donasi.

- Rumus: `jumlah_rumah × rata_rata_transaksi_per_tahun`
- Asumsi konservatif: 2 transaksi/rumah/tahun
- 150 rumah: 150 × 2 = **300 bills** (kecil)
- 300 rumah: 300 × 2 = **600 bills**
- 500 rumah: 500 × 2 = **1.000 bills**
- 1.000 rumah: 1.000 × 2 = **2.000 bills**
- **Kepastian: Rendah** — bergantung pada perilaku donasi warga

### Gabungan (150 rumah, asumsi 1 jenis iuran aktif per rumah)

| Skenario | Regular | Multi-Bulan | Sukarela | Total |
|----------|---------|-------------|----------|-------|
| 150 rumah | 1.800 | 1.800 | ~300 | ~3.900 |
| 300 rumah | 3.600 | 3.600 | ~600 | ~7.800 |
| 500 rumah | 6.000 | 6.000 | ~1.000 | ~13.000 |
| 1.000 rumah | 12.000 | 12.000 | ~2.000 | ~26.000 |

### Catatan Penting

- Bills sudah termasuk `paid_amount` dan `pending_amount` dalam query BE (via correlated subquery), jadi Android tidak perlu memanggil `listPayments()` per bill untuk menampilkan jumlah yang sudah dibayar.
- Namun, `fetchBills()` di repository Android **secara eksplisit memanggil** `apiClient.listPayments(billId: bill.id)` untuk setiap bill — menggantikan data yang sudah ada dari BE. Ini adalah **double-fetch** yang tidak perlu.

---

## 6. Analisis UI: Bentuk Daftar

### Apakah Daftar Saat Ini Berisi Tagihan Individual?

**Fakta: Satu item = satu tagihan individual.**

Bukti dari kode:
- `filteredBillsProvider` mengembalikan `List<IuranBill>` di mana setiap `IuranBill` mewakili satu baris di tabel `bills`.
- `_IuranCard` di `iuran_screen.dart:627` menerima satu `IuranBill` dan menampilkan: household name, iuran type, periode, nominal, paid amount, pending amount.
- 12 tagihan Multi-Bulan (Jan–Des) ditampilkan sebagai **12 item terpisah** di ListView.

### Perbandingan Opsi A vs B

#### Opsi A — Daftar Tagihan Individual (Saat Ini)

**Perubahan BE:** Minimal — BE sudah mendukung filter `period`, `status`, `search`, `payment_status`, `household_occupancy_id`, dan `my_bills`. Pagination sudah ada.

**Perubahan Android:**
1. Ganti `fetchAllBills()` → `fetchBills(page, pageSize)` untuk `Iuran Warga`.
2. Filter `period` dan `status` dikirim ke BE via query params.
3. Tambahkan pagination/infinite scroll di ListView atau pagination halaman.

**Dampak:**
- Payload berkurang drastis (20-100 items per halaman vs 100-300+ items sekaligus).
- Sorting/filtering di BE lebih akurat dan scalable.
- Tidak perlu `listPayments()` per bill di Android (paid_amount sudah dihitung di BE).

**Kompleksitas:** Rendah-Medium.

**Risiko inkonsensi:** Minimal — BE sudah menjadi sumber kebenaran untuk `paid_amount`.

#### Opsi B — Ringkasan Per Rumah

**Perubahan BE:** Perlu query baru yang GROUP BY `household_occupancy_id` dengan agregasi (total tagihan, total dibayar, total pending, status terbaru).

**Perubahan Android:**
1. Komponen card baru untuk ringkasan rumah.
2. Detail tagihan dimuat saat item diklik (lazy load).

**Dampak:**
- Jumlah item di UI berkurang dari hundreds ke tens.
- Navigasi lebih cepat untuk RT dengan 150+ rumah.
- Namun: filter `payment_status` (menunggu verifikasi, dll) jadi lebih kompleks karena satu rumah bisa memiliki beberapa tagihan dengan status berbeda.

**Kompleksitas:** Tinggi — perlu perubahan model data + query BE + komponen UI baru.

**Risiko inkonsensi:** Medium — perlu memastikan agregasi per rumah tetap akurat saat ada tagihan baru atau pembayaran masuk.

### Kesimpulan UI

Untuk saat ini, **Opsi A (pagination individual)** adalah langkah yang tepat karena:
1. Perubahan minimal di BE (sudah support).
2. Tidak mengubah UX saat ini (tetap per-tagihan).
3. Langsung menjawab masalah bandwidth dan memori.

Opsi B bisa dipertimbangkan di masa depan jika RT mencapai 300+ rumah dan warga mengeluh terlalu banyak item.

---

## 7. Benchmark

### Status: Belum Diukur

**Hambatan:**
- Tidak ada akses ke database production atau staging yang bisa dijalankan `EXPLAIN ANALYZE`.
- Tidak ada test fixture dengan data realistis (150 rumah × 12 bulan).
- Tidak ada tool profiling di Android (memori Dart, waktu parsing JSON).

### Rancangan Benchmark yang Disarankan

| Ukuran | Volume | Target Metric |
|--------|--------|---------------|
| Kecil | 15 rumah × 12 = 180 bills | Base latency |
| Menengah | 50 rumah × 12 = 600 bills | Load time, payload size |
| Besar | 150 rumah × 12 = 1.800 bills | Memory usage, scroll smoothness |

**Langkah:**
1. Seed database test dengan data realistis.
2. Ukur: `time curl -H "Authorization: ..." GET /api/v1/bills?page=1&page_size=100`
3. Jalankan `EXPLAIN ANALYZE` pada query `BillList`.
4. Di Android: gunakan `PerformanceOverlay` untuk ukur waktu render ListView.
5. Bandingkan payload JSON response size.

---

## 8. Perbandingan Strategi Pagination

### Strategi 1: LIMIT + OFFSET (Rekomendasi Utama)

**Bukti mendukung:**
- Backend sudah mengimplementasikan ini (`parsePagination()` di `handler.go:67-80`, `OFFSET/LIMIT` di `repository.go:550`).
- `BillCount` sudah ada untuk menghitung total.
- `page_size` max 100 sudah di-enforce.

**Kompleksitas BE:** Nol — sudah ada.  
**Kompleksitas Android:** Rendah — cukup ganti `fetchAllBalls()` ke `fetchBills()` dengan pagination params.

**Dampak performa:** Payload dari 1.800 items → 100 items per request. Pengurangan ~94%.

**Risiko konsistensi:** Low — OFFSET stabil untuk use case "lihat semua tagihan". Tidak ada masalah items bergeser saat scroll karena warga biasanya tidak scroll ribuan item.

**Kesesuaian:** Cocok untuk Regular, Multi-Bulan, dan Sukarela.

### Strategi 2: Cursor Pagination

**Bukti menentang:**
- Tidak perlu untuk kasus ini — warga tidak melakukan infinite scroll ribuan item.
- Cursor pagination menyulitkan filter yang mengubah hasil (periode berubah → cursor invalid).

**Kompleksitas:** Tinggi — perlu cursor value (misal: `created_at + id`), tidak kompatibel dengan filter period yang dinamis.

### Strategi 3: Pagination Tagihan + Ringkasan Per Rumah

**Bukti netral:**
- Model data saat ini sudah memiliki `household_occupancy_id`, `household_occupancies`, dan `households` yang bisa di-GROUP BY.
- Namun butuh query agregasi baru yang belum ada.

**Kompleksitas BE:** Medium — perlu query GROUP BY.  
**Kompleksitas Android:** Medium — perlu komponen card baru + detail view.

### Strategi 4: Optimasi Query Tanpa Mengubah Pagination

**Bukti mendukung:**
- Correlated subquery `paid_amount` bisa di-replace dengan LEFT JOIN + GROUP BY pada tabel payments.
- Hapus `listPayments()` per-bill di Android karena BE sudah menghitungnya.

**Kompleksitas:** Rendah — hanya ubah 1 query SQL.  
**Dampak:** Setiap halaman BE hanya perlu 1 query (tanpa subquery per baris).

---

## 9. Rekomendasi Final

### Rekomendasi Utama: Strategi 1 + 4 (Hybrid)

Gabungkan pagination server-side **tanpa** mengubah bentuk daftar, ditambah optimasi query correlated subquery.

**Alasan teknis:**
1. **Paling sedikit perubahan** — BE sudah siap, Android tinggal aktifkan.
2. **Tidak mengubah UX** — warga tetap melihat per-tagihan.
3. **Menghilangkan double-fetch** — `listPayments()` per-bill di Android dihapus karena paid_amount sudah dihitung di BE.
4. **Optimasi query** — correlated subquery digantikan JOIN aggregation.

### Alternatif (Future): Strategi 3

Jika RT tumbuh di atas 300 rumah, pertimbangkan ringkasan per rumah sebagai mode tampilan alternatif (toggle "Tampilan Ringkasan").

---

## 10. Rencana Implementasi Bertahap

### Fase 1: Aktifkan Pagination Server-Side (2-3 hari)

**Backend (tidak ada perubahan diperlukan):**
- Endpoint sudah siap. Hanya perlu memastikan `period` filter diteruskan dengan benar.

**Android:**
1. Di `billsProvider`, ganti `repository.fetchAllBills()` → `repository.fetchBills(page: 1, pageSize: 20, period: ...)`.
2. Di `filteredBillsProvider`, filter `period` dikirim ke BE via query param.
3. Tambahkan "Load More" button atau `NotificationListener<ScrollNotification>` untuk infinite scroll di `_BillListTab`.
4. Hapus atau nonaktifkan loop halaman di `fetchAllBills()` (masih bisa dipakai untuk admin view yang perlu semua data sekaligus).

### Fase 2: Hapus Double-Fetch Payments (1-2 hari)

**Android:**
1. Di `IuranRepository.fetchBills()`, hapus loop `apiClient.listPayments(billId: bill.id)` untuk setiap bill.
2. Gunakan `paid_amount` dan `pending_amount` yang sudah dihitung di BE (`billResponse.data[].paidAmount`).
3. Ini mengurangi dari N+1 calls (1 API bills + N API payments) menjadi 1 API call.

### Fase 3: Optimasi Query Backend (1-2 hari)

**Backend:**
1. Di `BillList()` repository, ganti:
   ```sql
   COALESCE((SELECT SUM(p.amount) FROM payments p
             WHERE p.bill_id = b.id AND p.status = 'APPROVED'), 0)::text
   ```
   Dengan:
   ```sql
   COALESCE(agg_paid.total, 0)::text
   ```
   Via `LEFT JOIN (SELECT bill_id, SUM(amount) FROM payments WHERE status='APPROVED' GROUP BY bill_id) agg_paid ON agg_paid.bill_id = b.id`

2. Lakukan hal yang sama untuk `pending_amount`.

### Fase 4: Unit Tests (2-3 hari)

1. **Android:** Test `_filterBills()` — verifikasi sorting unpaid→paid, chronological order.
2. **Android:** Test `_parsePeriodForSort()` — parsing string "Oktober 2026" → [2026, 10].
3. **Backend:** Test `BillList()` — verifikasi pagination, filter, dan agregasi pembayaran.

### Strategi Kompatibilitas API
- Endpoint `GET /api/v1/bills` tidak berubah (hanya berbeda response size).
- Android bisa secara gradual: `myBills` tetap pakai fetch-all (small dataset), `billsProvider` pindah ke pagination.

### Acceptance Criteria
- [ ] `Iuran Warga` tab hanya memuat 20-100 item per request (bukan seluruh dataset).
- [ ] Filter periode dilakukan di BE (query param `period`), bukan di client.
- [ ] Tidak ada panggilan `listPayments()` per-bill di `fetchBills()`.
- [ ] `paid_amount` ditampilkan akurat (sama dengan hasil sum approved payments).
- [ ] Sorting unpaid→paid dengan chronological order tetap benar setelah migrasi ke server-side.

---

## 11. Batasan Audit

| Batasan | Alasan |
|---------|--------|
| Execution plan SQL | Tidak ada akses ke database live untuk `EXPLAIN ANALYZE` |
| Ukurannya payload JSON | Tidak bisa diukur tanpa menjalankan request ke backend |
| Memory usage Android | Tidak bisa diukur tanpa profiling tool di perangkat |
| Latency database | Tidak ada akses ke query execution metrics |
| Jumlah bills aktual | 150 rumah adalah estimasi yang diberikan, bukan data aktual |
| Test fixture | Tidak ada fixture test yang terverifikasi untuk volume realistis |
| Index effectiveness | Keberadaan index diverifikasi dari migration file, tapi efektivitas saat query belum bisa diukur |

---

## 12. Verifikasi Tidak Ada Perubahan

**`git status` sebelum audit:** Tidak dieksekusi (audit hanya baca kode).  
**`git status` setelah audit:** Tidak ada perubahan file.  
**Konfirmasi:** Tidak ada file yang dimodifikasi, tidak ada commit, tidak ada push, tidak ada deploy, tidak ada perubahan database.
