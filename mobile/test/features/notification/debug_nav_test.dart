import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart' as rp;
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/app/app.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/notification/data/models/notification_models.dart';
import 'package:social_finance/features/notification/data/notification_api_client.dart';
import 'package:social_finance/features/notification/data/notification_repository.dart';

class _FakeNotificationRepository extends NotificationRepository {
  _FakeNotificationRepository() : super(apiClient: NotificationApiClient(dio: Dio()));
  @override
  Future<List<SystemNotificationType>> fetchSystemTypes() async => const [];
  @override
  Future<List<NotificationTemplate>> fetchTemplates() async => const [];
  @override
  Future<NotificationLogsResponse> fetchLogs({int page = 1, int perPage = 20, String? eventType, String? status}) async =>
      const NotificationLogsResponse(data: [], page: 1, perPage: 20, total: 0, totalPages: 1);
}

void main() {
  testWidgets('debug superadmin tabs', (tester) async {
    final superAdminUser = {
      'id': 'b69669cb-7145-4991-8192-8b017b12f79e',
      'email': 'admin@local.com',
      'name': 'Super Admin',
      'role': AppRole.superAdmin,
      'system_role': 'super_admin',
      'jabatan': null,
      'rt': '002',
      'rt_id': 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
    };
    final fakeRepo = _FakeNotificationRepository();
    await tester.pumpWidget(rp.ProviderScope(
      overrides: [
        authRepositoryProvider.overrideWith((_) {
          final client = ApiClient(customDio: Dio());
          final repo = AuthRepository(authService: AuthService(client), apiClient: client);
          repo.state = rp.AsyncData<Map<String, dynamic>?>(superAdminUser);
          return repo;
        }),
        notificationRepositoryProvider.overrideWithValue(fakeRepo),
        notificationSystemTypesProvider.overrideWith((_) => fakeRepo.fetchSystemTypes()),
        notificationTemplatesProvider.overrideWith((_) => fakeRepo.fetchTemplates()),
      ],
      child: const App(),
    ));
    await tester.pumpAndSettle();
    final bottomNav = find.byType(BottomAppBar);
    print('BOTTOM_NAV_FOUND: ${bottomNav.evaluate().length}');
    // find all text widgets inside bottom nav
    final texts = find.descendant(of: bottomNav, matching: find.byType(Text));
    for (final e in texts.evaluate()) {
      final w = e.widget as Text;
      print('TAB_TEXT: ${w.data}');
    }
    // also check all text
    for (final e in find.byType(Text).evaluate()) {
      final w = e.widget as Text;
      if ((w.data ?? '').contains('Notif')) print('ANY_NOTIF: ${w.data}');
    }
  });
}
