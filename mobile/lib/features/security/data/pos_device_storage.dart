import 'dart:convert';

import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import 'security_models.dart';

class PosDeviceSession {
  final String serverUrl;
  final String deviceToken;
  final SecurityDevice device;

  const PosDeviceSession({
    required this.serverUrl,
    required this.deviceToken,
    required this.device,
  });
}

/// Separate encrypted storage for Pos Security Device sessions.
/// Never mixes with user JWT tokens (`sf_access_token` / `sf_refresh_token`).
class PosDeviceStorage {
  static const _keyServerUrl = 'sf_pos_server_url';
  static const _keyDeviceToken = 'sf_pos_device_token';
  static const _keyDeviceJson = 'sf_pos_device_json';

  final FlutterSecureStorage _storage;

  PosDeviceStorage({FlutterSecureStorage? storage})
    : _storage = storage ?? const FlutterSecureStorage();

  Future<void> saveSession(PosDeviceSession session) async {
    await _storage.write(key: _keyServerUrl, value: session.serverUrl);
    await _storage.write(key: _keyDeviceToken, value: session.deviceToken);
    await _storage.write(
      key: _keyDeviceJson,
      value: jsonEncode(session.device.toJson()),
    );
  }

  Future<PosDeviceSession?> loadSession() async {
    try {
      final serverUrl = await _storage.read(key: _keyServerUrl);
      final token = await _storage.read(key: _keyDeviceToken);
      final rawDevice = await _storage.read(key: _keyDeviceJson);
      if (serverUrl == null ||
          serverUrl.trim().isEmpty ||
          token == null ||
          token.trim().isEmpty ||
          rawDevice == null ||
          rawDevice.trim().isEmpty) {
        return null;
      }
      final map = jsonDecode(rawDevice) as Map<String, dynamic>;
      return PosDeviceSession(
        serverUrl: serverUrl.trim(),
        deviceToken: token.trim(),
        device: SecurityDevice.fromJson(map),
      );
    } catch (_) {
      return null;
    }
  }

  Future<void> clearSession() async {
    await _storage.delete(key: _keyServerUrl);
    await _storage.delete(key: _keyDeviceToken);
    await _storage.delete(key: _keyDeviceJson);
  }
}
