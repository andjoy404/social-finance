-- 005_fix_rt_unique_indexes.up.sql
--
-- Phase 3: Fix RT unique index constraint.
--
-- The `idx_rts_rw_rt` full unique index blocked updating RT rw/rt codes
-- even when changing a deactivated RT. Only the partial index
-- `uniq_rts_active` (WHERE is_active = true) is needed to enforce
-- unique (rw, rt) among active RTs.

DROP INDEX IF EXISTS idx_rts_rw_rt;
