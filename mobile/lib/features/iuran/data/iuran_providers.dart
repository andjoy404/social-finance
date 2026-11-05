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
import 'package:social_finance/features/iuran/data/iuran_access.dart';
import 'package:social_finance/features/iuran/data/iuran_api_models.dart';
import 'package:social_finance/features/iuran/data/iuran_models.dart';
import 'package:social_finance/features/iuran/data/iuran_repository.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';

const Set<String> _managementJabatans = {
  'ketua',
  'ketua_rt',
  'ketua rt',
  'wakil_ketua',
  'wakil ketua',
  'sekretaris',
  'bendahara',
};

/// Returns true only when [user] is an ordinary RT resident (`warga`)
/// and NOT a `super_admin`, `pengurus`, `bendahara`, or RT/RW officer.
bool isPlainWargaUser(Map<String, dynamic>? user) {
  if (user == null) return false;
  final systemRole = (user['system_role'] ?? '').toString().trim().toLowerCase();
  if (systemRole == 'super_admin') return false;

  final rawRole = user['role'];
  final bool isWargaRole;
  if (rawRole is AppRole) {
    isWargaRole = rawRole == AppRole.warga;
  } else {
    isWargaRole = rawRole?.toString().trim().toLowerCase() == 'warga';
  }
  if (!isWargaRole) return false;

  final jabatan = (user['jabatan'] ?? '').toString().trim().toLowerCase();
  if (jabatan.isNotEmpty && _managementJabatans.contains(jabatan)) {
    return false;
  }

  final rawRwMemberships = user['rw_memberships'];
  if (rawRwMemberships is List) {
    for (final item in rawRwMemberships) {
      if (item is Map) {
        final rwJabatan =
            (item['jabatan'] ?? '').toString().trim().toLowerCase();
        if (rwJabatan.isNotEmpty && _managementJabatans.contains(rwJabatan)) {
          return false;
        }
      }
    }
  }

  return true;
}

/// Check whether the current user can verify payments.
///
/// Uses [canVerifyPayment] which verifies role/jabatan authorization
/// (super_admin, bendahara, pengurus, or jabatan ketua/bendahara)
/// and strictly requires the `finance.approve` permission.
final canVerifyPaymentProvider = Provider<bool>((ref) {
  final user = ref.watch(authRepositoryProvider)?.valueOrNull;
  return canVerifyPayment(user);
});

// ─── Filter/Search State ──────────────────────────────────────────────

/// Current search query.
final iuranSearchQueryProvider = StateProvider<String>((ref) => '');

/// Format current local calendar month & year ("Oktober 2026").
String currentCalendarPeriod([DateTime? now]) {
  final dt = now ?? DateTime.now();
  const months = [
    'Januari',
    'Februari',
    'Maret',
    'April',
    'Mei',
    'Juni',
    'Juli',
    'Agustus',
    'September',
    'Oktober',
    'November',
    'Desember',
  ];
  return '${months[dt.month - 1]} ${dt.year}';
}

/// Selected periode filter for Iuran Warga (default in IuranScreen: current month 'YYYY-MM').
final iuranSelectedPeriodeProvider = StateProvider<String>((ref) => '');

/// Selected periode filter for Iuran Saya (default in IuranScreen: current year 'YYYY').
final iuranSayaSelectedPeriodeProvider = StateProvider<String>((ref) => '');

/// Selected status filter (enum name: 'belumBayar', 'sebagian', 'lunas').
final iuranSelectedStatusProvider = StateProvider<String>((ref) => '');

/// Selected payment status filter ('', 'pending', 'approved', 'rejected').
final iuranSelectedPaymentStatusProvider = StateProvider<String>((ref) => '');

/// Filter expanded state per tab index (0=Iuran Saya, 1=Iuran Warga).
final iuranFilterExpandedProvider = StateProvider.family<bool, int>((ref, index) => false);

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

@immutable
class PaginatedBillsState {
  final List<IuranBill> bills;
  final int page;
  final int pageSize;
  final int total;
  final int totalPages;
  final bool hasMore;
  final bool isLoadingMore;
  final String? loadMoreError;

  const PaginatedBillsState({
    this.bills = const [],
    this.page = 1,
    this.pageSize = 30,
    this.total = 0,
    this.totalPages = 0,
    this.hasMore = false,
    this.isLoadingMore = false,
    this.loadMoreError,
  });

  PaginatedBillsState copyWith({
    List<IuranBill>? bills,
    int? page,
    int? pageSize,
    int? total,
    int? totalPages,
    bool? hasMore,
    bool? isLoadingMore,
    String? Function()? loadMoreError,
  }) {
    return PaginatedBillsState(
      bills: bills ?? this.bills,
      page: page ?? this.page,
      pageSize: pageSize ?? this.pageSize,
      total: total ?? this.total,
      totalPages: totalPages ?? this.totalPages,
      hasMore: hasMore ?? this.hasMore,
      isLoadingMore: isLoadingMore ?? this.isLoadingMore,
      loadMoreError: loadMoreError != null ? loadMoreError() : this.loadMoreError,
    );
  }
}

final paginatedBillsProvider =
    StateProvider<PaginatedBillsState>((ref) => const PaginatedBillsState());

String? _convertPeriodToBackend(String raw) {
  final trimmed = raw.trim();
  if (trimmed.isEmpty) return null;
  if (RegExp(r'^\d{4}-\d{2}$').hasMatch(trimmed) || RegExp(r'^\d{4}$').hasMatch(trimmed)) {
    return trimmed;
  }
  final parts = trimmed.split(' ');
  if (parts.length == 2) {
    const months = {
      'januari': '01',
      'februari': '02',
      'maret': '03',
      'april': '04',
      'mei': '05',
      'juni': '06',
      'juli': '07',
      'agustus': '08',
      'september': '09',
      'oktober': '10',
      'november': '11',
      'desember': '12',
    };
    final m = months[parts[0].toLowerCase()];
    final y = int.tryParse(parts[1]);
    if (m != null && y != null) {
      return '$y-$m';
    }
  }
  return trimmed;
}

String? _convertStatusToBackend(String raw) {
  final trimmed = raw.trim();
  if (trimmed.isEmpty) return null;
  switch (trimmed.toLowerCase()) {
    case 'belumbayar':
    case 'unpaid':
      return 'unpaid';
    case 'sebagian':
    case 'partial':
      return 'partial';
    case 'lunas':
    case 'paid':
      return 'paid';
    default:
      return trimmed;
  }
}

String? _convertPaymentStatusToBackend(String raw) {
  final trimmed = raw.trim();
  if (trimmed.isEmpty) return null;
  switch (trimmed.toLowerCase()) {
    case 'pending':
      return 'PENDING';
    case 'approved':
      return 'APPROVED';
    case 'rejected':
      return 'REJECTED';
    default:
      return trimmed.toUpperCase();
  }
}

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
    final authUser = ref.read(authRepositoryProvider)?.valueOrNull;

    if ((accessToken == null || accessToken.isEmpty) && authUser == null) {
      throw StateError('Sesi pengguna tidak tersedia.');
    }

    if (myBills) {
      if (accessToken == _lastToken) {
        return _fetchMyBills(propagateError: false);
      }
      _lastToken = accessToken;
      return _fetchMyBills(propagateError: true);
    }

    _lastToken = accessToken;
    final search = ref.watch(iuranSearchQueryProvider);
    final rawPeriod = ref.watch(iuranSelectedPeriodeProvider);
    final rawStatus = ref.watch(iuranSelectedStatusProvider);
    final rawPaymentStatus = ref.watch(iuranSelectedPaymentStatusProvider);

    return _fetchWargaBills(
      search: search,
      rawPeriod: rawPeriod,
      rawStatus: rawStatus,
      rawPaymentStatus: rawPaymentStatus,
    );
  }

  Future<List<IuranBill>> _fetchMyBills({bool propagateError = false}) async {
    final repository = ref.read(iuranRepositoryProvider);
    state = const AsyncLoading();

    try {
      final result = await repository.fetchAllBills(myBills: true);
      debugPrint('[IuranDebug] myBillsProvider got=${result.length} bills');
      state = AsyncData(result);
      Future.microtask(() {
        ref.read(overdueWarningSummaryProvider.notifier).refresh();
      });
      return result;
    } catch (e, st) {
      debugPrint('[IuranDebug] myBillsProvider error=$e');
      final error = repository.mapDioError(e);
      state = AsyncError(error, st);
      if (propagateError) Error.throwWithStackTrace(error, st);
      return [];
    }
  }

  Future<List<IuranBill>> _fetchWargaBills({
    required String search,
    required String rawPeriod,
    required String rawStatus,
    required String rawPaymentStatus,
  }) async {
    final repository = ref.read(iuranRepositoryProvider);
    state = const AsyncLoading();

    final backendSearch = search.trim().isEmpty ? null : search.trim();
    final backendPeriod = _convertPeriodToBackend(rawPeriod);
    final backendStatus = _convertStatusToBackend(rawStatus);
    final backendPaymentStatus = _convertPaymentStatusToBackend(rawPaymentStatus);

    try {
      final response = await repository.fetchBills(
        search: backendSearch,
        period: backendPeriod,
        status: backendStatus,
        paymentStatus: backendPaymentStatus,
        myBills: false,
        page: 1,
        pageSize: 30,
        order: 'unpaid_first',
      );

      final hasMore = response.pagination.page < response.pagination.totalPages;
      Future.microtask(() {
        ref.read(paginatedBillsProvider.notifier).state = PaginatedBillsState(
          bills: response.data,
          page: response.pagination.page,
          pageSize: response.pagination.pageSize,
          total: response.pagination.total,
          totalPages: response.pagination.totalPages,
          hasMore: hasMore,
          isLoadingMore: false,
          loadMoreError: null,
        );
      });

      debugPrint('[IuranDebug] billsProvider got=${response.data.length} bills, total=${response.pagination.total}');
      state = AsyncData(response.data);
      return response.data;
    } catch (e, st) {
      debugPrint('[IuranDebug] billsProvider error=$e');
      final error = repository.mapDioError(e);
      Future.microtask(() {
        ref.read(paginatedBillsProvider.notifier).state = PaginatedBillsState(
          bills: const [],
          page: 1,
          pageSize: 30,
          total: 0,
          totalPages: 0,
          hasMore: false,
          isLoadingMore: false,
          loadMoreError: error.toString(),
        );
      });
      state = AsyncError(error, st);
      return [];
    }
  }

  Future<void> loadMore() async {
    if (myBills) return;
    final current = ref.read(paginatedBillsProvider);
    if (!current.hasMore || current.isLoadingMore) return;

    ref.read(paginatedBillsProvider.notifier).state = current.copyWith(
      isLoadingMore: true,
      loadMoreError: () => null,
    );

    final repository = ref.read(iuranRepositoryProvider);
    final search = ref.read(iuranSearchQueryProvider);
    final rawPeriod = ref.read(iuranSelectedPeriodeProvider);
    final rawStatus = ref.read(iuranSelectedStatusProvider);
    final rawPaymentStatus = ref.read(iuranSelectedPaymentStatusProvider);

    final backendSearch = search.trim().isEmpty ? null : search.trim();
    final backendPeriod = _convertPeriodToBackend(rawPeriod);
    final backendStatus = _convertStatusToBackend(rawStatus);
    final backendPaymentStatus = _convertPaymentStatusToBackend(rawPaymentStatus);

    try {
      final nextPage = current.page + 1;
      final response = await repository.fetchBills(
        search: backendSearch,
        period: backendPeriod,
        status: backendStatus,
        paymentStatus: backendPaymentStatus,
        myBills: false,
        page: nextPage,
        pageSize: current.pageSize,
        order: 'unpaid_first',
      );

      final existingIds = current.bills.map((b) => b.id).toSet();
      final newBills = response.data.where((b) => !existingIds.contains(b.id)).toList();
      final combined = [...current.bills, ...newBills];
      final hasMore = response.pagination.page < response.pagination.totalPages;

      ref.read(paginatedBillsProvider.notifier).state = current.copyWith(
        bills: combined,
        page: response.pagination.page,
        total: response.pagination.total,
        totalPages: response.pagination.totalPages,
        hasMore: hasMore,
        isLoadingMore: false,
        loadMoreError: () => null,
      );

      state = AsyncData(combined);
    } catch (e) {
      ref.read(paginatedBillsProvider.notifier).state = current.copyWith(
        isLoadingMore: false,
        loadMoreError: () => e.toString(),
      );
    }
  }

  Future<void> retryLoadMore() async {
    await loadMore();
  }

  Future<void> refresh() async {
    if (myBills) {
      await _fetchMyBills(propagateError: true);
    } else {
      final search = ref.read(iuranSearchQueryProvider);
      final rawPeriod = ref.read(iuranSelectedPeriodeProvider);
      final rawStatus = ref.read(iuranSelectedStatusProvider);
      final rawPaymentStatus = ref.read(iuranSelectedPaymentStatusProvider);
      await _fetchWargaBills(
        search: search,
        rawPeriod: rawPeriod,
        rawStatus: rawStatus,
        rawPaymentStatus: rawPaymentStatus,
      );
    }
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
    paymentStatus: ref.watch(iuranSelectedPaymentStatusProvider),
  );
});

/// Filtered bills for the user-scoped Tagihan tab.
final filteredMyBillsProvider = Provider<List<IuranBill>>((ref) {
  final state = ref.watch(myBillsProvider);
  return _filterBills(
    state,
    query: ref.watch(iuranSearchQueryProvider),
    periode: ref.watch(iuranSayaSelectedPeriodeProvider),
    status: ref.watch(iuranSelectedStatusProvider),
    paymentStatus: ref.watch(iuranSelectedPaymentStatusProvider),
  );
});

List<IuranBill> _filterBills(
  AsyncValue<List<IuranBill>> state, {
  required String query,
  required String periode,
  required String status,
  String paymentStatus = '',
}) {
  final normalizedQuery = query.trim().toLowerCase();

  final bills = switch (state) {
    AsyncData(:final value) => value,
    AsyncLoading() => <IuranBill>[],
    AsyncError() => <IuranBill>[],
    _ => <IuranBill>[],
  };

  // Normalize periode filter: "2026-10" → [2026, 10], "2026" → [2026, null], empty → null
  int? filterYear, filterMonth;
  if (periode.isNotEmpty) {
    final parts = periode.split('-');
    if (parts.length == 2) {
      filterYear = int.tryParse(parts[0]);
      filterMonth = int.tryParse(parts[1]);
    } else if (parts.length == 1) {
      filterYear = int.tryParse(parts[0]);
    }
  }

  var filtered = bills.where((b) {
    final searchMatch =
        normalizedQuery.isEmpty ||
        b.householdName.toLowerCase().contains(normalizedQuery) ||
        b.iuranType.toLowerCase().contains(normalizedQuery) ||
        b.paymentNumbers.any((no) => no.toLowerCase().contains(normalizedQuery));

    // Period filter: match "Oktober 2026", "2026-10", or year-only "2026"
    final periodMatch =
        periode.isEmpty ||
        b.periode == periode ||
        (filterYear != null &&
            filterMonth != null &&
            _periodMatches(b.periode, filterYear, filterMonth)) ||
        (filterYear != null &&
            filterMonth == null &&
            _yearMatches(b.periode, filterYear));

    final statusMatch = status.isEmpty || b.status.name == status;
    final paymentStatusMatch = switch (paymentStatus.toLowerCase()) {
      'pending' => b.pendingAmount > 0,
      'approved' => b.paidAmount > 0,
      'rejected' => b.rejectedAmount > 0,
      _ => true,
    };
    return searchMatch && periodMatch && statusMatch && paymentStatusMatch;
  }).toList();

  debugPrint(
    '[IuranDebug] filteredBills input=${bills.length} -> output=${filtered.length} (query="$normalizedQuery", periode="$periode", status="$status")',
  );

  // Sort: pending (Tertunda) first, then unpaid/partial (oldest→newest), then paid (newest→oldest)
  filtered.sort((a, b) {
    final aPending = a.pendingAmount > 0;
    final bPending = b.pendingAmount > 0;
    if (aPending != bPending) {
      return aPending ? -1 : 1;
    }
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

/// Check if a period string (e.g. "Oktober 2026") matches the given year & month.
bool _periodMatches(String period, int year, int month) {
  final parts = period.split(' ');
  if (parts.length < 2) return false;
  final yearStr = parts[1];
  const monthNames = {
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
  final pYear = int.tryParse(yearStr);
  final pMonth = monthNames[parts[0].toLowerCase()];
  return pYear == year && pMonth == month;
}

/// Check if a period string (e.g. "Oktober 2026" or "2026") matches the given year.
bool _yearMatches(String period, int year) {
  if (period.contains(year.toString())) return true;
  final parts = period.split(' ');
  if (parts.length >= 2) {
    final pYear = int.tryParse(parts[1]);
    return pYear == year;
  }
  return false;
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

// ─── Overdue Warning Summary (Global Warning Card) ────────────────────

/// Non-autoDispose provider so switching Bottom Navigation tabs does not
/// re-trigger network calls. Refreshed on login/user change, app resume,
/// or after iuran/payment data refreshes.
final overdueWarningSummaryProvider =
    AsyncNotifierProvider<OverdueWarningSummaryNotifier, OverdueWarningSummary>(
      OverdueWarningSummaryNotifier.new,
    );

class OverdueWarningSummaryNotifier
    extends AsyncNotifier<OverdueWarningSummary> {
  DateTime? _lastFetchedAt;
  static const Duration _resumeThrottle = Duration(seconds: 2);

  @override
  Future<OverdueWarningSummary> build() async {
    final userKey = ref.watch(
      authRepositoryProvider.select((authAsync) {
        final u = authAsync?.valueOrNull;
        if (!isPlainWargaUser(u)) return '';
        final id = (u?['id'] ?? '').toString();
        final rtId = (u?['rt_id'] ?? u?['rt']?['id'] ?? '').toString();
        return '$id:$rtId';
      }),
    );

    if (userKey.isEmpty) {
      return OverdueWarningSummary.empty;
    }

    return _fetch();
  }

  Future<OverdueWarningSummary> _fetch() async {
    final user = ref.read(authRepositoryProvider)?.valueOrNull;
    if (!isPlainWargaUser(user)) {
      state = const AsyncData(OverdueWarningSummary.empty);
      return OverdueWarningSummary.empty;
    }

    final repository = ref.read(iuranRepositoryProvider);
    _lastFetchedAt = DateTime.now();

    try {
      final summary = await repository.fetchOverdueWarningSummary();
      state = AsyncData(summary);
      return summary;
    } catch (_) {
      // Never surface an error state that could break global layout;
      // preserve previous data or fall back to empty.
      final previous = state.valueOrNull ?? OverdueWarningSummary.empty;
      state = AsyncData(previous);
      return previous;
    }
  }

  Future<void> refresh({bool throttled = false}) async {
    final user = ref.read(authRepositoryProvider)?.valueOrNull;
    if (!isPlainWargaUser(user)) {
      state = const AsyncData(OverdueWarningSummary.empty);
      return;
    }
    if (throttled && _lastFetchedAt != null) {
      final elapsed = DateTime.now().difference(_lastFetchedAt!);
      if (elapsed < _resumeThrottle) return;
    }
    await _fetch();
  }
}
