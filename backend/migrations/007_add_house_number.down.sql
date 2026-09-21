-- 007_add_house_number.down.sql
--
-- Rollback: remove house_number column.

ALTER TABLE households
    DROP COLUMN IF EXISTS house_number;
