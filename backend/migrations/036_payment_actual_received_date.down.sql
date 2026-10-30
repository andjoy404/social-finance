-- 036_payment_actual_received_date.down.sql
ALTER TABLE payments DROP COLUMN IF EXISTS actual_received_date;

