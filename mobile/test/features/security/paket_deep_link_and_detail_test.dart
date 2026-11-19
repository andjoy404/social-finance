import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:social_finance/app/app.dart';
import 'package:social_finance/app/navigation_shell.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/notification/data/models/notification_models.dart';
import 'package:social_finance/features/notification/data/notification_repository.dart';
import 'package:social_finance/features/notification/presentation/screens/notification_management_screen.dart';
import 'package:social_finance/features/security/data/security_api_client.dart';
import 'package:social_finance/features/security/data/security_models.dart';
import 'package:social_finance/features/security/presentation/providers/security_providers.dart';
import 'package:social_finance/features/security/presentation/screens/paket_detail_screen.dart';
import 'package:social_finance/features/security/presentation/screens/security_management_screen.dart';
import 'package:social_finance/features/security/presentation/widgets/paket_detail_content.dart';
import 'package:social_finance/features/security/presentation/widgets/security_photo_viewer.dart';

class _MockAuthRepo extends StateNotifier<AsyncValue<Map<String, dynamic>?>?>
    implements AuthRepository {
  _MockAuthRepo(super.state);

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _MockNotificationRepo implements NotificationRepository {
  final List<NotificationLogEntry> mockLogs = [];

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);

  @override
  Future<NotificationLogsResponse> fetchLogs({
    int page = 1,
    int perPage = 20,
    String? eventType,
    String? status,
  }) async {
    return NotificationLogsResponse(
      data: mockLogs,
      page: 1,
      perPage: 20,
      total: mockLogs.length,
      totalPages: 1,
    );
  }

  @override
  Future<NotificationLogsResponse> fetchManagementHistory({
    required String category,
    int page = 1,
    int perPage = 50,
  }) async {
    return NotificationLogsResponse(
      data: mockLogs,
      page: 1,
      perPage: 50,
      total: mockLogs.length,
      totalPages: 1,
    );
  }

  @override
  Future<void> markAsRead(String id) async {
    final idx = mockLogs.indexWhere((l) => l.id == id);
    if (idx >= 0) {
      mockLogs[idx] = mockLogs[idx].copyWith(isRead: true);
    }
  }

  @override
  Future<int> markAllAsRead() async => 0;
}

class _MockSecurityApiClient extends SecurityApiClient {
  _MockSecurityApiClient()
      : super(ApiClient(baseUrl: 'https://example.test'));

  final List<PaketReport> mockReports = [];

  @override
  Future<List<PaketReport>> listPaketReports({String? status}) async {
    return mockReports;
  }

  @override
  Future<PaketReport> getPaketReport(String reportId) async {
    return mockReports.firstWhere(
      (r) => r.id == reportId,
      orElse: () => throw Exception('Laporan paket tidak ditemukan'),
    );
  }

  @override
  Future<List<SecurityOfficer>> listManagerOfficers() async => const [];
  @override
  Future<List<ShiftConfig>> listManagerShifts() async => const [];
  @override
  Future<List<SecuritySchedule>> listManagerSchedules({
    String? dateFrom,
    String? dateTo,
    String? userId,
    int page = 1,
  }) async => const [];
  @override
  Future<List<AttendanceRecord>> listManagerAttendance({
    String? dateFrom,
    String? dateTo,
    String? userId,
    String? status,
    int page = 1,
  }) async => const [];
  @override
  Future<List<AttendanceCorrection>> listManagerCorrections() async => const [];
  @override
  Future<List<SecurityDevice>> listManagerDevices() async => const [];
  @override
  Future<GuestBookPage> listManagerGuestBooks({
    String? search,
    int page = 1,
    int pageSize = 20,
  }) async => const GuestBookPage([], 0, 1);
  @override
  Future<List<PatrolTemplate>> listPatrolTemplates() async => const [];
  @override
  Future<List<PatrolBundle>> listPatrolBundles() async => const [];
}

void main() {
  final wargaUser = {
    'id': 'usr-warga-1',
    'name': 'Warga Bambang',
    'role': 'warga',
    'system_role': 'warga',
    'rt_id': 'rt-001',
    'effective_permissions': <String>[],
  };

  final securityUser = {
    'id': 'usr-sec-1',
    'name': 'Sutrisno',
    'role': 'pengurus',
    'system_role': 'pengurus',
    'jabatan': 'keamanan',
    'rt_id': 'rt-001',
    'effective_permissions': [
      'security.schedule.read',
      'patrol.read',
      'paket.read',
      'paket.create',
      'paket.status_change',
    ],
  };

  final samplePaket = PaketReport(
    id: 'rep-101',
    rtid: 'rt-001',
    receiverWargaId: 'res-warga-1',
    receiverName: 'Bambang Tri',
    receiverHouseNumber: 'B3/12',
    createdBy: 'usr-sec-1',
    createdByName: 'Sutrisno',
    overallStatus: 'dititipkan',
    items: [
      PaketItem(
        id: 'it-1',
        paketReportId: 'rep-101',
        description: 'Paket Dokumen Penting',
        status: 'dititipkan',
        createdAt: DateTime(2026, 11, 19, 10, 0),
        photos: [
          PaketPhoto(
            id: 'ph-1',
            paketItemId: 'it-1',
            paketReportId: 'rep-101',
            imageKey: 'community/rt-001/paket/doc.jpg',
            imageUrl: 'https://example.test/photos/ph-1.jpg',
            photoType: 'paket',
            capturedAt: DateTime(2026, 11, 19, 10, 0),
            createdAt: DateTime(2026, 11, 19, 10, 0),
          ),
        ],
      ),
    ],
    createdAt: DateTime(2026, 11, 19, 10, 0),
  );

  group('A. Warga Paket Deep Link & Full-Page Detail', () {
    testWidgets('Tapping paket notification opens PaketDetailScreen full page',
        (tester) async {
      final mockNotifRepo = _MockNotificationRepo();
      final mockSecApi = _MockSecurityApiClient();
      mockSecApi.mockReports.add(samplePaket);

      mockNotifRepo.mockLogs.add(
        NotificationLogEntry(
          id: 'notif-1',
          rtId: 'rt-001',
          userId: 'usr-warga-1',
          eventType: 'paket_received',
          title: 'Paket Baru Dititipkan',
          body: 'Ada paket baru yang dititipkan di pos keamanan untuk Anda.',
          payload: {'report_id': 'rep-101'},
          variables: const {},
          recipientsCount: 1,
          createdAt: DateTime(2026, 11, 19, 10, 5),
          isRead: false,
        ),
      );

      final router = GoRouter(
        initialLocation: '/home/notifikasi',
        routes: [
          GoRoute(
            path: '/home/notifikasi',
            builder: (ctx, state) => const NotificationManagementScreen(),
          ),
          GoRoute(
            path: '/home/paket/detail/:id',
            builder: (ctx, state) => PaketDetailScreen(
              reportId: state.pathParameters['id']!,
            ),
          ),
        ],
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authRepositoryProvider.overrideWith(
              (ref) => _MockAuthRepo(AsyncData(wargaUser)),
            ),
            notificationRepositoryProvider.overrideWithValue(mockNotifRepo),
            securityApiClientProvider.overrideWithValue(mockSecApi),
          ],
          child: MaterialApp.router(
            routerConfig: router,
          ),
        ),
      );

      await tester.pumpAndSettle();

      // Verify notification appears in history
      expect(find.text('Paket Baru Dititipkan'), findsOneWidget);
      expect(find.byKey(const Key('notification_log_item_notif-1')), findsOneWidget);

      // Tap notification
      await tester.tap(find.byKey(const Key('notification_log_item_notif-1')));
      await tester.pumpAndSettle();

      // Verify Detail Paket full page is opened
      expect(find.text('Detail Paket'), findsOneWidget);
      expect(find.text('Bambang Tri'), findsOneWidget);
      expect(find.text('Rumah: B3/12'), findsOneWidget);
      expect(find.byKey(const Key('paket_item_desc_it-1')), findsOneWidget);
      expect(find.text('Dititipkan'), findsWidgets);

      // Verify Back button returns to Notification screen
      final backButton = find.byKey(const Key('paket_detail_back_button'));
      expect(backButton, findsOneWidget);
      await tester.tap(backButton);
      await tester.pumpAndSettle();

      // We should be back on Notification history
      expect(find.byType(NotificationManagementScreen), findsOneWidget);
      expect(find.text('NOTIFIKASI'), findsWidgets);
    });

    testWidgets('Thumbnail tap opens SecurityPhotoViewerScreen and back returns to detail',
        (tester) async {
      final mockSecApi = _MockSecurityApiClient();
      mockSecApi.mockReports.add(samplePaket);

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authRepositoryProvider.overrideWith(
              (ref) => _MockAuthRepo(AsyncData(wargaUser)),
            ),
            securityApiClientProvider.overrideWithValue(mockSecApi),
          ],
          child: const MaterialApp(
            home: PaketDetailScreen(reportId: 'rep-101'),
          ),
        ),
      );

      await tester.pumpAndSettle();

      // Find photo thumbnail
      final thumbFinder = find.byKey(const Key('paket_photo_thumb_ph-1'));
      expect(thumbFinder, findsOneWidget);

      // Tap thumbnail
      await tester.tap(thumbFinder);
      await tester.pumpAndSettle();

      // Verify image viewer opened
      expect(find.byType(SecurityPhotoViewerScreen), findsOneWidget);
      expect(find.byType(InteractiveViewer), findsOneWidget);

      // Tap close button on image viewer
      final closeBtn = find.byTooltip('Tutup');
      expect(closeBtn, findsOneWidget);
      await tester.tap(closeBtn);
      await tester.pumpAndSettle();

      // Verify we returned to PaketDetailScreen without closing it
      expect(find.byType(SecurityPhotoViewerScreen), findsNothing);
      expect(find.text('Detail Paket'), findsOneWidget);
      expect(find.text('Bambang Tri'), findsOneWidget);
    });

    testWidgets('Warga has NO Riwayat Paket tab in NavigationShell',
        (tester) async {
      final router = GoRouter(
        initialLocation: '/home',
        routes: [
          ShellRoute(
            builder: (context, state, child) =>
                MainNavigationShell(child: child),
            routes: [
              GoRoute(
                path: '/home',
                builder: (context, state) => const SizedBox.shrink(),
              ),
            ],
          ),
        ],
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authRepositoryProvider.overrideWith(
              (ref) => _MockAuthRepo(AsyncData(wargaUser)),
            ),
          ],
          child: MaterialApp.router(
            routerConfig: router,
          ),
        ),
      );

      await tester.pumpAndSettle();

      // Verify Paket tab does NOT exist for Warga
      expect(find.text('Paket'), findsNothing);
      expect(find.text('Riwayat Paket'), findsNothing);
    });
  });

  group('B. Petugas Keamanan Daftar Paket & Modal Popup', () {
    testWidgets('Tapping package card opens popup modal with same detail content',
        (tester) async {
      final mockSecApi = _MockSecurityApiClient();
      mockSecApi.mockReports.add(samplePaket);

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authRepositoryProvider.overrideWith(
              (ref) => _MockAuthRepo(AsyncData(securityUser)),
            ),
            securityApiClientProvider.overrideWithValue(mockSecApi),
          ],
          child: const MaterialApp(
            home: SecurityManagementScreen(
              mode: SecurityManagementMode.officerPaket,
            ),
          ),
        ),
      );

      await tester.pumpAndSettle();

      // Switch to tab "Daftar Paket"
      expect(find.text('Daftar Paket'), findsOneWidget);
      await tester.tap(find.text('Daftar Paket'));
      await tester.pumpAndSettle();

      // Verify Daftar Paket is displayed
      expect(find.text('Bambang Tri (B3/12)'), findsOneWidget);

      // Tap on card to open popup modal
      final cardTap = find.byKey(const Key('paket_report_tap_rep-101'));
      expect(cardTap, findsOneWidget);
      await tester.tap(cardTap);
      await tester.pumpAndSettle();

      // Modal is open
      expect(find.byKey(const Key('paket_modal_close_button')), findsOneWidget);
      expect(find.byType(PaketDetailContent), findsOneWidget);
      expect(find.text('Bambang Tri'), findsOneWidget);
      expect(find.text('Rumah: B3/12'), findsOneWidget);
      expect(find.byKey(const Key('paket_item_desc_it-1')), findsOneWidget);

      // Close modal
      await tester.tap(find.byKey(const Key('paket_modal_close_button')));
      await tester.pumpAndSettle();

      // Modal is closed, Daftar Paket remains intact
      expect(find.byKey(const Key('paket_modal_close_button')), findsNothing);
      expect(find.text('Bambang Tri (B3/12)'), findsOneWidget);
    });
  });

  group('C. Error & Retry Handling', () {
    testWidgets('PaketDetailScreen displays retry button on error',
        (tester) async {
      final mockSecApi = _MockSecurityApiClient();
      // Empty reports -> triggers not found error

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authRepositoryProvider.overrideWith(
              (ref) => _MockAuthRepo(AsyncData(wargaUser)),
            ),
            securityApiClientProvider.overrideWithValue(mockSecApi),
          ],
          child: const MaterialApp(
            home: PaketDetailScreen(reportId: 'rep-missing'),
          ),
        ),
      );

      await tester.pumpAndSettle();

      expect(find.text('Gagal memuat detail paket'), findsOneWidget);
      expect(find.byKey(const Key('paket_detail_retry_btn')), findsOneWidget);
    });
  });
}
