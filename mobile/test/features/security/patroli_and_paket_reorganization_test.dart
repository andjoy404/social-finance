import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:social_finance/app/navigation_shell.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/api/device_id_service.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/notification/data/models/notification_models.dart';
import 'package:social_finance/features/notification/data/notification_repository.dart';
import 'package:social_finance/features/rw/data/rw_providers.dart';
import 'package:social_finance/features/security/data/security_api_client.dart';
import 'package:social_finance/features/security/data/security_models.dart';
import 'package:social_finance/features/security/presentation/providers/security_providers.dart';
import 'package:social_finance/features/security/presentation/screens/security_management_screen.dart';

class _FakeDeviceId extends DeviceIdService {
  @override
  Future<String> getDeviceId() async => 'test-device';
}

class _FakeAuthRepo extends AuthRepository {
  _FakeAuthRepo(Map<String, dynamic>? user)
      : super(
          authService: AuthService(
            ApiClient(baseUrl: 'http://test.local'),
            _FakeDeviceId(),
          ),
          apiClient: ApiClient(baseUrl: 'http://test.local'),
        ) {
    state = AsyncData(user);
  }
}

class _MockUnreadNotifier extends StateNotifier<int>
    implements NotificationUnreadCountNotifier {
  _MockUnreadNotifier(super.state);
  @override
  Future<void> refresh() async {}
  @override
  void decrement() {
    if (state > 0) state--;
  }

  @override
  void clear() {
    state = 0;
  }
}

class _MockSecurityApi extends SecurityApiClient {
  _MockSecurityApi({
    List<PatrolTemplate>? templates,
    List<PatrolBundle>? bundles,
    List<PaketReport>? pakets,
  })  : _templates = templates ??
            [
              PatrolTemplate(
                id: 'tpl-1',
                name: 'Gang Mawar 1',
                locationLabel: 'Pos Depan Utara',
                isActive: true,
                createdAt: DateTime(2026, 11, 1),
              ),
              PatrolTemplate(
                id: 'tpl-2',
                name: 'Gang Melati 2',
                locationLabel: 'Portal Selatan',
                isActive: true,
                createdAt: DateTime(2026, 11, 1),
              ),
            ],
        _bundles = bundles ??
            [
              PatrolBundle(
                id: 'bnd-1',
                createdBy: 'usr-satpam',
                createdByName: 'Pak Sutrisno',
                dutyOfficerName: 'Pak Sutrisno',
                submittedAt: DateTime(2026, 11, 20, 22, 15),
                notes: 'Situasi kondusif dan portal terkunci',
                createdAt: DateTime(2026, 11, 20, 22, 15),
                photoCount: 1,
                photos: [
                  PatrolPhoto(
                    id: 'ph-1',
                    patrolBundleId: 'bnd-1',
                    patrolTemplateId: 'tpl-1',
                    templateName: 'Gang Mawar 1',
                    imageKey: 'patrol/ph-1.jpg',
                    imageUrl: '',
                    capturedAt: DateTime(2026, 11, 20, 22, 15),
                    createdAt: DateTime(2026, 11, 20, 22, 15),
                  ),
                ],
              ),
            ],
        _pakets = pakets ??
            [
              PaketReport(
                id: 'rep-dititipkan',
                receiverWargaId: 'warga-1',
                receiverName: 'Budi Santoso',
                receiverHouseNumber: 'B3/12',
                createdBy: 'usr-satpam',
                createdByName: 'Pak Satpam',
                createdAt: DateTime.now().subtract(const Duration(hours: 26)),
                overallStatus: 'dititipkan',
                items: [
                  PaketItem(
                    id: 'item-1',
                    paketReportId: 'rep-dititipkan',
                    description: 'Kurir: JNE · Kardus Coklat',
                    status: 'dititipkan',
                    createdAt:
                        DateTime.now().subtract(const Duration(hours: 26)),
                    photos: const [],
                  ),
                ],
              ),
              PaketReport(
                id: 'rep-diambil',
                receiverWargaId: 'warga-2',
                receiverName: 'Siti Aminah',
                receiverHouseNumber: 'A1/05',
                createdBy: 'usr-satpam',
                createdByName: 'Pak Satpam',
                createdAt: DateTime.now().subtract(const Duration(hours: 48)),
                overallStatus: 'sudah_diambil',
                items: [
                  PaketItem(
                    id: 'item-2',
                    paketReportId: 'rep-diambil',
                    description: 'Kurir: SiCepat · Paket Sudah Diambil',
                    status: 'sudah_diambil',
                    createdAt:
                        DateTime.now().subtract(const Duration(hours: 48)),
                    statusChangedAt:
                        DateTime.now().subtract(const Duration(hours: 5)),
                    photos: const [],
                  ),
                ],
              ),
            ],
        super(ApiClient(baseUrl: 'http://test.local'));

  final List<PatrolTemplate> _templates;
  final List<PatrolBundle> _bundles;
  final List<PaketReport> _pakets;
  final List<String> readReceiptDomains = [];

  @override
  Future<void> markReadReceipt({
    required String domain,
    List<String>? itemIds,
  }) async {
    readReceiptDomains.add(domain);
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
  }) async =>
      const [];

  @override
  Future<List<AttendanceRecord>> listManagerAttendance({
    String? dateFrom,
    String? dateTo,
    String? userId,
    String? status,
    int page = 1,
  }) async =>
      const [];

  @override
  Future<List<AttendanceCorrection>> listManagerCorrections() async => const [];

  @override
  Future<List<SecurityDevice>> listManagerDevices() async => const [];

  @override
  Future<GuestBookPage> listManagerGuestBooks({
    String? search,
    int page = 1,
    String? status,
  }) async =>
      const GuestBookPage([], 0, 1);

  @override
  Future<List<PatrolTemplate>> listPatrolTemplates() async => _templates;

  @override
  Future<List<PatrolBundle>> listPatrolBundles() async => _bundles;

  @override
  Future<List<PaketReport>> listPaketReports({String? status}) async => _pakets;
}

void main() {
  const rwId = 'rw-test-01';

  final ketuaRtUser = <String, dynamic>{
    'id': 'usr-ketua-rt',
    'name': 'Ketua RT 02',
    'role': 'pengurus',
    'jabatan': 'ketua',
    'rt': '002',
    'rt_id': 'rt-02',
    'rw_id': rwId,
    'effective_permissions': [
      'finance.read',
      'security.schedule.read',
      'security.schedule.write',
      'security.device.manage',
      'security.correction',
      'security.guest_book',
      'patrol.read',
      'patrol.template.manage',
      'paket.read',
      'paket.status_change',
    ],
  };

  final humasUser = <String, dynamic>{
    'id': 'usr-humas-rt',
    'name': 'Humas RT 02',
    'role': 'pengurus',
    'jabatan': 'humas',
    'rt': '002',
    'rt_id': 'rt-02',
    'rw_id': rwId,
    // Even if legacy/unexpected paket.read were present, Humas MUST NOT see PAKET
    'effective_permissions': [
      'finance.read',
      'security.schedule.read',
      'patrol.read',
      'patrol.template.manage',
      'paket.read',
    ],
  };

  final securityOfficerUser = <String, dynamic>{
    'id': 'usr-satpam',
    'name': 'Petugas Keamanan',
    'role': 'perangkat',
    'jabatan': 'keamanan',
    'rt': '002',
    'rt_id': 'rt-02',
    'effective_permissions': [
      'warga.read',
      'security.attendance',
      'patrol.read',
      'patrol.create',
      'paket.read',
      'paket.create',
      'paket.status_change',
    ],
  };

  final superAdminUser = <String, dynamic>{
    'id': 'usr-sa',
    'name': 'Super Admin',
    'role': AppRole.superAdmin,
    'system_role': 'super_admin',
    'jabatan': null,
    'rt': '002',
    'rt_id': 'rt-02',
    'effective_permissions': [
      'security.schedule.read',
      'security.schedule.write',
      'security.device.manage',
      'security.correction',
      'patrol.read',
      'patrol.template.manage',
      'paket.read',
      'paket.create',
      'paket.status_change',
    ],
  };

  GoRouter buildAppRouter(Map<String, dynamic> user, {String initial = '/home'}) {
    return GoRouter(
      initialLocation: initial,
      redirect: (context, state) {
        final loc = state.uri.path;
        if (isHumasUser(user)) {
          if (loc == '/home/paket') return '/home';
          if (loc == '/home/security-management' &&
              state.uri.queryParameters['tab'] == 'paket') {
            return '/home/security-management';
          }
          if (loc == '/home/security-management' &&
              state.uri.queryParameters['tab'] == 'patrol') {
            return '/home/patroli';
          }
        }
        if (isKetuaRTUser(user) && loc == '/home/security-management') {
          if (state.uri.queryParameters['tab'] == 'patrol') {
            return '/home/patroli';
          }
          if (state.uri.queryParameters['tab'] == 'paket') {
            return '/home/paket';
          }
        }
        return null;
      },
      routes: [
        ShellRoute(
          builder: (context, state, child) => MainNavigationShell(child: child),
          routes: [
            GoRoute(
              path: '/home',
              builder: (context, state) =>
                  const Center(child: Text('Home Screen Body')),
            ),
            GoRoute(
              path: '/home/security-management',
              builder: (context, state) => const SecurityManagementScreen(
                mode: SecurityManagementMode.full,
              ),
            ),
            GoRoute(
              path: '/home/patroli',
              builder: (context, state) {
                final jab = (user['jabatan'] ?? '').toString().toLowerCase();
                final useManage = jab != 'keamanan' &&
                    (isKetuaRTUser(user) || isHumasUser(user));
                return SecurityManagementScreen(
                  mode: useManage
                      ? SecurityManagementMode.managePatrol
                      : SecurityManagementMode.officerPatrol,
                );
              },
            ),
            GoRoute(
              path: '/home/paket',
              builder: (context, state) {
                return SecurityManagementScreen(
                  mode: isKetuaRTUser(user)
                      ? SecurityManagementMode.managePaket
                      : SecurityManagementMode.officerPaket,
                );
              },
            ),
          ],
        ),
      ],
    );
  }

  Widget wrapWithRouter({
    required Map<String, dynamic> user,
    required GoRouter router,
    required _MockSecurityApi api,
    SecurityAlertsData alerts = const SecurityAlertsData(),
  }) {
    return ProviderScope(
      overrides: [
        authRepositoryProvider.overrideWith((_) => _FakeAuthRepo(user)),
        securityApiClientProvider.overrideWithValue(api),
        securityAlertsDataProvider.overrideWith((_) async => alerts),
        securityAlertsProvider.overrideWith((_) async => alerts.hasAlert),
        rwUnreadCountProvider(rwId).overrideWith((_) async => 0),
        notificationUnreadCountProvider.overrideWith(
          (_) => _MockUnreadNotifier(0),
        ),
      ],
      child: MaterialApp.router(routerConfig: router),
    );
  }

  group('Ketua RT — Reorganisasi PATROLI & Penyederhanaan PAKET', () {
    testWidgets(
      '1-5. Main menu PATROLI tersedia (dengan badge patroli), membuka Kelola Patroli (Template Gang & Riwayat), dan tab PATROLI di KEAMANAN tidak muncul',
      (tester) async {
        tester.view.physicalSize = const Size(800, 1200);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(tester.view.resetPhysicalSize);
        addTearDown(tester.view.resetDevicePixelRatio);

        final api = _MockSecurityApi();
        final router = buildAppRouter(ketuaRtUser);

        await tester.pumpWidget(
          wrapWithRouter(
            user: ketuaRtUser,
            router: router,
            api: api,
            alerts: const SecurityAlertsData(patrolUnread: 3),
          ),
        );
        await tester.pumpAndSettle();

        // Open Menu grid -> KEAMANAN, PATROLI, PAKET all available + patrol badge active
        await tester.tap(find.byKey(const Key('nav-item-menu')));
        await tester.pumpAndSettle();
        expect(find.byKey(const Key('nav-item-keamanan')), findsOneWidget);
        expect(find.byKey(const Key('nav-item-patroli')), findsOneWidget);
        expect(find.byKey(const Key('nav-item-paket')), findsOneWidget);
        expect(find.byKey(const Key('patrol-unread-dot')), findsOneWidget);

        // Tap Main Menu PATROLI -> directly opens Kelola Patroli (Template Gang + Riwayat)
        await tester.tap(find.byKey(const Key('nav-item-patroli')));
        await tester.pumpAndSettle();

        expect(find.text('Template Gang'), findsOneWidget);
        expect(find.text('Riwayat'), findsOneWidget);
        expect(find.text('Ambil Foto'), findsNothing);
        expect(find.text('Daftar Gang & Titik Kontrol (2)'), findsOneWidget);
        expect(find.text('Gang Mawar 1'), findsOneWidget);
        expect(find.text('Tambah Gang'), findsOneWidget);

        // Switch to Riwayat tab -> existing patrol history renders
        await tester.tap(find.text('Riwayat'));
        await tester.pumpAndSettle();
        expect(find.text('Pak Sutrisno'), findsOneWidget);
        expect(find.textContaining('Situasi kondusif'), findsOneWidget);

        // Now open KEAMANAN -> tab PATROLI & PAKET MUST NOT appear (Paket is in Main Menu)
        await tester.tap(find.byKey(const Key('nav-item-menu')));
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(const Key('nav-item-keamanan')));
        await tester.pumpAndSettle();

        final keamananTopTabBar = find.byType(TabBar).first;
        expect(
          find.descendant(of: keamananTopTabBar, matching: find.text('Kelola')),
          findsOneWidget,
        );
        expect(
          find.descendant(of: keamananTopTabBar, matching: find.text('Absensi')),
          findsOneWidget,
        );
        expect(
          find.descendant(of: keamananTopTabBar, matching: find.text('Perangkat')),
          findsOneWidget,
        );
        expect(
          find.descendant(of: keamananTopTabBar, matching: find.text('Patroli')),
          findsNothing,
        );
        expect(
          find.descendant(of: keamananTopTabBar, matching: find.text('Paket')),
          findsNothing,
        );
      },
    );

    testWidgets(
      '6-9. Main menu PAKET langsung menampilkan daftar paket titipan Pos Security tanpa tab Penerimaan, filter/detail/action & empty state bekerja',
      (tester) async {
        tester.view.physicalSize = const Size(800, 1200);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(tester.view.resetPhysicalSize);
        addTearDown(tester.view.resetDevicePixelRatio);

        final api = _MockSecurityApi();
        final router = buildAppRouter(ketuaRtUser);

        await tester.pumpWidget(
          wrapWithRouter(user: ketuaRtUser, router: router, api: api),
        );
        await tester.pumpAndSettle();

        // Tap Main Menu PAKET
        await tester.tap(find.byKey(const Key('nav-item-menu')));
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(const Key('nav-item-paket')));
        await tester.pumpAndSettle();

        // Penerimaan tab MUST NOT appear
        expect(find.text('Penerimaan'), findsNothing);
        expect(find.text('Simpan Penitipan Paket'), findsNothing);

        // Immediately shows ONLY packages deposited at Pos Security (dititipkan)
        expect(find.text('Budi Santoso (B3/12)'), findsOneWidget);
        expect(find.textContaining('Kardus Coklat'), findsOneWidget);
        expect(find.textContaining('> 24 Jam'), findsOneWidget);
        expect(find.text('Serah Terima'), findsOneWidget);
        // Already picked-up package is filtered out by default!
        expect(find.text('Siti Aminah (A1/05)'), findsNothing);

        // Switching filter chip to "Sudah Diambil" still works
        await tester.tap(find.widgetWithText(ChoiceChip, 'Sudah Diambil'));
        await tester.pumpAndSettle();
        expect(find.text('Siti Aminah (A1/05)'), findsOneWidget);
        expect(find.text('Budi Santoso (B3/12)'), findsNothing);

        // Switching back to "Dititipkan" and opening detail modal works
        await tester.tap(find.widgetWithText(ChoiceChip, 'Dititipkan'));
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(const Key('paket_report_tap_rep-dititipkan')));
        await tester.pumpAndSettle();
        expect(find.text('Detail Paket'), findsOneWidget);
        await tester.tap(find.byKey(const Key('paket_modal_close_button')));
        await tester.pumpAndSettle();

        // Empty state when no packages deposited at Pos Security
        final emptyApi = _MockSecurityApi(pakets: const []);
        await tester.pumpWidget(
          ProviderScope(
            key: const ValueKey('empty-paket-scope'),
            overrides: [
              authRepositoryProvider.overrideWith((_) => _FakeAuthRepo(ketuaRtUser)),
              securityApiClientProvider.overrideWithValue(emptyApi),
              securityAlertsDataProvider.overrideWith(
                (_) async => const SecurityAlertsData(),
              ),
            ],
            child: const MaterialApp(
              home: SecurityManagementScreen(
                mode: SecurityManagementMode.managePaket,
              ),
            ),
          ),
        );
        await tester.pumpAndSettle();
        expect(
          find.text('Tidak ada paket yang dititipkan di Pos Security.'),
          findsOneWidget,
        );
      },
    );
  });

  group('Humas — Menu KEAMANAN & PATROLI, tanpa PAKET', () {
    testWidgets(
      '1-6. Humas mendapat Main Menu PATROLI (Template Gang & Riwayat), KEAMANAN tanpa PATROLI & PAKET, dan tidak memiliki akses menu PAKET',
      (tester) async {
        tester.view.physicalSize = const Size(800, 1200);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(tester.view.resetPhysicalSize);
        addTearDown(tester.view.resetDevicePixelRatio);

        final api = _MockSecurityApi();
        final router = buildAppRouter(humasUser);

        await tester.pumpWidget(
          wrapWithRouter(
            user: humasUser,
            router: router,
            api: api,
            alerts: const SecurityAlertsData(patrolUnread: 2),
          ),
        );
        await tester.pumpAndSettle();

        // Open Menu grid -> KEAMANAN & PATROLI exist, PAKET MUST NOT exist
        await tester.tap(find.byKey(const Key('nav-item-menu')));
        await tester.pumpAndSettle();
        expect(find.byKey(const Key('nav-item-keamanan')), findsOneWidget);
        expect(find.byKey(const Key('nav-item-patroli')), findsOneWidget);
        expect(find.byKey(const Key('patrol-unread-dot')), findsOneWidget);
        expect(find.byKey(const Key('nav-item-paket')), findsNothing);
        expect(find.text('Paket'), findsNothing);

        // Tap Main Menu PATROLI -> same Kelola Patroli screen as Ketua RT
        await tester.tap(find.byKey(const Key('nav-item-patroli')));
        await tester.pumpAndSettle();
        expect(find.text('Template Gang'), findsOneWidget);
        expect(find.text('Riwayat'), findsOneWidget);
        expect(find.text('Daftar Gang & Titik Kontrol (2)'), findsOneWidget);
        await tester.tap(find.text('Riwayat'));
        await tester.pumpAndSettle();
        expect(find.text('Pak Sutrisno'), findsOneWidget);

        // Open KEAMANAN -> neither Patroli nor Paket tabs exist
        await tester.tap(find.byKey(const Key('nav-item-menu')));
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(const Key('nav-item-keamanan')));
        await tester.pumpAndSettle();
        final keamananTopTabBar = find.byType(TabBar).first;
        expect(
          find.descendant(of: keamananTopTabBar, matching: find.text('Kelola')),
          findsOneWidget,
        );
        expect(
          find.descendant(of: keamananTopTabBar, matching: find.text('Absensi')),
          findsOneWidget,
        );
        expect(
          find.descendant(of: keamananTopTabBar, matching: find.text('Perangkat')),
          findsOneWidget,
        );
        expect(
          find.descendant(of: keamananTopTabBar, matching: find.text('Patroli')),
          findsNothing,
        );
        expect(
          find.descendant(of: keamananTopTabBar, matching: find.text('Paket')),
          findsNothing,
        );

        // Direct route attempt to /home/paket redirects Humas safely back to /home
        router.go('/home/paket');
        await tester.pumpAndSettle();
        expect(find.text('Home Screen Body'), findsOneWidget);
        expect(find.text('Budi Santoso (B3/12)'), findsNothing);
      },
    );
  });

  group('Regression — Petugas Keamanan & Role Lain Tidak Terdampak', () {
    testWidgets(
      'Petugas Keamanan tetap memiliki tab Penerimaan dan Daftar Paket serta Ambil Foto dan Riwayat',
      (tester) async {
        tester.view.physicalSize = const Size(800, 1200);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(tester.view.resetPhysicalSize);
        addTearDown(tester.view.resetDevicePixelRatio);

        final api = _MockSecurityApi();
        final router = buildAppRouter(
          securityOfficerUser,
          initial: '/home/paket',
        );

        await tester.pumpWidget(
          wrapWithRouter(user: securityOfficerUser, router: router, api: api),
        );
        await tester.pumpAndSettle();

        // Petugas Keamanan still sees Penerimaan & Daftar Paket
        expect(find.text('Penerimaan'), findsOneWidget);
        expect(find.text('Daftar Paket'), findsOneWidget);

        // Petugas Keamanan on /home/patroli still sees Ambil Foto & Riwayat
        router.go('/home/patroli');
        await tester.pumpAndSettle();
        expect(find.text('Ambil Foto'), findsOneWidget);
        expect(find.text('Riwayat'), findsOneWidget);
      },
    );

    testWidgets(
      'Super Admin pada KEAMANAN tetap memiliki seluruh 5 tab tanpa kehilangan akses',
      (tester) async {
        tester.view.physicalSize = const Size(800, 1200);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(tester.view.resetPhysicalSize);
        addTearDown(tester.view.resetDevicePixelRatio);

        final api = _MockSecurityApi();
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              authRepositoryProvider.overrideWith(
                (_) => _FakeAuthRepo(superAdminUser),
              ),
              securityApiClientProvider.overrideWithValue(api),
              securityAlertsDataProvider.overrideWith(
                (_) async => const SecurityAlertsData(),
              ),
            ],
            child: const MaterialApp(
              home: SecurityManagementScreen(
                mode: SecurityManagementMode.full,
              ),
            ),
          ),
        );
        await tester.pumpAndSettle();

        final topTabBar = find.byType(TabBar).first;
        expect(
          find.descendant(of: topTabBar, matching: find.text('Kelola')),
          findsOneWidget,
        );
        expect(
          find.descendant(of: topTabBar, matching: find.text('Absensi')),
          findsOneWidget,
        );
        expect(
          find.descendant(of: topTabBar, matching: find.text('Perangkat')),
          findsOneWidget,
        );
        expect(
          find.descendant(of: topTabBar, matching: find.text('Patroli')),
          findsOneWidget,
        );
        expect(
          find.descendant(of: topTabBar, matching: find.text('Paket')),
          findsOneWidget,
        );
      },
    );
  });
}
