# PR 4 — Final Security, Data Integrity & Regression Audit Report

## 1. Kesimpulan: **PARTIALLY VERIFIED — 1 BLOCKER Ditemukan**

Tiga dari empat bidang audit (Authorization, KAS/Transactions, Locations) **PASS**. Namun ditemukan **1 BLOCKER** pada query `GetDashboardSummary()` yang menyebabkan **data double-counting** karena LEFT JOIN chain yang menggandakan baris, serta menggunakan kolom `lbc.net` yang **tidak ada** di skema database.

---

## 2. Inventaris Perubahan

### Modified (9 files):
| File | Lines | Deskripsi |
|------|-------|-----------|
| `backend/cmd/api/main.go` | +28/-1 | 4 route group RW + `RequireRWScope` wiring |
| `backend/cmd/api/router_auth_test.go` | +6/-1 | Added `ScopingConfig` ke test router |
| `backend/internal/auth/jwt.go` | +15/-12 | Added `RWContext` field ke `TokenClaims` |
| `backend/internal/auth/jwt_test.go` | +113 | 3 test cases baru: parsing RW context |
| `backend/internal/auth/middleware.go` | +82/-5 | `RequireAuth` mempopulasi `RWMemberships` dari DB per-request |
| `backend/internal/auth/models.go` | +20/-7 | Added `RWMembership` struct + `RWMemberships` field |
| `backend/internal/auth/service.go` | +110/-46 | `loadRWMemberships()` + `buildRWContextJSON()` ditambahkan |
| `backend/internal/fund_location/handler.go` | +25 | `checkRWBendaharaAccess()` ditambahkan |
| `docs/database.md` | +30 | Section 5.1 RW Scope architecture |

### New (untracked):
| File | Tipe | Deskripsi |
|------|------|-----------|
| `backend/internal/auth/scoping.go` | Created | `RequireRWScope` middleware + `rwPermissionMap` |
| `backend/internal/auth/scoping_test.go` | Created | 20 test cases scoping middleware |
| `backend/internal/fund_location/handler_security_test.go` | Created | Security test RW fund locations |
| `backend/internal/rw/model.go` | Created | Structs + validation logic (~95 lines) |
| `backend/internal/rw/repository.go` | Created | Queries: dashboard, kas, transactions, locations (~180 lines) |
| `backend/internal/rw/service.go` | Created | Service layer + `checkRWAccess()` (~180 lines) |
| `backend/internal/rw/handler.go` | Created | HTTP handlers + error mapping (~200 lines) |
| `backend/internal/rw/rw_handler_test.go` | Created | 24 test cases parsing/validation |
| `docs/rbac-operasional-rw-openapi.yaml` | Created | OpenAPI spec 4 RW endpoints |

**Tidak ada endpoint POST RW. Tidak ada Migration 042. Tidak ada `rw_transactions`. Tidak ada `rw_id` di `fund_locations`.** ✅

---

## 3. Tabel Verifikasi 4 Endpoint

| Endpoint | Middleware Stack | `checkRWAccess` | Repository Scope | Status |
|----------|-----------------|-----------------|------------------|--------|
| `GET /{rwId}/dashboard` | `RequireAuth` → `RequireRWScope` → handler | ✅ | ❌ **BLOCKER** (query salah) | BLOCKED |
| `GET /{rwId}/kas` | `RequireAuth` → `RequireRWScope` → handler | ✅ | ✅ | PASS |
| `GET /{rwId}/kas/transactions` | `RequireAuth` → `RequireRWScope` → handler | ✅ | ✅ | PASS |
| `GET /{rwId}/locations` | `RequireAuth` → `RequireRWScope` → handler | ✅ | ✅ | PASS |

### 3.1 Authorization Chain (Semua Endpoint) ✅

**Verifikasi middleware stack terpasang dan dieksekusi:**
- `main.go:756-757`: `r.Use(auth.RequireAuth)` kemudian `r.Use(auth.RequireRWScope(pool, *scopingCfg))`
- `RequireAuth` dieksekusi duluan, mem-populasi `AuthContext.RWMemberships`
- `RequireRWScope` membaca itu, validasi scope

**`rwId` berasal dari path yang benar dan divalidasi:**
- `extractRWIDFromPath()` di `scoping.go` mengambil dari URL path, bukan body/query
- `chi.URLParam(r, "rwId")` di handler mengambil dari route param
- Middleware dan service layer melakukan validasi independen

**Membership RW harus aktif:**
- `RequireAuth` menjalankan `SELECT rw_id, jabatan FROM user_rw_memberships WHERE user_id = $0 AND rw_id IN (...) AND is_active = true` per-request
- `RequireRWScope` memeriksa `ac.RWMemberships` yang hanya berisi membership aktif
- Revoke-safe karena query DB per-request

**Pengguna RW A tidak bisa meminta data RW B:**
- `RequireRWScope`: for loop memeriksa `rm.RWID == rwID` → 403 Forbidden
- `checkRWAccess` di service layer: pemeriksaan defensif kedua
- Repository: `rwID` adalah parameter SQL dari middleware, bukan input user

**Konfigurasi scoping yang hilang → fail-closed:**
- `LoadScoping()`: jika `SCOPED_RT_IDS` atau `SCOPED_RW_IDS` kosong → error saat startup
- Test `TestLoadScopingMissingRT` dan `TestLoadScopingMissingRW` membuktikan ini

**Service tidak memiliki jalur bypass otorisasi:**
- Setiap method publik memanggil `checkRWAccess(ac, rwID)` sebagai baris pertama sebelum query

---

## 4. Temuan Saldo — BLOCKER Ditemukan

### 🔴 BLOCKER 1: Query Dashboard Menggunakan Kolom Tidak Ada

**File:** `backend/internal/rw/repository.go:52`
**Baris kode:** `COALESCE(SUM(lbc.net), 0)::text`

**Masalah:** Kolom `net` **tidak ada** di tabel `location_balance_changes`. Kolom yang ada hanya:
```
id (UUID), rt_id (UUID), location_id (UUID), transaction_id (UUID),
change_type (TEXT), amount_delta (NUMERIC), notes (TEXT),
created_at (TIMESTAMPTZ), created_by (UUID)
```

**Dampak:** `lbc.net` selalu `NULL`. `SUM(lbc.net)` selalu `0`. **Total balance selalu 0 terlepas dari saldo aktual.**

### 🔴 BLOCKER 2: Dashboard Double-Counting Via LEFT JOIN Chain

**File:** `backend/internal/rw/repository.go:41-54`
**Baris kode:**
```sql
SELECT
    COUNT(DISTINCT fl.id),
    COALESCE(SUM(lbc.net), 0)::text,
    COALESCE(SUM(CASE WHEN t.type = 'income' AND t.status = 'posted' THEN t.amount ELSE 0 END), 0)::text,
    COALESCE(SUM(CASE WHEN t.type = 'expense' AND t.status = 'posted' THEN t.amount ELSE 0 END), 0)::text,
    COUNT(DISTINCT CASE WHEN t.status = 'posted' THEN t.id END)
FROM rts r
LEFT JOIN fund_locations fl ON fl.rt_id = r.id
LEFT JOIN location_balance_changes lbc ON lbc.location_id = fl.id
LEFT JOIN transactions t ON t.location_id = fl.id
WHERE r.rw_id = $1 AND r.is_active = true
```

**Masalah:** Query ini melakukan `LEFT JOIN` tiga tabel secara bersamaan, menghasilkan **Cartesian product** antara `location_balance_changes` dan `transactions` per lokasi:

- Misal 1 RT punya 1 lokasi dengan 3 baris LBC (opening, income, expense) dan 2 baris transaksi (income posting, expense posting)
- Hasil JOIN menghasilkan 3 × 2 = **6 baris** (bukan 5 baris terpisah)
- `SUM(t.amount)` dihitung 3x untuk setiap baris transaksi (dikali jumlah LBC)
- Saldo dan agregasi income/expense **terinflasi**

### 🔴 BLOCKER 3: Sumber Kebenaran Balance di RW vs RT Inconsistent

**Fakta skema yang diverifikasi:**
1. Setiap posting transaksi membuat 1 baris di `transactions` DAN 1 baris di `location_balance_changes`
2. `transactions` menggunakan `amount > 0`, jenis ditentukan oleh kolom `type` ('income' atau 'expense')
3. Reversal transaksi menggunakan `reverses_transaction_id` FK dan memiliki `amount_delta` berlawanan tanda
4. Balance RW = `SUM(lbc.amount_delta)` untuk semua transaksi yang terkait dengan RW

**Saat ini balance dihitung via `SUM(lbc.net)` = 0.** Seharusnya menggunakan `transactions.amount` yang sudah ada di RT-level, atau `SUM(lbc.amount_delta)` (hanya transaksi posted, bukan reversal).

---

## 5. Hasil Tes

| Perintah | Hasil |
|----------|-------|
| `go build ./...` | ✅ OK (0 errors) |
| `go test ./internal/rw/...` | ✅ 24/24 PASS |
| `go test ./internal/auth/...` | ✅ PASS (20+ test cases scoping + JWT) |
| `go test ./internal/fund_location/...` | ✅ PASS |
| `git diff --check` | ✅ OK (no whitespace errors) |
| OpenAPI spec validation | ✅ OK |

### Gap tes yang teridentifikasi:
- ❌ Tidak ada integration test yang membuktikan membership tidak aktif ditolak
- ❌ Tidak ada integration test yang membuktikan RW A tidak dapat membaca RW B
- ❌ Tidak ada integration test yang membuktikan query agregasi tidak memasukkan RT dari RW lain
- ❌ Tidak ada integration test yang membuktikan opening balance dan reversal diperlakukan benar
- ✅ Unit tests hanya mengcover parsing/validation logic (period, date, pagination, money parsing)
- ❌ Tidak ada test yang membuktikan endpoint GET tidak mengubah database

---

## 6. OpenAPI dan Regresi

| Item | Status |
|------|--------|
| `rbac-operasional-rw-openapi.yaml` valid | ✅ |
| 4 endpoint definitions present | ✅ dashboard, kas, kas/transactions, locations |
| 4 schema definitions present | ✅ RWDashboardSummary, LocationSummary, KASReport, TransactionItem |
| GET-only (no POST) | ✅ |
| Modified files do not alter RT-level behavior | ✅ |
| `fund_location/handler.go` tambahan `checkRWBendaharaAccess()` | ✅ (tidak mempengaruhi read-only endpoints) |

---

## 7. Temuan Keamanan Lainnya

| Temuan | File:Baris | Risiko | Detail |
|--------|-----------|--------|--------|
| SA middleware pool nil → 500 | `scoping.go:140-148` | MEDIUM | SA dengan pool nil mendapat 500 Internal Error, bukan 403. Intentional fail-closed tapi error code kurang semantik. |
| `RWMemberships` di JWT populated per-request | `middleware.go:85-150` | INFO | Setiap request melakukan DB query untuk memvalidasi RW memberships. Revoke-safe. Minor perf overhead. |

---

## 8. Kesimpulan Final

| Kategori | Status |
|----------|--------|
| Authorization chain (middleware + service + repo) | ✅ PASS |
| Endpoint `/kas` (KAS report) | ✅ PASS |
| Endpoint `/kas/transactions` (transactions list) | ✅ PASS |
| Endpoint `/locations` (fund locations) | ✅ PASS |
| Dashboard summary data accuracy | 🔴 **BLOCKED** — 2 blocker: (1) kolom `lbc.net` tidak ada, (2) double-counting dari JOIN chain |
| Integration test coverage | ⚠️ **GAP** — tidak ada test end-to-end |

**Rekomendasi:** Fix kedua blocker di `repository.go` (query `GetDashboardSummary()`) sebelum commit. Gunakan pendekatan subquery terpisah atau JOIN hanya pada satu tabel untuk menghindari double-counting. Tambahkan integration test di PR berikutnya.
