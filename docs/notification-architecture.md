# Social Finance — Notification Architecture Design

_Authored: 2026-09-27_

## Purpose

This document describes the **proposed notification architecture** for Social Finance.

It is a **design discussion** — no implementation has been made.

All items in this document are **PROPOSED / NOT IMPLEMENTED** unless explicitly noted otherwise.

---

## 1. Two Distinct Notification Mechanisms

Social Finance has two different notification mechanisms:

### A. Push Notification (FCM)

**Purpose:** Inform the user about an event.

**Delivery mechanism:** Firebase Cloud Messaging (FCM)

**Key characteristic:** Ephemeral delivery. If FCM fails, the notification is lost. This is acceptable for informational events.

### B. Persistent In-App Warning

**Purpose:** Represent an active business condition in the application UI.

**Source of truth:** Backend / database

**Key characteristic:** The warning is NOT a push notification. It is a persistent application UI state derived from authoritative backend state.

**It must NOT depend on whether an FCM notification was:**

- Delivered
- Opened
- Dismissed
- Deleted

---

## 2. CURRENT / EXISTING INFRASTRUCTURE

### 2.1 Notification System

**Status: NOT IMPLEMENTED**

The notification system was intentionally excluded from MVP:

> _"Notification System — Not required for MVP; deferred to v2+"_
> — `docs/requirements.md`, Section 3.2, item 3

No notification-related code, database tables, Firebase configuration, or FCM infrastructure exists in the repository.

### 2.2 Existing Authorization Model

**Status: IMPLEMENTED**

The system has three high-level system roles:

| system_role | Scope | Description |
|-------------|-------|-------------|
| `super_admin` | Platform | Platform administrator managing multiple RTs |
| _(null)_ | Single RT | Tenant-only users |

Tenant roles (stored in `user_rt_memberships.role`):

| tenant role | Scope | Description |
|-------------|-------|-------------|
| `pengurus` | Single RT | RT executive managing users, households, residents, admin tasks |
| `bendahara` | Single RT | RT treasurer managing finances |
| `warga` | Single RT | RT resident viewing own bills and payments |

Organizational positions (jabatan, **PLANNED / NOT IMPLEMENTED**):

| Position | Notes |
|----------|-------|
| Ketua | Chairman |
| Wakil Ketua | Vice Chairman |
| Sekretaris | Secretary |
| Bendahara | Treasurer |
| Seksi Keamanan | Security Section |
| Seksi Sosial | Social Section |
| Seksi Kebersihan/Pembangunan | Cleanliness/Development Section |
| Anggota | Member |

**IMPORTANT:** Notification targeting must operate on existing user/RT membership relationships. Do NOT create new notification-specific roles.

### 2.3 Tenant Isolation

**Status: IMPLEMENTED**

All tenant-scoped data access is derived from the authenticated user's JWT `rt_id` claim. The `rt_id` is **never** trusted from client input. Every repository query on tenant-scoped data includes `WHERE rt_id = ?`.

This isolation model applies to notification targeting as well.

---

## 3. Notification Events

### 3.5 Waste Truck Arrival

**Status: PLANNED / NOT IMPLEMENTED**

When the waste collection truck arrives at the RT gate, Security or a Pengurus can trigger a notification to all warga of that RT.

**Purpose:** Inform warga that the garbage truck has arrived so they can prepare and place their waste for collection.

**Use case flow:**

```
Security/Pengurus observes truck at RT gate
    ↓
Security/Pengurus opens app → Waste Truck Notification
    ↓
Selects RT (already derived from auth context, pre-filled)
    ↓
Enters message (optional, with sensible default)
    ↓
Sends notification
    ↓
All warga of that RT receive the notification
```

**Authorization:**

* Only users with role `pengurus` in `user_rt_memberships` or with `jabatan` in `seksi_keamanan` or `ketua`, `wakil_ketua`, `anggota` may send this notification.
* The RT is derived from the authenticated user's JWT `rt_id` claim — never from client input.
* A Security/Pengurus of RT 01 can only notify warga of RT 01.
* WARGA cannot send this notification.

**Target:** All warga of the sending user's RT.

**Example notification messages:**

* Subject: `Mobil Pengangkut Sampah Sudah Tiba`
* Body: `Mobil pengangkut sampah sudah sampai di depan gerbang RT. Silakan keluarkan sampah.`

The message text is configurable at send time but should provide sensible defaults in Indonesian.

**Data model:**

* No new database tables are required for the initial implementation.
* If notification history/audit is later needed, a `notifications` or `waste_truck_notifications` table may store:
  * `id` (UUID PK)
  * `rt_id` (tenant scope)
  * `sent_by_user_id` (who triggered it)
  * `message` (the notification text)
  * `notified_at` (TIMESTAMPTZ — when the notification was sent)
* These fields are for future auditability only. The initial version does not create any tables.

**What this does NOT include:**

* No GPS, geofencing, or background location tracking.
* No live vehicle tracking or GPS-based auto-trigger.
* No separate user model for garbage truck drivers or sanitation staff.
* No change to the Warga or Household data models.

**Trigger method:** Manual only. Security or Pengurus presses a button when they see the truck arrive.

**Runtime behavior (future):**

```
Notification triggered (manual)
    ↓
Backend validates sender has permission in this RT
    ↓
Backend identifies all warga of this RT
    ↓
Backend creates notification record (if persistence table exists)
    ↓
FCM push to all warga devices in this RT
    ↓
Persistent in-app notification (if persistent mechanism exists)
```

**Security rules:**

1. Notification targeting must be performed server-side.
2. Sender RT is derived from JWT `rt_id` — client-provided RT is ignored.
3. Only warga of the sender's RT receive the notification.
4. No cross-RT notification is possible.

---

### 3.1 New Income

**Status: NOT IMPLEMENTED**

When a new finance income transaction is created:

```
Income transaction
        ↓
Notification event
        ↓
Affected RT (derived from transaction.rt_id)
        ↓
All residents + pengurus in that RT
        ↓
FCM push notification
```

**Target:** All users belonging to the affected RT.

**The notification must NOT be broadcast system-wide.**

**Proposed future payload (example only):**

```json
{
  "type": "finance_income",
  "transaction_id": "<transaction-id>",
  "rt_id": "<rt-id>"
}
```

### 3.2 New Expense

**Status: NOT IMPLEMENTED**

When a new finance expense transaction is created:

```
Expense transaction
        ↓
Notification event
        ↓
Affected RT (derived from transaction.rt_id)
        ↓
All residents + pengurus in that RT
        ↓
FCM push notification
```

**Target:** All users belonging to the affected RT.

**Proposed future payload (example only):**

```json
{
  "type": "finance_expense",
  "transaction_id": "<transaction-id>",
  "rt_id": "<rt-id>"
}
```

### 3.3 Overdue — 2 Months

**Status: NOT IMPLEMENTED**

When an individual resident reaches 2 months of overdue dues:

```
Overdue >= 2 months
        ↓
Affected resident only
        ↓
FCM push notification
```

**Target:** The affected resident only.

**Do NOT send this warning to:**

- Other residents
- Other RTs
- All pengurus
- All system users

**The backend must determine the authoritative overdue state.** The client must not independently calculate the authoritative overdue status.

### 3.4 Overdue — 3 Months or More

**Status: NOT IMPLEMENTED**

When the overdue period reaches 3 months or more:

```
Overdue >= 3 months
        │
        ├───────────────┐
        ▼               ▼
   FCM Push       Persistent
                  In-App Warning
```

**FCM push target:** Affected resident only.

**Persistent warning target:** The affected resident's application.

**Proposed future response contract (example only):**

```json
{
  "waste_collection_suspended": true,
  "overdue_months": 3
}
```

---

## 4. Persistent Red Warning

**Status: NOT IMPLEMENTED**

The red warning is NOT a permanent push notification. It is an application UI state.

### 4.1 Visual Concept

```
┌──────────────────────────────────────────────┐
│ 🚨 PERINGATAN                                │
│                                              │
│ Iuran Anda telah menunggak 3 bulan.          │
│ Pengangkutan sampah sementara dihentikan.    │
│                                              │
│             [Lihat Tunggakan]                │
└──────────────────────────────────────────────┘
```

### 4.2 Behavior

```
overdue >= 3 months
        ↓
warning becomes active
        ↓
backend reports active state
        ↓
UI displays red warning
        ↓
user may navigate away / close app
        ↓
warning remains active
        ↓
backend determines overdue condition resolved
        ↓
warning becomes inactive
        ↓
UI no longer displays warning
```

### 4.3 Critical Rules

- Dismissing an FCM notification **must NOT** remove the warning.
- Opening a notification **must NOT** remove the warning.
- Restarting the application **must NOT** remove the warning.
- Logging out/in **must not** bypass the backend condition.
- The **backend remains authoritative**.

The persistent warning is derived from authoritative backend state (bill payment records, dues configuration, and arrears calculation).

---

## 5. Notification Targeting

### 5.1 Targeting Matrix

| Event | Sender | Target |
|-------|--------|--------|
| Waste truck arrival | Security / Pengurus | All warga of same RT |
| New income | System (backend) | All residents + pengurus in affected RT |
| New expense | System (backend) | All residents + pengurus in affected RT |
| 2-month overdue | System (backend) | Affected resident only |
| 3+ month overdue push | System (backend) | Affected resident only |
| 3+ month overdue banner | System (backend) | Affected resident's application |

### 5.2 Tenant Isolation

**Notification targeting must be strictly RT-scoped.**

A notification related to RT 03 must never be sent to users belonging only to RT 04, RT 05, RT 06, or RT 002/Urena.

- Do **not** hardcode RT IDs.
- Use the authenticated/authoritative RT relationship from the user's JWT claims.

### 5.3 Pengurus and Warga Targeting

**"Pengurus" targeting must be based on the existing membership/role model.**

Do not infer pengurus from display names or hardcoded position strings in frontend code. The backend should determine authoritative membership and targeting via `user_rt_memberships`.

Respect the existing distinction between:

- `system_role` (global: `super_admin` or null)
- RT membership role (tenant: `pengurus`, `bendahara`, `warga`)
- `jabatan` (organizational position, **PLANNED**)

---

## 6. Device / FCM Architecture

**Status: NOT IMPLEMENTED**

### 6.1 Proposed Device Registration

A future system may contain a `user_devices` table to map users to their FCM tokens.

**PROPOSED / NOT IMPLEMENTED — Schema is illustrative only:**

```
user_devices
├── id              UUID (PK)
├── user_id         UUID (FK → users.id)
├── fcm_token       TEXT
├── platform        TEXT (e.g., 'android', 'web')
├── device_id       TEXT (nullable)
├── is_active       BOOLEAN (default true)
├── last_seen_at    TIMESTAMPTZ
├── created_at      TIMESTAMPTZ
└── updated_at      TIMESTAMPTZ
```

**This schema is NOT final.** It is a conceptual proposal for discussion only. Final column names, constraints, and indexes will be designed during implementation.

### 6.2 FCM Token Lifecycle

Future implementation must handle:

- Token registration on app install / first launch
- Token refresh (FCM tokens can change)
- Token invalidation on logout
- Token cleanup on account deletion

---

## 7. Notification Persistence

**Status: NOT IMPLEMENTED**

FCM delivery alone is insufficient for persistent application state.

```
Notification delivery  ≠  Business condition/state

FCM
=
delivery mechanism

Overdue / waste suspension status
=
backend business state
```

**The persistent warning must be derived from authoritative backend state.**

If a notification history table is eventually required (for auditability or user review), it should be designed separately from the overdue business state.

---

## 8. Duplicate Prevention

**Status: NOT IMPLEMENTED**

The system must avoid generating duplicate recurring overdue notifications every day.

If a resident remains at **3 months overdue**, the system must not blindly send the same "3 months overdue" push every day.

The future scheduler/notification service must define deduplication rules.

**Potential design concepts (not implemented):**

```
event type
recipient
business entity
threshold
notification period
last sent
```

These are design considerations only. The exact deduplication strategy will be defined during implementation.

---

## 9. Scheduler

**Status: NOT IMPLEMENTED**

A future backend scheduler/job is required for overdue evaluation.

**Conceptual flow:**

```
Scheduled job
      ↓
Find residents with overdue dues
      ↓
Calculate authoritative overdue state
      ↓
Detect threshold transitions
      ↓
Generate required notification
      ↓
Update persistent business state
```

**The scheduler should detect meaningful state transitions** rather than blindly sending notifications on every execution.

---

## 10. Threshold Transitions

**Status: NOT IMPLEMENTED**

### 10.1 Overdue Progression

```
< 2 months
    ↓
2 months
    ↓
3+ months
```

### 10.2 At Each Threshold

| Threshold | Action |
|-----------|--------|
| < 2 months | No notification |
| 2 months | Push warning (FCM to affected resident only) |
| 3+ months | Push warning (FCM to affected resident only) + persistent in-app warning |

### 10.3 Overdue Resolved

When the overdue condition is resolved (all bills paid):

- Persistent warning becomes inactive.
- No new push notification is needed.
- Existing UI warning is removed on next state sync.

**The exact financial calculation and grace-period rules must be defined by the backend finance domain before implementation.** Do not invent financial rules not already present in the repository.

---

## 11. API Design

**Status: NOT IMPLEMENTED**

The backend may eventually expose an authenticated status endpoint for the current user's active warnings.

**Proposed (not final):**

```
GET /api/v1/notifications/status
```

**Proposed response (example only):**

```json
{
  "waste_collection_suspended": true,
  "overdue_months": 3
}
```

**The API should expose authoritative application state.** The future Web and Flutter clients should consume backend state rather than duplicate business calculations.

The exact endpoint path, response shape, and authorization model are **NOT FINAL** and will be designed during implementation.

---

## 12. Deep Linking

**Status: NOT IMPLEMENTED**

Finance push notifications should eventually support navigation to the relevant finance detail.

**Conceptual flow:**

```
FCM notification
      ↓
notification payload
      ↓
transaction_id
      ↓
application navigation
      ↓
finance detail
```

Deep links must re-check authorization when opening the target resource.

---

## 13. Failure Handling

**Status: NOT IMPLEMENTED**

The future system must handle:

- Invalid or expired FCM token
- Multiple devices per user
- Revoked device token
- Disabled device
- Notification delivery failure
- User logged out
- User changes device
- Duplicate device registration

**Critical rule:** FCM delivery failure must **not** alter authoritative business state.

```
FCM failed
    ≠
overdue condition resolved
```

---

## 14. Security / Tenant Isolation

**Status: DESIGN RULES (not implemented)**

1. **Notification targeting must be performed server-side.** The client must not self-identify as a notification recipient.
2. **Client-provided RT IDs must not be trusted for authorization.** Authenticated tenant context from JWT remains authoritative.
3. **Users must not be able to subscribe to another RT's notifications** by modifying request parameters.
4. **Finance notification payloads must not expose unnecessary sensitive data.** Transaction IDs are sufficient — do not include amounts, descriptions, or PII in push payloads.
5. **Notification deep links must re-check authorization** when opening the target resource.

---

## 15. Implementation Phases

**Status: DOCUMENT ONLY — no implementation performed**

| Phase | Title | Status |
|-------|-------|--------|
| Phase 1 | Notification domain/design | **CURRENT TASK: DOCUMENT ONLY** |
| Phase 2 | Device registration + FCM token management | NOT IMPLEMENTED |
| Phase 3 | Backend notification service | NOT IMPLEMENTED |
| Phase 4 | Finance income/expense notifications | NOT IMPLEMENTED |
| Phase 5 | Waste Truck Arrival notification (manual trigger) | NOT IMPLEMENTED |
| Phase 6 | Overdue detection + scheduler | NOT IMPLEMENTED |
| Phase 7 | 2-month overdue push | NOT IMPLEMENTED |
| Phase 8 | 3+ month overdue push + persistent warning | NOT IMPLEMENTED |
| Phase 9 | Flutter/Web notification UI + deep links | NOT IMPLEMENTED |

---

## 16. Testing Plan (Future)

**Status: NOT WRITTEN — to be created during implementation**

### 16.1 Unit Tests

- RT targeting logic
- User targeting logic
- Role/membership targeting logic
- Overdue threshold transitions
- Duplicate prevention logic
- State activation/deactivation

### 16.2 Integration Tests

- Income event → correct RT recipients
- Expense event → correct RT recipients
- 2-month overdue → affected resident only
- 3+ month overdue → affected resident only
- Overdue resolved → warning becomes inactive

### 16.3 Security Tests

- RT03 user cannot receive/inspect RT04 private notification state
- Client cannot override RT targeting
- Notification deep links enforce authorization

### 16.4 Device Tests

- Multiple devices per user
- Invalid FCM token handling
- Logged-out device cleanup
- Token refresh flow

---

## 17. Design Principles Summary

1. **Two mechanisms, separate concerns.** Push notification = informational. In-app warning = business state.
2. **Backend is authoritative.** The persistent warning must always derive from backend state.
3. **RT tenant isolation is mandatory.** Every notification target is derived from the user's authenticated RT membership.
4. **No new authorization roles.** Notification targeting uses existing `pengurus`, `bendahara`, `warga` roles and `user_rt_memberships`.
5. **Deduplication is required.** The system must not send redundant overdue notifications.
6. **FCM failure is acceptable.** Loss of a push notification does not change business state.
7. **Finance payloads are minimal.** Do not include sensitive financial data in push notifications.

---

## References

| Document | Relevance |
|----------|-----------|
| `docs/architecture.md` | System architecture overview |
| `docs/database.md` | Financial schema (bills, payments, dues) |
| `docs/security.md` | Auth, RBAC, tenant isolation |
| `docs/authorization-design.md` | System roles, tenant roles, jabatan |
| `docs/business-rules.md` | Dues, billing, arrears rules |
| `docs/requirements.md` | Notification deferred to v2+ |
| `docs/development-plan.md` | Phase ordering and dependencies |
| `AGENTS.md` | AI coding assistant instructions |
