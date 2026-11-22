import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:image_picker/image_picker.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/device_id_service.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/rw/data/rw_api_client.dart';
import 'package:social_finance/features/rw/data/rw_models.dart';
import 'package:social_finance/features/rw/data/rw_providers.dart';
import 'package:social_finance/features/rw/presentation/screens/rw_keuangan_screen.dart';
import 'package:social_finance/features/rw/presentation/screens/rw_notification_screen.dart';

class FakeAuthRepo extends AuthRepository {
  FakeAuthRepo(Map<String, dynamic>? user)
      : super(
          authService: AuthService(ApiClient(baseUrl: 'http://test.local'), _FakeDeviceId()),
          apiClient: ApiClient(baseUrl: 'http://test.local'),
        ) {
    state = AsyncData(user);
  }
}

class _FakeDeviceId extends DeviceIdService {
  _FakeDeviceId();
  @override
  Future<String> getDeviceId() async => 'test-device';
  Future<String> getOrCreateDeviceId() async => 'test-device';
}


class FakeRwApiClient extends RwApiClient {
  FakeRwApiClient() : super(ApiClient(baseUrl: 'http://test.local'));
  String? lastRwId;
  int notificationsCalls = 0;
  int templatesCalls = 0;
  int manualPushCalls = 0;
  int createTemplateCalls = 0;
  int updateTemplateCalls = 0;
  int toggleTemplateCalls = 0;
  int deleteTemplateCalls = 0;
  int uploadImageCalls = 0;
  String? lastUploadedEventType;
  String? lastUploadedFilename;
  String? lastCreatedImageUrl;
  String? lastUpdatedImageUrl;
  bool updateImageUrlPassed = false;
  Object? uploadImageError;
  Future<List<RwNotificationLog>> logsToReturn = Future.value(const []);
  Future<List<RwNotificationTemplate>> templatesToReturn = Future.value(const []);
  Object? notificationsError;
  Object? templatesError;

  RwNotificationLogsPage _page(List<RwNotificationLog> items) => RwNotificationLogsPage(
        items: items, page: 1, pageSize: 20, total: items.length, totalPages: 1);
  RwNotificationTemplate _tmpl(String id, {bool active = true}) => RwNotificationTemplate(
        id: id, rwId: 'rw-a', name: 'T$id', eventType: 'manual_push', channel: 'push',
        title: 'Title $id', body: 'Body $id', isActive: active);

  @override
  Future<RwDashboardSummary> getDashboard(String rwId, {String? period}) async =>
      const RwDashboardSummary(totalRts: 1, totalLocations: 0, totalBalance: '0', totalIncome: '0', totalExpense: '0', transactionCount: 0);
  @override
  Future<RwKasReport> getKas(String rwId, {String? period}) async =>
      const RwKasReport(totalIncome: '0', totalExpense: '0', netBalance: '0', transactionCount: 0);
  @override
  Future<RwKasReportDetail> getKasReportDetail(String rwId, {String? period, String? startDate, String? endDate}) async =>
      const RwKasReportDetail(period: '', startDate: '', endDate: '', openingBalance: '0.00', totalIncome: '0', totalExpense: '0', netBalance: '0', transactionCount: 0);
  @override
  Future<RwTransactionsResponse> getTransactions({required String rwId, String? period, String? startDate, String? endDate, int page = 1, int pageSize = 20, String? type, String? categoryId}) async =>
      const RwTransactionsResponse(data: [], pagination: RwPaginationInfo(page: 1, pageSize: 20, total: 0, totalPages: 1));
  @override
  Future<List<RwFundLocation>> getLocations(String rwId, {String? category, bool? isActive}) async => const [];
  @override
  Future<RwNotificationLogsPage> getNotifications(String rwId, {int page = 1, int pageSize = 20, String? eventType}) async {
    lastRwId = rwId; notificationsCalls++;
    if (notificationsError != null) throw notificationsError!;
    return _page(await logsToReturn);
  }
  @override
  Future<List<RwNotificationTemplate>> listNotificationTemplates(String rwId, {String? eventType}) async {
    templatesCalls++;
    if (templatesError != null) throw templatesError!;
    return templatesToReturn;
  }
  @override
  Future<int> sendManualPush(String rwId, {required String title, required String body}) async { manualPushCalls++; return 3; }
  @override
  Future<RwNotificationTemplate> createNotificationTemplate(String rwId, {required String name, required String eventType, required String title, required String body, String? imageUrl, String channel = 'push'}) async {
    createTemplateCalls++;
    lastCreatedImageUrl = imageUrl;
    return _tmpl('new');
  }
  @override
  Future<RwNotificationTemplate> updateNotificationTemplate(String rwId, String templateId, {String? name, String? title, String? body, String? imageUrl, bool? isActive}) async {
    updateTemplateCalls++;
    updateImageUrlPassed = imageUrl != null;
    lastUpdatedImageUrl = imageUrl;
    return _tmpl(templateId);
  }
  @override
  Future<String> uploadNotificationImage(String rwId, {required String eventType, required List<int> bytes, required String filename}) async {
    uploadImageCalls++;
    lastUploadedEventType = eventType;
    lastUploadedFilename = filename;
    if (uploadImageError != null) throw uploadImageError!;
    return '/api/v1/notifications/images/rw-manual/$rwId/$filename';
  }
  @override
  Future<void> deleteNotificationTemplate(String rwId, String templateId) async {
    deleteTemplateCalls++;
  }
  @override
  Future<RwNotificationTemplate> toggleNotificationTemplate(String rwId, String templateId) async { toggleTemplateCalls++; return _tmpl(templateId, active: false); }
  int previewTemplateCalls = 0;
  int sendTemplateCalls = 0;
  @override
  Future<Map<String, dynamic>> previewNotificationTemplate(
    String rwId,
    String templateId, {
    Map<String, String>? variables,
  }) async {
    previewTemplateCalls++;
    return {
      'rendered_title': 'R-Title $templateId',
      'rendered_body': 'R-Body $templateId',
      'placeholders': ['nama_warga'],
      'sample_data': {'nama_warga': 'Budi Santoso'},
    };
  }
  @override
  Future<int> sendTemplateNotification(
    String rwId,
    String templateId, {
    String? target,
    Map<String, String>? variables,
    String? title,
    String? body,
  }) async {
    sendTemplateCalls++;
    return 5;
  }
}

Map<String, dynamic> userFor({required bool superAdmin, String? role, String? jabatan, String? rwId, String? source, bool includeMembership = true}) {
  final m = <String, dynamic>{
    'id': 'u1', 'email': 'u@test.com', 'name': 'User',
    'role': superAdmin ? AppRole.superAdmin : (role ?? 'warga'),
    if (superAdmin) 'system_role': 'super_admin',
    if (includeMembership && rwId != null)
      'rw_memberships': [
        {'rw_id': rwId, 'rw_number': 2, 'rw_name': 'RW 002', 'jabatan': jabatan, 'source': source}
      ],
  };
  if (!includeMembership) m['rw_memberships'] = <Map<String, dynamic>>[];
  return m;
}

Widget wrap(Widget child, {required Map<String, dynamic>? user, required FakeRwApiClient api}) {
  return ProviderScope(
    overrides: [
      authRepositoryProvider.overrideWith((_) => FakeAuthRepo(user)),
      rwApiClientProvider.overrideWithValue(api),
    ],
    child: MaterialApp(home: Scaffold(body: child)),
  );
}

void main() {
  final logA = RwNotificationLog(id: 'log-a', rwId: 'rw-a', eventType: 'manual_push', title: 'A title', body: 'A body', createdAt: DateTime(2026, 1, 1));
  final logB = RwNotificationLog(id: 'log-b', rwId: 'rw-b', eventType: 'manual_push', title: 'B title', body: 'B body', createdAt: DateTime(2026, 1, 2));
  final tmpl = RwNotificationTemplate(id: 't1', rwId: 'rw-a', name: 'Template 1', eventType: 'manual_push', channel: 'push', title: 'T', body: 'B', isActive: true);

  group('RwKeuanganScreen notification tab visibility', () {
    testWidgets('rt_inferred hides NOTIFIKASI tab', (tester) async {
      final api = FakeRwApiClient();
      await tester.pumpWidget(wrap(const RwKeuanganScreen(rwId: 'rw-a'),
          user: userFor(superAdmin: false, role: 'pengurus', jabatan: 'ketua', rwId: 'rw-a', source: 'rt_inferred'), api: api));
      await tester.pumpAndSettle();
      expect(find.text('NOTIFIKASI'), findsNothing);
      expect(find.text('BERANDA'), findsOneWidget);
    });
    testWidgets('no RW membership hides NOTIFIKASI tab', (tester) async {
      final api = FakeRwApiClient();
      await tester.pumpWidget(wrap(const RwKeuanganScreen(rwId: 'rw-a'),
          user: userFor(superAdmin: false, role: 'warga', rwId: 'rw-a', includeMembership: false), api: api));
      await tester.pumpAndSettle();
      expect(find.text('NOTIFIKASI'), findsNothing);
    });
    testWidgets('explicit non-ketua/bendahara shows NOTIFIKASI tab (read)', (tester) async {
      final api = FakeRwApiClient()..logsToReturn = Future.value([logA])..templatesToReturn = Future.value([tmpl]);
      await tester.pumpWidget(wrap(const RwKeuanganScreen(rwId: 'rw-a'),
          user: userFor(superAdmin: false, role: 'pengurus', jabatan: 'sekretaris', rwId: 'rw-a', source: 'explicit'), api: api));
      await tester.pumpAndSettle();
      expect(find.text('NOTIFIKASI'), findsOneWidget);
      await tester.tap(find.text('NOTIFIKASI'));
      await tester.pumpAndSettle();
      expect(find.text('Riwayat Notifikasi Sistem'), findsOneWidget);
      expect(find.byKey(const Key('rwNotifManualPushBtn')), findsNothing);
      expect(find.byKey(const Key('rwNotifCreateTemplateBtn')), findsNothing);
    });
    testWidgets('ketua explicit shows write controls', (tester) async {
      final api = FakeRwApiClient()..logsToReturn = Future.value([logA])..templatesToReturn = Future.value([tmpl]);
      await tester.pumpWidget(wrap(const RwKeuanganScreen(rwId: 'rw-a'),
          user: userFor(superAdmin: false, role: 'pengurus', jabatan: 'ketua', rwId: 'rw-a', source: 'explicit'), api: api));
      await tester.pumpAndSettle();
      await tester.tap(find.text('NOTIFIKASI'));
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('rwNotifManualPushBtn')), findsOneWidget);
      expect(find.byKey(const Key('rwNotifCreateTemplateBtn')), findsOneWidget);
      expect(find.byKey(const Key('rwNotifTplToggle_t1')), findsOneWidget);
      expect(find.byKey(const Key('rwNotifTplDelete_t1')), findsOneWidget);
    });
    testWidgets('bendahara explicit shows write controls', (tester) async {
      final api = FakeRwApiClient()..logsToReturn = Future.value([logA])..templatesToReturn = Future.value([tmpl]);
      await tester.pumpWidget(wrap(const RwKeuanganScreen(rwId: 'rw-a'),
          user: userFor(superAdmin: false, role: 'bendahara', jabatan: 'bendahara', rwId: 'rw-a', source: 'explicit'), api: api));
      await tester.pumpAndSettle();
      await tester.tap(find.text('NOTIFIKASI'));
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('rwNotifManualPushBtn')), findsOneWidget);
    });
    testWidgets('super admin shows notification and write controls', (tester) async {
      final api = FakeRwApiClient()..logsToReturn = Future.value([logA])..templatesToReturn = Future.value([tmpl]);
      await tester.pumpWidget(wrap(const RwKeuanganScreen(rwId: 'rw-a'),
          user: userFor(superAdmin: true, rwId: 'rw-a', source: 'explicit'), api: api));
      await tester.pumpAndSettle();
      expect(find.text('NOTIFIKASI'), findsOneWidget);
      await tester.tap(find.text('NOTIFIKASI'));
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('rwNotifManualPushBtn')), findsOneWidget);
    });
  });

  group('RwNotificationScreen behavior/state', () {
    testWidgets('loads logs and templates for active rwId', (tester) async {
      final api = FakeRwApiClient()..logsToReturn = Future.value([logA])..templatesToReturn = Future.value([tmpl]);
      await tester.pumpWidget(wrap(const RwNotificationScreen(rwId: 'rw-a'),
          user: userFor(superAdmin: false, role: 'pengurus', jabatan: 'sekretaris', rwId: 'rw-a', source: 'explicit'), api: api));
      await tester.pumpAndSettle();
      expect(api.lastRwId, 'rw-a');
      expect(find.text('Template 1'), findsOneWidget);
      await tester.tap(find.text('Riwayat Notifikasi Sistem'));
      await tester.pumpAndSettle();
      expect(find.text('A title'), findsOneWidget);
    });

    testWidgets('switching rwId requests new tenant and does not show old data', (tester) async {
      final api = FakeRwApiClient()..logsToReturn = Future.value([logA])..templatesToReturn = Future.value(const []);
      await tester.pumpWidget(wrap(const RwNotificationScreen(rwId: 'rw-a'),
          user: userFor(superAdmin: false, role: 'pengurus', jabatan: 'sekretaris', rwId: 'rw-a', source: 'explicit'), api: api));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Riwayat Notifikasi Sistem'));
      await tester.pumpAndSettle();
      expect(find.text('A title'), findsOneWidget);

      api.logsToReturn = Future.value([logB]);
      await tester.pumpWidget(wrap(const RwNotificationScreen(rwId: 'rw-b'),
          user: userFor(superAdmin: false, role: 'pengurus', jabatan: 'sekretaris', rwId: 'rw-b', source: 'explicit'), api: api));
      await tester.pumpAndSettle();
      expect(api.lastRwId, 'rw-b');
      expect(find.text('B title'), findsOneWidget);
      expect(find.text('A title'), findsNothing);
    });

    testWidgets('403 error shows friendly message and no stale data on fresh scope', (tester) async {
      final api = FakeRwApiClient()..notificationsError = Exception('403 forbidden')..templatesToReturn = Future.value(const []);
      await tester.pumpWidget(wrap(const RwNotificationScreen(rwId: 'rw-a'),
          user: userFor(superAdmin: false, role: 'pengurus', jabatan: 'sekretaris', rwId: 'rw-a', source: 'explicit'), api: api));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Riwayat Notifikasi Sistem'));
      await tester.pumpAndSettle();
      expect(find.textContaining('izin'), findsWidgets);
      expect(find.text('A title'), findsNothing);
    });

    testWidgets('logout/login different account does not reuse previous scope data', (tester) async {
      final api = FakeRwApiClient()..logsToReturn = Future.value([logA])..templatesToReturn = Future.value(const []);
      final auth = FakeAuthRepo(userFor(superAdmin: false, role: 'pengurus', jabatan: 'sekretaris', rwId: 'rw-a', source: 'explicit'));
      await tester.pumpWidget(ProviderScope(
        overrides: [authRepositoryProvider.overrideWith((_) => auth), rwApiClientProvider.overrideWithValue(api)],
        child: const MaterialApp(home: RwNotificationScreen(rwId: 'rw-a')),
      ));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Riwayat Notifikasi Sistem'));
      await tester.pumpAndSettle();
      expect(find.text('A title'), findsOneWidget);

      auth.state = const AsyncData(null);
      await tester.pumpWidget(ProviderScope(
        overrides: [authRepositoryProvider.overrideWith((_) => auth), rwApiClientProvider.overrideWithValue(api)],
        child: const MaterialApp(home: RwNotificationScreen(rwId: 'rw-a')),
      ));
      await tester.pumpAndSettle();

      api.logsToReturn = Future.value([logB]);
      auth.state = AsyncData(userFor(superAdmin: false, role: 'pengurus', jabatan: 'sekretaris', rwId: 'rw-a', source: 'explicit')..['id'] = 'u2');
      await tester.pumpWidget(ProviderScope(
        overrides: [authRepositoryProvider.overrideWith((_) => auth), rwApiClientProvider.overrideWithValue(api)],
        child: const MaterialApp(home: RwNotificationScreen(rwId: 'rw-a')),
      ));
      await tester.pumpAndSettle();
      expect(find.text('A title'), findsNothing);
    });

    testWidgets('read-only cannot trigger write controls', (tester) async {
      final api = FakeRwApiClient()..logsToReturn = Future.value([logA])..templatesToReturn = Future.value([tmpl]);
      await tester.pumpWidget(wrap(const RwNotificationScreen(rwId: 'rw-a'),
          user: userFor(superAdmin: false, role: 'pengurus', jabatan: 'sekretaris', rwId: 'rw-a', source: 'explicit'), api: api));
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('rwNotifManualPushBtn')), findsNothing);
      expect(find.byKey(const Key('rwNotifTplDelete_t1')), findsNothing);
      expect(find.byKey(const Key('rwNotifTplSend_t1')), findsNothing);
      expect(find.byKey(const Key('rwNotifTplPreview_t1')), findsOneWidget);
      expect(api.manualPushCalls, 0);
      expect(api.deleteTemplateCalls, 0);
    });

    testWidgets('preview template opens preview dialog with simulated push', (tester) async {
      tester.view.physicalSize = const Size(800, 1200);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);

      final api = FakeRwApiClient()..logsToReturn = Future.value([logA])..templatesToReturn = Future.value([tmpl]);
      await tester.pumpWidget(wrap(const RwNotificationScreen(rwId: 'rw-a'),
          user: userFor(superAdmin: false, role: 'pengurus', jabatan: 'sekretaris', rwId: 'rw-a', source: 'explicit'), api: api));
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('rwNotifTplPreview_t1')), findsOneWidget);

      await tester.tap(find.byKey(const Key('rwNotifTplPreview_t1')));
      await tester.pumpAndSettle();
      expect(api.previewTemplateCalls, 1);
      expect(find.text('Preview: Template 1'), findsOneWidget);
      expect(find.text('R-Title t1'), findsOneWidget);
      expect(find.text('R-Body t1'), findsOneWidget);
    });

    testWidgets('write user (ketua) sees Send button and can send template notification', (tester) async {
      tester.view.physicalSize = const Size(800, 1200);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);

      final api = FakeRwApiClient()..logsToReturn = Future.value([logA])..templatesToReturn = Future.value([tmpl]);
      await tester.pumpWidget(wrap(const RwNotificationScreen(rwId: 'rw-a'),
          user: userFor(superAdmin: false, role: 'pengurus', jabatan: 'ketua', rwId: 'rw-a', source: 'explicit'), api: api));
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('rwNotifTplPreview_t1')), findsOneWidget);
      expect(find.byKey(const Key('rwNotifTplSend_t1')), findsOneWidget);

      await tester.tap(find.byKey(const Key('rwNotifTplSend_t1')));
      await tester.pumpAndSettle();
      expect(find.text('Kirim: Template 1'), findsOneWidget);
      expect(find.text('Kirim Sekarang'), findsOneWidget);

      await tester.tap(find.text('Kirim Sekarang'));
      await tester.pumpAndSettle();
      expect(api.sendTemplateCalls, 1);
    });

    testWidgets('tapping notification log item opens simulated preview modal with details', (tester) async {
      tester.view.physicalSize = const Size(800, 1200);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);

      final logWithMeta = RwNotificationLog(
        id: 'log-rich',
        rwId: 'rw-a',
        eventType: 'manual_push',
        title: 'Pengumuman Kerja Bakti',
        body: 'Kerja bakti hari Minggu di https://example.com/info',
        status: 'SENT',
        source: 'rw',
        url: 'https://example.com/info',
        recipientsCount: 50,
        sentCount: 48,
        failedCount: 2,
        deliveryCountsAvailable: true,
        createdAt: DateTime(2026, 3, 15, 10, 30),
      );

      final api = FakeRwApiClient()
        ..logsToReturn = Future.value([logWithMeta])
        ..templatesToReturn = Future.value(const []);

      await tester.pumpWidget(wrap(
        const RwNotificationScreen(rwId: 'rw-a'),
        user: userFor(superAdmin: false, role: 'pengurus', jabatan: 'ketua', rwId: 'rw-a', source: 'explicit'),
        api: api,
      ));
      await tester.pumpAndSettle();

      await tester.tap(find.text('Riwayat Notifikasi Sistem'));
      await tester.pumpAndSettle();

      // Check item in list with aligned delivery status
      expect(find.text('Pengumuman Kerja Bakti'), findsOneWidget);
      expect(find.text('TERKIRIM'), findsOneWidget);
      expect(find.text('RW'), findsOneWidget);

      // Tap the item to open preview modal
      await tester.tap(find.byKey(const Key('rwNotifLog_log-rich')));
      await tester.pumpAndSettle();

      // Verify simulated push modal elements
      expect(find.text('Preview Notifikasi'), findsOneWidget);
      expect(find.text('SOCIAL FINANCE URENA • RW'), findsOneWidget);
      expect(find.text('Baru saja'), findsOneWidget);
      expect(find.text('Detail Informasi'), findsOneWidget);
      expect(find.text('Waktu / Jadwal'), findsOneWidget);
      expect(find.text('Penerima'), findsOneWidget);
      expect(find.text('48 / 50 penerima terkirim (2 gagal)'), findsOneWidget);
      expect(find.text('https://example.com/info'), findsWidgets);

      // Tap Tutup to close modal cleanly
      await tester.tap(find.text('Tutup'));
      await tester.pumpAndSettle();
      expect(find.text('Preview Notifikasi'), findsNothing);
    });

    testWidgets('status badges display mapped Indonesian labels for various statuses', (tester) async {
      final logs = [
        RwNotificationLog(id: 'l1', eventType: 'p', title: 'T1', status: 'SENT', createdAt: DateTime(2026, 1, 1)),
        RwNotificationLog(id: 'l2', eventType: 'p', title: 'T2', status: 'FAILED', createdAt: DateTime(2026, 1, 1)),
        RwNotificationLog(id: 'l3', eventType: 'p', title: 'T3', status: 'PARTIAL', createdAt: DateTime(2026, 1, 1)),
        RwNotificationLog(id: 'l4', eventType: 'p', title: 'T4', status: 'NO_DEVICE', createdAt: DateTime(2026, 1, 1)),
        RwNotificationLog(id: 'l5', eventType: 'p', title: 'T5', status: 'PENDING', createdAt: DateTime(2026, 1, 1)),
      ];

      final api = FakeRwApiClient()
        ..logsToReturn = Future.value(logs)
        ..templatesToReturn = Future.value(const []);

      await tester.pumpWidget(wrap(
        const RwNotificationScreen(rwId: 'rw-a'),
        user: userFor(superAdmin: false, role: 'pengurus', jabatan: 'sekretaris', rwId: 'rw-a', source: 'explicit'),
        api: api,
      ));
      await tester.pumpAndSettle();

      await tester.tap(find.text('Riwayat Notifikasi Sistem'));
      await tester.pumpAndSettle();

      expect(find.text('TERKIRIM'), findsOneWidget);
      expect(find.text('GAGAL'), findsOneWidget);
      expect(find.text('SEBAGIAN'), findsOneWidget);
      expect(find.text('TANPA PERANGKAT'), findsOneWidget);
      expect(find.text('TERTUNDA'), findsOneWidget);
    });

    testWidgets('RBAC: user with effective_permissions requires rw.notification.write', (tester) async {
      final api = FakeRwApiClient()..logsToReturn = Future.value([logA])..templatesToReturn = Future.value([tmpl]);

      // User has explicit ketua membership but effective_permissions only contains rw.notification.read
      final readOnlyUser = userFor(superAdmin: false, role: 'pengurus', jabatan: 'ketua', rwId: 'rw-a', source: 'explicit')
        ..['effective_permissions'] = ['rw.notification.read'];

      final auth = FakeAuthRepo(readOnlyUser);
      await tester.pumpWidget(ProviderScope(
        overrides: [authRepositoryProvider.overrideWith((_) => auth), rwApiClientProvider.overrideWithValue(api)],
        child: const MaterialApp(home: Scaffold(body: RwNotificationScreen(rwId: 'rw-a'))),
      ));
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('rwNotifManualPushBtn')), findsNothing);
      expect(find.byKey(const Key('rwNotifTplDelete_t1')), findsNothing);

      // User with explicit rw.notification.write gets controls
      final writeUser = userFor(superAdmin: false, role: 'pengurus', jabatan: 'ketua', rwId: 'rw-a', source: 'explicit')
        ..['effective_permissions'] = ['rw.notification.read', 'rw.notification.write'];

      auth.state = AsyncData(writeUser);
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('rwNotifManualPushBtn')), findsOneWidget);
    });

    testWidgets('editing template opens form dialog with prefilled fields', (tester) async {
      final api = FakeRwApiClient()..logsToReturn = Future.value([logA])..templatesToReturn = Future.value([tmpl]);
      await tester.pumpWidget(wrap(const RwNotificationScreen(rwId: 'rw-a'),
          user: userFor(superAdmin: false, role: 'pengurus', jabatan: 'ketua', rwId: 'rw-a', source: 'explicit'), api: api));
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('rwNotifTplEdit_t1')), findsOneWidget);
      await tester.tap(find.byKey(const Key('rwNotifTplEdit_t1')));
      await tester.pumpAndSettle();

      expect(find.text('Ubah Template'), findsOneWidget);
      expect(find.byKey(const Key('rwTemplateNameField')), findsOneWidget);
      expect(find.byKey(const Key('rwTemplateTitleField')), findsOneWidget);
      expect(find.byKey(const Key('rwTemplateBodyField')), findsOneWidget);
      expect(find.byKey(const Key('rwTemplatePickGalleryBtn')), findsOneWidget);
      expect(find.byKey(const Key('rwTemplatePickCameraBtn')), findsOneWidget);

      await tester.tap(find.text('Batal'));
      await tester.pumpAndSettle();
      expect(find.text('Ubah Template'), findsNothing);
    });

    testWidgets('deleting template shows confirmation dialog before deletion', (tester) async {
      final api = FakeRwApiClient()..logsToReturn = Future.value([logA])..templatesToReturn = Future.value([tmpl]);
      await tester.pumpWidget(wrap(const RwNotificationScreen(rwId: 'rw-a'),
          user: userFor(superAdmin: false, role: 'pengurus', jabatan: 'ketua', rwId: 'rw-a', source: 'explicit'), api: api));
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('rwNotifTplDelete_t1')), findsOneWidget);
      await tester.tap(find.byKey(const Key('rwNotifTplDelete_t1')));
      await tester.pumpAndSettle();

      expect(find.text('Hapus Template?'), findsOneWidget);
      expect(find.textContaining('Apakah Anda yakin ingin menghapus template "Template 1"?'), findsOneWidget);

      await tester.tap(find.text('Batal'));
      await tester.pumpAndSettle();
      expect(api.deleteTemplateCalls, 0);
    });
  });

  group('RW Notification Template Finalization (Tab Style, RBAC, CRUD, Image Upload, Small Screen)', () {
    // Minimal 1x1 transparent PNG bytes for Image.memory preview tests
    final validPngBytes = base64Decode(
      'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==',
    );

    final tmplWithImage = RwNotificationTemplate(
      id: 't-img',
      rwId: 'rw-a',
      name: 'Template Bergambar',
      eventType: 'manual_push',
      channel: 'push',
      title: 'Judul Lama',
      body: 'Isi Lama',
      imageUrl: '/api/v1/notifications/images/rw-manual/rw-a/existing.png',
      isActive: true,
    );

    testWidgets('uses Notifikasi RT TabBar style (no SegmentedButton) in Light and Dark themes', (tester) async {
      for (final brightness in [Brightness.light, Brightness.dark]) {
        final api = FakeRwApiClient()
          ..logsToReturn = Future.value([logA])
          ..templatesToReturn = Future.value([tmpl]);

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              authRepositoryProvider.overrideWith(
                (_) => FakeAuthRepo(userFor(
                  superAdmin: false,
                  role: 'pengurus',
                  jabatan: 'ketua',
                  rwId: 'rw-a',
                  source: 'explicit',
                )),
              ),
              rwApiClientProvider.overrideWithValue(api),
            ],
            child: MaterialApp(
              theme: ThemeData(brightness: brightness),
              home: Scaffold(body: RwNotificationScreen(key: ValueKey(brightness), rwId: 'rw-a')),
            ),
          ),
        );
        await tester.pumpAndSettle();

        expect(find.byType(SegmentedButton<int>), findsNothing);
        final tabBarFinder = find.byKey(const Key('rwNotifSubTabBar'));
        expect(tabBarFinder, findsOneWidget);
        final tabBar = tester.widget<TabBar>(tabBarFinder);
        expect(tabBar.labelColor, AppColors.accent);
        expect(tabBar.indicatorColor, AppColors.accent);
        expect(tabBar.indicatorWeight, 3);
        expect(tabBar.tabAlignment, TabAlignment.fill);
        expect(find.descendant(of: tabBarFinder, matching: find.text('Template')), findsOneWidget);
        expect(find.descendant(of: tabBarFinder, matching: find.text('Riwayat Notifikasi Sistem')), findsOneWidget);

        // Switching to Riwayat Notifikasi Sistem still works
        await tester.tap(find.descendant(of: tabBarFinder, matching: find.text('Riwayat Notifikasi Sistem')));
        await tester.pumpAndSettle();
        expect(find.text('A title'), findsOneWidget);
      }
    });

    testWidgets('RBAC: permission-based READ vs WRITE aliases (notification.read vs notification.manage)', (tester) async {
      final api = FakeRwApiClient()
        ..logsToReturn = Future.value([logA])
        ..templatesToReturn = Future.value([tmpl]);

      // READ-only via notification.read alias -> can view & preview, NO mutation buttons
      final readUser = userFor(
        superAdmin: false,
        role: 'pengurus',
        jabatan: 'sekretaris',
        rwId: 'rw-a',
        source: 'explicit',
      )..['effective_permissions'] = ['notification.read'];

      final auth = FakeAuthRepo(readUser);
      await tester.pumpWidget(ProviderScope(
        overrides: [authRepositoryProvider.overrideWith((_) => auth), rwApiClientProvider.overrideWithValue(api)],
        child: const MaterialApp(home: Scaffold(body: RwNotificationScreen(rwId: 'rw-a'))),
      ));
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('rwNotifTplPreview_t1')), findsOneWidget);
      expect(find.byKey(const Key('rwNotifCreateTemplateBtn')), findsNothing);
      expect(find.byKey(const Key('rwNotifTplEdit_t1')), findsNothing);
      expect(find.byKey(const Key('rwNotifTplDelete_t1')), findsNothing);
      expect(find.byKey(const Key('rwNotifTplToggle_t1')), findsNothing);

      // WRITE via notification.manage alias (even for non-ketua/bendahara jabatan!) -> full CRUD visible
      final manageUser = userFor(
        superAdmin: false,
        role: 'pengurus',
        jabatan: 'sekretaris',
        rwId: 'rw-a',
        source: 'explicit',
      )..['effective_permissions'] = ['notification.read', 'notification.manage'];

      auth.state = AsyncData(manageUser);
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('rwNotifCreateTemplateBtn')), findsOneWidget);
      expect(find.byKey(const Key('rwNotifTplEdit_t1')), findsOneWidget);
      expect(find.byKey(const Key('rwNotifTplDelete_t1')), findsOneWidget);
      expect(find.byKey(const Key('rwNotifTplToggle_t1')), findsOneWidget);
    });

    testWidgets('CRUD: create, edit, toggle, and confirm-delete template refresh list', (tester) async {
      final api = FakeRwApiClient()
        ..logsToReturn = Future.value([logA])
        ..templatesToReturn = Future.value([tmpl]);

      await tester.pumpWidget(wrap(
        const RwNotificationScreen(rwId: 'rw-a'),
        user: userFor(superAdmin: false, role: 'pengurus', jabatan: 'ketua', rwId: 'rw-a', source: 'explicit')
          ..['effective_permissions'] = ['rw.notification.read', 'rw.notification.write'],
        api: api,
      ));
      await tester.pumpAndSettle();
      final initialListCalls = api.templatesCalls;

      // 1. Create
      await tester.tap(find.byKey(const Key('rwNotifCreateTemplateBtn')));
      await tester.pumpAndSettle();
      await tester.enterText(find.byKey(const Key('rwTemplateNameField')), 'Template Baru');
      await tester.enterText(find.byKey(const Key('rwTemplateTitleField')), 'Judul Baru');
      await tester.enterText(find.byKey(const Key('rwTemplateBodyField')), 'Isi Baru');
      await tester.tap(find.byKey(const Key('rwTemplateSaveBtn')));
      await tester.pumpAndSettle();
      expect(api.createTemplateCalls, 1);
      expect(api.templatesCalls, greaterThan(initialListCalls));

      // 2. Toggle active
      await tester.tap(find.byKey(const Key('rwNotifTplToggle_t1')));
      await tester.pumpAndSettle();
      expect(api.toggleTemplateCalls, 1);

      // 3. Delete with confirmation
      await tester.tap(find.byKey(const Key('rwNotifTplDelete_t1')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('rwTemplateConfirmDeleteBtn')));
      await tester.pumpAndSettle();
      expect(api.deleteTemplateCalls, 1);
    });

    testWidgets('Gallery upload: picks image from gallery, shows preview, uploads, and saves URL', (tester) async {
      final api = FakeRwApiClient()..templatesToReturn = Future.value(const []);
      ImageSource? pickedSource;

      await tester.pumpWidget(wrap(
        RwNotificationScreen(
          rwId: 'rw-a',
          imagePickerOverride: (source) async {
            pickedSource = source;
            return (bytes: validPngBytes, filename: 'gallery_banner.png');
          },
        ),
        user: userFor(superAdmin: false, role: 'pengurus', jabatan: 'ketua', rwId: 'rw-a', source: 'explicit')
          ..['effective_permissions'] = ['rw.notification.write'],
        api: api,
      ));
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const Key('rwNotifCreateTemplateBtn')));
      await tester.pumpAndSettle();

      await tester.enterText(find.byKey(const Key('rwTemplateNameField')), 'Info Kerja Bakti');
      await tester.enterText(find.byKey(const Key('rwTemplateTitleField')), 'Kerja Bakti RW');
      await tester.enterText(find.byKey(const Key('rwTemplateBodyField')), 'Minggu pukul 07.00');

      await tester.tap(find.byKey(const Key('rwTemplatePickGalleryBtn')));
      await tester.pumpAndSettle();

      expect(pickedSource, ImageSource.gallery);
      expect(find.byKey(const Key('rwTemplateDraftImagePreview')), findsOneWidget);
      expect(find.byKey(const Key('rwTemplateRemoveImageBtn')), findsOneWidget);

      await tester.tap(find.byKey(const Key('rwTemplateSaveBtn')));
      await tester.pumpAndSettle();

      expect(api.uploadImageCalls, 1);
      expect(api.lastUploadedFilename, 'gallery_banner.png');
      expect(api.lastUploadedEventType, 'manual_push');
      expect(api.createTemplateCalls, 1);
      expect(api.lastCreatedImageUrl, '/api/v1/notifications/images/rw-manual/rw-a/gallery_banner.png');
    });

    testWidgets('Camera capture: takes photo from camera, allows replace and remove draft image', (tester) async {
      final api = FakeRwApiClient()..templatesToReturn = Future.value(const []);
      final pickedSources = <ImageSource>[];

      await tester.pumpWidget(wrap(
        RwNotificationScreen(
          rwId: 'rw-a',
          imagePickerOverride: (source) async {
            pickedSources.add(source);
            if (source == ImageSource.gallery) {
              return (bytes: validPngBytes, filename: 'first_gallery.png');
            }
            return (bytes: validPngBytes, filename: 'camera_shot.png');
          },
        ),
        user: userFor(superAdmin: false, role: 'pengurus', jabatan: 'ketua', rwId: 'rw-a', source: 'explicit')
          ..['effective_permissions'] = ['rw.notification.write'],
        api: api,
      ));
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const Key('rwNotifCreateTemplateBtn')));
      await tester.pumpAndSettle();

      // Pick from gallery first
      await tester.ensureVisible(find.byKey(const Key('rwTemplatePickGalleryBtn')));
      await tester.tap(find.byKey(const Key('rwTemplatePickGalleryBtn')));
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('rwTemplateDraftImagePreview')), findsOneWidget);

      // Replace with camera photo
      await tester.ensureVisible(find.byKey(const Key('rwTemplatePickCameraBtn')));
      await tester.tap(find.byKey(const Key('rwTemplatePickCameraBtn')));
      await tester.pumpAndSettle();
      expect(pickedSources, [ImageSource.gallery, ImageSource.camera]);
      expect(find.byKey(const Key('rwTemplateDraftImagePreview')), findsOneWidget);

      // Remove draft image -> preview disappears
      await tester.ensureVisible(find.byKey(const Key('rwTemplateRemoveImageBtn')));
      await tester.tap(find.byKey(const Key('rwTemplateRemoveImageBtn')));
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('rwTemplateDraftImagePreview')), findsNothing);

      // Pick again with camera and save
      await tester.ensureVisible(find.byKey(const Key('rwTemplatePickCameraBtn')));
      await tester.tap(find.byKey(const Key('rwTemplatePickCameraBtn')));
      await tester.pumpAndSettle();
      await tester.enterText(find.byKey(const Key('rwTemplateNameField')), 'Foto Kamera');
      await tester.enterText(find.byKey(const Key('rwTemplateTitleField')), 'Judul Kamera');
      await tester.enterText(find.byKey(const Key('rwTemplateBodyField')), 'Isi Kamera');
      await tester.tap(find.byKey(const Key('rwTemplateSaveBtn')));
      await tester.pumpAndSettle();

      expect(api.uploadImageCalls, 1);
      expect(api.lastUploadedFilename, 'camera_shot.png');
      expect(api.lastCreatedImageUrl, '/api/v1/notifications/images/rw-manual/rw-a/camera_shot.png');
    });

    testWidgets('Edit template without changing image preserves existing image; cancelling picker keeps old image', (tester) async {
      final api = FakeRwApiClient()..templatesToReturn = Future.value([tmplWithImage]);

      await tester.pumpWidget(wrap(
        RwNotificationScreen(
          rwId: 'rw-a',
          imagePickerOverride: (_) async => null, // user cancels picker
        ),
        user: userFor(superAdmin: false, role: 'pengurus', jabatan: 'ketua', rwId: 'rw-a', source: 'explicit')
          ..['effective_permissions'] = ['rw.notification.write'],
        api: api,
      ));
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const Key('rwNotifTplEdit_t-img')));
      await tester.pumpAndSettle();

      // Existing saved image preview is visible
      expect(find.byKey(const Key('rwTemplateSavedImagePreview')), findsOneWidget);

      // Cancel image picker -> saved image must remain
      await tester.ensureVisible(find.byKey(const Key('rwTemplatePickGalleryBtn')));
      await tester.tap(find.byKey(const Key('rwTemplatePickGalleryBtn')));
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('rwTemplateSavedImagePreview')), findsOneWidget);

      // Save without changing image -> no upload, imageUrl omitted (null) so backend preserves it
      await tester.enterText(find.byKey(const Key('rwTemplateTitleField')), 'Judul Diperbarui');
      await tester.tap(find.byKey(const Key('rwTemplateSaveBtn')));
      await tester.pumpAndSettle();

      expect(api.uploadImageCalls, 0);
      expect(api.updateTemplateCalls, 1);
      expect(api.updateImageUrlPassed, isFalse);
      expect(api.lastUpdatedImageUrl, isNull);
    });

    testWidgets('Upload error and camera permission error show inline error and keep dialog open for retry', (tester) async {
      final api = FakeRwApiClient()
        ..templatesToReturn = Future.value(const [])
        ..uploadImageError = Exception('upload failed: 500');
      var failPermission = true;

      await tester.pumpWidget(wrap(
        RwNotificationScreen(
          rwId: 'rw-a',
          imagePickerOverride: (source) async {
            if (failPermission && source == ImageSource.camera) {
              throw PlatformException(code: 'camera_access_denied', message: 'Camera denied');
            }
            return (bytes: validPngBytes, filename: 'retry.png');
          },
        ),
        user: userFor(superAdmin: false, role: 'pengurus', jabatan: 'ketua', rwId: 'rw-a', source: 'explicit')
          ..['effective_permissions'] = ['rw.notification.write'],
        api: api,
      ));
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const Key('rwNotifCreateTemplateBtn')));
      await tester.pumpAndSettle();

      // 1. Camera permission denied shows friendly inline error
      await tester.ensureVisible(find.byKey(const Key('rwTemplatePickCameraBtn')));
      await tester.tap(find.byKey(const Key('rwTemplatePickCameraBtn')));
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('rwTemplateFormError')), findsOneWidget);
      expect(find.textContaining('Izin kamera ditolak'), findsOneWidget);

      // 2. Pick from gallery then fail upload -> dialog stays open with error
      failPermission = false;
      await tester.ensureVisible(find.byKey(const Key('rwTemplatePickGalleryBtn')));
      await tester.tap(find.byKey(const Key('rwTemplatePickGalleryBtn')));
      await tester.pumpAndSettle();
      await tester.enterText(find.byKey(const Key('rwTemplateNameField')), 'Retry Template');
      await tester.enterText(find.byKey(const Key('rwTemplateTitleField')), 'Retry Title');
      await tester.enterText(find.byKey(const Key('rwTemplateBodyField')), 'Retry Body');

      await tester.tap(find.byKey(const Key('rwTemplateSaveBtn')));
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('rwTemplateFormError')), findsOneWidget);
      expect(api.createTemplateCalls, 0);

      // 3. Clear upload error and retry -> succeeds!
      api.uploadImageError = null;
      await tester.tap(find.byKey(const Key('rwTemplateSaveBtn')));
      await tester.pumpAndSettle();
      expect(api.uploadImageCalls, 2);
      expect(api.createTemplateCalls, 1);
    });

    testWidgets('renders cleanly without overflow on small Android screen (320x640)', (tester) async {
      tester.view.physicalSize = const Size(320, 640);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(() {
        tester.view.resetPhysicalSize();
        tester.view.resetDevicePixelRatio();
      });

      final api = FakeRwApiClient()
        ..logsToReturn = Future.value([logA])
        ..templatesToReturn = Future.value([tmpl, tmplWithImage]);

      await tester.pumpWidget(wrap(
        const RwNotificationScreen(rwId: 'rw-a'),
        user: userFor(superAdmin: false, role: 'pengurus', jabatan: 'ketua', rwId: 'rw-a', source: 'explicit')
          ..['effective_permissions'] = ['rw.notification.read', 'rw.notification.write'],
        api: api,
      ));
      await tester.pumpAndSettle();

      expect(tester.takeException(), isNull);
      expect(find.byKey(const Key('rwNotifCreateTemplateBtn')), findsOneWidget);
      expect(find.byKey(const Key('rwNotifManualPushBtn')), findsOneWidget);
      expect(find.byKey(const Key('rwNotifTplEdit_t1')), findsOneWidget);
      expect(find.byKey(const Key('rwNotifTplDelete_t1')), findsOneWidget);
    });

    testWidgets('hides removed header wording and preserves Template / Riwayat Notifikasi Sistem tabs', (tester) async {
      final api = FakeRwApiClient()
        ..logsToReturn = Future.value([logA])
        ..templatesToReturn = Future.value([tmpl]);

      await tester.pumpWidget(wrap(
        const RwNotificationScreen(rwId: 'rw-a'),
        user: userFor(superAdmin: false, role: 'pengurus', jabatan: 'ketua', rwId: 'rw-a', source: 'explicit'),
        api: api,
      ));
      await tester.pumpAndSettle();

      expect(find.text('Notifikasi RW'), findsNothing);
      expect(find.text('Riwayat notifikasi dan template untuk RW ini.'), findsNothing);
      expect(find.text('Template'), findsOneWidget);
      expect(find.text('Riwayat Notifikasi Sistem'), findsOneWidget);
    });

    testWidgets('Ketua RW who is RT warga with non-empty RT effective_permissions still sees WRITE CRUD controls', (tester) async {
      // Reproduces live bug: /auth/me returns RT-scoped effective_permissions (finance.read, etc.)
      // without notification.manage for a Warga who is Ketua RW.
      final api = FakeRwApiClient()
        ..logsToReturn = Future.value([logA])
        ..templatesToReturn = Future.value([tmpl]);

      final ketuaRwWargaRt = <String, dynamic>{
        'id': 'u-mety',
        'email': 'mety@local.com',
        'name': 'Mety',
        'role': AppRole.warga,
        'effective_permissions': [
          'finance.read',
          'activities.read',
          'announcements.read',
          'banner_templates.read',
          'organization.read',
        ],
        'rw_memberships': [
          {
            'rw_id': 'rw-a',
            'rw_number': 16,
            'rw_name': 'RW 016',
            'jabatan': 'ketua',
            'source': 'explicit',
            'permissions': [
              'rw.dashboard.read',
              'rw.kas.read',
              'rw.kas.write',
              'rw.notification.read',
              'rw.notification.write',
              'notification.read',
              'notification.manage',
            ],
          }
        ],
      };

      await tester.pumpWidget(wrap(
        const RwNotificationScreen(rwId: 'rw-a'),
        user: ketuaRwWargaRt,
        api: api,
      ));
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('rwNotifCreateTemplateBtn')), findsOneWidget);
      expect(find.byKey(const Key('rwNotifManualPushBtn')), findsOneWidget);
      expect(find.byKey(const Key('rwNotifTplEdit_t1')), findsOneWidget);
      expect(find.byKey(const Key('rwNotifTplDelete_t1')), findsOneWidget);
    });

    testWidgets('Sekretaris RW who is also Ketua RT (RT notification.manage) stays READ-ONLY in RW', (tester) async {
      final api = FakeRwApiClient()
        ..logsToReturn = Future.value([logA])
        ..templatesToReturn = Future.value([tmpl]);

      final sekretarisRwKetuaRt = <String, dynamic>{
        'id': 'u-sek-rw',
        'email': 'sek@local.com',
        'name': 'Sek RW',
        'role': AppRole.pengurus,
        'jabatan': 'ketua',
        'effective_permissions': [
          'finance.read',
          'notification.read',
          'notification.manage',
        ],
        'rw_memberships': [
          {
            'rw_id': 'rw-a',
            'rw_number': 16,
            'rw_name': 'RW 016',
            'jabatan': 'sekretaris',
            'source': 'explicit',
            'permissions': [
              'rw.dashboard.read',
              'rw.kas.read',
              'rw.notification.read',
              'notification.read',
            ],
          }
        ],
      };

      await tester.pumpWidget(wrap(
        const RwNotificationScreen(rwId: 'rw-a'),
        user: sekretarisRwKetuaRt,
        api: api,
      ));
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('rwNotifCreateTemplateBtn')), findsNothing);
      expect(find.byKey(const Key('rwNotifManualPushBtn')), findsNothing);
      expect(find.byKey(const Key('rwNotifTplEdit_t1')), findsNothing);
      expect(find.byKey(const Key('rwNotifTplDelete_t1')), findsNothing);
      expect(find.byKey(const Key('rwNotifTplPreview_t1')), findsOneWidget);
    });
  });

  group('RW Notification Template Sub-Tab Counter & Heading Cleanup', () {
    testWidgets('hides old heading/subtitle and shows live template count next to Template sub-tab', (tester) async {
      final tmpl2 = RwNotificationTemplate(
        id: 't2',
        rwId: 'rw-a',
        name: 'Template 2',
        eventType: 'rw_cash_in',
        channel: 'push',
        title: 'Kas Masuk',
        body: 'Masuk {{amount}}',
        isActive: true,
      );
      final api = FakeRwApiClient()
        ..logsToReturn = Future.value([logA])
        ..templatesToReturn = Future.value([tmpl, tmpl2]);

      for (final brightness in [Brightness.light, Brightness.dark]) {
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              authRepositoryProvider.overrideWith(
                (_) => FakeAuthRepo(userFor(superAdmin: true, rwId: 'rw-a', source: 'explicit')),
              ),
              rwApiClientProvider.overrideWithValue(api),
            ],
            child: MaterialApp(
              theme: ThemeData(brightness: brightness),
              home: const Scaffold(body: RwNotificationScreen(rwId: 'rw-a')),
            ),
          ),
        );
        await tester.pumpAndSettle();

        final tabBar = find.byKey(const Key('rwNotifSubTabBar'));
        expect(find.descendant(of: tabBar, matching: find.byIcon(Icons.bookmark_border_rounded)), findsOneWidget);
        expect(find.descendant(of: tabBar, matching: find.text('Template')), findsOneWidget);
        expect(find.descendant(of: tabBar, matching: find.byKey(const Key('rwNotifTemplateCountBadge'))), findsOneWidget);
        expect(find.descendant(of: tabBar, matching: find.text('(2)')), findsOneWidget);

        // Old duplicated heading and subtitle must not appear
        expect(find.text('Template Notifikasi (2)'), findsNothing);
        expect(find.text('Kelola template pesan notifikasi RW'), findsNothing);

        // History tab & CRUD buttons remain intact
        expect(find.descendant(of: tabBar, matching: find.text('Riwayat Notifikasi Sistem')), findsOneWidget);
        expect(find.byKey(const Key('rwNotifCreateTemplateBtn')), findsOneWidget);
        expect(find.byKey(const Key('rwNotifManualPushBtn')), findsOneWidget);
      }
    });

    testWidgets('template sub-tab counter updates dynamically after create, delete, and refresh', (tester) async {
      final serverList = <RwNotificationTemplate>[tmpl];
      final api = _StatefulFakeRwApiClient(serverList, [logA]);

      await tester.pumpWidget(
        wrap(
          const RwNotificationScreen(rwId: 'rw-a'),
          user: userFor(superAdmin: true, rwId: 'rw-a', source: 'explicit'),
          api: api,
        ),
      );
      await tester.pumpAndSettle();

      final tabBar = find.byKey(const Key('rwNotifSubTabBar'));
      expect(find.descendant(of: tabBar, matching: find.text('(1)')), findsOneWidget);
      expect(find.text('Template Notifikasi (1)'), findsNothing);

      // 1) Create template -> count becomes (2)
      await tester.tap(find.byKey(const Key('rwNotifCreateTemplateBtn')));
      await tester.pumpAndSettle();
      await tester.enterText(find.byKey(const Key('rwTemplateNameField')), 'Template Baru');
      await tester.enterText(find.byKey(const Key('rwTemplateTitleField')), 'Judul Baru');
      await tester.enterText(find.byKey(const Key('rwTemplateBodyField')), 'Isi Baru');
      await tester.tap(find.byKey(const Key('rwTemplateSaveBtn')));
      await tester.pumpAndSettle();

      expect(find.descendant(of: tabBar, matching: find.text('(2)')), findsOneWidget);

      // 2) Delete first template (visible above SnackBar) -> count returns to (1)
      await tester.tap(find.byKey(const Key('rwNotifTplDelete_t1')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('rwTemplateConfirmDeleteBtn')));
      await tester.pumpAndSettle();

      expect(find.descendant(of: tabBar, matching: find.text('(1)')), findsOneWidget);

      // 3) Server changes + pull-to-refresh -> count reflects new server data (3)
      serverList.addAll([
        RwNotificationTemplate(
          id: 't-srv-2', rwId: 'rw-a', name: 'Srv 2', eventType: 'rw_cash_in',
          channel: 'push', title: 'S2', body: 'B2', isActive: true,
        ),
        RwNotificationTemplate(
          id: 't-srv-3', rwId: 'rw-a', name: 'Srv 3', eventType: 'rw_cash_out',
          channel: 'push', title: 'S3', body: 'B3', isActive: true,
        ),
      ]);
      await tester.drag(find.byType(ListView).first, const Offset(0, 300));
      await tester.pumpAndSettle();

      expect(find.descendant(of: tabBar, matching: find.text('(3)')), findsOneWidget);

      // 4) Switching to Riwayat Notifikasi Sistem tab still works cleanly
      await tester.tap(find.descendant(of: tabBar, matching: find.text('Riwayat Notifikasi Sistem')));
      await tester.pumpAndSettle();
      expect(find.text('A title'), findsOneWidget);
    });
  });
}

class _StatefulFakeRwApiClient extends FakeRwApiClient {
  final List<RwNotificationTemplate> _items;
  final List<RwNotificationLog> _logs;
  int _seq = 0;

  _StatefulFakeRwApiClient(this._items, this._logs);

  @override
  Future<RwNotificationLogsPage> getNotifications(String rwId, {int page = 1, int pageSize = 20, String? eventType}) async {
    return RwNotificationLogsPage(items: List.of(_logs), page: 1, pageSize: 20, total: _logs.length, totalPages: 1);
  }

  @override
  Future<List<RwNotificationTemplate>> listNotificationTemplates(String rwId, {String? eventType}) async {
    return List.of(_items);
  }

  @override
  Future<RwNotificationTemplate> createNotificationTemplate(
    String rwId, {
    required String name,
    required String eventType,
    required String title,
    required String body,
    String? imageUrl,
    String channel = 'push',
  }) async {
    _seq++;
    final created = RwNotificationTemplate(
      id: 't-created-$_seq',
      rwId: rwId,
      name: name,
      eventType: eventType,
      channel: channel,
      title: title,
      body: body,
      imageUrl: imageUrl,
      isActive: true,
    );
    _items.add(created);
    return created;
  }

  @override
  Future<void> deleteNotificationTemplate(String rwId, String templateId) async {
    _items.removeWhere((t) => t.id == templateId);
  }
}
