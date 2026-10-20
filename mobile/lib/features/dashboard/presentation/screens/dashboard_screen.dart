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
  bool _isAllPeriods = false;
  bool _isRefreshing = false;

  bool get _isCurrentMonth {
    final now = DateTime.now();
    return _activeDate.year == now.year && _activeDate.month == now.month;
  }

  String _formatMonthYear(DateTime dt) {
    const months = [
      '',
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
    return '${months[dt.month]} ${dt.year}';
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

  void _prevMonth() {
    setState(() {
      _isAllPeriods = false;
      _activeDate = DateTime(_activeDate.year, _activeDate.month - 1);
    });
    _fetchDashboard();
  }

  void _nextMonth() {
    setState(() {
      _isAllPeriods = false;
      _activeDate = DateTime(_activeDate.year, _activeDate.month + 1);
    });
    _fetchDashboard();
  }

  Future<void> _pickMonth() async {
    final picked = await showDatePicker(
      context: context,
      initialDate: _activeDate,
      firstDate: DateTime(2020),
      lastDate: DateTime(2035),
      initialDatePickerMode: DatePickerMode.year,
      helpText: 'PILIH PERIODE BULAN',
    );
    if (picked != null) {
      setState(() {
        _isAllPeriods = false;
        _activeDate = DateTime(picked.year, picked.month, 1);
      });
      _fetchDashboard();
    }
  }

  void _toggleAllPeriods() {
    setState(() {
      _isAllPeriods = !_isAllPeriods;
    });
    _fetchDashboard();
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
      final periodParam = _isAllPeriods
          ? 'all'
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

                // ── Month Filter Bar ──
                const SizedBox(height: _panelGap),
                Container(
                  decoration: BoxDecoration(
                    color: theme.scaffoldBackgroundColor,
                    borderRadius: BorderRadius.circular(AppRadius.base),
                    border: Border.all(
                      color: isDark
                          ? AppColors.darkBorder
                          : AppColors.lightBorder,
                    ),
                  ),
                  padding: const EdgeInsets.symmetric(
                    horizontal: AppSpacing.sm,
                    vertical: AppSpacing.xs,
                  ),
                  child: Row(
                    children: [
                      IconButton(
                        icon: const Icon(Icons.chevron_left),
                        onPressed: _isAllPeriods ? null : _prevMonth,
                        tooltip: 'Bulan Sebelumnya',
                        visualDensity: VisualDensity.compact,
                      ),
                      Expanded(
                        child: InkWell(
                          onTap: _isAllPeriods ? null : _pickMonth,
                          borderRadius: BorderRadius.circular(AppRadius.sm),
                          child: Padding(
                            padding: const EdgeInsets.symmetric(
                              vertical: 4,
                              horizontal: 8,
                            ),
                            child: Row(
                              mainAxisAlignment: MainAxisAlignment.center,
                              children: [
                                Text(
                                  _isAllPeriods
                                      ? 'Semua Periode'
                                      : _formatMonthYear(_activeDate),
                                  style: theme.textTheme.titleMedium?.copyWith(
                                    fontWeight: FontWeight.bold,
                                    color: _isAllPeriods
                                        ? theme.colorScheme.onSurfaceVariant
                                        : AppColors.accent,
                                  ),
                                ),
                                if (!_isAllPeriods) ...[
                                  const SizedBox(width: 4),
                                  const Icon(
                                    Icons.arrow_drop_down,
                                    size: 20,
                                    color: AppColors.accent,
                                  ),
                                ],
                              ],
                            ),
                          ),
                        ),
                      ),
                      IconButton(
                        icon: const Icon(Icons.chevron_right),
                        onPressed: _isAllPeriods ? null : _nextMonth,
                        tooltip: 'Bulan Berikutnya',
                        visualDensity: VisualDensity.compact,
                      ),
                      const SizedBox(width: 4),
                      ChoiceChip(
                        label: Text(
                          _isAllPeriods ? 'Per Bulan' : 'Semua',
                          style: const TextStyle(fontSize: 11),
                        ),
                        selected: _isAllPeriods,
                        visualDensity: VisualDensity.compact,
                        onSelected: (val) {
                          _toggleAllPeriods();
                        },
                      ),
                    ],
                  ),
                ),

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
