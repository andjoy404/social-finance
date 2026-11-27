import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/core/navigators/kas_form_navigation_guard.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/widgets/accent_action_fab.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/dashboard/data/community_repository.dart';
import 'package:social_finance/features/dashboard/data/models/community_models.dart';
import 'package:social_finance/features/kegiatan/presentation/screens/kegiatan_screen.dart';
import 'package:social_finance/features/pengumuman/presentation/screens/announcement_form_screen.dart';

class _FakeCommunityRepository extends CommunityRepository {
  _FakeCommunityRepository() : super(dio: Dio());

  @override
  Future<List<BannerTemplate>> getTemplates() async => [];

  @override
  Future<CommunityPage<CommunityActivity>> listActivities({
    String? status,
    String? publicationStatus,
    int page = 1,
    int pageSize = 20,
  }) async {
    final now = DateTime.now();
    return CommunityPage<CommunityActivity>(
      items: [
        CommunityActivity(
          id: 'act-1',
          rtId: 'rt-002',
          title: 'Kerja Bakti RT',
          description: 'Membersihkan selokan',
          location: 'Pos RW 016',
          startDate: now,
          status: 'upcoming',
          publicationStatus: 'published',
          createdAt: now,
          updatedAt: now,
        ),
      ],
      page: 1,
      perPage: 20,
      total: 1,
      totalPages: 1,
    );
  }

  @override
  Future<CommunityPage<CommunityAnnouncement>> listAnnouncements({
    String? status,
    int page = 1,
    int pageSize = 10,
  }) async {
    final now = DateTime.now();
    return CommunityPage<CommunityAnnouncement>(
      items: [
        CommunityAnnouncement(
          id: 'ann-1',
          rtId: 'rt-002',
          title: 'Pengumuman Penting',
          body: 'Isi pengumuman',
          status: 'published',
          priority: 10,
          displayOrder: 0,
          createdAt: now,
          updatedAt: now,
        ),
      ],
      page: 1,
      perPage: 10,
      total: 1,
      totalPages: 1,
    );
  }
}

class _TestAuthRepo extends AuthRepository {
  _TestAuthRepo()
      : super(
          authService: AuthService(ApiClient(baseUrl: 'http://test.local')),
          apiClient: ApiClient(baseUrl: 'http://test.local'),
        ) {
    state = const AsyncData({
      'id': 'user-1',
      'role': AppRole.superAdmin,
      'name': 'Super Admin',
      'effective_permissions': ['activities.create', 'activities.update'],
    });
  }
}

void main() {
  group('Global Button Color Standardization - Pengumuman & Kegiatan', () {
    testWidgets('AnnouncementFormScreen has solid red Batal button and green Simpan button with equal width', (tester) async {
      tester.view.physicalSize = const Size(800, 2500);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            communityRepositoryProvider.overrideWithValue(_FakeCommunityRepository()),
          ],
          child: const MaterialApp(
            home: Scaffold(
              body: AnnouncementFormScreen(),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      final batalFinder = find.widgetWithText(ElevatedButton, 'Batal');
      expect(batalFinder, findsOneWidget);
      final batalBtn = tester.widget<ElevatedButton>(batalFinder);
      expect(batalBtn.style?.backgroundColor?.resolve({}), AppColors.danger);
      expect(batalBtn.style?.foregroundColor?.resolve({}), Colors.white);

      final simpanFinder = find.widgetWithText(ElevatedButton, 'Simpan');
      expect(simpanFinder, findsOneWidget);
      final simpanBtn = tester.widget<ElevatedButton>(simpanFinder);
      expect(simpanBtn.style?.backgroundColor?.resolve({}), AppColors.success);
      expect(simpanBtn.style?.foregroundColor?.resolve({}), Colors.white);

      final batalSize = tester.getSize(batalFinder);
      final simpanSize = tester.getSize(simpanFinder);
      expect(batalSize.width, simpanSize.width);
      expect(batalSize.height, 48.0);
      expect(simpanSize.height, 48.0);
    });

    testWidgets('Announcement delete confirmation dialog has solid red Batal button and neutral action is not red', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Builder(
            builder: (context) => Scaffold(
              body: ElevatedButton(
                onPressed: () {
                  showDialog<bool>(
                    context: context,
                    builder: (ctx) => AlertDialog(
                      title: const Text('Hapus Pengumuman?'),
                      content: const Text('Apakah Anda yakin?'),
                      actions: [
                        ElevatedButton(
                          style: ElevatedButton.styleFrom(
                            backgroundColor: AppColors.danger,
                            foregroundColor: Colors.white,
                            elevation: 0,
                          ),
                          onPressed: () => Navigator.of(ctx).pop(false),
                          child: const Text('Batal'),
                        ),
                        FilledButton(
                          style: FilledButton.styleFrom(
                            backgroundColor: AppColors.danger,
                            foregroundColor: Colors.white,
                          ),
                          onPressed: () => Navigator.of(ctx).pop(true),
                          child: const Text('Hapus'),
                        ),
                      ],
                    ),
                  );
                },
                child: const Text('Open Dialog'),
              ),
            ),
          ),
        ),
      );

      await tester.tap(find.text('Open Dialog'));
      await tester.pumpAndSettle();

      final batalFinder = find.widgetWithText(ElevatedButton, 'Batal');
      expect(batalFinder, findsOneWidget);
      final batalBtn = tester.widget<ElevatedButton>(batalFinder);
      expect(batalBtn.style?.backgroundColor?.resolve({}), AppColors.danger);
      expect(batalBtn.style?.foregroundColor?.resolve({}), Colors.white);

      final hapusFinder = find.widgetWithText(FilledButton, 'Hapus');
      expect(hapusFinder, findsOneWidget);
      final hapusBtn = tester.widget<FilledButton>(hapusFinder);
      expect(hapusBtn.style?.backgroundColor?.resolve({}), AppColors.danger);
      expect(hapusBtn.style?.foregroundColor?.resolve({}), Colors.white);
    });

    testWidgets('Kegiatan modal form has solid red Batal button and green Simpan button with equal width', (tester) async {
      tester.view.physicalSize = const Size(800, 2500);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            communityRepositoryProvider.overrideWithValue(_FakeCommunityRepository()),
            authRepositoryProvider.overrideWith((ref) => _TestAuthRepo()),
          ],
          child: const MaterialApp(
            home: KegiatanScreen(),
          ),
        ),
      );
      await tester.pumpAndSettle();

      // Open Add Activity modal via AccentActionFab
      final fab = find.byType(AccentActionFab);
      expect(fab, findsOneWidget);
      await tester.tap(fab);
      await tester.pumpAndSettle();

      final batalFinder = find.widgetWithText(ElevatedButton, 'Batal');
      expect(batalFinder, findsOneWidget);
      final batalBtn = tester.widget<ElevatedButton>(batalFinder);
      expect(batalBtn.style?.backgroundColor?.resolve({}), AppColors.danger);
      expect(batalBtn.style?.foregroundColor?.resolve({}), Colors.white);

      final simpanFinder = find.widgetWithText(ElevatedButton, 'Simpan');
      expect(simpanFinder, findsOneWidget);
      final simpanBtn = tester.widget<ElevatedButton>(simpanFinder);
      expect(simpanBtn.style?.backgroundColor?.resolve({}), AppColors.success);
      expect(simpanBtn.style?.foregroundColor?.resolve({}), Colors.white);

      final batalSize = tester.getSize(batalFinder);
      final simpanSize = tester.getSize(simpanFinder);
      expect(batalSize.width, simpanSize.width);
      expect(batalSize.height, 48.0);
      expect(simpanSize.height, 48.0);
    });

    testWidgets('KasFormNavigationGuard confirm dialog: Ya Batalkan is red, Tidak is neutral', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Builder(
            builder: (context) => Scaffold(
              body: ElevatedButton(
                onPressed: () => KasFormNavigationGuard.confirm(context),
                child: const Text('Open Guard Dialog'),
              ),
            ),
          ),
        ),
      );

      await tester.tap(find.text('Open Guard Dialog'));
      await tester.pumpAndSettle();

      // "Ya, Batalkan" button must have danger background
      final yaBatalkanFinder = find.byKey(const Key('kas-cancel-confirm'));
      expect(yaBatalkanFinder, findsOneWidget);
      final yaBatalkanBtn = tester.widget<ElevatedButton>(yaBatalkanFinder);
      expect(yaBatalkanBtn.style?.backgroundColor?.resolve({}), AppColors.danger);

      // "Tidak, Tetap di Halaman" is neutral action, must NOT be danger colored
      final tidakFinder = find.byKey(const Key('kas-cancel-stay'));
      expect(tidakFinder, findsOneWidget);
      final tidakBtn = tester.widget<ElevatedButton>(tidakFinder);
      expect(tidakBtn.style?.backgroundColor?.resolve({}), isNot(AppColors.danger));
      expect(tidakBtn.style?.foregroundColor?.resolve({}), isNot(AppColors.danger));
    });

    testWidgets('Full-screen form bottomNavigationBar enforces equal full-width for Simpan and Batal', (tester) async {
      tester.view.physicalSize = const Size(400, 800);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);

      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: const Center(child: Text('Form Body')),
            bottomNavigationBar: SafeArea(
              child: Padding(
                padding: const EdgeInsets.all(16.0),
                child: Row(
                  children: [
                    Expanded(
                      child: SizedBox(
                        height: 48,
                        child: ElevatedButton(
                          onPressed: () {},
                          style: ElevatedButton.styleFrom(
                            backgroundColor: AppColors.success,
                            foregroundColor: Colors.white,
                            elevation: 0,
                          ),
                          child: const Text('Simpan'),
                        ),
                      ),
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: SizedBox(
                        height: 48,
                        child: ElevatedButton(
                          onPressed: () {},
                          style: ElevatedButton.styleFrom(
                            backgroundColor: AppColors.danger,
                            foregroundColor: Colors.white,
                            elevation: 0,
                          ),
                          child: const Text('Batal'),
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      final simpanFinder = find.widgetWithText(ElevatedButton, 'Simpan');
      final batalFinder = find.widgetWithText(ElevatedButton, 'Batal');
      final simpanSize = tester.getSize(simpanFinder);
      final batalSize = tester.getSize(batalFinder);

      // Equal width
      expect(simpanSize.width, equals(batalSize.width));
      // Standard 48px height
      expect(simpanSize.height, equals(48.0));
      expect(batalSize.height, equals(48.0));

      // Fills available width: (400 - 32 padding - 12 gap) / 2 = 178
      expect(simpanSize.width, equals(178.0));
      expect(batalSize.width, equals(178.0));
    });
  });
}
