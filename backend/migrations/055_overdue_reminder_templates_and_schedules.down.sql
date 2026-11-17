-- Migration 055: Down
-- Only delete rows that were seeded as defaults (is_default = true) AND have
-- not been customized by pengurus. Templates still referenced by any remaining
-- schedule are also preserved.

DELETE FROM notification_schedules
WHERE event_type = 'iuran_upcoming_due_3_months'
  AND name = 'Pengingat Pembayaran Iuran (3 Bulan Sebelum Jatuh Tempo)'
  AND is_default = true
  AND updated_by IS NULL
  AND is_active = true
  AND send_days = '[1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31]'::jsonb
  AND send_hour = 8
  AND send_minute = 0
  AND timezone = 'Asia/Jakarta';

DELETE FROM notification_templates
WHERE event_type = 'iuran_upcoming_due_3_months'
  AND name = 'Pengingat Pembayaran Iuran (3 Bulan Sebelum Jatuh Tempo)'
  AND title = 'Pengingat Pembayaran Iuran'
  AND body = 'Halo {{nama}}, kami mengingatkan bahwa tagihan iuran Anda akan jatuh tempo dalam waktu 3 bulan. Silakan periksa tagihan Anda dan lakukan pembayaran sesuai ketentuan. Terima kasih.'
  AND is_default = true
  AND is_active = true
  AND image_url IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM notification_schedules ns
      WHERE ns.template_id = notification_templates.id
  );

DELETE FROM notification_schedules
WHERE event_type = 'iuran_overdue_3_months'
  AND name = 'Pengingat Tunggakan Iuran Bulanan (3 Bulan+)'
  AND is_default = true
  AND updated_by IS NULL
  AND is_active = true
  AND send_days = '[1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31]'::jsonb
  AND send_hour = 8
  AND send_minute = 0
  AND timezone = 'Asia/Jakarta';

DELETE FROM notification_templates
WHERE event_type = 'iuran_overdue_3_months'
  AND name = 'Pengingat Tunggakan Iuran (3 Bulan+)'
  AND title = 'Pengingat Tunggakan Iuran'
  AND body = 'Halo {{nama}}, terdapat tagihan iuran Anda yang telah menunggak selama 3 bulan atau lebih. Mohon segera lakukan pembayaran untuk menyelesaikan kewajiban iuran Anda. Terima kasih.'
  AND is_default = true
  AND is_active = true
  AND image_url IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM notification_schedules ns
      WHERE ns.template_id = notification_templates.id
  );
