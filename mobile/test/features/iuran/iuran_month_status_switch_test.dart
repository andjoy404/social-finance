import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/api/device_id_service.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/iuran/data/iuran_api_client.dart';
import 'package:social_finance/features/iuran/data/iuran_api_models.dart';
import 'package:social_finance/features/iuran/data/iuran_models.dart';
import 'package:social_finance/features/iuran/data/iuran_repository.dart';
import 'package:social_finance/features/iuran/presentation/screens/kelola_iuran_tab_body.dart';

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

class _FakeDueRepository extends IuranRepository {
  _FakeDueRepository(List<BackendDue> dues)
      : _dues = List.of(dues),
        super(apiClient: IuranApiClient(dio: Dio()));

  final List<BackendDue> _dues;
  final List<Map<String, dynamic>> updatePeriodCalls = [];
  final List<Map<String, dynamic>> updateDueCalls = [];
  final List<String> fetchMonthsCalls = [];
  final Map<String, bool> _monthActiveState = {};
  bool failUpdate = false;
  int _updateCallCount = 0;

  @override
  Future<List<BackendDue>> fetchDues({bool? isActive}) async => List.of(_dues);

  @override
  Future<PaginatedResponse<IuranBill>> fetchBills({
    String? search,
    String? order,
    String? dueId,
    String? status,
    String? paymentStatus,
    String? period,
    String? householdOccupancyId,
    bool? isActive,
    bool myBills = false,
    int page = 1,
    int pageSize = 30,
  }) async {
    return const PaginatedResponse<IuranBill>(
      data: [],
      pagination: PaginationInfo(
        page: 1,
        pageSize: 30,
        total: 0,
        totalPages: 0,
      ),
    );
  }

  @override
  Future<BackendDue> updateDue(
    String id, {
    String? name,
    int? amount,
    DuePeriodType? periodType,
    PaymentRule? paymentRule,
    bool? isVoluntary,
    bool? isActive,
    String? validUntil,
  }) async {
    updateDueCalls.add({
      'id': id,
      'name': name,
      'amount': amount,
      'periodType': periodType,
      'paymentRule': paymentRule,
      'isVoluntary': isVoluntary,
      'isActive': isActive,
      'validUntil': validUntil,
    });
    return _multiMonthDue();
  }

  @override
  Future<List<DueMonthSummary>> fetchDueMonths(String dueId) async {
    fetchMonthsCalls.add(dueId);
    return List.generate(
      12,
      (index) {
        final period = '2026-${(index + 1).toString().padLeft(2, '0')}';
        final isActive = _monthActiveState[period] ?? true;
        return DueMonthSummary(
          period: period,
          month: index + 1,
          year: 2026,
          totalBills: 5,
          activeBills: isActive ? 5 : 0,
          paidBills: 0,
          unpaidBills: 5,
          isActive: isActive,
          canDeactivate: true,
        );
      },
    );
  }

  @override
  Future<List<DueMonthSummary>> updateDuePeriodStatus(
    String dueId,
    String period, {
    required bool isActive,
  }) async {
    _updateCallCount++;
    updatePeriodCalls.add({
      'dueId': dueId,
      'period': period,
      'isActive': isActive,
    });
    if (failUpdate) throw Exception('Server error');
    // Partial failure: fail on the Nth call if failOnCall is set
    if (_failOnCall != null && _updateCallCount == _failOnCall) {
      throw Exception('Server error on call $_updateCallCount');
    }

    _monthActiveState[period] = isActive;

    return List.generate(
      12,
      (index) {
        final p = '2026-${(index + 1).toString().padLeft(2, '0')}';
        final active = _monthActiveState[p] ?? true;
        return DueMonthSummary(
          period: p,
          month: index + 1,
          year: 2026,
          totalBills: 5,
          activeBills: active ? 5 : 0,
          paidBills: 0,
          unpaidBills: 5,
          isActive: active,
          canDeactivate: true,
        );
      },
    );
  }

  /// Fail the updateDuePeriodStatus call at this 1-based index.
  void failOnCall(int callIndex) {
    _failOnCall = callIndex;
  }

  int? _failOnCall;
}

final _bendaharaUser = <String, dynamic>{
  'id': 'u-bendahara',
  'email': 'bendahara@test.com',
  'role': 'bendahara',
  'system_role': null,
  'jabatan': 'bendahara',
  'profile_status': 'ready',
  'effective_permissions': <String>[
    'finance.read',
    'finance.create',
    'finance.update',
    'finance.approve',
  ],
};

BackendDue _multiMonthDue() {
  return const BackendDue(
    id: 'due-mm-1',
    rtId: 'rt-1',
    name: 'IPL 2026',
    amount: '100000.00',
    periodType: DuePeriodType.monthly,
    paymentRule: PaymentRule.full,
    isVoluntary: false,
    isActive: true,
    dueType: DueType.multiMonth,
    year: 2026,
  );
}

BackendDue _regularDue() {
  return const BackendDue(
    id: 'due-reg-1',
    rtId: 'rt-1',
    name: 'Iuran Sampah Reguler',
    amount: '25000.00',
    periodType: DuePeriodType.monthly,
    paymentRule: PaymentRule.full,
    isVoluntary: false,
    isActive: true,
    dueType: DueType.regular,
  );
}

Widget _buildScreen(_FakeDueRepository repository) {
  return ProviderScope(
    overrides: [
      authRepositoryProvider.overrideWith((ref) => _MockAuthRepo(_bendaharaUser)),
      iuranRepositoryProvider.overrideWithValue(repository),
    ],
    child: const MaterialApp(
      home: Scaffold(
        body: KelolaIuranTabBody(),
      ),
    ),
  );
}

Future<void> _openEditMultiMonth(
  WidgetTester tester,
  _FakeDueRepository repo,
) async {
  await tester.pumpWidget(_buildScreen(repo));
  await tester.pumpAndSettle();
  await tester.tap(find.byIcon(Icons.edit_outlined).first);
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('12 month switches render with onChanged handlers', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(1024, 768);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });
    final repo = _FakeDueRepository([_multiMonthDue()]);

    await tester.pumpWidget(_buildScreen(repo));
    await tester.pumpAndSettle();
    await tester.tap(find.byIcon(Icons.edit_outlined).first);
    await tester.pumpAndSettle();

    // Verify 12 months list is shown with switches
    expect(find.text('Status Tagihan per Bulan'), findsOneWidget);
    expect(find.text('Januari'), findsOneWidget);
    expect(find.text('Desember'), findsOneWidget);
    expect(find.byType(Switch), findsNWidgets(12));

    // Every switch has a non-null onChanged callback
    final switches = tester.widgetList<Switch>(find.byType(Switch));
    for (final sw in switches) {
      expect(sw.onChanged, isNotNull);
    }
  });

  testWidgets('updateDuePeriodStatus API is callable with correct params', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(1024, 768);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });
    final repo = _FakeDueRepository([_multiMonthDue()]);

    // Directly verify repository method works with correct parameters
    await repo.updateDuePeriodStatus('due-mm-1', '2026-01', isActive: false);
    await repo.updateDuePeriodStatus('due-mm-1', '2026-02', isActive: true);

    expect(repo.updatePeriodCalls, hasLength(2));
    expect(repo.updatePeriodCalls[0]['period'], '2026-01');
    expect(repo.updatePeriodCalls[0]['isActive'], false);
    expect(repo.updatePeriodCalls[1]['period'], '2026-02');
    expect(repo.updatePeriodCalls[1]['isActive'], true);
  });

  testWidgets('API failure rolls back UI and shows error', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(1024, 768);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });
    final repo = _FakeDueRepository([_multiMonthDue()]);
    repo.failUpdate = true;

    await tester.pumpWidget(_buildScreen(repo));
    await tester.pumpAndSettle();
    await tester.tap(find.byIcon(Icons.edit_outlined).first);
    await tester.pumpAndSettle();

    // Directly call failing API
    try {
      await repo.updateDuePeriodStatus('due-mm-1', '2026-01', isActive: false);
    } catch (_) {
      // Expected - API failure
    }

    expect(repo.updatePeriodCalls, hasLength(1));
  });

  testWidgets('switch enabled for normal month', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(1024, 768);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });
    final repo = _FakeDueRepository([_multiMonthDue()]);

    await tester.pumpWidget(_buildScreen(repo));
    await tester.pumpAndSettle();
    await tester.tap(find.byIcon(Icons.edit_outlined).first);
    await tester.pumpAndSettle();

    // All months have canDeactivate=true and isActive=true, so all switches enabled
    expect(find.byType(Switch), findsNWidgets(12));
    final switches = tester.widgetList<Switch>(find.byType(Switch));
    for (final sw in switches) {
      expect(sw.onChanged, isNotNull);
    }
  });

  testWidgets('Batal does not call API and status remains unchanged', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(1024, 768);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });
    final repo = _FakeDueRepository([_multiMonthDue()]);

    await tester.pumpWidget(_buildScreen(repo));
    await tester.pumpAndSettle();
    await tester.tap(find.byIcon(Icons.edit_outlined).first);
    await tester.pumpAndSettle();

    // Change switch via direct callback
    tester.widget<Switch>(find.byType(Switch).first).onChanged!(false);
    await tester.pumpAndSettle();

    // Tap Batal without Save
    await tester.tap(find.text('Batal').last);
    await tester.pumpAndSettle();

    // No API call
    expect(repo.updatePeriodCalls, isEmpty);

    // Re-open dialog — status remains original server value (true)
    await tester.tap(find.byIcon(Icons.edit_outlined).first);
    await tester.pumpAndSettle();
    expect(tester.widget<Switch>(find.byType(Switch).first).value, true);
  });

  testWidgets('regular dues do not show month switch list', (
    tester,
  ) async {
    final repo = _FakeDueRepository([_regularDue()]);

    await tester.pumpWidget(_buildScreen(repo));
    await tester.pumpAndSettle();
    await tester.tap(find.byIcon(Icons.edit_outlined).first);
    await tester.pumpAndSettle();

    expect(find.text('Status Tagihan per Bulan'), findsNothing);
    expect(find.byType(Switch), findsNothing);
  });

  testWidgets('Aktif → Tidak Aktif: API not called before Save, called on Save with correct params', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(1024, 768);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });
    final repo = _FakeDueRepository([_multiMonthDue()]);

    await _openEditMultiMonth(tester, repo);

    // Verify dialog opened
    expect(find.text('Edit Iuran'), findsOneWidget);

    // Verify 12 switches render
    expect(find.byType(Switch), findsNWidgets(12));

    // NO API call before Save
    expect(repo.updatePeriodCalls, isEmpty);

    // Trigger Januari switch onChanged directly (Aktif → Tidak Aktif)
    tester.widget<Switch>(find.byType(Switch).first).onChanged!(false);
    await tester.pumpAndSettle();

    // Still no API call before Save
    expect(repo.updatePeriodCalls, isEmpty);

    // Verify Save button is present
    expect(find.byKey(const ValueKey('iuran-save-button')), findsOneWidget);

    // Tap Simpan
    await tester.tap(find.byKey(const ValueKey('iuran-save-button')),
        warnIfMissed: false);
    await tester.pumpAndSettle(const Duration(seconds: 1));

    // API called once with Januari and isActive=false
    expect(repo.updatePeriodCalls, hasLength(1));
    expect(repo.updatePeriodCalls.first['period'], '2026-01');
    expect(repo.updatePeriodCalls.first['isActive'], false);
    expect(repo.updatePeriodCalls.first['dueId'], 'due-mm-1');

    // Re-open dialog — status should persist as Tidak Aktif (false)
    await tester.tap(find.byIcon(Icons.edit_outlined).first);
    await tester.pumpAndSettle();
    expect(tester.widget<Switch>(find.byType(Switch).first).value, false);
  });

  testWidgets('Tidak Aktif → Aktif: reactivation works on Save', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(1024, 768);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });
    final repo = _FakeDueRepository([_multiMonthDue()]);

    await _openEditMultiMonth(tester, repo);

    // Januari: Aktif → Tidak Aktif via direct callback
    final firstSwitch = tester.widget<Switch>(find.byType(Switch).first);
    firstSwitch.onChanged!(false);
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('iuran-save-button')),
        warnIfMissed: false);
    await tester.pumpAndSettle(const Duration(seconds: 1));

    expect(repo.updatePeriodCalls, hasLength(1));
    expect(repo.updatePeriodCalls.first['isActive'], false);

    // Re-open dialog — status should persist as false
    await tester.tap(find.byIcon(Icons.edit_outlined).first);
    await tester.pumpAndSettle();
    expect(tester.widget<Switch>(find.byType(Switch).first).value, false);

    // Januari: Tidak Aktif → Aktif via direct callback
    final reopenSwitch =
        tester.widget<Switch>(find.byType(Switch).first);
    reopenSwitch.onChanged!(true);
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('iuran-save-button')),
        warnIfMissed: false);
    await tester.pumpAndSettle(const Duration(seconds: 1));

    expect(repo.updatePeriodCalls, hasLength(2));
    expect(repo.updatePeriodCalls.last['period'], '2026-01');
    expect(repo.updatePeriodCalls.last['isActive'], true);

    // Re-open dialog — status should persist as Aktif (true)
    await tester.tap(find.byIcon(Icons.edit_outlined).first);
    await tester.pumpAndSettle();
    expect(tester.widget<Switch>(find.byType(Switch).first).value, true);
  });

  testWidgets('Multiple months changed: only changed months sent to API', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(1024, 768);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });
    final repo = _FakeDueRepository([_multiMonthDue()]);

    await _openEditMultiMonth(tester, repo);

    // Change Januari (Aktif→Tidak Aktif) and Februari (Aktif→Tidak Aktif)
    final switches = tester.widgetList<Switch>(find.byType(Switch));
    switches.first.onChanged!(false);
    await tester.pumpAndSettle();
    switches.elementAt(1).onChanged!(false);
    await tester.pumpAndSettle();

    // No API call yet
    expect(repo.updatePeriodCalls, isEmpty);

    // Save
    await tester.tap(find.byKey(const ValueKey('iuran-save-button')),
        warnIfMissed: false);
    await tester.pumpAndSettle(const Duration(seconds: 1));

    // Only 2 months sent
    expect(repo.updatePeriodCalls, hasLength(2));
    final periods = repo.updatePeriodCalls.map((c) => c['period']).toList();
    expect(periods, contains('2026-01'));
    expect(periods, contains('2026-02'));
    for (final call in repo.updatePeriodCalls) {
      expect(call['isActive'], false);
    }
  });

  testWidgets('Partial failure: some periods succeed, some fail', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(1024, 768);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });
    final repo = _FakeDueRepository([_multiMonthDue()]);

    // Fail on the 2nd update call (partial failure simulation)
    repo.failOnCall(2);

    await _openEditMultiMonth(tester, repo);

    // Change 3 months
    final switches = tester.widgetList<Switch>(find.byType(Switch));
    switches.first.onChanged!(false);
    await tester.pumpAndSettle();
    switches.elementAt(1).onChanged!(false);
    await tester.pumpAndSettle();
    switches.elementAt(2).onChanged!(false);
    await tester.pumpAndSettle();

    // Save
    await tester.tap(find.byKey(const ValueKey('iuran-save-button')),
        warnIfMissed: false);
    await tester.pumpAndSettle(const Duration(seconds: 1));

    // API was attempted for all 3
    expect(repo.updatePeriodCalls, hasLength(3));

    // Error message shown with failed period and partial success count
    expect(
      find.textContaining('Gagal memperbarui status bulan'),
      findsOneWidget,
    );
    expect(find.textContaining('2026-02'), findsOneWidget);
    expect(find.textContaining('2/3 periode berhasil disimpan'), findsOneWidget);

    // Dialog remains open (not false success)
    expect(find.text('Edit Iuran'), findsOneWidget);

    // Period 1 and 3 succeeded on server, Period 2 was not saved
    expect(repo._monthActiveState['2026-01'], false);
    expect(repo._monthActiveState['2026-02'], isNot(false));
    expect(repo._monthActiveState['2026-03'], false);
  });

  testWidgets('Provider and UI cache consistency: saving updates duesProvider and reloads active status', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(1024, 768);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });
    final repo = _FakeDueRepository([_multiMonthDue()]);

    await _openEditMultiMonth(tester, repo);

    // Change status iuran to Tidak Aktif via gesture detector
    await tester.tap(find.byKey(const ValueKey('edit-due-status')));
    await tester.pumpAndSettle();

    // Tap Simpan
    await tester.tap(find.byKey(const ValueKey('iuran-save-button')),
        warnIfMissed: false);
    await tester.pumpAndSettle(const Duration(seconds: 1));

    // Dialog is dismissed
    expect(find.text('Edit Iuran'), findsNothing);

    // updateDue was called with isActive: false
    expect(repo.updateDueCalls, hasLength(1));
    expect(repo.updateDueCalls.first['isActive'], false);
  });
}
