-- 025_update_position_permissions_and_kas_method.up.sql
-- Add payment_method to transactions and grant permissions to sekretaris and bendahara.

ALTER TABLE transactions 
ADD COLUMN IF NOT EXISTS payment_method text 
CHECK (payment_method IS NULL OR payment_method = ANY (ARRAY['CASH'::text, 'TRANSFER'::text])) 
DEFAULT 'CASH';

-- Backfill existing transactions linked to payments with payment.method
UPDATE transactions t
SET payment_method = p.method
FROM payments p
WHERE t.payment_id = p.id AND p.method IN ('CASH', 'TRANSFER');

-- Position Permissions: grant finance and settings to sekretaris
INSERT INTO position_permissions (position, permission_id)
SELECT 'sekretaris', id FROM permission_types 
WHERE code IN ('finance.read', 'finance.create', 'rt.settings.read')
ON CONFLICT (position, permission_id) DO NOTHING;

-- Position Permissions: grant warga export and import to bendahara
INSERT INTO position_permissions (position, permission_id)
SELECT 'bendahara', id FROM permission_types 
WHERE code IN ('warga.export', 'warga.import')
ON CONFLICT (position, permission_id) DO NOTHING;

