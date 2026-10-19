import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/api/client.dart';
import '../../../core/errors/app_errors.dart';
import 'rw_models.dart';

class RwRepository {
  final ApiClient client;

  RwRepository({required this.client});

  Future<List<RwModel>> listRws({
    bool? isActive,
    String? search,
    int page = 1,
    int pageSize = 50,
  }) async {
    final params = <String, dynamic>{
      'page': page.toString(),
      'page_size': pageSize.toString(),
    };
    if (isActive != null) params['is_active'] = isActive.toString();
    if (search != null && search.trim().isNotEmpty) params['search'] = search.trim();

    final response = await client.dio.get('/api/v1/rws', queryParameters: params);
    final data = response.data;
    if (data is! Map) throw const FormatException('Expected JSON object');
    final rawList = data['data'];
    if (rawList is! List) return [];
    return rawList
        .whereType<Map>()
        .map((e) => RwModel.fromJson(e.cast<String, dynamic>()))
        .toList();
  }

  Future<RwModel> createRw({
    required int rwNumber,
    String? name,
    String? headName,
    String? address,
  }) async {
    final body = <String, dynamic>{
      'rw_number': rwNumber,
      'name': (name != null && name.trim().isNotEmpty)
          ? name.trim()
          : 'RW ${rwNumber.toString().padLeft(3, '0')}',
    };
    if (headName != null && headName.trim().isNotEmpty) body['head_name'] = headName.trim();
    if (address != null && address.trim().isNotEmpty) body['address'] = address.trim();

    final response = await client.dio.post('/api/v1/rws', data: body);
    final data = response.data;
    if (data is! Map) throw const FormatException('Expected JSON object');
    return RwModel.fromJson(data.cast<String, dynamic>());
  }

  Future<RwModel> updateRw(
    String id, {
    int? rwNumber,
    String? name,
    String? headName,
    String? address,
    bool? isActive,
  }) async {
    final body = <String, dynamic>{};
    if (rwNumber != null) body['rw_number'] = rwNumber;
    if (name != null) body['name'] = name.trim();
    if (headName != null) body['head_name'] = headName.trim();
    if (address != null) body['address'] = address.trim();
    if (isActive != null) body['is_active'] = isActive;

    final response = await client.dio.patch('/api/v1/rws/$id', data: body);
    final data = response.data;
    if (data is! Map) throw const FormatException('Expected JSON object');
    return RwModel.fromJson(data.cast<String, dynamic>());
  }

  Future<void> deactivateRw(String id) async {
    await client.dio.patch('/api/v1/rws/$id/deactivate');
  }

  ServerException mapDioError(dynamic error) {
    if (error is ServerException) return error;
    if (error is DioException) {
      final status = error.response?.statusCode;
      if (status == 401) {
        return ServerException(statusCode: 401, message: 'Sesi Anda telah berakhir. Silakan login kembali.');
      } else if (status == 403) {
        return ServerException(statusCode: 403, message: 'Anda tidak memiliki akses.');
      } else if (status == 409) {
        final msg = error.response?.data is Map ? (error.response?.data['message'] ?? 'Konflik data.') : 'Konflik data.';
        return ServerException(statusCode: 409, message: msg.toString());
      } else if (status != null && status >= 500) {
        return ServerException(statusCode: status, message: 'Terjadi kesalahan pada server.');
      }
    }
    return ServerException(message: 'Terjadi kesalahan yang tidak diketahui.');
  }
}

final rwRepositoryProvider = Provider<RwRepository>((ref) {
  final client = ref.watch(apiClientProvider);
  return RwRepository(client: client);
});
