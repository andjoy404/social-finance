// Dashboard banner thumbnail rendering test — ensures banner image_url
// from the backend response reaches CommunityImage on Beranda.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/api/device_id_service.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/features/dashboard/data/community_repository.dart';
import 'package:social_finance/features/dashboard/data/models/community_models.dart';
import 'package:social_finance/features/dashboard/presentation/screens/dashboard_screen.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/core/widgets/community_image.dart';

class _MockAuthRepo extends AuthRepository {
  _MockAuthRepo(Map<String, dynamic>? user)
      : super(
          authService: AuthService(
            ApiClient(baseUrl: 'http://test.local'),
            DeviceIdService(),
          ),
          apiClient: ApiClient(baseUrl: 'http://test.local'),
        ) {
    state = AsyncData(user);
  }
}

HomeDashboardData _dataWithBanner({String? imageUrl}) {
  return HomeDashboardData(
    greeting: 'Selamat datang,',
    userName: 'Warga',
    userRole: 'warga',
    userJabatan: '',
    rt: '002',
    rw: '016',
    banners: [
      CommunityBanner(
        id: 'b1',
        rtId: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
        title: 'Pengumuman Penting',
        description: 'Isi pengumuman',
        imageUrl: imageUrl,
        sourceType: 'announcement',
        sourceId: 'a1',
        actionUrl: '/pengumuman',
        createdAt: DateTime(2026, 11, 11),
      ),
    ],
    security: const SecurityOnDutyInfo(
      isAvailable: false,
      officers: [],
      message: 'Belum ada petugas aktif / jadwal shift saat ini',
      subtext: '',
    ),
    organization: const OrganizationStructure(),
    upcomingActivities: const [],
  );
}

void main() {
  group('DashboardScreen — Banner thumbnail wiring (Android)', () {
    Widget buildApp(HomeDashboardData data, {required bool withImage}) {
      return ProviderScope(
        overrides: [
          authRepositoryProvider.overrideWith(
            (ref) => _MockAuthRepo({
              'id': 'user-1',
              'email': 'warga@example.com',
              'name': 'Warga Test',
              'role': AppRole.warga,
              'system_role': null,
              'jabatan': null,
              'rt': '002',
              'rw': '016',
              'rt_id': 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
            }),
          ),
          homeDashboardFutureProvider.overrideWith(
            (ref) => Future<HomeDashboardData>.value(data),
          ),
        ],
        child: const MaterialApp(home: DashboardScreen()),
      );
    }

    testWidgets('renders banner image through CommunityImage when present', (tester) async {
      final imageURL = '/api/v1/community/images/community/rt-1/announcement/tpl.png';
      final data = _dataWithBanner(imageUrl: imageURL);

      await tester.pumpWidget(buildApp(data, withImage: true));
      await tester.pumpAndSettle();

      expect(find.byType(CommunityImage), findsOneWidget);
      final widget = tester.widget<CommunityImage>(find.byType(CommunityImage));
      expect(widget.imageRef, equals(imageURL));
    });

    testWidgets('hides thumbnail when banner has no imageUrl', (tester) async {
      final data = _dataWithBanner(imageUrl: null);

      await tester.pumpWidget(buildApp(data, withImage: false));
      await tester.pumpAndSettle();

      expect(find.byType(CommunityImage), findsNothing);
    });
  });
}
