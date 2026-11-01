import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/features/iuran/data/iuran_api_models.dart';
import 'package:social_finance/features/iuran/data/iuran_models.dart';
import 'package:social_finance/features/iuran/data/iuran_providers.dart';
import 'package:social_finance/features/iuran/presentation/screens/iuran_detail_screen.dart';
import 'package:social_finance/features/iuran/presentation/screens/iuran_payment_screen.dart';

void main() {
  group('Move-in Date and Obligation Tests', () {
    test('IuranBill model properly computes isInArrears based on isObligatory', () {
      final nonObligatoryBill = IuranBill(
        id: 'bill-1',
        householdName: 'Keluarga Budi',
        iuranType: 'Iuran Wajib RT',
        periode: '2026-05',
        nominal: 50000,
        status: IuranStatus.belumBayar,
        isObligatory: false,
        moveInDate: '2026-10-15',
      );

      final obligatoryBill = IuranBill(
        id: 'bill-2',
        householdName: 'Keluarga Budi',
        iuranType: 'Iuran Wajib RT',
        periode: '2026-10',
        nominal: 50000,
        status: IuranStatus.belumBayar,
        isObligatory: true,
        moveInDate: '2026-10-15',
      );

      expect(nonObligatoryBill.isObligatory, isFalse);
      expect(nonObligatoryBill.isInArrears, isFalse);

      expect(obligatoryBill.isObligatory, isTrue);
      expect(obligatoryBill.isInArrears, isTrue);
    });

    testWidgets('IuranDetailScreen displays Tidak Wajib badge, banner, move-in info, and disabled button', (
      tester,
    ) async {
      final bill = IuranBill(
        id: 'bill-oct',
        householdName: 'Keluarga Agus',
        iuranType: 'Iuran Kebersihan',
        periode: '2026-05',
        nominal: 25000,
        status: IuranStatus.belumBayar,
        isObligatory: false,
        moveInDate: '2026-10-15T00:00:00Z',
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            billDetailProvider('bill-oct').overrideWith((ref) => bill),
            billPaymentsProvider('bill-oct').overrideWith((ref) => <BackendPayment>[]),
          ],
          child: const MaterialApp(
            home: IuranDetailScreen(billId: 'bill-oct'),
          ),
        ),
      );

      await tester.pumpAndSettle();

      // Check badge in header
      expect(find.text('Tidak Wajib'), findsOneWidget);

      // Check explanation banner
      expect(
        find.text(
          'Tagihan ini sebelum tanggal mulai menempati rumah (tidak wajib dibayar & bukan tunggakan).',
        ),
        findsOneWidget,
      );

      // Check information rows
      expect(find.text('Tanggal Mulai Menempati'), findsOneWidget);
      expect(find.text('15 Oktober 2026'), findsOneWidget);
      expect(find.text('Bulan Wajib Dibayar'), findsOneWidget);
      expect(find.text('Oktober–Desember 2026'), findsOneWidget);
      expect(find.text('Jumlah Periode Wajib'), findsOneWidget);
      expect(find.text('3 Bulan'), findsOneWidget);

      // Check disabled pay button
      expect(find.text('Tidak Wajib Bayar'), findsOneWidget);
      expect(find.byIcon(Icons.block), findsOneWidget);
    });

    testWidgets('IuranPaymentScreen displays warning banner when opened for non-obligatory bill', (
      tester,
    ) async {
      final bill = IuranBill(
        id: 'bill-may',
        householdName: 'Keluarga Agus',
        iuranType: 'Iuran Kebersihan',
        periode: '2026-05',
        nominal: 25000,
        status: IuranStatus.belumBayar,
        isObligatory: false,
        moveInDate: '2026-10-15T00:00:00Z',
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            billDetailProvider('bill-may').overrideWith((ref) => bill),
          ],
          child: const MaterialApp(
            home: IuranPaymentScreen(billId: 'bill-may'),
          ),
        ),
      );

      await tester.pumpAndSettle();

      // Warning banner should be displayed
      expect(
        find.text(
          'Tagihan ini sebelum tanggal mulai menempati rumah (tidak wajib dibayar & bukan tunggakan). Pembayaran tidak dapat diproses.',
        ),
        findsOneWidget,
      );

      // Scroll to save button and check it is disabled
      await tester.scrollUntilVisible(
        find.byKey(const Key('btn_save')),
        500,
        scrollable: find.byType(Scrollable).first,
      );
      final btn = tester.widget<OutlinedButton>(find.byKey(const Key('btn_save')));
      expect(btn.onPressed, isNull);
    });
  });
}
