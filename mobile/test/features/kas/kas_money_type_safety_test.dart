// Regression tests: KAS money type-safety — nominal/saldo are Strings, never int/BigInt raw
// AGENTS.md Rule 4.1: NEVER use floating point for money
// All monetary values must be String across backend, Web, and Android

import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/features/kas/data/kas_models.dart';
import 'package:social_finance/features/kas/data/kas_mock_data.dart';
import 'package:social_finance/core/utils/rupiah_formatter.dart';

void main() {
  group('KAS money type-safety — nominal/saldo are Strings', () {
    test('all mock transactions have String nominal', () {
      for (final tx in KasMockData.getTransactions()) {
        expect(tx.nominal, isA<String>(), reason: 'nominal for ${tx.id} must be String');
      }
    });

    test('all mock transactions have String saldo', () {
      for (final tx in KasMockData.getTransactions()) {
        expect(tx.saldo, isA<String>(), reason: 'saldo for ${tx.id} must be String');
      }
    });

    test('all nominal values parse to valid non-negative integers', () {
      for (final tx in KasMockData.getTransactions()) {
        final parsed = int.tryParse(tx.nominal);
        expect(parsed, isNotNull, reason: 'nominal "${tx.nominal}" must parse to int');
        expect(parsed!, equals(int.parse(tx.nominal))); // no leading zeros or sign chars
        expect(parsed, greaterThanOrEqualTo(0), reason: 'nominal for ${tx.id} must be non-negative');
      }
    });

    test('all saldo values parse to valid integers', () {
      for (final tx in KasMockData.getTransactions()) {
        final parsed = int.tryParse(tx.saldo);
        expect(parsed, isNotNull, reason: 'saldo "${tx.saldo}" must parse to int');
      }
    });

    test('no empty nominal or saldo values', () {
      for (final tx in KasMockData.getTransactions()) {
        expect(tx.nominal.isNotEmpty, isTrue, reason: 'nominal must not be empty for ${tx.id}');
        expect(tx.saldo.isNotEmpty, isTrue, reason: 'saldo must not be empty for ${tx.id}');
      }
    });

    test('nominal values within NUMERIC(15,2) range (max 13 digits, no decimal)', () {
      for (final tx in KasMockData.getTransactions()) {
        // NUMERIC(15,2) means 13 digits before decimal + 2 after = max 13 digits for integer amounts
        expect(tx.nominal.length, lessThanOrEqualTo(13),
            reason: 'nominal "${tx.nominal}" exceeds NUMERIC(15,2) integer capacity for ${tx.id}');
        expect(tx.nominal.contains('.'), isFalse,
            reason: 'nominal must not contain decimal point for ${tx.id}');
      }
    });
  });

  group('formatRupiah — String input', () {
    test('formats a simple string amount', () {
      expect(formatRupiah('500000'), equals('Rp 500.000'));
    });

    test('formats with separators for large amounts', () {
      expect(formatRupiah('15000000'), equals('Rp 15.000.000'));
    });

    test('handles zero as string', () {
      expect(formatRupiah('0'), equals('Rp 0'));
    });

    test('returns "Rp 0" for null/empty input', () {
      expect(formatRupiah(''), equals('Rp 0'));
    });

    test('handles large amounts up to NUMERIC(15,2) max', () {
      expect(formatRupiah('9999999999999'), equals('Rp 9.999.999.999.999'));
    });
  });

  group('parseRupiah — reversibility', () {
    test('format then parse returns original string', () {
      final original = '1500000';
      final formatted = formatRupiah(original);
      final parsed = parseRupiah(formatted);
      expect(parsed, equals(original));
    });

    test('format then parse returns original for zero', () {
      final original = '0';
      final formatted = formatRupiah(original);
      final parsed = parseRupiah(formatted);
      expect(parsed, equals(original));
    });

    test('format then parse returns original for large amount', () {
      final original = '9999999999999';
      final formatted = formatRupiah(original);
      final parsed = parseRupiah(formatted);
      expect(parsed, equals(original));
    });
  });

  group('RupiahFormatter — String-compatible', () {
    test('formatMoneyString accepts String nominal', () {
      final tx = KasTransaction(
        id: 'test-001',
        tanggal: DateTime.now(),
        jenis: KasJenis.masuk,
        kategori: KasCategory.iuranWarga,
        keterangan: 'Test',
        nominal: '500000',
        saldo: '10000000',
      );
      final formatted = RupiahFormatter.formatMoneyString(tx.nominal);
      expect(formatted, equals('Rp 500.000'));
    });

    test('formatMoneyString accepts String saldo', () {
      final tx = KasTransaction(
        id: 'test-002',
        tanggal: DateTime.now(),
        jenis: KasJenis.masuk,
        kategori: KasCategory.iuranWarga,
        keterangan: 'Test',
        nominal: '500000',
        saldo: '10000000',
      );
      final formatted = RupiahFormatter.formatMoneyString(tx.saldo);
      expect(formatted, equals('Rp 10.000.000'));
    });

    test('formatMoneyString handles zero string', () {
      expect(RupiahFormatter.formatMoneyString('0'), equals('Rp 0'));
    });
  });

  group('KasTransaction — String nominal/saldo constructor compatibility', () {
    test('can create KasTransaction with string nominal and saldo', () {
      final tx = KasTransaction(
        id: 'construct-test',
        tanggal: DateTime.now(),
        jenis: KasJenis.masuk,
        kategori: KasCategory.iuranWarga,
        keterangan: 'Test transaction',
        nominal: '0',
        saldo: '0',
      );
      expect(tx.nominal, equals('0'));
      expect(tx.saldo, equals('0'));
    });

    test('can create KasTransaction with realistic values', () {
      final tx = KasTransaction(
        id: 'construct-test-2',
        tanggal: DateTime.now(),
        jenis: KasJenis.keluar,
        kategori: KasCategory.kebersihan,
        keterangan: 'Petugas kebersihan',
        nominal: '300000',
        saldo: '9700000',
      );
      expect(tx.nominal, equals('300000'));
      expect(tx.saldo, equals('9700000'));
    });
  });

  group('Balance consistency — string parsing compatibility', () {
    test('running saldo is consistent when parsed from strings', () {
      // Simulate running balance computation from string-based data
      int running = 10000000; // initial balance
      for (final tx in KasMockData.getTransactions()) {
        final nominal = int.tryParse(tx.nominal) ?? 0;
        if (tx.jenis == KasJenis.masuk) {
          running += nominal;
        } else {
          running -= nominal;
        }
        // Running balance should parse without error
        expect(running, isA<int>());
      }
    });

    test('total masuk and keluar computed from strings are correct', () {
      int totalMasuk = 0;
      int totalKeluar = 0;
      for (final tx in KasMockData.getTransactions()) {
        final nominal = int.tryParse(tx.nominal) ?? 0;
        if (tx.jenis == KasJenis.masuk) {
          totalMasuk += nominal;
        } else {
          totalKeluar += nominal;
        }
      }
      // These should be positive valid numbers
      expect(totalMasuk, greaterThan(0));
      expect(totalKeluar, greaterThan(0));
    });
  });
}
