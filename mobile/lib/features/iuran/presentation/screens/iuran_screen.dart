import 'package:flutter/foundation.dart';
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
import '../../data/iuran_mock_data.dart';

/// Main Iuran list screen (WargaScreen equivalent).
class IuranScreen extends ConsumerStatefulWidget {
  const IuranScreen({super.key});

  @override
  ConsumerState<IuranScreen> createState() => _IuranScreenState();
}

class _IuranScreenState extends ConsumerState<IuranScreen> {
  late final TextEditingController _searchController;
  String _selectedPeriode = '';
  String _selectedStatus = '';

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

  void _clearFilters() {
    setState(() {
      _selectedPeriode = '';
      _selectedStatus = '';
    });
  }

  List<IuranBill> _getFilteredBills() {
    final bills = IuranMockData.bills;
    final query = _searchController.text.trim().toLowerCase();

    return bills.where((bill) {
      // Search filter
      final searchMatch = query.isEmpty ||
          bill.householdName.toLowerCase().contains(query) ||
          bill.iuranType.toLowerCase().contains(query) ||
          bill.rt.contains(query);

      // Periode filter
      final periodeMatch =
          _selectedPeriode.isEmpty || bill.periode == _selectedPeriode;

      // Status filter
      final statusMatch =
          _selectedStatus.isEmpty || bill.status.name == _selectedStatus;

      return searchMatch && periodeMatch && statusMatch;
    }).toList();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;
    final filtered = _getFilteredBills();
    final accentSoft = AppColors.accentSoftColor(context);

    return Scaffold(
      appBar: AppBar(
        centerTitle: false,
        title: const MenuAppBarTitle(title: 'Iuran'),
      ),
      floatingActionButton: kIsWeb
          ? _buildWebFab(accentSoft)
          : _buildAndroidFab(accentSoft),
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
              hintText: 'Cari nama, jenis iuran, atau RT...',
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

          // ── Filter Chips ──
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.base),
            child: Row(
              children: [
                // Periode chip
                _buildFilterChip(
                  label: _selectedPeriode.isEmpty
                      ? 'Periode'
                      : _selectedPeriode,
                  selected: _selectedPeriode.isNotEmpty,
                  onTap: () => _showPeriodeDialog(),
                ),
                const SizedBox(width: AppSpacing.sm),
                // Status chip
                _buildFilterChip(
                  label: _selectedStatus.isEmpty
                      ? 'Status'
                      : getStatusLabel(
                          parseStatus(_selectedStatus),
                        ),
                  selected: _selectedStatus.isNotEmpty,
                  onTap: () => _showStatusDialog(),
                ),
                if (_selectedPeriode.isNotEmpty ||
                    _selectedStatus.isNotEmpty) ...[
                  const SizedBox(width: AppSpacing.sm),
                  GestureDetector(
                    onTap: _clearFilters,
                    child: Icon(
                      Icons.close,
                      size: 16,
                      color: isDark
                          ? AppColors.darkTextMuted
                          : AppColors.lightTextMuted,
                    ),
                  ),
                ],
              ],
            ),
          ),

          // ── Summary Count ──
          Padding(
            padding: const EdgeInsets.symmetric(
              horizontal: AppSpacing.base,
              vertical: AppSpacing.xs,
            ),
            child: Text(
              'Menampilkan ${filtered.length} tagihan',
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
                fontWeight: FontWeight.w600,
              ),
            ),
          ),

          // ── Content: Empty / List ──
          Expanded(
            child: filtered.isEmpty
                ? Center(
                    child: SingleChildScrollView(
                      padding: const EdgeInsets.all(AppSpacing.lg),
                      child: _searchController.text.isNotEmpty ||
                              _selectedPeriode.isNotEmpty ||
                              _selectedStatus.isNotEmpty
                          ? EmptyState(
                              icon: Icons.receipt_long_outlined,
                              title: 'Tagihan Tidak Ditemukan',
                              description:
                                  'Tidak ada tagihan yang sesuai dengan filter saat ini.',
                              actionText: 'Reset Filter',
                              onAction: _clearFilters,
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

  Widget _buildAndroidFab(Color accentSoft) {
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
                SizedBox(width: 6),
                Text(
                  'Buat Tagihan',
                  style: TextStyle(
                    fontSize: 11,
                    fontWeight: FontWeight.w600,
                    color: AppColors.accent,
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildWebFab(Color accentSoft) {
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
                SizedBox(width: 6),
                Text(
                  'Buat Tagihan',
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                    color: AppColors.accent,
                  ),
                ),
              ],
            ),
          ),
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
                // "Semua" option
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

  void _showStatusDialog() {
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
                    groupValue: _selectedStatus,
                    onChanged: (value) {
                      setState(() => _selectedStatus = value!);
                      Navigator.of(dialogCtx).pop();
                    },
                  ),
                  onTap: () {
                    setState(() => _selectedStatus = '');
                    Navigator.of(dialogCtx).pop();
                  },
                ),
                const Divider(),
                for (final status in IuranStatus.values)
                  ListTile(
                    title: Text(getStatusLabel(status)),
                    leading: Radio<String>(
                      value: status.name,
                      groupValue: _selectedStatus,
                      onChanged: (value) {
                        setState(() => _selectedStatus = value!);
                        Navigator.of(dialogCtx).pop();
                      },
                    ),
                    onTap: () {
                      setState(() => _selectedStatus = status.name);
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
}

/// Card item for iuran list, following _ResidentCard pattern.
class _IuranCard extends StatelessWidget {
  final IuranBill bill;

  const _IuranCard({required this.bill});

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

              // Household name + details
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

              // Status badge
              AppBadge(
                label: getStatusLabel(bill.status),
                textColor: getStatusColor(bill.status, context),
                backgroundColor: getStatusColor(bill.status, context)
                    .withValues(alpha: isDark ? 0.18 : 0.10),
              ),
            ],
          ),

          const Divider(height: 20),

          // Amount details
          Row(
            children: [
              Icon(Icons.account_balance_wallet_outlined,
                  size: 16, color: iconColor),
              const SizedBox(width: 6),
              Expanded(
                child: Text(
                  'Nominal: ${formatRupiah(bill.nominal)}',
                  style: theme.textTheme.bodyMedium,
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
          if (bill.isInArrears) ...[
            const SizedBox(height: 4),
            Row(
              children: [
                const SizedBox(width: 22),
                Icon(
                  Icons.warning_outlined,
                  size: 16,
                  color: AppColors.warning,
                ),
                const SizedBox(width: 6),
                Expanded(
                  child: Text(
                    'Sisa: ${formatRupiah(bill.remainingAmount)}',
                    style: theme.textTheme.bodyMedium?.copyWith(
                      color: AppColors.warning,
                    ),
                  ),
                ),
              ],
            ),
          ],

          // Tap area for detail
          Padding(
            padding: const EdgeInsets.only(top: AppSpacing.sm),
            child: InkWell(
              onTap: () => context.push('/home/iuran/detail/${bill.id}'),
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
                      'Lihat Detail',
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
