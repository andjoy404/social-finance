# Iuran & KAS Module — Blueprint

> **Status:** UI Phase Complete · Backend Not Started · Iuran Subject Model: Physical House/Occupancy
> **Created:** 2026-10-02
> **Last Updated:** 2026-10-02

---

## 1. Status Dokumen

| Field | Value |
|-------|-------|
| **Status** | Blueprint — UI Phase Complete, Backend Not Started |
| **Created** | 2026-10-02 |
| **Last Updated** | 2026-10-02 |
| **Scope** | Modul Iuran (RT Dues) + Modul KAS (Cash Management) — planning, architecture, blueprint |
| **Non-goals (this document)** | Database schema, migrations, repository, service, handler, API implementation, source code changes |

### Iuran Subject Decision

**Business decision:** Iuran **tidak ditujukan kepada KK/Household**.

Iuran ditujukan kepada: **pemilik atau penyewa rumah**.

This means Iuran bills target **occupancy + house**, not `householdId`/`householdName`.

The existing `households` model may continue to exist as part of the Warga module, but **Iuran must not use `householdId`/`householdName` as its billing subject**. UI labels such as `KK`, `Kepala Keluarga`, or `WARGA/KK` must not be used for Iuran billing subject.

If a technical relationship to households is needed for resolving Warga data, document it as a **technical relationship**, not as a billing subject.

---

## 2. Goals

- Menyediakan struktur iuran yang dapat didefinisikan per RT, dengan jenis iuran, periode, dan nominal yang dapat dikelola.
- Mencatat tagihan iuran kepada **pemilik atau penyewa rumah** dan melacak status pembayarannya.
- Menyediakan modul Kas untuk mencatat pemasukan dan pengeluaran kas RT secara umum.
- Mengintegrasikan pembayaran iuran ke dalam transaksi Kas Masuk tanpa menyebabkan double-entry.
- Memastikan semua transaksi keuangan mengikuti prinsip immutability dan audit trail yang sama dengan Financial Ledger yang sudah ada.

---

## 3. Current Implementation Status

### 3.1 UI Phase — COMPLETE

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

### 3.2 Backend Phase — NOT STARTED

**Nothing has been implemented on the backend:**

- ❌ No database migrations
- ❌ No schema implementation
- ❌ No repository layer
- ❌ No service layer
- ❌ No handler / API routes
- ❌ No authorization integration
- ❌ No frontend API integration (both Web and Android use mock data)

Backend implementation must follow the phases in Section 11. No phase may begin until the business rules required for that phase are finalized.

---

## 3.3 Existing House/Occupancy Model — Evidence from Inspection

The backend already has a complete house/occupancy/owner/tenant model. This is **existing Warga infrastructure**, not new Iuran design.

### Entity Chain

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

### physical_houses

| Field | Type | Notes |
|-------|------|-------|
| `id` | uuid | Primary key |
| `rt_id` | uuid | FK → rts |
| `house_number` | text | Unique per RT (where is_active = true) |
| `address` | text | Free text |
| `is_active` | boolean | Soft delete |

### household_occupancies

| Field | Type | Notes |
|-------|------|-------|
| `id` | uuid | Primary key |
| `physical_house_id` | uuid | FK → physical_houses |
| `household_id` | uuid | FK → households |
| `occupancy_status` | text | `OWNER` or `TENANT` (CHECK constraint) |
| `start_date` | date | Nullable (NULL = legacy unknown start) |
| `end_date` | date | Nullable (NULL = current/ongoing) |

Unique index on `(physical_house_id) WHERE end_date IS NULL` — only one current occupant per physical house.

EXCLUDE constraint using GIST on `(physical_house_id, daterange(start_date, end_date))` — prevents temporal overlap.

### residency_periods

| Field | Type | Notes |
|-------|------|-------|
| `id` | uuid | Primary key |
| `resident_id` | uuid | FK → residents |
| `household_occupancy_id` | uuid | FK → household_occupancies |
| `relationship_to_head` | text | Head/Spouse/Child/etc. |
| `start_date` | date | Nullable |
| `end_date` | date | Nullable (NULL = current) |

Unique index on `(resident_id) WHERE end_date IS NULL` — one current residency per resident.

### Key Finding: Simultaneous Owner + Tenant is NOT Supported

The EXCLUDE constraint prevents two household occupancies from overlapping on the same physical house. Therefore:

```
House A1
 ├── OWNER → current ← NOT POSSIBLE simultaneously
 └── TENANT → current ← NOT POSSIBLE simultaneously
```

Only sequential (non-overlapping) periods are supported:

```
House A1
 ├── OWNER → 2020-01-01 → 2024-01-01
 └── TENANT → 2024-01-01 → current
```

This is **a database constraint decision**, not a business assumption. Any future need for simultaneous owner+tenant occupancy requires changing the EXCLUDE constraint.

---

## 3.4 Current Iuran Mock Model — Legacy (To Be Replaced)

The current Web Iuran mock uses a **household-based billing subject**. This is documented here as legacy only and **must be replaced** when backend implementation begins.

### Legacy IuranBill Type

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

### Legacy Mock Data Structure

```typescript
const HOUSEHOLDS = [
  { householdId: 'h001', name: 'Budi Santoso', rt: 'RT 03' },
  // ... 10 households total
]
```

### Legacy UI Labels (To Be Replaced)

| Location | Current Label | Replacement |
|----------|--------------|-------------|
| `IuranList.tsx` table header | `WARGA/KK` | House/Occupant name |
| `IuranPayment.tsx` form label | `Informasi Warga/KK` | House/Occupant info |
| All bill references | `householdName` | Occupant/house name |

These labels and fields are **prototype artifacts**. They must not be carried forward into the final Iuran design.

---

## 3.5 KAS UI Design Decisions

### Web — Git Diff Style Split Ledger

KAS Web uses a two-column split ledger presentation:

```
┌──────────────────────────────┬──────────────────────────────┐
│ + 02 Okt 2026                │ - 02 Okt 2026                │
│   Iuran Keamanan             │   Pembelian Lampu            │
│   Rp 500.000                 │   Rp 350.000                 │
├──────────────────────────────┼──────────────────────────────┤
│ + 01 Okt 2026                │                              │
│   Donasi                     │                              │
│   Rp 1.000.000               │                              │
├──────────────────────────────┼──────────────────────────────┤
│                              │ - 30 Sep 2026                │
│                              │   Perbaikan Pos              │
│                              │   Rp 750.000                 │
└──────────────────────────────┴──────────────────────────────┘
```

**Rules:**
- Left column = Kas Masuk (income)
- Right column = Kas Keluar (expense)
- Both columns are independent vertical streams
- Entries do NOT need to be aligned horizontally
- Do NOT create dummy/blank transactions just to align rows
- Vertical position represents order within each stream only
- Latest transaction is at the top
- Split is a **presentation layer only**
- Backend remains a single `kas_transactions` ledger

### Android — Single Chronological List

KAS Android uses a single chronological list due to screen width constraints:

```
02 Okt 2026  Kas Masuk   + Rp500.000
02 Okt 2026  Kas Keluar  - Rp350.000
01 Okt 2026  Kas Masuk   + Rp1.000.000
30 Sep 2026  Kas Keluar  - Rp750.000
```

**Rules:**
- All transactions in one list
- Global order: newest → oldest
- Do NOT group "Kas Masuk" separately from "Kas Keluar"
- No two-column split on Android

---

## 4. Backend Architecture Plan

### 4.1 Layered Architecture

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

### 4.2 Iuran Payment → KAS Integration Flow

```
POST /api/v1/iuran/payments
        ↓
   Handler (validation, auth)
        ↓
   IuranService.CreatePayment()
        ↓
   BEGIN TRANSACTION
        ↓
   Create Iuran Payment record
   ↓
   Create KAS Income record (reference_id → payment)
   ↓
   COMMIT
```

**If any operation fails:**

```
   ROLLBACK
```

This prevents conditions such as:
- Iuran marked as paid but KAS balance unchanged
- KAS increased but Iuran payment not recorded

### 4.3 Idempotency

Critical write endpoints accept an `Idempotency-Key` header. Duplicate keys within the configured window return the original response without re-executing the operation.

---

## 5. Iuran Domain Design

> **Candidate design only. Not a schema. Not final.**

### 5.1 Iuran Type / Master

Defines the recurring types of dues an RT collects.

| Field | Type | Notes |
|-------|------|-------|
| `id` | uuid | Primary key |
| `rt_id` | uuid | Tenant isolation |
| `name` | text | E.g., "Iuran Keamanan" |
| `description` | text | Optional |
| `default_amount` | numeric(15,2) | Default nominal |
| `frequency` | text | E.g., "monthly" |
| `is_active` | boolean | Soft delete |
| `created_at` | timestamptz | — |
| `updated_at` | timestamptz | — |

### 5.2 Iuran Bill — Billing Subject

> **Business decision:** Iuran bills target **pemilik atau penyewa rumah** (owner or tenant), not household/KK.

The billing subject chain:

```
physical_house → household_occupancy (OWNER/TENANT) → residents
```

| Field | Type | Notes |
|-------|------|-------|
| `id` | uuid | Primary key |
| `rt_id` | uuid | Tenant isolation |
| `iuran_type_id` | uuid | FK → iuran_type |
| `period` | text | E.g., "2026-10" |
| `amount` | numeric(15,2) | Always positive |
| `paid_amount` | numeric(15,2) | Running total of payments |
| `status` | text | `unpaid`, `partial`, `paid`, `cancelled` |
| `due_date` | date | Optional |
| `created_at` | timestamptz | — |
| `updated_at` | timestamptz | — |

**Foreign key decision TBD:**
The bill must reference the billing subject. Potential references:

| Option | FK | Pros | Cons |
|--------|-----|------|------|
| A | `physical_house_id` | Stable across household moves | House may be vacant |
| B | `household_occupancy_id` | Includes ownership status | New table needed |
| C | Other | TBD | TBD |

This is a business rule decision. Do NOT choose until BR-01 is finalized.

**What the bill represents:** A bill targets a **house occupancy** (owner or tenant), not a household per se. Even if the same household moves houses, the bill follows the occupancy, not the household.

### 5.3 Iuran Payment

A payment record referencing one or more bills. Payment history must be stored as separate records, not just via `paid_amount` on the bill.

| Field | Type | Notes |
|-------|------|-------|
| `id` | uuid | Primary key |
| `rt_id` | uuid | Tenant isolation |
| `bill_id` | uuid | FK → iuran_bill |
| `amount` | numeric(15,2) | Always positive |
| `paid_at` | timestamptz | When payment was recorded |
| `payment_method` | text | E.g., "cash", "transfer" |
| `reference` | text | External reference (bank ref, etc.) |
| `notes` | text | Optional notes |
| `created_by` | uuid | FK → users |
| `created_at` | timestamptz | — |

---

## 6. KAS Domain Design

> **Candidate design only. Not a schema. Not final.**

KAS is designed as a **transaction ledger**, not a mutable balance field.

### 6.1 KAS Transaction

| Field | Type | Notes |
|-------|------|-------|
| `id` | uuid | Primary key |
| `rt_id` | uuid | Tenant isolation |
| `jenis` | text | `income`, `expense` |
| `category_id` | uuid | FK → kas_category |
| `amount` | numeric(15,2) | Always positive |
| `transaction_date` | timestamptz | — |
| `description` | text | — |
| `reference` | text | External reference |
| `source_type` | text | `IURAN_PAYMENT`, `MANUAL`, etc. |
| `source_id` | uuid | Reference ID from source, nullable |
| `created_by` | uuid | FK → users |
| `created_at` | timestamptz | — |

**Jenis values:**
- `income` — pemasukan kas
- `expense` — pengeluaran kas

### 6.2 Balance Calculation (Conceptual)

```
Saldo = SUM(jenis = 'income' ∧ rt_id = ?) − SUM(jenis = 'expense' ∧ rt_id = ?)
```

Balance is **never stored** as a derived value. It is always calculated from the ledger.

### 6.3 KAS Transaction Sources

KAS transactions can come from:

**Automatic source:**
- Iuran Payment (auto-created when a payment is recorded)

**Manual source:**
- Donations, contributions
- Interest
- Any other income transactions
- Office supplies (ATK)
- Cleanliness/maintenance
- Security
- Repairs
- Any other expense transactions

**Not all KAS transactions originate from Iuran.** KAS is a general-purpose financial ledger.

---

## 7. Iuran → KAS Integration

### 7.1 Principle

Every successful Iuran payment must produce exactly one KAS `income` transaction.

### 7.2 Source/Reference Relationship

```
source_type = 'IURAN_PAYMENT'
source_id   = <payment_id>
```

A database or application-level mechanism must guarantee that one payment cannot produce two KAS income records.

### 7.3 Idempotency / Duplicate Posting Prevention

```
Iuran Payment ABC
       ↓
KAS Income ABC (source_id = ABC)

Retry request for same payment
       ↓
No second KAS Income created
```

Mechanisms to prevent duplicate posting:
- Unique constraint on `source_type + source_id` in `kas_transaction`
- `Idempotency-Key` header on payment endpoints
- Application-level check before creating KAS income from payment

---

## 8. Business Rules To Be Finalized

> **Status: OPEN / TBD**
>
> Do NOT make assumptions or choose answers for these decisions. These must be finalized before database schema and migration work begins.

### BR-01 — Billing Subject

**Question:** Is Iuran billed per household (KK) or per individual resident?

| Option | Description |
|--------|-------------|
| A | Per KK — one bill per household |
| B | Per warga — one bill per resident |

### BR-02 — Nominal

**Question:** Is the nominal fixed or can it differ per household?

| Option | Description |
|--------|-------------|
| A | Fixed nominal per iuran type (same for all households) |
| B | Variable nominal — can differ per household |

### BR-03 — Partial Payment

**Question:** Is partial payment allowed for a single bill?

```
Bill        = Rp50,000
Payment 1   = Rp20,000
Remaining   = Rp30,000
Status      = PARTIAL
```

| Option | Description |
|--------|-------------|
| A | Allowed — bill status transitions: `unpaid` → `partial` → `paid` |
| B | Not allowed — must pay full amount |

### BR-04 — Multi-Period Payment

**Question:** Can a single payment cover multiple periods at once?

| Option | Description |
|--------|-------------|
| A | Allowed — one payment can pay multiple bills across periods |
| B | Not allowed — one payment per bill |

### BR-05 — Cancellation / Correction

**Question:** How to handle cancellation or correction of an Iuran payment?

| Option | Description |
|--------|-------------|
| A | Reversal transaction (new entry, opposite sign, references original) |
| B | Direct edit/delete of original (only if not yet posted) |

### BR-06 — KAS Correction

**Question:** Can an existing KAS transaction be edited, or must it use a reversal/correction transaction?

| Option | Description |
|--------|-------------|
| A | Immutable after posting — corrections via reversal only |
| B | Editable if status is "draft", immutable if "posted" |

### BR-07 — Authorization

**Question:** Who is authorized to:
- View Iuran module?
- Create/modify Iuran master?
- Create bills?
- Record payments?
- View arrears?
- View KAS?
- Create income/expense transactions?
- Correct/reverse transactions?
- View reports?

**Authorization must follow the existing permission architecture**, but the specific Iuran/KAS permission matrix is TBD.

---

## 9. Proposed Backend Implementation Phases

> **Phase ordering is mandatory.** No phase may begin until the business rules required for that phase are finalized.

| Phase | Name | Description | Blocked By |
|-------|------|-------------|------------|
| **BE-01** | Business rules finalization | Document final decisions for BR-01 through BR-07. Stakeholder approval. | — |
| **BE-02** | Existing DB/schema/architecture inspection | Audit current database, schema, and backend architecture for compatibility with Iuran/KAS. | BE-01 |
| **BE-03** | Database migration/schema | Write migration files (up/down) for `iuran_type`, `iuran_bill`, `iuran_payment`, `kas_transaction`, `kas_category`. | BE-02 |
| **BE-04** | Models + repositories | Go domain structs. Repository methods for CRUD. | BE-03 |
| **BE-05** | Iuran service | Business logic for Iuran: bill generation, payment recording, arrears calculation. | BE-04 |
| **BE-06** | KAS service | Business logic for KAS: income/expense recording, balance calculation, reports. | BE-04 |
| **BE-07** | Iuran → KAS atomic integration | Transactional flow: payment → KAS income. Idempotency enforcement. | BE-05, BE-06 |
| **BE-08** | Authorization | Permission codes for Iuran/KAS actions. Middleware enforcement. | BE-01, BE-07 |
| **BE-09** | API handlers/routes | HTTP handlers, validation, error handling, pagination. | BE-05, BE-06, BE-07, BE-08 |
| **BE-10** | Backend automated tests | Unit tests for service layer. Integration tests for API endpoints. | BE-09 |
| **BE-11** | Web API integration | Replace mock data with real API calls in Web Iuran/KAS screens. | BE-10 |
| **BE-12** | Android API integration | Replace mock data with real API calls in Android Iuran/KAS screens. | BE-10 |
| **BE-13** | E2E validation | Full flow: create iuran type → generate bills → pay → KAS income → reports. Tenant isolation. Authorization gates. Idempotency. | BE-11, BE-12 |
| **BE-14** | Final checkpoint/audit | Compare implementation vs. blueprint. Commit checkpoint. Update documentation. | BE-13 |

---

## 10. API Draft — DRAFT / NOT FINAL

> All endpoints below are **conceptual proposals only**. Not implemented. Subject to change based on business rules (Section 8).

### 10.1 Iuran

```
GET    /api/v1/rt/:id/iuran/types          — List Iuran types per RT
POST   /api/v1/rt/:id/iuran/types          — Create Iuran type
PATCH  /api/v1/rt/:id/iuran/types/{id}     — Update Iuran type
DELETE /api/v1/rt/:id/iuran/types/{id}     — Deactivate Iuran type

GET    /api/v1/rt/:id/iuran/bills          — List bills (filter: period, status)
GET    /api/v1/rt/:id/iuran/bills/{id}     — Bill detail
POST   /api/v1/rt/:id/iuran/bills/generate — Generate bills (bulk)

GET    /api/v1/rt/:id/iuran/payments       — List payments
POST   /api/v1/rt/:id/iuran/payments       — Record payment (idempotent)

GET    /api/v1/rt/:id/iuran/arrears        — Overdue bills
GET    /api/v1/rt/:id/iuran/reports        — Iuran summary/report
```

### 10.2 KAS

```
GET    /api/v1/rt/:id/kas/categories       — List KAS categories
POST   /api/v1/rt/:id/kas/categories       — Create KAS category

GET    /api/v1/rt/:id/kas/transactions     — List transactions
GET    /api/v1/rt/:id/kas/transactions/{id} — Transaction detail

POST   /api/v1/rt/:id/kas/income           — Record income
POST   /api/v1/rt/:id/kas/expense          — Record expense

GET    /api/v1/rt/:id/kas/balance          — Current balance (calculated)
GET    /api/v1/rt/:id/kas/reports          — KAS summary/report
```

### 10.3 Notes

- All endpoints require `rt_id` from JWT claims (tenant isolation).
- SUPER_ADMIN endpoints do not require `rt_id` in JWT, but `rt_id` as path parameter.
- Payment endpoint accepts `Idempotency-Key` header to prevent duplicates.
- All KAS transactions are immutable after posting. Corrections use reversal.

---

## 11. Testing Strategy

> Target test cases for backend implementation. Not implemented yet.

### 11.1 Iuran

| Test | Description |
|------|-------------|
| Create Iuran Type | Valid create returns 201 |
| Duplicate Iuran Type | Same name+rt rejected |
| Create Bill | Valid bill creation |
| Duplicate Bill | Same type+household+period rejected |
| Payment | Payment records, bill status updates |
| Partial Payment | Bill transitions to PARTIAL |
| Full Payment | Bill transitions to PAID |
| Overpayment | Behavior when payment > bill amount |
| Cancellation/Correction | Reversal preserves audit trail |

### 11.2 KAS

| Test | Description |
|------|-------------|
| Income | Valid income recording |
| Expense | Valid expense recording |
| Balance | Calculated balance matches ledger |
| Invalid Amount | Zero/negative amount rejected |
| Invalid Category | Unknown category rejected |
| Correction/Reversal | Reversal creates opposite entry |

### 11.3 Integration

| Test | Description |
|------|-------------|
| Payment Creates KAS Income | One payment → exactly one KAS income |
| Retry No Duplicate | Duplicate payment request does not create second KAS income |
| KAS Failure Rolls Back | If KAS creation fails, Iuran payment also rolled back |
| Atomic Transaction | Payment + KAS income are in same DB transaction |

---

## 12. Important Design Principles

1. **Iuran = obligation/payment domain.** Iuran tracks who owes what, when, and what has been paid.
2. **KAS = financial ledger.** KAS records actual financial movements. It is a ledger, not a bank balance sheet.
3. **Iuran payment → KAS income must be atomic.** Both succeed or both fail. No partial commits.
4. **Payment history is stored as transaction/record.** Not just a `paid_amount` field on the bill.
5. **KAS transaction must not depend on a mutable balance field as source of truth.** Balance is always calculated from the ledger.
6. **Duplicate posting must be prevented.** Unique constraints, idempotency keys, and application-level checks.
7. **Auditability must be maintained.** All financial records have `created_at`, `updated_at`, `is_active`. No physical DELETE.
8. **Authorization must follow existing authorization architecture.** No parallel permission system for Iuran/KAS.
9. **Business rules must be finalized before schema/migration is created.** Do not design database tables without confirmed business rules.
10. **Tenant isolation is mandatory.** Every query on tenant-scoped data includes `rt_id` filter. `rt_id` is derived from JWT, never from client input.

---

## 13. Existing Content Preserved

The following sections from the original planning document have been retained where relevant:

- **Section 2 (Goals):** Unchanged — still valid.
- **Section 6 (Authorization):** Existing principles retained. Specific permission matrix still TBD (see BR-07).
- **Section 7 (Candidate Domain Model):** Retained as reference. Replaced by more detailed domain designs in Sections 5-6.
- **Section 9 (Web & Android Navigation):** Retained as reference.
- **Section 12 (Risks & Design Considerations):** Retained as reference.

---

## 14. Explicit Non-Changes

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

**This is a planning and blueprint document. All content is candidate and not final.**

---

*End of Document*
