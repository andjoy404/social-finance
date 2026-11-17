/// Urutan daftar Absensi — paling lama dulu (ascending).
///
/// Primary: tanggal absensi (shift_date) paling lama di atas.
/// Secondary: timestamp absensi — `checked_in_at` bila tersedia, jika tidak
/// ada digunakan jadwal mulai shift (`shiftDate` + `startTime`) — ASC.
/// Tie-breaker: id record (stabil & deterministik).
library;

import '../../data/security_models.dart';

/// Timestamp yang dipakai membandingkan dua record pada tanggal yang sama.
DateTime attendanceSortInstant(AttendanceRecord r) {
  final checkedIn = r.checkedInAt;
  if (checkedIn != null) return checkedIn.toLocal();
  try {
    final start = r.startTime.length >= 5
        ? r.startTime.substring(0, 5)
        : r.startTime;
    final parsed = DateTime.tryParse('${r.shiftDate}T$start:00');
    if (parsed != null) return parsed;
  } catch (_) {}
  try {
    final d = DateTime.tryParse(r.shiftDate);
    if (d != null) return d;
  } catch (_) {}
  return DateTime.fromMillisecondsSinceEpoch(0);
}

/// Comparator: urut dari tanggal/waktu paling lama ke terbaru (ASC).
///
/// - tanggal sama → timestamp (checked_in_at / jadwal mulai) ASC;
/// - timestamp sama → id ASC (tie-breaker stabil).
int compareAttendanceChronological(AttendanceRecord a, AttendanceRecord b) {
  final byDate = a.shiftDate.compareTo(b.shiftDate);
  if (byDate != 0) return byDate;
  final byTime = attendanceSortInstant(a).compareTo(attendanceSortInstant(b));
  if (byTime != 0) return byTime;
  return a.id.compareTo(b.id);
}

/// Backward-compatible alias; now sorts ascending (oldest first).
int compareAttendanceRecency(AttendanceRecord a, AttendanceRecord b) =>
    compareAttendanceChronological(a, b);

/// Timestamp kunjungan Buku Tamu (UTC-safe via toLocal when parsed from ISO).
DateTime guestBookSortInstant(GuestBookEntry g) {
  return g.checkedInAt.toLocal();
}

/// Comparator Buku Tamu: tanggal/waktu kunjungan paling lama ke terbaru (ASC).
/// Tie-breaker: id stabil.
int compareGuestBookChronological(GuestBookEntry a, GuestBookEntry b) {
  final byTime = guestBookSortInstant(a).compareTo(guestBookSortInstant(b));
  if (byTime != 0) return byTime;
  return a.id.compareTo(b.id);
}
