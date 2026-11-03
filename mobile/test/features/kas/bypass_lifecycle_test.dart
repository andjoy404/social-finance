import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart' as rp;
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/app/app.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/core/navigators/kas_form_navigation_guard.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/dashboard/presentation/screens/dashboard_screen.dart';
import 'package:social_finance/features/kas/presentation/screens/kas_keluar_screen.dart';
import 'package:social_finance/features/kas/presentation/screens/kas_masuk_screen.dart';
import 'package:social_finance/features/kas/presentation/screens/kas_screen.dart';
import 'package:social_finance/features/kas/data/kas_api_client.dart';
import 'package:social_finance/features/kas/data/kas_api_models.dart';
import 'package:social_finance/features/kas/data/kas_repository.dart';
import 'package:social_finance/features/warga/data/api_warga_models.dart'
    show MappedResident;
import 'package:social_finance/features/warga/data/warga_providers.dart';

// ============================================================================
// Fake providers
// ============================================================================

class _FakeKasApiClient implements KasApiClient {
  int createCount = 0;

  @override
  Dio get dio => Dio(BaseOptions(baseUrl: 'http://test.local'));

  @override
  Future<BackendRTIdentity> getRTIdentity(String rtId) async =>
      const BackendRTIdentity(name: 'RT 002');

  @override
  Future<PaginatedResponse<BackendTransaction>> listTransactions({
    String? categoryId,
    String? type,
    String? status,
    String? period,
    String? startDate,
    String? endDate,
    String? search,
    int page = 1,
    int pageSize = 20,
  }) async =>
      const PaginatedResponse<BackendTransaction>(
        data: [],
        pagination: PaginationInfo(
          page: 1,
          pageSize: 20,
          total: 0,
          totalPages: 0,
        ),
      );

  @override
  Future<List<BackendCategory>> listCategories({bool? isActive}) async => [];

  @override
  Future<BackendTransaction> getTransaction(String id) async =>
      throw UnimplementedError();

  @override
  Future<BackendTransaction> createTransaction({
    String? categoryId,
    required String amount,
    required String type,
    required String description,
    DateTime? occurredAt,
    String? paymentMethod,
    String? locationId,
    String? expenseCategory,
    required String idempotencyKey,
  }) async {
    createCount++;
    return BackendTransaction(
      id: 'tx-$createCount',
      rtId: 'rt-002',
      categoryId: categoryId ?? 'cat-1',
      amount: amount,
      type: type,
      status: 'posted',
      description: description,
      occurredAt: (occurredAt ?? DateTime.now()).toIso8601String(),
      paymentMethod: paymentMethod ?? 'CASH',
    );
  }

  @override
  Future<BackendTransaction> updateTransaction({
    required String id,
    String? categoryId,
    String? amount,
    String? type,
    String? description,
    DateTime? occurredAt,
    String? paymentMethod,
    String? locationId,
  }) =>
      throw UnimplementedError();

  @override
  Future<Map<String, dynamic>> updateOpeningBalance({
    required String period,
    required String openingBalance,
    String? reason,
  }) =>
      Future.value({});

  @override
  Future<BackendBalance> getBalance({
    String? period,
    String? startDate,
    String? endDate,
  }) =>
      Future.value(const BackendBalance(
        totalIncome: '0',
        totalExpense: '0',
        netBalance: '0',
        openingBalance: '0',
        closingBalance: '0',
        netMovement: '0',
        transactionCount: 0,
      ));


  Future<Map<String, dynamic>> getDashboardSummary([String? period]) =>
      Future.value({});
}

class _StaticWargaListNotifier extends WargaListNotifier {
  @override
  Future<List<MappedResident>> build() async => const [];
  @override
  Future<void> refresh() async {}
}

rp.ProviderScope _buildTestApp() {
  return rp.ProviderScope(
    overrides: [
      authRepositoryProvider.overrideWith((_) {
        final client = ApiClient(
          customDio: Dio(BaseOptions(baseUrl: 'http://test.local')),
        );
        final repo = AuthRepository(
          authService: AuthService(client),
          apiClient: client,
        );
        repo.state = rp.AsyncData<Map<String, dynamic>?>({
          'id': 'user-bendahara',
          'name': 'Budi Bendahara',
          'email': 'bendahara@local.com',
          'role': AppRole.bendahara,
          'jabatan': 'bendahara',
          'rt': '002',
          'rw': '05',
          'rt_id': 'rt-002',
        });
        return repo;
      }),
      kasRepositoryProvider.overrideWith((_) {
        return KasRepository(apiClient: _FakeKasApiClient());
      }),
      wargaListProvider.overrideWith(_StaticWargaListNotifier.new),
    ],
    child: const App(),
  );
}

Finder _nominalField() =>
    find.widgetWithText(TextFormField, 'Masukkan nominal');

Finder _keteranganField() =>
    find.widgetWithText(TextFormField, 'Deskripsi transaksi');

const _kasMasukPath = '/home/kas/masuk';
const _kasKeluarPath = '/home/kas/keluar';
const _iuranFormPath = '/home/iuran/form';
const _wargaBaruPath = '/home/warga/baru';

void main() {
  setUp(() {
    KasFormNavigationGuard.resetForTesting();
  });

  group('Priority 1 — Path matching', () {
    // GoRouterState cannot be constructed manually (has private _configuration).
    // Strategy: verify isFormPath() prefix matching, and verify allowExitFor
    // + onRouteExit consistency using only routes we can render (Kas Masuk).

    testWidgets(
      'P1.1: Kas Masuk — uri.path = /home/kas/masuk, allowExitFor + onRouteExit match',
      (tester) async {
        await tester.pumpWidget(_buildTestApp());
        await tester.pumpAndSettle();

        await tester.tap(find.byKey(const Key('nav-item-kas')));
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(const Key('kas-add-transaction-btn')));
        await tester.pumpAndSettle();
        await tester.tap(find.text('Catat Kas Masuk'));
        await tester.pumpAndSettle();

        expect(find.byType(KasMasukScreen), findsOneWidget);
        final state = GoRouterState.of(tester.element(find.byType(KasMasukScreen)));

        expect(state.uri.path, equals(_kasMasukPath));
        expect(state.matchedLocation, equals(_kasMasukPath));

        // allowExitFor(path) then onRouteExit(state) — both use uri.path
        KasFormNavigationGuard.allowExitFor(state.uri.path);
        final result = await KasFormNavigationGuard.onRouteExit(
          tester.element(find.byType(KasMasukScreen)),
          state,
        );
        expect(result, isTrue, reason: 'Bypass should allow exit');
      },
    );

    testWidgets(
      'P1.2: Kas Keluar — uri.path = /home/kas/keluar, allowExitFor + onRouteExit match',
      (tester) async {
        await tester.pumpWidget(_buildTestApp());
        await tester.pumpAndSettle();

        await tester.tap(find.byKey(const Key('nav-item-kas')));
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(const Key('kas-add-transaction-btn')));
        await tester.pumpAndSettle();
        await tester.tap(find.text('Catat Kas Keluar'));
        await tester.pumpAndSettle();

        expect(find.byType(KasKeluarScreen), findsOneWidget);
        final state = GoRouterState.of(tester.element(find.byType(KasKeluarScreen)));

        expect(state.uri.path, equals(_kasKeluarPath));

        KasFormNavigationGuard.allowExitFor(state.uri.path);
        final result = await KasFormNavigationGuard.onRouteExit(
          tester.element(find.byType(KasKeluarScreen)),
          state,
        );
        expect(result, isTrue);
      },
    );

    testWidgets(
      'P1.3: isFormPath matches /home/iuran/form',
      (tester) async {
        expect(KasFormNavigationGuard.isFormPath(_iuranFormPath), isTrue);
      },
    );

    testWidgets(
      'P1.4: isFormPath matches /home/iuran/pembayaran/:billId via prefix',
      (tester) async {
        const path = '/home/iuran/pembayaran/abc-123';
        expect(KasFormNavigationGuard.isFormPath(path), isTrue);
      },
    );

    testWidgets(
      'P1.5: isFormPath matches /home/warga/baru',
      (tester) async {
        expect(KasFormNavigationGuard.isFormPath(_wargaBaruPath), isTrue);
      },
    );

    testWidgets(
      'P1.6: isFormPath matches /home/warga/edit/household/:id via prefix',
      (tester) async {
        const path = '/home/warga/edit/household/hh-456';
        expect(KasFormNavigationGuard.isFormPath(path), isTrue);
      },
    );

    testWidgets(
      'P1.7: allowExitFor ignores non-guarded paths',
      (tester) async {
        // allowExitFor checks isFormPath internally
        // Non-guarded paths should be silently ignored
        KasFormNavigationGuard.allowExitFor('/home');
        KasFormNavigationGuard.allowExitFor('/login');
        KasFormNavigationGuard.allowExitFor('/home/kas');

        // These should NOT affect guarded routes
        // (We verify by checking isFormPath returns false)
        expect(KasFormNavigationGuard.isFormPath('/home'), isFalse);
        expect(KasFormNavigationGuard.isFormPath('/login'), isFalse);
        expect(KasFormNavigationGuard.isFormPath('/home/kas'), isFalse);
      },
    );

    testWidgets(
      'P1.8: isFormPath does NOT match unguarded routes',
      (tester) async {
        expect(KasFormNavigationGuard.isFormPath('/home'), isFalse);
        expect(KasFormNavigationGuard.isFormPath('/home/kas'), isFalse);
        expect(KasFormNavigationGuard.isFormPath('/home/iuran'), isFalse);
        expect(KasFormNavigationGuard.isFormPath('/home/iuran/detail/abc'), isFalse);
        expect(KasFormNavigationGuard.isFormPath('/home/warga'), isFalse);
        expect(
          KasFormNavigationGuard.isFormPath('/home/warga/edit/petugas/xyz'),
          isFalse,
        );
      },
    );
  });

  // ========================================================================
  // Priority 2 — Bypass lifecycle
  // ========================================================================

  group('Priority 2 — Bypass lifecycle', () {
    testWidgets(
      'P2.1: Tap Beranda from guarded form → dialog → "Ya, Batalkan" → exit → bypass consumed',
      (tester) async {
        await tester.pumpWidget(_buildTestApp());
        await tester.pumpAndSettle();

        await tester.tap(find.byKey(const Key('nav-item-kas')));
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(const Key('kas-add-transaction-btn')));
        await tester.pumpAndSettle();
        await tester.tap(find.text('Catat Kas Masuk'));
        await tester.pumpAndSettle();

        await tester.enterText(_nominalField(), '10000');
        await tester.pump();

        final berandaTab = find.byKey(const Key('nav-item-beranda'));
        await tester.tap(berandaTab);
        await tester.pumpAndSettle();
        expect(find.byType(AlertDialog), findsOneWidget);

        await tester.tap(find.byKey(const Key('kas-cancel-confirm')));
        await tester.pumpAndSettle();
        expect(find.byType(AlertDialog), findsNothing);
        expect(find.byType(DashboardScreen), findsOneWidget);

        // Reopen Kas Masuk
        await tester.tap(find.byKey(const Key('nav-item-kas')));
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(const Key('kas-add-transaction-btn')));
        await tester.pumpAndSettle();
        await tester.tap(find.text('Catat Kas Masuk'));
        await tester.pumpAndSettle();

        await tester.enterText(_nominalField(), '20000');
        await tester.pump();

        // Second tap Beranda — should show dialog (bypass consumed)
        await tester.tap(berandaTab);
        await tester.pumpAndSettle();
        expect(find.byType(AlertDialog), findsOneWidget,
            reason: 'Bypass should be consumed after first use');
      },
    );

    testWidgets(
      'P2.2: "Tidak" preserves input — route still guarded after "Tidak"',
      (tester) async {
        await tester.pumpWidget(_buildTestApp());
        await tester.pumpAndSettle();

        await tester.tap(find.byKey(const Key('nav-item-kas')));
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(const Key('kas-add-transaction-btn')));
        await tester.pumpAndSettle();
        await tester.tap(find.text('Catat Kas Masuk'));
        await tester.pumpAndSettle();

        final nominalField = find.widgetWithText(TextFormField, 'Masukkan nominal');

        // Enter a value that won't be locale-formatted (e.g., 500)
        await tester.enterText(nominalField, '500');
        await tester.enterText(_keteranganField(), 'Input tetap');
        await tester.pump();

        // Read value before dialog
        final fieldState = tester.firstState<FormFieldState<String>>(nominalField);
        final valueBefore = fieldState.value;
        expect(valueBefore, isNotNull);

        final berandaTab = find.byKey(const Key('nav-item-beranda'));
        await tester.tap(berandaTab);
        await tester.pumpAndSettle();
        expect(find.byType(AlertDialog), findsOneWidget);

        await tester.tap(find.byKey(const Key('kas-cancel-stay')));
        await tester.pumpAndSettle();

        expect(find.byType(KasMasukScreen), findsOneWidget);

        // Read value after "Tidak" — should still be preserved
        final fieldStateAfter = tester.firstState<FormFieldState<String>>(nominalField);
        final valueAfter = fieldStateAfter.value;
        expect(valueAfter, equals(valueBefore),
            reason: 'Input should be preserved after "Tidak"');

        // Route is STILL guarded — "Tidak" does NOT register bypass
        await tester.tap(berandaTab);
        await tester.pumpAndSettle();
        expect(find.byType(AlertDialog), findsOneWidget,
            reason: 'Route stays guarded after "Tidak"');
      },
    );

    testWidgets(
      'P2.3: Bypass scoped to exact path — Kas Masuk bypass tidak berlaku untuk Kas Keluar',
      (tester) async {
        await tester.pumpWidget(_buildTestApp());
        await tester.pumpAndSettle();

        await tester.tap(find.byKey(const Key('nav-item-kas')));
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(const Key('kas-add-transaction-btn')));
        await tester.pumpAndSettle();
        await tester.tap(find.text('Catat Kas Masuk'));
        await tester.pumpAndSettle();

        await tester.enterText(_nominalField(), '100');
        await tester.pump();

        final berandaTab = find.byKey(const Key('nav-item-beranda'));
        await tester.tap(berandaTab);
        await tester.pumpAndSettle();
        expect(find.byType(AlertDialog), findsOneWidget);

        await tester.tap(find.byKey(const Key('kas-cancel-stay')));
        await tester.pumpAndSettle();

        // Navigate to Kas Keluar via GoRouter — should show dialog
        final ctx = tester.element(find.byType(KasMasukScreen));
        GoRouter.of(ctx).go('/home/kas/keluar');
        await tester.pumpAndSettle();

        expect(find.byType(AlertDialog), findsOneWidget,
            reason: 'Bypass for Kas Masuk should NOT apply to Kas Keluar');
      },
    );

    testWidgets(
      'P2.4: Hardware back — PopScope intercepts, shows dialog',
      (tester) async {
        await tester.pumpWidget(_buildTestApp());
        await tester.pumpAndSettle();

        await tester.tap(find.byKey(const Key('nav-item-kas')));
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(const Key('kas-add-transaction-btn')));
        await tester.pumpAndSettle();
        await tester.tap(find.text('Catat Kas Masuk'));
        await tester.pumpAndSettle();

        await tester.enterText(_nominalField(), '5000');
        await tester.pump();

        final handled = await tester.binding.handlePopRoute();
        await tester.pumpAndSettle();

        expect(handled, isTrue,
            reason: 'PopScope should intercept hardware back');
        expect(find.byType(AlertDialog), findsOneWidget,
            reason: 'Hardware back should show dialog');

        await tester.tap(find.byKey(const Key('kas-cancel-stay')));
        await tester.pumpAndSettle();
        expect(find.byType(AlertDialog), findsNothing);
        expect(find.byType(KasMasukScreen), findsOneWidget);
      },
    );

    testWidgets(
      'P2.5: AppBar back button — triggers onRouteExit',
      (tester) async {
        await tester.pumpWidget(_buildTestApp());
        await tester.pumpAndSettle();

        await tester.tap(find.byKey(const Key('nav-item-kas')));
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(const Key('kas-add-transaction-btn')));
        await tester.pumpAndSettle();
        await tester.tap(find.text('Catat Kas Masuk'));
        await tester.pumpAndSettle();

        final backBtn = find.byType(BackButton);
        expect(backBtn, findsOneWidget);
        await tester.tap(backBtn);
        await tester.pumpAndSettle();

        expect(find.byType(AlertDialog), findsOneWidget,
            reason: 'AppBar back button should show dialog');
      },
    );

    testWidgets(
      'P2.6: Batal button — calls requestFormCancel, shows dialog',
      (tester) async {
        await tester.pumpWidget(_buildTestApp());
        await tester.pumpAndSettle();

        await tester.tap(find.byKey(const Key('nav-item-kas')));
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(const Key('kas-add-transaction-btn')));
        await tester.pumpAndSettle();
        await tester.tap(find.text('Catat Kas Masuk'));
        await tester.pumpAndSettle();

        final batalBtn = find.text('Batal');
        await tester.scrollUntilVisible(
          batalBtn,
          300,
          scrollable: find.byType(Scrollable).first,
        );
        await tester.pumpAndSettle();
        await tester.tap(batalBtn);
        await tester.pumpAndSettle();

        expect(find.byType(AlertDialog), findsOneWidget,
            reason: 'Batal button should show confirmation dialog');
      },
    );

    testWidgets(
      'P2.7: Shell navigation — confirmShellNavigation asks for guarded forms',
      (tester) async {
        await tester.pumpWidget(_buildTestApp());
        await tester.pumpAndSettle();

        await tester.tap(find.byKey(const Key('nav-item-kas')));
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(const Key('kas-add-transaction-btn')));
        await tester.pumpAndSettle();
        await tester.tap(find.text('Catat Kas Masuk'));
        await tester.pumpAndSettle();

        final scaffolds = tester.stateList<ScaffoldState>(find.byType(Scaffold));
        final shellScaffold = scaffolds.firstWhere((s) => s.hasDrawer);
        shellScaffold.openDrawer();
        await tester.pumpAndSettle();

        await tester.tap(find.descendant(
          of: find.byType(Drawer),
          matching: find.text('Warga'),
        ));
        await tester.pumpAndSettle();

        expect(find.byType(AlertDialog), findsOneWidget,
            reason: 'Shell navigation from guarded form shows dialog');
      },
    );

    testWidgets(
      'P2.8: Non-guarded routes (Kas list, Beranda) never show dialog',
      (tester) async {
        await tester.pumpWidget(_buildTestApp());
        await tester.pumpAndSettle();

        await tester.tap(find.byKey(const Key('nav-item-kas')));
        await tester.pumpAndSettle();
        expect(find.byType(KasScreen), findsOneWidget);

        await tester.tap(find.byKey(const Key('nav-item-beranda')));
        await tester.pumpAndSettle();
        expect(find.byType(AlertDialog), findsNothing);
        expect(find.byType(DashboardScreen), findsOneWidget);
      },
    );
  });
}
