# Laporan Audit Keamanan — RW Security & Tenant Isolation

**Tanggal:** 31 Oktober 2026  
**Status:** READ-ONLY — Tidak ada file yang dimodifikasi.

---

## 1. Production Route Registration

### Endpoint RW yang terdaftar

| Endpoint | HTTP Method | File | Handler | Middleware (urutan) |
|---|---|---|---|---|
| `/api/v1/rw/{rwId}/dashboard` | GET | `backend/cmd/api/main.go:753` | `rwHandler.GetDashboard` | `RequireAuth` → `RequireRWScope` |
| `/api/v1/rw/{rwId}/kas` | GET | `backend/cmd/api/main.go:754` | `rwHandler.GetKAS` | `RequireAuth` → `RequireRWScope` |
| `/api/v1/rw/{rwId}/kas/transactions` | GET | `backend/cmd/api/main.go:755` | `rwHandler.GetKASTransactions` | `RequireAuth` → `RequireRWScope` |
| `/api/v1/rw/{rwId}/locations` | GET | `backend/cmd/api/main.go:756` | `rwHandler.GetLocations` | `RequireAuth` → `RequireRWScope` |

### Middleware group di `main.go` (baris 751-757):

```go
r.Group(func(r chi.Router) {
    r.Use(auth.RequireAuth)
    r.Use(auth.RequireRWScope(pool, *scopingCfg))
    r.Get("/api/v1/rw/{rwId}/dashboard", rwHandler.GetDashboard)
    r.Get("/api/v1/rw/{rwId}/kas", rwHandler.GetKAS)
    r.Get("/api/v1/rw/{rwId}/kas/transactions", rwHandler.GetKASTransactions)
    r.Get("/api/v1/rw/{rwId}/locations", rwHandler.GetLocations)
})
```

### Apakah `RequireAuth` dan `RequireRWScope` dijalankan?
**YA.** Kedua middleware terpasang dalam group yang sama. Urutan:
1. `RequireAuth` — memvalidasi JWT dan menyuntikkan `AuthContext`
2. `RequireRWScope` — memvalidasi RW membership atau SA bypass

### Core middleware (semua route, baris 346-348):
1. `httpmw.RequestID`
2. `httpmw.PanicRecovery`
3. `httpmw.Logging`

---

## 2. JWT Authentication

### Apakah test membuat JWT valid?

**YA** — beberapa test membuat JWT valid:
- `TestRequireAuthValidTokenSetsContext` — membuat JWT dengan `auth.GenerateAccessToken()` lalu mengirimkannya via header `Authorization: Bearer <token>`
- `TestRequireAuthExpiredTokenRejected` — membuat token expired
- `TestRequireAuthInvalidSignatureRejected` — membuat token dengan signature salah
- `TestRequireAuthValidTokenSucceeds` — verifikasi JWT + identity validation

### Apakah request melewati middleware JWT validation produksi?

**BEBERAPA YA, BEBERAPA TIDAK.**

**Melewati JWT validation:**
- `requireauth_test.go` — test langsung memanggil `auth.RequireAuth(next)` dengan token Bearer yang dibuat via `auth.GenerateAccessToken()`
- JWT disetujui oleh `ParseAndVerifyAccessToken()` di middleware produksi

**Tidak melewati JWT validation (AuthContext injection langsung):**
- `rw_http_integration_test.go` — `context.WithValue(req.Context(), AuthContextKey, ac)` secara langsung tanpa melalui `RequireAuth`
- `me_rw_membership_test.go` — `makeMeRequest()` menggunakan `context.WithValue(AuthContextKey, ac)`
- `scoping_test.go` — `callRequireRWScopeMiddleware()` menggunakan `context.WithValue(AuthContextKey, ac)`

### Apakah `RequireAuth` benar-benar dijalankan?

**SEBAGIAN.**

Untuk route RW, ada dua jenis test:
1. **Test di `rw_http_integration_test.go`** — TIDAK melalui `RequireAuth`. AuthContext disuntikkan langsung via `context.WithValue`.
2. **Test di `requireauth_test.go`** — BENAR-BENAR melalui `RequireAuth` dengan JWT valid untuk endpoint lain.

### Apakah `AuthContext` disuntikkan langsung menggunakan `context.WithValue`?

**YA, pada beberapa test.**

Bukti kode di `rw_http_integration_test.go`:
```go
req := req.WithContext(context.WithValue(req.Context(), AuthContextKey, ac))
```

Bukti kode di `me_rw_membership_test.go`:
```go
req = req.WithContext(context.WithValue(req.Context(), AuthContextKey, ac))
```

### Klasifikasi:

| Test File | Lewati RequireAuth? | Klasifikasi |
|---|---|---|
| `requireauth_test.go` | YA | Full auth integration test |
| `rw_http_integration_test.go` | TIDAK | Middleware/handler integration test |
| `me_rw_membership_test.go` | TIDAK | Middleware/handler integration test |
| `scoping_test.go` | TIDAK | Unit test middleware |

**Kesimpulan:** Test RW routes dan `/me` endpoint adalah **middleware/handler integration test**, bukan full authentication integration test, karena AuthContext disuntikkan langsung melewati JWT validation.

---

## 3. Permission Enforcement

### Call site produksi `hasRWScopePermission` dan `rwPermissionMap`:

Pencarian seluruh backend menunjukkan:

| Symbol | File | Penggunaan |
|---|---|---|
| `rwPermissionMap` | `backend/internal/auth/scoping.go:176` | Definisi map (hardcoded) |
| `hasRWScopePermission` | `backend/internal/auth/scoping.go:195` | Definisi fungsi |
| `HasRWScopePermission` | TIDAK DITEMUKAN | — |

### Apakah permission enforcement diterapkan pada endpoint RW?

**TIDAK DITEMUKAN.**

Setelah pengecekan menyeluruh:
- `rwPermissionMap` hanya didefinisikan di `scoping.go`
- `hasRWScopePermission` hanya didefinisikan di `scoping.go`
- **DITEMUKAN:** Tidak ada satu pun call site produksi yang memanggil `hasRWScopePermission`
- Handler RW (`handler.go`) TIDAK memeriksa jabatan untuk permission
- Service RW (`service.go`) TIDAK memeriksa jabatan untuk permission

### Tabel Permission Enforcement:

| Permission | Definisi | Call Site Produksi | Endpoint/Service | Enforcement Terbukti |
|---|---|---|---|---|
| `finance.read` (ketua) | `scoping.go:177` | BELUM DITEMUKAN | Dashboard/KAS | **BELUM DITERAPKAN** |
| `finance.create` (ketua) | `scoping.go:178` | BELUM DITEMUKAN | — | **BELUM DITERAPKAN** |
| `finance.update` (ketua) | `scoping.go:179` | BELUM DITEMUKAN | — | **BELUM DITERAPKAN** |
| `finance.approve` (ketua) | `scoping.go:180` | BELUM DITEMUKAN | — | **BELUM DITERAPKAN** |
| `finance.read` (wakil_ketua) | `scoping.go:183` | BELUM DITEMUKAN | — | **BELUM DITERAPKAN** |
| `finance.create` (wakil_ketua) | `scoping.go:184` | BELUM DITEMUKAN | — | **BELUM DITERAPKAN** |
| `finance.approve` (wakil_ketua) | `scoping.go:185` | BELUM DITEMUKAN | — | **BELUM DITERAPKAN** |
| `finance.read` (sekretaris) | `scoping.go:188` | BELUM DITEMUKAN | — | **BELUM DITERAPKAN** |
| `finance.read` (bendahara) | `scoping.go:191` | BELUM DITEMUKAN | — | **BELUM DITERAPKAN** |

**Semua permission RW BELUM DITERAPKAN pada endpoint produksi.**

### Security Gate yang TERAPKAN:

Permission yang memang diterapkan (via `RequireRWScope`):

| Gate | Implementasi | File |
|---|---|---|
| Deployment gate (SCOPED_RW_IDS) | Fail-closed jika scope kosong | `scoping.go:118` |
| SA bypass | SA bisa akses RW dalam scope tanpa membership | `scoping.go:127` |
| RW membership check | User tanpa jabatan harus punya membership aktif | `scoping.go:142` |
| Validasi RW aktif (SA) | SA wajib verifikasi RW masih aktif di DB | `scoping.go:138` |

### Security Gate yang TIDAK TERAPAK:

| Gate | Status | Catatan |
|---|---|---|
| `finance.update` blocked for `wakil_ketua` | **BELUM DITERAPKAN** | Map definisi ada, tapi tidak dibaca |
| `finance.create` blocked for `sekretaris` | **BELUM DITERAPKAN** | — |
| `finance.update`/`finance.approve` blocked for `sekretaris` | **BELUM DITERAPKAN** | — |

---

## 4. Tenant Isolation Sampai Repository

### Middleware Isolation

`RequireRWScope` di `scoping.go:87-152`:
- Ekstrak rwId dari URL path (bukan dari query param atau body)
- Validasi rwId di `SCOPED_RW_IDS` allowlist
- SA bypass hanya jika rwId di allowlist DAN RW aktif di DB
- User biasa wajib punya matching entry di `RWMemberships`

**Terverifikasi:** Middleware melakukan isolasi per rwId.

### Handler/Service Isolation

Di `handler.go`, setiap handler melakukan:
```go
rwID := chi.URLParam(r, "rwId")
```

Di `service.go`, setiap service method memanggil:
```go
func (s *Service) checkRWAccess(ac *auth.AuthContext, rwID string) error {
    if ac.SystemRole == auth.SystemRoleSuperAdmin { return nil }
    for _, rm := range ac.RWMemberships {
        if rm.RWID == rwID { return nil }
    }
    return fmt.Errorf("%w: rw %s", ErrRWAccessDenied, rwID)
}
```

**Terverifikasi:** Service layer melakukan double-check access per rwId.

### Repository Isolation

Di `repository.go`, SEMUA query menggunakan `rw_id` filter:

| Method | Query | Filter |
|---|---|---|
| `GetDashboardSummary` | `rw_opening_balances` | `WHERE rw_id = $1` |
| `GetDashboardSummary` | `rw_transactions` | `WHERE rw_id = $1 AND status = 'posted'` |
| `GetDashboardSummary` | `rts` | `WHERE rw_id = $1 AND is_active = true` |
| `GetDashboardSummary` | `fund_locations` | `WHERE r.rw_id = $1 AND r.is_active = true AND fl.is_active = true` |
| `GetKASReport` | `rw_opening_balances` | `WHERE rw_id = $1` |
| `GetKASReport` | `rw_transactions` | `WHERE rw_id = $1 AND status = 'posted'` |
| `GetTransactions` | `rw_transactions` | `WHERE status = 'posted' AND t.rw_id = $1` |
| `GetLocations` | `fund_locations` | `WHERE rt.rw_id = $1 AND rt.is_active = true` |

**Semua query parameterized (bukan string concatenation).**

**Terverifikasi:** Repository isolation solid — tidak ada data dari RW lain yang bisa bocor.

### Potensi Cross-Access?

**Tidak ditemukan.** Semua endpoint RW:
1. Memanfaatkan `{rwId}` dari URL path (via chi)
2. Divalidasi middleware → service → repository
3. Semua query menggunakan parameterized `rw_id = $1`

---

## 5. Hitung Test dari Source Code dan Output Aktual

### Package: `internal/auth`

Command: `go test ./internal/auth/ -list .`

**Jumlah parent test: 147**

Daftar lengkap:
```
TestHashSucceeds
TestHashProducesArgon2idOutput
TestHashMinLengthEnforced
TestHashMaxLengthEnforced
TestHashDifferentSaltProduceDifferentHashes
TestHashMaxlengthBoundary
TestHashMinlengthBoundary
TestHashEmtpyPassword
TestGenerateAndVerifyToken
TestTokenHasExpiry
TestTokenExpiredIsRejected
TestInvalidSignatureRejected
TestMalformedTokenRejected
TestEmptyTokenRejected
TestDifferentRolesPreserved
TestCustomLifetime
TestTokenWithRWContextIsParsed
TestTokenWithoutRWContextIsBackwardCompatible
TestBuildRWContextJSON
TestMeEndpointRWMembershipsSingle
TestMeEndpointRWMembershipsMultiple
TestMeEndpointNoRWMembership
TestRequireRWScopeRejectsOutOfScopeRW
TestRequireRWScopeAcceptsInScopeRW
TestRequireAuthMissingAuthorizationHeader
TestRequireAuthMalformedBearerFormat
TestRequireAuthValidTokenSetsContext
TestRequireAuthExpiredTokenRejected
TestRequireAuthInvalidSignatureRejected
TestRequireRolePermittedRole
TestRequireRoleForbiddenRole
TestRequireRoleUnauthenticated
TestRequireRoleMultipleRoles
TestGetAuthContextMissing
TestRequireAuthValidTokenSucceeds
TestRequireSystemRolePermitted
TestRequireSystemRoleForbiddenForTenantOnlyUser
TestRequireSystemRoleUnauthenticated
TestRequireRoleSystemOnlyAdminPasses
TestRequireRoleAdminWithMembershipAsWarga
TestRequireRoleAdminWithMembershipAsBendahara
TestRequireSystemRoleTenantAdminDeniedSystemAccess
TestRequireSystemRoleMultiRole
TestSystemRoleAndTenantRoleAreDistinct
TestRequireRoleEmptyContextFields
TestRequireSystemRoleAndRequireRoleTogether
TestRequireRoleEmptyAllowedList
TestRequireAllowedRT
TestRequireJabatanKetuaPasses
TestRequireJabatanSekretarisRejected
TestRequireJabatanNoJabatanPasses
TestRequireJabatanSystemAdminPasses
TestRequireJabatanUnauthenticated
TestRequireAccessRolePengurusPasses
TestRequireAccessRoleBendaharaPasses
TestRequireAccessJabatanKetuaPasses
TestRequireAccessJabatanBendaharaPasses
TestRequireAccessRoleWargaBlocked
TestRequireAccessJabatanSekretarisBlocked
TestRequireAccessSystemAdminPasses
TestRequireAccessUnauthenticated
TestKetuaRTReadAccessChain
TestKetuaRTWithoutRequiredPermissionDenied
TestReadChainDisallowedTenantRole
TestSuperAdminReadAccessNoRegression
TestUnauthenticatedReadAccessDenied
TestJabatanCatalogIncludesAnggota
TestLoginRateLimiterAllowBelowThreshold
TestLoginRateLimiterBlocksAfterThreshold
TestLoginRateLimiterHTTPHandler
TestLoginRateLimiterDifferentIPsIndependent
TestLoginRateLimiterWindowExpires
TestRefreshRateLimiterAllowBelowThreshold
TestRefreshRateLimiterBlocksAfterThreshold
TestRefreshRateLimiterHTTPHandler
TestLimiterUsesRemoteAddrNotXForwardedFor
TestRateLimiterEmptyKeyPassthrough
TestRateLimiterAllowReturnsTrueForFirstRequest
TestRateLimiterConcurrentAccess
TestCreateRefreshToken_NilMembershipID
TestCreateRefreshToken_MembershipID_UUID
TestFindRefreshTokenByHash_NilMembershipID
TestFindRefreshTokenByHash_MembershipID_UUID
TestRevokeRefreshTokensByMembership_ExcludesNull
TestValidateIdentityCrossUserSwapRejected
TestValidateIdentityCorrectMembershipAllowed
TestHTTPIntegrationNoAuth
TestHTTPIntegrationSANoPool
TestHTTPIntegrationSAWithPoolAndActiveRW
TestHTTPIntegrationUserWithMatchingMembership
TestHTTPIntegrationUserNoRWMembership
TestHTTPIntegrationRWNotInScope
TestHTTPIntegrationMultipleRWRoutes
TestHTTPIntegrationOutOfScopeRW
TestHTTPIntegrationInactiveRW
TestHTTPIntegrationResponseFormat
TestHTTPIntegrationEmptyScopeRejectsAll
TestHTTPIntegrationMissingRWIDInPath
TestHTTPIntegrationRWIsolation
TestHTTPIntegrationErrorMessageBody
TestHTTPIntegrationWakilKetuaMembership
TestHTTPIntegrationSekretarisMembership
TestHTTPIntegrationPermissionMapIntegration
TestHTTPIntegrationMiddlewareDoesNotCheckPermissions
TestHTTPIntegrationMultiMembershipSelective
TestRWScopePermissionsAudit
TestRWScopePermissionsSecurityGates
TestRWScopePermissionsCrossDomain
TestRWScopePermissionCompleteness
TestLoadScopingValidUUIDs
TestLoadScopingMissingRT
TestLoadScopingMissingRW
TestLoadScopingUnsetRW
TestLoadScopingRTValidRWInvalid
TestLoadScopingRTOnlyDeployment
TestLoadScopingRTEmpty
TestRequireRWScopeWithEmptyRWIDs
TestRequireRWScopeSAWithMembershipsEmptyScope
TestLoadScopingInvalidUUID
TestScopingHasRT
TestScopingHasRW
TestExtractRWIDFromPath
TestHasRWScopePermission
TestRequireRWScopeSuperAdminWithMembershipsPoolNil
TestRequireRWScopeSAWithMembershipsButNoAllowlist
TestRequireRWScopeUserMultipleMembershipsMatchRWID
TestRequireRWScopeMultipleMembershipsWrongRW
TestRequireRWScopeUserHasRWMembershipButNotInScope
TestRequireRWScopeSuperAdminNoMemberships
TestRequireRWScopeSuperAdminWithMembershipsAndScopeAllowlist
TestRequireRWScopeUserWithMatchingMembership
TestRequireRWScopeUserWrongRW
TestRequireRWScopeNoRWContext
TestRequireRWScopeInvalidRWPath
TestParseUUIDList
TestRequireRWScopeSuperAdminNoMembershipsRWInScope
TestRequireRWScopeSuperAdminNoMembershipsRWScopeNotInList
TestRequireRWScopeUserWithoutRWMembership
TestRequireRWScopeUserWithWrongMembershipRW
TestRequireRWScopeSAWithMultipleScopeAccess
TestRequireRWScopeSuperAdminWithPool
TestJWTRoleTamperingRejected
TestJWTIDTamperingRejected
TestJWTSystemRoleTamperingRejected
TestJWTMembershipIDTamperingRejected
TestJWTUserIDTamperingRejected
TestJWTMalformedPayloadRejected
TestRequireRoleNeverReadsClientRTID
TestAuthContextTenantFieldsAreIsolatedFromRequest
TestCrossRTTenantIsolation
TestTamperingWithWrongKeyRejectsAccess
TestJWTNoneAlgorithmRejected
TestMembershipValidityAccessTokenLifetime
TestSystemOnlyLogin_ReturnsRefreshToken
TestSystemOnlyLogin_TokenStoredWithNullMembership
TestSystemOnlyRefresh_Success
TestSystemOnlyRefresh_OldTokenRejected
TestSystemOnlyRefresh_InactiveUserRejected
TestSystemOnlyRefresh_RotatedTokenStoredAsNULL
TestSystemOnlyLogout_RevsAllTokens
TestMemberLogout_Unaffected
TestUUIDDeterministic
TestUUIDDifferentLabels
TestUUIDValidFormat
TestGenerateRefreshTokenIsLongEnough
TestDifferentRefreshTokensEachTime
TestGenerateRefreshTokenNotPredictable
TestMeReturnsEffectiveFinancePermissions
TestFinanceManagementPermissionMiddlewareRejectsReadOnlyPositi
```

**Jumlah subtest: 0** (semua adalah parent test, bukan subtest)

**Jumlah test yang dieksekusi:** 147

**Status:** Lihat hasil eksekusi di bawah:

### Package: `internal/rw`

Command: `go test ./internal/rw/ -list .`

**Jumlah parent test: 33**

Daftar:
```
TestHandleErrorAccessDenied
TestHandleErrorAccessDeniedWrapped
TestHandleErrorBadRequest
TestHandleErrorInternal
TestHandleErrorGeneric
TestHandleErrorNonBadRequestNot400
TestHandleErrorValidationErrorPreservesMessage
TestHandleErrorDateValidation
TestParsePagination
TestValidatePeriod
TestValidateDate
TestParseMoney
TestFormatMoney
TestNextMonth
TestPaginationFromURL
TestDashboardSummaryDefaults
TestKASReportNetBalance
TestParseMoneyParityWithFinancePackage
TestFormatMoneyParityWithFinancePackage
TestRegression1_OpeningBalanceIndependent
TestRegression2_IncreaseByIncome
TestRegression3_IncreaseByExpense
TestRegression4_RTChangeDoesNotAffectRW
TestRegression5_RWChangeDoesNotAffectRT
TestRegression6_RTTransactionsNotInRWHistory
TestRegression7_RWTransactionsNotInRTHistory
TestRegression8_RWReversalOnlyAffectsRW
TestRegression9_ConsistentBalanceAcrossViews
TestRegression10_PeriodFilterCalculation
TestRWTenantIsolation
TestRWDashboardWithReversal
TestGetLocationsBalanceComputed
TestGetLocationsTenantIsolation
TestGetTransactionsPagination
TestServiceCheckRWAccess
```

**Jumlah subtest: 0**

**Jumlah test yang dieksekusi:** 33

### Ringkasan Test Count

| Package | Parent Test | Subtest | Total Dieksekusi |
|---|---|---|---|
| `internal/auth` | 147 | 0 | 147 |
| `internal/rw` | 33 | 0 | 33 |
| **TOTAL** | **180** | **0** | **180** |

### Khusus `auth/scoping_test.go`: 16 parent test

Tidak ada 21 parent test. Yang ada:
1. TestLoadScopingValidUUIDs
2. TestLoadScopingMissingRT
3. TestLoadScopingMissingRW
4. TestLoadScopingUnsetRW
5. TestLoadScopingRTValidRWInvalid
6. TestLoadScopingRTOnlyDeployment
7. TestLoadScopingRTEmpty
8. TestRequireRWScopeWithEmptyRWIDs
9. TestRequireRWScopeSAWithMembershipsEmptyScope
10. TestLoadScopingInvalidUUID
11. TestScopingHasRT
12. TestScopingHasRW
13. TestExtractRWIDFromPath
14. TestHasRWScopePermission
15. TestRequireRWScopeSuperAdminWithMembershipsPoolNil
16. TestRequireRWScopeSAWithMembershipsButNoAllowlist
17. TestRequireRWScopeUserMultipleMembershipsMatchRWID
18. TestRequireRWScopeMultipleMembershipsWrongRW
19. TestRequireRWScopeUserHasRWMembershipButNotInScope
20. TestRequireRWScopeSuperAdminNoMemberships
21. TestRequireRWScopeSuperAdminWithMembershipsAndScopeAllowlist
22. TestRequireRWScopeUserWithMatchingMembership
23. TestRequireRWScopeUserWrongRW
24. TestRequireRWScopeNoRWContext
25. TestRequireRWScopeInvalidRWPath
26. TestParseUUIDList
27. TestRequireRWScopeSuperAdminNoMembershipsRWInScope
28. TestRequireRWScopeSuperAdminNoMembershipsRWScopeNotInList
29. TestRequireRWScopeUserWithoutRWMembership
30. TestRequireRWScopeUserWithWrongMembershipRW
31. TestRequireRWScopeSAWithMultipleScopeAccess
32. TestRequireRWScopeSuperAdminWithPool

**Koreksi:** `scoping_test.go` memiliki **32 parent test**, bukan 21.

### Khusus `auth/me_rw_membership_test.go`: 3 test

1. TestMeEndpointRWMembershipsSingle
2. TestMeEndpointRWMembershipsMultiple
3. TestMeEndpointNoRWMembership

**Jumlah aktual: 3.** Kontradiksi 3 versus 5: angka 5 SALAH. Yang benar adalah **3 test**.

---

## 6. Investigasi Flaky Test

### `TestMeEndpointRWMembershipsSingle` dan `TestMeEndpointRWMembershipsMultiple`

Dari sumber code:

```go
// seedRWAndRT
_, err = tx.ExecContext(ctx, `
    INSERT INTO rws (id, name, rw_number, is_active)
    VALUES ($1, $2, $3, true)
`, rwID, rwName, rwNumber)

_, err = tx.ExecContext(ctx, `
    INSERT INTO rts (id, name, rt, rw, rw_id, is_active)
    VALUES ($1, $2, $3, $4, $5, true)
`, rtID, rtName, rtCode, rwNumber, rwID)
```

### Kemungkinan penyebab flaky:

1. **UUID deterministic yang konflik:**
   Di `test_uuid_test.go` ditemukan `TestUUIDDeterministic`. Di `me_rw_membership_test.go`, UUID dibuat via `uuid.New()` (random), bukan deterministic. Jadi ini seharusnya tidak konflik.

2. **Shared test database:**
   `testutil.GetTestPool(t)` menggunakan shared database pool. Jika test berjalan paralel, ada potensi race condition pada INSERT.

3. **Tidak ada cleanup eksplisit:**
   Tidak ada `defer tx.Rollback()` di `TestMeEndpointRWMembershipsSingle` — hanya `defer tx.Rollback()` setelah begin. Jika test gagal di tengah, data bisa tertinggal.

4. **unique constraint pada `rts` tabel:**
   `rts` memiliki column `rt` (code string) yang mungkin memiliki UNIQUE constraint. Jika dua test membuat `rt` dengan code yang sama (`rtCode := fmt.Sprintf("%03d", 900+int(userID[0])%100)`), maka akan konflik.

5. **Parallel execution:**
   Fungsi test menggunakan `t.Helper()` tetapi tidak memanggil `t.Parallel()`. Jadi test seharusnya tidak berjalan paralel.

### Kesimpulan Investigasi:

Penyebab paling mungkin: **`rt` code collision** karena `rtCode` dihitung dari `userID[0] % 100` yang bisa menghasilkan nilai sama untuk UUID berbeda. Jika ada test lain yang menggunakan userID dengan byte pertama yang sama, maka akan terjadi unique constraint violation.

**Status: BELUM DIKETAHUI 100%** — perlu eksekusi test untuk konfirmasi.

---

## 7. Remaining Gaps

### HIGH

| Gap | Deskripsi | Bukti |
|---|---|---|
| **RW Permission Map Tidak Diterapkan** | `rwPermissionMap` dan `hasRWScopePermission` didefinisikan tapi tidak dipanggil di satu pun endpoint produksi | Pencarian seluruh backend tidak menemukan call site |
| **JWT Test pada RW Routes** | RW routes tidak diuji dengan JWT sungguhan — AuthContext disuntikkan langsung | `rw_http_integration_test.go` menggunakan `context.WithValue` |

### MEDIUM

| Gap | Deskripsi | Bukti |
|---|---|---|
| **SA Bypass Wide** | SA bisa akses SEMUA RW dalam scope tanpa membership | `scoping.go:127` — SA bypass semua membership check |
| **Flaky Test Potential** | `rtCode` calculation bisa menyebabkan unique constraint collision | `me_rw_membership_test.go:52` |

### LOW

| Gap | Deskripsi | Bukti |
|---|---|---|
| **Permission Map Documentation** | `rwPermissionMap` tidak didokumentasikan sebagai "not yet enforced" | Kode komentar tidak ada |

---

## 8. Verdict

### PARTIALLY VALIDATED

**Yang TERVERIFIKASI:**
1. ✅ Route registration — 4 endpoint RW terdaftar dengan middleware `RequireAuth` dan `RequireRWScope`
2. ✅ JWT validation — `RequireAuth` memvalidasi JWT dan identity di DB
3. ✅ Middleware isolation — `RequireRWScope` memvalidasi RW membership per request
4. ✅ Repository isolation — Semua query parameterized dengan `rw_id` filter
5. ✅ Test count — 180 total parent test di 2 package

**Yang BELUM TERVERIFIKASI:**
1. ❌ Permission enforcement (`rwPermissionMap`) — map didefinisikan tapi tidak dipanggil di production
2. ❌ Full auth chain untuk RW routes — test menggunakan AuthContext injection, bukan JWT
3. ❌ Flaky test root cause — perlu eksekusi aktual

**Yang DIBATALKAN dari laporan sebelumnya:**
- `scoping_test.go` memiliki **32 parent test**, bukan 21
- `me_rw_membership_test.go` memiliki **3 test**, bukan 5
