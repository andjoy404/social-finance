# Migration 070 Safety — Community Delete Permissions

Status: APPROVED FOR DOCUMENTATION — NOT IMPLEMENTED AS AN AUTOMATED ROLLBACK
Author: Migration-safety review (070_community_delete_permissions)
Scope: `backend/migrations/070_community_delete_permissions.{up,down}.sql`

## 1. Executive Summary

Migration 070 registers two canonical permission types:

- `activities.delete` (community)
- `announcements.delete` (community)

and grants them to the Ketua RT position via `position_permissions`.

Both permission types are **permanent, canonical data**. They are created by
the canonical development seed (`backend/internal/devseed/seed.go`) and are
also created by `070_community_delete_permissions.up.sql` in installations
where the seed is not applied. There is no provenance flag, no ledger entry
beyond the version number in `schema_migrations`, and no other column that
distinguishes a seed-created grant from a migration-created one.

Consequence: an automatic `DOWN` that deletes these rows cannot be proved
safe in every environment. This document defines the applicable policy.

## 2. Lifecycle and Provenance

Observed and proven in this repository:

1. Fresh production database: 070 UP creates the two permission types and
   the two ketua grants (no competing writer). UP is idempotent
   (`ON CONFLICT (code) DO NOTHING`, `ON CONFLICT (position, permission_id) DO NOTHING`).
2. Fresh development database with `--seed-dev`: `devseed.Run` creates the
   two permission types (ON CONFLICT DO UPDATE) and grants them to `ketua`
   (ON CONFLICT DO NOTHING). A later 070 UP creates nothing new.
3. Existing development database with devseed: identical to (2). Observed in
   both `social_finance` and `social_finance_test` today.
4. Existing production database: 070 UP has been applied (both databases are
   at `schema_migrations.version >= 070`). The grant state reflects UP.
5. Manual grants: any additional position may hold these permissions at any
   time. The migration cannot and must not attempt to remove them.

## 3. Correct UP behavior

`070_community_delete_permissions.up.sql` is correct and must not be altered:

- Idempotent: repeat execution is a no-op.
- Fresh install: creates what is needed.
- Seeded install: no-op, because devseed already created the rows.
- Re-apply after programmatic removal of the grant: re-creates it
  idempotently.
- It never deletes or revokes anything.

## 4. DOWN: why an automatic delete is unsafe

The in-tree `070_community_delete_permissions.down.sql` deletes grants for
`ketua` and leaves `permission_types` untouched. That is the narrowest
possible delete, but it still deletes rows **indistinguishable** from rows
the canonical seed created. Demonstrated by
`TestMigration070_ShippedDown_IsNotProvablyReversible` and
`TestMigration070_ExistingPermissionAndAdditionalGrants_Preserved`:

- After devseed exists, a `DOWN` that removes the ketua grant removes
  seed-owned data.
- `permission_types` must survive in every environment.
- Non-ketua grants (`wakil_ketua`, `sekretaris`, `bendahara`, ...) are
  administratively created and must never be touched by 070's rollback.

There is no reliable way, in the current schema, to rollback 070 without
either (a) deleting seed-owned/graph rows, or (b) leaving some UP-created
rows behind with no way to tell them apart.

## 5. Policy (SAFE DOWN = NO-OP BY DEFAULT)

1. Do not execute `070_community_delete_permissions.down.sql` automatically.
2. A rollback of 070 is an administrative action, never a routine deploy
   step.
3. The safest statement is: **do nothing**. The permission types are
   harmless canonical RBAC entries; removing them requires manual, audited
   action by the database administrator.
4. If an environment truly needs to remove these permissions:
   - Confirm that no position besides `ketua` relies on the permissions.
   - Confirm that the environment originated from a migration-only install
     that never ran the seed (not a shared development DB).
   - Take an explicit backup or snapshot.
   - Remove grants and permission types with explicit `DELETE` statements,
     reviewing row identity before and after — never `DELETE WHERE code IN
     (...)` blind.
   - If any environment has already executed the in-tree `down.sql`, the
     canonical fix is to run `devseed.Run` or re-apply 070 UP; both restore
     the ketua grant (idempotent) — this is exactly why the rollback
     command must not be used in practice.

## 6. Recommendation for migration sequence 070/071

- `070_community_delete_permissions`: **APPLIED, IMMUTABLE**. Do not edit.
- `071_notification_manage_revoke_bendahara`: **APPLIED, IMMUTABLE**. Do not edit.
- Any change to either migration requires a new migration number and full
  sign-off.
- Fresh environments: apply 070, then 071. Do not roll back either.
- Shared development databases: keep canonical seed; do not execute rollback
  scripts.

## 7. Test coverage

See `backend/internal/auth/migration_070_safety_test.go`. Every test uses
isolated transactions that always roll back; snapshots of the 070 scope are
taken before each test and restored afterwards.

| Scenario | Test | Result |
|---|---|---|
| Fresh install: UP then idempotent UP | `TestMigration070_FreshEnvironment_UP_and_Policy` | PASS |
| Seeded install: seed properties intact | `TestMigration070_SeededEnvironment_Unaffected` | PASS |
| Existing types + additional grants | `TestMigration070_ExistingPermissionAndAdditionalGrants_Preserved` | PASS |
| Mixed ownership (one type exists, other missing) | `TestMigration070_MixedOwnership_PartialPreexistence` | PASS |
| Idempotency + UP→DOWN→UP | `TestMigration070_Idempotency_And_Reapply` | PASS |
| 070 DOWN vs 071 state | `TestMigration070_DownInteractionWith071` | PASS |
| Seed-owned permission type identity | `TestMigration070_SeedOwnedPermissionTypesRemain` | PASS |
| Shipped DOWN is not provably reversible | `TestMigration070_ShippedDown_IsNotProvablyReversible` | PASS |

## 8. What is NOT proved

- Nothing in this document proves anything about production deployments that
  may have had "070 DOWN" executed out of band. Such environments are
  unrecoverable by this document's policy alone.
- The dev seed is the only producer of the two permission types; if
  production is expected to differ, the production migration history must be
  audited independently.
- This document does NOT replace backups or manual review for any rollback
  that actually deletes permission types.
