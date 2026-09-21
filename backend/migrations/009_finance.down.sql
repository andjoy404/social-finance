-- 009_finance.down.sql
-- Revert Finance Schema

DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS idempotency_keys;
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS bills;
DROP TABLE IF EXISTS dues;
DROP TABLE IF EXISTS financial_categories;

