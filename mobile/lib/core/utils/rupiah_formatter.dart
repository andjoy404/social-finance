import 'package:flutter/services.dart';
import 'package:intl/intl.dart';

class RupiahFormatter {
  RupiahFormatter._();

  static final _formatter = NumberFormat.currency(
    locale: 'id_ID',
    symbol: 'Rp ',
    decimalDigits: 0,
  );

  /// Compact Indonesian Rupiah format with a space after the symbol,
  /// e.g. 10000 → "Rp 10.000", 1500000 → "Rp 1.500.000".
  ///
  /// Uses integer arithmetic only (no floating point). Used by the Iuran
  /// feature as its single shared money formatter.
  static String formatIdr(int amount) {
    final negative = amount < 0;
    final digits = amount.abs().toString().replaceAllMapped(
      RegExp(r'(\d{1,3})(?=(\d{3})+(?!\d))'),
      (Match m) => '${m[1]}.',
    );
    return negative ? '-Rp $digits' : 'Rp $digits';
  }

  /// Formats a backend NUMERIC money string (e.g. "1500000.00") as
  /// "Rp1.500.000". Only the whole-rupiah part is shown; the value is never
  /// multiplied or rounded through floating point.
  static String formatMoneyString(String value) {
    return formatIdr(parseMoneyString(value));
  }

  /// Parses a backend NUMERIC money string ("1500000.00") into whole rupiah.
  static int parseMoneyString(String value) {
    final whole = value.trim().split('.').first;
    return int.tryParse(whole) ?? 0;
  }

  static String format(int amount) {
    return _formatter.format(amount);
  }

  static String formatWithNegative(int amount) {
    if (amount < 0) {
      return '-${_formatter.format(amount.abs())}';
    }
    return _formatter.format(amount);
  }
}

/// Parses a user-entered Rupiah input such as "1.500.000" or "Rp1.500.000"
/// into whole rupiah (1500000). Returns null when no digits are present.
///
/// Thousand separators and the "Rp" prefix are display-only and are stripped
/// before the value is sent to the API.
int? parseRupiahInput(String input) {
  final digits = input.replaceAll(RegExp(r'[^0-9]'), '');
  if (digits.isEmpty) return null;
  return int.tryParse(digits);
}

/// Text input formatter that keeps only digits and groups them with dot
/// thousand separators while typing (e.g. "1500000" → "1.500.000").
class RupiahInputFormatter extends TextInputFormatter {
  @override
  TextEditingValue formatEditUpdate(
    TextEditingValue oldValue,
    TextEditingValue newValue,
  ) {
    final digits = newValue.text.replaceAll(RegExp(r'[^0-9]'), '');
    if (digits.isEmpty) {
      return const TextEditingValue(text: '');
    }
    // Drop leading zeros (but keep a single "0").
    final normalized = digits.replaceFirst(RegExp(r'^0+(?=\d)'), '');
    final formatted = normalized.replaceAllMapped(
      RegExp(r'(\d{1,3})(?=(\d{3})+(?!\d))'),
      (Match m) => '${m[1]}.',
    );
    return TextEditingValue(
      text: formatted,
      selection: TextSelection.collapsed(offset: formatted.length),
    );
  }

  /// Formats an integer for pre-filling an input controller ("1.500.000").
  static String formatForInput(int amount) {
    return amount.abs().toString().replaceAllMapped(
      RegExp(r'(\d{1,3})(?=(\d{3})+(?!\d))'),
      (Match m) => '${m[1]}.',
    );
  }
}
