-- 040_notification_templates.up.sql
-- Notification templates management for Urena.

CREATE TABLE IF NOT EXISTS notification_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id UUID NOT NULL REFERENCES rts(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    channel VARCHAR(20) NOT NULL DEFAULT 'push' CHECK (channel IN ('push', 'in_app')),
    title VARCHAR(200) NOT NULL,
    body TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    variables JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_rt_event_template_name UNIQUE (rt_id, event_type, name)
);

CREATE INDEX IF NOT EXISTS idx_notif_templates_rt_id ON notification_templates(rt_id);
CREATE INDEX IF NOT EXISTS idx_notif_templates_event_type ON notification_templates(event_type);
CREATE INDEX IF NOT EXISTS idx_notif_templates_active ON notification_templates(is_active);

