# Laporan Perbaikan: Super Admin 403 on RW Endpoints

## 1. Root Cause Analysis

### Error yang Ditemukan
```json
{
  "error": {
    "code": "forbidden",
    "message": "rw access requires active rw membership"
  }
}
```

### Lokasi dan Penyebab

**Lapisan 1 — `backend/internal/auth/scoping.go` → `RequireRWScope()`**

Di line ~174-185, middleware memeriksa apakah `ac.SystemRole == SystemRoleSuperAdmin` DAN `ac.RTID == "" && ac.MembershipID == ""`. Jika kondisi ini true (SA tanpa membership), middleware TETAP mensyaratkan SA punya RW membership:

```go
if ac.SystemRole == SystemRoleSuperAdmin && ac.RTID == "" && ac.MembershipID == "" {
    if len(ac.RWMemberships) == 0 {
        httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "rw access requires rw membership")
        return
    }
    // ... DB check ...
}
```

**Masalah:** Branch ini dibuat untuk SA TANPA membership, tapi logikanya MEMILIKI membership check sebelum lanjut ke DB verification. Ini berarti SA tanpa membership langsung ditolak 403.

**Lapisan 2 — `backend/internal/rw/service.go` → `checkRWAccess()`**

Method ini memeriksa membership di setiap service method tanpa melihat `SystemRole`:

```go
func (s *Service) checkRWAccess(ac *auth.AuthContext, rwID string) error {
    for _, rm := range ac.RWMemberships {
        if rm.RWID == rwID {
            return nil
        }
    }
    return fmt.Errorf("%w: rw %s", ErrRWAccessDenied, rwID)
}
```

**Masalah:** Tidak ada bypass untuk Super Admin. Meskipun middleware sudah mengizinkan akses, service layer juga melakukan membership check yang sama.

### Root Cause Summary
Super Admin tanpa RW membership ditolak di dua lapisan:
1. **Middleware `RequireRWScope()`**: Branch SA tanpa membership masih mensyaratkan membership check sebelum DB check
2. **Service `checkRWAccess()`**: Selalu memeriksa membership tanpa melihat system role

---

## 2. File yang Diubah

### 2.1 `backend/internal/auth/scoping.go`

**Perubahan:** Memisahkan branch "System-level SUPER_ADMIN" yang benar-benar bypass membership check.

**Before:**
```go
if ac.SystemRole == SystemRoleSuperAdmin && ac.RTID == "" && ac.MembershipID == "" {
    if len(ac.RWMemberships) == 0 {
        httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "rw access requires rw membership")
        return
    }
    // DB check
}
```

**After:**
```go
if ac.SystemRole == SystemRoleSuperAdmin {
    // Directly verify RW active status in DB, no membership check needed.
    // SA is authorized by system role alone, subject to:
    // 1. The RW ID must be in configured SCOPED_RW_IDS (checked above).
    // 2. The RW must exist and be active in the database.
    
    if pool == nil {
        httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "database unavailable")
        return
    }
    // ... DB check for active RW ...
    next.ServeHTTP(w, r)
    return
}

// Users WITHOUT super_admin system role
if len(ac.RWMemberships) == 0 {
    httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "rw access requires active rw membership")
    return
}
```

### 2.2 `backend/internal/rw/service.go`

**Perubahan:** Menambahkan Super Admin bypass di `checkRWAccess()`.

**Before:**
```go
func (s *Service) checkRWAccess(ac *auth.AuthContext, rwID string) error {
    for _, rm := range ac.RWMemberships {
        if rm.RWID == rwID {
            return nil
        }
    }
    return fmt.Errorf("%w: rw %s", ErrRWAccessDenied, rwID)
}
```

**After:**
```go
func (s *Service) checkRWAccess(ac *auth.AuthContext, rwID string) error {
    if ac == nil {
        return fmt.Errorf("no authentication context")
    }
    // Super Admin bypass: system-level authorization, no membership required.
    if ac.SystemRole == auth.SystemRoleSuperAdmin {
        return nil
    }
    for _, rm := range ac.RWMemberships {
        if rm.RWID == rwID {
            return nil
        }
    }
    return fmt.Errorf("%w: rw %s", ErrRWAccessDenied, rwID)
}
```

### 2.3 `backend/internal/auth/scoping_test.go`

**Perubahan:**
1. Update `TestRequireRWScopeSuperAdminNoMemberships` — expected 403 → 500 (karena pool nil, SA sekarang bypass membership check)
2. Tambah 5 test baru:
   - `TestRequireRWScopeSuperAdminNoMembershipsRWInScope` — SA tanpa membership, RW dalam scope → 500 (pool nil) / 200 (dengan pool + active RW)
   - `TestRequireRWScopeSuperAdminNoMembershipsRWScopeNotInList` — SA tanpa membership, RW di luar scope → 403
   - `TestRequireRWScopeUserWithoutRWMembership` — user biasa tanpa membership → 403
   - `TestRequireRWScopeUserWithWrongMembershipRW` — user dengan membership RW lain → 403
   - `TestRequireRWScopeSAWithMultipleScopeAccess` — SA bisa akses semua RW dalam configured scope

---

## 3. Hasil Test

### Semua test RequireRWScope: 20/20 PASS

| Test | Status |
|------|--------|
| TestRequireRWScopeRejectsOutOfScopeRW | ✅ PASS |
| TestRequireRWScopeAcceptsInScopeRW | ✅ PASS |
| TestRequireRWScopeWithEmptyRWIDs | ✅ PASS |
| TestRequireRWScopeSAWithMembershipsEmptyScope | ✅ PASS |
| TestRequireRWScopeSuperAdminWithMembershipsPoolNil | ✅ PASS |
| TestRequireRWScopeSAWithMembershipsButNoAllowlist | ✅ PASS |
| TestRequireRWScopeUserMultipleMembershipsMatchRWID | ✅ PASS |
| TestRequireRWScopeMultipleMembershipsWrongRW | ✅ PASS |
| TestRequireRWScopeUserHasRWMembershipButNotInScope | ✅ PASS |
| TestRequireRWScopeSuperAdminNoMemberships | ✅ PASS |
| TestRequireRWScopeSuperAdminWithMembershipsAndScopeAllowlist | ✅ PASS |
| TestRequireRWScopeUserWithMatchingMembership | ✅ PASS |
| TestRequireRWScopeUserWrongRW | ✅ PASS |
| TestRequireRWScopeNoRWContext | ✅ PASS |
| TestRequireRWScopeInvalidRWPath | ✅ PASS |
| **TestRequireRWScopeSuperAdminNoMembershipsRWInScope** | ✅ PASS (BARU) |
| **TestRequireRWScopeSuperAdminNoMembershipsRWScopeNotInList** | ✅ PASS (BARU) |
| **TestRequireRWScopeUserWithoutRWMembership** | ✅ PASS (BARU) |
| **TestRequireRWScopeUserWithWrongMembershipRW** | ✅ PASS (BARU) |
| **TestRequireRWScopeSAWithMultipleScopeAccess** | ✅ PASS (BARU) |

### Backend Build
✅ `go build ./...` — SUCCESS, no errors

### Test Isolation Issues (EXISTING)
2 test mengalami failure saat dijalankan bersama dengan test lain, tetapi PASS saat dijalankan sendiri:
- `TestMeEndpointRWMembershipsMultiple`
- `TestMeEndpointNoRWMembership`

Ini adalah **test isolation issue yang sudah ada sebelum perubahan**, disebabkan oleh duplicate key constraint pada database test fixture, bukan disebabkan oleh perubahan saya.

---

## 4. Checklist Verifikasi Lengkap

| Pemeriksaan | Hasil | Keterangan |
|-------------|-------|------------|
| Git commit Batch 1 (`1d154d2`) | ✅ VERIFIED | Tepat 11 file, perubahan Batch 2 tetap utuh |
| Backend health check | ✅ HEALTHY | `{"database":"healthy","status":"ok"}` |
| Database migration (027, 028, 029) | ✅ APPLIED | Version 41, tabel `rws` & `user_rw_memberships` ada |
| Konfigurasi `SCOPED_RT_IDS` | ✅ SET | 2 RT configured |
| Konfigurasi `SCOPED_RW_IDS` | ✅ SET | 1 RW configured (94aa4a72-aa7a-47ff-93f4-a7b9cb187b59 = RW 016) |
| Web TypeScript check | ✅ PASS | 0 errors |
| Web production build | ✅ PASS | 3939 modules, 13.11s |
| Web sidebar RW scope | ✅ VERIFIED | Loop dynamic per `m.rw_id` |
| Web route RW | ✅ VERIFIED | Menggunakan parameter dinamis `/rw-view/:rwId/*` |
| Android Flutter version | ✅ 3.47.6 | Stable |
| Android flutter analyze | ✅ 4 info, 0 errors | Style warnings only |
| Android debug APK | ✅ BUILD SUCCESS | `build/app/outputs/flutter-apk/app-debug.apk` |
| Backend `go build ./...` | ✅ SUCCESS | No compilation errors |
| Test `RequireRWScope` | ✅ 20/20 PASS | Termasuk 5 test baru |

---

## 5. Checklist Manual UI Testing

### 5.1 Persiapan Testing
- [ ] Backend container berjalan (`localhost:8080`)
- [ ] Database terhubung dan migration 027-029 sudah diterapkan
- [ ] Frontend Web berjalan di browser
- [ ] APK Android tersedia untuk testing mobile

### 5.2 Testing Flow

| # | Skenario | Expected Result | Status |
|---|----------|-----------------|--------|
| 1 | Login Super Admin tanpa RW membership | Login berhasil, JWT mengandung `system_role: super_admin` | [ ] |
| 2 | Akses `/rw/94aa4a72-aa7a-47ff-93f4-a7b9cb187b59/dashboard` | **200 OK** — Dashboard RW terbuka (bukan 403) | [ ] |
| 3 | Akses `/rw/94aa4a72-aa7a-47ff-93f4-a7b9cb187b59/kas` | **200 OK** — KAS RW terbuka (bukan 403) | [ ] |
| 4 | Akses `/rw/94aa4a72-aa7a-47ff-93f4-a7b9cb187b59/kas/transactions` | **200 OK** — Riwayat transaksi terbuka | [ ] |
| 5 | Akses `/rw/94aa4a72-aa7a-47ff-93f4-a7b9cb187b59/locations` | **200 OK** — Lokasi Dana terbuka | [ ] |
| 6 | Login Pengguna dengan membership Ketua RW | Dashboard RW terbuka | [ ] |
| 7 | Pengguna tanpa membership RW mencoba akses RW | **403 Forbidden** — "rw access requires active rw membership" | [ ] |
| 8 | Pengguna RW mencoba akses RW lain | **403 Forbidden** — "rw access denied" | [ ] |
| 9 | Pengguna biasa tidak melihat menu RW | Menu RW tidak muncul di sidebar | [ ] |
| 10 | Pengguna dengan membership RW melihat menu RW | Menu RW muncul di sidebar | [ ] |
| 11 | Multi-RW: Pengguna dengan 2+ membership | Dropdown menampilkan seluruh RW | [ ] |
| 12 | Super Admin akses RW di luar configured scope | **403 Forbidden** — "rw not in configured scope" | [ ] |
| 13 | Fitur RT yang sudah berjalan | Tetap berfungsi normal (no regression) | [ ] |

### 5.3 Testing Flow (Lanjutan)

| # | Skenario | Expected Result | Status |
|---|----------|-----------------|--------|
| 14 | Super Admin akses RT dalam configured scope | 200 OK — Data RT terbuka | [ ] |
| 15 | Super Admin akses RT di luar configured scope | 403 Forbidden | [ ] |
| 16 | Bottom navigation Android horizontal scroll | Bisa scroll untuk melihat semua menu | [ ] |
| 17 | Mutasi finansial oleh Ketua RW | Sesuai permission (create, update, approve) | [ ] |
| 18 | Mutasi finansial oleh Sekretaris RW | Read-only (tidak bisa create/update) | [ ] |

---

## 6. Status Kesiapan

### **READY WITH LIMITATIONS**

#### Yang Sudah Siap:
- ✅ Backend: 403 pada Super Admin sudah diperbaiki
- ✅ Database: Migration 027-029 sudah diterapkan
- ✅ Konfigurasi: `SCOPED_RT_IDS` dan `SCOPED_RW_IDS` configured
- ✅ Web: Build berhasil, TypeScript 0 errors, routing RW benar
- ✅ Android: Build APK berhasil, route RW tersedia
- ✅ Test: 20/20 RequireRWScope test PASS

#### Keterbatasan / Catatan:
1. **Token Testing Belum Tersedia**: Tidak ada kredensial login yang valid untuk akun dengan membership RW aktif. User `andriyan@local.com` ada di database dengan RW membership, tetapi password tidak diketahui. Perlu kredensial login yang valid untuk testing UI end-to-end.
2. **Test Isolation Issues**: 2 test `TestMeEndpointRWMembershipsMultiple` dan `TestMeEndpointNoRWMembership` mengalami failure saat dijalankan bersamaan dengan test lain (sudah ada sebelum perubahan), tetapi PASS saat dijalankan sendiri.
3. **Endpoint RW Belum Teregistrasi di Router**: Belum ditemukan routing explicit `/rw/{rwId}/...` yang mendaftahkan handler RW. Perlu verifikasi bahwa handler RW sudah terdaftar di `main.go` atau router utama.
4. **Live API Testing Terbatas**: Endpoint RW belum bisa diakses tanpa token valid. Test HTTP request sebelumnya menghasilkan 401 (unauthenticated) karena tidak ada kredensial yang valid.

#### Yang Perlu Disiapkan Sebelum Testing UI:
1. **Kredensial Login**: Pastikan ada user dengan password yang diketahui dan RW membership aktif
2. **Verifikasi RW Handler Registration**: Pastikan endpoint `/api/v1/rw/{rwId}/...` sudah terdaftar di router
3. **Frontend Navigation**: Pastikan sidebar dan navigation sudah menampilkan opsi RW untuk user yang memiliki membership
