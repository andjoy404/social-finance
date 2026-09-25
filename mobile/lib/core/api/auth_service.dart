import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'client.dart';
import 'models.dart';

class AuthService {
  final ApiClient client;

  AuthService(this.client);

  Future<LoginResponse> login(String email, String password) async {
    final response = await client.dio.post(
      '/api/v1/auth/login',
      data: {'email': email, 'password': password},
    );

    final data = response.data;
    if (data is! Map) {
      throw const FormatException('Expected JSON object in login response');
    }

    final loginResponse = LoginResponse.fromJson(data.cast<String, dynamic>());
    client.setToken(loginResponse.accessToken);
    return loginResponse;
  }

  Future<void> logout() async {
    try {
      await client.dio.post('/api/v1/auth/logout');
    } finally {
      client.clearToken();
    }
  }
}

final authServiceProvider = Provider<AuthService>((ref) {
  final client = ref.watch(apiClientProvider);
  return AuthService(client);
});
