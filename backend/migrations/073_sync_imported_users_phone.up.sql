-- Migration 073: Backfill users.phone from residents.phone
BEGIN;

UPDATE users u
SET phone = r.phone,
    updated_at = NOW()
FROM residents r
WHERE r.user_id = u.id
  AND (u.phone IS NULL OR btrim(u.phone) = '')
  AND (r.phone IS NOT NULL AND btrim(r.phone) != '');

COMMIT;

