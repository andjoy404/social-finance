-- Migration 086 (UP):
-- Tables and permissions for Patroli (Patrol) and Paket (Package Delivery)

-- 1. Insert Permission Types
INSERT INTO permission_types (code, domain, operation) VALUES
    ('patrol.create', 'patrol', 'create'),
    ('patrol.read', 'patrol', 'read'),
    ('patrol.template.manage', 'patrol', 'template.manage'),
    ('paket.create', 'paket', 'create'),
    ('paket.read', 'paket', 'read'),
    ('paket.status_change', 'paket', 'status_change')
ON CONFLICT (code) DO NOTHING;

-- 2. Grant permissions to positions
-- Keamanan: patrol.create, patrol.read, paket.create, paket.read, paket.status_change
INSERT INTO position_permissions (position, permission_id)
SELECT 'keamanan', id FROM permission_types
WHERE code IN ('patrol.create', 'patrol.read', 'paket.create', 'paket.read', 'paket.status_change')
ON CONFLICT DO NOTHING;

-- Ketua: patrol.read, patrol.template.manage, paket.read
INSERT INTO position_permissions (position, permission_id)
SELECT 'ketua', id FROM permission_types
WHERE code IN ('patrol.read', 'patrol.template.manage', 'paket.read')
ON CONFLICT DO NOTHING;

-- Sekretaris: patrol.read, paket.read
INSERT INTO position_permissions (position, permission_id)
SELECT 'sekretaris', id FROM permission_types
WHERE code IN ('patrol.read', 'paket.read')
ON CONFLICT DO NOTHING;

-- Bendahara: patrol.read, paket.read
INSERT INTO position_permissions (position, permission_id)
SELECT 'bendahara', id FROM permission_types
WHERE code IN ('patrol.read', 'paket.read')
ON CONFLICT DO NOTHING;

-- 3. Patrol Tables
CREATE TABLE IF NOT EXISTS patrol_templates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id uuid NOT NULL REFERENCES rts(id) ON DELETE CASCADE,
    name text NOT NULL,
    location_label text NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS patrol_bundles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id uuid NOT NULL REFERENCES rts(id) ON DELETE CASCADE,
    created_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    current_duty_officer_id uuid REFERENCES users(id) ON DELETE SET NULL,
    submitted_at timestamptz NOT NULL DEFAULT NOW(),
    notes text,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS patrol_photos (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    patrol_bundle_id uuid NOT NULL REFERENCES patrol_bundles(id) ON DELETE CASCADE,
    patrol_template_id uuid NOT NULL REFERENCES patrol_templates(id) ON DELETE RESTRICT,
    image_key text NOT NULL,
    captured_at timestamptz NOT NULL DEFAULT NOW(),
    created_at timestamptz NOT NULL DEFAULT NOW(),
    CONSTRAINT uniq_patrol_bundle_template UNIQUE (patrol_bundle_id, patrol_template_id)
);

CREATE INDEX IF NOT EXISTS idx_patrol_templates_rt ON patrol_templates(rt_id);
CREATE INDEX IF NOT EXISTS idx_patrol_bundles_rt ON patrol_bundles(rt_id);
CREATE INDEX IF NOT EXISTS idx_patrol_bundles_created_by ON patrol_bundles(created_by);
CREATE INDEX IF NOT EXISTS idx_patrol_photos_bundle ON patrol_photos(patrol_bundle_id);
CREATE INDEX IF NOT EXISTS idx_patrol_photos_template ON patrol_photos(patrol_template_id);

-- 4. Paket Tables
CREATE TABLE IF NOT EXISTS paket_reports (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id uuid NOT NULL REFERENCES rts(id) ON DELETE CASCADE,
    receiver_warga_id uuid NOT NULL REFERENCES residents(id) ON DELETE RESTRICT,
    created_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS paket_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    paket_report_id uuid NOT NULL REFERENCES paket_reports(id) ON DELETE CASCADE,
    description text,
    status text NOT NULL DEFAULT 'dititipkan' CHECK (status IN ('dititipkan', 'sudah_diambil', 'dibatalkan')),
    status_changed_at timestamptz,
    status_changed_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS paket_photos (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    paket_item_id uuid NOT NULL REFERENCES paket_items(id) ON DELETE CASCADE,
    paket_report_id uuid NOT NULL REFERENCES paket_reports(id) ON DELETE CASCADE,
    image_key text NOT NULL,
    captured_at timestamptz NOT NULL DEFAULT NOW(),
    photo_type text NOT NULL CHECK (photo_type IN ('paket', 'bukti_pengambilan')),
    created_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_paket_reports_rt ON paket_reports(rt_id);
CREATE INDEX IF NOT EXISTS idx_paket_reports_receiver ON paket_reports(receiver_warga_id);
CREATE INDEX IF NOT EXISTS idx_paket_items_report ON paket_items(paket_report_id);
CREATE INDEX IF NOT EXISTS idx_paket_photos_item ON paket_photos(paket_item_id);
CREATE INDEX IF NOT EXISTS idx_paket_photos_report ON paket_photos(paket_report_id);
