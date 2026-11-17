# Desain Jabatan Petugas Keamanan dan Kebersihan

**Status:** Final Design / Siap Implementasi  
**Platform utama:** Android  
**Modul terkait:** Warga, Notifikasi, Iuran, DevSeed, RBAC  
**Event notifikasi:** `waste_truck_arrival`

---

## 1. Latar Belakang

Aplikasi Social Finance Urena memiliki kebutuhan untuk mendukung tugas operasional Petugas Keamanan dan Petugas Kebersihan di lingkungan RT.

Kedua jabatan mempunyai tanggung jawab berbeda:

1. **Keamanan:** memberikan informasi kepada seluruh warga RT ketika truk sampah tiba dengan mengirimkan notifikasi yang dilengkapi foto aktual truk sampah.
2. **Kebersihan:** memantau daftar rumah yang memiliki tunggakan iuran wajib minimal 3 bulan dan masih belum lunas, memberikan catatan lapangan, serta mencetak daftar tersebut untuk digunakan ketika bertugas.

Desain ini mengatur menu, hak akses, alur operasional, dan aturan bisnis untuk kedua jabatan tersebut.

## 2. Prinsip Umum

1. Keamanan dan Kebersihan merupakan jabatan khusus, bukan role aplikasi baru.
2. Keduanya tetap menggunakan mekanisme autentikasi dan RBAC Social Finance Urena.
3. Hak akses dibatasi pada RT yang menjadi kewenangan akun.
4. Keduanya tidak memperoleh hak administrasi atau pengelolaan keuangan.
5. Informasi warga yang ditampilkan dibatasi sesuai kebutuhan pekerjaan.
6. Tidak boleh ada perubahan hak akses jabatan lain akibat implementasi fitur ini.
7. Implementasi harus kompatibel dengan mekanisme multi-RT dan multi-RW yang sudah berlaku.

---

# BAGIAN A — PETUGAS KEAMANAN

## 3. Tujuan

Petugas Keamanan bertugas memberikan informasi kepada warga ketika truk sampah tiba di lingkungan RT.

Petugas menggunakan aplikasi Android untuk memilih template notifikasi yang telah disiapkan pengurus, mengambil foto truk sampah, dan mengirimkan notifikasi kepada seluruh warga dalam RT yang menjadi kewenangannya.

## 4. Menu Petugas Keamanan

Petugas Keamanan hanya memiliki dua menu di Android:

| No. | Menu | Hak Akses |
|---|---|---|
| 1 | Notifikasi | Melihat template dan mengirim notifikasi khusus |
| 2 | Warga | Read Only |

Tidak boleh ada menu lain yang ditampilkan kepada Petugas Keamanan.

### 4.1 Menu Notifikasi

Menu Notifikasi hanya menampilkan template dengan event:

`waste_truck_arrival`

Template ini digunakan untuk memberitahukan bahwa truk sampah sudah tiba.

Petugas Keamanan hanya boleh menggunakan template yang sudah ditentukan. Petugas tidak boleh mengelola atau mengubah konfigurasi template.

#### Informasi template yang bersifat Read Only

- Event / kode notifikasi.
- Judul.
- Isi pesan / wording.
- Variabel template.
- Konfigurasi template lainnya.
- Pengaturan penerima.
- Pengaturan jadwal atau konfigurasi sistem.

Semua informasi tersebut hanya dapat diubah oleh pihak yang memiliki kewenangan administrasi notifikasi.

#### Informasi yang boleh dilakukan oleh Petugas Keamanan

Petugas Keamanan hanya diperbolehkan:

1. Membuka template `waste_truck_arrival`.
2. Melihat preview notifikasi.
3. Mengambil foto truk sampah menggunakan kamera Android.
4. Melampirkan foto truk sampah.
5. Mengirim notifikasi menggunakan template tersebut.

Petugas tidak boleh mengubah judul, wording, isi pesan, variabel, atau konfigurasi template.

### 4.2 Alur Pengiriman Notifikasi

1. Truk sampah tiba di lingkungan RT.
2. Petugas Keamanan membuka aplikasi Android.
3. Petugas membuka menu Notifikasi.
4. Petugas memilih template `waste_truck_arrival`.
5. Sistem menampilkan preview template dalam kondisi Read Only.
6. Petugas mengambil foto truk sampah.
7. Foto dilampirkan sebagai gambar pada notifikasi.
8. Petugas menekan tombol Kirim.
9. Sistem mengirim notifikasi kepada seluruh warga dalam RT yang menjadi kewenangan Petugas Keamanan.

#### Ketentuan Penerima

Penerima notifikasi adalah seluruh warga pada RT yang sama dengan konteks kewenangan Petugas Keamanan.

Ketentuan:

- Tidak hanya warga yang berada dekat dengan lokasi truk.
- Tidak hanya warga yang sedang membuka aplikasi.
- Tidak dikirim ke warga dari RT lain.
- Tidak dikirim lintas RW.
- Mengikuti mekanisme distribusi notifikasi umum RT yang sudah tersedia.

Pengiriman harus tetap mengikuti mekanisme deduplikasi penerima, status akun, dan aturan notifikasi yang berlaku di aplikasi.

#### Ketentuan Foto

- Foto diambil oleh Petugas Keamanan ketika truk sampah benar-benar tiba.
- Foto menjadi lampiran notifikasi.
- Foto tidak mengubah isi template.
- Sistem harus menangani kegagalan upload foto dan kegagalan pengiriman notifikasi dengan aman.
- Jangan mengirim notifikasi sukses apabila proses pengiriman belum berhasil.

### 4.3 Menu Warga

Menu Warga pada akun Keamanan hanya untuk melihat informasi warga.

Petugas Keamanan boleh:

- Melihat daftar warga sesuai RT yang menjadi kewenangannya.
- Membuka detail warga sesuai batasan informasi yang diberikan kepada petugas.

Petugas Keamanan tidak boleh:

- Menambahkan warga.
- Mengubah data warga.
- Menghapus warga.
- Mengubah status warga.
- Mengelola rumah tangga atau keanggotaan RT.

---

# BAGIAN B — PETUGAS KEBERSIHAN

## 5. Tujuan

Petugas Kebersihan bertugas memantau rumah yang masih mempunyai tunggakan iuran wajib minimal 3 bulan dan belum melunasi kewajibannya.

Daftar tersebut digunakan sebagai acuan operasional ketika Petugas Kebersihan melakukan pekerjaan di lingkungan RT.

Petugas juga membutuhkan fasilitas untuk:

- Memberikan catatan lapangan.
- Melihat daftar rumah yang masih memenuhi kriteria.
- Mengekspor daftar ke Excel.
- Mengekspor daftar ke PDF.
- Mencetak daftar secara langsung.
- Mencetak ulang daftar terbaru kapan saja.

## 6. Konsep Monitoring

Untuk tahap awal, status sampah tidak diangkut belum mempunyai sumber data terpisah dan masih berkaitan dengan kondisi tunggakan iuran.

Oleh karena itu, **tidak perlu membuat dua kategori monitoring yang terpisah**.

Cukup satu daftar monitoring berdasarkan tunggakan iuran wajib minimal 3 bulan.

Daftar ini menjadi acuan Petugas Kebersihan.

### 6.1 Kriteria Warga Masuk Daftar

Sebuah rumah masuk dalam daftar monitoring apabila:

1. Memiliki tagihan iuran yang bersifat wajib.
2. Tagihan sudah melewati jatuh tempo minimal 3 bulan kalender.
3. Masih mempunyai sisa tagihan yang harus dibayar.
4. Tagihan belum dinyatakan lunas.

Gunakan sumber data dan aturan perhitungan yang sama dengan mekanisme peringatan tunggakan 3 bulan yang sudah ada di aplikasi.

Jangan membuat algoritma perhitungan tunggakan baru yang berpotensi menghasilkan angka berbeda.

### 6.2 Daftar Bersifat Dinamis

Daftar monitoring harus selalu mencerminkan kondisi pembayaran terbaru.

Contoh:

- Warga memiliki tunggakan 4 bulan: muncul di daftar.
- Warga membayar sebagian tetapi masih memenuhi kriteria: tetap muncul.
- Warga melunasi seluruh tunggakan yang memenuhi kriteria: otomatis keluar dari daftar aktif.
- Warga kembali memenuhi kriteria di kemudian hari: muncul kembali.

Tidak diperlukan penghapusan manual oleh Petugas Kebersihan ketika warga sudah melunasi tunggakan.

Daftar aktif harus dihitung berdasarkan kondisi tagihan terbaru, bukan berdasarkan hasil export sebelumnya.

### 6.3 Cakupan RT

Petugas Kebersihan hanya boleh melihat rumah dari RT yang menjadi kewenangannya.

Tidak boleh ada akses ke daftar tunggakan dari RT lain yang tidak diizinkan.

---

## 7. Menu dan Tampilan Monitoring Kebersihan

Menu utama Petugas Kebersihan adalah Warga.

Pada halaman ini disediakan tampilan khusus:

**Monitoring Kebersihan — Tunggakan 3+ Bulan**

Contoh tampilan:

| No. | RT | No. Rumah | Catatan |
|---:|:---:|:---:|---|
| 1 | 001 | 05 | Sampah belum diangkut |
| 2 | 001 | 12 | Perlu diperiksa |
| 3 | 002 | 08 | Sudah dikunjungi |

Data di atas hanya contoh.

### 7.1 Informasi yang Boleh Ditampilkan

Informasi yang boleh dilihat Petugas Kebersihan:

- Nomor RT.
- Nomor rumah.
- Catatan operasional.
- Informasi tanggal pencatatan catatan lapangan.

Informasi yang tidak boleh ditampilkan:

- Nama warga.
- Nomor telepon.
- Email.
- Nominal tagihan.
- Detail transaksi pembayaran.
- Informasi pribadi lain yang tidak diperlukan.

Kebutuhan pekerjaan dapat dipenuhi hanya dengan mengetahui lokasi rumah.

### 7.2 Catatan Lapangan

Petugas Kebersihan boleh menambahkan atau memperbarui catatan untuk rumah yang sedang masuk daftar monitoring.

Contoh catatan:

- Sampah belum diangkut.
- Sudah dikunjungi.
- Perlu pemeriksaan ulang.
- Kondisi rumah sedang kosong.
- Catatan operasional lainnya.

Ketentuan:

1. Catatan bersifat opsional.
2. Catatan tidak mengubah status tagihan.
3. Catatan tidak mengubah transaksi pembayaran.
4. Catatan tidak mengubah data warga.
5. Catatan tersimpan sebagai riwayat operasional.
6. Catatan tetap tersimpan meskipun rumah sudah keluar dari daftar aktif karena tunggakan telah dilunasi.

### 7.3 Riwayat Catatan

Riwayat catatan sebaiknya menyimpan:

- RT.
- Nomor rumah.
- Isi catatan.
- Petugas yang membuat atau mengubah catatan.
- Waktu pencatatan.
- Waktu perubahan terakhir.

Riwayat catatan tidak boleh otomatis dihapus ketika status tunggakan berubah.

Jika rumah kembali memenuhi kriteria tunggakan, catatan terdahulu tetap dapat ditelusuri.

---

## 8. Export Excel, PDF, dan Cetak

Fasilitas export dan cetak disediakan agar Petugas Kebersihan dapat membawa daftar rumah dalam bentuk fisik ketika bertugas tanpa harus terus membuka aplikasi.

### 8.1 Format yang Didukung

| Format | Kegunaan |
|---|---|
| Excel (.xlsx) | Pengolahan atau pencatatan tambahan |
| PDF | Dokumen laporan siap dibagikan atau dicetak |
| Print | Mencetak langsung melalui perangkat yang tersedia |

### 8.2 Filter dan Periode

Petugas dapat menentukan periode atau cakupan laporan sesuai kebutuhan.

Setiap kali export atau print dilakukan, sistem harus mengambil data monitoring terbaru.

### 8.3 Format Dokumen

Contoh:

**LAPORAN MONITORING KEBERSIHAN**

Periode: 1–30 November 2026

| No. | RT | No. Rumah | Catatan |
|---:|:---:|:---:|---|
| 1 | 001 | 05 | Sampah belum diangkut |
| 2 | 001 | 12 | Perlu diperiksa |
| 3 | 002 | 08 | Sudah dikunjungi |

Ketentuan dokumen:

- Tidak menampilkan nama warga.
- Tidak menampilkan nomor telepon.
- Tidak menampilkan nominal tunggakan.
- Tidak menampilkan detail pembayaran.
- Hanya menampilkan nomor RT, nomor rumah, dan catatan operasional yang diperlukan.
- Format harus mudah dibaca ketika dicetak.

### 8.4 Ketentuan Cetak Ulang

Dokumen yang telah dicetak merupakan snapshot data pada waktu dokumen dibuat.

Apabila ada warga yang melunasi tunggakan setelah dokumen dicetak:

- Data rumah tersebut otomatis keluar dari daftar monitoring aktif.
- Dokumen lama tidak berubah.
- Ketika Petugas Kebersihan mencetak ulang, rumah yang sudah melunasi tidak lagi tercantum.

Dengan demikian, Petugas selalu dapat memperoleh daftar terbaru tanpa perlu menghapus data secara manual.

---

# BAGIAN C — HAK AKSES DAN KEAMANAN

## 9. Matriks Hak Akses

| Fitur | Keamanan | Kebersihan |
|---|---|---|
| Login Android | Ya | Ya |
| Menu Notifikasi | Ya | Tidak |
| Melihat template `waste_truck_arrival` | Ya | Tidak |
| Mengubah template | Tidak | Tidak |
| Mengirim `waste_truck_arrival` | Ya | Tidak |
| Upload foto truk | Ya | Tidak |
| Melihat daftar Warga | Read Only | Sesuai kebutuhan monitoring |
| Mengubah data Warga | Tidak | Tidak |
| Melihat monitoring tunggakan 3+ bulan | Tidak | Ya |
| Melihat nominal tunggakan | Tidak | Tidak |
| Menambah catatan lapangan | Tidak | Ya |
| Mengubah catatan lapangan | Tidak | Ya |
| Export Excel | Tidak | Ya |
| Export PDF | Tidak | Ya |
| Print | Tidak | Ya |
| Mengubah tagihan | Tidak | Tidak |
| Mencatat pembayaran | Tidak | Tidak |
| Mengubah transaksi | Tidak | Tidak |
| Mengelola template notifikasi | Tidak | Tidak |

Semua akses tetap mengikuti batas kewenangan RT.

## 10. Prinsip RBAC

Implementasi harus menggunakan permission khusus sesuai fungsi.

### Keamanan

Keamanan memerlukan:

- `warga.read`
- Permission khusus untuk mengirim event `waste_truck_arrival`.

Permission khusus pengiriman harus membatasi event yang dapat dikirim.

Jangan memberikan `notification.manage` atau hak administrasi template secara umum kepada Keamanan.

### Kebersihan

Kebersihan memerlukan:

- `warga.read`
- Permission khusus untuk membaca monitoring tunggakan 3+ bulan.
- Permission khusus untuk membuat dan memperbarui catatan lapangan.
- Permission export dan print sesuai kebutuhan.

Permission tersebut tidak boleh memberikan hak:

- Membaca seluruh transaksi keuangan.
- Mengubah tagihan.
- Mencatat pembayaran.
- Mengubah status pembayaran.
- Mengelola warga.

Jika dibutuhkan permission baru, definisikan secara eksplisit dan pastikan tidak memperluas hak akses jabatan lain.

## 11. Keamanan Data

1. Backend wajib memvalidasi permission setiap endpoint.
2. Pembatasan RT harus divalidasi di backend, bukan hanya menyembunyikan menu di frontend.
3. Jangan mempercayai `rt_id` yang dikirim dari client tanpa validasi membership dan scope.
4. Jangan mengembalikan nama warga atau nominal tunggakan ke response monitoring Kebersihan jika tidak diperlukan.
5. Endpoint export harus menerapkan pembatasan data yang sama dengan endpoint monitoring.
6. Upload gambar harus mengikuti mekanisme penyimpanan file yang sudah digunakan aplikasi.
7. Pengiriman notifikasi harus mengikuti mekanisme distribusi notifikasi yang sudah ada.
8. Jangan memberikan akses lintas RT atau lintas RW.
9. Super Admin dan jabatan lain tetap mengikuti aturan akses yang sudah berlaku.

---

# BAGIAN D — KEBUTUHAN TEKNIS

## 12. Backend

Kebutuhan backend yang perlu dievaluasi:

### Keamanan

- Endpoint atau mekanisme pengiriman khusus event `waste_truck_arrival`.
- Validasi jabatan Keamanan.
- Validasi RT scope.
- Validasi template Read Only.
- Upload dan penyimpanan foto.
- Distribusi kepada seluruh warga RT terkait.
- Pencatatan hasil pengiriman.

### Kebersihan

- Endpoint daftar rumah dengan tunggakan wajib 3+ bulan.
- Endpoint membuat catatan lapangan.
- Endpoint memperbarui catatan lapangan.
- Endpoint riwayat catatan lapangan.
- Endpoint export Excel.
- Endpoint export PDF jika proses dilakukan di backend; alternatifnya proses export di client dengan validasi data yang sesuai.

Daftar monitoring harus menggunakan sumber data tagihan yang sama dengan peringatan tunggakan 3 bulan yang sudah ada.

## 13. Android

### Keamanan

- Navigasi khusus dengan dua menu: Notifikasi dan Warga.
- Template notifikasi Read Only.
- Tombol ambil foto menggunakan kamera.
- Preview foto sebelum dikirim.
- Tombol kirim notifikasi.
- Feedback keberhasilan atau kegagalan pengiriman.

### Kebersihan

- Halaman monitoring tunggakan.
- Daftar RT dan nomor rumah.
- Form catatan lapangan.
- Riwayat catatan.
- Export Excel.
- Export PDF.
- Print langsung.
- Refresh daftar untuk memperoleh kondisi pembayaran terbaru.

## 14. Web

Implementasi awal berfokus pada Android sesuai kebutuhan operasional kedua petugas.

Tidak boleh secara otomatis memberikan akses menu baru kepada akun Web hanya karena jabatan Keamanan atau Kebersihan.

Jika suatu saat diperlukan dukungan Web, hal tersebut harus menjadi keputusan dan scope pengembangan tersendiri.

---

# BAGIAN E — KRITERIA PENERIMAAN

## 15. Acceptance Criteria Petugas Keamanan

- [ ] Akun Keamanan dapat login Android.
- [ ] Akun Keamanan hanya melihat menu Notifikasi dan Warga.
- [ ] Akun Keamanan tidak dapat mengubah data warga.
- [ ] Akun Keamanan hanya melihat template `waste_truck_arrival`.
- [ ] Seluruh konten template bersifat Read Only.
- [ ] Akun Keamanan dapat mengambil foto truk sampah.
- [ ] Akun Keamanan dapat melampirkan foto.
- [ ] Akun Keamanan dapat mengirim notifikasi.
- [ ] Notifikasi diterima seluruh warga pada RT yang menjadi kewenangannya.
- [ ] Tidak ada pengiriman lintas RT.
- [ ] Akun Keamanan tidak dapat mengelola template atau event lain.

## 16. Acceptance Criteria Petugas Kebersihan

- [ ] Akun Kebersihan dapat login Android.
- [ ] Akun Kebersihan dapat melihat monitoring tunggakan 3+ bulan.
- [ ] Daftar hanya berisi rumah yang masih memenuhi kriteria.
- [ ] Data yang ditampilkan hanya RT, nomor rumah, dan catatan operasional.
- [ ] Nama warga tidak ditampilkan.
- [ ] Nominal tunggakan tidak ditampilkan.
- [ ] Akun Kebersihan dapat menambahkan catatan.
- [ ] Akun Kebersihan dapat memperbarui catatan.
- [ ] Riwayat catatan tidak hilang setelah pelunasan.
- [ ] Rumah yang sudah melunasi otomatis keluar dari daftar aktif.
- [ ] Rumah yang kembali memenuhi kriteria dapat muncul kembali.
- [ ] Export Excel menghasilkan data terbaru.
- [ ] Export PDF menghasilkan data terbaru.
- [ ] Print mengambil data terbaru.
- [ ] Dokumen tidak menampilkan nama atau nominal tagihan.
- [ ] Akun Kebersihan tidak dapat mengubah transaksi atau tagihan.
- [ ] Akses monitoring dan export dibatasi berdasarkan RT.

## 17. Regression dan Compatibility

Implementasi tidak boleh merusak:

- Hak akses Keamanan yang sudah ada.
- Hak akses Operasional.
- Hak akses Kebersihan yang sudah ada.
- Hak akses Bendahara, Ketua, Wakil Ketua, dan Sekretaris.
- Hak akses Super Admin.
- Menu Notifikasi Ketua RT.
- Menu Notifikasi warga.
- Pengelolaan template notifikasi.
- Sistem pembayaran dan tagihan iuran.
- Sistem membership RT dan RW.
- Sistem pengiriman notifikasi yang sudah berjalan.

Jangan mengubah permission yang sudah ada secara luas hanya untuk mempermudah implementasi.

---

# BAGIAN F — BATASAN IMPLEMENTASI

## 18. Hal yang Tidak Termasuk Scope

Untuk tahap pertama, tidak termasuk:

1. Pencatatan status pengangkutan sampah sebagai modul terpisah.
2. Pembuatan sistem penjadwalan truk sampah.
3. Pelacakan GPS truk sampah.
4. Peta lokasi truk.
5. Pengelolaan template oleh Keamanan.
6. Pengelolaan pembayaran oleh Kebersihan.
7. Penampilan nominal tunggakan kepada Kebersihan.
8. Penampilan identitas pribadi warga dalam laporan cetak.
9. Pengiriman notifikasi `waste_truck_arrival` lintas RT.
10. Penambahan menu administrasi baru bagi kedua jabatan.

## 19. Urutan Implementasi yang Disarankan

1. Audit struktur RBAC dan navigasi Android untuk jabatan Keamanan dan Kebersihan.
2. Audit sistem template dan pengiriman notifikasi yang sudah tersedia.
3. Implementasi akses khusus Keamanan untuk event `waste_truck_arrival` beserta lampiran foto.
4. Implementasi navigasi Android Keamanan dengan dua menu.
5. Audit sumber data tunggakan 3+ bulan.
6. Implementasi endpoint monitoring khusus Kebersihan.
7. Implementasi catatan lapangan dan riwayatnya.
8. Implementasi export Excel, PDF, dan print.
9. Implementasi navigasi dan tampilan Android Kebersihan.
10. Regression check terhadap RBAC dan fungsi yang sudah ada.

Implementasi dilakukan secara bertahap dengan perubahan sekecil mungkin pada modul yang sudah stabil.

---

## 20. Keputusan Desain Final

| Area | Keputusan |
|---|---|
| Keamanan | Dua menu Android: Notifikasi dan Warga |
| Hak akses Warga Keamanan | Read Only |
| Template Keamanan | `waste_truck_arrival` |
| Hak akses template | Read Only |
| Konten yang dapat ditambahkan | Foto truk sampah |
| Penerima notifikasi | Seluruh warga RT terkait |
| Kebersihan | Monitoring rumah dengan tunggakan wajib 3+ bulan |
| Sumber monitoring | Data tagihan iuran yang sudah ada |
| Kriteria keluar daftar | Seluruh tunggakan yang memenuhi kriteria sudah lunas |
| Catatan lapangan | Bisa dibuat dan diperbarui manual |
| Riwayat catatan | Tetap tersimpan meskipun rumah keluar dari daftar aktif |
| Data yang ditampilkan | Nomor RT, nomor rumah, catatan |
| Data yang disembunyikan | Nama warga dan nominal tunggakan |
| Export | Excel, PDF, dan Print |
| Cetak ulang | Selalu mengambil kondisi data terbaru |
| Cakupan akses | RT sesuai kewenangan akun |

**Kesimpulan:** Keamanan bertanggung jawab menyampaikan informasi kedatangan truk sampah kepada warga RT. Kebersihan bertanggung jawab memantau rumah yang masih mempunyai tunggakan iuran 3+ bulan, membuat catatan lapangan, dan membawa daftar terbaru dalam bentuk cetak ketika bertugas.