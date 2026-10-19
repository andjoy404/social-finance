import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/errors/app_errors.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/iuran/data/iuran_api_client.dart';
import 'package:social_finance/features/iuran/data/iuran_models.dart';
import 'package:social_finance/features/iuran/data/iuran_providers.dart';
import 'package:social_finance/features/iuran/data/iuran_repository.dart';

class _RecordingIuranRepository extends IuranRepository {
  _RecordingIuranRepository({
    this.failure,
    List<List<IuranBill>> results = const [],
  }) : results = List.of(results),
       super(apiClient: IuranApiClient(dio: Dio()));

  final Object? failure;
  final scopes = <bool>[];
  final List<List<IuranBill>> results;

  @override
  Future<List<IuranBill>> fetchAllBills({
    String? search,
    String? status,
    bool myBills = false,
  }) async {
    scopes.add(myBills);
    if (failure case final failure?) throw failure;
    return results.isEmpty ? [] : results.removeAt(0);
  }
}

void main() {
  test(
    'Tagihan requests remain scoped and reload on account changes',
    () async {
      final apiClient = ApiClient(customDio: Dio());
      apiClient.setToken('test-token');
      final authRepository = AuthRepository(
        authService: AuthService(apiClient),
        apiClient: apiClient,
      );
      authRepository.state = AsyncData<Map<String, dynamic>?>({'id': 'user-1'});
      final repository = _RecordingIuranRepository(
        results: [
          [
            IuranBill(
              id: 'user-1-bill',
              householdName: 'User 1 household',
              iuranType: 'Iuran Bulanan',
              periode: 'Oktober 2026',
              nominal: 50000,
            ),
          ],
          [
            IuranBill(
              id: 'user-2-bill',
              householdName: 'User 2 household',
              iuranType: 'Iuran Bulanan',
              periode: 'Oktober 2026',
              nominal: 50000,
            ),
          ],
        ],
      );
      final container = ProviderContainer(
        overrides: [
          authRepositoryProvider.overrideWith((ref) => authRepository),
          iuranRepositoryProvider.overrideWithValue(repository),
        ],
      );
      final subscription = container.listen(myBillsProvider, (_, _) {});
      addTearDown(() {
        subscription.close();
        container.dispose();
        apiClient.dio.close();
        repository.apiClient.dio.close();
      });

      final user1Bills = await container.read(myBillsProvider.future);
      expect(user1Bills.single.id, 'user-1-bill');
      expect(repository.scopes, [true]);

      authRepository.state = AsyncData<Map<String, dynamic>?>({'id': 'user-2'});
      final user2Bills = await container.read(myBillsProvider.future);
      expect(user2Bills.single.id, 'user-2-bill');
      expect(repository.scopes, [true, true]);
    },
  );

  test('Tagihan does not fetch without an authenticated session', () async {
    final apiClient = ApiClient(customDio: Dio());
    final authRepository = AuthRepository(
      authService: AuthService(apiClient),
      apiClient: apiClient,
    );
    authRepository.state = const AsyncData<Map<String, dynamic>?>(null);
    final repository = _RecordingIuranRepository();
    final container = ProviderContainer(
      overrides: [
        authRepositoryProvider.overrideWith((ref) => authRepository),
        iuranRepositoryProvider.overrideWithValue(repository),
      ],
    );
    final subscription = container.listen(myBillsProvider, (_, _) {});
    addTearDown(() {
      subscription.close();
      container.dispose();
      apiClient.dio.close();
      repository.apiClient.dio.close();
    });

    await expectLater(
      container.read(myBillsProvider.future),
      throwsA(isA<StateError>()),
    );
    expect(repository.scopes, isEmpty);
  });

  test('Tagihan refresh remains scoped and preserves request errors', () async {
    final apiClient = ApiClient(customDio: Dio());
    apiClient.setToken('test-token');
    final authRepository = AuthRepository(
      authService: AuthService(apiClient),
      apiClient: apiClient,
    );
    authRepository.state = AsyncData<Map<String, dynamic>?>({'id': 'user-1'});
    final repository = _RecordingIuranRepository(
      failure: StateError('network unavailable'),
    );
    final container = ProviderContainer(
      overrides: [
        authRepositoryProvider.overrideWith((ref) => authRepository),
        iuranRepositoryProvider.overrideWithValue(repository),
      ],
    );
    final subscription = container.listen(myBillsProvider, (_, _) {});
    addTearDown(() {
      subscription.close();
      container.dispose();
      apiClient.dio.close();
      repository.apiClient.dio.close();
    });

    await expectLater(
      container.read(myBillsProvider.future),
      throwsA(isA<ServerException>()),
    );
    expect(container.read(myBillsProvider), isA<AsyncError<List<IuranBill>>>());
    await container.read(myBillsProvider.notifier).refresh();

    expect(repository.scopes, [true, true]);
    expect(container.read(myBillsProvider), isA<AsyncError<List<IuranBill>>>());
  });
}
