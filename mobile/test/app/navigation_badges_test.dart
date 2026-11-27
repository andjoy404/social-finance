import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/device_id_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/app/navigation_shell.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/rw/data/rw_providers.dart';
import 'package:social_finance/features/notification/data/notification_repository.dart';

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
      'security.schedule.read',
      'security.schedule.write',
      'security.correction',
      'security.guest_book',
      'rw.notification.read',
      'rw.kas.read',
    ],
  };

  testWidgets('shows Red Dot on Data RW when rw unread > 0, hides when 0', (
    tester,
  ) async {
    final router = GoRouter(
      initialLocation: '/home',
      routes: [
        ShellRoute(
          builder: (context, state, child) => MainNavigationShell(child: child),
          routes: [
            GoRoute(
              path: '/home',
              builder: (context, state) => const Text('Home Page'),
            ),
          ],
        ),
      ],
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authRepositoryProvider.overrideWith(
            (ref) => _MockAuthRepo(ketuaUser),
          ),
          rwUnreadCountProvider(rwId).overrideWith((ref) async => 2),
          securityAlertsProvider.overrideWith((ref) async => false),
        ],
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('menu-aggregate-unread-dot')), findsOneWidget);
    await tester.tap(find.byKey(const Key('nav-item-menu')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('rw-unread-dot')), findsOneWidget);
    expect(find.byKey(const Key('security-unread-dot')), findsNothing);
    await tester.tap(find.byIcon(Icons.close));
    await tester.pumpAndSettle();

    // Now re-pump with 0 unread
    await tester.pumpWidget(
      ProviderScope(
        key: const ValueKey('scope-2'),
        overrides: [
          authRepositoryProvider.overrideWith(
            (ref) => _MockAuthRepo(ketuaUser),
          ),
          rwUnreadCountProvider(rwId).overrideWith((ref) async => 0),
          securityAlertsProvider.overrideWith((ref) async => false),
        ],
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('menu-aggregate-unread-dot')), findsNothing);
    await tester.tap(find.byKey(const Key('nav-item-menu')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('rw-unread-dot')), findsNothing);
    await tester.tap(find.byIcon(Icons.close));
    await tester.pumpAndSettle();
  });

  testWidgets(
    'shows Red Dot on Keamanan when security alert is true, hides when false',
    (tester) async {
      final router = GoRouter(
        initialLocation: '/home',
        routes: [
          ShellRoute(
            builder: (context, state, child) =>
                MainNavigationShell(child: child),
            routes: [
              GoRoute(
                path: '/home',
                builder: (context, state) => const Text('Home Page'),
              ),
            ],
          ),
        ],
      );

      await tester.pumpWidget(
        ProviderScope(
          key: const ValueKey('scope-security-1'),
          overrides: [
            authRepositoryProvider.overrideWith(
              (ref) => _MockAuthRepo(ketuaUser),
            ),
            rwUnreadCountProvider(rwId).overrideWith((ref) async => 0),
            securityAlertsProvider.overrideWith((ref) async => true),
          ],
          child: MaterialApp.router(routerConfig: router),
        ),
      );
      await tester.pumpAndSettle();

      expect(
        find.byKey(const Key('menu-aggregate-unread-dot')),
        findsOneWidget,
      );
      await tester.tap(find.byKey(const Key('nav-item-menu')));
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('security-unread-dot')), findsOneWidget);
      expect(find.byKey(const Key('rw-unread-dot')), findsNothing);
      await tester.tap(find.byIcon(Icons.close));
      await tester.pumpAndSettle();

      // Now re-pump with alert false
      await tester.pumpWidget(
        ProviderScope(
          key: const ValueKey('scope-security-2'),
          overrides: [
            authRepositoryProvider.overrideWith(
              (ref) => _MockAuthRepo(ketuaUser),
            ),
            rwUnreadCountProvider(rwId).overrideWith((ref) async => 0),
            securityAlertsProvider.overrideWith((ref) async => false),
          ],
          child: MaterialApp.router(routerConfig: router),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('menu-aggregate-unread-dot')), findsNothing);
    },
  );

  testWidgets(
    'Petugas Keamanan has exactly 6 menus and suppresses all red dot badges',
    (tester) async {
      final officerUser = {
        'id': 'usr-keamanan-1',
        'name': 'Petugas Keamanan',
        'role': 'pengurus',
        'jabatan': 'keamanan',
        'rt_id': 'rt-01',
        'rw_id': rwId,
        'effective_permissions': [
          'patrol.create',
          'patrol.read',
          'paket.create',
          'paket.read',
          'paket.status_change',
          'security.attendance',
        ],
      };

      final router = GoRouter(
        initialLocation: '/home/patroli',
        routes: [
          ShellRoute(
            builder: (context, state, child) =>
                MainNavigationShell(child: child),
            routes: [
              GoRoute(
                path: '/home/warga',
                builder: (context, state) => const Text('Warga Page'),
              ),
              GoRoute(
                path: '/home/absen',
                builder: (context, state) => const Text('Absensi Page'),
              ),
              GoRoute(
                path: '/home/patroli',
                builder: (context, state) => const Text('Patroli Page'),
              ),
              GoRoute(
                path: '/home/notifikasi',
                builder: (context, state) => const Text('Sampah Page'),
              ),
              GoRoute(
                path: '/home/paket',
                builder: (context, state) => const Text('Paket Page'),
              ),
              GoRoute(
                path: '/home/profile',
                builder: (context, state) => const Text('Profil Page'),
              ),
            ],
          ),
        ],
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authRepositoryProvider.overrideWith(
              (ref) => _MockAuthRepo(officerUser),
            ),
            rwUnreadCountProvider(rwId).overrideWith((ref) async => 5),
            securityAlertsProvider.overrideWith((ref) async => true),
          ],
          child: MaterialApp.router(routerConfig: router),
        ),
      );
      await tester.pumpAndSettle();

      // Bottom navigation contains fixed items: Beranda and Menu
      expect(find.byKey(const Key('nav-item-beranda')), findsOneWidget);
      expect(find.byKey(const Key('nav-item-menu')), findsOneWidget);
      expect(find.byKey(const Key('nav-item-keuangan')), findsNothing);
      expect(find.byKey(const Key('nav-item-iuran')), findsNothing);

      // Open collapsed menu grid
      await tester.tap(find.byKey(const Key('nav-item-menu')));
      await tester.pumpAndSettle();

      // Verify exactly 6 navigation items exist in grid
      expect(find.byKey(const Key('nav-item-warga')), findsOneWidget);
      expect(find.byKey(const Key('nav-item-absensi')), findsOneWidget);
      expect(find.byKey(const Key('nav-item-patroli')), findsOneWidget);
      expect(find.byKey(const Key('nav-item-sampah')), findsOneWidget);
      expect(find.byKey(const Key('nav-item-paket')), findsOneWidget);
      expect(find.byKey(const Key('nav-item-profil')), findsOneWidget);

      // Verify all badges are suppressed for officer
      expect(find.byKey(const Key('patrol-unread-dot')), findsNothing);
      expect(find.byKey(const Key('paket-unread-dot')), findsNothing);
      expect(find.byKey(const Key('rw-unread-dot')), findsNothing);
      expect(find.byKey(const Key('security-unread-dot')), findsNothing);
      expect(find.byKey(const Key('notification-unread-dot')), findsNothing);
    },
  );

  testWidgets('Sekretaris RT does not have Lokasi Dana and Keamanan in tabs', (
    tester,
  ) async {
    final sekretarisUser = {
      'id': 'usr-sekretaris-1',
      'name': 'Sekretaris RT 01',
      'role': 'pengurus',
      'jabatan': 'sekretaris',
      'rt_id': 'rt-01',
      'rw_id': rwId,
      'effective_permissions': [
        'activities.read',
        'announcements.read',
        'warga.read',
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
              builder: (context, state) => const Text('Home Page'),
            ),
          ],
        ),
      ],
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authRepositoryProvider.overrideWith(
            (ref) => _MockAuthRepo(sekretarisUser),
          ),
        ],
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('nav-item-lokasi dana')), findsNothing);
    expect(find.byKey(const Key('nav-item-keamanan')), findsNothing);
  });

  testWidgets('shows Red Dot on Notifikasi when unread > 0, hides when 0', (
    tester,
  ) async {
    final router = GoRouter(
      initialLocation: '/home',
      routes: [
        ShellRoute(
          builder: (context, state, child) => MainNavigationShell(child: child),
          routes: [
            GoRoute(
              path: '/home',
              builder: (context, state) => const Text('Home Page'),
            ),
            GoRoute(
              path: '/home/notifikasi',
              builder: (context, state) => const Text('Notifikasi Page'),
            ),
          ],
        ),
      ],
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authRepositoryProvider.overrideWith(
            (ref) => _MockAuthRepo(ketuaUser),
          ),
          rwUnreadCountProvider(rwId).overrideWith((ref) async => 0),
          securityAlertsProvider.overrideWith((ref) async => false),
          notificationUnreadCountProvider.overrideWith(
            (ref) => _MockUnreadNotifier(3),
          ),
        ],
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('menu-aggregate-unread-dot')), findsOneWidget);
    await tester.tap(find.byKey(const Key('nav-item-menu')));
    await tester.pumpAndSettle();
    await tester.scrollUntilVisible(
      find.byKey(const Key('notification-unread-dot')),
      200,
    );
    expect(find.byKey(const Key('notification-unread-dot')), findsOneWidget);
    await tester.tap(find.byIcon(Icons.close));
    await tester.pumpAndSettle();

    // Now re-pump with 0 unread
    await tester.pumpWidget(
      ProviderScope(
        key: const ValueKey('notif-scope-2'),
        overrides: [
          authRepositoryProvider.overrideWith(
            (ref) => _MockAuthRepo(ketuaUser),
          ),
          rwUnreadCountProvider(rwId).overrideWith((ref) async => 0),
          securityAlertsProvider.overrideWith((ref) async => false),
          notificationUnreadCountProvider.overrideWith(
            (ref) => _MockUnreadNotifier(0),
          ),
        ],
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('menu-aggregate-unread-dot')), findsNothing);
  });

  testWidgets('operasional and sosial see Iuran but hide Kas', (
    tester,
  ) async {
    for (final roleJabatan in ['sosial', 'operasional']) {
      final user = {
        'id': 'usr-$roleJabatan-1',
        'name': 'User $roleJabatan',
        'role': 'pengurus',
        'jabatan': roleJabatan,
        'rt_id': 'rt-01',
        'effective_permissions': ['warga.read', 'security.guest_book'],
      };

      final router = GoRouter(
        initialLocation: '/home',
        routes: [
          ShellRoute(
            builder: (context, state, child) =>
                MainNavigationShell(child: child),
            routes: [
              GoRoute(
                path: '/home',
                builder: (context, state) => const Text('Home Page'),
              ),
            ],
          ),
        ],
      );

      await tester.pumpWidget(
        ProviderScope(
          key: ValueKey('scope-$roleJabatan'),
          overrides: [
            authRepositoryProvider.overrideWith((ref) => _MockAuthRepo(user)),
            securityAlertsProvider.overrideWith((ref) async => false),
            notificationUnreadCountProvider.overrideWith(
              (ref) => _MockUnreadNotifier(0),
            ),
          ],
          child: MaterialApp.router(routerConfig: router),
        ),
      );
      await tester.pumpAndSettle();

      expect(
        find.text('KAS RT'),
        findsNothing,
        reason: '$roleJabatan should not see KAS RT',
      );
      expect(
        find.text('Iuran'),
        findsOneWidget,
        reason: '$roleJabatan should see Iuran',
      );
    }
  });

  testWidgets('ketua has Buku Tamu in navigation tabs', (tester) async {
    final router = GoRouter(
      initialLocation: '/home',
      routes: [
        ShellRoute(
          builder: (context, state, child) => MainNavigationShell(child: child),
          routes: [
            GoRoute(
              path: '/home',
              builder: (context, state) => const Text('Home Page'),
            ),
          ],
        ),
      ],
    );

    await tester.pumpWidget(
      ProviderScope(
        key: const ValueKey('scope-buku-tamu-ketua'),
        overrides: [
          authRepositoryProvider.overrideWith(
            (ref) => _MockAuthRepo(ketuaUser),
          ),
          rwUnreadCountProvider(rwId).overrideWith((ref) async => 0),
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
    expect(find.text('Buku Tamu'), findsOneWidget);
  });
}
