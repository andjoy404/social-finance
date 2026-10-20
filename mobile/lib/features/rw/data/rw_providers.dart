import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../features/auth/data/mock_auth_repository.dart';
import 'rw_models.dart';
import 'rw_repository.dart';

final rwListProvider = AsyncNotifierProvider.autoDispose<RwListNotifier, List<RwModel>>(
  () => RwListNotifier(),
);

class RwListNotifier extends AutoDisposeAsyncNotifier<List<RwModel>> {
  @override
  Future<List<RwModel>> build() async {
    final authState = ref.watch(authRepositoryProvider);
    if (authState?.valueOrNull == null) return [];
    return _fetch();
  }

  Future<List<RwModel>> _fetch() async {
    final repo = ref.read(rwRepositoryProvider);
    state = const AsyncLoading();
    try {
      final result = await repo.listRws();
      state = AsyncData(result);
      return result;
    } catch (e, st) {
      final error = repo.mapDioError(e);
      state = AsyncError(error, st);
      return [];
    }
  }

  Future<void> refresh() async {
    await _fetch();
  }
}
