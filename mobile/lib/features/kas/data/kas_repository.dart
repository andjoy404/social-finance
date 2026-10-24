/// KAS repository — error mapping + model mapping (Backend → UI).
///
/// Follows the Warga/Iuran pattern: repository wraps the API client,
/// converts Backend DTOs to UI models, and maps Dio errors.

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/errors/app_errors.dart';
import 'package:social_finance/features/kas/data/kas_api_client.dart';
import 'package:social_finance/features/kas/data/kas_api_models.dart';
import 'package:social_finance/features/kas/data/kas_models.dart';

class KasRepository {
  final KasApiClient apiClient;

  KasRepository({required this.apiClient});

  // ─── RT identity ────────────────────────────────────────────────────

  Future<BackendRTIdentity> fetchRTIdentity(String rtId) {
    return apiClient.getRTIdentity(rtId);
  }

  // ─── Transactions ───────────────────────────────────────────────────

  /// Fetch paginated transactions with optional filters.
  Future<PaginatedResponse<KasTransaction>> listTransactions({
    String? categoryId,
    KasJenis? jenis,
    String? status,
    String? period,
    String? startDate,
    String? endDate,
    int page = 1,
    int pageSize = 20,
  }) async {
    final response = await apiClient.listTransactions(
      categoryId: categoryId,
      type: jenis?.value,
      status: status,
      period: period,
      startDate: startDate,
      endDate: endDate,
      page: page,
      pageSize: pageSize,
    );

    final uiTransactions = <KasTransaction>[];
    for (final tx in response.data) {
      uiTransactions.add(_toUiTransaction(tx));
    }

    return PaginatedResponse<KasTransaction>(
      data: uiTransactions,
      pagination: response.pagination,
    );
  }

  /// Fetch all transactions (no pagination) for client-side filtering.
  Future<List<KasTransaction>> fetchAllTransactions({
    KasJenis? jenis,
    String? status,
    String? period,
  }) async {
    final allTransactions = <KasTransaction>[];
    int currentPage = 1;
    int totalPages = 1;

    do {
      final response = await listTransactions(
        jenis: jenis,
        status: status,
        period: period,
        page: currentPage,
        pageSize: 100,
      );
      allTransactions.addAll(response.data);
      totalPages = response.pagination.totalPages;
      currentPage++;
    } while (currentPage <= totalPages);

    // Sort newest-first by transaction date (occurred_at), with deterministic
    // tie-breaker by created_at for transactions sharing the same timestamp.
    // Backend already sorts by ORDER BY occurred_at DESC, created_at DESC;
    // this client-side sort ensures correct newest-first order regardless of
    // backend behavior and across paginated results.
    // createdAt is preferred over id (UUID) as tie-breaker because id is random
    // and does not reflect creation order.
    allTransactions.sort((a, b) {
      final dateCompare = b.tanggal.compareTo(a.tanggal);
      if (dateCompare != 0) return dateCompare;
      final aCreated = a.createdAt ?? DateTime.fromMillisecondsSinceEpoch(0);
      final bCreated = b.createdAt ?? DateTime.fromMillisecondsSinceEpoch(0);
      return bCreated.compareTo(aCreated);
    });

    return allTransactions;
  }

  /// Fetch ALL backend transactions (every page, 100 per page = backend cap)
  /// without UI-model mapping, so report code keeps category name, payment
  /// method and status exactly as the API returned them.
  Future<List<BackendTransaction>> fetchAllBackendTransactions({
    String? status,
    String? period,
  }) async {
    final all = <BackendTransaction>[];
    var page = 1;
    var totalPages = 1;
    do {
      final response = await apiClient.listTransactions(
        status: status,
        period: period,
        page: page,
        pageSize: 100,
      );
      all.addAll(response.data);
      totalPages = response.pagination.totalPages;
      page++;
    } while (page <= totalPages);
    return all;
  }

  /// Fetch a single transaction detail by ID.
  Future<KasTransaction> getTransaction(String id) async {
    final backendTx = await apiClient.getTransaction(id);
    return _toUiTransaction(backendTx);
  }

  /// Create a new KAS transaction (income or expense).
  ///
  /// Uses idempotency key to prevent duplicate submissions.
  Future<KasTransaction> createTransaction({
    String? categoryId,
    required int amount,
    required KasJenis jenis,
    required String description,
    DateTime? occurredAt,
    String? paymentMethod,
    required String idempotencyKey,
  }) async {
    final backendTx = await apiClient.createTransaction(
      categoryId: categoryId,
      amount: _formatMoney(amount),
      type: jenis == KasJenis.keluar ? 'expense' : 'income',
      description: description,
      occurredAt: occurredAt,
      paymentMethod: paymentMethod,
      idempotencyKey: idempotencyKey,
    );
    return _toUiTransaction(backendTx);
  }

  // ─── Balance ────────────────────────────────────────────────────────

  /// Fetch current balance summary from the reports endpoint.
  Future<BackendBalance> fetchBalance({String? period}) async {
    return apiClient.getBalance(period: period);
  }

  // ─── Categories ─────────────────────────────────────────────────────

  /// Fetch all active KAS categories (income/expense) from backend.
  Future<List<BackendCategory>> fetchCategories({bool? isActive}) async {
    return apiClient.listCategories(isActive: isActive);
  }

  // ─── Model Mapping ──────────────────────────────────────────────────

  /// Convert BackendTransaction to KasTransaction (UI model).
  KasTransaction _toUiTransaction(BackendTransaction bt) {
    // Map backend type ('income'/'expense') to KasJenis (masuk/keluar)
    final jenis = bt.type == 'expense' ? KasJenis.keluar : KasJenis.masuk;

    // Use categoryName from backend if available, otherwise derive from category
    final kategori = _resolveKategoriFromBackend(
      bt.categoryName ?? bt.categoryId,
    );

    final ref =
        bt.paymentNo ??
        (bt.paymentId != null && bt.paymentId!.isNotEmpty
            ? (bt.paymentId!.length >= 8
                  ? 'PAY-${bt.paymentId!.substring(0, 8)}'
                  : bt.paymentId)
            : (bt.reversesTransactionId != null ? 'Reversal' : null));

    return KasTransaction(
      id: bt.id,
      tanggal: bt.occurredAt != null
          ? DateTime.tryParse(bt.occurredAt!) ?? DateTime.now()
          : DateTime.now(),
      jenis: jenis,
      kategori: kategori,
      keterangan: bt.description,
      referensi: ref,
      dueName: bt.dueName,
      citizenName: bt.citizenName,
      officerName: bt.officerName,
      houseNumber: bt.houseNumber,
      paymentMethod: bt.paymentMethod,
      createdAt: bt.createdAt != null ? DateTime.tryParse(bt.createdAt!) : null,
      nominal: bt.amount,
      saldo: '0', // Balance is computed server-side, not per-transaction
    );
  }

  /// Resolve KasCategory from backend category name or ID.
  KasCategory _resolveKategoriFromBackend(String name) {
    // Normalize name for matching
    final normalized = name.trim().toLowerCase();

    if (normalized.contains('operasional')) {
      return KasCategory.operasionalRT;
    }
    if (normalized.contains('iuran warga') ||
        normalized.contains('iuranwarga')) {
      return KasCategory.iuranWarga;
    }
    if (normalized.contains('iuran paksa') ||
        normalized.contains('iuranpaksa')) {
      return KasCategory.iuranPaksa;
    }
    if (normalized.contains('perlengkapan')) {
      return KasCategory.perlengkapan;
    }
    if (normalized.contains('konsumsi')) {
      return KasCategory.konsumsi;
    }
    if (normalized.contains('perbaikan')) {
      return KasCategory.perbaikan;
    }
    if (normalized.contains('kebersihan')) {
      return KasCategory.kebersihan;
    }
    if (normalized.contains('keamanan')) {
      return KasCategory.keamanan;
    }
    return KasCategory.lainLain;
  }

  /// Update an existing manual transaction.
  Future<KasTransaction> updateTransaction({
    required String id,
    String? categoryId,
    int? amount,
    KasJenis? jenis,
    String? description,
    DateTime? occurredAt,
    String? paymentMethod,
  }) async {
    final backendTx = await apiClient.updateTransaction(
      id: id,
      categoryId: categoryId,
      amount: amount != null ? _formatMoney(amount) : null,
      type: jenis != null
          ? (jenis == KasJenis.keluar ? 'expense' : 'income')
          : null,
      description: description,
      occurredAt: occurredAt,
      paymentMethod: paymentMethod,
    );
    return _toUiTransaction(backendTx);
  }

  /// Update opening balance for a period.
  Future<Map<String, dynamic>> updateOpeningBalance({
    required String period,
    required String openingBalance,
    String? reason,
  }) async {
    return apiClient.updateOpeningBalance(
      period: period,
      openingBalance: openingBalance,
      reason: reason,
    );
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
      } else if (status == 409) {
        return ServerException(
          statusCode: 409,
          message: 'Transaksi dengan kunci idempotensi yang sama sudah ada.',
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

    return ServerException(message: 'Terjadi kesalahan yang tidak diketahui.');
  }

  // ─── Helpers ────────────────────────────────────────────────────────

  /// Convert display int (rupiah) to backend money string.
  /// e.g. 50000 → "50000.00"
  String _formatMoney(int amount) {
    return '$amount.00';
  }
}

final kasApiClientProvider = Provider<KasApiClient>((ref) {
  final client = ref.watch(apiClientProvider);
  return KasApiClient(dio: client.dio);
});

final kasRepositoryProvider = Provider<KasRepository>((ref) {
  final apiClient = ref.watch(kasApiClientProvider);
  return KasRepository(apiClient: apiClient);
});
