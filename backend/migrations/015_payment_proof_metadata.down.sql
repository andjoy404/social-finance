-- 015_payment_proof_metadata.down.sql
ALTER TABLE payments
    DROP COLUMN IF EXISTS proof_original_name,
    DROP COLUMN IF EXISTS proof_mime_type,
    DROP COLUMN IF EXISTS proof_size_bytes,
    DROP COLUMN IF EXISTS proof_uploaded_at,
    DROP COLUMN IF EXISTS proof_uploaded_by;
