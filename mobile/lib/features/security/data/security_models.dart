import 'package:social_finance/features/monitoring/data/monitoring_models.dart';

class SecurityOfficer {
  final String userId;
  final String membershipId;
  final String rtId;
  final String fullName;
  final String email;
  final String? phone;
  final bool isActive;

  const SecurityOfficer({
    required this.userId,
    required this.membershipId,
    required this.rtId,
    required this.fullName,
    required this.email,
    this.phone,
    required this.isActive,
  });

  factory SecurityOfficer.fromJson(Map<String, dynamic> json) {
    return SecurityOfficer(
      userId: json['user_id']?.toString() ?? '',
      membershipId: json['membership_id']?.toString() ?? '',
      rtId: json['rt_id']?.toString() ?? '',
      fullName: json['full_name']?.toString() ?? '',
      email: json['email']?.toString() ?? '',
      phone: json['phone']?.toString(),
      isActive: json['is_active'] as bool? ?? true,
    );
  }
}

class SecuritySchedule {
  final String id;
  final String assignmentId;
  final String assignmentStartDate;
  final String assignmentEndDate;
  final String rtId;
  final String userId;
  final String officerName;
  final String membershipId;
  final String shiftConfigId;
  final int shiftVersion;
  final String shiftDate;
  final String shiftName;
  final String startTime;
  final String endTime;
  final bool crossesMidnight;
  final int latenessToleranceMinutes;
  final DateTime? scheduledStartAt;
  final DateTime? scheduledEndAt;
  final String? notes;
  final String status;
  final String? attendanceStatus;
  final String? attendanceStatusLabel;
  final DateTime? checkedInAt;
  final DateTime? checkedOutAt;

  const SecuritySchedule({
    required this.id,
    this.assignmentId = '',
    this.assignmentStartDate = '',
    this.assignmentEndDate = '',
    required this.rtId,
    required this.userId,
    required this.officerName,
    required this.membershipId,
    required this.shiftConfigId,
    required this.shiftVersion,
    required this.shiftDate,
    required this.shiftName,
    required this.startTime,
    required this.endTime,
    required this.crossesMidnight,
    required this.latenessToleranceMinutes,
    this.scheduledStartAt,
    this.scheduledEndAt,
    this.notes,
    required this.status,
    this.attendanceStatus,
    this.attendanceStatusLabel,
    this.checkedInAt,
    this.checkedOutAt,
  });

  factory SecuritySchedule.fromJson(Map<String, dynamic> json) {
    return SecuritySchedule(
      id: json['id']?.toString() ?? '',
      assignmentId: json['assignment_id']?.toString() ?? '',
      assignmentStartDate: json['assignment_start_date']?.toString() ?? '',
      assignmentEndDate: json['assignment_end_date']?.toString() ?? '',
      rtId: json['rt_id']?.toString() ?? '',
      userId: json['user_id']?.toString() ?? '',
      officerName: json['officer_name']?.toString() ?? '',
      membershipId: json['membership_id']?.toString() ?? '',
      shiftConfigId: json['shift_config_id']?.toString() ?? '',
      shiftVersion: (json['shift_version'] as num?)?.toInt() ?? 1,
      shiftDate: json['shift_date']?.toString() ?? '',
      shiftName: json['shift_name']?.toString() ?? '',
      startTime: json['start_time']?.toString() ?? '',
      endTime: json['end_time']?.toString() ?? '',
      crossesMidnight: json['crosses_midnight'] as bool? ?? false,
      latenessToleranceMinutes:
          (json['lateness_tolerance_minutes'] as num?)?.toInt() ?? 0,
      scheduledStartAt: _tryParseDateTime(json['scheduled_start_at']),
      scheduledEndAt: _tryParseDateTime(json['scheduled_end_at']),
      notes: json['notes']?.toString(),
      status: json['status']?.toString() ?? '',
      attendanceStatus: json['attendance_status']?.toString(),
      attendanceStatusLabel: json['attendance_status_label']?.toString(),
      checkedInAt: _tryParseDateTime(json['checked_in_at']),
      checkedOutAt: _tryParseDateTime(json['checked_out_at']),
    );
  }
}

class AttendanceRecord {
  final String id;
  final String rtId;
  final String scheduleId;
  final String userId;
  final String officerName;
  final String shiftConfigId;
  final int shiftVersion;
  final String shiftDate;
  final String shiftName;
  final String startTime;
  final String endTime;
  final bool crossesMidnight;
  final String status;
  final String statusLabel;
  final String? arrivalStatus;
  final int lateMinutes;
  final String? checkInDeviceName;
  final String? checkOutDeviceName;
  final DateTime? checkedInAt;
  final DateTime? checkedOutAt;
  final bool correctionLocked;

  const AttendanceRecord({
    required this.id,
    required this.rtId,
    required this.scheduleId,
    required this.userId,
    required this.officerName,
    required this.shiftConfigId,
    required this.shiftVersion,
    required this.shiftDate,
    required this.shiftName,
    required this.startTime,
    required this.endTime,
    required this.crossesMidnight,
    required this.status,
    required this.statusLabel,
    this.arrivalStatus,
    required this.lateMinutes,
    this.checkInDeviceName,
    this.checkOutDeviceName,
    this.checkedInAt,
    this.checkedOutAt,
    required this.correctionLocked,
  });

  factory AttendanceRecord.fromJson(Map<String, dynamic> json) {
    return AttendanceRecord(
      id: json['id']?.toString() ?? '',
      rtId: json['rt_id']?.toString() ?? '',
      scheduleId: json['schedule_id']?.toString() ?? '',
      userId: json['user_id']?.toString() ?? '',
      officerName: json['officer_name']?.toString() ?? '',
      shiftConfigId: json['shift_config_id']?.toString() ?? '',
      shiftVersion: (json['shift_version'] as num?)?.toInt() ?? 1,
      shiftDate: json['shift_date']?.toString() ?? '',
      shiftName: json['shift_name']?.toString() ?? '',
      startTime: json['start_time']?.toString() ?? '',
      endTime: json['end_time']?.toString() ?? '',
      crossesMidnight: json['crosses_midnight'] as bool? ?? false,
      status: json['status']?.toString() ?? '',
      statusLabel: json['status_label']?.toString() ?? '',
      arrivalStatus: json['arrival_status']?.toString(),
      lateMinutes: (json['late_minutes'] as num?)?.toInt() ?? 0,
      checkInDeviceName: json['check_in_device_name']?.toString(),
      checkOutDeviceName: json['check_out_device_name']?.toString(),
      checkedInAt: _tryParseDateTime(json['checked_in_at']),
      checkedOutAt: _tryParseDateTime(json['checked_out_at']),
      correctionLocked: json['correction_locked'] as bool? ?? false,
    );
  }
}

class OfficerAttendanceOverview {
  final DateTime serverTime;
  final String timezone;
  final SecuritySchedule? currentSchedule;
  final AttendanceRecord? currentRecord;
  final List<SecuritySchedule> upcomingShifts;
  final List<AttendanceRecord> recentHistory;

  const OfficerAttendanceOverview({
    required this.serverTime,
    required this.timezone,
    this.currentSchedule,
    this.currentRecord,
    required this.upcomingShifts,
    required this.recentHistory,
  });

  factory OfficerAttendanceOverview.fromJson(Map<String, dynamic> json) {
    return OfficerAttendanceOverview(
      serverTime: _tryParseDateTime(json['server_time']) ?? DateTime.now(),
      timezone: json['timezone']?.toString() ?? 'Asia/Jakarta',
      currentSchedule: json['current_schedule'] is Map<String, dynamic>
          ? SecuritySchedule.fromJson(
              json['current_schedule'] as Map<String, dynamic>,
            )
          : null,
      currentRecord: json['current_record'] is Map<String, dynamic>
          ? AttendanceRecord.fromJson(
              json['current_record'] as Map<String, dynamic>,
            )
          : null,
      upcomingShifts: (json['upcoming_shifts'] as List? ?? [])
          .whereType<Map<String, dynamic>>()
          .map(SecuritySchedule.fromJson)
          .toList(),
      recentHistory: (json['recent_history'] as List? ?? [])
          .whereType<Map<String, dynamic>>()
          .map(AttendanceRecord.fromJson)
          .toList(),
    );
  }
}

class AttendanceScanResult {
  final String action;
  final AttendanceRecord record;
  final DateTime serverTime;
  final String deviceId;
  final String deviceName;
  final String message;

  const AttendanceScanResult({
    required this.action,
    required this.record,
    required this.serverTime,
    required this.deviceId,
    required this.deviceName,
    required this.message,
  });

  factory AttendanceScanResult.fromJson(Map<String, dynamic> json) {
    return AttendanceScanResult(
      action: json['action']?.toString() ?? '',
      record: AttendanceRecord.fromJson(
        (json['record'] as Map?)?.cast<String, dynamic>() ?? {},
      ),
      serverTime: _tryParseDateTime(json['server_time']) ?? DateTime.now(),
      deviceId: json['device_id']?.toString() ?? '',
      deviceName: json['device_name']?.toString() ?? '',
      message: json['message']?.toString() ?? 'Absensi berhasil dicatat',
    );
  }
}

class ShiftSwap {
  final String id;
  final String rtId;
  final String requesterUserId;
  final String requesterName;
  final String targetUserId;
  final String targetName;
  final String requesterScheduleId;
  final String requesterShiftDate;
  final String requesterShiftName;
  final String targetScheduleId;
  final String targetShiftDate;
  final String targetShiftName;
  final String reason;
  final String status;
  final String? targetNote;
  final String? ketuaNote;
  final DateTime? createdAt;

  const ShiftSwap({
    required this.id,
    required this.rtId,
    required this.requesterUserId,
    required this.requesterName,
    required this.targetUserId,
    required this.targetName,
    required this.requesterScheduleId,
    required this.requesterShiftDate,
    required this.requesterShiftName,
    required this.targetScheduleId,
    required this.targetShiftDate,
    required this.targetShiftName,
    required this.reason,
    required this.status,
    this.targetNote,
    this.ketuaNote,
    this.createdAt,
  });

  factory ShiftSwap.fromJson(Map<String, dynamic> json) {
    return ShiftSwap(
      id: json['id']?.toString() ?? '',
      rtId: json['rt_id']?.toString() ?? '',
      requesterUserId: json['requester_user_id']?.toString() ?? '',
      requesterName: json['requester_name']?.toString() ?? '',
      targetUserId: json['target_user_id']?.toString() ?? '',
      targetName: json['target_name']?.toString() ?? '',
      requesterScheduleId: json['requester_schedule_id']?.toString() ?? '',
      requesterShiftDate: json['requester_shift_date']?.toString() ?? '',
      requesterShiftName: json['requester_shift_name']?.toString() ?? '',
      targetScheduleId: json['target_schedule_id']?.toString() ?? '',
      targetShiftDate: json['target_shift_date']?.toString() ?? '',
      targetShiftName: json['target_shift_name']?.toString() ?? '',
      reason: json['reason']?.toString() ?? '',
      status: json['status']?.toString() ?? '',
      targetNote: json['target_note']?.toString(),
      ketuaNote: json['ketua_note']?.toString(),
      createdAt: _tryParseDateTime(json['created_at']),
    );
  }
}

class AttendanceCorrection {
  final String id;
  final String attendanceRecordId;
  final String scheduleId;
  final String officerName;
  final String shiftDate;
  final String shiftName;
  final String requestType;
  final String requestedBy;
  final String? decidedBy;
  final String? decidedByName;
  final DateTime? decidedAt;
  final String originalStatus;
  final String requestedStatus;
  final String reason;
  final String status;
  final String? decisionNote;
  final DateTime? createdAt;

  const AttendanceCorrection({
    required this.id,
    required this.attendanceRecordId,
    required this.scheduleId,
    required this.officerName,
    required this.shiftDate,
    required this.shiftName,
    required this.requestType,
    this.requestedBy = '',
    this.decidedBy,
    this.decidedByName,
    this.decidedAt,
    required this.originalStatus,
    required this.requestedStatus,
    required this.reason,
    required this.status,
    this.decisionNote,
    this.createdAt,
  });

  factory AttendanceCorrection.fromJson(Map<String, dynamic> json) {
    return AttendanceCorrection(
      id: json['id']?.toString() ?? '',
      attendanceRecordId: json['attendance_record_id']?.toString() ?? '',
      scheduleId: json['schedule_id']?.toString() ?? '',
      officerName: json['officer_name']?.toString() ?? '',
      shiftDate: json['shift_date']?.toString() ?? '',
      shiftName: json['shift_name']?.toString() ?? '',
      requestType: json['request_type']?.toString() ?? '',
      requestedBy: json['requested_by']?.toString() ?? '',
      decidedBy: json['decided_by']?.toString(),
      decidedByName: json['decided_by_name']?.toString(),
      decidedAt: _tryParseDateTime(json['decided_at']),
      originalStatus: json['original_status']?.toString() ?? '',
      requestedStatus: json['requested_status']?.toString() ?? '',
      reason: json['reason']?.toString() ?? '',
      status: json['status']?.toString() ?? '',
      decisionNote: json['decision_note']?.toString(),
      createdAt: _tryParseDateTime(json['created_at']),
    );
  }
}

/// Result of creating a Pos device: the device row plus a one-time pairing
/// token to be entered into the kiosk during activation.
class DeviceProvisionResult {
  final SecurityDevice device;
  final String pairingToken;
  final DateTime? expiresAt;

  const DeviceProvisionResult({
    required this.device,
    required this.pairingToken,
    this.expiresAt,
  });
}

class SecurityDevice {
  final String id;
  final String rtId;
  final String rtName;
  final String rtCode;
  final String deviceName;
  final String locationLabel;
  final DateTime? activatedAt;
  final DateTime? lastActiveAt;
  final DateTime? pairingExpiresAt;
  final bool isActive;
  final bool isPaired;

  const SecurityDevice({
    required this.id,
    required this.rtId,
    required this.rtName,
    required this.rtCode,
    required this.deviceName,
    required this.locationLabel,
    this.activatedAt,
    this.lastActiveAt,
    this.pairingExpiresAt,
    required this.isActive,
    required this.isPaired,
  });

  factory SecurityDevice.fromJson(Map<String, dynamic> json) {
    return SecurityDevice(
      id: json['id']?.toString() ?? '',
      rtId: json['rt_id']?.toString() ?? '',
      rtName: json['rt_name']?.toString() ?? '',
      rtCode: json['rt_code']?.toString() ?? '',
      deviceName: json['device_name']?.toString() ?? '',
      locationLabel: json['location_label']?.toString() ?? '',
      activatedAt: _tryParseDateTime(json['activated_at']),
      lastActiveAt: _tryParseDateTime(json['last_active_at']),
      pairingExpiresAt: _tryParseDateTime(json['activation_expires_at']),
      isActive: json['is_active'] as bool? ?? true,
      isPaired: json['is_paired'] as bool? ?? false,
    );
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    'rt_id': rtId,
    'rt_name': rtName,
    'rt_code': rtCode,
    'device_name': deviceName,
    'location_label': locationLabel,
    'is_active': isActive,
    'is_paired': isPaired,
  };
}

class DynamicQrPayload {
  final String tokenId;
  final String qrToken;
  final String deviceId;
  final String deviceName;
  final String rtId;
  final DateTime issuedAt;
  final DateTime expiresAt;
  final int expiresInSeconds;

  const DynamicQrPayload({
    required this.tokenId,
    required this.qrToken,
    required this.deviceId,
    required this.deviceName,
    required this.rtId,
    required this.issuedAt,
    required this.expiresAt,
    required this.expiresInSeconds,
  });

  factory DynamicQrPayload.fromJson(Map<String, dynamic> json) {
    return DynamicQrPayload(
      tokenId: json['token_id']?.toString() ?? '',
      qrToken: json['qr_token']?.toString() ?? '',
      deviceId: json['device_id']?.toString() ?? '',
      deviceName: json['device_name']?.toString() ?? '',
      rtId: json['rt_id']?.toString() ?? '',
      issuedAt: _tryParseDateTime(json['issued_at']) ?? DateTime.now(),
      expiresAt:
          _tryParseDateTime(json['expires_at']) ??
          DateTime.now().add(const Duration(seconds: 45)),
      expiresInSeconds: (json['expires_in_seconds'] as num?)?.toInt() ?? 45,
    );
  }
}

class ShiftConfig {
  final String id;
  final String rtId;
  final String name;
  final String startTime;
  final String endTime;
  final bool crossesMidnight;
  final int latenessToleranceMinutes;
  final bool isActive;
  const ShiftConfig({
    required this.id,
    required this.rtId,
    required this.name,
    required this.startTime,
    required this.endTime,
    required this.crossesMidnight,
    required this.latenessToleranceMinutes,
    required this.isActive,
  });
  factory ShiftConfig.fromJson(Map<String, dynamic> json) => ShiftConfig(
    id: json['id']?.toString() ?? '',
    rtId: json['rt_id']?.toString() ?? '',
    name: json['name']?.toString() ?? '',
    startTime: json['start_time']?.toString() ?? '',
    endTime: json['end_time']?.toString() ?? '',
    crossesMidnight: json['crosses_midnight'] as bool? ?? false,
    latenessToleranceMinutes:
        (json['lateness_tolerance_minutes'] as num?)?.toInt() ?? 0,
    isActive: json['is_active'] as bool? ?? true,
  );
}

class GuestBookPage {
  final List<GuestBookEntry> data;
  final int total;
  final int totalPages;
  const GuestBookPage(this.data, this.total, this.totalPages);
}

class GuestBookEntry {
  final String id;
  final String rtId;
  final String deviceId;
  final String deviceName;
  final String guestName;
  final String? phone;
  final String? phoneMasked;
  final String purpose;
  final String? hostHouseholdId;
  final String destinationLabel;
  final String? vehiclePlate;
  final String? notes;
  final DateTime checkedInAt;
  final DateTime? checkedOutAt;

  const GuestBookEntry({
    required this.id,
    required this.rtId,
    required this.deviceId,
    required this.deviceName,
    required this.guestName,
    this.phone,
    this.phoneMasked,
    required this.purpose,
    this.hostHouseholdId,
    required this.destinationLabel,
    this.vehiclePlate,
    this.notes,
    required this.checkedInAt,
    this.checkedOutAt,
  });

  factory GuestBookEntry.fromJson(Map<String, dynamic> json) {
    return GuestBookEntry(
      id: json['id']?.toString() ?? '',
      rtId: json['rt_id']?.toString() ?? '',
      deviceId: json['device_id']?.toString() ?? '',
      deviceName: json['device_name']?.toString() ?? '',
      guestName: json['guest_name']?.toString() ?? '',
      phone: json['phone']?.toString(),
      phoneMasked: json['phone_masked']?.toString(),
      purpose: json['purpose']?.toString() ?? '',
      hostHouseholdId: json['host_household_id']?.toString(),
      destinationLabel: json['destination_label']?.toString() ?? '',
      vehiclePlate: json['vehicle_plate']?.toString(),
      notes: json['notes']?.toString(),
      checkedInAt: _tryParseDateTime(json['checked_in_at']) ?? DateTime.now(),
      checkedOutAt: _tryParseDateTime(json['checked_out_at']),
    );
  }
}

class PosResidentSummary {
  final String id;
  final String fullName;
  final String relationship;
  final bool isHeadOfFamily;
  final String residencyStatus;
  final String? avatarPath;

  const PosResidentSummary({
    required this.id,
    required this.fullName,
    required this.relationship,
    required this.isHeadOfFamily,
    required this.residencyStatus,
    this.avatarPath,
  });

  factory PosResidentSummary.fromJson(Map<String, dynamic> json) {
    return PosResidentSummary(
      id: json['id']?.toString() ?? '',
      fullName: json['full_name']?.toString() ?? '',
      relationship: json['relationship']?.toString() ?? '',
      isHeadOfFamily: json['is_head_of_family'] as bool? ?? false,
      residencyStatus: json['residency_status']?.toString() ?? '',
      avatarPath: json['avatar_path']?.toString(),
    );
  }
}

class PosHouseholdSummary {
  final String householdOccupancyId;
  final String houseNumber;
  final String? block;
  final String headOfFamilyName;
  final String occupancyType;
  final int residentCount;
  final String? headAvatarPath;
  final List<PosResidentSummary> residents;

  const PosHouseholdSummary({
    required this.householdOccupancyId,
    required this.houseNumber,
    this.block,
    required this.headOfFamilyName,
    required this.occupancyType,
    required this.residentCount,
    this.headAvatarPath,
    this.residents = const [],
  });

  factory PosHouseholdSummary.fromJson(Map<String, dynamic> json) {
    return PosHouseholdSummary(
      householdOccupancyId: json['household_occupancy_id']?.toString() ?? '',
      houseNumber: json['house_number']?.toString() ?? '',
      block: json['block']?.toString(),
      headOfFamilyName: json['head_of_family_name']?.toString() ?? '',
      occupancyType: json['occupancy_type']?.toString() ?? '',
      residentCount: (json['resident_count'] as num?)?.toInt() ?? 0,
      headAvatarPath: json['head_avatar_path']?.toString(),
      residents: (json['residents'] as List? ?? [])
          .whereType<Map<String, dynamic>>()
          .map(PosResidentSummary.fromJson)
          .toList(),
    );
  }
}

class PosHouseholdPage {
  final List<PosHouseholdSummary> data;
  final int total;
  final int totalPages;
  const PosHouseholdPage(this.data, this.total, this.totalPages);
}

class PosWasteMonitoringResponse {
  final List<MonitoringHousehold> items;
  final String evalDate;

  const PosWasteMonitoringResponse({
    required this.items,
    required this.evalDate,
  });

  factory PosWasteMonitoringResponse.fromJson(Map<String, dynamic> json) {
    return PosWasteMonitoringResponse(
      items: (json['data'] as List? ?? [])
          .whereType<Map<String, dynamic>>()
          .map(MonitoringHousehold.fromJson)
          .toList(),
      evalDate: json['eval_date']?.toString() ?? '',
    );
  }
}

DateTime? _tryParseDateTime(dynamic raw) {
  if (raw == null) return null;
  final str = raw.toString().trim();
  if (str.isEmpty) return null;
  return DateTime.tryParse(str)?.toLocal();
}
