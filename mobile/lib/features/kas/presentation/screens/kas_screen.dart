import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/utils/rupiah_formatter.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/app_text_field.dart';
import 'package:social_finance/core/widgets/double_back_exit_scope.dart';
import 'package:social_finance/core/widgets/empty_state.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import 'package:social_finance/features/kas/data/kas_mock_data.dart';
import 'package:social_finance/features/kas/data/kas_models.dart';
import 'package:social_finance/core/widgets/app_badge.dart';

/// Main KAS (Cash) screen with transaction list.
class KasScreen extends ConsumerStatefulWidget {
  const KasScreen({super.key});

  @override
  ConsumerState<KasScreen> createState() => _KasScreenState();
}

class _KasScreenState extends ConsumerState<KasScreen> {
  late final TextEditingController _searchController;
  String _selectedJenis = 'semua';
  String _selectedKategori = 'semua';

  static const _kategoriOptions = [
    'semua',
    'Iuran Warga',
    'Iuran Paksa',
    'Perlengkapan',
    'Konsumsi',
    'Perbaikan',
    'Kebersihan',
    'Keamanan',
    'Lainnya',
  ];

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

  List<KasTransaction> _filterTransactions(List<KasTransaction> all) {
    final query = _searchController.text.trim().toLowerCase();
    return all.where((tx) {
      if (_selectedJenis != 'semua' && tx.jenis.value != _selectedJenis) return false;
      if (_selectedKategori != 'semua' && tx.kategori.label != _selectedKategori) return false;
      if (query.isNotEmpty &&
          !tx.keterangan.toLowerCase().contains(query) &&
          !tx.kategori.label.toLowerCase().contains(query)) return false;
      return true;
    }).toList();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;
    final allTransactions = KasMockData.getTransactions();
    final filtered = _filterTransactions(allTransactions);
    final totalMasuk = filtered
        .where((tx) => tx.jenis == KasJenis.masuk)
        .fold<int>(0, (sum, tx) => sum + tx.nominal);
    final totalKeluar = filtered
        .where((tx) => tx.jenis == KasJenis.keluar)
        .fold<int>(0, (sum, tx) => sum + tx.nominal);
    // Last transaction's saldo is the running balance
    final runningSaldo = filtered.isNotEmpty ? filtered.first.saldo : 5000000;

    return DoubleBackExitScope(
      child: Scaffold(
        appBar: AppBar(
          centerTitle: false,
          title: const MenuAppBarTitle(title: 'Kas'),
          actions: [
            IconButton(
              icon: const Icon(Icons.bar_chart_outlined, size: 20),
              tooltip: 'Laporan KAS',
              onPressed: () => context.push('/home/kas/laporan'),
            ),
            const SizedBox(width: 8),
          ],
        ),
        body: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // ── Summary Cards ──
            Padding(
              padding: const EdgeInsets.fromLTRB(
                AppSpacing.base,
                AppSpacing.sm,
                AppSpacing.base,
                AppSpacing.xs,
              ),
              child: Row(
                children: [
                  Expanded(
                    child: SummaryChip(
                      label: 'Saldo',
                      value: RupiahFormatter.format(runningSaldo),
                      color: AppColors.accent,
                    ),
                  ),
                  const SizedBox(width: AppSpacing.sm),
                  Expanded(
                    child: SummaryChip(
                      label: 'Masuk',
                      value: RupiahFormatter.format(totalMasuk),
                      color: AppColors.success,
                    ),
                  ),
                  const SizedBox(width: AppSpacing.sm),
                  Expanded(
                    child: SummaryChip(
                      label: 'Keluar',
                      value: RupiahFormatter.format(totalKeluar),
                      color: AppColors.danger,
                    ),
                  ),
                ],
              ),
            ),

            // ── Search ──
            Padding(
              padding: const EdgeInsets.fromLTRB(
                AppSpacing.base,
                AppSpacing.xs,
                AppSpacing.base,
                AppSpacing.xs,
              ),
              child: AppTextField(
                controller: _searchController,
                hintText: 'Cari transaksi...',
                prefixIcon: const Icon(Icons.search, size: 20),
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
            Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                // Jenis filter
                Padding(
                  padding: const EdgeInsets.only(
                    left: AppSpacing.base,
                    bottom: AppSpacing.xxs,
                  ),
                  child: Text(
                    'Jenis',
                    style: theme.textTheme.bodySmall?.copyWith(
                      fontWeight: FontWeight.w600,
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  ),
                ),
                SizedBox(
                  height: 32,
                  child: ListView.horizontal(
                    padding: const EdgeInsets.symmetric(horizontal: AppSpacing.base),
                    children: [
                      _filterChip('Semua', _selectedJenis == 'semua', () {
                        setState(() => _selectedJenis = 'semua');
                      }),
                      _filterChip('Masuk', _selectedJenis == 'masuk', () {
                        setState(() => _selectedJenis = 'masuk');
                      }),
                      _filterChip('Keluar', _selectedJenis == 'keluar', () {
                        setState(() => _selectedJenis = 'keluar');
                      }),
                    ],
                  ),
                ),

                const SizedBox(height: AppSpacing.xs),

                // Kategori filter
                Padding(
                  padding: const EdgeInsets.only(
                    left: AppSpacing.base,
                    bottom: AppSpacing.xxs,
                  ),
                  child: Text(
                    'Kategori',
                    style: theme.textTheme.bodySmall?.copyWith(
                      fontWeight: FontWeight.w600,
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  ),
                ),
                SizedBox(
                  height: 32,
                  child: ListView.horizontal(
                    padding: const EdgeInsets.symmetric(horizontal: AppSpacing.base),
                    children: _kategoriOptions.map((kategori) {
                      final isSelected = _selectedKategori == kategori;
                      return _filterChip(
                        kategori,
                        isSelected,
                        () {
                          setState(() => _selectedKategori = kategori);
                        },
                      );
                    }).toList(),
                  ),
                ),
              ],
            ),

            const Divider(height: 1),

            // ── Content ──
            Expanded(
              child: filtered.isEmpty
                  ? Center(
                      child: SingleChildScrollView(
                        padding: const EdgeInsets.all(AppSpacing.lg),
                        child: _searchController.text.isNotEmpty
                            ? EmptyState(
                                icon: Icons.search_off,
                                title: 'Transaksi Tidak Ditemukan',
                                description:
                                    'Tidak ada transaksi yang sesuai dengan pencarian.',
                                actionText: 'Hapus Pencarian',
                                onAction: _clearSearch,
                              )
                            : const EmptyState(
                                icon: Icons.receipt_long_outlined,
                                title: 'Belum Ada Transaksi',
                                description:
                                    'Transaksi KAS akan muncul di sini.',
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
                        final tx = filtered[index];
                        return KasTransactionCard(
                          transaction: tx,
                          isDark: isDark,
                        );
                      },
                    ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _filterChip(
    String label,
    bool isSelected,
    VoidCallback onTap,
  ) {
    return Padding(
      padding: const EdgeInsets.only(right: 6),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(AppRadius.sm),
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
          decoration: BoxDecoration(
            color: isSelected
                ? AppColors.accent.withValues(alpha: 0.12)
                : Colors.transparent,
            borderRadius: BorderRadius.circular(AppRadius.sm),
            border: Border.all(
              color: isSelected
                  ? AppColors.accent.withValues(alpha: 0.35)
                  : AppColors.lightBorder,
              width: 1,
            ),
          ),
          child: Text(
            label,
            style: TextStyle(
              fontSize: 11,
              fontWeight: isSelected ? FontWeight.w600 : FontWeight.w500,
              color: isSelected ? AppColors.accent : AppColors.lightTextMuted,
            ),
          ),
        ),
      ),
    );
  }
}

/// Compact summary chip for the top summary row.
class SummaryChip extends StatelessWidget {
  final String label;
  final String value;
  final Color color;

  const SummaryChip({
    super.key,
    required this.label,
    required this.value,
    required this.color,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return AppCard(
      padding: const EdgeInsets.all(10),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            label,
            style: theme.textTheme.bodySmall?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
              fontSize: 10,
            ),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
          const SizedBox(height: 4),
          Text(
            value,
            style: theme.textTheme.titleSmall?.copyWith(
              color: color,
              fontWeight: FontWeight.bold,
              fontSize: 12,
            ),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
        ],
      ),
    );
  }
}

/// Transaction card for the KAS list.
class KasTransactionCard extends StatelessWidget {
  final KasTransaction transaction;
  final bool isDark;

  const KasTransactionCard({
    super.key,
    required this.transaction,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isIncoming = transaction.jenis == KasJenis.masuk;
    final accentColor = isIncoming ? AppColors.success : AppColors.danger;
    final icon = isIncoming ? Icons.arrow_downward : Icons.arrow_upward;
    final prefix = isIncoming ? '+' : '-';

    return AppCard(
      padding: const EdgeInsets.all(AppSpacing.base),
      child: Row(
        children: [
          // Icon circle
          Container(
            padding: const EdgeInsets.all(10),
            decoration: BoxDecoration(
              color: accentColor.withValues(alpha: isDark ? 0.18 : 0.10),
              borderRadius: BorderRadius.circular(AppRadius.sm),
            ),
            child: Icon(icon, size: 18, color: accentColor),
          ),
          const SizedBox(width: AppSpacing.md),

          // Description + date
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  transaction.keterangan,
                  style: theme.textTheme.bodyMedium,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
                const SizedBox(height: 4),
                Row(
                  children: [
                    Text(
                      transaction.kategori.label,
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: theme.colorScheme.onSurfaceVariant,
                        fontSize: 10,
                      ),
                    ),
                    const SizedBox(width: 8),
                    AppBadge(label: formatTanggal(transaction.tanggal)),
                  ],
                ),
              ],
            ),
          ),

          // Amount
          Column(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              Text(
                '$prefix${RupiahFormatter.format(transaction.nominal)}',
                style: theme.textTheme.titleSmall?.copyWith(
                  color: accentColor,
                  fontWeight: FontWeight.w600,
                  fontSize: 13,
                ),
              ),
              const SizedBox(height: 2),
              Text(
                formatRupiah(transaction.saldo),
                style: theme.textTheme.bodySmall?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                  fontSize: 10,
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}
