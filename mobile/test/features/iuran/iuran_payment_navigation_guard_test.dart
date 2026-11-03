// Widget tests for Iuran Payment Screen navigation guard.
//
// Tests:
// - Batal button shows confirmation dialog
// - Batal → "Tidak" keeps form
// - PopScope / back navigation is properly configured

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:social_finance/core/navigators/kas_form_navigation_guard.dart';
import 'package:social_finance/features/iuran/data/iuran_api_client.dart';
import 'package:social_finance/features/iuran/data/iuran_api_models.dart';
import 'package:social_finance/features/iuran/data/iuran_models.dart';
import 'package:social_finance/features/iuran/data/iuran_providers.dart';
import 'package:social_finance/features/iuran/data/iuran_repository.dart';
import 'package:social_finance/features/iuran/presentation/screens/iuran_payment_screen.dart';

// ─── Fake Repository ─────────────────────────────────────────────────

class _FakeIuranRepository extends IuranRepository {
  _FakeIuranRepository()
      : super(apiClient: IuranApiClient(dio: Dio(BaseOptions(baseUrl: 'http://test.local'))));

  @override
  Future<List<BackendDue>> fetchDues({bool? isActive}) async => [];
  @override
  Future<List<IuranBill>> fetchAllBills({
    String? search,
    String? status,
    String? paymentStatus,
    bool myBills = false,
  }) async => [];
}

// ─── Helpers ─────────────────────────────────────────────────────────

GlobalKey<NavigatorState> _createNavigatorKey() => GlobalKey<NavigatorState>();

/// Build a test app with a unique navigator key and bill ID.
Widget _buildTestApp(GlobalKey<NavigatorState> navigatorKey, String billId) {
  return ProviderScope(
    key: ValueKey('iuran-payment-test-${DateTime.now().millisecondsSinceEpoch}'),
    overrides: [
      iuranRepositoryProvider.overrideWithValue(_FakeIuranRepository()),
    ],
    child: MaterialApp.router(
      routerConfig: GoRouter(
        initialLocation: '/home/iuran/pembayaran/$billId',
        navigatorKey: navigatorKey,
        routes: [
          GoRoute(
            path: '/home/iuran/pembayaran/:billId',
            pageBuilder: (context, state) {
              final bid = state.pathParameters['billId']!;
              return MaterialPage(
                child: IuranPaymentScreen(billId: bid),
              );
            },
          ),
        ],
      ),
    ),
  );
}

/// Scroll down so the bottom buttons (Batal, Bayar) become visible.
Future<void> _scrollToBottom(WidgetTester tester) async {
  await tester.dragFrom(
    const Offset(400, 300),
    const Offset(0, -800),
  );
  await tester.pumpAndSettle();
}

// ─── Tests ───────────────────────────────────────────────────────────

void main() {
  setUp(() {
    KasFormNavigationGuard.resetForTesting();
  });

  group('IuranPaymentScreen — Batal button triggers guard dialog', () {
    testWidgets('Batal button shows confirmation dialog', (tester) async {
      final navigatorKey = _createNavigatorKey();
      await tester.pumpWidget(_buildTestApp(navigatorKey, 'test-bill-id-1'));
      await tester.pumpAndSettle();

      // Scroll to Batal button
      await _scrollToBottom(tester);
      await tester.tap(find.text('Batal'));
      await tester.pumpAndSettle();

      // Dialog shows "Apakah Anda yakin ingin membatalkan?" (same guard for all forms)
      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsOneWidget);
      expect(find.text('Tidak, Tetap di Halaman'), findsOneWidget);
    });

    testWidgets('Batal → "Tidak" keeps form', (tester) async {
      final navigatorKey = _createNavigatorKey();
      await tester.pumpWidget(_buildTestApp(navigatorKey, 'test-bill-id-2'));
      await tester.pumpAndSettle();

      await _scrollToBottom(tester);
      await tester.tap(find.text('Batal'));
      await tester.pumpAndSettle();

      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsOneWidget);

      // Tap "Tidak, Tetap di Halaman"
      await tester.tap(find.text('Tidak, Tetap di Halaman'));
      await tester.pumpAndSettle();

      // Dialog dismissed, form still visible
      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsNothing);
    });
  });

  group('IuranPaymentScreen — PopScope / back navigation', () {
    testWidgets('PopScope intercepts back navigation — triggers dialog', (
      tester,
    ) async {
      final navigatorKey = _createNavigatorKey();
      await tester.pumpWidget(_buildTestApp(navigatorKey, 'test-bill-id-3'));
      await tester.pumpAndSettle();

      // Simulate system back via handlePopRoute
      await tester.binding.handlePopRoute();
      await tester.pumpAndSettle();

      // Dialog should appear
      expect(
        find.text('Apakah Anda yakin ingin membatalkan?'),
        findsOneWidget,
      );
      expect(find.text('Tidak, Tetap di Halaman'), findsOneWidget);
    });
  });
}
