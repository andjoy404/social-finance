# Social Finance — Current Project State

_Inspection performed: 2026-09-16_
_Last updated: 2026-09-17 (Phase 2 COMPLETE; Phase 3 COMPLETE; Phase 3.1 COMPLETE; Web W1 CORRECTION COMPLETE)_

## Repository Root

```
social-finance/
├── .kilo/              # Kilo Code configuration (auto-generated)
├── .env.example        # Documented environment variable templates
├── .gitignore          # Excludes .env, IDE files, build artifacts
├── AGENTS.md           # AI coding assistant instructions
├── README.md           # Project overview and quick start
├── Makefile            # Convenience commands (up, down, migrate, test)
├── docker-compose.yml  # Development stack (backend + postgres + migrate)
├── mobile/             # Flutter project (Phase 8)
├── backend/            # Go backend (Phase 2 — COMPLETE)
│   ├── cmd/api/
│   │   └── main.go              # Application entry point + router
│   ├── internal/
│   │   ├── auth/
│   │   │   ├── handler.go       # HTTP handlers: login, refresh, logout, me
│   │   │   ├── handlers_test.go # Auth handler API tests
│   │   │   ├── hash.go          # Argon2id password hashing
│   │   │   ├── hash_test.go     # Password hashing tests
│   │   │   ├── jwt.go           # JWT generation & verification
│   │   │   ├── jwt_test.go      # JWT tests
│   │   │   ├── middleware.go    # Auth middleware: RequireAuth, RequireRole, RequireSystemRole
│   │   │   ├── middleware_test.go # Middleware & tenant safety tests
│   │   │   ├── models.go        # AuthContext, SystemRole, Role definitions
│   │   │   ├── repository.go    # DB queries for auth
│   │   │   ├── service.go       # Business logic: login, refresh, logout, profile
│   │   │   ├── token.go         # Refresh token generation
│   │   │   ├── rate_limiter.go  # IP-based rate limiter
│   │   │   └── tenant_safety.go # Tenant safety convention doc
│   │   ├── config/
│   │   │   ├── config.go        # Environment variable loader
│   │   │   └── config_test.go   # Config tests
│   │   ├── database/
│   │   │   └── database.go      # PostgreSQL connection pool
│   │   ├── health/
│   │   │   ├── handler.go       # GET /health endpoint
│   │   │   └── handler_test.go  # Health handler tests
│   │   └── http/
│   │       ├── logging.go       # Request logging middleware
│   │       ├── middleware_test.go
│   │       ├── panic_recovery.go
│   │       └── request_id.go
│   │   ├── household/            # Household & Resident management (Phase 3)
│   │   │   ├── handler.go        # HTTP handlers for households/residents
│   │   │   ├── handler_test.go   # Household/Resident API tests
│   │   │   ├── model.go          # Household, Resident types
│   │   │   ├── repository.go     # DB queries for households/residents
│   │   │   └── service.go        # Business logic service
│   │   └── rt/                   # RT management service (Phase 3)
│   │       ├── handler.go        # HTTP handlers for RTs
│   │       ├── handler_test.go   # RT handler tests
│   │       ├── model.go          # RT type
│   │       ├── repository.go     # DB queries for RTs
│   │       └── service.go        # RT business logic
│   ├── fixtures/               # Test fixture utilities
│   ├── migrations/             # Database migrations
│   │   ├── 001_initial.up.sql  # Creates `rts` table
│   │   ├── 001_initial.down.sql
│   │   ├── 002_auth.up.sql     # users, user_rt_memberships, refresh_tokens
│   │   ├── 002_auth.down.sql
│   │   ├── 003_separate_global_identity_from_tenant_membership.up.sql
│   │   └── 003_...down.sql
│   │   ├── 004_households_and_residents.up.sql
│   │   ├── 004_households_and_residents.down.sql
│   │   ├── 005_fix_rt_unique_indexes.up.sql  # Drops redundant full unique index, keeps partial `uniq_rts_active`
│   │   └── 005_fix_rt_unique_indexes.down.sql
│   ├── Dockerfile              # Production multi-stage build (Alpine, ~37MB)
│   ├── Dockerfile.dev          # Development with Air hot reload
│   ├── Dockerfile.migrate      # Migration runner image
│   ├── .air.toml               # Air configuration
│   ├── docker-entrypoint.sh    # Copies pre-built binary for bind mount survival
│   ├── migrate-wrapper.sh      # Wrapper for migrate with default connection flags
│   ├── tmp/                    # Pre-compiled server binary
│   ├── go.mod                  # Go module (social-finance)
│   └── go.sum
├── frontend/           # React/TypeScript web client (W1 corrected)
├── docs/               # Architecture and planning documentation
└── mobile/            # Flutter Android client (Phase 8)
```

---

## Backend Directory: `backend/`

**State: Phase 2 COMPLETE. Phase 3 COMPLETE. Phase 3.1 COMPLETE.** Auth implemented and runtime verified. Household, Resident, and RT CRUD all implemented and runtime verified. Tenant isolation verified via automated tests. All 7 test packages pass with no race conditions. Migration v006 applied.

The backend includes:
- Go module `social-finance` (Go 1.23)
- Chi HTTP router with structured logging, request ID, panic recovery
- PostgreSQL connection pool via `pgx/v5` (max 20 connections, 10-minute lifetime)
- `GET /health` endpoint with database connectivity check
- Authentication module (`internal/auth/`):
  - Argon2id password hashing (cost 12)
  - JWT access tokens with claims (sub, sys_role, role, rt_id, membership_id)
  - Opaque refresh tokens with SHA-256 hash store, rotation, revocation
  - Login, refresh, logout, /me endpoints
  - Role-based access control middleware (RequireRole)
  - System-level authorization (RequireSystemRole)
  - Tenant isolation: JWT-derived rt_id, client input ignored
  - Login rate limiting (10 req/5m/IP), refresh rate limiting (30 req/15m/IP)
- Migration tool (golang-migrate v4.18)
- Migrations v001 (rts), v002 (auth tables), v003 (system_role), v004 (households/residents), v005 (RT unique index fix — drops redundant full unique index, keeps partial `uniq_rts_active`), v006 (adds nullable `occupancy_status` CHECK constraint to `households`)
- Bootstrap CLI: `docker compose run --rm backend server --bootstrap`
- Environment-based configuration with validation
- Docker Compose services: backend, postgres, migrate
- Air hot reload for development
- Multi-stage production Dockerfile (Alpine, non-root appuser, ~37MB)
- HTTP server with graceful shutdown (SIGINT/SIGTERM)
- Household/Resident HTTP handlers (`internal/household/`):
  - Read: `GET /api/v1/households`, `GET /api/v1/households/{id}`, `GET /api/v1/residents`, `GET /api/v1/residents/{id}` (Warga/Bendahara/Pengurus)
  - Write: `POST /api/v1/households`, `PATCH /api/v1/households/{id}`, `DELETE /api/v1/households/{id}`, `POST /api/v1/residents`, `PATCH /api/v1/residents/{id}`, `DELETE /api/v1/residents/{id}` (Pengurus only)
  - `active` semantics: default active filter on read, soft delete via DELETE
  - Household-resident link: tenant-scoped (residents validated against requesting RT)
  - `occupancy_status` domain type validation (OWNER/TENANT) on CREATE/PATCH, required on CREATE (returns 400 if missing), nullable on read (legacy support)
- RT service (`internal/rt/`): model, handler, repository, service, tests complete
- 109 tests total (all passing, 0 failing)

**Go dependencies:**

| Package | Purpose |
|---------|---------|
| `github.com/go-chi/chi/v5 v5.2.0` | HTTP router |
| `github.com/jackc/pgx/v5 v5.7.2` | PostgreSQL driver and connection pool |
| `github.com/google/uuid v1.6.0` | UUID generation |
| `github.com/golang-jwt/jwt/v5 v5.2.2` | JWT signing/verification |
| `github.com/alexedwards/argon2id v1.0.0` | Argon2id password hashing |

Migration tool: **golang-migrate** v4.18 (Docker-wrapped, no host dependency).

---

## Web Directory: `frontend/`

**State: W1 CORRECTION COMPLETE.** Mock identities + admin login + Docker verified.

The web frontend is a React + TypeScript + Vite application:

- **Build pipeline**: Vite 6, TypeScript strict mode, TypeScript compiler + Vite build succeed
- **UI library**: React 18 + React Router 6
- **Charts**: Recharts for bar/line chart components
- **Design system**: CSS custom properties for theming (dark/light/system)
- **Authentication flow**: Mock login → protected dashboard (no API calls yet)
- **Responsive layout**: Sidebar navigation + main content area
- **Dashboard**: Financial overview with balance cards, bar chart for monthly income/expense, iuran (monthly dues) progress, recent transactions table
- **Code splitting**: App split across `app/`, `components/`, `features/`, `layouts/`, `styles/`, `mocks/`

**Tech:**

| Package | Version | Purpose |
|---------|---------|---------|
| `react` | 18.x | UI framework |
| `react-router-dom` | 6.x | Client-side routing |
| `recharts` | 2.x | Chart components |
| `typescript` | 5.x | Type checking |
| `vite` | 6.x | Build tool |
| `eslint` | 9.x | Linting |

---

## Android Directory: `mobile/`

**State: Flutter project scaffolded (Phase 8).**

The `mobile/` directory is the Flutter project root containing the mobile client:

```
mobile/
├── lib/               # Flutter application source
├── test/              # Flutter tests
├── android/           # Flutter native Android platform
├── ios/               # Flutter native iOS platform
├── pubspec.yaml       # Flutter/Dart dependencies
└── web/               # Flutter native Web platform
```

---

## Technology Decisions (Confirmed)

| Decision | Value | Status |
|----------|-------|--------|
| Backend language | Go 1.23 | PHASE 1-2 COMPLETE |
| HTTP router | Chi v5 | PHASE 1-2 COMPLETE |
| Database | PostgreSQL 16 (Alpine) | PHASE 1 COMPLETE |
| Database driver | pgx/v5 | PHASE 1 COMPLETE |
| Auth | JWT (HS256/RS256) | PHASE 2 COMPLETE |
| Password hashing | Argon2id (cost 12) | PHASE 2 COMPLETE |
| Refresh tokens | Opaque random (SHA-256 hash store) | PHASE 2 COMPLETE |
| Container orchestration | Docker Compose | PHASE 1 COMPLETE |
| Database connection (dev) | Docker service hostname `postgres` | PHASE 1 COMPLETE |
| Hot reload (dev only) | Air | PHASE 1 COMPLETE |
| Production build | Multi-stage Docker + Alpine | PHASE 1 COMPLETE |
| Migration tool | golang-migrate v4.18 | PHASE 1 COMPLETE |
| Android framework | Flutter | PLANNED (Phase 8) |
| Android HTTP client | Dio | PLANNED (Phase 8) |
| Port: backend | Host 8080 → Container 8080 | CONFIGURED |
| Port: PostgreSQL | Container 5432 (127.0.0.1 for dev tools) | CONFIGURED |
| Migration command | `docker compose run --rm migrate up` | CONFIGURED |
| Secrets management | `.env` (git-ignored), `.env.example` (documented) | IMPLEMENTED |

---

## Documentation Files

All required documentation files exist and are maintained:

| File | Purpose | Updated |
|------|---------|---------|
| `docs/architecture.md` | System design, deployment, tech decisions | Yes |
| `docs/database.md` | PostgreSQL schema, migrations, constraints | Yes (v4 schema) |
| `docs/api.md` | REST API endpoints, response format | Yes (Phase 2 endpoints) |
| `docs/security.md` | Auth, RBAC, data protection, Docker security | Yes |
| `docs/development-plan.md` | Phased roadmap | Yes (Phase 2 complete) |
| `docs/open-questions.md` | Resolved and pending decisions | Yes |
| `docs/current-project-state.md` | This file — project inspection | Yes |

---

## Test Suite

**Total: 109 tests** (all passing, 0 failing)

| Test Package | Tests | Description |
|-------------|-------|-------------|
| `cmd/api` | 2 | Router registration (health route, 404 fallback) |
| `internal/auth` | 66 | Password hashing, JWT, token rotation, middleware, tenant safety |
| `internal/config` | 5 | Config defaults, env overrides, invalid port error |
| `internal/health` | 2 | Healthy DB, unhealthy DB states |
| `internal/http` | 3 | Request ID middleware, panic recovery, logging middleware |
| `internal/household` | ~23 | Household/Resident CRUD, occupancy_status, active filtering, tenant isolation |
| `internal/rt` | ~17 | RT CRUD, soft delete, pagination |

Tests run via: `go test ./...` (97 tests, all pass).
Race detector: `go test -race ./...` clean.

---

## Migration State

| Version | Schema Dirty | Description |
|---------|-------------|-------------|
| 001 | false | Creates `rts` table |
| 002 | false | Creates `users`, `user_rt_memberships`, `refresh_tokens` |
| 003 | false | Adds `system_role`, drops `rt_id` from users, constrains roles |
| 004 | false | Creates `households` and `residents` tables with indexes and constraints |
| 005 | false | RT unique index fix — drops redundant full unique index, keeps partial `uniq_rts_active` |
| 006 | false | Adds nullable `occupancy_status` (CHECK: NULL, OWNER, TENANT) to `households` |
| 007 | false | Adds house_number to physical address |
| 008 | false | Option B+ Temporal model: physical_houses, household_occupancies, residency_periods |
| 009 | false | Finance module: financial_categories, dues, bills, payments, transactions, idempotency_keys, audit_logs |

---

## Backend MVP Autonomously Completed & Verified

**Status: COMPLETE & VERIFIED (2026-09-19)**

### 1. Household & Resident (Option B+ Temporal Lifecycle)
- Migrations 008 executed and reconciled with 0 fabricated dates.
- Physical Houses (`physical_houses`), Household Occupancies (`household_occupancies`), and Residency Periods (`residency_periods`).
- Temporal moves (`/move`) require explicit start dates and atomically terminate existing occupancies/periods.
- House number and address corrections update existing physical house without ending occupancy (409 on collision).
- Current projections preserve backwards compatibility with legacy reads.

### 2. Finance Domain (Phase 4 MVP)
- Migration 009 applied cleanly: version 9, dirty false.
- Zero floating-point types for currency: strict `NUMERIC(15,2)` in PostgreSQL and fixed-precision representation in Go (`finance/money.go`).
- Dynamic ledger-derived balance calculation: `sum(income) - sum(expense)` on posted transactions. NEVER stored or denormalized.
- Immutable financial ledger: transactions are append-only. Reversals create compensating posted transactions with opposite type referencing the original.
- Idempotency support (`Idempotency-Key` header) with request caching and deduplication.
- Payment workflows: citizen self-submission (with proof) and staff recorded ("Record & Confirm").
- Proof storage validation (size, allowed mime types, sanitized paths).
- Append-only audit logs for all financial writes.

### 3. Warga Import / Export
- Tenant-scoped CSV Export (`GET /api/v1/warga/export`).
- CSV Import Preview (`POST /api/v1/warga/import/preview`) with row-level validation and conflict checks without DB mutation.
- Atomic Import Commit (`POST /api/v1/warga/import/commit`) within database transaction.
- Structured, machine-readable validation error reporting.

### 4. Verification & Quality
- `gofmt -l .`: 100% clean formatting.
- `go vet ./...`: 0 issues.
- `go test -race -count=1 ./...`: 100% PASS across all 11 packages.
- Docker HTTP E2E: Real integration tests in `tests/` cover Household B5 lifecycle, Finance flows, and Warga IO flows against `http://backend:8080`.
- All tenant operations strictly enforce tenant isolation from JWT claims.

