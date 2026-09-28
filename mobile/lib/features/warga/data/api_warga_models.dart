import 'package:flutter/foundation.dart';
import 'package:flutter/painting.dart';

import 'package:social_finance/core/theme/app_colors.dart';

/// API response wrapper for paginated endpoints.
@immutable
class PaginatedResponse<T> {
  final List<T> data;
  final PaginationInfo pagination;

  const PaginatedResponse({required this.data, required this.pagination});

  factory PaginatedResponse.fromJson(
    Map<String, dynamic> json,
    T Function(dynamic) fromItem,
  ) {
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

  const PaginationInfo({
    required this.page,
    required this.pageSize,
    required this.total,
    required this.totalPages,
  });
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
  final String? rtNumber;
  final int? rw;
  final String? rtName;
  final String? jabatan;

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
    this.rtNumber,
    this.rw,
    this.rtName,
    this.jabatan,
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
      rtNumber: json['rt_number'] as String?,
      rw: (json['rw'] as num?)?.toInt(),
      rtName: json['rt_name'] as String?,
      jabatan: json['jabatan'] as String?,
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

/// Create request for a regular household (with resident data).
@immutable
class CreateHouseholdRequest {
  final String houseNumber;
  final String headName;
  final String nik;
  final String phone;
  final String email;
  final String occupancyStatus;
  final String startDate;
  final String? address;

  const CreateHouseholdRequest({
    required this.houseNumber,
    required this.headName,
    required this.nik,
    required this.phone,
    required this.email,
    required this.occupancyStatus,
    required this.startDate,
    this.address,
  });

  Map<String, dynamic> toJson() {
    final data = <String, dynamic>{
      'house_number': houseNumber,
      'head_name': headName,
      'nik': nik,
      'phone': phone,
      'email': email,
      'occupancy_status': occupancyStatus,
      'start_date': startDate,
    };
    if (address != null) data['address'] = address!;
    return data;
  }
}

/// Update request for a household (all fields optional).
@immutable
class UpdateHouseholdRequest {
  final String? houseNumber;
  final String? headName;
  final String? nik;
  final String? phone;
  final String? email;
  final String? occupancyStatus;
  final String? startDate;
  final String? address;

  const UpdateHouseholdRequest({
    this.houseNumber,
    this.headName,
    this.nik,
    this.phone,
    this.email,
    this.occupancyStatus,
    this.startDate,
    this.address,
  });

  Map<String, dynamic> toJson() {
    final data = <String, dynamic>{};
    if (houseNumber != null) data['house_number'] = houseNumber;
    if (headName != null) data['head_name'] = headName;
    if (nik != null) data['nik'] = nik;
    if (phone != null) data['phone'] = phone;
    if (email != null) data['email'] = email;
    if (occupancyStatus != null) data['occupancy_status'] = occupancyStatus;
    if (startDate != null) data['start_date'] = startDate;
    if (address != null) data['address'] = address;
    return data;
  }
}

/// Create request for a special resident (petugas khusus).
@immutable
class CreateSpecialResidentRequest {
  final String jabatan;
  final String fullName;
  final String nik;
  final String phone;
  final String email;
  final bool isActive;

  const CreateSpecialResidentRequest({
    required this.jabatan,
    required this.fullName,
    required this.nik,
    required this.phone,
    required this.email,
    required this.isActive,
  });

  Map<String, dynamic> toJson() {
    return {
      'jabatan': jabatan,
      'full_name': fullName,
      'nik': nik,
      'phone': phone,
      'email': email,
      'is_active': isActive,
    };
  }
}

/// Update request for a special resident (all fields optional).
@immutable
class UpdateSpecialResidentRequest {
  final String? fullName;
  final String? nik;
  final String? phone;
  final String? email;
  final String? jabatan;
  final bool? isActive;

  const UpdateSpecialResidentRequest({
    this.fullName,
    this.nik,
    this.phone,
    this.email,
    this.jabatan,
    this.isActive,
  });

  Map<String, dynamic> toJson() {
    final data = <String, dynamic>{};
    if (fullName != null) data['full_name'] = fullName;
    if (nik != null) data['nik'] = nik;
    if (phone != null) data['phone'] = phone;
    if (email != null) data['email'] = email;
    if (jabatan != null) data['jabatan'] = jabatan;
    if (isActive != null) data['is_active'] = isActive;
    return data;
  }
}

/// Mapped resident for the mobile UI.
@immutable
class MappedResident {
  final String id;
  final String? householdId;
  final String name;
  final String? nik;
  final String houseNumber;
  final bool isHeadOfHousehold;
  final String relationship;
  final String? phoneNumber;
  final String? email;
  final String? address;
  final String? occupancyStatus;
  final bool isActive;
  final String? rtNumber;
  final int? rw;
  final String? rtName;
  final String? jabatan;

  const MappedResident({
    required this.id,
    this.householdId,
    required this.name,
    this.nik,
    this.houseNumber = '',
    required this.isHeadOfHousehold,
    required this.relationship,
    this.phoneNumber,
    this.email,
    this.address,
    this.occupancyStatus,
    this.isActive = true,
    this.rtNumber,
    this.rw,
    this.rtName,
    this.jabatan,
  });

  String get maskedNik {
    if (nik == null || nik!.length < 10) return '***';
    return '${nik!.substring(0, 6)}******${nik!.substring(nik!.length - 4)}';
  }

  /// Formatted RT/RW and Alamat text, e.g. "RT 03 · RW 16 · Jl. Mawar No. 1".
  String? get formattedRtRwAlamat {
    final parts = <String>[];
    if (rtNumber != null && rtNumber!.isNotEmpty) {
      parts.add('RT $rtNumber');
    }
    if (rw != null) {
      parts.add('RW $rw');
    }
    if (address != null && address!.isNotEmpty) {
      parts.add(address!);
    } else if (rtName != null && rtName!.isNotEmpty) {
      parts.add(rtName!);
    }
    if (parts.isEmpty) return null;
    return parts.join(' · ');
  }

  /// Formatted RT/RW badge text, e.g. "RT 03 · RW 16 · Wisma Rukun Tunggal".
  String? get formattedRtRw {
    final parts = <String>[];
    if (rtNumber != null && rtNumber!.isNotEmpty) {
      parts.add('RT $rtNumber');
    }
    if (rw != null) {
      parts.add('RW $rw');
    }
    if (rtName != null && rtName!.isNotEmpty) {
      parts.add(rtName!);
    }
    if (parts.isEmpty) return null;
    return parts.join(' · ');
  }

  /// User-facing relationship label adhering to Social Finance rules:
  /// - HEAD -> Kepala Keluarga
  /// - CHILD -> Kerabat
  /// - SPOUSE -> Keluarga
  String get displayRelationship {
    if (isHeadOfHousehold) return 'Kepala Keluarga';
    final upper = relationship.toUpperCase();
    if (upper == 'CHILD') return 'Kerabat';
    if (upper == 'SPOUSE') return 'Keluarga';
    if (upper == 'HEAD') return 'Kepala Keluarga';
    if (relationship.isNotEmpty) return relationship;
    return 'Keluarga';
  }

  /// Badge background color for relationship
  /// Simplified: uses accent color for all relationship types
  /// This matches Web behavior which uses semantic accent color
  Color get relationshipColor {
    return AppColors.accent;
  }

  /// User-facing jabatan label (Indonesian position name).
  String? get displayJabatan {
    if (jabatan == null || jabatan!.isEmpty) return null;
    switch (jabatan!) {
      case 'ketua':
        return 'Ketua RT';
      case 'wakil_ketua':
        return 'Wakil Ketua RT';
      case 'sekretaris':
        return 'Sekretaris RT';
      case 'bendahara':
        return 'Bendahara RT';
      case 'keamanan':
        return 'Seksi Keamanan';
      case 'sosial':
        return 'Seksi Sosial';
      case 'kebersihan_pembangunan':
        return 'Seksi Kebersihan dan Pembangunan';
      default:
        return jabatan;
    }
  }

  /// Color for jabatan badge based on position.
  Color get jabatanColor {
    if (jabatan == null || jabatan!.isEmpty) return AppColors.accent;
    switch (jabatan!) {
      case 'ketua':
        return AppColors.accent;
      case 'wakil_ketua':
        return AppColors.info;
      case 'sekretaris':
        return AppColors.success;
      case 'bendahara':
        return AppColors.warning;
      case 'keamanan':
        return AppColors.danger;
      case 'kebersihan_pembangunan':
        return AppColors.info;
      case 'sosial':
        return AppColors.warning;
      default:
        return AppColors.accent;
    }
  }

  bool get isNeutralJabatan => jabatan == 'sosial';

  factory MappedResident.fromBackend(
    BackendResident resident,
    BackendHousehold? household,
  ) {
    final isHead = resident.relationshipToHead == 'HEAD';
    final relationshipLabel = resident.relationshipToHead;
    final mappedJabatan = resident.jabatan;
    return MappedResident(
      id: resident.id,
      householdId: resident.householdId,
      name: resident.fullName,
      nik: resident.nik,
      houseNumber: household?.houseNumber ?? '',
      isHeadOfHousehold: isHead,
      relationship: isHead
          ? (relationshipLabel ?? 'Kepala Keluarga')
          : (relationshipLabel ?? ''),
      phoneNumber: resident.phone,
      email: resident.email,
      address: household?.address,
      occupancyStatus: household?.occupancyStatus == 'OWNER'
          ? 'Pemilik'
          : household?.occupancyStatus == 'TENANT'
          ? 'Penyewa'
          : null,
      isActive: resident.isActive,
      rtNumber: resident.rtNumber,
      rw: resident.rw,
      rtName: resident.rtName,
      jabatan: mappedJabatan,
    );
  }
}
