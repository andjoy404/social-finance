import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:uuid/uuid.dart';

/// Generates and stores a persistent device identifier.
///
/// Returns a UUID that is unique per device install.
class DeviceIdService {
  static const _storage = FlutterSecureStorage(
    aOptions: AndroidOptions(resetOnError: true),
  );
  static const _key = '_device_id';
  String? _cachedDeviceId;

  Future<String> getDeviceId() async {
    if (_cachedDeviceId != null && _cachedDeviceId!.isNotEmpty) {
      return _cachedDeviceId!;
    }

    try {
      // First try reading from secure storage (persisted UUID).
      final stored = await _storage.read(key: _key);
      if (stored != null && stored.isNotEmpty) {
        _cachedDeviceId = stored;
        return stored;
      }
    } catch (_) {
      // Fall through to generate an in-memory UUID if secure storage is
      // unavailable (e.g., unit tests or temporary Android Keystore lock).
    }

    // Generate a new UUID and attempt to persist it.
    final newId = const Uuid().v4();
    _cachedDeviceId = newId;
    try {
      await _storage.write(key: _key, value: newId);
    } catch (_) {}
    return newId;
  }

  /// Clears the stored device ID (used on logout if desired).
  Future<void> clearDeviceId() async {
    _cachedDeviceId = null;
    try {
      await _storage.delete(key: _key);
    } catch (_) {}
  }
}

final deviceIdServiceProvider = Provider<DeviceIdService>((ref) {
  return DeviceIdService();
});
