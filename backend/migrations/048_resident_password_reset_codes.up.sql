-- Migration 048: Add resident password reset codes table
CREATE TABLE IF NOT EXISTS resident_password_reset_codes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    resident_id uuid NOT NULL REFERENCES residents(id) ON DELETE CASCADE,
    rt_id uuid NOT NULL REFERENCES rts(id) ON DELETE CASCADE,
    code_hash text NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at timestamptz NULL,
    created_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_reset_codes_user_id ON resident_password_reset_codes(user_id);
CREATE INDEX IF NOT EXISTS idx_reset_codes_resident_id ON resident_password_reset_codes(resident_id);
CREATE INDEX IF NOT EXISTS idx_reset_codes_rt_id ON resident_password_reset_codes(rt_id);
CREATE INDEX IF NOT EXISTS idx_reset_codes_active ON resident_password_reset_codes(user_id, expires_at) WHERE used_at IS NULL;

