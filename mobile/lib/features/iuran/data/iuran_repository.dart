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
  }) async {
    return apiClient.createDue(
      name: name,
      amount: '$amount.00',
      periodType: periodType,
      paymentRule: paymentRule,
      isVoluntary: isVoluntary,
      validUntil: validUntil,
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
    int pageSize = 20,
  }) async {
    final billResponse = await apiClient.listBills(
      dueId: dueId,
      status: status,
      paymentStatus: paymentStatus,
      period: period,
      householdOccupancyId: householdOccupancyId,
      myBills: myBills,
      page: page,
      pageSize: pageSize,
    );

    // DEBUG: log API response counts per household
    final backendHouseholds = <String, int>{};
    for (final bill in billResponse.data) {
      backendHouseholds.update(
        bill.headName ?? '—',
        (c) => c + 1,
        ifAbsent: () => 1,
      );
    }
    debugPrint(
      '[IuranDebug] fetchBills page=$page — total=${billResponse.data.length}, pagination=${billResponse.pagination.totalPages} pages',
    );
    debugPrint(
      '[IuranDebug]   Backend households: ${backendHouseholds.entries.map((e) => '${e.key}=${e.value}').join(', ')}',
    );

    final uiBills = <IuranBill>[];
    for (final bill in billResponse.data) {
      PaginatedResponse<BackendPayment> payments = const PaginatedResponse(
        data: [],
        pagination: PaginationInfo(
          page: 1,
          pageSize: 20,
          total: 0,
          totalPages: 0,
        ),
      );
      try {
        payments = await apiClient.listPayments(billId: bill.id);
      } catch (e) {
        debugPrint('[IuranDebug] listPayments failed for bill=${bill.id}: $e');
      }
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
      final isVol = bill.isVoluntary;
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
            b.iuranType.toLowerCase().contains(query);
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
  }) async {
    return apiClient.createPayment(
      billId: billId,
      amount: _formatMoney(amount),
      method: method,
      proofPath: proofPath,
      notes: notes,
      idempotencyKey: idempotencyKey,
    );
  }

  Future<BackendPayment> createDonation({
    required String dueId,
    required int amount,
    required PaymentMethod method,
    String? notes,
    String? idempotencyKey,
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
  }) async {
    return apiClient.verifyPayment(
      paymentId,
      action: action,
      rejectionReason: rejectionReason,
      officerNotes: officerNotes,
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
