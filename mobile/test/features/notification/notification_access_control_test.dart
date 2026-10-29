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
import 'package:social_finance/features/notification/presentation/screens/notification_management_screen.dart';

class _FakeNotificationRepository extends NotificationRepository {
  _FakeNotificationRepository()
      : super(apiClient: NotificationApiClient(dio: Dio()));

  @override
  Future<List<SystemNotificationType>> fetchSystemTypes() async {
    return const [
      SystemNotificationType(
        eventType: 'waste_truck_arrival',
        name: 'Kedatangan Mobil Sampah',
        trigger: 'Diterbitkan saat mobil sampah tiba di RT',
        recipientsRule: 'Seluruh warga RT',
        defaultTitle: 'Mobil Sampah Tiba',
        defaultBody: 'Truk pengangkut sampah sudah berada di lingkungan RT.',
        allowedVariables: [],
        supportsImage: true,
        supportsTest: true,
      ),
      SystemNotificationType(
        eventType: 'manual_push',
        name: 'Pengumuman Manual RT',
        trigger: 'Dikirim manual oleh Ketua RT atau Superadmin',
        recipientsRule: 'Seluruh warga RT',
        defaultTitle: 'Pengumuman RT',
        defaultBody: 'Informasi penting untuk warga RT.',
        allowedVariables: [],
        supportsImage: true,
        supportsTest: true,
      ),
    ];
  }

  @override
  Future<List<NotificationTemplate>> fetchTemplates() async => const [];

  @override
  Future<NotificationLogsResponse> fetchLogs({
    int page = 1,
    int perPage = 20,
    String? eventType,
  }) async {
    return const NotificationLogsResponse(
      data: [],
      page: 1,
      perPage: 20,
      total: 0,
      totalPages: 1,
    );
  }
}

rp.ProviderScope _createApp({
  required Map<String, dynamic> user,
}) {
  final fakeRepo = _FakeNotificationRepository();
  return rp.ProviderScope(
    overrides: [
      authRepositoryProvider.overrideWith((_) {
        final client = ApiClient(customDio: Dio());
        final repo = AuthRepository(
          authService: AuthService(client),
          apiClient: client,
        );
        repo.state = rp.AsyncData<Map<String, dynamic>?>(user);
        return repo;
      }),
      notificationRepositoryProvider.overrideWithValue(fakeRepo),
      notificationSystemTypesProvider
          .overrideWith((_) => fakeRepo.fetchSystemTypes()),
      notificationTemplatesProvider
          .overrideWith((_) => fakeRepo.fetchTemplates()),
    ],
    child: const App(),
  );
}

void main() {
  group('Notification Menu Access Control (Unit)', () {
    test('super_admin via system_role has full access', () {
      final user = {
        'id': 'u-admin',
        'system_role': 'super_admin',
        'role': AppRole.pengurus,
        'jabatan': null,
      };
      expect(canUserAccessNotificationMenu(user), isTrue);
    });

    test('super_admin via AppRole.superAdmin has full access', () {
      final user = {
        'id': 'u-admin-role',
        'system_role': null,
        'role': AppRole.superAdmin,
        'jabatan': null,
      };
      expect(canUserAccessNotificationMenu(user), isTrue);
    });

    test('Ketua RT has access', () {
      final user = {
        'id': 'u-ketua',
        'system_role': null,
        'role': AppRole.pengurus,
        'jabatan': 'ketua',
      };
      expect(canUserAccessNotificationMenu(user), isTrue);
    });

    test('Bendahara RT does NOT have access', () {
      final user = {
        'id': 'u-bendahara',
        'system_role': null,
        'role': AppRole.bendahara,
        'jabatan': 'bendahara',
      };
      expect(canUserAccessNotificationMenu(user), isFalse);
    });

    test('Warga does NOT have access', () {
      final user = {
        'id': 'u-warga',
        'system_role': null,
        'role': AppRole.warga,
        'jabatan': null,
      };
      expect(canUserAccessNotificationMenu(user), isFalse);
    });

    test('Pengurus non-ketua (sekretaris & wakil_ketua) does NOT have access', () {
      expect(
        canUserAccessNotificationMenu({
          'id': 'u-sekre',
          'system_role': null,
          'role': AppRole.pengurus,
          'jabatan': 'sekretaris',
        }),
        isFalse,
      );
      expect(
        canUserAccessNotificationMenu({
          'id': 'u-wakil',
          'system_role': null,
          'role': AppRole.pengurus,
          'jabatan': 'wakil_ketua',
        }),
        isFalse,
      );
    });
  });

  group('Notification Menu Navigation & Visibility (Widget)', () {
    testWidgets(
      'Superadmin sees Notifikasi tab in BottomAppBar and can open NotificationManagementScreen',
      (tester) async {
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

        await tester.pumpWidget(_createApp(user: superAdminUser));
        await tester.pumpAndSettle();

        final bottomNav = find.byType(BottomAppBar);
        final notifTab = find.descendant(
          of: bottomNav,
          matching: find.text('Notifikasi'),
        );
        expect(notifTab, findsOneWidget);

        await tester.tap(notifTab);
        await tester.pumpAndSettle();

        expect(find.byType(NotificationManagementScreen), findsOneWidget);
        expect(find.text('KELOLA NOTIFIKASI'), findsOneWidget);
        // Superadmin sees Test Notifikasi button
        expect(find.text('Test Notifikasi'), findsWidgets);
      },
    );

    testWidgets(
      'Ketua RT sees Notifikasi tab in BottomAppBar and can open NotificationManagementScreen (with Test Notifikasi hidden)',
      (tester) async {
        final ketuaUser = {
          'id': '0fa41f40-0000-4000-8000-000000000001',
          'email': 'ketua-002@local.com',
          'name': 'Position Ketua RT 002',
          'role': AppRole.pengurus,
          'system_role': null,
          'jabatan': 'ketua',
          'rt': '002',
          'rt_id': 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
        };

        await tester.pumpWidget(_createApp(user: ketuaUser));
        await tester.pumpAndSettle();

        final bottomNav = find.byType(BottomAppBar);
        final notifTab = find.descendant(
          of: bottomNav,
          matching: find.text('Notifikasi'),
        );
        expect(notifTab, findsOneWidget);

        await tester.tap(notifTab);
        await tester.pumpAndSettle();

        expect(find.byType(NotificationManagementScreen), findsOneWidget);
        expect(find.text('KELOLA NOTIFIKASI'), findsOneWidget);
        // Ketua RT can customize templates and broadcast manual push, but Test Notifikasi is Superadmin-only
        expect(find.text('Kustomisasi'), findsWidgets);
        expect(find.text('Kirim Notifikasi'), findsOneWidget);
        expect(find.text('Test Notifikasi'), findsNothing);
      },
    );

    testWidgets(
      'Unauthorized roles (Warga, Bendahara, Sekretaris) do NOT see Notifikasi tab in BottomAppBar',
      (tester) async {
        for (final unauthorizedUser in [
          {
            'id': 'u-warga',
            'email': 'warga@local.com',
            'name': 'Warga RT',
            'role': AppRole.warga,
            'system_role': null,
            'jabatan': null,
            'rt': '002',
            'rt_id': 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
          },
          {
            'id': 'u-bendahara',
            'email': 'bendahara-002@local.com',
            'name': 'Bendahara RT',
            'role': AppRole.bendahara,
            'system_role': null,
            'jabatan': 'bendahara',
            'rt': '002',
            'rt_id': 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
          },
          {
            'id': 'u-sekretaris',
            'email': 'sekretaris-002@local.com',
            'name': 'Sekretaris RT',
            'role': AppRole.pengurus,
            'system_role': null,
            'jabatan': 'sekretaris',
            'rt': '002',
            'rt_id': 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
          },
        ]) {
          await tester.pumpWidget(
            KeyedSubtree(
              key: ValueKey(unauthorizedUser['id']),
              child: _createApp(user: unauthorizedUser),
            ),
          );
          await tester.pumpAndSettle();

          final bottomNav = find.byType(BottomAppBar);
          expect(
            find.descendant(of: bottomNav, matching: find.text('Notifikasi')),
            findsNothing,
          );
        }
      },
    );
  });
}

