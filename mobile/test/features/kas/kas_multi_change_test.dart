import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/app/app.dart';
import 'package:social_finance/features/iuran/data/iuran_models.dart';
import 'package:social_finance/features/kas/data/kas_api_models.dart';
import 'package:social_finance/features/kas/export/kas_report_model.dart';
import 'package:social_finance/features/kas/presentation/screens/kas_keluar_screen.dart';

void main() {
  group('KasReportModel — Fund Locations Breakdown (Item H)', () {
    test('statementRows includes fund location balances and grand label', () {
      final model = buildKasReport(
        transactions: [
          BackendTransaction(
            id: 't1',
            rtId: 'rt-1',
            categoryId: 'c1',
            amount: '500000.00',
            type: 'income',
            status: 'posted',
            description: 'Pemasukan Kas',
            paymentMethod: 'CASH',
          ),
        ],
        openingBalance: '1000000.00',
        period: '2026-10',
        identity: const KasReportIdentity(orgName: 'RT 01'),
        fundLocations: const [
          KasFundLocationItem(
            name: 'Bendahara',
            category: 'COH',
            balanceCents: 50000000,
          ),
          KasFundLocationItem(
            name: 'Bank BCA',
            category: 'Rekening',
            balanceCents: 100000000,
          ),
        ],
      );

      final rows = model.statementRows();
      expect(rows.any((r) => r.label.contains('Saldo Bendahara (COH)')), isTrue);
      expect(rows.any((r) => r.label.contains('Saldo Bank BCA (REKENING)')), isTrue);
      expect(rows.any((r) => r.label == 'Saldo akhir bulan rekening+cash on hand'), isTrue);
    });
  });

  group('KAS Keluar — Standard 15 Expense Categories (TAMBAHAN)', () {
    test('contains all 15 required categories', () {
      const expected = [
        'Konsumsi',
        'ATK',
        'Kebersihan',
        'Keamanan',
        'Pemeliharaan dan Perbaikan',
        'Kegiatan Warga',
        'Sosial dan Santunan',
        'Operasional',
        'Transportasi',
        'Listrik dan Air',
        'Komunikasi dan Internet',
        'Administrasi dan Dokumentasi',
        'Perlengkapan dan Peralatan',
        'Sewa',
        'Lain-lain',
      ];
      expect(KasKeluarScreen.kExpenseCategories, equals(expected));
    });
  });

  group('Notification Deep Link — normalizeNotificationTarget (Item A)', () {
    test('iuran_payment_submitted generates action=verify for verification', () {
      final target = normalizeNotificationTarget(
        null,
        payload: {
          'type': 'iuran_payment_submitted',
          'bill_id': 'bill-123',
          'payment_id': 'pay-456',
        },
      );
      expect(target, contains('/home/iuran/detail/bill-123'));
      expect(target, contains('action=verify'));
      expect(target, contains('payment_id=pay-456'));
    });

    test('iuran_payment_verified generates view=status', () {
      final target = normalizeNotificationTarget(
        null,
        payload: {
          'type': 'iuran_payment_verified',
          'bill_id': 'bill-123',
          'payment_id': 'pay-456',
        },
      );
      expect(target, contains('/home/iuran/detail/bill-123'));
      expect(target, contains('view=status'));
    });
  });

  group('IuranBill — Payment Numbers Search (Item C)', () {
    test('stores paymentNumbers and matches search query', () {
      const bill = IuranBill(
        id: 'b-1',
        householdName: 'Budi Santoso',
        iuranType: 'Iuran Pengelolaan Lingkungan',
        periode: 'Oktober 2026',
        nominal: 50000,
        paymentNumbers: ['PAY-202610-0042', 'TRX-999'],
      );

      expect(bill.paymentNumbers.contains('PAY-202610-0042'), isTrue);
      expect(bill.paymentNumbers.any((n) => n.toLowerCase().contains('0042')), isTrue);
    });
  });
}
