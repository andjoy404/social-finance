-- 041_notification_manual_and_image.down.sql

DELETE FROM notification_templates WHERE event_type = 'manual_push';

DROP INDEX IF EXISTS idx_notification_logs_image;
DROP INDEX IF EXISTS idx_notif_templates_image;

ALTER TABLE notification_logs DROP COLUMN IF EXISTS image_url;
ALTER TABLE notification_templates DROP COLUMN IF EXISTS image_url;

