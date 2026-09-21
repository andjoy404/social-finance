# Social Finance

**Social Finance** — Social Finance for Your Neighborhood

Financial management application for your neighborhood.

## Overview

Social Finance provides transparent financial tracking, bill management, payment recording, and reporting capabilities for neighborhood administrators and residents.

### Features (Phases 1-2)

- PostgreSQL database with migration tooling (golang-migrate)
- Multi-stage Docker development and production images
- Health check endpoint with database connectivity verification
- Phase 2: User authentication (Argon2id password hashing, JWT access tokens, opaque refresh tokens with rotation/revocation)
- Phase 2: Role-based access control (SUPER_ADMIN, pengurus, bendahara, warga)
- Phase 2: System vs. tenant authorization separation
- Phase 2: Tenant isolation (JWT-derived `rt_id`, never from client input)
- Phase 2: Login and refresh rate limiting (per-IP, configurable)

### Platform

- **Backend:** REST API (Go) + PostgreSQL
- **Frontend:** Android application (Flutter)
- **Deployment:** Docker on VPS

## Quick Start

```bash
# Start the stack (backend + postgres)
docker compose up --build -d

# View logs
docker compose logs -f backend

# Run migrations
docker compose run --rm migrate up

# Stop the stack
docker compose down
```

## Technology Stack

| Category | Selected | Version |
|----------|----------|---------|
| Backend language | Go | 1.23 |
| HTTP router | Chi | v5 |
| Database | PostgreSQL | 16 (Alpine) |
| Database driver | pgx v5 | v5.7 |
| Migration tool | golang-migrate | v4.18 |
| Container orchestration | Docker Compose | v2+ |
| Hot reload (dev) | Air | latest |
| Production build | Multi-stage Docker | Alpine Linux |
| Android framework | Flutter | — |

### Go Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/go-chi/chi/v5` | HTTP router |
| `github.com/jackc/pgx/v5` | PostgreSQL driver and connection pool |
| `github.com/google/uuid` | UUID generation for request IDs and DB primary keys |
| `github.com/golang-jwt/jwt/v5` | JWT access token generation and verification |
| `github.com/alexedwards/argon2id` | Argon2id password hashing |

### Why golang-migrate

Selected over goose because:
- Simpler CLI with explicit up/down commands
- Single binary, no database schema table (uses a dedicated migrations table)
- Straightforward Docker integration via the official image

## Local Development

### Prerequisites

- Docker and Docker Compose installed
- Go (optional, for local tests without Docker)

### Environment Configuration

Copy the example file and adjust values if needed:

```bash
cp .env.example .env
```

The `.env.example` file contains all required variables with safe defaults.
The actual `.env` file is git-ignored.

### Docker Services

| Service | Description | Port |
|---------|-------------|------|
| `backend` | Go API server | `http://localhost:8080` |
| `postgres` | PostgreSQL database | `127.0.0.1:5432` (local tools) |
| `migrate` | Migration runner | — |

The backend connects to PostgreSQL using the Docker service hostname `postgres`, never via localhost.

### Commands

```bash
# Start services
docker compose up --build -d

# View backend logs
docker compose logs -f backend

# Apply migrations
docker compose run --rm migrate up

# Revert last migration
docker compose run --rm migrate down 1

# Run tests (inside Docker)
docker compose run --rm -T backend go test -v ./...

# Stop services (data preserved)
docker compose down

# Stop and remove data (DANGER)
docker compose down -v
```

### Database Connection (DBeaver, pgAdmin)

```
Host:     127.0.0.1
Port:     5432
Database: social_finance  (from .env DB_NAME)
User:     social_finance  (from .env DB_USER)
Password: CHANGE_ME (from .env DB_PASSWORD)
```

### Hot Reload

The development container runs [Air](https://github.com/cosmtrek/air) for Go hot
reload. Modifying source files under `backend/` triggers automatic recompilation
and server restart.

### Health Check

```bash
curl http://localhost:8080/health
```

Returns:

- **HTTP 200** — `{"database":"healthy","status":"ok"}` when all systems are healthy
- **HTTP 503** — `{"database":"unhealthy","status":"unhealthy"}` when PostgreSQL is unreachable

## Project Status

| Phase | Title | Status |
|-------|-------|--------|
| 0 | Requirements & Architecture | COMPLETE |
| 1 | Backend Foundation | **COMPLETE** |
| 2 | Authentication & Authorization | **COMPLETE** |
| 3 | RT, Household, Resident Management | Planned |
| 4 | Financial Categories & Ledger Foundation | Planned |
| 5 | Dues & Billing | Planned |
| 6 | Payments | Planned |
| 7 | Financial Reports & Dashboard | Planned |
| 8 | Android (Flutter) Foundation | Planned |
| 9 | Android Integration | Planned |
| 10 | Testing & Hardening | Planned |
| 11 | Deployment | Planned |

See [docs/development-plan.md](docs/development-plan.md) for the full roadmap.

## Directory Structure

```
social-finance/
├── .env.example          # Environment variable templates
├── .gitignore            # Excludes .env and build artifacts
├── docker-compose.yml    # Development stack (backend + postgres + migrate)
├── Makefile              # Convenience commands
├── README.md             # This file
├── AGENTS.md             # AI coding assistant instructions
├── backend/              # Go backend
│   ├── cmd/api/          # Application entry point + router
│   ├── internal/         # Internal packages
│   │   ├── auth/         # Authentication & authorization
│   │   ├── config/       # Environment configuration
│   │   ├── database/     # PostgreSQL connection pool
│   │   ├── health/       # Health check handler
│   │   └── http/         # Middleware (request ID, logging, panic recovery)
│   ├── fixtures/         # Test fixture utilities
│   ├── migrations/       # Database migrations
│   ├── Dockerfile        # Production multi-stage Dockerfile
│   ├── Dockerfile.dev    # Development Dockerfile (builds binary, installs Air)
│   ├── Dockerfile.migrate# Migration runner image
│   ├── .air.toml         # Air hot-reload configuration
│   ├── docker-entrypoint.sh # Copies pre-built binary for bind mount survival
│   ├── migrate-wrapper.sh  # Wrapper for migrate with default connection flags
│   ├── tmp/              # Pre-compiled server binary (created at container start)
│   ├── go.mod            # Go module definition
│   └── go.sum            # Dependency checksums
├── docs/                 # Architecture and planning documentation
└── mobile/             # Flutter mobile app (Phase 8)
```

## License

Private project.
