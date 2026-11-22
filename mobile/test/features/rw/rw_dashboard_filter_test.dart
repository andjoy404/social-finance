import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/api/device_id_service.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/rw/data/rw_api_client.dart';
import 'package:social_finance/features/rw/data/rw_models.dart';
import 'package:social_finance/features/rw/data/rw_providers.dart';
import 'package:social_finance/features/rw/presentation/screens/rw_dashboard_screen.dart';

class TrackingRwApiClient extends RwApiClient {
  TrackingRwApiClient() : super(ApiClient(baseUrl: 'http://test.local'));

  final List<String?> dashboardPeriodCalls = [];
  final List<String?> recentTxPeriodCalls = [];

  @override
  Future<RwDashboardSummary> getDashboard(String rwId, {String? period}) async {
    dashboardPeriodCalls.add(period);
    return const RwDashboardSummary(
      totalRts: 5,
      totalLocations: 3,
      totalBalance: '15000000.00',
      totalIncome: '10000000.00',
      totalExpense: '2000000.00',
      transactionCount: 12,
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
    if (pageSize == 5) {
      recentTxPeriodCalls.add(period);
    }
    return const RwTransactionsResponse(
      data: [],
      pagination: RwPaginationInfo(page: 1, pageSize: 20, total: 0, totalPages: 1),
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
  final testUser = {
    'id': 'user-1',
    'name': 'Ketua RW Test',
    'role': 'pengurus',
    'rw_id': 'rw-test-1',
    'rw_name': 'RW 05',
    'rw_memberships': [
      {
        'rw_id': 'rw-test-1',
        'rw_number': 5,
        'rw_name': 'RW 05',
        'jabatan': 'ketua',
        'source': 'explicit',
      }
    ],
  };

  testWidgets(
    'Dashboard Filter Audit: month, year, all periods, refresh, and no stale data',
    (tester) async {
      tester.view.physicalSize = const Size(800, 1600);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(() {
        tester.view.resetPhysicalSize();
        tester.view.resetDevicePixelRatio();
      });

      final trackingClient = TrackingRwApiClient();

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            rwApiClientProvider.overrideWithValue(trackingClient),
            authRepositoryProvider.overrideWith(
              (ref) => _TestMockAuthRepo(testUser),
            ),
          ],
          child: const MaterialApp(
            home: Scaffold(
              body: RwDashboardScreen(rwId: 'rw-test-1'),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      // Initial load: fetches with initial active period (current year & month)
      expect(trackingClient.dashboardPeriodCalls.isNotEmpty, isTrue);
      expect(trackingClient.recentTxPeriodCalls.isNotEmpty, isTrue);
      final initialPeriod = trackingClient.dashboardPeriodCalls.first;
      expect(initialPeriod, isNotNull);
      expect(trackingClient.recentTxPeriodCalls.first, equals(initialPeriod));

      // 1. Open period picker
      await tester.tap(find.byKey(const Key('rw-dashboard-period-picker-button')));
      await tester.pumpAndSettle();

      // Bottom sheet is visible
      expect(find.textContaining('Pilih Periode'), findsOneWidget);
      expect(find.byKey(const Key('rw-dashboard-all-periods-btn')), findsOneWidget);

      // 2. Test selecting month (Maret)
      await tester.tap(find.text('Maret'));
      await tester.pumpAndSettle();

      // Verify that dashboard and transaction fetch were called with matching period (-03)
      expect(trackingClient.dashboardPeriodCalls.last?.endsWith('-03'), isTrue);
      expect(trackingClient.recentTxPeriodCalls.last, equals(trackingClient.dashboardPeriodCalls.last));

      // 3. Test changing year: open picker, click chevron left (<)
      await tester.tap(find.byKey(const Key('rw-dashboard-period-picker-button')));
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const Key('rw-dashboard-prev-year')));
      await tester.pumpAndSettle();

      // Pick Oktober in that previous year
      await tester.tap(find.text('Oktober'));
      await tester.pumpAndSettle();

      final previousYearPeriod = trackingClient.dashboardPeriodCalls.last;
      expect(previousYearPeriod?.endsWith('-10'), isTrue);
      expect(trackingClient.recentTxPeriodCalls.last, equals(previousYearPeriod));

      // 4. Test changing year forward after month was chosen
      await tester.tap(find.byKey(const Key('rw-dashboard-period-picker-button')));
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const Key('rw-dashboard-next-year')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('rw-dashboard-next-year')));
      await tester.pumpAndSettle();

      // Pick Desember in next year
      await tester.tap(find.text('Desember'));
      await tester.pumpAndSettle();

      final nextYearPeriod = trackingClient.dashboardPeriodCalls.last;
      expect(nextYearPeriod?.endsWith('-12'), isTrue);
      expect(trackingClient.recentTxPeriodCalls.last, equals(nextYearPeriod));

      // 5. Test "Semua Periode" button
      await tester.tap(find.byKey(const Key('rw-dashboard-period-picker-button')));
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const Key('rw-dashboard-all-periods-btn')));
      await tester.pumpAndSettle();

      // "Semua Periode" sends null as period
      expect(trackingClient.dashboardPeriodCalls.last, isNull);
      expect(trackingClient.recentTxPeriodCalls.last, isNull);
      expect(find.text('Semua Periode'), findsWidgets);

      // 6. Test Refresh: pulling refresh preserves period=null
      final callCountBeforeRefresh = trackingClient.dashboardPeriodCalls.length;
      await tester.fling(find.byType(SingleChildScrollView), const Offset(0, 400), 1000);
      await tester.pump();
      await tester.pump(const Duration(seconds: 1));
      await tester.pumpAndSettle();

      expect(trackingClient.dashboardPeriodCalls.length, greaterThan(callCountBeforeRefresh));
      expect(trackingClient.dashboardPeriodCalls.last, isNull);
      expect(trackingClient.recentTxPeriodCalls.last, isNull);

      // 7. Verify no stale data: when switching back to a specific month, values update
      await tester.tap(find.byKey(const Key('rw-dashboard-period-picker-button')));
      await tester.pumpAndSettle();

      await tester.tap(find.text('Januari'));
      await tester.pumpAndSettle();

      expect(trackingClient.dashboardPeriodCalls.last?.endsWith('-01'), isTrue);
      expect(trackingClient.recentTxPeriodCalls.last, equals(trackingClient.dashboardPeriodCalls.last));
    },
  );
}
