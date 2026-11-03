// Widget tests for Warga Create Screen navigation guard.
//
// Tests:
// - Batal button shows confirmation dialog
// - Batal → "Tidak" keeps form
// - PopScope / back navigation is properly configured

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:social_finance/core/navigators/kas_form_navigation_guard.dart';
import 'package:social_finance/features/warga/data/api_warga_models.dart';
import 'package:social_finance/features/warga/data/warga_providers.dart';
import 'package:social_finance/features/warga/presentation/screens/household_create_screen.dart';

// ─── Static WargaList ────────────────────────────────────────────────

class _StaticWargaListNotifier extends WargaListNotifier {
  @override
  Future<List<MappedResident>> build() async => const [];

  @override
  Future<void> refresh() async {}
}

// ─── Helpers ─────────────────────────────────────────────────────────

GlobalKey<NavigatorState> _createNavigatorKey() => GlobalKey<NavigatorState>();

Widget _buildApp(GlobalKey<NavigatorState> navigatorKey) {
  return ProviderScope(
    key: ValueKey('warga-create-test-${DateTime.now().millisecondsSinceEpoch}'),
    overrides: [
      wargaListProvider.overrideWith(_StaticWargaListNotifier.new),
    ],
    child: MaterialApp.router(
      routerConfig: GoRouter(
        initialLocation: '/home/warga/baru',
        navigatorKey: navigatorKey,
        routes: [
          GoRoute(
            path: '/home/warga/baru',
            pageBuilder: (context, state) => MaterialPage(
              child: HouseholdCreateScreen(isSuperAdmin: false),
            ),
          ),
        ],
      ),
    ),
  );
}

/// Scroll to make the bottom area visible.
Future<void> _scrollToBottom(WidgetTester tester) async {
  for (int i = 0; i < 3; i++) {
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

  group('HouseholdCreateScreen — Batal button triggers guard dialog', () {
    testWidgets('Batal button shows confirmation dialog', (tester) async {
      final navigatorKey = _createNavigatorKey();
      await tester.pumpWidget(_buildApp(navigatorKey));
      await tester.pumpAndSettle();

      // Scroll to bottom to find Batal button
      await _scrollToBottom(tester);
      await tester.pumpAndSettle();

      // Tap Batal
      final batalFinder = find.text('Batal');
      expect(batalFinder, findsWidgets, reason: 'Batal button should be present');

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
    });
  });

  group('HouseholdCreateScreen — PopScope / back navigation', () {
    testWidgets('PopScope intercepts back navigation — triggers dialog', (
      tester,
    ) async {
      final navigatorKey = _createNavigatorKey();
      await tester.pumpWidget(_buildApp(navigatorKey));
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
