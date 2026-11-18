-- Migration 087 (DOWN):
DROP TABLE IF EXISTS security_read_receipts;
ALTER TABLE patrol_photos DROP COLUMN IF EXISTS template_name;
ALTER TABLE patrol_photos DROP COLUMN IF EXISTS location_label;
