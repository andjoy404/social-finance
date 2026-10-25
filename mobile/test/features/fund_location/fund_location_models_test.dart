import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/features/fund_location/data/models/fund_location_models.dart';

void main() {
  group('FundLocation Models & Money Precision', () {
    test('FundLocation parses JSON with exact integer rupiah conversion', () {
      final json = {
        'id': 'loc-1',
        'rt_id': 'rt-001',
        'name': 'BCA Operasional RT',
        'category': 'rekening',
        'bank_name': 'BCA',
        'account_number': '1234567890',
        'account_holder': 'RT 05',
        'is_active': true,
        'current_balance': '1500000.00',
        'created_at': '2026-10-25T10:00:00Z',
        'updated_at': '2026-10-25T10:00:00Z',
      };

      final loc = FundLocation.fromJson(json);

      expect(loc.id, 'loc-1');
      expect(loc.name, 'BCA Operasional RT');
      expect(loc.category, FundLocationCategory.rekening);
      expect(loc.bankName, 'BCA');
      expect(loc.accountNumber, '1234567890');
      expect(loc.isActive, true);
      expect(loc.currentBalance, '1500000.00');
      expect(loc.balanceRupiah, 1500000);
    });

    test('FundTransfer parses JSON with reversal detection', () {
      final json = {
        'id': 'txf-1',
        'rt_id': 'rt-001',
        'from_location_id': 'loc-1',
        'to_location_id': 'loc-2',
        'amount': '250000.00',
        'transfer_date': '2026-10-25T11:00:00Z',
        'notes': 'Kasbon operasional',
        'reverses_transfer_id': null,
        'from_location_name': 'BCA',
        'to_location_name': 'Brankas',
      };

      final transfer = FundTransfer.fromJson(json);
      expect(transfer.amountRupiah, 250000);
      expect(transfer.isReversal, false);

      final reversalJson = {
        'id': 'txf-2',
        'rt_id': 'rt-001',
        'from_location_id': 'loc-2',
        'to_location_id': 'loc-1',
        'amount': '250000.00',
        'transfer_date': '2026-10-25T12:00:00Z',
        'notes': 'Reversal txf-1',
        'reverses_transfer_id': 'txf-1',
      };

      final rev = FundTransfer.fromJson(reversalJson);
      expect(rev.isReversal, true);
      expect(rev.reversesTransferId, 'txf-1');
    });

    test('FundLocationReconciliationReport validates formula K = L + U', () {
      final json = {
        'rt_id': 'rt-001',
        'kas_balance': '5000000.00',
        'total_allocated_balance': '3500000.00',
        'unallocated_balance': '1500000.00',
        'discrepancy': '0.00',
        'status': 'Cocok',
        'as_of': '2026-10-25T12:00:00Z',
        'anomalies_detected': false,
      };

      final report = FundLocationReconciliationReport.fromJson(json);
      expect(report.kasBalanceRupiah, 5000000);
      expect(report.allocatedBalanceRupiah, 3500000);
      expect(report.unallocatedBalanceRupiah, 1500000);
      expect(report.discrepancyRupiah, 0);
      expect(report.isBalanced, true);
      expect(report.anomaliesDetected, false);

      // Verify formula: K == L + U
      expect(report.kasBalanceRupiah, report.allocatedBalanceRupiah + report.unallocatedBalanceRupiah);
    });
  });
}
