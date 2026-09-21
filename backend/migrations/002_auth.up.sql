-- 002_auth.up.sql
--
-- Phase 2: Authentication tables.
--
-- users: application identities (login accounts).
-- user_rt_memberships: connects a user to an RT with a role.
-- refresh_tokens: opaque refresh token store server-side hashes.

-- ------------------------------------------------------------------
-- users
-- ------------------------------------------------------------------
CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id         uuid NULL REFERENCES rts(id),
    email         text NULL,
    phone         text NULL,
    password_hash text    NOT NULL,
    full_name     text    NOT NULL,
    is_active     boolean NOT NULL DEFAULT true,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

COMMENT ON COLUMN users.rt_id IS
    'NULL for super_admin users; FK to rts.id for tenant users.';

COMMENT ON COLUMN users.email IS
    'Login identifier. NULL means no email login (e.g. bootstrap users).';

-- One (email, rt_id) pair per user (super_admin may have NULL rt_id).
-- Super-admin users: unique email across all NULL-rt_id row.
-- Tenant users: unique email+rt_id pair with rt_id NOT NULL.
CREATE UNIQUE INDEX idx_users_email_super_admin
    ON users (email)
    WHERE rt_id IS NULL AND email IS NOT NULL;

CREATE UNIQUE INDEX idx_users_email_rt_id
    ON users (email, rt_id)
    WHERE rt_id IS NOT NULL AND email IS NOT NULL;

CREATE INDEX idx_users_rt_id ON users (rt_id);
CREATE INDEX idx_users_is_active ON users (is_active);

-- ------------------------------------------------------------------
-- user_rt_memberships
-- ------------------------------------------------------------------
CREATE TABLE user_rt_memberships (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rt_id      uuid NOT NULL REFERENCES rts(id) ON DELETE CASCADE,
    role       text NOT NULL CHECK (role IN (
        'super_admin', 'pengurus', 'bendahara', 'warga'
    )),
    is_active  boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_memberships_user_rt_active
    ON user_rt_memberships (user_id, rt_id)
    WHERE is_active = true;
    -- Prevents duplicate active memberships for the same user/RT pair.

CREATE INDEX idx_memberships_user_id ON user_rt_memberships (user_id);
CREATE INDEX idx_memberships_rt_id  ON user_rt_memberships (rt_id);
CREATE INDEX idx_memberships_rt_active
    ON user_rt_memberships (rt_id, is_active);

-- ------------------------------------------------------------------
-- refresh_tokens
-- ------------------------------------------------------------------
CREATE TABLE refresh_tokens (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    membership_id       uuid NOT NULL REFERENCES user_rt_memberships(id) ON DELETE CASCADE,
    token_hash          text NOT NULL,
    expires_at          timestamptz NOT NULL,
    revoked_at          timestamptz NULL,
    replaced_by_hash    text NULL,
    created_at          timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_refresh_tokens_membership
    ON refresh_tokens (membership_id);
CREATE INDEX idx_refresh_tokens_token_hash
    ON refresh_tokens (token_hash);
CREATE INDEX idx_refresh_tokens_expires
    ON refresh_tokens (expires_at) WHERE revoked_at IS NULL;
