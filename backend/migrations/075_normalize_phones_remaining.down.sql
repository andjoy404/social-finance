-- Migration 075: Down
-- No-op. Normalising a phone is idempotent; reverting would re-create
-- ambiguity (multiple accounts matching the same canonical form).
BEGIN;
COMMIT;
