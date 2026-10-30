import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/app/app.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/api/config.dart';
import 'package:social_finance/core/api/device_id_service.dart';
import 'package:social_finance/core/api/device_service.dart';
import 'package:social_finance/core/notification/push_notification_service.dart';
import 'package:social_finance/core/storage/app_storage.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/notification/data/models/notification_models.dart';
import 'package:social_finance/features/notification/data/notification_api_client.dart';
import 'package:social_finance/features/notification/data/notification_repository.dart';
import 'package:social_finance/features/notification/presentation/screens/notification_detail_screen.dart';

class _MockAdapter implements HttpClientAdapter {
  final Future<ResponseBody> Function(RequestOptions options) handler;

  _MockAdapter(this.handler);

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) {
    return handler(options);
  }

  @override
  void close({bool force = false}) {}
}

class _FakeDeviceIdService extends DeviceIdService {
  @override
  Future<String> getDeviceId() async => 'device-test-uuid-001';
}

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

class _FakeNotificationRepository extends NotificationRepository {
  _FakeNotificationRepository()
    : super(apiClient: NotificationApiClient(dio: Dio()));

  @override
  Future<NotificationLogEntry> fetchNotificationDetail(String id) async {
    return NotificationLogEntry(
      id: id,
      rtId: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
      eventType: 'waste_truck_arrival',
      title: 'Mobil Sampah Tiba RT 002',
      body: 'Petugas sampah sudah di depan gang.',
      imageUrl: '/api/v1/notifications/images/rt-02/truck.png',
      variables: const {},
      recipientsCount: 12,
      createdAt: DateTime.utc(2026, 10, 29, 8, 0),
    );
  }
}

ResponseBody _json(dynamic data, int statusCode) {
  return ResponseBody.fromString(
    jsonEncode(data),
    statusCode,
    headers: {
      Headers.contentTypeHeader: [Headers.jsonContentType],
    },
  );
}

Map<String, dynamic> _sampleMeResponse() {
  return {
    'id': 'u-warga-01',
    'email': 'warga01@urena.local',
    'full_name': 'Pak RT Lansia',
    'role': 'warga',
    'rt_id': 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
    'rt_name': 'Urena',
    'rt_rt': '002',
    'rt_rw': 16,
    'effective_permissions': ['iuran.read', 'kas.read', 'warga.read'],
  };
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('Persistent Login, 30-Minute Idle & Token Rotation Recovery', () {
    test(
      '1. Survives 30+ minutes idle across multiple 15-minute access token expirations and process restarts (single-use refresh token rotation persisted immediately)',
      () async {
        const baseUrl = 'https://api.urena0216.id';
        final prefStorage = InMemoryPreferencesStorage(
          initialUrl: baseUrl,
          initialRememberMe: true,
        );
        final secureStorage = InMemorySecureAuthStorage();

        // Simulated backend state with strict single-use refresh token rotation:
        // Using a revoked refresh token immediately returns 401 invalid_token.
        var currentValidAccessToken = 'acc-t0';
        var currentValidRefreshToken = 'ref-t0';
        final revokedRefreshTokens = <String>{};
        var refreshCallCount = 0;

        Future<ResponseBody> backendHandler(RequestOptions options) async {
          if (options.path.contains('/auth/login')) {
            return _json({
              'access_token': currentValidAccessToken,
              'refresh_token': currentValidRefreshToken,
              'token_type': 'Bearer',
              'expires_in': 900,
              'user': {
                'id': 'u-warga-01',
                'email': 'warga01@urena.local',
                'full_name': 'Pak RT Lansia',
                'role': 'warga',
              },
            }, 200);
          }

          if (options.path.contains('/auth/refresh')) {
            refreshCallCount++;
            final body = options.data is Map
                ? (options.data as Map).cast<String, dynamic>()
                : jsonDecode(options.data.toString()) as Map<String, dynamic>;
            final presentedRefresh = body['refresh_token'] as String? ?? '';

            if (revokedRefreshTokens.contains(presentedRefresh) ||
                presentedRefresh != currentValidRefreshToken) {
              return _json({
                'error': {
                  'code': 'invalid_token',
                  'message': 'invalid or expired refresh token',
                },
              }, 401);
            }

            // Rotate single-use refresh token
            revokedRefreshTokens.add(presentedRefresh);
            currentValidAccessToken = 'acc-rotated-$refreshCallCount';
            currentValidRefreshToken = 'ref-rotated-$refreshCallCount';

            return _json({
              'access_token': currentValidAccessToken,
              'refresh_token': currentValidRefreshToken,
              'token_type': 'Bearer',
              'expires_in': 900,
              'user': {
                'id': 'u-warga-01',
                'email': 'warga01@urena.local',
                'full_name': 'Pak RT Lansia',
                'role': 'warga',
              },
            }, 200);
          }

          // Protected endpoints (/auth/me, /households, etc.)
          final authHeader = options.headers['Authorization'] as String? ?? '';
          if (authHeader != 'Bearer $currentValidAccessToken') {
            return _json({
              'error': {'code': 'unauthorized', 'message': 'token expired'},
            }, 401);
          }

          if (options.path.contains('/auth/me')) {
            return _json(_sampleMeResponse(), 200);
          }
          if (options.path.contains('/households')) {
            return _json({'data': []}, 200);
          }
          return _json({'ok': true}, 200);
        }

        // === T = 0 min: User logs in ===
        final client1 = ApiClient(baseUrl: baseUrl);
        client1.dio.httpClientAdapter = _MockAdapter(backendHandler);
        final repo1 = AuthRepository(
          authService: AuthService(client1, _FakeDeviceIdService()),
          apiClient: client1,
          preferencesStorage: prefStorage,
          secureAuthStorage: secureStorage,
        );

        await repo1.login('warga01@urena.local', 'Password123!', rememberMe: true);
        expect(repo1.currentUser, isNotNull);
        expect(client1.token, equals('acc-t0'));
        expect((await secureStorage.getSession())?.refreshToken, equals('ref-t0'));

        // === T = 15 min: Access token expires while app was in background & killed by Android ===
        currentValidAccessToken = 'acc-expired-on-server-at-15m';

        final client2 = ApiClient(baseUrl: baseUrl);
        client2.dio.httpClientAdapter = _MockAdapter(backendHandler);
        final repo2 = AuthRepository(
          authService: AuthService(client2, _FakeDeviceIdService()),
          apiClient: client2,
          preferencesStorage: prefStorage,
          secureAuthStorage: secureStorage,
        );

        final restoredAt15m = await repo2.restoreSession();
        expect(restoredAt15m, isTrue);
        expect(repo2.currentUser, isNotNull);
        expect(refreshCallCount, equals(1));
        expect(client2.token, equals('acc-rotated-1'));
        expect(client2.refreshToken, equals('ref-rotated-1'));

        // CRITICAL: Secure storage MUST now hold ref-rotated-1 (NOT revoked ref-t0!)
        final storedAfter15m = await secureStorage.getSession();
        expect(storedAfter15m?.accessToken, equals('acc-rotated-1'));
        expect(storedAfter15m?.refreshToken, equals('ref-rotated-1'));

        // === T = 30 min: User is idle for 30 minutes; second access token expires ===
        currentValidAccessToken = 'acc-expired-on-server-at-30m';

        // User taps menu in foreground (or app restarts again at 30m)
        final resAt30m = await client2.dio.get('/api/v1/households');
        expect(resAt30m.statusCode, equals(200));
        expect(refreshCallCount, equals(2));
        expect(repo2.currentUser, isNotNull);
        expect(client2.token, equals('acc-rotated-2'));
        expect((await secureStorage.getSession())?.refreshToken, equals('ref-rotated-2'));

        // === T = 45 min: Android kills process again; user reopens app ===
        currentValidAccessToken = 'acc-expired-on-server-at-45m';
        final client3 = ApiClient(baseUrl: baseUrl);
        client3.dio.httpClientAdapter = _MockAdapter(backendHandler);
        final repo3 = AuthRepository(
          authService: AuthService(client3, _FakeDeviceIdService()),
          apiClient: client3,
          preferencesStorage: prefStorage,
          secureAuthStorage: secureStorage,
        );

        final restoredAt45m = await repo3.restoreSession();
        expect(restoredAt45m, isTrue);
        expect(repo3.currentUser, isNotNull);
        expect(refreshCallCount, equals(3));
        expect(client3.token, equals('acc-rotated-3'));
        expect((await secureStorage.getSession())?.refreshToken, equals('ref-rotated-3'));
      },
    );

    test(
      '2. Concurrent 401 requests after 30-minute idle trigger only a single refresh call and all succeed without retry loops',
      () async {
        const baseUrl = 'https://api.urena0216.id';
        final prefStorage = InMemoryPreferencesStorage(initialUrl: baseUrl);
        final secureStorage = InMemorySecureAuthStorage(
          initialSession: StoredSession(
            accessToken: 'expired-acc',
            refreshToken: 'valid-ref-1',
            serverUrl: baseUrl,
            userData: _sampleMeResponse(),
          ),
        );

        var refreshCount = 0;
        var validAccessToken = 'fresh-acc-2';

        final client = ApiClient(baseUrl: baseUrl);
        client.setToken('expired-acc');
        client.setRefreshToken('valid-ref-1');

        client.dio.httpClientAdapter = _MockAdapter((options) async {
          if (options.path.contains('/auth/refresh')) {
            refreshCount++;
            await Future<void>.delayed(const Duration(milliseconds: 30));
            return _json({
              'access_token': validAccessToken,
              'refresh_token': 'valid-ref-2',
              'token_type': 'Bearer',
              'expires_in': 900,
              'user': {
                'id': 'u-warga-01',
                'email': 'warga01@urena.local',
                'full_name': 'Pak RT Lansia',
                'role': 'warga',
              },
            }, 200);
          }

          final auth = options.headers['Authorization'] as String? ?? '';
          if (auth != 'Bearer $validAccessToken') {
            return _json({'error': 'expired'}, 401);
          }
          if (options.path.contains('/auth/me')) {
            return _json(_sampleMeResponse(), 200);
          }
          return _json({'path': options.path, 'ok': true}, 200);
        });

        final repo = AuthRepository(
          authService: AuthService(client, _FakeDeviceIdService()),
          apiClient: client,
          preferencesStorage: prefStorage,
          secureAuthStorage: secureStorage,
        );

        // Fire 5 concurrent API requests with the expired access token
        final responses = await Future.wait([
          client.dio.get('/api/v1/households'),
          client.dio.get('/api/v1/bills'),
          client.dio.get('/api/v1/transactions'),
          client.dio.get('/api/v1/notifications/logs'),
          client.dio.get('/api/v1/residents'),
        ]);

        expect(responses.every((r) => r.statusCode == 200), isTrue);
        expect(refreshCount, equals(1));
        expect(client.token, equals('fresh-acc-2'));
        expect(client.refreshToken, equals('valid-ref-2'));
        expect((await secureStorage.getSession())?.refreshToken, equals('valid-ref-2'));
        expect(repo.state, isNull); // repo was not logged out
      },
    );

    test(
      '3. Temporary network failure during token refresh does NOT clear session or log user out, and succeeds on retry',
      () async {
        const baseUrl = 'https://api.urena0216.id';
        final prefStorage = InMemoryPreferencesStorage(initialUrl: baseUrl);
        final secureStorage = InMemorySecureAuthStorage();

        var currentValidToken = 'acc-initial';
        var currentRefreshToken = 'ref-initial';

        final client = ApiClient(baseUrl: baseUrl);
        client.dio.httpClientAdapter = _MockAdapter((options) async {
          if (options.path.contains('/auth/login')) {
            return _json({
              'access_token': currentValidToken,
              'refresh_token': currentRefreshToken,
              'token_type': 'Bearer',
              'expires_in': 900,
              'user': {
                'id': 'u-warga-01',
                'email': 'warga01@urena.local',
                'full_name': 'Pak RT Lansia',
                'role': 'warga',
              },
            }, 200);
          }

          if (options.path.contains('/auth/refresh')) {
            currentValidToken = 'acc-after-reconnect';
            currentRefreshToken = 'ref-after-reconnect';
            return _json({
              'access_token': currentValidToken,
              'refresh_token': currentRefreshToken,
              'token_type': 'Bearer',
              'expires_in': 900,
              'user': {
                'id': 'u-warga-01',
                'email': 'warga01@urena.local',
                'full_name': 'Pak RT Lansia',
                'role': 'warga',
              },
            }, 200);
          }

          final auth = options.headers['Authorization'] as String? ?? '';
          if (auth != 'Bearer $currentValidToken') {
            return _json({'error': 'token expired'}, 401);
          }

          if (options.path.contains('/auth/me')) {
            return _json(_sampleMeResponse(), 200);
          }
          return _json({'data': 'success'}, 200);
        });

        final repo = AuthRepository(
          authService: AuthService(client, _FakeDeviceIdService()),
          apiClient: client,
          preferencesStorage: prefStorage,
          secureAuthStorage: secureStorage,
        );

        await repo.login('warga01@urena.local', 'Password123!', rememberMe: true);
        expect(repo.currentUser, isNotNull);

        // Simulate 30 minutes idle: access token expired AND network drops right as refresh is attempted
        currentValidToken = 'acc-server-rotated';
        var refreshAttemptWhileOffline = false;
        client.dio.httpClientAdapter = _MockAdapter((options) async {
          if (options.path.contains('/auth/refresh') && !refreshAttemptWhileOffline) {
            refreshAttemptWhileOffline = true;
            throw DioException(
              requestOptions: options,
              type: DioExceptionType.connectionTimeout,
              message: 'Connection timed out while waking from idle',
            );
          }
          if (options.path.contains('/auth/refresh')) {
            currentValidToken = 'acc-recovered';
            currentRefreshToken = 'ref-recovered';
            return _json({
              'access_token': currentValidToken,
              'refresh_token': currentRefreshToken,
              'token_type': 'Bearer',
              'expires_in': 900,
              'user': {
                'id': 'u-warga-01',
                'email': 'warga01@urena.local',
                'full_name': 'Pak RT Lansia',
                'role': 'warga',
              },
            }, 200);
          }
          final auth = options.headers['Authorization'] as String? ?? '';
          if (auth != 'Bearer $currentValidToken') {
            return _json({'error': 'expired'}, 401);
          }
          if (options.path.contains('/auth/me')) {
            return _json(_sampleMeResponse(), 200);
          }
          return _json({'data': 'recovered-data'}, 200);
        });

        // First call fails with network timeout during refresh
        await expectLater(
          () => client.dio.get('/api/v1/households'),
          throwsA(isA<DioException>()),
        );

        // User MUST still be logged in and refresh token MUST still be preserved!
        expect(repo.currentUser, isNotNull);
        expect(client.refreshToken, equals('ref-initial'));
        expect((await secureStorage.getSession())?.refreshToken, equals('ref-initial'));

        // User taps Retry once network is back -> refreshes token and succeeds!
        final retryRes = await client.dio.get('/api/v1/households');
        expect(retryRes.statusCode, equals(200));
        expect(retryRes.data['data'], equals('recovered-data'));
        expect(repo.currentUser, isNotNull);
        expect(client.token, equals('acc-recovered'));
        expect((await secureStorage.getSession())?.refreshToken, equals('ref-recovered'));
      },
    );

    test(
      '4. Revoked or expired refresh token (401 on /auth/refresh) clears session and logs user out',
      () async {
        const baseUrl = 'https://api.urena0216.id';
        final prefStorage = InMemoryPreferencesStorage(initialUrl: baseUrl);
        final secureStorage = InMemorySecureAuthStorage();

        final client = ApiClient(baseUrl: baseUrl);
        client.dio.httpClientAdapter = _MockAdapter((options) async {
          if (options.path.contains('/auth/login')) {
            return _json({
              'access_token': 'acc-1',
              'refresh_token': 'ref-revoked',
              'token_type': 'Bearer',
              'expires_in': 900,
              'user': {
                'id': 'u-warga-01',
                'email': 'warga01@urena.local',
                'full_name': 'Pak RT Lansia',
                'role': 'warga',
              },
            }, 200);
          }
          if (options.path.contains('/auth/me') &&
              options.headers['Authorization'] == 'Bearer acc-1') {
            return _json(_sampleMeResponse(), 200);
          }
          if (options.path.contains('/auth/refresh')) {
            return _json({
              'error': {
                'code': 'invalid_token',
                'message': 'invalid or expired refresh token',
              },
            }, 401);
          }
          return _json({'error': 'unauthorized'}, 401);
        });

        final repo = AuthRepository(
          authService: AuthService(client, _FakeDeviceIdService()),
          apiClient: client,
          preferencesStorage: prefStorage,
          secureAuthStorage: secureStorage,
        );

        await repo.login('warga01@urena.local', 'Password123!', rememberMe: true);
        expect(repo.currentUser, isNotNull);

        // Now acc-1 expires and /auth/refresh returns 401 (revoked refresh token)
        await expectLater(
          () => client.dio.get('/api/v1/bills'),
          throwsA(isA<DioException>()),
        );

        expect(repo.currentUser, isNull);
        expect(repo.state, isNull);
        expect(client.token, isNull);
        expect(client.refreshToken, isNull);
        expect(await secureStorage.getSession(), isNull);
      },
    );

    testWidgets(
      '5. Notification tap when access token is expired (>30m idle) but refresh token is valid restores session via refresh and opens NotificationDetailScreen without login prompt',
      (tester) async {
        debugDefaultTargetPlatformOverride = TargetPlatform.android;
        try {
          const baseUrl = 'http://192.168.1.10:8085';
          final prefStorage = InMemoryPreferencesStorage(
            initialUrl: baseUrl,
            initialRememberMe: true,
          );
          final secureStorage = InMemorySecureAuthStorage(
            initialSession: StoredSession(
              accessToken: 'expired-acc-after-30m',
              refreshToken: 'valid-ref-after-30m',
              serverUrl: baseUrl,
              userData: _sampleMeResponse(),
            ),
          );

          final client = ApiClient(baseUrl: baseUrl);
          client.dio.httpClientAdapter = _MockAdapter((options) async {
            if (options.path.contains('/auth/refresh')) {
              return _json({
                'access_token': 'refreshed-acc-new',
                'refresh_token': 'refreshed-ref-new',
                'token_type': 'Bearer',
                'expires_in': 900,
                'user': {
                  'id': 'u-warga-01',
                  'email': 'warga01@urena.local',
                  'full_name': 'Pak RT Lansia',
                  'role': 'warga',
                },
              }, 200);
            }
            final auth = options.headers['Authorization'] as String? ?? '';
            if (auth != 'Bearer refreshed-acc-new') {
              return _json({'error': 'expired'}, 401);
            }
            if (options.path.contains('/auth/me')) {
              return _json(_sampleMeResponse(), 200);
            }
            return _json({'data': []}, 200);
          });

          final authRepo = AuthRepository(
            authService: AuthService(client, _FakeDeviceIdService()),
            apiClient: client,
            preferencesStorage: prefStorage,
            secureAuthStorage: secureStorage,
          );
          final pushService = PushNotificationService(_FakeDeviceService());
          final fakeNotifRepo = _FakeNotificationRepository();

          TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
              .setMockMethodCallHandler(
            const MethodChannel('com.example.social_finance/notifications'),
            (call) async {
              if (call.method == 'getInitialNotificationPayload') {
                return <String, dynamic>{
                  'type': 'waste_truck_arrival',
                  'notification_id': 'notif-restored-30m',
                  'deep_link': '/home/notifikasi/detail/notif-restored-30m',
                  'title': 'Mobil Sampah Tiba RT 002',
                  'body': 'Petugas sampah sudah di depan gang.',
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

          await tester.pumpWidget(
            ProviderScope(
              overrides: [
                apiClientProvider.overrideWithValue(client),
                preferencesStorageProvider.overrideWithValue(prefStorage),
                secureAuthStorageProvider.overrideWithValue(secureStorage),
                authRepositoryProvider.overrideWith((ref) => authRepo),
                pushNotificationServiceProvider.overrideWithValue(pushService),
                notificationRepositoryProvider.overrideWithValue(fakeNotifRepo),
              ],
              child: const App(),
            ),
          );

          await tester.pumpAndSettle();
          await tester.pump(const Duration(seconds: 3));

          // Session was automatically refreshed and NotificationDetailScreen opened directly!
          expect(authRepo.currentUser, isNotNull);
          expect(find.byType(NotificationDetailScreen), findsOneWidget);
          expect(find.text('Mobil Sampah Tiba RT 002'), findsOneWidget);
        } finally {
          debugDefaultTargetPlatformOverride = null;
          ApiConfig.setRuntimeBaseUrl('');
        }
      },
    );

    testWidgets(
      '6. Notification tap when refresh token fails permanently (401 revoked) saves pending destination, routes to /login, and opens NotificationDetailScreen after re-login',
      (tester) async {
        debugDefaultTargetPlatformOverride = TargetPlatform.android;
        try {
          const baseUrl = 'http://192.168.1.10:8085';
          final prefStorage = InMemoryPreferencesStorage(
            initialUrl: baseUrl,
            initialRememberMe: true,
          );
          final secureStorage = InMemorySecureAuthStorage(
            initialSession: StoredSession(
              accessToken: 'expired-acc',
              refreshToken: 'revoked-ref',
              serverUrl: baseUrl,
              userData: _sampleMeResponse(),
            ),
          );

          final client = ApiClient(baseUrl: baseUrl);
          client.dio.httpClientAdapter = _MockAdapter((options) async {
            if (options.path.contains('/auth/refresh')) {
              return _json({'error': 'revoked'}, 401);
            }
            if (options.path.contains('/auth/login')) {
              return _json({
                'access_token': 'fresh-login-acc',
                'refresh_token': 'fresh-login-ref',
                'token_type': 'Bearer',
                'expires_in': 900,
                'user': {
                  'id': 'u-warga-01',
                  'email': 'warga01@urena.local',
                  'full_name': 'Pak RT Lansia',
                  'role': 'warga',
                },
              }, 200);
            }
            final auth = options.headers['Authorization'] as String? ?? '';
            if (auth != 'Bearer fresh-login-acc') {
              return _json({'error': 'unauthorized'}, 401);
            }
            if (options.path.contains('/auth/me')) {
              return _json(_sampleMeResponse(), 200);
            }
            return _json({'data': []}, 200);
          });

          final authRepo = AuthRepository(
            authService: AuthService(client, _FakeDeviceIdService()),
            apiClient: client,
            preferencesStorage: prefStorage,
            secureAuthStorage: secureStorage,
          );
          final pushService = PushNotificationService(_FakeDeviceService());
          final fakeNotifRepo = _FakeNotificationRepository();

          TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
              .setMockMethodCallHandler(
            const MethodChannel('com.example.social_finance/notifications'),
            (call) async {
              if (call.method == 'getInitialNotificationPayload') {
                return <String, dynamic>{
                  'type': 'waste_truck_arrival',
                  'notification_id': 'notif-revoked-1',
                  'deep_link': '/home/notifikasi/detail/notif-revoked-1',
                  'title': 'Mobil Sampah Tiba RT 002',
                  'body': 'Petugas sampah sudah di depan gang.',
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

          await tester.pumpWidget(
            ProviderScope(
              overrides: [
                apiClientProvider.overrideWithValue(client),
                preferencesStorageProvider.overrideWithValue(prefStorage),
                secureAuthStorageProvider.overrideWithValue(secureStorage),
                authRepositoryProvider.overrideWith((ref) => authRepo),
                pushNotificationServiceProvider.overrideWithValue(pushService),
                notificationRepositoryProvider.overrideWithValue(fakeNotifRepo),
              ],
              child: const App(),
            ),
          );

          await tester.pumpAndSettle();
          await tester.pump(const Duration(seconds: 3));

          // Permanent refresh failure cleared session and routed to LoginScreen
          expect(authRepo.currentUser, isNull);
          expect(await secureStorage.getSession(), isNull);
          expect(find.byType(NotificationDetailScreen), findsNothing);

          // User logs back in -> automatically redirected to pending notification destination!
          final loginFuture = authRepo.login(
            'warga01@urena.local',
            'Password123!',
            rememberMe: true,
          );
          await tester.pumpAndSettle();
          await loginFuture;
          await tester.pumpAndSettle();
          await tester.pump(const Duration(seconds: 3));

          expect(authRepo.currentUser, isNotNull);
          expect(find.byType(NotificationDetailScreen), findsOneWidget);
          expect(find.text('Mobil Sampah Tiba RT 002'), findsOneWidget);
        } finally {
          debugDefaultTargetPlatformOverride = null;
          ApiConfig.setRuntimeBaseUrl('');
        }
      },
    );

    test(
      '7. Offline session restore marks cached profile as stale, preserves permissions, deduplicates concurrent reloadProfile calls, and updates to ready when connectivity returns',
      () async {
        const baseUrl = 'https://api.urena0216.id';
        final prefStorage = InMemoryPreferencesStorage(
          initialUrl: baseUrl,
          initialRememberMe: true,
        );
        final secureStorage = InMemorySecureAuthStorage(
          initialSession: StoredSession(
            accessToken: 'cached-acc-token',
            refreshToken: 'cached-ref-token',
            serverUrl: baseUrl,
            userData: _sampleMeResponse(),
          ),
        );

        var isOffline = true;
        var getMeCount = 0;

        final client = ApiClient(baseUrl: baseUrl);
        client.dio.httpClientAdapter = _MockAdapter((options) async {
          if (isOffline) {
            throw DioException(
              requestOptions: options,
              type: DioExceptionType.connectionError,
              message: 'No internet connection',
            );
          }
          if (options.path.contains('/auth/me')) {
            getMeCount++;
            await Future<void>.delayed(const Duration(milliseconds: 25));
            return _json({
              ..._sampleMeResponse(),
              'full_name': 'Pak RT Lansia (Tersinkronisasi)',
            }, 200);
          }
          return _json({'ok': true}, 200);
        });

        final repo = AuthRepository(
          authService: AuthService(client, _FakeDeviceIdService()),
          apiClient: client,
          preferencesStorage: prefStorage,
          secureAuthStorage: secureStorage,
        );

        // 1. Restore session while offline -> succeeds from cache, marked 'stale'
        final restored = await repo.restoreSession();
        expect(restored, isTrue);
        expect(repo.currentUser, isNotNull);
        expect(repo.currentUser?['profile_status'], equals('stale'));
        expect(
          repo.currentUser?['effective_permissions'],
          containsAll(['iuran.read', 'kas.read', 'warga.read']),
        );

        // 2. Reload attempt while still offline keeps 'stale' status and cached permissions
        await repo.reloadProfile();
        expect(repo.currentUser?['profile_status'], equals('stale'));
        expect(
          repo.currentUser?['effective_permissions'],
          containsAll(['iuran.read', 'kas.read', 'warga.read']),
        );

        // 3. Network returns -> concurrent reloadProfile() calls coalesce into 1 getMe()
        isOffline = false;
        await Future.wait([
          repo.reloadProfile(),
          repo.reloadProfile(),
          repo.reloadProfile(),
        ]);

        expect(getMeCount, equals(1));
        expect(repo.currentUser?['profile_status'], equals('ready'));
        expect(
          repo.currentUser?['name'],
          equals('Pak RT Lansia (Tersinkronisasi)'),
        );
      },
    );
  });
}
