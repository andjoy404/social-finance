import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/utils/permissions.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/utils/rupiah_formatter.dart';
import 'package:social_finance/core/widgets/app_badge.dart';
import 'package:social_finance/core/widgets/app_button.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/dashboard/data/community_repository.dart';
import 'package:social_finance/features/dashboard/data/models/community_models.dart';
import 'package:social_finance/core/widgets/accent_action_fab.dart';
import 'package:social_finance/features/kegiatan/presentation/screens/kegiatan_form_screen.dart';

class KegiatanScreen extends ConsumerStatefulWidget {
  final bool loadInTest;

  const KegiatanScreen({super.key, this.loadInTest = false});

  @override
  ConsumerState<KegiatanScreen> createState() => _KegiatanScreenState();
}

class _KegiatanScreenState extends ConsumerState<KegiatanScreen> {
  String _selectedFilter =
      'all'; // all, upcoming, ongoing, completed, cancelled, archived
  List<CommunityActivity> _activities = [];
  bool _isLoading = true;
  bool _isLoadingMore = false;
  int _page = 1;
  int _totalPages = 1;
  String? _error;

  @override
  void initState() {
    super.initState();
    _loadActivities();
  }

  String? get _statusParam {
    switch (_selectedFilter) {
      case 'all':
        return null;
      case 'upcoming':
        return 'upcoming';
      case 'ongoing':
        return 'ongoing';
      case 'completed':
        return 'completed';
      case 'cancelled':
        return 'cancelled';
      case 'archived':
        return 'archived';
      default:
        return null;
    }
  }

  Future<void> _loadActivities({int page = 1, bool append = false}) async {
    if (!widget.loadInTest && Platform.environment.containsKey('FLUTTER_TEST')) {
      if (mounted) {
        setState(() {
          _activities = [];
          _isLoading = false;
        });
      }
      return;
    }

    if (append) {
      setState(() => _isLoadingMore = true);
    } else {
      setState(() {
        _isLoading = true;
        _error = null;
      });
    }

    try {
      final repo = ref.read(communityRepositoryProvider);
      final result = await repo.listActivities(
        status: _statusParam,
        page: page,
        pageSize: 20,
      );
      if (mounted) {
        setState(() {
          _activities = append
              ? [..._activities, ...result.items]
              : result.items;
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

  List<CommunityActivity> get _filteredActivities => _activities;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;
    final authState = ref.watch(authRepositoryProvider);
    final user = authState?.valueOrNull;
    final canManage = canManageActivities(user);

    return Scaffold(
      appBar: AppBar(
        automaticallyImplyLeading: false,
        centerTitle: false,
        title: const MenuAppBarTitle(title: 'Kegiatan Warga'),
      ),
      floatingActionButton: canManage
          ? AccentActionFab(
              tooltip: 'Tambah Kegiatan',
              icon: Icons.event_outlined,
              onPressed: () => _showCreateActivityDialog(context),
            )
          : null,
      body: RefreshIndicator(
        onRefresh: _loadActivities,
        child: Column(
          children: [
            // Filter bar
            SingleChildScrollView(
              scrollDirection: Axis.horizontal,
              padding: const EdgeInsets.symmetric(
                horizontal: AppSpacing.base,
                vertical: 8,
              ),
              child: Row(
                children: [
                  _buildFilterChip('Semua', 'all', isDark),
                  const SizedBox(width: 8),
                  _buildFilterChip('Mendatang', 'upcoming', isDark),
                  const SizedBox(width: 8),
                  _buildFilterChip('Berlangsung', 'ongoing', isDark),
                  const SizedBox(width: 8),
                  _buildFilterChip('Selesai', 'completed', isDark),
                  const SizedBox(width: 8),
                  _buildFilterChip('Dibatalkan', 'cancelled', isDark),
                  const SizedBox(width: 8),
                  _buildFilterChip('Diarsipkan', 'archived', isDark),
                ],
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
                            Text(_error!, textAlign: TextAlign.center),
                            const SizedBox(height: 12),
                            ElevatedButton(
                              onPressed: _loadActivities,
                              child: const Text('Coba Lagi'),
                            ),
                          ],
                        ),
                      ),
                    )
                  : _filteredActivities.isEmpty
                  ? Center(
                      child: Column(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Icon(
                            Icons.event_busy_outlined,
                            size: 48,
                            color: AppColors.textMutedColor(context),
                          ),
                          const SizedBox(height: 12),
                          Text(
                            'Belum ada kegiatan.',
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
                      itemCount:
                          _filteredActivities.length +
                          (_page < _totalPages ? 1 : 0),
                      itemBuilder: (context, index) {
                        if (index == _filteredActivities.length) {
                          return Center(
                            child: Padding(
                              padding: const EdgeInsets.only(bottom: 12),
                              child: OutlinedButton(
                                onPressed: _isLoadingMore
                                    ? null
                                    : () => _loadActivities(
                                        page: _page + 1,
                                        append: true,
                                      ),
                                child: Text(
                                  _isLoadingMore
                                      ? 'Memuat...'
                                      : 'Muat Lebih Banyak',
                                ),
                              ),
                            ),
                          );
                        }
                        final act = _filteredActivities[index];
                        return _buildActivityCard(
                          act,
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

  Widget _buildFilterChip(String label, String value, bool isDark) {
    final isSelected = _selectedFilter == value;
    return ChoiceChip(
      label: Text(label),
      selected: isSelected,
      onSelected: (_) {
        setState(() {
          _selectedFilter = value;
        });
        _loadActivities();
      },
      selectedColor: AppColors.accent.withValues(alpha: 0.2),
      labelStyle: TextStyle(
        color: isSelected ? AppColors.accent : null,
        fontWeight: isSelected ? FontWeight.bold : FontWeight.normal,
      ),
    );
  }

  Widget _buildActivityCard(
    CommunityActivity act,
    ThemeData theme,
    bool isDark,
    bool canManage,
  ) {
    final mutedColor = isDark
        ? AppColors.darkTextMuted
        : AppColors.lightTextMuted;
    final scheduleLine =
        '${_formatDateTime(act.startDate)}${act.endDate != null ? ' – ${_formatDateTime(act.endDate!)}' : ''}${act.location.isNotEmpty ? ' • ${act.location}' : ''}';
    final hasExtraMeta =
        (act.organizer != null && act.organizer!.isNotEmpty) ||
        (act.picName != null && act.picName!.isNotEmpty) ||
        (act.budgetPlanned != null && act.budgetPlanned!.isNotEmpty);

    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: AppCard(
        padding: EdgeInsets.zero,
        child: InkWell(
          onTap: () async {
            await context.push('/home/kegiatan/${act.id}');
            if (mounted) _loadActivities();
          },
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
                        act.title,
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
                    _buildStatusBadge(act),
                    if (act.publicationStatus == 'draft' &&
                        act.status != 'cancelled')
                      const AppBadge(
                        label: 'Belum Terbit',
                        backgroundColor: Color(0x22F59E0B),
                        textColor: Colors.amber,
                      ),
                    if (act.showOnBanner)
                      AppBadge(
                        label: 'Banner Beranda',
                        backgroundColor: AppColors.accent.withValues(
                          alpha: 0.15,
                        ),
                        textColor: AppColors.accent,
                      ),
                    if (act.posterUrl != null && act.posterUrl!.isNotEmpty)
                      AppBadge(
                        label: 'Gambar Terlampir',
                        backgroundColor: AppColors.infoBlue.withValues(alpha: 0.15),
                        textColor: AppColors.infoBlue,
                      ),
                    if (act.archivedAt != null)
                      AppBadge(
                        label: 'Diarsipkan',
                        backgroundColor: AppColors.lightTextMuted.withValues(alpha: 0.15),
                        textColor: AppColors.lightTextMuted,
                      ),
                  ],
                ),
                const SizedBox(height: 8),
                Text(
                  scheduleLine,
                  style: theme.textTheme.bodyMedium?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
                ),
                if (act.description.isNotEmpty) ...[
                  const SizedBox(height: 2),
                  Text(
                    act.description,
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: mutedColor,
                    ),
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                ],
                if (hasExtraMeta) ...[
                  const SizedBox(height: 6),
                  Wrap(
                    spacing: 12,
                    runSpacing: 4,
                    children: [
                      if (act.organizer != null && act.organizer!.isNotEmpty)
                        Text(
                          'Penyelenggara: ${act.organizer}',
                          style: theme.textTheme.labelSmall?.copyWith(
                            color: mutedColor,
                          ),
                        ),
                      if (act.picName != null && act.picName!.isNotEmpty)
                        Text(
                          'PIC: ${act.picName}',
                          style: theme.textTheme.labelSmall?.copyWith(
                            color: mutedColor,
                          ),
                        ),
                      if (act.budgetPlanned != null &&
                          act.budgetPlanned!.isNotEmpty)
                        Text(
                          'Anggaran: ${RupiahFormatter.format(int.tryParse(act.budgetPlanned!) ?? 0)}',
                          style: theme.textTheme.labelSmall?.copyWith(
                            color: AppColors.accent,
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                    ],
                  ),
                ],
                const SizedBox(height: 12),
                Row(
                  mainAxisAlignment: MainAxisAlignment.end,
                  children: [
                    IconButton(
                      icon: const Icon(
                        Icons.visibility_outlined,
                        size: 20,
                        color: AppColors.infoBlue,
                      ),
                      tooltip: 'Detail',
                      onPressed: () => context.push('/home/kegiatan/${act.id}'),
                    ),
                    if (canManage) ...[
                      IconButton(
                        icon: const Icon(
                          Icons.edit_outlined,
                          size: 20,
                          color: AppColors.accent,
                        ),
                        tooltip: 'Ubah',
                        onPressed: () => _showEditActivityDialog(context, act),
                      ),
                      if (act.publicationStatus == 'draft')
                        IconButton(
                          icon: const Icon(
                            Icons.delete_outline,
                            size: 20,
                            color: AppColors.danger,
                          ),
                          tooltip: 'Hapus',
                          onPressed: () =>
                              _showDeleteConfirmationDialog(context, act),
                        ),
                      if (act.archivedAt == null)
                        IconButton(
                          icon: const Icon(
                            Icons.archive_outlined,
                            size: 20,
                            color: AppColors.warning,
                          ),
                          tooltip: 'Arsipkan',
                          onPressed: () => _confirmArchiveDialog(context, act),
                        ),
                      if (act.archivedAt != null)
                        IconButton(
                          icon: const Icon(
                            Icons.unarchive_outlined,
                            size: 20,
                            color: AppColors.success,
                          ),
                          tooltip: 'Buka dari Arsip',
                          onPressed: () => _confirmUnarchiveDialog(context, act),
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

  Widget _buildStatusBadge(CommunityActivity act) {
    if (act.status == 'cancelled') {
      return const AppBadge(
        label: 'Dibatalkan',
        backgroundColor: Color(0x22EF4444),
        textColor: AppColors.danger,
      );
    }
    if (act.publicationStatus == 'draft') {
      return const AppBadge(
        label: 'Draft',
        backgroundColor: Color(0x226B7280),
        textColor: Colors.grey,
      );
    }
    switch (act.status) {
      case 'upcoming':
        return const AppBadge(
          label: 'Mendatang',
          backgroundColor: Color(0x223B82F6),
          textColor: Colors.blue,
        );
      case 'completed':
        return const AppBadge(
          label: 'Selesai',
          backgroundColor: Color(0x2210B981),
          textColor: AppColors.income,
        );
      default:
        return const AppBadge(
          label: 'Berlangsung',
          backgroundColor: Color(0x22F59E0B),
          textColor: Colors.amber,
        );
    }
  }

  void _showDetailSheet(CommunityActivity act, bool canManage) {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(AppRadius.lg)),
      ),
      builder: (ctx) {
        final theme = Theme.of(ctx);
        return SafeArea(
          child: Padding(
            padding: const EdgeInsets.all(AppSpacing.base),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Center(
                  child: Container(
                    width: 36,
                    height: 4,
                    decoration: BoxDecoration(
                      color: Colors.grey.shade400,
                      borderRadius: BorderRadius.circular(2),
                    ),
                  ),
                ),
                const SizedBox(height: AppSpacing.base),
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        act.title,
                        style: theme.textTheme.headlineSmall?.copyWith(
                          fontWeight: FontWeight.bold,
                        ),
                      ),
                    ),
                    _buildStatusBadge(act),
                  ],
                ),
                const SizedBox(height: 8),
                Text(
                  '${act.startDate.day} ${_formatMonthShort(act.startDate.month)} ${act.startDate.year}',
                  style: theme.textTheme.bodyMedium?.copyWith(
                    fontWeight: FontWeight.w600,
                    color: AppColors.accent,
                  ),
                ),
                const SizedBox(height: 4),
                Row(
                  children: [
                    const Icon(
                      Icons.location_on_outlined,
                      size: 16,
                      color: Colors.grey,
                    ),
                    const SizedBox(width: 4),
                    Text(act.location, style: theme.textTheme.bodyMedium),
                  ],
                ),
                if (act.description.isNotEmpty) ...[
                  const SizedBox(height: 12),
                  Text('Deskripsi:', style: theme.textTheme.titleSmall),
                  const SizedBox(height: 4),
                  Text(act.description, style: theme.textTheme.bodyMedium),
                ],
                if (act.picName != null && act.picName!.isNotEmpty) ...[
                  const SizedBox(height: 12),
                  Text(
                    'Penanggung Jawab (PIC):',
                    style: theme.textTheme.titleSmall,
                  ),
                  const SizedBox(height: 4),
                  Text(
                    '${act.picName} ${act.picPhone != null ? '(${act.picPhone})' : ''}',
                    style: theme.textTheme.bodyMedium,
                  ),
                ],
                if (act.budgetPlanned != null &&
                    act.budgetPlanned!.isNotEmpty) ...[
                  const SizedBox(height: 12),
                  Text('Rencana Anggaran:', style: theme.textTheme.titleSmall),
                  const SizedBox(height: 4),
                  Text(
                    RupiahFormatter.format(
                      int.tryParse(act.budgetPlanned!) ?? 0,
                    ),
                    style: theme.textTheme.bodyMedium?.copyWith(
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                ],
                const SizedBox(height: 16),
                if (canManage && act.publicationStatus == 'draft') ...[
                  SizedBox(
                    width: double.infinity,
                    child: ElevatedButton.icon(
                      onPressed: () async {
                        Navigator.pop(ctx);
                        try {
                          await ref
                              .read(communityRepositoryProvider)
                              .publishActivity(act.id);
                          _loadActivities();
                        } catch (e) {
                          if (mounted) {
                            ScaffoldMessenger.of(context).showSnackBar(
                              SnackBar(content: Text('Gagal publikasi: $e')),
                            );
                          }
                        }
                      },
                      icon: const Icon(Icons.publish),
                      label: const Text('Publikasikan Kegiatan'),
                    ),
                  ),
                  const SizedBox(height: 8),
                ],
                if (canManage && act.status == 'cancelled' && act.archivedAt == null)
                  const SizedBox(height: 8),
                if (canManage && act.status != 'cancelled') ...[
                  if (act.archivedAt != null)
                    SizedBox(
                      width: double.infinity,
                      child: OutlinedButton.icon(
                        onPressed: () => _confirmUnarchiveDialog(context, act),
                        icon: const Icon(Icons.unarchive_outlined),
                        label: const Text('Buka dari Arsip'),
                      ),
                    ),
                  if (act.archivedAt != null)
                    const SizedBox(height: 8),
                  SizedBox(
                    width: double.infinity,
                    child: OutlinedButton.icon(
                      onPressed: () => _showCancelConfirmationDialog(context, act),
                      icon: const Icon(Icons.cancel_outlined),
                      label: const Text('Batalkan Kegiatan'),
                      style: OutlinedButton.styleFrom(
                        foregroundColor: AppColors.danger,
                      ),
                    ),
                  ),
                  if (act.archivedAt == null)
                    const SizedBox(height: 8),
                ],
                AppButton.danger(
                  text: 'Tutup',
                  fullWidth: true,
                  onPressed: () => Navigator.pop(ctx),
                ),
              ],
            ),
          ),
        );
      },
    );
  }

  Future<void> _showCreateActivityDialog(BuildContext context) async {
    final created = await Navigator.of(context).push<bool>(
      MaterialPageRoute(
        builder: (_) => const KegiatanFormScreen(),
      ),
    );
    if (created == true) {
      _loadActivities();
    }
  }

  Future<void> _showEditActivityDialog(BuildContext context, CommunityActivity act) async {
    final updated = await Navigator.of(context).push<bool>(
      MaterialPageRoute(
        builder: (_) => KegiatanFormScreen(
          activityId: act.id,
          initialActivity: act,
        ),
      ),
    );
    if (updated == true) {
      _loadActivities();
    }
  }

  void _showDeleteConfirmationDialog(
    BuildContext context,
    CommunityActivity act,
  ) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Hapus Kegiatan?'),
        content: Text(
          'Apakah Anda yakin ingin menghapus kegiatan "${act.title}"? Tindakan ini tidak dapat dibatalkan.',
        ),
        actions: [
          ElevatedButton(
            style: ElevatedButton.styleFrom(
              backgroundColor: AppColors.danger,
              foregroundColor: Colors.white,
              elevation: 0,
            ),
            onPressed: () => Navigator.pop(ctx),
            child: const Text('Batal'),
          ),
          ElevatedButton(
            style: ElevatedButton.styleFrom(backgroundColor: AppColors.danger),
            onPressed: () async {
              final messenger = ScaffoldMessenger.of(context);
              Navigator.pop(ctx);
              try {
                await ref
                    .read(communityRepositoryProvider)
                    .deleteActivity(act.id);
                _loadActivities();
              } catch (e) {
                if (mounted) {
                  messenger.showSnackBar(
                    SnackBar(content: Text('Gagal menghapus kegiatan: $e')),
                  );
                }
              }
            },
            child: const Text('Hapus', style: TextStyle(color: Colors.white)),
          ),
        ],
      ),
    );
  }

  String _formatDateTime(DateTime dt) {
    const months = ['','Jan','Feb','Mar','Apr','Mei','Jun','Jul','Agu','Sep','Okt','Nov','Des'];
    final day = dt.day.toString().padLeft(2, '0');
    final hour = dt.hour.toString().padLeft(2, '0');
    final minute = dt.minute.toString().padLeft(2, '0');
    return '$day ${months[dt.month]} ${dt.year}, $hour:$minute';
  }

  String _formatMonthShort(int m) {
    const months = [
      '',
      'Jan',
      'Feb',
      'Mar',
      'Apr',
      'Mei',
      'Jun',
      'Jul',
      'Agu',
      'Sep',
      'Okt',
      'Nov',
      'Des',
    ];
    return m >= 1 && m <= 12 ? months[m] : '';
  }

  void _confirmArchiveDialog(BuildContext context, CommunityActivity act) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Arsipkan Kegiatan?'),
        content: Text(
          'Apakah Anda yakin ingin mengarsipkan kegiatan "${act.title}"? Kegiatan akan disembunyikan dari daftar aktif.',
        ),
        actions: [
          ElevatedButton(
            style: ElevatedButton.styleFrom(
              backgroundColor: AppColors.danger,
              foregroundColor: Colors.white,
              elevation: 0,
            ),
            onPressed: () => Navigator.pop(ctx),
            child: const Text('Batal'),
          ),
          ElevatedButton(
            style: ElevatedButton.styleFrom(backgroundColor: AppColors.warning),
            onPressed: () async {
              final messenger = ScaffoldMessenger.of(context);
              Navigator.pop(ctx);
              try {
                await ref
                    .read(communityRepositoryProvider)
                    .archiveActivity(act.id);
                _loadActivities();
              } catch (e) {
                if (mounted) {
                  messenger.showSnackBar(
                    SnackBar(content: Text('Gagal mengarsip: $e')),
                  );
                }
              }
            },
            child: const Text('Ya, Arsipkan'),
          ),
        ],
      ),
    );
  }

  void _confirmUnarchiveDialog(BuildContext context, CommunityActivity act) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Buka dari Arsip?'),
        content: Text(
          'Apakah Anda yakin ingin membuka kegiatan "${act.title}" dari arsip? Kegiatan akan kembali muncul di daftar aktif.',
        ),
        actions: [
          ElevatedButton(
            style: ElevatedButton.styleFrom(
              backgroundColor: AppColors.danger,
              foregroundColor: Colors.white,
              elevation: 0,
            ),
            onPressed: () => Navigator.pop(ctx),
            child: const Text('Batal'),
          ),
          ElevatedButton(
            style: ElevatedButton.styleFrom(backgroundColor: AppColors.success),
            onPressed: () async {
              final messenger = ScaffoldMessenger.of(context);
              Navigator.pop(ctx);
              try {
                await ref
                    .read(communityRepositoryProvider)
                    .unarchiveActivity(act.id);
                _loadActivities();
              } catch (e) {
                if (mounted) {
                  messenger.showSnackBar(
                    SnackBar(content: Text('Gagal membuka arsip: $e')),
                  );
                }
              }
            },
            child: const Text('Ya, Buka'),
          ),
        ],
      ),
    );
  }

  void _showCancelConfirmationDialog(BuildContext context, CommunityActivity act) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Batalkan Kegiatan?'),
        content: Text(
          'Apakah Anda yakin ingin membatalkan kegiatan "${act.title}"? Banner di Beranda akan dinonaktifkan.',
        ),
        actions: [
          ElevatedButton(
            style: ElevatedButton.styleFrom(
              backgroundColor: AppColors.danger,
              foregroundColor: Colors.white,
              elevation: 0,
            ),
            onPressed: () => Navigator.pop(ctx),
            child: const Text('Batal'),
          ),
          ElevatedButton(
            style: ElevatedButton.styleFrom(backgroundColor: AppColors.danger),
            onPressed: () async {
              final messenger = ScaffoldMessenger.of(context);
              Navigator.pop(ctx);
              try {
                await ref
                    .read(communityRepositoryProvider)
                    .cancelActivity(act.id);
                _loadActivities();
              } catch (e) {
                if (mounted) {
                  messenger.showSnackBar(
                    SnackBar(content: Text('Gagal membatalkan kegiatan: $e')),
                  );
                }
              }
            },
            child: const Text('Ya, Batalkan', style: TextStyle(color: Colors.white)),
          ),
        ],
      ),
    );
  }
}
