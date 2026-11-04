-- Migration 052: Down
DROP TABLE IF EXISTS notification_schedules;
DELETE FROM position_permissions WHERE permission_id = (
    SELECT id FROM permission_types WHERE code = 'notification.manage'
);
DELETE FROM permission_types WHERE code = 'notification.manage';
