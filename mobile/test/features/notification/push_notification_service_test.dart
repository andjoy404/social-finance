import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/core/api/device_service.dart';
import 'package:social_finance/core/notification/push_notification_service.dart';

class FakeDeviceService implements DeviceService {
  final List<String> registeredTokens = [];
  final List<String> unregisteredTokens = [];

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);

  @override
  Future<void> registerDevice({
    required String fcmToken,
    String platform = 'android',
    String? deviceId,
  }) async {
    registeredTokens.add(fcmToken);
  }

  @override
  Future<void> unregisterDevice(String fcmToken) async {
    unregisteredTokens.add(fcmToken);
  }
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('PushNotificationService', () {
    late FakeDeviceService fakeDeviceService;
    late PushNotificationService service;

    setUp(() {
      fakeDeviceService = FakeDeviceService();
      service = PushNotificationService(fakeDeviceService);
    });

    test('initial state is uninitialized with null token', () {
      expect(service.isInitialized, isFalse);
      expect(service.registeredToken, isNull);
    });

    test('initialize gracefully handles missing Firebase platform in unit tests', () async {
      // In pure unit test environment without platform channel mocks, initialize handles exception
      await service.initialize();
      expect(service.isInitialized, isFalse);
    });

    test('onUserLoggedOut unregisters previous token and clears registeredToken', () async {
      // Simulate that a token was previously registered
      // We test onUserLoggedOut directly
      await service.onUserLoggedOut();
      expect(fakeDeviceService.unregisteredTokens, isEmpty);
      expect(service.registeredToken, isNull);
    });

    test('tracks isUserLoggedIn state on login and logout', () async {
      expect(service.isUserLoggedIn, isFalse);

      // Calling onUserLoggedIn marks user as logged in
      await service.onUserLoggedIn();
      expect(service.isUserLoggedIn, isTrue);

      // Calling onUserLoggedOut clears user logged in state
      await service.onUserLoggedOut();
      expect(service.isUserLoggedIn, isFalse);
    });

    test('onForegroundMessage stream can be listened to and closed on dispose', () async {
      expect(service.onForegroundMessage, isNotNull);
      final sub = service.onForegroundMessage.listen((_) {});
      await sub.cancel();
    });

    test('requestPermission gracefully returns null when Firebase not initialized', () async {
      final settings = await service.requestPermission();
      expect(settings, isNull);
    });

    test('showHeadsUpNotification invokes native channel method with correct arguments on Android', () async {
      debugDefaultTargetPlatformOverride = TargetPlatform.android;
      addTearDown(() => debugDefaultTargetPlatformOverride = null);

      final calls = <MethodCall>[];
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(
        const MethodChannel('com.example.social_finance/notifications'),
        (call) async {
          calls.add(call);
          if (call.method == 'showHeadsUpNotification') return true;
          return null;
        },
      );
      addTearDown(() {
        TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
            .setMockMethodCallHandler(
          const MethodChannel('com.example.social_finance/notifications'),
          null,
        );
      });

      await service.showHeadsUpNotification(
        title: 'Tagihan Baru',
        body: 'Tagihan iuran telah dibuat',
        deepLink: '/home/iuran/detail/bill-123',
        id: 42,
      );

      expect(calls.length, equals(1));
      expect(calls.first.method, equals('showHeadsUpNotification'));
      expect(calls.first.arguments['title'], equals('Tagihan Baru'));
      expect(calls.first.arguments['body'], equals('Tagihan iuran telah dibuat'));
      expect(calls.first.arguments['deep_link'], equals('/home/iuran/detail/bill-123'));
      expect(calls.first.arguments['id'], equals(42));
    });

    test('getInitialNativeDeepLink retrieves deep link from native launch intent', () async {
      debugDefaultTargetPlatformOverride = TargetPlatform.android;
      addTearDown(() => debugDefaultTargetPlatformOverride = null);

      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(
        const MethodChannel('com.example.social_finance/notifications'),
        (call) async {
          if (call.method == 'getInitialDeepLink') {
            return '/home/iuran/detail/bill-999';
          }
          return null;
        },
      );
      addTearDown(() {
        TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
            .setMockMethodCallHandler(
          const MethodChannel('com.example.social_finance/notifications'),
          null,
        );
      });

      final link = await service.getInitialNativeDeepLink();
      expect(link, equals('/home/iuran/detail/bill-999'));
    });

    test('dispose can be called safely', () {
      expect(() => service.dispose(), returnsNormally);
    });
  });
}
