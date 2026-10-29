import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/features/notification/data/models/notification_models.dart';
import 'package:social_finance/features/notification/data/notification_api_client.dart';
import 'package:social_finance/features/notification/data/notification_repository.dart';
import 'package:social_finance/features/notification/presentation/screens/notification_management_screen.dart';

class FakeNotificationRepository extends NotificationRepository {
  FakeNotificationRepository()
      : super(apiClient: NotificationApiClient(dio: Dio()));

  @override
  Future<List<SystemNotificationType>> fetchSystemTypes() async {
    return const [
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
  }) async {
    return NotificationLogsResponse(
      data: [
        NotificationLogEntry(
          id: 'log-1',
          rtId: 'rt-1',
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
}

void main() {
  Widget buildTestWidget() {
    final fakeRepo = FakeNotificationRepository();
    return ProviderScope(
      overrides: [
        notificationRepositoryProvider.overrideWithValue(fakeRepo),
        notificationSystemTypesProvider.overrideWith((ref) => fakeRepo.fetchSystemTypes()),
        notificationTemplatesProvider.overrideWith((ref) => fakeRepo.fetchTemplates()),
      ],
      child: const MaterialApp(
        home: NotificationManagementScreen(),
      ),
    );
  }

  testWidgets('renders NotificationManagementScreen with 3 tabs and system items',
      (tester) async {
    await tester.pumpWidget(buildTestWidget());
    await tester.pumpAndSettle();

    expect(find.text('KELOLA NOTIFIKASI'), findsOneWidget);
    expect(find.text('Sistem'), findsOneWidget);
    expect(find.text('Template'), findsOneWidget);
    expect(find.text('Riwayat'), findsOneWidget);

    expect(find.text('Penerbitan Tagihan Iuran'), findsOneWidget);
    expect(find.text('Kedatangan Mobil Sampah'), findsOneWidget);
    expect(find.text('FCM PUSH'), findsNWidgets(2));
  });

  testWidgets('switches to Template tab and shows custom templates',
      (tester) async {
    await tester.pumpWidget(buildTestWidget());
    await tester.pumpAndSettle();

    await tester.tap(find.text('Template'));
    await tester.pumpAndSettle();

    expect(find.text('Tagihan Khusus Warga'), findsOneWidget);
    expect(find.text('Halo Warga!'), findsOneWidget);
    expect(find.byType(Switch), findsOneWidget);
  });

  testWidgets('switches to Riwayat tab and shows notification logs',
      (tester) async {
    await tester.pumpWidget(buildTestWidget());
    await tester.pumpAndSettle();

    await tester.tap(find.text('Riwayat'));
    await tester.pumpAndSettle();

    expect(find.text('Mohon lunasi tagihan nomor 101'), findsOneWidget);
    expect(find.text('TERKIRIM'), findsOneWidget);
    expect(find.text('Penerima: 4 perangkat'), findsOneWidget);
  });

  testWidgets('opens push notification preview banner modal',
      (tester) async {
    await tester.pumpWidget(buildTestWidget());
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
}
