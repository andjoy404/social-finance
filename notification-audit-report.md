# Audit Laporan — Notifikasi Urena: Image/Attachment, Testing, dan Free/Manual-Trigger

**Tanggal:** 2026-10-29  
**Branch:** `urena`  
**Mode:** READ-ONLY AUDIT (tidak ada perubahan kode)

---

## A. Ringkasan Eksekutif

Sistem notifikasi push FCM telah diimplementasikan secara fungsional dengan empat event sistem, manajemen template kustom, dan riwayat pengiriman. Namun, untuk tiga agenda audit yang ditanyakan:

1. **Gambar/Attachment:** **TIDAK ADA**. Tidak ada field gambar di template, payload FCM hanya mengirim `notification.title` + `notification.body` + `data` string map. Tidak ada download gambar di Android, tidak ada BigPictureStyle, tidak ada form upload URL gambar.
2. **Fitur Testing:** **SEBAGIAN ADA**. Hanya ada satu endpoint `POST /api/v1/notifications/test-self` yang mengirim notifikasi test ke perangkat pengguna yang terautentikasi sendiri (bukan broadcast ke RT). Endpoint ini nonaktif di production. Tidak ada endpoint `test-push` terpisah untuk broadcast, tidak ada tombol testing di UI web maupun Android, dan tidak ada pembatasan event untuk testing.
3. **Template Bebas/Manual-Trigger:** **SEBAGIAN ADA**. Hanya ada satu manual trigger yaitu `NotifyWasteTruckArrival` (truk sampah). Tidak ada mekanisme umum "free/manual-trigger" yang bisa mengirim notifikasi kustom tanpa event otomatis. Endpoint `POST /api/v1/notifications/waste-truck` hanya bisa dipanggil oleh Pengurus.

**Risiko Terbesar:** Payload FCM saat ini tidak mendukung gambar sama sekali. Jika desain mengharuskan gambar untuk truk sampah dan notifikasi manual, infrastruktur FCM perlu dimodifikasi secara signifikan.

---

## B. Matriks Evidence

| Agenda | Temuan | Status | File:Baris | Bukti | Risiko/Catatan |
|--------|--------|--------|------------|-------|----------------|
| **1. Gambar** | Field `image`/`imageUrl`/`attachment` di template DB | **TERBUKTI TIDAK ADA** | `backend/migrations/040_notification_templates.up.sql:1-27` | Schema hanya punya: `id, rt_id, name, event_type, channel, title, body, is_active, variables`. Tidak ada kolom gambar. | — |
| **1. Gambar** | Field gambar di NotificationTemplate struct | **TERBUKTI TIDAK ADA** | `backend/internal/notification/model.go:91-104` | `NotificationTemplate` hanya punya: `ID, RTID, Name, EventType, Channel, Title, Body, IsActive, Variables`. | — |
| **1. Gambar** | Field gambar di NotificationPayload (FCM data) | **TERBUKTI TIDAK ADA** | `backend/internal/notification/model.go:54-66` | `NotificationPayload` hanya punya: `Type, RTID, BillID, PaymentID, Status, DeepLink, SentAt`. Tidak ada image field. | — |
| **1. Gambar** | Gambar di FCM payload | **TERBUKTI TIDAK ADA** | `backend/internal/notification/provider.go:370-395` | FCM `messagePayload` hanya mengirim `notification.title`, `notification.body`, `data` (string map), dan `android.notification`. Tidak ada `webpush.notification.image` atau Android BigPicture. | — |
| **1. Gambar** | Image URL di CreateTemplateInput | **TERBUKTI TIDAK ADA** | `backend/internal/notification/model.go:107-113` | Hanya: `Name, EventType, Channel, Title, Body`. | — |
| **1. Gambar** | Download gambar di Android | **TERBUKTI TIDAK ADA** | `mobile/lib/core/notification/push_notification_service.dart` | Service hanya memanggil `FirebaseMessaging.onMessage`, `onMessageOpenedApp`, `getInitialMessage`. Tidak ada download gambar, tidak ada `flutter_local_notifications` dengan BigPictureStyle. | — |
| **1. Gambar** | BigPictureStyle / gambar notifikasi tray | **TERBUKTI TIDAK ADA** | `mobile/lib/features/notification/` + `mobile/lib/core/notification/push_notification_service.dart` | Tidak ada import `flutter_local_notifications`, tidak ada `BigPictureStyleInformation`, tidak ada `setOnNotificationOpenedApp` untuk handle tap gambar. | — |
| **1. Gambar** | Tombol upload/URL gambar di UI web | **TERBUKTI TIDAK ADA** | `frontend/src/features/notification/NotificationScreen.tsx:1-500,501-956` | Form create/edit hanya punya: event_type dropdown, name, title, body textarea. Tidak ada image URL field atau preview gambar. | — |
| **1. Gambar** | Pembatasan gambar hanya untuk truk sampah & manual | **BELUM DAPAT DIPASTIKAN** | — | Karena gambar TIDAK ADA sama sekali, pembatasan desain tidak bisa dicek. Masalah utama: tidak ada implementasi gambar. | — |
| **2. Testing** | Endpoint `POST /api/v1/notifications/test-push` | **TERBUKTI TIDAK ADA** | `backend/cmd/api/main.go:760-795` | Route yang ada: `test-self`, `waste-truck`, CRUD templates, logs. Tidak ada `test-push`. | — |
| **2. Testing** | Endpoint `POST /api/v1/notifications/test-self` | **TERBUKTI ADA** | `backend/cmd/api/main.go:770`, `backend/internal/notification/handler.go:107-132`, `backend/internal/notification/service.go:344-361` | Mengirim push ke perangkat user yang login sendiri. Tidak broadcast ke RT. Middleware: `RequireAuth` saja. Nonaktif di production. | Hanya untuk dev testing konektivitas per-user. |
| **2. Testing** | TestSelf hanya targets authenticated user | **TERBUKTI ADA** | `backend/internal/notification/handler.go:107-132` | Handler memanggil `h.svc.SendSelfTest(r.Context(), ac.UserID)` yang memanggil `GetTokensForUser(ctx, userID)`. | Tidak bisa test broadcast. |
| **2. Testing** | TestSelf nonaktif di production | **TERBUKTI ADA** | `backend/cmd/api/main.go:769-770` | `if appEnv != "production"` sebelum route registration. | — |
| **2. Testing** | Unit test test-self | **TERBUKTI ADA** | `backend/internal/notification/notification_test.go:579-670` | 4 test: auth required, production rejection, only targets user, no broadcast/zero devices. | Tidak test FCM gateway sesungguhnya. |
| **2. Testing** | Tombol test di UI web | **TERBUKTI TIDAK ADA** | `frontend/src/features/notification/NotificationScreen.tsx` | Tidak ada tombol "Test" atau "Send Test" di semua tab (system, templates, logs). | — |
| **2. Testing** | Tombol test di UI Android | **TERBUKTI TIDAK ADA** | `mobile/lib/features/notification/presentation/screens/notification_management_screen.dart` | Hanya ada 3 tab: Sistem, Template, Riwayat. Tidak ada tombol test. | — |
| **2. Testing** | Pembatasan testing hanya truk sampah & manual | **BELUM DAPAT DIPASTIKAN** | — | Karena tidak ada endpoint test-push dan tidak ada tombol test UI, pembatasan event tidak bisa dicek. | — |
| **2. Testing** | Pembatasan role untuk test-self | **TERBUKTI ADA** | `backend/internal/notification/handler.go:116-118` | Hanya `RequireAuth` — semua user yang login bisa test. Tidak ada batasan super_admin/ketua. | — |
| **3. Manual** | Event sistem (daftar lengkap) | **TERBUKTI ADA** | `backend/internal/notification/model.go:8-12`, `service.go:447-483` | 4 event: `iuran_bill_created`, `iuran_payment_submitted`, `iuran_payment_verified`, `waste_truck_arrival`. | — |
| **3. Manual** | `waste_truck_arrival` punya trigger manual | **TERBUKTI ADA** | `backend/internal/notification/model.go:10-11` | Trigger: "Pemicu manual oleh Pengurus atau Petugas Keamanan RT". | — |
| **3. Manual** | Endpoint `POST /api/v1/notifications/waste-truck` | **TERBUKTI ADA** | `backend/cmd/api/main.go:779-784` | Middleware: `RequireAuth`, `RequireAllowedRT`, `RequireRole(Pengurus)`. Hanya Pengurus bisa kirim. | Tidak ada untuk Security atau role lain. |
| **3. Manual** | Payload WasteTruckNotificationInput | **TERBUKTI ADA** | `backend/internal/notification/model.go:46-48` | Hanya: `Message string` (optional). Tidak ada image/attachment. | — |
| **3. Manual** | Service `NotifyWasteTruckArrival` | **TERBUKTI ADA** | `backend/internal/notification/service.go:295-342` | Kirim ke seluruh RT (`GetTokensForRT`), cooldown dedup 30 menit per RT. Bisa override body via `customMessage`. Tidak support gambar. | — |
| **3. Manual** | Model/template bebas/manual-trigger umum | **TERBUKTI TIDAK ADA** | — | Tidak ada `ManualTriggerInput`, tidak ada endpoint `POST /notifications/send`, tidak ada event `manual_trigger` atau `free_send`. | — |
| **3. Manual** | Backend pengiriman manual umum | **TERBUKTI TIDAK ADA** | `backend/internal/notification/service.go` | Hanya `NotifyWasteTruckArrival` yang manual. Tidak ada `SendManualNotification` atau fungsi serupa. | — |
| **3. Manual** | Template bisa dibuat tanpa event otomatis | **TERBUKTI TIDAK ADA** | `backend/internal/notification/service.go:419-421` | `CreateTemplate` memanggil `findSystemType(in.EventType)` dan reject jika event tidak dikenal. Semua template harus event sistem. | — |
| **3. Manual** | Target penerima manual | **TERBUKTI ADA (truk sampah saja)** | `backend/internal/notification/service.go:332-336` | `GetTokensForRT(rtID)` → seluruh warga di RT. | — |
| **3. Manual** | Tenant isolation untuk waste-truck | **TERBUKTI ADA** | `backend/cmd/api/main.go:781` | `RequireAllowedRT(pool)` middleware memastikan pengguna hanya trigger di RT mereka sendiri. | — |
| **3. Manual** | Hasil pengiriman masuk log | **TERBUKTI ADA** | `backend/internal/notification/service.go:323-330` | `CheckAndRecordDedup` mencatat ke `notification_logs` dengan status SENT atau dedup. | — |
| **3. Manual** | Risiko double-send | **RISKO RENDAH** | `backend/internal/notification/service.go:323-330` | Cooldown dedup per time-slot 30 menit mencegah spam. | — |
| **3. Manual** | Risiko ke tenant/user salah | **RISKO RENDAH** | `backend/cmd/api/main.go:781`, `service.go:332` | Middleware `RequireAllowedRT` dan query `WHERE rt_id = $1` menjaga isolasi tenant. | — |

---

## C. Peta Alur Kode

### 1. Event Otomatis (contoh: Bill Created)

```
Finance Service (bill created)
  → notifSvc.NotifyBillCreated(ctx, rtID, billID, recipientUserIDs)
     [goroutine background, 15s timeout]
  → GetActiveTemplateForEvent(rtID, "iuran_bill_created")
     → gunakan template kustom atau default
  → interpolate title/body dengan {{bill_id}}
  → for each recipient userID:
       CheckAndRecordDedup("bill:{billID}:{userID}") → skip jika duplikat
       GetTokensForUser(userID) → ambil FCM tokens aktif
       sendPushToTokens(tokens, title, body, payloadMap)
          → FCMProvider.Send(token, title, body, data)
             → OAuth JWT → FCM HTTP v1 API
```

**File:** `service.go:153-189`, `provider.go:350-446`

### 2. Push Notification (FCM transport)

```
FCMProvider.Send(token, title, body, data)
  → [Production] require valid credentials, reject mock
  → [Dev] fallback to MockPushProvider if no credentials
  → getAccessToken() → OAuth2 JWT RS256 → googleapis
  → Build JSON payload:
     {
       "message": {
         "token": token,
         "notification": { "title": "...", "body": "..." },
         "data": { "type": "...", "rt_id": "...", ... },
         "android": {
           "priority": "HIGH",
           "notification": {
             "channel_id": "social_finance_notifications",
             "sound": "default",
             "notification_priority": "PRIORITY_MAX"
           }
         }
       }
     }
  → POST https://fcm.googleapis.com/v1/projects/{project}/messages:send
  → Parse response: UNREGISTERED → ErrTokenUnregistered, 400 → ErrTokenInvalid
```

**File:** `provider.go:350-446`

### 3. Test Push

```
POST /api/v1/notifications/test-self
  [RequireAuth] (hanya non-production)
  → TestSelf handler
     → SendSelfTest(ctx, ac.UserID)
        → GetTokensForUser(userID) → ambil tokens aktif user yang login
        → Kirim 1 push dengan title "Uji Notifikasi SocialFinance"
          ke semua device user tersebut
  → Response: { "status": "dispatched", "dispatched_count": N }
```

**File:** `handler.go:107-132`, `service.go:344-361`

### 4. Notifikasi Bebas/Manual — Waste Truck

```
POST /api/v1/notifications/waste-truck
  [RequireAuth + RequireAllowedRT + RequireRole(Pengurus)]
  → TriggerWasteTruckArrival handler
     → WasteTruckNotificationInput { Message: optional_string }
     → NotifyWasteTruckArrival(ctx, ac.RTID, ac.UserID, message)
        [goroutine background, 20s timeout]
     → Cek template aktif: GetActiveTemplateForEvent("waste_truck_arrival")
     → Cooldown dedup: 30 menit per RT (time slot)
     → GetTokensForRT(rtID) → seluruh warga RT
     → Kirim push ke semua device warga
  → Response: { "status": "accepted", "message": "..." }
```

**File:** `handler.go:86-105`, `service.go:295-342`

---

## D. Gap Analysis

### Fitur yang Terbukti Sudah Tersedia

| Fitur | Keterangan |
|-------|-----------|
| 4 event sistem | `iuran_bill_created`, `iuran_payment_submitted`, `iuran_payment_verified`, `waste_truck_arrival` |
| Manajemen template kustom (CRUD) | Web UI + API lengkap: list, create, update, toggle active, delete |
| Template variable interpolation | `{{bill_id}}`, `{{payment_id}}`, `{{status}}`, `{{rejection_reason}}` |
| Validasi variabel template | Hanya variabel yang diizinkan per event yang boleh digunakan |
| Token registrasi perangkat | Android/IOS/Web + token refresh |
| Push FCM production-ready | OAuth2, credential validation, token invalid handling |
| Mock provider (dev/test) | `MockPushProvider` mencatat pesan di memori |
| Deduplication | Per-user per-event (bill), per-RT (payment), per-RT per-30min (waste truck) |
| Riwayat pengiriman | `notification_logs` + paginated listing di web UI |
| Preview banner | Web UI dan Android menampilkan simulasi notifikasi |
| Waste truck manual trigger | Endpoint + UI button untuk Pengurus |
| Tenant isolation | `RequireAllowedRT` middleware |
| Notification log dengan dedup key | `UNIQUE(dedup_key)` constraint di DB |

### Fitur yang Belum Tersedia

| Fitur | Keterangan |
|-------|-----------|
| **Gambar/attachment di template** | Tidak ada field, tidak ada upload, tidak ada URL field |
| **Gambar di FCM payload** | Payload hanya `title` + `body` string, tidak ada `image` field |
| **Gambar di Android notification** | Tidak ada download gambar, tidak ada BigPictureStyle |
| **Gambar di preview UI** | Preview hanya teks, tidak ada gambar |
| **Endpoint test-push broadcast** | Hanya ada `test-self` (per-user). Tidak ada test ke seluruh RT atau grup |
| **Tombol test di UI web** | Tidak ada tombol test/send di tab manapun |
| **Tombol test di UI Android** | Tidak ada tombol test/send |
| **Template bebas/manual umum** | Hanya waste_truck. Tidak ada cara kirim notifikasi kustom tanpa event otomatis |
| **Endpoint manual trigger umum** | Tidak ada `POST /notifications/send` yang menerima isi notifikasi langsung |
| **Pembatasan testing per event** | Tidak ada pembatasan karena tidak ada endpoint test-push |
| **Gambar khusus untuk truk sampah & manual** | Tidak ada karena fitur gambar belum dimulai |

### Fitur Ada Tapi Belum Memenuhi Batasan Desain

| Batasan | Status |
|---------|--------|
| Gambar hanya untuk truk sampah & manual-trigger | **TIDAK DAPAT DIPASTIKAN** — fitur gambar belum ada, tidak bisa dibatasi |
| Testing hanya untuk truk sampah & manual-trigger | **TIDAK DAPAT DIPASTIKAN** — tidak ada endpoint test-push, tidak ada pembatasan |
| Free/manual-trigger support gambar | **TIDAK DAPAT DIPASTIKAN** — fitur gambar belum ada |

### Hal yang Belum Dapat Dipastikan

| Hal | Alasan |
|-----|--------|
| Apakah ada custom Android native code untuk gambar notifikasi | Tidak ada import `flutter_local_notifications` atau native channel handler yang terkait gambar |
| Apakah Firebase console dikonfigurasi dengan notification image | Tidak bisa dicek dari kode |
| Apakah ada backend yang memanggil notifikasi dari luar module notification | Hanya finance module yang trigger event otomatis, tidak ada yang lain ditemukan |

---

## E. Rekomendasi Desain

### 1. Gambar/Attachment

**Arahan:** Tambahkan dukungan gambar jika benar-benar diperlukan untuk truk sampah dan notifikasi manual.

- **Database:** Tambah kolom `image_url TEXT` di `notification_templates` dan `image_url TEXT` di `notification_logs`.
- **Model:** Tambah `ImageURL *string` ke `NotificationTemplate`, `CreateTemplateInput`, `UpdateTemplateInput`, dan `NotificationPayload`.
- **FCM Payload:** Tambahkan field `webpush.notification.image` dan gunakan Android `BigPictureStyleInformation` di `flutter_local_notifications` untuk render gambar di tray.
- **Validasi:** Validasi URL (format, size limit, optional image download untuk security), tidak boleh ada file upload langsung ke backend — hanya URL yang disimpan.
- **Pembatasan:** Tambah validasi di `CreateTemplate` dan `UpdateTemplate` bahwa field `image_url` hanya boleh diisi untuk event `waste_truck_arrival` dan event manual-trigger. Reject untuk event sistem lain.

### 2. Fitur Testing

**Arahan:** Buat endpoint test-push yang terpisah dan dibatasi.

- **Endpoint baru:** `POST /api/v1/notifications/test-push` dengan middleware `RequireAuth` + `RequireRole(Pengurus)` atau `RequireNotificationAdmin()`.
- **Payload:** `{"target": "rt" | "all_users" | "specific_roles", "event_type": "...", "title": "...", "body": "...", "image_url": "..."}`
- **Tujuan:** Test harus bisa kirim ke seluruh RT atau ke role tertentu, bukan hanya user sendiri.
- **Pembatasan event:** Di backend, validasi `event_type` hanya mengizinkan `waste_truck_arrival` dan event manual-trigger. Event billing/payment tidak boleh diuji via test-push (mengingat ini mengubah data nyata).
- **UI:** Tambahkan tombol "Test Push" di tab Sistem (setiap kartu event) dan di tab Riwayat.

### 3. Template Bebas/Manual-Trigger

**Arahan:** Buat mekanisme manual trigger yang umum, bukan hanya untuk truk sampah.

- **Endpoint baru:** `POST /api/v1/notifications/manual` dengan payload:
  ```json
  {
    "title": "Pesan kustom",
    "body": "Isi pesan kustom",
    "image_url": "https://...",
    "target": "all_users",
    "deep_link": "/home"
  }
  ```
- **Tanpa event otomatis:** Endpoint ini tidak memerlukan template tersimpan — mengirim notifikasi langsung.
- **Authorization:** `RequireNotificationAdmin()` (Super Admin atau Ketua RT).
- **Dedup:** Gunakan waktu saat ini sebagai dedup key untuk mencegah spam.
- **Target:** `all_users` (semua warga RT), atau filter berdasarkan role (bendahara, pengurus, warga).
- **Log:** Catat ke `notification_logs` dengan `event_type = "manual_push"`.

### 4. Pembatasan Gambar/Testing

Agar gambar dan testing hanya tersedia untuk truk sampah dan manual-trigger:

- Di backend service, tambah enum/constant `AllowedImageEvents = ["waste_truck_arrival", "manual_push"]`.
- Di handler CreateTemplate/UpdateTemplate, validasi: jika `image_url` ada, hanya izinkan untuk event dalam `AllowedImageEvents`.
- Di handler test-push, validasi: hanya izinkan `event_type` dalam `AllowedImageEvents`.
- Di FCM provider, gambar dikirim melalui `data.image_url` atau field spesifik platform.

---

## F. Integritas Workspace

**Branch saat audit:** `urena`

**Status workspace sebelum audit:**
```
 M artifacts/kas-report-acceptance/Laporan-KAS-2026-10.pdf
 M artifacts/kas-report-acceptance/Laporan-KAS-2026-10.xlsx
 M backend/cmd/api/main.go
 ... (banyak file modifikasi dan untracked, termasuk modul notifikasi baru)
```

**Status workspace setelah audit:**
TIDAK ADA PERUBAHAN. Audit ini 100% read-only. Tidak ada file yang dimodifikasi, dibuat, dihapus, diformat, atau di-commit. Tidak ada migrasi database yang dijalankan. Tidak ada operasi destructive yang dilakukan.

**Test yang dijalankan:** Tidak ada. Audit ini sepenuhnya statis (pemeriksaan kode dan dokumentasi).

**File yang diperiksa:**

| Area | File |
|------|------|
| Backend Model | `backend/internal/notification/model.go` |
| Backend Handler | `backend/internal/notification/handler.go` |
| Backend Service | `backend/internal/notification/service.go` |
| Backend Provider (FCM) | `backend/internal/notification/provider.go` |
| Backend Repository | `backend/internal/notification/repository.go` |
| Backend Route | `backend/cmd/api/main.go` (baris 760-809) |
| Backend Migration | `backend/migrations/022_user_devices_and_notifications.up.sql` |
| Backend Migration | `backend/migrations/040_notification_templates.up.sql` |
| Backend Test | `backend/internal/notification/notification_test.go` |
| Backend Test | `backend/internal/notification/notification_management_test.go` |
| Frontend API Types | `frontend/src/app/api.ts` (baris 1670+) |
| Frontend UI | `frontend/src/features/notification/NotificationScreen.tsx` |
| Frontend Test | `frontend/src/features/notification/NotificationScreen.test.tsx` |
| Mobile API Client | `mobile/lib/features/notification/data/notification_api_client.dart` |
| Mobile Repository | `mobile/lib/features/notification/data/notification_repository.dart` |
| Mobile Models | `mobile/lib/features/notification/data/models/notification_models.dart` |
| Mobile UI Screen | `mobile/lib/features/notification/presentation/screens/notification_management_screen.dart` |
| Mobile Push Service | `mobile/lib/core/notification/push_notification_service.dart` |
| Mobile Tests | `mobile/test/features/notification/notification_management_screen_test.dart` |
| Mobile Tests | `mobile/test/features/notification/notification_models_test.dart` |
| Mobile Tests | `mobile/test/features/notification/notification_access_control_test.dart` |
| Docs API | `docs/api.md` (bagian notification) |
| Docs DB | `docs/database.md` (bagian notification) |

---

*Audit selesai — semua kesimpulan didukung evidence dari kode sumber.*
