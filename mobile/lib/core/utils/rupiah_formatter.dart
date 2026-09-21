import 'package:intl/intl.dart';

class RupiahFormatter {
  RupiahFormatter._();

  static final _formatter = NumberFormat.currency(
    locale: 'id_ID',
    symbol: 'Rp ',
    decimalDigits: 0,
  );

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
