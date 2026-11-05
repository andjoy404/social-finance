import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/core/api/device_service.dart';
import 'package:social_finance/core/notification/notification_url_helper.dart';
import 'package:social_finance/core/notification/push_notification_service.dart';
import 'package:social_finance/features/notification/data/models/notification_models.dart';
import 'package:social_finance/features/notification/data/notification_repository.dart';
import 'package:social_finance/features/notification/presentation/screens/notification_detail_screen.dart';

class _FakeDeviceService implements DeviceService {
  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);

  @override
  Future<void> registerDevice({
    required String fcmToken,
    String platform = 'android',
    String? deviceId,
  }) async {}

  @override
  Future<void> unregisterDevice(String fcmToken) async {}
}

class _FakeNotificationRepository implements NotificationRepository {
  final Map<String, NotificationLogEntry> logsById;

  _FakeNotificationRepository(this.logsById);

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);

  @override
  Future<NotificationLogEntry> fetchNotificationDetail(String id) async {
    final entry = logsById[id];
    if (entry == null) {
      throw Exception('Log not found');
    }
    return entry;
  }

  @override
  Future<void> markAsRead(String id) async {}
}

List<TextSpan> _collectLinkSpans(WidgetTester tester) {
  final links = <TextSpan>[];
  void visitSpan(InlineSpan span) {
    if (span is TextSpan) {
      if (span.recognizer is TapGestureRecognizer) {
        links.add(span);
      }
      final children = span.children;
      if (children != null) {
        for (final child in children) {
          visitSpan(child);
        }
      }
    }
  }

  for (final richText in tester.widgetList<RichText>(find.byType(RichText))) {
    visitSpan(richText.text);
  }
  return links;
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  const channel = MethodChannel('com.example.social_finance/notifications');
  final openedUrls = <String>[];

  setUp(() {
    openedUrls.clear();
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, (call) async {
      if (call.method == 'openExternalUrl') {
        final args = call.arguments as Map?;
        final url = args?['url']?.toString();
        if (url != null) {
          openedUrls.add(url);
        }
        return true;
      }
      return null;
    });
  });

  tearDown(() {
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, null);
  });

  group('Manual Notification URL Helpers & Tray Resolution', () {
    test('recognizes https:// and http:// URLs ONLY for manual_push', () {
      expect(
        isManualPushNotification(
          eventType: 'manual_push',
          dedupKey: 'manual_push:rt1:100',
        ),
        isTrue,
      );
      expect(
        isManualPushNotification(
          eventType: 'manual_push',
          dedupKey: 'schedule:sched-1:2026-11-05',
        ),
        isFalse,
      );
      expect(
        isManualPushNotification(
          eventType: 'iuran_bill_created',
          dedupKey: 'bill:100',
        ),
        isFalse,
      );
      expect(
        isManualPushNotification(
          eventType: 'iuran_upcoming_due_3_months',
          dedupKey: 'schedule:sched-1',
        ),
        isFalse,
      );
      expect(
        isManualPushNotification(
          eventType: 'iuran_overdue_3_months',
          dedupKey: 'schedule:sched-2',
        ),
        isFalse,
      );
    });

    test('validates only http/https and rejects unsafe or malformed schemes', () {
      expect(
        validateSafeHttpUrl(
          'https://urena.id/download?file=laporan.pdf&token=abc_123',
        ),
        equals('https://urena.id/download?file=laporan.pdf&token=abc_123'),
      );
      expect(
        validateSafeHttpUrl('http://rt02.urena.id/pengumuman'),
        equals('http://rt02.urena.id/pengumuman'),
      );
      expect(validateSafeHttpUrl('javascript:alert(1)'), isNull);
      expect(validateSafeHttpUrl('data:text/html,<h1>x</h1>'), isNull);
      expect(validateSafeHttpUrl('file:///sdcard/Download/test.pdf'), isNull);
      expect(validateSafeHttpUrl('intent://scan/#Intent;end'), isNull);
      expect(validateSafeHttpUrl('market://details?id=com.example'), isNull);
      expect(validateSafeHttpUrl('https://'), isNull);
      expect(validateSafeHttpUrl('http://'), isNull);
      expect(validateSafeHttpUrl('https://.'), isNull);
      expect(validateSafeHttpUrl(''), isNull);
    });

    test(
        'notification tray tap resolves first valid HTTP/HTTPS URL ONLY for manual_push',
        () {
      // 1 & 7. Manual push notification with multiple URLs -> returns first valid HTTP/HTTPS URL
      final manualWithUrls = resolveManualNotificationTrayExternalUrl(
        '/home/notifikasi/detail/notif-1',
        payload: {
          'type': 'manual_push',
          'dedup_key': 'manual_push:rt1:1',
          'body':
              'Unduh laporan di https://urena.id/files/lap.pdf?dl=1 atau http://urena.id/cadangan.',
        },
      );
      expect(manualWithUrls, equals('https://urena.id/files/lap.pdf?dl=1'));

      // 3. Manual push notification without URL -> returns null (preserves normal in-app navigation)
      final manualWithoutUrl = resolveManualNotificationTrayExternalUrl(
        '/home/notifikasi/detail/notif-2',
        payload: {
          'type': 'manual_push',
          'dedup_key': 'manual_push:rt1:2',
          'body': 'Kerja bakti hari Minggu pukul 07.00 WIB.',
        },
      );
      expect(manualWithoutUrl, isNull);

      // 4. System notification with URL -> returns null (never opens external browser)
      final systemWithUrl = resolveManualNotificationTrayExternalUrl(
        '/home/iuran/detail/bill-1',
        payload: {
          'type': 'iuran_bill_created',
          'body': 'Lihat tagihan di https://urena.id/tagihan/1',
        },
      );
      expect(systemWithUrl, isNull);

      // 5. Scheduler notifications (`iuran_upcoming_due_3_months` & `iuran_overdue_3_months`) -> returns null
      final upcomingWithUrl = resolveManualNotificationTrayExternalUrl(
        '/home/iuran/detail/bill-2',
        payload: {
          'type': 'iuran_upcoming_due_3_months',
          'dedup_key': 'schedule:sched-1:2026-11-05:bill-2:u1',
          'body': 'Bayar sebelum jatuh tempo di https://urena.id/pay',
        },
      );
      expect(upcomingWithUrl, isNull);

      final overdueWithUrl = resolveManualNotificationTrayExternalUrl(
        '/home/iuran/detail/bill-3',
        payload: {
          'type': 'iuran_overdue_3_months',
          'dedup_key': 'schedule:sched-2:2026-11-05:bill-3:u1',
          'body': 'Segera lunasi tunggakan di https://urena.id/overdue',
        },
      );
      expect(overdueWithUrl, isNull);

      // 6. Manual push with only non-HTTP/HTTPS or malformed URL -> returns null
      final manualUnsafeUrl = resolveManualNotificationTrayExternalUrl(
        '/home/notifikasi/detail/notif-3',
        payload: {
          'type': 'manual_push',
          'body': 'Tautan: javascript:alert(1) intent://open https://.',
        },
      );
      expect(manualUnsafeUrl, isNull);
    });
  });

  group('Android Widget Rendering for Manual vs System/Scheduler Notifications',
      () {
    testWidgets(
        'renders tappable https:// and http:// links in NotificationDetailScreen ONLY for manual_push',
        (tester) async {
      final pushService = PushNotificationService(_FakeDeviceService());
      final fakeRepo = _FakeNotificationRepository({
        'log-manual-1': NotificationLogEntry(
          id: 'log-manual-1',
          rtId: 'rt-1',
          eventType: 'manual_push',
          dedupKey: 'manual_push:rt-1:1',
          title: 'Dokumen Rapat Warga',
          body:
              'Unduh notulen di https://urena.id/notulen.pdf?ver=1 dan jadwal di http://rt02.urena.id/jadwal.',
          variables: const {},
          recipientsCount: 10,
          isRead: false,
          createdAt: DateTime.parse('2026-11-05T08:00:00Z'),
        ),
      });

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            pushNotificationServiceProvider.overrideWithValue(pushService),
            notificationRepositoryProvider.overrideWithValue(fakeRepo),
          ],
          child: const MaterialApp(
            home: NotificationDetailScreen(
              notificationId: 'log-manual-1',
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      final linkSpans = _collectLinkSpans(tester);
      expect(linkSpans, hasLength(2));
      expect(
        linkSpans[0].text,
        equals('https://urena.id/notulen.pdf?ver=1'),
      );
      expect(
        linkSpans[1].text,
        equals('http://rt02.urena.id/jadwal'),
      );

      // Tap both links and verify native openExternalUrl is invoked with exact URLs
      (linkSpans[0].recognizer as TapGestureRecognizer).onTap!();
      await tester.pump();
      (linkSpans[1].recognizer as TapGestureRecognizer).onTap!();
      await tester.pump();

      expect(
        openedUrls,
        equals([
          'https://urena.id/notulen.pdf?ver=1',
          'http://rt02.urena.id/jadwal',
        ]),
      );
    });

    testWidgets(
        'does NOT render tappable links for system or scheduler notifications even when body contains URL',
        (tester) async {
      final pushService = PushNotificationService(_FakeDeviceService());

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            pushNotificationServiceProvider.overrideWithValue(pushService),
          ],
          child: MaterialApp(
            home: Scaffold(
              body: Column(
                children: [
                  NotificationBodyText(
                    text: 'Tagihan di https://urena.id/bill/123',
                    eventType: 'iuran_bill_created',
                    dedupKey: 'bill:123',
                    onOpenUrl: pushService.openExternalUrl,
                  ),
                  NotificationBodyText(
                    text: 'Jatuh tempo di https://urena.id/upcoming/123',
                    eventType: 'iuran_upcoming_due_3_months',
                    dedupKey: 'schedule:s1:2026-11-05',
                    onOpenUrl: pushService.openExternalUrl,
                  ),
                  NotificationBodyText(
                    text: 'Tunggakan di https://urena.id/overdue/123',
                    eventType: 'iuran_overdue_3_months',
                    dedupKey: 'schedule:s2:2026-11-05',
                    onOpenUrl: pushService.openExternalUrl,
                  ),
                  NotificationBodyText(
                    text:
                        'Skema berbahaya javascript:alert(1) data:text/html,hi file:///a intent://b market://c',
                    eventType: 'manual_push',
                    dedupKey: 'manual_push:rt1:99',
                    onOpenUrl: pushService.openExternalUrl,
                  ),
                ],
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      final linkSpans = _collectLinkSpans(tester);
      expect(linkSpans, isEmpty);
      expect(openedUrls, isEmpty);
    });
  });
}
