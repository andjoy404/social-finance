import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/security/data/security_api_client.dart';
import 'package:social_finance/features/security/data/security_models.dart';
import 'package:social_finance/features/security/presentation/providers/security_providers.dart';
import 'package:social_finance/features/security/presentation/screens/security_management_screen.dart';

class _FakeAuthRepository
    extends StateNotifier<AsyncValue<Map<String, dynamic>?>?>
    implements AuthRepository {
  _FakeAuthRepository(super.state);

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _MockSecurityApiClient extends SecurityApiClient {
  _MockSecurityApiClient({List<ShiftConfig>? initialShifts})
      : shifts = initialShifts ??
            [
              const ShiftConfig(
                id: 'shift-1',
                rtId: 'rt-002',
                name: 'Shift Pagi',
                startTime: '08:00:00',
                endTime: '16:00:00',
                crossesMidnight: false,
                latenessToleranceMinutes: 15,
                isActive: true,
              ),
            ],
        super(ApiClient(baseUrl: 'https://example.test'));

  final List<ShiftConfig> shifts;

  Map<String, dynamic>? lastCreatedPayload;
  String? lastUpdatedId;
  Map<String, dynamic>? lastUpdatedPayload;
  bool shouldThrowOnSave = false;

  @override
  Future<List<SecurityOfficer>> listManagerOfficers() async => [];

  @override
  Future<List<ShiftConfig>> listManagerShifts() async => List.of(shifts);

  @override
  Future<List<SecuritySchedule>> listManagerSchedules({
    String? dateFrom,
    String? dateTo,
    String? userId,
    int page = 1,
  }) async => [];

  @override
  Future<List<AttendanceRecord>> listManagerAttendance({
    String? dateFrom,
    String? dateTo,
    String? userId,
    String? status,
    int page = 1,
  }) async => [];

  @override
  Future<List<AttendanceCorrection>> listManagerCorrections() async => [];

  @override
  Future<List<SecurityDevice>> listManagerDevices() async => [];

  @override
  Future<GuestBookPage> listManagerGuestBooks({
    String? status,
    String? search,
    int page = 1,
  }) async => const GuestBookPage([], 0, 0);

  @override
  Future<ShiftConfig> createManagerShift(Map<String, dynamic> payload) async {
    if (shouldThrowOnSave) {
      throw Exception('Gagal menghubungi server');
    }
    lastCreatedPayload = payload;
    final created = ShiftConfig(
      id: 'shift-${shifts.length + 1}',
      rtId: 'rt-002',
      name: payload['name']?.toString() ?? '',
      startTime: payload['start_time']?.toString() ?? '08:00',
      endTime: payload['end_time']?.toString() ?? '16:00',
      crossesMidnight: payload['crosses_midnight'] as bool? ?? false,
      latenessToleranceMinutes:
          payload['lateness_tolerance_minutes'] as int? ?? 15,
      isActive: payload['is_active'] as bool? ?? true,
    );
    shifts.add(created);
    return created;
  }

  @override
  Future<void> updateManagerShift(
    String id,
    Map<String, dynamic> payload,
  ) async {
    if (shouldThrowOnSave) {
      throw Exception('Gagal memperbarui shift di server');
    }
    lastUpdatedId = id;
    lastUpdatedPayload = payload;
    final idx = shifts.indexWhere((s) => s.id == id);
    if (idx != -1) {
      shifts[idx] = ShiftConfig(
        id: id,
        rtId: shifts[idx].rtId,
        name: payload['name']?.toString() ?? shifts[idx].name,
        startTime: payload['start_time']?.toString() ?? shifts[idx].startTime,
        endTime: payload['end_time']?.toString() ?? shifts[idx].endTime,
        crossesMidnight:
            payload['crosses_midnight'] as bool? ?? shifts[idx].crossesMidnight,
        latenessToleranceMinutes:
            payload['lateness_tolerance_minutes'] as int? ??
                shifts[idx].latenessToleranceMinutes,
        isActive: payload['is_active'] as bool? ?? shifts[idx].isActive,
      );
    }
  }
}

void main() {
  setUp(() {
    SharedPreferences.setMockInitialValues({});
  });

  final managerUser = {
    'id': 'user-ketua-1',
    'email': 'ketua@local.test',
    'name': 'Ketua RT',
    'role': AppRole.warga,
    'jabatan': 'ketua',
    'effective_permissions': [
      'security.schedule.write',
      'security.schedule.read',
      'security.device.manage',
      'security.correction',
      'security.guest_book',
      'security.attendance',
    ],
    'rt': '002',
    'rt_id': 'rt-002',
  };

  testWidgets('A. Tambah Shift: membuka dialog, validasi, simpan, dan perbarui list',
      (tester) async {
    final mockApi = _MockSecurityApiClient();

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authRepositoryProvider.overrideWith(
            (_) => _FakeAuthRepository(AsyncData(managerUser)),
          ),
          securityApiClientProvider.overrideWithValue(mockApi),
        ],
        child: const MaterialApp(
          home: SecurityManagementScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    // Switch to Shift tab
    await tester.tap(find.text('Shift'));
    await tester.pumpAndSettle();

    // Tap Tambah Shift FAB
    final addFab = find.byTooltip('Tambah Shift');
    expect(addFab, findsOneWidget);
    await tester.tap(addFab);
    await tester.pumpAndSettle();

    // Dialog should be open
    expect(find.text('Tambah Shift'), findsOneWidget);
    expect(find.text('Nama shift *'), findsOneWidget);

    // 1. Test validation on empty name
    await tester.tap(find.text('Simpan'));
    await tester.pumpAndSettle();
    expect(find.text('Nama shift wajib diisi'), findsOneWidget);

    // 2. Fill valid shift name
    await tester.enterText(find.byType(TextFormField).first, 'Shift Malam');
    await tester.pumpAndSettle();

    // 3. Tap Simpan to create shift
    await tester.tap(find.text('Simpan'));
    await tester.pumpAndSettle();

    // Dialog should close
    expect(find.text('Tambah Shift'), findsNothing);

    // Feedback SnackBar should appear
    expect(find.text('Shift "Shift Malam" berhasil ditambahkan'), findsOneWidget);

    // Verify mock received correct payload
    expect(mockApi.lastCreatedPayload, isNotNull);
    expect(mockApi.lastCreatedPayload!['name'], 'Shift Malam');
    expect(mockApi.lastCreatedPayload!['start_time'], '08:00');
    expect(mockApi.lastCreatedPayload!['end_time'], '16:00');

    // Verify new shift appears in list
    expect(find.text('Shift Malam'), findsOneWidget);
  });

  testWidgets('B. Ubah Shift: membuka dialog dengan data terisi, simpan, dan perbarui list',
      (tester) async {
    final mockApi = _MockSecurityApiClient();

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authRepositoryProvider.overrideWith(
            (_) => _FakeAuthRepository(AsyncData(managerUser)),
          ),
          securityApiClientProvider.overrideWithValue(mockApi),
        ],
        child: const MaterialApp(
          home: SecurityManagementScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    // Switch to Shift tab
    await tester.tap(find.text('Shift'));
    await tester.pumpAndSettle();

    // Verify Shift Pagi is in the list
    expect(find.text('Shift Pagi'), findsOneWidget);

    // Tap Ubah Shift button
    final editBtn = find.byTooltip('Ubah Shift');
    expect(editBtn, findsOneWidget);
    await tester.tap(editBtn);
    await tester.pumpAndSettle();

    // Edit dialog should be open with prefilled data
    expect(find.text('Ubah Shift'), findsOneWidget);
    expect(find.widgetWithText(TextFormField, 'Shift Pagi'), findsOneWidget);

    // Change shift name
    await tester.enterText(find.widgetWithText(TextFormField, 'Shift Pagi'), 'Shift Pagi Revisi');
    await tester.pumpAndSettle();

    // Tap Simpan
    await tester.tap(find.text('Simpan'));
    await tester.pumpAndSettle();

    // Dialog should close
    expect(find.text('Ubah Shift'), findsNothing);

    // Feedback SnackBar should appear
    expect(find.text('Shift "Shift Pagi Revisi" berhasil diperbarui'), findsOneWidget);

    // Verify mock received update
    expect(mockApi.lastUpdatedId, 'shift-1');
    expect(mockApi.lastUpdatedPayload!['name'], 'Shift Pagi Revisi');

    // Verify updated shift in list
    expect(find.text('Shift Pagi Revisi'), findsOneWidget);
  });

  testWidgets('C. Batal: tombol Batal menutup form tanpa menyimpan', (tester) async {
    final mockApi = _MockSecurityApiClient();

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authRepositoryProvider.overrideWith(
            (_) => _FakeAuthRepository(AsyncData(managerUser)),
          ),
          securityApiClientProvider.overrideWithValue(mockApi),
        ],
        child: const MaterialApp(
          home: SecurityManagementScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('Shift'));
    await tester.pumpAndSettle();

    await tester.tap(find.byTooltip('Tambah Shift'));
    await tester.pumpAndSettle();

    expect(find.text('Tambah Shift'), findsOneWidget);

    // Tap Batal
    await tester.tap(find.text('Batal'));
    await tester.pumpAndSettle();

    expect(find.text('Tambah Shift'), findsNothing);
    expect(mockApi.lastCreatedPayload, isNull);
  });

  testWidgets('D. Error handling: kegagalan API mempertahankan dialog dan menampilkan pesan error',
      (tester) async {
    final mockApi = _MockSecurityApiClient();
    mockApi.shouldThrowOnSave = true;

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authRepositoryProvider.overrideWith(
            (_) => _FakeAuthRepository(AsyncData(managerUser)),
          ),
          securityApiClientProvider.overrideWithValue(mockApi),
        ],
        child: const MaterialApp(
          home: SecurityManagementScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('Shift'));
    await tester.pumpAndSettle();

    await tester.tap(find.byTooltip('Tambah Shift'));
    await tester.pumpAndSettle();

    await tester.enterText(find.byType(TextFormField).first, 'Shift Error Test');
    await tester.pumpAndSettle();

    await tester.tap(find.text('Simpan'));
    await tester.pumpAndSettle();

    // Dialog stays open with error banner visible
    expect(find.text('Tambah Shift'), findsOneWidget);
    expect(find.text('Gagal menghubungi server'), findsOneWidget);
    // Data remains in text field
    expect(find.widgetWithText(TextFormField, 'Shift Error Test'), findsOneWidget);
  });

  testWidgets('E. Empty state: menampilkan tombol Tambah Shift saat daftar kosong',
      (tester) async {
    final mockApi = _MockSecurityApiClient(initialShifts: []);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authRepositoryProvider.overrideWith(
            (_) => _FakeAuthRepository(AsyncData(managerUser)),
          ),
          securityApiClientProvider.overrideWithValue(mockApi),
        ],
        child: const MaterialApp(
          home: SecurityManagementScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('Shift'));
    await tester.pumpAndSettle();

    expect(find.text('Belum ada shift.'), findsOneWidget);
    // Empty state should have Tambah Shift button
    final emptyStateBtn = find.widgetWithText(FilledButton, 'Tambah Shift');
    expect(emptyStateBtn, findsOneWidget);

    await tester.tap(emptyStateBtn);
    await tester.pumpAndSettle();

    expect(find.text('Tambah Shift'), findsWidgets);
  });

  testWidgets('F. Validasi: jam mulai dan jam selesai tidak boleh sama', (tester) async {
    final mockApi = _MockSecurityApiClient(
      initialShifts: [
        const ShiftConfig(
          id: 'shift-kembar',
          rtId: 'rt-002',
          name: 'Shift Kembar',
          startTime: '08:00',
          endTime: '08:00',
          crossesMidnight: false,
          latenessToleranceMinutes: 15,
          isActive: true,
        ),
      ],
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authRepositoryProvider.overrideWith(
            (_) => _FakeAuthRepository(AsyncData(managerUser)),
          ),
          securityApiClientProvider.overrideWithValue(mockApi),
        ],
        child: const MaterialApp(
          home: SecurityManagementScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('Shift'));
    await tester.pumpAndSettle();

    // Tap Ubah Shift button on Shift Kembar
    final editBtn = find.byTooltip('Ubah Shift');
    expect(editBtn, findsOneWidget);
    await tester.tap(editBtn);
    await tester.pumpAndSettle();

    // Edit dialog should be open
    expect(find.text('Ubah Shift'), findsOneWidget);

    // Tap Simpan with identical start and end time (08:00 == 08:00)
    await tester.tap(find.text('Simpan'));
    await tester.pumpAndSettle();

    // Should display validation error
    expect(find.text('Jam mulai dan jam selesai tidak boleh sama'), findsOneWidget);
    // Dialog should stay open
    expect(find.text('Ubah Shift'), findsOneWidget);
    expect(mockApi.lastUpdatedPayload, isNull);
  });

  testWidgets('G. Tap ListTile pada shift card membuka Ubah Shift', (tester) async {
    final mockApi = _MockSecurityApiClient();

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authRepositoryProvider.overrideWith(
            (_) => _FakeAuthRepository(AsyncData(managerUser)),
          ),
          securityApiClientProvider.overrideWithValue(mockApi),
        ],
        child: const MaterialApp(
          home: SecurityManagementScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('Shift'));
    await tester.pumpAndSettle();

    // Tap the card / ListTile text directly
    await tester.tap(find.text('Shift Pagi'));
    await tester.pumpAndSettle();

    // Ubah Shift dialog should open
    expect(find.text('Ubah Shift'), findsOneWidget);
    expect(find.widgetWithText(TextFormField, 'Shift Pagi'), findsOneWidget);
  });
}
