-- Migration 029: Add RW membership and jabatan support

-- =====================================================================
-- user_rw_memberships: connects a user to an RW with a role/jabatan.
-- =====================================================================

CREATE TABLE IF NOT EXISTS user_rw_memberships (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rw_id      uuid NOT NULL REFERENCES rws(id) ON DELETE CASCADE,
    role       text NOT NULL CHECK (role IN (
        'super_admin', 'pengurus', 'bendahara', 'warga'
    )),
    jabatan    text NULL
        CHECK (jabatan IN (
            'ketua', 'wakil_ketua', 'sekretaris', 'bendahara'
        )),
    rt_id      uuid REFERENCES rts(id) ON DELETE SET NULL,
    is_active  boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_rw_memberships_user_rw_active
    ON user_rw_memberships (user_id, rw_id)
    WHERE is_active = true;

CREATE UNIQUE INDEX idx_rw_memberships_rw_jabatan_active
    ON user_rw_memberships (rw_id, jabatan)
    WHERE is_active = true AND jabatan IS NOT NULL;

CREATE INDEX idx_rw_memberships_user_id ON user_rw_memberships (user_id);
CREATE INDEX idx_rw_memberships_rw_id ON user_rw_memberships (rw_id);
CREATE INDEX idx_rw_memberships_rt_id ON user_rw_memberships (rt_id);

-- =====================================================================
-- user_rw_membership_history: audit trail for RW jabatan changes
-- =====================================================================

CREATE TABLE IF NOT EXISTS user_rw_membership_history (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    rw_membership_id uuid NOT NULL REFERENCES user_rw_memberships(id) ON DELETE CASCADE,
    user_id         uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rw_id           uuid NOT NULL REFERENCES rws(id) ON DELETE CASCADE,
    rt_id           uuid REFERENCES rts(id) ON DELETE SET NULL,
    jabatan_old     text NULL,
    jabatan_new     text NULL,
    action          text NOT NULL CHECK (action IN ('assign', 'change', 'revoke')),
    actor_id        uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_rw_membership_history_rw_id ON user_rw_membership_history (rw_id);
CREATE INDEX idx_rw_membership_history_user_id ON user_rw_membership_history (user_id);
CREATE INDEX idx_rw_membership_history_created_at ON user_rw_membership_history (created_at);
