import 'dart:io';

import 'package:fl_chart/fl_chart.dart';
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
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import 'package:social_finance/core/widgets/section_header.dart';
import 'package:social_finance/core/widgets/summary_card.dart';
import '../../data/mock_dashboard_data.dart';
import 'package:go_router/go_router.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/kas/data/kas_models.dart';
import 'package:social_finance/features/kas/data/kas_repository.dart';
import 'package:social_finance/features/kas/presentation/screens/kas_screen.dart';
import '../widgets/financial_summary_chart.dart';

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

class DashboardScreen extends ConsumerStatefulWidget {
  final Future<void> Function()? onRefreshOverride;

  const DashboardScreen({super.key, this.onRefreshOverride});

  @override
  ConsumerState<DashboardScreen> createState() => _DashboardScreenState();
}

class _DashboardScreenState extends ConsumerState<DashboardScreen> {
  DashboardSummary? _apiSummary;
  DateTime _activeDate = DateTime.now();
  int _selectedYear = DateTime.now().year;
  bool _isYearFilterActive = false;
  bool _isRefreshing = false;
  bool _filterExpanded = false;

  static const _monthNames = [
    'Januari',
    'Februari',
    'Maret',
    'April',
    'Mei',
    'Juni',
    'Juli',
    'Agustus',
    'September',
    'Oktober',
    'November',
    'Desember',
  ];

  bool get _isCurrentMonth {
    final now = DateTime.now();
    return _activeDate.year == now.year && _activeDate.month == now.month;
  }

  String _formatMonthYearShort(DateTime dt) {
    const months = [
      '',
      'Jan',
      'Feb',
      'Mar',
      'Apr',
      'Mei',
      'Jun',
      'Jul',
      'Agu',
      'Sep',
      'Okt',
      'Nov',
      'Des',
    ];
    return '${months[dt.month]} ${dt.year}';
  }

  Future<void> _pickMonth() async {
    final months = [
      'Semua Bulan', 'Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni',
      'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember',
    ];
    int selYear = _activeDate.year;
    int selMonth = _activeDate.month;

    return await showModalBottomSheet<void>(
      context: context,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(AppRadius.base)),
      ),
      builder: (ctx) => StatefulBuilder(
        builder: (_, setModalState) {
          final isDark = Theme.of(ctx).brightness == Brightness.dark;

          return Padding(
            padding: EdgeInsets.fromLTRB(
              AppSpacing.base,
              AppSpacing.base,
              AppSpacing.base,
              MediaQuery.of(ctx).viewInsets.bottom + AppSpacing.base,
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Pilih Bulan',
                  style: Theme.of(ctx).textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.w600,
                  ),
                ),
                const SizedBox(height: AppSpacing.sm),
                GridView.builder(
                  shrinkWrap: true,
                  physics: const NeverScrollableScrollPhysics(),
                  gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
                    crossAxisCount: 4,
                    childAspectRatio: 2.2,
                    crossAxisSpacing: 8,
                    mainAxisSpacing: 8,
                  ),
                  itemCount: months.length,
                  itemBuilder: (_, i) {
                    final isAll = i == 0;
                    final isSelected = isAll ? _isYearFilterActive : (i == selMonth && !_isYearFilterActive);
                    return InkWell(
                      onTap: () {
                        setState(() {
                          if (isAll) {
                            _isYearFilterActive = true;
                          } else {
                            _activeDate = DateTime(selYear, i, 1);
                            _isYearFilterActive = false;
                          }
                        });
                        _fetchDashboard();
                        if (ctx.mounted) Navigator.of(ctx).pop();
                      },
                      borderRadius: BorderRadius.circular(AppRadius.base),
                      child: Container(
                        decoration: BoxDecoration(
                          color: isSelected
                              ? AppColors.accent.withValues(alpha: 0.2)
                              : (isDark ? AppColors.darkSurfaceHover : AppColors.lightSurfaceHover),
                          borderRadius: BorderRadius.circular(AppRadius.base),
                          border: Border.all(
                            color: isSelected ? AppColors.accent : (isDark ? AppColors.darkBorder : AppColors.lightBorder),
                            width: isSelected ? 2 : 1,
                          ),
                        ),
                        alignment: Alignment.center,
                        child: Text(
                          months[i],
                          style: TextStyle(
                            color: isSelected
                                ? AppColors.accent
                                : (isDark ? AppColors.darkText : AppColors.lightText),
                            fontWeight: isSelected ? FontWeight.w600 : null,
                          ),
                        ),
                      ),
                    );
                  },
                ),
              ],
            ),
          );
        },
      ),
    );
  }

  Future<void> _pickYear() async {
    final currentYear = DateTime.now().year;
    int selYear = _selectedYear;

    return await showModalBottomSheet<void>(
      context: context,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(AppRadius.base)),
      ),
      builder: (ctx) => StatefulBuilder(
        builder: (_, setModalState) {
          final isDark = Theme.of(ctx).brightness == Brightness.dark;
          final years = List.generate(15, (i) => currentYear - 7 + i);

          return Padding(
            padding: EdgeInsets.fromLTRB(
              AppSpacing.base,
              AppSpacing.base,
              AppSpacing.base,
              MediaQuery.of(ctx).viewInsets.bottom + AppSpacing.base,
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Pilih Tahun',
                  style: Theme.of(ctx).textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.w600,
                  ),
                ),
                const SizedBox(height: AppSpacing.md),
                GridView.builder(
                  shrinkWrap: true,
                  physics: const NeverScrollableScrollPhysics(),
                  gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
                    crossAxisCount: 4,
                    childAspectRatio: 2.0,
                    crossAxisSpacing: 8,
                    mainAxisSpacing: 8,
                  ),
                  itemCount: years.length,
                  itemBuilder: (_, i) {
                    final year = years[i];
                    final isSelected = year == selYear;
                    return InkWell(
                      onTap: () {
                        setState(() {
                          _selectedYear = year;
                          _isYearFilterActive = true;
                        });
                        _fetchDashboard();
                        if (ctx.mounted) Navigator.of(ctx).pop();
                      },
                      borderRadius: BorderRadius.circular(AppRadius.base),
                      child: Container(
                        decoration: BoxDecoration(
                          color: isSelected
                              ? (isDark ? AppColors.darkAccentSoft : AppColors.lightAccentSoft)
                              : (isDark ? AppColors.darkSurfaceHover : AppColors.lightSurfaceHover),
                          borderRadius: BorderRadius.circular(AppRadius.base),
                          border: Border.all(
                            color: isSelected ? AppColors.accent : (isDark ? AppColors.darkBorder : AppColors.lightBorder),
                            width: isSelected ? 2 : 1,
                          ),
                        ),
                        alignment: Alignment.center,
                        child: Text(
                          year.toString(),
                          style: TextStyle(
                            color: isSelected ? AppColors.accent : (isDark ? AppColors.darkText : AppColors.lightText),
                            fontWeight: isSelected ? FontWeight.w600 : null,
                          ),
                        ),
                      ),
                    );
                  },
                ),
              ],
            ),
          );
        },
      ),
    );
  }

  @override
  void initState() {
    super.initState();
    _fetchDashboard();
  }

  Future<void> _fetchDashboard() async {
    // Skip real HTTP fetch during widget/unit tests to avoid pending timers
    if (Platform.environment.containsKey('FLUTTER_TEST')) return;

    try {
      final api = ref.read(kasApiClientProvider);
      // Year filter: send "YYYY" only (backend will use Jan-Dec of that year)
      // Month filter: send "YYYY-MM"
      final periodParam = _isYearFilterActive
          ? '${_selectedYear}'
          : '${_activeDate.year}-${_activeDate.month.toString().padLeft(2, '0')}';

      final json = await api.getDashboardSummary(periodParam);
      if (mounted) {
        setState(() {
          _apiSummary = DashboardSummary.fromJson(json);
        });
      }
    } catch (_) {
      // Fallback to default in tests or offline mode
    }
  }

  Future<void> _handleRefresh() async {
    if (_isRefreshing) return;
    _isRefreshing = true;
    try {
      final user = ref.read(authRepositoryProvider)?.valueOrNull;
      final profileStatus = user?['profile_status'] as String?;
      if (user != null &&
          (profileStatus == 'stale' ||
              profileStatus == 'error' ||
              profileStatus == 'permissions_unavailable')) {
        await ref.read(authRepositoryProvider.notifier).reloadProfile();
      }
      ref.invalidate(dashboardSummaryProvider);
      ref.invalidate(recentKasTransactionsProvider);
      ref.invalidate(recentTransactionsProvider);
      if (widget.onRefreshOverride != null) {
        await widget.onRefreshOverride!();
      } else {
        await _fetchDashboard();
      }
    } finally {
      _isRefreshing = false;
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final authState = ref.watch(authRepositoryProvider);
    final defaultSummary = ref.watch(dashboardSummaryProvider);
    final DashboardSummary summary = _apiSummary ?? defaultSummary;
    final defaultKasTransactions = ref.watch(recentKasTransactionsProvider);
    final List<KasTransaction> kasTransactions = _apiSummary != null
        ? _apiSummary!.recentKasTransactions
        : defaultKasTransactions;

    final user = authState?.valueOrNull;
    final userName = user?['name'] ?? 'Pengguna';
    final roleUser = user?['role'] as AppRole? ?? AppRole.warga;
    final rt = user?['rt'] ?? '';
    final rw = user?['rw'] ?? '';

    final isDark = theme.brightness == Brightness.dark;

    return Scaffold(
        appBar: AppBar(
          automaticallyImplyLeading: false,
          centerTitle: false,
          title: const MenuAppBarTitle(title: 'Beranda'),
          actions: [
            _ThemeActionButton(
              onSelected: (mode) {
                ref.read(themeProvider.notifier).setMode(mode);
              },
            ),
            const SizedBox(width: 8),
          ],
        ),
        body: RefreshIndicator(
          key: const Key('dashboard-refresh-indicator'),
          onRefresh: _handleRefresh,
          child: SingleChildScrollView(
            physics: const AlwaysScrollableScrollPhysics(),
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
                          Row(
                            crossAxisAlignment: CrossAxisAlignment.center,
                            children: [
                              Text(
                                'Selamat datang,',
                                style: theme.textTheme.bodyMedium,
                              ),
                              const Spacer(),
                              AppBadge(
                                label: appRoleDisplayName(roleUser),
                                isNeonStyle: isDark,
                              ),
                            ],
                          ),
                          const SizedBox(height: 4),
                          Text(userName, style: theme.textTheme.headlineMedium),
                          Text(
                            'RT $rt / RW $rw',
                            style: theme.textTheme.bodySmall,
                          ),
                        ],
                      ),
                    ),
                  ],
                ),

                // ── Periode + Tahun Filter (expandable seperti KAS) ──
                _buildFilterBar(theme, isDark),

                // ── Saldo Kas ──
                const SizedBox(height: _panelGap),
                Container(
                  width: double.infinity,
                  padding: const EdgeInsets.all(24),
                  decoration: BoxDecoration(
                    color: isDark
                        ? AppColors.darkSurface
                        : AppColors.lightSurface,
                    border: Border.all(
                      color: isDark
                          ? AppColors.darkBorder.withValues(alpha: 0.5)
                          : AppColors.lightBorder,
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
                          color: AppColors.accent,
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
                                  color: AppColors.accent,
                                ),
                                const SizedBox(width: 8),
                                Text(
                                  'Saldo Kas',
                                  style: theme.textTheme.bodyMedium?.copyWith(
                                    color: AppColors.textMutedColor(context),
                                  ),
                                ),
                              ],
                            ),
                            const SizedBox(height: 12),
                            Text(
                              RupiahFormatter.format(summary.saldoKas),
                              style: theme.textTheme.headlineLarge?.copyWith(
                                color: isDark
                                    ? AppColors.darkTextPrimary
                                    : AppColors.lightTextPrimary,
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
                        title: _isCurrentMonth
                            ? 'Pemasukan Bulan Ini'
                            : 'Pemasukan (${_formatMonthYearShort(_activeDate)})',
                        value: RupiahFormatter.format(
                          summary.pemasukanBulanIni,
                        ),
                        icon: Icons.arrow_downward,
                        accentColor: AppColors.income,
                      ),
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: SummaryCard(
                        title: _isCurrentMonth
                            ? 'Pengeluaran Bulan Ini'
                            : 'Pengeluaran (${_formatMonthYearShort(_activeDate)})',
                        value: RupiahFormatter.format(
                          summary.pengeluaranBulanIni,
                        ),
                        icon: Icons.arrow_upward,
                        accentColor: AppColors.expense,
                      ),
                    ),
                  ],
                ),

                // ── Ringkasan Keuangan ──
                const SizedBox(height: _panelGap),
                FinancialSummaryChart(summary: summary, theme: theme),

                // ── Metode Pembayaran (Donut Chart: Transfer vs Tunai) ──
                const SizedBox(height: _sectionGap),
                _buildPaymentMethodDonutSection(
                  summary.paymentMethodBreakdown,
                  theme,
                ),

                // ── Iuran Bulan Ini ──
                const SizedBox(height: _sectionGap),
                _buildDuesSection(summary.duesProgress, theme),

                // ── Transaksi Terbaru ──
                const SizedBox(height: _sectionGap),
                const SectionHeader(title: 'Transaksi Terbaru'),
                const SizedBox(height: _titleGap),
                if (kasTransactions.isEmpty)
                  const Padding(
                    padding: EdgeInsets.symmetric(vertical: 16),
                    child: Center(child: Text('Belum ada transaksi.')),
                  )
                else
                  ...kasTransactions.map(
                    (tx) => Padding(
                      padding: const EdgeInsets.only(bottom: 8),
                      child: KasTransactionCard(
                        transaction: tx,
                        isDark: isDark,
                        onTap: () => context.push('/home/kas/${tx.id}'),
                      ),
                    ),
                  ),
                const SizedBox(height: AppSpacing.base),
              ],
            ),
          ),
        ),
    );
  }

  // ── Month/Year Filter Button Bar (like Kas) ──
  Widget _buildFilterBar(ThemeData theme, bool isDark) {
    return InkWell(
      onTap: () {
        setState(() {
          _filterExpanded = !_filterExpanded;
        });
      },
      borderRadius: BorderRadius.circular(AppRadius.base),
      child: Column(
        children: [
          Container(
            padding: const EdgeInsets.symmetric(
              horizontal: 12,
              vertical: 10,
            ),
            decoration: BoxDecoration(
              color: isDark
                  ? AppColors.darkSurface
                  : AppColors.lightSurface,
              borderRadius: BorderRadius.circular(AppRadius.base),
              border: Border.all(
                color: isDark
                    ? AppColors.accent.withValues(alpha: 0.35)
                    : AppColors.accent.withValues(alpha: 0.25),
                width: 1,
              ),
              boxShadow: isDark
                  ? [
                      BoxShadow(
                        color: AppColors.accent
                            .withValues(alpha: 0.12),
                        blurRadius: 8,
                        spreadRadius: 0,
                      ),
                    ]
                  : null,
            ),
            child: Row(
              children: [
                Icon(
                  Icons.tune,
                  size: 18,
                  color: isDark
                      ? AppColors.darkText
                      : AppColors.lightText,
                ),
                const SizedBox(width: 8),
                Text(
                  'Filter',
                  style: theme.textTheme.bodyMedium?.copyWith(
                    fontWeight: FontWeight.w600,
                  ),
                ),
                const Spacer(),
                AnimatedRotation(
                  turns: _filterExpanded ? 0 : 0.5,
                  duration: const Duration(milliseconds: 200),
                  child: Icon(
                    Icons.keyboard_arrow_down,
                    size: 20,
                    color: AppColors.accent,
                  ),
                ),
              ],
            ),
          ),
          AnimatedSize(
            duration: const Duration(milliseconds: 200),
            curve: Curves.easeInOut,
            alignment: Alignment.topCenter,
            child: _filterExpanded
                ? Column(
                    mainAxisSize: MainAxisSize.min,
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      const SizedBox(height: 12),
                      // Row 1: Periode + Tahun
                      Padding(
                        padding: const EdgeInsets.symmetric(
                          horizontal: AppSpacing.base,
                        ),
                        child: Row(
                          children: [
                            Expanded(
                              child: InkWell(
                                onTap: _pickMonth,
                                borderRadius: BorderRadius.circular(AppRadius.base),
                                child: Container(
                                  height: 38,
                                  alignment: Alignment.centerLeft,
                                  padding: const EdgeInsets.symmetric(horizontal: 10),
                                  decoration: BoxDecoration(
                                    color: isDark
                                        ? AppColors.darkSurface
                                        : AppColors.lightSurface,
                                    borderRadius: BorderRadius.circular(AppRadius.base),
                                    border: Border.all(
                                      color: isDark
                                          ? AppColors.accent.withValues(alpha: 0.35)
                                          : AppColors.accent.withValues(alpha: 0.25),
                                      width: 1,
                                    ),
                                  ),
                                  child: Text(
                                    _isYearFilterActive ? 'Semua Bulan' : _monthNames[_activeDate.month - 1],
                                    style: theme.textTheme.bodySmall?.copyWith(
                                      fontWeight: FontWeight.w600,
                                    ),
                                    maxLines: 1,
                                    overflow: TextOverflow.ellipsis,
                                  ),
                                ),
                              ),
                            ),
                            const SizedBox(width: AppSpacing.sm),
                            Expanded(
                              child: InkWell(
                                onTap: _pickYear,
                                borderRadius: BorderRadius.circular(AppRadius.base),
                                child: Container(
                                  height: 38,
                                  alignment: Alignment.centerLeft,
                                  padding: const EdgeInsets.symmetric(horizontal: 10),
                                  decoration: BoxDecoration(
                                    color: isDark
                                        ? AppColors.darkSurface
                                        : AppColors.lightSurface,
                                    borderRadius: BorderRadius.circular(AppRadius.base),
                                    border: Border.all(
                                      color: isDark
                                          ? AppColors.accent.withValues(alpha: 0.35)
                                          : AppColors.accent.withValues(alpha: 0.25),
                                      width: 1,
                                    ),
                                  ),
                                  child: Text(
                                    _selectedYear.toString(),
                                    style: theme.textTheme.bodySmall?.copyWith(
                                      fontWeight: FontWeight.w600,
                                    ),
                                    maxLines: 1,
                                    overflow: TextOverflow.ellipsis,
                                  ),
                                ),
                              ),
                            ),
                          ],
                        ),
                      ),
                    ],
                  )
                : const SizedBox.shrink(),
          ),
        ],
      ),
    );
  }

  Widget _buildPaymentMethodDonutSection(
    PaymentMethodBreakdown breakdown,
    ThemeData theme,
  ) {
    final isDark = theme.brightness == Brightness.dark;
    final hasData = breakdown.totalAmount > 0 || breakdown.totalCount > 0;
    final transferVal = breakdown.transferAmount > 0
        ? breakdown.transferAmount.toDouble()
        : (breakdown.transferCount > 0
              ? breakdown.transferCount.toDouble()
              : 0.0);
    final cashVal = breakdown.cashAmount > 0
        ? breakdown.cashAmount.toDouble()
        : (breakdown.cashCount > 0 ? breakdown.cashCount.toDouble() : 0.0);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const SectionHeader(title: 'Metode Pembayaran'),
        const SizedBox(height: _titleGap),
        AppCard(
          key: const Key('dashboard-payment-method-donut-card'),
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: !hasData
                ? Padding(
                    padding: const EdgeInsets.symmetric(vertical: 16),
                    child: Center(
                      child: Column(
                        children: [
                          Icon(
                            Icons.donut_large_outlined,
                            size: 32,
                            color: AppColors.textMutedColor(context),
                          ),
                          const SizedBox(height: 8),
                          Text(
                            'Belum ada transaksi Tunai atau Transfer pada periode ini.',
                            style: theme.textTheme.bodySmall?.copyWith(
                              color: AppColors.textMutedColor(context),
                            ),
                            textAlign: TextAlign.center,
                          ),
                        ],
                      ),
                    ),
                  )
                : Row(
                    children: [
                      SizedBox(
                        width: 112,
                        height: 112,
                        child: Stack(
                          alignment: Alignment.center,
                          children: [
                            PieChart(
                              PieChartData(
                                sectionsSpace: 2,
                                centerSpaceRadius: 34,
                                sections: [
                                  if (transferVal > 0)
                                    PieChartSectionData(
                                      color: AppColors.accent,
                                      value: transferVal,
                                      radius: 16,
                                      showTitle: false,
                                    ),
                                  if (cashVal > 0)
                                    PieChartSectionData(
                                      color: AppColors.info,
                                      value: cashVal,
                                      radius: 16,
                                      showTitle: false,
                                    ),
                                ],
                              ),
                            ),
                            Column(
                              mainAxisSize: MainAxisSize.min,
                              children: [
                                Text(
                                  'Total',
                                  style: theme.textTheme.bodySmall?.copyWith(
                                    fontSize: 10,
                                    color: AppColors.textMutedColor(context),
                                  ),
                                ),
                                Text(
                                  '${breakdown.totalCount} Tx',
                                  style: theme.textTheme.bodyMedium?.copyWith(
                                    fontWeight: FontWeight.bold,
                                    fontSize: 12,
                                  ),
                                ),
                              ],
                            ),
                          ],
                        ),
                      ),
                      const SizedBox(width: 16),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            _buildPaymentMethodBreakdownItem(
                              theme: theme,
                              isDark: isDark,
                              label: 'Transfer',
                              color: AppColors.accent,
                              amount: breakdown.transferAmount,
                              percentage: breakdown.transferPercentage,
                              count: breakdown.transferCount,
                            ),
                            const SizedBox(height: 10),
                            _buildPaymentMethodBreakdownItem(
                              theme: theme,
                              isDark: isDark,
                              label: 'Tunai',
                              color: AppColors.info,
                              amount: breakdown.cashAmount,
                              percentage: breakdown.cashPercentage,
                              count: breakdown.cashCount,
                            ),
                          ],
                        ),
                      ),
                    ],
                  ),
          ),
        ),
      ],
    );
  }

  Widget _buildPaymentMethodBreakdownItem({
    required ThemeData theme,
    required bool isDark,
    required String label,
    required Color color,
    required int amount,
    required int percentage,
    required int count,
  }) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
      decoration: BoxDecoration(
        color: color.withValues(alpha: isDark ? 0.10 : 0.06),
        borderRadius: BorderRadius.circular(AppRadius.sm),
        border: Border.all(
          color: color.withValues(alpha: isDark ? 0.30 : 0.22),
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 8,
                height: 8,
                decoration: BoxDecoration(color: color, shape: BoxShape.circle),
              ),
              const SizedBox(width: 6),
              Expanded(
                child: Text(
                  label,
                  style: theme.textTheme.bodySmall?.copyWith(
                    fontWeight: FontWeight.w700,
                  ),
                ),
              ),
              Text(
                '$percentage%',
                style: theme.textTheme.bodySmall?.copyWith(
                  color: color,
                  fontWeight: FontWeight.bold,
                ),
              ),
            ],
          ),
          const SizedBox(height: 2),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Flexible(
                child: Text(
                  RupiahFormatter.format(amount),
                  style: theme.textTheme.bodySmall?.copyWith(
                    fontWeight: FontWeight.w700,
                    fontSize: 12,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              const SizedBox(width: 4),
              Text(
                '$count tx',
                style: theme.textTheme.bodySmall?.copyWith(
                  fontSize: 10.5,
                  color: AppColors.textMutedColor(context),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildDuesSection(DuesProgress dues, ThemeData theme) {
    final isDark = theme.brightness == Brightness.dark;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SectionHeader(
          title: _isCurrentMonth
              ? 'Iuran Bulan Ini'
              : 'Iuran (${_formatMonthYearShort(_activeDate)})',
        ),
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
                        color: isDark
                            ? AppColors.darkAccentSoft
                            : AppColors.lightAccentSoft,
                        borderRadius: BorderRadius.circular(AppRadius.sm),
                      ),
                      child: Icon(Icons.receipt_long, color: AppColors.accent),
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
                        color: AppColors.accent,
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
                          backgroundColor: AppColors.accent.withValues(
                            alpha: isDark ? 0.2 : 0.15,
                          ),
                          valueColor: AlwaysStoppedAnimation(AppColors.accent),
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
