-- 069_community_banner_dedup_and_idempotency
--
-- Fixes three production-hardening gaps from 068 without editing 068:
--   1. Concurrent/failed delete-then-insert banner syncs in 068 could leave
--      multiple active banner rows for the same (rt_id, source_type, source_id).
--   2. 068's system-template seed was not idempotent (ON CONFLICT DO NOTHING
--      had no matching unique constraint), so re-applying it duplicated
--      system banner templates.
--   3. Source-scoped and display-window query paths had no dedicated indexes.
--
-- Safety:
--   * No rows are deleted. Duplicate banners are DEACTIVATED (is_active = false)
--     keeping the most recently updated row per source.
--   * Duplicate system templates are DEACTIVATED the same way.
--   * Rollback (.down.sql) drops only the objects created here and cannot
--     restore the previously active duplicates' previous state — it only
--     re-marks them active again via a best-effort inverse.
-- Do NOT run this migration against production without a verified backup.

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. Deactivate duplicate ACTIVE banners per (rt_id, source_type, source_id).
--    Keep the most recently updated/created row. Non-source rows
--    (source_id IS NULL, e.g. manual banners) are untouched.
-- ---------------------------------------------------------------------------
WITH ranked AS (
    SELECT id,
           ROW_NUMBER() OVER (
               PARTITION BY rt_id, source_type, source_id
               ORDER BY updated_at DESC NULLS LAST, created_at DESC NULLS LAST, id DESC
           ) AS rn
    FROM banners
    WHERE source_id IS NOT NULL AND is_active = true
)
UPDATE banners b
SET is_active = false, updated_at = NOW()
FROM ranked r
WHERE b.id = r.id AND r.rn > 1;

-- At most one ACTIVE banner per (rt_id, source_type, source_id).
-- Used by the atomic upsert in the application (ON CONFLICT ... WHERE
-- source_id IS NOT NULL AND is_active = true).
CREATE UNIQUE INDEX IF NOT EXISTS banners_one_active_per_source_idx
    ON banners (rt_id, source_type, source_id)
    WHERE source_id IS NOT NULL AND is_active = true;

-- Supports DeleteBannerBySource / DeactivateBannerBySource across all rows.
CREATE INDEX IF NOT EXISTS banners_source_idx
    ON banners (rt_id, source_type, source_id);

-- Supports warga banner listing by RT + active flag + display window.
CREATE INDEX IF NOT EXISTS banners_rt_active_window_idx
    ON banners (rt_id, is_active, start_at, end_at);

-- Supports announcement display-window filtering for warga lists.
CREATE INDEX IF NOT EXISTS announcements_rt_status_window_idx
    ON announcements (rt_id, status, scheduled_start, scheduled_end);

-- ---------------------------------------------------------------------------
-- 2. Deactivate duplicate SYSTEM (rt_id IS NULL) banner templates per
--    (name, category), keeping the oldest canonical row. No deletion.
-- ---------------------------------------------------------------------------
WITH ranked_templates AS (
    SELECT id,
           ROW_NUMBER() OVER (
               PARTITION BY name, category
               ORDER BY created_at ASC NULLS LAST, id ASC
           ) AS rn
    FROM banner_templates
    WHERE rt_id IS NULL
)
UPDATE banner_templates t
SET is_active = false, updated_at = NOW()
FROM ranked_templates r
WHERE t.id = r.id AND r.rn > 1 AND t.is_active = true;

-- Idempotency anchor for system template seeds: at most one ACTIVE system
-- template per (name, category). Deactivated duplicates keep their rows
-- (they may be referenced by historical banners/snapshots).
CREATE UNIQUE INDEX IF NOT EXISTS banner_templates_system_unique_idx
    ON banner_templates (name, category)
    WHERE rt_id IS NULL AND is_active = true;

COMMIT;
