/// Iuran Riverpod providers — data fetching, filtering, and detail.
///
/// Mirrors the Warga pattern:
/// - AsyncNotifierProvider.autoDispose for data fetching (auto-refreshes when dependencies change)
/// - StateProvider for filter/search state
/// - Provider for derived/computed state (filtering composition)
/// - FutureProvider.family for parameterized lookups

import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:social_finance/features/iuran/data/iuran_api_models.dart';
import 'package:social_finance/features/iuran/data/iuran_models.dart';
import 'package:social_finance/features/iuran/data/iuran_repository.dart';

// ─── Filter/Search State ──────────────────────────────────────────────

/// Current search query.
final iuranSearchQueryProvider = StateProvider<String>((ref) => '');

/// Selected periode filter (e.g. 'Oktober 2026').
final iuranSelectedPeriodeProvider = StateProvider<String>((ref) => '');

/// Selected status filter (enum name: 'belumBayar', 'sebagian', 'lunas').
final iuranSelectedStatusProvider = StateProvider<String>((ref) => '');

// ─── Bills (list with computed paidAmount) ────────────────────────────

final billsProvider =
    AsyncNotifierProvider.autoDispose<BillsNotifier, List<IuranBill>>(
      () => BillsNotifier(),
    );

class BillsNotifier extends AutoDisposeAsyncNotifier<List<IuranBill>> {
  @override
  Future<List<IuranBill>> build() async {
    return _fetch();
  }

  Future<List<IuranBill>> _fetch() async {
    final repository = ref.read(iuranRepositoryProvider);

    state = const AsyncLoading();

    try {
      final result = await repository.fetchAllBills();
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

/// Filtered bills derived from billsProvider + search/periode/status.
final filteredBillsProvider = Provider<List<IuranBill>>((ref) {
  final state = ref.watch(billsProvider);
  final query = ref.watch(iuranSearchQueryProvider).trim().toLowerCase();
  final periode = ref.watch(iuranSelectedPeriodeProvider);
  final status = ref.watch(iuranSelectedStatusProvider);

  final bills = switch (state) {
    AsyncData(:final value) => value,
    AsyncLoading() => <IuranBill>[],
    AsyncError() => <IuranBill>[],
    _ => <IuranBill>[],
  };

  if (query.isEmpty && periode.isEmpty && status.isEmpty) {
    return bills;
  }

  return bills.where((b) {
    final searchMatch = query.isEmpty ||
        b.householdName.toLowerCase().contains(query) ||
        b.iuranType.toLowerCase().contains(query);
    final periodeMatch = periode.isEmpty || b.periode == periode;
    final statusMatch = status.isEmpty || b.status.name == status;
    return searchMatch && periodeMatch && statusMatch;
  }).toList();
});

// ─── Bill Detail (parameterized by ID) ────────────────────────────────

final billDetailProvider =
    FutureProvider.family.autoDispose<IuranBill?, String>((ref, billId) {
  final repository = ref.read(iuranRepositoryProvider);
  return repository.fetchBillDetail(billId);
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
