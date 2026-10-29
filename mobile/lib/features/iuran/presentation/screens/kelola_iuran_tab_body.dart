import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/widgets/app_badge.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/app_text_field.dart';
import 'package:social_finance/core/widgets/empty_state.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/iuran/data/iuran_access.dart';
import 'package:social_finance/features/iuran/data/iuran_api_models.dart';
import 'package:social_finance/features/iuran/data/iuran_models.dart';
import 'package:social_finance/features/iuran/data/iuran_providers.dart';
import 'package:social_finance/features/iuran/data/iuran_repository.dart';

/// The body widget for the Kelola Iuran tab (to be used inside TabBarView).
class KelolaIuranTabBody extends ConsumerStatefulWidget {
  const KelolaIuranTabBody({super.key});

  @override
  ConsumerState<KelolaIuranTabBody> createState() =>
      _KelolaIuranTabBodyState();
}

class _KelolaIuranTabBodyState extends ConsumerState<KelolaIuranTabBody> {
  static const _monthNames = [
    'Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni',
    'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember',
  ];

  late final TextEditingController _searchController;
  DateTime _activeDate = DateTime.now();
  String _filterJenis = 'all';
  String _filterFrekuensi = 'all';
  bool _filtersExpanded = false;
  bool _isYearFilterActive = false; // Track if "Semua Bulan" is active

  @override
  void initState() {
    super.initState();
    _searchController = TextEditingController();
  }

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  String _formatMonthYear(DateTime dt) =>
      '${_monthNames[dt.month - 1]} ${dt.year}';

  void _prevMonth() => setState(() {
        _activeDate = DateTime(_activeDate.year, _activeDate.month - 1, 1);
      });

  void _nextMonth() => setState(() {
        _activeDate = DateTime(_activeDate.year, _activeDate.month + 1, 1);
      });

  Future<void> _pickMonth() async {
    final currentYear = DateTime.now().year;
    int selYear = _activeDate.year;
    int selMonth = _activeDate.month;

    return await showModalBottomSheet<void>(
      context: context,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(AppRadius.base)),
      ),
      builder: (ctx) => StatefulBuilder(
        builder: (_, setModalState) {
          final isDark = Theme.of(ctx).brightness == Brightness.dark;
          final months = ['Semua Bulan', ...List.generate(12, (i) => _monthNames[i])];

          return Padding(
            padding: EdgeInsets.fromLTRB(
              AppSpacing.base,
              AppSpacing.base,
              AppSpacing.base,
              MediaQuery.of(ctx).viewInsets.bottom + AppSpacing.base,
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Pilih Bulan',
                  style: Theme.of(ctx).textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.w600,
                  ),
                ),
                const SizedBox(height: AppSpacing.sm),
                GridView.builder(
                  shrinkWrap: true,
                  physics: const NeverScrollableScrollPhysics(),
                  gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
                    crossAxisCount: 4,
                    childAspectRatio: 2.5,
                    crossAxisSpacing: 8,
                    mainAxisSpacing: 8,
                  ),
                  itemCount: 13,
                  itemBuilder: (_, i) {
                    final isAll = i == 0;
                    final isSelected = isAll ? _isYearFilterActive : (i == selMonth && !_isYearFilterActive);
                    return InkWell(
                      onTap: () {
                        setState(() {
                          if (isAll) {
                            _isYearFilterActive = true;
                            // Clear all month filters to show all data
                            _filterJenis = 'all';
                            _filterFrekuensi = 'all';
                          } else {
                            _isYearFilterActive = false;
                            _activeDate = DateTime(selYear, i + 1, 1);
                          }
                        });
                        ref.read(billsProvider.notifier).refresh();
                        if (ctx.mounted) Navigator.of(ctx).pop();
                      },
                      borderRadius: BorderRadius.circular(AppRadius.base),
                      child: Container(
                        decoration: BoxDecoration(
                          color: isSelected
                              ? (isDark ? AppColors.darkAccentSoft : AppColors.lightAccentSoft)
                              : (isDark ? AppColors.darkSurfaceHover : AppColors.lightSurfaceHover),
                          borderRadius: BorderRadius.circular(AppRadius.base),
                          border: Border.all(
                            color: isSelected ? AppColors.accent : (isDark ? AppColors.darkBorder : AppColors.lightBorder),
                            width: isSelected ? 2 : 1,
                          ),
                        ),
                        alignment: Alignment.center,
                        child: Text(
                          months[i],
                          style: TextStyle(
                            color: isSelected ? AppColors.accent : (isDark ? AppColors.darkText : AppColors.lightText),
                            fontWeight: isSelected ? FontWeight.w600 : null,
                          ),
                        ),
                      ),
                    );
                  },
                ),
              ],
            ),
          );
        },
      ),
    );
  }

  Future<void> _pickYear() async {
    final currentYear = DateTime.now().year;
    int selYear = _activeDate.year;

    return await showModalBottomSheet<void>(
      context: context,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(AppRadius.base)),
      ),
      builder: (ctx) => StatefulBuilder(
        builder: (_, setModalState) {
          final isDark = Theme.of(ctx).brightness == Brightness.dark;
          final years = List.generate(15, (i) => currentYear - 7 + i);

          return Padding(
            padding: EdgeInsets.fromLTRB(
              AppSpacing.base,
              AppSpacing.base,
              AppSpacing.base,
              MediaQuery.of(ctx).viewInsets.bottom + AppSpacing.base,
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Pilih Tahun',
                  style: Theme.of(ctx).textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.w600,
                  ),
                ),
                const SizedBox(height: AppSpacing.sm),
                GridView.builder(
                  shrinkWrap: true,
                  physics: const NeverScrollableScrollPhysics(),
                  gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
                    crossAxisCount: 4,
                    childAspectRatio: 2.0,
                    crossAxisSpacing: 8,
                    mainAxisSpacing: 8,
                  ),
                  itemCount: years.length,
                  itemBuilder: (_, i) {
                    final year = years[i];
                    final isSelected = year == selYear;
                    return InkWell(
                      onTap: () {
                        setState(() {
                          selYear = year;
                          _activeDate = DateTime(year, _isYearFilterActive ? 1 : _activeDate.month, 1);
                        });
                        if (_isYearFilterActive) {
                          // Keep "Semua Bulan" — just update year
                          _filterJenis = 'all';
                          _filterFrekuensi = 'all';
                        }
                        ref.read(billsProvider.notifier).refresh();
                        if (ctx.mounted) Navigator.of(ctx).pop();
                      },
                      borderRadius: BorderRadius.circular(AppRadius.base),
                      child: Container(
                        decoration: BoxDecoration(
                          color: isSelected
                              ? (isDark ? AppColors.darkAccentSoft : AppColors.lightAccentSoft)
                              : (isDark ? AppColors.darkSurfaceHover : AppColors.lightSurfaceHover),
                          borderRadius: BorderRadius.circular(AppRadius.base),
                          border: Border.all(
                            color: isSelected ? AppColors.accent : (isDark ? AppColors.darkBorder : AppColors.lightBorder),
                            width: isSelected ? 2 : 1,
                          ),
                        ),
                        alignment: Alignment.center,
                        child: Text(
                          year.toString(),
                          style: TextStyle(
                            color: isSelected ? AppColors.accent : (isDark ? AppColors.darkText : AppColors.lightText),
                            fontWeight: isSelected ? FontWeight.w600 : null,
                          ),
                        ),
                      ),
                    );
                  },
                ),
              ],
            ),
          );
        },
      ),
    );
  }

  void _resetFilters() {
    setState(() {
      _searchController.clear();
      _filterJenis = 'all';
      _filterFrekuensi = 'all';
    });
  }

  bool get _hasActiveFilter =>
      _searchController.text.trim().isNotEmpty ||
      _filterJenis != 'all' ||
      _filterFrekuensi != 'all';

  List<BackendDue> _applyFilters(List<BackendDue> all) {
    final query = _searchController.text.trim().toLowerCase();
    return all.where((due) {
      if (query.isNotEmpty &&
          !due.name.toLowerCase().contains(query) &&
          !_periodLabel(due.periodType).toLowerCase().contains(query)) {
        return false;
      }
      if (_filterJenis == 'reguler' && due.isVoluntary) return false;
      if (_filterJenis == 'sukarela' && !due.isVoluntary) return false;
      if (_filterFrekuensi != 'all') {
        if (_filterFrekuensi == 'monthly' &&
            due.periodType != DuePeriodType.monthly) {
          return false;
        }
        if (_filterFrekuensi == 'yearly' &&
            due.periodType != DuePeriodType.yearly) {
          return false;
        }
        if (_filterFrekuensi == 'one_time' &&
            due.periodType != DuePeriodType.oneTime) {
          return false;
        }
      }
      final refDate = due.createdAt ?? due.updatedAt;
      if (!_isYearFilterActive && refDate != null &&
          (refDate.year != _activeDate.year ||
              refDate.month != _activeDate.month)) {
        return false;
      }
      return true;
    }).toList();
  }

  String? _buildPeriodBadgeLabel(BackendDue due) {
    final dt = due.createdAt;
    if (dt != null && dt.year >= 2000) {
      return '${_monthNames[dt.month - 1].toUpperCase()} ${dt.year}';
    }
    if (due.periodType == DuePeriodType.monthly ||
        due.periodType == DuePeriodType.yearly) {
      return _periodLabel(due.periodType).toUpperCase();
    }
    return null;
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;
    final user = ref.watch(authRepositoryProvider)?.valueOrNull;
    if (!canManageIuran(user)) {
      return const Center(
          child: Text('Anda tidak memiliki akses untuk mengelola iuran.'));
    }

    final duesState = ref.watch(duesProvider);
    return RefreshIndicator(
      onRefresh: () => ref.read(duesProvider.notifier).refresh(),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const SizedBox(height: AppSpacing.sm),

          // ── FILTER COLLAPSIBLE ──
          InkWell(
            onTap: () {
              setState(() {
                _filtersExpanded = !_filtersExpanded;
              });
            },
            borderRadius: BorderRadius.circular(AppRadius.base),
            child: Column(
              children: [
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 12,
                    vertical: 10,
                  ),
                  decoration: BoxDecoration(
                    color: isDark
                        ? AppColors.darkSurface
                        : AppColors.lightSurface,
                    borderRadius: BorderRadius.circular(AppRadius.base),
                    border: Border.all(
                      color: isDark
                          ? AppColors.accent.withValues(alpha: 0.35)
                          : AppColors.accent.withValues(alpha: 0.25),
                      width: 1,
                    ),
                    boxShadow: isDark
                        ? [
                            BoxShadow(
                              color: AppColors.accent.withValues(alpha: 0.12),
                              blurRadius: 8,
                              spreadRadius: 0,
                            ),
                          ]
                        : null,
                  ),
                  child: Row(
                    children: [
                      Icon(
                        Icons.tune,
                        size: 18,
                        color: isDark
                            ? AppColors.darkText
                            : AppColors.lightText,
                      ),
                      const SizedBox(width: 8),
                      Text(
                        'Filter',
                        style: theme.textTheme.bodyMedium?.copyWith(
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                      const Spacer(),
                      AnimatedRotation(
                        turns: _filtersExpanded ? 0 : 0.5,
                        duration: const Duration(milliseconds: 200),
                        child: Icon(
                          Icons.keyboard_arrow_down,
                          size: 20,
                          color: AppColors.accent,
                        ),
                      ),
                    ],
                  ),
                ),
                AnimatedSize(
                  duration: const Duration(milliseconds: 200),
                  curve: Curves.easeInOut,
                  alignment: Alignment.topCenter,
                  child: _filtersExpanded
                      ? Column(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            const SizedBox(height: AppSpacing.sm),

                            // ── Row 1: Bulan + Tahun ──
                            Padding(
                              padding: const EdgeInsets.symmetric(
                                  horizontal: AppSpacing.base),
                              child: Row(
                                children: [
                                  Expanded(
                                    child: InkWell(
                                      onTap: _pickMonth,
                                      borderRadius:
                                          BorderRadius.circular(AppRadius.base),
                                      child: Container(
                                        height: 38,
                                        alignment: Alignment.centerLeft,
                                        padding: const EdgeInsets.symmetric(
                                            horizontal: 10),
                                        decoration: BoxDecoration(
                                          color: isDark
                                              ? AppColors.darkSurface
                                              : AppColors.lightSurface,
                                          borderRadius: BorderRadius.circular(
                                              AppRadius.base),
                                          border: Border.all(
                                            color: isDark
                                                ? AppColors.accent.withValues(
                                                    alpha: 0.35)
                                                : AppColors.accent.withValues(
                                                    alpha: 0.25),
                                            width: 1,
                                          ),
                                        ),
                                        child: Text(
                                          _isYearFilterActive ? 'Semua Bulan' : _monthNames[_activeDate.month - 1],
                                          style: theme.textTheme.bodySmall?.copyWith(
                                            fontWeight: FontWeight.w600,
                                          ),
                                          maxLines: 1,
                                          overflow: TextOverflow.ellipsis,
                                        ),
                                      ),
                                    ),
                                  ),
                                  const SizedBox(width: AppSpacing.sm),
                                  Expanded(
                                    child: InkWell(
                                      onTap: _pickYear,
                                      borderRadius:
                                          BorderRadius.circular(AppRadius.base),
                                      child: Container(
                                        height: 38,
                                        alignment: Alignment.centerLeft,
                                        padding: const EdgeInsets.symmetric(
                                            horizontal: 10),
                                        decoration: BoxDecoration(
                                          color: isDark
                                              ? AppColors.darkSurface
                                              : AppColors.lightSurface,
                                          borderRadius: BorderRadius.circular(
                                              AppRadius.base),
                                          border: Border.all(
                                            color: isDark
                                                ? AppColors.accent.withValues(
                                                    alpha: 0.35)
                                                : AppColors.accent.withValues(
                                                    alpha: 0.25),
                                            width: 1,
                                          ),
                                        ),
                                        child: Text(
                                          _activeDate.year.toString(),
                                          style: theme.textTheme.bodySmall?.copyWith(
                                            fontWeight: FontWeight.w600,
                                          ),
                                          maxLines: 1,
                                          overflow: TextOverflow.ellipsis,
                                        ),
                                      ),
                                    ),
                                  ),
                                ],
                              ),
                            ),

                            const SizedBox(height: AppSpacing.xs),

                            // ── Row 2: Jenis Iuran & Frekuensi Periode ──
                            Padding(
                              padding: const EdgeInsets.symmetric(
                                  horizontal: AppSpacing.base),
                              child: Row(
                                children: [
                                  Expanded(
                                    child: Container(
                                      height: 38,
                                      padding: const EdgeInsets.symmetric(
                                          horizontal: 10),
                                      decoration: BoxDecoration(
                                        color: isDark
                                            ? AppColors.darkSurface
                                            : AppColors.lightSurface,
                                        borderRadius: BorderRadius.circular(
                                            AppRadius.base),
                                        border: Border.all(
                                          color: isDark
                                              ? AppColors.accent.withValues(
                                                  alpha: 0.35)
                                              : AppColors.accent.withValues(
                                                  alpha: 0.25),
                                          width: 1,
                                        ),
                                      ),
                                      child: DropdownButtonHideUnderline(
                                        child: DropdownButton<String>(
                                          key: const Key(
                                              'kelola-iuran-jenis-filter'),
                                          value: _filterJenis,
                                          isExpanded: true,
                                          style: theme.textTheme.bodySmall
                                              ?.copyWith(
                                                  fontWeight: FontWeight.w600),
                                          dropdownColor: isDark
                                              ? AppColors.darkSurface
                                              : AppColors.lightSurface,
                                          items: const [
                                            DropdownMenuItem(
                                                value: 'all',
                                                child: Text(
                                                    'Semua Jenis Iuran')),
                                            DropdownMenuItem(
                                                value: 'reguler',
                                                child: Text('Reguler')),
                                            DropdownMenuItem(
                                                value: 'sukarela',
                                                child: Text('Sukarela')),
                                          ],
                                          onChanged: (val) {
                                            if (val != null) {
                                              setState(() => _filterJenis = val);
                                            }
                                          },
                                        ),
                                      ),
                                    ),
                                  ),
                                  const SizedBox(width: AppSpacing.sm),
                                  Expanded(
                                    child: Container(
                                      height: 38,
                                      padding: const EdgeInsets.symmetric(
                                          horizontal: 10),
                                      decoration: BoxDecoration(
                                        color: isDark
                                            ? AppColors.darkSurface
                                            : AppColors.lightSurface,
                                        borderRadius: BorderRadius.circular(
                                            AppRadius.base),
                                        border: Border.all(
                                          color: isDark
                                              ? AppColors.accent.withValues(
                                                  alpha: 0.35)
                                              : AppColors.accent.withValues(
                                                  alpha: 0.25),
                                          width: 1,
                                        ),
                                      ),
                                      child: DropdownButtonHideUnderline(
                                        child: DropdownButton<String>(
                                          key: const Key(
                                              'kelola-iuran-periode-filter'),
                                          value: _filterFrekuensi,
                                          isExpanded: true,
                                          style: theme.textTheme.bodySmall
                                              ?.copyWith(
                                                  fontWeight: FontWeight.w600),
                                          dropdownColor: isDark
                                              ? AppColors.darkSurface
                                              : AppColors.lightSurface,
                                          items: const [
                                            DropdownMenuItem(
                                                value: 'all',
                                                child: Text(
                                                    'Semua Frekuensi')),
                                            DropdownMenuItem(
                                                value: 'monthly',
                                                child: Text('Bulanan')),
                                            DropdownMenuItem(
                                                value: 'yearly',
                                                child: Text('Tahunan')),
                                            DropdownMenuItem(
                                                value: 'one_time',
                                                child: Text('Sekali')),
                                          ],
                                          onChanged: (val) {
                                            if (val != null) {
                                              setState(
                                                  () => _filterFrekuensi = val);
                                            }
                                          },
                                        ),
                                      ),
                                    ),
                                  ),
                                ],
                              ),
                            ),

                            const SizedBox(height: AppSpacing.xs),

                            // ── Row 3: Search ──
                            Padding(
                              padding: const EdgeInsets.symmetric(
                                  horizontal: AppSpacing.base),
                              child: SizedBox(
                                height: 38,
                                child: AppTextField(
                                  controller: _searchController,
                                  hintText: 'Cari jenis iuran...',
                                  isDense: true,
                                  fillColor: isDark
                                      ? AppColors.darkSurface
                                      : AppColors.lightSurface,
                                  textAlignVertical: TextAlignVertical.center,
                                  contentPadding:
                                      const EdgeInsets.symmetric(
                                          horizontal: 12, vertical: 0),
                                  prefixIconConstraints:
                                      const BoxConstraints(
                                          minWidth: 36, minHeight: 38),
                                  suffixIconConstraints:
                                      const BoxConstraints(
                                          minWidth: 36, minHeight: 38),
                                  prefixIcon:
                                      const Icon(Icons.search, size: 18),
                                  suffixIcon:
                                      _searchController.text.isNotEmpty
                                          ? IconButton(
                                              padding: EdgeInsets.zero,
                                              icon: const Icon(
                                                  Icons.clear, size: 18),
                                              onPressed: () {
                                                setState(() =>
                                                    _searchController.clear());
                                              },
                                            )
                                          : null,
                                  onChanged: (_) => setState(() {}),
                                ),
                              ),
                            ),

                            const SizedBox(height: AppSpacing.xs),
                          ],
                        )
                      : const SizedBox.shrink(),
                ),
              ],
            ),
          ),
          const SizedBox(height: AppSpacing.sm),

          // Content: Dues list
          Expanded(
            child: duesState.when(
              loading: () => const Center(child: CircularProgressIndicator()),
              error: (error, _) => Center(
                child: Padding(
                  padding: const EdgeInsets.all(AppSpacing.lg),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text('Gagal memuat daftar iuran: $error'),
                      const SizedBox(height: AppSpacing.md),
                      FilledButton(
                        onPressed: () =>
                            ref.read(duesProvider.notifier).refresh(),
                        child: const Text('Coba Lagi'),
                      ),
                    ],
                  ),
                ),
              ),
              data: (dues) {
                if (dues.isEmpty) {
                  return const SingleChildScrollView(
                    physics: AlwaysScrollableScrollPhysics(),
                    child: SizedBox(
                      height: 300,
                      child: Center(child: Text('Belum ada jenis iuran.')),
                    ),
                  );
                }
                final filteredDues = _applyFilters(dues);
                if (filteredDues.isEmpty) {
                  return SingleChildScrollView(
                    physics: const AlwaysScrollableScrollPhysics(),
                    padding: const EdgeInsets.all(AppSpacing.lg),
                    child: EmptyState(
                      icon: Icons.search_off,
                      title: 'Iuran Tidak Ditemukan',
                      description:
                          'Tidak ada jenis iuran yang sesuai dengan filter saat ini.',
                      actionText: _hasActiveFilter ? 'Reset Filter' : null,
                      onAction: _hasActiveFilter ? _resetFilters : null,
                    ),
                  );
                }
                return ListView.separated(
                  physics: const AlwaysScrollableScrollPhysics(),
                  padding: const EdgeInsets.fromLTRB(
                      AppSpacing.base, AppSpacing.xs, AppSpacing.base, 96),
                  itemCount: filteredDues.length,
                  separatorBuilder: (_, _) =>
                      const SizedBox(height: AppSpacing.sm),
                  itemBuilder: (context, index) {
                    final due = filteredDues[index];
                    final periodBadgeLabel = _buildPeriodBadgeLabel(due);
                    return Material(
                      color: Colors.transparent,
                      child: InkWell(
                        onTap: () => _showEditDialog(context, ref, due),
                        borderRadius:
                            BorderRadius.circular(AppRadius.base),
                        child: AppCard(
                          padding: const EdgeInsets.all(AppSpacing.base),
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              // Header row
                              Row(
                                crossAxisAlignment: CrossAxisAlignment.center,
                                children: [
                                  CircleAvatar(
                                    radius: 20,
                                    backgroundColor: AppColors.accent
                                        .withValues(
                                            alpha: isDark ? 0.25 : 0.12),
                                    child: const Icon(
                                      Icons.receipt_long_outlined,
                                      size: 20,
                                      color: AppColors.accent,
                                    ),
                                  ),
                                  const SizedBox(width: AppSpacing.md),
                                  Expanded(
                                    child: Text(
                                      due.name,
                                      style: theme.textTheme.titleMedium
                                          ?.copyWith(
                                              fontWeight: FontWeight.w600),
                                      overflow: TextOverflow.ellipsis,
                                    ),
                                  ),
                                  IconButton(
                                    icon: const Icon(
                                      Icons.edit_outlined,
                                      size: 20,
                                    ),
                                    tooltip: 'Ubah Iuran',
                                    visualDensity: VisualDensity.compact,
                                    onPressed: () =>
                                        _showEditDialog(context, ref, due),
                                  ),
                                ],
                              ),
                              const Divider(height: 20),
                              // Body row
                              Row(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Expanded(
                                    child: Wrap(
                                      spacing: 6,
                                      runSpacing: 6,
                                      children: [
                                        AppBadge(
                                          label: due.name,
                                          textColor: AppColors.info,
                                          backgroundColor:
                                              AppColors.info.withValues(
                                                  alpha: isDark ? 0.25 : 0.12),
                                        ),
                                        AppBadge(
                                          label: due.isVoluntary
                                              ? 'Sukarela'
                                              : 'Reguler',
                                          textColor: due.isVoluntary
                                              ? AppColors.info
                                              : AppColors.success,
                                          backgroundColor: (due.isVoluntary
                                                  ? AppColors.info
                                                  : AppColors.success)
                                              .withValues(
                                                  alpha: isDark ? 0.25 : 0.12),
                                        ),
                                        if (periodBadgeLabel != null)
                                          AppBadge(
                                            label: periodBadgeLabel,
                                            textColor: AppColors.accent,
                                            backgroundColor: AppColors.accent
                                                .withValues(
                                                    alpha: isDark ? 0.25 : 0.12),
                                          ),
                                      ],
                                    ),
                                  ),
                                  const SizedBox(width: AppSpacing.sm),
                                  Column(
                                    crossAxisAlignment:
                                        CrossAxisAlignment.end,
                                    children: [
                                      Text(
                                        due.isVoluntary
                                            ? 'Sukarela'
                                            : formatRupiahString(due.amount),
                                        style: theme.textTheme.bodyMedium
                                            ?.copyWith(
                                                fontWeight: FontWeight.w600),
                                        textAlign: TextAlign.right,
                                      ),
                                      const SizedBox(height: 4),
                                      AppBadge(
                                        label: due.isActive ? 'Aktif' : 'Tidak Aktif',
                                        textColor: due.isActive
                                            ? AppColors.success
                                            : AppColors.danger,
                                        backgroundColor: due.isActive
                                            ? AppColors.success
                                                .withValues(
                                                    alpha: isDark ? 0.2 : 0.1)
                                            : AppColors.danger
                                                .withValues(
                                                    alpha: isDark ? 0.2 : 0.1),
                                      ),
                                    ],
                                  ),
                                ],
                              ),
                            ],
                          ),
                        ),
                      ),
                    );
                  },
                );
              },
            ),
          ),
        ],
      ),
    );
  }

  Future<void> _showEditDialog(
      BuildContext context, WidgetRef ref, BackendDue due) async {
    final updated = await showDialog<bool>(
      context: context,
      builder: (ctx) => _EditDueDialog(due: due),
    );
    if (updated == true && context.mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('Iuran berhasil diperbarui.'),
          backgroundColor: AppColors.success,
        ),
      );
      await ref.read(duesProvider.notifier).refresh();
    }
  }

  String _periodLabel(DuePeriodType periodType) => switch (periodType) {
        DuePeriodType.monthly => 'Bulanan',
        DuePeriodType.yearly => 'Tahunan',
        DuePeriodType.oneTime => 'Sekali',
      };
}

// ── Edit Due Dialog (extracted from IuranManagementScreen) ────────────

class _EditDueDialog extends ConsumerStatefulWidget {
  final BackendDue due;

  const _EditDueDialog({required this.due});

  @override
  ConsumerState<_EditDueDialog> createState() => _EditDueDialogState();
}

class _EditDueDialogState extends ConsumerState<_EditDueDialog> {
  static const _monthNames = [
    'Januari',
    'Februari',
    'Maret',
    'April',
    'Mei',
    'Juni',
    'Juli',
    'Agustus',
    'September',
    'Oktober',
    'November',
    'Desember',
  ];

  final _formKey = GlobalKey<FormState>();
  late final TextEditingController _nameController;
  late final TextEditingController _amountController;
  late final TextEditingController _validUntilController;
  late DuePeriodType _periodType;
  late PaymentRule _paymentRule;
  late bool _isVoluntary;
  late bool _isActive;
  bool _loading = false;
  String? _errorMessage;

  bool _syncingBills = false;
  String? _syncSuccessMessage;
  List<DueMonthSummary> _dueMonths = [];
  final Map<String, bool> _stagedMonthActive = {};
  bool _loadingMonths = false;

  @override
  void initState() {
    super.initState();
    _nameController = TextEditingController(text: widget.due.name);
    final rawAmount = widget.due.amount.split('.').first;
    _amountController = TextEditingController(text: rawAmount);
    _validUntilController =
        TextEditingController(text: widget.due.validUntil ?? '');
    _periodType = widget.due.periodType;
    _paymentRule = widget.due.paymentRule;
    _isVoluntary = widget.due.isVoluntary;
    _isActive = widget.due.isActive;
    if (widget.due.dueType == DueType.multiMonth) {
      _loadMonths();
    }
  }

  @override
  void dispose() {
    _nameController.dispose();
    _amountController.dispose();
    _validUntilController.dispose();
    super.dispose();
  }

  Future<void> _loadMonths() async {
    setState(() {
      _loadingMonths = true;
      _errorMessage = null;
    });
    try {
      final months = await ref
          .read(iuranRepositoryProvider)
          .fetchDueMonths(widget.due.id);
      if (mounted) {
        setState(() {
          _dueMonths = months;
          _stagedMonthActive.clear();
          for (final m in months) {
            _stagedMonthActive[m.period] = m.isActive;
          }
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          final mapped = ref.read(iuranRepositoryProvider).mapDioError(e);
          _errorMessage = 'Gagal memuat status bulan: ${mapped.message}';
        });
      }
    } finally {
      if (mounted) {
        setState(() => _loadingMonths = false);
      }
    }
  }

  Future<void> _handleSyncBills() async {
    final year = widget.due.year ?? DateTime.now().year;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Sinkronkan Tagihan Warga'),
        content: Text(
          'Sistem akan menerbitkan 12 tagihan ($year) '
          'untuk setiap Warga yang belum memiliki tagihan pada iuran ini.\n\n'
          'Tagihan lama dan riwayat pembayaran yang sudah ada tidak akan diubah.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: const Text('Batal'),
          ),
          FilledButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: const Text('Sinkronkan'),
          ),
        ],
      ),
    );

    if (confirmed != true || !mounted) return;

    setState(() {
      _syncingBills = true;
      _errorMessage = null;
      _syncSuccessMessage = null;
    });

    try {
      final result = await ref.read(iuranRepositoryProvider).generateBills(
        dueId: widget.due.id,
        period: year.toString(),
        dueDate: '$year-01-10',
        idempotencyKey:
            'sync-${widget.due.id}-$year-${DateTime.now().millisecondsSinceEpoch}',
      );

      if (mounted) {
        setState(() {
          _syncingBills = false;
          _syncSuccessMessage = result.generatedCount > 0
              ? 'Sinkronisasi berhasil: ${result.generatedCount} tagihan baru diterbitkan.'
              : 'Sinkronisasi selesai: Semua Warga sudah memiliki tagihan.';
        });
        await ref.read(duesProvider.notifier).refresh();
        await ref.read(billsProvider.notifier).refresh();
        ref.invalidate(myBillsProvider);
        await _loadMonths();
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _syncingBills = false;
          final mapped = ref.read(iuranRepositoryProvider).mapDioError(e);
          _errorMessage = mapped.message;
        });
      }
    }
  }

  Future<void> _handleSave() async {
    if (!_formKey.currentState!.validate()) return;

    setState(() {
      _loading = true;
      _errorMessage = null;
    });

    try {
      final name = _nameController.text.trim();
      final amount = _isVoluntary
          ? 0
          : int.tryParse(_amountController.text.trim()) ?? 0;

      final validUntil = _isVoluntary &&
              _validUntilController.text.trim().isNotEmpty
          ? _validUntilController.text.trim()
          : null;

      await ref.read(iuranRepositoryProvider).updateDue(
            widget.due.id,
            name: name,
            amount: amount,
            periodType: _periodType,
            paymentRule: _paymentRule,
            isVoluntary: _isVoluntary,
            isActive: _isActive,
            validUntil: validUntil,
          );

      if (widget.due.dueType == DueType.multiMonth && _dueMonths.isNotEmpty) {
        final changedMonths = _dueMonths.where((m) {
          final staged = _stagedMonthActive[m.period];
          return staged != null && staged != m.isActive;
        }).toList();

        if (changedMonths.isNotEmpty) {
          int successCount = 0;
          String? firstFailedPeriod;
          dynamic firstError;

          for (final m in changedMonths) {
            final newActive = _stagedMonthActive[m.period]!;
            try {
              final updated = await ref
                  .read(iuranRepositoryProvider)
                  .updateDuePeriodStatus(
                    widget.due.id,
                    m.period,
                    isActive: newActive,
                  );
              successCount++;
              _dueMonths = updated;
            } catch (e) {
              firstFailedPeriod ??= m.period;
              firstError ??= e;
              // Revert staged value for the failed period to the server's current value
              _stagedMonthActive[m.period] = m.isActive;
            }
          }

          if (firstFailedPeriod != null) {
            if (mounted) {
              setState(() {
                _loading = false;
                final mapped =
                    ref.read(iuranRepositoryProvider).mapDioError(firstError);
                _errorMessage =
                    'Gagal memperbarui status bulan $firstFailedPeriod: ${mapped.message}. '
                    '$successCount/${changedMonths.length} periode berhasil disimpan.';
              });
            }
            return;
          }
        }
      }

      if (mounted) {
        await ref.read(duesProvider.notifier).refresh();
        await ref.read(billsProvider.notifier).refresh();
        ref.invalidate(myBillsProvider);
        if (mounted) {
          Navigator.of(context).pop(true);
        }
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          final mapped = ref.read(iuranRepositoryProvider).mapDioError(e);
          _errorMessage = mapped.message;
          _loading = false;
        });
      }
    }
  }

  Future<void> _pickValidUntil() async {
    final initial = DateTime.tryParse(_validUntilController.text) ??
        DateTime.now().add(const Duration(days: 365));
    final picked = await showDatePicker(
      context: context,
      initialDate: initial,
      firstDate: DateTime.now(),
      lastDate: DateTime(2100),
      helpText: 'PILIH TANGGAL BERLAKU SAMPAI',
    );
    if (picked != null) {
      setState(() {
        _validUntilController.text =
            '${picked.year.toString().padLeft(4, '0')}-${picked.month.toString().padLeft(2, '0')}-${picked.day.toString().padLeft(2, '0')}';
      });
    }
  }

  void _clearValidUntil() => setState(() => _validUntilController.text = '');

  Widget _buildNeonActionButton({
    Key? key,
    required String label,
    required IconData icon,
    required Color neonColor,
    required Color softBg,
    required VoidCallback? onPressed,
    bool isSaving = false,
  }) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    return Container(
      key: key,
      decoration: BoxDecoration(
        color: softBg,
        borderRadius: BorderRadius.circular(AppRadius.base),
        border: Border.all(color: neonColor, width: 1.5),
        boxShadow: [
          BoxShadow(
            color: neonColor.withValues(
                alpha: isDark ? 0.35 : 0.22),
            blurRadius: 8,
            spreadRadius: 1,
            offset: const Offset(0, 1),
          ),
        ],
      ),
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: isSaving ? null : onPressed,
          borderRadius: BorderRadius.circular(AppRadius.base),
          child: Padding(
            padding: const EdgeInsets.symmetric(
                horizontal: 16, vertical: 10),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                if (isSaving)
                  SizedBox(
                    width: 18,
                    height: 18,
                    child: CircularProgressIndicator(
                      strokeWidth: 2,
                      backgroundColor: Colors.transparent,
                      valueColor:
                          AlwaysStoppedAnimation<Color>(neonColor),
                    ),
                  )
                else ...[
                  Icon(icon, size: 18, color: neonColor),
                  const SizedBox(width: 6),
                ],
                Text(
                  label,
                  style: TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.w700,
                    color: neonColor,
                    letterSpacing: 0.3,
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return AlertDialog(
      insetPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 24),
      titlePadding: const EdgeInsets.fromLTRB(24, 20, 24, 8),
      contentPadding: const EdgeInsets.fromLTRB(24, 8, 24, 16),
      actionsPadding: const EdgeInsets.fromLTRB(24, 8, 24, 16),
      title: const Text('Ubah Iuran'),
      content: SizedBox(
        width: double.maxFinite,
        child: ConstrainedBox(
          constraints: BoxConstraints(
            maxWidth: 480,
            maxHeight: MediaQuery.sizeOf(context).height * 0.72,
          ),
          child: SingleChildScrollView(
            clipBehavior: Clip.antiAlias,
            padding: const EdgeInsets.only(top: 8, bottom: 8),
            child: Form(
              key: _formKey,
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  if (_errorMessage != null) ...[
                    Container(
                      padding: const EdgeInsets.all(10),
                      decoration: BoxDecoration(
                        color: AppColors.danger.withValues(alpha: 0.12),
                        borderRadius: BorderRadius.circular(AppRadius.sm),
                        border: Border.all(
                            color: AppColors.danger.withValues(alpha: 0.3)),
                      ),
                      child: Text(
                        _errorMessage!,
                        style: theme.textTheme.bodySmall?.copyWith(
                          color: AppColors.danger,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                    ),
                    const SizedBox(height: 14),
                  ],
                  TextFormField(
                    controller: _nameController,
                    decoration: const InputDecoration(
                      labelText: 'Nama Iuran',
                      hintText: 'Contoh: Iuran Kebersihan',
                    ),
                    validator: (val) {
                      if (val == null || val.trim().isEmpty) {
                        return 'Nama iuran wajib diisi.';
                      }
                      return null;
                    },
                    enabled: !_loading,
                  ),
                  const SizedBox(height: 14),
                  TextFormField(
                    controller: _amountController,
                    keyboardType: TextInputType.number,
                    inputFormatters:
                        [FilteringTextInputFormatter.digitsOnly],
                    readOnly: _isVoluntary,
                    decoration: InputDecoration(
                      labelText: 'Nominal (Rp)',
                      hintText: 'Contoh: 50000',
                      enabled: !_isVoluntary && !_loading,
                    ),
                    validator: (val) {
                      if (_isVoluntary) return null;
                      final parsed = int.tryParse(val?.trim() ?? '');
                      if (parsed == null || parsed <= 0) {
                        return 'Nominal harus lebih dari 0.';
                      }
                      return null;
                    },
                  ),
                  const SizedBox(height: 14),
                  DropdownButtonFormField<DuePeriodType>(
                    isExpanded: true,
                    value: _periodType,
                    decoration:
                        const InputDecoration(labelText: 'Frekuensi'),
                    items: const [
                      DropdownMenuItem(
                        value: DuePeriodType.monthly,
                        child: Text('Bulanan'),
                      ),
                      DropdownMenuItem(
                        value: DuePeriodType.yearly,
                        child: Text('Tahunan'),
                      ),
                      DropdownMenuItem(
                        value: DuePeriodType.oneTime,
                        child: Text('Sekali'),
                      ),
                    ],
                    onChanged: _loading
                        ? null
                        : (v) {
                            if (v != null) setState(() => _periodType = v);
                          },
                  ),
                  const SizedBox(height: 14),
                  DropdownButtonFormField<PaymentRule>(
                    isExpanded: true,
                    value: _paymentRule,
                    decoration: const InputDecoration(
                      labelText: 'Aturan Pembayaran',
                    ),
                    items: const [
                      DropdownMenuItem(
                        value: PaymentRule.full,
                        child: Text('Seluruh (Penuh)'),
                      ),
                      DropdownMenuItem(
                        value: PaymentRule.partial,
                        child: Text('Sebagian (Cicilan)'),
                      ),
                    ],
                    onChanged: _loading
                        ? null
                        : (v) {
                            if (v != null) setState(() => _paymentRule = v);
                          },
                  ),
                  if (_isVoluntary) ...[
                    const SizedBox(height: 14),
                    ListTile(
                      dense: true,
                      title: const Text('Berlaku Sampai'),
                      subtitle: Text(
                        _validUntilController.text.isEmpty
                            ? 'Tidak ada (selalu aktif)'
                            : _validUntilController.text,
                        style: theme.textTheme.bodySmall,
                      ),
                      trailing: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          IconButton(
                            icon: const Icon(Icons.calendar_today,
                                size: 20),
                            tooltip: 'Pilih tanggal',
                            onPressed: _loading ? null : _pickValidUntil,
                          ),
                          if (_validUntilController.text.isNotEmpty)
                            IconButton(
                              icon: const Icon(Icons.clear, size: 20),
                              tooltip: 'Hapus tanggal',
                              onPressed: _loading ? null : _clearValidUntil,
                            ),
                        ],
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      'Tanggal maksimum tagihan iuran sukarela dibuat. Kosongkan agar berlaku tanpa batas.',
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: theme.colorScheme.onSurfaceVariant,
                      ),
                    ),
                  ],
                  const SizedBox(height: 14),
                  GestureDetector(
                    key: const ValueKey('edit-due-status'),
                    onTap: _loading
                        ? null
                        : () => setState(() => _isActive = !_isActive),
                    child: Container(
                      width: double.infinity,
                      padding: const EdgeInsets.symmetric(
                        horizontal: 12, vertical: 14),
                      decoration: BoxDecoration(
                        color: theme.colorScheme.surfaceContainerHighest,
                        border:
                            Border.all(color: theme.dividerColor),
                        borderRadius:
                            BorderRadius.circular(AppRadius.base),
                      ),
                      child: Row(
                        mainAxisAlignment: MainAxisAlignment.spaceBetween,
                        children: [
                          Text(
                            'Status',
                            style: theme.textTheme.bodyMedium?.copyWith(
                              color: theme.colorScheme.onSurface,
                            ),
                          ),
                          AppBadge(
                            label: _isActive ? 'Aktif' : 'Tidak Aktif',
                            backgroundColor: _isActive
                                ? AppColors.success
                                    .withValues(alpha: 0.12)
                                : AppColors.danger
                                    .withValues(alpha: 0.12),
                            textColor: _isActive
                                ? AppColors.success
                                : AppColors.danger,
                          ),
                        ],
                      ),
                    ),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    'Iuran tidak aktif tidak dapat menghasilkan tagihan baru. '
                    'Tagihan dan pembayaran lama tetap tersimpan.',
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  ),
                  if (widget.due.dueType == DueType.multiMonth) ...[
                    const SizedBox(height: 16),
                    const Divider(),
                    const SizedBox(height: 8),
                    Text(
                      'Penerbitan Susulan Warga (${widget.due.year ?? DateTime.now().year})',
                      style: theme.textTheme.titleSmall?.copyWith(
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      'Terbitkan 12 tagihan untuk Warga yang baru didaftarkan setelah iuran ini dibuat. '
                      'Tagihan yang sudah ada tidak akan diubah atau diduplikasi.',
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: theme.colorScheme.onSurfaceVariant,
                      ),
                    ),
                    const SizedBox(height: 10),
                    SizedBox(
                      width: double.infinity,
                      child: OutlinedButton.icon(
                        key: const ValueKey('btn-sync-bills'),
                        style: OutlinedButton.styleFrom(
                          padding: const EdgeInsets.symmetric(
                              horizontal: 14, vertical: 12),
                          side: const BorderSide(
                              color: AppColors.accent, width: 1.5),
                          shape: RoundedRectangleBorder(
                            borderRadius:
                                BorderRadius.circular(AppRadius.base),
                          ),
                        ),
                        icon: _syncingBills
                            ? const SizedBox(
                                width: 16,
                                height: 16,
                                child: CircularProgressIndicator(
                                    strokeWidth: 2),
                              )
                            : const Icon(Icons.sync,
                                size: 18, color: AppColors.accent),
                        label: Text(
                          _syncingBills
                              ? 'Menyinkronkan...'
                              : 'Sinkronkan Tagihan Warga',
                          style: const TextStyle(
                            fontWeight: FontWeight.w600,
                            color: AppColors.accent,
                          ),
                        ),
                        onPressed: (_syncingBills || _loading)
                            ? null
                            : _handleSyncBills,
                      ),
                    ),
                    if (_syncSuccessMessage != null) ...[
                      const SizedBox(height: 10),
                      Container(
                        width: double.infinity,
                        padding: const EdgeInsets.all(10),
                        decoration: BoxDecoration(
                          color: AppColors.success
                              .withValues(alpha: 0.12),
                          borderRadius:
                              BorderRadius.circular(AppRadius.sm),
                          border: Border.all(
                            color: AppColors.success
                                .withValues(alpha: 0.3),
                          ),
                        ),
                        child: Row(
                          children: [
                            const Icon(
                              Icons.check_circle_outline,
                              size: 18,
                              color: AppColors.success,
                            ),
                            const SizedBox(width: 8),
                            Expanded(
                              child: Text(
                                _syncSuccessMessage!,
                                style:
                                    theme.textTheme.bodySmall?.copyWith(
                                  color: AppColors.success,
                                  fontWeight: FontWeight.w600,
                                ),
                              ),
                            ),
                          ],
                        ),
                      ),
                    ],
                    const SizedBox(height: 16),
                    Text(
                      'Status Tagihan per Bulan',
                      style: theme.textTheme.titleSmall?.copyWith(
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    const SizedBox(height: 8),
                    if (_loadingMonths)
                      const Center(
                        child: Padding(
                          padding: EdgeInsets.symmetric(vertical: 16),
                          child: CircularProgressIndicator(),
                        ),
                      )
                    else if (_dueMonths.isEmpty)
                      const Text(
                        'Belum ada data status bulan.',
                        style: TextStyle(fontSize: 12),
                      )
                    else
                      Column(
                        mainAxisSize: MainAxisSize.min,
                        children: _dueMonths.map((m) {
                          final isPaidAll = m.paidBills > 0 &&
                              m.paidBills == m.totalBills;
                          final currentActive =
                              _stagedMonthActive[m.period] ?? m.isActive;
                          return ListTile(
                            dense: true,
                            contentPadding: EdgeInsets.zero,
                            title: Text(
                              _monthNames[m.month - 1],
                              style: TextStyle(
                                fontSize: 13,
                                fontWeight: currentActive
                                    ? FontWeight.w600
                                    : FontWeight.normal,
                                color: currentActive
                                    ? null
                                    : theme.colorScheme.onSurfaceVariant,
                              ),
                            ),
                            subtitle: Text(
                              '${m.activeBills}/${m.totalBills} tagihan'
                              '${m.paidBills > 0 ? ', ${m.paidBills} lunas' : ''}',
                              style: TextStyle(
                                fontSize: 11,
                                color:
                                    theme.colorScheme.onSurfaceVariant,
                              ),
                            ),
                            trailing: Switch(
                              value: currentActive,
                              onChanged: (_loading ||
                                      _loadingMonths ||
                                      isPaidAll ||
                                      !m.canDeactivate)
                                  ? null
                                  : (val) {
                                      setState(() {
                                        _stagedMonthActive[m.period] =
                                            val;
                                      });
                                    },
                            ),
                          );
                        }).toList(),
                      ),
                  ],
                ],
              ),
            ),
          ),
        ),
      ),
      actions: [
        _buildNeonActionButton(
          label: 'Batal',
          icon: Icons.cancel_outlined,
          neonColor: AppColors.danger,
          softBg: Theme.of(context).brightness == Brightness.dark
              ? AppColors.darkDangerSoft
              : AppColors.lightDangerSoft,
          onPressed:
              _loading ? null : () => Navigator.of(context).pop(false),
          isSaving: false,
        ),
        const SizedBox(width: AppSpacing.sm),
        _buildNeonActionButton(
          key: const ValueKey('iuran-save-button'),
          label: _loading ? '' : 'Simpan',
          icon: Icons.save_outlined,
          neonColor: AppColors.accent,
          softBg: Theme.of(context).brightness == Brightness.dark
              ? AppColors.darkAccentSoft
              : AppColors.lightAccentSoft,
          onPressed: _loading ? null : _handleSave,
          isSaving: _loading,
        ),
      ],
    );
  }
}
