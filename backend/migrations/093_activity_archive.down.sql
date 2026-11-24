-- Rollback: Remove archived_at column and 'activities.archive' permission

-- Drop index
DROP INDEX IF EXISTS idx_activities_archived;

-- Remove archived_at column
ALTER TABLE activities DROP COLUMN IF EXISTS archived_at;

-- Remove permission type
DELETE FROM permission_types WHERE code = 'activities.archive';

-- Remove position permissions
DELETE FROM position_permissions 
WHERE permission_id IN (
    SELECT id FROM permission_types WHERE code = 'activities.archive'
);
