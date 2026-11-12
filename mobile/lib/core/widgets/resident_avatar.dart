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
  final double size;

  const ResidentAvatar({
    super.key,
    required this.name,
    this.avatarPath,
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
    final path = (avatarPath ?? '').trim();

    if (path.isNotEmpty) {
      return GestureDetector(
        behavior: HitTestBehavior.opaque,
        onTap: () {
          Navigator.of(context).push(
            MaterialPageRoute<void>(
              builder: (_) => ResidentImageViewer(
                imageRef: path,
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
              imageRef: path,
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
