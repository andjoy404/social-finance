-- 090 UP: Add photo_key column to security_guest_books for structured photo storage

ALTER TABLE security_guest_books
    ADD COLUMN photo_key TEXT;
