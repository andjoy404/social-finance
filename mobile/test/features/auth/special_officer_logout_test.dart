import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/api/device_id_service.dart';
import 'package:social_finance/core/storage/app_storage.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';

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
  Future<String> getDeviceId() async => 'special-officer-test-device';
}

ResponseBody _jsonResponse(Object data, int statusCode) {
  return ResponseBody.fromString(
    jsonEncode(data),
    statusCode,
    headers: {
      Headers.contentTypeHeader: [Headers.jsonContentType],
    },
  );
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test(
    'logout clears the security session before another officer logs in',
    () async {
      final preferences = InMemoryPreferencesStorage(
        initialUrl: 'https://api.example.test',
        initialRememberMe: true,
      );
      final secureStorage = InMemorySecureAuthStorage();
      final client = ApiClient(baseUrl: 'https://api.example.test');
      client.dio.httpClientAdapter = _MockAdapter((options) async {
        if (options.path.endsWith('/auth/login')) {
          final body = (options.data as Map).cast<String, dynamic>();
          final isSecurity = body['email'] == 'keamanan@example.test';
          final email = body['email'] as String;
          final jabatan = isSecurity ? 'keamanan' : 'kebersihan';
          return _jsonResponse({
            'access_token': 'access-$jabatan',
            'refresh_token': 'refresh-$jabatan',
            'token_type': 'Bearer',
            'expires_in': 900,
            'user': {
              'id': 'user-$jabatan',
              'email': email,
              'full_name': jabatan,
              'role': 'warga',
            },
          }, 200);
        }
        if (options.path.endsWith('/auth/me')) {
          final security = client.token == 'access-keamanan';
          final jabatan = security ? 'keamanan' : 'kebersihan';
          return _jsonResponse({
            'id': 'user-$jabatan',
            'email': security
                ? 'keamanan@example.test'
                : 'kebersihan@example.test',
            'full_name': jabatan,
            'role': 'warga',
            'jabatan': jabatan,
            'rt_id': 'rt-test',
            'rt_rt': '002',
            'rt_rw': 16,
            'effective_permissions': <String>[],
          }, 200);
        }
        if (options.path.endsWith('/auth/logout')) {
          return ResponseBody.fromString('', 204);
        }
        throw StateError(
          'Unexpected API request: ${options.method} ${options.path}',
        );
      });

      final repository = AuthRepository(
        authService: AuthService(client, _FakeDeviceIdService()),
        apiClient: client,
        preferencesStorage: preferences,
        secureAuthStorage: secureStorage,
      );

      await repository.login('keamanan@example.test', 'Password123!');
      expect(repository.currentJabatan, 'keamanan');
      expect(client.token, 'access-keamanan');
      expect(await secureStorage.hasSession(), isTrue);
      expect(await secureStorage.getLoginCredentials(), isNotNull);

      await repository.logout();

      expect(client.token, isNull);
      expect(client.refreshToken, isNull);
      expect(repository.currentUser, isNull);
      expect(repository.state, isNull);
      expect(await secureStorage.hasSession(), isFalse);
      expect(await secureStorage.getLoginCredentials(), isNull);
      expect(await preferences.getRememberMe(), isFalse);

      await repository.login('kebersihan@example.test', 'Password456!');
      expect(repository.currentJabatan, 'kebersihan');
      expect(repository.currentUser?['email'], 'kebersihan@example.test');
      expect(client.token, 'access-kebersihan');
    },
  );
}
