# RW / RT Architecture Plan

**Status: PLANNING / ARCHITECTURE REQUIREMENTS**
**Current Backend: RT-Level MVP Verified (W4.2C)**
**RW Implementation: NOT STARTED**
**RW Schema: NOT YET FROZEN**

**Next RW Decision Gate: RW.1 — read-only architecture/backend-v9 discovery + Indonesian administrative-data research, after W4.5 and before W5 Finance.**

---

## 1. Purpose

Social Finance has a **verified RT-level MVP** (W4.2C). Support for RW (Rukun Warga) is a **future domain extension** — not yet started, not yet implemented, not yet frozen.

**Core philosophy:**

> Shared transparency, isolated authority, audited emergency override.
> Vertical verified visibility, horizontal tenant isolation.
> No transitive permissions.

RW must **NOT** be modeled as an administrator of RT.
RT must **NOT** be modeled as an administrator of RW.

---

## 2. Context: Current State

### 2.1 Verified Current Architecture

| Component | Status | Notes |
|-----------|--------|-------|
| Backend foundation | RT-Level MVP Verified | Docker Compose, PostgreSQL, Chi router, golang-migrate |
| Migration runner | Operational | Manual execution via `docker compose run --rm migrate up` |
| Authentication & Authorization | Verified | JWT, refresh tokens, argon2id, rate limiting, tenant isolation |
| RT Management | Verified | SUPER_ADMIN CRUD, active-only uniqueness on (rw, rt) |
| Household & Resident Management | Verified | CRUD for Pengurus; Warga scope limited |
| Financial Categories, Dues, Bills, Payments, Transactions | Schema in Migration 009 | Implementation phased (W4–W7) |
| Security hardening | Phase 2.5 In Progress | API errors, validation, logging, pagination |

### 2.2 What This Document Does NOT Do

This document does **NOT**:

- Finalize SQL schema.
- Authorize implementation now.
- Modify backend v9 or existing database.
- Redefine the existing RT-level verified behavior.
- Implement Migration 010.
- Choose the authoritative Indonesian administrative dataset.
- Finalize Ketua role modeling.
- Expose sibling RT data.
- Create write inheritance.
- Merge RW and RT finance.
- Make RW mandatory.

### 2.3 What This Document DOES

- Presents **frozen business and security requirements** (Section 3).
- Outlines a **planned architecture direction** (Section 4).
- Identifies **open design questions** (Section 6).
- Documents **securiy invariants and threat cases** (Sections 5–7).
- Defines the **roadmap and timing** (Section 8).

---

## 3. Frozen Requirements

**These requirements are considered authoritative and must NOT be contradicted or bypassed during RW implementation.**

### 3.1 Optional Hierarchy / Partial Adoption

**Status: FROZEN**

RW participation is **optional**. An RT can operate independently even if its real-world RW does not use Social Finance.

All of the following are **valid and expected**:

| Case | RW | RT(s) |
|------|----|-------|
| A | Inactive / not managed | Active (e.g. RT 002) |
| B | Inactive | Some active (e.g. RT 002, RT 007), others not registered |
| C | Active | Some active (RT 002, RT 007), others inactive / not registered |
| D | Active | All active (RT 001–010) |

#### 3.1.1 No Mandatory RW Foreign Key (FROZEN)

**Do NOT freeze `rts.rw_id NOT NULL`.**

Existing RTs **must not need recreation** or destructive migration when their RW later starts using Social Finance. An operational RW foreign key on `rts` is a **deferred design decision only**.

### 3.2 Separate Operational Authority

**Status: FROZEN**

RW and RT are **separate operational scopes**:

| Scope | Writable By |
|-------|-------------|
| RW operational resources | Authorized RW perangkat / memberships only |
| RT operational resources | Authorized perangkat of that RT only |

**Key rules:**

- RW perangkat **MUST NOT** gain write access to RT data merely because the RT is linked to the RW.
- RT perangkat **MUST NOT** gain write access to RW operational data merely because the RT is linked to the RW.
- Other RTs **MUST NOT** gain write access.
- Warga have **no general operational write authority** except explicitly defined resident-specific workflows (e.g., their own payment submission).
- Organizational relationships **do NOT imply write inheritance**.

### 3.3 Provisioning Authority

**Status: FROZEN**

- **SUPER_ADMIN provisions** RW and RT organizations.
- RT operators:
  - **Cannot** create additional RTs.
  - May update their own RT only where policy explicitly permits.
- RW operators:
  - **Should not** create RTs.
  - May update their own RW only where policy explicitly permits.
- Parent-child administrative relationships **must not create provisioning authority**.

### 3.4 Financial Integrity Rules (Inherited from RT MVP)

**Status: FROZEN — applies to all scopes including RW.**

| Rule | Enforcement |
|------|-------------|
| Fixed-precision money | `NUMERIC(15, 2)` in PostgreSQL; `string` in Go / `String` in Flutter |
| Balance derived from ledger | NEVER stored as denormalized value |
| Appended-only financial history | No DELETE or UPDATE on posted transactions |
| Reversal for corrections | New transaction with `reverses_transaction_id` referencing original |
| Idempotency on write endpoints | `Idempotency-Key` header supported |
| Audit trail preserved | All financial events logged; no silent mutation |

### 3.5 No Transitive Permissions

**Status: FROZEN — security invariant.**

If `A → B` is allowed and `B → C` is allowed, this **does NOT imply** `A → C` is allowed.

**Example (non-transitive):**

| Actor | Can read | Can NOT read |
|-------|----------|-------------|
| Warga RT002 | RT002 report, verified parent RW016 report | RT007 report |
| RW016 | RT002 report, RT007 report | — |

This **MUST NOT** imply Warga RT002 → RT007 report. Sibling isolation always applies.

### 3.6 No Cross-Organization Operational Exposing

**Status: FROZEN.**

Cross-organization transparency must use **dedicated published/report read models or endpoints**.

Report visibility **must NOT automatically expose**:

- Payment proof files.
- Idempotency keys.
- Internal audit implementation details.
- Approval workflow internals.
- Private payer information.
- Internal notes.
- Operational write endpoints.
- Unrelated household/resident private data.

---

## 4. Planned Architecture Direction

**This section describes the planned architectural direction for RW support. Exact implementations are deferred to RW.1.**

### 4.1 Architectural Overview

```
Platform (SUPER_ADMIN)
├── RW Organization N (Optional)
│   ├── RT Linked To This RW (Verified, Active)
│   └── RT Linked To This RW (Verified, Active)
├── RW Organization M (Optional, maybe inactive)
└── Standalone RT (No operational parent RW)
    └── Standalone RT (No operational parent RW)
```

Each tier owns its own operational resources.

**Relationships between tiers are verified and explicit — not automatic.**

### 4.2 Organizational Scope

#### 4.2.1 RW Organization Entity (Planned)

RW organizations will likely require a dedicated organizational entity in a future migration.

Key characteristics **planned** but **NOT yet defined**:

- Provisioned by SUPER_ADMIN.
- Canonical administrative area assignment (Provinsi → Kota/Kabupaten → Kecamatan → Kelurahan).
- RW number.
- Operational status (active / inactive).
- Head / leadership designation (open design question — see Section 6).

#### 4.2.2 RW RT Link / Relationship (Planned)

RW ↔ RT links are **explicit, verified, and durable**:

- Discrete organizational entity/table tracking the relationship.
- Separate from both RW and RT organizational records.
- Survives leadership turnover.
- Survives membership expiry (unless manually broken).

### 4.3 Membership Model (Planned Direction)

#### 4.3.1 Current (Frozen — RT Only)

| Table | Purpose |
|-------|---------|
| `user_rt_memberships` | Maps user → RT with role (`pengurus`, `bendahara`, `warga`) |

#### 4.3.2 Planned Extension (RW Memberships)

RW memberships will likely introduce:

- A new membership table or extension to the existing membership model, e.g., `user_rw_memberships` with roles such as `pengurus_rw`, `bendahara_rw`.
- A person may independently hold both an **RW membership** and an **RT membership**.
- Permissions come from **each membership independently**.
- **No transitive permission inheritance** between the two.

**Exact schema is an OPEN QUESTION (Section 6).**

### 4.4 Finance Ownership (Planned Direction)

**RW finance and RT finance are independent operational scopes.**

- RW finance: owned by RW scope, writable only by authorized RW perangkat.
- RT finance: owned by that RT scope, writable only by authorized RT perangkat.

Balances and ledgers **MUST NOT be merged** merely because organizations are linked.

**Do NOT implement RW finance as an `rt_id = NULL` workaround.**

Future finance schema should use **explicit organizational ownership** — scope column, not a `NULL` RT_id hack.

### 4.5 Report Visibility Model (Planned Direction)

#### 4.5.1 Vertical Visibility (Allowed)

```
RW016
├── RT002
└── RT007
```

| Source | Target | Allowed? |
|--------|--------|----------|
| RW016 | RT002 report | YES |
| RW016 | RT007 report | YES |
| RT002 | RW016 report | YES |
| RT007 | RW016 report | YES |
| Warga RT002 | RT002 report | YES |
| Warga RT002 | RW016 report | YES |
| Warga RT007 | RT007 report | YES |
| Warga RT007 | RW016 report | YES |

#### 4.5.2 Sibling Isolation (Forbidden)

| Source | Target | Allowed? |
|--------|--------|----------|
| RT002 | RT007 report | NO |
| RT007 | RT002 report | NO |
| Warga RT002 | RT007 report | NO |
| Warga RT007 | RT002 report | NO |

#### 4.5.3 Horizontal Isolation (Forbidden)

| Source | Target | Allowed? |
|--------|--------|----------|
| RW016 | RW017 report | NO |
| RW017 | RW016 report | NO |
| Any unrelated RT/RW | — | NO |

Administrative-area similarity alone grants **NO** report access.

### 4.6 Published Reports vs Operational APIs

Cross-organization access uses **dedicated read/report endpoints**. Raw operational APIs are scoped to a single organization and do not grant cross-organization visibility.

### 4.7 Behavioral Requirement: RW Inactive

If RW is inactive:

- Linked RTs **remain operational**.
- No cross-RT visibility opens.
- No RW report exists.
- Administrative-area matching grants **no access**.

When RW later becomes active and organizational relationships are verified:

- RW may read published reports from linked RTs.
- RT/Warga may read the RW's published report.
- Sibling RTs remain isolated from each other.

### 4.8 Partial-Adoption Reporting

RW dashboards **MUST NEVER imply complete coverage** when only some RTs are connected.

Real-world RW may represent an area containing RTs that do not use Social Finance.

UI must communicate concepts such as:

- "2 RT terhubung" (X RTs connected).
- "Data dari RT yang menggunakan Social Finance" (Data from RTs using Social Finance).

Aggregates must **include only participating / verified connected RTs**.

### 4.9 SUPER_ADMIN Roles (Planned)

SUPER_ADMIN responsibilities include:

| Responsibility | Description |
|----------------|-------------|
| Provision RW | Create new RW organizations |
| Provision RT | Create new RT organizations |
| Activate/deactivate | Toggle organization status |
| Membership recovery | Kick/replace expired or inactive Ketua/perangkat memberships |
| Organizational recovery | Resolve broken membership ownership |
| Relationship management | Manually break/unlink RT ↔ RW when necessary |
| Emergency intervention | Explicit, audited tenant override (see Section 4.9.1) |

**Distinguish: MEMBERSHIP REVOCATION from ORGANIZATION DEACTIVATION.**
Revoking a Ketua's authority **does NOT** deactivate the organization.

### 4.9.1 Emergency Override (Planned — Not Yet Designed)

Emergency tenant intervention:

- Explicit privileged action.
- Targets a specific organization.
- Requires mandatory **reason**.
- Captures: actor, timestamp, action performed, old/new state, result.
- **Prefers restoring legitimate organizational authority** over directly editing financial records.
- If emergency financial intervention is required:
  - Append-only integrity **remains mandatory**.
  - No silent edit/delete of finalized transactions.
  - Correction/reversal mechanisms **still used**.
  - All SUPER_ADMIN actions affecting organization, membership, RT↔RW link, or finance **must be auditable**.

**Exact mechanism is an OPEN QUESTION (Section 6).**

---

## 5. Canonical Indonesian Administrative Geography

**Required for RW organizational identity — NOT for standalone RTs.**

### 5.1 Cascading Structure

Onboarding / registration should use canonical Indonesian administrative areas:

```
Provinsi
  → Kota/Kabupaten
    → Kecamatan
      → Kelurahan/Desa
```

### 5.2 RW Identity Concepts

RW organizational identity should conceptually include:

- Canonical administrative area (with authoritative identifier/code).
- RW number.

RT organizational identity should conceptually include:

- Canonical administrative area.
- RT number.
- Address (metadata/display only).

**Street / full address:** display metadata, **NOT** the organizational identity key.

**Postal code:** may be useful metadata or validation, **MUST NOT yet be frozen** as a mandatory identity component.

### 5.3 Matching Principle

Conceptual matching / candidate discovery for RT ↔ RW linking should use:

```
RW number + canonical administrative area
```

**Do NOT rely on `RW016` alone as globally unique identity.**

### 5.4 Research Required (Open Question)

RW.1 must research the authoritative Indonesian administrative master-data source:

| Investigation Area |
|--------------------|
| Source authority |
| Identifiers / codes |
| Update mechanism |
| Seed vs sync strategy |
| Versioning implications |
| Licensing / redistribution considerations |

---

## 6. RT ↔ RW Linking — Verification & Durability

### 6.1 Link Activation Flow (Planned)

1. RW organization is provisioned with canonical administrative area + RW number.
2. System discovers existing RT candidates matching same administrative area + RW number.
3. Candidate discovery **does NOT grant authorization**.
4. RW and RT must establish an **explicit organizational link**.
5. The link requires **mutual consent** — Ketua RT **AND** Ketua RW both accept.
6. Only after mutual acceptance is the relationship **VERIFIED / ACTIVE**.
7. Only then does vertical report visibility open.
8. Existing RT Household, Resident, Finance, and historical data **remain untouched**.

**No automatic link based solely on administrative matching.**

### 6.2 Invitation Lifecycle (Conceptual — Not Frozen)

| State | Description |
|-------|-------------|
| PENDING | Link requested, awaiting acceptance |
| ACCEPTED | Both parties accepted |
| REJECTED | One party rejected |
| CANCELLED | Requestor withdrew |
| EXPIRED | Timed out |

### 6.3 Organizational Relationship States (Conceptual — Not Frozen)

| State | Description |
|-------|-------------|
| ACTIVE / VERIFIED | Link is operational |
| UNLINKED | Link was manually broken |

### 6.4 Audit Fields (Conceptual — Not Frozen)

| Concept | Description |
|---------|-------------|
| `requested_by` | Who initiated |
| `requested_at` | When |
| `rw_accepted_by` | RW-side acceptor |
| `rw_accepted_at` | RW-side acceptance time |
| `rt_accepted_by` | RT-side acceptor |
| `rt_accepted_at` | RT-side acceptance time |
| `activated_at` | When VERIFIED |

### 6.5 Link Durability (FROZEN)

The RT ↔ RW link represents a **relationship between organizations**, not between individual officeholders.

**Therefore:**

| Event | Effect on link |
|-------|---------------|
| Leadership turnover | Link **remains** |
| Membership expiry | Link **remains** |
| Revoking old Ketua | Link **remains** |
| Replacing Ketua | Link **remains** |
| Manual unlink | Link **broken** (administrative action) |

---

## 7. Security Invariants

The following must be enforced at the architectural level:

| # | Invariant |
|---|-----------|
| 1 | Candidate administrative match grants **no authorization**. |
| 2 | Unverified RT ↔ RW relationship grants **no vertical visibility**. |
| 3 | Verified relationship grants **only explicitly defined** report visibility (read-only). |
| 4 | Verified relationship grants **no write inheritance**. |
| 5 | Sibling RTs **remain isolated**. |
| 6 | Unrelated RWs **remain isolated**. |
| 7 | Permissions are **non-transitive**. |
| 8 | RW finance and RT finance remain **operationally isolated**. |
| 9 | Cross-organization transparency uses report/read models, **not raw operational access**. |
| 10 | Leadership turnover **does not destroy** organizational relationships. |
| 11 | SUPER_ADMIN emergency actions are **explicit and audited**. |
| 12 | Financial integrity rules **cannot be bypassed** by administrative override. |

---

## 8. Authorization Matrix (Planned Direction — To Be Finalized in RW.1)

**Actors:** SUPER_ADMIN, Pengurus_RW, Bendahara_RW, Pengurus_RT, Bendahara_RT, Warga

**Legend:**
- `✓` — Allowed
- `✗` — Forbidden
- `?` — Conditional / to be determined in RW.1
- `—` — Not applicable

| Action / Resource | SUPER_ADMIN | Pengurus_RW | Bendahara_RW | Pengurus_RT | Bendahara_RT | Warga |
|-------------------|:-----------:|:-----------:|:------------:|:-----------:|:------------:|:-----:|
| Provision RW | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ |
| Provision RT | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ |
| Reactivate RW | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ |
| Deactivate RW | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ |
| Reactivate RT | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ |
| Deactivate RT | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ |
| Update own RW | ? | ✓ | ✓ | ✗ | ✗ | ✗ |
| Update own RT | ✓ | ✗ | ✗ | ✓ | ✗ | ✗ |
| Manage RT ↔ RW link (mutual accept) | ✓ | ? | ? | ✓ | ✗ | ✗ |
| Emergency unlink | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ |
| Write RW finance | ? | ? | ✓ | ✗ | ✗ | ✗ |
| Write RT finance (own) | ✓ | ✗ | ✗ | ✓ | ✓ | ✗ |
| Write another RT finance | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ |
| Read own RT report | ✓ | ✗ | ✗ | ✓ | ✓ | ✓ |
| Read verified parent RW report | ? | ✓ | ? | ✓ | ✓ | ✓ |
| Read verified child RT report | ✓ | ✓ | ? | ✗ | ✗ | ✗ |
| Read sibling RT report | — | ✗ | ✗ | ✗ | ✗ | ✗ |
| Emergency financial override | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ |
| Access payment proof files | ✓ | ? | ? | ✓ | ✓ | ✗ |

**Notes:**

- `?` indicates conditional behavior depending on RW-specific role design decisions deferred to RW.1.
- **No write inheritance**: even verified parent RW has **no** write access to child RT finance or operational resources.
- **Access to payment proof files** in cross-org context is intentionally restricted; report read models should redact sensitive attachments.

---

## 9. Threat / Abuse Cases

| # | Threat Scenario | Expected Security Behavior |
|---|----------------|--------------------------|
| 1 | RW attempts to edit child RT finance | Denied (`403 Forbidden`). RW finance and RT finance scopes are isolated. |
| 2 | RT002 attempts to read RT007 report | Denied (`403 Forbidden`). Sibling RTs are isolated. |
| 3 | Warga RT002 attempts to read RT007 report | Denied (`403 Forbidden`). Warga scope extends to own RT and verified parent RW only. |
| 4 | Candidate administrative match used as authorization | Denied. Administrative matching is **not** authorization. |
| 5 | Invitation accepted by unauthorized user | Denied. Acceptance requires matching Ketua/designation validation. |
| 6 | Expired Ketua attempts to approve link | Denied. Role/membership term is validated at acceptance time. |
| 7 | Leadership turnover accidentally destroys link | Link **must not** be affected. Organizational relationship is independent of officeholder tenure. |
| 8 | SUPER_ADMIN emergency action without reason/audit | Denied or logged. All emergency actions **must** capture reason, actor, timestamp. |
| 9 | Partial-adoption RW report falsely presented as complete coverage | UI must clearly indicate "X RT terhubung" and "Data dari RT yang menggunakan Social Finance". |
| 10 | Cross-org report endpoint leaks raw payment proof / private payer data | Report read models **must redact** private attachments, idempotency keys, internal notes. |

---

## 10. Open Questions for RW.1

**These decisions ARE NOT YET FROZEN.** RW.1 must research and resolve them.

| # | Question | Notes |
|---|----------|-------|
| 1 | Final RW organization schema | What columns, constraints, indexes? |
| 2 | Does RT reference administrative RW identity vs operational RW organization? | Separate columns? FK to RW org? Or administrative data only? |
| 3 | Final user_rw_memberships schema | New table? Extension of `user_rt_memberships`? |
| 4 | Ketua as role vs position/designation? | Is Ketua a membership role? A leadership flag on a membership? Something else? |
| 5 | Membership term model | Exact `starts_at` / `ends_at` schema and enforcement mechanism? |
| 6 | Final RT ↔ RW link schema and state machine | Exact states, transitions, constraints? |
| 7 | Invitation expiration / retry / revocation semantics | How long is a link request valid? Can it be retried? |
| 8 | Authoritative Indonesian administrative-area dataset | Source, codes, update frequency, versioning? |
| 9 | Master-data seed / sync / version strategy | One-time seed? Periodic sync? Versioned? |
| 10 | Postal code role | Metadata only? Mandatory? Validation? |
| 11 | Final organizational finance ownership schema | How is RW vs RT financial scope encoded? |
| 12 | Published report schemas / endpoints | What data is visible cross-org? How is it aggregated? |
| 13 | Report privacy / redaction rules | What fields are excluded from cross-org visibility? |
| 14 | SUPER_ADMIN emergency override mechanism | Exact API, constraints, safeguards? |
| 15 | Audit-event schema for RW | Do existing audit logs cover RW events? New event types needed? |
| 16 | Migration strategy from backend v9 | How to safely add RW tables without breaking existing RT ops? |
| 17 | Exact Migration 010 scope | Which RW concepts land in which migration? Phased rollout? |

---

## 11. Roadmap and Implementation Timing

### 11.1 Current Status

| Item | Status |
|------|--------|
| W4.2C — Household List + Detail | **VERIFIED** |
| Database checkpoint (this document) | **IN PROGRESS** |

### 11.2 Next Steps (Immediate)

| Step | Description |
|------|-------------|
| W4.3 | Household Create / Edit / Deactivate / Move |
| W4.4 | Resident UI / Lifecycle |
| W4.5 | Household / Resident E2E + manual verification |

### 11.3 STOP Before W5 Finance UI

After W4.5:

```
┌─────────────────────────────────────────────────────────────┐
│  STOP BEFORE IMPLEMENTING W5 Finance UI.                     │
│                                                              │
│  Execute RW.1 RESEARCH FIRST:                                │
│  - Read-only architecture / backend-v9 discovery             │
│  - Indonesian administrative-data research                   │
│                                                              │
│  Then review and freeze:                                     │
│  - Schema                                                    │
│  - Authorization                                             │
│  - Membership                                                │
│  - Linking                                                   │
│  - Reports                                                   │
│  - Finance ownership                                         │
│  - SUPER_ADMIN controls                                      │
│  - Migration strategy                                        │
└─────────────────────────────────────────────────────────────┘
```

### 11.4 Post-RW.1 (Tentative)

| Phase | Description |
|-------|-------------|
| Migration 010 + RW Backend | Implementation + security tests + E2E |
| W5 Finance Web UI | Finance UI (only after RW backend contract is stable) |
| Payment UI / Dashboard / Reports / Import-Export | Post-finance foundation |
| Final Web E2E | End-to-end testing |
| Android Integration | Mobile UI for RW + Finance |

---

## 12. Checklist for RW.1

### 12.1 Discovery

- [ ] Research backend v9 structure (current state of all modules)
- [ ] Research authoritative Indonesian administrative-area dataset
- [ ] Map existing `user_rt_memberships` to potential RW membership model
- [ ] Assess Migration 010 scope without committing to specifics yet

### 12.2 Decisions to Finalize

- [ ] Final RW organization schema
- [ ] RT administrative area reference (RW code vs canonical codes)
- [ ] Final RT ↔ RW link schema and state machine
- [ ] Ketua designation model
- [ ] Membership term enforcement
- [ ] Authoritative admin dataset selection
- [ ] Published report schemas
- [ ] SUPER_ADMIN emergency override design
- [ ] Migration 010 scope

### 12.3 Frozen at RW.1 Outcome

Before W5 Finance begins, the following **MUST** be frozen:

- [ ] Database schema (RW tables, link table, membership extension)
- [ ] Authorization model (roles, matrix, middleware extensions)
- [ ] Membership model (how RW users are created, managed, terminated)
- [ ] Linking workflow (invitations, accept/reject, verification)
- [ ] Report visibility contracts (API responses, field redaction rules)
- [ ] Finance ownership model (RW vs RT financial scope)
- [ ] SUPER_ADMIN controls (recovery, override, emergency)
- [ ] Migration strategy (steps, rollback, backwards compatibility)

---

## 13. Summary

This document captures the **frozen requirements** and **planned direction** for RW support in Social Finance.

**Frozen:** Business rules, security invariants, financial integrity, non-transitive permissions, sibling isolation, RW optional, no mandatory FK, separate authority.

**Planned Direction:** RW organizations with explicit verified links to RTs, independent RW and RT scope, vertical read-only report visibility, canonical Indonesian address, mutual Ketua verification.

**Deferred to RW.1:** Schema design, invitation semantics, Ketua modeling, admin dataset selection, emergency override mechanism, migration strategy.

**RW implementation has NOT started.**

---

**Status: PLANNING / ARCHITECTURE REQUIREMENTS**
**Current Backend: RT-LEVEL MVP VERIFIED**
**RW Implementation: NOT STARTED**
**RW Schema: NOT YET FROZEN**
**Next RW Decision Gate: RW.1 after W4.5 and before W5 Finance**
