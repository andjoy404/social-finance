import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'api_warga_models.dart';
import 'warga_repository.dart';

/// Current search query in Warga screen.
final wargaSearchQueryProvider = StateProvider<String>((ref) => '');

/// Async state of the warga list.
/// null = never loaded / loading, AsyncError = error, AsyncData = success (possibly empty list).
final wargaListProvider = AsyncNotifierProvider<WargaListNotifier, List<MappedResident>>(() {
  return WargaListNotifier();
});

class WargaListNotifier extends AsyncNotifier<List<MappedResident>> {
  @override
  Future<List<MappedResident>> build() async {
    return _fetch();
  }

  Future<List<MappedResident>> _fetch() async {
    final repository = ref.read(wargaRepositoryProvider);
    final query = ref.read(wargaSearchQueryProvider).trim();

    state = const AsyncLoading();

    try {
      final response = await repository.fetchResidentsWithHouseholds(
        page: 1,
        pageSize: 100,
        search: query.isEmpty ? null : query,
      );
      return response.data;
    } catch (e, st) {
      final error = repository.mapDioError(e);
      state = AsyncError(error, st);
      return [];
    }
  }

  Future<void> refresh() async {
    await _fetch();
  }
}

/// Filtered resident list provider based on search query.
/// Uses the API-backed data instead of mock data.
final filteredWargaListProvider = Provider<List<MappedResident>>((ref) {
  final wargaState = ref.watch(wargaListProvider);
  final query = ref.watch(wargaSearchQueryProvider).trim().toLowerCase();

  // If loading or error, return empty so the screen shows the appropriate state.
  final residents = switch (wargaState) {
    AsyncData(:final value) => value,
    AsyncLoading() => <MappedResident>[],
    AsyncError() => <MappedResident>[],
    _ => <MappedResident>[],
  };

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
