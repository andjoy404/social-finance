-- Migration 093: Add activity archive support
-- Adds archived_at column to activities and 'activities.archive' permission

-- Add archived_at column (NULL = not archived, timestamp = archived at)
ALTER TABLE activities ADD COLUMN archived_at TIMESTAMPTZ;

-- Add 'activities.archive' permission type
INSERT INTO permission_types (code, domain, operation)
VALUES ('activities.archive', 'community', 'archive')
ON CONFLICT (code) DO NOTHING;

-- Grant activities.archive to Ketua position
INSERT INTO position_permissions (position, permission_id)
SELECT 'ketua', id FROM permission_types
WHERE code = 'activities.archive'
ON CONFLICT (position, permission_id) DO NOTHING;

-- Grant activities.archive to Sekretaris position
INSERT INTO position_permissions (position, permission_id)
SELECT 'sekretaris', id FROM permission_types
WHERE code = 'activities.archive'
ON CONFLICT (position, permission_id) DO NOTHING;

-- Grant activities.archive to Bendahara position
INSERT INTO position_permissions (position, permission_id)
SELECT 'bendahara', id FROM permission_types
WHERE code = 'activities.archive'
ON CONFLICT (position, permission_id) DO NOTHING;

-- Index for archived activities queries
CREATE INDEX IF NOT EXISTS idx_activities_archived ON activities(rt_id, archived_at) WHERE archived_at IS NOT NULL;
