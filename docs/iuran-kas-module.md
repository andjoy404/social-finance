# Iuran & KAS Module — Blueprint & Implementation Plan

> **Status:** Backend Complete (Iuran + KAS Ledger) · UI Phase Mock-Only · Period Balance + Manual Opening Balance Correction PLANNED
> **Created:** 2026-10-02
> **Last Updated:** 2026-10-15 (Consolidated: blueprint + implementation audit + Opening Balance specification)
> **Related:** [KAS Deep Implementation Audit](./kas-implementation-audit-report.md)

---

## 1. Status Dokumen

| Field | Value |
|-------|-------|
| **Status** | Backend Complete (Iuran + KAS Ledger) · UI Mock-Only · Period Balance NOT Implemented |
| **Created** | 2026-10-02 |
| **Last Updated** | 2026-10-15 |
| **Scope** | Modul Iuran (RT Dues) + Modul KAS (Cash Management) — blueprint, implementation status, implementation plan |
| **Sources of Truth** | [Database Schema](./database.md) · [API Reference](./api.md) · [Implementation Audit](./kas-implementation-audit-report.md) |

---

## 1.1 Current Implementation Snapshot

### Backend — IMPLEMENTED (Verified 2026-10-15)

| Area | Status | Details |
|------|--------|---------|
| **Iuran domain** | ✅ Complete | `dues` (migration 017), `bills` (migration 015-016), `payments` (migration 016, 020), multi-period (migration 020) |
| **KAS ledger** | ✅ Complete | `transactions` table (migration 009), `financial_categories`, `idempotency_keys`, `audit_logs`, `payment_allocations` |
| **Balance calculation** | ⚠️ Cumulative only | `CalculateBalance` (repository.go:1078) returns total income, total expense, net balance — **NO period filter** |
| **Iuran → KAS integration** | ✅ Working | Payment approval auto-creates KAS income transaction in same DB transaction (atomic) |
| **Reversal mechanism** | ✅ Working | `reverses_transaction_id` FK, opposite-type transaction, original stays `posted` |
| **Idempotency** | ✅ Working | `idempotency_keys` table + PostgreSQL advisory locks |
| **Tenant isolation** | ✅ Working | JWT-derived `rt_id`, never client-supplied |
| **Manual transactions** | ✅ Working | `CreateManualTransaction` endpoint (status always `posted`) |
| **Period balance** | ❌ NOT IMPLEMENTED | No opening/closing balance per period, no period filters |
| **Historical opening balance** | ❌ BUSINESS_DECISION_REQUIRED | No mechanism for RT existing balance before app use |
| **Web KAS API integration** | ❌ MOCK ONLY | 6 files, zero API calls |
| **Android KAS API integration** | ❌ MOCK ONLY | 7 files, zero API calls |

### Frontend — MOCK-ONLY

| Platform | Feature | Status | Data |
|----------|---------|--------|------|
| **Web (React)** | Iuran — list, detail, payment, arrears, report | ✅ Implemented | Mock / local |
| **Web (React)** | KAS — list, income, expense, report | ✅ Implemented | Mock / local |
| **Android (Flutter)** | Iuran — list, detail, payment, arrears, report | ✅ Implemented | Mock / local |
| **Android (Flutter)** | KAS — list, income, expense, report | ✅ Implemented | Mock / local |
| **Web** | Navigation — sidebar/menu wiring | ✅ Implemented | — |
| **Android** | Navigation — bottom bar, routes, shells | ✅ Implemented | — |

**Validation results:**
- `flutter analyze`: 0 errors
- `flutter test`: 111 passed
- `flutter build apk --debug`: success
- Web build: success

---

## 2. Goals

- Menyediakan struktur iuran yang dapat didefinisikan per RT, dengan jenis iuran, periode, dan nominal yang dapat dikelola.
- Mencatat tagihan iuran kepada **pemilik atau penyewa rumah** dan melacak status pembayarannya.
- Menyediakan modul Kas untuk mencatat pemasukan dan pengeluaran kas RT secara umum.
- Mengintegrasikan pembayaran iuran ke dalam transaksi Kas Masuk tanpa menyebabkan double-entry.
- Memastikan semua transaksi keuangan mengikuti prinsip immutability dan audit trail yang sama dengan Financial Ledger yang sudah ada.

---

## 3. Business Rules

### 3.1 Iuran Subject Decision

**Business decision:** Iuran **tidak ditujukan kepada KK/Household**.

Iuran ditujukan kepada: **pemilik atau penyewa rumah**.

This means Iuran bills target **occupancy + house**, not `householdId`/`householdName`.

The existing `households` model may continue to exist as part of the Warga module, but **Iuran must not use `householdId`/`householdName` as its billing subject**. UI labels such as `KK`, `Kepala Keluarga`, or `WARGA/KK` must not be used for Iuran billing subject.

If a technical relationship to households is needed for resolving Warga data, document it as a **technical relationship**, not as a billing subject.

### 3.2 Business Rules Status

| BR | Status | Description |
|----|--------|-------------|
| **BR-01** Billing subject | ✅ FINALIZED | Per occupancy (OWNER/TENANT). `bills.household_occupancy_id` FK. |
| **BR-02** Nominal | ✅ FINALIZED | Fixed nominal per due type. `dues.amount` applied to all bills. |
| **BR-03** Partial payment | ✅ FINALIZED | Allowed. `BillStatus` supports: `unpaid` → `partial` → `paid`. |
| **BR-04** Multi-period | ✅ FINALIZED | Allowed via `payment_allocations` table. `CreateMultiPeriodPayment` service method. |
| **BR-05** Cancellation | ✅ FINALIZED | Pilihan A: Reversal only. Original stays `posted`. |
| **BR-06** KAS correction | ⚠️ PARTIALLY FINALIZED | Schema has `draft`/`posted`/`cancelled`, but manual transaction always created as `posted`. No draft workflow for manual transactions. |
| **BR-07** Authorization | ✅ Enforcement active | 4 roles: `super_admin`, `pengurus`, `bendahara`, `warga`. JWT-based routing via `RequireRole` middleware. |

---

## 4. Current Implementation Status

### 4.1 Existing House/Occupancy Model

The backend already has a complete house/occupancy/owner/tenant model. This is **existing Warga infrastructure**, not new Iuran design.

#### Entity Chain

```
physical_houses
    ↓ (via physical_house_id)
household_occupancies
    ↓ (via household_id)
households
    ↓ (via residency_periods)
residency_periods
    ↓ (via resident_id)
residents
```

#### Key Finding: Simultaneous Owner + Tenant is NOT Supported

The EXCLUDE constraint prevents two household occupancies from overlapping on the same physical house. Only sequential (non-overlapping) periods are supported.

### 4.2 Iuran Legacy Mock Model — SUPERSEDED

The current Web Iuran mock uses a **household-based billing subject**. This is documented here as legacy only and **must not be carried forward**.

#### Legacy IuranBill Type (SUPERSEDED)

```typescript
interface IuranBill {
  householdId: string      // ← TO BE REPLACED
  householdName: string    // ← TO BE REPLACED
  rt: string
  iuranType: string
  periode: string
  nominal: number
  paidAmount: number
  status: IuranStatus
  payments?: IuranPaymentRecord[]
}
```

These labels and fields are **prototype artifacts**. The actual implementation uses `household_occupancy_id` FK, `dues` table, and `bills` table (see [database.md](./database.md)).

### 4.3 KAS UI Design Decisions — Current Mock

#### Web — Blueprint: Split Ledger (Design Only, Not Implemented)

KAS Web blueprint describes a two-column split ledger presentation:

```
┌──────────────────────┬──────────────────────┐
│ + 02 Okt 2026        │ - 02 Okt 2026        │
│   Iuran Keamanan     │   Pembelian Lampu    │
│   Rp 500.000         │   Rp 350.000         │
├──────────────────────┼──────────────────────┤
│ + 01 Okt 2026        │                      │
│   Donasi             │                      │
│   Rp 1.000.000       │                      │
└──────────────────────┴──────────────────────┘
```

**Rules:**
- Left column = Kas Masuk (income)
- Right column = Kas Keluar (expense)
- Latest transaction at top
- Split is **presentation layer only**
- Backend remains a single `transactions` ledger

**Current mock reality:** `KasList.tsx` uses a flat table with columns: Tanggal, Keterangan, Pemasukan, Pengeluaran, Saldo. **NOT split ledger.** This is a design-vs-prototype discrepancy.

#### Android — Single Chronological List

KAS Android uses a single chronological list:

```
02 Okt 2026  Kas Masuk   + Rp500.000
02 Okt 2026  Kas Keluar  - Rp350.000
01 Okt 2026  Kas Masuk   + Rp1.000.000
30 Sep 2026  Kas Keluar  - Rp750.000
```

**Rules:**
- All transactions in one list
- Global order: newest → oldest
- No two-column split on Android

---

## 5. Architecture

### 5.1 Layered Architecture

```
HTTP Handler
   ↓
Service (business logic)
   ↓
Repository (data access)
   ↓
PostgreSQL
```

Each layer has a single responsibility. Handlers do not contain business logic. Services do not directly access the database. Repositories do not contain business rules.

### 5.2 Source of Truth References

For detailed database schema, see **[database.md](./database.md)**.

For detailed API endpoints, see **[api.md](./api.md)**.

---

## 6. Iuran Domain

### 6.1 Iuran Type — SUPERSEDED

The conceptual `iuran_type` table described in the original blueprint has been superseded by the `dues` table (migration 017-019).

**Actual table: `dues`** (see [database.md](./database.md))

| Field | Type | Notes |
|-------|------|-------|
| `id` | uuid | Primary key |
| `rt_id` | uuid | Tenant isolation |
| `name` | text | E.g., "Iuran IPL" |
| `amount` | NUMERIC(15,2) | Always >= 0 |
| `period_type` | text | `monthly`, `yearly`, `one_time` |
| `payment_rule` | text | `full` or `partial` |
| `is_voluntary` | boolean | If true, amount can be Rp0, warga sets donation amount |
| `is_active` | boolean | Soft delete |
| `created_at` | timestamptz | — |
| `updated_at` | timestamptz | — |

### 6.2 Iuran Bill — SUPERSEDED

The conceptual `iuran_bill` table has been superseded by the `bills` table (migration 015-016).

**Actual table: `bills`** (see [database.md](./database.md))

| Field | Type | Notes |
|-------|------|-------|
| `id` | uuid | Primary key |
| `rt_id` | uuid | Tenant isolation |
| `household_occupancy_id` | uuid | FK → `household_occupancies` |
| `due_id` | uuid | FK → `dues` |
| `amount` | NUMERIC(15,2) | Always positive |
| `period` | text | E.g., "2026-10" |
| `due_date` | date | — |
| `status` | text | `unpaid`, `partial`, `paid`, `cancelled` |
| `created_at` | timestamptz | — |
| `updated_at` | timestamptz | — |

### 6.3 Iuran Payment — SUPERSEDED

The conceptual `iuran_payment` table has been superseded by the `payments` table (migration 016, 020).

**Actual table: `payments`** (see [database.md](./database.md))

Key fields:
- `bill_id` FK → `bills` (for bill payments)
- `due_id` FK → `dues` (for voluntary donations, mutually exclusive with `bill_id`)
- `status`: `PENDING`, `APPROVED`, `REJECTED`
- `method`: `CASH`, `TRANSFER`
- `origin`: `SELF_SUBMITTED` (warga), `STAFF_RECORDED` (pengurus/bendahara)
- `proof_path`: uploaded proof image
- `submitted_by`, `verified_by`, `verified_at`, `payment_no`, `rejection_reason`, `officer_notes`

See [audit report](./kas-implementation-audit-report.md) Section B.3 for complete flow.

---

## 7. KAS Domain

### 7.1 KAS Transaction — ACTUAL

**Actual table: `transactions`** (migration 009, see [database.md](./database.md))

| Field | Type | Notes |
|-------|------|-------|
| `id` | uuid | Primary key |
| `rt_id` | uuid | Tenant isolation |
| `category_id` | uuid | FK → `financial_categories` |
| `amount` | NUMERIC(15,2) | Always positive, CHECK > 0 |
| `type` | text | `income` or `expense` (CHECK constraint) |
| `status` | text | `draft`, `posted`, `cancelled` (CHECK, default 'posted') |
| `reverses_transaction_id` | uuid | FK → `transactions(id)` — reversal references original |
| `payment_id` | uuid | FK → `payments(id)` — traceable to payment source |
| `description` | text | NOT NULL |
| `occurred_at` | timestamptz | — |
| `created_at` | timestamptz | — |
| `updated_at` | timestamptz | — |

**Notes:**
- Blueprint Section 6.1 originally described `kas_transaction` with `source_type`/`source_id` columns. **These columns do not exist in the actual schema.**
- Traceability to payment source is via `payment_id` FK, not `source_type`/`source_id`.
- Idempotency is enforced via `idempotency_keys` table (migration 009) + PostgreSQL advisory locks, NOT a unique constraint on `source_type + source_id`.
- Full schema details: see [database.md](./database.md).

### 7.2 Balance Calculation

#### 7.2.1 Cumulative Balance (IMPLEMENTED)

Current implementation: `CalculateBalance` (repository.go:1078)

```sql
SELECT COALESCE(SUM(CASE WHEN type = 'income' THEN amount ELSE 0 END), 0)::text as total_income,
       COALESCE(SUM(CASE WHEN type = 'expense' THEN amount ELSE 0 END), 0)::text as total_expense,
       COUNT(*) as tx_count
FROM transactions
WHERE rt_id = $1 AND status = 'posted'
```

**Properties:**
- ✅ Fixed-precision: `NUMERIC(15,2)` + `ParseMoney()`/`FormatMoney()`
- ✅ Only `status = 'posted'` — excludes draft, cancelled
- ✅ Per `rt_id` — tenant isolation
- ✅ No floating-point

**Limitation:**
- ❌ No period filtering — returns cumulative total of all history
- ❌ No opening/closing balance per period
- ❌ No running balance

#### 7.2.2 Period Balance (NOT IMPLEMENTED)

Period-based balance calculation is **NOT implemented**. This is the foundation needed for the KAS neraca (balance sheet) table.

**Timezone & interval:** All period queries use half-open interval `[start, end_exclusive)` in timezone `Asia/Jakarta` (UTC+7).

| Concept | Calculation | Status |
|---------|-------------|--------|
| **Opening balance** | SUM(net impact of all posted transactions before period start) — or manual correction value | ❌ NOT IMPLEMENTED |
| **Period income** | SUM(income WHERE occurred_at >= start AND occurred_at < end) | ❌ NOT IMPLEMENTED |
| **Period expense** | SUM(expense WHERE occurred_at >= start AND occurred_at < end) | ❌ NOT IMPLEMENTED |
| **Closing balance** | Opening + Income - Expense | ❌ NOT IMPLEMENTED |

**Period balance semantics:**
- `draft` and `cancelled` transactions do NOT affect balance
- Reversal: original stays `posted` + reversal creates opposite-type entry. Net effect = zero. Correct for period calculation because both entries are `posted` and appear in the ledger.
- Cross-period reversal (e.g., income in Month 1, reversal in Month 3):
  - Month 1: Opening=0, Income=X, Closing=X ✅
  - Month 3: Opening=X, Expense=X, Closing=0 ✅
- All calculations per `rt_id` — tenant isolated.

**Interval notation for `period=YYYY-MM`:**
```
start      = YYYY-MM-01 00:00:00+07:00  (inclusive)
end_excl   = (next month)-01 00:00:00+07:00  (exclusive)
```
Example: `period=2026-10` → `[2026-10-01T00:00:00+07:00, 2026-11-01T00:00:00+07:00)`

**Interval notation for explicit `start_date` + `end_date`:**
```
start_date=2026-10-01 → 2026-10-01T00:00:00+07:00 (inclusive)
end_date=2026-11-01   → 2026-11-01T00:00:00+07:00 (exclusive)
```
`end_date` represents the **first moment after** the period ends (half-open interval). This avoids `23:59:59` precision issues.

**Quarterly/rolling period note:** `start_date` and `end_date` must be sent as a pair. If only one is present, reject with HTTP 400.

### 7.2.3 Period Opening Balance — Auto & Manual Correction

**Auto-calculation rule:** For each period, the opening balance equals the closing balance of the previous period.

`Closing = Opening + Total Income − Total Expense`

| Periode | Saldo pembuka | Pemasukan | Pengeluaran | Mutasi bersih | Saldo penutup |
|---|---:|---:|---:|---:|---:|
| September | Rp500.000 | Rp200.000 | Rp0 | +Rp200.000 | Rp700.000 |
| Oktober | Rp700.000 | Rp100.000 | Rp0 | +Rp100.000 | Rp800.000 |
| November | Rp800.000 | Rp0 | Rp50.000 | −Rp50.000 | Rp750.000 |

**Source field:** `period_opening_balances.opening_balance` (type `NUMERIC(15,2)`).

| Field | Value |
|---|---|
| **Source type** | `auto` (from previous period closing) or `manual` (set/corrected by authorized user) |
| **Audit trail** | `period` (year-month), `opening_balance_before`, `opening_balance_after`, `reason`, `corrected_by`, `corrected_at`, `source` |

**Propagation rule (Antar-bulan):**

- Opening balance is **auto-propagated** from the previous period's closing balance, **unless** it has been **manually corrected**.
- Once manually corrected, the new value is **locked** and is **NOT overwritten** by automatic recalculation when earlier periods change.
- If an earlier period changes (due to a reversal or new transaction), periods with **auto** opening balances are **recalculated**. Periods with **manual** opening balances are **not changed**.

**Example:**

1. Oktober closing = Rp800.000 → November auto opening = Rp800.000.
2. Bendahara corrects November opening to Rp900.000 (`source=manual`).
3. If Oktober later changes, November remains Rp900.000 (manual lock preserved).
4. If November opening is **not** manually corrected, it would automatically become the new Oktober closing.

### 7.2.4 Historical Opening Balance (Before App Use)

The auto-calculation from ledger only works if the RT has no existing kas balance before using the application. If an RT has RpXX.XXX.XXX in kas before going live:
- `Opening(first_period)` auto-calculates to Rp0 (no transactions before the first period).
- Actual balance will be higher than displayed.
- **Decision required at onboarding:** RT admin sets a one-time historical opening balance during RT initialization. This is **NOT** a monthly input — it is a one-time entry.

**Status:** `BUSINESS_DECISION_REQUIRED` — no mechanism exists in code or documentation to represent historical opening balance prior to app use.

### 7.4 KAS Transaction Sources

KAS transactions come from:

**Automatic source:**
- Iuran Payment (auto-created when payment is approved: `VerifyPayment` or staff auto-approve `CreatePayment`)
- Voluntary Donation (auto-created via `due_id` path)

**Manual source:**
- `CreateManualTransaction` endpoint — created by pengurus/bendahara with status `posted`

**Not all KAS transactions originate from Iuran.** KAS is a general-purpose financial ledger.

### 7.5 Iuran → KAS Integration

Every successful Iuran payment produces exactly one KAS `income` transaction in the same database transaction:

| Flow | Location | Description |
|------|----------|-------------|
| Staff auto-approve | `service.go:319-366` | `CreatePayment` with status `APPROVED` posts income |
| Warga submit → Bendahara approve | `service.go:568-602` | `verifySinglePayment` posts income after approve |
| Donation via `due_id` | `service.go:273-282` | `postPaymentIncome()` on staff-approved donation |
| Donation approval | `service.go:516-530` | `postPaymentIncome()` on donation approve |
| Multi-period payment | `service.go:891-907` | ONE income transaction for entire payment |

**Rejection:** Payment rejection (overpayment or bendahara reject) does NOT create a KAS transaction. Payment row set to `REJECTED`, no `TransactionCreate` call.

**Atomicity:** Payment update + bill status update + KAS transaction creation in one `tx *sql.Tx`. If any step fails, entire transaction rolls back.

**Traceability:** `transactions.payment_id` FK links to `payments.id`. No `source_type`/`source_id` columns exist.

### 7.6 Reversal

Manual reversal via `ReverseTransaction` (service.go:960-1012):

- Creates opposite-type transaction (income → expense, expense → income)
- Same amount as original
- `reverses_transaction_id` set to original's ID
- `payment_id` copied from original (traceability preserved)
- Original transaction REMAINS `posted`
- Cannot reverse already-reversed transaction
- Cannot reverse non-posted transaction
- Both original and reversal stay `posted` — ledger immutability maintained

### 7.7 Idempotency

Mechanism: PostgreSQL advisory lock + `idempotency_keys` table (migration 009) + `idempotency.go`.

Protected endpoints:
- `POST /api/v1/transactions` — CreateTransaction
- `POST /api/v1/transactions/{id}/reverse` — ReverseTransaction
- `POST /api/v1/payments` — CreatePayment
- `POST /api/v1/bills/generate` — GenerateBills

### 7.8 Tenant Isolation

- `rt_id` derived from JWT claims (`AuthContext.RTID`), never from URL path or query parameter
- `requireTenantContext(ac, w)` guards writes from super_admin without RT membership
- Tenant-scoped users: all queries filter by `rt_id`
- SUPER_ADMIN system-only: `ac.RTID == ""` → cross-RT queries

### 7.9 Financial Categories

**Actual table: `financial_categories`** (migration 009)

| Field | Type | Notes |
|-------|------|-------|
| `id` | uuid | Primary key |
| `rt_id` | uuid | Tenant isolation |
| `name` | text | — |
| `type` | text | CHECK: `income` or `expense` |
| `is_active` | boolean | Soft delete |
| `created_at` | timestamptz | — |
| `updated_at` | timestamptz | — |

No default seed data. `FindOrCreateDefaultIncomeCategory` auto-creates "Iuran Warga" on first payment approval. Expense categories require manual seed.

---

## 8. API Contract Summary

### 8.1 Iuran (Finance Resource)

| Method | Endpoint | Handler | Auth |
|--------|----------|---------|------|
| `GET` | `/api/v1/dues` | ListDues | JWT + tenant |
| `POST` | `/api/v1/dues` | CreateDue | JWT + tenant |
| `PATCH` | `/api/v1/dues/{id}` | UpdateDue | JWT + tenant |
| `DELETE` | `/api/v1/dues/{id}/deactivate` | DeactivateDue | JWT + tenant |
| `GET` | `/api/v1/bills` | ListBills | JWT + tenant |
| `GET` | `/api/v1/bills/{id}` | GetBill | JWT + tenant |
| `POST` | `/api/v1/bills/generate` | GenerateBills | JWT + tenant + idempotency |
| `POST` | `/api/v1/payments` | CreatePayment | JWT + tenant + multipart + idempotency |
| `GET` | `/api/v1/payments` | ListPayments | JWT + tenant |
| `GET` | `/api/v1/payments/{id}` | GetPayment | JWT + tenant |
| `POST` | `/api/v1/payments/{id}/verify` | VerifyPayment | JWT + tenant (pengurus) |

### 8.2 KAS (Transactions & Reports)

| Method | Endpoint | Handler | Auth |
|--------|----------|---------|------|
| `GET` | `/api/v1/transactions` | ListTransactions | JWT + tenant |
| `GET` | `/api/v1/transactions/{id}` | GetTransaction | JWT + tenant |
| `POST` | `/api/v1/transactions` | CreateTransaction | JWT + tenant + idempotency |
| `POST` | `/api/v1/transactions/{id}/reverse` | ReverseTransaction | JWT + tenant + idempotency |
| `GET` | `/api/v1/reports/balance` | GetBalance | JWT + tenant |

**Note:** Blueprint Section 10 described endpoints under `/api/v1/rt/:id/iuran/*` and `/api/v1/rt/:id/kas/*`. Actual endpoints use base path `/api/v1/` with resource names (dues, bills, payments, transactions). See [api.md](./api.md) for full reference.

### 8.3 Period Balance API (NOT IMPLEMENTED)

**Endpoint:** `GET /api/v1/reports/balance`

**Current behavior:** Accepts **no period parameters**. Returns cumulative total for all history.

**Backward compatibility:** When called **without** period parameters, response format is **exactly the same** as current:
```json
{
  "total_income": "1200000.00",
  "total_expense": "320000.00",
  "net_balance": "880000.00",
  "transaction_count": 25
}
```

**Query parameters (optional, mutually exclusive sets):**

| Param | Type | Required | Conflict | Default |
|---|---|:---:|---|---|
| `period` | string | ❌ | Cannot use with `start_date` or `end_date` | — |
| `start_date` | string | ❌ | Must be used with `end_date` | — |
| `end_date` | string | ❌ | Must be used with `start_date` | — |

**Parameter validation rules:**

| Rule | Condition | HTTP Response |
|---|---|---|
| P-1 | `period` + `start_date` together | **400** — "period dan start_date tidak bisa digunakan bersamaan" |
| P-2 | `period` + `end_date` together | **400** — "period dan end_date tidak bisa digunakan bersamaan" |
| P-3 | `start_date` without `end_date` | **400** — "end_date wajib diberikan jika start_date digunakan" |
| P-4 | `end_date` without `start_date` | **400** — "start_date wajib diberikan jika end_date digunakan" |
| P-5 | `period` format invalid (not YYYY-MM) | **400** — "format period harus YYYY-MM" |
| P-6 | `period` month not 01–12 | **400** — "bulan harus antara 01 dan 12" |
| P-7 | `start_date`/`end_date` format invalid (not YYYY-MM-DD) | **400** — "format tanggal harus YYYY-MM-DD" |
| P-8 | `end_date` ≤ `start_date` | **400** — "end_date harus setelah start_date" |

**Period interval semantics (half-open `[start, end_exclusive)`):**

| Param | Expansion (Asia/Jakarta timezone) |
|---|---|
| `period=2026-10` | `start=2026-10-01T00:00:00+07:00`, `end_excl=2026-11-01T00:00:00+07:00` |
| `start_date=2026-10-01&end_date=2026-11-01` | Same as above |

`end_date` represents the **first moment after** the period ends. This avoids `23:59:59` precision issues with `timestamptz`.

**Response with period params (proposed):**

```json
{
  "opening_balance": "1200000.00",
  "total_income": "850000.00",
  "total_expense": "320000.00",
  "net_movement": "530000.00",
  "closing_balance": "1730000.00",
  "transaction_count": 25
}
```

| Field | Type | Description |
|---|---|---|
| `opening_balance` | string | Balance at period start (from ledger auto-calc or manual correction) |
| `total_income` | string | SUM(income WHERE `occurred_at >= start AND < end`) |
| `total_expense` | string | SUM(expense WHERE `occurred_at >= start AND < end`) |
| `net_movement` | string | `total_income - total_expense` |
| `closing_balance` | string | `opening_balance + net_movement` |
| `transaction_count` | int | COUNT(POSTED transactions in period) |

**Note:** `total_income` and `total_expense` in period response refer to **income/expense within the period only**, not cumulative. For backward compatibility, the cumulative fields (`total_income`, `total_expense`, `net_balance`) are returned **only when no period params are provided**.

**SUPER_ADMIN behavior:** This endpoint is under the `finance.read` permission route group, which only allows `warga`, `bendahara`, `pengurus` roles. `SUPER_ADMIN sistem-only` (`ac.RTID == ""`) is **blocked by middleware** before reaching the handler. If SUPER_ADMIN period balance is needed in the future, a separate admin endpoint with explicit RT selection must be created.

### 8.4 Opening Balance Correction API (NOT IMPLEMENTED)

**Endpoint:** `PUT /api/v1/reports/balance/opening/:period`

**Auth:** `finance.write` / `finance.update` permissions, `bendahara` or `pengurus` role, or `SUPER_ADMIN` with explicit RT selection.

**Request body:**

```json
{
  "opening_balance": "900000.00",
  "reason": "Koreksi saldo pembuka berdasarkan cek kas fisik"
}
```

| Field | Type | Required | Notes |
|---|---|:---:|---|
| `opening_balance` | string | ✅ | `NUMERIC(15,2)`, non-negative |
| `reason` | string | ✅ | Human-readable reason for the correction |

**Response:**

```json
{
  "period": "2026-11",
  "opening_balance_before": "800000.00",
  "opening_balance_after": "900000.00",
  "source": "manual",
  "corrected_by": "user-id-uuid",
  "corrected_at": "2026-10-15T14:30:00+07:00",
  "reason": "Koreksi saldo pembuka berdasarkan cek kas fisik"
}
```

**SUPER_ADMIN authorization:** SUPER_ADMIN system-only must select a target RT explicitly (e.g., via `?rt_id=uuid` in URL path, not body). The correction request must then pass through the same `requireTenantContext` authorization logic as other tenant-scoped endpoints.

### 8.5 Authorization Matrix for Opening Balance Operations

| Role | Read period balance | Set/correct opening balance |
|---|:---:|:---:|
| `warga` | ✅ (own RT only) | ❌ |
| `bendahara` | ✅ (own RT only) | ✅ (own RT only) |
| `pengurus` | ✅ (own RT only) | ✅ (own RT only) |
| `SUPER_ADMIN sistem-only` | ❌ (blocked by middleware) | ✅ (via admin endpoint + explicit RT selection) |

### 8.6 Audit Trail Requirements for Opening Balance Correction

Every opening balance correction must be logged with these fields:

| Field | Source | Notes |
|---|---|---|
| `period` (year-month) | Request body / URL param | Target period |
| `opening_balance_before` | Calculated from previous correction or auto-calc | Value before correction |
| `opening_balance_after` | Request body | Value after correction |
| `source` | `"manual"` | Always `"manual"` for correction; `"auto"` for system propagation |
| `reason` | Request body | Human-readable explanation |
| `corrected_by` | JWT claims (`AuthContext.UserID`) | User who made the correction |
| `corrected_at` | Server timestamp (`now()`) | When correction was applied |

**Persistence:** Correction records must be stored in a dedicated table (`period_opening_balances`) to survive refresh, restart, and recalculation. The `source` field (`auto` vs `manual`) determines whether a period's opening balance is **locked** (manual) or **recalculatable** (auto).

---

## 9. Implementation Plan

### Phase 1: Period Balance + Opening Balance Correction (Backend) — 5-7 hari (estimate)

| # | Task | Dependencies | Files/Modules | Acceptance Criteria | Tests |
|---|------|-------------|---------------|---------------------|-------|
| 1.1 | Create migration for `period_opening_balances` table | — | New migration file | Table has `period`, `opening_balance`, `source`, `reason`, `corrected_by`, `corrected_at` | DB migration up/down |
| 1.2 | Add `PeriodBalanceSummary` model | — | `model.go` | Struct: `opening_balance`, `total_income`, `total_expense`, `net_movement`, `closing_balance`, `transaction_count` | Compile |
| 1.3 | Implement `CalculatePeriodBalance` repository function | 1.2 | `repository.go` | Single query with `FILTER` clause: opening + income + expense + count | Unit: cumulative works, period works |
| 1.4 | Implement `PeriodOpeningBalanceUpdate` repository function | 1.1 | `repository.go` | INSERT/UPDATE `period_opening_balances` with audit fields | Unit: save + retrieve |
| 1.5 | Implement `RecalculateAutoPeriods` repository function | 1.1, 1.3 | `repository.go` | Recalculate all auto periods from a given start month | Unit: propagation correct |
| 1.6 | Update `GetBalance` handler — parse/validate period params | 1.2 | `handler.go` | Reject conflicts (P-1..P-4), invalid format (P-5..P-7), reversed (P-8) | Integration: 400 on bad params |
| 1.7 | Update `GetBalance` handler — dispatch to `CalculatePeriodBalance` | 1.3–1.5 | `handler.go` | Returns period response with opening/closing; backward compatible when no params | Integration: correct numbers |
| 1.8 | Implement `UpdateOpeningBalance` handler | 1.4 | `handler.go` | Requires auth, saves correction, logs audit, returns before/after values | Integration: 403 on unauthorized |
| 1.9 | Write period balance tests | 1.6–1.7 | `handler_test.go` | 12 test cases (see Section 11.2) | All pass |
| 1.10 | Write opening balance correction tests | 1.8 | `handler_test.go` | Auth enforcement, propagation rules, persistence | All pass |

### Phase 2: Web KAS API Integration — 3-4 hari (estimate)

| # | Task | Dependencies | Files/Modules | Acceptance Criteria | Tests |
|---|------|-------------|---------------|---------------------|-------|
| 2.1 | Create API client module | — | `frontend/src/features/kas/kas-api.ts` | Functions: `apiGetBalance`, `apiListTransactions` | Compile, type check |
| 2.2 | Replace mock in `KasList.tsx` | 2.1 | `KasList.tsx` | Real transactions displayed | Manual: renders real data |
| 2.3 | Replace mock in `KasReport.tsx` | 2.1, Phase 1 | `KasReport.tsx` | Period selector, real summary | Manual: period filter works |
| 2.4 | Period selector component | 2.3 | New or shared | Dropdown month/year picker | Manual: navigation works |
| 2.5 | Loading/error/empty states | 2.2-2.4 | All KAS files | Proper UX states | Manual: network failure handled |

### Phase 3: Web KAS Neraca Table — 3-4 hari (estimate)

| # | Task | Dependencies | Files/Modules | Acceptance Criteria | Tests |
|---|------|-------------|---------------|---------------------|-------|
| 3.1 | Balance summary card component | Phase 1, 2.3 | New component | Opening, income, expense, closing display | Manual: numbers match API |
| 3.2 | Mutasi kas table | 2.2 | `KasList.tsx` | Running balance, period filter, pagination | Manual: sorted, balanced |
| 3.3 | Reversal indicator | 3.2 | Component | ↻ icon on reversal rows | Manual: reversal shown |
| 3.4 | Period navigation | 3.3 | Component | < > buttons, "Bulan Ini" | Manual: navigation works |
| 3.5 | Pagination integration | 3.2 | Component | 20 per page, summary consistent | Manual: summary not affected by pagination |

### Phase 4: Android KAS API Integration — 2-3 hari (estimate)

| # | Task | Dependencies | Files/Modules | Acceptance Criteria | Tests |
|---|------|-------------|---------------|---------------------|-------|
| 4.1 | Create API client module | — | `mobile/lib/features/kas/kas_api_client.dart` | Functions: `getBalance`, `listTransactions` | Compile |
| 4.2 | Update models to match backend | 4.1 | `kas_models.dart` | `BalanceSummary` matches Go struct | Type check |
| 4.3 | Replace mock in `kas_screen.dart` | 4.1 | `kas_screen.dart` | Real transactions displayed | Manual: renders real data |
| 4.4 | Period filter | 4.3 | Component | Date picker | Manual: filter works |

### Phase 5: Seed Data & Polish — 1-2 hari (estimate)

| # | Task | Dependencies | Files/Modules | Acceptance Criteria | Tests |
|---|------|-------------|---------------|---------------------|-------|
| 5.1 | Seed expense categories | — | Migration or script | Default expense categories exist | DB check |
| 5.2 | Sync category names (Web vs Android) | — | Mock data files | Web and Android use same category names | Manual: consistent |
| 5.3 | End-to-end verification | All phases | — | Full flow: balance → transactions → UI | Manual: E2E |

---

## 10. Web KAS Target Design — Neraca Kas Table

**Status:** Design target, NOT implemented. All mock data currently uses flat table.

### 10.1 Period Summary Card (Header)

```
┌─────────────────────────────────────────────────────────────┐
│  Periode: [Oktober 2026 ▼]                                   │
├─────────────────────────────────────────────────────────────┤
│  Saldo Awal:     Rp 1,200,000                                │
│  Pemasukan:      Rp   850,000                                │
│  Pengeluaran:   -Rp   320,000                                │
│  ─────────────────────────────────                           │
│  Mutasi Bersih:  Rp   530,000                                │
│  Saldo Akhir:    Rp 1,730,000                                │
└─────────────────────────────────────────────────────────────┘
```

### 10.2 Mutasi Kas Table

| No | Tanggal | Keterangan | Kategori | Referensi | Pemasukan | Pengeluaran | Saldo |
|----|---------|-----------|----------|-----------|----------:|-----------:|--------:|
| — | — | **Saldo Awal** | — | — | — | — | Rp 1,200,000 |
| 1 | 02 Okt | Pembayaran IPL Budi | Iuran Warga | PAY-20261002-001 | Rp 100,000 | — | Rp 1,300,000 |
| 2 | 05 Okt | Konsumsi Rapat | Konsumsi | — | — | Rp 150,000 | Rp 1,150,000 |

**Running balance:** Calculated on frontend from received transaction data. For paginated data, running balance is accurate only for current page.

**Reversal indicator:** ↻ icon or "(reversal)" text in description column.

### 10.3 Filter & Pagination

- **Period selector:** Month/year dropdown with < > navigation
- **"Bulan Ini"** button for current period
- **"Semua Periode"** for cumulative view
- **Transaction detail:** Paginated (20 per page)
- **Summary:** NOT paginated — calculated from all matching data

### 10.4 UX States

- **Loading:** Skeleton loader for summary card + table
- **Error:** Retry button, user-friendly message
- **Empty:** Illustration, "Belum ada transaksi" text
- **No period data:** Summary shows zeros, table empty

### 10.5 Mobile (Android)

Android should use a card-based presentation appropriate for narrow screens:
- Period summary as collapsible card
- Transaction list with swipe/reveal for details
- Same API contract, same balance calculation, different layout

---

## 11. Testing Strategy

### 11.1 Backend (Implemented)

| Test | Location | Status |
|------|----------|--------|
| Payment approval → KAS transaction | `handler_test.go:543` | ✅ PASSED |
| Balance calculation (cumulative) | `handler_test.go:560` | ✅ PASSED |
| Reversal (same period) | `handler_test.go:576` | ✅ PASSED |
| Transaction idempotency | `handler_test.go:655` | ✅ PASSED |
| Overpayment rejection | `handler_test.go:~1327` | ✅ PASSED |
| Bill cancellation guard | `handler_test.go:1576` | ✅ PASSED |
| Donation → ledger transaction | `handler_test.go:1550` | ✅ PASSED |
| Multi-period exact one KAS transaction | `multi_period_payment_test.go:1117` | ✅ PASSED |
| Cross-RT tenant isolation | `handler_test.go:1970` | ✅ PASSED |

### 11.2 Backend (NOT YET TESTED — Period Balance + Opening Balance Correction)

| Test | Priority | Acceptance Criteria |
|------|:--------:|---------------------|
| `AC-1` Period balance — basic | P0 | Response with `opening_balance`, `total_income`, `total_expense`, `closing_balance` correct for a period with transactions |
| `AC-2` Backward compatibility — no params | P0 | `GET /reports/balance` without period params returns `total_income`, `total_expense`, `net_balance` (exact current format) |
| `AC-3` Half-open interval — boundary inclusion | P0 | Transaction at `start_date 00:00:00` is INCLUDED; transaction at `end_date 00:00:00` is EXCLUDED |
| `AC-4` Timezone — Asia/Jakarta | P0 | `period=2026-10` expands to `start=2026-10-01T00:00:00+07:00`, `end=2026-11-01T00:00:00+07:00` |
| `AC-5` Parameter validation — conflicts | P1 | `period` + `start_date` → 400; `period` + `end_date` → 400; `start_date` without `end_date` → 400; `end_date` without `start_date` → 400 |
| `AC-6` Parameter validation — format | P1 | `period=2026-13` → 400; `period=abcd` → 400; `start_date=2026-13-01` → 400 |
| `AC-7` Parameter validation — reversed | P1 | `start_date > end_date` → 400; `start_date = end_date` → 400 |
| `AC-8` Tenant isolation | P1 | RT A balance does not include RT B transactions |
| `AC-9` Draft/cancelled exclusion | P1 | Transactions with `status = 'draft'` or `status = 'cancelled'` do NOT affect balance |
| `AC-10` Cross-period reversal | P0 | Income in Month 1 + reversal in Month 3: Month 1 closing = X, Month 3 closing = 0 |
| `AC-11` Empty period | P1 | Period with no transactions: all balance fields = "0.00", count = 0 |
| `AC-12` Snapshot consistency | P1 | All 4 balance values (opening, income, expense, closing) from single query snapshot |
| `AC-13` Opening balance auto-propagation | P0 | Period N opening = Period N-1 closing (when source = "auto") |
| `AC-14` Opening balance manual correction | P0 | Manual correction changes opening, closing recalculates with corrected opening |
| `AC-15` Manual correction persists after refresh | P0 | `period_opening_balances` table retains correction data across restarts |
| `AC-16` Manual correction not overwritten | P0 | Period with `source = "manual"` NOT recalculated when earlier period changes |
| `AC-17` Manual correction follows corrected period | P0 | Period with manual correction becomes new base for next period auto-propagation |
| `AC-18` Authorization — warga cannot correct | P1 | Warga role returns 403 on `PUT /reports/balance/opening` |
| `AC-19` Authorization — bendahara/pengurus can correct | P1 | Bendahara/pengurus with valid RT can set opening balance |
| `AC-20` SUPER_ADMIN requires explicit RT | P1 | SUPER_ADMIN must select RT explicitly; cannot use tenant-scoped endpoint |
| `AC-21` No fictitious transactions | P1 | Opening balance correction is stored in `period_opening_balances`, NOT as a `transactions` entry |
| `AC-22` Historical opening balance | P2 | When no historical opening set, first period auto-opens at Rp0 (documented as `BUSINESS_DECISION_REQUIRED`) |
| `AC-23` Reversal in period — same type | P0 | Reversal expense in period reduces total_income net effect correctly |

### 11.3 E2E

| Test | Priority |
|------|:--------:|
| Web KAS API integration | P0 |
| Android KAS API integration | P0 |
| Period balance E2E | P0 |
| Reconciliation: API summary matches UI numbers | P0 |
| Opening balance correction flow (UI → API → DB → API → UI) | P0 |
| Manual correction propagation: korreksi Oktober → saldo November otomatis ikut | P0 |

---

## 12. Design Principles

1. **Iuran = obligation/payment domain.** Iuran tracks who owes what, when, and what has been paid.
2. **KAS = financial ledger.** KAS records actual financial movements. It is a ledger, not a bank balance sheet.
3. **Iuran payment → KAS income must be atomic.** Both succeed or both fail. No partial commits.
4. **Payment history is stored as transaction/record.** Not just a `paid_amount` field on the bill.
5. **KAS transaction must not depend on a mutable balance field as source of truth.** Balance is always calculated from the ledger.
6. **Duplicate posting must be prevented.** Idempotency keys, advisory locks, and application-level checks.
7. **Auditability must be maintained.** All financial records have `created_at`, `updated_at`, `is_active`. No physical DELETE.
8. **Authorization must follow existing authorization architecture.** No parallel permission system for Iuran/KAS.
9. **Business rules must be finalized before schema/migration is created.** Do not design database tables without confirmed business rules.
10. **Tenant isolation is mandatory.** Every query on tenant-scoped data includes `rt_id` filter. `rt_id` is derived from JWT, never from client input.

---

## 13. Explicit Non-Changes

This documentation update **does not change** any of the following:

| Item | Status |
|------|--------|
| Backend source code | Unchanged |
| Database / schema | Unchanged |
| Migrations | Unchanged |
| Repository / service / handler | Unchanged |
| API implementation | Unchanged |
| Web source code | Unchanged |
| Android source code | Unchanged |
| Test code | Unchanged |
| Authorization implementation | Unchanged |

---

*End of Document — Consolidated 2026-10-15*
*For detailed audit evidence, see [KAS Implementation Audit Report](./kas-implementation-audit-report.md)*
