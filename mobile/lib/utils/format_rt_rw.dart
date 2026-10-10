/// RT/RW 3-digit formatting helpers for Flutter mobile.
///
/// All RT/RW codes displayed in the app should use these helpers to ensure
/// consistent 3-digit zero-padded format: '003', '016', '100', etc.
///
/// Only pass actual RT/RW codes to these functions. Do NOT pass UUIDs,
/// NIKs, phone numbers, monetary amounts, or other non-wilayah identifiers.
library;

/// Format an RT code to 3-digit zero-padded string.
///
/// Accepts: `String?` ('03', '003', '3'), `int?` (3, 16), or null.
/// Returns: '003', '016', '001', etc. Always 3 digits.
/// Preserves alphanumeric codes (e.g. 'C01', 'A12') as-is.
/// Returns '000' for null, empty, invalid, or negative values.
String formatRtCode(dynamic value) {
  if (value == null) return '000';
  if (value is String) {
    final trimmed = value.trim();
    if (trimmed.isEmpty) return '000';
    try {
      final n = int.parse(trimmed);
      if (n < 0) return '000';
      return n.toString().padLeft(3, '0');
    } catch (_) {
      // Not a pure number — preserve as-is (alphanumeric codes like 'C01')
      return value;
    }
  }
  if (value is num) {
    if (value < 0) return '000';
    return value.toInt().toString().padLeft(3, '0');
  }
  return '000';
}

/// Format an RW code to 3-digit zero-padded string.
///
/// Accepts: `int?`, `num?`, or null.
/// Returns: '016', '001', etc. Always 3 digits.
/// Returns '000' for null, invalid, or negative values.
String formatRwCode(dynamic value) {
  if (value == null) return '000';
  if (value is num) {
    if (value < 0) return '000';
    return value.toInt().toString().padLeft(3, '0');
  }
  // Try parsing as string
  try {
    final n = int.parse(value.toString().trim());
    if (n < 0) return '000';
    return n.toString().padLeft(3, '0');
  } catch (_) {
    return '000';
  }
}

/// Combined RT/RW formatter for display.
///
/// Returns formatted string like "RT 003 · RW 016".
/// If both are '000', returns empty string.
String formatRtRw(String? rt, int? rw) {
  final formattedRt = formatRtCode(rt);
  final formattedRw = formatRwCode(rw);
  if (formattedRt == '000' && formattedRw == '000') return '';
  return 'RT $formattedRt${formattedRw != '000' ? ' · RW $formattedRw' : ''}';
}
