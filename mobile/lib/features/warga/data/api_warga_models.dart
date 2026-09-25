import 'package:flutter/foundation.dart';

/// API response wrapper for paginated endpoints.
@immutable
class PaginatedResponse<T> {
  final List<T> data;
  final PaginationInfo pagination;

  const PaginatedResponse({required this.data, required this.pagination});

  factory PaginatedResponse.fromJson(Map<String, dynamic> json, T Function(dynamic) fromItem) {
    final raw = json['data'];
    final items = raw is List ? raw.map((e) => fromItem(e)).toList() : <T>[];
    final pag = json['pagination'] as Map<String, dynamic>?;
    return PaginatedResponse(
      data: items,
      pagination: PaginationInfo(
        page: (pag?['page'] as num?)?.toInt() ?? 1,
        pageSize: (pag?['page_size'] as num?)?.toInt() ?? 20,
        total: (pag?['total'] as num?)?.toInt() ?? 0,
        totalPages: (pag?['total_pages'] as num?)?.toInt() ?? 1,
      ),
    );
  }
}

/// Pagination metadata from API.
@immutable
class PaginationInfo {
  final int page;
  final int pageSize;
  final int total;
  final int totalPages;

  const PaginationInfo({required this.page, required this.pageSize, required this.total, required this.totalPages});
}

/// Backend Resident DTO.
@immutable
class BackendResident {
  final String id;
  final String rtId;
  final String? householdId;
  final String fullName;
  final String? phone;
  final String? nik;
  final String? email;
  final String? relationshipToHead;
  final bool isActive;

  const BackendResident({
    required this.id,
    required this.rtId,
    this.householdId,
    required this.fullName,
    this.phone,
    this.nik,
    this.email,
    this.relationshipToHead,
    required this.isActive,
  });

  factory BackendResident.fromJson(Map<String, dynamic> json) {
    return BackendResident(
      id: json['id'] as String? ?? '',
      rtId: json['rt_id'] as String? ?? '',
      householdId: json['household_id'] as String?,
      fullName: json['full_name'] as String? ?? '',
      phone: json['phone'] as String?,
      nik: json['nik'] as String?,
      email: json['email'] as String?,
      relationshipToHead: json['relationship_to_head'] as String?,
      isActive: json['is_active'] as bool? ?? true,
    );
  }
}

/// Backend Household DTO.
@immutable
class BackendHousehold {
  final String id;
  final String rtId;
  final String? houseNumber;
  final String headName;
  final String? address;
  final String? occupancyStatus;
  final bool isActive;

  const BackendHousehold({
    required this.id,
    required this.rtId,
    this.houseNumber,
    required this.headName,
    this.address,
    this.occupancyStatus,
    required this.isActive,
  });

  factory BackendHousehold.fromJson(Map<String, dynamic> json) {
    return BackendHousehold(
      id: json['id'] as String? ?? '',
      rtId: json['rt_id'] as String? ?? '',
      houseNumber: json['house_number'] as String?,
      headName: json['head_name'] as String? ?? '',
      address: json['address'] as String?,
      occupancyStatus: json['occupancy_status'] as String?,
      isActive: json['is_active'] as bool? ?? true,
    );
  }
}

/// Mapped resident for the mobile UI.
@immutable
class MappedResident {
  final String id;
  final String name;
  final String? nik;
  final String houseNumber;
  final bool isHeadOfHousehold;
  final String relationship;
  final String? phoneNumber;
  final String? occupancyStatus;
  final bool isActive;

  const MappedResident({
    required this.id,
    required this.name,
    this.nik,
    this.houseNumber = '',
    required this.isHeadOfHousehold,
    required this.relationship,
    this.phoneNumber,
    this.occupancyStatus,
    this.isActive = true,
  });

  String get maskedNik {
    if (nik == null || nik!.length < 10) return '***';
    return '${nik!.substring(0, 6)}******${nik!.substring(nik!.length - 4)}';
  }

  factory MappedResident.fromBackend(BackendResident resident, BackendHousehold? household) {
    final isHead = resident.relationshipToHead == 'HEAD';
    final relationshipLabel = resident.relationshipToHead;
    return MappedResident(
      id: resident.id,
      name: resident.fullName,
      nik: resident.nik,
      houseNumber: household?.houseNumber ?? '',
      isHeadOfHousehold: isHead,
      relationship: isHead ? (relationshipLabel ?? 'Kepala Keluarga') : (relationshipLabel ?? ''),
      phoneNumber: resident.phone,
      occupancyStatus: household?.occupancyStatus == 'OWNER' ? 'Pemilik' : household?.occupancyStatus == 'TENANT' ? 'Penyewa' : null,
      isActive: resident.isActive,
    );
  }
}
