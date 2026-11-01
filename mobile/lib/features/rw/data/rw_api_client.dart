/// API client untuk endpoint PR 4 — RW-scoped aggregated data.
///
/// Mirip dengan pattern kas_api_client.dart tetapi untuk data agregat RW.

import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/features/rw/data/rw_models.dart';

/// API client untuk endpoint PR 4 RW.
class RwApiClient {
  final ApiClient _apiClient;

  RwApiClient(this._apiClient);

  Future<RwDashboardSummary> getDashboard(String rwId, {String? period}) async {
    String url;
    if (period != null) {
      url = '/api/v1/rw/${Uri.encodeComponent(rwId)}/dashboard?period=${Uri.encodeComponent(period)}';
    } else {
      url = '/api/v1/rw/${Uri.encodeComponent(rwId)}/dashboard';
    }

    try {
      final response = await _apiClient.get(url);
      final data = response.data as Map<String, dynamic>;
      return RwDashboardSummary.fromJson(data['data'] as Map<String, dynamic>);
    } catch (e) {
      rethrow;
    }
  }

  /// GET /api/v1/rw/{rwId}/kas?period=YYYY-MM
  Future<RwKasReport> getKas(String rwId, {String? period}) async {
    String url;
    if (period != null) {
      url = '/api/v1/rw/${Uri.encodeComponent(rwId)}/kas?period=${Uri.encodeComponent(period)}';
    } else {
      url = '/api/v1/rw/${Uri.encodeComponent(rwId)}/kas';
    }

    try {
      final response = await _apiClient.get(url);
      final data = response.data as Map<String, dynamic>;
      return RwKasReport.fromJson(data['data'] as Map<String, dynamic>);
    } catch (e) {
      rethrow;
    }
  }

  /// GET /api/v1/rw/{rwId}/kas/transactions
  Future<RwTransactionsResponse> getTransactions({
    required String rwId,
    String? period,
    String? startDate,
    String? endDate,
    int page = 1,
    int pageSize = 20,
  }) async {
    final query = <String, dynamic>{
      'page': page,
      'page_size': pageSize,
    };
    if (period != null) query['period'] = period;
    if (startDate != null) query['start_date'] = startDate;
    if (endDate != null) query['end_date'] = endDate;

    final url = '/api/v1/rw/${Uri.encodeComponent(rwId)}/kas/transactions';
    try {
      final response = await _apiClient.get(url, queryParameters: query);
      final data = response.data as Map<String, dynamic>;
      return RwTransactionsResponse.fromJson(data);
    } catch (e) {
      rethrow;
    }
  }

  /// GET /api/v1/rw/{rwId}/locations?is_active=true
  Future<List<RwFundLocation>> getLocations(String rwId, {String? category, bool? isActive}) async {
    final query = <String, dynamic>{};
    if (isActive != null) {
      query['is_active'] = isActive;
    }
    if (category != null && category.isNotEmpty) {
      query['category'] = category;
    }

    final url = '/api/v1/rw/${Uri.encodeComponent(rwId)}/locations';
    try {
      final response = await _apiClient.get(url, queryParameters: query);
      final data = response.data as Map<String, dynamic>;
      final items = data['data'] as List?;
      if (items == null) return [];

      return items
          .whereType<Map<String, dynamic>>()
          .map((e) => RwFundLocation.fromJson(e))
          .toList();
    } catch (e) {
      rethrow;
    }
  }

  /// POST /api/v1/rw/{rwId}/locations
  Future<RwFundLocation> createLocation(String rwId, Map<String, dynamic> body) async {
    final url = '/api/v1/rw/${Uri.encodeComponent(rwId)}/locations';
    try {
      final response = await _apiClient.post(url, data: body);
      final data = response.data as Map<String, dynamic>;
      return RwFundLocation.fromJson(data['data'] as Map<String, dynamic>);
    } catch (e) {
      rethrow;
    }
  }

  /// PUT /api/v1/rw/{rwId}/locations/{id}
  Future<RwFundLocation> updateLocation(String rwId, String locationId, Map<String, dynamic> body) async {
    final url = '/api/v1/rw/${Uri.encodeComponent(rwId)}/locations/${Uri.encodeComponent(locationId)}';
    try {
      final response = await _apiClient.put(url, data: body);
      final data = response.data as Map<String, dynamic>;
      return RwFundLocation.fromJson(data['data'] as Map<String, dynamic>);
    } catch (e) {
      rethrow;
    }
  }

  /// PATCH /api/v1/rw/{rwId}/locations/{id}/status
  Future<RwFundLocation> toggleLocationStatus(String rwId, String locationId, bool isActive) async {
    final url = '/api/v1/rw/${Uri.encodeComponent(rwId)}/locations/${Uri.encodeComponent(locationId)}/status';
    try {
      final response = await _apiClient.patch(url, data: {'is_active': isActive});
      final data = response.data as Map<String, dynamic>;
      return RwFundLocation.fromJson(data['data'] as Map<String, dynamic>);
    } catch (e) {
      rethrow;
    }
  }

  /// GET /api/v1/rw/{rwId}/locations/reconcile
  Future<RwReconciliationReport> getReconciliation(String rwId) async {
    final url = '/api/v1/rw/${Uri.encodeComponent(rwId)}/locations/reconcile';
    try {
      final response = await _apiClient.get(url);
      final data = response.data as Map<String, dynamic>;
      return RwReconciliationReport.fromJson(data['data'] as Map<String, dynamic>);
    } catch (e) {
      rethrow;
    }
  }

  /// GET /api/v1/rw/{rwId}/locations/{id}/mutations
  Future<List<RwLocationMutation>> getLocationMutations(String rwId, String locationId, {int limit = 50}) async {
    final url = '/api/v1/rw/${Uri.encodeComponent(rwId)}/locations/${Uri.encodeComponent(locationId)}/mutations?limit=$limit';
    try {
      final response = await _apiClient.get(url);
      final data = response.data as Map<String, dynamic>;
      final items = data['data'] as List?;
      if (items == null) return [];

      return items
          .whereType<Map<String, dynamic>>()
          .map((e) => RwLocationMutation.fromJson(e))
          .toList();
    } catch (e) {
      rethrow;
    }
  }

  /// GET /api/v1/rw/{rwId}/transfers
  Future<List<RwFundTransfer>> getTransfers(String rwId, {int page = 1, int pageSize = 20}) async {
    final url = '/api/v1/rw/${Uri.encodeComponent(rwId)}/transfers?page=$page&page_size=$pageSize';
    try {
      final response = await _apiClient.get(url);
      final data = response.data as Map<String, dynamic>;
      final items = data['data'] as List?;
      if (items == null) return [];

      return items
          .whereType<Map<String, dynamic>>()
          .map((e) => RwFundTransfer.fromJson(e))
          .toList();
    } catch (e) {
      rethrow;
    }
  }

  /// POST /api/v1/rw/{rwId}/transfers
  Future<RwFundTransfer> createTransfer(String rwId, Map<String, dynamic> body) async {
    final url = '/api/v1/rw/${Uri.encodeComponent(rwId)}/transfers';
    try {
      final response = await _apiClient.post(url, data: body);
      final data = response.data as Map<String, dynamic>;
      return RwFundTransfer.fromJson(data['data'] as Map<String, dynamic>);
    } catch (e) {
      rethrow;
    }
  }

  /// POST /api/v1/rw/{rwId}/transfers/{id}/reverse
  Future<RwFundTransfer> reverseTransfer(String rwId, String transferId, String reason) async {
    final url = '/api/v1/rw/${Uri.encodeComponent(rwId)}/transfers/${Uri.encodeComponent(transferId)}/reverse';
    try {
      final response = await _apiClient.post(url, data: {'reason': reason});
      final data = response.data as Map<String, dynamic>;
      return RwFundTransfer.fromJson(data['data'] as Map<String, dynamic>);
    } catch (e) {
      rethrow;
    }
  }

  /// POST /api/v1/rw/{rwId}/kas/transactions
  Future<RwTransaction> createTransaction(String rwId, Map<String, dynamic> body) async {
    final url = '/api/v1/rw/${Uri.encodeComponent(rwId)}/kas/transactions';
    try {
      final response = await _apiClient.post(url, data: body);
      final data = response.data as Map<String, dynamic>;
      return RwTransaction.fromJson(data['data'] as Map<String, dynamic>);
    } catch (e) {
      rethrow;
    }
  }

  /// POST /api/v1/rw/{rwId}/kas/transactions/{id}/proof
  Future<Map<String, dynamic>> uploadTransactionProof(
    String rwId,
    String transactionId,
    List<int> fileBytes,
    String filename,
  ) async {
    final url = '/api/v1/rw/${Uri.encodeComponent(rwId)}/kas/transactions/${Uri.encodeComponent(transactionId)}/proof';
    final formData = FormData.fromMap({
      'file': MultipartFile.fromBytes(fileBytes, filename: filename),
    });
    try {
      final response = await _apiClient.dio.post(url, data: formData);
      return response.data as Map<String, dynamic>;
    } catch (e) {
      rethrow;
    }
  }

  /// GET /api/v1/rw/{rwId}/kas/transactions/{id}/proof
  Future<Uint8List> getTransactionProofBytes(String rwId, String transactionId) async {
    final url = '/api/v1/rw/${Uri.encodeComponent(rwId)}/kas/transactions/${Uri.encodeComponent(transactionId)}/proof';
    try {
      final response = await _apiClient.dio.get<List<int>>(
        url,
        options: Options(responseType: ResponseType.bytes),
      );
      return Uint8List.fromList(response.data!);
    } catch (e) {
      rethrow;
    }
  }
}


