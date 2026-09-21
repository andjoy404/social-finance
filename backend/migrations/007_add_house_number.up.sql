-- 007_add_house_number.up.sql
--
-- Add first-class house_number field to households.

ALTER TABLE households
    ADD COLUMN house_number text;

COMMENT ON COLUMN households.house_number IS
    'House number identifier (e.g. U12/14, A-03). Not a street address.';
