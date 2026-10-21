# Desain Teknis — Otorisasi, FIFO, Tutup Buku, & Siklus Hidup Iuran

**Branch:** `urena` | **HEAD:** `2def69e` | **Base:** `origin/urena` | **Upstream main:** `65a5acd` (notification fix ada di main, belum merge ke urena)
**Status working tree:** untracked file `followup_audit_report.md` saja
**Scope:** READ-ONLY — desain dan test plan. Tidak mengubah kode, database, atau konfigurasi.

---

## 1. Baseline dan Keterbatasan Audit

### 1.1 Verifikasi Baseline

| Item | Nilai | Sumber |
|------|-------|--------|
| Branch | `urena` | `git branch` |
| HEAD commit | `2def69e` ("Merge pull request #8 from andjoy404/main") | `git log` |
| Parent | `65a5acd` ("fix(notification): notify RT chair on payment submission") | `git log` |
| Working tree | Clean (untracked `followup_audit_report.md`) | `git status` |
| Remote origin | `https://github.com/andjoy404/social-finance` | `git remote -v` |
| Upstream main | `65a5acd` | `git log` |
| Notifikasi fix | Sudah di main (`65a5acd`) | `git log` |
| Tracking | `origin/urena` | `git branch -vv` |

### 1.2 Keterbatasan

- **Tidak ada operasi remote.** Tidak fetch/pull/merge — tidak bisa memverifikasi apakah ada commit di main yang belum ada di `urena`.
- **Tidak ada interaksi database.** Audit berbasis kode statis dan dokumentasi.
- **Asumsi bisnis.** Beberapa keputusan desain berdasarkan requirement yang diberikan pengguna. Jika ada nuance bisnis yang hilang, akan ditandai sebagai "Perlu persetujuan produk".

---

## 2. Matriks Role dan Otorisasi

### 2.1 Definisi Role dan Jabatan

**Role (JWT `role` claim):**

| Role | User ID | Scope |
|------|---------|-------|
| `super_admin` | Platform admin | Semua RT (system-wide) |
| `pengurus` | RT executive | Satu RT (tenant-scoped) |
| `bendahara` | RT treasurer | Satu RT (tenant-scoped) |
| `warga` | RT resident | Satu RT + own household only |

**Jabatan (JWT `jabatan` claim, dari `user_rt_memberships.jabatan`):**

`ketua`, `sekretaris`, `bendahara`, `wakil_ketua`, keamanan, dan jabatan lain yang bisa punya `position_permissions`.

### 2.2 Matriks Permission (`position_permissions`)

**File:** `backend/internal/auth/permission.go`

Permission yang ada di sistem (ditelusuri dari routing `main.go` dan test `bills_rbac_test.go`):

| Permission | Ketua | Bendahara | Sekretaris | Wakil Ketua | Keamanan | Pengurus (role) | Bendahara (role) | Warga (role) |
|-------------|-------|-----------|------------|-------------|----------|-----------------|------------------|-------------|
| `finance.read` | Via table | Via table | Via table | Via table | No | Via RequireRole | Via RequireRole | Via RequireRole |
| `finance.create` | Via table | Via table | Via table | Via table | No | Via RequireRole | Via RequireRole | No |
| `finance.update` | Via table | Via table | Via table | Via table | No | Via RequireRole | Via RequireRole | No |
| `finance.approve` | Via table | Via table | Via table | Via table | No | Via RequireRole | Via RequireRole | No |
| `warga.read` | Via table | Via table | Via table | Via table | Via table | Via RequireRole | Via RequireRole | Via RequireRole |
| `warga.create` | Via table | Via table | Via table | Via table | Via table | Via RequireRole | Via RequireRole | Via RequireRole |
| `warga.update` | Via table | Via table | Via table | Via table | Via table | Via RequireRole | Via RequireRole | Via RequireRole |
| `warga.import` | Via table | Via table | Via table | Via table | Via table | Via RequireRole | Via RequireRole | Via RequireRole |
| `warga.export` | Via table | Via table | Via table | Via table | Via table | Via RequireRole | Via RequireRole | Via RequireRole |

Sumber: `bills_rbac_test.go` (line 489—517: keamanan lacks `finance.read`; line 546—588: wakil_ketua has `finance.read` but NOT `finance.approve`).

### 2.3 Routing dan Middleware Aktual

**File:** `backend/cmd/api/main.go`

#### Group 1 — Financial Read + Self-Submitted Payment (lines 642-665)

```
Middleware stack:
  1. RequireAuth (auth middleware, JWT validation)
  2. RequireAllowedRT (tenant validation)
  3. RequirePermission("finance.read") (jabatan-based via position_permissions)
  4. RequireRole(warga, bendahara, pengurus) (role gate)

Routes:
  GET  /api/v1/categories              → ListCategories
  GET  /api/v1/dues                     → ListDues
  GET  /api/v1/bills                    → ListBills
  GET  /api/v1/bills/{id}              → GetBill
  GET  /api/v1/payments                → ListPayments
  GET  /api/v1/payments/{id}           → GetPayment
  GET  /api/v1/payments/{id}/proof     → GetPaymentProof
  GET  /api/v1/transactions            → ListTransactions
  GET  /api/v1/transactions/{id}       → GetTransaction
  GET  /api/v1/reports/balance         → GetBalance
  GET  /api/v1/dashboard/summary       → GetDashboardSummary

  POST /api/v1/payments                → CreatePayment  ⚠️ NO finance.create permission check!
  POST /api/v1/payments/multi-period   → CreateMultiPeriodPayment ⚠️ NO finance.create!
  POST /api/v1/payments/{id}/proof     → UploadPaymentProof
```

**Gap #1:** `CreatePayment` dan `CreateMultiPeriodPayment` ada di group ini yang hanya punya `finance.read` — bukan `finance.create`. Warga bisa submit payment karena ini intended, tapi `finance.create` permission tidak dicek di sini.

#### Group 2 — Master Category Management (lines 671-682)

```
Middleware stack:
  1. RequireAuth
  2. RequireAllowedRT
  3. RequireAccess([pengurus], [ketua])

Routes:
  POST   /api/v1/categories          → CreateCategory
  PATCH  /api/v1/categories/{id}     → UpdateCategory
  DELETE /api/v1/categories/{id}     → DeactivateCategory
```

#### Group 3 — Financial Operations (lines 685-707)

```
Middleware stack:
  1. RequireAuth
  2. RequireAllowedRT
  3. RequirePermission("finance.create") (jabatan-based)
  4. RequirePermission("finance.update") (jabatan-based)
  5. RequirePermission("finance.approve") (jabatan-based)
  6. RequireAccess([bendahara, pengurus], [ketua, bendahara])

Routes:
  POST   /api/v1/dues                  → CreateDue
  PATCH  /api/v1/dues/{id}             → UpdateDue
  DELETE /api/v1/dues/{id}             → DeactivateDue
  POST   /api/v1/bills                 → CreateBill
  POST   /api/v1/bills/generate        → GenerateBills
  DELETE /api/v1/bills/{id}            → CancelBill
  POST   /api/v1/payments/{id}/verify  → VerifyPayment
  POST   /api/v1/transactions          → CreateTransaction
  PUT    /api/v1/transactions/{id}     → UpdateTransaction
  PATCH  /api/v1/transactions/{id}     → UpdateTransaction
  POST   /api/v1/transactions/{id}/reverse → ReverseTransaction
  PUT    /api/v1/reports/balance/opening/{period} → UpdateOpeningBalance
```

**Catatan kritis:** Middleware di Group 3 (`RequirePermission` + `RequireAccess`) adalah pengecekan yang **seharusnya** berlaku untuk semua route. Namun `VerifyPayment`, `CreateTransaction`, dan lainnya juga memiliki pemeriksaan di level handler mereka sendiri melalui `getAuthContext()` dan `requireTenantContext()`.

### 2.4 Matriks Otorisasi Aktual per Operasi

| Operasi | Endpoint | Middleware Check | Handler Check | Result |
|---------|----------|-----------------|---------------|--------|
| List iuran (read) | `GET /dues` | `finance.read` + `RequireRole(bendahara/pengurus/warga)` | JWT + rt_id | ✅ OK |
| Buat iuran | `POST /dues` | `finance.create` + `finance.update` + `finance.approve` + `RequireAccess(bendahara/pengurus/ketua)` | JWT + rt_id | ✅ OK |
| Update iuran | `PATCH /dues/:id` | Sama seperti Buat iuran | JWT + rt_id | ✅ OK |
| **Submit pembayaran (warga)** | `POST /payments` | `finance.read` (bukan create!) + `RequireRole(warga/bendahara/pengurus)` | JWT + rt_id | ⚠️ Partial — warga bisa submit, tidak bisa verifikasi |
| **Buat pembayaran (staff)** | `POST /payments` (STAFF_RECORDED) | Sama seperti submit | JWT + rt_id | ⚠️ Warga biasa bisa set `origin = STAFF_RECORDED` karena validasi di handler |
| **Verifikasi pembayaran** | `POST /payments/:id/verify` | `finance.create` + `finance.update` + `finance.approve` + `RequireAccess(bendahara/pengurus/ketua)` | JWT + rt_id | ✅ Middleware OK, **TAPI handler tidak melakukan role check lagi** — middleware sudah menangani |
| Buat transaksi | `POST /transactions` | `finance.create` + `finance.update` + `finance.approve` + `RequireAccess(bendahara/pengurus/ketua)` | JWT + rt_id | ✅ OK |
| Reverse transaksi | `POST /transactions/:id/reverse` | Sama seperti Buat transaksi | JWT + rt_id | ✅ OK |
| Update opening balance | `PUT /reports/balance/opening/:period` | Sama seperti Buat transaksi | JWT + rt_id | ✅ OK |

### 2.5 Analisis Mendalam — Apakah VerifyPayment Aman?

Sebelumnya follow-up audit menyatakan "Warga bisa memverifikasi pembayaran" karena handler-level check tidak ada. Namun setelah inspect routing:

**`VerifyPayment` ADA di Group 3** (main.go line 685-707) yang punya middleware stack lengkap:
1. `RequirePermission("finance.create")`
2. `RequirePermission("finance.update")`
3. `RequirePermission("finance.approve")`
4. `RequireAccess([bendahara, pengurus], [ketua, bendahara])`

Artinya:
- Jabatan-based user: dicek via `position_permissions` — jabatan harus punya `finance.approve`
- Role-based user: dicek via `RequireRole(bendahara, pengurus)` — warga DIBLOKIR
- `ketua` bisa verifikasi karena ada di `RequireAccess` + `position_permissions`

**Koreksi dari follow-up audit:** `VerifyPayment` **AMAN** melalui middleware stack Group 3. Tidak ada authorization gap di sini.

### 2.6 Authorization Gap yang Masih Ada

#### Gap 1: CreatePayment — Warga bisa set `origin = STAFF_RECORDED`

**File:** `backend/internal/finance/handler.go:1062-1200`, routing line 642-665

```go
// Di CreatePayment service.go line 328-332:
if payment.Origin == PaymentOriginStaffRecorded {
    payment.Status = PaymentStatusApproved // langsung approved!
}
```

Masalahnya: `CreatePayment` ada di Group 1 yang hanya punya `finance.read` + `RequireRole(warga/bendahara/pengurus)`. Tidak ada middleware yang mengecek apakah user boleh membuat payment dengan `origin = STAFF_RECORDED`.

Warga biasa yang ter-autentikasi bisa:
1. Login sebagai warga
2. POST `/api/v1/payments` dengan `origin: "STAFF_RECORDED"` (jika frontend mengirim)
3. Payment langsung APPROVED tanpa perlu bendahara

**Mitigasi saat ini:** Web frontend warga selalu mengirim `origin = SELF_SUBMITTED`. Android juga demikian. Tapi jika warga membuat custom client atau mengubah request via curl, tidak ada blocking di backend.

**Rekomendasi:** Validasi `origin` di handler berdasarkan role user:

```go
func (h *Handler) CreatePayment(...) {
    ac := getAuthContext(r)

    // Infer intended origin from request
    var intendedOrigin PaymentOrigin
    if isStaffRecorded {
        intendedOrigin = PaymentOriginStaffRecorded
    } else {
        intendedOrigin = PaymentOriginSelfSubmitted
    }

    // Warga boleh hanya SELF_SUBMITTED
    if ac.TenantRole == auth.RoleWarga && intendedOrigin == PaymentOriginStaffRecorded {
        httpx.WriteError(w, http.StatusForbidden, "forbidden", "only staff can record payments")
        return
    }
}
```

#### Gap 2: No self-approval prevention

Tidak ada aturan bahwa pembuat pembayaran tidak boleh memverifikasi bayarannya sendiri. Jika warga dan bendahara adalah user yang sama (bukan praktik yang baik tapi mungkin), warga bisa:
1. Submit payment (SELF_SUBMITTED → PENDING)
2. Verify payment (karena bendahara juga warga) → APPROVED

**Rekomendasi:** Di `VerifyPayment` service, tambahkan check:
```go
if payment.SubmittedBy == verifierID {
    return nil, errors.New("cannot verify your own payment")
}
```

#### Gap 3: CreateTransaction — Warga bisa buat transaksi keuangan

**File:** `backend/internal/finance/handler.go:1836-1883`, routing Group 3

`CreateTransaction` ada di Group 3 yang sudah punya middleware `finance.create` + `RequireAccess(bendahara/pengurus/ketua)`. **AMAN melalui middleware.**

### 2.7 Rekomendasi Perubahan Otorisasi

Tidak ada perubahan routing yang diperlukan karena Group 3 sudah benar. Yang perlu diubah:

1. **Handler-level guard di `CreatePayment`:** Cegah warga set `origin = STAFF_RECORDED`
2. **Service-level guard di `VerifyPayment`:** Cegah self-approval
3. **Tambah test:** Regression test untuk Gap 1 dan Gap 2

---

## 3. Rekomendasi Desain FIFO

### 3.1 Kondisi Aktual

| Aspek | Fakta |
|-------|-------|
| `CreatePayment` (service.go:318) | Target specific `bill_id` — no FIFO check |
| `payment_rule` (`full`/`partial`) | Hanya validasi jumlah, bukan urutan |
| `verifyMultiPeriodPayment` (service.go:847) | Sort UUID untuk deadlock prevention |
| Frontend sort | Display only — Android `iuran_providers.dart:201` unpaid → oldest first |
| `payment_allocations` | No ordering column, no FIFO logic |
| Constraint DB | `UNIQUE(bill_id, due_id, period)` on bills — satu bill per occupancy per due per period |

### 3.2 Pilihan Desain yang Dipertimbangkan

#### Opsi A: Backend FIFO Enforcement (Recommended)

**Prinsip:** Backend menolak pembayaran yang melampaui tagihan yang lebih tua.

```go
func (s *Service) CreatePayment(ctx, billID, rtID, userID string) (*Payment, error) {
    // 1. Resolve bill → get occupancy_id + due_id
    bill, err := BillGetByID(ctx, tx, billID, rtID)

    // 2. Get all unpaid/partial bills for THIS occupancy + due_id, ordered oldest first
    unpaidBills, err := BillsGetOldestFirst(ctx, tx, bill.HouseholdOccupancyID, bill.DueID, rtID)

    // 3. Find position of target bill in ordered list
    targetIndex := -1
    for i, b := range unpaidBills {
        if b.ID == billID {
            targetIndex = i
            break
        }
    }

    // 4. FIFO check: only allow payment if this is the oldest unpaid bill
    if targetIndex != 0 {
        oldestBill := unpaidBills[0]
        return nil, fmt.Errorf(
            "please pay the oldest bill first: %s (period %s, due %s)",
            oldestBill.Period, oldestBill.DueName,
        )
    }

    // 5. Proceed with existing payment logic...
}
```

**Database query helper:**
```sql
SELECT b.id, b.period, b.status, d.name as due_name
FROM bills b
JOIN dues d ON b.due_id = d.id
WHERE b.rt_id = $1
  AND b.household_occupancy_id = $2
  AND b.due_id = $3
  AND b.status IN ('unpaid', 'partial')
ORDER BY
    d.period_type ASC,  -- one_time < monthly < yearly
    d.name ASC,         -- alphabetical within same period_type
    b.period ASC,       -- oldest first within same due
    b.created_at ASC    -- deterministic tiebreaker
LIMIT 100
```

**Kelebihan:**
- Enforce FIFO secara server-side — tidak bisa di-bypass frontend
- Error message jelas: warga tahu bill mana yang harus dibayar dulu
- Scalable: bekerja untuk single dan multi-period payment

**Kekurangan:**
- Breaking change untuk frontend yang mengirim bill_id bukan tertua
- Perlu UI update: tampilkan pesan FIFO di Web/Android
- Warga dengan tunggakan harus bayar dari yang tertua (by design)

#### Opsi B: Backend FIFO Validation + Auto-Allocated Multi-Period

**Prinsip:** Warga cukup kirim `amount` — backend yang alokasikan ke bill tertua.

```go
// New API endpoint (alternative to CreatePayment):
// POST /api/v1/payments/allocate
// Body: { "due_id": "...", "amount": "150000", "method": "CASH" }
// Backend:
// 1. Get all unpaid bills for user's occupancy + due_id
// 2. Allocate amount to oldest first, then next, etc.
// 3. Create payment with payment_allocations
// 4. Auto-update bill statuses
```

**Kelebihan:**
- User experience lebih baik — warga tidak perlu pilih bill
- FIFO enforced secara otomatis
- Cocok untuk "bayar lunas tunggakan 3 bulan sekaligus"

**Kekurangan:**
- Breaking change besar — API baru
- Frontend perlu redesign payment flow
- Kompleksitas lebih tinggi (edge cases: partial allocation across many bills)

#### Opsi C: Frontend-Only FIFO (Status Quo dengan Enhance)

**Prinsip:** Android/Web hanya tampilkan bill tertua, disable yang lain.

**Kelebihan:**
- Non-breaking change
- Cepat implementasi

**Kekurangan:**
- Bisa di-bypass via curl/custom client
- Tidak memenuhi requirement "backend enforces FIFO"

### 3.3 Rekomendasi Final

**Pilih Opsi A** sebagai implementasi pertama, dengan Opsi B sebagai roadmap phase 2.

**Alasan:**
1. Non-breaking untuk user flow saat ini (warga tetap pilih bill_id)
2. Error message edukatif, tidak menghalangi
3. Multi-period payment via `CreateMultiPeriodPayment` bisa di-enforce FIFO sama
4. Opsi B bisa diimplementasi kemudian sebagai `POST /payments/allocate` endpoint

### 3.4 Detail Implementasi Opsi A

#### 3.4.1 Perubahan Model & Repository

**File:** `backend/internal/finance/model.go` — tambah method signature:

```go
// BillsGetOldestUnpaidByOccupancy returns unpaid/partial bills for an occupancy
// and due_id, ordered from oldest to newest.
func BillsGetOldestUnpaidByOccupancy(ctx context.Context, tx *sql.Tx, occupancyID, dueID, rtID string) ([]*Bill, error)
```

**File:** `backend/internal/finance/repository.go` — tambah implementation:

```go
func BillsGetOldestUnpaidByOccupancy(ctx context.Context, tx *sql.Tx, occupancyID, dueID, rtID string) ([]*Bill, error) {
    rows, err := tx.QueryContext(ctx, `
        SELECT b.id, b.period, b.status, b.amount, b.due_date,
               d.name as due_name, d.period_type, d.payment_rule, d.is_voluntary
        FROM bills b
        JOIN dues d ON b.due_id = d.id
        WHERE b.rt_id = $1
          AND b.household_occupancy_id = $2
          AND b.due_id = $3
          AND b.status IN ('unpaid', 'partial')
        ORDER BY
            CASE d.period_type
                WHEN 'one_time' THEN 1
                WHEN 'monthly' THEN 2
                WHEN 'yearly' THEN 3
            END ASC,
            d.name ASC,
            b.period ASC,
            b.created_at ASC
    `, rtID, occupancyID, dueID)
    if err != nil {
        return nil, err
    }
    // scan rows...
}
```

#### 3.4.2 Perubahan Service

**File:** `backend/internal/finance/service.go` — di `CreatePayment` (~line 336):

```go
// FIFO check: ensure this is the oldest unpaid bill for this occupancy + due
unpaidBills, err := BillsGetOldestUnpaidByOccupancy(ctx, tx, bill.HouseholdOccupancyID, bill.DueID, rtID)
if err != nil {
    return nil, fmt.Errorf("check FIFO: %w", err)
}
if len(unpaidBills) > 0 && unpaidBills[0].ID != billID {
    oldest := unpaidBills[0]
    return nil, fmt.Errorf(
        "harap lunasi tagihan terlebih dahulu: %s (%s, %s)",
        oldest.DueName, oldest.Period, formatDate(oldest.DueDate),
    )
}
```

**File:** `backend/internal/finance/service.go` — di `CreateMultiPeriodPayment` (~line 1220):

```go
// Validate each bill is payable in order (no gaps in unpaid sequence)
for i, billID := range input.BillIDs {
    unpaidBills, err := BillsGetOldestUnpaidByOccupancy(ctx, tx, occupancyID, dueID, rtID)
    if err != nil {
        return nil, fmt.Errorf("check FIFO for bill %s: %w", billID, err)
    }

    // First bill in request must be the oldest unpaid
    if i == 0 && len(unpaidBills) > 0 && unpaidBills[0].ID != billID {
        return nil, fmt.Errorf("harap mulai dari tagihan tertua...")
    }

    // Subsequent bills must follow sequentially (no gaps)
    // ...
}
```

#### 3.4.3 Perubahan API Contract

**Tidak ada perubahan path/method.** Response error code:

```json
// HTTP 400 Bad Request
{
    "error": {
        "code": "fifo_violation",
        "message": "harap lunasi tagihan terlebih dahulu: Iuran Reguler (2026-07)",
        "details": [
            {
                "field": "bill_id",
                "oldest_unpaid_bill_id": "uuid-of-oldest",
                "oldest_unpaid_period": "2026-07"
            }
        ]
    }
}
```

#### 3.4.4 Dampak Frontend

| Platform | Perubahan |
|----------|-----------|
| Web | `IuranPayment.tsx`: tangkap error 400, tampilkan pesan FIFO. `IuranList.tsx`: highlight bill tertua |
| Android | `iuran_screen.dart`: tangkap error FIFO, show snackbar. `iuran_providers.dart`: prioritas render sudah benar |

### 3.5 Concurrency & Idempotency

**Race condition:** Dua request bersamaan dari warga yang sama ke bill berbeda.

**Solusi:** `HandleIdempotentRequest` (service.go) sudah ada — menggunakan `idempotency_keys` table. Kunci berdasarkan `(rt_id, user_id, endpoint)`.

Tapi untuk FIFO, perlu **row-level lock** pada bill yang sedang dibayar:

```go
// Di CreatePayment, setelah SELECT bill:
err := tx.QueryRowContext(ctx, `
    SELECT status FROM bills WHERE id = $1 AND rt_id = $2 FOR UPDATE
`, billID, rtID).Scan(&status)
```

Ini memastikan dua concurrent request tidak melewati FIFO check pada state yang sama.

### 3.6 Migrasi yang Diperlukan

**Tidak ada migration.** FIFO enforcement murni logic di service layer. Query menggunakan kolom yang sudah ada (`period`, `status`, `due_id`, `household_occupancy_id`).

---

## 4. Rekomendasi Desain Tutup Buku

### 4.1 Kondisi Aktual

| Komponen | Status |
|----------|--------|
| `period_opening_balances` table | ADA (migration 024) |
| Kolom `locked BOOLEAN` | ADA, default FALSE |
| `CalculatePeriodBalance` | ADA — hitung dari ledger + manual opening |
| `UpdateOpeningBalance` handler | ADA — tanpa cek `locked` |
| Period-closing API | TIDAK ADA |
| Post-period transaction prevention | TIDAK ADA |

### 4.2 Desain Rekomendasi

#### 4.2.1 Tambah Kolom `periods.status`

Alih-alih hanya mengandalkan `period_opening_balances.locked` (yang flat dan mudah dilewati), tambahkan explicit period management table:

**Migration (conceptual):**
```sql
CREATE TABLE IF NOT EXISTS period_status (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id UUID NOT NULL REFERENCES rts(id),
    period VARCHAR(7) NOT NULL,  -- YYYY-MM
    status VARCHAR(20) NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'closed', 'locked')),
    closed_by TEXT,              -- user_id of closer
    closed_at TIMESTAMPTZ,
    reason TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_period_status_rt_period UNIQUE (rt_id, period)
);
```

**Status lifecycle:**
- `open` → default, transaksi normal diperbolehkan
- `closed` → set oleh bendahara/pengurus, no new transactions for this period
- `locked` → set oleh pengurus/super_admin, even opening balance changes blocked

#### 4.2.2 API Endpoint

```
POST /api/v1/reports/periods/:period/close
Body: { "reason": "Monthly closing — September 2026" }
Middleware: RequireAccess([pengurus], [ketua])  // pengurus + ketua saja yang bisa tutup

PUT /api/v1/reports/periods/:period/unlock
Body: { "reason": "Audit correction" }
Middleware: RequireAccess([pengurus], [])  // pengurus saja, bukan ketua
```

#### 4.2.3 Middleware Guard untuk Semua Write Operations

Di setiap handler yang menulis transaksi (CreatePayment, CreateTransaction, ReverseTransaction, UpdateOpeningBalance):

```go
func (h *Handler) CreatePayment(w http.ResponseWriter, r *http.Request) {
    // ... existing auth checks

    // NEW: Check if payment affects a closed period
    if err := CheckPeriodAccess(ctx, tx, ac.RTID, paymentDate, "write"); err != nil {
        httpx.WriteError(w, http.StatusForbidden, "period_closed", err.Error())
        return
    }
    // ...
}
```

**Implementation:**
```sql
SELECT status FROM period_status
WHERE rt_id = $1 AND period = $2  -- period = DATE_TRUNC('month', date)::TEXT
```

Jika `status IN ('closed', 'locked')` → tolak.

#### 4.2.4 Update Opening Balance Guard

**File:** `backend/internal/finance/handler.go:2056`

```go
func (h *Handler) UpdateOpeningBalance(...) {
    // ... existing validation

    // NEW: Check if period is locked
    var status string
    err := tx.QueryRowContext(ctx,
        `SELECT status FROM period_status WHERE rt_id = $1 AND period = $2`,
        ac.RTID, period,
    ).Scan(&status)

    if err == nil && status == "locked" {
        httpx.WriteError(w, http.StatusForbidden, "period_locked",
            "cannot change opening balance of a locked period")
        return
    }
    // if period_status row doesn't exist → open → allow
}
```

#### 4.2.5 Ledger Transaction Date Validation

**File:** `backend/internal/finance/service.go` — `CreateManualTransaction` (~line 1000):

```go
// Validate occurred_at is not in a closed period
occurredPeriod := FormatMonth(occurredAt)
if err := CheckPeriodAccess(ctx, tx, rtID, occurredPeriod, "write"); err != nil {
    return nil, fmt.Errorf("cannot post transaction in closed period: %w", err)
}
```

#### 4.2.6 Period Balance Calculation Update

**File:** `backend/internal/finance/repository.go` — `CalculatePeriodBalance` (~line 2052):

Tambahan: return `is_closed` flag di response:

```go
type PeriodBalanceSummary struct {
    OpeningBalance   string `json:"opening_balance"`
    TotalIncome      string `json:"total_income"`
    TotalExpense     string `json:"total_expense"`
    NetMovement      string `json:"net_movement"`
    ClosingBalance   string `json:"closing_balance"`
    TransactionCount int    `json:"transaction_count"`
    IsClosed         bool   `json:"is_closed"`        // NEW
    ClosedAt         string `json:"closed_at,omitempty"` // NEW
}
```

### 4.3 Trade-off Tutup Buku

| Aspek | Dengan period_status table | Tanpa (hanya locked column) |
|-------|---------------------------|---------------------------|
| Clarity | Explicit lifecycle, clear status | Implicit, easy to miss |
| Audit trail | closed_by, closed_at, reason | Existing reason column |
| Granularity | 3 statuses (open/closed/locked) | Boolean (locked/unlocked) |
| Query cost | 1 extra JOIN per write | None |
| Complexity | More tables to manage | Simpler |

**Rekomendasi:** Gunakan `period_status` table. Klarens lebih penting daripada complexity tambahan. Satu JOIN per write operation adalah overhead yang bisa diterima untuk financial integrity.

---

## 5. Rekomendasi Desain Siklus Hidup Iuran

### 5.1 Requirement Bisnis

| # | Rule | Detail |
|---|------|--------|
| 1 | Iuran Reguler aktif default | `is_active = true` saat dibuat |
| 2 | Iuran Reguler bisa nonaktif manual | Bendahara set `is_active = false` |
| 3 | Tunggakan tetap bisa dibayar | Bill status unaffected by due `is_active` |
| 4 | Iuran Sukarela otomatis nonaktif | 1 hari setelah `due_date` |
| 5 | Semua iuran otomatis nonaktif 31 Des | Year-end blackout |
| 6 | Awal tahun: semua nonaktif sampai diaktifkan | Staff harus re-enable |
| 7 | Generate bills harus hormati status iuran | Tidak buat bill untuk iuran nonaktif (kecuali tunggakan) |

### 5.2 Pendekatan: Runtime Checks + Periodic Job

**Rekomendasi:** Kombinasi keduanya. Runtime check untuk aturan sederhana (voluntary expiration), periodic job untuk year-end reset.

#### 5.2.1 Runtime Check — Voluntary Bill Expiration

**File:** `backend/internal/finance/service.go` — `GenerateBills` (~line 734):

```go
func (s *Service) GenerateBills(ctx, tx, rtID, userID string, in GenerateBillsInput) ([]*Bill, error) {
    // 1. Get due by ID
    due, err := DueGetByID(ctx, tx, in.DueID, rtID)

    // 2. Rule: auto-expire voluntary dues after due_date
    if due.IsVoluntary {
        // Parse the period to check if it's past due_date
        periodDate, err := ParsePeriodToMonth(in.Period)
        dueDate := ParseDate(in.DueDate) // e.g., "2026-09-10"

        // If period month is after due_date month of same year, skip
        if periodDate.After(dueDate) {
            // Mark as expired (don't set is_active = false, just skip generation)
            return nil, fmt.Errorf(
                "iuran sukarela tidak bisa dibuat untuk periode %s (jatuh tempo: %s)",
                in.Period, in.DueDate,
            )
        }
    }

    // 3. Rule: skip inactive regular dues (but allow generating bills
    //    for past periods = tunggakan)
    if !due.IsActive && !due.IsVoluntary {
        // Allow if period is in the past (tunggakan)
        periodDate, _ := ParsePeriodToMonth(in.Period)
        now := time.Now()
        if periodDate.After(now) {
            return nil, fmt.Errorf("iuran tidak aktif, tidak bisa buat tagihan periode mendatang")
        }
        // Allow tunggakan — fall through to bill generation
    }

    // ... existing bill generation logic
}
```

#### 5.2.2 Periodic Job — Year-End Reset

**File:** New file `backend/internal/finance/year_end.go` (conceptual):

```go
// YearEndReset processes all RTs and sets is_active = false for all active dues
// on January 1st. Called via cron or manual trigger.
func YearEndReset(ctx context.Context, pool *database.Pool) error {
    now := time.Now()
    if now.Month() != time.January {
        return nil // Only run in January
    }

    // Get all RTs
    rows, err := pool.Raw().QueryContext(ctx,
        `SELECT id, rt_code FROM rts WHERE is_active = true`)
    // ... for each RT, set dues.is_active = false
}
```

**Atau lebih sederhana:** Manual trigger endpoint:

```
POST /api/v1/dues/year-end-reset
Body: { "year": 2027 }
Middleware: RequireAccess([pengurus], [])
```

**Implementation:**
```go
func (s *Service) YearEndReset(ctx, tx, rtID, userID string, year string) error {
    // Set all regular dues to is_active = false
    _, err := tx.ExecContext(ctx, `
        UPDATE dues
        SET is_active = false, updated_at = CURRENT_TIMESTAMP
        WHERE rt_id = $1
          AND is_active = true
          AND is_voluntary = false
    `, rtID)
    return err
}
```

#### 5.2.3 Generate Bills Respect Due Status

**File:** `backend/internal/finance/service.go` — `GenerateBills`:

```go
// Filter active occupancies for bill generation
rows := tx.QueryContext(ctx, `
    SELECT ho.id
    FROM household_occupancies ho
    JOIN households h ON ho.household_id = h.id
    WHERE h.rt_id = $1 AND ho.is_active = true
`, rtID)

// Generate bill only if:
// 1. Due is_active = true → generate for current/future periods
// 2. Due is_active = false AND is_voluntary = false → generate only for past periods (tunggakan)
// 3. Due is_active = false AND is_voluntary = true → skip (expired)
```

**Database constraint option (alternative):**
```sql
-- Optional: Add CHECK to prevent generating bills for expired voluntary dues
ALTER TABLE bills ADD CONSTRAINT chk_bill_due_active
    CHECK (
        EXISTS (
            SELECT 1 FROM dues d
            WHERE d.id = bills.due_id
              AND d.is_active = true
        )
        OR (SELECT is_voluntary FROM dues WHERE id = bills.due_id) = false  -- tunggakan
        OR (SELECT period FROM dues WHERE id = bills.due_id) <= bills.period  -- past period
    )
```

**Rekomendasi:** Jangan pakai DB constraint. Too rigid. Gunakan application-level validation instead.

### 5.3 Trade-off Pendekatan

| Aspek | Runtime Check | Cron Job | Manual Trigger |
|-------|--------------|----------|----------------|
| Reliability | Always runs when bills generated | Depends on scheduler running | Depends on human action |
| Idempotency |天然 idempotent (generate per period) | Needs dedup logic | Needs confirmation |
| Complexity | Zero extra infra | Need scheduler (cron/celery) | Simplest |
| Year-end reset | Checked at generation time | Bulk update all dues at once | Manual endpoint per RT |
| Voluntary expiration | Immediate (at generation time) | N/A | N/A |

**Rekomendasi:**
1. **Runtime check** untuk voluntary expiration + inactive due prevention (selalu reliable)
2. **Manual trigger** untuk year-end reset (bisa di-trigger Januari atau tidak sama sekali — bisnis decision)
3. **Tidak perlu cron job** — menghindari infrastructure complexity

---

## 6. Test Plan

### 6.1 Test Matrix Overview

| Area | Unit | Integration | E2E/API | UI/Web | UI/Android |
|------|------|-------------|---------|--------|-----------|
| Authorization | ● | ● | ● | ○ | ○ |
| FIFO | ● | ● | ● | ○ | ○ |
| Period Closing | ● | ● | ● | ○ | ○ |
| Iuran Lifecycle | ● | ● | ● | ○ | ○ |
| Concurrency | ● | ● | ○ | — | — |
| Idempotency | ○ | ● | ● | — | — |
| Regression Verify | ● | ● | ● | ○ | ○ |

**Legend:** ● Wajib | ○ Opsional | — Tidak relevan

### 6.2 Authorization Tests

| # | Scenario | Prasyarat | Langkah | Hasil Diharapkan | Layer |
|---|----------|-----------|---------|-----------------|-------|
| AUTH-01 | Warga submit payment → PENDING | Warga punya bill unpaid | POST /payments with SELF_SUBMITTED | Payment created, status PENDING | Integration |
| AUTH-02 | Warga set origin = STAFF_RECORDED (bypass) | Warga ter-autentikasi | POST /payments with origin=STAFF_RECORDED via curl | **HTTP 403 Forbidden** | Integration |
| AUTH-03 | Warga verifikasi payment sendiri | Warga = submitter + bendahara (user sama) | POST /payments/:id/verify with action=approve | **HTTP 403 or 400** (self-approval blocked) | Integration |
| AUTH-04 | Warga biasa verifikasi payment | Warga ter-autentikasi | POST /payments/:id/verify | **HTTP 403** (RequireRole blocks) | Integration |
| AUTH-05 | Bendahara verifikasi payment | Bendahara punya PENDING payment | POST /payments/:id/verify with action=approve | Payment APPROVED, bill status updated | Integration |
| AUTH-06 | Ketua verifikasi payment | Ketua punya jabatan, jabatan punya finance.approve | POST /payments/:id/verify | **Approved** (position_permissions allows) | Integration |
| AUTH-07 | Keamanan verifikasi payment | Keamanan punya jabatan, TIDAK punya finance.approve | POST /payments/:id/verify | **HTTP 403** (position_permissions blocks) | Integration |
| AUTH-08 | Cross-RT verify | User RT A, coba verify payment RT B | POST /payments/:id/verify (ID dari RT B) | **HTTP 404** (tenant isolation) | Integration |
| AUTH-09 | Super admin tidak bisa finance ops | Super admin ter-autentikasi, RTID nil | GET /api/v1/dues | **HTTP 403** (no tenant context) | Integration |
| AUTH-10 | CreateTransaction by bendahara | Bendahara punya category | POST /transactions | Transaction created with auto-category | Integration |

### 6.3 FIFO Tests

| # | Scenario | Prasyarat | Langkah | Hasil Diharapkan | Layer |
|---|----------|-----------|---------|-----------------|-------|
| FIFO-01 | Bayar bill tertua | Warga punya 1 unpaid bill | POST /payments (bill tertua) | **Payment created** | Integration |
| FIFO-02 | Lewati bill tertua | Warga punya bill Jul(unpaid) + Aug(unpaid) | POST /payments (bill Aug) | **HTTP 400** "harap lunasi tagihan terlebih dahulu: Jul" | Integration |
| FIFO-03 | Bayar tunggakan berurutan | Warga punya Jul+Aug+Sep(unpaid) | Bayar Jul → Bayar Aug → Bayar Sep | **Semua berhasil** | Integration |
| FIFO-04 | Multi-period FIFO valid | Warga punya Jul+Aug(unpaid) | POST /payments/multi-period [Jul, Aug] | **Payment created with 2 allocations** | Integration |
| FIFO-05 | Multi-period FIFO invalid (skip) | Warga punya Jul+Aug+Sep(unpaid) | POST /payments/multi-period [Aug, Sep] | **HTTP 400** (tidak mulai dari tertua) | Integration |
| FIFO-06 | Partial payment + FIFO | Warga punya Jul(unpaid, 100k) + Aug(unpaid, 100k) | Bayar Jul 50k → Jul masih partial → Bayar Aug | **HTTP 400** (Jul masih unpaid/partial) | Integration |
| FIFO-07 | Due berbeda tidak terpengaruh | Warga punya 2 due (Reguler + Sukarela), masing-masing punya bill Jul+Aug | Bayar Reguler Aug tanpa bayar Reguler Jul → Bayar Sukarela Aug | **Reguler Jul=400, Sukarela Aug=OK** (FIFO per due_id) | Integration |
| FIFO-08 | Bill sudah paid dilewati | Warga punya Jul(paid) + Aug(unpaid) | POST /payments (bill Aug) | **Payment created** (FIFO check hanya unpaid/partial) | Integration |
| FIFO-09 | Frontend bypass attempt | Warga punya 2 unpaid bills | curl POST bill yang bukan tertua | **HTTP 400** | API |

### 6.4 Period Closing Tests

| # | Scenario | Prasyarat | Langkah | Hasil Diharapkan | Layer |
|---|----------|-----------|---------|-----------------|-------|
| CLOSE-01 | CreatePayment di period open | Period September = open | POST /payments (occurred_at Sep) | **Approved** | Integration |
| CLOSE-02 | CreatePayment di period closed | Period September = closed | POST /payments (occurred_at Sep) | **HTTP 403** "period_closed" | Integration |
| CLOSE-03 | UpdateOpeningBalance di period locked | Period September = locked | PUT /reports/balance/opening/2026-09 | **HTTP 403** "period_locked" | Integration |
| CLOSE-04 | UpdateOpeningBalance di period closed (not locked) | Period September = closed | PUT /reports/balance/opening/2026-09 | **Allowed** (closing ≠ locked) | Integration |
| CLOSE-05 | CreateTransaction di period closed | Period September = closed | POST /transactions (occurred_at Sep) | **HTTP 403** | Integration |
| CLOSE-06 | Close period via API | Bendahara ter-autentikasi | POST /reports/periods/2026-09/close | Period marked closed | Integration |
| CLOSE-07 | Warga close period | Warga ter-autentikasi | POST /reports/periods/2026-09/close | **HTTP 403** (bukan pengurus) | Integration |
| CLOSE-08 | CalculatePeriodBalance return is_closed | Period September closed | GET /reports/balance?period=2026-09 | Response includes `is_closed: true` | Integration |
| CLOSE-09 | Pembayaran di periode masa depan tidak terpengaruh | September closed, Oktober open | POST /payments (occurred_at Oct) | **Approved** | Integration |

### 6.5 Iuran Lifecycle Tests

| # | Scenario | Prasyarat | Langkah | Hasil Diharapkan | Layer |
|---|----------|-----------|---------|-----------------|-------|
| LIFE-01 | Generate bill untuk iuran aktif | Iuran Reguler is_active=true | POST /dues/:id/generate | Bills created | Integration |
| LIFE-02 | Generate bill untuk iuran nonaktif (masa depan) | Iuran Reguler is_active=false | POST /dues/:id/generate period=2027-01 | **HTTP 400** "tidak bisa buat tagihan periode mendatang" | Integration |
| LIFE-03 | Generate bill tunggakan (iuran nonaktif, masa lalu) | Iuran Reguler is_active=false | POST /dues/:id/generate period=2026-06 | **Bills created** (tunggakan) | Integration |
| LIFE-04 | Generate bill untuk iuran sukarela (past due_date) | Iuran Sukarela due_date=2026-09-10 | POST /dues/:id/generate period=2026-10 | **HTTP 400** "iuran sukarela tidak bisa dibuat untuk periode yang sudah lewat jatuh tempo" | Integration |
| LIFE-05 | Generate bill untuk iuran sukarela (sebelum due_date) | Iuran Sukarela due_date=2026-09-10 | POST /dues/:id/generate period=2026-08 | **Bills created** | Integration |
| LIFE-06 | Bayar tunggakan iuran nonaktif | Warga punya bill dari iuran yang sudah di-nonaktifkan | POST /payments (bill tunggakan) | **Payment created** (tunggakan tetap bisa dibayar) | Integration |
| LIFE-07 | Year-end reset | Banyak iuran aktif | POST /dues/year-end-reset (Januari) | Semua iuran reguler → is_active=false | Integration |
| LIFE-08 | Generate bill setelah year-end reset | Semua iuran is_active=false | POST /dues/:id/generate | **HTTP 400** (karena iuran nonaktif) | Integration |
| LIFE-09 | Aktifkan ulang iuran setelah year-end | Bendahara update due | PATCH /dues/:id {is_active: true} | Iuran aktif kembali | Integration |

### 6.6 Concurrency & Idempotency Tests

| # | Scenario | Prasyarat | Langkah | Hasil Diharapkan | Layer |
|---|----------|-----------|---------|-----------------|-------|
| CONC-01 | Concurrent verify payment | 1 PENDING payment | 2 concurrent POST /payments/:id/verify | **1 approved, 1 409/400** (FOR UPDATE lock) | Integration |
| CONC-02 | Concurrent FIFO check | Warga punya 1 unpaid bill | 2 concurrent POST /payments (bill yang sama) | **1 succeeds, 1 FIFO violation** (bill sudah paid) | Integration |
| CONC-03 | Concurrent payment allocation | Warga punya 2 unpaid bills | 2 concurrent POST /payments/multi-period [bill1, bill2] | **Race handled by FOR UPDATE lock** | Integration |
| IDEM-01 | Duplicate idempotency key | Warga submit payment | 2x POST /payments dengan Idempotency-Key sama | **Response kedua = original response** (tidak double charge) | Integration |
| IDEM-02 | Duplicate multi-period key | Warga submit multi-period | 2x POST /payments/multi-period dengan key sama | **Payment created once, allocations created once** | Integration |

### 6.7 Regression Tests (Verify Payment Already Working)

| # | Scenario | Prasyarat | Langkah | Hasil Diharapkan | Layer |
|---|----------|-----------|---------|-----------------|-------|
| REG-01 | Warga submit, bendahara approve | Full lifecycle | Submit → Verify → Bill status updated → Ledger created | Payment APPROVED, bill PAID/Partial, ledger income | Integration |
| REG-02 | Bendahara reject payment | PENDING payment | POST /payments/:id/verify with action=reject, reason="..." | Payment REJECTED, bill unchanged | Integration |
| REG-03 | Multi-period approval | Multi-period payment dengan 3 allocations | Verify approval | All 3 bills updated, ONE ledger transaction | Integration |
| REG-04 | Overpayment detection | Payment amount > bill remaining | Bendahara try to approve | **HTTP 400** "payment amount exceeds remaining balance", payment REJECTED | Integration |
| REG-05 | Payment proof upload | Payment PENDING | POST /payments/:id/proof with image | Proof stored, payment updated | Integration |
| REG-06 | Notification on payment submitted | FCM configured | Warga submit payment | Push notification to bendahara + ketua | Integration |
| REG-07 | Notification on payment verified | Payment APPROVED | Bendahara approve | Push notification to submitter | Integration |

---

## 7. Urutan Implementasi dan Dependensi

### Phase 1: Authorization Guards (Critical)

**File yang diubah:**
- `backend/internal/finance/handler.go` — CreatePayment handler
- `backend/internal/finance/service.go` — CreatePayment, VerifyPayment service
- `backend/internal/finance/handler_test.go` —新增 tests
- `backend/internal/finance/service_test.go` —新增 tests

**Perubahan:**
1. Handler-level guard: warga tidak bisa set `origin = STAFF_RECORDED`
2. Service-level guard: self-approval prevention di VerifyPayment

**Test wajib:** AUTH-02, AUTH-03, AUTH-04, AUTH-05, AUTH-06, AUTH-07

**Risiko regresi:** Low — hanya menambahkan deny conditions. Existing workflow tidak berubah.

**Kriteria penerimaan:**
- Warga tidak bisa create STAFF_RECORDED payment (tested via curl)
- Self-approval blocked (tested: same user submit + verify)
- Bendahara/pengurus/ketua tetap bisa verifikasi (existing flow)
- 6/6 AUTH tests pass

**Prasyarat:** Tidak ada. Independent.

### Phase 2: FIFO Enforcement (High)

**File yang diubah:**
- `backend/internal/finance/repository.go` —新增 `BillsGetOldestUnpaidByOccupancy`
- `backend/internal/finance/service.go` — CreatePayment, CreateMultiPeriodPayment
- `backend/internal/finance/handler_test.go` —新增 FIFO tests
- `frontend/src/features/iuran/IuranPayment.tsx` — Handle FIFO error message
- `mobile/lib/features/iuran/` — Handle FIFO error snackbar

**Perubahan:**
1. Repository method: query unpaid bills ordered oldest first per due_id
2. Service check: FIFO validation di CreatePayment dan CreateMultiPeriodPayment
3. API error response: structured error dengan oldest bill info
4. Frontend: display FIFO error to user

**Test wajib:** FIFO-01 through FIFO-09

**Risiko regresi:** Medium — breaking change untuk payment flow. Warga harus bayar bill tertua dulu.

**Kriteria penerimaan:**
- FIFO enforced per due_id (not across dues)
- Error message informatif
- Multi-period payment validasi urutan
- 9/9 FIFO tests pass
- Frontend menampilkan error FIFO dengan benar

**Prasyarat:** Phase 1 selesai (authorization stabil sebelum tambah logic).

### Phase 3: Period Closing (Medium)

**File yang diubah:**
- `backend/migrations/XXX_period_status.up.sql` —新增 period_status table
- `backend/internal/finance/repository.go` —新增 `CheckPeriodAccess`
- `backend/internal/finance/handler.go` — UpdateOpeningBalance, CreatePayment, CreateTransaction
- `backend/internal/finance/service.go` — CreateManualTransaction date check
- `backend/internal/finance/model.go` —新增 PeriodStatus model, Update PeriodBalanceSummary
- `backend/internal/finance/handler_test.go` —新增 closing tests

**Perubahan:**
1. Migration: create period_status table
2. Repository: CheckPeriodAccess function
3. Handler guards: semua write operations check period status
4. New API endpoints: close/unlock period
5. Response model: add is_closed flag

**Test wajib:** CLOSE-01 through CLOSE-09

**Risiko regresi:** Medium — write operations blocked on closed periods. Harus komunikasi ke pengguna sebelum implementasi.

**Kriteria penerimaan:**
- Write operations blocked on closed periods
- UpdateOpeningBalance blocked on locked periods
- Period closing requires pengurus role
- Balance response includes is_closed flag
- 9/9 CLOSE tests pass

**Prasyarat:** Phase 1 & 2 selesai. Koordinasi dengan pengguna tentang kapan period closing aktif.

### Phase 4: Iuran Lifecycle (Medium)

**File yang diubah:**
- `backend/internal/finance/service.go` — GenerateBills (runtime checks)
- `backend/internal/finance/service.go` — UpdateDue (voluntary expiration check)
- `backend/internal/finance/handler.go` —新增 YearEndReset handler
- `backend/internal/finance/handler_test.go` —新增 lifecycle tests
- `backend/cmd/api/main.go` —新增 year-end-reset route

**Perubahan:**
1. GenerateBills: runtime check voluntary expiration + inactive due prevention
2. Year-end reset: manual trigger endpoint
3. Bill generation respects due status

**Test wajib:** LIFE-01 through LIFE-09

**Risiko regresi:** Medium — bisa mencegah bill generation untuk periode tertentu. Perlu regression test on existing bills.

**Kriteria penerimaan:**
- Voluntary dues expire after due_date
- Inactive regular dues block future bill generation
- Tunggakan tetap bisa di-generate
- Year-end reset works via manual trigger
- 9/9 LIFE tests pass

**Prasyarat:** Phase 1 selesai. Phase 3 (period closing) recommended tapi tidak required.

### Phase 5: Concurrency Hardening & Idempotency (Low)

**File yang diubah:**
- `backend/internal/finance/service.go` — FOR UPDATE lock di CreatePayment
- `backend/internal/finance/handler_test.go` —新增 concurrency tests

**Perubahan:**
1. FOR UPDATE lock pada bill row saat CreatePayment
2. Concurrency test suite

**Test wajib:** CONC-01 through CONC-03, IDEM-01 through IDEM-02

**Risiko regresi:** Very low — hanya menambah locking yang sudah ada di VerifyPayment.

**Kriteria penerimaan:**
- Concurrent verify → only one succeeds
- Concurrent payment → FIFO enforced correctly
- Duplicate idempotency key → no double posting
- 4/4 concurrency tests pass

**Prasyarat:** Semua phase sebelumnya.

---

## 8. Keputusan yang Memerlukan Persetujuan Pengguna

### 8.1 FIFO: Per-Due_id vs Global

**Pertanyaan:** FIFO enforcement berlaku per `due_id` (jenis iuran) atau global (semua jenis)?

**Rekomendasi:** Per `due_id`. Warga bisa bayar tunggakan Reguler Juli sementara Sukarela Agustus tetap unpaid.

**Jika global:** Warga harus bayar semua jenis iuran tertua sebelum yang baru. Lebih strict, mungkin terlalu membingungkan.

### 8.2 Period Closing: Status 2 atau 3?

**Pertanyaan:** `closed` (bisa dibuka kembali) vs `locked` (hanya super_admin)?

**Opsi A — 2 status:** `open` → `closed` (bisa reverse). Simple.
**Opsi B — 3 status:** `open` → `closed` → `locked`. More control.

**Rekomendasi:** Opsi B (3 status) untuk financial audit trail.

### 8.3 Year-End Reset: Auto vs Manual

**Pertanyaan:** Reset semua iuran ke `is_active = false` otomatis (cron) atau manual trigger?

**Rekomendasi:** Manual trigger. Hindari dependency pada scheduler yang belum ada.

**Jika otomatis:** Perlu cron service, error handling, retry logic, timezone handling.

### 8.4 Self-Approval: Block Semua atau Case-based?

**Pertanyaan:** Bolehkah bendahara memverifikasi pembayaran yang dia buat sendiri?

**Opsi A — Block semua:** Tidak bisa verifikasi payment dengan submitted_by = verifier_id.
**Opsi B — Case-based:** Bendahara yang merekam (STAFF_RECORDED) boleh verifikasi. Warga yang submit (SELF_SUBMITTED) tidak boleh verifikasi sendiri.

**Rekomendasi:** Opsi B. Bendahara yang merekam pembayaran tunai langsung seharusnya bisa approve.

### 8.5 Multi-Period FIFO: Strict atau Lenient?

**Pertanyaan:** Saat multi-period payment, apakah harus连续 dari tertua (Jul, Aug, Sep) atau boleh skip (Jul, Sep)?

**Rekomendasi:** Strict — harus连续 tanpa gap. Jika ada gap, gap itu tunggakan yang harus dibayar terpisah.

### 8.6 Frontend: Auto-allocate vs Manual Selection

**Pertanyaan:** Setelah FIFO enforced, apakah frontend tetap tampilkan semua unpaid bills untuk dipilih, atau otomatis select tertua?

**Rekomendasi:**
- Phase 1: Tampilkan semua unpaid, highlight tertua, disable yang bukan tertua
- Phase 2 (future): Auto-allocate — user hanya input amount

---

## 9. Daftar Risiko dan Kriteria Penerimaan

### 9.1 Risiko Implementasi

| # | Risiko | Severity | Mitigasi |
|---|--------|----------|----------|
| R1 | FIFO breaking change — warga bingung kenapa bayar bill tertentu ditolak | High | Error message edukatif, UI hint, onboarding |
| R2 | Period closing — transaksi yang sudah ada di closed period | Medium | Validation hanya untuk transaksi BARU, tidak retroaktif |
| R3 | Year-end reset accidental — semua iuran nonaktif di Januari | Medium | Manual trigger, confirmation dialog, bisa re-enable instantly |
| R4 | Self-approval block — bendahara perlu approve payment warga (user sama) | Low | Opsi B: block hanya SELF_SUBMITTED by same user |
| R5 | Concurrent FIFO race — FOR UPDATE lock meningkatkan latency | Low | Lock scope minimal (satu bill row), timeout 5s |
| R6 | Multi-period payment complexity — alokasi ke banyak bill bisa error | Medium | Graceful error: rollback semua bila satu alokasi gagal |
| R7 | Frontend regression — error handling baru tidak ditangani UI | Medium | Test frontend integration, error toast/snackbar |

### 9.2 Kriteria Penerimaan per Phase

**Phase 1 — Authorization:**
- [ ] AUTH-02: Warga tidak bisa set STAFF_RECORDED (tested via curl)
- [ ] AUTH-03: Self-approval blocked
- [ ] AUTH-04—07: Role-based access verified
- [ ] Existing payment flow (AUTH-01, AUTH-05) tetap berfungsi
- [ ] `go test ./internal/finance/...` — semua test pass

**Phase 2 — FIFO:**
- [ ] FIFO-01: Bayar bill tertua berhasil
- [ ] FIFO-02: Lewati bill tertua ditolak dengan error informatif
- [ ] FIFO-07: FIFO per due_id (bukan global)
- [ ] FIFO-08: Bill yang sudah paid tidak dihambat FIFO
- [ ] Frontend menampilkan error FIFO
- [ ] `go test ./internal/finance/...` —新增 9 test pass

**Phase 3 — Period Closing:**
- [ ] CLOSE-02: Write blocked on closed period
- [ ] CLOSE-03: Opening balance blocked on locked period
- [ ] CLOSE-06—07: Period closing via API, role-gated
- [ ] Balance response includes `is_closed`
- [ ] Tidak ada transaksi yang terhapus atau berubah

**Phase 4 — Iuran Lifecycle:**
- [ ] LIFE-04: Voluntary dues expired after due_date
- [ ] LIFE-03: Tunggakan tetap bisa di-generate
- [ ] LIFE-07—09: Year-end reset workflow
- [ ] Bill generation respects due status

**Phase 5 — Concurrency:**
- [ ] CONC-01: Concurrent verify → only one succeeds
- [ ] IDEM-01: Duplicate key → no double payment
- [ ] No deadlocks after 100 concurrent requests

---

*Desain teknis selesai. READ-ONLY. Tidak mengubah kode, database, migration, git state, atau konfigurasi.*
*Semua referensi file dan baris berdasarkan inspeksi kode pada 2026-10-21, branch urena HEAD 2def69e.*
