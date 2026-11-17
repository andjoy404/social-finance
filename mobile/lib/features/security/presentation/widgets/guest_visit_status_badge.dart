/// Badge status kunjungan Buku Tamu.
///
/// - checkedOutAt == null  -> "Belum Keluar" (oranye)
/// - checkedOutAt != null  -> "Keluar" (hijau)
/// Status tidak dikenal/kosong ditangani secara netral (tidak diasumsikan).
library;

import 'package:flutter/material.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';

enum GuestVisitStatus { keluar, belumKeluar, unknown }

GuestVisitStatus guestVisitStatusOf(
  DateTime? checkedOutAt, {
  String? rawStatus,
}) {
  if (checkedOutAt != null) return GuestVisitStatus.keluar;
  // Jika ada indikasi eksplisit "checked_out" tapi timestamp null (inkonsisten),
  // jangan asumsikan; tampilkan netral. Default tanpa checkedOutAt = Belum Keluar.
  final s = (rawStatus ?? '').trim().toLowerCase().replaceAll(
    RegExp(r'[\s\-]+'),
    '_',
  );
  if (s == 'checked_out' || s == 'keluar' || s == 'sudah_keluar') {
    return GuestVisitStatus.unknown;
  }
  return GuestVisitStatus.belumKeluar;
}

String guestVisitStatusLabelOf(DateTime? checkedOutAt, {String? rawStatus}) {
  switch (guestVisitStatusOf(checkedOutAt, rawStatus: rawStatus)) {
    case GuestVisitStatus.keluar:
      return 'Keluar';
    case GuestVisitStatus.belumKeluar:
      return 'Belum Keluar';
    case GuestVisitStatus.unknown:
      final trimmed = (rawStatus ?? '').trim();
      return trimmed.isEmpty ? 'Tidak Diketahui' : trimmed;
  }
}

({Color background, Color text, Color border}) guestVisitStatusColors(
  DateTime? checkedOutAt, {
  String? rawStatus,
  required bool isDark,
}) {
  final Color base;
  switch (guestVisitStatusOf(checkedOutAt, rawStatus: rawStatus)) {
    case GuestVisitStatus.keluar:
      base = AppColors.success; // hijau
      break;
    case GuestVisitStatus.belumKeluar:
      base = const Color(0xFFF57C00); // oranye
      break;
    case GuestVisitStatus.unknown:
      base = const Color(0xFF8A9AB0); // abu-abu netral
      break;
  }
  final text = isDark
      ? Color.lerp(base, Colors.white, 0.22)!
      : Color.lerp(base, Colors.black, 0.40)!;
  return (
    background: base.withValues(alpha: isDark ? 0.22 : 0.13),
    text: text,
    border: base.withValues(alpha: isDark ? 0.55 : 0.45),
  );
}

class GuestVisitStatusBadge extends StatelessWidget {
  const GuestVisitStatusBadge({
    super.key,
    required this.checkedOutAt,
    this.rawStatus,
    this.fontSize = 11,
  });

  final DateTime? checkedOutAt;
  final String? rawStatus;
  final double fontSize;

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final colors = guestVisitStatusColors(
      checkedOutAt,
      rawStatus: rawStatus,
      isDark: isDark,
    );
    final label = guestVisitStatusLabelOf(checkedOutAt, rawStatus: rawStatus);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(
        color: colors.background,
        borderRadius: BorderRadius.circular(AppRadius.sm),
        border: Border.all(color: colors.border, width: 0.8),
      ),
      child: Text(
        label,
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
        style: TextStyle(
          fontSize: fontSize,
          fontWeight: FontWeight.w600,
          color: colors.text,
          height: 1.15,
        ),
      ),
    );
  }
}
