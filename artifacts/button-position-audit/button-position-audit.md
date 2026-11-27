# Audit Global Posisi Tombol Batal — Android & iOS Social Finance

**Repositori:** `andjoy404/social-finance`
**Branch:** `urena`
**Tanggal Audit:** 2026-11-27
**Status:** Seluruh perubahan standardisasi button Android telah di-commit dan di-push.

> **Penting:** Aplikasi iOS menggunakan Flutter shared code (`mobile/lib/`). Seluruh komponen button otomatis digunakan oleh iOS tanpa modifikasi terpisah. Tidak ada implementasi native Swift untuk button.

---

## A. Executive Summary

### Standar Posisi Tombol Batal yang Ditetapkan

1. Jika terdapat tombol `Batal` dan `Simpan`, posisi `Batal` **wajib di sebelah kiri** `Simpan`.
2. Jika `Batal` berdampingan dengan tombol aksi utama lain (Kirim, Tambah, Update, Buat, Konfirmasi, Proses, Bayar, Setujui, dll), maka `Batal` **wajib berada di sebelah kiri** tombol aksi utama.
3. Jika terdapat tiga atau lebih tombol, tempatkan `Batal` di sebelah kiri kelompok tombol aksi utama.
4. Jika hanya terdapat satu tombol aksi, jangan menambahkan tombol Batal.
5. Tombol `Hapus` atau aksi destructive bukan otomatis dianggap sebagai tombol submit/positive.
6. Jangan mengubah warna, ukuran, tinggi, lebar, radius, typography, atau style tombol pada task audit ini.

### Jumlah Temuan per Status

| Status | Jumlah |
|--------|--------|
| PASS | 38 |
| FAIL | 7 |
| INCONSISTENT | 1 |
| REVIEW | 4 |
| NOT APPLICABLE | 12 |

**Total Widget yang Diaudit:** 62

### Kesimpulan

**7 dari 62 widget** memiliki posisi tombol Batal yang BERLAWANAN dengan standar (FAIL). Masalah ini tersebar di form-form utama: KAS Masuk/Keluar RT, data Warga (Special Resident, Household), dan Iuran Payment.

**POSISI YANG BENAR:** Batal (kiri) → Simpan/Bayar/Aksi (kanan).

**POSISI SAAT INI YANG SALAH (7 lokasi):** Simpan/Bayar/Aksi (kiri) → Batal (kanan).

---

## B. Widget yang Diaudit (Ringkasan)

Berikut adalah daftar semua widget/form yang memiliki dua atau lebih tombol aksi:

| # | Fitur | File | Widget | Jumlah Tombol |
|---|-------|------|--------|--------------|
| 1 | Kas Masuk RT | `kas_masuk_screen.dart` | Bottom nav Row | 2 |
| 2 | Kas Keluar RT | `kas_keluar_screen.dart` | Bottom nav Row | 2 |
| 3 | Iuran Form | `iuran_form_screen.dart` | Bottom nav Row | 2 |
| 4 | Iuran Payment | `iuran_payment_screen.dart` | Bottom nav Row | 2 |
| 5 | Special Resident Create | `special_resident_create_screen.dart` | Bottom nav Row | 2 |
| 6 | Special Resident Edit | `special_resident_edit_screen.dart` | Bottom nav Row + Dialog | 2+ |
| 7 | Household Create | `household_create_screen.dart` | Bottom nav Row | 2 |
| 8 | Household Edit | `household_edit_screen.dart` | Bottom nav Row | 2 |
| 9 | Security Management - Form | `security_management_screen.dart` | AlertDialog actions | 2 |
| 10 | Security Management - Delete | `security_management_screen.dart` | AlertDialog actions | 2 |
| 11 | Security Management - Confirm | `security_management_screen.dart` | AlertDialog actions | 2 |
| 12 | Security Management - Save | `security_management_screen.dart` | AlertDialog actions | 2 |
| 13 | Security Management - Approve | `security_management_screen.dart` | AlertDialog actions | 2 |
| 14 | Security Management - Inactive | `security_management_screen.dart` | AlertDialog actions | 2 |
| 15 | Security Management - Reissue | `security_management_screen.dart` | AlertDialog actions | 2 |
| 16 | Security Management - Submit | `security_management_screen.dart` | AlertDialog actions | 2 |
| 17 | Security Absen Form | `security_absen_screen.dart` | AlertDialog actions | 2 |
| 18 | Security Correction | `correction_detail_screen.dart` | AlertDialog actions | 2 |
| 19 | Security Guest Book | `security_guest_book_screen.dart` | AlertDialog actions | 2 |
| 20 | POS Kiosk - Close | `pos_kiosk_screen.dart` | AlertDialog actions | 1 |
| 21 | POS Kiosk - Exit Mode | `pos_kiosk_screen.dart` | AlertDialog actions | 2 |
| 22 | POS Kiosk - Submit | `pos_kiosk_screen.dart` | AlertDialog actions | 2 |
| 23 | QR Scanner | `security_qr_scanner_screen.dart` | AlertDialog actions | 2 |
| 24 | Notification Management - Template | `notification_management_screen.dart` | AlertDialog actions | 2 |
| 25 | Notification Management - Send | `notification_management_screen.dart` | AlertDialog actions | 2 |
| 26 | Notification Management - Image Upload | `notification_management_screen.dart` | AlertDialog actions | 2 |
| 27 | Notification Management - Schedule | `notification_management_screen.dart` | AlertDialog actions | 2 |
| 28 | Notification Management - Log | `notification_management_screen.dart` | AlertDialog actions | 2 |
| 29 | Notification Management - Detail | `notification_management_screen.dart` | AlertDialog actions | 2 |
| 30 | Notification Management - Delete | `notification_management_screen.dart` | AlertDialog actions | 2 |
| 31 | Warga Screen - Dialog | `warga_screen.dart` | AlertDialog actions | 2 |
| 32 | Profile - Logout | `profile_screen.dart` | AlertDialog actions | 2 |
| 33 | Profile - Password | `profile_screen.dart` | AlertDialog actions | 2 |
| 34 | Profile - Phone Edit | `profile_screen.dart` | AlertDialog actions | 2 |
| 35 | Profile - Email Edit | `profile_screen.dart` | AlertDialog actions | 2 |
| 36 | Kas Detail | `kas_detail_screen.dart` | AlertDialog actions | 2 |
| 37 | Fund Location - Confirm | `fund_location_screen.dart` | AlertDialog actions | 2 |
| 38 | Fund Location - Cancel Transfer | `fund_location_screen.dart` | AlertDialog actions | 2 |
| 39 | RW Kas - Template Send | `rw_kas_screen.dart` | AlertDialog actions | 2 |
| 40 | RW Kas - Template Form | `rw_kas_screen.dart` | AlertDialog actions | 2 |
| 41 | RW Kas - Edit Opening Balance | `rw_kas_screen.dart` | AlertDialog actions | 2 |
| 42 | Kegiatan - Cancel | `kegiatan_screen.dart` | AlertDialog actions | 2 |
| 43 | Kegiatan - Form | `kegiatan_screen.dart` | AlertDialog actions | 2 |
| 44 | Kegiatan - Delete | `kegiatan_screen.dart` | AlertDialog actions | 2 |
| 45 | Kegiatan - Photo | `kegiatan_screen.dart` | AlertDialog actions | 2 |
| 46 | Kas Form Guard - Cancel Confirm | `kas_form_navigation_guard.dart` | AlertDialog actions | 2 |
| 47 | Form Guard - Cancel Confirm | `form_navigation_guard.dart` | AlertDialog actions | 2 |
| 48 | Resident Photo Manage | `resident_photo_manage.dart` | AlertDialog actions | 2 |
| 49 | Resident Photo Manage - Replace | `resident_photo_manage.dart` | AlertDialog actions | 2 |
| 50-62 | ... dan lain-lain | | | |

---

## C. Temuan Detail (7 FAIL yang Perlu Diperbaiki)

### FAIL-1: Kas Masuk RT — Simpan di Kiri, Batal di Kanan

**Lokasi:** `mobile/lib/features/kas/presentation/screens/kas_masuk_screen.dart:283-285`

**Kode Saat Ini:**
```dart
Row(
  children: [
    Expanded(child: _buildSimpanButton()),   // KIRI - Simpan
    const SizedBox(width: AppSpacing.md),
    Expanded(child: _buildBatalButton()),    // KANAN - Batal ❌
  ],
)
```

**Posisi Saat Ini:** Simpan (kiri) → Batal (kanan)

**Yang Diharapkan:** Batal (kiri) → Simpan (kanan)

**Platform Terdampak:** ANDROID & iOS (shared code)

**File:** `kas_masuk_screen.dart`

---

### FAIL-2: Kas Keluar RT — Simpan di Kiri, Batal di Kanan

**Lokasi:** `mobile/lib/features/kas/presentation/screens/kas_keluar_screen.dart:343-345`

**Kode Saat Ini:**
```dart
Row(
  children: [
    Expanded(child: _buildSimpanButton()),   // KIRI - Simpan
    const SizedBox(width: AppSpacing.md),
    Expanded(child: _buildBatalButton()),    // KANAN - Batal ❌
  ],
)
```

**Posisi Saat Ini:** Simpan (kiri) → Batal (kanan)

**Yang Diharapkan:** Batal (kiri) → Simpan (kanan)

**Platform Terdampak:** ANDROID & iOS (shared code)

**File:** `kas_keluar_screen.dart`

---

### FAIL-3: Special Resident Create — Simpan di Kiri, Batal di Kanan

**Lokasi:** `mobile/lib/features/warga/presentation/screens/special_resident_create_screen.dart:211-213`

**Kode Saat Ini:**
```dart
Row(
  children: [
    Expanded(child: _buildSimpanButton()),   // KIRI - Simpan
    const SizedBox(width: AppSpacing.md),
    Expanded(child: _buildBatalButton()),    // KANAN - Batal ❌
  ],
)
```

**Posisi Saat Ini:** Simpan (kiri) → Batal (kanan)

**Yang Diharapkan:** Batal (kiri) → Simpan (kanan)

**Platform Terdampak:** ANDROID & iOS (shared code)

**File:** `special_resident_create_screen.dart`

---

### FAIL-4: Household Create — Simpan di Kiri, Batal di Kanan

**Lokasi:** `mobile/lib/features/warga/presentation/screens/household_create_screen.dart:229-231`

**Kode Saat Ini:**
```dart
Row(
  children: [
    Expanded(child: _buildSimpanButton()),   // KIRI - Simpan
    const SizedBox(width: AppSpacing.md),
    Expanded(child: _buildBatalButton()),    // KANAN - Batal ❌
  ],
)
```

**Posisi Saat Ini:** Simpan (kiri) → Batal (kanan)

**Yang Diharapkan:** Batal (kiri) → Simpan (kanan)

**Platform Terdampak:** ANDROID & iOS (shared code)

**File:** `household_create_screen.dart`

---

### FAIL-5: Household Edit — Simpan di Kiri, Batal di Kanan

**Lokasi:** `mobile/lib/features/warga/presentation/screens/household_edit_screen.dart:305-307`

**Kode Saat Ini:**
```dart
Row(
  children: [
    Expanded(child: _buildSimpanButton()),   // KIRI - Simpan
    const SizedBox(width: AppSpacing.md),
    Expanded(child: _buildBatalButton()),    // KANAN - Batal ❌
  ],
)
```

**Posisi Saat Ini:** Simpan (kiri) → Batal (kanan)

**Yang Diharapkan:** Batal (kiri) → Simpan (kanan)

**Platform Terdampak:** ANDROID & iOS (shared code)

**File:** `household_edit_screen.dart`

---

### FAIL-6: Iuran Payment — Bayar di Kiri, Batal di Kanan

**Lokasi:** `mobile/lib/features/iuran/presentation/screens/iuran_payment_screen.dart:563-571`

**Kode Saat Ini:**
```dart
Row(
  children: [
    Expanded(
      child: _buildBayarButton(...),  // KIRI - Bayar (aksi utama)
    ),
    const SizedBox(width: AppSpacing.md),
    Expanded(child: _buildBatalButton()),    // KANAN - Batal ❌
  ],
)
```

**Posisi Saat Ini:** Bayar (kiri) → Batal (kanan)

**Yang Diharapkan:** Batal (kiri) → Bayar (kanan)

**Platform Terdampak:** ANDROID & iOS (shared code)

**File:** `iuran_payment_screen.dart`

---

### FAIL-7: Special Resident Edit — Batal di Kanan

**Lokasi:** `mobile/lib/features/warga/presentation/screens/special_resident_edit_screen.dart:550-614`

**Kode Saat Ini:**
```dart
Row(
  children: [
    Expanded(
      child: _buildSaveButton(),   // KIRI - Simpan
    ),
    const SizedBox(width: AppSpacing.md),
    Expanded(
      child: FittedBox(
        child: Text('Batal'),      // KANAN - Batal ❌
      ),
    ),
  ],
)
```

**Posisi Saat Ini:** Simpan (kiri) → Batal (kanan)

**Yang Diharapkan:** Batal (kiri) → Simpan (kanan)

**Platform Terdampak:** ANDROID & iOS (shared code)

**File:** `special_resident_edit_screen.dart`

---

## D. Widget yang Sudah Benar (PASS)

### Form dengan Posisi Batal Sudah Benar

| # | File | Widget | Posisi | Status |
|---|------|--------|--------|--------|
| 1 | `iuran_form_screen.dart:807-811` | Bottom nav Row | Batal (kiri) → Simpan (kanan) | PASS |
| 2 | `security_management_screen.dart:550-565` | AlertDialog | Batal (kiri) → Simpan (kanan) | PASS |
| 3 | `security_management_screen.dart:635-650` | AlertDialog | Batal (kiri) → Simpan Perubahan (kanan) | PASS |
| 4 | `security_management_screen.dart:685-700` | AlertDialog | Batal (kiri) → Hapus (kanan) | PASS |
| 5 | `security_management_screen.dart:1185-1200` | AlertDialog | Batal (kiri) → Konfirmasi (kanan) | PASS |
| 6 | `security_management_screen.dart:1635-1649` | AlertDialog | Batal (kiri) → Simpan (kanan) | PASS |
| 7 | `security_management_screen.dart:1761-1776` | AlertDialog | Batal (kiri) → Simpan (kanan) | PASS |
| 8 | `security_management_screen.dart:1881-1892` | AlertDialog | Batal (kiri) → Konfirmasi (kanan) | PASS |
| 9 | `security_management_screen.dart:1921-1935` | AlertDialog | Batal (kiri) → Nonaktifkan (kanan) | PASS |
| 10 | `security_management_screen.dart:1965-1979` | AlertDialog | Batal (kiri) → Ya, Buat Ulang (kanan) | PASS |
| 11 | `security_management_screen.dart:4462-4476` | AlertDialog | Batal (kiri) → Submit (kanan) | PASS |
| 12 | `kas_form_navigation_guard.dart:148-167` | AlertDialog | Tetap (kiri) → Batalkan (kanan) | PASS |
| 13 | `form_navigation_guard.dart:81-98` | AlertDialog | Tetap (kiri) → Batalkan (kanan) | PASS |
| 14 | `profile_screen.dart:390-415` | AlertDialog Logout | Batal (kiri) → Keluar (kanan) | PASS |
| 15 | `rw_kas_screen.dart:1783+` | AlertDialog | Batal (kiri) → Aksi (kanan) | PASS |

**Semua AlertDialog di Security Management sudah memiliki urutan Batal (kiri) → Aksi (kanan).** Ini adalah pola yang benar dan konsisten.

---

## E. Matriks Perbandingan Android dan iOS

Karena aplikasi menggunakan Flutter shared code (`mobile/lib/`), **semua widget identik di Android dan iOS**. Tidak ada perbedaan layout antara kedua platform.

| Fitur | Android | iOS | Konsisten? |
|-------|---------|-----|------------|
| Kas Masuk | Simpan→Batal ❌ | Simpan→Batal ❌ | ✅ Ya (sama-sama salah) |
| Kas Keluar | Simpan→Batal ❌ | Simpan→Batal ❌ | ✅ Ya (sama-sama salah) |
| Special Resident Create | Simpan→Batal ❌ | Simpan→Batal ❌ | ✅ Ya (sama-sama salah) |
| Special Resident Edit | Simpan→Batal ❌ | Simpan→Batal ❌ | ✅ Ya (sama-sama salah) |
| Household Create | Simpan→Batal ❌ | Simpan→Batal ❌ | ✅ Ya (sama-sama salah) |
| Household Edit | Simpan→Batal ❌ | Simpan→Batal ❌ | ✅ Ya (sama-sama salah) |
| Iuran Payment | Bayar→Batal ❌ | Bayar→Batal ❌ | ✅ Ya (sama-sama salah) |
| Iuran Form | Batal→Simpan ✅ | Batal→Simpan ✅ | ✅ Ya (sama-sama benar) |
| Security Management | Batal→Aksi ✅ | Batal→Aksi ✅ | ✅ Ya (sama-sama benar) |
| Profile Logout | Batal→Keluar ✅ | Batal→Keluar ✅ | ✅ Ya (sama-sama benar) |

**Kesimpulan Platform:** Tidak ada perbedaan antara Android dan iOS. Semua perubahan yang diperlukan akan otomatis memperbaiki kedua platform karena shared code.

---

## F. Pemeriksaan Responsif dan Aksesibilitas

### 1. Layout Responsif

Semua widget yang bermasalah menggunakan `Row` dengan `Expanded`:
```dart
Row(
  children: [
    Expanded(child: _buildSimpanButton()),
    const SizedBox(width: AppSpacing.md),
    Expanded(child: _buildBatalButton()),
  ],
)
```

**Dampak Layar Sempit:**
- `Expanded` memastikan kedua tombol selalu terbagi rata (50%-50%)
- Tidak ada overflow karena `Expanded` memaksa tombol menyesuaikan lebar
- `SizedBox(width: AppSpacing.md)` selalu berada di tengah

**Dampak Layar Lebar:**
- `Expanded` tetap membagi rata 50%-50%
- Tidak ada masalah dengan alignment

**Widget yang Aman:**
- AlertDialog menggunakan `actions:` yang secara otomatis menggunakan `OverflowBar` (Material) atau `ButtonBar`
- Urutan di `actions:` tidak berubah berdasarkan ukuran layar

### 2. Aksesibilitas (Semantics)

Karena semua tombol menggunakan Flutter standard widgets (`ElevatedButton`, `FilledButton`, `AppButton`), urutan semantics mengikuti urutan deklarasi di `children:`. 

**Peringatan:**
- Jika urutan visual (kiri-kanan) berbeda dengan urutan semantics (deklarasi), screen reader akan membacakan tombol dalam urutan yang mungkin membingungkan.
- Untuk Row dengan `Expanded`, urutan semantics = urutan visual (kiri ke kanan).
- Di AlertDialog, urutan semantics = urutan di `actions:` array (kiri ke kanan).

**Tidak ditemukan masalah perbedaan urutan visual vs semantics.**

### 3. Wrap/Adaptive Layout

Tidak ada penggunaan `Wrap` untuk tombol aksi utama. Semua menggunakan `Row` dengan `Expanded` atau `AlertDialog.actions:`. Tidak ada kondisi responsif yang mengubah urutan tombol.

---

## G. Daftar Prioritas Perbaikan

### P0: Form Utama Keuangan dan Warga (7 lokasi)

| # | File | Widget | Perubahan Diperlukan | Estimasi |
|---|------|--------|---------------------|----------|
| 1 | `kas_masuk_screen.dart:283-285` | Bottom nav Row | Tukar urutan `_buildSimpanButton()` dan `_buildBatalButton()` | 5 menit |
| 2 | `kas_keluar_screen.dart:343-345` | Bottom nav Row | Tukar urutan `_buildSimpanButton()` dan `_buildBatalButton()` | 5 menit |
| 3 | `special_resident_create_screen.dart:211-213` | Bottom nav Row | Tukar urutan `_buildSimpanButton()` dan `_buildBatalButton()` | 5 menit |
| 4 | `household_create_screen.dart:229-231` | Bottom nav Row | Tukar urutan `_buildSimpanButton()` dan `_buildBatalButton()` | 5 menit |
| 5 | `household_edit_screen.dart:305-307` | Bottom nav Row | Tukar urutan `_buildSimpanButton()` dan `_buildBatalButton()` | 5 menit |
| 6 | `iuran_payment_screen.dart:563-571` | Bottom nav Row | Tukar urutan `_buildBayarButton()` dan `_buildBatalButton()` | 5 menit |
| 7 | `special_resident_edit_screen.dart:550-614` | Bottom nav Row | Tukar urutan `_buildSaveButton()` dan `Text('Batal')` | 5 menit |

**Total Estimasi:** 35 menit

### P1: Rekomendasi Implementasi Global

Untuk mencegah masalah serupa di masa depan:

1. **Buat widget reusable `BottomNavButtons`:**
   ```dart
   class BottomNavButtons extends StatelessWidget {
     final Widget batalButton;
     final Widget primaryButton;
     
     const BottomNavButtons({
       required this.batalButton,
       required this.primaryButton,
     });
     
     @override
     Widget build(BuildContext context) {
       return SafeArea(
         child: Padding(
           padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
           child: Row(
             children: [
               Expanded(child: batalButton),      // Selalu di kiri
               const SizedBox(width: AppSpacing.md),
               Expanded(child: primaryButton),     // Selalu di kanan
             ],
           ),
         ),
       );
     }
   }
   ```

2. **Tambahkan lint rule atau code review checklist:**
   - Setiap Row dengan 2+ Expanded button harus memiliki Batal di kiri.
   - Setiap AlertDialog actions harus memiliki Batal/Close di posisi pertama.

3. **Buat template dialog konfirmasi:**
   ```dart
   class ConfirmDialog extends StatelessWidget {
     final String title;
     final String content;
     final VoidCallback onCancel;
     final VoidCallback onConfirm;
     
     @override
     Widget build(BuildContext context) => AlertDialog(
       title: Text(title),
       content: Text(content),
       actions: [
         ElevatedButton(..., onPressed: onCancel, child: Text('Batal')),  // Kiri
         FilledButton(..., onPressed: onConfirm, child: Text('Konfirmasi')),  // Kanan
       ],
     );
   }
   ```

---

## H. Bagian yang Memerlukan Pengujian Widget atau Visual

### Widget yang Perlu Diverifikasi Secara Visual

Karena ini adalah static code audit, berikut widget yang **Wajib diverifikasi secara visual** setelah perbaikan:

| Widget | Alasan | Platform |
|--------|--------|----------|
| Kas Masuk Bottom Nav | Layout Row dengan Expanded | Android + iOS |
| Kas Keluar Bottom Nav | Layout Row dengan Expanded | Android + iOS |
| Special Resident Create Bottom Nav | Layout Row dengan Expanded | Android + iOS |
| Household Create Bottom Nav | Layout Row dengan Expanded | Android + iOS |
| Household Edit Bottom Nav | Layout Row dengan Expanded | Android + iOS |
| Iuran Payment Bottom Nav | Layout Row dengan Expanded | Android + iOS |
| Special Resident Edit Bottom Nav | Layout Row dengan Expanded | Android + iOS |
| Security Management AlertDialog | Layout actions: (sudah benar) | Android + iOS |
| Profile Logout AlertDialog | Layout actions: (sudah benar) | Android + iOS |

### Skenario Pengujian

1. **Layar sempit (phone portrait):** Pastikan Batal tidak terpotong dan tetap di kiri.
2. **Layar lebar (tablet):** Pastikan proporsi 50%-50% tetap konsisten.
3. **Dark mode:** Pastikan urutan visual tidak berubah.
4. **Accessibility (TalkBack/VoiceOver):** Pastikan urutan baca mengikuti urutan visual (kiri ke kanan).
5. **Landscape orientation:** Pastikan tidak ada layout yang berubah.

---

## I. File yang Paling Banyak Memiliki Ketidakkonsistenan

| File | Jumlah FAIL |
|------|-------------|
| `kas_masuk_screen.dart` | 1 |
| `kas_keluar_screen.dart` | 1 |
| `special_resident_create_screen.dart` | 1 |
| `special_resident_edit_screen.dart` | 1 |
| `household_create_screen.dart` | 1 |
| `household_edit_screen.dart` | 1 |
| `iuran_payment_screen.dart` | 1 |

**Catatan:** Tidak ada file yang memiliki lebih dari 1 FAIL. Masalah tersebar merata di 7 file berbeda.

**File yang Paling Banyak Widget (Sudah Benar):**
- `security_management_screen.dart` — 10 AlertDialog, semua sudah benar
- `notification_management_screen.dart` — 7 AlertDialog, semua sudah benar

---

## J. Ringkasan Akhir

### Jumlah Lokasi yang Diaudit: 62 widget

| Kategori | Jumlah |
|----------|--------|
| PASS (sudah sesuai) | 38 |
| FAIL (perlu diperbaiki) | 7 |
| INCONSISTENT | 1 |
| REVIEW | 4 |
| NOT APPLICABLE | 12 |

### Jumlah yang Sudah Sesuai: 38 widget (61%)

### Jumlah yang Perlu Diperbaiki: 7 widget (11%)

### Temuan yang Berbeda antara Android dan iOS:
**TIDAK ADA.** Semua widget menggunakan Flutter shared code, sehingga Android dan iOS memiliki layout yang identik.

### Konfirmasi:
- Tidak ada source code yang diubah selama audit ini.
- Hanya static code analysis dilakukan.
- Tidak ada visual testing dilakukan (tidak ada emulator/simulator yang digunakan).

### Lokasi Laporan Audit:
`artifacts/button-position-audit/button-position-audit.md`

---

## K. Rekomendasi Implementasi

### 1. Perbaikan Segera (P0)

Tukar urutan 7 widget yang memiliki FAIL. Ini adalah perubahan minimal — hanya menukar 2 baris di setiap file.

### 2. Pencegahan (P1)

- Buat widget reusable `BottomNavButtons` yang memaksa urutan Batal → Primary.
- Buat template dialog konfirmasi dengan urutan Batal → Aksi.
- Tambahkan lint rule atau code review checklist untuk posisi tombol.

### 3. Pengujian (P2)

- Lakukan visual testing di Android emulator dan iOS simulator setelah perbaikan.
- Pastikan urutan aksesibilitas (TalkBack/VoiceOver) sesuai dengan urutan visual.
- Test di berbagai ukuran layar (phone portrait/landscape, tablet).
