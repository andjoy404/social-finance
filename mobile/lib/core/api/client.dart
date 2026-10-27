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
  Completer<LoginResponse?>? _refreshGate;
  bool _refreshInFlight = false;

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
          if (_token != null && _token!.isNotEmpty) {
            options.headers['Authorization'] = 'Bearer $_token';
          } else {
            options.headers.remove('Authorization');
          }
          handler.next(options);
        },
        onError: (DioException err, handler) async {
          if (err.response?.statusCode == 401) {
            final isLoginRequest = err.requestOptions.path.contains(
              '/auth/login',
            );
            final isRefreshRequest = err.requestOptions.path.contains(
              '/auth/refresh',
            );

            if (isRefreshRequest) {
              // Refresh endpoint itself failed — give up.
              _refreshGate?.complete();
              _refreshGate = null;
              _refreshInFlight = false;
              clearToken();
              clearRefreshToken();
              onUnauthorized?.call();
            } else if (!isLoginRequest && _refreshToken != null) {
              if (_refreshGate == null && !_refreshInFlight) {
                // ===== FIRST CALLER: create gate and refresh =====
                _refreshInFlight = true;
                _refreshGate = Completer<LoginResponse?>();
                final gate = _refreshGate!;

                LoginResponse? result;
                try {
                  result = await _doRefresh();
                } catch (_) {
                  result = null;
                }

                gate.complete(result);

                _refreshInFlight = false;
                _refreshGate = null;

                if (result != null) {
                  // Retry the original request with the new access token.
                  final opts = err.requestOptions;
                  opts.headers['Authorization'] =
                      'Bearer ${result.accessToken}';
                  final response = await dio.fetch(opts);
                  return handler.resolve(response);
                }

                // Refresh failed — log out.
                clearToken();
                clearRefreshToken();
                onUnauthorized?.call();
              } else {
                // ===== SUBSEQUENT CALLER: wait for first caller =====
                final result = await _refreshGate!.future;

                if (result != null) {
                  final opts = err.requestOptions;
                  opts.headers['Authorization'] =
                      'Bearer ${result.accessToken}';
                  final response = await dio.fetch(opts);
                  return handler.resolve(response);
                }

                clearToken();
                clearRefreshToken();
                onUnauthorized?.call();
              }
            } else {
              clearToken();
              if (!isLoginRequest) {
                onUnauthorized?.call();
              }
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
    dio.options.baseUrl = _baseUrl;
  }

  /// Resets the server configuration atomically.
  ///
  /// Clears in-memory tokens and refresh gate to prevent cross-server token leakage.
  void resetServer(String newBaseUrl) {
    clearAllTokens();
    _refreshGate?.complete();
    _refreshGate = null;
    _refreshInFlight = false;
    setBaseUrl(newBaseUrl);
  }

  /// Performs a single refresh-token operation.
  Future<LoginResponse?> _doRefresh() async {
    final rt = _refreshToken;
    if (rt == null || rt.isEmpty) {
      return null;
    }
    final fn = onRefreshToken;
    if (fn == null) {
      return null;
    }
    try {
      return await fn(rt);
    } catch (_) {
      return null;
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

final apiClientProvider = Provider<ApiClient>((ref) {
  return ApiClient();
});
