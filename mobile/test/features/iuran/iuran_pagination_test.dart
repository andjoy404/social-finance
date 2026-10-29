import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/api/device_id_service.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/iuran/data/iuran_api_client.dart';
import 'package:social_finance/features/iuran/data/iuran_api_models.dart';
import 'package:social_finance/features/iuran/data/iuran_providers.dart';
import 'package:social_finance/features/iuran/data/iuran_repository.dart';

class _FakeIuranApiClient extends Fake implements IuranApiClient {
  final List<Map<String, dynamic>> calls = [];
  int listPaymentsCallCount = 0;
  bool failNextPage = false;

  @override
  Future<PaginatedResponse<BackendBill>> listBills({
    String? search,
    String? order,
    String? dueId,
    String? status,
    String? paymentStatus,
    String? period,
    String? householdOccupancyId,
    bool? isActive,
    bool myBills = false,
    int page = 1,
    int pageSize = 20,
  }) async {
    calls.add({
      'search': search,
      'order': order,
      'dueId': dueId,
      'status': status,
      'paymentStatus': paymentStatus,
      'period': period,
      'myBills': myBills,
      'page': page,
      'pageSize': pageSize,
    });

    if (failNextPage && page > 1) {
      failNextPage = false;
      throw Exception('Network connection timed out');
    }

    final items = <BackendBill>[];
    final startIndex = (page - 1) * pageSize;
    const total = 60;
    const count = 30;

    for (var i = 0; i < count; i++) {
      final idx = startIndex + i + 1;
      items.add(
        BackendBill(
          id: 'bill-$idx',
          rtId: 'rt-1',
          householdOccupancyId: 'occ-1',
          dueId: 'due-1',
          amount: '50000.00',
          paidAmount: '20000.00',
          pendingAmount: '0.00',
          period: '2026-10',
          dueDate: '2026-10-25',
          status: BillStatus.partial,
          isActive: true,
          dueName: 'Iuran Warga RT',
          headName: 'Warga $idx',
          isVoluntary: false,
        ),
      );
    }

    return PaginatedResponse(
      data: items,
      pagination: PaginationInfo(
        page: page,
        pageSize: pageSize,
        total: total,
        totalPages: 2,
      ),
    );
  }

  @override
  Future<PaginatedResponse<BackendPayment>> listPayments({
    String? billId,
    String? status,
    String? dueId,
    String? householdOccupancyId,
    String? method,
    String? period,
    int page = 1,
    int pageSize = 20,
  }) async {
    listPaymentsCallCount++;
    return const PaginatedResponse(
      data: [],
      pagination: PaginationInfo(page: 1, pageSize: 20, total: 0, totalPages: 0),
    );
  }
}

AuthRepository _createTestAuthRepository() {
  final client = ApiClient(customDio: Dio());
  client.setToken('test-access-token');
  final authRepo = AuthRepository(
    authService: AuthService(client, DeviceIdService()),
    apiClient: client,
  );
  authRepo.state = const AsyncData({'id': 'u1', 'role': 'pengurus'});
  return authRepo;
}

void main() {
  late _FakeIuranApiClient fakeApiClient;
  late IuranRepository repository;

  setUp(() {
    fakeApiClient = _FakeIuranApiClient();
    repository = IuranRepository(apiClient: fakeApiClient);
  });

  group('Iuran Server-Side Pagination Tests', () {
    test('Initial load fetches page 1 with pageSize 30 and unpaid_first order', () async {
      final authRepo = _createTestAuthRepository();
      final container = ProviderContainer(
        overrides: [
          iuranRepositoryProvider.overrideWithValue(repository),
          authRepositoryProvider.overrideWith((ref) => authRepo),
        ],
      );

      final state = await container.read(billsProvider.future);

      expect(state, hasLength(30));
      expect(fakeApiClient.calls, hasLength(1));
      expect(fakeApiClient.calls.first['page'], 1);
      expect(fakeApiClient.calls.first['pageSize'], 30);
      expect(fakeApiClient.calls.first['order'], 'unpaid_first');

      // Verify N+1 queries eliminated: listPayments was NEVER called during fetchBills
      expect(fakeApiClient.listPaymentsCallCount, 0);

      final paginated = container.read(paginatedBillsProvider);
      expect(paginated.page, 1);
      expect(paginated.pageSize, 30);
      expect(paginated.total, 60);
      expect(paginated.totalPages, 2);
      expect(paginated.hasMore, isTrue);
      expect(paginated.isLoadingMore, isFalse);
      expect(paginated.loadMoreError, isNull);
    });

    test('loadMore() fetches page 2, deduplicates items, and updates pagination state', () async {
      final authRepo = _createTestAuthRepository();
      final container = ProviderContainer(
        overrides: [
          iuranRepositoryProvider.overrideWithValue(repository),
          authRepositoryProvider.overrideWith((ref) => authRepo),
        ],
      );

      await container.read(billsProvider.future);
      expect(fakeApiClient.calls, hasLength(1));

      // Trigger loadMore
      await container.read(billsProvider.notifier).loadMore();

      expect(fakeApiClient.calls, hasLength(2));
      expect(fakeApiClient.calls[1]['page'], 2);
      expect(fakeApiClient.calls[1]['pageSize'], 30);

      final paginated = container.read(paginatedBillsProvider);
      expect(paginated.bills, hasLength(60));
      expect(paginated.page, 2);
      expect(paginated.hasMore, isFalse); // Reached last page (page 2 of 2)
      expect(paginated.isLoadingMore, isFalse);

      // Subsequent loadMore when hasMore == false should be a no-op
      await container.read(billsProvider.notifier).loadMore();
      expect(fakeApiClient.calls, hasLength(2)); // No extra request made
    });

    test('loadMore() handles network error gracefully without losing existing data', () async {
      fakeApiClient.failNextPage = true;

      final authRepo = _createTestAuthRepository();
      final container = ProviderContainer(
        overrides: [
          iuranRepositoryProvider.overrideWithValue(repository),
          authRepositoryProvider.overrideWith((ref) => authRepo),
        ],
      );

      await container.read(billsProvider.future);
      expect(container.read(paginatedBillsProvider).bills, hasLength(30));

      // Attempt loadMore which will fail
      await container.read(billsProvider.notifier).loadMore();

      final paginatedAfterError = container.read(paginatedBillsProvider);
      expect(paginatedAfterError.bills, hasLength(30)); // Existing data preserved!
      expect(paginatedAfterError.isLoadingMore, isFalse);
      expect(paginatedAfterError.loadMoreError, isNotNull);
      expect(paginatedAfterError.loadMoreError, contains('timed out'));

      // Retry loadMore
      await container.read(billsProvider.notifier).retryLoadMore();

      final paginatedAfterRetry = container.read(paginatedBillsProvider);
      expect(paginatedAfterRetry.bills, hasLength(60));
      expect(paginatedAfterRetry.page, 2);
      expect(paginatedAfterRetry.loadMoreError, isNull);
    });

    test('Filter changes reset to page 1 with backend filter parameters', () async {
      final authRepo = _createTestAuthRepository();
      final container = ProviderContainer(
        overrides: [
          iuranRepositoryProvider.overrideWithValue(repository),
          authRepositoryProvider.overrideWith((ref) => authRepo),
        ],
      );

      await container.read(billsProvider.future);
      expect(fakeApiClient.calls, hasLength(1));

      // Change search query
      container.read(iuranSearchQueryProvider.notifier).state = 'Warga 1';
      // Riverpod rebuilds
      await container.read(billsProvider.future);

      expect(fakeApiClient.calls.length, greaterThanOrEqualTo(2));
      final lastCall = fakeApiClient.calls.last;
      expect(lastCall['page'], 1);
      expect(lastCall['search'], 'Warga 1');

      // Change period to Indonesian string
      container.read(iuranSelectedPeriodeProvider.notifier).state = 'Oktober 2026';
      await container.read(billsProvider.future);

      final callWithPeriod = fakeApiClient.calls.last;
      expect(callWithPeriod['page'], 1);
      expect(callWithPeriod['period'], '2026-10'); // Properly converted to YYYY-MM!

      // Change status to belumBayar
      container.read(iuranSelectedStatusProvider.notifier).state = 'belumBayar';
      await container.read(billsProvider.future);

      final callWithStatus = fakeApiClient.calls.last;
      expect(callWithStatus['page'], 1);
      expect(callWithStatus['status'], 'unpaid'); // Properly converted to backend status!

      // Change payment status to pending
      container.read(iuranSelectedPaymentStatusProvider.notifier).state = 'pending';
      await container.read(billsProvider.future);

      final callWithPaymentStatus = fakeApiClient.calls.last;
      expect(callWithPaymentStatus['page'], 1);
      expect(callWithPaymentStatus['paymentStatus'], 'PENDING');
    });

    test('refresh() resets to page 1 and reloads data', () async {
      final authRepo = _createTestAuthRepository();
      final container = ProviderContainer(
        overrides: [
          iuranRepositoryProvider.overrideWithValue(repository),
          authRepositoryProvider.overrideWith((ref) => authRepo),
        ],
      );

      await container.read(billsProvider.future);
      await container.read(billsProvider.notifier).loadMore();
      expect(container.read(paginatedBillsProvider).page, 2);

      await container.read(billsProvider.notifier).refresh();

      final paginated = container.read(paginatedBillsProvider);
      expect(paginated.page, 1);
      expect(paginated.bills, hasLength(30));
    });
  });
}
