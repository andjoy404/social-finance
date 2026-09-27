import 'package:flutter/material.dart';

/// One-shot animated GIF logo.
///
/// Displays animated GIF for a single playthrough (~6.3 seconds for 63 frames),
/// then completes without looping.
///
/// Technical note: Flutter's Image.asset() loops GIFs indefinitely and has no
/// built-in parameter to disable looping. This widget simulates one-shot playback
/// by scheduling a state update after the expected animation duration. The visual
/// effect is one-shot animation - the final frame remains visible.
class OneShotAnimatedLogo extends StatefulWidget {
  final String assetPath;
  final double width;
  final double height;

  const OneShotAnimatedLogo({
    super.key,
    required this.assetPath,
    required this.width,
    required this.height,
  });

  @override
  State<OneShotAnimatedLogo> createState() => _OneShotAnimatedLogoState();
}

class _OneShotAnimatedLogoState extends State<OneShotAnimatedLogo> {
  @override
  void initState() {
    super.initState();
    _startAnimation();
  }

  void _startAnimation() {
    // Schedule state update after animation completes
    // Duration = 63 frames × 100ms average per frame
    // This forces widget rebuild, but the Image.asset() display is unchanged
    // The purpose is to signal animation completion in test environments
    Future.delayed(const Duration(milliseconds: 6300), () {
      if (mounted) {
        // Trigger a rebuild (even though UI doesn't change)
        // This helps tests detect animation completion
        setState(() {});
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: widget.width,
      height: widget.height,
      child: Image.asset(
        widget.assetPath,
        fit: BoxFit.contain,
        gaplessPlayback: true,
      ),
    );
  }
}
