/// KAS Report screen — connected to real backend API.
///
/// Fetches balance from GET /api/v1/reports/balance and transactions
/// from GET /api/v1/transactions, using kasRepositoryProvider.
/// No mock data.

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/utils/rupiah_formatter.dart';
import 'package:social_finance/core/widgets/app_badge.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import 'package:social_finance/core/widgets/section_header.dart';
import 'package:social_finance/core/widgets/summary_card.dart';
import 'package:social_finance/features/kas/data/kas_api_models.dart';
import 'package:social_finance/features/kas/data/kas_models.dart';
import 'package:social_finance/features/kas/data/kas_repository.dart';

/// Dashboard/report screen for KAS feature.
class KasReportScreen extends ConsumerStatefulWidget {
  const KasReportScreen({super.key});

  @override
  ConsumerState<KasReportScreen> createState() => _KasReportScreenState();
}

class _KasReportScreenState extends ConsumerState<KasReportScreen> {
  bool _loading = true;
  String? _error;
  BackendBalance? _balance;
  List<KasTransaction> _transactions = [];
  List<Map<String, dynamic>> _breakdown = [];

  @override
  void initState() {
    super.initState();
    _loadData();
  }

  Future<void> _loadData() async {
    if (!mounted) return;
    setState(() {
      _loading = true;
      _error = null;
    });

    final repository = ref.read(kasRepositoryProvider);

    try {
      final futureTransactions = repository.fetchAllTransactions();
      final futureBalance = repository.fetchBalance();

      final results = await Future.wait([futureTransactions, futureBalance]);

      if (!mounted) return;

      final transactions = results[0] as List<KasTransaction>;
      final balance = results[1] as BackendBalance;

      final breakdown = _calculateBreakdown(transactions);

      setState(() {
        _loading = false;
        _error = null;
        _transactions = transactions;
        _balance = balance;
        _breakdown = breakdown;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = repository.mapDioError(e).message;
      });
    }
  }

  /// Calculate per-category breakdown from transactions.
  List<Map<String, dynamic>> _calculateBreakdown(
    List<KasTransaction> transactions,
  ) {
    final categoryMap = <String, Map<String, int>>{};

    for (final tx in transactions) {
      final label = tx.kategori.label;
      if (!categoryMap.containsKey(label)) {
        categoryMap[label] = {'pemasukan': 0, 'pengeluaran': 0};
      }
      final amount = int.tryParse(tx.nominal) ?? 0;
      if (tx.jenis == KasJenis.masuk) {
        categoryMap[label]!['pemasukan'] =
            (categoryMap[label]!['pemasukan'] ?? 0) + amount;
      } else {
        categoryMap[label]!['pengeluaran'] =
            (categoryMap[label]!['pengeluaran'] ?? 0) + amount;
      }
    }

    return categoryMap.entries
        .map(
          (e) => {
            'kategori': e.key,
            'pemasukan': e.value['pemasukan'] ?? 0,
            'pengeluaran': e.value['pengeluaran'] ?? 0,
            'saldo':
                (e.value['pemasukan'] ?? 0) - (e.value['pengeluaran'] ?? 0),
          },
        )
        .toList()
      ..sort((a, b) => (b['pemasukan'] as int) - (a['pemasukan'] as int));
  }

  /// Get recent transactions (last 5, most recent first).
  List<KasTransaction> _getRecentTransactions() {
    // Transactions from backend are already sorted by created_at DESC
    return _transactions.take(5).toList();
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

    if (_error != null) {
      return Scaffold(
        appBar: AppBar(
          centerTitle: false,
          title: const MenuAppBarTitle(title: 'Laporan Kas'),
        ),
        body: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(AppSpacing.lg),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                const Icon(Icons.error_outline, size: 48, color: Colors.red),
                const SizedBox(height: AppSpacing.md),
                Text(
                  'Gagal memuat data',
                  style: theme.textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.w600,
                  ),
                ),
                const SizedBox(height: AppSpacing.sm),
                Text(
                  _error!,
                  textAlign: TextAlign.center,
                  style: theme.textTheme.bodyMedium?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                  ),
                ),
                const SizedBox(height: AppSpacing.lg),
                ElevatedButton.icon(
                  onPressed: _loadData,
                  icon: const Icon(Icons.refresh),
                  label: const Text('Coba Lagi'),
                ),
              ],
            ),
          ),
        ),
      );
    }

    // Calculate summary from real balance
    final totalMasuk = int.tryParse(_balance?.totalIncome ?? '0') ?? 0;
    final totalKeluar = int.tryParse(_balance?.totalExpense ?? '0') ?? 0;
    final netBalance = int.tryParse(_balance?.netBalance ?? '0') ?? 0;

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
                    value: RupiahFormatter.format(netBalance),
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
                        value: RupiahFormatter.format(totalMasuk),
                        icon: Icons.arrow_downward,
                        accentColor: AppColors.success,
                      ),
                    ),
                    const SizedBox(width: AppSpacing.sm),
                    Expanded(
                      child: SummaryCard(
                        title: 'Total Kas Keluar',
                        value: RupiahFormatter.format(totalKeluar),
                        icon: Icons.arrow_upward,
                        accentColor: AppColors.danger,
                      ),
                    ),
                  ],
                ),
              ],
            ),

            if (_breakdown.isNotEmpty) ...[
              const SizedBox(height: AppSpacing.lg),

              // ── Rincian Per Kategori ──
              const SectionHeader(title: 'Rincian Per Kategori'),
              const SizedBox(height: AppSpacing.sm),
              ..._breakdown.map((cat) {
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
            ],

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
                              alpha: isDark ? 0.18 : 0.10,
                            ),
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
                                  Container(
                                    padding: const EdgeInsets.symmetric(horizontal: 5, vertical: 1.5),
                                    decoration: BoxDecoration(
                                      color: accentColor.withValues(alpha: isDark ? 0.20 : 0.12),
                                      borderRadius: BorderRadius.circular(4),
                                      border: Border.all(color: accentColor.withValues(alpha: 0.35)),
                                    ),
                                    child: Row(
                                      mainAxisSize: MainAxisSize.min,
                                      children: [
                                        Icon(_jenisIcon(tx.jenis), size: 9, color: accentColor),
                                        const SizedBox(width: 2),
                                        Text(
                                          isIncoming ? 'Masuk' : 'Keluar',
                                          style: TextStyle(
                                            color: accentColor,
                                            fontWeight: FontWeight.bold,
                                            fontSize: 10,
                                          ),
                                        ),
                                      ],
                                    ),
                                  ),
                                  const SizedBox(width: 6),
                                  AppBadge(
                                    label: tx.kategori.label,
                                    backgroundColor: accentColor.withValues(
                                      alpha: 0.12,
                                    ),
                                    textColor: accentColor,
                                  ),
                                  const SizedBox(width: 6),
                                  AppBadge(label: formatTanggal(tx.tanggal)),
                                ],
                              ),
                            ],
                          ),
                        ),

                        // Amount
                        Text(
                          '$prefix${RupiahFormatter.formatMoneyString(tx.nominal)}',
                          style: theme.textTheme.titleSmall?.copyWith(
                            color: accentColor,
                            fontWeight: FontWeight.bold,
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
