# IMPLEMENTASI TAHAP B — APNs REGISTRATION & PUSH NOTIFICATIONS

Kerjakan konfigurasi APNs (Apple Push Notification service) pada iOS Social Finance.

## Konteks

- Tahap A (Firebase Core + Messaging) sudah selesai: `GoogleService-Info.plist` terdaftar di Xcode, CocoaPods terpasang, Firebase SDK terinstall.
- Remaining blocker untuk push notification iOS: APNs entitlements, notification usage descriptions, background modes.

## Tugas yang Selesai

### 1. Info.plist — NSUserNotificationsUsageDescription

File: `mobile/ios/Runner/Info.plist`

Menambahkan key wajib untuk iOS 10+:

```xml
<key>NSUserNotificationsUsageDescription</key>
<string>BumiCerita perlu izin untuk mengirim notifikasi penting seputar keuangan RT Anda</string>
```

Tanpa key ini, `requestPermission()` pada Firebase Messaging akan gagal silently atau throw exception pada iOS 10+.

### 2. Info.plist — NSLocalNetworkUsageDescription

```xml
<key>NSLocalNetworkUsageDescription</key>
<string>Aplikasi ini menggunakan jaringan lokal untuk komunikasi dengan layanan Firebase</string>
```

Diperlukan oleh Firebase untuk inspect network connections saat debug. Tanpa ini, debug toolbar Firebase tidak berfungsi.

### 3. Info.plist — UIBackgroundModes

```xml
<key>UIBackgroundModes</key>
<array>
    <string>remote-notification</string>
</array>
```

Wajib agar aplikasi bisa menerima push notification saat di background/suspend.

### 4. Runner.entitlements — aps-environment

File baru: `mobile/ios/Runner/Runner.entitlements`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>aps-environment</key>
    <string>production</string>
</dict>
</plist>
```

Key `aps-environment` wajib bagi aplikasi yang mengakses APNs API. Set ke `production` karena `GoogleService-Info.plist` sudah ber-scheme `id.urena0216.socialfinance` (bukan bundle ID development seperti `com.example.socialFinance`).

### 5. project.pbxproj — CODE_SIGN_ENTITLEMENTS

Menambahkan `CODE_SIGN_ENTITLEMENTS = Runner/Runner.entitlements;` ke 3 konfigurasi build Runner:

| Konfigurasi | UUID | Baris |
|---|---|---|
| Debug | 97C147061CF9000F007C117D | 695 |
| Release | 97C147071CF9000F007C117D | 719 |
| Profile | 249021D4217E4FDB00AE95B9 | 512 |

### 6. project.pbxproj — PBXFileReference Runner.entitlements

Menambahkan entry di PBXFileReference section (UUID: `F2A84B7C90D1E356`) dan merujuk ke PBXGroup Runner children.

## Validasi

### flutter analyze
- 0 errors baru. 72 issues (semua pre-existing dari `household_create_save_flow_test.dart`).
- Tidak ada error atau warning akibat perubahan Tahap B.

### Info.plist validation
Semua 3 key baru tervalidasi sebagai plist boolean:
- `NSUserNotificationsUsageDescription`: ✓ string
- `NSLocalNetworkUsageDescription`: ✓ string  
- `UIBackgroundModes`: ✓ array ['remote-notification']

### Entitlements file
- `Runner.entitlements` ada di `mobile/ios/Runner/` (245 bytes)
- Referensi lengkap di pbxproj: PBXFileReference + Runner group children + CODE_SIGN_ENTITLEMENTS di semua 3 build configs

### Firebase pods
- Podfile.lock sudah mencakup `Firebase/Messaging (12.19.0)` dan `firebase_messaging (17.0.0)` dari Tahap A
- Tidak ada perubahan CocoaPods karena tidak ada podspec baru — hanya konfigurasi Xcode

## File Berubah

| File | Aksi | Deskripsi |
|---|---|---|
| `mobile/ios/Runner/Info.plist` | Modified | +4 key: NSUserNotificationsUsageDescription, NSLocalNetworkUsageDescription, UIBackgroundModes |
| `mobile/ios/Runner/Runner.entitlements` | New | aps-environment = production |
| `mobile/ios/Runner.xcodeproj/project.pbxproj` | Modified | PBXFileReference + Runner group children + CODE_SIGN_ENTITLEMENTS (3 configs) |

## Blocker yang Tersisa untuk Tahap C

1. **Apple Developer Account**: Push notification di production memerlukan:
   - APNs Key yang di-generate di [developer.apple.com](https://developer.apple.com)
   - App ID dengan Push Notifications capability enabled
   - Provisioning profile yang sesuai

2. **Firebase Console**: APNs Key harus di-upload ke Firebase project `social-finance-urena` (section Cloud Messaging → Configuration → iOS app → APNs Authentication Key)

3. **Code signing**: Karena `CODE_SIGN_STYLE = Automatic` dan `DEVELOPMENT_TEAM = X7S23439Y6`, Xcode akan otomatis mengambil provisioning profile. Tapi profile harus punya capability `push-notifications`.

4. **arm64 simulator build**: Masih blocked oleh Google ML Kit upstream (mobile_scanner → GoogleMLKit transitive dependency). Debug mode menggunakan x86_64 saat ini, tapi release/device build memerlukan arm64 yang terpengaruh.

## Ringkasan Tahap B

| Item | Status |
|---|---|
| NSUserNotificationsUsageDescription | ✅ DONE |
| NSLocalNetworkUsageDescription | ✅ DONE |
| UIBackgroundModes → remote-notification | ✅ DONE |
| aps-environment entitlement | ✅ DONE |
| CODE_SIGN_ENTITLEMENTS di pbxproj | ✅ DONE |
| flutter analyze | ✅ 0 errors baru |
| Info.plist validation | ✅ 3 key valid |
| Apple Developer config | 🔴 BLOCKED (manual action required) |
| Firebase Console APNs Key | 🔴 BLOCKED (manual action required) |
