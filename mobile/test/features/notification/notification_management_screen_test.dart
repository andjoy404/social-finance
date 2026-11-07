import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/notification/data/models/notification_models.dart';
import 'package:social_finance/features/notification/data/notification_api_client.dart';
import 'package:social_finance/features/notification/data/notification_repository.dart';
import 'package:social_finance/features/notification/presentation/screens/notification_management_screen.dart';

final requestedHistoryCategories = <String>[];

class FakeNotificationRepository extends NotificationRepository {
  FakeNotificationRepository({this.additionalSystemTypes = 0})
    : super(apiClient: NotificationApiClient(dio: Dio()));

  final int additionalSystemTypes;

  @override
  Future<List<SystemNotificationType>> fetchSystemTypes() async {
    return [
      SystemNotificationType(
        eventType: 'iuran_bill_created',
        name: 'Penerbitan Tagihan Iuran',
        trigger: 'Diterbitkan saat bendahara men-generate tagihan iuran baru',
        recipientsRule: 'Warga kepala keluarga',
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
    return NotificationLogsResponse(
      data: [
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
      ],
      page: 1,
      perPage: 20,
      total: 1,
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
}

void main() {
  Widget buildTestWidget({
    Map<String, dynamic>? user,
    int additionalSystemTypes = 0,
  }) {
    final fakeRepo = FakeNotificationRepository(
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
      expect(find.text('SISTEM'), findsOneWidget);
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
      expect(tester.getRect(find.text('SISTEM').last).center.dx, closeTo(configRect.left + configRect.width / 6, 2));
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
      expect(find.text('SISTEM'), findsOneWidget);
      expect(find.text('KUSTOM'), findsOneWidget);
      expect(find.text('PENJADWALAN'), findsOneWidget);
      expect(tester.takeException(), isNull);

      await tester.drag(find.byType(ListView).first, const Offset(0, -450));
      await tester.pumpAndSettle();

      expect(tester.getTopLeft(configTabs).dy, topBefore);
      expect(find.text('SISTEM'), findsOneWidget);
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
      await tester.tap(find.text('SISTEM'));
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

  testWidgets('opens push notification preview banner modal', (tester) async {
    await tester.pumpWidget(buildTestWidget());
    await tester.pumpAndSettle();
    await tester.tap(find.text('TEMPLATE'));
    await tester.pumpAndSettle();

    final previewButtons = find.text('Preview Banner');
    expect(previewButtons, findsAtLeastNWidgets(1));

    await tester.tap(previewButtons.first);
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
}
