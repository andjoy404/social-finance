import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:social_finance/core/utils/permissions.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/utils/rupiah_formatter.dart';
import 'package:social_finance/core/widgets/app_badge.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/dashboard/data/community_repository.dart';
import 'package:social_finance/features/dashboard/data/models/community_models.dart';

class KegiatanScreen extends ConsumerStatefulWidget {
  const KegiatanScreen({super.key});

  @override
  ConsumerState<KegiatanScreen> createState() => _KegiatanScreenState();
}

class _KegiatanScreenState extends ConsumerState<KegiatanScreen> {
  String _selectedFilter =
      'all'; // all, upcoming, ongoing, completed, cancelled
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
      default:
        return null;
    }
  }

  Future<void> _loadActivities({int page = 1, bool append = false}) async {
    if (Platform.environment.containsKey('FLUTTER_TEST')) {
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
          ? FloatingActionButton.extended(
              onPressed: () => _showCreateActivityDialog(context),
              icon: const Icon(Icons.add),
              label: const Text('Tambah Kegiatan'),
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
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: AppCard(
        child: InkWell(
          onTap: () => _showDetailSheet(act, canManage),
          borderRadius: BorderRadius.circular(AppRadius.base),
          child: Padding(
            padding: const EdgeInsets.all(14),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Container(
                      width: 50,
                      height: 50,
                      decoration: BoxDecoration(
                        color: AppColors.accent.withValues(alpha: 0.12),
                        borderRadius: BorderRadius.circular(AppRadius.sm),
                      ),
                      child: Column(
                        mainAxisAlignment: MainAxisAlignment.center,
                        children: [
                          Text(
                            act.startDate.day.toString(),
                            style: const TextStyle(
                              fontSize: 18,
                              fontWeight: FontWeight.bold,
                              color: AppColors.accent,
                            ),
                          ),
                          Text(
                            _formatMonthShort(act.startDate.month),
                            style: const TextStyle(
                              fontSize: 10,
                              fontWeight: FontWeight.bold,
                              color: AppColors.accent,
                            ),
                          ),
                        ],
                      ),
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            act.title,
                            style: theme.textTheme.titleMedium?.copyWith(
                              fontWeight: FontWeight.bold,
                            ),
                          ),
                          const SizedBox(height: 4),
                          Row(
                            children: [
                              Icon(
                                Icons.location_on_outlined,
                                size: 14,
                                color: AppColors.textMutedColor(context),
                              ),
                              const SizedBox(width: 4),
                              Expanded(
                                child: Text(
                                  act.location,
                                  style: theme.textTheme.bodySmall?.copyWith(
                                    color: AppColors.textMutedColor(context),
                                  ),
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                ),
                              ),
                            ],
                          ),
                        ],
                      ),
                    ),
                    _buildStatusBadge(act),
                  ],
                ),
                if (act.description.isNotEmpty) ...[
                  const SizedBox(height: 8),
                  Text(
                    act.description,
                    style: theme.textTheme.bodySmall,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                ],
                const SizedBox(height: 8),
                Row(
                  children: [
                    if (act.picName != null && act.picName!.isNotEmpty) ...[
                      const Icon(
                        Icons.person_outline,
                        size: 14,
                        color: Colors.grey,
                      ),
                      const SizedBox(width: 4),
                      Text(
                        'PIC: ${act.picName}',
                        style: const TextStyle(
                          fontSize: 11,
                          color: Colors.grey,
                        ),
                      ),
                      const Spacer(),
                    ],
                    if (act.budgetPlanned != null &&
                        act.budgetPlanned!.isNotEmpty) ...[
                      Text(
                        'Anggaran: ${RupiahFormatter.format(int.tryParse(act.budgetPlanned!) ?? 0)}',
                        style: const TextStyle(
                          fontSize: 11,
                          fontWeight: FontWeight.w600,
                          color: AppColors.accent,
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
                if (canManage && act.status != 'cancelled') ...[
                  SizedBox(
                    width: double.infinity,
                    child: OutlinedButton.icon(
                      onPressed: () async {
                        Navigator.pop(ctx);
                        try {
                          await ref
                              .read(communityRepositoryProvider)
                              .cancelActivity(act.id);
                          _loadActivities();
                        } catch (e) {
                          if (mounted) {
                            ScaffoldMessenger.of(context).showSnackBar(
                              SnackBar(content: Text('Gagal membatalkan: $e')),
                            );
                          }
                        }
                      },
                      icon: const Icon(Icons.cancel_outlined),
                      label: const Text('Batalkan Kegiatan'),
                      style: OutlinedButton.styleFrom(
                        foregroundColor: AppColors.danger,
                      ),
                    ),
                  ),
                  const SizedBox(height: 8),
                ],
                SizedBox(
                  width: double.infinity,
                  child: OutlinedButton(
                    onPressed: () => Navigator.pop(ctx),
                    child: const Text('Tutup'),
                  ),
                ),
              ],
            ),
          ),
        );
      },
    );
  }

  void _showCreateActivityDialog(BuildContext context) {
    final titleCtrl = TextEditingController();
    final descCtrl = TextEditingController();
    final locationCtrl = TextEditingController();
    final picNameCtrl = TextEditingController();
    final budgetCtrl = TextEditingController();
    DateTime startDate = DateTime.now().add(const Duration(days: 3));
    bool showOnBanner = false;

    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(AppRadius.lg)),
      ),
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setModalState) {
          return Padding(
            padding: EdgeInsets.fromLTRB(
              AppSpacing.base,
              AppSpacing.base,
              AppSpacing.base,
              MediaQuery.of(ctx).viewInsets.bottom + AppSpacing.base,
            ),
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Tambah Kegiatan Baru',
                    style: Theme.of(ctx).textTheme.titleLarge,
                  ),
                  const SizedBox(height: 12),
                  TextField(
                    controller: titleCtrl,
                    decoration: const InputDecoration(
                      labelText: 'Judul Kegiatan *',
                      border: OutlineInputBorder(),
                    ),
                  ),
                  const SizedBox(height: 10),
                  TextField(
                    controller: locationCtrl,
                    decoration: const InputDecoration(
                      labelText: 'Lokasi *',
                      border: OutlineInputBorder(),
                    ),
                  ),
                  const SizedBox(height: 10),
                  TextField(
                    controller: descCtrl,
                    maxLines: 2,
                    decoration: const InputDecoration(
                      labelText: 'Deskripsi',
                      border: OutlineInputBorder(),
                    ),
                  ),
                  const SizedBox(height: 10),
                  TextField(
                    controller: picNameCtrl,
                    decoration: const InputDecoration(
                      labelText: 'Nama Penanggung Jawab (PIC)',
                      border: OutlineInputBorder(),
                    ),
                  ),
                  const SizedBox(height: 10),
                  TextField(
                    controller: budgetCtrl,
                    keyboardType: TextInputType.number,
                    decoration: const InputDecoration(
                      labelText: 'Rencana Anggaran (Rp)',
                      border: OutlineInputBorder(),
                    ),
                  ),
                  const SizedBox(height: 10),
                  SwitchListTile(
                    title: const Text('Tampilkan di Banner Beranda'),
                    value: showOnBanner,
                    onChanged: (val) {
                      setModalState(() {
                        showOnBanner = val;
                      });
                    },
                  ),
                  const SizedBox(height: 12),
                  SizedBox(
                    width: double.infinity,
                    child: ElevatedButton(
                      onPressed: () async {
                        if (titleCtrl.text.trim().isEmpty ||
                            locationCtrl.text.trim().isEmpty) {
                          ScaffoldMessenger.of(context).showSnackBar(
                            const SnackBar(
                              content: Text('Judul dan lokasi wajib diisi'),
                            ),
                          );
                          return;
                        }
                        final messenger = ScaffoldMessenger.of(context);
                        Navigator.pop(ctx);
                        try {
                          await ref
                              .read(communityRepositoryProvider)
                              .createActivity({
                                'title': titleCtrl.text.trim(),
                                'location': locationCtrl.text.trim(),
                                'description': descCtrl.text.trim(),
                                'pic_name': picNameCtrl.text.trim(),
                                'budget_planned': budgetCtrl.text.trim(),
                                'start_date': startDate.toIso8601String(),
                                'show_on_banner': showOnBanner,
                                'publication_status': 'draft',
                              });
                          _loadActivities();
                        } catch (e) {
                          if (mounted) {
                            messenger.showSnackBar(
                              SnackBar(
                                content: Text('Gagal membuat kegiatan: $e'),
                              ),
                            );
                          }
                        }
                      },
                      child: const Text('Simpan'),
                    ),
                  ),
                ],
              ),
            ),
          );
        },
      ),
    );
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
}
