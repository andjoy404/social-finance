// Regression tests for Android system-back navigation on Warga forms.
//
// Scenario (bug report): with the Create/Edit Warga form open, the Android
// system back button used to exit the application instead of popping back to
// the Warga list. These tests simulate the system back button via
// WidgetsBinding.handlePopRoute() — the same entry point the Android engine
// uses for the hardware/gesture back button — and assert that:
//
//   1. The back event is *handled by the app* (an unhandled back event makes
//      the OS close the top activity, i.e. "exit app").
//   2. The user lands back on the Warga list (not the login screen, not exit).
//   3. A second back on Warga tab stays on Warga and shows exit SnackBar.

import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart' as rp;

import 'package:social_finance/app/app.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/warga/data/api_warga_models.dart';
import 'package:social_finance/features/warga/data/warga_providers.dart';
import 'package:social_finance/features/warga/data/warga_repository.dart';
import 'package:social_finance/features/warga/presentation/screens/household_create_screen.dart';
import 'package:social_finance/features/warga/presentation/screens/household_edit_screen.dart';
import 'package:social_finance/features/warga/presentation/screens/warga_screen.dart';

MappedResident _sampleResident() => const MappedResident(
      id: 'resident-1',
      householdId: 'household-1',
      name: 'Budi Santoso',
      nik: '3201234567890001',
      houseNumber: '01',
      isHeadOfHousehold: true,
      relationship: 'Kepala Keluarga',
    );

const _sampleHousehold = BackendHousehold(
  id: 'household-1',
  rtId: 'rt-1',
  houseNumber: '01',
  headName: 'Budi Santoso',
  isActive: true,
);

/// Deterministic repository: no network access in widget tests.
class _FakeWargaRepository extends WargaRepository {
  _FakeWargaRepository() : super(client: ApiClient(customDio: Dio()));

  @override
  Future<PaginatedResponse<MappedResident>> fetchResidentsWithHouseholds({
    int page = 1,
    int pageSize = 100,
    String? search,
  }) async =>
      PaginatedResponse<MappedResident>(
        data: [_sampleResident()],
        pagination:
            PaginationInfo(page: page, pageSize: pageSize, total: 1, totalPages: 1),
      );

  @override
  Future<Map<String, BackendHousehold>> fetchHouseholds({
    int page = 1,
    int pageSize = 100,
  }) async =>
      const {
        'household-1': _sampleHousehold,
      };
}

class _StaticWargaListNotifier extends WargaListNotifier {
  @override
  Future<List<MappedResident>> build() async => [_sampleResident()];

  @override
  Future<void> refresh() async {}
}

rp.ProviderScope _app() {
  return rp.ProviderScope(
    overrides: [
      authRepositoryProvider.overrideWith((_) {
        final client = ApiClient(customDio: Dio());
        final repo = AuthRepository(
          authService: AuthService(client),
          apiClient: client,
        );
        repo.state = rp.AsyncData<Map<String, dynamic>?>({
          'name': 'Budi Santoso',
          'role': AppRole.bendahara,
          'rt': '01',
          'rw': '05',
        });
        return repo;
      }),
      wargaRepositoryProvider.overrideWithValue(_FakeWargaRepository()),
      wargaListProvider.overrideWith(_StaticWargaListNotifier.new),
    ],
    child: const App(),
  );
}

Future<void> _openWargaTab(WidgetTester tester) async {
  await tester.tap(
    find.descendant(
      of: find.byType(BottomAppBar),
      matching: find.text('Warga'),
    ),
  );
  await tester.pumpAndSettle();
}

/// Simulates the Android predictive-back gesture (edge swipe) exactly as the
/// engine delivers it: method calls on the `flutter/backgesture` channel.
Future<void> _sendBackGesture(
  WidgetTester tester,
  String method,
  Map<Object?, Object?> args,
) {
  const codec = StandardMethodCodec();
  final data = codec.encodeMethodCall(MethodCall(method, args));
  return tester.binding.defaultBinaryMessenger.handlePlatformMessage(
    'flutter/backgesture',
    data,
    (ByteData? reply) {},
  );
}

void main() {
  /// Helper: tap "Ya, Batalkan" in the confirmation dialog.
  Future<void> tapDialogConfirm(WidgetTester tester) async {
    await tester.tap(find.text('Ya, Batalkan'));
    await tester.pumpAndSettle();
  }

  /// Helper: tap "Tidak, Tetap di Halaman" in the confirmation dialog.
  Future<void> tapDialogCancel(WidgetTester tester) async {
    await tester.tap(find.text('Tidak, Tetap di Halaman'));
    await tester.pumpAndSettle();
  }

  testWidgets(
    'system back from Tambah Warga — first back shows dialog, "Tidak" stays, "Ya" pops',
    (tester) async {
      await tester.pumpWidget(_app());
      await tester.pumpAndSettle();
      await _openWargaTab(tester);
      expect(find.byType(WargaScreen), findsOneWidget);

      // "Tambah" button (FAB) -> choice dialog -> pick "Warga".
      await tester.tap(
        find.descendant(
          of: find.byType(WargaScreen),
          matching: find.byIcon(Icons.person_add),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(
        find.descendant(
          of: find.byType(Dialog),
          matching: find.text('Warga'),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.byType(HouseholdCreateScreen), findsOneWidget);

      // First Android system back button — should show confirmation dialog, NOT pop.
      final handled = await tester.binding.handlePopRoute();
      await tester.pumpAndSettle();

      expect(
        handled,
        isTrue,
        reason: 'back event was not handled by the app; the OS would exit '
            'the application instead of showing the confirmation dialog',
      );
      // Form is still visible.
      expect(find.byType(HouseholdCreateScreen), findsOneWidget);
      // Dialog is visible.
      expect(find.byType(AlertDialog), findsOneWidget);
      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsOneWidget);
      expect(find.text('Perubahan yang belum disimpan akan hilang.'), findsOneWidget);
      expect(find.text('Tidak, Tetap di Halaman'), findsOneWidget);
      expect(find.text('Ya, Batalkan'), findsOneWidget);

      // Tap "Tidak, Tetap di Halaman" — dialog dismisses, form stays.
      await tapDialogCancel(tester);
      expect(find.byType(HouseholdCreateScreen), findsOneWidget);
      expect(find.byType(AlertDialog), findsNothing);

      // Second back button — dialog appears again.
      final handledAgain = await tester.binding.handlePopRoute();
      await tester.pumpAndSettle();
      expect(handledAgain, isTrue);
      expect(find.byType(HouseholdCreateScreen), findsOneWidget);
      expect(find.byType(AlertDialog), findsOneWidget);

      // Tap "Ya, Batalkan" — form pops, Warga list visible.
      await tapDialogConfirm(tester);
      expect(find.byType(HouseholdCreateScreen), findsNothing);
      expect(find.byType(WargaScreen), findsOneWidget);

      // Under Part 6, back on Warga tab does NOT pop to Beranda.
      // It stays on Warga and shows the exit confirmation SnackBar.
      final handledThird = await tester.binding.handlePopRoute();
      await tester.pumpAndSettle();
      expect(handledThird, isTrue);
      expect(find.byType(WargaScreen), findsOneWidget);
      expect(find.text('Tekan sekali lagi untuk keluar dari aplikasi'), findsOneWidget);
    },
  );

  testWidgets(
    'gesture back (predictive back) from Tambah Warga — shows dialog, then user dismisses',
    (tester) async {
      debugDefaultTargetPlatformOverride = TargetPlatform.android;
      addTearDown(() => debugDefaultTargetPlatformOverride = null);
      try {
        await tester.pumpWidget(_app());
        await tester.pumpAndSettle();
        await _openWargaTab(tester);
        expect(find.byType(WargaScreen), findsOneWidget);

        // Open the create form via the choice dialog.
        await tester.tap(
          find.descendant(
            of: find.byType(WargaScreen),
            matching: find.byIcon(Icons.person_add),
          ),
        );
        await tester.pumpAndSettle();
        await tester.tap(
          find.descendant(
            of: find.byType(Dialog),
            matching: find.text('Warga'),
          ),
        );
        await tester.pumpAndSettle();
        expect(find.byType(HouseholdCreateScreen), findsOneWidget);

        // Android edge-swipe back gesture (predictive back).
        await _sendBackGesture(tester, 'startBackGesture', {
          'touchOffset': <Object?>[50.0, 500.0],
          'progress': 0.5,
          'swipeEdge': 0,
        });
        await tester.pump();
        await _sendBackGesture(tester, 'commitBackGesture', {
          'touchOffset': <Object?>[50.0, 500.0],
          'progress': 1.0,
          'swipeEdge': 0,
        });
        await tester.pumpAndSettle();

        // The confirmation dialog must appear (not the form popping directly).
        expect(find.byType(HouseholdCreateScreen), findsOneWidget);
        expect(find.byType(AlertDialog), findsOneWidget);
        expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsOneWidget);

        // Tap "Ya, Batalkan" to confirm exit.
        await tapDialogConfirm(tester);

        // The form must close and the Warga list must be visible.
        expect(find.byType(HouseholdCreateScreen), findsNothing);
        expect(find.byType(WargaScreen), findsOneWidget);
        // The app shell (bottom nav) must still be mounted.
        expect(find.byType(BottomAppBar), findsOneWidget);
      } finally {
        debugDefaultTargetPlatformOverride = null;
      }
    },
  );

  testWidgets(
    'system back from Edit Warga — shows dialog, "Ya" pops back to Warga list',
    (tester) async {
      await tester.pumpWidget(_app());
      await tester.pumpAndSettle();
      await _openWargaTab(tester);
      expect(find.byType(WargaScreen), findsOneWidget);

      // Edit icon on the resident card.
      final editButton = find.byTooltip('Ubah');
      expect(editButton, findsOneWidget);
      await tester.tap(editButton);
      await tester.pumpAndSettle();

      expect(find.byType(HouseholdEditScreen), findsOneWidget);

      // Android system back button — should show confirmation dialog.
      final handled = await tester.binding.handlePopRoute();
      await tester.pumpAndSettle();

      expect(
        handled,
        isTrue,
        reason: 'back event was not handled by the app; the OS would exit '
            'the application instead of showing the confirmation dialog',
      );
      // Form is still visible.
      expect(find.byType(HouseholdEditScreen), findsOneWidget);
      // Dialog is visible.
      expect(find.byType(AlertDialog), findsOneWidget);
      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsOneWidget);

      // Tap "Ya, Batalkan" — form pops, Warga list visible.
      await tapDialogConfirm(tester);
      expect(find.byType(HouseholdEditScreen), findsNothing);
      expect(find.byType(WargaScreen), findsOneWidget);

      // Under Part 6, back on Warga tab does NOT pop to Beranda.
      // It stays on Warga and shows the exit confirmation SnackBar.
      final handledAgain = await tester.binding.handlePopRoute();
      await tester.pumpAndSettle();
      expect(handledAgain, isTrue);
      expect(find.byType(WargaScreen), findsOneWidget);
      expect(find.text('Tekan sekali lagi untuk keluar dari aplikasi'), findsOneWidget);
    },
  );

  testWidgets(
    'system back on Beranda (/home) stays and shows exit snackbar',
    (tester) async {
      await tester.pumpWidget(_app());
      await tester.pumpAndSettle();

      expect(find.text('Tekan sekali lagi untuk keluar dari aplikasi'), findsNothing);

      final handled = await tester.binding.handlePopRoute();
      await tester.pumpAndSettle();

      expect(handled, isTrue, reason: 'Beranda back must be handled by app');
      expect(find.text('Tekan sekali lagi untuk keluar dari aplikasi'), findsOneWidget);
    },
  );
}
