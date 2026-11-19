import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/app/app.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
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

class _PatrolPaketMockApi extends SecurityApiClient {
  _PatrolPaketMockApi() : super(ApiClient(baseUrl: 'https://example.test'));

  final List<PatrolTemplate> templates = [
    PatrolTemplate(
      id: 'tpl-1',
      name: 'Gang Mawar',
      locationLabel: 'Titik Kontrol 1',
      isActive: true,
      createdAt: DateTime(2026, 11, 1),
    ),
    PatrolTemplate(
      id: 'tpl-2',
      name: 'Gang Melati',
      locationLabel: 'Titik Kontrol 2',
      isActive: true,
      createdAt: DateTime(2026, 11, 1),
    ),
  ];

  final List<PatrolBundle> bundles = [
    PatrolBundle(
      id: 'bnd-1',
      createdBy: 'usr-sec-1',
      dutyOfficerName: 'Sutrisno',
      submittedAt: DateTime(2026, 11, 15, 23, 30),
      notes: 'Lampu gang menyala dan portal terkunci aman',
      photoCount: 2,
      photos: [
        PatrolPhoto(
          id: 'ph-1',
          patrolBundleId: 'bnd-1',
          patrolTemplateId: 'tpl-1',
          templateName: 'Gang Mawar',
          locationLabel: 'Titik Kontrol 1',
          imageKey: 'key-1',
          imageUrl: 'https://example.test/img1.jpg',
          capturedAt: DateTime(2026, 11, 15, 23, 25),
          createdAt: DateTime(2026, 11, 15, 23, 30),
        ),
      ],
      createdAt: DateTime(2026, 11, 15, 23, 30),
    ),
  ];

  final List<PaketReport> reports = [
    PaketReport(
      id: 'rep-1',
      receiverWargaId: 'wrg-1',
      receiverName: 'Budi Santoso',
      receiverHouseNumber: 'B3/12',
      createdBy: 'usr-sec-1',
      createdByName: 'Sutrisno',
      overallStatus: 'dititipkan',
      items: [
        PaketItem(
          id: 'item-1',
          paketReportId: 'rep-1',
          description: 'Kurir: JNE · Resi: JNE12345 · Kardus Sedang',
          status: 'dititipkan',
          createdAt: DateTime.now().subtract(const Duration(hours: 30)), // > 24 hours overdue
        ),
      ],
      createdAt: DateTime.now().subtract(const Duration(hours: 30)),
    ),
  ];

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
  Future<List<PatrolTemplate>> listPatrolTemplates() async => templates;
  @override
  Future<List<PatrolBundle>> listPatrolBundles() async => bundles;
  @override
  Future<List<PaketReport>> listPaketReports({String? status}) async => reports;
  @override
  Future<PaketReport> getPaketReport(String reportId) async {
    return reports.firstWhere(
      (r) => r.id == reportId,
      orElse: () => throw Exception('Laporan paket tidak ditemukan'),
    );
  }
}

void main() {
  final securityUser = {
    'id': 'usr-sec-1',
    'name': 'Sutrisno',
    'role': 'pengurus',
    'jabatan': 'keamanan',
    'effective_permissions': [
      'security.schedule.read',
      'security.schedule.write',
      'security.device.manage',
      'security.correction',
      'patrol.create',
      'patrol.read',
      'paket.create',
      'paket.read',
      'paket.status_change',
    ],
  };

  testWidgets('renders Patroli tab, displays active templates and history bundles', (tester) async {
    final mockApi = _PatrolPaketMockApi();

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authRepositoryProvider.overrideWith(
            (ref) => _FakeAuthRepo(AsyncData(securityUser)),
          ),
          securityApiClientProvider.overrideWithValue(mockApi),
        ],
        child: const MaterialApp(home: SecurityManagementScreen()),
      ),
    );
    await tester.pumpAndSettle();

    // Tap on top tab "Patroli"
    expect(find.text('Patroli'), findsOneWidget);
    await tester.tap(find.text('Patroli'));
    await tester.pumpAndSettle();

    // Verify subtabs "Ambil Foto" and "Riwayat" exist
    expect(find.text('Ambil Foto'), findsOneWidget);
    expect(find.text('Riwayat'), findsOneWidget);

    // Form tab shows templates
    expect(find.text('Gang Mawar'), findsOneWidget);
    expect(find.text('Gang Melati'), findsOneWidget);
    expect(find.text('Ambil Foto Kontrol'), findsNWidgets(2));
    await tester.drag(find.byType(ListView).last, const Offset(0, -300));
    await tester.pumpAndSettle();
    expect(find.text('Kirim Laporan Patroli'), findsOneWidget);

    // Switch to Riwayat subtab
    await tester.tap(find.text('Riwayat'));
    await tester.pumpAndSettle();

    expect(find.text('Sutrisno'), findsOneWidget);
    expect(find.textContaining('2 Foto Titik Kontrol'), findsOneWidget);
    expect(find.textContaining('portal terkunci aman'), findsOneWidget);
  });

  testWidgets('renders Paket tab, displays overdue warning and multi-item UI', (tester) async {
    final mockApi = _PatrolPaketMockApi();

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authRepositoryProvider.overrideWith(
            (ref) => _FakeAuthRepo(AsyncData(securityUser)),
          ),
          securityApiClientProvider.overrideWithValue(mockApi),
        ],
        child: const MaterialApp(home: SecurityManagementScreen()),
      ),
    );
    await tester.pumpAndSettle();

    // Tap on top tab "Paket"
    expect(find.text('Paket'), findsOneWidget);
    await tester.tap(find.text('Paket'));
    await tester.pumpAndSettle();

    // Verify subtabs "Penerimaan" and "Daftar Paket"
    expect(find.text('Penerimaan'), findsOneWidget);
    expect(find.text('Daftar Paket'), findsOneWidget);

    // Penerimaan form: resident search & multi-item add button
    expect(find.text('Penerima Paket (Warga RT)'), findsOneWidget);
    await tester.drag(find.byType(ListView).last, const Offset(0, -300));
    await tester.pumpAndSettle();
    expect(find.text('Tambah Paket Lain (Multi-Item)'), findsOneWidget);
    expect(find.text('Simpan Penitipan Paket'), findsOneWidget);

    // Switch to Daftar Paket subtab
    await tester.tap(find.text('Daftar Paket'));
    await tester.pumpAndSettle();

    // Verify package report is listed with overdue badge
    expect(find.text('Budi Santoso (B3/12)'), findsOneWidget);
    expect(find.textContaining('Kurir: JNE'), findsOneWidget);
    expect(find.textContaining('> 24 Jam'), findsOneWidget);
    expect(find.text('Serah Terima'), findsOneWidget);
  });

  test('normalizeNotificationTarget generates deep links with stable entity IDs', () {
    // Patrol with bundle_id in payload
    final patrolLink = normalizeNotificationTarget(null, payload: {
      'type': 'patrol_bundle_submitted',
      'bundle_id': 'bnd-123',
    });
    expect(patrolLink, equals('/home/security-management?tab=patrol&subtab=history&bundle_id=bnd-123'));

    // Patrol with dedup_key fallback
    final patrolDedupLink = normalizeNotificationTarget(null, payload: {
      'type': 'patrol_bundle_submitted',
      'dedup_key': 'patrol:bnd-999:user-1',
    });
    expect(patrolDedupLink, equals('/home/security-management?tab=patrol&subtab=history&bundle_id=bnd-999'));

    // Paket with report_id in payload routes to full-page paket detail
    final paketLink = normalizeNotificationTarget(null, payload: {
      'type': 'paket_received',
      'report_id': 'rep-456',
    });
    expect(paketLink, equals('/home/paket/detail/rep-456'));

    // Paket with dedup_key fallback
    final paketDedupLink = normalizeNotificationTarget(null, payload: {
      'type': 'paket_received',
      'dedup_key': 'paket_ketua:rep-888:user-1',
    });
    expect(paketDedupLink, equals('/home/security-management?tab=paket&subtab=list&report_id=rep-888'));

    // Missing entity ID falls back safely to subtab without ID
    final patrolFallback = normalizeNotificationTarget(null, payload: {
      'type': 'patrol_bundle_submitted',
    });
    expect(patrolFallback, equals('/home/security-management?tab=patrol&subtab=history'));

    final paketFallback = normalizeNotificationTarget(null, payload: {
      'type': 'paket_received',
    });
    expect(paketFallback, equals('/home/security-management?tab=paket&subtab=list'));
  });

  testWidgets('deep link to patrol with valid targetId opens photo dialog', (tester) async {
    final mockApi = _PatrolPaketMockApi();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authRepositoryProvider.overrideWith(
            (ref) => _FakeAuthRepo(AsyncData(securityUser)),
          ),
          securityApiClientProvider.overrideWithValue(mockApi),
        ],
        child: const MaterialApp(
          home: SecurityManagementScreen(
            initialTopTabIndex: 3, // patrol
            initialSubTab: 'history',
            targetId: 'bnd-1',
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    // Dialog should open showing patrol photo details
    expect(find.text('Foto Patroli - Sutrisno'), findsOneWidget);
    expect(find.text('Tutup'), findsOneWidget);
    await tester.tap(find.text('Tutup'));
    await tester.pumpAndSettle();
  });

  testWidgets('deep link to patrol with non-existent targetId falls back to history list', (tester) async {
    final mockApi = _PatrolPaketMockApi();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authRepositoryProvider.overrideWith(
            (ref) => _FakeAuthRepo(AsyncData(securityUser)),
          ),
          securityApiClientProvider.overrideWithValue(mockApi),
        ],
        child: const MaterialApp(
          home: SecurityManagementScreen(
            initialTopTabIndex: 3, // patrol
            initialSubTab: 'history',
            targetId: 'non-existent-bundle',
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    // Photo dialog should NOT open, cleanly stays on history list
    expect(find.text('Foto Patroli: Sutrisno'), findsNothing);
    expect(find.text('Riwayat'), findsOneWidget);
    expect(find.textContaining('Sutrisno'), findsOneWidget);
  });

  testWidgets('deep link to paket with non-existent targetId falls back to list cleanly', (tester) async {
    final mockApi = _PatrolPaketMockApi();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authRepositoryProvider.overrideWith(
            (ref) => _FakeAuthRepo(AsyncData(securityUser)),
          ),
          securityApiClientProvider.overrideWithValue(mockApi),
        ],
        child: const MaterialApp(
          home: SecurityManagementScreen(
            initialTopTabIndex: 4, // paket
            initialSubTab: 'list',
            targetId: 'non-existent-report',
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    // Dialog should NOT open, cleanly stays on Daftar Paket list
    expect(find.text('Foto Paket'), findsNothing);
    expect(find.text('Daftar Paket'), findsOneWidget);
    expect(find.text('Budi Santoso (B3/12)'), findsOneWidget);
  });
}
