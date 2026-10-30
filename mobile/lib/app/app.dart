import 'dart:async';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/models/role.dart';

import 'theme/app_theme.dart';
import 'navigation_shell.dart';
import '../core/theme/app_colors.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/api/config.dart';
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
import '../features/iuran/presentation/screens/iuran_report_screen.dart';
import '../features/iuran/data/iuran_access.dart';
import '../features/kas/presentation/screens/kas_screen.dart';
import '../features/kas/presentation/screens/kas_detail_screen.dart';
import '../features/kas/presentation/screens/kas_masuk_screen.dart';
import '../features/kas/presentation/screens/kas_keluar_screen.dart';
import '../features/kas/presentation/screens/kas_report_screen.dart';
import '../features/fund_location/presentation/screens/fund_location_screen.dart';
import '../features/notification/data/models/notification_models.dart';
import '../features/notification/presentation/screens/notification_detail_screen.dart';
import '../features/notification/presentation/screens/notification_management_screen.dart';

/// Normalizes a notification deep link and/or FCM data payload into a valid
/// internal GoRouter destination path.
///
/// Rules:
/// - Payment/bill notifications (`iuran_payment_submitted`,
///   `iuran_payment_verified`, `iuran_bill_created`, or links targeting
///   `/iuran/detail/:billId`) route to `/home/iuran/detail/:billId`
///   (preserving `payment_id` query parameter).
/// - Broadcast/informational notifications (`manual_push`,
///   `waste_truck_arrival`, `test_self`, `/notifications`, or payloads with
///   `notification_id`) route to `/home/notifikasi/detail` (or
///   `/home/notifikasi/detail/:id`) with fallback query parameters.
/// - Unknown or incomplete notification payloads safely route to
///   `/home/notifikasi/detail` instead of producing an unregistered path.
String normalizeNotificationTarget(
  String? link, {
  Map<String, dynamic>? payload,
}) {
  String cleanString(dynamic val) => val?.toString().trim() ?? '';

  final rawLink = link?.trim() ?? '';
  final uri = rawLink.isNotEmpty ? Uri.tryParse(rawLink) : null;
  final path = uri?.path ?? '';
  final existingQuery = <String, String>{
    if (uri != null) ...uri.queryParameters,
  };

  final type = cleanString(payload?['type'] ?? existingQuery['type']);
  final notificationId = cleanString(
    payload?['notification_id'] ?? existingQuery['notification_id'],
  );
  final billIdFromPayload = cleanString(
    payload?['bill_id'] ?? existingQuery['bill_id'],
  );
  final paymentIdFromPayload = cleanString(
    payload?['payment_id'] ?? existingQuery['payment_id'],
  );
  final title = cleanString(payload?['title'] ?? existingQuery['title']);
  final body = cleanString(payload?['body'] ?? existingQuery['body']);
  final imageUrl =
      cleanString(payload?['image_url'] ?? existingQuery['image_url']);
  final sentAt = cleanString(payload?['sent_at'] ?? existingQuery['sent_at']);

  final isPaymentType = type == 'iuran_payment_submitted' ||
      type == 'iuran_payment_verified' ||
      type == 'iuran_bill_created';

  final isDetailNotificationType = type == 'manual_push' ||
      type == 'waste_truck_arrival' ||
      type == 'test_self';

  // 1. Check explicit /iuran/detail/:billId or /iuran/:billId patterns
  if (!isDetailNotificationType && path.isNotEmpty) {
    final billMatch = RegExp(
      r'^/(?:home/)?iuran/(?:detail/)?([a-zA-Z0-9_-]+)$',
    ).firstMatch(path);
    if (billMatch != null) {
      final matchedBillId = billMatch.group(1);
      if (matchedBillId != null &&
          matchedBillId.isNotEmpty &&
          matchedBillId != 'detail' &&
          matchedBillId != 'tunggakan' &&
          matchedBillId != 'laporan' &&
          matchedBillId != 'form' &&
          matchedBillId != 'tagihan' &&
          matchedBillId != 'warga' &&
          matchedBillId != 'list' &&
          matchedBillId != 'kelola') {
        final queryParams = <String, String>{...existingQuery};
        if (paymentIdFromPayload.isNotEmpty &&
            !queryParams.containsKey('payment_id')) {
          queryParams['payment_id'] = paymentIdFromPayload;
        }
        final queryString = queryParams.isNotEmpty
            ? '?${Uri(queryParameters: queryParams).query}'
            : '';
        return '/home/iuran/detail/$matchedBillId$queryString';
      }
    }
  }

  // 2. Payment notification type with bill_id in payload
  if (isPaymentType) {
    if (billIdFromPayload.isNotEmpty) {
      final queryParams = <String, String>{};
      if (paymentIdFromPayload.isNotEmpty) {
        queryParams['payment_id'] = paymentIdFromPayload;
      }
      final queryString = queryParams.isNotEmpty
          ? '?${Uri(queryParameters: queryParams).query}'
          : '';
      return '/home/iuran/detail/$billIdFromPayload$queryString';
    }
    return '/home/iuran';
  }

  // 3. Notification detail helper
  String buildNotificationDetailUrl({String? explicitId}) {
    final resolvedId =
        (explicitId != null && explicitId.isNotEmpty) ? explicitId : notificationId;
    final basePath = resolvedId.isNotEmpty
        ? '/home/notifikasi/detail/$resolvedId'
        : '/home/notifikasi/detail';
    final queryParams = <String, String>{};
    if (title.isNotEmpty) queryParams['title'] = title;
    if (body.isNotEmpty) queryParams['body'] = body;
    if (imageUrl.isNotEmpty) queryParams['image_url'] = imageUrl;
    if (type.isNotEmpty) queryParams['type'] = type;
    if (sentAt.isNotEmpty) queryParams['sent_at'] = sentAt;
    final queryString = queryParams.isNotEmpty
        ? '?${Uri(queryParameters: queryParams).query}'
        : '';
    return '$basePath$queryString';
  }

  // 4. Explicit notification detail path or legacy /notifications path
  final notifDetailMatch = RegExp(
    r'^/(?:home/)?(?:notifikasi/detail|notifications/detail)(?:/([a-zA-Z0-9_-]+))?$',
  ).firstMatch(path);
  if (notifDetailMatch != null) {
    return buildNotificationDetailUrl(explicitId: notifDetailMatch.group(1));
  }

  if (path == '/notifications' ||
      path == '/home/notifications' ||
      isDetailNotificationType ||
      notificationId.isNotEmpty) {
    return buildNotificationDetailUrl();
  }

  // 5. Standard known routes
  if (rawLink == '/iuran' || rawLink == '/home/iuran') {
    return '/home/iuran';
  }

  if (rawLink.isEmpty) {
    if (payload != null && payload.isNotEmpty) {
      return buildNotificationDetailUrl();
    }
    return '/home';
  }

  final candidate = rawLink.startsWith('/home')
      ? rawLink
      : (rawLink.startsWith('/') ? '/home$rawLink' : '/home/$rawLink');
  final candidatePath = Uri.tryParse(candidate)?.path ?? candidate;

  final isKnownRoute = candidatePath == '/home' ||
      candidatePath == '/home/cash' ||
      candidatePath == '/home/dues' ||
      candidatePath == '/home/iuran' ||
      candidatePath.startsWith('/home/iuran/') ||
      candidatePath == '/home/kas' ||
      candidatePath.startsWith('/home/kas/') ||
      candidatePath == '/home/lokasi-dana' ||
      candidatePath == '/home/warga' ||
      candidatePath.startsWith('/home/warga/') ||
      candidatePath == '/home/reports' ||
      candidatePath == '/home/profile' ||
      candidatePath == '/home/rw' ||
      candidatePath == '/home/notifikasi' ||
      candidatePath.startsWith('/home/notifikasi/');

  if (isKnownRoute) {
    return candidate;
  }

  // 6. Safe fallback for unknown notification links/payloads so GoRouter never
  // lands on the error page when opening a notification.
  return buildNotificationDetailUrl();
}

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

class _AppState extends ConsumerState<App> with WidgetsBindingObserver {
  // Track targets currently being processed to prevent duplicate navigation
  // from multiple callbacks triggered by the same user interaction
  // (e.g., initial native link + initial Firebase message arriving within
  // milliseconds).  Entries are removed after a short debounce window so
  // that subsequent user taps — even on the same target — are always
  // allowed to navigate.
  final Set<String> _deeplinkInFlight = {};
  static const Duration _deeplinkDedupWindow = Duration(milliseconds: 2000);

  /// Pending destination captured when a notification is tapped while the user
  /// is not authenticated (or while startup session restore is in flight, or
  /// when token refresh fails permanently while opening a notification).
  /// Consumed and cleared once authentication completes.
  String? _pendingDeepLink;
  String? _lastNotificationTarget;
  bool _isExplicitLogout = false;

  late final _RouterRefreshNotifier _refreshNotifier;
  late final GoRouter _router;
  final GlobalKey<ScaffoldMessengerState> _scaffoldMessengerKey =
      GlobalKey<ScaffoldMessengerState>();
  ProviderSubscription<AsyncValue<Map<String, dynamic>?>?>? _authSubscription;
  StreamSubscription? _foregroundSub;
  StreamSubscription? _messageOpenedSub;
  StreamSubscription? _nativeTappedSub;
  late final WidgetRef _ref;

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    super.didChangeAppLifecycleState(state);
    if (state == AppLifecycleState.resumed) {
      // Never clear session on background/idle. When resuming, if the user's
      // profile is stale (restored from cache while offline) or in an error/
      // permissions_unavailable state, trigger a single-flight profile reload.
      final user = ref.read(authRepositoryProvider)?.valueOrNull;
      final profileStatus = user?['profile_status'] as String?;
      if (user != null &&
          (profileStatus == 'stale' ||
              profileStatus == 'error' ||
              profileStatus == 'permissions_unavailable')) {
        ref.read(authRepositoryProvider.notifier).reloadProfile();
      }
    }
  }

  String _normalizeDeepLink(String? link, {Map<String, dynamic>? payload}) {
    return normalizeNotificationTarget(link, payload: payload);
  }

  bool _canUserAccessNotificationMenu(Map<String, dynamic>? user) {
    return canUserAccessNotificationMenu(user);
  }

  String _resolveAuthorizedTarget(
    String target,
    Map<String, dynamic>? user,
  ) {
    final uri = Uri.tryParse(target);
    final loc = uri?.path ?? target;

    if (loc == '/home/notifikasi' && !_canUserAccessNotificationMenu(user)) {
      return '/home';
    }

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

    return target;
  }

  void _navigateToDeepLink(String? link, {Map<String, dynamic>? payload}) {
    final target = _normalizeDeepLink(link, payload: payload);

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
    if (user == null) {
      // User is not logged in (or session restoration is still in progress).
      // Save the validated pending destination so GoRouter can navigate to it
      // immediately after login or session restore succeeds.
      if (target != '/home' && target != '/login') {
        _pendingDeepLink = target;
      }
      _router.go('/login');
      return;
    }

    if (target != '/home' && target != '/login') {
      _lastNotificationTarget = target;
    }
    final authorizedTarget = _resolveAuthorizedTarget(target, user);
    try {
      _router.go(authorizedTarget);
    } catch (e) {
      debugPrint('Navigation error to $authorizedTarget: $e');
      _router.go('/home');
    }
  }

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _ref = ref;
    _refreshNotifier = _RouterRefreshNotifier();
    final initialAuth = ref.read(authRepositoryProvider);
    _refreshNotifier._lastLoggedIn = initialAuth?.valueOrNull != null;

    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(authRepositoryProvider.notifier).restoreSession();
      final notifService = ref.read(pushNotificationServiceProvider);
      notifService.getInitialNativeNotificationPayload().then((payload) {
        if (payload != null && payload.isNotEmpty) {
          final deepLink = payload['deep_link'] as String?;
          _navigateToDeepLink(deepLink, payload: payload);
        } else {
          notifService.getInitialNativeDeepLink().then((deepLink) {
            if (deepLink != null && deepLink.isNotEmpty) {
              _navigateToDeepLink(deepLink);
            }
          });
        }
      });
      notifService.initialize().then((_) {
        if (_refreshNotifier._lastLoggedIn) {
          notifService.onUserLoggedIn();
        }
        notifService.getInitialMessage().then((message) {
          if (message != null) {
            final notification = message.notification;
            final payload = <String, dynamic>{
              ...message.data,
              if ((message.data['title'] == null ||
                      message.data['title'].toString().isEmpty) &&
                  notification?.title != null)
                'title': notification!.title,
              if ((message.data['body'] == null ||
                      message.data['body'].toString().isEmpty) &&
                  notification?.body != null)
                'body': notification!.body,
              if ((message.data['image_url'] == null ||
                      message.data['image_url'].toString().isEmpty) &&
                  notification?.android?.imageUrl != null)
                'image_url': notification!.android!.imageUrl,
            };
            final deepLink = payload['deep_link'] as String?;
            if ((deepLink != null && deepLink.isNotEmpty) ||
                payload.isNotEmpty) {
              _navigateToDeepLink(deepLink, payload: payload);
            }
          }
        });
      });

      _nativeTappedSub =
          notifService.onNativeNotificationPayloadTapped.listen((payload) {
        final deepLink = payload['deep_link'] as String?;
        _navigateToDeepLink(deepLink, payload: payload);
      });

      _foregroundSub = notifService.onForegroundMessage.listen((message) {
        final notification = message.notification;
        final title =
            notification?.title ?? message.data['title'] ?? 'Notifikasi Baru';
        final body = notification?.body ?? message.data['body'] ?? '';
        final rawDeepLink = message.data['deep_link'] as String?;
        final rawImageUrl = (message.data['image_url'] as String?) ??
            notification?.android?.imageUrl;
        final activeBaseUrl = ref.read(apiClientProvider).baseUrl;
        final imageUrl = ApiConfig.resolveImageUrl(
          rawImageUrl,
          fallbackBaseUrl: activeBaseUrl,
        );
        final payload = <String, dynamic>{
          ...message.data,
          'title': title,
          'body': body,
          if (imageUrl != null && imageUrl.isNotEmpty) 'image_url': imageUrl,
        };
        final normalizedTarget = _normalizeDeepLink(
          rawDeepLink,
          payload: payload,
        );

        final isAndroid =
            !kIsWeb && defaultTargetPlatform == TargetPlatform.android;
        if (isAndroid) {
          // Display true native Android heads-up banner at top of screen with high priority & action button
          notifService.showHeadsUpNotification(
            title: title,
            body: body,
            deepLink: normalizedTarget,
            imageUrl: imageUrl,
            notificationId: message.data['notification_id'] as String?,
            type: message.data['type'] as String?,
            sentAt: message.data['sent_at'] as String?,
            id: (message.messageId ??
                    '${message.data['notification_id'] ?? title}:$body')
                .hashCode,
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
              action: SnackBarAction(
                label: 'LIHAT',
                textColor: const Color(0xFFFBBF24),
                onPressed: () {
                  _navigateToDeepLink(rawDeepLink, payload: payload);
                },
              ),
            ),
          );
        }
      });

      _messageOpenedSub = notifService.onMessageOpenedApp.listen((message) {
        final notification = message.notification;
        final payload = <String, dynamic>{
          ...message.data,
          if ((message.data['title'] == null ||
                  message.data['title'].toString().isEmpty) &&
              notification?.title != null)
            'title': notification!.title,
          if ((message.data['body'] == null ||
                  message.data['body'].toString().isEmpty) &&
              notification?.body != null)
            'body': notification!.body,
          if ((message.data['image_url'] == null ||
                  message.data['image_url'].toString().isEmpty) &&
              notification?.android?.imageUrl != null)
            'image_url': notification!.android!.imageUrl,
        };
        final deepLink = payload['deep_link'] as String?;
        if ((deepLink != null && deepLink.isNotEmpty) || payload.isNotEmpty) {
          _navigateToDeepLink(deepLink, payload: payload);
        }
      });
    });

    final authNotifier = ref.read(authRepositoryProvider.notifier);
    authNotifier.onBeforeLogout = () async {
      _isExplicitLogout = true;
      _pendingDeepLink = null;
      _lastNotificationTarget = null;
      await _ref.read(pushNotificationServiceProvider).onUserLoggedOut();
    };

    _authSubscription = ref.listenManual<AsyncValue<Map<String, dynamic>?>?>(
      authRepositoryProvider,
      (previous, next) {
        final isLoggedIn = next?.valueOrNull != null;
        if (isLoggedIn != _refreshNotifier._lastLoggedIn) {
          if (!isLoggedIn) {
            if (!_isExplicitLogout &&
                _pendingDeepLink == null &&
                _lastNotificationTarget != null) {
              // If session refresh failed permanently while opening a
              // notification destination, preserve it as pendingDeepLink so
              // the user lands on the notification target after re-login.
              _pendingDeepLink = _lastNotificationTarget;
            }
            _lastNotificationTarget = null;
            _isExplicitLogout = false;
            _ref.read(wargaSearchQueryProvider.notifier).state = '';
            _ref.read(pushNotificationServiceProvider).onUserLoggedOut();
          } else {
            _isExplicitLogout = false;
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
        if (loggedIn && _pendingDeepLink != null) {
          final pending = _pendingDeepLink!;
          _pendingDeepLink = null;
          _lastNotificationTarget = null;
          final user = authState?.valueOrNull;
          return _resolveAuthorizedTarget(pending, user);
        }
        if (loggedIn && state.matchedLocation == '/login') {
          return '/home';
        }
        if (!loggedIn && state.matchedLocation != '/login') {
          return '/login';
        }
        if (loggedIn) {
          final user = authState?.valueOrNull;
          final loc = state.matchedLocation;
          if (loc == '/home/notifikasi' &&
              !_canUserAccessNotificationMenu(user)) {
            return '/home';
          }
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
        GoRoute(
          path: '/notifications',
          redirect: (context, state) {
            final query = state.uri.hasQuery ? '?${state.uri.query}' : '';
            return '/home/notifikasi/detail$query';
          },
        ),
        GoRoute(path: '/iuran', redirect: (context, state) => '/home/iuran'),
        GoRoute(
          path: '/iuran/detail/:billId',
          redirect: (context, state) {
            final query = state.uri.hasQuery ? '?${state.uri.query}' : '';
            return '/home/iuran/detail/${state.pathParameters['billId']}$query';
          },
        ),
        GoRoute(
          path: '/iuran/:billId',
          redirect: (context, state) {
            final billId = state.pathParameters['billId'];
            if (billId == null || billId.isEmpty) return '/home/iuran';
            if (billId == 'tunggakan') return '/home/iuran/tunggakan';
            if (billId == 'tagihan') return '/home/iuran';
            if (billId == 'kelola') return '/home/iuran/kelola';
            if (billId == 'laporan') return '/home/iuran/laporan';
            if (billId == 'form') return '/home/iuran/form';
            if (billId == 'detail') return '/home/iuran';
            final query = state.uri.hasQuery ? '?${state.uri.query}' : '';
            return '/home/iuran/detail/$billId$query';
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
            ),
            GoRoute(
              path: '/home/notifikasi',
              name: 'notifikasi',
              builder: (context, state) =>
                  const NotificationManagementScreen(),
              routes: [
                GoRoute(
                  path: 'detail',
                  name: 'notifikasi_detail',
                  builder: (context, state) {
                    final q = state.uri.queryParameters;
                    return NotificationDetailScreen(
                      notificationId: q['notification_id'],
                      initialTitle: q['title'],
                      initialBody: q['body'],
                      initialImageUrl: q['image_url'],
                      initialEventType: q['type'],
                      initialSentAt: q['sent_at'],
                      openedFromNotificationList: q['from'] == 'notifikasi',
                    );
                  },
                ),
                GoRoute(
                  path: 'detail/:id',
                  name: 'notifikasi_detail_by_id',
                  builder: (context, state) {
                    final q = state.uri.queryParameters;
                    return NotificationDetailScreen(
                      notificationId: state.pathParameters['id'],
                      initialTitle: q['title'],
                      initialBody: q['body'],
                      initialImageUrl: q['image_url'],
                      initialEventType: q['type'],
                      initialSentAt: q['sent_at'],
                      openedFromNotificationList: q['from'] == 'notifikasi',
                    );
                  },
                ),
              ],
            ),
            GoRoute(
              path: '/home/cash',
              name: 'cash',
              builder: (context, state) => const CashScreen(),
            ),
            GoRoute(
              path: '/home/dues',
              name: 'dues',
              builder: (context, state) => const DuesScreen(),
            ),
            GoRoute(
              path: '/home/iuran',
              name: 'iuran',
              builder: (context, state) => const IuranScreen(),
              routes: [
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
              path: '/home/kas',
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
              ],
            ),
            GoRoute(
              path: '/home/lokasi-dana',
              name: 'lokasi_dana',
              builder: (context, state) => const FundLocationScreen(),
            ),
            GoRoute(
              path: '/home/warga',
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
              path: '/home/reports',
              name: 'reports',
              builder: (context, state) => const ReportsScreen(),
            ),
            GoRoute(
              path: '/home/profile',
              name: 'profile',
              builder: (context, state) => const ProfileScreen(),
            ),
            GoRoute(
              path: '/home/rw',
              name: 'rw',
              builder: (context, state) => const RwListScreen(),
            ),
          ],
        ),
      ],
    );
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
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
