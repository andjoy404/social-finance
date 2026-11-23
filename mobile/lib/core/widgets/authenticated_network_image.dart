import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../features/auth/data/mock_auth_repository.dart';
import '../api/client.dart';
import '../api/config.dart';

/// Displays an image fetched through the authenticated [ApiClient] (Dio).
///
/// - Sends `Authorization: Bearer <token>` via [ApiClient]'s interceptor and
///   automatically retries after a 401 refresh-token rotation.
/// - Attaches `X-RT-ID` when an active RT context is present (never in URL).
/// - Cancels in-flight requests via [CancelToken] when disposed or when the URL
///   changes to prevent race conditions and leaks.
/// - Renders [errorWidget] only when loading genuinely fails.
class AuthenticatedNetworkImage extends ConsumerStatefulWidget {
  final String imageUrl;
  final double? width;
  final double? height;
  final BoxFit fit;
  final Widget? errorWidget;
  final String? rtId;

  const AuthenticatedNetworkImage({
    super.key,
    required this.imageUrl,
    this.width,
    this.height,
    this.fit = BoxFit.cover,
    this.errorWidget,
    this.rtId,
  });

  @override
  ConsumerState<AuthenticatedNetworkImage> createState() =>
      _AuthenticatedNetworkImageState();
}

class _AuthenticatedNetworkImageState
    extends ConsumerState<AuthenticatedNetworkImage> {
  Uint8List? _bytes;
  bool _loading = true;
  bool _failed = false;
  CancelToken? _cancelToken;
  String? _loadedKey;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) {
        _loadImage();
      }
    });
  }

  @override
  void didUpdateWidget(covariant AuthenticatedNetworkImage oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.imageUrl != widget.imageUrl || oldWidget.rtId != widget.rtId) {
      _loadImage();
    }
  }

  @override
  void dispose() {
    _cancelToken?.cancel('disposed');
    _cancelToken = null;
    super.dispose();
  }

  String? _resolveUrl(ApiClient apiClient) {
    final trimmed = widget.imageUrl.trim();
    if (trimmed.isEmpty) return null;

    String resolved =
        ApiConfig.resolveImageUrl(trimmed, fallbackBaseUrl: apiClient.baseUrl) ??
        '';
    if (resolved.isEmpty) {
      if (trimmed.startsWith('http://') || trimmed.startsWith('https://')) {
        resolved = trimmed;
      } else if (trimmed.startsWith('/')) {
        final base = apiClient.baseUrl.trim();
        if (base.isNotEmpty) {
          var cleanBase = base.endsWith('/')
              ? base.substring(0, base.length - 1)
              : base;
          if (cleanBase.endsWith('/api/v1') &&
              (trimmed == '/api/v1' || trimmed.startsWith('/api/v1/'))) {
            cleanBase = cleanBase.substring(
              0,
              cleanBase.length - '/api/v1'.length,
            );
          }
          resolved = '$cleanBase$trimmed';
        }
      }
    }
    if (resolved.isEmpty) return null;
    return resolved.replaceAll(
      RegExp(r'(\/api\/v1)(?:\/api\/v1)+(?=\/|$)'),
      '/api/v1',
    );
  }

  String? _resolveRtId() {
    final explicit = widget.rtId?.trim();
    if (explicit != null && explicit.isNotEmpty) return explicit;
    try {
      final user = ref.read(authRepositoryProvider)?.valueOrNull;
      final rtId =
          (user?['rt_id'] ?? (user?['rt'] is Map ? user!['rt']['id'] : null))
              ?.toString()
              .trim();
      if (rtId != null && rtId.isNotEmpty) return rtId;
    } catch (_) {
      // Provider may not be overridden in isolated widget tests.
    }
    return null;
  }

  Future<void> _loadImage() async {
    final apiClient = ref.read(apiClientProvider);
    final resolved = _resolveUrl(apiClient);
    final rtId = _resolveRtId();
    final token = apiClient.token ?? '';
    final requestKey = '$resolved|$rtId|$token';

    if (resolved == null || resolved.isEmpty) {
      _cancelToken?.cancel('empty_url');
      _cancelToken = null;
      if (mounted) {
        setState(() {
          _bytes = null;
          _loading = false;
          _failed = true;
          _loadedKey = null;
        });
      }
      return;
    }

    if (_loadedKey == requestKey && _bytes != null && !_failed) {
      return;
    }

    _cancelToken?.cancel('url_changed');
    final cancelToken = CancelToken();
    _cancelToken = cancelToken;

    if (mounted) {
      setState(() {
        _loading = true;
        _failed = false;
      });
    }

    try {
      final headers = <String, dynamic>{
        'Accept': 'image/*',
        if (rtId != null && rtId.isNotEmpty) 'X-RT-ID': rtId,
      };
      final response = await apiClient.dio.get<List<int>>(
        resolved,
        cancelToken: cancelToken,
        options: Options(
          responseType: ResponseType.bytes,
          headers: headers,
          receiveTimeout: const Duration(seconds: 15),
        ),
      );
      if (!mounted || cancelToken.isCancelled) return;
      final data = response.data;
      if (data == null || data.isEmpty) {
        setState(() {
          _bytes = null;
          _loading = false;
          _failed = true;
          _loadedKey = null;
        });
        return;
      }
      setState(() {
        _bytes = Uint8List.fromList(data);
        _loading = false;
        _failed = false;
        _loadedKey = '$resolved|$rtId|${apiClient.token ?? ''}';
      });
    } on DioException catch (e) {
      if (CancelToken.isCancel(e) || !mounted || cancelToken.isCancelled) {
        return;
      }
      setState(() {
        _bytes = null;
        _loading = false;
        _failed = true;
        _loadedKey = null;
      });
    } catch (_) {
      if (!mounted || cancelToken.isCancelled) return;
      setState(() {
        _bytes = null;
        _loading = false;
        _failed = true;
        _loadedKey = null;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    // Watch apiClientProvider so changes to baseUrl/client trigger rebuild.
    final apiClient = ref.watch(apiClientProvider);
    final resolved = _resolveUrl(apiClient);
    final rtId = _resolveRtId();
    final token = apiClient.token ?? '';
    final desiredKey = '$resolved|$rtId|$token';
    if (_loadedKey != null &&
        _loadedKey != desiredKey &&
        !_loading &&
        resolved != null) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) _loadImage();
      });
    }

    if (_loading) {
      return SizedBox(
        width: widget.width,
        height: widget.height ?? 140,
        child: const Center(
          child: SizedBox(
            width: 24,
            height: 24,
            child: CircularProgressIndicator(strokeWidth: 2),
          ),
        ),
      );
    }

    if (_failed || _bytes == null || _bytes!.isEmpty) {
      return _buildFallback();
    }

    return Image.memory(
      _bytes!,
      width: widget.width,
      height: widget.height,
      fit: widget.fit,
      errorBuilder: (context, error, stackTrace) => _buildFallback(),
    );
  }

  Widget _buildFallback() {
    return widget.errorWidget ??
        Container(
          width: widget.width,
          height: widget.height ?? 140,
          color: Colors.grey.shade200,
          child: const Center(
            child: Icon(Icons.broken_image, color: Colors.grey),
          ),
        );
  }
}
