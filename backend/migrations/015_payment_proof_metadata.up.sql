-- 015_payment_proof_metadata.up.sql
-- Add payment proof metadata columns for RustFS storage

ALTER TABLE payments
    ADD COLUMN IF NOT EXISTS proof_original_name TEXT,
    ADD COLUMN IF NOT EXISTS proof_mime_type TEXT,
    ADD COLUMN IF NOT EXISTS proof_size_bytes BIGINT,
    ADD COLUMN IF NOT EXISTS proof_uploaded_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS proof_uploaded_by UUID REFERENCES users(id) ON DELETE SET NULL;
