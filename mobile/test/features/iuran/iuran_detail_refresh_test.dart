import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:social_finance/features/iuran/data/iuran_api_models.dart';
import 'package:social_finance/features/iuran/data/iuran_models.dart';
import 'package:social_finance/features/iuran/data/iuran_providers.dart';
import 'package:social_finance/features/iuran/presentation/screens/iuran_detail_screen.dart';

void main() {
  group('IuranDetailScreen Pull-To-Refresh and Scroll Physics', () {
    testWidgets(
      'renders RefreshIndicator and AlwaysScrollableScrollPhysics even on short content',
      (tester) async {
        final testBill = IuranBill(
          id: 'test-bill-refresh',
          householdName: 'Rumah 10 — Pak Budi',
          iuranType: 'Kebersihan',
          periode: '2026-10',
          nominal: 35000,
          paidAmount: 0,
          status: IuranStatus.belumBayar,
        );

        await tester.pumpWidget(
          MaterialApp(
            home: ProviderScope(
              overrides: [
                billDetailProvider('test-bill-refresh')
                    .overrideWith((ref) async => testBill),
                billPaymentsProvider('test-bill-refresh')
                    .overrideWith((ref) async => const <BackendPayment>[]),
                canVerifyPaymentProvider.overrideWith((ref) => false),
                pendingPaymentsProvider(true).overrideWith((ref) async => const []),
              ],
              child: const IuranDetailScreen(billId: 'test-bill-refresh'),
            ),
          ),
        );

        await tester.pumpAndSettle();

        // Verify RefreshIndicator exists
        expect(find.byType(RefreshIndicator), findsOneWidget);

        // Verify SingleChildScrollView has AlwaysScrollableScrollPhysics
        final scrollView = tester.widget<SingleChildScrollView>(
          find.byType(SingleChildScrollView),
        );
        expect(
          scrollView.physics,
          isA<AlwaysScrollableScrollPhysics>(),
        );

        // Verify household name and nominal are visible
        expect(find.text('Rumah 10 — Pak Budi'), findsWidgets);
        expect(find.text('Kebersihan'), findsWidgets);
      },
    );

    testWidgets(
      'pull-to-refresh preserves loaded state if refresh fails and shows SnackBar',
      (tester) async {
        int fetchCount = 0;
        final testBill = IuranBill(
          id: 'test-bill-fail',
          householdName: 'Rumah 12 — Pak Joko',
          iuranType: 'Keamanan',
          periode: '2026-10',
          nominal: 50000,
          paidAmount: 0,
          status: IuranStatus.belumBayar,
        );

        await tester.pumpWidget(
          MaterialApp(
            home: ProviderScope(
              overrides: [
                billDetailProvider('test-bill-fail').overrideWith((ref) async {
                  fetchCount++;
                  if (fetchCount > 1) {
                    throw Exception('Koneksi timeout');
                  }
                  return testBill;
                }),
                billPaymentsProvider('test-bill-fail')
                    .overrideWith((ref) async => const <BackendPayment>[]),
                canVerifyPaymentProvider.overrideWith((ref) => false),
                pendingPaymentsProvider(true).overrideWith((ref) async => const []),
              ],
              child: const IuranDetailScreen(billId: 'test-bill-fail'),
            ),
          ),
        );

        await tester.pumpAndSettle();
        expect(find.text('Rumah 12 — Pak Joko'), findsWidgets);

        // Trigger refresh
        final refreshIndicator =
            tester.widget<RefreshIndicator>(find.byType(RefreshIndicator));
        await refreshIndicator.onRefresh();
        await tester.pump();

        // Bill should still be visible because cached state is preserved
        expect(find.text('Rumah 12 — Pak Joko'), findsWidgets);

        // SnackBar error should be displayed
        expect(find.byType(SnackBar), findsOneWidget);
        expect(find.textContaining('Koneksi timeout'), findsOneWidget);
      },
    );
  });
}
