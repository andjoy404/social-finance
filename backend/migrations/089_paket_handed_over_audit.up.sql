-- 089_paket_handed_over_audit.up.sql
-- Add audit columns for package handover (serah terima paket ke warga)

ALTER TABLE paket_items
    ADD COLUMN IF NOT EXISTS handed_over_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS handed_over_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_paket_items_handed_over_by
    ON paket_items(handed_over_by_user_id)
    WHERE handed_over_by_user_id IS NOT NULL;

