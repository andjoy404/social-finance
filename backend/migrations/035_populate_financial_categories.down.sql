-- 034_populate_financial_categories.down.sql
-- Hapus kategori baru yang ditambahkan migration 034 (keep Operasional RT dan Iuran Warga)

DELETE FROM financial_categories WHERE name IN (
    'Pemeliharaan Fasilitas',
    'Kebersihan',
    'Keamanan',
    'Konsumsi Rapat',
    'Perlengkapan & ATK',
    'Sumbangan & Sosial',
    'Iuran Paksa',
    'Iuran Sukarela',
    'Sumbangan Warga',
    'Denda'
);
