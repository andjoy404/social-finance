import 'dart:io';
import 'dart:ui' as ui;

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/api/device_id_service.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/core/widgets/community_image.dart';
import 'package:social_finance/app/theme/app_theme.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/dashboard/data/community_repository.dart';
import 'package:social_finance/features/dashboard/data/models/community_models.dart';
import 'package:social_finance/features/kegiatan/presentation/screens/kegiatan_screen.dart';
import 'package:social_finance/features/notification/data/models/notification_models.dart';
import 'package:social_finance/features/notification/data/notification_api_client.dart';
import 'package:social_finance/features/notification/data/notification_repository.dart';
import 'package:social_finance/features/notification/presentation/screens/notification_management_screen.dart';
import 'package:social_finance/features/pengumuman/presentation/screens/pengumuman_screen.dart';

const String _artifactDir =
    '/Users/Andriyan/.gemini/antigravity-cli/brain/e42d1612-d432-4ef1-9b50-68dadad92eb7';
const String _mode = String.fromEnvironment(
  'SCREENSHOT_MODE',
  defaultValue: 'before',
);

class _MockAuthRepo extends AuthRepository {
  _MockAuthRepo(Map<String, dynamic>? user)
    : super(
        authService: AuthService(
          ApiClient(baseUrl: 'http://test.local'),
          DeviceIdService(),
        ),
        apiClient: ApiClient(baseUrl: 'http://test.local'),
      ) {
    state = AsyncData(user);
  }
}

class _FakeNotifRepo extends NotificationRepository {
  _FakeNotifRepo() : super(apiClient: NotificationApiClient(dio: Dio()));

  @override
  Future<List<SystemNotificationType>> fetchSystemTypes() async {
    return [
      SystemNotificationType(
        eventType: 'manual_push',
        name: 'Notifikasi Siaran Manual',
        trigger: 'Dikirim manual oleh Pengurus RT',
        recipientsRule: 'Seluruh warga RT',
        defaultTitle: 'Informasi Pengurus RT',
        defaultBody: 'Pengumuman penting untuk seluruh warga RT 002.',
        allowedVariables: const [],
      ),
      SystemNotificationType(
        eventType: 'iuran_payment_reminder',
        name: 'Pengingat Tagihan Iuran',
        trigger: 'Pengiriman pengingat tagihan bulanan',
        recipientsRule: 'Warga RT dengan tagihan belum lunas',
        defaultTitle: 'Pengingat Iuran Bulanan',
        defaultBody: 'Mohon segera menyelesaikan pembayaran iuran RT.',
        allowedVariables: const ['bill_id', 'period'],
      ),
    ];
  }

  @override
  Future<List<NotificationTemplate>> fetchTemplates() async {
    return const [
      NotificationTemplate(
        id: 'tmpl-1',
        rtId: 'rt-1',
        eventType: 'manual_push',
        name: 'Himbauan Kerja Bakti RT 002',
        title: 'Kerja Bakti Minggu Pagi',
        body:
            'Mari bersama-sama membersihkan saluran air dan taman blok A-D mulai pukul 07.00 WIB.',
        imageUrl: '/api/v1/community/images/community/rt-1/notification/kb.png',
        isActive: true,
      ),
      NotificationTemplate(
        id: 'tmpl-2',
        rtId: 'rt-1',
        eventType: 'iuran_payment_reminder',
        name: 'Pengingat Iuran Kas RT',
        title: 'Tagihan Iuran Bulan {{period}}',
        body:
            'Yth. Bapak/Ibu warga RT 002, mohon melakukan pembayaran iuran sebelum tanggal 10.',
        isActive: true,
      ),
      NotificationTemplate(
        id: 'tmpl-3',
        rtId: 'rt-1',
        eventType: 'manual_push',
        name: 'Siaran Keamanan Lingkungan',
        title: 'Penutupan Portal Pukul 23.00 WIB',
        body:
            'Portal utama akan ditutup mulai pukul 23.00 WIB demi keamanan lingkungan.',
        isActive: false,
      ),
    ];
  }

  @override
  Future<NotificationLogsResponse> fetchManagementHistory({
    required String category,
    int page = 1,
    int perPage = 50,
  }) async {
    return const NotificationLogsResponse(
      data: [],
      page: 1,
      perPage: 50,
      total: 0,
      totalPages: 1,
    );
  }
}

class _FakeCommunityRepo extends CommunityRepository {
  _FakeCommunityRepo() : super(dio: Dio());

  @override
  Future<CommunityPage<CommunityActivity>> listActivities({
    String? status,
    String? publicationStatus,
    int page = 1,
    int pageSize = 20,
  }) async {
    return CommunityPage(
      items: [
        CommunityActivity(
          id: 'act-1',
          rtId: 'rt-1',
          title: 'Kerja Bakti Pembersihan Saluran Air',
          description:
              'Gotong royong membersihkan drainase utama dan pemangkasan pohon di sepanjang jalan RT 002.',
          startDate: DateTime(2026, 11, 15, 7, 0),
          endDate: DateTime(2026, 11, 15, 11, 0),
          location: 'Balai Warga & Sepanjang Blok A-C',
          organizer: 'Seksi Kebersihan RT 002',
          picName: 'Pak Hendra',
          budgetPlanned: '750000',
          posterUrl: '/api/v1/community/images/community/rt-1/activity/p1.png',
          status: 'upcoming',
          publicationStatus: 'published',
          showOnBanner: true,
          createdAt: DateTime(2026, 11, 10, 8, 0),
          updatedAt: DateTime(2026, 11, 10, 8, 0),
        ),
        CommunityActivity(
          id: 'act-2',
          rtId: 'rt-1',
          title: 'Rapat Koordinasi Persiapan Akhir Tahun',
          description: '',
          startDate: DateTime(2026, 11, 20, 19, 30),
          location: 'Posko Sekretariat RT 002',
          organizer: 'Pengurus Inti RT 002',
          status: 'upcoming',
          publicationStatus: 'draft',
          showOnBanner: false,
          createdAt: DateTime(2026, 11, 10, 9, 0),
          updatedAt: DateTime(2026, 11, 10, 9, 0),
        ),
        CommunityActivity(
          id: 'act-3',
          rtId: 'rt-1',
          title: 'Posyandu Lansia & Balita Bulanan',
          description:
              'Pemeriksaan kesehatan rutin, penimbangan balita, serta pemberian vitamin gratis.',
          startDate: DateTime(2026, 11, 5, 8, 0),
          endDate: DateTime(2026, 11, 5, 11, 30),
          location: 'Taman Bermain RT 002',
          organizer: 'Kader PKK & Seksi Sosial',
          picName: 'Ibu Sari',
          budgetPlanned: '500000',
          status: 'completed',
          publicationStatus: 'published',
          showOnBanner: false,
          createdAt: DateTime(2026, 11, 1, 8, 0),
          updatedAt: DateTime(2026, 11, 5, 12, 0),
        ),
      ],
      page: 1,
      perPage: 20,
      total: 3,
      totalPages: 1,
    );
  }

  @override
  Future<CommunityPage<CommunityAnnouncement>> listAnnouncements({
    String? status,
    int page = 1,
    int pageSize = 10,
  }) async {
    return CommunityPage(
      items: [
        CommunityAnnouncement(
          id: 'ann-1',
          rtId: 'rt-1',
          title: 'Jadwal Pemadaman Listrik Bergilir Blok A & B',
          body:
              'PLN akan melakukan pemeliharaan gardu distribusi pada hari Sabtu pukul 09.00 - 13.00 WIB. Mohon warga mengantisipasi kebutuhan daya.',
          imageUrl:
              '/api/v1/community/images/community/rt-1/announcement/pln.png',
          deepLink: '/kegiatan',
          status: 'active',
          priority: 10,
          displayOrder: 1,
          scheduledStart: DateTime(2026, 11, 10, 7, 0),
          scheduledEnd: DateTime(2026, 11, 16, 23, 59),
          publishedAt: DateTime(2026, 11, 10, 8, 0),
          createdAt: DateTime(2026, 11, 10, 8, 0),
          updatedAt: DateTime(2026, 11, 10, 8, 0),
        ),
        CommunityAnnouncement(
          id: 'ann-2',
          rtId: 'rt-1',
          title: 'Rancangan Penyesuaian Iuran Keamanan 2027',
          body:
              'Draf pengumuman hasil musyawarah pengurus terkait penyesuaian operasional pos keamanan.',
          deepLink: '/iuran',
          status: 'draft',
          priority: 5,
          displayOrder: 2,
          createdAt: DateTime(2026, 11, 11, 9, 0),
          updatedAt: DateTime(2026, 11, 11, 9, 0),
        ),
        CommunityAnnouncement(
          id: 'ann-3',
          rtId: 'rt-1',
          title: 'Pendaftaran Lomba Kebersihan Antar Gang',
          body:
              'Pengumuman terjadwal untuk pembukaan lomba kebersihan bulan depan.',
          imageUrl:
              '/api/v1/community/images/community/rt-1/announcement/lomba.png',
          status: 'scheduled',
          priority: 8,
          displayOrder: 3,
          scheduledStart: DateTime(2026, 11, 25, 7, 0),
          scheduledEnd: DateTime(2026, 12, 5, 23, 59),
          createdAt: DateTime(2026, 11, 11, 10, 0),
          updatedAt: DateTime(2026, 11, 11, 10, 0),
        ),
      ],
      page: 1,
      perPage: 10,
      total: 3,
      totalPages: 1,
    );
  }
}

Future<void> _loadRealFonts() async {
  const fontDir =
      '/Users/Andriyan/Documents/data/homework/flutter/perpus-digital/flutter/bin/cache/artifacts/material_fonts';
  final regularFile = File('$fontDir/Roboto-Regular.ttf');
  if (!regularFile.existsSync()) return;

  final robotoLoader = FontLoader('Roboto');
  final montserratLoader = FontLoader('Montserrat');
  for (final name in [
    'Roboto-Regular.ttf',
    'Roboto-Medium.ttf',
    'Roboto-Bold.ttf',
  ]) {
    final f = File('$fontDir/$name');
    if (f.existsSync()) {
      final bytes = f.readAsBytesSync();
      robotoLoader.addFont(
        Future.value(ByteData.view(Uint8List.fromList(bytes).buffer)),
      );
      montserratLoader.addFont(
        Future.value(ByteData.view(Uint8List.fromList(bytes).buffer)),
      );
    }
  }
  await robotoLoader.load();
  await montserratLoader.load();

  final materialFont = File('$fontDir/MaterialIcons-Regular.otf');
  if (materialFont.existsSync()) {
    final iconLoader = FontLoader('MaterialIcons');
    final iconBytes = materialFont.readAsBytesSync();
    iconLoader.addFont(
      Future.value(ByteData.view(Uint8List.fromList(iconBytes).buffer)),
    );
    await iconLoader.load();
  }
}

Future<void> _capturePng(
  WidgetTester tester,
  GlobalKey boundaryKey,
  String fileName,
) async {
  final boundary =
      boundaryKey.currentContext!.findRenderObject() as RenderRepaintBoundary;
  final image = await boundary.toImage(pixelRatio: 2.0);
  final byteData = await image.toByteData(format: ui.ImageByteFormat.png);
  final file = File('$_artifactDir/$fileName');
  await file.writeAsBytes(byteData!.buffer.asUint8List());
}

void main() {
  setUpAll(() async {
    await _loadRealFonts();
  });

  final adminUser = <String, dynamic>{
    'id': 'u-ketua',
    'name': 'Budi Santoso',
    'email': 'ketua@rt02.local',
    'system_role': 'super_admin',
    'role': AppRole.pengurus,
    'jabatan': 'ketua',
    'effective_permissions': [
      'notification.manage',
      'notification.read',
      'activities.read',
      'activities.create',
      'activities.update',
      'activities.delete',
      'announcements.read',
      'announcements.create',
      'announcements.update',
      'announcements.delete',
    ],
    'rt': '002',
    'rw': '016',
    'rt_id': 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
  };

  testWidgets('Capture Android Reference & Kegiatan/Pengumuman screenshots', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(430, 960));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    if (_mode == 'before') {
      final notifKey = GlobalKey();
      final fakeNotifRepo = _FakeNotifRepo();
      await tester.pumpWidget(
        ProviderScope(
          key: UniqueKey(),
          overrides: [
            authRepositoryProvider.overrideWith(
              (ref) => _MockAuthRepo(adminUser),
            ),
            notificationRepositoryProvider.overrideWithValue(fakeNotifRepo),
            notificationSystemTypesProvider.overrideWith(
              (ref) => fakeNotifRepo.fetchSystemTypes(),
            ),
            notificationTemplatesProvider.overrideWith(
              (ref) => fakeNotifRepo.fetchTemplates(),
            ),
          ],
          child: MaterialApp(
            debugShowCheckedModeBanner: false,
            theme: AppTheme.darkTheme,
            home: RepaintBoundary(
              key: notifKey,
              child: const NotificationManagementScreen(),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      // Switch primary tab to TEMPLATE, then sub-tab to KUSTOM
      await tester.tap(find.text('TEMPLATE'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('KUSTOM'));
      await tester.pumpAndSettle();

      await tester.runAsync(() async {
        await _capturePng(tester, notifKey, 'android_ref_notifikasi.png');
      });
    }

    // Capture KegiatanScreen
    final kegiatanKey = GlobalKey();
    await tester.pumpWidget(
      ProviderScope(
        key: UniqueKey(),
        overrides: [
          authRepositoryProvider.overrideWith(
            (ref) => _MockAuthRepo(adminUser),
          ),
          communityRepositoryProvider.overrideWithValue(_FakeCommunityRepo()),
        ],
        child: MaterialApp(
          debugShowCheckedModeBanner: false,
          theme: AppTheme.darkTheme,
          home: RepaintBoundary(
            key: kegiatanKey,
            child: const KegiatanScreen(loadInTest: true),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.runAsync(() async {
      await _capturePng(
        tester,
        kegiatanKey,
        'android_${_mode}_kegiatan.png',
      );
    });

    // Capture PengumumanScreen
    final pengumumanKey = GlobalKey();
    await tester.pumpWidget(
      ProviderScope(
        key: UniqueKey(),
        overrides: [
          authRepositoryProvider.overrideWith(
            (ref) => _MockAuthRepo(adminUser),
          ),
          communityRepositoryProvider.overrideWithValue(_FakeCommunityRepo()),
        ],
        child: MaterialApp(
          debugShowCheckedModeBanner: false,
          theme: AppTheme.darkTheme,
          home: RepaintBoundary(
            key: pengumumanKey,
            child: const PengumumanScreen(loadInTest: true),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.runAsync(() async {
      await _capturePng(
        tester,
        pengumumanKey,
        'android_${_mode}_pengumuman.png',
      );
    });
  });

  testWidgets(
    'Android Kegiatan & Pengumuman list standardization and action preservation',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(430, 960));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      // 1. KegiatanScreen assertions
      await tester.pumpWidget(
        ProviderScope(
          key: UniqueKey(),
          overrides: [
            authRepositoryProvider.overrideWith(
              (ref) => _MockAuthRepo(adminUser),
            ),
            communityRepositoryProvider.overrideWithValue(_FakeCommunityRepo()),
          ],
          child: MaterialApp(
            debugShowCheckedModeBanner: false,
            theme: AppTheme.darkTheme,
            home: const KegiatanScreen(loadInTest: true),
          ),
        ),
      );
      await tester.pumpAndSettle();

      // No CommunityImage thumbnail in the Kegiatan list view
      expect(find.byType(CommunityImage), findsNothing);
      // Attachment badge shown for item with posterUrl
      expect(find.text('Gambar Terlampir'), findsOneWidget);
      // All 3 items rendered with full/partial metadata and status badges
      expect(
        find.text('Kerja Bakti Pembersihan Saluran Air'),
        findsOneWidget,
      );
      expect(
        find.text('Rapat Koordinasi Persiapan Akhir Tahun'),
        findsOneWidget,
      );
      expect(find.text('Posyandu Lansia & Balita Bulanan'), findsOneWidget);
      expect(find.text('Banner Beranda'), findsOneWidget);
      expect(find.text('Belum Terbit'), findsOneWidget);
      expect(
        find.text('Penyelenggara: Seksi Kebersihan RT 002'),
        findsOneWidget,
      );
      expect(find.text('PIC: Pak Hendra'), findsOneWidget);
      expect(find.text('Anggaran: Rp 750.000'), findsOneWidget);

      // Action buttons preserved: Detail (3), Ubah (3), Hapus (1 — draft only)
      expect(find.byTooltip('Detail'), findsNWidgets(3));
      expect(find.byTooltip('Ubah'), findsNWidgets(3));
      expect(find.byTooltip('Hapus'), findsOneWidget);

      // Tapping Ubah on first item opens Edit dialog where CommunityImage preview IS preserved
      await tester.tap(find.byTooltip('Ubah').first);
      await tester.pumpAndSettle();
      expect(find.text('Ubah Kegiatan'), findsOneWidget);
      expect(find.byType(CommunityImage), findsOneWidget);
      await tester.tap(find.text('Batal'));
      await tester.pumpAndSettle();

      // Tapping Hapus on draft item opens delete confirmation dialog
      await tester.tap(find.byTooltip('Hapus'));
      await tester.pumpAndSettle();
      expect(find.text('Hapus Kegiatan?'), findsOneWidget);
      await tester.tap(find.text('Batal'));
      await tester.pumpAndSettle();

      // 2. PengumumanScreen assertions
      await tester.pumpWidget(
        ProviderScope(
          key: UniqueKey(),
          overrides: [
            authRepositoryProvider.overrideWith(
              (ref) => _MockAuthRepo(adminUser),
            ),
            communityRepositoryProvider.overrideWithValue(_FakeCommunityRepo()),
          ],
          child: MaterialApp(
            debugShowCheckedModeBanner: false,
            theme: AppTheme.darkTheme,
            home: const PengumumanScreen(loadInTest: true),
          ),
        ),
      );
      await tester.pumpAndSettle();

      // No CommunityImage thumbnail in the Pengumuman list view
      expect(find.byType(CommunityImage), findsNothing);
      // Attachment badge shown for the 2 items with imageUrl
      expect(find.text('Gambar Terlampir'), findsNWidgets(2));
      // Priority, deepLink, schedule window, and status badges rendered cleanly
      expect(
        find.text('Jadwal Pemadaman Listrik Bergilir Blok A & B'),
        findsOneWidget,
      );
      expect(
        find.text('Rancangan Penyesuaian Iuran Keamanan 2027'),
        findsOneWidget,
      );
      expect(
        find.text('Pendaftaran Lomba Kebersihan Antar Gang'),
        findsOneWidget,
      );
      expect(find.text('Penting'), findsOneWidget);
      expect(find.text('/kegiatan'), findsOneWidget);
      expect(find.text('/iuran'), findsOneWidget);
      expect(find.text('Prioritas: 10'), findsOneWidget);
      expect(find.text('Prioritas: 5'), findsOneWidget);
      expect(find.text('Prioritas: 8'), findsOneWidget);
    },
  );
}
