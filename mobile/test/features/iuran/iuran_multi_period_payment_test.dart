import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:social_finance/core/errors/app_errors.dart';
import 'package:social_finance/features/iuran/data/iuran_api_client.dart';
import 'package:social_finance/features/iuran/data/iuran_api_models.dart';
import 'package:social_finance/features/iuran/data/iuran_models.dart';
import 'package:social_finance/features/iuran/data/iuran_providers.dart';
import 'package:social_finance/features/iuran/data/iuran_repository.dart';
import 'package:social_finance/features/iuran/presentation/screens/iuran_payment_screen.dart';

class MockIuranApiClient extends Fake implements IuranApiClient {
  List<BackendBill> cannedBills = [];

  @override
  Future<PaginatedResponse<BackendBill>> listBills({
    String? search,
    String? dueId,
    String? status,
    String? paymentStatus,
    String? period,
    String? householdOccupancyId,
    bool myBills = false,
    int page = 1,
    int pageSize = 20,
  }) async {
    return PaginatedResponse(
      data: cannedBills,
      pagination: PaginationInfo(
        page: 1,
        pageSize: pageSize,
        total: cannedBills.length,
        totalPages: 1,
      ),
    );
  }
}

class MockIuranRepository extends Fake implements IuranRepository {
  final MockIuranApiClient client;
  List<String>? lastBillIds;
  List<Map<String, String>>? lastAllocations;
  int? lastAmount;
  PaymentMethod? lastMethod;
  bool createCalled = false;

  MockIuranRepository({required this.client});

  @override
  IuranApiClient get apiClient => client;

  @override
  ServerException mapDioError(dynamic error) =>
      ServerException(message: error.toString());

  @override
  Future<BackendMultiPeriodPaymentResult> createMultiPeriodPayment({
    required List<String> billIds,
    required List<Map<String, String>> allocations,
    required int amount,
    required PaymentMethod method,
    String? proofPath,
    String? notes,
    String? idempotencyKey,
    Uint8List? fileBytes,
    String? filename,
  }) async {
    createCalled = true;
    lastBillIds = billIds;
    lastAllocations = allocations;
    lastAmount = amount;
    lastMethod = method;

    return BackendMultiPeriodPaymentResult(
      payment: BackendPayment(
        id: 'multi-pay-1',
        rtId: 'rt-1',
        billId: null,
        dueId: 'due-1',
        amount: '$amount.00',
        method: method,
        origin: PaymentOrigin.selfSubmitted,
        status: PaymentStatus.pending,
      ),
      allocations: allocations
          .map(
            (a) => BackendPaymentAllocation(
              id: 'alloc-${a['bill_id']}',
              paymentId: 'multi-pay-1',
              billId: a['bill_id']!,
              amount: a['amount']!,
            ),
          )
          .toList(),
    );
  }
}

void main() {
  const kBillId = 'bill-1';

  final testMultiMonthBill = IuranBill(
    id: kBillId,
    householdName: 'Bpk. Ahmad',
    iuranType: 'Iuran Sampah & Keamanan',
    periode: '2026',
    nominal: 50000,
    paidAmount: 0,
    status: IuranStatus.belumBayar,
    dueId: 'due-1',
    dueIsActive: true,
    householdOccupancyId: 'occ-1',
    requiresFullPayment: true,
    dueType: DueType.multiMonth,
    year: 2026,
  );

  final testRegularBill = IuranBill(
    id: kBillId,
    householdName: 'Bpk. Ahmad',
    iuranType: 'Iuran Kebersihan',
    periode: 'Oktober 2026',
    nominal: 50000,
    paidAmount: 0,
    status: IuranStatus.belumBayar,
    dueId: 'due-reg',
    dueIsActive: true,
    householdOccupancyId: 'occ-1',
    requiresFullPayment: true,
    dueType: DueType.regular,
  );

  final List<BackendBill> unpaidBills = [
    const BackendBill(
      id: 'bill-1',
      rtId: 'rt-1',
      householdOccupancyId: 'occ-1',
      dueId: 'due-1',
      amount: '50000.00',
      period: '2026-10',
      dueDate: '2026-10-10',
      status: BillStatus.unpaid,
      dueName: 'Iuran Sampah & Keamanan',
      dueIsActive: true,
      headName: 'Bpk. Ahmad',
      dueType: DueType.multiMonth,
      year: 2026,
    ),
    const BackendBill(
      id: 'bill-2',
      rtId: 'rt-1',
      householdOccupancyId: 'occ-1',
      dueId: 'due-1',
      amount: '50000.00',
      period: '2026-11',
      dueDate: '2026-11-10',
      status: BillStatus.unpaid,
      dueName: 'Iuran Sampah & Keamanan',
      dueIsActive: true,
      headName: 'Bpk. Ahmad',
      dueType: DueType.multiMonth,
      year: 2026,
    ),
    const BackendBill(
      id: 'bill-3',
      rtId: 'rt-1',
      householdOccupancyId: 'occ-1',
      dueId: 'due-1',
      amount: '75000.00', // Stored tariff difference
      period: '2026-12',
      dueDate: '2026-12-10',
      status: BillStatus.unpaid,
      dueName: 'Iuran Sampah & Keamanan',
      dueIsActive: true,
      headName: 'Bpk. Ahmad',
      dueType: DueType.multiMonth,
      year: 2026,
    ),
  ];

  Widget buildApp({
    required MockIuranRepository repo,
    IuranBill? bill,
  }) {
    return MaterialApp(
      home: ProviderScope(
        overrides: [
          billDetailProvider(kBillId).overrideWith(
            (ref) async => bill ?? testMultiMonthBill,
          ),
          iuranRepositoryProvider.overrideWithValue(repo),
        ],
        child: const IuranPaymentScreen(billId: kBillId),
      ),
    );
  }

  group('IuranPaymentScreen — Multi-Bulan & Reguler Payment Features', () {
    testWidgets(
      '1. Tagihan Reguler tidak menampilkan pemilih multi-bulan dan mempertahankan alur standar 1 tagihan',
      (tester) async {
        final client = MockIuranApiClient();
        client.cannedBills = unpaidBills;
        final repo = MockIuranRepository(client: client);

        await tester.pumpWidget(buildApp(repo: repo, bill: testRegularBill));
        await tester.pumpAndSettle();

        expect(find.text('Iuran Kebersihan'), findsOneWidget);
        expect(find.byKey(const Key('dropdown_month_count')), findsNothing);
      },
    );

    testWidgets(
      '2. Tagihan Multi-Bulan memuat tagihan tertua dan dropdown bulan dimulai dari 0 Bulan dengan tombol disabled',
      (tester) async {
        final client = MockIuranApiClient();
        client.cannedBills = unpaidBills;
        final repo = MockIuranRepository(client: client);

        await tester.pumpWidget(buildApp(repo: repo));
        await tester.pumpAndSettle();

        expect(find.text('Jumlah Bulan yang Dibayar'), findsOneWidget);
        expect(find.byKey(const Key('dropdown_month_count')), findsOneWidget);
        expect(find.textContaining('0 Bulan'), findsOneWidget);

        // Submit button is disabled when 0 months selected
        await tester.drag(find.byType(ListView), const Offset(0, -500));
        await tester.pumpAndSettle();

        final btn = tester.widget<OutlinedButton>(
          find.byKey(const Key('btn_save')),
        );
        expect(btn.onPressed, isNull);
      },
    );

    testWidgets(
      '3. Memilih jumlah bulan mengaktifkan tombol dan memperbarui nominal sesuai stored bills (FIFO)',
      (tester) async {
        final client = MockIuranApiClient();
        client.cannedBills = unpaidBills;
        final repo = MockIuranRepository(client: client);

        await tester.pumpWidget(buildApp(repo: repo));
        await tester.pumpAndSettle();

        // Open dropdown and select 3 Bulan
        await tester.tap(find.byKey(const Key('dropdown_month_count')));
        await tester.pumpAndSettle();

        await tester.tap(find.text('3 Bulan (3 Tagihan)').last);
        await tester.pumpAndSettle();

        // All 3 periods shown
        expect(find.text('Oktober 2026'), findsOneWidget);
        expect(find.text('November 2026'), findsOneWidget);
        expect(find.text('Desember 2026'), findsOneWidget);

        // Total = 50k + 50k + 75k = 175k
        expect(find.text('Rp 75.000'), findsOneWidget);
        expect(find.text('Rp 175.000'), findsOneWidget);

        // Submit button is now enabled
        await tester.drag(find.byType(ListView), const Offset(0, -600));
        await tester.pumpAndSettle();

        final btn = tester.widget<OutlinedButton>(
          find.byKey(const Key('btn_save')),
        );
        expect(btn.onPressed, isNotNull);
      },
    );

    testWidgets(
      '4. Menekan Bayar memanggil createMultiPeriodPayment dengan alokasi FIFO yang tepat',
      (tester) async {
        final client = MockIuranApiClient();
        client.cannedBills = unpaidBills;
        final repo = MockIuranRepository(client: client);

        await tester.pumpWidget(buildApp(repo: repo));
        await tester.pumpAndSettle();

        // Select 2 months
        await tester.tap(find.byKey(const Key('dropdown_month_count')));
        await tester.pumpAndSettle();
        await tester.tap(find.text('2 Bulan (2 Tagihan)').last);
        await tester.pumpAndSettle();

        // Drag list to reveal buttons
        await tester.drag(find.byType(ListView), const Offset(0, -600));
        await tester.pumpAndSettle();

        await tester.tap(find.byKey(const Key('btn_save')));
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 500));

        expect(repo.createCalled, isTrue);
        expect(repo.lastBillIds, ['bill-1', 'bill-2']);
        expect(repo.lastAllocations, [
          {'bill_id': 'bill-1', 'amount': '50000.00'},
          {'bill_id': 'bill-2', 'amount': '50000.00'},
        ]);
        expect(repo.lastAmount, 100000);
      },
    );

    testWidgets(
      '5. Menampilkan empty state ketika tidak ada tagihan valid yang dapat dibayar',
      (tester) async {
        final client = MockIuranApiClient();
        client.cannedBills = []; // No bills
        final repo = MockIuranRepository(client: client);

        await tester.pumpWidget(buildApp(repo: repo));
        await tester.pumpAndSettle();

        expect(
          find.text('Semua tagihan untuk iuran ini telah lunas atau tidak ada tagihan tertunggak.'),
          findsOneWidget,
        );

        // Submit button is disabled
        await tester.drag(find.byType(ListView), const Offset(0, -300));
        await tester.pumpAndSettle();

        final btn = tester.widget<OutlinedButton>(
          find.byKey(const Key('btn_save')),
        );
        expect(btn.onPressed, isNull);
      },
    );

    testWidgets(
      '6. Maksimal 12 tagihan dalam pilihan dropdown',
      (tester) async {
        final fourteenBills = List.generate(
          14,
          (i) => BackendBill(
            id: 'bill-$i',
            rtId: 'rt-1',
            householdOccupancyId: 'occ-1',
            dueId: 'due-1',
            amount: '50000.00',
            period: '2026-${(i + 1).toString().padLeft(2, '0')}',
            dueDate: '2026-10-10',
            status: BillStatus.unpaid,
            dueName: 'Iuran Sampah & Keamanan',
            dueIsActive: true,
            dueType: DueType.multiMonth,
            year: 2026,
          ),
        );

        final client = MockIuranApiClient();
        client.cannedBills = fourteenBills;
        final repo = MockIuranRepository(client: client);

        await tester.pumpWidget(buildApp(repo: repo));
        await tester.pumpAndSettle();

        final dropdown = tester.widget<DropdownButton<int>>(
          find.byType(DropdownButton<int>),
        );
        // 1 item for 0 Bulan + 12 items for months = 13 items total
        expect(dropdown.items?.length, 13);
      },
    );
  });
}
