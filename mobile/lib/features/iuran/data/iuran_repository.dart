/// Iuran repository — error mapping + model mapping (Backend → UI).
///
/// Follows the Warga pattern: repository wraps the API client,
/// converts Backend DTOs to UI models, and maps Dio errors.

import 'package:dio/dio.dart';
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

  // ─── Bills (with paidAmount computation) ────────────────────────────

  /// Fetch paginated bills with computed paidAmount per bill.
  ///
  /// paidAmount is computed from approved payments (NOT a backend field).
  Future<PaginatedResponse<IuranBill>> fetchBills({
    String? search,
    String? dueId,
    String? status,
    String? period,
    String? householdOccupancyId,
    int page = 1,
    int pageSize = 20,
  }) async {
    final billResponse = await apiClient.listBills(
      dueId: dueId,
      status: status,
      period: period,
      householdOccupancyId: householdOccupancyId,
      page: page,
      pageSize: pageSize,
    );

    final uiBills = <IuranBill>[];
    for (final bill in billResponse.data) {
      final payments = await apiClient.listPayments(billId: bill.id);
      final approvedTotal = payments.data
          .where((p) => p.status == PaymentStatus.approved)
          .fold<int>(
            0,
            (sum, p) => sum + moneyToDisplayRupiah(p.amount),
          );

      uiBills.add(IuranBill(
        id: bill.id,
        householdName: bill.headName ?? '—',
        iuranType: bill.dueName ?? '—',
        periode: formatPeriod(bill.period),
        nominal: moneyToDisplayRupiah(bill.amount),
        paidAmount: approvedTotal,
        status: _mapBillStatus(bill.status),
      ));
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
  }) async {
    final allBills = <IuranBill>[];
    int currentPage = 1;
    int totalPages = 1;

    do {
      final response = await fetchBills(
        status: status,
        page: currentPage,
        pageSize: 100,
      );
      allBills.addAll(response.data);
      totalPages = response.pagination.totalPages;
      currentPage++;
    } while (currentPage <= totalPages);

    // Apply search filter client-side if needed
    if (search != null && search.trim().isNotEmpty) {
      final query = search.trim().toLowerCase();
      return allBills.where((b) {
        return b.householdName.toLowerCase().contains(query) ||
            b.iuranType.toLowerCase().contains(query);
      }).toList();
    }

    return allBills;
  }

  /// Fetch a single bill detail with computed paidAmount.
  Future<IuranBill?> fetchBillDetail(String billId) async {
    final bill = await apiClient.getBill(billId);
    final payments = await apiClient.listPayments(billId: bill.id);
    final approvedTotal = payments.data
        .where((p) => p.status == PaymentStatus.approved)
        .fold<int>(
          0,
          (sum, p) => sum + moneyToDisplayRupiah(p.amount),
        );

    return IuranBill(
      id: bill.id,
      householdName: bill.headName ?? '—',
      iuranType: bill.dueName ?? '—',
      periode: formatPeriod(bill.period),
      nominal: moneyToDisplayRupiah(bill.amount),
      paidAmount: approvedTotal,
      status: _mapBillStatus(bill.status),
    );
  }

  // ─── Payments ───────────────────────────────────────────────────────

  /// Fetch payments for a specific bill.
  Future<List<BackendPayment>> listPaymentsForBill(String billId) async {
    final response = await apiClient.listPayments(billId: billId);
    return response.data;
  }

  Future<BackendPayment> createPayment({
    required String billId,
    required int amount, // rupiah in int (display format)
    required PaymentMethod method,
    String? notes,
  }) async {
    return apiClient.createPayment(
      billId: billId,
      amount: _formatMoney(amount),
      method: method,
      notes: notes,
    );
  }

  Future<BackendPayment> verifyPayment(
    String paymentId, {
    required String action,
    String? rejectionReason,
  }) async {
    return apiClient.verifyPayment(
      paymentId,
      action: action,
      rejectionReason: rejectionReason,
    );
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
      return ServerException(message: 'Format respons dari server tidak sesuai.');
    }

    if (error is ApiConfigException) {
      return ServerException(message: error.message);
    }

    return ServerException(message: 'Terjadi kesalahan yang tidak diketahui.');
  }

  // ─── Helpers ────────────────────────────────────────────────────────

  IuranStatus _mapBillStatus(BillStatus status) {
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
    return '${amount}.00';
  }
}

final iuranRepositoryProvider = Provider<IuranRepository>((ref) {
  final client = ref.watch(apiClientProvider);
  return IuranRepository(apiClient: IuranApiClient(dio: client.dio));
});
