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
  required GoRouterWidgetBuilder builder,
  List<RouteBase> childRoutes = const <RouteBase>[],
}) {
  return GoRoute(
    path: path,
    builder: builder,
    routes: childRoutes,
  );
}

/// Wraps page content in a Container with a key so assertions can scope
/// to the page body (excluding the bottom nav bar labels).
Widget _scoped(Widget child, String path) {
  return Container(
    key: Key('page-$path'),
    child: child,
  );
}

/// Finds page-content widgets by delegating to a scoped container.
Finder scopedFinder(String path, Finder matcher) {
  return find.descendant(
    of: find.byKey(Key('page-$path')),
    matching: matcher,
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
                  builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Beranda')), '/home'),
                ),
                _tabRoute(
                  path: '/home/kas',
                  builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Kas')), '/home/kas'),
                ),
                _tabRoute(
                  path: '/home/profile',
                  builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Profil')), '/home/profile'),
                ),
              ],
            ),
          ),
        ),
      );
      await tester.pump();

      expect(scopedFinder('/home', find.text('Beranda')), findsOneWidget);
      expect(scopedFinder('/home', find.text('Kas')), findsNothing);

      await tester.fling(
        find.byKey(const Key('page-/home')),
        const Offset(-300, 0),
        1000,
        warnIfMissed: false,
      );
      await tester.pump(const Duration(milliseconds: 200));
      await tester.pump();

      expect(scopedFinder('/home', find.text('Beranda')), findsOneWidget);
      expect(scopedFinder('/home', find.text('Kas')), findsNothing);
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
                  builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Beranda')), '/home'),
                ),
                _tabRoute(
                  path: '/home/kas',
                  builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Kas')), '/home/kas'),
                ),
                _tabRoute(
                  path: '/home/profile',
                  builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Profil')), '/home/profile'),
                ),
              ],
            ),
          ),
        ),
      );
      await tester.pump();

      expect(scopedFinder('/home', find.text('Beranda')), findsOneWidget);

      await tester.fling(
        find.byKey(const Key('page-/home')),
        const Offset(300, 0),
        1000,
        warnIfMissed: false,
      );
      await tester.pump(const Duration(milliseconds: 200));
      await tester.pump();

      expect(scopedFinder('/home', find.text('Beranda')), findsOneWidget);
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
                  builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Beranda')), '/home'),
                ),
                _tabRoute(
                  path: '/home/kas',
                  builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Kas')), '/home/kas'),
                ),
                _tabRoute(
                  path: '/home/profile',
                  builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Profil')), '/home/profile'),
                ),
              ],
            ),
          ),
        ),
      );
      await tester.pump();

      expect(scopedFinder('/home/kas', find.text('Kas')), findsOneWidget);

      await tester.fling(
        find.byKey(const Key('page-/home/kas')),
        const Offset(-300, 0),
        1000,
        warnIfMissed: false,
      );
      await tester.pump(const Duration(milliseconds: 200));
      await tester.pump();
      expect(scopedFinder('/home/kas', find.text('Kas')), findsOneWidget);
      expect(scopedFinder('/home/kas', find.text('Beranda')), findsNothing);

      await tester.fling(
        find.byKey(const Key('page-/home/kas')),
        const Offset(300, 0),
        1000,
        warnIfMissed: false,
      );
      await tester.pump(const Duration(milliseconds: 200));
      await tester.pump();
      expect(scopedFinder('/home/kas', find.text('Kas')), findsOneWidget);
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
                  builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Beranda')), '/home'),
                ),
                _tabRoute(
                  path: '/home/iuran',
                  builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Iuran')), '/home/iuran'),
                ),
                _tabRoute(
                  path: '/home/profile',
                  builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Profil')), '/home/profile'),
                ),
              ],
            ),
          ),
        ),
      );
      await tester.pump();

      expect(scopedFinder('/home/iuran', find.text('Iuran')), findsOneWidget);

      await tester.fling(
        find.byKey(const Key('page-/home/iuran')),
        const Offset(-300, 0),
        1000,
        warnIfMissed: false,
      );
      await tester.pump(const Duration(milliseconds: 200));
      await tester.pump();
      expect(scopedFinder('/home/iuran', find.text('Iuran')), findsOneWidget);

      await tester.fling(
        find.byKey(const Key('page-/home/iuran')),
        const Offset(300, 0),
        1000,
        warnIfMissed: false,
      );
      await tester.pump(const Duration(milliseconds: 200));
      await tester.pump();
      expect(scopedFinder('/home/iuran', find.text('Iuran')), findsOneWidget);
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
                  builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Beranda')), '/home'),
                ),
                _tabRoute(
                  path: '/home/warga',
                  builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Warga')), '/home/warga'),
                ),
                _tabRoute(
                  path: '/home/profile',
                  builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Profil')), '/home/profile'),
                ),
              ],
            ),
          ),
        ),
      );
      await tester.pump();

      expect(scopedFinder('/home/warga', find.text('Warga')), findsOneWidget);

      await tester.fling(
        find.byKey(const Key('page-/home/warga')),
        const Offset(-300, 0),
        1000,
        warnIfMissed: false,
      );
      await tester.pump(const Duration(milliseconds: 200));
      await tester.pump();
      expect(scopedFinder('/home/warga', find.text('Warga')), findsOneWidget);

      await tester.fling(
        find.byKey(const Key('page-/home/warga')),
        const Offset(300, 0),
        1000,
        warnIfMissed: false,
      );
      await tester.pump(const Duration(milliseconds: 200));
      await tester.pump();
      expect(scopedFinder('/home/warga', find.text('Warga')), findsOneWidget);
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
                  builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Beranda')), '/home'),
                ),
                _tabRoute(
                  path: '/home/profile',
                  builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Profil')), '/home/profile'),
                ),
              ],
            ),
          ),
        ),
      );
      await tester.pump();

      expect(scopedFinder('/home/profile', find.text('Profil')), findsOneWidget);

      await tester.fling(
        find.byKey(const Key('page-/home/profile')),
        const Offset(-300, 0),
        1000,
        warnIfMissed: false,
      );
      await tester.pump(const Duration(milliseconds: 200));
      await tester.pump();
      expect(scopedFinder('/home/profile', find.text('Profil')), findsOneWidget);

      await tester.fling(
        find.byKey(const Key('page-/home/profile')),
        const Offset(300, 0),
        1000,
        warnIfMissed: false,
      );
      await tester.pump(const Duration(milliseconds: 200));
      await tester.pump();
      expect(scopedFinder('/home/profile', find.text('Profil')), findsOneWidget);
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
                  builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Beranda')), '/home'),
                ),
                _tabRoute(
                  path: '/home/warga',
                  builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Warga')), '/home/warga'),
                  childRoutes: [
                    GoRoute(
                      path: 'baru',
                      builder: (BuildContext c, GoRouterState s) =>
                          _scoped(const Scaffold(body: Text('Tambah Warga')), '/home/warga/baru'),
                    ),
                  ],
                ),
                _tabRoute(
                  path: '/home/profile',
                  builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Profil')), '/home/profile'),
                ),
              ],
            ),
          ),
        ),
      );
      await tester.pump();

      expect(scopedFinder('/home/warga/baru', find.text('Tambah Warga')), findsOneWidget);

      await tester.fling(
        find.byKey(const Key('page-/home/warga/baru')),
        const Offset(-300, 0),
        1000,
        warnIfMissed: false,
      );
      await tester.pump(const Duration(milliseconds: 200));
      await tester.pump();

      expect(scopedFinder('/home/warga/baru', find.text('Tambah Warga')), findsOneWidget);
      expect(scopedFinder('/home/warga/baru', find.text('Beranda')), findsNothing);
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
                  builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Beranda')), '/home'),
                ),
                _tabRoute(
                  path: '/home/kas',
                  builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Kas')), '/home/kas'),
                  childRoutes: [
                    GoRoute(
                      path: ':id',
                      builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Detail Kas')), '/home/kas/abc123'),
                    ),
                  ],
                ),
                _tabRoute(
                  path: '/home/profile',
                  builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Profil')), '/home/profile'),
                ),
              ],
            ),
          ),
        ),
      );
      await tester.pump();

      expect(scopedFinder('/home/kas/abc123', find.text('Detail Kas')), findsOneWidget);

      await tester.fling(
        find.byKey(const Key('page-/home/kas/abc123')),
        const Offset(-300, 0),
        1000,
        warnIfMissed: false,
      );
      await tester.pump(const Duration(milliseconds: 200));
      await tester.pump();

      expect(scopedFinder('/home/kas/abc123', find.text('Detail Kas')), findsOneWidget);
      expect(scopedFinder('/home/kas/abc123', find.text('Beranda')), findsNothing);
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
                  builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Beranda')), '/home'),
                ),
                _tabRoute(
                  path: '/home/iuran',
                  builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Iuran')), '/home/iuran'),
                  childRoutes: [
                    GoRoute(
                      path: 'detail/:billId',
                      builder: (BuildContext c, GoRouterState s) =>
                          _scoped(const Scaffold(body: Text('Detail Tagihan')), '/home/iuran/detail/bill1'),
                    ),
                  ],
                ),
                _tabRoute(
                  path: '/home/profile',
                  builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Profil')), '/home/profile'),
                ),
              ],
            ),
          ),
        ),
      );
      await tester.pump();

      expect(scopedFinder('/home/iuran/detail/bill1', find.text('Detail Tagihan')), findsOneWidget);

      await tester.fling(
        find.byKey(const Key('page-/home/iuran/detail/bill1')),
        const Offset(-300, 0),
        1000,
        warnIfMissed: false,
      );
      await tester.pump(const Duration(milliseconds: 200));
      await tester.pump();

      expect(scopedFinder('/home/iuran/detail/bill1', find.text('Detail Tagihan')), findsOneWidget);
      expect(scopedFinder('/home/iuran/detail/bill1', find.text('Beranda')), findsNothing);
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
                      return _scoped(const Scaffold(body: Text('Beranda')), '/home');
                    },
                  ),
                ),
                _tabRoute(
                  path: '/home/kas',
                  builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Kas')), '/home/kas'),
                ),
                _tabRoute(
                  path: '/home/profile',
                  builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Profil')), '/home/profile'),
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
      expect(scopedFinder('/home', find.text('Beranda')), findsOneWidget);

      await tester.fling(
        find.byKey(const Key('page-/home')),
        const Offset(-300, 0),
        1000,
        warnIfMissed: false,
      );
      await tester.pump(const Duration(milliseconds: 200));
      await tester.pump();

      expect(scopedFinder('/home', find.text('Beranda')), findsOneWidget);
      expect(scopedFinder('/home', find.text('Kas')), findsNothing);
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
                  builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Beranda')), '/home'),
                ),
                _tabRoute(
                  path: '/home/kas',
                  builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Kas')), '/home/kas'),
                ),
                _tabRoute(
                  path: '/home/iuran',
                  builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Iuran')), '/home/iuran'),
                ),
                _tabRoute(
                  path: '/home/warga',
                  builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Warga')), '/home/warga'),
                ),
                _tabRoute(
                  path: '/home/profile',
                  builder: (BuildContext c, GoRouterState s) => _scoped(const Scaffold(body: Text('Profil')), '/home/profile'),
                ),
              ],
            ),
          ),
        ),
      );
      await tester.pump();

      expect(scopedFinder('/home', find.text('Beranda')), findsOneWidget);

      // Tap Kas tab item
      await tester.tap(find.byKey(const Key('nav-item-kas')));
      await tester.pumpAndSettle();

      // Scoping to SafeArea (route content area) to exclude BottomAppBar nav labels
      final Finder contentArea = find.byType(SafeArea).first;
      expect(
        find.descendant(of: contentArea, matching: find.text('Kas')),
        findsOneWidget,
      );
      expect(
        find.descendant(of: contentArea, matching: find.text('Beranda')),
        findsNothing,
      );

      // Tap Iuran tab item
      await tester.tap(find.byKey(const Key('nav-item-iuran')));
      await tester.pumpAndSettle();

      expect(
        find.descendant(of: find.byType(SafeArea).first, matching: find.text('Iuran')),
        findsOneWidget,
      );
      expect(
        find.descendant(of: find.byType(SafeArea).first, matching: find.text('Kas')),
        findsNothing,
      );
    },
  );
}
