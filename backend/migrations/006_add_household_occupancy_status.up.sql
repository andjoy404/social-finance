-- 006_add_household_occupancy_status.up.sql
--
-- Phase 3: Add household occupancy status (OWNER / TENANT).
--
-- The column is nullable so existing rows remain UNCLASSIFIED until
-- the Household API/domain integration assigns a value.

ALTER TABLE households
    ADD COLUMN occupancy_status text CHECK (
        occupancy_status IS NULL
        OR occupancy_status IN ('OWNER', 'TENANT')
    );
