# Social Finance - Architecture Document

## 1. Architectural Style

**Modular Monolith**

The system is a single deployed unit with logically separated modules organized by domain. Modules communicate through internal function/method calls, not inter-process or network calls.

No microservices. No message queues. No event-driven architecture beyond basic audit log writes.

### Why Modular Monolith

- Simplicity for a small development team
- Single codebase to maintain, test, and deploy
- All data resides in a single database with no distributed transactions
- Scales adequately for the first target size (5,000 residents per RT)
- Can be re-architectured later if needed

---

## 2. System Overview

```
                    +------------------------+
                    |   Android App          |
                    |   (Flutter)            |
                    +--------+---------------+
                             |
                      HTTPS / API
                             |
                    +--------v----------+
                    |   Nginx            |
                    |   (Reverse Proxy)  |
                    +--------+----------+
                             |
                    +--------v----------+
                    |   Backend API      |
                    |   (Go / Chi)      |
                    |   Dockerized      |
                    +--------+----------+
                             |
                    |================|
                    | Docker internal|
                    | network        |
                    |================|
                             |
                    +--------v----------+
                    |   PostgreSQL       |
                    |   Dockerized       |
                    |   (hostname only)  |
                    +-------------------+
```

### Components

| Component | Technology | Purpose |
|-----------|-----------|---------|
| Android Client | Flutter | Resident-facing UI |
| Reverse Proxy | Nginx (Caddy or Traefik) | TLS termination, routing |
| Backend API | Go + Chi | Business logic, API layer |
| Database | PostgreSQL | Persistence |
| File Storage | Local/VPS filesystem (v2: cloud storage) | Receipt attachments (deferred) |

---

## 3. Backend Architecture

### 3.1 Directory Structure

```
backend/
├── cmd/
│   └── server/
│       └── main.go          # Application entry point
├── internal/
│   ├── config/              # Configuration loading (env vars only)
│   ├── middleware/          # HTTP middleware (auth, CORS, logging)
│   ├── migration/           # Database migration runner
│   ├── model/               # Shared database models
│   ├── pkg/                 # Shared utility packages
│   └── module/              # Domain modules
│       ├── auth/            # Authentication & authorization
│       ├── rt/              # RT management (super admin)
│       ├── user/            # User management
│       ├── household/       # Household management
│       ├── resident/        # Resident management
│       ├── category/        # Financial categories
│       ├── dues/            # RT dues configuration
│       ├── bill/            # Bill management
│       ├── payment/         # Payment recording
│       ├── transaction/     # Financial transactions (income/expense)
│       ├── report/          # Financial reports and dashboard
│       ├── audit/           # Audit log persistence
│       └── profile/         # User profile management
├── migrations/
│   ├── 001_create_rts.up.sql
│   ├── 001_create_rts.down.sql
│   └── ...
├── tests/
│   ├── unit/
│   ├── integration/
│   └── api/
├── Makefile                 # Convenience commands (dev, build, migrate)
├── Dockerfile.dev           # Development container with hot reload
├── Dockerfile               # Production multi-stage build
├── go.mod
├── go.sum
└── .air.toml                # Air configuration (dev only)
```

**Note:** `backend/docker-compose.yml` lives in the project root, not in `backend/`, alongside `.env.example` and the project-level `.gitignore`.

### 3.2 Module Responsibilities

Each module is a self-contained package within `internal/module/<name>/` with:

```
internal/module/<name>/
├── handler.go       # HTTP handlers (thin, delegate to service)
├── service.go       # Business logic
├── repository.go    # Database access
├── model.go         # Domain models (internal to module)
└── service_test.go  # Unit tests
```

**Handler (thin)** receives HTTP request, validates input, calls service.
**Service** contains business logic, enforces business rules, calls repository.
**Repository** handles SQL queries, ORM or raw queries.
**Model** defines domain-specific structures (may differ from db models).

### 3.3 Cross-Cutting Concerns

| Concern | Implementation |
|---------|---------------|
| Auth | JWT middleware per route or route group |
| RBAC | Authorization middleware checking role against route permissions |
| Tenant Isolation | Middleware extracting `rt_id` from JWT claims; injected into every query |
| Logging | Structured logging (JSON) with request ID |
| Error Handling | Standard error response structure |
| Validation | Request body validation before reaching service layer |
| Rate Limiting | Per-IP or per-user rate limiting on auth endpoints |
| Migration | Docker-run migration tool (e.g., golang-migrate or goose) |

### 3.4 No Premature Abstractions

- No repository interfaces unless multiple implementations are needed (e.g., test doubles).
- No event bus or pub/sub.
- No CQRS.
- No saga or distributed transaction patterns.
- No service discovery.

---

## 4. Tenant Isolation Architecture

### 4.1 Multi-Tenancy Approach

**Row-Level Tenant Isolation via `rt_id`**

Every tenant-scoped table includes an `rt_id` column. Every query filtering on user-visible data must include `rt_id` in the WHERE clause.

The `rt_id` is **never** accepted from client input.

### 4.2 Tenant Context Derivation

1. User authenticates → receives JWT access token.
2. JWT payload includes `rt_id` and `role` claims.
3. Auth middleware decodes JWT and extracts `rt_id`.
4. `rt_id` is placed in request context.
5. Repository layer reads `rt_id` from context and appends to `WHERE rt_id = ?`.

### 4.3 SUPER_ADMIN (Platform) Isolation

- SUPER_ADMIN users have `rt_id: null` in tenant scope.
- SUPER_ADMIN endpoints do not require `rt_id` context.
- SUPER_ADMIN queries explicitly join or filter by the requested `rt_id` (passed as path parameter, validated by handler).
- SUPER_ADMIN can view any RT's data but must explicitly specify which RT.

### 4.4 Enforcement Rules

```
NEVER trust rt_id from client JSON body or query parameters for authorization.
NEVER allow a client to set rt_id in request context.
rt_id must always flow through the JWT token issued during authentication.
All repository methods must assert rt_id context presence (panic in production).
```

---

## 5. API Architecture

### 5.1 REST API

- Base path: `/api/v1/`
- All endpoints use nouns (plural) for resource names.
- Standard HTTP methods and status codes.
- JSON request/response bodies.
- Pagination via query parameters (`page`, `per_page`).

### 5.2 API Versioning

URL-based versioning: `/api/v1/`. Future: `/api/v2/` if breaking changes are introduced.

### 5.3 API Client (Android)

- Token-based auth: access token (short-lived) + refresh token (long-lived).
- Token storage via Flutter secure storage (Android Keystore / iOS Keychain).
- Repository pattern: data access abstracted from UI.
- Error handling: consistent error model mapped to user-friendly messages.

---

## 6. Docker & Deployment Architecture

### 6.1 Local Development Environment

**Goal:** Backend and PostgreSQL run entirely inside Docker containers. No PostgreSQL server needs to be installed on macOS.

**Conceptual structure:**

```
macOS (Developer Machine)
├── backend/         # Source code on host
│   └── (bind-mounted into container)
├── docker-compose.yml  # Orchestrates the stack
├── .env.example      # Documented config template
└── .env               # Actual config (git-ignored)
                          │
                          │  bind-mount (source)
                          │  HTTP (port 8080)
                          ▼
┌─────────────────────────────────────┐
│  docker-compose:                    │
│  ┌──────────┐    ┌────────────────┐ │
│  │ backend  │───▶│postgres        │ │
│  │ container│    │ container      │ │
│  │ :8080    │    │ :5432          │ │
│  └──────────┘    └────────────────┘ │
│           internal Docker network    │
└─────────────────────────────────────┘
     │                           │
     │ (127.0.0.1:8080)          │ (127.0.0.1:5432 — optional)
     ▼                           ▼
DBeaver / REST client       pgAdmin / CLI tools
(backend API)               (database management)
```

**Key decisions:**

- Backend binds `host:8080 → container:8080`.
- PostgreSQL binds `container:5432` only. For local debugging with DBeaver/pgAdmin, optionally expose `127.0.0.1:5432 → container:5432`. **PostgreSQL must NOT be publicly exposed in production.**
- Backend source code remains on the host; bind-mounted into the development container (`-v`).
- Backend connects to PostgreSQL using the **Docker service hostname** (e.g., `postgres`), NOT `localhost`.
- PostgreSQL data persists via a named Docker volume, not bind-mount.

**Container networking:**

- Backend and PostgreSQL share an internal Docker Compose network.
- Backend connects via `postgres:5432` (service name, not IP).
- `network_mode: host` is NOT used.
- Container IP addresses are NOT hardcoded.

### 6.2 Hot Reload (Development Only)

Development containers use **Air** (https://github.com/cosmtrek/air) for Go hot reload:

- `Dockerfile.dev` installs Air in the development image.
- Air watches source files (mounted from host) and recompiles on changes.
- Hot reload is DEVELOPMENT ONLY. The production image does NOT include Air or any development tooling.

### 6.3 Development Docker Compose

The project root `docker-compose.yml` orchestrates the local development stack:

```yaml
services:
  backend:
    build:
      context: ./backend
      dockerfile: Dockerfile.dev
    volumes:
      - ./backend:/app
      # ... Air hot reload mounts ...
    ports:
      - "8080:8080"
    environment:
      # Environment variables loaded from .env file
      - DB_HOST=postgres
      - DB_PORT=5432
      - DB_NAME=social_finance
      - DB_USER=
      - DB_PASSWORD=
      # ... more env vars ...
    depends_on:
      postgres:
        condition: service_healthy
    networks:
      - social-finance-internal

  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: social_finance
      POSTGRES_USER:
      POSTGRES_PASSWORD:
    volumes:
      - postgres_data:/var/lib/postgresql/data
    ports:
      # For local dev tooling ONLY — not required for production
      - "127.0.0.1:5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U $$POSTGRES_USER -d $$POSTGRES_DB"]
      interval: 5s
      timeout: 5s
      retries: 5
    networks:
      - social-finance-internal

volumes:
  postgres_data:  # Data persists across container restarts

networks:
  social-finance-internal:
    driver: bridge
```

**Note:** This is the ARCHITECTURE. The actual `docker-compose.yml` is created during Phase 1 implementation. The structure above is documented for reference.

### 6.4 Migration Strategy

Migrations are executed via Docker to avoid requiring migration tooling on the developer's macOS:

```bash
# Run migrations up
docker compose run --rm migrate up

# Revert last migration
docker compose run --rm migrate down

# Check migration status
docker compose run --rm migrate status
```

Migration images and tooling are included in the development build context, not in production.

**Safety rules:**
- Never automatically run destructive migrations on application startup.
- Migrations are version-controlled under `backend/migrations/`.
- All migrations have up and down files.
- Migrations are tested before application deployment.

### 6.5 Docker Compose Configuration

**Environment variables:**

- All secrets are loaded from `.env` (git-ignored).
- `.env.example` documents required variables and acceptable value ranges.
- No real secrets are committed to the repository.

**PostgreSQL configuration:**

- PostgreSQL is NOT publicly exposed in production.
- PostgreSQL data persists via named volume (`postgres_data`).
- Container restart will NOT delete database data.

### 6.6 Production Environment

**VPS architecture:**

```
VPS (Linux)
├── Docker Engine
    │   ├── social-finance-backend (Go API)
    │   ├── social-finance-postgres (PostgreSQL)
│   └── nginx (reverse proxy)
├── Docker Volumes
│   ├── postgres_data (PG data)
│   └── uploads (receipt attachments, v2)
└── Backup cron → offsite (S3)
```

**Production Dockerfile Strategy (multi-stage build):**

```
Stage 1: Build
├── golang:1.xx-alpine (build image)
├── Install dependencies
├── Build binary
└── Output: compiled binary

Stage 2: Runtime
├── alpine (minimal)
├── Copy compiled binary
├── Set non-root user
├── Healthcheck endpoint
├── Production environment variables
└── Graceful shutdown support
```

**Production constraints:**
- Production image does NOT contain source code, Air, or development tools.
- Production image runs the compiled Go binary only.
- Production PostgreSQL is NOT publicly exposed.
- No Docker volumes are bind-mounted with sensitive data exposed.
- TLS termination at Nginx/Caddy (Let's Encrypt).
- Persistent PostgreSQL storage via Docker volume or dedicated host mount.
- Graceful shutdown signal handling (SIGTERM).

---

## 7. Technology Decisions (Resolved)

All major technology decisions are confirmed:

| Category | Selected | Status |
|----------|----------|--------|
| Backend language | Go | CONFIRMED |
| HTTP router | Chi | CONFIRMED |
| Database | PostgreSQL | CONFIRMED |
| Backend runtime (dev) | Docker container | CONFIRMED |
| Database runtime (dev) | Docker container | CONFIRMED |
| Container orchestration (dev) | Docker Compose | CONFIRMED |
| Database connection (dev) | Docker service hostname | CONFIRMED |
| Hot reload (dev) | Air | CONFIRMED |
| Hot reload (production) | Not applicable | CONFIRMED — dev only |
| Production build | Multi-stage Docker | CONFIRMED |
| Android framework | Flutter | CONFIRMED |
| Android HTTP client | Dio | CONFIRMED |
| Production HTTP server | Nginx (Caddy fallback) | CONFIRMED |
| Production DB | PostgreSQL (Dockerized) | CONFIRMED |

**Phase 1 decisions (to select during implementation):**

| Category | Candidate Options | Notes |
|----------|-----------------|-------|
| ORM / Query builder | Raw SQL / sqlx | Prefer explicit SQL for financial queries |
| Migration tool | golang-migrate / goose | Must support up/down migrations |
| Auth library | Custom JWT or library with claims | JWT is preferred for stateless auth |
| Config loading | Viper or manual | Viper preferred for env/file support |
| Logging | slog (stdlib) or zerolog | Prefer stdlib slog for simplicity |

**Phase 8 decisions (to select during Flutter implementation):**

| Category | Candidate Options | Notes |
|----------|-----------------|-------|
| State management | BLoC, Riverpod, provider | Depends on team preference |
| Navigation | go_router, navigator 2.0 | go_router preferred for complex navigation |
| Local storage (if any) | hive, sqflite | MVP may not require local cache |

---

## 8. Port Allocation

### Local Development

| Service | Host Port | Container Port | Notes |
|---------|-----------|----------------|-------|
| Backend | 8080 | 8080 | Accessible for API testing |
| PostgreSQL | 127.0.0.1:5432 | 5432 | Optional — for DBeaver/debugging only |

### Production

| Service | Port | Notes |
|---------|------|-------|
| Backend | 8080 (internal only) | Behind Nginx reverse proxy |
| PostgreSQL | 5432 (internal only) | Docker internal network only, NOT publicly exposed |
| HTTPS | 443 | Via Nginx |
| HTTP | 80 | Redirects to HTTPS |

**Critical:** PostgreSQL must never be publicly exposed in production. It is reachable only from the backend container via Docker's internal network.
