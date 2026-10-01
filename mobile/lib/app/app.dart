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
import '../features/warga/data/warga_providers.dart';
import '../features/warga/presentation/screens/warga_screen.dart';
import '../features/warga/presentation/screens/household_create_screen.dart';
import '../features/warga/presentation/screens/household_edit_screen.dart';
import '../features/warga/presentation/screens/special_resident_create_screen.dart';
import '../features/warga/presentation/screens/special_resident_edit_screen.dart';
import '../features/iuran/presentation/screens/iuran_screen.dart';
import '../features/iuran/presentation/screens/iuran_detail_screen.dart';
import '../features/iuran/presentation/screens/iuran_payment_screen.dart';
import '../features/iuran/presentation/screens/iuran_arrears_screen.dart';
import '../features/iuran/presentation/screens/iuran_report_screen.dart';
import '../features/kas/presentation/screens/kas_screen.dart';
import '../features/kas/presentation/screens/kas_detail_screen.dart';
import '../features/kas/presentation/screens/kas_masuk_screen.dart';
import '../features/kas/presentation/screens/kas_keluar_screen.dart';
import '../features/kas/presentation/screens/kas_report_screen.dart';

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
  late final WidgetRef _ref;

  @override
  void initState() {
    super.initState();
    _ref = ref;
    _refreshNotifier = _RouterRefreshNotifier();
    final initialAuth = ref.read(authRepositoryProvider);
    _refreshNotifier._lastLoggedIn = initialAuth?.valueOrNull != null;

    _authSubscription = ref.listenManual<AsyncValue<Map<String, dynamic>?>?>(
      authRepositoryProvider,
      (previous, next) {
        final isLoggedIn = next?.valueOrNull != null;
        if (isLoggedIn != _refreshNotifier._lastLoggedIn) {
          if (!isLoggedIn) {
            _ref.read(wargaSearchQueryProvider.notifier).state = '';
          }
          _refreshNotifier.update(isLoggedIn);
        }
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
                  path: 'iuran',
                  name: 'iuran',
                  builder: (context, state) => const IuranScreen(),
                  routes: [
                    GoRoute(
                      path: 'detail/:billId',
                      name: 'iuran_detail',
                      builder: (context, state) {
                        final billId = state.pathParameters['billId']!;
                        return IuranDetailScreen(billId: billId);
                      },
                    ),
                    GoRoute(
                      path: 'pembayaran/:billId',
                      name: 'iuran_payment',
                      builder: (context, state) {
                        final billId = state.pathParameters['billId']!;
                        return IuranPaymentScreen(billId: billId);
                      },
                    ),
                    GoRoute(
                      path: 'tunggakan',
                      name: 'iuran_arrears',
                      builder: (context, state) => const IuranArrearsScreen(),
                    ),
                    GoRoute(
                      path: 'laporan',
                      name: 'iuran_report',
                      builder: (context, state) => const IuranReportScreen(),
                    ),
                    GoRoute(
                      path: 'form',
                      name: 'iuran_form',
                      builder: (context, state) => Scaffold(
                        appBar: AppBar(
                          title: const Text('Buat Tagihan'),
                          leading: IconButton(
                            icon: const Icon(Icons.arrow_back),
                            onPressed: () => context.pop(),
                          ),
                        ),
                        body: const Center(
                          child: Text(
                            'Halaman pembuatan tagihan akan segera tersedia.',
                            textAlign: TextAlign.center,
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
                GoRoute(
                  path: 'kas',
                  name: 'kas',
                  builder: (context, state) => const KasScreen(),
                  routes: [
                    GoRoute(
                      path: 'masuk',
                      name: 'kas_masuk',
                      builder: (context, state) => const KasMasukScreen(),
                    ),
                    GoRoute(
                      path: 'keluar',
                      name: 'kas_keluar',
                      builder: (context, state) => const KasKeluarScreen(),
                    ),
                    GoRoute(
                      path: ':id',
                      name: 'kas_detail',
                      builder: (context, state) {
                        final txId = state.pathParameters['id']!;
                        return KasDetailScreen(
                          transaction: KasTransaction(
                            id: txId,
                            tanggal: DateTime.now(),
                            jenis: KasJenis.masuk,
                            kategori: KasCategory.iuranWarga,
                            keterangan: 'Detail Transaksi',
                            nominal: 0,
                            saldo: 0,
                          ),
                        );
                      },
                    ),
                    GoRoute(
                      path: 'laporan',
                      name: 'kas_report',
                      builder: (context, state) => const KasReportScreen(),
                    ),
                  ],
                ),
                GoRoute(
                  path: 'warga',
                  name: 'warga',
                  builder: (context, state) => const WargaScreen(),
                  routes: [
                    GoRoute(
                      path: 'baru',
                      name: 'warga_baru',
                      builder: (context, state) {
                        final authState = ref.read(authRepositoryProvider);
                        final systemRole = authState?.valueOrNull?['system_role'] as String?;
                        return HouseholdCreateScreen(isSuperAdmin: systemRole == 'super_admin');
                      },
                    ),
                    GoRoute(
                      path: 'edit/household/:householdId',
                      name: 'warga_edit',
                      builder: (context, state) {
                        final householdId = state.pathParameters['householdId']!;
                        return HouseholdEditScreen(householdId: householdId);
                      },
                    ),
                    GoRoute(
                      path: 'baru/petugas',
                      name: 'warga_petugas_baru',
                      builder: (context, state) {
                        final authState = ref.read(authRepositoryProvider);
                        final systemRole = authState?.valueOrNull?['system_role'] as String?;
                        return SpecialResidentCreateScreen(isSuperAdmin: systemRole == 'super_admin');
                      },
                    ),
                    GoRoute(
                      path: 'edit/petugas/:residentId',
                      name: 'warga_resident_edit',
                      builder: (context, state) {
                        final residentId = state.pathParameters['residentId']!;
                        return SpecialResidentEditScreen(residentId: residentId);
                      },
                    ),
                  ],
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
