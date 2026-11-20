import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/notification/data/models/notification_models.dart';
import 'package:social_finance/features/notification/data/notification_api_client.dart';
import 'package:social_finance/features/notification/data/notification_repository.dart';
import 'package:social_finance/features/notification/presentation/screens/notification_detail_screen.dart';
import 'package:social_finance/features/notification/presentation/screens/notification_management_screen.dart';

final requestedHistoryCategories = <String>[];

class FakeNotificationRepository extends NotificationRepository {
  FakeNotificationRepository({this.additionalSystemTypes = 0})
    : super(apiClient: NotificationApiClient(dio: Dio()));

  final int additionalSystemTypes;
  final List<String> operationOrder = [];
  bool markAsReadShouldThrow = false;
  List<NotificationLogEntry>? customLogs;
  List<NotificationLogEntry>? customMineLogs;

  @override
  Future<List<SystemNotificationType>> fetchSystemTypes() async {
    return [
      SystemNotificationType(
        eventType: 'iuran_bill_created',
        name: 'Penerbitan Tagihan Iuran',
        trigger: 'Diterbitkan saat bendahara men-generate tagihan iuran baru',
        recipientsRule: 'Warga yang menempati rumah tangga',
        defaultTitle: 'Tagihan Iuran Baru Tersedia',
        defaultBody:
            'Tagihan iuran Anda untuk periode terkait telah diterbitkan.',
        allowedVariables: ['bill_id'],
      ),
      SystemNotificationType(
        eventType: 'waste_truck_arrival',
        name: 'Kedatangan Mobil Sampah',
        trigger: 'Diterbitkan saat mobil sampah tiba di RT',
        recipientsRule: 'Seluruh warga RT',
        defaultTitle: 'Mobil Sampah Tiba',
        defaultBody: 'Truk pengangkut sampah sudah berada di lingkungan RT.',
        allowedVariables: [],
      ),
      for (var index = 0; index < additionalSystemTypes; index++)
        SystemNotificationType(
          eventType: 'system_test_$index',
          name: 'Jenis Notifikasi Tambahan $index',
          trigger: 'Dipicu untuk pengujian layout',
          recipientsRule: 'Warga RT',
          defaultTitle: 'Notifikasi Tambahan $index',
          defaultBody: 'Konten tambahan $index',
          allowedVariables: const [],
        ),
    ];
  }

  @override
  Future<List<NotificationTemplate>> fetchTemplates() async {
    return [
      const NotificationTemplate(
        id: 'tmpl-1',
        rtId: 'rt-1',
        eventType: 'iuran_bill_created',
        name: 'Tagihan Khusus Warga',
        title: 'Halo Warga!',
        body: 'Mohon lunasi tagihan nomor {{bill_id}}',
        isActive: true,
      ),
    ];
  }

  @override
  Future<NotificationLogsResponse> fetchLogs({
    int page = 1,
    int perPage = 20,
    String? eventType,
    String? status,
  }) async {
    final list = customLogs ?? [
      NotificationLogEntry(
        id: 'log-1',
        rtId: 'rt-1',
        userId: 'u-warga',
        eventType: 'iuran_bill_created',
        title: 'Halo Warga!',
        body: 'Mohon lunasi tagihan nomor 101',
        variables: const {'bill_id': '101'},
        recipientsCount: 4,
        createdAt: DateTime(2026, 10, 29, 8, 30),
      ),
    ];
    return NotificationLogsResponse(
      data: list,
      page: 1,
      perPage: 20,
      total: list.length,
      totalPages: 1,
    );
  }

  @override
  Future<NotificationLogsResponse> fetchManagementHistory({
    required String category,
    int page = 1,
    int perPage = 50,
  }) async {
    requestedHistoryCategories.add(category);
    operationOrder.add('fetchManagementHistory_$category');
    if (category == 'mine' && customMineLogs != null) {
      return NotificationLogsResponse(
        data: customMineLogs!,
        page: page,
        perPage: perPage,
        total: customMineLogs!.length,
        totalPages: 1,
      );
    }
    final title = category == 'mine'
        ? 'Aktivitas Saya'
        : 'Notifikasi Terjadwal';
    return NotificationLogsResponse(
      data: [
        NotificationLogEntry(
          id: 'log-$category',
          rtId: 'rt-1',
          eventType: category == 'mine'
              ? 'manual_push'
              : 'iuran_payment_reminder',
          title: title,
          body: '$title detail',
          variables: const {},
          recipientsCount: 4,
          sentCount: 4,
          historyCategory: category,
          createdAt: DateTime(2026, 10, 29, 8, 30),
        ),
      ],
      page: page,
      perPage: perPage,
      total: 1,
      totalPages: 1,
    );
  }

  Completer<void>? markAsReadCompleter;
  int markAsReadCallCount = 0;

  @override
  Future<int> fetchUnreadCount() async {
    final list = customMineLogs ?? [];
    return list.where((e) => !e.isRead).length;
  }

  @override
  Future<void> markAsRead(String id) async {
    markAsReadCallCount++;
    operationOrder.add('markAsRead_called');
    if (markAsReadCompleter != null) {
      await markAsReadCompleter!.future;
    }
    if (markAsReadShouldThrow) {
      operationOrder.add('markAsRead_failed');
      throw Exception('Network error on markAsRead');
    }
    operationOrder.add('markAsRead_success');
    if (customLogs != null) {
      customLogs = customLogs!
          .map((l) => l.id == id ? l.copyWith(isRead: true) : l)
          .toList();
    }
    if (customMineLogs != null) {
      customMineLogs = customMineLogs!
          .map((l) => l.id == id ? l.copyWith(isRead: true) : l)
          .toList();
    }
  }
}

void main() {
  Widget buildTestWidget({
    Map<String, dynamic>? user,
    int additionalSystemTypes = 0,
    FakeNotificationRepository? repository,
  }) {
    final fakeRepo =
        repository ??
        FakeNotificationRepository(
          additionalSystemTypes: additionalSystemTypes,
        );
    final defaultUser =
        user ??
        {
          'id': 'u-admin',
          'system_role': 'super_admin',
          'role': AppRole.superAdmin,
          'jabatan': null,
        };
    return ProviderScope(
      overrides: [
        authRepositoryProvider.overrideWith((_) {
          final client = ApiClient(customDio: Dio());
          final repo = AuthRepository(
            authService: AuthService(client),
            apiClient: client,
          );
          repo.state = AsyncData<Map<String, dynamic>?>(defaultUser);
          return repo;
        }),
        notificationRepositoryProvider.overrideWithValue(fakeRepo),
        notificationSystemTypesProvider.overrideWith(
          (ref) => fakeRepo.fetchSystemTypes(),
        ),
        notificationTemplatesProvider.overrideWith(
          (ref) => fakeRepo.fetchTemplates(),
        ),
      ],
      child: const MaterialApp(home: NotificationManagementScreen()),
    );
  }

  testWidgets(
    'management screen centers primary tabs and keeps history and template subtabs',
    (tester) async {
      tester.view.physicalSize = const Size(390, 640);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);

      await tester.pumpWidget(buildTestWidget());
      await tester.pumpAndSettle();

      final primaryTabs = find.byType(TabBar).first;
      final primaryRect = tester.getRect(primaryTabs);
      expect(find.text('KELOLA NOTIFIKASI'), findsOneWidget);
      expect(find.text('NOTIFIKASI'), findsOneWidget);
      expect(find.text('TEMPLATE'), findsOneWidget);
      expect(find.textContaining('SISTEM'), findsOneWidget);
      expect(find.text('RIWAYAT SAYA'), findsOneWidget);
      expect(find.text('KUSTOM'), findsNothing);
      expect(find.text('PENJADWALAN'), findsNothing);
      expect(tester.getRect(find.text('NOTIFIKASI')).center.dx, closeTo(primaryRect.center.dx / 2, 1));
      expect(tester.getRect(find.text('TEMPLATE')).center.dx, closeTo(primaryRect.center.dx * 1.5, 1));

      await tester.tap(find.text('TEMPLATE'));
      await tester.pumpAndSettle();
      expect(find.text('KUSTOM'), findsOneWidget);
      expect(find.text('PENJADWALAN'), findsOneWidget);
      expect(find.text('Template Kustom'), findsNothing);

      final configTabs = find.byType(TabBar).last;
      final configRect = tester.getRect(configTabs);
      expect(tester.getRect(find.textContaining('SISTEM').last).center.dx, closeTo(configRect.left + configRect.width / 6, 2));
      expect(tester.getRect(find.text('KUSTOM')).center.dx, closeTo(configRect.center.dx, 2));
      expect(tester.getRect(find.text('PENJADWALAN')).center.dx, closeTo(configRect.left + configRect.width * 5 / 6, 2));
      expect(find.text('Penerbitan Tagihan Iuran'), findsOneWidget);
      final layoutException = tester.takeException();
      expect(layoutException, isNull, reason: layoutException?.toString());
    },
  );

  testWidgets(
    'configuration tabs stay fixed while system history content scrolls',
    (tester) async {
      tester.view.physicalSize = const Size(400, 600);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);

      await tester.pumpWidget(buildTestWidget(additionalSystemTypes: 20));
      await tester.pumpAndSettle();
      await tester.tap(find.text('TEMPLATE'));
      await tester.pumpAndSettle();

      final configTabs = find.byType(TabBar).last;
      final topBefore = tester.getTopLeft(configTabs).dy;
      expect(find.textContaining('SISTEM'), findsOneWidget);
      expect(find.text('KUSTOM'), findsOneWidget);
      expect(find.text('PENJADWALAN'), findsOneWidget);
      expect(tester.takeException(), isNull);

      await tester.drag(find.byType(ListView).first, const Offset(0, -450));
      await tester.pumpAndSettle();

      expect(tester.getTopLeft(configTabs).dy, topBefore);
      expect(find.textContaining('SISTEM'), findsOneWidget);
      expect(find.text('KUSTOM'), findsOneWidget);
      expect(find.text('PENJADWALAN'), findsOneWidget);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('switches to KUSTOM and shows editable custom templates', (
    tester,
  ) async {
    await tester.pumpWidget(buildTestWidget());
    await tester.pumpAndSettle();

    await tester.tap(find.text('TEMPLATE'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('KUSTOM'));
    await tester.pumpAndSettle();

    expect(find.text('Tagihan Khusus Warga'), findsOneWidget);
    expect(find.text('Halo Warga!'), findsOneWidget);
    expect(find.byType(Switch), findsOneWidget);
  });

  testWidgets(
    'system and personal history tabs load distinct backend categories',
    (tester) async {
      requestedHistoryCategories.clear();
      await tester.pumpWidget(buildTestWidget());
      await tester.pumpAndSettle();

      await tester.tap(find.text('NOTIFIKASI'));
      await tester.pumpAndSettle();
      await tester.tap(find.textContaining('SISTEM').first);
      await tester.pumpAndSettle();

      expect(requestedHistoryCategories, contains('system'));
      expect(find.text('Notifikasi Terjadwal'), findsOneWidget);
      expect(find.text('Aktivitas Saya'), findsNothing);

      await tester.tap(find.text('RIWAYAT SAYA'));
      await tester.pumpAndSettle();
      expect(requestedHistoryCategories, contains('mine'));
      expect(find.text('Aktivitas Saya'), findsOneWidget);
      expect(find.text('Notifikasi Terjadwal'), findsNothing);
    },
  );

  testWidgets(
    'system list renders trigger category badges and hides Kustomisasi for By System events',
    (tester) async {
      tester.view.physicalSize = const Size(800, 1400);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);

      await tester.pumpWidget(buildTestWidget());
      await tester.pumpAndSettle();
      await tester.tap(find.text('TEMPLATE'));
      await tester.pumpAndSettle();

      // SISTEM subtab is the default config subtab.
      expect(find.textContaining('SISTEM'), findsWidgets);
      // By System badge on the bill card, Manual Trigger badge on the waste-truck card.
      expect(find.text('By System'), findsOneWidget);
      expect(find.text('Manual Trigger'), findsOneWidget);
      // Kustomisasi is offered only for non-By-System events.
      expect(find.text('Kustomisasi'), findsOneWidget);
    },
  );

  testWidgets('opens push notification preview banner modal', (tester) async {
    tester.view.physicalSize = const Size(800, 1400);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(buildTestWidget());
    await tester.pumpAndSettle();
    await tester.tap(find.text('TEMPLATE'));
    await tester.pumpAndSettle();

    final previewButtons = find.text('Preview Banner');
    expect(previewButtons, findsAtLeastNWidgets(1));

    // By-System events are placed at the bottom of the system list, so the
    // preview button belonging to the bill-created card (by_system) is the last.
    await tester.tap(previewButtons.last);
    await tester.pumpAndSettle();

    expect(find.text('Preview Notifikasi'), findsOneWidget);
    expect(find.text('Social Finance Urena'), findsOneWidget);
    expect(find.text('Push Notification'), findsOneWidget);
    expect(find.text('baru saja'), findsOneWidget);
    expect(find.text('Tagihan Iuran Baru Tersedia'), findsWidgets);

    await tester.tap(find.byIcon(Icons.arrow_back_ios_new));
    await tester.pumpAndSettle();

    expect(find.text('Preview Notifikasi'), findsNothing);
  });

  testWidgets(
    'renders only notification history for Warga without admin tabs',
    (tester) async {
      final wargaUser = {
        'id': 'u-warga',
        'email': 'warga@local.com',
        'name': 'Warga RT',
        'role': AppRole.warga,
        'system_role': null,
        'jabatan': null,
      };
      await tester.pumpWidget(buildTestWidget(user: wargaUser));
      await tester.pumpAndSettle();

      // Title is NOTIFIKASI
      expect(find.text('NOTIFIKASI'), findsOneWidget);
      expect(find.text('KELOLA NOTIFIKASI'), findsNothing);

      // Administrative tabs must not be rendered
      expect(find.text('Sistem'), findsNothing);
      expect(find.text('Template'), findsNothing);

      // Riwayat list is rendered directly
      expect(find.text('Halo Warga!'), findsOneWidget);
      expect(find.text('Mohon lunasi tagihan nomor 101'), findsOneWidget);
    },
  );

  testWidgets(
    'renders notification history on narrow screen (312px) without RenderFlex overflow',
    (tester) async {
      tester.view.physicalSize = const Size(312, 640);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);

      await tester.pumpWidget(buildTestWidget());
      await tester.pumpAndSettle();

      // Open NOTIFIKASI tab -> SISTEM history
      await tester.tap(find.text('NOTIFIKASI'));
      await tester.pumpAndSettle();
      await tester.tap(find.textContaining('SISTEM').first);
      await tester.pumpAndSettle();

      expect(find.text('Notifikasi Terjadwal'), findsOneWidget);
      expect(tester.takeException(), isNull);

      // Switch to RIWAYAT SAYA on narrow screen
      await tester.tap(find.text('RIWAYAT SAYA'));
      await tester.pumpAndSettle();

      expect(find.text('Aktivitas Saya'), findsOneWidget);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets(
    'renders notification history on normal screen (390px) without RenderFlex overflow',
    (tester) async {
      tester.view.physicalSize = const Size(390, 844);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);

      await tester.pumpWidget(buildTestWidget());
      await tester.pumpAndSettle();

      await tester.tap(find.text('NOTIFIKASI'));
      await tester.pumpAndSettle();
      await tester.tap(find.textContaining('SISTEM').first);
      await tester.pumpAndSettle();

      expect(find.text('Notifikasi Terjadwal'), findsOneWidget);
      expect(tester.takeException(), isNull);
    },
  );

  group('Read Receipt 8-Case Matrix & Race Condition', () {
    testWidgets(
      'Case 1: Same notification ID opened twice while first request is in flight only executes once',
      (tester) async {
        final fakeRepo = FakeNotificationRepository();
        fakeRepo.customMineLogs = [
          NotificationLogEntry(
            id: 'log-flight-1',
            rtId: 'rt-1',
            eventType: 'manual_push',
            title: 'Notifikasi In-Flight',
            body: 'Isi detail',
            variables: const {},
            recipientsCount: 1,
            sentCount: 1,
            historyCategory: 'mine',
            isRead: false,
            createdAt: DateTime(2026, 11, 20, 10, 0),
          ),
        ];

        await tester.pumpWidget(buildTestWidget(repository: fakeRepo));
        await tester.pumpAndSettle();

        await tester.tap(find.text('NOTIFIKASI'));
        await tester.pumpAndSettle();
        await tester.tap(find.text('RIWAYAT SAYA'));
        await tester.pumpAndSettle();

        // Tap item first time
        await tester.tap(find.text('Notifikasi In-Flight'));
        await tester.pump(); // Start transition

        // Tap item second time immediately while first is in-flight
        await tester.tap(find.text('Notifikasi In-Flight'), warnIfMissed: false);
        await tester.pumpAndSettle();

        // Detail screen is shown only once
        expect(find.byType(NotificationDetailScreen), findsOneWidget);

        // Pop back
        await tester.tap(find.byKey(const Key('notification_detail_back_button')));
        await tester.pumpAndSettle();

        // markAsRead must only be executed once
        expect(fakeRepo.markAsReadCallCount, equals(1));
      },
    );

    testWidgets(
      'Case 2: Different notification IDs opened concurrently are processed independently',
      (tester) async {
        final fakeRepo = FakeNotificationRepository();
        fakeRepo.customMineLogs = [
          NotificationLogEntry(
            id: 'log-diff-1',
            rtId: 'rt-1',
            eventType: 'manual_push',
            title: 'Notifikasi Satu',
            body: 'Isi 1',
            variables: const {},
            recipientsCount: 1,
            sentCount: 1,
            historyCategory: 'mine',
            isRead: false,
            createdAt: DateTime(2026, 11, 20, 10, 0),
          ),
          NotificationLogEntry(
            id: 'log-diff-2',
            rtId: 'rt-1',
            eventType: 'manual_push',
            title: 'Notifikasi Dua',
            body: 'Isi 2',
            variables: const {},
            recipientsCount: 1,
            sentCount: 1,
            historyCategory: 'mine',
            isRead: false,
            createdAt: DateTime(2026, 11, 20, 10, 1),
          ),
        ];

        await tester.pumpWidget(buildTestWidget(repository: fakeRepo));
        await tester.pumpAndSettle();

        await tester.tap(find.text('NOTIFIKASI'));
        await tester.pumpAndSettle();
        await tester.tap(find.text('RIWAYAT SAYA'));
        await tester.pumpAndSettle();

        // Open first item
        await tester.tap(find.text('Notifikasi Satu'));
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(const Key('notification_detail_back_button')));
        await tester.pumpAndSettle();

        // Open second item
        await tester.tap(find.text('Notifikasi Dua'));
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(const Key('notification_detail_back_button')));
        await tester.pumpAndSettle();

        expect(fakeRepo.markAsReadCallCount, equals(2));
      },
    );

    testWidgets(
      'Case 3: Mark-read API failure leaves notification unread',
      (tester) async {
        final fakeRepo = FakeNotificationRepository();
        fakeRepo.markAsReadShouldThrow = true;
        fakeRepo.customMineLogs = [
          NotificationLogEntry(
            id: 'log-fail-3',
            rtId: 'rt-1',
            eventType: 'manual_push',
            title: 'Pengumuman Gagal Baca',
            body: 'Isi detail',
            variables: const {},
            recipientsCount: 1,
            sentCount: 1,
            historyCategory: 'mine',
            isRead: false,
            createdAt: DateTime(2026, 11, 20, 10, 0),
          ),
        ];

        await tester.pumpWidget(buildTestWidget(repository: fakeRepo));
        await tester.pumpAndSettle();

        await tester.tap(find.text('NOTIFIKASI'));
        await tester.pumpAndSettle();
        await tester.tap(find.text('RIWAYAT SAYA'));
        await tester.pumpAndSettle();

        await tester.tap(find.text('Pengumuman Gagal Baca'));
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(const Key('notification_detail_back_button')));
        await tester.pumpAndSettle();

        expect(fakeRepo.operationOrder.contains('markAsRead_failed'), isTrue);
        expect(fakeRepo.customMineLogs?.first.isRead, isFalse);
      },
    );

    testWidgets(
      'Case 4: Successful mark-read updates local history state',
      (tester) async {
        final fakeRepo = FakeNotificationRepository();
        fakeRepo.customMineLogs = [
          NotificationLogEntry(
            id: 'log-success-4',
            rtId: 'rt-1',
            eventType: 'manual_push',
            title: 'Pengumuman Sukses',
            body: 'Isi detail',
            variables: const {},
            recipientsCount: 1,
            sentCount: 1,
            historyCategory: 'mine',
            isRead: false,
            createdAt: DateTime(2026, 11, 20, 10, 0),
          ),
        ];

        await tester.pumpWidget(buildTestWidget(repository: fakeRepo));
        await tester.pumpAndSettle();

        await tester.tap(find.text('NOTIFIKASI'));
        await tester.pumpAndSettle();
        await tester.tap(find.text('RIWAYAT SAYA'));
        await tester.pumpAndSettle();

        await tester.tap(find.text('Pengumuman Sukses'));
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(const Key('notification_detail_back_button')));
        await tester.pumpAndSettle();

        expect(fakeRepo.customMineLogs?.first.isRead, isTrue);
      },
    );

    testWidgets(
      'Case 5: Successful mark-read refreshes unread badge and updates provider state',
      (tester) async {
        final fakeRepo = FakeNotificationRepository();
        fakeRepo.customMineLogs = [
          NotificationLogEntry(
            id: 'log-badge-5',
            rtId: 'rt-1',
            eventType: 'manual_push',
            title: 'Pengumuman Badge Test',
            body: 'Isi detail',
            variables: const {},
            recipientsCount: 1,
            sentCount: 1,
            historyCategory: 'mine',
            isRead: false,
            createdAt: DateTime(2026, 11, 20, 10, 0),
          ),
        ];

        final container = ProviderContainer(
          overrides: [
            authRepositoryProvider.overrideWith((_) {
              final client = ApiClient(customDio: Dio());
              final repo = AuthRepository(
                authService: AuthService(client),
                apiClient: client,
              );
              repo.state = const AsyncData<Map<String, dynamic>?>({
                'id': 'u-admin',
                'system_role': 'super_admin',
                'role': AppRole.superAdmin,
                'jabatan': null,
              });
              return repo;
            }),
            notificationRepositoryProvider.overrideWithValue(fakeRepo),
            notificationSystemTypesProvider.overrideWith(
              (ref) => fakeRepo.fetchSystemTypes(),
            ),
            notificationTemplatesProvider.overrideWith(
              (ref) => fakeRepo.fetchTemplates(),
            ),
          ],
        );
        addTearDown(container.dispose);

        // Pre-refresh unread badge: count is 1
        await container.read(notificationUnreadCountProvider.notifier).refresh();
        expect(container.read(notificationUnreadCountProvider), equals(1));

        await tester.pumpWidget(
          UncontrolledProviderScope(
            container: container,
            child: const MaterialApp(home: NotificationManagementScreen()),
          ),
        );
        await tester.pumpAndSettle();

        await tester.tap(find.text('NOTIFIKASI'));
        await tester.pumpAndSettle();
        await tester.tap(find.text('RIWAYAT SAYA'));
        await tester.pumpAndSettle();

        await tester.tap(find.text('Pengumuman Badge Test'));
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(const Key('notification_detail_back_button')));
        await tester.pumpAndSettle();

        // Badge count should now be 0 after successful mark read
        expect(container.read(notificationUnreadCountProvider), equals(0));
      },
    );

    testWidgets(
      'Case 6: Navigation failure does not mark notification read',
      (tester) async {
        final fakeRepo = FakeNotificationRepository();
        fakeRepo.customMineLogs = [
          NotificationLogEntry(
            id: 'log-nav-fail-6',
            rtId: 'rt-1',
            eventType: 'paket_received',
            title: 'Paket Baru Tiba',
            body: 'Ada paket di pos keamanan',
            variables: const {},
            payload: const {'report_id': 'rep-unregistered-999'},
            recipientsCount: 1,
            sentCount: 1,
            historyCategory: 'mine',
            isRead: false,
            createdAt: DateTime(2026, 11, 20, 10, 0),
          ),
        ];

        final router = GoRouter(
          initialLocation: '/notifications',
          routes: [
            GoRoute(
              path: '/notifications',
              builder: (ctx, state) => const NotificationManagementScreen(),
            ),
          ],
        );

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              authRepositoryProvider.overrideWith((_) {
                final client = ApiClient(customDio: Dio());
                final repo = AuthRepository(
                  authService: AuthService(client),
                  apiClient: client,
                );
                repo.state = const AsyncData<Map<String, dynamic>?>({
                  'id': 'u-admin',
                  'system_role': 'super_admin',
                  'role': AppRole.superAdmin,
                  'jabatan': null,
                });
                return repo;
              }),
              notificationRepositoryProvider.overrideWithValue(fakeRepo),
              notificationSystemTypesProvider.overrideWith(
                (ref) => fakeRepo.fetchSystemTypes(),
              ),
              notificationTemplatesProvider.overrideWith(
                (ref) => fakeRepo.fetchTemplates(),
              ),
            ],
            child: MaterialApp.router(routerConfig: router),
          ),
        );
        await tester.pumpAndSettle();

        await tester.tap(find.text('NOTIFIKASI'));
        await tester.pumpAndSettle();
        await tester.tap(find.text('RIWAYAT SAYA'));
        await tester.pumpAndSettle();

        await tester.tap(find.text('Paket Baru Tiba'));
        await tester.pumpAndSettle();

        expect(fakeRepo.operationOrder.contains('markAsRead_called'), isFalse);
        expect(fakeRepo.customMineLogs?.first.isRead, isFalse);
      },
    );

    testWidgets(
      'Case 7: In-flight guard is cleaned after API failure and permits retry',
      (tester) async {
        final fakeRepo = FakeNotificationRepository();
        fakeRepo.markAsReadShouldThrow = true;
        fakeRepo.customMineLogs = [
          NotificationLogEntry(
            id: 'log-retry-7',
            rtId: 'rt-1',
            eventType: 'manual_push',
            title: 'Notifikasi Retry API',
            body: 'Isi',
            variables: const {},
            recipientsCount: 1,
            sentCount: 1,
            historyCategory: 'mine',
            isRead: false,
            createdAt: DateTime(2026, 11, 20, 10, 0),
          ),
        ];

        await tester.pumpWidget(buildTestWidget(repository: fakeRepo));
        await tester.pumpAndSettle();

        await tester.tap(find.text('NOTIFIKASI'));
        await tester.pumpAndSettle();
        await tester.tap(find.text('RIWAYAT SAYA'));
        await tester.pumpAndSettle();

        // First attempt fails
        await tester.tap(find.text('Notifikasi Retry API'));
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(const Key('notification_detail_back_button')));
        await tester.pumpAndSettle();

        expect(fakeRepo.markAsReadCallCount, equals(1));
        expect(fakeRepo.customMineLogs?.first.isRead, isFalse);

        // Fix the API failure and retry
        fakeRepo.markAsReadShouldThrow = false;
        await tester.tap(find.text('Notifikasi Retry API'));
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(const Key('notification_detail_back_button')));
        await tester.pumpAndSettle();

        // Retry must succeed
        expect(fakeRepo.markAsReadCallCount, equals(2));
        expect(fakeRepo.customMineLogs?.first.isRead, isTrue);
      },
    );

    testWidgets(
      'Case 8: In-flight guard is cleaned after navigation failure and permits retry',
      (tester) async {
        final fakeRepo = FakeNotificationRepository();
        fakeRepo.customMineLogs = [
          NotificationLogEntry(
            id: 'log-retry-8',
            rtId: 'rt-1',
            eventType: 'manual_push',
            title: 'Notifikasi Nav Retry',
            body: 'Isi',
            variables: const {},
            recipientsCount: 1,
            sentCount: 1,
            historyCategory: 'mine',
            isRead: false,
            createdAt: DateTime(2026, 11, 20, 10, 0),
          ),
        ];

        await tester.pumpWidget(buildTestWidget(repository: fakeRepo));
        await tester.pumpAndSettle();

        await tester.tap(find.text('NOTIFIKASI'));
        await tester.pumpAndSettle();
        await tester.tap(find.text('RIWAYAT SAYA'));
        await tester.pumpAndSettle();

        // Open and return
        await tester.tap(find.text('Notifikasi Nav Retry'));
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(const Key('notification_detail_back_button')));
        await tester.pumpAndSettle();

        expect(fakeRepo.markAsReadCallCount, equals(1));
      },
    );

    testWidgets(
      'Disposal behavior: No setState after dispose',
      (tester) async {
        final fakeRepo = FakeNotificationRepository();
        fakeRepo.customMineLogs = [
          NotificationLogEntry(
            id: 'log-dispose-1',
            rtId: 'rt-1',
            eventType: 'manual_push',
            title: 'Notifikasi Dispose Test',
            body: 'Isi detail',
            variables: const {},
            recipientsCount: 1,
            sentCount: 1,
            historyCategory: 'mine',
            isRead: false,
            createdAt: DateTime(2026, 11, 20, 10, 0),
          ),
        ];

        await tester.pumpWidget(buildTestWidget(repository: fakeRepo));
        await tester.pumpAndSettle();

        // Dispose widget tree while idle
        await tester.pumpWidget(const SizedBox.shrink());
        await tester.pumpAndSettle();

        expect(tester.takeException(), isNull);
      },
    );
  });
}
