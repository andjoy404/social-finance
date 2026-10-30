-- 034_populate_financial_categories.up.sql
-- Populate financial_categories dengan daftar kategori pemasukan dan pengeluaran yang lengkap
-- Setiap RT akan memiliki set kategori yang sama (income + expense)

-- Hapus duplikat lama (keep lowest id per rt_id+name combo)
WITH to_delete AS (
    SELECT id
    FROM financial_categories
    WHERE id NOT IN (
        SELECT DISTINCT ON (rt_id, name) id
        FROM financial_categories
        ORDER BY rt_id, name, id ASC
    )
    AND name IN ('Operasional RT', 'Iuran Warga')
)
DELETE FROM financial_categories WHERE id IN (SELECT id FROM to_delete);

-- Tambahkan kategori pengeluaran jika belum ada
INSERT INTO financial_categories (id, rt_id, name, type, is_active)
SELECT gen_random_uuid(), r.id, 'Pemeliharaan Fasilitas', 'expense', true
FROM rts r
WHERE NOT EXISTS (
    SELECT 1 FROM financial_categories fc
    WHERE fc.rt_id = r.id AND fc.name = 'Pemeliharaan Fasilitas' AND fc.type = 'expense'
)
UNION ALL
SELECT gen_random_uuid(), r.id, 'Kebersihan', 'expense', true
FROM rts r
WHERE NOT EXISTS (
    SELECT 1 FROM financial_categories fc
    WHERE fc.rt_id = r.id AND fc.name = 'Kebersihan' AND fc.type = 'expense'
)
UNION ALL
SELECT gen_random_uuid(), r.id, 'Keamanan', 'expense', true
FROM rts r
WHERE NOT EXISTS (
    SELECT 1 FROM financial_categories fc
    WHERE fc.rt_id = r.id AND fc.name = 'Keamanan' AND fc.type = 'expense'
)
UNION ALL
SELECT gen_random_uuid(), r.id, 'Konsumsi Rapat', 'expense', true
FROM rts r
WHERE NOT EXISTS (
    SELECT 1 FROM financial_categories fc
    WHERE fc.rt_id = r.id AND fc.name = 'Konsumsi Rapat' AND fc.type = 'expense'
)
UNION ALL
SELECT gen_random_uuid(), r.id, 'Perlengkapan & ATK', 'expense', true
FROM rts r
WHERE NOT EXISTS (
    SELECT 1 FROM financial_categories fc
    WHERE fc.rt_id = r.id AND fc.name = 'Perlengkapan & ATK' AND fc.type = 'expense'
)
UNION ALL
SELECT gen_random_uuid(), r.id, 'Sumbangan & Sosial', 'expense', true
FROM rts r
WHERE NOT EXISTS (
    SELECT 1 FROM financial_categories fc
    WHERE fc.rt_id = r.id AND fc.name = 'Sumbangan & Sosial' AND fc.type = 'expense'
)
UNION ALL
SELECT gen_random_uuid(), r.id, 'Iuran Paksa', 'expense', true
FROM rts r
WHERE NOT EXISTS (
    SELECT 1 FROM financial_categories fc
    WHERE fc.rt_id = r.id AND fc.name = 'Iuran Paksa' AND fc.type = 'expense'
);

-- Tambahkan kategori pemasukan jika belum ada
INSERT INTO financial_categories (id, rt_id, name, type, is_active)
SELECT gen_random_uuid(), r.id, 'Iuran Warga', 'income', true
FROM rts r
WHERE NOT EXISTS (
    SELECT 1 FROM financial_categories fc
    WHERE fc.rt_id = r.id AND fc.name = 'Iuran Warga' AND fc.type = 'income'
)
UNION ALL
SELECT gen_random_uuid(), r.id, 'Iuran Sukarela', 'income', true
FROM rts r
WHERE NOT EXISTS (
    SELECT 1 FROM financial_categories fc
    WHERE fc.rt_id = r.id AND fc.name = 'Iuran Sukarela' AND fc.type = 'income'
)
UNION ALL
SELECT gen_random_uuid(), r.id, 'Sumbangan Warga', 'income', true
FROM rts r
WHERE NOT EXISTS (
    SELECT 1 FROM financial_categories fc
    WHERE fc.rt_id = r.id AND fc.name = 'Sumbangan Warga' AND fc.type = 'income'
)
UNION ALL
SELECT gen_random_uuid(), r.id, 'Denda', 'income', true
FROM rts r
WHERE NOT EXISTS (
    SELECT 1 FROM financial_categories fc
    WHERE fc.rt_id = r.id AND fc.name = 'Denda' AND fc.type = 'income'
);
