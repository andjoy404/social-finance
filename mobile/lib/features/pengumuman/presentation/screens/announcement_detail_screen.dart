import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/community_image.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import 'package:social_finance/core/utils/permissions.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/dashboard/data/community_repository.dart';
import 'package:social_finance/features/dashboard/data/models/community_models.dart';

import 'announcement_ui.dart';

class AnnouncementDetailScreen extends ConsumerStatefulWidget {
  final String announcementId;

  const AnnouncementDetailScreen({super.key, required this.announcementId});

  @override
  ConsumerState<AnnouncementDetailScreen> createState() =>
      _AnnouncementDetailScreenState();
}

class _AnnouncementDetailScreenState
    extends ConsumerState<AnnouncementDetailScreen> {
  CommunityAnnouncement? _announcement;
  bool _isLoading = true;
  String? _error;
  String? _actionError;

  @override
  void initState() {
    super.initState();
    if (Platform.environment.containsKey('FLUTTER_TEST')) {
      _isLoading = false;
    } else {
      _load();
    }
  }

  Future<void> _load() async {
    setState(() {
      _isLoading = true;
      _error = null;
    });
    try {
      final repo = ref.read(communityRepositoryProvider);
      final ann = await repo.getAnnouncement(widget.announcementId);
      if (mounted) {
        setState(() {
          _announcement = ann;
          _isLoading = false;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _error = e.toString();
          _isLoading = false;
        });
      }
    }
  }

  Future<void> _action(
    Future<void> Function() action,
    String errorMessage,
  ) async {
    setState(() => _actionError = null);
    try {
      await action();
      await _load();
    } catch (_) {
      if (mounted) {
        setState(() => _actionError = errorMessage);
      }
    }
  }

  Future<void> _confirmAction(
    String title,
    String message,
    Future<void> Function() action,
    String errorMessage, {
    bool popAfter = false,
  }) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(title),
        content: Text(message),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: const Text('Batal'),
          ),
          FilledButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: const Text('Ya'),
          ),
        ],
      ),
    );
    if (confirmed == true) {
      if (popAfter) {
        setState(() => _actionError = null);
        try {
          await action();
          if (mounted) context.pop(true);
        } catch (_) {
          if (mounted) setState(() => _actionError = errorMessage);
        }
      } else {
        await _action(action, errorMessage);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final authState = ref.watch(authRepositoryProvider);
    final user = authState?.valueOrNull;
    final canManage = canManageAnnouncements(user);
    final a = _announcement;
    final canPublish = a != null && a.status == 'draft';
    final canRepublish =
        a != null &&
        (a.status == 'active' ||
            a.status == 'scheduled' ||
            a.status == 'archived');
    final canArchive =
        a != null && (a.status == 'active' || a.status == 'scheduled');
    final canDelete = a != null && a.status == 'draft';

    return Scaffold(
      appBar: AppBar(
        centerTitle: false,
        title: const MenuAppBarTitle(title: 'Detail Pengumuman'),
        actions: [
          if (a != null && canManage)
            IconButton(
              tooltip: 'Ubah pengumuman',
              icon: const Icon(Icons.edit_outlined),
              onPressed: () async {
                final changed = await context.push<bool>(
                  '/home/pengumuman/${a.id}/edit',
                );
                if (changed == true && mounted) {
                  await _load();
                }
              },
            ),
        ],
      ),
      body: _isLoading
          ? const Center(child: CircularProgressIndicator.adaptive())
          : _error != null
          ? Center(
              child: Padding(
                padding: const EdgeInsets.all(AppSpacing.base),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    const Icon(
                      Icons.error_outline,
                      size: 40,
                      color: AppColors.danger,
                    ),
                    const SizedBox(height: 8),
                    Text(
                      'Pengumuman tidak dapat dibuka',
                      textAlign: TextAlign.center,
                      style: theme.textTheme.titleMedium,
                    ),
                    const SizedBox(height: 4),
                    Text(_error!, textAlign: TextAlign.center),
                    const SizedBox(height: 12),
                    ElevatedButton(
                      onPressed: _load,
                      child: const Text('Coba Lagi'),
                    ),
                  ],
                ),
              ),
            )
          : a == null
          ? const SizedBox.shrink()
          : ListView(
              padding: const EdgeInsets.all(AppSpacing.base),
              children: [
                if (a.imageUrl != null && a.imageUrl!.isNotEmpty) ...[
                  ClipRRect(
                    borderRadius: BorderRadius.circular(AppRadius.base),
                    child: CommunityImage(
                      imageRef: a.imageUrl!,
                      width: double.infinity,
                      height: 180,
                      fit: BoxFit.cover,
                      alt: 'Gambar pengumuman ${a.title}',
                    ),
                  ),
                  const SizedBox(height: AppSpacing.base),
                ],
                Row(
                  children: [
                    announcementStatusBadge(a.status),
                    const SizedBox(width: 8),
                    if (a.priority >= 10)
                      const Row(
                        children: [
                          Icon(Icons.star, size: 12, color: Colors.amber),
                          SizedBox(width: 4),
                          Text(
                            'Penting',
                            style: TextStyle(fontSize: 11, color: Colors.amber),
                          ),
                        ],
                      ),
                  ],
                ),
                const SizedBox(height: 8),
                Text(
                  a.title,
                  style: theme.textTheme.headlineSmall?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
                ),
                const SizedBox(height: 8),
                if (scheduleWindowLabel(a.scheduledStart, a.scheduledEnd) !=
                    null)
                  Text(
                    scheduleWindowLabel(a.scheduledStart, a.scheduledEnd)!,
                    style: theme.textTheme.labelMedium?.copyWith(
                      color: AppColors.textMutedColor(context),
                    ),
                  ),
                if (a.publishedAt != null) ...[
                  const SizedBox(height: 4),
                  Text(
                    'Diterbitkan ${a.publishedAt!.toLocal().day}/${a.publishedAt!.toLocal().month}/${a.publishedAt!.toLocal().year}',
                    style: theme.textTheme.labelMedium?.copyWith(
                      color: AppColors.textMutedColor(context),
                    ),
                  ),
                ],
                const SizedBox(height: AppSpacing.base),
                Text(
                  a.body,
                  style: theme.textTheme.bodyLarge?.copyWith(height: 1.6),
                ),
                if (_actionError != null) ...[
                  const SizedBox(height: AppSpacing.base),
                  Container(
                    padding: const EdgeInsets.all(12),
                    decoration: BoxDecoration(
                      color: Colors.red.withValues(alpha: 0.12),
                      border: Border.all(
                        color: Colors.red.withValues(alpha: 0.4),
                      ),
                      borderRadius: BorderRadius.circular(AppRadius.sm),
                    ),
                    child: Text(
                      _actionError!,
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: AppColors.danger,
                      ),
                    ),
                  ),
                ],
                if (canManage &&
                    (canPublish ||
                        canRepublish ||
                        canArchive ||
                        canDelete)) ...[
                  const SizedBox(height: AppSpacing.base),
                  Wrap(
                    spacing: 8,
                    runSpacing: 8,
                    children: [
                      if (canPublish)
                        OutlinedButton.icon(
                          icon: const Icon(Icons.publish),
                          label: const Text('Publikasikan'),
                          onPressed: () => _confirmAction(
                            'Publikasikan pengumuman?',
                            'Pengumuman akan langsung tayang di Beranda.',
                            () => ref
                                .read(communityRepositoryProvider)
                                .publishAnnouncement(a.id),
                            'Gagal mempublikasikan pengumuman',
                          ),
                        ),
                      if (canRepublish)
                        OutlinedButton.icon(
                          icon: const Icon(Icons.refresh),
                          label: const Text('Terbitkan Ulang'),
                          onPressed: () => _confirmAction(
                            'Terbitkan ulang?',
                            'Snapshot konten & banner Beranda ikut diperbarui.',
                            () => ref
                                .read(communityRepositoryProvider)
                                .publishAnnouncement(a.id),
                            'Gagal menerbitkan ulang pengumuman',
                          ),
                        ),
                      if (canArchive)
                        OutlinedButton.icon(
                          icon: const Icon(Icons.archive_outlined),
                          label: const Text('Arsipkan'),
                          onPressed: () => _confirmAction(
                            'Arsipkan pengumuman?',
                            'Pengumuman dan banner-nya akan dinonaktifkan.',
                            () => ref
                                .read(communityRepositoryProvider)
                                .archiveAnnouncement(a.id),
                            'Gagal mengarsipkan pengumuman',
                          ),
                        ),
                      if (canDelete)
                        OutlinedButton.icon(
                          icon: const Icon(Icons.delete_outline),
                          label: const Text('Hapus'),
                          style: OutlinedButton.styleFrom(
                            foregroundColor: AppColors.danger,
                          ),
                          onPressed: () => _confirmAction(
                            'Hapus pengumuman?',
                            'Tindakan ini tidak dapat dibatalkan.',
                            () => ref
                                .read(communityRepositoryProvider)
                                .deleteAnnouncement(a.id),
                            'Gagal menghapus pengumuman',
                            popAfter: true,
                          ),
                        ),
                    ],
                  ),
                ],
                const SizedBox(height: AppSpacing.base),
                AppCard(
                  child: Padding(
                    padding: const EdgeInsets.all(AppSpacing.base),
                    child: Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        Text(
                          'ID: ${a.id}',
                          style: theme.textTheme.labelSmall?.copyWith(
                            color: AppColors.textMutedColor(context),
                          ),
                        ),
                        if (a.deepLink != null && a.deepLink!.isNotEmpty)
                          Text(
                            'Link: ${a.deepLink}',
                            style: theme.textTheme.labelSmall?.copyWith(
                              color: AppColors.textMutedColor(context),
                            ),
                          ),
                      ],
                    ),
                  ),
                ),
              ],
            ),
    );
  }
}
