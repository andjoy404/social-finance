-- Migration 047: Add proof support for rw_transactions
ALTER TABLE rw_transactions
    ADD COLUMN IF NOT EXISTS proof_path text NULL,
    ADD COLUMN IF NOT EXISTS proof_original_name text NULL,
    ADD COLUMN IF NOT EXISTS proof_mime_type varchar(100) NULL,
    ADD COLUMN IF NOT EXISTS proof_size_bytes bigint NULL,
    ADD COLUMN IF NOT EXISTS proof_uploaded_at timestamptz NULL,
    ADD COLUMN IF NOT EXISTS proof_uploaded_by uuid NULL REFERENCES users(id) ON DELETE SET NULL;

