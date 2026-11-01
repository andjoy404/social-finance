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
- **Refresh tokens**: Opaque random strings (32 bytes, URL-safe base64), 90-day lifetime.
- **Rotation**: Each refresh generates new access + refresh tokens; old refresh token revoked.
- **Revocation**: Logout or password change invalidates the refresh token.
- **Device binding**: Each refresh token is associated with a `device_id` from the client. Login and refresh requests should include `device_id` in the request body for session tracking.

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

Request (supports email, phone, or identifier):
```json
{
  "email": "user@example.com",
  "password": "secret123"
}
```
Or with phone number:
```json
{
  "phone": "+6281234567890",
  "password": "secret123"
}
```
Or with generic identifier (email or phone):
```json
{
  "identifier": "081234567890",
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
  "system_role": null,
  "jabatan": null,
  "effective_permissions": [
    "finance.read"
  ]
}
```

`effective_permissions` is an array of effective finance permission codes used by
the Iuran client (`finance.read`, `finance.create`, `finance.update`, and
`finance.approve`). The server computes each entry through the same
`PermissionResolver` used by finance endpoint middleware and includes it only
when the endpoint's existing role gate also permits that operation. Users with
a `jabatan` are resolved against their active RT membership and
`position_permissions`; role-based users follow the existing `RequireRole`
rules. System-level `super_admin` follows the existing middleware bypass.
Permission lookup failures fail the `/auth/me` request with an internal error;
they do not produce elevated or success-shaped permission data. This additive
field does not change existing profile fields or make client-supplied
permissions authoritative for backend operations.

---

## 5.6 RW & RT Management (SUPER_ADMIN)

### RW Management
```
GET    /api/v1/rws
POST   /api/v1/rws
GET    /api/v1/rws/{id}
PATCH  /api/v1/rws/{id}
PATCH  /api/v1/rws/{id}/deactivate
```

Authentication: **Required (Bearer token)**
Authorization: `GET /api/v1/rws` and `GET /api/v1/rws/{id}` are accessible to authenticated users (with SUPER_ADMIN seeing all RWs and tenant users seeing their RW); write operations (`POST`, `PATCH`, `deactivate`) require **SUPER_ADMIN**.

Request body (CREATE RW):
```json
{
  "rw_number": 16,
  "name": "RW 016",
  "head_name": "Hendra Wijaya",
  "address": "Sekretariat RW 016"
}
```

Response (RW Object):
```json
{
  "id": "e4ecf485-c707-41e1-b753-f164a4a3bef2",
  "rw_number": 16,
  "name": "RW 016",
  "head_name": "Hendra Wijaya",
  "address": "Sekretariat RW 016",
  "rt_count": 5,
  "is_active": true,
  "created_at": "2026-09-16T00:00:00Z",
  "updated_at": "2026-09-16T00:00:00Z"
}
```

### RT Management
```
GET    /api/v1/rts
POST   /api/v1/rts
GET    /api/v1/rts/{id}
PATCH  /api/v1/rts/{id}
PATCH  /api/v1/rts/{id}/deactivate
```

Authentication: **SUPER_ADMIN** (system-level)
Authorization: SUPER_ADMIN role required. RT CRUD is a global operation — not tenant-scoped. `GET /api/v1/rts` supports optional query parameters `page`, `per_page`, `is_active`, and `rw_id`.

Request body (CREATE):
```json
{
  "name": "RT 05 / RW 03",
  "rw_id": "e4ecf485-c707-41e1-b753-f164a4a3bef2",
  "rw": 3,
  "rt": "002",
  "address": "Jalan Mawar No. 1",
  "head_name": "Budi Santoso"
}
```
If `rw_id` is provided, the server resolves the RW entity and synchronizes `rw` (`rw_number`). If `rw` is provided without `rw_id`, the server links to the matching active RW entity (or auto-provisions it if not yet present).

Request body (UPDATE):
```json
{
  "name": "RT 05 / RW 03 — Updated",
  "rw_id": "e4ecf485-c707-41e1-b753-f164a4a3bef2",
  "head_name": "Andi Wijaya"
}
```

**Active RT uniqueness:** Only one active RT can own a given `(rw, rt)` combination. Creating or updating an RT to reuse an (rw, rt) code already occupied by another active RT returns HTTP 409 Conflict. Deactivating an RT frees its (rw, rt) code for reuse.

Response:
```json
{
  "id": "04ecf485-c707-41e1-b753-f164a4a3bef2",
  "rw_id": "e4ecf485-c707-41e1-b753-f164a4a3bef2",
  "rw_name": "RW 003",
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

## 5.7 RW Operational Read-Only Endpoints

These four endpoints provide read-only financial and operational data aggregated across all RTs within an RW.

**Authorization:** Bearer token with role `pengurus`, `bendahara`, `Ketua`, `Wakil Ketua`, `Sekretaris`, atau `Bendahara` jabatan yang memiliki keanggotaan aktif pada RW yang diminta. Endpoint ini tidak bisa diakses oleh `warga`. **System-level SUPER_ADMIN** dapat mengakses semua RW endpoint tanpa memerlukan RW membership.

**Access control:** `RequireAuth` (JWT) → `RequireRWScope` (env-based allowlist) → `checkRWAccess()` (active RW membership). The `rwId` path parameter is matched against the user's active RW memberships on the server. The `rw_id` is **never** derived from the client — it is always resolved from the user's JWT claims and verified against the database.

**Authorization layers (from strictest to most permissive):**

| Layer | Middleware | Bypass for SUPER_ADMIN? | What it enforces |
|-------|-----------|------------------------|------------------|
| 1 | `RequireAuth` | No | Valid JWT Bearer token required |
| 2 | `RequireRWScope` | **Scope gate only** | RW ID must be in `SCOPED_RW_IDS`; SA must have active RW in DB |
| 3 | `checkRWAccess()` | **Full bypass** | Non-SA users must have active RW membership for the requested RW |

**Authorization rules by user type:**

| User Type | Membership Required | RW ID Must Be In `SCOPED_RW_IDS` | RW Must Be Active in DB |
|-----------|---------------------|----------------------------------|------------------------|
| **SUPER_ADMIN** | No | Yes (deployment gate) | Yes |
| **Ketua RT** | No (via RT) | Yes | Yes |
| **Wakil Ketua RT** | No (via RT) | Yes | Yes |
| **Pengurus / Bendahara** | No (via role) | Yes | Yes |
| **Ketua RW** | Yes | Yes | Yes |
| **Wakil Ketua RW** | Yes | Yes | Yes |
| **Sekretaris RW** | Yes | Yes | Yes |
| **Bendahara RW** | Yes | Yes | Yes |
| **Warga** | No access | N/A | N/A |

**What "no membership required" means:** Users with RT-level positions (Ketua RT, Wakil Ketua RT) or tenant-level roles (Pengurus, Bendahara) can access RW data without a specific RW membership record. They are authorized by their RT/tenant context. Users with RW-level positions (Ketua RW, Sekretaris RW, Bendahara RW) must have an active RW membership record.

---

### GET `/api/v1/rw/{rwId}/dashboard`

Returns an aggregated financial overview for the RW, including total RTs, fund locations, total balance, total income/expense, and transaction count. Only RTs with `is_active = true` are included.

**Path parameters:**

| Parameter | Type   | Required | Description                          |
|-----------|--------|----------|--------------------------------------|
| `rwId`    | string | Yes      | UUID of the RW (from database)       |

**Query parameters:** None.

**Response:** `200 OK`

```json
{
  "data": {
    "total_rts": 3,
    "total_locations": 5,
    "total_balance": "15000000.00",
    "total_income": "25000000.00",
    "total_expense": "10000000.00",
    "transaction_count": 42
  }
}
```

**Field description:**

| Field                | Type   | Description                                        |
|----------------------|--------|----------------------------------------------------|
| `total_rts`          | int64  | Count of active RTs in this RW                     |
| `total_locations`    | int64  | Count of active fund locations across all RTs      |
| `total_balance`      | string | Sum of all `location_balance_changes.amount_delta` across active RTs, formatted `"X.XX"` |
| `total_income`       | string | Sum of `transactions.amount` where `type = 'income'` and `status = 'posted'` |
| `total_expense`      | string | Sum of `transactions.amount` where `type = 'expense'` and `status = 'posted'` |
| `transaction_count`  | int64  | Count of posted transactions across all RTs        |

**Monetary format:** All monetary values are strings with exactly 2 decimal places (e.g., `"0.00"`, `"100000.00"`).

**Error responses:**

| Status | Code | Description |
|--------|------|-------------|
| 401 | `unauthorized` | Missing or invalid Bearer token |
| 403 | `forbidden` | User has no active RW membership (or not in configured scope) |
| 403 | `forbidden` | RW not found or inactive in database |
| 403 | `forbidden` | RW ID not in `SCOPED_RW_IDS` (applies to all users including SUPER_ADMIN) |
| 400 | `bad_request` | Invalid `rwId` format |
| 500 | `internal_error` | Database or internal error |

---

### GET `/api/v1/rw/{rwId}/kas`

Returns a KAS report aggregating income and expense across all RTs in the RW. Optionally filtered to a specific month.

**Path parameters:**

| Parameter | Type   | Required | Description                          |
|-----------|--------|----------|--------------------------------------|
| `rwId`    | string | Yes      | UUID of the RW (from database)       |

**Query parameters:**

| Parameter | Type   | Required | Description                                          |
|-----------|--------|----------|------------------------------------------------------|
| `period`  | string | No       | Month filter in `YYYY-MM` format (e.g., `"2026-10"`) |

If `period` is omitted, returns all-time totals.

**Response (all-time):** `200 OK`

```json
{
  "data": {
    "total_income": "25000000.00",
    "total_expense": "10000000.00",
    "net_balance": "15000000.00",
    "transaction_count": 42
  }
}
```

**Response (period-filtered):** `200 OK`

```json
{
  "data": {
    "total_income": "2000000.00",
    "total_expense": "800000.00",
    "net_balance": "1200000.00",
    "transaction_count": 5
  }
}
```

**Field description:**

| Field                | Type   | Description                                              |
|----------------------|--------|----------------------------------------------------------|
| `total_income`       | string | Income within filter scope, formatted `"X.XX"`           |
| `total_expense`      | string | Expense within filter scope, formatted `"X.XX"`          |
| `net_balance`        | string | `total_income - total_expense`, formatted `"X.XX"`       |
| `transaction_count`  | int64  | Count of posted transactions within filter scope         |

**Error responses:**

| Status | Code | Description |
|--------|------|-------------|
| 401 | `unauthorized` | Missing or invalid Bearer token |
| 403 | `forbidden` | User has no active RW membership (or not in configured scope) |
| 403 | `forbidden` | RW not found or inactive in database |
| 400 | `bad_request` | Invalid `rwId` format or invalid `period` format |
| 500 | `internal_error` | Database or internal error |

---

### GET `/api/v1/rw/{rwId}/kas/transactions`

Returns a paginated list of posted financial transactions from all RTs in the RW. Only `status = 'posted'` transactions are included.

**Path parameters:**

| Parameter | Type   | Required | Description                          |
|-----------|--------|----------|--------------------------------------|
| `rwId`    | string | Yes      | UUID of the RW (from database)       |

**Query parameters:**

| Parameter   | Type   | Required | Description                                                 |
|-------------|--------|----------|-------------------------------------------------------------|
| `period`    | string | No       | Month filter in `YYYY-MM` format                            |
| `start_date`| string | No       | Date filter in `YYYY-MM-DD` format (inclusive)              |
| `end_date`  | string | No       | Date filter in `YYYY-MM-DD` format (inclusive)              |
| `page`      | int    | No       | Page number, default 1, minimum 1                           |
| `page_size` | int    | No       | Items per page, default 20, maximum 100                     |

Filters are combined with AND logic. At least one of `period`, `start_date`, or `end_date` may be provided.

**Response:** `200 OK`

```json
{
  "data": [
    {
      "id": "a1b2c3d4-...",
      "rt_id": "e5f6a7b8-...",
      "rt_name": "RT 03 / RW 05",
      "type": "income",
      "amount": "500000.00",
      "description": "Iuran warga bulan Oktober",
      "category_id": "c1d2e3f4-...",
      "status": "posted",
      "payment_id": null,
      "occurred_at": "2026-10-15T10:30:00+07:00",
      "created_at": "2026-10-15T10:35:00+07:00"
    }
  ],
  "pagination": {
    "page": 1,
    "per_page": 20,
    "total": 42,
    "total_pages": 3
  }
}
```

**Field description:**

| Field             | Type   | Description                                                    |
|-------------------|--------|----------------------------------------------------------------|
| `id`              | string | UUID of the transaction                                        |
| `rt_id`           | string | UUID of the RT that owns this transaction                      |
| `rt_name`         | string | Display name of the RT (e.g., `"RT 03 / RW 05"`)              |
| `type`            | string | `"income"` or `"expense"`                                      |
| `amount`          | string | Amount as `"X.XX"` string                                      |
| `description`     | string | Transaction description                                        |
| `category_id`     | string or null | UUID of the financial category, or null              |
| `status`          | string | Always `"posted"` (filtered at query level)                    |
| `payment_id`      | string or null | UUID of the related payment, or null                     |
| `occurred_at`     | string | ISO 8601 timestamp of the transaction (RFC 3339)             |
| `created_at`      | string | ISO 8601 timestamp of creation (RFC 3339)                    |

**Pagination:** Empty results return `{"data": [], "pagination": {"page": 1, "per_page": 20, "total": 0, "total_pages": 0}}`.

**Error responses:**

| Status | Code | Description |
|--------|------|-------------|
| 401 | `unauthorized` | Missing or invalid Bearer token |
| 403 | `forbidden` | User has no active RW membership (or not in configured scope) |
| 403 | `forbidden` | RW not found or inactive in database |
| 400 | `bad_request` | Invalid parameters (rwId, period, dates, pagination) |
| 500 | `internal_error` | Database or internal error |

---

### GET `/api/v1/rw/{rwId}/locations`

Returns a read-only list of fund locations across all RTs in the RW. The `current_balance` for each location is computed from `SUM(location_balance_changes.amount_delta)`.

**Path parameters:**

| Parameter | Type   | Required | Description                          |
|-----------|--------|----------|--------------------------------------|
| `rwId`    | string | Yes      | UUID of the RW (from database)       |

**Query parameters:**

| Parameter  | Type   | Required | Description                                    |
|------------|--------|----------|------------------------------------------------|
| `is_active`| string | No       | Filter by active status: `"true"`, `"1"`, `"false"`, `"0"` |

If omitted, returns all locations (both active and inactive) within active RTs.

**Response:** `200 OK`

```json
{
  "data": [
    {
      "id": "b2c3d4e5-...",
      "rt_id": "f6a7b8c9-...",
      "rt_name": "RT 01 / RW 05",
      "name": "Kas RT 01",
      "category": "kas_umum",
      "bank_name": "BRI",
      "account_number": "1234567890",
      "account_holder": "Ahmad Budi",
      "is_active": true,
      "current_balance": "1250000.00",
      "created_at": "2026-01-15T00:00:00+07:00"
    }
  ]
}
```

**Field description:**

| Field                | Type   | Description                                              |
|----------------------|--------|----------------------------------------------------------|
| `id`                 | string | UUID of the fund location                                |
| `rt_id`              | string | UUID of the RT that owns this location                   |
| `rt_name`            | string | Display name of the RT                                   |
| `name`               | string | Human-readable location name                             |
| `category`           | string | Category code (e.g., `"kas_umum"`, `"dana_kegiatan"`)    |
| `bank_name`          | string or null | Bank name, or null if not set                   |
| `account_number`     | string or null | Bank account number, or null if not set           |
| `account_holder`     | string or null | Account holder name, or null if not set             |
| `is_active`          | bool   | Whether the location is active                           |
| `current_balance`    | string | Computed balance from `SUM(amount_delta)`, formatted `"X.XX"` |
| `created_at`         | string | ISO 8601 timestamp of creation (RFC 3339)              |

**Sort order:** Results are sorted by `rt_name`, then `category`, then `name` (ascending).

**Balance computation:** `current_balance` equals `COALESCE(SUM(lbc.amount_delta), 0)` where `lbc` is `location_balance_changes`. Locations with no balance change entries return `"0.00"`.

**Error responses:**

| Status | Code         | Description                                        |
|--------|--------------|----------------------------------------------------|
| 401    | unauthorized | Missing or invalid Bearer token                    |
| 403    | forbidden    | User has no active RW membership                   |
| 400    | bad_request  | Invalid `rwId` format or invalid `is_active` value |
| 500    | internal_error | Database or internal error                       |

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
  "phone": "+6281234567891"
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

- `GET /api/v1/dues`: List dues (Bendahara, Pengurus, Warga). Each due includes `valid_until` (`YYYY-MM-DD` or `null`).
- `POST /api/v1/dues`: Create dues (`name`, `amount`, `period_type`: `monthly`|`yearly`|`one_time`, optional `is_voluntary`, optional `valid_until`, optional `due_type`: `regular`|`voluntary`|`multi_month`, optional `year`) (Bendahara, Pengurus). `valid_until` is accepted only for voluntary dues and must not be in the past. For `multi_month`, only 1 issuance is allowed per calendar year per RT; duplicate issuance returns 400 with message `"IPL Multi-Bulan untuk tahun {year} sudah pernah diterbitkan di RT ini."`.
- `PATCH /api/v1/dues/{id}`: Update dues (Bendahara, Pengurus), including `valid_until` for voluntary dues. Send `valid_until: ""` to clear an existing deadline. Activating an expired voluntary due in the same request requires a `valid_until` of today or later; otherwise the request is rejected.
- `DELETE /api/v1/dues/{id}`: Soft delete dues (Bendahara, Pengurus).

Voluntary expiry rule:
- A voluntary due stays active through the whole `valid_until` date in `Asia/Jakarta` and expires at `00:00 WIB` the following date.
- A backend job runs daily at `00:05 WIB` to set expired voluntary dues to `is_active = false`.
- Bill generation and new donations are rejected synchronously once the due is expired, even if the scheduler has not run yet. Existing bills (including arrears) remain payable, and historical payments/transactions are unchanged.
- `valid_until` is ignored/cleared for regular dues; regular dues never expire automatically.

### 9.3 Bills

- `GET /api/v1/bills`: List bills (Bendahara, Pengurus, Warga [scoped to own household occupancy]). Query parameters: `page`, `page_size`, `household_occupancy_id`, `due_id`, `status`, `payment_status`, `is_active`, `period`, `search` / `q` (case-insensitive substring filter matching bill/due name), `order` (`unpaid_first` for unpaid/partial bills first chronologically, followed by paid bills in reverse chronological order).
- `GET /api/v1/bills/{id}`: Get bill by ID.
- `POST /api/v1/bills/generate`: Batch generate bills for a due & period across active occupancies (Bendahara, Pengurus).
- `POST /api/v1/bills`: Create single bill (Bendahara, Pengurus).
- `PATCH /api/v1/bills/{id}/status`: Update bill active status (`{"is_active": boolean}`) (Bendahara, Pengurus). Rejects deactivation if bill is already paid.
- `GET /api/v1/dues/{id}/months`: Get 12-month calendar summary and bill counts for a Multi-Bulan due (Bendahara, Pengurus).
- `PATCH /api/v1/dues/{id}/periods/{period}/status`: Update active status for all unpaid bills of this due in the specified period (`{"is_active": boolean}`) (Bendahara, Pengurus). Rejects deactivation if all bills in the period are already paid.

### 9.4 Payments

- `GET /api/v1/payments`: List payments (Bendahara, Pengurus, Warga [scoped to own payments]).
- `GET /api/v1/payments/{id}`: Get payment by ID.
- `POST /api/v1/payments`: Create payment (Warga [self-submitted with proof], Bendahara/Pengurus [staff-recorded]). Supports `Idempotency-Key` header. Concurrent requests with the same key are serialized in PostgreSQL; an identical request replays its stored response, while reusing the key with a different user, path, or payload returns `409 Conflict`. The payment and its idempotency response are committed atomically. The request must contain exactly one target: `bill_id` for ordinary bill payments or `due_id` for donations against an active, voluntary, Rp0 due in the authenticated RT. Donation responses include `bill_id: null` and the selected `due_id`. `amount` must be positive and `method` must be `CASH` or `TRANSFER`. A voluntary Rp0 due creates no bill; pending donations do not affect the ledger.
- `POST /api/v1/payments/{id}/verify`: Verify payment (`action`: `approve`|`reject`) (Bendahara, Pengurus). Approval marks an ordinary bill paid and posts one income transaction to the ledger; approval of a donation posts one income transaction without attempting to update a bill. Rejected payments do not affect the ledger. Only the submitter can read or manage their own donation as a Warga.

### 9.5 Transactions & Ledger

- `GET /api/v1/transactions`: Query immutable ledger entries (Bendahara, Pengurus, Warga). Supports query parameters: `category_id`, `type`, `status`, `period` (`YYYY-MM`), `start_date`, `end_date`, `page`, `per_page`. Results are ordered by `occurred_at DESC, created_at DESC`. Each transaction includes `reference_no` and `payment_no` (for manual KAS transactions, the server auto-generates `KAS-YYYYMMDD-XXXXX`; for Iuran payments, `PAY-YYYYMMDD-XXXXX`).
- `POST /api/v1/transactions`: Create manual ledger transaction (Bendahara, Pengurus). Supports `Idempotency-Key` header. Automatically generates a unique `reference_no` (`KAS-YYYYMMDD-XXXXX`) persisted in the database and returned in the response.
- `POST /api/v1/transactions/{id}/reverse`: Create compensating reversal transaction (Bendahara, Pengurus).
- `GET /api/v1/reports/balance`: Calculate real-time balance from posted transactions (Bendahara, Pengurus, Warga).
  - Without parameters: returns cumulative net balance (`total_income`, `total_expense`, `net_balance`, `transaction_count`).
  - With `period=YYYY-MM` or (`start_date` and `end_date`): returns period balance sheet (`opening_balance`, `total_income`, `total_expense`, `net_movement`, `closing_balance`, `transaction_count`). Calculations use `Asia/Jakarta` timezone and `[start, end_exclusive)` interval on `posted` transactions.
- `PUT /api/v1/reports/balance/opening/{period}`: Set or correct opening balance for a period in `period_opening_balances` (Bendahara, Pengurus). Request body requires `opening_balance` and `reason`. Logs audit trail with before/after values and marks source as `manual`.
- `GET /api/v1/dashboard/summary`: Aggregated financial dashboard summary (Bendahara, Pengurus, Warga, SuperAdmin). Supports `period=YYYY-MM` or `period=all` (cumulative all-time metrics). Returns `period`, `rt_number`, `rw_number`, `total_balance`, `period_income`, `period_expense`, `dues_progress`, `payment_method_breakdown` (`cash_amount`, `transfer_amount`, `cash_count`, `transfer_count`), `monthly_chart`, and `recent_transactions` (including `payment_no`/`reference_no`).

---

## 10. Warga Import & Export Endpoints

### 10.1 Warga Export
- `GET /api/v1/warga/export` (Pengurus, Bendahara)
  - Exports tenant-scoped current resident and household projection to CSV.
  - Response: `Content-Type: text/csv; charset=utf-8`, `Content-Disposition: attachment; filename="warga_export.csv"`.
  - Columns: `house_number,kk_number,head_name,occupancy_status,address,household_phone,resident_name,resident_phone`.

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

---

## 11. Notification Management Endpoints

Accessible strictly to **SUPER_ADMIN** and **Ketua RT** (`jabatan: ketua`).

### 11.1 List System Notification Types
- `GET /api/v1/notifications/system-types`
  - Returns inventory of supported system events, triggers, channels, and allowed variables.

### 11.2 List Notification Templates
- `GET /api/v1/notifications/templates?event_type=&is_active=`
  - Returns list of configured templates for the tenant RT.

### 11.3 Create Notification Template
- `POST /api/v1/notifications/templates`
  - Body: `{"name": "...", "event_type": "...", "title": "...", "body": "...", "image_url": "...", "channel": "push"}`
  - Validation: `image_url` is only permitted for `waste_truck_arrival` and `manual_push`. Rejects image for Iuran events.

### 11.4 Get Notification Template Detail
- `GET /api/v1/notifications/templates/{id}`

### 11.5 Update Notification Template
- `PUT /api/v1/notifications/templates/{id}`
  - Body: `{"name": "...", "title": "...", "body": "...", "image_url": "...", "is_active": true}`

### 11.6 Toggle Notification Template Active Status
- `PATCH /api/v1/notifications/templates/{id}/toggle-active`
  - Toggles `is_active` flag.

### 11.7 Delete Notification Template
- `DELETE /api/v1/notifications/templates/{id}`

### 11.8 List Notification Delivery Logs / Riwayat Notifikasi
- `GET /api/v1/notifications/logs?page=1&page_size=20&event_type=&status=`
  - Accessible to all authenticated RT members (Warga, Pengurus, Bendahara, Super Admin).
  - Scope:
    - **Warga**: Returns personal notifications targeted to the caller (`user_id = auth.UserID`) plus public RT announcements (`event_type IN ('manual_push', 'waste_truck_arrival')` or `user_id IS NULL`) in the caller's RT. Test pushes and other residents' private notifications are excluded.
    - **Officers (Ketua RT, Super Admin, Pengurus, Bendahara)**: Returns all delivery records for the tenant RT.
  - Query parameters:
    - `page` (default 1), `page_size` (default 20, max 100)
    - `event_type` (optional filter)
    - `status` (optional: `unread`, `read`, or `all`)
  - Response includes `is_read: boolean` and `read_at: string | null` per item for the authenticated caller.

### 11.9 Notification Read Receipts & Unread Count
- `GET /api/v1/notifications/unread-count`
  - Returns total number of unread notifications for the caller in their tenant RT: `{"unread_count": 3}`.
- `POST /api/v1/notifications/logs/{id}/read` (also supports `PATCH`)
  - Marks notification `id` as read for the caller. Validates recipient ownership/scope. Idempotent.
- `POST /api/v1/notifications/logs/read-all` (also supports `PATCH`)
  - Marks all eligible unread notifications as read for the caller in their tenant RT.

### 11.10 Trigger Manual Push Broadcast
- `POST /api/v1/notifications/manual`
  - Body: `{"title": "...", "body": "...", "image_url": "..."}`
  - Permitted: **SUPER_ADMIN** and **Ketua RT** (`jabatan: ketua`).
  - Broadcasts to all active device tokens within the caller's tenant RT.

### 11.11 Send Test Push Notification
- `POST /api/v1/notifications/test-push`
  - Body: `{"event_type": "...", "template_id": "...", "title": "...", "body": "...", "image_url": "..."}`
  - Permitted: **SUPER_ADMIN** only (`system_role = super_admin`).
  - Targets ONLY the active device tokens belonging to the caller Super Admin.

### 11.12 Upload Notification Image
- `POST /api/v1/notifications/upload-image`
  - Multipart form file `image` and form field `event_type`.
  - Permitted: **SUPER_ADMIN** and **Ketua RT**.
  - Stores image in RustFS under `notifications/waste-truck/` or `notifications/manual/`. Returns public relative stream URL.

### 11.13 Stream Notification Image
- `GET /api/v1/notifications/images/*`
  - Public read endpoint streaming notification images stored in RustFS.

---

## 12. RW Finance & Fund Locations API

Endpoints scoped to RW organizations under `/api/v1/rw/{rwId}/...`. Protected by `RequireRWScope` middleware.
Permissions:
- `finance.read`: Ketua RW, Wakil Ketua RW, Sekretaris RW, Bendahara RW, Super Admin.
- `finance.create`: Ketua RW, Bendahara RW, Super Admin.
- `finance.update`: Ketua RW, Bendahara RW, Super Admin.

### 12.1 RW Dashboard Summary
- `GET /api/v1/rw/{rwId}/dashboard?period=YYYY-MM`
  - Returns `{ "data": { "total_rts": 5, "total_locations": 3, "total_balance": "...", "total_income": "...", "total_expense": "...", "transaction_count": 12 } }`.

### 12.2 RW KAS Report & Transactions
- `GET /api/v1/rw/{rwId}/kas?period=YYYY-MM`
  - Returns `{ "data": { "total_income": "...", "total_expense": "...", "net_balance": "...", "transaction_count": 12 } }`.
- `GET /api/v1/rw/{rwId}/kas/transactions?page=1&page_size=20&period=YYYY-MM`
  - Paginated transactions with `rt_name`, `location_name`, and `source_rt_name`.
- `POST /api/v1/rw/{rwId}/kas/transactions`
  - Body: `{"type": "income" | "expense", "amount": "100000.00", "location_id": "uuid", "description": "...", "payment_method": "transfer" | "cash", "source_rt_id": "uuid" (optional)}`
  - Records KAS transaction, appends `rw_location_balance_changes` mutation, checks sufficient balance on expense.
- `POST /api/v1/rw/{rwId}/kas/transactions/{id}/proof`
  - Multipart form upload for transaction proof (`file` field). Max 5MB.
- `GET /api/v1/rw/{rwId}/kas/transactions/{id}/proof`
  - Streams the proof image for an authorized RW transaction.

### 12.3 RW Fund Locations CRUD & Management
Supports 3 categories: `rekening`, `brankas`, and `coh`. Dana Hibah/Donasi is eliminated.
- `GET /api/v1/rw/{rwId}/locations?category=&is_active=`
  - Returns array of active/filtered fund locations with calculated current balance.
- `POST /api/v1/rw/{rwId}/locations`
  - Body: `{"name": "...", "category": "rekening" | "brankas" | "coh", "bank_name": "...", "account_number": "...", "account_holder": "..."}`
- `GET /api/v1/rw/{rwId}/locations/{id}`
  - Returns detail of fund location `id`.
- `PUT /api/v1/rw/{rwId}/locations/{id}`
  - Body: `{"name": "...", "bank_name": "...", "account_number": "...", "account_holder": "..."}`
- `PATCH /api/v1/rw/{rwId}/locations/{id}/status`
  - Body: `{"is_active": boolean}`. Deactivation strictly blocked if `current_balance != 0`.
- `GET /api/v1/rw/{rwId}/locations/{id}/mutations?limit=50`
  - Returns ledger mutation history (`rw_location_balance_changes`) for location `id`.
- `GET /api/v1/rw/{rwId}/locations/reconcile`
  - Returns reconciliation report comparing calculated ledger KAS balance with total sum of active location balances.

### 12.4 RW Fund Transfers
- `GET /api/v1/rw/{rwId}/transfers?page=1&page_size=20`
  - Returns list of intra-RW fund transfers.
- `POST /api/v1/rw/{rwId}/transfers`
  - Body: `{"from_location_id": "uuid", "to_location_id": "uuid", "amount": "...", "transfer_date": "YYYY-MM-DD", "notes": "..."}`
  - Validates source has sufficient balance and both locations belong to the same RW.
- `POST /api/v1/rw/{rwId}/transfers/{id}/reverse`
  - Body: `{"reason": "..."}`
  - Reverses previous transfer if recipient has sufficient balance to refund. Restores sender balance.

## 13. Resident Password Reset via 4-Digit Code

### 13.1 Generate Reset Code (Pengurus RT / Super Admin)
- `POST /api/v1/residents/{id}/reset-password-code`
  - Authorization: Pengurus of the resident's RT or Super Admin.
  - Action: Generates a cryptographically secure 4-digit numeric code, hashes it with Argon2id, invalidates previous active codes for this resident, and stores with a 24-hour expiration window.
  - Response:
    ```json
    {
      "code": "0037",
      "expires_at": "2026-11-02T18:00:00Z",
      "resident_id": "uuid",
      "resident_name": "Budi Santoso",
      "identifier": "budi@example.com"
    }
    ```

### 13.2 Redeem Reset Code (Public / Resident)
- `POST /api/v1/auth/reset-password-with-code`
  - Rate limited: 5 requests per 15 minutes per IP/identifier.
  - Body:
    ```json
    {
      "identifier": "budi@example.com",
      "code": "0037",
      "new_password": "NewSecurePassword123",
      "confirm_password": "NewSecurePassword123"
    }
    ```
  - Validates 4-digit code against latest non-expired active code hash, atomically updates password hash in `users`, marks code as used, revokes active refresh tokens, and writes audit trail.
  - Response: `200 OK`
    ```json
    {
      "status": "success",
      "message": "Kata sandi berhasil diubah. Silakan masuk dengan kata sandi baru."
    }
    ```

