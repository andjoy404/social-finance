import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/device_id_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/app/navigation_shell.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/notification/data/notification_repository.dart';
import 'package:social_finance/features/rw/data/rw_api_client.dart';
import 'package:social_finance/features/rw/data/rw_models.dart';
import 'package:social_finance/features/rw/data/rw_providers.dart';
import 'package:social_finance/features/rw/presentation/screens/rw_kas_screen.dart';

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

class _TestMockRwApiClient extends RwApiClient {
  _TestMockRwApiClient() : super(ApiClient(baseUrl: 'http://test.local'));

  @override
  Future<RwKasReportDetail> getKasReportDetail(
    String rwId, {
    String? period,
    String? startDate,
    String? endDate,
  }) async {
    return const RwKasReportDetail(
      period: '2026-10',
      startDate: '',
      endDate: '',
      openingBalance: '1000000.00',
      totalIncome: '5000000.00',
      totalExpense: '2000000.00',
      netBalance: '3000000.00',
      totalBalance: '4000000.00',
      transactionCount: 2,
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
          id: 'tx-income-1',
          rtId: 'rt-1',
          rtName: 'RT 001',
          sourceRtName: '001',
          type: 'income',
          amount: '5000000.00',
          description: 'Iuran Warga RT 001',
          categoryName: 'Iuran Warga',
          locationName: 'Kas Bank RW',
          paymentMethod: 'TRANSFER',
          status: 'posted',
          occurredAt: DateTime(2026, 10, 10),
          createdAt: DateTime(2026, 10, 10),
        ),
        RwTransaction(
          id: 'tx-expense-1',
          rtId: 'rt-1',
          rtName: 'Kas RW',
          type: 'expense',
          amount: '2000000.00',
          description: 'Perbaikan Lampu Jalan RW',
          expenseCategory: 'Pemeliharaan',
          locationName: 'Kas Operasional',
          paymentMethod: 'CASH',
          status: 'posted',
          occurredAt: DateTime(2026, 10, 12),
          createdAt: DateTime(2026, 10, 12),
        ),
      ],
      pagination: const RwPaginationInfo(
        page: 1,
        pageSize: 20,
        total: 2,
        totalPages: 1,
      ),
    );
  }

  @override
  Future<List<RwFundLocation>> getLocations(String rwId, {String? category, bool? isActive}) async => [];

  @override
  Future<List<RwRtInfo>> getRts(String rwId) async => [];

  @override
  Future<List<RwFinancialCategory>> getCategories({bool? isActive}) async => [];
}

void main() {
  final anggotaPengurusUser = {
    'id': 'user-anggota-pengurus',
    'email': 'anggota.pengurus@test.com',
    'name': 'Anggota Pengurus RT 01',
    'role': 'pengurus',
    'jabatan': 'anggota',
    'rt': '001',
    'rw': '002',
    'rw_id': 'rw-002',
    'rw_memberships': <Map<String, dynamic>>[
      {
        'rw_id': 'rw-002',
        'rw_name': 'RW 002 Sukajadi',
        'jabatan': 'anggota',
        'source': 'rt_inferred',
      }
    ],
  };

  group('Anggota RT Bottom Navigation (Requirement 1 & 4)', () {
    testWidgets('shows exactly 4 tabs in order: Data RW, Warga, Notifikasi, Profil', (tester) async {
      final router = GoRouter(
        initialLocation: '/home',
        routes: [
          ShellRoute(
            builder: (context, state, child) => MainNavigationShell(child: child),
            routes: [
              GoRoute(
                path: '/home',
                builder: (context, state) => const Text('Content Home'),
              ),
            ],
          ),
        ],
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authRepositoryProvider.overrideWith((ref) => _MockAuthRepo(anggotaPengurusUser)),
            notificationUnreadCountProvider.overrideWith((ref) => NotificationUnreadCountNotifierMock(0)),
          ],
          child: MaterialApp.router(routerConfig: router),
        ),
      );
      await tester.pumpAndSettle();

      // Exactly 4 tabs
      expect(find.descendant(of: find.byType(BottomAppBar), matching: find.byType(Expanded)), findsNWidgets(4));

      // 4 expected tabs exist
      expect(find.text('Data RW'), findsWidgets);
      expect(find.text('Warga'), findsWidgets);
      expect(find.text('Notifikasi'), findsWidgets);
      expect(find.text('Profil'), findsWidgets);

      // Other RT tabs must NOT exist
      expect(find.text('Beranda'), findsNothing);
      expect(find.text('Keuangan'), findsNothing);
      expect(find.text('Iuran'), findsNothing);
      expect(find.text('Kegiatan'), findsNothing);
      expect(find.text('Keamanan'), findsNothing);
      expect(find.text('Sampah'), findsNothing);
      expect(find.text('Lokasi Dana'), findsNothing);

      // No unread badge when unread count is 0
      expect(find.byKey(const Key('notification-unread-dot')), findsNothing);
    });

    testWidgets('shows red dot badge when unread notifications exist and hides when 0', (tester) async {
      final router = GoRouter(
        initialLocation: '/home',
        routes: [
          ShellRoute(
            builder: (context, state, child) => MainNavigationShell(child: child),
            routes: [
              GoRoute(
                path: '/home',
                builder: (context, state) => const Text('Content Home'),
              ),
            ],
          ),
        ],
      );

      // Unread count = 3 -> red dot must appear
      final unreadNotifier = NotificationUnreadCountNotifierMock(3);
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authRepositoryProvider.overrideWith((ref) => _MockAuthRepo(anggotaPengurusUser)),
            notificationUnreadCountProvider.overrideWith((ref) => unreadNotifier),
          ],
          child: MaterialApp.router(routerConfig: router),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('notification-unread-dot')), findsOneWidget);

      // Mark all as read (clear) -> red dot disappears
      unreadNotifier.clear();
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('notification-unread-dot')), findsNothing);
    });
  });

  group('Anggota RT Homepage and Route Redirections (Requirement 2)', () {
    testWidgets('navigating to /home redirects directly to /rw/:rwId/keuangan?tab=beranda', (tester) async {
      String? redirectedLocation;
      final router = GoRouter(
        initialLocation: '/home',
        redirect: (context, state) {
          final loc = state.matchedLocation;
          final user = anggotaPengurusUser;
          final jabatan = (user['jabatan'] ?? '').toString().trim().toLowerCase();
          final role = (user['role'] ?? '').toString().trim().toLowerCase();
          final isAnggotaRT = jabatan == 'anggota_rt' || (jabatan == 'anggota' && role == 'pengurus');
          if (isAnggotaRT) {
            final rwMemberships = (user['rw_memberships'] as List?)
                ?.whereType<Map<String, dynamic>>()
                .toList() ?? <Map<String, dynamic>>[];
            String rwId = rwMemberships.isNotEmpty
                ? (rwMemberships.first['rw_id'] as String? ?? '')
                : (user['rw_id'] ?? '').toString().trim();
            if (rwId.isNotEmpty) {
              if (loc == '/login' || loc == '/home' || loc == '/') {
                return '/rw/$rwId/keuangan?tab=beranda';
              }
              if (loc.startsWith('/rw/') ||
                  loc == '/home/rw-data' ||
                  loc.startsWith('/home/rw-data/') ||
                  loc == '/home/warga' ||
                  loc.startsWith('/home/warga/') ||
                  loc == '/home/notifikasi' ||
                  loc.startsWith('/home/notifikasi/') ||
                  loc == '/home/profile' ||
                  loc.startsWith('/home/profile/')) {
                return null;
              }
              return '/rw/$rwId/keuangan?tab=beranda';
            }
          }
          return null;
        },
        routes: [
          GoRoute(
            path: '/home',
            builder: (context, state) => const Text('RT Home'),
          ),
          GoRoute(
            path: '/rw/:rwId/keuangan',
            builder: (context, state) {
              redirectedLocation = state.uri.toString();
              return Text('RW Keuangan: ${state.uri}');
            },
          ),
          GoRoute(
            path: '/home/warga',
            builder: (context, state) => const Text('Warga Screen'),
          ),
        ],
      );

      await tester.pumpWidget(MaterialApp.router(routerConfig: router));
      await tester.pumpAndSettle();

      // Verified: Lands on RW Data (Beranda tab)
      expect(redirectedLocation, '/rw/rw-002/keuangan?tab=beranda');
      expect(find.text('RW Keuangan: /rw/rw-002/keuangan?tab=beranda'), findsOneWidget);
      expect(find.text('RT Home'), findsNothing);

      // Navigating to Warga is allowed
      router.go('/home/warga');
      await tester.pumpAndSettle();
      expect(find.text('Warga Screen'), findsOneWidget);

      // Navigating back to Data RW lands cleanly
      router.go('/rw/rw-002/keuangan?tab=beranda');
      await tester.pumpAndSettle();
      expect(find.text('RW Keuangan: /rw/rw-002/keuangan?tab=beranda'), findsOneWidget);
    });
  });

  group('KAS RW Card View (Requirement 3)', () {
    testWidgets('renders mobile vertical cards (no DataTable) for both Masuk and Keluar', (tester) async {
      tester.view.physicalSize = const Size(800, 1200);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(() {
        tester.view.resetPhysicalSize();
        tester.view.resetDevicePixelRatio();
      });

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            rwApiClientProvider.overrideWithValue(_TestMockRwApiClient()),
            authRepositoryProvider.overrideWith((ref) => _MockAuthRepo(anggotaPengurusUser)),
          ],
          child: const MaterialApp(
            home: RwKasScreen(rwId: 'rw-002'),
          ),
        ),
      );
      await tester.pumpAndSettle();

      // Verify no horizontal DataTable
      expect(find.byType(DataTable), findsNothing);

      // Verify cards exist
      expect(find.byType(RwKasTransactionCard), findsNWidgets(2));

      // Verify income card details
      expect(find.text('Iuran Warga RT 001'), findsOneWidget);
      expect(find.text('Kas Masuk'), findsWidgets);
      expect(find.text('+Rp 5.000.000'), findsOneWidget);

      // Verify expense card details
      expect(find.text('Perbaikan Lampu Jalan RW'), findsOneWidget);
      expect(find.text('Kas Keluar'), findsWidgets);
      expect(find.text('-Rp 2.000.000'), findsOneWidget);

      // Verify tapping expense card opens detail dialog and shows Category Name, NEVER UUID
      await tester.tap(find.text('Perbaikan Lampu Jalan RW'));
      await tester.pumpAndSettle();

      expect(find.text('Detail KAS Keluar RW'), findsOneWidget);
      expect(find.text('Kategori Kas Keluar:'), findsOneWidget);
      expect(
        find.descendant(
          of: find.byType(AlertDialog),
          matching: find.text('Pemeliharaan'),
        ),
        findsOneWidget,
      );
      expect(
        find.descendant(
          of: find.byType(AlertDialog),
          matching: find.textContaining('tx-expense-1'),
        ),
        findsNothing,
      );
      await tester.tap(find.text('Tutup'));
      await tester.pumpAndSettle();

      // Verify mode read-only for Anggota RT
      expect(find.textContaining('Mode Read-Only'), findsOneWidget);
      expect(find.text('Catat Kas Masuk'), findsNothing);
      expect(find.text('Catat Kas Keluar'), findsNothing);

      // Verify tapping card opens detail dialog
      await tester.tap(find.text('Iuran Warga RT 001'));
      await tester.pumpAndSettle();

      expect(find.text('Detail KAS Masuk RW'), findsOneWidget);
      expect(find.text('Tutup'), findsOneWidget);
      await tester.tap(find.text('Tutup'));
      await tester.pumpAndSettle();

      // Verify quick filter chips for Kas Masuk / Kas Keluar
      expect(find.text('Semua (2)'), findsOneWidget);
      expect(find.text('Masuk (1)'), findsOneWidget);
      expect(find.text('Keluar (1)'), findsOneWidget);

      // Tap 'Masuk (1)' -> only 1 card shown
      await tester.tap(find.text('Masuk (1)'));
      await tester.pumpAndSettle();
      expect(find.byType(RwKasTransactionCard), findsOneWidget);
      expect(find.text('Iuran Warga RT 001'), findsOneWidget);
      expect(find.text('Perbaikan Lampu Jalan RW'), findsNothing);

      // Tap 'Keluar (1)' -> only 1 card shown
      await tester.tap(find.text('Keluar (1)'));
      await tester.pumpAndSettle();
      expect(find.byType(RwKasTransactionCard), findsOneWidget);
      expect(find.text('Perbaikan Lampu Jalan RW'), findsOneWidget);
      expect(find.text('Iuran Warga RT 001'), findsNothing);
    });
  });
}

class NotificationUnreadCountNotifierMock extends StateNotifier<int>
    implements NotificationUnreadCountNotifier {
  NotificationUnreadCountNotifierMock(super.initial);

  @override
  Future<void> refresh() async {}

  @override
  void decrement() {
    if (state > 0) state = state - 1;
  }

  @override
  void clear() {
    state = 0;
  }
}
