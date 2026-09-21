-- 006_add_household_occupancy_status.down.sql
--
-- Phase 3: Rollback household occupancy status column.

ALTER TABLE households DROP COLUMN IF EXISTS occupancy_status;
