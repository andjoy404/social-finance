import 'package:dio/dio.dart';
import 'models/notification_models.dart';

class NotificationApiClient {
  final Dio dio;

  NotificationApiClient({required this.dio});

  /// GET /api/v1/notifications/system-types
  Future<List<SystemNotificationType>> getSystemTypes() async {
    final response = await dio.get('/api/v1/notifications/system-types');
    final data = response.data;
    if (data is Map && data['data'] is List) {
      final list = data['data'] as List;
      return list
          .map((e) => SystemNotificationType.fromJson(e as Map<String, dynamic>))
          .toList();
    }
    return [];
  }

  /// GET /api/v1/notifications/templates
  Future<List<NotificationTemplate>> getTemplates() async {
    final response = await dio.get('/api/v1/notifications/templates');
    final data = response.data;
    if (data is Map && data['data'] is List) {
      final list = data['data'] as List;
      return list
          .map((e) => NotificationTemplate.fromJson(e as Map<String, dynamic>))
          .toList();
    }
    return [];
  }

  /// POST /api/v1/notifications/templates
  Future<NotificationTemplate> createTemplate({
    required String eventType,
    required String name,
    required String title,
    required String body,
    String? imageUrl,
  }) async {
    final response = await dio.post(
      '/api/v1/notifications/templates',
      data: {
        'event_type': eventType,
        'name': name,
        'title': title,
        'body': body,
        if (imageUrl != null && imageUrl.isNotEmpty) 'image_url': imageUrl,
      },
    );
    final data = response.data;
    if (data is Map) {
      final payload = (data['data'] is Map ? data['data'] : data) as Map<String, dynamic>;
      return NotificationTemplate.fromJson(payload);
    }
    throw const FormatException('Unexpected response creating template');
  }

  /// PUT /api/v1/notifications/templates/{id}
  Future<NotificationTemplate> updateTemplate({
    required String id,
    required String name,
    required String title,
    required String body,
    String? imageUrl,
  }) async {
    final response = await dio.put(
      '/api/v1/notifications/templates/$id',
      data: {
        'name': name,
        'title': title,
        'body': body,
        if (imageUrl != null) 'image_url': imageUrl,
      },
    );
    final data = response.data;
    if (data is Map) {
      final payload = (data['data'] is Map ? data['data'] : data) as Map<String, dynamic>;
      return NotificationTemplate.fromJson(payload);
    }
    throw const FormatException('Unexpected response updating template');
  }

  /// POST /api/v1/notifications/manual
  Future<void> sendManualPush({
    required String title,
    required String body,
    String? imageUrl,
    String? deepLink,
  }) async {
    await dio.post(
      '/api/v1/notifications/manual',
      data: {
        'title': title,
        'body': body,
        if (imageUrl != null && imageUrl.isNotEmpty) 'image_url': imageUrl,
        if (deepLink != null && deepLink.isNotEmpty) 'deep_link': deepLink,
      },
    );
  }

  /// POST /api/v1/notifications/test-push
  Future<int> sendTestPush({
    required String eventType,
    required String title,
    required String body,
    String? imageUrl,
  }) async {
    final response = await dio.post(
      '/api/v1/notifications/test-push',
      data: {
        'event_type': eventType,
        'title': title,
        'body': body,
        if (imageUrl != null && imageUrl.isNotEmpty) 'image_url': imageUrl,
      },
    );
    final data = response.data;
    if (data is Map && data['dispatched_count'] is num) {
      return (data['dispatched_count'] as num).toInt();
    }
    return 1;
  }

  /// PATCH /api/v1/notifications/templates/{id}/toggle-active
  Future<NotificationTemplate> toggleTemplateActive(String id) async {
    final response = await dio.patch(
      '/api/v1/notifications/templates/$id/toggle-active',
    );
    final data = response.data;
    if (data is Map) {
      final payload = (data['data'] is Map ? data['data'] : data) as Map<String, dynamic>;
      return NotificationTemplate.fromJson(payload);
    }
    throw const FormatException('Unexpected response toggling template active');
  }

  /// DELETE /api/v1/notifications/templates/{id}
  Future<void> deleteTemplate(String id) async {
    await dio.delete('/api/v1/notifications/templates/$id');
  }

  /// POST /api/v1/notifications/upload-image
  Future<String> uploadImage({
    required String eventType,
    required List<int> bytes,
    required String filename,
  }) async {
    final formData = FormData.fromMap({
      'event_type': eventType,
      'file': MultipartFile.fromBytes(bytes, filename: filename),
    });
    final response = await dio.post(
      '/api/v1/notifications/upload-image',
      data: formData,
    );
    final data = response.data;
    if (data is Map && data['image_url'] != null) {
      return data['image_url'] as String;
    }
    throw const FormatException('Unexpected response uploading image');
  }

  /// GET /api/v1/notifications/logs
  Future<NotificationLogsResponse> getLogs({
    int page = 1,
    int perPage = 20,
    String? eventType,
    String? status,
  }) async {
    final queryParams = <String, dynamic>{
      'page': page,
      'per_page': perPage,
    };
    if (eventType != null && eventType.isNotEmpty) {
      queryParams['event_type'] = eventType;
    }
    if (status != null && status.isNotEmpty && status != 'all') {
      queryParams['status'] = status;
    }

    final response = await dio.get(
      '/api/v1/notifications/logs',
      queryParameters: queryParams,
    );
    final data = response.data;
    if (data is Map) {
      return NotificationLogsResponse.fromJson(data as Map<String, dynamic>);
    }
    return const NotificationLogsResponse(
      data: [],
      page: 1,
      perPage: 20,
      total: 0,
      totalPages: 1,
    );
  }

  /// POST /api/v1/notifications/logs/{id}/read
  Future<void> markAsRead(String id) async {
    await dio.post('/api/v1/notifications/logs/$id/read');
  }

  /// POST /api/v1/notifications/logs/read-all
  Future<int> markAllAsRead() async {
    final response = await dio.post('/api/v1/notifications/logs/read-all');
    final data = response.data;
    if (data is Map && data['marked'] is num) {
      return (data['marked'] as num).toInt();
    }
    return 0;
  }

  /// GET /api/v1/notifications/unread-count
  Future<int> getUnreadCount() async {
    final response = await dio.get('/api/v1/notifications/unread-count');
    final data = response.data;
    if (data is Map && data['unread_count'] is num) {
      return (data['unread_count'] as num).toInt();
    }
    return 0;
  }

  /// GET /api/v1/notifications/detail/{id}
  Future<NotificationLogEntry> getNotificationDetail(String id) async {
    final response = await dio.get('/api/v1/notifications/detail/$id');
    final data = response.data;
    if (data is Map) {
      final payload =
          (data['data'] is Map ? data['data'] : data) as Map<String, dynamic>;
      return NotificationLogEntry.fromJson(payload);
    }
    throw const FormatException('Unexpected response fetching notification detail');
  }

  /// GET /api/v1/notifications/schedules
  Future<List<NotificationSchedule>> getSchedules({
    String? eventType,
    bool? isActive,
  }) async {
    final queryParams = <String, dynamic>{};
    if (eventType != null && eventType.isNotEmpty) {
      queryParams['event_type'] = eventType;
    }
    if (isActive != null) {
      queryParams['is_active'] = isActive ? 'true' : 'false';
    }
    final response = await dio.get(
      '/api/v1/notifications/schedules',
      queryParameters: queryParams,
    );
    final data = response.data;
    if (data is Map && data['data'] is List) {
      final list = data['data'] as List;
      return list
          .map((e) => NotificationSchedule.fromJson(e as Map<String, dynamic>))
          .toList();
    }
    return [];
  }

  /// GET /api/v1/notifications/schedules/{id}
  Future<NotificationSchedule> getSchedule(String id) async {
    final response = await dio.get('/api/v1/notifications/schedules/$id');
    final data = response.data;
    if (data is Map) {
      final payload = (data['data'] is Map ? data['data'] : data) as Map<String, dynamic>;
      return NotificationSchedule.fromJson(payload);
    }
    throw const FormatException('Unexpected response fetching schedule');
  }

  /// POST /api/v1/notifications/schedules
  Future<NotificationSchedule> createSchedule(CreateNotificationScheduleInput input) async {
    final response = await dio.post(
      '/api/v1/notifications/schedules',
      data: input.toJson(),
    );
    final data = response.data;
    if (data is Map) {
      final payload = (data['data'] is Map ? data['data'] : data) as Map<String, dynamic>;
      return NotificationSchedule.fromJson(payload);
    }
    throw const FormatException('Unexpected response creating schedule');
  }

  /// PUT /api/v1/notifications/schedules/{id}
  Future<NotificationSchedule> updateSchedule(String id, UpdateNotificationScheduleInput input) async {
    final response = await dio.put(
      '/api/v1/notifications/schedules/$id',
      data: input.toJson(),
    );
    final data = response.data;
    if (data is Map) {
      final payload = (data['data'] is Map ? data['data'] : data) as Map<String, dynamic>;
      return NotificationSchedule.fromJson(payload);
    }
    throw const FormatException('Unexpected response updating schedule');
  }

  /// PATCH /api/v1/notifications/schedules/{id}/toggle-active
  Future<NotificationSchedule> toggleScheduleActive(String id) async {
    final response = await dio.patch(
      '/api/v1/notifications/schedules/$id/toggle-active',
    );
    final data = response.data;
    if (data is Map) {
      final payload = (data['data'] is Map ? data['data'] : data) as Map<String, dynamic>;
      return NotificationSchedule.fromJson(payload);
    }
    throw const FormatException('Unexpected response toggling schedule active');
  }

  /// DELETE /api/v1/notifications/schedules/{id}
  Future<void> deleteSchedule(String id) async {
    await dio.delete('/api/v1/notifications/schedules/$id');
  }
}

