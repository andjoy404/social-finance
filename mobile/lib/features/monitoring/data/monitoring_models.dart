/// Model untuk satu rumah dalam daftar monitoring kebersihan.
/// Hanya berisi data yang boleh ditampilkan kepada petugas kebersihan:
/// nomor rumah, catatan operasi, dan metadata catatan.
/// Tidak boleh berisi nama warga, NIK, email, telepon, nominal tagihan,
/// atau detail transaksi apapun.
class MonitoringHousehold {
  final String householdOccupancyId;
  final String houseNumber;
  final String? note;
  final String? noteCreatedAt;
  final String? noteCreatedBy;
  final String? noteUpdatedAt;

  const MonitoringHousehold({
    required this.householdOccupancyId,
    required this.houseNumber,
    this.note,
    this.noteCreatedAt,
    this.noteCreatedBy,
    this.noteUpdatedAt,
  });

  factory MonitoringHousehold.fromJson(Map<String, dynamic> json) {
    return MonitoringHousehold(
      householdOccupancyId: json['household_occupancy_id'] as String? ?? '',
      houseNumber: json['house_number'] as String? ?? '',
      note: json['note'] as String?,
      noteCreatedAt: json['note_created_at'] as String?,
      noteCreatedBy: json['note_created_by'] as String?,
      noteUpdatedAt: json['note_updated_at'] as String?,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'household_occupancy_id': householdOccupancyId,
      'house_number': houseNumber,
      if (note != null) 'note': note,
      if (noteCreatedAt != null) 'note_created_at': noteCreatedAt,
      if (noteCreatedBy != null) 'note_created_by': noteCreatedBy,
      if (noteUpdatedAt != null) 'note_updated_at': noteUpdatedAt,
    };
  }
}

/// Response untuk daftar monitoring rumah tangga.
class MonitoringListResponse {
  final List<MonitoringHousehold> households;
  final int total;
  final String evalDate;

  const MonitoringListResponse({
    required this.households,
    required this.total,
    required this.evalDate,
  });

  factory MonitoringListResponse.fromJson(Map<String, dynamic> json) {
    final households = (json['data'] ?? json['households']) as List<dynamic>?;
    return MonitoringListResponse(
      households:
          households
              ?.map(
                (e) => MonitoringHousehold.fromJson(e as Map<String, dynamic>),
              )
              .toList() ??
          const [],
      total: (json['total'] as num?)?.toInt() ?? 0,
      evalDate: json['eval_date'] as String? ?? '',
    );
  }
}

/// Model untuk catatan lapangan satu rumah.
class MonitoringNote {
  final String householdOccupancyId;
  final String houseNumber;
  final String note;
  final String? createdBy;
  final String? updatedBy;
  final String? createdAt;
  final String? updatedAt;

  const MonitoringNote({
    required this.householdOccupancyId,
    required this.houseNumber,
    required this.note,
    this.createdBy,
    this.updatedBy,
    this.createdAt,
    this.updatedAt,
  });

  factory MonitoringNote.fromJson(Map<String, dynamic> json) {
    return MonitoringNote(
      householdOccupancyId: json['household_occupancy_id'] as String? ?? '',
      houseNumber: json['house_number'] as String? ?? '',
      note: json['note'] as String? ?? '',
      createdBy: json['created_by'] as String?,
      updatedBy: json['updated_by'] as String?,
      createdAt: json['created_at'] as String?,
      updatedAt: json['updated_at'] as String?,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'household_occupancy_id': householdOccupancyId,
      'house_number': houseNumber,
      'note': note,
      if (createdBy != null) 'created_by': createdBy,
      if (updatedBy != null) 'updated_by': updatedBy,
      if (createdAt != null) 'created_at': createdAt,
      if (updatedAt != null) 'updated_at': updatedAt,
    };
  }
}
