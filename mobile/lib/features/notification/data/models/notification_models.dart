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
      allowedVariables: (json['allowed_variables'] as List<dynamic>?)
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

class NotificationLogEntry {
  final String id;
  final String rtId;
  final String? templateId;
  final String eventType;
  final String title;
  final String body;
  final String? imageUrl;
  final Map<String, dynamic> variables;
  final int recipientsCount;
  final DateTime? createdAt;
  final bool isRead;
  final DateTime? readAt;

  const NotificationLogEntry({
    required this.id,
    required this.rtId,
    this.templateId,
    required this.eventType,
    required this.title,
    required this.body,
    this.imageUrl,
    required this.variables,
    required this.recipientsCount,
    this.createdAt,
    this.isRead = false,
    this.readAt,
  });

  factory NotificationLogEntry.fromJson(Map<String, dynamic> json) {
    return NotificationLogEntry(
      id: json['id'] as String? ?? '',
      rtId: json['rt_id'] as String? ?? '',
      templateId: json['template_id'] as String?,
      eventType: json['event_type'] as String? ?? '',
      title: json['title'] as String? ?? '',
      body: json['body'] as String? ?? '',
      imageUrl: json['image_url'] as String?,
      variables: json['variables'] is Map
          ? (json['variables'] as Map).cast<String, dynamic>()
          : const {},
      recipientsCount: (json['recipients_count'] as num?)?.toInt() ?? 0,
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
        'template_id': templateId,
        'event_type': eventType,
        'title': title,
        'body': body,
        if (imageUrl != null) 'image_url': imageUrl,
        'variables': variables,
        'recipients_count': recipientsCount,
        'created_at': createdAt?.toIso8601String(),
        'is_read': isRead,
        if (readAt != null) 'read_at': readAt?.toIso8601String(),
      };

  NotificationLogEntry copyWith({
    String? id,
    String? rtId,
    String? templateId,
    String? eventType,
    String? title,
    String? body,
    String? imageUrl,
    Map<String, dynamic>? variables,
    int? recipientsCount,
    DateTime? createdAt,
    bool? isRead,
    DateTime? readAt,
  }) {
    return NotificationLogEntry(
      id: id ?? this.id,
      rtId: rtId ?? this.rtId,
      templateId: templateId ?? this.templateId,
      eventType: eventType ?? this.eventType,
      title: title ?? this.title,
      body: body ?? this.body,
      imageUrl: imageUrl ?? this.imageUrl,
      variables: variables ?? this.variables,
      recipientsCount: recipientsCount ?? this.recipientsCount,
      createdAt: createdAt ?? this.createdAt,
      isRead: isRead ?? this.isRead,
      readAt: readAt ?? this.readAt,
    );
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
    final list = (json['data'] as List<dynamic>?)
            ?.map((e) => NotificationLogEntry.fromJson(e as Map<String, dynamic>))
            .toList() ??
        const [];
    final pagination = json['pagination'] as Map<String, dynamic>? ?? {};
    return NotificationLogsResponse(
      data: list,
      page: (pagination['page'] as num?)?.toInt() ?? 1,
      perPage: (pagination['per_page'] as num?)?.toInt() ?? 20,
      total: (pagination['total'] as num?)?.toInt() ?? list.length,
      totalPages: (pagination['total_pages'] as num?)?.toInt() ?? 1,
    );
  }
}

/// Helper functions to check if the current user has Superadmin or Ketua RT
/// access to Notification Management.
bool isSuperAdminUser(Map<String, dynamic>? user) {
  if (user == null) return false;
  final systemRole =
      (user['system_role'] ?? '').toString().trim().toLowerCase();
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
  return isSuperAdminUser(user) || isKetuaRTUser(user);
}

/// Helper function to check if the current user has access to Notification Menu.
/// All authenticated users (including Warga) can access notifications.
bool canUserAccessNotificationMenu(Map<String, dynamic>? user) {
  return user != null;
}


