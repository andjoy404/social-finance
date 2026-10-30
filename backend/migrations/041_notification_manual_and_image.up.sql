-- 041_notification_manual_and_image.up.sql
-- Add image support and manual_push event type for notifications.

-- 1. Add image_url to notification_templates
ALTER TABLE notification_templates ADD COLUMN IF NOT EXISTS image_url TEXT NULL;

-- 2. Add image_url to notification_logs
ALTER TABLE notification_logs ADD COLUMN IF NOT EXISTS image_url TEXT NULL;

-- 3. Update check constraint on notification_templates if applicable
-- In 040_notification_templates, event_type was VARCHAR(50) without strict CHECK,
-- but we ensure index and validation support manual_push.
CREATE INDEX IF NOT EXISTS idx_notif_templates_image ON notification_templates(image_url) WHERE image_url IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_notification_logs_image ON notification_logs(image_url) WHERE image_url IS NOT NULL;

-- 4. Seed default manual_push template for existing active RTs
INSERT INTO notification_templates (id, rt_id, name, event_type, channel, title, body, is_active, variables, created_at, updated_at)
SELECT
    gen_random_uuid(),
    r.id,
    'Pengumuman Warga Default',
    'manual_push',
    'push',
    'Pengumuman Pengurus RT',
    'Pemberitahuan penting untuk seluruh warga.',
    true,
    '[]'::jsonb,
    NOW(),
    NOW()
FROM rts r
WHERE r.is_active = true
ON CONFLICT (rt_id, event_type, name) DO NOTHING;

