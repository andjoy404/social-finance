import 'dart:async';
import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/device_service.dart';

/// Top-level background message handler for FCM.
/// Must be annotated with `@pragma('vm:entry-point')` for Flutter execution.
@pragma('vm:entry-point')
Future<void> firebaseMessagingBackgroundHandler(RemoteMessage message) async {
  try {
    if (Firebase.apps.isEmpty) {
      await Firebase.initializeApp();
    }
  } catch (_) {
    // Graceful fallback if already initialized or in test environment
  }
  if (kDebugMode) {
    debugPrint('FCM background message: ${message.messageId} - ${message.notification?.title}');
  }
}

/// Service managing FCM lifecycle, token sync with backend, and message listeners.
class PushNotificationService {
  static const MethodChannel _nativeChannel =
      MethodChannel('com.example.social_finance/notifications');
  static bool _firebaseCoreUnavailable = false;

  final DeviceService _deviceService;
  bool _isInitialized = false;
  Future<void>? _initInFlight;
  bool _isUserLoggedIn = false;
  String? _registeredToken;
  StreamSubscription<String>? _tokenRefreshSub;
  StreamSubscription<RemoteMessage>? _foregroundSub;
  StreamSubscription<RemoteMessage>? _messageOpenedSub;
  final StreamController<RemoteMessage> _foregroundMessageController =
      StreamController<RemoteMessage>.broadcast();
  final StreamController<RemoteMessage> _messageOpenedController =
      StreamController<RemoteMessage>.broadcast();
  final StreamController<String> _nativeTappedController =
      StreamController<String>.broadcast();
  final StreamController<Map<String, dynamic>> _nativePayloadTappedController =
      StreamController<Map<String, dynamic>>.broadcast();
  final Set<String> _processedMessageIds = <String>{};

  PushNotificationService(this._deviceService);

  bool get isInitialized => _isInitialized;
  bool get isUserLoggedIn => _isUserLoggedIn;
  String? get registeredToken => _registeredToken;
  Stream<RemoteMessage> get onForegroundMessage => _foregroundMessageController.stream;
  Stream<RemoteMessage> get onMessageOpenedApp => _messageOpenedController.stream;
  Stream<String> get onNativeNotificationTapped => _nativeTappedController.stream;
  Stream<Map<String, dynamic>> get onNativeNotificationPayloadTapped =>
      _nativePayloadTappedController.stream;

  void _registerNativeMethodHandler() {
    _nativeChannel.setMethodCallHandler((call) async {
      if (call.method == 'onNotificationTapped') {
        if (call.arguments is Map) {
          final map = Map<String, dynamic>.from(call.arguments as Map);
          _nativePayloadTappedController.add(map);
          final link = map['deep_link'] as String?;
          if (link != null && link.isNotEmpty) {
            _nativeTappedController.add(link);
          }
        } else if (call.arguments is String) {
          final link = call.arguments as String;
          if (link.isNotEmpty) {
            _nativePayloadTappedController.add({'deep_link': link});
            _nativeTappedController.add(link);
          }
        }
      }
    });
  }

  /// Initializes Firebase and FCM listeners.
  /// Gracefully catches failures if Firebase/Google Play services are unavailable.
  Future<void> initialize({
    FirebaseMessaging? messagingInstance,
    bool requestPermissionOnStartup = true,
  }) async {
    _registerNativeMethodHandler();
    if (_isInitialized) return;
    if (_initInFlight != null) {
      return _initInFlight!;
    }

    final future = _doInitialize(
      messagingInstance: messagingInstance,
      requestPermissionOnStartup: requestPermissionOnStartup,
    );
    _initInFlight = future;
    try {
      await future;
    } finally {
      _initInFlight = null;
    }
  }

  Future<void> _doInitialize({
    FirebaseMessaging? messagingInstance,
    bool requestPermissionOnStartup = true,
  }) async {
    if (messagingInstance == null && _firebaseCoreUnavailable) {
      return;
    }

    try {
      if (messagingInstance == null && Firebase.apps.isEmpty) {
        await Firebase.initializeApp();
      }

      final messaging = messagingInstance ?? FirebaseMessaging.instance;

      if (requestPermissionOnStartup) {
        final settings = await messaging.requestPermission(
          alert: true,
          announcement: false,
          badge: true,
          carPlay: false,
          criticalAlert: false,
          provisional: false,
          sound: true,
        );
        if (kDebugMode) {
          debugPrint('PushNotificationService: Permission status: ${settings.authorizationStatus}');
        }
      }

      // Register background handler
      FirebaseMessaging.onBackgroundMessage(firebaseMessagingBackgroundHandler);

      // Listen for token updates from FCM
      _tokenRefreshSub = messaging.onTokenRefresh.listen((newToken) {
        _onTokenRefreshed(newToken);
      });

      // Listen for foreground messages with deduplication and delay logging
      _foregroundSub = FirebaseMessaging.onMessage.listen(_handleForegroundMessage);

      // Listen for message opened app interactions
      _messageOpenedSub = FirebaseMessaging.onMessageOpenedApp.listen((message) {
        if (kDebugMode) {
          debugPrint('FCM message opened app: ${message.messageId}');
        }
        _messageOpenedController.add(message);
      });

      _isInitialized = true;
    } catch (e) {
      if (messagingInstance == null && Firebase.apps.isEmpty) {
        _firebaseCoreUnavailable = true;
      }
      if (kDebugMode) {
        debugPrint('PushNotificationService initialization deferred/unavailable: $e');
      }
      _isInitialized = false;
    }
  }

  void _handleForegroundMessage(RemoteMessage message) {
    if (message.messageId != null) {
      if (_processedMessageIds.contains(message.messageId)) {
        if (kDebugMode) {
          debugPrint('FCM foreground message duplicate suppressed: ${message.messageId}');
        }
        return;
      }
      _processedMessageIds.add(message.messageId!);
      if (_processedMessageIds.length > 100) {
        _processedMessageIds.clear();
      }
    }

    // Delay audit
    final receivedAt = DateTime.now().toUtc();
    final sentAtStr = message.data['sent_at'] as String?;
    if (sentAtStr != null) {
      final sentAt = DateTime.tryParse(sentAtStr);
      if (sentAt != null) {
        final diffMs = receivedAt.difference(sentAt).inMilliseconds;
        debugPrint('FCM delivery delay: ${diffMs}ms (sent: $sentAtStr, received: ${receivedAt.toIso8601String()})');
      }
    }

    if (kDebugMode) {
      debugPrint('FCM foreground message: ${message.messageId} - ${message.notification?.title}');
    }
    _foregroundMessageController.add(message);
  }

  /// Test helper to emit a simulated foreground FCM message through deduplication.
  @visibleForTesting
  void simulateForegroundMessage(RemoteMessage message) {
    _handleForegroundMessage(message);
  }

  /// Test helper to emit a simulated notification tap from FCM background.
  @visibleForTesting
  void simulateMessageOpenedApp(RemoteMessage message) {
    _messageOpenedController.add(message);
  }

  /// Posts a native Android heads-up notification banner at the top of the screen.
  Future<void> showHeadsUpNotification({
    required String title,
    required String body,
    required String deepLink,
    String? imageUrl,
    String? notificationId,
    String? type,
    String? sentAt,
    int? id,
  }) async {
    if (kIsWeb || defaultTargetPlatform != TargetPlatform.android) return;
    try {
      await _nativeChannel.invokeMethod('showHeadsUpNotification', {
        'title': title,
        'body': body,
        'deep_link': deepLink,
        'image_url': imageUrl,
        if (notificationId != null && notificationId.isNotEmpty)
          'notification_id': notificationId,
        if (type != null && type.isNotEmpty) 'type': type,
        if (sentAt != null && sentAt.isNotEmpty) 'sent_at': sentAt,
        'id': id ?? (DateTime.now().millisecondsSinceEpoch % 100000),
      });
    } catch (e) {
      if (kDebugMode) {
        debugPrint('PushNotificationService: failed to show native heads-up notification: $e');
      }
    }
  }

  /// Gets the initial deep link if the app was launched by tapping a native notification.
  Future<String?> getInitialNativeDeepLink() async {
    if (kIsWeb || defaultTargetPlatform != TargetPlatform.android) return null;
    try {
      final link = await _nativeChannel.invokeMethod<String>('getInitialDeepLink');
      return link;
    } catch (e) {
      return null;
    }
  }

  /// Gets the full initial notification payload if the app was launched from a native notification tap.
  Future<Map<String, dynamic>?> getInitialNativeNotificationPayload() async {
    if (kIsWeb || defaultTargetPlatform != TargetPlatform.android) return null;
    try {
      final raw = await _nativeChannel.invokeMethod<dynamic>('getInitialNotificationPayload');
      if (raw is Map && raw.isNotEmpty) {
        return Map<String, dynamic>.from(raw);
      }
    } catch (_) {}
    final link = await getInitialNativeDeepLink();
    if (link != null && link.isNotEmpty) {
      return {'deep_link': link};
    }
    return null;
  }

  /// Gets the initial message if the app was launched by tapping a notification from terminated state.
  Future<RemoteMessage?> getInitialMessage({FirebaseMessaging? messagingInstance}) async {
    try {
      final messaging = messagingInstance ?? (Firebase.apps.isNotEmpty ? FirebaseMessaging.instance : null);
      if (messaging == null) return null;
      return await messaging.getInitialMessage();
    } catch (e) {
      if (kDebugMode) {
        debugPrint('PushNotificationService: getInitialMessage error: $e');
      }
      return null;
    }
  }

  /// Requests notification permission explicitly (recommended for Android 13+).
  Future<NotificationSettings?> requestPermission({FirebaseMessaging? messagingInstance}) async {
    try {
      final messaging = messagingInstance ?? (Firebase.apps.isNotEmpty ? FirebaseMessaging.instance : null);
      if (messaging == null) return null;
      return await messaging.requestPermission(
        alert: true,
        announcement: false,
        badge: true,
        carPlay: false,
        criticalAlert: false,
        provisional: false,
        sound: true,
      );
    } catch (e) {
      if (kDebugMode) {
        debugPrint('PushNotificationService: Failed to request permission: $e');
      }
      return null;
    }
  }

  /// Called after successful login or session restore to register device token with backend.
  Future<void> onUserLoggedIn({FirebaseMessaging? messagingInstance}) async {
    _isUserLoggedIn = true;
    try {
      if (!_isInitialized) {
        await initialize(messagingInstance: messagingInstance);
      }
      final messaging = messagingInstance ?? (Firebase.apps.isNotEmpty ? FirebaseMessaging.instance : null);
      if (messaging == null) {
        if (kDebugMode) {
          debugPrint('PushNotificationService: Firebase not ready for token registration');
        }
        return;
      }

      final token = await messaging.getToken();
      if (token != null && token.isNotEmpty) {
        if (_registeredToken != token) {
          await _deviceService.registerDevice(
            fcmToken: token,
            platform: 'android',
          );
          _registeredToken = token;
          if (kDebugMode) {
            debugPrint('PushNotificationService: Device token registered successfully');
          }
        }
      } else {
        if (kDebugMode) {
          debugPrint('PushNotificationService: No FCM token returned by provider');
        }
      }
    } catch (e) {
      if (kDebugMode) {
        debugPrint('PushNotificationService: Failed to register device token on login: $e');
      }
    }
  }

  /// Called on logout to unregister device from backend so other users don't receive alerts.
  /// Returns `true` if device was unregistered successfully or no token was active;
  /// returns `false` if unregister call failed over network (failure not disguised as success).
  Future<bool> onUserLoggedOut() async {
    _isUserLoggedIn = false;
    final tokenToUnregister = _registeredToken;
    _registeredToken = null;
    if (tokenToUnregister != null && tokenToUnregister.isNotEmpty) {
      try {
        await _deviceService.unregisterDevice(tokenToUnregister);
        if (kDebugMode) {
          debugPrint('PushNotificationService: Device token unregistered successfully on logout');
        }
        return true;
      } catch (e) {
        if (kDebugMode) {
          debugPrint('PushNotificationService: Failed to unregister device token on logout: $e');
        }
        return false;
      }
    }
    return true;
  }

  Future<void> _onTokenRefreshed(String newToken) async {
    if (_isUserLoggedIn) {
      try {
        await _deviceService.registerDevice(
          fcmToken: newToken,
          platform: 'android',
        );
        _registeredToken = newToken;
        if (kDebugMode) {
          debugPrint('PushNotificationService: Refreshed device token registered successfully');
        }
      } catch (e) {
        if (kDebugMode) {
          debugPrint('PushNotificationService: Failed to update refreshed device token: $e');
        }
      }
    }
  }

  void dispose() {
    _tokenRefreshSub?.cancel();
    _foregroundSub?.cancel();
    _messageOpenedSub?.cancel();
    _foregroundMessageController.close();
    _messageOpenedController.close();
    _nativeTappedController.close();
    _nativePayloadTappedController.close();
  }
}

final pushNotificationServiceProvider = Provider<PushNotificationService>((ref) {
  final deviceService = ref.watch(deviceServiceProvider);
  final service = PushNotificationService(deviceService);
  ref.onDispose(() => service.dispose());
  return service;
});

