-- Migration 066: Normalize phone numbers to local format starting with 08...
-- Standardizes stored phones across users and residents, eliminating +62 / 62 prefixes.

UPDATE users
SET phone = '0' || regexp_replace(phone, '^(\+620|\+62|620|62)', '')
WHERE phone ~ '^(\+62|62)';

UPDATE residents
SET phone = '0' || regexp_replace(phone, '^(\+620|\+62|620|62)', '')
WHERE phone IS NOT NULL AND phone ~ '^(\+62|62)';

