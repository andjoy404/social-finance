import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:dio/dio.dart';

import '../../../core/api/client.dart';
import '../../../core/api/config.dart';
import '../../../core/errors/app_errors.dart';
import 'api_warga_models.dart';

class WargaRepository {
  final ApiClient client;

  WargaRepository({required this.client});

  Future<PaginatedResponse<BackendResident>> fetchResidents({
    int page = 1,
    int pageSize = 100,
    String? search,
  }) async {
    final queries = <String, dynamic>{
      'page': page.toString(),
      'page_size': pageSize.toString(),
    };
    if (search != null && search.trim().isNotEmpty) {
      queries['search'] = search.trim();
    }

    final response = await client.dio.get(
      '/api/v1/residents',
      queryParameters: queries,
    );

    final data = response.data;
    if (data is! Map) {
      throw const FormatException('Expected JSON object in residents response');
    }

    final rawList = data['data'];
    final items = <BackendResident>[];

    if (rawList is List) {
      for (final item in rawList) {
        if (item is Map) {
          items.add(BackendResident.fromJson(item.cast<String, dynamic>()));
        }
      }
    }

    final pag = data['pagination'] as Map<String, dynamic>?;
    return PaginatedResponse<BackendResident>(
      data: items,
      pagination: PaginationInfo(
        page: (pag?['page'] as num?)?.toInt() ?? page,
        pageSize: (pag?['page_size'] as num?)?.toInt() ?? pageSize,
        total: (pag?['total'] as num?)?.toInt() ?? items.length,
        totalPages: (pag?['total_pages'] as num?)?.toInt() ?? 1,
      ),
    );
  }

  Future<Map<String, BackendHousehold>> fetchHouseholds({
    int page = 1,
    int pageSize = 100,
  }) async {
    final allHouseholds = <BackendHousehold>[];
    int currentPage = 1;
    int totalPages = 1;

    do {
      final response = await client.dio.get(
        '/api/v1/households',
        queryParameters: {
          'page': currentPage.toString(),
          'page_size': pageSize.toString(),
        },
      );

      final data = response.data;
      if (data is Map) {
        final rawList = data['data'];
        if (rawList is List) {
          for (final item in rawList) {
            if (item is Map) {
              allHouseholds.add(
                BackendHousehold.fromJson(item.cast<String, dynamic>()),
              );
            }
          }
        }

        final pag = data['pagination'] as Map<String, dynamic>?;
        totalPages = (pag?['total_pages'] as num?)?.toInt() ?? 1;
      }

      currentPage++;
    } while (currentPage <= totalPages);

    final result = <String, BackendHousehold>{};
    for (final hh in allHouseholds) {
      result[hh.id] = hh;
    }

    return result;
  }

  Future<PaginatedResponse<MappedResident>> fetchResidentsWithHouseholds({
    int page = 1,
    int pageSize = 100,
    String? search,
  }) async {
    final allResidents = <BackendResident>[];
    int currentPage = 1;
    int totalPages = 1;

    do {
      final result = await fetchResidents(
        page: currentPage,
        pageSize: pageSize,
        search: search,
      );
      allResidents.addAll(result.data);
      totalPages = result.pagination.totalPages;
      currentPage++;
    } while (currentPage <= totalPages);

    final households = await fetchHouseholds();

    final mapped = allResidents
        .map((r) {
          final hh = households[r.householdId ?? ''];
          return MappedResident.fromBackend(r, hh);
        })
        .toList();

    return PaginatedResponse<MappedResident>(
      data: mapped,
      pagination: PaginationInfo(
        page: page,
        pageSize: pageSize,
        total: allResidents.length,
        totalPages: totalPages,
      ),
    );
  }

  ServerException mapDioError(dynamic error) {
    if (error is ServerException) return error;

    if (error is DioException) {
      final status = error.response?.statusCode;
      if (status == 401) {
        return ServerException(
          statusCode: 401,
          message: 'Sesi Anda telah berakhir. Silakan login kembali.',
        );
      } else if (status == 403) {
        return ServerException(
          statusCode: 403,
          message: 'Anda tidak memiliki akses untuk melihat data warga.',
        );
      } else if (status != null && status >= 500) {
        return ServerException(
          statusCode: status,
          message: 'Terjadi kesalahan pada server. Silakan coba lagi nanti.',
        );
      } else {
        return ServerException(
          statusCode: status,
          message: 'Permintaan gagal diproses (${status ?? "unknown"}).',
        );
      }
    }

    if (error is FormatException) {
      return ServerException(
        message: 'Format respons dari server tidak sesuai.',
      );
    }

    if (error is ApiConfigException) {
      return ServerException(message: error.message);
    }

    return ServerException(
      message: 'Terjadi kesalahan yang tidak diketahui.',
    );
  }
}

final wargaRepositoryProvider = Provider<WargaRepository>((ref) {
  final client = ref.watch(apiClientProvider);
  return WargaRepository(client: client);
});
