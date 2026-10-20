import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../features/auth/data/mock_auth_repository.dart';
import 'api_warga_models.dart';
import 'warga_repository.dart';

/// Current search query in Warga screen.
final wargaSearchQueryProvider = StateProvider<String>((ref) => '');

/// Async state of the warga list.
/// null = never loaded / loading, AsyncError = error, AsyncData = success (possibly empty list).
final wargaListProvider =
    AsyncNotifierProvider.autoDispose<WargaListNotifier, List<MappedResident>>(
      () => WargaListNotifier(),
    );

class WargaListNotifier extends AutoDisposeAsyncNotifier<List<MappedResident>> {
  @override
  Future<List<MappedResident>> build() async {
    // Depend on the current auth state so this provider is recreated/refetched
    // when the authenticated user changes (e.g. warga -> super_admin).
    final authState = ref.watch(authRepositoryProvider);
    final user = authState?.valueOrNull;
    if (user == null) {
      return [];
    }
    return _fetch();
  }

  Future<List<MappedResident>> _fetch() async {
    final repository = ref.read(wargaRepositoryProvider);
    final query = ref.read(wargaSearchQueryProvider).trim();

    try {
      final response = await repository.fetchResidentsWithHouseholds(
        page: 1,
        pageSize: 100,
        search: query.isEmpty ? null : query,
      );
      return response.data;
    } catch (e) {
      throw repository.mapDioError(e);
    }
  }

  Future<void> refresh() async {
    state = const AsyncLoading<List<MappedResident>>().copyWithPrevious(state);
    state = await AsyncValue.guard(() => _fetch());
  }
}

/// Filtered resident list provider based on search query.
/// Uses the API-backed data instead of mock data.
final filteredWargaListProvider = Provider<List<MappedResident>>((ref) {
  final wargaState = ref.watch(wargaListProvider);
  final query = ref.watch(wargaSearchQueryProvider).trim().toLowerCase();

  final residents = wargaState.valueOrNull ?? <MappedResident>[];

  if (query.isEmpty) {
    return residents;
  }

  return residents.where((resident) {
    final nameMatch = resident.name.toLowerCase().contains(query);
    final nikMatch = resident.nik?.toLowerCase().contains(query) ?? false;
    final houseMatch = resident.houseNumber.toLowerCase().contains(query);
    return nameMatch || nikMatch || houseMatch;
  }).toList();
});
