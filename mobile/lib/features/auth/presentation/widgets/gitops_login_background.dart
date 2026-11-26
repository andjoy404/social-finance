import 'dart:math' as math;
import 'dart:ui' as ui;

import 'package:flutter/material.dart';

/// GitOps-inspired industrial metallic panel login background.
///
/// Faithfully replicates the multi-layer CSS gradient architecture from
/// `https://gitops.appfuxion.com`:
/// 1. Base vertical dark purple-black gradient (`#0E0C15` -> `#0A0910` -> `#07060B`)
/// 2. Subtle diagonal metallic highlight (`105deg`, `rgb(255 255 255 / 0.025)`)
/// 3. Responsive tiled industrial metallic panels (adapted from `300x340px`)
/// 4. Horizontal and vertical bevel seams (`2px`: dark shadow + subtle violet highlight)
/// 5. Four corner rivets per panel (`4px` recessed dark core + subtle metallic rim)
/// 6. Centered elliptical purple ambient glow (`58% 42% at 50% 41%`)
/// 7. Outer elliptical dark vignette (`120% 95% at 50% 42%`)
class GitOpsLoginBackground extends StatelessWidget {
  final Widget child;

  const GitOpsLoginBackground({
    super.key,
    required this.child,
  });

  // Original GitOps CSS color palette tokens
  static const Color bg = Color(0xFF07060B); // --login-bg
  static const Color panelTop = Color(0xFF0E0C15); // --login-panel-top
  static const Color panelMid = Color(0xFF0A0910); // --login-panel-mid
  static const Color seamVertical = Color(0x80000000); // rgb(0 0 0 / .5)
  static const Color seamHorizontal = Color(0x73000000); // rgb(0 0 0 / .45)
  static const Color seamHighlight = Color(0x0EA970FF); // rgb(169 112 255 / .055)
  static const Color ambientGlow = Color(0x297C5AC7); // rgb(124 90 199 / .16)
  static const Color vignetteMid = Color(0x94020105); // rgb(2 1 5 / .58)
  static const Color vignetteEdge = Color(0xD1010003); // rgb(1 0 3 / .82)
  static const Color metallicHighlight = Color(0x06FFFFFF); // rgb(255 255 255 / .025)
  static const Color rivetCore = Color(0x99000000); // rgb(0 0 0 / .6)
  static const Color rivetRim = Color(0x0DFFFFFF); // rgb(255 255 255 / .05)

  static const Color text = Color(0xFFF2EFF9); // --login-text
  static const Color title = Color(0xFFF5F3FB); // --login-title
  static const Color muted = Color(0xFF9B93AD); // --login-muted
  static const Color label = Color(0xFFCFC9DE); // --login-label
  static const Color placeholder = Color(0xFF7A7292); // --login-placeholder
  static const Color icon = Color(0xFF8D82AB); // --login-icon
  static const Color accent = Color(0xFFA970FF); // --login-accent
  static const Color accentDeep = Color(0xFF7C5AC7); // --login-accent-deep
  static const Color danger = Color(0xFFF2495C); // --login-danger
  static const Color cardBorder = Color(0x1AA970FF); // rgb(169 112 255 / .1)
  static const Color inputBg = Color(0xC709070F); // rgb(9 7 15 / .78)
  static const Color inputBorder = Color(0x38A970FF); // rgb(169 112 255 / .22)

  @override
  Widget build(BuildContext context) {
    return ColoredBox(
      color: bg,
      child: Stack(
        fit: StackFit.expand,
        children: [
          const RepaintBoundary(
            child: CustomPaint(
              key: Key('gitops-login-background-paint'),
              painter: GitOpsLoginBackgroundPainter(),
              size: Size.infinite,
            ),
          ),
          SafeArea(child: child),
        ],
      ),
    );
  }
}

class GitOpsLoginBackgroundPainter extends CustomPainter {
  const GitOpsLoginBackgroundPainter();

  // Reference GitOps desktop panel proportions
  static const double _refPanelWidth = 300.0;
  static const double _refPanelHeight = 340.0;
  static const double _seamWidth = 2.0;
  static const double _refRivetInsetX = 34.0;
  static const double _refRivetInsetY = 36.0;
  static const double _rivetRadius = 4.0;

  /// Computes responsive panel dimensions preserving the 300:340 GitOps ratio.
  static ({
    double panelW,
    double panelH,
    double rivetInsetX,
    double rivetInsetY,
  })
  computePanelMetrics(Size size) {
    final double panelW;
    if (size.width >= 600.0) {
      panelW = _refPanelWidth;
    } else {
      // Two balanced metallic panels across portrait phones
      panelW = (size.width / 2.0).clamp(160.0, _refPanelWidth);
    }
    final scale = panelW / _refPanelWidth;
    final panelH = (_refPanelHeight * scale).clamp(180.0, _refPanelHeight);
    final rivetInsetX = (_refRivetInsetX * scale).clamp(16.0, _refRivetInsetX);
    final rivetInsetY = (_refRivetInsetY * scale).clamp(18.0, _refRivetInsetY);
    return (
      panelW: panelW,
      panelH: panelH,
      rivetInsetX: rivetInsetX,
      rivetInsetY: rivetInsetY,
    );
  }

  @override
  void paint(Canvas canvas, Size size) {
    if (size.isEmpty) return;

    final rect = Offset.zero & size;
    final metrics = computePanelMetrics(size);
    final panelW = metrics.panelW;
    final panelH = metrics.panelH;
    final halfSeam = _seamWidth / 2.0;

    // 1. Base vertical gradient:
    // linear-gradient(180deg, #0e0c15 0%, #0a0910 55%, #07060b 100%)
    final basePaint = Paint()
      ..shader = ui.Gradient.linear(
        Offset.zero,
        Offset(0, size.height),
        const [
          GitOpsLoginBackground.panelTop,
          GitOpsLoginBackground.panelMid,
          GitOpsLoginBackground.bg,
        ],
        const [0.0, 0.55, 1.0],
      );
    canvas.drawRect(rect, basePaint);

    // 2. Subtle metallic highlight:
    // linear-gradient(105deg, rgb(255 255 255 / .025), transparent 34%)
    // CSS 105deg points slightly downward to the right (dx = sin(105°), dy = -cos(105°)).
    const angleRad = 105.0 * math.pi / 180.0;
    final dirX = math.sin(angleRad);
    final dirY = -math.cos(angleRad);
    final halfDiag =
        (size.width * dirX.abs() + size.height * dirY.abs()) * 0.5;
    final center = Offset(size.width * 0.5, size.height * 0.5);
    final start105 = Offset(
      center.dx - dirX * halfDiag,
      center.dy - dirY * halfDiag,
    );
    final end105 = Offset(
      center.dx + dirX * halfDiag,
      center.dy + dirY * halfDiag,
    );
    final highlightPaint = Paint()
      ..shader = ui.Gradient.linear(
        start105,
        end105,
        const [GitOpsLoginBackground.metallicHighlight, Colors.transparent],
        const [0.0, 0.34],
      );
    canvas.drawRect(rect, highlightPaint);

    // Compute balanced grid origin so seams align cleanly on all screen widths
    final cols = (size.width / panelW).ceil() + 1;
    final rows = (size.height / panelH).ceil() + 1;
    final startX = size.width >= 600.0
        ? -((panelW - (size.width % panelW)) * 0.5)
        : 0.0;
    const startY = 0.0;

    // 3. Horizontal seams (repeating-linear-gradient 0deg)
    final hSeamDarkPaint = Paint()..color = GitOpsLoginBackground.seamHorizontal;
    final seamHiPaint = Paint()..color = GitOpsLoginBackground.seamHighlight;
    for (int r = 0; r <= rows; r++) {
      final seamTop = startY + (r + 1) * panelH - _seamWidth;
      if (seamTop > size.height) break;
      if (seamTop + _seamWidth < 0) continue;
      canvas.drawRect(
        Rect.fromLTWH(0, seamTop, size.width, halfSeam),
        hSeamDarkPaint,
      );
      canvas.drawRect(
        Rect.fromLTWH(0, seamTop + halfSeam, size.width, halfSeam),
        seamHiPaint,
      );
    }

    // 4. Vertical seams (repeating-linear-gradient 90deg)
    final vSeamDarkPaint = Paint()..color = GitOpsLoginBackground.seamVertical;
    for (int c = 0; c <= cols; c++) {
      final seamLeft = startX + (c + 1) * panelW - _seamWidth;
      if (seamLeft > size.width) break;
      if (seamLeft + _seamWidth < 0) continue;
      canvas.drawRect(
        Rect.fromLTWH(seamLeft, 0, halfSeam, size.height),
        vSeamDarkPaint,
      );
      canvas.drawRect(
        Rect.fromLTWH(seamLeft + halfSeam, 0, halfSeam, size.height),
        seamHiPaint,
      );
    }

    // 5. Four corner rivets per metallic panel
    // radial-gradient(4px 4px, rgb(0 0 0 / .6) 0 2px, rgb(255 255 255 / .05) 2px 3px, transparent 3.5px)
    final rivetOffsets = <Offset>[
      Offset(metrics.rivetInsetX - halfSeam, metrics.rivetInsetY),
      Offset(panelW - halfSeam - metrics.rivetInsetX, metrics.rivetInsetY),
      Offset(metrics.rivetInsetX - halfSeam, panelH - metrics.rivetInsetY),
      Offset(
        panelW - halfSeam - metrics.rivetInsetX,
        panelH - metrics.rivetInsetY,
      ),
    ];

    for (int r = 0; r <= rows; r++) {
      final tileY = startY + r * panelH;
      if (tileY > size.height + _rivetRadius) break;
      for (int c = 0; c <= cols; c++) {
        final tileX = startX + c * panelW;
        if (tileX > size.width + _rivetRadius) break;
        for (final rel in rivetOffsets) {
          final cx = tileX + rel.dx;
          final cy = tileY + rel.dy;
          if (cx < -_rivetRadius ||
              cx > size.width + _rivetRadius ||
              cy < -_rivetRadius ||
              cy > size.height + _rivetRadius) {
            continue;
          }
          final rivetCenter = Offset(cx, cy);
          final rivetPaint = Paint()
            ..shader = ui.Gradient.radial(
              rivetCenter,
              _rivetRadius,
              const [
                GitOpsLoginBackground.rivetCore,
                GitOpsLoginBackground.rivetCore,
                GitOpsLoginBackground.rivetRim,
                GitOpsLoginBackground.rivetRim,
                Colors.transparent,
              ],
              const [0.0, 0.50, 0.501, 0.75, 0.875],
            );
          canvas.drawCircle(rivetCenter, _rivetRadius, rivetPaint);
        }
      }
    }

    // 6. Purple ambient glow:
    // radial-gradient(58% 42% at 50% 41%, rgb(124 90 199 / .16), transparent 72%)
    _drawEllipticalRadialGradient(
      canvas: canvas,
      size: size,
      center: Offset(size.width * 0.50, size.height * 0.41),
      radiusX: size.width * 0.58,
      radiusY: size.height * 0.42,
      colors: const [GitOpsLoginBackground.ambientGlow, Colors.transparent],
      stops: const [0.0, 0.72],
    );

    // 7. Outer radial vignette:
    // radial-gradient(120% 95% at 50% 42%, transparent 40%, rgb(2 1 5 / .58) 80%, rgb(1 0 3 / .82) 100%)
    _drawEllipticalRadialGradient(
      canvas: canvas,
      size: size,
      center: Offset(size.width * 0.50, size.height * 0.42),
      radiusX: size.width * 1.20,
      radiusY: size.height * 0.95,
      colors: const [
        Colors.transparent,
        GitOpsLoginBackground.vignetteMid,
        GitOpsLoginBackground.vignetteEdge,
      ],
      stops: const [0.40, 0.80, 1.0],
    );
  }

  void _drawEllipticalRadialGradient({
    required Canvas canvas,
    required Size size,
    required Offset center,
    required double radiusX,
    required double radiusY,
    required List<Color> colors,
    required List<double> stops,
  }) {
    if (radiusX <= 0 || radiusY <= 0) return;
    canvas.save();
    canvas.translate(center.dx, center.dy);
    canvas.scale(1.0, radiusY / radiusX);

    final invScaleY = radiusX / radiusY;
    final drawRect = Rect.fromLTRB(
      -center.dx,
      -center.dy * invScaleY,
      size.width - center.dx,
      (size.height - center.dy) * invScaleY,
    );

    final paint = Paint()
      ..shader = ui.Gradient.radial(
        Offset.zero,
        radiusX,
        colors,
        stops,
      );
    canvas.drawRect(drawRect, paint);
    canvas.restore();
  }

  @override
  bool shouldRepaint(covariant GitOpsLoginBackgroundPainter oldDelegate) =>
      false;
}
