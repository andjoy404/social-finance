-- Migration 011: Add RT organizational positions and permission infrastructure

-- =====================================================================
-- Positions (jabatan) — defined once as a text CHECK constraint
-- =====================================================================

ALTER TABLE user_rt_memberships
    ADD COLUMN IF NOT EXISTS jabatan text NULL
    CHECK (jabatan IN (
        'ketua', 'wakil_ketua', 'sekretaris', 'bendahara',
        'keamanan', 'sosial', 'kebersihan_pembangunan', 'anggota'
    ));

COMMENT ON COLUMN user_rt_memberships.jabatan IS
    'RT organizational position (jabatan). NULL means no specific position.';

-- =====================================================================
-- Permission types — granular capabilities
-- =====================================================================

CREATE TABLE permission_types (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code       text NOT NULL UNIQUE CHECK (char_length(code) > 0),
    domain     text NOT NULL,
    operation  text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE permission_types IS 'Defines all granular permission codes (e.g. warga.read, finance.create).';
COMMENT ON COLUMN permission_types.code IS 'Unique permission identifier, e.g. warga.read';
COMMENT ON COLUMN permission_types.domain IS 'Feature domain: warga, finance, reports, rt.settings';
COMMENT ON COLUMN permission_types.operation IS 'Action: read, create, update, approve, import, export, manage';

CREATE INDEX idx_permission_types_domain ON permission_types (domain);

-- =====================================================================
-- Position permissions — which positions get which permissions
-- =====================================================================

CREATE TABLE position_permissions (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    position       text NOT NULL CHECK (position IN (
        'ketua', 'wakil_ketua', 'sekretaris', 'bendahara',
        'keamanan', 'sosial', 'kebersihan_pembangunan', 'anggota'
    )),
    permission_id  uuid NOT NULL REFERENCES permission_types(id) ON DELETE CASCADE,
    created_at     timestamptz NOT NULL DEFAULT now(),
    UNIQUE (position, permission_id)
);

COMMENT ON TABLE position_permissions IS 'Maps RT positions to granular permissions.';

CREATE INDEX idx_position_permissions_position ON position_permissions (position);
CREATE INDEX idx_position_permissions_permission ON position_permissions (permission_id);
