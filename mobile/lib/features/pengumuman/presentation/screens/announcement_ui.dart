import 'package:flutter/material.dart';

import 'package:social_finance/core/widgets/app_badge.dart';

const _statusLabels = {
  'active': 'Aktif',
  'scheduled': 'Terjadwal',
  'draft': 'Draf',
  'archived': 'Diarsipkan',
  'expired': 'Kedaluwarsa',
};

String announcementStatusLabel(String status) =>
    _statusLabels[status] ?? status;

AppBadge announcementStatusBadge(String status) {
  switch (status) {
    case 'active':
      return const AppBadge(
        label: 'Aktif',
        backgroundColor: Color(0x2210B981),
        textColor: Colors.green,
      );
    case 'scheduled':
      return const AppBadge(
        label: 'Terjadwal',
        backgroundColor: Color(0x223B82F6),
        textColor: Colors.blue,
      );
    case 'draft':
      return const AppBadge(
        label: 'Draf',
        backgroundColor: Color(0x226B7280),
        textColor: Colors.grey,
      );
    case 'archived':
      return const AppBadge(
        label: 'Diarsipkan',
        backgroundColor: Color(0x226B7280),
        textColor: Colors.grey,
      );
    case 'expired':
      return const AppBadge(
        label: 'Kedaluwarsa',
        backgroundColor: Color(0x22EF4444),
        textColor: Colors.red,
      );
    default:
      return AppBadge(
        label: status,
        backgroundColor: Color(0x226B7280),
        textColor: Colors.grey,
      );
  }
}

/// Display window label for the tayang period, in local time.
String? scheduleWindowLabel(DateTime? start, DateTime? end) {
  String fmt(DateTime t) {
    final local = t.toLocal();
    const months = [
      'Jan',
      'Feb',
      'Mar',
      'Apr',
      'Mei',
      'Jun',
      'Jul',
      'Agu',
      'Sep',
      'Okt',
      'Nov',
      'Des',
    ];
    final day = local.day.toString().padLeft(2, '0');
    final month = months[local.month - 1];
    final hour = local.hour.toString().padLeft(2, '0');
    final minute = local.minute.toString().padLeft(2, '0');
    return '$day $month ${local.year}, $hour:$minute';
  }

  if (start != null && end != null) {
    return 'Tayang ${fmt(start)} — ${fmt(end)}';
  }
  if (start != null) return 'Tayang mulai ${fmt(start)}';
  if (end != null) return 'Tayang sampai ${fmt(end)}';
  return null;
}

/// Maps internal routes to human-readable labels matching Web Urena.
String formatDeepLinkDestination(String? link) {
  if (link == null || link.trim().isEmpty) return '';
  final clean = link.trim().toLowerCase();
  if (clean == '/iuran' || clean.startsWith('/iuran/')) return 'Iuran';
  if (clean == '/keuangan' || clean.startsWith('/keuangan/')) return 'Keuangan';
  if (clean == '/kas' || clean.startsWith('/kas/')) return 'Kas';
  if (clean == '/kegiatan' || clean.startsWith('/kegiatan/')) return 'Kegiatan';
  if (clean == '/warga' || clean.startsWith('/warga/')) return 'Warga';
  if (clean == '/notifikasi' || clean.startsWith('/notifikasi/')) {
    return 'Notifikasi';
  }
  if (clean == '/pengumuman' || clean.startsWith('/pengumuman/')) {
    return 'Pengumuman';
  }
  if (clean == '/profil' || clean.startsWith('/profil/')) return 'Profil';
  if (clean == '/buku-tamu' ||
      clean.startsWith('/buku-tamu/') ||
      clean == '/security-guestbook' ||
      clean.startsWith('/security-guestbook/')) {
    return 'Buku Tamu';
  }
  if (clean == '/patroli' || clean.startsWith('/patroli/')) return 'Patroli';
  if (clean == '/paket' || clean.startsWith('/paket/')) return 'Paket';
  final stripped = link.trim().replaceAll(RegExp(r'^/+'), '');
  if (stripped.isEmpty) return '';
  return stripped[0].toUpperCase() + stripped.substring(1);
}
