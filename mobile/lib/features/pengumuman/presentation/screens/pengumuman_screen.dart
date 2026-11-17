import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/widgets/app_badge.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import 'package:social_finance/core/utils/permissions.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/dashboard/data/community_repository.dart';
import 'package:social_finance/features/dashboard/data/models/community_models.dart';
import 'package:social_finance/core/widgets/accent_action_fab.dart';

import 'announcement_ui.dart';

class PengumumanScreen extends ConsumerStatefulWidget {
  final bool loadInTest;

  const PengumumanScreen({super.key, this.loadInTest = false});

  @override
  ConsumerState<PengumumanScreen> createState() => _PengumumanScreenState();
}

class _PengumumanScreenState extends ConsumerState<PengumumanScreen> {
  List<CommunityAnnouncement> _items = [];
  bool _isLoading = true;
  bool _isLoadingMore = false;
  int _page = 1;
  int _totalPages = 1;
  String? _error;
  String? _actionError;
  String _statusFilter = 'all'; // warga never uses this (server forces active)

  @override
  void initState() {
    super.initState();
    if (!_isFlutterTest) {
      _load(page: 1);
    } else {
      _isLoading = false;
    }
  }

  // Widget tests run offline; never hit the network unless loadInTest is true.
  bool get _isFlutterTest =>
      !widget.loadInTest && Platform.environment.containsKey('FLUTTER_TEST');

  String? get _statusParam {
    switch (_statusFilter) {
      case 'all':
        return null;
      case 'active':
        return 'active';
      case 'draft':
        return 'draft';
      case 'scheduled':
        return 'scheduled';
      case 'archived':
        return 'archived';
      case 'history':
        return 'history';
      default:
        return null;
    }
  }

  Future<void> _load({int page = 1, bool append = false}) async {
    setState(() {
      if (append) {
        _isLoadingMore = true;
      } else {
        _isLoading = true;
        _error = null;
      }
    });

    try {
      final repo = ref.read(communityRepositoryProvider);
      final result = await repo.listAnnouncements(
        status: _statusParam,
        page: page,
        pageSize: 10,
      );
      if (mounted) {
        setState(() {
          _items = append ? [..._items, ...result.items] : result.items;
          _page = result.page;
          _totalPages = result.totalPages;
          _isLoading = false;
          _isLoadingMore = false;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _error = e.toString();
          _isLoading = false;
          _isLoadingMore = false;
        });
      }
    }
  }


  String? _formatWindow(CommunityAnnouncement a) =>
      scheduleWindowLabel(a.scheduledStart, a.scheduledEnd);

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
    String errorMessage,
  ) async {
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
      await _action(action, errorMessage);
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;
    final authState = ref.watch(authRepositoryProvider);
    final user = authState?.valueOrNull;
    final canManage = canManageAnnouncements(user);

    return Scaffold(
      appBar: AppBar(
        automaticallyImplyLeading: false,
        centerTitle: false,
        title: const MenuAppBarTitle(title: 'Pengumuman'),
      ),
      floatingActionButton: canManage
          ? AccentActionFab(
              tooltip: 'Tambah Pengumuman',
              icon: Icons.campaign_outlined,
              onPressed: () => context.push('/home/pengumuman/create'),
            )
          : null,
      body: RefreshIndicator(
        onRefresh: () => _load(),
        child: Column(
          children: [
            SingleChildScrollView(
              scrollDirection: Axis.horizontal,
              padding: const EdgeInsets.symmetric(
                horizontal: AppSpacing.base,
                vertical: 8,
              ),
              child: Row(
                children: [
                  for (final entry in (canManage
                      ? const [
                          ('Semua', 'all'),
                          ('Aktif', 'active'),
                          ('Draf', 'draft'),
                          ('Terjadwal', 'scheduled'),
                          ('Diarsipkan', 'archived'),
                        ]
                      : const [
                          ('Semua', 'all'),
                          ('Aktif', 'active'),
                          ('Riwayat', 'history'),
                        ])) ...[
                    ChoiceChip(
                      label: Text(entry.$1),
                      selected: _statusFilter == entry.$2,
                      onSelected: (_) {
                        setState(() => _statusFilter = entry.$2);
                        _load();
                      },
                    ),
                    const SizedBox(width: 8),
                  ],
                ],
              ),
            ),
            if (_actionError != null)
              Padding(
                padding: const EdgeInsets.symmetric(
                  horizontal: AppSpacing.base,
                ),
                child: Container(
                  width: double.infinity,
                  padding: const EdgeInsets.all(12),
                  margin: const EdgeInsets.only(bottom: 8),
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
              ),
            Expanded(
              child: _isLoading
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
                              'Gagal memuat pengumuman',
                              textAlign: TextAlign.center,
                              style: theme.textTheme.titleMedium,
                            ),
                            const SizedBox(height: 4),
                            Text(_error!, textAlign: TextAlign.center),
                            const SizedBox(height: 12),
                            ElevatedButton(
                              onPressed: () => _load(),
                              child: const Text('Coba Lagi'),
                            ),
                          ],
                        ),
                      ),
                    )
                  : _items.isEmpty
                  ? Center(
                      child: Column(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Icon(
                            Icons.campaign_outlined,
                            size: 48,
                            color: AppColors.textMutedColor(context),
                          ),
                          const SizedBox(height: 12),
                          Text(
                            canManage
                                ? 'Belum ada pengumuman.'
                                : 'Belum ada pengumuman dari pengurus.',
                            style: theme.textTheme.bodyMedium?.copyWith(
                              color: AppColors.textMutedColor(context),
                            ),
                          ),
                        ],
                      ),
                    )
                  : ListView.builder(
                      physics: const AlwaysScrollableScrollPhysics(),
                      padding: const EdgeInsets.all(AppSpacing.base),
                      itemCount: _items.length + (_page < _totalPages ? 1 : 0),
                      itemBuilder: (context, index) {
                        if (index == _items.length) {
                          return Center(
                            child: Padding(
                              padding: const EdgeInsets.only(bottom: 12),
                              child: OutlinedButton(
                                onPressed: _isLoadingMore
                                    ? null
                                    : () =>
                                          _load(page: _page + 1, append: true),
                                child: Text(
                                  _isLoadingMore
                                      ? 'Memuat...'
                                      : 'Muat Lebih Banyak',
                                ),
                              ),
                            ),
                          );
                        }
                        return _buildCard(
                          _items[index],
                          theme,
                          isDark,
                          canManage,
                        );
                      },
                    ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildCard(
    CommunityAnnouncement a,
    ThemeData theme,
    bool isDark,
    bool canManage,
  ) {
    final window = _formatWindow(a);
    final mutedColor = isDark
        ? AppColors.darkTextMuted
        : AppColors.lightTextMuted;

    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: AppCard(
        padding: EdgeInsets.zero,
        child: InkWell(
          onTap: () => context.push('/home/pengumuman/${a.id}'),
          borderRadius: BorderRadius.circular(AppRadius.base),
          child: Padding(
            padding: const EdgeInsets.all(AppSpacing.base),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Expanded(
                      child: Text(
                        a.title,
                        style: theme.textTheme.titleMedium?.copyWith(
                          fontWeight: FontWeight.bold,
                        ),
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 6),
                Wrap(
                  spacing: 6,
                  runSpacing: 6,
                  children: [
                    announcementStatusBadge(a.status),
                    if (a.priority >= 10)
                      AppBadge(
                        label: 'Penting',
                        backgroundColor: AppColors.warning.withValues(
                          alpha: 0.15,
                        ),
                        textColor: AppColors.warning,
                      ),
                    if (a.imageUrl != null && a.imageUrl!.isNotEmpty)
                      AppBadge(
                        label: 'Gambar Terlampir',
                        backgroundColor: AppColors.info.withValues(alpha: 0.15),
                        textColor: AppColors.info,
                      ),
                    if (a.deepLink != null && a.deepLink!.isNotEmpty)
                      AppBadge(
                        label: a.deepLink!,
                        backgroundColor: AppColors.accent.withValues(
                          alpha: 0.15,
                        ),
                        textColor: AppColors.accent,
                      ),
                  ],
                ),
                if (window != null || a.body.isNotEmpty)
                  const SizedBox(height: 8),
                if (window != null) ...[
                  Text(
                    window,
                    style: theme.textTheme.bodyMedium?.copyWith(
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                  if (a.body.isNotEmpty) const SizedBox(height: 2),
                ],
                if (a.body.isNotEmpty)
                  Text(
                    a.body,
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: mutedColor,
                    ),
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                const SizedBox(height: 6),
                Text(
                  'Prioritas: ${a.priority}',
                  style: theme.textTheme.labelSmall?.copyWith(
                    color: mutedColor,
                  ),
                ),
                const SizedBox(height: 12),
                Row(
                  mainAxisAlignment: MainAxisAlignment.end,
                  children: [
                    IconButton(
                      icon: const Icon(
                        Icons.visibility_outlined,
                        size: 20,
                        color: AppColors.info,
                      ),
                      tooltip: 'Detail',
                      onPressed: () => context.push('/home/pengumuman/${a.id}'),
                    ),
                    if (canManage) ...[
                      IconButton(
                        icon: const Icon(
                          Icons.edit_outlined,
                          size: 20,
                          color: AppColors.accent,
                        ),
                        tooltip: 'Ubah',
                        onPressed: () async {
                          final changed = await context.push<bool>(
                            '/home/pengumuman/${a.id}/edit',
                          );
                          if (changed == true && mounted) {
                            await _load();
                          }
                        },
                      ),
                      if (a.status == 'draft')
                        IconButton(
                          icon: const Icon(
                            Icons.publish,
                            size: 20,
                            color: AppColors.success,
                          ),
                          tooltip: 'Publikasikan',
                          onPressed: () => _confirmAction(
                            'Publikasikan pengumuman?',
                            'Pengumuman akan langsung tayang di Beranda.',
                            () => ref
                                .read(communityRepositoryProvider)
                                .publishAnnouncement(a.id),
                            'Gagal mempublikasikan pengumuman',
                          ),
                        ),
                      if (a.status == 'active' ||
                          a.status == 'scheduled' ||
                          a.status == 'archived')
                        IconButton(
                          icon: const Icon(
                            Icons.refresh,
                            size: 20,
                            color: AppColors.accent,
                          ),
                          tooltip: 'Terbitkan Ulang',
                          onPressed: () => _confirmAction(
                            'Terbitkan ulang?',
                            'Snapshot konten & banner Beranda ikut diperbarui.',
                            () => ref
                                .read(communityRepositoryProvider)
                                .publishAnnouncement(a.id),
                            'Gagal menerbitkan ulang pengumuman',
                          ),
                        ),
                      if (a.status == 'active' || a.status == 'scheduled')
                        IconButton(
                          icon: const Icon(
                            Icons.archive_outlined,
                            size: 20,
                            color: AppColors.warning,
                          ),
                          tooltip: 'Arsipkan',
                          onPressed: () => _confirmAction(
                            'Arsipkan pengumuman?',
                            'Pengumuman dan banner-nya akan dinonaktifkan.',
                            () => ref
                                .read(communityRepositoryProvider)
                                .archiveAnnouncement(a.id),
                            'Gagal mengarsipkan pengumuman',
                          ),
                        ),
                      if (a.status == 'draft')
                        IconButton(
                          icon: const Icon(
                            Icons.delete_outline,
                            size: 20,
                            color: AppColors.danger,
                          ),
                          tooltip: 'Hapus',
                          onPressed: () => _confirmAction(
                            'Hapus pengumuman?',
                            'Tindakan ini tidak dapat dibatalkan.',
                            () => ref
                                .read(communityRepositoryProvider)
                                .deleteAnnouncement(a.id),
                            'Gagal menghapus pengumuman',
                          ),
                        ),
                    ],
                  ],
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
