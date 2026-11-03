-- 051_add_lain_lain_income_category.down.sql
-- Hapus kategori "Lain-Lain" yang ditambahkan migration 051

DELETE FROM financial_categories
WHERE name = 'Lain-Lain' AND type = 'income';
