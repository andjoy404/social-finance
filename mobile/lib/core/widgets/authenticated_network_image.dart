import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../api/client.dart';
import '../api/config.dart';

class AuthenticatedNetworkImage extends ConsumerWidget {
  final String imageUrl;
  final double? width;
  final double? height;
  final BoxFit fit;
  final Widget? errorWidget;

  const AuthenticatedNetworkImage({
    super.key,
    required this.imageUrl,
    this.width,
    this.height,
    this.fit = BoxFit.cover,
    this.errorWidget,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final trimmed = imageUrl.trim();
    if (trimmed.isEmpty) {
      return _buildFallback();
    }

    final apiClient = ref.watch(apiClientProvider);
    String resolved = ApiConfig.resolveImageUrl(trimmed, fallbackBaseUrl: apiClient.baseUrl) ?? '';
    if (resolved.isEmpty) {
      if (trimmed.startsWith('http://') || trimmed.startsWith('https://')) {
        resolved = trimmed;
      } else if (trimmed.startsWith('/')) {
        final base = apiClient.baseUrl.trim();
        final cleanBase = base.endsWith('/') ? base.substring(0, base.length - 1) : base;
        resolved = '$cleanBase$trimmed';
      }
    }

    if (resolved.isEmpty) {
      return _buildFallback();
    }

    final headers = <String, String>{};
    final token = apiClient.token;
    if (token != null && token.isNotEmpty) {
      headers['Authorization'] = 'Bearer $token';
    }

    return Image.network(
      resolved,
      headers: headers.isNotEmpty ? headers : null,
      width: width,
      height: height,
      fit: fit,
      errorBuilder: (context, error, stackTrace) =>
          errorWidget ?? _buildFallback(),
      loadingBuilder: (context, child, loadingProgress) {
        if (loadingProgress == null) return child;
        return SizedBox(
          width: width,
          height: height,
          child: const Center(
            child: SizedBox(
              width: 24,
              height: 24,
              child: CircularProgressIndicator(strokeWidth: 2),
            ),
          ),
        );
      },
    );
  }

  Widget _buildFallback() {
    return errorWidget ??
        Container(
          width: width,
          height: height ?? 140,
          color: Colors.grey.shade200,
          child: const Center(
            child: Icon(Icons.broken_image, color: Colors.grey),
          ),
        );
  }
}
