# Dokumentasi RBAC Operasional RW — Social Finance Urena

**Status:** FINAL — Acuan Audit dan Implementasi  
**Cakupan:** Beranda, KAS, dan Lokasi Dana untuk lingkup RT dan RW  
**Platform:** Backend/API, Web, dan Android  
**Prinsip utama:** Mempertahankan fitur RT yang sudah berjalan dan menambahkan dukungan operasional RW tanpa merusak kompatibilitas.

---

## 1. Tujuan

Dokumen ini menetapkan aturan Role-Based Access Control (RBAC) untuk operasional keuangan RT dan RW pada Social Finance Urena.

Fitur yang sudah tersedia pada lingkup RT menjadi acuan fungsional bagi lingkup RW. Pekerjaan ini berfokus pada pengaturan hak akses, pemisahan menu, batas data, dan dukungan pengguna yang memiliki lebih dari satu jabatan.

Tujuan implementasi:

1. Mempertahankan seluruh menu, nama, alur, dan perilaku fitur RT yang sudah berjalan.
2. Menambahkan menu RW dengan nama yang membedakannya secara eksplisit dari menu RT.
3. Memungkinkan perangkat RT melihat Beranda RW dan KAS RW dalam batas akses baca.
4. Memungkinkan perangkat RW melihat Beranda RT dan KAS RT dalam batas akses baca.
5. Menjamin pengguna dengan jabatan ganda memperoleh gabungan menu sesuai seluruh jabatan yang dimilikinya.
6. Menjamin perangkat RT tidak dapat mengakses data RT lain.
7. Memberikan akses penuh kepada Super Admin untuk RT 002 dan RW 016 sesuai lingkup kewenangan yang ditetapkan saat ini.
8. Memastikan otorisasi diberlakukan pada backend/API, bukan hanya melalui tampilan menu.

## 2. Ruang Lingkup

### 2.1 Menu existing RT

Menu RT berikut wajib dipertahankan:

- `BERANDA`
- `KAS`
- `LOKASI DANA`

Ketentuan:

- Jangan mengubah nama `KAS` menjadi `KAS RT`.
- Jangan mengganti nama menu existing dengan nama baru.
- Jangan memindahkan atau merombak struktur menu RT hanya untuk mengakomodasi menu RW.
- Jangan mengubah alur transaksi, tampilan, permission, endpoint, atau perilaku RT yang sudah benar, kecuali perubahan minimum yang terbukti diperlukan untuk mendukung RBAC baru.
- Jangan menghapus atau mengganti implementasi existing RT dengan implementasi RW.

### 2.2 Menu tambahan RW

Tambahkan tiga menu RW sebagai menu terpisah:

- `BERANDA RW`
- `KAS RW`
- `LOKASI DANA RW`

Menu RW tidak boleh menggantikan, mengambil alih, atau mengubah identitas menu RT existing.

Beranda RW, KAS RW, dan Lokasi Dana RW memanfaatkan kemampuan fitur yang telah tersedia pada lingkup RT sebagai acuan, dengan penyesuaian lingkup data dan otorisasi RW.

### 2.3 Di luar ruang lingkup

Pekerjaan ini tidak mencakup:

- Perubahan fitur Iuran.
- Perombakan fitur keuangan RT yang tidak diperlukan untuk RBAC.
- Refactor besar pada navigasi atau arsitektur aplikasi.
- Perubahan skema database tanpa kebutuhan yang telah dibuktikan melalui audit.
- Perubahan aturan bisnis transaksi hanya untuk membuat menu RW tampil.

## 3. Prinsip Dasar RBAC

Implementasi wajib mengikuti prinsip berikut.

### 3.1 Menu bukan otorisasi

Menampilkan atau menyembunyikan menu hanya mengatur navigasi. Backend/API tetap menjadi sumber otorisasi yang menentukan apakah pengguna dapat membaca atau melakukan tindakan terhadap suatu resource.

### 3.2 Hak akses mengikuti jabatan dan lingkup

Hak akses ditentukan berdasarkan:

- Role atau jabatan pengguna.
- Lingkup RT atau RW yang menjadi kewenangannya.
- Permission untuk menu, data, dan tindakan.
- Batas wilayah yang secara eksplisit diizinkan.

Pengguna tidak boleh memperoleh hak akses hanya dengan mengubah URL, parameter ID, request API, atau identifier wilayah.

### 3.3 Hak akses jabatan ganda merupakan gabungan

Apabila seorang pengguna memiliki jabatan RT sekaligus RW, hak aksesnya merupakan gabungan hak yang diberikan oleh kedua jabatan tersebut.

Contoh: Bendahara RT yang merangkap Wakil Ketua RW dapat melihat menu RT dan RW sesuai matriks RBAC pada dokumen ini.

Penggabungan hak akses tidak menghilangkan batas lingkup data, pembatasan tindakan, atau aturan isolasi antar-RT.

### 3.4 Akses silang bersifat baca saja

Akses silang RT dan RW terhadap Beranda dan KAS diberikan untuk melihat informasi.

Akses silang tersebut tidak otomatis memberikan izin untuk:

- Membuat transaksi.
- Mengubah transaksi.
- Menghapus transaksi.
- Menyetujui atau menolak transaksi.
- Mengelola lokasi dana.
- Melakukan transfer antar-lokasi.
- Melakukan tindakan lain yang membutuhkan permission khusus.

Hak tindakan tetap mengikuti kewenangan pada lingkup masing-masing.

## 4. Matriks Akses Menu

Keterangan:

- **Penuh:** dapat mengakses dan melakukan tindakan sesuai permission role pada lingkupnya.
- **Baca:** hanya dapat melihat informasi pada lingkup yang diizinkan.
- **Ditolak:** tidak memperoleh akses.
- **Sesuai RT sendiri:** hanya untuk RT yang menjadi kewenangan pengguna.
- **Sesuai RW:** hanya untuk RW yang menjadi kewenangan pengguna.

| Role | Beranda RT | KAS RT | Lokasi Dana RT | Beranda RW | KAS RW | Lokasi Dana RW |
|---|---|---|---|---|---|---|
| Perangkat RT | Penuh sesuai role | Penuh sesuai role | Sesuai permission RT existing | Baca | Baca | Ditolak |
| Ketua RW | Baca | Baca | Ditolak | Penuh | Penuh | Penuh |
| Wakil Ketua RW | Baca | Baca | Ditolak | Penuh | Penuh | Penuh |
| Bendahara RW | Baca | Baca | Ditolak | Penuh | Penuh | Penuh |
| Super Admin | Penuh dalam lingkup yang diizinkan | Penuh dalam lingkup yang diizinkan | Penuh dalam lingkup yang diizinkan | Penuh dalam lingkup yang diizinkan | Penuh dalam lingkup yang diizinkan | Penuh dalam lingkup yang diizinkan |

**Catatan:**

1. Untuk perangkat RT, hak pada Beranda RT, KAS RT, dan Lokasi Dana RT tetap mengikuti permission existing. Dokumen ini tidak memberikan izin baru yang bertentangan dengan aturan RT yang sudah berjalan.
2. Akses baca lintas wilayah hanya berlaku untuk Beranda dan KAS sesuai matriks. Akses tersebut tidak mencakup Lokasi Dana.
3. Role RW lain yang tidak tercantum sebagai Ketua RW, Wakil Ketua RW, atau Bendahara RW tidak otomatis memperoleh akses ke tiga menu operasional RW.
4. Jika terdapat role tambahan atau permission existing yang bertentangan dengan matriks ini, lakukan audit sebelum perubahan. Jangan menghapus atau memperluas permission secara diam-diam.
5. Super Admin memiliki akses penuh untuk lingkup RT 002 dan RW 016 yang saat ini digunakan. Ketentuan ini bukan izin global tanpa batas terhadap seluruh wilayah.

## 5. Aturan Akses Silang RT dan RW

### 5.1 Perangkat RT melihat RW

Perangkat RT boleh:

- Membuka Beranda RW dalam mode baca.
- Membuka KAS RW dalam mode baca.
- Melihat informasi RW yang memang diizinkan untuk perangkat RT.

Perangkat RT tidak boleh:

- Membuka Lokasi Dana RW.
- Membuat atau mengubah transaksi RW.
- Mengelola rekening, brankas fisik, atau kas di tangan RW.
- Melakukan transfer antar-lokasi dana RW.
- Mengakses tindakan RW melalui endpoint langsung.

### 5.2 Perangkat RW melihat RT

Perangkat RW boleh:

- Membuka Beranda RT dalam mode baca.
- Membuka KAS RT dalam mode baca.
- Melihat informasi RT sesuai lingkup yang diizinkan.

Perangkat RW tidak boleh otomatis memperoleh akses ke Lokasi Dana RT atau tindakan pengelolaan keuangan RT.

### 5.3 Isolasi antar-RT

Perangkat suatu RT hanya boleh mengakses data RT yang menjadi kewenangannya untuk tindakan yang berada dalam lingkup RT.

Contoh:

- Perangkat RT 021 tidak boleh membuka detail transaksi RT 022.
- Perangkat RT 021 tidak boleh mengubah identifier RT pada request untuk memperoleh data RT 022.
- Perangkat RT 021 tidak boleh mengakses endpoint pengelolaan RT lain hanya karena endpoint tersebut dapat dipanggil dari aplikasi.
- Hak akses baca ke RW tidak memberikan akses ke seluruh RT di bawah RW.

Aturan ini wajib berlaku di Web, Android, dan backend/API.

## 6. Hak Akses Lokasi Dana RW

Lokasi Dana RW merupakan menu tersendiri dan tidak boleh disamakan dengan KAS RW.

Role yang mendapat akses penuh:

- Ketua RW.
- Wakil Ketua RW.
- Bendahara RW.
- Super Admin, terbatas pada lingkup kewenangannya.

Ketua RW, Wakil Ketua RW, dan Bendahara RW memiliki hak penuh sesuai kemampuan fitur Lokasi Dana RW, termasuk:

- Melihat ringkasan dan saldo lokasi.
- Melihat riwayat transaksi lokasi.
- Membuat lokasi dana sesuai fitur yang tersedia.
- Mengubah informasi lokasi sesuai permission.
- Mengaktifkan dan menonaktifkan lokasi.
- Melakukan transfer antar-lokasi sesuai aturan bisnis.
- Mengakses tindakan pengelolaan lain yang telah menjadi bagian dari fitur.

Akses penuh tetap tunduk pada validasi bisnis, konsistensi saldo, audit transaksi, dan aturan keamanan backend.

Perangkat RT maupun perangkat RW yang tidak memiliki role RW yang diizinkan tidak boleh mengakses Lokasi Dana RW.

## 7. Dukungan Jabatan Ganda

### 7.1 Perilaku yang diharapkan

Sistem harus membaca seluruh jabatan yang dimiliki pengguna dan menyusun menu berdasarkan gabungan hak aksesnya.

Contoh: pengguna dengan jabatan Bendahara RT dan Wakil Ketua RW.

Pengguna tersebut dapat memperoleh menu:

- `BERANDA`
- `KAS`
- `LOKASI DANA` apabila diizinkan oleh permission RT existing
- `BERANDA RW`
- `KAS RW`
- `LOKASI DANA RW`

Daftar aktual tetap ditentukan oleh role, permission, dan lingkup wilayah yang sah.

### 7.2 Tidak boleh hanya membaca satu role

Implementasi tidak boleh mengandalkan asumsi bahwa setiap pengguna hanya memiliki satu jabatan.

Jika kode existing memilih satu role utama lalu mengabaikan role lainnya, audit harus mengidentifikasi hal tersebut dan menentukan perubahan minimum agar hak akses gabungan dapat diterapkan dengan aman.

### 7.3 Tidak menggandakan hak tindakan

Gabungan jabatan bukan berarti semua menu menjadi dapat ditulis.

Contoh:

- Pengguna dapat mengelola KAS RT berdasarkan jabatan RT.
- Pengguna yang sama dapat melihat KAS RW hanya dalam mode baca apabila hak RW yang dimilikinya tidak memberikan izin tulis untuk KAS RW.
- Pengguna hanya boleh melakukan tindakan Lokasi Dana RW jika memiliki role RW yang secara eksplisit diizinkan.

## 8. Aturan Menu Web

Web harus menampilkan menu sesuai hak akses gabungan pengguna.

Ketentuan:

1. Nama menu RT existing tidak boleh berubah.
2. Menu RW ditambahkan dengan label `BERANDA RW`, `KAS RW`, dan `LOKASI DANA RW`.
3. Menu RT dan RW harus mengarah ke halaman dan lingkup data yang benar.
4. Pengguna dengan jabatan ganda dapat menjangkau kedua kelompok menu.
5. Menu yang tidak diizinkan tidak ditampilkan sebagai menu yang dapat digunakan.
6. Akses langsung ke URL tetap diperiksa oleh backend/API dan mekanisme otorisasi halaman yang tersedia.
7. Jangan membuat duplikasi menu RT dengan mengganti namanya menjadi menu RW.
8. Pertahankan pola visual dan navigasi Web existing, kecuali penyesuaian minimum yang diperlukan untuk menambahkan menu RW.

## 9. Aturan Navigasi Android

### 9.1 Horizontal sliding

Android bottom navigation harus mendukung pengguliran horizontal agar semua menu yang diizinkan tetap dapat dijangkau ketika jumlah menu melebihi lebar layar.

Solusi harus disesuaikan dengan struktur navigasi Flutter existing. Jika navigasi saat ini menggunakan komponen yang tidak mendukung pengguliran horizontal, komponen tersebut dapat disesuaikan secara terfokus tanpa merombak seluruh arsitektur navigasi.

### 9.2 Persyaratan perilaku

- Pengguna dapat menggeser navigasi secara horizontal untuk menjangkau menu tambahan.
- Semua menu yang diizinkan tetap tersedia dan tidak hilang karena keterbatasan lebar layar.
- Nama menu RT dan RW tetap dapat dibedakan dengan jelas.
- Menu aktif memiliki indikator visual yang konsisten.
- Saat pengguna membuka halaman atau berpindah menu, status aktif mencerminkan halaman yang sedang dibuka.
- Menu aktif sebaiknya tetap berada dalam area yang terlihat setelah navigasi atau deep link.
- Navigasi tetap nyaman digunakan pada layar Android dengan lebar berbeda.
- Jangan memaksakan semua menu tampil sekaligus jika menyebabkan teks terpotong atau target sentuh terlalu kecil.
- Pertahankan perilaku back navigation, deep link, notifikasi, dan alur navigasi existing.
- Pengguna dengan satu jabatan tidak boleh mengalami regresi hanya karena dukungan jabatan ganda ditambahkan.

### 9.3 Batas tanggung jawab

Horizontal sliding hanya mengubah cara menu dijangkau. Mekanisme tersebut tidak boleh menjadi sumber penentuan permission.

Menu yang ditampilkan tetap berasal dari hak akses yang valid, sedangkan backend/API tetap memvalidasi setiap request.

## 10. Ketentuan Backend dan API

Backend/API merupakan sumber otorisasi utama.

Implementasi wajib memastikan:

1. Setiap endpoint memvalidasi autentikasi, role, permission, dan lingkup wilayah.
2. Akses baca lintas RT–RW tidak otomatis memberikan izin tulis.
3. Request langsung ke endpoint yang tidak diizinkan ditolak meskipun pengguna mengetahui URL atau identifier resource.
4. Pengguna tidak dapat mengakses data RT lain dengan mengubah parameter atau identifier.
5. Endpoint Lokasi Dana RW tidak dapat diakses oleh perangkat RT atau role yang tidak diizinkan.
6. Endpoint Lokasi Dana RT tetap mengikuti aturan RT existing.
7. Akses Super Admin dibatasi pada lingkup RT 002 dan RW 016 yang telah ditetapkan.
8. Validasi backend tidak hanya mengandalkan nama menu, route frontend, atau parameter role dari client.
9. Permission existing yang masih benar dipertahankan.
10. Jika perubahan backend tidak diperlukan karena mekanisme existing sudah memenuhi persyaratan, perubahan tersebut tidak perlu dilakukan.

Jika diperlukan penambahan permission, gunakan mekanisme otorisasi yang sudah ada sejauh memungkinkan. Hindari menciptakan sistem RBAC kedua yang berjalan paralel tanpa alasan teknis yang kuat.

## 11. Prinsip Integritas Data

Penambahan akses RW tidak boleh mengubah aturan akuntansi atau perilaku transaksi yang sudah berjalan.

- KAS RT tetap bekerja dengan data dan aturan RT.
- KAS RW harus membaca dan mengelola data dalam lingkup RW sesuai kewenangannya.
- Lokasi Dana RW tetap terpisah dari Lokasi Dana RT.
- Akses baca tidak boleh memicu perubahan saldo atau pembuatan transaksi.
- Transfer antar-lokasi harus mengikuti validasi dan aturan transaksi existing pada lingkup yang bersangkutan.
- Tidak boleh terjadi duplikasi transaksi akibat akses dari dua platform.
- Tidak boleh terjadi kebocoran data hanya karena RT dan RW memiliki halaman dengan fungsi serupa.

Detail implementasi saldo dan transaksi mengikuti kontrak fitur existing. Perubahan akuntansi tidak boleh dilakukan sebagai bagian dari pekerjaan RBAC kecuali ditemukan kebutuhan yang terbukti dan disetujui secara terpisah.

## 12. Strategi Implementasi

Pekerjaan harus dilakukan bertahap agar perubahan tetap terkontrol.

### Tahap 1 — Audit tanpa perubahan

Periksa:

- Daftar role dan permission existing.
- Model hubungan pengguna, RT, dan RW.
- Mekanisme jabatan ganda.
- Middleware dan pemeriksaan otorisasi backend.
- Konfigurasi menu Web.
- Navigasi Android dan daftar menu yang ditampilkan.
- Endpoint Beranda, KAS, dan Lokasi Dana.
- Batas lingkup data serta permission Super Admin.
- Test existing yang terkait.

Catat file, fungsi, endpoint, dan aturan yang benar-benar ditemukan dalam kode. Jangan menganggap implementasi yang dilaporkan sebelumnya pasti masih sesuai dengan kondisi repository terkini.

### Tahap 2 — Matriks gap

Bandingkan hasil audit dengan dokumen ini. Kelompokkan temuan menjadi:

- Sudah sesuai.
- Belum tersedia.
- Permission terlalu luas.
- Permission terlalu sempit.
- Risiko kebocoran data.
- Potensi regresi fitur RT.
- Penyesuaian navigasi Android.
- Ketidakjelasan kontrak atau relasi data.

### Tahap 3 — Implementasi minimum

Lakukan hanya perubahan yang dibutuhkan untuk memenuhi matriks RBAC.

Prioritas:

1. Otorisasi backend dan batas lingkup data.
2. Penambahan menu RW tanpa mengubah menu RT existing.
3. Penggabungan menu sesuai jabatan ganda.
4. Horizontal sliding Android bottom navigation.
5. Penyesuaian pengujian.

Jangan menggabungkan pekerjaan ini dengan refactor besar atau perubahan fitur di luar cakupan.

### Tahap 4 — Verifikasi

Jalankan pengujian backend, Web, dan Android yang relevan. Laporkan hasil aktual, termasuk test yang gagal, test yang dilewati, dan keterbatasan lingkungan.

## 13. Kriteria Penerimaan

Implementasi dinyatakan memenuhi dokumen ini apabila seluruh kriteria berikut terpenuhi.

### Menu dan kompatibilitas

- [ ] Nama menu RT `BERANDA`, `KAS`, dan `LOKASI DANA` tetap tidak berubah.
- [ ] Menu `BERANDA RW`, `KAS RW`, dan `LOKASI DANA RW` tersedia secara terpisah.
- [ ] Menu RT dan RW tidak saling menggantikan atau menampilkan lingkup data yang salah.
- [ ] Fitur RT existing tidak mengalami regresi.

### RBAC dan jabatan ganda

- [ ] Perangkat RT dapat melihat Beranda RW dan KAS RW sesuai aturan baca.
- [ ] Perangkat RT tidak dapat mengakses Lokasi Dana RW.
- [ ] Perangkat RW dapat melihat Beranda RT dan KAS RT sesuai aturan baca.
- [ ] Perangkat RW tanpa role yang diizinkan tidak dapat mengakses Lokasi Dana RT.
- [ ] Ketua RW dapat mengakses tiga menu RW dengan hak penuh.
- [ ] Wakil Ketua RW dapat mengakses tiga menu RW dengan hak penuh.
- [ ] Bendahara RW dapat mengakses tiga menu RW dengan hak penuh.
- [ ] Role RW lain tidak otomatis memperoleh akses menu RW yang dibatasi.
- [ ] Pengguna dengan jabatan ganda melihat gabungan menu yang diizinkan.
- [ ] Penggabungan role tidak memperluas hak tindakan di luar kewenangan yang sah.

### Batas wilayah dan Super Admin

- [ ] Perangkat RT tidak dapat membaca atau mengubah data RT lain.
- [ ] Manipulasi URL, parameter ID, atau request API tidak melewati otorisasi.
- [ ] Super Admin memiliki akses penuh pada RT 002 dan RW 016 sesuai ketentuan.
- [ ] Akses Super Admin tidak otomatis meluas ke seluruh wilayah tanpa izin eksplisit.

### Android

- [ ] Bottom navigation dapat digeser horizontal.
- [ ] Semua menu yang diizinkan dapat dijangkau.
- [ ] Menu aktif terlihat dan mengikuti halaman yang sedang dibuka.
- [ ] Navigasi tidak mengalami overflow yang menghalangi interaksi.
- [ ] Back navigation dan deep link existing tetap berfungsi.
- [ ] Pengguna dengan satu jabatan tetap dapat menggunakan aplikasi seperti sebelumnya.

### Backend dan kualitas

- [ ] Akses silang baca saja tidak dapat digunakan untuk melakukan tindakan tulis.
- [ ] Endpoint Lokasi Dana memvalidasi permission dan lingkup wilayah.
- [ ] Pengujian backend, Web, dan Android yang relevan lulus.
- [ ] Pemeriksaan format perubahan, termasuk `git diff --check`, lulus.
- [ ] Tidak ada perubahan tidak terkait yang ikut masuk.
- [ ] Seluruh hasil pengujian dilaporkan berdasarkan eksekusi aktual.

## 14. Skenario Pengujian Minimum

| Skenario | Hasil yang diharapkan |
|---|---|
| Perangkat RT membuka KAS | Berhasil sesuai permission RT existing |
| Perangkat RT membuka KAS RW | Berhasil dalam mode baca |
| Perangkat RT mencoba mengubah transaksi RW | Ditolak |
| Perangkat RT membuka Lokasi Dana RW | Ditolak |
| Perangkat RT mencoba mengakses RT lain | Ditolak |
| Ketua RW membuka Beranda RW | Berhasil |
| Wakil Ketua RW membuka KAS RW | Berhasil |
| Bendahara RW mengelola Lokasi Dana RW | Berhasil |
| Perangkat RW membuka KAS RT | Berhasil dalam mode baca |
| Perangkat RW mencoba mengubah transaksi RT | Ditolak melalui akses silang |
| Role RW yang tidak diizinkan membuka Lokasi Dana RW | Ditolak |
| Pengguna dengan jabatan Bendahara RT dan Wakil Ketua RW login | Menu RT dan RW yang diizinkan tampil bersama |
| Pengguna jabatan ganda membuka KAS RT | Menggunakan lingkup RT |
| Pengguna jabatan ganda membuka KAS RW | Menggunakan lingkup RW |
| Super Admin mengakses RT 002 dan RW 016 | Berhasil sesuai lingkup |
| Super Admin mencoba mengakses wilayah di luar lingkup | Ditolak kecuali ada izin eksplisit |
| Android memiliki lebih banyak menu daripada lebar layar | Menu dapat dijangkau dengan horizontal sliding |
| Pengguna membuka halaman melalui deep link | Halaman dan status navigasi sesuai hak akses |
| Client mengirim request tanpa permission yang sah | Backend menolak request |

## 15. Perlindungan Working Tree dan Perubahan Existing

Sebelum implementasi:

1. Periksa branch, HEAD, status working tree, dan perubahan lokal.
2. Catat file yang sudah dimodifikasi atau belum terlacak.
3. Jangan membuang, menimpa, melakukan reset, atau menghapus perubahan yang bukan bagian dari pekerjaan ini.
4. Jangan melakukan amend atau force push.
5. Jangan mengubah file di luar cakupan tanpa alasan yang terdokumentasi.
6. Jangan menyatakan pekerjaan berhasil sebelum test benar-benar dijalankan.

Jika ditemukan masalah yang memerlukan perubahan database atau kontrak API besar, berhenti pada temuan dan jelaskan alasan serta dampaknya sebelum memperluas ruang lingkup.

## 16. Format Laporan Audit dan Implementasi

Laporan wajib mencakup:

1. Branch, HEAD, dan status working tree sebelum serta sesudah perubahan.
2. Kondisi RBAC existing yang ditemukan.
3. Matriks perbedaan antara implementasi existing dan dokumen ini.
4. Daftar file yang diubah beserta alasan setiap perubahan.
5. Endpoint dan permission yang diperiksa atau diubah.
6. Perilaku menu Web dan Android.
7. Hasil pengujian backend, Web, dan Android yang benar-benar dijalankan.
8. Hasil analyzer dan pemeriksaan perubahan.
9. Masalah yang belum terselesaikan dan risiko yang masih ada.
10. Konfirmasi bahwa nama, alur, dan perilaku fitur RT existing tetap dipertahankan.
11. Rekomendasi langkah berikutnya yang paling kecil dan aman.

## 17. Keputusan Final

Dokumen ini menjadi acuan RBAC operasional RT dan RW untuk fitur Beranda, KAS, dan Lokasi Dana.

Keputusan yang tidak boleh dilanggar:

- Menu RT existing tidak diubah namanya atau dirusak.
- Menu RW menggunakan suffix `RW`.
- Akses silang RT–RW untuk Beranda dan KAS bersifat baca saja.
- Lokasi Dana RW hanya dapat dikelola oleh role RW yang secara eksplisit diizinkan dan Super Admin dalam lingkupnya.
- Hak akses jabatan ganda merupakan gabungan izin yang sah, bukan satu role tunggal.
- Perangkat RT tidak dapat mengakses data RT lain.
- Super Admin dibatasi pada RT 002 dan RW 016 sesuai ketentuan saat ini.
- Android bottom navigation harus mendukung horizontal sliding.
- Backend/API tetap menjadi sumber otorisasi utama.
- Perubahan harus minimal, teruji, dan tidak merusak fitur existing.

**Akhir dokumen.**