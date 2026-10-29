# Laporan Pre-Implementation Audit — Pagination Iuran Warga Android

Tanggal: 2026-10-28  
Status: **Audit only — tidak ada perubahan file**

---

## 1. Go/No-Go

**Go.** Implementasi dapat dimulai.

Endpoint `GET /api/v1/bills` sudah mendukung seluruh parameter yang diperlukan, pagination sudah di-enforce, isolasi data berbasis JWT sudah diterapkan, dan respons memiliki format `pagination` yang lengkap. Tidak ada perubahan kontrak BE yang diperlukan sebelum dimulai.

Catatan: ada **ketidakcocokan sorting** antara BE dan Android yang perlu ditangani (detail di Bagian 2.4), tetapi ini bukan penghalang — hanya membutuhkan penyesuaian di sisi Android.

---

## 2. Kontrak BE Terverifikasi

### 2.1 Endpoint: `GET /api/v1/bills`

| Parameter | Tipe | Default | Maks | Deskripsi |
|-----------|------|---------|------|-----------|
| `page` | int | 1 | — | Nomor halaman (1-indexed) |
| `page_size` | int | 20 | 100 | Jumlah item per halaman |
| `due_id` | string | — | — | Filter berdasarkan iuran |
| `status` | string | — | — | `unpaid`, `partial`, `paid`, `cancelled` |
| `payment_status` | string | — | — | `PENDING`, `APPROVED`, `REJECTED` |
| `period` | string | — | — | Filter periode (`Oktober 2026` atau format `2026-10`) |
| `household_occupancy_id` | string | — | — | Filter rumah spesifik |
| `my_bills` | string | — | — | `"true"` → batasi ke occupancy pengguna |
| `search` / `q` | string | — | — | Pencarian ILIKE pada `d.name`, `h.head_name`, `ph.house_number` |
| `due_name` | string | — | — | Filter tepat pada nama iuran |
| `is_active` | bool | — | — | `true`/`false` |
| `rt_id` | string | — | — | Hanya untuk SUPER_ADMIN (cross-tenant) |

**Bukti:** `backend/internal/finance/handler.go:570-600` (parse params), `handler.go:66-80` (`parsePagination`), `handler.go:630-780` (handler logic).

### 2.2 Pagination & Sorting

| Aspek | Status | Detail |
|-------|--------|--------|
| LIMIT/OFFSET langsung di DB | ✅ | `repository.go:550`: `OFFSET $N LIMIT $M` |
| Max page_size | ✅ | `handler.go:76`: `n > 100 → n = 100` |
| Sebelum/sesudah agregasi | **Sesudah** | `paid_amount` dihitung via correlated subquery **pada setiap baris hasil**, lalu hasil yang sudah dikalkulasi baru di-LIMIT |
| Urutan stabil | ⚠️ | `repository.go:550-553`: dua jalur sorting — `ORDER BY b.period ASC, b.created_at ASC` jika `occupancyID` ada, **atau** `ORDER BY b.created_at DESC` jika tidak |
| Tie-breaker unik | ❌ | `created_at` bukan kolom unik — dua bill yang dibuat bersamaan bisa berurutan berbeda antar-halaman |
| Metadata respons | ✅ | `{ "data": [...], "pagination": { "page", "per_page", "total", "total_pages" } }` |

**⚠️ Catatan Sorting — Ketidakcocokan BE vs Android:**

| Aspek | BE (`BillList`) | Android (`_filterBills`) |
|-------|-----------------|--------------------------|
| Urutan dasar | `created_at DESC` (umum) / `period ASC` (occupancy ID) | **Unpaid/partial dulu**, baru **Lunas** |
| Urutan dalam unpaid | Oldest `created_at` pertama | Oldest `periode` pertama |
| Urutan dalam lunas | Newest `created_at` pertama | Newest `periode` pertama |

**Implikasi:** Jika filter `period` diterapkan di BE (`?period=Oktober 2026`), semua item di halaman yang sama memiliki periode identik, sehingga urutan berdasarkan `created_at` bisa diterima. Namun, **sorting "unpaid dulu"** hanya dilakukan di Android — BE tidak pernah mengelompokkan unpaid di atas paid.

**Rekomendasi:** Untuk paginasi halaman pertama (yang memuat semua bill warga), `my_bills=true` akan memicu jalur `occupancyID != null` → sorting `period ASC, created_at ASC`. Ini sudah menghasilkan urut chronological yang masuk akal. Namun, **pengelompokan unpaid→paid** perlu tetap dilakukan di client-side (setelah halaman pertama dimuat) karena BE tidak melakukan ini.

### 2.3 Isolasi Data

| Jalur | Mekanisme | Bukti |
|-------|-----------|-------|
| Warga biasa | `GetOccupancyIDForUser()` → `occupancyID` filter otomatis | `handler.go:699-709` |
| Pengurus/Bendahara (staff) | Bisa pilih `household_occupancy_id` atau lihat semua | `handler.go:710-712` |
| SUPER_ADMIN | Query terpisah `BillCountAll`/`BillListAll` dengan `rt_id` param | `handler.go:610-668` |
| JWT tidak di-mutate | `ac.RTID` selalu dari JWT claim | `handler.go:571` |

### 2.4 Kesimpulan Kontrak BE

API **siap langsung digunakan**. Hanya ada satu pertimbangan:
- BE sorting (`period ASC` untuk `my_bills`) sudah cukup baik untuk pagination chronological.
- Android tetap perlu melakukan sorting "unpaid→paid" di client-side **setelah memuat halaman pertama**, karena BE tidak mengelompokkan berdasarkan status pembayaran.
- Jika filter `period` juga diteruskan ke BE, maka seluruh hasil di halaman tersebut sudah terfilter secara server-side, dan sorting client-side hanya perlu menata unpaid vs paid.

---

## 3. Rencana Perubahan Android

### 3.1 File dan Fungsi yang Perlu Diubah

| File | Fungsi/Bagian | Perubahan |
|------|--------------|-----------|
| `iuran_repository.dart` | `fetchAllBills()` | **Ganti atau buat `fetchBillsPaginated()`** yang memanggil `fetchBills(page, pageSize, ...)` sekali saja, bukan loop |
| `iuran_repository.dart` | `fetchBills()` | Pastikan params `period`, `status`, `search` diteruskan ke API |
| `iuran_providers.dart` | `BillsNotifier.build()` | Panggil paginated fetch dengan `period` dari provider |
| `iuran_providers.dart` | `_filterBills()` | Hanya filter `status` dan `payment_status` (tidak filter `period` — sudah di BE) |
| `iuran_screen.dart` | `_BillListTab` | Tambahkan "Load More" / infinite scroll |
| `iuran_screen.dart` | `_IuranCard` | Tidak berubah (sudah menggunakan `paid_amount` dari BE) |
| `iuran_api_client.dart` | `listBills()` | Tidak berubah (sudah return `PaginatedResponse`) |

### 3.2 Detail Per File

#### `iuran_repository.dart` — `fetchBillsPaginated()`

```dart
Future<PaginatedResponse<IuranBill>> fetchBillsPaginated({
  String? period,
  String? status,
  String? paymentStatus,
  bool myBills = false,
  int page = 1,
  int pageSize = 20,
}) async {
  // Panggil fetchBills yang sudah ada (sudah support pagination di BE)
  // Hapus loop halaman — cukup ambil halaman tertentu
  final response = await fetchBills(
    status: status,
    paymentStatus: paymentStatus,
    period: period,
    householdOccupancyId: myBills ? null : null,  // BE handle via my_bills param
    myBills: myBills,
    page: page,
    pageSize: pageSize,
  );
  // Hapus loop listPayments per-bill (lihat Bagian 4)
  return response;
}
```

**Penting:** `fetchBills()` saat ini sudah memanggil `fetchBills()` dari API client dengan `page` dan `pageSize`, lalu melakukan loop halaman di `fetchAllBills()`. Yang perlu diubah adalah **menghentikan loop halaman** dan **menghapus loop `listPayments()` per-bill**.

#### `iuran_providers.dart` — `BillsNotifier`

```dart
class BillsNotifier extends AutoDisposeAsyncNotifier<PaginatedIuranState> {
  // Ganti AsyncNotifier<List<IuranBill>> → custom state
  // State: { List<IuranBill> bills, int currentPage, bool hasMore, bool isLoadingMore }
}
```

Alternatif lebih sederhana: gunakan pattern yang sudah ada (`AsyncNotifier`) tapi tambahkan `loadMore()` method:

```dart
Future<void> loadMore({int page = 2}) async {
  // Fetch halaman berikutnya dan append ke state
}
```

#### `iuran_screen.dart` — Infinite Scroll

Tambahkan di `_BillListTab`:
```dart
NotificationListener<ScrollNotification>(
  onNotification: (ScrollInfo info) {
    if (info.metrics.pixels == info.metrics.maxScrollExtent) {
      // Trigger loadMore
    }
  },
  child: ListView...,
)
```

### 3.3 Perilaku yang Dijamin

| Perilaku | Implementasi |
|----------|-------------|
| Halaman pertama dimuat saat layar dibuka | `build()` → `fetchBillsPaginated(page: 1)` |
| Halaman berikutnya dimuat saat scroll ke bawah | `NotificationListener<ScrollNotification>` → `loadMore()` |
| Tidak ada concurrent request | Flag `isLoadingMore` mencegah request ganda |
| Filter/search reset pagination | Provider reset → `currentPage = 1`, clear bills |
| Refresh → halaman pertama | `refresh()` → clear semua, fetch `page: 1` |
| Error halaman berikutnya retryable | State menyimpan `List<IuranBill>` yang sudah tampil + error per-halaman |
| Loading/empty state | `AsyncLoading` untuk halaman pertama, `CircularProgressIndicator` kecil untuk halaman berikutnya |
| Perubahan token → tidak leak data | `_lastToken` check di `build()` (sudah ada) |

### 3.4 Filter vs Server-Side

| Filter | Lokasi | Alasan |
|--------|--------|--------|
| `period` | **Server-side** (`?period=...`) | Payload besar — perlu dibatasi di sumber |
| `status` (`belumBayar`, `sebagian`, `lunas`) | **Client-side** | Hanya memfilter subset hasil yang sudah dimuat, tidak mempengaruhi halaman lain |
| `payment_status` (`pending`, `approved`, `rejected`) | **Server-side** (`?payment_status=...`) | Sudah di-support BE, penting untuk filter awal |
| `search` | **Server-side** (`?search=...`) | Sudah di-support BE (ILIKE), lebih akurat |

**Catatan:** Jika filter `status` (belumBayar/sebagian/lunas) di-filter di server (`?status=...`), maka user bisa kehilangan data jika hasil > pageSize karena halaman berikutnya mungkin memiliki status berbeda. **Rekomendasi: status filter tetap di client-side.**

---

## 4. Audit Pemanggilan `listPayments()` Per Bill

### 4.1 Lokasi Pemanggilan

| Lokasi | File | Fungsi | Data yang Dibutuhkan |
|--------|------|--------|---------------------|
| #1 | `iuran_repository.dart:74-90` | `fetchBills()` | `paid_amount`, `pending_amount`, `rejected_amount` |
| #2 | `iuran_repository.dart:135-150` | `fetchBillDetail()` | `paid_amount`, `pending_amount`, `rejected_amount`, `due_payment_rule` |
| #3 | `iuran_providers.dart:246-250` | `billPaymentsProvider` | **Semua** payment detail (untuk riwayat pembayaran) |

### 4.2 Analisis Detail

#### Lokasi #1: `fetchBills()` — **AMAN DIHAPUS**

**Kode saat ini:**
```dart
for (final bill in billResponse.data) {
  PaginatedResponse<BackendPayment> payments = const PaginatedResponse(...);
  try {
    payments = await apiClient.listPayments(billId: bill.id);  // N+1 call!
  } catch (e) { debugPrint(...); }
  
  int approvedTotal = payments.data.where((p) => p.status == PaymentStatus.approved)
      .fold<int>(0, (sum, p) => sum + moneyToDisplayRupiah(p.amount));
  if (approvedTotal == 0 && bill.paidAmount != null) {
    approvedTotal = moneyToDisplayRupiah(bill.paidAmount!);  // Fallback ke BE
  }
  // ... pending dan rejected juga sama
}
```

**Ketergantungan:**
- `paid_amount` → **sudah ada di BE** (`bill.paidAmount` dari `billResponse.data`).
- `pending_amount` → **sudah ada di BE** (`bill.pendingAmount` dari `billResponse.data`).
- `rejected_amount` → **TIDAK ada di BE** — hanya dihitung dari `listPayments()`.
- Data detail pembayaran (proof path, verified_by, dll) → **tidak dipakai** di `_IuranCard` atau ListView.

**Keputusan: AMAN DIHAPUS.**  
`rejected_amount` bukan field yang ditampilkan di `_IuranCard` (hanya `paidAmount` dan `pendingAmount` yang dipakai). Lihat `iuran_screen.dart:690-710`: card hanya menampilkan nominal, "Dibayar", "Tertunda", dan "Sisa".

#### Lokasi #2: `fetchBillDetail()` — **PERLU PERUBAHAN**

**Kode saat ini:**
```dart
final payments = await apiClient.listPayments(billId: bill.id);
// ... hitung approvedTotal, pendingTotal, rejectedTotal dari payments
// Juga: fetch dues untuk paymentRule
final dues = await apiClient.listDues();
final matching = dues.where((d) => d.id == bill.dueId).firstOrNull;
```

**Ketergantungan:**
- `paid_amount`, `pending_amount` → **sudah ada di BE** (response dari `getBill()`).
- `due_payment_rule` → diperlukan untuk menentukan `requiresFullPayment`.

**Keputusan: PERLU PERUBAHAN.**  
Dapat diganti dengan:
1. Gunakan `paidAmount` dan `pendingAmount` dari response `getBill()`.
2. `due_payment_rule` bisa dikirim di response `getBill()` (via JOIN di BE) — **ini perubahan BE kecil yang direkomendasikan**.

#### Lokasi #3: `billPaymentsProvider` — **TETAP PERTAHANKAN**

**Kode:**
```dart
final billPaymentsProvider = FutureProvider.family
    .autoDispose<List<BackendPayment>, String>((ref, billId) {
      final repository = ref.read(iuranRepositoryProvider);
      return repository.listPaymentsForBill(billId);
    });
```

**Ketergantungan:** Provider ini digunakan di **detail pembayaran** (screen `/home/iuran/detail/{billId}`) untuk menampilkan riwayat pembayaran.

**Keputusan: TETAP PERTAHANKAN.**  
Tidak ada cara untuk mengganti ini tanpa memanggil API.

### 4.3 Kesimpulan Per Lokasi

| Lokasi | Status | Alasan |
|--------|--------|--------|
| `fetchBills()` (#1) | **Aman dihapus** |paid_amount & pending_amount sudah dari BE; rejected_amount tidak ditampilkan di card |
| `fetchBillDetail()` (#2) | **Perlu perubahan BE** | paid_amount dari BE, tapi due_payment_rule perlu ditambahkan di response GET /bills/{id} |
| `billPaymentsProvider` (#3) | **Pertahankan** | Digunakan di detail view untuk riwayat pembayaran — tidak ada pengganti |

---

## 5. Validasi Integritas Bisnis

### 5.1 Hal yang Tetap Berjalan Sama

| Aturan Bisnis | Apakah Terpengaruh? | Penjelasan |
|---------------|---------------------|-----------|
| Perhitungan `paid_amount` | **Tidak** | BE menghitung via correlated subquery → sama persis sebelum dan sesudah |
| Status belum bayar/sebagian/lunas | **Tidak** | BE menghitung dari `bills.status` → sama |
| Pemisahan Reguler/Multi-Bulan/Sukarela | **Tidak** | Berdasarkan `due.due_type` dan `due.is_voluntary` → sudah di BE |
| Aturan aktif/nonaktif Multi-Bulan | **Tidak** | `bills.is_active` filter → BE handle via `isActive` param |
| Validasi FIFO pembayaran Multi-Bulan | **Tidak** | FIFO terjadi di BE saat pembayaran diverifikasi → tidak terpengaruh pagination |
| Pembatasan data berdasarkan JWT | **Tidak** | `ac.RTID` dan `GetOccupancyIDForUser()` → sama |

### 5.2 Logika yang Wajib Tetap di BE

| Logika | Mengapa di BE |
|--------|---------------|
| `paid_amount` = SUM approved payments | Harus akurat; client tidak boleh menghitung dari subset |
| `pending_amount` = SUM pending payments | Sama |
| Tenant isolation (`rt_id` dari JWT) | Keamanan |
| Staff vs warga access control | Keamanan |
| `is_active` filter untuk warga | Warga hanya lihat tagihan aktif |
| `payment_status` filter via EXISTS | Akurat dan efisien |

### 5.3 Potensi Risiko Integritas

1. **Sorting "unpaid dulu"** — Jika hanya halaman pertama yang difilter sort unpaid→paid di client, dan user scroll ke halaman berikutnya (yang tidak disortir), ada transisi mendadak dari unpaid ke sorted chronological. **Mitigasi:** filter `status` di client-side (tidak di BE) sehingga seluruh hasil tetap bisa disortir secara konsisten. Atau: tambahkan `order=unpaid_first` param di BE.

2. **Pagination + status filter di client** — Jika user memilih "Status: Belum Bayar" lalu scroll ke halaman berikutnya (yang dimuat tanpa filter status), halaman berikutnya akan menampilkan semua status. **Mitigasi:** saat filter `status` diaktifkan, nonaktifkan "Load More" dan tampilkan pesan "Filter hanya berlaku untuk halaman pertama" atau migrasi status filter ke BE.

---

## 6. Rencana Pengujian Sebelum Coding

### 6.1 Test Cases

| # | Test Case | Tipe | Status Existing | Lokasi Test |
|---|-----------|------|-----------------|-------------|
| T1 | `GET /api/v1/bills?page=1&page_size=20` mengembalikan 20 item dengan pagination metadata | BE Integration | ✅ Ada (`bills_rbac_test.go`) | `backend/cmd/api/bills_rbac_test.go` |
| T2 | Halaman terakhir < `page_size` (misal 13 item dari 100) | BE Integration | ❌ Belum ada | Tambah di `bills_rbac_test.go` |
| T3 | Halaman kosong (filter tidak cocok) → `data: [], pagination: { total: 0 }` | BE Integration | ❌ Belum ada | Tambah di `bills_rbac_test.go` |
| T4 | `page=1&page_size=100` → max 100 item | BE Integration | ✅ Ada (max enforced di `parsePagination`) | `backend/internal/finance/handler_test.go` |
| T5 | Warga tidak bisa melihat data RT lain (`my_bills=true`) | BE Integration | ✅ Ada | `bills_rbac_test.go:314-356` |
| T6 | Warga tidak bisa mengakses bill RT lain via `/bills/{id}` | BE Integration | ✅ Ada | `bills_rbac_test.go:346-356` |
| T7 | `period` filter menghasilkan hasil konsisten di semua halaman | BE Integration | ❌ Belum ada | Tambah |
| T8 | `search` filter ILIKE bekerja di `head_name`, `due_name`, `house_number` | BE Integration | ❌ Belum ada | Tambah |
| T9 | Pagination tidak mengubah `total` count saat ada pembayaran baru | BE Integration | ❌ Belum ada | Tambah |
| T10 | Android: load halaman pertama, tampilkan 20 item | Android Unit | ❌ Belum ada | `test/features/iuran/...` |
| T11 | Android: scroll ke bawah → load halaman kedua, append 20 item | Android Widget | ❌ Belum ada | `test/features/iuran/...` |
| T12 | Android: filter periode → reset ke halaman 1, refresh list | Android Widget | ❌ Belum ada | `test/features/iuran/...` |
| T13 | Android: filter status "Belum Bayar" → hanya item unpaid | Android Unit | ❌ Belum ada | `test/features/iuran/...` |
| T14 | Android: sort unpaid→paid (terbukti di `_filterBills`) | Android Unit | ❌ Belum ada | `test/features/iuran/iuran_providers_test.dart` |
| T15 | Android: `listPayments()` tidak dipanggil saat load bills list | Android Unit | ❌ Belum ada | `test/features/iuran/iuran_repository_test.dart` |
| T16 | Android: payment detail masih bekerja (`billPaymentsProvider`) | Android Widget | ❌ Belum ada | `test/features/iuran/...` |
| T17 | BE: correlated subquery `paid_amount` akurat untuk bill dengan 0, 1, 3 payments | BE Unit | ❌ Belum ada | `backend/internal/finance/bill_repository_test.go` |
| T18 | BE: `BillCount` mengembalikan total yang konsisten dengan `BillList` | BE Unit | ❌ Belum ada | `backend/internal/finance/bill_repository_test.go` |
| T19 | Multi-Bulan: FIFO pembayaran tetap setelah pagination | BE Integration | ❌ Belum ada | `backend/internal/finance/` |
| T20 | Sokarela: pagination tidak mempengaruhi donasi per transaksi | BE Integration | ❌ Belum ada | `backend/internal/finance/` |

### 6.2 Test yang Perlu Ditambahkan (Prioritas)

1. **T2, T3**: Edge case pagination di BE.
2. **T7**: Konsistensi filter `period` across pages.
3. **T14**: Unit test `_filterBills()` — sangat penting karena sorting logic kompleks dan tidak terdokumentasi.
4. **T17**: Verifikasi akurasi correlated subquery `paid_amount`.

### 6.3 Test yang Sudah Ada

- **T1, T5, T6**: Ada di `backend/cmd/api/bills_rbac_test.go`.
- **Pagination max enforcement**: Ada di `parsePagination()` logic.

---

## 7. Risiko Tersisa

| Risiko | Tingkat | Mitigasi |
|--------|---------|----------|
| Sorting "unpaid→paid" tidak konsisten across pages | Medium | Filter `status` di client-side (bukan BE); atau tambahkan `order=unpaid_first` di BE |
| Status filter + pagination = user kehilangan konteks | Medium | Nonaktifkan "Load More" saat filter aktif, atau migrasi status ke BE |
| Correlated subquery `paid_amount` melambat pada 1000+ rumah | Low-Medium | Optimasi via JOIN aggregation (Fase 3) jika benchmark menunjukkan masalah |
| `created_at` tiebreaker tidak unik → urutan bisa berubah antar-halaman | Low | Tambah `ORDER BY b.created_at DESC, b.id DESC` (UUID unik) di BE |
| Token refresh saat "Load More" → state inconsistent | Low | Sudah ada `_lastToken` check; tambahkan error handling untuk concurrent refresh |

---

## 8. Acceptance Criteria

Implementasi dinyatakan selesai jika **semua** kriteria berikut terpenuhi:

### Backend (tidak ada perubahan diperlukan, tapi verifikasi)
- [ ] `GET /api/v1/bills?page=1&page_size=20` mengembalikan response dengan 20 items dan `pagination.total_pages > 0`.
- [ ] `GET /api/v1/bills?page=1&period=Oktober%202026` mengembalikan data yang sama di semua halaman yang relevan.
- [ ] Warga hanya menerima bill dari occupancy-nya sendiri (`my_bills=true`).
- [ ] Response selalu memiliki struktur `{ "data": [...], "pagination": { "page", "per_page", "total", "total_pages" } }`.

### Android
- [ ] `Iuran Warga` tab memuat maksimal 20-100 item per request (bukan seluruh dataset).
- [ ] Filter `period` diteruskan ke BE sebagai query param.
- [ ] Filter `status` dan `payment_status` tetap di client-side (tidak memengaruhi halaman berikutnya).
- [ ] `listPayments()` **tidak dipanggil** saat memuat daftar bills di `_BillListTab`.
- [ ] `paid_amount` dan `pending_amount` ditampilkan akurat (sama dengan nilai dari BE).
- [ ] "Load More" muncul saat scroll ke bawah (atau infinite scroll bekerja).
- [ ] Filter baru mereset pagination ke halaman 1.
- [ ] Refresh mengembalikan daftar ke halaman 1.
- [ ] Sorting "unpaid→paid" tetap konsisten di halaman pertama.
- [ ] `billPaymentsProvider` (detail view) masih bekerja — payment history tetap ditampilkan.

### Integritas Bisnis
- [ ] Pembayaran Multi-Bulan FIFO masih valid.
- [ ] Warga tidak bisa melihat data rumah lain.
- [ ] Token refresh tidak menyebabkan data leak antar-user.
- [ ] Endpoint tidak berubah — client lain (jika ada) tetap kompatibel.

---

**Berhenti setelah laporan. Tidak ada coding dimulai.**
