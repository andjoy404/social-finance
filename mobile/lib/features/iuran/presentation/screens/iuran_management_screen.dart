import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/widgets/app_badge.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/app_text_field.dart';
import 'package:social_finance/core/widgets/empty_state.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/iuran/data/iuran_access.dart';
import 'package:social_finance/features/iuran/data/iuran_api_models.dart';
import 'package:social_finance/features/iuran/data/iuran_models.dart';
import 'package:social_finance/features/iuran/data/iuran_providers.dart';
import 'package:social_finance/features/iuran/data/iuran_repository.dart';
import 'package:social_finance/features/iuran/presentation/screens/iuran_menu_screen.dart';
import 'package:social_finance/features/iuran/presentation/widgets/iuran_tab_bar.dart';

class IuranManagementScreen extends ConsumerStatefulWidget {
  const IuranManagementScreen({super.key});

  @override
  ConsumerState<IuranManagementScreen> createState() =>
      _IuranManagementScreenState();
}

class _IuranManagementScreenState extends ConsumerState<IuranManagementScreen> {
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

  late final TextEditingController _searchController;
  DateTime _activeDate = DateTime.now();
  bool _isAllPeriods = true;
  String _filterJenis = 'all'; // 'all', 'reguler', 'sukarela'
  String _filterFrekuensi = 'all'; // 'all', 'monthly', 'yearly', 'one_time'

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

  String _formatMonthYear(DateTime dt) {
    return '${_monthNames[dt.month - 1]} ${dt.year}';
  }

  void _prevMonth() {
    setState(() {
      _isAllPeriods = false;
      _activeDate = DateTime(_activeDate.year, _activeDate.month - 1, 1);
    });
  }

  void _nextMonth() {
    setState(() {
      _isAllPeriods = false;
      _activeDate = DateTime(_activeDate.year, _activeDate.month + 1, 1);
    });
  }

  Future<void> _pickMonth() async {
    final picked = await showDatePicker(
      context: context,
      initialDate: _activeDate,
      firstDate: DateTime(2020),
      lastDate: DateTime(2035),
      initialDatePickerMode: DatePickerMode.year,
      helpText: 'PILIH PERIODE BULAN',
    );
    if (picked != null) {
      setState(() {
        _isAllPeriods = false;
        _activeDate = DateTime(picked.year, picked.month, 1);
      });
    }
  }

  void _resetFilters() {
    setState(() {
      _searchController.clear();
      _isAllPeriods = true;
      _filterJenis = 'all';
      _filterFrekuensi = 'all';
    });
  }

  bool get _hasActiveFilter =>
      _searchController.text.trim().isNotEmpty ||
      !_isAllPeriods ||
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
      if (_filterJenis == 'reguler' && due.isVoluntary) {
        return false;
      }
      if (_filterJenis == 'sukarela' && !due.isVoluntary) {
        return false;
      }
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
      if (!_isAllPeriods) {
        final refDate = due.createdAt ?? due.updatedAt;
        if (refDate != null) {
          if (refDate.year != _activeDate.year ||
              refDate.month != _activeDate.month) {
            return false;
          }
        }
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
    final profileStatus = user?['profile_status'] as String? ?? 'ready';
    if (profileStatus != 'ready') {
      return IuranProfileStatusScreen(status: profileStatus);
    }
    if (!canManageIuran(user)) {
      return const Scaffold(
        body: Center(
          child: Text('Anda tidak memiliki akses untuk mengelola iuran.'),
        ),
      );
    }

    final duesState = ref.watch(duesProvider);
    return Scaffold(
      appBar: AppBar(
        centerTitle: false,
        title: const MenuAppBarTitle(title: 'Iuran'),
        bottom: const IuranTabBar(selectedIndex: 2),
      ),
      floatingActionButton: _buildFab(context),
      body: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const SizedBox(height: AppSpacing.sm),

          // ── KAS-style Filter Area (D3) ──
          // Row 1: Month/Periode Navigator & Semua Chip
          Container(
            margin: const EdgeInsets.symmetric(horizontal: AppSpacing.base),
            decoration: BoxDecoration(
              color: theme.scaffoldBackgroundColor,
              borderRadius: BorderRadius.circular(AppRadius.base),
              border: Border.all(
                color: isDark ? AppColors.darkBorder : AppColors.lightBorder,
              ),
            ),
            height: 38,
            padding: const EdgeInsets.symmetric(
              horizontal: AppSpacing.sm,
              vertical: AppSpacing.xs,
            ),
            child: Row(
              children: [
                IconButton(
                  icon: const Icon(Icons.chevron_left),
                  onPressed: _isAllPeriods ? null : _prevMonth,
                  tooltip: 'Bulan Sebelumnya',
                  visualDensity: VisualDensity.compact,
                ),
                Expanded(
                  child: InkWell(
                    onTap: _isAllPeriods ? null : _pickMonth,
                    borderRadius: BorderRadius.circular(AppRadius.sm),
                    child: Padding(
                      padding: const EdgeInsets.symmetric(
                        vertical: 4,
                        horizontal: 8,
                      ),
                      child: Row(
                        mainAxisAlignment: MainAxisAlignment.center,
                        children: [
                          Text(
                            _isAllPeriods
                                ? 'Semua Periode'
                                : _formatMonthYear(_activeDate),
                            style: theme.textTheme.titleMedium?.copyWith(
                              fontWeight: FontWeight.bold,
                              color: _isAllPeriods
                                  ? theme.colorScheme.onSurfaceVariant
                                  : AppColors.accent,
                            ),
                          ),
                          if (!_isAllPeriods) ...[
                            const SizedBox(width: 4),
                            const Icon(
                              Icons.arrow_drop_down,
                              size: 20,
                              color: AppColors.accent,
                            ),
                          ],
                        ],
                      ),
                    ),
                  ),
                ),
                IconButton(
                  icon: const Icon(Icons.chevron_right),
                  onPressed: _isAllPeriods ? null : _nextMonth,
                  tooltip: 'Bulan Berikutnya',
                  visualDensity: VisualDensity.compact,
                ),
                const SizedBox(width: 4),
                SizedBox(
                  height: 38,
                  child: Align(
                    alignment: Alignment.centerLeft,
                    child: ChoiceChip(
                      label: Text(
                        _isAllPeriods ? 'Per Bulan' : 'Semua',
                        style: const TextStyle(fontSize: 11),
                      ),
                      selected: _isAllPeriods,
                      visualDensity: VisualDensity.compact,
                      onSelected: (_) {
                        setState(() {
                          _isAllPeriods = !_isAllPeriods;
                        });
                      },
                    ),
                  ),
                ),
              ],
            ),
          ),

          const SizedBox(height: AppSpacing.xs),

          // Row 2: Jenis Iuran (Reguler / Sukarela) & Frekuensi Periode Dropdowns
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.base),
            child: Row(
              children: [
                Expanded(
                  child: Container(
                    height: 38,
                    padding: const EdgeInsets.symmetric(horizontal: 10),
                    decoration: BoxDecoration(
                      color: theme.scaffoldBackgroundColor,
                      borderRadius: BorderRadius.circular(AppRadius.base),
                      border: Border.all(
                        color: isDark
                            ? AppColors.darkBorder
                            : AppColors.lightBorder,
                      ),
                    ),
                    child: DropdownButtonHideUnderline(
                      child: DropdownButton<String>(
                        key: const Key('kelola-iuran-jenis-filter'),
                        value: _filterJenis,
                        isExpanded: true,
                        style: theme.textTheme.bodySmall?.copyWith(
                          fontWeight: FontWeight.w600,
                        ),
                        dropdownColor: theme.scaffoldBackgroundColor,
                        items: const [
                          DropdownMenuItem(
                            value: 'all',
                            child: Text('Semua Jenis Iuran'),
                          ),
                          DropdownMenuItem(
                            value: 'reguler',
                            child: Text('Reguler'),
                          ),
                          DropdownMenuItem(
                            value: 'sukarela',
                            child: Text('Sukarela'),
                          ),
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
                    padding: const EdgeInsets.symmetric(horizontal: 10),
                    decoration: BoxDecoration(
                      color: theme.scaffoldBackgroundColor,
                      borderRadius: BorderRadius.circular(AppRadius.base),
                      border: Border.all(
                        color: isDark
                            ? AppColors.darkBorder
                            : AppColors.lightBorder,
                      ),
                    ),
                    child: DropdownButtonHideUnderline(
                      child: DropdownButton<String>(
                        key: const Key('kelola-iuran-periode-filter'),
                        value: _filterFrekuensi,
                        isExpanded: true,
                        style: theme.textTheme.bodySmall?.copyWith(
                          fontWeight: FontWeight.w600,
                        ),
                        dropdownColor: theme.scaffoldBackgroundColor,
                        items: const [
                          DropdownMenuItem(
                            value: 'all',
                            child: Text('Semua Frekuensi'),
                          ),
                          DropdownMenuItem(
                            value: 'monthly',
                            child: Text('Bulanan'),
                          ),
                          DropdownMenuItem(
                            value: 'yearly',
                            child: Text('Tahunan'),
                          ),
                          DropdownMenuItem(
                            value: 'one_time',
                            child: Text('Sekali'),
                          ),
                        ],
                        onChanged: (val) {
                          if (val != null) {
                            setState(() => _filterFrekuensi = val);
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

          // Row 3: Search Box
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.base),
            child: SizedBox(
              height: 38,
              child: AppTextField(
                controller: _searchController,
                hintText: 'Cari jenis iuran...',
                isDense: true,
                fillColor: theme.scaffoldBackgroundColor,
                textAlignVertical: TextAlignVertical.center,
                contentPadding: const EdgeInsets.symmetric(
                  horizontal: 12,
                  vertical: 0,
                ),
                prefixIconConstraints: const BoxConstraints(
                  minWidth: 36,
                  minHeight: 38,
                ),
                suffixIconConstraints: const BoxConstraints(
                  minWidth: 36,
                  minHeight: 38,
                ),
                prefixIcon: const Icon(Icons.search, size: 18),
                suffixIcon: _searchController.text.isNotEmpty
                    ? IconButton(
                        padding: EdgeInsets.zero,
                        icon: const Icon(Icons.clear, size: 18),
                        onPressed: () {
                          setState(() {
                            _searchController.clear();
                          });
                        },
                      )
                    : null,
                onChanged: (_) => setState(() {}),
              ),
            ),
          ),

          const SizedBox(height: AppSpacing.sm),

          // ── Content ──
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
                  return const Center(child: Text('Belum ada jenis iuran.'));
                }
                final filteredDues = _applyFilters(dues);
                if (filteredDues.isEmpty) {
                  return Center(
                    child: SingleChildScrollView(
                      padding: const EdgeInsets.all(AppSpacing.lg),
                      child: EmptyState(
                        icon: Icons.search_off,
                        title: 'Iuran Tidak Ditemukan',
                        description:
                            'Tidak ada jenis iuran yang sesuai dengan filter saat ini.',
                        actionText: _hasActiveFilter ? 'Reset Filter' : null,
                        onAction: _hasActiveFilter ? _resetFilters : null,
                      ),
                    ),
                  );
                }
                return RefreshIndicator(
                  onRefresh: () => ref.read(duesProvider.notifier).refresh(),
                  child: ListView.separated(
                    padding: const EdgeInsets.fromLTRB(
                      AppSpacing.base,
                      AppSpacing.xs,
                      AppSpacing.base,
                      96,
                    ),
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
                          borderRadius: BorderRadius.circular(AppRadius.base),
                          child: AppCard(
                            padding: const EdgeInsets.all(AppSpacing.base),
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                // Header row: Avatar + Title + Edit button (no Switch — D2)
                                Row(
                                  crossAxisAlignment: CrossAxisAlignment.center,
                                  children: [
                                    CircleAvatar(
                                      radius: 20,
                                      backgroundColor: AppColors.accent
                                          .withValues(
                                            alpha: isDark ? 0.25 : 0.12,
                                          ),
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
                                              fontWeight: FontWeight.w600,
                                            ),
                                        overflow: TextOverflow.ellipsis,
                                      ),
                                    ),
                                    IconButton(
                                      icon: const Icon(
                                        Icons.edit_outlined,
                                        size: 20,
                                      ),
                                      tooltip: 'Edit Iuran',
                                      visualDensity: VisualDensity.compact,
                                      onPressed: () =>
                                          _showEditDialog(context, ref, due),
                                    ),
                                  ],
                                ),

                                const Divider(height: 20),

                                // Body row: Badges on left (D4), Nominal on right
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
                                            backgroundColor: AppColors.info
                                                .withValues(
                                                  alpha: isDark ? 0.25 : 0.12,
                                                ),
                                          ),
                                          AppBadge(
                                            label: due.isVoluntary
                                                ? 'Sukarela'
                                                : 'Reguler',
                                            textColor: due.isVoluntary
                                                ? AppColors.info
                                                : AppColors.success,
                                            backgroundColor:
                                                (due.isVoluntary
                                                        ? AppColors.info
                                                        : AppColors.success)
                                                    .withValues(
                                                      alpha: isDark
                                                          ? 0.25
                                                          : 0.12,
                                                    ),
                                          ),
                                          if (periodBadgeLabel != null)
                                            AppBadge(
                                              label: periodBadgeLabel,
                                              textColor: AppColors.accent,
                                              backgroundColor: AppColors.accent
                                                  .withValues(
                                                    alpha: isDark ? 0.25 : 0.12,
                                                  ),
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
                                                fontWeight: FontWeight.w600,
                                              ),
                                          textAlign: TextAlign.right,
                                        ),
                                        const SizedBox(height: 2),
                                        Text(
                                          '${_periodLabel(due.periodType)} · ${due.isActive ? 'Aktif' : 'Nonaktif'}',
                                          style: theme.textTheme.bodySmall
                                              ?.copyWith(
                                                color: theme
                                                    .colorScheme
                                                    .onSurfaceVariant,
                                              ),
                                          textAlign: TextAlign.right,
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
                  ),
                );
              },
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildFab(BuildContext context) {
    final accentSoft = AppColors.accentSoftColor(context);
    final button = Container(
      key: const Key('kelola-iuran-create-btn'),
      decoration: BoxDecoration(
        color: accentSoft,
        borderRadius: BorderRadius.circular(24),
        border: Border.all(color: AppColors.accent, width: 1),
        boxShadow: [
          BoxShadow(
            color: AppColors.accent.withValues(alpha: 0.15),
            blurRadius: 8,
            offset: const Offset(0, 0),
          ),
        ],
      ),
      child: Material(
        color: Colors.transparent,
        clipBehavior: Clip.none,
        child: InkWell(
          onTap: () => context.push('/home/iuran/form'),
          borderRadius: BorderRadius.circular(24),
          child: const SizedBox(
            height: 44,
            child: Padding(
              padding: EdgeInsets.symmetric(horizontal: 16),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(
                    '+',
                    style: TextStyle(
                      color: AppColors.accent,
                      fontWeight: FontWeight.w700,
                      fontSize: 16,
                    ),
                  ),
                  SizedBox(width: 6),
                  Icon(Icons.receipt_long, size: 18, color: AppColors.accent),
                  SizedBox(width: 6),
                  Text(
                    'Buat Iuran',
                    style: TextStyle(
                      color: AppColors.accent,
                      fontWeight: FontWeight.w600,
                      fontSize: 13,
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
    return Tooltip(message: 'Buat Iuran', child: button);
  }

  Future<void> _showEditDialog(
    BuildContext context,
    WidgetRef ref,
    BackendDue due,
  ) async {
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

class _EditDueDialog extends ConsumerStatefulWidget {
  final BackendDue due;

  const _EditDueDialog({required this.due});

  @override
  ConsumerState<_EditDueDialog> createState() => _EditDueDialogState();
}

class _EditDueDialogState extends ConsumerState<_EditDueDialog> {
  final _formKey = GlobalKey<FormState>();
  late final TextEditingController _nameController;
  late final TextEditingController _amountController;
  late DuePeriodType _periodType;
  late PaymentRule _paymentRule;
  late bool _isVoluntary;
  bool _loading = false;
  String? _errorMessage;

  @override
  void initState() {
    super.initState();
    _nameController = TextEditingController(text: widget.due.name);
    final rawAmount = widget.due.amount.split('.').first;
    _amountController = TextEditingController(text: rawAmount);
    _periodType = widget.due.periodType;
    _paymentRule = widget.due.paymentRule;
    _isVoluntary = widget.due.isVoluntary;
  }

  @override
  void dispose() {
    _nameController.dispose();
    _amountController.dispose();
    super.dispose();
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

      await ref
          .read(iuranRepositoryProvider)
          .updateDue(
            widget.due.id,
            name: name,
            amount: amount,
            periodType: _periodType,
            paymentRule: _paymentRule,
            isVoluntary: _isVoluntary,
          );

      if (mounted) {
        Navigator.of(context).pop(true);
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

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return AlertDialog(
      title: const Text('Edit Iuran'),
      content: SingleChildScrollView(
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
                      color: AppColors.danger.withValues(alpha: 0.3),
                    ),
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
              SwitchListTile(
                contentPadding: EdgeInsets.zero,
                title: const Text(
                  'Iuran Sukarela',
                  style: TextStyle(fontSize: 14, fontWeight: FontWeight.w500),
                ),
                subtitle: const Text(
                  'Nominal fleksibel / sukarela',
                  style: TextStyle(fontSize: 12),
                ),
                value: _isVoluntary,
                onChanged: _loading
                    ? null
                    : (val) {
                        setState(() {
                          _isVoluntary = val;
                          if (val) {
                            _amountController.text = '0';
                          }
                        });
                      },
              ),
              const SizedBox(height: 8),
              TextFormField(
                controller: _amountController,
                keyboardType: TextInputType.number,
                inputFormatters: [FilteringTextInputFormatter.digitsOnly],
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
                value: _periodType,
                decoration: const InputDecoration(labelText: 'Frekuensi'),
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
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: _loading ? null : () => Navigator.of(context).pop(false),
          child: const Text('Batal'),
        ),
        FilledButton(
          onPressed: _loading ? null : _handleSave,
          child: _loading
              ? const SizedBox(
                  width: 18,
                  height: 18,
                  child: CircularProgressIndicator(
                    strokeWidth: 2,
                    color: Colors.white,
                  ),
                )
              : const Text('Simpan'),
        ),
      ],
    );
  }
}
