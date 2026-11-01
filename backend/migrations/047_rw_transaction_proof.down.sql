-- Migration 047 down: Remove proof support for rw_transactions
ALTER TABLE rw_transactions
    DROP COLUMN IF EXISTS proof_uploaded_by,
    DROP COLUMN IF EXISTS proof_uploaded_at,
    DROP COLUMN IF EXISTS proof_size_bytes,
    DROP COLUMN IF EXISTS proof_mime_type,
    DROP COLUMN IF EXISTS proof_original_name,
    DROP COLUMN IF EXISTS proof_path;

