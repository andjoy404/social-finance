// Save-success & save-failure flow tests for IuranFormScreen.

import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/navigators/kas_form_navigation_guard.dart';
import 'package:social_finance/core/widgets/app_text_field.dart';
import 'package:social_finance/features/iuran/data/iuran_api_client.dart';
import 'package:social_finance/features/iuran/data/iuran_api_models.dart';
import 'package:social_finance/features/iuran/data/iuran_models.dart';
import 'package:social_finance/features/iuran/data/iuran_repository.dart';
import 'package:social_finance/features/iuran/presentation/screens/iuran_form_screen.dart';

// ─── Fake Repository (success) ───────────────────────────────────────────

class _SuccessIuranRepository extends IuranRepository {
  _SuccessIuranRepository() : super(apiClient: _FakeIuranApiClient());

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
      id: 'due-created-success',
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
  Future<BackendDue> updateDue(String id, {String? name, String? amount, DuePeriodType? periodType, PaymentRule? paymentRule, bool? isVoluntary, bool? isActive, String? validUntil}) async => throw UnimplementedError();

  @override
  Future<void> deactivateDue(String id) async {}

  @override
  Future<List<DueMonthSummary>> getDueMonths(String dueId) async => [];

  @override
  Future<List<DueMonthSummary>> updateDuePeriodStatus(String dueId, String period, {required bool isActive}) async => [];

  @override
  Future<PaginatedResponse<BackendBill>> listBills({String? search, String? dueId, String? status, String? paymentStatus, String? period, String? householdOccupancyId, bool myBills = false, int page = 1, int pageSize = 30, String? order}) async {
    return const PaginatedResponse<BackendBill>(data: [], pagination: PaginationInfo(page: 1, pageSize: 30, total: 0, totalPages: 0));
  }

  @override
  Future<BackendBill> getBill(String id) async => throw UnimplementedError();

  @override
  Future<GenerateBillsResult> generateBills({required String dueId, required String period, required String dueDate, required String idempotencyKey}) async => const GenerateBillsResult(generatedCount: 1, bills: []);

  @override
  GenerateBillsResult parseGenerateBillsResponse(dynamic data) => throw UnimplementedError();

  @override
  Future<void> cancelBill(String id) async {}

  @override
  Future<PaginatedResponse<BackendPayment>> listPayments({String? billId, String? dueId, String? status, int page = 1, int pageSize = 20}) async {
    return const PaginatedResponse<BackendPayment>(data: [], pagination: PaginationInfo(page: 1, pageSize: 20, total: 0, totalPages: 0));
  }

  @override
  Future<BackendPayment> getPayment(String id) async => throw UnimplementedError();

  @override
  Future<BackendPayment> createPayment({required String billId, required String amount, required PaymentMethod method, String? proofPath, String? notes, String? idempotencyKey, Uint8List? fileBytes, String? filename}) async => throw UnimplementedError();

  @override
  Future<BackendPayment> createDonation({required String dueId, required String amount, required PaymentMethod method, String? notes, String? idempotencyKey, Uint8List? fileBytes, String? filename}) async => throw UnimplementedError();

  @override
  Future<BackendPayment> uploadPaymentProof({required String paymentId, required List<int> fileBytes, required String filename}) async => throw UnimplementedError();

  @override
  Future<Uint8List> getPaymentProofBytes(String paymentId) async => throw UnimplementedError();

  @override
  Future<BackendPayment> verifyPayment(String paymentId, {required String action, String? rejectionReason, String? officerNotes, String? locationId, String? actualReceivedDate}) async => throw UnimplementedError();

  @override
  Future<BackendPayment> reversePayment(String paymentId, {required String reason}) async => throw UnimplementedError();

  @override
  Future<List<BackendPayment>> listPaymentsForBill(String billId) async => [];

  @override
  Future<List<BackendPayment>> listPaymentsForDue(String dueId) async => [];

  @override
  Future<List<BackendPayment>> listPaymentsForBillTarget({required String billId, String? dueId, bool isVoluntary = false}) async => [];

  @override
  Future<List<BackendDue>> listDues({bool? isActive}) async => [];

  @override
  Future<BackendBalance> getBalance({String? period, String? startDate, String? endDate}) async => throw UnimplementedError();

  @override
  Future<Map<String, dynamic>> updateOpeningBalance({required String period, required String openingBalance, String? reason}) async => {};

  @override
  Future<Map<String, dynamic>> getDashboardSummary([String? period]) async => {};

  @override
  Future<List<BackendCategory>> listCategories({bool? isActive}) async => [];

  @override
  Future<BackendMultiPeriodPaymentResult> createMultiPeriodPayment({required List<String> billIds, required List<Map<String, String>> allocations, required String amount, required PaymentMethod method, String? notes, String? proofPath, String? idempotencyKey, Uint8List? fileBytes, String? filename}) async => throw UnimplementedError();
}

// ─── Fake Repository (failure) ───────────────────────────────────────────

class _FailureIuranRepository extends IuranRepository {
  _FailureIuranRepository() : super(apiClient: _FailingIuranApiClient());

  @override
  Future<List<BackendDue>> fetchDues({bool? isActive}) async => [];

  @override
  Future<List<IuranBill>> fetchAllBills({String? search, String? status, String? paymentStatus, bool myBills = false}) async => [];
}

class _FailingIuranApiClient implements IuranApiClient {
  @override
  Dio get dio => Dio(BaseOptions(baseUrl: 'http://test.local'));

  @override
  Future<BackendDue> createDue({required String name, required String amount, required DuePeriodType periodType, PaymentRule? paymentRule, bool? isVoluntary, String? validUntil, DueType? dueType, int? year}) async {
    throw DioException(
      requestOptions: RequestOptions(path: '/api/v1/dues'),
      response: Response<dynamic>(statusCode: 500, requestOptions: RequestOptions(path: '/api/v1/dues')),
    );
  }

  @override
  Future<BackendDue> updateDue(String id, {String? name, String? amount, DuePeriodType? periodType, PaymentRule? paymentRule, bool? isVoluntary, bool? isActive, String? validUntil}) async => throw UnimplementedError();

  @override
  Future<void> deactivateDue(String id) async {}

  @override
  Future<List<DueMonthSummary>> getDueMonths(String dueId) async => [];

  @override
  Future<List<DueMonthSummary>> updateDuePeriodStatus(String dueId, String period, {required bool isActive}) async => [];

  @override
  Future<PaginatedResponse<BackendBill>> listBills({String? search, String? dueId, String? status, String? paymentStatus, String? period, String? householdOccupancyId, bool myBills = false, int page = 1, int pageSize = 30, String? order}) async {
    return const PaginatedResponse<BackendBill>(data: [], pagination: PaginationInfo(page: 1, pageSize: 30, total: 0, totalPages: 0));
  }

  @override
  Future<BackendBill> getBill(String id) async => throw UnimplementedError();

  @override
  Future<GenerateBillsResult> generateBills({required String dueId, required String period, required String dueDate, required String idempotencyKey}) async => throw UnimplementedError();

  @override
  GenerateBillsResult parseGenerateBillsResponse(dynamic data) => throw UnimplementedError();

  @override
  Future<void> cancelBill(String id) async {}

  @override
  Future<PaginatedResponse<BackendPayment>> listPayments({String? billId, String? dueId, String? status, int page = 1, int pageSize = 20}) async {
    return const PaginatedResponse<BackendPayment>(data: [], pagination: PaginationInfo(page: 1, pageSize: 20, total: 0, totalPages: 0));
  }

  @override
  Future<BackendPayment> getPayment(String id) async => throw UnimplementedError();

  @override
  Future<BackendPayment> createPayment({required String billId, required String amount, required PaymentMethod method, String? proofPath, String? notes, String? idempotencyKey, Uint8List? fileBytes, String? filename}) async => throw UnimplementedError();

  @override
  Future<BackendPayment> createDonation({required String dueId, required String amount, required PaymentMethod method, String? notes, String? idempotencyKey, Uint8List? fileBytes, String? filename}) async => throw UnimplementedError();

  @override
  Future<BackendPayment> uploadPaymentProof({required String paymentId, required List<int> fileBytes, required String filename}) async => throw UnimplementedError();

  @override
  Future<Uint8List> getPaymentProofBytes(String paymentId) async => throw UnimplementedError();

  @override
  Future<BackendPayment> verifyPayment(String paymentId, {required String action, String? rejectionReason, String? officerNotes, String? locationId, String? actualReceivedDate}) async => throw UnimplementedError();

  @override
  Future<BackendPayment> reversePayment(String paymentId, {required String reason}) async => throw UnimplementedError();

  @override
  Future<List<BackendPayment>> listPaymentsForBill(String billId) async => [];

  @override
  Future<List<BackendPayment>> listPaymentsForDue(String dueId) async => [];

  @override
  Future<List<BackendPayment>> listPaymentsForBillTarget({required String billId, String? dueId, bool isVoluntary = false}) async => [];

  @override
  Future<List<BackendDue>> listDues({bool? isActive}) async => [];

  @override
  Future<BackendBalance> getBalance({String? period, String? startDate, String? endDate}) async => throw UnimplementedError();

  @override
  Future<Map<String, dynamic>> updateOpeningBalance({required String period, required String openingBalance, String? reason}) async => {};

  @override
  Future<Map<String, dynamic>> getDashboardSummary([String? period]) async => {};

  @override
  Future<List<BackendCategory>> listCategories({bool? isActive}) async => [];

  @override
  Future<BackendMultiPeriodPaymentResult> createMultiPeriodPayment({required List<String> billIds, required List<Map<String, String>> allocations, required String amount, required PaymentMethod method, String? notes, String? proofPath, String? idempotencyKey, Uint8List? fileBytes, String? filename}) async => throw UnimplementedError();
}

// ─── Helpers ─────────────────────────────────────────────────────────────

GlobalKey<NavigatorState> _createNavigatorKey() => GlobalKey<NavigatorState>();

Widget _buildApp(IuranRepository repository, GlobalKey<NavigatorState> navigatorKey) {
  return ProviderScope(
    key: ValueKey('iuran-form-save-test-${DateTime.now().millisecondsSinceEpoch}'),
    overrides: [iuranRepositoryProvider.overrideWithValue(repository)],
    child: MaterialApp.router(
      routerConfig: GoRouter(
        initialLocation: '/home/iuran/form',
        navigatorKey: navigatorKey,
        routes: [
          GoRoute(
            path: '/home/iuran/form',
            pageBuilder: (context, state) => MaterialPage(child: IuranFormScreen()),
          ),
        ],
      ),
    ),
  );
}

Finder _simpanButton() => find.byKey(const ValueKey('iuran-save-button'));
Finder _batalButton() => find.text('Batal');

// ─── Tests ───────────────────────────────────────────────────────────────

void main() {
  setUp(() {
    KasFormNavigationGuard.resetForTesting();
  });

  group('IuranFormScreen — Save success', () {
    testWidgets('Simpan berhasil → tidak muncul dialog, navigasi keluar', (tester) async {
      final navigatorKey = _createNavigatorKey();
      final repo = _SuccessIuranRepository();
      await tester.pumpWidget(_buildApp(repo, navigatorKey));
      await tester.pumpAndSettle();

      expect(find.byType(IuranFormScreen), findsOneWidget);

      // Fill the form fields using descendant finders
      // Field 0: Nama Iuran (AppTextField)
      final nameTF = find.descendant(
        of: find.byWidgetPredicate((w) => w is AppTextField && w.label == 'Nama Iuran'),
        matching: find.byType(TextField),
      );
      if (nameTF.evaluate().isNotEmpty) {
        await tester.enterText(nameTF, 'Iuran Sampah');
        await tester.pumpAndSettle();
      }

      // Field 1: Nominal (plain TextField with prefixText 'Rp ')
      final nominalTF = find.byWidgetPredicate(
        (w) => w is TextField && w.decoration?.prefixText == 'Rp ',
      );
      if (nominalTF.evaluate().isNotEmpty) {
        await tester.enterText(nominalTF, '25000');
        await tester.pumpAndSettle();
      }

      // Field 2: Tanggal Jatuh Tempo (AppTextField)
      final dueDateTF = find.descendant(
        of: find.byWidgetPredicate((w) => w is AppTextField && w.label == 'Tanggal Jatuh Tempo'),
        matching: find.byType(TextField),
      );
      if (dueDateTF.evaluate().isNotEmpty) {
        await tester.enterText(dueDateTF, '2026-12-10');
        await tester.pumpAndSettle();
      }

      // Scroll to bottom
      await tester.dragFrom(const Offset(400, 300), const Offset(0, -500));
      await tester.pumpAndSettle();

      expect(_simpanButton(), findsOneWidget);
      await tester.tap(_simpanButton());
      await tester.pumpAndSettle();

      // Spinner muncul
      expect(find.byType(CircularProgressIndicator), findsWidgets);
      await tester.pumpAndSettle(const Duration(seconds: 2));

      // Snackbar muncul — tidak ada dialog
      expect(find.byType(SnackBar), findsOneWidget);
      expect(find.byType(AlertDialog), findsNothing);

      await tester.pumpAndSettle();
      expect(find.byType(IuranFormScreen), findsNothing);
    });
  });

  group('IuranFormScreen — Save failure', () {
    testWidgets('Simpan gagal → dialog tidak muncul, form tetap tampil', (tester) async {
      final navigatorKey = _createNavigatorKey();
      final repo = _FailureIuranRepository();
      await tester.pumpWidget(_buildApp(repo, navigatorKey));
      await tester.pumpAndSettle();

      // Fill fields
      final nameTF = find.descendant(
        of: find.byWidgetPredicate((w) => w is AppTextField && w.label == 'Nama Iuran'),
        matching: find.byType(TextField),
      );
      if (nameTF.evaluate().isNotEmpty) {
        await tester.enterText(nameTF, 'Iuran Sampah');
        await tester.pumpAndSettle();
      }

      final nominalTF = find.byWidgetPredicate(
        (w) => w is TextField && w.decoration?.prefixText == 'Rp ',
      );
      if (nominalTF.evaluate().isNotEmpty) {
        await tester.enterText(nominalTF, '25000');
        await tester.pumpAndSettle();
      }

      final dueDateTF = find.descendant(
        of: find.byWidgetPredicate((w) => w is AppTextField && w.label == 'Tanggal Jatuh Tempo'),
        matching: find.byType(TextField),
      );
      if (dueDateTF.evaluate().isNotEmpty) {
        await tester.enterText(dueDateTF, '2026-12-10');
        await tester.pumpAndSettle();
      }

      await tester.dragFrom(const Offset(400, 300), const Offset(0, -500));
      await tester.pumpAndSettle();

      await tester.tap(_simpanButton());
      await tester.pumpAndSettle();

      await tester.pumpAndSettle(const Duration(seconds: 2));

      expect(find.byType(AlertDialog), findsNothing);
      expect(find.byType(IuranFormScreen), findsOneWidget);
      expect(find.textContaining('Permintaan gagal diproses'), findsOneWidget);
    });

    testWidgets('Setelah save gagal, Batal menampilkan dialog', (tester) async {
      final navigatorKey = _createNavigatorKey();
      final repo = _FailureIuranRepository();
      await tester.pumpWidget(_buildApp(repo, navigatorKey));
      await tester.pumpAndSettle();

      // Fill fields
      final nameTF = find.descendant(
        of: find.byWidgetPredicate((w) => w is AppTextField && w.label == 'Nama Iuran'),
        matching: find.byType(TextField),
      );
      if (nameTF.evaluate().isNotEmpty) {
        await tester.enterText(nameTF, 'Iuran Sampah');
        await tester.pumpAndSettle();
      }

      final nominalTF = find.byWidgetPredicate(
        (w) => w is TextField && w.decoration?.prefixText == 'Rp ',
      );
      if (nominalTF.evaluate().isNotEmpty) {
        await tester.enterText(nominalTF, '25000');
        await tester.pumpAndSettle();
      }

      // Save fails
      await tester.dragFrom(const Offset(400, 300), const Offset(0, -500));
      await tester.pumpAndSettle();
      await tester.tap(_simpanButton());
      await tester.pumpAndSettle();
      await tester.pumpAndSettle(const Duration(seconds: 2));
      expect(find.byType(AlertDialog), findsNothing);

      await tester.tap(_batalButton());
      await tester.pumpAndSettle();

      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsOneWidget);
    });
  });
}
