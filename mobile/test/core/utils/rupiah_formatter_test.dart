import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/core/utils/rupiah_formatter.dart';

void main() {
  group('RupiahFormatter & parseRupiahInput', () {
    test('formatIdr formats numbers with Indonesian dots', () {
      expect(RupiahFormatter.formatIdr(1000), 'Rp 1.000');
      expect(RupiahFormatter.formatIdr(15000), 'Rp 15.000');
      expect(RupiahFormatter.formatIdr(1250000), 'Rp 1.250.000');
      expect(RupiahFormatter.formatIdr(0), 'Rp 0');
    });

    test('parseRupiahInput strips dots and prefix', () {
      expect(parseRupiahInput('Rp 1.000'), 1000);
      expect(parseRupiahInput('Rp 15.000'), 15000);
      expect(parseRupiahInput('1.250.000'), 1250000);
      expect(parseRupiahInput(''), null);
      expect(parseRupiahInput('abc'), null);
    });

    test('RupiahInputFormatter formats on-the-fly with stable cursor', () {
      const formatter = RupiahInputFormatter.withPrefix();

      // Type 1 -> Rp 1
      final val1 = formatter.formatEditUpdate(
        const TextEditingValue(text: '', selection: TextSelection.collapsed(offset: 0)),
        const TextEditingValue(text: '1', selection: TextSelection.collapsed(offset: 1)),
      );
      expect(val1.text, 'Rp 1');

      // Type 000 -> Rp 1.000
      final val2 = formatter.formatEditUpdate(
        val1,
        const TextEditingValue(text: 'Rp 1000', selection: TextSelection.collapsed(offset: 7)),
      );
      expect(val2.text, 'Rp 1.000');
      expect(val2.selection.baseOffset, 8);

      // Empty input -> empty
      final valEmpty = formatter.formatEditUpdate(
        val2,
        const TextEditingValue(text: '', selection: TextSelection.collapsed(offset: 0)),
      );
      expect(valEmpty.text, '');
    });
  });
}

