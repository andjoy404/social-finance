-- Migration 049: Add is_obligatory to bills table
ALTER TABLE bills ADD COLUMN IF NOT EXISTS is_obligatory boolean NOT NULL DEFAULT true;

-- Index for FIFO queue lookup and filtering obligatory bills
CREATE INDEX IF NOT EXISTS idx_bills_occ_due_obligatory ON bills (household_occupancy_id, due_id, is_obligatory);

-- Backfill existing bills:
-- For household occupancies that have a known start_date (move-in date),
-- any unpaid bill whose period is strictly earlier than the start_date's month is marked as non-obligatory.
UPDATE bills b
SET is_obligatory = false
FROM household_occupancies ho
WHERE b.household_occupancy_id = ho.id
  AND ho.start_date IS NOT NULL
  AND b.period < to_char(ho.start_date, 'YYYY-MM')
  AND b.status = 'unpaid';

-- For household occupancies that have a known end_date (move-out date),
-- any unpaid bill whose period is strictly later than the end_date's month is marked as non-obligatory.
UPDATE bills b
SET is_obligatory = false
FROM household_occupancies ho
WHERE b.household_occupancy_id = ho.id
  AND ho.end_date IS NOT NULL
  AND b.period > to_char(ho.end_date, 'YYYY-MM')
  AND b.status = 'unpaid';

