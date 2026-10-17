import 'dart:developer' as developer;

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/api/auth_service.dart';
import '../../../core/api/client.dart';
import '../../../core/api/config.dart';
import '../../../core/api/models.dart';
import '../../../core/errors/app_errors.dart';
import '../../../core/models/role.dart';

/// Maps backend role string to [AppRole].
/// Unknown roles safely default to non-privileged [AppRole.warga].
///
/// The backend assigns [systemRoleStr] (system_role) to platform-wide admins
/// and sets [roleStr] (role) to the RT-scoped role for ordinary users.
/// For super_admin users, [roleStr] is empty and [systemRoleStr] is
/// "super_admin". For all other users, [systemRoleStr] is null/empty.
///
/// Mapping priority: use [roleStr] if non-empty; otherwise fall back to
/// [systemRoleStr]. Unknown strings → [AppRole.warga] (never elevated).
AppRole mapStringToAppRole(String? roleStr, {String? systemRoleStr}) {
  final effective = (roleStr?.trim().isNotEmpty == true)
      ? roleStr!.trim().toLowerCase()
      : (systemRoleStr?.trim().toLowerCase() ?? '');
  switch (effective) {
    case 'super_admin':
      return AppRole.superAdmin;
    case 'pengurus':
      return AppRole.pengurus;
    case 'bendahara':
      return AppRole.bendahara;
    case 'warga':
      return AppRole.warga;
    case 'perangkat':
      return AppRole.perangkat;
    default:
      return AppRole.warga;
  }
}

/// Authentication repository managing login, logout, and session state.
class AuthRepository extends StateNotifier<AsyncValue<Map<String, dynamic>?>?> {
  static const _profileReady = 'ready';
  static const _profileLoading = 'loading';
  static const _profileError = 'error';
  static const _profilePermissionsUnavailable = 'permissions_unavailable';

  final AuthService authService;
  final ApiClient apiClient;

  Map<String, dynamic>? _currentUser;

  AuthRepository({required this.authService, required this.apiClient})
    : super(null) {
    apiClient.onUnauthorized = _handleUnauthorized;
    apiClient.onRefreshToken = _refreshTokens;
  }

  Future<LoginResponse?> _refreshTokens(String refreshToken) async {
    try {
      final response = await authService.refreshToken(refreshToken);
      if (_currentUser != null) {
        _currentUser = {
          ..._currentUser!,
          'effective_permissions': <String>[],
          'profile_status': _profileLoading,
          'access_token': response.accessToken,
        };
        state = AsyncData(_currentUser);
        try {
          _setCurrentUser(await authService.getMe(), response.accessToken);
        } catch (_, stackTrace) {
          _currentUser = {
            ..._currentUser!,
            'effective_permissions': <String>[],
            'profile_status': _profileError,
          };
          state = AsyncData(_currentUser);
          developer.log(
            'Profile refresh failed; access remains blocked until profile reload.',
            name: 'AuthRepository',
            stackTrace: stackTrace,
          );
        }
      }
      return response;
    } catch (_, stackTrace) {
      developer.log(
        'Token refresh failed.',
        name: 'AuthRepository',
        stackTrace: stackTrace,
      );
      return null;
    }
  }

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

    _currentUser = null;
    apiClient.clearAllTokens();
    state = const AsyncLoading();

    try {
      final response = await authService.login(trimmedEmail, password);
      late final AuthUser effectiveUser;
      try {
        effectiveUser = await authService.getMe();
      } catch (error, stackTrace) {
        apiClient.clearAllTokens();
        _currentUser = null;
        developer.log(
          'Profile load after login failed; authenticated state was not created.',
          name: 'AuthRepository',
          stackTrace: stackTrace,
        );
        state = AsyncError(
          AuthError(
            'Login berhasil, tetapi profil akses gagal dimuat. Periksa koneksi lalu coba lagi.',
          ),
          stackTrace,
        );
        return;
      }
      _setCurrentUser(effectiveUser, response.accessToken);
    } catch (e, st) {
      _currentUser = null;
      final authError = _mapError(e);
      state = AsyncError(authError, st);
    }
  }

  void _setCurrentUser(AuthUser user, String accessToken) {
    _currentUser = {
      'id': user.id,
      'email': user.email,
      'name': user.name.isNotEmpty ? user.name : user.email,
      'role': mapStringToAppRole(user.role, systemRoleStr: user.systemRole),
      'system_role': user.systemRole,
      'jabatan': user.jabatan,
      'effective_permissions': user.effectivePermissions,
      'profile_status': user.effectivePermissionsAvailable
          ? _profileReady
          : _profilePermissionsUnavailable,
      'rt_id': user.rtId,
      'rt': user.rtCode,
      'rw': null,
      'access_token': accessToken,
    };
    state = AsyncData(_currentUser);
  }

  Future<void> reloadProfile() async {
    final currentUser = _currentUser;
    final accessToken = apiClient.token;
    if (currentUser == null || accessToken == null) return;

    _currentUser = {
      ...currentUser,
      'effective_permissions': <String>[],
      'profile_status': _profileLoading,
    };
    state = AsyncData(_currentUser);

    try {
      _setCurrentUser(await authService.getMe(), accessToken);
    } catch (_, stackTrace) {
      _currentUser = {
        ..._currentUser!,
        'effective_permissions': <String>[],
        'profile_status': _profileError,
      };
      state = AsyncData(_currentUser);
      developer.log(
        'Profile reload failed; access remains blocked.',
        name: 'AuthRepository',
        stackTrace: stackTrace,
      );
    }
  }

  Future<void> logout() async {
    try {
      await authService.logout();
    } catch (_) {
      // Ignored: local session must always be cleared even if remote logout fails.
    } finally {
      apiClient.clearAllTokens();
      _currentUser = null;
      state = null;
    }
  }

  AppRole get currentRole => _currentUser?['role'] as AppRole? ?? AppRole.warga;

  String? get currentJabatan => _currentUser?['jabatan'] as String?;

  Map<String, dynamic>? get currentUser => _currentUser;

  String? get accessToken => apiClient.token;

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

const allowedRTCodes = {'002', '003'};
const allowedRTIds = {
  'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b', // RT 002
  '8dfa1e36-6727-45be-8fea-24d958efc155', // RT 003
};

bool isAllowedRT(Map<String, dynamic>? user) {
  if (user == null) return false;
  final systemRole = user['system_role'] as String?;
  if (systemRole == 'super_admin') return true;
  final rtCode = user['rt'] as String?;
  if (rtCode != null && allowedRTCodes.contains(rtCode)) return true;
  final rtId = user['rt_id'] as String?;
  if (rtId != null && allowedRTIds.contains(rtId)) return true;
  return false;
}
