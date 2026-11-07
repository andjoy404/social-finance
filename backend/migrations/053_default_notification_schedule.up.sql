-- Migration 053: Seed default iuran payment reminder template + schedule per RT.
--
-- Idempotent: safe to run multiple times.
--   * A default active template is created per RT only if none exists for the
--     event type iuran_payment_reminder (ON CONFLICT on (rt_id,event_type,name)).
--     Default rows are flagged is_default = true.
--   * A default schedule is created per RT only if NO schedule already exists
--     for that RT + event type (so a Ketua RT's customised schedule is never
--     overwritten). Default: tanggal 2 dan 25 setiap bulan, pukul 08.00 WIB,
--     timezone Asia/Jakarta.
--   * Everything is scoped per RT (no global/cross-RT schedules).

-- 0. Ensure is_default columns exist before seeding default templates and schedules.
ALTER TABLE notification_templates
    ADD COLUMN IF NOT EXISTS is_default BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE notification_schedules
    ADD COLUMN IF NOT EXISTS is_default BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_notif_templates_is_default ON notification_templates(is_default);
CREATE INDEX IF NOT EXISTS idx_notif_schedules_is_default ON notification_schedules(is_default);

-- 1. Default reminder template per RT (idempotent).
INSERT INTO notification_templates (id, rt_id, name, event_type, channel, title, body, is_active, variables, is_default, created_at, updated_at)
SELECT
    gen_random_uuid(),
    r.id,
    'Pengingat Pembayaran Iuran',
    'iuran_payment_reminder',
    'push',
    'Pengingat Pembayaran Iuran',
    'Anda masih memiliki tagihan iuran yang belum lunas. Segera lakukan pembayaran sebelum jatuh tempo.',
    true,
    '["bill_id","period","amount","due_date","deep_link"]'::jsonb,
    true,
    NOW(),
    NOW()
FROM rts r
WHERE r.is_active = true
ON CONFLICT (rt_id, event_type, name) DO NOTHING;

-- Ensure default rows already seeded before migration 054 are flagged.
UPDATE notification_templates
SET is_default = true
WHERE event_type = 'iuran_payment_reminder'
  AND name = 'Pengingat Pembayaran Iuran'
  AND is_default = false;

-- 2. Unique guard so a schedule is seeded at most once per RT + event type.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'uq_notif_schedules_rt_event_name'
          AND conrelid = 'notification_schedules'::regclass
    ) THEN
        ALTER TABLE notification_schedules
            ADD CONSTRAINT uq_notif_schedules_rt_event_name
            UNIQUE (rt_id, event_type, name);
    END IF;
END $$;

-- 3. Default schedule per RT (idempotent: only when no schedule exists for
--    this RT + event type, so Ketua RT customisations are preserved).
INSERT INTO notification_schedules (
    id, rt_id, event_type, template_id, name, is_active,
    send_days, send_hour, send_minute, timezone, created_at, updated_at, created_by, updated_by, is_default
)
SELECT
    gen_random_uuid(),
    r.id,
    'iuran_payment_reminder',
    t.id,
    'Pengingat Pembayaran Iuran Bulanan',
    true,
    '[2, 25]'::jsonb,
    8,
    0,
    'Asia/Jakarta',
    NOW(),
    NOW(),
    NULL,
    NULL,
    true
FROM rts r
JOIN notification_templates t
  ON t.rt_id = r.id
 AND t.event_type = 'iuran_payment_reminder'
 AND t.is_default = true
 AND t.is_active = true
WHERE r.is_active = true
  AND NOT EXISTS (
      SELECT 1 FROM notification_schedules s
      WHERE s.rt_id = r.id AND s.event_type = 'iuran_payment_reminder'
  )
ON CONFLICT (rt_id, event_type, name) DO NOTHING;

-- Ensure default rows already seeded before migration 054 are flagged.
UPDATE notification_schedules
SET is_default = true
WHERE event_type = 'iuran_payment_reminder'
  AND name = 'Pengingat Pembayaran Iuran Bulanan'
  AND is_default = false;
