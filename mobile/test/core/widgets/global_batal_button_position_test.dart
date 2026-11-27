import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/app/theme/app_theme.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/core/navigators/kas_form_navigation_guard.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/fund_location/data/fund_location_repository.dart';
import 'package:social_finance/features/fund_location/data/models/fund_location_models.dart';
import 'package:social_finance/features/iuran/data/iuran_api_client.dart';
import 'package:social_finance/features/iuran/data/iuran_api_models.dart'
    show BackendDue;
import 'package:social_finance/features/iuran/data/iuran_models.dart';
import 'package:social_finance/features/iuran/data/iuran_providers.dart';
import 'package:social_finance/features/iuran/data/iuran_repository.dart';
import 'package:social_finance/features/iuran/presentation/screens/iuran_payment_screen.dart';
import 'package:social_finance/features/kas/data/kas_api_client.dart';
import 'package:social_finance/features/kas/data/kas_api_models.dart'
    show BackendCategory;
import 'package:social_finance/features/kas/data/kas_repository.dart';
import 'package:social_finance/features/kas/presentation/screens/kas_keluar_screen.dart';
import 'package:social_finance/features/kas/presentation/screens/kas_masuk_screen.dart';
import 'package:social_finance/features/warga/data/api_warga_models.dart';
import 'package:social_finance/features/warga/data/warga_providers.dart';
import 'package:social_finance/features/warga/data/warga_repository.dart';
import 'package:social_finance/features/warga/presentation/screens/household_create_screen.dart';
import 'package:social_finance/features/warga/presentation/screens/household_edit_screen.dart';
import 'package:social_finance/features/warga/presentation/screens/special_resident_create_screen.dart';
import 'package:social_finance/features/warga/presentation/screens/special_resident_edit_screen.dart';

// ─── Fakes ───────────────────────────────────────────────────────────

class _FakeAuthRepo extends AuthRepository {
  _FakeAuthRepo()
    : super(
        authService: AuthService(ApiClient(baseUrl: 'http://test.local')),
        apiClient: ApiClient(baseUrl: 'http://test.local'),
      ) {
    state = const AsyncData({
      'id': 'user-admin-1',
      'full_name': 'Bendahara RT 002',
      'email': 'bendahara@test.local',
      'role': AppRole.bendahara,
      'system_role': 'user',
      'rt_id': 'rt-002',
      'jabatan': 'bendahara',
      'effective_permissions': [
        'finance.create',
        'finance.approve',
        'resident.write',
        'iuran.pay',
      ],
    });
  }

  @override
  Future<void> reloadProfile() async {}
}

class _FakeKasApiClient extends KasApiClient {
  _FakeKasApiClient() : super(dio: Dio(BaseOptions(baseUrl: 'http://test.local')));

  @override
  Future<List<BackendCategory>> listCategories({bool? isActive}) async => const [
    BackendCategory(
      id: 'cat-in-1',
      rtId: 'rt-002',
      name: 'Iuran Warga',
      type: 'income',
      isActive: true,
    ),
    BackendCategory(
      id: 'cat-out-1',
      rtId: 'rt-002',
      name: 'Operasional RT',
      type: 'expense',
      isActive: true,
    ),
  ];
}

class _FakeIuranRepository extends IuranRepository {
  _FakeIuranRepository()
    : super(
        apiClient: IuranApiClient(
          dio: Dio(BaseOptions(baseUrl: 'http://test.local')),
        ),
      );

  @override
  Future<List<BackendDue>> fetchDues({bool? isActive}) async => const [];

  @override
  Future<List<IuranBill>> fetchAllBills({
    String? search,
    String? status,
    String? paymentStatus,
    bool myBills = false,
  }) async => const [];
}

class _StaticWargaListNotifier extends WargaListNotifier {
  @override
  Future<List<MappedResident>> build() async => const [];

  @override
  Future<void> refresh() async {}
}

class _FakeWargaRepository extends WargaRepository {
  int updateSpecialCalls = 0;

  _FakeWargaRepository()
    : super(client: ApiClient(baseUrl: 'http://test.local'));

  @override
  Future<BackendResident> fetchResidentById(String id) async {
    return BackendResident(
      id: id,
      rtId: 'rt-002',
      fullName: 'Petugas Keamanan Uji',
      phone: '081234567890',
      nik: '3201010101900001',
      email: 'keamanan@test.local',
      isActive: true,
      jabatan: 'keamanan',
    );
  }

  @override
  Future<PaginatedResponse<MappedResident>> fetchResidentsWithHouseholds({
    int page = 1,
    int pageSize = 100,
    String? search,
  }) async {
    return const PaginatedResponse<MappedResident>(
      data: [
        MappedResident(
          id: 'res-hh-1',
          householdId: 'hh-1',
          name: 'Budi Santoso',
          houseNumber: 'A-01',
          occupancyStatus: 'Pemilik',
          address: 'Jl. Melati No. 1',
          nik: '3201010101900002',
          phoneNumber: '081234567891',
          email: 'budi@test.local',
          isActive: true,
        ),
      ],
      pagination: PaginationInfo(
        page: 1,
        pageSize: 100,
        total: 1,
        totalPages: 1,
      ),
    );
  }

  @override
  Future<Map<String, BackendHousehold>> fetchHouseholds({
    int page = 1,
    int pageSize = 100,
  }) async => const {};

  @override
  Future<void> updateSpecialResident(
    String id, {
    required UpdateSpecialResidentRequest request,
  }) async {
    updateSpecialCalls++;
  }
}

// ─── Assertions ──────────────────────────────────────────────────────

/// Verifies visually (via rendered Rect coordinates) that the Batal button sits
/// strictly to the LEFT of the primary action button on the same horizontal row,
/// with zero overlap and zero RenderFlex overflow.
void expectBatalVisuallyLeftOfPrimary(
  WidgetTester tester, {
  required String primaryLabel,
  required String screenName,
}) {
  final batalFinder = find.ancestor(
    of: find.text('Batal'),
    matching: find.bySubtype<ButtonStyleButton>(),
  );
  final primaryFinder = find.ancestor(
    of: find.text(primaryLabel),
    matching: find.bySubtype<ButtonStyleButton>(),
  );

  expect(
    batalFinder,
    findsOneWidget,
    reason: '$screenName must render exactly one Batal button',
  );
  expect(
    primaryFinder,
    findsOneWidget,
    reason: '$screenName must render exactly one $primaryLabel button',
  );

  final batalRect = tester.getRect(batalFinder);
  final primaryRect = tester.getRect(primaryFinder);

  expect(
    batalRect.left,
    lessThan(primaryRect.left),
    reason:
        '$screenName: Batal (left=${batalRect.left}) must be visually to the left of $primaryLabel (left=${primaryRect.left})',
  );
  expect(
    batalRect.right,
    lessThanOrEqualTo(primaryRect.left),
    reason:
        '$screenName: Batal (right=${batalRect.right}) must not overlap $primaryLabel (left=${primaryRect.left})',
  );
  expect(
    (batalRect.center.dy - primaryRect.center.dy).abs(),
    lessThan(2.0),
    reason:
        '$screenName: Batal (cy=${batalRect.center.dy}) and $primaryLabel (cy=${primaryRect.center.dy}) must be aligned on the same horizontal row',
  );
  expect(
    tester.takeException(),
    isNull,
    reason: '$screenName must not throw RenderFlex overflow or layout errors',
  );
}

Widget _wrapWithRouter({
  required Widget screen,
  required List<Override> overrides,
  required TargetPlatform platform,
}) {
  final router = GoRouter(
    initialLocation: '/test-form',
    routes: [
      GoRoute(
        path: '/test-form',
        builder: (context, state) => screen,
      ),
      GoRoute(
        path: '/home',
        builder: (context, state) => const Scaffold(body: Text('Home')),
      ),
    ],
  );

  return ProviderScope(
    overrides: overrides,
    child: MaterialApp.router(
      theme: AppTheme.lightTheme.copyWith(platform: platform),
      routerConfig: router,
    ),
  );
}

void main() {
  setUp(() {
    KasFormNavigationGuard.resetForTesting();
  });

  List<Override> commonOverrides({_FakeWargaRepository? wargaRepo}) => [
    authRepositoryProvider.overrideWith((ref) => _FakeAuthRepo()),
    kasApiClientProvider.overrideWithValue(_FakeKasApiClient()),
    fundLocationsProvider.overrideWith(
      (ref) async => <FundLocation>[
        FundLocation(
          id: 'loc-1',
          rtId: 'rt-002',
          name: 'Kas Tunai RT',
          category: FundLocationCategory.coh,
          isActive: true,
          currentBalance: '500000.00',
          createdAt: DateTime(2026, 1, 1),
          updatedAt: DateTime(2026, 1, 1),
        ),
      ],
    ),
    publicRekeningProvider.overrideWith(
      (ref) async => const <PublicRekening>[],
    ),
    iuranRepositoryProvider.overrideWithValue(_FakeIuranRepository()),
    wargaListProvider.overrideWith(_StaticWargaListNotifier.new),
    wargaRepositoryProvider.overrideWithValue(
      wargaRepo ?? _FakeWargaRepository(),
    ),
  ];

  Future<void> verifyScreenOnPlatform(
    WidgetTester tester, {
    required String name,
    required Widget screen,
    required String primaryLabel,
    required TargetPlatform platform,
    bool scrollToActionRow = false,
    bool tapPrimaryAndExpectNoCancelDialog = true,
    _FakeWargaRepository? wargaRepo,
  }) async {
    debugDefaultTargetPlatformOverride = platform;
    tester.view.physicalSize = const Size(800, 1000);
    tester.view.devicePixelRatio = 1.0;
    try {
      await tester.pumpWidget(
        _wrapWithRouter(
          screen: screen,
          overrides: commonOverrides(wargaRepo: wargaRepo),
          platform: platform,
        ),
      );
      await tester.pumpAndSettle();

      if (scrollToActionRow) {
        await tester.scrollUntilVisible(
          find.text('Batal'),
          250,
          scrollable: find.byType(Scrollable).first,
        );
        await tester.pumpAndSettle();
      }

      // 1. Visual coordinate assertion: Batal is strictly LEFT of primary action
      expectBatalVisuallyLeftOfPrimary(
        tester,
        primaryLabel: primaryLabel,
        screenName: '$name (${platform.name})',
      );

      // 2. Primary callback works without invoking cancel dialog
      if (tapPrimaryAndExpectNoCancelDialog) {
        await tester.tap(find.text(primaryLabel));
        await tester.pumpAndSettle();
        expect(
          find.text('Apakah Anda yakin ingin membatalkan?'),
          findsNothing,
          reason:
              '$name: Tapping $primaryLabel must invoke primary handler, not cancel confirmation',
        );
      }

      // 3. Batal callback opens cancel confirmation dialog & dismissing keeps form
      await tester.tap(find.text('Batal'));
      await tester.pumpAndSettle();
      expect(
        find.text('Apakah Anda yakin ingin membatalkan?'),
        findsOneWidget,
        reason: '$name: Tapping Batal must show cancel confirmation dialog',
      );
      await tester.tap(find.text('Tidak, Tetap di Halaman'));
      await tester.pumpAndSettle();
      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsNothing);
    } finally {
      debugDefaultTargetPlatformOverride = null;
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    }
  }

  group('Global Batal Button Position (Batal Left -> Primary Right) on All 7 Fixed Locations', () {
    for (final platform in [TargetPlatform.android, TargetPlatform.iOS]) {
      testWidgets('1. KasMasukScreen [${platform.name}]: Batal (left) -> Simpan (right)', (tester) async {
        await verifyScreenOnPlatform(
          tester,
          name: 'KasMasukScreen',
          screen: const KasMasukScreen(),
          primaryLabel: 'Simpan',
          platform: platform,
        );
      });

      testWidgets('2. KasKeluarScreen [${platform.name}]: Batal (left) -> Simpan (right)', (tester) async {
        await verifyScreenOnPlatform(
          tester,
          name: 'KasKeluarScreen',
          screen: const KasKeluarScreen(),
          primaryLabel: 'Simpan',
          platform: platform,
        );
      });

      testWidgets('3. SpecialResidentCreateScreen [${platform.name}]: Batal (left) -> Simpan (right)', (tester) async {
        await verifyScreenOnPlatform(
          tester,
          name: 'SpecialResidentCreateScreen',
          screen: const SpecialResidentCreateScreen(isSuperAdmin: false),
          primaryLabel: 'Simpan',
          platform: platform,
        );
      });

      testWidgets('4. HouseholdCreateScreen [${platform.name}]: Batal (left) -> Simpan (right)', (tester) async {
        await verifyScreenOnPlatform(
          tester,
          name: 'HouseholdCreateScreen',
          screen: const HouseholdCreateScreen(isSuperAdmin: false),
          primaryLabel: 'Simpan',
          platform: platform,
        );
      });

      testWidgets('5. HouseholdEditScreen [${platform.name}]: Batal (left) -> Simpan Perubahan (right)', (tester) async {
        await verifyScreenOnPlatform(
          tester,
          name: 'HouseholdEditScreen',
          screen: const HouseholdEditScreen(householdId: 'hh-1'),
          primaryLabel: 'Simpan Perubahan',
          platform: platform,
        );
      });

      testWidgets('6. IuranPaymentScreen [${platform.name}]: Batal (left) -> Bayar (right)', (tester) async {
        await verifyScreenOnPlatform(
          tester,
          name: 'IuranPaymentScreen',
          screen: const IuranPaymentScreen(billId: 'bill-1'),
          primaryLabel: 'Bayar',
          platform: platform,
        );
      });

      testWidgets('7. SpecialResidentEditScreen [${platform.name}]: Batal (left) -> Simpan (right)', (tester) async {
        final fakeRepo = _FakeWargaRepository();
        await verifyScreenOnPlatform(
          tester,
          name: 'SpecialResidentEditScreen',
          screen: const SpecialResidentEditScreen(residentId: 'res-special-1'),
          primaryLabel: 'Simpan',
          platform: platform,
          scrollToActionRow: true,
          tapPrimaryAndExpectNoCancelDialog: true,
          wargaRepo: fakeRepo,
        );
        expect(fakeRepo.updateSpecialCalls, equals(1));
      });
    }
  });
}
