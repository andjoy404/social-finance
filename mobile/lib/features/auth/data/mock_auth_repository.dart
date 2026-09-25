import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/api/auth_service.dart';
import '../../../core/api/client.dart';
import '../../../core/api/config.dart';
import '../../../core/errors/app_errors.dart';
import '../../../core/models/role.dart';

/// Maps backend role string to [AppRole].
/// Unknown roles safely default to non-privileged [AppRole.warga].
AppRole mapStringToAppRole(String? roleStr) {
  switch (roleStr?.toLowerCase().trim()) {
    case 'super_admin':
      return AppRole.superAdmin;
    case 'pengurus':
      return AppRole.pengurus;
    case 'bendahara':
      return AppRole.bendahara;
    case 'warga':
      return AppRole.warga;
    default:
      return AppRole.warga;
  }
}

/// Authentication repository managing login, logout, and session state.
class AuthRepository extends StateNotifier<AsyncValue<Map<String, dynamic>?>?> {
  final AuthService? authService;
  final ApiClient? apiClient;

  Map<String, dynamic>? _currentUser;

  AuthRepository({this.authService, this.apiClient}) : super(null) {
    apiClient?.onUnauthorized = _handleUnauthorized;
  }

  AuthService get _effectiveAuthService =>
      authService ?? AuthService(_effectiveApiClient);

  ApiClient get _effectiveApiClient => apiClient ?? ApiClient();

  void _handleUnauthorized() {
    _currentUser = null;
    state = null;
  }

  Future<void> login(String email, String password) async {
    final trimmedEmail = email.trim();
    if (trimmedEmail.isEmpty || password.isEmpty) {
      state = AsyncError(
        AuthError('Email dan kata sandi wajib diisi.'),
        StackTrace.current,
      );
      return;
    }

    state = const AsyncLoading();

    try {
      final response = await _effectiveAuthService.login(
        trimmedEmail,
        password,
      );
      _currentUser = {
        'id': response.user.id,
        'email': response.user.email,
        'name': response.user.name.isNotEmpty
            ? response.user.name
            : response.user.email,
        'role': mapStringToAppRole(response.user.role),
        'system_role': response.user.systemRole,
        'rt_id': response.user.rtId,
        'rt': null,
        'rw': null,
        'access_token': response.accessToken,
      };
      state = AsyncData(_currentUser);
    } catch (e, st) {
      final authError = _mapError(e);
      state = AsyncError(authError, st);
    }
  }

  Future<void> logout() async {
    try {
      await _effectiveAuthService.logout();
    } catch (_) {
      // Ignored: local session must always be cleared even if remote logout fails.
    } finally {
      _effectiveApiClient.clearToken();
      _currentUser = null;
      state = null;
    }
  }

  AppRole get currentRole => _currentUser?['role'] as AppRole? ?? AppRole.warga;

  Map<String, dynamic>? get currentUser => _currentUser;

  String? get accessToken => _effectiveApiClient.token;

  AuthError _mapError(dynamic error) {
    if (error is AuthError) return error;
    if (error is ApiConfigException) return AuthError(error.message);

    if (error is DioException) {
      switch (error.type) {
        case DioExceptionType.connectionTimeout:
        case DioExceptionType.sendTimeout:
        case DioExceptionType.receiveTimeout:
          return AuthError(
            'Koneksi ke server batas waktu berakhir. Silakan coba lagi.',
          );
        case DioExceptionType.connectionError:
          return AuthError(
            'Gagal terhubung ke server. Periksa koneksi internet Anda.',
          );
        case DioExceptionType.badResponse:
          final status = error.response?.statusCode;
          if (status == 401) {
            final data = error.response?.data;
            if (data is Map && data['message'] != null) {
              return AuthError(data['message'].toString());
            }
            if (data is Map &&
                data['error'] is Map &&
                data['error']['message'] != null) {
              return AuthError(data['error']['message'].toString());
            }
            return AuthError('Email atau kata sandi salah.');
          } else if (status != null && status >= 500) {
            return AuthError(
              'Terjadi kesalahan pada server ($status). Silakan coba lagi nanti.',
            );
          } else {
            return AuthError(
              'Permintaan gagal diproses (${status ?? 'unknown'}).',
            );
          }
        case DioExceptionType.cancel:
          return AuthError('Permintaan dibatalkan.');
        case DioExceptionType.badCertificate:
          return AuthError('Sertifikat keamanan server tidak valid.');
        case DioExceptionType.unknown:
        default:
          if (error.error is ApiConfigException) {
            return AuthError((error.error as ApiConfigException).message);
          }
          return AuthError('Terjadi kesalahan saat menghubungi server.');
      }
    }

    if (error is FormatException) {
      return AuthError('Format respons dari server tidak sesuai.');
    }

    return AuthError('Terjadi kesalahan yang tidak diketahui.');
  }
}

final authRepositoryProvider =
    StateNotifierProvider<AuthRepository, AsyncValue<Map<String, dynamic>?>?>((
      ref,
    ) {
      final authService = ref.watch(authServiceProvider);
      final apiClient = ref.watch(apiClientProvider);
      return AuthRepository(authService: authService, apiClient: apiClient);
    });
