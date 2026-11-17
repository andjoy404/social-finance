-- Migration 074: Revoke banner_templates.manage from Sekretaris RT
-- Migration 072 grants announcements CRUD to Sekretaris RT but banner_templates.manage
-- is not required for managing announcements; the banner template library is reserved
-- for Ketua RT. This migration revokes that overgrant from already-applied databases.
BEGIN;

DELETE FROM position_permissions
WHERE position = 'sekretaris'
  AND permission_id = (SELECT id FROM permission_types WHERE code = 'banner_templates.manage');

COMMIT;
