import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/utils/rupiah_formatter.dart';
import 'package:social_finance/core/widgets/app_badge.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import 'package:social_finance/core/widgets/section_header.dart';
import 'package:social_finance/core/widgets/summary_card.dart';
import 'package:social_finance/features/kas/data/kas_models.dart';
import 'package:social_finance/features/kas/data/kas_mock_data.dart';

/// Dashboard/report screen for KAS feature.
class KasReportScreen extends StatefulWidget {
  const KasReportScreen({super.key});

  @override
  State<KasReportScreen> createState() => _KasReportScreenState();
}

class _KasReportScreenState extends State<KasReportScreen> {
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _loadData();
  }

  Future<void> _loadData() async {
    // Simulate data loading
    await Future.delayed(const Duration(milliseconds: 300));
    if (mounted) setState(() => _loading = false);
  }

  /// Calculate summary values from all mock transactions.
  Map<String, dynamic> _calculateSummary() {
    final transactions = KasMockData.getTransactions();
    int totalMasuk = 0;
    int totalKeluar = 0;

    for (final tx in transactions) {
      if (tx.jenis == KasJenis.masuk) {
        totalMasuk += tx.nominal;
      } else {
        totalKeluar += tx.nominal;
      }
    }

    // Running saldo = first (most recent) transaction's saldo, or calculate
    final runningSaldo = transactions.isNotEmpty ? transactions.first.saldo : 0;

    return {
      'runningSaldo': runningSaldo,
      'totalMasuk': totalMasuk,
      'totalKeluar': totalKeluar,
    };
  }

  /// Calculate per-category breakdown.
  List<Map<String, dynamic>> _getCategoryBreakdown() {
    final transactions = KasMockData.getTransactions();
    final categoryMap = <String, Map<String, int>>{};

    for (final tx in transactions) {
      final label = tx.kategori.label;
      if (!categoryMap.containsKey(label)) {
        categoryMap[label] = {'pemasukan': 0, 'pengeluaran': 0, 'saldo': 0};
      }
      if (tx.jenis == KasJenis.masuk) {
        categoryMap[label]!['pemasukan'] =
            (categoryMap[label]!['pemasukan'] ?? 0) + tx.nominal;
      } else {
        categoryMap[label]!['pengeluaran'] =
            (categoryMap[label]!['pengeluaran'] ?? 0) + tx.nominal;
      }
    }

    return categoryMap.entries
        .map((e) => {
              'kategori': e.key,
              'pemasukan': e.value['pemasukan'] ?? 0,
              'pengeluaran': e.value['pengeluaran'] ?? 0,
              'saldo': (e.value['pemasukan'] ?? 0) -
                  (e.value['pengeluaran'] ?? 0),
            })
        .toList()
      ..sort((a, b) => (b['pemasukan'] as int) - (a['pemasukan'] as int));
  }

  /// Get recent transactions (last 5, most recent first).
  List<KasTransaction> _getRecentTransactions() {
    final transactions = KasMockData.getTransactions();
    return transactions.take(5).toList();
  }

  Color _jenisTextColor(KasJenis jenis) {
    return jenis == KasJenis.masuk ? AppColors.success : AppColors.danger;
  }

  Color _jenisBgColor(KasJenis jenis, Brightness brightness) {
    final isIncoming = jenis == KasJenis.masuk;
    return isIncoming
        ? (brightness == Brightness.dark
            ? AppColors.darkSuccessSoft
            : AppColors.lightSuccessSoft)
        : (brightness == Brightness.dark
            ? AppColors.darkDangerSoft
            : AppColors.lightDangerSoft);
  }

  IconData _jenisIcon(KasJenis jenis) {
    return jenis == KasJenis.masuk ? Icons.arrow_downward : Icons.arrow_upward;
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    if (_loading) {
      return Scaffold(
        appBar: AppBar(
          centerTitle: false,
          title: const MenuAppBarTitle(title: 'Laporan Kas'),
        ),
        body: const Center(child: CircularProgressIndicator()),
      );
    }

    final summary = _calculateSummary();
    final breakdown = _getCategoryBreakdown();
    final recent = _getRecentTransactions();

    return Scaffold(
      appBar: AppBar(
        centerTitle: false,
        title: const MenuAppBarTitle(title: 'Laporan Kas'),
      ),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(AppSpacing.base),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // ── Summary Cards ──
            const SectionHeader(title: 'Ringkasan'),
            const SizedBox(height: AppSpacing.sm),
            Column(
              children: [
                // Running saldo — highlighted card
                SizedBox(
                  width: double.infinity,
                  child: SummaryCard(
                    title: 'Saldo Berjalan',
                    value: RupiahFormatter.format(
                        summary['runningSaldo'] as int),
                    icon: Icons.account_balance_wallet,
                    accentColor: AppColors.accent,
                    valueStyle: theme.textTheme.titleLarge?.copyWith(
                      color: AppColors.accent,
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                ),
                const SizedBox(height: AppSpacing.sm),
                Row(
                  children: [
                    Expanded(
                      child: SummaryCard(
                        title: 'Total Kas Masuk',
                        value: RupiahFormatter.format(
                            summary['totalMasuk'] as int),
                        icon: Icons.arrow_downward,
                        accentColor: AppColors.success,
                      ),
                    ),
                    const SizedBox(width: AppSpacing.sm),
                    Expanded(
                      child: SummaryCard(
                        title: 'Total Kas Keluar',
                        value: RupiahFormatter.format(
                            summary['totalKeluar'] as int),
                        icon: Icons.arrow_upward,
                        accentColor: AppColors.danger,
                      ),
                    ),
                  ],
                ),
              ],
            ),

            const SizedBox(height: AppSpacing.lg),

            // ── Rincian Per Kategori ──
            const SectionHeader(title: 'Rincian Per Kategori'),
            const SizedBox(height: AppSpacing.sm),
            ...breakdown.map((cat) {
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
                            Icons.category_outlined,
                            size: 18,
                            color: AppColors.accent,
                          ),
                          const SizedBox(width: AppSpacing.sm),
                          Expanded(
                            child: Text(
                              cat['kategori'] as String,
                              style: theme.textTheme.titleMedium?.copyWith(
                                fontWeight: FontWeight.w600,
                              ),
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: AppSpacing.sm),
                      Row(
                        mainAxisAlignment: MainAxisAlignment.spaceBetween,
                        children: [
                          Row(
                            children: [
                              Icon(
                                Icons.arrow_downward,
                                size: 14,
                                color: AppColors.success,
                              ),
                              const SizedBox(width: 4),
                              Text(
                                'Pemasukan: ${RupiahFormatter.format(cat['pemasukan'] as int)}',
                                style: theme.textTheme.bodySmall?.copyWith(
                                  color: AppColors.success,
                                ),
                              ),
                            ],
                          ),
                          Row(
                            children: [
                              Icon(
                                Icons.arrow_upward,
                                size: 14,
                                color: AppColors.danger,
                              ),
                              const SizedBox(width: 4),
                              Text(
                                'Pengeluaran: ${RupiahFormatter.format(cat['pengeluaran'] as int)}',
                                style: theme.textTheme.bodySmall?.copyWith(
                                  color: AppColors.danger,
                                ),
                              ),
                            ],
                          ),
                        ],
                      ),
                      const SizedBox(height: AppSpacing.xs),
                      Divider(height: 1),
                      const SizedBox(height: 4),
                      Row(
                        mainAxisAlignment: MainAxisAlignment.spaceBetween,
                        children: [
                          Text(
                            'Saldo',
                            style: theme.textTheme.bodySmall?.copyWith(
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                          Text(
                            RupiahFormatter.format(cat['saldo'] as int),
                            style: theme.textTheme.bodySmall?.copyWith(
                              color: AppColors.accent,
                              fontWeight: FontWeight.bold,
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

            // ── Transaksi Terakhir ──
            const SectionHeader(title: 'Transaksi Terakhir'),
            const SizedBox(height: AppSpacing.sm),
            if (recent.isEmpty)
              const Center(
                child: Padding(
                  padding: EdgeInsets.all(AppSpacing.lg),
                  child: Text(
                    'Belum ada transaksi KAS.',
                    style: TextStyle(color: Colors.grey),
                  ),
                ),
              )
            else
              ...recent.map((tx) {
                final isIncoming = tx.jenis == KasJenis.masuk;
                final accentColor = isIncoming
                    ? AppColors.success
                    : AppColors.danger;
                final prefix = isIncoming ? '+' : '-';

                return Padding(
                  padding: const EdgeInsets.only(bottom: AppSpacing.sm),
                  child: AppCard(
                    padding: const EdgeInsets.all(AppSpacing.base),
                    child: Row(
                      children: [
                        // Icon circle
                        Container(
                          padding: const EdgeInsets.all(10),
                          decoration: BoxDecoration(
                            color: accentColor.withValues(
                                alpha: isDark ? 0.18 : 0.10),
                            borderRadius: BorderRadius.circular(AppRadius.sm),
                          ),
                          child: Icon(
                            _jenisIcon(tx.jenis),
                            size: 18,
                            color: accentColor,
                          ),
                        ),
                        const SizedBox(width: AppSpacing.md),

                        // Description
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                tx.keterangan,
                                style: theme.textTheme.bodyMedium,
                                maxLines: 2,
                                overflow: TextOverflow.ellipsis,
                              ),
                              const SizedBox(height: 4),
                              Row(
                                children: [
                                  AppBadge(
                                    label: tx.kategori.label,
                                    backgroundColor: accentColor
                                        .withValues(alpha: 0.12),
                                    textColor: accentColor,
                                  ),
                                  const SizedBox(width: 8),
                                  AppBadge(
                                    label: formatTanggal(tx.tanggal),
                                  ),
                                ],
                              ),
                            ],
                          ),
                        ),

                        // Amount
                        Text(
                          '$prefix${RupiahFormatter.format(tx.nominal)}',
                          style: theme.textTheme.titleSmall?.copyWith(
                            color: accentColor,
                            fontWeight: FontWeight.w600,
                            fontSize: 13,
                          ),
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
