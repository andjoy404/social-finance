import 'package:flutter/material.dart';
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

import '../../data/iuran_models.dart';
import '../../data/iuran_providers.dart';
import '../../data/iuran_access.dart';
import '../widgets/iuran_tab_bar.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';

/// Main Iuran list screen (WargaScreen equivalent).
class IuranScreen extends ConsumerStatefulWidget {
  const IuranScreen({super.key, this.myBills = false});

  final bool myBills;

  @override
  ConsumerState<IuranScreen> createState() => _IuranScreenState();
}

class _IuranScreenState extends ConsumerState<IuranScreen> {
  final TextEditingController _searchController = TextEditingController();

  @override
  void initState() {
    super.initState();
    if (widget.myBills) {
      _searchController.text = ref.read(iuranSearchQueryProvider);
    }
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) {
        final current = ref.read(iuranSelectedPeriodeProvider);
        if (current.isEmpty) {
          ref.read(iuranSelectedPeriodeProvider.notifier).state =
              currentCalendarPeriod();
        }
      }
    });
  }

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  /// Sync search controller text to provider.
  void _syncSearchToProvider() {
    ref.read(iuranSearchQueryProvider.notifier).state = _searchController.text
        .trim();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    // Watch providers for data + filtering
    final billsState = widget.myBills
        ? ref.watch(myBillsProvider)
        : ref.watch(billsProvider);
    final filtered = widget.myBills
        ? ref.watch(filteredMyBillsProvider)
        : ref.watch(filteredBillsProvider);
    final periode = ref.watch(iuranSelectedPeriodeProvider);
    final status = ref.watch(iuranSelectedStatusProvider);
    debugPrint('[IuranDebug] UI build: filtered.length=${filtered.length}');

    final user = ref.watch(authRepositoryProvider)?.valueOrNull;
    final canManage = canManageIuran(user);

    return Scaffold(
      appBar: AppBar(
        automaticallyImplyLeading: false,
        centerTitle: false,
        title: const MenuAppBarTitle(title: 'Iuran'),
        bottom: canManage
            ? IuranTabBar(selectedIndex: widget.myBills ? 1 : 0)
            : null,
      ),
      body: RefreshIndicator(
        onRefresh: _refreshBills,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // ── Search Section ──
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
                  controller: _searchController,
                  hintText: 'Cari nama, jenis iuran, atau RT...',
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
                            _searchController.clear();
                            _syncSearchToProvider();
                            setState(() {});
                          },
                        )
                      : null,
                  onChanged: (_) => _syncSearchToProvider(),
                ),
              ),
            ),

            // ── Filter Chips + Summary Count ──
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: AppSpacing.base),
              child: Row(
                children: [
                  // Periode chip
                  _buildFilterChip(
                    label: periode.isEmpty ? 'Periode' : periode,
                    selected: periode.isNotEmpty,
                    onTap: () => _showPeriodeDialog(context),
                  ),
                  const SizedBox(width: AppSpacing.sm),
                  // Status chip
                  _buildFilterChip(
                    label: status.isEmpty
                        ? 'Status'
                        : getStatusLabel(parseStatus(status)),
                    selected: status.isNotEmpty,
                    onTap: () => _showStatusDialog(context),
                  ),
                  if (periode.isNotEmpty || status.isNotEmpty) ...[
                    const SizedBox(width: AppSpacing.sm),
                    GestureDetector(
                      onTap: () {
                        ref.read(iuranSelectedPeriodeProvider.notifier).state =
                            '';
                        ref.read(iuranSelectedStatusProvider.notifier).state =
                            '';
                        setState(() {});
                      },
                      child: Icon(
                        Icons.close,
                        size: 16,
                        color: isDark
                            ? AppColors.darkTextMuted
                            : AppColors.lightTextMuted,
                      ),
                    ),
                  ],
                  const Spacer(),
                  // Count label — right-aligned
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

            // ── Content: Loading / Error / Empty / List ──
            Expanded(
              child: switch (billsState) {
                AsyncLoading() => const Center(
                  child: CircularProgressIndicator(),
                ),
                AsyncError(:final error) => Center(
                  child: SingleChildScrollView(
                    physics: widget.myBills
                        ? const AlwaysScrollableScrollPhysics()
                        : null,
                    padding: const EdgeInsets.all(AppSpacing.lg),
                    child: EmptyState(
                      icon: Icons.error_outline,
                      title: 'Gagal Memuat Data',
                      description: error.toString(),
                      actionText: 'Coba Lagi',
                      onAction: _refreshBills,
                    ),
                  ),
                ),
                _ =>
                  filtered.isEmpty
                      ? Center(
                          child: SingleChildScrollView(
                            physics: const AlwaysScrollableScrollPhysics(),
                            padding: const EdgeInsets.all(AppSpacing.lg),
                            child:
                                (billsState.valueOrNull?.isNotEmpty ?? false) &&
                                (_searchController.text.trim().isNotEmpty ||
                                    periode.isNotEmpty ||
                                    status.isNotEmpty)
                                ? EmptyState(
                                    icon: Icons.receipt_long_outlined,
                                    title: 'Tagihan Tidak Ditemukan',
                                    description:
                                        'Tidak ada tagihan yang sesuai dengan filter saat ini.',
                                    actionText: 'Reset Filter',
                                    onAction: () {
                                      _searchController.clear();
                                      ref
                                              .read(
                                                iuranSelectedPeriodeProvider
                                                    .notifier,
                                              )
                                              .state =
                                          '';
                                      ref
                                              .read(
                                                iuranSelectedStatusProvider
                                                    .notifier,
                                              )
                                              .state =
                                          '';
                                      setState(() {});
                                    },
                                  )
                                : const EmptyState(
                                    icon: Icons.receipt_long_outlined,
                                    title: 'Belum Ada Tagihan',
                                    description:
                                        'Tagihan iuran RT belum tersedia.',
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
                          separatorBuilder: (context, index) =>
                              const SizedBox(height: AppSpacing.sm),
                          itemBuilder: (context, index) {
                            final bill = filtered[index];
                            return _IuranCard(bill: bill);
                          },
                        ),
              },
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _refreshBills() {
    return widget.myBills
        ? ref.read(myBillsProvider.notifier).refresh()
        : ref.read(billsProvider.notifier).refresh();
  }

  // ── Filter Chip ──

  Widget _buildFilterChip({
    required String label,
    required bool selected,
    required VoidCallback onTap,
  }) {
    final bgColor = selected
        ? AppColors.accent.withValues(alpha: 0.12)
        : Theme.of(context).scaffoldBackgroundColor;
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
                : Theme.of(context).brightness == Brightness.dark
                ? AppColors.darkBorder
                : AppColors.lightBorder,
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
                    : (Theme.of(context).brightness == Brightness.dark
                          ? AppColors.darkTextMuted
                          : AppColors.lightTextMuted),
              ),
            ),
            const SizedBox(width: 4),
            Icon(
              Icons.keyboard_arrow_down,
              size: 16,
              color: selected
                  ? AppColors.accent
                  : (Theme.of(context).brightness == Brightness.dark
                        ? AppColors.darkTextMuted
                        : AppColors.lightTextMuted),
            ),
          ],
        ),
      ),
    );
  }

  // ── Dialogs ──

  void _showPeriodeDialog(BuildContext context) {
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
                  // "Semua" option
                  ListTile(
                    title: const Text('Semua'),
                    leading: Radio<String>(
                      value: '',
                      groupValue: periode,
                      onChanged: (value) {
                        ref.read(iuranSelectedPeriodeProvider.notifier).state =
                            value ?? '';
                        Navigator.of(dialogCtx).pop();
                      },
                    ),
                    onTap: () {
                      ref.read(iuranSelectedPeriodeProvider.notifier).state =
                          '';
                      Navigator.of(dialogCtx).pop();
                    },
                  ),
                  const Divider(),
                  // Use unique periode options from data
                  for (final p in _uniquePeriodes())
                    ListTile(
                      title: Text(p),
                      leading: Radio<String>(
                        value: p,
                        groupValue: periode,
                        onChanged: (value) {
                          ref
                                  .read(iuranSelectedPeriodeProvider.notifier)
                                  .state =
                              value ?? '';
                          Navigator.of(dialogCtx).pop();
                        },
                      ),
                      onTap: () {
                        ref.read(iuranSelectedPeriodeProvider.notifier).state =
                            p;
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

  void _showStatusDialog(BuildContext context) {
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
                // "Semua" option
                ListTile(
                  title: const Text('Semua'),
                  leading: Radio<String>(
                    value: '',
                    groupValue: status,
                    onChanged: (value) {
                      ref.read(iuranSelectedStatusProvider.notifier).state =
                          value ?? '';
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
                        ref.read(iuranSelectedStatusProvider.notifier).state =
                            value ?? '';
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

  /// Extract unique periods from the current bills list.
  List<String> _uniquePeriodes() {
    final billsState = widget.myBills
        ? ref.read(myBillsProvider)
        : ref.read(billsProvider);
    return switch (billsState) {
      AsyncData(:final value) =>
        value.map((b) => b.periode).toSet().toList()
          ..sort((a, b) => b.compareTo(a)),
      _ => const [],
    };
  }
}

/// Card item for iuran list, following _ResidentCard pattern.
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
        onTap: () {
          context.push('/home/iuran/detail/${bill.id}');
        },
        borderRadius: BorderRadius.circular(AppRadius.base),
        child: AppCard(
          padding: const EdgeInsets.all(AppSpacing.base),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // ── BAGIAN A: HEADER — satu baris horizontal ──
              Row(
                crossAxisAlignment: CrossAxisAlignment.center,
                children: [
                  // Icon placeholder
                  CircleAvatar(
                    radius: 20,
                    backgroundColor: AppColors.accent.withValues(
                      alpha: isDark ? 0.25 : 0.12,
                    ),
                    child: Icon(
                      Icons.receipt_long_outlined,
                      size: 20,
                      color: AppColors.accent,
                    ),
                  ),
                  const SizedBox(width: AppSpacing.md),

                  // Nama warga — Expanded agar tidak overflow
                  Expanded(
                    child: Text(
                      bill.householdName,
                      style: theme.textTheme.titleMedium?.copyWith(
                        fontWeight: FontWeight.w600,
                      ),
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),

                  // Badge status tagihan — kanan
                  AppBadge(
                    label: getStatusLabel(bill.status),
                    textColor: getStatusColor(bill.status, context),
                    backgroundColor: getStatusColor(
                      bill.status,
                      context,
                    ).withValues(alpha: isDark ? 0.18 : 0.10),
                  ),
                ],
              ),

              // ── BAGIAN B: GARIS PEMISAH ──
              const Divider(height: 20),

              // ── BAGIAN C: BODY — satu Row kiri badge, kanan rupiah ──
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // Kolom kiri: Badge vertikal (jenis + periode)
                  Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      AppBadge(
                        label: bill.iuranType,
                        textColor: AppColors.info,
                        backgroundColor: AppColors.info.withValues(
                          alpha: isDark ? 0.25 : 0.12,
                        ),
                      ),
                      const SizedBox(height: 6),
                      AppBadge(
                        label: bill.periode.toUpperCase(),
                        textColor: AppColors.accent,
                        backgroundColor: AppColors.accent.withValues(
                          alpha: isDark ? 0.25 : 0.12,
                        ),
                      ),
                    ],
                  ),

                  const Spacer(),

                  // Kolom kanan: Seluruh detail rupiah, rata kanan, tepi kanan sama
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
                            style: theme.textTheme.bodySmall?.copyWith(
                              color: AppColors.success,
                            ),
                            textAlign: TextAlign.right,
                          ),
                        if (bill.isInArrears)
                          Text(
                            'Sisa: ${formatRupiah(bill.remainingAmount)}',
                            style: theme.textTheme.bodySmall?.copyWith(
                              color: AppColors.warning,
                            ),
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
