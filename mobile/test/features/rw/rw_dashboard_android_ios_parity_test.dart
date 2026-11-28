import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/api/device_id_service.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/dashboard/presentation/widgets/financial_summary_chart.dart';
import 'package:social_finance/features/rw/data/rw_api_client.dart';
import 'package:social_finance/features/rw/data/rw_models.dart';
import 'package:social_finance/features/rw/data/rw_providers.dart';
import 'package:social_finance/features/rw/presentation/screens/rw_dashboard_screen.dart';

class ParityMockRwApiClient extends RwApiClient {
  ParityMockRwApiClient({this.emptyData = false})
      : super(ApiClient(baseUrl: 'http://test.local'));

  final bool emptyData;
  final List<String> calls = [];

  @override
  Future<RwDashboardSummary> getDashboard(String rwId, {String? period}) async {
    calls.add('getDashboard(period: $period)');
    return RwDashboardSummary(
      totalRts: 3,
      totalLocations: 2,
      totalBalance: emptyData ? '0.00' : '15000000.00',
      totalIncome: emptyData ? '0.00' : '20000000.00',
      totalExpense: emptyData ? '0.00' : '5000000.00',
      transactionCount: emptyData ? 0 : 9,
    );
  }

  @override
  Future<RwTransactionsResponse> getTransactions({
    required String rwId,
    String? period,
    int page = 1,
    int pageSize = 20,
    String? search,
    String? startDate,
    String? endDate,
    String? categoryId,
    String? fundLocationId,
    String? type,
  }) async {
    calls.add('getTransactions(pageSize: $pageSize, period: $period, startDate: $startDate)');
    if (emptyData) {
      return const RwTransactionsResponse(
        data: [],
        pagination: RwPaginationInfo(page: 1, pageSize: 20, total: 0, totalPages: 0),
      );
    }

    final txs = [
      RwTransaction(
        id: 'tx-1',
        rwId: rwId,
        rtId: 'rt-1',
        rtName: 'RT 001',
        type: 'income',
        amount: '10000000.00',
        description: 'Setoran Kas RT 001',
        locationName: 'Rekening BCA',
        status: 'posted',
        occurredAt: DateTime(DateTime.now().year, 11, 23, 10, 0),
        createdAt: DateTime(DateTime.now().year, 11, 23, 10, 0),
      ),
      RwTransaction(
        id: 'tx-2',
        rwId: rwId,
        rtId: 'rt-2',
        rtName: 'RT 002',
        type: 'expense',
        amount: '2500000.00',
        description: 'Perbaikan Lampu Jalan',
        locationName: 'Kas Tunai',
        status: 'posted',
        occurredAt: DateTime(DateTime.now().year, 11, 24, 14, 0),
        createdAt: DateTime(DateTime.now().year, 11, 24, 14, 0),
      ),
    ];

    return RwTransactionsResponse(
      data: txs,
      pagination: RwPaginationInfo(page: 1, pageSize: pageSize, total: txs.length, totalPages: 1),
    );
  }
}

class _TestMockAuthRepo extends AuthRepository {
  _TestMockAuthRepo(Map<String, dynamic>? user)
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

void main() {
  final rtOfficialUser = {
    'id': 'u-rt-official',
    'name': 'Budi RT Official',
    'role': 'pengurus',
    'jabatan': 'ketua',
    'rt': '002',
    'rw': '016',
    'rw_id': 'rw-test-1',
    'rw_memberships': [
      {
        'rw_id': 'rw-test-1',
        'rw_number': 16,
        'rw_name': 'RW 016',
        'jabatan': 'anggota_rt',
        'source': 'rt_inferred',
      }
    ],
  };

  final rwExplicitAdminUser = {
    'id': 'u-rw-admin',
    'name': 'Mety RW Admin',
    'role': 'pengurus',
    'jabatan': 'ketua',
    'rw_id': 'rw-test-1',
    'rw_memberships': [
      {
        'rw_id': 'rw-test-1',
        'rw_number': 16,
        'rw_name': 'RW 016',
        'jabatan': 'ketua',
        'source': 'explicit',
      }
    ],
  };

  testWidgets(
    'Android (RT-inferred official): vertical bar chart & recent transactions render without client blocking',
    (tester) async {
      tester.view.physicalSize = const Size(360, 800);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(() {
        tester.view.resetPhysicalSize();
        tester.view.resetDevicePixelRatio();
      });

      final mockApi = ParityMockRwApiClient();

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authRepositoryProvider.overrideWith((ref) => _TestMockAuthRepo(rtOfficialUser)),
            rwApiClientProvider.overrideWithValue(mockApi),
          ],
          child: MaterialApp(
            theme: ThemeData(platform: TargetPlatform.android),
            home: const Scaffold(
              body: RwDashboardScreen(rwId: 'rw-test-1', isTabView: true),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      // Check API calls: both fetchRecentTx (pageSize: 5) and fetchYearTx (pageSize: 100) are called
      expect(mockApi.calls.any((c) => c.contains('pageSize: 5')), isTrue);
      expect(mockApi.calls.any((c) => c.contains('pageSize: 100')), isTrue);

      // Verify FinancialSummaryChart is displayed
      expect(find.byType(FinancialSummaryChart), findsOneWidget);
      expect(find.text('Ringkasan Keuangan'), findsOneWidget);

      // Verify Transaksi Terbaru panel renders actual items
      expect(find.text('Transaksi Terbaru'), findsOneWidget);
      expect(find.text('Setoran Kas RT 001'), findsOneWidget);
      expect(find.text('Perbaikan Lampu Jalan'), findsOneWidget);

      // No RenderFlex overflow or uncaught exceptions
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets(
    'iOS (Explicit RW admin): parity check - vertical bar chart & recent transactions render identically',
    (tester) async {
      tester.view.physicalSize = const Size(393, 852);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(() {
        tester.view.resetPhysicalSize();
        tester.view.resetDevicePixelRatio();
      });

      final mockApi = ParityMockRwApiClient();

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authRepositoryProvider.overrideWith((ref) => _TestMockAuthRepo(rwExplicitAdminUser)),
            rwApiClientProvider.overrideWithValue(mockApi),
          ],
          child: MaterialApp(
            theme: ThemeData(platform: TargetPlatform.iOS),
            home: const Scaffold(
              body: RwDashboardScreen(rwId: 'rw-test-1', isTabView: true),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(mockApi.calls.any((c) => c.contains('pageSize: 5')), isTrue);
      expect(mockApi.calls.any((c) => c.contains('pageSize: 100')), isTrue);
      expect(find.byType(FinancialSummaryChart), findsOneWidget);
      expect(find.text('Transaksi Terbaru'), findsOneWidget);
      expect(find.text('Setoran Kas RT 001'), findsOneWidget);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets(
    'Android empty transactions: renders clean empty state without crashing or overflowing',
    (tester) async {
      tester.view.physicalSize = const Size(360, 800);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(() {
        tester.view.resetPhysicalSize();
        tester.view.resetDevicePixelRatio();
      });

      final mockApi = ParityMockRwApiClient(emptyData: true);

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authRepositoryProvider.overrideWith((ref) => _TestMockAuthRepo(rtOfficialUser)),
            rwApiClientProvider.overrideWithValue(mockApi),
          ],
          child: MaterialApp(
            theme: ThemeData(platform: TargetPlatform.android),
            home: const Scaffold(
              body: RwDashboardScreen(rwId: 'rw-test-1', isTabView: true),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.byType(FinancialSummaryChart), findsOneWidget);
      expect(find.text('Belum ada transaksi di RW ini'), findsOneWidget);
      expect(tester.takeException(), isNull);
    },
  );
}
