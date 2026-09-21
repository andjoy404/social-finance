-- 005_fix_rt_unique_indexes.down.sql
--
-- Phase 3: Rollback RT unique index fix.
--
-- Recreate the original full unique index.

CREATE UNIQUE INDEX IF NOT EXISTS idx_rts_rw_rt ON rts (rw, rt);
