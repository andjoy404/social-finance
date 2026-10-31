# Notification History — Final Design
## Social Finance Urena

**Status:** FINAL — READY FOR IMPLEMENTATION  
**Scope:** Backend, Web, Android  
**Feature:** Notification History / Riwayat Notifikasi

---

## 1. Tujuan dan Latar Belakang

Membangun fitur riwayat notifikasi yang memungkinkan seluruh warga dan pengguna aplikasi melihat kembali notifikasi yang pernah diterima.

Saat ini, notifikasi push pada perangkat memiliki keterbatasan: notifikasi dapat hilang karena dihapus pengguna, perangkat berganti, aplikasi diinstal ulang, atau pengguna tidak membuka notifikasi ketika pertama kali diterima.

Notification History menyediakan penyimpanan permanen di backend agar seluruh notifikasi yang relevan dapat diakses kembali melalui aplikasi.

### Tujuan utama

1. Menyediakan arsip notifikasi yang pernah diterima pengguna.
2. Mendukung notifikasi pribadi dan pengumuman umum RT.
3. Menyediakan status belum dibaca dan sudah dibaca.
4. Menjamin isolasi data antar pengguna dan antar RT.
5. Mengintegrasikan riwayat dengan sistem push notification yang sudah ada.
6. Menyediakan akses ke detail atau halaman tujuan notifikasi jika masih tersedia.

## 2. Keputusan Bisnis Final

| ID | Keputusan | Status |
|---|---|---|
| ND-01 | Semua warga memiliki menu Notifikasi | Approved |
| ND-02 | Notifikasi pribadi hanya terlihat oleh penerima | Approved |
| ND-03 | Pengumuman umum dapat dilihat seluruh warga RT tujuan | Approved |
| ND-04 | Setiap pengguna memiliki status baca sendiri | Approved |
| ND-05 | Riwayat disimpan di backend | Approved |
| ND-06 | Riwayat tetap tersedia setelah logout atau ganti perangkat | Approved |
| ND-07 | Urutan notifikasi terbaru di atas | Approved |
| ND-08 | Tersedia detail notifikasi | Approved |
| ND-09 | Tidak ada penghapusan otomatis pada MVP | Approved |
| ND-10 | Status baca disinkronkan antarperangkat | Approved |
| ND-11 | Notifikasi push dan riwayat merupakan dua mekanisme berbeda | Approved |
| ND-12 | Pengguna tidak dapat membaca notifikasi pribadi milik pengguna lain | Approved |

## 3. Cakupan Pengguna dan Hak Akses

Menu Notifikasi tersedia untuk seluruh pengguna yang memiliki akses aplikasi.

### 3.1 Warga

Warga dapat:
- Melihat notifikasi pribadi.
- Melihat pengumuman umum RT.
- Membuka detail notifikasi.
- Menandai notifikasi sebagai sudah dibaca.
- Melihat jumlah notifikasi belum dibaca.

Warga tidak dapat:
- Melihat notifikasi pribadi warga lain.
- Membuat pengumuman umum.
- Mengubah notifikasi milik pengguna lain.

### 3.2 Pengurus RT

Pengurus RT dapat:
- Mengakses Notification History.
- Melihat notifikasi pribadi miliknya.
- Melihat pengumuman umum RT yang menjadi bagian dari scope membership-nya.
- Membaca dan menandai notifikasi miliknya.

Pembuatan pengumuman mengikuti permission pengelolaan pengumuman yang berlaku, bukan otomatis diberikan kepada seluruh pengurus.

### 3.3 Pengurus RW

Pengurus RW dapat melihat notifikasi yang memang ditujukan kepadanya serta pengumuman sesuai scope penerimaannya.

Tidak boleh ada kebocoran riwayat notifikasi RT lain.

### 3.4 Super Admin

Super Admin mengikuti mekanisme scope RT yang sedang dipilih atau diotorisasi.

Tidak boleh menggunakan hak Super Admin untuk membaca riwayat notifikasi pribadi seluruh warga.

## 4. Jenis Notifikasi

Notification History mendukung dua jenis penerima.

### 4.1 Personal Notification

Notifikasi yang ditujukan kepada satu atau beberapa akun tertentu.

Contoh:

- Pembayaran iuran berhasil diverifikasi.
- Pembayaran iuran ditolak.
- Pengingat tagihan iuran.
- Informasi administrasi pribadi.
- Perubahan status transaksi.
- Informasi khusus akun pengguna.

Contoh:

**Pembayaran Iuran Berhasil**

Pembayaran Iuran Oktober 2026 telah diverifikasi oleh Bendahara.

Penerima: akun warga yang melakukan pembayaran.

### 4.2 Public RT Announcement

Pengumuman yang ditujukan kepada seluruh pengguna aktif dalam satu RT.

Contoh:

- Pengumuman kerja bakti.
- Informasi rapat warga.
- Informasi kegiatan lingkungan.
- Pengumuman jadwal pelayanan.
- Informasi penting dari pengurus RT.

Contoh:

**Pengumuman Kerja Bakti**

Diberitahukan kepada seluruh warga RT 021 bahwa kerja bakti akan dilaksanakan pada hari Minggu pukul 07.00 WIB.

Penerima: seluruh pengguna yang memiliki keanggotaan aktif pada RT tujuan.

Pengumuman umum tidak boleh otomatis tersebar ke RT lain.

## 5. Desain UI/UX

### 5.1 Menu Navigasi

Tambahkan menu:

**Notifikasi**

Menu tersedia bagi warga dan pengurus.

Ikon menggunakan `bell`.

Badge angka menunjukkan jumlah notifikasi yang belum dibaca.

Contoh:

- Notifikasi (3)
- Notifikasi (0)

Angka harus berasal dari backend dan konsisten pada seluruh perangkat.

### 5.2 Halaman Notification History

Komponen utama:

1. Header Notifikasi.
2. Informasi jumlah belum dibaca.
3. Daftar riwayat notifikasi.
4. Penanda sudah dibaca atau belum.
5. Waktu notifikasi.
6. Ikon kategori.
7. Pagination atau infinite scrolling.

Contoh tampilan:

| Elemen | Keterangan |
|---|---|
| Judul | Pembayaran Iuran Berhasil |
| Deskripsi | Pembayaran Oktober telah diverifikasi |
| Kategori | Iuran |
| Waktu | 30 Oktober 2026, 10:30 |
| Status | Belum dibaca |

### 5.3 Status Visual

Notifikasi belum dibaca:
- Background sedikit berbeda.
- Judul lebih tegas.
- Indikator titik belum dibaca.

Notifikasi sudah dibaca:
- Background normal.
- Judul normal.
- Tidak menampilkan indikator belum dibaca.

Jangan menggunakan warna terlalu mencolok.

### 5.4 Detail Notifikasi

Ketika pengguna memilih notifikasi:

- Tampilkan judul.
- Tampilkan isi lengkap.
- Tampilkan kategori.
- Tampilkan tanggal dan waktu.
- Tampilkan informasi pengirim jika relevan.
- Tampilkan tombol menuju halaman terkait apabila tersedia.

Notifikasi otomatis ditandai sudah dibaca ketika detail berhasil dibuka.

Jika halaman tujuan sudah tidak tersedia, detail notifikasi tetap bisa dibaca.

### 5.5 Filter

Untuk MVP, cukup menyediakan:
- Semua.
- Belum Dibaca.
- Sudah Dibaca.

Filter kategori dapat ditambahkan pada tahap pengembangan berikutnya.

### 5.6 Empty State

Jika belum ada notifikasi:

**Belum Ada Notifikasi**

Belum ada pemberitahuan untuk akun Anda.

Jangan menampilkan daftar kosong tanpa penjelasan.

## 6. Arsitektur Backend

### 6.1 Prinsip Arsitektur

Notification History menjadi bagian dari Notification Service yang sudah ada.

WAJIB memeriksa implementasi notifikasi existing terlebih dahulu.

Jangan membuat Notification Service kedua jika sistem existing sudah menyediakan mekanisme tersebut.

Alur yang disarankan:

1. Business event terjadi.
2. Backend membuat record notifikasi.
3. Backend menentukan penerima.
4. Backend menyimpan data penerima.
5. Backend mengirim push notification melalui mekanisme existing.
6. Pengguna membuka menu Notifikasi.
7. Backend mengambil riwayat berdasarkan akun dan scope.
8. Status baca disimpan di backend.

Kegagalan pengiriman push tidak boleh menghapus riwayat notifikasi.

### 6.2 Model Database Konseptual

Desain berikut merupakan model target. Nama tabel dan kolom harus disesuaikan dengan struktur database existing setelah audit.

#### A. notifications

Menyimpan konten utama notifikasi.

| Field | Tipe konseptual | Keterangan |
|---|---|---|
| id | UUID | Primary key |
| title | VARCHAR | Judul |
| body | TEXT | Isi notifikasi |
| category | VARCHAR | Kategori |
| audience_type | VARCHAR | personal / public_rt |
| rt_id | UUID nullable | RT tujuan |
| actor_user_id | UUID nullable | Pembuat/pemicu |
| entity_type | VARCHAR nullable | Jenis objek terkait |
| entity_id | UUID nullable | ID objek terkait |
| deep_link | VARCHAR nullable | Tujuan navigasi |
| created_at | TIMESTAMP | Waktu pembuatan |

#### B. notification_recipients

Menyimpan relasi penerima dan status membaca.

| Field | Tipe konseptual | Keterangan |
|---|---|---|
| id | UUID | Primary key |
| notification_id | UUID | FK notifications |
| user_id | UUID | FK users |
| rt_id | UUID nullable | Scope RT penerimaan |
| read_at | TIMESTAMP nullable | Waktu dibaca |
| created_at | TIMESTAMP | Waktu penerimaan |

Constraint:

- Unique `(notification_id, user_id)` untuk setiap penerimaan.
- FK menggunakan kebijakan penghapusan yang aman.
- Indeks `(user_id, created_at)`.
- Indeks `(user_id, read_at)`.
- Indeks `(notification_id)`.
- Indeks `(rt_id, user_id)` jika diperlukan oleh pola query.

Apabila satu notifikasi memiliki beberapa penerima, kontennya tidak perlu digandakan.

Status baca tetap terpisah untuk masing-masing pengguna.

### 6.3 Strategi Penerima

**Personal Notification**

Backend membuat satu record notifikasi dan satu atau lebih record penerima sesuai target.

**Public RT Announcement**

Backend membuat satu record notifikasi dengan `audience_type = public_rt`.

Kemudian membuat record penerima untuk seluruh akun pengguna yang memiliki keanggotaan aktif pada RT tujuan ketika pengumuman dipublikasikan.

Dengan strategi ini:
- Riwayat setiap pengguna dapat dipisahkan.
- Status baca tidak saling memengaruhi.
- Jumlah belum dibaca mudah dihitung.
- Pengumuman tidak perlu digandakan sebagai konten utama.

Pengguna yang baru bergabung setelah pengumuman diterbitkan tidak otomatis memperoleh notifikasi historis sebelum tanggal keanggotaannya.

### 6.4 Kebijakan Membership

Setiap pembacaan riwayat harus mempertimbangkan scope penerima.

- Personal notification: hanya penerima yang dapat membaca.
- Public RT announcement: penerima harus memiliki relasi penerimaan yang valid dan scope RT yang sesuai.
- Keanggotaan RT yang dinonaktifkan tidak boleh membuka akses ke arsip pengumuman RT tersebut.
- Jika pengguna kembali memiliki membership yang valid, riwayat yang masih tersimpan dapat diakses kembali sesuai kebijakan akses.

Perubahan membership tidak boleh mengubah kepemilikan notifikasi pribadi.

## 7. API Design

Endpoint berikut merupakan rancangan konseptual. Ikuti pola routing, response envelope, autentikasi, dan error handling backend existing.

### 7.1 List Notification History

`GET /api/notifications`

Query parameters:

- `status=all|unread|read`
- `category=iuran|kas|announcement|system`
- `page=1`
- `limit=20`

Contoh response:

```json
{
  "data": [
    {
      "id": "notification-uuid",
      "title": "Pembayaran Iuran Berhasil",
      "body": "Pembayaran Oktober telah diverifikasi.",
      "category": "iuran",
      "audience_type": "personal",
      "read_at": null,
      "created_at": "2026-10-30T10:30:00+07:00",
      "deep_link": "/iuran/riwayat"
    }
  ],
  "meta": {
    "page": 1,
    "limit": 20,
    "total": 1,
    "unread_count": 1
  }
}
```

`unread_count` adalah jumlah seluruh notifikasi belum dibaca yang dapat diakses pengguna, bukan hanya jumlah pada halaman aktif.

### 7.2 Notification Detail

`GET /api/notifications/{id}`

Backend wajib memvalidasi:
- Autentikasi pengguna.
- Kepemilikan sebagai penerima.
- Scope RT untuk pengumuman umum.

Membuka detail dapat menandai notifikasi sebagai sudah dibaca.

### 7.3 Unread Count

`GET /api/notifications/unread-count`

Contoh:

```json
{
  "unread_count": 3
}
```

Endpoint ini digunakan untuk badge menu Notifikasi.

### 7.4 Mark as Read

`PATCH /api/notifications/{id}/read`

Contoh:

```json
{
  "success": true
}
```

Operasi harus idempotent.

Jika notifikasi sudah dibaca, request berikutnya tetap berhasil tanpa mengubah waktu baca pertama.

### 7.5 Mark All as Read

`PATCH /api/notifications/read-all`

Hanya berlaku untuk notifikasi milik pengguna yang sedang login dan masih berada dalam scope aksesnya.

Tidak boleh memengaruhi status baca pengguna lain.

### 7.6 Authorization

Seluruh endpoint harus:
- Memerlukan autentikasi.
- Menggunakan user ID dari token/session.
- Tidak mempercayai user ID dari request untuk menentukan kepemilikan.
- Memvalidasi membership dan scope RT.
- Mengembalikan 404 atau respons aman sesuai konvensi backend untuk notifikasi yang tidak dapat diakses.
- Mencegah IDOR.

## 8. Integrasi Push Notification

Notification History dan Push Notification harus menggunakan satu sumber event yang sama.

Contoh:

```text
Payment Verified
       |
       v
Notification Service
       |
       +---- Save Notification
       |
       +---- Save Recipients
       |
       +---- Send Push Notification
                    |
                    v
               Firebase FCM
```

Aturan:

1. Penyimpanan notifikasi tidak bergantung pada keberhasilan FCM.
2. Kegagalan push tetap meninggalkan riwayat.
3. Pengiriman push ulang tidak boleh membuat duplikasi riwayat.
4. Gunakan idempotency key atau event reference sesuai arsitektur existing.
5. Status delivery push tidak boleh disamakan dengan status baca.
6. Jangan menganggap push diterima berarti pengguna telah membaca notifikasi.

Jika push gagal, gunakan mekanisme retry existing jika tersedia.

## 9. Kategori dan Event

Kategori awal yang disarankan:

| Kategori | Contoh |
|---|---|
| Iuran | Pembayaran diverifikasi, ditolak, pengingat |
| KAS | Informasi laporan atau transaksi |
| Pengumuman | Rapat, kerja bakti, kegiatan |
| Administrasi | Informasi layanan RT |
| Sistem | Informasi penting aplikasi |

Integrasi harus memanfaatkan event yang sudah tersedia.

Jangan membuat event bisnis baru hanya untuk memenuhi Notification History.

Setiap event harus menentukan:
- Judul.
- Isi.
- Kategori.
- Jenis penerima.
- Scope RT.
- Deep link jika tersedia.
- Referensi objek terkait jika relevan.

## 10. Keamanan dan Privasi

Keamanan merupakan persyaratan utama.

### 10.1 Isolasi Data

- Warga A tidak dapat membaca notifikasi pribadi warga B.
- Warga RT 021 tidak dapat membaca pengumuman RT 022.
- Pengurus RT tidak otomatis memperoleh akses ke notifikasi pribadi warga.
- Pengurus RW tidak otomatis memperoleh akses ke seluruh riwayat RT.
- Super Admin tidak memperoleh akses global ke notifikasi pribadi.

### 10.2 Data Pribadi

Isi notifikasi tidak boleh membocorkan:
- Informasi pembayaran warga lain.
- Nominal transaksi pribadi kepada penerima yang tidak berhak.
- Data administrasi pribadi.
- Token autentikasi atau informasi rahasia.

### 10.3 Deep Link

Deep link hanya berfungsi sebagai navigasi.

Backend tetap wajib melakukan pemeriksaan otorisasi ketika halaman tujuan dibuka.

Tidak boleh menggunakan deep link sebagai pengganti authorization.

## 11. Implementasi Web

Tambahkan menu Notifikasi pada navigasi yang sesuai dengan role.

Komponen:

- NotificationMenuItem.
- NotificationBadge.
- NotificationHistoryPage.
- NotificationList.
- NotificationItem.
- NotificationDetailModal atau halaman detail.
- NotificationFilter.

Persyaratan:

1. Badge menggunakan unread count dari backend.
2. Daftar menggunakan pagination.
3. Status baca diperbarui setelah detail dibuka.
4. Data diperbarui setelah aksi membaca.
5. Tampilan mengikuti pola desain Social Finance Urena.
6. Empty state dan loading state tersedia.
7. Error state menyediakan opsi memuat ulang.

## 12. Implementasi Android

Tambahkan menu Notifikasi pada navigasi utama.

Komponen konseptual:

- NotificationScreen.
- NotificationListItem.
- NotificationDetailScreen.
- NotificationBadge.
- NotificationRepository.
- NotificationService.

Persyaratan:

1. Menggunakan API backend yang sama.
2. Status baca tersinkronisasi antarperangkat.
3. Mendukung pull-to-refresh.
4. Mendukung pagination atau infinite scrolling.
5. Mendukung deep link internal.
6. Menampilkan loading, empty, dan error state.
7. Badge mengikuti unread count backend.
8. Setelah pengguna membuka notifikasi, status baca diperbarui di server.

## 13. Performance dan Skalabilitas

Untuk menghindari query berat:

- Gunakan pagination berbasis database.
- Hindari mengambil seluruh riwayat sekaligus.
- Hitung unread count menggunakan query terindeks.
- Gunakan batch insert untuk pengumuman umum.
- Hindari N+1 query.
- Batasi panjang judul dan isi sesuai kebutuhan.
- Gunakan timestamp server sebagai acuan.

MVP menggunakan pagination biasa. Cursor pagination dapat dipertimbangkan apabila volume notifikasi meningkat.

## 14. Kebijakan Retensi

Keputusan MVP:

- Tidak ada penghapusan otomatis.
- Riwayat tetap tersedia setelah logout.
- Riwayat tetap tersedia setelah instal ulang aplikasi.
- Menghapus notifikasi dari tray perangkat tidak menghapus riwayat server.
- Tidak menyediakan tombol hapus riwayat untuk pengguna pada MVP.

Penghapusan atau retensi administratif harus mempertimbangkan kebutuhan audit dan kebijakan perlindungan data.

## 15. Database Migration

Migration baru hanya dibuat setelah memeriksa tabel notifikasi existing.

Ketentuan:

1. Jangan menduplikasi tabel yang sudah memiliki fungsi serupa.
2. Jangan menghapus data notifikasi existing.
3. Pertahankan kompatibilitas dengan push notification existing.
4. Tambahkan indeks sesuai kebutuhan query.
5. FK harus sesuai pola integritas database.
6. Migration harus aman terhadap data existing.
7. Pastikan rollback tidak menghilangkan data bisnis atau notifikasi existing secara tidak sengaja.

Nomor migration mengikuti nomor terakhir repository ketika implementasi dimulai.

## 16. Testing dan Acceptance Criteria

### Backend

- [ ] Personal notification hanya dapat dibaca penerima.
- [ ] Public RT announcement diterima seluruh akun aktif pada RT tujuan.
- [ ] Pengumuman tidak bocor ke RT lain.
- [ ] Status baca setiap penerima independen.
- [ ] Unread count akurat.
- [ ] Mark as read idempotent.
- [ ] Mark all as read hanya memengaruhi akun sendiri.
- [ ] Pagination berjalan benar.
- [ ] Pengguna tidak dapat mengakses notification ID milik orang lain.
- [ ] Membership yang tidak valid tidak dapat membuka pengumuman RT.
- [ ] Pengguna multi-RT hanya mendapatkan pengumuman dari RT yang sesuai.
- [ ] Pengiriman push gagal tidak menghilangkan riwayat.
- [ ] Retry push tidak menggandakan riwayat.
- [ ] Deep link tidak melewati authorization.

### Web

- [ ] Semua role yang berhak melihat menu Notifikasi.
- [ ] Badge unread count tampil dengan benar.
- [ ] Daftar notifikasi tampil.
- [ ] Filter semua/belum dibaca/sudah dibaca bekerja.
- [ ] Detail dapat dibuka.
- [ ] Status baca tersinkronisasi.
- [ ] Empty, loading, dan error state bekerja.
- [ ] Pengguna tidak dapat mengakses notifikasi akun lain.

### Android

- [ ] Menu Notifikasi tersedia.
- [ ] Badge unread count benar.
- [ ] Daftar notifikasi tampil.
- [ ] Pull-to-refresh bekerja.
- [ ] Pagination bekerja.
- [ ] Detail notifikasi bekerja.
- [ ] Status baca tersinkronisasi.
- [ ] Deep link bekerja.
- [ ] Empty, loading, dan error state bekerja.
- [ ] Riwayat tetap tersedia setelah logout dan login kembali.

## 17. Urutan Implementasi

### Phase 0 — Audit Existing Notification System

- Periksa tabel dan model notification.
- Periksa Notification Service.
- Periksa event yang menghasilkan notifikasi.
- Periksa FCM integration.
- Periksa endpoint existing.
- Periksa Web dan Android notification handling.
- Identifikasi fungsi yang bisa digunakan kembali.

**Output:** laporan audit dan gap analysis.

### Phase 1 — Backend

- Lengkapi persistence riwayat.
- Lengkapi relasi penerima.
- Implementasi read/unread.
- Implementasi unread count.
- Implementasi endpoint.
- Implementasi authorization.
- Implementasi pengumuman umum RT.
- Tambahkan test.

### Phase 2 — Web

- Menu Notifikasi.
- Badge.
- Notification History.
- Detail.
- Filter.
- Pagination.

### Phase 3 — Android

- Menu Notifikasi.
- Badge.
- Riwayat.
- Detail.
- Status baca.
- Deep link.
- Pull-to-refresh.

### Phase 4 — Integration Testing

- Uji personal notification.
- Uji public RT announcement.
- Uji multi-user.
- Uji multi-RT.
- Uji lintas perangkat.
- Uji push gagal.
- Uji keamanan IDOR.
- Uji regresi seluruh fitur existing.

## 18. Batasan MVP

Fitur yang tidak termasuk implementasi awal:

- Mengirim pengumuman langsung dari menu Notification History.
- Chat atau percakapan antarwarga.
- Komentar pengumuman.
- Reaksi atau likes.
- Penghapusan massal.
- Pencarian teks lanjutan.
- Kategori custom.
- Penjadwalan pengumuman.
- Statistik engagement.

Fokus MVP adalah menyimpan, menampilkan, membaca, dan mengamankan riwayat notifikasi.

## 19. Definition of Done

Notification History dinyatakan selesai apabila:

1. Seluruh pengguna yang berhak dapat mengakses menu Notifikasi.
2. Notifikasi pribadi dan pengumuman umum RT tersimpan di backend.
3. Pengguna hanya dapat membaca notifikasi yang menjadi haknya.
4. Status baca berjalan secara independen.
5. Badge unread count akurat.
6. Riwayat tersinkronisasi antara Web dan Android.
7. Riwayat tidak bergantung pada push notification perangkat.
8. Tidak ada kebocoran data lintas pengguna atau RT.
9. Seluruh test backend, Web, dan Android yang relevan lulus.
10. Tidak ada regresi pada notification flow existing.

---

## 20. Instruksi Implementasi

Dokumen ini menjadi acuan utama pengembangan Notification History Social Finance Urena.

**PENTING:**

- Tahap desain ini tidak mengizinkan implementasi otomatis.
- Implementasi dimulai setelah dokumen diperiksa dan disetujui.
- Jangan membuat ulang sistem notifikasi yang sudah ada.
- Jangan mengubah business flow pembayaran Iuran, KAS, atau fitur lainnya.
- Pertahankan seluruh permission dan RBAC existing.
- Semua data harus mengikuti isolasi tenant RT.
- Backend merupakan sumber kebenaran untuk riwayat dan status baca.
- Web dan Android harus menggunakan kontrak API yang konsisten.

**Final Design Status: READY FOR IMPLEMENTATION.**

### Addendum — Role-Based Notification Menu

**Keputusan final:**

1. Menu Notifikasi milik Ketua RT tidak boleh diubah. Pertahankan tiga tab existing:
   - Sistem
   - Template
   - Riwayat

2. Untuk Warga, sediakan menu Notifikasi tersendiri yang hanya menggunakan fungsi Riwayat dari sistem notifikasi existing.

3. Warga langsung diarahkan ke halaman daftar Riwayat Notifikasi ketika membuka menu Notifikasi.

4. Tab Sistem dan Template tidak boleh ditampilkan atau dapat diakses oleh Warga.

5. Gunakan Notification Service, database, API, dan mekanisme penyimpanan riwayat yang sama jika kompatibel.

6. Jangan menduplikasi sistem penyimpanan notifikasi.

7. Jangan mengubah fungsi pengelolaan Sistem dan Template milik Ketua RT.

8. Hak akses Riwayat tetap mengikuti aturan personal notification, public RT announcement, membership, dan status baca per pengguna.

**Acceptance Criteria tambahan:**
- [ ] Menu Notifikasi Ketua RT tetap memiliki tiga tab existing.
- [ ] Seluruh fungsi Sistem tidak mengalami regresi.
- [ ] Seluruh fungsi Template tidak mengalami regresi.
- [ ] Riwayat Ketua RT tetap berfungsi.
- [ ] Warga memiliki menu Notifikasi.
- [ ] Warga langsung melihat Riwayat.
- [ ] Warga tidak dapat mengakses Sistem maupun Template.
- [ ] Riwayat warga menggunakan data penerima dan status baca milik akun masing-masing.