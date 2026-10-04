import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'config.dart';
import 'models.dart';

class ApiClient {
  final Dio dio;
  String? _token;
  String? _refreshToken;
  void Function()? onUnauthorized;
  Future<LoginResponse?> Function(String refreshToken)? onRefreshToken;

  // Completer-based single-flight gate.
  //
  // When `null` there is no refresh in flight — the next 401 creates a
  // new Completer and proceeds with the refresh.
  //
  // When non-null, the gate represents an *in-flight* refresh.  Exactly
  // one caller (the first one to arrive when the gate was null)
  // performs the refresh; all subsequent callers simply await
  // `gate.future`.
  //
  // After the refresh completes (success or failure) the gate is cleared
  // to `_refreshGate = null`, so that a later 401 can start a fresh
  // refresh cycle with the updated refresh token.
  //
  // `_refreshInFlight` prevents a new refresh cycle from starting the
  // microsecond between the previous refresh completing and the code
  // clearing `_refreshGate`.
  Completer<LoginResponse?>? _refreshGate;
  bool _refreshInFlight = false;

  ApiClient({
    Dio? customDio,
    String? baseUrl,
    this.onUnauthorized,
    this.onRefreshToken,
  })
    : dio =
          customDio ??
          Dio(
            BaseOptions(
              baseUrl: baseUrl ?? _resolveInitialBaseUrl(),
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
            if (!ApiConfig.hasBaseUrl) {
              return handler.reject(
                DioException(
                  requestOptions: options,
                  error: const ApiConfigException(),
                  type: DioExceptionType.unknown,
                ),
              );
            }
            options.baseUrl = ApiConfig.normalizedBaseUrl;
          }
          if (_token != null && _token!.isNotEmpty) {
            options.headers['Authorization'] = 'Bearer $_token';
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
              // --- Single-flight refresh gate ---
              //
              // A Completer is a synchronous primitive: creating it and
              // assigning it to _refreshGate happens before any `await`,
              // so no other event-loop iteration can slip in between
              // "seeing that a refresh is in-flight" and "waiting on it".
              //
              // FIRST CALLER (gate is null): creates the gate, performs
              // the refresh, resolves it, then clears the gate.
              //
              // SUBSEQUENT CALLER (gate is non-null): simply awaits the
              // gate's future, then retries with the new token or logs out.
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

                // Clean up the gate so a later 401 can start a new
                // refresh cycle with the updated refresh token.
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

  /// Performs a single refresh-token operation.
  ///
  /// Returns the new [LoginResponse] on success, or `null` if the refresh
  /// failed (expired token, network error, etc.).  Callers are responsible
  /// for clearing tokens and calling [onUnauthorized] on failure.
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
  }

  String? get refreshToken => _refreshToken;

  void setRefreshToken(String? token) {
    _refreshToken = token;
  }

  void clearToken() {
    _token = null;
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
