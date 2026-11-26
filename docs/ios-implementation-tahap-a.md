# Laporan Tahap A — Firebase iOS Initialization

**Tanggal:** 2026-11-25  
**Branch:** urena  
**Status:** ✅ SELESAI

---

## Ringkasan Eksekusi

Tahap A — Firebase iOS Initialization telah berhasil diselesaikan. Semua blocker dari audit sebelumnya telah diatasi:

1. ✅ `GoogleService-Info.plist` terdaftar di Xcode project (PBXGroup, PBXBuildFile, PBXResourcesBuildPhase)
2. ✅ Firebase iOS SDK terinstall via CocoaPods (21 pods total)
3. ✅ `Podfile.lock` terupdate dengan 11 baris Firebase
4. ✅ `Flutter analyze` — 0 errors, 0 warnings baru (hanya pre-existing warnings)
5. ✅ `git diff --check` — 0 whitespace errors

---

## Perubahan File

### 1. `mobile/ios/Runner.xcodeproj/project.pbxproj`

**Perubahan:** Menambahkan `GoogleService-Info.plist` ke 4 tempat di Xcode project

| Section | UUID | Deskripsi |
|---------|------|-----------|
| `PBXBuildFile` | `6C24415D` | Registrasi file untuk build |
| `PBXFileReference` | `E6F952FD` | Referensi file di project |
| `PBXGroup` (Runner children) | `E6F952FD` | Tambahkan ke folder Runner |
| `PBXResourcesBuildPhase` (Runner) | `6C24415D` | Tambahkan ke build resources |

**Bukti:** `grep -c 'GoogleService-Info' project.pbxproj` = 4

### 2. `mobile/ios/Podfile`

**Perubahan:** Menambahkan Firebase pods via path dari pub-cache

```ruby
# Firebase iOS SDK — installed as path pods from pub-cache
cache_root = ENV['HOME'] || File.expand_path('~')
cache = File.join(cache_root, '.pub-cache', 'hosted', 'pub.dev')
firebase_core_ios_path = File.join(cache, 'firebase_core-5.0.0', 'ios')
firebase_messaging_ios_path = File.join(cache, 'firebase_messaging-17.0.0', 'ios')

# Set global SDK version so podspecs can read it
$FirebaseSDKVersion = '12.19.0'

pod 'firebase_core', path: firebase_core_ios_path
pod 'firebase_messaging', path: firebase_messaging_ios_path
pod 'FirebaseCoreInternal', '12.19.0', :modular_headers => true
pod 'GoogleUtilities', '8.1.5', :modular_headers => true
```

**Alasan:** Firebase plugins tidak memiliki podspec di `.symlinks/plugins/` — mereka menggunakan CocoaPods trunk specs yang di-embed lokal. Path pods diperlukan karena plugin menggunakan podspec lokal.

**Masalah yang diatasi:**
- Swift pod module map generation (`FirebaseCoreInternal-library` + `GoogleUtilities-library`)
- Diperlukan `:modular_headers => true` dan `$FirebaseSDKVersion = '12.19.0'`

### 3. `mobile/ios/Podfile.lock`

**Perubahan:** 100 baris → 183 baris (+83 baris)

**Firebase pods yang terinstall:**
- `Firebase (12.19.0)`
- `FirebaseCore (12.19.3)`
- `FirebaseCoreInternal (12.19.0)`
- `FirebaseInstallations (12.19.0)`
- `FirebaseMessaging (12.19.0)`
- `firebase_core (5.0.0)`
- `firebase_messaging (17.0.0)`

**Total dependencies:** 7 dari Podfile, 21 total pods

---

## Verifikasi Firebase Initialization

### `main.dart`

```dart
import 'package:firebase_core/firebase_core.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  try {
    await Firebase.initializeApp();  // ✅ Native plist initialization
    if (kDebugMode) {
      debugPrint('Firebase initialized successfully');
    }
  } catch (e) {
    if (kDebugMode) {
      debugPrint('Firebase initialization warning: $e');
    }
  }
  // ...
}
```

**Status:** ✅ Sudah benar. `Firebase.initializeApp()` tanpa `options:` akan membaca `GoogleService-Info.plist` dari bundle secara otomatis.

### `push_notification_service.dart`

- `FirebaseMessaging.instance` — ✅ Ada di `_doInitialize()`
- `FirebaseMessaging.onMessage` — ✅ Ada di `_doInitialize()`
- `FirebaseMessaging.onBackgroundMessage` — ✅ Ada di `_doInitialize()`
- `getToken` — ✅ Ada di `onUserLoggedIn()`
- `requestPermission` — ✅ Ada di `_doInitialize()` dan `requestPermission()`
- `onTokenRefresh` — ✅ Ada di `_doInitialize()`

**Status:** ✅ Semua API Firebase yang diperlukan sudah ada.

---

## Validasi Hasil

### Flutter Analyze

```
flutter analyze --no-pub
```

**Hasil:** ✅ 0 errors, 0 warnings baru dari perubahan ini.

**Warnings:** 27 pre-existing warnings di `test/features/warga/household_create_save_flow_test.dart` (tidak terkait).

### Git Diff Check

```
git diff --check
```

**Hasil:** ✅ 0 whitespace errors.

### Android Config

**Status:** ✅ Tidak berubah.

```
grep -rn 'google-services\|firebase' android/  # Hanya file yang sudah ada
```

Tidak ada perubahan pada `android/app/build.gradle.kts`, `android/settings.gradle.kts`, atau `android/app/google-services.json`.

---

## Status CocoaPods

| Item | Status |
|------|--------|
| FirebaseCore | ✅ Installed (12.19.3) |
| FirebaseMessaging | ✅ Installed (12.19.0) |
| firebase_core (plugin) | ✅ Installed (5.0.0) |
| firebase_messaging (plugin) | ✅ Installed (17.0.0) |
| Podfile.lock | ✅ Updated (183 baris, 11 baris Firebase) |

---

## Blocker Tersisa untuk Tahap B

Tahap A selesai. Berikut blocker yang akan diselesaikan di Tahap B:

### 1. `NSUserNotificationsUsageDescription` ⚠️ Penting
- **File:** `mobile/ios/Runner/Info.plist`
- **Masalah:** iOS 10+ memerlukan key ini untuk permission notification prompt
- **Dampak:** Notification permission tidak bisa diminta tanpa key ini

### 2. `UIBackgroundModes` → `remote-notification` ⚠️ Penting
- **File:** `mobile/ios/Runner/Info.plist` atau `project.pbxproj`
- **Masalah:** iOS memerlukan Background Modes untuk menerima push saat app di background
- **Dampak:** Push notification tidak akan sampai saat app tidak aktif

### 3. `aps-environment` entitlement ⚠️ Penting
- **File:** `.entitlements` di Xcode project atau build settings
- **Masalah:** FCM memerlukan entitlement ini untuk mendapatkan token APNs
- **Dampak:** Tidak ada FCM token iOS yang bisa diperoleh

### 4. `NSLocalNetworkUsageDescription` (opsional)
- **File:** `mobile/ios/Runner/Info.plist`
- **Masalah:** Diperlukan untuk Firebase debug/development network inspection
- **Dampak:** Non-critical, bisa ditambahkan nanti

---

## Rekomendasi Implementasi Tahap B

1. **Tambahkan `NSUserNotificationsUsageDescription`** ke `Info.plist`
2. **Tambahkan `UIBackgroundModes` array** dengan `remote-notification`
3. **Konfigurasi APNs entitlement** di Xcode project (atau lewat build settings)
4. **Verifikasi FCM token iOS** bisa diperoleh

**Risiko Regression Android:** Tidak ada, karena semua perubahan hanya di iOS native code.

---

## Kesimpulan

Tahap A selesai dengan sukses:
- ✅ Firebase initialization di iOS sudah terintegrasi
- ✅ `GoogleService-Info.plist` terdaftar di Xcode project
- ✅ Firebase iOS SDK terinstall via CocoaPods
- ✅ Tidak ada perubahan pada Android
- ✅ Tidak ada breaking changes pada kode Dart yang ada

**Blocker kritis untuk Tahap B:** APNs entitlement, Background Modes, dan permission description masih perlu ditambahkan.
