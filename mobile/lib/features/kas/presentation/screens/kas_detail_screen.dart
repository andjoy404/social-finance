import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/utils/rupiah_formatter.dart';
import 'package:social_finance/core/widgets/app_badge.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/features/kas/data/kas_models.dart';

/// Detail screen for a single KAS transaction.
class KasDetailScreen extends StatelessWidget {
  final KasTransaction transaction;

  const KasDetailScreen({
    super.key,
    required this.transaction,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;
    final isIncoming = transaction.jenis == KasJenis.masuk;
    final accentColor = isIncoming ? AppColors.success : AppColors.danger;
    final icon = isIncoming ? Icons.arrow_downward : Icons.arrow_upward;

    return Scaffold(
      appBar: AppBar(
        centerTitle: false,
        title: const Text('Detail Transaksi'),
        leading: IconButton(
          icon: const Icon(Icons.arrow_back),
          onPressed: () => context.pop(),
        ),
      ),
      body: ListView(
        padding: const EdgeInsets.all(AppSpacing.base),
        children: [
          // Header with amount
          Container(
            width: double.infinity,
            padding: const EdgeInsets.all(20),
            decoration: BoxDecoration(
              color: accentColor.withValues(alpha: isDark ? 0.15 : 0.08),
              borderRadius: BorderRadius.circular(AppRadius.lg),
              border: Border.all(
                color: accentColor.withValues(alpha: isDark ? 0.3 : 0.2),
                width: 1,
              ),
            ),
            child: Row(
              children: [
                Container(
                  padding: const EdgeInsets.all(12),
                  decoration: BoxDecoration(
                    color: accentColor.withValues(alpha: isDark ? 0.25 : 0.15),
                    borderRadius: BorderRadius.circular(AppRadius.sm),
                  ),
                  child: Icon(icon, size: 24, color: accentColor),
                ),
                const SizedBox(width: 16),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        isIncoming ? 'Pemasukan' : 'Pengeluaran',
                        style: theme.textTheme.bodyMedium?.copyWith(
                          color: accentColor,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        RupiahFormatter.format(transaction.nominal),
                        style: theme.textTheme.headlineSmall?.copyWith(
                          color: accentColor,
                          fontWeight: FontWeight.bold,
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),

          const SizedBox(height: AppSpacing.lg),

          // Detail rows
          AppCard(
            padding: const EdgeInsets.all(AppSpacing.base),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                _DetailRow(
                  label: 'Tanggal',
                  value: formatTanggal(transaction.tanggal),
                  isDark: isDark,
                  theme: theme,
                ),
                const Divider(height: 24),
                _DetailRow(
                  label: 'Jenis',
                  value: transaction.jenis.value.isEmpty
                      ? (isIncoming ? 'Masuk' : 'Keluar')
                      : (transaction.jenis.value == 'masuk' ? 'Masuk' : 'Keluar'),
                  isDark: isDark,
                  theme: theme,
                  badge: true,
                  badgeColor: accentColor,
                ),
                const Divider(height: 24),
                _DetailRow(
                  label: 'Kategori',
                  value: transaction.kategori.label,
                  isDark: isDark,
                  theme: theme,
                ),
                const Divider(height: 24),
                _DetailRow(
                  label: 'Keterangan',
                  value: transaction.keterangan,
                  isDark: isDark,
                  theme: theme,
                ),
                if (transaction.referensi != null) ...[
                  const Divider(height: 24),
                  _DetailRow(
                    label: 'Referensi',
                    value: transaction.referensi!,
                    isDark: isDark,
                    theme: theme,
                  ),
                ],
                const Divider(height: 24),
                _DetailRow(
                  label: 'Saldo Berjalan',
                  value: RupiahFormatter.format(transaction.saldo),
                  isDark: isDark,
                  theme: theme,
                  valueColor: AppColors.accent,
                ),
              ],
            ),
          ),

          const SizedBox(height: AppSpacing.xxl),
        ],
      ),
    );
  }
}

class _DetailRow extends StatelessWidget {
  final String label;
  final String value;
  final bool isDark;
  final ThemeData theme;
  final bool badge;
  final Color? badgeColor;
  final Color? valueColor;

  const _DetailRow({
    required this.label,
    required this.value,
    required this.isDark,
    required this.theme,
    this.badge = false,
    this.badgeColor,
    this.valueColor,
  });

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          width: 100,
          child: Text(
            label,
            style: theme.textTheme.bodySmall?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
              fontWeight: FontWeight.w500,
            ),
          ),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: badge && badgeColor != null
              ? AppBadge(
                  label: value,
                  backgroundColor: badgeColor!.withValues(alpha: 0.12),
                  textColor: badgeColor,
                )
              : Text(
                  value,
                  style: theme.textTheme.bodyMedium?.copyWith(
                    color: valueColor,
                  ),
                ),
        ),
      ],
    );
  }
}
