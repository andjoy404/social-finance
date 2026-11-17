# Desain Notifikasi Tunggakan Iuran dan Global Warning Card

**Status:** Approved for implementation planning  
**Platform:** Backend Go, Web, Android Flutter  
**Modul:** Iuran, Notification Scheduler, Notification Management  
**Repository:** Social Finance Urena  
**Branch:** `urena`

---

## 1. Latar Belakang

Sistem Social Finance Urena telah memiliki Notification Scheduler generik yang dapat menjalankan pengiriman notifikasi berdasarkan jadwal dan event tertentu.

Diperlukan pengembangan lanjutan untuk membantu warga mengetahui kewajiban iuran, khususnya:

1. Mengingatkan warga sebelum tagihan mencapai tanggal jatuh tempo.
2. Mengingatkan warga yang memiliki tunggakan selama 3 bulan atau lebih.
3. Menampilkan peringatan tunggakan secara konsisten selama warga menggunakan aplikasi, bukan hanya melalui push notification.

Pengembangan ini harus memanfaatkan mekanisme Notification Scheduler dan Template Sistem yang sudah ada. Tidak boleh membuat scheduler terpisah atau mekanisme perhitungan tunggakan kedua yang berbeda dari backend.

## 2. Tujuan

Fitur ini memiliki dua bagian utama:

### A. Automatic Iuran Reminder

Mengirim notifikasi otomatis kepada warga berdasarkan kondisi tagihan dan tanggal jatuh tempo.

### B. Global Outstanding Dues Warning Card

Menampilkan card peringatan pada seluruh menu aplikasi Web dan Android apabila akun warga memiliki tagihan yang menunggak minimal 3 bulan.

Kedua fitur harus menggunakan sumber data dan aturan perhitungan tunggakan yang sama.

---

# BAGIAN A — AUTOMATIC IURAN REMINDER

## 3. Jenis Notifikasi Sistem

Diperlukan dua event sistem baru.

| Event | Keterangan |
|---|---|
| `iuran_upcoming_due_3_months` | Pengingat ketika tagihan memasuki periode 3 bulan sebelum jatuh tempo |
| `iuran_overdue_3_months` | Pengingat ketika tagihan telah menunggak minimal 3 bulan |

Event tersebut harus terdaftar sebagai event sistem dan dapat dihubungkan dengan Template Sistem melalui Notification Scheduler.

Nama event merupakan usulan desain. Sebelum implementasi, periksa konvensi penamaan dan registrasi event yang sudah digunakan oleh source code.

## 4. Template Sistem

Kedua event harus menggunakan Template Sistem yang dapat dikelola oleh pengurus yang memiliki permission `notification.manage`.

Ketentuan:

1. Template dapat diedit melalui menu Notifikasi → Template.
2. Pengurus dapat mengaktifkan atau menonaktifkan template sesuai mekanisme yang tersedia.
3. Scheduler hanya mengirim jika schedule aktif dan template terkait aktif.
4. Perubahan isi template tidak membutuhkan perubahan source code.
5. Template yang sudah ada tidak boleh dihapus atau diubah secara tidak sengaja.
6. Template baru harus mengikuti mekanisme pengelolaan template sistem yang sudah berlaku.

Contoh isi template:

**A. Pengingat sebelum jatuh tempo**

Judul:
`Pengingat Pembayaran Iuran`

Pesan:
`Halo {{nama}}, kami mengingatkan bahwa tagihan iuran Anda akan jatuh tempo dalam waktu 3 bulan. Silakan periksa tagihan Anda dan lakukan pembayaran sesuai ketentuan. Terima kasih.`

**B. Pengingat tunggakan minimal 3 bulan**

Judul:
`Pengingat Tunggakan Iuran`

Pesan:
`Halo {{nama}}, terdapat tagihan iuran Anda yang telah menunggak selama 3 bulan atau lebih. Mohon segera lakukan pembayaran untuk menyelesaikan kewajiban iuran Anda. Terima kasih.`

Placeholder harus mengikuti sistem template yang benar-benar tersedia. Jangan mengasumsikan placeholder baru tanpa memeriksa implementasi template engine.

## 5. Aturan Pengingat Sebelum Jatuh Tempo

Pengingat dikirim ketika tagihan memasuki periode 3 bulan sebelum tanggal jatuh tempo.

Ketentuan:

- Perhitungan menggunakan tanggal jatuh tempo aktual.
- Menggunakan perhitungan bulan kalender, bukan asumsi 90 hari.
- Tagihan harus merupakan kewajiban iuran (`is_obligatory = true`).
- Tagihan harus masih memiliki sisa kewajiban.
- Tagihan tidak boleh sudah lunas.
- Pengingat dikirim satu kali untuk setiap tagihan pada saat memasuki ambang tersebut.
- Jika scheduler terlambat berjalan, mekanisme catch-up harus tetap dapat menemukan pengingat yang belum dikirim tanpa mengirim duplikat.

Contoh:

Tanggal jatuh tempo: 15 Oktober 2026.

Tanggal pengingat: 15 Juli 2026.

## 6. Aturan Pengingat Tunggakan Minimal 3 Bulan

Tagihan masuk kategori tunggakan apabila belum lunas dan telah melewati tanggal jatuh tempo sekurang-kurangnya 3 bulan kalender.

Contoh:

| Tanggal jatuh tempo | Tanggal evaluasi | Status |
|---|---|---|
| 15 Juli 2026 | 14 Oktober 2026 | Belum mencapai 3 bulan |
| 15 Juli 2026 | 15 Oktober 2026 | Mencapai 3 bulan |
| 15 Juli 2026 | 20 Oktober 2026 | Minimal 3 bulan |

Ketentuan pengiriman:

1. Notifikasi pertama dikirim ketika tagihan pertama kali mencapai ambang 3 bulan.
2. Jika masih menunggak, pengingat berikutnya dikirim setiap bulan.
3. Pengiriman berhenti ketika kewajiban sudah lunas atau tidak lagi memenuhi kriteria.
4. Pengiriman harus mempertimbangkan timezone `Asia/Jakarta`.
5. Scheduler harus menggunakan mekanisme catch-up yang sudah tersedia.
6. Jangan mengirim notifikasi berulang pada satu periode pengingat yang sama.

## 7. Kriteria Penerima

Penerima notifikasi harus memenuhi ketentuan berikut:

- Akun merupakan warga yang berhak menerima pengingat iuran.
- Memiliki keanggotaan RT yang aktif.
- Tagihan merupakan tagihan wajib.
- Tagihan belum lunas.
- Masih memiliki sisa kewajiban.
- Memenuhi kondisi tanggal jatuh tempo sesuai jenis event.
- Memenuhi aturan masa mulai kewajiban atau tanggal mulai tinggal (`move_in_date`).

Tagihan sukarela atau non-obligatory tidak termasuk dalam pengingat ini.

Tagihan dari periode sebelum warga mulai memiliki kewajiban iuran tidak boleh dihitung sebagai tunggakan.

## 8. Perhitungan Sisa Kewajiban

Perhitungan harus menggunakan mekanisme perhitungan saldo tagihan yang sudah ada.

Prinsip:

`Sisa kewajiban = Nilai tagihan - Total pembayaran yang telah disetujui`

Ketentuan:

- Pembayaran berstatus APPROVED mengurangi sisa kewajiban.
- Pembayaran PENDING tidak mengurangi sisa kewajiban.
- Pembayaran REJECTED tidak mengurangi sisa kewajiban.
- Pembayaran parsial harus diperhitungkan.
- Jika sisa kewajiban sudah nol, tagihan tidak lagi menjadi penerima pengingat.

Jangan membuat formula baru apabila repository sudah memiliki perhitungan outstanding balance yang dapat digunakan kembali.

## 9. Pencegahan Notifikasi Duplikat

Setiap pengiriman harus memiliki identitas deduplikasi yang stabil.

Identitas deduplikasi minimal mempertimbangkan:

- Schedule ID.
- Bill ID.
- User ID.
- Event type.
- Periode pengiriman.

Untuk pengingat tunggakan bulanan, satu tagihan tidak boleh menghasilkan notifikasi berulang pada periode yang sama.

Jika warga memiliki beberapa tagihan yang memenuhi kondisi, notifikasi sebaiknya diringkas agar tidak membanjiri warga dengan banyak push notification.

Notifikasi harus tetap dapat mengarahkan warga ke halaman Iuran atau detail tagihan yang relevan.

---

# BAGIAN B — GLOBAL OUTSTANDING DUES WARNING CARD

## 10. Tujuan Warning Card

Warning Card berfungsi sebagai pengingat visual yang selalu terlihat ketika warga menggunakan aplikasi.

Card bukan pengganti push notification. Keduanya saling melengkapi:

- Push notification mengingatkan warga ketika tidak sedang membuka aplikasi.
- Warning Card mengingatkan warga ketika sedang menggunakan aplikasi.
- Halaman Iuran Saya menyediakan detail tagihan dan tindakan pembayaran.

## 11. Posisi Warning Card

### A. Android

Warning Card ditempatkan:

1. Di bawah AppBar/Header.
2. Di atas konten utama halaman.
3. Sebelum konten masing-masing menu.
4. Tidak menutupi Bottom Navigation maupun elemen interaktif lainnya.

### B. Web

Warning Card ditempatkan:

1. Di area konten utama.
2. Tepat di bawah Header halaman.
3. Sebelum konten utama menu.
4. Tidak menutupi Sidebar atau navigasi lainnya.

### C. Global Component

Warning Card harus diimplementasikan sebagai komponen bersama pada layout utama aplikasi.

Jangan menambahkan implementasi card secara terpisah pada setiap halaman.

Tujuannya agar:

- Tampilan konsisten.
- Logika visibilitas konsisten.
- Tidak terjadi duplikasi kode.
- Perubahan desain dapat dilakukan pada satu komponen.
- Status tunggakan dapat digunakan bersama oleh semua menu.

## 12. Desain Visual

Judul:

`Peringatan Tunggakan Iuran`

Pesan:

`Anda memiliki tagihan iuran yang belum lunas selama 3 bulan atau lebih. Mohon segera lakukan pembayaran agar kewajiban iuran tidak semakin menumpuk.`

Informasi yang ditampilkan:

- Total sisa tunggakan.
- Jumlah tagihan yang memenuhi kriteria.
- Tombol atau tautan `Lihat Tagihan Saya`.

Elemen visual:

- Latar kuning muda atau amber.
- Ikon peringatan (`alert-triangle`).
- Teks berwarna gelap dengan kontras yang baik.
- Badge `Tunggakan 3 Bulan+`.
- Sudut card mengikuti design system aplikasi.
- Tampilan responsif untuk ukuran layar Android dan Web.

Jangan menggunakan warna merah secara berlebihan. Warna amber/oranye menjadi warna utama untuk peringatan tunggakan.

### Ilustrasi

```text
┌──────────────────────────────────────────┐
│ ⚠ Peringatan Tunggakan Iuran             │
│                            [3 Bulan+]    │
│                                          │
│ Anda memiliki tagihan iuran yang belum   │
│ lunas selama 3 bulan atau lebih.         │
│                                          │
│ Total sisa tunggakan                     │
│ Rp450.000                                │
│                                          │
│ 3 tagihan                                │
│                                          │
│ Lihat Tagihan Saya →                     │
└──────────────────────────────────────────┘
```

Angka di atas hanya ilustrasi.

## 13. Aturan Visibilitas Card

Card hanya ditampilkan apabila akun warga memiliki minimal satu tagihan yang memenuhi kriteria tunggakan minimal 3 bulan.

| Kondisi | Warning Card |
|---|---|
| Tidak memiliki tunggakan | Tidak tampil |
| Memiliki tagihan yang belum jatuh tempo | Tidak tampil |
| Tunggakan kurang dari 3 bulan | Tidak tampil |
| Tunggakan minimal 3 bulan | Tampil |
| Sudah melunasi seluruh tagihan | Hilang |
| Pembayaran diajukan tetapi belum diverifikasi | Tetap tampil jika masih memenuhi perhitungan sisa kewajiban |
| Tunggakan hanya berasal dari tagihan non-obligatory | Tidak tampil |

Ambang batas menggunakan 3 bulan kalender setelah tanggal jatuh tempo.

Card tidak boleh dapat disembunyikan secara permanen oleh warga selama kondisi tunggakan masih terpenuhi.

## 14. Hak Akses dan Batasan Pengguna

Warning Card ini khusus untuk akun warga.

Ketentuan:

- Akun warga dengan tunggakan memenuhi syarat dapat melihat card.
- Akun warga tanpa tunggakan tidak melihat card.
- Ketua RT tidak otomatis melihat card ini sebagai warga.
- Bendahara, pengurus, dan Super Admin tidak otomatis melihat card ini.
- Jangan mengubah RBAC atau permission pengurus untuk mengimplementasikan fitur ini.

Jika akun memiliki beberapa jabatan atau keanggotaan, penentuan apakah akun termasuk akun warga harus mengikuti aturan autentikasi dan identifikasi pengguna yang sudah ada. Jangan menggunakan pemeriksaan role sederhana yang dapat salah mengklasifikasikan pengurus sebagai warga.

## 15. Backend Summary API

Backend perlu menyediakan ringkasan tunggakan untuk akun yang sedang login.

Nama endpoint dan bentuk response ditentukan setelah audit terhadap konvensi API yang sudah ada.

Data minimal yang dibutuhkan:

```json
{
  "has_overdue_3_months": true,
  "total_outstanding": 450000,
  "overdue_bill_count": 3
}
```

Response tersebut hanya contoh kontrak konseptual.

Ketentuan keamanan:

1. Identitas pengguna harus berasal dari JWT atau AuthContext.
2. Client tidak boleh menentukan `user_id` pemilik tagihan secara bebas.
3. Backend hanya menghitung tagihan milik pengguna yang sedang login.
4. Jangan mengembalikan informasi tunggakan warga lain.
5. Query harus memperhitungkan RT dan keanggotaan yang relevan.
6. Jika terjadi error, jangan menampilkan informasi tunggakan yang tidak dapat diverifikasi.
7. Endpoint harus efisien dan tidak melakukan query berulang untuk setiap menu.

## 16. Integrasi Web dan Android

### Web

Gunakan global layout atau shell utama yang sudah digunakan oleh halaman-halaman aplikasi.

Warning Card harus:

- Muncul di semua menu warga.
- Tidak muncul pada halaman login.
- Tidak muncul pada halaman yang tidak menggunakan shell warga.
- Menggunakan state atau provider yang sesuai dengan arsitektur frontend saat ini.
- Tidak memicu request summary berulang setiap kali komponen halaman berubah.

### Android

Gunakan ShellRoute, root scaffold, atau struktur global yang benar-benar digunakan aplikasi.

Warning Card harus:

- Muncul pada seluruh menu warga.
- Tidak muncul pada halaman login.
- Tidak muncul pada menu atau konteks pengurus.
- Mengikuti layout responsif.
- Tidak menutupi Bottom Navigation.
- Tidak menyebabkan duplikasi pemanggilan API saat berpindah menu.

## 17. Refresh dan Sinkronisasi Data

Status Warning Card perlu diperbarui:

- Saat warga membuka aplikasi atau login.
- Saat berpindah kembali ke aplikasi setelah berada di background, sesuai mekanisme lifecycle yang tersedia.
- Setelah pembayaran berhasil diverifikasi.
- Setelah perubahan tagihan yang memengaruhi saldo kewajiban.
- Saat melakukan refresh data Iuran.

Jangan hanya mengandalkan cache lokal yang dapat menyebabkan card tetap muncul setelah tagihan lunas.

Jika pembayaran masih pending, card tetap ditampilkan selama outstanding balance yang disetujui masih memenuhi kriteria.

## 18. Hubungan dengan Notification Scheduler

Notification Scheduler dan Warning Card harus menggunakan satu sumber kebenaran untuk menentukan tunggakan.

```text
                 BACKEND
                    |
          Outstanding Dues Logic
                    |
          +---------+---------+
          |                   |
          v                   v
 Notification Scheduler   Summary API
          |                   |
          v                   v
    Push Notification    Global Warning Card
                         /             \
                        v               v
                       Web           Android
```

Jangan membuat:

- Perhitungan tunggakan khusus Web.
- Perhitungan tunggakan khusus Android.
- Perhitungan tunggakan terpisah di Scheduler.
- Query tunggakan yang berbeda untuk setiap menu.

Jika diperlukan, ekstrak atau gunakan service/repository yang sudah ada agar seluruh fitur menggunakan aturan yang konsisten.

---

# BAGIAN C — IMPLEMENTATION PLAN

## 19. Tahapan Implementasi

### Tahap 1 — Read-only Audit

Sebelum mengubah source code, lakukan audit terhadap:

**Backend**
- Struktur tabel tagihan.
- Kolom tanggal jatuh tempo yang benar-benar digunakan.
- Perhitungan outstanding balance.
- Status pembayaran dan payment allocation.
- Aturan `is_obligatory`.
- Aturan `move_in_date` atau tanggal mulai kewajiban.
- Notification Scheduler.
- Template engine dan placeholder.
- Event registry.
- Deduplication key.
- Deep link notifikasi.

**Web**
- Struktur layout utama.
- Header dan Sidebar.
- State management dan API service.
- Mekanisme refresh data.
- Penentuan konteks akun warga.

**Android**
- Struktur ShellRoute dan root layout.
- AppBar dan Bottom Navigation.
- State management.
- API repository.
- Mekanisme refresh data.
- Penentuan konteks akun warga.

Hasil audit harus berupa laporan yang menunjukkan file, fungsi, tabel, endpoint, dan mekanisme aktual yang relevan.

### Tahap 2 — Backend Shared Logic

Implementasikan atau gunakan kembali satu sumber perhitungan tunggakan.

Pastikan aturan tanggal, pembayaran, tagihan wajib, move-in, dan sisa kewajiban benar.

### Tahap 3 — Notification Scheduler

Tambahkan dua event baru dan Template Sistem yang dapat diedit.

Gunakan scheduler generik yang sudah ada.

Pastikan deduplikasi dan mekanisme catch-up berjalan sesuai aturan.

### Tahap 4 — Summary API

Sediakan API untuk mengambil ringkasan tunggakan milik pengguna yang sedang login.

Pastikan tidak terjadi kebocoran data lintas warga atau lintas RT.

### Tahap 5 — Global Warning Card

Implementasikan komponen global pada Web dan Android.

Tempatkan di bawah Header/AppBar dan di atas konten utama.

### Tahap 6 — Testing

Lakukan pengujian terarah terhadap seluruh skenario.

---

# BAGIAN D — ACCEPTANCE CRITERIA

## 20. Backend

- [ ] Penghitungan tanggal jatuh tempo menggunakan kolom aktual.
- [ ] Perhitungan 3 bulan menggunakan bulan kalender.
- [ ] Hanya tagihan wajib yang diperhitungkan.
- [ ] Pembayaran APPROVED mengurangi outstanding balance.
- [ ] Pembayaran PENDING tidak mengurangi outstanding balance.
- [ ] Pembayaran REJECTED tidak mengurangi outstanding balance.
- [ ] Pembayaran parsial dihitung dengan benar.
- [ ] Move-in date dihormati.
- [ ] Tagihan lunas tidak masuk pengingat.
- [ ] API hanya mengembalikan ringkasan milik akun yang login.
- [ ] Tidak ada kebocoran data antarwarga atau antar-RT.

## 21. Notification Scheduler

- [ ] Event pengingat sebelum jatuh tempo tersedia.
- [ ] Event tunggakan minimal 3 bulan tersedia.
- [ ] Template kedua event dapat diedit.
- [ ] Template dan schedule harus aktif untuk mengirim.
- [ ] Pengingat sebelum jatuh tempo dikirim satu kali pada ambang yang ditentukan.
- [ ] Pengingat tunggakan pertama dikirim saat mencapai 3 bulan.
- [ ] Pengingat berikutnya dikirim setiap bulan selama masih memenuhi syarat.
- [ ] Tidak ada pengiriman duplikat dalam periode yang sama.
- [ ] Notifikasi berhenti setelah tagihan lunas atau tidak lagi memenuhi syarat.
- [ ] Deep link mengarah ke halaman Iuran atau detail tagihan yang benar.
- [ ] Scheduler menggunakan `Asia/Jakarta`.

## 22. Web

- [ ] Warning Card tampil pada semua menu akun warga.
- [ ] Posisi tepat di bawah Header.
- [ ] Card tidak tampil jika warga tidak memiliki tunggakan yang memenuhi syarat.
- [ ] Card hilang setelah tunggakan tidak lagi memenuhi syarat.
- [ ] Total sisa tunggakan benar.
- [ ] Jumlah tagihan benar.
- [ ] Tombol mengarah ke Iuran Saya.
- [ ] Tidak muncul pada akun pengurus dan Super Admin.
- [ ] Tidak ada request berulang yang tidak diperlukan.

## 23. Android

- [ ] Warning Card tampil pada semua menu akun warga.
- [ ] Posisi tepat di bawah AppBar.
- [ ] Tidak menutupi Bottom Navigation.
- [ ] Tidak tampil pada akun tanpa tunggakan yang memenuhi syarat.
- [ ] Hilang setelah tunggakan tidak lagi memenuhi syarat.
- [ ] Total sisa tunggakan benar.
- [ ] Jumlah tagihan benar.
- [ ] Tombol mengarah ke Iuran Saya.
- [ ] Tidak muncul pada akun pengurus dan Super Admin.
- [ ] Tidak ada request berulang yang tidak diperlukan.
- [ ] Tampilan baik pada layar Android kecil.

## 24. Skenario Pengujian

| Skenario | Hasil yang diharapkan |
|---|---|
| Warga tidak memiliki tagihan | Tidak ada card |
| Tagihan belum jatuh tempo | Tidak ada card |
| Tagihan menunggak kurang dari 3 bulan | Tidak ada card |
| Tagihan mencapai 3 bulan | Card tampil |
| Tagihan menunggak lebih dari 3 bulan | Card tetap tampil |
| Ada beberapa tagihan yang memenuhi syarat | Total dan jumlah tagihan terakumulasi dengan benar |
| Pembayaran parsial disetujui | Outstanding balance berkurang |
| Pembayaran pending | Outstanding balance belum berkurang |
| Pembayaran ditolak | Outstanding balance tidak berkurang |
| Seluruh tagihan dilunasi | Card hilang |
| Ada tagihan non-obligatory | Tidak masuk perhitungan |
| Warga baru dengan move-in date | Tidak menghitung periode sebelum kewajiban dimulai |
| Pengurus login | Tidak melihat card warga |
| Warga berpindah menu | Card tetap tampil jika masih memenuhi syarat |

---

# BAGIAN E — BATASAN IMPLEMENTASI

## 25. Hal yang Tidak Boleh Dilakukan

- Jangan membuat scheduler baru.
- Jangan membuat perhitungan tunggakan terpisah di frontend.
- Jangan mengubah aturan pembayaran yang sudah berjalan.
- Jangan mengubah status pembayaran otomatis hanya karena notifikasi dikirim.
- Jangan mengubah permission atau RBAC pengurus.
- Jangan memberikan akses ke tagihan warga lain.
- Jangan menghapus atau mengubah template lama tanpa kebutuhan yang terverifikasi.
- Jangan membuat migrasi sebelum audit membuktikan migrasi memang diperlukan.
- Jangan melakukan refactor besar di luar ruang lingkup.
- Jangan mengubah data produksi.
- Jangan melakukan deploy atau migrasi server tanpa persetujuan terpisah.

## 26. Ketentuan Git dan Working Tree

Sebelum implementasi:

1. Periksa branch dan HEAD terkini.
2. Periksa seluruh modified dan untracked files.
3. Jangan menghapus atau menimpa perubahan yang sudah ada.
4. Jangan menggunakan `git reset --hard`, `git clean`, atau perintah destruktif lainnya.
5. Jangan melakukan commit atau push tanpa otorisasi eksplisit.

Perubahan harus tetap kompatibel dengan implementasi Notification Scheduler yang sudah ada.

## 27. Definition of Done

Fitur dianggap selesai apabila:

1. Kedua jenis pengingat berjalan menggunakan Notification Scheduler generik.
2. Template Sistem dapat diedit oleh pengurus yang berwenang.
3. Perhitungan tunggakan konsisten antara Scheduler dan Summary API.
4. Warning Card muncul di seluruh menu warga pada Web dan Android.
5. Warning Card hanya muncul untuk akun yang memenuhi kriteria.
6. Data ringkasan benar dan aman.
7. Semua acceptance criteria penting telah diuji.
8. Tidak ada regresi pada fitur Iuran, pembayaran, notifikasi, maupun RBAC yang sudah berjalan.

---

**Prinsip utama desain:**

Satu sumber perhitungan tunggakan di backend, satu mekanisme scheduler untuk push notification, dan satu komponen global Warning Card untuk Web serta Android.

Dengan pendekatan ini, warga menerima pengingat secara konsisten, sedangkan pengurus dapat mengelola isi notifikasi melalui Template Sistem tanpa perlu mengubah kode aplikasi.