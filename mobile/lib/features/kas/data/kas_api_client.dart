/// KAS API client — thin Dio wrapper for KAS/finance transaction endpoints.
///
/// Uses the shared `ApiClient` (Dio) from `client.dart` for HTTP transport,
/// auth token injection, and single-flight refresh token handling.
/// No duplication of auth/interceptor behavior.

import 'dart:math';

import 'package:dio/dio.dart';

import 'package:social_finance/features/kas/data/kas_api_models.dart';

/// Generates a v4-style random UUID string.
/// Uses `dart:math` only — no external uuid package in pubspec.yaml.
String _generateUuid() {
  final r = Random();
  String hex(int count) =>
      List.generate(count, (_) => '0123456789abcdef'[r.nextInt(16)]).join();

  return '${hex(8)}-${hex(4)}-4${hex(3)}-'
      '${'89ab'[r.nextInt(4)]}${hex(3)}-'
      '${hex(12)}';
}

/// Create a new idempotency key for transaction POST.
String newTransactionIdempotencyKey() => _generateUuid();

class KasApiClient {
  final Dio dio;

  KasApiClient({required this.dio});

  // ─── RT identity ────────────────────────────────────────────────────

  /// GET /api/v1/rts/{id}; uses the authenticated RT context.
  Future<BackendRTIdentity> getRTIdentity(String rtId) async {
    final response = await dio.get('/api/v1/rts/$rtId');
    final data = response.data;
    if (data is! Map) throw const FormatException('Unexpected RT response');
    return BackendRTIdentity.fromJson(data.cast<String, dynamic>());
  }

  // ─── Transactions ───────────────────────────────────────────────────

  /// GET /api/v1/transactions
  Future<PaginatedResponse<BackendTransaction>> listTransactions({
    String? categoryId,
    String? type,
    String? status,
    String? period,
    String? startDate,
    String? endDate,
    int page = 1,
    int pageSize = 20,
  }) async {
    final query = <String, dynamic>{
      'page': page.toString(),
      'page_size': pageSize.toString(),
    };
    if (categoryId != null && categoryId.isNotEmpty) {
      query['category_id'] = categoryId;
    }
    if (type != null && type.isNotEmpty) {
      query['type'] = type;
    }
    if (status != null && status.isNotEmpty) {
      query['status'] = status;
    }
    if (period != null && period.isNotEmpty) {
      query['period'] = period;
    }
    if (startDate != null && startDate.isNotEmpty) {
      query['start_date'] = startDate;
    }
    if (endDate != null && endDate.isNotEmpty) {
      query['end_date'] = endDate;
    }

    final response = await dio.get(
      '/api/v1/transactions',
      queryParameters: query,
    );
    return _parsePaginatedTransactions(response.data);
  }

  /// GET /api/v1/transactions/{id}
  Future<BackendTransaction> getTransaction(String id) async {
    final response = await dio.get('/api/v1/transactions/$id');
    final data = response.data;
    if (data is! Map) throw const FormatException('Unexpected response');
    return BackendTransaction.fromJson(data.cast<String, dynamic>());
  }

  /// POST /api/v1/transactions — with Idempotency-Key header
  Future<BackendTransaction> createTransaction({
    String? categoryId,
    required String amount,
    required String type,
    required String description,
    DateTime? occurredAt,
    String? paymentMethod,
    String? locationId,
    required String idempotencyKey,
  }) async {
    final Map<String, dynamic> body = {
      'amount': amount,
      'type': type,
      'description': description,
    };
    if (categoryId != null && categoryId.isNotEmpty) {
      body['category_id'] = categoryId;
    }
    if (locationId != null && locationId.isNotEmpty) {
      body['location_id'] = locationId;
    }
    if (paymentMethod != null && paymentMethod.isNotEmpty) {
      body['payment_method'] = paymentMethod;
    }
    if (occurredAt != null) {
      body['occurred_at'] = occurredAt.toIso8601String();
    }

    final response = await dio.post(
      '/api/v1/transactions',
      data: body,
      options: Options(headers: {'Idempotency-Key': idempotencyKey}),
    );

    final data = response.data;
    if (data is! Map) throw const FormatException('Unexpected response');
    return BackendTransaction.fromJson(data.cast<String, dynamic>());
  }

  /// PUT /api/v1/transactions/{id}
  Future<BackendTransaction> updateTransaction({
    required String id,
    String? categoryId,
    String? amount,
    String? type,
    String? description,
    DateTime? occurredAt,
    String? paymentMethod,
    String? locationId,
  }) async {
    final Map<String, dynamic> body = {};
    if (categoryId != null && categoryId.isNotEmpty) {
      body['category_id'] = categoryId;
    }
    if (locationId != null && locationId.isNotEmpty) {
      body['location_id'] = locationId;
    }
    if (amount != null && amount.isNotEmpty) {
      body['amount'] = amount;
    }
    if (type != null && type.isNotEmpty) {
      body['type'] = type;
    }
    if (description != null && description.isNotEmpty) {
      body['description'] = description;
    }
    if (paymentMethod != null && paymentMethod.isNotEmpty) {
      body['payment_method'] = paymentMethod;
    }
    if (occurredAt != null) {
      body['occurred_at'] = occurredAt.toIso8601String();
    }

    final response = await dio.put('/api/v1/transactions/$id', data: body);
    final data = response.data;
    if (data is! Map) throw const FormatException('Unexpected response');
    return BackendTransaction.fromJson(data.cast<String, dynamic>());
  }

  // ─── Balance ────────────────────────────────────────────────────────

  /// GET /api/v1/reports/balance
  Future<BackendBalance> getBalance({
    String? period,
    String? startDate,
    String? endDate,
  }) async {
    final query = <String, dynamic>{};
    if (period != null && period.isNotEmpty) {
      query['period'] = period;
    }
    if (startDate != null && startDate.isNotEmpty) {
      query['start_date'] = startDate;
    }
    if (endDate != null && endDate.isNotEmpty) {
      query['end_date'] = endDate;
    }
    final response = await dio.get(
      '/api/v1/reports/balance',
      queryParameters: query,
    );
    final data = response.data;
    if (data is! Map) throw const FormatException('Unexpected response');
    return BackendBalance.fromJson(data.cast<String, dynamic>());
  }

  /// PUT /api/v1/reports/balance/opening/{period}
  Future<Map<String, dynamic>> updateOpeningBalance({
    required String period,
    required String openingBalance,
    String? reason,
  }) async {
    final response = await dio.put(
      '/api/v1/reports/balance/opening/$period',
      data: {
        'opening_balance': openingBalance,
        'reason': reason ?? 'Koreksi saldo awal',
      },
    );
    final data = response.data;
    if (data is! Map) throw const FormatException('Unexpected response');
    return data.cast<String, dynamic>();
  }

  /// GET /api/v1/dashboard/summary
  Future<Map<String, dynamic>> getDashboardSummary([String? period]) async {
    final response = await dio.get(
      '/api/v1/dashboard/summary',
      queryParameters: period != null ? {'period': period} : null,
    );
    final data = response.data;
    if (data is! Map) throw const FormatException('Unexpected response');
    return data.cast<String, dynamic>();
  }

  /// GET /api/v1/categories
  Future<List<BackendCategory>> listCategories({bool? isActive}) async {
    final query = <String, dynamic>{};
    if (isActive != null) query['is_active'] = isActive.toString();
    final response = await dio.get(
      '/api/v1/categories',
      queryParameters: query,
    );
    final data = response.data;
    if (data is List) {
      return data
          .whereType<Map>()
          .map((e) => BackendCategory.fromJson(e.cast<String, dynamic>()))
          .toList();
    }
    return [];
  }

  // ─── Helpers ────────────────────────────────────────────────────────

  PaginatedResponse<BackendTransaction> _parsePaginatedTransactions(
    dynamic raw,
  ) {
    if (raw is! Map) {
      return const PaginatedResponse(
        data: [],
        pagination: PaginationInfo(
          page: 1,
          pageSize: 20,
          total: 0,
          totalPages: 0,
        ),
      );
    }

    final items = <BackendTransaction>[];
    final rawList = raw['data'];
    if (rawList is List) {
      for (final item in rawList) {
        if (item is Map) {
          items.add(BackendTransaction.fromJson(item.cast<String, dynamic>()));
        }
      }
    }

    final pag = raw['pagination'] as Map<String, dynamic>?;
    return PaginatedResponse(
      data: items,
      pagination: PaginationInfo(
        page: (pag?['page'] as num?)?.toInt() ?? 1,
        pageSize: (pag?['page_size'] as num?)?.toInt() ?? 20,
        total: (pag?['total'] as num?)?.toInt() ?? items.length,
        totalPages: (pag?['total_pages'] as num?)?.toInt() ?? 1,
      ),
    );
  }
}
