-- Migration 052: Add generic notification schedules + notification.manage permission
--
-- Creates notification_schedules table for managing automated notification
-- delivery schedules per RT (e.g. iuran payment reminder). A schedule binds
-- an event type to a template, a list of send days (1-31), a time, and a
-- timezone. Schedules are owned by a single RT and cannot cross RTs.
--
-- Also registers the granular permission "notification.manage" and grants it
-- to the Ketua RT position so that notification management (templates +
-- schedules) is controlled through the position_permissions RBAC table
-- instead of hardcoded jabatan checks.

-- =====================================================================
-- notification_schedules table
-- =====================================================================

CREATE TABLE IF NOT EXISTS notification_schedules (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id             UUID NOT NULL REFERENCES rts(id) ON DELETE CASCADE,
    event_type        VARCHAR(64) NOT NULL CHECK (char_length(event_type) > 0),
    template_id       UUID NOT NULL REFERENCES notification_templates(id) ON DELETE RESTRICT,
    name              VARCHAR(128) NOT NULL,
    is_active         BOOLEAN NOT NULL DEFAULT true,
    send_days         JSONB NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(send_days) = 'array'),
    send_hour         INT NOT NULL CHECK (send_hour >= 0 AND send_hour <= 23),
    send_minute       INT NOT NULL CHECK (send_minute >= 0 AND send_minute <= 59),
    timezone          VARCHAR(64) NOT NULL DEFAULT 'Asia/Jakarta',
    last_executed_at  TIMESTAMPTZ,
    last_executed_status VARCHAR(16),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by        UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_by        UUID REFERENCES users(id) ON DELETE SET NULL
);

COMMENT ON TABLE notification_schedules IS
    'Generic notification schedules for automated events (e.g. iuran payment reminder). One schedule may fire on multiple days per month.';
COMMENT ON COLUMN notification_schedules.send_days IS
    'Array of day-of-month integers (1-31). A day that does not exist in a given month simply does not fire that month (never shifted silently).';
COMMENT ON COLUMN notification_schedules.last_executed_status IS
    'Last execution outcome: success / failed / partial.';

CREATE INDEX IF NOT EXISTS idx_notification_schedules_rt ON notification_schedules(rt_id);
CREATE INDEX IF NOT EXISTS idx_notification_schedules_event ON notification_schedules(event_type);
CREATE INDEX IF NOT EXISTS idx_notification_schedules_active ON notification_schedules(is_active);

-- =====================================================================
-- Permission: notification.manage
-- =====================================================================

INSERT INTO permission_types (code, domain, operation) VALUES
    ('notification.manage', 'notification', 'manage')
ON CONFLICT (code) DO NOTHING;

-- Ketua RT (jabatan "ketua") receives notification.manage.
-- Super Admin always passes permission checks regardless of position.
INSERT INTO position_permissions (position, permission_id)
SELECT 'ketua', id FROM permission_types WHERE code = 'notification.manage'
ON CONFLICT (position, permission_id) DO NOTHING;

-- =====================================================================
-- Down
-- =====================================================================

-- DROP TABLE IF EXISTS notification_schedules;
-- DELETE FROM position_permissions WHERE permission_id = (
--     SELECT id FROM permission_types WHERE code = 'notification.manage'
-- );
-- DELETE FROM permission_types WHERE code = 'notification.manage';
