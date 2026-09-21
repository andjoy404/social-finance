# Social Finance - Security Requirements

## 1. Authentication

### 1.1 Password Storage

- Passwords are hashed server-side using **Argon2id** (preferred) or **bcrypt** with a cost factor of at least 12.
- A unique, randomly generated salt is used per password.
- Passwords are **never** stored in plaintext, logs, or audit data.

### 1.2 Token-Based Authentication

#### Access Token
- Type: JWT (JSON Web Token)
- Algorithm: `RS256` (asymmetric) or `HS256` (symmetric, acceptable for MVP)
- Lifetime: **15 minutes** (900 seconds)
- Payload contains: `sub` (user ID), `role`, `rt_id`, `iat`, `exp`
- Tokens are stateless; no server-side token store required.

#### Refresh Token
- Type: Opaque random token (not JWT)
- Lifetime: **7 days**
- Stored server-side in `users.refresh_token` as a hash (argon2)
- Used to obtain new access tokens without re-authentication.
- Single-use: each refresh invalidates the previous refresh token (rotation).
- Revoked on logout, password change, or account deactivation.

### 1.3 Login Process

1. User submits email and password.
2. Server verifies credentials against hashed password.
3. On success: generate access token and refresh token, store refresh token hash.
4. On failure: log attempt (event type `user.login_failed`), return generic error.

### 1.4 Brute Force Protection

- Rate limit login endpoints: max 10 attempts per 5 minutes per source IP.
- After excessive failed attempts, return `429 Too Many Requests`.

---

## 2. Authorization

### 2.1 Role-Based Access Control (RBAC)

All authenticated endpoints enforce role-based access. The authorization middleware:

1. Decodes and validates the access token.
2. Extracts the `role` claim.
3. Checks the role against the route's permission table.
4. Returns `403 Forbidden` if unauthorized.

### 2.2 Tenant Isolation

**All tenant-scoped data access is derived from the authenticated user's JWT `rt_id` claim. The `rt_id` is NEVER trusted from client input.**

The auth middleware:
1. Verifies the JWT signature and expiration.
2. Extracts `rt_id` from JWT claims.
3. Injects `rt_id` into the request context.
4. Every repository layer query appends `WHERE rt_id = ?` automatically.

For SUPER_ADMIN users:
- `rt_id` is not required in the JWT.
- SUPER_ADMIN endpoints accept an explicit `rt_id` path parameter (e.g., `/api/v1/rt/:id/users`) that is validated against the `rts` table.

### 2.3 Resource-Level Authorization

Some endpoints require checking resource ownership:

- **WARGA** role: only see their own household's bills and payments. The handler checks if the resource's `household_id` matches the user's assigned `household_id`.
- This check is enforced server-side and cannot be bypassed by the client.

---

## 3. Request Validation

### 3.1 Input Validation

All request bodies and query parameters are validated before reaching the service layer:

- **Type checking:** JSON schema validation on all POST/PUT bodies.
- **Required fields:** All required fields must be present and non-empty.
- **Format validation:** email format, date format, phone format (if provided).
- **Enum validation:** status, role, type fields constrained to allowed values.
- **Length constraints:** text fields bounded by database column sizes.
- **Numeric constraints:** positive numbers where negative values are not valid (e.g., amounts).

### 3.2 SQL Injection Protection

- All database queries use **parameterized queries** (prepared statements).
- ORM or query builder must not allow raw SQL user input in query clauses.
- No string concatenation for SQL queries.

---

## 4. Data Protection

### 4.1 Transit Encryption

- **HTTPS/TLS required** in production.
- TLS 1.2 minimum. TLS 1.3 preferred.
- TLS terminated at the reverse proxy (Nginx/Caddy).

### 4.2 Sensitive Data Masking

| Field | Full Access | Masked Access | No Access |
|-------|------------|---------------|-----------|
| NIK | SUPER_ADMIN | Pengurusan, Bendahara (last 4 only) | WARGA |
| Phone | SUPER_ADMIN | Pengurusan, Bendahara | WARGA |
| Address | All roles | All roles | — |
| Password | N/A | N/A | N/A (never exposed) |

### 4.3 Audit Logging

All sensitive operations are logged. See `business-rules.md` for the full event list.

Audit log entries do NOT include:
- Passwords or password hashes
- Access tokens or refresh tokens
- Authentication credentials
- Full NIK numbers (masked if referenced)
- File attachment contents

---

## 5. Secure File Upload (Deferred to v2)

File upload for receipts/attachments will be secured with:

- File size limit: 5 MB per file
- Allowed MIME types: `image/jpeg`, `image/png`, `image/webp`
- Content-type verification (not just extension)
- Malware scanning (if feasible in v2)
- Storage: Cloud storage (S3-compatible) preferred over local filesystem
- URL generation: signed URLs with expiration

---

## 6. Rate Limiting

| Endpoint Group | Limit | Window |
|---------------|-------|--------|
| `/api/v1/auth/login` | 10 requests | 5 minutes |
| `/api/v1/auth/refresh` | 30 requests | 15 minutes |
| All other endpoints | 100 requests | 15 minutes |

Limits are per source IP address.

---

## 7. HTTP Security Headers

The reverse proxy adds these headers:

```
Strict-Transport-Security: max-age=31536000; includeSubDomains
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
X-XSS-Protection: 1; mode=block
Content-Security-Policy: default-src 'self'
Referrer-Policy: strict-origin-when-cross-origin
Cache-Control: no-store, no-cache, must-revalidate
```

---

## 8. Secrets Management

### 8.1 Environment Variables

All secrets are loaded from environment variables at startup. No secrets are:
- Hard-coded in source code
- Stored in configuration files tracked in Git
- Written to logs
- Passed in URL query strings

### 8.2 Required Environment Variables

| Variable | Purpose | Example |
|----------|---------|---------|
| `DB_HOST` | PostgreSQL host | `localhost` |
| `DB_PORT` | PostgreSQL port | `5432` |
| `DB_NAME` | Database name | `social_finance` |
| `DB_USER` | Database user | `social_finance` |
| `DB_PASSWORD` | Database password | (secret) |
| `JWT_SECRET` | JWT signing secret | (32+ char random) |
| `JWT_EXPIRY` | Access token lifetime | `900` |
| `REFRESH_EXPIRY` | Refresh token lifetime | `604800` |
| `APP_SECRET` | Encryption key for sensitive data | (32+ char random) |
| `LOG_LEVEL` | Logging verbosity | `info` |
| `UPLOAD_DIR` | File upload directory path | `/data/uploads` |
| `CORS_ALLOWED_ORIGINS` | Allowed CORS origins | `https://social-finance.example.com` |

### 8.3 Secrets File Location

```
# .env (NOT tracked in Git)
DB_PASSWORD=...
JWT_SECRET=...
```

```
# .env.example (tracked in Git — without actual values)
DB_PASSWORD=
JWT_SECRET=
```

---

## 9. Database Security

### 9.1 Database User Permissions

PostgreSQL uses a dedicated database user with minimal permissions:

- CREATE: no
- DROP: no
- ALTER: no (migrations run as a separate privileged user in CI/CD)
- SELECT, INSERT, UPDATE: within specific schemas

In production, the application user should only have access to the application schema, not system tables.

### 9.2 Connection Security

- Database connections use TLS where available.
- Connection pooling managed by the application server (e.g., `pgx` pool in Go).
- Maximum connection limit configured to prevent exhaustion.

### 9.3 Backups

- Automated daily database dumps.
- Backups stored offsite (S3-compatible storage).
- Backups retained for at least 30 days.
- Backup restore documented and tested periodically.

---

## 10. Production Configuration

### 10.1 Server Configuration

```yaml
server:
  host: 0.0.0.0
  port: 8080
  tls_enabled: true        # Enforced by reverse proxy
  read_timeout: 15s
  write_timeout: 30s
  idle_timeout: 120s
```

### 10.2 Health Check

```
GET /health
```

Returns `200 OK` with body:
```json
{
  "status": "ok",
  "version": "1.0.0",
  "database": "connected"
}
```

Returns `503 Service Unavailable` if the database connection fails.

### 10.3 Logging

Structured JSON logging:
```json
{
  "time": "2025-01-15T10:30:00Z",
  "level": "INFO",
  "msg": "request completed",
  "method": "GET",
  "path": "/api/v1/dashboard",
  "status": 200,
  "duration_ms": 45,
  "request_id": "abc-123"
}
```

**Never logged:**
- Request/response body containing passwords
- Access tokens or refresh tokens
- Full NIK numbers
- Authentication credentials

---

## 11. Android Client Security

### 11.1 Secure Token Storage

- Access and refresh tokens stored via `flutter_secure_storage` (wraps Android Keystore / iOS Keychain).
- Tokens are NEVER stored in SharedPreferences, plain text files, or web storage.
- Tokens are cleared on logout and app uninstall.

### 11.2 Certificate Pinning (Future)

For future implementation:
- Pin the server TLS certificate or public key.
- Prevents MITM attacks even with installed CA certificates.

### 11.3 Network Security

- All API calls use HTTPS only.
- Self-signed certificates rejected in production builds.
- No direct database or backend container access from the Android app — only through the API.

---

## 12. Docker & Infrastructure Security

### 12.1 Secrets Management

All secrets are loaded from environment variables (`.env` file). Critical rules:

- `.env` is added to `.gitignore` and NEVER committed.
- `.env.example` documents required variables with placeholder values (never real secrets).
- Secrets in `.env` may include:
  - Database credentials (`DB_PASSWORD`)
  - JWT signing secret (`JWT_SECRET`)
  - Application encryption key (`APP_SECRET`)
- In production, secrets are supplied via Docker secrets, Kubernetes secrets, or a secrets manager (HashiCorp Vault, AWS Secrets Manager).
- Secrets are NEVER hardcoded in `docker-compose.yml` — they are loaded through the `.env` file.

### 12.2 Docker Compose Security

- Development and production Docker files are separate: `Dockerfile` (production) and `Dockerfile.dev` (development).
- `Dockerfile` does NOT include development tools (Air, build dependencies).
- Production images run as a non-root user.
- PostgreSQL data volume (`postgres_data`) persists independently from container lifecycle.
- Container restart will NOT delete the database.
- PostgreSQL is NOT publicly exposed in production — only accessible via Docker internal network.
- Backend source code is NOT bind-mounted in production.
- Docker Compose uses an internal network (`social-finance-internal`) for backend-to-database communication.

### 12.3 Database Security in Docker

- PostgreSQL runs in a Docker container with a named volume (`postgres_data:/var/lib/postgresql/data`).
- The container uses a strong random password (or reads from `.env`).
- Connection from backend uses the Docker service hostname (e.g., `postgres`), NEVER `localhost` or `127.0.0.1`.
- PostgreSQL data is accessible during development via `127.0.0.1:5432` for DBeaver/pgAdmin/CLI tools.
- In production, PostgreSQL is accessible ONLY through the Backend container via Docker internal network.

### 12.4 Migration Security

- Migrations are executed manually via Docker: `docker compose run --rm migrate up`.
- Migrations are NEVER run automatically on application startup.
- The migration Docker image contains only the migration tool, NOT the application code.
- No destructive migrations are permitted without a corresponding down migration.
- Migrations are reviewed and approved before deployment to production.
