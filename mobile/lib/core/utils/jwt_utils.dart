import 'dart:convert';

/// Lightweight JWT expiry inspection helper for proactive access token refresh.
///
/// Never validates signatures (server enforces cryptographic verification) and
/// safely returns `null` / `false` for opaque non-JWT tokens (such as test tokens).
class JwtUtils {
  JwtUtils._();

  /// Parses the `exp` claim (UTC) from a standard 3-segment JWT string.
  /// Returns `null` if [token] is empty, malformed, or not a JWT with numeric `exp`.
  static DateTime? parseExpiry(String? token) {
    if (token == null) return null;
    final trimmed = token.trim();
    if (trimmed.isEmpty) return null;
    final parts = trimmed.split('.');
    if (parts.length != 3) return null;

    try {
      final normalized = base64Url.normalize(parts[1]);
      final decoded = utf8.decode(base64Url.decode(normalized));
      final payload = jsonDecode(decoded);
      if (payload is! Map) return null;
      final exp = payload['exp'];
      if (exp is int) {
        return DateTime.fromMillisecondsSinceEpoch(exp * 1000, isUtc: true);
      }
      if (exp is num) {
        return DateTime.fromMillisecondsSinceEpoch(
          exp.toInt() * 1000,
          isUtc: true,
        );
      }
      return null;
    } catch (_) {
      return null;
    }
  }

  /// Returns `true` when [token] is a parseable JWT whose `exp` is already in
  /// the past or within [buffer] of [now] (UTC).
  ///
  /// Returns `false` for non-JWT opaque tokens so callers fall back to normal
  /// server verification / 401 refresh handling.
  static bool isExpiredOrExpiringSoon(
    String? token, {
    Duration buffer = const Duration(minutes: 2),
    DateTime? now,
  }) {
    final expiry = parseExpiry(token);
    if (expiry == null) return false;
    final current = (now ?? DateTime.now()).toUtc();
    return !expiry.isAfter(current.add(buffer));
  }
}
