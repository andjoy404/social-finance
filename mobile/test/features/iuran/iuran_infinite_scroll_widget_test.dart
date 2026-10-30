import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/api/device_id_service.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/iuran/data/iuran_models.dart';
import 'package:social_finance/features/iuran/data/iuran_providers.dart';
import 'package:social_finance/features/iuran/presentation/screens/iuran_screen.dart';

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

class _TestPaginatedNotifier extends BillsNotifier {
  _TestPaginatedNotifier({required this.initialState});

  final PaginatedBillsState initialState;
  int loadMoreCallCount = 0;
  int refreshCallCount = 0;

  @override
  Future<List<IuranBill>> build() async {
    return initialState;
  }

  @override
  Future<void> loadMore() async {
    loadMoreCallCount++;
    state = AsyncData(
      initialState.copyWith(
        isLoadingMore: true,
      ),
    );
  }

  @override
  Future<void> retryLoadMore() async {
    loadMoreCallCount++;
  }

  @override
  Future<void> refresh() async {
    refreshCallCount++;
  }
}

void main() {
  Widget buildTestWidget({
    required Widget child,
    required List<Override> overrides,
  }) {
    return ProviderScope(
      overrides: overrides,
      child: MaterialApp(
        home: child,
      ),
    );
  }

  group('IuranScreen — Infinite Scroll & Pagination UI Tests', () {
    testWidgets('Displays total count header and renders bill cards', (tester) async {
      final bills = List.generate(
        5,
        (i) => IuranBill(
          id: 'b-$i',
          householdName: 'Keluarga $i',
          iuranType: 'Iuran Kebersihan',
          periode: 'Oktober 2026',
          nominal: 50000,
          status: IuranStatus.belumBayar,
        ),
      );

      final pagState = PaginatedBillsState(
        bills: bills,
        page: 1,
        pageSize: 30,
        total: 75,
        totalPages: 3,
        hasMore: true,
      );

      final notifier = _TestPaginatedNotifier(initialState: pagState);

      await tester.pumpWidget(
        buildTestWidget(
          child: const IuranScreen(),
          overrides: [
            billsProvider.overrideWith(() => notifier),
            myBillsProvider.overrideWith(() => notifier),
            authRepositoryProvider.overrideWith(
              (ref) => _createTestAuthRepository(),
            ),
          ],
        ),
      );

      await tester.pumpAndSettle();

      // Check total bills count in header
      expect(find.text('75 Iuran'), findsOneWidget);

      // Check iuranType badge is rendered
      expect(find.text('Iuran Kebersihan'), findsWidgets);
    });

    testWidgets('Renders loading indicator at bottom when isLoadingMore is true', (tester) async {
      final bills = List.generate(
        2,
        (i) => IuranBill(
          id: 'b-$i',
          householdName: 'Keluarga $i',
          iuranType: 'Iuran Kebersihan',
          periode: 'Oktober 2026',
          nominal: 50000,
          status: IuranStatus.belumBayar,
        ),
      );

      final pagState = PaginatedBillsState(
        bills: bills,
        page: 1,
        pageSize: 30,
        total: 50,
        totalPages: 2,
        hasMore: true,
        isLoadingMore: true,
      );

      final notifier = _TestPaginatedNotifier(initialState: pagState);

      await tester.pumpWidget(
        buildTestWidget(
          child: const IuranScreen(),
          overrides: [
            billsProvider.overrideWith(() => notifier),
            myBillsProvider.overrideWith(() => notifier),
            authRepositoryProvider.overrideWith(
              (ref) => _createTestAuthRepository(),
            ),
          ],
        ),
      );

      await tester.pump();

      // Verify CircularProgressIndicator is present in the list footer
      expect(find.byType(CircularProgressIndicator), findsWidgets);

      // Stop loading state so animation stops
      notifier.state = AsyncData(pagState.copyWith(isLoadingMore: false));
      await tester.pumpAndSettle();
    });

    testWidgets('Renders retry button at bottom when loadMoreError is present and retries on tap', (tester) async {
      final bills = List.generate(
        2,
        (i) => IuranBill(
          id: 'b-$i',
          householdName: 'Keluarga $i',
          iuranType: 'Iuran Kebersihan',
          periode: 'Oktober 2026',
          nominal: 50000,
          status: IuranStatus.belumBayar,
        ),
      );

      final pagState = PaginatedBillsState(
        bills: bills,
        page: 1,
        pageSize: 30,
        total: 50,
        totalPages: 2,
        hasMore: true,
        isLoadingMore: false,
        loadMoreError: 'Koneksi gagal',
      );

      final notifier = _TestPaginatedNotifier(initialState: pagState);

      await tester.pumpWidget(
        buildTestWidget(
          child: const IuranScreen(),
          overrides: [
            billsProvider.overrideWith(() => notifier),
            myBillsProvider.overrideWith(() => notifier),
            authRepositoryProvider.overrideWith(
              (ref) => _createTestAuthRepository(),
            ),
          ],
        ),
      );

      await tester.pumpAndSettle();

      expect(find.text('Gagal memuat data lanjutan'), findsOneWidget);
      expect(find.text('Coba Lagi'), findsOneWidget);

      await tester.tap(find.text('Coba Lagi'));
      expect(notifier.loadMoreCallCount, 1);
    });
  });
}
