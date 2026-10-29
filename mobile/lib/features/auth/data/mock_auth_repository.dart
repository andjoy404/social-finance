import 'dart:developer' as developer;

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/api/auth_service.dart';
import '../../../core/api/client.dart';
import '../../../core/api/config.dart';
import '../../../core/api/models.dart';
import '../../../core/errors/app_errors.dart';
import '../../../core/models/role.dart';
import '../../../core/storage/app_storage.dart';
import '../../../core/utils/phone_utils.dart';
import '../../../core/utils/url_utils.dart';

/// Maps backend role string to [AppRole].
/// Unknown roles safely default to non-privileged [AppRole.warga].
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

/// Authentication repository managing login, logout, session restoration,
/// and secure persistence.
class AuthRepository extends StateNotifier<AsyncValue<Map<String, dynamic>?>?> {
  static const _profileReady = 'ready';
  static const _profileStale = 'stale';
  static const _profileLoading = 'loading';
  static const _profileError = 'error';
  static const _profilePermissionsUnavailable = 'permissions_unavailable';

  final AuthService authService;
  final ApiClient apiClient;
  final PreferencesStorage preferencesStorage;
  final SecureAuthStorage secureAuthStorage;

  Map<String, dynamic>? _currentUser;
  Future<void>? _profileReloadFuture;
  Future<void> Function()? onBeforeLogout;

  AuthRepository({
    required this.authService,
    required this.apiClient,
    PreferencesStorage? preferencesStorage,
    SecureAuthStorage? secureAuthStorage,
  }) : preferencesStorage = preferencesStorage ?? SharedPrefsStorage(),
       secureAuthStorage = secureAuthStorage ?? FlutterSecureAuthStorage(),
       super(null) {
    apiClient.onUnauthorized = _handleUnauthorized;
    apiClient.onRefreshToken = _refreshTokens;
  }

  Future<LoginResponse?> _refreshTokens(String refreshToken) async {
    try {
      final response = await authService.refreshToken(refreshToken);
      final newRefreshToken = response.refreshToken.isNotEmpty
          ? response.refreshToken
          : refreshToken;
      apiClient.setToken(response.accessToken);
      apiClient.setRefreshToken(newRefreshToken);

      // Persist rotated tokens IMMEDIATELY before any follow-up request (/auth/me)
      // and even when _currentUser is still null (e.g. during restoreSession()).
      // Backend refresh tokens are single-use rotated, so failing to save the
      // new refresh token immediately would leave a revoked token on disk.
      final stored = await secureAuthStorage.getSession();
      if (stored != null) {
        await secureAuthStorage.saveSession(
          accessToken: response.accessToken,
          refreshToken: newRefreshToken,
          serverUrl: stored.serverUrl.isNotEmpty
              ? stored.serverUrl
              : apiClient.baseUrl,
          userData: stored.userData,
        );
      }

      if (_currentUser != null) {
        final previousPermissions =
            (_currentUser!['effective_permissions'] as List?)?.cast<String>() ??
            <String>[];

        _currentUser = {
          ..._currentUser!,
          'access_token': response.accessToken,
        };
        state = AsyncData(_currentUser);
        try {
          final freshUser = await authService.getMe();
          _setCurrentUser(freshUser, response.accessToken);
          if (stored != null) {
            await secureAuthStorage.saveSession(
              accessToken: response.accessToken,
              refreshToken: newRefreshToken,
              serverUrl: stored.serverUrl.isNotEmpty
                  ? stored.serverUrl
                  : apiClient.baseUrl,
              userData: freshUser.toJson(),
            );
          }
        } catch (meErr, stackTrace) {
          if (ApiClient.isTransientNetworkOrServerError(meErr)) {
            _currentUser = {
              ..._currentUser!,
              'effective_permissions': previousPermissions,
              'profile_status': _profileStale,
              'access_token': response.accessToken,
            };
          } else {
            _currentUser = {
              ..._currentUser!,
              'effective_permissions': <String>[],
              'profile_status': _profileError,
              'access_token': response.accessToken,
            };
          }
          state = AsyncData(_currentUser);
          developer.log(
            'Profile refresh failed after token rotation.',
            name: 'AuthRepository',
            stackTrace: stackTrace,
          );
        }
      }
      return response;
    } catch (e, stackTrace) {
      developer.log(
        'Token refresh failed.',
        name: 'AuthRepository',
        stackTrace: stackTrace,
      );
      if (ApiClient.isTransientNetworkOrServerError(e)) {
        // Rethrow transient network/server errors so ApiClient keeps the
        // refresh token and active session intact for retry.
        rethrow;
      }
      await secureAuthStorage.clearSession();
      return null;
    }
  }

  void _handleUnauthorized() {
    _currentUser = null;
    state = null;
    secureAuthStorage.clearSession();
  }

  bool _restoreCachedUserIfAvailable(StoredSession stored) {
    if (stored.userData.isEmpty) {
      return false;
    }
    try {
      final cachedUser = AuthUser.fromJson(stored.userData);
      if (cachedUser.id.isEmpty && cachedUser.email.isEmpty) {
        return false;
      }
      final activeToken = apiClient.token ?? stored.accessToken;
      _setCurrentUser(
        cachedUser,
        activeToken,
        profileStatusOverride: _profileStale,
      );
      return true;
    } catch (_) {
      return false;
    }
  }

  /// Restores session on app startup if "Ingat Saya" was active.
  ///
  /// Returns `true` if session was successfully restored, `false` otherwise.
  Future<bool> restoreSession() async {
    try {
      // 1. Ensure API URL is initialized on ApiClient before any auth or network action
      var currentUrl = apiClient.baseUrl;
      if (currentUrl.isEmpty) {
        final lastSavedUrl = await preferencesStorage.getLastApiUrl();
        if (lastSavedUrl != null && lastSavedUrl.isNotEmpty) {
          if (UrlUtils.validateUrl(lastSavedUrl) == null) {
            currentUrl = UrlUtils.normalizeUrl(lastSavedUrl);
            apiClient.setBaseUrl(currentUrl);
          }
        }
      }

      final rememberMe = await preferencesStorage.getRememberMe();
      if (!rememberMe) {
        await secureAuthStorage.clearSession();
        await secureAuthStorage.clearLoginCredentials();
        return false;
      }

      final stored = await secureAuthStorage.getSession();
      if (stored == null ||
          (stored.accessToken.isEmpty && stored.refreshToken.isEmpty)) {
        return false;
      }

      // If server changed, do NOT send old server tokens to new server!
      if (currentUrl.isNotEmpty &&
          stored.serverUrl.isNotEmpty &&
          UrlUtils.normalizeUrl(currentUrl) !=
              UrlUtils.normalizeUrl(stored.serverUrl)) {
        await secureAuthStorage.clearSession();
        return false;
      }

      if (currentUrl.isEmpty && stored.serverUrl.isNotEmpty) {
        currentUrl = UrlUtils.normalizeUrl(stored.serverUrl);
        apiClient.setBaseUrl(currentUrl);
      }

      if (stored.accessToken.isNotEmpty) {
        apiClient.setToken(stored.accessToken);
      }
      if (stored.refreshToken.isNotEmpty) {
        apiClient.setRefreshToken(stored.refreshToken);
      }

      try {
        final user = await authService.getMe();
        final activeAccessToken = apiClient.token ?? stored.accessToken;
        final activeRefreshToken =
            apiClient.refreshToken ?? stored.refreshToken;
        _setCurrentUser(user, activeAccessToken);
        await secureAuthStorage.saveSession(
          accessToken: activeAccessToken,
          refreshToken: activeRefreshToken,
          serverUrl: currentUrl.isNotEmpty ? currentUrl : stored.serverUrl,
          userData: user.toJson(),
        );
        return true;
      } catch (meError) {
        // If getMe() failed due to a transient network/server error, do NOT
        // wipe stored credentials or force re-login. Restore from cached
        // user profile if available and keep tokens for retry.
        if (ApiClient.isTransientNetworkOrServerError(meError)) {
          return _restoreCachedUserIfAvailable(stored);
        }

        // Only attempt manual refresh if the 401 interceptor did not already
        // consume and clear apiClient.refreshToken.
        final remainingRefreshToken = apiClient.refreshToken;
        if (remainingRefreshToken != null && remainingRefreshToken.isNotEmpty) {
          try {
            final refreshed = await _refreshTokens(remainingRefreshToken);
            if (refreshed != null) {
              final user = await authService.getMe();
              final activeAccessToken =
                  apiClient.token ?? refreshed.accessToken;
              final activeRefreshToken =
                  apiClient.refreshToken ??
                  (refreshed.refreshToken.isNotEmpty
                      ? refreshed.refreshToken
                      : remainingRefreshToken);
              _setCurrentUser(user, activeAccessToken);
              await secureAuthStorage.saveSession(
                accessToken: activeAccessToken,
                refreshToken: activeRefreshToken,
                serverUrl:
                    currentUrl.isNotEmpty ? currentUrl : stored.serverUrl,
                userData: user.toJson(),
              );
              return true;
            }
          } catch (refreshErr) {
            if (ApiClient.isTransientNetworkOrServerError(refreshErr)) {
              return _restoreCachedUserIfAvailable(stored);
            }
          }
        }

        // Permanent authentication failure (revoked/expired refresh token):
        apiClient.clearAllTokens();
        await secureAuthStorage.clearSession();
        _currentUser = null;
        state = null;
        return false;
      }
    } catch (e) {
      developer.log('Failed to restore session: $e', name: 'AuthRepository');
      return false;
    }
  }

  /// Changes the active server URL atomically.
  ///
  /// Clears in-flight requests, tokens, session storage, and in-memory state.
  Future<void> changeServer(String newServerUrl) async {
    final normalized = UrlUtils.normalizeUrl(newServerUrl);
    final storedLogin = await secureAuthStorage.getLoginCredentials();
    if (storedLogin != null &&
        UrlUtils.normalizeUrl(storedLogin.serverUrl) != normalized) {
      await secureAuthStorage.clearLoginCredentials();
    }
    if (apiClient.baseUrl == normalized) return;

    // 1. Reset client (clears tokens and in-flight refresh)
    apiClient.resetServer(normalized);

    // 2. Clear old session in secure storage to prevent cross-server leaks
    await secureAuthStorage.clearSession();

    // 3. Clear user and state
    _currentUser = null;
    state = null;

    // 4. Save to preferences
    await preferencesStorage.setLastApiUrl(normalized);
  }

  /// Logs in using email or phone number.
  ///
  /// Positional [emailOrIdentifier] supports backward compatibility with
  /// existing tests and callers. Named parameters allow passing explicit
  /// [phone], [serverUrl], and [rememberMe].
  Future<void> login(
    String emailOrIdentifier,
    String password, {
    String? phone,
    String? serverUrl,
    bool rememberMe = true,
  }) async {
    // 1. Validate and configure server URL if provided
    if (serverUrl != null && serverUrl.trim().isNotEmpty) {
      final urlErr = UrlUtils.validateUrl(serverUrl);
      if (urlErr != null) {
        state = AsyncError(AuthError(urlErr), StackTrace.current);
        return;
      }
      try {
        await changeServer(serverUrl);
      } catch (_) {
        state = AsyncError(
          AuthError(
            'Kredensial tersimpan tidak dapat diperiksa dengan aman. Coba lagi.',
          ),
          StackTrace.current,
        );
        return;
      }
    }

    // 2. Determine and validate login identifier
    String? effectiveEmail;
    String? effectivePhone;

    if (phone != null && phone.trim().isNotEmpty) {
      final phoneErr = PhoneUtils.validatePhone(phone);
      if (phoneErr != null) {
        state = AsyncError(AuthError(phoneErr), StackTrace.current);
        return;
      }
      effectivePhone = PhoneUtils.normalizePhone(phone);
    } else {
      final identifier = emailOrIdentifier.trim();
      if (identifier.isEmpty || password.isEmpty) {
        state = AsyncError(
          AuthError('Email dan kata sandi wajib diisi.'),
          StackTrace.current,
        );
        return;
      }

      // Check if identifier is phone number
      if (PhoneUtils.validatePhone(identifier) == null) {
        effectivePhone = PhoneUtils.normalizePhone(identifier);
      } else {
        effectiveEmail = identifier;
      }
    }

    if (password.isEmpty) {
      state = AsyncError(
        AuthError('Kata sandi wajib diisi.'),
        StackTrace.current,
      );
      return;
    }

    _currentUser = null;
    apiClient.clearAllTokens();
    state = const AsyncLoading();

    var loginSucceeded = false;
    try {
      final response = await authService.login(
        email: effectiveEmail,
        phone: effectivePhone,
        password: password,
      );
      loginSucceeded = true;
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

      // Save preferences and secure session
      final activeServerUrl = apiClient.baseUrl;
      try {
        await preferencesStorage.setRememberMe(rememberMe);
        if (activeServerUrl.isNotEmpty) {
          await preferencesStorage.setLastApiUrl(activeServerUrl);
        }
        if (effectiveEmail != null && effectiveEmail.isNotEmpty) {
          await preferencesStorage.setLastEmail(effectiveEmail);
          await preferencesStorage.setLastLoginMethod('email');
        } else if (effectivePhone != null && effectivePhone.isNotEmpty) {
          await preferencesStorage.setLastPhone(effectivePhone);
          await preferencesStorage.setLastLoginMethod('phone');
        }
      } catch (e) {
        developer.log('Preferences save error: $e', name: 'AuthRepository');
      }

      if (rememberMe) {
        await secureAuthStorage.saveLoginCredentials(
          identifier: effectivePhone ?? effectiveEmail!,
          loginMethod: effectivePhone == null ? 'email' : 'phone',
          password: password,
          serverUrl: activeServerUrl,
        );
        await secureAuthStorage.saveSession(
          accessToken: response.accessToken,
          refreshToken: response.refreshToken,
          serverUrl: activeServerUrl,
          userData: effectiveUser.toJson(),
        );
      } else {
        await secureAuthStorage.clearLoginCredentials();
        await secureAuthStorage.clearSession();
      }
      _setCurrentUser(effectiveUser, response.accessToken);
    } catch (e, st) {
      if (loginSucceeded) {
        try {
          await authService.logout();
        } catch (_) {}
        apiClient.clearAllTokens();
        try {
          await secureAuthStorage.clearSession();
        } catch (_) {}
      }
      _currentUser = null;
      final authError = e is AuthError
          ? e
          : (loginSucceeded
                ? AuthError(
                    'Login berhasil, tetapi data Ingat Saya tidak dapat disimpan dengan aman. Coba lagi atau nonaktifkan Ingat Saya.',
                  )
                : _mapError(e));
      state = AsyncError(authError, st);
    }
  }

  void _setCurrentUser(
    AuthUser user,
    String accessToken, {
    String? profileStatusOverride,
  }) {
    _currentUser = {
      'id': user.id,
      'email': user.email,
      'phone': user.phone,
      'name': user.name.isNotEmpty ? user.name : user.email,
      'role': mapStringToAppRole(user.role, systemRoleStr: user.systemRole),
      'system_role': user.systemRole,
      'jabatan': user.jabatan,
      'house_number': user.houseNumber,
      'effective_permissions': user.effectivePermissions,
      'profile_status': profileStatusOverride ??
          (user.effectivePermissionsAvailable
              ? _profileReady
              : _profilePermissionsUnavailable),
      'rt_id': user.rtId,
      'rt': user.rtCode,
      'rt_name': user.rtName,
      'rt_address': user.rtAddress,
      'rw': user.rwCode,
      'rw_id': user.rwId,
      'rw_name': user.rwName,
      'access_token': accessToken,
    };
    state = AsyncData(_currentUser);
  }

  Future<void> reloadProfile() {
    if (_profileReloadFuture != null) {
      return _profileReloadFuture!;
    }
    final future = _doReloadProfile();
    _profileReloadFuture = future;
    return future.whenComplete(() {
      if (identical(_profileReloadFuture, future)) {
        _profileReloadFuture = null;
      }
    });
  }

  Future<void> _doReloadProfile() async {
    final currentUser = _currentUser;
    final accessToken = apiClient.token;
    if (currentUser == null || accessToken == null) return;

    final previousPermissions =
        (currentUser['effective_permissions'] as List?)?.cast<String>() ??
        <String>[];
    final previousStatus =
        currentUser['profile_status'] as String? ?? _profileReady;

    _currentUser = {
      ...currentUser,
      'effective_permissions': previousPermissions,
      'profile_status': _profileLoading,
    };
    state = AsyncData(_currentUser);

    try {
      final freshUser = await authService.getMe();
      final activeAccessToken = apiClient.token ?? accessToken;
      _setCurrentUser(freshUser, activeAccessToken);
      final stored = await secureAuthStorage.getSession();
      if (stored != null) {
        final activeRefreshToken =
            apiClient.refreshToken ?? stored.refreshToken;
        await secureAuthStorage.saveSession(
          accessToken: activeAccessToken,
          refreshToken: activeRefreshToken,
          serverUrl: stored.serverUrl.isNotEmpty
              ? stored.serverUrl
              : apiClient.baseUrl,
          userData: freshUser.toJson(),
        );
      }
    } catch (err, stackTrace) {
      if (_currentUser == null) {
        // Session was cleared permanently (e.g. 401 refresh failure triggered
        // _handleUnauthorized). Do not resurrect _currentUser.
        return;
      }
      if (ApiClient.isTransientNetworkOrServerError(err)) {
        _currentUser = {
          ..._currentUser!,
          'effective_permissions': previousPermissions,
          'profile_status': previousStatus == _profileError
              ? _profileError
              : _profileStale,
        };
        state = AsyncData(_currentUser);
        developer.log(
          'Profile reload failed due to transient network error; cached profile retained as stale.',
          name: 'AuthRepository',
          stackTrace: stackTrace,
        );
      } else {
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
  }

  Future<void> logout() async {
    try {
      await onBeforeLogout?.call();
    } catch (e) {
      developer.log('onBeforeLogout hook failed: $e', name: 'AuthRepository');
    }
    try {
      await authService.logout();
    } catch (_) {
      // Ignored: local session must always be cleared even if remote logout fails.
    } finally {
      apiClient.clearAllTokens();
      await secureAuthStorage.clearSession();
      _currentUser = null;
      state = null;
    }
  }

  Future<void> changePassword({
    required String currentPassword,
    required String newPassword,
  }) async {
    try {
      await authService.changePassword(
        currentPassword: currentPassword,
        newPassword: newPassword,
      );
    } catch (e) {
      throw _mapError(e);
    }
  }

  Future<void> updateProfile({
    required String fullName,
    String? phone,
    String? email,
  }) async {
    try {
      await authService.updateProfile(
        fullName: fullName,
        phone: phone,
        email: email,
      );
      await reloadProfile();
    } catch (e) {
      throw _mapError(e);
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
      final preferencesStorage = ref.watch(preferencesStorageProvider);
      final secureAuthStorage = ref.watch(secureAuthStorageProvider);
      return AuthRepository(
        authService: authService,
        apiClient: apiClient,
        preferencesStorage: preferencesStorage,
        secureAuthStorage: secureAuthStorage,
      );
    });

const allowedRTCodes = {'002', '003'};
const allowedRTIds = {
  'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b', // RT 002
  '8dfa1e36-6727-45be-8fea-24d958efc155', // RT 003
};

bool isAllowedRT(Map<String, dynamic>? user) {
  if (user == null) return false;
  final role = user['role'];
  final systemRole =
      (user['system_role'] ?? '').toString().trim().toLowerCase();
  if (role == AppRole.superAdmin ||
      role == 'super_admin' ||
      systemRole == 'super_admin') {
    return true;
  }
  final rtCode = user['rt'] as String?;
  if (rtCode != null && allowedRTCodes.contains(rtCode)) return true;
  final rtId = user['rt_id'] as String?;
  if (rtId != null && allowedRTIds.contains(rtId)) return true;
  return false;
}
