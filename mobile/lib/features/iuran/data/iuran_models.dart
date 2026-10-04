import 'package:flutter/material.dart';

import 'package:social_finance/core/theme/app_colors.dart';

/// Enum representing the status of an iuran (dues) bill.
enum IuranStatus {
  belumBayar,
  sebagian,
  lunas,
}

/// Iuran bill model for the IURAN feature.
class IuranBill {
  final String id;
  final String householdName;
  final String iuranType;
  final String periode;
  final int nominal; // amount in rupiah (int, no floating point)
  final int paidAmount;
  final IuranStatus status;

  const IuranBill({
    required this.id,
    required this.householdName,
    required this.iuranType,
    required this.periode,
    required this.nominal,
    this.paidAmount = 0,
    this.status = IuranStatus.belumBayar,
  });

  /// Returns the remaining amount to be paid.
  int get remainingAmount => nominal - paidAmount;

  /// Returns true if this bill is in arrears (not yet paid or partially paid).
  bool get isInArrears => status != IuranStatus.lunas;
}

/// Format a nominal amount in rupiah to Indonesian currency string.
/// e.g. 50000 → "Rp 50.000"
String formatRupiah(int amount) {
  return 'Rp ${amount.toString().replaceAllMapped(
    RegExp(r'(\d{1,3})(?=(\d{3})+(?!\d))'),
    (Match m) => '${m[1]}.',
  )}';
}

/// Convert IuranStatus enum to a display label.
String getStatusLabel(IuranStatus status) {
  return switch (status) {
    IuranStatus.belumBayar => 'Belum Bayar',
    IuranStatus.sebagian => 'Sebagian',
    IuranStatus.lunas => 'Lunas',
  };
}

/// Return the color associated with the given status.
Color getStatusColor(IuranStatus status, BuildContext context) {
  final isDark = Theme.of(context).brightness == Brightness.dark;
  return switch (status) {
    IuranStatus.belumBayar => AppColors.warning,
    IuranStatus.sebagian => AppColors.warning,
    IuranStatus.lunas => AppColors.success,
  };
}

/// Convert a status string from API to enum (for future API integration).
IuranStatus parseStatus(String value) {
  return switch (value.toLowerCase()) {
    'belum_bayar' || 'belum bayar' => IuranStatus.belumBayar,
    'sebagian' => IuranStatus.sebagian,
    'lunas' => IuranStatus.lunas,
    _ => IuranStatus.belumBayar,
  };
}
