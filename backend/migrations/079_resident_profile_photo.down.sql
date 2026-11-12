-- Migration 079 Rollback: Resident profile photo support
-- Drops the avatar reference and the manage permission/grant seeded by 079.
-- Does not touch data belonging to other positions.

BEGIN;

ALTER TABLE residents
    DROP COLUMN IF EXISTS avatar_path;

DELETE FROM position_permissions
WHERE permission_id IN (
    SELECT id FROM permission_types WHERE code = 'resident.profile_photo.manage'
);

DELETE FROM permission_types
WHERE code = 'resident.profile_photo.manage';

COMMIT;
