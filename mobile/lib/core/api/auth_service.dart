import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'client.dart';
import 'device_id_service.dart';
import 'models.dart';

class AuthService {
  final ApiClient client;
  final DeviceIdService deviceIdService;

  AuthService(this.client, this.deviceIdService);

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

    // Attach device ID to login request.
    final deviceId = await deviceIdService.getDeviceId();
    data['device_id'] = deviceId;

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
    // Attach device ID to refresh request.
    final deviceId = await deviceIdService.getDeviceId();
    final response = await client.dio.post(
      '/api/v1/auth/refresh',
      data: {
        'refresh_token': rawRefreshToken,
        'device_id': deviceId,
      },
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

  Future<void> changePassword({
    required String currentPassword,
    required String newPassword,
  }) async {
    await client.dio.put(
      '/api/v1/auth/password',
      data: {
        'current_password': currentPassword,
        'new_password': newPassword,
      },
    );
  }

  Future<void> updateProfile({
    required String fullName,
    String? phone,
    String? email,
  }) async {
    final body = <String, dynamic>{
      'full_name': fullName,
      if (phone != null) 'phone': phone,
      if (email != null) 'email': email,
    };
    await client.dio.put('/api/v1/auth/profile', data: body);
  }
}

final authServiceProvider = Provider<AuthService>((ref) {
  final client = ref.watch(apiClientProvider);
  final deviceIdService = ref.watch(deviceIdServiceProvider);
  return AuthService(client, deviceIdService);
});
