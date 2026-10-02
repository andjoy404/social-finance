-- Migrate 014 down: restore NOT NULL constraint.
-- WARNING: This will fail if any rows currently have NULL membership_id.
-- Ensure the database is clean before running.

ALTER TABLE refresh_tokens
ALTER COLUMN membership_id SET NOT NULL;
