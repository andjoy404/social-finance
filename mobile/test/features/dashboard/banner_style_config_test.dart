import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/features/dashboard/data/models/community_models.dart';
import 'package:social_finance/features/dashboard/presentation/widgets/banner_style_editor.dart';

void main() {
  group('BannerStyleConfig & neon gradient computation', () {
    test('parseHexColor handles 6-digit, 3-digit, and uppercase', () {
      final c1 = parseHexColor('#A970FF');
      expect(c1.a, equals(1.0));
      expect((c1.r * 255).round(), equals(0xA9));
      expect((c1.g * 255).round(), equals(0x70));
      expect((c1.b * 255).round(), equals(0xFF));

      final c2 = parseHexColor('#FFF');
      expect((c2.r * 255).round(), equals(255));
      expect((c2.g * 255).round(), equals(255));
      expect((c2.b * 255).round(), equals(255));

      final cFallback = parseHexColor('invalid');
      expect(cFallback, isNotNull);
    });

    test('computeNeonGradientConfig derives correct 135deg glow gradient', () {
      final grad = computeNeonGradientConfig('#A970FF');
      expect(grad.from, equals('#A970FF'));
      expect(grad.angle, equals(135));
      expect(grad.glow, isTrue);
      expect(grad.to.startsWith('#'), isTrue);
      expect(grad.to.length, equals(7));
    });

    test('BannerStyleConfig color mode serialization round-trip', () {
      final config = BannerStyleConfig(
        backgroundType: 'color',
        backgroundColor: '#00F0FF',
        gradient: computeNeonGradientConfig('#00F0FF'),
        showDetail: true,
      );

      final json = config.toJson();
      expect(json['background_type'], equals('color'));
      expect(json['background_color'], equals('#00F0FF'));
      expect(json['gradient'], isNotNull);
      expect((json['gradient'] as Map)['angle'], equals(135));
      expect(json['show_detail'], isTrue);

      final parsed = BannerStyleConfig.fromJson(json);
      expect(parsed.backgroundType, equals('color'));
      expect(parsed.backgroundColor, equals('#00F0FF'));
      expect(parsed.gradient?.glow, isTrue);
      expect(parsed.showDetail, isTrue);
    });

    test('BannerStyleConfig image mode serialization with crop and zoom', () {
      final config = BannerStyleConfig(
        backgroundType: 'image',
        image: ImageConfig(
          url: '/api/v1/community/upload/banner123.jpg',
          zoom: 1.75,
          crop: CropRect(x: 0.1, y: 0.2, width: 0.8, height: 0.6),
          focalPoint: FocalPoint(x: 0.5, y: 0.5),
        ),
        showDetail: false,
        overlayOpacity: 0.45,
      );

      final json = config.toJson();
      expect(json['background_type'], equals('image'));
      expect(json['image'], isNotNull);
      final imgMap = json['image'] as Map<String, dynamic>;
      expect(imgMap['url'], equals('/api/v1/community/upload/banner123.jpg'));
      expect(imgMap['zoom'], equals(1.75));
      expect((imgMap['crop'] as Map)['x'], equals(0.1));
      expect(json['show_detail'], isFalse);
      expect(json['overlay_opacity'], equals(0.45));

      final parsed = BannerStyleConfig.fromJson(json);
      expect(parsed.backgroundType, equals('image'));
      expect(parsed.image?.zoom, equals(1.75));
      expect(parsed.image?.crop?.width, equals(0.8));
      expect(parsed.showDetail, isFalse);
      expect(parsed.overlayOpacity, equals(0.45));
    });

    test('CommunityBanner.fromJson supports legacy without style_config', () {
      final legacyJson = {
        'id': 'ban-1',
        'title': 'Pengumuman Penting',
        'source_type': 'announcement',
        'source_id': 'ann-1',
        'image_url': 'https://example.com/legacy.png',
        'is_active': true,
        'priority': 10,
        'display_order': 0,
        'created_at': '2026-11-28T00:00:00Z',
        'updated_at': '2026-11-28T00:00:00Z',
      };

      final banner = CommunityBanner.fromJson(legacyJson);
      expect(banner.id, equals('ban-1'));
      expect(banner.title, equals('Pengumuman Penting'));
      expect(banner.styleConfig, isNull);
    });

    test('CommunityBanner.fromJson parses style_config when present', () {
      final bannerJson = {
        'id': 'ban-2',
        'title': 'Kerja Bakti Bersama',
        'source_type': 'activity',
        'source_id': 'act-1',
        'image_url': 'https://example.com/banner.png',
        'style_config': {
          'background_type': 'color',
          'background_color': '#FF2A85',
          'gradient': {
            'from': '#FF2A85',
            'to': '#2D0717',
            'angle': 135,
            'glow': true,
          },
          'show_detail': true,
        },
        'is_active': true,
        'priority': 10,
        'display_order': 0,
        'created_at': '2026-11-28T00:00:00Z',
        'updated_at': '2026-11-28T00:00:00Z',
      };

      final banner = CommunityBanner.fromJson(bannerJson);
      expect(banner.styleConfig, isNotNull);
      expect(banner.styleConfig!.backgroundType, equals('color'));
      expect(banner.styleConfig!.backgroundColor, equals('#FF2A85'));
      expect(banner.styleConfig!.gradient?.from, equals('#FF2A85'));
    });
  });
}
