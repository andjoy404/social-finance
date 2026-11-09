# Spesifikasi Beranda Baru, Kegiatan, dan Pengumuman

**Proyek:** Social Finance Urena  
**Status:** Draft spesifikasi kebutuhan — belum merupakan persetujuan implementasi  
**Cakupan platform:** Backend/API, Web, dan Flutter  
**Tujuan dokumen:** Menjadi acuan redesain Beranda dan pengembangan fitur Kegiatan serta Pengumuman.

---

## 1. Ringkasan dan Keputusan Utama

Beranda aplikasi yang sekarang akan diubah fungsinya menjadi pusat informasi lingkungan RT.

Konten Beranda lama yang berfokus pada informasi keuangan dipindahkan ke menu baru bernama **Keuangan**. Fungsi dan konten keuangan yang sudah ada harus dipertahankan; perubahan ini terutama menyangkut pengelompokan navigasi dan penamaan menu, bukan menghapus atau menyederhanakan fitur keuangan.

Beranda baru menampilkan:
1. Banner pengumuman.
2. Informasi petugas keamanan yang sedang aktif bertugas.
3. Struktur organisasi/pengurus RT dalam bentuk diagram hierarki.
4. Agenda kegiatan terdekat.
5. Informasi tambahan yang bersifat umum dan tidak melanggar privasi.

Fitur tambahan:
- Menu **Kegiatan** untuk mengelola dan melihat agenda RT.
- Menu **Kelola Pengumuman** untuk mengelola banner dan template pengumuman.

**Keputusan privasi eksplisit:** Beranda tidak boleh menampilkan informasi status pengangkutan sampah, daftar rumah yang sampahnya belum terangkut, jadwal pengangkutan, atau gangguan layanan pengangkutan. Informasi tersebut tidak dimasukkan ke Beranda karena pertimbangan privasi.

## 2. Tujuan UX

Beranda harus menjawab kebutuhan warga dengan cepat:
- Siapa petugas keamanan yang sedang bertugas dan bagaimana menghubunginya?
- Siapa pengurus RT dan apa jabatannya?
- Apa pengumuman penting saat ini?
- Kegiatan apa yang sedang berlangsung atau akan segera dilaksanakan?

Beranda bukan halaman untuk menampilkan seluruh statistik dan seluruh data aplikasi. Detail tetap berada di menu masing-masing.

## 3. Perubahan Navigasi

### 3.1 Menu Beranda

**Beranda** menjadi halaman informasi umum RT dengan komponen:
- Banner Pengumuman.
- Petugas Keamanan Bertugas.
- Struktur Pengurus RT.
- Agenda Terdekat.
- Opsional: kontak penting dan aktivitas/pengumuman terbaru jika sesuai kebijakan privasi.

### 3.2 Menu Keuangan

Konten Beranda lama yang terkait keuangan dipindahkan ke menu **Keuangan**, termasuk konten yang saat ini tersedia seperti:
- Ringkasan kas.
- Iuran/IPL.
- Pemasukan.
- Pengeluaran.
- Laporan atau ringkasan keuangan yang sudah ada.

Jangan menghapus fitur keuangan yang sudah berjalan. Audit implementasi yang ada terlebih dahulu dan pertahankan seluruh kemampuan, data, permission, serta alur yang sudah tersedia.

### 3.3 Menu Kegiatan

Menu baru untuk daftar dan detail kegiatan RT.

### 3.4 Menu Kelola Pengumuman

Menu pengelolaan bagi Ketua RT atau role lain yang diberi permission:
- Daftar Pengumuman.
- Template Banner.
- Pengaturan Pengumuman.

Menu pengelolaan tidak boleh tampil atau dapat diakses oleh warga tanpa permission.

## 4. Komponen Beranda

### 4.1 Banner Pengumuman

Banner menampilkan pengumuman penting dalam bentuk visual, terdiri dari:
- Gambar/banner.
- Judul.
- Keterangan singkat.
- Periode tayang jika relevan.
- Tautan/tombol tujuan jika disediakan, misalnya menuju detail kegiatan atau halaman Keuangan.

Jika terdapat beberapa banner:
- Dukung lebih dari satu pengumuman aktif.
- Sediakan prioritas/urutan.
- Tampilkan banner utama dengan jelas.
- Pertimbangkan carousel atau daftar banner sesuai pola desain aplikasi.
- Jangan membuat informasi penting sulit ditemukan hanya karena banner lain lebih baru.

Banner yang sudah diterbitkan harus memiliki konten snapshot sendiri. Perubahan template di kemudian hari tidak boleh mengubah banner yang sudah diterbitkan.

### 4.2 Petugas Keamanan yang Sedang Bertugas

Tampilkan petugas berdasarkan **jadwal dan status absensi aktual**, bukan hanya jadwal.

Informasi yang direncanakan:
- Nama petugas.
- Nomor telepon dan aksi hubungi, hanya jika viewer memiliki hak akses melihat nomor tersebut.
- Shift yang sedang berlangsung.
- Jam mulai dan selesai.
- Status aktual: Hadir, Terlambat, Belum Hadir, atau Tidak Hadir.

Aturan:
- Jika ada lebih dari satu petugas pada shift aktif, tampilkan seluruh petugas yang relevan.
- Jangan menampilkan petugas sebagai sedang bertugas/hadir jika belum ada bukti absensi yang sesuai.
- Jika belum ada petugas yang tercatat hadir, tampilkan status yang jujur dan jelas.
- Jika tidak ada shift aktif, tampilkan keadaan kosong yang informatif.
- Sumber data harus terintegrasi dengan Modul Keamanan.
- Hak akses nomor telepon wajib mengikuti aturan privasi dan backend authorization yang berlaku. Jangan mengirim nomor telepon kepada pengguna yang tidak berhak hanya untuk kemudian menyembunyikannya di UI.

### 4.3 Struktur Organisasi Pengurus RT

Struktur pengurus ditampilkan sebagai **diagram hierarki/flow diagram**, bukan tabel biasa.

Contoh konseptual:

```text
                       KETUA RT
                          |
                +---------+---------+
                |                   |
            WAKIL KETUA         SEKRETARIS
                                    |
                                BENDAHARA
                                    |
              +----------+----------+----------+
              |          |          |          |
          KEAMANAN   KEBERSIHAN   SEKSI ...   SEKSI ...
```

Diagram di atas hanya ilustrasi hubungan. Struktur final harus mengikuti model jabatan dan hubungan organisasi yang benar-benar tersedia di aplikasi. Jangan mengarang hubungan pelaporan apabila model data tidak menyediakannya.

Informasi pada setiap node:
- Jabatan.
- Nama pengurus yang menjabat.
- Foto profil jika tersedia dan diizinkan.
- Aksi kontak hanya jika viewer berhak melihat informasi kontak.

Persyaratan UI:
- Diagram dibuat dinamis dari data pengurus/keanggotaan aktif.
- Perubahan pengurus harus tercermin dari data terbaru.
- Mendukung tampilan layar HP dan Web.
- Di HP, diagram dapat diperbesar/digeser atau menggunakan pola expand/collapse per cabang agar mudah dibaca.
- Nama/jabatan yang panjang harus tetap terbaca.
- Node tanpa pengurus aktif ditampilkan sesuai kebijakan, misalnya jabatan kosong, atau tidak ditampilkan. Tentukan konsistensinya.
- Jangan membuat diagram statis yang harus diedit manual setiap ada pergantian pengurus.

### 4.4 Agenda Terdekat

Beranda menampilkan sekitar 3–5 kegiatan terdekat yang relevan, dengan tombol **Lihat Semua Kegiatan** menuju menu Kegiatan.

Informasi ringkas:
- Nama kegiatan.
- Tanggal.
- Jam.
- Lokasi atau keterangan lokasi jika tersedia.
- Status kegiatan.

Contoh:
- Kerja Bakti RT — Minggu, 07.00 WIB.
- Rapat Pengurus — tanggal dan jam sesuai jadwal.

Kegiatan yang dibatalkan atau sudah selesai tidak boleh ditampilkan sebagai agenda mendatang. Kegiatan yang sedang berlangsung dapat diberi penanda khusus.

### 4.5 Konten yang Tidak Boleh Ditampilkan

Karena pertimbangan privasi, jangan menambahkan komponen berikut ke Beranda:
- Status pengangkutan sampah hari ini.
- Daftar rumah yang sampahnya belum terangkut.
- Jadwal pengangkutan sampah berikutnya.
- Informasi gangguan layanan pengangkutan sampah.
- Informasi operasional lain yang dapat mengungkap kondisi rumah/warga secara tidak semestinya.

Jangan menganggap menu **Sampah Tidak Terangkut** pada akun Perangkat Pos Keamanan sebagai izin untuk menampilkan data tersebut pada Beranda warga.

## 5. Fitur Kegiatan

### 5.1 Tujuan

Menu **Kegiatan** menjadi pusat informasi kegiatan RT agar warga mengetahui kegiatan yang sedang berlangsung dan yang akan datang.

Contoh jenis kegiatan:
- Kerja Bakti RT.
- Rapat Warga.
- Rapat Pengurus.
- Posyandu.
- Perayaan hari besar.
- Perlombaan/acara warga.
- Kegiatan sosial.
- Kegiatan lingkungan lainnya.

Jenis kegiatan sebaiknya fleksibel dan tidak dibatasi pada daftar hardcoded yang sulit diperluas.

### 5.2 Data Kegiatan

Field yang direncanakan:

| Field | Keterangan |
|---|---|
| Judul kegiatan | Wajib |
| Deskripsi | Keterangan kegiatan |
| Gambar/poster | Opsional, jika tersedia |
| Tanggal dan waktu mulai | Wajib |
| Tanggal dan waktu selesai | Perlu validasi setelah waktu mulai |
| Lokasi | Dapat diwajibkan sesuai jenis kegiatan |
| Penyelenggara | Pengurus/kelompok penyelenggara |
| Status | Akan Datang, Sedang Berlangsung, Selesai, Dibatalkan |
| Publikasi | Draft atau diterbitkan |
| Pembuat/pengubah | Audit metadata |

Tambahkan field lain hanya jika kebutuhan bisnis dan privasi jelas.

### 5.3 Hak Akses

- Ketua RT: dapat membuat, mengubah, menerbitkan, membatalkan, dan mengelola kegiatan sesuai permission.
- Pengurus lain: dapat mengelola jika diberikan permission.
- Warga: dapat melihat daftar dan detail kegiatan yang dipublikasikan.
- Pengguna tanpa permission tidak boleh mengakses endpoint mutasi hanya dengan memanggil API secara langsung.

### 5.4 Siklus Kegiatan

Status kegiatan perlu dibedakan dari status publikasi.

Status kegiatan:
- Akan Datang.
- Sedang Berlangsung.
- Selesai.
- Dibatalkan.

Status publikasi:
- Draft.
- Diterbitkan.

Perubahan status otomatis berdasarkan waktu dapat dipertimbangkan, tetapi harus memperhatikan zona waktu RT dan tidak boleh mengubah kegiatan yang dibatalkan menjadi berlangsung/selesai secara tidak tepat.

### 5.5 Integrasi Kegiatan dan Banner

Pada form pembuatan/edit kegiatan, sediakan opsi:

**Tampilkan sebagai Banner di Beranda**

Jika diaktifkan:
- Informasi kegiatan dapat dipublikasikan sebagai banner.
- Poster/gambar kegiatan dapat digunakan sebagai gambar banner.
- Judul dan deskripsi dapat menjadi konten awal banner.
- Admin dapat melakukan penyesuaian konten banner.
- Banner memiliki periode tayang yang sesuai dengan kebutuhan kegiatan.

Kegiatan dan pengumuman tetap merupakan entitas/fungsi yang berbeda. Integrasi banner tidak boleh membuat data kegiatan dan data pengumuman tercampur sehingga sulit dikelola.

## 6. Fitur Kelola Pengumuman

### 6.1 Struktur Menu

**Kelola Pengumuman**
1. Daftar Pengumuman.
2. Template Banner.
3. Pengaturan Pengumuman.

### 6.2 Daftar Pengumuman

Kemampuan yang direncanakan:
- Membuat pengumuman.
- Mengedit pengumuman.
- Melihat preview.
- Menerbitkan sekarang.
- Menjadwalkan waktu tayang.
- Mengaktifkan/menonaktifkan.
- Mengarsipkan pengumuman.
- Mengatur prioritas/urutan.
- Mengatur target audiens jika memang diperlukan dan didukung permission.

Field pengumuman:
- Judul.
- Keterangan/deskripsi.
- Gambar/banner.
- Template sumber (opsional).
- Waktu mulai tayang.
- Waktu berakhir.
- Status draft/terjadwal/aktif/nonaktif/kedaluwarsa/arsip.
- Prioritas.
- Tautan tujuan (opsional dan harus divalidasi).
- Target audiens (jika digunakan).
- Pembuat dan waktu perubahan.

Jangan menghapus pengumuman historis secara permanen jika masih diperlukan untuk audit.

### 6.3 Template Banner

Template disediakan agar Ketua RT tidak perlu membuat desain dan struktur pengumuman dari awal setiap kali ada kebutuhan.

Contoh template bawaan:
- Pengingat IPL/Iuran.
- Kerja Bakti.
- Rapat Warga.
- Pengumuman Umum.
- Kegiatan/Event.

Template dapat berisi:
- Nama template.
- Desain/layout.
- Gambar default atau placeholder.
- Judul awal.
- Teks/keterangan awal.
- Field yang perlu diisi.
- Kategori.
- Status aktif/nonaktif.

Alur penggunaan:
1. Admin membuka Template Banner.
2. Memilih template.
3. Sistem membuat draft pengumuman berdasarkan template.
4. Admin mengisi atau menyesuaikan judul, keterangan, gambar, periode tayang, dan tujuan.
5. Admin melihat preview.
6. Admin menerbitkan atau menjadwalkan pengumuman.

Persyaratan:
- Template dapat digunakan berulang kali.
- Template bawaan tidak boleh hilang tanpa mekanisme pengelolaan yang jelas.
- Perubahan template hanya memengaruhi penggunaan berikutnya.
- Pengumuman yang telah diterbitkan menyimpan snapshot kontennya sendiri.
- Jangan menyimpan hanya referensi template lalu merender ulang pengumuman historis dari template yang mungkin sudah berubah.

### 6.4 Pengaturan Pengumuman

Pengaturan yang dapat dipertimbangkan:
- Periode tayang default.
- Prioritas default.
- Aturan banner utama.
- Target audiens.
- Pengaturan banner bawaan.
- Aturan penjadwalan.

Pengaturan harus tetap sederhana dan sesuai kebutuhan RT. Jangan membuat konfigurasi kompleks tanpa kebutuhan nyata.

## 7. Banner Default Pengingat IPL/Iuran

Salah satu banner bawaan adalah **Pengingat Pembayaran IPL/Iuran**.

Tujuan:
- Melengkapi push notification/pengingat yang sudah ada.
- Memberi pengingat visual yang mudah ditemukan warga ketika membuka Beranda.
- Mengarahkan warga ke menu Keuangan atau halaman tagihan yang relevan.

Persyaratan:
- Template pengingat IPL/Iuran tersedia secara default.
- Banner dapat ditampilkan otomatis berdasarkan aturan periode pengingat dan kewajiban pembayaran yang berlaku.
- Isi banner dapat dikonfigurasi sesuai permission.
- Banner harus mengarah ke halaman Keuangan/tagihan yang tepat jika tautan tujuan disediakan.
- Mekanisme banner tidak boleh menggantikan, merusak, atau mengubah scheduler/notifikasi yang sudah ada.
- Aturan kapan banner muncul harus konsisten dengan status kewajiban pembayaran yang sebenarnya.
- Jangan menampilkan informasi tunggakan individual atau nominal pribadi pada banner umum Beranda.
- Banner umum harus bersifat generik; detail tagihan ditampilkan setelah warga membuka halaman yang memang berhak diaksesnya.

### 7.1 Prioritas Banner IPL/Iuran dan Event

Banner event dapat diprioritaskan ketika ada kegiatan penting, tetapi pengingat IPL/Iuran tidak boleh hilang permanen hanya karena banner event aktif.

Desain yang disarankan:
- Mendukung beberapa banner aktif.
- Mendukung prioritas dan urutan.
- Banner event dapat menjadi banner utama untuk periode tertentu.
- Banner IPL/Iuran tetap dapat diakses/ditampilkan sebagai banner lain atau slot pengingat.
- Setelah event berakhir, banner event tidak lagi diprioritaskan.
- Pengumuman yang kedaluwarsa tidak ditampilkan sebagai pengumuman aktif.

Aturan final slot/rotasi banner dapat diputuskan saat desain UI/UX.

## 8. Hak Akses dan Privasi

- Warga hanya melihat pengumuman dan kegiatan yang dipublikasikan untuk audiensnya.
- Pengelolaan pengumuman dan template memerlukan permission.
- Pengelolaan kegiatan memerlukan permission.
- Data struktur organisasi mengikuti data keanggotaan/jabatan aktif.
- Nomor telepon pengurus dan petugas hanya boleh diberikan kepada pengguna yang berhak.
- Jangan mengirim field kontak sensitif dari backend lalu sekadar menyembunyikannya di frontend.
- Setiap endpoint harus melakukan authorization dan pembatasan RT.
- Jangan menampilkan data pribadi keuangan warga di banner umum.
- Jangan menampilkan data sampah/rumah yang belum terangkut di Beranda.
- Target audiens harus divalidasi di backend dan tidak boleh digunakan untuk melewati isolasi data RT.

## 9. Responsif dan Konsistensi Web/Flutter

Beranda dan menu terkait perlu dirancang untuk Web dan Flutter.

Pertimbangan:
- Urutan konten tetap jelas pada layar kecil maupun besar.
- Banner tidak mengambil seluruh ruang layar secara berlebihan.
- Diagram organisasi dapat dinavigasi pada HP.
- Agenda terdekat ringkas dan mudah dibaca.
- Empty state dan loading state disediakan.
- Gambar memiliki fallback jika gagal dimuat.
- Pengumuman yang sudah kedaluwarsa tidak muncul karena cache yang stale.
- Navigasi ke Keuangan, Kegiatan, dan detail pengumuman konsisten.
- Perubahan menu tidak merusak deep link atau navigasi lama yang masih digunakan.

## 10. Kebutuhan Pengujian

### Beranda
- Konten Beranda baru sesuai urutan dan role.
- Konten keuangan lama tersedia di menu Keuangan.
- Tidak ada ringkasan status sampah/pengangkutan pada Beranda.
- Data petugas aktif berasal dari jadwal dan absensi aktual.
- Petugas yang belum check-in tidak ditampilkan sebagai hadir.
- Nomor telepon hanya tersedia untuk role yang berhak.
- Struktur organisasi mengikuti data aktif.
- Diagram tetap dapat dibaca di Web dan HP.

### Kegiatan
- CRUD sesuai permission.
- Draft tidak terlihat oleh warga.
- Kegiatan terbit terlihat sesuai audiens.
- Kegiatan selesai/dibatalkan tidak salah ditampilkan sebagai agenda mendatang.
- Validasi tanggal dan waktu.
- Integrasi tampilkan sebagai banner.

### Pengumuman dan template
- Membuat pengumuman dari template.
- Template dapat digunakan berulang kali.
- Perubahan template tidak mengubah pengumuman lama.
- Penjadwalan dan periode tayang.
- Prioritas beberapa banner.
- Pengumuman kedaluwarsa tidak ditampilkan.
- Tautan tujuan tervalidasi.
- Hak akses backend diuji, bukan hanya UI.

### IPL/Iuran
- Banner default tersedia.
- Banner muncul sesuai aturan kewajiban/periode yang berlaku.
- Banner umum tidak membocorkan tunggakan individual.
- Banner event tidak menghapus pengingat IPL/Iuran secara permanen.
- Tidak ada regresi terhadap scheduler atau push notification yang sudah berjalan.

## 11. Hal yang Masih Perlu Diputuskan

Catat dan konfirmasi pada tahap desain rinci:

1. Apakah semua pengurus yang memiliki permission dapat membuat kegiatan/pengumuman, atau hanya Ketua RT pada versi pertama?
2. Apakah pengumuman mendukung target audiens selain seluruh warga RT?
3. Apakah banner ditampilkan sebagai carousel, satu banner utama plus daftar, atau beberapa slot?
4. Aturan prioritas final antara banner event dan banner IPL/Iuran.
5. Apakah template dapat dibuat dan diedit oleh Ketua RT atau hanya menggunakan template bawaan pada versi pertama?
6. Field dan sumber data yang tersedia untuk membentuk diagram hierarki pengurus.
7. Aturan persis kapan banner IPL/Iuran muncul, disesuaikan dengan mekanisme tagihan dan scheduler yang sudah ada.
8. Apakah kegiatan memerlukan RSVP/konfirmasi kehadiran pada fase berikutnya? Ini bukan bagian dari scope awal kecuali disetujui.

## 12. Catatan Implementasi

Dokumen ini adalah spesifikasi kebutuhan, bukan instruksi untuk langsung mengimplementasikan fitur.

Sebelum implementasi:
- Audit Beranda lama dan seluruh fitur Keuangan yang ada.
- Pastikan pemindahan menu tidak menghapus fungsi, data, atau permission.
- Audit model keanggotaan/jabatan untuk diagram organisasi.
- Audit sumber data jadwal dan absensi dari Modul Keamanan.
- Audit scheduler/notifikasi dan model kewajiban IPL/Iuran.
- Audit mekanisme upload gambar yang tersedia.
- Ikuti pola API, RBAC, tenant scoping, migration, dan UI yang sudah ada.
- Buat rencana implementasi terpisah untuk backend, Web, Flutter, dan pengujian.
- Hindari perubahan pada fitur Notifikasi yang tidak terkait.
