import 'client.dart';

/// Service for device registration and push notification endpoints.
class DeviceService {
  final ApiClient client;

  DeviceService(this.client);

  /// Registers an FCM device token for the currently authenticated user.
  Future<void> registerDevice({
    required String fcmToken,
    String platform = 'android',
    String? deviceId,
  }) async {
    await client.dio.post(
      '/api/v1/notifications/devices',
      data: {
        'fcm_token': fcmToken,
        'platform': platform,
        if (deviceId != null) 'device_id': deviceId,
      },
    );
  }

  /// Unregisters an FCM device token for the currently authenticated user.
  Future<void> unregisterDevice(String fcmToken) async {
    await client.dio.delete(
      '/api/v1/notifications/devices',
      data: {
        'fcm_token': fcmToken,
      },
    );
  }

  /// Triggers waste truck arrival alert to residents in the sender's RT (Pengurus only).
  Future<Map<String, dynamic>> triggerWasteTruckArrival({String? message}) async {
    final response = await client.dio.post(
      '/api/v1/notifications/waste-truck',
      data: {
        if (message != null && message.trim().isNotEmpty) 'message': message.trim(),
      },
    );
    if (response.data is Map) {
      return (response.data as Map).cast<String, dynamic>();
    }
    return {'status': 'accepted'};
  }
}

