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

/// Warna badge per kode status (konsisten Jadwal & Absensi):
///   Hadir / Sudah Absen Pulang -> hijau (success)
///   Terlambat / Belum Absen Pulang -> kuning/oranye (warning)
///   Belum Hadir / Tidak Hadir -> merah (danger)
///   Lainnya/unknown/null -> abu-abu netral
///
/// Teks sengaja dibuat lebih gelap pada tema terang dan lebih terang pada
/// tema gelap agar kontras terjaga; latar diberi alpha agar menyatu dengan
/// permukaan tema.
({Color background, Color text, Color border}) attendanceStatusColors(
  String? raw, {
  required bool isDark,
}) {
  final Color base;
  switch (normalizeAttendanceStatusCode(raw)) {
    case 'hadir':
    case 'sudah_absen_pulang':
      base = AppColors.success;
      break;
    case 'terlambat':
    case 'belum_absen_pulang':
      base = AppColors.warning;
      break;
    case 'belum_hadir':
    case 'tidak_hadir':
      base = AppColors.danger;
      break;
    default:
      base = const Color(0xFF8A9AB0); // abu-abu netral
  }
  final text = isDark
      ? Color.lerp(base, Colors.white, 0.18)!
      : Color.lerp(base, Colors.black, 0.38)!;
  return (
    background: base.withValues(alpha: isDark ? 0.22 : 0.13),
    text: text,
    border: base.withValues(alpha: isDark ? 0.55 : 0.45),
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
