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
  final List<Map<String, dynamic>> generateCalls = [];

  @override
  Future<List<BackendDue>> fetchDues({bool? isActive}) async => List.of(_dues);

  @override
  Future<List<DueMonthSummary>> fetchDueMonths(String dueId) async {
    return List.generate(
      12,
      (index) => DueMonthSummary(
        period: '2026-${(index + 1).toString().padLeft(2, '0')}',
        month: index + 1,
        year: 2026,
        totalBills: 5,
        activeBills: 5,
        paidBills: 0,
        unpaidBills: 5,
        isActive: true,
        canDeactivate: true,
      ),
    );
  }

  @override
  Future<GenerateBillsResult> generateBills({
    required String dueId,
    String? period,
    String? dueDate,
    String? idempotencyKey,
  }) async {
    generateCalls.add({
      'dueId': dueId,
      'period': period,
      'dueDate': dueDate,
      'idempotencyKey': idempotencyKey,
    });
    return const GenerateBillsResult(
      generatedCount: 12,
      bills: [],
    );
  }
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

void main() {
  testWidgets('renders "Sinkronkan Tagihan Warga" button and cancels on dialog Batal', (
    tester,
  ) async {
    final repo = _FakeDueRepository([_multiMonthDue()]);

    await tester.pumpWidget(_buildScreen(repo));
    await tester.pumpAndSettle();

    // Tap edit icon on IPL 2026
    await tester.tap(find.byIcon(Icons.edit_outlined).first);
    await tester.pumpAndSettle();

    expect(find.text('Edit Iuran'), findsOneWidget);
    expect(find.text('Nama Iuran'), findsOneWidget);
    // Pastikan baris switch "Iuran Sukarela" sudah tidak ada di popup Edit Iuran
    expect(find.text('Iuran Sukarela'), findsNothing);

    final syncBtn = find.byKey(const ValueKey('btn-sync-bills'));
    await tester.ensureVisible(syncBtn);
    await tester.pumpAndSettle();

    expect(syncBtn, findsOneWidget);
    expect(find.text('Sinkronkan Tagihan Warga'), findsOneWidget);
    expect(find.text('Penerbitan Susulan Warga (2026)'), findsOneWidget);

    // Tap Sinkronkan Tagihan Warga button
    await tester.tap(syncBtn);
    await tester.pumpAndSettle();

    // Confirm dialog appears
    expect(find.text('Sinkronkan Tagihan Warga'), findsNWidgets(2)); // in background button and dialog title
    expect(find.text('Sinkronkan'), findsOneWidget);
    expect(
      find.textContaining('Sistem akan menerbitkan 12 tagihan (2026) untuk setiap Warga'),
      findsOneWidget,
    );

    // Click Batal on confirmation dialog
    await tester.tap(find.text('Batal').last);
    await tester.pumpAndSettle();

    // Dialog closed, no API call made
    expect(find.text('Sinkronkan Tagihan Warga'), findsOneWidget);
    expect(repo.generateCalls, isEmpty);
  });

  testWidgets('clicking Sinkronkan calls generateBills and displays success feedback', (
    tester,
  ) async {
    final repo = _FakeDueRepository([_multiMonthDue()]);

    await tester.pumpWidget(_buildScreen(repo));
    await tester.pumpAndSettle();

    // Tap edit icon
    await tester.tap(find.byIcon(Icons.edit_outlined).first);
    await tester.pumpAndSettle();

    final syncBtn = find.byKey(const ValueKey('btn-sync-bills'));
    await tester.ensureVisible(syncBtn);
    await tester.pumpAndSettle();

    // Tap Sinkronkan Tagihan Warga
    await tester.tap(syncBtn);
    await tester.pumpAndSettle();

    // Click Sinkronkan in confirmation dialog
    await tester.tap(find.text('Sinkronkan'));
    await tester.pumpAndSettle();

    // Verify generateBills was called
    expect(repo.generateCalls.length, 1);
    expect(repo.generateCalls.first['dueId'], 'due-mm-1');
    expect(repo.generateCalls.first['period'], '2026');

    // Verify success banner is shown
    expect(
      find.text('Sinkronisasi berhasil: 12 tagihan baru diterbitkan.'),
      findsOneWidget,
    );
  });

  testWidgets('does NOT render "Sinkronkan Tagihan Warga" for regular dues', (
    tester,
  ) async {
    final repo = _FakeDueRepository([_regularDue()]);

    await tester.pumpWidget(_buildScreen(repo));
    await tester.pumpAndSettle();

    // Tap edit icon on regular due
    await tester.tap(find.byIcon(Icons.edit_outlined).first);
    await tester.pumpAndSettle();

    expect(find.text('Edit Iuran'), findsOneWidget);
    expect(find.text('Iuran Sukarela'), findsNothing);
    expect(find.byKey(const ValueKey('btn-sync-bills')), findsNothing);
  });

  testWidgets('responsiveness test: renders cleanly on small screen without RenderFlex overflow', (
    tester,
  ) async {
    // Set small screen size (360x640)
    tester.view.physicalSize = const Size(360, 640);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });

    final repo = _FakeDueRepository([_multiMonthDue()]);

    await tester.pumpWidget(_buildScreen(repo));
    await tester.pumpAndSettle();

    // Open Edit Iuran dialog
    await tester.tap(find.byIcon(Icons.edit_outlined).first);
    await tester.pumpAndSettle();

    // Both title and first input field are rendered without clipping or collision
    expect(find.text('Edit Iuran'), findsOneWidget);
    expect(find.text('Nama Iuran'), findsOneWidget);

    // Save and Cancel buttons are accessible
    expect(find.text('Batal'), findsOneWidget);
    expect(find.text('Simpan'), findsOneWidget);

    // Verify 12 months list is scrollable
    final lastMonth = find.text('Desember');
    await tester.scrollUntilVisible(
      lastMonth,
      100,
      scrollable: find.byType(Scrollable).last,
    );
    await tester.pumpAndSettle();

    expect(lastMonth, findsOneWidget);
  });
}
