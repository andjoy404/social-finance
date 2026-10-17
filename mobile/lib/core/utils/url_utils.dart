/// Utilities for validating and normalizing API server URLs.
class UrlUtils {
  UrlUtils._();

  /// Normalizes an API server URL.
  ///
  /// - Trims outside whitespace.
  /// - Strips trailing slashes.
  /// - Strips trailing `/api/v1` (case-insensitive) to prevent path duplication,
  ///   as repositories and services append `/api/v1/...` themselves.
  static String normalizeUrl(String input) {
    var trimmed = input.trim();
    if (trimmed.isEmpty) return '';

    // Remove any trailing slashes
    while (trimmed.endsWith('/')) {
      trimmed = trimmed.substring(0, trimmed.length - 1).trim();
    }

    // If user provided a path ending in /api/v1 or /api/v1/, strip it
    final lower = trimmed.toLowerCase();
    if (lower.endsWith('/api/v1')) {
      trimmed = trimmed.substring(0, trimmed.length - '/api/v1'.length);
      while (trimmed.endsWith('/')) {
        trimmed = trimmed.substring(0, trimmed.length - 1).trim();
      }
    }

    return trimmed;
  }

  /// Validates a raw API server URL string.
  ///
  /// Returns `null` if the URL is valid, or a descriptive error message in
  /// Indonesian if invalid.
  static String? validateUrl(String? raw) {
    if (raw == null || raw.trim().isEmpty) {
      return 'URL API / Server tidak boleh kosong.';
    }

    final trimmed = raw.trim();
    final uri = Uri.tryParse(trimmed);
    if (uri == null) {
      return 'Format URL tidak valid.';
    }

    final scheme = uri.scheme.toLowerCase();
    if (scheme.isEmpty) {
      return 'URL harus menyertakan protokol (misal: https://).';
    }

    if (scheme != 'http' && scheme != 'https') {
      return 'Hanya protokol HTTP dan HTTPS yang didukung.';
    }

    if (uri.userInfo.isNotEmpty) {
      return 'URL server tidak boleh memuat username atau password.';
    }

    if (uri.hasQuery) {
      return 'URL server tidak boleh memuat query parameter (?query).';
    }

    if (uri.hasFragment) {
      return 'URL server tidak boleh memuat fragmen (#fragmen).';
    }

    final host = uri.host.trim();
    if (host.isEmpty) {
      return 'Host URL server tidak boleh kosong.';
    }

    // HTTPS is required for production.
    // HTTP is only allowed for local development environments.
    if (scheme == 'http') {
      if (!_isAllowedDevelopmentHost(host)) {
        return 'URL server produksi wajib menggunakan HTTPS.';
      }
    }

    return null;
  }

  /// Determines if [host] is an explicitly permitted local development host.
  static bool _isAllowedDevelopmentHost(String host) {
    final lower = host.toLowerCase();

    // Standard dev loopbacks
    if (lower == 'localhost' ||
        lower == '127.0.0.1' ||
        lower == '::1' ||
        lower == '10.0.2.2' || // Android emulator loopback to host
        lower == '10.0.3.2') {
      // Genymotion loopback
      return true;
    }

    // Local network suffixes
    if (lower.endsWith('.local') || lower.endsWith('.internal')) {
      return true;
    }

    // Private IPv4 ranges (e.g. 192.168.x.x, 10.x.x.x, 172.16-31.x.x)
    final ipv4Parts = lower.split('.');
    if (ipv4Parts.length == 4) {
      final p0 = int.tryParse(ipv4Parts[0]);
      final p1 = int.tryParse(ipv4Parts[1]);
      final p2 = int.tryParse(ipv4Parts[2]);
      final p3 = int.tryParse(ipv4Parts[3]);
      if (p0 != null && p1 != null && p2 != null && p3 != null) {
        if (p0 == 10) return true;
        if (p0 == 192 && p1 == 168) return true;
        if (p0 == 172 && p1 >= 16 && p1 <= 31) return true;
      }
    }

    return false;
  }
}
