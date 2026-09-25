import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'theme/app_theme.dart';
import 'navigation_shell.dart';
import 'package:social_finance/core/providers/theme_provider.dart';
import '../features/auth/data/mock_auth_repository.dart';
import '../features/auth/presentation/screens/login_screen.dart';
import '../features/cash/presentation/screens/cash_screen.dart';
import '../features/dues/presentation/screens/dues_screen.dart';
import '../features/dashboard/presentation/screens/dashboard_screen.dart';
import '../features/profile/presentation/screens/profile_screen.dart';
import '../features/reports/presentation/screens/reports_screen.dart';
import '../features/warga/presentation/screens/warga_screen.dart';

class App extends ConsumerStatefulWidget {
  const App({super.key});

  @override
  ConsumerState<App> createState() => _AppState();
}

class _RouterRefreshNotifier extends ChangeNotifier {
  bool _lastLoggedIn = false;

  void update(bool isLoggedIn) {
    if (_lastLoggedIn != isLoggedIn) {
      _lastLoggedIn = isLoggedIn;
      notifyListeners();
    }
  }
}

class _AppState extends ConsumerState<App> {
  late final _RouterRefreshNotifier _refreshNotifier;
  late final GoRouter _router;
  ProviderSubscription<AsyncValue<Map<String, dynamic>?>?>? _authSubscription;

  @override
  void initState() {
    super.initState();
    _refreshNotifier = _RouterRefreshNotifier();
    final initialAuth = ref.read(authRepositoryProvider);
    _refreshNotifier._lastLoggedIn = initialAuth?.valueOrNull != null;

    _authSubscription = ref.listenManual<AsyncValue<Map<String, dynamic>?>?>(
      authRepositoryProvider,
      (previous, next) {
        final isLoggedIn = next?.valueOrNull != null;
        _refreshNotifier.update(isLoggedIn);
      },
    );

    _router = GoRouter(
      initialLocation: _refreshNotifier._lastLoggedIn ? '/home' : '/login',
      debugLogDiagnostics: true,
      refreshListenable: _refreshNotifier,
      redirect: (context, state) {
        final authState = ref.read(authRepositoryProvider);
        final loggedIn = authState?.valueOrNull != null;
        if (loggedIn && state.matchedLocation == '/login') {
          return '/home';
        }
        if (!loggedIn && state.matchedLocation != '/login') {
          return '/login';
        }
        return null;
      },
      routes: [
        GoRoute(
          path: '/login',
          name: 'login',
          builder: (context, state) => const LoginScreen(),
        ),
        ShellRoute(
          builder: (context, state, child) {
            return MainNavigationShell(child: child);
          },
          routes: [
            GoRoute(
              path: '/home',
              name: 'home',
              builder: (context, state) => const DashboardScreen(),
              routes: [
                GoRoute(
                  path: 'cash',
                  name: 'cash',
                  builder: (context, state) => const CashScreen(),
                ),
                GoRoute(
                  path: 'dues',
                  name: 'dues',
                  builder: (context, state) => const DuesScreen(),
                ),
                GoRoute(
                  path: 'warga',
                  name: 'warga',
                  builder: (context, state) => const WargaScreen(),
                ),
                GoRoute(
                  path: 'reports',
                  name: 'reports',
                  builder: (context, state) => const ReportsScreen(),
                ),
                GoRoute(
                  path: 'profile',
                  name: 'profile',
                  builder: (context, state) => const ProfileScreen(),
                ),
              ],
            ),
          ],
        ),
      ],
    );
  }

  @override
  void dispose() {
    _authSubscription?.close();
    _router.dispose();
    _refreshNotifier.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final themeMode = ref.watch(themeProvider);

    return MaterialApp.router(
      title: 'Social Finance',
      debugShowCheckedModeBanner: false,
      theme: AppTheme.lightTheme,
      darkTheme: AppTheme.darkTheme,
      themeMode: themeMode.themeMode,
      routerConfig: _router,
    );
  }
}
