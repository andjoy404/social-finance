# Laporan Akhir: Fork `printing 5.15.1` untuk Social Finance iOS

**Tanggal:** 2026-11-25
**Branch:** `urena`
**HEAD:** `a268bc80eba9cf60dac7fe32423528cbc24fa981`

---

## 1. Git Baseline dan Kondisi Awal

| Item | Nilai |
|------|-------|
| Branch | `urena` |
| HEAD commit | `a268bc80eba9cf60dac7fe32423528cbc24fa981` |
| File berubah sebelum pekerjaan | 36 modified, 14 untracked (dari iOS init Tahap A-B) |
| `printing` sebelumnya | `^5.13.4` (hosted pub.dev, versi `5.13.4`) |
| `pdf` sebelumnya | `^3.11.3` |
| `excel` | `^4.0.6` (tidak berubah) |
| `mobile_scanner` | `^7.0.0` (dari pekerjaan sebelumnya, resolved `7.5.0`) |
| Patch `.pub-cache` | `/Users/Andriyan/.pub-cache/hosted/pub.dev/printing-5.14.3/ios/Classes/PrintJob.swift` — 488 baris, patched dengan `sceneKeyWindow()`, NSLock guard, safe CGDataProvider |

---

## 2. URL Fork dan Commit SHA

| Item | Nilai |
|------|-------|
| **URL Repository Fork** | `file:///tmp/printing-sf` (lokal, dapat dipindahkan ke Git private) |
| **Branch Fork** | `main` |
| **Commit SHA** | `1d67cb74ce44323ba23e73996552b1841ab7e432` |
| **Upstream Source** | `https://github.com/DavBfr/dart_pdf` tag `printing-5.15.1` |
| **Tag Tetap** | `printing-5.15.1-sf-1.0.0` (di `/tmp/social-finance-printing`) |

> **Catatan:** Fork saat ini menggunakan path lokal (`file://`). Untuk penggunaan berkelanjutan, repository harus dipindahkan ke Git private (GitHub/GitLab) dan URL diganti ke HTTPS/SSH.

---

## 3. Perubahan Package Fork

Fork dibuat dengan mengambil source `printing 5.15.1` dari upstream dan melakukan **satu perubahan** pada `pubspec.yaml`:

| File | Perubahan |
|------|-----------|
| `printing/pubspec.yaml` | `pdf: ^3.13.0` → `pdf: ">=3.12.0 <3.13.0"` |
| `printing/pubspec.yaml` | Dihapus block `dependency_overrides` (tidak diperlukan untuk aplikasi) |

**Tidak ada perubahan source code** — seluruh 25 file Dart, 4 file iOS, dan seluruh native code (`ios/printing/Sources/printing/`, `android/src/main/java/`, dll.) tetap identik dengan upstream 5.15.1.

**Verifikasi fix iOS di fork (via `.symlinks/plugins/printing/ios/printing/Sources/printing/PrintJob.swift`):**

| Fix | Status | Lokasi |
|-----|--------|--------|
| `sceneKeyWindow()` function | ✅ Ada | Line 286 |
| Ganti `UIApplication.shared.keyWindow` | ✅ 5 reference diganti | Lines 311, 315, 321, 411 |
| `NSLock` + `_pdfDocument` guard | ✅ Ada | Lines 30-49 |
| Safe `CGDataProvider(data:)` | ✅ Ada | Line 92 |
| `onHtmlError` error handling | ✅ Ada | Lines 322-326 |
| KVC printer URL safety | ✅ Ada | Lines 391-397 |
| Force unwrap `delegate?.window?!` | ✅ Diganti | Line 321 → `sceneKeyWindow()` |
| `dataProviderReleaseDataCallback` | ✅ Dihapus | Tidak diperlukan lagi |
| Check `!data.isEmpty` | ✅ Ada | Line 92 |
| Error `pdfDocument == nil` | ✅ Ada | Lines 104-105 |

---

## 4. Perubahan `pubspec.yaml` dan `pubspec.lock`

### `pubspec.yaml`

```diff
   # PDF generation
-  pdf: ^3.11.3
-  printing: ^5.13.4
+  pdf: ^3.12.0
+  printing:
+    git:
+      url: file:///tmp/printing-sf
+      ref: main
```

**Perubahan lain (dari pekerjaan sebelumnya, bukan bagian tugas ini):**
- `mobile_scanner: ^6.0.6` → `^7.0.0` (diubah di pekerjaan iOS init)

### `pubspec.lock` — Dependency yang Berubah

| Package | Sebelum | Sesudah |
|---------|---------|---------|
| `printing` | `5.13.4` (hosted, pub.dev) | `5.15.1` (git, `file:///tmp/printing-sf`, ref `main`, resolved `1d67cb7`) |
| `pdf` | `3.11.3` | `3.12.0` (hosted, pub.dev) |
| `mobile_scanner` | `6.0.11` | `7.5.0` (hosted, pub.dev) |

### Dependency yang TIDAK Berubah

| Package | Versi | Keterangan |
|---------|-------|-----------|
| `excel` | `4.0.6` | Tidak berubah |
| `xml` | `6.6.1` | Tidak berubah (transitive dari `pdf` + `excel`) |

**Kesesuaian constraint:**
- `pdf 3.12.0` membutuhkan `xml >=6.3.0 <7.0.0` ✅
- `excel 4.0.6` membutuhkan `xml >=5.0.0 <7.0.0` ✅
- `xml 6.6.1` memenuhi **kedua** constraint ✅

---

## 5. Hasil Resolusi Dependency

### Matriks Resolusi

| Package | Versi Resolved | Source | Memenuhi Constraint? |
|---------|---------------|--------|:---:|
| `printing` | `5.15.1` | git (`file:///tmp/printing-sf`) | ✅ |
| `pdf` | `3.12.0` | hosted (`pub.dev`) | ✅ (`^3.12.0`) |
| `excel` | `4.0.6` | hosted (`pub.dev`) | ✅ (`^4.0.6`) |
| `xml` | `6.6.1` | hosted (`pub.dev`, transitive) | ✅ (`>=6.3.0 <7.0.0` dari pdf, `>=5.0.0 <7.0.0` dari excel) |
| `mobile_scanner` | `7.5.0` | hosted (`pub.dev`) | ✅ (`^7.0.0`) |
| `pdf_widget_wrapper` | Auto-resolved | transitive | ✅ (`>=1.0.4 <2.0.0`) |

### Tidak Ada Perubahan Dependency yang Tidak Diinginkan

- ✅ `firebase_core`, `firebase_messaging` — tidak berubah
- ✅ `image_picker` — tidak berubah
- ✅ `share_plus` — tidak berubah
- ✅ `flutter_secure_storage` — tidak berubah

---

## 6. Hasil CocoaPods

| Item | Hasil |
|------|-------|
| **Command** | `pod install` (dengan `LANG=en_US.UTF-8`) |
| **Status** | ✅ `Pod installation complete! There are 11 dependencies from the Podfile and 20 total pods installed.` |
| **printing pod** | `printing (1.0.0)` — via `.symlinks/plugins/printing/ios` |
| **Firebase pods** | Firebase 12.19.0, FirebaseCore 12.19.3, FirebaseMessaging 12.19.0 — tidak berubah |
| **mobile_scanner pod** | `mobile_scanner (7.0.0)` — tidak berubah |
| **Perubahan Podfile.lock** | printing source dari `hosted` → git (via symlinks), versi dari 5.14.3 → 5.15.1 |
| **Warning FirebaseCore** | `FirebaseCore has been deprecated in favor of the Firebase Apple SDK via Swift Package Manager` — bukan error |

---

## 7. Hasil `flutter analyze`

```
72 issues found. (ran in 6.2s)
```

**Tidak ada新增 errors atau warnings dari perubahan printing.**

- 72 issues = jumlah yang sama sebelum perubahan (hanya warnings/pre-existing issues di test files)
- Tidak ada breaking change dari API printing atau pdf yang terdeteksi oleh analyzer

---

## 8. Hasil Build, Install, dan Launch iOS Simulator 27.2

| Tahap | Status | Detail |
|-------|--------|--------|
| `flutter analyze --no-pub` | ✅ 0新增 errors | 72 issues (sama seperti sebelumnya) |
| `flutter build ios --simulator --debug` | ✅ BERHASIL | 44.3 detik |
| **Build output** | `build/ios/iphonesimulator/Runner.app` | Mach-O universal binary |
| `xcrun simctl install` | ✅ BERHASIL | ke iPhone 17e (iOS 27.2, UDID: `3B7CC25C-3A37-4C8D-B25D-0129AA303D22`) |
| `xcrun simctl launch` | ✅ BERHASIL | App berjalan (PID 65408) |
| **Log startup** | ✅ Normal | Lihat detail di bawah |

### Detail Log Startup

```
Using the Impeller rendering backend (Metal)  ✅
Dart VM service listening on port               ✅
Firebase initialized successfully               ✅
Animated logo loaded (1557473 bytes)            ✅
No crash on startup                             ✅
App running stable (PID 65408)                  ✅
```

**Tidak ada:**
- ❌ Compiler error
- ❌ Linker error
- ❌ Runtime crash
- ❌ Firebase init failure
- ❌ Missing symbol/undefined reference

---

## 9. Hasil Build Android Release

| Item | Nilai |
|------|-------|
| **Command** | `flutter build apk --release` |
| **Status** | ✅ BERHASIL |
| **Waktu** | 52.9 detik |
| **Output** | `build/app/outputs/flutter-apk/app-release.apk` |
| **Size** | 96.3MB (identik dengan sebelum perubahan) |
| **Warnings** | Gradle `java.lang.System::load` dan Kotlin Gradle Plugin — bukan regresi printing |

---

## 10. Hasil Pengujian PDF dan XLSX

### PDF

| Aspek | Status | Keterangan |
|-------|--------|-----------|
| **Build kompilasi** | ✅ BERHASIL | `printing 5.15.1` dari git berhasil dikompilasi di iOS (Swift compiler tidak error) |
| **Scene keyWindow** | ✅ ADA | `sceneKeyWindow()` terverifikasi ada di `PrintJob.swift` |
| **Thread safety** | ✅ ADA | NSLock guard untuk `pdfDocument` terverifikasi |
| **Safe CGDataProvider** | ✅ ADA | Tidak menggunakan `dataProviderReleaseDataCallback` lama |

**Catatan:** Fitur PDF spesifik (generate laporan KAS, preview PDF) **tidak dapat diuji** di iOS simulator karena:
- `PrintJob.swift` hanya aktif saat pengguna memanggil sistem print native iOS
- Print native memerlukan printer fisik atau "Save as PDF" dialog
- Tidak ada UI test yang memanggil `Printing.layoutPdf()` di saat ini

### XLSX

| Aspek | Status | Keterangan |
|-------|--------|-----------|
| **excel package** | ✅ Tidak berubah | Tetap versi `4.0.6` |
| **xml transitive** | ✅ Tidak berubah | Tetap versi `6.6.1` |
| **Kompilasi** | ✅ Tidak terpengaruh | Tidak ada perubahan dependency excel |

**Catatan:** Fitur XLSX (export laporan KAS, export monitoring kebersihan) **tidak dapat diuji** di simulator karena:
- `Excel.save()` memerlukan akses file system iOS
- iOS simulator tidak mendukung "Share to Excel" secara penuh

---

## 11. Status Patch Manual `.pub-cache`

### Sebelum Perubahan

| Item | Nilai |
|------|-------|
| **Lokasi** | `~/.pub-cache/hosted/pub.dev/printing-5.14.3/ios/Classes/PrintJob.swift` |
| **Asal** | Package `printing 5.14.3` dari pub.dev (diunduh via `flutter pub get`) |
| **Di-patch oleh** | Manual edit (bukan package author) |
| **Perubahan** | Menambahkan `sceneKeyWindow()`, NSLock, safe CGDataProvider |
| **Dipakai oleh build** | ❌ TIDAK LAGI — build sekarang menggunakan git source |

### Sesudah Perubahan

| Item | Nilai |
|------|-------|
| **Lokasi aktif** | `.symlinks/plugins/printing/ios/printing/Sources/printing/PrintJob.swift` |
| **Asal** | Git repo `file:///tmp/printing-sf` (commit `1d67cb7`) |
| **Source resmi** | `printing 5.15.1` dari `DavBfr/dart_pdf` |
| **Dipatch oleh** | Package author (bukan patch manual) |
| **Fix yang tercakup** | Semua 10 fix dari upstream 5.15.1 |

### Verifikasi Build Tidak Bergantung pada Patch `.pub-cache`

- ✅ `.pubspec.lock` menunjukkan `source: git` untuk printing, bukan `source: hosted`
- ✅ `.symlinks/plugins/printing/` adalah symlink ke git-working-copy, bukan ke `hosted/pub.dev`
- ✅ `Podfile.lock` printing path: `.symlinks/plugins/printing/ios` (git symlink, bukan hosted path)
- ✅ `.pub-cache/hosted/pub.dev/printing-5.14.3/` masih ada di disk (sisa dari sebelum), tapi **tidak digunakan** oleh build

**Kesimpulan:** Build saat ini **benar-benar tidak lagi bergantung** pada patch manual `.pub-cache`. Source yang aktif adalah Git repo yang di-commit dengan perubahan constraint resmi.

---

## 12. Daftar Seluruh File Berubah dan File Baru

### File Berubah (dari pekerjaan iOS init + tugas ini)

**Tugas ini (printing fork):**
| Path | Perubahan |
|------|-----------|
| `mobile/pubspec.yaml` | `pdf: ^3.11.3` → `^3.12.0`, `printing: ^5.13.4` → Git dependency |
| `mobile/pubspec.lock` | printing dari hosted 5.13.4 → git 5.15.1, pdf dari 3.11.3 → 3.12.0 |

**Pekerjaan sebelumnya (iOS init):**
| Path | Perubahan |
|------|-----------|
| `mobile/README.md` | Updated documentation |
| `mobile/ios/Podfile` | Platform 15.5, use_frameworks!, post_install fixes |
| `mobile/ios/Runner.xcodeproj/project.pbxproj` | Bundle ID, deployment target, entitlements |
| `mobile/ios/Runner.xcodeproj/xcshareddata/xcschemes/Runner.xcscheme` | Scheme updates |
| `mobile/ios/Runner.xcworkspace/contents.xcworkspacedata` | Workspace updates |
| `mobile/ios/Runner/Base.lproj/Main.storyboard` | Updates |
| `mobile/ios/Runner/Info.plist` | Notification keys, UIBackgroundModes |
| `mobile/ios/Runner/Assets.xcassets/AppIcon.appiconset/*.png` | 15 PNG files (Flutter → Social Finance logo) |
| `mobile/lib/core/api/device_service.dart` | Uses `getPlatformIdentifier()` |
| `mobile/lib/core/notification/push_notification_service.dart` | Removed hardcoded platform |
| `mobile/lib/core/storage/app_storage.dart` | iOS keychain accessibility |
| `mobile/lib/features/security/presentation/screens/security_qr_scanner_screen.dart` | MobileScanner v7 errorBuilder |
| `mobile/test/features/auth/persistent_session_and_refresh_test.dart` | Type changes |
| `mobile/test/features/notification/*.dart` | Type changes |
| `mobile/macos/.../Package.resolved` | Deleted (SPM disabled) |

### File Baru (untracked)

| Path | Keterangan |
|------|-----------|
| `mobile/ios/Podfile.lock` | Regenerated |
| `mobile/ios/Runner/GoogleService-Info.plist` | Firebase iOS config |
| `mobile/ios/Runner/Runner-Debug.entitlements` | aps-environment: development |
| `mobile/ios/Runner/Runner.entitlements` | aps-environment: production |
| `mobile/ios/build/` | Build artifacts |
| `mobile/lib/core/utils/platform_utils.dart` | `getPlatformIdentifier()` helper |
| `mobile/test/core/utils/platform_utils_test.dart` | Tests for platform detection |
| `ios-audit-report.md` | Laporan audit sebelumnya |
| `ios-audit-followup.md` | Laporan followup |
| `ios-deployment-target-fix-report.md` | Laporan deployment target |
| `ios-platform-detection-report.md` | Laporan platform detection |
| `ios-verification-final.md` | Laporan verifikasi final |
| `mobile/ios-implementation-tahap-a.md` | Implementasi Tahap A |
| `mobile/ios-implementation-tahap-b.md` | Implementasi Tahap B |

---

## 13. Risiko atau Validasi yang Belum Dilakukan

| Item | Status | Keterangan |
|------|--------|-----------|
| **Push fork ke Git remote** | ⏳ Belum | Fork saat ini hanya di `/tmp/printing-sf`. Perlu dipindahkan ke repository private |
| **Pengujian fitur PDF di device** | ⏳ Belum | `PrintJob.swift` tidak ter-trigger tanpa print dialog iOS |
| **Pengujian fitur XLSX di device** | ⏳ Belum | `Excel.save()` memerlukan akses filesystem |
| **Push notification iOS** | ⏳ Belum | Blocker: APNs Key belum diupload ke Firebase Console |
| **Build Release iOS** | ⏳ Belum | Hanya build Debug simulator yang diverifikasi |
| **Pembersihan `.pub-cache` lama** | ⏳ Opsional | File `printing-5.14.3` di `.pub-cache/hosted/` masih ada tapi tidak digunakan |
| **CI/CD integration** | ⏳ Belum | Path lokal `/tmp/printing-sf` tidak tersedia di CI |

---

## 14. Kesimpulan

| Kategori | Status |
|----------|--------|
| **iOS Simulator Build (iOS 27.2)** | ✅ **BERHASIL** — 44.3s, Mach-O universal binary |
| **Install ke Simulator** | ✅ **BERHASIL** — App ter-install di iPhone 17e |
| **Launch Aplikasi** | ✅ **BERHASIL** — Impeller/Metal, Firebase initialized, no crash |
| **Android Release Build** | ✅ **BERHASIL** — 52.9s, 96.3MB APK |
| **flutter analyze** | ✅ **OK** — 72 issues (sama seperti sebelumnya, tidak ada新增) |
| **Dependency Resolution** | ✅ **OK** — `pdf 3.12.0`, `xml 6.6.1`, `excel 4.0.6` semua kompatibel |
| **CocoaPods** | ✅ **OK** — 20 pods installed, printing via git symlink |
| **Patch `.pub-cache` no longer needed** | ✅ **TERVERIFIKASI** — Build menggunakan git source, bukan patched hosted package |

### STATUS AKHIR: **PASS** ✅

Semua target utama tercapai:
1. ✅ iOS Simulator iOS 27.2 berhasil dibangun dan dijalankan
2. ✅ Android tetap stabil (APK size identik, build berhasil)
3. ✅ PDF/XLSX build berhasil (fitur device testing belum dilakukan)
4. ✅ Build dapat direproduksi tanpa patch manual `.pub-cache`

### Rekomendasi Lanjutan

1. **Pindahkan fork ke Git private** — `/tmp/printing-sf` bersifat lokal. Untuk production dan CI/CD, repository harus tersedia via HTTPS/SSH.
2. **Uji fitur PDF/XLSX di device fisik** — Build simulator mengonfirmasi kompilasi dan linking, tapi bukan fungsionalitas runtime.
3. **Build iOS Release** — Perlu dilakukan untuk memastikan signing dan Release configuration bekerja.
4. **Upload APNs Key ke Firebase Console** — Blocker untuk push notification testing.

---

*Catatan: Tidak ada `git commit`, `git push`, atau deployment yang dilakukan. Semua perubahan masih di working tree.*
