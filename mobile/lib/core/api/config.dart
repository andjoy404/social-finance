class ApiConfig {
  /// Permanent default official Social Finance server URL.
  static const String defaultOfficialBaseUrl = 'https://api.urena0216.id';

  // Optional compile-time override via `--dart-define=API_BASE_URL=...`.
  static const String baseUrl = String.fromEnvironment(
    'API_BASE_URL',
    defaultValue: '',
  );

  /// Effective default base URL used when no user override is stored.
  static String get effectiveDefaultBaseUrl =>
      hasBaseUrl ? normalizedBaseUrl : defaultOfficialBaseUrl;

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

  static const String _internalNotificationImagePrefix =
      '/api/v1/notifications/images/';

  /// Safely resolves a potentially relative image URL against [fallbackBaseUrl]
  /// or [normalizedBaseUrl] without throwing [ApiConfigException].
  /// Returns `null` if the URL is empty, invalid, or contains path traversal.
  ///
  /// Also normalizes bare notification storage keys (`notifications/...`) and
  /// rebases internal `/api/v1/notifications/images/...` absolute URLs onto the
  /// active API base URL when available so stale server LAN IPs or cleartext
  /// `PUBLIC_BASE_URL` values do not break authenticated image loading.
  static String? resolveImageUrl(String? rawUrl, {String? fallbackBaseUrl}) {
    if (rawUrl == null) return null;
    var trimmed = rawUrl.trim();
    if (trimmed.isEmpty) return null;
    if (trimmed.contains('..') || trimmed.contains(r'\')) return null;

    if (trimmed.startsWith('notifications/')) {
      trimmed = '$_internalNotificationImagePrefix$trimmed';
    } else if (trimmed.startsWith('api/v1/')) {
      trimmed = '/$trimmed';
    }

    String base = (fallbackBaseUrl ?? '').trim();
    if (base.isEmpty && _runtimeBaseUrl.trim().isNotEmpty) {
      base = _runtimeBaseUrl.trim();
    }
    if (base.isEmpty && hasBaseUrl) {
      base = normalizedBaseUrl;
    }
    if (base.endsWith('/')) {
      base = base.substring(0, base.length - 1);
    }
    var originBase = base;
    if (originBase.endsWith('/api/v1')) {
      originBase = originBase.substring(
        0,
        originBase.length - '/api/v1'.length,
      );
    }

    String candidate = trimmed;
    if (trimmed.startsWith('/')) {
      if (base.isEmpty) return null;
      final useBase =
          (trimmed == '/api/v1' || trimmed.startsWith('/api/v1/'))
              ? originBase
              : base;
      if (useBase.isEmpty) return null;
      candidate = '$useBase$trimmed';
    } else if (originBase.isNotEmpty) {
      final parsed = Uri.tryParse(trimmed);
      final parsedPath = parsed?.path ?? '';
      final normalizedPath = parsedPath.replaceAll(
        RegExp(r'(\/api\/v1)(?:\/api\/v1)+(?=\/|$)'),
        '/api/v1',
      );
      final notifIdx = normalizedPath.indexOf(_internalNotificationImagePrefix);
      if (parsed != null &&
          (parsed.scheme.toLowerCase() == 'http' ||
              parsed.scheme.toLowerCase() == 'https') &&
          notifIdx >= 0) {
        final baseUri = Uri.tryParse(originBase);
        if (baseUri != null && baseUri.host.trim().isNotEmpty) {
          candidate = '$originBase${normalizedPath.substring(notifIdx)}';
        }
      }
    }
    candidate = candidate.replaceAll(
      RegExp(r'(\/api\/v1)(?:\/api\/v1)+(?=\/|$)'),
      '/api/v1',
    );

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
