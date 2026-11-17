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
  ],
  "avatar_path": "community/550e8400-e29b-41d4-a716-446655440000/profile/1700000000_photo.jpg",
  "avatar_thumb_path": "community/550e8400-e29b-41d4-a716-446655440000/profile/1700000000_photo_thumb.jpg"
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
  "rt_name": "Wisma Rukun Tunggal",
  "avatar_path": "community/550e8400-e29b-41d4-a716-446655440000/profile/1700000000_photo.jpg",
  "avatar_thumb_path": "community/550e8400-e29b-41d4-a716-446655440000/profile/1700000000_photo_thumb.jpg"
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

### 6.11 Special Resident (Petugas Khusus)

- `POST /api/v1/residents/special` — Creates a special resident without a household chain (`jabatan` must be one of `'keamanan'`, `'operasional'`, or `'kebersihan'`).
  - Permitted: Pengurus or Bendahara role, Ketua/Wakil Ketua/Sekretaris/Bendahara position, or SUPER_ADMIN with a valid allowed-RT context.
  - Tenant scope always comes from the authenticated RT context; a tenant caller cannot select another `rt_id`.
  - Email must be unused. Existing accounts are not reused or moved between RTs; duplicate email returns `409 Conflict`.
  - The new membership uses role `warga` and the selected `jabatan`; an initial random password is set and is not returned. Use the authorized password-reset flow to establish credentials.
- `PATCH /api/v1/residents/special/{id}` — Updates a special resident and/or their `jabatan` (`'keamanan'`, `'operasional'`, or `'kebersihan'`). Uses the same access roles and tenant scope; cross-RT IDs return `404`.

### 6.12 Resident Profile Photo Management

- `POST /api/v1/residents/{id}/photo` — Upload resident profile photo.
  - Authentication: **Required (Bearer token)** with `resident.profile_photo.manage` permission (Ketua RT).
  - Content-Type: `multipart/form-data` with form field `file` (max 5 MB; JPEG, PNG, or WebP).
  - Processing: Stores original image and automatically generates an optimized 200×200 center-cropped JPEG thumbnail (quality 82).
  - Response (200 OK):
    ```json
    {
      "resident_id": "550e8400-e29b-41d4-a716-446655440001",
      "avatar_path": "community/550e8400-e29b-41d4-a716-446655440000/profile/1700000000_photo.jpg",
      "avatar_thumb_path": "community/550e8400-e29b-41d4-a716-446655440000/profile/1700000000_photo_thumb.jpg",
      "updated_at": "2026-11-13T10:00:00Z"
    }
    ```
- `DELETE /api/v1/residents/{id}/photo` — Remove resident profile photo.
  - Authentication: **Required (Bearer token)** with `resident.profile_photo.manage` permission (Ketua RT).
  - Clears both `avatar_path` and `avatar_thumb_path` in DB and deletes both objects from storage.
  - Response: 204 No Content.

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

- `GET /api/v1/bills`: List bills (Bendahara, Pengurus, Warga [scoped to own household occupancy]). Query parameters: `page`, `page_size`, `household_occupancy_id`, `due_id`, `status`, `payment_status`, `is_active`, `is_obligatory`, `period`, `search` / `q` (case-insensitive substring filter matching bill/due name), `order` (`unpaid_first` for unpaid/partial bills first chronologically, followed by paid bills in reverse chronological order). Each item includes `is_obligatory` (boolean indicating if the period is an obligation based on `move_in_date`), `move_in_date`, and `move_out_date`.
- `GET /api/v1/bills/{id}`: Get bill by ID. Includes `is_obligatory`, `move_in_date`, and `move_out_date`.
- `POST /api/v1/bills/generate`: Batch generate bills for a due & period across active occupancies (Bendahara, Pengurus).
- `POST /api/v1/bills`: Create single bill (Bendahara, Pengurus).
- `PATCH /api/v1/bills/{id}/status`: Update bill active status (`{"is_active": boolean}`) (Bendahara, Pengurus). Rejects deactivation if bill is already paid.
- `GET /api/v1/dues/{id}/months`: Get 12-month calendar summary and bill counts for a Multi-Bulan due (Bendahara, Pengurus).
- `PATCH /api/v1/dues/{id}/periods/{period}/status`: Update active status for all unpaid bills of this due in the specified period (`{"is_active": boolean}`) (Bendahara, Pengurus). Rejects deactivation if all bills in the period are already paid.

### 9.4 Payments

- `GET /api/v1/payments`: List payments (Bendahara, Pengurus, Warga [scoped to own payments]).
- `GET /api/v1/payments/{id}`: Get payment by ID.
- `POST /api/v1/payments`: Create payment (Warga [self-submitted with proof], Bendahara/Pengurus [staff-recorded]). Supports `Idempotency-Key` header. Concurrent requests with the same key are serialized in PostgreSQL; an identical request replays its stored response, while reusing the key with a different user, path, or payload returns `409 Conflict`. The payment and its idempotency response are committed atomically. The request must contain exactly one target: `bill_id` for ordinary bill payments or `due_id` for donations against an active, voluntary, Rp0 due in the authenticated RT. Donation responses include `bill_id: null` and the selected `due_id`. `amount` must be positive and `method` must be `CASH` or `TRANSFER`. A voluntary Rp0 due creates no bill; pending donations do not affect the ledger. Attempting to pay a non-obligatory bill (`is_obligatory = false`) returns `400 Bad Request`.
- `POST /api/v1/payments/multi-period`: Multi-period payment across consecutive periods in FIFO order (Warga, Bendahara, Pengurus). Non-obligatory periods (`is_obligatory = false`) are excluded from FIFO validation and cannot be paid. Trying to pay a non-obligatory period returns `400 Bad Request`.
- `POST /api/v1/payments/{id}/verify`: Verify payment (`action`: `approve`|`reject`) (Bendahara, Pengurus). Approval marks an ordinary bill paid and posts one income transaction to the ledger; approval of a donation posts one income transaction without attempting to update a bill. Rejected payments do not affect the ledger. Only the submitter can read or manage their own donation as a Warga.

### 9.5 Transactions & Ledger

- `GET /api/v1/transactions`: Query immutable ledger entries (Bendahara, Pengurus, Warga). Supports query parameters: `category_id`, `type`, `status`, `period` (`YYYY-MM`), `start_date`, `end_date`, `page`, `per_page`. Results are ordered by `occurred_at DESC, created_at DESC`. Each transaction includes `reference_no` and `payment_no` (for manual KAS transactions, the server auto-generates `KAS-YYYYMMDD-XXXXX`; for Iuran payments, `PAY-YYYYMMDD-XXXXX`).
- `POST /api/v1/transactions`: Create manual ledger transaction (Bendahara, Pengurus). Supports `Idempotency-Key` header. Automatically generates a unique `reference_no` (`KAS-YYYYMMDD-XXXXX`) persisted in the database and returned in the response. Supports `expense_category` for expense transactions.
- `GET /api/v1/transactions`: List ledger transactions with pagination and filters (`type`, `status`, `category_id`, `start_date`, `end_date`, `search` by reference_no/payment_no/description).
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

### 10.4 Warga Import Commit (XLSX)
- `POST /api/v1/warga/import/commit/xlsx` (Pengurus)
  - Excel (.xlsx) variant of the bulk import. Parses `house_number, head_name, occupancy_status, address?, resident_name, nik?, phone?, email?, move_in_date?, jabatan?, password?`.
  - Same atomic transaction semantics as 10.3.
  - When an import moves a resident's `move_in_date` / `residency_periods.start_date` backwards (or otherwise causes previously non-obligatory `bills` to become obligatory), the backend reconciles bill obligations inside the same transaction and reports the change **per affected occupancy**.
  - The response includes:
    ```json
    {
      "status": "committed",
      "imported_households": 1,
      "imported_residents": 1,
      "obligation_warnings": [
        {
          "household_occupancy_id": "…uuid…",
          "house_number": "AW-01",
          "resident_names": ["Budi A"],
          "activated_periods": ["2026-01", "2026-02", "2026-03", "2026-04", "2026-05", "2026-06", "2026-07"],
          "activated_bill_count": 7,
          "total_activated_amount": "700000.00"
        }
      ]
    }
    ```
  - `obligation_warnings` is present **only** when the synchronization actually flipped bills to `is_obligatory = true` with outstanding balance (amount minus approved APPROVED payment allocations and direct APPROVED payments). Fully paid bills that become obligatory are not reported.
  - Rows with no obligation change produce no warning; importing identical data again is idempotent and produces no warnings.
  - If any part of the import transaction aborts (validation, sync failure), no warning is emitted and no obligation/bill state is visible outside the transaction.
  - `total_activated_amount` is the sum of outstanding (amount minus approved paid) for the newly obligatory bills, formatted as `NUMERIC(15,2)` string.

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
    - Returns only records addressed to the caller (`user_id = auth.UserID`) in the caller's RT. Broadcast records are visible through their per-recipient delivery records, not through the sender's RT-wide dispatch record. Test pushes are visible only to their authenticated owner.
    - The user ID and RT scope come from the authenticated session; client-supplied recipient, sender, actor, or RT identifiers do not widen access.
  - Query parameters:
    - `page` (default 1), `page_size` (default 20, max 100)
    - `event_type` (optional filter)
    - `status` (optional: `unread`, `read`, or `all`)
  - Response includes `is_read: boolean` and `read_at: string | null` per item for the authenticated caller. `status` is the delivery result on that recipient's record; explicit provider success remains `SENT`, provider failure remains `FAILED`, and missing delivery evidence is `UNKNOWN`.

### 11.8.1 List Pengurus Notification Activity
- `GET /api/v1/notifications/management-history?category=system|mine&page=1&page_size=20`
  - Requires the existing `notification.manage` authorization and the caller's authorized RT scope.
  - `category=system` includes only scheduler executions proven by the `schedule:` delivery dedup key, grouped once per schedule execution.
  - `category=mine` preserves actions and broadcasts sent by the authenticated user and includes every actual notification recipient record addressed to them, including manual and waste-truck broadcasts. Recipient records are selected by authenticated `user_id` and tenant RT, not inferred from a sender's role or a broadcast's RT-wide parent record.
  - The caller identity is read from the authenticated token. There is no `sender_user_id` or actor query parameter.
  - Events without reliable system/actor provenance, including historical records that cannot be classified, are excluded rather than guessed into either category.
  - Response entries include `history_category`, `activity_relation` (`system`, `sent_by_me`, `received_by_me`, or `performed_by_me`), `status`, and `delivery_counts_available`. Payment action status is not a delivery status. A grouped scheduled activity is `PARTIAL` only with evidence of both a successful and a failed provider attempt, `SENT` when there is a successful attempt and no proven failure, `FAILED` when all recorded provider outcomes failed, and `UNKNOWN` when no provider outcome is known. Recipients with no token remain `UNKNOWN` individually and do not convert successful attempts into failures. Incomplete/legacy delivery evidence is not inferred as success or failure.
  - Waste-truck notifications are recorded once per eligible current household resident in `notification_logs`, in addition to the sender's dispatch record. Management history uses authenticated actor/recipient IDs and the token-derived RT; it does not expose another Pengurus's personal history.
  - `page_size` defaults to 20 and is capped at 100.
  - Users without `notification.manage` can use the regular `/notifications/logs` endpoint for their own received records only; management-history categories, templates, and schedules remain inaccessible.

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
- `POST /api/v1/notifications/waste-truck/upload-image`
  - Multipart form file `file` (or `image`) and form field `event_type`.
  - Permitted: existing RT officers; Petugas Keamanan only for `event_type=waste_truck_arrival` and only with `notification.send_waste_truck`.
  - Other event types, including `manual_push` for Petugas Keamanan, are forbidden.

### 11.13 Waste Truck Arrival
- `GET /api/v1/notifications/waste-truck/template`
  - Returns the active `waste_truck_arrival` template for the caller's RT.
  - Permitted: existing RT officers and Petugas Keamanan in the authenticated RT. Read access does not grant dispatch permission.
- `POST /api/v1/notifications/waste-truck`
  - Triggers a public arrival notification for current household residents in the caller's RT.
  - Permitted: existing RT officers or Petugas Keamanan with `notification.send_waste_truck`.
  - Dispatch is permitted to existing RT officers or Petugas Keamanan with `notification.send_waste_truck`; only this event may be dispatched by Petugas Keamanan.
  - Push recipients are current household residents with an active user, membership in the same RT, and residency/occupancy period currently in effect. Active device tokens are used when present; residents without a token remain visible in history with `UNKNOWN` delivery outcome. A Petugas Keamanan account is included only if it independently qualifies as a current household resident; no household or occupancy is synthesized.
  - Returns `202 Accepted`; this confirms dispatch was queued, not delivery. Delivery status is recorded asynchronously and is available from the sender-only history endpoint.
- `GET /api/v1/notifications/waste-truck/history?page=1&per_page=20`
  - Returns paginated waste-truck dispatch records sent by the authenticated Petugas Keamanan in the authenticated RT.
  - Requires `jabatan=keamanan` and the authenticated RT, but not dispatch permission. The backend filters by token-derived `rt_id` and `sender_user_id`; caller-supplied sender or tenant identifiers are not accepted.
  - Dispatch records include `event_type`, `title`, `body`, `image_url`, `status` (`PENDING`, `SENT`, `PARTIAL`, `FAILED`, or `UNKNOWN` when the result cannot be established), safe `failure_reason` when applicable, and dispatch `created_at`.

### 11.14 Stream Notification Image
- `GET /api/v1/notifications/images/*`
  - Public read endpoint streaming notification images stored in RustFS.

### 11.15 Notification Schedules API
- `GET /api/v1/notifications/schedules`
  - Lists all notification schedules for the caller's RT.
  - Requires `notification.manage` permission.
  - Response items include `id`, `name`, `event_type`, `template_id`, `send_days`, `send_hour`, `send_minute`, `timezone`, `is_active`, `last_executed_at`, `last_executed_status` (`success`, `partial`, `failed`, `no_device`, `no_recipients`), `last_executed_error` (safe error message), and `last_executed_summary` (`total_recipients`, `sent_count`, `failed_count`, `no_device_count`, `failure_reason`).
- `POST /api/v1/notifications/schedules`
  - Creates a new notification schedule for the caller's RT.
  - Body: `{"name": "...", "event_type": "...", "template_id": "...", "send_days": [1, 15], "send_hour": 8, "send_minute": 0, "timezone": "Asia/Jakarta"}`
- `PUT /api/v1/notifications/schedules/{id}`
  - Updates an existing schedule.
- `DELETE /api/v1/notifications/schedules/{id}`
  - Deletes a schedule.
- `PATCH /api/v1/notifications/schedules/{id}/toggle`
  - Toggles `is_active` state of the schedule.
- `GET /api/v1/notifications/schedules/history`
  - Lists aggregated execution history for notification schedules in the caller's RT.
  - Query parameters: `page`, `page_size`.
  - Supports configuration versioning: multiple executions on the same date with distinct delivery configurations appear as separate history records with their respective `config_hash`, execution time, and delivery metrics.
- `GET /api/v1/notifications/schedules/{schedule_id}/recipients`
  - Returns individual recipient delivery results for a specific schedule execution.
  - Query parameters: `exec_date` (required, YYYY-MM-DD), `config_hash` (optional, filters to a specific configuration version).

### 11.16 Activity (Kegiatan) Notifications
Activity lifecycle events are first-class, customizable notification events. Like other notification events, they honor RT scope and the `notification.manage` permission for management surfaces (creation/toggling/deleting templates and schedules).

Event types:
| Event | Trigger | Audience |
|---|---|---|
| `activity_created` | Pengurus RT creates or publishes an activity | Seluruh warga RT |
| `activity_updated` | Pengurus RT updates an activity | Seluruh warga RT |
| `activity_reminder` | One-shot schedule tied to a specific activity fires at `execution_at` | Seluruh warga RT |
| `activity_cancelled` | Pengurus RT cancels an activity | Seluruh warga RT |
| `activity_completed` | Activity ends (`ends_at` telah lewat) | Seluruh warga RT |

Permitted variables (all activity events):
- `{{nama_kegiatan}}` — the activity title.
- `{{deskripsi_kegiatan}}` — the activity description (em dash if missing).
- `{{tanggal_kegiatan}}` — starts_at date in Asia/Jakarta (`2 January 2006`).
- `{{jam_kegiatan}}` — starts_at time in Asia/Jakarta (`15:04`).
- `{{lokasi_kegiatan}}` — activity location.
- `{{penyelenggara}}` — activity organizer or issuing party.
- `{{nama_warga}}` — recipient's full name (per-recipient rendering).

**One-shot activity reminders.** `activity_reminder` schedules are created with `activity_id` + absolute `execution_at`, have no `send_days` semantics, and deactivate themselves after firing or after the activity terminates. When the activity is cancelled or completes (`ends_at` elapsed) the pending schedule is deactivated and `'skipped'` is recorded in `notification_schedule_executions`. History rows in `notification_logs` are never deleted — a cancellation is recorded as a new `activity_cancelled` log row.

Schedule endpoint bodies now accept:
```json
{
  "name": "Pengingat H-3 Kegiatan",
  "event_type": "activity_reminder",
  "template_id": "<template id>",
  "send_days": [],
  "send_hour": 8,
  "send_minute": 0,
  "timezone": "Asia/Jakarta",
  "activity_id": "<activity id>",
  "execution_at": "2026-11-20T09:00:00+07:00"
}
```
Constraints:
- `activity_reminder` requires a valid, published, uncancelled activity in the same RT.
- `execution_at` must be in the future at creation time.
- The schedule is disabled as soon as the activity is cancelled, or as soon as its `ends_at` elapses. Editing the activity later (or re-creating it) does **not** re-arm cancelled schedules — residual reminders must be created anew.
- Activity events are per-recipient fan-out; the body is rendered per user so `{{nama_warga}}` resolves to that user's name.


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
  - Body: `{"type": "income" | "expense", "amount": "100000.00", "location_id": "uuid", "description": "...", "payment_method": "transfer" | "cash", "source_rt_id": "uuid" (optional), "expense_category": "string" (optional)}`
  - Records KAS transaction, appends `rw_location_balance_changes` mutation, checks sufficient balance on expense.
- `POST /api/v1/rw/{rwId}/opening-balance`
  - Body: `{"opening_balance": "1000000.00"}`
  - Sets or updates persistent opening balance for RW. Formula: `Total Saldo = Saldo Awal + Saldo Bersih`.
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
  - Error responses (`400 Bad Request`):
    - `invalid_identity`: identitas akun atau kode tidak cocok
    - `invalid_code`: kode reset tidak sesuai
    - `code_expired`: kode reset sudah kedaluwarsa
    - `code_used`: kode reset sudah digunakan
  - Note: Pada endpoint `/api/v1/auth/login`, jika pengguna memasukkan 4-digit kode reset aktif pada input kata sandi biasa, sistem mengembalikan error `reset_code_entered` (HTTP 400) yang mengarahkan pengguna ke dialog "Lupa Kata Sandi?".

## 14. Community Dashboard API (Beranda, Kegiatan, Pengumuman, Banner, Struktur Organisasi)

RT-scoped Community module. Base rules: authentication required (JWT Bearer), plus a valid RT
context in an allowed RT (`002`, `003`) for every endpoint except the public-signed image route.
Authorization is permission-driven (single source of truth: `position_permissions` via the
`PermissionResolver`; Super Admin bypasses). There are no hardcoded jabatan shortcuts.

### 14.1 Read Endpoints (all authenticated RT members)
- `GET /api/v1/home/dashboard` — greeting, banners (in-window only), upcoming activities (published), organization structure, security on-duty (name, shift, avatar + phone number always included as the warga duty hotline).
- `GET /api/v1/activities?page=&page_size=&status=&publication_status=` — paginated. Warga only ever see published activities; status filter uses the dynamic execution status (`upcoming`, `ongoing`, `completed`, `cancelled`). Draft visibility requires `activities.create|update`.
- `GET /api/v1/activities/{id}` — detail. Draft/non-published returns 404 for non-managers.
- `GET /api/v1/announcements?page=&page_size=&status=` — paginated. Warga only see announcements inside the tayang window (`scheduled_start <= now < scheduled_end`); managers can filter by status.
- `GET /api/v1/announcements/{id}` — detail, same visibility rules (404 for out-of-window/draft for Warga).
- `GET /api/v1/banner-templates` / `GET /api/v1/banner-templates/{id}` — active templates (system or owned by the RT).
- `GET /api/v1/banners` — active banners in display window, merged with the generic iuran reminder (in stable order: pinned → priority → display_order). No per-resident financial data ever included.
- `GET /api/v1/organization-structure` — active members per position. Phone/email included only with `organization.contact.read`.

### 14.2 Write Endpoints (permission per operation)
- `POST /api/v1/community/upload` — multipart `file` + `category` (`activity|announcement|banner|general`). Category-scoped: `activities.create` or `announcements.create` required. Images are stored under `community/{rt_id}/{category}/` with content sniffing; never public-read.
- `POST /api/v1/activities` (`activities.create`), `PUT /api/v1/activities/{id}` (`activities.update`), `POST /api/v1/activities/{id}/publish` (`activities.publish`), `POST /api/v1/activities/{id}/cancel` (`activities.cancel`), `DELETE /api/v1/activities/{id}` (`activities.create`).
- `POST /api/v1/announcements` (`announcements.create`), `PUT /api/v1/announcements/{id}` (`announcements.update`), `POST /api/v1/announcements/{id}/publish` (`announcements.publish`), `POST /api/v1/announcements/{id}/archive` (`announcements.archive`), `DELETE /api/v1/announcements/{id}` (`announcements.create`).

Lifecycle rules:
- Drafts may be edited and hard-deleted.
- Published content MUST NOT be hard-deleted (`DELETE` returns 400). Published activities are cancelled; published/scheduled announcements are archived; archived is never auto-reactivated (explicit publish only).
- Publishing/republishing rewrites the announcement snapshot and the banner snapshot atomically in one transaction.
- Editing published content never mutates the live banner (banners are immutable snapshots taken at publish time).

Schedule rules (announcements): `scheduled_start` / `scheduled_end` must be RFC3339 with explicit timezone; invalid input returns 400; `scheduled_end` must be after `scheduled_start`. Display state is computed at query time — no scheduler/cron involved. Publishing with a future start stores state `scheduled` (never `active`); warga see content only inside the window.

### 14.3 Community Images
- `GET /api/v1/community/images/{key}` — two authorization modes:
  1. **Bearer** (`Authorization: Bearer <jwt>`): JWT validated, identity confirmed, caller's RT must be allowed+active; key must live under `community/{caller_rt_id}/`; key must be referenced by content the caller may see (draft content only for managers, published/active in-window content otherwise); unreferenced keys return 404.
  2. **Signed URL** (`?exp=<unix>&sig=<hmac>`): HMAC-SHA256 signature over `{key}.{exp}.{flags}` with the server signing secret; short-lived (15 minutes). Issued only after the same authorization as the Bearer mode. On read, the content reference is revalidated against the current state with the issuer's privilege class (`{flags}`), so an archived/cancelled/expired piece of content stops serving images immediately, even within the TTL. The signature is a bearer credential and cannot be individually revoked; the TTL plus read-time revalidation is the revocation mechanism.
- `POST /api/v1/community/images/sign` — authenticated endpoint; body `{"key": "community/<rt>/<category>/<file>"}`. Returns `{ "url": "/api/v1/community/images/...?exp=...&sig=...", "expires_at": <unix> }`. Applies the same content authorization as the Bearer mode. Used by `<img>` tags and Flutter.
- Keys are strictly validated (`community/<rt_id>/<category>/<file>.<jpg|png|webp>`); traversal, backslashes, NULs, and double encoding are rejected.
- There is no public listing of the community prefix and no public-read bucket.

### 14.4 Deep Links
Internal deep-link allowlist: `/keuangan`, `/iuran`, `/kas`, `/kegiatan`, `/pengumuman`, `/warga`, `/notifikasi`, `/profil` (prefix match with `/` boundary). External URLs are rejected with 400.

### 14.5 Pagination Contract
List endpoints accept `page` (default 1) and `page_size` (default 20, max 100) and return
`{ "data": [...], "pagination": { "page": n, "per_page": n, "total": n, "total_pages": n } }`.

---

## 15. Security Attendance API (`/api/v1/security` & `/api/v1/security-device`)

Time authority: all check-in/out timestamps, shift boundaries, lateness calculations, absent transitions, and QR expiration checks use server time in `Asia/Jakarta`. Tenant scoping (`rt_id`) is derived strictly from the authenticated user JWT or authenticated Pos device credential; client-supplied `rt_id` overrides are rejected/ignored.

### 15.1 Permissions (`permission_types` & `position_permissions`)
- `security.attendance` — Officer self check-in/out & personal swap/correction submissions (`keamanan`).
- `security.schedule.read` — Read shift configs, schedules, attendance, dashboard summary (`ketua`, `keamanan`).
- `security.schedule.write` — Create/update shifts, schedules, approve/reject swaps, manage officer list (`ketua`).
- `security.device.manage` — Register, rotate activation token, revoke Pos devices (`ketua`).
- `security.correction` — Approve/reject attendance corrections (`ketua`; maker-checker prevents self-approval).
- `security.guest_book` — Monitor guest book logs (`ketua`).
- `system_role = super_admin` bypasses RT RBAC (and specifies `?rt_id=` when managing a target RT).

### 15.2 Management & Officer Endpoints (`/api/v1/security/*`, JWT Bearer)
- **Dashboard & Officers**:
  - `GET /api/v1/security/dashboard` (`security.schedule.read`) — summary counts, active shifts, today's officers, pending swaps/corrections, active Pos devices, active guests count.
  - `GET /api/v1/security/officers` (`security.schedule.read`) — active officers with `jabatan = 'keamanan'` in caller's RT (plus monthly summary stats).
- **Shift Configs**:
  - `GET /api/v1/security/shifts` (`security.schedule.read`)
  - `POST /api/v1/security/shifts` (`security.schedule.write`)
  - `PUT /api/v1/security/shifts/{id}` (`security.schedule.write`) — bumps `version`; historical schedules/attendance keep original snapshots.
- **Schedules**:
  - `GET /api/v1/security/schedules` (`security.schedule.read`) — supports `date_from`, `date_to`, `user_id`, `shift_config_id`, `status`, pagination.
  - `POST /api/v1/security/schedules` (`security.schedule.write`) — accepts inclusive `start_date` and `end_date` (legacy `shift_date` remains supported); atomically expands into one daily schedule and attendance row per date, validating active officer/shift and shift-time overlap for every day.
  - `GET /api/v1/security/schedules/{id}` (`security.schedule.read`) — returns all daily schedule/attendance records for the assignment period.
  - `PUT /api/v1/security/schedules/{id}` (`security.schedule.write`) — edits the existing assignment; validates conflicts excluding its own daily rows, preserves past dates and rejects changes affecting dates with attendance/correction history. Writes an audit log only when changed.
  - `DELETE /api/v1/security/schedules/{id}` (`security.schedule.write`) — soft-cancels the selected daily schedule if no recorded attendance.
- **Shift Swaps (2-Step Workflow)**:
  - `GET /api/v1/security/swaps` (`security.schedule.read` / `security.attendance`)
  - `POST /api/v1/security/swaps` (`security.attendance`) — officer proposes swap between own `scheduled` shift and target officer's `scheduled` shift (same RT, not checked-in, no conflict after swap).
  - `POST /api/v1/security/swaps/{id}/target-respond` (`security.attendance`) — target officer accepts (`pending_ketua`) or rejects (`rejected_target`).
  - `POST /api/v1/security/swaps/{id}/decide` (`security.schedule.write`) — Ketua RT / Super Admin approves or rejects. On approve, swaps `(user_id, membership_id)` on both schedules & updates `belum_hadir` attendance rows atomically.
- **Attendance Check-in / Check-out & Monitoring**:
  - `GET /api/v1/security/attendance/me` (`security.attendance`) — officer's current/upcoming shift, today's attendance state, and history.
  - `POST /api/v1/security/attendance/scan` (`security.attendance`, rate-limited) — officer scans Pos dynamic QR `{ "qr_token": "...", "action": "check_in" | "check_out" | "auto" }`. Validates QR atomically (`SELECT ... FOR UPDATE`), device active/unrevoked, RT match, schedule match, check-in window, server-side `Asia/Jakarta` status calculation, anti-replay, and idempotency.
  - `GET /api/v1/security/attendance` (`security.schedule.read`) — monitoring list with filters (`date_from`, `date_to`, `user_id`, `shift_config_id`, `status`, pagination).
  - `GET /api/v1/security/attendance/export` (`security.schedule.read`) — CSV/JSON/Excel-friendly export summary scoped to RT.
- **Attendance Corrections (Maker-Checker & Audit Log)**:
  - `GET /api/v1/security/corrections` (`security.schedule.read` / `security.attendance`)
  - `POST /api/v1/security/corrections` (`security.attendance` or `security.correction`) — requires non-empty `reason`; officers may only request for their own record.
  - `POST /api/v1/security/corrections/{id}/decide` (`security.correction`) — approves or rejects. Rejects self-approval (`decided_by != requested_by` and `decided_by != officer_user_id`), preserves original values, updates attendance atomically when approved, writes immutable entry to `audit_logs`.
- **Pos Device Provisioning & Revocation**:
  - `GET /api/v1/security/devices` (`security.device.manage`)
  - `POST /api/v1/security/devices` (`security.device.manage`) — creates device and returns one-time `pairing_token` (shown once; stored hashed).
  - `POST /api/v1/security/devices/{id}/rotate-pairing` (`security.device.manage`) — issues fresh one-time `pairing_token`.
  - `POST /api/v1/security/devices/{id}/revoke` (`security.device.manage`) — immediately revokes device token and invalidates active QR tokens.
- **Guest Book Monitoring (Ketua RT)**:
  - `GET /api/v1/security/guest-books` (`security.guest_book`) — read/filter RT guest book entries (`date_from`, `date_to`, `status=active|checked_out`, `search`).

### 15.3 Security Pos Device Endpoints (`/api/v1/security-device/*`, Device Credential)
- **Activation (Unauthenticated, Rate-Limited)**:
  - `POST /api/v1/security/devices/activate` — exchanges one-time `pairing_token` for persistent `device_token` bound to `(device_id, rt_id)`.
- **Authenticated Pos Device Endpoints** (`Authorization: Device <device_token>` or `X-Device-Token: <device_token>`):
  - `GET /api/v1/security-device/me` — Pos device profile and bound RT metadata.
  - `POST /api/v1/security-device/qr` (rate-limited) — generates fresh short-lived (45s) single-use dynamic QR token for the Pos device.
  - `GET /api/v1/security-device/attendance/today` — today's Pos shift & attendance summary (names/statuses only, no PII).
  - `GET /api/v1/security-device/guest-books` — list RT guest book entries.
  - `POST /api/v1/security-device/guest-books` — register guest check-in (Pos Device only).
  - `POST /api/v1/security-device/guest-books/{id}/checkout` — record guest check-out (Pos Device only).
  - `GET /api/v1/security-device/warga/households` — read-only RT household directory (for Pos `Warga` menu). Returns `household_occupancy_id`, `house_number`, `block`, `head_of_family_name`, `occupancy_type`, `resident_count`, and `head_avatar_path` (opaque key resolved only through `security-device/images/*`).
  - `GET /api/v1/security-device/warga/households/{id}` — read-only RT household detail (no NIK/passwords/PII leaks). Resident rows carry `full_name`, `relationship`, `is_head_of_family`, `residency_status`, and `avatar_path` (opaque key resolved only through `security-device/images/*`).
  - `GET /api/v1/security-device/images/{key}` — read-only resident avatar bytes for the device's bound RT (opaque `community/{rt}/profile/*` keys only; the key must reference an active resident avatar in that RT; content serves with `no-store`). User credentials and cross-RT keys are rejected (403/404); image bytes are identical to what resident menus render.
  - `GET /api/v1/security-device/monitoring/sampah` — read-only uncollected waste monitoring list for device's RT.
  - `GET /api/v1/security/guest-books` — read-only Ketua RT Buku Tamu list; supports `search`, `status`, `page`, and `page_size`. RT scope comes from authenticated membership.
  - `GET /api/v1/security-device/guest-books` — paired-device Buku Tamu list; supports server-side `search`, `status`, `page`, and `page_size`, scoped to the bound device RT.
  - `GET /api/v1/rw/{rwId}/monitoring/read-only` — Ketua RT read-only monitoring list. Tenant RT is resolved from auth context; the path parameter is not an authorization scope. Notes mutation endpoints remain gated by `rw.monitoring.notes.write`.
