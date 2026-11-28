// Widget tests for Warga Edit Screen navigation guard.
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
import 'package:social_finance/features/warga/presentation/screens/household_edit_screen.dart';

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
    key: ValueKey('warga-edit-test-${DateTime.now().millisecondsSinceEpoch}'),
    overrides: [
      wargaListProvider.overrideWith(_StaticWargaListNotifier.new),
    ],
    child: MaterialApp.router(
      routerConfig: GoRouter(
        initialLocation: '/home/warga/edit/household/abc123',
        navigatorKey: navigatorKey,
        routes: [
          GoRoute(
            path: '/home/warga/edit/household/:householdId',
            pageBuilder: (context, state) {
              final householdId = state.pathParameters['householdId'];
              return MaterialPage(
                child: HouseholdEditScreen(householdId: householdId ?? 'abc123'),
              );
            },
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

  group('HouseholdEditScreen — Batal button triggers guard dialog', () {
    testWidgets('Batal button shows confirmation dialog when dirty', (tester) async {
      final navigatorKey = _createNavigatorKey();
      await tester.pumpWidget(_buildApp(navigatorKey));
      await tester.pumpAndSettle();

      // Edit a field to make form dirty
      await tester.enterText(find.byType(TextFormField).first, 'Alamat Baru');
      await tester.pump();

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
      expect(find.text('Tetap di Halaman'), findsOneWidget);
      expect(find.text('Ya, Batalkan'), findsOneWidget);
    });

    testWidgets('Batal → "Tetap di Halaman" keeps form', (tester) async {
      final navigatorKey = _createNavigatorKey();
      await tester.pumpWidget(_buildApp(navigatorKey));
      await tester.pumpAndSettle();

      await tester.enterText(find.byType(TextFormField).first, 'Alamat Baru');
      await tester.pump();

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

      // Tap "Tetap di Halaman"
      await tester.tap(find.text('Tetap di Halaman'));
      await tester.pumpAndSettle();

      // Dialog dismissed, form still visible with input preserved
      expect(
        find.text('Apakah Anda yakin ingin membatalkan?'),
        findsNothing,
      );
      await tester.dragFrom(const Offset(400, 300), const Offset(0, 1200));
      await tester.pumpAndSettle();
      expect(find.text('Alamat Baru'), findsOneWidget);
    });
  });

  group('HouseholdEditScreen — PopScope / back navigation', () {
    testWidgets('PopScope intercepts back navigation when dirty — triggers dialog', (
      tester,
    ) async {
      final navigatorKey = _createNavigatorKey();
      await tester.pumpWidget(_buildApp(navigatorKey));
      await tester.pumpAndSettle();

      await tester.enterText(find.byType(TextFormField).first, 'Alamat Baru');
      await tester.pump();

      // Simulate system back via handlePopRoute
      await tester.binding.handlePopRoute();
      await tester.pumpAndSettle();

      // Dialog should appear
      expect(
        find.text('Apakah Anda yakin ingin membatalkan?'),
        findsOneWidget,
      );
      expect(find.text('Tetap di Halaman'), findsOneWidget);
    });
  });
}
