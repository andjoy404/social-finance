# Social Finance - Development Plan

## 1. Development Strategy

The project follows a **vertical slice** approach: each phase delivers a working feature (including database, API, and a basic test) before moving to the next. This ensures that each part of the system is fully functional and testable before adding complexity.

### Principles

1. **Backend first:** Backend APIs are developed and tested before the Android client.
2. **Vertical slices:** Each module includes migration, repository, service, API handler, and unit tests.
3. **Incremental delivery:** Each phase results in a deployable, testable build.
4. **Tests alongside code:** Unit and integration tests are written for each module as it's built.
5. **No feature without tests:** Every feature is accompanied by corresponding tests.

---

## 2. Phase Overview

| Phase | Title | Scope | Status |
|-------|-------|-------|--------|
| 0 | Requirements & Architecture | This document | COMPLETE |
| 1 | Backend Foundation | Infra, DB, migrations, health check | COMPLETE |
| 2 | Authentication & Authorization | Login, JWT, RBAC | COMPLETE |
| 2.5 | Backend Hardening | API errors, validation, logging, pagination, CI | In Progress |
| 3 | RT, Household, Resident Management | CRUD for tenants and people | COMPLETE |
| 4 | Financial Categories & Ledger | Income, expenses, transactions, audit | Planned |
| 5 | Dues & Monthly Billing | Monthly household billing, dues config | Planned |
| 6 | Payments | Payment recording, linkage to bills | Planned |
| 7 | Arrears & Service Operations | Derived arrears, garbage collection eligibility | Planned |
| 8 | General Income & Categorization | Non-regular income, transaction categories | Planned |
| 9 | Dashboard & Reports | Summary views, reports | Planned |
| 10 | Ops — Operator Accounts & View | Non-resident accounts, garbage ops | Planned |
| 11 | Web Enhancements | Warga arrears badges, filters, household summary | Planned |
| 12 | Android Foundation | Auth state, API client, navigation | Planned |
| 13 | Android Integration | Feature-by-feature mobile UI | Planned |
| 14 | Testing & Hardening | Security audit, performance, E2E | Planned |
| 15 | Deployment | VPS, Docker, CI/CD | Planned |

---

## 3. Detailed Phases

### Phase 0: Requirements & Architecture

**Complete.** This step produces the documentation files in `docs/`.

---

### Phase 1: Backend Foundation

**IMPLEMENTED AND RUNTIME VERIFIED.** Completed on 2026-09-16.

**Status:** All tasks complete. The entire stack builds and is runtime verified:
migration runner applies DDL, health check returns correct status under all conditions,
tests pass (9 tests), Air hot reload recompiles on source changes, production image
runs as non-root `appuser` (Alpine, 30MB), database persistence survives `docker compose down`,
air hot reload, graceful restart.

**Runtime validation performed:**
1. `docker compose build` — backend + migrate images build successfully
2. `docker compose run --rm migrate up` — migration `001_initial.up.sql` applied successfully
3. `docker compose run --rm migrate up` (second run) — reports `no change` (idempotent)
4. `docker compose run --rm migrate down 1` — `rts` table removed as expected
5. `docker compose run --rm migrate up` — `rts` restored, migration version returns to 1
6. Database inspection: version=1, dirty=false, rts table with correct schema
7. `curl http://localhost:8080/health` — returns HTTP 200 `{"database":"healthy","status":"ok"}`
8. PostgreSQL stopped → `/health` returns HTTP 503 `{"database":"unhealthy","status":"unhealthy"}`
9. PostgreSQL restarted → `/health` returns HTTP 200 (automatic recovery)
10. `docker compose down` + `docker compose up -d` — schema persisted in named volume
11. `docker compose run --rm backend go test` — all 9 tests pass
12. `docker compose run --rm backend go vet` — all packages clean
13. Air hot reload — `touch` triggers recompilation, binary updated, server survives
14. `docker compose restart backend` — graceful shutdown/startup (SIGTERM handled)
15. Production image build — 30MB Alpine, non-root `appuser`, Air/golang NOT included
16. Security: `.env` git-ignored, PG bound to 127.0.0.1:5432, no secrets in Dockerfiles

**Migrate runner implementation:** Uses golang-migrate v4.18 built from source with the
`postgres` build tag to include the PostgreSQL database driver. A wrapper script
(`migrate-wrapper.sh`) supplies default `-source` and `-database` flags so that
`docker compose run --rm migrate [command]` works without specifying connection details
each time. The Dockerfile.migrate uses `go install -tags 'postgres'` to embed the
`github.com/lib/pq` driver.

**Completed tasks:**
- [x] Initialize Go project (`go mod init social-finance`, Go 1.23)
- [x] Configure Chi HTTP router
- [x] Configure PostgreSQL connection pool (pgx/v5, max 20 conns, 10-minute lifetime)
- [x] Implement configuration loading from environment variables (`.env`)
  - `HTTP_PORT`: defaults to 8080 if absent; returns error if present but not a valid integer
  - Required vars (`DB_HOST`): exits with error if absent
  - Optional vars (`DB_PORT`, `DB_NAME`, etc.): default to documented values
- [x] Implement database migration runner (golang-migrate v4.18, Docker-wrapped)
  - PostgreSQL driver embedded via `-tags 'postgres'` at build time
  - Wrapper script provides connection defaults via Docker Compose env vars
  - Migration command: `docker compose run --rm migrate [up|down|version|status]`
- [x] Create initial migration: `rts` table with indexes
- [x] Implement health check endpoint (`GET /health`) with database connectivity check
  - Returns 200 `{"status":"ok","database":"healthy"}` when DB is reachable
  - Returns 503 `{"status":"unhealthy","database":"unhealthy"}` when DB is unreachable
- [x] Build and test with local Docker Compose (docker-compose.yml)
- [x] Write `Dockerfile.dev` (builds binary to /tmp/build/api, entrypoint copies to ./tmp/api,
  installs Air; runs pre-compiled binary as CMD, Air available for hot reload)
- [x] Write `Dockerfile` (multi-stage production build, Alpine, non-root user)
- [x] Write `Makefile` for dev convenience (`up`, `down`, `migrate-up`, `migrate-down`,
  `test`, `rebuild`, `clean`)
- [x] Configure `.gitignore` for project
- [x] Write `.env.example` with documented placeholder values
- [x] Write unit and integration tests (9 tests, all passing)
  - Config: 5 tests (defaults, env override, error on invalid input)
  - Health: 2 tests (healthy and unhealthy DB states)
  - Router: 2 tests (health route registered, 404 fallback)
  - Middleware: 2 tests (request ID, panic recovery, logging)
- [x] Docker Compose network: `social-finance-internal` (bridge)
- [x] Docker Compose volumes: `postgres_data` (named volume for DB persistence)
- [x] Health check handler: returns `"status":"unhealthy"` when database is unavailable
- [x] Config validation: invalid `HTTP_PORT` returns error (not silent fallback)
- [x] Graceful shutdown handling (SIGINT/SIGTERM)

---

### Phase 2: Authentication & Authorization

**Goal:** Users can log in, get tokens, refresh tokens, access protected endpoints with role-based and tenant-scoped authorization.

**Status: IMPLEMENTED AND RUNTIME VERIFIED.** Completed on 2026-09-16.

**Auth test count:** 78 total (66 in auth package, 12 in other packages).

**Completed tasks:**
- [x] Implement password hashing: Argon2id, cost 12-128 (`internal/auth/hash.go`)
- [x] Implement JWT access tokens: HS256/RS256, 15-minute lifetime, claims include sub, sys_role, role, rt_id, membership_id (`internal/auth/jwt.go`)
- [x] Implement refresh tokens: opaque random strings (32 bytes), server-side hash store, rotation/revocation (`internal/auth/token.go`, `internal/auth/service.go`)
- [x] Create `/api/v1/auth/login` endpoint with tenant context from JWT (`internal/auth/handlers.go`)
- [x] Create `/api/v1/auth/refresh` endpoint with token rotation (`internal/auth/handlers.go`)
- [x] Create `/api/v1/auth/logout` endpoint (`internal/auth/handlers.go`)
- [x] Create `/api/v1/auth/me` endpoint (`internal/auth/handlers.go`)
- [x] Implement role-based auth middleware: `RequireRole(role)` checks tenant role (`internal/auth/middleware.go`)
- [x] Implement system role middleware: `RequireSystemRole(sysRole)` for platform-level endpoints (`internal/auth/middleware.go`)
- [x] Implement tenant isolation: AuthContext derived from JWT claims, never from client input (`internal/auth/models.go`)
- [x] Implement login rate limiting: 10 requests per 5 minutes per source IP (`internal/auth/rate_limiter.go`)
- [x] Implement refresh rate limiting: 30 requests per 15 minutes per source IP (`internal/auth/rate_limiter.go`)
- [x] Implement bootstrap: `docker compose run --rm backend server --bootstrap` creates first super_admin (`cmd/api/main.go`)
- [x] Write auth unit tests (66 tests): password hashing, JWT generation/validation, token rotation, refresh token cleanup, middleware behavior, tenant safety
- [x] Write auth API tests: login, refresh, logout, /me, rate limiting, tenant isolation
- [x] Write tenant isolation tests (RT A/B cross-access, 12 tests)
- [x] Write authorization tests (WARGA cannot access admin endpoints)
- [x] Write JWT tampering tests (role/rt/membership tampering → 401)
- [x] Write RequireRole bypass tests (system-only SUPER_ADMIN → 403)

**NOT implemented (deferred):**
- `/api/v1/auth/change-password` — deferred to later phase
- Password change detection (password_changed_at column exists, logic deferred)

**Runtime validation:** All 78 tests passing (go test ./...), race detector clean, vet clean. Login, refresh, /me verified via Docker containers. Cross-tenant isolation verified (RT A cannot access RT B data). JWT tampering rejection verified.

---

### Phase 2.5: Backend Hardening

**Goal:** Establish minimal foundations that prevent Phase 3 from becoming inconsistent: standard API error responses, request validation, structured logging, pagination convention, CI pipeline.

**Status:** In Progress (documentation reconciliation complete)

**Tasks:**
- [x] Standardized API error responses (`error.code`, `error.message`, `error.details`)
- [x] Reusable request validation approach
- [x] Structured logging with `log/slog`
- [x] Pagination convention (`page`, `page_size`, defaults, max 100)
- [ ] CI pipeline: test + build jobs
- [x] Documentation reconciliation (README, api.md, database.md, current-project-state)

---

### Phase 3: RT, Household, Resident Management

**Goal:** SUPER_ADMIN can create RTs. PENGURUS/BENDAHARA can manage households and residents within their RT.

**Status: FULLY COMPLETE. All routes wired into router. Migration v005 applied. Active RT uniqueness verified.**

**Completed tasks:**
- [x] Migration: `households` table (v004)
- [x] Migration: `residents` table (v004)
- [x] Migration: RT unique index fix — `uniq_rts_active` partial unique index (v005)
- [x] RT service: model, repository, service layer, handler
- [x] SUPER_ADMIN CRUD for `/api/v1/rts` (routes wired into router)
- [x] Active RT uniqueness enforced via partial unique index — only active RTs are constrained by (rw, rt)
- [x] Typed SQLSTATE 23505 error detection for PostgreSQL unique violations
- [x] Household CRUD: create, list, get, update, deactivate (Pengurus)
- [x] Household read: list, get (Warga/Bendahara/Pengurus)
- [x] Resident CRUD: create, list, get, update, deactivate (Pengurus)
- [x] Resident read: list, get (Warga/Bendahara/Pengurus)
- [x] Role-check middleware on all handlers (RequireRole for tenant roles)
- [x] Cross-tenant validation: household/resident operations scoped to requesting RT
- [x] Active/inactive semantics: default active filter on read, soft delete on write
- [x] Unit tests for all handlers (8 tests in RT, 17 in auth, 20 in household, 3 in http)
- [x] Integration tests for tenant isolation
- [x] API tests for CRUD operations
- [x] Runtime E2E verification: all 11 RT endpoint checks pass
- [x] Race detector: clean (`go test -race`)
- [x] Static analysis: clean (`go vet`)

**Deliverable:** Complete, runtime-verified CRUD for RTs, Households, and Residents. Tenant isolation verified. Migration v005 applied and verified.

---

### Phase 4: Financial Categories & Ledger Foundation

**Goal:** Configure financial categories, manage dues, generate bills, and record payments/income/expenses.

**Tasks:**
- [ ] Migration: `financial_categories` table
- [ ] Migration: `dues` table
- [ ] Migration: `bills` table
- [ ] CRUD for `/api/v1/categories`
- [ ] Config endpoint for `/api/v1/dues`
- [ ] Generate bills endpoint (one bill per active household for a period)
- [ ] Bill CRUD (limited: draft editing)
- [ ] Bill status tracking (`draft`, `active`, `paid`, `overdue`)
- [ ] Bill outstanding balance calculation
- [ ] Unit tests for dues/billing logic
- [ ] Integration tests for bill generation
- [ ] API tests for bills endpoint
- [ ] Audit logs for dues and bill changes

**Deliverable:** Bills can be generated for all households. Categories are configurable.

---

### Phase 5: Payments

**Goal:** Record payments against bills. Support partial payments. Link payments to financial transactions.

**Tasks:**
- [ ] Migration: `payments` table
- [ ] Migration: `transactions` table
- [ ] Create `/api/v1/payments` endpoint
- [ ] Payment creates a linked financial transaction (income)
- [ ] Payment applies to a bill and updates bill status
- [ ] Handle partial payments
- [ ] Handle overpayment (credit)
- [ ] Implement payment cancellation (update status, do not delete)
- [ ] Implement idempotency keys on `/api/v1/payments`
- [ ] Calculate bill outstanding balance from payments
- [ ] Unit tests for payment logic
- [ ] Integration test: payment creates linked transaction
- [ ] Integration test: idempotency key prevents duplicate payments
- [ ] API tests for payment endpoints
- [ ] Audit logs for payment creation and cancellation

**Deliverable:** Payments can be recorded, linked to bills, and cancelled. No duplicate payments possible.

---

### Phase 6: Financial Ledger (Income & Expense)

**Goal:** Record cash income and expenses outside of bills. Full transaction lifecycle management.

**Tasks:**
- [ ] Implement `/api/v1/transactions` CRUD
- [ ] Transaction state machine (draft → posted)
- [ ] Cannot edit or delete posted transactions
- [ ] Implement `/api/v1/transactions/:id/reversal` endpoint
- [ ] Reversal creates a new transaction with opposite type
- [ ] Implement `/api/v1/transactions` ledger view (all posted)
- [ ] Running balance calculation on ledger
- [ ] Financial category linking
- [ ] Unit tests for transaction lifecycle
- [ ] Unit tests for reversal logic
- [ ] Integration test: ledger balance consistency
- [ ] Integration test: cannot delete posted transaction
- [ ] API tests for all transaction endpoints
- [ ] Audit logs for all financial transactions and reversals

**Deliverable:** Complete financial ledger with immutable posted transactions and audit trail.

---

### Phase 7: Dashboard & Reports

**Goal:** Administrative users can view RT financial summaries and generated reports.

**Tasks:**
- [ ] Implement `/api/v1/dashboard` endpoint
- [ ] Dashboard data: summary stats, recent activity
- [ ] Implement `/api/v1/reports/cashflow` endpoint
- [ ] Implement `/api/v1/reports/household-balances` endpoint
- [ ] Query optimization for report endpoints
- [ ] Unit tests for report calculations
- [ ] Integration tests for reported balances
- [ ] API tests for dashboard and report endpoints

**Deliverable:** Dashboard and report API endpoints returning accurate financial summaries.

---

### Phase 8: Android Foundation

**Goal:** Flutter application with authentication flow, API client, and navigation scaffold.

**Framework:** Flutter (confirmed). Android module initialized in `mobile/` directory.

**Tasks:**
- [ ] Scaffold Flutter project in `mobile/` directory
- [ ] Set up project structure (core/data/domain/presentation)
- [ ] Implement secure token storage (android: Keystore via flutter_secure_storage)
- [ ] Implement API client (Dio)
- [ ] Implement error handling and mapping
- [ ] Implement login screen
- [ ] Implement dashboard screen (stub)
- [ ] Implement bottom navigation scaffold
- [ ] Implement logout and token refresh flow

**Deliverable:** Android app that can authenticate and display a basic dashboard screen.

---

### Phase 9: Android Feature Integration

**Goal:** Connect Flutter screens to backend APIs feature-by-feature.

**Tasks (in order):**
- [ ] Household list/detail/create/edit/delete
- [ ] Resident list/detail/create/edit/deactivate
- [ ] Categories list/create/edit/delete
- [ ] Dues config and bill generation
- [ ] Bills list/detail (role-based view)
- [ ] Payments list/create/cancel
- [ ] Transactions list/create/post/reverse
- [ ] Ledger view
- [ ] Reports view
- [ ] User management (Pengurus)
- [ ] Audit log view
- [ ] Profile screen
**Deliverable:** Fully functional Android application with all MVP features.

---

### Phase 7: Arrears & Service Operations — PLANNED / NOT IMPLEMENTED

**Goal:** Derive monthly arrears from bill/payment records, link arrears to garbage collection service eligibility, and support administrative overrides.

**Dependency order (must be built in this order):**

```
Monthly Billing  →  Payment Allocation  →  Arrears Calculation  →  Service Eligibility  →  Garbage Collection Operations
```

**IMPORTANT:** Garbage collection service suspension must be DERIVED from financial state plus authorized exceptions. No simple manual boolean such as `trash_pickup = false` may be used as the sole source of truth.

**Prerequisite:** Phase 5 (Payments) must be working before this phase.

**Tasks (PLANNED / NOT IMPLEMENTED):**

- [ ] **Billing.1:** Monthly household billing — generate monthly bills per active household with period, amount, due_date, status
- [ ] **Billing.2:** Payment allocation — allow partial payments, track `PAID` / `PARTIAL` / `UNPAID` bill status, prevent double-allocation
- [ ] **Arrears.1:** Derived overdue month count + outstanding Rupiah amount per household — calculate from unpaid/partial overdue bills, NEVER store `arrears_months` or `arrears_amount` as a denormalized field
- [ ] **Service.1:** Garbage collection eligibility rule — >= 3 overdue monthly bills → service suspended (SUSPENDED state), 0–2 → ACTIVE
- [ ] **Service.2:** Administrative exemption/override + audit trail — `AUTO` / `SUSPENDED` / `EXEMPTED` with `reason`, `changed_by`, `changed_at`, optional expiry
- [ ] **Web.1:** Warga arrears badges and filters — "Ada Tunggakan", ">= 3 Bulan", "Sampah Ditangguhkan"
- [ ] **Web.2:** Household financial summary + bill/payment history detail view

**Conceptual bill model (final schema TBD):**

| Field | Note |
|-------|------|
| period | e.g. "2026-09" |
| amount | numeric(15, 2) — Rupiah |
| due_date | date |
| status | UNPAID / PARTIAL / PAID (planned) |

**Derived arrears (from bill/payment records only):**

```
Jan 2026    PAID       Rp50.000
Feb 2026    UNPAID     Rp50.000
Mar 2026    UNPAID     Rp50.000
Apr 2026    UNPAID     Rp50.000

Result (derived):
Outstanding months: 3
Outstanding amount: Rp150.000
```

**Domain decisions recorded:**

1. Financial records are the source of truth for arrears.
2. Service eligibility is derived from financial state plus authorized exceptions.
3. Administrative overrides must be audit-tracked.

**Open items — policy still needs confirmation:**

- How should PARTIAL payment affect overdue-month counting?
- Payment allocation order: oldest-first? user-selected? automatic?
- Does the 3-month threshold mean any 3 overdue or 3 consecutive? (Working assumption: past due and not fully paid, any order.)
- Vacant houses — do they generate bills?
- Temporary exemptions — can Pengurus grant them? Expiry?
- Historical tariff changes — how represented?
- Overpayments/credits — how handled?
- Multiple future months paid in advance?

---

### Phase 8: General Income & Transaction Categorization — PLANNED / NOT IMPLEMENTED

**Goal:** Support non-regular/unexpected income types using a general transaction categorization system rather than hardcoded one-off transaction types.

**Tasks (PLANNED / NOT IMPLEMENTED):**

- [ ] **Finance.1:** Transaction categories for non-regular income: Sumbangan, THR, Kegiatan, Bantuan, Lainnya
- [ ] Create financial categories for non-regular income: Sumbangan, THR, Kegiatan, Bantuan, Lainnya
- [ ] Allow recording general income with: category, description, amount, date, source, payment method, notes, recorded by
- [ ] Ensure categorization does not prevent future event/accounting module capability

**Domain decision recorded:**

- General income uses categorization rather than hardcoded one-off transaction models.

**Future possibility (NOT in scope):** Event accounting — grouping income/expenses by event (e.g., HUT RI 2027) with a resulting event balance. Categorization should not block this capability.

---

### Phase 9: Dashboard & Reports

**Goal:** Administrative users can view RT financial summaries and generated reports.

**Tasks:**

- [ ] Implement `/api/v1/dashboard` endpoint
- [ ] Dashboard data: summary stats, recent activity
- [ ] Implement `/api/v1/reports/cashflow` endpoint
- [ ] Implement `/api/v1/reports/household-balances` endpoint
- [ ] Query optimization for report endpoints
- [ ] Unit tests for report calculations
- [ ] Integration tests for reported balances
- [ ] API tests for dashboard and report endpoints

**Deliverable:** Dashboard and report API endpoints returning accurate financial summaries.

---

### Phase 10: Ops — Non-Resident Operational Accounts & View — PLANNED / NOT IMPLEMENTED

**Goal:** Support non-resident user accounts for operational purposes and provide a least-privilege garbage collection operational view.

**IMPORTANT:** A system User does NOT necessarily represent a resident. A non-resident operational worker may have RT Membership but NO Resident/Household association. Do NOT create fake Warga/Resident records merely to provide login access.

This preserves the existing separation between:
- authentication identity
- tenant membership
- resident/household domain records

**Tasks (PLANNED / NOT IMPLEMENTED):**

- [ ] **Ops.1:** Non-resident operational account/RBAC design — `operator` role as a FUTURE tenant role (not currently implemented; current roles: `pengurus`, `bendahara`, `warga`)
- [ ] **Ops.2:** Garbage collection operational view — minimum information for operators: house number, head name, pickup eligibility; NO unnecessary access to financial balance, NIK, or sensitive resident data
- [ ] RBAC migration to support `operator` role (requires future formal design)
- [ ] Preliminary permission model (subject to formal RBAC design):

| Role | Permissions |
|------|------------|
| Pengurus | administration, resident management, reports, configuration, authorized service overrides |
| Bendahara | income, expenses, billing, payments, arrears |
| Operator (PLANNED) | operational service list, see eligibility status, update permitted operational statuses; NO financial/admin access, NO sensitive resident data |
| Warga | access to permitted household/self-service information |

**Domain decision recorded:**

- User/account identity is independent from resident/household identity.
- Operational access follows least privilege.

---

### Phase 11: Web Enhancements — PLANNED / NOT IMPLEMENTED

**Goal:** Warga list and household detail page enhancements: arrears display, service status badges, filters, billing history.

**Tasks (PLANNED / NOT IMPLEMENTED):**

- [ ] Warga list: add "Tunggakan" column, "Total Tunggakan", "Status Sampah", "Status Warga"
- [ ] Warga list filters: Semua, Aktif, Tidak Aktif, Ada Tunggakan, >= 3 Bulan, Sampah Ditangguhkan
- [ ] Household detail: financial summary sidebar — Iuran per bulan, Terakhir bayar, Tunggakan, Total, Status Sampah
- [ ] Household detail: billing history (Period / Status per month)
- [ ] Reuse `.sf-page-header-surface` and `.sf-content-surface` patterns established in prior UI work

---

### Phase 12: Android Foundation

**Goal:** Flutter application with authentication flow, API client, and navigation scaffold.

**Framework:** Flutter (confirmed). Android module initialized in `mobile/` directory.

**Tasks (PLANNED / NOT IMPLEMENTED):**

- [ ] Scaffold Flutter project in `mobile/` directory
- [ ] Set up project structure (core/data/domain/presentation)
- [ ] Implement secure token storage (android: Keystore via flutter_secure_storage)
- [ ] Implement API client (Dio)
- [ ] Implement error handling and mapping
- [ ] Implement login screen
- [ ] Implement dashboard screen (stub)
- [ ] Implement bottom navigation scaffold
- [ ] Implement logout and token refresh flow

**Deliverable:** Android app that can authenticate and display a basic dashboard screen.

---

### Phase 13: Android Feature Integration

**Goal:** Connect Flutter screens to backend APIs feature-by-feature.

**Tasks (in order) — PLANNED / NOT IMPLEMENTED:**

- [ ] Household list/detail/create/edit/delete
- [ ] Resident list/detail/create/edit/deactivate
- [ ] Categories list/create/edit/delete
- [ ] Dues config and bill generation
- [ ] Bills list/detail (role-based view)
- [ ] Payments list/create/cancel
- [ ] Transactions list/create/post/reverse
- [ ] Ledger view
- [ ] Reports view
- [ ] User management (Pengurus)
- [ ] Audit log view
- [ ] Profile screen

**Deliverable:** Fully functional Android application with all MVP features.

---

### Phase 14: Testing & Security Hardening

**Goal:** Comprehensive test coverage and security review.

**Tasks:**

- [ ] Increase unit test coverage for all modules
- [ ] Integration test suite covering full workflows
- [ ] Tenant isolation test suite (RT A vs RT B)
- [ ] Financial accuracy test suite (balance calculations)
- [ ] Authorization test suite (all role combinations)
- [ ] Idempotency test suite (duplicate payment prevention)
- [ ] Security review (dependencies, secrets, TLS config)
- [ ] Performance testing (dashboard load time, report queries)
- [ ] Fix any issues found

**Deliverable:** High test coverage, verified security, performance within targets.

---

### Phase 15: Deployment

**Goal:** Deploy to production VPS with Docker.

**Tasks:**

- [ ] Write Dockerfile for backend
- [ ] Write `docker-compose.yml` with backend + PostgreSQL
- [ ] Configure Nginx/Caddy for TLS termination
- [ ] Set up Let's Encrypt SSL certificates
- [ ] Configure database backups (cron job + S3)
- [ ] Write deployment runbook
- [ ] Deploy to VPS
- [ ] Verify health check and all endpoints
- [ ] Set up monitoring (optional, future)

**Deliverable:** Production deployment with TLS, backups, and documented operations.

---

## 5. Approved Phase Dependency Order

Certain phases depend on prior phases and must be built in order:

```
Phase 5 (Payments)
    |
    v
Phase 7 (Arrears & Service)  ← requires Payments
    |
    v
Phase 10 (Ops)               ← requires Phase 7
    |
    v
Phase 11 (Web Enhancements)  ← requires Phase 7 + 8

Phase 8 (General Income)     ← independent (after Finance)
```

---

## 6. Summary of ALL Phases

| # | Phase | Prerequisites | Status |
|---|-------|--------------|--------|
| 0 | Requirements & Architecture | — | COMPLETE |
| 1 | Backend Foundation | — | COMPLETE |
| 2 | Authentication & Authorization | Phase 1 | COMPLETE |
| 2.5 | Backend Hardening | Phase 2 | In Progress |
| 3 | RT, Household, Resident Mgmt | Phase 2 | COMPLETE |
| 4 | Financial Categories & Ledger | Phase 2 | Planned |
| 5 | Monthly Billing & Payments | Phase 4 | Planned |
| 6 | Ledger (Income/Expense) | Phase 5 | Planned |
| 7 | Arrears & Service Operations | Phase 5 | Planned (NOT IMPLEMENTED) |
| 8 | General Income & Categorization | Phase 4 | Planned (NOT IMPLEMENTED) |
| 9 | Dashboard & Reports | Phase 6 | Planned |
| 10 | Ops — Operator Accounts & View | Phase 7 | Planned (NOT IMPLEMENTED) |
| 11 | Web Enhancements | Phase 7, 8 | Planned (NOT IMPLEMENTED) |
| 12 | Android Foundation | Phase 2 | Planned |
| 13 | Android Feature Integration | Phase 8+ | Planned |
| 14 | Testing & Hardening | All phases | Planned |
| 15 | Deployment | All phases | Planned |

**Dependency note:** Phase 7 (Arrears & Service) REQUIRES Phase 5 (Payments) to be reliable. Do not build service suspension before billing/arrears is working. Phase 10 (Ops) REQUIRES Phase 7.

---

## 7. Next Immediate Step

**Future finance/billing implementation begins only after the current frontend
stabilization work is completed and the Billing.1 domain/schema/API design is
reviewed.**

The current phase focus is on stabilizing the web frontend layout (W1/B corrections).
No billing, arrears, or operations work starts until the frontend baseline is solid.
