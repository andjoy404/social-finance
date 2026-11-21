-- 090 DOWN: Remove photo_key column from security_guest_books

ALTER TABLE security_guest_books
    DROP COLUMN IF EXISTS photo_key;
