import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/errors/app_errors.dart';
import 'models/notification_models.dart';
import 'notification_api_client.dart';

class NotificationRepository {
  final NotificationApiClient apiClient;

  NotificationRepository({required this.apiClient});

  Future<List<SystemNotificationType>> fetchSystemTypes() async {
    try {
      return await apiClient.getSystemTypes();
    } catch (e) {
      throw _handleError(e);
    }
  }

  Future<List<NotificationTemplate>> fetchTemplates() async {
    try {
      return await apiClient.getTemplates();
    } catch (e) {
      throw _handleError(e);
    }
  }

  Future<NotificationTemplate> createTemplate({
    required String eventType,
    required String name,
    required String title,
    required String body,
    String? imageUrl,
  }) async {
    try {
      return await apiClient.createTemplate(
        eventType: eventType,
        name: name,
        title: title,
        body: body,
        imageUrl: imageUrl,
      );
    } catch (e) {
      throw _handleError(e);
    }
  }

  Future<NotificationTemplate> updateTemplate({
    required String id,
    required String name,
    required String title,
    required String body,
    String? imageUrl,
  }) async {
    try {
      return await apiClient.updateTemplate(
        id: id,
        name: name,
        title: title,
        body: body,
        imageUrl: imageUrl,
      );
    } catch (e) {
      throw _handleError(e);
    }
  }

  Future<void> sendManualPush({
    required String title,
    required String body,
    String? imageUrl,
    String? deepLink,
  }) async {
    try {
      await apiClient.sendManualPush(
        title: title,
        body: body,
        imageUrl: imageUrl,
        deepLink: deepLink,
      );
    } catch (e) {
      throw _handleError(e);
    }
  }

  Future<int> sendTestPush({
    required String eventType,
    required String title,
    required String body,
    String? imageUrl,
  }) async {
    try {
      return await apiClient.sendTestPush(
        eventType: eventType,
        title: title,
        body: body,
        imageUrl: imageUrl,
      );
    } catch (e) {
      throw _handleError(e);
    }
  }

  Future<NotificationTemplate> toggleTemplateActive(String id) async {
    try {
      return await apiClient.toggleTemplateActive(id);
    } catch (e) {
      throw _handleError(e);
    }
  }

  Future<void> deleteTemplate(String id) async {
    try {
      await apiClient.deleteTemplate(id);
    } catch (e) {
      throw _handleError(e);
    }
  }

  Future<NotificationLogsResponse> fetchLogs({
    int page = 1,
    int perPage = 20,
    String? eventType,
  }) async {
    try {
      return await apiClient.getLogs(
        page: page,
        perPage: perPage,
        eventType: eventType,
      );
    } catch (e) {
      throw _handleError(e);
    }
  }

  Future<NotificationLogEntry> fetchNotificationDetail(String id) async {
    try {
      return await apiClient.getNotificationDetail(id);
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
        message: 'Gagal memproses permintaan notifikasi (${status ?? "koneksi"}).',
      );
    }
    return ServerException(message: 'Terjadi kesalahan tidak terduga.');
  }
}

final notificationApiClientProvider = Provider<NotificationApiClient>((ref) {
  final client = ref.watch(apiClientProvider);
  return NotificationApiClient(dio: client.dio);
});

final notificationRepositoryProvider = Provider<NotificationRepository>((ref) {
  final apiClient = ref.watch(notificationApiClientProvider);
  return NotificationRepository(apiClient: apiClient);
});

final notificationSystemTypesProvider =
    FutureProvider.autoDispose<List<SystemNotificationType>>((ref) async {
  final repo = ref.watch(notificationRepositoryProvider);
  return repo.fetchSystemTypes();
});

final notificationTemplatesProvider =
    FutureProvider.autoDispose<List<NotificationTemplate>>((ref) async {
  final repo = ref.watch(notificationRepositoryProvider);
  return repo.fetchTemplates();
});
