import 'dart:async';
import 'dart:ui' as ui;
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

/// Wraps a platform image [ui.Codec] to enforce a single playback cycle (`repetitionCount = 0`).
///
/// When [repetitionCount] is 0, Flutter's [MultiFrameImageStreamCompleter] naturally
/// decodes frames 0 through (frameCount - 1), emits the final frame, and stops decoding.
/// The final frame remains continuously displayed with no looping, no timer-based swaps,
/// and no background placeholder layers.
class _OneShotCodec implements ui.Codec {
  final ui.Codec _delegate;
  _OneShotCodec(this._delegate);

  @override
  int get frameCount => _delegate.frameCount;

  @override
  int get repetitionCount => 0; // Exactly one playback cycle, then stop on final frame.

  @override
  Future<ui.FrameInfo> getNextFrame() => _delegate.getNextFrame();

  @override
  void dispose() => _delegate.dispose();
}

/// An [ImageProvider] that decodes a GIF buffer via [_OneShotCodec].
///
/// The [generation] parameter uniquely identifies each playback invocation. When
/// [generation] increments on tap, Flutter's [Image] widget (with `gaplessPlayback: true`)
/// resolves a fresh codec starting from frame 0 while seamlessly holding the previous
/// rendered frame until frame 0 is decoded and ready to display—eliminating all blink/flicker.
class _OneShotGifImageProvider extends ImageProvider<_OneShotGifImageProvider> {
  final Uint8List bytes;
  final int generation;

  const _OneShotGifImageProvider(this.bytes, this.generation);

  @override
  Future<_OneShotGifImageProvider> obtainKey(ImageConfiguration configuration) {
    return SynchronousFuture<_OneShotGifImageProvider>(this);
  }

  @override
  ImageStreamCompleter loadImage(_OneShotGifImageProvider key, ImageDecoderCallback decode) {
    return MultiFrameImageStreamCompleter(
      codec: _loadAsync(decode),
      scale: 1.0,
      debugLabel: 'OneShotGif(gen=$generation)',
    );
  }


  Future<ui.Codec> _loadAsync(Function decode) async {
    final buffer = await ui.ImmutableBuffer.fromUint8List(bytes);
    final rawCodec = await (decode(buffer) as Future<ui.Codec>);
    return _OneShotCodec(rawCodec);
  }

  @override
  bool operator ==(Object other) {
    if (other.runtimeType != runtimeType) return false;
    return other is _OneShotGifImageProvider &&
        other.bytes == bytes &&
        other.generation == generation;
  }

  @override
  int get hashCode => Object.hash(bytes.hashCode, generation);
}

/// One-shot animated GIF logo with seamless restart.
///
/// Behavior:
/// 1. Autoplays once on initial display from frame 0, stops at final frame.
/// 2. Each tap restarts playback from frame 0 and plays exactly once.
/// 3. Tapping while playing immediately restarts from frame 0 without blinking.
/// 4. Never loops automatically.
/// 5. Renders ONLY the GIF—no static PNG coin or background image behind or over it.
class OneShotAnimatedLogo extends StatefulWidget {
  final String assetPath;
  final String? fallbackAssetPath;
  final double width;
  final double height;
  final Duration duration;

  const OneShotAnimatedLogo({
    super.key,
    required this.assetPath,
    this.fallbackAssetPath,
    required this.width,
    required this.height,
    this.duration = const Duration(milliseconds: 2400),
  });

  @override
  State<OneShotAnimatedLogo> createState() => _OneShotAnimatedLogoState();
}

class _OneShotAnimatedLogoState extends State<OneShotAnimatedLogo> {
  Uint8List? _gifBytes;
  int _generation = 0;
  bool _hasError = false;

  @override
  void initState() {
    super.initState();
    debugPrint('[OneShotAnimatedLogo] initState');
    _loadAndPlay();
  }

  Future<void> _loadAndPlay() async {
    try {
      debugPrint('[OneShotAnimatedLogo] loading asset: ${widget.assetPath}');
      final ByteData data = await rootBundle.load(widget.assetPath);
      if (!mounted) return;
      _gifBytes = data.buffer.asUint8List();
      debugPrint('[OneShotAnimatedLogo] loaded ${_gifBytes!.lengthInBytes} bytes');
      setState(() {
        _generation = 1;
      });
      debugPrint('[OneShotAnimatedLogo] startPlayback (autoplay) gen=1');
    } catch (e) {
      debugPrint('[OneShotAnimatedLogo] Error loading GIF: $e');
      if (mounted) {
        setState(() {
          _hasError = true;
        });
      }
    }
  }

  void _onTap() {
    debugPrint('[OneShotAnimatedLogo] onTap detected (current gen=$_generation)');
    if (_gifBytes == null || _hasError) return;
    setState(() {
      _generation++;
    });
    debugPrint('[OneShotAnimatedLogo] startPlayback (tap) gen=$_generation');
  }

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: _onTap,
      child: SizedBox(
        width: widget.width,
        height: widget.height,
        child: _buildContent(),
      ),
    );
  }

  Widget _buildContent() {
    // Static fallback is rendered ONLY if the GIF failed to load or is not ready yet.
    if (_hasError || _gifBytes == null || _generation == 0) {
      return _buildStaticFallback();
    }

    // Only the GIF is rendered: no underlying PNG coin, no background stack layer.
    return Image(
      image: _OneShotGifImageProvider(_gifBytes!, _generation),
      width: widget.width,
      height: widget.height,
      fit: BoxFit.contain,
      gaplessPlayback: true,
    );
  }

  Widget _buildStaticFallback() {
    if (widget.fallbackAssetPath != null) {
      return Image.asset(
        widget.fallbackAssetPath!,
        width: widget.width,
        height: widget.height,
        fit: BoxFit.contain,
      );
    }
    return const SizedBox.shrink();
  }
}
