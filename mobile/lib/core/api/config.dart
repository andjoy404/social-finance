class ApiConfig {
  static const String baseUrl = String.fromEnvironment(
    'API_BASE_URL',
    defaultValue: '',
  );

  static const Duration connectTimeout = Duration(seconds: 10);
  static const Duration receiveTimeout = Duration(seconds: 10);
  static const Duration sendTimeout = Duration(seconds: 10);

  static bool get hasBaseUrl => baseUrl.trim().isNotEmpty;

  static String get normalizedBaseUrl {
    final trimmed = baseUrl.trim();
    if (trimmed.isEmpty) {
      throw const ApiConfigException();
    }
    return trimmed.endsWith('/')
        ? trimmed.substring(0, trimmed.length - 1)
        : trimmed;
  }
}

class ApiConfigException implements Exception {
  final String message;
  const ApiConfigException([
    this.message =
        'API_BASE_URL belum dikonfigurasi. Jalankan aplikasi dengan --dart-define=API_BASE_URL=<backend-url>.',
  ]);

  @override
  String toString() => message;
}
