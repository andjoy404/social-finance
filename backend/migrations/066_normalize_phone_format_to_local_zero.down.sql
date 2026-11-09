-- Revert 066: Re-add +62 prefix to standard local phones
UPDATE users
SET phone = '+62' || SUBSTRING(phone FROM 2)
WHERE phone ~ '^08';

UPDATE residents
SET phone = '+62' || SUBSTRING(phone FROM 2)
WHERE phone IS NOT NULL AND phone ~ '^08';

