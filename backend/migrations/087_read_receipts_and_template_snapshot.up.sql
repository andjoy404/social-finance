-- Migration 087 (UP):
-- Read receipts for security domain items and snapshot columns for patrol photos

-- 1. Add snapshot columns to patrol_photos to preserve history when templates change
ALTER TABLE patrol_photos ADD COLUMN IF NOT EXISTS template_name text;
ALTER TABLE patrol_photos ADD COLUMN IF NOT EXISTS location_label text;
ALTER TABLE patrol_photos ALTER COLUMN patrol_template_id DROP NOT NULL;

-- Backfill snapshot data for any existing patrol photos
UPDATE patrol_photos pp
SET template_name = pt.name,
    location_label = pt.location_label
FROM patrol_templates pt
WHERE pp.patrol_template_id = pt.id
  AND (pp.template_name IS NULL OR pp.location_label IS NULL);

-- 2. Create security_read_receipts table for persistent unread tracking
CREATE TABLE IF NOT EXISTS security_read_receipts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rt_id uuid NOT NULL REFERENCES rts(id) ON DELETE CASCADE,
    domain text NOT NULL CHECK (domain IN ('correction', 'patrol', 'paket')),
    item_id uuid NOT NULL,
    read_at timestamptz NOT NULL DEFAULT NOW(),
    CONSTRAINT uniq_security_user_domain_item UNIQUE (user_id, domain, item_id)
);

CREATE INDEX IF NOT EXISTS idx_security_reads_lookup ON security_read_receipts(user_id, domain, rt_id);
