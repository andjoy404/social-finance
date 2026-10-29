# Spesifikasi Fitur Menu Notifikasi --- Social Finance Urena

**Status:** Draft spesifikasi untuk audit dan implementasi\
**Repository:** `andjoy404/social-finance`\
**Branch kerja:** `urena`\
**Tujuan:** Menambahkan menu Notifikasi untuk mengelola notifikasi
sistem yang sudah ada dan menyiapkan fondasi pengelolaan template, hak
akses, serta pengingat otomatis di tahap berikutnya.

------------------------------------------------------------------------

## 1. Tujuan

Menyediakan satu menu **Notifikasi** sebagai pusat pengelolaan
notifikasi Urena. Menu ini harus terlebih dahulu menampilkan dan
mengelola sistem notifikasi yang memang sudah ada, bukan mengganti atau
merusak alur notifikasi existing.

Ruang lingkup awal: 1. Melihat daftar notifikasi/jenis notifikasi yang
sudah ada di sistem. 2. Menyiapkan pengelolaan template notifikasi:
tambah, edit, dan aktif/nonaktif. 3. Mengatur hak akses ke menu
Notifikasi. 4. Menyediakan rancangan untuk melihat riwayat pengiriman
jika mekanisme pencatatan tersedia atau perlu ditambahkan setelah audit.
5. Menyiapkan fondasi agar pengingat iuran bulanan dapat dikembangkan
kemudian, tetapi **jangan langsung mengimplementasikan scheduler
pengingat** kecuali diminta secara terpisah.

## 2. Hak akses awal

Untuk tahap awal, hanya: - `super_admin` - Ketua RT (gunakan sumber
jabatan/permission yang sudah berlaku di aplikasi; normalisasi nilai
sesuai pola existing)

yang boleh mengakses menu Notifikasi dan endpoint pengelolaannya.

Semua role/jabatan lain harus ditolak oleh backend meskipun mereka
mencoba memanggil endpoint secara langsung. Menyembunyikan menu atau
tombol di frontend saja tidak cukup.

Ketentuan: - Gunakan mekanisme autentikasi dan otorisasi yang sudah
ada. - Jangan memberi akses otomatis kepada semua pengurus atau
bendahara. - Jangan membuat sistem permission baru jika mekanisme yang
sudah ada bisa digunakan dengan aman. - Pastikan endpoint daftar,
detail, create, update, aktif/nonaktif, serta riwayat (jika ada)
menerapkan aturan akses yang sama sesuai kebutuhan. - Catat perubahan
konfigurasi penting melalui audit log jika pola audit log sudah
tersedia.

## 3. Konteks RT untuk superadmin --- RT 002

Ada permintaan agar akun `super_admin` memiliki konteks `rt_id` untuk
**RT 002**, sehingga dapat melihat seluruh menu aplikasi.

**Jangan langsung mengubah database atau menyamakan `rt_id` dengan hak
akses global. Audit dulu model tenant dan middleware otorisasi.**

Gemini harus: 1. Menemukan sumber `rt_id` yang dipakai oleh sesi/token
dan frontend untuk menentukan menu serta konteks RT. 2. Menjelaskan
apakah `super_admin` saat ini memang membutuhkan `rt_id` untuk merender
menu, memilih RT aktif, atau memanggil endpoint. 3. Menggunakan pola
resmi yang sudah ada untuk memberi konteks RT 002 kepada superadmin,
jika memang diperlukan dan aman. 4. Memastikan `super_admin` tetap
memiliki akses global sesuai desain aplikasi dan tidak terbatasi hanya
ke RT 002. 5. Memastikan perubahan ini tidak membuat pengguna biasa
dapat mengganti `rt_id` atau mengakses data RT lain. 6. Memastikan akses
data tenant tetap diverifikasi backend; `rt_id` pada UI/token tidak
boleh menjadi satu-satunya kontrol keamanan. 7. Tidak mengubah akun
warga/pengurus lain atau melakukan update massal. 8. Tidak mengubah data
produksi. Jika perlu perubahan data lokal untuk verifikasi, jelaskan
record yang akan disentuh dan minta persetujuan sebelum melakukan
perubahan yang berisiko.

Jika model aplikasi tidak mendukung superadmin global sekaligus konteks
RT aktif, **jangan melakukan workaround yang melemahkan isolasi
tenant**. Laporkan desain yang ada dan usulkan perubahan minimal yang
aman.

## 4. Menu dan rancangan antarmuka

Buat menu utama **Notifikasi** mengikuti pola navigasi, layout, desain,
responsivitas, dan komponen yang sudah digunakan Urena.

Bagian yang direncanakan:

### 4.1 Daftar Notifikasi Sistem

Tampilkan inventaris notifikasi yang ditemukan saat audit, misalnya: -
nama/jenis notifikasi; - pemicu atau event; - kanal yang benar-benar
didukung; - status konfigurasi; - template yang digunakan; - penerima
atau aturan penerima; - tindakan yang tersedia berdasarkan hak akses.

Jangan mengarang jenis notifikasi. Daftar harus berdasarkan hasil audit
kode dan konfigurasi existing.

### 4.2 Template Notifikasi

Rancang pengelolaan template dengan field yang benar-benar dibutuhkan,
misalnya: - nama template; - jenis/event pemicu; - judul; - isi pesan; -
kanal; - status aktif/nonaktif; - variabel yang diizinkan.

Validasi semua variabel terhadap daftar yang didukung sistem. Berikan
preview yang aman jika sesuai pola komponen aplikasi.

Jangan mengizinkan template mengubah status pembayaran, status periode,
saldo, atau aturan bisnis. Template hanya mengendalikan isi pesan, bukan
logika pemicu dan pengiriman.

### 4.3 Hak Akses

Pada tahap awal, akses menu hanya untuk `super_admin` dan Ketua RT.
Tidak perlu membuat UI pemberian izin ke pengurus lain pada tahap ini.
Hak akses yang lebih granular dapat menjadi pengembangan terpisah
setelah kebutuhan dikonfirmasi.

### 4.4 Riwayat Pengiriman

Audit apakah sistem sudah menyimpan riwayat pengiriman. Jika tersedia,
tampilkan status yang sesuai data sebenarnya. Jika belum tersedia,
laporkan kebutuhan skema dan dampaknya; jangan mengklaim status terkirim
jika sistem hanya mencatat bahwa notifikasi dibuat atau dimasukkan ke
antrean.

## 5. Batasan implementasi

-   Audit kode dan database schema terlebih dahulu sebelum menentukan
    desain teknis.
-   Pertahankan notifikasi existing; jangan mengubah pemicu, penerima,
    atau isi notifikasi yang sedang berjalan tanpa kebutuhan dan test
    yang jelas.
-   Jangan mengubah alur pembayaran atau verifikasi pembayaran.
-   Jangan mengubah Iuran Sukarela, FIFO, status aktif/nonaktif tagihan,
    saldo, atau aturan keuangan.
-   Jangan mengimplementasikan pengingat bulanan otomatis pada tahap ini
    kecuali diminta secara eksplisit.
-   Jangan membuat kanal WhatsApp/email jika integrasinya belum ada dan
    belum disetujui.
-   Jangan menghapus data atau riwayat notifikasi.
-   Jangan menjalankan `docker compose down -v`.
-   Jangan reset database.
-   Jangan commit, push, atau deploy.
-   Pertahankan semua perubahan lokal yang sudah ada.

## 6. Tahapan kerja yang diminta kepada Gemini

### Tahap A --- Audit

1.  Periksa `git status --short` dan diff sebelum mengedit.
2.  Inventarisasi notifikasi existing dari backend, frontend, mobile,
    database, worker/scheduler, dan push provider yang benar-benar ada.
3.  Petakan event/pemicu, template, penerima, kanal, dan alur otorisasi.
4.  Audit struktur `rt_id`, role/jabatan, superadmin, dan aturan isolasi
    tenant.
5.  Identifikasi menu/navigasi yang relevan dan pola halaman pengelolaan
    yang dapat digunakan ulang.
6.  Laporkan temuan dan rencana perubahan minimal sebelum melakukan
    perubahan yang berisiko.

### Tahap B --- Implementasi

Setelah audit: 1. Tambahkan menu Notifikasi dengan akses UI hanya untuk
`super_admin` dan Ketua RT. 2. Terapkan otorisasi yang sama di backend.
3. Implementasikan bagian yang didukung oleh struktur existing; jangan
membuat UI palsu yang tidak tersambung ke API. 4. Jika template dinamis
membutuhkan perubahan schema/API, implementasikan dengan migrasi yang
mengikuti pola proyek dan validasi yang sesuai. 5. Tangani konteks
`rt_id` superadmin untuk RT 002 dengan pendekatan yang aman sebagaimana
Bagian 3. Jangan melemahkan isolasi tenant demi menampilkan menu. 6.
Tambahkan test regresi dan pertahankan kompatibilitas notifikasi
existing.

Jika lingkup terlalu besar untuk satu perubahan aman, prioritaskan
audit + menu/inventaris existing terlebih dahulu, lalu laporkan bagian
template/riwayat yang memerlukan keputusan atau tahap lanjutan.

## 7. Verifikasi dan acceptance criteria

Fitur belum dianggap selesai kecuali: - Menu Notifikasi tampil untuk
`super_admin` dan Ketua RT. - Menu tidak tampil bagi role/jabatan yang
tidak berwenang. - Endpoint backend juga menolak akses tidak
berwenang. - Daftar notifikasi mencerminkan sistem existing yang
benar-benar ditemukan. - Perubahan template, jika diimplementasikan,
tersimpan ke backend dan tetap benar setelah reload. - Notifikasi
existing tetap berjalan dengan perilaku yang sama. - Konteks RT 002 pada
superadmin diverifikasi tanpa membatasi akses global atau melemahkan
isolasi tenant. - Test backend, frontend, dan Flutter yang relevan
dijalankan sesuai cakupan perubahan. - `git diff --check` bersih. -
Tidak ada commit, push, deploy, reset database, atau tindakan
destruktif.

## 8. Format laporan akhir

Gemini harus melaporkan: 1. Inventaris notifikasi existing beserta
pemicu, kanal, dan penerimanya. 2. Temuan mengenai akses menu Notifikasi
dan otorisasi backend. 3. Cara `rt_id` superadmin ditentukan dan apakah
konteks RT 002 diperlukan. 4. File yang diubah dan alasan setiap
perubahan. 5. Migrasi/API baru, jika ada. 6. Test yang benar-benar
dijalankan dan hasil aktual. 7. Bukti bahwa notifikasi existing tidak
rusak. 8. Risiko atau bagian yang belum diimplementasikan. 9. Output
`git status --short` dan `git diff --check`.

**Instruksi terakhir:** jangan commit, push, deploy, atau mengubah data
produksi. Jangan menganggap pemberian `rt_id = RT 002` sebagai pengganti
otorisasi superadmin global. Keamanan tenant harus tetap dijaga.
