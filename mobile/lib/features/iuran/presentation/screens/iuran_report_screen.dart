import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/widgets/app_badge.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import 'package:social_finance/core/widgets/section_header.dart';
import 'package:social_finance/core/widgets/summary_card.dart';

import '../../data/iuran_models.dart';
import '../../data/iuran_providers.dart';

/// Dashboard/report screen for Iuran feature.
///
/// All financial data comes from real API providers.
class IuranReportScreen extends ConsumerWidget {
  const IuranReportScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    final billsAsync = ref.watch(billsProvider);

    // ─── Loading state ─────────────────────────────────────────────────
    if (billsAsync.isLoading) {
      return Scaffold(
        appBar: AppBar(
          centerTitle: false,
          title: const MenuAppBarTitle(title: 'Laporan Iuran'),
        ),
        body: Center(
          child: const CircularProgressIndicator(),
        ),
      );
    }

    // ─── Error state ───────────────────────────────────────────────────
    if (billsAsync.hasError) {
      final error = billsAsync.error;
      final errorMessage = switch (error) {
        String s => s,
        _ => 'Gagal memuat data laporan. Silakan coba lagi.',
      };
      return Scaffold(
        appBar: AppBar(
          centerTitle: false,
          title: const MenuAppBarTitle(title: 'Laporan Iuran'),
        ),
        body: Center(
          child: Padding(
            padding: const EdgeInsets.all(AppSpacing.lg),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(
                  Icons.error_outline,
                  size: 64,
                  color: AppColors.danger,
                ),
                const SizedBox(height: AppSpacing.md),
                Text(
                  errorMessage,
                  textAlign: TextAlign.center,
                  style: theme.textTheme.bodyLarge,
                ),
                const SizedBox(height: AppSpacing.lg),
                FilledButton.icon(
                  onPressed: () => ref.refresh(billsProvider),
                  icon: const Icon(Icons.refresh),
                  label: const Text('Coba Lagi'),
                ),
              ],
            ),
          ),
        ),
      );
    }

    // ─── Extract data ──────────────────────────────────────────────────
    final bills = switch (billsAsync) {
      AsyncData(:final value) => value,
      _ => <IuranBill>[],
    };

    // ─── Empty state ───────────────────────────────────────────────────
    if (bills.isEmpty) {
      return Scaffold(
        appBar: AppBar(
          centerTitle: false,
          title: const MenuAppBarTitle(title: 'Laporan Iuran'),
        ),
        body: Center(
          child: Padding(
            padding: const EdgeInsets.all(AppSpacing.lg),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(
                  Icons.receipt_long_outlined,
                  size: 64,
                  color: isDark
                      ? AppColors.darkTextMuted
                      : AppColors.lightTextMuted,
                ),
                const SizedBox(height: AppSpacing.md),
                Text(
                  'Belum ada tagihan iuran.',
                  style: theme.textTheme.titleMedium?.copyWith(
                    color: isDark
                        ? AppColors.darkTextMuted
                        : AppColors.lightTextMuted,
                  ),
                  textAlign: TextAlign.center,
                ),
              ],
            ),
          ),
        ),
      );
    }

    // ─── Compute summary ───────────────────────────────────────────────
    final totalTagihan = bills.fold<int>(0, (s, b) => s + b.nominal);
    final totalDibayar = bills.fold<int>(0, (s, b) => s + b.paidAmount);
    final totalTunggakan = bills
        .where((b) => b.status != IuranStatus.lunas)
        .fold<int>(0, (s, b) => s + b.remainingAmount);
    final collectionRate = totalTagihan > 0
        ? (totalDibayar / totalTagihan * 100).clamp(0.0, 100.0)
        : 0.0;

    // ─── Compute breakdown by iuran type ───────────────────────────────
    final typeGroups = <String, List<IuranBill>>{};
    for (final bill in bills) {
      typeGroups.putIfAbsent(bill.iuranType, () => []).add(bill);
    }

    final typeNames = typeGroups.keys.toList();

    // ─── Recent transactions (most recent period first) ────────────────
    final sortedBills = List<IuranBill>.from(bills)
      ..sort((a, b) {
        final aParts = _parsePeriodForSort(a.periode);
        final bParts = _parsePeriodForSort(b.periode);
        // Primary: period descending
        if (aParts[0] != bParts[0] || aParts[1] != bParts[1]) {
          if (bParts[0] != aParts[0]) return bParts[0].compareTo(aParts[0]);
          return bParts[1].compareTo(aParts[1]);
        }
        // Secondary: household name ascending
        return a.householdName.compareTo(b.householdName);
      });

    final recent = sortedBills.take(10).toList();

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
                    value: formatRupiah(totalTagihan),
                    icon: Icons.receipt_long_outlined,
                    accentColor: AppColors.accent,
                  ),
                ),
                SizedBox(
                  width: double.infinity,
                  child: SummaryCard(
                    title: 'Total Dibayar',
                    value: formatRupiah(totalDibayar),
                    icon: Icons.check_circle_outline,
                    accentColor: AppColors.success,
                  ),
                ),
                SizedBox(
                  width: double.infinity,
                  child: SummaryCard(
                    title: 'Total Tunggakan',
                    value: formatRupiah(totalTunggakan),
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
                        '${collectionRate.toStringAsFixed(1)}%',
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
                      value: collectionRate / 100,
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
                        'Dibayar: ${formatRupiah(totalDibayar)}',
                        style: theme.textTheme.bodySmall?.copyWith(
                          color: AppColors.success,
                        ),
                      ),
                      Text(
                        'Belum: ${formatRupiah(totalTunggakan)}',
                        style: theme.textTheme.bodySmall?.copyWith(
                          color: AppColors.warning,
                        ),
                      ),
                    ],
                  ),
                ],
              ),
            ),

            // ── Breakdown by Type ──
            if (typeNames.isNotEmpty) ...[
              const SizedBox(height: AppSpacing.lg),
              const SectionHeader(title: 'Rincian per Jenis Iuran'),
              const SizedBox(height: AppSpacing.sm),
              ...typeNames.map((typeName) {
                final typeBills = typeGroups[typeName] ?? [];
                if (typeBills.isEmpty) return const SizedBox.shrink();

                final typeTotal =
                    typeBills.fold<int>(0, (s, b) => s + b.nominal);
                final typePaid =
                    typeBills.fold<int>(0, (s, b) => s + b.paidAmount);
                final rate = typeTotal > 0
                    ? (typePaid / typeTotal * 100).toStringAsFixed(0)
                    : '0';

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
                                typeName,
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
            ],

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
                        backgroundColor: _statusColorWithBackground(
                          bill.status,
                          isDark,
                        ),
                        child: Icon(
                          bill.status == IuranStatus.lunas
                              ? Icons.check_circle
                              : bill.status == IuranStatus.sebagian
                                  ? Icons.star_border
                                  : Icons.circle_outlined,
                          size: 18,
                          color: _statusColorForIcon(bill.status, isDark),
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
                            textColor: _statusColorForIcon(
                              bill.status,
                              isDark,
                            ),
                            backgroundColor: _statusColorWithBackground(
                              bill.status,
                              isDark,
                            ),
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

  /// Color for status icon background (light tint).
  Color _statusColorWithBackground(IuranStatus status, bool isDark) {
    final baseColor = _statusColorForIcon(status, isDark);
    return baseColor.withValues(alpha: isDark ? 0.2 : 0.12);
  }

  /// Color for status icon and badge text.
  Color _statusColorForIcon(IuranStatus status, bool isDark) {
    return switch (status) {
      IuranStatus.belumBayar => AppColors.warning,
      IuranStatus.sebagian => AppColors.warning,
      IuranStatus.lunas => AppColors.success,
    };
  }

  /// Parse a period string like "Oktober 2026" into [year, month] for sorting.
  static List<int> _parsePeriodForSort(String period) {
    final parts = period.split(' ');
    if (parts.length < 2) return [0, 0];
    final year = int.tryParse(parts[1]) ?? 0;
    const months = {
      'januari': 1,
      'februari': 2,
      'maret': 3,
      'april': 4,
      'mei': 5,
      'juni': 6,
      'juli': 7,
      'agustus': 8,
      'september': 9,
      'oktober': 10,
      'november': 11,
      'desember': 12,
    };
    final month = months[parts[0].toLowerCase()] ?? 0;
    return [year, month];
  }
}
