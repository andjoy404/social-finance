/// Utilities for validating and normalizing Indonesian mobile phone numbers.
class PhoneUtils {
  PhoneUtils._();

  /// Validates an Indonesian phone number input.
  ///
  /// Accepts:
  /// - Local format starting with `08` (e.g. `081298918220`, 10 to 14 digits)
  /// - International format starting with `+628` (12 to 16 characters)
  /// - Unprefixed international starting with `628` (11 to 15 digits)
  ///
  /// Returns `null` if valid, or a descriptive error message in Indonesian.
  static String? validatePhone(String? raw) {
    if (raw == null || raw.trim().isEmpty) {
      return 'Nomor HP wajib diisi.';
    }

    final trimmed = raw.trim();
    final clean = trimmed.replaceAll(RegExp(r'[\s\-]'), '');

    // Allow digits with optional leading '+'
    final validCharsRegex = RegExp(r'^\+?[0-9]+$');
    if (!validCharsRegex.hasMatch(clean)) {
      return 'Nomor HP hanya boleh berisi angka.';
    }

    if (clean.startsWith('08')) {
      if (clean.length < 10) {
        return 'Nomor HP minimal 10 digit (contoh: 081298918220).';
      }
      if (clean.length > 14) {
        return 'Nomor HP maksimal 14 digit.';
      }
      return null;
    }

    if (clean.startsWith('+628')) {
      if (clean.length < 12) {
        return 'Nomor HP internasional minimal 12 karakter.';
      }
      if (clean.length > 16) {
        return 'Nomor HP internasional maksimal 16 karakter.';
      }
      return null;
    }

    if (clean.startsWith('628')) {
      if (clean.length < 11) {
        return 'Nomor HP minimal 11 digit.';
      }
      if (clean.length > 15) {
        return 'Nomor HP maksimal 15 digit.';
      }
      return null;
    }

    if (clean.startsWith('0')) {
      return 'Nomor HP Indonesia harus diawali 08 (contoh: 081298918220).';
    }

    return 'Format nomor HP tidak valid. Gunakan format seperti 081298918220.';
  }

  /// Normalizes an Indonesian phone number to canonical `+628...` format.
  ///
  /// Examples:
  /// - `081298918220` -> `+6281298918220`
  /// - `6281298918220` -> `+6281298918220`
  /// - `+6281298918220` -> `+6281298918220`
  static String normalizePhone(String raw) {
    final clean = raw.trim().replaceAll(RegExp(r'[\s\-]'), '');
    if (clean.startsWith('+628')) {
      return clean;
    }
    if (clean.startsWith('08')) {
      return '+628${clean.substring(2)}';
    }
    if (clean.startsWith('628')) {
      return '+628${clean.substring(3)}';
    }
    return clean;
  }

  /// Formats a phone number for user display using local Indonesian format (08...).
  ///
  /// Examples:
  /// - `+6281298918220` -> `081298918220`
  /// - `6281298918220` -> `081298918220`
  /// - `081298918220` -> `081298918220`
  /// - `null` or empty -> `-`
  static String formatDisplay(String? raw) {
    if (raw == null) return '-';
    final clean = raw.trim().replaceAll(RegExp(r'[\s\-]'), '');
    if (clean.isEmpty || clean == '-' || clean == '—') return '-';
    if (clean.startsWith('+628')) {
      return '08${clean.substring(4)}';
    }
    if (clean.startsWith('+62')) {
      return '0${clean.substring(3)}';
    }
    if (clean.startsWith('628')) {
      return '08${clean.substring(3)}';
    }
    if (clean.startsWith('62')) {
      return '0${clean.substring(2)}';
    }
    return clean;
  }
}
