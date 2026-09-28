-- Migration 012 down: remove user_id from residents

ALTER TABLE residents DROP CONSTRAINT IF EXISTS residents_user_id_fkey;
DROP INDEX IF EXISTS uniq_residents_user_id;
ALTER TABLE residents DROP COLUMN IF EXISTS user_id;
