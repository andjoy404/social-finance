import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/features/rw/data/rw_api_client.dart';
import 'package:social_finance/features/rw/data/rw_models.dart';
import 'package:social_finance/features/rw/data/rw_repository.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';

/// Provider untuk RwApiClient (PR 4 — RW API operasional agregat).
final rwApiClientProvider = Provider<RwApiClient>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  return RwApiClient(apiClient);
});

/// Provider untuk RW Management CRUD (fitur lama yang sudah ada sebelum PR 4).
final rwListProvider = AsyncNotifierProvider.autoDispose<RwListNotifier, List<RwModel>>(
  () => RwListNotifier(),
);

class RwListNotifier extends AutoDisposeAsyncNotifier<List<RwModel>> {
  @override
  Future<List<RwModel>> build() async {
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

/// RW notification logs (paginated). Family key: (rwId, page).
/// Watches auth state so logout/account switch in the same app scope cannot reuse
/// a previous account's RW notification cache for the same rwId.
final rwNotificationLogsProvider = FutureProvider.autoDispose
    .family<RwNotificationLogsPage, ({String rwId, int page, String? eventType})>((ref, arg) async {
  ref.watch(authRepositoryProvider); // invalidate cache on auth/session change
  final client = ref.watch(rwApiClientProvider);
  return client.getNotifications(arg.rwId, page: arg.page, eventType: arg.eventType);
});

final rwNotificationTemplatesProvider = FutureProvider.autoDispose
    .family<List<RwNotificationTemplate>, String>((ref, rwId) async {
  ref.watch(authRepositoryProvider); // invalidate cache on auth/session change
  final client = ref.watch(rwApiClientProvider);
  return client.listNotificationTemplates(rwId);
});
