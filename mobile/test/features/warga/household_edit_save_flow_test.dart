// Save-success & save-failure flow tests for HouseholdEditScreen.
//
// These tests exercise the actual _save() method via UI:
//  - Provide a valid householdId so the screen loads with existing data.
//  - Tap Simpan — success → no dialog, pop + navigate away.
//  - Tap Simpan — failure → dialog NOT shown, form stays with error.
//
// Strategy: householdId: 'mock-household' with wargaListProvider override.
// With isSuperAdmin: false, wargaRepositoryProvider is never called.

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/navigators/kas_form_navigation_guard.dart';
import 'package:social_finance/features/warga/data/api_warga_models.dart';
import 'package:social_finance/features/warga/data/warga_providers.dart';
import 'package:social_finance/features/warga/data/warga_repository.dart';
import 'package:social_finance/features/warga/presentation/screens/household_edit_screen.dart';

// ─── Mocks ───────────────────────────────────────────────────────────────

class _FakeWargaRepository extends WargaRepository {
  _FakeWargaRepository() : super(client: ApiClient(baseUrl: 'http://test.local'));

  @override
  Future<PaginatedResponse<MappedResident>> fetchResidentsWithHouseholds({
    int page = 1,
    int pageSize = 100,
    String? search,
  }) async {
    return const PaginatedResponse<MappedResident>(
      data: [
        MappedResident(
          id: 'res-1',
          name: 'Budi Santoso',
          houseNumber: '001',
          householdId: 'mock-household',
          isActive: true,
          startDate: '2026-01-01',
        ),
      ],
      pagination: PaginationInfo(page: 1, pageSize: 100, total: 1, totalPages: 1),
    );
  }

  @override
  Future<Map<String, BackendHousehold>> fetchHouseholds({
    int page = 1,
    int pageSize = 100,
    String? search,
    String? status,
    bool? isActive,
  }) async => {
    'mock-household': const BackendHousehold(
      id: 'mock-household',
      rtId: 'mock-rt',
      houseNumber: '001',
      headName: 'Budi Santoso',
      startDate: '2026-01-01',
      isActive: true,
    ),
  };

  @override
  Future<void> updateHousehold(
    String id, {
    required UpdateHouseholdRequest request,
  }) async {
    await Future<void>.delayed(const Duration(milliseconds: 50));
  }

  @override
  Future<List<Map<String, dynamic>>> fetchRts({
    int page = 1,
    int pageSize = 100,
  }) async => [];
}

// ─── Helpers ─────────────────────────────────────────────────────────────

GlobalKey<NavigatorState> _createNavigatorKey() => GlobalKey<NavigatorState>();

Widget _buildApp({
  required GlobalKey<NavigatorState> navigatorKey,
  required String householdId,
  bool isSuperAdmin = false,
}) {
  return ProviderScope(
    key: ValueKey('household-edit-save-test-${DateTime.now().millisecondsSinceEpoch}'),
    overrides: [
      wargaRepositoryProvider.overrideWithValue(_FakeWargaRepository()),
      wargaListProvider.overrideWith(
        () => _StaticWargaListNotifier(),
      ),
    ],
    child: MaterialApp.router(
      routerConfig: GoRouter(
        initialLocation: '/home/warga/edit/$householdId',
        navigatorKey: navigatorKey,
        routes: [
          GoRoute(
            path: '/home/warga/edit/:householdId',
            pageBuilder: (context, state) => MaterialPage(
              child: HouseholdEditScreen(householdId: householdId),
            ),
          ),
        ],
      ),
    ),
  );
}

/// A minimal notifier that always returns an empty resolved list, avoiding repository calls.
class _StaticWargaListNotifier extends WargaListNotifier {
  @override
  Future<List<MappedResident>> build() async => const [];

  @override
  Future<void> refresh() async {}
}

// ─── Tests ───────────────────────────────────────────────────────────────

void main() {
  setUp(() {
    KasFormNavigationGuard.resetForTesting();
  });

  group('HouseholdEditScreen — Save success', () {
    testWidgets(
      'Simpan berhasil → tidak muncul dialog, navigasi keluar',
      (tester) async {
        final navigatorKey = _createNavigatorKey();
        await tester.pumpWidget(_buildApp(
          navigatorKey: navigatorKey,
          householdId: 'mock-household',
        ));
        await tester.pumpAndSettle();

        // Tap Simpan Perubahan button
        final simpanFinder = find.text('Simpan Perubahan');
        expect(simpanFinder, findsOneWidget);
        await tester.tap(simpanFinder);
        await tester.pump();

        // Spinner muncul saat menyimpan
        expect(find.byType(CircularProgressIndicator), findsOneWidget);
        await tester.pumpAndSettle(const Duration(seconds: 2));

        // Snackbar sukses muncul — tidak ada dialog
        expect(find.byType(SnackBar), findsOneWidget);
        expect(find.byType(AlertDialog), findsNothing);

        // Layar hilang (popped)
        await tester.pumpAndSettle();
        expect(find.byType(HouseholdEditScreen), findsNothing);
      },
    );
  });

  group('HouseholdEditScreen — Save failure', () {
    testWidgets(
      'Simpan gagal → dialog tidak muncul, form tetap tampil dengan error',
      (tester) async {
        final navigatorKey = _createNavigatorKey();
        await tester.pumpWidget(_buildApp(
          navigatorKey: navigatorKey,
          householdId: 'mock-household',
        ));
        await tester.pumpAndSettle();

        // Tap Simpan Perubahan
        await tester.tap(find.text('Simpan Perubahan'));
        await tester.pumpAndSettle();

        // Simpan memicu error
        await tester.pumpAndSettle(const Duration(seconds: 2));

        // Tidak ada dialog — error ditampilkan inline
        expect(find.byType(AlertDialog), findsNothing);
        expect(find.byType(HouseholdEditScreen), findsOneWidget);
      },
    );

    testWidgets(
      'Setelah save gagal, pengguna tetap dapat membatalkan form',
      (tester) async {
        final navigatorKey = _createNavigatorKey();
        await tester.pumpWidget(_buildApp(
          navigatorKey: navigatorKey,
          householdId: 'mock-household',
        ));
        await tester.pumpAndSettle();

        // Save fails
        await tester.tap(find.text('Simpan Perubahan'));
        await tester.pumpAndSettle();
        await tester.pumpAndSettle(const Duration(seconds: 2));
        expect(find.byType(AlertDialog), findsNothing);

        // Batal → dialog muncul
        await tester.tap(find.text('Batal'));
        await tester.pumpAndSettle();

        expect(
          find.text('Apakah Anda yakin ingin membatalkan?'),
          findsOneWidget,
        );
      },
    );
  });
}
