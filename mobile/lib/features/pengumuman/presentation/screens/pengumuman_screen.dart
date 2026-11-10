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

class PengumumanScreen extends ConsumerStatefulWidget {
  const PengumumanScreen({super.key});

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

  // Widget tests run offline; never hit the network.
  bool get _isFlutterTest => Platform.environment.containsKey('FLUTTER_TEST');

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

  Future<void> _runAction(
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

  String? _formatWindow(CommunityAnnouncement a) =>
      scheduleWindowLabel(a.scheduledStart, a.scheduledEnd);

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
          ? FloatingActionButton.extended(
              onPressed: () => context.push('/home/pengumuman/create'),
              icon: const Icon(Icons.add),
              label: const Text('Tambah Pengumuman'),
            )
          : null,
      body: RefreshIndicator(
        onRefresh: () => _load(),
        child: Column(
          children: [
            if (canManage)
              SingleChildScrollView(
                scrollDirection: Axis.horizontal,
                padding: const EdgeInsets.symmetric(
                  horizontal: AppSpacing.base,
                  vertical: 8,
                ),
                child: Row(
                  children: [
                    for (final entry in const [
                      ('Semua', 'all'),
                      ('Aktif', 'active'),
                      ('Draf', 'draft'),
                      ('Terjadwal', 'scheduled'),
                      ('Diarsipkan', 'archived'),
                    ]) ...[
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
    final isDraft = a.status == 'draft';
    final canRepublish =
        a.status == 'active' ||
        a.status == 'scheduled' ||
        a.status == 'archived';

    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: AppCard(
        child: InkWell(
          onTap: () => context.push('/home/pengumuman/${a.id}'),
          borderRadius: BorderRadius.circular(AppRadius.base),
          child: Padding(
            padding: const EdgeInsets.all(14),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (a.imageUrl != null && a.imageUrl!.isNotEmpty) ...[
                  ClipRRect(
                    borderRadius: BorderRadius.circular(AppRadius.sm),
                    child: Container(
                      width: 72,
                      height: 72,
                      color: isDark
                          ? AppColors.darkSurface
                          : AppColors.lightSurfaceSubtle,
                      alignment: Alignment.center,
                      child: CommunityImage(
                        imageRef: a.imageUrl!,
                        width: 72,
                        height: 72,
                        fit: BoxFit.contain,
                        alt: 'Gambar pengumuman ${a.title}',
                      ),
                    ),
                  ),
                  const SizedBox(width: 12),
                ],
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          announcementStatusBadge(a.status),
                          const Spacer(),
                          if (a.priority >= 10)
                            const Row(
                              children: [
                                Icon(Icons.star, size: 12, color: Colors.amber),
                                SizedBox(width: 4),
                                Text(
                                  'Penting',
                                  style: TextStyle(
                                    fontSize: 10,
                                    color: Colors.amber,
                                  ),
                                ),
                              ],
                            ),
                        ],
                      ),
                      const SizedBox(height: 8),
                      Text(
                        a.title,
                        style: theme.textTheme.titleMedium?.copyWith(
                          fontWeight: FontWeight.bold,
                        ),
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                      ),
                      const SizedBox(height: 4),
                      Text(
                        a.body,
                        style: theme.textTheme.bodySmall?.copyWith(
                          color: AppColors.textMutedColor(context),
                        ),
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                      ),
                      if (window != null) ...[
                        const SizedBox(height: 6),
                        Text(
                          window,
                          style: theme.textTheme.labelSmall?.copyWith(
                            color: AppColors.textMutedColor(context),
                          ),
                        ),
                      ],
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
