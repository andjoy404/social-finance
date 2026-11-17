import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:social_finance/app/navigation_shell.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/api/device_id_service.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/rw/data/rw_api_client.dart';
import 'package:social_finance/features/rw/data/rw_models.dart';
import 'package:social_finance/features/rw/data/rw_providers.dart';
import 'package:social_finance/features/rw/presentation/screens/rw_dashboard_screen.dart';
import 'package:social_finance/features/rw/presentation/screens/rw_fund_location_screen.dart';
import 'package:social_finance/features/rw/presentation/screens/rw_kas_screen.dart';
import 'package:social_finance/features/rw/presentation/screens/rw_keuangan_screen.dart';

class MockRwApiClient extends RwApiClient {
  MockRwApiClient() : super(ApiClient(baseUrl: 'http://test.local'));

  @override
  Future<RwDashboardSummary> getDashboard(String rwId, {String? period}) async {
    return const RwDashboardSummary(
      totalRts: 5,
      totalLocations: 3,
      totalBalance: '15000000.00',
      totalIncome: '5000000.00',
      totalExpense: '2000000.00',
      transactionCount: 12,
    );
  }

  @override
  Future<RwKasReport> getKas(String rwId, {String? period}) async {
    return const RwKasReport(
      totalIncome: '5000000.00',
      totalExpense: '2000000.00',
      netBalance: '3000000.00',
      transactionCount: 8,
    );
  }

  @override
  Future<RwKasReportDetail> getKasReportDetail(String rwId,
      {String? period, String? startDate, String? endDate}) async {
    return const RwKasReportDetail(
      period: '2026-10',
      startDate: '',
      endDate: '',
      openingBalance: '1000000.00',
      totalIncome: '5000000.00',
      totalExpense: '2000000.00',
      netBalance: '3000000.00',
      totalBalance: '4000000.00',
      transactionCount: 8,
    );
  }

  @override
  Future<RwTransactionsResponse> getTransactions({
    required String rwId,
    String? period,
    String? startDate,
    String? endDate,
    int page = 1,
    int pageSize = 20,
    String? type,
    String? categoryId,
  }) async {
    return RwTransactionsResponse(
      data: [
        RwTransaction(
          id: 'tx-1',
          rtId: 'rt-1',
          rtName: 'RT 001',
          type: 'income',
          amount: '1000000.00',
          description: 'Penerimaan Iuran RT 01',
          locationName: 'Rekening BCA Operasional RW',
          status: 'posted',
          occurredAt: DateTime(2026, 10, 15),
          createdAt: DateTime(2026, 10, 15),
        ),
      ],
      pagination: const RwPaginationInfo(
        page: 1,
        pageSize: 20,
        total: 1,
        totalPages: 1,
      ),
    );
  }

  @override
  Future<List<RwFundLocation>> getLocations(String rwId, {String? category, bool? isActive}) async {
    return [
      RwFundLocation(
        id: 'loc-1',
        rtId: '',
        rtName: 'RW',
        name: 'Rekening BCA Operasional RW',
        category: 'rekening',
        bankName: 'BCA',
        accountNumber: '888001122',
        accountHolder: 'Pengurus RW',
        isActive: true,
        currentBalance: '10000000.00',
        createdAt: DateTime(2026, 1, 1),
      ),
      RwFundLocation(
        id: 'loc-2',
        rtId: '',
        rtName: 'RW',
        name: 'Brankas Sekretariat RW',
        category: 'brankas',
        isActive: true,
        currentBalance: '3000000.00',
        createdAt: DateTime(2026, 1, 1),
      ),
      RwFundLocation(
        id: 'loc-3',
        rtId: '',
        rtName: 'RW',
        name: 'Kas Di Tangan Bendahara RW',
        category: 'coh',
        isActive: true,
        currentBalance: '2000000.00',
        createdAt: DateTime(2026, 1, 1),
      ),
    ];
  }

  @override
  Future<List<RwFundTransfer>> getTransfers(String rwId, {int page = 1, int pageSize = 20}) async {
    return [
      RwFundTransfer(
        id: 'tf-1',
        rwId: 'rw-002',
        fromLocationId: 'loc-1',
        fromLocationName: 'Rekening BCA Operasional RW',
        toLocationId: 'loc-2',
        toLocationName: 'Brankas Sekretariat RW',
        amount: '1000000.00',
        transferDate: '2026-10-20',
        notes: 'Pencairan kas operasional',
        isReversal: false,
        createdAt: DateTime(2026, 10, 20),
      ),
    ];
  }

  @override
  Future<RwReconciliationReport> getReconciliation(String rwId) async {
    return const RwReconciliationReport(
      openingBalance: '0.00',
      totalIncome: '5000000.00',
      totalExpense: '2000000.00',
      calculatedBalance: '15000000.00',
      totalLocationBalances: '15000000.00',
      isReconciled: true,
      discrepancy: '0.00',
      unallocatedBalance: '0.00',
      allocatedPercentage: 100.0,
      status: 'matched',
      statusLabel: 'TEREKONSILIASI (Cocok Sempurna)',
      categoryBreakdown: [],
    );
  }

  @override
  Future<List<RwRtInfo>> getRTRs(String rwId) async {
    return const [
      RwRtInfo(id: 'rt-1', name: 'RT 001', rt: '001', rwNumber: 2, label: 'RT 001'),
      RwRtInfo(id: 'rt-2', name: 'RT 002', rt: '002', rwNumber: 2, label: 'RT 002'),
    ];
  }

  @override
  Future<List<RwFinancialCategory>> getCategories({bool? isActive}) async {
    return const [
      RwFinancialCategory(id: 'cat-1', name: 'Konsumsi', type: 'expense', isActive: true),
      RwFinancialCategory(id: 'cat-2', name: 'ATK', type: 'expense', isActive: true),
      RwFinancialCategory(id: 'cat-3', name: 'Pemasukan Lain', type: 'income', isActive: true),
    ];
  }
}

void main() {
  final mockUser = {
    'id': 'user-1',
    'email': 'bendahara.rw@test.com',
    'name': 'Bendahara RW 002',
    'role': AppRole.bendahara,
    'jabatan': 'bendahara',
    'rw': '002',
    'rw_memberships': [
      {'rw_id': 'rw-002', 'jabatan': 'bendahara', 'rw_name': 'RW 002', 'source': 'explicit'},
    ],
  };

  group('RW Screens UI & Elimination of Dana Hibah', () {
    testWidgets('RwDashboardScreen renders with MenuAppBarTitle and metrics', (tester) async {
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            rwApiClientProvider.overrideWithValue(MockRwApiClient()),
            authRepositoryProvider.overrideWith((ref) => _MockAuthRepo(mockUser)),
          ],
          child: const MaterialApp(
            home: RwDashboardScreen(rwId: 'rw-002'),
          ),
        ),
      );

      await tester.pumpAndSettle();

      expect(find.text('BERANDA RW'), findsOneWidget);
      expect(find.text('TOTAL SALDO'), findsOneWidget);
      expect(find.text('TOTAL PEMASUKAN'), findsOneWidget);
      expect(find.text('TOTAL PENGELUARAN'), findsOneWidget);
      expect(find.text('Ringkasan Keuangan'), findsOneWidget);
      expect(find.text('Transaksi Terbaru'), findsOneWidget);
      // Ensure redundant "Menu Keuangan RW" Card is completely removed
      expect(find.text('Menu Keuangan RW'), findsNothing);
    });

    testWidgets('RwFundLocationScreen renders 3 categories and NO Dana Hibah/Donasi', (tester) async {
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            rwApiClientProvider.overrideWithValue(MockRwApiClient()),
            authRepositoryProvider.overrideWith((ref) => _MockAuthRepo(mockUser)),
          ],
          child: const MaterialApp(
            home: RwFundLocationScreen(rwId: 'rw-002'),
          ),
        ),
      );

      await tester.pumpAndSettle();

      expect(find.text('Lokasi Dana RW'), findsAtLeastNWidgets(1));
      expect(find.text('Rekening Bank'), findsAtLeastNWidgets(1));
      expect(find.text('Brankas Fisik'), findsAtLeastNWidgets(1));
      expect(find.text('Kas Di Tangan (COH)'), findsAtLeastNWidgets(1));

      // Strictly verify that 'Kas Hibah/Donasi' and 'Dana Hibah' DO NOT EXIST
      expect(find.text('Kas Hibah/Donasi'), findsNothing);
      expect(find.text('Dana Hibah'), findsNothing);
      expect(find.text('Hibah/Donasi'), findsNothing);

      // Verify action buttons for authorized role
      expect(find.text('Tambah Lokasi'), findsOneWidget);
      expect(find.text('Transfer Antar-Lokasi'), findsOneWidget);
      expect(find.text('Riwayat Transfer'), findsOneWidget);
      expect(find.text('Rekonsiliasi'), findsOneWidget);

      // Verify no misleading aggregate warning banner
      expect(find.textContaining('data agregat untuk'), findsNothing);

      // Tap 'Riwayat Transfer' button and verify dialog opens
      await tester.tap(find.text('Riwayat Transfer'));
      await tester.pumpAndSettle();
      expect(find.text('Riwayat Transfer Antar-Lokasi RW'), findsOneWidget);
      expect(find.textContaining('Rekening BCA Operasional RW'), findsAtLeastNWidgets(1));
      expect(find.text('Tutup'), findsOneWidget);
      await tester.tap(find.text('Tutup'));
      await tester.pumpAndSettle();

      // Tap 'Rekonsiliasi' button and verify reconciliation dialog opens
      await tester.tap(find.text('Rekonsiliasi'));
      await tester.pumpAndSettle();
      expect(find.text('Rekonsiliasi Kas RW'), findsOneWidget);
      expect(find.textContaining('TEREKONSILIASI'), findsAtLeastNWidgets(1));
      await tester.tap(find.text('Tutup'));
      await tester.pumpAndSettle();
    });

    testWidgets('RwFundLocationScreen filter dropdowns do not overflow on normal screen', (tester) async {
      tester.view.physicalSize = const Size(1200, 1000);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(() {
        tester.view.resetPhysicalSize();
        tester.view.resetDevicePixelRatio();
      });

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            rwApiClientProvider.overrideWithValue(MockRwApiClient()),
            authRepositoryProvider.overrideWith((ref) => _MockAuthRepo(mockUser)),
          ],
          child: const MaterialApp(
            home: RwFundLocationScreen(rwId: 'rw-002'),
          ),
        ),
      );
      await tester.pumpAndSettle();

      // Both dropdowns exist as DropdownButtonFormField, no overflow exception
      expect(find.byType(DropdownButtonFormField<String>), findsNWidgets(2));
      expect(tester.takeException(), isNull);

      // Open category dropdown and verify menu can be opened without overflow
      await tester.tap(find.byType(DropdownButtonFormField<String>).first);
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
    });

    testWidgets('RwFundLocationScreen filter dropdowns do not overflow on narrow screen', (tester) async {
      tester.view.physicalSize = const Size(320, 640);
      tester.view.devicePixelRatio = 2.0;
      addTearDown(() {
        tester.view.resetPhysicalSize();
        tester.view.resetDevicePixelRatio();
      });

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            rwApiClientProvider.overrideWithValue(MockRwApiClient()),
            authRepositoryProvider.overrideWith((ref) => _MockAuthRepo(mockUser)),
          ],
          child: const MaterialApp(
            home: RwFundLocationScreen(rwId: 'rw-002'),
          ),
        ),
      );
      await tester.pumpAndSettle();

      // No overflow exception on narrow screen
      expect(tester.takeException(), isNull);
    });

    testWidgets('RwKasScreen renders with MenuAppBarTitle, KAS summary, and action buttons', (tester) async {
      tester.view.physicalSize = const Size(1200, 1500);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(() {
        tester.view.resetPhysicalSize();
        tester.view.resetDevicePixelRatio();
      });

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            rwApiClientProvider.overrideWithValue(MockRwApiClient()),
            authRepositoryProvider.overrideWith((ref) => _MockAuthRepo(mockUser)),
          ],
          child: const MaterialApp(
            home: RwKasScreen(rwId: 'rw-002'),
          ),
        ),
      );

      await tester.pumpAndSettle();

      expect(find.text('KAS RW'), findsAtLeastNWidgets(1));
      expect(find.text('Total Pemasukan'), findsOneWidget);
      expect(find.text('Total Pengeluaran'), findsOneWidget);
      expect(find.text('Saldo Bersih'), findsOneWidget);

      // Verify action buttons for authorized role
      expect(find.text('Catat Kas Masuk'), findsOneWidget);
      expect(find.text('Catat Kas Keluar'), findsOneWidget);

      // Tap transaction to open detail dialog
      await tester.tap(find.text('Penerimaan Iuran RT 01'));
      await tester.pumpAndSettle();
      expect(find.text('Detail KAS Masuk RW'), findsOneWidget);
      expect(find.text('Tutup'), findsOneWidget);
      await tester.tap(find.text('Tutup'));
      await tester.pumpAndSettle();
    });

    testWidgets('RwFundLocationScreen & RwKasScreen hide action buttons and show Read-Only for non-manager role', (tester) async {
      final nonManagerUser = {
        'id': 'user-warga-view',
        'email': 'warga.view@test.com',
        'name': 'Warga RW 002',
        'role': AppRole.warga,
        'rw': '002',
        'rw_memberships': [
          {'rw_id': 'rw-002', 'jabatan': 'anggota', 'rw_name': 'RW 002', 'source': 'explicit'},
        ],
      };

      // 1. Check Fund Location screen in read-only
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            rwApiClientProvider.overrideWithValue(MockRwApiClient()),
            authRepositoryProvider.overrideWith((ref) => _MockAuthRepo(nonManagerUser)),
          ],
          child: const MaterialApp(
            home: RwFundLocationScreen(rwId: 'rw-002'),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Tambah Lokasi'), findsNothing);
      expect(find.text('Transfer Antar-Lokasi'), findsNothing);
      expect(find.textContaining('Mode Read-Only'), findsOneWidget);

      // 2. Check Kas screen in read-only
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            rwApiClientProvider.overrideWithValue(MockRwApiClient()),
            authRepositoryProvider.overrideWith((ref) => _MockAuthRepo(nonManagerUser)),
          ],
          child: const MaterialApp(
            home: RwKasScreen(rwId: 'rw-002'),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Catat Kas Masuk'), findsNothing);
      expect(find.text('Catat Kas Keluar'), findsNothing);
      expect(find.textContaining('Mode Read-Only'), findsOneWidget);
    });

    testWidgets('RwFundLocationScreen & RwKasScreen respect wakil_ketua permissions (read-only for all features, cannot write)', (tester) async {
      final wakilKetuaUser = {
        'id': 'user-wk',
        'email': 'wakil.ketua@test.com',
        'name': 'Wakil Ketua RW 002',
        'role': AppRole.warga,
        'rw': '002',
        'rw_memberships': [
          {'rw_id': 'rw-002', 'jabatan': 'wakil_ketua', 'rw_name': 'RW 002', 'source': 'explicit'},
        ],
      };

      // 1. Fund location screen for wakil_ketua (read-only per final matrix)
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            rwApiClientProvider.overrideWithValue(MockRwApiClient()),
            authRepositoryProvider.overrideWith((ref) => _MockAuthRepo(wakilKetuaUser)),
          ],
          child: const MaterialApp(
            home: RwFundLocationScreen(rwId: 'rw-002'),
          ),
        ),
      );
      await tester.pumpAndSettle();

      // Final matrix: Pengurus RW has READ ONLY, cannot create or update
      expect(find.text('Tambah Lokasi'), findsNothing);
      expect(find.text('Transfer Antar-Lokasi'), findsNothing);
      // Can view Riwayat Transfer & Rekonsiliasi
      expect(find.text('Riwayat Transfer'), findsOneWidget);
      expect(find.text('Rekonsiliasi'), findsOneWidget);
      // Mode Read-Only badge is visible
      expect(find.textContaining('Mode Read-Only'), findsOneWidget);

      // 2. KAS screen for wakil_ketua (read-only per final matrix)
      tester.view.physicalSize = const Size(1200, 1000);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(() {
        tester.view.resetPhysicalSize();
        tester.view.resetDevicePixelRatio();
      });

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            rwApiClientProvider.overrideWithValue(MockRwApiClient()),
            authRepositoryProvider.overrideWith((ref) => _MockAuthRepo(wakilKetuaUser)),
          ],
          child: const MaterialApp(
            home: RwKasScreen(rwId: 'rw-002'),
          ),
        ),
      );
      await tester.pumpAndSettle();

      // Final matrix: cannot record kas masuk & kas keluar
      expect(find.text('Catat Kas Masuk'), findsNothing);
      expect(find.text('Catat Kas Keluar'), findsNothing);
      expect(find.textContaining('Mode Read-Only'), findsOneWidget);
    });

    testWidgets('Navigation Shell distributes tabs evenly for Warga (<= 4 tabs) and provides Drawer', (tester) async {
      final wargaUser = {
        'id': 'user-warga',
        'email': 'warga@test.com',
        'name': 'Warga Urena',
        'role': AppRole.warga,
        'rt': '001',
        'rw': '002',
        'rw_memberships': <Map<String, dynamic>>[],
      };

      final router = GoRouter(
        initialLocation: '/home',
        routes: [
          ShellRoute(
            builder: (context, state, child) => MainNavigationShell(child: child),
            routes: [
              GoRoute(
                path: '/home',
                builder: (context, state) => const Text('Warga Home Content'),
              ),
            ],
          ),
        ],
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authRepositoryProvider.overrideWith((ref) => _MockAuthRepo(wargaUser)),
          ],
          child: MaterialApp.router(
            routerConfig: router,
          ),
        ),
      );

      await tester.pumpAndSettle();

      expect(find.text('Warga Home Content'), findsOneWidget);
      expect(find.byType(BottomAppBar), findsOneWidget);

      // Verify that Drawer is configured on the Scaffold
      final scaffold = tester.widget<Scaffold>(find.byType(Scaffold));
      expect(scaffold.drawer, isNotNull);

      // Verify each item inside BottomAppBar is wrapped in Expanded (equal width division)
      expect(find.descendant(of: find.byType(BottomAppBar), matching: find.byType(Expanded)), findsNWidgets(4));

      // Open drawer via ScaffoldState and verify drawer contents
      final scaffoldState = tester.state<ScaffoldState>(find.byType(Scaffold));
      scaffoldState.openDrawer();
      await tester.pumpAndSettle();

      expect(find.byType(Drawer), findsOneWidget);
      expect(find.text('Warga Urena'), findsOneWidget);
      expect(find.text('WARGA'), findsOneWidget);
      expect(find.text('RT 001 / RW 002'), findsOneWidget);
    });

    testWidgets('Navigation Shell renders without overflow on narrow and wide screens for multiple roles', (tester) async {
      final bendaharaUser = {
        'id': 'user-bendahara',
        'email': 'bendahara@test.com',
        'name': 'Bendahara RT/RW',
        'role': AppRole.bendahara,
        'jabatan': 'bendahara',
        'rt': '001',
        'rw': '002',
        'rw_memberships': <Map<String, dynamic>>[
          {
            'rw_id': 'rw-002',
            'rw_name': 'RW 002 Urena',
            'jabatan': 'bendahara',
            'source': 'explicit',
          }
        ],
      };

      final router = GoRouter(
        initialLocation: '/home',
        routes: [
          ShellRoute(
            builder: (context, state, child) => MainNavigationShell(child: child),
            routes: [
              GoRoute(
                path: '/home',
                builder: (context, state) => const Text('Bendahara Home Content'),
              ),
            ],
          ),
        ],
      );

      // Test on small screen (320x640)
      tester.view.physicalSize = const Size(320 * 2, 640 * 2);
      tester.view.devicePixelRatio = 2.0;

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authRepositoryProvider.overrideWith((ref) => _MockAuthRepo(bendaharaUser)),
          ],
          child: MaterialApp.router(
            routerConfig: router,
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(tester.takeException(), isNull);
      expect(find.byType(BottomAppBar), findsOneWidget);

      // Test on wide tablet screen (800x1280)
      tester.view.physicalSize = const Size(800 * 2, 1280 * 2);
      await tester.pumpAndSettle();

      expect(tester.takeException(), isNull);

      // Reset physical size
      addTearDown(() {
        tester.view.resetPhysicalSize();
        tester.view.resetDevicePixelRatio();
      });
    });

    testWidgets('RwKeuanganScreen has 3 tabs (BERANDA, KAS, LOKASI DANA) and defaults to BERANDA', (tester) async {
      final mockUser = {
        'id': 'user-1',
        'name': 'Pengurus RW Test',
        'email': 'pengurus@rw.local',
        'role': 'pengurus',
        'system_role': 'warga',
        'rw_memberships': [
          {
            'rw_id': 'rw-002',
            'rw_number': 2,
            'rw_name': 'RW 002 Sukamaju',
            'jabatan': 'ketua',
            'status': 'active',
            'source': 'explicit',
          }
        ],
      };

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            rwApiClientProvider.overrideWithValue(MockRwApiClient()),
            authRepositoryProvider.overrideWith((ref) => _MockAuthRepo(mockUser)),
          ],
          child: const MaterialApp(
            home: RwKeuanganScreen(rwId: 'rw-002'),
          ),
        ),
      );

      await tester.pumpAndSettle();

      // Check the 3 Tab labels exist
      expect(find.widgetWithText(Tab, 'BERANDA'), findsOneWidget);
      expect(find.widgetWithText(Tab, 'KAS'), findsOneWidget);
      expect(find.widgetWithText(Tab, 'LOKASI DANA'), findsOneWidget);

      // Verify BERANDA is the default active tab showing welcome greeting
      expect(find.textContaining('Selamat datang'), findsOneWidget);
      expect(find.text('TOTAL SALDO'), findsOneWidget);
      expect(find.text('TOTAL PEMASUKAN'), findsOneWidget);
      expect(find.text('TOTAL PENGELUARAN'), findsOneWidget);
    });

    testWidgets('RwKeuanganScreen switches to KAS and LOKASI DANA tabs', (tester) async {
      final mockUser = {
        'id': 'user-1',
        'name': 'Pengurus RW Test',
        'email': 'pengurus@rw.local',
        'role': 'pengurus',
        'system_role': 'warga',
        'rw_memberships': [
          {
            'rw_id': 'rw-002',
            'rw_number': 2,
            'rw_name': 'RW 002 Sukamaju',
            'jabatan': 'ketua',
            'status': 'active',
            'source': 'explicit',
          }
        ],
      };

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            rwApiClientProvider.overrideWithValue(MockRwApiClient()),
            authRepositoryProvider.overrideWith((ref) => _MockAuthRepo(mockUser)),
          ],
          child: const MaterialApp(
            home: RwKeuanganScreen(rwId: 'rw-002'),
          ),
        ),
      );

      await tester.pumpAndSettle();

      // Tap KAS tab
      await tester.tap(find.widgetWithText(Tab, 'KAS'));
      await tester.pumpAndSettle();

      // KAS tab content should be visible
      expect(find.text('Catat Kas Masuk'), findsOneWidget);

      // Tap LOKASI DANA tab
      await tester.tap(find.widgetWithText(Tab, 'LOKASI DANA'));
      await tester.pumpAndSettle();

      // LOKASI DANA tab content should be visible
      expect(find.text('Rekening Bank'), findsAtLeastNWidgets(1));
      expect(find.text('Brankas Fisik'), findsAtLeastNWidgets(1));
    });

    testWidgets('RwKeuanganScreen does not crash on multiple rebuilds with tab count change', (tester) async {
      // User with explicit membership → LOKASI DANA tab shown (3 tabs).
      // This exercises the ticker recreation path: initState() creates a
      // TabController with length=2, then build() recreates it with length=3
      // when showLokasiDana is true. With SingleTickerProviderStateMixin this
      // would crash because createTicker() can only be called once.
      final explicitUser = {
        'id': 'user-1',
        'name': 'Pengurus RW Test',
        'email': 'pengurus@rw.local',
        'role': 'pengurus',
        'system_role': 'warga',
        'rw_memberships': [
          {
            'rw_id': 'rw-002',
            'rw_number': 2,
            'rw_name': 'RW 002 Sukamaju',
            'jabatan': 'ketua',
            'status': 'active',
            'source': 'explicit',
          }
        ],
      };

      // 1. Open screen with explicit user (3 tabs). First build triggers
      //    initState (length=2) then build() (recreate with length=3).
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            rwApiClientProvider.overrideWithValue(MockRwApiClient()),
            authRepositoryProvider.overrideWith((ref) => _MockAuthRepo(explicitUser)),
          ],
          child: const MaterialApp(
            home: RwKeuanganScreen(rwId: 'rw-002'),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.widgetWithText(Tab, 'BERANDA'), findsOneWidget);
      expect(find.widgetWithText(Tab, 'KAS'), findsOneWidget);
      expect(find.widgetWithText(Tab, 'LOKASI DANA'), findsOneWidget);
      expect(tester.takeException(), isNull);

      // 2. Rebuild with same explicit user (tab count unchanged → no controller
      //    recreation, but still exercises the TabController lifecycle).
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            rwApiClientProvider.overrideWithValue(MockRwApiClient()),
            authRepositoryProvider.overrideWith((ref) => _MockAuthRepo(explicitUser)),
          ],
          child: const MaterialApp(
            home: RwKeuanganScreen(rwId: 'rw-002'),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.widgetWithText(Tab, 'LOKASI DANA'), findsOneWidget);
      expect(tester.takeException(), isNull);

      // 3. Tap KAS tab and verify content (per-tab state preserved).
      await tester.tap(find.widgetWithText(Tab, 'KAS'));
      await tester.pumpAndSettle();
      expect(find.text('Catat Kas Masuk'), findsOneWidget);
      expect(tester.takeException(), isNull);
    });

    testWidgets('RwKeuanganScreen rt_inferred user shows 1 tab only (BERANDA)', (tester) async {
      final rtInferredUser = {
        'id': 'user-rt-inferred',
        'email': 'ketua.rt@test.com',
        'name': 'Ketua RT 002',
        'role': AppRole.warga,
        'rw': '002',
        'jabatan': 'ketua',
        'rw_memberships': [
          {'rw_id': 'rw-016', 'jabatan': 'ketua', 'rw_name': 'RW 016', 'source': 'rt_inferred'},
        ],
      };

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            rwApiClientProvider.overrideWithValue(MockRwApiClient()),
            authRepositoryProvider.overrideWith((ref) => _MockAuthRepo(rtInferredUser)),
          ],
          child: const MaterialApp(
            home: RwKeuanganScreen(rwId: 'rw-016'),
          ),
        ),
      );

      await tester.pumpAndSettle();

      // Only BERANDA tab for rt_inferred user (KAS and LOKASI DANA are hidden)
      expect(find.widgetWithText(Tab, 'BERANDA'), findsOneWidget);
      expect(find.widgetWithText(Tab, 'KAS'), findsNothing);
      expect(find.widgetWithText(Tab, 'LOKASI DANA'), findsNothing);
      expect(tester.takeException(), isNull);
    });

    testWidgets('RwDashboardScreen summary cards render without overflow at standard mobile width', (tester) async {
      tester.view.physicalSize = const Size(400 * 2, 800 * 2);
      tester.view.devicePixelRatio = 2.0;

      final mockUser = {
        'id': 'user-1',
        'name': 'Pengurus RW Test',
        'email': 'pengurus@rw.local',
        'role': 'pengurus',
        'system_role': 'warga',
        'rw_memberships': [
          {
            'rw_id': 'rw-002',
            'rw_number': 2,
            'rw_name': 'RW 002 Sukamaju',
            'jabatan': 'ketua',
            'status': 'active',
          }
        ],
      };

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            rwApiClientProvider.overrideWithValue(MockRwApiClient()),
            authRepositoryProvider.overrideWith((ref) => _MockAuthRepo(mockUser)),
          ],
          child: const MaterialApp(
            home: RwDashboardScreen(rwId: 'rw-002'),
          ),
        ),
      );

      await tester.pumpAndSettle();

      expect(tester.takeException(), isNull);
      expect(find.text('TOTAL SALDO'), findsOneWidget);
      expect(find.text('TOTAL PEMASUKAN'), findsOneWidget);
      expect(find.text('TOTAL PENGELUARAN'), findsOneWidget);

      addTearDown(() {
        tester.view.resetPhysicalSize();
        tester.view.resetDevicePixelRatio();
      });
    });

    testWidgets('Summary cards layout renders without overflow even at narrow 320px width', (tester) async {
      tester.view.physicalSize = const Size(320 * 2, 640 * 2);
      tester.view.devicePixelRatio = 2.0;

      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 20),
              child: Column(
                children: [
                  Container(
                    padding: const EdgeInsets.all(14),
                    decoration: BoxDecoration(
                      color: AppColors.accent.withValues(alpha: 0.05),
                      borderRadius: BorderRadius.circular(AppRadius.base),
                      border: Border.all(color: AppColors.accent.withValues(alpha: 0.1)),
                    ),
                    child: Row(
                      children: [
                        Container(
                          padding: const EdgeInsets.all(6),
                          decoration: BoxDecoration(
                            color: AppColors.accent.withValues(alpha: 0.1),
                            borderRadius: BorderRadius.circular(AppRadius.sm),
                          ),
                          child: const Icon(Icons.account_balance_wallet_outlined, size: 16, color: AppColors.accent),
                        ),
                        const SizedBox(width: 12),
                        const Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text('TOTAL SALDO', style: TextStyle(fontSize: 10, fontWeight: FontWeight.w600)),
                              SizedBox(height: 2),
                              FittedBox(
                                fit: BoxFit.scaleDown,
                                alignment: Alignment.centerLeft,
                                child: Text('Rp 15.000.000', style: TextStyle(fontSize: 16, fontWeight: FontWeight.w700)),
                              ),
                              Text('Periode: Semua', style: TextStyle(fontSize: 11)),
                            ],
                          ),
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(height: 12),
                  Row(
                    children: [
                      Expanded(
                        child: Container(
                          padding: const EdgeInsets.all(14),
                          decoration: BoxDecoration(
                            color: AppColors.success.withValues(alpha: 0.05),
                            borderRadius: BorderRadius.circular(AppRadius.base),
                            border: Border.all(color: AppColors.success.withValues(alpha: 0.1)),
                          ),
                          child: Row(
                            children: [
                              Container(
                                padding: const EdgeInsets.all(6),
                                decoration: BoxDecoration(
                                  color: AppColors.success.withValues(alpha: 0.1),
                                  borderRadius: BorderRadius.circular(AppRadius.sm),
                                ),
                                child: const Icon(Icons.arrow_downward_outlined, size: 16, color: AppColors.success),
                              ),
                              const SizedBox(width: 12),
                              const Expanded(
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Text('TOTAL PEMASUKAN', style: TextStyle(fontSize: 10, fontWeight: FontWeight.w600), maxLines: 1),
                                    SizedBox(height: 2),
                                    FittedBox(
                                      fit: BoxFit.scaleDown,
                                      alignment: Alignment.centerLeft,
                                      child: Text('Rp 5.000.000', style: TextStyle(fontSize: 16, fontWeight: FontWeight.w700)),
                                    ),
                                    Text('Periode: Semua', style: TextStyle(fontSize: 11)),
                                  ],
                                ),
                              ),
                            ],
                          ),
                        ),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: Container(
                          padding: const EdgeInsets.all(14),
                          decoration: BoxDecoration(
                            color: AppColors.danger.withValues(alpha: 0.05),
                            borderRadius: BorderRadius.circular(AppRadius.base),
                            border: Border.all(color: AppColors.danger.withValues(alpha: 0.1)),
                          ),
                          child: Row(
                            children: [
                              Container(
                                padding: const EdgeInsets.all(6),
                                decoration: BoxDecoration(
                                  color: AppColors.danger.withValues(alpha: 0.1),
                                  borderRadius: BorderRadius.circular(AppRadius.sm),
                                ),
                                child: const Icon(Icons.arrow_upward_outlined, size: 16, color: AppColors.danger),
                              ),
                              const SizedBox(width: 12),
                              const Expanded(
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Text('TOTAL PENGELUARAN', style: TextStyle(fontSize: 10, fontWeight: FontWeight.w600), maxLines: 1),
                                    SizedBox(height: 2),
                                    FittedBox(
                                      fit: BoxFit.scaleDown,
                                      alignment: Alignment.centerLeft,
                                      child: Text('Rp 2.000.000', style: TextStyle(fontSize: 16, fontWeight: FontWeight.w700)),
                                    ),
                                    Text('Periode: Semua', style: TextStyle(fontSize: 11)),
                                  ],
                                ),
                              ),
                            ],
                          ),
                        ),
                      ),
                    ],
                  ),
                ],
              ),
            ),
          ),
        ),
      );

      await tester.pumpAndSettle();

      expect(tester.takeException(), isNull);
      expect(find.text('TOTAL SALDO'), findsOneWidget);
      expect(find.text('TOTAL PEMASUKAN'), findsOneWidget);
      expect(find.text('TOTAL PENGELUARAN'), findsOneWidget);

      addTearDown(() {
        tester.view.resetPhysicalSize();
        tester.view.resetDevicePixelRatio();
      });
    });

    testWidgets('RT inferred membership → read-only UI for all RW screens', (tester) async {
      final rtInferredUser = {
        'id': 'user-rt-inferred',
        'email': 'ketua.rt@test.com',
        'name': 'Ketua RT 002',
        'role': AppRole.warga,
        'rw': '002',
        'jabatan': 'ketua',
        'rw_memberships': [
          {'rw_id': 'rw-016', 'jabatan': 'ketua', 'rw_name': 'RW 016', 'source': 'rt_inferred'},
        ],
      };

      // 1. Fund location screen should be read-only
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            rwApiClientProvider.overrideWithValue(MockRwApiClient()),
            authRepositoryProvider.overrideWith((ref) => _MockAuthRepo(rtInferredUser)),
          ],
          child: const MaterialApp(
            home: RwFundLocationScreen(rwId: 'rw-016'),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.textContaining('Mode Read-Only'), findsOneWidget);
      expect(find.text('Tambah Lokasi'), findsNothing);
      expect(find.text('Transfer Antar-Lokasi'), findsNothing);

      // 2. KAS screen should be read-only
      tester.view.physicalSize = const Size(1200, 1000);
      tester.view.devicePixelRatio = 1.0;
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            rwApiClientProvider.overrideWithValue(MockRwApiClient()),
            authRepositoryProvider.overrideWith((ref) => _MockAuthRepo(rtInferredUser)),
          ],
          child: const MaterialApp(
            home: RwKasScreen(rwId: 'rw-016'),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.textContaining('Mode Read-Only'), findsOneWidget);
      expect(find.text('Catat Kas Masuk'), findsNothing);
      expect(find.text('Catat Kas Keluar'), findsNothing);

      addTearDown(() {
        tester.view.resetPhysicalSize();
        tester.view.resetDevicePixelRatio();
      });
    });

    testWidgets('multi-RW: explicit RW has write, inferred RW has read-only', (tester) async {
      final multiMembershipUser = {
        'id': 'user-multi',
        'email': 'multi@test.com',
        'name': 'Ketua RT',
        'role': AppRole.warga,
        'jabatan': 'ketua',
        'rw_memberships': [
          {'rw_id': 'rw-016', 'jabatan': 'ketua', 'rw_name': 'RW 016', 'source': 'rt_inferred'},
          {'rw_id': 'rw-017', 'jabatan': 'bendahara', 'rw_name': 'RW 017', 'source': 'explicit'},
        ],
      };

      // 1. Viewing inferred RW-016 → read-only
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            rwApiClientProvider.overrideWithValue(MockRwApiClient()),
            authRepositoryProvider.overrideWith((ref) => _MockAuthRepo(multiMembershipUser)),
          ],
          child: const MaterialApp(
            home: RwFundLocationScreen(rwId: 'rw-016'),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.textContaining('Mode Read-Only'), findsOneWidget);
      expect(find.text('Tambah Lokasi'), findsNothing);
      expect(find.text('Transfer Antar-Lokasi'), findsNothing);

      // 2. Switching to explicit RW-017 → full controls
      tester.view.physicalSize = const Size(1200, 1000);
      tester.view.devicePixelRatio = 1.0;
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            rwApiClientProvider.overrideWithValue(MockRwApiClient()),
            authRepositoryProvider.overrideWith((ref) => _MockAuthRepo(multiMembershipUser)),
          ],
          child: const MaterialApp(
            home: RwKasScreen(rwId: 'rw-017'),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.textContaining('Mode Read-Only'), findsNothing);
      expect(find.text('Catat Kas Masuk'), findsOneWidget);
      expect(find.text('Catat Kas Keluar'), findsOneWidget);

      addTearDown(() {
        tester.view.resetPhysicalSize();
        tester.view.resetDevicePixelRatio();
      });
    });
  });
}

class _MockAuthRepo extends AuthRepository {
  _MockAuthRepo(Map<String, dynamic>? user)
      : super(
          authService: AuthService(
            ApiClient(baseUrl: 'http://test.local'),
            DeviceIdService(),
          ),
          apiClient: ApiClient(baseUrl: 'http://test.local'),
        ) {
    state = AsyncData(user);
  }
}
