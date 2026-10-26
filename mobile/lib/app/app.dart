import 'dart:async';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/models/role.dart';

import 'theme/app_theme.dart';
import 'navigation_shell.dart';
import '../core/theme/app_colors.dart';
import 'package:social_finance/core/providers/theme_provider.dart';
import '../core/notification/push_notification_service.dart';
import '../features/auth/data/mock_auth_repository.dart';
import '../features/auth/presentation/screens/login_screen.dart';
import '../features/cash/presentation/screens/cash_screen.dart';
import '../features/dues/presentation/screens/dues_screen.dart';
import '../features/dashboard/presentation/screens/dashboard_screen.dart';
import '../features/profile/presentation/screens/profile_screen.dart';
import '../features/reports/presentation/screens/reports_screen.dart';
import '../features/warga/data/warga_providers.dart';
import 'package:social_finance/features/warga/presentation/screens/warga_screen.dart';
import 'package:social_finance/features/rw/presentation/screens/rw_list_screen.dart';
import '../features/warga/presentation/screens/household_create_screen.dart';
import '../features/warga/presentation/screens/household_edit_screen.dart';
import '../features/warga/presentation/screens/special_resident_create_screen.dart';
import '../features/warga/presentation/screens/special_resident_edit_screen.dart';
import '../features/iuran/presentation/screens/iuran_screen.dart';
import 'package:social_finance/features/iuran/presentation/screens/iuran_form_screen.dart';

import '../features/iuran/presentation/screens/iuran_detail_screen.dart';
import '../features/iuran/presentation/screens/iuran_payment_screen.dart';
import '../features/iuran/presentation/screens/iuran_arrears_screen.dart';
import '../features/iuran/presentation/screens/iuran_report_screen.dart';
import '../features/iuran/presentation/screens/iuran_management_screen.dart';
import '../features/iuran/data/iuran_access.dart';
import '../features/kas/presentation/screens/kas_screen.dart';
import '../features/kas/presentation/screens/kas_detail_screen.dart';
import '../features/kas/presentation/screens/kas_masuk_screen.dart';
import '../features/kas/presentation/screens/kas_keluar_screen.dart';
import '../features/kas/presentation/screens/kas_report_screen.dart';
import '../features/fund_location/presentation/screens/fund_location_screen.dart';

class App extends ConsumerStatefulWidget {
  const App({super.key});

  @override
  ConsumerState<App> createState() => _AppState();
}

class _RouterRefreshNotifier extends ChangeNotifier {
  bool _lastLoggedIn = false;

  void update(bool isLoggedIn) {
    _lastLoggedIn = isLoggedIn;
    notifyListeners();
  }
}

class _AppState extends ConsumerState<App> {
  // Track targets currently being processed to prevent duplicate navigation
  // from multiple callbacks triggered by the same user interaction
  // (e.g., initial native link + initial Firebase message arriving within
  // milliseconds).  Entries are removed after a short debounce window so
  // that subsequent user taps — even on the same target — are always
  // allowed to navigate.
  final Set<String> _deeplinkInFlight = {};
  static const Duration _deeplinkDedupWindow = Duration(milliseconds: 2000);

  late final _RouterRefreshNotifier _refreshNotifier;
  late final GoRouter _router;
  final GlobalKey<ScaffoldMessengerState> _scaffoldMessengerKey =
      GlobalKey<ScaffoldMessengerState>();
  ProviderSubscription<AsyncValue<Map<String, dynamic>?>?>? _authSubscription;
  StreamSubscription? _foregroundSub;
  StreamSubscription? _messageOpenedSub;
  StreamSubscription? _nativeTappedSub;
  late final WidgetRef _ref;

  String _normalizeDeepLink(String? link) {
    if (link == null || link.trim().isEmpty) return '/home';
    final trimmed = link.trim();
    final uri = Uri.tryParse(trimmed);
    if (uri != null) {
      final path = uri.path;
      final billMatch = RegExp(
        r'^/(?:home/)?iuran/(?:detail/)?([a-zA-Z0-9_-]+)$',
      ).firstMatch(path);
      if (billMatch != null) {
        final billId = billMatch.group(1);
        if (billId != null &&
            billId.isNotEmpty &&
            billId != 'detail' &&
            billId != 'tunggakan' &&
            billId != 'laporan' &&
            billId != 'form' &&
            billId != 'tagihan' &&
            billId != 'warga' &&
            billId != 'list' &&
            billId != 'kelola') {
          final query = uri.hasQuery ? '?${uri.query}' : '';
          return '/home/iuran/detail/$billId$query';
        }
      }
    }
    if (trimmed == '/iuran' || trimmed == '/home/iuran') {
      return '/home/iuran';
    }
    if (trimmed.startsWith('/home')) {
      return trimmed;
    }
    if (trimmed.startsWith('/')) {
      return '/home$trimmed';
    }
    return '/home/$trimmed';
  }

  void _navigateToDeepLink(String? link) {
    final target = _normalizeDeepLink(link);

    // Skip if this target is already in the dedup window (covers duplicate
    // callbacks from the same user tap — initial native link + onMessage,
    // tap stream + onMessage, etc.).  After the window expires the target
    // is automatically removed so a subsequent user tap is always allowed.
    if (_deeplinkInFlight.contains(target)) {
      return;
    }
    _deeplinkInFlight.add(target);
    Future.delayed(_deeplinkDedupWindow, () {
      _deeplinkInFlight.remove(target);
    });

    final user = ref.read(authRepositoryProvider)?.valueOrNull;
    final isRTOperational =
        target.startsWith('/home/kas') ||
        target.startsWith('/home/iuran') ||
        target.startsWith('/home/warga');
    if (isRTOperational && !isAllowedRT(user)) {
      _router.go('/home');
      return;
    }
    try {
      _router.go(target);
    } catch (e) {
      debugPrint('Navigation error to $target: $e');
      _router.go('/home');
    }
  }


  @override
  void initState() {
    super.initState();
    _ref = ref;
    _refreshNotifier = _RouterRefreshNotifier();
    final initialAuth = ref.read(authRepositoryProvider);
    _refreshNotifier._lastLoggedIn = initialAuth?.valueOrNull != null;

    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(authRepositoryProvider.notifier).restoreSession();
      final notifService = ref.read(pushNotificationServiceProvider);
      notifService.initialize().then((_) {
        if (_refreshNotifier._lastLoggedIn) {
          notifService.onUserLoggedIn();
        }
        notifService.getInitialNativeDeepLink().then((deepLink) {
          if (deepLink != null && deepLink.isNotEmpty) {
            _navigateToDeepLink(deepLink);
          }
        });
        notifService.getInitialMessage().then((message) {
          if (message != null) {
            final deepLink = message.data['deep_link'] as String?;
            if (deepLink != null && deepLink.isNotEmpty) {
              _navigateToDeepLink(deepLink);
            }
          }
        });
      });

      _nativeTappedSub = notifService.onNativeNotificationTapped.listen((
        deepLink,
      ) {
        _navigateToDeepLink(deepLink);
      });

      _foregroundSub = notifService.onForegroundMessage.listen((message) {
        final notification = message.notification;
        final title =
            notification?.title ?? message.data['title'] ?? 'Notifikasi Baru';
        final body = notification?.body ?? message.data['body'] ?? '';
        final deepLink = message.data['deep_link'] as String?;

        final isAndroid =
            !kIsWeb && defaultTargetPlatform == TargetPlatform.android;
        if (isAndroid) {
          // Display true native Android heads-up banner at top of screen with high priority & action button
          notifService.showHeadsUpNotification(
            title: title,
            body: body,
            deepLink: deepLink ?? '',
            id: message.messageId.hashCode,
          );
          // Avoid showing dual notifications for the same message on Android
        } else {
          // Fallback to floating SnackBar on web or unsupported platforms
          _scaffoldMessengerKey.currentState?.hideCurrentSnackBar();
          _scaffoldMessengerKey.currentState?.showSnackBar(
            SnackBar(
              behavior: SnackBarBehavior.floating,
              elevation: 8,
              backgroundColor: const Color(
                0xFF1E293B,
              ), // Dark solid Slate-800 for high contrast in all themes
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(12),
                side: const BorderSide(color: Color(0xFF334155), width: 1),
              ),
              margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
              duration: const Duration(seconds: 6),
              content: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Container(
                    padding: const EdgeInsets.all(8),
                    decoration: BoxDecoration(
                      color: const Color(0xFFFBBF24).withValues(alpha: 0.15),
                      shape: BoxShape.circle,
                    ),
                    child: const Icon(
                      Icons.notifications_active_rounded,
                      color: Color(0xFFFBBF24),
                      size: 22,
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          title,
                          style: const TextStyle(
                            fontWeight: FontWeight.bold,
                            fontSize: 14,
                            color: Colors.white,
                          ),
                        ),
                        if (body.isNotEmpty) ...[
                          const SizedBox(height: 4),
                          Text(
                            body,
                            style: const TextStyle(
                              fontSize: 13,
                              color: Color(0xFFCBD5E1),
                            ),
                          ),
                        ],
                      ],
                    ),
                  ),
                ],
              ),
              action: (deepLink != null && deepLink.isNotEmpty)
                  ? SnackBarAction(
                      label: 'LIHAT',
                      textColor: const Color(0xFFFBBF24),
                      onPressed: () {
                        _navigateToDeepLink(deepLink);
                      },
                    )
                  : null,
            ),
          );
        }
      });

      _messageOpenedSub = notifService.onMessageOpenedApp.listen((message) {
        final deepLink = message.data['deep_link'] as String?;
        if (deepLink != null && deepLink.isNotEmpty) {
          _navigateToDeepLink(deepLink);
        }
      });
    });

    final authNotifier = ref.read(authRepositoryProvider.notifier);
    authNotifier.onBeforeLogout = () async {
      await _ref.read(pushNotificationServiceProvider).onUserLoggedOut();
    };

    _authSubscription = ref.listenManual<AsyncValue<Map<String, dynamic>?>?>(
      authRepositoryProvider,
      (previous, next) {
        final isLoggedIn = next?.valueOrNull != null;
        if (isLoggedIn != _refreshNotifier._lastLoggedIn) {
          if (!isLoggedIn) {
            _ref.read(wargaSearchQueryProvider.notifier).state = '';
            _ref.read(pushNotificationServiceProvider).onUserLoggedOut();
          } else {
            _ref.read(pushNotificationServiceProvider).onUserLoggedIn();
          }
        }
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
        if (loggedIn) {
          final user = authState?.valueOrNull;
          final loc = state.matchedLocation;
          final isRTOperational =
              loc.startsWith('/home/kas') ||
              loc.startsWith('/home/lokasi-dana') ||
              loc.startsWith('/home/iuran') ||
              loc.startsWith('/home/warga');
          if (isRTOperational && !isAllowedRT(user)) {
            return '/home';
          }
          if (loc.startsWith('/home/lokasi-dana')) {
            final role = user?['role'] as AppRole?;
            final systemRole = user?['system_role'] as String?;
            final jabatan = (user?['jabatan'] ?? '').toString().toLowerCase();
            final canAccess = systemRole == 'super_admin' ||
                role == AppRole.bendahara ||
                role == AppRole.pengurus ||
                jabatan == 'ketua' ||
                jabatan == 'bendahara';
            if (!canAccess) {
              return '/home';
            }
          }
          if (loc.startsWith('/home/iuran/')) {
            if (!canAccessIuranPath(loc, user)) {
              return '/home/iuran';
            }
          }
        }
        return null;
      },
      errorBuilder: (context, state) => Scaffold(
        appBar: AppBar(
          title: const Text('Halaman Tidak Ditemukan'),
        ),
        body: Center(
          child: Padding(
            padding: const EdgeInsets.all(24.0),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                const Icon(
                  Icons.error_outline,
                  size: 64,
                  color: AppColors.warning,
                ),
                const SizedBox(height: 16),
                const Text(
                  'Halaman Tidak Ditemukan',
                  style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
                ),
                const SizedBox(height: 8),
                Text(
                  'Rute "${state.uri.path}" tidak tersedia.',
                  textAlign: TextAlign.center,
                  style: const TextStyle(color: Colors.grey),
                ),
                const SizedBox(height: 24),
                ElevatedButton(
                  onPressed: () => context.go('/home'),
                  child: const Text('Kembali ke Beranda'),
                ),
              ],
            ),
          ),
        ),
      ),
      routes: [
        GoRoute(
          path: '/login',
          name: 'login',
          builder: (context, state) => const LoginScreen(),
        ),
        GoRoute(path: '/iuran', redirect: (context, state) => '/home/iuran'),
        GoRoute(
          path: '/iuran/detail/:billId',
          redirect: (context, state) =>
              '/home/iuran/detail/${state.pathParameters['billId']}',
        ),
        GoRoute(
          path: '/iuran/:billId',
          redirect: (context, state) {
            final billId = state.pathParameters['billId'];
            if (billId == null || billId.isEmpty) return '/home/iuran';
            if (billId == 'tunggakan') return '/home/iuran/tunggakan';
            if (billId == 'tagihan') return '/home/iuran';
            if (billId == 'warga') return '/home/iuran/warga';
            if (billId == 'kelola') return '/home/iuran/kelola';
            if (billId == 'list') return '/home/iuran/list';
            if (billId == 'laporan') return '/home/iuran/laporan';
            if (billId == 'form') return '/home/iuran/form';
            if (billId == 'detail') return '/home/iuran';
            return '/home/iuran/detail/$billId';
          },
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
                  builder: (context, state) => const IuranArrearsScreen(),
                  routes: [
                    GoRoute(
                      path: 'list',
                      name: 'iuran_list',
                      redirect: (context, state) => '/home/iuran/warga',
                    ),
                    GoRoute(
                      path: 'warga',
                      name: 'iuran_warga',
                      builder: (context, state) => const IuranScreen(),
                    ),
                    GoRoute(
                      path: 'tagihan',
                      name: 'iuran_tagihan',
                      redirect: (context, state) => '/home/iuran',
                    ),
                    GoRoute(
                      path: 'kelola',
                      name: 'iuran_kelola',
                      builder: (context, state) =>
                          const IuranManagementScreen(),
                    ),
                    GoRoute(
                      path: 'detail/:billId',
                      name: 'iuran_detail',
                      builder: (context, state) {
                        final billId = state.pathParameters['billId']!;
                        final paymentId =
                            state.uri.queryParameters['payment_id'];
                        return IuranDetailScreen(
                          billId: billId,
                          initialPaymentId: paymentId,
                        );
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
                      redirect: (context, state) => '/home/iuran',
                    ),
                    GoRoute(
                      path: 'laporan',
                      name: 'iuran_report',
                      builder: (context, state) => const IuranReportScreen(),
                    ),
                    GoRoute(
                      path: 'form',
                      name: 'iuran_form',
                      builder: (context, state) => const IuranFormScreen(),
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
                      path: 'detail/:id',
                      name: 'kas_detail_prefixed',
                      builder: (context, state) {
                        final txId = state.pathParameters['id']!;
                        return KasDetailScreen(transactionId: txId);
                      },
                    ),
                    GoRoute(
                      path: ':id',
                      name: 'kas_detail',
                      builder: (context, state) {
                        final txId = state.pathParameters['id']!;
                        return KasDetailScreen(transactionId: txId);
                      },
                    ),
                    GoRoute(
                      path: 'laporan',
                      name: 'kas_report',
                      builder: (context, state) => const KasReportScreen(),
                    ),
                    GoRoute(
                      path: 'lokasi-dana',
                      name: 'kas_lokasi_dana',
                      redirect: (context, state) => '/home/lokasi-dana',
                    ),
                  ],
                ),
                GoRoute(
                  path: 'lokasi-dana',
                  name: 'lokasi_dana',
                  builder: (context, state) => const FundLocationScreen(),
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
                        final systemRole =
                            authState?.valueOrNull?['system_role'] as String?;
                        return HouseholdCreateScreen(
                          isSuperAdmin: systemRole == 'super_admin',
                        );
                      },
                    ),
                    GoRoute(
                      path: 'edit/household/:householdId',
                      name: 'warga_edit',
                      builder: (context, state) {
                        final householdId =
                            state.pathParameters['householdId']!;
                        return HouseholdEditScreen(householdId: householdId);
                      },
                    ),
                    GoRoute(
                      path: 'baru/petugas',
                      name: 'warga_petugas_baru',
                      builder: (context, state) {
                        final authState = ref.read(authRepositoryProvider);
                        final systemRole =
                            authState?.valueOrNull?['system_role'] as String?;
                        return SpecialResidentCreateScreen(
                          isSuperAdmin: systemRole == 'super_admin',
                        );
                      },
                    ),
                    GoRoute(
                      path: 'edit/petugas/:residentId',
                      name: 'warga_resident_edit',
                      builder: (context, state) {
                        final residentId = state.pathParameters['residentId']!;
                        return SpecialResidentEditScreen(
                          residentId: residentId,
                        );
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
                GoRoute(
                  path: 'rw',
                  name: 'rw',
                  builder: (context, state) => const RwListScreen(),
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
    _foregroundSub?.cancel();
    _messageOpenedSub?.cancel();
    _nativeTappedSub?.cancel();
    _authSubscription?.close();
    _router.dispose();
    _refreshNotifier.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final themeMode = ref.watch(themeProvider);

    return MaterialApp.router(
      scaffoldMessengerKey: _scaffoldMessengerKey,
      title: 'Social Finance',
      debugShowCheckedModeBanner: false,
      theme: AppTheme.lightTheme,
      darkTheme: AppTheme.darkTheme,
      themeMode: themeMode.themeMode,
      routerConfig: _router,
    );
  }
}
