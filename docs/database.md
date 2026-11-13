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

#### `rws` — RW Organizations

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uuid` | PK, default `gen_random_uuid()` | RW identifier |
| `rw_number` | `int` | NOT NULL, CHECK `rw_number > 0` | RW number |
| `name` | `text` | NOT NULL | RW display name (e.g., "RW 016") |
| `head_name` | `text` | NULL | RW head (Ketua RW) name |
| `address` | `text` | NULL | RW secretariat/area address |
| `is_active` | `boolean` | NOT NULL, default `true` | Soft delete flag |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | |
| `updated_at` | `timestamptz` | NOT NULL, default `now()` | |

Indexes:
- `uniq_rws_active` — UNIQUE `(rw_number) WHERE is_active = true` — enforces unique `rw_number` among active RWs.

#### `rts` — RT Organizations

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uuid` | PK, default `gen_random_uuid()` | Tenant identifier |
| `rw_id` | `uuid` | NULL, FK → `rws.id` ON DELETE RESTRICT | Parent RW organization reference |
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
- `idx_rts_rw_id` — Index on `(rw_id)` for RW-to-RT lookups and filtering.

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
| `jabatan` | `text` | NULL, CHECK IN (`ketua`, `wakil_ketua`, `sekretaris`, `bendahara`, `keamanan`, `sosial`, `operasional`, `kebersihan`, `anggota`) | RT organizational position |
| `is_active` | `boolean` | NOT NULL, default `true` | |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | |
| `updated_at` | `timestamptz` | NOT NULL, default `now()` | |

Indexes:
- `idx_memberships_user_rt_active` — UNIQUE `(user_id, rt_id) WHERE is_active = true`
- `idx_memberships_user_id`
- `idx_memberships_rt_id`
- `idx_memberships_rt_active` — `(rt_id, is_active)`

**Key design:** This table replaces the old approach where `users.role` and `users.rt_id` encoded tenant access. The separation allows a `super_admin` global user to hold zero tenant memberships, while tenant users hold exactly one (or more) memberships.

---

## 2.3 RW (Rukun Warga) Domain Tables

### `rws` — RW Organization Domain

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uuid` | PK, default `gen_random_uuid()` | |
| `name` | `text` | NOT NULL | RW display name (e.g., "RW 01") |
| `rw_number` | `int` | NOT NULL | Numeric RW identifier (e.g., 1, 16) |
| `head_name` | `text` | NOT NULL | RW head (Ketua RW) name |
| `is_active` | `boolean` | NOT NULL, default `true` | Soft delete flag |
| `community_id` | `uuid` | NULL, FK → `communities.id` ON DELETE RESTRICT | |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | |
| `updated_at` | `timestamptz` | NOT NULL, default `now()` | |

Indexes:
- `idx_rws_id` — PK
- `idx_rws_is_active` — For active RW filtering

**Key design:** RW is a parent organization that contains multiple RTs. The `rts.rw_id` FK links RTs to their parent RW. RW data is read-only aggregated from RT-level tables — no separate RW financial transactions or locations exist.

---

### `user_rw_memberships` — RW Role Assignment

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uuid` | PK, default `gen_random_uuid()` | |
| `user_id` | `uuid` | NOT NULL, FK → `users.id` ON DELETE CASCADE | |
| `rw_id` | `uuid` | NOT NULL, FK → `rws.id` ON DELETE CASCADE | RW scope |
| `jabatan` | `text` | NOT NULL, CHECK IN (`ketua`, `wakil_ketua`, `sekretaris`, `bendahara`) | RW position |
| `term_start` | `date` | NOT NULL | Membership term start date |
| `term_end` | `date` | NULL | Membership term end date (NULL = ongoing) |
| `is_active` | `boolean` | NOT NULL, default `true` | Soft delete flag |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | |
| `updated_at` | `timestamptz` | NOT NULL, default `now()` | |

Indexes:
- `idx_rw_memberships_user_rw_active` — UNIQUE `(user_id, rw_id) WHERE is_active = true`
- `idx_rw_memberships_user_id`
- `idx_rw_memberships_rw_id`
- `idx_rw_memberships_rw_active` — `(rw_id, is_active)`

**Key design:** This table manages RW-level roles separate from RT-level roles (`user_rt_memberships`). A user can hold both RT and RW memberships simultaneously. RW membership is required for accessing RW endpoints (dashboard, KAS, locations). RW positions (`ketua`, `wakil_ketua`, `sekretaris`, `bendahara`) determine permissions within the RW context.

**Position permissions:**
- `ketua`: Full RW access — read and write operations on KAS, transactions, and locations
- `wakil_ketua`: RW access — read operations on KAS, transactions, and locations
- `sekretaris`: RW access — read operations on KAS and transactions (no locations)
- `bendahara`: RW access — read and write operations on KAS and transactions (no locations)

**Authorization flow:**
1. `RequireAuth` middleware populates `AuthContext.RWMemberships` from DB
2. `RequireRWScope` validates the RW ID against `SCOPED_RW_IDS` and checks DB for active RW status
3. `checkRWAccess()` verifies the user has an active RW membership for the requested RW

**Note:** System-level `SUPER_ADMIN` users bypass the RW membership check and can access any RW that is in the configured `SCOPED_RW_IDS` and is active in the database.

---

### `refresh_tokens` — Server-Side Refresh Token Store

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
| `avatar_path` | `text` | NULL | Storage key for original profile photo (Migration 079) |
| `avatar_thumb_path` | `text` | NULL | Storage key for 200x200 JPEG thumbnail (Migration 082) |
| `is_active` | `boolean` | NOT NULL, default `true` | Soft delete flag |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | |
| `updated_at` | `timestamptz` | NOT NULL, default `now()` | |

Indexes:
- `idx_residents_rt_id` on `(rt_id)`
- `idx_residents_household_id` on `(household_id)`
- `idx_residents_rt_household_active` on `(rt_id, household_id, is_active)`
- `idx_residents_avatar_thumb_path` on `(avatar_thumb_path) WHERE avatar_thumb_path IS NOT NULL`

**Behavior:** Residents are tied to a specific household. Cross-tenant assignment is enforced at the repository level (household must belong to the requesting RT). Read endpoints default to filtering active residents only; write reject updates on inactive residents.

### 3.3 Physical Houses & Temporal Occupancy (Phase 3 / Migration 008)

- `physical_houses` — Physical structures (`rt_id`, `house_number`, `address`, `is_active`)
- `household_occupancies` — Temporal occupancy periods (`household_id`, `physical_house_id`, `occupancy_status`, `start_date`, `end_date`)
- `residency_periods` — Resident-to-occupancy temporal links (`resident_id`, `household_occupancy_id`, `start_date`, `end_date`)

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
| `amount` | `numeric(15, 2)` | NOT NULL | Mandatory amount must be positive; voluntary dues may be Rp0 |
| `period_type` | `text` | NOT NULL, CHECK IN (`monthly`, `yearly`, `one_time`) | Billing interval |
| `payment_rule` | `text` | NOT NULL, CHECK IN (`full`, `partial`) | Bill payment rule |
| `is_voluntary` | `boolean` | NOT NULL, default `false` | Whether this due definition accepts voluntary donations |
| `due_type` | `text` | NOT NULL, default `'regular'`, CHECK IN (`regular`, `voluntary`, `multi_month`) | Due category |
| `year` | `integer` | NULL | Calendar year for multi-month dues |
| `is_active` | `boolean` | NOT NULL, default `true` | Active status |
| `valid_until` | `date` | NULL, CHECK `valid_until IS NULL OR is_voluntary` | Optional last valid date for a voluntary due |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | |
| `updated_at` | `timestamptz` | NOT NULL, default `now()` | |

Indexes:
- Index on `(rt_id)`
- Index on `(due_type)`
- Unique partial index on `(rt_id, year)` WHERE `due_type = 'multi_month'` (enforces at most one multi-month IPL issuance per calendar year per RT)
- Partial index on `(valid_until)` WHERE `is_voluntary AND is_active AND valid_until IS NOT NULL` for the daily expiry job

Automatic expiry semantics:
- `valid_until` is only valid for voluntary dues. Regular dues keep it `NULL` and are never expired automatically.
- The due remains active for the entire `valid_until` date in `Asia/Jakarta` and becomes expired at `00:00 WIB` on the next date.
- A daily backend job runs at `00:05 WIB` and sets `is_active = false` for expired voluntary dues. The same expiry rule is enforced synchronously when creating bills or accepting new donations, so correctness does not depend on the scheduler.
- Re-activating an expired voluntary due requires explicitly extending `valid_until` to today or later and setting `is_active = true`; the backend rejects activation while the stored date is already past.
- Expiry only changes `dues.is_active`. Existing bills, payments, allocations, transactions, balances, and historical reports are never modified.

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
| `status` | `text` | NOT NULL, default `'unpaid'`, CHECK IN (`unpaid`, `partial`, `paid`, `cancelled`) | Bill status |
| `is_active` | `boolean` | NOT NULL, default `true` | Active/inactive status of the bill |
| `is_obligatory` | `boolean` | NOT NULL, default `true` | Indicates whether the bill period is an obligation based on move_in_date |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | |
| `updated_at` | `timestamptz` | NOT NULL, default `now()` | |

Indexes & Constraints:
- UNIQUE `(household_occupancy_id, due_id, period)`
- Index on `(rt_id)`
- Index on `(household_occupancy_id)`
- Index on `(due_id)`
- Index on `(status)`
- Index on `(rt_id, is_active)`
- Index on `(household_occupancy_id, due_id, is_active)`
- Index on `(household_occupancy_id, due_id, is_obligatory)`

### 4.4 `payments` — Payment Records

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uuid` | PK, default `gen_random_uuid()` | |
| `rt_id` | `uuid` | NOT NULL, FK → `rts.id` ON DELETE CASCADE | Tenant scope |
| `bill_id` | `uuid` | NULL, FK → `bills.id` ON DELETE RESTRICT | Target bill for ordinary payments |
| `due_id` | `uuid` | NULL, FK → `dues.id` ON DELETE RESTRICT | Target due definition for voluntary donations |
| `submitted_by` | `uuid` | NULL, FK → `users.id` ON DELETE SET NULL | Submitter; used to scope warga donation access |
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
- Index on `(due_id)`
- Index on `(status)`

Exactly one of `bill_id` or `due_id` must be non-NULL. A due-targeted payment is accepted only for an active `is_voluntary = true` definition in the authenticated RT. Rp0 voluntary definitions do not create bills or cash ledger entries; a donation requires a positive amount. Pending and rejected payments do not affect the ledger.

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
| `reference_no` | `text` | NULL, UNIQUE | System-generated transaction reference number (e.g. `KAS-YYYYMMDD-XXXXX`) |
| `payment_method` | `text` | NULL, CHECK IN (`CASH`, `TRANSFER`) | Payment method (`CASH` or `TRANSFER`) |
| `created_by` | `uuid` | NULL, FK → `users.id` ON DELETE SET NULL | Officer who recorded the manual transaction |
| `expense_category` | `text` | NULL | Standardized expense category for expense transactions |
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
- UNIQUE `(reference_no)` (`transactions_reference_no_unique`)

### 4.6 `idempotency_keys` & `audit_logs`

- `idempotency_keys`: `PRIMARY KEY (rt_id, key)` storing request identity (`user_id`, `request_path`, `request_hash`), `response_code`, `response_body`, and `created_at`. API instances serialize use of a key with a PostgreSQL advisory lock; payment creation and its cached response share one transaction.
### 4.7 `period_opening_balances` — Monthly Period Opening Balance & Audit Trail

| Column | Type | Constraints | Description |
|---|---|---|---|
| `id` | `uuid` | PK, default `gen_random_uuid()` | |
| `rt_id` | `uuid` | NOT NULL, FK → `rts.id` ON DELETE CASCADE | Tenant scope |
| `period` | `text` | NOT NULL | Format 'YYYY-MM' |
| `opening_balance` | `numeric(15, 2)` | NOT NULL, default `0.00` | Opening balance for the period |
| `source` | `text` | NOT NULL, default `'automatic'`, CHECK IN (`automatic`, `manual`) | Origin of opening balance |
| `previous_balance` | `numeric(15, 2)` | NULL | Previous opening balance before manual adjustment |
| `reason` | `text` | NULL | Reason for manual adjustment |
| `updated_by` | `uuid` | NULL, FK → `users.id` ON DELETE SET NULL | User who performed manual correction |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | |
| `updated_at` | `timestamptz` | NOT NULL, default `now()` | |

Indexes & Constraints:
- UNIQUE `(rt_id, period)`
- Index on `(rt_id)`

---

## 4.8 `rw_transactions` — RW-Level Independent Financial Ledger

| Column | Type | Constraints | Description |
|---|---|---|---|
| `id` | `uuid` | PK, default `gen_random_uuid()` | |
| `rw_id` | `uuid` | NOT NULL, FK → `rws.id` ON DELETE CASCADE | RW scope |
| `category_id` | `uuid` | NULL, FK → `financial_categories.id` ON DELETE SET NULL | Financial category |
| `account_id` | `uuid` | NULL, FK → `fund_locations.id` ON DELETE SET NULL | Target fund location |
| `amount` | `numeric(15,2)` | NOT NULL, CHECK `amount > 0` | Always positive |
| `type` | `text` | NOT NULL, CHECK IN (`income`, `expense`) | Direction |
| `status` | `text` | NOT NULL, default `'posted'`, CHECK IN (`draft`, `posted`, `cancelled`) | Immutability status |
| `reverses_transaction_id` | `uuid` | NULL, FK → `rw_transactions.id` ON DELETE RESTRICT | Reversal link |
| `payment_id` | `uuid` | NULL, FK → `payments.id` ON DELETE SET NULL | Payment link |
| `description` | `text` | NOT NULL | Audit ledger note |
| `occurred_at` | `timestamptz` | NOT NULL, default `now()` | Event date |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | |
| `updated_at` | `timestamptz` | NOT NULL, default `now()` | |
| `payment_method` | `text` | NULL, CHECK IN (`CASH`, `TRANSFER`) | Payment method |
| `created_by` | `uuid` | NULL, FK → `users.id` ON DELETE SET NULL | Officer who recorded the manual transaction |
| `expense_category` | `text` | NULL | Standardized expense category for expense transactions |
| `reference_no` | `text` | NULL | System-generated transaction reference number |

Indexes & Constraints:
- Index on `(rw_id)`
- Index on `(status)`
- Index on `(type)`
- Index on `(occurred_at)`
- Index on `(reverses_transaction_id)`
- Index on `(category_id)`
- Index on `(payment_id)`
- Index on `(created_by)`
- Partial unique index `uniq_rw_transactions_reference_no` on `(reference_no) WHERE reference_no IS NOT NULL`

**Key design:** RW-level financial transactions (income, expense, ledger) are stored in this **independent** table — NOT aggregated from RT transactions. RW balance (fund locations) still comes from aggregated RT `fund_locations`. This separation implements the "Koreksi Fundamental" principle: KAS RT and KAS RW are completely independent ledgers.

**Transaction lifecycle:** Same as `transactions`:
- `draft` — not yet posted, can be edited/deleted
- `posted` — finalized, visible in ledger
- `cancelled` — a reversal transaction referencing the original via `reverses_transaction_id`

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
| 020 | `020_payments_donations.up.sql` | Allows exactly one payment target (`bill_id` or voluntary `due_id`) and records payment submitter | false |
| 024 | `024_period_opening_balances.up.sql` | Creates `period_opening_balances` table for monthly balance and audit trail | false |
| 026 | `026_enrich_transactions_creator_and_household_updates.up.sql` | Adds `created_by` to `transactions` and allows editing household metadata | false |
| 027 | `027_rw_domain_and_transaction_reference.up.sql` | Creates `rws` domain table, adds `rts.rw_id` foreign key, and adds `transactions.reference_no` for auto-generated KAS transaction references | false |
| 032 | `032_dues_valid_until.up.sql` | Adds nullable `dues.valid_until` for voluntary-due auto-expiry plus a partial expiry index | false |
| 042 | `042_rw_transactions.up.sql` | Creates `rw_transactions` table — RW-level independent financial ledger separate from RT transactions (Brief 2: Koreksi Fundamental) | false |
| 049 | `049_bills_is_obligatory.up.sql` | Adds `is_obligatory` to `bills` and indexes obligation status based on `move_in_date` | false |

Migration execution: `docker compose run --rm migrate up`

---

## 5.1 RW Scope — Independent Ledger Architecture (Brief 2: Koreksi Fundamental)

**RW accounting is independent from RT accounting.** There is no aggregation in either direction.

RW-level operational endpoints (**GET /api/v1/rw/{rwId}/dashboard**, **/kas**, **/kas/transactions**, **/locations**) have a separate ledger for transactions:

**Data flow:**

```
RT transactions      →  NOT aggregated into RW
RT fund_locations    → JOIN rts.rw_id  →  RW dashboard total_balance & locations list
rw_transactions      → WHERE rw_id     →  RW dashboard total_income, total_expense, transaction_count
rw_transactions      → WHERE rw_id     →  RW KAS report (income/expense)
rw_transactions      → WHERE rw_id     →  RW transaction list (paginated)
```

**What is independent vs what is aggregated:**

| Data | Source | Why |
|------|--------|-----|
| RW balance | Aggregated RT `fund_locations` | RW's cash is physically held in RT locations |
| RW KAS (income/expense) | `rw_transactions` table | RW-level independent ledger |
| RW transaction history | `rw_transactions` table | RW-level independent ledger |
| RW locations list | Aggregated RT `fund_locations` | Locations are RT-level entities |

**Two independent ledgers:**

| Ledger | Table | Scoped By |
|--------|-------|-----------|
| KAS RT | `transactions` | `rt_id` |
| KAS RW | `rw_transactions` | `rw_id` |

**Key design:** RW-level financial transactions (income, expense, ledger) are stored in the **independent** `rw_transactions` table — NOT aggregated from RT transactions. RW balance (fund locations) still comes from aggregated RT `fund_locations`. This separation implements the "Koreksi Fundamental" principle: KAS RT and KAS RW are completely independent ledgers.

**Authorization chain:**
1. `RequireAuth` middleware populates `AuthContext.RWMemberships` from DB
2. `RequireRWScope(pool, cfg)` middleware validates `rwId` param against memberships + SCOPED_RW_IDS allowlist
3. `Service.checkRWAccess(ctx, rwID)` provides defense-in-depth: rejects if user has no active RW membership

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

---

## 12. Notification Schema

### 12.1 `user_devices` — FCM Device Registrations

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uuid` | PK, default `gen_random_uuid()` | Device identifier |
| `user_id` | `uuid` | NOT NULL, FK → `users.id` ON DELETE CASCADE | Device owner |
| `fcm_token` | `text` | NOT NULL | FCM registration token |
| `platform` | `text` | NOT NULL, CHECK in (`android`, `ios`, `web`) | Device platform |
| `device_id` | `text` | NULL | Unique hardware/installation ID |
| `is_active` | `boolean` | NOT NULL, default `true` | Active registration flag |
| `last_seen_at` | `timestamptz` | NOT NULL, default `now()` | Last activity timestamp |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | |
| `updated_at` | `timestamptz` | NOT NULL, default `now()` | |

Unique: `uq_user_devices_user_token (user_id, fcm_token)`.

### 12.2 `notification_logs` — Delivery Audit Logs & Deduplication

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uuid` | PK, default `gen_random_uuid()` | Log identifier |
| `rt_id` | `uuid` | NOT NULL, FK → `rts.id` ON DELETE CASCADE | Tenant scope |
| `user_id` | `uuid` | NULL, FK → `users.id` ON DELETE CASCADE | Targeted user (or NULL for RT-wide) |
| `sender_user_id` | `uuid` | NULL, FK → `users.id` ON DELETE SET NULL | Authenticated sender for audited waste-truck dispatches |
| `event_type` | `text` | NOT NULL | Event type identifier |
| `dedup_key` | `text` | NOT NULL, UNIQUE | Deduplication idempotency key |
| `title` | `text` | NOT NULL | Notification title sent |
| `body` | `text` | NOT NULL | Notification body sent |
| `image_url` | `text` | NULL | Public relative or absolute URL of attached image |
| `payload` | `jsonb` | NOT NULL, default `'{}'` | Non-sensitive data payload |
| `status` | `text` | NOT NULL, default `'SENT'` | Delivery status |
| `failure_reason` | `text` | NULL | Safe summary when waste-truck dispatch delivery fails or is partial |
| `created_at` | `timestamptz` | NOT NULL, default `NOW()` | Timestamp of dispatch |

Waste-truck dispatches use `sender_user_id` from the authenticated backend
context, begin with `PENDING`, then record `SENT`, `PARTIAL`, or `FAILED` after
the push provider returns. `failure_reason` contains only a safe summary, not
provider tokens or raw error details. Migration 058 backfills the sender from
legacy waste-truck `user_id` values and indexes sender-scoped history.

### 12.3 `notification_templates` — Configurable Notification Templates

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uuid` | PK, default `gen_random_uuid()` | Template identifier |
| `rt_id` | `uuid` | NOT NULL, FK → `rts.id` ON DELETE CASCADE | Tenant scope |
| `name` | `varchar(100)`| NOT NULL | Display name of template |
| `event_type` | `varchar(50)` | NOT NULL | Target notification event |
| `channel` | `varchar(20)` | NOT NULL, default `'push'`, CHECK in (`push`, `in_app`) | Delivery channel |
| `title` | `varchar(200)`| NOT NULL | Template title with optional `{{var}}` |
| `body` | `text` | NOT NULL | Template body with optional `{{var}}` |
| `image_url` | `text` | NULL | Optional image attachment URL (only for allowed events) |
| `is_active` | `boolean` | NOT NULL, default `true` | Active status toggle |
| `variables` | `jsonb` | NOT NULL, default `'[]'` | List of allowed variables |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | |
| `updated_at` | `timestamptz` | NOT NULL, default `now()` | |

Unique: `uq_rt_event_template_name (rt_id, event_type, name)`.
Index: `idx_notification_templates_event_active (rt_id, event_type, is_active)`.

### 12.4 `notification_reads` — Per-User Notification Read Receipts

Tracks read timestamps per user for notification logs, ensuring read receipts for personal notifications and RT-wide announcements remain isolated per user.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `notification_id` | `uuid` | NOT NULL, FK → `notification_logs.id` ON DELETE CASCADE | Associated notification log |
| `user_id` | `uuid` | NOT NULL, FK → `users.id` ON DELETE CASCADE | User who read the notification |
| `read_at` | `timestamptz` | NOT NULL, default `now()` | Timestamp when opened/marked as read |

Primary Key: `(notification_id, user_id)`.
Indexes:
- `idx_notification_reads_user_id (user_id)`
- `idx_notification_reads_notif_id (notification_id)`

### 12.5 `notification_schedules` — Scheduled Notification Triggers (Migration 052, 054, 059)

Stores recurring notification schedules for automated reminder dispatches (such as monthly iuran payment reminders, 3-month upcoming due reminders, and 3+ month overdue reminders).

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uuid` | PK, default `gen_random_uuid()` | Schedule identifier |
| `rt_id` | `uuid` | NOT NULL, FK → `rts.id` ON DELETE CASCADE | RT organization scope |
| `event_type` | `varchar(64)` | NOT NULL | Notification event type |
| `template_id` | `uuid` | NOT NULL, FK → `notification_templates.id` ON DELETE RESTRICT | Linked template |
| `name` | `varchar(128)` | NOT NULL | Friendly schedule name |
| `is_active` | `boolean` | NOT NULL, default `true` | Active status |
| `send_days` | `jsonb` | NOT NULL, default `'[]'` | Days of month (1–31) |
| `send_hour` | `integer` | NOT NULL, CHECK (0–23) | Send hour in schedule timezone |
| `send_minute` | `integer` | NOT NULL, CHECK (0–59) | Send minute in schedule timezone |
| `timezone` | `varchar(64)` | NOT NULL, default `'Asia/Jakarta'` | Timezone of execution |
| `last_executed_at` | `timestamptz` | NULL | Timestamp of last execution |
| `last_executed_status` | `varchar(16)` | NULL | Aggregated outcome (`success`, `partial`, `failed`, `no_device`, `no_recipients`) |
| `last_executed_error` | `text` | NULL | Safe error/failure explanation for UI display |
| `last_executed_summary` | `jsonb` | NULL | Detailed recipient delivery metrics (`total_recipients`, `sent_count`, `failed_count`, `no_device_count`, `failure_reason`) |
| `is_default` | `boolean` | NOT NULL, default `false` | System default schedule flag |
| `created_by` | `uuid` | NULL, FK → `users.id` | Creator user |
| `updated_by` | `uuid` | NULL, FK → `users.id` | Updater user |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | Audit timestamp |
| `updated_at` | `timestamptz` | NOT NULL, default `now()` | Audit timestamp |

### 12.6 `notification_schedule_executions` — Schedule Configuration Versioning & Execution Records (Migration 060)

Tracks execution instances of notification schedules per schedule, local execution date, and canonical configuration version hash (`config_hash`). Enforces at most one delivery per configuration version per date while allowing same-day schedule updates to re-execute safely at their new scheduled times.

| Column | Type | Constraints | Description |
|---|---|---|---|
| `id` | `uuid` | PK, default `gen_random_uuid()` | Execution identifier |
| `schedule_id` | `uuid` | NOT NULL, FK → `notification_schedules.id` ON DELETE CASCADE | Target schedule |
| `rt_id` | `uuid` | NOT NULL, FK → `rts.id` ON DELETE CASCADE | RT organization scope |
| `exec_date` | `date` | NOT NULL | Local execution calendar date (YYYY-MM-DD) |
| `config_hash` | `varchar(64)` | NOT NULL | Canonical SHA-256 hash of delivery configuration |
| `status` | `varchar(32)` | NOT NULL, default `'success'` | Outcome (`success`, `partial`, `failed`, `no_device`, `no_recipients`) |
| `error_detail` | `text` | NULL | Safe error description |
| `summary` | `jsonb` | NOT NULL, default `'{}'` | Delivery metrics (`total_recipients`, `sent_count`, etc.) |
| `executed_at` | `timestamptz` | NOT NULL, default `now()` | Wall-clock execution timestamp |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | Record creation timestamp |
| `updated_at` | `timestamptz` | NOT NULL, default `now()` | Record update timestamp |

Constraints:
- `uq_schedule_execution UNIQUE (schedule_id, exec_date, config_hash)`

---

## 13. RW Finance & Fund Locations (Migration 042, 043, 045)

### 13.1 `rw_transactions` — RW Financial Ledger

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uuid` | PK, default `gen_random_uuid()` | Transaction identifier |
| `rw_id` | `uuid` | NOT NULL, FK → `rws.id` ON DELETE RESTRICT | RW organization scope |
| `reference_no` | `varchar(50)` | NULL | Unique reference number |
| `type` | `varchar(20)` | NOT NULL, CHECK (`income`, `expense`) | Transaction direction |
| `amount` | `numeric(15, 2)` | NOT NULL, CHECK (`amount > 0`) | Transaction amount |
| `description` | `text` | NOT NULL | Transaction description |
| `category_id` | `uuid` | NULL, FK → `categories.id` ON DELETE SET NULL | Category classification |
| `payment_method` | `varchar(20)` | NOT NULL, default `'cash'`, CHECK (`cash`, `transfer`, `other`) | Payment method |
| `status` | `varchar(20)` | NOT NULL, default `'posted'`, CHECK (`draft`, `posted`, `cancelled`) | Immutability status |
| `location_id` | `uuid` | NULL, FK → `(location_id, rw_id)` REFERENCES `rw_fund_locations` | Source/destination fund location |
| `source_rt_id` | `uuid` | NULL, FK → `rts.id` ON DELETE SET NULL | Source RT for incoming dues/funds |
| `proof_path` | `text` | NULL | Uploaded receipt/transaction proof file path |
| `proof_original_name` | `text` | NULL | Original filename of uploaded proof |
| `proof_mime_type` | `varchar(100)` | NULL | MIME type of proof image |
| `proof_size_bytes` | `bigint` | NULL | File size in bytes |
| `proof_uploaded_at` | `timestamptz` | NULL | Upload timestamp |
| `proof_uploaded_by` | `uuid` | NULL, FK → `users.id` | User who uploaded proof |
| `created_by` | `uuid` | NULL, FK → `users.id` ON DELETE SET NULL | Officer recording transaction |
| `transaction_date` | `date` | NOT NULL, default `CURRENT_DATE` | Date of financial event |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | Audit timestamp |

Indexes:
- `idx_rw_transactions_rw_date (rw_id, transaction_date)`
- `idx_rw_transactions_location_id (location_id)`
- `idx_rw_transactions_source_rt_id (source_rt_id)`
- `uniq_rw_transactions_reference_no (reference_no) WHERE reference_no IS NOT NULL`

### 13.2 `rw_fund_locations` — RW Fund Storage Locations

Supports 3 distinct categories: `rekening` (Rekening Bank), `brankas` (Brankas Fisik), and `coh` (Kas Di Tangan / Cash on Hand). The concept of "Dana Hibah/Donasi" is eliminated.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uuid` | PK, default `gen_random_uuid()` | Location identifier |
| `rw_id` | `uuid` | NOT NULL, FK → `rws.id` ON DELETE RESTRICT | RW organization scope |
| `name` | `varchar(100)` | NOT NULL | Location display name |
| `category` | `varchar(20)` | NOT NULL, CHECK in (`rekening`, `brankas`, `coh`) | Storage category |
| `bank_name` | `varchar(50)` | NULL | Bank name (required for `rekening`) |
| `account_number` | `varchar(50)` | NULL | Bank account number |
| `account_holder` | `varchar(100)` | NULL | Account holder name |
| `is_active` | `boolean` | NOT NULL, default `true` | Soft toggle (deactivation requires balance = 0) |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | Creation timestamp |
| `updated_at` | `timestamptz` | NOT NULL, default `now()` | Last updated timestamp |

Unique: `uniq_rw_fund_locations_rw_name (rw_id, name)`.

### 13.3 `rw_fund_transfers` — Intra-RW Fund Transfers

Transfers funds between locations within the same RW organization. Blocked across different RWs.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uuid` | PK, default `gen_random_uuid()` | Transfer identifier |
| `rw_id` | `uuid` | NOT NULL, FK → `rws.id` ON DELETE RESTRICT | RW organization scope |
| `from_location_id` | `uuid` | NOT NULL, FK → `(from_location_id, rw_id)` REFERENCES `rw_fund_locations` | Source location |
| `to_location_id` | `uuid` | NOT NULL, FK → `(to_location_id, rw_id)` REFERENCES `rw_fund_locations` | Destination location |
| `amount` | `numeric(15, 2)` | NOT NULL, CHECK (`amount > 0`) | Transfer amount |
| `transfer_date` | `date` | NOT NULL | Date of transfer |
| `notes` | `text` | NULL | Transfer remarks |
| `created_by` | `uuid` | NULL, FK → `users.id` ON DELETE SET NULL | Initiating officer |
| `reverses_transfer_id` | `uuid` | NULL, FK → `rw_fund_transfers.id` | Reversed original transfer |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | Timestamp |

Constraints:
- `chk_rw_fund_transfers_diff_locations (from_location_id != to_location_id)`

### 13.4 `rw_location_balance_changes` — RW Location Mutation Ledger

Append-only immutable balance changes for RW fund locations.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uuid` | PK, default `gen_random_uuid()` | Mutation identifier |
| `rw_id` | `uuid` | NOT NULL, FK → `rws.id` ON DELETE RESTRICT | RW organization scope |
| `location_id` | `uuid` | NOT NULL, FK → `(location_id, rw_id)` REFERENCES `rw_fund_locations` | Targeted location |
| `transaction_id` | `uuid` | NULL, FK → `rw_transactions.id` | Associated KAS transaction |
| `transfer_id` | `uuid` | NULL, FK → `rw_fund_transfers.id` | Associated transfer |
| `change_type` | `text` | NOT NULL | Mutation type |
| `amount_delta` | `numeric(15, 2)` | NOT NULL, CHECK (`amount_delta != 0`) | Positive (credit) or negative (debit) |
| `notes` | `text` | NULL | Mutation notes |
| `created_by` | `uuid` | NULL, FK → `users.id` ON DELETE SET NULL | Author |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | Timestamp |

## 14. Resident Password Reset Codes (Migration 048)

Enables RT Pengurus to generate temporary 4-digit numeric reset codes for residents in their RT. Codes are hashed in the database, valid for 24 hours, strictly single-use, and rate-limited.

### 14.1 `resident_password_reset_codes`

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uuid` | PK, default `gen_random_uuid()` | Reset request identifier |
| `user_id` | `uuid` | NOT NULL, FK → `users.id` ON DELETE CASCADE | Associated user account |
| `resident_id` | `uuid` | NOT NULL, FK → `residents.id` ON DELETE CASCADE | Target resident record |
| `rt_id` | `uuid` | NOT NULL, FK → `rts.id` ON DELETE CASCADE | Scoped RT tenant |
| `code_hash` | `text` | NOT NULL | Argon2id hash of the 4-digit code |
| `expires_at` | `timestamptz` | NOT NULL | Expiration timestamp (default now() + 24 hours) |
| `used_at` | `timestamptz` | NULL | Timestamp when code was successfully redeemed |
| `created_by` | `uuid` | NOT NULL, FK → `users.id` ON DELETE RESTRICT | Pengurus or Admin who issued the code |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | Creation timestamp |

Indexes:
- `idx_reset_codes_user_id (user_id)`
- `idx_reset_codes_resident_id (resident_id)`
- `idx_reset_codes_rt_id (rt_id)`
- `idx_reset_codes_active (user_id, expires_at) WHERE used_at IS NULL`

---

## 15. Security Attendance Module (Migration 077)

All Security Attendance tables are RT-scoped (`rt_id` FK -> `rts(id)` ON DELETE RESTRICT). Shift schedules and attendance records store immutable shift snapshots (`shift_version`, times, tolerances, computed WIB boundaries) so editing shift configurations only affects future schedules/attendance and never alters historical records.

### 15.1 `security_devices` — Registered Security Pos Devices

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uuid` | PK, default `gen_random_uuid()` | Device identifier |
| `rt_id` | `uuid` | NOT NULL, FK → `rts.id` ON DELETE RESTRICT | Bound RT tenant |
| `device_name` | `text` | NOT NULL | Human-readable pos device name |
| `location_label` | `text` | NOT NULL, default `'Pos Keamanan Utama'` | Guard post location label |
| `activation_code_hash` | `text` | NULL | SHA-256 hash of single-use pairing secret |
| `activation_expires_at` | `timestamptz` | NULL | Pairing token expiration |
| `token_hash` | `text` | NULL | SHA-256 hash of active device access credential |
| `is_active` | `boolean` | NOT NULL, default `true` | Active/revoked state |
| `activated_at` | `timestamptz` | NULL | First successful activation timestamp |
| `last_active_at` | `timestamptz` | NULL | Latest authenticated device heartbeat/request |
| `revoked_at` | `timestamptz` | NULL | Revocation timestamp |
| `revoked_by` | `uuid` | NULL, FK → `users.id` ON DELETE SET NULL | Revoking administrator |
| `created_by` | `uuid` | NOT NULL, FK → `users.id` ON DELETE RESTRICT | Provisioning administrator |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | |
| `updated_at` | `timestamptz` | NOT NULL, default `now()` | |

Indexes:
- `idx_security_devices_rt_active (rt_id, is_active)`
- `uniq_security_devices_token_hash UNIQUE (token_hash) WHERE token_hash IS NOT NULL`

### 15.2 `security_shift_configs` — Flexible Shift Definitions (N Shifts)

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uuid` | PK, default `gen_random_uuid()` | Shift config ID |
| `rt_id` | `uuid` | NOT NULL, FK → `rts.id` ON DELETE RESTRICT | RT tenant |
| `name` | `text` | NOT NULL | Shift display name |
| `start_time` | `time` | NOT NULL | Shift start time (Asia/Jakarta wall clock) |
| `end_time` | `time` | NOT NULL | Shift end time (Asia/Jakarta wall clock) |
| `crosses_midnight` | `boolean` | NOT NULL, default `false` | True when shift crosses midnight |
| `lateness_tolerance_minutes` | `int` | NOT NULL, default `15`, CHECK `0..180` | Tolerance before marked `terlambat` |
| `check_in_window_before_minutes` | `int` | NOT NULL, default `60`, CHECK `0..360` | Minutes before start when check-in opens |
| `check_in_window_after_minutes` | `int` | NOT NULL, default `120`, CHECK `0..480` | Minutes after start when check-in remains open |
| `absent_cutoff_minutes` | `int` | NOT NULL, default `120`, CHECK `0..480` | Minutes after start before scheduler marks `tidak_hadir` |
| `missing_checkout_cutoff_minutes` | `int` | NOT NULL, default `180`, CHECK `0..720` | Minutes after end before scheduler marks `belum_absen_pulang` |
| `version` | `int` | NOT NULL, default `1`, CHECK `version >= 1` | Incremented on every update for historical immutability |
| `is_active` | `boolean` | NOT NULL, default `true` | Active toggle |
| `created_by` | `uuid` | NOT NULL, FK → `users.id` ON DELETE RESTRICT | |
| `updated_by` | `uuid` | NULL, FK → `users.id` ON DELETE SET NULL | |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | |
| `updated_at` | `timestamptz` | NOT NULL, default `now()` | |

Indexes:
- `uniq_security_shift_configs_rt_name_active UNIQUE (rt_id, LOWER(name)) WHERE is_active = true`
- `idx_security_shift_configs_rt_active (rt_id, is_active)`

### 15.3 `security_schedules` — Officer Shift Assignments

Migration 078 adds `assignment_id`, `assignment_start_date`, and `assignment_end_date` to this daily schedule table. One assignment period is represented by one row per local shift date; `security_schedules` remains the sole source of truth, while `security_attendance_records` remains one-to-one with each daily row. Create/edit and attendance initialization are transactional. Existing records migrate as one-day assignments.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uuid` | PK, default `gen_random_uuid()` | Daily schedule ID |
| `assignment_id` | `uuid` | NOT NULL | Shared identifier grouping daily rows for a multi-date assignment |
| `assignment_start_date` | `date` | NOT NULL | Inclusive assignment start date |
| `assignment_end_date` | `date` | NOT NULL | Inclusive assignment end date |
| `rt_id` | `uuid` | NOT NULL, FK → `rts.id` ON DELETE RESTRICT | RT tenant |
| `user_id` | `uuid` | NOT NULL, FK → `users.id` ON DELETE RESTRICT | Assigned security officer |
| `membership_id` | `uuid` | NOT NULL, FK → `user_rt_memberships.id` ON DELETE RESTRICT | Active officer RT membership |
| `shift_config_id` | `uuid` | NOT NULL, FK → `security_shift_configs.id` ON DELETE RESTRICT | Source shift template |
| `shift_version` | `int` | NOT NULL, default `1` | Snapshot shift version |
| `shift_date` | `date` | NOT NULL | Shift start calendar date (`Asia/Jakarta`) |
| `shift_name_snapshot` | `text` | NOT NULL | Immutable shift name snapshot |
| `start_time_snapshot` | `time` | NOT NULL | Immutable start time snapshot |
| `end_time_snapshot` | `time` | NOT NULL | Immutable end time snapshot |
| `crosses_midnight_snapshot` | `boolean` | NOT NULL, default `false` | Immutable midnight-crossing flag |
| `lateness_tolerance_minutes` | `int` | NOT NULL | Snapshot tolerance |
| `check_in_window_before_minutes` | `int` | NOT NULL | Snapshot check-in open offset |
| `check_in_window_after_minutes` | `int` | NOT NULL | Snapshot check-in close offset |
| `absent_cutoff_minutes` | `int` | NOT NULL | Snapshot absent cutoff |
| `missing_checkout_cutoff_minutes` | `int` | NOT NULL | Snapshot missing checkout cutoff |
| `scheduled_start_at` | `timestamptz` | NOT NULL | Exact UTC/WIB instant for shift start |
| `scheduled_end_at` | `timestamptz` | NOT NULL | Exact UTC/WIB instant for shift end |
| `notes` | `text` | NULL | Optional schedule notes |
| `status` | `text` | NOT NULL, default `'scheduled'`, CHECK IN `('scheduled', 'cancelled')` | Lifecycle state |
| `created_by` | `uuid` | NOT NULL, FK → `users.id` ON DELETE RESTRICT | |
| `updated_by` | `uuid` | NULL, FK → `users.id` ON DELETE SET NULL | |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | |
| `updated_at` | `timestamptz` | NOT NULL, default `now()` | |

Indexes:
- `uniq_security_schedules_active_slot UNIQUE (rt_id, user_id, shift_date, shift_config_id) WHERE status = 'scheduled'`
- `idx_security_schedules_rt_date (rt_id, shift_date, status)`
- `idx_security_schedules_user_window (user_id, scheduled_start_at, scheduled_end_at)`

### 15.4 `security_shift_swaps` — Two-Step Shift Swap Requests

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uuid` | PK, default `gen_random_uuid()` | Swap request ID |
| `rt_id` | `uuid` | NOT NULL, FK → `rts.id` ON DELETE RESTRICT | RT tenant |
| `requester_user_id` | `uuid` | NOT NULL, FK → `users.id` ON DELETE RESTRICT | Proposing officer |
| `target_user_id` | `uuid` | NOT NULL, FK → `users.id` ON DELETE RESTRICT | Counterpart officer |
| `requester_schedule_id` | `uuid` | NOT NULL, FK → `security_schedules.id` ON DELETE RESTRICT | Requester's shift |
| `target_schedule_id` | `uuid` | NOT NULL, FK → `security_schedules.id` ON DELETE RESTRICT | Target officer's shift |
| `reason` | `text` | NOT NULL | Swap reason |
| `status` | `text` | NOT NULL, default `'pending_target'`, CHECK IN `('pending_target', 'rejected_target', 'pending_ketua', 'approved', 'rejected_ketua', 'cancelled')` | Workflow state |
| `target_responded_at` | `timestamptz` | NULL | Target officer response timestamp |
| `target_note` | `text` | NULL | Optional target officer note |
| `ketua_decided_by` | `uuid` | NULL, FK → `users.id` ON DELETE SET NULL | Ketua RT / Super Admin final approver |
| `ketua_decided_at` | `timestamptz` | NULL | Final decision timestamp |
| `ketua_note` | `text` | NULL | Optional approval/rejection note |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | |
| `updated_at` | `timestamptz` | NOT NULL, default `now()` | |

### 15.5 `security_qr_tokens` — Dynamic Single-Use QR Tokens

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uuid` | PK, default `gen_random_uuid()` | Token ID |
| `rt_id` | `uuid` | NOT NULL, FK → `rts.id` ON DELETE RESTRICT | Bound RT |
| `device_id` | `uuid` | NOT NULL, FK → `security_devices.id` ON DELETE RESTRICT | Issuing Pos device |
| `token_hash` | `text` | NOT NULL, UNIQUE | SHA-256 hash of raw QR token |
| `expires_at` | `timestamptz` | NOT NULL | Server expiry (`Asia/Jakarta` UTC instant) |
| `used_at` | `timestamptz` | NULL | Populated atomically via `SELECT ... FOR UPDATE` on first consume |
| `used_by_user_id` | `uuid` | NULL, FK → `users.id` ON DELETE SET NULL | Officer who redeemed the token |
| `used_for_action` | `text` | NULL, CHECK IN `('check_in', 'check_out')` | Redemption action |
| `invalidated_at` | `timestamptz` | NULL | Set if device revoked before expiration |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | |

### 15.6 `security_attendance_records` — Immutable Attendance Ledger

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uuid` | PK, default `gen_random_uuid()` | Attendance record ID |
| `rt_id` | `uuid` | NOT NULL, FK → `rts.id` ON DELETE RESTRICT | RT tenant |
| `schedule_id` | `uuid` | NOT NULL, UNIQUE, FK → `security_schedules.id` ON DELETE RESTRICT | One record per schedule |
| `user_id` | `uuid` | NOT NULL, FK → `users.id` ON DELETE RESTRICT | Officer |
| `shift_config_id` | `uuid` | NOT NULL, FK → `security_shift_configs.id` ON DELETE RESTRICT | Shift reference |
| `shift_version` | `int` | NOT NULL | Snapshot version |
| `shift_date` | `date` | NOT NULL | Shift start date (`Asia/Jakarta`) |
| `status` | `text` | NOT NULL, default `'belum_hadir'`, CHECK IN `('belum_hadir', 'hadir', 'terlambat', 'sudah_absen_pulang', 'tidak_hadir', 'belum_absen_pulang')` | Current attendance state |
| `arrival_status` | `text` | NULL, CHECK IN `('hadir', 'terlambat')` | Preserved arrival punctuality after checkout |
| `late_minutes` | `int` | NOT NULL, default `0`, CHECK `late_minutes >= 0` | Minutes late past start + tolerance |
| `check_in_device_id` | `uuid` | NULL, FK → `security_devices.id` ON DELETE SET NULL | Pos device used for check-in |
| `check_out_device_id` | `uuid` | NULL, FK → `security_devices.id` ON DELETE SET NULL | Pos device used for check-out |
| `check_in_qr_token_id` | `uuid` | NULL, FK → `security_qr_tokens.id` ON DELETE SET NULL | Redeemed check-in QR token |
| `check_out_qr_token_id` | `uuid` | NULL, FK → `security_qr_tokens.id` ON DELETE SET NULL | Redeemed check-out QR token |
| `checked_in_at` | `timestamptz` | NULL | Authoritative server check-in time |
| `checked_out_at` | `timestamptz` | NULL | Authoritative server check-out time |
| `marked_absent_at` | `timestamptz` | NULL | Scheduler timestamp when marked `tidak_hadir` |
| `marked_missing_checkout_at` | `timestamptz` | NULL | Scheduler timestamp when marked `belum_absen_pulang` |
| `correction_locked` | `boolean` | NOT NULL, default `false` | Prevents scheduler from overwriting an approved correction |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | |
| `updated_at` | `timestamptz` | NOT NULL, default `now()` | |

### 15.7 `security_attendance_corrections` — Maker-Checker Attendance Corrections

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uuid` | PK, default `gen_random_uuid()` | Correction ID |
| `rt_id` | `uuid` | NOT NULL, FK → `rts.id` ON DELETE RESTRICT | RT tenant |
| `attendance_record_id` | `uuid` | NOT NULL, FK → `security_attendance_records.id` ON DELETE RESTRICT | Target attendance record |
| `schedule_id` | `uuid` | NOT NULL, FK → `security_schedules.id` ON DELETE RESTRICT | Associated schedule |
| `officer_user_id` | `uuid` | NOT NULL, FK → `users.id` ON DELETE RESTRICT | Subject officer |
| `requested_by` | `uuid` | NOT NULL, FK → `users.id` ON DELETE RESTRICT | Submitter |
| `request_type` | `text` | NOT NULL, CHECK IN `('status_change', 'time_adjustment', 'manual_check_in', 'manual_check_out')` | Correction kind |
| `original_status` | `text` | NOT NULL | Original status before correction |
| `original_checked_in_at` | `timestamptz` | NULL | Original check-in time |
| `original_checked_out_at` | `timestamptz` | NULL | Original check-out time |
| `requested_status` | `text` | NOT NULL, CHECK IN `('belum_hadir', 'hadir', 'terlambat', 'sudah_absen_pulang', 'tidak_hadir', 'belum_absen_pulang')` | Requested final status |
| `requested_checked_in_at` | `timestamptz` | NULL | Requested check-in time |
| `requested_checked_out_at` | `timestamptz` | NULL | Requested check-out time |
| `reason` | `text` | NOT NULL | Mandatory justification |
| `status` | `text` | NOT NULL, default `'pending'`, CHECK IN `('pending', 'approved', 'rejected')` | Approval status |
| `decided_by` | `uuid` | NULL, FK → `users.id` ON DELETE SET NULL | Approver (`decided_by != requested_by`) |
| `decided_at` | `timestamptz` | NULL | Decision timestamp |
| `decision_note` | `text` | NULL | Approver remarks |
| `final_status` | `text` | NULL | Applied final status when approved |
| `final_checked_in_at` | `timestamptz` | NULL | Applied final check-in time when approved |
| `final_checked_out_at` | `timestamptz` | NULL | Applied final check-out time when approved |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | |
| `updated_at` | `timestamptz` | NOT NULL, default `now()` | |

### 15.8 `security_guest_books` — Security Pos Guest Book

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `uuid` | PK, default `gen_random_uuid()` | Entry ID |
| `rt_id` | `uuid` | NOT NULL, FK → `rts.id` ON DELETE RESTRICT | Bound RT |
| `device_id` | `uuid` | NOT NULL, FK → `security_devices.id` ON DELETE RESTRICT | Check-in Pos device |
| `guest_name` | `text` | NOT NULL | Visitor name |
| `phone` | `text` | NULL | Visitor contact number (PII-protected in logs/masked where appropriate) |
| `purpose` | `text` | NOT NULL | Visit purpose |
| `host_household_id` | `uuid` | NULL, FK → `household_occupancies.id` ON DELETE SET NULL | Optional visited household |
| `destination_label` | `text` | NOT NULL | Destination house/person |
| `vehicle_plate` | `text` | NULL | Optional plate number |
| `notes` | `text` | NULL | Optional remarks |
| `checked_in_at` | `timestamptz` | NOT NULL, default `now()` | Check-in timestamp |
| `checked_out_at` | `timestamptz` | NULL | Check-out timestamp |
| `checkout_device_id` | `uuid` | NULL, FK → `security_devices.id` ON DELETE SET NULL | Check-out Pos device |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | |
| `updated_at` | `timestamptz` | NOT NULL, default `now()` | |
