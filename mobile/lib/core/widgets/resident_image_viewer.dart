import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/client.dart';
import '../theme/app_colors.dart';
import 'community_image_resolution.dart';

/// Full-screen pinch-to-zoom profile-image viewer.
///
/// Authorization pipeline is identical to [CommunityImage] (signed URL for
/// warga-visible content). Only opens when an avatar_path is present — the
/// caller (resident avatar) is responsible for invoking it only for residents
/// with an actual stored photo.
class ResidentImageViewer extends ConsumerStatefulWidget {
  final String imageRef;
  final String title;

  const ResidentImageViewer({
    super.key,
    required this.imageRef,
    this.title = 'Foto Profil',
  });

  @override
  ConsumerState<ResidentImageViewer> createState() =>
      _ResidentImageViewerState();
}

class _ResidentImageViewerState extends ConsumerState<ResidentImageViewer> {
  Future<String?>? _resolveFuture;

  @override
  void initState() {
    super.initState();
    final dio = ref.read(apiClientProvider).dio;
    _resolveFuture = resolveAuthorizedImageUrl(
      ref: ref,
      dio: dio,
      imageRef: widget.imageRef,
    );
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    return Scaffold(
      backgroundColor: Colors.black,
      body: SafeArea(
        child: FutureBuilder<String?>(
          future: _resolveFuture,
          builder: (context, snapshot) {
            if (snapshot.connectionState != ConnectionState.done) {
              return _Loading(title: widget.title);
            }
            final url = snapshot.data;
            if (url == null) {
              return _ErrorState(title: widget.title);
            }
            return _ImageContent(
              url: url,
              title: widget.title,
              isDark: isDark,
            );
          },
        ),
      ),
      floatingActionButton: Padding(
        padding: const EdgeInsets.only(top: 12, right: 12),
        child: Align(
          alignment: Alignment.topRight,
          child: Material(
            color: Colors.black54,
            shape: const CircleBorder(),
            child: IconButton(
              icon: const Icon(Icons.close, color: Colors.white),
              onPressed: () => Navigator.of(context).pop(),
              tooltip: 'Tutup',
            ),
          ),
        ),
      ),
    );
  }
}

class _Loading extends StatelessWidget {
  final String title;
  const _Loading({required this.title});

  @override
  Widget build(BuildContext context) {
    return Stack(
      children: [
        const Center(child: CircularProgressIndicator()),
        Positioned(
          top: 12,
          left: 12,
          child: Text(
            title,
            style: const TextStyle(color: Colors.white, fontSize: 16),
          ),
        ),
      ],
    );
  }
}

class _ErrorState extends StatelessWidget {
  final String title;
  const _ErrorState({required this.title});

  @override
  Widget build(BuildContext context) {
    return Stack(
      children: [
        const Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(Icons.broken_image_outlined,
                  size: 72, color: Colors.white70),
              SizedBox(height: 12),
              Text(
                'Gagal memuat foto profil.',
                style: TextStyle(color: Colors.white70, fontSize: 14),
              ),
            ],
          ),
        ),
        Positioned(
          top: 12,
          left: 12,
          child: Text(
            title,
            style: const TextStyle(color: Colors.white, fontSize: 16),
          ),
        ),
      ],
    );
  }
}

class _ImageContent extends StatelessWidget {
  final String url;
  final String title;
  final bool isDark;

  const _ImageContent({
    required this.url,
    required this.title,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    return Stack(
      children: [
        InteractiveViewer(
          minScale: 0.5,
          maxScale: 4,
          child: Center(
            child: Image.network(
              url,
              fit: BoxFit.contain,
              loadingBuilder: (context, child, progress) {
                if (progress == null) return child;
                return const Center(
                  child: CircularProgressIndicator(
                    value: null,
                    color: AppColors.accent,
                  ),
                );
              },
              errorBuilder: (_, _, _) => const Center(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Icon(Icons.broken_image_outlined,
                        size: 72, color: Colors.white70),
                    SizedBox(height: 12),
                    Text(
                      'Gagal memuat foto profil.',
                      style: TextStyle(color: Colors.white70, fontSize: 14),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
        Positioned(
          top: 12,
          left: 12,
          child: Text(
            title,
            style: const TextStyle(color: Colors.white, fontSize: 16),
          ),
        ),
        Positioned(
          bottom: 12,
          left: 12,
          right: 12,
          child: const Text(
            'Cubit untuk memperbesar · Gulir untuk menggeser',
            textAlign: TextAlign.center,
            style: TextStyle(color: Colors.white54, fontSize: 12),
          ),
        ),
      ],
    );
  }
}
