-- Preserve the identity and outcome of each waste-truck dispatch separately
-- from the per-recipient notification/read-receipt fields.
ALTER TABLE notification_logs
    ADD COLUMN IF NOT EXISTS sender_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS failure_reason TEXT;

UPDATE notification_logs
SET sender_user_id = user_id
WHERE event_type = 'waste_truck_arrival'
  AND sender_user_id IS NULL
  AND user_id IS NOT NULL;

UPDATE notification_logs
SET status = 'UNKNOWN',
    failure_reason = 'Delivery result is unavailable for this historical dispatch'
WHERE event_type = 'waste_truck_arrival'
  AND sender_user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_notification_logs_waste_truck_sender
    ON notification_logs (rt_id, sender_user_id, created_at DESC)
    WHERE event_type = 'waste_truck_arrival';
