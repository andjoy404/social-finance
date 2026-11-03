/// Iuran repository — error mapping + model mapping (Backend → UI).
///
/// Follows the Warga pattern: repository wraps the API client,
/// converts Backend DTOs to UI models, and maps Dio errors.

import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/api/config.dart';
import 'package:social_finance/core/errors/app_errors.dart';
import 'package:social_finance/features/iuran/data/iuran_api_client.dart';
import 'package:social_finance/features/iuran/data/iuran_api_models.dart';
import 'package:social_finance/features/iuran/data/iuran_models.dart';

class IuranRepository {
  final IuranApiClient apiClient;

  IuranRepository({required this.apiClient});

  // ─── Dues ───────────────────────────────────────────────────────────

  Future<List<BackendDue>> fetchDues({bool? isActive}) async {
    return apiClient.listDues(isActive: isActive);
  }

  /// Create a new due (iuran type). Pengurus only.
  Future<BackendDue> createDue({
    required String name,
    required int amount,
    required DuePeriodType periodType,
    PaymentRule? paymentRule,
    bool? isVoluntary,
    String? validUntil,
    DueType? dueType,
    int? year,
  }) async {
    return apiClient.createDue(
      name: name,
      amount: '$amount.00',
      periodType: periodType,
      paymentRule: paymentRule,
      isVoluntary: isVoluntary,
      validUntil: validUntil,
      dueType: dueType,
      year: year,
    );
  }

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
    return apiClient.updateDue(
      id,
      name: name,
      amount: amount != null ? '$amount.00' : null,
      periodType: periodType,
      paymentRule: paymentRule,
      isVoluntary: isVoluntary,
      isActive: isActive,
      validUntil: validUntil,
    );
  }

  Future<GenerateBillsResult> generateBills({
    required String dueId,
    required String period,
    required String dueDate,
    required String idempotencyKey,
  }) async {
    return apiClient.generateBills(
      dueId: dueId,
      period: period,
      dueDate: dueDate,
      idempotencyKey: idempotencyKey,
    );
  }

  Future<List<DueMonthSummary>> fetchDueMonths(String dueId) async {
    return apiClient.getDueMonths(dueId);
  }

  Future<List<DueMonthSummary>> updateDuePeriodStatus(
    String dueId,
    String period, {
    required bool isActive,
  }) async {
    return apiClient.updateDuePeriodStatus(dueId, period, isActive: isActive);
  }

  /// Fetch paginated bills with computed paidAmount per bill.
  ///
  /// paidAmount is computed from approved payments (NOT a backend field).
  Future<PaginatedResponse<IuranBill>> fetchBills({
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
    final billResponse = await apiClient.listBills(
      search: search,
      dueId: dueId,
      status: status,
      paymentStatus: paymentStatus,
      period: period,
      householdOccupancyId: householdOccupancyId,
      myBills: myBills,
      page: page,
      pageSize: pageSize,
      order: order,
    );

    final uiBills = <IuranBill>[];
    for (final bill in billResponse.data) {
      int approvedTotal = 0;
      if (bill.paidAmount != null) {
        approvedTotal = moneyToDisplayRupiah(bill.paidAmount!);
      }
      int pendingTotal = 0;
      if (bill.pendingAmount != null) {
        pendingTotal = moneyToDisplayRupiah(bill.pendingAmount!);
      }
      const rejectedTotal = 0;

      final paymentNumbers = (bill.paymentNumbers ?? '')
          .split(',')
          .map((s) => s.trim())
          .where((s) => s.isNotEmpty)
          .toList();

      final nominalVal = moneyToDisplayRupiah(bill.amount);
      final isVol = bill.isVoluntary;
      final isActive = bill.dueIsActive ?? true;
      uiBills.add(
        IuranBill(
          id: bill.id,
          householdName: bill.headName ?? '—',
          iuranType: bill.dueName ?? '—',
          periode: formatPeriod(bill.period),
          nominal: nominalVal,
          paidAmount: approvedTotal,
          pendingAmount: pendingTotal,
          rejectedAmount: rejectedTotal,
          status: _mapBillStatus(bill.status, isVoluntary: isVol),
          isVoluntary: isVol,
          requiresFullPayment: !isVol && bill.status == BillStatus.unpaid,
          dueId: bill.dueId,
          dueName: bill.dueName,
          duePeriodType: null,
          dueRtCode: null,
          dueIsVoluntary: isVol,
          dueIsActive: isActive,
          householdOccupancyId: bill.householdOccupancyId,
          dueType: bill.dueType,
          year: bill.year,
          isObligatory: bill.isObligatory,
          moveInDate: bill.moveInDate,
          moveOutDate: bill.moveOutDate,
          paymentNumbers: paymentNumbers,
        ),
      );
    }

    return PaginatedResponse<IuranBill>(
      data: uiBills,
      pagination: billResponse.pagination,
    );
  }

  /// Fetch all bills (no pagination) for client-side filtering.
  Future<List<IuranBill>> fetchAllBills({
    String? search,
    String? status,
    String? paymentStatus,
    bool myBills = false,
  }) async {
    final allBills = <IuranBill>[];
    int currentPage = 1;
    int totalPages = 1;

    do {
      final response = await fetchBills(
        search: search,
        status: status,
        paymentStatus: paymentStatus,
        myBills: myBills,
        page: currentPage,
        pageSize: 100,
      );
      allBills.addAll(response.data);
      totalPages = response.pagination.totalPages;
      currentPage++;
    } while (currentPage <= totalPages);

    // No synthetic voluntary rows — all bills come from the API.
    // Search filter is applied client-side if needed.
    if (search != null && search.trim().isNotEmpty) {
      final query = search.trim().toLowerCase();
      return allBills.where((b) {
        return b.householdName.toLowerCase().contains(query) ||
            b.iuranType.toLowerCase().contains(query) ||
            b.paymentNumbers.any((no) => no.toLowerCase().contains(query));
      }).toList();
    }

    debugPrint('[IuranDebug] fetchAllBills accumulated=${allBills.length}');
    return allBills;
  }

  /// Fetch a single bill detail with computed paidAmount, pendingAmount, and rejectedAmount.
  Future<IuranBill?> fetchBillDetail(String billId) async {
    final bill = await apiClient.getBill(billId);
    final payments = await apiClient.listPayments(billId: bill.id);
    int approvedTotal = payments.data
        .where((p) => p.status == PaymentStatus.approved)
        .fold<int>(0, (sum, p) => sum + moneyToDisplayRupiah(p.amount));
    if (approvedTotal == 0 && bill.paidAmount != null) {
      approvedTotal = moneyToDisplayRupiah(bill.paidAmount!);
    }
    int pendingTotal = payments.data
        .where((p) => p.status == PaymentStatus.pending)
        .fold<int>(0, (sum, p) => sum + moneyToDisplayRupiah(p.amount));
    if (pendingTotal == 0 && bill.pendingAmount != null) {
      pendingTotal = moneyToDisplayRupiah(bill.pendingAmount!);
    }
    final rejectedTotal = payments.data
        .where((p) => p.status == PaymentStatus.rejected)
        .fold<int>(0, (sum, p) => sum + moneyToDisplayRupiah(p.amount));

    final nominalVal = moneyToDisplayRupiah(bill.amount);
    bool isVol = bill.isVoluntary;
    PaymentRule? duePaymentRule;
    try {
      final dues = await apiClient.listDues();
      final matching = dues.where((d) => d.id == bill.dueId).firstOrNull;
      if (matching != null) {
        if (!isVol && matching.isVoluntary) {
          isVol = true;
        }
        duePaymentRule = matching.paymentRule;
      }
    } catch (_) {}

    final bool requiresFull = !isVol &&
        (duePaymentRule != null
            ? duePaymentRule == PaymentRule.full
            : bill.status == BillStatus.unpaid);

    return IuranBill(
      id: bill.id,
      householdName: bill.headName ?? '—',
      iuranType: bill.dueName ?? '—',
      periode: formatPeriod(bill.period),
      nominal: nominalVal,
      paidAmount: approvedTotal,
      pendingAmount: pendingTotal,
      rejectedAmount: rejectedTotal,
      status: _mapBillStatus(bill.status, isVoluntary: isVol),
      isVoluntary: isVol,
      requiresFullPayment: requiresFull,
      dueId: bill.dueId,
      dueName: bill.dueName,
      duePeriodType: null,
      dueRtCode: null,
      dueIsVoluntary: isVol,
      dueIsActive: bill.dueIsActive ?? true,
      householdOccupancyId: bill.householdOccupancyId,
      dueType: bill.dueType,
      year: bill.year,
      isObligatory: bill.isObligatory,
      moveInDate: bill.moveInDate,
      moveOutDate: bill.moveOutDate,
      paymentNumbers: payments.data
          .map((p) => p.paymentNo)
          .whereType<String>()
          .where((no) => no.trim().isNotEmpty)
          .toList(),
    );
  }

  // ─── Payments ───────────────────────────────────────────────────────

  /// Fetch payments for a specific bill.
  Future<List<BackendPayment>> listPaymentsForBill(String billId) async {
    final response = await apiClient.listPayments(billId: billId);
    return response.data;
  }

  /// Fetch payments for a specific due (voluntary donations).
  Future<List<BackendPayment>> listPaymentsForDue(String dueId) async {
    final response = await apiClient.listPayments(dueId: dueId);
    return response.data;
  }

  /// Fetch payments for a bill target. Always queries by billId to prevent leaking.
  Future<List<BackendPayment>> listPaymentsForBillTarget({
    required String billId,
    String? dueId,
    bool isVoluntary = false,
  }) async {
    return listPaymentsForBill(billId);
  }

  Future<BackendPayment> createPayment({
    required String billId,
    required int amount, // rupiah in int (display format)
    required PaymentMethod method,
    String? proofPath,
    String? notes,
    String? idempotencyKey,
    Uint8List? fileBytes,
    String? filename,
  }) async {
    return apiClient.createPayment(
      billId: billId,
      amount: _formatMoney(amount),
      method: method,
      proofPath: proofPath,
      notes: notes,
      idempotencyKey: idempotencyKey,
      fileBytes: fileBytes,
      filename: filename,
    );
  }

  Future<BackendMultiPeriodPaymentResult> createMultiPeriodPayment({
    required List<String> billIds,
    required List<Map<String, String>> allocations,
    required int amount, // total rupiah in int
    required PaymentMethod method,
    String? proofPath,
    String? notes,
    String? idempotencyKey,
    Uint8List? fileBytes,
    String? filename,
  }) async {
    return apiClient.createMultiPeriodPayment(
      billIds: billIds,
      allocations: allocations,
      amount: _formatMoney(amount),
      method: method,
      proofPath: proofPath,
      notes: notes,
      idempotencyKey: idempotencyKey,
      fileBytes: fileBytes,
      filename: filename,
    );
  }

  Future<BackendPayment> createDonation({
    required String dueId,
    required int amount,
    required PaymentMethod method,
    String? notes,
    String? idempotencyKey,
    Uint8List? fileBytes,
    String? filename,
  }) async {
    if (dueId.trim().isEmpty || amount <= 0) {
      throw ArgumentError('Donation requires a due and a positive amount.');
    }
    return apiClient.createDonation(
      dueId: dueId,
      amount: _formatMoney(amount),
      method: method,
      notes: notes,
      idempotencyKey: idempotencyKey,
      fileBytes: fileBytes,
      filename: filename,
    );
  }

  Future<BackendPayment> uploadPaymentProof({
    required String paymentId,
    required List<int> fileBytes,
    required String filename,
  }) async {
    return apiClient.uploadPaymentProof(
      paymentId: paymentId,
      fileBytes: fileBytes,
      filename: filename,
    );
  }

  Future<Uint8List> getPaymentProofBytes(String paymentId) async {
    return apiClient.getPaymentProofBytes(paymentId);
  }

  Future<BackendPayment> verifyPayment(
    String paymentId, {
    required String action,
    String? rejectionReason,
    String? officerNotes,
    String? locationId,
    String? actualReceivedDate,
  }) async {
    return apiClient.verifyPayment(
      paymentId,
      action: action,
      rejectionReason: rejectionReason,
      officerNotes: officerNotes,
      locationId: locationId,
      actualReceivedDate: actualReceivedDate,
    );
  }

  Future<BackendPayment> reversePayment(
    String paymentId, {
    required String reason,
  }) async {
    return apiClient.reversePayment(
      paymentId,
      reason: reason,
    );
  }

  /// Fetch all PENDING payments for verification by bendahara/pengurus.
  Future<List<BackendPayment>> fetchPendingPayments() async {
    final response = await apiClient.listPayments(status: 'PENDING');
    return response.data;
  }

  Future<List<BackendPayment>> fetchPendingDonations(String dueId) async {
    final response = await apiClient.listPayments(
      dueId: dueId,
      status: 'PENDING',
      pageSize: 100,
    );
    return response.data
        .where((payment) => payment.billId == null && payment.dueId == dueId)
        .toList();
  }

  // ─── Reports ────────────────────────────────────────────────────────

  Future<BackendBalance> fetchBalance() async {
    return apiClient.getBalance();
  }

  // ─── Error mapping ──────────────────────────────────────────────────

  ServerException mapDioError(dynamic error) {
    if (error is ServerException) return error;

    if (error is DioException) {
      final status = error.response?.statusCode;
      if (status == 401) {
        return ServerException(
          statusCode: 401,
          message: 'Sesi Anda telah berakhir. Silakan login kembali.',
        );
      } else if (status == 403) {
        return ServerException(
          statusCode: 403,
          message: 'Anda tidak memiliki akses untuk operasi ini.',
        );
      } else if (status == 400) {
        // Try to extract validation message from response body
        final body = error.response?.data;
        if (body is Map && body['error'] is Map) {
          final msg = body['error']['message'] as String?;
          if (msg != null && msg.isNotEmpty) {
            return ServerException(statusCode: 400, message: msg);
          }
        }
        return ServerException(
          statusCode: 400,
          message: 'Permintaan tidak valid. Periksa kembali data yang diisi.',
        );
      } else if (status != null && status >= 500) {
        return ServerException(
          statusCode: status,
          message: 'Terjadi kesalahan pada server. Silakan coba lagi nanti.',
        );
      } else {
        return ServerException(
          statusCode: status,
          message: 'Permintaan gagal diproses (${status ?? "unknown"}).',
        );
      }
    }

    if (error is FormatException) {
      return ServerException(
        message: 'Format respons dari server tidak sesuai.',
      );
    }

    if (error is ApiConfigException) {
      return ServerException(message: error.message);
    }

    return ServerException(message: 'Terjadi kesalahan yang tidak diketahui.');
  }

  // ─── Helpers ────────────────────────────────────────────────────────

  IuranStatus _mapBillStatus(BillStatus status, {bool isVoluntary = false}) {
    if (isVoluntary) {
      return IuranStatus.sukarela;
    }
    switch (status) {
      case BillStatus.unpaid:
        return IuranStatus.belumBayar;
      case BillStatus.partial:
        return IuranStatus.sebagian;
      case BillStatus.paid:
        return IuranStatus.lunas;
      case BillStatus.cancelled:
        return IuranStatus.lunas;
    }
  }

  /// Convert display int (rupiah) to backend money string.
  /// e.g. 50000 → "50000.00"
  String _formatMoney(int amount) {
    return '$amount.00';
  }
}

final iuranRepositoryProvider = Provider<IuranRepository>((ref) {
  final client = ref.watch(apiClientProvider);
  return IuranRepository(apiClient: IuranApiClient(dio: client.dio));
});
