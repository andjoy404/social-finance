import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:uuid/uuid.dart';

import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/utils/url_utils.dart';

import 'pos_device_storage.dart';
import 'security_models.dart';

class SecurityApiException implements Exception {
  final String message;
  final String? code;
  final int? statusCode;

  const SecurityApiException(this.message, {this.code, this.statusCode});

  bool get isDeviceRevokedOrUnauthorized =>
      statusCode == 401 ||
      statusCode == 403 ||
      code == 'device_revoked' ||
      code == 'unauthorized';

  @override
  String toString() => message;
}

class SecurityApiClient {
  final ApiClient apiClient;
  final Dio _deviceDio;
  final Uuid _uuid = const Uuid();

  SecurityApiClient(this.apiClient, {Dio? deviceDio})
    : _deviceDio =
          deviceDio ??
          Dio(
            BaseOptions(
              connectTimeout: const Duration(seconds: 12),
              receiveTimeout: const Duration(seconds: 15),
              headers: {'Content-Type': 'application/json'},
            ),
          );

  SecurityApiException _extractDioError(DioException e, String fallback) {
    final status = e.response?.statusCode;
    final data = e.response?.data;
    if (data is Map<String, dynamic>) {
      final err = data['error'];
      if (err is Map<String, dynamic>) {
        final msg = err['message']?.toString().trim();
        final code = err['code']?.toString().trim();
        if (msg != null && msg.isNotEmpty) {
          return SecurityApiException(msg, code: code, statusCode: status);
        }
      }
      final directMsg = data['message']?.toString().trim();
      if (directMsg != null && directMsg.isNotEmpty) {
        return SecurityApiException(directMsg, statusCode: status);
      }
    }
    if (e.type == DioExceptionType.connectionTimeout ||
        e.type == DioExceptionType.connectionError ||
        e.type == DioExceptionType.receiveTimeout) {
      return const SecurityApiException(
        'Koneksi ke server bermasalah. Periksa jaringan Anda.',
        code: 'network_error',
      );
    }
    return SecurityApiException(fallback, statusCode: status);
  }

  // ── Officer Endpoints (JWT Bearer) ─────────────────────────────────────────

  Future<OfficerAttendanceOverview> fetchMyAttendanceOverview() async {
    try {
      final res = await apiClient.get('/api/v1/security/attendance/me');
      return OfficerAttendanceOverview.fromJson(res.data ?? {});
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal memuat ringkasan absensi');
    }
  }

  Future<AttendanceScanResult> scanQrAttendance({
    required String qrToken,
    String action = 'auto',
    String? shiftConfigId,
    String? idempotencyKey,
  }) async {
    final key = idempotencyKey ?? _uuid.v4();
    try {
      final body = <String, dynamic>{'qr_token': qrToken.trim(), 'action': action};
      if (shiftConfigId != null && shiftConfigId.isNotEmpty) {
        body['shift_config_id'] = shiftConfigId;
      }
      final res = await apiClient.dio.post<Map<String, dynamic>>(
        '${apiClient.baseUrl}/api/v1/security/attendance/scan',
        data: body,
        options: Options(headers: {'Idempotency-Key': key}),
      );
      return AttendanceScanResult.fromJson(res.data ?? {});
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal memproses scan QR absensi');
    }
  }

  Future<List<SecurityOfficer>> listManagerOfficers() async {
    try {
      final res = await apiClient.get('/api/v1/security/officers');
      final list = res.data?['data'] as List? ?? [];
      return list
          .whereType<Map<String, dynamic>>()
          .map(SecurityOfficer.fromJson)
          .toList();
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal memuat petugas');
    }
  }

  Future<List<ShiftConfig>> listManagerShifts() async {
    try {
      final res = await apiClient.get('/api/v1/security/shifts');
      final list = res.data?['data'] as List? ?? [];
      return list
          .whereType<Map<String, dynamic>>()
          .map(ShiftConfig.fromJson)
          .toList();
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal memuat shift');
    }
  }

  Future<List<SecurityOfficer>> listOfficers() async {
    try {
      final res = await apiClient.get('/api/v1/security/officers');
      final list = res.data?['data'] as List? ?? [];
      return list
          .whereType<Map<String, dynamic>>()
          .map(SecurityOfficer.fromJson)
          .toList();
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal memuat daftar petugas');
    }
  }

  Future<List<SecuritySchedule>> listManagerSchedules({
    String? dateFrom,
    String? dateTo,
    String? userId,
    int page = 1,
  }) async {
    try {
      final res = await apiClient.get(
        '/api/v1/security/schedules',
        queryParameters: {
          if (dateFrom != null && dateFrom.isNotEmpty) 'date_from': dateFrom,
          if (dateTo != null && dateTo.isNotEmpty) 'date_to': dateTo,
          if (userId != null && userId.isNotEmpty) 'user_id': userId,
          'page': page,
          'page_size': 100,
        },
      );
      return (res.data?['data'] as List? ?? [])
          .whereType<Map<String, dynamic>>()
          .map(SecuritySchedule.fromJson)
          .toList();
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal memuat jadwal');
    }
  }

  Future<List<AttendanceRecord>> listManagerAttendance({
    String? dateFrom,
    String? dateTo,
    String? userId,
    String? status,
    int page = 1,
  }) async {
    try {
      final res = await apiClient.get(
        '/api/v1/security/attendance',
        queryParameters: {
          if (dateFrom != null && dateFrom.isNotEmpty) 'date_from': dateFrom,
          if (dateTo != null && dateTo.isNotEmpty) 'date_to': dateTo,
          if (userId != null && userId.isNotEmpty) 'user_id': userId,
          if (status != null && status.isNotEmpty) 'status': status,
          'page': page,
          'page_size': 100,
        },
      );
      return (res.data?['data'] as List? ?? [])
          .whereType<Map<String, dynamic>>()
          .map(AttendanceRecord.fromJson)
          .toList();
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal memuat absensi');
    }
  }

  Future<List<AttendanceCorrection>> listManagerCorrections() async {
    try {
      final res = await apiClient.get('/api/v1/security/corrections');
      return (res.data?['data'] as List? ?? [])
          .whereType<Map<String, dynamic>>()
          .map(AttendanceCorrection.fromJson)
          .toList();
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal memuat koreksi absensi');
    }
  }

  Future<List<SecurityDevice>> listManagerDevices() async {
    try {
      final res = await apiClient.get('/api/v1/security/devices');
      return (res.data?['data'] as List? ?? [])
          .whereType<Map<String, dynamic>>()
          .map(SecurityDevice.fromJson)
          .toList();
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal memuat perangkat');
    }
  }

  /// POST /api/v1/security/devices
  /// Creates a kiosk device and returns a one-time pairing token for
  /// activation on the Pos device itself.
  Future<DeviceProvisionResult> createManagerDevice({
    required String deviceName,
    required String locationLabel,
  }) async {
    try {
      final res = await apiClient.post(
        '/api/v1/security/devices',
        data: {
          'device_name': deviceName,
          'location_label': locationLabel,
        },
      );
      return _parseProvisionResult(res.data);
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal menambahkan perangkat pos');
    }
  }

  /// POST /api/v1/security/devices/{id}/rotate-pairing
  /// Generates a new one-time pairing token for an existing device.
  /// The previous pairing is invalidated by the backend.
  Future<DeviceProvisionResult> rotateDevicePairing(String deviceId) async {
    try {
      final res = await apiClient.post(
        '/api/v1/security/devices/$deviceId/rotate-pairing',
      );
      return _parseProvisionResult(res.data);
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal membuat token pairing baru');
    }
  }

  DeviceProvisionResult _parseProvisionResult(dynamic body) {
    final data = (body as Map?)?.cast<String, dynamic>();
    if (data == null) {
      throw const SecurityApiException('Respon perangkat tidak valid');
    }
    final deviceJson = (data['device'] as Map?)?.cast<String, dynamic>();
    if (deviceJson == null) {
      throw const SecurityApiException('Respon perangkat tidak valid');
    }
    final pairingToken = (data['pairing_token'] ?? '').toString();
    final expiresRaw = (data['expires_at'] ?? '').toString().trim();
    DateTime? expiresAt;
    if (expiresRaw.isNotEmpty) {
      expiresAt = DateTime.tryParse(expiresRaw)?.toLocal();
    }
    return DeviceProvisionResult(
      device: SecurityDevice.fromJson(deviceJson),
      pairingToken: pairingToken,
      expiresAt: expiresAt,
    );
  }

  Future<GuestBookPage> listManagerGuestBooks({
    String? search,
    int page = 1,
    String? status,
  }) async {
    try {
      final res = await apiClient.get(
        '/api/v1/security/guest-books',
        queryParameters: {
          if (search != null && search.trim().isNotEmpty)
            'search': search.trim(),
          if (status != null && status.isNotEmpty) 'status': status,
          'page': page,
          'page_size': 25,
        },
      );
      final rows = (res.data?['data'] as List? ?? [])
          .whereType<Map<String, dynamic>>()
          .map(GuestBookEntry.fromJson)
          .toList();
      final pagination =
          (res.data?['pagination'] as Map?)?.cast<String, dynamic>() ?? {};
      return GuestBookPage(
        rows,
        (pagination['total'] as num?)?.toInt() ?? rows.length,
        (pagination['total_pages'] as num?)?.toInt() ?? 1,
      );
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal memuat buku tamu');
    }
  }

  Future<void> decideCorrection(
    String id, {
    required bool approve,
    String? note,
  }) async {
    try {
      await apiClient.post(
        '/api/v1/security/corrections/$id/decide',
        data: {
          'approve': approve,
          if (note != null && note.isNotEmpty) 'note': note,
        },
      );
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal memproses koreksi');
    }
  }

  Future<void> revokeDevice(String id) async {
    try {
      await apiClient.post('/api/v1/security/devices/$id/revoke');
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal menonaktifkan perangkat');
    }
  }

  Future<SecuritySchedule> createManagerSchedule(
    Map<String, dynamic> payload,
  ) async {
    try {
      final res = await apiClient.post(
        '/api/v1/security/schedules',
        data: payload,
      );
      return SecuritySchedule.fromJson(
        (res.data as Map).cast<String, dynamic>(),
      );
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal membuat jadwal');
    }
  }

  Future<void> updateManagerSchedule(
    String id,
    Map<String, dynamic> payload,
  ) async {
    try {
      await apiClient.put('/api/v1/security/schedules/$id', data: payload);
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal mengubah jadwal');
    }
  }

  Future<ShiftConfig> createManagerShift(Map<String, dynamic> payload) async {
    try {
      final res = await apiClient.post(
        '/api/v1/security/shifts',
        data: payload,
      );
      return ShiftConfig.fromJson((res.data as Map).cast<String, dynamic>());
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal membuat shift');
    }
  }

  Future<void> updateManagerShift(
    String id,
    Map<String, dynamic> payload,
  ) async {
    try {
      await apiClient.put('/api/v1/security/shifts/$id', data: payload);
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal mengubah shift');
    }
  }

  Future<List<SecuritySchedule>> listSchedules({
    String? dateFrom,
    String? dateTo,
    String? userId,
  }) async {
    try {
      final res = await apiClient.get(
        '/api/v1/security/schedules',
        queryParameters: {
          if (dateFrom != null && dateFrom.isNotEmpty) 'date_from': dateFrom,
          if (dateTo != null && dateTo.isNotEmpty) 'date_to': dateTo,
          if (userId != null && userId.isNotEmpty) 'user_id': userId,
          'page': 1,
          'page_size': 25,
        },
      );
      final list = res.data?['data'] as List? ?? [];
      return list
          .whereType<Map<String, dynamic>>()
          .map(SecuritySchedule.fromJson)
          .toList();
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal memuat jadwal petugas');
    }
  }

  Future<List<ShiftSwap>> listSwaps() async {
    try {
      final res = await apiClient.get('/api/v1/security/swaps');
      final list = res.data?['data'] as List? ?? [];
      return list
          .whereType<Map<String, dynamic>>()
          .map(ShiftSwap.fromJson)
          .toList();
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal memuat daftar tukar shift');
    }
  }

  Future<ShiftSwap> createSwap({
    required String requesterScheduleId,
    required String targetScheduleId,
    required String reason,
  }) async {
    try {
      final res = await apiClient.post(
        '/api/v1/security/swaps',
        data: {
          'requester_schedule_id': requesterScheduleId,
          'target_schedule_id': targetScheduleId,
          'reason': reason.trim(),
        },
      );
      return ShiftSwap.fromJson(res.data ?? {});
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal mengajukan tukar shift');
    }
  }

  Future<ShiftSwap> respondToSwapTarget({
    required String swapId,
    required bool accept,
    String? note,
  }) async {
    try {
      final res = await apiClient.post(
        '/api/v1/security/swaps/$swapId/target-respond',
        data: {
          'accept': accept,
          if (note != null && note.isNotEmpty) 'note': note,
        },
      );
      return ShiftSwap.fromJson(res.data ?? {});
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal merespons pengajuan tukar shift');
    }
  }

  Future<List<AttendanceCorrection>> listCorrections() async {
    try {
      final res = await apiClient.get('/api/v1/security/corrections');
      final list = res.data?['data'] as List? ?? [];
      return list
          .whereType<Map<String, dynamic>>()
          .map(AttendanceCorrection.fromJson)
          .toList();
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal memuat pengajuan koreksi');
    }
  }

  Future<AttendanceCorrection> createCorrection({
    String? attendanceRecordId,
    String? scheduleId,
    String? shiftDate,
    required String requestType,
    required String requestedStatus,
    required String reason,
  }) async {
    try {
      final body = <String, dynamic>{
        'request_type': requestType,
        'requested_status': requestedStatus,
        'reason': reason.trim(),
      };
      if (attendanceRecordId != null && attendanceRecordId.isNotEmpty) {
        body['attendance_record_id'] = attendanceRecordId;
      }
      if (scheduleId != null && scheduleId.isNotEmpty) {
        body['schedule_id'] = scheduleId;
      }
      if (shiftDate != null && shiftDate.isNotEmpty) {
        body['shift_date'] = shiftDate;
      }
      final res = await apiClient.post(
        '/api/v1/security/corrections',
        data: body,
      );
      return AttendanceCorrection.fromJson(res.data ?? {});
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal mengajukan koreksi absensi');
    }
  }

  // ── Patrol & Paket Endpoints ───────────────────────────────────────────────

  Future<List<PatrolTemplate>> listPatrolTemplates() async {
    try {
      final res = await apiClient.dio.get('/api/v1/security/patrol/templates');
      final list = res.data?['data'] as List? ?? [];
      return list
          .whereType<Map<String, dynamic>>()
          .map(PatrolTemplate.fromJson)
          .toList();
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal memuat template patroli');
    }
  }

  Future<PatrolTemplate> createPatrolTemplate({
    required String name,
    required String locationLabel,
  }) async {
    try {
      final res = await apiClient.dio.post(
        '/api/v1/security/patrol/templates',
        data: {
          'name': name.trim(),
          'location_label': locationLabel.trim(),
        },
      );
      final data = (res.data?['data'] as Map?)?.cast<String, dynamic>() ??
          (res.data as Map?)?.cast<String, dynamic>() ??
          {};
      return PatrolTemplate.fromJson(data);
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal menambah template patroli');
    }
  }

  Future<PatrolTemplate> updatePatrolTemplate(
    String templateId, {
    required String name,
    required String locationLabel,
    bool? isActive,
  }) async {
    try {
      final res = await apiClient.dio.put(
        '/api/v1/security/patrol/templates/$templateId',
        data: {
          'name': name.trim(),
          'location_label': locationLabel.trim(),
          if (isActive != null) 'is_active': isActive,
        },
      );
      final data = (res.data?['data'] as Map?)?.cast<String, dynamic>() ??
          (res.data as Map?)?.cast<String, dynamic>() ??
          {};
      return PatrolTemplate.fromJson(data);
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal memperbarui template patroli');
    }
  }

  Future<void> deletePatrolTemplate(String templateId) async {
    try {
      await apiClient.dio.delete('/api/v1/security/patrol/templates/$templateId');
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal menghapus template patroli');
    }
  }

  Future<void> markReadReceipt({
    required String domain,
    List<String>? itemIds,
  }) async {
    try {
      await apiClient.dio.post(
        '/api/v1/security/read-receipts',
        data: {
          'domain': domain,
          if (itemIds != null && itemIds.isNotEmpty) 'item_ids': itemIds,
        },
      );
    } catch (_) {}
  }

  Future<SecurityAlertsData> fetchSecurityAlerts() async {
    try {
      final res = await apiClient.dio.get('/api/v1/security/alerts');
      final data = res.data;
      if (data is Map && data['data'] is Map) {
        return SecurityAlertsData.fromJson(
          (data['data'] as Map).cast<String, dynamic>(),
        );
      }
      return const SecurityAlertsData();
    } catch (_) {
      return const SecurityAlertsData();
    }
  }

  Future<PatrolBundle> createPatrolBundle({
    String? notes,
    required List<Map<String, dynamic>> photos,
  }) async {
    try {
      final res = await apiClient.dio.post(
        '/api/v1/security/patrol/bundles',
        data: {
          if (notes != null && notes.isNotEmpty) 'notes': notes,
          'photos': photos,
        },
      );
      final data = (res.data?['data'] as Map?)?.cast<String, dynamic>() ??
          (res.data as Map?)?.cast<String, dynamic>() ??
          {};
      return PatrolBundle.fromJson(data);
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal membuat laporan patroli');
    }
  }

  Future<List<PatrolBundle>> listPatrolBundles() async {
    try {
      final res = await apiClient.dio.get('/api/v1/security/patrol/bundles');
      final list = res.data?['data'] as List? ?? [];
      return list
          .whereType<Map<String, dynamic>>()
          .map(PatrolBundle.fromJson)
          .toList();
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal memuat riwayat patroli');
    }
  }

  Future<String> uploadPatrolPhoto(Uint8List bytes, String filename) async {
    try {
      final formData = FormData.fromMap({
        'photo': MultipartFile.fromBytes(bytes, filename: filename),
      });
      final res = await apiClient.dio.post(
        '/api/v1/security/patrol/photos',
        data: formData,
      );
      final data = res.data;
      if (data is Map && data['image_key'] != null) {
        return data['image_key'].toString();
      }
      return '';
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal mengunggah foto patroli');
    }
  }

  Future<List<MaskedWargaSearchResult>> searchWargaMasked(String query) async {
    try {
      final res = await apiClient.dio.get(
        '/api/v1/security/warga/search',
        queryParameters: {'q': query},
      );
      final list = res.data?['data'] as List? ?? [];
      return list
          .whereType<Map<String, dynamic>>()
          .map(MaskedWargaSearchResult.fromJson)
          .toList();
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal mencari warga');
    }
  }

  Future<PaketReport> createPaketReport({
    required String receiverWargaId,
    required List<Map<String, dynamic>> items,
  }) async {
    try {
      final res = await apiClient.dio.post(
        '/api/v1/security/paket/reports',
        data: {
          'receiver_warga_id': receiverWargaId,
          'items': items,
        },
      );
      final data = (res.data?['data'] as Map?)?.cast<String, dynamic>() ??
          (res.data as Map?)?.cast<String, dynamic>() ??
          {};
      return PaketReport.fromJson(data);
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal membuat laporan paket');
    }
  }

  Future<List<PaketReport>> listPaketReports({String? status}) async {
    try {
      final res = await apiClient.dio.get(
        '/api/v1/security/paket/reports',
        queryParameters: {
          if (status != null && status.isNotEmpty) 'status': status,
        },
      );
      final list = res.data?['data'] as List? ?? [];
      return list
          .whereType<Map<String, dynamic>>()
          .map(PaketReport.fromJson)
          .toList();
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal memuat daftar paket');
    }
  }

  Future<PaketReport> getPaketReport(String reportId) async {
    try {
      final res = await apiClient.dio.get('/api/v1/security/paket/reports/$reportId');
      final data = (res.data?['data'] as Map?)?.cast<String, dynamic>() ??
          (res.data as Map?)?.cast<String, dynamic>() ??
          {};
      return PaketReport.fromJson(data);
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal memuat detail paket');
    }
  }

  Future<PaketItem> updatePaketItemStatus(
    String reportId,
    String itemId, {
    required String status,
    String? proofImageKey,
  }) async {
    try {
      final res = await apiClient.dio.patch(
        '/api/v1/security/paket/reports/$reportId/items/$itemId/status',
        data: {
          'status': status,
          if (proofImageKey != null && proofImageKey.isNotEmpty)
            'proof_image_key': proofImageKey,
        },
      );
      final data = (res.data?['data'] as Map?)?.cast<String, dynamic>() ??
          (res.data as Map?)?.cast<String, dynamic>() ??
          {};
      return PaketItem.fromJson(data);
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal memperbarui status paket');
    }
  }

  Future<String> uploadPaketPhoto(Uint8List bytes, String filename) async {
    try {
      final formData = FormData.fromMap({
        'photo': MultipartFile.fromBytes(bytes, filename: filename),
      });
      final res = await apiClient.dio.post(
        '/api/v1/security/paket/photos',
        data: formData,
      );
      final data = res.data;
      if (data is Map && data['image_key'] != null) {
        return data['image_key'].toString();
      }
      return '';
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal mengunggah foto paket');
    }
  }

  // ── Pos Device Endpoints (X-Device-Token) ──────────────────────────────────

  Future<PosDeviceSession> activatePosDevice({
    required String serverUrl,
    required String pairingToken,
  }) async {
    final normalizedUrl = UrlUtils.normalizeUrl(serverUrl);
    try {
      final res = await _deviceDio.post<Map<String, dynamic>>(
        '$normalizedUrl/api/v1/security/devices/activate',
        data: {'pairing_token': pairingToken.trim()},
      );
      final data = res.data ?? {};
      final token = data['device_token']?.toString() ?? '';
      final deviceMap = (data['device'] as Map?)?.cast<String, dynamic>() ?? {};
      if (token.isEmpty) {
        throw const SecurityApiException('Token perangkat tidak valid');
      }
      return PosDeviceSession(
        serverUrl: normalizedUrl,
        deviceToken: token,
        device: SecurityDevice.fromJson(deviceMap),
      );
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal mengaktifkan perangkat pos');
    }
  }

  Options _deviceOptions(PosDeviceSession session) =>
      Options(headers: {'X-Device-Token': session.deviceToken});

  /// Fetch a resident avatar as raw bytes through the device-scoped image
  /// endpoint. The backend restricts this to `community/{deviceRT}/profile/*`
  /// keys referenced by active residents of that RT.
  Future<Uint8List?> fetchDeviceImage(PosDeviceSession session, String key) async {
    try {
      final res = await _deviceDio.get<List<int>>(
        '${session.serverUrl}/api/v1/security-device/images/$key',
        options: Options(
          responseType: ResponseType.bytes,
          headers: {'X-Device-Token': session.deviceToken},
        ),
      );
      final data = res.data;
      if (data == null || data.isEmpty) return null;
      return Uint8List.fromList(data);
    } on DioException {
      return null;
    }
  }

  /// PATCH /api/v1/security-device/monitoring/sampah/{householdOccupancyID}/note
  ///
  /// Device-scoped note write: the backend requires the household to belong to
  /// the device's bound RT and updates the same rw_monitoring_notes table used
  /// by officers/users.
  Future<void> updateSampahNote(
    PosDeviceSession session, {
    required String householdOccupancyId,
    required String note,
  }) async {
    try {
      await _deviceDio.patch<Map<String, dynamic>>(
        '${session.serverUrl}/api/v1/security-device/monitoring/sampah/$householdOccupancyId/note',
        data: {'note': note},
        options: _deviceOptions(session),
      );
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal menyimpan catatan');
    }
  }

  Future<SecurityDevice> fetchPosDeviceMe(PosDeviceSession session) async {
    try {
      final res = await _deviceDio.get<Map<String, dynamic>>(
        '${session.serverUrl}/api/v1/security-device/me',
        options: _deviceOptions(session),
      );
      return SecurityDevice.fromJson(res.data ?? {});
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal memeriksa status perangkat pos');
    }
  }

  Future<DynamicQrPayload> issuePosQrToken(PosDeviceSession session) async {
    try {
      final res = await _deviceDio.post<Map<String, dynamic>>(
        '${session.serverUrl}/api/v1/security-device/qr',
        options: _deviceOptions(session),
      );
      return DynamicQrPayload.fromJson(res.data ?? {});
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal menghasilkan QR absensi');
    }
  }

  Future<List<AttendanceRecord>> fetchPosTodayAttendance(
    PosDeviceSession session,
  ) async {
    try {
      final res = await _deviceDio.get<Map<String, dynamic>>(
        '${session.serverUrl}/api/v1/security-device/attendance/today',
        options: _deviceOptions(session),
      );
      final list = res.data?['data'] as List? ?? [];
      return list
          .whereType<Map<String, dynamic>>()
          .map(AttendanceRecord.fromJson)
          .toList();
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal memuat absensi pos hari ini');
    }
  }

  Future<GuestBookPage> fetchPosGuestBooks(
    PosDeviceSession session, {
    String? status,
    String? search,
    int page = 1,
  }) async {
    try {
      final res = await _deviceDio.get<Map<String, dynamic>>(
        '${session.serverUrl}/api/v1/security-device/guest-books',
        queryParameters: {
          if (status != null && status.isNotEmpty) 'status': status,
          if (search != null && search.isNotEmpty) 'search': search,
          'page': 1,
          'page_size': 50,
        },
        options: _deviceOptions(session),
      );
      final list = (res.data?['data'] as List? ?? [])
          .whereType<Map<String, dynamic>>()
          .map(GuestBookEntry.fromJson)
          .toList();
      final pagination =
          (res.data?['pagination'] as Map?)?.cast<String, dynamic>() ?? {};
      return GuestBookPage(
        list,
        (pagination['total'] as num?)?.toInt() ?? list.length,
        (pagination['total_pages'] as num?)?.toInt() ?? 1,
      );
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal memuat buku tamu');
    }
  }

  Future<GuestBookEntry> createPosGuestBook(
    PosDeviceSession session, {
    required String guestName,
    String? phone,
    required String purpose,
    required String destinationLabel,
    String? hostHouseholdId,
    String? vehiclePlate,
    String? notes,
    String? photoKey,
  }) async {
    try {
      final res = await _deviceDio.post<Map<String, dynamic>>(
        '${session.serverUrl}/api/v1/security-device/guest-books',
        data: {
          'guest_name': guestName.trim(),
          if (phone != null && phone.trim().isNotEmpty) 'phone': phone.trim(),
          'purpose': purpose.trim(),
          'destination_label': destinationLabel.trim(),
          if (hostHouseholdId != null && hostHouseholdId.isNotEmpty)
            'host_household_id': hostHouseholdId,
          if (vehiclePlate != null && vehiclePlate.trim().isNotEmpty)
            'vehicle_plate': vehiclePlate.trim(),
          if (notes != null && notes.trim().isNotEmpty) 'notes': notes.trim(),
          if (photoKey != null && photoKey.trim().isNotEmpty) 'photo_key': photoKey.trim(),
        },
        options: _deviceOptions(session),
      );
      return GuestBookEntry.fromJson(res.data ?? {});
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal mencatat tamu masuk');
    }
  }

  /// Upload a guest book photo and return the image key.
  Future<String> uploadGuestBookPhoto(Uint8List bytes, String filename) async {
    try {
      final formData = FormData.fromMap({
        'photo': MultipartFile.fromBytes(bytes, filename: filename),
      });
      final res = await apiClient.dio.post(
        '/api/v1/security/guest-books/photos',
        data: formData,
      );
      final data = res.data;
      if (data is Map && data['image_key'] != null) {
        return data['image_key'].toString();
      }
      return '';
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal mengunggah foto tamu');
    }
  }

  /// Fetch a guest book photo as raw bytes through the authenticated endpoint.
  Future<Uint8List?> fetchGuestBookPhoto(String photoKey) async {
    try {
      final res = await apiClient.dio.get<List<int>>(
        '/api/v1/security/guest-books/photos/$photoKey',
        options: Options(responseType: ResponseType.bytes),
      );
      final data = res.data;
      if (data == null || data.isEmpty) return null;
      return Uint8List.fromList(data);
    } on DioException {
      return null;
    }
  }

  Future<GuestBookEntry> checkoutPosGuestBook(
    PosDeviceSession session,
    String entryId,
  ) async {
    try {
      final res = await _deviceDio.post<Map<String, dynamic>>(
        '${session.serverUrl}/api/v1/security-device/guest-books/$entryId/checkout',
        options: _deviceOptions(session),
      );
      return GuestBookEntry.fromJson(res.data ?? {});
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal mencatat tamu keluar');
    }
  }

  Future<List<PosHouseholdSummary>> fetchPosHouseholds(
    PosDeviceSession session, {
    String? search,
  }) async {
    try {
      final res = await _deviceDio.get<Map<String, dynamic>>(
        '${session.serverUrl}/api/v1/security-device/warga/households',
        queryParameters: {
          if (search != null && search.trim().isNotEmpty)
            'search': search.trim(),
          'page': 1,
          'page_size': 50,
        },
        options: _deviceOptions(session),
      );
      final list = res.data?['data'] as List? ?? [];
      return list
          .whereType<Map<String, dynamic>>()
          .map(PosHouseholdSummary.fromJson)
          .toList();
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal memuat data warga RT');
    }
  }

  /// Paged variant used by the kiosk Warga tab with server-side search.
  Future<PosHouseholdPage> fetchPosHouseholdsPage(
    PosDeviceSession session, {
    String? search,
    int page = 1,
    int perPage = 20,
  }) async {
    try {
      final res = await _deviceDio.get<Map<String, dynamic>>(
        '${session.serverUrl}/api/v1/security-device/warga/households',
        queryParameters: {
          if (search != null && search.trim().isNotEmpty)
            'search': search.trim(),
          'page': page,
          'page_size': perPage,
        },
        options: _deviceOptions(session),
      );
      final list = res.data?['data'] as List? ?? [];
      final pag = res.data?['pagination'] as Map? ?? {};
      final total = (pag['total'] as num?)?.toInt() ?? list.length;
      final totalPages = (pag['total_pages'] as num?)?.toInt() ?? 1;
      return PosHouseholdPage(
        list
            .whereType<Map<String, dynamic>>()
            .map(PosHouseholdSummary.fromJson)
            .toList(),
        total,
        totalPages,
      );
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal memuat data warga RT');
    }
  }

  Future<PosHouseholdSummary> fetchPosHouseholdDetail(
    PosDeviceSession session,
    String id,
  ) async {
    try {
      final res = await _deviceDio.get<Map<String, dynamic>>(
        '${session.serverUrl}/api/v1/security-device/warga/households/$id',
        options: _deviceOptions(session),
      );
      return PosHouseholdSummary.fromJson(res.data ?? {});
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal memuat detail rumah warga');
    }
  }

  Future<PosWasteMonitoringResponse> fetchPosMonitoringSampah(
    PosDeviceSession session,
  ) async {
    try {
      final res = await _deviceDio.get<Map<String, dynamic>>(
        '${session.serverUrl}/api/v1/security-device/monitoring/sampah',
        options: _deviceOptions(session),
      );
      return PosWasteMonitoringResponse.fromJson(res.data ?? {});
    } on DioException catch (e) {
      throw _extractDioError(e, 'Gagal memuat daftar sampah belum diangkut');
    }
  }
}
