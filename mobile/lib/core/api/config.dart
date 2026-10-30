class ApiConfig {
  // Optional environment fallback, NOT mandatory.
  static const String baseUrl = String.fromEnvironment(
    'API_BASE_URL',
    defaultValue: '',
  );

  static String _runtimeBaseUrl = '';

  static void setRuntimeBaseUrl(String url) {
    final trimmed = url.trim();
    _runtimeBaseUrl = trimmed.endsWith('/')
        ? trimmed.substring(0, trimmed.length - 1)
        : trimmed;
  }

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

  /// Safely resolves a potentially relative image URL against [fallbackBaseUrl]
  /// or [normalizedBaseUrl] without throwing [ApiConfigException].
  /// Returns `null` if the URL is empty, invalid, or contains path traversal.
  static String? resolveImageUrl(String? rawUrl, {String? fallbackBaseUrl}) {
    if (rawUrl == null) return null;
    final trimmed = rawUrl.trim();
    if (trimmed.isEmpty) return null;
    if (trimmed.contains('..') || trimmed.contains(r'\')) return null;

    String candidate = trimmed;
    if (trimmed.startsWith('/')) {
      String base = (fallbackBaseUrl ?? '').trim();
      if (base.isEmpty && _runtimeBaseUrl.trim().isNotEmpty) {
        base = _runtimeBaseUrl.trim();
      }
      if (base.isEmpty && hasBaseUrl) {
        base = normalizedBaseUrl;
      }
      if (base.isEmpty) return null;
      if (base.endsWith('/')) {
        base = base.substring(0, base.length - 1);
      }
      candidate = '$base$trimmed';
    }

    final uri = Uri.tryParse(candidate);
    if (uri == null) return null;
    final scheme = uri.scheme.toLowerCase();
    if ((scheme != 'http' && scheme != 'https') || uri.host.trim().isEmpty) {
      return null;
    }
    return candidate;
  }
}

class ApiConfigException implements Exception {
  final String message;
  const ApiConfigException([
    this.message =
        'API_BASE_URL belum dikonfigurasi. Masukkan URL server pada halaman login.',
  ]);

  @override
  String toString() => message;
}
