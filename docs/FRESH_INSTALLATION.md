# Fresh Installation Guide — Social Finance Urena

This document describes how to perform a fresh production-like installation of Social Finance from scratch, with zero demo data and no automatic seeding.

## 1. Prerequisites

- Docker and Docker Compose installed
- Git client
- At least 4 GB RAM available for Docker containers

## 2. Environment Setup

### 2.1 Clone the Repository

```bash
git clone <repository-url>
cd social-finance
git checkout urena
```

### 2.2 Create Environment File

Copy `.env.example` to `.env` and fill in the required values:

```bash
cp .env.example .env
```

**Required environment variables** (do NOT use defaults in production):

| Variable | Description | Example |
|----------|-------------|---------|
| `JWT_SECRET` | Secret key for JWT signing (min 32 bytes) | `your-32-byte-minimum-secret-key-here` |
| `BOOTSTRAP_EMAIL` | Email for first Super Admin (required for `--bootstrap`) | `admin@yourdomain.com` |
| `BOOTSTRAP_PASSWORD` | Password for first Super Admin (required for `--bootstrap`) | `StrongPassword123!` |
| `BOOTSTRAP_NAME` | Display name for first Super Admin | `Super Admin` |

**Optional environment variables** with defaults:

| Variable | Default | Description |
|----------|---------|-------------|
| `APP_ENV` | `development` | Application environment (`development`, `test`, `production`) |
| `HTTP_PORT` | `8080` | Backend HTTP port |
| `DB_HOST` | `postgres` | PostgreSQL host (Docker service name) |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_NAME` | `social_finance` | Database name |
| `DB_USER` | `social_finance` | Database user |
| `DB_PASSWORD` | `social_finance_dev_password` | Database password |
| `JWT_EXPIRY` | `900` | Access token lifetime in seconds (15 minutes) |
| `REFRESH_EXPIRY` | `604800` | Refresh token lifetime in seconds (7 days) |
| `RUSTFS_ENDPOINT` | `rustfs:9000` | Object storage endpoint |
| `RUSTFS_ACCESS_KEY` | `rustfsadmin` | Object storage access key |
| `RUSTFS_SECRET_KEY` | `rustfsadmin123` | Object storage secret key |
| `RUSTFS_BUCKET` | `social-finance-payment-proofs` | Object storage bucket name |
| `RUSTFS_USE_SSL` | `false` | Use SSL for object storage |
| `FCM_PROJECT_ID` | `social-finance-urena` | Firebase Cloud Messaging project ID |
| `PUBLIC_BASE_URL` | `` | Public base URL for the application |
| `API_PUBLIC_URL` | `` | Public API URL |
| `SCOPED_RT_IDS` | `` | Comma-separated RT IDs allowed (frontend) |
| `SCOPED_RW_IDS` | `` | Comma-separated RW IDs allowed (frontend) |

**Never commit `.env`** — it contains secrets and is git-ignored.

## 3. Docker Compose Fresh Start

### 3.1 Stop and Remove Existing Containers (Optional)

If you have existing containers from a previous installation:

```bash
docker compose down
```

> **Warning:** `docker compose down -v` will delete all data including PostgreSQL volumes. Only run this if you intend to start completely fresh.

### 3.2 Build and Start Infrastructure

Start PostgreSQL, RustFS, and the migration runner:

```bash
docker compose up -d postgres rustfs rustfs-init migrate
```

Wait for all services to become healthy:

```bash
docker compose ps
```

All services should show `healthy` or `completed` status.

### 3.3 Apply Database Migrations

Migrations are applied manually via the `migrate` service:

```bash
docker compose run --rm migrate up
```

This applies all migration files from `001` to the latest in sequential order. Migrations are idempotent — running again reports "no change".

Verify migration status:

```bash
docker compose run --rm migrate version
```

### 3.4 Start Backend

```bash
docker compose up -d backend
```

The backend starts without any automatic seeding or bootstrapping. No demo users, RTs, or data are created.

### 3.5 Start Frontend (Optional)

```bash
docker compose up -d frontend
```

The frontend connects to the backend at `http://backend:8080` internally.

## 4. First-Time Super Admin Provisioning

A fresh installation has **no users**. You must create the first Super Admin explicitly.

### 4.1 Run Bootstrap

```bash
docker compose run --rm backend /tmp/build/api --bootstrap
```

This requires `BOOTSTRAP_EMAIL` and `BOOTSTRAP_PASSWORD` to be set in `.env`.

### 4.2 What Bootstrap Does

- Creates a user with `system_role = 'super_admin'`
- Hashes the password with Argon2id (never plaintext)
- Creates an active RW membership in the canonical RT 002 (Urena)
- Is **idempotent**: running again when a Super Admin already exists is a no-op
- **Cannot be re-run** to create additional Super Admins after the first exists

### 4.3 Verify Bootstrap

Check that the Super Admin was created:

```bash
docker compose exec backend psql -U social_finance -d social_finance -c "SELECT email, system_role, is_active FROM users WHERE system_role = 'super_admin';"
```

## 5. First Login

1. Open the frontend at `http://localhost:8081` (or your configured URL)
2. You will be redirected to `/login`
3. Login with the Super Admin email and password from `.env`
4. You are now authenticated as Super Admin

## 6. Create RT and RW

After logging in as Super Admin:

1. Navigate to **RT** management to create RTs (Rukun Tetangga)
2. Navigate to **RW** management to create RWs (Rukun Warga)
3. Each RW can have multiple RTs under it

## 7. Create Households and Residents

1. Navigate to **Warga** (Residents) to create households and residents
2. Use the import feature to bulk-import residents from Excel/CSV
3. Each resident must have a valid NIK and unique email

## 8. Create Pengurus (RT/RW Management)

1. Assign `pengurus`, `bendahara`, `warga`, or `sekretaris` roles to residents
2. `pengurus` and `bendahara` get full RW write access
3. `warga` gets read-only access to their own household data
4. `sekretaris` gets notification read access

## 9. Empty State Behavior

When no data exists:

| Screen | Behavior |
|--------|----------|
| RT List | Shows "Tidak ada data RT." (empty banner) |
| RW List | Shows "Belum ada data RW." (empty banner) |
| Dashboard | Shows zero balances, no transactions |
| Warga List | Shows empty state, "Tambah" button available |
| Kas List | Shows empty state, "Catat Kas Masuk/Keluar" buttons available |

All "Tambah" (Add) buttons remain functional — you can create the first record of any type.

## 10. Running Demo Seed (Development Only)

Demo seed is **disabled by default**. To run it explicitly for development:

```bash
docker compose --profile seed up seed
```

Or run once:

```bash
docker compose run --rm seed
```

The seed service requires `APP_ENV=development` or `APP_ENV=test` — it will NOT run in production.

## 11. Running Tests

### Backend Tests

```bash
cd backend
go build ./...
go vet ./...
go test -count=1 ./...
```

### Frontend Tests

```bash
cd frontend
npx tsc --noEmit
npx vitest run
npx vite build
```

## 12. Troubleshooting

### Backend fails to start

Check logs:

```bash
docker compose logs backend
```

Common causes:
- `JWT_SECRET` not set → backend exits with error
- Database not ready → wait for postgres to become healthy
- Migration not applied → run `docker compose run --rm migrate up`

### Cannot login after bootstrap

1. Verify Super Admin was created: check `users` table
2. Verify `BOOTSTRAP_EMAIL` matches the login email
3. Check backend logs for authentication errors

### Migration fails

1. Check migration logs: `docker compose logs migrate`
2. Verify all migration SQL files are valid
3. Run `docker compose run --rm migrate version` to see current version

### Frontend cannot connect to backend

1. Verify backend is running: `docker compose ps backend`
2. Check frontend network configuration
3. Verify `PUBLIC_BASE_URL` and `API_PUBLIC_URL` if set

## 13. Production vs Local Differences

| Aspect | Local Development | Production |
|--------|----------------|------------|
| `APP_ENV` | `development` | `production` |
| Bootstrap | Auto-runs on non-production startup (disabled in this version) | Must run `--bootstrap` explicitly |
| Seed service | Available via `--profile seed` | Not available |
| Backend image | `Dockerfile.dev` (with Air hot reload) | `Dockerfile` (production, non-root) |
| Database SSL | `prefer` | `require` |
| Secrets | `.env` file | Production secret management |

## 14. Security Notes

- **Never** commit `.env` to version control
- **Never** use default/bootstrap credentials in production
- **Always** set a strong `JWT_SECRET` (min 32 bytes)
- **Always** run migrations before starting backend
- **Always** create Super Admin via `--bootstrap` before first login
- PostgreSQL is bound to Docker internal network only (not exposed publicly)
