import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/api/models.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/core/utils/permissions.dart';
import 'package:social_finance/features/rw/data/rw_api_client.dart';
import 'package:social_finance/features/rw/data/rw_models.dart';
import 'package:social_finance/features/rw/data/rw_repository.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/security/data/security_models.dart';

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

/// RW notification logs sent by current user (paginated).
final rwNotificationLogsMineProvider = FutureProvider.autoDispose
    .family<RwNotificationLogsPage, ({String rwId, int page, String? eventType})>((ref, arg) async {
  ref.watch(authRepositoryProvider); // invalidate cache on auth/session change
  final client = ref.watch(rwApiClientProvider);
  return client.getNotificationLogsMine(arg.rwId, page: arg.page, eventType: arg.eventType);
});

final rwNotificationTemplatesProvider = FutureProvider.autoDispose
    .family<List<RwNotificationTemplate>, String>((ref, rwId) async {
  ref.watch(authRepositoryProvider); // invalidate cache on auth/session change
  final client = ref.watch(rwApiClientProvider);
  return client.listNotificationTemplates(rwId);
});

final rwUnreadCountProvider = FutureProvider.autoDispose
    .family<int, String>((ref, rwId) async {
  ref.watch(authRepositoryProvider);
  final client = ref.watch(rwApiClientProvider);
  return client.getRWUnreadCount(rwId);
});

/// Per-tab unread state for the DATA RW screen.
///
/// In the current backend API, only RW notifications (`GET /api/v1/rw/{rwId}/notifications/unread-count`)
/// produce unread counts; `berandaUnread`, `kasUnread`, and `lokasiDanaUnread` remain `0` unless
/// a dedicated unread source exists. Bottom navigation and DATA RW tabs both derive their red dot
/// badges from this single state so they stay strictly synchronized.
class RwTabUnreadState {
  final int berandaUnread;
  final int kasUnread;
  final int lokasiDanaUnread;
  final int notifikasiUnread;

  const RwTabUnreadState({
    this.berandaUnread = 0,
    this.kasUnread = 0,
    this.lokasiDanaUnread = 0,
    this.notifikasiUnread = 0,
  });

  bool get hasBerandaUnread => berandaUnread > 0;
  bool get hasKasUnread => kasUnread > 0;
  bool get hasLokasiDanaUnread => lokasiDanaUnread > 0;
  bool get hasNotifikasiUnread => notifikasiUnread > 0;
  int get totalUnread =>
      berandaUnread + kasUnread + lokasiDanaUnread + notifikasiUnread;
  bool get hasAnyUnread => totalUnread > 0;
}

final rwTabUnreadStateProvider = Provider.autoDispose
    .family<RwTabUnreadState, String>((ref, rwId) {
  final notifikasiUnread =
      ref.watch(rwUnreadCountProvider(rwId)).valueOrNull ?? 0;
  return RwTabUnreadState(
    notifikasiUnread: notifikasiUnread > 0 ? notifikasiUnread : 0,
  );
});

/// Shared eligibility check for showing DATA RW unread indicators across bottom nav,
/// DATA RW tabs, and RW Notification subtabs. Mirrors existing MainNavigationShell rules.
bool isRwUnreadBadgeEligible(
  Map<String, dynamic>? user, {
  required List<RWMembership> rwMemberships,
  String? rwId,
}) {
  if (user == null) return false;
  final jabatan = (user['jabatan'] ?? '').toString().trim().toLowerCase();
  if (jabatan == 'keamanan') return false;

  final role = user['role'];
  final sysRole = (user['system_role'] ?? '').toString().trim().toLowerCase();
  if (role == AppRole.superAdmin ||
      role?.toString().trim().toLowerCase() == 'super_admin' ||
      sysRole == 'super_admin') {
    return true;
  }

  final relevantMemberships = (rwId != null && rwId.isNotEmpty)
      ? rwMemberships.where((m) => m.rwId == rwId)
      : rwMemberships;
  final hasExecRole = relevantMemberships.any(
    (m) =>
        m.source != 'rt_inferred' &&
        (m.jabatan == 'ketua' || m.jabatan == 'bendahara'),
  );
  return hasExecRole || userHasPermission(user, 'rw.notification.write');
}

/// Extracts all RW IDs associated with the user and invalidates their unread count providers
/// so bottom navigation and DATA RW tabs never display stale badges after reading notifications.
void invalidateUserRwUnreadCounts(
  WidgetRef ref,
  Map<String, dynamic>? user, {
  String? activeRwId,
}) {
  final rwIds = <String>{};
  if (activeRwId != null && activeRwId.trim().isNotEmpty) {
    rwIds.add(activeRwId.trim());
  }
  final rawRwId = (user?['rw_id'] ?? '').toString().trim();
  if (rawRwId.isNotEmpty) {
    rwIds.add(rawRwId);
  }
  final rawMemberships = user?['rw_memberships'];
  if (rawMemberships is List) {
    for (final entry in rawMemberships) {
      if (entry is Map) {
        final id = (entry['rw_id'] ?? '').toString().trim();
        if (id.isNotEmpty) rwIds.add(id);
      }
    }
  }
  for (final id in rwIds) {
    ref.invalidate(rwUnreadCountProvider(id));
  }
}

final securityAlertsDataProvider =
    FutureProvider.autoDispose<SecurityAlertsData>((ref) async {
  ref.watch(authRepositoryProvider);
  final client = ref.watch(apiClientProvider);
  try {
    final res = await client.dio.get('/api/v1/security/alerts');
    final data = res.data;
    if (data is Map && data['data'] is Map) {
      return SecurityAlertsData.fromJson(
        (data['data'] as Map).cast<String, dynamic>(),
      );
    }
    return const SecurityAlertsData();
  } catch (_) {
    return const SecurityAlertsData();
  }
});

final securityAlertsProvider = FutureProvider.autoDispose<bool>((ref) async {
  final data = await ref.watch(securityAlertsDataProvider.future);
  return data.hasAlert;
});
