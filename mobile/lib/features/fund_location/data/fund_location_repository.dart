import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/errors/app_errors.dart';
import 'package:social_finance/features/kas/data/kas_api_models.dart';
import 'fund_location_api_client.dart';
import 'models/fund_location_models.dart';

class FundLocationRepository {
  final FundLocationApiClient apiClient;

  FundLocationRepository({required this.apiClient});

  Future<List<FundLocation>> fetchLocations({
    String? category,
    bool? isActive,
  }) async {
    return apiClient.listLocations(category: category, isActive: isActive);
  }

  Future<FundLocation> fetchLocation(String id) async {
    return apiClient.getLocation(id);
  }

  Future<FundLocation> createLocation({
    required String name,
    required String category,
    String? bankName,
    String? accountNumber,
    String? accountHolder,
  }) async {
    return apiClient.createLocation(
      name: name,
      category: category,
      bankName: bankName,
      accountNumber: accountNumber,
      accountHolder: accountHolder,
    );
  }

  Future<FundLocation> updateLocation({
    required String id,
    String? name,
    String? bankName,
    String? accountNumber,
    String? accountHolder,
    bool? isActive,
  }) async {
    return apiClient.updateLocation(
      id: id,
      name: name,
      bankName: bankName,
      accountNumber: accountNumber,
      accountHolder: accountHolder,
      isActive: isActive,
    );
  }

  Future<void> deactivateLocation(String id) async {
    return apiClient.deactivateLocation(id);
  }

  Future<PaginatedResponse<LocationBalanceChange>> fetchLocationHistory({
    required String id,
    int page = 1,
    int pageSize = 20,
  }) async {
    return apiClient.getLocationHistory(id: id, page: page, pageSize: pageSize);
  }

  Future<void> setOpeningBalance({
    required String id,
    required int amountRupiah,
    String? notes,
  }) async {
    final amountStr = '$amountRupiah.00';
    return apiClient.setOpeningBalance(
      id: id,
      amount: amountStr,
      notes: notes,
    );
  }

  Future<PaginatedResponse<FundTransfer>> fetchTransfers({
    int page = 1,
    int pageSize = 20,
  }) async {
    return apiClient.listTransfers(page: page, pageSize: pageSize);
  }

  Future<FundTransfer> createTransfer({
    required String fromLocationId,
    required String toLocationId,
    required int amountRupiah,
    String? notes,
    String? idempotencyKey,
  }) async {
    final amountStr = '$amountRupiah.00';
    return apiClient.createTransfer(
      fromLocationId: fromLocationId,
      toLocationId: toLocationId,
      amount: amountStr,
      notes: notes,
      idempotencyKey: idempotencyKey,
    );
  }

  Future<void> reverseTransfer({
    required String id,
    required String reason,
  }) async {
    return apiClient.reverseTransfer(id: id, reason: reason);
  }

  Future<FundLocationReconciliationReport> fetchReconciliationReport() async {
    return apiClient.getReconciliationReport();
  }

  ServerException mapDioError(dynamic error) {
    if (error is ServerException) return error;
    if (error is DioException) {
      final status = error.response?.statusCode;
      final body = error.response?.data;
      if (body is Map && body['error'] is Map) {
        final msg = body['error']['message'] as String?;
        if (msg != null && msg.isNotEmpty) {
          return ServerException(statusCode: status ?? 400, message: msg);
        }
      }
      return ServerException(
        statusCode: status ?? 500,
        message: 'Gagal memproses permintaan lokasi dana (${status ?? "koneksi"}).',
      );
    }
    return ServerException(message: 'Terjadi kesalahan tidak terduga.');
  }
}

final fundLocationApiClientProvider = Provider<FundLocationApiClient>((ref) {
  final client = ref.watch(apiClientProvider);
  return FundLocationApiClient(dio: client.dio);
});

final fundLocationRepositoryProvider = Provider<FundLocationRepository>((ref) {
  final apiClient = ref.watch(fundLocationApiClientProvider);
  return FundLocationRepository(apiClient: apiClient);
});

final fundLocationsProvider = FutureProvider.autoDispose<List<FundLocation>>((ref) async {
  final repo = ref.watch(fundLocationRepositoryProvider);
  return repo.fetchLocations();
});

final fundTransfersProvider = FutureProvider.autoDispose<PaginatedResponse<FundTransfer>>((ref) async {
  final repo = ref.watch(fundLocationRepositoryProvider);
  return repo.fetchTransfers(page: 1, pageSize: 50);
});

final fundReconciliationProvider = FutureProvider.autoDispose<FundLocationReconciliationReport>((ref) async {
  final repo = ref.watch(fundLocationRepositoryProvider);
  return repo.fetchReconciliationReport();
});
