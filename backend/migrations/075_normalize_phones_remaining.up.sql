-- Migration 075: Normalize remaining non-canonical phone numbers
-- Migration 066 normalized +62/62 to local 08 format but was applied only once.
-- This migration repeats the same normalization for any users/residents whose
-- phone still carries +62/62 prefixes (e.g. rows written by older fixtures or
-- imported data that predates migration 066's run, or rows re-created after it).
--
-- Safety policy:
--   - Rows whose normalized form would collide with an existing account are
--     SKIPPED rather than merged. They are left untouched for manual
--     reconciliation by a database administrator.
--   - No user is merged into another user automatically.
--   - The migration is idempotent; rows already canonical are no-ops.
--
-- Down migration: no-op. reverting would re-introduce ambiguity.

BEGIN;

-- Users: normalise +62 / 62 prefixes to 08 (skip ambiguous rows).
UPDATE users
SET phone = '0' || regexp_replace(phone, '^(\+620|\+62|620|62)', '')
WHERE phone ~ '^(\+62|62)'
  AND NOT EXISTS (
    SELECT 1 FROM users u2
    WHERE u2.id <> users.id
      AND u2.phone = ('0' || regexp_replace(users.phone, '^(\+620|\+62|620|62)', ''))
  );

-- Residents: same rule.
UPDATE residents
SET phone = '0' || regexp_replace(phone, '^(\+620|\+62|620|62)', '')
WHERE phone IS NOT NULL
  AND phone ~ '^(\+62|62)'
  AND NOT EXISTS (
    SELECT 1 FROM residents r2
    WHERE r2.id <> residents.id
      AND r2.phone = ('0' || regexp_replace(residents.phone, '^(\+620|\+62|620|62)', ''))
  );

COMMIT;
