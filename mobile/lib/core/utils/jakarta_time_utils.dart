/// Jakarta (Asia/Jakarta, WIB, UTC+07:00) timezone utilities for Flutter mobile.
///
/// Indonesia does NOT observe daylight saving time, so the offset is always +07:00.
/// Guarantees that regardless of the mobile device's system timezone (UTC, America/New_York,
/// Europe/London, Asia/Tokyo, etc.), all notification timestamps and logs strictly reflect
/// Asia/Jakarta (WIB) wall-clock time without any 7-hour discrepancy or double offset.

const Duration jakartaOffset = Duration(hours: 7);

/// Converts a [DateTime], ISO 8601 [String], or dynamic input into a [DateTime]
/// whose [DateTime.year], [DateTime.month], [DateTime.day], [DateTime.hour],
/// [DateTime.minute], and [DateTime.second] strictly represent Asia/Jakarta (WIB)
/// wall-clock time.
///
/// The returned [DateTime] has `isUtc == true` to prevent mobile OS timezone
/// shifts when calling `.hour`, `.day`, etc.
DateTime? toJakartaDateTime(dynamic input) {
  if (input == null) return null;

  DateTime parsed;
  if (input is DateTime) {
    parsed = input;
  } else if (input is String) {
    final trimmed = input.trim();
    if (trimmed.isEmpty) return null;
    final dt = DateTime.tryParse(trimmed);
    if (dt == null) return null;
    parsed = dt;
  } else {
    return null;
  }

  // If the input is UTC or parsed from an explicit timezone string (Z or offset),
  // convert to UTC instant and add 7 hours for WIB.
  // If the input was a bare string parsed locally without UTC flag, check if string had timezone.
  if (parsed.isUtc) {
    final wib = parsed.add(jakartaOffset);
    return DateTime.utc(
      wib.year,
      wib.month,
      wib.day,
      wib.hour,
      wib.minute,
      wib.second,
      wib.millisecond,
      wib.microsecond,
    );
  }

  // If input was a String without timezone offset, the parsed local fields
  // already represent the literal components.
  if (input is String) {
    final upper = input.toUpperCase();
    final hasTz = upper.endsWith('Z') ||
        upper.contains('+') ||
        (upper.contains('T') && upper.substring(upper.indexOf('T')).contains('-'));
    if (!hasTz) {
      return DateTime.utc(
        parsed.year,
        parsed.month,
        parsed.day,
        parsed.hour,
        parsed.minute,
        parsed.second,
        parsed.millisecond,
        parsed.microsecond,
      );
    }
  }

  // Otherwise, normalize to UTC instant first, then add 7 hours.
  final wib = parsed.toUtc().add(jakartaOffset);
  return DateTime.utc(
    wib.year,
    wib.month,
    wib.day,
    wib.hour,
    wib.minute,
    wib.second,
    wib.millisecond,
    wib.microsecond,
  );
}

/// Formats the input to `DD/MM/YYYY HH:mm` in Asia/Jakarta (WIB) timezone.
/// Example: `07/11/2026 08:05`
String formatJakartaDateTime(dynamic input, {String fallback = '-'}) {
  final jdt = toJakartaDateTime(input);
  if (jdt == null) return fallback;

  final d = jdt.day.toString().padLeft(2, '0');
  final m = jdt.month.toString().padLeft(2, '0');
  final y = jdt.year.toString().padLeft(4, '0');
  final h = jdt.hour.toString().padLeft(2, '0');
  final min = jdt.minute.toString().padLeft(2, '0');

  return '$d/$m/$y $h:$min';
}

/// Formats the input to `DD/MM/YYYY • HH:mm` in Asia/Jakarta (WIB) timezone.
/// Example: `07/11/2026 • 08:05`
String formatJakartaDetailDateTime(dynamic input, {String fallback = '-'}) {
  final jdt = toJakartaDateTime(input);
  if (jdt == null) return fallback;

  final d = jdt.day.toString().padLeft(2, '0');
  final m = jdt.month.toString().padLeft(2, '0');
  final y = jdt.year.toString().padLeft(4, '0');
  final h = jdt.hour.toString().padLeft(2, '0');
  final min = jdt.minute.toString().padLeft(2, '0');

  return '$d/$m/$y • $h:$min';
}

/// Formats the input to `YYYY-MM-DD HH:mm` in Asia/Jakarta (WIB) timezone.
/// Example: `2026-11-07 08:05`
String formatJakartaIsoDateHour(dynamic input, {String fallback = '-'}) {
  final jdt = toJakartaDateTime(input);
  if (jdt == null) return fallback;

  final y = jdt.year.toString().padLeft(4, '0');
  final m = jdt.month.toString().padLeft(2, '0');
  final d = jdt.day.toString().padLeft(2, '0');
  final h = jdt.hour.toString().padLeft(2, '0');
  final min = jdt.minute.toString().padLeft(2, '0');

  return '$y-$m-$d $h:$min';
}

