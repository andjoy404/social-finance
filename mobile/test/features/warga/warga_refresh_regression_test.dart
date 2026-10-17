import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/features/warga/data/api_warga_models.dart';
import 'package:social_finance/features/warga/data/warga_providers.dart';
import 'package:social_finance/features/warga/data/warga_repository.dart';

class _FakeWargaRepository extends WargaRepository {
  _FakeWargaRepository() : super(client: ApiClient(baseUrl: 'http://test.local'));

  int callCount = 0;
  List<MappedResident> dataToReturn = [];

  @override
  Future<PaginatedResponse<MappedResident>> fetchResidentsWithHouseholds({
    int page = 1,
    int pageSize = 100,
    String? search,
  }) async {
    callCount++;
    return PaginatedResponse(
      data: dataToReturn,
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
  group('WargaListNotifier and filteredWargaListProvider refresh regression', () {
    test('refresh() updates state to AsyncData and does not leave it stuck on AsyncLoading', () async {
      final fakeRepo = _FakeWargaRepository();
      fakeRepo.dataToReturn = [
        const MappedResident(
          id: 'res-1',
          name: 'Budi Santoso',
          houseNumber: 'A1',
          isHeadOfHousehold: true,
          relationship: 'Kepala Keluarga',
        ),
      ];

      final container = ProviderContainer(
        overrides: [
          wargaRepositoryProvider.overrideWithValue(fakeRepo),
        ],
      );
      addTearDown(container.dispose);

      // Initial read
      final initialData = await container.read(wargaListProvider.future);
      expect(initialData.length, equals(1));
      expect(initialData.first.name, equals('Budi Santoso'));
      expect(container.read(wargaListProvider), isA<AsyncData<List<MappedResident>>>());

      // Update data in repo (as if a new resident was added)
      fakeRepo.dataToReturn = [
        const MappedResident(
          id: 'res-1',
          name: 'Budi Santoso',
          houseNumber: 'A1',
          isHeadOfHousehold: true,
          relationship: 'Kepala Keluarga',
        ),
        const MappedResident(
          id: 'res-2',
          name: 'Siti Aminah',
          houseNumber: 'A2',
          isHeadOfHousehold: true,
          relationship: 'Kepala Keluarga',
        ),
      ];

      // Call refresh
      await container.read(wargaListProvider.notifier).refresh();

      // State MUST be AsyncData with 2 residents (NOT AsyncLoading or empty list)
      final stateAfterRefresh = container.read(wargaListProvider);
      expect(stateAfterRefresh, isA<AsyncData<List<MappedResident>>>());
      expect(stateAfterRefresh.value!.length, equals(2));

      // filteredWargaListProvider must also contain 2 residents
      final filteredList = container.read(filteredWargaListProvider);
      expect(filteredList.length, equals(2));
      expect(filteredList[1].name, equals('Siti Aminah'));
    });
  });
}
