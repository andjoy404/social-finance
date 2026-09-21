-- 003_separate_global_identity_from_tenant_membership.down.sql
--
-- Reverses migration 003. Note: this may restore data that was
-- cleaned up during the up migration, so existing rows may be
-- incomplete after a down migration (e.g. role='super_admin' memberships
-- are gone and cannot be restored).

-- ================================================================
-- 1. Drop normalized email index, revert email to nullable
-- ================================================================
DROP INDEX IF EXISTS idx_users_email_normalized;
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;

-- ================================================================
-- 2. Drop system_role
-- ================================================================
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_system_role_check;
ALTER TABLE users DROP COLUMN system_role;

-- ================================================================
-- 3. Restore rt_id to users with conditional email indexes
-- ================================================================
ALTER TABLE users ADD COLUMN rt_id uuid NULL REFERENCES rts(id);
CREATE INDEX IF NOT EXISTS idx_users_rt_id ON users (rt_id);
-- Restore conditional uniqueness: super_admin users have email unique across NULL rt_id
-- Tenant users have email unique within their rt_id
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_super_admin
    ON users (email)
    WHERE rt_id IS NULL AND email IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_rt_id
    ON users (email, rt_id)
    WHERE rt_id IS NOT NULL AND email IS NOT NULL;

-- ================================================================
-- 4. Restore super_admin in membership role constraint
-- ================================================================
ALTER TABLE user_rt_memberships DROP CONSTRAINT IF EXISTS user_rt_memberships_role_check;
ALTER TABLE user_rt_memberships ADD CONSTRAINT user_rt_memberships_role_check
    CHECK (role IN ('super_admin', 'pengurus', 'bendahara', 'warga'));
