import 'package:flutter/foundation.dart';

@immutable
class CommunityBanner {
  final String id;
  final String rtId;
  final String title;
  final String description;
  final String? imageUrl;
  final String sourceType;
  final String? sourceId;
  final String? actionUrl;
  final bool isActive;
  final bool isPinned;
  final int priority;
  final int displayOrder;
  final DateTime? startDate;
  final DateTime? endDate;
  final DateTime createdAt;

  const CommunityBanner({
    required this.id,
    required this.rtId,
    required this.title,
    required this.description,
    this.imageUrl,
    this.sourceType = 'custom',
    this.sourceId,
    this.actionUrl,
    this.isActive = true,
    this.isPinned = false,
    this.priority = 0,
    this.displayOrder = 0,
    this.startDate,
    this.endDate,
    required this.createdAt,
  });

  factory CommunityBanner.fromJson(Map<String, dynamic> json) {
    // Backend sends: title, body, image_url, source_type, source_id,
    // deep_link, start_at, end_at, is_pinned, display_order.
    // Accept legacy aliases (action_url, start_date, end_date, description)
    // as a fallback for resilience.
    return CommunityBanner(
      id: json['id']?.toString() ?? '',
      rtId: json['rt_id']?.toString() ?? '',
      title: json['title']?.toString() ?? '',
      description: (json['body'] ?? json['description'])?.toString() ?? '',
      imageUrl: json['image_url'] as String?,
      sourceType: json['source_type']?.toString() ?? 'custom',
      sourceId: json['source_id'] as String?,
      actionUrl: (json['deep_link'] ?? json['action_url']) as String?,
      isActive: json['is_active'] != false,
      isPinned: json['is_pinned'] == true,
      priority: (json['priority'] as num?)?.toInt() ?? 0,
      displayOrder: (json['display_order'] as num?)?.toInt() ?? 0,
      startDate: json['start_at'] != null
          ? DateTime.tryParse(json['start_at'].toString())
          : (json['start_date'] != null
                ? DateTime.tryParse(json['start_date'].toString())
                : null),
      endDate: json['end_at'] != null
          ? DateTime.tryParse(json['end_at'].toString())
          : (json['end_date'] != null
                ? DateTime.tryParse(json['end_date'].toString())
                : null),
      createdAt: json['created_at'] != null
          ? DateTime.tryParse(json['created_at'].toString()) ?? DateTime.now()
          : DateTime.now(),
    );
  }
}

@immutable
class CommunityActivity {
  final String id;
  final String rtId;
  final String title;
  final String description;
  final String location;
  final String? posterUrl;
  final String status; // upcoming, ongoing, completed, cancelled
  final String publicationStatus; // draft, published
  final DateTime startDate;
  final DateTime? endDate;
  final String? budgetPlanned;
  final String? budgetActual;
  final String? picUserId;
  final String? picName;
  final String? picPhone;
  final bool showOnBanner;
  final String? organizer;
  final String? notes;
  final DateTime createdAt;
  final DateTime updatedAt;

  const CommunityActivity({
    required this.id,
    required this.rtId,
    required this.title,
    required this.description,
    required this.location,
    this.posterUrl,
    required this.status,
    required this.publicationStatus,
    required this.startDate,
    this.endDate,
    this.budgetPlanned,
    this.budgetActual,
    this.picUserId,
    this.picName,
    this.picPhone,
    this.showOnBanner = false,
    this.organizer,
    this.notes,
    required this.createdAt,
    required this.updatedAt,
  });

  factory CommunityActivity.fromJson(Map<String, dynamic> json) {
    final startsAt = json['starts_at'] ?? json['start_date'];
    final endsAt = json['ends_at'] ?? json['end_date'];
    return CommunityActivity(
      id: json['id']?.toString() ?? '',
      rtId: json['rt_id']?.toString() ?? '',
      title: json['title']?.toString() ?? '',
      description: json['description']?.toString() ?? '',
      location: json['location']?.toString() ?? '',
      posterUrl: json['poster_url'] as String?,
      status: json['status']?.toString() ?? 'upcoming',
      publicationStatus: json['publication_status']?.toString() ?? 'draft',
      startDate: startsAt != null
          ? DateTime.tryParse(startsAt.toString()) ?? DateTime.now()
          : DateTime.now(),
      endDate: endsAt != null ? DateTime.tryParse(endsAt.toString()) : null,
      budgetPlanned: json['budget_planned']?.toString(),
      budgetActual: json['budget_actual']?.toString(),
      picUserId: json['pic_user_id'] as String?,
      picName: json['pic_name'] as String?,
      picPhone: json['pic_phone'] as String?,
      showOnBanner: json['show_on_banner'] == true,
      organizer: json['organizer'] as String?,
      notes: json['notes'] as String?,
      createdAt: json['created_at'] != null
          ? DateTime.tryParse(json['created_at'].toString()) ?? DateTime.now()
          : DateTime.now(),
      updatedAt: json['updated_at'] != null
          ? DateTime.tryParse(json['updated_at'].toString()) ?? DateTime.now()
          : DateTime.now(),
    );
  }
}

@immutable
class CommunityAnnouncement {
  final String id;
  final String rtId;
  final String title;
  final String body;
  final String? imageUrl;
  final String? deepLink;
  final String? templateId;
  final DateTime? scheduledStart;
  final DateTime? scheduledEnd;
  final int priority;
  final int displayOrder;
  final String status; // draft, scheduled, active, expired, archived
  final DateTime? publishedAt;
  final DateTime createdAt;
  final DateTime updatedAt;

  const CommunityAnnouncement({
    required this.id,
    required this.rtId,
    required this.title,
    required this.body,
    this.imageUrl,
    this.deepLink,
    this.templateId,
    this.scheduledStart,
    this.scheduledEnd,
    required this.priority,
    required this.displayOrder,
    required this.status,
    this.publishedAt,
    required this.createdAt,
    required this.updatedAt,
  });

  factory CommunityAnnouncement.fromJson(Map<String, dynamic> json) {
    return CommunityAnnouncement(
      id: json['id']?.toString() ?? '',
      rtId: json['rt_id']?.toString() ?? '',
      title: json['title']?.toString() ?? '',
      body: json['body']?.toString() ?? '',
      imageUrl: json['image_url'] as String?,
      deepLink: json['deep_link'] as String?,
      templateId: json['template_id'] as String?,
      scheduledStart: json['scheduled_start'] != null
          ? DateTime.tryParse(json['scheduled_start'].toString())
          : null,
      scheduledEnd: json['scheduled_end'] != null
          ? DateTime.tryParse(json['scheduled_end'].toString())
          : null,
      priority: (json['priority'] as num?)?.toInt() ?? 1,
      displayOrder: (json['display_order'] as num?)?.toInt() ?? 0,
      status: json['status']?.toString() ?? 'draft',
      publishedAt: json['published_at'] != null
          ? DateTime.tryParse(json['published_at'].toString())
          : null,
      createdAt: json['created_at'] != null
          ? DateTime.tryParse(json['created_at'].toString()) ?? DateTime.now()
          : DateTime.now(),
      updatedAt: json['updated_at'] != null
          ? DateTime.tryParse(json['updated_at'].toString()) ?? DateTime.now()
          : DateTime.now(),
    );
  }
}

@immutable
class BannerTemplate {
  final String id;
  final String? rtId;
  final String name;
  final String category;
  final String titleTemplate;
  final String bodyTemplate;
  final String? imageUrl;
  final int defaultPriority;
  final bool isSystem;
  final bool isActive;

  const BannerTemplate({
    required this.id,
    this.rtId,
    required this.name,
    required this.category,
    required this.titleTemplate,
    required this.bodyTemplate,
    this.imageUrl,
    required this.defaultPriority,
    this.isSystem = false,
    this.isActive = true,
  });

  factory BannerTemplate.fromJson(Map<String, dynamic> json) {
    return BannerTemplate(
      id: json['id']?.toString() ?? '',
      rtId: json['rt_id'] as String?,
      name: json['name']?.toString() ?? '',
      category: json['category']?.toString() ?? 'general',
      titleTemplate: json['title_template']?.toString() ?? '',
      bodyTemplate: json['body_template']?.toString() ?? '',
      imageUrl: json['image_url'] as String?,
      defaultPriority: (json['default_priority'] as num?)?.toInt() ?? 1,
      isSystem: json['is_system'] == true,
      isActive: json['is_active'] != false,
    );
  }
}

@immutable
class OrgMember {
  final String userId;
  final String membershipId;
  final String name;
  final String jabatan;
  final String? phone;
  final int level;

  const OrgMember({
    required this.userId,
    this.membershipId = '',
    required this.name,
    required this.jabatan,
    this.phone,
    this.level = 1,
  });

  factory OrgMember.fromJson(Map<String, dynamic> json) {
    return OrgMember(
      userId: json['user_id']?.toString() ?? '',
      membershipId: json['membership_id']?.toString() ?? '',
      name: (json['full_name'] ?? json['name'])?.toString() ?? '',
      jabatan: json['jabatan']?.toString() ?? '',
      phone: json['phone'] as String?,
      level: (json['level'] as num?)?.toInt() ?? 1,
    );
  }
}

@immutable
class OrgNode {
  final String position;
  final String positionLabel;
  final int level;
  final String parentPosition;
  final List<OrgMember> members;

  const OrgNode({
    required this.position,
    required this.positionLabel,
    required this.level,
    this.parentPosition = '',
    required this.members,
  });

  factory OrgNode.fromJson(Map<String, dynamic> json) {
    final rawMembers = json['members'] as List? ?? [];
    return OrgNode(
      position: (json['position'] ?? json['key'])?.toString() ?? '',
      positionLabel:
          (json['position_label'] ?? json['title'])?.toString() ?? '',
      level: (json['level'] as num?)?.toInt() ?? 1,
      parentPosition:
          (json['parent_position'] ?? json['parent'])?.toString() ?? '',
      members: rawMembers
          .whereType<Map<String, dynamic>>()
          .map((m) => OrgMember.fromJson(m))
          .toList(),
    );
  }
}

@immutable
class OrganizationStructure {
  final String rtId;
  final String rtName;
  final String rtNumber;
  final String rwNumber;
  final List<OrgNode> nodes;

  const OrganizationStructure({
    this.rtId = '',
    this.rtName = '',
    this.rtNumber = '',
    this.rwNumber = '',
    this.nodes = const [],
  });

  /// Nodes grouped by level, ordered ascending.
  Map<int, List<OrgNode>> get levels {
    final grouped = <int, List<OrgNode>>{};
    for (final node in nodes) {
      grouped.putIfAbsent(node.level, () => <OrgNode>[]).add(node);
    }
    return grouped;
  }

  factory OrganizationStructure.fromJson(Map<String, dynamic> json) {
    // Backend sends { rt_id, rt_name, rt_number, rw_number, nodes: [...] }.
    // Accept the legacy { levels: { "1": [...] } } shape as a fallback.
    final rawNodes = json['nodes'] as List?;
    if (rawNodes != null) {
      return OrganizationStructure(
        rtId: json['rt_id']?.toString() ?? '',
        rtName: json['rt_name']?.toString() ?? '',
        rtNumber: json['rt_number']?.toString() ?? '',
        rwNumber: json['rw_number']?.toString() ?? '',
        nodes: rawNodes
            .whereType<Map<String, dynamic>>()
            .map((n) => OrgNode.fromJson(n))
            .toList(),
      );
    }

    final legacyNodes = <OrgNode>[];
    final rawLevels = json['levels'] as Map<String, dynamic>? ?? {};
    rawLevels.forEach((key, val) {
      final lvl = int.tryParse(key) ?? 1;
      if (val is List) {
        for (final item in val.whereType<Map<String, dynamic>>()) {
          legacyNodes.add(OrgNode.fromJson({...item, 'level': lvl}));
        }
      }
    });

    return OrganizationStructure(nodes: legacyNodes);
  }
}

@immutable
class SecurityOfficer {
  final String id;
  final String name;
  final String? phone;
  final String shift;
  final String? shiftStartTime;
  final String? shiftEndTime;

  const SecurityOfficer({
    required this.id,
    required this.name,
    this.phone,
    required this.shift,
    this.shiftStartTime,
    this.shiftEndTime,
  });

  factory SecurityOfficer.fromJson(Map<String, dynamic> json) {
    return SecurityOfficer(
      id: json['user_id']?.toString() ?? json['id']?.toString() ?? '',
      name: (json['full_name'] ?? json['name'])?.toString() ?? '',
      phone: (json['phone_number'] ?? json['phone']) as String?,
      shift: (json['shift_name'] ?? json['shift'])?.toString() ?? '',
      shiftStartTime:
          (json['shift_start'] ?? json['shift_start_time']) as String?,
      shiftEndTime: (json['shift_end'] ?? json['shift_end_time']) as String?,
    );
  }
}

@immutable
class SecurityOnDutyInfo {
  final bool isAvailable;
  final List<SecurityOfficer> officers;
  final String message;
  final String subtext;

  const SecurityOnDutyInfo({
    required this.isAvailable,
    required this.officers,
    required this.message,
    required this.subtext,
  });

  factory SecurityOnDutyInfo.fromJson(Map<String, dynamic> json) {
    final rawOfficers = json['officers'] as List? ?? [];
    return SecurityOnDutyInfo(
      isAvailable: json['is_available'] == true,
      officers: rawOfficers
          .whereType<Map<String, dynamic>>()
          .map((o) => SecurityOfficer.fromJson(o))
          .toList(),
      message:
          json['message']?.toString() ??
          'Belum ada petugas aktif / jadwal shift saat ini',
      subtext:
          json['subtext']?.toString() ??
          'Data shift terintegrasi langsung saat Modul Keamanan berjalan',
    );
  }
}

@immutable
class HomeDashboardData {
  final String greeting;
  final String userName;
  final String userRole;
  final String userJabatan;
  final String rt;
  final String rw;
  final String rtName;
  final List<CommunityBanner> banners;
  final SecurityOnDutyInfo security;
  final OrganizationStructure organization;
  final List<CommunityActivity> upcomingActivities;

  const HomeDashboardData({
    required this.greeting,
    required this.userName,
    this.userRole = '',
    this.userJabatan = '',
    required this.rt,
    required this.rw,
    this.rtName = '',
    required this.banners,
    required this.security,
    required this.organization,
    required this.upcomingActivities,
  });

  factory HomeDashboardData.fromJson(Map<String, dynamic> json) {
    final rawBanners = json['banners'] as List? ?? [];
    final rawActivities = json['upcoming_activities'] as List? ?? [];
    // Backend sends 'security_on_duty'; accept 'security' as fallback.
    final rawSecurity = json['security_on_duty'] ?? json['security'];

    return HomeDashboardData(
      greeting: json['greeting']?.toString() ?? 'Selamat datang,',
      userName:
          (json['user_full_name'] ?? json['user_name'])?.toString() ??
          'Pengguna',
      userRole: json['user_role']?.toString() ?? '',
      userJabatan: json['user_jabatan']?.toString() ?? '',
      rt: (json['rt_number'] ?? json['rt'])?.toString() ?? '',
      rw: (json['rw_number'] ?? json['rw'])?.toString() ?? '',
      rtName: json['rt_name']?.toString() ?? '',
      banners: rawBanners
          .whereType<Map<String, dynamic>>()
          .map((b) => CommunityBanner.fromJson(b))
          .toList(),
      security: rawSecurity is Map<String, dynamic>
          ? SecurityOnDutyInfo.fromJson(rawSecurity)
          : const SecurityOnDutyInfo(
              isAvailable: false,
              officers: [],
              message: 'Belum ada petugas aktif / jadwal shift saat ini',
              subtext:
                  'Data shift terintegrasi langsung saat Modul Keamanan berjalan',
            ),
      organization: json['organization'] is Map<String, dynamic>
          ? OrganizationStructure.fromJson(
              json['organization'] as Map<String, dynamic>,
            )
          : const OrganizationStructure(),
      upcomingActivities: rawActivities
          .whereType<Map<String, dynamic>>()
          .map((a) => CommunityActivity.fromJson(a))
          .toList(),
    );
  }
}
