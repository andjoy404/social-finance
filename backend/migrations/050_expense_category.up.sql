ALTER TABLE transactions ADD COLUMN IF NOT EXISTS expense_category text;
ALTER TABLE rw_transactions ADD COLUMN IF NOT EXISTS expense_category text;

