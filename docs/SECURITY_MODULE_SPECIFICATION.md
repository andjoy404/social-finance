# Spesifikasi Modul Keamanan

**Proyek:** Social Finance Urena  
**Status:** Draft spesifikasi kebutuhan — belum merupakan persetujuan implementasi  
**Cakupan platform:** Backend/API, Web, Flutter, dan akun Perangkat Pos Keamanan  
**Tujuan dokumen:** Menjadi acuan analisis, perancangan, implementasi, dan pengujian Modul Keamanan.

---

## 1. Ringkasan

Modul Keamanan mengelola operasional petugas keamanan lingkungan RT, meliputi data petugas, shift, jadwal, absensi berbasis QR Code, pertukaran shift, koreksi absensi, buku tamu, perangkat pos, monitoring, dan laporan.

Fitur dirancang untuk mendukung tiga jenis aktor utama:

1. **Ketua RT / pengelola yang berwenang** — mengatur operasional dan memonitor data keamanan RT.
2. **Petugas Keamanan** — menggunakan akun pribadinya untuk melihat informasi yang relevan dan melakukan scan QR saat absensi.
3. **Perangkat Pos Keamanan** — akun/perangkat khusus di pos untuk menampilkan QR absensi dan menerima input buku tamu.

Data harus terisolasi per RT. Hak akses wajib ditegakkan di backend/API, bukan hanya dengan menyembunyikan menu pada aplikasi.

## 2. Prinsip dan batasan desain

- Jumlah shift dan jam kerja tidak boleh di-hardcode.
- Identitas petugas berasal dari akun pribadi petugas, bukan dari akun perangkat pos.
- Perangkat pos memiliki identitas/credential tersendiri dan terikat pada RT tertentu.
- QR Code diterbitkan dan diverifikasi oleh server.
- Perangkat pos tidak boleh melakukan absensi atas nama petugas.
- Data historis absensi harus tetap dapat diaudit meskipun shift atau jadwal berubah.
- Koreksi absensi harus melalui approval; tidak boleh mengubah catatan secara diam-diam.
- Input buku tamu hanya dilakukan melalui akun Perangkat Pos Keamanan.
- Pencabutan perangkat harus segera menghentikan akses perangkat.
- Waktu pencatatan berasal dari server dan ditampilkan dengan zona waktu RT yang berlaku.
- Jangan menganggap QR scan saja membuktikan lokasi fisik secara mutlak. QR membuktikan bahwa aplikasi petugas memindai kode yang diterbitkan perangkat terdaftar; pemeriksaan tambahan seperti GPS dapat dipertimbangkan sebagai opsi kebijakan, bukan asumsi wajib.

## 3. Aktor, akun, dan menu

### 3.1 Ketua RT

Menu pengelolaan:

**Kelola Keamanan**
- Data Petugas
- Pengaturan Shift
- Jadwal Petugas
- Perangkat Pos
- Pengaturan Absensi
- Pengaturan Buku Tamu

**Monitoring Keamanan**
- Dashboard
- Monitoring Absensi
- Monitoring Buku Tamu
- Monitoring Petugas
- Monitoring Perangkat
- Laporan Keamanan

Akses aktual harus mengikuti role dan permission yang berlaku di aplikasi. Jika pengelolaan nantinya didelegasikan ke pengurus lain, perlu menggunakan permission yang dapat diperluas, bukan ketergantungan permanen pada nama jabatan yang di-hardcode.

### 3.2 Akun Petugas Keamanan (HP pribadi)

Menu yang disepakati:
- Warga
- Absen — scan QR Code untuk check-in/check-out.
- Notifikasi — fitur yang sudah ada, termasuk foto mobil sampah dan riwayat notifikasi.
- Profil

**Batasan penting:** fitur Notifikasi yang sudah ada tidak boleh diubah sebagai bagian dari implementasi Modul Keamanan, kecuali ada kebutuhan terpisah dan persetujuan eksplisit.

Petugas hanya boleh melihat data absensi miliknya sendiri, kecuali ada permission tambahan yang secara eksplisit diberikan.

### 3.3 Akun Perangkat Pos Keamanan

Akun khusus untuk perangkat yang ditempatkan di pos keamanan. Akun ini memiliki menu tersendiri:
- Warga
- Buku Tamu — input dan pengelolaan operasional buku tamu di pos.
- Absensi — menghasilkan/menampilkan QR Code yang akan dipindai petugas.
- Sampah Tidak Terangkut — nama baru untuk menu Monitoring yang saat ini tersedia pada akun perangkat.

Akun perangkat bukan akun petugas dan tidak boleh memperoleh hak akses personal petugas. Implementasi boleh menggunakan principal/credential perangkat tersendiri atau mekanisme akun perangkat yang sesuai arsitektur autentikasi, tetapi identitas perangkat harus dapat dibedakan secara tegas dari user manusia.

Satu RT dapat memiliki lebih dari satu pos/perangkat di masa mendatang.

## 4. Data Petugas Keamanan

Ketua RT yang berwenang dapat:
- Menambahkan petugas.
- Mengubah data petugas.
- Mengaktifkan atau menonaktifkan petugas.
- Melihat riwayat penugasan.

Data minimal yang perlu dipertimbangkan:
- Identitas user yang menjadi petugas.
- Nama tampilan.
- Nomor telepon sesuai aturan akses data kontak.
- Status aktif/nonaktif.
- RT tempat bertugas.
- Tanggal mulai dan berakhir penugasan jika diperlukan.

Jangan membuat identitas petugas duplikat jika akun user sudah tersedia. Relasi petugas dengan user harus konsisten dengan pola data keanggotaan dan role aplikasi.

## 5. Pengaturan Shift

Ketua RT dapat membuat shift dengan jumlah bebas sesuai kebutuhan RT.

Contoh konfigurasi (ilustrasi, bukan nilai baku):

| Shift | Mulai | Selesai |
|---|---:|---:|
| Shift 1 | 06:00 | 14:00 |
| Shift 2 | 14:00 | 22:00 |
| Shift 3 | 22:00 | 06:00 |

Persyaratan:
- Nama shift dapat diubah.
- Jam mulai dan selesai dapat diatur.
- Mendukung shift yang melewati tengah malam.
- Pengaturan toleransi keterlambatan dan batas waktu absensi dapat dikonfigurasi.
- Jumlah shift tidak dibatasi pada tiga.
- Perubahan konfigurasi tidak boleh mengubah arti atau hasil absensi historis.
- Simpan snapshot atau referensi versi konfigurasi shift pada penugasan/absensi agar histori tetap dapat dijelaskan.

Perlu dirancang validasi untuk shift yang tumpang tindih, durasi tidak masuk akal, dan konfigurasi yang tidak lengkap.

## 6. Jadwal Petugas dan Pertukaran Shift

Ketua RT dapat menempatkan petugas pada shift dan tanggal tertentu.

Dua mekanisme penempatan yang disepakati:
1. Ketua RT dapat menetapkan jadwal petugas.
2. Petugas dapat mengajukan atau mengatur permintaan pertukaran shift dengan petugas lain melalui alur persetujuan.

Alur pertukaran shift yang direkomendasikan:
1. Petugas A mengajukan pertukaran dengan Petugas B.
2. Petugas B menyetujui atau menolak.
3. Jika keduanya setuju, permintaan diteruskan kepada Ketua RT/pihak berwenang.
4. Ketua RT menyetujui atau menolak.
5. Jadwal baru berlaku hanya setelah persetujuan final.
6. Semua keputusan dan perubahan dicatat dalam audit trail.

Jangan mengubah jadwal secara final sebelum persetujuan lengkap. Perlu aturan untuk pembatalan permintaan, permintaan yang bertabrakan, dan batas waktu pengajuan sebelum shift dimulai.

## 7. Absensi menggunakan QR Code

### 7.1 Alur umum

1. Perangkat Pos Keamanan login menggunakan akun/credential perangkat.
2. Perangkat membuka menu **Absensi**.
3. Server menerbitkan token QR yang bersifat sementara dan terikat pada perangkat serta RT.
4. Perangkat menampilkan QR Code.
5. Petugas login menggunakan akun pribadinya di HP.
6. Petugas membuka menu **Absen** dan memindai QR.
7. Backend memvalidasi identitas petugas, RT, perangkat, jadwal/shift, token QR, masa berlaku, serta status absensi.
8. Jika valid, server mencatat waktu absensi.
9. Aplikasi menampilkan hasil berdasarkan respons server.

### 7.2 Ketentuan QR

- Token dibuat oleh server dengan entropi/keacakan yang memadai.
- QR tidak memuat data pribadi atau rahasia.
- QR memiliki masa berlaku yang dapat dikonfigurasi; nilai awal yang disarankan 30–60 detik dan harus dapat ditinjau melalui pengaturan.
- Token terikat pada perangkat terdaftar dan RT.
- Token kedaluwarsa tidak dapat digunakan.
- Token yang sudah digunakan atau tidak lagi aktif tidak dapat dipakai ulang sesuai kebijakan anti-replay.
- QR harus diperbarui secara berkala.
- QR tidak boleh menjadi satu-satunya dasar untuk melewati validasi jadwal dan status absensi.
- Jika jaringan gagal atau server tidak mengonfirmasi transaksi, aplikasi tidak boleh menampilkan absensi berhasil.
- Hindari menyimpan token QR sensitif dalam log.

### 7.3 Check-in dan check-out

Sistem harus mendukung check-in dan check-out sesuai konfigurasi shift.

Validasi minimal:
- Akun petugas aktif dan berwenang.
- Petugas terdaftar pada RT yang sama dengan perangkat.
- Perangkat terdaftar dan belum dicabut.
- QR masih berlaku.
- Shift/jadwal relevan.
- Tidak terjadi check-in/check-out duplikat.
- Urutan absensi valid.

Sistem mencatat timestamp dari server. Aturan ketika petugas lupa check-out harus ditentukan dalam pengaturan atau alur koreksi, bukan dengan mengarang waktu otomatis tanpa jejak.

## 8. Status Absensi dan Ketidakhadiran

Status perlu membedakan petugas yang belum melakukan absensi dari petugas yang benar-benar tidak hadir.

Siklus status konseptual:
- Terjadwal
- Belum Hadir
- Hadir
- Terlambat
- Tidak Hadir
- Selesai

Ketentuan:
- Setelah waktu mulai shift dan masa toleransi tertentu, status dapat menjadi **Belum Hadir**.
- Jika melewati batas akhir/ambang yang dikonfigurasi tanpa check-in, sistem dapat menandai **Tidak Hadir**.
- Check-in setelah batas keterlambatan menghasilkan status **Terlambat**, sesuai aturan.
- Status otomatis harus menggunakan jadwal dan aturan yang berlaku untuk shift tersebut.
- Ketua RT dapat mengajukan koreksi status dengan alasan; koreksi tetap melalui approval.
- Simpan status awal, hasil otomatis, dan status final agar perubahan dapat ditelusuri.

Jangan mengubah status menjadi Tidak Hadir terlalu dini hanya karena petugas belum check-in beberapa menit setelah shift dimulai.

## 9. Koreksi Absensi dan Approval

Keputusan yang disepakati: koreksi absensi memerlukan persetujuan.

Alur:
1. Pemohon memilih catatan absensi yang perlu dikoreksi.
2. Pemohon mengisi nilai yang diminta dan alasan wajib.
3. Sistem membuat permintaan koreksi, tanpa langsung mengubah catatan final.
4. Approver yang berwenang menyetujui atau menolak.
5. Jika disetujui, perubahan diterapkan dan audit trail disimpan.
6. Pemohon dapat melihat status permintaan dan keputusan.

Audit trail minimal:
- Nilai lama.
- Nilai yang diminta/nilai baru.
- Alasan.
- Identitas pemohon.
- Identitas approver.
- Waktu permintaan.
- Waktu keputusan.
- Hasil keputusan.
- Catatan approver jika ada.

Aturan anti-self-approval:
- Pemohon tidak boleh menyetujui permintaannya sendiri.
- Jika petugas mengajukan koreksi, Ketua RT yang berwenang dapat menjadi approver.
- Jika Ketua RT sendiri mengajukan koreksi, persetujuan harus dialihkan ke Super Admin atau approver lain yang sah dan berbeda dari pemohon.

**Catatan keputusan terbuka:** mekanisme final untuk kasus Ketua RT sebagai pemohon perlu dikonfirmasi ketika desain permission dan proses bisnis difinalisasi. Jangan mengasumsikan Ketua RT boleh menyetujui koreksinya sendiri.

## 10. Buku Tamu

### 10.1 Kanal input

Keputusan yang disepakati: **input Buku Tamu hanya melalui Perangkat Pos Keamanan**. Akun HP pribadi petugas tidak mendapatkan menu input Buku Tamu.

Ketua RT/pihak berwenang dapat memonitor data melalui menu Monitoring Keamanan sesuai permission.

### 10.2 Field yang direncanakan

| Field | Ketentuan awal |
|---|---|
| Nama tamu | Wajib |
| Nomor telepon | Opsional |
| Alamat asal | Opsional |
| Warga/rumah tujuan | Wajib |
| Keperluan kunjungan | Wajib |
| Jenis kendaraan | Opsional |
| Nomor polisi | Opsional |
| Waktu masuk | Otomatis dari server |
| Waktu keluar | Dicatat saat tamu keluar |
| Petugas/perangkat pencatat | Otomatis dari identitas sesi |
| Catatan | Opsional |

Status:
- Masih di Dalam
- Sudah Keluar

Ketentuan:
- Waktu masuk tidak boleh diambil dari jam perangkat klien.
- Waktu keluar harus tercatat dengan identitas aktor dan timestamp.
- Data yang telah disimpan tidak boleh diedit tanpa jejak.
- Koreksi data buku tamu harus meninggalkan audit trail.
- Perlu mekanisme untuk menemukan kunjungan yang belum ditandai keluar.
- Terapkan prinsip minimisasi data; jangan meminta data pribadi yang tidak diperlukan.
- Tentukan masa retensi dan siapa yang boleh melihat nomor telepon/alamat tamu sebelum implementasi.

## 11. Perangkat Pos Keamanan

Ketua RT dapat:
- Mendaftarkan/memasangkan perangkat.
- Memberikan nama perangkat.
- Mengikat perangkat ke RT dan pos.
- Melihat status perangkat.
- Mencabut akses perangkat.

Informasi monitoring:
- Nama perangkat.
- RT/pos terkait.
- Status aktif/nonaktif.
- Online/offline berdasarkan heartbeat atau aktivitas terakhir.
- Waktu aktivitas terakhir.
- Status layanan QR.
- Riwayat registrasi dan pencabutan.

Keamanan:
- Credential/token perangkat berbeda dari credential petugas.
- Token perangkat dapat dicabut.
- Pencabutan harus segera menginvalidasi akses dan menghentikan penerbitan QR serta operasi Buku Tamu.
- Semua operasi perangkat harus memverifikasi RT dan status perangkat di backend.
- Jangan mengandalkan status yang hanya disimpan di UI atau cache klien.

## 12. Monitoring dan Dashboard Ketua RT

Ringkasan yang dapat ditampilkan:
- Petugas yang dijadwalkan/bertugas.
- Petugas yang sudah hadir.
- Petugas yang belum hadir.
- Petugas terlambat/tidak hadir.
- Tamu yang masih berada di dalam.
- Jumlah tamu hari ini.
- Permintaan koreksi yang menunggu persetujuan.
- Status perangkat pos.

### Monitoring Absensi
- Tanggal.
- Nama petugas.
- Shift.
- Jadwal.
- Check-in.
- Check-out.
- Status.
- Durasi jika relevan.
- Riwayat koreksi.

### Monitoring Buku Tamu
- Identitas tamu sesuai field yang diizinkan.
- Rumah/warga tujuan.
- Keperluan.
- Waktu masuk/keluar.
- Status masih di dalam/sudah keluar.
- Perangkat/petugas pencatat.

### Monitoring Petugas
- Daftar petugas aktif.
- Penugasan shift.
- Riwayat absensi.
- Keterlambatan/ketidakhadiran.
- Riwayat pertukaran shift.

### Monitoring Perangkat
- Status perangkat.
- Aktivitas terakhir.
- Riwayat registrasi/pencabutan.
- Status QR.

## 13. Laporan

Rencana laporan:
- Absensi per petugas.
- Absensi per tanggal/periode.
- Absensi per shift.
- Keterlambatan dan ketidakhadiran.
- Riwayat pertukaran shift.
- Riwayat koreksi dan approval.
- Buku tamu.
- Daftar tamu yang belum tercatat keluar.
- Aktivitas perangkat.

Ekspor PDF/Excel dapat dipertimbangkan mengikuti pola ekspor yang sudah ada di aplikasi. Hak akses ekspor harus sama ketatnya dengan hak akses melihat data.

## 14. Keamanan, Privasi, dan Isolasi Data

Persyaratan wajib:
- Semua query dan mutasi data dibatasi berdasarkan RT yang berwenang.
- Petugas hanya dapat melihat absensinya sendiri, kecuali ada permission khusus.
- Perangkat hanya dapat beroperasi untuk RT/pos tempat perangkat terdaftar.
- Ketua RT hanya dapat mengakses data RT yang menjadi kewenangannya.
- Validasi permission dilakukan di backend untuk setiap endpoint.
- UI menyembunyikan menu yang tidak berhak diakses, tetapi UI bukan mekanisme keamanan.
- Hindari IDOR dengan memvalidasi kepemilikan RT pada setiap resource.
- Perubahan data penting memiliki audit trail.
- Data kontak dan data tamu mengikuti prinsip least privilege.
- Jangan mencatat token, credential, atau data pribadi sensitif secara berlebihan di log.

## 15. Kebutuhan Pengujian

Minimal mencakup:

### Shift dan jadwal
- Jumlah shift bebas.
- Shift melewati tengah malam.
- Validasi jadwal bertabrakan.
- Perubahan shift tidak mengubah histori.
- Pertukaran shift memerlukan persetujuan kedua petugas dan approval Ketua RT.

### QR dan absensi
- QR valid, kedaluwarsa, tidak valid, dan replay.
- Perangkat dicabut.
- Petugas dari RT lain mencoba scan.
- Petugas tidak terjadwal mencoba absen.
- Check-in/check-out duplikat.
- Koneksi terputus sebelum server mengonfirmasi.
- Batas keterlambatan dan ketidakhadiran.
- Tidak ada keberhasilan palsu di UI.

### Approval
- Pemohon tidak dapat menyetujui permintaannya sendiri.
- Approval dan rejection tercatat.
- Perubahan hanya diterapkan setelah disetujui.
- Riwayat nilai lama dan baru tetap tersedia.

### Buku tamu
- Input hanya melalui akun perangkat.
- Validasi field wajib.
- Timestamp server.
- Check-out dan status tamu.
- Koreksi meninggalkan audit trail.
- Isolasi data antar-RT.

### RBAC dan perangkat
- Menu dan endpoint sesuai role.
- Petugas tidak memperoleh hak perangkat.
- Perangkat tidak dapat bertindak sebagai petugas.
- Pencabutan token langsung menolak operasi berikutnya.
- Pengguna RT lain tidak dapat mengakses data.

## 16. Hal yang Masih Perlu Diputuskan

Jangan mengarang keputusan untuk hal-hal berikut. Catat sebagai open questions saat desain rinci:

1. Approver pengganti untuk koreksi yang diajukan Ketua RT: Super Admin saja atau role lain yang ditunjuk?
2. Detail ambang waktu status Belum Hadir menjadi Tidak Hadir.
3. Apakah GPS/geofence diperlukan sebagai lapisan validasi opsional.
4. Kebijakan jika petugas lupa check-out.
5. Masa retensi data Buku Tamu.
6. Field Buku Tamu mana yang wajib disimpan sesuai kebijakan privasi.
7. Apakah satu RT langsung mendukung banyak perangkat/pos sejak versi pertama atau disiapkan untuk ekspansi.
8. Apakah pengelolaan dapat didelegasikan kepada jabatan selain Ketua RT melalui permission.

## 17. Catatan Implementasi

Dokumen ini adalah spesifikasi kebutuhan, bukan izin untuk langsung mengubah kode.

Sebelum implementasi:
- Audit arsitektur backend, autentikasi, RBAC, model RT, dan pola migration.
- Audit menu Web dan Flutter yang sudah ada.
- Identifikasi fitur yang dapat digunakan kembali.
- Buat rencana migration/API/UI dan strategi kompatibilitas.
- Jangan mengubah fitur Notifikasi yang sudah ada sebagai efek samping.
- Jangan mengubah data historis tanpa strategi migrasi dan audit yang jelas.
