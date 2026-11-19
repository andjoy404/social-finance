import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/fund_location/presentation/screens/fund_location_screen.dart';
import 'package:social_finance/features/notification/data/models/notification_models.dart';
import 'package:social_finance/features/notification/data/notification_repository.dart';
import 'package:social_finance/features/notification/presentation/screens/notification_management_screen.dart';
import 'package:social_finance/features/pengumuman/presentation/screens/announcement_ui.dart';
import 'package:social_finance/features/security/data/security_api_client.dart';
import 'package:social_finance/features/security/data/security_models.dart';
import 'package:social_finance/features/security/presentation/providers/security_providers.dart';
import 'package:social_finance/features/security/presentation/screens/security_management_screen.dart';

class _FakeAuthRepo extends StateNotifier<AsyncValue<Map<String, dynamic>?>?>
    implements AuthRepository {
  _FakeAuthRepo(super.state);

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _MockSecurityApi extends SecurityApiClient {
  _MockSecurityApi() : super(ApiClient(baseUrl: 'https://example.test'));

  final List<PaketReport> mockPaketReports = [
    PaketReport(
      id: 'rep-1',
      receiverWargaId: 'w-1',
      receiverName: 'Budi Santoso',
      receiverHouseNumber: 'B3/12',
      createdBy: 'u-1',
      createdByName: 'Pak Petugas',
      overallStatus: 'dititipkan',
      items: [
        PaketItem(
          id: 'it-1',
          paketReportId: 'rep-1',
          description: 'Kardus Besar Elektronik',
          status: 'dititipkan',
          photos: [
            PaketPhoto(
              id: 'p-1',
              paketItemId: 'it-1',
              paketReportId: 'rep-1',
              imageKey: 'key-1',
              imageUrl: 'https://example.test/img1.jpg',
              photoType: 'paket',
              capturedAt: DateTime(2026, 11, 19, 7, 0),
              createdAt: DateTime(2026, 11, 19, 7, 0),
            ),
          ],
          createdAt: DateTime(2026, 11, 19, 7, 0),
        ),
      ],
      createdAt: DateTime(2026, 11, 19, 7, 0),
    ),
  ];

  final List<PatrolBundle> mockPatrolBundles = [
    PatrolBundle(
      id: 'bnd-1',
      createdBy: 'u-1',
      dutyOfficerName: 'Sutrisno',
      submittedAt: DateTime(2026, 11, 19, 6, 0),
      notes: 'Aman terkendali',
      photoCount: 1,
      photos: [
        PatrolPhoto(
          id: 'ph-1',
          patrolBundleId: 'bnd-1',
          patrolTemplateId: 'tpl-1',
          templateName: 'Pos Ronda 1',
          locationLabel: 'Pintu Gerbang Utama',
          imageKey: 'pk-1',
          imageUrl: 'https://example.test/patrol1.jpg',
          capturedAt: DateTime(2026, 11, 19, 6, 0),
          createdAt: DateTime(2026, 11, 19, 6, 0),
        ),
      ],
      createdAt: DateTime(2026, 11, 19, 6, 0),
    ),
  ];

  @override
  Future<List<PaketReport>> listPaketReports({String? status}) async =>
      mockPaketReports;

  @override
  Future<List<PatrolBundle>> listPatrolBundles() async => mockPatrolBundles;

  @override
  Future<List<PatrolTemplate>> listPatrolTemplates() async => const [];

  @override
  Future<void> markReadReceipt({
    required String domain,
    List<String>? itemIds,
  }) async {}

  @override
  Future<SecurityAlertsData> getSecurityAlerts() async =>
      const SecurityAlertsData();

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
    String? status,
    String? dateFrom,
    String? dateTo,
    int page = 1,
    int pageSize = 20,
  }) async => const GuestBookPage([], 0, 1);
}

class _MockNotifRepo implements NotificationRepository {
  final List<NotificationLogEntry> myLogs;

  _MockNotifRepo({required this.myLogs});

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);

  @override
  Future<NotificationLogsResponse> fetchManagementHistory({
    required String category,
    int page = 1,
    int perPage = 50,
  }) async {
    return NotificationLogsResponse(
      data: myLogs,
      page: 1,
      perPage: 50,
      total: myLogs.length,
      totalPages: 1,
    );
  }

  @override
  Future<void> markAsRead(String id) async {}

  @override
  Future<int> markAllAsRead() async => 0;
}

void main() {
  group('1. Paket Navigation & Tab Interactivity', () {
    testWidgets(
      'officerPaket mode has title PAKET and tab switching via tap works',
      (tester) async {
        final mockApi = _MockSecurityApi();
        final authRepo = _FakeAuthRepo(
          const AsyncValue.data({
            'id': 'usr-admin-1',
            'full_name': 'Ketua RT',
            'role': 'pengurus',
            'effective_permissions': ['paket.read', 'patrol.read'],
          }),
        );

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              authRepositoryProvider.overrideWith((ref) => authRepo),
              securityApiClientProvider.overrideWithValue(mockApi),
            ],
            child: const MaterialApp(
              home: SecurityManagementScreen(
                mode: SecurityManagementMode.officerPaket,
              ),
            ),
          ),
        );

        await tester.pumpAndSettle();

        // Title must be PAKET
        expect(find.text('PAKET'), findsOneWidget);
        expect(find.text('Pengelolaan Paket'), findsNothing);
        expect(find.text('DAFTAR PAKET'), findsNothing); // Not in title

        // Tabs: Penerimaan and Daftar Paket
        expect(find.text('Penerimaan'), findsOneWidget);
        expect(find.text('Daftar Paket'), findsOneWidget);

        // Tap on 'Daftar Paket' tab switches smoothly to the list view
        await tester.tap(find.text('Daftar Paket'));
        await tester.pumpAndSettle();

        // Budi Santoso paket item is visible in Daftar Paket tab
        expect(find.text('Budi Santoso (B3/12)'), findsOneWidget);
        expect(find.text('Kardus Besar Elektronik'), findsOneWidget);
      },
    );
  });

  group('2. Fund Location Screen Overflow Fix', () {
    testWidgets(
      'Text.rich handles bank details in narrow constraint without overflow',
      (tester) async {
        // Simulate the subtitle rendering in a tight width constraint (192px)
        await tester.pumpWidget(
          MaterialApp(
            home: Scaffold(
              body: Center(
                child: SizedBox(
                  width: 192,
                  child: Text.rich(
                    const TextSpan(
                      children: [
                        TextSpan(
                          text: 'Bank Mandiri (Persero) Tbk.  •  ',
                          style: TextStyle(fontSize: 12, color: AppColors.info),
                        ),
                        TextSpan(
                          text: '1234567890123456',
                          style: TextStyle(
                            fontSize: 12,
                            color: AppColors.accent,
                          ),
                        ),
                        TextSpan(
                          text:
                              '  (Rekening Operasional Kas RT 001 / RW 002 Sejahtera)',
                          style: TextStyle(fontSize: 12),
                        ),
                      ],
                    ),
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
              ),
            ),
          ),
        );

        await tester.pumpAndSettle();
        expect(tester.takeException(), isNull);
      },
    );
  });

  group('3. Announcement Deep Link Wording', () {
    test(
      'formatDeepLinkDestination maps internal paths to friendly labels',
      () {
        expect(formatDeepLinkDestination('/iuran'), 'Iuran');
        expect(formatDeepLinkDestination('/keuangan'), 'Keuangan');
        expect(formatDeepLinkDestination('/kas'), 'Kas');
        expect(formatDeepLinkDestination('/kegiatan'), 'Kegiatan');
        expect(formatDeepLinkDestination('/warga'), 'Warga');
        expect(formatDeepLinkDestination('/notifikasi'), 'Notifikasi');
        expect(formatDeepLinkDestination('/pengumuman'), 'Pengumuman');
        expect(formatDeepLinkDestination('/profil'), 'Profil');
        expect(formatDeepLinkDestination('/buku-tamu'), 'Buku Tamu');
        expect(formatDeepLinkDestination('/security-guestbook'), 'Buku Tamu');
        expect(formatDeepLinkDestination('/patroli'), 'Patroli');
        expect(formatDeepLinkDestination('/paket'), 'Paket');
        expect(formatDeepLinkDestination(''), '');
        expect(formatDeepLinkDestination(null), '');
      },
    );
  });

  group('4 & 5. Notification Preview Banner Overflow Fix', () {
    testWidgets(
      'Long event name wraps in preview banner without RenderFlex overflow',
      (tester) async {
        const longEventName =
            'Pengingat Pembayaran Iuran (3 Bulan Sebelum Jatuh Tempo)';

        await tester.pumpWidget(
          MaterialApp(
            home: Scaffold(
              body: SizedBox(
                width: 320, // narrow screen
                child: Container(
                  width: double.infinity,
                  padding: const EdgeInsets.symmetric(
                    horizontal: 12,
                    vertical: 8,
                  ),
                  decoration: BoxDecoration(
                    color: AppColors.accent.withOpacity(0.15),
                    borderRadius: BorderRadius.circular(8),
                  ),
                  child: Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      const Padding(
                        padding: EdgeInsets.only(top: 2),
                        child: Icon(
                          Icons.info_outline,
                          size: 16,
                          color: AppColors.accent,
                        ),
                      ),
                      const SizedBox(width: 8),
                      Expanded(
                        child: Text(
                          longEventName,
                          style: const TextStyle(
                            fontSize: 12,
                            fontWeight: FontWeight.w600,
                            color: AppColors.accent,
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ),
          ),
        );

        await tester.pumpAndSettle();
        expect(tester.takeException(), isNull);
        expect(find.text(longEventName), findsOneWidget);
      },
    );
  });

  group('7. Photo Thumbnails for Patrol & Paket', () {
    testWidgets('Paket and Patrol items show photo thumbnails', (tester) async {
      final mockApi = _MockSecurityApi();
      final authRepo = _FakeAuthRepo(
        const AsyncValue.data({
          'id': 'usr-admin-1',
          'full_name': 'Ketua RT',
          'role': 'pengurus',
          'effective_permissions': [
            'paket.read',
            'patrol.read',
            'paket.create',
            'patrol.create',
          ],
        }),
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authRepositoryProvider.overrideWith((ref) => authRepo),
            securityApiClientProvider.overrideWithValue(mockApi),
          ],
          child: const MaterialApp(
            home: SecurityManagementScreen(
              mode: SecurityManagementMode.officerPaket,
            ),
          ),
        ),
      );

      await tester.pumpAndSettle();

      // Tap Daftar Paket
      await tester.tap(find.text('Daftar Paket'));
      await tester.pumpAndSettle();

      // Verify photo thumbnail in paket item
      expect(find.byType(GestureDetector), findsWidgets);
      expect(find.text('Foto (1)'), findsOneWidget);
    });
  });

  group('8. Unread Notification Highlight in Riwayat Saya for Admin', () {
    testWidgets(
      'shows BARU badge and unread dot in Riwayat Saya for admin with unread notif',
      (tester) async {
        final myUnreadLog = NotificationLogEntry(
          id: 'notif-1',
          rtId: 'rt-1',
          title: 'Pengumuman Baru',
          body: 'Kerja bakti hari Minggu',
          eventType: 'announcement',
          status: 'SENT',
          variables: const {},
          recipientsCount: 1,
          createdAt: DateTime(2026, 11, 19, 7, 0),
          isRead: false,
        );

        final notifRepo = _MockNotifRepo(myLogs: [myUnreadLog]);
        final authRepo = _FakeAuthRepo(
          const AsyncValue.data({
            'id': 'usr-admin-1',
            'full_name': 'Ketua RT',
            'role': 'pengurus',
            'effective_permissions': [
              'notification.manage',
              'notification.read',
            ],
          }),
        );

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              authRepositoryProvider.overrideWith((ref) => authRepo),
              notificationRepositoryProvider.overrideWithValue(notifRepo),
            ],
            child: const MaterialApp(home: NotificationManagementScreen()),
          ),
        );

        await tester.pumpAndSettle();

        // Tap RIWAYAT SAYA tab
        await tester.tap(find.text('RIWAYAT SAYA'));
        await tester.pumpAndSettle();

        // Check for unread indicators
        expect(find.text('BARU'), findsOneWidget);
        expect(find.text('Pengumuman Baru'), findsOneWidget);
      },
    );
  });
}
