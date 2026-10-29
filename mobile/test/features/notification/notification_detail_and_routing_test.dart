import 'package:dio/dio.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/app/app.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/api/config.dart';
import 'package:social_finance/core/api/device_service.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/core/notification/push_notification_service.dart';
import 'package:social_finance/core/storage/app_storage.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/dashboard/presentation/screens/dashboard_screen.dart';
import 'package:social_finance/features/notification/data/models/notification_models.dart';
import 'package:social_finance/features/notification/data/notification_api_client.dart';
import 'package:social_finance/features/notification/data/notification_repository.dart';
import 'package:social_finance/features/notification/presentation/screens/notification_detail_screen.dart';
import 'package:social_finance/features/notification/presentation/screens/notification_management_screen.dart';

class _FakeDeviceService implements DeviceService {
  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);

  @override
  Future<void> registerDevice({
    required String fcmToken,
    String platform = 'android',
    String? deviceId,
  }) async {}

  @override
  Future<void> unregisterDevice(String fcmToken) async {}
}

class _FakePreferencesStorage implements PreferencesStorage {
  String? lastApiUrl = 'http://192.168.1.10:8085';
  bool rememberMe = false;

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);

  @override
  Future<String?> getLastApiUrl() async => lastApiUrl;

  @override
  Future<void> setLastApiUrl(String url) async => lastApiUrl = url;

  @override
  Future<void> clearLastApiUrl() async => lastApiUrl = null;

  @override
  Future<String?> getLastEmail() async => null;

  @override
  Future<void> setLastEmail(String email) async {}

  @override
  Future<void> clearLastEmail() async {}

  @override
  Future<String?> getLastLoginMethod() async => 'email';

  @override
  Future<void> setLastLoginMethod(String method) async {}

  @override
  Future<String?> getLastPhone() async => null;

  @override
  Future<void> setLastPhone(String phone) async {}

  @override
  Future<void> clearLastPhone() async {}

  @override
  Future<bool> getRememberMe() async => rememberMe;

  @override
  Future<void> setRememberMe(bool value) async => rememberMe = value;
}

class _FakeSecureAuthStorage implements SecureAuthStorage {
  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);

  @override
  Future<void> clearLoginCredentials() async {}

  @override
  Future<void> clearSession() async {}

  @override
  Future<StoredLoginCredentials?> getLoginCredentials() async => null;

  @override
  Future<StoredSession?> getSession() async => null;

  @override
  Future<bool> hasSession() async => false;

  @override
  Future<void> saveLoginCredentials({
    required String loginMethod,
    required String identifier,
    required String password,
    required String serverUrl,
  }) async {}

  @override
  Future<void> saveSession({
    required String accessToken,
    required String refreshToken,
    required String serverUrl,
    required Map<String, dynamic> userData,
  }) async {}
}

class _FakeAuthService implements AuthService {
  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _ControllableAuthRepository extends AuthRepository {
  _ControllableAuthRepository({
    required super.apiClient,
    super.preferencesStorage,
    super.secureAuthStorage,
    Map<String, dynamic>? initialUser,
  }) : super(authService: _FakeAuthService()) {
    if (initialUser != null) {
      state = AsyncData(initialUser);
    }
  }

  @override
  Future<bool> restoreSession() async {
    return state?.valueOrNull != null;
  }

  void completeLogin(Map<String, dynamic> user) {
    state = AsyncData(user);
  }

  void setLoggedOut() {
    state = null;
  }
}

class _FakeNotificationRepository extends NotificationRepository {
  int detailFetchCount = 0;
  NotificationLogEntry? entryToReturn;

  _FakeNotificationRepository({this.entryToReturn})
      : super(apiClient: NotificationApiClient(dio: Dio()));

  @override
  Future<NotificationLogEntry> fetchNotificationDetail(String id) async {
    detailFetchCount++;
    return entryToReturn ??
        NotificationLogEntry(
          id: id,
          rtId: 'rt-01',
          eventType: 'waste_truck_arrival',
          title: 'Mobil Sampah Tiba di RT 01',
          body: 'Silakan keluarkan sampah rumah tangga Anda sekarang.',
          imageUrl: '/api/v1/notifications/images/rt-01/truck.png',
          variables: const {
            'image_url': '/api/v1/notifications/images/rt-01/truck.png',
          },
          recipientsCount: 15,
          createdAt: DateTime.utc(2026, 10, 29, 7, 30),
        );
  }

  @override
  Future<List<SystemNotificationType>> fetchSystemTypes() async {
    return const [
      SystemNotificationType(
        eventType: 'waste_truck_arrival',
        name: 'Kedatangan Mobil Sampah',
        trigger: 'Pemicu manual oleh Pengurus RT',
        recipientsRule: 'Seluruh warga RT',
        defaultTitle: 'Mobil Sampah Tiba',
        defaultBody: 'Truk sampah sudah berada di lingkungan RT.',
        allowedVariables: [],
      ),
    ];
  }

  @override
  Future<List<NotificationTemplate>> fetchTemplates() async => const [];

  @override
  Future<NotificationLogsResponse> fetchLogs({
    int page = 1,
    int perPage = 20,
    String? eventType,
  }) async {
    return NotificationLogsResponse(
      data: [
        NotificationLogEntry(
          id: 'log-in-app-1',
          rtId: 'rt-01',
          eventType: 'waste_truck_arrival',
          title: 'Mobil Sampah Tiba di RT 01',
          body: 'Silakan keluarkan sampah rumah tangga Anda sekarang.',
          variables: const {},
          recipientsCount: 15,
          createdAt: DateTime.utc(2026, 10, 29, 7, 30),
        ),
      ],
      page: 1,
      perPage: 20,
      total: 1,
      totalPages: 1,
    );
  }
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('normalizeNotificationTarget — Routing & Deep Link Parser', () {
    test('routes manual_push and legacy /notifications to /home/notifikasi/detail', () {
      final target = normalizeNotificationTarget(
        '/notifications',
        payload: {
          'type': 'manual_push',
          'notification_id': 'notif-123',
          'title': 'Kerja Bakti Minggu Pagi',
          'body': 'Warga dimohon berkumpul pukul 07.00 WIB.',
          'image_url': '/api/v1/notifications/images/rt-1/banner.jpg',
        },
      );

      final uri = Uri.parse(target);
      expect(uri.path, equals('/home/notifikasi/detail/notif-123'));
      expect(uri.queryParameters['title'], equals('Kerja Bakti Minggu Pagi'));
      expect(
        uri.queryParameters['body'],
        equals('Warga dimohon berkumpul pukul 07.00 WIB.'),
      );
      expect(
        uri.queryParameters['image_url'],
        equals('/api/v1/notifications/images/rt-1/banner.jpg'),
      );
      expect(uri.queryParameters['type'], equals('manual_push'));
    });

    test('routes waste_truck_arrival to /home/notifikasi/detail even if legacy link was /iuran', () {
      final target = normalizeNotificationTarget(
        '/iuran',
        payload: {
          'type': 'waste_truck_arrival',
          'notification_id': 'truck-456',
          'title': 'Mobil Sampah Tiba',
          'body': 'Petugas kebersihan sudah tiba.',
        },
      );

      final uri = Uri.parse(target);
      expect(uri.path, equals('/home/notifikasi/detail/truck-456'));
      expect(uri.queryParameters['type'], equals('waste_truck_arrival'));
      expect(uri.queryParameters['title'], equals('Mobil Sampah Tiba'));
    });

    test('preserves payment verification deep link for iuran_payment_submitted', () {
      final target = normalizeNotificationTarget(
        '/iuran/detail/bill-777?payment_id=pay-888',
        payload: {
          'type': 'iuran_payment_submitted',
          'bill_id': 'bill-777',
          'payment_id': 'pay-888',
          'status': 'pending',
        },
      );

      expect(
        target,
        equals('/home/iuran/detail/bill-777?payment_id=pay-888'),
      );
    });

    test('constructs payment verification route from payload if deep_link is missing', () {
      final target = normalizeNotificationTarget(
        '',
        payload: {
          'type': 'iuran_payment_verified',
          'bill_id': 'bill-555',
          'payment_id': 'pay-666',
          'status': 'verified',
        },
      );

      expect(
        target,
        equals('/home/iuran/detail/bill-555?payment_id=pay-666'),
      );
    });

    test('routes unknown or incomplete notification payload safely to /home/notifikasi/detail', () {
      final target = normalizeNotificationTarget(
        '/unregistered/legacy/route',
        payload: {
          'title': 'Pengumuman RT',
          'body': 'Isi pengumuman penting.',
        },
      );

      final uri = Uri.parse(target);
      expect(uri.path, equals('/home/notifikasi/detail'));
      expect(uri.queryParameters['title'], equals('Pengumuman RT'));
      expect(uri.queryParameters['body'], equals('Isi pengumuman penting.'));
    });
  });

  group('ApiConfig.resolveImageUrl — Image URL Normalization & Security', () {
    tearDown(() {
      ApiConfig.setRuntimeBaseUrl('');
    });

    test('resolves relative /api/v1/... URL using runtime base URL without throwing ApiConfigException', () {
      ApiConfig.setRuntimeBaseUrl('http://192.168.1.50:8085');

      final resolved = ApiConfig.resolveImageUrl(
        '/api/v1/notifications/images/rt-01/photo.png',
      );

      expect(
        resolved,
        equals('http://192.168.1.50:8085/api/v1/notifications/images/rt-01/photo.png'),
      );
    });

    test('blocks path traversal and non-http schemes', () {
      ApiConfig.setRuntimeBaseUrl('http://192.168.1.50:8085');

      expect(
        ApiConfig.resolveImageUrl('/api/v1/notifications/images/../secret.png'),
        isNull,
      );
      expect(
        ApiConfig.resolveImageUrl('file:///etc/passwd'),
        isNull,
      );
      expect(
        ApiConfig.resolveImageUrl('javascript:alert(1)'),
        isNull,
      );
    });
  });

  group('NotificationDetailScreen — Content, API Enrichment & Image Fallback', () {
    testWidgets('renders title, body, category, and fetches detail from API once', (tester) async {
      ApiConfig.setRuntimeBaseUrl('http://192.168.1.10:8085');
      addTearDown(() => ApiConfig.setRuntimeBaseUrl(''));

      final fakeRepo = _FakeNotificationRepository();

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            notificationRepositoryProvider.overrideWithValue(fakeRepo),
          ],
          child: const MaterialApp(
            home: NotificationDetailScreen(
              notificationId: 'notif-001',
              initialTitle: 'Judul Awal',
              initialBody: 'Pesan Awal',
              initialEventType: 'waste_truck_arrival',
            ),
          ),
        ),
      );

      // Initial payload data is shown immediately while loading
      expect(find.text('Judul Awal'), findsOneWidget);

      await tester.pumpAndSettle();

      // Enriched API response replaces title/body and only fetched once
      expect(fakeRepo.detailFetchCount, equals(1));
      expect(find.text('Mobil Sampah Tiba di RT 01'), findsOneWidget);
      expect(
        find.text('Silakan keluarkan sampah rumah tangga Anda sekarang.'),
        findsOneWidget,
      );
      expect(find.text('Kedatangan Truk Sampah'), findsOneWidget);
      expect(find.byKey(const Key('notification_detail_image_tap')), findsOneWidget);
    });

    testWidgets('shows graceful fallback when image fails to load instead of error page', (tester) async {
      ApiConfig.setRuntimeBaseUrl('http://127.0.0.1:9');
      addTearDown(() => ApiConfig.setRuntimeBaseUrl(''));

      final fakeRepo = _FakeNotificationRepository(
        entryToReturn: NotificationLogEntry(
          id: 'notif-img-fail',
          rtId: 'rt-01',
          eventType: 'manual_push',
          title: 'Pengumuman Gambar Rusak',
          body: 'Tetap tampilkan isi pesan meskipun gambar gagal dimuat.',
          imageUrl: '/api/v1/notifications/images/rt-01/missing.png',
          variables: const {},
          recipientsCount: 5,
          createdAt: DateTime.utc(2026, 10, 29, 9, 0),
        ),
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            notificationRepositoryProvider.overrideWithValue(fakeRepo),
          ],
          child: const MaterialApp(
            home: NotificationDetailScreen(
              notificationId: 'notif-img-fail',
            ),
          ),
        ),
      );

      await tester.pumpAndSettle();

      expect(find.text('Pengumuman Gambar Rusak'), findsOneWidget);
      expect(
        find.text('Tetap tampilkan isi pesan meskipun gambar gagal dimuat.'),
        findsOneWidget,
      );
      expect(find.byKey(const Key('notification_detail_image_fallback')), findsOneWidget);
      expect(find.text('Gambar lampiran tidak dapat dimuat.'), findsOneWidget);
    });
  });

  group('End-to-End App Notification Flows (Masalah 1, 2, 3, 4)', () {
    late ApiClient apiClient;
    late _FakePreferencesStorage fakePrefs;
    late _FakeSecureAuthStorage fakeSecure;
    late PushNotificationService pushService;
    late _FakeNotificationRepository fakeNotifRepo;
    late List<MethodCall> nativeCalls;

    setUp(() {
      apiClient = ApiClient(baseUrl: 'http://192.168.1.10:8085');
      fakePrefs = _FakePreferencesStorage();
      fakeSecure = _FakeSecureAuthStorage();
      pushService = PushNotificationService(_FakeDeviceService());
      fakeNotifRepo = _FakeNotificationRepository();
      nativeCalls = <MethodCall>[];

      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(
        const MethodChannel('com.example.social_finance/notifications'),
        (call) async {
          nativeCalls.add(call);
          if (call.method == 'showHeadsUpNotification') return true;
          if (call.method == 'getInitialNotificationPayload') return null;
          if (call.method == 'getInitialDeepLink') return null;
          return null;
        },
      );
    });

    tearDown(() {
      debugDefaultTargetPlatformOverride = null;
      ApiConfig.setRuntimeBaseUrl('');
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(
        const MethodChannel('com.example.social_finance/notifications'),
        null,
      );
    });

    testWidgets(
      'Masalah 2 & 4: Foreground message on Android triggers single native heads-up notification with normalized image_url',
      (tester) async {
        debugDefaultTargetPlatformOverride = TargetPlatform.android;
        try {
          final authRepo = _ControllableAuthRepository(
            apiClient: apiClient,
            preferencesStorage: fakePrefs,
            secureAuthStorage: fakeSecure,
            initialUser: {
              'id': 'user-warga-1',
              'name': 'Budi Warga',
              'role': AppRole.warga,
              'system_role': 'warga',
              'rt_id': 'rt-01',
              'profile_status': 'ready',
            },
          );

          await tester.pumpWidget(
            ProviderScope(
              overrides: [
                apiClientProvider.overrideWithValue(apiClient),
                preferencesStorageProvider.overrideWithValue(fakePrefs),
                secureAuthStorageProvider.overrideWithValue(fakeSecure),
                authRepositoryProvider.overrideWith((ref) => authRepo),
                pushNotificationServiceProvider.overrideWithValue(pushService),
                notificationRepositoryProvider.overrideWithValue(fakeNotifRepo),
              ],
              child: const App(),
            ),
          );
          await tester.pump();

          // Emit a foreground FCM message with a relative image URL
          pushService.simulateForegroundMessage(
            const RemoteMessage(
              messageId: 'msg-fg-1',
              notification: RemoteNotification(
                title: 'Truk Sampah Tiba',
                body: 'Segera keluarkan sampah Anda.',
              ),
              data: {
                'type': 'waste_truck_arrival',
                'notification_id': 'notif-fg-1',
                'deep_link': '/home/notifikasi/detail/notif-fg-1',
                'image_url': '/api/v1/notifications/images/rt-01/truck.png',
              },
            ),
          );
          await tester.pumpAndSettle();

          final showCalls = nativeCalls
              .where((c) => c.method == 'showHeadsUpNotification')
              .toList();
          expect(showCalls.length, equals(1));
          final args =
              Map<String, dynamic>.from(showCalls.first.arguments as Map);
          expect(args['title'], equals('Truk Sampah Tiba'));
          expect(args['body'], equals('Segera keluarkan sampah Anda.'));
          expect(
            args['image_url'],
            equals(
              'http://192.168.1.10:8085/api/v1/notifications/images/rt-01/truck.png',
            ),
          );
          expect(
            args['deep_link'],
            contains('/home/notifikasi/detail/notif-fg-1'),
          );
        } finally {
          debugDefaultTargetPlatformOverride = null;
        }
      },
    );

    testWidgets(
      'Masalah 1 & 3: Tapping notification while logged out saves pending destination, routes to /login, and opens NotificationDetailScreen after login (then clears pending destination)',
      (tester) async {
        debugDefaultTargetPlatformOverride = TargetPlatform.android;
        try {
          final authRepo = _ControllableAuthRepository(
            apiClient: apiClient,
            preferencesStorage: fakePrefs,
            secureAuthStorage: fakeSecure,
            initialUser: null, // Starts unauthenticated
          );

          await tester.pumpWidget(
            ProviderScope(
              overrides: [
                apiClientProvider.overrideWithValue(apiClient),
                preferencesStorageProvider.overrideWithValue(fakePrefs),
                secureAuthStorageProvider.overrideWithValue(fakeSecure),
                authRepositoryProvider.overrideWith((ref) => authRepo),
                pushNotificationServiceProvider.overrideWithValue(pushService),
                notificationRepositoryProvider.overrideWithValue(fakeNotifRepo),
              ],
              child: const App(),
            ),
          );
          await tester.pumpAndSettle();

          // Simulate user tapping a system notification while logged out
          pushService.simulateMessageOpenedApp(
            const RemoteMessage(
              messageId: 'msg-tap-unauth',
              notification: RemoteNotification(
                title: 'Mobil Sampah Tiba di RT 01',
                body: 'Silakan keluarkan sampah rumah tangga Anda sekarang.',
              ),
              data: {
                'type': 'waste_truck_arrival',
                'notification_id': 'notif-pending-1',
                'deep_link': '/notifications',
                'image_url': '/api/v1/notifications/images/rt-01/truck.png',
              },
            ),
          );
          await tester.pumpAndSettle();

          // Still on LoginScreen while unauthenticated (never bypasses auth)
          expect(find.byType(NotificationDetailScreen), findsNothing);
          expect(find.text('Halaman Tidak Ditemukan'), findsNothing);

          // User completes login
          authRepo.completeLogin({
            'id': 'user-warga-1',
            'name': 'Budi Warga',
            'role': AppRole.warga,
            'system_role': 'warga',
            'rt_id': 'rt-01',
            'profile_status': 'ready',
          });
          await tester.pumpAndSettle();
          await tester.pump(const Duration(seconds: 3));

          // Automatically navigated to NotificationDetailScreen!
          expect(find.byType(NotificationDetailScreen), findsOneWidget);
          expect(find.text('Mobil Sampah Tiba di RT 01'), findsOneWidget);
          expect(
            find.text('Silakan keluarkan sampah rumah tangga Anda sekarang.'),
            findsOneWidget,
          );
          expect(find.text('Halaman Tidak Ditemukan'), findsNothing);
        } finally {
          debugDefaultTargetPlatformOverride = null;
        }
      },
    );

    testWidgets(
      'Back Navigation Case A: Detail opened via external notification tap (background/foreground/terminated) -> both in-app Back and Android Back navigate to Beranda (/home), never to NotificationManagementScreen',
      (tester) async {
        debugDefaultTargetPlatformOverride = TargetPlatform.android;
        try {
          // Use a Super Admin user who HAS permission to access /home/notifikasi
          // to prove that tapping an external notification STILL returns to
          // Beranda (/home) and never drops the user into Kelola Notifikasi.
          final authRepo = _ControllableAuthRepository(
            apiClient: apiClient,
            preferencesStorage: fakePrefs,
            secureAuthStorage: fakeSecure,
            initialUser: {
              'id': 'user-admin-1',
              'name': 'Super Admin',
              'role': AppRole.pengurus,
              'system_role': 'super_admin',
              'jabatan': 'ketua',
              'rt_id': 'rt-01',
              'rt': '002',
              'profile_status': 'ready',
            },
          );

          await tester.pumpWidget(
            ProviderScope(
              overrides: [
                apiClientProvider.overrideWithValue(apiClient),
                preferencesStorageProvider.overrideWithValue(fakePrefs),
                secureAuthStorageProvider.overrideWithValue(fakeSecure),
                authRepositoryProvider.overrideWith((ref) => authRepo),
                pushNotificationServiceProvider.overrideWithValue(pushService),
                notificationRepositoryProvider.overrideWithValue(fakeNotifRepo),
              ],
              child: const App(),
            ),
          );
          await tester.pumpAndSettle();

          // 1. Tap external notification from background (onMessageOpenedApp)
          pushService.simulateMessageOpenedApp(
            const RemoteMessage(
              messageId: 'msg-ext-bg-1',
              notification: RemoteNotification(
                title: 'Mobil Sampah Tiba di RT 01',
                body: 'Silakan keluarkan sampah rumah tangga Anda sekarang.',
              ),
              data: {
                'type': 'waste_truck_arrival',
                'notification_id': 'notif-ext-bg-1',
                'deep_link': '/notifications',
              },
            ),
          );
          await tester.pumpAndSettle();
          await tester.pump(const Duration(seconds: 3));

          expect(find.byType(NotificationDetailScreen), findsOneWidget);
          expect(find.byType(NotificationManagementScreen), findsNothing);

          // Press in-app Back button -> must go to Beranda (DashboardScreen), NOT NotificationManagementScreen
          await tester.tap(
            find.byKey(const Key('notification_detail_back_button')),
          );
          await tester.pumpAndSettle();

          expect(find.byType(DashboardScreen), findsOneWidget);
          expect(find.byType(NotificationDetailScreen), findsNothing);
          expect(find.byType(NotificationManagementScreen), findsNothing);

          // 2. Tap external native heads-up notification (foreground banner tap via MethodChannel 'onNotificationTapped')
          const codec = StandardMethodCodec();
          final callData = codec.encodeMethodCall(
            const MethodCall('onNotificationTapped', <String, dynamic>{
              'type': 'manual_push',
              'notification_id': 'notif-ext-fg-2',
              'title': 'Kerja Bakti Minggu',
              'body': 'Berkumpul di balai warga.',
              'deep_link': '/home/notifikasi/detail/notif-ext-fg-2',
            }),
          );
          await tester.binding.defaultBinaryMessenger.handlePlatformMessage(
            'com.example.social_finance/notifications',
            callData,
            (_) {},
          );
          await tester.pumpAndSettle();
          await tester.pump(const Duration(seconds: 3));

          expect(find.byType(NotificationDetailScreen), findsOneWidget);

          // Press Android system Back button (handlePopRoute) -> must ALSO go to Beranda (DashboardScreen)
          final handled = await tester.binding.handlePopRoute();
          await tester.pumpAndSettle();

          expect(handled, isTrue);
          expect(find.byType(DashboardScreen), findsOneWidget);
          expect(find.byType(NotificationDetailScreen), findsNothing);
          expect(find.byType(NotificationManagementScreen), findsNothing);
        } finally {
          debugDefaultTargetPlatformOverride = null;
        }
      },
    );

    testWidgets(
      'Back Navigation Case B: Detail opened from in-app Notification screen (Riwayat tab) -> both in-app Back and Android Back return to NotificationManagementScreen preserving Riwayat tab state',
      (tester) async {
        debugDefaultTargetPlatformOverride = TargetPlatform.android;
        try {
          final authRepo = _ControllableAuthRepository(
            apiClient: apiClient,
            preferencesStorage: fakePrefs,
            secureAuthStorage: fakeSecure,
            initialUser: {
              'id': 'user-admin-1',
              'name': 'Ketua RT',
              'role': AppRole.pengurus,
              'system_role': 'pengurus',
              'jabatan': 'ketua',
              'rt_id': 'rt-01',
              'rt': '002',
              'profile_status': 'ready',
            },
          );

          await tester.pumpWidget(
            ProviderScope(
              overrides: [
                apiClientProvider.overrideWithValue(apiClient),
                preferencesStorageProvider.overrideWithValue(fakePrefs),
                secureAuthStorageProvider.overrideWithValue(fakeSecure),
                authRepositoryProvider.overrideWith((ref) => authRepo),
                pushNotificationServiceProvider.overrideWithValue(pushService),
                notificationRepositoryProvider.overrideWithValue(fakeNotifRepo),
              ],
              child: const App(),
            ),
          );
          await tester.pumpAndSettle();

          // Navigate to Profile tab -> open "Kelola Notifikasi"
          await tester.tap(find.byKey(const Key('nav-item-profil')));
          await tester.pumpAndSettle();

          await tester.scrollUntilVisible(
            find.text('Kelola Notifikasi'),
            200,
            scrollable: find.byType(Scrollable).first,
          );
          await tester.tap(find.text('Kelola Notifikasi'));
          await tester.pumpAndSettle();

          expect(find.byType(NotificationManagementScreen), findsOneWidget);

          // Switch to "Riwayat" tab inside NotificationManagementScreen
          await tester.tap(find.text('Riwayat'));
          await tester.pumpAndSettle();

          expect(
            find.byKey(const Key('notification_log_item_log-in-app-1')),
            findsOneWidget,
          );

          // 1. Tap the notification item in Riwayat -> opens NotificationDetailScreen
          await tester.tap(
            find.byKey(const Key('notification_log_item_log-in-app-1')),
          );
          await tester.pumpAndSettle();

          expect(find.byType(NotificationDetailScreen), findsOneWidget);

          // Press in-app Back button -> returns to NotificationManagementScreen on Riwayat tab!
          await tester.tap(
            find.byKey(const Key('notification_detail_back_button')),
          );
          await tester.pumpAndSettle();

          expect(find.byType(NotificationDetailScreen), findsNothing);
          expect(find.byType(NotificationManagementScreen), findsOneWidget);
          expect(
            find.byKey(const Key('notification_log_item_log-in-app-1')),
            findsOneWidget,
          );

          // 2. Tap the notification item again -> test Android system Back button (handlePopRoute)
          await tester.tap(
            find.byKey(const Key('notification_log_item_log-in-app-1')),
          );
          await tester.pumpAndSettle();

          expect(find.byType(NotificationDetailScreen), findsOneWidget);

          final handled = await tester.binding.handlePopRoute();
          await tester.pumpAndSettle();

          expect(handled, isTrue);
          expect(find.byType(NotificationDetailScreen), findsNothing);
          expect(find.byType(NotificationManagementScreen), findsOneWidget);
          expect(
            find.byKey(const Key('notification_log_item_log-in-app-1')),
            findsOneWidget,
          );
        } finally {
          debugDefaultTargetPlatformOverride = null;
        }
      },
    );

    testWidgets(
      'Terminated state launch via initial native notification payload opens NotificationDetailScreen and Back goes to Beranda',
      (tester) async {
        debugDefaultTargetPlatformOverride = TargetPlatform.android;
        try {
          TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
              .setMockMethodCallHandler(
            const MethodChannel('com.example.social_finance/notifications'),
            (call) async {
              nativeCalls.add(call);
              if (call.method == 'getInitialNotificationPayload') {
                return <String, dynamic>{
                  'type': 'waste_truck_arrival',
                  'notification_id': 'notif-term-1',
                  'title': 'Truk Sampah Pagi',
                  'body': 'Truk sampah sudah di depan gang.',
                  'deep_link': '/home/notifikasi/detail/notif-term-1',
                };
              }
              return null;
            },
          );

          final authRepo = _ControllableAuthRepository(
            apiClient: apiClient,
            preferencesStorage: fakePrefs,
            secureAuthStorage: fakeSecure,
            initialUser: {
              'id': 'user-warga-1',
              'name': 'Budi Warga',
              'role': AppRole.warga,
              'system_role': 'warga',
              'rt_id': 'rt-01',
              'profile_status': 'ready',
            },
          );

          await tester.pumpWidget(
            ProviderScope(
              overrides: [
                apiClientProvider.overrideWithValue(apiClient),
                preferencesStorageProvider.overrideWithValue(fakePrefs),
                secureAuthStorageProvider.overrideWithValue(fakeSecure),
                authRepositoryProvider.overrideWith((ref) => authRepo),
                pushNotificationServiceProvider.overrideWithValue(pushService),
                notificationRepositoryProvider.overrideWithValue(fakeNotifRepo),
              ],
              child: const App(),
            ),
          );
          await tester.pumpAndSettle();
          await tester.pump(const Duration(seconds: 3));

          expect(find.byType(NotificationDetailScreen), findsOneWidget);

          // Press Android system Back -> lands on Beranda (DashboardScreen)
          final handled = await tester.binding.handlePopRoute();
          await tester.pumpAndSettle();

          expect(handled, isTrue);
          expect(find.byType(DashboardScreen), findsOneWidget);
          expect(find.byType(NotificationManagementScreen), findsNothing);
        } finally {
          debugDefaultTargetPlatformOverride = null;
        }
      },
    );
  });
}
