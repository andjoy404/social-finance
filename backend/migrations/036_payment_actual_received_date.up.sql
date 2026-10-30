-- 036_payment_actual_received_date.up.sql
-- Add explicit actual_received_date column to payments table.
-- This records the actual calendar date when the funds were received by RT (confirmed by treasurer/officer).
-- Transactions generated from payments use this date for occurred_at in the Asia/Jakarta timezone.

ALTER TABLE payments ADD COLUMN actual_received_date DATE NULL;

COMMENT ON COLUMN payments.actual_received_date IS 'Actual date when the money was received by RT (confirmed by treasurer/officer). Used as occurred_at for KAS transactions.';

