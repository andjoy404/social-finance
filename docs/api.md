# Social Finance - API Design

## 1. API Overview

- Base path: `/api/v1/`
- All endpoints use REST conventions with noun resource names.
- All responses are JSON.
- Authenticated endpoints require `Authorization: Bearer <token>` header.
- Content-Type: `application/json` for all requests and responses.

---

## 2. Response Format

### 2.1 Successful Responses

#### Singleton Resource

HTTP `200 OK` with resource JSON body. HTTP `201 Created` for newly created resources.

#### Collection (Paginated)

```json
{
  "data": [...],
  "pagination": {
    "page": 1,
    "per_page": 20,
    "total": 150,
    "total_pages": 8
  }
}
```

Parameters:
- `page` (default: 1)
- `per_page` (default: 20, max: 100)

#### Empty Collection

```json
{
  "data": [],
  "pagination": {
    "page": 1,
    "per_page": 20,
    "total": 0,
    "total_pages": 0
  }
}
```

#### Success Without Body

HTTP `204 No Content`.

#### Created Resource

HTTP `201 Created` with the created resource body.

---

## 3. Error Response Format

All error responses use this structure:

```json
{
  "error": {
    "code": "validation_error",
    "message": "Invalid request body",
    "details": [
      {
        "field": "email",
        "message": "Field is required"
      },
      {
        "field": "password",
        "message": "Must be at least 8 characters"
      }
    ]
  }
}
```

Error codes are machine-readable:

| HTTP Status | Error Code | Description |
|------------|------------|-------------|
| 400 | `validation_error` | Bad request body, missing fields, invalid format |
| 401 | `unauthorized` | Missing or invalid authentication token |
| 401 | `token_expired` | Access token has expired |
| 403 | `forbidden` | Authenticated but insufficient permissions |
| 404 | `not_found` | Resource does not exist or no access |
| 409 | `conflict` | Duplicate resource |
| 429 | `rate_limited` | Too many requests |
| 500 | `internal_error` | Unexpected server error |

---

## 4. Authenticated Request Model

### 4.1 JWT Claims

Access tokens include:

| Claim | Type | Description |
|-------|------|-------------|
| `sub` | string (UUID) | User ID |
| `sys_role` | string | System role: `super_admin` or `null` |
| `role` | string | Tenant role: `pengurus`, `bendahara`, `warga` (null for super_admin) |
| `rt_id` | string (UUID) | Tenant ID (null for super_admin) |
| `membership_id` | string (UUID) | Tenant membership ID |
| `tenant_role` | string | Same as `role` |
| `iat` | number | Issued at timestamp |
| `exp` | number | Expiration timestamp |

### 4.2 Token Lifecycle

- **Access tokens**: 15-minute lifetime, stateless JWT, HS256 or RS256.
- **Refresh tokens**: Opaque random strings (32 bytes, URL-safe base64), 7-day lifetime.
- **Rotation**: Each refresh generates new access + refresh tokens; old refresh token revoked.
- **Revocation**: Logout or password change invalidates the refresh token.

---

## 5. Implemented Endpoints

### 5.1 Health Check

```
GET /health
```

Authentication: **None**

Response (200 OK):
```json
{
  "status": "ok",
  "database": "healthy"
}
```

Response (503):
```json
{
  "status": "unhealthy",
  "database": "unhealthy"
}
```

### 5.2 Login

```
POST /api/v1/auth/login
```

Authentication: **None**

Request:
```json
{
  "email": "user@example.com",
  "password": "secret123"
}
```

Response (200 OK):
```json
{
  "access_token": "<jwt>",
  "refresh_token": "<opaque_string>",
  "token_type": "Bearer",
  "expires_in": 900,
  "user": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "user@example.com",
    "full_name": "Budi Santoso",
    "phone": "+6281234567890",
    "role": "bendahara",
    "is_active": true,
    "rt_id": "a]b0c1d2-e3f4-5678-90ab-cdef123456789",
    "rt_name": "RT 05 / RW 03",
    "system_role": null
  }
}
```

Rate limit: 10 requests per 5 minutes per source IP (TCP endpoint-stripped `RemoteAddr`).

### 5.3 Refresh Token

```
POST /api/v1/auth/refresh
```

Authentication: **None** (refresh token in body)

Request:
```json
{
  "refresh_token": "<opaque_string>"
}
```

Response (200 OK):
```json
{
  "access_token": "<new_jwt>",
  "refresh_token": "<new_opaque_string>",
  "token_type": "Bearer",
  "expires_in": 900
}
```

Rate limit: 30 requests per 15 minutes per source IP.

Behavior:
- Validates refresh token hash against database.
- On success: generates new access token + new refresh token.
- Invalidates old refresh token (`replaced_by_hash` set, `revoked_at` set).
- Rate-limited independently from login.

### 5.4 Logout

```
POST /api/v1/auth/logout
```

Authentication: **Required (Bearer token + refresh token)**

Headers: `Authorization: Bearer <access_token>`

Request:
```json
{
  "refresh_token": "<opaque_string>"
}
```

Response: 204 No Content

Behavior: Invalidates the current refresh token.

### 5.5 Get Own Profile

```
GET /api/v1/auth/me
```

Authentication: **Required (Bearer token)**

Response (200 OK):
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "email": "user@example.com",
  "full_name": "Budi Santoso",
  "phone": "+6281234567890",
  "role": "bendahara",
  "rt_id": "a]b0c1d2-e3f4-5678-90ab-cdef123456789",
  "rt_name": "RT 05 / RW 03",
  "system_role": null
}
```

---

## 5.6 RT Management (SUPER_ADMIN)

```
GET    /api/v1/rts
POST   /api/v1/rts
GET    /api/v1/rts/{id}
PATCH  /api/v1/rts/{id}
PATCH  /api/v1/rts/{id}/deactivate
```

Authentication: **SUPER_ADMIN** (system-level)
Authorization: SUPER_ADMIN role required. RT CRUD is a global operation — not tenant-scoped.

Request body (CREATE):
```json
{
  "name": "RT 05 / RW 03",
  "rw": 3,
  "rt": "002",
  "address": "Jalan Mawar No. 1",
  "head_name": "Budi Santoso"
}
```

Request body (UPDATE):
```json
{
  "name": "RT 05 / RW 03 — Updated",
  "head_name": "Andi Wijaya"
}
```

**Active RT uniqueness:** Only one active RT can own a given `(rw, rt)` combination. Creating or updating an RT to reuse an (rw, rt) code already occupied by another active RT returns HTTP 409 Conflict. Deactivating an RT frees its (rw, rt) code for reuse.

Response:
```json
{
  "id": "04ecf485-c707-41e1-b753-f164a4a3bef2",
  "name": "RT 05 / RW 03",
  "rw": 3,
  "rt": "002",
  "address": "Jalan Mawar No. 1",
  "head_name": "Budi Santoso",
  "is_active": true,
  "created_at": "2026-09-16T00:00:00Z",
  "updated_at": "2026-09-16T00:00:00Z"
}
```

Collection response (LIST):
```json
{
  "data": [ /* RT objects */ ],
  "pagination": {
    "page": 1,
    "per_page": 20,
    "total": 150,
    "total_pages": 8
  }
}
```

---

## 6. Household & Resident Management

### 6.1 List Households

```
GET /api/v1/households
```

Authentication: **Required (Bearer token)**
Role: **Warga, Bendahara, Pengurus**

Query parameters:
- `page` (default: 1) — page number
- `page_size` (default: 20, max: 100) — items per page
- `is_active` — filter by active status (`true`/`false`)
- `search` — ILIKE search on `head_name`, `address`, `kk_number`

Tenant scope: Derived from JWT `rt_id` claim. Returns only households for the user's RT.
Default read behavior: returns active households only (unless `is_active=false`).

Response (200 OK):
```json
{
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "rt_id": "a]b0c1d2-e3f4-5678-90ab-cdef123456789",
      "kk_number": "3201234567890123",
      "head_name": "Budi Santoso",
      "address": "Jl. Merdeka No. 10",
      "phone": "+6281234567890",
      "occupancy_status": "OWNER",
      "is_active": true,
      "created_at": "2026-09-16T10:00:00Z",
      "updated_at": "2026-09-16T10:00:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "page_size": 20,
    "total": 50,
    "total_pages": 3
  }
}
```

`occupancy_status` is one of `OWNER`, `TENANT`, or `null` (for legacy records before this field was required).

### 6.2 Get Household

```
GET /api/v1/households/{id}
```

Authentication: **Required (Bearer token)**
Role: **Warga, Bendahara, Pengurus**

Tenant scope: Derived from JWT `rt_id`. Returns household only if it belongs to user's RT and is active.

Response (200 OK):
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "rt_id": "a]b0c1d2-e3f4-5678-90ab-cdef123456789",
  "kk_number": "3201234567890123",
  "head_name": "Budi Santoso",
  "address": "Jl. Merdeka No. 10",
  "phone": "+6281234567890",
  "occupancy_status": "OWNER",
  "is_active": true,
  "created_at": "2026-09-16T10:00:00Z",
  "updated_at": "2026-09-16T10:00:00Z"
}
```

`occupancy_status` is one of `OWNER`, `TENANT`, or `null` (for legacy records).

### 6.3 Create Household

```
POST /api/v1/households
```

Authentication: **Required (Bearer token)**
Role: **Pengurus** only

Request:
```json
{
  "kk_number": "3201234567890123",
  "head_name": "Budi Santoso",
  "address": "Jl. Merdeka No. 10",
  "phone": "+6281234567890",
  "occupancy_status": "OWNER"
}
```

- `head_name` is required and must be non-empty.
- `occupancy_status` is required and must be one of `OWNER` or `TENANT`.
- `kk_number` is optional but must be unique among active households in the RT.
- `rt_id` is derived from JWT — never from request body.

Response (201 Created): the created household object.

### 6.4 Update Household

```
PATCH /api/v1/households/{id}
```

Authentication: **Required (Bearer token)**
Role: **Pengurus** only

Request body: partial update, any combination of:
```json
{
  "kk_number": "3201234567890123",
  "head_name": "Budi Santoso",
  "address": "Jl. Merdeka No. 10",
  "phone": "+6281234567890",
  "occupancy_status": "OWNER",
  "is_active": true
}
```

- `occupancy_status` is optional. May be set to `OWNER` or `TENANT`. Omitting it preserves the existing value.
- Updates only provided fields (nil = skip).
- Target household must belong to requesting RT and be active.

Response (200 OK): updated household object.

### 6.5 Deactivate Household

```
DELETE /api/v1/households/{id}
```

Authentication: **Required (Bearer token)**
Role: **Pengurus** only

- Performs soft delete: sets `is_active = false`.
- Household must belong to requesting RT and currently be active.
- No response body (204 No Content).

### 6.6 List Residents

```
GET /api/v1/residents
```

Authentication: **Required (Bearer token)**
Role: **Warga, Bendahara, Pengurus**

Query parameters:
- `page` (default: 1) — page number
- `page_size` (default: 20, max: 100) — items per page
- `household_id` (optional UUID) — filter to a specific household
- `is_active` (optional: `true`/`false`) — filter by active status
- `search` (optional) — ILIKE search on `full_name`

Tenant scope: Derived from JWT `rt_id`. Default: active residents only unless `is_active=false`.

Response (200 OK): paginated list of resident objects. Resident objects include projected RT details (`rt_number`, `rw`, `rt_name`) when available.

### 6.7 Get Resident

```
GET /api/v1/residents/{id}
```

Authentication: **Required (Bearer token)**
Role: **Warga, Bendahara, Pengurus**

Tenant scope: Derived from JWT `rt_id`. Returns resident if belonging to user's RT and is active.

Response (200 OK): the resident object:
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440001",
  "rt_id": "550e8400-e29b-41d4-a716-446655440000",
  "household_id": "550e8400-e29b-41d4-a716-446655440002",
  "full_name": "Ahmad Subarjo",
  "phone": "+6281234567890",
  "nik": "3201234567890001",
  "email": "ahmad@example.com",
  "relationship_to_head": "HEAD",
  "is_active": true,
  "created_at": "2026-09-16T10:00:00Z",
  "updated_at": "2026-09-16T10:00:00Z",
  "rt_number": "03",
  "rw": 16,
  "rt_name": "Wisma Rukun Tunggal"
}
```

### 6.8 Create Resident

```
POST /api/v1/residents
```

Authentication: **Required (Bearer token)**
Role: **Pengurus** only

Request:
```json
{
  "household_id": "550e8400-e29b-41d4-a716-446655440000",
  "full_name": "Siti Aminah",
  "phone": "+6281234567891",
  "relationship_to_head": "spouse"
}
```

- `full_name` is required and must be non-empty.
- `household_id` must reference a household in the requesting RT.
- Cross-tenant household assignment is rejected (404 if household not in RT).

Response (201 Created): the created resident object.

### 6.9 Update Resident

```
PATCH /api/v1/residents/{id}
```

Authentication: **Required (Bearer token)**
Role: **Pengurus** only

Request body: partial update, any combination of:
```json
{
  "full_name": "Siti Aminah",
  "phone": "+6281234567891",
  "relationship_to_head": "spouse",
  "is_active": true
}
```

- Target resident must belong to requesting RT and be active.

### 6.10 Deactivate Resident

```
DELETE /api/v1/residents/{id}
```

Authentication: **Required (Bearer token)**
Role: **Pengurus** only

- Soft delete: sets `is_active = false`.
- Target resident must belong to requesting RT and currently be active.
- No response body (204 No Content).

---

## 7. Internal Endpoints

### 6.1 Bootstrap (System-Level)

```
docker compose run --rm backend server --bootstrap
```

This is NOT an HTTP endpoint. It runs as a CLI subcommand (`--bootstrap` flag) to create the first SUPER_ADMIN user when the database is empty.

Environment variables:
- `BOOTSTRAP_EMAIL` — email for the super admin (required)
- `BOOTSTRAP_PASSWORD` — password for the super admin (required)
- `BOOTSTRAP_NAME` — full name (default: "Super Admin")

Idempotent: If a super admin already exists, the command is a no-op.

---

## 7. Tenant Isolation

**The `rt_id` is never part of the URL path or request body.** It flows through the JWT token issued during authentication.

- Auth middleware extracts `rt_id` from JWT claims.
- All repository methods append `WHERE rt_id = ?` to queries.
- TENANT users derive their `rt_id` from the JWT; client-supplied values are ignored.
- SUPER_ADMIN users have `rt_id: null` and operate via system-level endpoints.

---

## 8. API Conventions

### 8.1 Pagination

All collection endpoints support:
- `page` (default: 1)
- `page_size` (default: 20, max: 100)

### 8.2 Filtering

Optional query parameters. No implicit sorting unless specified.

### 8.3 REST Conventions

| Method | Collection | Singleton |
|--------|-----------|-----------|
| GET | List (paginated) | Get by ID |
| POST | Create | — |
| PUT | — | Full update |
| PATCH | — | Partial update |
| DELETE | — | Soft delete (is_active = false) |

### 8.4 HTTP Method Semantics

- `POST` — create new resource
- `GET` — read resource(s)
- `PUT` — full update (idempotent)
- `PATCH` — partial update
- `DELETE` — soft delete (set `is_active = false`)

---

## 9. Financial Endpoints (Phase 4)

### 9.1 Financial Categories

- `GET /api/v1/categories`: List categories (Bendahara, Pengurus, Warga).
- `POST /api/v1/categories`: Create category (`name`, `type`: `income`|`expense`) (Pengurus).
- `PATCH /api/v1/categories/{id}`: Update category (Pengurus).
- `DELETE /api/v1/categories/{id}`: Soft delete category (Pengurus; Bendahara cannot delete categories).

### 9.2 Dues Configuration

- `GET /api/v1/dues`: List dues (Bendahara, Pengurus, Warga).
- `POST /api/v1/dues`: Create dues (`name`, `amount`, `period_type`: `monthly`|`yearly`|`one_time`) (Bendahara, Pengurus).
- `PATCH /api/v1/dues/{id}`: Update dues (Bendahara, Pengurus).
- `DELETE /api/v1/dues/{id}`: Soft delete dues (Bendahara, Pengurus).

### 9.3 Bills

- `GET /api/v1/bills`: List bills (Bendahara, Pengurus, Warga [scoped to own household occupancy]).
- `GET /api/v1/bills/{id}`: Get bill by ID.
- `POST /api/v1/bills/generate`: Batch generate bills for a due & period across active occupancies (Bendahara, Pengurus).
- `POST /api/v1/bills`: Create single bill (Bendahara, Pengurus).

### 9.4 Payments

- `GET /api/v1/payments`: List payments (Bendahara, Pengurus, Warga [scoped to own payments]).
- `GET /api/v1/payments/{id}`: Get payment by ID.
- `POST /api/v1/payments`: Create payment (Warga [self-submitted with proof], Bendahara/Pengurus [staff-recorded]). Supports `Idempotency-Key` header.
- `POST /api/v1/payments/{id}/verify`: Verify payment (`action`: `approve`|`reject`) (Bendahara, Pengurus). Approval marks bill paid and posts income transaction to ledger.

### 9.5 Transactions & Ledger

- `GET /api/v1/transactions`: Query immutable ledger entries (Bendahara, Pengurus, Warga).
- `POST /api/v1/transactions`: Create manual ledger transaction (Bendahara, Pengurus). Supports `Idempotency-Key` header.
- `POST /api/v1/transactions/{id}/reverse`: Create compensating reversal transaction (Bendahara, Pengurus).
- `GET /api/v1/reports/balance`: Calculate real-time net balance from posted transactions: `sum(income) - sum(expense)` (Bendahara, Pengurus, Warga). NEVER derived or denormalized.

---

## 10. Warga Import & Export Endpoints

### 10.1 Warga Export
- `GET /api/v1/warga/export` (Pengurus, Bendahara)
  - Exports tenant-scoped current resident and household projection to CSV.
  - Response: `Content-Type: text/csv; charset=utf-8`, `Content-Disposition: attachment; filename="warga_export.csv"`.
  - Columns: `house_number,kk_number,head_name,occupancy_status,address,household_phone,resident_name,resident_phone,relationship_to_head`.

### 10.2 Warga Import Preview
- `POST /api/v1/warga/import/preview` (Pengurus)
  - Validates CSV upload without mutating database.
  - Identifies valid rows, invalid rows, duplicates, conflicts, and format errors.
  - Response:
    ```json
    {
      "total_rows": 10,
      "valid_rows": 8,
      "invalid_rows": 2,
      "errors": [
        {
          "row": 3,
          "field": "occupancy_status",
          "code": "invalid_occupancy_status",
          "message": "occupancy_status must be OWNER or TENANT"
        }
      ],
      "preview_data": []
    }
    ```

### 10.3 Warga Import Commit
- `POST /api/v1/warga/import/commit` (Pengurus)
  - Atomically creates physical houses, households, current household occupancies, residents, and current residency periods from CSV rows.
  - Runs in a strict transaction boundary; aborts on any validation error.
  - Never fabricates retrospective dates; uses explicit onboarding start dates (`CURRENT_DATE`).
  - Response: `201 Created` with summary:
    ```json
    {
      "status": "committed",
      "imported_households": 5,
      "imported_residents": 12
    }
    ```

