import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../utils/url_utils.dart';
import 'config.dart';
import 'models.dart';

class ApiClient {
  final Dio dio;
  String _baseUrl = '';
  String? _token;
  String? _refreshToken;
  String? _deviceID;
  void Function()? onUnauthorized;
  Future<LoginResponse?> Function(String refreshToken)? onRefreshToken;

  /// Current device identifier (set once at app start).
  String? get deviceID => _deviceID;

  /// Sets the persistent device ID for refresh token rotation.
  void setDeviceID(String? id) {
    _deviceID = id;
  }

  // Completer-based single-flight gate.
  Completer<_RefreshAttemptResult>? _refreshGate;
  bool _refreshInFlight = false;

  static bool isTransientNetworkOrServerError(Object error) {
    if (error is DioException) {
      switch (error.type) {
        case DioExceptionType.connectionTimeout:
        case DioExceptionType.sendTimeout:
        case DioExceptionType.receiveTimeout:
        case DioExceptionType.connectionError:
          return true;
        case DioExceptionType.badResponse:
          final status = error.response?.statusCode;
          if (status != null && (status >= 500 || status == 429)) {
            return true;
          }
          return false;
        case DioExceptionType.unknown:
          if (error.error is ApiConfigException) return false;
          return error.response == null;
        case DioExceptionType.cancel:
        case DioExceptionType.badCertificate:
        case DioExceptionType.transformTimeout:
          return false;
      }
    }
    return false;
  }

  ApiClient({
    Dio? customDio,
    String? baseUrl,
    this.onUnauthorized,
    this.onRefreshToken,
  }) : _baseUrl = baseUrl != null
           ? UrlUtils.normalizeUrl(baseUrl)
           : _resolveInitialBaseUrl(),
       dio =
           customDio ??
           Dio(
             BaseOptions(
               baseUrl: baseUrl != null
                   ? UrlUtils.normalizeUrl(baseUrl)
                   : _resolveInitialBaseUrl(),
               connectTimeout: ApiConfig.connectTimeout,
               receiveTimeout: ApiConfig.receiveTimeout,
               sendTimeout: ApiConfig.sendTimeout,
               headers: {
                 'Content-Type': 'application/json',
                 'Accept': 'application/json',
               },
             ),
           ) {
    dio.interceptors.add(
      InterceptorsWrapper(
        onRequest: (options, handler) {
          if (options.baseUrl.isEmpty) {
            if (_baseUrl.isNotEmpty) {
              options.baseUrl = _baseUrl;
            } else if (ApiConfig.hasBaseUrl) {
              options.baseUrl = ApiConfig.normalizedBaseUrl;
            } else {
              return handler.reject(
                DioException(
                  requestOptions: options,
                  error: const ApiConfigException(),
                  type: DioExceptionType.unknown,
                ),
              );
            }
          }
          if (!options.headers.containsKey('Authorization')) {
            if (_token != null && _token!.isNotEmpty) {
              options.headers['Authorization'] = 'Bearer $_token';
            } else {
              options.headers.remove('Authorization');
            }
          } else if (options.extra['_auth_retried'] != true &&
              _token != null &&
              _token!.isNotEmpty) {
            options.headers['Authorization'] = 'Bearer $_token';
          }
          handler.next(options);
        },
        onError: (DioException err, handler) async {
          if (err.response?.statusCode == 401) {
            final path = err.requestOptions.path;
            final isLoginRequest = path.contains('/auth/login');
            final isRefreshRequest = path.contains('/auth/refresh');
            final isLogoutRequest = path.contains('/auth/logout');
            final alreadyRetried =
                err.requestOptions.extra['_auth_retried'] == true;

            if (isRefreshRequest || isLoginRequest || isLogoutRequest) {
              // Pass through directly. For /auth/refresh, the caller inside
              // _doRefresh() handles completing _refreshGate and revoking state.
              return handler.next(err);
            }

            if (alreadyRetried) {
              // Prevent infinite retry loops if a request still returns 401
              // even after refreshing the access token.
              return handler.next(err);
            }

            if (_refreshToken != null && _refreshToken!.isNotEmpty) {
              // Check if another concurrent request ALREADY refreshed _token
              // while this request was in flight with an older access token.
              final sentAuth =
                  err.requestOptions.headers['Authorization'] as String?;
              final currentAuth = (_token != null && _token!.isNotEmpty)
                  ? 'Bearer $_token'
                  : null;
              if (_refreshGate == null &&
                  !_refreshInFlight &&
                  currentAuth != null &&
                  sentAuth != null &&
                  sentAuth != currentAuth) {
                final opts = err.requestOptions;
                opts.extra['_auth_retried'] = true;
                opts.headers['Authorization'] = currentAuth;
                try {
                  final response = await dio.fetch(opts);
                  return handler.resolve(response);
                } on DioException catch (retryErr) {
                  return handler.next(retryErr);
                }
              }

              if (_refreshGate == null && !_refreshInFlight) {
                // ===== FIRST CALLER: create gate and refresh once =====
                _refreshInFlight = true;
                final gate = Completer<_RefreshAttemptResult>();
                _refreshGate = gate;

                late final _RefreshAttemptResult attempt;
                try {
                  attempt = await _doRefresh();
                } catch (_) {
                  attempt = const _RefreshAttemptResult.permanentFailure();
                } finally {
                  if (!gate.isCompleted) {
                    gate.complete(attempt);
                  }
                  _refreshInFlight = false;
                  _refreshGate = null;
                }

                if (attempt.status == _RefreshStatus.refreshed &&
                    attempt.response != null) {
                  final opts = err.requestOptions;
                  opts.extra['_auth_retried'] = true;
                  opts.headers['Authorization'] =
                      'Bearer ${attempt.response!.accessToken}';
                  try {
                    final response = await dio.fetch(opts);
                    return handler.resolve(response);
                  } on DioException catch (retryErr) {
                    return handler.next(retryErr);
                  }
                }

                if (attempt.status == _RefreshStatus.transientFailure) {
                  // Temporary network/server failure during refresh:
                  // Keep session & refreshToken intact so user can retry!
                  if (attempt.error is DioException) {
                    return handler.next(attempt.error as DioException);
                  }
                  return handler.next(err);
                }

                // Permanent refresh failure (revoked/expired refresh token):
                clearToken();
                clearRefreshToken();
                onUnauthorized?.call();
              } else {
                // ===== SUBSEQUENT CALLER: wait for first caller =====
                final activeGate = _refreshGate;
                if (activeGate != null) {
                  final attempt = await activeGate.future;
                  if (attempt.status == _RefreshStatus.refreshed &&
                      attempt.response != null) {
                    final opts = err.requestOptions;
                    opts.extra['_auth_retried'] = true;
                    opts.headers['Authorization'] =
                        'Bearer ${attempt.response!.accessToken}';
                    try {
                      final response = await dio.fetch(opts);
                      return handler.resolve(response);
                    } on DioException catch (retryErr) {
                      return handler.next(retryErr);
                    }
                  }
                  if (attempt.status == _RefreshStatus.transientFailure &&
                      attempt.error is DioException) {
                    return handler.next(attempt.error as DioException);
                  }
                }
              }
            } else {
              clearToken();
              onUnauthorized?.call();
            }
          }
          handler.next(err);
        },
      ),
    );
  }

  static String _resolveInitialBaseUrl() {
    if (ApiConfig.hasBaseUrl) {
      return ApiConfig.normalizedBaseUrl;
    }
    return '';
  }

  /// Current configured base URL.
  String get baseUrl => _baseUrl;

  /// Sets the active base URL dynamically.
  void setBaseUrl(String url) {
    _baseUrl = UrlUtils.normalizeUrl(url);
    ApiConfig.setRuntimeBaseUrl(_baseUrl);
    dio.options.baseUrl = _baseUrl;
  }

  /// Resets the server configuration atomically.
  ///
  /// Clears in-memory tokens and refresh gate to prevent cross-server token leakage.
  void resetServer(String newBaseUrl) {
    clearAllTokens();
    if (_refreshGate != null && !_refreshGate!.isCompleted) {
      _refreshGate!.complete(const _RefreshAttemptResult.permanentFailure());
    }
    _refreshGate = null;
    _refreshInFlight = false;
    setBaseUrl(newBaseUrl);
  }

  /// Performs a single refresh-token operation and classifies the outcome.
  Future<_RefreshAttemptResult> _doRefresh() async {
    final rt = _refreshToken;
    if (rt == null || rt.isEmpty) {
      return const _RefreshAttemptResult.permanentFailure();
    }
    final fn = onRefreshToken;
    if (fn == null) {
      return const _RefreshAttemptResult.permanentFailure();
    }
    try {
      final result = await fn(rt);
      if (result != null && result.accessToken.isNotEmpty) {
        return _RefreshAttemptResult.refreshed(result);
      }
      return const _RefreshAttemptResult.permanentFailure();
    } catch (e) {
      if (isTransientNetworkOrServerError(e)) {
        return _RefreshAttemptResult.transientFailure(e);
      }
      return const _RefreshAttemptResult.permanentFailure();
    }
  }

  String? get token => _token;

  void setToken(String? token) {
    _token = token;
    if (token != null && token.isNotEmpty) {
      dio.options.headers['Authorization'] = 'Bearer $token';
    } else {
      dio.options.headers.remove('Authorization');
    }
  }

  String? get refreshToken => _refreshToken;

  void setRefreshToken(String? token) {
    _refreshToken = token;
  }

  void clearToken() {
    _token = null;
    dio.options.headers.remove('Authorization');
  }

  void clearRefreshToken() {
    _refreshToken = null;
  }

  void clearAllTokens() {
    clearToken();
    clearRefreshToken();
  }

  void handleUnauthorized() {
    clearAllTokens();
    onUnauthorized?.call();
  }
}

enum _RefreshStatus {
  refreshed,
  permanentFailure,
  transientFailure,
}

class _RefreshAttemptResult {
  final _RefreshStatus status;
  final LoginResponse? response;
  final Object? error;

  const _RefreshAttemptResult.refreshed(this.response)
    : status = _RefreshStatus.refreshed,
      error = null;

  const _RefreshAttemptResult.permanentFailure()
    : status = _RefreshStatus.permanentFailure,
      response = null,
      error = null;

  const _RefreshAttemptResult.transientFailure(this.error)
    : status = _RefreshStatus.transientFailure,
      response = null;
}

final apiClientProvider = Provider<ApiClient>((ref) {
  return ApiClient();
});
