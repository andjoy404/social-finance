/// Regression tests for swipe-to-change-menu gesture removal.
///
/// Ensures horizontal swipe NEVER changes the active main tab, regardless of
/// whether the user is on a root tab (BERANDA, KAS, IURAN, WARGA, PROFIL) or
/// a child page/form/detail/dialog. Main-menu switching is tap-based only.
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart' as rp;
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:social_finance/app/navigation_shell.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/warga/data/api_warga_models.dart';
import 'package:social_finance/features/warga/data/warga_providers.dart';

// ─────────────────────────── helpers ────────────────────────────────────────

AuthRepository _createTestAuthRepo([Map<String, dynamic>? initialData]) {
  final client = ApiClient(baseUrl: 'http://test.local');
  final repo = AuthRepository(
    authService: AuthService(client),
    apiClient: client,
  );
  if (initialData != null) {
    repo.state = rp.AsyncData<Map<String, dynamic>?>(initialData);
  }
  return repo;
}

class _MockWargaNotifier extends WargaListNotifier {
  @override
  Future<List<MappedResident>> build() async {
    state = rp.AsyncData(const <MappedResident>[]);
    return const [];
  }
}

/// Shared auth overrides for all swipe-shell apps.
List<rp.Override> _swipeOverrides() => [
      authRepositoryProvider.overrideWith(
        (_) => _createTestAuthRepo({
          'name': 'Heri Prastyo',
          'role': AppRole.bendahara,
          'rt': '002',
          'rw': '016',
        }),
      ),
      wargaListProvider.overrideWith(() => _MockWargaNotifier()),
    ];

/// Builds a ShellRoute-wrapped app with MainNavigationShell.
GoRouter _buildRouter({
  required String initialLocation,
  required List<GoRoute> tabRoutes,
  List<GoRoute> extraRoutes = const [],
}) {
  return GoRouter(
    initialLocation: initialLocation,
    routes: [
      ShellRoute(
        builder: (context, state, child) =>
            MainNavigationShell(child: child),
        routes: tabRoutes,
      ),
      ...extraRoutes,
    ],
  );
}

GoRoute _tabRoute({
  required String path,
  required RouteBuilder builder,
  List<RouteBase>? childRoutes,
}) {
  return GoRoute(
    path: path,
    builder: builder,
    routes: childRoutes,
  );
}

void main() {
  // ── 1. Root tabs — swipe does NOT change tab ────────────────────────────

  testWidgets(
    'Swipe left on BERANDA does not change the active tab',
    (tester) async {
      await tester.pumpWidget(
        rp.ProviderScope(
          overrides: _swipeOverrides(),
          child: MaterialApp.router(
            routerConfig: _buildRouter(
              initialLocation: '/home',
              tabRoutes: [
                _tabRoute(
                  path: '/home',
                  builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Beranda')),
                ),
                _tabRoute(
                  path: '/home/kas',
                  builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Kas')),
                ),
                _tabRoute(
                  path: '/home/profile',
                  builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Profil')),
                ),
              ],
            ),
          ),
        ),
      );
      await tester.pump();

      expect(find.text('Beranda'), findsOneWidget);
      expect(find.text('Kas'), findsNothing);

      await tester.fling(
        find.byType(SafeArea),
        const Offset(-300, 0),
        1000,
        warnIfMissed: false,
      );
      await tester.pump(const Duration(milliseconds: 200));
      await tester.pump();

      expect(find.text('Beranda'), findsOneWidget);
      expect(find.text('Kas'), findsNothing);
    },
  );

  testWidgets(
    'Swipe right on BERANDA does not change the active tab',
    (tester) async {
      await tester.pumpWidget(
        rp.ProviderScope(
          overrides: _swipeOverrides(),
          child: MaterialApp.router(
            routerConfig: _buildRouter(
              initialLocation: '/home',
              tabRoutes: [
                _tabRoute(
                  path: '/home',
                  builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Beranda')),
                ),
                _tabRoute(
                  path: '/home/kas',
                  builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Kas')),
                ),
                _tabRoute(
                  path: '/home/profile',
                  builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Profil')),
                ),
              ],
            ),
          ),
        ),
      );
      await tester.pump();

      expect(find.text('Beranda'), findsOneWidget);

      await tester.fling(
        find.byType(SafeArea),
        const Offset(300, 0),
        1000,
        warnIfMissed: false,
      );
      await tester.pump(const Duration(milliseconds: 200));
      await tester.pump();

      expect(find.text('Beranda'), findsOneWidget);
    },
  );

  testWidgets(
    'Swipe left/right on KAS does not change the active tab',
    (tester) async {
      await tester.pumpWidget(
        rp.ProviderScope(
          overrides: _swipeOverrides(),
          child: MaterialApp.router(
            routerConfig: _buildRouter(
              initialLocation: '/home/kas',
              tabRoutes: [
                _tabRoute(
                  path: '/home',
                  builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Beranda')),
                ),
                _tabRoute(
                  path: '/home/kas',
                  builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Kas')),
                ),
                _tabRoute(
                  path: '/home/profile',
                  builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Profil')),
                ),
              ],
            ),
          ),
        ),
      );
      await tester.pump();

      expect(find.text('Kas'), findsOneWidget);

      await tester.fling(
        find.byType(SafeArea),
        const Offset(-300, 0),
        1000,
        warnIfMissed: false,
      );
      await tester.pump(const Duration(milliseconds: 200));
      await tester.pump();
      expect(find.text('Kas'), findsOneWidget);
      expect(find.text('Beranda'), findsNothing);

      await tester.fling(
        find.byType(SafeArea),
        const Offset(300, 0),
        1000,
        warnIfMissed: false,
      );
      await tester.pump(const Duration(milliseconds: 200));
      await tester.pump();
      expect(find.text('Kas'), findsOneWidget);
    },
  );

  testWidgets(
    'Swipe left/right on IURAN does not change the active tab',
    (tester) async {
      await tester.pumpWidget(
        rp.ProviderScope(
          overrides: _swipeOverrides(),
          child: MaterialApp.router(
            routerConfig: _buildRouter(
              initialLocation: '/home/iuran',
              tabRoutes: [
                _tabRoute(
                  path: '/home',
                  builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Beranda')),
                ),
                _tabRoute(
                  path: '/home/iuran',
                  builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Iuran')),
                ),
                _tabRoute(
                  path: '/home/profile',
                  builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Profil')),
                ),
              ],
            ),
          ),
        ),
      );
      await tester.pump();

      expect(find.text('Iuran'), findsOneWidget);

      await tester.fling(
        find.byType(SafeArea),
        const Offset(-300, 0),
        1000,
        warnIfMissed: false,
      );
      await tester.pump(const Duration(milliseconds: 200));
      await tester.pump();
      expect(find.text('Iuran'), findsOneWidget);

      await tester.fling(
        find.byType(SafeArea),
        const Offset(300, 0),
        1000,
        warnIfMissed: false,
      );
      await tester.pump(const Duration(milliseconds: 200));
      await tester.pump();
      expect(find.text('Iuran'), findsOneWidget);
    },
  );

  testWidgets(
    'Swipe left/right on WARGA does not change the active tab',
    (tester) async {
      await tester.pumpWidget(
        rp.ProviderScope(
          overrides: _swipeOverrides(),
          child: MaterialApp.router(
            routerConfig: _buildRouter(
              initialLocation: '/home/warga',
              tabRoutes: [
                _tabRoute(
                  path: '/home',
                  builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Beranda')),
                ),
                _tabRoute(
                  path: '/home/warga',
                  builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Warga')),
                ),
                _tabRoute(
                  path: '/home/profile',
                  builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Profil')),
                ),
              ],
            ),
          ),
        ),
      );
      await tester.pump();

      expect(find.text('Warga'), findsOneWidget);

      await tester.fling(
        find.byType(SafeArea),
        const Offset(-300, 0),
        1000,
        warnIfMissed: false,
      );
      await tester.pump(const Duration(milliseconds: 200));
      await tester.pump();
      expect(find.text('Warga'), findsOneWidget);

      await tester.fling(
        find.byType(SafeArea),
        const Offset(300, 0),
        1000,
        warnIfMissed: false,
      );
      await tester.pump(const Duration(milliseconds: 200));
      await tester.pump();
      expect(find.text('Warga'), findsOneWidget);
    },
  );

  testWidgets(
    'Swipe left/right on PROFIL does not change the active tab',
    (tester) async {
      await tester.pumpWidget(
        rp.ProviderScope(
          overrides: _swipeOverrides(),
          child: MaterialApp.router(
            routerConfig: _buildRouter(
              initialLocation: '/home/profile',
              tabRoutes: [
                _tabRoute(
                  path: '/home',
                  builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Beranda')),
                ),
                _tabRoute(
                  path: '/home/profile',
                  builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Profil')),
                ),
              ],
            ),
          ),
        ),
      );
      await tester.pump();

      expect(find.text('Profil'), findsOneWidget);

      await tester.fling(
        find.byType(SafeArea),
        const Offset(-300, 0),
        1000,
        warnIfMissed: false,
      );
      await tester.pump(const Duration(milliseconds: 200));
      await tester.pump();
      expect(find.text('Profil'), findsOneWidget);

      await tester.fling(
        find.byType(SafeArea),
        const Offset(300, 0),
        1000,
        warnIfMissed: false,
      );
      await tester.pump(const Duration(milliseconds: 200));
      await tester.pump();
      expect(find.text('Profil'), findsOneWidget);
    },
  );

  // ── 2. Child pages — swipe does NOT change tab ─────────────────────────

  testWidgets(
    'Swipe on Warga edit form does not change tab',
    (tester) async {
      await tester.pumpWidget(
        rp.ProviderScope(
          overrides: _swipeOverrides(),
          child: MaterialApp.router(
            routerConfig: _buildRouter(
              initialLocation: '/home/warga/baru',
              tabRoutes: [
                _tabRoute(
                  path: '/home',
                  builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Beranda')),
                ),
                _tabRoute(
                  path: '/home/warga',
                  builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Warga')),
                  childRoutes: [
                    GoRoute(
                      path: 'baru',
                      builder: (BuildContext c, GoRouterState s) =>
                          const Scaffold(body: Text('Tambah Warga')),
                    ),
                  ],
                ),
                _tabRoute(
                  path: '/home/profile',
                  builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Profil')),
                ),
              ],
            ),
          ),
        ),
      );
      await tester.pump();

      expect(find.text('Tambah Warga'), findsOneWidget);

      await tester.fling(
        find.byType(SafeArea),
        const Offset(-300, 0),
        1000,
        warnIfMissed: false,
      );
      await tester.pump(const Duration(milliseconds: 200));
      await tester.pump();

      expect(find.text('Tambah Warga'), findsOneWidget);
      expect(find.text('Beranda'), findsNothing);
    },
  );

  testWidgets(
    'Swipe on Kas detail page does not change tab',
    (tester) async {
      await tester.pumpWidget(
        rp.ProviderScope(
          overrides: _swipeOverrides(),
          child: MaterialApp.router(
            routerConfig: _buildRouter(
              initialLocation: '/home/kas/abc123',
              tabRoutes: [
                _tabRoute(
                  path: '/home',
                  builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Beranda')),
                ),
                _tabRoute(
                  path: '/home/kas',
                  builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Kas')),
                  childRoutes: [
                    GoRoute(
                      path: ':id',
                      builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Detail Kas')),
                    ),
                  ],
                ),
                _tabRoute(
                  path: '/home/profile',
                  builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Profil')),
                ),
              ],
            ),
          ),
        ),
      );
      await tester.pump();

      expect(find.text('Detail Kas'), findsOneWidget);

      await tester.fling(
        find.byType(SafeArea),
        const Offset(-300, 0),
        1000,
        warnIfMissed: false,
      );
      await tester.pump(const Duration(milliseconds: 200));
      await tester.pump();

      expect(find.text('Detail Kas'), findsOneWidget);
      expect(find.text('Beranda'), findsNothing);
    },
  );

  testWidgets(
    'Swipe on Iuran detail page does not change tab',
    (tester) async {
      await tester.pumpWidget(
        rp.ProviderScope(
          overrides: _swipeOverrides(),
          child: MaterialApp.router(
            routerConfig: _buildRouter(
              initialLocation: '/home/iuran/detail/bill1',
              tabRoutes: [
                _tabRoute(
                  path: '/home',
                  builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Beranda')),
                ),
                _tabRoute(
                  path: '/home/iuran',
                  builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Iuran')),
                  childRoutes: [
                    GoRoute(
                      path: 'detail/:billId',
                      builder: (BuildContext c, GoRouterState s) =>
                          const Scaffold(body: Text('Detail Tagihan')),
                    ),
                  ],
                ),
                _tabRoute(
                  path: '/home/profile',
                  builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Profil')),
                ),
              ],
            ),
          ),
        ),
      );
      await tester.pump();

      expect(find.text('Detail Tagihan'), findsOneWidget);

      await tester.fling(
        find.byType(SafeArea),
        const Offset(-300, 0),
        1000,
        warnIfMissed: false,
      );
      await tester.pump(const Duration(milliseconds: 200));
      await tester.pump();

      expect(find.text('Detail Tagihan'), findsOneWidget);
      expect(find.text('Beranda'), findsNothing);
    },
  );

  // ── 3. Dialog open — swipe does NOT change tab behind it ───────────────

  testWidgets(
    'Swipe does not change tab when a dialog is open',
    (tester) async {
      await tester.pumpWidget(
        rp.ProviderScope(
          overrides: _swipeOverrides(),
          child: MaterialApp.router(
            routerConfig: _buildRouter(
              initialLocation: '/home',
              tabRoutes: [
                _tabRoute(
                  path: '/home',
                  builder: (BuildContext c, GoRouterState s) => Builder(
                    builder: (ctx) {
                      Future.microtask(() {
                        showDialog<void>(
                          context: ctx,
                          builder: (_) => AlertDialog(
                            title: const Text('Modal'),
                            content: const Text('Dialog content'),
                            actions: [
                              TextButton(
                                onPressed: () => Navigator.of(ctx).pop(),
                                child: const Text('OK'),
                              ),
                            ],
                          ),
                        );
                      });
                      return const Scaffold(body: Text('Beranda'));
                    },
                  ),
                ),
                _tabRoute(
                  path: '/home/kas',
                  builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Kas')),
                ),
                _tabRoute(
                  path: '/home/profile',
                  builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Profil')),
                ),
              ],
            ),
          ),
        ),
      );
      tester.binding.scheduleWarmUpFrame();
      await tester.pump();
      await tester.pump();

      expect(find.text('Modal'), findsOneWidget);
      expect(find.text('Beranda'), findsOneWidget);

      await tester.fling(
        find.byType(SafeArea),
        const Offset(-300, 0),
        1000,
        warnIfMissed: false,
      );
      await tester.pump(const Duration(milliseconds: 200));
      await tester.pump();

      expect(find.text('Beranda'), findsOneWidget);
      expect(find.text('Kas'), findsNothing);
    },
  );

  // ── 4. Vertical scroll still works ─────────────────────────────────────

  testWidgets(
    'Vertical scroll is not affected by swipe removal',
    (tester) async {
      await tester.pumpWidget(
        rp.ProviderScope(
          overrides: _swipeOverrides(),
          child: MaterialApp(
            home: Scaffold(
              body: SingleChildScrollView(
                child: Column(
                  children: List.generate(
                    50,
                    (i) => Container(
                      height: 60,
                      padding: const EdgeInsets.all(8),
                      child: Text('Item $i'),
                    ),
                  ),
                ),
              ),
            ),
          ),
        ),
      );
      await tester.pump();

      await tester.fling(
        find.byType(SingleChildScrollView),
        const Offset(0, -400),
        1000,
      );
      await tester.pump(const Duration(milliseconds: 200));
      await tester.pump();

      expect(find.text('Item 49'), findsOneWidget);
    },
  );

  // ── 5. Menu tap navigation still works ─────────────────────────────────

  testWidgets(
    'Tab tap navigation still switches main tab',
    (tester) async {
      await tester.pumpWidget(
        rp.ProviderScope(
          overrides: _swipeOverrides(),
          child: MaterialApp.router(
            routerConfig: _buildRouter(
              initialLocation: '/home',
              tabRoutes: [
                _tabRoute(
                  path: '/home',
                  builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Beranda')),
                ),
                _tabRoute(
                  path: '/home/kas',
                  builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Kas')),
                ),
                _tabRoute(
                  path: '/home/iuran',
                  builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Iuran')),
                ),
                _tabRoute(
                  path: '/home/warga',
                  builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Warga')),
                ),
                _tabRoute(
                  path: '/home/profile',
                  builder: (BuildContext c, GoRouterState s) => const Scaffold(body: Text('Profil')),
                ),
              ],
            ),
          ),
        ),
      );
      await tester.pump();

      expect(find.text('Beranda'), findsOneWidget);

      // Tap Kas tab item
      await tester.tap(find.byKey(const Key('nav-item-kas')));
      await tester.pump();

      expect(find.text('Kas'), findsOneWidget);
      expect(find.text('Beranda'), findsNothing);

      // Tap Iuran tab item
      await tester.tap(find.byKey(const Key('nav-item-iuran')));
      await tester.pump();

      expect(find.text('Iuran'), findsOneWidget);
      expect(find.text('Kas'), findsNothing);
    },
  );
}
