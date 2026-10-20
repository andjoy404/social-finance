-- 028_communities_rw_scope.up.sql
-- RW.8 source foundation: add operational communities and scoped RW/RT relations.
-- This migration intentionally leaves community_id nullable during the mapping phase.
-- It must be followed by a reviewed backfill before NOT NULL enforcement.

CREATE TABLE IF NOT EXISTS communities (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code text NOT NULL,
    name text NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT communities_code_format CHECK (code ~ '^[a-z0-9]+([a-z0-9-]*[a-z0-9])?$'),
    CONSTRAINT communities_code_length CHECK (char_length(code) BETWEEN 1 AND 63)
);

CREATE UNIQUE INDEX IF NOT EXISTS communities_code_unique
    ON communities (code);

ALTER TABLE rws
    ADD COLUMN IF NOT EXISTS community_id uuid REFERENCES communities(id) ON DELETE RESTRICT;

ALTER TABLE rts
    ADD COLUMN IF NOT EXISTS community_id uuid REFERENCES communities(id) ON DELETE RESTRICT;

-- Supporting keys for the composite RT -> RW relationship.
CREATE UNIQUE INDEX IF NOT EXISTS rws_id_community_unique
    ON rws (id, community_id);

CREATE UNIQUE INDEX IF NOT EXISTS rts_id_community_unique
    ON rts (id, community_id);

-- Migration 027 enforced rw_number globally. Remove that old scope before
-- applying the community-scoped uniqueness rule. Existing rows remain valid
-- while community_id is NULL during the reviewed mapping phase.
DROP INDEX IF EXISTS uniq_rws_active;

CREATE UNIQUE INDEX IF NOT EXISTS rws_community_number_unique
    ON rws (community_id, rw_number);

CREATE INDEX IF NOT EXISTS idx_rws_community_id
    ON rws (community_id);

CREATE INDEX IF NOT EXISTS idx_rts_community_id
    ON rts (community_id);

-- A nullable composite FK permits standalone RTs while preventing an attached
-- RT from referencing an RW belonging to another community.
ALTER TABLE rts
    DROP CONSTRAINT IF EXISTS rts_rw_community_fkey;

ALTER TABLE rts
    ADD CONSTRAINT rts_rw_community_fkey
    FOREIGN KEY (rw_id, community_id)
    REFERENCES rws (id, community_id)
    DEFERRABLE INITIALLY IMMEDIATE;

CREATE OR REPLACE FUNCTION enforce_rt_rw_projection()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    target_rw_number integer;
    target_community uuid;
    target_active boolean;
BEGIN
    IF NEW.rw_id IS NULL THEN
        -- Legacy rows remain untouched until reviewed mapping. New canonical
        -- standalone writes must use rw = 0; final enforcement is deferred.
        RETURN NEW;
    END IF;

    SELECT rw_number, community_id, is_active
      INTO target_rw_number, target_community, target_active
      FROM rws
     WHERE id = NEW.rw_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'rw_id % does not reference an existing RW', NEW.rw_id
            USING ERRCODE = '23503';
    END IF;

    IF NEW.community_id IS NOT NULL AND target_community IS DISTINCT FROM NEW.community_id THEN
        RAISE EXCEPTION 'RT community_id must match RW community_id'
            USING ERRCODE = '23514';
    END IF;

    IF NOT target_active THEN
        RAISE EXCEPTION 'inactive RW cannot receive a new RT assignment'
            USING ERRCODE = '23514';
    END IF;

    NEW.rw := target_rw_number;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_rts_rw_projection ON rts;
CREATE TRIGGER trg_rts_rw_projection
BEFORE INSERT OR UPDATE OF rw_id, community_id, rw ON rts
FOR EACH ROW
EXECUTE FUNCTION enforce_rt_rw_projection();

-- RW number changes are intentionally blocked once an RW has RT history.
-- This preserves the identity represented by rws.id and the legacy projection.
CREATE OR REPLACE FUNCTION prevent_used_rw_number_change()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.rw_number IS DISTINCT FROM OLD.rw_number
       AND EXISTS (SELECT 1 FROM rts WHERE rw_id = OLD.id)
    THEN
        RAISE EXCEPTION 'rw_number cannot change after RT assignment'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_rws_immutable_used_number ON rws;
CREATE TRIGGER trg_rws_immutable_used_number
BEFORE UPDATE OF rw_number ON rws
FOR EACH ROW
EXECUTE FUNCTION prevent_used_rw_number_change();
