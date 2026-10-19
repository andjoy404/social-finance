import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/warga/data/api_warga_models.dart';
import 'package:social_finance/features/warga/data/warga_providers.dart';
import 'package:social_finance/features/warga/data/warga_repository.dart';

class _MutableAuthRepo extends AuthRepository {
  _MutableAuthRepo(ApiClient client)
      : super(
          authService: AuthService(client),
          apiClient: client,
        );

  void setUser(Map<String, dynamic>? user) {
    if (user == null) {
      state = null;
    } else {
      state = AsyncData(user);
    }
  }
}

class _TrackingWargaRepository extends WargaRepository {
  _TrackingWargaRepository({required super.client});

  final List<String?> tokensUsed = [];

  @override
  Future<PaginatedResponse<MappedResident>> fetchResidentsWithHouseholds({
    int page = 1,
    int pageSize = 100,
    String? search,
  }) async {
    tokensUsed.add(client.token);
    return PaginatedResponse(
      data: [
        MappedResident(
          id: 'res-${tokensUsed.length}',
          name: 'Data for ${client.token?.substring(0, 8) ?? "none"}',
          houseNumber: 'A1',
          isHeadOfHousehold: true,
          relationship: 'Kepala Keluarga',
        ),
      ],
      pagination: const PaginationInfo(
        total: 1,
        page: 1,
        pageSize: 100,
        totalPages: 1,
      ),
    );
  }
}

void main() {
  group('Warga auth lifecycle regression', () {
    test('warga -> logout -> super_admin without restart', () async {
      final client = ApiClient(baseUrl: 'http://test.local');
      final authRepo = _MutableAuthRepo(client);
      final wargaRepo = _TrackingWargaRepository(client: client);

      final container = ProviderContainer(
        overrides: [
          authRepositoryProvider.overrideWith((ref) => authRepo),
          wargaRepositoryProvider.overrideWithValue(wargaRepo),
        ],
      );
      addTearDown(container.dispose);

      // 1. Login as warga
      authRepo.setUser({
        'id': 'warga-1',
        'email': 'warga@example.com',
        'role': AppRole.warga,
        'system_role': null,
        'jabatan': null,
        'access_token': 'warga-token-123',
      });
      client.setToken('warga-token-123');

      final wargaData = await container.read(wargaListProvider.future);
      expect(wargaData.length, equals(1));
      expect(wargaData.first.name, contains('warga-to'));
      expect(wargaRepo.tokensUsed.last, equals('warga-token-123'));

      // 2. Logout
      authRepo.setUser(null);
      client.clearAllTokens();

      final afterLogout = await container.read(wargaListProvider.future);
      expect(afterLogout, isEmpty);

      // 3. Login as super_admin
      authRepo.setUser({
        'id': 'admin-1',
        'email': 'admin@example.com',
        'role': AppRole.superAdmin,
        'system_role': 'super_admin',
        'jabatan': null,
        'access_token': 'admin-token-456',
      });
      client.setToken('admin-token-456');

      final adminData = await container.read(wargaListProvider.future);
      expect(adminData.length, equals(1));
      expect(adminData.first.name, contains('admin-to'));
      expect(wargaRepo.tokensUsed.last, equals('admin-token-456'));
      expect(wargaRepo.tokensUsed.length, greaterThanOrEqualTo(2));
    });
  });
}
