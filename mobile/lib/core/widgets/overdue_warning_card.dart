import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/utils/rupiah_formatter.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/iuran/data/iuran_providers.dart';

/// Global warning card displayed across all resident (`warga`) screens when
/// the resident has at least one obligatory bill overdue by >= 3 calendar months.
class OverdueWarningCard extends ConsumerWidget {
  const OverdueWarningCard({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final user = ref.watch(authRepositoryProvider)?.valueOrNull;
    if (!isPlainWargaUser(user)) {
      return const SizedBox.shrink();
    }

    final summaryAsync = ref.watch(overdueWarningSummaryProvider);
    final summary = summaryAsync.valueOrNull;
    if (summary == null ||
        !summary.hasOverdue3Months ||
        summary.totalOutstanding <= 0 ||
        summary.overdueBillCount <= 0) {
      return const SizedBox.shrink();
    }

    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;
    const amberPrimary = Color(0xFFD97706);
    const amberDarkText = Color(0xFFB45309);
    final cardBg = isDark
        ? AppColors.warning.withValues(alpha: 0.14)
        : const Color(0xFFFFFBEB);
    final borderColor = isDark
        ? AppColors.warning.withValues(alpha: 0.45)
        : const Color(0xFFF59E0B).withValues(alpha: 0.55);

    return Material(
      color: Colors.transparent,
      child: Container(
        key: const Key('global-overdue-warning-card'),
        width: double.infinity,
        margin: const EdgeInsets.fromLTRB(12, 8, 12, 4),
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
        decoration: BoxDecoration(
          color: cardBg,
          borderRadius: BorderRadius.circular(AppRadius.base),
          border: Border.all(color: borderColor, width: 1),
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Container(
                  width: 30,
                  height: 30,
                  decoration: BoxDecoration(
                    color: AppColors.warning.withValues(alpha: 0.20),
                    borderRadius: BorderRadius.circular(AppRadius.sm),
                  ),
                  alignment: Alignment.center,
                  child: const Icon(
                    Icons.warning_amber_rounded,
                    size: 18,
                    color: amberPrimary,
                  ),
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Wrap(
                        spacing: 6,
                        runSpacing: 4,
                        crossAxisAlignment: WrapCrossAlignment.center,
                        children: [
                          Text(
                            'Peringatan Tunggakan Iuran',
                            style: theme.textTheme.titleSmall?.copyWith(
                              fontWeight: FontWeight.w700,
                              fontSize: 13,
                              color: isDark
                                  ? AppColors.darkText
                                  : AppColors.lightText,
                            ),
                          ),
                          Container(
                            padding: const EdgeInsets.symmetric(
                              horizontal: 7,
                              vertical: 2,
                            ),
                            decoration: BoxDecoration(
                              color: amberPrimary.withValues(alpha: 0.16),
                              borderRadius: BorderRadius.circular(999),
                              border: Border.all(
                                color: amberPrimary.withValues(alpha: 0.35),
                              ),
                            ),
                            child: Text(
                              'Tunggakan 3 Bulan+',
                              style: theme.textTheme.labelSmall?.copyWith(
                                fontWeight: FontWeight.w700,
                                fontSize: 10.5,
                                color: isDark
                                    ? AppColors.warning
                                    : amberDarkText,
                              ),
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 4),
                      Text(
                        'Anda memiliki tagihan iuran yang belum lunas selama 3 bulan atau lebih. Mohon segera lakukan pembayaran agar kewajiban iuran tidak semakin menumpuk.',
                        style: theme.textTheme.bodySmall?.copyWith(
                          fontSize: 11.5,
                          height: 1.35,
                          color: isDark
                              ? AppColors.darkTextMuted
                              : AppColors.lightTextSecondary,
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
            const SizedBox(height: 8),
            Wrap(
              spacing: 10,
              runSpacing: 6,
              alignment: WrapAlignment.spaceBetween,
              crossAxisAlignment: WrapCrossAlignment.center,
              children: [
                Wrap(
                  spacing: 6,
                  runSpacing: 2,
                  crossAxisAlignment: WrapCrossAlignment.center,
                  children: [
                    Text(
                      'Total Tunggakan:',
                      style: theme.textTheme.bodySmall?.copyWith(
                        fontSize: 11.5,
                        color: isDark
                            ? AppColors.darkTextMuted
                            : AppColors.lightTextSecondary,
                      ),
                    ),
                    Text(
                      RupiahFormatter.formatIdr(summary.totalOutstanding),
                      style: theme.textTheme.bodySmall?.copyWith(
                        fontSize: 12,
                        fontWeight: FontWeight.w700,
                        color: isDark ? AppColors.warning : amberDarkText,
                      ),
                    ),
                    Text(
                      '• ${summary.overdueBillCount} tagihan',
                      style: theme.textTheme.bodySmall?.copyWith(
                        fontSize: 11.5,
                        fontWeight: FontWeight.w600,
                        color: isDark
                            ? AppColors.darkText
                            : AppColors.lightText,
                      ),
                    ),
                  ],
                ),
                SizedBox(
                  height: 30,
                  child: FilledButton.icon(
                    key: const Key('overdue-warning-action-btn'),
                    style: FilledButton.styleFrom(
                      backgroundColor: amberPrimary,
                      foregroundColor: Colors.white,
                      padding: const EdgeInsets.symmetric(
                        horizontal: 10,
                        vertical: 0,
                      ),
                      visualDensity: VisualDensity.compact,
                      textStyle: const TextStyle(
                        fontSize: 11.5,
                        fontWeight: FontWeight.w600,
                      ),
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(AppRadius.sm),
                      ),
                    ),
                    onPressed: () {
                      context.go('/home/iuran');
                    },
                    icon: const Icon(Icons.arrow_forward, size: 13),
                    label: const Text('Lihat Tagihan Saya'),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
