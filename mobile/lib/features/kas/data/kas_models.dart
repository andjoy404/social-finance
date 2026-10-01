import 'package:flutter/material.dart';

import 'package:social_finance/core/theme/app_colors.dart';

/// Kategori untuk transaksi KAS.
enum KasCategory {
  iuranWarga('Iuran Warga'),
  iuranPaksa('Iuran Paksa'),
  perlengkapan('Perlengkapan'),
  konsumsi('Konsumsi'),
  perbaikan('Perbaikan'),
  kebersihan('Kebersihan'),
  keamanan('Keamanan'),
  lainLain('Lainnya');

  final String label;
  const KasCategory(this.label);
}

/// Jenis transaksi KAS.
enum KasJenis {
  masuk('masuk'),
  keluar('keluar');

  final String value;
  const KasJenis(this.value);

  static KasJenis fromString(String value) {
    return KasJenis.values.firstWhere(
      (e) => e.value == value,
      orElse: () => KasJenis.masuk,
    );
  }
}

/// Model transaksi KAS.
class KasTransaction {
  final String id;
  final DateTime tanggal;
  final KasJenis jenis;
  final KasCategory kategori;
  final String keterangan;
  final String? referensi;
  final int nominal;
  final int saldo;

  const KasTransaction({
    required this.id,
    required this.tanggal,
    required this.jenis,
    required this.kategori,
    required this.keterangan,
    this.referensi,
    required this.nominal,
    required this.saldo,
  });
}

/// Format rupiah ke string.
String formatRupiah(int amount) {
  final negative = amount < 0;
  final absAmount = amount.abs();
  final formatted = absAmount.toString().replaceAllMapped(
    RegExp(r'(\d{1,3})(?=(\d{3})+(?!\d))'),
    (Match m) => '${m[1]}.',
  );
  return '${negative ? '-' : ''}Rp $formatted';
}

/// Format tanggal ke string Bahasa Indonesia.
String formatTanggal(DateTime date) {
  const bulan = [
    'Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun',
    'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des',
  ];
  return '${date.day} ${bulan[date.month - 1]} ${date.year}';
}

/// Warna untuk jenis transaksi.
Color getJenisColor(bool isIncoming, Brightness brightness) {
  if (isIncoming) {
    return brightness == Brightness.dark
        ? AppColors.darkSuccessSoft
        : AppColors.lightSuccessSoft;
  }
  return brightness == Brightness.dark
      ? AppColors.darkDangerSoft
      : AppColors.lightDangerSoft;
}

/// Warna text untuk jenis transaksi.
Color getJenisTextColor(Brightness brightness, bool isIncoming) {
  if (isIncoming) {
    return AppColors.success;
  }
  return AppColors.danger;
}

/// Background color untuk badge jenis.
Color getJenisBgColor(Brightness brightness, bool isIncoming) {
  if (isIncoming) {
    return brightness == Brightness.dark
        ? AppColors.darkSuccessSoft
        : AppColors.lightSuccessSoft;
  }
  return brightness == Brightness.dark
      ? AppColors.darkDangerSoft
      : AppColors.lightDangerSoft;
}

/// Text color untuk badge jenis.
Color getJenisBadgeColor(Brightness brightness, bool isIncoming) {
  if (isIncoming) return AppColors.success;
  return AppColors.danger;
}

/// Category display name.
String getKategoriLabel(KasCategory kategori) {
  return kategori.label;
}
