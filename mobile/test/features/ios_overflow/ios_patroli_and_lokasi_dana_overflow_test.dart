import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart' as rp;
import 'package:flutter_test/flutter_test.dart';

import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/fund_location/data/fund_location_repository.dart';
import 'package:social_finance/features/fund_location/data/models/fund_location_models.dart';
import 'package:social_finance/features/fund_location/presentation/screens/fund_location_screen.dart';
import 'package:social_finance/features/kas/data/kas_api_models.dart';
import 'package:social_finance/features/security/data/security_api_client.dart';
import 'package:social_finance/features/security/data/security_models.dart';
import 'package:social_finance/features/security/presentation/providers/security_providers.dart';
import 'package:social_finance/features/security/presentation/screens/security_management_screen.dart';

class _FakeAuthRepo extends rp.StateNotifier<rp.AsyncValue<Map<String, dynamic>?>?>
    implements AuthRepository {
  _FakeAuthRepo(super.state);

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _PatrolOverflowMockApi extends SecurityApiClient {
  _PatrolOverflowMockApi({required this.templates})
    : super(ApiClient(baseUrl: 'https://example.test'));

  final List<PatrolTemplate> templates;

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
    String? status,
  }) async => const GuestBookPage([], 0, 1);
  @override
  Future<List<PatrolTemplate>> listPatrolTemplates() async => templates;
  @override
  Future<List<PatrolBundle>> listPatrolBundles() async => const [];
  @override
  Future<List<PaketReport>> listPaketReports({String? status}) async =>
      const [];
}

final List<FundLocation> _sampleLocations = [
  FundLocation(
    id: 'loc-1',
    rtId: 'rt-002',
    name: 'Rekening RT Mandiri Operasional Utama',
    category: FundLocationCategory.rekening,
    bankName: 'MANDIRI',
    accountNumber: '1640002607929',
    accountHolder: 'Bendahara RT 002 RW 016',
    isActive: true,
    currentBalance: '6900000.00',
    createdAt: DateTime.parse('2026-10-25T10:00:00Z'),
    updatedAt: DateTime.parse('2026-10-25T10:00:00Z'),
  ),
  FundLocation(
    id: 'loc-2',
    rtId: 'rt-002',
    name: 'Kas Di Tangan Bendahara RT',
    category: FundLocationCategory.coh,
    isActive: true,
    currentBalance: '1250000.00',
    createdAt: DateTime.parse('2026-10-25T10:00:00Z'),
    updatedAt: DateTime.parse('2026-10-25T10:00:00Z'),
  ),
];

final PaginatedResponse<FundTransfer> _sampleTransfers =
    PaginatedResponse<FundTransfer>(
      data: [
        FundTransfer(
          id: 'tr-1',
          rtId: 'rt-002',
          fromLocationId: 'loc-1',
          toLocationId: 'loc-2',
          fromLocationName: 'Rekening RT Mandiri Operasional',
          toLocationName: 'Kas Di Tangan Bendahara RT',
          amount: '1250000.00',
          transferDate: DateTime(2026, 11, 25),
          notes: 'Penarikan dana operasional kegiatan kerja bakti lingkungan',
          createdAt: DateTime(2026, 11, 25, 10, 0),
        ),
      ],
      pagination: const PaginationInfo(
        page: 1,
        pageSize: 50,
        total: 1,
        totalPages: 1,
      ),
    );

final Map<String, dynamic> _bendaharaUser = {
  'id': 'user-bendahara',
  'email': 'bendahara@local.com',
  'name': 'Bendahara RT',
  'role': AppRole.bendahara,
  'rt': '002',
  'rt_id': '43d1ed42-d982-4299-9b29-30080591fcf9',
};

final Map<String, dynamic> _pengurusPatrolUser = {
  'id': 'usr-pengurus-1',
  'name': 'Ketua RT',
  'role': 'pengurus',
  'jabatan': 'ketua',
  'effective_permissions': [
    'security.schedule.read',
    'security.schedule.write',
    'security.device.manage',
    'security.correction',
    'patrol.template.manage',
    'patrol.read',
    'paket.read',
  ],
};

FundLocationReconciliationReport _makeReconReport(String status) {
  return FundLocationReconciliationReport(
    rtId: '43d1ed42-d982-4299-9b29-30080591fcf9',
    kasBalance: '16900000.00',
    totalAllocatedBalance: status == 'MATCH' ? '16900000.00' : '14250000.00',
    unallocatedBalance: status == 'MATCH' ? '0.00' : '2650000.00',
    discrepancy: status == 'DISCREPANCY_ANOMALY' ? '150000.00' : '0.00',
    status: status,
    asOf: DateTime(2026, 11, 28, 2, 0, 0),
    anomaliesDetected: status == 'DISCREPANCY_ANOMALY',
  );
}

void main() {
  group('iOS Overflow Regression — KEAMANAN -> PATROLI', () {
    for (final cfg in const [
      (width: 402.0, height: 874.0, scale: 1.0, count: 5, label: 'iPhone 18 Pro 402px @1.0x'),
      (width: 402.0, height: 874.0, scale: 1.25, count: 128, label: 'iPhone 18 Pro 402px @1.25x long count'),
      (width: 360.0, height: 800.0, scale: 1.0, count: 24, label: 'Narrow iOS 360px @1.0x'),
    ]) {
      testWidgets(
        'Template Gang header and cards render with ZERO RenderFlex overflow on iOS (${cfg.label})',
        (tester) async {
          debugDefaultTargetPlatformOverride = TargetPlatform.iOS;
          final originalOnError = FlutterError.onError;
          final capturedErrors = <FlutterErrorDetails>[];
          FlutterError.onError = (details) {
            capturedErrors.add(details);
          };
          addTearDown(() {
            FlutterError.onError = originalOnError;
            tester.binding.setSurfaceSize(null);
          });

          try {
            await tester.binding.setSurfaceSize(Size(cfg.width, cfg.height));
            final templates = List.generate(
              cfg.count,
              (i) => PatrolTemplate(
                id: 'tpl-${i + 1}',
                name: 'Pos Kontrol Gang Blok U ${i + 1}',
                locationLabel:
                    'Lorong U ${i + 1} Dekat Pos Keamanan Utara RT 002 RW 016',
                isActive: true,
                createdAt: DateTime(2026, 11, 1),
              ),
            );
            final mockApi = _PatrolOverflowMockApi(templates: templates);

            await tester.pumpWidget(
              rp.ProviderScope(
                overrides: [
                  authRepositoryProvider.overrideWith(
                    (_) => _FakeAuthRepo(rp.AsyncData(_pengurusPatrolUser)),
                  ),
                  securityApiClientProvider.overrideWithValue(mockApi),
                ],
                child: MaterialApp(
                  theme: ThemeData(platform: TargetPlatform.iOS),
                  builder: (context, child) => MediaQuery(
                    data: MediaQuery.of(context).copyWith(
                      textScaler: TextScaler.linear(cfg.scale),
                    ),
                    child: child!,
                  ),
                  home: const SecurityManagementScreen(initialTopTabIndex: 3),
                ),
              ),
            );
            await tester.pumpAndSettle();

            final overflowErrors = capturedErrors.where(
              (e) => e.exceptionAsString().contains('RenderFlex overflowed'),
            );
            expect(
              overflowErrors,
              isEmpty,
              reason: overflowErrors.map((e) => e.exceptionAsString()).join('\n'),
            );
            expect(
              find.text('Daftar Gang & Titik Kontrol (${cfg.count})'),
              findsOneWidget,
            );
            expect(find.text('Tambah Gang'), findsOneWidget);

            // Verify primary button interaction remains intact
            await tester.tap(find.text('Tambah Gang'));
            await tester.pumpAndSettle();
            expect(find.text('Tambah Titik Kontrol Gang'), findsOneWidget);
          } finally {
            debugDefaultTargetPlatformOverride = null;
          }
        },
      );
    }
  });

  group('iOS Overflow Regression — LOKASI DANA', () {
    for (final statusEntry in const [
      ('MATCH', 'Cocok Sempurna'),
      ('MATCH_WITH_NOTES', 'Sesuai Catatan'),
      ('DISCREPANCY_ANOMALY', 'Selisih / Anomali'),
    ]) {
      testWidgets(
        'Reconciliation header, expanded metrics, and transfers tab render with ZERO RenderFlex overflow on iPhone 18 Pro iOS (${statusEntry.$1})',
        (tester) async {
          debugDefaultTargetPlatformOverride = TargetPlatform.iOS;
          final originalOnError = FlutterError.onError;
          final capturedErrors = <FlutterErrorDetails>[];
          FlutterError.onError = (details) {
            capturedErrors.add(details);
          };
          addTearDown(() {
            FlutterError.onError = originalOnError;
            tester.binding.setSurfaceSize(null);
          });

          try {
            await tester.binding.setSurfaceSize(const Size(402, 874));
            await tester.pumpWidget(
              rp.ProviderScope(
                overrides: [
                  authRepositoryProvider.overrideWith((_) {
                    final client = ApiClient(customDio: Dio());
                    final repo = AuthRepository(
                      authService: AuthService(client),
                      apiClient: client,
                    );
                    repo.state = rp.AsyncData<Map<String, dynamic>?>(
                      _bendaharaUser,
                    );
                    return repo;
                  }),
                  fundLocationsProvider.overrideWith(
                    (_) async => _sampleLocations,
                  ),
                  fundTransfersProvider.overrideWith(
                    (_) async => _sampleTransfers,
                  ),
                  fundReconciliationProvider.overrideWith(
                    (_) async => _makeReconReport(statusEntry.$1),
                  ),
                ],
                child: MaterialApp(
                  theme: ThemeData(platform: TargetPlatform.iOS),
                  home: const FundLocationScreen(),
                ),
              ),
            );
            await tester.pumpAndSettle();

            // 1. Collapsed header: zero overflow & all elements visible
            expect(
              capturedErrors.where(
                (e) => e.exceptionAsString().contains('RenderFlex overflowed'),
              ),
              isEmpty,
            );
            expect(find.text('Status Rekonsiliasi Kas'), findsOneWidget);
            expect(find.text(statusEntry.$2), findsOneWidget);
            expect(find.byIcon(Icons.expand_more), findsOneWidget);
            expect(find.text('Transfer Antar Lokasi'), findsOneWidget);

            // 2. Tap header to expand 4-metric reconciliation section: zero overflow & toggle works
            await tester.tap(find.text('Status Rekonsiliasi Kas'));
            await tester.pumpAndSettle();
            expect(
              capturedErrors.where(
                (e) => e.exceptionAsString().contains('RenderFlex overflowed'),
              ),
              isEmpty,
            );
            expect(find.byIcon(Icons.expand_less), findsOneWidget);
            expect(find.text('Total Kas (K)'), findsOneWidget);
            expect(find.text('Selisih (D)'), findsOneWidget);

            // 3. Switch to Riwayat Transfer tab with long location names: zero overflow
            await tester.tap(find.text('Riwayat Transfer'));
            await tester.pumpAndSettle();
            expect(
              capturedErrors.where(
                (e) => e.exceptionAsString().contains('RenderFlex overflowed'),
              ),
              isEmpty,
            );
            expect(
              find.text('Rekening RT Mandiri Operasional'),
              findsOneWidget,
            );
            expect(find.text('Kas Di Tangan Bendahara RT'), findsOneWidget);
          } finally {
            debugDefaultTargetPlatformOverride = null;
          }
        },
      );
    }

    testWidgets(
      'Lokasi Dana renders with ZERO RenderFlex overflow at narrow 360px and 1.25x iOS text scaling',
      (tester) async {
        debugDefaultTargetPlatformOverride = TargetPlatform.iOS;
        final originalOnError = FlutterError.onError;
        final capturedErrors = <FlutterErrorDetails>[];
        FlutterError.onError = (details) {
          capturedErrors.add(details);
        };
        addTearDown(() {
          FlutterError.onError = originalOnError;
          tester.binding.setSurfaceSize(null);
        });

        try {
          await tester.binding.setSurfaceSize(const Size(360, 800));
          await tester.pumpWidget(
            rp.ProviderScope(
              overrides: [
                authRepositoryProvider.overrideWith((_) {
                  final client = ApiClient(customDio: Dio());
                  final repo = AuthRepository(
                    authService: AuthService(client),
                    apiClient: client,
                  );
                  repo.state = rp.AsyncData<Map<String, dynamic>?>(
                    _bendaharaUser,
                  );
                  return repo;
                }),
                fundLocationsProvider.overrideWith(
                  (_) async => _sampleLocations,
                ),
                fundTransfersProvider.overrideWith(
                  (_) async => _sampleTransfers,
                ),
                fundReconciliationProvider.overrideWith(
                  (_) async => _makeReconReport('DISCREPANCY_ANOMALY'),
                ),
              ],
              child: MaterialApp(
                theme: ThemeData(platform: TargetPlatform.iOS),
                builder: (context, child) => MediaQuery(
                  data: MediaQuery.of(context).copyWith(
                    textScaler: const TextScaler.linear(1.25),
                  ),
                  child: child!,
                ),
                home: const FundLocationScreen(),
              ),
            ),
          );
          await tester.pumpAndSettle();

          expect(
            capturedErrors.where(
              (e) => e.exceptionAsString().contains('RenderFlex overflowed'),
            ),
            isEmpty,
          );
          expect(find.text('Status Rekonsiliasi Kas'), findsOneWidget);
          expect(find.text('Selisih / Anomali'), findsOneWidget);
        } finally {
          debugDefaultTargetPlatformOverride = null;
        }
      },
    );
  });
}
