-- Add due_type and year columns to dues table for Multi-Bulan support
ALTER TABLE dues ADD COLUMN IF NOT EXISTS due_type text NOT NULL DEFAULT 'regular';
ALTER TABLE dues ADD COLUMN IF NOT EXISTS year integer;

-- Constraint for due_type
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'dues_due_type_check'
    ) THEN
        ALTER TABLE dues ADD CONSTRAINT dues_due_type_check 
            CHECK (due_type = ANY (ARRAY['regular'::text, 'voluntary'::text, 'multi_month'::text]));
    END IF;
END $$;

-- Backfill existing dues
UPDATE dues SET due_type = 'voluntary' WHERE is_voluntary = true;
UPDATE dues SET due_type = 'regular' WHERE is_voluntary = false AND due_type != 'multi_month';

-- Index on due_type
CREATE INDEX IF NOT EXISTS idx_dues_due_type ON dues (due_type);

