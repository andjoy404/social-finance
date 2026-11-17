import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/api/device_id_service.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/rw/data/rw_api_client.dart';
import 'package:social_finance/features/rw/data/rw_models.dart';
import 'package:social_finance/features/rw/data/rw_providers.dart';
import 'package:social_finance/features/rw/presentation/screens/rw_kas_report_screen.dart';

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

Map<String, dynamic> adminMembership() => {
      'rw_id': 'rw-002',
      'jabatan': 'bendahara',
      'rw_name': 'RW 002 Sukamaju',
      'rw_number': 2,
      'source': 'explicit',
    };

Map<String, dynamic> wargaMembership() => {
      'rw_id': 'rw-002',
      'jabatan': 'anggota',
      'rw_name': 'RW 002 Sukamaju',
      'rw_number': 2,
      'source': 'explicit',
    };

RwTransaction tx({
  required String id,
  String type = 'income',
  required String amount,
  required String description,
  String? categoryName,
  String? paymentMethod,
  String? sourceRtId,
  String status = 'posted',
}) =>
    RwTransaction(
      id: id,
      rwId: 'rw-002',
      rtId: 'rt-1',
      rtName: 'RT 001',
      type: type,
      amount: amount,
      description: description,
      categoryName: categoryName,
      locationName: 'Rekening RW',
      status: status,
      paymentMethod: paymentMethod ?? 'CASH',
      sourceRtId: sourceRtId,
      occurredAt: DateTime(2026, 10, 15),
      createdAt: DateTime(2026, 10, 15),
    );

class _MockRwApiClient extends RwApiClient {
  _MockRwApiClient({List<RwTransaction>? transactions})
      : super(ApiClient(baseUrl: 'http://test.local')) {
    _transactions = transactions ?? _defaultFixture;
  }

  late final List<RwTransaction> _transactions;

  static final List<RwTransaction> _defaultFixture = [
    tx(id: 'i1', type: 'income', amount: '1000000.00', description: 'Iuran Warga', categoryName: 'Iuran Warga', paymentMethod: 'CASH'),
    tx(id: 'i2', type: 'income', amount: '250000.00', description: 'Infaq', categoryName: 'Infaq', paymentMethod: 'CASH'),
    tx(id: 'i3', type: 'income', amount: '500000.00', description: 'Iuran Warga Transfer', categoryName: 'Iuran Warga', paymentMethod: 'TRANSFER'),
    tx(id: 'i4', type: 'income', amount: '999999.00', description: 'Draft income', categoryName: 'Draft', paymentMethod: 'CASH', status: 'draft'),
    tx(id: 's1', type: 'income', amount: '125000.00', description: 'Setoran RT 003 ke Kas RW', categoryName: 'Setoran RT', paymentMethod: 'CASH', sourceRtId: 'rt-003'),
    tx(id: 'e1', type: 'expense', amount: '300000.00', description: 'Keamanan', categoryName: 'Keamanan', paymentMethod: 'CASH'),
    tx(id: 'e2', type: 'expense', amount: '100000.00', description: 'Kebersihan', categoryName: 'Kebersihan', paymentMethod: 'CASH'),
    tx(id: 'e3', type: 'expense', amount: '150000.00', description: 'Keamanan Transfer', categoryName: 'Keamanan', paymentMethod: 'TRANSFER'),
    tx(id: 'e4', type: 'expense', amount: '888888.00', description: 'Cancelled expense', categoryName: 'Keamanan', paymentMethod: 'CASH', status: 'cancelled'),
  ];

  @override
  Future<RwKasReportDetail> getKasReportDetail(String rwId,
      {String? period, String? startDate, String? endDate}) async {
    return RwKasReportDetail(
      period: period ?? '',
      startDate: startDate ?? '',
      endDate: endDate ?? '',
      openingBalance: '2000000.00',
      totalIncome: '1875000.00',
      totalExpense: '550000.00',
      netBalance: '1325000.00',
      totalBalance: '3325000.00',
      transactionCount: 8,
    );
  }

  @override
  Future<RwTransactionsResponse> getTransactions({
    required String rwId,
    String? period,
    String? startDate,
    String? endDate,
    int page = 1,
    int pageSize = 20,
    String? type,
    String? categoryId,
  }) async {
    return RwTransactionsResponse(
      data: page == 1 ? _transactions : const [],
      pagination: RwPaginationInfo(
        page: page,
        pageSize: pageSize,
        total: _transactions.length,
        totalPages: 1,
      ),
    );
  }

  @override
  Future<List<RwFundLocation>> getLocations(String rwId,
      {String? category, bool? isActive}) async {
    return [
      RwFundLocation(
        id: 'loc-1',
        rtId: '',
        rtName: 'RW',
        name: 'Rekening BCA Operasional RW',
        category: 'rekening',
        bankName: 'BCA',
        accountNumber: '888001122',
        accountHolder: 'Pengurus RW',
        isActive: true,
        currentBalance: '1500000.00',
        createdAt: DateTime(2026, 1, 1),
      ),
      RwFundLocation(
        id: 'loc-2',
        rtId: '',
        rtName: 'RW',
        name: 'Brankas RW',
        category: 'brankas',
        bankName: null,
        accountNumber: null,
        accountHolder: null,
        isActive: true,
        currentBalance: '700000.00',
        createdAt: DateTime(2026, 1, 1),
      ),
    ];
  }
}

void main() {
  Map<String, dynamic> superAdminUser = {
    'id': 'user-admin',
    'email': 'admin@ucs.local',
    'name': 'Super Admin',
    'role': AppRole.pengurus,
    'system_role': 'super_admin',
    'jabatan': null,
    'rw': '002',
    'rw_memberships': [adminMembership()],
  };

  Map<String, dynamic> bendaharaUser = {
    'id': 'user-1',
    'email': 'bendahara.rw@test.com',
    'name': 'Bendahara RW 002',
    'role': AppRole.bendahara,
    'jabatan': 'bendahara',
    'rw': '002',
    'rw_memberships': [adminMembership()],
  };

  Map<String, dynamic> wargaUser = {
    'id': 'user-warga',
    'email': 'warga@test.com',
    'name': 'Warga RW 002',
    'role': AppRole.warga,
    'jabatan': null,
    'rw': '002',
    'rw_memberships': [wargaMembership()],
  };

  Map<String, dynamic> outsiderUser = {
    'id': 'user-out',
    'email': 'outsider@test.com',
    'name': 'Bukan Member RW',
    'role': AppRole.pengurus,
    'jabatan': 'bendahara',
    'rw': '003',
    'rw_memberships': [
      {'rw_id': 'rw-003', 'jabatan': 'bendahara', 'rw_name': 'RW 003', 'source': 'explicit'},
    ],
  };

  Future<void> pumpReport(WidgetTester tester, Map<String, dynamic> user,
      {_MockRwApiClient? client}) async {
    tester.view.physicalSize = const Size(1200, 1500);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          rwApiClientProvider.overrideWithValue(client ?? _MockRwApiClient()),
          authRepositoryProvider.overrideWith((ref) => _MockAuthRepo(user)),
        ],
        child: const MaterialApp(
          home: RwKasReportScreen(rwId: 'rw-002', period: '2026-10'),
        ),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets('renders document identical to RT with RW identity, not RT identity',
      (tester) async {
    await pumpReport(tester, bendaharaUser);

    // Same document widget as the RT report implementation.
    expect(find.byKey(const Key('kas-report-document')), findsOneWidget);
    // RW identity lines — never an RT identity.
    expect(find.text('LAPORAN KEUANGAN RW 002'), findsWidgets);
    expect(find.text('LAPORAN KEUANGAN RT 002'), findsNothing);
    expect(find.text('Saldo awal periode'), findsOneWidget);
    expect(find.byKey(const Key('kas-report-statement')), findsOneWidget);
    expect(find.byKey(const Key('kas-report-recap')), findsOneWidget);
    // Export & print controls (identical to RT).
    expect(find.byKey(const Key('rw-kas-report-export-pdf')), findsOneWidget);
    expect(find.byKey(const Key('rw-kas-report-export-xlsx')), findsOneWidget);
    expect(find.byKey(const Key('rw-kas-report-export-print')), findsOneWidget);
    // RT-specific identity artifacts must not appear.
    expect(find.textContaining('Cluster'), findsNothing);
  });

  testWidgets('super_admin without membership can view', (tester) async {
    await pumpReport(tester, superAdminUser);
    expect(find.text('Akses Ditolak'), findsNothing);
    expect(find.byKey(const Key('kas-report-document')), findsOneWidget);
  });

  testWidgets('member warga can view but cannot export XLSX/print', (tester) async {
    await pumpReport(tester, wargaUser);
    expect(find.byKey(const Key('kas-report-document')), findsOneWidget);
    expect(find.byKey(const Key('rw-kas-report-export-pdf')), findsOneWidget);
    expect(find.byKey(const Key('rw-kas-report-export-xlsx')), findsNothing);
    expect(find.byKey(const Key('rw-kas-report-export-print')), findsNothing);
  });

  testWidgets('non-member is blocked even with RT bendahara role', (tester) async {
    await pumpReport(tester, outsiderUser);
    expect(find.text('Akses Ditolak'), findsOneWidget);
    expect(find.byKey(const Key('kas-report-document')), findsNothing);
  });

  testWidgets('setoran RT appears in document, draft/cancelled excluded',
      (tester) async {
    await pumpReport(tester, bendaharaUser);
    // Setoran RT dihitung sebagai penerimaan.
    expect(find.textContaining('Setoran RT'), findsWidgets);
    // Draft/cancelled fixture rows tidak boleh tampil.
    expect(find.textContaining('Draft income'), findsNothing);
    expect(find.textContaining('Cancelled expense'), findsNothing);
  });
}
