/// Urutan daftar Absensi — terbaru dulu ("descending").
///
/// Primary: tanggal absensi (shift_date) terbaru di atas.
/// Secondary: timestamp absensi — `checked_in_at` bila tersedia, jika tidak
/// ada digunakan jadwal mulai shift (`shiftDate` + `startTime`).
/// Tie-breaker: id record (stabil & deterministik).
library;

import '../../data/security_models.dart';

/// Timestamp yang dipakai membandingkan dua record pada tanggal yang sama.
DateTime attendanceSortInstant(AttendanceRecord r) {
  final checkedIn = r.checkedInAt;
  if (checkedIn != null) return checkedIn;
  try {
    final start = r.startTime.length >= 5
        ? r.startTime.substring(0, 5)
        : r.startTime;
    final parsed = DateTime.tryParse('${r.shiftDate}T$start:00');
    if (parsed != null) return parsed;
  } catch (_) {}
  return DateTime.fromMillisecondsSinceEpoch(0);
}

/// Comparator: urut dari tanggal/waktu terbaru ke terlama.
///
/// - tanggal sama → timestamp (checked_in_at / jadwal mulai) DESC;
/// - timestamp sama → id ASC (tie-breaker stabil).
int compareAttendanceRecency(AttendanceRecord a, AttendanceRecord b) {
  final byDate = b.shiftDate.compareTo(a.shiftDate);
  if (byDate != 0) return byDate;
  final byTime = attendanceSortInstant(b).compareTo(attendanceSortInstant(a));
  if (byTime != 0) return byTime;
  return a.id.compareTo(b.id);
}
