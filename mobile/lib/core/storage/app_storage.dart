import 'dart:convert';
import 'dart:developer' as developer;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// Stored authentication session data.
class StoredSession {
  final String accessToken;
  final String refreshToken;
  final String serverUrl;
  final Map<String, dynamic> userData;

  const StoredSession({
    required this.accessToken,
    required this.refreshToken,
    required this.serverUrl,
    required this.userData,
  });

  Map<String, dynamic> toJson() => {
    'access_token': accessToken,
    'refresh_token': refreshToken,
    'server_url': serverUrl,
    'user_data': userData,
  };

  factory StoredSession.fromJson(Map<String, dynamic> json) {
    return StoredSession(
      accessToken: json['access_token'] as String? ?? '',
      refreshToken: json['refresh_token'] as String? ?? '',
      serverUrl: json['server_url'] as String? ?? '',
      userData: (json['user_data'] as Map?)?.cast<String, dynamic>() ?? {},
    );
  }
}

/// Abstract contract for device preferences (non-sensitive data).
abstract class PreferencesStorage {
  Future<String?> getLastApiUrl();
  Future<void> setLastApiUrl(String url);
  Future<void> clearLastApiUrl();

  Future<String?> getLastEmail();
  Future<void> setLastEmail(String email);
  Future<void> clearLastEmail();

  Future<String?> getLastPhone();
  Future<void> setLastPhone(String phone);
  Future<void> clearLastPhone();

  Future<String?> getLastLoginMethod();
  Future<void> setLastLoginMethod(String method);

  Future<bool> getRememberMe();
  Future<void> setRememberMe(bool value);
}

/// SharedPreferences implementation of [PreferencesStorage].
class SharedPrefsStorage implements PreferencesStorage {
  static const _keyLastApiUrl = 'sf_last_api_url';
  static const _keyLastEmail = 'sf_last_email';
  static const _keyLastPhone = 'sf_last_phone';
  static const _keyLastLoginMethod = 'sf_last_login_method';
  static const _keyRememberMe = 'sf_remember_me';

  @override
  Future<String?> getLastApiUrl() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      final url = prefs.getString(_keyLastApiUrl);
      if (url != null && url.trim().isNotEmpty) {
        return url.trim();
      }
      return null;
    } catch (e) {
      developer.log(
        'Error reading last API URL: $e',
        name: 'SharedPrefsStorage',
      );
      return null;
    }
  }

  @override
  Future<void> setLastApiUrl(String url) async {
    try {
      final prefs = await SharedPreferences.getInstance();
      await prefs.setString(_keyLastApiUrl, url.trim());
    } catch (e) {
      developer.log(
        'Error saving last API URL: $e',
        name: 'SharedPrefsStorage',
      );
    }
  }

  @override
  Future<void> clearLastApiUrl() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      await prefs.remove(_keyLastApiUrl);
    } catch (e) {
      developer.log(
        'Error clearing last API URL: $e',
        name: 'SharedPrefsStorage',
      );
    }
  }

  @override
  Future<String?> getLastEmail() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      final email = prefs.getString(_keyLastEmail);
      if (email != null && email.trim().isNotEmpty) {
        return email.trim();
      }
      return null;
    } catch (e) {
      developer.log('Error reading last email: $e', name: 'SharedPrefsStorage');
      return null;
    }
  }

  @override
  Future<void> setLastEmail(String email) async {
    try {
      final prefs = await SharedPreferences.getInstance();
      await prefs.setString(_keyLastEmail, email.trim());
    } catch (e) {
      developer.log('Error saving last email: $e', name: 'SharedPrefsStorage');
    }
  }

  @override
  Future<void> clearLastEmail() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      await prefs.remove(_keyLastEmail);
    } catch (e) {
      developer.log('Error clearing last email: $e', name: 'SharedPrefsStorage');
    }
  }

  @override
  Future<String?> getLastPhone() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      final phone = prefs.getString(_keyLastPhone);
      if (phone != null && phone.trim().isNotEmpty) {
        return phone.trim();
      }
      return null;
    } catch (e) {
      developer.log('Error reading last phone: $e', name: 'SharedPrefsStorage');
      return null;
    }
  }

  @override
  Future<void> setLastPhone(String phone) async {
    try {
      final prefs = await SharedPreferences.getInstance();
      await prefs.setString(_keyLastPhone, phone.trim());
    } catch (e) {
      developer.log('Error saving last phone: $e', name: 'SharedPrefsStorage');
    }
  }

  @override
  Future<void> clearLastPhone() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      await prefs.remove(_keyLastPhone);
    } catch (e) {
      developer.log('Error clearing last phone: $e', name: 'SharedPrefsStorage');
    }
  }

  @override
  Future<String?> getLastLoginMethod() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      return prefs.getString(_keyLastLoginMethod);
    } catch (e) {
      developer.log('Error reading last login method: $e', name: 'SharedPrefsStorage');
      return null;
    }
  }

  @override
  Future<void> setLastLoginMethod(String method) async {
    try {
      final prefs = await SharedPreferences.getInstance();
      await prefs.setString(_keyLastLoginMethod, method);
    } catch (e) {
      developer.log('Error saving last login method: $e', name: 'SharedPrefsStorage');
    }
  }

  @override
  Future<bool> getRememberMe() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      // Default to true as per specification
      return prefs.getBool(_keyRememberMe) ?? true;
    } catch (e) {
      developer.log(
        'Error reading remember me: $e',
        name: 'SharedPrefsStorage',
      );
      return true;
    }
  }

  @override
  Future<void> setRememberMe(bool value) async {
    try {
      final prefs = await SharedPreferences.getInstance();
      await prefs.setBool(_keyRememberMe, value);
    } catch (e) {
      developer.log('Error saving remember me: $e', name: 'SharedPrefsStorage');
    }
  }
}

/// In-memory implementation of [PreferencesStorage] for unit & widget tests.
class InMemoryPreferencesStorage implements PreferencesStorage {
  String? _lastApiUrl;
  String? _lastEmail;
  String? _lastPhone;
  String? _lastLoginMethod;
  bool _rememberMe = true;

  InMemoryPreferencesStorage({
    String? initialUrl,
    String? initialEmail,
    String? initialPhone,
    String? initialLoginMethod,
    bool initialRememberMe = true,
  }) : _lastApiUrl = initialUrl,
       _lastEmail = initialEmail,
       _lastPhone = initialPhone,
       _lastLoginMethod = initialLoginMethod,
       _rememberMe = initialRememberMe;

  @override
  Future<String?> getLastApiUrl() async => _lastApiUrl;

  @override
  Future<void> setLastApiUrl(String url) async {
    _lastApiUrl = url;
  }

  @override
  Future<void> clearLastApiUrl() async {
    _lastApiUrl = null;
  }

  @override
  Future<String?> getLastEmail() async => _lastEmail;

  @override
  Future<void> setLastEmail(String email) async {
    _lastEmail = email;
  }

  @override
  Future<void> clearLastEmail() async {
    _lastEmail = null;
  }

  @override
  Future<String?> getLastPhone() async => _lastPhone;

  @override
  Future<void> setLastPhone(String phone) async {
    _lastPhone = phone;
  }

  @override
  Future<void> clearLastPhone() async {
    _lastPhone = null;
  }

  @override
  Future<String?> getLastLoginMethod() async => _lastLoginMethod;

  @override
  Future<void> setLastLoginMethod(String method) async {
    _lastLoginMethod = method;
  }

  @override
  Future<bool> getRememberMe() async => _rememberMe;

  @override
  Future<void> setRememberMe(bool value) async {
    _rememberMe = value;
  }
}

/// Abstract contract for secure token and session storage.
abstract class SecureAuthStorage {
  Future<void> saveSession({
    required String accessToken,
    required String refreshToken,
    required String serverUrl,
    required Map<String, dynamic> userData,
  });

  Future<StoredSession?> getSession();
  Future<void> clearSession();
  Future<bool> hasSession();
}

/// FlutterSecureStorage implementation of [SecureAuthStorage].
class FlutterSecureAuthStorage implements SecureAuthStorage {
  static const _keyAccessToken = 'sf_sec_access_token';
  static const _keyRefreshToken = 'sf_sec_refresh_token';
  static const _keyServerUrl = 'sf_sec_server_url';
  static const _keyUserData = 'sf_sec_user_data';

  final FlutterSecureStorage _storage;

  FlutterSecureAuthStorage({FlutterSecureStorage? storage})
    : _storage = storage ?? const FlutterSecureStorage();

  @override
  Future<void> saveSession({
    required String accessToken,
    required String refreshToken,
    required String serverUrl,
    required Map<String, dynamic> userData,
  }) async {
    try {
      await _storage.write(key: _keyAccessToken, value: accessToken);
      await _storage.write(key: _keyRefreshToken, value: refreshToken);
      await _storage.write(key: _keyServerUrl, value: serverUrl);
      await _storage.write(key: _keyUserData, value: jsonEncode(userData));
    } catch (e) {
      developer.log(
        'Error saving session to secure storage: $e',
        name: 'FlutterSecureAuthStorage',
      );
    }
  }

  @override
  Future<StoredSession?> getSession() async {
    try {
      final accessToken = await _storage.read(key: _keyAccessToken);
      final refreshToken = await _storage.read(key: _keyRefreshToken) ?? '';
      final serverUrl = await _storage.read(key: _keyServerUrl) ?? '';
      final userDataStr = await _storage.read(key: _keyUserData);

      if (accessToken == null || accessToken.trim().isEmpty) {
        return null;
      }

      Map<String, dynamic> userData = {};
      if (userDataStr != null && userDataStr.isNotEmpty) {
        try {
          final decoded = jsonDecode(userDataStr);
          if (decoded is Map) {
            userData = decoded.cast<String, dynamic>();
          }
        } catch (_) {}
      }

      return StoredSession(
        accessToken: accessToken,
        refreshToken: refreshToken,
        serverUrl: serverUrl,
        userData: userData,
      );
    } catch (e) {
      developer.log(
        'Error reading session from secure storage: $e',
        name: 'FlutterSecureAuthStorage',
      );
      return null;
    }
  }

  @override
  Future<void> clearSession() async {
    try {
      await _storage.delete(key: _keyAccessToken);
      await _storage.delete(key: _keyRefreshToken);
      await _storage.delete(key: _keyServerUrl);
      await _storage.delete(key: _keyUserData);
    } catch (e) {
      developer.log(
        'Error clearing session in secure storage: $e',
        name: 'FlutterSecureAuthStorage',
      );
    }
  }

  @override
  Future<bool> hasSession() async {
    try {
      final token = await _storage.read(key: _keyAccessToken);
      return token != null && token.trim().isNotEmpty;
    } catch (e) {
      return false;
    }
  }
}

/// In-memory implementation of [SecureAuthStorage] for testing.
class InMemorySecureAuthStorage implements SecureAuthStorage {
  StoredSession? _session;

  InMemorySecureAuthStorage({StoredSession? initialSession})
    : _session = initialSession;

  @override
  Future<void> saveSession({
    required String accessToken,
    required String refreshToken,
    required String serverUrl,
    required Map<String, dynamic> userData,
  }) async {
    _session = StoredSession(
      accessToken: accessToken,
      refreshToken: refreshToken,
      serverUrl: serverUrl,
      userData: userData,
    );
  }

  @override
  Future<StoredSession?> getSession() async => _session;

  @override
  Future<void> clearSession() async {
    _session = null;
  }

  @override
  Future<bool> hasSession() async => _session != null;
}

final preferencesStorageProvider = Provider<PreferencesStorage>((ref) {
  return SharedPrefsStorage();
});

final secureAuthStorageProvider = Provider<SecureAuthStorage>((ref) {
  return FlutterSecureAuthStorage();
});
