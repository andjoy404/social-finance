-- 051_add_lain_lain_income_category.up.sql
-- Tambahkan kategori pemasukan "Lain-Lain" untuk semua RT yang belum memiliki

INSERT INTO financial_categories (id, rt_id, name, type, is_active)
SELECT gen_random_uuid(), r.id, 'Lain-Lain', 'income', true
FROM rts r
WHERE NOT EXISTS (
    SELECT 1 FROM financial_categories fc
    WHERE fc.rt_id = r.id AND fc.name = 'Lain-Lain' AND fc.type = 'income'
);
