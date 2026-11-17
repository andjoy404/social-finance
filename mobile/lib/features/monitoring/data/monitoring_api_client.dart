import 'package:dio/dio.dart';

import 'monitoring_models.dart';

class MonitoringApiClient {
  final Dio dio;

  MonitoringApiClient({required this.dio});

  /// GET /api/v1/rw/{rtId}/monitoring
  Future<MonitoringListResponse> getMonitoringHouseholds(
    String rtId, {
    bool readOnly = false,
  }) async {
    final endpoint = readOnly ? '/read-only' : '';
    final response = await dio.get('/api/v1/rw/$rtId/monitoring$endpoint');
    final data = response.data;
    if (data is Map) {
      return MonitoringListResponse.fromJson(data.cast<String, dynamic>());
    }
    throw const FormatException(
      'Unexpected response fetching monitoring households',
    );
  }

  /// POST /api/v1/rw/{rtId}/monitoring/notes/{householdOccupancyId}
  Future<MonitoringNote> createMonitoringNote({
    required String rtId,
    required String householdOccupancyId,
    required String note,
  }) async {
    final response = await dio.post(
      '/api/v1/rw/$rtId/monitoring/notes/$householdOccupancyId',
      data: {'note': note},
    );
    final data = response.data;
    if (data is Map && data['data'] is Map) {
      return MonitoringNote.fromJson(data['data'] as Map<String, dynamic>);
    }
    throw const FormatException('Unexpected response creating monitoring note');
  }

  /// GET /api/v1/rw/{rtId}/monitoring/notes/{householdOccupancyId}
  Future<List<MonitoringNote>> getMonitoringNotes({
    required String rtId,
    required String householdOccupancyId,
  }) async {
    final response = await dio.get(
      '/api/v1/rw/$rtId/monitoring/notes/$householdOccupancyId',
    );
    final data = response.data;
    if (data is Map && data['data'] is List) {
      final list = data['data'] as List;
      return list
          .map((e) => MonitoringNote.fromJson(e as Map<String, dynamic>))
          .toList();
    }
    return [];
  }

  /// PATCH /api/v1/rw/{rtId}/monitoring/notes/{householdOccupancyId}
  Future<MonitoringNote> updateMonitoringNote({
    required String rtId,
    required String householdOccupancyId,
    required String note,
  }) async {
    final response = await dio.patch(
      '/api/v1/rw/$rtId/monitoring/notes/$householdOccupancyId',
      data: {'note': note},
    );
    final data = response.data;
    if (data is Map && data['data'] is Map) {
      return MonitoringNote.fromJson(data['data'] as Map<String, dynamic>);
    }
    throw const FormatException('Unexpected response updating monitoring note');
  }

  /// GET /api/v1/rw/{rtId}/monitoring/export/excel
  Future<List<int>> exportMonitoringExcel(String rtId) async {
    final response = await dio.get(
      '/api/v1/rw/$rtId/monitoring/export/excel',
      options: Options(responseType: ResponseType.bytes),
    );
    return response.data as List<int>;
  }

  /// GET /api/v1/rw/{rtId}/monitoring/export/pdf
  Future<List<int>> exportMonitoringPDF(String rtId) async {
    final response = await dio.get(
      '/api/v1/rw/$rtId/monitoring/export/pdf',
      options: Options(responseType: ResponseType.bytes),
    );
    return response.data as List<int>;
  }
}
