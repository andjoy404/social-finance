# Social Finance — Authorization Design Document

_Authored: 2026-09-27_

## Purpose

This document describes the **proposed authorization design** for Social Finance,
covering system roles, RT organizational positions (jabatan), and the permission
model. It is a **design discussion** — no implementation has been made.

---

## 1. Core Principle

> **Role** answers "what class of user is this?"
>
> **Position** answers "what is this person's position in the RT organization?"
>
> **Permission** answers "what can this person actually do?"

These three concepts must remain **separate** throughout the system.

---

## 2. Current System Roles (Implemented)

The system has **three** high-level system roles stored in the `system_role`
column on the `users` table:

| system_role     | Scope       | Description |
|-----------------|-------------|-------------|
| `super_admin`   | Platform    | Platform administrator managing multiple RTs |
| _(null)_        | Single RT   | Tenant-only users (no global authority) |

Tenant-only users carry a **tenant role** (`role` column in JWT claims, stored
in `user_rt_memberships.role`):

| tenant role  | Scope   | Description |
|--------------|---------|-------------|
| `pengurus`   | Single RT | RT executive managing users, households, residents, and admin tasks |
| `bendahara`  | Single RT | RT treasurer managing finances |
| `warga`      | Single RT | RT resident viewing own bills and payments |

These roles are currently the **only** authorization dimension in the system.

### Current AuthContext (backend)

```go
type AuthContext struct {
    UserID       string
    SystemRole   SystemRole // "super_admin" or empty
    MembershipID string     // empty for system-only users
    RTID         string     // empty for system-only users
    TenantRole   Role       // "pengurus", "bendahara", "warga", or empty
}
```

### Current AuthUser (frontend)

```typescript
interface AuthUser {
  id: string
  name: string
  email: string
  systemRole: string | null   // "super_admin" or null
  role: string                // "pengurus", "bendahara", "warga"
  rt: { id: string; name: string } | null
}
```

---

## 3. RT Organizational Positions (Jabatan)

RTs have an internal organizational structure. These positions are **not**
authentication roles — they describe a person's standing within their RT.

### Defined Positions

| No | Position                    | English Equivalent |
|----|-----------------------------|--------------------|
| 1  | Ketua                     | Chairman |
| 2  | Wakil Ketua               | Vice Chairman |
| 3  | Sekretaris                | Secretary |
| 4  | Bendahara                 | Treasurer |
| 5  | Seksi Keamanan            | Security Section |
| 6  | Seksi Sosial              | Social Section |
| 7  | Seksi Kebersihan/Pembangunan | Cleanliness/Development Section |
| 8  | Anggota                   | Member |

### Database Representation (proposed)

```
user_rt_memberships
├── id (UUID, PK)
├── user_id (FK → users.id)
├── rt_id (FK → rts.id)
├── role (text: "pengurus", "bendahara", "warga")
├── jabatan (text, nullable: "ketua", "wakil_ketua", ...)
└── is_active (boolean)
```

The `jabatan` column is **optional** — not every user needs an RT position.
A `warga` user, for example, is typically not assigned a jabatan.

---

## 4. Why Positions Are NOT Roles

### Current model (before this design)

Authentication roles served double duty: they determined **who the user is**
(system role) **and** **what they can do** (tenant role).

### Problem with this approach

If we made each RT position (`ketua`, `sekretaris`, etc.) a system
authentication role, we would:

1. **Bloat the role space** — each RT would need its own set of auth roles,
   making system-level authorization complex.
2. **Break tenant isolation** — a `ketua` of RT-003 would need special-cased
   queries to prevent access to RT-004 data.
3. **Create overlap** — `bendahara` (current tenant role) and `sekretaris`
   (proposed RT position) would both represent different dimensions of
   authority, leading to confusion.
4. **Make RBAC unmaintainable** — permissions would be tied to specific
   position names rather than abstract capabilities.

### Decision

> RT positions are **organizational metadata**, not authentication roles.
> A user is authenticated by their system role and tenant role. Their position
> within the RT is stored separately and may influence future permission
> checks.

---

## 5. Proposed Authorization Hierarchy

```
System Role          →  super_admin / null (global authority)
    ↓
RT / Tenant          →  Which RT does this user belong to? (rt_id)
    ↓
Tenant Role          →  pengurus / bendahara / warga (access level)
    ↓
Position (Jabatan)   →  ketua / sekretaris / bendahara / anggota, etc.
    ↓
Permission           →  warga.read, finance.create, reports.read, etc.
    ↓
Allow / Deny         →  Final decision
```

### Example

```
system_role = (null)          — not a super admin
rt_id     = "rt-003"          — member of RT 003
role      = "pengurus"        — administrative access level
jabatan   = "sekretaris"      — holds secretary position in RT 003
```

This user:
- Is authenticated as a **pengurus** (tenant role).
- Belongs to **RT-003** only.
- Holds the **sekretaris** position within that RT.
- Their permissions would be determined by combining their tenant role
  (`pengurus`) with their position (`sekretaris`) through a permission matrix.

---

## 6. Permission Concept (Proposed)

Permissions are **granular capabilities** that answer "what can be done?"
rather than "who is this person?".

### Permission naming convention

```
<domain>.<operation>
```

### Example permissions

```
warga.read              — view resident/household data
warga.create            — create a new resident
warga.update            — modify existing resident data
warga.import            — import resident data via CSV
warga.export            — export resident data

finance.read            — view financial records
finance.create          — create income/expense transactions
finance.update          — modify transactions
finance.approve         — approve pending transactions

reports.read            — access financial reports and dashboards

rt.settings.read        — view RT configuration
rt.settings.manage      — modify RT settings (name, number, etc.)
```

### Permission evaluation (proposed)

```
Permission = SystemRole  →  (system-level permissions for super_admin)
           + TenantRole   →  (base permissions for pengurus/bendahara/warga)
           + Jabatan      →  (position-specific permissions within the RT)
```

A user is **allowed** if any combination grants the permission.
A user is **denied** only if no combination grants it.

---

## 7. Tenant / RT Isolation

### Current behavior (unchanged)

- `rt_id` is **never** trusted from client input. It is derived from the
  authenticated user's JWT claims.
- Every repository query on tenant-scoped data includes a `rt_id` filter.
- `super_admin` endpoints accept `rt_id` as a path parameter but verify it
  against the `rts` table.

### Position does not override isolation

A `pengurus` with jabatan `ketua` of RT-003 **still**:
- Can only operate within RT-003.
- Cannot access RT-004 data.
- Has their JWT `rt_id` claim enforced at every API layer.

Position affects **what** a user can do **within** their RT, not **which** RT
they can access.

---

## 8. Super Admin Behavior

```
system_role = super_admin
```

- Remains a **system-level** administrative role.
- Is **not** an RT position (jabatan).
- Can manage multiple RTs, view cross-RT reports, create RTs, and manage
  other super admins.
- A super admin user may or may not have a tenant membership (RT affiliation).
- When operating as super_admin, the user has **no rt_id** restriction.

---

## 9. Warga Behavior

```
system_role = null
role        = warga
```

- Remains a separate system role.
- Does **not** require an RT organizational position (jabatan).
- Typically has **read-only** access to their own household's data.
- May have limited write capabilities (e.g., self-registration, bill payment)
  as defined by future permission rules.
- Their access is always scoped to their affiliated RT.

---

## 10. Example Users

### Example 1: Super Admin

```
id:            "usr-001"
system_role:   "super_admin"
role:          (none)
rt_id:         (none)
jabatan:       (not applicable)
```

Can manage all RTs, view cross-RT reports, create users across all RTs.

### Example 2: Pengurus with Ketua Position (RT 003)

```
id:            "usr-010"
system_role:   null
role:          "pengurus"
rt_id:         "rt-003"
jabatan:       "ketua"
```

Can manage users, households, residents, and finances **within RT 003 only**.
The `ketua` position may grant additional permissions (e.g., RT settings
management) beyond the base `pengurus` role.

### Example 3: Bendahara (RT 003)

```
id:            "usr-011"
system_role:   null
role:          "bendahara"
rt_id:         "rt-003"
jabatan:       (none)
```

Can manage finances **within RT 003**. No household/resident management
capabilities (unless granted via position).

### Example 4: Warga (RT 003)

```
id:            "usr-020"
system_role:   null
role:          "warga"
rt_id:         "rt-003"
jabatan:       (none)
```

Can view own household's bills and payments within RT 003.

### Example 5: Anggota with Keamanan Position (RT 004)

```
id:            "usr-030"
system_role:   null
role:          "warga"
rt_id:         "rt-004"
jabatan:       "keamanan"
```

Is a regular citizen (`warga`) but serves as security in RT 004.
The `keamanan` position may grant limited read access to certain
reports or incident data within RT 004.

---

## 11. Example Permission Matrix (PROPOSED — NOT IMPLEMENTED)

> **This matrix is a design discussion baseline only.**
> It does NOT reflect implemented permissions.
> Do NOT treat these as finalized requirements.

| Position               | Warga          | Finance        | Reports     | RT Settings   |
|------------------------|----------------|----------------|-------------|---------------|
| Ketua                  | Full           | Full           | Full        | Manage        |
| Wakil Ketua            | Full           | Configurable   | Full        | Limited       |
| Sekretaris             | Full           | None/Config    | Full        | None/Config   |
| Bendahara              | Read           | Full           | Full        | None          |
| Keamanan               | Read           | None           | Limited     | None          |
| Sosial                 | Read           | None           | Limited     | None          |
| Kebersihan/Pembangunan | Read           | None           | Limited     | None          |
| Anggota                | Read           | None           | Limited     | None          |

**Permission level legend:**
- **Full** — full read/write access to the domain within the RT.
- **Read** — view-only access.
- **Limited** — access to specific reports or summaries only.
- **Configurable** — depends on the RT's configuration or the base tenant role.
- **None** — no access.
- **Manage** — can modify settings.
- **None/Config** — depends on configuration.

### Base permissions by tenant role (without position)

These are the current implicit permissions, which would be made explicit
in the permission matrix above:

| Tenant Role  | Base Capabilities |
|--------------|-------------------|
| Pengurus     | Manage users, households, residents |
| Bendahara    | Manage finances (bills, payments, transactions) |
| Warga        | View own bills and payments |

---

## 12. Future Implementation Considerations

### Database changes (when ready to implement)

1. Add `jabatan` column to `user_rt_memberships`:
   ```sql
   ALTER TABLE user_rt_memberships
     ADD COLUMN jabatan TEXT NULL
     CHECK (jabatan IN (
        'ketua', 'wakil_ketua', 'sekretaris', 'bendahara',
        'keamanan', 'sosial', 'kebersihan_pembangunan', 'anggota'
     ));
   ```

2. Optionally create a `permissions` table for configurable RBAC:
   ```sql
   CREATE TABLE user_permissions (
     id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
     user_id    UUID NOT NULL REFERENCES users(id),
     rt_id      UUID NOT NULL REFERENCES rts(id),
     permission TEXT NOT NULL,
     granted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
     UNIQUE (user_id, rt_id, permission)
   );
   ```

### Permission middleware (Go backend)

```go
// Current middleware (role-based)
RequireRole("pengurus") → allow/deny based on JWT role claim

// Future middleware (permission-based)
RequirePermission("warga.read") → check permission table / position matrix
```

### Frontend considerations

- Current `AuthUser` interface would gain a `jabatan` field:
  ```typescript
  interface AuthUser {
    // ... existing fields ...
    jabatan: string | null
  }
  ```
- UI visibility would transition from role-based to permission-based:
  ```typescript
  // Current
  if (user.role === 'pengurus') { showUserManagement }

  // Future
  if (hasPermission('warga.read')) { showUserManagement }
  ```

---

## 13. Explicit Non-Goals / Things NOT Being Implemented Yet

The following are **explicitly out of scope** for this design document:

| # | Not Implemented | Reason |
|---|-----------------|--------|
| 1 | New authentication roles | The system keeps 3 roles: super_admin, pengurus, bendahara, warga |
| 2 | RT position as auth role | Positions are organizational metadata, not auth roles |
| 3 | Permission table/schema | Database changes deferred until permission system is prioritized |
| 4 | Permission middleware | Current role-based middleware remains; permission middleware is future work |
| 5 | Permission enforcement on API | No API endpoint changes; current role checks remain |
| 6 | Position assignment UI | No UI for assigning jabatan to users |
| 7 | Permission matrix implementation | The matrix in Section 11 is discussion only |
| 8 | Migration of existing data | No data migration strategy for current users |
| 9 | Breaking API changes | The API contract remains unchanged |
| 10 | Multi-role users | A user has ONE system_role and ONE tenant_role |

---

## References

| Document | Relevance |
|----------|-----------|
| `docs/architecture.md` | System architecture overview |
| `docs/database.md` | PostgreSQL schema including `users`, `user_rt_memberships` |
| `docs/security.md` | Auth, RBAC, data protection |
| `docs/current-project-state.md` | Current project state and migrations |
| `backend/internal/auth/models.go` | AuthContext, SystemRole, Role definitions |
| `frontend/src/app/AuthContext.tsx` | Frontend AuthUser interface |
| `AGENTS.md` | AI coding assistant instructions |
