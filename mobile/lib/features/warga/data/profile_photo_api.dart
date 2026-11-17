import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/api/client.dart';

/// Thin API wrapper for resident profile photos.
/// Authorization (resident.profile_photo.manage) is enforced server-side;
/// this client only shields callers from raw Dio boilerplate.
class ProfilePhotoApi {
  ProfilePhotoApi(this._client);

  final ApiClient _client;

  /// Upload (or replace) the resident's profile photo (multipart "file").
  /// Returns the stored avatar_path reference on success.
  Future<String> upload(
    String residentId,
    String filePath,
    String fileName,
  ) async {
    final form = FormData.fromMap({
      'file': await MultipartFile.fromFile(filePath, filename: fileName),
    });
    final res = await _client.dio.post<Map<String, dynamic>>(
      '/api/v1/residents/$residentId/photo',
      data: form,
    );
    final data = res.data ?? const <String, dynamic>{};
    final path = (data['avatar_path'] ?? data['key']) as String?;
    if (path == null || path.isEmpty) {
      throw const FormatException('Server did not return an avatar reference.');
    }
    return path;
  }

  /// Clear the resident's profile photo.
  Future<void> delete(String residentId) async {
    await _client.dio.delete<Map<String, dynamic>>(
      '/api/v1/residents/$residentId/photo',
    );
  }
}

final profilePhotoApiProvider = Provider<ProfilePhotoApi>((ref) {
  final client = ref.watch(apiClientProvider);
  return ProfilePhotoApi(client);
});
