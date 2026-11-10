import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../features/auth/data/mock_auth_repository.dart';
import '../api/client.dart';
import '../utils/permissions.dart';

/// Renders a community image that lives behind the authorized image endpoint.
///
/// Resolution order:
///  1. Short-lived signed URL — only serves warga-visible content, so
///     draft previews for managers do NOT work this way.
///  2. Bearer fetch with authorization header — only for users who may
///     manage activities or announcements; image bytes are kept in memory.
///  3. Neutral placeholder otherwise.
///
/// Lifecycle: results carry their own [ImageProvider]. On asset replacement,
/// user/RT/permission/role changes, and disposal, the previous provider is
/// evicted from Flutter's [PaintingBinding] image cache so stale private
/// images never re-appear. The widget is resilient to stale async results
/// completing after the asset or auth state changed (generation guard).
class CommunityImage extends ConsumerStatefulWidget {
  final String imageRef; // stored ref: full API path or raw storage key
  final double? width;
  final double? height;
  final BoxFit fit;
  final String? alt;

  const CommunityImage({
    super.key,
    required this.imageRef,
    this.width,
    this.height,
    this.fit = BoxFit.cover,
    this.alt,
  });

  @override
  ConsumerState<CommunityImage> createState() => _CommunityImageState();
}

class _CommunityImageState extends ConsumerState<CommunityImage> {
  int _generation = 0;
  _ImageResult? _result;
  bool _isLoading = true;
  late final ProviderSubscription<dynamic> _authSub;

  @override
  void initState() {
    super.initState();
    _startResolve();
    _authSub = ref.listenManual(authRepositoryProvider, (prev, next) {
      if (_scopeKey(_scopeUser(prev)) != _scopeKey(_scopeUser(next))) {
        _evictCurrentProvider();
        _startResolve();
      }
    });
  }

  @override
  void didUpdateWidget(covariant CommunityImage oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.imageRef != widget.imageRef) {
      _evictCurrentProvider();
      _startResolve();
    }
  }

  @override
  void dispose() {
    _authSub.close();
    _evictCurrentProvider();
    super.dispose();
  }

  Map<String, dynamic>? _scopeUser(dynamic state) {
    if (state is AsyncData) {
      return state.value as Map<String, dynamic>?;
    }
    return null;
  }

  String _scopeKey(Map<String, dynamic>? user) {
    if (user == null) return 'null';
    final perms =
        (user['effective_permissions'] as List?)?.cast<String>().join(',') ??
        '';
    return '${user['id']}|${user['rt_id']}|$perms';
  }

  void _evictCurrentProvider() {
    final provider = _result?.provider;
    if (provider != null) {
      PaintingBinding.instance.imageCache?.evict(provider);
    }
  }

  void _startResolve() {
    final gen = ++_generation;
    setState(() {
      _result = null;
      _isLoading = true;
    });
    _resolve().then((result) {
      if (!mounted || gen != _generation) {
        // Stale result for an older asset/scope: discard eagerly.
        if (result?.provider != null) {
          PaintingBinding.instance.imageCache?.evict(result!.provider!);
        }
        return;
      }
      setState(() {
        _result = result;
        _isLoading = false;
      });
    });
  }

  String _extractKey(String ref) {
    const marker = '/api/v1/community/images/';
    final idx = ref.indexOf(marker);
    final trimmed = ref.trim();
    final raw = idx >= 0 ? trimmed.substring(idx + marker.length) : trimmed;
    return raw.replaceFirst(RegExp(r'^/+'), '');
  }

  Future<_ImageResult?> _resolve() async {
    final key = _extractKey(widget.imageRef);
    if (key.isEmpty) return null;

    final apiClient = ref.read(apiClientProvider);

    // 1. Signed URL (warga-visible content only).
    try {
      final response = await apiClient.dio.post(
        '/api/v1/community/images/sign',
        data: {'key': key},
      );
      final data = response.data;
      if (data is Map && data['url'] is String) {
        final url = (data['url'] as String).trim();
        if (url.isNotEmpty) return _ImageResult.network(url);
      }
    } catch (_) {
      // fall through to Bearer fallback for managers
    }

    // 2. Bearer fetch for content managers (draft/archived content).
    final user = _scopeUser(ref.read(authRepositoryProvider));
    final canManage = canManageActivities(user) || canManageAnnouncements(user);
    if (!canManage) return null;
    try {
      final response = await apiClient.dio.get<List<int>>(
        '/api/v1/community/images/$key',
        options: Options(responseType: ResponseType.bytes),
      );
      final bytes = response.data;
      if (bytes != null && bytes.isNotEmpty) {
        return _ImageResult.memory(Uint8List.fromList(bytes));
      }
      return null;
    } catch (_) {
      return null;
    }
  }

  @override
  Widget build(BuildContext context) {
    final placeholder = Container(
      width: widget.width,
      height: widget.height,
      decoration: BoxDecoration(
        color: Theme.of(context).dividerColor.withValues(alpha: 0.25),
        borderRadius: BorderRadius.circular(8),
      ),
      child: const Icon(Icons.image_outlined, size: 28),
    );

    if (_isLoading || _result == null) {
      return placeholder;
    }

    final provider = _result!.provider!;
    return Image(
      image: provider,
      width: widget.width,
      height: widget.height,
      fit: widget.fit,
      semanticLabel: widget.alt,
      errorBuilder: (_, __, ___) => placeholder,
    );
  }
}

class _ImageResult {
  final ImageProvider? provider;

  _ImageResult.network(String url) : provider = NetworkImage(url);
  _ImageResult.memory(Uint8List bytes) : provider = MemoryImage(bytes);
}
