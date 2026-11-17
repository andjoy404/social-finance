-- Rollback for 069_community_banner_dedup_and_idempotency.
--
-- Drops the objects created by the up migration. It intentionally does NOT
-- re-activate the deduplicated templates/banners: reactivating them would
-- resurrect rows that were hidden for consistency reasons and, for banners,
-- would violate the (now dropped) uniqueness invariant. Historical rows are
-- preserved untouched; operators may re-activate specific rows manually
-- after review if needed.

BEGIN;

DROP INDEX IF EXISTS banner_templates_system_unique_idx;
DROP INDEX IF EXISTS announcements_rt_status_window_idx;
DROP INDEX IF EXISTS banners_rt_active_window_idx;
DROP INDEX IF EXISTS banners_source_idx;
DROP INDEX IF EXISTS banners_one_active_per_source_idx;

COMMIT;
