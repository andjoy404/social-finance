-- 001_initial.up.sql
--
-- This migration creates the minimal tables needed to verify the migration
-- mechanism works. Only the `rts` table is included; all other tables will
-- be added in later phases.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS rts (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name         text    NOT NULL,
    rw           int     NOT NULL,
    rt           text    NOT NULL,
    address      text,
    head_name    text,
    is_active    boolean NOT NULL DEFAULT true,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_rts_rw_rt ON rts (rw, rt);
CREATE UNIQUE INDEX uniq_rts_active ON rts (rw, rt) WHERE is_active = true;
