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

  /// GET /api/v1/rw/{rwId}/kas?period=YYYY-MM (query param)
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

  /// POST /api/v1/rw/{rwId}/opening-balance
  Future<void> updateOpeningBalance(String rwId, String openingBalance) async {
    final url = '/api/v1/rw/${Uri.encodeComponent(rwId)}/opening-balance';
    try {
      await _apiClient.post(url, data: {
        'opening_balance': openingBalance,
      });
    } catch (e) {
      rethrow;
    }
  }

  /// POST /api/v1/rw/{rwId}/locations/{id}/opening-balance
  Future<RwFundLocation> allocateOpeningBalance(
    String rwId,
    String locationId,
    Map<String, dynamic> body,
  ) async {
    final url = '/api/v1/rw/${Uri.encodeComponent(rwId)}/locations/${Uri.encodeComponent(locationId)}/opening-balance';
    try {
      final response = await _apiClient.post(url, data: body);
      final data = response.data as Map<String, dynamic>;
      return RwFundLocation.fromJson(data['data'] as Map<String, dynamic>);
    } catch (e) {
      rethrow;
    }
  }

  /// GET /api/v1/rw/{rwId}/rts — RT di bawah RW untuk dropdown Setoran RT
  Future<List<RwRtInfo>> getRTRs(String rwId) async {
    final url = '/api/v1/rw/${Uri.encodeComponent(rwId)}/rts';
    try {
      final response = await _apiClient.get(url);
      final data = response.data as Map<String, dynamic>;
      final items = data['data'] as List?;
      if (items is! List) return [];
      return items.whereType<Map<String, dynamic>>()
          .map((e) => RwRtInfo.fromJson(e))
          .toList();
    } catch (e) {
      rethrow;
    }
  }

  /// GET /api/v1/categories?is_active=true — kategori keuangan (expense/income)
  /// Backend returns a raw JSON array (not wrapped in { "data": ... }), so we
  /// must not use _apiClient.get() which forces `dio.get<Map<String, dynamic>>()`.
  Future<List<RwFinancialCategory>> getCategories({bool? isActive}) async {
    final query = <String, dynamic>{};
    if (isActive != null) query['is_active'] = isActive;
    final url = '/api/v1/categories';
    try {
      final response = await _apiClient.dio.get<List<dynamic>>(
        url,
        queryParameters: query,
      );
      final items = response.data;
      if (items is! List) return [];
      return items.whereType<Map<String, dynamic>>()
          .map((e) => RwFinancialCategory.fromJson(e))
          .toList();
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

  /// GET /api/v1/rw/{rwId}/locations (optional is_active/category)
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
      if (items is! List) return [];

      return items.whereType<Map<String, dynamic>>()
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
      if (items is! List) return [];

      return items.whereType<Map<String, dynamic>>()
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
      if (items is! List) return [];

      return items.whereType<Map<String, dynamic>>()
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
  // -------------------------------------------------------------------------
  // RW Notifications (RequireRWScope; rw.notification.read/write enforced by backend)
  // -------------------------------------------------------------------------

  Future<RwNotificationLogsPage> getNotifications(
    String rwId, {
    int page = 1,
    int pageSize = 20,
    String? eventType,
  }) async {
    final query = <String, dynamic>{
      'page': page,
      'page_size': pageSize,
    };
    if (eventType != null && eventType.isNotEmpty) query['event_type'] = eventType;
    final url = '/api/v1/rw/${Uri.encodeComponent(rwId)}/notifications';
    final response = await _apiClient.get(url, queryParameters: query);
    final data = response.data as Map<String, dynamic>;
    return RwNotificationLogsPage.fromJson(data);
  }

  Future<int> sendManualPush(String rwId, {required String title, required String body}) async {
    final url = '/api/v1/rw/${Uri.encodeComponent(rwId)}/notifications/manual';
    final response = await _apiClient.post(url, data: {'title': title, 'body': body});
    final data = response.data as Map<String, dynamic>;
    final inner = data['data'];
    if (inner is Map && inner['sent_count'] != null) {
      return (inner['sent_count'] as num).toInt();
    }
    return 0;
  }

  Future<List<RwNotificationTemplate>> listNotificationTemplates(String rwId, {String? eventType}) async {
    var url = '/api/v1/rw/${Uri.encodeComponent(rwId)}/notifications/templates';
    if (eventType != null && eventType.isNotEmpty) {
      url += '?event_type=${Uri.encodeComponent(eventType)}';
    }
    final response = await _apiClient.get(url);
    final data = response.data as Map<String, dynamic>;
    final items = data['data'] as List? ?? const [];
    return items
        .whereType<Map>()
        .map((e) => RwNotificationTemplate.fromJson(e.cast<String, dynamic>()))
        .toList();
  }

  Future<RwNotificationTemplate> createNotificationTemplate(
    String rwId, {
    required String name,
    required String eventType,
    required String title,
    required String body,
    String channel = 'push',
  }) async {
    final url = '/api/v1/rw/${Uri.encodeComponent(rwId)}/notifications/templates';
    final response = await _apiClient.post(url, data: {
      'name': name,
      'event_type': eventType,
      'channel': channel,
      'title': title,
      'body': body,
    });
    final data = response.data as Map<String, dynamic>;
    return RwNotificationTemplate.fromJson((data['data'] as Map).cast<String, dynamic>());
  }

  Future<RwNotificationTemplate> toggleNotificationTemplate(String rwId, String templateId) async {
    final url = '/api/v1/rw/${Uri.encodeComponent(rwId)}/notifications/templates/${Uri.encodeComponent(templateId)}/toggle';
    final response = await _apiClient.patch(url);
    final data = response.data as Map<String, dynamic>;
    return RwNotificationTemplate.fromJson((data['data'] as Map).cast<String, dynamic>());
  }

  Future<void> deleteNotificationTemplate(String rwId, String templateId) async {
    final url = '/api/v1/rw/${Uri.encodeComponent(rwId)}/notifications/templates/${Uri.encodeComponent(templateId)}';
    await _apiClient.dio.delete(url);
  }

  Future<Map<String, dynamic>> previewNotificationTemplate(
    String rwId,
    String templateId, {
    Map<String, String>? variables,
  }) async {
    final url = '/api/v1/rw/${Uri.encodeComponent(rwId)}/notifications/templates/${Uri.encodeComponent(templateId)}/preview';
    final response = await _apiClient.post(url, data: {
      if (variables != null) 'variables': variables,
    });
    final data = response.data as Map<String, dynamic>;
    return (data['data'] as Map).cast<String, dynamic>();
  }

  Future<int> sendTemplateNotification(
    String rwId,
    String templateId, {
    String? target,
    Map<String, String>? variables,
    String? title,
    String? body,
  }) async {
    final url = '/api/v1/rw/${Uri.encodeComponent(rwId)}/notifications/templates/${Uri.encodeComponent(templateId)}/send';
    final response = await _apiClient.post(url, data: {
      if (target != null) 'target': target,
      if (variables != null) 'variables': variables,
      if (title != null) 'title': title,
      if (body != null) 'body': body,
    });
    final data = response.data as Map<String, dynamic>;
    final inner = data['data'];
    if (inner is Map && inner['sent_count'] != null) {
      return (inner['sent_count'] as num).toInt();
    }
    return 0;
  }
}

