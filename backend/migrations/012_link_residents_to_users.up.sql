-- Migration 012: Link residents to users
--
-- Add user_id FK on residents table to explicitly link resident records
-- to user accounts. This enables position users (Pengurus) to have
-- resident records tied to their authenticated user identity.

-- =====================================================================
-- RESIDENTS: link to user account
-- =====================================================================

ALTER TABLE residents
    ADD COLUMN IF NOT EXISTS user_id uuid NULL
        REFERENCES users (id)
        ON DELETE SET NULL;

COMMENT ON COLUMN residents.user_id IS
    'Optional link to a users.id for position-based residents.';

-- One resident per user account (position residents only).
-- Allows legacy residents without user_id to coexist.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_residents_user_id
    ON residents (user_id)
    WHERE user_id IS NOT NULL;
