/// Jakarta (Asia/Jakarta, WIB, UTC+07:00) timezone utilities for Dart.
///
/// Indonesia does NOT observe daylight saving time, so the offset is always +07:00.
/// All manual KAS transactions must use Jakarta calendar date and Jakarta wall-clock
/// time, then be converted to an unambiguous ISO 8601 timestamp for the backend.
///
/// Dart's DateTime has no built-in IANA timezone support. Since WIB is fixed at
/// UTC+07:00 (no DST), we compute Jakarta wall-clock time from UTC by adding 7 hours,
/// combine with the selected Jakarta date, then convert back to UTC for output.

const int _jakartaOffsetHours = 7;

/// Get the current wall-clock time components in Asia/Jakarta.
///
/// Since Dart DateTime has no timezone support, we derive Jakarta time from UTC:
/// Jakarta wall clock = UTC + 7 hours (fixed, no DST in Indonesia).
DateTime _nowJakarta() {
  return DateTime.now().toUtc().add(const Duration(hours: _jakartaOffsetHours));
}

String _pad2(int n) => n.toString().padLeft(2, '0');

String _pad3(int n) => n.toString().padLeft(3, '0');

/// Convert a Jakarta calendar date string ("YYYY-MM-DD") plus current Jakarta
/// wall-clock time into a UTC [DateTime] that represents the same instant.
///
/// This avoids the pitfall of `DateTime.tryParse("YYYY-MM-DD")`, which creates a
/// DateTime in the device's local timezone and can shift the date when the device
/// is not in WIB.
///
/// @param dateStr - Jakarta calendar date in "YYYY-MM-DD" format (e.g. "2026-10-22")
/// @returns UTC [DateTime] representing the selected Jakarta date at current Jakarta time
DateTime toJakartaUtc(String dateStr) {
  final parts = dateStr.split('-');
  final year = int.parse(parts[0]);
  final month = int.parse(parts[1]);
  final day = int.parse(parts[2]);

  // Current Jakarta wall-clock time
  final nowJakarta = _nowJakarta();

  // Combine selected Jakarta date with current Jakarta time as a UTC instant.
  // Jakarta wall clock = UTC + 7, so UTC instant = Jakarta wall clock - 7 hours.
  // We build a UTC DateTime with the Jakarta wall-clock components, then subtract 7h.
  return DateTime.utc(
    year,
    month,
    day,
    nowJakarta.hour,
    nowJakarta.minute,
    nowJakarta.second,
    nowJakarta.millisecond,
  ).subtract(const Duration(hours: _jakartaOffsetHours));
}

/// Format a [DateTime] as an ISO 8601 string with explicit +07:00 offset,
/// representing the Jakarta wall-clock time.
///
/// This is useful for display or debugging. The backend receives UTC (via
/// [toJakartaUtc].toIso8601String()), which is the canonical instant.
///
/// @param dateTime - UTC [DateTime]
/// @returns ISO 8601 string like "2026-10-22T10:30:00.000+07:00"
String formatJakartaIso8601(DateTime dateTime) {
  final jakarta = dateTime.add(const Duration(hours: _jakartaOffsetHours));
  return ('${jakarta.year}-${_pad2(jakarta.month)}-${_pad2(jakarta.day)}T'
      '${_pad2(jakarta.hour)}:${_pad2(jakarta.minute)}:${_pad2(jakarta.second)}.'
      '${_pad3(jakarta.millisecond)}+07:00');
}
