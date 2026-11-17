// Widget tests for Iuran Form Screen navigation guard.
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
import 'package:social_finance/features/iuran/presentation/screens/iuran_form_screen.dart';

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

Widget _buildApp(GlobalKey<NavigatorState> navigatorKey) {
  return ProviderScope(
    key: ValueKey('iuran-form-test-${DateTime.now().millisecondsSinceEpoch}'),
    overrides: [
      iuranRepositoryProvider.overrideWithValue(_FakeIuranRepository()),
    ],
    child: MaterialApp.router(
      routerConfig: GoRouter(
        initialLocation: '/home/iuran/form',
        navigatorKey: navigatorKey,
        routes: [
          GoRoute(
            path: '/home/iuran/form',
            pageBuilder: (context, state) => MaterialPage(
              child: IuranFormScreen(),
            ),
          ),
        ],
      ),
    ),
  );
}

/// Scroll to make the bottom area visible.
Future<void> _scrollToBottom(WidgetTester tester) async {
  for (int i = 0; i < 4; i++) {
    await tester.dragFrom(
      const Offset(400, 300),
      const Offset(0, -400),
    );
    await tester.pumpAndSettle();
  }
}

// ─── Tests ───────────────────────────────────────────────────────────

void main() {
  setUp(() {
    KasFormNavigationGuard.resetForTesting();
  });

  group('IuranFormScreen — Batal button triggers guard dialog', () {
    testWidgets('Batal button shows confirmation dialog', (tester) async {
      final navigatorKey = _createNavigatorKey();
      await tester.pumpWidget(_buildApp(navigatorKey));
      await tester.pumpAndSettle();

      // Scroll to bottom to find Batal button
      await _scrollToBottom(tester);
      await tester.pumpAndSettle();

      // Tap Batal
      final batalFinder = find.text('Batal');
      expect(batalFinder, findsWidgets);

      await tester.tap(batalFinder.first);
      await tester.pumpAndSettle();

      // Dialog should appear — guard uses SAME dialog text for ALL forms
      expect(
        find.text('Apakah Anda yakin ingin membatalkan?'),
        findsOneWidget,
      );
      expect(find.text('Tidak, Tetap di Halaman'), findsOneWidget);
    });

    testWidgets('Batal → "Tidak" keeps form', (tester) async {
      final navigatorKey = _createNavigatorKey();
      await tester.pumpWidget(_buildApp(navigatorKey));
      await tester.pumpAndSettle();

      // Scroll to bottom
      await _scrollToBottom(tester);
      await tester.pumpAndSettle();

      await tester.tap(find.text('Batal').first);
      await tester.pumpAndSettle();

      // Dialog exists
      expect(
        find.text('Apakah Anda yakin ingin membatalkan?'),
        findsOneWidget,
      );

      // Tap "Tidak, Tetap di Halaman"
      await tester.tap(find.text('Tidak, Tetap di Halaman'));
      await tester.pumpAndSettle();

      // Dialog dismissed, form still visible
      expect(
        find.text('Apakah Anda yakin ingin membatalkan?'),
        findsNothing,
      );
      expect(find.text('Batal'), findsOneWidget);
    });
  });

  group('IuranFormScreen — PopScope / back navigation', () {
    testWidgets('PopScope intercepts back navigation — triggers dialog', (
      tester,
    ) async {
      final navigatorKey = _createNavigatorKey();
      await tester.pumpWidget(_buildApp(navigatorKey));
      await tester.pumpAndSettle();

      // Simulate system back via handlePopRoute (Batal not tapped)
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
