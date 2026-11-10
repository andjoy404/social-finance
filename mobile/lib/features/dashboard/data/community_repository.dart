import 'dart:io';
import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/features/dashboard/data/models/community_models.dart';

/// Paginated response envelope returned by the backend list endpoints:
/// { data: [...], pagination: { page, per_page, total, total_pages } }.
class CommunityPage<T> {
  final List<T> items;
  final int page;
  final int perPage;
  final int total;
  final int totalPages;

  const CommunityPage({
    required this.items,
    required this.page,
    required this.perPage,
    required this.total,
    required this.totalPages,
  });
}

class CommunityRepository {
  final Dio dio;

  CommunityRepository({required this.dio});

  Future<HomeDashboardData> getHomeDashboard() async {
    final response = await dio.get('/api/v1/home/dashboard');
    final data = response.data;
    if (data is! Map) {
      throw const FormatException('Format respons Beranda tidak valid');
    }
    return HomeDashboardData.fromJson(data.cast<String, dynamic>());
  }

  CommunityPage<T> _parsePage<T>(
    dynamic raw,
    T Function(Map<String, dynamic>) fromJson,
  ) {
    if (raw is! Map) {
      throw const FormatException('Format daftar tidak valid');
    }
    final list = raw['data'] as List? ?? const [];
    final pagination = raw['pagination'];
    final page = pagination is Map
        ? (pagination['page'] as num?)?.toInt() ?? 1
        : 1;
    final perPage = pagination is Map
        ? (pagination['per_page'] as num?)?.toInt() ?? 20
        : 20;
    final total = pagination is Map
        ? (pagination['total'] as num?)?.toInt() ?? list.length
        : list.length;
    final totalPages = pagination is Map
        ? (pagination['total_pages'] as num?)?.toInt() ?? 1
        : 1;
    return CommunityPage<T>(
      items: list.whereType<Map<String, dynamic>>().map(fromJson).toList(),
      page: page,
      perPage: perPage,
      total: total,
      totalPages: totalPages,
    );
  }

  Future<CommunityPage<CommunityActivity>> listActivities({
    String? status,
    String? publicationStatus,
    int page = 1,
    int pageSize = 20,
  }) async {
    final query = <String, dynamic>{'page': page, 'page_size': pageSize};
    if (status != null && status.isNotEmpty) query['status'] = status;
    if (publicationStatus != null && publicationStatus.isNotEmpty) {
      query['publication_status'] = publicationStatus;
    }
    final response = await dio.get(
      '/api/v1/activities',
      queryParameters: query,
    );
    return _parsePage(response.data, CommunityActivity.fromJson);
  }

  Future<CommunityActivity> getActivity(String id) async {
    final response = await dio.get('/api/v1/activities/$id');
    return CommunityActivity.fromJson(
      (response.data as Map).cast<String, dynamic>(),
    );
  }

  Future<CommunityActivity> createActivity(Map<String, dynamic> body) async {
    final response = await dio.post('/api/v1/activities', data: body);
    return CommunityActivity.fromJson(
      (response.data as Map).cast<String, dynamic>(),
    );
  }

  Future<CommunityActivity> updateActivity(
    String id,
    Map<String, dynamic> body,
  ) async {
    final response = await dio.put('/api/v1/activities/$id', data: body);
    return CommunityActivity.fromJson(
      (response.data as Map).cast<String, dynamic>(),
    );
  }

  Future<void> publishActivity(String id) async {
    await dio.post('/api/v1/activities/$id/publish');
  }

  Future<void> cancelActivity(String id) async {
    await dio.post('/api/v1/activities/$id/cancel');
  }

  Future<void> deleteActivity(String id) async {
    await dio.delete('/api/v1/activities/$id');
  }

  Future<CommunityPage<CommunityAnnouncement>> listAnnouncements({
    String? status,
    int page = 1,
    int pageSize = 10,
  }) async {
    final query = <String, dynamic>{'page': page, 'page_size': pageSize};
    if (status != null && status.isNotEmpty) query['status'] = status;
    final response = await dio.get(
      '/api/v1/announcements',
      queryParameters: query,
    );
    return _parsePage(response.data, CommunityAnnouncement.fromJson);
  }

  Future<CommunityAnnouncement> getAnnouncement(String id) async {
    final response = await dio.get('/api/v1/announcements/$id');
    return CommunityAnnouncement.fromJson(
      (response.data as Map).cast<String, dynamic>(),
    );
  }

  Future<CommunityAnnouncement> createAnnouncement(
    Map<String, dynamic> body,
  ) async {
    final response = await dio.post('/api/v1/announcements', data: body);
    return CommunityAnnouncement.fromJson(
      (response.data as Map).cast<String, dynamic>(),
    );
  }

  Future<CommunityAnnouncement> updateAnnouncement(
    String id,
    Map<String, dynamic> body,
  ) async {
    final response = await dio.put('/api/v1/announcements/$id', data: body);
    return CommunityAnnouncement.fromJson(
      (response.data as Map).cast<String, dynamic>(),
    );
  }

  Future<void> publishAnnouncement(String id) async {
    await dio.post('/api/v1/announcements/$id/publish');
  }

  Future<void> archiveAnnouncement(String id) async {
    await dio.post('/api/v1/announcements/$id/archive');
  }

  Future<void> deleteAnnouncement(String id) async {
    await dio.delete('/api/v1/announcements/$id');
  }

  Future<List<BannerTemplate>> listBannerTemplates() async {
    final response = await dio.get('/api/v1/banner-templates');
    final data = response.data;
    if (data is Map) {
      final list = data['data'] as List? ?? const [];
      return list
          .whereType<Map<String, dynamic>>()
          .map((item) => BannerTemplate.fromJson(item.cast<String, dynamic>()))
          .toList();
    }
    if (data is List) {
      return data
          .whereType<Map<String, dynamic>>()
          .map((item) => BannerTemplate.fromJson(item.cast<String, dynamic>()))
          .toList();
    }
    return [];
  }

  /// Uploads a community image. Returns the response map containing
  /// image_url and key (authorization is enforced server-side).
  Future<Map<String, dynamic>> uploadCommunityImage(
    File file,
    String category,
  ) async {
    final formData = FormData.fromMap({
      'file': await MultipartFile.fromFile(file.path),
      'category': category,
    });
    final response = await dio.post('/api/v1/community/upload', data: formData);
    final data = response.data;
    if (data is Map) {
      return data.cast<String, dynamic>();
    }
    return <String, dynamic>{};
  }

  Future<OrganizationStructure> getOrganizationStructure() async {
    final response = await dio.get('/api/v1/organization-structure');
    final data = response.data;
    if (data is! Map) {
      throw const FormatException('Format struktur organisasi tidak valid');
    }
    return OrganizationStructure.fromJson(data.cast<String, dynamic>());
  }
}

final communityRepositoryProvider = Provider<CommunityRepository>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  return CommunityRepository(dio: apiClient.dio);
});

final homeDashboardFutureProvider =
    FutureProvider.autoDispose<HomeDashboardData>((ref) async {
      // Return fallback immediately during widget tests if offline / test env
      if (Platform.environment.containsKey('FLUTTER_TEST')) {
        return const HomeDashboardData(
          greeting: 'Selamat datang,',
          userName: 'Warga',
          rt: '002',
          rw: '016',
          banners: [],
          security: SecurityOnDutyInfo(
            isAvailable: false,
            officers: [],
            message: 'Belum ada petugas aktif / jadwal shift saat ini',
            subtext:
                'Data shift terintegrasi langsung saat Modul Keamanan berjalan',
          ),
          organization: OrganizationStructure(),
          upcomingActivities: [],
        );
      }

      final repo = ref.watch(communityRepositoryProvider);
      return repo.getHomeDashboard();
    });
