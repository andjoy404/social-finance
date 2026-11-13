import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/device_id_service.dart';
import 'package:social_finance/core/models/role.dart';
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
  int toggleTemplateCalls = 0;
  int deleteTemplateCalls = 0;
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
  Future<RwNotificationTemplate> createNotificationTemplate(String rwId, {required String name, required String eventType, required String title, required String body, String channel = 'push'}) async {
    createTemplateCalls++; return _tmpl('new');
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
      expect(find.text('Riwayat Notifikasi'), findsOneWidget);
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
      expect(find.text('A title'), findsOneWidget);
      expect(find.text('Template 1'), findsOneWidget);
    });

    testWidgets('switching rwId requests new tenant and does not show old data', (tester) async {
      final api = FakeRwApiClient()..logsToReturn = Future.value([logA])..templatesToReturn = Future.value(const []);
      await tester.pumpWidget(wrap(const RwNotificationScreen(rwId: 'rw-a'),
          user: userFor(superAdmin: false, role: 'pengurus', jabatan: 'sekretaris', rwId: 'rw-a', source: 'explicit'), api: api));
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
  });
}
