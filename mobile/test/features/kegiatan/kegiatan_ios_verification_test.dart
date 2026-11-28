import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/app/theme/app_theme.dart';
import 'package:social_finance/features/dashboard/data/community_repository.dart';
import 'package:social_finance/features/dashboard/data/models/community_models.dart';
import 'package:social_finance/features/dashboard/presentation/widgets/banner_style_editor.dart';
import 'package:social_finance/features/kegiatan/presentation/screens/kegiatan_form_screen.dart';
import 'package:social_finance/features/pengumuman/presentation/screens/announcement_form_screen.dart';

class _MockCommunityRepository extends CommunityRepository {
  _MockCommunityRepository() : super(dio: Dio());

  @override
  Future<CommunityActivity> createActivity(Map<String, dynamic> data) async {
    return CommunityActivity(
      id: 'ios-act-001',
      rtId: 'rt-1',
      title: data['title'] as String,
      description: data['description'] as String? ?? '',
      startDate: DateTime.parse(data['starts_at'] as String),
      endDate: data['ends_at'] != null ? DateTime.parse(data['ends_at'] as String) : null,
      location: data['location'] as String,
      status: 'upcoming',
      publicationStatus: (data['publish_now'] == true) ? 'published' : 'draft',
      createdAt: DateTime.now(),
      updatedAt: DateTime.now(),
    );
  }

  @override
  Future<CommunityActivity> getActivity(String id) async {
    return CommunityActivity(
      id: id,
      rtId: 'rt-1',
      title: 'Kegiatan iOS $id',
      description: 'Deskripsi iOS',
      startDate: DateTime(2026, 12, 1, 9, 0),
      endDate: DateTime(2026, 12, 1, 12, 0),
      location: 'Balai Warga',
      status: 'upcoming',
      publicationStatus: 'published',
      showOnBanner: true,
      createdAt: DateTime.now(),
      updatedAt: DateTime.now(),
    );
  }

  @override
  Future<List<BannerTemplate>> listBannerTemplates() async {
    return [];
  }

  @override
  Future<CommunityAnnouncement> getAnnouncement(String id) async {
    return CommunityAnnouncement(
      id: id,
      rtId: 'rt-1',
      title: 'Pengumuman iOS',
      body: 'Isi pengumuman iOS',
      priority: 10,
      displayOrder: 0,
      status: 'active',
      createdAt: DateTime.now(),
      updatedAt: DateTime.now(),
    );
  }
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  late _MockCommunityRepository mockRepo;

  setUp(() {
    mockRepo = _MockCommunityRepository();
  });

  group('iOS Platform Verification (TargetPlatform.iOS)', () {
    testWidgets('KegiatanFormScreen on iOS: clean state exits immediately via Batal or AppBar back', (tester) async {
      tester.view.physicalSize = const Size(800, 3000);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);

      bool popped = false;

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            communityRepositoryProvider.overrideWithValue(mockRepo),
          ],
          child: MaterialApp(
            theme: AppTheme.darkTheme.copyWith(platform: TargetPlatform.iOS),
            home: Navigator(
              onPopPage: (route, result) {
                popped = true;
                return route.didPop(result);
              },
              pages: const [
                MaterialPage(child: Text('Home Page')),
                MaterialPage(child: KegiatanFormScreen()),
              ],
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Tambah Kegiatan'), findsOneWidget);

      // Clean exit
      final batalFinder = find.widgetWithText(ElevatedButton, 'Batal');
      await tester.tap(batalFinder);
      await tester.pumpAndSettle();

      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsNothing);
      expect(popped, isTrue);
    });

    testWidgets('KegiatanFormScreen on iOS: dirty state triggers confirmation dialog, preserves input on Stay, and exits on Confirm', (tester) async {
      tester.view.physicalSize = const Size(800, 3000);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);

      bool popped = false;

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            communityRepositoryProvider.overrideWithValue(mockRepo),
          ],
          child: MaterialApp(
            theme: AppTheme.darkTheme.copyWith(platform: TargetPlatform.iOS),
            home: Navigator(
              onPopPage: (route, result) {
                popped = true;
                return route.didPop(result);
              },
              pages: const [
                MaterialPage(child: Text('Home Page')),
                MaterialPage(child: KegiatanFormScreen()),
              ],
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      // Enter text to make dirty
      await tester.enterText(find.widgetWithText(TextField, 'Judul Kegiatan *'), 'Gotong Royong iOS');
      await tester.pumpAndSettle();

      // Tap AppBar back arrow
      final backArrow = find.byIcon(Icons.arrow_back);
      await tester.tap(backArrow);
      await tester.pumpAndSettle();

      // Confirmation dialog appears
      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsOneWidget);
      expect(find.text('Perubahan yang belum disimpan akan hilang.'), findsOneWidget);
      expect(find.text('Tetap di Halaman'), findsOneWidget);
      expect(find.text('Ya, Batalkan'), findsOneWidget);

      // Tap Tetap di Halaman
      await tester.tap(find.text('Tetap di Halaman'));
      await tester.pumpAndSettle();

      // Input is preserved
      expect(find.text('Gotong Royong iOS'), findsOneWidget);
      expect(popped, isFalse);

      // Tap Batal button
      final batalFinder = find.widgetWithText(ElevatedButton, 'Batal');
      await tester.tap(batalFinder);
      await tester.pumpAndSettle();

      // Confirm cancel
      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsOneWidget);
      await tester.tap(find.text('Ya, Batalkan'));
      await tester.pumpAndSettle();

      expect(popped, isTrue);
    });

    testWidgets('AnnouncementFormScreen on iOS: dirty state triggers confirmation dialog', (tester) async {
      tester.view.physicalSize = const Size(800, 3000);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);

      bool popped = false;

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            communityRepositoryProvider.overrideWithValue(mockRepo),
          ],
          child: MaterialApp(
            theme: AppTheme.darkTheme.copyWith(platform: TargetPlatform.iOS),
            home: Navigator(
              onPopPage: (route, result) {
                popped = true;
                return route.didPop(result);
              },
              pages: const [
                MaterialPage(child: Text('Home Page')),
                MaterialPage(child: AnnouncementFormScreen()),
              ],
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      // Edit title to make dirty
      await tester.enterText(find.widgetWithText(TextField, 'Judul Pengumuman *'), 'Pengumuman iOS');
      await tester.pumpAndSettle();

      final batalFinder = find.widgetWithText(ElevatedButton, 'Batal');
      await tester.tap(batalFinder);
      await tester.pumpAndSettle();

      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsOneWidget);
      expect(find.text('Perubahan yang belum disimpan akan hilang.'), findsOneWidget);
      expect(find.text('Tetap di Halaman'), findsOneWidget);
      expect(find.text('Ya, Batalkan'), findsOneWidget);

      await tester.tap(find.text('Tetap di Halaman'));
      await tester.pumpAndSettle();

      expect(find.text('Pengumuman iOS'), findsOneWidget);
      expect(popped, isFalse);

      await tester.tap(batalFinder);
      await tester.pumpAndSettle();
      await tester.tap(find.text('Ya, Batalkan'));
      await tester.pumpAndSettle();

      expect(popped, isTrue);
    });

    testWidgets('BannerNeonGlowBackground on iOS: renders multi-layer ambient glow without errors', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          theme: AppTheme.darkTheme.copyWith(platform: TargetPlatform.iOS),
          home: const Scaffold(
            body: SizedBox(
              width: 360,
              height: 202,
              child: BannerNeonGlowBackground(color: Color(0xFF1E3A8A)),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.byType(BannerNeonGlowBackground), findsOneWidget);
    });
  });
}
