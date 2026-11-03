// Save-success & save-failure flow tests for IuranPaymentScreen.
//
// These tests exercise the actual save flow via UI:
//  - Provide billDetailProvider with an IuranBill so the form renders.
//  - Enter payment method and amount.
//  - Tap Simpan — success → no dialog, pop + navigate away.
//  - Tap Simpan — failure → dialog NOT shown, form stays with error.

import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/navigators/kas_form_navigation_guard.dart';
import 'package:social_finance/features/iuran/data/iuran_api_client.dart';
import 'package:social_finance/features/iuran/data/iuran_api_models.dart';
import 'package:social_finance/features/iuran/data/iuran_models.dart';
import 'package:social_finance/features/iuran/data/iuran_providers.dart';
import 'package:social_finance/features/iuran/data/iuran_repository.dart';
import 'package:social_finance/features/iuran/presentation/screens/iuran_payment_screen.dart';

// ─── Fake Iuran Repository ───────────────────────────────────────────────

class _FakeIuranRepository extends IuranRepository {
  _FakeIuranRepository({this.failCreatePayment = false})
      : super(apiClient: _FakeIuranApiClient(failCreate: failCreatePayment));

  final bool failCreatePayment;

  @override
  Future<List<BackendDue>> fetchDues({bool? isActive}) async => [];

  @override
  Future<List<IuranBill>> fetchAllBills({
    String? search,
    String? status,
    String? paymentStatus,
    bool myBills = false,
  }) async => [];
}

class _FakeIuranApiClient implements IuranApiClient {
  final bool failCreate;

  _FakeIuranApiClient({this.failCreate = false});

  @override
  Dio get dio => Dio(BaseOptions(baseUrl: 'http://test.local'));

  @override
  Future<BackendDue> createDue({
    required String name,
    required String amount,
    required DuePeriodType periodType,
    PaymentRule? paymentRule,
    bool? isVoluntary,
    String? validUntil,
    DueType? dueType,
    int? year,
  }) async {
    return const BackendDue(
      id: 'due-created',
      rtId: 'rt-002',
      name: 'Iuran Sampah',
      amount: '25000.00',
      periodType: DuePeriodType.monthly,
      paymentRule: PaymentRule.full,
      isVoluntary: false,
      isActive: true,
    );
  }

  @override
  Future<BackendDue> updateDue(
    String id, {
    String? name,
    String? amount,
    DuePeriodType? periodType,
    PaymentRule? paymentRule,
    bool? isVoluntary,
    bool? isActive,
    String? validUntil,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<void> deactivateDue(String id) async {}

  @override
  Future<List<DueMonthSummary>> getDueMonths(String dueId) async => [];

  @override
  Future<List<DueMonthSummary>> updateDuePeriodStatus(
    String dueId,
    String period, {
    required bool isActive,
  }) async => [];

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
    int pageSize = 30,
    String? order,
  }) async {
    return const PaginatedResponse<BackendBill>(
      data: [],
      pagination: PaginationInfo(page: 1, pageSize: 30, total: 0, totalPages: 0),
    );
  }

  @override
  Future<BackendBill> getBill(String id) async {
    throw UnimplementedError();
  }

  @override
  Future<GenerateBillsResult> generateBills({
    required String dueId,
    required String period,
    required String dueDate,
    required String idempotencyKey,
  }) async {
    return GenerateBillsResult(generatedCount: 1, bills: []);
  }

  @override
  GenerateBillsResult parseGenerateBillsResponse(dynamic data) {
    throw UnimplementedError();
  }

  GenerateBillsResult _parseGenerateBillsResponse(dynamic data) {
    throw UnimplementedError();
  }

  @override
  Future<void> cancelBill(String id) async {}

  @override
  Future<PaginatedResponse<BackendPayment>> listPayments({
    String? billId,
    String? dueId,
    String? status,
    int page = 1,
    int pageSize = 20,
  }) async {
    return const PaginatedResponse<BackendPayment>(
      data: [],
      pagination: PaginationInfo(page: 1, pageSize: 20, total: 0, totalPages: 0),
    );
  }

  @override
  Future<BackendPayment> getPayment(String id) async {
    throw UnimplementedError();
  }

  @override
  Future<BackendPayment> createPayment({
    required String billId,
    required String amount,
    required PaymentMethod method,
    String? proofPath,
    String? notes,
    String? idempotencyKey,
    Uint8List? fileBytes,
    String? filename,
  }) async {
    if (failCreate) {
      throw DioException(
        requestOptions: RequestOptions(path: '/api/v1/payments'),
        response: Response<dynamic>(statusCode: 500, requestOptions: RequestOptions(path: '/api/v1/payments')),
      );
    }
    return BackendPayment(
      id: 'payment-created',
      rtId: 'rt-002',
      billId: billId,
      amount: '25000',
      method: method,
      origin: PaymentOrigin.selfSubmitted,
      status: PaymentStatus.pending,
      notes: notes ?? '',
      proofPath: proofPath,
    );
  }

  @override
  Future<BackendPayment> createDonation({
    required String dueId,
    required String amount,
    required PaymentMethod method,
    String? notes,
    String? idempotencyKey,
    Uint8List? fileBytes,
    String? filename,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<BackendPayment> uploadPaymentProof({
    required String paymentId,
    required List<int> fileBytes,
    required String filename,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<Uint8List> getPaymentProofBytes(String paymentId) async {
    throw UnimplementedError();
  }

  @override
  Future<BackendPayment> verifyPayment(
    String paymentId, {
    required String action,
    String? rejectionReason,
    String? officerNotes,
    String? locationId,
    String? actualReceivedDate,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<BackendPayment> reversePayment(
    String paymentId, {
    required String reason,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<List<BackendPayment>> listPaymentsForBill(String billId) async => [];

  @override
  Future<List<BackendPayment>> listPaymentsForDue(String dueId) async => [];

  @override
  Future<List<BackendPayment>> listPaymentsForBillTarget({
    required String billId,
    String? dueId,
    bool isVoluntary = false,
  }) async => [];

  @override
  Future<List<BackendDue>> listDues({bool? isActive}) async => [];

  @override
  Future<BackendBalance> getBalance({
    String? period,
    String? startDate,
    String? endDate,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<Map<String, dynamic>> updateOpeningBalance({
    required String period,
    required String openingBalance,
    String? reason,
  }) async => {};

  @override
  Future<Map<String, dynamic>> getDashboardSummary([String? period]) async => {};

  @override
  Future<List<BackendCategory>> listCategories({bool? isActive}) async => [];

  @override
  Future<BackendMultiPeriodPaymentResult> createMultiPeriodPayment({
    required List<String> billIds,
    required List<Map<String, String>> allocations,
    required String amount,
    required PaymentMethod method,
    String? notes,
    String? proofPath,
    String? idempotencyKey,
    Uint8List? fileBytes,
    String? filename,
  }) async {
    throw UnimplementedError();
  }
}

// ─── Helpers ─────────────────────────────────────────────────────────────

GlobalKey<NavigatorState> _createNavigatorKey() => GlobalKey<NavigatorState>();

Widget _buildApp({
  required IuranRepository repository,
  required IuranBill bill,
}) {
  final navigatorKey = _createNavigatorKey();
  return ProviderScope(
    key: ValueKey('iuran-payment-save-test-${DateTime.now().millisecondsSinceEpoch}'),
    overrides: [
      iuranRepositoryProvider.overrideWithValue(repository),
      billDetailProvider.overrideWith((ref, billId) => AsyncData<IuranBill?>(bill)),
    ],
    child: MaterialApp.router(
      routerConfig: GoRouter(
        initialLocation: '/home/iuran/payment/test-bill-id',
        navigatorKey: navigatorKey,
        routes: [
          GoRoute(
            path: '/home/iuran/payment/:billId',
            pageBuilder: (context, state) => MaterialPage(
              child: IuranPaymentScreen(billId: 'test-bill-id'),
            ),
          ),
        ],
      ),
    ),
  );
}

IuranBill _makeBill({
  String id = 'test-bill-id',
  String nominal = '25000',
}) {
  return IuranBill(
    id: id,
    householdName: 'Rumah Test',
    iuranType: 'Iuran Rutin',
    periode: '2026-11',
    nominal: int.tryParse(nominal.replaceAll('.', '')) ?? 25000,
    status: IuranStatus.belumBayar,
    isVoluntary: false,
    requiresFullPayment: true,
    dueType: DueType.regular,
    isObligatory: true,
    paymentNumbers: const [],
  );
}

Finder _simpanButton() => find.text('Simpan');

// ─── Tests ───────────────────────────────────────────────────────────────

void main() {
  setUp(() {
    KasFormNavigationGuard.resetForTesting();
  });

  group('IuranPaymentScreen — Save success', () {
    testWidgets(
      'Simpan pembayaran berhasil → tidak muncul dialog, navigasi keluar',
      (tester) async {
        final repo = _FakeIuranRepository();
        final bill = _makeBill();

        await tester.pumpWidget(_buildApp(
          repository: repo,
          bill: bill,
        ));
        await tester.pumpAndSettle();

        // Pilih metode pembayaran Tunai
        await tester.tap(find.byIcon(Icons.payments_outlined));
        await tester.pumpAndSettle();

        // Nominal otomatis terisi karena tagihan reguler
        expect(find.byKey(const Key('payment-amount-field')), findsOneWidget);

        // Tap Simpan
        await tester.tap(_simpanButton());
        await tester.pumpAndSettle();

        // Spinner muncul saat menyimpan
        expect(find.byType(CircularProgressIndicator), findsOneWidget);
        await tester.pumpAndSettle(const Duration(seconds: 2));

        // Snackbar sukses muncul — tidak ada dialog
        expect(find.byType(SnackBar), findsOneWidget);
        expect(find.byType(AlertDialog), findsNothing);

        // Layar hilang (popped)
        await tester.pumpAndSettle();
        expect(find.byType(IuranPaymentScreen), findsNothing);
      },
    );
  });

  group('IuranPaymentScreen — Save failure', () {
    testWidgets(
      'Simpan pembayaran gagal → dialog tidak muncul, form tetap tampil dengan error',
      (tester) async {
        final repo = _FakeIuranRepository(failCreatePayment: true);
        final bill = _makeBill();

        await tester.pumpWidget(_buildApp(
          repository: repo,
          bill: bill,
        ));
        await tester.pumpAndSettle();

        await tester.tap(find.byIcon(Icons.payments_outlined));
        await tester.pumpAndSettle();

        // Tap Simpan
        await tester.tap(_simpanButton());
        await tester.pumpAndSettle();

        // Error muncul
        await tester.pumpAndSettle(const Duration(seconds: 2));

        // Tidak ada dialog — error inline
        expect(find.byType(AlertDialog), findsNothing);
        expect(find.byType(IuranPaymentScreen), findsOneWidget);

        // Pesan error muncul
        expect(
          find.textContaining('Permintaan gagal diproses'),
          findsOneWidget,
        );
      },
    );

    testWidgets(
      'Setelah pembayaran gagal, pengguna tetap dapat membatalkan form',
      (tester) async {
        final repo = _FakeIuranRepository(failCreatePayment: true);
        final bill = _makeBill();

        await tester.pumpWidget(_buildApp(
          repository: repo,
          bill: bill,
        ));
        await tester.pumpAndSettle();

        await tester.tap(find.byIcon(Icons.payments_outlined));
        await tester.pumpAndSettle();

        // Save fails
        await tester.tap(_simpanButton());
        await tester.pumpAndSettle();
        await tester.pumpAndSettle(const Duration(seconds: 2));
        expect(find.byType(AlertDialog), findsNothing);

        // Batal → dialog muncul
        await tester.tap(find.text('Batal'));
        await tester.pumpAndSettle();

        expect(
          find.text('Apakah Anda yakin ingin membatalkan?'),
          findsOneWidget,
        );
      },
    );
  });
}
