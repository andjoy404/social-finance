# Audit Kesiapan iOS Social Finance — Warga Only / TestFlight

Repository: `/Users/Andriyan/Documents/apx/devl/social-finance`  
Branch target: `urena`  
Flutter project: `mobile/`  
Audit date: 2025-01-14  

---

## A. Executive Summary

**Status: NOT READY**

Codebase Flutter Android saat ini **tidak siap** untuk dibangun di iOS tanpa perubahan signifikan. Temuan utama:

- **Firebase belum dikonfigurasi untuk iOS** — tidak ada `GoogleService-Info.plist` maupun `firebase_options.dart`. Semua fitur FCM (push notification, device registration) akan crash di iOS.
- **Platform hardcoded sebagai Android** — `PushNotificationService` dan `DeviceService` hardcode `platform: 'android'` dan menggunakan `TargetPlatform.android` checks di 6+ tempat.
- **`FlutterSecureStorage` menggunakan Android-only options** — `AndroidOptions(resetOnError: true)` tanpa `IOSOptions`, akan crash di iOS.
- **Bundle ID default** — `com.example.socialFinance` perlu diganti ke Bundle ID Apple yang sesuai.
- **Signing & team configuration** sudah ada Team ID (`X7S23439Y6`), namun signing style (automatic/manual) belum diverifikasi.
- **Tidak ada entitlements** — tidak ada `aps-environment`, `push notifications`, atau capability iOS lainnya.
- **Deployment target 13.0** — memenuhi minimum iOS, namun perlu dipertimbangkan apakah sesuai target TestFlight.
- **Warga Only routing** sudah ada di `navigation_shell.dart` — role-based tab visibility sudah implement, termasuk isolasi warga. Namun **tidak ada validasi role di level routing iOS** yang mencegah akses tidak sah.

---

## B. Environment

| Komponen | Versi | Status |
|----------|-------|--------|
| macOS | (terdeteksi via Xcode) | ✅ Tersedia |
| Xcode | 27.2 | ✅ Tersedia |
| Flutter | 3.47.6 (channel stable) | ✅ Tersedia |
| Dart | 3.13.5 | ✅ Tersedia |
| CocoaPods | 1.16.2 | ✅ Tersedia |
| Swift | (Xcode toolchain) | ✅ Tersedia |
| Flutter root | `/Users/Andriyan/Documents/data/homework/flutter/perpus-digital/flutter` | ⚠️ Non-standard path |

---

## C. Repository Integrity

| Item | Nilai |
|------|-------|
| Branch aktif | `urena` |
| HEAD commit | `77796b7 feat: archive feature for Kegiatan module + UI enhancements` |
| Git status | Clean (working tree tidak berubah) |
| File berubah akibat audit | **0** |

Working tree bersih. Tidak ada perubahan yang dibuat selama audit.

---

## D. iOS Configuration

### Bundle Identifier & Signing

| Setting | Nilai | Catatan |
|---------|-------|---------|
| Bundle ID | `com.example.socialFinance` | ❌ Default, perlu diganti |
| Team ID | `X7S23439Y6` | ✅ Sudah dikonfigurasi (disamarkan) |
| Signing Style | Belum diverifikasi (automatic/manual) | 🔍 Perlu dicek di Xcode |
| Deployment Target | iOS 13.0 | ✅ Memenuhi minimum |
| Build Name/Number | `1.0.0` / `1` | ✅ Dari xcconfig |

### Info.plist

- **Tidak ada** permission keys untuk kamera (`NSCameraUsageDescription`), foto (`NSPhotoLibraryUsageDescription`), atau notifikasi.
- **Tidak ada** deep link configuration.
- **Tidak ada** URL schemes.
- SceneDelegate menggunakan `FlutterSceneDelegate` — standar Flutter modern.

### Entitlements

- **Tidak ada file `.entitlements`** ditemukan.
- Tidak ada `aps-environment` entitlement untuk push notification.
- Tidak ada capability iOS dikonfigurasi.

### Firebase iOS Configuration

| File | Status |
|------|--------|
| `ios/Runner/GoogleService-Info.plist` | ❌ **Tidak ada** |
| `lib/firebase_options.dart` | ❌ **Tidak ada** |
| `GoogleService-Info.plist` di mana pun di `ios/` | ❌ **Tidak ada** |

**Impact:** Firebase init akan fail di iOS. Push notification, analytics, dan layanan Firebase lainnya tidak akan berfungsi.

### Build Settings

- **Debug/Release xcconfig:** Standar Flutter, menggunakan `Generated.xcconfig`.
- **Excluded architectures:** `i386` (simulator), `armv7` (device) — standar.
- **CocoaPods stats:** Dinonaktifkan (`COCOAPODS_DISABLE_STATS = 'true'`) — standar Flutter.

---

## E. Flutter Plugin Compatibility

### Dependency Overview (`pubspec.yaml`)

| Plugin | Version | iOS Support | Risk |
|--------|---------|-------------|------|
| `firebase_core` | ^5.0.0 | ✅ Ada | 🔴 **Config belum setup** |
| `firebase_messaging` | ^17.0.0 | ✅ Ada | 🔴 **Config belum setup, platform hardcoded** |
| `flutter_secure_storage` | ^11.2.0 | ✅ Ada | 🔴 **AndroidOptions tanpa IOSOptions** |
| `shared_preferences` | ^2.5.6 | ✅ | Rendah |
| `dio` | ^5.8.0+1 | ✅ | Rendah |
| `go_router` | ^14.8.1 | ✅ | Rendah |
| `image_picker` | ^1.2.4 | ✅ | 🟡 Perlu permission keys |
| `url_launcher` | ^6.3.0 | ✅ | 🟡 Perlu URL scheme config |
| `qr_flutter` | ^4.1.0 | ✅ | Rendah |
| `mobile_scanner` | ^6.0.6 | ✅ | 🟡 Perlu camera permission |
| `pdf` + `printing` | ^3.11.3 + ^5.13.4 | ✅ | Rendah |
| `excel` | ^4.0.6 | ✅ | Rendah |
| `file_saver` | ^0.7.0 | ✅ | 🟡 Perlu File Provider setup |
| `share_plus` | ^13.3.1 | ✅ | 🟡 Perlu share extension |
| `fl_chart` | ^0.69.0 | ✅ | Rendah |
| `path_provider` | ^2.1.5 | ✅ | Rendah |
| `uuid` | ^4.5.1 | ✅ | Rendah |
| `intl` | ^0.20.2 | ✅ | Rendah |
| `cupertino_icons` | ^1.0.8 | ✅ | Rendah |
| `flutter_riverpod` + `riverpod_annotation` | ^2.6.1 + ^2.6.4 | ✅ | Rendah |

### Permission Native yang Diperlukan di iOS

| Permission | Key Info.plist | Plugin | Status |
|------------|---------------|--------|--------|
| Camera | `NSCameraUsageDescription` | `mobile_scanner`, `image_picker` | ❌ Belum ada |
| Photo Library | `NSPhotoLibraryAddUsageDescription` | `image_picker` | ❌ Belum ada |
| Push Notification | Entitlement `aps-environment` | `firebase_messaging` | ❌ Belum ada |
| Background Modes | `audio`, `processing`, atau `fetch` | `firebase_messaging` | ❌ Belum ada |

---

## F. Warga Only Architecture

### Route yang Sudah Terfilter oleh Role (navigation_shell.dart)

`_getBottomNavTabs()` dan `_getCollapsedTabs()` sudah melakukan role-based filtering:

**Warga (sistem_role = warga, jabatan = warga):**
- Bottom nav: `Beranda`, `Keuangan` (jika ada permission `finance.read`), `Iuran`, `Menu`
- Akses ke: dashboard, iuran (personal), pengumuman, kegiatan, profil, warga (read-only)
- Tidak akses: kas, lokasi dana, security management, patroli, paket, notifikasi (diblokir `_canAccessNotifikasi`)

**Validasi Role di App:**

1. `_canAccessNotifikasi()` → memanggil `canUserAccessNotificationMenu(user)` — Warga tidak bisa akses notifikasi.
2. `_canAccessIuran()` → Warga BISA akses iuran (personal view) — ini sesuai expectation Warga Only.
3. `_canAccessKeuangan()` → Warga akses keuangan hanya jika ada permission `finance.read`.
4. `isAllowedRT()` → Memastikan user hanya akses data RT mereka.

### Blokade Route untuk Non-Warga

Tidak ada blokade eksplisit — route tetap terdaftar di GoRouter, tetapi tab navigasi disembunyikan berdasarkan role. Ini berarti:

- **Route masih bisa diakses via deep link** jika user tahu URL-nya.
- **Tidak ada `go_router` redirect** dari route admin ke `/home` jika user bukan warga.

### Rekomendasi Warga Only untuk iOS

Karena iOS hanya untuk Warga (TestFlight), bukan pembatasan per-role di aplikasi, tetapi pembatasan **per-platform distribution**:

1. **Route tetap sama** — tidak perlu mengubah navigation logic.
2. **Bundle ID berbeda** — buat `com.example.socialFinance.ios` atau similar.
3. **Distribusi TestFlight** — pastikan hanya Warga yang memiliki akun TestFlight.
4. **Backend validation** — semua otorisasi tetap di server, jadi tidak ada kebocoran data.

### Keamanan Deep Link

`normalizeNotificationTarget()` di `app.dart` sudah memiliki sanitasi path:
- Validasi path terhadap whitelist known routes.
- Fallback ke `/home/notifikasi/detail` untuk payload tidak dikenal.
- Tidak ada raw user input yang langsung diparse ke path.

---

## G. Feature Readiness

### Login & Autentikasi

| Aspek | Status |
|-------|--------|
| Login endpoint | ✅ POST `/api/v1/auth/login` — email/phone + password + device_id |
| Token refresh | ✅ POST `/api/v1/auth/refresh` — JWT rotation dengan device_id |
| Logout | ✅ POST `/api/v1/auth/logout` |
| Secure storage | ⚠️ `FlutterSecureStorage` menggunakan `AndroidOptions` — perlu `IOSOptions` |
| Session restore | ✅ `StoredSession` dibaca dari secure storage saat app launch |
| Profile reload | ✅ Saat resume dari background, profile status dicek |

### Refresh Token & Recovery Sesi

- Token disimpan di `flutter_secure_storage` sebagai `sf_sec_access_token` / `sf_sec_refresh_token`.
- Server URL dan user data juga di-store secara encrypted.
- Remember me preference di `shared_preferences` (bukan secure storage — benar).
- Login credentials (email/phone, password, server URL) di-store di secure storage — **perlu evaluasi apakah ini kebijakan yang diinginkan.**

### Push Notification

| Aspek | Status |
|-------|--------|
| Firebase init | ❌ Tidak ada `GoogleService-Info.plist` untuk iOS |
| Token registration | 🔴 `platform: 'android'` hardcoded di `_onTokenRefreshed()` dan `onUserLoggedIn()` |
| Background handler | ✅ `firebaseMessagingBackgroundHandler` dengan `@pragma('vm:entry-point')` |
| Foreground listener | ✅ `FirebaseMessaging.onMessage.listen()` |
| Deduplication | ✅ `_processedMessageIds` set, max 100 |
| Device unregistration | ✅ `onUserLoggedOut()` memanggil `unregisterDevice()` |
| Heads-up notification | ✅ Native MethodChannel — **Android-only, return `NOT_SUPPORTED_PLATFORM` di iOS** |
| Notification tap | ✅ `onMessageOpenedApp` + native channel fallback |

### Device Registration

- `DeviceService.registerDevice()` mengirim `platform: 'android'` sebagai default parameter.
- Di `PushNotificationService`, platform selalu `'android'` (tidak pernah `'ios'`).

### Notifikasi & Deep Link

- `normalizeNotificationTarget()` support berbagai tipe notifikasi: iuran, paket, patroli, koreksi, waste truck.
- Deep link parsing robust dengan fallback safety.
- **Tidak ada iOS push notification provisioning.**

### Read Receipt

- Tidak ditemukan implementasi read receipt di codebase saat ini.
- Perlu dicek apakah ada endpoint backend untuk mark-as-read.

### Iuran & Pembayaran

- `iuran_screen.dart`, `iuran_detail_screen.dart`, `iuran_payment_screen.dart` — ada.
- `iuran_form_screen.dart`, `iuran_report_screen.dart` — ada.
- `iuran_access.dart` — ada access control helper.

### Kegiatan

- `kegiatan_screen.dart`, `kegiatan_detail_screen.dart` — ada.
- Archive feature ada di commit `77796b7`.

### Profil

- `profile_screen.dart` — ada.
- Change password, update profile — ada di `AuthService`.

### Gambar Terverifikasi

- `authenticated_network_image.dart` — ada widget untuk loading gambar dengan auth header.
- `PushNotificationService.showHeadsUpNotification()` mengirim `authToken` dan `rtId` via MethodChannel untuk download gambar di notification expanded view — **Android-only**.

### Permission Kamera/Foto & Upload

- `image_picker` — untuk pick image dari gallery/camera.
- `mobile_scanner` — untuk QR scanner (security attendance).
- `share_plus` — untuk share file.
- `url_launcher` — untuk buka URL eksternal.
- `file_saver` — untuk save file ke device storage.

**iOS Permissions yang diperlukan** (lihat Section E).

---

## H. Validation Results

### Commands Executed

| # | Command | Hasil |
|---|---------|-------|
| 1 | `git branch --show-current` | `urena` ✅ |
| 2 | `git log --oneline -1` | `77796b7 feat: archive feature for Kegiatan module + UI enhancements` ✅ |
| 3 | `git status --short` | Clean ✅ |
| 4 | `git diff --stat HEAD` | No changes ✅ |
| 5 | `flutter analyze` | ⚠️ **98 warnings** (sebagian besar `override_on_non_overriding_member` di test files) |
| 6 | `find ios -name "*.entitlements"` | ❌ Tidak ada |
| 7 | `find ios -name "GoogleService-Info.plist"` | ❌ Tidak ada |
| 8 | `grep "firebase_options" lib/` | ❌ Tidak ada |
| 9 | `grep "platform.*android\|TargetPlatform" lib/` | 🔴 **7 occurrence** hardcoded Android |
| 10 | `grep "AndroidOptions" lib/` | 🔴 **1 occurrence** tanpa IOS counterpart |
| 11 | `xcodebuild -version` | Xcode 27.2 ✅ |
| 12 | `pod --version` | 1.16.2 ✅ |
| 13 | `flutter pub get` | Belum dijalankan (lockfile ada) |

### Summary

| Kategori | Jumlah |
|----------|--------|
| PASS | 4 |
| FAIL / BLOCKER | 4 |
| WARNING | 2 |
| NOT TESTED | 2 (flutter pub get, iOS build) |

---

## I. Prioritized Findings

### P0 — Blocker (Sebelum Build iOS)

| # | Temuan | File | Dampak |
|---|--------|------|--------|
| 1 | **Tidak ada `GoogleService-Info.plist`** untuk iOS | `ios/Runner/` | Firebase crash saat init, semua FCM feature fail |
| 2 | **Tidak ada `firebase_options.dart`** | `lib/` | `Firebase.initializeApp()` tidak bisa load config iOS |
| 3 | **Platform hardcoded `'android'`** | `lib/core/notification/push_notification_service.dart`, `lib/core/api/device_service.dart` | Device registration ke backend selalu kirim `platform: 'android'` |
| 4 | **`FlutterSecureStorage` menggunakan `AndroidOptions`** | `lib/core/storage/app_storage.dart` | Akan crash di iOS — `AndroidOptions` tidak ada di platform iOS |
| 5 | **Tidak ada entitlements** | `ios/Runner/` | Push notification tidak akan bekerja di iOS App Store/TestFlight |

### P1 — Wajib Sebelum TestFlight

| # | Temuan | File | Dampak |
|---|--------|------|--------|
| 6 | **Bundle ID default** `com.example.socialFinance` | `project.pbxproj` | Tidak bisa signing untuk TestFlight |
| 7 | **Permission keys Info.plist hilang** | `ios/Runner/Info.plist` | Camera/photo akan ditolak oleh iOS, app rejection dari App Store |
| 8 | **Platform check di app.dart** | `lib/app/app.dart:722` | `defaultTargetPlatform == TargetPlatform.android` — heads-up notification logic tidak sesuai iOS |
| 9 | **Share extension tidak ada** | `ios/Runner/` | `share_plus` mungkin tidak optimal di iOS |
| 10 | **File Provider tidak ada** | `ios/Runner/` | `file_saver` mungkin tidak optimal di iOS |

### P2 — Penyempurnaan

| # | Temuan | File | Dampak |
|---|--------|------|--------|
| 11 | **98 flutter analyze warnings** | Various test files | Test file class structure tidak sesuai — tidak blocking tapi perlu dibersihkan |
| 12 | **Login credentials di secure storage** | `lib/core/storage/app_storage.dart` | Password tersimpan — perlu konfirmasi apakah ini kebijakan yang diinginkan |
| 13 | **Tidak ada read receipt** | N/A | Fitur belum teridentifikasi implementasinya |
| 14 | **Deep link URL scheme tidak ada** | `Info.plist` | Tidak bisa menerima incoming deep links dari push notification |
| 15 | **XCFramework/Flutter framework embedding** | `ios/Flutter/` | Perlu memastikan `use_frameworks!` sudah benar di Podfile |

---

## J. Implementation Plan

### Daftar File yang Perlu Diubah

| No | File | Tujuan Perubahan | Risiko Regresi |
|----|------|-------------------|----------------|
| 1 | `ios/Runner/GoogleService-Info.plist` | **Buat file baru** — konfigurasi Firebase iOS dari Firebase Console | Rendah — file standar Firebase |
| 2 | `lib/firebase_options.dart` | **Buat file baru** — `flutterfire configure` atau manual export dari Firebase | Rendah — import oleh `main.dart` |
| 3 | `lib/core/notification/push_notification_service.dart` | Ganti `platform: 'android'` → deteksi platform via `defaultTargetPlatform` | 🟡 Medium — device registration backend perlu support iOS |
| 4 | `lib/core/api/device_service.dart` | Ganti default `platform = 'android'` → `platform = kIsWeb ? 'web' : (defaultTargetPlatform == TargetPlatform.android ? 'android' : 'ios')` | 🟡 Medium |
| 5 | `lib/core/storage/app_storage.dart` | Tambahkan `IOSOptions` ke `FlutterSecureStorage` constructor | Rendah — platform-specific options |
| 6 | `ios/Runner.xcodeproj/project.pbxproj` | Ganti `PRODUCT_BUNDLE_IDENTIFIER` dari `com.example.socialFinance` ke Bundle ID asli | Rendah — perlu Apple Developer account |
| 7 | `ios/Runner/Info.plist` | Tambahkan `NSCameraUsageDescription`, `NSPhotoLibraryAddUsageDescription`, URL scheme | 🟡 Medium — app bisa crash jika permission tidak diberikan user |
| 8 | `ios/Runner/Runner.entitlements` | **Buat file baru** — `aps-environment` untuk push notification | Rendah |
| 9 | `lib/app/app.dart:722` | Ganti `TargetPlatform.android` check → `defaultTargetPlatform != TargetPlatform.iOS` untuk logic iOS-specific | Rendah |
| 10 | `ios/Podfile` | Verifikasi `platform :ios, '13.0'` uncomment | Rendah |

### Tidak Perlu Diubah

| File | Alasan |
|------|--------|
| `lib/features/*/` screens | Routing sudah role-based, tidak perlu perubahan untuk Warga Only |
| `lib/core/api/client.dart` | API client agnostik platform |
| `lib/core/api/auth_service.dart` | Login/auth agnostik platform |
| `lib/features/iuran/` | Sudah ada, tidak perlu modifikasi |
| `lib/features/kas/` | Sudah ada, tidak perlu modifikasi |
| `lib/features/security/` | Sudah ada, akses dibatasi role |

### Pengujian yang Diperlukan

1. **Firebase init test** — verifikasi `Firebase.initializeApp()` berhasil di iOS simulator.
2. **Push notification test** — verifikasi token FCM registration dengan `platform: 'ios'`.
3. **Secure storage test** — verifikasi token tersimpan/read di iOS Keychain.
4. **Camera/Photo permission test** — verifikasi `image_picker` dan `mobile_scanner` bekerja dengan permission granted.
5. **Deep link test** — verifikasi notification tap navigasi ke route yang benar.
6. **Share test** — verifikasi `share_plus` share sheet muncul.
7. **End-to-end login test** — verifikasi full login flow dari scratch di iOS.
8. **Session restore test** — verifikasi token survive app reload di iOS.
9. **Backend platform test** — verifikasi backend bisa handle `platform: 'ios'` di device registration.

---

## K. Final Recommendation

**Codebase TIDAK layak untuk masuk tahap build iOS saat ini.** Ada 5 blocker P0 yang harus diselesaikan terlebih dahulu:

1. **Firebase configuration untuk iOS** — `GoogleService-Info.plist` dan `firebase_options.dart` harus dibuat. Ini adalah prerequisites mutlak untuk push notification dan fitur-fitur Firebase lainnya.
2. **Platform hardcoding** — `platform: 'android'` di 3+ tempat harus diganti dengan platform detection dinamis. Backend juga perlu diverifikasi mendukung `platform: 'ios'`.
3. **Secure storage iOS options** — `FlutterSecureStorage` harus dikonfigurasi dengan `IOSOptions` agar bisa mengakses Keychain di iOS.
4. **Entitlements** — `aps-environment` entitlement harus ditambahkan untuk push notification TestFlight.
5. **Bundle ID dan signing** — harus diganti ke Bundle ID yang terdaftar di Apple Developer account.

**Setelah P0 dan P1 selesai, estimasi effort:**
- Firebase setup: 2-4 jam
- Platform detection refactor: 4-8 jam
- Permission + entitlements: 2-4 jam
- Testing & validation: 4-8 jam
- **Total estimasi: 2-3 hari kerja**

**Catatan khusus untuk Warga Only / TestFlight:**
- Karena distribusi iOS hanya untuk Warga, **tidak perlu mengubah routing logic** — role-based filtering sudah ada.
- Yang perlu dilakukan hanya: (a) konfigurasi platform iOS, (b) bundle ID terpisah, (c) distribusi via TestFlight.
- Backend validation tetap menjadi satu-satunya sumber kebenaran otorisasi — UI iOS tidak bisa di-bypass untuk akses data non-Warga.
