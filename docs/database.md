# Social Finance - Database Design

## 1. Design Principles

1. **Tenant isolation.** Every tenant-scoped table has `rt_id`.
2. **Immutable financial ledger.** No DELETE or UPDATE on posted transactions. Corrections use reversal transactions.
3. **Fixed-precision money.** All monetary columns use `NUMERIC(15, 2)`. Never `REAL`, `DOUBLE PRECISION`, or `FLOAT`.
4. **Soft deletes where appropriate.** Users, residents, and households use `is_active`. Financial transactions are NEVER deleted.
5. **Calculated balance.** No denormalized balance columns. Balance is always derived from ledger.
6. **Standard timestamps.** All tables include `created_at` and `updated_at` (or equivalent). Use `TIMESTAMPTZ`.
7. **UUID primary keys.** Use `uuid` type for primary keys.
8. **Foreign keys.** All cross-table references use foreign keys with `ON DELETE RESTRICT` or `ON DELETE CASCADE`.
9. **Indexes on foreign keys and frequently filtered columns.**
10. **Check constraints** enforce valid enum-like values.

---

## 2. Current Schema (Phase 2)

### 2.1 Global Tables (SUPER_ADMIN Scope)

#### `rts` — RT Organizations

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uuid` | PK, default `gen_random_uuid()` | Tenant identifier |
| `name` | `text` | NOT NULL | RT name (e.g., "RT 05 / RW 03") |
| `rw` | `int` | NOT NULL | RW number |
| `rt` | `text` | NOT NULL | RT number |
| `address` | `text` | | RT address |
| `head_name` | `text` | | RT head (Ketua RT) name |
| `is_active` | `boolean` | NOT NULL, default `true` | Soft delete flag |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | |
| `updated_at` | `timestamptz` | NOT NULL, default `now()` | |

Indexes:
- `uniq_rts_active` — UNIQUE `(rw, rt) WHERE is_active = true` — enforces unique `(rw, rt)` among active RTs only. Inactive historical records do not block reuse.

---

### 2.2 Authentication & Users

#### `users` — Application Users (Global Identity)

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uuid` | PK, default `gen_random_uuid()` | |
| `email` | `text` | NOT NULL, UNIQUE (normalized) | Login identifier (`LOWER(TRANSLATE(email,' ',''))`) |
| `phone` | `text` | | Phone number |
| `password_hash` | `text` | NOT NULL | Argon2id hash, cost 12 |
| `full_name` | `text` | NOT NULL | Display name |
| `password_changed_at` | `timestamptz` | NOT NULL, default `zero` | For forcing re-auth on compromise |
| `system_role` | `text` | NULL, CHECK = `super_admin` or NULL | Global authorization. `NULL` = not a system admin |
| `is_active` | `boolean` | NOT NULL, default `true` | Account active flag |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | |
| `updated_at` | `timestamptz` | NOT NULL, default `now()` | |

Indexes:
- `idx_users_system_role` — Filter super_admin users
- `idx_users_email_normalized` — UNIQUE `(LOWER(TRANSLATE(email,' ','')))` — global unique login
- `idx_users_is_active` — For tenant-scoped active lookups

**Key design:** The `rt_id` column was removed from `users` in migration v3. User-to-tenant relationship is managed via `user_rt_memberships`. A `system_role` column on `users` encodes global authorization (`super_admin`) separate from tenant membership.

**SUPER_ADMIN users:** `system_role = 'super_admin'`, no `user_rt_memberships` row, no tenant scope.

#### `user_rt_memberships` — Tenant Role Assignment

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uuid` | PK, default `gen_random_uuid()` | |
| `user_id` | `uuid` | NOT NULL, FK → `users.id` ON DELETE CASCADE | |
| `rt_id` | `uuid` | NOT NULL, FK → `rts.id` ON DELETE CASCADE | Tenant scope |
| `role` | `text` | NOT NULL, CHECK IN (`pengurus`, `bendahara`, `warga`) | Tenant role |
| `is_active` | `boolean` | NOT NULL, default `true` | |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | |
| `updated_at` | `timestamptz` | NOT NULL, default `now()` | |

Indexes:
- `idx_memberships_user_rt_active` — UNIQUE `(user_id, rt_id) WHERE is_active = true`
- `idx_memberships_user_id`
- `idx_memberships_rt_id`
- `idx_memberships_rt_active` — `(rt_id, is_active)`

**Key design:** This table replaces the old approach where `users.role` and `users.rt_id` encoded tenant access. The separation allows a `super_admin` global user to hold zero tenant memberships, while tenant users hold exactly one (or more) memberships.

#### `refresh_tokens` — Server-Side Refresh Token Store

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uuid` | PK, default `gen_random_uuid()` | |
| `user_id` | `uuid` | NOT NULL, FK → `users.id` ON DELETE CASCADE | |
| `membership_id` | `uuid` | NOT NULL, FK → `user_rt_memberships.id` ON DELETE CASCADE | Ties token to specific tenant role |
| `token_hash` | `text` | NOT NULL | SHA-256 of 32-byte random token |
| `expires_at` | `timestamptz` | NOT NULL | Token expiry |
| `revoked_at` | `timestamptz` | NULL | Set on logout or rotation |
| `replaced_by_hash` | `text` | NULL | Set on token rotation (revokes old token) |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | |

Indexes:
- `idx_refresh_tokens_membership` — For cleanup by membership
- `idx_refresh_tokens_token_hash` — For validation on refresh
- `idx_refresh_tokens_expires` — `(expires_at) WHERE revoked_at IS NULL` — for GC

**Key design:** Refresh tokens are opaque random strings (not JWTs). Only their SHA-256 hash is stored. On refresh, the token is hashed and looked up. Successful rotation marks the old token as revoked and replaces it. Logout marks it revoked. Expired/revoked tokens are garbage-collected.

---

## 3. Current Schema (Phase 3)

### 3.1 Household Management

#### `households` (Phase 3)

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uuid` | PK, default `gen_random_uuid()` | |
| `rt_id` | `uuid` | NOT NULL, FK → `rts.id` | Tenant scope |
| `kk_number` | `text` | NULL, UNIQUE per RT | Kartu Keluarga number |
| `head_name` | `text` | NOT NULL | Head of household name |
| `address` | `text` | | Household address detail |
| `phone` | `text` | NULL | Household contact phone |
| `is_active` | `boolean` | NOT NULL, default `true` | Soft delete flag |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | |
| `updated_at` | `timestamptz` | NOT NULL, default `now()` | |
| `occupancy_status` | `text` | NULL, CHECK | Home ownership status |

Occupancy status values:
- `OWNER`: Household occupies an owned home.
- `TENANT`: Household occupies a rented home.
- `NULL`: Not yet classified / unknown. Transitional until Household API domain integration assigns a value.

Allowed values enforced via CHECK constraint: `occupancy_status IS NULL OR occupancy_status IN ('OWNER', 'TENANT')`.

Indexes:
- `idx_households_rt_id` on `(rt_id)`
- `idx_households_rt_is_active` on `(rt_id, is_active)`
- `uniq_households_rt_kk` UNIQUE `(rt_id, kk_number) WHERE is_active = true AND kk_number IS NOT NULL`

**Behavior:** `kk_number` is unique among active households within an RT. Soft-deleted households (`is_active = false`) can reuse the same `kk_number`. Read endpoints default to filtering active households only; write endpoints reject updates on inactive households.

**Note:** `occupancy_status` is a future-facing column. NULL values represent unclassified / not-yet-assigned status. When tenant lifecycle (rental contract) functionality is eventually implemented, the rental-contract entity must NOT overwrite or replace the historical `occupancy_status` of a household. Historical financial records must remain associated with the correct historical household/resident identity.

### 3.2 Resident Management

#### `residents` (Phase 3)

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uuid` | PK, default `gen_random_uuid()` | |
| `rt_id` | `uuid` | NOT NULL, FK → `rts.id` | Tenant scope |
| `household_id` | `uuid` | NOT NULL, FK → `households.id` | Household membership |
| `full_name` | `text` | NOT NULL | |
| `phone` | `text` | NULL | |
| `relationship_to_head` | `text` | NULL | e.g. "self", "spouse", "child" |
| `is_active` | `boolean` | NOT NULL, default `true` | Soft delete flag |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | |
| `updated_at` | `timestamptz` | NOT NULL, default `now()` | |

Indexes:
- `idx_residents_rt_id` on `(rt_id)`
- `idx_residents_household_id` on `(household_id)`
- `idx_residents_rt_household_active` on `(rt_id, household_id, is_active)`

**Behavior:** Residents are tied to a specific household. Cross-tenant assignment is enforced at the repository level (household must belong to the requesting RT). Read endpoints default to filtering active residents only; write reject updates on inactive residents.

### 3.3 Physical Houses & Temporal Occupancy (Phase 3 / Migration 008)

- `physical_houses` — Physical structures (`rt_id`, `house_number`, `address`, `is_active`)
- `household_occupancies` — Temporal occupancy periods (`household_id`, `physical_house_id`, `occupancy_status`, `start_date`, `end_date`)
- `residency_periods` — Resident-to-occupancy temporal links (`resident_id`, `household_occupancy_id`, `relationship_to_head`, `start_date`, `end_date`)

---

## 4. Financial Schema (Phase 4 / Migration 009)

### 4.1 `financial_categories` — Income and Expense Categories

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uuid` | PK, default `gen_random_uuid()` | |
| `rt_id` | `uuid` | NOT NULL, FK → `rts.id` ON DELETE CASCADE | Tenant scope |
| `name` | `text` | NOT NULL | Category name (e.g. "Iuran Kebersihan") |
| `type` | `text` | NOT NULL, CHECK IN (`income`, `expense`) | Financial category type |
| `is_active` | `boolean` | NOT NULL, default `true` | Active status |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | |
| `updated_at` | `timestamptz` | NOT NULL, default `now()` | |

Indexes & Constraints:
- UNIQUE `(rt_id, name)`
- Index on `(rt_id)`

### 4.2 `dues` — Dues Configuration

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uuid` | PK, default `gen_random_uuid()` | |
| `rt_id` | `uuid` | NOT NULL, FK → `rts.id` ON DELETE CASCADE | Tenant scope |
| `name` | `text` | NOT NULL | Dues title |
| `amount` | `numeric(15, 2)` | NOT NULL, CHECK `amount > 0` | Mandatory dues amount (Rupiah) |
| `period_type` | `text` | NOT NULL, CHECK IN (`monthly`, `yearly`, `one_time`) | Billing interval |
| `is_active` | `boolean` | NOT NULL, default `true` | Active status |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | |
| `updated_at` | `timestamptz` | NOT NULL, default `now()` | |

Indexes:
- Index on `(rt_id)`

### 4.3 `bills` — Household Occupancy Bills

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uuid` | PK, default `gen_random_uuid()` | |
| `rt_id` | `uuid` | NOT NULL, FK → `rts.id` ON DELETE CASCADE | Tenant scope |
| `household_occupancy_id` | `uuid` | NOT NULL, FK → `household_occupancies.id` ON DELETE RESTRICT | Billed occupancy period |
| `due_id` | `uuid` | NOT NULL, FK → `dues.id` ON DELETE RESTRICT | Dues definition |
| `amount` | `numeric(15, 2)` | NOT NULL, CHECK `amount > 0` | Billed amount |
| `period` | `text` | NOT NULL | e.g. "2026-09" |
| `due_date` | `date` | NOT NULL | Payment deadline |
| `status` | `text` | NOT NULL, default `'unpaid'`, CHECK IN (`unpaid`, `paid`, `cancelled`) | Bill status |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | |
| `updated_at` | `timestamptz` | NOT NULL, default `now()` | |

Indexes & Constraints:
- UNIQUE `(household_occupancy_id, due_id, period)`
- Index on `(rt_id)`
- Index on `(household_occupancy_id)`
- Index on `(due_id)`
- Index on `(status)`

### 4.4 `payments` — Payment Records

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uuid` | PK, default `gen_random_uuid()` | |
| `rt_id` | `uuid` | NOT NULL, FK → `rts.id` ON DELETE CASCADE | Tenant scope |
| `bill_id` | `uuid` | NOT NULL, FK → `bills.id` ON DELETE RESTRICT | Target bill |
| `amount` | `numeric(15, 2)` | NOT NULL, CHECK `amount > 0` | Paid amount |
| `method` | `text` | NOT NULL, CHECK IN (`CASH`, `TRANSFER`) | Payment method |
| `origin` | `text` | NOT NULL, CHECK IN (`SELF_SUBMITTED`, `STAFF_RECORDED`) | Submission origin |
| `status` | `text` | NOT NULL, default `'PENDING'`, CHECK IN (`PENDING`, `APPROVED`, `REJECTED`) | Verification status |
| `proof_path` | `text` | NULL | Uploaded receipt image file path |
| `paid_at` | `timestamptz` | NOT NULL, default `now()` | Actual payment timestamp |
| `verified_by` | `uuid` | NULL, FK → `users.id` ON DELETE SET NULL | Verifier |
| `verified_at` | `timestamptz` | NULL | Verification timestamp |
| `rejection_reason` | `text` | NULL | Reason if rejected |
| `notes` | `text` | NULL | Optional remarks |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | |
| `updated_at` | `timestamptz` | NOT NULL, default `now()` | |

Indexes:
- Index on `(rt_id)`
- Index on `(bill_id)`
- Index on `(status)`

### 4.5 `transactions` — Append-Only Financial Ledger

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uuid` | PK, default `gen_random_uuid()` | |
| `rt_id` | `uuid` | NOT NULL, FK → `rts.id` ON DELETE CASCADE | Tenant scope |
| `category_id` | `uuid` | NOT NULL, FK → `financial_categories.id` ON DELETE RESTRICT | Financial category |
| `amount` | `numeric(15, 2)` | NOT NULL, CHECK `amount > 0` | Always positive |
| `type` | `text` | NOT NULL, CHECK IN (`income`, `expense`) | Direction |
| `status` | `text` | NOT NULL, default `'posted'`, CHECK IN (`draft`, `posted`, `cancelled`) | Immutability status |
| `reverses_transaction_id` | `uuid` | NULL, FK → `transactions.id` ON DELETE RESTRICT | Reversal link |
| `payment_id` | `uuid` | NULL, FK → `payments.id` ON DELETE RESTRICT | Payment link |
| `description` | `text` | NOT NULL | Audit ledger note |
| `occurred_at` | `timestamptz` | NOT NULL, default `now()` | Event date |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | |
| `updated_at` | `timestamptz` | NOT NULL, default `now()` | |

Indexes:
- Index on `(rt_id)`
- Index on `(category_id)`
- Index on `(status)`
- Index on `(reverses_transaction_id)`
- Index on `(payment_id)`

### 4.6 `idempotency_keys` & `audit_logs`

- `idempotency_keys`: `PRIMARY KEY (rt_id, key)` storing `request_hash`, `response_code`, `response_body`, `created_at`.
- `audit_logs`: `(id, rt_id, user_id, action, entity_type, entity_id, old_values, new_values, ip_address, created_at)`.

---

## 5. Migration History

| Version | File | Description | Schema Dirty |
|---------|------|-------------|-------------|
| 001 | `001_initial.up.sql` | Creates `rts` table | false |
| 002 | `002_auth.up.sql` | Creates `users`, `user_rt_memberships`, `refresh_tokens` | false |
| 003 | `003_separate_global_identity_from_tenant_membership.up.sql` | Adds `system_role` to users, drops `rt_id` from users, removes super_admin memberships, constrains membership roles | false |
| 004 | `004_households_and_residents.up.sql` | Creates `households` and `residents` tables with indexes and constraints | false |
| 005 | `005_fix_rt_unique_indexes.up.sql` | Drops redundant full `idx_rts_rw_rt` unique index; keeps partial `uniq_rts_active` (active-only uniqueness) | false |
| 006 | `006_add_household_occupancy_status.up.sql` | Adds nullable `occupancy_status` text column to `households` with CHECK constraint for `OWNER`/`TENANT`; NULL for transitional unclassified data | false |
| 007 | `007_audit_and_idempotency.up.sql` | Adds foundation audit logging & idempotency tracking | false |
| 008 | `008_physical_houses_and_occupancies.up.sql` | Adds Option B+ temporal physical house and occupancy normalization | false |
| 009 | `009_finance.up.sql` | Creates financial categories, dues, bills, payments, transactions ledger | false |

Migration execution: `docker compose run --rm migrate up`

---

## 6. Security Constraints Summary

| Constraint | Table | Enforces |
|-----------|-------|----------|
| UNIQUE `LOWER(TRANSLATE(email,' ',''))` | `users` | Global unique login identity |
| CHECK `system_role IS NULL OR system_role = 'super_admin'` | `users` | Only valid system role |
| UNIQUE `(user_id, rt_id)` WHERE `is_active` | `user_rt_memberships` | No duplicate active memberships |
| CHECK `role IN ('pengurus', 'bendahara', 'warga')` | `user_rt_memberships` | Valid tenant roles only |
| CHECK `occupancy_status IS NULL OR (occupancy_status = ANY (ARRAY['OWNER', 'TENANT']))` | `households` | Home is owned, rented, or not yet classified |
| CHECK `system_role IS NULL OR system_role = 'super_admin'` | `users` | No arbitrary system privileges |
| FK CASCADE on `refresh_tokens` | `refresh_tokens` | Cleanup on user/membership deletion |

---

## 6. Key Architectural Decisions

### 6.1 Global Identity vs Tenant Membership

The `users` table holds the **global identity** (email, password hash, display name, phone). A user has zero or more **tenant memberships** in `user_rt_memberships`, each with a tenant role.

```
User (system_role = super_admin) → no memberships, platform-wide access
User (system_role = NULL)        → 1+ memberships, tenant-scoped access
```

### 6.2 Refresh Token Security

Refresh tokens are **opaque** (not JWTs). Only a hash is stored on the server. This means:
- A stolen token can be revoked (by deleting the hash)
- Rotation invalidates the old token
- No server-side JWT blacklist needed
- Token theft is limited to the single-use window

### 6.3 Password Change Detection

The `password_changed_at` column enables logout on all sessions when a user changes their password. This is checked on refresh token validation.

# SOCIAL FINANCE — CONCEPTUAL BILLING & OPERATIONS

_Documenting planned future functionality only. All items below are **PLANNED / NOT IMPLEMENTED**. No final schema, migrations, or API contracts are defined here. A future design phase will define constraints, indexes, and implementation details._

---

## 7. Household Billing Model — PLANNED / NOT IMPLEMENTED

Monthly dues are represented as actual monthly bills/invoices attached to each household.

**Key principle:** Financial records are the **source of truth** for arrears. Arrears are derived from bill and payment records. Do not use manually maintained aggregate fields such as `arrears_months` or `arrears_amount` as the financial source of truth.

### 7.1 Conceptual Household → Bill → Payment Model

```
Household
  |
  +-- Monthly Bills
  |     period          e.g. "2026-09"
  |     amount          numeric(15, 2) — Rupiah
  |     due_date        date
  |     status
  |
  +-- Payments
        bill_id         link to target bill
        amount          numeric(15, 2) — Rupiah
        paid_at         timestamptz
        method          CASH / TRANSFER
```

### 7.2 Conceptual Bill States — PLANNED

- `UNPAID` — no payment applied, or past due date with no payment
- `PARTIAL` — at least one payment applied, but bill is not fully paid
- `PAID` — fully paid

**Exact enum values and transitions are not finalized.** The conceptual states above inform the planned behavior.

### 7.3 Arrears Derivation Example

```
Monthly fee:          Rp50.000

Jan 2026   PAID       Rp50.000
Feb 2026   UNPAID     Rp50.000
Mar 2026   UNPAID     Rp50.000
Apr 2026   UNPAID     Rp50.000

Derived result:
  Outstanding months:  3
  Outstanding amount:  Rp150.000
```

Outstanding months and amounts are **calculated queries**, never stored columns.

---

## 8. Garbage Collection Service Rule — PLANNED / NOT IMPLEMENTED

### 8.1 Eligibility Derivation

```
Monthly Bill
    ↓
Payment
    ↓
Arrears Calculation (derived)
    ↓
Service Eligibility (derived)
    ↓
Garbage Collection Operation
```

### 8.2 Initial Planned Rule

| Overdue monthly bills | Service state |
|-----------------------|---------------|
| 0–2 | ACTIVE |
| >= 3 | SUSPENDED |

**IMPORTANT:** Service eligibility must be derived from financial state. A manual boolean such as `trash_pickup = false` is **not** the source of truth.

### 8.3 Administrative Override / Dispensation — PLANNED

Conceptual override mechanism for exceptional cases:

| Override state | Meaning |
|----------------|---------|
| `AUTO` | Service state derived automatically from billing |
| `SUSPENDED` | Manually suspended (overrides billing state) |
| `EXEMPTED` | Granted exemption (overrides billing state) |

Conceptual audit metadata:

| Field | Note |
|-------|------|
| reason | Why override was granted |
| changed_by | User who granted override |
| changed_at | When override was granted |
| expiry_date | Optional — exemption expires on this date |

Billing remains the financial source of truth. Overrides are temporary, auditable exceptions.

---

## 9. User != Resident — PLANNED / NOT IMPLEMENTED

This is an **architectural principle** to be preserved in the final schema design:

A system `User` does NOT necessarily represent a resident.

A non-resident operational worker may have:

```
User
  |
  +-- RT Membership (user_rt_memberships)
  |
  +-- NO Resident/Household association
```

**Do NOT create fake Warga/Resident records merely to provide login access.** This preserves the separation between:
- authentication identity (`users` table)
- tenant membership (`user_rt_memberships`)
- resident/household domain records (`residents`, `households`)

### 9.1 Operator Role — PLANNED (Not Currently Implemented)

**Current tenant roles** (as defined by the CHECK constraint on `user_rt_memberships.role`):

- `pengurus`
- `bendahara`
- `warga`

**`operator` is PLANNED.** It requires a future RBAC design including migration to expand the role CHECK constraint. Do not treat it as an existing role.

Preliminary permission model (subject to formal RBAC design):

| Role | Permissions |
|------|------------|
| Pengurus | administration, resident management, reports, configuration, authorized service overrides |
| Bendahara | income, expenses, billing, payments, arrears |
| Operator (PLANNED) | operational service list, see eligibility status, update explicitly permitted operational statuses; NO financial/admin access, NO sensitive resident data |
| Warga | access to permitted household/self-service information |

**Domain decision:** User/account identity is independent from resident/household identity. Operational access follows least privilege.

---

## 10. General / Unexpected Income — PLANNED / NOT IMPLEMENTED

Social Finance should support income beyond standard monthly dues using a general transaction categorization system.

### 10.1 Conceptual Income Categories

```
INCOME
  ├── Iuran Warga        (standard monthly dues)
  ├── Sumbangan          (donations)
  ├── THR                (religious holiday allowance)
  ├── Kegiatan            (events/activities)
  ├── Bantuan             (assistance)
  └── Lainnya            (miscellaneous)
```

Category names are initial product concepts. They may become tenant-configurable in future phases.

### 10.2 General Income Record — PLANNED

Expected information for non-regular income records:

| Field | Note |
|-------|------|
| Category | Matches an income category |
| Description | Human-readable description |
| Amount | numeric(15, 2) — Rupiah |
| Date | Occurrence date |
| Source | Who contributed (e.g., "Warga") |
| Payment method | CASH / TRANSFER / etc. |
| Notes | Additional remarks |
| Recorded by | User who recorded the transaction |

**Example:**
```
Category:     Sumbangan
Description:  Sumbangan warga untuk 17 Agustus
Amount:       Rp1.500.000
Source:       Warga
Method:       Transfer
```

### 10.3 Future: Event Accounting (Possibility Only)

Event accounting — grouping income/expenses by specific events (e.g., "HUT RI 2027" — stage costs, food, prizes, resulting balance) — is a **future possibility only**. It is NOT in scope for this roadmap phase.

Design decisions made here (categorization over hardcoded types) must not prevent this future capability.

---

## 11. Domain Principles

These principles govern future schema and API design decisions:

1. **Financial records are the source of truth for arrears.** Derive counts and totals from bill and payment records. Never store denormalized aggregates as financial truth.

2. **Service eligibility is derived from financial state plus authorized exceptions.** No manual service toggle replaces billing.

3. **User/account identity is independent from resident/household identity.** A User may have zero resident/household records.

4. **Operational access follows least privilege.** Operator accounts see only what is necessary for service operations.

5. **General income uses categorization rather than hardcoded one-off transaction models.** No hardcoded per-event transaction type.

6. **Financial history must remain auditable.** Posted transactions and payments are never silently modified or deleted.

7. **Service automation must not silently mutate historical financial records.** Any service-related status changes are additions, not modifications.

8. **Tenant isolation remains RT-scoped.** Every table and query on tenant data includes `rt_id` filtering.
