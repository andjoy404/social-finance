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

class _NotificationUnreadCountMock extends StateNotifier<int>
    implements NotificationUnreadCountNotifier {
  _NotificationUnreadCountMock(super.initial);
  @override
  Future<void> refresh() async {}

  @override
  void decrement() {
    if (state > 0) state = state - 1;
  }

  @override
  void setUnreadCount(int count) {
    state = count;
  }

  @override
  void clear() {
    state = 0;
  }
}

void main() {
  final anggotaRwUser = {
    'id': 'user-anggota-rw',
    'email': 'anggota.rw@test.com',
    'name': 'Petugas Anggota RW',
    'role': 'warga',
    'jabatan': 'anggota_rw',
    'rt': '001',
    'rw': '002',
    'rw_id': 'rw-002',
    'rw_memberships': <Map<String, dynamic>>[
      {
        'rw_id': 'rw-002',
        'rw_name': 'RW 002 Sukajadi',
        'jabatan': 'anggota_rw',
        'source': 'rt_inferred',
      },
    ],
  };

  group('Anggota RW Bottom Navigation', () {
    testWidgets(
      'displays strictly 3 bottom nav tabs: Data RW, Notifikasi, and Profil',
      (tester) async {
        final router = GoRouter(
          initialLocation: '/rw/rw-002/keuangan',
          routes: [
            ShellRoute(
              builder: (context, state, child) =>
                  MainNavigationShell(child: child),
              routes: [
                GoRoute(
                  path: '/rw/:rwId/keuangan',
                  builder: (context, state) => const Text('Content Data RW'),
                ),
                GoRoute(
                  path: '/home/notifikasi',
                  builder: (context, state) => const Text('Content Notifikasi'),
                ),
                GoRoute(
                  path: '/home/profile',
                  builder: (context, state) => const Text('Content Profil'),
                ),
              ],
            ),
          ],
        );

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              authRepositoryProvider.overrideWith(
                (ref) => _MockAuthRepo(anggotaRwUser),
              ),
              notificationUnreadCountProvider.overrideWith(
                (ref) => _NotificationUnreadCountMock(0),
              ),
            ],
            child: MaterialApp.router(routerConfig: router),
          ),
        );
        await tester.pumpAndSettle();

        // Exactly 3 tabs in bottom nav
        expect(
          find.descendant(
            of: find.byType(BottomAppBar),
            matching: find.byType(Expanded),
          ),
          findsNWidgets(3),
        );
        expect(find.text('Data RW'), findsOneWidget);
        expect(find.text('Notifikasi'), findsOneWidget);
        expect(find.text('Profil'), findsOneWidget);

        // Other RT tabs must NOT exist
        expect(find.text('Beranda'), findsNothing);
        expect(find.text('Menu'), findsNothing);
        expect(find.text('Keuangan'), findsNothing);
        expect(find.text('Iuran'), findsNothing);
        expect(find.text('Warga'), findsNothing);
      },
    );

    testWidgets(
      'active indicator follows current route and tapping tabs switches routes',
      (tester) async {
        final router = GoRouter(
          initialLocation: '/rw/rw-002/keuangan',
          routes: [
            ShellRoute(
              builder: (context, state, child) =>
                  MainNavigationShell(child: child),
              routes: [
                GoRoute(
                  path: '/rw/:rwId/keuangan',
                  builder: (context, state) => const Text('Content Data RW'),
                ),
                GoRoute(
                  path: '/home/notifikasi',
                  builder: (context, state) => const Text('Content Notifikasi'),
                ),
                GoRoute(
                  path: '/home/profile',
                  builder: (context, state) => const Text('Content Profil'),
                ),
              ],
            ),
          ],
        );

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              authRepositoryProvider.overrideWith(
                (ref) => _MockAuthRepo(anggotaRwUser),
              ),
              notificationUnreadCountProvider.overrideWith(
                (ref) => _NotificationUnreadCountMock(0),
              ),
            ],
            child: MaterialApp.router(routerConfig: router),
          ),
        );
        await tester.pumpAndSettle();

        // 1. Initially on /rw/rw-002/keuangan -> Data RW tab is selected
        expect(find.text('Content Data RW'), findsOneWidget);
        final dataRwText = tester.widget<Text>(find.text('Data RW'));
        expect(dataRwText.style?.fontWeight, equals(FontWeight.w700));

        // 2. Tap Notifikasi tab
        await tester.tap(find.text('Notifikasi'));
        await tester.pumpAndSettle();

        expect(find.text('Content Notifikasi'), findsOneWidget);
        final notifText = tester.widget<Text>(find.text('Notifikasi'));
        expect(notifText.style?.fontWeight, equals(FontWeight.w700));

        // 3. Tap Profil tab
        await tester.tap(find.text('Profil'));
        await tester.pumpAndSettle();

        expect(find.text('Content Profil'), findsOneWidget);
        final profileText = tester.widget<Text>(find.text('Profil'));
        expect(profileText.style?.fontWeight, equals(FontWeight.w700));

        // 4. Tap Data RW tab to return to Data RW
        await tester.tap(find.text('Data RW'));
        await tester.pumpAndSettle();

        expect(find.text('Content Data RW'), findsOneWidget);
        final dataRwTextAgain = tester.widget<Text>(find.text('Data RW'));
        expect(dataRwTextAgain.style?.fontWeight, equals(FontWeight.w700));
      },
    );
  });
}
