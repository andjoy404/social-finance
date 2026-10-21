# Follow-Up Audit — FIFO, Iuran Lifecycle, Authorization, Closing, Web/Mobile

**Branch:** `urena` | **HEAD:** `2def69e` | **Working tree:** clean
**Remote:** `origin → https://github.com/andjoy404/social-finance`

---

## 1. Baseline

| Item | Value | Status |
|------|-------|--------|
| Branch | `urena` | Confirmed |
| HEAD | `2def69e` | Confirmed |
| Working tree | Clean (no uncommitted changes) | Confirmed |
| Remote | `origin → andjoy404/social-finance` | Confirmed |
| Additional remote | `andjoy-gitops → andjoy404/andjoy-gitops` | Extra, not used for app code |

---

## 2. FIFO & Pembayaran Parsial

### 2.1 CreatePayment — Apakah menerapkan FIFO per bill_id?

**File:** `backend/internal/finance/service.go` lines 318-563

**Finding: TIDAK ada FIFO enforcement.**

`CreatePayment` menerima `bill_id` spesifik sebagai input dari frontend. Fungsi ini:
1. Mencari bill berdasarkan `bill_id` + `rt_id`
2. Menghitung remaining balance dari bill tersebut
3. Validasi: `amount <= remaining`
4. Untuk `payment_rule = 'full'`: amount HARUS sama persis dengan remaining
5. Untuk `payment_rule = 'partial'`: amount bisa berapa saja (0 < amount <= remaining)
6. Langsung membuat payment row yang linked ke `bill_id` spesifik

**Tidak ada kode yang:**
- Mengambil daftar unpaid bills dari warga (by occupancy_id)
- Mengurutkan bills berdasarkan periode/tanggal
- Memastikan bill periode paling lama dibayar duluan
- Mencegah warga bayar bill periode terbaru sebelum yang lama

### 2.2 PaymentRule — Apakah 'full' vs 'partial' mempengaruhi FIFO?

**File:** `backend/internal/finance/model.go` lines 37-41
**Migration:** `017_dues_payment_rule.up.sql`

`payment_rule` hanya mengontrol jumlah pembayaran:
- `'full'`: warga harus bayar sisa penuh (exact match)
- `'partial'`: warga bisa bayar sebagian (any positive amount <= remaining)

Tidak ada kaitan dengan urutan FIFO.

### 2.3 PaymentAllocations (Multi-Period) — FIFO di level approval?

**Migration 019:** `payment_allocations` table dengan `payment_id`, `bill_id`, `amount` + unique constraint `(payment_id, bill_id)`.

**`verifyMultiPeriodPayment` (service.go line 847-977):**
- Mengambil semua `bill_ids` dari `payment_allocations`
- **Mengurutkan bill IDs secara alfabetis** (`sort.Strings(billIDs)`) untuk mencegah deadlock — BUKAN berdasarkan periode
- Lock semua bills, validasi alokasi <= remaining per bill
- Update status semua bills
- Posting SATU kas transaction untuk seluruh payment

**FIFO per due_id TIDAK ter-enforcement di backend.** Urutan bill yang di-lock adalah alfabetis UUID, bukan kronologis.

### 2.4 Apakah FIFO enforced di frontend?

**Web (frontend/src):**
- `IuranList.tsx:555` — `Array.from(s).sort().reverse()` — sorting labels bulan, BUKAN FIFO enforcement
- `IuranArrears.tsx:60` — `Array.from(s).sort().reverse()` — sorting untuk display
- `IuranReport.tsx:66` — sorting filtered data untuk laporan
- Tidak ada kode yang memaksa warga membayar bill tertua dulu

**Android (mobile/lib/features/iuran):**
- `iuran_providers.dart:201` — `filtered.sort((a, b) { ... })` — Unpaid/partial dulu, lalu chronological (oldest first)
- `iuran_screen.dart:636` — `value.map((b) => b.periode).toList()..sort((a, b) => b.compareTo(a))` — sorting untuk dropdown

**Kesimpulan Android:** `iuran_providers.dart` mengurutkan bills secara kronologis (unpaid → oldest first) untuk **display purposes**. Ini membantu warga melihat tunggakan dari yang paling lama, tapi **tidak mencegah** warga memilih bill periode baru untuk dibayar.

### Verdict

| Aspek | Backend | Frontend |
|-------|---------|----------|
| FIFO enforcement | TIDAK ada | TIDAK ada (display only) |
| Payment targeting | Specific bill_id | Specific bill_id |
| PaymentRule effect | Amount validation only | Display hint only |
| Multi-period allocation | Sorted by UUID (deadlock prevention) | N/A |

**FIFO adalah UX concern, bukan backend constraint.**

---

## 3. Siklus Hidup Iuran

### 3.1 Apakah ada auto nonaktif iuran?

**Search:** `cron`, `scheduler`, `periodic`, `go func`, `time.Ticker`, `time.After` di seluruh `backend/`

**Finding: TIDAK ADA background job/scheduler/cron.**

Satu-satunya `goroutine` di `backend/` adalah:
1. `notification/service.go:63,112,175,223,295` — Async FCM push (fire-and-forget)
2. `cmd/api/main.go:144` — Shutdown signal handler
3. `cmd/api/main.go:170` — Health check server
4. Test files — concurrent test scenarios

**Tidak ada:**
- Scheduler untuk auto nonaktif dues (is_active = false)
- Cron job untuk generate bills periodik
- Background worker untuk reconciliasi
- Periodic task untuk closing

### 3.2 Bagaimana iuran "nonaktif" secara manual?

**File:** `backend/internal/finance/handler.go` — `UpdateDue` handler

Bendahara bisa update `is_active = false` via API:
```
PUT /api/v1/dues/:id
Body: { "is_active": false }
```

Migration 018 (`018_dues_is_voluntary.up.sql`) menambahkan `is_voluntary` column. Dues dengan `is_voluntary = true` bisa punya `amount = 0`. Dues dengan `is_voluntary = false` wajib `amount > 0`.

### 3.3 Generate Bills

**File:** `backend/internal/finance/handler.go:734` — `GenerateBills` handler

Bendahara secara manual memicu generation melalui API:
```
POST /api/v1/dues/:dueId/generate
Body: { "period": "2026-09", "due_date": "2026-09-10" }
```

Tidak ada auto-generation. Tidak ada scheduler.

### Verdict

| Aspek | Current State |
|-------|---------------|
| Auto nonaktif dues | TIDAK ADA |
| Auto generate bills | TIDAK ADA |
| Manual toggle is_active | ADA (via API) |
| Manual generate bills | ADA (via API) |
| Scheduler/Cron/Background job | TIDAK ADA |
| Periodic reconciliation | TIDAK ADA |

**Siklus hidup iuran sepenuhnya manual. Tidak ada automation.**

---

## 4. Verifikasi Pembayaran & Otorisasi

### 4.1 VerifyPayment Handler — Middleware Authorization

**File:** `backend/internal/finance/handler.go` lines 1520-1608

```go
func (h *Handler) VerifyPayment(w http.ResponseWriter, r *http.Request) {
    ac, ok := getAuthContext(r)
    if !ok {
        httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
        return
    }

    if !requireTenantContext(ac, w) {
        return
    }
    // ... no additional role check
```

**Ditemukan: Tidak ada explicit role check di VerifyPayment handler.**

### 4.2 hasStaffFinancePermission — Apakah dipanggil untuk VerifyPayment?

**File:** `backend/internal/finance/handler.go` lines 425-471

```go
func (h *Handler) hasStaffFinancePermission(ctx context.Context, tx *sql.Tx, ac *auth.AuthContext, permission string) (bool, error) {
    // 1. System-level super_admin → TRUE
    // 2. Users with jabatan → evaluate position_permissions
    // 3. Role-based pengurus/bendahara → TRUE
    // 4. Warga (no jabatan) → FALSE
```

**TAPI: `hasStaffFinancePermission` TIDAK dipanggil di `VerifyPayment` handler!**

VerifyPayment hanya mengandalkan:
1. `getAuthContext` — user harus ter-autentikasi (JWT valid)
2. `requireTenantContext` — rt_id dari JWT match

**Tidak ada cek: apakah user adalah bendahara/pengurus?**

### 4.3 Apakah ini bug?

Setiap user ter-autentikasi (termasuk WARGA) yang memiliki valid JWT + rt_id context bisa memanggil VerifyPayment. Namun ada safety net:

**Di `PaymentVerify` repository method (service.go line 599):**
```go
func PaymentVerify(...) (*Payment, error) {
    // ... FOR UPDATE lock on payment
    // Updates payment.status, verified_by, verified_at
}
```

Tidak ada role check di repository level. Siapa saja yang authenticated bisa approve/reject payment.

**Verdict: AUTHORIZATION GAP — Warga bisa memverifikasi pembayaran sendiri.**

### 4.4 CreatePayment — Apakah ada authorization check?

**File:** `backend/internal/finance/handler.go` lines 1062-1200

```go
func (h *Handler) CreatePayment(w http.ResponseWriter, r *http.Request) {
    ac, ok := getAuthContext(r)
    if !ok { ... }
    if !requireTenantContext(ac, w) { ... }
    // ... no role check
```

**Sama: Tidak ada explicit role check.** Namun CreatePayment membedakan:
- `origin = SELF_SUBMITTED` → status `PENDING` (butuh approval bendahara)
- `origin = STAFF_RECORDED` → status `APPROVED` (langsung valid)

Warga bisa submit payment, tapi payment tidak efektif tanpa approval bendahara.

### 4.5 Staff-recorded Payment — Apakah ada protection?

Di `CreatePayment` (line 332):
```go
// Staff recorded by pengurus or bendahara -> immediately approved
if payment.Origin == PaymentOriginStaffRecorded {
    payment.Status = PaymentStatusApproved
}
```

Tidak ada verifikasi apakah caller memang pengurus/bendahara. Warga bisa membuat payment dengan `origin = STAFF_RECORDED` dan langsung di-approve.

### Verdict

| Endpoint | Auth Middleware | Tenant Context | Role Check | Status |
|----------|----------------|----------------|------------|--------|
| CreatePayment | Yes (JWT) | Yes (rt_id from JWT) | TIDAK | GAP |
| VerifyPayment | Yes (JWT) | Yes (rt_id from JWT) | TIDAK | GAP |
| GenerateBills | Yes (JWT) | Yes | TIDAK | GAP |

**Semua finance handlers hanya mengandalkan `RequireAuth` + `requireTenantContext`. Tidak ada RBAC per-endpoint.**

RBAC hanya ada di:
- `hasStaffFinancePermission` — defined tapi tidak dipakai di CreatePayment/VerifyPayment
- `position_permissions` table — digunakan untuk warga write access, bukan finance read/write

---

## 5. Tutup Buku & Saldo

### 5.1 Period Balance Calculation

**File:** `backend/internal/finance/repository.go` lines 2052-2168
**Migration 024:** `024_period_opening_balances.up.sql`

```sql
CREATE TABLE IF NOT EXISTS period_opening_balances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id UUID NOT NULL,
    period VARCHAR(7) NOT NULL,   -- YYYY-MM
    opening_balance NUMERIC(15, 2) NOT NULL DEFAULT 0.00,
    source VARCHAR(20) NOT NULL DEFAULT 'auto' CHECK (source IN ('auto', 'manual')),
    previous_balance NUMERIC(15, 2),
    reason TEXT,
    updated_by TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    locked BOOLEAN NOT NULL DEFAULT false,
    CONSTRAINT uq_period_opening_balances_rt_period UNIQUE (rt_id, period)
);
```

### 5.2 Bagaimana Closing Balance dihitung?

**`CalculatePeriodBalance` (repository.go):**
1. **Opening Balance:**
   - Cek `period_opening_balances` untuk exact period match + `source = 'manual'`
   - Jika tidak ada manual, cari manual record lebih awal → cascading
   - Jika tidak ada manual → hitung dari ledger (`SUM(income) - SUM(expense)`)
2. **Current Period Movement:**
   - `Income = SUM(transactions WHERE type = 'income' AND occurred_at IN [period_start, period_end))`
   - `Expense = SUM(transactions WHERE type = 'expense' AND occurred_at IN [period_start, period_end))`
3. **Closing Balance = Opening + Income - Expense**

### 5.3 Apakah ada "tutup buku" yang mengunci periode?

**Migration 024:** Ada kolom `locked BOOLEAN` di `period_opening_balances`.

**TAPI:** Tidak ada handler/API yang:
- Menetapkan `locked = true` untuk menutup periode
- Mencegah transaksi di periode yang sudah ditutup
- Mencegah perubahan opening balance pada periode terkunci (hanya `INSERT ... ON CONFLICT UPDATE`, tanpa cek `locked`)

**Update opening balance (repository.go line 2201):**
```sql
INSERT INTO period_opening_balances (...)
ON CONFLICT (rt_id, period)
DO UPDATE SET
    opening_balance = EXCLUDED.opening_balance,
    source = EXCLUDED.source,
    previous_balance = period_opening_balances.opening_balance,
    ...
```

Tidak ada pengecekan `locked`. Siapa saja yang punya akses bisa update.

### 5.4 Apakah saldo dihitung dari ledger atau ditimbun?

**Ledger-only — sesuai AGENTS.md Section 4.2:**

Balance selalu dihitung dari ledger:
```
Balance = Total Posted Income - Total Posted Expenses
```

Tidak ada kolom `current_balance` di tabel apapun. Ini sesuai dengan "NEVER store a derived value" rule.

### 5.5 Period-Closing Lock

**Finding: TIDAK ADA period-closing lock.**

| Fitur | Status |
|-------|--------|
| Period opening balance table | ADA |
| Manual opening balance correction | ADA |
| Cascading from previous period | ADA |
| Closing balance calculation | ADA |
| Lock periode (tutup buku) | TIDAK ADA |
| Prevent post-period transactions | TIDAK ADA |
| Audit trail untuk opening correction | ADA (source + previous_balance + reason) |

### Verdict

Perhitungan saldo benar (ledger-only, fixed precision). Namun tidak ada mekanisme "tutup buku" yang mengunci periode. Opening balance bisa diubah kapan saja.

---

## 6. Perbandingan Web dan Android

### 6.1 FIFO/Bill Ordering

| Aspek | Web | Android |
|-------|-----|---------|
| Sort unpaid bills | Display only (`sort().reverse()` di IuranList, IuranArrears) | Display only (`filtered.sort` di iuran_providers.dart: unpaid dulu, oldest first) |
| Enforce FIFO on submit | TIDAK | TIDAK |
| Payment targeting | User pilih bill_id spesifik | User pilih bill_id spesifik |
| Period sort helper | `formatPeriodeLabel()` — sorting labels | `_parsePeriodForSort()` — parse "Oktober 2026" → [2026, 10] |

**Konsistensi: KONSISTEN.** Kedua platform tidak enforce FIFO, kedua platform sort untuk display.

### 6.2 Payment Submission

| Aspek | Web | Android |
|-------|-----|---------|
| Proof upload | `multipart/form-data`, storage upload | Dio multipart, storage upload |
| Payment method | CASH / TRANSFER | CASH / TRANSFER |
| Amount validation | Backend: `cents > 0` | Backend: `cents > 0` |
| Self-submit → PENDING | Yes (origin = SELF_SUBMITTED) | Yes (origin = SELF_SUBMITTED) |
| Staff-record → APPROVED | Yes (origin = STAFF_RECORDED) | Yes (origin = STAFF_RECORDED) |

**Konsistensi: KONSISTEN.**

### 6.3 Bill Status Display

| Status | Web | Android |
|--------|-----|---------|
| unpaid | Badge merah | Badge merah |
| partial | Badge kuning | Badge kuning |
| paid | Badge hijau | Badge hijau |
| cancelled | Badge abu | Badge abu |

**Konsistensi: KONSISTEN.**

### 6.4 Notification Support

| Platform | Notification |
|----------|-------------|
| Web | No in-app notification system (relies on page refresh) |
| Android | FCM push notifications (NotifyPaymentSubmitted, NotifyPaymentVerified) |

### Verdict

**Web dan Android secara fungsional konsisten.** Tidak ada perbedaan dalam logic pembayaran, FIFO, billing, atau status. Perbedaan hanya di:
- Android punya FCM push notifications
- Web uses React hooks for real-time data invalidation
- UI styling berbeda (CSS vs Flutter widgets)

---

## 7. Validasi Repository & Remote

| Item | Value |
|------|-------|
| Primary remote | `origin → https://github.com/andjoy404/social-finance` |
| Secondary remote | `andjoy-gitops → https://github.com/andjoy404/andjoy-gitops` |
| Branch on origin | `main` (default) |
| Current branch | `urena` |
| Working tree | Clean |
| Git LFS | Not detected |
| SSH vs HTTPS | HTTPS |

**andjoy-gitops** adalah remote terpisah untuk gitops config, bukan app source code. Tidak terkait dengan workflow `origin`.

---

## 8. Migration Inventory Summary

| Migration | File | Purpose |
|-----------|------|---------|
| 009 | `009_finance.up.sql` | Core finance schema: categories, dues, bills, payments, transactions, idempotency_keys, audit_logs |
| 017 | `017_dues_payment_rule.up.sql` | Add `payment_rule` ('full'/'partial') to dues |
| 018 | `018_dues_is_voluntary.up.sql` | Add `is_voluntary` to dues, allow amount >= 0 |
| 019 | `019_payment_allocations.up.sql` | Multi-period payment allocation table |
| 023 | `023_reconcile_iuran_payments.up.sql` | Iuran→KAS reconciliation |
| 024 | `024_period_opening_balances.up.sql` | Period opening balances with manual correction |

Total migrations: **24+** (sequence may have gaps from intermediate migrations)

---

## 9. Critical Findings Summary

### HIGH — Authorization Gaps

1. **VerifyPayment has no role check** — Any authenticated warga can approve/reject payments
2. **CreatePayment has no role check** — Any authenticated warga can create STAFF_RECORDED (immediately approved) payments
3. **hasStaffFinancePermission defined but never called** — Authorization infrastructure exists but not wired to finance handlers

### MEDIUM — FIFO Not Enforced

4. **CreatePayment targets specific bill_id only** — No backend FIFO enforcement per due_id or occupancy
5. **PaymentAllocations sorted by UUID** — Deadlock prevention, not business-logic ordering

### MEDIUM — No Period Closing

6. **No period-closing lock mechanism** — `period_opening_balances.locked` column exists but no code sets it
7. **Opening balance changeable anytime** — No audit gate or lock enforcement on `period_opening_balances`

### LOW — Auto Nonaktif Not Found

8. **No scheduler/cron for dues** — All iuran lifecycle (generate bills, toggle is_active) is manual
9. **No auto-recurring bills** — Bendahara must manually trigger bill generation

---

## 10. Recommendations

### A. Authorization (HIGH Priority)

Add explicit role/rbac check to finance handlers:

```go
func (h *Handler) VerifyPayment(w http.ResponseWriter, r *http.Request) {
    ac, ok := getAuthContext(r)
    if !ok { ... }
    if !requireTenantContext(ac, w) { ... }

    // ADD THIS:
    if allowed, err := h.hasStaffFinancePermission(r.Context(), nil, ac, "finance.update"); err != nil || !allowed {
        httpx.WriteForbidden(w, "only staff can verify payments", nil)
        return
    }
    // ...
}
```

Same for `CreatePayment` and `GenerateBills`.

### B. FIFO Enforcement (MEDIUM Priority)

If FIFO is a business requirement, enforce at backend level:

```go
func (s *Service) CreatePayment(ctx, billID, rtID) error {
    // Get all unpaid/partial bills for this occupancy
    unpaidBills, err := BillsGetUnpaidByOccupancy(ctx, tx, occupancyID, rtID)
    // Check: is this the oldest unpaid bill?
    if oldestID := unpaidBills[0].ID; billID != oldestID {
        return fmt.Errorf("please pay the oldest bill first: %s", oldestID)
    }
}
```

Alternatively, make multi-period payment the default and let backend handle FIFO allocation.

### C. Period Closing (MEDIUM Priority)

Implement period-lock API:

```
POST /api/v1/periods/:period/close
Body: { "reason": "Monthly closing" }
```

This sets `locked = true` and prevents:
- Opening balance changes on locked periods
- New transactions with `occurred_at` in closed periods

### D. Background Jobs (LOW Priority)

Consider adding:
- Periodic bill generation (monthly cron)
- Auto-archive old dues (yearly)
- Reconciliation job (daily)

---

*Audit completed: Read-only inspection. No files modified. No database changes.*
