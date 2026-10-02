import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/widgets/app_badge.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import 'package:social_finance/core/widgets/section_header.dart';

import '../../data/iuran_models.dart';
import '../../data/iuran_mock_data.dart';

/// Detail screen for a single iuran bill.
class IuranDetailScreen extends StatelessWidget {
  final String billId;

  const IuranDetailScreen({super.key, required this.billId});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    // Find the bill from mock data
    final bill = IuranMockData.bills.firstWhere(
      (b) => b.id == billId,
      orElse: () => IuranMockData.bills.first,
    );

    return Scaffold(
      appBar: AppBar(
        centerTitle: false,
        title: const MenuAppBarTitle(title: 'Detail Tagihan'),
        leading: IconButton(
          icon: const Icon(Icons.arrow_back),
          onPressed: () => context.pop(),
        ),
      ),
      floatingActionButton: bill.status != IuranStatus.lunas
          ? _buildAndroidFab(context, AppColors.accentSoftColor(context))
          : null,
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(AppSpacing.base),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // ── Status Header ──
            AppCard(
              useEmphasis: true,
              padding: const EdgeInsets.all(AppSpacing.lg),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Icon(
                        Icons.receipt_long,
                        size: 24,
                        color: AppColors.accent,
                      ),
                      const SizedBox(width: AppSpacing.md),
                      Expanded(
                        child: Text(
                          bill.householdName,
                          style: theme.textTheme.headlineSmall?.copyWith(
                            fontWeight: FontWeight.w600,
                          ),
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
                ],
              ),
            ),

            const SizedBox(height: AppSpacing.lg),

            // ── Bill Details ──
            SectionHeader(title: 'Informasi Tagihan'),
            const SizedBox(height: AppSpacing.sm),

            AppCard(
              padding: const EdgeInsets.all(AppSpacing.base),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  _buildDetailRow(
                    context,
                    icon: Icons.groups_outlined,
                    label: 'Nama Kepala Keluarga',
                    value: bill.householdName,
                  ),
                  const SizedBox(height: AppSpacing.md),
                  _buildDetailRow(
                    context,
                    icon: Icons.home_outlined,
                    label: 'RT',
                    value: 'RT ${bill.rt}',
                  ),
                  const SizedBox(height: AppSpacing.md),
                  _buildDetailRow(
                    context,
                    icon: Icons.category_outlined,
                    label: 'Jenis Iuran',
                    value: bill.iuranType,
                  ),
                  const SizedBox(height: AppSpacing.md),
                  _buildDetailRow(
                    context,
                    icon: Icons.calendar_today_outlined,
                    label: 'Periode',
                    value: bill.periode,
                  ),
                ],
              ),
            ),

            const SizedBox(height: AppSpacing.lg),

            // ── Payment Details ──
            SectionHeader(title: 'Detail Pembayaran'),
            const SizedBox(height: AppSpacing.sm),

            AppCard(
              padding: const EdgeInsets.all(AppSpacing.base),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  _buildPaymentRow(
                    context,
                    icon: Icons.account_balance_wallet_outlined,
                    label: 'Nominal Tagihan',
                    value: formatRupiah(bill.nominal),
                    highlight: true,
                  ),
                  const Divider(height: 24),
                  _buildPaymentRow(
                    context,
                    icon: Icons.check_circle_outline,
                    label: 'Jumlah Dibayar',
                    value: formatRupiah(bill.paidAmount),
                    valueColor: AppColors.success,
                  ),
                  const SizedBox(height: AppSpacing.md),
                  if (bill.isInArrears) ...[
                    _buildPaymentRow(
                      context,
                      icon: Icons.warning_outlined,
                      label: 'Sisa Tagihan',
                      value: formatRupiah(bill.remainingAmount),
                      valueColor: AppColors.warning,
                      highlight: true,
                    ),
                  ] else ...[
                    _buildPaymentRow(
                      context,
                      icon: Icons.celebration_outlined,
                      label: 'Status',
                      value: 'Sudah Lunas',
                      valueColor: AppColors.success,
                    ),
                  ],
                ],
              ),
            ),

            // ── Progress Bar for Partial Payment ──
            if (bill.status == IuranStatus.sebagian) ...[
              const SizedBox(height: AppSpacing.lg),
              SectionHeader(title: 'Progress Pembayaran'),
              const SizedBox(height: AppSpacing.sm),
              AppCard(
                padding: const EdgeInsets.all(AppSpacing.base),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        Text(
                          '${((bill.paidAmount / bill.nominal) * 100).toStringAsFixed(0)}%',
                          style: theme.textTheme.titleMedium?.copyWith(
                            fontWeight: FontWeight.w600,
                            color: AppColors.accent,
                          ),
                        ),
                        Text(
                          '$bill.paidAmount / ${bill.nominal}',
                          style: theme.textTheme.bodySmall?.copyWith(
                            color: AppColors.lightTextMuted,
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(height: AppSpacing.sm),
                    ClipRRect(
                      borderRadius:
                          BorderRadius.circular(AppRadius.xs),
                      child: LinearProgressIndicator(
                        value: bill.paidAmount / bill.nominal,
                        minHeight: 8,
                        borderRadius: BorderRadius.circular(AppRadius.xs),
                        backgroundColor: isDark
                            ? AppColors.darkSurface
                            : AppColors.lightSurfaceSubtle,
                        valueColor: const AlwaysStoppedAnimation<Color>(
                          AppColors.accent,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ],

            const SizedBox(height: AppSpacing.xxl),
          ],
        ),
      ),
    );
  }

  Widget _buildDetailRow(
    BuildContext context, {
    required IconData icon,
    required String label,
    required String value,
  }) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Icon(icon, size: 18, color: AppColors.accent),
        const SizedBox(width: AppSpacing.sm),
        SizedBox(
          width: 120,
          child: Text(
            label,
            style: theme.textTheme.bodySmall?.copyWith(
              color: isDark
                  ? AppColors.darkTextMuted
                  : AppColors.lightTextMuted,
            ),
          ),
        ),
        Expanded(
          child: Text(
            value,
            style: theme.textTheme.bodyMedium?.copyWith(
              fontWeight: FontWeight.w500,
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildPaymentRow(
    BuildContext context, {
    required IconData icon,
    required String label,
    required String value,
    Color? valueColor,
    bool highlight = false,
  }) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Icon(icon, size: 18, color: valueColor ?? AppColors.accent),
        const SizedBox(width: AppSpacing.sm),
        SizedBox(
          width: 120,
          child: Text(
            label,
            style: theme.textTheme.bodySmall?.copyWith(
              color: isDark
                  ? AppColors.darkTextMuted
                  : AppColors.lightTextMuted,
            ),
          ),
        ),
        Expanded(
          child: Text(
            value,
            style: theme.textTheme.bodyMedium?.copyWith(
              fontWeight: highlight ? FontWeight.w700 : FontWeight.w500,
              color: valueColor ??
                  (isDark ? AppColors.darkText : AppColors.lightText),
              fontSize: highlight ? 16 : null,
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildAndroidFab(BuildContext context, Color accentSoft) {
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
          onTap: () => context.push('/home/iuran/pembayaran/${billId}'),
          borderRadius: BorderRadius.circular(24),
          child: const Padding(
            padding: EdgeInsets.symmetric(horizontal: 16, vertical: 8),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(Icons.payment, size: 20, color: AppColors.accent),
                SizedBox(width: 6),
                Text(
                  'Bayar',
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
}
