import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/features/dashboard/data/models/community_models.dart';

// Helper simulating Flutter's BannerCropImage layout math
Map<String, double> computeFlutterCropGeometry({
  required double containerWidth,
  required double containerHeight,
  required CropRect crop,
}) {
  final cropW = crop.width.clamp(0.01, 1.0);
  final cropH = crop.height.clamp(0.01, 1.0);
  final cropX = crop.x.clamp(0.0, 1.0 - cropW);
  final cropY = crop.y.clamp(0.0, 1.0 - cropH);

  final imgW = containerWidth / cropW;
  final imgH = containerHeight / cropH;
  final left = -cropX * imgW;
  final top = -cropY * imgH;

  return {
    'left': left,
    'top': top,
    'width': imgW,
    'height': imgH,
  };
}

void main() {
  group('Crop Math Consistency with Web CSS', () {
    test('identical geometry across various 16:9 container resolutions', () {
      const resolutions = [
        {'w': 1920.0, 'h': 1080.0},
        {'w': 1280.0, 'h': 720.0},
        {'w': 800.0, 'h': 450.0},
        {'w': 360.0, 'h': 202.5},
      ];

      final testCrops = [
        CropRect(x: 0.0, y: 0.0, width: 1.0, height: 1.0),
        CropRect(x: 0.1, y: 0.2, width: 0.5, height: 0.5),
        CropRect(x: 0.25, y: 0.15, width: 0.6, height: 0.6),
        CropRect(x: 0.33, y: 0.44, width: 0.33, height: 0.33),
      ];

      for (final res in resolutions) {
        final cw = res['w']!;
        final ch = res['h']!;

        for (final crop in testCrops) {
          final flutterGeom = computeFlutterCropGeometry(
            containerWidth: cw,
            containerHeight: ch,
            crop: crop,
          );

          // Web CSS equivalent:
          // width: (1 / crop.width) * 100% of cw
          // height: (1 / crop.height) * 100% of ch
          // left: -(crop.x / crop.width) * 100% of cw
          // top: -(crop.y / crop.height) * 100% of ch
          final webWidth = (1.0 / crop.width) * cw;
          final webHeight = (1.0 / crop.height) * ch;
          final webLeft = -(crop.x / crop.width) * cw;
          final webTop = -(crop.y / crop.height) * ch;

          expect(flutterGeom['width']!, closeTo(webWidth, 0.0001));
          expect(flutterGeom['height']!, closeTo(webHeight, 0.0001));
          expect(flutterGeom['left']!, closeTo(webLeft, 0.0001));
          expect(flutterGeom['top']!, closeTo(webTop, 0.0001));
        }
      }
    });

    test('clamping prevents out-of-bounds crop window', () {
      final outOfBoundsCrop = CropRect(x: 0.8, y: 0.7, width: 0.5, height: 0.5);
      final geom = computeFlutterCropGeometry(
        containerWidth: 1000.0,
        containerHeight: 562.5,
        crop: outOfBoundsCrop,
      );

      // max x is 1.0 - 0.5 = 0.5. With x clamped to 0.5, left = -0.5 * 2000 = -1000
      expect(geom['width']!, equals(2000.0));
      expect(geom['left']!, equals(-1000.0));
      // max y is 1.0 - 0.5 = 0.5. With y clamped to 0.5, top = -0.5 * 1125 = -562.5
      expect(geom['top']!, equals(-562.5));
    });
  });
}
