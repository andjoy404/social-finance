import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/widgets/app_badge.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/app_text_field.dart';
import 'package:social_finance/core/widgets/double_back_exit_scope.dart';
import 'package:social_finance/core/widgets/empty_state.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';

import 'package:social_finance/core/models/role.dart';

import '../../data/iuran_models.dart';
import '../../data/iuran_providers.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'kelola_iuran_tab_body.dart';

/// Main Iuran screen with standard TabBar (Iuran Saya / Iuran Warga / Kelola Iuran).
/// For warga role, the TabBar is hidden and only their own bill list is shown.
class IuranScreen extends ConsumerStatefulWidget {
  const IuranScreen({super.key});

  @override
  ConsumerState<IuranScreen> createState() => _IuranScreenState();
}

class _IuranScreenState extends ConsumerState<IuranScreen>
    with SingleTickerProviderStateMixin {
  final TextEditingController _searchController = TextEditingController();
  late final TabController _tabController;
  bool _isWarga = false;

  @override
  void initState() {
    super.initState();
    _isWarga = _checkIsWarga();
    _tabController = TabController(length: _isWarga ? 1 : 3, vsync: this, initialIndex: 0);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) {
        final current = ref.read(iuranSelectedPeriodeProvider);
        if (current.isEmpty) {
          ref.read(iuranSelectedPeriodeProvider.notifier).state =
              currentCalendarPeriod();
        }
      }
    });
    _tabController.addListener(_onTabChanged);
  }

  void _onTabChanged() {
    if (!_tabController.indexIsChanging) {
      setState(() {});
    }
  }

  @override
  void dispose() {
    _searchController.dispose();
    _tabController.dispose();
    super.dispose();
  }

  bool _checkIsWarga() {
    final user = ref.read(authRepositoryProvider)?.valueOrNull;
    if (user == null) return false;
    final role = _roleValue(user['role']);
    if (role != 'warga') return false;
    // A warga with a management jabatan is not a plain warga for tab purposes.
    // They hold an RT position (ketua, wakil_ketua, sekretaris, bendahara) and
    // should see the full iuran tab navigation, not be clamped to "Iuran Saya".
    final jabatan = (user['jabatan'] ?? '').toString().trim().toLowerCase();
    final managementJabatans = {'ketua', 'wakil_ketua', 'sekretaris', 'bendahara'};
    if (managementJabatans.contains(jabatan)) return false;
    return true;
  }

  static String? _roleValue(Object? role) {
    if (role is AppRole) {
      return switch (role) {
        AppRole.superAdmin => 'super_admin',
        AppRole.pengurus => 'pengurus',
        AppRole.bendahara => 'bendahara',
        AppRole.warga => 'warga',
        AppRole.perangkat => 'perangkat',
      };
    }
    return role is String ? role : null;
  }

  @override
  Widget build(BuildContext context) {
    if (_isWarga) {
      // Warga: no tabs, show only their own bills directly
      return DoubleBackExitScope(
        child: Scaffold(
          appBar: AppBar(
            automaticallyImplyLeading: false,
            centerTitle: false,
            title: const MenuAppBarTitle(title: 'Iuran'),
          ),
          body: _BillListTab(isMyBills: true, searchController: _searchController),
        ),
      );
    }

    // Non-warga (pengurus, bendahara, etc): show full TabBar
    final isKelolaActive = _tabController.index == 2;
    return DoubleBackExitScope(
      child: Scaffold(
        appBar: AppBar(
          automaticallyImplyLeading: false,
          centerTitle: false,
          title: const MenuAppBarTitle(title: 'Iuran'),
          bottom: TabBar(
            controller: _tabController,
            labelStyle: const TextStyle(fontSize: 13, fontWeight: FontWeight.w600),
            unselectedLabelStyle: const TextStyle(fontSize: 13, fontWeight: FontWeight.w500),
            indicatorSize: TabBarIndicatorSize.tab,
            tabs: const [
              Tab(
                icon: Icon(Icons.pending_actions_outlined, size: 20),
                text: 'Iuran Saya',
              ),
              Tab(
                icon: Icon(Icons.receipt_long_outlined, size: 20),
                text: 'Iuran Warga',
              ),
              Tab(
                icon: Icon(Icons.settings_outlined, size: 20),
                text: 'Kelola Iuran',
              ),
            ],
          ),
        ),
        body: TabBarView(
          controller: _tabController,
          children: [
            _BillListTab(isMyBills: true, searchController: _searchController),
            _BillListTab(isMyBills: false, searchController: _searchController),
            KelolaIuranTabBody(),
          ],
        ),
        floatingActionButton: isKelolaActive ? _buildFab(context) : null,
      ),
    );
  }

  Widget _buildFab(BuildContext context) {
    final accentSoft = AppColors.accentSoftColor(context);
    return Container(
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
          child: const Padding(
            padding: EdgeInsets.symmetric(horizontal: 16, vertical: 8),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(Icons.add, size: 20, color: AppColors.accent),
                SizedBox(width: 4),
                Icon(Icons.receipt_long, size: 18, color: AppColors.accent),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

// ── Shared Bill List Tab (reused for both Iuran Saya & Iuran Warga) ────

class _BillListTab extends ConsumerWidget {
  final bool isMyBills;
  final TextEditingController searchController;

  const _BillListTab({
    required this.isMyBills,
    required this.searchController,
  });

  /// Determine whether the household name should be shown in the bill card.
  ///
  /// Rules:
  /// - Warga role: always hidden (they only see their own bills).
  /// - Pengurus/Bendahara on "Iuran Saya" tab: hidden.
  /// - Pengurus/Bendahara on "Iuran Warga" tab: visible.
  bool _shouldShowHouseholdName(WidgetRef ref) {
    final authState = ref.read(authRepositoryProvider);
    final user = authState?.valueOrNull;
    if (user == null) return false;

    final role = _roleValue(user['role']);
    // WARGA: always hide household name
    if (role == 'warga') return false;

    // Non-warga (pengurus/bendahara): show only on "Iuran Warga" tab
    return !isMyBills;
  }

  static String? _roleValue(Object? role) {
    if (role is AppRole) {
      return switch (role) {
        AppRole.superAdmin => 'super_admin',
        AppRole.pengurus => 'pengurus',
        AppRole.bendahara => 'bendahara',
        AppRole.warga => 'warga',
        AppRole.perangkat => 'perangkat',
      };
    }
    return role is String ? role : null;
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    final billsState =
        isMyBills ? ref.watch(myBillsProvider) : ref.watch(billsProvider);
    final filtered = isMyBills
        ? ref.watch(filteredMyBillsProvider)
        : ref.watch(filteredBillsProvider);
    final periode = ref.watch(iuranSelectedPeriodeProvider);
    final status = ref.watch(iuranSelectedStatusProvider);
    final paymentStatus = ref.watch(iuranSelectedPaymentStatusProvider);

    // Expanded state keyed by tab index (0=my bills, 1=warga bills)
    final filterIndex = isMyBills ? 0 : 1;
    final isExpanded = ref.watch(iuranFilterExpandedProvider(filterIndex));
    final onToggleFilter = (bool v) =>
        ref.read(iuranFilterExpandedProvider(filterIndex).notifier).state = v;

    debugPrint(
        '[IuranDebug] Tab ${isMyBills ? "my" : "warga"} filtered=${filtered.length} billsState=${billsState.when(data: (d) => d.length, loading: () => '-', error: (e, _) => "err")}');

    return RefreshIndicator(
      onRefresh: isMyBills
          ? () => ref.read(myBillsProvider.notifier).refresh()
          : () => ref.read(billsProvider.notifier).refresh(),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const SizedBox(height: AppSpacing.sm),

          // ── Filter Box Header (expand/collapse trigger) ──
          InkWell(
            onTap: () => onToggleFilter(!isExpanded),
            borderRadius: BorderRadius.circular(AppRadius.base),
            child: Container(
              width: double.infinity,
              padding: const EdgeInsets.symmetric(horizontal: AppSpacing.base, vertical: 10),
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
                    isExpanded
                        ? Icons.keyboard_arrow_up
                        : Icons.keyboard_arrow_down,
                    color: AppColors.accent,
                    size: 20,
                  ),
                  const SizedBox(width: 8),
                  const Text(
                    'Filter',
                    style: TextStyle(
                      fontSize: 14,
                      fontWeight: FontWeight.w600,
                      color: AppColors.accent,
                    ),
                  ),
                  const Spacer(),
                  // ── Badge ──
                  Builder(
                    builder: (ctx) {
                      final count = [
                        if (periode.isNotEmpty) 1,
                        if (status.isNotEmpty) 1,
                        if (paymentStatus.isNotEmpty) 1,
                      ].length;
                      return count > 0
                          ? Container(
                              padding: const EdgeInsets.symmetric(
                                  horizontal: 8, vertical: 2),
                              decoration: BoxDecoration(
                                color: AppColors.accent,
                                borderRadius: BorderRadius.circular(10),
                              ),
                              child: Text(
                                '$count',
                                style: const TextStyle(
                                  color: Colors.white,
                                  fontSize: 11,
                                  fontWeight: FontWeight.bold,
                                ),
                              ),
                            )
                          : const SizedBox.shrink();
                    },
                  ),
                ],
              ),
            ),
          ),

          // ── Filter Box Content (expandable) ──
          AnimatedSize(
            duration: const Duration(milliseconds: 200),
            curve: Curves.easeInOut,
            child: isExpanded
                ? Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      const SizedBox(height: 12),

                      // ── Row 1: Bulan + Tahun ──
                      Padding(
                        padding: const EdgeInsets.symmetric(
                          horizontal: AppSpacing.base,
                        ),
                        child: Row(
                          children: [
                            Expanded(
                              child: InkWell(
                                onTap: () => _pickMonth(context, ref),
                                borderRadius: BorderRadius.circular(AppRadius.base),
                                child: Container(
                                  height: 38,
                                  alignment: Alignment.centerLeft,
                                  padding: const EdgeInsets.symmetric(horizontal: 10),
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
                                  ),
                                  child: Text(
                                    _monthNameFromPeriode(periode),
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
                                onTap: () => _pickYear(context, ref),
                                borderRadius: BorderRadius.circular(AppRadius.base),
                                child: Container(
                                  height: 38,
                                  alignment: Alignment.centerLeft,
                                  padding: const EdgeInsets.symmetric(horizontal: 10),
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
                                  ),
                                  child: Text(
                                    _yearFromPeriode(periode),
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

                      // ── Row 2: Status + Pembayaran ──
                      Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.base),
            child: Row(
              children: [
                Expanded(
                  child: Container(
                    height: 38,
                    padding: const EdgeInsets.symmetric(horizontal: 10),
                    decoration: BoxDecoration(
                      color: isDark ? AppColors.darkSurface : AppColors.lightSurface,
                      borderRadius: BorderRadius.circular(AppRadius.base),
                      border: Border.all(
                          color: isDark
                              ? AppColors.accent.withValues(alpha: 0.35)
                              : AppColors.accent.withValues(alpha: 0.25),
                          width: 1,
                    ),
                    ),
                    child: DropdownButtonHideUnderline(
                      child: DropdownButton<String>(
                        value: status.isNotEmpty ? status : null,
                        isExpanded: true,
                        hint: const Text('Status'),
                        style: theme.textTheme.bodySmall?.copyWith(
                            fontWeight: FontWeight.w600),
                        dropdownColor: isDark
                            ? AppColors.darkSurface
                            : AppColors.lightSurface,
                        items: [
                          const DropdownMenuItem(value: null, child: Text('Semua')),
                          ...IuranStatus.values.map((s) => DropdownMenuItem(
                                value: s.name,
                                child: Text(getStatusLabel(s)),
                              )),
                        ],
                        onChanged: (val) {
                          ref.read(iuranSelectedStatusProvider.notifier).state = val ?? '';
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
                      color: isDark ? AppColors.darkSurface : AppColors.lightSurface,
                      borderRadius: BorderRadius.circular(AppRadius.base),
                      border: Border.all(
                          color: isDark
                              ? AppColors.accent.withValues(alpha: 0.35)
                              : AppColors.accent.withValues(alpha: 0.25),
                          width: 1,
                    ),
                    ),
                    child: DropdownButtonHideUnderline(
                      child: DropdownButton<String>(
                        value: paymentStatus.isNotEmpty ? paymentStatus : null,
                        isExpanded: true,
                        hint: const Text('Pembayaran'),
                        style: theme.textTheme.bodySmall?.copyWith(
                            fontWeight: FontWeight.w600),
                        dropdownColor: isDark
                            ? AppColors.darkSurface
                            : AppColors.lightSurface,
                        items: [
                          const DropdownMenuItem(value: null, child: Text('Semua')),
                          const DropdownMenuItem(value: 'pending', child: Text('Menunggu')),
                          const DropdownMenuItem(value: 'approved', child: Text('Disetujui')),
                          const DropdownMenuItem(value: 'rejected', child: Text('Ditolak')),
                        ],
                        onChanged: (val) {
                          ref.read(iuranSelectedPaymentStatusProvider.notifier).state = val ?? '';
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
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.base),
            child: SizedBox(
              height: 38,
              child: AppTextField(
                controller: searchController,
                hintText: 'Cari nama, jenis iuran, atau RT...',
                isDense: true,
                fillColor: isDark
                    ? AppColors.darkSurface
                    : AppColors.lightSurface,
                textAlignVertical: TextAlignVertical.center,
                contentPadding:
                    const EdgeInsets.symmetric(horizontal: 12, vertical: 0),
                prefixIconConstraints:
                    const BoxConstraints(minWidth: 36, minHeight: 38),
                suffixIconConstraints:
                    const BoxConstraints(minWidth: 36, minHeight: 38),
                prefixIcon: const Icon(Icons.search, size: 18),
                suffixIcon: searchController.text.isNotEmpty
                    ? IconButton(
                        padding: EdgeInsets.zero,
                        icon: const Icon(Icons.clear, size: 18),
                        onPressed: () {
                          searchController.clear();
                          ref.read(iuranSearchQueryProvider.notifier).state = '';
                        },
                      )
                    : null,
                onChanged: (_) {
                  ref.read(iuranSearchQueryProvider.notifier).state =
                      searchController.text.trim();
                },
              ),
            ),
          ),

                      const SizedBox(height: AppSpacing.xs),
                    ],
                  )
                : const SizedBox.shrink(),
          ),

          const SizedBox(height: AppSpacing.sm),

          // ── Bill list ──
          Expanded(
            child: switch (billsState) {
              AsyncLoading() =>
                const Center(child: CircularProgressIndicator()),
              AsyncError(:final error) => Center(
                  child: SingleChildScrollView(
                    physics: const AlwaysScrollableScrollPhysics(),
                    padding: const EdgeInsets.all(AppSpacing.lg),
                    child: EmptyState(
                      icon: Icons.error_outline,
                      title: 'Gagal Memuat Data',
                      description: error.toString(),
                      actionText: 'Coba Lagi',
                      onAction: () {},
                    ),
                  ),
                ),
              _ => filtered.isEmpty
                  ? Center(
                      child: SingleChildScrollView(
                        physics: const AlwaysScrollableScrollPhysics(),
                        padding: const EdgeInsets.all(AppSpacing.lg),
                        child: EmptyState(
                          icon: Icons.receipt_long_outlined,
                          title: 'Belum Ada Tagihan',
                          description: 'Tagihan iuran RT belum tersedia.',
                          actionText: (billsState.valueOrNull?.isNotEmpty ??
                                  false)
                              ? 'Reset Filter'
                              : null,
                          onAction: (billsState.valueOrNull?.isNotEmpty ??
                                  false)
                              ? () {
                                  searchController.clear();
                                  ref.read(
                                      iuranSelectedPeriodeProvider.notifier)
                                      .state = '';
                                  ref.read(
                                      iuranSelectedStatusProvider.notifier)
                                      .state = '';
                                  ref.read(
                                      iuranSelectedPaymentStatusProvider
                                          .notifier)
                                      .state = '';
                                }
                              : null,
                        ),
                      ),
                    )
                  : ListView.separated(
                      physics: const AlwaysScrollableScrollPhysics(),
                      padding: const EdgeInsets.fromLTRB(
                        AppSpacing.base,
                        AppSpacing.xs,
                        AppSpacing.base,
                        AppSpacing.xl,
                      ),
                      itemCount: filtered.length,
                      separatorBuilder: (_, __) =>
                          const SizedBox(height: AppSpacing.sm),
                      itemBuilder: (context, index) =>
                          _IuranCard(
                            bill: filtered[index],
                            showHouseholdName: _shouldShowHouseholdName(ref),
                          ),
                    ),
            },
          ),
        ],
      ),
    );
  }

  /// Navigate to previous month period (YYYY-MM)
  String _prevPeriode(String periode) {
    if (periode.isEmpty) return periode;
    final parts = periode.split('-');
    if (parts.length != 2) return periode;
    final year = int.tryParse(parts[0]) ?? DateTime.now().year;
    final month = int.tryParse(parts[1]) ?? 1;
    final prev = DateTime(year, month - 1, 1);
    return '${prev.year}-${prev.month.toString().padLeft(2, '0')}';
  }

  /// Navigate to next month period (YYYY-MM)
  String _nextPeriode(String periode) {
    if (periode.isEmpty) return periode;
    final parts = periode.split('-');
    if (parts.length != 2) return periode;
    final year = int.tryParse(parts[0]) ?? DateTime.now().year;
    final month = int.tryParse(parts[1]) ?? 1;
    final next = DateTime(year, month + 1, 1);
    return '${next.year}-${next.month.toString().padLeft(2, '0')}';
  }

  /// Select a specific periode and update the filter
  void _selectSpecificPeriode(BuildContext context, WidgetRef ref, String periode) {
    if (periode.isEmpty) return;
    ref.read(iuranSelectedPeriodeProvider.notifier).state = periode;
  }

  static const _monthNames = [
    'Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni',
    'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember',
  ];

  String _monthNameFromPeriode(String periode) {
    if (periode.isEmpty) {
      return 'Semua Bulan';
    }
    final parts = periode.split('-');
    if (parts.length >= 2) {
      final month = int.tryParse(parts[1]);
      if (month != null && month >= 1 && month <= 12) {
        return _monthNames[month - 1];
      }
    }
    return _monthNames[DateTime.now().month - 1];
  }

  String _yearFromPeriode(String periode) {
    if (periode.isEmpty) {
      return DateTime.now().year.toString();
    }
    final parts = periode.split('-');
    final year = int.tryParse(parts[0]);
    return year?.toString() ?? DateTime.now().year.toString();
  }

  Future<void> _pickMonth(BuildContext context, WidgetRef ref) async {
    final periode = ref.read(iuranSelectedPeriodeProvider);
    final parts = periode.split('-');
    final currentYear = DateTime.now().year;
    int selYear = currentYear;
    int selMonth = DateTime.now().month;
    if (parts.length >= 2) {
      selYear = int.tryParse(parts[0]) ?? currentYear;
      selMonth = int.tryParse(parts[1]) ?? DateTime.now().month;
    }

    final isDark = Theme.of(context).brightness == Brightness.dark;

    return await showModalBottomSheet<void>(
      context: context,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(AppRadius.base)),
      ),
      builder: (ctx) => StatefulBuilder(
        builder: (_, setModalState) {
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
                    childAspectRatio: 2.2,
                    crossAxisSpacing: 8,
                    mainAxisSpacing: 8,
                  ),
                  itemCount: 13,
                  itemBuilder: (_, i) {
                    final isAll = i == 0;
                    final months = ['Semua Bulan', 'Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni', 'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember'];
                    final isSelected = isAll ? (ref.read(iuranSelectedPeriodeProvider).isEmpty) : (ref.read(iuranSelectedPeriodeProvider).isNotEmpty && i == selMonth);
                    return InkWell(
                      onTap: () {
                        if (isAll) {
                          ref.read(iuranSelectedPeriodeProvider.notifier).state = '';
                        } else {
                          final newPeriode = '${selYear.toString().padLeft(4, '0')}-${(i).toString().padLeft(2, '0')}';
                          ref.read(iuranSelectedPeriodeProvider.notifier).state = newPeriode;
                        }
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

  Future<void> _pickYear(BuildContext context, WidgetRef ref) async {
    final periode = ref.read(iuranSelectedPeriodeProvider);
    final parts = periode.split('-');
    final currentYear = DateTime.now().year;
    int selYear = int.tryParse(parts[0]) ?? currentYear;

    final isDark = Theme.of(context).brightness == Brightness.dark;

    await showModalBottomSheet<void>(
      context: context,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(AppRadius.base)),
      ),
      builder: (ctx) => StatefulBuilder(
        builder: (_, setModalState) {
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
                        setModalState(() => selYear = year);
                        final currentPeriode = ref.read(iuranSelectedPeriodeProvider);
                        if (currentPeriode.isEmpty) {
                          // Keep "Semua Bulan" — just update year context
                          ref.read(iuranSelectedPeriodeProvider.notifier).state = '';
                        } else {
                          final parts2 = currentPeriode.split('-');
                          final month = (parts2.length > 1 ? int.tryParse(parts2[1]) : null) ?? DateTime.now().month;
                          final newPeriode = '${selYear.toString().padLeft(4, '0')}-${month.toString().padLeft(2, '0')}';
                          ref.read(iuranSelectedPeriodeProvider.notifier).state = newPeriode;
                        }
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

  List<String> _uniquePeriodes(WidgetRef ref) {
    final bs = isMyBills ? ref.read(myBillsProvider) : ref.read(billsProvider);
    return switch (bs) {
      AsyncData(:final value) =>
        value.map((b) => b.periode).toSet().toList()..sort((a, b) => b.compareTo(a)),
      _ => const [],
    };
  }
}

// ── Iuran Card ────────────────────────────────────────────────────────

class _IuranCard extends StatelessWidget {
  final IuranBill bill;
  final bool showHouseholdName;

  const _IuranCard({required this.bill, required this.showHouseholdName});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: () => context.push('/home/iuran/detail/${bill.id}'),
        borderRadius: BorderRadius.circular(AppRadius.base),
        child: AppCard(
          padding: const EdgeInsets.all(AppSpacing.base),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Header
              Row(
                crossAxisAlignment: CrossAxisAlignment.center,
                children: [
                  CircleAvatar(
                    radius: 20,
                    backgroundColor: AppColors.accent.withValues(
                        alpha: isDark ? 0.25 : 0.12),
                    child: Icon(Icons.receipt_long_outlined,
                        size: 20, color: AppColors.accent),
                  ),
                  const SizedBox(width: AppSpacing.md),
                  if (showHouseholdName)
                    Expanded(
                      child: Text(
                        bill.householdName,
                        style: theme.textTheme.titleMedium
                            ?.copyWith(fontWeight: FontWeight.w600),
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                  AppBadge(
                    label: getStatusLabel(bill.status),
                    textColor: getStatusColor(bill.status, context),
                    backgroundColor: getStatusColor(bill.status, context)
                        .withValues(alpha: isDark ? 0.18 : 0.10),
                  ),
                ],
              ),
              const Divider(height: 20),
              // Body
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      AppBadge(
                        label: bill.iuranType,
                        textColor: AppColors.info,
                        backgroundColor: AppColors.info
                            .withValues(alpha: isDark ? 0.25 : 0.12),
                      ),
                      const SizedBox(height: 6),
                      AppBadge(
                        label: bill.periode.toUpperCase(),
                        textColor: AppColors.accent,
                        backgroundColor: AppColors.accent
                            .withValues(alpha: isDark ? 0.25 : 0.12),
                      ),
                    ],
                  ),
                  const Spacer(),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.end,
                      children: [
                        Text(
                          formatRupiah(bill.nominal),
                          style: theme.textTheme.bodyMedium,
                          textAlign: TextAlign.right,
                        ),
                        if (bill.paidAmount > 0)
                          Text(
                            'Dibayar: ${formatRupiah(bill.paidAmount)}',
                            style: theme.textTheme.bodySmall
                                ?.copyWith(color: AppColors.success),
                            textAlign: TextAlign.right,
                          ),
                        if (bill.pendingAmount > 0)
                          Text(
                            'Tertunda ${formatRupiah(bill.pendingAmount)}',
                            style: theme.textTheme.bodySmall?.copyWith(
                              color: AppColors.warning,
                              fontWeight: FontWeight.w600,
                            ),
                            textAlign: TextAlign.right,
                          ),
                        if (bill.isInArrears)
                          Text(
                            'Sisa ${formatRupiah(bill.remainingAmount)}',
                            style: theme.textTheme.bodySmall
                                ?.copyWith(color: AppColors.warning),
                            textAlign: TextAlign.right,
                          ),
                      ],
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}
