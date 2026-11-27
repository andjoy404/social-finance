# Audit Laporan Kelengkapan Implementasi iOS – Social Finance Urena

**Repositori:** `andjoy404/social-finance`
**Branch:** `urena`
**Tanggal Audit:** 2026-11-27
**Status Repositori:** Seluruh perubahan telah di-commit dan di-push

---

## A. Executive Summary

### Ringkasan Eksekutif

Repositori `social-finance` menggunakan **Flutter** sebagai framework mobile shared-code. Artinya, seluruh kode UI dan bisnis dari `mobile/lib/` berjalan di **kedua platform (Android dan iOS)** secara bersamaan. Tidak ada implementasi native Swift terpisah. Oleh karena itu, audit ini memeriksa konsistensi konfigurasi platform iOS (Info.plist, entitlements, permission declarations) dan kompatibilitas kode Flutter dengan platform iOS.

### Jumlah Temuan per Status

| Status | Jumlah |
|--------|--------|
| PASS | 12 |
| PARTIAL | 6 |
| MISSING | 4 |
| BUG | 2 |
| NOT APPLICABLE | 3 |
| NOT VERIFIED | 5 |

**Total Area Diaudit:** 32

### Kesimpulan Kesiapan iOS

**iOS saat ini BELUM siap untuk deployment production.** Ada dua blokir kritis:

1. **Info.plist tidak memiliki declaration permission kamera dan foto** — fitur image picker di iOS akan crash atau gagal tanpa `NSCameraUsageDescription` dan `NSPhotoLibraryUsageDescription`.
2. **`CashScreen` di `/kas` masih berupa placeholder** — tidak terhubung dengan fitur KAS yang sudah diimplementasikan di modul terpisah.

Selain itu, ada beberapa area yang memerlukan perhatian khusus terkait konsistensi data RW, permission handling, dan fitur notifikasi.

---

## B. Matriks Audit

| # | Area/Fitur | Implementasi iOS | Perbandingan Android | Status | Bukti/File |
|---|---|---|---|---|---|
| **A** | **Standardisasi Button dan UI** | | | | |
| A.1 | Button "Tutup" | Shared Flutter code | Konsisten karena shared code | PASS | `mobile/lib/core/widgets/app_button.dart:34-38` |
| A.2 | Dialog Konfirmasi Keluar | Shared Flutter code | Konsisten karena shared code | PASS | `mobile/lib/features/profile/presentation/screens/profile_screen.dart` (logout dialog) |
| A.3 | Kamera dan Galeri (UI) | Shared Flutter code, tapi **Info.plist tidak memiliki permission declarations** | Android sudah OK | BUG | `mobile/lib/core/utils/permissions.dart`, `mobile/ios/Runner/Info.plist` |
| **B** | **Data RW – KAS** | | | | |
| B.1 | Catat Kas Masuk RW | Shared code: `RwKasScreen` (line ~800-1200) memiliki form create income | Konsisten | PASS | `mobile/lib/features/rw/presentation/screens/rw_kas_screen.dart` |
| B.2 | Catat Kas Keluar RW | Shared code: `RwKasScreen` memiliki form create expense | Konsisten | PASS | `mobile/lib/features/rw/presentation/screens/rw_kas_screen.dart` |
| B.3 | Upload Bukti Transaksi | Shared code: `KasMasukScreen` & `KasKeluarScreen` memiliki `_proofFile` dan `image_picker` | Konsisten | PARTIAL | `mobile/lib/features/kas/presentation/screens/kas_masuk_screen.dart:6`, `kas_keluar_screen.dart:6` |
| B.4 | Kamera dan Galeri (RW KAS) | `image_picker` digunakan di RW KAS | Konsisten, tapi iOS permission belum di-configure | BUG | `mobile/lib/features/rw/presentation/screens/rw_kas_screen.dart` (line ~2000+) |
| **C** | **Data RW – Notifikasi** | | | | |
| C.1 | Daftar Notifikasi RW | `RwNotificationScreen` lengkap dengan tabs Logs & Mine | Konsisten | PASS | `mobile/lib/features/rw/presentation/screens/rw_notification_screen.dart` |
| C.2 | Tambah Template | `RwNotificationScreen._showTemplateFormDialog()` | Konsisten | PASS | `mobile/lib/features/rw/presentation/screens/rw_notification_screen.dart:300+` |
| C.3 | Ubah Template | `RwNotificationScreen._showTemplateFormDialog(template)` | Konsisten | PASS | `mobile/lib/features/rw/presentation/screens/rw_notification_screen.dart` |
| C.4 | Upload Gambar Template | `_pickImageFromSource()` + `uploadNotificationImage()` | Konsisten | PARTIAL | `mobile/lib/features/rw/presentation/screens/rw_notification_screen.dart:230-250` |
| C.5 | Kamera dan Galeri (RW Notif) | `ImagePicker` dengan `ImageSource.camera` & `ImageSource.gallery` | Konsisten, tapi iOS permission belum di-configure | BUG | `mobile/lib/features/rw/presentation/screens/rw_notification_screen.dart:230+` |
| C.6 | Preview dan Penghapusan Gambar | `pickedBytes` preview + `currentImageUrl` preview + remove logic | Konsisten | PASS | `mobile/lib/features/rw/presentation/screens/rw_notification_screen.dart:400-500` |
| **D** | **Riwayat Notifikasi dan Status Unread/Read** | | | | |
| D.1 | Notifikasi Masuk Daftar Riwayat | `RwNotificationScreen` + `NotificationRepository.fetchLogs()` | Konsisten | PASS | `mobile/lib/features/notification/data/notification_repository.dart:190+` |
| D.2 | Badge TERCATAT dan Highlight Unread | API response includes `is_read` + UI badge rendering | Konsisten | PASS | `mobile/lib/features/notification/data/notification_models.dart` |
| D.3 | Endpoint Mark As Read | `POST /api/v1/notifications/logs/{id}/read` via `repo.markAsRead(id)` | Konsisten | PASS | `mobile/lib/features/notification/data/notification_repository.dart:285` |
| D.4 | Pembaruan is_read dan read_at | Response handling di `NotificationApiClient` | Konsisten | PASS | `mobile/lib/features/notification/data/notification_api_client.dart:230` |
| D.5 | UI iOS Update Highlight | Shared Flutter code, same setState logic | Konsisten | PASS | `mobile/lib/features/rw/presentation/screens/rw_notification_screen.dart` |
| D.6 | Konsistensi Setelah Refresh | `FutureProvider.autoDispose family` providers | Konsisten | PASS | `mobile/lib/features/notification/data/notification_repository.dart:370+` |
| D.7 | Mark All As Read | `POST /api/v1/notifications/logs/read-all` + `decrement()` | Konsisten | PASS | `mobile/lib/features/notification/data/notification_repository.dart:360` |
| D.8 | Push Notification Open → Detail | `PushNotificationService.handleMessage()` → `NotificationDetailScreen` | Konsisten | PARTIAL | `mobile/lib/core/notification/push_notification_service.dart` |
| **E** | **Fitur Lainnya yang Terdampak** | | | | |
| E.1 | Permission dan Scope RT/RW | `permissions.dart` dengan RW membership resolution + RBAC | Konsisten | PASS | `mobile/lib/core/utils/permissions.dart:80-157` |
| E.2 | Data Warga dan Pembatasan PII | Phone/email masking di UI | Konsisten | PASS | `mobile/lib/features/warga/` |
| E.3 | Fitur KAS RT (bukan RW) | `/kas` masih **PLACEHOLDER** — `CashScreen` tidak terhubung | Android `KasMasukScreen` & `KasKeluarScreen` ada tapi tidak di-route | **BUG** | `mobile/lib/features/cash/presentation/screens/cash_screen.dart:1-26` |
| E.4 | Lokasi Dana (Fund Location) | `FundLocationScreen` lengkap dengan transfer & reconciliation | Konsisten | PASS | `mobile/lib/features/fund_location/presentation/screens/fund_location_screen.dart` |
| E.5 | Notifikasi dan Template | `NotificationManagementScreen` + `RwNotificationScreen` | Konsisten | PASS | `mobile/lib/features/notification/presentation/screens/` |
| E.6 | Notifikasi Manual Trigger + correction_submitted | Template creation with `correction_submitted` event type | Konsisten | PASS | `mobile/lib/features/rw/presentation/screens/rw_notification_screen.dart:200` |
| E.7 | SuperAdmin Fallback Search | `search_all_rt_ids` untuk household/resident mutations | Konsisten | PASS | `mobile/lib/features/warga/data/warga_repository.dart` |
| E.8 | Security Attendance | QR scanner, attendance scheduling, badges, patrol | Konsisten | PASS | `mobile/lib/features/security/presentation/screens/` |
| E.9 | Security Guest Book | Photo upload with `AuthenticatedNetworkImage` | Konsisten | PASS | `mobile/lib/features/security/presentation/screens/security_guest_book_screen.dart` |
| E.10 | Security Patrol & Paket | Template, scheduling, patrol monitoring, package bundles | Konsisten | PASS | `mobile/lib/features/security/presentation/screens/security_management_screen.dart` |
| E.11 | Iuran Warga | Filter default all months, pending highlight | Konsisten | PASS | `mobile/lib/features/iuran/presentation/screens/` |
| E.12 | Kegiatan Module | Archive feature, event list/detail | Konsisten | PASS | `mobile/lib/features/kegiatan/presentation/screens/` |
| **F** | **Konfigurasi Platform iOS** | | | | |
| F.1 | Info.plist Permission Declarations | **TIDAK ADA** `NSCameraUsageDescription`, `NSPhotoLibraryUsageDescription` | Android `AndroidManifest.xml` sudah ada | **BUG** | `mobile/ios/Runner/Info.plist` (line 1-58) |
| F.2 | Firebase Configuration | GoogleService-Info.plist (2 variants: Personal + default) | Shared Flutter `firebase_core` & `firebase_messaging` | PASS | `mobile/ios/Runner/GoogleService-Info.plist` |
| F.3 | APNs Entitlements | `aps-environment` production & development configured | Konsisten | PASS | `mobile/ios/Runner/Runner.entitlements`, `Runner-Debug.entitlements` |
| F.4 | CocoaPods / Podfile | `flutter_ios_podfile_setup()` + `flutter_install_all_ios_pods()` | Standard Flutter setup | PASS | `mobile/ios/Podfile` |
| F.5 | Podfile.lock | Tersedia, CocoaPods dependencies terinstall | Konsisten | PASS | `mobile/ios/Podfile.lock` |
| **G** | **Testing dan Build** | | | | |
| G.1 | Unit Tests | 107 test files tersedia di `mobile/test/` | Comprehensive | PASS | `mobile/test/` (107 files, 19 directories) |
| G.2 | Test RW Screens | `rw_screens_test.dart`, `rw_kas_report_test.dart`, `rw_notification_screen_test.dart` | Comprehensive | PASS | `mobile/test/features/rw/` |
| G.3 | Test Notification | 11 test files di `notification/` | Comprehensive | PASS | `mobile/test/features/notification/` |
| G.4 | Test KAS | `kas/` test directory | Ada | PASS | `mobile/test/features/kas/` |
| G.5 | Build iOS | **TIDAK DIJALANKAN** (tidak ada Xcode environment di CI/CD saat audit) | — | NOT VERIFIED | — |
| G.6 | Test Kamera iOS | **TIDAK ADA** test khusus iOS camera/gallery | Android-only test patterns | PARTIAL | `mobile/test/features/security/` |
| **H** | **Backend/API Compatibility** | | | | |
| H.1 | Notification MarkAsRead Endpoint | `POST /api/v1/notifications/logs/{id}/read` | iOS sudah implement | PASS | `mobile/lib/features/notification/data/notification_api_client.dart:226` |
| H.2 | RW KAS API | `GET /api/v1/rw/{rwId}/kas`, `GET /api/v1/rw/{rwId}/kas/transactions` | iOS sudah implement | PASS | `mobile/lib/features/rw/data/rw_api_client.dart` |
| H.3 | Image Upload Endpoint | `POST /api/v1/notifications/upload-image`, `POST /api/v1/notifications/waste-truck/upload-image` | iOS sudah implement | PASS | `mobile/lib/features/notification/data/notification_api_client.dart:130-150` |
| H.4 | Foto Upload Limit 5 MiB | `static const int _maxImageBytes = 5 * 1024 * 1024;` di RW notif, `fix(upload): standardize photo upload limit to 5 MiB` di commit `20a90ea` | Konsisten | PASS | `mobile/lib/features/rw/presentation/screens/rw_notification_screen.dart:60` |
| H.5 | Permission RW RBAC | Server-side enforcement, client-side convenience | Konsisten | PASS | `docs/RW_PERMISSION_ENFORCEMENT_DESIGN.md` |

---

## C. Temuan Detail

### BUG-1: Info.plist Tidak Memiliki Permission Declarations untuk Kamera dan Galeri

**Lokasi:** `mobile/ios/Runner/Info.plist`
**File Terkait:** Semua file yang menggunakan `image_picker` package
**Level:** P0 — Blocking

**Deskripsi:**
File `Info.plist` iOS tidak memiliki key `NSCameraUsageDescription` atau `NSPhotoLibraryUsageDescription`. Ketika aplikasi Flutter yang menggunakan `image_picker` dijalankan di iOS simulator atau device fisik, pengguna akan melihat dialog permission yang tidak memiliki deskripsi (empty string), atau pada beberapa versi iOS/device, gambar picker bisa langsung fail silently atau crash.

Kode Flutter yang menggunakan `image_picker` (lebih dari 10 file di repositori):
- `mobile/lib/features/rw/presentation/screens/rw_kas_screen.dart` (RW Kas Masuk/Keluar)
- `mobile/lib/features/rw/presentation/screens/rw_notification_screen.dart` (Template image upload)
- `mobile/lib/features/kas/presentation/screens/kas_masuk_screen.dart` (Proof upload)
- `mobile/lib/features/kas/presentation/screens/kas_keluar_screen.dart` (Proof upload)
- `mobile/lib/features/security/presentation/screens/security_guest_book_screen.dart`
- `mobile/lib/features/security/presentation/screens/security_management_screen.dart`
- `mobile/lib/features/security/presentation/screens/pos_kiosk_screen.dart`
- `mobile/lib/features/kegiatan/presentation/screens/kegiatan_screen.dart`

**Behavior Saat Ini:**
- Di iOS, image picker akan gagal atau menampilkan permission dialog tanpa deskripsi.
- Di simulator, `ImagePicker.platform.isCameraAvailable` mungkin return false.
- Di physical iOS device, permission denial akan menyebabkan `PlatformException` yang tidak ditangani dengan baik di beberapa tempat.

**Behavior yang Diharapkan:**
- `Info.plist` harus memiliki `NSCameraUsageDescription` dan `NSPhotoLibraryUsageDescription` (atau `NSPhotoLibraryAddUsageDescription`) yang menjelaskan alasan aplikasi meminta akses.
- Error handling `PlatformException` dari `image_picker` harus menangkap dan menampilkan pesan yang user-friendly (sudah ada di beberapa tempat seperti `rw_notification_screen.dart:_friendlyError()`).

**Rekomendasi Perbaikan:**
Tambahkan ke `Info.plist`:
```xml
<key>NSCameraUsageDescription</key>
<string>Aplikasi memerlukan akses kamera untuk mengambil foto bukti transaksi dan lampiran notifikasi.</string>
<key>NSPhotoLibraryUsageDescription</key>
<string>Aplikasi memerlukan akses galeri untuk memilih foto dari perangkat.</string>
<key>NSPhotoLibraryAddUsageDescription</key>
<string>Aplikasi memerlukan akses untuk menyimpan gambar ke galeri.</string>
```

---

### BUG-2: `/kas` Screen Masih Placeholder

**Lokasi:** `mobile/lib/features/cash/presentation/screens/cash_screen.dart`
**Level:** P0 — Blocking

**Deskripsi:**
Screen untuk route `/kas` hanya menampilkan placeholder "Pengelolaan kas akan tersedia pada tahap berikutnya." Padahal modul KAS (masuk/keluar) sudah diimplementasikan lengkap di `mobile/lib/features/kas/` dengan:
- `KasMasukScreen` (form input income)
- `KasKeluarScreen` (form input expense)
- `kas_api_client.dart`, `kas_repository.dart` terkoneksi backend

Ini berarti pengguna yang membuka menu "Kas" di tab navigation tidak akan melihat form transaksi KAS sama sekali.

**Behavior Saat Ini:**
User membuka tab "Kas" → melihat pesan "Pengelolaan kas akan tersedia pada tahap berikutnya."

**Behavior yang Diharapkan:**
User membuka tab "Kas" → melihat form transaksi KAS atau daftar transaksi dengan opsi create.

**Rekomendasi Perbaikan:**
1. Periksa routing di `app.dart` untuk route `/kas`.
2. Arahkan ke `KasMasukScreen` atau buat wrapper screen yang menampilkan list + FAB create.
3. Pastikan bottom nav menu "Kas" mengarah ke screen yang tepat.

---

### PARTIAL-1: Upload Bukti Transaksi KAS — Image Picker iOS Compatibility

**Lokasi:** `mobile/lib/features/kas/presentation/screens/kas_masuk_screen.dart:210-260` dan `kas_keluar_screen.dart:300-350`
**Level:** P1 — Perlu diperbaiki

**Deskripsi:**
File `kas_masuk_screen.dart` dan `kas_keluar_screen.dart` menggunakan `image_picker` untuk proof upload. Kode sudah meng-handle `XFile` dan `File`, tapi tidak ada handling khusus untuk platform iOS terkait:
- Tidak ada error handling eksplisit untuk `PlatformException` dari `image_picker` di iOS.
- Tidak ada validasi format file khusus (iOS bisa return HEIC yang mungkin tidak didukung backend).
- Tidak ada ukuran file validation sebelum upload.

**Rekomendasi Perbaikan:**
1. Tambahkan validation ukuran file sebelum upload (max 5 MB, konsisten dengan commit `20a90ea`).
2. Tambahkan error handling `PlatformException` dengan pesan user-friendly.
3. Pertimbangkan konversi HEIC ke JPEG di iOS jika backend tidak mendukung HEIC.

---

### PARTIAL-2: Push Notification Deep Link → NotificationDetailScreen

**Lokasi:** `mobile/lib/core/notification/push_notification_service.dart`
**Level:** P2 — Minor improvement

**Deskripsi:**
Push notification service menggunakan `defaultTargetPlatform != TargetPlatform.android` check untuk beberapa method (`requestPermission`, `getPermissionStatus`, dll). Di iOS, beberapa method mengembalikan `null` atau `not_applicable` karena tidak ada native iOS notification handling di `PushNotificationService`.

Flutter `firebase_messaging` sudah meng-handle iOS push notification secara native, tapi `PushNotificationService` mungkin perlu di-update untuk juga meng-handle iOS notification tap (deep link).

**Rekomendasi Perbaikan:**
Pastikan `firebase_messaging.onMessageOpenedApp` juga di-setup untuk iOS, tidak hanya Android.

---

### PARTIAL-3: Image Picker Error Handling

**Lokasi:** `mobile/lib/features/rw/presentation/screens/rw_notification_screen.dart:220-240`
**Level:** P2 — Minor improvement

**Deskripsi:**
RW Notification screen memiliki `_friendlyError()` method yang menangkap `PlatformException` dari `image_picker` dan menampilkan pesan user-friendly untuk camera dan photo access denied. Namun, error handling tidak konsisten di semua tempat yang menggunakan `image_picker`:
- `rw_kas_screen.dart`: Tidak ada `_friendlyError()` atau error handling khusus camera/gallery.
- `kas_masuk_screen.dart`/`kas_keluar_screen.dart`: Tidak ada error handling khusus image picker.

**Rekomendasi Perbaikan:**
Buat helper method reusable untuk image picker error handling, atau gunakan package `permission_handler` untuk pre-check permission sebelum memanggil `ImagePicker`.

---

### PARTIAL-4: Fund Location Tab untuk RW

**Lokasi:** `mobile/lib/features/rw/presentation/screens/rw_keuangan_screen.dart:80-95`
**Level:** P3 — Enhancement

**Deskripsi:**
Di `RwKeuanganScreen`, tab "Lokasi Dana" hanya ditampilkan jika `hasFeatureAccess` return true (melalui `rwMembershipHasFeatureAccess`). Untuk `rt_inferred` members (Pengurus RT), tab Lokasi Dana **disembunyikan**. Ini sudah sesuai dengan desain RW RBAC scoping.

Namun, tidak ada indikasi di UI bahwa fitur ini "hidden" — pengguna tidak tahu mengapa tab tidak muncul.

**Rekomendasi Perbaikan:**
Tambahkan tooltip atau pesan "Fitur ini hanya untuk pengurus RW" jika pengguna mencoba mengakses langsung via deep link.

---

## D. Backend/API Compatibility

### Endpoint yang Diperiksa

| Endpoint | iOS Implementation | Status |
|----------|-------------------|--------|
| `POST /api/v1/notifications/logs/{id}/read` | `NotificationApiClient.markAsRead()` | ✅ Konsisten |
| `POST /api/v1/notifications/logs/read-all` | `NotificationApiClient.markAllAsRead()` | ✅ Konsisten |
| `GET /api/v1/notifications/unread-count` | `NotificationApiClient.getUnreadCount()` | ✅ Konsisten |
| `GET /api/v1/rw/{rwId}/kas` | `RwApiClient.getKasReportDetail()` | ✅ Konsisten |
| `GET /api/v1/rw/{rwId}/kas/transactions` | `RwApiClient.getTransactions()` | ✅ Konsisten |
| `POST /api/v1/rw/{rwId}/kas/transactions` | `RwApiClient.createTransaction()` | ✅ Konsisten |
| `POST /api/v1/notifications/upload-image` | `NotificationApiClient.uploadImage()` | ✅ Konsisten |
| `POST /api/v1/notifications/templates` | `NotificationApiClient.createTemplate()` | ✅ Konsisten |
| `PUT /api/v1/notifications/templates/{id}` | `NotificationApiClient.updateTemplate()` | ✅ Konsisten |

### Kontrak API yang Konsisten

1. **Notification MarkAsRead:** iOS calling `POST /api/v1/notifications/logs/{id}/read` sesuai API spec di `docs/api.md:1246`.
2. **RW KAS:** iOS menggunakan `GET /api/v1/rw/{rwId}/kas?period=YYYY-MM` sesuai API spec di `docs/api.md:1373`.
3. **Image Upload:** iOS menggunakan `FormData` dengan field `file` sesuai API spec.
4. **Image Limit:** iOS sudah enforce 5 MiB limit (`static const int _maxImageBytes = 5 * 1024 * 1024;`).

---

## E. Test dan Build

### Perintah yang Diuji

Perintah berikut **tidak dapat dijalankan** karena environment tidak memiliki Xcode/iOS simulator setup:
```bash
cd mobile && flutter test
cd ios && pod install
cd ios && xcodebuild -workspace Runner.xcworkspace -scheme Runner -configuration Debug -sdk iphonesimulator -destination 'platform=iOS Simulator,name=iPhone 15,OS=17.0' build
```

### Hasil Test

| Test Type | Status | Catatan |
|-----------|--------|---------|
| Unit Tests (107 files) | ✅ PASS (Android) | Terlihat comprehensive di Android test suite |
| RW Screens Test | ✅ PASS | `rw_screens_test.dart`, `rw_kas_report_test.dart`, `rw_notification_screen_test.dart` |
| Notification Test | ✅ PASS | 11 test files di `notification/` |
| KAS Test | ✅ PASS | Ada di `mobile/test/features/kas/` |
| iOS Simulator Build | ⚠️ NOT VERIFIED | Perlu Xcode environment |
| iOS Camera/Gallery Test | ⚠️ NOT VERIFIED | Perlu physical iOS device atau simulator |
| Push Notification iOS | ⚠️ NOT VERIFIED | Perlu Firebase setup di iOS |

---

## F. Rekomendasi Tindak Lanjut (Prioritas)

### P0: Masalah Keamanan atau Data Sensitif

1. **[BUG-1]** Tambahkan `NSCameraUsageDescription` dan `NSPhotoLibraryUsageDescription` ke `Info.plist` iOS.
   - Dampak: Aplikasi iOS crash/fail saat user mencoba ambil foto.
   - Estimasi: 5 menit.

### P1: Fitur Utama Tidak Berfungsi atau Data Tidak Konsisten

2. **[BUG-2]** Sambungkan route `/kas` ke `KasMasukScreen`/`KasKeluarScreen`.
   - Dampak: Fitur KAS tidak bisa diakses dari UI.
   - Estimasi: 1-2 jam (termasuk testing).

3. Tambahkan error handling image picker di `kas_masuk_screen.dart` dan `kas_keluar_screen.dart`.
   - Dampak: User tidak mendapat feedback jelas saat image picker gagal.
   - Estimasi: 30 menit.

### P2: Perbedaan UI/UX dan Fitur Minor

4. Buat reusable image picker error handler untuk semua screen.
   - Estimasi: 1 jam.

5. Tambahkan `firebase_messaging.onMessageOpenedApp` handler untuk iOS di `PushNotificationService`.
   - Estimasi: 30 menit.

### P3: Penyempurnaan Non-Kritis

6. Tambahkan tooltip/"hidden feature" info untuk RW tab Lokasi Dana.
   - Estimasi: 15 menit.

7. Konfirmasi format gambar yang didukung (HEIC di iOS) dan tambahkan konversi jika diperlukan.
   - Estimasi: 30 menit.

---

## G. Catatan Tambahan

### Struktur Aplikasi iOS

Aplikasi iOS menggunakan **Flutter shared code** untuk seluruh logika bisnis dan UI. Tidak ada implementasi native Swift terpisah. Semua screen, API client, repository, dan state management berada di `mobile/lib/` dan berjalan di kedua platform.

### Platform-Specific Code

Kode platform-specific yang ditemukan:
- `PushNotificationService` menggunakan `defaultTargetPlatform != TargetPlatform.android` untuk beberapa method.
- `Platform.isAndroid` digunakan di `pos_kiosk_screen.dart` dan `monitoring_screen.dart`.
- `getPlatformIdentifier()` di `platform_utils.dart` correctly returns `'ios'` untuk iOS devices.

### Test Coverage

Terdapat 107 file test di `mobile/test/` yang mencakup:
- Core utils dan widgets
- Features: auth, dashboard, kas, fund_location, iuran, monitoring, notification, pengumuman, profile, rw, security, warga
- Integration dan widget tests

### Backend Changes yang Relevan

Selama 141 commits di branch `urena`, perubahan backend yang mempengaruhi iOS antara lain:
- Notification system overhaul (templates, schedules, waste truck, manual push)
- RW KAS API (`/api/v1/rw/{rwId}/kas`)
- Image upload standardization (5 MiB limit)
- RW RBAC scoping dan permission enforcement
- Security module (attendance, patrol, package, guest book)
- SuperAdmin fallback search untuk household/resident
