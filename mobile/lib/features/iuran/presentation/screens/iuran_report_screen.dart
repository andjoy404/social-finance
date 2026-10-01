import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/widgets/app_badge.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import 'package:social_finance/core/widgets/section_header.dart';
import 'package:social_finance/core/widgets/summary_card.dart';

import '../../data/iuran_models.dart';
import '../../data/iuran_mock_data.dart';

/// Dashboard/report screen for Iuran feature.
class IuranReportScreen extends StatelessWidget {
  const IuranReportScreen({super.key});

  /// Calculate summary values from all mock bills.
  Map<String, int> _calculateSummary() {
    final bills = IuranMockData.bills;
    int totalTagihan = 0;
    int totalDibayar = 0;
    int totalTunggakan = 0;

    for (final bill in bills) {
      totalTagihan += bill.nominal;
      totalDibayar += bill.paidAmount;
      if (bill.isInArrears) {
        totalTunggakan += bill.remainingAmount;
      }
    }

    return {
      'totalTagihan': totalTagihan,
      'totalDibayar': totalDibayar,
      'totalTunggakan': totalTunggakan,
    };
  }

  /// Get bills sorted by payment date (most recent first, mock order).
  List<IuranBill> _getRecentTransactions() {
    final bills = IuranMockData.bills;
    // Reverse to show most recent first (mock data is chronological)
    final sorted = List<IuranBill>.from(bills.reversed);
    // Limit to 10
    return sorted.take(10).toList();
  }

  /// Color for a status icon in the report.
  Color _reportStatusColor(IuranStatus status, BuildContext context) {
    return getStatusColor(status, context);
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;
    final summary = _calculateSummary();
    final recent = _getRecentTransactions();

    return Scaffold(
      appBar: AppBar(
        centerTitle: false,
        title: const MenuAppBarTitle(title: 'Laporan Iuran'),
      ),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(AppSpacing.base),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // ── Summary Cards ──
            const SectionHeader(title: 'Ringkasan'),
            const SizedBox(height: AppSpacing.sm),
            Wrap(
              spacing: AppSpacing.sm,
              runSpacing: AppSpacing.sm,
              children: [
                SizedBox(
                  width: double.infinity,
                  child: SummaryCard(
                    title: 'Total Tagihan',
                    value: formatRupiah(summary['totalTagihan']!),
                    icon: Icons.receipt_long_outlined,
                    accentColor: AppColors.accent,
                  ),
                ),
                SizedBox(
                  width: double.infinity,
                  child: SummaryCard(
                    title: 'Total Dibayar',
                    value: formatRupiah(summary['totalDibayar']!),
                    icon: Icons.check_circle_outline,
                    accentColor: AppColors.success,
                  ),
                ),
                SizedBox(
                  width: double.infinity,
                  child: SummaryCard(
                    title: 'Total Tunggakan',
                    value: formatRupiah(summary['totalTunggakan']!),
                    icon: Icons.warning_amber_rounded,
                    accentColor: AppColors.warning,
                  ),
                ),
              ],
            ),

            const SizedBox(height: AppSpacing.lg),

            // ── Collection Rate ──
            const SectionHeader(title: 'Persentase Penagihan'),
            const SizedBox(height: AppSpacing.sm),
            AppCard(
              useEmphasis: true,
              padding: const EdgeInsets.all(AppSpacing.lg),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Text(
                        'Tingkat Koleksi',
                        style: theme.textTheme.titleMedium?.copyWith(
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                      Text(
                        '${((summary['totalDibayar']! / summary['totalTagihan']!) * 100).toStringAsFixed(1)}%',
                        style: theme.textTheme.headlineSmall?.copyWith(
                          color: AppColors.accent,
                          fontWeight: FontWeight.bold,
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: AppSpacing.md),
                  ClipRRect(
                    borderRadius: BorderRadius.circular(AppRadius.xs),
                    child: LinearProgressIndicator(
                      value: summary['totalDibayar']! /
                          summary['totalTagihan']!,
                      minHeight: 12,
                      borderRadius: BorderRadius.circular(AppRadius.xs),
                      backgroundColor: isDark
                          ? AppColors.darkSurface
                          : AppColors.lightSurfaceSubtle,
                      valueColor: const AlwaysStoppedAnimation<Color>(
                        AppColors.accent,
                      ),
                    ),
                  ),
                  const SizedBox(height: AppSpacing.sm),
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Text(
                        'Dibayar: ${formatRupiah(summary['totalDibayar']!)}',
                        style: theme.textTheme.bodySmall?.copyWith(
                          color: AppColors.success,
                        ),
                      ),
                      Text(
                        'Belum: ${formatRupiah(summary['totalTunggakan']!)}',
                        style: theme.textTheme.bodySmall?.copyWith(
                          color: AppColors.warning,
                        ),
                      ),
                    ],
                  ),
                ],
              ),
            ),

            const SizedBox(height: AppSpacing.lg),

            // ── Breakdown by Type ──
            const SectionHeader(title: 'Rincian per Jenis Iuran'),
            const SizedBox(height: AppSpacing.sm),
            ...IuranMockData.iuranTypes.map((type) {
              final typeBills = IuranMockData.bills
                  .where((b) => b.iuranType == type)
                  .toList();
              final typeTotal =
                  typeBills.fold(0, (s, b) => s + b.nominal);
              final typePaid =
                  typeBills.fold(0, (s, b) => s + b.paidAmount);
              final rate = (typePaid / typeTotal * 100).toStringAsFixed(0);

              return Padding(
                padding: const EdgeInsets.only(bottom: AppSpacing.sm),
                child: AppCard(
                  padding: const EdgeInsets.all(AppSpacing.base),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Icon(
                            Icons.receipt_long_outlined,
                            size: 18,
                            color: AppColors.accent,
                          ),
                          const SizedBox(width: AppSpacing.sm),
                          Expanded(
                            child: Text(
                              type,
                              style: theme.textTheme.titleMedium?.copyWith(
                                fontWeight: FontWeight.w600,
                              ),
                            ),
                          ),
                          Text(
                            '$rate%',
                            style: theme.textTheme.titleSmall?.copyWith(
                              color: AppColors.accent,
                              fontWeight: FontWeight.bold,
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: AppSpacing.sm),
                      Row(
                        mainAxisAlignment: MainAxisAlignment.spaceBetween,
                        children: [
                          Text(
                            'Tagihan: ${formatRupiah(typeTotal)}',
                            style: theme.textTheme.bodySmall?.copyWith(
                              color: isDark
                                  ? AppColors.darkTextMuted
                                  : AppColors.lightTextMuted,
                            ),
                          ),
                          Text(
                            'Dibayar: ${formatRupiah(typePaid)}',
                            style: theme.textTheme.bodySmall?.copyWith(
                              color: AppColors.success,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                        ],
                      ),
                    ],
                  ),
                ),
              );
            }),

            const SizedBox(height: AppSpacing.lg),

            // ── Recent Transactions ──
            const SectionHeader(title: 'Transaksi Terakhir'),
            const SizedBox(height: AppSpacing.sm),
            ...recent.map((bill) {
              return Padding(
                padding: const EdgeInsets.only(bottom: AppSpacing.sm),
                child: AppCard(
                  padding: const EdgeInsets.all(AppSpacing.base),
                  child: Row(
                    children: [
                      CircleAvatar(
                        radius: 18,
                        backgroundColor: _reportStatusColor(
                          bill.status,
                          context,
                        ).withValues(alpha: isDark ? 0.2 : 0.12),
                        child: Icon(
                          bill.status == IuranStatus.lunas
                              ? Icons.check_circle
                              : bill.status == IuranStatus.sebagian
                                  ? Icons.partially_checked
                                  : Icons.warning,
                          size: 18,
                          color: _reportStatusColor(
                            bill.status,
                            context,
                          ),
                        ),
                      ),
                      const SizedBox(width: AppSpacing.md),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              bill.householdName,
                              style: theme.textTheme.titleSmall?.copyWith(
                                fontWeight: FontWeight.w600,
                              ),
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                            ),
                            const SizedBox(height: 2),
                            Text(
                              '${bill.iuranType} — ${bill.periode}',
                              style: theme.textTheme.bodySmall?.copyWith(
                                color: isDark
                                    ? AppColors.darkTextMuted
                                    : AppColors.lightTextMuted,
                              ),
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                            ),
                          ],
                        ),
                      ),
                      Column(
                        crossAxisAlignment: CrossAxisAlignment.end,
                        children: [
                          Text(
                            formatRupiah(bill.paidAmount > 0
                                ? bill.paidAmount
                                : bill.nominal),
                            style: theme.textTheme.bodyMedium?.copyWith(
                              fontWeight: FontWeight.w600,
                              color: bill.status == IuranStatus.lunas
                                  ? AppColors.success
                                  : AppColors.warning,
                            ),
                          ),
                          const SizedBox(height: 2),
                          AppBadge(
                            label: getStatusLabel(bill.status),
                            textColor: getStatusColor(
                              bill.status,
                              context,
                            ),
                            backgroundColor: getStatusColor(
                              bill.status,
                              context,
                            ).withValues(alpha: isDark ? 0.18 : 0.10),
                          ),
                        ],
                      ),
                    ],
                  ),
                ),
              );
            }),

            const SizedBox(height: AppSpacing.xxl),
          ],
        ),
      ),
    );
  }
}
