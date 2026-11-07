-- Migration 053: Down
-- Only delete rows that were seeded as defaults (is_default = true).
-- User-customised templates/schedules are never touched.
ALTER TABLE notification_schedules DROP CONSTRAINT IF EXISTS uq_notif_schedules_rt_event_name;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'notification_schedules'
          AND column_name = 'is_default'
    ) THEN
        DELETE FROM notification_schedules
        WHERE event_type = 'iuran_payment_reminder'
          AND name = 'Pengingat Pembayaran Iuran Bulanan'
          AND is_default = true;
    ELSE
        DELETE FROM notification_schedules
        WHERE event_type = 'iuran_payment_reminder'
          AND name = 'Pengingat Pembayaran Iuran Bulanan';
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'notification_templates'
          AND column_name = 'is_default'
    ) THEN
        DELETE FROM notification_templates
        WHERE event_type = 'iuran_payment_reminder'
          AND name = 'Pengingat Pembayaran Iuran'
          AND is_default = true;
    ELSE
        DELETE FROM notification_templates
        WHERE event_type = 'iuran_payment_reminder'
          AND name = 'Pengingat Pembayaran Iuran';
    END IF;
END $$;

DROP INDEX IF EXISTS idx_notif_templates_is_default;
DROP INDEX IF EXISTS idx_notif_schedules_is_default;
ALTER TABLE notification_templates DROP COLUMN IF EXISTS is_default;
ALTER TABLE notification_schedules DROP COLUMN IF EXISTS is_default;
