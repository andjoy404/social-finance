-- 013_finance_partial_bill_status.up.sql
-- Add 'partial' to bills.status CHECK constraint to support partial payments (DR-04).

ALTER TABLE bills DROP CONSTRAINT bills_status_check;

ALTER TABLE bills
    ADD CONSTRAINT bills_status_check
    CHECK (status IN ('unpaid', 'partial', 'paid', 'cancelled'));
