import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/features/notification/data/models/notification_models.dart';
import 'package:social_finance/features/notification/data/notification_api_client.dart';
import 'package:social_finance/features/notification/data/notification_repository.dart';
import 'package:social_finance/features/notification/presentation/screens/security_waste_truck_history_screen.dart';

class _FakeNotificationRepository extends NotificationRepository {
  _FakeNotificationRepository()
    : super(apiClient: NotificationApiClient(dio: Dio()));

  @override
  Future<WasteTruckDispatchHistoryResponse> fetchWasteTruckHistory({
    int page = 1,
    int perPage = 20,
  }) async {
    return WasteTruckDispatchHistoryResponse(
      data: [
        WasteTruckDispatchEntry(
          id: 'dispatch-1',
          eventType: 'waste_truck_arrival',
          title: 'Truk Sampah Tiba',
          body: 'Truk telah sampai di lingkungan.',
          imageUrl: 'https://example.test/truck.jpg',
          status: 'FAILED',
          failureReason: 'Provider menolak satu atau lebih tujuan.',
          createdAt: DateTime.utc(2026, 10, 29, 1),
        ),
      ],
      page: page,
      total: 1,
    );
  }
}

void main() {
  testWidgets('history opens read-only details and zoomable photo viewer', (
    tester,
  ) async {
    final repository = _FakeNotificationRepository();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          notificationRepositoryProvider.overrideWithValue(repository),
        ],
        child: const MaterialApp(home: SecurityWasteTruckHistoryScreen()),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('Ketuk untuk melihat detail'));
    await tester.pumpAndSettle();
    expect(find.text('Event: waste_truck_arrival'), findsOneWidget);
    expect(find.text('Truk telah sampai di lingkungan.'), findsOneWidget);
    expect(find.text('Informasi kegagalan'), findsOneWidget);
    expect(
      find.text(
        'Jumlah penerima dan rincian tiap perangkat tidak tersedia pada data dispatch.',
      ),
      findsOneWidget,
    );

    await tester.ensureVisible(
      find.text('Foto gagal dimuat atau URL tidak valid.'),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('Foto gagal dimuat atau URL tidak valid.'));
    await tester.pumpAndSettle();
    expect(find.text('Foto Pengiriman'), findsOneWidget);
    expect(find.byType(InteractiveViewer), findsOneWidget);
    expect(find.text('Foto tidak dapat dimuat.'), findsOneWidget);

    await tester.pageBack();
    await tester.pumpAndSettle();
    expect(find.text('Detail Pengiriman'), findsOneWidget);
    await tester.pageBack();
    await tester.pumpAndSettle();
    expect(find.text('Ketuk untuk melihat detail'), findsOneWidget);
  });
}
