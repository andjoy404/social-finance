-- 003_separate_global_identity_from_tenant_membership.up.sql
--
-- Phase 3: Separate global identity from tenant membership.
--
-- Changes:
--   1. Add nullable system_role to users for global authorization (e.g. super_admin)
--   2. Remove redundant rt_id from users (membership is in user_rt_memberships)
--   3. Constrain membership role to tenant roles only
--   4. Enforce NOT NULL + global normalized email uniqueness

-- ================================================================
-- 1. Add system_role column with constraint
-- ================================================================
ALTER TABLE users ADD COLUMN system_role text NULL;
ALTER TABLE users ADD CONSTRAINT users_system_role_check
    CHECK (system_role IS NULL OR system_role = 'super_admin');

-- ================================================================
-- 2. Migrate existing super_admin memberships to system_role
-- ================================================================
-- Mark super_admin memberships for deletion after migration
UPDATE user_rt_memberships
SET is_active = false
WHERE role = 'super_admin';

-- Set system_role on the user
UPDATE users
SET system_role = 'super_admin'
WHERE system_role IS NULL
  AND id IN (
    SELECT urm.user_id
    FROM user_rt_memberships urm
    WHERE urm.role = 'super_admin'
  );

-- Delete migrated memberships
DELETE FROM user_rt_memberships WHERE role = 'super_admin';

-- ================================================================
-- 3. Remove redundant rt_id from users
-- ================================================================
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_rt_id_fkey;
DROP INDEX IF EXISTS idx_users_rt_id;
DROP INDEX IF EXISTS idx_users_email_rt_id;
DROP INDEX IF EXISTS idx_users_email_super_admin;
ALTER TABLE users DROP COLUMN rt_id;

-- ================================================================
-- 4. Update membership role constraint to tenant roles only
-- ================================================================
ALTER TABLE user_rt_memberships DROP CONSTRAINT IF EXISTS user_rt_memberships_role_check;
ALTER TABLE user_rt_memberships ADD CONSTRAINT user_rt_memberships_role_check
    CHECK (role IN ('pengurus', 'bendahara', 'warga'));

-- ================================================================
-- 5. Enforce NOT NULL + global normalized email uniqueness
-- ================================================================
-- Ensure all existing emails are normalized before adding constraint
UPDATE users SET email = lower(trim(coalesce(email, ''))) WHERE email IS NOT NULL;

ALTER TABLE users ALTER COLUMN email SET NOT NULL;

CREATE UNIQUE INDEX idx_users_email_normalized
    ON users ((lower(trim(email))));
