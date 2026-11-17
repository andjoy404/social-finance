import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart' as rp;
import 'package:flutter_test/flutter_test.dart';

import 'package:social_finance/app/app.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/core/navigators/kas_form_navigation_guard.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/fund_location/data/fund_location_api_client.dart';
import 'package:social_finance/features/fund_location/data/fund_location_repository.dart';
import 'package:social_finance/features/fund_location/data/models/fund_location_models.dart';
import 'package:social_finance/features/kas/data/kas_api_client.dart';
import 'package:social_finance/features/kas/data/kas_api_models.dart';
import 'package:social_finance/features/kas/data/kas_repository.dart';
import 'package:social_finance/features/kas/presentation/screens/kas_keluar_screen.dart';
import 'package:social_finance/features/kas/presentation/screens/kas_masuk_screen.dart';
import 'package:social_finance/features/kas/presentation/screens/kas_screen.dart';

// ─── Fake KasApiClient (same as kas_form_navigation_guard_test.dart) ────────

class _FakeKasApiClient implements KasApiClient {
  int createCount = 0;

  @override
  Dio get dio => Dio(BaseOptions(baseUrl: 'http://test.local'));

  @override
  Future<BackendRTIdentity> getRTIdentity(String rtId) async {
    return const BackendRTIdentity(name: 'RT 002');
  }

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
  }) async {
    return const PaginatedResponse<BackendTransaction>(
      data: [
        BackendTransaction(
          id: 'tx-sample-1',
          rtId: 'rt-002',
          categoryId: 'cat-income-1',
          amount: '150000.00',
          type: 'income',
          status: 'posted',
          description: 'Iuran Kas Sample',
          occurredAt: '2026-10-15T10:00:00Z',
          categoryName: 'Iuran Warga',
          paymentMethod: 'CASH',
        ),
      ],
      pagination: PaginationInfo(
        page: 1,
        pageSize: 20,
        total: 1,
        totalPages: 1,
      ),
    );
  }

  @override
  Future<List<BackendCategory>> listCategories({bool? isActive}) async {
    return const [
      BackendCategory(
        id: 'cat-income-1',
        rtId: 'rt-002',
        name: 'Iuran Warga',
        type: 'income',
        isActive: true,
      ),
      BackendCategory(
        id: 'cat-expense-1',
        rtId: 'rt-002',
        name: 'Operasional RT',
        type: 'expense',
        isActive: true,
      ),
    ];
  }

  @override
  Future<BackendTransaction> getTransaction(String id) async {
    return BackendTransaction(
      id: id,
      rtId: 'rt-002',
      categoryId: 'cat-income-1',
      amount: '150000.00',
      type: 'income',
      status: 'posted',
      description: 'Iuran Kas Sample',
      occurredAt: '2026-10-15T10:00:00Z',
      categoryName: 'Iuran Warga',
      paymentMethod: 'CASH',
    );
  }

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
      id: 'tx-created-$createCount',
      rtId: 'rt-002',
      categoryId: categoryId ?? 'cat-income-1',
      amount: amount,
      type: type,
      status: 'posted',
      description: description,
      occurredAt: (occurredAt ?? DateTime.now()).toIso8601String(),
      paymentMethod: paymentMethod ?? 'CASH',
      expenseCategory: expenseCategory,
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
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<Map<String, dynamic>> updateOpeningBalance({
    required String period,
    required String openingBalance,
    String? reason,
  }) {
    return Future.value({});
  }

  @override
  Future<BackendBalance> getBalance({
    String? period,
    String? startDate,
    String? endDate,
  }) async {
    return const BackendBalance(
      totalIncome: '150000.00',
      totalExpense: '0.00',
      netBalance: '150000.00',
      openingBalance: '0.00',
      closingBalance: '150000.00',
      netMovement: '150000.00',
      transactionCount: 1,
    );
  }

  @override
  Future<Map<String, dynamic>> getDashboardSummary([String? period]) async {
    return {
      'kas_balance': '150000.00',
      'monthly_income': '150000.00',
      'monthly_expense': '0.00',
    };
  }
}

// ─── Fund Location Stub ─────────────────────────────────────────────────────

class _FakeFundLocationRepository extends FundLocationRepository {
  _FakeFundLocationRepository()
      : super(
          apiClient: FundLocationApiClient(
            dio: Dio(BaseOptions(baseUrl: 'http://test.local')),
          ),
        );

  @override
  Future<List<FundLocation>> fetchLocations({
    String? category,
    bool? isActive,
  }) async {
    return [];
  }
}

// ─── Test App Builder (uses full App widget with ProviderScope) ──────────────

rp.ProviderScope _buildTestApp(_FakeKasApiClient kasApiClient) {
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
      kasRepositoryProvider.overrideWithValue(
        KasRepository(apiClient: kasApiClient),
      ),
      fundLocationRepositoryProvider.overrideWithValue(
        _FakeFundLocationRepository(),
      ),
    ],
    child: const App(),
  );
}

// ─── Helpers ────────────────────────────────────────────────────────────────

Finder _nominalField() =>
    find.widgetWithText(TextFormField, 'Masukkan nominal');

Finder _keteranganField() =>
    find.widgetWithText(TextFormField, 'Deskripsi transaksi');

/// Scroll until the 'Batal' button is visible, then return its finder.
/// Batal lives inside a ListView so it may be off-screen.
Future<Finder> _batalBtn(WidgetTester tester) async {
  final batalBtn = find.text('Batal');
  await tester.scrollUntilVisible(
    batalBtn,
    200,
    scrollable: find.byType(Scrollable).first,
  );
  return batalBtn;
}

Future<void> _openKasTab(WidgetTester tester) async {
  final kasTab = find.byKey(const Key('nav-item-kas'));
  await tester.ensureVisible(kasTab);
  await tester.tap(kasTab);
  await tester.pumpAndSettle();
  expect(find.byType(KasScreen), findsOneWidget);
}

Future<void> _openKasMasukForm(WidgetTester tester) async {
  await _openKasTab(tester);
  await tester.tap(find.byKey(const Key('kas-add-transaction-btn')));
  await tester.pumpAndSettle();
  await tester.tap(find.text('Catat Kas Masuk'));
  await tester.pumpAndSettle();
  expect(find.byType(KasMasukScreen), findsOneWidget);
}

Future<void> _openKasKeluarForm(WidgetTester tester) async {
  await _openKasTab(tester);
  await tester.tap(find.byKey(const Key('kas-add-transaction-btn')));
  await tester.pumpAndSettle();
  await tester.tap(find.text('Catat Kas Keluar'));
  await tester.pumpAndSettle();
  expect(find.byType(KasKeluarScreen), findsOneWidget);
}

// ─── Regression Tests: Dialog Behavior ──────────────────────────────────────

void main() {
  setUp(() {
    KasFormNavigationGuard.resetForTesting();
  });

  group('Dialog Regression Tests', () {
    // ─── Kas Masuk variants ─────────────────────────────────────────────

    testWidgets(
      '7. Batal button shows dialog with both options',
      (tester) async {
        final api = _FakeKasApiClient();
        await tester.pumpWidget(_buildTestApp(api));
        await tester.pumpAndSettle();

        await _openKasMasukForm(tester);

        // Fill input (so we can verify it's preserved later)
        await tester.enterText(_nominalField(), '50000');
        await tester.enterText(_keteranganField(), 'Data preserved test');
        await tester.pump();

        // Tap Batal
        final batalBtn = await _batalBtn(tester);
        await tester.tap(batalBtn);
        await tester.pumpAndSettle();

        // Verify: dialog with both buttons
        expect(find.byType(AlertDialog), findsOneWidget);
        expect(
          find.text('Apakah Anda yakin ingin membatalkan?'),
          findsOneWidget,
        );
        expect(find.text('Tidak, Tetap di Halaman'), findsOneWidget);
        expect(find.text('Ya, Batalkan'), findsOneWidget);
      },
    );

    testWidgets(
      '8. Batal → Tidak, Tetap di Halaman → form stays with input preserved',
      (tester) async {
        final api = _FakeKasApiClient();
        await tester.pumpWidget(_buildTestApp(api));
        await tester.pumpAndSettle();

        await _openKasMasukForm(tester);

        // Fill input
        await tester.enterText(_nominalField(), '50000');
        await tester.enterText(_keteranganField(), 'Data preserved test');
        await tester.pump();

        // Tap Batal
        final batalBtn = await _batalBtn(tester);
        await tester.tap(batalBtn);
        await tester.pumpAndSettle();
        expect(find.byType(AlertDialog), findsOneWidget);

        // Tap "Tidak, Tetap di Halaman" (key: kas-cancel-stay)
        await tester.tap(find.byKey(const Key('kas-cancel-stay')));
        await tester.pumpAndSettle();

        // Verify: dialog closed, still on form, input preserved
        expect(find.byType(AlertDialog), findsNothing);
        expect(find.byType(KasMasukScreen), findsOneWidget);
        expect(find.text('Data preserved test'), findsOneWidget);
      },
    );

    testWidgets(
      '9. Batal → Ya, Batalkan → navigates to list',
      (tester) async {
        final api = _FakeKasApiClient();
        await tester.pumpWidget(_buildTestApp(api));
        await tester.pumpAndSettle();

        await _openKasMasukForm(tester);

        // Fill input (should be discarded on "Ya")
        await tester.enterText(_nominalField(), '50000');
        await tester.enterText(_keteranganField(), 'Should be discarded');
        await tester.pump();

        // Tap Batal
        final batalBtn = await _batalBtn(tester);
        await tester.tap(batalBtn);
        await tester.pumpAndSettle();
        expect(find.byType(AlertDialog), findsOneWidget);

        // Tap "Ya, Batalkan" (key: kas-cancel-confirm)
        await tester.tap(find.byKey(const Key('kas-cancel-confirm')));
        await tester.pumpAndSettle();

        // Verify: navigated away
        expect(find.byType(AlertDialog), findsNothing);
        expect(find.byType(KasMasukScreen), findsNothing);
        expect(find.byType(KasScreen), findsOneWidget);
      },
    );

    testWidgets(
      '10. AppBar back button shows dialog',
      (tester) async {
        final api = _FakeKasApiClient();
        await tester.pumpWidget(_buildTestApp(api));
        await tester.pumpAndSettle();

        await _openKasMasukForm(tester);

        // Tap AppBar back
        await tester.tap(find.byIcon(Icons.arrow_back));
        await tester.pumpAndSettle();

        // Verify: dialog shown
        expect(find.byType(AlertDialog), findsOneWidget);
        expect(
          find.text('Apakah Anda yakin ingin membatalkan?'),
          findsOneWidget,
        );
      },
    );

    testWidgets(
      '11. AppBar back → Tidak → stays on form',
      (tester) async {
        final api = _FakeKasApiClient();
        await tester.pumpWidget(_buildTestApp(api));
        await tester.pumpAndSettle();

        await _openKasMasukForm(tester);

        // Fill input
        await tester.enterText(_nominalField(), '30000');
        await tester.enterText(_keteranganField(), 'AppBar back test');
        await tester.pump();

        // AppBar back
        await tester.tap(find.byIcon(Icons.arrow_back));
        await tester.pumpAndSettle();
        expect(find.byType(AlertDialog), findsOneWidget);

        // "Tidak"
        await tester.tap(find.byKey(const Key('kas-cancel-stay')));
        await tester.pumpAndSettle();

        expect(find.byType(AlertDialog), findsNothing);
        expect(find.byType(KasMasukScreen), findsOneWidget);
        expect(find.text('AppBar back test'), findsOneWidget);
      },
    );

    testWidgets(
      '12. Hardware back shows dialog and "Tidak" keeps form',
      (tester) async {
        final api = _FakeKasApiClient();
        await tester.pumpWidget(_buildTestApp(api));
        await tester.pumpAndSettle();

        await _openKasMasukForm(tester);

        // Fill input
        await tester.enterText(_nominalField(), '20000');
        await tester.enterText(_keteranganField(), 'Hardware back test');
        await tester.pump();

        // Hardware back
        await tester.binding.handlePopRoute();
        await tester.pumpAndSettle();

        // Verify: dialog shown
        expect(find.byType(AlertDialog), findsOneWidget);
        expect(
          find.text('Apakah Anda yakin ingin membatalkan?'),
          findsOneWidget,
        );

        // "Tidak"
        await tester.tap(find.byKey(const Key('kas-cancel-stay')));
        await tester.pumpAndSettle();

        expect(find.byType(AlertDialog), findsNothing);
        expect(find.byType(KasMasukScreen), findsOneWidget);
        expect(find.text('Hardware back test'), findsOneWidget);
      },
    );

    testWidgets(
      '13. Hardware back → Ya → navigates to list',
      (tester) async {
        final api = _FakeKasApiClient();
        await tester.pumpWidget(_buildTestApp(api));
        await tester.pumpAndSettle();

        await _openKasMasukForm(tester);

        // Hardware back
        await tester.binding.handlePopRoute();
        await tester.pumpAndSettle();
        expect(find.byType(AlertDialog), findsOneWidget);

        // "Ya"
        await tester.tap(find.byKey(const Key('kas-cancel-confirm')));
        await tester.pumpAndSettle();

        // Verify: navigated to list
        expect(find.byType(AlertDialog), findsNothing);
        expect(find.byType(KasMasukScreen), findsNothing);
        expect(find.byType(KasScreen), findsOneWidget);
      },
    );

    testWidgets(
      '14. Dialog can be reused (Not stuck after first dismiss)',
      (tester) async {
        final api = _FakeKasApiClient();
        await tester.pumpWidget(_buildTestApp(api));
        await tester.pumpAndSettle();

        await _openKasMasukForm(tester);

        // First: open dialog and dismiss with "Tidak"
        final batalBtn = await _batalBtn(tester);
        await tester.tap(batalBtn);
        await tester.pumpAndSettle();
        expect(find.byType(AlertDialog), findsOneWidget);

        await tester.tap(find.byKey(const Key('kas-cancel-stay')));
        await tester.pumpAndSettle();

        expect(find.byType(KasMasukScreen), findsOneWidget);
        expect(find.byType(AlertDialog), findsNothing);

        // Second: open dialog again and dismiss with "Ya"
        final batalBtn2 = await _batalBtn(tester);
        await tester.tap(batalBtn2);
        await tester.pumpAndSettle();
        expect(find.byType(AlertDialog), findsOneWidget);

        await tester.tap(find.byKey(const Key('kas-cancel-confirm')));
        await tester.pumpAndSettle();

        // Verify: navigated to list
        expect(find.byType(KasScreen), findsOneWidget);
        expect(find.byType(KasMasukScreen), findsNothing);
      },
    );

    testWidgets(
      '15. Save success bypass → navigation without dialog',
      (tester) async {
        final api = _FakeKasApiClient();
        await tester.pumpWidget(_buildTestApp(api));
        await tester.pumpAndSettle();

        await _openKasMasukForm(tester);

        // Fill input
        await tester.enterText(_nominalField(), '100000');
        await tester.enterText(_keteranganField(), 'Should be saved');
        await tester.pump();

        // Simulate allowExitFor called after successful save
        KasFormNavigationGuard.allowExitFor('/home/kas/masuk');

        // Tap Batal
        final batalBtn = await _batalBtn(tester);
        await tester.tap(batalBtn);
        await tester.pumpAndSettle();

        // Verify: navigated without dialog (allowExitFor consumed by onRouteExit)
        expect(find.byType(AlertDialog), findsNothing);
        expect(find.byType(KasScreen), findsOneWidget);
      },
    );

    testWidgets(
      '16. Double tap back → single dialog (no stacking)',
      (tester) async {
        final api = _FakeKasApiClient();
        await tester.pumpWidget(_buildTestApp(api));
        await tester.pumpAndSettle();

        await _openKasMasukForm(tester);

        // Double back: first triggers dialog
        await tester.binding.handlePopRoute();
        await tester.pumpAndSettle();
        expect(find.byType(AlertDialog), findsOneWidget);

        // Second back while dialog still open
        await tester.binding.handlePopRoute();
        await tester.pumpAndSettle();

        // Should still be exactly one dialog (not two)
        expect(find.byType(AlertDialog), findsOneWidget);

        // Dismiss
        await tester.tap(find.byKey(const Key('kas-cancel-stay')));
        await tester.pumpAndSettle();

        // No stacked dialog
        expect(find.byType(AlertDialog), findsNothing);
        expect(find.byType(KasMasukScreen), findsOneWidget);
      },
    );

    // ─── Kas Keluar variants ──────────────────────────────────────────────

    testWidgets(
      '17. Kas Keluar: Batal → Tidak → form preserved',
      (tester) async {
        final api = _FakeKasApiClient();
        await tester.pumpWidget(_buildTestApp(api));
        await tester.pumpAndSettle();

        await _openKasKeluarForm(tester);

        // Fill input
        await tester.enterText(_nominalField(), '75000');
        await tester.enterText(_keteranganField(), 'Keluar test');
        await tester.pump();

        // Tap Batal
        final batalBtn = await _batalBtn(tester);
        await tester.tap(batalBtn);
        await tester.pumpAndSettle();
        expect(find.byType(AlertDialog), findsOneWidget);

        // "Tidak"
        await tester.tap(find.byKey(const Key('kas-cancel-stay')));
        await tester.pumpAndSettle();

        expect(find.byType(AlertDialog), findsNothing);
        expect(find.byType(KasKeluarScreen), findsOneWidget);
        expect(find.text('Keluar test'), findsOneWidget);
      },
    );

    testWidgets(
      '18. Kas Keluar: AppBar back → dialog',
      (tester) async {
        final api = _FakeKasApiClient();
        await tester.pumpWidget(_buildTestApp(api));
        await tester.pumpAndSettle();

        await _openKasKeluarForm(tester);

        // AppBar back
        await tester.tap(find.byIcon(Icons.arrow_back));
        await tester.pumpAndSettle();

        expect(find.byType(AlertDialog), findsOneWidget);
        expect(
          find.text('Apakah Anda yakin ingin membatalkan?'),
          findsOneWidget,
        );
      },
    );

    testWidgets(
      '19. Kas Keluar: Hardware back → dialog → Ya → navigates',
      (tester) async {
        final api = _FakeKasApiClient();
        await tester.pumpWidget(_buildTestApp(api));
        await tester.pumpAndSettle();

        await _openKasKeluarForm(tester);

        // Hardware back
        await tester.binding.handlePopRoute();
        await tester.pumpAndSettle();
        expect(find.byType(AlertDialog), findsOneWidget);

        // "Ya"
        await tester.tap(find.byKey(const Key('kas-cancel-confirm')));
        await tester.pumpAndSettle();

        expect(find.byType(AlertDialog), findsNothing);
        expect(find.byType(KasKeluarScreen), findsNothing);
        expect(find.byType(KasScreen), findsOneWidget);
      },
    );

    testWidgets(
      '20. Kas Keluar: Dialog reusable after "Tidak"',
      (tester) async {
        final api = _FakeKasApiClient();
        await tester.pumpWidget(_buildTestApp(api));
        await tester.pumpAndSettle();

        await _openKasKeluarForm(tester);

        // First: Batal → Tidak
        final batalBtn = await _batalBtn(tester);
        await tester.tap(batalBtn);
        await tester.pumpAndSettle();
        expect(find.byType(AlertDialog), findsOneWidget);
        await tester.tap(find.byKey(const Key('kas-cancel-stay')));
        await tester.pumpAndSettle();
        expect(find.byType(AlertDialog), findsNothing);
        expect(find.byType(KasKeluarScreen), findsOneWidget);

        // Second: Batal → Ya
        final batalBtn2 = await _batalBtn(tester);
        await tester.tap(batalBtn2);
        await tester.pumpAndSettle();
        expect(find.byType(AlertDialog), findsOneWidget);
        await tester.tap(find.byKey(const Key('kas-cancel-confirm')));
        await tester.pumpAndSettle();

        expect(find.byType(AlertDialog), findsNothing);
        expect(find.byType(KasKeluarScreen), findsNothing);
        expect(find.byType(KasScreen), findsOneWidget);
      },
    );
  });
}
