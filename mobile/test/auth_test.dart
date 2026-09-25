import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/api/models.dart';
import 'package:social_finance/core/errors/app_errors.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';

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
  group('Auth Models Parsing', () {
    test('LoginResponse correctly parses all fields', () {
      final json = {
        'access_token': 'jwt-access-token-123',
        'refresh_token': 'jwt-refresh-token-456',
        'token_type': 'Bearer',
        'expires_in': 900,
        'user': {
          'id': 'user-uuid-1',
          'email': 'bendahara@example.com',
          'name': 'Budi Santoso',
          'role': 'bendahara',
          'rt_id': 'rt-uuid-001',
          'system_role': 'super_admin',
        },
      };

      final response = LoginResponse.fromJson(json);

      expect(response.accessToken, equals('jwt-access-token-123'));
      expect(response.refreshToken, equals('jwt-refresh-token-456'));
      expect(response.tokenType, equals('Bearer'));
      expect(response.expiresIn, equals(900));
      expect(response.user.id, equals('user-uuid-1'));
      expect(response.user.email, equals('bendahara@example.com'));
      expect(response.user.name, equals('Budi Santoso'));
      expect(response.user.role, equals('bendahara'));
      expect(response.user.rtId, equals('rt-uuid-001'));
      expect(response.user.systemRole, equals('super_admin'));
    });

    test('AuthUser correctly parses full_name and rt.id fallback', () {
      final json = {
        'id': 'user-uuid-2',
        'email': 'warga@example.com',
        'full_name': 'Siti Rahayu',
        'role': 'warga',
        'rt': {'id': 'rt-from-object', 'name': 'RT 002'},
      };

      final user = AuthUser.fromJson(json);

      expect(user.id, equals('user-uuid-2'));
      expect(user.email, equals('warga@example.com'));
      expect(user.name, equals('Siti Rahayu'));
      expect(user.role, equals('warga'));
      expect(user.rtId, equals('rt-from-object'));
      expect(user.systemRole, isNull);
    });

    test('AuthUser correctly handles missing optional fields', () {
      final json = {
        'id': 'user-3',
        'email': 'test@example.com',
        'name': 'Test',
        'role': 'pengurus',
      };

      final user = AuthUser.fromJson(json);

      expect(user.id, equals('user-3'));
      expect(user.rtId, isNull);
      expect(user.systemRole, isNull);
    });
  });

  group('Role Mapping', () {
    test('maps all supported roles correctly', () {
      expect(mapStringToAppRole('super_admin'), equals(AppRole.superAdmin));
      expect(mapStringToAppRole('pengurus'), equals(AppRole.pengurus));
      expect(mapStringToAppRole('bendahara'), equals(AppRole.bendahara));
      expect(mapStringToAppRole('warga'), equals(AppRole.warga));
    });

    test('case and whitespace insensitive mapping', () {
      expect(mapStringToAppRole('  SUPER_ADMIN  '), equals(AppRole.superAdmin));
      expect(mapStringToAppRole('Pengurus'), equals(AppRole.pengurus));
      expect(mapStringToAppRole('BENDAHARA'), equals(AppRole.bendahara));
      expect(mapStringToAppRole('Warga '), equals(AppRole.warga));
    });

    test(
      'unknown or null roles safely default to non-privileged AppRole.warga',
      () {
        expect(mapStringToAppRole('admin'), equals(AppRole.warga));
        expect(mapStringToAppRole('root'), equals(AppRole.warga));
        expect(mapStringToAppRole(''), equals(AppRole.warga));
        expect(mapStringToAppRole(null), equals(AppRole.warga));
        expect(mapStringToAppRole('unknown_role'), equals(AppRole.warga));
      },
    );
  });

  group('Auth Flow with Mocked Network', () {
    late ApiClient apiClient;
    late AuthService authService;
    late AuthRepository authRepository;

    setUp(() {
      apiClient = ApiClient(baseUrl: 'http://test-server.local');
      authService = AuthService(apiClient);
      authRepository = AuthRepository(
        authService: authService,
        apiClient: apiClient,
      );
    });

    test('Successful login sets token and populates auth state', () async {
      final mockData = {
        'access_token': 'test-access-token',
        'refresh_token': 'test-refresh-token',
        'token_type': 'Bearer',
        'expires_in': 900,
        'user': {
          'id': 'uuid-123',
          'email': 'andi@example.com',
          'name': 'Andi Wijaya',
          'role': 'bendahara',
          'rt_id': 'rt-uuid-456',
        },
      };

      apiClient.dio.httpClientAdapter = MockAdapter((options) async {
        expect(options.path, equals('/api/v1/auth/login'));
        expect(options.method, equals('POST'));
        expect(options.data['email'], equals('andi@example.com'));
        expect(options.data['password'], equals('secret123'));
        return _jsonResponse(mockData, 200);
      });

      await authRepository.login('andi@example.com', 'secret123');

      expect(apiClient.token, equals('test-access-token'));
      expect(authRepository.state, isA<AsyncData<Map<String, dynamic>?>>());

      final userMap = authRepository.state!.value!;
      expect(userMap['id'], equals('uuid-123'));
      expect(userMap['email'], equals('andi@example.com'));
      expect(userMap['name'], equals('Andi Wijaya'));
      expect(userMap['role'], equals(AppRole.bendahara));
      expect(userMap['rt_id'], equals('rt-uuid-456'));
      expect(
        userMap['rt'],
        isNull,
      ); // Ensures UUID is NOT displayed as RT number
      expect(userMap['rw'], isNull);
      expect(userMap['access_token'], equals('test-access-token'));
      expect(authRepository.currentRole, equals(AppRole.bendahara));
    });

    test(
      'Invalid credentials (HTTP 401) sets error state and retains no token',
      () async {
        apiClient.dio.httpClientAdapter = MockAdapter((options) async {
          return _jsonResponse({
            'error': {
              'code': 'unauthorized',
              'message': 'Email atau kata sandi salah.',
            },
          }, 401);
        });

        await authRepository.login('andi@example.com', 'wrongpassword');

        expect(apiClient.token, isNull);
        expect(authRepository.state, isA<AsyncError<Map<String, dynamic>?>>());
        final error = authRepository.state!.error;
        expect(error, isA<AuthError>());
        expect(
          (error as AuthError).message,
          equals('Email atau kata sandi salah.'),
        );
      },
    );

    test('Empty email or password rejects without network call', () async {
      var networkCalled = false;
      apiClient.dio.httpClientAdapter = MockAdapter((options) async {
        networkCalled = true;
        return _jsonResponse({}, 200);
      });

      await authRepository.login('', '');

      expect(networkCalled, isFalse);
      expect(authRepository.state, isA<AsyncError<Map<String, dynamic>?>>());
      expect(
        (authRepository.state!.error as AuthError).message,
        equals('Email dan kata sandi wajib diisi.'),
      );
    });

    test('Successful logout calls API and clears token and state', () async {
      var logoutCalled = false;
      apiClient.setToken('existing-token');
      authRepository.state = AsyncData({
        'name': 'Andi',
        'role': AppRole.bendahara,
      });

      apiClient.dio.httpClientAdapter = MockAdapter((options) async {
        if (options.path.contains('/auth/logout')) {
          logoutCalled = true;
          expect(
            options.headers['Authorization'],
            equals('Bearer existing-token'),
          );
          return _jsonResponse(null, 204);
        }
        return _jsonResponse(null, 404);
      });

      await authRepository.logout();

      expect(logoutCalled, isTrue);
      expect(apiClient.token, isNull);
      expect(authRepository.state, isNull);
      expect(authRepository.currentUser, isNull);
    });

    test(
      'Logout clears local state even when server returns 500 error',
      () async {
        apiClient.setToken('existing-token');
        authRepository.state = AsyncData({
          'name': 'Andi',
          'role': AppRole.bendahara,
        });

        apiClient.dio.httpClientAdapter = MockAdapter((options) async {
          return _jsonResponse({'error': 'server error'}, 500);
        });

        await authRepository.logout();

        expect(apiClient.token, isNull);
        expect(authRepository.state, isNull);
        expect(authRepository.currentUser, isNull);
      },
    );

    test(
      'Authenticated request attaches Authorization: Bearer <token>',
      () async {
        apiClient.setToken('my-secret-jwt');
        var headerReceived = '';

        apiClient.dio.httpClientAdapter = MockAdapter((options) async {
          headerReceived = options.headers['Authorization'] as String? ?? '';
          return _jsonResponse({'status': 'ok'}, 200);
        });

        await apiClient.dio.get('/api/v1/dummy');

        expect(headerReceived, equals('Bearer my-secret-jwt'));
      },
    );

    test(
      '401 on authenticated request triggers token clear and unauthenticates state',
      () async {
        apiClient.setToken('expired-jwt');
        authRepository.state = AsyncData({
          'name': 'Andi',
          'role': AppRole.bendahara,
        });

        apiClient.dio.httpClientAdapter = MockAdapter((options) async {
          return _jsonResponse({'message': 'token expired'}, 401);
        });

        try {
          await apiClient.dio.get('/api/v1/protected-resource');
        } catch (_) {}

        expect(apiClient.token, isNull);
        expect(authRepository.state, isNull);
        expect(authRepository.currentUser, isNull);
      },
    );

    test(
      'Missing API_BASE_URL throws clear ApiConfigException without calling localhost',
      () async {
        // Client with no custom baseUrl and empty environment
        final unconfiguredClient = ApiClient(baseUrl: '');
        var requestMade = false;

        unconfiguredClient.dio.httpClientAdapter = MockAdapter((options) async {
          requestMade = true;
          return _jsonResponse({}, 200);
        });

        final service = AuthService(unconfiguredClient);
        final repo = AuthRepository(
          authService: service,
          apiClient: unconfiguredClient,
        );

        await repo.login('test@example.com', 'password');

        expect(requestMade, isFalse);
        expect(repo.state, isA<AsyncError<Map<String, dynamic>?>>());
        final error = repo.state!.error as AuthError;
        expect(error.message, contains('API_BASE_URL belum dikonfigurasi'));
        expect(error.message, isNot(contains('localhost')));
        expect(error.message, isNot(contains('127.0.0.1')));
      },
    );
  });
}
