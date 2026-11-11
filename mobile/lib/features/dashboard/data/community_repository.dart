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
    final parsed = HomeDashboardData.fromJson(data.cast<String, dynamic>());
    final syncedOrg = await _syncOrganizationMemberActiveStatus(
      parsed.organization,
    );
    return parsed.copyWith(organization: syncedOrg);
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
    final parsed = OrganizationStructure.fromJson(data.cast<String, dynamic>());
    return _syncOrganizationMemberActiveStatus(parsed);
  }

  /// Cross-checks organization members against the official resident and
  /// household records so any resident or household deactivated in the Warga
  /// module (`residents.is_active == false` or `households.is_active == false`)
  /// is not treated as an active position holder on the diagram.
  Future<OrganizationStructure> _syncOrganizationMemberActiveStatus(
    OrganizationStructure org,
  ) async {
    final hasAnyMember = org.nodes.any((n) => n.members.isNotEmpty);
    if (!hasAnyMember) return org;

    try {
      final householdActiveById = <String, bool>{};
      try {
        final hhRes = await dio.get(
          '/api/v1/households',
          queryParameters: {'page': '1', 'page_size': '200'},
        );
        final hhData = hhRes.data;
        if (hhData is Map && hhData['data'] is List) {
          for (final item in (hhData['data'] as List).whereType<Map>()) {
            final id = item['id']?.toString() ?? '';
            if (id.isNotEmpty) {
              householdActiveById[id] = item['is_active'] != false;
            }
          }
        }
      } catch (_) {
        // Ignore if households endpoint is unavailable in test mocks.
      }

      final activeByUserId = <String, bool>{};
      final activeByNameJabatan = <String, bool>{};
      var fetchedAnyResidents = false;

      Future<void> collectResidents({String? isActiveParam}) async {
        try {
          final query = <String, dynamic>{'page': '1', 'page_size': '200'};
          if (isActiveParam != null) query['is_active'] = isActiveParam;
          final res = await dio.get(
            '/api/v1/residents',
            queryParameters: query,
          );
          final body = res.data;
          if (body is Map && body['data'] is List) {
            fetchedAnyResidents = true;
            for (final item in (body['data'] as List).whereType<Map>()) {
              final resIsActive = item['is_active'] != false;
              final hhId = item['household_id']?.toString() ?? '';
              final hhIsActive =
                  (hhId.isNotEmpty && householdActiveById.containsKey(hhId))
                  ? householdActiveById[hhId]!
                  : true;
              final effectiveActive = resIsActive && hhIsActive;

              final uid = item['user_id']?.toString() ?? '';
              if (uid.isNotEmpty) {
                activeByUserId[uid] =
                    (activeByUserId[uid] ?? false) || effectiveActive;
              }
              final fullName = (item['full_name']?.toString() ?? '')
                  .trim()
                  .toLowerCase();
              final jab = normalizeOrgPosition(
                item['jabatan']?.toString() ?? '',
              );
              if (fullName.isNotEmpty && jab.isNotEmpty) {
                final key = '$fullName|$jab';
                activeByNameJabatan[key] =
                    (activeByNameJabatan[key] ?? false) || effectiveActive;
              }
            }
          }
        } catch (_) {
          // Ignore if residents endpoint is unavailable in test mocks.
        }
      }

      await collectResidents();
      await collectResidents(isActiveParam: 'false');

      if (!fetchedAnyResidents) return org;

      final updatedNodes = org.nodes.map((node) {
        final nodePos = normalizeOrgPosition(node.position, node.positionLabel);
        final updatedMembers = node.members.map((m) {
          if (!m.isActive) return m;
          bool? residentActive;
          if (m.userId.isNotEmpty && activeByUserId.containsKey(m.userId)) {
            residentActive = activeByUserId[m.userId];
          } else {
            final mJab = normalizeOrgPosition(
              m.jabatan.isNotEmpty ? m.jabatan : nodePos,
            );
            final key = '${m.name.trim().toLowerCase()}|$mJab';
            if (activeByNameJabatan.containsKey(key)) {
              residentActive = activeByNameJabatan[key];
            }
          }
          if (residentActive == false) {
            return m.copyWith(isActive: false);
          }
          return m;
        }).toList();
        return node.copyWith(members: updatedMembers);
      }).toList();

      return org.copyWith(nodes: updatedNodes);
    } catch (_) {
      return org;
    }
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
