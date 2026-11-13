import 'package:flutter/material.dart';

import 'community_image.dart';
import 'resident_image_viewer.dart';

/// Circular resident avatar.
///
/// Renders the resident's profile photo (authorized community-images pipeline,
/// signed-URL served) when available, otherwise a deterministic initials
/// fallback. Never surfaces the storage URL directly.
class ResidentAvatar extends StatelessWidget {
  final String name;
  final String? avatarPath;
  final String? avatarThumbPath;
  final double size;

  const ResidentAvatar({
    super.key,
    required this.name,
    this.avatarPath,
    this.avatarThumbPath,
    this.size = 40,
  });

  /// Up-to-two-letter initials from a display name (stable, locale-safe).
  static String initialsOf(String name) {
    final words = name
        .trim()
        .split(RegExp(r'\s+'))
        .where((w) => w.isNotEmpty)
        .toList();
    if (words.isEmpty) return '?';
    final first = words.first.substring(0, 1).toUpperCase();
    if (words.length == 1) return first;
    final last = words.last.substring(0, 1).toUpperCase();
    return (first + last).substring(0, 2);
  }

  @override
  Widget build(BuildContext context) {
    final radius = size / 2;
    final theme = Theme.of(context);
    final fullPath = (avatarPath ?? '').trim();
    final thumbPath = (avatarThumbPath ?? '').trim();
    final primaryPath = thumbPath.isNotEmpty ? thumbPath : fullPath;

    if (primaryPath.isNotEmpty) {
      return GestureDetector(
        behavior: HitTestBehavior.opaque,
        onTap: () {
          Navigator.of(context).push(
            MaterialPageRoute<void>(
              builder: (_) => ResidentImageViewer(
                imageRef: fullPath.isNotEmpty ? fullPath : thumbPath,
                title: name.isEmpty ? 'Foto Profil' : name,
              ),
            ),
          );
        },
        child: ClipOval(
          child: SizedBox(
            width: size,
            height: size,
            child: CommunityImage(
              imageRef: primaryPath,
              fallbackRef: thumbPath.isNotEmpty && fullPath.isNotEmpty ? fullPath : null,
              width: size,
              height: size,
              fit: BoxFit.cover,
              alt: name,
            ),
          ),
        ),
      );
    }

    return CircleAvatar(
      radius: radius,
      backgroundColor: theme.colorScheme.primary.withValues(alpha: 0.16),
      child: Text(
        initialsOf(name),
        style: TextStyle(
          color: theme.colorScheme.primary,
          fontWeight: FontWeight.bold,
          fontSize: size * 0.38,
        ),
      ),
    );
  }
}
