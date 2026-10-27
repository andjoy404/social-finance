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
import '../../data/iuran_access.dart';
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

  void _syncSearchToProvider() {
    ref.read(iuranSearchQueryProvider.notifier).state =
        _searchController.text.trim();
  }

  void _resetFilters() {
    _searchController.clear();
    ref.read(iuranSelectedPeriodeProvider.notifier).state = '';
    ref.read(iuranSelectedStatusProvider.notifier).state = '';
    ref.read(iuranSelectedPaymentStatusProvider.notifier).state = '';
    setState(() {});
  }

  bool _checkIsWarga() {
    final user = ref.read(authRepositoryProvider)?.valueOrNull;
    if (user == null) return false;
    final role = _roleValue(user['role']);
    return role == 'warga';
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

    debugPrint(
        '[IuranDebug] Tab ${isMyBills ? "my" : "warga"} filtered=${filtered.length} billsState=${billsState.when(data: (d) => d.length, loading: () => '-', error: (e, _) => "err")}');

    return RefreshIndicator(
      onRefresh: isMyBills
          ? () => ref.read(myBillsProvider.notifier).refresh()
          : () => ref.read(billsProvider.notifier).refresh(),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // ── Search ──
          Padding(
            padding: const EdgeInsets.only(
              top: AppSpacing.sm,
              left: AppSpacing.base,
              right: AppSpacing.base,
              bottom: AppSpacing.sm,
            ),
            child: SizedBox(
              height: 38,
              child: AppTextField(
                controller: searchController,
                hintText: 'Cari nama, jenis iuran, atau RT...',
                isDense: true,
                fillColor: theme.scaffoldBackgroundColor,
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

          // ── Filter chips + count ──
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.base),
            child: Row(
              children: [
                Expanded(
                  child: SingleChildScrollView(
                    scrollDirection: Axis.horizontal,
                    child: Row(
                      children: [
                        _FilterChip(
                          label:
                              periode.isEmpty ? 'Periode' : periode,
                          selected: periode.isNotEmpty,
                          onTap: () => _showPeriodeDialog(context, ref),
                        ),
                        const SizedBox(width: AppSpacing.sm),
                        _FilterChip(
                          label: status.isEmpty
                              ? 'Status'
                              : getStatusLabel(parseStatus(status)),
                          selected: status.isNotEmpty,
                          onTap: () => _showStatusDialog(context, ref),
                        ),
                        const SizedBox(width: AppSpacing.sm),
                        _FilterChip(
                          label: _paymentStatusLabel(paymentStatus),
                          selected: paymentStatus.isNotEmpty,
                          onTap: () =>
                              _showPaymentStatusDialog(context, ref),
                        ),
                        if (periode.isNotEmpty ||
                            status.isNotEmpty ||
                            paymentStatus.isNotEmpty) ...[
                          const SizedBox(width: AppSpacing.sm),
                          GestureDetector(
                            onTap: () {
                              ref
                                  .read(
                                      iuranSelectedPeriodeProvider.notifier)
                                  .state = '';
                              ref
                                  .read(iuranSelectedStatusProvider.notifier)
                                  .state = '';
                              ref.read(
                                  iuranSelectedPaymentStatusProvider.notifier)
                                  .state = '';
                            },
                            child: const Icon(
                              Icons.close,
                              size: 16,
                            ),
                          ),
                        ],
                      ],
                    ),
                  ),
                ),
                const SizedBox(width: AppSpacing.sm),
                Text(
                  '${filtered.length} Iuran',
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ],
            ),
          ),

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
                          _IuranCard(bill: filtered[index]),
                    ),
            },
          ),
        ],
      ),
    );
  }

  String _paymentStatusLabel(String ps) {
    return switch (ps.toLowerCase()) {
      'pending' => 'Menunggu Verifikasi',
      'approved' => 'Disetujui',
      'rejected' => 'Ditolak',
      _ => 'Pembayaran',
    };
  }

  void _showPeriodeDialog(BuildContext context, WidgetRef ref) {
    final periode = ref.read(iuranSelectedPeriodeProvider);
    showDialog<void>(
      context: context,
      builder: (dialogCtx) {
        return AlertDialog(
          title: const Text('Pilih Periode'),
          content: SizedBox(
            width: double.maxFinite,
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  ListTile(
                    title: const Text('Semua'),
                    leading: Radio<String>(
                      value: '',
                      groupValue: periode,
                      onChanged: (value) {
                        ref
                            .read(iuranSelectedPeriodeProvider.notifier)
                            .state = value ?? '';
                        Navigator.of(dialogCtx).pop();
                      },
                    ),
                    onTap: () {
                      ref
                          .read(iuranSelectedPeriodeProvider.notifier)
                          .state = '';
                      Navigator.of(dialogCtx).pop();
                    },
                  ),
                  const Divider(),
                  // Unique periodes from current bills
                  for (final p in _uniquePeriodes(ref))
                    ListTile(
                      title: Text(p),
                      leading: Radio<String>(
                        value: p,
                        groupValue: periode,
                        onChanged: (value) {
                          ref
                              .read(iuranSelectedPeriodeProvider.notifier)
                              .state = value ?? '';
                          Navigator.of(dialogCtx).pop();
                        },
                      ),
                      onTap: () {
                        ref
                            .read(iuranSelectedPeriodeProvider.notifier)
                            .state = p;
                        Navigator.of(dialogCtx).pop();
                      },
                    ),
                ],
              ),
            ),
          ),
        );
      },
    );
  }

  void _showStatusDialog(BuildContext context, WidgetRef ref) {
    final status = ref.read(iuranSelectedStatusProvider);
    showDialog<void>(
      context: context,
      builder: (dialogCtx) {
        return AlertDialog(
          title: const Text('Pilih Status'),
          content: SizedBox(
            width: double.maxFinite,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                ListTile(
                  title: const Text('Semua'),
                  leading: Radio<String>(
                    value: '',
                    groupValue: status,
                    onChanged: (value) {
                      ref
                          .read(iuranSelectedStatusProvider.notifier)
                          .state = value ?? '';
                      Navigator.of(dialogCtx).pop();
                    },
                  ),
                  onTap: () {
                    ref.read(iuranSelectedStatusProvider.notifier).state = '';
                    Navigator.of(dialogCtx).pop();
                  },
                ),
                const Divider(),
                for (final s in IuranStatus.values)
                  ListTile(
                    title: Text(getStatusLabel(s)),
                    leading: Radio<String>(
                      value: s.name,
                      groupValue: status,
                      onChanged: (value) {
                        ref
                            .read(iuranSelectedStatusProvider.notifier)
                            .state = value ?? '';
                        Navigator.of(dialogCtx).pop();
                      },
                    ),
                    onTap: () {
                      ref.read(iuranSelectedStatusProvider.notifier).state =
                          s.name;
                      Navigator.of(dialogCtx).pop();
                    },
                  ),
              ],
            ),
          ),
        );
      },
    );
  }

  void _showPaymentStatusDialog(BuildContext context, WidgetRef ref) {
    final paymentStatus = ref.read(iuranSelectedPaymentStatusProvider);
    showDialog<void>(
      context: context,
      builder: (dialogCtx) {
        return AlertDialog(
          title: const Text('Status Pembayaran'),
          content: SizedBox(
            width: double.maxFinite,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                ListTile(
                  title: const Text('Semua'),
                  leading: Radio<String>(
                    value: '',
                    groupValue: paymentStatus,
                    onChanged: (value) {
                      ref
                          .read(iuranSelectedPaymentStatusProvider.notifier)
                          .state = value ?? '';
                      Navigator.of(dialogCtx).pop();
                    },
                  ),
                  onTap: () {
                    ref
                        .read(iuranSelectedPaymentStatusProvider.notifier)
                        .state = '';
                    Navigator.of(dialogCtx).pop();
                  },
                ),
                const Divider(),
                for (final entry in [
                  ('pending', 'Menunggu Verifikasi'),
                  ('approved', 'Disetujui'),
                  ('rejected', 'Ditolak'),
                ])
                  ListTile(
                    title: Text(entry.$2),
                    leading: Radio<String>(
                      value: entry.$1,
                      groupValue: paymentStatus,
                      onChanged: (value) {
                        ref
                            .read(
                                iuranSelectedPaymentStatusProvider.notifier)
                            .state = value ?? '';
                        Navigator.of(dialogCtx).pop();
                      },
                    ),
                    onTap: () {
                      ref
                          .read(iuranSelectedPaymentStatusProvider.notifier)
                          .state = entry.$1;
                      Navigator.of(dialogCtx).pop();
                    },
                  ),
              ],
            ),
          ),
        );
      },
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

// ── Filter Chip ───────────────────────────────────────────────────────

class _FilterChip extends StatelessWidget {
  final String label;
  final bool selected;
  final VoidCallback onTap;

  const _FilterChip({
    required this.label,
    required this.selected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;
    final bgColor = selected
        ? AppColors.accent.withValues(alpha: 0.12)
        : theme.scaffoldBackgroundColor;
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
        decoration: BoxDecoration(
          color: bgColor,
          borderRadius: BorderRadius.circular(AppRadius.base),
          border: Border.all(
            color: selected
                ? AppColors.accent
                : isDark ? AppColors.darkBorder : AppColors.lightBorder,
            width: 0.5,
          ),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              label,
              style: TextStyle(
                fontSize: 12,
                fontWeight: FontWeight.w500,
                color: selected
                    ? AppColors.accent
                    : isDark
                        ? AppColors.darkTextMuted
                        : AppColors.lightTextMuted,
              ),
            ),
            const SizedBox(width: 4),
            Icon(
              Icons.keyboard_arrow_down,
              size: 16,
              color: selected
                  ? AppColors.accent
                  : isDark
                      ? AppColors.darkTextMuted
                      : AppColors.lightTextMuted,
            ),
          ],
        ),
      ),
    );
  }
}

// ── Iuran Card ────────────────────────────────────────────────────────

class _IuranCard extends StatelessWidget {
  final IuranBill bill;

  const _IuranCard({required this.bill});

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
