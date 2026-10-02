# Phase A — Iuran Business Rules Discovery

> **Created:** 2026-10-02  
> **Status:** Discovery Complete — Business Rules Pending Finalization  
> **Scope:** Documentation-only output. NO code changes, NO schema changes.

---

## 1. Current Evidence

### 1.1 FACT — From Database Schema

| # | Evidence | Source |
|---|----------|--------|
| E-DB-01 | `physical_houses` table exists with `rt_id`, `house_number`, `address`, `is_active` | `migrations/008_temporal_household_model.up.sql` |
| E-DB-02 | `household_occupancies` has `occupancy_status` with CHECK constraint: `OWNER` or `TENANT` only | Same |
| E-DB-03 | EXCLUDE constraint using GIST on `(physical_house_id, daterange(start_date, end_date))` prevents overlapping occupancies on same physical house | Same |
| E-DB-04 | UNIQUE `(physical_house_id) WHERE end_date IS NULL` — only one current occupant per house | Same |
| E-DB-05 | Temporal model uses half-open intervals `[start_date, end_date)`, NULL end_date = current | Same |
| E-DB-06 | Migration 008 drops `house_number`, `address`, `occupancy_status` from `households` table | Same |
| E-DB-07 | `residency_periods` links `resident_id` → `household_occupancy_id` with temporal support | Same |
| E-DB-08 | Legacy migration bridge tables preserve provenance for rollback (`migration_008_*`) | Same |

### 1.2 FACT — From Backend Code

| # | Evidence | Source |
|---|----------|--------|
| E-BE-01 | Go types: `OccupancyStatus`, `OccupancyOwner`, `OccupancyTenant` defined in `household/model.go` | `backend/internal/household/model.go` |
| E-BE-02 | `Household` struct reconstructed via LEFT JOINs through `physical_house_id` → `household_occupancies` → `residency_periods` | Same |
| E-BE-03 | `MoveHouseholdInput` supports occupancy status change + start date for temporal chain | Same |
| E-BE-04 | Position enum: `keamanan`, `sosial`, `kebersihan_pembangunan` for Pengurus | `backend/internal/auth/models.go` |
| E-BE-05 | PermissionResolver queries `position_permissions` → `permission_types` for jabatan-based access | `backend/internal/auth/permission.go` |
| E-BE-06 | `AuthContext` carries `UserID`, `RTID`, `Jabatan` from JWT — never from client input | Same |

### 1.3 FACT — From Web Frontend

| # | Evidence | Source |
|---|----------|--------|
| E-WB-01 | Iuran types include `householdId`, `householdName` in mock data | `frontend/src/features/iuran/iuranTypes.ts` |
| E-WB-02 | Mock data uses `HOUSEHOLDS` array with `householdId`, `name` as billing subject | `frontend/src/features/iuran/iuranMockData.ts` |
| E-WB-03 | Iuran routes: list, detail, payment, arrears, report (5 routes) | `frontend/src/features/iuran/iuranRoutes.tsx` |
| E-WB-04 | Sidebar includes Iuran route (`/iuran`) with `AccountBookOutlined` icon | `frontend/src/components/Sidebar.tsx` |
| E-WB-05 | Web warga models use `occupancy_status: 'OWNER' | 'TENANT' | null` in API | `frontend/src/app/api.ts` |

### 1.4 FACT — From Android Frontend

| # | Evidence | Source |
|---|----------|--------|
| E-AD-01 | `BackendHousehold` has `occupancyStatus: 'OWNER' | 'TENANT'` field | `mobile/lib/features/warga/data/api_warga_models.dart` |
| E-AD-02 | `MappedResident` maps `occupancyStatus` to display values `'Pemilik' | 'Penyewa'` | Same |
| E-AD-03 | Bottom bar tabs now navigate to `/home/kas` and `/home/iuran` (path mismatch fixed) | `mobile/lib/app/navigation_shell.dart` |
| E-AD-04 | Iuran + KAS screens fully implemented in Android (UI Phase Complete) | Inspection finding |

### 1.5 ASSUMPTION — To Be Confirmed

| # | Assumption | How To Verify |
|---|------------|---------------|
| A-01 | Iuran nominal is expected to be configurable per RT, not globally fixed | Stakeholder interview |
| A-02 | Iuran bills are expected to be generated monthly | Stakeholder interview |
| A-03 | Partial payments are expected to be supported | Stakeholder interview |
| A-04 | Overpayments should be handled as credit for future periods | Stakeholder interview |
| A-05 | House vacant handling — no bills generated | Stakeholder interview |
| A-06 | Owner and tenant may have different Iuran obligations (e.g., owner pays infrastructure, tenant pays security) | Stakeholder interview |

---

## 2. Proposed Business Rules

### BR-01 — Billing Subject

| Property | Value |
|----------|-------|
| **Rule** | Iuran bills target **pemilik atau penyewa rumah** (owner or tenant), not household/KK. |
| **Evidence** | E-DB-02, E-DB-03, E-BE-01, E-WB-05, E-AD-01 — the entire temporal model is built around `physical_house` → `household_occupancy` with occupancy_status |
| **Confidence** | **HIGH** — this is a confirmed business decision, not an assumption |

### BR-02 — Billing Subject Reference (FK Decision)

| Property | Value |
|----------|-------|
| **Rule** | Bill must reference the billing subject via a foreign key. Two options are technically feasible: |
| **Option A** | `physical_house_id` — stable across household moves; house never changes identity |
| **Option B** | `household_occupancy_id` — captures specific owner/tenant period; historically accurate |
| **Evidence** | E-DB-03 (EXCLUDE prevents simultaneous), E-DB-05 (temporal chains), E-BE-03 (MoveHouseholdInput changes occupancy) |
| **Confidence** | **HIGH** (Option A vs B is a trade-off decision) |
| **Recommendation** | **Option B (`household_occupancy_id`)** — bills track the obligation of a specific occupancy. When an occupant moves, the historical bill follows the occupancy (not the house). This is more accurate for financial auditing. |
| **Trade-off** | Option B requires a new `iuran_bill_occupancy_snapshot` or `iuran_bill.occupancy_id` table. Option A is simpler but loses the owner/tenant distinction in historical records. |

### BR-03 — Billing Subject: Simultaneous Owner + Tenant

| Property | Value |
|----------|-------|
| **Rule** | The database does NOT support simultaneous owner + tenant on the same physical house. The EXCLUDE constraint prevents this. |
| **Evidence** | E-DB-03 (EXCLUDE using GIST), E-DB-04 (UNIQUE constraint) |
| **Confidence** | **HIGH** — this is a database constraint, not a business assumption |
| **Action** | If business requires simultaneous occupancy in the future, the EXCLUDE constraint must be removed or modified. |

### BR-04 — Billing Period

| Property | Value |
|----------|-------|
| **Rule** | Iuran billing period is defined by a time range (e.g., `2026-10` for October). Bills are generated per period per active occupancy. |
| **Evidence** | Existing Iuran mock uses `periode: string` (e.g., `"2026-10"`) |
| **Confidence** | **MEDIUM** — format (YYYY-MM vs date range) TBD |

### BR-05 — Mid-Month Moves: Historical Bills Do Not Change

| Property | Value |
|----------|-------|
| **Rule** | When an occupant moves mid-month, bills for PAST periods remain immutable. Only FUTURE bills are affected: the new occupancy gets billed, the old occupancy's past bills are final. |
| **Evidence** | E-DB-05 (temporal model with `[start_date, end_date)`), E-BE-03 (`MoveHouseholdInput` with `start_date`) |
| **Confidence** | **HIGH** — this follows from the temporal model + financial immutability principles in AGENTS.md |

### BR-06 — Prorata / Partial Period Bills

| Property | Value |
|----------|-------|
| **Rule** | TBD — is prorata supported for mid-month moves? |
| **Options** | (A) No prorata — full month bill regardless of move date. (B) Prorata — bill proportional to days occupied. |
| **Confidence** | **LOW** — requires stakeholder input |

### BR-07 — Iuran Type

| Property | Value |
|----------|-------|
| **Rule** | Iuran types are defined per RT (e.g., "Iuran Keamanan", "Iuran Infrastruktur"). Each type has a default amount. |
| **Evidence** | Candidate design in `docs/iuran-kas-module.md` Section 5.1 |
| **Confidence** | **HIGH** — this is the most common pattern for RT dues |
| **Sub-rule** | Owner and tenant may have DIFFERENT nominal amounts for the SAME iuran type. |

### BR-08 — Payment: Partial Payments Allowed

| Property | Value |
|----------|-------|
| **Rule** | Partial payments are allowed. Bill status transitions: `unpaid` → `partial` → `paid`. |
| **Confidence** | **LOW** — requires stakeholder input |
| **Default Recommendation** | Allow partial payments. This is the standard practice for RT dues. |

### BR-09 — Payment: Multiple Payments Per Bill

| Property | Value |
|----------|-------|
| **Rule** | Multiple payments can be recorded against a single bill. `paid_amount` on the bill is a running total. |
| **Evidence** | Financial immutability principle in AGENTS.md Section 4.3 |
| **Confidence** | **HIGH** — required by financial immutability |

### BR-10 — Payment: Overpayment Handling

| Property | Value |
|----------|-------|
| **Rule** | TBD — should overpayment be (A) a credit for future periods, (B) refunded, or (C) recorded as positive balance? |
| **Confidence** | **LOW** — requires stakeholder input |

### BR-11 — Cancellation / Reversal

| Property | Value |
|----------|-------|
| **Rule** | Posted financial transactions are IMMUTABLE. Corrections use **reversal transactions** (opposite sign, references original). |
| **Evidence** | AGENTS.md Section 4.3 (Ledger Immutability), Section 4.4 (Transaction lifecycle: draft/posted/cancelled) |
| **Confidence** | **HIGH** — this is a platform-wide policy |

### BR-12 — Iuran → KAS Integration: Atomicity

| Property | Value |
|----------|-------|
| **Rule** | Every Iuran payment MUST create exactly one KAS income transaction in the SAME database transaction. Both succeed or both fail. |
| **Evidence** | `docs/iuran-kas-module.md` Section 7 |
| **Confidence** | **HIGH** — this is a design principle in the blueprint |

### BR-13 — Iuran → KAS: Idempotency

| Property | Value |
|----------|-------|
| **Rule** | Duplicate payment requests (via `Idempotency-Key` header or same payment ID) MUST NOT create duplicate KAS income records. |
| **Evidence** | `docs/iuran-kas-module.md` Section 7.3, AGENTS.md Section 7.3 (Idempotency) |
| **Confidence** | **HIGH** — platform-wide requirement |

### BR-14 — State Machine: Bill Lifecycle

| Property | Value |
|----------|-------|
| **Rule** | Valid transitions: |
| **States** | `generated` → `unpaid` → `partial` → `paid` |
| **Reversal** | `paid` → `reversed` (via reversal transaction) |
| **Cancellation** | `generated` / `unpaid` → `cancelled` (only if no payments recorded) |
| **Confidence** | **MEDIUM** — standard pattern, details TBD |

### BR-15 — State Machine: Payment Lifecycle

| Property | Value |
|----------|-------|
| **Rule** | Valid transitions: |
| **States** | `pending` → `posted` / `pending` → `reversed` |
| **Reversal** | `posted` → `reversed` (via reversal transaction) |
| **Confidence** | **MEDIUM** — follows AGENTS.md Section 4.4 |

### BR-16 — Authorization: Mapping to Existing Positions

| Position | Iuran Access | KAS Access |
|----------|-------------|-----------|
| `super_admin` | Platform-wide view | Platform-wide view |
| `ketua` | Full (create, modify, generate bills) | Full (create, reverse transactions) |
| `wakil_ketua` | Full (same as ketua) | Full (same as ketua) |
| `sekretaris` | Generate bills (view all) | View only |
| `bendahara` | Full (payments, bills, arrears) | Full (income, expense, reversal) |
| `keamanan` | View only (type: "Keamanan") | View only |
| `sosial` | View only | View only |
| `kebersihan_pembangunan` | View only (type: "Kebersihan") | View only |
| `warga` | View own bills, pay | View own bills |

**Confidence: LOW** — This matrix is a PROPOSAL. The actual authorization for Iuran/KAS must be defined in the `position_permissions` table and documented in the final spec.

### BR-17 — KAS Ledger: Single Source of Truth

| Property | Value |
|----------|-------|
| **Rule** | KAS uses a single `kas_transactions` ledger. Web split-ledger (income left, expense right) and Android single-list are **PRESENTATION LAYER ONLY**. The backend does NOT distinguish income/expense ledgers. |
| **Evidence** | `docs/iuran-kas-module.md` Section 3.5, Section 6.2 |
| **Confidence** | **HIGH** — confirmed design decision |

### BR-18 — KAS Transaction Sources

| Property | Value |
|----------|-------|
| **Rule** | KAS transactions have a `source_type` + `source_id` field: |
| **Sources** | `IURAN_PAYMENT` (auto-created from Iuran), `MANUAL` (manual entry by Pengurus) |
| **Evidence** | `docs/iuran-kas-module.md` Section 6.1 |
| **Confidence** | **HIGH** — confirmed design decision |

### BR-19 — House Vacant: Bill Generation

| Property | Value |
|----------|-------|
| **Rule** | TBD — should bills be generated for houses with no current occupancy? |
| **Options** | (A) No bills for vacant houses. (B) Bills continue (owner still responsible). |
| **Confidence** | **LOW** — requires stakeholder input |
| **Recommendation** | **(B)** — the property still exists; if ownership doesn't change, the owner's obligation continues. But this is a business decision. |

### BR-20 — Historical Integrity

| Property | Value |
|----------|-------|
| **Rule** | Bills for PAST periods must NEVER change when occupant changes. The bill follows the occupancy, not the household. |
| **Evidence** | AGENTS.md Section 4.3 (Ledger Immutability), BR-05 |
| **Confidence** | **HIGH** — platform-wide policy + temporal model |

---

## 3. Decisions Required From User

### DR-01 — Bill FK Reference (BR-02)

**Decision:** Should `iuran_bill` reference `physical_house_id` (Option A) or `household_occupancy_id` (Option B)?

| Option | Pros | Cons |
|--------|------|------|
| A: `physical_house_id` | Simpler schema; house is stable | Loses owner/tenant distinction in historical records |
| B: `household_occupancy_id` | Historically accurate; tracks who owes what | Requires new table or snapshot; slightly more complex |

**Recommendation:** Option B for audit accuracy.

---

### DR-02 — Prorata for Mid-Month Moves (BR-06)

**Decision:** When an occupant moves mid-month, should bills be prorated?

| Option | Description |
|--------|-------------|
| A | No prorata — full month bill regardless of move date |
| B | Prorata — bill proportional to days occupied in the period |

---

### DR-03 — Overpayment Handling (BR-10)

**Decision:** What happens when a payment exceeds the bill amount?

| Option | Description |
|--------|-------------|
| A | Credit for future periods (automatic carry-forward) |
| B | Recorded as positive balance (manual refund or credit) |
| C | Not allowed — system rejects payments exceeding bill balance |

---

### DR-04 — Partial Payments Allowed (BR-08)

**Decision:** Should partial payments be allowed for a single bill?

| Option | Description |
|--------|-------------|
| A | Yes — `unpaid` → `partial` → `paid` transitions |
| B | No — must pay full amount per bill |

**Recommendation:** Option A. Partial payments are standard for RT dues.

---

### DR-05 — House Vacant Handling (BR-19)

**Decision:** Should Iuran bills be generated for houses with no current occupancy?

| Option | Description |
|--------|-------------|
| A | No bills — no occupant = no obligation |
| B | Bills continue — owner's obligation persists regardless of vacancy |

**Recommendation:** Option B. Property ownership doesn't change just because a house is temporarily vacant.

---

### DR-06 — Billing Period Format

**Decision:** What is the billing period format?

| Option | Description |
|--------|-------------|
| A | `YYYY-MM` (e.g., `"2026-10"` for October 2026) |
| B | Date range (e.g., `"2026-10-01/2026-10-31"`) |
| C | Custom period with start/end dates |

**Recommendation:** Option A. Simple, standard, easy to filter/sort.

---

### DR-07 — Iuran Type Amount: Fixed vs Variable

**Decision:** Can the same Iuran type have DIFFERENT amounts for different households/occupants?

| Option | Description |
|--------|-------------|
| A | Fixed — same nominal for all occupants per Iuran type |
| B | Variable — owner pays X, tenant pays Y, or per-household negotiation |
| C | Default + override — default per type, but bendahara can override per bill |

---

### DR-08 — Simultaneous Owner + Tenant

**Decision:** Should the EXCLUDE constraint be modified to allow simultaneous owner AND tenant on the same physical house?

| Option | Description |
|--------|-------------|
| A | No — keep current constraint (sequential occupancy only) |
| B | Yes — remove or modify EXCLUDE to allow simultaneous occupancies |

**Impact:** Option B requires schema change. Option A means the business accepts only one occupant per house at a time.

---

### DR-09 — Authorization Matrix (BR-16)

**Decision:** The proposed authorization matrix in BR-16 is a proposal. Should it be adopted, modified, or entirely redesigned?

**Note:** This requires mapping to the existing `position_permissions` table in the database.

---

### DR-10 — Payment → KAS: Timing

**Decision:** When should the KAS income transaction be created?

| Option | Description |
|--------|-------------|
| A | Immediately on payment recording (atomic within same transaction) |
| B | Batch — end-of-day cron job creates KAS entries for posted payments |

**Recommendation:** Option A. Atomic = safer, aligns with AGENTS.md Section 7.3.

---

## 4. Recommended Entity Relationship

```
┌─────────────────┐       ┌──────────────────────────┐       ┌─────────────────┐
│  iuran_type      │       │  iuran_bill              │       │  iuran_payment   │
├─────────────────┤       ├──────────────────────────┤       ├─────────────────┤
│ id (PK)         │───┐   │ id (PK)                  │   ┌───│ id (PK)         │
│ rt_id (FK)      │   │   │ rt_id (FK)               │   │   │ rt_id (FK)      │
│ name            │   └──>│ iuran_type_id (FK)       │───┘   │ bill_id (FK)    │
│ default_amount  │       │ household_occupancy_id   │       │ amount          │
│ frequency       │       │   (FK → household_       │       │ paid_at         │
│ is_active       │       │    occupancies)          │       │ payment_method  │
│ created_at      │       │ period (YYYY-MM)         │       │ reference       │
│ updated_at      │       │ amount (numeric 15,2)    │       │ notes           │
└─────────────────┘       │ paid_amount (numeric)    │       │ created_by (FK) │
                          │ status                   │       │ created_at      │
                          │ due_date               └───────│ updated_at      │
                          │ created_at              └───────└─────────────────┘
                          │ updated_at
└───────────────────────────────────────────────────────────────────────────┘
                          │
                          │  (references existing table)
                          ▼
              ┌──────────────────────────┐
              │  household_occupancies    │  (EXISTING, not new)
              ├──────────────────────────┤
              │ id (PK)                  │
              │ physical_house_id (FK)   │
              │ household_id (FK)        │
              │ occupancy_status         │
              │ start_date               │
              │ end_date                 │
              └──────────────────────────┘


┌───────────────────────────────────────────────────────────────────────────┐
│  kas_transaction                          │ (EXISTING ledger pattern)
├───────────────────────────────────────────┤
│ id (PK)                                   │
│ rt_id (FK)                                │
│ jenis (income/expense)                    │
│ category_id (FK → kas_category)           │
│ amount (numeric 15,2)                     │
│ transaction_date                          │
│ description                               │
│ source_type (IURAN_PAYMENT / MANUAL)      │
│ source_id (FK → iuran_payment.id if IURAN)│
│ created_by (FK → users)                   │
│ created_at                                │
└───────────────────────────────────────────┘
```

### New Tables Required

| Table | Columns | Purpose |
|-------|---------|---------|
| `iuran_type` | id, rt_id, name, description, default_amount, frequency, is_active, created_at, updated_at | Master: defines recurring Iuran types per RT |
| `iuran_bill` | id, rt_id, iuran_type_id, household_occupancy_id (TBD: physical_house_id vs ho_id), period, amount, paid_amount, status, due_date, created_at, updated_at | Per-bill: one row per Iuran type per occupancy per period |
| `iuran_payment` | id, rt_id, bill_id, amount, paid_at, payment_method, reference, notes, created_by, created_at, updated_at | Payment record: multiple per bill |

### Key Constraints

| Constraint | Table | Description |
|------------|-------|-------------|
| `chk_iuran_amount_positive` | `iuran_bill.amount > 0`, `iuran_payment.amount > 0` | Money always positive |
| `chk_iuran_status` | `iuran_bill.status IN ('generated','unpaid','partial','paid','cancelled','reversed')` | State machine |
| `chk_payment_status` | `iuran_payment.status IN ('pending','posted','reversed')` | State machine |
| `uq_iuran_bill_unique` | `(iuran_type_id, household_occupancy_id, period)` | No duplicate bills |
| `uq_kas_transaction_source` | `(source_type, source_id)` where source_type = 'IURAN_PAYMENT' | No duplicate KAS entries |

---

## 5. State Machine

### 5.1 Bill Lifecycle

```
                    ┌──────────────┐
                    │  generated   │  ← Bill record created (not yet sent)
                    └──────┬───────┘
                           │
                    generate()
                           │
                           ▼
                    ┌──────────────┐
              ┌────│    unpaid    │◄──── re-generate (if cancelled)
              │     └──────┬───────┘
              │            │
              │     record_payment()  (partial or full)
              │            │
              │            ▼
              │     ┌──────────────┐
              │     │   partial    │◄──── record_payment() (more paid)
              │     └──────┬───────┘
              │            │
              │     record_payment()  (completes balance)
              │            │
              │            ▼
              │     ┌──────────────┐
              └─────│    paid      │
                    └──────┬───────┘
                           │
                    reverse_posted_payment()
                           │
                           ▼
                    ┌──────────────┐
                    │   reversed   │  ← Reversal transaction created
                    └──────────────┘


  Cancellation (no payments yet):
    generated ──────→ cancelled
    unpaid  ──────→ cancelled
```

**Valid Transitions:**

| From → To | Condition |
|-----------|-----------|
| `generated` → `unpaid` | Bill is published (sent to occupant) |
| `unpaid` → `partial` | Payment recorded, but not full amount |
| `partial` → `paid` | Additional payment completes balance |
| `paid` → `reversed` | Reversal transaction created |
| `generated` → `cancelled` | No payments recorded yet |
| `unpaid` → `cancelled` | No payments recorded yet |

**Invalid Transitions:**

| From → To | Reason |
|-----------|--------|
| `paid` → `unpaid` | Financial immutability — use reversal instead |
| `reversed` → any | Reversal is terminal |
| `cancelled` → any | Cancelled bills are terminal |

### 5.2 Payment Lifecycle

```
              ┌──────────────┐
              │   pending    │  ← Payment recorded, not yet posted
              └──────┬───────┘
                     │
              post()  │
                     │
                     ▼
              ┌──────────────┐
              │   posted     │  ← Posted to ledger, creates KAS income
              └──────┬───────┘
                     │
              reverse() │
                     │
                     ▼
              ┌──────────────┐
              │   reversed   │  ← Reversal transaction created
              └──────────────┘
```

**Valid Transitions:**

| From → To | Condition |
|-----------|-----------|
| `pending` → `posted` | Payment confirmed, posted to ledger |
| `posted` → `reversed` | Reversal via authorized user |

**Invalid Transitions:**

| From → To | Reason |
|-----------|--------|
| `posted` → `pending` | Financial immutability |
| `reversed` → any | Terminal state |

---

## 6. Edge Cases

### EC-01 — Occupant Moves Mid-Month

**Scenario:** Owner moves out on Oct 15. Tenant moves in Oct 15. October bills generated before move.

**Handling:**
- October bill (generated before move): stays with owner (or is re-generated for new occupant — decision TBD).
- November bill onwards: billed to new tenant.
- Past bills (Jan–Sep): unchanged.
- **Key principle:** Past bills are immutable. Only future bill generation is affected.

### EC-02 — House Becomes Vacant

**Scenario:** Current owner sells house. New owner not yet registered. House is vacant Oct 2026.

**Handling:**
- Per BR-19 recommendation: bill continues for vacant house (owner obligation persists until new occupancy is registered).
- When new occupant is registered, bill follows new occupancy.

### EC-03 — Overpayment

**Scenario:** Bill = Rp50,000. Payment = Rp100,000.

**Handling:**
- Per BR-10: overpayment handling TBD (DR-03).
- **Recommendation:** Overpayment becomes credit for future periods (e.g., November bill = Rp50,000 - Rp50,000 credit = Rp0).

### EC-04 — Duplicate Payment Request

**Scenario:** Same payment request sent twice (network retry).

**Handling:**
- Per BR-13: `Idempotency-Key` header prevents duplicate.
- `uq_kas_transaction_source` constraint prevents duplicate KAS income.
- Second request returns original response (200 OK, no new records).

### EC-05 — Payment Recording Fails Mid-Transaction

**Scenario:** Iuran payment created, but KAS income creation fails.

**Handling:**
- Per BR-12: atomic transaction → ROLLBACK both Iuran payment and KAS income.
- Neither record is persisted.

### EC-06 — Reverse a Payment After KAS Report Already Generated

**Scenario:** Payment posted in October. KAS monthly report generated. Payment reversed in November.

**Handling:**
- Reversal creates a reversal transaction in November.
- KAS report for October remains unchanged (historical integrity).
- KAS report for November includes the reversal.
- Cumulative balance is correct.

### EC-07 — User Creates Bill for Non-Existent Occupancy

**Scenario:** Bendahara tries to generate bill for a household_occupancy that has `end_date` set (past occupancy).

**Handling:**
- Validation: bill generation must only target CURRENT occupancies (NULL end_date).
- Error response: 400 Bad Request — "Cannot generate bill for past occupancy."

### EC-08 — Tenant Becomes Owner

**Scenario:** Same person, same house, same household. Status changes from TENANT → OWNER.

**Handling:**
- New household_occupancy record created with new occupancy_status = OWNER, start_date = change date.
- Past bills (as TENANT): unchanged.
- Future bills (as OWNER): follow new occupancy.
- May have different nominal amounts (owner vs tenant).

### EC-09 — Multiple Iuran Types for Same Occupant

**Scenario:** One house has both OWNER and TENANT (if EXCLUDE constraint is removed, DR-08).

**Handling:**
- Bills generated separately per occupancy per Iuran type.
- Owner may pay "Iuran Infrastruktur", tenant may pay "Iuran Keamanan".

### EC-10 — RT Renamed or Migrated

**Scenario:** RT structure changes (RT merge, RT split).

**Handling:**
- rt_id is immutable on existing financial records.
- New bills use new rt_id.
- Historical data remains under original rt_id.
- SUPER_ADMIN can query across RTs.

### EC-11 — Bill Generated But Occupant No Longer Exists

**Scenario:** Bill generated for October. Occupant moved out September 30. Bill is still generated.

**Handling:**
- Bill follows the OCCUPANCY record, not the physical presence.
- If occupancy is current (NULL end_date), bill is generated.
- If occupancy ended September 30, bill for October goes to NEW occupancy (or not generated if house vacant).

### EC-12 — Concurrent Payment Recording

**Scenario:** Two users (or retry) attempt to record payment for the same bill simultaneously.

**Handling:**
- Database-level constraint: `paid_amount` on bill must never exceed `amount`.
- Or: payment record must be unique per bill + idempotency key.
- Second payment attempt returns error: 409 Conflict — "Payment already exists."

---

## 7. Recommended Next Phase

### Phase B — Schema Design

1. **Finalize all Decisions Required (DR-01 through DR-10)** with stakeholder input.
2. **Design database schema** based on finalized business rules:
   - `iuran_type` table
   - `iuran_bill` table (with FK decision from DR-01)
   - `iuran_payment` table
   - `kas_category` table (if not exists)
3. **Write migration files** (up/down) for new tables.
4. **Design API contract** (request/response formats, status codes).
5. **Design authorization matrix** (DR-09) — map to `position_permissions`.

### Phase C — Backend Implementation

1. **BE-01:** Database migrations
2. **BE-02:** Models + repositories
3. **BE-03:** Iuran service layer
4. **BE-04:** KAS service layer (extension)
5. **BE-05:** Iuran → KAS atomic integration
6. **BE-06:** Authorization
7. **BE-07:** API handlers/routes
8. **BE-08:** Automated tests

### Phase D — Frontend Integration

1. **BE-09:** Web API integration (replace mock data)
2. **BE-10:** Android API integration (replace mock data)
3. **BE-11:** E2E validation

### Phase E — Final Audit

1. **BE-12:** Code review
2. **BE-13:** Compliance with AGENTS.md (tenant isolation, immutability, security)
3. **BE-14:** Commit checkpoint

---

## 8. Validation

### Files Changed: NONE

This document is a **discovery and planning output only**.

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

### Output Format Compliance

| Section | Present |
|---------|---------|
| 1. Current Evidence | ✅ Section 1 — FACT vs ASSUMPTION |
| 2. Proposed Business Rules | ✅ Section 2 — 20 rules with Confidence levels |
| 3. Decisions Required | ✅ Section 3 — 10 decisions (DR-01 through DR-10) |
| 4. Recommended Entity Relationship | ✅ Section 4 — ASCII diagram + table list |
| 5. State Machine | ✅ Section 5 — Bill + Payment lifecycle diagrams |
| 6. Edge Cases | ✅ Section 6 — 12 edge cases with handling |
| 7. Recommended Next Phase | ✅ Section 7 — Phases B through E |
| 8. Validation | ✅ Section 8 — Files changed: NONE |

---

*End of Phase A Discovery Report*
