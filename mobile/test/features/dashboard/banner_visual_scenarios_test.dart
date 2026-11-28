import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/app/theme/app_theme.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/features/dashboard/data/models/community_models.dart';
import 'package:social_finance/features/dashboard/presentation/widgets/banner_crop_image.dart';
import 'package:social_finance/features/dashboard/presentation/widgets/banner_style_editor.dart';

const String _artifactDir =
    '/Users/Andriyan/.gemini/antigravity/brain/65270e50-57a1-4343-9ed5-7a78b8706dd8';

Future<void> _capturePng(
  WidgetTester tester,
  GlobalKey boundaryKey,
  String fileName,
) async {
  final boundary =
      boundaryKey.currentContext?.findRenderObject() as RenderRepaintBoundary?;
  if (boundary == null) return;
  final image = await boundary.toImage(pixelRatio: 2.0);
  final byteData = await image.toByteData(format: ui.ImageByteFormat.png);
  if (byteData == null) return;
  final file = File('$_artifactDir/$fileName');
  await file.writeAsBytes(byteData.buffer.asUint8List());
}

Widget _wrapApp(Widget child) {
  return ProviderScope(
    child: MaterialApp(
      theme: AppTheme.darkTheme,
      home: Scaffold(
        body: Center(
          child: SizedBox(
            width: 380,
            child: child,
          ),
        ),
      ),
    ),
  );
}

Widget _buildTestBannerWidget({
  required CommunityBanner banner,
  required GlobalKey boundaryKey,
}) {
  final styleCfg = banner.styleConfig;
  final isIuran = banner.sourceType == 'iuran_reminder';
  final isColorBg = !isIuran && styleCfg?.backgroundType == 'color';
  final isImageBg = !isIuran &&
      (styleCfg?.backgroundType == 'image' ||
          (styleCfg == null &&
              banner.imageUrl != null &&
              banner.imageUrl!.isNotEmpty));
  final showDetail = styleCfg?.showDetail ?? true;
  final customColorHex = styleCfg?.backgroundColor ?? '#A970FF';
  final customColor = parseHexColor(customColorHex);
  final imgUrl = styleCfg?.image?.url ?? banner.imageUrl;
  final zoom = styleCfg?.image?.zoom ?? 1.0;
  final crop = styleCfg?.image?.crop;
  final overlayOpacity = styleCfg?.overlayOpacity ?? 0.35;

  return RepaintBoundary(
    key: boundaryKey,
    child: AspectRatio(
      aspectRatio: 16 / 9,
      child: ClipRRect(
        borderRadius: BorderRadius.circular(AppRadius.base),
        child: Stack(
          children: [
            // 1. Background Layers
            if (isIuran) ...[
              Container(
                decoration: const BoxDecoration(
                  gradient: LinearGradient(
                    colors: [Color(0xFF6A1B9A), Color(0xFF4A148C)],
                    begin: Alignment.topLeft,
                    end: Alignment.bottomRight,
                  ),
                ),
              ),
            ] else if (isColorBg) ...[
              BannerNeonGlowBackground(color: customColor),
            ] else if (isImageBg && imgUrl != null && imgUrl.isNotEmpty) ...[
              // Base violet gradient for transparency support
              Container(
                decoration: const BoxDecoration(
                  gradient: LinearGradient(
                    colors: [Color(0xFF2A124D), Color(0xFF130924)],
                    begin: Alignment.topLeft,
                    end: Alignment.bottomRight,
                  ),
                ),
              ),
              Positioned.fill(
                child: BannerCropImage(
                  imageUrl: imgUrl,
                  crop: crop,
                  zoom: zoom,
                  alt: 'Banner Test',
                ),
              ),
              // Dark bottom gradient overlay
              Positioned.fill(
                child: Container(
                  decoration: BoxDecoration(
                    gradient: LinearGradient(
                      colors: [
                        Colors.black.withValues(alpha: overlayOpacity),
                        Colors.black.withValues(alpha: overlayOpacity * 0.4),
                        Colors.transparent,
                      ],
                      stops: const [0.0, 0.5, 1.0],
                      begin: Alignment.bottomCenter,
                      end: Alignment.topCenter,
                    ),
                  ),
                ),
              ),
            ],

            // 2. Foreground Content
            Padding(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  // Category badge
                  Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 8,
                      vertical: 4,
                    ),
                    decoration: BoxDecoration(
                      color: isIuran
                          ? Colors.orange.withValues(alpha: 0.85)
                          : Colors.black.withValues(alpha: 0.4),
                      borderRadius: BorderRadius.circular(AppRadius.xs),
                      border: Border.all(
                        color: Colors.white.withValues(alpha: 0.25),
                      ),
                    ),
                    child: Text(
                      banner.sourceType == 'activity'
                          ? 'Kegiatan'
                          : banner.sourceType == 'iuran_reminder'
                              ? 'Pengingat Iuran'
                              : 'Pengumuman',
                      style: const TextStyle(
                        color: Colors.white,
                        fontSize: 10,
                        fontWeight: FontWeight.bold,
                      ),
                    ),
                  ),

                  // Title and detail
                  Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        banner.title,
                        style: const TextStyle(
                          color: Colors.white,
                          fontSize: 16,
                          fontWeight: FontWeight.bold,
                          shadows: [
                            Shadow(color: Colors.black87, blurRadius: 4),
                          ],
                        ),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                      if (showDetail &&
                          banner.description != null &&
                          banner.description!.isNotEmpty) ...[
                        const SizedBox(height: 2),
                        Text(
                          banner.description!,
                          style: const TextStyle(
                            color: Colors.white,
                            fontSize: 12,
                            shadows: [
                              Shadow(color: Colors.black87, blurRadius: 3),
                            ],
                          ),
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ],
                    ],
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    ),
  );
}

void main() {
  group('Visual Scenarios Validation & Capture', () {
    testWidgets('Validates and captures all 7 banner scenarios', (tester) async {
      await tester.binding.setSurfaceSize(const Size(430, 960));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      final boundaryKey = GlobalKey();

      // Skenario 1: Custom Neon Color
      final bannerNeon = CommunityBanner(
        id: 'b-neon',
        rtId: 'rt-1',
        title: 'Pengumuman Gotong Royong Lingkungan',
        description: 'Seluruh warga diharapkan berkumpul di pos RT pukul 07:00 WIB membawa peralatan kebersihan.',
        sourceType: 'announcement',
        isActive: true,
        priority: 10,
        displayOrder: 1,
        createdAt: DateTime(2026, 11, 28),
        styleConfig: BannerStyleConfig(
          backgroundType: 'color',
          backgroundColor: '#00F0FF',
          gradient: computeNeonGradientConfig('#00F0FF'),
          showDetail: true,
        ),
      );

      await tester.pumpWidget(
        _wrapApp(
          _buildTestBannerWidget(
            banner: bannerNeon,
            boundaryKey: boundaryKey,
          ),
        ),
      );
      await tester.pumpAndSettle();

      final box1 = tester.getRect(find.byKey(boundaryKey));
      expect(box1.width / box1.height, closeTo(16.0 / 9.0, 0.05));
      expect(find.text('Pengumuman Gotong Royong Lingkungan'), findsOneWidget);

      await tester.runAsync(() async {
        await _capturePng(tester, boundaryKey, 'visual_banner_1_neon_color.png');
      });

      // Skenario 2 & 7: Banner Image with Crop & Zoom 2.0x
      final key2 = GlobalKey();
      final bannerImage = CommunityBanner(
        id: 'b-img',
        rtId: 'rt-1',
        title: 'Bazar UMKM & Pentas Seni Warga',
        description: 'Dimeriahkan oleh stan kuliner khas warga dan pertunjukan musik akustik pemuda RT.',
        sourceType: 'activity',
        isActive: true,
        priority: 10,
        displayOrder: 2,
        createdAt: DateTime(2026, 11, 28),
        styleConfig: BannerStyleConfig(
          backgroundType: 'image',
          image: ImageConfig(
            url: '/api/v1/community/images/bazar.png',
            zoom: 2.0,
            crop: CropRect(x: 0.15, y: 0.1, width: 0.5, height: 0.5),
            focalPoint: FocalPoint(x: 0.4, y: 0.35),
          ),
          showDetail: true,
          overlayOpacity: 0.45,
        ),
      );

      await tester.pumpWidget(
        _wrapApp(
          _buildTestBannerWidget(
            banner: bannerImage,
            boundaryKey: key2,
          ),
        ),
      );
      await tester.pumpAndSettle();

      final box2 = tester.getRect(find.byKey(key2));
      expect(box2.width / box2.height, closeTo(16.0 / 9.0, 0.05));
      expect(find.text('Bazar UMKM & Pentas Seni Warga'), findsOneWidget);

      await tester.runAsync(() async {
        await _capturePng(tester, key2, 'visual_banner_2_image_crop_zoom.png');
      });

      // Skenario 3: Banner PNG Transparan (Violet Background)
      final key3 = GlobalKey();
      final bannerTrans = CommunityBanner(
        id: 'b-trans',
        rtId: 'rt-1',
        title: 'Pemeriksaan Kesehatan Gratis Warga Lansia',
        description: 'Pemeriksaan tensi, gula darah, dan konsultasi dokter umum di Balai Warga.',
        sourceType: 'activity',
        isActive: true,
        priority: 10,
        displayOrder: 3,
        createdAt: DateTime(2026, 11, 28),
        styleConfig: BannerStyleConfig(
          backgroundType: 'image',
          image: ImageConfig(
            url: '/api/v1/community/images/transparent_logo.png',
            zoom: 1.0,
            crop: CropRect(x: 0.0, y: 0.0, width: 1.0, height: 1.0),
          ),
          showDetail: true,
          overlayOpacity: 0.3,
        ),
      );

      await tester.pumpWidget(
        _wrapApp(
          _buildTestBannerWidget(
            banner: bannerTrans,
            boundaryKey: key3,
          ),
        ),
      );
      await tester.pumpAndSettle();

      await tester.runAsync(() async {
        await _capturePng(tester, key3, 'visual_banner_3_png_transparent.png');
      });

      // Skenario 4 & 5: Show Detail Toggle (true vs false)
      final key4 = GlobalKey();
      final bannerDetailOff = CommunityBanner(
        id: 'b-nodetail',
        rtId: 'rt-1',
        title: 'Lomba Menghias Gapura HUT RI',
        description: 'Teks deskripsi ini harus disembunyikan karena show_detail bernilai false.',
        sourceType: 'announcement',
        isActive: true,
        priority: 10,
        displayOrder: 4,
        createdAt: DateTime(2026, 11, 28),
        styleConfig: BannerStyleConfig(
          backgroundType: 'color',
          backgroundColor: '#FF2A85',
          gradient: computeNeonGradientConfig('#FF2A85'),
          showDetail: false,
        ),
      );

      await tester.pumpWidget(
        _wrapApp(
          _buildTestBannerWidget(
            banner: bannerDetailOff,
            boundaryKey: key4,
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Lomba Menghias Gapura HUT RI'), findsOneWidget);
      expect(find.text('Teks deskripsi ini harus disembunyikan karena show_detail bernilai false.'), findsNothing);

      await tester.runAsync(() async {
        await _capturePng(tester, key4, 'visual_banner_4_detail_disabled.png');
      });

      // Skenario 6: Judul Sangat Panjang (Strict 1-line ellipsis, no overflow)
      final key5 = GlobalKey();
      final bannerLongTitle = CommunityBanner(
        id: 'b-long',
        rtId: 'rt-1',
        title: 'Pemberitahuan Sangat Penting Terkait Penyesuaian Tarif Retribusi Pengelolaan Sampah dan Kebersihan Lingkungan RT 002 RW 016 Tahun Anggaran 2027',
        description: 'Rapat koordinasi bersama seluruh kepala keluarga akan diselenggarakan di aula serbaguna pada hari Minggu malam.',
        sourceType: 'announcement',
        isActive: true,
        priority: 10,
        displayOrder: 5,
        createdAt: DateTime(2026, 11, 28),
        styleConfig: BannerStyleConfig(
          backgroundType: 'color',
          backgroundColor: '#00E599',
          gradient: computeNeonGradientConfig('#00E599'),
          showDetail: true,
        ),
      );

      await tester.pumpWidget(
        _wrapApp(
          _buildTestBannerWidget(
            banner: bannerLongTitle,
            boundaryKey: key5,
          ),
        ),
      );
      await tester.pumpAndSettle();

      final boxLong = tester.getRect(find.byKey(key5));
      expect(boxLong.width / boxLong.height, closeTo(16.0 / 9.0, 0.05));
      expect(tester.takeException(), isNull);

      await tester.runAsync(() async {
        await _capturePng(tester, key5, 'visual_banner_5_long_title_stable_16_9.png');
      });

      // Skenario Preserved Iuran Reminder
      final key6 = GlobalKey();
      final bannerIuran = CommunityBanner(
        id: 'b-iuran',
        rtId: 'rt-1',
        title: 'Pengingat Iuran Warga',
        description: 'Anda memiliki 1 tagihan iuran yang belum diselesaikan untuk periode November 2026.',
        sourceType: 'iuran_reminder',
        isActive: true,
        priority: 100,
        displayOrder: 0,
        createdAt: DateTime(2026, 11, 28),
      );

      await tester.pumpWidget(
        _wrapApp(
          _buildTestBannerWidget(
            banner: bannerIuran,
            boundaryKey: key6,
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Pengingat Iuran'), findsOneWidget);
      expect(find.text('Pengingat Iuran Warga'), findsOneWidget);

      await tester.runAsync(() async {
        await _capturePng(tester, key6, 'visual_banner_6_iuran_reminder_preserved.png');
      });

      // Skenario 8: BannerStyleEditor Live Preview Neon Color Consistency
      final keyEditor = GlobalKey();
      await tester.pumpWidget(
        _wrapApp(
          RepaintBoundary(
            key: keyEditor,
            child: BannerStyleEditor(
              value: BannerStyleConfig(
                backgroundType: 'color',
                backgroundColor: '#00F0FF',
                gradient: computeNeonGradientConfig('#00F0FF'),
                showDetail: true,
              ),
              titlePreview: 'Preview Editor Banner Neon',
              bodyPreview: 'Deskripsi live preview di dalam editor',
              category: 'activity',
              onChanged: (_) {},
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      await tester.runAsync(() async {
        await _capturePng(tester, keyEditor, 'visual_banner_editor_neon.png');
      });
    });
  });
}
