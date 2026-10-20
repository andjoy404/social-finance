/// Iuran Riverpod providers — data fetching, filtering, and detail.
///
/// Mirrors the Warga pattern:
/// - AsyncNotifierProvider.autoDispose for data fetching (auto-refreshes when dependencies change)
/// - StateProvider for filter/search state
/// - Provider for derived/computed state (filtering composition)
/// - FutureProvider.family for parameterized lookups

import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/features/iuran/data/iuran_api_models.dart';
import 'package:social_finance/features/iuran/data/iuran_models.dart';
import 'package:social_finance/features/iuran/data/iuran_repository.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';

/// Check whether the current user can verify payments.
/// Returns true for pengurus, bendahara, and super_admin.
///
/// The role value from [authRepositoryProvider] is always [AppRole]
/// (mapped via [mapStringToAppRole] in [AuthRepository]), but we
/// defensively handle the raw-string case too for cached/replay states.
final canVerifyPaymentProvider = Provider<bool>((ref) {
  final role = ref.read(authRepositoryProvider);
  final roleValue = role?.valueOrNull?['role'];
  if (roleValue == null) return false;

  // Primary path: roleValue is AppRole enum (normal runtime)
  if (roleValue is AppRole) {
    return switch (roleValue) {
      AppRole.pengurus || AppRole.bendahara || AppRole.superAdmin => true,
      _ => false,
    };
  }

  // Fallback: raw string from cached state or unexpected serialization
  if (roleValue is String) {
    return roleValue == 'pengurus' ||
        roleValue == 'bendahara' ||
        roleValue == 'super_admin';
  }

  return false;
});

// ─── Filter/Search State ──────────────────────────────────────────────

/// Current search query.
final iuranSearchQueryProvider = StateProvider<String>((ref) => '');

/// Selected periode filter (e.g. 'Oktober 2026').
final iuranSelectedPeriodeProvider = StateProvider<String>((ref) => '');

/// Selected status filter (enum name: 'belumBayar', 'sebagian', 'lunas').
final iuranSelectedStatusProvider = StateProvider<String>((ref) => '');

// ─── Pending Payments (for verification) ────────────────────────────

/// Loads all PENDING payments for verification by bendahara/pengurus.
final pendingPaymentsProvider = FutureProvider.autoDispose
    .family<List<BackendPayment>, bool>((ref, canVerify) async {
      if (!canVerify) return [];

      final repository = ref.read(iuranRepositoryProvider);

      try {
        final result = await repository.fetchPendingPayments();
        return result;
      } catch (e) {
        return [];
      }
    });

// ─── Bills (list with computed paidAmount) ────────────────────────────

final billsProvider =
    AsyncNotifierProvider.autoDispose<BillsNotifier, List<IuranBill>>(
      () => BillsNotifier(),
    );

/// Loads bills limited by the backend to the authenticated user's household.
final myBillsProvider =
    AsyncNotifierProvider.autoDispose<BillsNotifier, List<IuranBill>>(
      () => BillsNotifier(myBills: true),
    );

class BillsNotifier extends AutoDisposeAsyncNotifier<List<IuranBill>> {
  BillsNotifier({this.myBills = false});

  final bool myBills;
  String? _lastToken;

  @override
  Future<List<IuranBill>> build() async {
    ref.watch(authRepositoryProvider);
    ref.watch(iuranRepositoryProvider);
    final accessToken = ref.read(authRepositoryProvider.notifier).accessToken;

    if (accessToken == null || accessToken.isEmpty) {
      throw StateError('Sesi pengguna tidak tersedia.');
    }

    if (accessToken == _lastToken) {
      return _fetch(propagateError: myBills);
    }
    _lastToken = accessToken;
    return _fetch(propagateError: true);
  }

  Future<List<IuranBill>> _fetch({bool propagateError = false}) async {
    final repository = ref.read(iuranRepositoryProvider);

    state = const AsyncLoading();

    try {
      final result = await repository.fetchAllBills(myBills: myBills);
      debugPrint('[IuranDebug] billsProvider got=${result.length} bills');
      state = AsyncData(result);
      return result;
    } catch (e, st) {
      debugPrint('[IuranDebug] billsProvider error=$e');
      final error = repository.mapDioError(e);
      state = AsyncError(error, st);
      if (propagateError) Error.throwWithStackTrace(error, st);
      return [];
    }
  }

  Future<void> refresh() async {
    await _fetch();
  }
}

/// Filtered bills derived from billsProvider + search/periode/status,
/// sorted oldest to newest by period.
final filteredBillsProvider = Provider<List<IuranBill>>((ref) {
  final state = ref.watch(billsProvider);
  return _filterBills(
    state,
    query: ref.watch(iuranSearchQueryProvider),
    periode: ref.watch(iuranSelectedPeriodeProvider),
    status: ref.watch(iuranSelectedStatusProvider),
  );
});

/// Filtered bills for the user-scoped Tagihan tab.
final filteredMyBillsProvider = Provider<List<IuranBill>>((ref) {
  final state = ref.watch(myBillsProvider);
  return _filterBills(
    state,
    query: ref.watch(iuranSearchQueryProvider),
    periode: ref.watch(iuranSelectedPeriodeProvider),
    status: ref.watch(iuranSelectedStatusProvider),
  );
});

List<IuranBill> _filterBills(
  AsyncValue<List<IuranBill>> state, {
  required String query,
  required String periode,
  required String status,
}) {
  final normalizedQuery = query.trim().toLowerCase();

  final bills = switch (state) {
    AsyncData(:final value) => value,
    AsyncLoading() => <IuranBill>[],
    AsyncError() => <IuranBill>[],
    _ => <IuranBill>[],
  };

  var filtered = bills.where((b) {
    final searchMatch =
        normalizedQuery.isEmpty ||
        b.householdName.toLowerCase().contains(normalizedQuery) ||
        b.iuranType.toLowerCase().contains(normalizedQuery);
    final periodeMatch = periode.isEmpty || b.periode == periode;
    final statusMatch = status.isEmpty || b.status.name == status;
    return searchMatch && periodeMatch && statusMatch;
  }).toList();

  debugPrint(
    '[IuranDebug] filteredBills input=${bills.length} -> output=${filtered.length} (query="$normalizedQuery", periode="$periode", status="$status")',
  );

  // Sort: unpaid/partial first (oldest→newest), then paid (newest→oldest)
  filtered.sort((a, b) {
    final aPaid = a.status == IuranStatus.lunas;
    final bPaid = b.status == IuranStatus.lunas;
    if (aPaid != bPaid) {
      // Unpaid/partial first: a is not paid, b is paid → a comes first (-1)
      return aPaid ? 1 : -1;
    }
    final aParsed = _parsePeriodForSort(a.periode);
    final bParsed = _parsePeriodForSort(b.periode);
    final yearCmp = aParsed[0].compareTo(bParsed[0]);
    if (yearCmp != 0) return aPaid ? bParsed[0].compareTo(aParsed[0]) : yearCmp;
    return aPaid
        ? bParsed[1].compareTo(aParsed[1])
        : yearCmp != 0
        ? yearCmp
        : aParsed[1].compareTo(bParsed[1]);
  });

  return filtered;
}

/// Sort helper: parse "Oktober 2026" → [2026, 10] for chronological ordering.
List<int> _parsePeriodForSort(String period) {
  final parts = period.split(' ');
  if (parts.length < 2) return [0, 0];
  final year = int.tryParse(parts[1]) ?? 0;
  const months = {
    'januari': 1,
    'februari': 2,
    'maret': 3,
    'april': 4,
    'mei': 5,
    'juni': 6,
    'juli': 7,
    'agustus': 8,
    'september': 9,
    'oktober': 10,
    'november': 11,
    'desember': 12,
  };
  final month = months[parts[0].toLowerCase()] ?? 0;
  return [year, month];
}

// ─── Bill Detail (parameterized by ID) ────────────────────────────────

final billDetailProvider = FutureProvider.family
    .autoDispose<IuranBill?, String>((ref, billId) {
      final repository = ref.read(iuranRepositoryProvider);
      return repository.fetchBillDetail(billId);
    });

/// List of all payments for a specific bill (for history and proof inspection).
final billPaymentsProvider = FutureProvider.family
    .autoDispose<List<BackendPayment>, String>((ref, billId) {
      final repository = ref.read(iuranRepositoryProvider);
      return repository.listPaymentsForBill(billId);
    });

// ─── Dues List ────────────────────────────────────────────────────────

final duesProvider =
    AsyncNotifierProvider.autoDispose<DuesNotifier, List<BackendDue>>(
      () => DuesNotifier(),
    );

class DuesNotifier extends AutoDisposeAsyncNotifier<List<BackendDue>> {
  @override
  Future<List<BackendDue>> build() async {
    return _fetch();
  }

  Future<List<BackendDue>> _fetch() async {
    final repository = ref.read(iuranRepositoryProvider);

    state = const AsyncLoading();

    try {
      final result = await repository.fetchDues();
      state = AsyncData(result);
      return result;
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

// ─── Balance / Report ────────────────────────────────────────────────

final balanceProvider =
    AsyncNotifierProvider.autoDispose<BalanceNotifier, BackendBalance>(
      () => BalanceNotifier(),
    );

class BalanceNotifier extends AutoDisposeAsyncNotifier<BackendBalance> {
  @override
  Future<BackendBalance> build() async {
    return _fetch();
  }

  Future<BackendBalance> _fetch() async {
    final repository = ref.read(iuranRepositoryProvider);

    state = const AsyncLoading();

    try {
      final result = await repository.fetchBalance();
      state = AsyncData(result);
      return result;
    } catch (e, st) {
      final error = repository.mapDioError(e);
      state = AsyncError(error, st);
      return const BackendBalance(
        totalIncome: '0',
        totalExpense: '0',
        netBalance: '0',
        transactionCount: 0,
      );
    }
  }
}
