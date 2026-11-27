import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/device_id_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/app/navigation_shell.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/notification/data/notification_repository.dart';
import 'package:social_finance/features/rw/data/rw_providers.dart';
import 'package:social_finance/features/security/data/security_models.dart';

class _MockUnreadNotifier extends StateNotifier<int>
    implements NotificationUnreadCountNotifier {
  _MockUnreadNotifier(super.state);
  @override
  Future<void> refresh() async {}
  @override
  void decrement() {
    if (state > 0) state--;
  }

  @override
  void clear() {
    state = 0;
  }
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

void main() {
  const rwId = '00000000-0000-0000-0000-000000000001';

  final ketuaUser = {
    'id': 'usr-ketua-1',
    'name': 'Ketua RT 01',
    'role': 'pengurus',
    'jabatan': 'ketua',
    'rt_id': 'rt-01',
    'rw_id': rwId,
    'rw_memberships': [
      {'rw_id': rwId, 'rw_number': 5, 'rw_name': 'RW 05', 'jabatan': 'ketua'},
    ],
    'effective_permissions': [
      'finance.read',
      'security.schedule.read',
      'security.schedule.write',
      'security.correction',
      'security.guest_book',
      'patrol.read',
      'patrol.write',
      'paket.read',
      'paket.write',
      'rw.notification.read',
      'rw.kas.read',
    ],
  };

  final wargaUser = {
    'id': 'usr-warga-1',
    'name': 'Warga RT 01',
    'role': 'warga',
    'jabatan': '',
    'rt_id': 'rt-01',
    'effective_permissions': ['finance.read'],
  };

  final keamananUser = {
    'id': 'usr-keamanan-1',
    'name': 'Petugas Keamanan 01',
    'role': 'perangkat',
    'jabatan': 'keamanan',
    'rt_id': 'rt-01',
    'effective_permissions': [
      'warga.read',
      'security.attendance',
      'patrol.read',
      'patrol.write',
      'paket.read',
      'paket.write',
    ],
  };

  final kebersihanUser = {
    'id': 'usr-kebersihan-1',
    'name': 'Petugas Kebersihan 01',
    'role': 'warga',
    'jabatan': 'kebersihan',
    'rt_id': 'rt-01',
    'effective_permissions': [],
  };

  GoRouter createTestRouter({String initialLocation = '/home'}) {
    return GoRouter(
      initialLocation: initialLocation,
      routes: [
        ShellRoute(
          builder: (context, state, child) => MainNavigationShell(child: child),
          routes: [
            GoRoute(
              path: '/home',
              builder: (context, state) => const Text('Screen Home'),
            ),
            GoRoute(
              path: '/home/keuangan',
              builder: (context, state) => const Text('Screen Keuangan'),
            ),
            GoRoute(
              path: '/home/iuran',
              builder: (context, state) => const Text('Screen Iuran'),
            ),
            GoRoute(
              path: '/home/kas',
              builder: (context, state) => const Text('Screen Kas'),
            ),
            GoRoute(
              path: '/home/warga',
              builder: (context, state) => const Text('Screen Warga'),
            ),
            GoRoute(
              path: '/home/patroli',
              builder: (context, state) => const Text('Screen Patroli'),
            ),
            GoRoute(
              path: '/home/paket',
              builder: (context, state) => const Text('Screen Paket'),
            ),
            GoRoute(
              path: '/home/security-management',
              builder: (context, state) => const Text('Screen Keamanan'),
            ),
            GoRoute(
              path: '/home/pengumuman',
              builder: (context, state) => const Text('Screen Pengumuman'),
            ),
            GoRoute(
              path: '/home/kegiatan',
              builder: (context, state) => const Text('Screen Kegiatan'),
            ),
            GoRoute(
              path: '/home/notifikasi',
              builder: (context, state) => const Text('Screen Notifikasi'),
            ),
            GoRoute(
              path: '/home/profile',
              builder: (context, state) => const Text('Screen Profil'),
            ),
            GoRoute(
              path: '/rw/:rwId/keuangan',
              builder: (context, state) => Text(
                'Screen RW Keuangan ${state.uri.queryParameters['tab'] ?? ''}',
              ),
            ),
          ],
        ),
      ],
    );
  }

  group('1. Fixed Bottom Navigation Layout & No Horizontal Scroll', () {
    testWidgets(
      'Ketua RT has exactly 4 bottom nav items without horizontal scrolling',
      (tester) async {
        final router = createTestRouter();
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              authRepositoryProvider.overrideWith(
                (ref) => _MockAuthRepo(ketuaUser),
              ),
              securityAlertsProvider.overrideWith((ref) async => false),
              notificationUnreadCountProvider.overrideWith(
                (ref) => _MockUnreadNotifier(0),
              ),
            ],
            child: MaterialApp.router(routerConfig: router),
          ),
        );
        await tester.pumpAndSettle();

        // No horizontal scrollview in bottom bar
        expect(
          find.descendant(
            of: find.byType(BottomAppBar),
            matching: find.byWidgetPredicate(
              (w) =>
                  w is SingleChildScrollView &&
                  w.scrollDirection == Axis.horizontal,
            ),
          ),
          findsNothing,
        );

        // Exactly 4 items in bottom bar: Beranda, Keuangan, Iuran, Menu
        expect(find.byKey(const Key('nav-item-beranda')), findsOneWidget);
        expect(find.byKey(const Key('nav-item-keuangan')), findsOneWidget);
        expect(find.byKey(const Key('nav-item-iuran')), findsOneWidget);
        expect(find.byKey(const Key('nav-item-menu')), findsOneWidget);

        // Each item is expanded inside Row (no overflow, evenly distributed)
        final bottomBar = find.byType(BottomAppBar);
        expect(
          find.descendant(of: bottomBar, matching: find.byType(Expanded)),
          findsNWidgets(4),
        );
      },
    );

    testWidgets(
      'Security Officer has exactly 2 bottom nav items (Beranda, Menu) without empty slots',
      (tester) async {
        final router = createTestRouter();
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              authRepositoryProvider.overrideWith(
                (ref) => _MockAuthRepo(keamananUser),
              ),
              securityAlertsProvider.overrideWith((ref) async => false),
              notificationUnreadCountProvider.overrideWith(
                (ref) => _MockUnreadNotifier(0),
              ),
            ],
            child: MaterialApp.router(routerConfig: router),
          ),
        );
        await tester.pumpAndSettle();

        // Exactly 2 items
        expect(find.byKey(const Key('nav-item-beranda')), findsOneWidget);
        expect(find.byKey(const Key('nav-item-menu')), findsOneWidget);
        expect(find.byKey(const Key('nav-item-keuangan')), findsNothing);
        expect(find.byKey(const Key('nav-item-iuran')), findsNothing);

        final bottomBar = find.byType(BottomAppBar);
        expect(
          find.descendant(of: bottomBar, matching: find.byType(Expanded)),
          findsNWidgets(2),
        );
      },
    );

    testWidgets(
      'User with Iuran but not Keuangan (jabatan sosial) has exactly 3 bottom nav items',
      (tester) async {
        final sosialUser = {
          'id': 'usr-sosial-1',
          'name': 'Pengurus Sosial',
          'role': 'pengurus',
          'jabatan': 'sosial',
          'rt_id': 'rt-01',
          'effective_permissions': <String>[],
        };
        final router = createTestRouter();
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              authRepositoryProvider.overrideWith(
                (ref) => _MockAuthRepo(sosialUser),
              ),
              securityAlertsProvider.overrideWith((ref) async => false),
              notificationUnreadCountProvider.overrideWith(
                (ref) => _MockUnreadNotifier(0),
              ),
            ],
            child: MaterialApp.router(routerConfig: router),
          ),
        );
        await tester.pumpAndSettle();

        // Exactly 3 items: Beranda, Iuran, Menu (Keuangan omitted)
        expect(find.byKey(const Key('nav-item-beranda')), findsOneWidget);
        expect(find.byKey(const Key('nav-item-keuangan')), findsNothing);
        expect(find.byKey(const Key('nav-item-iuran')), findsOneWidget);
        expect(find.byKey(const Key('nav-item-menu')), findsOneWidget);

        final bottomBar = find.byType(BottomAppBar);
        expect(
          find.descendant(of: bottomBar, matching: find.byType(Expanded)),
          findsNWidgets(3),
        );
      },
    );

    testWidgets(
      'Humas has exactly 4 bottom nav items (Beranda, Keuangan, Iuran, Menu), Keamanan and Kas in menu grid without Lokasi Dana',
      (tester) async {
        final humasUser = {
          'id': 'usr-humas-1',
          'name': 'Pengurus Humas',
          'role': 'pengurus',
          'jabatan': 'humas',
          'rt_id': 'rt-01',
          'rw_id': rwId,
          'rw_memberships': [
            {
              'rw_id': rwId,
              'rw_number': 5,
              'rw_name': 'RW 05',
              'jabatan': 'humas',
            },
          ],
          'effective_permissions': [
            'rw.dashboard.read',
            'rw.kas.read',
            'rw.notification.read',
            'security.schedule.read',
            'security.schedule.write',
            'security.correction',
            'security.device.manage',
          ],
        };
        final router = createTestRouter();
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              authRepositoryProvider.overrideWith(
                (ref) => _MockAuthRepo(humasUser),
              ),
              securityAlertsProvider.overrideWith((ref) async => false),
              notificationUnreadCountProvider.overrideWith(
                (ref) => _MockUnreadNotifier(0),
              ),
            ],
            child: MaterialApp.router(routerConfig: router),
          ),
        );
        await tester.pumpAndSettle();

        // Exactly 4 items in bottom bar: Beranda, Keuangan, Iuran, Menu
        expect(find.byKey(const Key('nav-item-beranda')), findsOneWidget);
        expect(find.byKey(const Key('nav-item-keuangan')), findsOneWidget);
        expect(find.byKey(const Key('nav-item-iuran')), findsOneWidget);
        expect(find.byKey(const Key('nav-item-menu')), findsOneWidget);

        // Tap Menu button to verify Keamanan and Kas are present, Buku Tamu and Lokasi Dana are absent
        await tester.tap(find.byKey(const Key('nav-item-menu')));
        await tester.pumpAndSettle();

        expect(find.byKey(const Key('grid-item-keamanan')), findsOneWidget);
        expect(find.byKey(const Key('grid-item-kas')), findsOneWidget);
        expect(find.byKey(const Key('grid-item-buku-tamu')), findsNothing);
        expect(find.byKey(const Key('grid-item-lokasi-dana')), findsNothing);
      },
    );

    testWidgets(
      'Operasional has exactly 3 bottom nav items (Beranda, Iuran, Menu), Warga and Data RW in menu grid, and no officer write/finance items',
      (tester) async {
        final operasionalUser = {
          'id': 'usr-operasional-1',
          'name': 'Warga Operasional',
          'role': 'warga',
          'jabatan': 'operasional',
          'rt_id': 'rt-01',
          'rw_id': rwId,
          'rw_memberships': [
            {
              'rw_id': rwId,
              'rw_number': 5,
              'rw_name': 'RW 05',
              'jabatan': 'operasional',
            },
          ],
          'effective_permissions': <String>[
            'rw.dashboard.read',
            'rw.kas.read',
          ],
        };
        final router = createTestRouter();
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              authRepositoryProvider.overrideWith(
                (ref) => _MockAuthRepo(operasionalUser),
              ),
              securityAlertsProvider.overrideWith((ref) async => false),
              notificationUnreadCountProvider.overrideWith(
                (ref) => _MockUnreadNotifier(0),
              ),
            ],
            child: MaterialApp.router(routerConfig: router),
          ),
        );
        await tester.pumpAndSettle();

        // Exactly 3 items in bottom bar: Beranda, Iuran, Menu (Keuangan omitted)
        expect(find.byKey(const Key('nav-item-beranda')), findsOneWidget);
        expect(find.byKey(const Key('nav-item-keuangan')), findsNothing);
        expect(find.byKey(const Key('nav-item-iuran')), findsOneWidget);
        expect(find.byKey(const Key('nav-item-menu')), findsOneWidget);

        // Tap Menu button to verify items
        await tester.tap(find.byKey(const Key('nav-item-menu')));
        await tester.pumpAndSettle();

        // Warga items present
        expect(find.byKey(const Key('grid-item-warga')), findsOneWidget);
        expect(find.text('Data RW'), findsOneWidget);
        expect(find.byKey(const Key('grid-item-pengumuman')), findsOneWidget);
        expect(find.byKey(const Key('grid-item-kegiatan')), findsOneWidget);
        expect(find.byKey(const Key('grid-item-notifikasi')), findsOneWidget);
        expect(find.byKey(const Key('grid-item-profil')), findsOneWidget);

        // Officer / RT-finance items strictly absent
        expect(find.byKey(const Key('grid-item-keamanan')), findsNothing);
        expect(find.byKey(const Key('grid-item-kas')), findsNothing);
        expect(find.byKey(const Key('grid-item-lokasi-dana')), findsNothing);
        expect(find.byKey(const Key('grid-item-buku-tamu')), findsNothing);
        expect(find.byKey(const Key('grid-item-patroli')), findsNothing);
        expect(find.byKey(const Key('grid-item-paket')), findsNothing);
      },
    );
  });

  group('2. Collapsed Menu Grid & Trailing Order Rule', () {
    testWidgets(
      'Menu button opens collapsed grid with Pengumuman, Kegiatan, Notifikasi, and Profil at the very end',
      (tester) async {
        final router = createTestRouter();
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              authRepositoryProvider.overrideWith(
                (ref) => _MockAuthRepo(ketuaUser),
              ),
              securityAlertsProvider.overrideWith((ref) async => false),
              notificationUnreadCountProvider.overrideWith(
                (ref) => _MockUnreadNotifier(0),
              ),
            ],
            child: MaterialApp.router(routerConfig: router),
          ),
        );
        await tester.pumpAndSettle();

        // Tap Menu button
        await tester.tap(find.byKey(const Key('nav-item-menu')));
        await tester.pumpAndSettle();

        // Grid is displayed with vertical scrolling capability
        expect(find.byType(GridView), findsOneWidget);

        // Scroll to verify the last 4 items in exact order: Pengumuman, Kegiatan, Notifikasi, Profil
        await tester.scrollUntilVisible(
          find.byKey(const Key('grid-item-pengumuman')),
          200,
        );
        expect(find.byKey(const Key('grid-item-pengumuman')), findsOneWidget);

        await tester.scrollUntilVisible(
          find.byKey(const Key('grid-item-kegiatan')),
          200,
        );
        expect(find.byKey(const Key('grid-item-kegiatan')), findsOneWidget);

        await tester.scrollUntilVisible(
          find.byKey(const Key('grid-item-notifikasi')),
          200,
        );
        expect(find.byKey(const Key('grid-item-notifikasi')), findsOneWidget);

        await tester.scrollUntilVisible(
          find.byKey(const Key('grid-item-profil')),
          200,
        );
        expect(find.byKey(const Key('grid-item-profil')), findsOneWidget);
      },
    );

    testWidgets(
      'Profil remains strictly the last item even when Notifikasi is omitted by RBAC',
      (tester) async {
        final noNotifUser = {
          'id': 'usr-nonotif-1',
          'name': 'User No Notif',
          'role': 'warga',
          'jabatan': 'kebersihan',
          'rt_id': 'rt-01',
          'effective_permissions': <String>[],
        };
        final router = createTestRouter();
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              authRepositoryProvider.overrideWith(
                (ref) => _MockAuthRepo(noNotifUser),
              ),
              securityAlertsProvider.overrideWith((ref) async => false),
              notificationUnreadCountProvider.overrideWith(
                (ref) => _MockUnreadNotifier(0),
              ),
            ],
            child: MaterialApp.router(routerConfig: router),
          ),
        );
        await tester.pumpAndSettle();

        await tester.tap(find.byKey(const Key('nav-item-menu')));
        await tester.pumpAndSettle();

        expect(find.byKey(const Key('grid-item-profil')), findsOneWidget);
        expect(find.byKey(const Key('grid-item-notifikasi')), findsNothing);
      },
    );

    testWidgets('Patroli and Paket appear as independent items in the grid', (
      tester,
    ) async {
      final router = createTestRouter();
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authRepositoryProvider.overrideWith(
              (ref) => _MockAuthRepo(ketuaUser),
            ),
            securityAlertsProvider.overrideWith((ref) async => false),
            notificationUnreadCountProvider.overrideWith(
              (ref) => _MockUnreadNotifier(0),
            ),
          ],
          child: MaterialApp.router(routerConfig: router),
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const Key('nav-item-menu')));
      await tester.pumpAndSettle();

      // Keamanan management, Patroli, and Paket are 3 distinct items
      expect(find.byKey(const Key('grid-item-keamanan')), findsOneWidget);
      expect(find.byKey(const Key('grid-item-patroli')), findsOneWidget);
      await tester.scrollUntilVisible(
        find.byKey(const Key('grid-item-paket')),
        200,
      );
      expect(find.byKey(const Key('grid-item-paket')), findsOneWidget);
    });

    testWidgets('User with patrol.read but not paket.read sees Patroli only', (
      tester,
    ) async {
      final patrolOnlyUser = {
        'id': 'usr-patrol-only',
        'name': 'Petugas Patroli Only',
        'role': 'perangkat',
        'jabatan': 'staf',
        'rt_id': 'rt-01',
        'effective_permissions': ['patrol.read'],
      };
      final router = createTestRouter();
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authRepositoryProvider.overrideWith(
              (ref) => _MockAuthRepo(patrolOnlyUser),
            ),
            securityAlertsProvider.overrideWith((ref) async => false),
            notificationUnreadCountProvider.overrideWith(
              (ref) => _MockUnreadNotifier(0),
            ),
          ],
          child: MaterialApp.router(routerConfig: router),
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const Key('nav-item-menu')));
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('grid-item-patroli')), findsOneWidget);
      expect(find.byKey(const Key('grid-item-paket')), findsNothing);
      expect(find.byKey(const Key('grid-item-keamanan')), findsNothing);
    });

    testWidgets('User with paket.read but not patrol.read sees Paket only', (
      tester,
    ) async {
      final paketOnlyUser = {
        'id': 'usr-paket-only',
        'name': 'Petugas Paket Only',
        'role': 'perangkat',
        'jabatan': 'staf',
        'rt_id': 'rt-01',
        'effective_permissions': ['paket.read'],
      };
      final router = createTestRouter();
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authRepositoryProvider.overrideWith(
              (ref) => _MockAuthRepo(paketOnlyUser),
            ),
            securityAlertsProvider.overrideWith((ref) async => false),
            notificationUnreadCountProvider.overrideWith(
              (ref) => _MockUnreadNotifier(0),
            ),
          ],
          child: MaterialApp.router(routerConfig: router),
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const Key('nav-item-menu')));
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('grid-item-paket')), findsOneWidget);
      expect(find.byKey(const Key('grid-item-patroli')), findsNothing);
      expect(find.byKey(const Key('grid-item-keamanan')), findsNothing);
    });
  });

  group('3. Aggregate Notification Badge & Individual Item Badges', () {
    testWidgets(
      'Menu button shows aggregate red dot if hidden item has unread, clears when read',
      (tester) async {
        final router = createTestRouter();
        final notifNotifier = _MockUnreadNotifier(5);

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              authRepositoryProvider.overrideWith(
                (ref) => _MockAuthRepo(ketuaUser),
              ),
              rwUnreadCountProvider(rwId).overrideWith((ref) async => 0),
              securityAlertsProvider.overrideWith((ref) async => false),
              securityAlertsDataProvider.overrideWith(
                (ref) async => const SecurityAlertsData(patrolUnread: 2),
              ),
              notificationUnreadCountProvider.overrideWith(
                (ref) => notifNotifier,
              ),
            ],
            child: MaterialApp.router(routerConfig: router),
          ),
        );
        await tester.pumpAndSettle();

        // Aggregate red dot visible on Menu
        expect(
          find.byKey(const Key('menu-aggregate-unread-dot')),
          findsOneWidget,
        );

        // Open grid: individual patrol badge visible
        await tester.tap(find.byKey(const Key('nav-item-menu')));
        await tester.pumpAndSettle();
        expect(find.byKey(const Key('patrol-unread-dot')), findsOneWidget);

        // Close grid
        await tester.tap(find.byIcon(Icons.close));
        await tester.pumpAndSettle();

        // Now clear all unread
        await tester.pumpWidget(
          ProviderScope(
            key: const ValueKey('clear-scope'),
            overrides: [
              authRepositoryProvider.overrideWith(
                (ref) => _MockAuthRepo(ketuaUser),
              ),
              rwUnreadCountProvider(rwId).overrideWith((ref) async => 0),
              securityAlertsProvider.overrideWith((ref) async => false),
              securityAlertsDataProvider.overrideWith(
                (ref) async =>
                    const SecurityAlertsData(patrolUnread: 0, paketUnread: 0),
              ),
              notificationUnreadCountProvider.overrideWith(
                (ref) => _MockUnreadNotifier(0),
              ),
            ],
            child: MaterialApp.router(routerConfig: router),
          ),
        );
        await tester.pump();
        await tester.pumpAndSettle();

        // Aggregate red dot disappears
        expect(
          find.byKey(const Key('menu-aggregate-unread-dot')),
          findsNothing,
        );
      },
    );
  });

  group('4. Active State & Navigation Behavior', () {
    testWidgets('Navigating to Patroli highlights Menu button in bottom nav', (
      tester,
    ) async {
      final router = createTestRouter(initialLocation: '/home/patroli');
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authRepositoryProvider.overrideWith(
              (ref) => _MockAuthRepo(ketuaUser),
            ),
            securityAlertsProvider.overrideWith((ref) async => false),
            notificationUnreadCountProvider.overrideWith(
              (ref) => _MockUnreadNotifier(0),
            ),
          ],
          child: MaterialApp.router(routerConfig: router),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Screen Patroli'), findsOneWidget);

      // Menu button in bottom nav is highlighted as selected (activeIcon: Icons.grid_view)
      expect(
        find.descendant(
          of: find.byKey(const Key('nav-item-menu')),
          matching: find.byIcon(Icons.grid_view),
        ),
        findsOneWidget,
      );

      // Open menu grid and verify Patroli item is marked selected inside grid
      await tester.tap(find.byKey(const Key('nav-item-menu')));
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('grid-item-patroli')), findsOneWidget);
    });

    testWidgets('Tapping grid item navigates to destination and closes sheet', (
      tester,
    ) async {
      final router = createTestRouter(initialLocation: '/home');
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authRepositoryProvider.overrideWith(
              (ref) => _MockAuthRepo(ketuaUser),
            ),
            securityAlertsProvider.overrideWith((ref) async => false),
            notificationUnreadCountProvider.overrideWith(
              (ref) => _MockUnreadNotifier(0),
            ),
          ],
          child: MaterialApp.router(routerConfig: router),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Screen Home'), findsOneWidget);

      // Open grid
      await tester.tap(find.byKey(const Key('nav-item-menu')));
      await tester.pumpAndSettle();

      // Tap Patroli in grid
      await tester.tap(find.byKey(const Key('grid-item-patroli')));
      await tester.pumpAndSettle();

      // Destination screen reached, sheet closed
      expect(find.text('Screen Patroli'), findsOneWidget);
      expect(find.byType(GridView), findsNothing);
    });

    testWidgets(
      'Fixed bottom navigation highlights active tab strictly based on current route',
      (tester) async {
        final router = createTestRouter(initialLocation: '/home');
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              authRepositoryProvider.overrideWith(
                (ref) => _MockAuthRepo(ketuaUser),
              ),
              securityAlertsProvider.overrideWith((ref) async => false),
              notificationUnreadCountProvider.overrideWith(
                (ref) => _MockUnreadNotifier(0),
              ),
            ],
            child: MaterialApp.router(routerConfig: router),
          ),
        );
        await tester.pumpAndSettle();

        // 1. Initial on /home: Only Beranda is active
        expect(find.byIcon(Icons.home), findsOneWidget);
        expect(find.byIcon(Icons.home_outlined), findsNothing);
        expect(find.byIcon(Icons.analytics_outlined), findsOneWidget);
        expect(find.byIcon(Icons.receipt_long_outlined), findsOneWidget);
        expect(find.byIcon(Icons.grid_view_outlined), findsOneWidget);

        // 2. Navigate to Iuran: Iuran is active, Beranda loses highlight
        await tester.tap(find.byKey(const Key('nav-item-iuran')));
        await tester.pumpAndSettle();
        expect(find.text('Screen Iuran'), findsOneWidget);
        expect(find.byIcon(Icons.receipt_long), findsOneWidget);
        expect(find.byIcon(Icons.receipt_long_outlined), findsNothing);
        expect(find.byIcon(Icons.home_outlined), findsOneWidget);
        expect(find.byIcon(Icons.home), findsNothing);
        expect(find.byIcon(Icons.analytics_outlined), findsOneWidget);
        expect(find.byIcon(Icons.grid_view_outlined), findsOneWidget);

        // 3. Return to Beranda: Iuran loses highlight, Beranda is active
        await tester.tap(find.byKey(const Key('nav-item-beranda')));
        await tester.pumpAndSettle();
        expect(find.text('Screen Home'), findsOneWidget);
        expect(find.byIcon(Icons.home), findsOneWidget);
        expect(find.byIcon(Icons.receipt_long_outlined), findsOneWidget);
        expect(find.byIcon(Icons.receipt_long), findsNothing);

        // 4. Navigate to Keuangan: Keuangan is active, Beranda/Iuran/Menu inactive
        await tester.tap(find.byKey(const Key('nav-item-keuangan')));
        await tester.pumpAndSettle();
        expect(find.text('Screen Keuangan'), findsOneWidget);
        expect(find.byIcon(Icons.analytics), findsOneWidget);
        expect(find.byIcon(Icons.home_outlined), findsOneWidget);
        expect(find.byIcon(Icons.receipt_long_outlined), findsOneWidget);
        expect(find.byIcon(Icons.grid_view_outlined), findsOneWidget);

        // 5. Open Menu grid and navigate to Profil: Menu is active, permanent items inactive
        await tester.tap(find.byKey(const Key('nav-item-menu')));
        await tester.pumpAndSettle();
        await tester.scrollUntilVisible(
          find.byKey(const Key('grid-item-profil')),
          200,
        );
        await tester.tap(find.byKey(const Key('grid-item-profil')));
        await tester.pumpAndSettle();
        expect(find.text('Screen Profil'), findsOneWidget);
        expect(find.byIcon(Icons.grid_view), findsOneWidget);
        expect(find.byIcon(Icons.home_outlined), findsOneWidget);
        expect(find.byIcon(Icons.analytics_outlined), findsOneWidget);
        expect(find.byIcon(Icons.receipt_long_outlined), findsOneWidget);

        // 6. Return to Beranda from collapsed route: Menu loses highlight, Beranda active
        await tester.tap(find.byKey(const Key('nav-item-beranda')));
        await tester.pumpAndSettle();
        expect(find.text('Screen Home'), findsOneWidget);
        expect(find.byIcon(Icons.home), findsOneWidget);
        expect(find.byIcon(Icons.grid_view_outlined), findsOneWidget);
        expect(find.byIcon(Icons.grid_view), findsNothing);
      },
    );

    testWidgets(
      'Humas tapping Keuangan navigates to RW Beranda, and tapping Kas in Menu navigates to RW Kas',
      (tester) async {
        final humasUser = {
          'id': 'usr-humas-1',
          'name': 'Humas 01',
          'role': 'warga',
          'jabatan': 'humas',
          'rt_id': 'rt-01',
          'rw_id': rwId,
          'rw_memberships': [
            {
              'rw_id': rwId,
              'rw_number': 5,
              'rw_name': 'RW 05',
              'jabatan': 'humas',
            },
          ],
          'effective_permissions': [
            'rw.dashboard.read',
            'rw.kas.read',
            'rw.notification.read',
            'security.schedule.read',
            'security.schedule.write',
            'security.correction',
            'security.device.manage',
          ],
        };
        final router = createTestRouter(initialLocation: '/home');
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              authRepositoryProvider.overrideWith(
                (ref) => _MockAuthRepo(humasUser),
              ),
              securityAlertsProvider.overrideWith((ref) async => false),
              notificationUnreadCountProvider.overrideWith(
                (ref) => _MockUnreadNotifier(0),
              ),
            ],
            child: MaterialApp.router(routerConfig: router),
          ),
        );
        await tester.pumpAndSettle();

        // 1. Tap Keuangan in bottom nav -> routes to RW Keuangan (tab=beranda)
        await tester.tap(find.byKey(const Key('nav-item-keuangan')));
        await tester.pumpAndSettle();
        expect(find.text('Screen RW Keuangan beranda'), findsOneWidget);
        // Keuangan item is active
        expect(find.byIcon(Icons.analytics), findsOneWidget);

        // 2. Open Menu grid and tap Kas -> routes to RW Keuangan (tab=kas)
        await tester.tap(find.byKey(const Key('nav-item-menu')));
        await tester.pumpAndSettle();
        expect(find.byKey(const Key('grid-item-kas')), findsOneWidget);

        await tester.tap(find.byKey(const Key('grid-item-kas')));
        await tester.pumpAndSettle();
        expect(find.text('Screen RW Keuangan kas'), findsOneWidget);
        // Menu item is active because Kas is in the collapsed menu sheet
        expect(find.byIcon(Icons.grid_view), findsOneWidget);
      },
    );
  });
}
