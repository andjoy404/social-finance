import 'dart:io';

import 'package:fl_chart/fl_chart.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/utils/rupiah_formatter.dart';
import 'package:social_finance/core/utils/rw_utils.dart';
import 'package:social_finance/core/widgets/app_badge.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import 'package:social_finance/core/widgets/section_header.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/dashboard/data/mock_dashboard_data.dart';
import 'package:social_finance/features/dashboard/presentation/screens/dashboard_screen.dart';
import 'package:social_finance/features/dashboard/presentation/widgets/financial_summary_chart.dart';
import 'package:social_finance/features/kas/data/kas_models.dart';
import 'package:social_finance/features/kas/data/kas_repository.dart';
import 'package:social_finance/features/kas/presentation/screens/kas_screen.dart';

const _panelGap = AppSpacing.md;
const _sectionGap = AppSpacing.lg;
const _titleGap = AppSpacing.sm;

class KeuanganScreen extends ConsumerStatefulWidget {
  final Future<void> Function()? onRefreshOverride;

  const KeuanganScreen({super.key, this.onRefreshOverride});

  @override
  ConsumerState<KeuanganScreen> createState() => _KeuanganScreenState();
}

class _KeuanganScreenState extends ConsumerState<KeuanganScreen> {
  DashboardSummary? _apiSummary;
  String? _summaryError;
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
      'Semua Bulan',
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
    int selYear = _activeDate.year;
    int selMonth = _activeDate.month;

    return await showModalBottomSheet<void>(
      context: context,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(
          top: Radius.circular(AppRadius.base),
        ),
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
                    final isSelected = isAll
                        ? _isYearFilterActive
                        : (i == selMonth && !_isYearFilterActive);
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
                              : (isDark
                                    ? AppColors.darkSurfaceHover
                                    : AppColors.lightSurfaceHover),
                          borderRadius: BorderRadius.circular(AppRadius.base),
                          border: Border.all(
                            color: isSelected
                                ? AppColors.accent
                                : (isDark
                                      ? AppColors.darkBorder
                                      : AppColors.lightBorder),
                            width: isSelected ? 2 : 1,
                          ),
                        ),
                        alignment: Alignment.center,
                        child: Text(
                          months[i],
                          style: TextStyle(
                            color: isSelected
                                ? AppColors.accent
                                : (isDark
                                      ? AppColors.darkText
                                      : AppColors.lightText),
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
        borderRadius: BorderRadius.vertical(
          top: Radius.circular(AppRadius.base),
        ),
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
                              ? (isDark
                                    ? AppColors.darkAccentSoft
                                    : AppColors.lightAccentSoft)
                              : (isDark
                                    ? AppColors.darkSurfaceHover
                                    : AppColors.lightSurfaceHover),
                          borderRadius: BorderRadius.circular(AppRadius.base),
                          border: Border.all(
                            color: isSelected
                                ? AppColors.accent
                                : (isDark
                                      ? AppColors.darkBorder
                                      : AppColors.lightBorder),
                            width: isSelected ? 2 : 1,
                          ),
                        ),
                        alignment: Alignment.center,
                        child: Text(
                          year.toString(),
                          style: TextStyle(
                            color: isSelected
                                ? AppColors.accent
                                : (isDark
                                      ? AppColors.darkText
                                      : AppColors.lightText),
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
    if (Platform.environment.containsKey('FLUTTER_TEST')) return;

    try {
      final api = ref.read(kasApiClientProvider);
      final periodParam = _isYearFilterActive
          ? '$_selectedYear'
          : '${_activeDate.year}-${_activeDate.month.toString().padLeft(2, '0')}';

      final json = await api.getDashboardSummary(periodParam);
      if (mounted) {
        setState(() {
          _apiSummary = DashboardSummary.fromJson(json);
          _summaryError = null;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          // Never fall back to mock/current-month data on failure; surface a
          // real error state instead.
          _summaryError = e.toString();
          _apiSummary = null;
        });
      }
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

  Widget _buildSummaryErrorState(ThemeData theme, bool isDark) {
    return ListView(
      physics: const AlwaysScrollableScrollPhysics(),
      padding: const EdgeInsets.all(AppSpacing.base),
      children: [
        const SizedBox(height: AppSpacing.xl),
        Icon(Icons.error_outline, size: 48, color: AppColors.danger),
        const SizedBox(height: AppSpacing.sm),
        Text(
          'Gagal memuat ringkasan keuangan',
          textAlign: TextAlign.center,
          style: theme.textTheme.titleMedium?.copyWith(
            color: AppColors.danger,
          ),
        ),
        const SizedBox(height: AppSpacing.xs),
        Text(
          _summaryError ?? 'Terjadi kesalahan',
          textAlign: TextAlign.center,
          style: theme.textTheme.bodySmall?.copyWith(
            color: Theme.of(context).colorScheme.onSurface.withValues(alpha: 0.7),
          ),
        ),
        const SizedBox(height: AppSpacing.base),
        Center(
          child: OutlinedButton.icon(
            onPressed: () {
              setState(() {
                _summaryError = null;
              });
              _fetchDashboard();
            },
            icon: const Icon(Icons.refresh),
            label: const Text('Coba Lagi'),
          ),
        ),
      ],
    );
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
    final rawRw = user?['rw']?.toString() ?? '';
    final rw = looksLikeUuid(rawRw)
        ? (resolveRwNumber(rawRw, user?['rw_memberships']) ?? '')
        : rawRw;
    final rtRwDisplay = [
      if (rt.isNotEmpty) 'RT $rt',
      if (rw.isNotEmpty) 'RW $rw',
    ].join(' / ');

    final isDark = theme.brightness == Brightness.dark;

    return Scaffold(
      appBar: AppBar(
        automaticallyImplyLeading: false,
        centerTitle: false,
        title: const MenuAppBarTitle(title: 'Keuangan RT'),
      ),
      body: RefreshIndicator(
        key: const Key('dashboard-refresh-indicator'),
        onRefresh: _handleRefresh,
        child: _summaryError != null && _apiSummary == null
            ? _buildSummaryErrorState(theme, isDark)
            : SingleChildScrollView(
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
                          rtRwDisplay.isNotEmpty ? rtRwDisplay : 'RT / RW',
                          style: theme.textTheme.bodySmall,
                        ),
                      ],
                    ),
                  ),
                ],
              ),

              const SizedBox(height: AppSpacing.sm),

              // ── Periode + Tahun Filter ──
              _buildFilterBar(theme, isDark),

              // ── Saldo Kas ──
              const SizedBox(height: _panelGap),
              _buildSummaryCard(
                context: context,
                title: 'Saldo Kas',
                value: RupiahFormatter.format(summary.saldoKas),
                color: AppColors.accent,
                icon: Icons.account_balance_wallet_rounded,
              ),

              // ── Pemasukan / Pengeluaran ──
              const SizedBox(height: 12),
              Row(
                children: [
                  Expanded(
                    child: _buildSummaryCard(
                      context: context,
                      title: _isCurrentMonth
                          ? 'Pemasukan Bulan Ini'
                          : 'Pemasukan (${_formatMonthYearShort(_activeDate)})',
                      value: RupiahFormatter.format(summary.pemasukanBulanIni),
                      color: AppColors.income,
                      icon: Icons.arrow_downward,
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: _buildSummaryCard(
                      context: context,
                      title: _isCurrentMonth
                          ? 'Pengeluaran Bulan Ini'
                          : 'Pengeluaran (${_formatMonthYearShort(_activeDate)})',
                      value: RupiahFormatter.format(
                        summary.pengeluaranBulanIni,
                      ),
                      color: AppColors.expense,
                      icon: Icons.arrow_upward,
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
                      onCopy: () async {
                        try {
                          await Clipboard.setData(
                            ClipboardData(text: tx.referensi ?? ''),
                          );
                          if (context.mounted) {
                            ScaffoldMessenger.of(context).showSnackBar(
                              const SnackBar(
                                content: Text('Referensi berhasil disalin.'),
                                behavior: SnackBarBehavior.floating,
                                duration: Duration(milliseconds: 1500),
                              ),
                            );
                          }
                        } catch (e) {
                          if (context.mounted) {
                            ScaffoldMessenger.of(context).showSnackBar(
                              SnackBar(
                                content: Text('Gagal menyalin: $e'),
                                behavior: SnackBarBehavior.floating,
                                backgroundColor: AppColors.danger,
                              ),
                            );
                          }
                        }
                      },
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
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
            decoration: BoxDecoration(
              color: isDark ? AppColors.darkSurface : AppColors.lightSurface,
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
                        color: AppColors.accent.withValues(alpha: 0.12),
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
                  color: isDark ? AppColors.darkText : AppColors.lightText,
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
                  child: const Icon(
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
                      Padding(
                        padding: const EdgeInsets.symmetric(
                          horizontal: AppSpacing.base,
                        ),
                        child: Row(
                          children: [
                            Expanded(
                              child: InkWell(
                                onTap: _pickMonth,
                                borderRadius: BorderRadius.circular(
                                  AppRadius.base,
                                ),
                                child: Container(
                                  height: 38,
                                  alignment: Alignment.centerLeft,
                                  padding: const EdgeInsets.symmetric(
                                    horizontal: 10,
                                  ),
                                  decoration: BoxDecoration(
                                    color: isDark
                                        ? AppColors.darkSurface
                                        : AppColors.lightSurface,
                                    borderRadius: BorderRadius.circular(
                                      AppRadius.base,
                                    ),
                                    border: Border.all(
                                      color: isDark
                                          ? AppColors.accent.withValues(
                                              alpha: 0.35,
                                            )
                                          : AppColors.accent.withValues(
                                              alpha: 0.25,
                                            ),
                                      width: 1,
                                    ),
                                  ),
                                  child: Text(
                                    _isYearFilterActive
                                        ? 'Semua Bulan'
                                        : _monthNames[_activeDate.month - 1],
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
                                borderRadius: BorderRadius.circular(
                                  AppRadius.base,
                                ),
                                child: Container(
                                  height: 38,
                                  alignment: Alignment.centerLeft,
                                  padding: const EdgeInsets.symmetric(
                                    horizontal: 10,
                                  ),
                                  decoration: BoxDecoration(
                                    color: isDark
                                        ? AppColors.darkSurface
                                        : AppColors.lightSurface,
                                    borderRadius: BorderRadius.circular(
                                      AppRadius.base,
                                    ),
                                    border: Border.all(
                                      color: isDark
                                          ? AppColors.accent.withValues(
                                              alpha: 0.35,
                                            )
                                          : AppColors.accent.withValues(
                                              alpha: 0.25,
                                            ),
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
                      child: const Icon(Icons.receipt_long, color: AppColors.accent),
                    ),
                    const SizedBox(width: 14),
                    Expanded(
                      child: Text(
                        '${dues.paid} dari ${dues.target} warga sudah membayar',
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
                          valueColor: const AlwaysStoppedAnimation(AppColors.accent),
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

  void _showSummaryCardDetail({
    required BuildContext context,
    required String title,
    required String value,
    required Color color,
    IconData? icon,
    String? subtitle,
  }) {
    showModalBottomSheet<void>(
      context: context,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(AppRadius.lg)),
      ),
      builder: (ctx) {
        final isDark = Theme.of(ctx).brightness == Brightness.dark;
        return SafeArea(
          child: Padding(
            padding: const EdgeInsets.all(AppSpacing.base),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Center(
                  child: Container(
                    width: 36,
                    height: 4,
                    decoration: BoxDecoration(
                      color: isDark
                          ? AppColors.darkBorder
                          : AppColors.lightBorder,
                      borderRadius: BorderRadius.circular(2),
                    ),
                  ),
                ),
                const SizedBox(height: AppSpacing.base),
                Row(
                  children: [
                    if (icon != null) ...[
                      Container(
                        padding: const EdgeInsets.all(8),
                        decoration: BoxDecoration(
                          color: color.withValues(
                            alpha: isDark ? 0.2 : 0.1,
                          ),
                          borderRadius: BorderRadius.circular(AppRadius.sm),
                        ),
                        child: Icon(icon, size: 20, color: color),
                      ),
                      const SizedBox(width: AppSpacing.sm),
                    ],
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            title,
                            style: Theme.of(
                              ctx,
                            ).textTheme.bodySmall?.copyWith(
                              color: isDark
                                  ? AppColors.darkTextMuted
                                  : AppColors.lightTextMuted,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                          const SizedBox(height: 2),
                          SelectableText(
                            value,
                            style: Theme.of(ctx).textTheme.headlineSmall
                                ?.copyWith(
                                  color: color,
                                  fontWeight: FontWeight.w700,
                                ),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
                if (subtitle != null) ...[
                  const SizedBox(height: AppSpacing.sm),
                  Text(
                    subtitle,
                    style: Theme.of(ctx).textTheme.bodyMedium?.copyWith(
                      color: isDark
                          ? AppColors.darkTextMuted
                          : AppColors.lightTextMuted,
                    ),
                  ),
                ],
                const SizedBox(height: AppSpacing.base),
                SizedBox(
                  width: double.infinity,
                  child: OutlinedButton(
                    onPressed: () => Navigator.pop(ctx),
                    child: const Text('Tutup'),
                  ),
                ),
              ],
            ),
          ),
        );
      },
    );
  }

  Widget _buildSummaryCard({
    required BuildContext context,
    required String title,
    required String value,
    required Color color,
    required IconData icon,
    String? subtitle,
  }) {
    return InkWell(
      onTap: () => _showSummaryCardDetail(
        context: context,
        title: title,
        value: value,
        color: color,
        icon: icon,
        subtitle: subtitle,
      ),
      borderRadius: BorderRadius.circular(AppRadius.base),
      child: Container(
        padding: const EdgeInsets.all(14),
        decoration: BoxDecoration(
          color: color.withValues(alpha: 0.05),
          borderRadius: BorderRadius.circular(AppRadius.base),
          border: Border.all(color: color.withValues(alpha: 0.1)),
        ),
        child: Row(
          children: [
            Container(
              padding: const EdgeInsets.all(6),
              decoration: BoxDecoration(
                color: color.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(AppRadius.sm),
              ),
              child: Icon(icon, size: 16, color: color),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    title,
                    style: const TextStyle(
                      fontSize: 10,
                      fontWeight: FontWeight.w600,
                      color: AppColors.lightTextMuted,
                    ),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                  const SizedBox(height: 2),
                  FittedBox(
                    fit: BoxFit.scaleDown,
                    alignment: Alignment.centerLeft,
                    child: Text(
                      value,
                      style: TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.w700,
                        color: color,
                      ),
                    ),
                  ),
                  if (subtitle != null && subtitle.isNotEmpty)
                    Text(
                      subtitle,
                      style: TextStyle(
                        fontSize: 11,
                        color: AppColors.textMutedColor(context),
                      ),
                    ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
