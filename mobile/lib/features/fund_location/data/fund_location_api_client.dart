import 'package:dio/dio.dart';
import 'package:social_finance/features/kas/data/kas_api_models.dart';
import 'models/fund_location_models.dart';

class FundLocationApiClient {
  final Dio dio;

  FundLocationApiClient({required this.dio});

  /// GET /api/v1/fund-locations
  Future<List<FundLocation>> listLocations({
    String? category,
    bool? isActive,
  }) async {
    final queryParams = <String, dynamic>{};
    if (category != null && category.isNotEmpty) {
      queryParams['category'] = category;
    }
    if (isActive != null) {
      queryParams['is_active'] = isActive;
    }

    final response = await dio.get(
      '/api/v1/fund-locations',
      queryParameters: queryParams,
    );

    final data = response.data;
    if (data is Map && data['data'] is List) {
      final list = data['data'] as List;
      return list
          .map((e) => FundLocation.fromJson(e as Map<String, dynamic>))
          .toList();
    }
    return [];
  }

  /// GET /api/v1/fund-locations/{id}
  Future<FundLocation> getLocation(String id) async {
    final response = await dio.get('/api/v1/fund-locations/$id');
    final data = response.data;
    if (data is Map) {
      final payload = (data['data'] is Map ? data['data'] : data) as Map<String, dynamic>;
      return FundLocation.fromJson(payload);
    }
    throw const FormatException('Unexpected response');
  }

  /// POST /api/v1/fund-locations
  Future<FundLocation> createLocation({
    required String name,
    required String category,
    String? bankName,
    String? accountNumber,
    String? accountHolder,
  }) async {
    final response = await dio.post(
      '/api/v1/fund-locations',
      data: {
        'name': name,
        'category': category,
        if (bankName != null && bankName.isNotEmpty) 'bank_name': bankName,
        if (accountNumber != null && accountNumber.isNotEmpty) 'account_number': accountNumber,
        if (accountHolder != null && accountHolder.isNotEmpty) 'account_holder': accountHolder,
      },
    );

    final data = response.data;
    if (data is Map) {
      final payload = (data['data'] is Map ? data['data'] : data) as Map<String, dynamic>;
      return FundLocation.fromJson(payload);
    }
    throw const FormatException('Unexpected response');
  }

  /// PATCH /api/v1/fund-locations/{id}
  Future<FundLocation> updateLocation({
    required String id,
    String? name,
    String? bankName,
    String? accountNumber,
    String? accountHolder,
    bool? isActive,
  }) async {
    final body = <String, dynamic>{};
    if (name != null) body['name'] = name;
    if (bankName != null) body['bank_name'] = bankName;
    if (accountNumber != null) body['account_number'] = accountNumber;
    if (accountHolder != null) body['account_holder'] = accountHolder;
    if (isActive != null) body['is_active'] = isActive;

    final response = await dio.patch(
      '/api/v1/fund-locations/$id',
      data: body,
    );

    final data = response.data;
    if (data is Map) {
      final payload = (data['data'] is Map ? data['data'] : data) as Map<String, dynamic>;
      return FundLocation.fromJson(payload);
    }
    throw const FormatException('Unexpected response');
  }

  /// DELETE /api/v1/fund-locations/{id}
  Future<void> deactivateLocation(String id) async {
    await dio.delete('/api/v1/fund-locations/$id');
  }

  /// GET /api/v1/fund-locations/{id}/history
  Future<PaginatedResponse<LocationBalanceChange>> getLocationHistory({
    required String id,
    int page = 1,
    int pageSize = 20,
  }) async {
    final response = await dio.get(
      '/api/v1/fund-locations/$id/history',
      queryParameters: {
        'page': page,
        'per_page': pageSize,
      },
    );

    final data = response.data;
    if (data is Map && data['data'] is List) {
      final list = (data['data'] as List)
          .map((e) => LocationBalanceChange.fromJson(e as Map<String, dynamic>))
          .toList();
      final paginationMap = (data['pagination'] as Map<String, dynamic>?) ?? {};
      return PaginatedResponse(
        data: list,
        pagination: PaginationInfo(
          page: paginationMap['page'] as int? ?? page,
          pageSize: paginationMap['per_page'] as int? ?? pageSize,
          total: paginationMap['total'] as int? ?? list.length,
          totalPages: paginationMap['total_pages'] as int? ?? 1,
        ),
      );
    }
    return PaginatedResponse(
      data: [],
      pagination: PaginationInfo(page: page, pageSize: pageSize, total: 0, totalPages: 0),
    );
  }

  /// PUT /api/v1/fund-locations/{id}/opening-balance
  Future<void> setOpeningBalance({
    required String id,
    required String amount,
    String? notes,
  }) async {
    await dio.put(
      '/api/v1/fund-locations/$id/opening-balance',
      data: {
        'amount': amount,
        if (notes != null && notes.isNotEmpty) 'notes': notes,
      },
    );
  }

  /// GET /api/v1/fund-transfers
  Future<PaginatedResponse<FundTransfer>> listTransfers({
    int page = 1,
    int pageSize = 20,
  }) async {
    final response = await dio.get(
      '/api/v1/fund-transfers',
      queryParameters: {
        'page': page,
        'per_page': pageSize,
      },
    );

    final data = response.data;
    if (data is Map && data['data'] is List) {
      final list = (data['data'] as List)
          .map((e) => FundTransfer.fromJson(e as Map<String, dynamic>))
          .toList();
      final paginationMap = (data['pagination'] as Map<String, dynamic>?) ?? {};
      return PaginatedResponse(
        data: list,
        pagination: PaginationInfo(
          page: paginationMap['page'] as int? ?? page,
          pageSize: paginationMap['per_page'] as int? ?? pageSize,
          total: paginationMap['total'] as int? ?? list.length,
          totalPages: paginationMap['total_pages'] as int? ?? 1,
        ),
      );
    }
    return PaginatedResponse(
      data: [],
      pagination: PaginationInfo(page: page, pageSize: pageSize, total: 0, totalPages: 0),
    );
  }

  /// POST /api/v1/fund-transfers
  Future<FundTransfer> createTransfer({
    required String fromLocationId,
    required String toLocationId,
    required String amount,
    String? notes,
    String? idempotencyKey,
  }) async {
    final options = Options();
    if (idempotencyKey != null && idempotencyKey.isNotEmpty) {
      options.headers = {'Idempotency-Key': idempotencyKey};
    }

    final response = await dio.post(
      '/api/v1/fund-transfers',
      data: {
        'from_location_id': fromLocationId,
        'to_location_id': toLocationId,
        'amount': amount,
        if (notes != null && notes.isNotEmpty) 'notes': notes,
      },
      options: options,
    );

    final data = response.data;
    if (data is Map) {
      final payload = (data['data'] is Map ? data['data'] : data) as Map<String, dynamic>;
      return FundTransfer.fromJson(payload);
    }
    throw const FormatException('Unexpected response');
  }

  /// POST /api/v1/fund-transfers/{id}/reverse
  Future<void> reverseTransfer({
    required String id,
    required String reason,
  }) async {
    await dio.post(
      '/api/v1/fund-transfers/$id/reverse',
      data: {
        'reason': reason,
      },
    );
  }

  /// GET /api/v1/reports/fund-locations/reconciliation
  Future<FundLocationReconciliationReport> getReconciliationReport() async {
    final response = await dio.get('/api/v1/reports/fund-locations/reconciliation');
    final data = response.data;
    if (data is Map) {
      final payload = (data['data'] is Map ? data['data'] : data) as Map<String, dynamic>;
      return FundLocationReconciliationReport.fromJson(payload);
    }
    throw const FormatException('Unexpected response');
  }
}
