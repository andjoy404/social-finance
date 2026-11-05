-- Migration 053: Down
-- Only delete rows that were seeded as defaults (is_default = true).
-- User-customised templates/schedules are never touched.
ALTER TABLE notification_schedules DROP CONSTRAINT IF EXISTS uq_notif_schedules_rt_event_name;
DELETE FROM notification_schedules WHERE event_type = 'iuran_payment_reminder' AND name = 'Pengingat Pembayaran Iuran Bulanan' AND is_default = true;
DELETE FROM notification_templates WHERE event_type = 'iuran_payment_reminder' AND name = 'Pengingat Pembayaran Iuran' AND is_default = true;
