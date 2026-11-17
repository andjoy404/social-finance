-- Migration 065 (down): remove sekretaris notification.manage grant added by 065.
-- Safe only if no dependency requires this specific grant; additive grant rollback.
DELETE FROM position_permissions
WHERE position = 'sekretaris'
  AND permission_id = (SELECT id FROM permission_types WHERE code = 'notification.manage');
