-- 028_communities_rw_scope.down.sql
-- Development/test rollback definition only. Never use as production recovery.

DROP TRIGGER IF EXISTS trg_rws_immutable_used_number ON rws;
DROP FUNCTION IF EXISTS prevent_used_rw_number_change();
DROP TRIGGER IF EXISTS trg_rts_rw_projection ON rts;
DROP FUNCTION IF EXISTS enforce_rt_rw_projection();

ALTER TABLE rts
    DROP CONSTRAINT IF EXISTS rts_rw_community_fkey;

DROP INDEX IF EXISTS rts_id_community_unique;
DROP INDEX IF EXISTS rws_id_community_unique;
DROP INDEX IF EXISTS idx_rts_community_id;
DROP INDEX IF EXISTS idx_rws_community_id;
DROP INDEX IF EXISTS rws_community_number_unique;
DROP INDEX IF EXISTS communities_code_unique;

ALTER TABLE rts DROP COLUMN IF EXISTS community_id;
ALTER TABLE rws DROP COLUMN IF EXISTS community_id;
DROP TABLE IF EXISTS communities;
