import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/errors/app_errors.dart';
import 'monitoring_models.dart';
import 'monitoring_api_client.dart';

class MonitoringRepository {
  final MonitoringApiClient apiClient;

  MonitoringRepository({required this.apiClient});

  Future<MonitoringListResponse> fetchMonitoringHouseholds(String rtId) async {
    try {
      return await apiClient.getMonitoringHouseholds(rtId);
    } catch (e) {
      throw _handleError(e);
    }
  }

  Future<MonitoringNote> createMonitoringNote({
    required String rtId,
    required String householdOccupancyId,
    required String note,
  }) async {
    try {
      return await apiClient.createMonitoringNote(
        rtId: rtId,
        householdOccupancyId: householdOccupancyId,
        note: note,
      );
    } catch (e) {
      throw _handleError(e);
    }
  }

  Future<List<MonitoringNote>> fetchMonitoringNotes({
    required String rtId,
    required String householdOccupancyId,
  }) async {
    try {
      return await apiClient.getMonitoringNotes(
        rtId: rtId,
        householdOccupancyId: householdOccupancyId,
      );
    } catch (e) {
      throw _handleError(e);
    }
  }

  Future<MonitoringNote> updateMonitoringNote({
    required String rtId,
    required String householdOccupancyId,
    required String note,
  }) async {
    try {
      return await apiClient.updateMonitoringNote(
        rtId: rtId,
        householdOccupancyId: householdOccupancyId,
        note: note,
      );
    } catch (e) {
      throw _handleError(e);
    }
  }

  Future<List<int>> exportMonitoringExcel(String rtId) async {
    try {
      return await apiClient.exportMonitoringExcel(rtId);
    } catch (e) {
      throw _handleError(e);
    }
  }

  Future<List<int>> exportMonitoringPDF(String rtId) async {
    try {
      return await apiClient.exportMonitoringPDF(rtId);
    } catch (e) {
      throw _handleError(e);
    }
  }

  Object _handleError(dynamic error) {
    if (error is ServerException) return error;
    if (error is AppWithMessageError) return error;
    if (error is DioException) {
      final status = error.response?.statusCode;
      final body = error.response?.data;
      if (body is Map && body['error'] is Map) {
        final msg = body['error']['message'] as String?;
        if (msg != null && msg.isNotEmpty) {
          return ServerException(statusCode: status ?? 400, message: msg);
        }
      }
      return ServerException(
        statusCode: status ?? 500,
        message: 'Gagal memproses permintaan monitoring (${status ?? "koneksi"}).',
      );
    }
    return ServerException(message: 'Terjadi kesalahan tidak terduga.');
  }
}

final monitoringApiClientProvider = Provider<MonitoringApiClient>((ref) {
  final client = ref.watch(apiClientProvider);
  return MonitoringApiClient(dio: client.dio);
});

final monitoringRepositoryProvider = Provider<MonitoringRepository>((ref) {
  final apiClient = ref.watch(monitoringApiClientProvider);
  return MonitoringRepository(apiClient: apiClient);
});

final monitoringHouseholdsProvider =
    FutureProvider.family<MonitoringListResponse, String>((ref, rtId) async {
  final repo = ref.watch(monitoringRepositoryProvider);
  return repo.fetchMonitoringHouseholds(rtId);
});
