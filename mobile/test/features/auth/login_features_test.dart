import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/api/config.dart';
import 'package:social_finance/core/storage/app_storage.dart';
import 'package:social_finance/core/utils/phone_utils.dart';
import 'package:social_finance/core/utils/url_utils.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/auth/presentation/screens/login_screen.dart';

class MockAdapter implements HttpClientAdapter {
  final Future<ResponseBody> Function(RequestOptions options) handler;

  MockAdapter(this.handler);

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    return handler(options);
  }

  @override
  void close({bool force = false}) {}
}

class _FailingLoginCredentialStorage extends InMemorySecureAuthStorage {
  @override
  Future<void> saveLoginCredentials({
    required String identifier,
    required String loginMethod,
    required String password,
    required String serverUrl,
  }) async {
    throw StateError('secure storage unavailable');
  }
}

ResponseBody _jsonResponse(dynamic data, int statusCode) {
  return ResponseBody.fromString(
    jsonEncode(data),
    statusCode,
    headers: {
      Headers.contentTypeHeader: [Headers.jsonContentType],
    },
  );
}

void main() {
  group('1. URL Validation and Normalization', () {
    test('Valid HTTPS URL is accepted', () {
      expect(UrlUtils.validateUrl('https://api.urena0216.id'), isNull);
      expect(UrlUtils.validateUrl('https://example.com/api'), isNull);
    });

    test('Allowed local development HTTP hosts are accepted', () {
      expect(UrlUtils.validateUrl('http://localhost:8080'), isNull);
      expect(UrlUtils.validateUrl('http://127.0.0.1:8080'), isNull);
      expect(UrlUtils.validateUrl('http://10.0.2.2:8080'), isNull);
      expect(UrlUtils.validateUrl('http://192.168.1.50:8080'), isNull);
    });

    test('Production HTTP without HTTPS is rejected', () {
      final err = UrlUtils.validateUrl('http://api.urena0216.id');
      expect(err, contains('HTTPS'));
    });

    test('Empty or null URL is rejected', () {
      expect(UrlUtils.validateUrl(''), contains('tidak boleh kosong'));
      expect(UrlUtils.validateUrl('   '), contains('tidak boleh kosong'));
      expect(UrlUtils.validateUrl(null), contains('tidak boleh kosong'));
    });

    test('URL without scheme is rejected', () {
      expect(UrlUtils.validateUrl('api.urena0216.id'), contains('protokol'));
    });

    test('URL with credentials (username:password) is rejected', () {
      final err = UrlUtils.validateUrl('https://admin:secret@api.urena0216.id');
      expect(err, contains('username atau password'));
    });

    test('URL with query parameter is rejected', () {
      final err = UrlUtils.validateUrl('https://api.urena0216.id?token=secret');
      expect(err, contains('query parameter'));
    });

    test('URL with fragment is rejected', () {
      final err = UrlUtils.validateUrl('https://api.urena0216.id#login');
      expect(err, contains('fragmen'));
    });

    test(
      'URL normalization trims and removes trailing slashes and /api/v1',
      () {
        expect(
          UrlUtils.normalizeUrl('  https://api.urena0216.id/  '),
          equals('https://api.urena0216.id'),
        );
        expect(
          UrlUtils.normalizeUrl('https://api.urena0216.id/api/v1/'),
          equals('https://api.urena0216.id'),
        );
        expect(
          UrlUtils.normalizeUrl('https://api.urena0216.id/API/V1'),
          equals('https://api.urena0216.id'),
        );
      },
    );
  });

  group('2. Indonesian Phone Number Validation and Normalization', () {
    test('Local phone starting with 08 is accepted', () {
      expect(PhoneUtils.validatePhone('081298918220'), isNull);
      expect(PhoneUtils.validatePhone('0812-9891-8220'), isNull);
      expect(PhoneUtils.validatePhone('0812 9891 8220'), isNull);
    });

    test('International phone starting with +628 is accepted', () {
      expect(PhoneUtils.validatePhone('+6281298918220'), isNull);
      expect(PhoneUtils.validatePhone('6281298918220'), isNull);
    });

    test('Empty phone is rejected', () {
      expect(PhoneUtils.validatePhone(''), contains('wajib diisi'));
      expect(PhoneUtils.validatePhone(null), contains('wajib diisi'));
    });

    test('Non-numeric characters are rejected', () {
      expect(PhoneUtils.validatePhone('0812abc8220'), contains('angka'));
    });

    test('Fixed-line or invalid prefix is rejected', () {
      expect(
        PhoneUtils.validatePhone('0215551234'),
        contains('harus diawali 08'),
      );
      expect(
        PhoneUtils.validatePhone('07115551234'),
        contains('harus diawali 08'),
      );
    });

    test('Normalization converts 08 and 628 to canonical +628 format', () {
      expect(
        PhoneUtils.normalizePhone('081298918220'),
        equals('+6281298918220'),
      );
      expect(
        PhoneUtils.normalizePhone('6281298918220'),
        equals('+6281298918220'),
      );
      expect(
        PhoneUtils.normalizePhone('+6281298918220'),
        equals('+6281298918220'),
      );
      expect(
        PhoneUtils.normalizePhone('0812-9891-8220'),
        equals('+6281298918220'),
      );
    });
  });

  group('3. ApiClient Dynamic Base URL & Server Isolation', () {
    test(
      'ApiClient updates baseUrl dynamically and sends requests with it',
      () async {
        final client = ApiClient(baseUrl: 'https://server-a.local');
        expect(client.baseUrl, equals('https://server-a.local'));

        String? requestedBaseUrl;
        client.dio.httpClientAdapter = MockAdapter((options) async {
          requestedBaseUrl = options.baseUrl;
          return _jsonResponse({'ok': true}, 200);
        });

        await client.dio.get('/test');
        expect(requestedBaseUrl, equals('https://server-a.local'));

        client.setBaseUrl('https://server-b.local');
        expect(client.baseUrl, equals('https://server-b.local'));

        await client.dio.get('/test');
        expect(requestedBaseUrl, equals('https://server-b.local'));
      },
    );

    test(
      'resetServer clears tokens and prevents sending old server token',
      () async {
        final client = ApiClient(baseUrl: 'https://server-a.local');
        client.setToken('token-for-server-a');
        client.setRefreshToken('refresh-for-server-a');

        expect(client.token, equals('token-for-server-a'));

        // Switch to Server B
        client.resetServer('https://server-b.local');

        expect(client.baseUrl, equals('https://server-b.local'));
        expect(client.token, isNull);
        expect(client.refreshToken, isNull);

        String? sentAuthHeader;
        client.dio.httpClientAdapter = MockAdapter((options) async {
          sentAuthHeader = options.headers['Authorization'] as String?;
          return _jsonResponse({'ok': true}, 200);
        });

        await client.dio.get('/test');
        expect(sentAuthHeader, isNull);
      },
    );

    test('Unconfigured client throws ApiConfigException', () async {
      final unconfiguredClient = ApiClient(baseUrl: '');
      var requestMade = false;

      unconfiguredClient.dio.httpClientAdapter = MockAdapter((options) async {
        requestMade = true;
        return _jsonResponse({}, 200);
      });

      expect(
        () => unconfiguredClient.dio.get('/test'),
        throwsA(
          isA<DioException>().having(
            (e) => e.error,
            'error',
            isA<ApiConfigException>(),
          ),
        ),
      );
      expect(requestMade, isFalse);
    });
  });

  group('4. Phone and Email Login Network Contracts', () {
    test('Login with email sends {email, password}', () async {
      final client = ApiClient(baseUrl: 'https://api.urena0216.id');
      Map<String, dynamic>? receivedBody;

      client.dio.httpClientAdapter = MockAdapter((options) async {
        if (options.path.contains('/auth/login')) {
          receivedBody = options.data as Map<String, dynamic>?;
          return _jsonResponse({
            'access_token': 'tok-email-123',
            'refresh_token': 'ref-email-456',
            'token_type': 'Bearer',
            'expires_in': 900,
            'user': {
              'id': 'usr-1',
              'email': 'warga@local.com',
              'name': 'Warga Satu',
              'role': 'warga',
            },
          }, 200);
        }
        if (options.path.contains('/auth/me')) {
          return _jsonResponse({
            'id': 'usr-1',
            'email': 'warga@local.com',
            'name': 'Warga Satu',
            'role': 'warga',
          }, 200);
        }
        return _jsonResponse({}, 404);
      });

      final service = AuthService(client);
      final repo = AuthRepository(
        authService: service,
        apiClient: client,
        preferencesStorage: InMemoryPreferencesStorage(),
        secureAuthStorage: InMemorySecureAuthStorage(),
      );

      await repo.login('warga@local.com', 'pass123');

      expect(receivedBody, isNotNull);
      expect(receivedBody!['email'], equals('warga@local.com'));
      expect(receivedBody!['password'], equals('pass123'));
      expect(receivedBody!.containsKey('phone'), isFalse);
      expect(repo.currentUser?['email'], equals('warga@local.com'));
    });

    test(
      'Login with phone number sends {phone, password} with canonical phone',
      () async {
        final client = ApiClient(baseUrl: 'https://api.urena0216.id');
        Map<String, dynamic>? receivedBody;

        client.dio.httpClientAdapter = MockAdapter((options) async {
          if (options.path.contains('/auth/login')) {
            receivedBody = options.data as Map<String, dynamic>?;
            return _jsonResponse({
              'access_token': 'tok-phone-123',
              'refresh_token': 'ref-phone-456',
              'token_type': 'Bearer',
              'expires_in': 900,
              'user': {
                'id': 'usr-2',
                'email': 'phoneuser@local.com',
                'name': 'Budi Phone',
                'role': 'warga',
              },
            }, 200);
          }
          if (options.path.contains('/auth/me')) {
            return _jsonResponse({
              'id': 'usr-2',
              'email': 'phoneuser@local.com',
              'name': 'Budi Phone',
              'role': 'warga',
            }, 200);
          }
          return _jsonResponse({}, 404);
        });

        final service = AuthService(client);
        final repo = AuthRepository(
          authService: service,
          apiClient: client,
          preferencesStorage: InMemoryPreferencesStorage(),
          secureAuthStorage: InMemorySecureAuthStorage(),
        );

        // Pass local 081298918220
        await repo.login('', 'pass123', phone: '081298918220');

        expect(receivedBody, isNotNull);
        expect(receivedBody!['phone'], equals('+6281298918220'));
        expect(receivedBody!['password'], equals('pass123'));
        expect(receivedBody!.containsKey('email'), isFalse);
        expect(repo.currentUser?['name'], equals('Budi Phone'));
      },
    );
  });

  group('5. Remember Me and Session Restoration', () {
    test(
      'Remember Me active saves session to secure storage and restores on startup',
      () async {
        final client = ApiClient(baseUrl: 'https://api.urena0216.id');
        final secureStorage = InMemorySecureAuthStorage();
        final prefStorage = InMemoryPreferencesStorage(
          initialUrl: 'https://api.urena0216.id',
        );

        client.dio.httpClientAdapter = MockAdapter((options) async {
          if (options.path.contains('/auth/login')) {
            return _jsonResponse({
              'access_token': 'saved-acc-tok',
              'refresh_token': 'saved-ref-tok',
              'token_type': 'Bearer',
              'expires_in': 900,
              'user': {
                'id': 'u-restored',
                'email': 'restored@local.com',
                'name': 'Restored User',
                'role': 'pengurus',
              },
            }, 200);
          }
          if (options.path.contains('/auth/me')) {
            return _jsonResponse({
              'id': 'u-restored',
              'email': 'restored@local.com',
              'name': 'Restored User',
              'role': 'pengurus',
            }, 200);
          }
          return _jsonResponse({}, 404);
        });

        final repo = AuthRepository(
          authService: AuthService(client),
          apiClient: client,
          preferencesStorage: prefStorage,
          secureAuthStorage: secureStorage,
        );

        // Login with rememberMe = true
        await repo.login('restored@local.com', 'secret', rememberMe: true);

        // Verify stored in secure storage
        final stored = await secureStorage.getSession();
        expect(stored, isNotNull);
        expect(stored!.accessToken, equals('saved-acc-tok'));
        expect(stored.serverUrl, equals('https://api.urena0216.id'));
        final rememberedLogin = await secureStorage.getLoginCredentials();
        expect(rememberedLogin?.identifier, equals('restored@local.com'));
        expect(rememberedLogin?.loginMethod, equals('email'));
        expect(rememberedLogin?.password, equals('secret'));
        expect(rememberedLogin?.serverUrl, equals('https://api.urena0216.id'));

        // Create new fresh repository instance (simulating app restart)
        final newClient = ApiClient(baseUrl: 'https://api.urena0216.id');
        newClient.dio.httpClientAdapter = client.dio.httpClientAdapter;

        final restoredRepo = AuthRepository(
          authService: AuthService(newClient),
          apiClient: newClient,
          preferencesStorage: prefStorage,
          secureAuthStorage: secureStorage,
        );

        expect(restoredRepo.currentUser, isNull);

        final didRestore = await restoredRepo.restoreSession();
        expect(didRestore, isTrue);
        expect(restoredRepo.currentUser, isNotNull);
        expect(
          restoredRepo.currentUser?['email'],
          equals('restored@local.com'),
        );
        expect(newClient.token, equals('saved-acc-tok'));
      },
    );

    test(
      'Remember Me inactive does NOT persist session after logout or restart',
      () async {
        final client = ApiClient(baseUrl: 'https://api.urena0216.id');
        final secureStorage = InMemorySecureAuthStorage();
        final prefStorage = InMemoryPreferencesStorage(
          initialUrl: 'https://api.urena0216.id',
        );

        client.dio.httpClientAdapter = MockAdapter((options) async {
          if (options.path.contains('/auth/login') ||
              options.path.contains('/auth/me')) {
            return _jsonResponse({
              'access_token': 'temp-acc-tok',
              'refresh_token': 'temp-ref-tok',
              'token_type': 'Bearer',
              'expires_in': 900,
              'user': {
                'id': 'u-temp',
                'email': 'temp@local.com',
                'name': 'Temp User',
                'role': 'warga',
              },
            }, 200);
          }
          return _jsonResponse({}, 404);
        });

        final repo = AuthRepository(
          authService: AuthService(client),
          apiClient: client,
          preferencesStorage: prefStorage,
          secureAuthStorage: secureStorage,
        );

        // Login with rememberMe = false
        await repo.login('temp@local.com', 'secret', rememberMe: false);

        expect(repo.currentUser, isNotNull);
        // Secure storage must be empty!
        final stored = await secureStorage.getSession();
        expect(stored, isNull);
        expect(await secureStorage.getLoginCredentials(), isNull);

        // Simulating restart
        final newRepo = AuthRepository(
          authService: AuthService(client),
          apiClient: client,
          preferencesStorage: prefStorage,
          secureAuthStorage: secureStorage,
        );

        final didRestore = await newRepo.restoreSession();
        expect(didRestore, isFalse);
        expect(newRepo.currentUser, isNull);
      },
    );

    test(
      'Server change cleans old session and does not restore old server session',
      () async {
        final client = ApiClient(baseUrl: 'https://server-1.local');
        final secureStorage = InMemorySecureAuthStorage();
        final prefStorage = InMemoryPreferencesStorage();

        await secureStorage.saveSession(
          accessToken: 'tok-server-1',
          refreshToken: 'ref-server-1',
          serverUrl: 'https://server-1.local',
          userData: {'name': 'User 1'},
        );

        // Set active server to server-2
        client.setBaseUrl('https://server-2.local');

        final repo = AuthRepository(
          authService: AuthService(client),
          apiClient: client,
          preferencesStorage: prefStorage,
          secureAuthStorage: secureStorage,
        );

        // restoreSession should detect mismatch between stored server-1 and active server-2
        final didRestore = await repo.restoreSession();
        expect(didRestore, isFalse);
        expect(await secureStorage.getSession(), isNull);
        expect(client.token, isNull);
      },
    );

    test(
      'Secure credential storage failure does not leave an active session',
      () async {
        final client = ApiClient(baseUrl: 'https://api.urena0216.id');
        client.dio.httpClientAdapter = MockAdapter((options) async {
          if (options.path.contains('/auth/login')) {
            return _jsonResponse({
              'access_token': 'temporary-token',
              'refresh_token': 'temporary-refresh',
              'token_type': 'Bearer',
              'expires_in': 900,
              'user': {
                'id': 'u-secure-storage-error',
                'email': 'safe@example.com',
                'name': 'Safe User',
                'role': 'warga',
              },
            }, 200);
          }
          if (options.path.contains('/auth/me')) {
            return _jsonResponse({
              'id': 'u-secure-storage-error',
              'email': 'safe@example.com',
              'name': 'Safe User',
              'role': 'warga',
            }, 200);
          }
          return _jsonResponse({}, 200);
        });
        final secureStorage = _FailingLoginCredentialStorage();
        final repo = AuthRepository(
          authService: AuthService(client),
          apiClient: client,
          preferencesStorage: InMemoryPreferencesStorage(),
          secureAuthStorage: secureStorage,
        );

        await repo.login(
          'safe@example.com',
          'never-log-this',
          serverUrl: 'https://api.urena0216.id',
        );

        expect(repo.state, isA<AsyncError>());
        expect(client.token, isNull);
        expect(await secureStorage.hasSession(), isFalse);
        expect(
          (repo.state!.error as dynamic).message,
          contains('tidak dapat disimpan dengan aman'),
        );
      },
    );
  });

  group('6. LoginScreen UI & Widgets', () {
    testWidgets(
      'Fresh install shows empty URL API, Email tab active, and Remember Me checked',
      (tester) async {
        final prefStorage = InMemoryPreferencesStorage(); // empty initial URL
        final secureStorage = InMemorySecureAuthStorage();
        final client = ApiClient();
        final repo = AuthRepository(
          authService: AuthService(client),
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
        await tester.pumpAndSettle();

        // Three text fields: Server URL, Email, Password
        expect(find.byType(TextField), findsNWidgets(3));

        // 1. First textfield (Server URL) is empty on fresh install
        final urlField = tester.widget<TextField>(find.byType(TextField).at(0));
        expect(urlField.controller?.text, isEmpty);
        expect(
          urlField.decoration?.hintText,
          equals('https://api.socialfinance.com'),
        );

        // 2. Email method tab active
        expect(find.text('Email'), findsWidgets);
        expect(find.text('Nomor HP'), findsOneWidget);

        // 3. Second textfield is Email
        final emailField = tester.widget<TextField>(
          find.byType(TextField).at(1),
        );
        expect(emailField.decoration?.hintText, equals('Masukkan email'));

        // 4. Third textfield is Password
        final passField = tester.widget<TextField>(
          find.byType(TextField).at(2),
        );
        expect(passField.decoration?.hintText, equals('Masukkan kata sandi'));
        expect(passField.obscureText, isTrue);

        // 5. Remember Me checkbox is checked by default
        final checkbox = tester.widget<Checkbox>(find.byType(Checkbox));
        expect(checkbox.value, isTrue);
        expect(find.text('Ingat saya'), findsOneWidget);

        // 6. Masuk button visible
        expect(find.text('Masuk'), findsOneWidget);
      },
    );

    testWidgets(
      'Toggling login method switches between Email and Nomor HP with 08... hint',
      (tester) async {
        final prefStorage = InMemoryPreferencesStorage();
        final secureStorage = InMemorySecureAuthStorage();
        final client = ApiClient();
        final repo = AuthRepository(
          authService: AuthService(client),
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
        await tester.pumpAndSettle();

        // Tap "Nomor HP" tab
        await tester.tap(find.text('Nomor HP'));
        await tester.pumpAndSettle();

        // Middle textfield is now Phone with hint 081200000001
        final phoneField = tester.widget<TextField>(
          find.byType(TextField).at(1),
        );
        expect(phoneField.decoration?.hintText, equals('081200000001'));
        expect(phoneField.keyboardType, equals(TextInputType.phone));

        // Enter local phone format 081200000001
        await tester.enterText(find.byType(TextField).at(1), '081200000001');
        expect(phoneField.controller?.text, equals('081200000001'));

        // Tap back to "Email"
        await tester.tap(find.text('Email').first);
        await tester.pumpAndSettle();

        final backToEmailField = tester.widget<TextField>(
          find.byType(TextField).at(1),
        );
        expect(backToEmailField.decoration?.hintText, equals('Masukkan email'));
      },
    );

    testWidgets('Password obscure toggle reveals and hides password', (
      tester,
    ) async {
      final prefStorage = InMemoryPreferencesStorage();
      final secureStorage = InMemorySecureAuthStorage();
      final client = ApiClient();
      final repo = AuthRepository(
        authService: AuthService(client),
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
      await tester.pumpAndSettle();

      final initialPassField = tester.widget<TextField>(
        find.byType(TextField).at(2),
      );
      expect(initialPassField.obscureText, isTrue);

      // Tap visibility icon
      await tester.tap(find.byIcon(Icons.visibility_off));
      await tester.pumpAndSettle();

      final revealedPassField = tester.widget<TextField>(
        find.byType(TextField).at(2),
      );
      expect(revealedPassField.obscureText, isFalse);

      // Tap again to hide
      await tester.tap(find.byIcon(Icons.visibility));
      await tester.pumpAndSettle();

      final hiddenPassField = tester.widget<TextField>(
        find.byType(TextField).at(2),
      );
      expect(hiddenPassField.obscureText, isTrue);
    });

    testWidgets('Validation error is shown when URL API is empty', (
      tester,
    ) async {
      final prefStorage = InMemoryPreferencesStorage();
      final secureStorage = InMemorySecureAuthStorage();
      final client = ApiClient();
      final repo = AuthRepository(
        authService: AuthService(client),
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
      await tester.pumpAndSettle();

      // Enter email and password without server URL
      await tester.enterText(find.byType(TextField).at(1), 'user@example.com');
      await tester.enterText(find.byType(TextField).at(2), 'password');
      await tester.tap(find.text('Masuk'));
      await tester.pumpAndSettle();

      expect(find.text('URL API / Server wajib diisi.'), findsOneWidget);
    });

    testWidgets('Saved last API URL is remembered on startup', (tester) async {
      final prefStorage = InMemoryPreferencesStorage(
        initialUrl: 'https://api.urena0216.id',
      );
      final secureStorage = InMemorySecureAuthStorage();
      final client = ApiClient();
      final repo = AuthRepository(
        authService: AuthService(client),
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
      await tester.pumpAndSettle();

      final urlField = tester.widget<TextField>(find.byType(TextField).at(0));
      expect(urlField.controller?.text, equals('https://api.urena0216.id'));
    });

    testWidgets(
      'Corrupted or invalid URL in preferences displays warning on startup',
      (tester) async {
        final prefStorage = InMemoryPreferencesStorage(
          initialUrl: 'not-a-valid-url',
        );
        final secureStorage = InMemorySecureAuthStorage();
        final client = ApiClient();
        final repo = AuthRepository(
          authService: AuthService(client),
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
        await tester.pumpAndSettle();

        expect(
          find.textContaining('URL server tersimpan tidak valid'),
          findsOneWidget,
        );
      },
    );
  });

  group('7. Comprehensive Phone Number Edge Cases & Stored Lookup', () {
    test('Phone number with invalid symbols or punctuation is rejected', () {
      expect(PhoneUtils.validatePhone('0812#98918220'), contains('angka'));
      expect(PhoneUtils.validatePhone('0812@98918220'), contains('angka'));
      expect(PhoneUtils.validatePhone('+62812*98918220'), contains('angka'));
    });

    test('Phone number length boundaries for local format (10-14 digits)', () {
      // 9 digits (too short)
      expect(
        PhoneUtils.validatePhone('081234567'),
        contains('minimal 10 digit'),
      );
      // 10 digits (valid lower bound)
      expect(PhoneUtils.validatePhone('0812345678'), isNull);
      // 14 digits (valid upper bound)
      expect(PhoneUtils.validatePhone('08123456789012'), isNull);
      // 15 digits (too long)
      expect(
        PhoneUtils.validatePhone('081234567890123'),
        contains('maksimal 14 digit'),
      );
    });

    test('International phone length boundaries (+628, 12-16 characters)', () {
      // 11 chars (too short)
      expect(
        PhoneUtils.validatePhone('+6281234567'),
        contains('minimal 12 karakter'),
      );
      // 12 chars (valid lower bound)
      expect(PhoneUtils.validatePhone('+62812345678'), isNull);
      // 16 chars (valid upper bound)
      expect(PhoneUtils.validatePhone('+628123456789012'), isNull);
      // 17 chars (too long)
      expect(
        PhoneUtils.validatePhone('+6281234567890123'),
        contains('maksimal 16 karakter'),
      );
    });

    test(
      'restoreSession preloads active base URL from preferences even when logged out',
      () async {
        final prefStorage = InMemoryPreferencesStorage(
          initialUrl: 'https://api.urena0216.id',
        );
        final secureStorage = InMemorySecureAuthStorage(); // empty session
        final client = ApiClient(baseUrl: '');

        final repo = AuthRepository(
          authService: AuthService(client),
          apiClient: client,
          preferencesStorage: prefStorage,
          secureAuthStorage: secureStorage,
        );

        expect(client.baseUrl, isEmpty);
        final restored = await repo.restoreSession();
        expect(restored, isFalse);
        // Crucial: Base URL in ApiClient is now populated from preferences!
        expect(client.baseUrl, equals('https://api.urena0216.id'));
      },
    );
  });

  group('8. Network Errors, TLS, Timeout, and Corrupted Response Handling', () {
    test('Connection timeout reports readable error in Indonesian', () async {
      final client = ApiClient(baseUrl: 'https://api.urena0216.id');
      client.dio.httpClientAdapter = MockAdapter((options) async {
        throw DioException(
          requestOptions: options,
          type: DioExceptionType.connectionTimeout,
        );
      });

      final repo = AuthRepository(
        authService: AuthService(client),
        apiClient: client,
        preferencesStorage: InMemoryPreferencesStorage(),
        secureAuthStorage: InMemorySecureAuthStorage(),
      );

      await repo.login('warga@local.com', 'pass');
      expect(repo.state, isA<AsyncError>());
      final err = repo.state!.error as dynamic;
      expect(err.message, contains('batas waktu berakhir'));
    });

    test('Connection error (offline) reports readable error', () async {
      final client = ApiClient(baseUrl: 'https://api.urena0216.id');
      client.dio.httpClientAdapter = MockAdapter((options) async {
        throw DioException(
          requestOptions: options,
          type: DioExceptionType.connectionError,
        );
      });

      final repo = AuthRepository(
        authService: AuthService(client),
        apiClient: client,
        preferencesStorage: InMemoryPreferencesStorage(),
        secureAuthStorage: InMemorySecureAuthStorage(),
      );

      await repo.login('warga@local.com', 'pass');
      expect(repo.state, isA<AsyncError>());
      final err = repo.state!.error as dynamic;
      expect(err.message, contains('Gagal terhubung ke server'));
    });

    test('Bad TLS certificate reports security certificate error', () async {
      final client = ApiClient(baseUrl: 'https://api.urena0216.id');
      client.dio.httpClientAdapter = MockAdapter((options) async {
        throw DioException(
          requestOptions: options,
          type: DioExceptionType.badCertificate,
        );
      });

      final repo = AuthRepository(
        authService: AuthService(client),
        apiClient: client,
        preferencesStorage: InMemoryPreferencesStorage(),
        secureAuthStorage: InMemorySecureAuthStorage(),
      );

      await repo.login('warga@local.com', 'pass');
      expect(repo.state, isA<AsyncError>());
      final err = repo.state!.error as dynamic;
      expect(err.message, contains('Sertifikat keamanan server tidak valid'));
    });

    test(
      'Internal Server Error 500 reports server error with status code',
      () async {
        final client = ApiClient(baseUrl: 'https://api.urena0216.id');
        client.dio.httpClientAdapter = MockAdapter((options) async {
          return _jsonResponse({'error': 'crash'}, 500);
        });

        final repo = AuthRepository(
          authService: AuthService(client),
          apiClient: client,
          preferencesStorage: InMemoryPreferencesStorage(),
          secureAuthStorage: InMemorySecureAuthStorage(),
        );

        await repo.login('warga@local.com', 'pass');
        expect(repo.state, isA<AsyncError>());
        final err = repo.state!.error as dynamic;
        expect(err.message, contains('Terjadi kesalahan pada server (500)'));
      },
    );
  });

  group(
    '7. Login Retention & Security (Email, Phone, Logout, Password Protection)',
    () {
      testWidgets('Restores last email and sets Email tab active on startup', (
        tester,
      ) async {
        final prefStorage = InMemoryPreferencesStorage(
          initialUrl: 'https://api.socialfinance.com',
          initialEmail: 'warga01@test.com',
          initialLoginMethod: 'email',
        );
        final secureStorage = InMemorySecureAuthStorage(
          initialLoginCredentials: const StoredLoginCredentials(
            identifier: 'warga01@test.com',
            loginMethod: 'email',
            password: 'remembered-password',
            serverUrl: 'https://api.socialfinance.com',
          ),
        );
        final client = ApiClient(baseUrl: 'https://api.socialfinance.com');
        final repo = AuthRepository(
          authService: AuthService(client),
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
        await tester.pumpAndSettle();

        final emailField = tester.widget<TextField>(
          find.byType(TextField).at(1),
        );
        expect(emailField.controller?.text, equals('warga01@test.com'));

        final passField = tester.widget<TextField>(
          find.byType(TextField).at(2),
        );
        expect(passField.controller?.text, equals('remembered-password'));
      });

      testWidgets(
        'Restores last phone and sets Nomor HP tab active on startup',
        (tester) async {
          final prefStorage = InMemoryPreferencesStorage(
            initialUrl: 'https://api.socialfinance.com',
            initialPhone: '081200000001',
            initialLoginMethod: 'phone',
          );
          final secureStorage = InMemorySecureAuthStorage(
            initialLoginCredentials: const StoredLoginCredentials(
              identifier: '081200000001',
              loginMethod: 'phone',
              password: 'remembered-password',
              serverUrl: 'https://api.socialfinance.com',
            ),
          );
          final client = ApiClient(baseUrl: 'https://api.socialfinance.com');
          final repo = AuthRepository(
            authService: AuthService(client),
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
          await tester.pumpAndSettle();

          final phoneField = tester.widget<TextField>(
            find.byType(TextField).at(1),
          );
          expect(phoneField.controller?.text, equals('081200000001'));

          final passField = tester.widget<TextField>(
            find.byType(TextField).at(2),
          );
          expect(passField.controller?.text, equals('remembered-password'));
        },
      );

      testWidgets('Disabling Remember Me removes secure password prefill', (
        tester,
      ) async {
        final prefStorage = InMemoryPreferencesStorage(
          initialUrl: 'https://api.socialfinance.com',
        );
        final secureStorage = InMemorySecureAuthStorage(
          initialLoginCredentials: const StoredLoginCredentials(
            identifier: 'user@example.com',
            loginMethod: 'email',
            password: 'remembered-password',
            serverUrl: 'https://api.socialfinance.com',
          ),
        );
        final client = ApiClient();
        final repo = AuthRepository(
          authService: AuthService(client),
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
        await tester.pumpAndSettle();
        await tester.tap(find.byType(Checkbox));
        await tester.pumpAndSettle();

        expect(tester.widget<Checkbox>(find.byType(Checkbox)).value, isFalse);
        expect(
          tester
              .widget<TextField>(find.byType(TextField).at(2))
              .controller
              ?.text,
          isEmpty,
        );
        expect(await secureStorage.getLoginCredentials(), isNull);
        expect(await prefStorage.getRememberMe(), isFalse);
      });

      testWidgets('Changing account clears the remembered password', (
        tester,
      ) async {
        final prefStorage = InMemoryPreferencesStorage(
          initialUrl: 'https://api.socialfinance.com',
        );
        final secureStorage = InMemorySecureAuthStorage(
          initialLoginCredentials: const StoredLoginCredentials(
            identifier: 'user@example.com',
            loginMethod: 'email',
            password: 'remembered-password',
            serverUrl: 'https://api.socialfinance.com',
          ),
        );
        final client = ApiClient();
        final repo = AuthRepository(
          authService: AuthService(client),
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
        await tester.pumpAndSettle();
        await tester.enterText(
          find.byType(TextField).at(1),
          'another-user@example.com',
        );
        await tester.pumpAndSettle();

        expect(
          tester
              .widget<TextField>(find.byType(TextField).at(2))
              .controller
              ?.text,
          isEmpty,
        );
        expect(await secureStorage.getLoginCredentials(), isNull);
      });

      testWidgets('Remembered password is cleared when server does not match', (
        tester,
      ) async {
        final prefStorage = InMemoryPreferencesStorage(
          initialUrl: 'https://new-server.example',
          initialEmail: 'user@example.com',
          initialLoginMethod: 'email',
        );
        final secureStorage = InMemorySecureAuthStorage(
          initialLoginCredentials: const StoredLoginCredentials(
            identifier: 'user@example.com',
            loginMethod: 'email',
            password: 'remembered-password',
            serverUrl: 'https://old-server.example',
          ),
        );
        final client = ApiClient();
        final repo = AuthRepository(
          authService: AuthService(client),
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
        await tester.pumpAndSettle();

        expect(
          tester
              .widget<TextField>(find.byType(TextField).at(2))
              .controller
              ?.text,
          isEmpty,
        );
        expect(await secureStorage.getLoginCredentials(), isNull);
      });

      test(
        'Logout clears session tokens but preserves non-secret preferences',
        () async {
          final prefStorage = InMemoryPreferencesStorage(
            initialUrl: 'https://api.socialfinance.com',
            initialEmail: 'user@example.com',
            initialPhone: '081200000001',
            initialLoginMethod: 'email',
          );
          final secureStorage = InMemorySecureAuthStorage(
            initialSession: const StoredSession(
              accessToken: 'active-token-123',
              refreshToken: 'refresh-token-456',
              serverUrl: 'https://api.socialfinance.com',
              userData: {'name': 'Warga'},
            ),
            initialLoginCredentials: const StoredLoginCredentials(
              identifier: 'user@example.com',
              loginMethod: 'email',
              password: 'remembered-password',
              serverUrl: 'https://api.socialfinance.com',
            ),
          );
          final client = ApiClient(baseUrl: 'https://api.socialfinance.com');
          client.setToken('active-token-123');
          client.dio.httpClientAdapter = MockAdapter((options) async {
            if (options.path == '/api/v1/auth/logout') {
              return _jsonResponse({}, 200);
            }
            return _jsonResponse({}, 404);
          });

          final repo = AuthRepository(
            authService: AuthService(client),
            apiClient: client,
            preferencesStorage: prefStorage,
            secureAuthStorage: secureStorage,
          );

          // Verify tokens exist prior to logout
          expect(client.token, equals('active-token-123'));
          expect(await secureStorage.hasSession(), isTrue);

          await repo.logout();

          // Session tokens in memory and secure storage are purged
          expect(client.token, isNull);
          expect(await secureStorage.hasSession(), isFalse);
          expect(
            (await secureStorage.getLoginCredentials())?.password,
            equals('remembered-password'),
          );
          expect(repo.state, isNull);

          // Non-secret preferences are retained
          expect(
            await prefStorage.getLastApiUrl(),
            equals('https://api.socialfinance.com'),
          );
          expect(await prefStorage.getLastEmail(), equals('user@example.com'));
          expect(await prefStorage.getLastPhone(), equals('081200000001'));
          expect(await prefStorage.getLastLoginMethod(), equals('email'));
        },
      );

      test(
        'logout invokes onBeforeLogout while client token is still valid',
        () async {
          final prefStorage = InMemoryPreferencesStorage();
          final secureStorage = InMemorySecureAuthStorage(
            initialSession: StoredSession(
              accessToken: 'active-token-123',
              refreshToken: 'refresh-token-456',
              serverUrl: 'https://api.socialfinance.com',
              userData: {'name': 'Warga'},
            ),
          );
          final client = ApiClient(baseUrl: 'https://api.socialfinance.com');
          client.setToken('active-token-123');
          client.dio.httpClientAdapter = MockAdapter((options) async {
            if (options.path == '/api/v1/auth/logout') {
              return _jsonResponse({}, 200);
            }
            return _jsonResponse({}, 404);
          });

          final repo = AuthRepository(
            authService: AuthService(client),
            apiClient: client,
            preferencesStorage: prefStorage,
            secureAuthStorage: secureStorage,
          );

          String? capturedTokenDuringBeforeLogout;
          repo.onBeforeLogout = () async {
            capturedTokenDuringBeforeLogout = client.token;
          };

          await repo.logout();

          expect(capturedTokenDuringBeforeLogout, equals('active-token-123'));
          expect(client.token, isNull);
        },
      );
    },
  );
}
