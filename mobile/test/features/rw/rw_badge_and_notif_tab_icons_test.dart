import 'package:dio/dio.dart';
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
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/notification/data/models/notification_models.dart';
import 'package:social_finance/features/notification/data/notification_api_client.dart';
import 'package:social_finance/features/notification/data/notification_repository.dart';
import 'package:social_finance/features/notification/presentation/screens/notification_management_screen.dart';
import 'package:social_finance/features/rw/data/rw_api_client.dart';
import 'package:social_finance/features/rw/data/rw_models.dart';
import 'package:social_finance/features/rw/data/rw_providers.dart';
import 'package:social_finance/features/rw/presentation/screens/rw_keuangan_screen.dart';
import 'package:social_finance/features/rw/presentation/screens/rw_notification_screen.dart';

class _FakeDeviceId extends DeviceIdService {
  @override
  Future<String> getDeviceId() async => 'test-device';
}

class _FakeAuthRepo extends AuthRepository {
  _FakeAuthRepo(Map<String, dynamic>? user)
      : super(
          authService: AuthService(
            ApiClient(baseUrl: 'http://test.local'),
            _FakeDeviceId(),
          ),
          apiClient: ApiClient(baseUrl: 'http://test.local'),
        ) {
    state = AsyncData(user);
  }
}

class _MockUnreadNotifier extends StateNotifier<int>
    implements NotificationUnreadCountNotifier {
  _MockUnreadNotifier(super.state);
  int refreshCalls = 0;

  @override
  Future<void> refresh() async {
    refreshCalls++;
  }

  @override
  void decrement() {
    if (state > 0) state--;
  }

  @override
  void clear() {
    state = 0;
  }
}

class _StatefulRwApiClient extends RwApiClient {
  _StatefulRwApiClient({
    this.unreadCount = 0,
    List<RwNotificationLog>? logs,
  })  : _logs = logs ?? const [],
        super(ApiClient(baseUrl: 'http://test.local'));

  int unreadCount;
  int unreadCountFetchCalls = 0;
  final List<RwNotificationLog> _logs;

  @override
  Future<int> getRWUnreadCount(String rwId) async {
    unreadCountFetchCalls++;
    return unreadCount;
  }

  @override
  Future<RwDashboardSummary> getDashboard(String rwId, {String? period}) async =>
      const RwDashboardSummary(
        totalRts: 1,
        totalLocations: 0,
        totalBalance: '0',
        totalIncome: '0',
        totalExpense: '0',
        transactionCount: 0,
      );

  @override
  Future<RwKasReport> getKas(String rwId, {String? period}) async =>
      const RwKasReport(
        totalIncome: '0',
        totalExpense: '0',
        netBalance: '0',
        transactionCount: 0,
      );

  @override
  Future<RwKasReportDetail> getKasReportDetail(
    String rwId, {
    String? period,
    String? startDate,
    String? endDate,
  }) async =>
      const RwKasReportDetail(
        period: '',
        startDate: '',
        endDate: '',
        openingBalance: '0.00',
        totalIncome: '0',
        totalExpense: '0',
        netBalance: '0',
        transactionCount: 0,
      );

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
  }) async =>
      const RwTransactionsResponse(
        data: [],
        pagination: RwPaginationInfo(
          page: 1,
          pageSize: 20,
          total: 0,
          totalPages: 1,
        ),
      );

  @override
  Future<List<RwFundLocation>> getLocations(
    String rwId, {
    String? category,
    bool? isActive,
  }) async =>
      const [];

  @override
  Future<RwNotificationLogsPage> getNotifications(
    String rwId, {
    int page = 1,
    int pageSize = 20,
    String? eventType,
  }) async =>
      RwNotificationLogsPage(
        items: _logs,
        page: 1,
        pageSize: 20,
        total: _logs.length,
        totalPages: 1,
      );

  @override
  Future<List<RwNotificationTemplate>> listNotificationTemplates(
    String rwId, {
    String? eventType,
  }) async =>
      const [];
}

class _FakeNotificationRepo extends NotificationRepository {
  _FakeNotificationRepo({
    this.onMarkRead,
    this.onMarkAllRead,
    this.failMarkRead = false,
  }) : super(apiClient: NotificationApiClient(dio: Dio()));

  final void Function(String id)? onMarkRead;
  final void Function()? onMarkAllRead;
  final bool failMarkRead;
  final List<String> markedIds = [];
  final List<String> requestedCategories = [];

  @override
  Future<List<SystemNotificationType>> fetchSystemTypes() async => const [
        SystemNotificationType(
          eventType: 'iuran_bill_created',
          name: 'Penerbitan Tagihan Iuran',
          trigger: 'Otomatis saat tagihan dibuat',
          recipientsRule: 'Warga RT',
          defaultTitle: 'Tagihan Baru',
          defaultBody: 'Tagihan baru tersedia.',
          allowedVariables: ['bill_id'],
        ),
        SystemNotificationType(
          eventType: 'waste_truck_arrival',
          name: 'Kedatangan Mobil Sampah',
          trigger: 'Manual oleh petugas',
          recipientsRule: 'Seluruh warga RT',
          defaultTitle: 'Mobil Sampah Tiba',
          defaultBody: 'Mobil sampah tiba.',
          allowedVariables: [],
        ),
      ];

  @override
  Future<List<NotificationTemplate>> fetchTemplates() async => const [];

  @override
  Future<NotificationLogsResponse> fetchManagementHistory({
    required String category,
    int page = 1,
    int perPage = 50,
  }) async {
    requestedCategories.add(category);
    return NotificationLogsResponse(
      data: [
        NotificationLogEntry(
          id: 'notif-$category',
          rtId: 'rt-1',
          eventType: category == 'mine' ? 'manual_push' : 'iuran_bill_created',
          title: category == 'mine' ? 'Riwayat Pribadi' : 'Riwayat Sistem',
          body: 'Detail $category',
          variables: const {},
          recipientsCount: 1,
          historyCategory: category,
          createdAt: DateTime(2026, 11, 20, 9, 0),
        ),
      ],
      page: 1,
      perPage: 50,
      total: 1,
      totalPages: 1,
    );
  }

  @override
  Future<void> markAsRead(String id) async {
    if (failMarkRead) {
      throw Exception('network error');
    }
    markedIds.add(id);
    onMarkRead?.call(id);
  }

  @override
  Future<int> markAllAsRead() async {
    onMarkAllRead?.call();
    return 1;
  }

  @override
  Future<int> fetchUnreadCount() async => 0;
}

void main() {
  const rwId = 'rw-test-001';
  final rwKetuaUser = <String, dynamic>{
    'id': 'usr-ketua-rw',
    'name': 'Ketua RW 01',
    'email': 'ketua.rw@local.com',
    'role': 'pengurus',
    'jabatan': 'ketua',
    'rt_id': 'rt-01',
    'rw_id': rwId,
    'rw_memberships': [
      {
        'rw_id': rwId,
        'rw_number': 1,
        'rw_name': 'RW 001',
        'jabatan': 'ketua',
        'source': 'explicit',
      },
    ],
    'effective_permissions': [
      'rw.notification.read',
      'rw.notification.write',
      'rw.kas.read',
      'rw.kas.write',
      'notification.manage',
    ],
  };

  GoRouter buildShellRouter({String initialLocation = '/home'}) {
    return GoRouter(
      initialLocation: initialLocation,
      routes: [
        ShellRoute(
          builder: (context, state, child) => MainNavigationShell(child: child),
          routes: [
            GoRoute(
              path: '/home',
              builder: (context, state) => const Center(child: Text('Home')),
            ),
            GoRoute(
              path: '/rw/:rwId/keuangan',
              builder: (context, state) => RwKeuanganScreen(
                rwId: state.pathParameters['rwId'] ?? rwId,
              ),
            ),
            GoRoute(
              path: '/home/notifikasi',
              builder: (context, state) => const NotificationManagementScreen(),
            ),
          ],
        ),
      ],
    );
  }

  group('TASK 1 — DATA RW Unread Badge Per Tab & Bottom Nav Sync', () {
    testWidgets(
      '1. Tidak ada unread -> badge bottom nav DATA RW dan semua tab DATA RW tidak muncul',
      (tester) async {
        final api = _StatefulRwApiClient(unreadCount: 0);
        final router = buildShellRouter(initialLocation: '/rw/$rwId/keuangan');

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              authRepositoryProvider.overrideWith((_) => _FakeAuthRepo(rwKetuaUser)),
              rwApiClientProvider.overrideWithValue(api),
              securityAlertsProvider.overrideWith((_) async => false),
              notificationUnreadCountProvider.overrideWith(
                (_) => _MockUnreadNotifier(0),
              ),
            ],
            child: MaterialApp.router(routerConfig: router),
          ),
        );
        await tester.pumpAndSettle();

        expect(find.byKey(const Key('menu-aggregate-unread-dot')), findsNothing);
        expect(find.byKey(const Key('rw-unread-dot')), findsNothing);
        expect(find.byKey(const Key('rw-tab-beranda-unread-dot')), findsNothing);
        expect(find.byKey(const Key('rw-tab-kas-unread-dot')), findsNothing);
        expect(find.byKey(const Key('rw-tab-lokasi-dana-unread-dot')), findsNothing);
        expect(find.byKey(const Key('rw-tab-notifikasi-unread-dot')), findsNothing);

        await tester.tap(find.text('NOTIFIKASI'));
        await tester.pumpAndSettle();
        expect(find.byKey(const Key('rw-subtab-riwayat-unread-dot')), findsNothing);
      },
    );

    testWidgets(
      '2. Tab NOTIFIKASI unread -> bottom nav DATA RW, tab NOTIFIKASI, dan subtab Riwayat muncul; tab lain tidak',
      (tester) async {
        final api = _StatefulRwApiClient(
          unreadCount: 2,
          logs: [
            RwNotificationLog(
              id: 'rw-log-1',
              rwId: rwId,
              eventType: 'manual_push',
              title: 'Pengumuman RW',
              body: 'Rapat RW malam ini',
              createdAt: DateTime(2026, 11, 20, 10, 0),
            ),
          ],
        );
        final router = buildShellRouter(initialLocation: '/rw/$rwId/keuangan');

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              authRepositoryProvider.overrideWith((_) => _FakeAuthRepo(rwKetuaUser)),
              rwApiClientProvider.overrideWithValue(api),
              securityAlertsProvider.overrideWith((_) async => false),
              notificationUnreadCountProvider.overrideWith(
                (_) => _MockUnreadNotifier(0),
              ),
            ],
            child: MaterialApp.router(routerConfig: router),
          ),
        );
        await tester.pumpAndSettle();

        // Bottom nav Menu aggregate dot + Data RW item dot
        expect(find.byKey(const Key('menu-aggregate-unread-dot')), findsOneWidget);
        expect(find.byKey(const Key('rw-tab-notifikasi-unread-dot')), findsOneWidget);
        expect(find.byKey(const Key('rw-tab-beranda-unread-dot')), findsNothing);
        expect(find.byKey(const Key('rw-tab-kas-unread-dot')), findsNothing);
        expect(find.byKey(const Key('rw-tab-lokasi-dana-unread-dot')), findsNothing);

        // Open Menu sheet -> Data RW has rw-unread-dot
        await tester.tap(find.byKey(const Key('nav-item-menu')));
        await tester.pumpAndSettle();
        expect(find.byKey(const Key('rw-unread-dot')), findsOneWidget);
        await tester.tap(find.byIcon(Icons.close));
        await tester.pumpAndSettle();

        // Switch to NOTIFIKASI tab -> subtab Riwayat Notifikasi Sistem shows dot
        await tester.tap(find.text('NOTIFIKASI'));
        await tester.pumpAndSettle();
        expect(find.byKey(const Key('rw-subtab-riwayat-unread-dot')), findsOneWidget);
      },
    );

    testWidgets(
      '3-5. Multi-tab state: Tab A (KAS) saja, Tab A + B (KAS & NOTIFIKASI), lalu Tab A dibaca sementara Tab B masih unread',
      (tester) async {
        final api = _StatefulRwApiClient(unreadCount: 0);
        final router = buildShellRouter(initialLocation: '/rw/$rwId/keuangan');

        // Case 3: Only Tab A (KAS) unread
        await tester.pumpWidget(
          ProviderScope(
            key: const ValueKey('tab-a-only'),
            overrides: [
              authRepositoryProvider.overrideWith((_) => _FakeAuthRepo(rwKetuaUser)),
              rwApiClientProvider.overrideWithValue(api),
              rwTabUnreadStateProvider(rwId).overrideWithValue(
                const RwTabUnreadState(kasUnread: 1),
              ),
              securityAlertsProvider.overrideWith((_) async => false),
              notificationUnreadCountProvider.overrideWith(
                (_) => _MockUnreadNotifier(0),
              ),
            ],
            child: MaterialApp.router(routerConfig: router),
          ),
        );
        await tester.pumpAndSettle();

        expect(find.byKey(const Key('menu-aggregate-unread-dot')), findsOneWidget);
        expect(find.byKey(const Key('rw-tab-kas-unread-dot')), findsOneWidget);
        expect(find.byKey(const Key('rw-tab-notifikasi-unread-dot')), findsNothing);

        // Case 4: Tab A (KAS) and Tab B (NOTIFIKASI) both unread
        await tester.pumpWidget(
          ProviderScope(
            key: const ValueKey('tab-a-and-b'),
            overrides: [
              authRepositoryProvider.overrideWith((_) => _FakeAuthRepo(rwKetuaUser)),
              rwApiClientProvider.overrideWithValue(api),
              rwTabUnreadStateProvider(rwId).overrideWithValue(
                const RwTabUnreadState(kasUnread: 1, notifikasiUnread: 2),
              ),
              securityAlertsProvider.overrideWith((_) async => false),
              notificationUnreadCountProvider.overrideWith(
                (_) => _MockUnreadNotifier(0),
              ),
            ],
            child: MaterialApp.router(routerConfig: router),
          ),
        );
        await tester.pumpAndSettle();

        expect(find.byKey(const Key('menu-aggregate-unread-dot')), findsOneWidget);
        expect(find.byKey(const Key('rw-tab-kas-unread-dot')), findsOneWidget);
        expect(find.byKey(const Key('rw-tab-notifikasi-unread-dot')), findsOneWidget);

        // Case 5: Tab A read (0), Tab B still unread (2) -> Tab A dot gone, Tab B + bottom nav stay
        await tester.pumpWidget(
          ProviderScope(
            key: const ValueKey('tab-b-only'),
            overrides: [
              authRepositoryProvider.overrideWith((_) => _FakeAuthRepo(rwKetuaUser)),
              rwApiClientProvider.overrideWithValue(api),
              rwTabUnreadStateProvider(rwId).overrideWithValue(
                const RwTabUnreadState(kasUnread: 0, notifikasiUnread: 2),
              ),
              securityAlertsProvider.overrideWith((_) async => false),
              notificationUnreadCountProvider.overrideWith(
                (_) => _MockUnreadNotifier(0),
              ),
            ],
            child: MaterialApp.router(routerConfig: router),
          ),
        );
        await tester.pumpAndSettle();

        expect(find.byKey(const Key('rw-tab-kas-unread-dot')), findsNothing);
        expect(find.byKey(const Key('rw-tab-notifikasi-unread-dot')), findsOneWidget);
        expect(find.byKey(const Key('menu-aggregate-unread-dot')), findsOneWidget);
      },
    );

    testWidgets(
      '6. Membaca notifikasi RW di Riwayat Notifikasi Sistem menandai read dan menghilangkan semua badge saat unread=0',
      (tester) async {
        tester.view.physicalSize = const Size(412, 915);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(tester.view.resetPhysicalSize);
        addTearDown(tester.view.resetDevicePixelRatio);

        final api = _StatefulRwApiClient(
          unreadCount: 1,
          logs: [
            RwNotificationLog(
              id: 'rw-log-unread-1',
              rwId: rwId,
              eventType: 'manual_push',
              title: 'Info Penting RW',
              body: 'Harap hadir tepat waktu.',
              status: 'SENT',
              source: 'rw',
              createdAt: DateTime(2026, 11, 20, 10, 0),
            ),
          ],
        );
        final notifRepo = _FakeNotificationRepo(
          onMarkRead: (_) {
            if (api.unreadCount > 0) api.unreadCount--;
          },
        );
        final unreadNotifier = _MockUnreadNotifier(1);
        final router = buildShellRouter(initialLocation: '/rw/$rwId/keuangan');

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              authRepositoryProvider.overrideWith((_) => _FakeAuthRepo(rwKetuaUser)),
              rwApiClientProvider.overrideWithValue(api),
              notificationRepositoryProvider.overrideWithValue(notifRepo),
              securityAlertsProvider.overrideWith((_) async => false),
              notificationUnreadCountProvider.overrideWith((_) => unreadNotifier),
            ],
            child: MaterialApp.router(routerConfig: router),
          ),
        );
        await tester.pumpAndSettle();

        expect(find.byKey(const Key('rw-tab-notifikasi-unread-dot')), findsOneWidget);

        await tester.tap(find.text('NOTIFIKASI'));
        await tester.pumpAndSettle();
        expect(find.byKey(const Key('rw-subtab-riwayat-unread-dot')), findsOneWidget);

        await tester.tap(find.text('Riwayat Notifikasi Sistem'));
        await tester.pumpAndSettle();

        // Tap the unread RW log item -> opens preview & marks read via existing endpoint
        await tester.tap(find.byKey(const Key('rwNotifLog_rw-log-unread-1')));
        await tester.pumpAndSettle();
        expect(find.text('Preview Notifikasi'), findsOneWidget);
        expect(notifRepo.markedIds, contains('rw-log-unread-1'));

        await tester.tap(find.text('Tutup'));
        await tester.pumpAndSettle();

        // All badges cleared cleanly
        expect(find.byKey(const Key('rw-tab-notifikasi-unread-dot')), findsNothing);
        expect(find.byKey(const Key('rw-subtab-riwayat-unread-dot')), findsNothing);
        expect(find.byKey(const Key('rw-unread-dot')), findsNothing);
        expect(unreadNotifier.refreshCalls, greaterThanOrEqualTo(1));
      },
    );

    testWidgets(
      '7 & 8. Navigasi keluar-masuk tidak stale & refresh tidak menghilangkan unread yang masih valid',
      (tester) async {
        final api = _StatefulRwApiClient(
          unreadCount: 2,
          logs: [
            RwNotificationLog(
              id: 'rw-log-fail-1',
              rwId: rwId,
              eventType: 'manual_push',
              title: 'Masih Unread',
              body: 'Belum dibaca.',
              createdAt: DateTime(2026, 11, 20, 10, 0),
            ),
          ],
        );
        // Simulate markAsRead network failure -> badge must NOT falsely disappear
        final failingNotifRepo = _FakeNotificationRepo(failMarkRead: true);
        final router = buildShellRouter(initialLocation: '/rw/$rwId/keuangan');

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              authRepositoryProvider.overrideWith((_) => _FakeAuthRepo(rwKetuaUser)),
              rwApiClientProvider.overrideWithValue(api),
              notificationRepositoryProvider.overrideWithValue(failingNotifRepo),
              securityAlertsProvider.overrideWith((_) async => false),
              notificationUnreadCountProvider.overrideWith(
                (_) => _MockUnreadNotifier(0),
              ),
            ],
            child: MaterialApp.router(routerConfig: router),
          ),
        );
        await tester.pumpAndSettle();

        expect(find.byKey(const Key('menu-aggregate-unread-dot')), findsOneWidget);
        expect(find.byKey(const Key('rw-tab-notifikasi-unread-dot')), findsOneWidget);

        // Navigate out to /home while server still has unread = 2 -> badge stays valid
        router.go('/home');
        await tester.pumpAndSettle();
        expect(find.byKey(const Key('menu-aggregate-unread-dot')), findsOneWidget);

        // Server unread cleared externally -> navigating back to DATA RW refreshes and clears stale badge
        api.unreadCount = 0;
        router.go('/rw/$rwId/keuangan');
        await tester.pumpAndSettle();
        expect(find.byKey(const Key('menu-aggregate-unread-dot')), findsNothing);
        expect(find.byKey(const Key('rw-tab-notifikasi-unread-dot')), findsNothing);

        // Server gets new unread = 2 -> navigating out and back shows badge again
        api.unreadCount = 2;
        router.go('/home');
        await tester.pumpAndSettle();
        expect(find.byKey(const Key('menu-aggregate-unread-dot')), findsOneWidget);

        router.go('/rw/$rwId/keuangan');
        await tester.pumpAndSettle();
        expect(find.byKey(const Key('rw-tab-notifikasi-unread-dot')), findsOneWidget);
      },
    );

    testWidgets(
      'Sync: Tandai Semua Dibaca di NotificationManagementScreen mengosongkan badge DATA RW',
      (tester) async {
        final api = _StatefulRwApiClient(unreadCount: 3);
        final notifRepo = _FakeNotificationRepo(
          onMarkAllRead: () {
            api.unreadCount = 0;
          },
        );
        final router = buildShellRouter(initialLocation: '/home/notifikasi');

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              authRepositoryProvider.overrideWith((_) => _FakeAuthRepo(rwKetuaUser)),
              rwApiClientProvider.overrideWithValue(api),
              notificationRepositoryProvider.overrideWithValue(notifRepo),
              notificationSystemTypesProvider.overrideWith(
                (_) => notifRepo.fetchSystemTypes(),
              ),
              notificationTemplatesProvider.overrideWith(
                (_) => notifRepo.fetchTemplates(),
              ),
              securityAlertsProvider.overrideWith((_) async => false),
              notificationUnreadCountProvider.overrideWith(
                (_) => _MockUnreadNotifier(3),
              ),
            ],
            child: MaterialApp.router(routerConfig: router),
          ),
        );
        await tester.pumpAndSettle();

        // Initially DATA RW has unread = 3 -> menu aggregate dot visible
        expect(find.byKey(const Key('menu-aggregate-unread-dot')), findsOneWidget);

        // Tap "Tandai Semua Dibaca"
        await tester.tap(find.byIcon(Icons.done_all_rounded));
        await tester.pumpAndSettle();

        // Both notification unread AND RW unread refreshed to 0 -> badge gone!
        expect(find.byKey(const Key('menu-aggregate-unread-dot')), findsNothing);
      },
    );
  });

  group('TASK 2 — Icon Tab NOTIFIKASI → NOTIFIKASI (Parity with DATA RW)', () {
    testWidgets(
      'SISTEM and RIWAYAT SAYA have icons matching DATA RW reference style, preserve wording/order/function, and have zero overflow on 320px',
      (tester) async {
        for (final brightness in [Brightness.light, Brightness.dark]) {
          tester.view.physicalSize = const Size(320, 640);
          tester.view.devicePixelRatio = 1.0;

          final notifRepo = _FakeNotificationRepo();
          final rwApi = _StatefulRwApiClient(unreadCount: 0);

          // 1) Inspect reference TabBar in DATA RW → NOTIFIKASI
          await tester.pumpWidget(
            ProviderScope(
              key: ValueKey('rw-ref-$brightness'),
              overrides: [
                authRepositoryProvider.overrideWith(
                  (_) => _FakeAuthRepo(rwKetuaUser),
                ),
                rwApiClientProvider.overrideWithValue(rwApi),
              ],
              child: MaterialApp(
                theme: ThemeData(brightness: brightness),
                home: const Scaffold(body: RwNotificationScreen(rwId: rwId)),
              ),
            ),
          );
          await tester.pumpAndSettle();

          final refTabBar = tester.widget<TabBar>(
            find.byKey(const Key('rwNotifSubTabBar')),
          );

          // 2) Inspect target TabBar in NOTIFIKASI → NOTIFIKASI
          await tester.pumpWidget(
            ProviderScope(
              key: ValueKey('notif-target-$brightness'),
              overrides: [
                authRepositoryProvider.overrideWith(
                  (_) => _FakeAuthRepo(rwKetuaUser),
                ),
                notificationRepositoryProvider.overrideWithValue(notifRepo),
                notificationSystemTypesProvider.overrideWith(
                  (_) => notifRepo.fetchSystemTypes(),
                ),
                notificationTemplatesProvider.overrideWith(
                  (_) => notifRepo.fetchTemplates(),
                ),
              ],
              child: MaterialApp(
                theme: ThemeData(brightness: brightness),
                home: const NotificationManagementScreen(),
              ),
            ),
          );
          await tester.pumpAndSettle();

          final historyTabBarFinder = find.byKey(const Key('notifHistorySubTabBar'));
          expect(historyTabBarFinder, findsOneWidget);
          final targetTabBar = tester.widget<TabBar>(historyTabBarFinder);

          // Visual style parity with DATA RW → NOTIFIKASI reference
          expect(targetTabBar.tabAlignment, refTabBar.tabAlignment);
          expect(targetTabBar.labelPadding, refTabBar.labelPadding);
          expect(targetTabBar.labelStyle, refTabBar.labelStyle);
          expect(targetTabBar.unselectedLabelStyle, refTabBar.unselectedLabelStyle);
          expect(targetTabBar.labelColor, refTabBar.labelColor);
          expect(targetTabBar.unselectedLabelColor, refTabBar.unselectedLabelColor);
          expect(targetTabBar.indicatorColor, refTabBar.indicatorColor);
          expect(targetTabBar.indicatorWeight, refTabBar.indicatorWeight);
          expect(targetTabBar.labelColor, AppColors.accent);

          // Icons present with exact 18px size matching DATA RW reference
          final sistemIconFinder = find.descendant(
            of: historyTabBarFinder,
            matching: find.byIcon(Icons.apps_rounded),
          );
          final riwayatIconFinder = find.descendant(
            of: historyTabBarFinder,
            matching: find.byIcon(Icons.history_outlined),
          );
          expect(sistemIconFinder, findsOneWidget);
          expect(riwayatIconFinder, findsOneWidget);
          expect(tester.widget<Icon>(sistemIconFinder).size, 18);
          expect(tester.widget<Icon>(riwayatIconFinder).size, 18);

          // Wording & order preserved
          final sistemText = find.descendant(
            of: historyTabBarFinder,
            matching: find.text('SISTEM (2)'),
          );
          final riwayatText = find.descendant(
            of: historyTabBarFinder,
            matching: find.text('RIWAYAT SAYA'),
          );
          expect(sistemText, findsOneWidget);
          expect(riwayatText, findsOneWidget);
          expect(
            tester.getCenter(sistemText).dx,
            lessThan(tester.getCenter(riwayatText).dx),
          );

          // Functionality preserved (system -> mine)
          expect(notifRepo.requestedCategories, contains('system'));
          expect(find.text('Riwayat Sistem'), findsOneWidget);

          await tester.tap(riwayatText);
          await tester.pumpAndSettle();
          expect(notifRepo.requestedCategories, contains('mine'));
          expect(find.text('Riwayat Pribadi'), findsOneWidget);

          // Zero layout overflow on 320px narrow screen
          expect(tester.takeException(), isNull);
        }

        tester.view.resetPhysicalSize();
        tester.view.resetDevicePixelRatio();
      },
    );
  });
}
