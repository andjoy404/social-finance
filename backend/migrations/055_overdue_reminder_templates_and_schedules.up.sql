-- Migration 055: Seed default notification templates and schedules for
-- iuran_upcoming_due_3_months and iuran_overdue_3_months for every active RT.
-- Idempotent: existing templates or schedules are never overwritten.

-- 1. Seed default template for iuran_upcoming_due_3_months
INSERT INTO notification_templates (id, rt_id, name, event_type, channel, title, body, is_active, variables, is_default, created_at, updated_at)
SELECT
    gen_random_uuid(),
    r.id,
    'Pengingat Pembayaran Iuran (3 Bulan Sebelum Jatuh Tempo)',
    'iuran_upcoming_due_3_months',
    'push',
    'Pengingat Pembayaran Iuran',
    'Halo {{nama}}, kami mengingatkan bahwa tagihan iuran Anda akan jatuh tempo dalam waktu 3 bulan. Silakan periksa tagihan Anda dan lakukan pembayaran sesuai ketentuan. Terima kasih.',
    true,
    '["nama","bill_id","period","amount","total_outstanding","bill_count","due_date","deep_link"]'::jsonb,
    true,
    NOW(),
    NOW()
FROM rts r
WHERE r.is_active = true
  AND NOT EXISTS (
    SELECT 1 FROM notification_templates nt
    WHERE nt.rt_id = r.id AND nt.event_type = 'iuran_upcoming_due_3_months'
  )
ON CONFLICT (rt_id, event_type, name) DO NOTHING;

-- 2. Seed default schedule for iuran_upcoming_due_3_months
-- Evaluates daily at 08:00 Asia/Jakarta against each bill's relative (due_date - 3 calendar months) trigger.
INSERT INTO notification_schedules (
    id, rt_id, event_type, template_id, name, is_active,
    send_days, send_hour, send_minute, timezone, created_at, updated_at, created_by, updated_by, is_default
)
SELECT
    gen_random_uuid(),
    r.id,
    'iuran_upcoming_due_3_months',
    nt.id,
    'Pengingat Pembayaran Iuran (3 Bulan Sebelum Jatuh Tempo)',
    true,
    '[1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31]'::jsonb,
    8,
    0,
    'Asia/Jakarta',
    NOW(),
    NOW(),
    NULL,
    NULL,
    true
FROM rts r
JOIN LATERAL (
    SELECT t.id
    FROM notification_templates t
    WHERE t.rt_id = r.id
      AND t.event_type = 'iuran_upcoming_due_3_months'
      AND t.is_active = true
    ORDER BY t.is_default DESC, t.created_at ASC
    LIMIT 1
) nt ON true
WHERE r.is_active = true
  AND NOT EXISTS (
    SELECT 1 FROM notification_schedules ns
    WHERE ns.rt_id = r.id AND ns.event_type = 'iuran_upcoming_due_3_months'
  )
ON CONFLICT (rt_id, event_type, name) DO NOTHING;

-- 3. Seed default template for iuran_overdue_3_months
INSERT INTO notification_templates (id, rt_id, name, event_type, channel, title, body, is_active, variables, is_default, created_at, updated_at)
SELECT
    gen_random_uuid(),
    r.id,
    'Pengingat Tunggakan Iuran (3 Bulan+)',
    'iuran_overdue_3_months',
    'push',
    'Pengingat Tunggakan Iuran',
    'Halo {{nama}}, terdapat tagihan iuran Anda yang telah menunggak selama 3 bulan atau lebih. Mohon segera lakukan pembayaran untuk menyelesaikan kewajiban iuran Anda. Terima kasih.',
    true,
    '["nama","bill_id","period","amount","total_outstanding","bill_count","due_date","deep_link"]'::jsonb,
    true,
    NOW(),
    NOW()
FROM rts r
WHERE r.is_active = true
  AND NOT EXISTS (
    SELECT 1 FROM notification_templates nt
    WHERE nt.rt_id = r.id AND nt.event_type = 'iuran_overdue_3_months'
  )
ON CONFLICT (rt_id, event_type, name) DO NOTHING;

-- 4. Seed default schedule for iuran_overdue_3_months
-- Evaluates daily at 08:00 Asia/Jakarta against each bill's (due_date + 3 calendar months) threshold with monthly deduplication.
INSERT INTO notification_schedules (
    id, rt_id, event_type, template_id, name, is_active,
    send_days, send_hour, send_minute, timezone, created_at, updated_at, created_by, updated_by, is_default
)
SELECT
    gen_random_uuid(),
    r.id,
    'iuran_overdue_3_months',
    nt.id,
    'Pengingat Tunggakan Iuran Bulanan (3 Bulan+)',
    true,
    '[1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31]'::jsonb,
    8,
    0,
    'Asia/Jakarta',
    NOW(),
    NOW(),
    NULL,
    NULL,
    true
FROM rts r
JOIN LATERAL (
    SELECT t.id
    FROM notification_templates t
    WHERE t.rt_id = r.id
      AND t.event_type = 'iuran_overdue_3_months'
      AND t.is_active = true
    ORDER BY t.is_default DESC, t.created_at ASC
    LIMIT 1
) nt ON true
WHERE r.is_active = true
  AND NOT EXISTS (
    SELECT 1 FROM notification_schedules ns
    WHERE ns.rt_id = r.id AND ns.event_type = 'iuran_overdue_3_months'
  )
ON CONFLICT (rt_id, event_type, name) DO NOTHING;
