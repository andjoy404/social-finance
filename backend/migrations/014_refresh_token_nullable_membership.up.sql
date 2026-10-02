-- Migrate 014 up: allow system-only superadmin refresh tokens.
-- Membership_id becomes nullable so that users without a user_rt_memberships
-- row can still store refresh tokens keyed only to their user_id.

ALTER TABLE refresh_tokens
ALTER COLUMN membership_id DROP NOT NULL;
