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

### 3.6 New Iuran Bill

**Status: PLANNED / NOT IMPLEMENTED**

When a new bill is successfully created or generated for a resident/household:

```
New bill created/generated (backend persisted)
        ↓
Backend derives RT and eligible resident(s) for the household
        ↓
Notification event queued / dispatched
        ↓
FCM push notification to affected resident(s) in that RT
```

**Trigger:** A new bill is successfully created (single creation) or generated (batch generation) and committed in the database.

**Target:** The affected resident(s) who are entitled to receive that bill, strictly within the correct RT (derived from the bill's household occupancy in that RT).

**Requirements & Constraints:**
- **Post-Persistence Dispatch:** Send a notification only after the bill record has been successfully persisted in the database.
- **Backend-Authoritative Data:** Use backend-authoritative bill, household, and recipient relationships. Never accept client-supplied recipient lists.
- **Tenant & Household Isolation:** Avoid notifying unrelated residents in the same RT or any residents in other RTs.
- **Deep Linking:** Provide a deep link directing to the relevant bill or Iuran detail screen (`/iuran/:id`), subject to authorization checks upon opening.
- **Minimal Payload:** Do not expose unnecessary financial amounts, breakdown, or personal information in the FCM payload. Use entity identifiers (`bill_id`, `rt_id`, `type: "iuran_bill_created"`).

**Proposed future payload (example only):**

```json
{
  "type": "iuran_bill_created",
  "bill_id": "<bill-id>",
  "rt_id": "<rt-id>"
}
```

---

### 3.7 Iuran Payment Verification Result (Highest Priority)

**Status: PLANNED / NOT IMPLEMENTED**

When a payment submitted by a resident has its verification finalized as either accepted (`APPROVED`) or rejected (`REJECTED`) by verification staff:

```
Payment verified (APPROVED or REJECTED persisted in DB)
        ↓
Backend retrieves payment owner (warga submitter)
        ↓
Notification event dispatched
        ↓
FCM push notification strictly to payment owner
```

**Trigger:** A payment transition out of `PENDING` is finalized and persisted as accepted (`APPROVED`) or rejected (`REJECTED`).

**Target:** **Only the owner** of the payment being verified (the specific citizen/resident who submitted the payment).

**Requirements & Constraints:**
- **Both Outcomes Notified:** Send a notification for both accepted and rejected verification results.
- **Backend Truth:** Ensure the notification reflects the actual persisted backend result.
- **Privacy of Rejection Explanation:** For rejected payments, include a rejection explanation only if the existing backend contract and privacy rules permit it (e.g., standard citizen-facing `rejection_reason`). Internal officer notes (`officer_notes`) must **never** be exposed in notifications or citizen-facing payloads.
- **No Broadcast:** Do **not** broadcast the verification outcome to other residents, all Pengurus, or unrelated users.
- **Deduplication Identity:** Prevent repeated notifications when the same verification operation is retried or processed more than once (deduplicated by `payment_id` + finalized status/transition).
- **Authoritative Application State:** Keep authoritative payment status, receipt details, and any permitted verification explanations available through the application/backend API, rather than relying solely on the push payload.
- **Deep Linking:** Link to the relevant payment or bill detail screen (`/iuran/:bill_id`), with authorization validated upon navigation.

**Proposed future payload (example only):**

```json
{
  "type": "iuran_payment_verified",
  "payment_id": "<payment-id>",
  "bill_id": "<bill-id>",
  "status": "APPROVED",
  "rt_id": "<rt-id>"
}
```

---

### 3.8 New Iuran Payment Submitted (Highest Priority)

**Status: PLANNED / NOT IMPLEMENTED**

When a resident successfully submits an Iuran payment for verification:

```
Resident submits payment (persisted in DB with status PENDING)
        ↓
Backend derives RT and eligible verification staff (Bendahara / authorized staff)
        ↓
Notification event queued / dispatched
        ↓
FCM push notification to authorized verification staff of that RT
```

**Trigger:** A resident successfully submits a payment and it is persisted in the database with status `PENDING`.

**Target:** `bendahara` and other staff who actually have permission to verify payments for the affected RT.

**Requirements & Constraints:**
- **Targeting Verification Personnel:** Target only authorized verification personnel in the payment's RT. Do **not** automatically broadcast to all Pengurus unless the existing authorization model and an explicit operational requirement justify it.
- **Trusted Backend Context:** Derive the RT and recipient eligibility strictly from trusted backend database records and authorization rules (`user_rt_memberships`). Never trust a client-supplied RT identifier or user list for notification targeting.
- **Post-Persistence Dispatch:** Do not notify verification staff before the payment submission is successfully persisted.
- **Deduplication Identity:** Prevent duplicate notifications for retries, network retransmissions, or repeated processing of the same payment submission (deduplicated by `payment_id` + submission lifecycle state).
- **Deep Linking:** Link to the relevant payment verification screen / Iuran detail verification modal, with strict role/permission checks re-evaluated when the link is opened.
- **Permission Mapping Confirmation:** The exact mapping of verification staff is based on existing role `bendahara` (and authorized `pengurus` with payment verification capability if configured). During implementation, confirm the exact permission check mapping against the authoritative RBAC matrix rather than inventing new roles.

**Proposed future payload (example only):**

```json
{
  "type": "iuran_payment_submitted",
  "payment_id": "<payment-id>",
  "bill_id": "<bill-id>",
  "rt_id": "<rt-id>"
}
```

---

### 3.9 Iuran Due-Date and Arrears Reminder

**Status: PLANNED / NOT IMPLEMENTED**

When an active bill reaches an applicable reminder point or an existing arrears rule is satisfied:

```
Backend scheduler/evaluation job runs
        ↓
Backend computes due dates, overdue durations, and reminder thresholds
        ↓
Identifies bills eligible for reminder at current stage
        ↓
Deduplication check (has this stage been notified for this bill?)
        ↓
FCM push notification to responsible resident(s) of that household
```

**Trigger:** A bill reaches an applicable reminder point (e.g., approaching due date) or satisfies an arrears rule (e.g., past due date).

**Target:** The resident(s) responsible for the relevant bill, strictly within the correct RT.

**Requirements & Constraints:**
- **Backend-Authoritative Evaluation:** The backend must determine due dates, overdue duration, and reminder eligibility according to established financial domain rules. Neither the Flutter app nor the Web frontend may independently calculate reminder eligibility or decide that a bill is overdue for notification purposes.
- **Reuse Authoritative Financial Logic:** Financial status is derived from bills and approved payments in the database; notification jobs must reuse these canonical calculations.
- **Deduplication by Stage:** Prevent duplicate reminders for the same bill and reminder stage (deduplicated by `bill_id` + reminder stage/period).
- **Respect Overdue Policy:** Adhere to the established overdue policy distinguishing ephemeral push notifications from persistent in-app warnings (Section 1 and Section 4).
- **Confirmation of Intervals:** Specific reminder intervals (e.g., N days before due date, post-due grace periods, or milestone thresholds) must be confirmed with the RT dues business specifications rather than invented.

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

| Event | Sender / Trigger | Target | RT Isolation Scope | Delivery Channel | Deep Link Destination | Deduplication Identity | Scheduling Required | Authoritative Backend State |
|---|---|---|---|---|---|---|---|---|
| **New Iuran bill** | System (backend) / Bill created or batch generated | Resident(s) of affected household | Affected RT only (derived from bill household occupancy in RT) | FCM Push | `/iuran/:bill_id` | `bill_id` | No (event-driven on persistence) | `bills` table (persisted bill record) |
| **Iuran payment verification result (Highest Priority)** | System (backend) / Verification finalized (`APPROVED` or `REJECTED`) | Payment submitter / owner only | Affected RT only (payment submitter's RT) | FCM Push | `/iuran/:bill_id` | `payment_id` + finalized status (`APPROVED`/`REJECTED`) | No (event-driven on verification completion) | `payments.status`, `payments.rejection_reason` |
| **New Iuran payment submitted (Highest Priority)** | System (backend) / Resident submits payment | `bendahara` and authorized verification staff in affected RT only (no broad broadcast) | Affected RT only (derived from payment's RT) | FCM Push | `/iuran/:bill_id` (verification view/modal) | `payment_id` + submission lifecycle state (`PENDING`) | No (event-driven on payment persistence) | `payments` table (`status = 'PENDING'`) |
| **Iuran due-date & arrears reminder** | System (backend scheduler) / Due date approaching or arrears rule triggered | Resident(s) responsible for the bill | Affected RT only (derived from bill/household) | FCM Push | `/iuran/:bill_id` | `bill_id` + reminder stage/period | Yes (scheduled recurring evaluation job) | `bills` + approved `payments` calculation |
| **Waste truck arrival** | Security / Pengurus (manual trigger) | All warga of same RT | Sender's RT only (derived from JWT `rt_id`) | FCM Push | N/A (informational banner / home screen) | Manual trigger cooldown / timestamp token | No (manual trigger) | `user_rt_memberships` (RT warga list) |
| **New income** | System (backend) / Income transaction created | All residents + pengurus in affected RT | Affected RT only (transaction's `rt_id`) | FCM Push | `/finance/transactions/:id` | `transaction_id` | No (event-driven) | `transactions` table (`type = 'income'`) |
| **New expense** | System (backend) / Expense transaction created | All residents + pengurus in affected RT | Affected RT only (transaction's `rt_id`) | FCM Push | `/finance/transactions/:id` | `transaction_id` | No (event-driven) | `transactions` table (`type = 'expense'`) |
| **2-month overdue** | System (backend scheduler) / Arrears reaches 2 months | Affected resident only | Affected RT only (resident's RT) | FCM Push | `/iuran` (arrears overview) | `household_id`/`resident_id` + `threshold:2m` + period | Yes (scheduled recurring evaluation job) | Backend ledger arrears computation (`bills` - `payments`) |
| **3+ month overdue push** | System (backend scheduler) / Arrears reaches 3+ months | Affected resident only | Affected RT only (resident's RT) | FCM Push | `/iuran` (arrears overview) | `household_id`/`resident_id` + `threshold:3m` + period | Yes (scheduled recurring evaluation job) | Backend ledger arrears computation (`bills` - `payments`) |
| **3+ month overdue banner** | System (backend API query) / Active business condition | Affected resident's application UI | Affected RT only (resident's RT) | In-App UI State (Persistent Warning) | In-app prompt to `/iuran` | N/A (continuous state evaluated on API sync) | No (real-time query / sync via status API) | Backend ledger arrears computation (`bills` - `payments`) |

### 5.2 Tenant Isolation

**Notification targeting must be strictly RT-scoped.**

A notification related to RT 03 must never be sent to users belonging only to RT 04, RT 05, RT 06, or RT 002/Urena.

- Do **not** hardcode RT IDs.
- Use the authenticated/authoritative RT relationship from the user's JWT claims.

### 5.3 Pengurus and Warga Targeting

**Role-based targeting must be based on the authoritative backend membership/role model.**

Do not infer pengurus, bendahara, or warga from display names, freeform strings, or client-provided position strings in frontend code. The backend must determine authoritative membership and targeting via `user_rt_memberships`.

Respect the existing distinction between:

- `system_role` (global: `super_admin` or null)
- RT membership role (tenant: `pengurus`, `bendahara`, `warga`)
- `jabatan` (organizational position, **PLANNED**)

#### Verification Staff Targeting vs. Broad Broadcast

For **New Iuran Payment Submitted** notifications:
- Target **only `bendahara` and authorized verification personnel** in the payment's RT.
- Do **not** broadcast payment submissions indiscriminately to all Pengurus or non-finance staff. Verification is a specific financial responsibility.
- During future implementation, the exact mapping of verification staff should be confirmed against the authoritative RBAC matrix (e.g. users holding tenant role `bendahara` or role `pengurus` with explicit `finance.approve` / verification capability) rather than inventing new ad-hoc roles.

#### Payment Owner Targeting

For **Iuran Payment Verification Result** notifications:
- The notification must be sent **strictly to the resident who submitted the payment**.
- Do not broadcast verification outcomes to other household members, other residents, or all Pengurus.

#### Privacy Boundaries: Officer Notes vs. Citizen Explanations

In payment verification workflows:
- **Citizen-facing explanation (`rejection_reason`):** For rejected payments, a brief, user-facing explanation may be communicated to the resident in accordance with existing backend contracts and privacy rules.
- **Internal audit notes (`officer_notes`):** Internal verification remarks, reviewer comments, and administrative notes must **never** be exposed in FCM push payloads, notification logs delivered to clients, or citizen-facing endpoints. Officer notes are strictly confidential to authorized management staff.

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

The notification system must guarantee that duplicate notifications are not emitted across retries, network retransmissions, concurrent worker executions, or recurring scheduler runs.

### 8.1 Deduplication Identities by Event Type

To prevent redundant alerts while maintaining reliable delivery, each notification event defines a deterministic deduplication identity:

| Event Type | Deduplication Identity | Mechanism & Rules |
|---|---|---|
| **New Iuran bill** | `bill_id` | Idempotent on bill record. A bill is created once; retries of bill creation or batch dispatch ignore already-notified `bill_id`s. |
| **Iuran payment verification result** | `payment_id` + finalized status (`APPROVED` / `REJECTED`) | Deduplicated on the terminal status transition. If verification is retried due to network drops, only one push notification is emitted for that final outcome. |
| **New Iuran payment submitted** | `payment_id` + lifecycle state (`PENDING`) | Deduplicated on initial payment submission. Network retransmissions or idempotency retries from the client do not produce multiple notifications to verification staff. |
| **Iuran due-date & arrears reminder** | `bill_id` + reminder stage / period (e.g., `bill_123:due_minus_3d` or `bill_123:overdue_2m_2026_10`) | Deduplicated by milestone stage and evaluation window. The scheduler evaluates whether a notification has already been sent for that specific stage, preventing daily spam while a resident remains in the same overdue status. |
| **Overdue dues (General)** | `household_id` + threshold (`2m` / `3m`) + billing period | Scheduler checks whether the current threshold transition was already notified for that period before dispatching. |

### 8.2 Idempotency, Retries, and Concurrency Safety

1. **Idempotency Keys on Dispatch:**
   The backend notification dispatcher should record emitted notification identities (e.g., in a deduplication cache or a persistent `dispatched_notifications` ledger) before or atomically with FCM dispatch. If a job crashes or retries, duplicate notifications for the same identity are discarded.
2. **State Transition Triggers:**
   Notifications are driven by authoritative state transitions (e.g., status changing from `PENDING` to `APPROVED`), rather than ambient query loops without transition guards.
3. **Concurrent Execution Safety:**
   Scheduled evaluation jobs and webhook/API handlers must use database-level advisory locks or row locking when processing transitions to prevent race conditions from generating duplicate events.

### 8.3 Financial State and Ledger Immutability Guarantees

**Critical Financial Integrity Rules:**
- **Decoupled Failure Domains:** Notification delivery failures, FCM server timeouts, or notification retries must **never** roll back, alter, or corrupt an already committed financial transaction, bill status, or payment verification.
- **Financial Immutability Uncompromised:** A payment verification is finalized as `APPROVED` or `REJECTED` in the database regardless of whether the push notification succeeds. The financial ledger remains the immutable source of truth.
- **No Side-Effect Retries on Business Data:** Retrying a notification must only retry the push delivery pipeline; it must never re-execute the payment submission or verification business logic.

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

Push notifications should support seamless in-app navigation directly to the relevant resource upon user tap, while enforcing strict security and tenant isolation boundaries.

### 12.1 Deep Link Destinations by Notification Type

| Event Type | Target Role | Deep Link Destination | Action / Destination View |
|---|---|---|---|
| **New Iuran Bill** | Warga (Resident) | `/iuran/:bill_id` | Opens bill detail screen showing payment options and bill breakdown. |
| **Iuran Payment Verification Result** | Warga (Submitter) | `/iuran/:bill_id` | Opens bill/payment detail view showing approved receipt or rejection reason. |
| **New Iuran Payment Submitted** | Bendahara / Verification Staff | `/iuran/:bill_id` (or `/iuran/:bill_id/verify`) | Opens payment verification modal with proof viewer, approve button, and reject input. |
| **Iuran Due-Date & Reminder** | Warga (Resident) | `/iuran/:bill_id` | Opens bill payment screen with urgency indicator. |
| **Finance Income / Expense** | Warga / Pengurus | `/finance/transactions/:id` | Opens transaction detail screen. |
| **Overdue Arrears Warning** | Warga (Resident) | `/iuran` | Opens Iuran overview listing all unpaid bills. |

### 12.2 Deep Link Execution Flow

```
User taps FCM notification on device
          ↓
OS launches application with deep link URL (e.g. /iuran/550e8400-e29b-41d4-a716-446655440000)
          ↓
App verifies active user session (if unauthenticated, routes to Login, preserves destination)
          ↓
App requests resource from backend API with user's JWT
          ↓
Backend re-evaluates RBAC and Tenant Isolation (WHERE rt_id = jwt.rt_id)
          ↓
Authorized: App navigates to target screen
Unauthorized / Cross-RT: App displays 403 Forbidden / 404 Not Found error
```

### 12.3 Mandatory Security Requirements for Deep Links

1. **Mandatory Authorization Re-Check:** Deep linking must **never** bypass backend authentication, RBAC, or tenant checks. Tapping a deep link must re-evaluate user credentials against the resource.
   - If a `warga` attempts to open a verification link (`/iuran/:id/verify`), the backend must return `403 Forbidden`.
   - If a user from RT 02 attempts to open a bill belonging to RT 01, the backend query filters by `rt_id` and returns `404 Not Found`.
2. **No Sensitive Data in URLs:** Deep link URLs must contain **only opaque identifiers** (UUIDs). Never include tokens, passwords, payment amounts, bank details, citizen PII, or internal notes in deep link parameters.

---

## 13. Failure Handling

**Status: NOT IMPLEMENTED**

Notification delivery operates across external network boundaries and third-party push gateways (Firebase Cloud Messaging, APNs). The notification architecture must maintain robustness without violating financial integrity.

### 13.1 Handling Transport Failures

The future system must gracefully handle:
- **Invalid or Expired FCM Tokens:** Token unregistered with FCM; system flags token as inactive in `user_devices`.
- **Multiple Devices per User:** Notification dispatched to all active tokens of the recipient; failed tokens pruned individually.
- **Revoked or Disabled Device Tokens:** Automatically deactivated upon receiving `UNREGISTERED` or `INVALID_ARGUMENT` from FCM.
- **Offline Devices or Network Outages:** FCM handles standard TTL buffering; backend does not block waiting for device delivery.
- **Logged-Out Users:** FCM token invalidated on logout to prevent subsequent account holders on a shared device from receiving private alerts.

### 13.2 Decoupling Financial State from Delivery Status

**Critical Rule:** Notification delivery failures must **never** alter authoritative business or financial state.

```
FCM delivery failed / device offline
                ≠
Payment verification cancelled or rolled back
```

- If an FCM push notification fails to reach the user, an approved payment remains **APPROVED**, an unpaid bill remains **UNPAID**, and ledger transactions remain immutable.
- **FCM Delivery $\neq$ User Acknowledged:** A successful push response from FCM servers indicates only that the push payload was accepted by the gateway. It does **not** guarantee that the user has seen, read, or acted upon the notification. Authoritative business actions must never rely on delivery receipts.

### 13.3 Idempotent Retries

- If the backend notification dispatcher fails due to a transient internal error or network drop to the FCM gateway, retries must be managed via an exponential backoff policy.
- Retries must be strictly idempotent using the deduplication identity (Section 8) to avoid sending duplicate alerts to the device.
- Notification retries must **never re-execute** financial business logic (e.g., must never re-trigger payment verification or bill creation).

---

## 14. Security / Tenant Isolation

**Status: DESIGN RULES (not implemented)**

1. **Server-Side Targeting Only:** Notification recipient lists must be resolved strictly on the backend. Clients must never supply recipient IDs or self-identify as notification targets.
2. **Client-Provided RT IDs Never Trusted:** The RT context is derived strictly from the authenticated user's JWT claims (`rt_id`) or authoritative database relationships. Request payloads cannot override the RT scope.
3. **Strict Cross-RT Isolation:** Users cannot subscribe to, listen for, or query notification channels or device tokens belonging to another RT.
4. **Minimal Push Payloads:**
   - Push notifications traverse external third-party infrastructure and device notification trays that may be visible on lock screens.
   - Payloads must contain **minimal opaque identifiers only** (`type`, `bill_id`, `payment_id`, `rt_id`).
   - Push payloads must **never** contain monetary amounts, payment breakdowns, full resident names, NIK, or bank account details.
5. **Privacy of Rejection Explanations vs. Officer Notes:**
   - Citizen-facing rejection explanations (`rejection_reason`) are shared with the payment submitter in compliance with standard API contracts.
   - Internal administrative remarks (`officer_notes`) must **never** be included in push notifications, citizen notification lists, or citizen API responses.
6. **Authorization Re-Evaluation on Deep Links:** Any screen navigated to via a push notification must re-authenticate the user and re-validate permissions via backend endpoints before rendering sensitive data.

---

## 15. Implementation Phases

**Status: DOCUMENT ONLY — no implementation performed**

The notification system will be phased incrementally, prioritizing payment-critical interactions before general community broadcasts:

| Phase | Title | Scope & Description | Status |
|---|---|---|---|
| **Phase 1** | Notification domain & architecture documentation | System architecture, events, targeting matrix, deduplication, and security rules. | **CURRENT TASK: DOCUMENT ONLY** |
| **Phase 2** | Device registration & FCM token management | `user_devices` table, device registration API, token refresh, and logout cleanup. | NOT IMPLEMENTED |
| **Phase 3** | Backend notification dispatcher core | Dispatcher service, deduplication storage, FCM client integration, and failure decoupling. | NOT IMPLEMENTED |
| **Phase 4** | **High-Priority Iuran Payment Notifications** | | NOT IMPLEMENTED |
| ↳ *Phase 4a* | *New Iuran Payment Submitted* | Event-driven notification to `bendahara` / authorized verification staff in affected RT. | NOT IMPLEMENTED |
| ↳ *Phase 4b* | *Iuran Payment Verification Result* | Event-driven notification to payment submitter only (`APPROVED` and `REJECTED`). | NOT IMPLEMENTED |
| **Phase 5** | New Iuran Bill Notification | Event-driven notification to household resident upon single or batch bill generation. | NOT IMPLEMENTED |
| **Phase 6** | Iuran Due-Date & Arrears Reminders | Backend scheduler evaluation, arrears stage deduplication, and push alerts. | NOT IMPLEMENTED |
| **Phase 7** | Waste Truck Arrival Notification | Manual trigger by Security/Pengurus with RT-scoped warga broadcast. | NOT IMPLEMENTED |
| **Phase 8** | Finance Income & Expense Notifications | Informational notifications for new posted transactions within the RT. | NOT IMPLEMENTED |
| **Phase 9** | Persistent Overdue Warning & Status API | In-app warning UI state derived from authoritative backend arrears status. | NOT IMPLEMENTED |
| **Phase 10** | Client Notification Center & Deep Linking | Flutter and React Web deep link routing, authorization guards, and history UI. | NOT IMPLEMENTED |

---

## 16. Testing Plan (Future)

**Status: DESIGN-LEVEL TEST SPECIFICATIONS (PLANNED / NOT IMPLEMENTED)**
*(These test specifications define requirements for future notification implementation and are clearly distinguished from the current codebase automated tests in `backend/internal/...` and `frontend/src/...`)*

### 16.1 Unit Tests

- **RT & Role Targeting Logic:**
  - Verify that `New Iuran Payment Submitted` targets only users with role `bendahara` (or authorized verification permissions) in the target RT, and excludes regular `pengurus` or `warga`.
  - Verify that `Iuran Payment Verification Result` resolves only to the submitter's `user_id`.
  - Verify that `New Iuran Bill` resolves to the registered occupants of the billed household.
- **Deduplication Identity Generator:**
  - Test deterministic key generation for all four Iuran events (`bill_id`, `payment_id:status`, `payment_id:pending`, `bill_id:stage`).
- **Payload Sanitizer & Privacy Guards:**
  - Verify that notification payload serialization strips monetary amounts, bank numbers, and NIK.
  - Verify that `officer_notes` is strictly omitted from push payloads for rejected payments, allowing only `rejection_reason`.
- **Reminder Threshold & Stage Calculation:**
  - Verify backend-authoritative overdue days/months calculation against mock billing fixtures.

### 16.2 Integration Tests

- **New Bill Notification Dispatch:**
  - Create bill via API → verify notification event dispatched to household resident in that RT.
- **Payment Submission Notification:**
  - Resident submits payment → payment persisted with status `PENDING` → verify notification dispatched to RT bendahara; verify non-finance pengurus receive nothing.
- **Payment Verification Result (Approved & Rejected):**
  - Bendahara verifies payment as `APPROVED` → notification dispatched strictly to submitter.
  - Bendahara verifies payment as `REJECTED` with citizen reason → notification dispatched strictly to submitter with user-facing reason; verify `officer_notes` not present.
- **Duplicate Prevention on Retries:**
  - Simulate retried payment submission or verification API call with same idempotency key → verify only one push notification is dispatched.
- **Reminder Stage Deduplication:**
  - Run scheduler job on day 1 (emits stage 1 reminder) → run scheduler job on day 2 while still in stage 1 → verify no duplicate push notification is emitted.
- **Decoupled Financial Integrity:**
  - Mock FCM gateway error (500 / timeout) during payment verification → verify payment transaction still commits successfully as `APPROVED` or `REJECTED` in PostgreSQL database.

### 16.3 Security & Tenant Isolation Tests

- **Cross-RT Isolation:**
  - RT 01 resident creates bill/payment → verify no FCM tokens belonging to RT 02 users are targeted.
  - User from RT 02 cannot receive or query notifications from RT 01.
- **Client Parameter Tampering Prevention:**
  - API call attempting to pass arbitrary `recipient_id` or `rt_id` in notification triggers is rejected or overridden by JWT context.
- **Deep Link Authorization Enforcement:**
  - User taps deep link `/iuran/:id/verify` without `bendahara` or verification role → app/backend returns HTTP 403 Forbidden.
  - Warga taps deep link to another household's bill → backend returns HTTP 404/403.
- **Payload Privacy Verification:**
  - Validate that raw FCM push message inspection reveals zero PII, zero amounts, and zero internal management notes.

### 16.4 Device & Transport Tests

- **Multi-Device Dispatch:** User with two active devices receives push on both.
- **Invalid Token Invalidation:** When FCM returns `UNREGISTERED`, device token is deactivated in database.
- **Logout Invalidation:** Logging out from client explicitly deactivates device token.

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
