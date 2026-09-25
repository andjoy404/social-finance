import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'config.dart';

class ApiClient {
  final Dio dio;
  String? _token;
  void Function()? onUnauthorized;

  ApiClient({Dio? customDio, String? baseUrl, this.onUnauthorized})
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
        onError: (DioException err, handler) {
          if (err.response?.statusCode == 401) {
            clearToken();
            final isLoginRequest = err.requestOptions.path.contains(
              '/auth/login',
            );
            if (!isLoginRequest) {
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

  String? get token => _token;

  void setToken(String? token) {
    _token = token;
  }

  void clearToken() {
    _token = null;
  }

  void handleUnauthorized() {
    clearToken();
    onUnauthorized?.call();
  }
}

final apiClientProvider = Provider<ApiClient>((ref) {
  return ApiClient();
});
