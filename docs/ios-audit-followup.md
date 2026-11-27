# Audit iOS Readiness — Follow-Up Verification

Project: Social Finance Urena  
Branch: `urena`  
Audit follow-up date: 2025-01-14  
Status: Read-only, no code changes

---

## 1. Temuan yang Sudah Terverifikasi

### A.1 Firebase — Konfigurasi Android vs iOS

| Item | Android | iOS | Status |
|------|---------|-----|--------|
| `google-services.json` | ✅ Ada di `android/app/google-services.json` | ❌ Tidak ada | BLOCKER |
| `firebase_options.dart` | ❌ Tidak ada (di mana pun) | ❌ Tidak ada | BLOCKER |
| Gradle plugin `com.google.gms.google-services` | ✅ Ada di `android/app/build.gradle.kts` | N/A | Android OK |
| `Firebase.initializeApp()` di `main.dart` | ✅ Tanpa arguments (baca dari google-services.json) | ❌ Akan crash — tidak ada config iOS | BLOCKER |
| `firebaseMessagingBackgroundHandler` | ✅ Ada dengan `@pragma('vm:entry-point')` | ✅ Ada, sama (Android-only pragma) | Perlu pragma iOS juga |
| Firebase init di PushNotificationService | ✅ Graceful fallback (`Firebase.apps.isEmpty` check) | ❌ Akan fail — tidak ada config | Partial mitigated |

**Kesimpulan Firebase:**
- Android berjalan tanpa `firebase_options.dart` karena Flutter Android secara otomatis membaca `google-services.json` saat build.
- iOS **TIDAK** punya mekanisme sama — perlu `GoogleService-Info.plist` di `ios/Runner/` **dan** `firebase_options.dart` untuk dipanggil dari `main.dart` atau `PushNotificationService`.
- Firebase project untuk iOS **harus berbeda** — bukan mengganti konfigurasi Android. Perlu membuat app iOS terpisah di Firebase Console yang sama (atau project berbeda).
- **Tidak ada konfigurasi Firebase yang perlu mengganti Android.** iOS hanya perlu konfigurasi tambahan.

### A.2 Platform Detection — Peta Lengkap Hardcode

**Hardcoded `'android'` sebagai string platform (device registration):**

| File | Line | Kode | Dampak |
|------|------|------|--------|
| `lib/core/notification/push_notification_service.dart` | 392 | `platform: 'android'` (di `onUserLoggedIn`) | Token FCM didaftarkan sebagai `android` di backend |
| `lib/core/notification/push_notification_service.dart` | 440 | `platform: 'android'` (di `_onTokenRefreshed`) | Token refresh juga selalu `android` |
| `lib/core/api/device_service.dart` | 14 | `String platform = 'android'` (default param) | Semua call tanpa argumen platform = `android` |

**Platform check `TargetPlatform.android` (Android-only features):**

| File | Line | Kode | Dampak iOS |
|------|------|------|-----------|
| `lib/core/notification/push_notification_service.dart` | 232 | `showHeadsUpNotification` guard | Return `NOT_SUPPORTED_PLATFORM` — ✅ Benar |
| `lib/core/notification/push_notification_service.dart` | 270 | `areNotificationsEnabled()` guard | Return `true` — ✅ Benar |
| `lib/core/notification/push_notification_service.dart` | 281 | `checkNotificationPermissionStatus()` | Return `'not_applicable'` — ✅ Benar |
| `lib/core/notification/push_notification_service.dart` | 292 | `getInitialNativeDeepLink()` | Return `null` — ✅ Benar |
| `lib/core/notification/push_notification_service.dart` | 322 | `getInitialNativeNotificationPayload()` | Return `null` — ✅ Benar |
| `lib/app/app.dart` | 722 | `defaultTargetPlatform == TargetPlatform.android` | Logika heads-up — perlu review konteks |

**Platform check `Platform.isAndroid` (dart:io):**

| File | Line | Fungsi | Dampak |
|------|------|--------|--------|
| `lib/features/security/presentation/screens/pos_kiosk_screen.dart` | 1490 | Pos kiosk screen | Android-only kiosk mode — iOS tidak terdampak |
| `lib/features/monitoring/presentation/screens/monitoring_screen.dart` | 214 | Monitoring screen | Android-only feature — iOS tidak terdampak |

**Kesimpulan Platform Detection:**
- **Backend sudah support `ios`** — migration 022: `CHECK (platform IN ('android', 'ios', 'web'))`
- Backend handler (`handler.go`) tidak memvalidasi nilai platform — hanya menyimpan apa yang dikirim.
- **Strategi aman:** Ganti hardcode `'android'` dengan platform detection dinamis via `defaultTargetPlatform` atau `kIsWeb`.
- **Penting:** Tidak perlu `import 'dart:io'` di Flutter platform detection — gunakan `foundation.dart` (`defaultTargetPlatform`, `kIsWeb`, `TargetPlatform`).

### A.3 Secure Storage — AndroidOptions Unconditional

```dart
// lib/core/storage/app_storage.dart, line 357-364
FlutterSecureAuthStorage({FlutterSecureStorage? storage})
  : _storage =
        storage ??
        const FlutterSecureStorage(
          aOptions: AndroidOptions(
            resetOnError: true,
          ),
        );
```

**Verifikasi:**
- `AndroidOptions(resetOnError: true)` digunakan **secara unconditional** — tidak ada kondisi platform.
- Tidak ada `IOSOptions` counterpart.
- `flutter_secure_storage` v11+ di iOS menggunakan **Keychain** secara default, tetapi constructor ini **hanya** mendefinisikan `AndroidOptions`.
- Di iOS, `AndroidOptions` tidak dikenali oleh plugin dan akan menyebabkan **compile error** atau **runtime crash**.

**Konfigurasi iOS yang diperlukan:**

```dart
const FlutterSecureStorage(
  aOptions: AndroidOptions(resetOnError: true),
  aOptions: IosSecureStorageOptions(
    accessibility: KeychainAccessibility.first_unlock_this_device,
    inclusionAllowed: true,
  ),
);
```

Atau gunakan conditional import untuk platform-specific options.

**Dampak update aplikasi:**
- Keychain iOS **survive app updates** secara default — token tidak akan hilang.
- `accessibility: KeychainAccessibility.first_unlock_this_device` adalah pilihan yang tepat (token survive reboot pertama, tapi tidak survive device erase).
- Jika upgrade dari Android build yang sama, Keychain iOS akan kosong di第一次 launch — user perlu login ulang. Ini **diharapkan**.

### A.4 Entitlements & Signing & Capabilities

| Item | Status | Detail |
|------|--------|--------|
| CODE_SIGN_STYLE | Automatic | ✅ |
| DEVELOPMENT_TEAM | `X7S23439Y6` | ✅ Sudah dikonfigurasi |
| CODE_SIGN_IDENTITY | `iPhone Developer` (sdk=iphoneos*) | ✅ |
| CODE_SIGN_ENTITLEMENTS | ❌ Tidak ada | Perlu dibuat |
| aps-environment entitlement | ❌ Tidak ada | Diperlukan untuk push notification |
| Background Modes — Remote notifications | ❌ Tidak ada | Diperlukan untuk FCM background |
| Info.plist NSUserActivityTypes | ❌ Tidak ada | Tidak relevan untuk FCM |

**APNs Configuration:**
- **Simulator:** FCM tidak bisa mengirim push ke iOS simulator — hanya ke perangkat fisik. Push notification testing di simulator akan fail.
- **Perangkat fisik:** Perlu:
  1. `aps-environment` entitlement di `.entitlements`
  2. Push Notification capability di Xcode Signing & Capabilities
  3. APNs Key atau Certificate di Firebase Console
  4. `GoogleService-Info.plist` yang benar

### A.5 Bundle Identifier — Full Audit

| Target | Bundle ID | Signing Style | Team |
|--------|-----------|---------------|------|
| Runner (Debug) | `com.example.socialFinance` | Automatic | `X7S23439Y6` |
| Runner (Release) | `com.example.socialFinance` | Automatic | `X7S23439Y6` |
| RunnerTests | `com.example.socialFinance.RunnerTests` | Automatic | `X7S23439Y6` |

**Rekomendasi Bundle ID permanen:**
- Tidak bisa menentukan Bundle ID yang tepat tanpa akses ke Apple Developer Account.
- Pola yang disarankan: `com.[company].socialfinance.[variant]` misalnya `com.socfin.urenapp` atau `com.example.socialfinance` jika ini memang intended.
- RunnerTests biasanya tidak perlu diubah (bisa tetap `.RunnerTests` suffix).
- **Jika TestFlight hanya untuk Warga:** Bundle ID bisa sama dengan Android atau berbeda — tidak ada impact pada distribusi.

---

## 2. Temuan Audit Sebelumnya yang Kurang Tepat

| Temuan Sebelumnya | Koreksi |
|--------------------|---------|
| "Firebase initialization akan fail" | **Lebih spesifik:** Fail hanya untuk iOS. Android berjalan tanpa `firebase_options.dart` karena Google Services plugin. iOS perlu `GoogleService-Info.plist` **dan** `firebase_options.dart`. |
| "Platform hardcoded 'android' di 6+ tempat" | **Revisi:** 3 tempat hardcoded string platform (`'android'`), 6 tempat platform check `TargetPlatform.android` (sebagian besar guard benign untuk iOS, return early). |
| "flutter_secure_storage crash di iOS" | **Lebih spesifik:** Crash terjadi karena `AndroidOptions` tanpa `IOSOptions` di constructor. Bukan karena plugin tidak support iOS — plugin support iOS, tapi konfigurasi incomplete. |
| "Tidak ada backend platform validation" | **Koreksi:** Backend migration sudah memiliki `CHECK (platform IN ('android', 'ios', 'web'))` — backend sudah menerima `ios`. Handler tidak memvalidasi nilai, tapi tidak perlu karena CHECK constraint di DB menangani validasi. |
| "Warga Only otomatis dari TestFlight" | **Peringatan:** TestFlight hanya membatasi siapa yang bisa install APK/IPA, bukan membatasi role. Akun Pengurus/Bendahara yang punya akun TestFlight bisa login dan akses fitur admin. |

---

## 3. File dan Baris Terdampak

### Blocker (P0) — Wajib Diubah Sebelum Build iOS

| # | File | Line(s) | Perubahan |
|---|------|---------|-----------|
| 1 | `ios/Runner/GoogleService-Info.plist` | — | **Buat baru** — export dari Firebase Console |
| 2 | `lib/firebase_options.dart` | — | **Buat baru** — via `flutterfire configure` atau manual |
| 3 | `lib/main.dart` | 15 | Ganti `Firebase.initializeApp()` → `Firebase.initializeApp(name: defaultFirebaseAppName, options: DefaultFirebaseOptions.currentPlatform)` |
| 4 | `lib/core/notification/push_notification_service.dart` | 392, 440 | Ganti `platform: 'android'` → platform detection |
| 5 | `lib/core/api/device_service.dart` | 14 | Ganti default `String platform = 'android'` → platform detection |
| 6 | `lib/core/storage/app_storage.dart` | 357-364 | Tambahkan `IOSOptions` ke constructor |
| 7 | `ios/Runner/Runner.entitlements` | — | **Buat baru** — `aps-environment` entitlement |
| 8 | `ios/Runner.xcodeproj/project.pbxproj` | — | Tambahkan Signing & Capabilities untuk Push Notifications |
| 9 | `ios/Runner/Info.plist` | — | Tambahkan permission keys |
| 10 | `ios/Runner.xcodeproj/project.pbxproj` | — | Ganti Bundle ID |

### Non-Blocker (P1/P2) — Penyempurnaan

| # | File | Line(s) | Perubahan |
|---|------|---------|-----------|
| 11 | `lib/app/app.dart` | 722 | Review context `TargetPlatform.android` check |
| 12 | `ios/Podfile` | 1 | Uncomment `platform :ios, '15.0'` atau sesuaikan |
| 13 | `ios/Runner.xcodeproj/project.pbxproj` | — | Verifikasi `IPHONEOS_DEPLOYMENT_TARGET` konsisten |
| 14 | `ios/Runner/Info.plist` | — | Tambahkan URL scheme untuk deep link |

---

## 4. Rencana Implementasi dengan Urutan Aman

### Phase 1: Firebase Setup (Estimated: 2-3 jam)

1. Buat Firebase project baru atau app iOS di project Firebase yang sama dengan Android
2. Download `GoogleService-Info.plist` — simpan di `ios/Runner/GoogleService-Info.plist`
3. Generate `lib/firebase_options.dart` via `flutterfire configure` atau manual
4. Update `lib/main.dart` untuk load `firebase_options.dart`
5. Test `Firebase.initializeApp()` di iOS simulator (akan fail untuk FCM tapi init OK)

### Phase 2: Platform Detection Refactor (Estimated: 4-6 jam)

6. Di `lib/core/api/device_service.dart`:
   - Tambahkan import `package:flutter/foundation.dart`
   - Buat helper function: `String getPlatformString() => kIsWeb ? 'web' : (defaultTargetPlatform == TargetPlatform.android ? 'android' : 'ios')`
   - Ganti default param `platform` dari `'android'` → panggil helper
7. Di `lib/core/notification/push_notification_service.dart`:
   - Ganti kedua `platform: 'android'` → gunakan helper function
   - Ganti `TargetPlatform.android` checks → `defaultTargetPlatform != TargetPlatform.ios` untuk iOS-specific logic
8. Commit perubahan

### Phase 3: Secure Storage iOS Options (Estimated: 1-2 jam)

9. Di `lib/core/storage/app_storage.dart`:
   - Tambahkan `IOSOptions` atau `IosSecureStorageOptions` ke constructor
   - Pertimbangkan conditional platform check
10. Test secure storage di iOS simulator

### Phase 4: Entitlements & Signing (Estimated: 2-3 jam)

11. Buat `ios/Runner/Runner.entitlements` dengan `aps-environment`
12. Di Xcode: Signing & Capabilities → Add Capability → Push Notifications
13. Add Background Mode → Remote Notifications
14. Test push notification di perangkat fisik

### Phase 5: Bundle ID & Info.plist (Estimated: 1-2 jam)

15. Ganti Bundle ID di project.pbxproj
16. Tambahkan permission keys di Info.plist
17. Archive & TestFlight

---

## 5. Test yang Diperlukan

### Pre-Implementation Tests

| # | Test | Cara | Expected |
|---|------|------|----------|
| 1 | `Firebase.initializeApp()` di iOS simulator | Run app di iOS simulator | Init succeed (tanpa FCM) |
| 2 | `flutter analyze` di iOS | `flutter analyze` | Tidak ada error baru |
| 3 | `flutter pub get` | `cd mobile && flutter pub get` | Success |
| 4 | Pod install | `cd ios && pod install` | Success |

### Post-Implementation Tests

| # | Test | Cara | Expected |
|---|------|------|----------|
| 5 | Login + session restore di iOS | Login → kill app → reopen | Token survive, auto-login |
| 6 | Push notification registration | Login di iOS perangkat fisik | `platform: 'ios'` di backend DB |
| 7 | Push notification foreground | Kirim FCM test push | Notification muncul di app |
| 8 | Push notification background | App di background → kirim push | Banner muncul |
| 9 | Push notification terminated | App closed → kirim push | Tap notification → app open |
| 10 | Device token refresh | App running → token refresh | Backend update token dengan platform `ios` |
| 11 | Logout + unregister | Logout | Device unregistered dari backend |
| 12 | Secure storage survival | Login → background → reopen → kill → reopen | Session survive semua lifecycle |
| 13 | Camera permission | Buka QR scanner | Permission dialog muncul, OK |
| 14 | Photo library | Image picker from gallery | Permission dialog muncul, OK |
| 15 | Share sheet | Share feature | iOS share sheet muncul |

---

## 6. Risiko Regresi Android

### Analisis Keamanan Android

| Aspek | Perubahan | Risiko | Mitigasi |
|-------|-----------|--------|----------|
| Firebase init | Tambahan options | 🟢 **Tidak ada** | `defaultFirebaseOptions.currentPlatform` otomatis pilih config per platform |
| Platform detection | `kIsWeb`, `defaultTargetPlatform` | 🟢 **Tidak ada** | Flutter API yang sama di semua platform |
| Secure storage | Tambahan `IOSOptions` | 🟢 **Tidak ada** | `AndroidOptions` tidak terpengaruh jika pakai conditional atau multi-option |
| Device registration | Platform dinamis | 🟢 **Tidak ada** | Backend sudah support `ios`, `android` tetap diterima |
| Notification feature | Platform guard | 🟢 **Tidak ada** | `TargetPlatform.android` check tetap sama untuk Android |

**Kesimpulan:** Perubahan yang disarankan **tidak akan mempengaruhi Android**. Flutter API `kIsWeb` dan `defaultTargetPlatform` mengembalikan nilai yang benar untuk setiap platform secara otomatis.

### Hal yang Perlu Tetap Dipertahankan

1. **Android tidak boleh kehilangan `google-services.json`** — tidak diubah.
2. **Android tidak boleh kehilangan Firebase initialization tanpa arguments** — akan diubah untuk kedua platform, tapi `firebase_options.dart` akan menyediakan config per-platform.
3. **Android secure storage tetap pakai `AndroidOptions`** — dengan multi-option constructor, Android options tidak berubah.
4. **Android notification platform tetap `'android'`** — platform detection akan mengembalikan `'android'` di Android.

---

## 7. Keputusan yang Butuh Konfigurasi Manual dari Pemilik Akun

### A. Apple Developer Account

| Item | Butuh Aksi | Detail |
|------|-----------|--------|
| Bundle ID | ✅ Register | Daftar Bundle ID di Apple Developer Portal → Certificates, Identifiers & Profiles |
| Push Notifications capability | ✅ Enable | Aktifkan Push Notifications untuk Bundle ID yang didaftarkan |
| APNs Key | ✅ Generate | Buat APNs Key di Apple Developer Portal → Keys → + |
| APNs Key upload | ✅ Upload | Upload APNs Key ke Firebase Console → Project Settings → Cloud Messaging → iOS app config |

### B. Firebase Console

| Item | Butuh Aksi | Detail |
|------|-----------|--------|
| iOS app registration | ✅ Register | Tambahkan app iOS di Firebase project yang sama (atau baru) |
| GoogleService-Info.plist | ✅ Download | Download setelah register iOS app |
| Bundle ID di Firebase | ✅ Input | Input Bundle ID yang sama dengan yang didaftarkan di Apple |
| APNs Key | ✅ Upload | Upload APNs Key dari Apple Developer Portal |

### C. TestFlight (Jika Digunakan)

| Item | Butuh Aksi | Detail |
|------|-----------|--------|
| App Store Connect app | ✅ Create | Buat app baru di App Store Connect dengan Bundle ID yang sama |
| Build upload | ✅ Upload | Archive dari Xcode → Upload |
| TestFlight users | ✅ Add | Tambahkan user TestFlight (Warga yang dituju) |

---

## 8. Warga Only — Koreksi Arsitektur

### Login & Session Restoration

| Stage | Saat Ini | Risiko iOS |
|-------|----------|------------|
| Login | `POST /api/v1/auth/login` — validasi email/phone + password | ✅ Tidak berubah |
| JWT payload | `sub`, `role`, `rt_id` dari JWT claims | ✅ Tidak berubah |
| Session restore | Baca dari secure storage → `GET /api/v1/auth/me` → restore user | ✅ Tidak berubah |
| **Role validation di iOS** | ❌ **Tidak ada** | User non-Warga bisa login via TestFlight |

### Backend Identity Resolution

- Backend tidak memfilter role saat login — semua role bisa login dengan kredensial valid.
- Role di JWT → dikirim ke Flutter → disimpan di secure storage.
- **Tidak ada backend-side role restriction per platform.**

### Role Fallback

- Jika role tidak dikenali (tidak ada di `AppRole` enum), Flutter menggunakan fallback logic di berbagai widget.
- Role yang tidak dikenal cenderung → akses minimal atau deny.
- **Di iOS, role fallback bukan concern** karena hanya Warga yang punya akses TestFlight.

### Navigation Guard

| Guard | Lokasi | Status |
|-------|--------|--------|
| `_getBottomNavTabs()` | `navigation_shell.dart` | Role-based ✅ |
| `_getCollapsedTabs()` | `navigation_shell.dart` | Role-based ✅ |
| `_canAccessKeuangan()` | `navigation_shell.dart` | Permission-based ✅ |
| `_canAccessNotifikasi()` | `navigation_shell.dart` | Role-based ✅ |
| **GoRouter redirect guard** | ❌ Tidak ada | **Risiko:** user bisa navigate langsung via URL |
| **Deep link validation** | `normalizeNotificationTarget()` | Path sanitasi ✅ |

### Direct Route Access Risk

- GoRouter **tidak punya redirect guard** dari route admin ke `/home`.
- Jika user non-Warga bisa mengakses app (melalui TestFlight), mereka bisa membuka URL `home/iuran/form`, `home/kas`, dll secara langsung.
- **Mitigasi yang disarankan:** Tambahkan GoRouter redirect guard yang memvalidasi role user sebelum render halaman.

### Rekomendasi Enforce untuk iOS (Tanpa Mengubah Android)

**Strategi: Environment-based role enforcement di iOS saja.**

1. **Bundle ID berbeda** — iOS pakai Bundle ID berbeda dari Android (misalnya `com.example.socialFinance.ios`).
2. **Environment variable atau conditional compilation** — Di `ios/Runner/AppDelegate.swift`, set `IS_IOS_WAREHOUSE_APP = true` atau similar.
3. **Flutter-side detection** — Buat `PlatformInfoProvider` yang return role restriction based on bundle ID or environment.
4. **Navigation guard override** — Di `navigation_shell.dart`, jika iOS build → force role filter ke "warga-only".
5. **Backend validation tetap berlaku** — Semua API call tetap memvalidasi role di backend.

**Alternatif lebih sederhana:** Karena iOS didistribusikan via TestFlight untuk Warga tertentu, pastikan hanya Warga yang mendapatkan akun TestFlight. Tidak perlu perubahan code.

### Paket Detail Security

- `paket_detail_screen.dart` diakses via deep link dari notifikasi.
- `normalizeNotificationTarget()` sudah memvalidasi payload notifikasi dan route ke path yang benar.
- **Tidak ada perubahan diperlukan** — deep link sudah ter-sanitasi.

---

## 9. iOS Deployment Target — Verifikasi

### Current State

| File | Deployment Target | Status |
|------|-------------------|--------|
| `project.pbxproj` | `IPHONEOS_DEPLOYMENT_TARGET = 13.0` | Ada |
| `Podfile` | `# platform :ios, '15.0'` (commented) | ❌ Not enforced |
| `AppFrameworkInfo.plist` | Ditgenerate Flutter | Tergantung Flutter SDK |
| `Podfile.lock` | Tidak ada (belum run pod install) | — |

### `mobile_scanner` Podspec Requirement

- `mobile_scanner ^6.0.6` — perlu dicek podspec versi yang diinstal.
- Versi 6.x umumnya memerlukan **iOS 14.0** minimum (berdasarkan Flutter plugins modern).
- Jika `mobile_scanner` memerlukan iOS 15+, maka deployment target 13.0 tidak cukup.

### Rekomendasi Deployment Target

Berdasarkan plugin modern dan target TestFlight:

| Target | Value | Alasan |
|--------|-------|--------|
| Minimum iOS | `15.0` | `mobile_scanner` 6.x, Firebase iOS SDK terbaru, fitur modern iOS |
| Podfile | Uncomment `platform :ios, '15.0'` | Konsisten dengan Xcode project |
| Xcode project | Ubah `IPHONEOS_DEPLOYMENT_TARGET` ke `15.0` | Konsisten dengan Podfile |
| `AppFrameworkInfo.plist` | Akan diupdate Flutter secara otomatis | Saat build |

### Tidak Perlu Update Package

- Tidak perlu upgrade `mobile_scanner` atau plugin lain.
- Hanya perlu mengubah deployment target configuration.

---

## 10. Ringkasan Akhir

| Kategori | Status |
|----------|--------|
| Firebase iOS | ❌ Belum ada — butuh GoogleService-Info.plist + firebase_options.dart |
| Backend platform support | ✅ Sudah support `ios` via migration 022 |
| Platform detection | 🔴 3 tempat hardcode `'android'` — perlu refactoring |
| Secure storage | 🔴 AndroidOptions only — perlu IOSOptions |
| Entitlements | 🔴 Tidak ada aps-environment |
| Bundle ID | 🔴 Default `com.example.socialFinance` |
| Deployment target | ⚠️ 13.0 — perlu naik ke 15.0 |
| Navigation guard | ⚠️ Tidak ada GoRouter redirect untuk role enforcement |
| Android regression risk | 🟢 **Tidak ada** — perubahan terisolasi per-platform |

**Estimasi total effort: 2-3 hari kerja** (setelah keputusan Apple/Firebase selesai).
