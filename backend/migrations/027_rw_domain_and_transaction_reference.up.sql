-- 027_rw_domain_and_transaction_reference.up.sql
-- Adds RW domain table (rws), links rts.rw_id -> rws.id, and adds system-generated reference_no to transactions.

CREATE TABLE IF NOT EXISTS rws (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    rw_number int NOT NULL CHECK (rw_number > 0),
    name text NOT NULL,
    head_name text,
    address text,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_rws_active
    ON rws (rw_number) WHERE is_active = true;

-- Backfill RW entities from existing RTs safely
INSERT INTO rws (rw_number, name, is_active)
SELECT DISTINCT r.rw, 'RW ' || LPAD(r.rw::text, 3, '0'), true
FROM rts r
WHERE r.rw > 0
  AND NOT EXISTS (
      SELECT 1 FROM rws w WHERE w.rw_number = r.rw AND w.is_active = true
  );

ALTER TABLE rts
    ADD COLUMN IF NOT EXISTS rw_id uuid REFERENCES rws(id) ON DELETE RESTRICT;

UPDATE rts r
SET rw_id = w.id
FROM rws w
WHERE r.rw_id IS NULL
  AND r.rw = w.rw_number
  AND w.is_active = true;

CREATE INDEX IF NOT EXISTS idx_rts_rw_id ON rts (rw_id);

-- Add reference_no to transactions for auto-generated KAS references
ALTER TABLE transactions
    ADD COLUMN IF NOT EXISTS reference_no text;

WITH numbered AS (
    SELECT t.id,
           CASE
               WHEN t.payment_id IS NOT NULL
                    AND NULLIF(p.payment_no, '') IS NOT NULL
                    AND ROW_NUMBER() OVER (PARTITION BY t.payment_id ORDER BY t.created_at, t.id) = 1
               THEN p.payment_no
               ELSE 'KAS-' || TO_CHAR(t.created_at AT TIME ZONE 'UTC', 'YYYYMMDD') || '-' ||
                    LPAD(ROW_NUMBER() OVER (
                        PARTITION BY TO_CHAR(t.created_at AT TIME ZONE 'UTC', 'YYYYMMDD')
                        ORDER BY t.created_at, t.id
                    )::text, 5, '0')
           END AS computed_ref
    FROM transactions t
    LEFT JOIN payments p ON p.id = t.payment_id
    WHERE t.reference_no IS NULL OR t.reference_no = ''
)
UPDATE transactions t
SET reference_no = n.computed_ref
FROM numbered n
WHERE t.id = n.id
  AND (t.reference_no IS NULL OR t.reference_no = '');

CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_reference_no_unique
    ON transactions (reference_no) WHERE reference_no IS NOT NULL;
