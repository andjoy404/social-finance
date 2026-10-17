import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'client.dart';
import 'models.dart';

class AuthService {
  final ApiClient client;

  AuthService(this.client);

  /// Authenticates using email or phone number.
  Future<LoginResponse> login({
    String? email,
    String? phone,
    required String password,
  }) async {
    final Map<String, dynamic> data = {'password': password};
    if (email != null && email.trim().isNotEmpty) {
      data['email'] = email.trim();
    } else if (phone != null && phone.trim().isNotEmpty) {
      data['phone'] = phone.trim();
    } else {
      throw const FormatException('Email atau nomor HP wajib diisi.');
    }

    final response = await client.dio.post('/api/v1/auth/login', data: data);

    final responseData = response.data;
    if (responseData is! Map) {
      throw const FormatException('Expected JSON object in login response');
    }

    final loginResponse = LoginResponse.fromJson(
      responseData.cast<String, dynamic>(),
    );
    client.setToken(loginResponse.accessToken);
    if (loginResponse.refreshToken.isNotEmpty) {
      client.setRefreshToken(loginResponse.refreshToken);
    }
    return loginResponse;
  }

  /// Convenience method for login with email.
  Future<LoginResponse> loginWithEmail(String email, String password) {
    return login(email: email, password: password);
  }

  /// Convenience method for login with phone number.
  Future<LoginResponse> loginWithPhone(String phone, String password) {
    return login(phone: phone, password: password);
  }

  Future<void> logout() async {
    await client.dio.post('/api/v1/auth/logout');
  }

  Future<LoginResponse> refreshToken(String rawRefreshToken) async {
    final response = await client.dio.post(
      '/api/v1/auth/refresh',
      data: {'refresh_token': rawRefreshToken},
    );
    final data = response.data;
    if (data is! Map) {
      throw const FormatException('Expected JSON object in refresh response');
    }
    final result = LoginResponse.fromJson(data.cast<String, dynamic>());
    client.setToken(result.accessToken);
    if (result.refreshToken.isNotEmpty) {
      client.setRefreshToken(result.refreshToken);
    }
    return result;
  }

  Future<AuthUser> getMe() async {
    final response = await client.dio.get('/api/v1/auth/me');
    final data = response.data;
    if (data is! Map) {
      throw const FormatException('Expected JSON object in /auth/me response');
    }
    return AuthUser.fromJson(data.cast<String, dynamic>());
  }
}

final authServiceProvider = Provider<AuthService>((ref) {
  final client = ref.watch(apiClientProvider);
  return AuthService(client);
});
