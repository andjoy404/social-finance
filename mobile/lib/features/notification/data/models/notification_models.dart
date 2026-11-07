import 'package:social_finance/core/models/role.dart';

class SystemNotificationType {
  final String eventType;
  final String name;
  final String trigger;
  final String recipientsRule;
  final String defaultTitle;
  final String defaultBody;
  final List<String> allowedVariables;
  final bool supportsImage;
  final bool supportsTest;

  const SystemNotificationType({
    required this.eventType,
    required this.name,
    required this.trigger,
    required this.recipientsRule,
    required this.defaultTitle,
    required this.defaultBody,
    required this.allowedVariables,
    this.supportsImage = false,
    this.supportsTest = false,
  });

  factory SystemNotificationType.fromJson(Map<String, dynamic> json) {
    return SystemNotificationType(
      eventType: json['event_type'] as String? ?? '',
      name: json['name'] as String? ?? '',
      trigger: json['trigger'] as String? ?? '',
      recipientsRule: json['recipients_rule'] as String? ?? '',
      defaultTitle: json['default_title'] as String? ?? '',
      defaultBody: json['default_body'] as String? ?? '',
      allowedVariables:
          (json['allowed_variables'] as List<dynamic>?)
              ?.map((e) => e.toString())
              .toList() ??
          const [],
      supportsImage: json['supports_image'] as bool? ?? false,
      supportsTest: json['supports_test'] as bool? ?? false,
    );
  }

  Map<String, dynamic> toJson() => {
    'event_type': eventType,
    'name': name,
    'trigger': trigger,
    'recipients_rule': recipientsRule,
    'default_title': defaultTitle,
    'default_body': defaultBody,
    'allowed_variables': allowedVariables,
    'supports_image': supportsImage,
    'supports_test': supportsTest,
  };
}

class NotificationTemplate {
  final String id;
  final String rtId;
  final String eventType;
  final String name;
  final String title;
  final String body;
  final String? imageUrl;
  final bool isActive;
  final DateTime? createdAt;
  final DateTime? updatedAt;

  const NotificationTemplate({
    required this.id,
    required this.rtId,
    required this.eventType,
    required this.name,
    required this.title,
    required this.body,
    this.imageUrl,
    required this.isActive,
    this.createdAt,
    this.updatedAt,
  });

  factory NotificationTemplate.fromJson(Map<String, dynamic> json) {
    return NotificationTemplate(
      id: json['id'] as String? ?? '',
      rtId: json['rt_id'] as String? ?? '',
      eventType: json['event_type'] as String? ?? '',
      name: json['name'] as String? ?? '',
      title: json['title'] as String? ?? '',
      body: json['body'] as String? ?? '',
      imageUrl: json['image_url'] as String?,
      isActive: json['is_active'] as bool? ?? false,
      createdAt: json['created_at'] != null
          ? DateTime.tryParse(json['created_at'].toString())
          : null,
      updatedAt: json['updated_at'] != null
          ? DateTime.tryParse(json['updated_at'].toString())
          : null,
    );
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    'rt_id': rtId,
    'event_type': eventType,
    'name': name,
    'title': title,
    'body': body,
    'image_url': imageUrl,
    'is_active': isActive,
    'created_at': createdAt?.toIso8601String(),
    'updated_at': updatedAt?.toIso8601String(),
  };

  NotificationTemplate copyWith({
    String? id,
    String? rtId,
    String? eventType,
    String? name,
    String? title,
    String? body,
    String? imageUrl,
    bool? isActive,
    DateTime? createdAt,
    DateTime? updatedAt,
  }) {
    return NotificationTemplate(
      id: id ?? this.id,
      rtId: rtId ?? this.rtId,
      eventType: eventType ?? this.eventType,
      name: name ?? this.name,
      title: title ?? this.title,
      body: body ?? this.body,
      imageUrl: imageUrl ?? this.imageUrl,
      isActive: isActive ?? this.isActive,
      createdAt: createdAt ?? this.createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
    );
  }
}

String _parseDeliveryStatus(dynamic value) {
  final status = value?.toString().trim().toUpperCase() ?? '';
  const knownStatuses = {
    'SENT',
    'FAILED',
    'PARTIAL',
    'UNKNOWN',
    'NO_DEVICE',
    'PENDING',
    'APPROVED',
    'REJECTED',
    'RECORDED',
  };
  return knownStatuses.contains(status) ? status : 'UNKNOWN';
}

class NotificationLogEntry {
  final String id;
  final String rtId;
  final String? userId;
  final String? templateId;
  final String eventType;
  final String? dedupKey;
  final String title;
  final String body;
  final String status;
  final String? imageUrl;
  final Map<String, dynamic> variables;
  final Map<String, dynamic>? payload;
  final int recipientsCount;
  final int sentCount;
  final int failedCount;
  final String? historyCategory;
  final String? activityRelation;
  final bool deliveryCountsAvailable;
  final DateTime? createdAt;
  final bool isRead;
  final DateTime? readAt;

  const NotificationLogEntry({
    required this.id,
    required this.rtId,
    this.userId,
    this.templateId,
    required this.eventType,
    this.dedupKey,
    required this.title,
    required this.body,
    this.status = 'UNKNOWN',
    this.imageUrl,
    required this.variables,
    this.payload,
    required this.recipientsCount,
    this.sentCount = 0,
    this.failedCount = 0,
    this.historyCategory,
    this.activityRelation,
    this.deliveryCountsAvailable = false,
    this.createdAt,
    this.isRead = false,
    this.readAt,
  });

  factory NotificationLogEntry.fromJson(Map<String, dynamic> json) {
    return NotificationLogEntry(
      id: json['id'] as String? ?? '',
      rtId: json['rt_id'] as String? ?? '',
      userId: json['user_id'] as String?,
      templateId: json['template_id'] as String?,
      eventType: json['event_type'] as String? ?? '',
      dedupKey: json['dedup_key'] as String?,
      title: json['title'] as String? ?? '',
      body: json['body'] as String? ?? '',
      status: _parseDeliveryStatus(json['status']),
      imageUrl: json['image_url'] as String?,
      variables: json['variables'] is Map
          ? (json['variables'] as Map).cast<String, dynamic>()
          : const {},
      payload: json['payload'] is Map
          ? (json['payload'] as Map).cast<String, dynamic>()
          : null,
      recipientsCount: (json['recipients_count'] as num?)?.toInt() ?? 0,
      sentCount: (json['sent_count'] as num?)?.toInt() ?? 0,
      failedCount: (json['failed_count'] as num?)?.toInt() ?? 0,
      historyCategory: json['history_category'] as String?,
      activityRelation: json['activity_relation'] as String?,
      deliveryCountsAvailable:
          json['delivery_counts_available'] as bool? ?? false,
      createdAt: json['created_at'] != null
          ? DateTime.tryParse(json['created_at'].toString())
          : null,
      isRead: json['is_read'] as bool? ?? false,
      readAt: json['read_at'] != null
          ? DateTime.tryParse(json['read_at'].toString())
          : null,
    );
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    'rt_id': rtId,
    if (userId != null) 'user_id': userId,
    'template_id': templateId,
    'event_type': eventType,
    if (dedupKey != null) 'dedup_key': dedupKey,
    'title': title,
    'body': body,
    'status': status,
    if (imageUrl != null) 'image_url': imageUrl,
    'variables': variables,
    if (payload != null) 'payload': payload,
    'recipients_count': recipientsCount,
    'sent_count': sentCount,
    'failed_count': failedCount,
    if (historyCategory != null) 'history_category': historyCategory,
    if (activityRelation != null) 'activity_relation': activityRelation,
    'delivery_counts_available': deliveryCountsAvailable,
    'created_at': createdAt?.toIso8601String(),
    'is_read': isRead,
    if (readAt != null) 'read_at': readAt?.toIso8601String(),
  };

  NotificationLogEntry copyWith({
    String? id,
    String? rtId,
    String? userId,
    String? templateId,
    String? eventType,
    String? dedupKey,
    String? title,
    String? body,
    String? status,
    String? imageUrl,
    Map<String, dynamic>? variables,
    Map<String, dynamic>? payload,
    int? recipientsCount,
    int? sentCount,
    int? failedCount,
    String? historyCategory,
    String? activityRelation,
    bool? deliveryCountsAvailable,
    DateTime? createdAt,
    bool? isRead,
    DateTime? readAt,
  }) {
    return NotificationLogEntry(
      id: id ?? this.id,
      rtId: rtId ?? this.rtId,
      userId: userId ?? this.userId,
      templateId: templateId ?? this.templateId,
      eventType: eventType ?? this.eventType,
      dedupKey: dedupKey ?? this.dedupKey,
      title: title ?? this.title,
      body: body ?? this.body,
      status: status ?? this.status,
      imageUrl: imageUrl ?? this.imageUrl,
      variables: variables ?? this.variables,
      payload: payload ?? this.payload,
      recipientsCount: recipientsCount ?? this.recipientsCount,
      sentCount: sentCount ?? this.sentCount,
      failedCount: failedCount ?? this.failedCount,
      historyCategory: historyCategory ?? this.historyCategory,
      activityRelation: activityRelation ?? this.activityRelation,
      deliveryCountsAvailable:
          deliveryCountsAvailable ?? this.deliveryCountsAvailable,
      createdAt: createdAt ?? this.createdAt,
      isRead: isRead ?? this.isRead,
      readAt: readAt ?? this.readAt,
    );
  }
}

class ScheduleExecutionSummary {
  final int totalRecipients;
  final int sentCount;
  final int failedCount;
  final int noDeviceCount;
  final String? failureReason;

  const ScheduleExecutionSummary({
    required this.totalRecipients,
    required this.sentCount,
    required this.failedCount,
    required this.noDeviceCount,
    this.failureReason,
  });

  factory ScheduleExecutionSummary.fromJson(Map<String, dynamic> json) {
    return ScheduleExecutionSummary(
      totalRecipients: (json['total_recipients'] as num?)?.toInt() ?? 0,
      sentCount: (json['sent_count'] as num?)?.toInt() ?? 0,
      failedCount: (json['failed_count'] as num?)?.toInt() ?? 0,
      noDeviceCount: (json['no_device_count'] as num?)?.toInt() ?? 0,
      failureReason: json['failure_reason'] as String?,
    );
  }

  Map<String, dynamic> toJson() => {
    'total_recipients': totalRecipients,
    'sent_count': sentCount,
    'failed_count': failedCount,
    'no_device_count': noDeviceCount,
    if (failureReason != null) 'failure_reason': failureReason,
  };
}

/// Jadwal notifikasi generik yang terkelola per RT.
class NotificationSchedule {
  final String id;
  final String rtId;
  final String eventType;
  final String templateId;
  final String name;
  final bool isActive;
  final List<int> sendDays;
  final int sendHour;
  final int sendMinute;
  final String timezone;
  final DateTime? lastExecutedAt;
  final String? lastExecutedStatus;
  final String? lastExecutedError;
  final ScheduleExecutionSummary? lastExecutedSummary;
  final DateTime createdAt;
  final DateTime updatedAt;

  const NotificationSchedule({
    required this.id,
    required this.rtId,
    required this.eventType,
    required this.templateId,
    required this.name,
    required this.isActive,
    required this.sendDays,
    required this.sendHour,
    required this.sendMinute,
    required this.timezone,
    this.lastExecutedAt,
    this.lastExecutedStatus,
    this.lastExecutedError,
    this.lastExecutedSummary,
    required this.createdAt,
    required this.updatedAt,
  });

  factory NotificationSchedule.fromJson(Map<String, dynamic> json) {
    return NotificationSchedule(
      id: json['id'] as String? ?? '',
      rtId: json['rt_id'] as String? ?? '',
      eventType: json['event_type'] as String? ?? '',
      templateId: json['template_id'] as String? ?? '',
      name: json['name'] as String? ?? '',
      isActive: json['is_active'] as bool? ?? false,
      sendDays:
          (json['send_days'] as List<dynamic>?)
              ?.map((e) => (e as num).toInt())
              .toList() ??
          const [],
      sendHour: (json['send_hour'] as num?)?.toInt() ?? 0,
      sendMinute: (json['send_minute'] as num?)?.toInt() ?? 0,
      timezone: json['timezone'] as String? ?? 'Asia/Jakarta',
      lastExecutedAt: json['last_executed_at'] != null
          ? DateTime.tryParse(json['last_executed_at'].toString())
          : null,
      lastExecutedStatus: json['last_executed_status'] as String?,
      lastExecutedError: json['last_executed_error'] as String?,
      lastExecutedSummary: json['last_executed_summary'] != null &&
              json['last_executed_summary'] is Map<String, dynamic>
          ? ScheduleExecutionSummary.fromJson(
              json['last_executed_summary'] as Map<String, dynamic>,
            )
          : null,
      createdAt: json['created_at'] != null
          ? (DateTime.tryParse(json['created_at'].toString()) ?? DateTime.now())
          : DateTime.now(),
      updatedAt: json['updated_at'] != null
          ? (DateTime.tryParse(json['updated_at'].toString()) ?? DateTime.now())
          : DateTime.now(),
    );
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    'rt_id': rtId,
    'event_type': eventType,
    'template_id': templateId,
    'name': name,
    'is_active': isActive,
    'send_days': sendDays,
    'send_hour': sendHour,
    'send_minute': sendMinute,
    'timezone': timezone,
    'last_executed_at': lastExecutedAt?.toIso8601String(),
    'last_executed_status': lastExecutedStatus,
    'last_executed_error': lastExecutedError,
    'last_executed_summary': lastExecutedSummary?.toJson(),
    'created_at': createdAt.toIso8601String(),
    'updated_at': updatedAt.toIso8601String(),
  };

  NotificationSchedule copyWith({
    String? id,
    String? rtId,
    String? eventType,
    String? templateId,
    String? name,
    bool? isActive,
    List<int>? sendDays,
    int? sendHour,
    int? sendMinute,
    String? timezone,
    DateTime? lastExecutedAt,
    String? lastExecutedStatus,
    String? lastExecutedError,
    ScheduleExecutionSummary? lastExecutedSummary,
    DateTime? createdAt,
    DateTime? updatedAt,
  }) {
    return NotificationSchedule(
      id: id ?? this.id,
      rtId: rtId ?? this.rtId,
      eventType: eventType ?? this.eventType,
      templateId: templateId ?? this.templateId,
      name: name ?? this.name,
      isActive: isActive ?? this.isActive,
      sendDays: sendDays ?? this.sendDays,
      sendHour: sendHour ?? this.sendHour,
      sendMinute: sendMinute ?? this.sendMinute,
      timezone: timezone ?? this.timezone,
      lastExecutedAt: lastExecutedAt ?? this.lastExecutedAt,
      lastExecutedStatus: lastExecutedStatus ?? this.lastExecutedStatus,
      lastExecutedError: lastExecutedError ?? this.lastExecutedError,
      lastExecutedSummary: lastExecutedSummary ?? this.lastExecutedSummary,
      createdAt: createdAt ?? this.createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
    );
  }
}

/// Input untuk membuat jadwal notifikasi baru.
class CreateNotificationScheduleInput {
  final String eventType;
  final String templateId;
  final String name;
  final List<int> sendDays;
  final int sendHour;
  final int sendMinute;
  final String timezone;

  const CreateNotificationScheduleInput({
    required this.eventType,
    required this.templateId,
    required this.name,
    required this.sendDays,
    required this.sendHour,
    required this.sendMinute,
    this.timezone = 'Asia/Jakarta',
  });

  Map<String, dynamic> toJson() => {
    'event_type': eventType,
    'template_id': templateId,
    'name': name,
    'send_days': sendDays,
    'send_hour': sendHour,
    'send_minute': sendMinute,
    'timezone': timezone,
  };
}

/// Input untuk mengubah jadwal notifikasi yang ada.
class UpdateNotificationScheduleInput {
  final String? name;
  final bool? isActive;
  final List<int>? sendDays;
  final int? sendHour;
  final int? sendMinute;
  final String? timezone;

  const UpdateNotificationScheduleInput({
    this.name,
    this.isActive,
    this.sendDays,
    this.sendHour,
    this.sendMinute,
    this.timezone,
  });

  Map<String, dynamic> toJson() {
    final map = <String, dynamic>{};
    if (name != null) map['name'] = name;
    if (isActive != null) map['is_active'] = isActive;
    if (sendDays != null) map['send_days'] = sendDays;
    if (sendHour != null) map['send_hour'] = sendHour;
    if (sendMinute != null) map['send_minute'] = sendMinute;
    if (timezone != null) map['timezone'] = timezone;
    return map;
  }
}

class NotificationLogsResponse {
  final List<NotificationLogEntry> data;
  final int page;
  final int perPage;
  final int total;
  final int totalPages;

  const NotificationLogsResponse({
    required this.data,
    required this.page,
    required this.perPage,
    required this.total,
    required this.totalPages,
  });

  factory NotificationLogsResponse.fromJson(Map<String, dynamic> json) {
    final list =
        (json['data'] as List<dynamic>?)
            ?.map(
              (e) => NotificationLogEntry.fromJson(e as Map<String, dynamic>),
            )
            .toList() ??
        const [];
    final pagination = json['pagination'] as Map<String, dynamic>? ?? {};
    return NotificationLogsResponse(
      data: list,
      page: (pagination['page'] as num?)?.toInt() ?? 1,
      perPage:
          (pagination['page_size'] as num?)?.toInt() ??
          (pagination['per_page'] as num?)?.toInt() ??
          20,
      total: (pagination['total'] as num?)?.toInt() ?? list.length,
      totalPages: (pagination['total_pages'] as num?)?.toInt() ?? 1,
    );
  }
}

class WasteTruckDispatchEntry {
  final String id;
  final String eventType;
  final String title;
  final String body;
  final String? imageUrl;
  final String status;
  final String? failureReason;
  final DateTime? createdAt;

  const WasteTruckDispatchEntry({
    required this.id,
    required this.eventType,
    required this.title,
    required this.body,
    this.imageUrl,
    required this.status,
    this.failureReason,
    this.createdAt,
  });

  factory WasteTruckDispatchEntry.fromJson(Map<String, dynamic> json) {
    return WasteTruckDispatchEntry(
      id: json['id'] as String? ?? '',
      eventType: json['event_type'] as String? ?? '',
      title: json['title'] as String? ?? '',
      body: json['body'] as String? ?? '',
      imageUrl: json['image_url'] as String?,
      status: json['status'] as String? ?? 'PENDING',
      failureReason: json['failure_reason'] as String?,
      createdAt: json['created_at'] != null
          ? DateTime.tryParse(json['created_at'].toString())
          : null,
    );
  }
}

class WasteTruckDispatchHistoryResponse {
  final List<WasteTruckDispatchEntry> data;
  final int page;
  final int total;

  const WasteTruckDispatchHistoryResponse({
    required this.data,
    required this.page,
    required this.total,
  });

  factory WasteTruckDispatchHistoryResponse.fromJson(
    Map<String, dynamic> json,
  ) {
    final list =
        (json['data'] as List<dynamic>?)
            ?.map(
              (item) => WasteTruckDispatchEntry.fromJson(
                (item as Map).cast<String, dynamic>(),
              ),
            )
            .toList() ??
        const <WasteTruckDispatchEntry>[];
    final pagination = json['pagination'] is Map
        ? (json['pagination'] as Map).cast<String, dynamic>()
        : const <String, dynamic>{};
    return WasteTruckDispatchHistoryResponse(
      data: list,
      page: (pagination['page'] as num?)?.toInt() ?? 1,
      total: (pagination['total'] as num?)?.toInt() ?? list.length,
    );
  }
}

/// Helper functions to check if the current user has Superadmin or Ketua RT
/// access to Notification Management.
bool isSuperAdminUser(Map<String, dynamic>? user) {
  if (user == null) return false;
  final systemRole = (user['system_role'] ?? '')
      .toString()
      .trim()
      .toLowerCase();
  final role = user['role'];
  final roleStr = (role ?? '').toString().trim().toLowerCase();
  return systemRole == 'super_admin' ||
      role == AppRole.superAdmin ||
      roleStr == 'super_admin' ||
      roleStr == 'approle.superadmin';
}

bool isKetuaRTUser(Map<String, dynamic>? user) {
  if (user == null) return false;
  final jabatan = (user['jabatan'] ?? '').toString().trim().toLowerCase();
  return jabatan == 'ketua' || jabatan == 'ketua_rt' || jabatan == 'ketua rt';
}

bool isPengurusUser(Map<String, dynamic>? user) {
  if (user == null) return false;
  final role = user['role'];
  final roleStr = (role ?? '').toString().trim().toLowerCase();
  return role == AppRole.pengurus || roleStr == 'pengurus';
}

/// Checks if the current user has administrative access to notification management
/// (Super Admin or Ketua RT).
bool canUserAccessNotificationAdmin(Map<String, dynamic>? user) {
  if (isSuperAdminUser(user)) return true;
  final permissions = (user?['effective_permissions'] as List?)
          ?.whereType<String>()
          .toSet() ??
      const <String>{};
  return permissions.contains('notification.manage');
}

/// Helper function to check if the current user has access to Notification Menu.
/// All authenticated users (including Warga) can access notifications.
bool canUserAccessNotificationMenu(Map<String, dynamic>? user) {
  return user != null;
}

/// Model untuk template waste_truck_arrival yang dilihat petugas keamanan.
class WasteTruckTemplate {
  final String id;
  final String eventType;
  final String name;
  final String title;
  final String body;
  final String? imageUrl;
  final bool isActive;
  final List<String> variables;

  const WasteTruckTemplate({
    required this.id,
    required this.eventType,
    required this.name,
    required this.title,
    required this.body,
    this.imageUrl,
    required this.isActive,
    this.variables = const [],
  });

  factory WasteTruckTemplate.fromJson(Map<String, dynamic> json) {
    return WasteTruckTemplate(
      id: json['id'] as String? ?? '',
      eventType: json['event_type'] as String? ?? '',
      name: json['name'] as String? ?? '',
      title: json['title'] as String? ?? '',
      body: json['body'] as String? ?? '',
      imageUrl: json['image_url'] as String?,
      isActive: json['is_active'] as bool? ?? false,
      variables:
          (json['variables'] as List<dynamic>?)
              ?.map((e) => e.toString())
              .toList() ??
          const [],
    );
  }
}
