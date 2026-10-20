/// Iuran API client — thin Dio wrapper for Iuran/finance endpoints.
///
/// Uses the shared `ApiClient` (Dio) from `client.dart` for HTTP transport,
/// auth token injection, and single-flight refresh token handling.
/// No duplication of auth/interceptor behavior.

import 'dart:math';
import 'dart:typed_data';

import 'package:dio/dio.dart';

import 'iuran_api_models.dart';

/// Generates a v4-style random UUID string.
/// Uses `dart:math` only — no external uuid package in pubspec.yaml.
String _generateUuid() {
  final r = Random();
  String hex(int count) =>
      List.generate(count, (_) => '0123456789abcdef'[r.nextInt(16)]).join();

  // 8-4-4-4-12 format
  return '${hex(8)}-${hex(4)}-4${hex(3)}-'
      '${'89ab'[r.nextInt(4)]}${hex(3)}-'
      '${hex(12)}';
}

String newPaymentIdempotencyKey() => _generateUuid();

class GenerateBillsResult {
  final int generatedCount;
  final List<BackendBill> bills;

  const GenerateBillsResult({
    required this.generatedCount,
    required this.bills,
  });
}

class IuranApiClient {
  final Dio dio;

  IuranApiClient({required this.dio});

  // ─── Dues ───────────────────────────────────────────────────────────

  /// GET /api/v1/dues
  Future<List<BackendDue>> listDues({bool? isActive}) async {
    final query = <String, dynamic>{};
    if (isActive != null) query['is_active'] = isActive ? 'true' : 'false';

    final response = await dio.get('/api/v1/dues', queryParameters: query);

    final data = response.data;
    if (data is! List) {
      throw const FormatException('Unexpected dues response');
    }

    final dues = <BackendDue>[];
    for (final item in data) {
      if (item is! Map) {
        throw const FormatException('Malformed due in response');
      }
      dues.add(BackendDue.fromJson(item.cast<String, dynamic>()));
    }
    return dues;
  }

  /// POST /api/v1/dues — Pengurus only
  Future<BackendDue> createDue({
    required String name,
    required String amount,
    required DuePeriodType periodType,
    PaymentRule? paymentRule,
    bool? isVoluntary,
  }) async {
    final Map<String, dynamic> body = {
      'name': name,
      'amount': amount,
      'period_type': _serializePeriodType(periodType),
    };
    if (paymentRule != null) {
      body['payment_rule'] = _serializePaymentRule(paymentRule);
    }
    if (isVoluntary != null) {
      body['is_voluntary'] = isVoluntary;
    }

    final response = await dio.post('/api/v1/dues', data: body);

    final bodyMap = response.data;
    if (bodyMap is! Map) throw const FormatException('Unexpected response');
    return BackendDue.fromJson(bodyMap.cast<String, dynamic>());
  }

  /// PATCH /api/v1/dues/{id}
  Future<BackendDue> updateDue(
    String id, {
    String? name,
    String? amount,
    DuePeriodType? periodType,
    PaymentRule? paymentRule,
    bool? isVoluntary,
    bool? isActive,
  }) async {
    final body = <String, dynamic>{};
    if (name != null) body['name'] = name;
    if (amount != null) body['amount'] = amount;
    if (periodType != null) {
      body['period_type'] = _serializePeriodType(periodType);
    }
    if (paymentRule != null) {
      body['payment_rule'] = _serializePaymentRule(paymentRule);
    }
    if (isVoluntary != null) body['is_voluntary'] = isVoluntary;
    if (isActive != null) body['is_active'] = isActive;

    final response = await dio.patch(
      '/api/v1/dues/$id',
      data: body.isEmpty ? null : body,
    );

    final data = response.data;
    if (data is! Map) throw const FormatException('Unexpected response');
    return BackendDue.fromJson(data.cast<String, dynamic>());
  }

  /// DELETE /api/v1/dues/{id} — soft deactivate
  Future<void> deactivateDue(String id) async {
    await dio.delete('/api/v1/dues/$id');
  }

  // ─── Bills ──────────────────────────────────────────────────────────

  /// GET /api/v1/bills
  Future<PaginatedResponse<BackendBill>> listBills({
    String? dueId,
    String? status,
    String? period,
    String? householdOccupancyId,
    bool myBills = false,
    int page = 1,
    int pageSize = 20,
  }) async {
    final query = <String, dynamic>{
      'page': page.toString(),
      'page_size': pageSize.toString(),
    };
    if (dueId != null && dueId.isNotEmpty) query['due_id'] = dueId;
    if (status != null && status.isNotEmpty) query['status'] = status;
    if (period != null && period.isNotEmpty) query['period'] = period;
    if (householdOccupancyId != null && householdOccupancyId.isNotEmpty) {
      query['household_occupancy_id'] = householdOccupancyId;
    }
    if (myBills) query['my_bills'] = 'true';

    final response = await dio.get('/api/v1/bills', queryParameters: query);
    return _parsePaginatedBills(response.data);
  }

  /// GET /api/v1/bills/{id}
  Future<BackendBill> getBill(String id) async {
    final response = await dio.get('/api/v1/bills/$id');
    final data = response.data;
    if (data is! Map) throw const FormatException('Unexpected response');
    return BackendBill.fromJson(data.cast<String, dynamic>());
  }

  /// POST /api/v1/bills/generate — with Idempotency-Key
  Future<GenerateBillsResult> generateBills({
    required String dueId,
    required String period,
    required String dueDate,
    required String idempotencyKey,
  }) async {
    final response = await dio.post(
      '/api/v1/bills/generate',
      data: {'due_id': dueId, 'period': period, 'due_date': dueDate},
      options: Options(headers: {'Idempotency-Key': idempotencyKey}),
    );

    return _parseGenerateBillsResponse(response.data);
  }

  /// Parses the actual backend response: {generated_count, bills}.
  GenerateBillsResult parseGenerateBillsResponse(dynamic data) {
    return _parseGenerateBillsResponse(data);
  }

  GenerateBillsResult _parseGenerateBillsResponse(dynamic data) {
    if (data is! Map) {
      throw const FormatException('Unexpected generate bills response');
    }

    final rawCount = data['generated_count'];
    final rawBills = data['bills'];
    if (rawCount is! num || rawBills is! List) {
      throw const FormatException('Malformed generate bills response');
    }

    final bills = <BackendBill>[];
    for (final item in rawBills) {
      if (item is! Map) {
        throw const FormatException(
          'Malformed bill in generate bills response',
        );
      }
      bills.add(BackendBill.fromJson(item.cast<String, dynamic>()));
    }

    final generatedCount = rawCount.toInt();
    if (generatedCount < 0 || generatedCount != bills.length) {
      throw const FormatException(
        'generated_count does not match bills response',
      );
    }

    return GenerateBillsResult(generatedCount: generatedCount, bills: bills);
  }

  /// DELETE /api/v1/bills/{id} — cancel bill
  Future<void> cancelBill(String id) async {
    await dio.delete('/api/v1/bills/$id');
  }

  // ─── Payments ───────────────────────────────────────────────────────

  /// GET /api/v1/payments
  Future<PaginatedResponse<BackendPayment>> listPayments({
    String? billId,
    String? dueId,
    String? status,
    int page = 1,
    int pageSize = 20,
  }) async {
    final query = <String, dynamic>{
      'page': page.toString(),
      'page_size': pageSize.toString(),
    };
    if (billId != null && billId.isNotEmpty) query['bill_id'] = billId;
    if (dueId != null && dueId.isNotEmpty) query['due_id'] = dueId;
    if (status != null && status.isNotEmpty) query['status'] = status;

    final response = await dio.get('/api/v1/payments', queryParameters: query);
    return _parsePaginatedPayments(response.data);
  }

  /// GET /api/v1/payments/{id}
  Future<BackendPayment> getPayment(String id) async {
    final response = await dio.get('/api/v1/payments/$id');
    final data = response.data;
    if (data is! Map) throw const FormatException('Unexpected response');
    return BackendPayment.fromJson(data.cast<String, dynamic>());
  }

  /// POST /api/v1/payments — with Idempotency-Key
  Future<BackendPayment> createPayment({
    required String billId,
    required String amount,
    required PaymentMethod method,
    String? proofPath,
    String? notes,
    String? idempotencyKey,
  }) async {
    return _createPayment(
      billId: billId,
      amount: amount,
      method: method,
      proofPath: proofPath,
      notes: notes,
      idempotencyKey: idempotencyKey,
    );
  }

  Future<BackendPayment> createDonation({
    required String dueId,
    required String amount,
    required PaymentMethod method,
    String? notes,
    String? idempotencyKey,
  }) async {
    return _createPayment(
      dueId: dueId,
      amount: amount,
      method: method,
      notes: notes,
      idempotencyKey: idempotencyKey,
    );
  }

  Future<BackendPayment> _createPayment({
    String? billId,
    String? dueId,
    required String amount,
    required PaymentMethod method,
    String? proofPath,
    String? notes,
    String? idempotencyKey,
  }) async {
    final hasBill = billId?.trim().isNotEmpty == true;
    final hasDue = dueId?.trim().isNotEmpty == true;
    if (hasBill == hasDue) {
      throw ArgumentError('Exactly one of billId or dueId is required.');
    }
    final response = await dio.post(
      '/api/v1/payments',
      data: {
        if (billId != null) 'bill_id': billId,
        if (dueId != null) 'due_id': dueId,
        'amount': amount,
        'method': _serializePaymentMethod(method),
        if (notes != null && notes.isNotEmpty) 'notes': notes,
        if (proofPath != null && proofPath.isNotEmpty) 'proof_path': proofPath,
      },
      options: Options(
        headers: {'Idempotency-Key': idempotencyKey ?? _generateUuid()},
      ),
    );

    final data = response.data;
    if (data is! Map) throw const FormatException('Unexpected response');
    return BackendPayment.fromJson(data.cast<String, dynamic>());
  }

  /// POST /api/v1/payments/{id}/verify
  Future<BackendPayment> verifyPayment(
    String paymentId, {
    required String action,
    String? rejectionReason,
    String? officerNotes,
  }) async {
    final response = await dio.post(
      '/api/v1/payments/$paymentId/verify',
      data: {
        'action': action,
        if (rejectionReason != null && rejectionReason.isNotEmpty)
          'rejection_reason': rejectionReason,
        if (officerNotes != null && officerNotes.isNotEmpty)
          'officer_notes': officerNotes,
      },
    );

    final data = response.data;
    if (data is! Map) throw const FormatException('Unexpected response');
    return BackendPayment.fromJson(data.cast<String, dynamic>());
  }

  /// POST /api/v1/payments/{id}/proof
  Future<BackendPayment> uploadPaymentProof({
    required String paymentId,
    required List<int> fileBytes,
    required String filename,
  }) async {
    final formData = FormData.fromMap({
      'file': MultipartFile.fromBytes(fileBytes, filename: filename),
    });

    final response = await dio.post(
      '/api/v1/payments/$paymentId/proof',
      data: formData,
    );

    final data = response.data;
    if (data is! Map) throw const FormatException('Unexpected response');
    return BackendPayment.fromJson(data.cast<String, dynamic>());
  }

  /// GET /api/v1/payments/{id}/proof
  /// Returns raw bytes of the image using authenticated session.
  Future<Uint8List> getPaymentProofBytes(String paymentId) async {
    final response = await dio.get<List<int>>(
      '/api/v1/payments/$paymentId/proof',
      options: Options(responseType: ResponseType.bytes),
    );

    if (response.data == null) {
      throw const FormatException('Empty proof image received');
    }
    return Uint8List.fromList(response.data!);
  }

  // ─── Categories ─────────────────────────────────────────────────────

  /// GET /api/v1/categories
  Future<List<BackendCategory>> listCategories({bool? isActive}) async {
    final query = <String, dynamic>{};
    if (isActive != null) query['is_active'] = isActive ? 'true' : 'false';

    final response = await dio.get(
      '/api/v1/categories',
      queryParameters: query,
    );

    final data = response.data;
    if (data is! List) return [];

    return data
        .whereType<Map>()
        .map((e) => BackendCategory.fromJson(e.cast<String, dynamic>()))
        .toList();
  }

  // ─── Reports ────────────────────────────────────────────────────────

  /// GET /api/v1/reports/balance
  Future<BackendBalance> getBalance() async {
    final response = await dio.get('/api/v1/reports/balance');
    final data = response.data;
    if (data is! Map) throw const FormatException('Unexpected response');
    return BackendBalance.fromJson(data.cast<String, dynamic>());
  }

  // ─── Helpers ────────────────────────────────────────────────────────

  PaginatedResponse<BackendBill> _parsePaginatedBills(dynamic raw) {
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

    final items = <BackendBill>[];
    final rawList = raw['data'];
    if (rawList is List) {
      for (final item in rawList) {
        if (item is Map) {
          items.add(BackendBill.fromJson(item.cast<String, dynamic>()));
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

  PaginatedResponse<BackendPayment> _parsePaginatedPayments(dynamic raw) {
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

    final items = <BackendPayment>[];
    final rawList = raw['data'];
    if (rawList is List) {
      for (final item in rawList) {
        if (item is Map) {
          items.add(BackendPayment.fromJson(item.cast<String, dynamic>()));
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

  String _serializePeriodType(DuePeriodType type) {
    switch (type) {
      case DuePeriodType.monthly:
        return 'monthly';
      case DuePeriodType.yearly:
        return 'yearly';
      case DuePeriodType.oneTime:
        return 'one_time';
    }
  }

  String _serializePaymentRule(PaymentRule rule) {
    switch (rule) {
      case PaymentRule.full:
        return 'full';
      case PaymentRule.partial:
        return 'partial';
    }
  }

  String _serializePaymentMethod(PaymentMethod method) {
    switch (method) {
      case PaymentMethod.cash:
        return 'CASH';
      case PaymentMethod.transfer:
        return 'TRANSFER';
    }
  }
}
