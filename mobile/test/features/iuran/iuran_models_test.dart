/// Unit tests for `IuranBill` model — `remainingAmount` and `paymentCapacity` calculations.
///
/// Covers:
/// - `remainingAmount` = nominal - paidAmount (display only, APPROVED payments)
/// - `paymentCapacity` = max(0, nominal - paidAmount - pendingAmount) (validation cap)
/// - Edge cases: zero/negative results capped at 0
/// - Existing behaviors: `isInArrears`, `formatRupiah`, `getStatusLabel`

import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/features/iuran/data/iuran_api_models.dart';
import 'package:social_finance/features/iuran/data/iuran_models.dart';

void main() {
  // ─── remainingAmount ───────────────────────────────────────────────────
  // Rule 3: Sisa Tagihan display = nominal - paidAmount (APPROVED only)
  // Pending does NOT reduce the displayed balance.

  group('IuranBill.remainingAmount', () {
    test('nominal 25000, approved 0, pending 0 → sisa 25000', () {
      final bill = const IuranBill(
        id: 'b1',
        householdName: 'Test',
        iuranType: 'Keamanan',
        periode: 'Oktober 2026',
        nominal: 25000,
        paidAmount: 0,
        pendingAmount: 0,
      );
      expect(bill.remainingAmount, equals(25000));
    });

    test('nominal 25000, approved 0, pending 10000 → sisa 25000', () {
      final bill = const IuranBill(
        id: 'b2',
        householdName: 'Test',
        iuranType: 'Keamanan',
        periode: 'Oktober 2026',
        nominal: 25000,
        paidAmount: 0,
        pendingAmount: 10000,
      );
      // Pending payment does NOT reduce display — still shows full nominal
      expect(bill.remainingAmount, equals(25000));
    });

    test('nominal 25000, approved 10000, pending 5000 → sisa 15000', () {
      final bill = const IuranBill(
        id: 'b3',
        householdName: 'Test',
        iuranType: 'Keamanan',
        periode: 'Oktober 2026',
        nominal: 25000,
        paidAmount: 10000,
        pendingAmount: 5000,
      );
      expect(bill.remainingAmount, equals(15000));
    });

    test('nominal 25000, approved 25000, pending 0 → sisa 0', () {
      final bill = const IuranBill(
        id: 'b4',
        householdName: 'Test',
        iuranType: 'Keamanan',
        periode: 'Oktober 2026',
        nominal: 25000,
        paidAmount: 25000,
        pendingAmount: 0,
      );
      expect(bill.remainingAmount, equals(0));
    });

    test('nominal 25000, approved 25000, pending 10000 → sisa 0', () {
      final bill = const IuranBill(
        id: 'b5',
        householdName: 'Test',
        iuranType: 'Keamanan',
        periode: 'Oktober 2026',
        nominal: 25000,
        paidAmount: 25000,
        pendingAmount: 10000,
      );
      expect(bill.remainingAmount, equals(0));
    });
  });

  // ─── paymentCapacity ──────────────────────────────────────────────────
  // Rule 4: Payment capacity = max(0, nominal - paidAmount - pendingAmount)

  group('IuranBill.paymentCapacity', () {
    test('nominal 25000, approved 0, pending 0 → kapasitas 25000', () {
      final bill = const IuranBill(
        id: 'b1',
        householdName: 'Test',
        iuranType: 'Keamanan',
        periode: 'Oktober 2026',
        nominal: 25000,
        paidAmount: 0,
        pendingAmount: 0,
      );
      expect(bill.paymentCapacity, equals(25000));
    });

    test('nominal 25000, approved 0, pending 10000 → kapasitas 15000', () {
      final bill = const IuranBill(
        id: 'b2',
        householdName: 'Test',
        iuranType: 'Keamanan',
        periode: 'Oktober 2026',
        nominal: 25000,
        paidAmount: 0,
        pendingAmount: 10000,
      );
      expect(bill.paymentCapacity, equals(15000));
    });

    test('nominal 25000, approved 10000, pending 5000 → kapasitas 10000', () {
      final bill = const IuranBill(
        id: 'b3',
        householdName: 'Test',
        iuranType: 'Keamanan',
        periode: 'Oktober 2026',
        nominal: 25000,
        paidAmount: 10000,
        pendingAmount: 5000,
      );
      expect(bill.paymentCapacity, equals(10000));
    });

    test('nominal 25000, approved 25000, pending 0 → kapasitas 0', () {
      final bill = const IuranBill(
        id: 'b4',
        householdName: 'Test',
        iuranType: 'Keamanan',
        periode: 'Oktober 2026',
        nominal: 25000,
        paidAmount: 25000,
        pendingAmount: 0,
      );
      expect(bill.paymentCapacity, equals(0));
    });

    test(
      'nominal 25000, approved 20000, pending 10000 → kapasitas 0 (not negative)',
      () {
        final bill = const IuranBill(
          id: 'b6',
          householdName: 'Test',
          iuranType: 'Keamanan',
          periode: 'Oktober 2026',
          nominal: 25000,
          paidAmount: 20000,
          pendingAmount: 10000,
        );
        // 25000 - 20000 - 10000 = -5000, capped at 0
        expect(bill.paymentCapacity, equals(0));
      },
    );

    test(
      'nominal 25000, approved 30000, pending 5000 → kapasitas 0 (overpayment)',
      () {
        final bill = const IuranBill(
          id: 'b7',
          householdName: 'Test',
          iuranType: 'Keamanan',
          periode: 'Oktober 2026',
          nominal: 25000,
          paidAmount: 30000,
          pendingAmount: 5000,
        );
        expect(bill.paymentCapacity, equals(0));
      },
    );
  });

  // ─── rejectedAmount field ────────────────────────────────────────────────

  test('rejectedAmount defaults to 0', () {
    final bill = const IuranBill(
      id: 'b-rej-default',
      householdName: 'Test',
      iuranType: 'Keamanan',
      periode: 'Oktober 2026',
      nominal: 100000,
      paidAmount: 50000,
      pendingAmount: 25000,
    );
    expect(bill.rejectedAmount, equals(0));
  });

  test('rejectedAmount non-zero when set', () {
    final bill = const IuranBill(
      id: 'b-rej-set',
      householdName: 'Test',
      iuranType: 'Keamanan',
      periode: 'Oktober 2026',
      nominal: 100000,
      paidAmount: 50000,
      pendingAmount: 25000,
      rejectedAmount: 25000,
    );
    expect(bill.rejectedAmount, equals(25000));
  });

  group('remainingAmount with rejectedAmount', () {
    test('rejected does NOT reduce remaining display', () {
      final bill = const IuranBill(
        id: 'b-rem-rej',
        householdName: 'Test',
        iuranType: 'Keamanan',
        periode: 'Oktober 2026',
        nominal: 100000,
        paidAmount: 50000,
        pendingAmount: 25000,
        rejectedAmount: 25000,
      );
      // Remaining = nominal - paidAmount only
      expect(bill.remainingAmount, equals(50000));
    });
  });

  group('paymentCapacity with rejectedAmount', () {
    test('rejected does NOT reduce capacity', () {
      final bill = const IuranBill(
        id: 'b-cap-rej',
        householdName: 'Test',
        iuranType: 'Keamanan',
        periode: 'Oktober 2026',
        nominal: 100000,
        paidAmount: 50000,
        pendingAmount: 25000,
        rejectedAmount: 25000,
      );
      // Capacity = max(0, nominal - paid - pending) = 100000 - 50000 - 25000 = 25000
      // Rejected does NOT affect capacity
      expect(bill.paymentCapacity, equals(25000));
    });
  });

  // ─── Key Distinction ─────────────────────────────────────────────────

  group('remainingAmount vs paymentCapacity distinction', () {
    test('pending reduces capacity but NOT remaining display', () {
      final bill = const IuranBill(
        id: 'b-diff',
        householdName: 'Test',
        iuranType: 'Keamanan',
        periode: 'Oktober 2026',
        nominal: 25000,
        paidAmount: 0,
        pendingAmount: 10000,
      );

      // Display shows full remaining (what user sees as "Sisa Tagihan")
      expect(bill.remainingAmount, equals(25000));

      // Validation cap accounts for pending (what can still be paid)
      expect(bill.paymentCapacity, equals(15000));
    });
  });

  // ─── isInArrears ──────────────────────────────────────────────────────

  group('IuranBill.isInArrears', () {
    test('belumBayar → isInArrears true', () {
      final bill = const IuranBill(
        id: 'b-arrears',
        householdName: 'Test',
        iuranType: 'Keamanan',
        periode: 'Oktober 2026',
        nominal: 25000,
        status: IuranStatus.belumBayar,
      );
      expect(bill.isInArrears, isTrue);
    });

    test('sebagian → isInArrears true', () {
      final bill = const IuranBill(
        id: 'b-partial',
        householdName: 'Test',
        iuranType: 'Keamanan',
        periode: 'Oktober 2026',
        nominal: 25000,
        status: IuranStatus.sebagian,
      );
      expect(bill.isInArrears, isTrue);
    });

    test('lunas → isInArrears false', () {
      final bill = const IuranBill(
        id: 'b-paid',
        householdName: 'Test',
        iuranType: 'Keamanan',
        periode: 'Oktober 2026',
        nominal: 25000,
        status: IuranStatus.lunas,
      );
      expect(bill.isInArrears, isFalse);
    });
  });

  // ─── formatRupiah ─────────────────────────────────────────────────────

  group('formatRupiah', () {
    test('formats 50000 as "Rp50.000"', () {
      expect(formatRupiah(50000), equals('Rp50.000'));
    });

    test('formats 0 as "Rp0"', () {
      expect(formatRupiah(0), equals('Rp0'));
    });

    test('formats 1000000 as "Rp1.000.000"', () {
      expect(formatRupiah(1000000), equals('Rp1.000.000'));
    });
  });

  // ─── getStatusLabel ───────────────────────────────────────────────────

  group('getStatusLabel', () {
    test('belumBayar → "Belum Bayar"', () {
      expect(getStatusLabel(IuranStatus.belumBayar), equals('Belum Bayar'));
    });

    test('sebagian → "Sebagian"', () {
      expect(getStatusLabel(IuranStatus.sebagian), equals('Sebagian'));
    });

    test('lunas → "Lunas"', () {
      expect(getStatusLabel(IuranStatus.lunas), equals('Lunas'));
    });
  });

  // ─── parseStatus ──────────────────────────────────────────────────────

  group('parseStatus', () {
    test('parses "belum_bayar" → belumBayar', () {
      expect(parseStatus('belum_bayar'), equals(IuranStatus.belumBayar));
    });

    test('parses "belum bayar" → belumBayar', () {
      expect(parseStatus('belum bayar'), equals(IuranStatus.belumBayar));
    });

    test('parses "sebagian" → sebagian', () {
      expect(parseStatus('sebagian'), equals(IuranStatus.sebagian));
    });

    test('parses "lunas" → lunas', () {
      expect(parseStatus('lunas'), equals(IuranStatus.lunas));
    });

    test('unknown value defaults to belumBayar', () {
      expect(parseStatus('unknown'), equals(IuranStatus.belumBayar));
    });
  });

  group('donationStatusMessage', () {
    test('clearly distinguishes pending, approved, and rejected', () {
      expect(
        donationStatusMessage(PaymentStatus.pending),
        contains('menunggu verifikasi'),
      );
      expect(
        donationStatusMessage(PaymentStatus.approved),
        contains('disetujui'),
      );
      expect(
        donationStatusMessage(PaymentStatus.rejected),
        contains('ditolak'),
      );
      expect(
        donationStatusMessage(PaymentStatus.rejected),
        isNot(contains('berhasil dicatat')),
      );
    });
  });
}
