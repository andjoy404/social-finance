import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/core/providers/theme_provider.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/utils/rupiah_formatter.dart';
import 'package:social_finance/core/widgets/app_badge.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/section_header.dart';
import 'package:social_finance/core/widgets/summary_card.dart';
import '../../data/mock_dashboard_data.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import '../widgets/financial_summary_chart.dart';
import '../widgets/transaction_row.dart';

/// Standard gap between equivalent adjacent dashboard panels.
const _panelGap = AppSpacing.md; // 12

/// Extra vertical gap before a new section title.
const _sectionGap = AppSpacing.lg; // 20

/// Gap between a section title and its first child card.
const _titleGap = AppSpacing.sm; // 8

/// Dashboard screen with a single canonical horizontal grid.
///
/// All full-width panels share the same left/right edges defined by
/// [AppSpacing.screenPadding].  Children never apply their own
/// horizontal offset — only the scroll view controls that.
class DashboardScreen extends ConsumerWidget {
  const DashboardScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final authState = ref.watch(authRepositoryProvider);
    final summary = ref.watch(dashboardSummaryProvider);
    final transactions = ref.watch(recentTransactionsProvider);

    final user = authState?.valueOrNull;
    final userName = user?['name'] ?? 'Pengguna';
    final roleUser = user?['role'] as AppRole? ?? AppRole.warga;
    final rt = user?['rt'] ?? '';
    final rw = user?['rw'] ?? '';

    final isDark = theme.brightness == Brightness.dark;

    return SingleChildScrollView(
      padding: EdgeInsets.fromLTRB(
        AppSpacing.base,
        AppSpacing.base,
        AppSpacing.base,
        DashboardMedia.bottomPadding(context),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // ── Header ──
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text('Selamat datang,', style: theme.textTheme.bodyMedium),
                    Text(userName, style: theme.textTheme.headlineMedium),
                    Text('RT $rt / RW $rw', style: theme.textTheme.bodySmall),
                  ],
                ),
              ),
              const SizedBox(width: 8),
              Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  _ThemeActionButton(
                    onSelected: (mode) {
                      ref.read(themeProvider.notifier).setMode(mode);
                    },
                  ),
                  const SizedBox(width: 4),
                  IconButton(
                    onPressed: () async {
                      ref.read(authRepositoryProvider.notifier).logout();
                    },
                    icon: Icon(
                      Icons.logout_rounded,
                      size: 18,
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                    tooltip: 'Keluar',
                  ),
                ],
              ),
            ],
          ),

          // ── Badge → Saldo Kas ──
          const SizedBox(height: _panelGap),

          // ── Role badge ──
          Align(
            alignment: Alignment.centerLeft,
            child: AppBadge(
              label: appRoleDisplayName(roleUser),
              isNeonStyle: isDark,
            ),
          ),

          // ── Saldo Kas ──
          const SizedBox(height: _panelGap),
          Container(
            width: double.infinity,
            padding: const EdgeInsets.all(24),
            decoration: BoxDecoration(
              color: isDark ? const Color(0xFF262C30) : const Color(0xFFFDFCFF),
              border: Border.all(
                color: isDark
                    ? const Color(0xFF9B8FD6).withValues(alpha: 0.4)
                    : AppColors.seed.withValues(alpha: 0.3),
                width: 1.5,
              ),
              borderRadius: BorderRadius.circular(AppRadius.lg),
            ),
            child: Row(
              children: [
                Container(
                  width: 4,
                  height: 60,
                  decoration: BoxDecoration(
                    color: isDark ? const Color(0xFF9B8FD6) : AppColors.seed,
                    borderRadius: BorderRadius.circular(2),
                  ),
                ),
                const SizedBox(width: 20),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Icon(
                            Icons.account_balance_wallet_rounded,
                            size: 16,
                            color: isDark
                                ? const Color(0xFF9B8FD6)
                                : AppColors.seed,
                          ),
                          const SizedBox(width: 8),
                          Text(
                            'Saldo Kas',
                            style: theme.textTheme.bodyMedium?.copyWith(
                              color: isDark
                                  ? const Color(0xFFB0B8C0)
                                  : const Color(0xFF6B7280),
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 12),
                      Text(
                        RupiahFormatter.format(summary.saldoKas),
                        style: theme.textTheme.headlineLarge?.copyWith(
                          color: isDark
                              ? Colors.white
                              : const Color(0xFF1B1D1E),
                          fontWeight: FontWeight.bold,
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),

          // ── Pemasukan / Pengeluaran ──
          const SizedBox(height: _panelGap),
          Row(
            children: [
              Expanded(
                child: SummaryCard(
                  title: 'Pemasukan Bulan Ini',
                  value: RupiahFormatter.format(summary.pemasukanBulanIni),
                  icon: Icons.arrow_downward,
                  accentColor: AppColors.income,
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: SummaryCard(
                  title: 'Pengeluaran Bulan Ini',
                  value: RupiahFormatter.format(summary.pengeluaranBulanIni),
                  icon: Icons.arrow_upward,
                  accentColor: AppColors.expense,
                ),
              ),
            ],
          ),

          // ── Ringkasan Keuangan ──
          const SizedBox(height: _panelGap),
          FinancialSummaryChart(summary: summary, theme: theme),

          // ── Iuran Bulan Ini ──
          const SizedBox(height: _sectionGap),
          _buildDuesSection(summary.duesProgress, theme),

          // ── Transaksi Terbaru ──
          const SizedBox(height: _sectionGap),
          const SectionHeader(title: 'Transaksi Terbaru'),
          const SizedBox(height: _titleGap),
          ...transactions.map(
            (tx) => TransactionRow(transaction: tx, theme: theme),
          ),
          const SizedBox(height: AppSpacing.base),
        ],
      ),
    );
  }

  Widget _buildDuesSection(DuesProgress dues, ThemeData theme) {
    final isDark = theme.brightness == Brightness.dark;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const SectionHeader(title: 'Iuran Bulan Ini'),
        const SizedBox(height: _titleGap),
        AppCard(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Container(
                      padding: const EdgeInsets.all(10),
                      decoration: BoxDecoration(
                        color: theme.colorScheme.secondary.withValues(
                          alpha: isDark ? 0.2 : 0.12,
                        ),
                        borderRadius: BorderRadius.circular(AppRadius.sm),
                      ),
                      child: Icon(
                        Icons.receipt_long,
                        color: theme.colorScheme.secondary,
                      ),
                    ),
                    const SizedBox(width: 14),
                    Expanded(
                      child: Text(
                        '${dues.paid} dari ${dues.target} KK sudah membayar',
                        style: theme.textTheme.titleMedium,
                      ),
                    ),
                    Text(
                      '${dues.percentage}%',
                      style: theme.textTheme.headlineSmall?.copyWith(
                        color: theme.colorScheme.primary,
                        fontWeight: FontWeight.bold,
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 16),
                LayoutBuilder(
                  builder: (context, constraints) {
                    return SizedBox(
                      height: 8,
                      child: ClipRRect(
                        borderRadius: BorderRadius.circular(AppRadius.xs),
                        child: LinearProgressIndicator(
                          value: dues.ratio,
                          minHeight: 8,
                          backgroundColor: theme.colorScheme.primary.withValues(
                            alpha: isDark ? 0.2 : 0.15,
                          ),
                          valueColor: AlwaysStoppedAnimation(
                            theme.colorScheme.primary,
                          ),
                        ),
                      ),
                    );
                  },
                ),
              ],
            ),
          ),
        ),
      ],
    );
  }
}

/// Media query helper that returns the combined bottom padding from safe area
/// plus a small spacer so content doesn't sit behind bottom navigation.
class DashboardMedia {
  DashboardMedia._();

  static double bottomPadding(BuildContext context) {
    final mediaQuery = MediaQuery.of(context);
    return mediaQuery.padding.bottom + AppSpacing.base;
  }
}

/// Separate theme action button for dashboard header.
/// Opens a popup with three theme mode options.
class _ThemeActionButton extends ConsumerWidget {
  final void Function(AppThemeMode) onSelected;

  const _ThemeActionButton({required this.onSelected});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final currentMode = ref.watch(themeProvider);
    return PopupMenuButton<AppThemeMode>(
      icon: Icon(
        Icons.palette_outlined,
        size: 18,
        color: theme.colorScheme.onSurfaceVariant,
      ),
      tooltip: 'Tampilan',
      onSelected: onSelected,
      itemBuilder: (_) => [
        _popupItem(
          context,
          currentMode,
          icon: Icons.settings_brightness,
          label: 'Ikuti Sistem',
          value: AppThemeMode.system,
        ),
        _popupItem(
          context,
          currentMode,
          icon: Icons.wb_sunny,
          label: 'Terang',
          value: AppThemeMode.light,
        ),
        _popupItem(
          context,
          currentMode,
          icon: Icons.nightlight,
          label: 'Gelap',
          value: AppThemeMode.dark,
        ),
      ],
    );
  }

  PopupMenuItem<AppThemeMode> _popupItem(
    BuildContext context,
    AppThemeMode currentMode, {
    required IconData icon,
    required String label,
    required AppThemeMode value,
  }) {
    final isSelected = currentMode == value;
    return PopupMenuItem<AppThemeMode>(
      value: value,
      child: Row(
        children: [
          Icon(
            isSelected ? Icons.check : icon,
            size: 18,
            color: isSelected ? Theme.of(context).colorScheme.primary : null,
          ),
          const SizedBox(width: 12),
          Text(label, style: const TextStyle(fontSize: 14)),
        ],
      ),
    );
  }
}
