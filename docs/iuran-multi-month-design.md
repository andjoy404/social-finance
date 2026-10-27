# Iuran Reguler Multi-Bulan — Desain & Spesifikasi

> **Dibuat:** 2026-10-26  
> **Direvisi:** 2026-10-27  
> **Status:** Design Draft — Pending Stakeholder Confirmation  
> **Cakupan:** Dokumentasi dan desain saja. Tidak ada perubahan kode, skema database, migration, API, atau UI dalam dokumen ini.

## Tujuan Dokumen

Dokumen ini menjadi acuan tunggal untuk analisis fitur Iuran Reguler satu atau beberapa bulan sekaligus. Dokumen membedakan:
- **Keputusan bisnis yang sudah final** dan harus dipertahankan.
- **Perilaku saat ini yang dilaporkan**, yang belum dianggap terverifikasi sampai diperiksa terhadap kode dan tes.
- **Keputusan terbuka**, yang harus diputuskan stakeholder atau diverifikasi pada analisis implementasi.

Jangan menganggap penulisan spesifikasi, keberadaan file kode, atau keberadaan file tes sebagai bukti fitur telah diimplementasikan atau tes telah lulus.

---

## 1. Jenis Iuran

| Jenis | Ketentuan | Penerbitan |
|---|---|---|
| **Sukarela** | Nominal ditentukan saat membayar; tidak memiliki kewajiban tagihan bulanan tetap; tidak tunduk pada FIFO tagihan Reguler. **Alur, UI, validasi, dan perilaku Sukarela harus tetap persis seperti implementasi saat ini.** | Tidak memerlukan penerbitan tagihan tetap. |
| **Reguler** | Iuran wajib bertarif tetap. Satu alur pembayaran digunakan untuk satu tagihan maupun beberapa tagihan bulanan sekaligus. Setiap tagihan yang dialokasikan harus dibayar penuh; pembayaran parsial per tagihan tidak diizinkan. | Dikelola oleh Bendahara/Ketua; satu tagihan terpisah per bulan per unit tagihan yang disepakati, dapat diterbitkan sekaligus untuk beberapa bulan tanpa entri manual satu per satu. |

Pilihan **Reguler Bulanan** dan **Reguler Multi-Bulan** di form **PEMBAYARAN IURAN** digantikan oleh satu pilihan permanen: **Reguler**.

Jenis iuran dan jumlah tagihan dalam satu transaksi adalah konsep berbeda. `Reguler` tetap merupakan satu jenis iuran, baik satu maupun beberapa tagihan dibayar dalam satu pengajuan.

Perubahan ini merupakan keputusan desain untuk implementasi berikutnya, bukan pernyataan bahwa kode saat ini sudah berubah.

---

## 2. Penerbitan 12 Tagihan Bulanan

Untuk setiap unit yang memenuhi syarat, sistem perlu mendukung **12 tagihan terpisah untuk satu tahun kalender**. Contoh tahun 2026: Januari sampai Desember 2026.

1. Setiap bulan adalah tagihan tersendiri dengan identitas, periode, nominal tersimpan, dan status penyelesaian masing-masing.
2. Penerbitan tagihan tidak mencatat penerimaan uang dan tidak berarti tagihan telah dibayar.
3. Pengurus tidak perlu memasukkan setiap bulan satu per satu secara manual; sistem harus mendukung penyediaan tagihan tahunan secara efisien.
4. Mekanisme teknis penerbitan (misalnya batch, tombol, atau jadwal) **belum ditentukan** dan harus dipilih setelah memeriksa implementasi yang ada.
5. Tagihan yang telah diterbitkan mempertahankan nominal tersimpannya. Perubahan tarif tidak boleh diam-diam mengubah tagihan yang sudah terbit.
6. Unit yang baru memenuhi syarat di tengah tahun tidak otomatis menerima 12 bulan penuh. Periode awal tagihan dan kemungkinan prorata masih memerlukan keputusan stakeholder.
7. Tagihan bulan yang belum diterbitkan tidak boleh dimasukkan ke jumlah pembayaran atau dibuatkan alokasi seolah-olah sudah ada. UI boleh menampilkannya sebagai `Belum diterbitkan`, tetapi tidak boleh menawarkannya sebagai tagihan yang bisa dibayar.

### 2.1 Unit tagihan: kepenghunian, rumah, dan warga

Rujukan teknis dalam dokumen yang tersedia:
- `backend/migrations/009_finance.up.sql` menyebut `household_occupancy_id` pada struktur tagihan dan constraint unik `(household_occupancy_id, due_id, period)`.
- `backend/internal/finance/model.go` disebut sebagai model Go terkait `Bill`.
- `residency_periods` disebut sebagai hubungan `resident_id` dengan `household_occupancy_id`.

Rujukan tersebut perlu diperiksa lagi langsung di repository sebelum implementasi. Nama field dan struktur teknis saja **tidak menentukan keputusan bisnis** apakah kewajiban melekat pada rumah fisik, unit kepenghunian, keluarga, atau warga tertentu.

Sampai stakeholder memutuskan sebaliknya, spesifikasi menggunakan istilah **unit tagihan** dan menyebut `household_occupancy_id` hanya sebagai rujukan teknis yang perlu diverifikasi. Dampak perpindahan penghuni, satu rumah dengan beberapa keluarga, dan beberapa akun warga belum diputuskan.

---

## 3. Pembayaran Satu atau Beberapa Bulan

Pada alur Reguler, warga dapat mengajukan pembayaran untuk satu atau beberapa tagihan yang memenuhi syarat, dengan batas **maksimal 12 tagihan per satu transaksi pembayaran**.

- Hanya tagihan yang sudah diterbitkan, valid, belum lunas, dan memenuhi urutan FIFO yang dapat dialokasikan.
- Nilai pembayaran adalah penjumlahan nominal tersimpan pada tagihan yang dialokasikan, bukan tarif saat ini dikalikan jumlah bulan.
- Setiap tagihan harus dilunasi penuh. Jangan menambahkan pembayaran parsial atau saldo kredit otomatis untuk Reguler.
- Status teknis `partial`, jika ada di model atau skema, tidak dengan sendirinya mengizinkan pembayaran parsial Reguler.
- Batas 12 tagihan tidak boleh digunakan untuk melewati tagihan tertua. Perilaku saat tunggakan valid melebihi 12 tagihan—misalnya pembayaran melalui beberapa transaksi berurutan—masih perlu diputuskan stakeholder.
- Tagihan yang belum diterbitkan tidak termasuk dalam total dan tidak dapat dialokasikan.

---

## 4. FIFO dan Pemilihan Periode Akhir

**FIFO (First In, First Out) adalah aturan bisnis wajib:** tagihan valid tertua yang masih belum lunas harus diselesaikan sebelum tagihan dengan periode lebih baru.

Warga memilih **periode akhir** yang ingin dicapai, bukan memilih tagihan baru secara terpisah sambil melompati tunggakan lama. Sistem menentukan rangkaian tagihan mulai dari kewajiban tertua yang belum lunas hingga periode akhir pilihan, dengan syarat seluruh tagihan dalam rangkaian tersebut sudah diterbitkan dan memenuhi syarat.

### 4.1 Contoh perilaku

- **Januari–September belum lunas, warga memilih Oktober:** jika Januari–Oktober sudah diterbitkan dan valid, pembayaran harus mencakup urutan dari Januari sampai Oktober; Oktober saja tidak boleh dipilih.
- **Januari–September lunas, warga memilih Oktober:** pembayaran dapat mencakup Oktober.
- **Januari–September lunas, warga memilih Desember:** pembayaran dapat mencakup Oktober–Desember jika semua tagihan tersebut sudah diterbitkan dan valid.
- **November sudah diterbitkan, warga memilih Oktober:** November boleh terlihat sebagai informasi, tetapi tidak otomatis masuk ke total pembayaran.
- **Tagihan valid tertua sedang terkait pengajuan pending:** sistem tidak boleh diam-diam membolehkan pengajuan lain melewati kewajiban tersebut. Kebijakan konflik dan mekanisme teknisnya perlu ditetapkan.
- **Tagihan periode lama belum diterbitkan, periode baru sudah terbit:** sistem tidak boleh menganggap ketiadaan record lama otomatis membebaskan FIFO. Kebijakan `issuance gap` masih menunggu keputusan stakeholder.
- **Tunggakan lintas tahun:** tagihan Desember tahun sebelumnya lebih tua daripada tagihan Januari tahun berjalan.
- **Tagihan telah dibatalkan secara sah:** hanya jika kewajiban benar-benar dinyatakan tidak berlaku melalui proses berotorisasi dan tercatat, tagihan tersebut dapat dikeluarkan dari rangkaian kewajiban. Label teknis `cancelled`, `disabled`, atau `invalid` saja tidak cukup.
- **Status tagihan tidak jelas atau disengketakan:** jangan menganggap lunas dan jangan melewatinya secara diam-diam. Tahan proses yang terdampak sampai statusnya diselesaikan.

### 4.2 Persyaratan UI dan backend

1. UI harus menjelaskan bahwa tunggakan yang lebih tua wajib diselesaikan lebih dahulu; jangan hanya menonaktifkan tombol tanpa penjelasan.
2. Backend wajib memvalidasi FIFO. Validasi UI saja tidak cukup.
3. Validasi harus berlaku pada Web dan Android.
4. Daftar tagihan yang bisa dilihat harus dibedakan dari daftar tagihan yang benar-benar dialokasikan ke pembayaran.
5. Permintaan langsung ke API tidak boleh dapat melewati tagihan yang lebih tua.

### 4.3 Invarian pembayaran pending, konkurensi, dan idempotensi

Hasil bisnis berikut wajib dijamin:

- Pembayaran berstatus pending tidak melunasi tagihan dan tidak mem-posting penerimaan KAS final.
- Tagihan tidak boleh dilunasi atau dialokasikan secara final ke lebih dari satu pembayaran.
- Dua request bersamaan tidak boleh menyebabkan pelanggaran FIFO, alokasi ganda, nominal yang tidak konsisten, atau transaksi KAS ganda.
- Persetujuan pembayaran, penyelesaian alokasi, perubahan status tagihan, dan posting KAS harus konsisten secara atomik.
- Penolakan atau pembatalan pembayaran pending harus memungkinkan tagihan yang masih valid dan belum lunas diproses kembali sesuai FIFO; tagihan tidak boleh terkunci permanen.
- Request yang diulang akibat retry/timeout tidak boleh menggandakan pembayaran, alokasi, atau transaksi KAS.
- Konflik antar-pengajuan tidak boleh diselesaikan dengan melewati kewajiban yang lebih lama.

Istilah seperti **“pengajuan pending mengklaim tagihan”** hanya menggambarkan kondisi bisnis bahwa pengajuan pending mencantumkan tagihan tertentu. Istilah itu **bukan bukti** bahwa sistem saat ini memiliki reservasi, locking, atau status khusus di database.

Pilihan teknis—misalnya locking database, constraint unik, idempotency key, reservasi, atau penolakan cepat (*fast-fail*)—belum diputuskan. Evaluasi dan pilih pendekatan yang sesuai saat analisis implementasi; daftar contoh ini bukan kewajiban untuk menerapkan semuanya.

### 4.4 Status tagihan dan FIFO

- **Valid dan belum lunas:** tetap wajib dibayar menurut FIFO.
- **Lunas:** tidak boleh dialokasikan lagi sebagai kewajiban baru.
- **Dibatalkan/tidak dapat ditagih secara sah:** dikecualikan hanya jika proses resmi, kewenangan, alasan, dan audit trail telah ditetapkan.
- **Disengketakan atau tidak konsisten:** jangan otomatis dianggap lunas atau diabaikan; hentikan/hold alokasi yang berkonflik sampai ada keputusan yang sah.
- **Periode lama belum diterbitkan:** perlakuan FIFO masih terbuka; tidak boleh diasumsikan otomatis dapat dilewati.

---

## 5. Pencatatan Penerimaan KAS

Bedakan lima konsep berikut:

1. **Periode kewajiban:** bulan yang ditagihkan (`bills.period`).
2. **Tanggal pengajuan:** waktu warga mengirim pengajuan pembayaran (`payments.created_at` / `payments.paid_at`).
3. **Tanggal uang diterima:** tanggal dana benar-benar diterima di rekening/kas RT (`payments.actual_received_date`), dikonfirmasi oleh Bendahara pada saat persetujuan.
4. **Tanggal verifikasi/persetujuan:** waktu petugas menyetujui pengajuan (`payments.verified_at`).
5. **Tanggal transaksi KAS:** tanggal yang dipakai pada buku kas (`transactions.occurred_at`), diatur sesuai `actual_received_date` pada zona waktu operasional `Asia/Jakarta` (WIB).

Aturan bisnis final (**O-12 DECIDED**): penerimaan KAS dicatat pada tanggal uang benar-benar diterima. Tanggal pengajuan atau tanggal verifikasi tidak boleh otomatis menggantikan tanggal penerimaan.

Contoh: warga membayar tagihan Januari–Desember pada 31 Oktober 2026 dan pengajuan disetujui pada 1 November 2026. Uang benar-benar masuk rekening pada 31 Oktober 2026. Tanggal transaksi KAS yang dicatat adalah **31 Oktober 2026** (masuk ke laporan KAS Oktober 2026), sedangkan tanggal verifikasi adalah 1 November 2026. Detail alokasi tetap menunjuk ke tagihan Januari–Desember. Satu pembayaran multi-tagihan yang disetujui menghasilkan **satu transaksi penerimaan KAS** untuk jumlah yang diterima, bukan satu transaksi KAS per bulan.

Ketentuan implementasi tanggal penerimaan aktual (O-12):
- Field eksplisit: `payments.actual_received_date` (tipe `DATE`, format API string `YYYY-MM-DD`).
- Pihak berwenang: Bendahara/petugas memverifikasi mutasi rekening/fisik kas dan mengonfirmasi tanggal penerimaan aktual sebelum pembayaran disetujui.
- Validasi ketat: Wajib diisi pada approval (`action=approve`), format `YYYY-MM-DD`, valid calendar date, dan dilarang tanggal masa depan (`actual_received_date <= todayJakarta`).
- Pencatatan KAS: Transaksi KAS (`occurred_at`) dipetakan ke tanggal kalender tersebut pada jam 12:00:00 WIB untuk mencegah pergeseran tanggal saat konversi zona waktu.
- Audit trail: `actual_received_date` tersimpan dalam pembayaran dan dipancarkan ke audit log. Nilai tanggal historis yang tidak memiliki `actual_received_date` tetap dipertahankan apa adanya (tanpa klaim palsu).
- Ketentuan tambahan:
  - Pending/belum disetujui bukan penerimaan KAS final.
  - Persetujuan, alokasi, status tagihan, dan posting KAS harus konsisten dan atomik dalam satu transaksi database.
  - Retry tidak boleh membuat transaksi KAS ganda.
  - Koreksi, pembatalan, dan refund harus berotorisasi serta memiliki audit trail dan hubungan ke transaksi asal.

---

## 6. Bug yang Dilaporkan dan Skenario Risiko

Status berikut adalah **laporan yang perlu diverifikasi**, bukan pernyataan bahwa bug masih pasti ada atau sudah diperbaiki. Status hanya boleh diubah setelah kode, data, dan tes yang relevan diperiksa.

### 6.1 Bug A — Tagihan bulan berikutnya tidak muncul

**Laporan:** saat memilih pembayaran multi-bulan pada Oktober, UI hanya menampilkan Oktober, meskipun November telah diterbitkan.

**Perilaku yang diharapkan:**
- Tagihan berikutnya yang sudah diterbitkan boleh terlihat sebagai informasi.
- Tagihan setelah periode akhir yang dipilih tidak otomatis masuk ke total.
- Untuk membayar sampai November, warga memilih periode akhir November dan sistem memasukkan semua kewajiban valid dari tunggakan tertua hingga November sesuai FIFO.
- Investigasi perlu memeriksa query backend, filter periode/status, pengurutan, pemetaan data, dan UI. Jangan menyimpulkan akar masalah sebelum bukti diperiksa.

### 6.2 Risiko pembayaran pending, konkurensi, dan retry

| Skenario | Hasil yang wajib dijamin |
|---|---|
| Dua request membayar Januari secara bersamaan | Tidak ada dua pelunasan final atau dua transaksi KAS. |
| Pengajuan A pending Januari–Maret; pengajuan B mencakup Februari–April | Konflik ditangani secara konsisten; tidak ada alokasi ganda atau pelanggaran FIFO. |
| Pengajuan pending ditolak | Tagihan valid yang belum lunas dapat diproses kembali menurut FIFO. |
| Request yang sama diulang | Tidak ada pembayaran, alokasi, atau KAS ganda. |
| Pengajuan lain mencoba membayar April saat Januari–Maret masih pending | Sistem tidak boleh melewati kewajiban Januari yang lebih tua. |

### 6.3 Bug B — Pembayaran melompati tunggakan lama

**Laporan:** alur saat ini diduga memungkinkan pembayaran periode lebih baru walaupun ada tunggakan periode lama.

**Perilaku yang diharapkan:** backend menolak atau mengarahkan pengajuan yang mencoba melewati tagihan valid tertua. Tes harus mencakup tunggakan lintas tahun, status lunas, pembatalan yang sah, status tidak jelas, `issuance gap`, dan pembayaran pending yang tumpang tindih.

---

## 7. Konsistensi Web, Android, dan Backend

Aturan bisnis harus sama di semua platform:
- Satu pilihan `Reguler` untuk satu maupun beberapa bulan.
- Sukarela tidak berubah.
- Maksimal 12 tagihan per transaksi.
- FIFO divalidasi di backend.
- Nominal berasal dari tagihan tersimpan.
- Tidak ada pembayaran parsial per tagihan Reguler.
- Pending tidak melunasi dan tidak mem-posting KAS final.
- UI membedakan tagihan terlihat dengan tagihan yang dialokasikan.
- Pesan kesalahan konflik harus jelas dan tidak mendorong pengguna melewati FIFO.

Implementasi tampilan dapat berbeda sesuai platform, tetapi hasil bisnis dan validasinya tidak boleh berbeda.

---

## 8. Contoh Alokasi Pembayaran

Contoh: tagihan Januari–Desember sudah diterbitkan; Januari–Februari lunas; Maret–Juni belum lunas; Juli–Desember belum lunas.

- Warga memilih periode akhir Mei: alokasi yang diharapkan adalah Maret, April, dan Mei, masing-masing dengan nominal tersimpan dan pelunasan penuh.
- Warga memilih periode akhir Desember: alokasi mencakup Maret–Desember, yaitu 10 tagihan, jika semuanya valid dan belum lunas serta tidak ada konflik pembayaran.
- Warga tidak boleh memilih hanya Desember sambil melewati Maret–November yang masih menjadi kewajiban.
- Jika jumlah tagihan wajib yang harus dialokasikan melampaui batas 12, sistem tidak boleh melewati yang tertua. Cara melanjutkan pembayaran harus diputuskan stakeholder.

Satu pembayaran dapat memiliki beberapa alokasi tagihan. Status pengajuan pembayaran, status setiap tagihan, dan transaksi KAS adalah entitas konseptual berbeda yang harus dapat ditelusuri dan dijaga konsistensinya.

---

## 9. Perubahan Tarif

### 9.1 Prinsip final

1. Tagihan yang sudah diterbitkan mempertahankan nominal tersimpannya.
2. Perubahan tarif tidak boleh mengubah tagihan terbit secara diam-diam.
3. Pembayaran memakai nominal pada tagihan yang dialokasikan.
4. Tarif baru tidak boleh digunakan untuk menghitung ulang kewajiban lama tanpa proses koreksi resmi yang disepakati.

### 9.2 Keputusan terbuka

Stakeholder perlu menetapkan:
- Bagaimana tanggal efektif keputusan tarif dipetakan ke periode tagihan.
- Tarif mana yang digunakan untuk periode yang belum diterbitkan ketika tarif berubah.
- Apakah dan bagaimana tagihan yang sudah terbit dapat dikoreksi, oleh siapa, dengan alasan dan audit trail seperti apa.

Jangan menetapkan pemetaan tanggal efektif atau prosedur koreksi secara sepihak.

---

## 10. Warga Baru atau Pindah

### 10.1 Prinsip umum

- Unit yang baru memenuhi syarat di tengah tahun tidak otomatis dikenai 12 bulan penuh.
- Unit tagihan harus konsisten dengan keputusan bisnis final tentang siapa yang bertanggung jawab atas kewajiban.
- Riwayat tagihan dan pembayaran harus tetap dapat ditelusuri ketika penghuni berubah.

### 10.2 Keputusan terbuka

Stakeholder harus memutuskan:
- Periode pertama yang ditagihkan kepada warga/unit baru.
- Apakah prorata berlaku.
- Bagaimana tanggung jawab atas tunggakan dan tagihan berjalan diperlakukan saat penghuni pindah.
- Bagaimana menangani satu rumah dengan beberapa keluarga atau beberapa akun warga.

---

## 11. Penonaktifan Iuran, Pembatalan Kewajiban, dan Refund

### 11.1 Penonaktifan iuran

Menonaktifkan jenis iuran atau konfigurasi penerbitan tidak otomatis membatalkan tagihan yang sudah terbit. Efek penonaktifan terhadap tagihan yang belum lunas perlu mengikuti kebijakan yang disepakati.

### 11.2 Pembatalan kewajiban

Pembatalan tagihan harus memiliki otorisasi, alasan, dan audit trail. Status teknis `cancelled`, `disabled`, atau `invalid` saja tidak membuktikan kewajiban sudah tidak berlaku. Hanya kewajiban yang secara sah dinyatakan tidak dapat ditagih yang boleh dikeluarkan dari FIFO.

### 11.3 Refund dan koreksi

Pembayaran yang sudah disetujui tidak boleh dihapus atau dibatalkan melalui proses biasa tanpa jejak. Prosedur refund/koreksi perlu menetapkan otorisasi, alasan, relasi ke transaksi asal, dampak terhadap tagihan, dan pencatatan KAS. Rincian prosedur masih terbuka dan harus diputuskan sebelum implementasi.

---

## 12. Notifikasi

### 12.1 Prinsip

- Notifikasi penerbitan tagihan **diwajibkan** dan berbeda dari pengingat tagihan yang belum lunas.
- Pengingat bulanan untuk tagihan yang belum lunas **diwajibkan** dan hanya dikirim untuk tagihan yang sudah diterbitkan.
- Notifikasi tidak boleh menyatakan pembayaran lunas sebelum persetujuan sah.
- Informasi yang ditampilkan harus sesuai status tagihan dan pengajuan pembayaran yang sebenarnya.
- Notifikasi tidak boleh menjadi mekanisme yang mengubah status atau melunasi tagihan.

### 12.2 Penerbitan vs pengingat

Penerbitan memberi tahu bahwa tagihan telah dibuat. Pengingat memberi tahu bahwa kewajiban masih belum lunas. Keduanya tidak boleh dianggap sebagai transaksi uang.

### 12.3 Verifikasi terbuka

Periksa pemicu notifikasi, scheduler, zona waktu, deduplikasi, dan perilaku retry yang sudah ada. Jangan mengklaim ada scheduler atau deduplikasi tertentu sebelum diverifikasi. Kebijakan frekuensi dan penerima notifikasi yang belum didefinisikan perlu dikonfirmasi.

---

## 13. Riwayat dan Pelaporan KAS

### 13.1 Tanggal yang berbeda

Periode kewajiban, tanggal pengajuan, tanggal uang diterima, tanggal persetujuan, dan tanggal transaksi KAS merupakan konsep berbeda. Laporan kewajiban menunjukkan periode tagihan; buku KAS menunjukkan penerimaan berdasarkan tanggal uang benar-benar diterima.

### 13.2 Prinsip pelaporan

- Satu pembayaran multi-tagihan yang disetujui menghasilkan satu penerimaan KAS sebesar uang yang diterima.
- Detail alokasi tetap menampilkan tagihan bulanan yang dilunasi.
- Pending tidak ditampilkan sebagai penerimaan KAS final.
- Koreksi/refund harus bisa ditelusuri ke transaksi asal.
- Definisi field tanggal yang dipakai untuk laporan harus diverifikasi dari kode dan database.

### 13.3 Contoh

Tagihan Januari–Desember dibayar sekaligus dan uang diterima pada Oktober. Laporan KAS mencatat satu penerimaan pada tanggal penerimaan di Oktober; detail pembayaran menunjukkan alokasi ke 12 tagihan Januari–Desember. Ini tidak berarti KAS harus membuat satu transaksi per bulan tagihan.

---

## 14. Daftar Keputusan Terbuka

Butir berikut **belum boleh dianggap selesai** hanya karena telah dicantumkan dalam dokumen. Tutup tiap butir dengan keputusan stakeholder atau bukti teknis yang sesuai.

| ID | Keputusan / verifikasi yang dibutuhkan | Jenis |
|---|---|---|
| O-01 | Mekanisme teknis penerbitan 12 tagihan tanpa input manual satu per satu | Teknis |
| O-02 | Periode awal tagihan bagi warga/unit yang baru memenuhi syarat di tengah tahun | Bisnis |
| O-03 | Apakah prorata berlaku, termasuk aturan perhitungannya | Bisnis |
| O-04 | Pemetaan tanggal efektif perubahan tarif ke periode tagihan | Bisnis |
| O-05 | Tarif yang digunakan untuk tagihan yang belum diterbitkan saat tarif berubah | Bisnis |
| O-06 | Prosedur koreksi tagihan terbit, otorisasi, dan audit trail | Bisnis/teknis |
| O-07 | Prosedur refund dan dampaknya pada tagihan serta KAS | Bisnis/teknis |
| O-08 | Jadwal, pemicu, penerima, dan deduplikasi notifikasi/pengingat | Bisnis/teknis |
| O-09 | Status dan proses sah untuk tagihan dibatalkan/tidak dapat ditagih | Bisnis |
| O-10 | Kompatibilitas status teknis `partial` dengan fitur Iuran existing, tanpa mengizinkan parsial pada Reguler | Teknis |
| O-11 | Struktur aktual relasi pembayaran, alokasi tagihan, dan posting KAS | Teknis |
| O-12 | Semantik field tanggal untuk penerimaan uang dan posting KAS | Teknis/Bisnis | **DECIDED:** Tanggal transaksi KAS wajib mengikuti tanggal uang benar-benar diterima. Field `actual_received_date` (`DATE` / string `YYYY-MM-DD`) dikonfirmasi oleh Bendahara pada saat persetujuan pembayaran (atau diinput petugas saat pencatatan langsung). Tanggal transaksi KAS (`occurred_at`) dipetakan ke tanggal penerimaan aktual dalam zona waktu Asia/Jakarta (WIB). Wajib diisi saat approval, dilarang tanggal masa depan, dan divalidasi ketat di backend. |
| O-13 | Pendekatan teknis untuk concurrency, idempotensi, dan konflik pengajuan pending | Teknis |
| O-14 | Penanganan status tagihan tidak konsisten atau sedang disengketakan | Bisnis/teknis |
| O-15 | Unit kewajiban: unit kepenghunian vs rumah vs warga; termasuk mutasi dan multi-warga | Bisnis |
| O-16 | Perilaku jika tunggakan valid melebihi 12 tagihan dalam satu transaksi | Bisnis |
| O-17 | Kebijakan FIFO jika tagihan periode lama belum diterbitkan (`issuance gap`) | Bisnis |
| O-18 | Status terkini dua bug laporan pada Section 6, berdasarkan bukti kode dan tes | Verifikasi |
| O-19 | Otoritas dan audit trail untuk pembatalan tagihan, koreksi, serta refund | Bisnis/operasional |

---

## 15. Acceptance Criteria

Kriteria berikut mendefinisikan hasil yang harus diuji. **Belum ada kriteria yang boleh ditandai lulus tanpa hasil tes aktual.**

1. Form pembayaran menampilkan satu pilihan `Reguler`; pilihan lama `Reguler Bulanan` dan `Reguler Multi-Bulan` tidak lagi menjadi dua alur terpisah.
2. Alur Sukarela tidak berubah dari perilaku sebelum implementasi.
3. Sistem mendukung 12 tagihan bulanan terpisah untuk unit yang memenuhi syarat.
4. Penerbitan tagihan tidak membuat penerimaan KAS.
5. Pembayaran Reguler dapat mencakup satu atau beberapa tagihan, maksimum 12 per transaksi.
6. Setiap tagihan Reguler yang dialokasikan dibayar penuh; tidak ada pembayaran parsial per tagihan atau kredit otomatis.
7. Nominal pembayaran bersumber dari nominal tersimpan pada tiap tagihan.
8. FIFO berlaku lintas tahun dan divalidasi backend, termasuk pada request langsung ke API.
9. Pemilihan periode akhir tidak memungkinkan pengguna melompati tagihan valid tertua.
10. Tagihan yang belum diterbitkan tidak dapat dialokasikan atau dimasukkan ke total pembayaran.
11. Tagihan setelah periode akhir boleh terlihat jika sudah diterbitkan, tetapi tidak otomatis dimasukkan ke pembayaran.
12. Tagihan yang sah dibatalkan hanya dikecualikan dari FIFO setelah proses bisnis yang berotorisasi dan tercatat.
13. Tagihan dengan status tidak jelas/disengketakan tidak otomatis dianggap lunas atau dilewati.
14. Pending tidak melunasi tagihan dan tidak mem-posting KAS final.
15. Konflik request bersamaan tidak menimbulkan alokasi ganda, pelanggaran FIFO, atau KAS ganda.
16. Retry request yang sama tidak menggandakan pembayaran, alokasi, atau transaksi KAS.
17. Penolakan/pembatalan pengajuan pending tidak mengunci tagihan secara permanen.
18. Satu pembayaran multi-tagihan yang disetujui menghasilkan satu penerimaan KAS, sedangkan detail alokasi mempertahankan periode masing-masing.
19. KAS memakai tanggal uang benar-benar diterima; pemetaan field tanggal diverifikasi dari implementasi.
20. Web dan Android menerapkan hasil bisnis yang sama.
21. Bug tagihan bulan berikutnya dan bug lompatan tunggakan memiliki hasil verifikasi yang tercatat, bukan sekadar asumsi.
22. Seluruh keputusan terbuka yang memengaruhi alur implementasi diselesaikan sebelum bagian terkait dibangun.

---

## 16. Verifikasi Rujukan Teknis

Rujukan teknis yang disebut dalam draf sumber dan harus diperiksa langsung sebelum implementasi:

- `backend/migrations/009_finance.up.sql`
- `backend/internal/finance/model.go`
- relasi `residency_periods` antara `resident_id` dan `household_occupancy_id`

Daftar tersebut merupakan **petunjuk pemeriksaan**, bukan hasil audit repository pada revisi dokumen ini.

Saat verifikasi teknis:
1. Periksa kode aktual, migration terkait, constraint, handler/service/repository, alur persetujuan pembayaran, dan posting KAS.
2. Periksa tes yang relevan dan jalankan tes; jangan menyimpulkan tes lulus dari nama atau keberadaan file.
3. Periksa status migration pada database aktif secara terpisah dari keberadaan berkas migration.
4. Catat hasil sebagai `Terverifikasi`, `Gagal`, atau `Belum diverifikasi`, sertakan perintah/bukti yang dapat diulang.
5. Jangan mencatat nama field sebagai tanggal penerimaan uang tanpa menelusuri bagaimana nilainya diisi dan digunakan.

Tidak ada hasil verifikasi kode, tes, atau database aktif yang diklaim oleh dokumen ini.

---

## 17. Ringkasan Keputusan Final vs Terbuka

### 17.1 Final

- Hanya ada dua jenis yang dibahas: Sukarela dan Reguler.
- Form Reguler menggunakan satu pilihan `Reguler` untuk pembayaran satu atau beberapa bulan.
- Alur Sukarela tetap persis seperti implementasi saat ini dan tidak disentuh.
- Setiap bulan adalah tagihan tersendiri; target kebutuhan adalah 12 tagihan bulanan per tahun untuk unit yang memenuhi syarat.
- Maksimal 12 tagihan per transaksi.
- Pembayaran per tagihan Reguler harus penuh.
- FIFO wajib, termasuk lintas tahun, dan harus divalidasi backend.
- Tagihan belum terbit tidak dapat dibayar.
- Nominal tagihan terbit tersimpan dan tidak berubah diam-diam karena tarif baru.
- Pending bukan pelunasan dan bukan penerimaan KAS final.
- Satu pembayaran multi-tagihan yang disetujui menghasilkan satu penerimaan KAS, dengan detail alokasi terpisah.
- **O-12 DECIDED:** Transaksi KAS wajib memakai tanggal uang benar-benar diterima (`actual_received_date`), dikonfirmasi oleh Bendahara pada saat approval atau saat pencatatan staf, dan dipetakan ke zona waktu Asia/Jakarta (WIB). Tanggal pengajuan/verifikasi tidak menggantikan tanggal penerimaan.
- Konkurensi dan idempotensi adalah hasil bisnis yang wajib dijamin; metode teknisnya belum ditentukan.

### 17.2 Terbuka

Seluruh butir terbuka lainnya pada Section 14 (kecuali O-12 yang telah diputuskan) masih perlu keputusan atau bukti. Khususnya: mekanisme penerbitan, unit kewajiban, periode warga baru/prorata, perubahan tarif, pembatalan/refund, notifikasi, penanganan konflik pending, batas 12 tagihan, `issuance gap`, dan status dua bug laporan.

---

## 18. Checklist Sebelum Implementasi

- [ ] Keputusan terbuka yang relevan sudah disepakati dan dicatat.
- [ ] Unit tagihan dan dampak mutasi penghuni sudah diputuskan.
- [ ] Cara menerbitkan 12 tagihan sudah ditentukan setelah analisis implementasi.
- [ ] Periode awal dan aturan prorata warga baru sudah ditentukan.
- [ ] Pemetaan tarif efektif dan koreksi tagihan sudah ditentukan.
- [ ] Kebijakan tunggakan lebih dari 12 tagihan sudah ditentukan tanpa melanggar FIFO.
- [ ] Kebijakan `issuance gap` sudah ditentukan.
- [ ] Backend memvalidasi FIFO lintas tahun dan tidak dapat dilewati melalui API.
- [ ] Status lunas, batal sah, disengketakan, dan tidak konsisten dibedakan.
- [ ] Pending tidak melunasi tagihan atau mem-posting KAS final.
- [ ] Concurrency safety dan idempotensi diuji dengan request bersamaan dan retry.
- [ ] Penolakan/pembatalan pending melepaskan konflik sesuai kebijakan dan tidak mengunci tagihan permanen.
- [ ] Alokasi, status tagihan, persetujuan, dan posting KAS konsisten.
- [ ] Satu pembayaran multi-tagihan yang disetujui hanya menghasilkan satu transaksi penerimaan KAS.
- [x] Field tanggal penerimaan uang (`actual_received_date`) dan tanggal posting KAS (`occurred_at`) sudah diverifikasi dan diimplementasikan dari alur kode aktual (O-12 DECIDED).
- [ ] Web dan Android memiliki hasil bisnis yang konsisten.
- [ ] Sukarela terbukti tidak berubah melalui review diff dan tes regresi.
- [ ] Kedua bug laporan diuji dan statusnya didukung bukti.
- [ ] Tes benar-benar dijalankan dan hasil aktual dicatat.
- [ ] Status migration pada database aktif diverifikasi terpisah dari berkas migration.
- [ ] Tidak ada klaim bahwa kode, tes, atau database telah berubah/lulus tanpa bukti.

---

## 19. Batasan dan Penutupan

Dokumen ini adalah spesifikasi desain, bukan bukti implementasi selesai. Isinya menetapkan prinsip bisnis yang sudah final, merekam risiko, menetapkan acceptance criteria, dan mempertahankan daftar keputusan terbuka.

**Cakupan perubahan dokumen ini saja:**

| Area | Status dalam revisi ini |
|---|---|
| Backend source code | Tidak diubah |
| Database/schema | Tidak diubah |
| Migration | Tidak diubah |
| Repository/service/handler | Tidak diubah |
| API implementation | Tidak diubah |
| Web source code | Tidak diubah |
| Android source code | Tidak diubah |
| Test code | Tidak diubah |

Sebelum implementasi, selesaikan keputusan terbuka yang memengaruhi perilaku bisnis dan lakukan audit teknis terhadap repository serta database aktif. Jangan mengubah perilaku Sukarela.

---

*End of Document — Iuran Reguler Multi-Bulan Design Draft (Revisi 2026-10-27)*
