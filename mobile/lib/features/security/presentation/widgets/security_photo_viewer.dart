import 'package:flutter/material.dart';
import 'package:social_finance/core/widgets/authenticated_network_image.dart';

class SecurityPhotoViewerItem {
  final String imageUrl;
  final String title;
  final String? subtitle;

  const SecurityPhotoViewerItem({
    required this.imageUrl,
    required this.title,
    this.subtitle,
  });
}

class SecurityPhotoViewerScreen extends StatefulWidget {
  final List<SecurityPhotoViewerItem> photos;
  final int initialIndex;

  const SecurityPhotoViewerScreen({
    super.key,
    required this.photos,
    this.initialIndex = 0,
  });

  static Future<void> show(
    BuildContext context, {
    required List<SecurityPhotoViewerItem> photos,
    int initialIndex = 0,
  }) {
    if (photos.isEmpty) return Future.value();
    return Navigator.of(context, rootNavigator: true).push(
      PageRouteBuilder(
        opaque: false,
        barrierDismissible: true,
        pageBuilder: (ctx, anim1, anim2) => SecurityPhotoViewerScreen(
          photos: photos,
          initialIndex: initialIndex,
        ),
        transitionsBuilder: (ctx, anim1, anim2, child) =>
            FadeTransition(opacity: anim1, child: child),
      ),
    );
  }

  @override
  State<SecurityPhotoViewerScreen> createState() =>
      _SecurityPhotoViewerScreenState();
}

class _SecurityPhotoViewerScreenState extends State<SecurityPhotoViewerScreen> {
  late final PageController _pageController;
  late int _currentIndex;

  @override
  void initState() {
    super.initState();
    _currentIndex = widget.initialIndex.clamp(0, widget.photos.length - 1);
    _pageController = PageController(initialPage: _currentIndex);
  }

  @override
  void dispose() {
    _pageController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (widget.photos.isEmpty) {
      return const SizedBox.shrink();
    }
    final current = widget.photos[_currentIndex];

    return Scaffold(
      backgroundColor: Colors.black.withOpacity(0.92),
      body: Stack(
        children: [
          // Swipeable Photo Gallery
          PageView.builder(
            controller: _pageController,
            itemCount: widget.photos.length,
            onPageChanged: (idx) {
              setState(() => _currentIndex = idx);
            },
            itemBuilder: (ctx, idx) {
              final item = widget.photos[idx];
              return Center(
                child: InteractiveViewer(
                  minScale: 0.8,
                  maxScale: 4.0,
                  child: AuthenticatedNetworkImage(
                    imageUrl: item.imageUrl,
                    fit: BoxFit.contain,
                  ),
                ),
              );
            },
          ),

          // Top Bar with Counter and Close Button
          Positioned(
            top: MediaQuery.of(context).padding.top + 8,
            left: 16,
            right: 16,
            child: Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                if (widget.photos.length > 1)
                  Container(
                    padding:
                        const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                    decoration: BoxDecoration(
                      color: Colors.black54,
                      borderRadius: BorderRadius.circular(16),
                    ),
                    child: Text(
                      '${_currentIndex + 1} / ${widget.photos.length}',
                      style: const TextStyle(
                        color: Colors.white,
                        fontSize: 13,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  )
                else
                  const SizedBox.shrink(),
                Material(
                  color: Colors.black54,
                  shape: const CircleBorder(),
                  child: IconButton(
                    icon: const Icon(Icons.close, color: Colors.white),
                    tooltip: 'Tutup',
                    onPressed: () => Navigator.of(context).pop(),
                  ),
                ),
              ],
            ),
          ),

          // Left Arrow
          if (widget.photos.length > 1 && _currentIndex > 0)
            Positioned(
              left: 12,
              top: 0,
              bottom: 0,
              child: Center(
                child: Material(
                  color: Colors.black45,
                  shape: const CircleBorder(),
                  child: IconButton(
                    icon: const Icon(Icons.chevron_left,
                        color: Colors.white, size: 28),
                    onPressed: () {
                      _pageController.previousPage(
                        duration: const Duration(milliseconds: 250),
                        curve: Curves.easeInOut,
                      );
                    },
                  ),
                ),
              ),
            ),

          // Right Arrow
          if (widget.photos.length > 1 &&
              _currentIndex < widget.photos.length - 1)
            Positioned(
              right: 12,
              top: 0,
              bottom: 0,
              child: Center(
                child: Material(
                  color: Colors.black45,
                  shape: const CircleBorder(),
                  child: IconButton(
                    icon: const Icon(Icons.chevron_right,
                        color: Colors.white, size: 28),
                    onPressed: () {
                      _pageController.nextPage(
                        duration: const Duration(milliseconds: 250),
                        curve: Curves.easeInOut,
                      );
                    },
                  ),
                ),
              ),
            ),

          // Bottom Information Bar
          Positioned(
            bottom: MediaQuery.of(context).padding.bottom + 16,
            left: 20,
            right: 20,
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
              decoration: BoxDecoration(
                color: Colors.black54,
                borderRadius: BorderRadius.circular(10),
              ),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    current.title,
                    style: const TextStyle(
                      color: Colors.white,
                      fontSize: 15,
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                  if (current.subtitle != null && current.subtitle!.isNotEmpty) ...[
                    const SizedBox(height: 4),
                    Text(
                      current.subtitle!,
                      style: const TextStyle(
                        color: Colors.white70,
                        fontSize: 12,
                      ),
                    ),
                  ],
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}
