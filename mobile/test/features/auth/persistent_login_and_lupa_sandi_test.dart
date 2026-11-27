import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/api/device_id_service.dart';
import 'package:social_finance/core/api/device_service.dart';
import 'package:social_finance/core/notification/push_notification_service.dart';
import 'package:social_finance/core/storage/app_storage.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/utils/jwt_utils.dart';
import 'package:social_finance/core/widgets/authenticated_network_image.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/auth/presentation/screens/login_screen.dart';

class _MockAdapter implements HttpClientAdapter {
  final Future<ResponseBody> Function(RequestOptions options) handler;

  _MockAdapter(this.handler);

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) => handler(options);

  @override
  void close({bool force = false}) {}
}

class _FakeDeviceIdService extends DeviceIdService {
  final String id;
  _FakeDeviceIdService([this.id = 'device-persistent-test-01']);

  @override
  Future<String> getDeviceId() async => id;
}

class _RecordingDeviceService implements DeviceService {
  final List<Map<String, dynamic>> registered = [];
  final List<Map<String, dynamic>> unregistered = [];

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);

  @override
  Future<void> registerDevice({
    required String fcmToken,
    String platform = 'android',
    String? deviceId,
  }) async {
    registered.add({
      'fcm_token': fcmToken,
      'platform': platform,
      'device_id': deviceId,
    });
  }

  @override
  Future<void> unregisterDevice(String fcmToken) async {
    unregistered.add({'fcm_token': fcmToken});
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

String _makeJwt({required DateTime exp, String sub = 'u-ketua-002'}) {
  final header = base64Url
      .encode(utf8.encode(jsonEncode({'alg': 'HS256', 'typ': 'JWT'})))
      .replaceAll('=', '');
  final payload = base64Url
      .encode(
        utf8.encode(
          jsonEncode({
            'sub': sub,
            'exp': exp.toUtc().millisecondsSinceEpoch ~/ 1000,
          }),
        ),
      )
      .replaceAll('=', '');
  return '$header.$payload.sig';
}

Map<String, dynamic> _ketuaMeJson() => {
  'id': 'u-ketua-002',
  'email': 'ketua-002@local.com',
  'full_name': 'Ketua RT 002',
  'role': 'pengurus',
  'jabatan': 'ketua',
  'rt_id': '43d1ed42-d982-4299-9b29-30080591fcf9',
  'rt_name': 'Urena',
  'rt_rt': '002',
  'rt_rw': 16,
  'effective_permissions': [
    'notification.manage',
    'notification.read',
    'kas.read',
    'iuran.read',
  ],
};

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('Part B — Persistent Login (Ingat Saya), Token Rotation & Security', () {
    test('JwtUtils accurately detects expired and near-expiry JWTs while ignoring opaque tokens', () {
      final now = DateTime.utc(2026, 11, 27, 12, 0);
      final expiredJwt = _makeJwt(exp: now.subtract(const Duration(minutes: 5)));
      final soonJwt = _makeJwt(exp: now.add(const Duration(seconds: 60)));
      final freshJwt = _makeJwt(exp: now.add(const Duration(minutes: 12)));

      expect(JwtUtils.isExpiredOrExpiringSoon(expiredJwt, now: now), isTrue);
      expect(JwtUtils.isExpiredOrExpiringSoon(soonJwt, now: now), isTrue);
      expect(JwtUtils.isExpiredOrExpiringSoon(freshJwt, now: now), isFalse);
      expect(JwtUtils.isExpiredOrExpiringSoon('opaque-test-token', now: now), isFalse);
      expect(JwtUtils.isExpiredOrExpiringSoon(null, now: now), isFalse);
    });

    test(
      'Login with Ingat Saya persists tokens (NEVER password), proactively refreshes expired JWT on cold startup & app resume',
      () async {
        const baseUrl = 'https://api.urena0216.id';
        final prefStorage = InMemoryPreferencesStorage(initialUrl: baseUrl);
        final secureStorage = InMemorySecureAuthStorage();

        var validAccess = _makeJwt(exp: DateTime.now().toUtc().add(const Duration(minutes: 15)));
        var validRefresh = 'ref-initial-0';
        var refreshCalls = 0;

        Future<ResponseBody> backend(RequestOptions options) async {
          if (options.path.contains('/auth/login')) {
            return _json({
              'access_token': validAccess,
              'refresh_token': validRefresh,
              'token_type': 'Bearer',
              'expires_in': 900,
              'user': _ketuaMeJson(),
            }, 200);
          }
          if (options.path.contains('/auth/refresh')) {
            refreshCalls++;
            final body = options.data is Map
                ? (options.data as Map).cast<String, dynamic>()
                : jsonDecode(options.data.toString()) as Map<String, dynamic>;
            if (body['refresh_token'] != validRefresh) {
              return _json({'error': {'code': 'invalid_token', 'message': 'revoked'}}, 401);
            }
            validAccess = _makeJwt(exp: DateTime.now().toUtc().add(const Duration(minutes: 15)));
            validRefresh = 'ref-rotated-$refreshCalls';
            return _json({
              'access_token': validAccess,
              'refresh_token': validRefresh,
              'token_type': 'Bearer',
              'expires_in': 900,
              'user': _ketuaMeJson(),
            }, 200);
          }
          if (options.path.contains('/auth/me')) {
            if (options.headers['Authorization'] != 'Bearer $validAccess') {
              return _json({'error': {'code': 'unauthorized', 'message': 'expired'}}, 401);
            }
            return _json(_ketuaMeJson(), 200);
          }
          return _json({'ok': true}, 200);
        }

        // 1. Login with rememberMe=true
        final c1 = ApiClient(baseUrl: baseUrl)..dio.httpClientAdapter = _MockAdapter(backend);
        final repo1 = AuthRepository(
          authService: AuthService(c1, _FakeDeviceIdService()),
          apiClient: c1,
          preferencesStorage: prefStorage,
          secureAuthStorage: secureStorage,
        );
        await repo1.login('ketua-002@local.com', 'SuperSecret123!', rememberMe: true);
        expect(repo1.currentUser, isNotNull);
        expect(await secureStorage.hasSession(), isTrue);
        // Security invariant: raw password MUST NEVER be persisted!
        expect(await secureStorage.getLoginCredentials(), isNull);

        // 2. Simulate app closed overnight: persisted JWT is now expired
        final expiredJwt = _makeJwt(exp: DateTime.now().toUtc().subtract(const Duration(hours: 3)));
        await secureStorage.saveSession(
          accessToken: expiredJwt,
          refreshToken: validRefresh,
          serverUrl: baseUrl,
          userData: _ketuaMeJson(),
        );

        final c2 = ApiClient(baseUrl: baseUrl)..dio.httpClientAdapter = _MockAdapter(backend);
        final repo2 = AuthRepository(
          authService: AuthService(c2, _FakeDeviceIdService()),
          apiClient: c2,
          preferencesStorage: prefStorage,
          secureAuthStorage: secureStorage,
        );
        final restored = await repo2.restoreSession();
        expect(restored, isTrue);
        expect(refreshCalls, equals(1));
        expect(c2.token, equals(validAccess));
        expect((await secureStorage.getSession())?.refreshToken, equals('ref-rotated-1'));

        // 3. Simulate app backgrounded for 20 minutes -> resume calls ensureFreshSession()
        c2.setToken(_makeJwt(exp: DateTime.now().toUtc().subtract(const Duration(minutes: 5))));
        await repo2.ensureFreshSession();
        expect(refreshCalls, equals(2));
        expect(c2.token, equals(validAccess));
        expect(repo2.currentUser?['access_token'], equals(validAccess));
        expect((await secureStorage.getSession())?.refreshToken, equals('ref-rotated-2'));
      },
    );

    test(
      'Concurrent 401 requests trigger exactly ONE single-flight /auth/refresh and all retry cleanly',
      () async {
        const baseUrl = 'https://api.urena0216.id';
        final prefStorage = InMemoryPreferencesStorage(
          initialUrl: baseUrl,
          initialRememberMe: true,
        );
        final secureStorage = InMemorySecureAuthStorage(
          initialSession: StoredSession(
            accessToken: 'expired-acc',
            refreshToken: 'ref-concurrent-1',
            serverUrl: baseUrl,
            userData: _ketuaMeJson(),
          ),
        );

        var refreshCalls = 0;
        var currentAccess = 'fresh-acc-after-single-flight';

        final client = ApiClient(baseUrl: baseUrl);
        client.setToken('expired-acc');
        client.setRefreshToken('ref-concurrent-1');
        client.dio.httpClientAdapter = _MockAdapter((options) async {
          if (options.path.contains('/auth/refresh')) {
            refreshCalls++;
            await Future<void>.delayed(const Duration(milliseconds: 30));
            return _json({
              'access_token': currentAccess,
              'refresh_token': 'ref-concurrent-2',
              'token_type': 'Bearer',
              'expires_in': 900,
              'user': _ketuaMeJson(),
            }, 200);
          }
          if (options.headers['Authorization'] != 'Bearer $currentAccess') {
            return _json({'error': {'code': 'unauthorized'}}, 401);
          }
          if (options.path.contains('/auth/me')) {
            return _json(_ketuaMeJson(), 200);
          }
          return _json({'endpoint': options.path, 'ok': true}, 200);
        });

        final repo = AuthRepository(
          authService: AuthService(client, _FakeDeviceIdService()),
          apiClient: client,
          preferencesStorage: prefStorage,
          secureAuthStorage: secureStorage,
        );
        expect(repo, isNotNull);

        final results = await Future.wait([
          client.dio.get('/api/v1/households'),
          client.dio.get('/api/v1/notifications/unread-count'),
          client.dio.get('/api/v1/bills'),
        ]);

        expect(refreshCalls, equals(1));
        for (final res in results) {
          expect(res.statusCode, equals(200));
        }
        expect((await secureStorage.getSession())?.refreshToken, equals('ref-concurrent-2'));
      },
    );

    testWidgets(
      'Revoked/expired refresh token clears credentials, avoids retry loop, and shows informative message on LoginScreen',
      (tester) async {
        const baseUrl = 'https://api.urena0216.id';
        final prefStorage = InMemoryPreferencesStorage(
          initialUrl: baseUrl,
          initialRememberMe: true,
        );
        final secureStorage = InMemorySecureAuthStorage(
          initialSession: StoredSession(
            accessToken: 'expired-acc',
            refreshToken: 'revoked-ref',
            serverUrl: baseUrl,
            userData: _ketuaMeJson(),
          ),
        );

        var refreshAttempts = 0;
        final client = ApiClient(baseUrl: baseUrl);
        client.dio.httpClientAdapter = _MockAdapter((options) async {
          if (options.path.contains('/auth/refresh')) {
            refreshAttempts++;
            return _json({
              'error': {'code': 'invalid_token', 'message': 'refresh token revoked'},
            }, 401);
          }
          return _json({'error': {'code': 'unauthorized'}}, 401);
        });

        final repo = AuthRepository(
          authService: AuthService(client, _FakeDeviceIdService()),
          apiClient: client,
          preferencesStorage: prefStorage,
          secureAuthStorage: secureStorage,
        );

        final restored = await tester.runAsync(() => repo.restoreSession());
        expect(restored, isFalse);
        expect(refreshAttempts, equals(1)); // Zero infinite retry loops
        expect(client.token, isNull);
        expect(client.refreshToken, isNull);
        expect(await secureStorage.hasSession(), isFalse);
        expect(
          repo.sessionExpiredMessage,
          equals(AuthRepository.sessionExpiredNoticeMessage),
        );

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              preferencesStorageProvider.overrideWithValue(prefStorage),
              secureAuthStorageProvider.overrideWithValue(secureStorage),
              authRepositoryProvider.overrideWith((ref) => repo),
            ],
            child: const MaterialApp(home: LoginScreen()),
          ),
        );
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 200));

        expect(
          find.text(AuthRepository.sessionExpiredNoticeMessage),
          findsOneWidget,
        );
      },
    );

    test(
      'Manual logout sends scoped refresh_token + device_id with Authorization header, clears local storage, and does NOT set session expired notice',
      () async {
        const baseUrl = 'https://api.urena0216.id';
        final prefStorage = InMemoryPreferencesStorage(
          initialUrl: baseUrl,
          initialRememberMe: true,
        );
        final secureStorage = InMemorySecureAuthStorage();

        String? capturedLogoutAuth;
        Map<String, dynamic>? capturedLogoutBody;

        final client = ApiClient(baseUrl: baseUrl);
        client.dio.httpClientAdapter = _MockAdapter((options) async {
          if (options.path.contains('/auth/login')) {
            return _json({
              'access_token': 'acc-logout-test',
              'refresh_token': 'ref-logout-test',
              'token_type': 'Bearer',
              'expires_in': 900,
              'user': _ketuaMeJson(),
            }, 200);
          }
          if (options.path.contains('/auth/me')) {
            return _json(_ketuaMeJson(), 200);
          }
          if (options.path.contains('/auth/logout')) {
            capturedLogoutAuth = options.headers['Authorization'] as String?;
            if (options.data is Map) {
              capturedLogoutBody = (options.data as Map).cast<String, dynamic>();
            }
            return _json({}, 204);
          }
          return _json({}, 404);
        });

        final repo = AuthRepository(
          authService: AuthService(client, _FakeDeviceIdService('android-dev-99')),
          apiClient: client,
          preferencesStorage: prefStorage,
          secureAuthStorage: secureStorage,
        );

        await repo.login('ketua-002@local.com', 'Password123!', rememberMe: true);
        await repo.logout();

        expect(capturedLogoutAuth, equals('Bearer acc-logout-test'));
        expect(capturedLogoutBody?['refresh_token'], equals('ref-logout-test'));
        expect(capturedLogoutBody?['device_id'], equals('android-dev-99'));
        expect(await secureStorage.hasSession(), isFalse);
        expect(await prefStorage.getRememberMe(), isFalse);
        expect(repo.sessionExpiredMessage, isNull);
      },
    );

    test(
      'Ingat Saya unchecked keeps session non-persistent across refresh and app restart',
      () async {
        const baseUrl = 'https://api.urena0216.id';
        final prefStorage = InMemoryPreferencesStorage(initialUrl: baseUrl);
        final secureStorage = InMemorySecureAuthStorage();

        var access = 'acc-ephemeral-1';
        final client = ApiClient(baseUrl: baseUrl);
        client.dio.httpClientAdapter = _MockAdapter((options) async {
          if (options.path.contains('/auth/login')) {
            return _json({
              'access_token': access,
              'refresh_token': 'ref-ephemeral-1',
              'token_type': 'Bearer',
              'expires_in': 900,
              'user': _ketuaMeJson(),
            }, 200);
          }
          if (options.path.contains('/auth/refresh')) {
            access = 'acc-ephemeral-2';
            return _json({
              'access_token': access,
              'refresh_token': 'ref-ephemeral-2',
              'token_type': 'Bearer',
              'expires_in': 900,
              'user': _ketuaMeJson(),
            }, 200);
          }
          if (options.headers['Authorization'] != 'Bearer $access') {
            return _json({'error': {'code': 'unauthorized'}}, 401);
          }
          if (options.path.contains('/auth/me')) {
            return _json(_ketuaMeJson(), 200);
          }
          return _json({'ok': true}, 200);
        });

        final repo = AuthRepository(
          authService: AuthService(client, _FakeDeviceIdService()),
          apiClient: client,
          preferencesStorage: prefStorage,
          secureAuthStorage: secureStorage,
        );

        await repo.login('ketua-002@local.com', 'Password123!', rememberMe: false);
        expect(await secureStorage.hasSession(), isFalse);

        // Even after in-memory token refresh, nothing is written to secure storage.
        access = 'expired-force-refresh';
        final res = await client.dio.get('/api/v1/households');
        expect(res.statusCode, equals(200));
        expect(await secureStorage.hasSession(), isFalse);

        // App restart returns to login.
        final repoAfterRestart = AuthRepository(
          authService: AuthService(ApiClient(baseUrl: baseUrl), _FakeDeviceIdService()),
          apiClient: ApiClient(baseUrl: baseUrl),
          preferencesStorage: prefStorage,
          secureAuthStorage: secureStorage,
        );
        expect(await repoAfterRestart.restoreSession(), isFalse);
      },
    );

    testWidgets(
      'Token refresh syncs new access_token to NativeNotificationAuthStore, keeps authenticated private image loading working, and manual logout clears native auth + storage',
      (tester) async {
        debugDefaultTargetPlatformOverride = TargetPlatform.android;
        try {
          const baseUrl = 'https://api.urena0216.id';
          final validPng = base64Decode(
            'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==',
          );
          final nativeCalls = <MethodCall>[];
          TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
              .setMockMethodCallHandler(
                const MethodChannel('com.example.social_finance/notifications'),
                (call) async {
                  nativeCalls.add(call);
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

          final expiredJwt = _makeJwt(
            exp: DateTime.now().toUtc().subtract(const Duration(minutes: 20)),
          );
          final refreshedJwt = _makeJwt(
            exp: DateTime.now().toUtc().add(const Duration(minutes: 15)),
          );
          var activeJwt = refreshedJwt;
          final imageAuthHeaders = <String?>[];

          final prefStorage = InMemoryPreferencesStorage(
            initialUrl: baseUrl,
            initialRememberMe: true,
          );
          final secureStorage = InMemorySecureAuthStorage(
            initialSession: StoredSession(
              accessToken: expiredJwt,
              refreshToken: 'ref-before-native-sync',
              serverUrl: baseUrl,
              userData: _ketuaMeJson(),
            ),
          );

          final client = ApiClient(baseUrl: baseUrl);
          client.dio.httpClientAdapter = _MockAdapter((options) async {
            if (options.path.contains('/auth/refresh')) {
              return _json({
                'access_token': activeJwt,
                'refresh_token': 'ref-after-native-sync',
                'token_type': 'Bearer',
                'expires_in': 900,
                'user': _ketuaMeJson(),
              }, 200);
            }
            if (options.path.contains('/auth/logout')) {
              return _json({}, 204);
            }
            if (options.path.contains('/notifications/images/')) {
              final auth = options.headers['Authorization'] as String?;
              imageAuthHeaders.add(auth);
              if (auth != 'Bearer $activeJwt') {
                return _json({'error': {'code': 'unauthorized'}}, 401);
              }
              return ResponseBody.fromBytes(
                validPng,
                200,
                headers: {
                  Headers.contentTypeHeader: ['image/png'],
                },
              );
            }
            if (options.headers['Authorization'] != 'Bearer $activeJwt') {
              return _json({'error': {'code': 'unauthorized'}}, 401);
            }
            if (options.path.contains('/auth/me')) {
              return _json(_ketuaMeJson(), 200);
            }
            return _json({'ok': true}, 200);
          });

          final authRepo = AuthRepository(
            authService: AuthService(client, _FakeDeviceIdService('android-sync-dev')),
            apiClient: client,
            preferencesStorage: prefStorage,
            secureAuthStorage: secureStorage,
          );
          final deviceSvc = _RecordingDeviceService();
          final pushSvc = PushNotificationService(deviceSvc, _FakeDeviceIdService('android-sync-dev'));
          addTearDown(pushSvc.dispose);

          // 1. Restore session with expired JWT -> triggers real refresh before mounting UI
          final restored = await tester.runAsync(() => authRepo.restoreSession());
          expect(restored, isTrue);
          expect(client.token, equals(refreshedJwt));

          // Wire a minimal listener replicating App's _authSubscription + onBeforeLogout
          final container = ProviderContainer(
            overrides: [
              apiClientProvider.overrideWithValue(client),
              preferencesStorageProvider.overrideWithValue(prefStorage),
              secureAuthStorageProvider.overrideWithValue(secureStorage),
              authRepositoryProvider.overrideWith((_) => authRepo),
              pushNotificationServiceProvider.overrideWithValue(pushSvc),
            ],
          );
          addTearDown(container.dispose);

          authRepo.onBeforeLogout = () async {
            await Future.wait([
              pushSvc.clearNativeNotificationAuth(),
              pushSvc.onUserLoggedOut(),
            ]);
          };
          final authSub = container.listen<AsyncValue<Map<String, dynamic>?>?>(
            authRepositoryProvider,
            (prev, next) {
              final activeUser = next?.valueOrNull;
              if (activeUser != null) {
                final activeRtId = activeUser['rt_id']?.toString().trim();
                pushSvc.syncNativeNotificationAuth(
                  accessToken: client.token,
                  rtId: activeRtId,
                  baseUrl: client.baseUrl,
                );
              } else {
                pushSvc.clearNativeNotificationAuth();
              }
            },
            fireImmediately: true,
          );
          addTearDown(authSub.close);
          await tester.runAsync(() => Future<void>.delayed(const Duration(milliseconds: 20)));

          final syncCallsAfterRestore = nativeCalls
              .where((c) => c.method == 'syncNotificationAuth')
              .toList();
          expect(syncCallsAfterRestore, isNotEmpty);
          expect(syncCallsAfterRestore.last.arguments['access_token'], equals(refreshedJwt));
          expect(
            syncCallsAfterRestore.last.arguments['rt_id'],
            equals('43d1ed42-d982-4299-9b29-30080591fcf9'),
          );
          expect(syncCallsAfterRestore.last.arguments['base_url'], equals(baseUrl));

          // 2. Simulate subsequent 401 rotation during active session -> NativeNotificationAuthStore updated again!
          final rotatedJwt2 = _makeJwt(
            exp: DateTime.now().toUtc().add(const Duration(minutes: 30)),
          );
          activeJwt = rotatedJwt2;
          final imgRes = await tester.runAsync(
            () => client.dio.get<List<int>>(
              '/api/v1/notifications/images/notifications/manual/rt-002/photo.png',
              options: Options(responseType: ResponseType.bytes),
            ),
          );
          expect(imgRes?.statusCode, equals(200));
          expect(imageAuthHeaders, contains('Bearer $rotatedJwt2'));
          await tester.runAsync(() => Future<void>.delayed(const Duration(milliseconds: 20)));

          final syncCallsAfter401 = nativeCalls
              .where((c) => c.method == 'syncNotificationAuth')
              .toList();
          expect(syncCallsAfter401.last.arguments['access_token'], equals(rotatedJwt2));

          // 3. Verify AuthenticatedNetworkImage renders private image with rotated token
          await tester.pumpWidget(
            UncontrolledProviderScope(
              container: container,
              child: const MaterialApp(
                home: Scaffold(
                  body: AuthenticatedNetworkImage(
                    imageUrl: '/api/v1/notifications/images/notifications/manual/rt-002/photo.png',
                    errorWidget: Text('ERR_IMG'),
                  ),
                ),
              ),
            ),
          );
          await tester.runAsync(() => Future<void>.delayed(const Duration(milliseconds: 50)));
          await tester.pump();
          await tester.pump(const Duration(milliseconds: 100));
          expect(find.byType(Image), findsOneWidget);
          expect(find.text('ERR_IMG'), findsNothing);

          // 4. Manual logout clears NativeNotificationAuthStore, clears secure storage,
          //    and never sets sessionExpiredMessage (even if a trailing 401 arrives).
          await tester.pumpWidget(const SizedBox.shrink());
          await tester.runAsync(() => authRepo.logout());
          await tester.pump(const Duration(seconds: 6));
          final clearCalls = nativeCalls
              .where((c) => c.method == 'clearNotificationAuth')
              .toList();
          expect(clearCalls, isNotEmpty);
          expect(await secureStorage.hasSession(), isFalse);
          expect(client.token, isNull);
          expect(client.refreshToken, isNull);
          expect(authRepo.sessionExpiredMessage, isNull);
        } finally {
          debugDefaultTargetPlatformOverride = null;
        }
      },
    );

    test(
      'NativeNotificationAuthStore hardening: rememberMe=false clears native auth on restoreSession(), transient refresh error preserves credentials, manual logout unregisters device with Bearer token, and repeated/unavailable cleanup is idempotent',
      () async {
        debugDefaultTargetPlatformOverride = TargetPlatform.android;
        try {
          const baseUrl = 'https://api.urena0216.id';
          final nativeCalls = <MethodCall>[];
          var throwMissingPlugin = false;

          TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
              .setMockMethodCallHandler(
                const MethodChannel('com.example.social_finance/notifications'),
                (call) async {
                  nativeCalls.add(call);
                  if (throwMissingPlugin) {
                    throw MissingPluginException('channel unavailable');
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

          // 1. rememberMe == false -> restoreSession() returns false AND clears native notification auth
          final prefFalse = InMemoryPreferencesStorage(
            initialUrl: baseUrl,
            initialRememberMe: false,
          );
          final secFalse = InMemorySecureAuthStorage();
          final clientFalse = ApiClient(baseUrl: baseUrl);
          final deviceSvcFalse = _RecordingDeviceService();
          final pushSvcFalse = PushNotificationService(
            deviceSvcFalse,
            _FakeDeviceIdService('dev-remember-false'),
          );
          addTearDown(pushSvcFalse.dispose);
          final repoFalse = AuthRepository(
            authService: AuthService(
              clientFalse,
              _FakeDeviceIdService('dev-remember-false'),
            ),
            apiClient: clientFalse,
            preferencesStorage: prefFalse,
            secureAuthStorage: secFalse,
          )..onClearNativeAuth = pushSvcFalse.clearNativeNotificationAuth;

          expect(await repoFalse.restoreSession(), isFalse);
          expect(
            nativeCalls.where((c) => c.method == 'clearNotificationAuth'),
            hasLength(1),
          );

          // 2. Repeated cleanup & unavailable MethodChannel are idempotent and never crash
          expect(await repoFalse.restoreSession(), isFalse);
          await pushSvcFalse.clearNativeNotificationAuth();
          throwMissingPlugin = true;
          expect(await repoFalse.restoreSession(), isFalse);
          await pushSvcFalse.clearNativeNotificationAuth();
          throwMissingPlugin = false;

          // 3. rememberMe == true + successful restoreSession() -> syncs newest token, NEVER clears native auth
          nativeCalls.clear();
          final expiredJwt = _makeJwt(
            exp: DateTime.now().toUtc().subtract(const Duration(minutes: 25)),
          );
          final refreshedJwt = _makeJwt(
            exp: DateTime.now().toUtc().add(const Duration(minutes: 15)),
          );
          final prefTrue = InMemoryPreferencesStorage(
            initialUrl: baseUrl,
            initialRememberMe: true,
          );
          final secTrue = InMemorySecureAuthStorage(
            initialSession: StoredSession(
              accessToken: expiredJwt,
              refreshToken: 'ref-ok-1',
              serverUrl: baseUrl,
              userData: _ketuaMeJson(),
            ),
          );
          final clientTrue = ApiClient(baseUrl: baseUrl);
          clientTrue.dio.httpClientAdapter = _MockAdapter((options) async {
            if (options.path.contains('/auth/refresh')) {
              return _json({
                'access_token': refreshedJwt,
                'refresh_token': 'ref-ok-2',
                'token_type': 'Bearer',
                'expires_in': 900,
                'user': _ketuaMeJson(),
              }, 200);
            }
            if (options.path.contains('/auth/me')) {
              return _json(_ketuaMeJson(), 200);
            }
            return _json({'ok': true}, 200);
          });
          final pushSvcTrue = PushNotificationService(
            _RecordingDeviceService(),
            _FakeDeviceIdService('dev-remember-true'),
          );
          addTearDown(pushSvcTrue.dispose);
          final repoTrue = AuthRepository(
            authService: AuthService(
              clientTrue,
              _FakeDeviceIdService('dev-remember-true'),
            ),
            apiClient: clientTrue,
            preferencesStorage: prefTrue,
            secureAuthStorage: secTrue,
          )..onClearNativeAuth = pushSvcTrue.clearNativeNotificationAuth;

          final containerTrue = ProviderContainer(
            overrides: [
              apiClientProvider.overrideWithValue(clientTrue),
              authRepositoryProvider.overrideWith((_) => repoTrue),
              pushNotificationServiceProvider.overrideWithValue(pushSvcTrue),
            ],
          );
          addTearDown(containerTrue.dispose);
          final subTrue = containerTrue.listen<AsyncValue<Map<String, dynamic>?>?>(
            authRepositoryProvider,
            (_, next) {
              final activeUser = next?.valueOrNull;
              if (activeUser != null) {
                pushSvcTrue.syncNativeNotificationAuth(
                  accessToken: clientTrue.token,
                  rtId: activeUser['rt_id']?.toString().trim(),
                  baseUrl: clientTrue.baseUrl,
                );
              }
            },
          );
          addTearDown(subTrue.close);

          expect(await repoTrue.restoreSession(), isTrue);
          await Future<void>.delayed(const Duration(milliseconds: 20));
          expect(
            nativeCalls.where((c) => c.method == 'clearNotificationAuth'),
            isEmpty,
          );
          final synced = nativeCalls
              .where((c) => c.method == 'syncNotificationAuth')
              .toList();
          expect(synced, isNotEmpty);
          expect(synced.last.arguments['access_token'], equals(refreshedJwt));

          // 4. Transient network error during refresh -> credentials & native auth are NOT cleared
          nativeCalls.clear();
          final secTransient = InMemorySecureAuthStorage(
            initialSession: StoredSession(
              accessToken: expiredJwt,
              refreshToken: 'ref-transient-keep',
              serverUrl: baseUrl,
              userData: _ketuaMeJson(),
            ),
          );
          final clientTransient = ApiClient(baseUrl: baseUrl);
          clientTransient.dio.httpClientAdapter = _MockAdapter((options) async {
            throw DioException(
              requestOptions: options,
              type: DioExceptionType.connectionError,
              error: 'offline',
            );
          });
          final repoTransient = AuthRepository(
            authService: AuthService(
              clientTransient,
              _FakeDeviceIdService('dev-transient'),
            ),
            apiClient: clientTransient,
            preferencesStorage: InMemoryPreferencesStorage(
              initialUrl: baseUrl,
              initialRememberMe: true,
            ),
            secureAuthStorage: secTransient,
          )..onClearNativeAuth = pushSvcTrue.clearNativeNotificationAuth;

          final restoredOffline = await repoTransient.restoreSession();
          expect(restoredOffline, isTrue);
          expect(
            nativeCalls.where((c) => c.method == 'clearNotificationAuth'),
            isEmpty,
          );
          expect(
            (await secTransient.getSession())?.refreshToken,
            equals('ref-transient-keep'),
          );
          expect(clientTransient.refreshToken, equals('ref-transient-keep'));

          // 5. Manual logout clears native auth AND sends authenticated DELETE /notifications/devices + POST /auth/logout
          nativeCalls.clear();
          String? deleteDeviceAuth;
          String? logoutAuth;
          final clientLogout = ApiClient(baseUrl: baseUrl);
          clientLogout.dio.httpClientAdapter = _MockAdapter((options) async {
            if (options.path.contains('/auth/login')) {
              return _json({
                'access_token': 'acc-manual-logout',
                'refresh_token': 'ref-manual-logout',
                'token_type': 'Bearer',
                'expires_in': 900,
                'user': _ketuaMeJson(),
              }, 200);
            }
            if (options.path.contains('/auth/me')) {
              return _json(_ketuaMeJson(), 200);
            }
            if (options.method == 'DELETE' &&
                options.path.contains('/notifications/devices')) {
              deleteDeviceAuth = options.headers['Authorization'] as String?;
              return _json({}, 204);
            }
            if (options.path.contains('/auth/logout')) {
              logoutAuth = options.headers['Authorization'] as String?;
              return _json({}, 204);
            }
            return _json({'ok': true}, 200);
          });
          final realDeviceSvc = DeviceService(clientLogout);
          final pushSvcLogout = PushNotificationService(
            realDeviceSvc,
            _FakeDeviceIdService('dev-manual-logout'),
          );
          addTearDown(pushSvcLogout.dispose);
          // Seed registered token so onUserLoggedOut executes DELETE /notifications/devices
          final secLogout = InMemorySecureAuthStorage();
          final repoLogout = AuthRepository(
            authService: AuthService(
              clientLogout,
              _FakeDeviceIdService('dev-manual-logout'),
            ),
            apiClient: clientLogout,
            preferencesStorage: InMemoryPreferencesStorage(
              initialUrl: baseUrl,
              initialRememberMe: true,
            ),
            secureAuthStorage: secLogout,
          );
          repoLogout.onBeforeLogout = () async {
            await Future.wait([
              pushSvcLogout.clearNativeNotificationAuth(),
              realDeviceSvc.unregisterDevice('fcm-tok-manual-logout'),
            ]);
          };
          repoLogout.onClearNativeAuth = pushSvcLogout.clearNativeNotificationAuth;

          await repoLogout.login(
            'ketua-002@local.com',
            'Password123!',
            rememberMe: true,
          );
          await repoLogout.logout();

          expect(deleteDeviceAuth, equals('Bearer acc-manual-logout'));
          expect(logoutAuth, equals('Bearer acc-manual-logout'));
          expect(
            nativeCalls.where((c) => c.method == 'clearNotificationAuth'),
            isNotEmpty,
          );
          expect(await secLogout.hasSession(), isFalse);
          expect(repoLogout.sessionExpiredMessage, isNull);
        } finally {
          debugDefaultTargetPlatformOverride = null;
        }
      },
    );
  });

  group('Part A (Mobile) — Same-Account Push Notification Not Suppressed', () {
    test('PushNotificationService emits foreground message even when sender_user_id matches logged-in user', () async {
      final deviceSvc = _RecordingDeviceService();
      final pushSvc = PushNotificationService(deviceSvc, _FakeDeviceIdService());
      addTearDown(pushSvc.dispose);

      final received = <RemoteMessage>[];
      final sub = pushSvc.onForegroundMessage.listen(received.add);
      addTearDown(sub.cancel);

      pushSvc.simulateForegroundMessage(
        const RemoteMessage(
          messageId: 'msg-same-account-001',
          data: {
            'type': 'manual_push',
            'notification_id': 'notif-same-account-001',
            'sender_user_id': 'u-ketua-002',
            'title': 'Info Rapat RT',
            'body': 'Dikirim dari Web oleh Ketua RT sendiri.',
          },
        ),
      );
      await Future<void>.delayed(Duration.zero);

      expect(received, hasLength(1));
      expect(received.first.data['sender_user_id'], equals('u-ketua-002'));
      expect(received.first.data['title'], equals('Info Rapat RT'));
    });
  });

  group('Part C — Lupa Kata Sandi? Without Underline (Android & iOS)', () {
    for (final platform in [TargetPlatform.android, TargetPlatform.iOS]) {
      testWidgets(
        'Lupa Kata Sandi? has NO underline and opens reset dialog on ${platform.name}',
        (tester) async {
          debugDefaultTargetPlatformOverride = platform;
          tester.view.physicalSize = const Size(412, 915);
          tester.view.devicePixelRatio = 1.0;
          addTearDown(() {
            tester.view.resetPhysicalSize();
            tester.view.resetDevicePixelRatio();
          });

          try {
            final prefStorage = InMemoryPreferencesStorage(
              initialUrl: 'https://api.urena0216.id',
            );
            final secureStorage = InMemorySecureAuthStorage();
            final client = ApiClient(baseUrl: 'https://api.urena0216.id');
            final repo = AuthRepository(
              authService: AuthService(client, _FakeDeviceIdService()),
              apiClient: client,
              preferencesStorage: prefStorage,
              secureAuthStorage: secureStorage,
            );

            await tester.pumpWidget(
              ProviderScope(
                overrides: [
                  preferencesStorageProvider.overrideWithValue(prefStorage),
                  secureAuthStorageProvider.overrideWithValue(secureStorage),
                  authRepositoryProvider.overrideWith((ref) => repo),
                ],
                child: const MaterialApp(home: LoginScreen()),
              ),
            );
            await tester.pump();
            await tester.pump(const Duration(milliseconds: 200));

            final lupaFinder = find.text('Lupa Kata Sandi?');
            expect(lupaFinder, findsOneWidget);

            final lupaText = tester.widget<Text>(lupaFinder);
            expect(lupaText.style?.decoration, isNot(equals(TextDecoration.underline)));
            expect(lupaText.style?.decoration, equals(TextDecoration.none));
            expect(lupaText.style?.color, equals(AppColors.accent));
            expect(lupaText.style?.fontSize, equals(12));

            // Tapping Lupa Kata Sandi? must still open the reset password dialog.
            await tester.ensureVisible(lupaFinder);
            await tester.pump(const Duration(milliseconds: 100));
            await tester.tap(lupaFinder);
            await tester.pump();
            await tester.pump(const Duration(milliseconds: 250));

            expect(find.text('Reset Kata Sandi Warga'), findsOneWidget);
          } finally {
            debugDefaultTargetPlatformOverride = null;
          }
        },
      );
    }
  });
}
