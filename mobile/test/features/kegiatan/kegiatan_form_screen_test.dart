import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/app/theme/app_theme.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/features/dashboard/data/community_repository.dart';
import 'package:social_finance/features/dashboard/data/models/community_models.dart';
import 'package:social_finance/features/kegiatan/presentation/screens/kegiatan_form_screen.dart';
import 'package:social_finance/features/pengumuman/presentation/screens/announcement_form_screen.dart';

class _MockCommunityRepository extends CommunityRepository {
  _MockCommunityRepository() : super(dio: Dio());

  Map<String, dynamic>? lastCreatedPayload;
  Map<String, dynamic>? lastUpdatedPayload;
  String? lastUpdatedId;

  @override
  Future<CommunityActivity> createActivity(Map<String, dynamic> data) async {
    lastCreatedPayload = data;
    return CommunityActivity(
      id: 'act-new',
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
  Future<CommunityActivity> updateActivity(String id, Map<String, dynamic> data) async {
    lastUpdatedId = id;
    lastUpdatedPayload = data;
    return CommunityActivity(
      id: id,
      rtId: 'rt-1',
      title: data['title'] as String,
      description: data['description'] as String? ?? '',
      startDate: DateTime.parse(data['starts_at'] as String),
      endDate: data['ends_at'] != null ? DateTime.parse(data['ends_at'] as String) : null,
      location: data['location'] as String,
      status: 'upcoming',
      publicationStatus: 'published',
      createdAt: DateTime.now(),
      updatedAt: DateTime.now(),
    );
  }

  @override
  Future<CommunityActivity> getActivity(String id) async {
    return CommunityActivity(
      id: id,
      rtId: 'rt-1',
      title: 'Kegiatan Existing $id',
      description: 'Deskripsi existing',
      startDate: DateTime(2026, 12, 1, 9, 0),
      endDate: DateTime(2026, 12, 1, 12, 0),
      location: 'Balai Warga',
      picName: 'Pak RT',
      budgetPlanned: '1000000',
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
      title: 'Pengumuman Existing',
      body: 'Isi pengumuman',
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

  group('KegiatanFormScreen Comprehensive Verification', () {
    testWidgets('Mode Tambah: renders full Scaffold landing page, AppBar title, and all expected fields', (tester) async {
      tester.view.physicalSize = const Size(800, 3000);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            communityRepositoryProvider.overrideWithValue(mockRepo),
          ],
          child: MaterialApp(
            theme: AppTheme.darkTheme,
            home: const KegiatanFormScreen(),
          ),
        ),
      );
      await tester.pumpAndSettle();

      // Full Scaffold with AppBar
      expect(find.byType(Scaffold), findsOneWidget);
      expect(find.text('Tambah Kegiatan'), findsOneWidget);
      expect(find.byIcon(Icons.arrow_back), findsOneWidget);

      // Form fields
      expect(find.widgetWithText(TextField, 'Judul Kegiatan *'), findsOneWidget);
      expect(find.widgetWithText(TextField, 'Lokasi Kegiatan *'), findsOneWidget);
      expect(find.widgetWithText(TextField, 'Deskripsi Kegiatan'), findsOneWidget);
      expect(find.widgetWithText(TextField, 'Nama Penanggung Jawab (PIC)'), findsOneWidget);
      expect(find.widgetWithText(TextField, 'Rencana Anggaran (Rp)'), findsOneWidget);

      // Schedule tiles
      expect(find.text('Waktu Mulai *'), findsOneWidget);
      expect(find.text('Waktu Selesai (Opsional)'), findsOneWidget);

      // Switches
      expect(find.text('Tampilkan di Banner Beranda'), findsOneWidget);
      expect(find.text('Publikasikan langsung'), findsOneWidget);

      // Standard buttons: Batal (red) and Simpan (green) with equal size
      final batalFinder = find.widgetWithText(ElevatedButton, 'Batal');
      final simpanFinder = find.widgetWithText(ElevatedButton, 'Simpan');
      expect(batalFinder, findsOneWidget);
      expect(simpanFinder, findsOneWidget);

      final batalBtn = tester.widget<ElevatedButton>(batalFinder);
      final simpanBtn = tester.widget<ElevatedButton>(simpanFinder);
      expect(batalBtn.style?.backgroundColor?.resolve({}), AppColors.danger);
      expect(simpanBtn.style?.backgroundColor?.resolve({}), AppColors.success);

      final batalSize = tester.getSize(batalFinder);
      final simpanSize = tester.getSize(simpanFinder);
      expect(batalSize.width, simpanSize.width);
      expect(batalSize.height, 48.0);
      expect(simpanSize.height, 48.0);
    });

    testWidgets('Validation: empty required fields prevent submission and show error message', (tester) async {
      tester.view.physicalSize = const Size(800, 3000);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            communityRepositoryProvider.overrideWithValue(mockRepo),
          ],
          child: MaterialApp(
            theme: AppTheme.darkTheme,
            home: const KegiatanFormScreen(),
          ),
        ),
      );
      await tester.pumpAndSettle();

      // Tap Simpan with empty fields
      final simpanFinder = find.widgetWithText(ElevatedButton, 'Simpan');
      await tester.tap(simpanFinder);
      await tester.pumpAndSettle();

      expect(find.text('Judul dan lokasi kegiatan wajib diisi'), findsOneWidget);
      expect(mockRepo.lastCreatedPayload, isNull);
    });

    testWidgets('Mode Tambah: successfully creates new activity when valid', (tester) async {
      tester.view.physicalSize = const Size(800, 3000);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            communityRepositoryProvider.overrideWithValue(mockRepo),
          ],
          child: MaterialApp(
            theme: AppTheme.darkTheme,
            home: const KegiatanFormScreen(),
          ),
        ),
      );
      await tester.pumpAndSettle();

      await tester.enterText(find.widgetWithText(TextField, 'Judul Kegiatan *'), 'Senam Sehat Warga');
      await tester.enterText(find.widgetWithText(TextField, 'Lokasi Kegiatan *'), 'Lapangan RT 02');
      await tester.enterText(find.widgetWithText(TextField, 'Nama Penanggung Jawab (PIC)'), 'Bu Dewi');
      await tester.enterText(find.widgetWithText(TextField, 'Rencana Anggaran (Rp)'), '300000');
      await tester.pumpAndSettle();

      final simpanFinder = find.widgetWithText(ElevatedButton, 'Simpan');
      await tester.tap(simpanFinder);
      await tester.pumpAndSettle();

      expect(mockRepo.lastCreatedPayload, isNotNull);
      expect(mockRepo.lastCreatedPayload!['title'], 'Senam Sehat Warga');
      expect(mockRepo.lastCreatedPayload!['location'], 'Lapangan RT 02');
      expect(mockRepo.lastCreatedPayload!['pic_name'], 'Bu Dewi');
      expect(mockRepo.lastCreatedPayload!['budget_planned'], '300000');
    });

    testWidgets('Mode Ubah: populates existing activity data and updates on save', (tester) async {
      tester.view.physicalSize = const Size(800, 3000);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);

      final initial = CommunityActivity(
        id: 'act-99',
        rtId: 'rt-1',
        title: 'Bazar UMKM',
        description: 'Bazar kuliner warga RT',
        startDate: DateTime(2026, 12, 10, 8, 0),
        endDate: DateTime(2026, 12, 10, 15, 0),
        location: 'Sepanjang Blok C',
        picName: 'Pak Joko',
        budgetPlanned: '500000',
        status: 'upcoming',
        publicationStatus: 'published',
        showOnBanner: true,
        createdAt: DateTime.now(),
        updatedAt: DateTime.now(),
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            communityRepositoryProvider.overrideWithValue(mockRepo),
          ],
          child: MaterialApp(
            theme: AppTheme.darkTheme,
            home: KegiatanFormScreen(
              activityId: 'act-99',
              initialActivity: initial,
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Ubah Kegiatan'), findsOneWidget);
      expect(find.text('Bazar UMKM'), findsWidgets);
      expect(find.text('Sepanjang Blok C'), findsOneWidget);
      expect(find.text('Pak Joko'), findsOneWidget);

      // Modify title
      await tester.enterText(find.widgetWithText(TextField, 'Judul Kegiatan *'), 'Bazar UMKM & Festival Kuliner');
      await tester.pumpAndSettle();

      final simpanFinder = find.widgetWithText(ElevatedButton, 'Simpan');
      await tester.tap(simpanFinder);
      await tester.pumpAndSettle();

      expect(mockRepo.lastUpdatedId, 'act-99');
      expect(mockRepo.lastUpdatedPayload!['title'], 'Bazar UMKM & Festival Kuliner');
    });

    testWidgets('Back Navigation: clean state leaves immediately without dialog', (tester) async {
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
            theme: AppTheme.darkTheme,
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

      // Tap Batal on clean form
      final batalFinder = find.widgetWithText(ElevatedButton, 'Batal');
      await tester.tap(batalFinder);
      await tester.pumpAndSettle();

      // Should pop immediately without confirmation dialog
      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsNothing);
      expect(popped, isTrue);
    });

    testWidgets('Back Navigation: dirty state triggers confirmation dialog ("Apakah Anda yakin ingin membatalkan?")', (tester) async {
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
            theme: AppTheme.darkTheme,
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

      // Edit a field to make it dirty
      await tester.enterText(find.widgetWithText(TextField, 'Judul Kegiatan *'), 'Perubahan Baru');
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

      // Tap 'Tetap di Halaman' -> stays on form
      await tester.tap(find.text('Tetap di Halaman'));
      await tester.pumpAndSettle();

      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsNothing);
      expect(find.text('Tambah Kegiatan'), findsOneWidget);
      expect(popped, isFalse);

      // Tap 'Batal' button -> dialog reappears
      final batalFinder = find.widgetWithText(ElevatedButton, 'Batal');
      await tester.tap(batalFinder);
      await tester.pumpAndSettle();

      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsOneWidget);

      // Tap 'Ya, Batalkan' -> pops and leaves
      await tester.tap(find.text('Ya, Batalkan'));
      await tester.pumpAndSettle();

      expect(popped, isTrue);
    });

    testWidgets('No regression on AnnouncementFormScreen dirty confirmation dialog', (tester) async {
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
            theme: AppTheme.darkTheme,
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
      await tester.enterText(find.widgetWithText(TextField, 'Judul Pengumuman *'), 'Pengumuman Baru');
      await tester.pumpAndSettle();

      // Tap Batal
      final batalFinder = find.widgetWithText(ElevatedButton, 'Batal');
      await tester.tap(batalFinder);
      await tester.pumpAndSettle();

      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsOneWidget);
      expect(find.text('Perubahan yang belum disimpan akan hilang.'), findsOneWidget);
      expect(find.text('Tetap di Halaman'), findsOneWidget);
      expect(find.text('Ya, Batalkan'), findsOneWidget);

      // Confirm cancel
      await tester.tap(find.text('Ya, Batalkan'));
      await tester.pumpAndSettle();

      expect(popped, isTrue);
    });
  });
}
