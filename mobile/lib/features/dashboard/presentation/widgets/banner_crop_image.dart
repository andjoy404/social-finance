import 'dart:io';
import 'package:flutter/material.dart';
import 'package:social_finance/core/widgets/community_image.dart';
import 'package:social_finance/features/dashboard/data/models/community_models.dart';

/// Renders a banner image with normalized 16:9 crop geometry matching Web CSS:
///   width:  (1 / crop.width) * 100%
///   height: (1 / crop.height) * 100%
///   left:   -(crop.x / crop.width) * 100%
///   top:    -(crop.y / crop.height) * 100%
class BannerCropImage extends StatelessWidget {
  final String? imageUrl;
  final File? localFile;
  final CropRect? crop;
  final double zoom;
  final String alt;

  const BannerCropImage({
    super.key,
    this.imageUrl,
    this.localFile,
    this.crop,
    this.zoom = 1.0,
    this.alt = 'Banner image',
  });

  @override
  Widget build(BuildContext context) {
    final hasLocal = localFile != null;
    final hasRemote = imageUrl != null && imageUrl!.isNotEmpty;

    if (!hasLocal && !hasRemote) {
      return const SizedBox.shrink();
    }

    Widget buildRawImage(BoxFit fit) {
      if (hasLocal) {
        return Image.file(
          localFile!,
          fit: fit,
          errorBuilder: (context, error, stackTrace) => const Center(
            child: Icon(Icons.broken_image, color: Colors.white54, size: 32),
          ),
        );
      }
      return CommunityImage(
        imageRef: imageUrl!,
        fit: fit,
        alt: alt,
      );
    }

    if (crop == null) {
      // Legacy fallback when no explicit crop rect is stored
      return Transform.scale(
        scale: zoom.clamp(1.0, 3.0),
        child: buildRawImage(BoxFit.cover),
      );
    }

    final cropW = crop!.width.clamp(0.01, 1.0);
    final cropH = crop!.height.clamp(0.01, 1.0);
    final cropX = crop!.x.clamp(0.0, 1.0 - cropW);
    final cropY = crop!.y.clamp(0.0, 1.0 - cropH);

    return LayoutBuilder(
      builder: (context, constraints) {
        final cw = constraints.maxWidth;
        final ch = constraints.maxHeight;

        final imgW = cw / cropW;
        final imgH = ch / cropH;
        final left = -cropX * imgW;
        final top = -cropY * imgH;

        return ClipRect(
          child: Stack(
            clipBehavior: Clip.hardEdge,
            children: [
              Positioned(
                left: left,
                top: top,
                width: imgW,
                height: imgH,
                child: buildRawImage(BoxFit.fill),
              ),
            ],
          ),
        );
      },
    );
  }
}
