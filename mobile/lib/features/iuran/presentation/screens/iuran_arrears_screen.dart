import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/widgets/app_badge.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/app_text_field.dart';
import 'package:social_finance/core/widgets/empty_state.dart';
import 'package:social_finance/core/widgets/section_header.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';

import '../../data/iuran_models.dart';
import '../../data/iuran_mock_data.dart';

/// Screen showing arrears (unpaid + partially paid) iuran bills.
class IuranArrearsScreen extends StatefulWidget {
  const IuranArrearsScreen({super.key});

  @override
  State<IuranArrearsScreen> createState() => _IuranArrearsScreenState();
}

class _IuranArrearsScreenState extends State<IuranArrearsScreen> {
  late final TextEditingController _searchController;
  String _selectedPeriode = '';

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

  void _clearSearch() {
    _searchController.clear();
    setState(() {});
  }

  List<IuranBill> _getArrearsBills() {
    final query = _searchController.text.trim().toLowerCase();

    return IuranMockData.bills.where((bill) {
      // Only unpaid or partially paid
      if (bill.status == IuranStatus.lunas) return false;

      // Search filter
      final searchMatch = query.isEmpty ||
          bill.householdName.toLowerCase().contains(query) ||
          bill.iuranType.toLowerCase().contains(query) ||
          bill.rt.contains(query);

      // Periode filter
      final periodeMatch =
          _selectedPeriode.isEmpty || bill.periode == _selectedPeriode;

      return searchMatch && periodeMatch;
    }).toList();
  }

  /// Calculate total arrears from the arrears list.
  int _calculateTotalArrears() {
    final arrears = _getArrearsBills();
    return arrears.fold(0, (sum, bill) => sum + bill.remainingAmount);
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;
    final arrears = _getArrearsBills();
    final totalArrears = _calculateTotalArrears();

    return Scaffold(
      appBar: AppBar(
        centerTitle: false,
        title: const MenuAppBarTitle(title: 'Tunggakan'),
        leading: IconButton(
          icon: const Icon(Icons.arrow_back),
          onPressed: () => context.pop(),
        ),
      ),
      body: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // ── Search Section ──
          Padding(
            padding: const EdgeInsets.fromLTRB(
              AppSpacing.base,
              AppSpacing.sm,
              AppSpacing.base,
              AppSpacing.sm,
            ),
            child: AppTextField(
              controller: _searchController,
              hintText: 'Cari nama atau jenis iuran...',
              prefixIcon: const Icon(Icons.search),
              suffixIcon: _searchController.text.isNotEmpty
                  ? IconButton(
                      icon: const Icon(Icons.clear, size: 20),
                      onPressed: _clearSearch,
                    )
                  : null,
              onChanged: (_) => setState(() {}),
            ),
          ),

          // ── Filter Chip ──
          Padding(
            padding:
                const EdgeInsets.symmetric(horizontal: AppSpacing.base),
            child: Row(
              children: [
                _buildFilterChip(
                  label: _selectedPeriode.isEmpty
                      ? 'Periode'
                      : _selectedPeriode,
                  selected: _selectedPeriode.isNotEmpty,
                  onTap: _showPeriodeDialog,
                ),
              ],
            ),
          ),

          // ── Summary Card ──
          Padding(
            padding: const EdgeInsets.fromLTRB(
              AppSpacing.base,
              AppSpacing.sm,
              AppSpacing.base,
              AppSpacing.xs,
            ),
            child: Container(
              padding: const EdgeInsets.all(AppSpacing.base),
              decoration: BoxDecoration(
                color: AppColors.warning.withValues(alpha: isDark ? 0.15 : 0.08),
                borderRadius: BorderRadius.circular(AppRadius.base),
                border: Border.all(
                  color: AppColors.warning.withValues(alpha: isDark ? 0.3 : 0.2),
                ),
              ),
              child: Row(
                children: [
                  Icon(
                    Icons.warning_amber_rounded,
                    color: AppColors.warning,
                    size: 20,
                  ),
                  const SizedBox(width: AppSpacing.sm),
                  Expanded(
                    child: Text(
                      'Total Tunggakan',
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: AppColors.warning,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ),
                  Text(
                    formatRupiah(totalArrears),
                    style: theme.textTheme.titleMedium?.copyWith(
                      color: AppColors.warning,
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                ],
              ),
            ),
          ),

          // ── Count ──
          Padding(
            padding: const EdgeInsets.symmetric(
              horizontal: AppSpacing.base,
              vertical: AppSpacing.xs,
            ),
            child: Text(
              '${arrears.length} tagihan belum terselesaikan',
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
                fontWeight: FontWeight.w600,
              ),
            ),
          ),

          // ── Arrears List ──
          Expanded(
            child: arrears.isEmpty
                ? Center(
                    child: SingleChildScrollView(
                      padding: const EdgeInsets.all(AppSpacing.lg),
                      child: const EmptyState(
                        icon: Icons.check_circle_outline,
                        title: 'Tidak Ada Tunggakan',
                        description:
                            'Semua tagihan iuran sudah terselesaikan.',
                      ),
                    ),
                  )
                : ListView.separated(
                    padding: const EdgeInsets.fromLTRB(
                      AppSpacing.base,
                      AppSpacing.xs,
                      AppSpacing.base,
                      AppSpacing.xl,
                    ),
                    itemCount: arrears.length,
                    separatorBuilder: (context, index) =>
                        const SizedBox(height: AppSpacing.sm),
                    itemBuilder: (context, index) {
                      final bill = arrears[index];
                      return _ArrearsCard(bill: bill);
                    },
                  ),
          ),
        ],
      ),
    );
  }

  Widget _buildFilterChip({
    required String label,
    required bool selected,
    required VoidCallback onTap,
  }) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
        decoration: BoxDecoration(
          color: selected
              ? AppColors.accent.withValues(alpha: 0.12)
              : (Theme.of(context).brightness == Brightness.dark
                  ? AppColors.darkSurface
                  : AppColors.lightSurface),
          borderRadius: BorderRadius.circular(AppRadius.base),
          border: Border.all(
            color: selected
                ? AppColors.accent
                : (Theme.of(context).brightness == Brightness.dark
                    ? AppColors.darkBorder
                    : AppColors.lightBorder),
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

  void _showPeriodeDialog() {
    showDialog<void>(
      context: context,
      builder: (dialogCtx) {
        return AlertDialog(
          title: const Text('Pilih Periode'),
          content: SizedBox(
            width: double.maxFinite,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                ListTile(
                  title: const Text('Semua'),
                  leading: Radio<String>(
                    value: '',
                    groupValue: _selectedPeriode,
                    onChanged: (value) {
                      setState(() => _selectedPeriode = value!);
                      Navigator.of(dialogCtx).pop();
                    },
                  ),
                  onTap: () {
                    setState(() => _selectedPeriode = '');
                    Navigator.of(dialogCtx).pop();
                  },
                ),
                const Divider(),
                ...IuranMockData.periodeOptions.map((periode) {
                  return ListTile(
                    title: Text(periode),
                    leading: Radio<String>(
                      value: periode,
                      groupValue: _selectedPeriode,
                      onChanged: (value) {
                        setState(() => _selectedPeriode = value!);
                        Navigator.of(dialogCtx).pop();
                      },
                    ),
                    onTap: () {
                      setState(() => _selectedPeriode = periode);
                      Navigator.of(dialogCtx).pop();
                    },
                  );
                }),
              ],
            ),
          ),
        );
      },
    );
  }
}

/// Card for arrears list, showing the outstanding amount prominently.
class _ArrearsCard extends StatelessWidget {
  final IuranBill bill;

  const _ArrearsCard({required this.bill});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;
    final iconColor = isDark
        ? AppColors.darkTextMuted
        : AppColors.lightTextMuted;

    return AppCard(
      padding: const EdgeInsets.all(AppSpacing.base),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              CircleAvatar(
                radius: 20,
                backgroundColor: AppColors.warning.withValues(
                  alpha: isDark ? 0.2 : 0.12,
                ),
                child: Icon(
                  Icons.warning_amber_rounded,
                  size: 20,
                  color: AppColors.warning,
                ),
              ),
              const SizedBox(width: AppSpacing.md),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      bill.householdName,
                      style: theme.textTheme.titleMedium?.copyWith(
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Wrap(
                      spacing: 6,
                      runSpacing: 4,
                      children: [
                        AppBadge(
                          label: bill.iuranType,
                          backgroundColor: AppColors.info.withValues(
                            alpha: isDark ? 0.2 : 0.12,
                          ),
                          textColor: AppColors.info,
                        ),
                        AppBadge(
                          label: bill.periode,
                          backgroundColor: isDark
                              ? AppColors.darkAccentSoft
                              : AppColors.lightAccentSoft,
                          textColor: AppColors.accent,
                        ),
                      ],
                    ),
                  ],
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

          Row(
            children: [
              Icon(Icons.account_balance_wallet_outlined,
                  size: 16, color: iconColor),
              const SizedBox(width: 6),
              Expanded(
                child: Text(
                  'Tagihan: ${formatRupiah(bill.nominal)}',
                  style: theme.textTheme.bodyMedium?.copyWith(
                    color: isDark
                        ? AppColors.darkTextMuted
                        : AppColors.lightTextMuted,
                  ),
                ),
              ),
            ],
          ),
          if (bill.paidAmount > 0) ...[
            const SizedBox(height: 4),
            Row(
              children: [
                const SizedBox(width: 22),
                Icon(
                  Icons.check_circle_outline,
                  size: 16,
                  color: AppColors.success,
                ),
                const SizedBox(width: 6),
                Expanded(
                  child: Text(
                    'Dibayar: ${formatRupiah(bill.paidAmount)}',
                    style: theme.textTheme.bodyMedium?.copyWith(
                      color: AppColors.success,
                    ),
                  ),
                ),
              ],
            ),
          ],
          const SizedBox(height: 4),
          Row(
            children: [
              Icon(
                Icons.money_off_csred_outlined,
                size: 16,
                color: AppColors.warning,
              ),
              const SizedBox(width: 6),
              Expanded(
                child: Text(
                  'Sisa: ${formatRupiah(bill.remainingAmount)}',
                  style: theme.textTheme.bodyMedium?.copyWith(
                    color: AppColors.warning,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
            ],
          ),

          Padding(
            padding: const EdgeInsets.only(top: AppSpacing.sm),
            child: InkWell(
              onTap: () =>
                  context.push('/home/iuran/detail/${bill.id}'),
              borderRadius: BorderRadius.circular(AppRadius.xs),
              child: Padding(
                padding: const EdgeInsets.symmetric(
                  vertical: AppSpacing.xxs,
                  horizontal: AppSpacing.sm,
                ),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    const Icon(Icons.visibility_outlined, size: 16),
                    const SizedBox(width: 4),
                    Text(
                      'Bayar Sekarang',
                      style: TextStyle(
                        fontSize: 12,
                        fontWeight: FontWeight.w600,
                        color: AppColors.accent,
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}
