/// Status kehadiran petugas keamanan — badge warna.
///
/// Sumber kebenaran nilai status: backend `internal/security/model.go`
/// (CHECK constraint `security_attendance.status`):
///   belum_hadir, hadir, terlambat, sudah_absen_pulang, tidak_hadir,
///   belum_absen_pulang
///
/// Label mengikuti [StatusLabel] backend (Bahasa Indonesia). Klasifikasi
/// berbasis *kode status* (field `status` / `attendance_status`), bukan
/// label tampilan, sehingga tahan terhadap variasi kapitalisasi/spasi.
/// Status tak dikenal/null ditampilkan dengan warna netral (abu-abu).

import 'package:flutter/material.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';

/// Normalisasi kode status: trim + lowercase + spasi/dash -> underscore.
String normalizeAttendanceStatusCode(String? raw) {
  final v = (raw ?? '').trim().toLowerCase();
  if (v.isEmpty) return '';
  return v.replaceAll(RegExp(r'[\s\-]+'), '_');
}

/// Label kanonik (identik dengan `StatusLabel` backend).
/// Menerima baik kode status maupun label backend (keduanya dinormalkan
/// sehingga "Belum Absen Pulang" dan "belum_absen_pulang" tetap cocok).
/// Status tak dikenal ditampilkan apa adanya (trim) atau 'Tidak Diketahui'
/// jika kosong.
String attendanceStatusLabelOf(String? raw) {
  switch (normalizeAttendanceStatusCode(raw)) {
    case 'belum_hadir':
      return 'Belum Hadir';
    case 'hadir':
      return 'Hadir';
    case 'terlambat':
      return 'Terlambat';
    case 'sudah_absen_pulang':
      return 'Sudah Absen Pulang';
    case 'tidak_hadir':
      return 'Tidak Hadir';
    case 'belum_absen_pulang':
      return 'Belum Absen Pulang';
    default:
      final trimmed = (raw ?? '').trim();
      return trimmed.isEmpty ? 'Tidak Diketahui' : trimmed;
  }
}

/// Warna badge per kode status (konsisten KELOLA→JADWAL & ABSENSI→ABSENSI):
///   Belum Hadir         -> abu-abu terang (netral)
///   Tidak Hadir         -> merah (danger)
///   Hadir               -> hijau (success)
///   Sudah Absen Pulang  -> biru (info)
///   Terlambat           -> oranye/amber (status lain, pembeda)
///   Belum Absen Pulang  -> kuning (status lain, pembeda)
///   Lainnya/unknown/null -> abu-abu netral
///
/// Label asli status tidak diubah. Teks kontras untuk light/dark theme.
({Color background, Color text, Color border}) attendanceStatusColors(
  String? raw, {
  required bool isDark,
}) {
  final Color base;
  switch (normalizeAttendanceStatusCode(raw)) {
    case 'hadir':
      base = AppColors.success; // hijau
      break;
    case 'sudah_absen_pulang':
      base = AppColors.info; // biru
      break;
    case 'tidak_hadir':
      base = AppColors.danger; // merah
      break;
    case 'belum_hadir':
      base = const Color(0xFFCFD8DC); // abu-abu terang
      break;
    case 'terlambat':
      base = const Color(0xFFF57C00); // oranye (pembeda)
      break;
    case 'belum_absen_pulang':
      base = AppColors.warning; // kuning (pembeda)
      break;
    default:
      base = const Color(0xFF90A4AE); // abu-abu netral (unknown/null)
  }
  final text = isDark
      ? Color.lerp(base, Colors.white, 0.20)!
      : Color.lerp(base, Colors.black, 0.45)!;
  return (
    background: base.withValues(alpha: isDark ? 0.24 : 0.18),
    text: text,
    border: base.withValues(alpha: isDark ? 0.55 : 0.50),
  );
}

/// Badge pill untuk status kehadiran — kompatibel light/dark theme.
class AttendanceStatusBadge extends StatelessWidget {
  const AttendanceStatusBadge({
    super.key,
    required this.statusCode,
    this.statusLabel,
    this.fontSize = 11,
  });

  /// Kode status mentah dari backend (e.g. "hadir"). Boleh null — aman.
  final String? statusCode;

  /// Label tampilan dari backend (dipakai bila tersedia); jika kosong,
  /// label diturunkan dari kode.
  final String? statusLabel;

  final double fontSize;

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final colors = attendanceStatusColors(statusCode, isDark: isDark);
    final rawLabel = (statusLabel ?? '').trim();
    final label = rawLabel.isNotEmpty
        ? rawLabel
        : attendanceStatusLabelOf(statusCode);
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
