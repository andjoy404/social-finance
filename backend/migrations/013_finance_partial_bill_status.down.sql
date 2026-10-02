-- 013_finance_partial_bill_status.down.sql
-- Revert bills.status to original three-state CHECK constraint.

ALTER TABLE bills DROP CONSTRAINT bills_status_check;

ALTER TABLE bills
    ADD CONSTRAINT bills_status_check
    CHECK (status IN ('unpaid', 'paid', 'cancelled'));
