import 'dart:io';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:image_picker/image_picker.dart';

import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/utils/permissions.dart';
import 'package:social_finance/core/widgets/resident_avatar.dart';
import '../../data/profile_photo_api.dart';

/// Avatar + "Ubah Foto / Hapus Foto" actions for a Warga or Petugas Keamanan.
///
/// The Ubah/Hapus actions are visible only to users carrying the
/// resident.profile_photo.manage capability (currently position 'ketua').
/// Authorization is enforced server-side; this widget only hides/shows UI.
///
/// On a successful server upload the new avatar_path is returned via [onChanged];
/// the UI never shows a photo the server has not accepted.
class ResidentPhotoManage extends ConsumerStatefulWidget {
  final String residentId;
  final String fullName;
  final String? avatarPath;
  final bool busy;
  final void Function(String? newAvatarPath) onChanged;

  const ResidentPhotoManage({
    super.key,
    required this.residentId,
    required this.fullName,
    this.avatarPath,
    this.busy = false,
    required this.onChanged,
  });

  @override
  ConsumerState<ResidentPhotoManage> createState() =>
      _ResidentPhotoManageState();
}

class _ResidentPhotoManageState extends ConsumerState<ResidentPhotoManage> {
  bool _busy = false;
  String? _error;

  bool get _isBusy => _busy || widget.busy;

  static const int _maxBytes = 5 * 1024 * 1024; // 5 MiB — must match the backend limit.

  Future<void> _pickAndUpload() async {
    final picker = ImagePicker();
    XFile? picked;
    try {
      picked = await picker.pickImage(
        source: ImageSource.gallery,
        maxWidth: 1024,
        maxHeight: 1024,
        imageQuality: 85,
      );
    } catch (_) {
      _setError('Tidak dapat membuka galeri.');
      return;
    }
    if (picked == null) return;

    try {
      final length = await picked.length();
      if (length > _maxBytes) {
        _setError('Ukuran maksimum foto profil adalah 5 MB.');
        return;
      }
    } on FileSystemException {
      // If size cannot be read, fall through and let the server reject.
    }

    // Show preview confirmation before committing to the server.
    if (!mounted) return;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Simpan Foto?'),
        content: SizedBox(
          width: 220,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              ClipRRect(
                borderRadius: BorderRadius.circular(12),
                child: SizedBox(
                  width: 160,
                  height: 160,
                  child: Image.file(File(picked!.path), fit: BoxFit.cover),
                ),
              ),
              const SizedBox(height: 12),
              Text(
                'Foto ${widget.fullName} akan diganti.',
                textAlign: TextAlign.center,
              ),
            ],
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: const Text('Batal'),
          ),
          FilledButton(
            key: const Key('resident-photo-confirm-save'),
            onPressed: () => Navigator.of(ctx).pop(true),
            child: const Text('Simpan'),
          ),
        ],
      ),
    );
    if (confirmed != true) return;

    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final api = ref.read(profilePhotoApiProvider);
      final newPath = await api.upload(
        widget.residentId,
        picked.path,
        picked.name,
      );
      if (!mounted) return;
      widget.onChanged(newPath);
    } on DioException catch (e) {
      final status = e.response?.statusCode;
      String message = 'Gagal mengunggah foto.';
      if (status == 403 || status == 401) {
        message = 'Anda tidak berwenang mengubah foto ini.';
      } else if (status == 400) {
        message = 'File foto tidak valid (format atau ukuran).';
      } else if (status == 413) {
        message = 'File terlalu besar (maks 5 MB).';
      }
      _setError(message);
    } catch (_) {
      _setError('Gagal mengunggah foto.');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _delete() async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Hapus Foto?'),
        content: Text('Foto ${widget.fullName} akan dihapus.'),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: const Text('Batal'),
          ),
          FilledButton(
            key: const Key('resident-photo-confirm-delete'),
            onPressed: () => Navigator.of(ctx).pop(true),
            child: const Text('Hapus'),
          ),
        ],
      ),
    );
    if (confirmed != true) return;

    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final api = ref.read(profilePhotoApiProvider);
      await api.delete(widget.residentId);
      if (!mounted) return;
      widget.onChanged(null);
    } on DioException catch (e) {
      final status = e.response?.statusCode;
      _setError(
        status == 403 || status == 401
            ? 'Anda tidak berwenang mengubah foto ini.'
            : 'Gagal menghapus foto.',
      );
    } catch (_) {
      _setError('Gagal menghapus foto.');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  void _setError(String message) {
    if (!mounted) return;
    setState(() {
      _error = message;
    });
  }

  @override
  Widget build(BuildContext context) {
    final user = ref.watch(authRepositoryProvider)?.valueOrNull;
    final canManage = canManageResidentProfilePhoto(user);

    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        ResidentAvatar(
          name: widget.fullName,
          avatarPath: widget.avatarPath,
          size: 84,
        ),
        const SizedBox(height: 8),
        if (canManage)
          Wrap(
            spacing: 8,
            alignment: WrapAlignment.center,
            children: [
              OutlinedButton.icon(
                key: const Key('resident-photo-upload'),
                onPressed: _isBusy ? null : _pickAndUpload,
                icon: const Icon(Icons.photo_camera_outlined, size: 16),
                label: const Text('Ubah Foto'),
              ),
              if ((widget.avatarPath ?? '').trim().isNotEmpty)
                TextButton.icon(
                  key: const Key('resident-photo-delete'),
                  onPressed: _isBusy ? null : _delete,
                  icon: const Icon(Icons.delete_outline, size: 16),
                  label: const Text('Hapus'),
                ),
              if (_isBusy)
                const Padding(
                  padding: EdgeInsets.all(6),
                  child: SizedBox(
                    width: 18,
                    height: 18,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  ),
                ),
            ],
          ),
        if (_error != null) ...[
          const SizedBox(height: 6),
          Text(
            _error!,
            textAlign: TextAlign.center,
            style: TextStyle(color: AppColors.danger, fontSize: 12),
          ),
        ],
      ],
    );
  }
}
