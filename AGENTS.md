# Social Finance - Agent Instructions

This document defines the rules, conventions, and constraints for working on the Social Finance project.

These rules apply to all implementations: backend (Go), Android (Flutter), and related tooling.

## 1. Project Purpose

Social Finance is a financial management application for the neighborhood. It tracks finances transparently with immutable ledgers, role-based access control, and multi-tenant isolation.

**AI is NOT a feature of this application.** Do not add AI, LLM, chatbot, recommendation, or AI-related functionality unless explicitly requested.

## 2. Architecture

The system is a **modular monolith** with:

- Go REST API backend (`backend/`)
- PostgreSQL database
- Flutter Android client (`mobile/`)
- Docker-based deployment

Modules are separated by domain within `backend/internal/<name>/`:

```
auth/       authentication & authorization
config/     environment configuration
database/   PostgreSQL connection pool
health/     health check endpoint
http/       HTTP middleware (request ID, logging, panic recovery)
```

Future modules will be added under `backend/internal/<name>/`:
```
rt/          RT management (SUPER_ADMIN only)
household/   household management
resident/    resident management
category/    financial categories
dues/        RT dues configuration
bill/        bill management
payment/     payment recording
transaction/ financial transactions (income & expense)
report/      dashboard & financial reports
audit/       audit log persistence
```

Keep modules focused. No cross-module cyclic dependencies. Modules communicate via internal function calls, not HTTP or messages.

## 3. Coding Philosophy

- **Simplicity over cleverness.** Prefer obvious, readable code.
- **No unnecessary abstractions.** No interfaces unless multiple implementations are needed. No service interfaces unless testing demands it.
- **Small team friendly.** The code should be understandable by a developer new to the project.
- **Defensive by default.** Validate all input. Never trust client data.
- **Document decisions.** If a non-obvious choice is made, document the reasoning.

## 4. Financial Integrity Rules

### 4.1 Money Values

**NEVER use floating point types (float32, float64, REAL, DOUBLE PRECISION, FLOAT) for monetary values.**

All monetary values use fixed-precision types:

- **PostgreSQL:** `NUMERIC(15, 2)`
- **Go:** `string` for JSON parsing, convert to/from `int64` representing smallest currency unit (rupiah), or use a decimal library
- **Flutter:** `String` for API transfer, `BigInt` or custom decimal for calculations

### 4.2 Balance Computation

**NEVER store a "current balance" as a derived value.** Balance is always calculated from the financial ledger:

```
Balance = Total Posted Income - Total Posted Expenses
```

(Reversal transactions reduce the effective total.)

### 4.3 Ledger Immutability

**NEVER silently delete or modify a finalized financial transaction.**

Transaction lifecycle:

| Status | Editable | Deletable | What it means |
|--------|----------|-----------|---------------|
| `draft` | Yes | Yes | Not yet posted |
| `posted` | No | No | Finalized, visible in ledger |
| `cancelled` | No | No | A reversal transaction referencing the original |

To correct a posted transaction:
1. Create a **new reversal transaction** of opposite type and amount.
2. The reversal references the original via `reverses_transaction_id`.
3. Both the original and reversal remain in the ledger and audit trail.

**NEVER modify a finalized financial transaction without preserving an audit trail.**

### 4.4 Financial Transactions

Financial transactions have:

- `type`: `"income"` or `"expense"` (NEVER negative amounts — use type instead)
- `amount`: always positive
- `status`: `"draft"`, `"posted"`, or `"cancelled"`
- `reverses_transaction_id`: references the transaction being reversed, or `null`

## 5. Tenant Isolation

### 5.1 Tenant Context

**NEVER trust `rt_id` supplied by the client** (in URL path, query parameter, or JSON body) for authorization.

**Always derive `rt_id` from the authenticated user's JWT token claims.**

The JWT payload includes:
- `sub`: user ID
- `role`: one of `super_admin`, `pengurus`, `bendahara`, `warga`
- `rt_id`: tenant ID (nullable for `super_admin`)

### 5.2 Enforcing Tenant Isolation

Every repository method must inject the tenant context into queries:

```sql
SELECT * FROM bills WHERE rt_id = ? ...
```

**NEVER allow a client to set or override the `rt_id` context.**

For SUPER_ADMIN endpoints, `rt_id` is passed as a path parameter (e.g., `/api/v1/rt/:id/users`) and verified against the `rts` table after extracting the requested ID from the URL.

### 5.3 Data Exclusion

**NEVER expose another RT's data.** Every query on tenant-scoped data must include a `rt_id` filter unless operating as `super_admin` on an explicitly specified RT.

## 6. Security Rules

### 6.1 Secrets

**NEVER hard-code credentials.** All secrets load from environment variables.

**NEVER log:**
- Passwords or password hashes
- Access tokens or refresh tokens
- Authentication credentials
- Full NIK numbers

### 6.2 Password Handling

- Hash passwords with Argon2id or bcrypt (cost >= 12).
- Passwords are never stored in plaintext, logs, or response bodies.

### 6.3 Authentication

- JWT access tokens: short-lived (15 minutes).
- Refresh tokens: opaque, stored server-side as hashes, single-use, rotated on refresh.
- Brute-force protection: rate-limit login endpoints.

### 6.4 Input Validation

All API inputs are validated before reaching business logic:
- Required fields present and non-empty
- Correct types
- Correct formats (email, date, etc.)
- Enum values constrained
- Numeric values bounded and non-negative where required

### 6.5 SQL Injection Protection

**Always use parameterized queries.** Never concatenate user input into SQL strings.

## 7. API Conventions

### 7.1 REST Design

- Base path: `/api/v1/`
- Resource names are nouns, plural: `/api/v1/users`, `/api/v1/bills`
- Standard HTTP methods: POST (create), GET (read), PUT (update), DELETE (delete), PATCH (partial update)
- Standard HTTP status codes:
  - `200` success (with body)
  - `201` created
  - `204` success (no body)
  - `400` validation error
  - `401` unauthorized
  - `403` forbidden
  - `404` not found
  - `409` conflict (duplicate, locked resource)
  - `429` rate limited
  - `500` internal error

### 7.2 Response Format

Singleton responses: resource JSON directly.

Collection responses: paginated wrapper:
```json
{ "data": [...], "pagination": { "page": 1, "per_page": 20, "total": 100, "total_pages": 5 } }
```

Error responses:
```json
{ "error": { "code": "validation_error", "message": "Invalid request body", "details": [...] } }
```

### 7.3 Idempotency

Critical financial write endpoints accept an `Idempotency-Key` header. Duplicate keys within the configurable window return the original response without re-executing the operation.

## 8. Database Conventions

### 8.1 Schema

- UUID primary keys: `uuid` type, default `gen_random_uuid()`.
- Foreign keys with `ON DELETE RESTRICT` or `ON DELETE CASCADE` as appropriate.
- Common columns: `created_at` timestamptz, `updated_at` timestamptz.
- Soft deletes via boolean `is_active` where physical deletion is not appropriate.
- Enum values stored as `text` with `CHECK` constraints.

### 8.2 Indexes

- All tenant-scoped tables indexed on `rt_id`.
- Foreign key columns indexed.
- Frequently filtered columns indexed.
- Composite indexes for common query patterns.

### 8.3 Migrations

- Migrations stored as SQL files with sequential integer prefixes.
- Every migration has an up and down file.
- Migrations must be idempotent where possible.

### 8.4 What NOT to Do

- Do NOT store derived financial values (balance, totals) in a denormalized column.
- Do NOT use `DELETE` on financial transactions.
- Do NOT use `UPDATE` on posted transactions.
- Do NOT expose `rt_id` as an API parameter for authorization.
- Do NOT store passwords, tokens, or credentials in the database in plaintext.

## 9. Testing Expectations

### 9.1 Coverage Requirements

- All business logic must have unit tests.
- All API endpoints must have integration tests.
- Financial calculations must have calculation tests.
- Authorization checks must have role-based tests.
- Tenant isolation must have cross-tenant tests.

### 9.2 Critical Test Scenarios

These scenarios MUST be covered by automated tests:

1. **Tenant isolation:** RT A data is not accessible by RT B's users.
2. **Authorization:** WARGA cannot create or modify administrative transactions.
3. **Authorization:** BENDAHARA cannot delete financial categories.
4. **Authorization:** Users cannot manipulate IDs to access resources outside their scope.
5. **Duplicate prevention:** Idempotent payment requests do not create duplicate financial transactions.
6. **Financial immutability:** Attempting to edit or delete a posted transaction fails.
7. **Reversal correctness:** Creating a reversal preserves the original transaction and updates the net balance.
8. **Ledger consistency:** Sum of all posted income minus posted expense equals the calculated balance.

## 10. Prohibited Shortcuts

The following are STRICTLY PROHIBITED:

| Shortcut | Why It's Forbidden |
|----------|--------------------|
| Floating point for money | Rounding errors in financial data |
| Client-provided `rt_id` for auth | Bypasses tenant isolation |
| Silent delete of financial history | Destroys audit trail |
| Unhashed password storage | Critical security vulnerability |
| Logging tokens or passwords | Credentials exposure |
| Single-tenant data models | Prevents multi-RT scaling |
| Authorization in client code only | Easily bypassed |
| Stored balance calculations | Source of truth must be the ledger |
| Hard-coded configuration | Secrets in source code |
| Dropping tables instead of migrating | Data loss |

## 11. Documentation-First Policy

Before modifying database structures or API contracts:

1. Check `docs/database.md` and `docs/api.md`.
2. Update the relevant documentation to reflect the changes.
3. Then implement the code.

The documentation files are the source of truth for the system design.

## 12. Role Definitions

| Role | Scope | Description |
|------|-------|-------------|
| `super_admin` | Platform-wide | Platform administrator managing multiple RTs |
| `pengurus` | Single RT | RT executive managing users, households, residents, and some admin tasks |
| `bendahara` | Single RT | RT treasurer managing finances: bills, payments, transactions |
| `warga` | Single RT (own household) | RT resident viewing own bills and payments |

**Financial data always has stricter permissions than ordinary resident data.**

## 13. Implementation Order

Follow the phased development plan in `docs/development-plan.md`.

Do not implement a backend module before its database migration is in place.
Do not implement an Android screen before its backend API is stable.

## 14. Additional Prohibited Actions

The following rules supplement Section 10 (Prohibited Shortcuts):

- **NEVER rely on Android UI permissions as the only authorization mechanism.** All authorization must be enforced server-side. Android UI restrictions are convenience only.
- **NEVER introduce AI functionality into the Social Finance application unless explicitly requested by the human developer.** AI, LLM, chatbot, recommendation engines, and AI APIs are prohibited.
- **NEVER delete or recreate the existing Android project** without explicit human approval.
- **NEVER make breaking API or database changes** without updating the corresponding documentation files first.
- **NEVER expose NIK, passwords, tokens, credentials, or PII** in any log output, error response, or API response body.
- **NEVER assume localhost refers to another Docker container.** Use Docker service names for container-to-container communication (e.g., `postgres:5432`, NOT `localhost:5432`).
- **NEVER hardcode Docker container IP addresses.** Use service names and Docker internal networks.
- **NEVER expose PostgreSQL publicly in production.** PostgreSQL must only be accessible via Docker internal network to the backend container.
- **NEVER commit `.env` or secrets.** Only `.env.example` (documented templates) may be committed. Real secrets must be loaded from `.env` (git-ignored) or production secret management.
- **NEVER run destructive database migrations on application startup.** Migrations must be manually executed via Docker: `docker compose run --rm migrate up`.
- **NEVER put development tooling (Air, hot reload) in the production Docker image.** Hot reload tools are development-only.
- **NEVER use `network_mode: host` in Docker Compose.** Use Docker internal networks for container communication.
- **NEVER bind-mount PostgreSQL data directory.** Use named Docker volumes (`postgres_data`) for database persistence.

### 14.1 Confirmed Technology Stack

| Category | Selected |
|----------|----------|
| Backend language | Go |
| HTTP router | Chi |
| Database | PostgreSQL |
| Backend runtime (dev) | Docker container |
| Database runtime (dev) | Docker container |
| Container orchestration | Docker Compose |
| Hot reload (dev) | Air |
| Hot reload (prod) | Not applicable |
| Frontend | Flutter |
| Frontend HTTP client | Dio |
| Production server | Nginx (Caddy acceptable) |
