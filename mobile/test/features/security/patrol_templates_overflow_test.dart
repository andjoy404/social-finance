import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
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

class _PatrolTemplateMockApi extends SecurityApiClient {
  _PatrolTemplateMockApi() : super(ApiClient(baseUrl: 'https://example.test'));

  final List<PatrolTemplate> templates = List.generate(
    5,
    (i) => PatrolTemplate(
      id: 'tpl-${i + 1}',
      name: '0${i + 8}-0${i + 9}',
      locationLabel: 'Lorong U 0${i + 8} - 0${i + 9}',
      isActive: true,
      createdAt: DateTime(2026, 11, 1),
    ),
  );

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

void main() {
  final pengurusManageOnlyUser = {
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

  group('SecurityManagementScreen — Patrol Template Gang header overflow', () {
    for (final width in const [402.0, 360.0]) {
      testWidgets(
        'does not overflow at ${width.toInt()} px width on Template Gang tab',
        (tester) async {
          await tester.binding.setSurfaceSize(Size(width, 874));
          addTearDown(() => tester.binding.setSurfaceSize(null));

          final mockApi = _PatrolTemplateMockApi();

          await tester.pumpWidget(
            ProviderScope(
              overrides: [
                authRepositoryProvider.overrideWith(
                  (ref) => _FakeAuthRepo(AsyncData(pengurusManageOnlyUser)),
                ),
                securityApiClientProvider.overrideWithValue(mockApi),
              ],
              child: const MaterialApp(
                home: SecurityManagementScreen(
                  mode: SecurityManagementMode.managePatrol,
                ),
              ),
            ),
          );
          await tester.pumpAndSettle();

          expect(tester.takeException(), isNull);
          expect(find.text('Template Gang'), findsOneWidget);
          expect(find.text('Daftar Gang & Titik Kontrol (5)'), findsOneWidget);
          expect(find.text('Tambah Gang'), findsOneWidget);
        },
      );
    }
  });
}
