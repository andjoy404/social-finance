# Desain Implementasi: RW Permission Enforcement

**Tanggal:** 31 Oktober 2026  
**Status:** DESIGN-ONLY — Tidak ada file yang diubah.

---

## 1. Kondisi Aktual Produksi

Berdasarkan verifikasi langsung terhadap source code:

### 1.1 `rwPermissionMap` — Definisi Ada, Pemakaian Tidak

| File | Baris | Kode |
|---|---|---|
| `internal/auth/scoping.go` | 237–264 | Definisi `rwPermissionMap` (4 jabatan × 4 permission) |
| `internal/auth/scoping.go` | 267–278 | Definisi `hasRWScopePermission(jabatan, permission)` |

**Call site produksi: TIDAK DITEMUKAN.**

`grep -rn "hasRWScopePermission"` di seluruh backend:
- Hanya ditemukan di `scoping.go` (definisi fungsi)
- Hanya ditemukan di `scoping_test.go` dan `rw_permission_test.go` (test)
- **Tidak ada call site di handler, service, atau middleware produksi**

`grep -rn "rwPermissionMap"` di seluruh backend:
- Hanya ditemukan di `scoping.go` (definisi map)
- Hanya ditemukan di `scoping_test.go` dan `rw_permission_test.go` (test)
- **Tidak ada call site di handler, service, atau middleware produksi**

### 1.2 `RequireRWScope` — Middleware Ada, Tapi Hanya Validasi Scope

Middleware di `scoping.go:87-152` melakukan 3 hal:
1. Ekstrak rwId dari URL path
2. Validasi rwId di `SCOPED_RW_IDS` allowlist (deployment gate)
3. Untuk SA: bypass semua, verifikasi RW aktif di DB
4. Untuk user biasa: verifikasi RW membership cocok di `RWMemberships`

**Yang TIDAK dilakukan middleware ini:**
- ❌ Tidak memeriksa jabatan
- ❌ Tidak memeriksa permission (`finance.read`, `finance.create`, dll)
- ❌ Tidak membedakan antara read dan write operation

### 1.3 RW Handler — Hanya Cek AuthContext, Bukan Jabatan

Di `rw/handler.go`, setiap handler melakukan:
```go
ac, ok := getAuthContext(r)  // Hanya cek apakah AuthContext ada
rwID := chi.URLParam(r, "rwId")  // Ekstrak rwId dari URL
// TIDAK ada pengecekan jabatan
// TIDAK ada pengecekan permission
h.svc.GetDashboard(r.Context(), ac, rwID)  // Langsung panggil service
```

### 1.4 RW Service — Hanya Cek Membership, Bukan Jabatan

Di `rw/service.go`, setiap service method memanggil:
```go
func (s *Service) checkRWAccess(ac *auth.AuthContext, rwID string) error {
    if ac.SystemRole == auth.SystemRoleSuperAdmin { return nil }  // SA bypass semua
    for _, rm := range ac.RWMemberships {
        if rm.RWID == rwID { return nil }  // Hanya cek rwId cocok
    }
    return ErrRWAccessDenied
}
```

**Tidak ada pengecekan jabatan atau permission.**

### 1.5 Ringkasan Kondisi Produksi

| Komponen | Status | Keterangan |
|---|---|---|
| `rwPermissionMap` | Definisi saja | Map hardcoded, tidak dibaca siapa pun |
| `hasRWScopePermission` | Definisi saja | Fungsi utilitas, tidak dipanggil siapa pun |
| `RequireRWScope` | Aktif | Hanya validasi scope/membership |
| Handler RW | Aktif | Hanya cek AuthContext, tidak cek jabatan/permission |
| Service RW | Aktif | `checkRWAccess` hanya cek membership, tidak cek jabatan/permission |
| Endpoint RW | 4 endpoint, READ-ONLY | Dashboard, KAS, Transactions, Locations — semua GET |

### 1.6 Catatan: Semua Endpoint RW Saat Ini READ-ONLY

Ke-4 endpoint RW saat ini hanya menggunakan HTTP GET:
1. `GET /api/v1/rw/{rwId}/dashboard` — Dashboard summary
2. `GET /api/v1/rw/{rwId}/kas` — KAS report
3. `GET /api/v1/rw/{rwId}/kas/transactions` — Transaction list
4. `GET /api/v1/rw/{rwId}/locations` — Fund locations list

**Tidak ada endpoint write untuk RW** (create, update, delete, reverse).

Ini mengubah strategi enforcement:
- **Saat ini:** Semua operasi adalah read → semua jabatan butuh `finance.read`
- **Ke depan:** Jika endpoint write ditambahkan, perlu enforcement per-permission

---

## 2. Matriks Permission Final

### 2.1 Matriks Jabatan

| Jabatan | finance.read | finance.create | finance.update | finance.approve |
|---|:---:|:---:|:---:|:---:|
| ketua | Ya | Ya | Ya | Ya |
| wakil_ketua | Ya | Ya | Tidak | Ya |
| sekretaris | Ya | Tidak | Tidak | Tidak |
| bendahara | Ya | Ya | Ya | Ya |

Matriks ini **sudah ada di `rwPermissionMap`** di `scoping.go`. Tidak ada perubahan.

### 2.2 Perbedaan `finance.update` vs `finance.approve`

| Permission | Tanggung Jawab | Contoh Operasi RW |
|---|---|---|
| `finance.update` | Mengubah data transaksi yang sudah ada (jumlah, deskripsi, tanggal, jenis, dll) | Koreksi transaksi KAS yang salah input, mengubah detail transaksi yang sudah posted |
| `finance.approve` | Memberikan persetujuan pada transaksi yang menunggu review (jika ada workflow approval) | Menyetujui transaksi yang diajukan oleh jabatan lain, confirming transaksi untuk di-post |

**Kenapa dipisah?** Dalam arsitektur kontrol internal, pemisahan ini memungkinkan prinsip segregation of duties: orang yang bisa mengedit transaksi (update) tidak harus sama dengan orang yang bisa menyetujui transaksi (approve). Ini mencegah satu jabatan melakukan perubahan tanpa persetujuan.

Untuk RW yang saat ini READ-ONLY, pemisahan ini belum relevan, tapi perlu dipertahankan untuk saat endpoint write ditambahkan di masa depan.

---

## 3. Pemetaan Permission per Operasi

### 3.1 Operasi RW yang Saat Ini Tersedia (READ-ONLY)

| Operasi | HTTP Method | Endpoint | Permission | Jabatan Butuh | Handler | Service | Status |
|---|---|---|---|---|---|---|---|
| Dashboard KAS | GET | `/api/v1/rw/{rwId}/dashboard` | `finance.read` | ketua, wakil_ketua, sekretaris, bendahara | `rw.Handler.GetDashboard` | `rw.Service.GetDashboard` | READ-ONLY |
| KAS Report | GET | `/api/v1/rw/{rwId}/kas` | `finance.read` | ketua, wakil_ketua, sekretaris, bendahara | `rw.Handler.GetKAS` | `rw.Service.GetKASReport` | READ-ONLY |
| Riwayat Transaksi | GET | `/api/v1/rw/{rwId}/kas/transactions` | `finance.read` | ketua, wakil_ketua, sekretaris, bendahara | `rw.Handler.GetKASTransactions` | `rw.Service.GetTransactions` | READ-ONLY |
| Daftar Lokasi Dana | GET | `/api/v1/rw/{rwId}/locations` | `finance.read` | ketua, wakil_ketua, sekretaris, bendahara | `rw.Handler.GetLocations` | `rw.Service.GetLocations` | READ-ONLY |

### 3.2 Operasi RW yang Belum Ada (Future)

| Operasi | HTTP Method | Endpoint | Permission | Jabatan Butuh | Status |
|---|---|---|---|---|---|
| Buat Transaksi KAS | POST | `/api/v1/rw/{rwId}/kas/transactions` | `finance.create` | ketua, wakil_ketua, bendahara | BELUM ADA |
| Koreksi Transaksi | PATCH | `/api/v1/rw/{rwId}/transactions/{id}` | `finance.update` | ketua, bendahara | BELUM ADA |
| Reversal Transaksi | POST | `/api/v1/rw/{rwId}/transactions/{id}/reverse` | `finance.approve` | ketua, wakil_ketua, bendahara | BELUM ADA |
| Transfer Antar-Lokasi | POST | `/api/v1/rw/{rwId}/transfers` | `finance.create` + `finance.update` | bendahara (RW) | BELUM ADA |
| Set Opening Balance | PUT | `/api/v1/rw/{rwId}/opening-balance` | `finance.update` | ketua, bendahara | BELUM ADA |

### 3.3 Penjelasan Pemetaan Permission

**Semua operasi baca (GET)** menggunakan `finance.read`. Alasan:
- Dashboard menampilkan ringkasan keuangan RW
- KAS report menampilkan income/expense
- Riwayat transaksi menampilkan daftar transaksi
- Lokasi dana menampilkan daftar lokasi dana

**Operasi create/write (POST)** akan menggunakan `finance.create`. Alasan:
- Membuat transaksi KAS baru merupakan operasi write
- Menambah opening balance merupakan operasi write
- Membuat transfer merupakan operasi write

**Operasi update (PATCH/PUT)** akan menggunakan `finance.update`. Alasan:
- Koreksi transaksi memerlukan perubahan pada data yang sudah ada
- Transfer memerlukan perubahan saldo dari dua lokasi berbeda

**Operasi approve (POST)** akan menggunakan `finance.approve`. Alasan:
- Reversal transaksi memerlukan persetujuan (bukan semua jabatan bisa membalikkan transaksi)
- Menyetujui transaksi yang menunggu review

**Jika operasi melibatkan lebih dari satu permission:**
- Transfer antar-lokasi memerlukan `finance.create` (membuat record transfer) DAN `finance.update` (mengubah saldo lokasi)
- Urutan: cek `finance.create` dulu, lalu `finance.update`
- Jika salah satu ditolak → 403 Forbidden

---

## 4. Rekomendasi Enforcement Architecture

### 4.1 Dua Pendekatan yang Dipertimbangkan

#### A. Permission Check di Handler

```go
func (h *Handler) GetDashboard(w http.ResponseWriter, r *http.Request) {
    ac, ok := getAuthContext(r)
    if !ok {
        httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
        return
    }
    rwID := chi.URLParam(r, "rwId")
    
    // Permission check di handler
    jabatan := ac.RWMemberships.findByRWID(rwID).Jabatan
    if !hasRWScopePermission(jabatan, "finance.read") {
        httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "insufficient permissions")
        return
    }
    
    summary, err := h.svc.GetDashboard(r.Context(), ac, rwID)
    // ...
}
```

**Kelebihan:**
- Eksplisit, mudah dibaca di setiap handler
- Handler tahu persis permission apa yang dibutuhkan
- Tidak perlu mengubah service interface

**Kekurangan:**
- Handler bisa lupa memeriksa permission
- Jika service bisa dipanggil dari selain HTTP, permission bisa terlewat
- Duplikasi logika permission check di setiap handler
- Tidak konsisten dengan pola `RequirePermission` yang sudah ada di auth module

#### B. Permission Check di Service

```go
func (s *Service) GetDashboard(ctx context.Context, ac *auth.AuthContext, rwID string) (*DashboardSummary, error) {
    if err := s.checkRWAccess(ac, rwID); err != nil {
        return nil, err
    }
    if err := s.checkRWRoleship(ac, rwID, "finance.read"); err != nil {
        return nil, err
    }
    // ...
}
```

**Kelebihan:**
- Satu tempat untuk semua permission check
- Service bisa dipanggil dari manapun (HTTP, worker, CLI) → permission selalu enforced
- Tidak ada handler yang bisa lupa memeriksa permission
- Konsisten dengan pola `checkRWAccess` yang sudah ada

**Kekurangan:**
- Service jadi tahu tentang permission codes
- Service interface perlu didekorasi atau dimodifikasi
- Stack trace bisa lebih dalam

### 4.2 Evaluasi Detail

| Faktor | Handler | Service |
|---|---|---|
| Semua call site protected? | Tidak — handler bisa lupa | Ya — service adalah satu-satunya entry point |
| Service dipanggil dari jalur selain HTTP? | Tidak relevan | Ya — service bisa dipanggil dari worker, CLI, atau test |
| Mencegah handler lupa permission? | Tidak — perlu reminder developer | Ya — automatic di service |
| Mencegah permission salah scope? | Bergantung handler | Ya — service menerima rwID sebagai parameter |
| Kompatibel dengan SA bypass? | Ya | Ya |
| Konsisten dengan pola existing? | Ya (handler-level check ada di fund_location) | Ya (`checkRWAccess` sudah ada) |

### 4.3 Rekomendasi: Service-Layer Enforcement dengan Middleware Optional

**Arsitektur yang direkomendasikan:**

```
HTTP Request
    │
    ▼
RequireAuth (middleware) ──── JWT validasi + AuthContext injection
    │
    ▼
RequireRWScope (middleware) ──── Scope validation + membership check + SA verification
    │
    ▼
Handler ──── Extract rwId, decode request body
    │
    ▼
Service.checkRWAccess(ac, rwID) ──── Membership SA validation (existing)
    │
    ▼
Service.checkRWPermission(ac, rwID, permissionCode) ──── Jabatan permission check (NEW)
    │
    ▼
Repository query ──── Parameterized rw_id filter
```

**Detail implementasi:**

1. **Tambah `checkRWPermission` di `rw.Service`:**
   ```go
   func (s *Service) checkRWPermission(ac *auth.AuthContext, rwID string, requiredPermission string) error {
       if ac.SystemRole == auth.SystemRoleSuperAdmin {
           return nil // SA always passes
       }
       for _, rm := range ac.RWMemberships {
           if rm.RWID == rwID {
               if hasRWScopePermission(rm.Jabatan, requiredPermission) {
                   return nil
               }
               return fmt.Errorf("%w: required %s for jabatan %s", ErrPermissionDenied, requiredPermission, rm.Jabatan)
           }
       }
       return fmt.Errorf("%w: no RW membership for %s", ErrRWAccessDenied, rwID)
   }
   ```

2. **Panggil dari setiap service method sebelum query:**
   ```go
   func (s *Service) GetDashboard(ctx context.Context, ac *auth.AuthContext, rwID string) (*DashboardSummary, error) {
       if err := s.checkRWAccess(ac, rwID); err != nil { return nil, err }
       if err := s.checkRWPermission(ac, rwID, "finance.read"); err != nil { return nil, err }
       // ... existing logic
   }
   ```

3. **Handler tetap ada untuk validasi input** (rwId extraction, body decode, pagination) tapi tidak memeriksa permission.

**Mengapa bukan middleware?**
- Middleware `RequireRWScope` sudah bertugas memvalidasi scope dan membership
- Permission check memerlukan jabatan yang didapat dari `RWMemberships` setelah `RequireRWScope` berhasil
- Middleware perlu tahu operation type (read/create/update/approve) yang tidak selalu tersedia di middleware level
- Service-level check lebih konsisten dengan pola existing (`checkRWAccess`) dan bisa dipanggil dari non-HTTP context

---

## 5. Kebijakan Super Admin

### 5.1 Kebijakan yang Sudah Ada (Dipertahankan)

| Aturan | Implementasi | File |
|---|---|---|
| SA bisa akses semua RW dalam `SCOPED_RW_IDS` | `cfg.hasRW(rwID)` | `scoping.go:118` |
| SA perlu verifikasi RW aktif di DB | `SELECT is_active FROM rws WHERE id = $1` | `scoping.go:138` |
| SA bypass semua membership check | `ac.SystemRole == SystemRoleSuperAdmin` | `scoping.go:127` |
| SA tidak boleh akses RW di luar allowlist | `cfg.hasRW(rwID)` return false → 403 | `scoping.go:118` |

### 5.2 Kebijakan SA untuk Permission

**SA Bypass Semua Permission Jabatan.**

Alasan:
1. SA adalah platform administrator, bukan jabatan RT/RW
2. SA tidak seharusnya memiliki "jabatan palsu" di RW manapun
3. SA sudah dibatasi oleh `SCOPED_RW_IDS` — hanya bisa akses RW yang dikonfigurasi
4. SA sudah divalidasi oleh `RequireAuth` + `RequireRWScope`

**Implementasi:**
```go
func (s *Service) checkRWPermission(ac *auth.AuthContext, rwID string, requiredPermission string) error {
    if ac.SystemRole == auth.SystemRoleSuperAdmin {
        return nil // SA always passes — validated by middleware
    }
    // ... regular permission check
}
```

### 5.3 Batasan SA

| Batasan | Implementasi | Status |
|---|---|---|
| SA tidak boleh akses RW di luar `SCOPED_RW_IDS` | Middleware `RequireRWScope` | ✅ Existing |
| SA tidak boleh akses RW yang `is_active = false` | Middleware `RequireRWScope` + DB query | ✅ Existing |
| SA tidak bisa mengakses data RW lain | Repository `rw_id = $1` filter | ✅ Existing |
| SA tidak memerlukan membership jabatan | Bypass di `checkRWAccess` + `checkRWPermission` | ✅ Proposed |

---

## 6. HTTP Error dan Security Behavior

### 6.1 Error Response Matrix

| Kondisi | Status Code | Error Code | Message | Contoh |
|---|---|---|---|---|
| Tidak terautentikasi | 401 | `unauthorized` | `"missing authorization header"` / `"authentication failed"` | Header Authorization tidak ada atau format salah |
| Token invalid/expired | 401 | `invalid_token` / `token_expired` | `"authentication failed"` / `"access token expired"` | JWT signature salah atau expired |
| Tidak memiliki scope RW | 403 | `forbidden` | `"rw not in configured scope"` | RW tidak di `SCOPED_RW_IDS` |
| RW tidak aktif | 403 | `forbidden` | `"rw not found or inactive"` | RW exists tapi `is_active = false` |
| Tidak memiliki membership RW | 403 | `forbidden` | `"rw access denied"` | User tidak punya entry di `user_rw_memberships` untuk rwId ini |
| Tidak memiliki permission | 403 | `forbidden` | `"insufficient permissions"` | Sekretaris mencoba create transaksi |
| Jabatan tidak dikenal | 403 | `forbidden` | `"insufficient permissions"` | Jabatan tidak ada di `rwPermissionMap` → `hasRWScopePermission` return false |

### 6.2 Perilaku Keamanan untuk Resource Cross-RW

**Prinsip:** Resource di RW lain harus selalu ditampilkan sebagai **404**, bukan 403.

Alasan:
- 403 membocorkan informasi bahwa resource tersebut ada tapi user tidak bisa mengakses
- 404 melindungi informasi tentang keberadaan resource di RW lain
- Ini konsisten dengan prinsip security through obscurity untuk tenant isolation

**Implementasi:**
```go
func (h *Handler) handleError(w http.ResponseWriter, err error) {
    switch {
    case errors.Is(err, ErrRWAccessDenied):
        httpx.ErrorJSON(w, http.StatusNotFound, "not_found", "resource not found")
        // 404, bukan 403, untuk mencegah data leakage
    case errors.Is(err, ErrPermissionDenied):
        httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "insufficient permissions")
    // ...
    }
}
```

**Pengecualian:** Error "insufficient permissions" tetap 403 karena ini terkait **jabatan**, bukan **scope**. User sudah terbukti berada di scope yang benar, tapi jabatannya tidak memiliki permission yang diminta.

### 6.3 Respons Tidak BOCORKAN Data

| Sumber Kebocoran | Pencegahan |
|---|---|
| rwId dari URL | rwId hanya digunakan di query parameterized (`rw_id = $1`) |
| rwId dari response | Response hanya berisi data RW yang di-request, tidak ada identifier RW lain |
| rwId dari pagination | Pagination tidak menyertakan rwId di response metadata |
| Error messages | Error message untuk access denial → "resource not found", bukan "access denied for rw-X" |

---

## 7. Rencana Regression Test

### 7.A. Test Permission

| Test ID | Deskripsi | Jabatan | Permission Test | Expected |
|---|---|---|---|---|
| `TestRWPermissionSekretarisRead` | Sekretaris bisa baca dashboard | sekretaris | `finance.read` | 200 OK |
| `TestRWPermissionSekretarisCreateBlocked` | Sekretaris tidak bisa create transaksi | sekretaris | `finance.create` | 403 Forbidden |
| `TestRWPermissionSekretarisUpdateBlocked` | Sekretaris tidak bisa koreksi transaksi | sekretaris | `finance.update` | 403 Forbidden |
| `TestRWPermissionSekretarisApproveBlocked` | Sekretaris tidak bisa approve | sekretaris | `finance.approve` | 403 Forbidden |
| `TestRWPermissionWakilKetuaCreateAllowed` | Wakil ketua bisa create transaksi | wakil_ketua | `finance.create` | 200 OK |
| `TestRWPermissionWakilKetuaUpdateBlocked` | Wakil ketua tidak bisa update transaksi | wakil_ketua | `finance.update` | 403 Forbidden |
| `TestRWPermissionWakilKetuaApproveAllowed` | Wakil ketua bisa approve | wakil_ketua | `finance.approve` | 200 OK |
| `TestRWPermissionKetuaFullAccess` | Ketua bisa semua operasi | ketua | `finance.read/create/update/approve` | 200 OK |
| `TestRWPermissionBendaharaFullAccess` | Bendahara bisa semua operasi | bendahara | `finance.read/create/update/approve` | 200 OK |
| `TestRWPermissionUnknownJabatanDenied` | Jabatan tidak dikenal ditolak | "unknown_jabatan" | semua | 403 Forbidden |
| `TestRWPermissionNoJabatanDenied` | User tanpa jabatan RW ditolak | "" (kosong) | semua | 403 Forbidden |

### 7.B. Test Tenant Isolation

| Test ID | Deskripsi | Expected |
|---|---|---|
| `TestRWTenantIsolationDashboard` | User RW A tidak bisa baca dashboard RW B | 403/404 |
| `TestRWTenantIsolationTransactions` | User RW A tidak bisa baca transaksi RW B | 403/404 |
| `TestRWTenantIsolationLocations` | User RW A tidak bisa akses lokasi RW B | 403/404 |
| `TestRWTenantIsolationRWIDTamper` | User RW A mengirim request ke RW C | 403/404 |
| `TestRWTenantIsolationRepository` | Query repo RW A tidak return data RW B | 0 rows returned |

### 7.C. Test Super Admin

| Test ID | Deskripsi | Expected |
|---|---|---|
| `TestSAReadInScopedRW` | SA akses dashboard RW dalam allowlist | 200 OK |
| `TestSAWriteInScopedRW` | SA create transaksi di RW dalam allowlist | 200 OK |
| `TestSAReadOutsideScopedRW` | SA akses dashboard RW di luar allowlist | 403 Forbidden |
| `TestSAInactiveRWBlocked` | SA akses RW yang is_active = false | 403 Forbidden |
| `TestSANoJabatanRequired` | SA tidak perlu membership jabatan | 200 OK (tanpa membership) |

### 7.D. Test Regression RT

| Test ID | Deskripsi | Expected |
|---|---|---|
| `TestRWPermNoEffectOnRTRBAC` | Permission RW tidak mengubah akses RT | RT RBAC unchanged |
| `TestRTPengurusNoRWAutoAccess` | Pengurus RT tidak otomatis dapat akses RW | 403 Forbidden |
| `TestRTMembershipNotValidForRW` | Membership RT tidak bisa dipakai untuk akses RW | 403 Forbidden |
| `TestRWPermDoesNotBreakExisting` | Perubahan RW tidak merusak endpoint RT | Existing RT tests pass |

### 7.E. Test JWT

| Test ID | Deskripsi | Expected |
|---|---|---|
| `TestJWTValidPassesAuth` | Token valid melewati RequireAuth | AuthContext tersedia |
| `TestJWTExpiredRejected` | Token expired ditolak RequireAuth | 401 Unauthorized |
| `TestJWTInvalidSignatureRejected` | Signature salah ditolak RequireAuth | 401 Unauthorized |
| `TestJWTValidWithScopeCheck` | Token valid tapi RW tidak di scope | 403 Forbidden |
| `TestJWTValidButNoPermission` | Token valid, scope OK, tapi jabatan tidak punya permission | 403 Forbidden |

---

## 8. File Impact dan Risiko

### 8.1 File Produksi yang Perlu Diubah

| File | Perubahan | Risiko |
|---|---|---|
| `internal/auth/scoping.go` | Export `hasRWScopePermission` (lower → upper case) | LOW — existing test references masih valid |
| `internal/rw/service.go` | Tambah `checkRWPermission` dan panggil di setiap public method | MEDIUM — perlu test coverage |
| `internal/rw/handler.go` | Tambah error type `ErrPermissionDenied` | LOW — hanya error handling |
| `internal/rw/model.go` | Tambah `ErrPermissionDenied` error variable | LOW — hanya error definition |

### 8.2 File Test yang Perlu Ditambah/Diubah

| File | Perubahan |
|---|---|
| `internal/rw/service_test.go` | Tambah test untuk `checkRWPermission` |
| `internal/rw/rw_integration_test.go` | Tambah test permission enforcement per jabatan |
| `internal/auth/scoping_test.go` | Tambah test `hasRWScopePermission` edge cases |

### 8.3 Risiko Perubahan

| Risiko | Mitigasi |
|---|---|
| Breaking existing tests jika SA behavior berubah | SA bypass permission dipertahankan — tidak ada perubahan pada SA test |
| Service jadi lebih lambat karena extra permission check | 1 additional loop per request → negligible impact |
| Error response berubah dari 403 ke 404 untuk cross-RW | Perubahan ini security improvement, bukan regression |
| Permission map hardcoded → sulit diubah di production | Map bisa dipindahkan ke DB (position_permissions) di fase selanjutnya |

### 8.4 Kompatibilitas dengan RT

- Permission RW **tidak terkait** dengan RT RBAC (`RequireRole`, `RequirePermission` RT)
- RW namespace berbeda: `/api/v1/rw/{rwId}/...` vs `/api/v1/bills`, `/api/v1/transactions`
- RW menggunakan `RWMemberships` (jabatan RW) vs RT menggunakan `role` + `jabatan` dari JWT
- Tidak ada overlap antara sistem permission RT dan RW
- Semua perubahan RW bersifat lokal ke package `internal/rw` dan `internal/auth/scoping.go`

### 8.5 Menjaga Working Tree Changes

- Semua perubahan RW bersifat non-breaking untuk RT, household, resident, finance module
- Tidak ada perubahan database schema
- Tidak ada perubahan API contract (semua endpoint tetap GET)
- Tidak ada perubahan pada auth middleware `RequireAuth` yang digunakan oleh semua module

---

## 9. Keputusan yang Butuh Persetujuan

### 9.1 Apakah SA Perlu Bypass Semua Permission?

**Rekomendasi:** YA — SA bypass semua permission jabatan.

**Pertimbangan:**
- ✅ SA sudah dibatasi oleh `SCOPED_RW_IDS`
- ✅ SA tidak memiliki jabatan RT/RW, jadi pengecekan jabatan tidak relevan
- ✅ Jika SA dibatasi permission, SA perlu "jabatan default" → arsitektur yang rumit

**Alternatif (jika user ingin SA tetap terikat permission):**
- SA menggunakan jabatan "super_admin" yang tidak ada di `rwPermissionMap`
- Semua pengecekan permission untuk SA return false
- SA perlu "override" khusus → membingungkan

### 9.2 Apakah Response untuk Cross-RW Harus 404 atau 403?

**Rekomendasi:** 404 untuk cross-RW access, 403 untuk insufficient permission.

**Pertimbangan:**
- ✅ 404 melindungi informasi tentang keberadaan resource di RW lain
- ✅ 403 untuk permission denial (scope benar, jabatan salah)
- ✅ Konsisten dengan security best practices

**Alternatif:** Selalu 403 untuk semua authorization failure → lebih sederhana tapi membocorkan informasi.

### 9.3 Apakah Perlu Middleware Permission atau Service-Level?

**Rekomendasi:** Service-level (`checkRWPermission` di `rw.Service`).

**Pertimbangan:**
- ✅ Satu tempat untuk semua permission check
- ✅ Service bisa dipanggil dari HTTP, worker, CLI → permission selalu enforced
- ✅ Tidak ada handler yang bisa lupa permission
- ✅ Konsisten dengan pola `checkRWAccess` yang sudah ada

**Alternatif:** Middleware `RequireRWPermission` → perlu tahu operation type (read/create/update) yang tidak selalu tersedia di middleware.

---

## 10. Verdict

### DESIGN NEEDS DECISION

Dokumen desain ini lengkap secara teknis tapi memerlukan persetujuan pengguna pada 3 keputusan:

| Keputusan | Rekomendasi | Status |
|---|---|---|
| SA bypass permission | Ya — SA bypass semua permission jabatan | ⏳ Needs Decision |
| Response code cross-RW | 404 untuk cross-RW, 403 untuk permission denied | ⏳ Needs Decision |
| Enforcement location | Service-layer (`checkRWPermission` di `rw.Service`) | ⏳ Needs Decision |

Setelah ketiga keputusan disetujui, status berubah menjadi **DESIGN READY** dan implementasi bisa dimulai.

---

## Appendiks

### A. Kode Referensi: `rwPermissionMap` yang Sudah Ada

```go
// scoping.go:243-264
var rwPermissionMap = map[string][]string{
    "ketua": {
        "finance.read",
        "finance.create",
        "finance.update",
        "finance.approve",
    },
    "wakil_ketua": {
        "finance.read",
        "finance.create",
        // TIDAK punya finance.update
        "finance.approve",
    },
    "sekretaris": {
        "finance.read",
    },
    "bendahara": {
        "finance.read",
        "finance.create",
        "finance.update",
        "finance.approve",
    },
}
```

### B. Kode Referensi: `checkRWBendaharaAccess` di Fund Location

Fund location handler sudah memiliki pattern yang bisa diadopsi:

```go
// fund_location/handler.go:117-128
func (h *Handler) checkRWBendaharaAccess(ac *auth.AuthContext, rwID string) error {
    if ac.SystemRole == auth.SystemRoleSuperAdmin {
        return nil
    }
    for _, rm := range ac.RWMemberships {
        if rm.RWID == rwID && rm.Jabatan == "bendahara" {
            return nil
        }
    }
    return errors.New("only bendahara can execute or reverse fund transfers at RW level")
}
```

Pattern ini menunjukkan bahwa `RWMemberships` sudah digunakan di produksi untuk pengecekan jabatan.

### C. Test Count Referensi (dari audit sebelumnya)

| Package | Parent Test | Subtest |
|---|---|---|
| `internal/auth` | 147 | 0 |
| `internal/rw` | 33 | 0 |
| **TOTAL** | **180** | **0** |
