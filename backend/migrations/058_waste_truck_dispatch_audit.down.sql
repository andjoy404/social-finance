DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM notification_logs
        WHERE event_type = 'waste_truck_arrival'
          AND (sender_user_id IS NOT NULL OR failure_reason IS NOT NULL)
    ) THEN
        RAISE EXCEPTION 'Cannot rollback migration 058: waste-truck dispatch audit data exists. Export it before rollback.';
    END IF;
END $$;

DROP INDEX IF EXISTS idx_notification_logs_waste_truck_sender;

ALTER TABLE notification_logs
    DROP COLUMN IF EXISTS failure_reason,
    DROP COLUMN IF EXISTS sender_user_id;
