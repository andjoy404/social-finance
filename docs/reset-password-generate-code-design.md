# Desain Fitur Generate Code untuk Aktivasi Akun dan Reset Password

**Project:** Social Finance Urena  
**Status:** Draft – Business Rules  
**Dokumen:** `docs/reset-password-generate-code-design.md`

---

## 1. Latar Belakang

Saat ini, pengguna baru yang dibuat melalui fitur import Excel memiliki akun di sistem, tetapi belum memiliki password yang dapat digunakan untuk login.

Agar warga dapat menggunakan akun tersebut, pengurus yang berwenang harus membuatkan password secara manual satu per satu.

Cara ini kurang efisien, terutama ketika jumlah warga yang diimpor cukup banyak.

Untuk mengatasi masalah tersebut, diperlukan fitur **Generate Code** yang memungkinkan pengurus menghasilkan kode verifikasi agar warga dapat menetapkan password sendiri.

Fitur ini juga dapat digunakan untuk membantu pengguna yang sudah memiliki akun tetapi lupa password.

## 2. Tujuan

1. Mempermudah aktivasi akun warga hasil import Excel.
2. Menghilangkan kebutuhan pengurus untuk membuat password warga satu per satu.
3. Memungkinkan warga menetapkan password sendiri menggunakan kode yang diberikan pengurus.
4. Menyediakan mekanisme reset password bagi pengguna yang sudah aktif.
5. Menjaga keamanan proses aktivasi dan reset password.
6. Mencatat aktivitas generate dan penggunaan kode untuk keperluan audit.

## 3. Konsep Utama

Pengurus yang memiliki kewenangan dapat melakukan Generate Code untuk akun pengguna tertentu.

Kode tersebut diberikan kepada pengguna yang bersangkutan agar dapat digunakan untuk menetapkan password baru.

Sistem hanya memperbolehkan satu kode aktif untuk setiap akun pada satu waktu.

### 3.1 Aturan Generate Code

- Generate Code dilakukan untuk satu akun pengguna yang dipilih.
- Pengurus yang berwenang dapat menghasilkan kode.
- Tidak diperlukan persetujuan dari beberapa pengurus.
- Tidak diperlukan empat kode dari empat pengurus berbeda.
- Jika pengurus lain melakukan Generate Code untuk akun yang sama, kode sebelumnya langsung tidak berlaku.
- Hanya kode yang terakhir dihasilkan yang dapat digunakan.

**Contoh:**

| Urutan | Aktivitas | Status kode |
|---|---|---|
| 1 | Pengurus A generate kode 1234 | Aktif |
| 2 | Pengurus B generate kode 5678 | Kode 1234 tidak berlaku |
| 3 | Pengguna memasukkan kode 1234 | Ditolak |
| 4 | Pengguna memasukkan kode 5678 | Diterima jika masih memenuhi seluruh validasi |

Tidak ada FIFO atau antrean kode.

Kode terbaru langsung menggantikan kode sebelumnya.

### 3.2 Satu Kode Aktif per Pengguna

Ketentuan:

- Satu pengguna hanya boleh memiliki satu kode aktif.
- Generate ulang akan mengganti kode sebelumnya.
- Kode lama tidak boleh digunakan kembali.
- Kode yang sudah digunakan tidak boleh digunakan kembali.
- Kode yang sudah kedaluwarsa tidak boleh digunakan.

Aturan ini berlaku secara individual untuk setiap akun, bukan secara global untuk seluruh RT.

## 4. Skenario Penggunaan

### 4.1 Aktivasi Akun Baru Hasil Import Excel

Alur:

1. Pengurus melakukan import data warga melalui Excel.
2. Sistem membuat akun pengguna untuk warga yang memenuhi ketentuan import.
3. Akun belum memiliki password aktif yang dapat digunakan untuk login.
4. Pengurus membuka pengelolaan pengguna.
5. Pengurus memilih akun warga.
6. Pengurus menekan tombol Generate Code.
7. Sistem menghasilkan kode aktivasi.
8. Pengurus menyampaikan kode kepada warga melalui kanal komunikasi yang tersedia.
9. Warga membuka halaman aktivasi atau pengaturan password.
10. Warga memasukkan kode.
11. Warga mengisi password baru dan konfirmasi password.
12. Sistem memvalidasi kode.
13. Jika valid, sistem menyimpan password menggunakan mekanisme hashing yang aman.
14. Kode ditandai sudah digunakan.
15. Warga dapat login menggunakan akun dan password yang baru dibuat.

### 4.2 Reset Password Pengguna yang Sudah Aktif

Alur:

1. Pengguna tidak dapat login karena lupa password.
2. Pengurus yang berwenang membuka pengelolaan pengguna.
3. Pengurus memilih akun pengguna.
4. Pengurus menekan Generate Code.
5. Sistem menghasilkan kode baru.
6. Kode sebelumnya, jika ada, langsung tidak berlaku.
7. Pengurus menyampaikan kode kepada pengguna.
8. Pengguna memasukkan kode pada halaman Reset Password.
9. Pengguna mengisi password baru dan konfirmasi password.
10. Sistem memvalidasi kode.
11. Jika valid, password diperbarui.
12. Kode ditandai sudah digunakan.
13. Pengguna dapat login menggunakan password baru.

## 5. Aturan Keamanan

Fitur ini berkaitan langsung dengan keamanan akun sehingga harus memperhatikan ketentuan berikut.

### 5.1 Validasi Kode

Kode harus memenuhi ketentuan:

- Terikat pada satu akun pengguna.
- Terikat pada RT yang sesuai.
- Hanya kode terbaru yang berlaku.
- Tidak dapat digunakan oleh akun lain.
- Tidak dapat digunakan lebih dari satu kali.
- Memiliki masa berlaku.
- Tidak dapat digunakan setelah kedaluwarsa.
- Tidak dapat digunakan setelah diganti oleh Generate Code berikutnya.

### 5.2 Penyimpanan Kode

Rekomendasi implementasi:

- Jangan menyimpan kode dalam bentuk plaintext di database.
- Simpan hash kode.
- Jangan mengembalikan kode melalui API setelah proses generate selesai.
- Kode hanya ditampilkan kepada pengurus yang melakukan generate pada saat proses tersebut berlangsung.
- Hindari mencatat kode asli ke application log.

### 5.3 Masa Berlaku

Masa berlaku kode harus dibatasi.

Nilai durasi belum ditetapkan dan perlu dikonfirmasi sebelum implementasi final.

### 5.4 Pembatasan Percobaan

Sistem perlu membatasi percobaan memasukkan kode untuk mencegah brute force.

Ketentuan yang perlu diterapkan:

- Batas percobaan kode yang salah.
- Cooldown atau pembatasan sementara setelah terlalu banyak kegagalan.
- Rate limiting pada endpoint validasi.
- Respons API tidak boleh membocorkan apakah kode tertentu valid untuk akun lain.

### 5.5 Penggantian Password

- Password harus di-hash menggunakan mekanisme keamanan yang sudah digunakan backend.
- Password baru harus memenuhi kebijakan password aplikasi.
- Kode langsung dinonaktifkan setelah reset berhasil.
- Pertimbangkan invalidasi sesi atau token login lama setelah password berhasil diubah.

## 6. Hak Akses

Generate Code hanya boleh dilakukan oleh pengurus yang memiliki kewenangan.

Ketentuan:

- Warga tidak boleh membuat kode aktivasi untuk dirinya sendiri melalui endpoint pengurus.
- Pengurus hanya dapat melakukan Generate Code untuk akun dalam RT yang menjadi kewenangannya.
- Super Admin harus mengikuti batasan scope RT yang dipilih atau diotorisasi.
- Tidak boleh ada akses lintas RT.
- Semua endpoint harus memvalidasi permission di backend, bukan hanya menyembunyikan tombol pada UI.

**Catatan:** Daftar role pengurus yang secara spesifik boleh melakukan Generate Code masih perlu dikonfirmasi berdasarkan RBAC Social Finance Urena.

## 7. Audit Log

Setiap aktivitas penting perlu dicatat.

Data audit yang direkomendasikan:

- ID pengguna yang menjadi target.
- ID pengurus yang melakukan Generate Code.
- RT terkait.
- Waktu generate.
- Waktu kode digunakan.
- Status kode: aktif, diganti, digunakan, kedaluwarsa, atau dibatalkan.
- Jumlah percobaan validasi yang gagal.
- Waktu reset password berhasil.

Jangan menyimpan kode asli maupun password ke dalam audit log.

## 8. Kebutuhan UI

### 8.1 Pengelolaan Pengguna oleh Pengurus

Pada detail pengguna, sediakan:

- Tombol Generate Code.
- Informasi status aktivasi akun.
- Informasi apakah kode masih aktif atau sudah kedaluwarsa.
- Waktu generate terakhir.
- Informasi pengurus yang melakukan generate terakhir.

Setelah Generate Code berhasil:

- Tampilkan kode yang baru dihasilkan.
- Sediakan tombol Copy.
- Berikan informasi bahwa kode lama sudah tidak berlaku.
- Jangan menampilkan kode lama.

### 8.2 Halaman Aktivasi / Reset Password

Sediakan form:

- Kode verifikasi.
- Password baru.
- Konfirmasi password.
- Tombol Simpan Password.

Berikan pesan yang sesuai untuk kondisi:

- Kode tidak valid.
- Kode sudah digunakan.
- Kode sudah kedaluwarsa.
- Kode sudah diganti dengan kode baru.
- Terlalu banyak percobaan.
- Password tidak memenuhi persyaratan.
- Password berhasil diperbarui.

Pesan kesalahan tidak boleh membocorkan informasi sensitif.

## 9. Rekomendasi Endpoint API

Nama endpoint berikut merupakan usulan desain, bukan endpoint yang sudah tersedia.

| Method | Endpoint | Fungsi |
|---|---|---|
| POST | `/api/users/:id/generate-password-code` | Generate kode untuk pengguna |
| POST | `/api/auth/activate-account` | Aktivasi akun dan menetapkan password |
| POST | `/api/auth/reset-password-with-code` | Reset password menggunakan kode |

Backend harus memastikan proses validasi kode dan perubahan password berlangsung secara atomik agar satu kode tidak dapat digunakan bersamaan oleh beberapa request.

## 10. Aturan Penggantian Kode

Contoh kondisi:

**Kondisi A: Belum pernah Generate**

- Belum ada kode aktif.
- Pengurus melakukan Generate Code.
- Kode baru menjadi kode aktif.

**Kondisi B: Generate ulang oleh pengurus yang sama**

- Kode lama langsung tidak berlaku.
- Kode baru menjadi satu-satunya kode aktif.

**Kondisi C: Generate oleh pengurus berbeda**

- Kode lama langsung tidak berlaku.
- Kode baru menjadi satu-satunya kode aktif.
- Tidak diperlukan persetujuan pengurus sebelumnya.

**Kondisi D: Kode sudah digunakan**

- Kode menjadi tidak aktif.
- Kode tidak dapat digunakan kembali.
- Untuk proses baru, pengurus harus melakukan Generate Code lagi.

**Kondisi E: Kode kedaluwarsa**

- Kode tidak dapat digunakan.
- Pengurus harus melakukan Generate Code kembali.

## 11. Hal yang Tidak Termasuk dalam Fitur

- Tidak ada persetujuan empat pengurus.
- Tidak ada pengumpulan empat kode.
- Tidak ada FIFO atau antrean kode.
- Tidak ada mekanisme kode lama tetap berlaku setelah generate ulang.
- Tidak ada pembuatan password warga secara manual oleh pengurus.
- Tidak ada perubahan data identitas warga dari proses reset password.

## 12. Kriteria Penerimaan (Acceptance Criteria)

1. Pengurus berwenang dapat Generate Code untuk akun warga.
2. Pengguna hasil import Excel dapat mengaktifkan akun menggunakan kode.
3. Pengguna aktif dapat reset password menggunakan kode.
4. Satu akun hanya memiliki satu kode aktif.
5. Generate ulang langsung membatalkan kode sebelumnya.
6. Kode lama tidak dapat digunakan setelah generate ulang.
7. Kode hanya berlaku untuk akun yang dituju.
8. Kode tidak dapat digunakan lebih dari satu kali.
9. Kode kedaluwarsa tidak dapat digunakan.
10. Percobaan kode dibatasi.
11. Password disimpan menggunakan hashing yang aman.
12. Aktivitas generate dan penggunaan kode tercatat dalam audit log.
13. Tidak ada akses lintas RT.
14. Permission diperiksa di backend.
15. Proses reset password tidak mengubah data identitas atau data keuangan pengguna.

## 13. Keputusan yang Sudah Disepakati

| No. | Keputusan | Status |
|---|---|---|
| 1 | Fitur digunakan untuk aktivasi akun hasil import Excel | Disepakati |
| 2 | Fitur juga mendukung reset password akun aktif | Disepakati |
| 3 | Kode dibuat oleh pengurus yang berwenang | Disepakati |
| 4 | Tidak memerlukan empat pengurus berbeda | Disepakati |
| 5 | Tidak ada persetujuan berlapis | Disepakati |
| 6 | Hanya satu kode aktif per akun | Disepakati |
| 7 | Generate ulang menggantikan kode sebelumnya | Disepakati |
| 8 | Hanya kode terbaru yang berlaku | Disepakati |
| 9 | Tidak menggunakan FIFO | Disepakati |

## 14. Keputusan yang Masih Perlu Dikonfirmasi

| No. | Pertanyaan | Status |
|---|---|---|
| 1 | Apakah kode terdiri dari 4 digit angka? | Belum dikonfirmasi |
| 2 | Berapa lama masa berlaku kode? | Belum ditetapkan |
| 3 | Role apa saja yang boleh Generate Code? | Belum ditetapkan |
| 4 | Bagaimana distribusi kode kepada warga? | Belum ditetapkan |
| 5 | Apakah kode ditampilkan sekali saja setelah generate? | Perlu konfirmasi |
| 6 | Berapa batas percobaan kode yang salah? | Belum ditetapkan |
| 7 | Apakah seluruh sesi login lama harus dicabut setelah reset? | Perlu keputusan teknis |

---

**Status dokumen: Draft desain bisnis dan keamanan.**

Dokumen ini menjadi dasar pembahasan dan implementasi fitur Generate Code untuk aktivasi akun dan reset password. Implementasi belum boleh menganggap keputusan pada bagian 14 sudah disetujui.