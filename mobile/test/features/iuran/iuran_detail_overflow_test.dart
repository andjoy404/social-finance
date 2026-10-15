/// Regression test for B1: payment history Row overflow fix.
///
/// Ensures that payment history items with long paymentNo values
/// do not cause RenderFlex horizontal overflow when rendered in the
/// available width.
///
/// Fix: Both the amount+method Text and paymentNo Text are wrapped
/// in Flexible with TextOverflow.ellipsis.

import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:social_finance/features/iuran/data/iuran_api_models.dart';
import 'package:social_finance/features/iuran/data/iuran_models.dart';
import 'package:social_finance/features/iuran/data/iuran_providers.dart';
import 'package:social_finance/features/iuran/presentation/screens/iuran_detail_screen.dart';
import 'package:social_finance/features/iuran/presentation/widgets/payment_proof_thumbnail.dart';

/// Build a minimal IuranDetailScreen that renders payment history
/// without needing a real API call.
Widget _buildDetailScreen({required List<BackendPayment> payments}) {
  final testBill = IuranBill(
    id: 'test-bill',
    householdName: 'Rumah 01 — Budi Santoso',
    iuranType: 'Keamanan',
    periode: '2026-10',
    nominal: 50000,
    paidAmount: 0,
    status: IuranStatus.belumBayar,
  );
  return MaterialApp(
    home: ProviderScope(
      overrides: [
        billDetailProvider('test-bill').overrideWith((ref) async => testBill),
        billPaymentsProvider('test-bill').overrideWith((ref) async => payments),
        canVerifyPaymentProvider.overrideWith((ref) => false),
        pendingPaymentsProvider(true).overrideWith((ref) async => const []),
      ],
      child: const IuranDetailScreen(billId: 'test-bill'),
    ),
  );
}

Widget _buildDetailScreenWithError({required Object error}) {
  final testBill = IuranBill(
    id: 'test-bill',
    householdName: 'Rumah 01 — Budi Santoso',
    iuranType: 'Keamanan',
    periode: '2026-10',
    nominal: 50000,
    paidAmount: 0,
    status: IuranStatus.belumBayar,
  );
  return MaterialApp(
    home: ProviderScope(
      overrides: [
        billDetailProvider('test-bill').overrideWith((ref) async => testBill),
        billPaymentsProvider('test-bill').overrideWith((ref) async => throw error),
        canVerifyPaymentProvider.overrideWith((ref) => false),
        pendingPaymentsProvider(true).overrideWith((ref) async => const []),
      ],
      child: const IuranDetailScreen(billId: 'test-bill'),
    ),
  );
}

Widget _buildDetailScreenWithBill({required IuranBill bill}) {
  return MaterialApp(
    home: ProviderScope(
      overrides: [
        billDetailProvider(bill.id).overrideWith((ref) async => bill),
        billPaymentsProvider(bill.id).overrideWith((ref) async => const []),
        canVerifyPaymentProvider.overrideWith((ref) => false),
        pendingPaymentsProvider(true).overrideWith((ref) async => const []),
      ],
      child: IuranDetailScreen(billId: bill.id),
    ),
  );
}

void main() {
  group('IuranDetailScreen — B1 overflow regression', () {
    testWidgets(
      'payment history Row renders with long paymentNo without overflow',
      (tester) async {
        final longPaymentNo = 'PAY-20261012-00001';

        final payments = <BackendPayment>[
          BackendPayment(
            id: 'pay-1',
            rtId: 'rt-uuid-1',
            billId: 'test-bill',
            amount: '50000',
            paidAt: DateTime(2026, 10, 10),
            method: PaymentMethod.transfer,
            origin: PaymentOrigin.selfSubmitted,
            status: PaymentStatus.approved,
            paymentNo: longPaymentNo,
          ),
        ];

        await tester.pumpWidget(_buildDetailScreen(payments: payments));
        await tester.pumpAndSettle();

        // The key assertion: no RenderFlex overflow exception was thrown.
        // flutter_test would have thrown if a Row exceeded constraints.
        // Just verify the key widgets are present:
        expect(find.textContaining(longPaymentNo), findsOneWidget);
        expect(find.textContaining('Transfer'), findsOneWidget);
      },
    );

    testWidgets('payment history Row renders with long notes without overflow', (
      tester,
    ) async {
      final payments = <BackendPayment>[
        BackendPayment(
          id: 'pay-2',
          rtId: 'rt-uuid-1',
          billId: 'test-bill',
          amount: '50000',
          paidAt: DateTime(2026, 10, 10),
          method: PaymentMethod.transfer,
          origin: PaymentOrigin.selfSubmitted,
          status: PaymentStatus.pending,
          paymentNo: 'PAY-20261012-00001',
          notes:
              'Pembayaran untuk bulan Oktober 2026 — transfer dari Bank BCA a.n. Budi Santoso',
        ),
      ];

      await tester.pumpWidget(_buildDetailScreen(payments: payments));
      await tester.pumpAndSettle();

      // No overflow exception should be thrown
      expect(find.textContaining('PAY-20261012-00001'), findsOneWidget);
      expect(find.byType(InkWell), findsWidgets);
    });
  });

  group('PaymentProofThumbnail — full image dialog overflow', () {
    /// Generate a minimal 1x1 red PNG for test images.
    Uint8List makeRedPng() {
      // Minimal valid PNG: 1x1 red pixel, then scaled by InteractiveViewer.
      // In practice, flutter_test can render any valid PNG bytes.
      // Use a tiny base64-encoded 1x1 red PNG.
      return Uint8List.fromList(
        base64Decode(
          'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==',
        ),
      );
    }

    testWidgets(
      'full image dialog does not overflow with long title on narrow screen',
      (tester) async {
        // Extremely long title that would exceed 363px dialog width.
        final longTitle =
            'Bukti Pembayaran Rp 1.000.000 untuk bulan Oktober dua ribu dua puluh enam transaksi dari Bank BCA a.n. Budi Santoso';

        await tester.pumpWidget(
          MaterialApp(
            home: ProviderScope(
              overrides: [
                paymentProofBytesProvider.overrideWith(
                  (ref, _) async => makeRedPng(),
                ),
              ],
              child: Builder(
                builder: (context) {
                  return Scaffold(
                    body: SizedBox(
                      width: 360, // narrow Android screen
                      child: PaymentProofThumbnail(
                        paymentId: 'test-payment',
                        size: 56,
                        label: longTitle,
                      ),
                    ),
                  );
                },
              ),
            ),
          ),
        );

        await tester.pumpAndSettle();
        expect(find.byType(InkWell), findsOneWidget);
        await tester.tap(find.byType(InkWell));
        await tester.pumpAndSettle();

        // The title text should be visible (truncated) and no overflow.
        expect(find.textContaining('Bukti Pembayaran'), findsOneWidget);
        expect(find.byType(IconButton), findsOneWidget);

        // The key assertion: no RenderFlex overflow exception should be thrown.
        // flutter_test throws if constraints are exceeded.
      },
    );
  });

  group('IuranDetailScreen — Voluntary and Error states', () {
    testWidgets('renders error card with Coba Lagi button on payment error', (tester) async {
      await tester.pumpWidget(_buildDetailScreenWithError(error: Exception('Network timeout')));
      await tester.pumpAndSettle();

      expect(find.text('Gagal memuat riwayat pembayaran'), findsOneWidget);
      expect(find.text('Coba Lagi'), findsOneWidget);
    });

    testWidgets('renders Status Sukarela for voluntary bill', (tester) async {
      final voluntaryBill = IuranBill(
        id: 'vol-bill',
        householdName: 'Rumah 01 — Budi Santoso',
        iuranType: 'Kerja Bakti',
        periode: '2026-12',
        nominal: 0,
        paidAmount: 0,
        status: IuranStatus.sukarela,
        isVoluntary: true,
      );
      await tester.pumpWidget(_buildDetailScreenWithBill(bill: voluntaryBill));
      await tester.pumpAndSettle();

      expect(find.text('Sukarela'), findsWidgets);
      expect(find.text('Sudah Lunas'), findsNothing);
    });
  });
}
