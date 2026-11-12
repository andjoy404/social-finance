-- Migration 079: Resident profile photo support + manage permission
-- Adds an avatar_path reference on residents (shared by Warga and Special
-- Residents such as Petugas Keamanan) and grants the dedicated
-- resident.profile_photo.manage capability to position 'ketua' only.
-- Effective permission mapping for the query is extended in
-- internal/auth/handlers.go. Image binaries live in object storage;
-- only the storage key is persisted.

BEGIN;

INSERT INTO permission_types (code, domain, operation) VALUES
    ('resident.profile_photo.manage', 'warga', 'profile_photo.manage')
ON CONFLICT (code) DO NOTHING;

INSERT INTO position_permissions (position, permission_id)
SELECT 'ketua', id FROM permission_types WHERE code = 'resident.profile_photo.manage'
ON CONFLICT (position, permission_id) DO NOTHING;

ALTER TABLE residents
    ADD COLUMN IF NOT EXISTS avatar_path text;

COMMIT;
