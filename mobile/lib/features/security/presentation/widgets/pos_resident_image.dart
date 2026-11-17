import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../providers/security_providers.dart';

/// Circular resident photo for the paired Pos device.
///
/// Loads bytes through the device-scoped image endpoint (`security-device`,
/// bound to the paired RT) — never via the user-image pipeline. Renders the
/// resident's profile photo when present; otherwise a deterministic initials
/// fallback (same source as every other Warga menu).
class PosResidentAvatar extends ConsumerStatefulWidget {
  final String name;
  final String? avatarPath;
  final double size;

  const PosResidentAvatar({
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
  ConsumerState<PosResidentAvatar> createState() => _PosResidentAvatarState();
}

class _PosResidentAvatarState extends ConsumerState<PosResidentAvatar> {
  Uint8List? _bytes;
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void didUpdateWidget(covariant PosResidentAvatar oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.avatarPath != widget.avatarPath ||
        oldWidget.name != widget.name) {
      _load();
    }
  }

  Future<void> _load() async {
    final path = (widget.avatarPath ?? '').trim();
    if (path.isEmpty) {
      if (mounted) {
        setState(() {
          _bytes = null;
          _loading = false;
        });
      }
      return;
    }
    final session = ref.read(posSessionProvider).session;
    if (session == null || session.serverUrl.isEmpty) {
      if (mounted) {
        setState(() {
          _bytes = null;
          _loading = false;
        });
      }
      return;
    }
    if (mounted && !_loading) setState(() => _loading = true);
    final bytes = await ref
        .read(securityApiClientProvider)
        .fetchDeviceImage(session, path);
    if (!mounted) return;
    setState(() {
      _bytes = bytes;
      _loading = false;
    });
  }

  @override
  Widget build(BuildContext context) {
    final size = widget.size;
    final radius = size / 2;
    final theme = Theme.of(context);
    final path = (widget.avatarPath ?? '').trim();

    if (path.isNotEmpty) {
      if (_loading) {
        return SizedBox(
          width: size,
          height: size,
          child: Center(
            child: SizedBox(
              width: size * 0.4,
              height: size * 0.4,
              child: const CircularProgressIndicator(strokeWidth: 2),
            ),
          ),
        );
      }
      final bytes = _bytes;
      if (bytes != null) {
        return GestureDetector(
          behavior: HitTestBehavior.opaque,
          onTap: () => Navigator.of(context).push(
            MaterialPageRoute<void>(
              builder: (_) => PosResidentImageViewer(
                title: widget.name.isEmpty ? 'Foto Profil' : widget.name,
                bytes: bytes,
              ),
            ),
          ),
          child: ClipOval(
            child: SizedBox(
              width: size,
              height: size,
              child: Image.memory(
                bytes,
                width: size,
                height: size,
                fit: BoxFit.cover,
                gaplessPlayback: true,
              ),
            ),
          ),
        );
      }
    }

    return CircleAvatar(
      radius: radius,
      backgroundColor: theme.colorScheme.primary.withValues(alpha: 0.16),
      child: Text(
        PosResidentAvatar.initialsOf(widget.name),
        style: TextStyle(
          color: theme.colorScheme.primary,
          fontWeight: FontWeight.bold,
          fontSize: size * 0.38,
        ),
      ),
    );
  }
}

/// Full-screen pinch-to-zoom viewer for a resident profile photo loaded
/// through the device-scoped image endpoint.
class PosResidentImageViewer extends StatelessWidget {
  final String title;
  final Uint8List bytes;

  const PosResidentImageViewer({
    super.key,
    required this.title,
    required this.bytes,
  });

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      body: SafeArea(
        child: Stack(
          children: [
            InteractiveViewer(
              minScale: 0.5,
              maxScale: 4,
              child: Center(child: Image.memory(bytes, fit: BoxFit.contain)),
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
              tooltip: 'Tutup',
              onPressed: () => Navigator.of(context).pop(),
            ),
          ),
        ),
      ),
    );
  }
}
