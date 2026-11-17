-- Migration 080: Permanent system default announcement — "Pengingat Pembayaran Iuran Warga"
--
-- Adds an is_permanent marker on announcements and seeds one permanent row per
-- active RT. The permanent entry can be renamed/reworded and toggled between
-- active/archived (≈ aktif/tidak aktif) but cannot be deleted or removed from
-- the list. It is published/republished through the normal publish flow so the
-- home banner behaves identically to other announcement banners.
--
-- Idempotent: the partial unique index guards reruns.

BEGIN;

ALTER TABLE announcements
    ADD COLUMN IF NOT EXISTS is_permanent BOOLEAN NOT NULL DEFAULT false;

CREATE UNIQUE INDEX IF NOT EXISTS announcements_permanent_one_per_rt
    ON announcements (rt_id)
    WHERE is_permanent = true;

INSERT INTO announcements (
    rt_id,
    title,
    body,
    status,
    priority,
    display_order,
    is_permanent,
    published_at,
    created_at,
    updated_at
)
SELECT r.id,
       'Pengingat Pembayaran Iuran Warga',
       'Jangan lupa melakukan pembayaran Iuran/IPL sesuai periode berjalan melalui menu Keuangan.',
       'active',
       3,
       0,
       true,
       NOW(),
       NOW(),
       NOW()
FROM rts r
WHERE r.is_active = true
ON CONFLICT (rt_id) WHERE is_permanent = true DO NOTHING;

COMMIT;
