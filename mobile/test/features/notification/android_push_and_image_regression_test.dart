import 'dart:convert';
import 'dart:io';

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
import 'package:social_finance/core/widgets/authenticated_network_image.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/notification/data/models/notification_models.dart';
import 'package:social_finance/features/notification/data/notification_api_client.dart';
import 'package:social_finance/features/notification/data/notification_repository.dart';
import 'package:social_finance/features/notification/presentation/screens/notification_detail_screen.dart';

final Uint8List _validPng = base64Decode(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==',
);

class _RecordingAdapter implements HttpClientAdapter {
  final Future<ResponseBody> Function(RequestOptions options, int callIndex)
      handler;
  final List<RequestOptions> requests = [];

  _RecordingAdapter(this.handler);

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) {
    requests.add(options);
    return handler(options, requests.length);
  }

  @override
  void close({bool force = false}) {}
}

class _FakeDeviceService implements DeviceService {
  final List<Map<String, dynamic>> registrations = [];
  final List<String> unregistrations = [];
  bool failNextRegister = false;

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);

  @override
  Future<void> registerDevice({
    required String fcmToken,
    String platform = 'android',
    String? deviceId,
  }) async {
    if (failNextRegister) {
      failNextRegister = false;
      throw Exception('transient network error');
    }
    registrations.add({
      'fcm_token': fcmToken,
      'platform': platform,
      'device_id': deviceId,
    });
  }

  @override
  Future<void> unregisterDevice(String fcmToken) async {
    unregistrations.add(fcmToken);
  }
}

class _FakePrefs implements PreferencesStorage {
  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
  @override
  Future<String?> getLastApiUrl() async => 'https://api.urena0216.id';
  @override
  Future<bool> getRememberMe() async => false;
}

class _FakeSecure implements SecureAuthStorage {
  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
  @override
  Future<StoredSession?> getSession() async => null;
  @override
  Future<void> clearSession() async {}
  @override
  Future<void> clearLoginCredentials() async {}
}

class _FakeAuthSvc implements AuthService {
  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _TestAuthRepo extends AuthRepository {
  _TestAuthRepo({
    required super.apiClient,
    Map<String, dynamic>? initialUser,
  }) : super(
          authService: _FakeAuthSvc(),
          preferencesStorage: _FakePrefs(),
          secureAuthStorage: _FakeSecure(),
        ) {
    if (initialUser != null) {
      state = AsyncData(initialUser);
    }
  }

  @override
  Future<bool> restoreSession() async => state?.valueOrNull != null;

  void setUser(Map<String, dynamic>? user) {
    state = user == null ? null : AsyncData(user);
  }
}

class _TestNotifRepo extends NotificationRepository {
  final NotificationLogEntry entry;

  _TestNotifRepo(this.entry)
      : super(apiClient: NotificationApiClient(dio: Dio()));

  @override
  Future<NotificationLogEntry> fetchNotificationDetail(String id) async => entry;
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('Pillar 2 — Notification Attachment Image Resolution & Authenticated Loading', () {
    tearDown(() {
      ApiConfig.setRuntimeBaseUrl('');
    });

    test('normalizes relative, bare storage keys, and rebases stale PUBLIC_BASE_URL onto active baseUrl', () {
      // Relative path against local HTTP dev server
      expect(
        ApiConfig.resolveImageUrl(
          '/api/v1/notifications/images/notifications/waste-truck/rt-002/truck.jpg',
          fallbackBaseUrl: 'http://192.168.0.41:8080',
        ),
        equals(
          'http://192.168.0.41:8080/api/v1/notifications/images/notifications/waste-truck/rt-002/truck.jpg',
        ),
      );

      // Bare storage key (notifications/...)
      expect(
        ApiConfig.resolveImageUrl(
          'notifications/manual/rt-002/banner.png',
          fallbackBaseUrl: 'https://api.urena0216.id',
        ),
        equals(
          'https://api.urena0216.id/api/v1/notifications/images/notifications/manual/rt-002/banner.png',
        ),
      );

      // Stale cleartext LAN IP from backend PUBLIC_BASE_URL rebased to active HTTPS server
      expect(
        ApiConfig.resolveImageUrl(
          'http://192.168.0.41:8080/api/v1/notifications/images/notifications/manual/rt-002/banner.png',
          fallbackBaseUrl: 'https://api.urena0216.id',
        ),
        equals(
          'https://api.urena0216.id/api/v1/notifications/images/notifications/manual/rt-002/banner.png',
        ),
      );

      // Traversal & unsafe schemes still blocked
      expect(
        ApiConfig.resolveImageUrl('notifications/../secrets/key.png', fallbackBaseUrl: 'https://api.urena0216.id'),
        isNull,
      );
      expect(ApiConfig.resolveImageUrl('file:///etc/passwd'), isNull);
    });

    testWidgets('AuthenticatedNetworkImage auto-recovers when token becomes available after initial failure', (tester) async {
      final adapter = _RecordingAdapter((options, _) async {
        final authHeader = options.headers['Authorization'] as String?;
        if (authHeader == null || authHeader.isEmpty) {
          return ResponseBody.fromString(
            '{"error":{"code":"unauthorized"}}',
            401,
            headers: {Headers.contentTypeHeader: ['application/json']},
          );
        }
        return ResponseBody.fromBytes(
          _validPng,
          200,
          headers: {Headers.contentTypeHeader: ['image/png']},
        );
      });
      final dio = Dio(BaseOptions(baseUrl: 'https://api.urena0216.id'))
        ..httpClientAdapter = adapter;
      final apiClient = ApiClient(customDio: dio, baseUrl: 'https://api.urena0216.id');
      final authRepo = _TestAuthRepo(apiClient: apiClient);

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            apiClientProvider.overrideWithValue(apiClient),
            authRepositoryProvider.overrideWith((_) => authRepo),
          ],
          child: const MaterialApp(
            home: Scaffold(
              body: AuthenticatedNetworkImage(
                imageUrl: '/api/v1/notifications/images/notifications/manual/rt-002/photo.png',
                errorWidget: Text('Gambar lampiran tidak dapat dimuat.'),
              ),
            ),
          ),
        ),
      );

      await tester.pumpAndSettle();
      // Initially failed because token was not yet restored
      expect(find.text('Gambar lampiran tidak dapat dimuat.'), findsOneWidget);

      // Session restores token + user RT -> widget auto-retries and renders image!
      apiClient.setToken('restored-jwt-token');
      authRepo.setUser({'id': 'u-1', 'rt_id': 'rt-002', 'role': 'warga'});
      await tester.pumpAndSettle();

      expect(find.byType(Image), findsOneWidget);
      expect(find.text('Gambar lampiran tidak dapat dimuat.'), findsNothing);
      expect(adapter.requests.last.headers['Authorization'], equals('Bearer restored-jwt-token'));
      expect(adapter.requests.last.headers['X-RT-ID'], equals('rt-002'));
    });

    testWidgets('AuthenticatedNetworkImage rejects non-image JSON 200 payload and recovers on manual retry', (tester) async {
      var attempt = 0;
      final adapter = _RecordingAdapter((options, _) async {
        attempt++;
        if (attempt == 1) {
          return ResponseBody.fromString(
            '{"error":"unexpected_proxy_page"}',
            200,
            headers: {Headers.contentTypeHeader: ['application/json']},
          );
        }
        return ResponseBody.fromBytes(
          _validPng,
          200,
          headers: {Headers.contentTypeHeader: ['image/png']},
        );
      });
      final dio = Dio(BaseOptions(baseUrl: 'https://api.urena0216.id'))
        ..httpClientAdapter = adapter;
      final apiClient = ApiClient(customDio: dio, baseUrl: 'https://api.urena0216.id')
        ..setToken('valid-jwt');
      final entry = NotificationLogEntry(
        id: 'notif-rt-1',
        rtId: 'rt-002',
        eventType: 'manual_push',
        title: 'Pengumuman RT',
        body: 'Lampiran foto kegiatan RT.',
        imageUrl: '/api/v1/notifications/images/notifications/manual/rt-002/kegiatan.png',
        variables: const {},
        recipientsCount: 10,
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            apiClientProvider.overrideWithValue(apiClient),
            notificationRepositoryProvider.overrideWithValue(_TestNotifRepo(entry)),
          ],
          child: const MaterialApp(
            home: NotificationDetailScreen(notificationId: 'notif-rt-1'),
          ),
        ),
      );

      await tester.pumpAndSettle();
      expect(find.text('Gambar lampiran tidak dapat dimuat.'), findsOneWidget);
      expect(find.byKey(const Key('notification_detail_image_retry_button')), findsOneWidget);

      // Tap retry button -> succeeds!
      await tester.tap(find.byKey(const Key('notification_detail_image_retry_button')));
      await tester.pumpAndSettle();

      expect(
        find.descendant(
          of: find.byType(AuthenticatedNetworkImage),
          matching: find.byType(Image),
        ),
        findsOneWidget,
      );
      expect(find.text('Gambar lampiran tidak dapat dimuat.'), findsNothing);
    });

    testWidgets('NotificationDetailScreen without image renders cleanly without image error box', (tester) async {
      final entry = NotificationLogEntry(
        id: 'notif-no-img',
        rtId: 'rt-002',
        eventType: 'manual_push',
        title: 'Pesan Teks Saja',
        body: 'Tidak ada lampiran gambar.',
        variables: const {},
        recipientsCount: 5,
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            notificationRepositoryProvider.overrideWithValue(_TestNotifRepo(entry)),
          ],
          child: const MaterialApp(
            home: NotificationDetailScreen(notificationId: 'notif-no-img'),
          ),
        ),
      );

      await tester.pumpAndSettle();
      expect(find.text('Pesan Teks Saja'), findsOneWidget);
      expect(find.text('Gambar lampiran tidak dapat dimuat.'), findsNothing);
      expect(find.byKey(const Key('notification_detail_image_fallback')), findsNothing);
    });
  });

  group('Pillar 3 — Android Heads-Up Push Notification Lifecycle & Deduplication', () {
    test('deduplicates foreground messages by messageId AND fallback notification_id', () async {
      final service = PushNotificationService(_FakeDeviceService());
      final received = <RemoteMessage>[];
      final sub = service.onForegroundMessage.listen(received.add);
      addTearDown(sub.cancel);

      // Duplicate with same messageId
      service.simulateForegroundMessage(
        const RemoteMessage(messageId: 'msg-1', data: {'title': 'A'}),
      );
      service.simulateForegroundMessage(
        const RemoteMessage(messageId: 'msg-1', data: {'title': 'A'}),
      );

      // Duplicate without messageId but same notification_id
      service.simulateForegroundMessage(
        const RemoteMessage(data: {'notification_id': 'nid-99', 'title': 'B'}),
      );
      service.simulateForegroundMessage(
        const RemoteMessage(data: {'notification_id': 'nid-99', 'title': 'B'}),
      );

      // Distinct notification_id passes through
      service.simulateForegroundMessage(
        const RemoteMessage(data: {'notification_id': 'nid-100', 'title': 'C'}),
      );

      await Future<void>.delayed(Duration.zero);
      expect(received.length, equals(3));
      expect(received.map((m) => m.data['title']).toList(), equals(['A', 'B', 'C']));
    });

    testWidgets('Android foreground push displays native heads-up without duplicate SnackBar, and falls back to SnackBar only on DISPLAY_FAILED', (tester) async {
      debugDefaultTargetPlatformOverride = TargetPlatform.android;

      try {
        final nativeCalls = <MethodCall>[];
        var nativeShouldFail = false;

        TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
            .setMockMethodCallHandler(
          const MethodChannel('com.example.social_finance/notifications'),
          (call) async {
            nativeCalls.add(call);
            if (call.method == 'showHeadsUpNotification') {
              if (nativeShouldFail) {
                return {'status': 'DISPLAY_FAILED', 'reason': 'permission_denied'};
              }
              return {
                'status': 'ANDROID_DISPLAY_REQUESTED',
                'id': call.arguments['id'],
                'channel_id': 'social_finance_notifications_v2',
              };
            }
            return null;
          },
        );
        addTearDown(() {
          TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
              .setMockMethodCallHandler(
            const MethodChannel('com.example.social_finance/notifications'),
            null,
          );
        });

        final apiClient = ApiClient(baseUrl: 'https://api.urena0216.id')
          ..setToken('jwt-active-user');
        final authRepo = _TestAuthRepo(
          apiClient: apiClient,
          initialUser: {
            'id': 'u-warga',
            'full_name': 'Warga RT 002',
            'email': 'warga@test.local',
            'role': AppRole.warga,
            'rt_id': 'rt-002',
          },
        );
        final pushService = PushNotificationService(_FakeDeviceService());

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              apiClientProvider.overrideWithValue(apiClient),
              authRepositoryProvider.overrideWith((_) => authRepo),
              pushNotificationServiceProvider.overrideWithValue(pushService),
            ],
            child: const App(),
          ),
        );
        await tester.pump();

        // Case 1: Native heads-up succeeds -> NO duplicate SnackBar
        pushService.simulateForegroundMessage(
          const RemoteMessage(
            messageId: 'fg-ok-1',
            notification: RemoteNotification(
              title: 'Truk Sampah Tiba',
              body: 'Silakan keluarkan sampah.',
            ),
            data: {
              'notification_id': 'notif-11',
              'type': 'waste_truck_arrival',
              'deep_link': '/home/notifikasi/detail/notif-11',
              'image_url': '/api/v1/notifications/images/notifications/waste-truck/rt-002/truck.jpg',
            },
          ),
        );
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 50));

        final headsUpCalls = nativeCalls
            .where((c) => c.method == 'showHeadsUpNotification')
            .toList();
        expect(headsUpCalls.length, equals(1));
        expect(headsUpCalls.first.arguments['title'], equals('Truk Sampah Tiba'));
        expect(headsUpCalls.first.arguments['auth_token'], equals('jwt-active-user'));
        expect(headsUpCalls.first.arguments['rt_id'], equals('rt-002'));
        expect(headsUpCalls.first.arguments['base_url'], equals('https://api.urena0216.id'));
        expect(
          headsUpCalls.first.arguments['image_url'],
          equals('https://api.urena0216.id/api/v1/notifications/images/notifications/waste-truck/rt-002/truck.jpg'),
        );
        expect(find.byType(SnackBar), findsNothing);

        // Case 2: Data-only FCM message (private image payload) ALSO triggers native heads-up cleanly
        pushService.simulateForegroundMessage(
          const RemoteMessage(
            messageId: 'fg-data-only-1',
            data: {
              'notification_id': 'notif-data-only-1',
              'title': 'Pengumuman Bergambar',
              'body': 'Foto lampiran kerja bakti RT.',
              'type': 'manual_push',
              'deep_link': '/home/notifikasi/detail/notif-data-only-1',
              'image_url': '/api/v1/notifications/images/notifications/manual/rt-002/kerja.jpg',
            },
          ),
        );
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 50));

        final dataOnlyCalls = nativeCalls
            .where((c) => c.method == 'showHeadsUpNotification')
            .toList();
        expect(dataOnlyCalls.length, equals(2));
        expect(dataOnlyCalls.last.arguments['title'], equals('Pengumuman Bergambar'));
        expect(
          dataOnlyCalls.last.arguments['image_url'],
          equals('https://api.urena0216.id/api/v1/notifications/images/notifications/manual/rt-002/kerja.jpg'),
        );
        expect(find.byType(SnackBar), findsNothing);

        // Case 3: Native heads-up returns DISPLAY_FAILED -> falls back gracefully to SnackBar
        nativeShouldFail = true;
        pushService.simulateForegroundMessage(
          const RemoteMessage(
            messageId: 'fg-fallback-2',
            notification: RemoteNotification(
              title: 'Info Penting RT',
              body: 'Rapat warga malam ini.',
            ),
            data: {
              'notification_id': 'notif-12',
              'type': 'manual_push',
            },
          ),
        );
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 50));

        expect(find.byType(SnackBar), findsOneWidget);
        expect(find.text('Info Penting RT'), findsOneWidget);
      } finally {
        debugDefaultTargetPlatformOverride = null;
      }
    });

    test('Android native notification channel v2, BigPictureStyle, Keystore auth store, and background FCM service are synchronized', () {
      final manifest = File('android/app/src/main/AndroidManifest.xml').readAsStringSync();
      final appKt = File('android/app/src/main/kotlin/com/example/social_finance/SocialFinanceApplication.kt').readAsStringSync();
      final mainActivityKt = File('android/app/src/main/kotlin/com/example/social_finance/MainActivity.kt').readAsStringSync();
      final msgServiceKt = File('android/app/src/main/kotlin/com/example/social_finance/SocialFinanceMessagingService.kt').readAsStringSync();
      final authStoreKt = File('android/app/src/main/kotlin/com/example/social_finance/NativeNotificationAuthStore.kt').readAsStringSync();
      final statIcon = File('android/app/src/main/res/drawable/ic_stat_notification.xml');

      expect(manifest, contains('android:name=".SocialFinanceApplication"'));
      expect(manifest, contains('social_finance_notifications_v2'));
      expect(manifest, contains('com.google.firebase.messaging.default_notification_icon'));
      expect(manifest, contains('android.permission.POST_NOTIFICATIONS'));
      expect(manifest, contains('android:name=".SocialFinanceMessagingService"'));
      expect(manifest, contains('com.google.firebase.MESSAGING_EVENT'));
      expect(appKt, contains('ensureNotificationChannel(this)'));
      expect(mainActivityKt, contains('const val CHANNEL_ID = "social_finance_notifications_v2"'));
      expect(mainActivityKt, contains('NotificationManager.IMPORTANCE_HIGH'));
      expect(mainActivityKt, contains('NotificationCompat.Builder'));
      expect(mainActivityKt, contains('NotificationCompat.BigPictureStyle'));
      expect(mainActivityKt, contains('.bigLargeIcon(null as Bitmap?)'));
      expect(mainActivityKt, contains('NotificationCompat.CATEGORY_MESSAGE'));
      expect(mainActivityKt, contains('decodeSafeNotificationBitmap'));
      expect(mainActivityKt, contains('ANDROID_DISPLAY_UPDATED_BIG_PICTURE'));
      expect(msgServiceKt, contains('class SocialFinanceMessagingService : FlutterFirebaseMessagingService()'));
      expect(msgServiceKt, contains('NativeNotificationAuthStore.read'));
      expect(authStoreKt, contains('AES/GCM/NoPadding'));
      expect(authStoreKt, contains('AndroidKeyStore'));
      expect(statIcon.existsSync(), isTrue);
    });
  });
}
