import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/client.dart';
import '../api/config.dart';

/// Shared helper to resolve a community image reference into an authorized
/// signed URL. Used by the full-screen image viewer and the avatar pipeline.
///
/// Signed URLs are warga-visibility level and expire in 15 minutes. Draft
/// content for managers continues to use the Bearer-blob path in
/// CommunityImage itself.
Future<String?> resolveAuthorizedImageUrl({
  required WidgetRef ref,
  required Dio dio,
  required String imageRef,
}) async {
  final key = _extractKey(imageRef);
  if (key.isEmpty) return null;

  try {
    final response = await dio.post(
      '/api/v1/community/images/sign',
      data: {'key': key},
    );
    final data = response.data;
    if (data is Map && data['url'] is String) {
      final url = (data['url'] as String).trim();
      if (url.isNotEmpty) {
        if (!url.startsWith('http://') && !url.startsWith('https://')) {
          final base = _resolveImageBaseUrl(ref);
          if (base == null) return null;
          final path = url.startsWith('/') ? url : '/$url';
          return '$base$path';
        }
        return url;
      }
    }
  } catch (_) {
    return null;
  }
  return null;
}

/// Resolve the base URL for image requests without throwing.
///
/// Development builds frequently omit the compile-time `API_BASE_URL`
/// (`String.fromEnvironment`) and rely on the runtime server URL chosen on
/// the login screen instead; `ApiConfig.normalizedBaseUrl` throws in that
/// setup, which previously swallowed the whole sign-flow into an error
/// state on Android ("Gagal memuat foto profil.") even though avatars
/// rendered through the API client (which carries the runtime URL) worked
/// fine. Mirror the API client's resolution here.
String? _resolveImageBaseUrl(WidgetRef ref) {
  try {
    final fromClient = ref.read(apiClientProvider).baseUrl.trim();
    if (fromClient.isNotEmpty) {
      return fromClient.replaceAll(RegExp(r'/+$'), '');
    }
  } catch (_) {
    // fall through to compile-time config
  }
  if (ApiConfig.hasBaseUrl) {
    return ApiConfig.normalizedBaseUrl.replaceAll(RegExp(r'/+$'), '');
  }
  return null;
}

String _extractKey(String ref) {
  const marker = '/api/v1/community/images/';
  final idx = ref.indexOf(marker);
  final trimmed = ref.trim();
  final raw = idx >= 0 ? trimmed.substring(idx + marker.length) : trimmed;
  return raw.replaceFirst(RegExp(r'^/+'), '');
}
