# Audit Multi-Bulan Iuran Urena — Laporan Analisis dan Keputusan Implementasi

> **Tanggal audit:** 2026-10-28  
> **Branch:** `urena`  
> **Commit terakhir saat audit:** `f0e5349` — fix: rename RT 003 to RT 001, fix refresh token device_id param, add debug logging  
> **Status:** Analisis read-only berdasarkan laporan audit yang tersedia, dilengkapi keputusan bisnis terbaru.  
> **Catatan:** Pembaruan dokumen ini tidak membuktikan bahwa kode atau skema repository telah berubah. Seluruh temuan teknis tetap harus diverifikasi terhadap kondisi repository aktual sebelum implementasi.

---

## 1. Ringkasan Eksekutif

Implementasi Multi-Bulan Iuran Urena telah memiliki fondasi penerbitan tagihan, pembayaran multi-periode, FIFO, persetujuan pembayaran, dan pencatatan KAS. Namun, fitur pengaturan status aktif/nonaktif untuk setiap tagihan bulanan belum tersedia.

Keputusan bisnis terbaru menetapkan bahwa sistem harus menyediakan 12 tagihan untuk Januari–Desember pada tahun yang dipilih, dengan semua bulan aktif secara default. Bendahara bertanggung jawab mengatur status aktif/nonaktif setiap bulan melalui menu Edit Iuran.

Status aktif/nonaktif menentukan apakah tagihan boleh dibayar. Status tersebut tidak berubah otomatis ketika bulan berganti.

FIFO tetap wajib ditegakkan: tagihan aktif tertua yang belum lunas harus diprioritaskan. Tagihan nonaktif tidak dapat dibayar dan tidak menghalangi pembayaran tagihan aktif berikutnya. Dengan demikian, FIFO berlaku pada rangkaian tagihan yang aktif dan dapat dibayar, bukan berarti setiap bulan kalender harus selalu aktif.

Pembayaran tagihan lama yang diterima pada bulan berikutnya tetap melunasi periode tagihan asal, sedangkan transaksi KAS dan saldo dicatat berdasarkan tanggal penerimaan aktual sesuai alur pembayaran yang berlaku.

### 1.1 Temuan utama

**Sudah dilaporkan sesuai:**
1. Generator Multi-Bulan menghasilkan 12 periode Januari–Desember.
2. Constraint unik dan `ON CONFLICT DO NOTHING` mencegah duplikasi penerbitan.
3. Backend memiliki pemeriksaan FIFO untuk pembayaran single dan multi-periode.
4. Pembayaran pending belum melunasi tagihan.
5. Approval pembayaran multi-periode memposting satu transaksi KAS.
6. `actual_received_date` digunakan sebagai dasar tanggal transaksi KAS.
7. Backend menggunakan mekanisme locking dan idempotensi untuk mengurangi risiko duplikasi dan konflik transaksi.

**Belum tersedia atau perlu diverifikasi:**
1. Status aktif/nonaktif pada setiap tagihan bulanan.
2. Endpoint dan validasi backend untuk mengubah status tagihan individual.
3. UI Web dan Android untuk mengelola status 12 bulan.
4. Penerapan status aktif pada seluruh jalur pembayaran, FIFO, dan approval.
5. Perlindungan ketika status tagihan berubah saat pembayaran masih pending.
6. Kebijakan tagihan untuk warga baru atau pindah di tengah tahun.

---

## 2. Keputusan Bisnis Final

Bagian ini menjadi acuan implementasi. Jika perilaku kode saat ini berbeda, kode harus disesuaikan dengan keputusan ini setelah dampaknya diverifikasi.

### 2.1 Penerbitan Multi-Bulan

1. Pengurus memilih tahun kalender penerbitan.
2. Sistem menyediakan 12 tagihan bulanan untuk Januari sampai Desember pada tahun yang dipilih.
3. Setiap tagihan menggunakan periode `YYYY-MM`.
4. Jatuh tempo otomatis pada tanggal 10 setiap bulan, disesuaikan dengan hari terakhir bulan apabila diperlukan.
5. Semua 12 bulan berstatus aktif secara default, termasuk bulan-bulan yang sudah lewat saat iuran diterbitkan.
6. Penerbitan tidak boleh hanya menghasilkan tagihan dari bulan berjalan sampai Desember.
7. Penerbitan ulang tidak boleh membuat tagihan duplikat atau mengubah status bulan yang sudah dikonfigurasi bendahara secara tidak sengaja.

### 2.2 Status Aktif/Nonaktif per Bulan

1. Setiap tagihan bulanan Multi-Bulan memiliki status aktif/nonaktif tersendiri.
2. Semua tagihan yang baru dibuat berstatus aktif.
3. Bendahara dapat mengubah status melalui menu Edit Iuran.
4. UI pengelolaan harus menampilkan 12 bulan beserta statusnya.
5. Pergantian bulan tidak otomatis mengubah status tagihan.
6. Tagihan aktif dapat dibayar apabila seluruh validasi pembayaran dan FIFO terpenuhi.
7. Tagihan nonaktif tidak dapat dipilih atau dibayar.
8. Tagihan nonaktif tidak dihapus dan tidak kehilangan riwayat pembayaran.
9. Bendahara boleh mengaktifkan kembali tagihan yang sebelumnya dinonaktifkan, selama tagihan tersebut belum lunas dan tidak melanggar validasi lain.
10. Tagihan yang sudah lunas tidak boleh dinonaktifkan melalui pengaturan biasa.
11. Status aktif/nonaktif merupakan status kelayakan tagihan untuk dibayar, bukan pengganti status pembayaran seperti belum lunas atau lunas.
12. Hak mengubah status harus divalidasi di backend, bukan hanya disembunyikan di UI.

### 2.3 FIFO

1. FIFO wajib ditegakkan pada tagihan aktif yang belum lunas dalam kelompok pembayaran yang sama, sesuai cakupan FIFO yang sudah berlaku.
2. Pembayaran tidak boleh melompati tagihan yang lebih lama dan masih aktif serta belum lunas.
3. Tagihan nonaktif tidak dapat dibayar dan tidak menghalangi pembayaran tagihan aktif berikutnya.
4. Contoh: Januari aktif dan belum lunas, Februari nonaktif, Maret aktif dan belum lunas. Warga harus melunasi Januari sebelum dapat membayar Maret. Setelah Januari lunas, Maret dapat dibayar meskipun Februari tetap nonaktif.
5. Contoh lain: Agustus dan September aktif serta belum lunas, Oktober aktif. Warga yang membayar dua bulan harus melunasi Agustus lalu September; pembayaran tidak boleh dialokasikan langsung ke Oktober.
6. Partial payment yang belum melunasi tagihan tertua tidak boleh membuka akses untuk membayar tagihan aktif berikutnya, sesuai aturan FIFO yang berlaku.
7. Perubahan status harus diterapkan secara konsisten di backend. Filter UI saja tidak cukup untuk menegakkan FIFO.
8. FIFO tetap dibatasi pada kelompok yang berlaku di implementasi saat ini, yaitu berdasarkan household occupancy dan jenis iuran terkait, kecuali audit kode membuktikan aturan lain.

### 2.4 Tagihan Lunas dan Pembayaran Pending

1. Tagihan yang sudah lunas tidak boleh dinonaktifkan melalui pengaturan biasa.
2. Status lunas ditentukan oleh pembayaran yang telah disetujui dan alokasi pembayaran yang sah, bukan sekadar adanya pengajuan pembayaran.
3. Pembayaran pending belum menambah saldo KAS dan belum melunasi tagihan.
4. Perubahan status tagihan tidak boleh menyebabkan pembayaran dialokasikan dua kali atau melanggar FIFO.
5. Jika tagihan memiliki pembayaran pending ketika bendahara hendak menonaktifkannya, sistem harus menerapkan validasi yang konsisten dan memberikan pesan yang jelas.
6. Jika status tagihan berubah sebelum approval, proses approval harus memeriksa ulang validitas tagihan dan alokasinya.
7. Jangan membatalkan atau mengubah riwayat pembayaran yang sudah disetujui sebagai efek samping perubahan status tagihan.

**Perlindungan pembayaran pending:** implementasi harus menentukan dan menguji aturan transaksi yang konsisten. Minimal, sistem tidak boleh menyetujui pembayaran yang alokasinya sudah tidak valid atau membuat pembayaran yang sama melunasi tagihan lebih dari sekali.

### 2.5 Pencatatan KAS dan Saldo

1. Bulan tagihan yang dilunasi berbeda dari tanggal penerimaan uang.
2. KAS masuk dicatat menggunakan tanggal penerimaan aktual sesuai alur pembayaran yang berlaku.
3. Jika approval diwajibkan, transaksi KAS hanya dibuat pada tahap yang sudah ditentukan sistem, dan tidak boleh tercatat dua kali.
4. Saldo dan laporan KAS mengikuti tanggal transaksi aktual, bukan periode tagihan.
5. Contoh: tagihan Januari diaktifkan kembali dan dibayar pada 15 Oktober. Riwayat pembayaran melunasi Januari, tetapi penerimaan KAS dicatat pada 15 Oktober sesuai tanggal penerimaan aktual.
6. Implementasi tidak boleh mengubah alur KAS Reguler atau Sukarela.

### 2.6 Reguler dan Sukarela

1. Alur Reguler yang sekarang berlaku harus tetap sama.
2. Alur Sukarela yang sekarang berlaku harus tetap sama.
3. Perubahan status bulanan dan FIFO tambahan harus dibatasi pada Multi-Bulan.
4. Jika model atau query dipakai bersama, implementasi wajib memastikan perubahan tidak mengubah perilaku Reguler dan Sukarela.

---

## 3. Kondisi Implementasi yang Dilaporkan dalam Audit

Bagian ini merangkum temuan dari laporan audit sebelumnya. Path, nama fungsi, dan perilaku harus diperiksa kembali pada repository aktual sebelum implementasi.

### 3.1 Database

| Aspek | Kondisi yang dilaporkan | Implikasi |
|---|---|---|
| Tabel `bills` | Status pembayaran berupa `unpaid`, `paid`, `cancelled` | Belum ada status aktif/nonaktif per bulan |
| Tabel `dues` | Memiliki `is_active` dan konfigurasi jenis iuran | Status level jenis iuran tidak cukup untuk mengatur bulan individual |
| Tabel `dues` | Memiliki `due_type`, `year`, dan `period_type` | Menjadi dasar identifikasi Multi-Bulan |
| Tabel `payments` | Memiliki `actual_received_date` dan status pembayaran | Dasar pemeriksaan siklus approval |
| Tabel `payments` | Memiliki relasi `bill_id` dan `due_id` | Perlu mempertahankan konsistensi alokasi |
| Tabel `payment_allocations` | Menyimpan relasi pembayaran ke tagihan untuk multi-periode | Harus tetap konsisten ketika status tagihan berubah |
| Tabel `transactions` | Memiliki `occurred_at`, `payment_id`, dan `reverses_transaction_id` | Mendukung pemeriksaan tanggal KAS dan idempotensi |
| Constraint unik `bills` | `(household_occupancy_id, due_id, period)` | Mencegah duplikasi periode tagihan |
| `bills.is_active` | Dilaporkan belum tersedia | Perlu dipertimbangkan dalam migrasi baru |

### 3.2 Backend Go

**File yang dilaporkan: `backend/internal/finance/service.go`**

- `generateMultiMonthBills()` menghasilkan 12 periode Januari–Desember.
- `CreatePayment()` memeriksa FIFO untuk pembayaran single sesuai cakupan yang berlaku.
- `CreateMultiPeriodPayment()` memeriksa FIFO berdasarkan kelompok occupancy dan jenis iuran.
- `VerifyPayment()` menentukan jalur approval pembayaran.
- `verifyMultiPeriodPayment()` menyetujui pembayaran multi-periode dan memposting satu transaksi KAS.
- `verifySinglePayment()` menangani approval pembayaran single.
- `reconcileBillStatus()` memperbarui status pembayaran tagihan.
- `ParseActualReceivedDate()` dan `JakartaDate()` menangani validasi tanggal aktual.
- `DeactivateDue()` menonaktifkan jenis iuran secara keseluruhan, bukan satu tagihan bulanan.

**File yang dilaporkan: `backend/internal/finance/repository.go`**

- `GetOldestUnpaidBillID()` mengambil tagihan tertua yang belum lunas.
- `GetOldestUnpaidBillIDs()` mengambil urutan tagihan tertua untuk pembayaran multi-periode.
- `GetPendingPaymentTotal()` menghitung nilai pembayaran pending.
- `BillGetByIDForUpdate()` menggunakan row locking.
- `GetActiveOccupanciesWithDates()` memilih occupancy berdasarkan masa aktifnya.

**File yang dilaporkan: `backend/internal/finance/handler.go`**

- `GenerateBills` menangani penerbitan tagihan.
- `CreateMultiPeriodPayment` menangani pembuatan pembayaran multi-periode.
- `DeactivateDue` menonaktifkan jenis iuran.
- `VerifyPayment` menangani persetujuan pembayaran.

### 3.3 Frontend Web

**File yang dilaporkan: `frontend/src/features/iuran/IuranList.tsx`**

- Daftar Kelola Iuran menampilkan jenis iuran, termasuk jenis iuran nonaktif.
- Edit Iuran mengubah konfigurasi pada tingkat jenis iuran.
- Belum ada pengaturan status individual untuk 12 bulan Multi-Bulan.

**File yang dilaporkan: `frontend/src/features/iuran/IuranPayment.tsx`**

- Pembayaran Multi-Bulan memuat tagihan yang dapat dibayar melalui API tagihan.
- UI memiliki kontrol jumlah bulan.
- Pembayaran menggunakan endpoint multi-periode.
- Filter aktif/nonaktif per tagihan belum tersedia menurut laporan audit.

**File yang dilaporkan: `frontend/src/features/iuran/DueCreate.tsx`**

- Form Multi-Bulan menerima tahun kalender.
- Penerbitan tagihan dipanggil setelah jenis iuran dibuat.

### 3.4 Mobile Flutter

**File yang dilaporkan: `mobile/lib/features/iuran/presentation/screens/iuran_management_screen.dart`**

- Status aktif/nonaktif ditampilkan pada tingkat jenis iuran.
- Edit Iuran belum menyediakan toggle status individual per bulan.

**File yang dilaporkan: `mobile/lib/features/iuran/presentation/screens/iuran_payment_screen.dart`**

- Mendukung proses pembayaran multi-periode melalui repository.

**File yang dilaporkan: `mobile/lib/features/iuran/data/iuran_api_client.dart`**

- Menyediakan wrapper API untuk penerbitan tagihan dan pembayaran multi-periode.

### 3.5 Test yang Dilaporkan

| Test | Cakupan yang dilaporkan | Tindak lanjut |
|---|---|---|
| `multi_month_test.go` | Pembuatan 12 tagihan, retry, pembayaran multi-periode, FIFO, satu KAS | Pertahankan dan tambahkan test status |
| `fifo_test.go` | FIFO single/multi, isolasi kelompok, partial payment | Tambahkan skenario bulan nonaktif |
| `iuran_inactive_test.go` | Nonaktif pada tingkat jenis iuran | Pertahankan dan bedakan dari status tagihan |
| `multi_period_payment_test.go` | Pembuatan dan validasi pembayaran multi-periode | Verifikasi dampak filter status |
| `due_status_test.go` | Status jenis iuran | Verifikasi tidak terjadi regresi |
| `actual_received_date_test.go` | Validasi tanggal penerimaan | Pertahankan |
| `donation_integration_test.go` | Pembayaran Sukarela | Jalankan sebagai regresi |

Hasil eksekusi test terbaru belum dibuktikan oleh laporan ini. Status pass/fail harus diperoleh dari eksekusi aktual pada lingkungan yang aman.

---

## 4. Matriks Keputusan Bisnis vs Implementasi

| Keputusan | Kondisi yang dilaporkan | Status |
|---|---|---|
| Memilih tahun kalender | Input tahun tersedia | Sesuai |
| Membuat Januari–Desember | Generator menggunakan loop 1–12 | Sesuai |
| Format periode `YYYY-MM` | Digunakan generator | Sesuai |
| Jatuh tempo tanggal 10 | Dilaporkan tersedia | Perlu verifikasi kode |
| Semua 12 bulan aktif default | Tidak ada status per tagihan | Belum sesuai |
| Penerbitan tidak dimulai dari bulan berjalan | Generator tidak memfilter bulan berjalan | Sesuai |
| Mencegah duplikasi | Constraint unik dan `ON CONFLICT DO NOTHING` | Sesuai |
| Status per bulan dikelola manual | Belum tersedia | Belum sesuai |
| UI menampilkan 12 bulan | Belum tersedia | Belum sesuai |
| Status tidak berubah otomatis | Tidak ada scheduler yang dilaporkan | Sesuai menurut audit |
| Tagihan nonaktif tidak dapat dibayar | Belum ada status per tagihan | Belum sesuai |
| Tagihan lunas tidak dapat dinonaktifkan | Belum ada mekanisme per tagihan | Belum sesuai |
| FIFO aktif-tertua | Pemeriksaan FIFO sudah ada | Sebagian sesuai |
| FIFO dengan bulan nonaktif | Belum ada implementasi | Belum sesuai |
| `actual_received_date` | Dilaporkan diwajibkan saat approval | Sesuai menurut audit |
| KAS mengikuti tanggal penerimaan | Dilaporkan memakai `occurred_at` dari tanggal aktual | Sesuai menurut audit |
| Satu KAS per pembayaran multi-tagihan | Dilaporkan tersedia | Sesuai menurut audit |
| Reguler/Sukarela tidak berubah | Alur dilaporkan terisolasi | Wajib diuji regresi |

---

## 5. Analisis Skenario Wajib

### Skenario 1 — Penerbitan pada Oktober

**Kondisi:** Tahun 2026 dipilih ketika bulan berjalan Oktober.

**Ekspektasi:** Semua periode Januari–Desember 2026 tersedia dan aktif secara default.

**Kondisi yang dilaporkan:** Generator sudah membuat 12 periode.

**Perubahan yang diperlukan:** Pastikan status `is_active` ditetapkan `true` ketika tagihan baru dibuat. Penerbitan ulang tidak boleh menimpa status yang sudah diubah bendahara.

### Skenario 2 — Bendahara menonaktifkan Januari

**Kondisi:** Januari aktif dan belum lunas.

**Ekspektasi:** Bendahara dapat menonaktifkan Januari melalui pengaturan per tagihan.

**Validasi yang diperlukan:**
- Pengguna memiliki otorisasi yang benar.
- Tagihan memang milik jenis iuran Multi-Bulan yang relevan.
- Tagihan belum lunas.
- Tidak ada konflik dengan pembayaran pending yang menyebabkan status pembayaran menjadi tidak konsisten.
- Perubahan tidak menghapus tagihan atau riwayat.

### Skenario 3 — FIFO dengan bulan nonaktif di tengah

**Kondisi:**
- Januari aktif dan belum lunas.
- Februari nonaktif.
- Maret aktif dan belum lunas.

**Ekspektasi:**
- Pembayaran Maret saja ditolak karena Januari masih aktif dan belum lunas.
- Setelah Januari lunas, Maret boleh dibayar meskipun Februari tetap nonaktif.

**Perubahan yang diperlukan:** Query FIFO harus memperhitungkan status aktif. Tagihan aktif tertua yang belum lunas tetap menghalangi pembayaran bulan aktif yang lebih baru. Tagihan nonaktif tidak dapat dibayar dan tidak menjadi penghalang FIFO.

### Skenario 4 — Tunggakan Agustus dan September

**Kondisi:**
- Agustus aktif dan belum lunas.
- September aktif dan belum lunas.
- Oktober aktif.

**Ekspektasi:** Pembayaran dua bulan harus melunasi Agustus lalu September. Pembayaran Oktober tidak boleh dipilih dengan melewati salah satu dari dua tagihan tersebut.

### Skenario 5 — Mengaktifkan kembali bulan lama

**Kondisi:** Januari nonaktif, kemudian diaktifkan pada Oktober. Warga membayar pada 15 Oktober.

**Ekspektasi:**
- Januari dapat dibayar setelah diaktifkan.
- Januari menjadi lunas setelah pembayaran disetujui.
- Riwayat pembayaran tetap menunjukkan tagihan Januari.
- KAS masuk dan saldo mengikuti tanggal penerimaan aktual pada Oktober.

### Skenario 6 — Pembayaran pending saat status berubah

**Kondisi:** Pembayaran sudah pending untuk tagihan aktif, kemudian bendahara mencoba menonaktifkan tagihan sebelum approval.

**Ekspektasi:** Sistem harus mencegah inkonsistensi antara status tagihan, alokasi pembayaran, dan approval.

**Keputusan teknis yang harus dipastikan:** Penonaktifan harus ditolak atau proses approval harus menolak pembayaran yang sudah tidak valid. Pilihan implementasi harus konsisten, transaksional, dan diuji; jangan membiarkan pembayaran yang tidak valid disetujui secara diam-diam.

### Skenario 7 — Tagihan lunas

**Kondisi:** Tagihan sudah lunas.

**Ekspektasi:** Bendahara tidak dapat menonaktifkannya melalui pengaturan biasa. Riwayat pembayaran, alokasi, dan KAS harus tetap utuh.

### Skenario 8 — Penerbitan ulang

**Kondisi:** Sebagian atau semua tagihan sudah dibuat dan beberapa statusnya telah diubah.

**Ekspektasi:**
- Tidak ada duplikasi.
- Status yang sudah diatur bendahara tidak di-reset secara tidak sengaja.
- Tagihan yang sudah lunas tetap lunas.
- Penerbitan ulang tidak membuat transaksi KAS.

### Skenario 9 — Regresi Reguler dan Sukarela

**Ekspektasi:** Alur Reguler dan Sukarela tidak berubah. Perubahan status bulanan hanya berlaku pada Multi-Bulan.

**Pengujian:** Jalankan test pembayaran, FIFO, approval, KAS, dan test Sukarela yang relevan setelah implementasi.

---

## 6. Spesifikasi FIFO untuk Implementasi

Bagian ini menjadi acuan penerapan status aktif/nonaktif terhadap FIFO.

### 6.1 Urutan pembayaran

Dalam setiap kelompok FIFO yang berlaku, sistem harus:

1. Mengambil tagihan yang aktif dan belum lunas.
2. Mengurutkan berdasarkan periode tertua terlebih dahulu.
3. Memastikan tagihan yang dipilih untuk pembayaran cocok dengan urutan tersebut.
4. Menolak pembayaran yang melewati tagihan aktif yang lebih tua dan belum lunas.
5. Mengabaikan tagihan nonaktif untuk kelayakan pembayaran, tetapi tetap menyimpan tagihan dan riwayatnya.

### 6.2 Contoh perilaku

| Januari | Februari | Maret | Perilaku |
|---|---|---|---|
| Aktif, belum lunas | Nonaktif | Aktif, belum lunas | Maret ditolak sampai Januari lunas |
| Lunas | Nonaktif | Aktif, belum lunas | Maret boleh dibayar |
| Nonaktif | Aktif, belum lunas | Aktif, belum lunas | Februari harus didahulukan sebelum Maret |
| Aktif, belum lunas | Aktif, belum lunas | Aktif, belum lunas | Pembayaran mengikuti urutan Januari, Februari, Maret |
| Aktif, pending payment | Nonaktif | Aktif | Sistem harus menerapkan aturan pending dan locking yang konsisten |

### 6.3 Pembayaran multi-periode

Untuk pembayaran beberapa bulan sekaligus:
- Semua tagihan yang dipilih harus aktif dan belum lunas.
- Urutan pilihan harus sesuai urutan FIFO tagihan aktif yang belum lunas.
- Tagihan nonaktif tidak dapat dimasukkan ke daftar alokasi.
- Jumlah tagihan yang dibayar tidak boleh melebihi tagihan aktif yang tersedia dan memenuhi FIFO.
- Validasi harus dilakukan di backend meskipun UI sudah menyembunyikan tagihan nonaktif.

### 6.4 Pembayaran pending

Pending payment harus diperhitungkan untuk mencegah overpayment dan alokasi ganda. Perubahan status harus memeriksa pembayaran pending terkait dengan locking/transaksi yang sesuai. Implementasi tidak boleh hanya mengandalkan status `is_active` yang dibaca sebelumnya tanpa validasi ulang saat approval.

---

## 7. Perubahan Minimum yang Diperlukan

Daftar berikut adalah rencana perubahan berdasarkan temuan audit, bukan konfirmasi bahwa perubahan tersebut sudah dibuat.

### 7.1 Database

| Perubahan | Tujuan |
|---|---|
| Tambahkan `bills.is_active BOOLEAN NOT NULL DEFAULT true` | Menyimpan status aktif/nonaktif per tagihan |
| Pertahankan kolom `bills.status` yang mengatur status pembayaran | Memisahkan status kelayakan pembayaran dari status pelunasan |
| Pertimbangkan audit log perubahan status | Mengetahui siapa yang mengubah status dan kapan |
| Tambahkan index hanya jika query plan dan volume data membenarkannya | Menghindari indeks yang tidak diperlukan |

Kolom `is_active` harus diperlakukan sebagai status yang terpisah dari `unpaid`, `paid`, dan `cancelled`. Jangan menggunakan `cancelled` sebagai pengganti status nonaktif karena keduanya memiliki makna berbeda.

### 7.2 Backend — Model

File yang diperkirakan terdampak:
- `backend/internal/finance/model.go`

Kebutuhan:
- Tambahkan field status aktif pada model tagihan.
- Perbarui request/response yang relevan.
- Pastikan serialisasi dan validasi konsisten.

### 7.3 Backend — Repository

File yang diperkirakan terdampak:
- `backend/internal/finance/repository.go`

Kebutuhan:
- Menyimpan `is_active=true` untuk tagihan baru.
- Menyediakan operasi perubahan status tagihan.
- Memuat status aktif dalam query yang digunakan UI dan pembayaran.
- Memperbarui query FIFO untuk menggunakan tagihan aktif yang belum lunas.
- Memastikan query pembayaran dan pending tetap menghitung alokasi secara benar.
- Mencegah perubahan status pada tagihan lunas.

### 7.4 Backend — Service

File yang diperkirakan terdampak:
- `backend/internal/finance/service.go`

Kebutuhan:
- Validasi perubahan status.
- Validasi tagihan aktif pada pembayaran single dan multi-periode.
- FIFO yang konsisten dengan status aktif.
- Pemeriksaan ulang pada approval.
- Pengamanan terhadap pembayaran pending, overpayment, dan konkurensi.
- Memastikan penerbitan ulang tidak mengubah status yang telah dipilih bendahara.

Jangan mengubah `generateMultiMonthBills()` jika pemeriksaan ulang membuktikan generator tersebut sudah sesuai. Perubahan generator hanya diperlukan jika ada masalah nyata terkait status awal atau penerbitan ulang.

### 7.5 Backend — Handler dan API

File yang diperkirakan terdampak:
- `backend/internal/finance/handler.go`
- Routing dan middleware terkait.

Kebutuhan:
- Endpoint untuk mengubah status aktif/nonaktif per tagihan.
- Validasi otorisasi bendahara di backend.
- Respons error yang jelas untuk tagihan lunas, tagihan tidak ditemukan, jenis iuran tidak sesuai, atau konflik pembayaran pending.
- API daftar tagihan harus mengembalikan status aktif.

Nama endpoint final ditentukan setelah pola routing dan API yang ada diperiksa. Jangan mengasumsikan endpoint baru harus menggunakan metode atau path tertentu tanpa mempertimbangkan konvensi repository.

### 7.6 Frontend Web

File yang diperkirakan terdampak:
- `frontend/src/features/iuran/IuranList.tsx`
- `frontend/src/features/iuran/IuranPayment.tsx`
- `frontend/src/features/iuran/DueCreate.tsx`, hanya jika ditemukan kebutuhan nyata.
- API client dan tipe data terkait.

Kebutuhan:
- Menu Edit Iuran untuk Multi-Bulan menampilkan Januari–Desember.
- Setiap bulan memiliki indikator dan kontrol aktif/nonaktif.
- Tagihan lunas tidak dapat dinonaktifkan melalui pengaturan biasa.
- Perubahan status memiliki feedback sukses/gagal yang jelas.
- Daftar pembayaran hanya menyediakan tagihan yang aktif dan memenuhi FIFO.
- UI tidak menjadi satu-satunya lapisan validasi.

### 7.7 Mobile Flutter

File yang diperkirakan terdampak:
- `mobile/lib/features/iuran/presentation/screens/iuran_management_screen.dart`
- `mobile/lib/features/iuran/presentation/screens/iuran_payment_screen.dart`
- `mobile/lib/features/iuran/data/iuran_api_client.dart`
- Model/API response terkait.

Kebutuhan:
- Menampilkan 12 bulan dan status masing-masing.
- Menyediakan kontrol status sesuai hak akses bendahara.
- Menampilkan pesan ketika perubahan ditolak.
- Memastikan daftar pembayaran dan jumlah bulan mengikuti status aktif serta FIFO.
- Menangani perubahan status yang terjadi ketika layar pembayaran masih terbuka.

### 7.8 Audit Log

Audit log perubahan status sangat disarankan untuk akuntabilitas keuangan. Jika belum ada mekanisme audit yang cocok, evaluasi kebutuhan untuk mencatat:
- Tagihan yang diubah.
- Status sebelum dan sesudah perubahan.
- Pengguna yang melakukan perubahan.
- Waktu perubahan.
- Alasan perubahan jika memang diwajibkan oleh kebijakan.

Jangan menambahkan skema audit baru sebelum memeriksa apakah repository sudah memiliki mekanisme audit yang dapat digunakan kembali.

---

## 8. Risiko dan Kompatibilitas

### 8.1 Data lama

Migration `is_active DEFAULT true` akan membuat tagihan lama aktif jika tidak diberikan aturan backfill lain. Ini sesuai keputusan bahwa semua 12 bulan aktif secara default, tetapi perlu diverifikasi dampaknya terhadap tagihan lama yang mungkin sudah dibuat sebelumnya.

Jangan menonaktifkan tagihan lama secara otomatis hanya berdasarkan periode kalender. Keputusan final menetapkan bahwa status dikelola manual oleh bendahara.

### 8.2 Penerbitan ulang

`ON CONFLICT DO NOTHING` menjaga agar tagihan yang sudah ada tidak diduplikasi. Implementasi harus memastikan penerbitan ulang tidak mereset status aktif/nonaktif yang telah diatur bendahara.

### 8.3 Pembayaran pending

Perubahan status saat ada pembayaran pending merupakan risiko utama. Sistem harus menggunakan validasi transaksional yang konsisten agar:
- Pembayaran tidak dialokasikan dua kali.
- Tagihan nonaktif tidak dibayar melalui jalur yang tidak memeriksa status.
- Approval tidak mengesahkan alokasi yang sudah tidak valid.
- Perhitungan pending tidak membuka peluang overpayment.

### 8.4 KAS dan saldo

Perubahan status tidak boleh membuat transaksi KAS baru atau mengubah tanggal transaksi yang sudah ada. KAS hanya diposting melalui alur pembayaran yang sah dan menggunakan tanggal penerimaan aktual sesuai ketentuan yang berlaku.

### 8.5 Regresi Reguler/Sukarela

Query bersama dan model bersama perlu diperiksa. Jangan menerapkan filter status baru pada alur Reguler atau Sukarela secara tidak sengaja. Pastikan pengujian membuktikan perilaku lama tetap sama.

### 8.6 Status jenis iuran vs status tagihan

`dues.is_active` dan status aktif pada `bills` memiliki cakupan berbeda:
- `dues.is_active`: mengatur status jenis iuran.
- `bills.is_active`: mengatur kelayakan pembayaran satu tagihan bulanan.

Validasi harus menentukan bagaimana kedua status berinteraksi. Jangan menganggap tagihan individual yang aktif otomatis dapat dibayar jika jenis iurannya sendiri tidak aktif, tanpa memeriksa perilaku dan aturan bisnis yang sudah ada.

---

## 9. Rencana Pengujian

### 9.1 Test baru

| No. | Test | Ekspektasi |
|---|---|---|
| 1 | Bendahara menonaktifkan tagihan belum lunas | Berhasil jika tidak ada konflik pembayaran |
| 2 | Bendahara mengaktifkan kembali tagihan nonaktif | Berhasil jika belum lunas dan valid |
| 3 | Warga mencoba mengubah status tagihan | Ditolak oleh backend |
| 4 | Tagihan lunas dinonaktifkan | Ditolak |
| 5 | Pembayaran langsung ke tagihan nonaktif | Ditolak |
| 6 | FIFO: Januari aktif belum lunas, Februari nonaktif, Maret aktif | Maret ditolak sampai Januari lunas |
| 7 | FIFO: Januari lunas, Februari nonaktif, Maret aktif | Maret boleh dibayar |
| 8 | FIFO: Januari nonaktif, Februari aktif belum lunas, Maret aktif | Februari harus didahulukan |
| 9 | Pembayaran Agustus dan September | Dialokasikan sesuai urutan FIFO |
| 10 | Multi-periode memasukkan tagihan nonaktif | Ditolak |
| 11 | Pembayaran pending saat bendahara menonaktifkan tagihan | Hasil konsisten; tidak ada approval/alokasi yang invalid |
| 12 | Status berubah sebelum approval | Approval memvalidasi ulang status dan alokasi |
| 13 | Penerbitan ulang setelah status diubah | Tidak membuat duplikat atau mereset status |
| 14 | KAS dari pembayaran tagihan lama | Tanggal KAS mengikuti tanggal penerimaan aktual |
| 15 | Web Edit Iuran Multi-Bulan | Menampilkan 12 bulan dan status |
| 16 | Mobile Edit Iuran Multi-Bulan | Menampilkan 12 bulan dan status |
| 17 | Regresi Reguler | Alur lama tetap sama |
| 18 | Regresi Sukarela | Alur lama tetap sama |

### 9.2 Test regresi yang dilaporkan relevan

- `multi_month_test.go`
- `fifo_test.go`
- `iuran_inactive_test.go`
- `multi_period_payment_test.go`
- `due_status_test.go`
- `actual_received_date_test.go`
- `donation_integration_test.go`
- `due_voluntary_test.go`
- `multi_period_settlement_test.go`
- `overpayment_transaction_test.go`

Nama test dan path harus diverifikasi terhadap repository aktual. Jangan menyatakan test lulus sebelum benar-benar menjalankannya.

### 9.3 Lingkungan test

Laporan audit sebelumnya menyebutkan bahwa test membutuhkan PostgreSQL aktif di container Docker. Sebelum menjalankan test, pastikan konfigurasi menggunakan database test yang terisolasi dan tidak mengubah data produksi atau database bersama.

Hasil test yang belum dijalankan harus dilaporkan sebagai belum diverifikasi, bukan diasumsikan lulus.

---

## 10. Pertanyaan Bisnis yang Masih Terbuka

| ID | Pertanyaan | Status |
|---|---|---|
| Q-01 | Apakah tagihan lunas boleh dinonaktifkan? | **Terjawab:** tidak melalui pengaturan biasa |
| Q-02 | Apakah tagihan nonaktif menghalangi FIFO? | **Terjawab:** tidak dapat dibayar dan tidak menghalangi tagihan aktif berikutnya; tagihan aktif tertua yang belum lunas tetap wajib didahulukan |
| Q-03 | Apakah tagihan nonaktif bisa diaktifkan kembali? | **Terjawab:** ya, selama belum lunas dan validasi lainnya terpenuhi |
| Q-04 | Bagaimana menangani pembayaran pending ketika status tagihan berubah? | Perlu keputusan teknis terperinci dan pengujian. Sistem wajib mencegah inkonsistensi |
| Q-05 | Apakah Edit Iuran perlu menampilkan status 12 bulan? | **Terjawab:** ya, untuk Multi-Bulan |
| Q-06 | Apakah tagihan tahun sebelumnya dapat diaktifkan? | Belum ditentukan secara khusus; verifikasi kebutuhan dan perilaku yang ada |
| Q-07 | Apakah perlu audit log perubahan status? | Direkomendasikan; periksa mekanisme audit yang sudah tersedia |
| Q-08 | Bagaimana tagihan warga baru/pindah di tengah tahun? | Belum diputuskan; verifikasi desain dan implementasi saat ini sebelum mengusulkan perubahan |

---

## 11. Rekomendasi Tahap Implementasi

### Tahap 1 — Verifikasi sebelum coding

1. Periksa `git status` dan perubahan yang belum di-commit.
2. Verifikasi ulang kode dan skema yang dirujuk laporan audit.
3. Periksa interaksi `dues.is_active` dengan status aktif/nonaktif setiap bill.
4. Periksa pembayaran pending dan jalur approval.
5. Pastikan migration dan test memakai lingkungan yang aman.
6. Konfirmasi aturan Q-04 secara teknis sebelum memilih perilaku perubahan status ketika pembayaran pending.

### Tahap 2 — Implementasi status per tagihan

1. Tambahkan kolom status aktif/nonaktif pada `bills` jika belum tersedia.
2. Pastikan tagihan baru aktif secara default.
3. Tambahkan operasi repository, service, dan endpoint untuk perubahan status.
4. Terapkan otorisasi dan validasi di backend.
5. Perbarui query tagihan dan FIFO.
6. Perbarui validasi pembayaran single dan multi-periode.
7. Lindungi approval serta pembayaran pending.
8. Pastikan penerbitan ulang tidak mereset status.
9. Implementasikan UI Web dan Mobile.
10. Tambahkan test untuk seluruh skenario wajib.

### Tahap 3 — Verifikasi

1. Jalankan test backend yang relevan.
2. Jalankan test Web.
3. Jalankan test Flutter.
4. Verifikasi penerbitan 12 bulan dan penerbitan ulang.
5. Verifikasi FIFO dengan bulan nonaktif di tengah.
6. Verifikasi pembayaran pending, approval, KAS, dan saldo.
7. Verifikasi regresi Reguler dan Sukarela.
8. Lakukan pemeriksaan manual UI Web dan Android bila lingkungan tersedia.
9. Laporkan hasil aktual, file yang berubah, serta keterbatasan verifikasi.

---

## 12. Kesimpulan

**Yang sudah dilaporkan benar:**
- Generator 12 tagihan bulanan Januari–Desember.
- Pencegahan duplikasi penerbitan.
- FIFO dasar untuk pembayaran single dan multi-periode.
- Pembayaran multi-tagihan menghasilkan satu transaksi KAS.
- `actual_received_date` digunakan dalam pencatatan KAS.
- Pembayaran pending belum melunasi tagihan.
- Terdapat mekanisme locking dan idempotensi.
- Alur Reguler dan Sukarela telah dipisahkan secara logis.

**Yang perlu diimplementasikan:**
- Status aktif/nonaktif per tagihan bulanan.
- Endpoint dan validasi untuk perubahan status.
- UI Web dan Mobile untuk pengaturan 12 bulan.
- FIFO yang mempertimbangkan status aktif.
- Perlindungan pembayaran pending dan approval.
- Pengujian regresi dan skenario status bulanan.

**Keputusan bisnis yang sudah final:**
- Semua 12 bulan aktif secara default.
- Bendahara mengatur status secara manual.
- Status tidak berubah otomatis saat bulan berganti.
- Tagihan lunas tidak boleh dinonaktifkan melalui pengaturan biasa.
- Tagihan nonaktif tidak dapat dibayar dan tidak menghalangi pembayaran tagihan aktif berikutnya.
- FIFO wajib mendahulukan tagihan aktif tertua yang belum lunas.
- KAS mengikuti tanggal penerimaan aktual.
- Reguler dan Sukarela tidak boleh berubah.

**Hal yang masih perlu diverifikasi atau diputuskan:**
- Penanganan pembayaran pending ketika status berubah.
- Kebijakan tagihan tahun sebelumnya.
- Perilaku tagihan warga baru/pindah di tengah tahun.
- Ketersediaan mekanisme audit log.
- Kondisi aktual repository, skema, dan test pada saat implementasi.

**Rekomendasi:** implementasi dapat dimulai setelah verifikasi singkat kondisi repository dan penyelesaian rancangan teknis untuk pembayaran pending. Fokus pada perubahan minimum yang diperlukan, tanpa refactor luas dan tanpa mengubah alur Reguler maupun Sukarela.

---

*Dokumen ini adalah spesifikasi keputusan dan rencana implementasi berdasarkan laporan audit yang diberikan. Dokumen ini sendiri tidak membuktikan bahwa perubahan kode atau database telah dilakukan atau diuji.*

---

## 12. Catatan Implementasi End-to-End & Verifikasi Aktual (2026-10-28)

Fitur pengelolaan status aktif/nonaktif untuk 12 tagihan Multi-Bulan telah selesai diimplementasikan end-to-end pada branch `urena`:

1. **Database & Migrasi:**
   - Dibuat `backend/migrations/039_bills_is_active.up.sql` dan `.down.sql`.
   - Menambahkan kolom `is_active BOOLEAN NOT NULL DEFAULT true` pada tabel `bills`.
   - Menambahkan indeks komposit: `idx_bills_rt_is_active` (`rt_id`, `is_active`) dan `idx_bills_occ_due_active` (`household_occupancy_id`, `due_id`, `status`, `is_active`).
   - Migrasi berhasil diaplikasikan via `docker compose run --rm migrate up`.

2. **Backend (Go REST API):**
   - Model `Bill`, `DueMonthSummary`, dan `UpdateBillStatusInput`.
   - Repository: `BillUpdateActiveStatus`, `DueGetMonths`, `DueUpdatePeriodActiveStatus`, pembaruan `BillList` & `BillCount` (filter `is_active`), dan `GetOldestUnpaidBillID`/`GetOldestUnpaidBillIDs` (`AND b.is_active = true`).
   - Service:
     - Deaktivasi tagihan yang sudah lunas ditolak dengan pesan error Indonesia `"tagihan yang sudah lunas tidak dapat dinonaktifkan"`.
     - Deaktivasi seluruh periode yang sudah lunas ditolak.
     - Pembayaran single & multi-period menolak tagihan nonaktif.
     - Persetujuan pembayaran (`verifySinglePayment` & `verifyMultiPeriodPayment`) memvalidasi status aktif tagihan di dalam transaksi database ber-lock `FOR UPDATE`. Jika tagihan dinonaktifkan saat pembayaran pending, approval gagal secara atomik tanpa mutasi status atau transaksi KAS.
   - Handlers & Router:
     - `GET /api/v1/dues/{id}/months`
     - `PATCH /api/v1/dues/{id}/periods/{period}/status`
     - `PATCH /api/v1/bills/{id}/status`
   - Test suite: `TestMultiMonth_ActiveInactiveManagementAndFIFO` dan seluruh test `TestMultiMonth_*`, `TestFIFO_*`, `TestO12_*`, dan `TestMultiPeriod_*` lulus (PASS, exit code 0).

3. **Web Frontend (React/TypeScript):**
   - `frontend/src/app/api.ts`: API clients `apiGetDueMonths`, `apiUpdateDuePeriodStatus`, `apiUpdateBillStatus`, serta pembaruan `ApiBill`.
   - `frontend/src/features/iuran/IuranList.tsx`: Panel kalender 12 bulan di dalam Modal Edit Iuran untuk tipe `multi_month` dengan toggle status aktif/nonaktif dan proteksi tombol lunas, serta badge status pada tabel tagihan.
   - `frontend/src/features/iuran/IuranPayment.tsx`: Filter tagihan payable hanya untuk tagihan aktif (`is_active = true`).
   - `frontend/src/features/iuran/IuranDetail.tsx`: Penampilan badge status tidak aktif.
   - Vitest: 102/102 test lulus (`npx vitest run src/features/iuran/`).
   - Build: `npm run build` sukses (`tsc -b && vite build` built in 11.64s).

4. **Android Client (Flutter/Dart):**
   - Data models: `isActive` pada `BackendBill` dan `IuranBill`, serta class `DueMonthSummary`.
   - API client & Repository: `fetchDueMonths`, `updateDuePeriodStatus`, `updateBillStatus`, dan filter `isActive`.
   - UI: Dialog Edit Iuran (`kelola_iuran_tab_body.dart`) menyediakan daftar status 12 bulan dan tombol toggle untuk Bendahara. `iuran_payment_screen.dart` membatasi pilihan tagihan pada tagihan yang aktif (`isActive == true`). `iuran_detail_screen.dart` dan `iuran_screen.dart` menampilkan badge 'Tidak Aktif' dan memblokir FAB bayar untuk tagihan nonaktif.
   - Flutter tests: `iuran_multi_period_payment_test.dart` dan `iuran_models_test.dart` (35 tests) lulus 100%. Flutter analyze bersih tanpa error.

---

## 13. Investigasi dan Perbaikan Anomali Multi-Bulan (2026-10-28)

Berdasarkan laporan pengujian di lingkungan dev Urena, ditemukan tiga anomali utama pada fitur Multi-Bulan:

### 13.1 Analisis & Akar Masalah Terbukti

1. **Munculnya 4 Baris Konfigurasi Iuran di Kelola Iuran (`IPL 20206` x3 dan `IPL 2026` x1):**
   - **Bukti DB & Audit Log:** Tabel `dues` mencatat tiga record `IPL 20206` (nominal Rp1.000.000) yang dibuat beruntun pada detik 05:28:56, 05:29:00, dan 05:29:05, disusul satu record `IPL 2026` (nominal Rp100.000) pada 05:29:21.
   - **Akar Masalah:**
     - `POST /api/v1/dues` pada backend belum diproteksi oleh middleware/session `HandleIdempotentRequest`.
     - Frontend `DueCreate.tsx` tidak mengirimkan header `Idempotency-Key` saat memanggil `apiCreateDue`.
     - `DueCreate.tsx` memanggil `apiGenerateBills` secara manual setelah `apiCreateDue`, padahal backend `CreateDue` sudah otomatis memanggil generator tagihan di dalam transaksi yang sama. Keterlambatan respons pemanggilan ganda ini memicu kemungkinan klik berulang oleh pengguna.
     - Keempat konfigurasi tidak pernah menerima pembayaran (`payments_count = 0`) dan tidak memiliki transaksi KAS.

2. **Hilangnya Periode Januari–Agustus dan September Hanya Memiliki 6 Tagihan:**
   - **Bukti DB:** Dari 11 hunian KK aktif di RT terkait, 6 KK memiliki `start_date` pada 20-25 September 2026, dan 5 KK (staf) didaftarkan pada 28 Oktober 2026. Tidak ada KK yang memiliki `start_date <= 2026-08-31`.
   - **Akar Masalah Kode:** Di `backend/internal/finance/service.go` (`generateMultiMonthBills`), generator membandingkan tanggal hunian terhadap akhir bulan tagihan (`*occ.StartDate > periodEnd`). Akibatnya:
     - Untuk Januari–Agustus (`periodEnd <= 2026-08-31`), seluruh 11 KK dilewati (`continue`), menghasilkan 0 tagihan.
     - Untuk September (`periodEnd = 2026-09-30`), hanya 6 KK yang sudah ada, menghasilkan 6 tagihan.
     - Untuk Oktober–Desember, seluruh 11 KK sudah terdaftar, menghasilkan 11 tagihan per bulan.
   - **Konflik Aturan:** Iuran Multi-Bulan bersifat kalender tahunan penuh (12 bulan). Tanggal pendaftaran sistem hunian di tengah tahun berjalan tidak boleh menyebabkan hilangnya tagihan Januari–Agustus.

3. **Gejala Tampilan "Iuran Saya" (12 Baris Mengulang Okt-Nov-Des) dan Angka Identik pada Ringkasan Modal:**
   - **Akar Masalah "Iuran Saya":** Akun warga yang login adalah salah satu dari 5 hunian staf yang terdaftar tanggal 28 Oktober. Karena ada 4 konfigurasi iuran, dan untuk masing-masing konfigurasi akun staf tersebut memiliki 3 tagihan (Okt, Nov, Des), total tagihan yang tampil adalah 3 bulan x 4 konfigurasi = 12 baris.
   - **Akar Masalah Ringkasan Modal:** Query `GET /api/v1/dues/{id}/months` sudah menyaring secara benar per `due_id`, namun karena generator menghasilkan data yang persis sama untuk keempat-empatnya, setiap modal menampilkan angka ringkasan yang identik (0 Jan–Ags, 6 Sep, 11 Okt–Des).
   - **Akar Masalah Urutan:** Query `BillList` sebelumnya menggunakan default `ORDER BY b.created_at DESC`, sehingga urutan tagihan warga tidak terurut kronologis dari Januari sampai Desember.

---

### 13.2 Perbaikan yang Dilakukan

1. **Backend Generator (`backend/internal/finance/service.go`):**
   - Logika filter hunian diubah dari pemotongan per-bulan menjadi batas tahun kalender:
     `occ.StartDate > yearEnd` dan `occ.EndDate < yearStart`.
   - Seluruh hunian yang aktif di RT pada tahun kalender terkait dijamin menerima seluruh 12 periode (Januari sampai Desember).

2. **Idempotensi & Proteksi Double Submission (`handler.go`, `api.ts`, `DueCreate.tsx`, `iuran_api_client.dart`):**
   - Menambahkan `HandleIdempotentRequest` pada `POST /api/v1/dues` di backend Go.
   - Frontend web `apiCreateDue` dan Flutter `createDue` kini mendukung parameter `idempotencyKey`.
   - `DueCreate.tsx` mengirim `crypto.randomUUID()`, memvalidasi guard `if (loading) return`, dan menghapus pemanggilan ganda `apiGenerateBills` untuk jenis multi-bulan.

3. **Urutan Kronologis & Filter Tagihan Aktif Warga (`repository.go`, `handler.go`):**
   - Query `BillList` saat diakses warga (`occupancyID != nil`) diurutkan secara kronologis: `ORDER BY b.period ASC, b.created_at ASC`.
   - Tagihan yang dilihat warga di tab "Iuran Saya" (`my_bills=true`) default memfilter tagihan yang aktif (`b.is_active = true`), sehingga tagihan dari due yang dinonaktifkan tidak mencemari daftar tagihan warga.

4. **Rekonsiliasi Database Aktual:**
   - Tiga konfigurasi salah ketik `IPL 20206` dinonaktifkan (`is_active = false`), beserta 117 tagihan unpaid-nya.
   - Konfigurasi sah `IPL 2026` diterbitkan tagihan Januari–Agustus dan sisa September yang hilang (93 tagihan baru) dengan `ON CONFLICT DO NOTHING`, menghasilkan total 132 tagihan (11 KK x 12 bulan).

---

### 13.3 Hasil Verifikasi Aktual

- **Backend Go:** Seluruh test suite `finance` (termasuk `TestMultiMonth_*`, `TestFIFO_*`, `TestO12_*`, dan `TestDueActiveStatusLifecycle`) lulus 100% (`ok social-finance/internal/finance 12.303s`).
- **Web Frontend:** Seluruh test `src/features/iuran/` lulus 100% (7 test files, 102 tests passed). Build production `npm run build` berhasil (`built in 20.98s`).
- **Android Mobile:** Test suite `iuran_multi_period_payment_test.dart`, `iuran_form_screen_test.dart`, dan `iuran_models_test.dart` lulus 100% (47 tests passed).
- **Verifikasi HTTP Live:**
  - `GET /api/v1/dues/06700233-8290-4adc-91f0-b6bb7673924f/months`: Mengembalikan 11 tagihan per bulan untuk ke-12 bulan dari 2026-01 s/d 2026-12.
  - `GET /api/v1/bills?my_bills=true` (sebagai warga): Mengembalikan tepat 12 tagihan `IPL 2026` dari periode `2026-01` sampai `2026-12` secara urut kronologis, tanpa tagihan nonaktif dari konfigurasi salah ketik.

