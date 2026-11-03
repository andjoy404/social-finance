-- Migration 049 Down: Revert is_obligatory from bills table
DROP INDEX IF EXISTS idx_bills_occ_due_obligatory;
ALTER TABLE bills DROP COLUMN IF EXISTS is_obligatory;

