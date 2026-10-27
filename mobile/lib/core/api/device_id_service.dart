import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:uuid/uuid.dart';

/// Generates and stores a persistent device identifier.
///
/// Returns a UUID that is unique per device install.
class DeviceIdService {
  static const _storage = FlutterSecureStorage();
  static const _key = '_device_id';

  Future<String> getDeviceId() async {
    // First try reading from secure storage (persisted UUID).
    final stored = await _storage.read(key: _key);
    if (stored != null && stored.isNotEmpty) {
      return stored;
    }

    // Generate a new UUID and persist it.
    final newId = const Uuid().v4();
    await _storage.write(key: _key, value: newId);
    return newId;
  }

  /// Clears the stored device ID (used on logout if desired).
  Future<void> clearDeviceId() async {
    await _storage.delete(key: _key);
  }
}

final deviceIdServiceProvider = Provider<DeviceIdService>((ref) {
  return DeviceIdService();
});
