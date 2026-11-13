import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/models.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/utils/rupiah_formatter.dart';
import 'package:social_finance/core/utils/rw_utils.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/dashboard/data/mock_dashboard_data.dart';
import 'package:social_finance/features/dashboard/presentation/widgets/financial_summary_chart.dart';
import 'package:social_finance/features/rw/data/rw_models.dart';
import 'package:social_finance/features/rw/data/rw_providers.dart';

class RwDashboardScreen extends ConsumerStatefulWidget {
  final String rwId;
  final bool isTabView;

  const RwDashboardScreen({
    super.key,
    required this.rwId,
    this.isTabView = false,
  });

  @override
  ConsumerState<RwDashboardScreen> createState() => _RwDashboardScreenState();
}

class _RwDashboardScreenState extends ConsumerState<RwDashboardScreen>
    with AutomaticKeepAliveClientMixin {
  @override
  bool get wantKeepAlive => true;
  RwDashboardSummary? _data;
  List<RwTransaction> _recentTransactions = [];
  List<RwTransaction> _yearTransactions = [];
  DateTime _activeDate = DateTime.now();
  bool _isAllPeriods = false;
  bool _isLoading = true;
  String? _error;
  String? _userName;

  static const _monthNames = [
    '', 'Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni',
    'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember',
  ];

  static const _monthShortNames = [
    '', 'Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun',
    'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des',
  ];

  String _formatPeriod() {
    if (_isAllPeriods) return 'all';
    return '${_activeDate.year}-${_activeDate.month.toString().padLeft(2, '0')}';
  }

  String _formatMonthYear(DateTime dt) {
    return '${_monthNames[dt.month]} ${dt.year}';
  }

  Future<void> _fetch() async {
    setState(() {
      _isLoading = true;
      _error = null;
    });

    try {
      final api = ref.read(rwApiClientProvider);
      final periodStr = _formatPeriod();

      // Read-only RT officials (rt_inferred membership) may only read Beranda
      // (rw.dashboard.read). KAS/transaction endpoints require rw.kas.read and
      // are denied by the backend — mirror the web, which also renders Beranda
      // without transactions for these users.
      final user = ref.read(authRepositoryProvider)?.valueOrNull;
      final hasFeatureAccess =
          rwMembershipHasFeatureAccess(user, widget.rwId);

      final dashboardFuture = api.getDashboard(
        widget.rwId,
        period: periodStr,
      );

      Future<RwTransactionsResponse> fetchRecentTx() async {
        if (!hasFeatureAccess) {
          return const RwTransactionsResponse(
            data: [],
            pagination: RwPaginationInfo(
              page: 1,
              pageSize: 5,
              total: 0,
              totalPages: 0,
            ),
          );
        }
        try {
          return await api.getTransactions(
            rwId: widget.rwId,
            page: 1,
            pageSize: 5,
          );
        } catch (_) {
          return const RwTransactionsResponse(
            data: [],
            pagination: RwPaginationInfo(
              page: 1,
              pageSize: 5,
              total: 0,
              totalPages: 0,
            ),
          );
        }
      }

      Future<RwTransactionsResponse> fetchYearTx() async {
        if (!hasFeatureAccess) {
          return const RwTransactionsResponse(
            data: [],
            pagination: RwPaginationInfo(
              page: 1,
              pageSize: 100,
              total: 0,
              totalPages: 0,
            ),
          );
        }
        try {
          return await api.getTransactions(
            rwId: widget.rwId,
            startDate: '${_activeDate.year}-01-01',
            endDate: '${_activeDate.year}-12-31',
            pageSize: 100,
          );
        } catch (_) {
          return const RwTransactionsResponse(
            data: [],
            pagination: RwPaginationInfo(
              page: 1,
              pageSize: 100,
              total: 0,
              totalPages: 0,
            ),
          );
        }
      }

      final results = await Future.wait([
        dashboardFuture,
        fetchRecentTx(),
        fetchYearTx(),
      ]);

      if (mounted) {
        setState(() {
          _data = results[0] as RwDashboardSummary;
          _recentTransactions = (results[1] as RwTransactionsResponse).data;
          _yearTransactions = (results[2] as RwTransactionsResponse).data;
          _isLoading = false;
        });
      }
    } catch (_) {
      if (mounted) {
        setState(() {
          _isLoading = false;
          _error = 'Ringkasan RW gagal dimuat. Silakan coba kembali.';
        });
      }
    }
  }

  Future<void> _loadUser() async {
    try {
      final authService = ref.read(authServiceProvider);
      final user = await authService.getMe();
      if (mounted) {
        setState(() {
          _userName = user.name;
        });
      }
    } catch (_) {
      // ignore
    }
  }

  @override
  void initState() {
    super.initState();
    _loadUser();
    _fetch();
  }

  List<MonthlyFinancePoint> _buildMonthlyPoints() {
    return List.generate(12, (index) {
      final monthNum = index + 1;
      int inc = 0;
      int exp = 0;
      int incCount = 0;
      int expCount = 0;

      for (final tx in _yearTransactions) {
        if (tx.occurredAt.year == _activeDate.year && tx.occurredAt.month == monthNum) {
          final amt = (double.tryParse(tx.amount) ?? 0).toInt();
          if (tx.type == 'income') {
            inc += amt;
            incCount++;
          } else if (tx.type == 'expense') {
            exp += amt;
            expCount++;
          }
        }
      }

      return MonthlyFinancePoint(
        month: _monthShortNames[monthNum],
        income: inc,
        expense: exp,
        incomeCount: incCount,
        expenseCount: expCount,
      );
    });
  }

  Future<void> _pickMonth(BuildContext context) async {
    final pickedYear = _activeDate.year;
    final pickedMonth = _activeDate.month;

    await showModalBottomSheet<void>(
      context: context,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(AppRadius.base)),
      ),
      builder: (ctx) {
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
                'Pilih Bulan — $pickedYear',
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
                itemCount: 12,
                itemBuilder: (_, i) {
                  final m = i + 1;
                  final isSelected = !_isAllPeriods && m == pickedMonth;
                  return InkWell(
                    onTap: () {
                      Navigator.of(ctx).pop();
                      setState(() {
                        _activeDate = DateTime(pickedYear, m, 1);
                        _isAllPeriods = false;
                      });
                      _fetch();
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
                        _monthNames[m],
                        style: TextStyle(
                          fontSize: 12,
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
    );
  }

  @override
  Widget build(BuildContext context) {
    super.build(context);
    final name = _userName ?? '';
    final authState = ref.watch(authRepositoryProvider);
    final user = authState?.valueOrNull;
    final rwMemberships = (user?['rw_memberships'] as List?)
        ?.whereType<Map<String, dynamic>>()
        .map((e) => RWMembership.fromJson(e.cast<String, dynamic>()))
        .toList() ?? <RWMembership>[];
    final currentMembership = rwMemberships.where((m) => m.rwId == widget.rwId).firstOrNull;
    final rwNumberText = currentMembership?.rwNumber != null
        ? currentMembership!.rwNumber!.toString().padLeft(3, '0')
        : resolveRwNumber(widget.rwId, rwMemberships);
    final rwDisplayName = (currentMembership?.rwName?.isNotEmpty == true)
        ? currentMembership!.rwName!
        : (rwNumberText != null ? 'RW $rwNumberText' : 'RW');
    final isDark = Theme.of(context).brightness == Brightness.dark;

    final body = NestedScrollView(
        headerSliverBuilder: (ctx, innerBoxIsScrolled) => [
          SliverToBoxAdapter(
            child: Padding(
              padding: const EdgeInsets.all(20),
              child: Row(
                children: [
                  Container(
                    width: 32,
                    height: 32,
                    decoration: BoxDecoration(
                      color: AppColors.accent,
                      borderRadius: BorderRadius.circular(AppRadius.sm),
                    ),
                    child: const Icon(Icons.bar_chart, color: Colors.white, size: 18),
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          'Selamat datang, $name',
                          style: const TextStyle(
                            fontSize: 16,
                            fontWeight: FontWeight.w700,
                          ),
                        ),
                        const SizedBox(height: 2),
                        Text(
                          rwDisplayName,
                          style: TextStyle(
                            fontSize: 12,
                            color: AppColors.textMutedColor(context),
                          ),
                        ),
                      ],
                    ),
                  ),
                  Builder(builder: (context) {
                    final badgeStyle = RwBadgeStyle.accent(isDark: isDark);
                    return Container(
                      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                      decoration: BoxDecoration(
                        color: badgeStyle.background,
                        borderRadius: BorderRadius.circular(AppRadius.sm),
                        border: Border.all(color: badgeStyle.border),
                      ),
                      child: Text(
                        'Keuangan RW',
                        style: TextStyle(
                          fontSize: 11,
                          fontWeight: FontWeight.w700,
                          color: badgeStyle.text,
                        ),
                      ),
                    );
                  }),
                ],
              ),
            ),
          ),
        ],
        body: RefreshIndicator(
          onRefresh: _fetch,
          child: SingleChildScrollView(
            physics: const AlwaysScrollableScrollPhysics(),
            padding: const EdgeInsets.only(bottom: 32),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                _buildPeriodFilter(context),
                const SizedBox(height: 16),
                if (_error != null)
                  Padding(
                    padding: const EdgeInsets.symmetric(horizontal: 20),
                    child: Container(
                      padding: const EdgeInsets.all(12),
                      decoration: BoxDecoration(
                        color: AppColors.danger.withValues(alpha: 0.1),
                        borderRadius: BorderRadius.circular(AppRadius.sm),
                        border: Border.all(color: AppColors.danger.withValues(alpha: 0.3)),
                      ),
                      child: Row(
                        children: [
                          const Icon(Icons.error_outline, color: AppColors.danger, size: 20),
                          const SizedBox(width: 10),
                          Expanded(
                            child: Text(
                              _error ?? 'Ringkasan RW gagal dimuat. Silakan coba kembali.',
                              style: const TextStyle(color: AppColors.danger, fontSize: 12),
                            ),
                          ),
                          TextButton(
                            onPressed: _fetch,
                            child: const Text('Coba Lagi'),
                          ),
                        ],
                      ),
                    ),
                  ),
                // 1. Horizontal Summary Cards: TOTAL SALDO, TOTAL PEMASUKAN, TOTAL PENGELUARAN
                _buildMetricsRow(context),
                const SizedBox(height: 16),
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 20),
                  child: Card(
                    child: Padding(
                      padding: const EdgeInsets.all(12),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          const Text(
                            'Ringkasan RW',
                            style: TextStyle(
                              fontSize: 14,
                              fontWeight: FontWeight.w700,
                            ),
                          ),
                          Text(
                            'Pencatatan kas dan lokasi dana mandiri tingkat RW',
                            style: TextStyle(
                              fontSize: 11,
                              color: AppColors.textMutedColor(context),
                            ),
                          ),
                          const SizedBox(height: 12),
                          _buildRwStats(context),
                        ],
                      ),
                    ),
                  ),
                ),
                const SizedBox(height: 16),
                // 2. Ringkasan Keuangan Chart
                _buildFinancialChartCard(context),
                const SizedBox(height: 16),
                // 3. Transaksi Terbaru
                _buildRecentTransactionsCard(context),
              ],
            ),
          ),
        ),
      );

    if (widget.isTabView) {
      return body;
    }

    return Scaffold(
      appBar: AppBar(
        automaticallyImplyLeading: false,
        centerTitle: false,
        title: MenuAppBarTitle(title: 'Beranda RW', rw: rwNumberText),
      ),
      body: body,
    );
  }

  Widget _buildPeriodFilter(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final surfaceColor = isDark ? AppColors.darkSurface : AppColors.lightSurface;
    final infoStyle = RwBadgeStyle.info(isDark: isDark);
    final accentStyle = RwBadgeStyle.accent(isDark: isDark);

    return Container(
      color: surfaceColor,
      padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 8),
      child: Row(
        children: [
          Expanded(
            child: InkWell(
              onTap: () => _pickMonth(context),
              borderRadius: BorderRadius.circular(AppRadius.sm),
              child: Container(
                padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
                decoration: BoxDecoration(
                  color: infoStyle.background,
                  borderRadius: BorderRadius.circular(AppRadius.sm),
                  border: Border.all(color: infoStyle.border),
                ),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Flexible(
                      child: Text(
                        _isAllPeriods
                            ? 'Semua Periode'
                            : _formatMonthYear(_activeDate),
                        style: TextStyle(
                          fontSize: 12,
                          fontWeight: FontWeight.w600,
                          color: infoStyle.text,
                        ),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                    const SizedBox(width: 4),
                    Icon(Icons.arrow_drop_down, size: 18, color: infoStyle.text),
                  ],
                ),
              ),
            ),
          ),
          const SizedBox(width: 8),
          GestureDetector(
            onTap: () {
              setState(() => _isAllPeriods = !_isAllPeriods);
              _fetch();
            },
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
              decoration: BoxDecoration(
                color: _isAllPeriods
                    ? accentStyle.background
                    : (isDark ? AppColors.darkSurfaceHover : AppColors.lightSurfaceHover),
                borderRadius: BorderRadius.circular(AppRadius.sm),
                border: Border.all(
                  color: _isAllPeriods
                      ? accentStyle.border
                      : (isDark ? AppColors.darkBorder : AppColors.lightBorder),
                ),
              ),
              child: Text(
                'Semua Periode',
                style: TextStyle(
                  fontSize: 12,
                  fontWeight: FontWeight.w600,
                  color: _isAllPeriods ? accentStyle.text : (isDark ? AppColors.darkText : AppColors.lightText),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildMetricsRow(BuildContext context) {
    final openingBal = _data != null ? RupiahFormatter.parseMoneyString(_data!.totalOpeningBalance) : 0;
    final incomeBal = _data != null ? RupiahFormatter.parseMoneyString(_data!.totalIncome) : 0;
    final expenseBal = _data != null ? RupiahFormatter.parseMoneyString(_data!.totalExpense) : 0;
    final totalSaldo = openingBal + incomeBal - expenseBal;
    final netBal = incomeBal - expenseBal;

    final saldoStr = _data != null ? RupiahFormatter.format(totalSaldo) : 'Rp 0';
    final incomeStr = _data != null ? RupiahFormatter.format(incomeBal) : 'Rp 0';
    final expenseStr = _data != null ? RupiahFormatter.format(expenseBal) : 'Rp 0';
    final netStr = _data != null ? RupiahFormatter.format(netBal) : 'Rp 0';

    final periodLabel = _isAllPeriods ? 'Semua' : _monthShortNames[_activeDate.month];

    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 20),
      child: Column(
        children: [
          _buildSummaryCard(
            context,
            label: 'TOTAL SALDO',
            period: periodLabel,
            value: _isLoading && _data == null ? '...' : saldoStr,
            color: AppColors.accent,
            icon: Icons.account_balance_wallet_outlined,
          ),
          const SizedBox(height: 12),
          _buildSummaryCard(
            context,
            label: 'SALDO BERSIH',
            period: periodLabel,
            value: _isLoading && _data == null ? '...' : netStr,
            color: AppColors.info,
            icon: Icons.account_balance_outlined,
          ),
          const SizedBox(height: 12),
          _buildSummaryCard(
            context,
            label: 'TOTAL PEMASUKAN',
            period: periodLabel,
            value: _isLoading && _data == null ? '...' : incomeStr,
            color: AppColors.success,
            icon: Icons.arrow_downward_outlined,
          ),
          const SizedBox(height: 12),
          _buildSummaryCard(
            context,
            label: 'TOTAL PENGELUARAN',
            period: periodLabel,
            value: _isLoading && _data == null ? '...' : expenseStr,
            color: AppColors.danger,
            icon: Icons.arrow_upward_outlined,
          ),
        ],
      ),
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

  Widget _buildSummaryCard(
    BuildContext context, {
    required String label,
    required String period,
    required String value,
    required Color color,
    required IconData icon,
  }) {
    return InkWell(
      onTap: () => _showSummaryCardDetail(
        context: context,
        title: label,
        value: value,
        color: color,
        icon: icon,
        subtitle: 'Periode: $period',
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
                    label,
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
                  Text(
                    'Periode: $period',
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

  Widget _buildRwStats(BuildContext context) {
    if (_isLoading && _data == null) {
      return const Center(
        child: Padding(
          padding: EdgeInsets.symmetric(vertical: 24),
          child: CircularProgressIndicator(),
        ),
      );
    }

    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceAround,
      children: [
        Expanded(
          child: _buildStatItem(context, (_data?.totalRts ?? 0).toString(), 'Total RT', AppColors.accent),
        ),
        Expanded(
          child: _buildStatItem(context, (_data?.transactionCount ?? 0).toString(), 'Transaksi', AppColors.info,
              onTap: () => context.push('/rw/${widget.rwId}/keuangan?tab=kas')),
        ),
        Expanded(
          child: _buildStatItem(context, (_data?.totalLocations ?? 0).toString(), 'Lokasi Dana', AppColors.success,
              onTap: () => context.push('/rw/${widget.rwId}/keuangan?tab=lokasi')),
        ),
      ],
    );
  }

  Widget _buildStatItem(BuildContext context, String value, String label, Color color, {VoidCallback? onTap}) {
    final content = Column(
      children: [
        Text(
          value,
          style: TextStyle(
            fontSize: 22,
            fontWeight: FontWeight.w700,
            color: color,
          ),
        ),
        Text(
          label,
          style: TextStyle(
            fontSize: 11,
            color: AppColors.textMutedColor(context),
          ),
        ),
      ],
    );
    if (onTap != null) {
      return InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(AppRadius.sm),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
          child: content,
        ),
      );
    }
    return content;
  }

  Widget _buildFinancialChartCard(BuildContext context) {
    if (_isLoading && _data == null) {
      return const Padding(
        padding: EdgeInsets.symmetric(horizontal: 20),
        child: Card(
          child: SizedBox(
            height: 220,
            child: Center(child: CircularProgressIndicator()),
          ),
        ),
      );
    }

    final points = _buildMonthlyPoints();
    final chartSummary = DashboardSummary(
      saldoKas: (double.tryParse(_data?.totalBalance ?? '0') ?? 0).toInt(),
      pemasukanBulanIni: (double.tryParse(_data?.totalIncome ?? '0') ?? 0).toInt(),
      pengeluaranBulanIni: (double.tryParse(_data?.totalExpense ?? '0') ?? 0).toInt(),
      duesProgress: const DuesProgress(paid: 0, target: 0),
      monthlyData: points,
    );

    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 20),
      child: FinancialSummaryChart(
        summary: chartSummary,
        theme: Theme.of(context),
      ),
    );
  }

  Widget _buildRecentTransactionsCard(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 20),
      child: Card(
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  const Expanded(
                    child: Text(
                      'Transaksi Terbaru',
                      style: TextStyle(
                        fontSize: 14,
                        fontWeight: FontWeight.w700,
                      ),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                  if (_recentTransactions.isNotEmpty)
                    TextButton(
                      onPressed: () => context.push('/rw/${widget.rwId}/kas'),
                      style: TextButton.styleFrom(
                        padding: EdgeInsets.zero,
                        minimumSize: const Size(50, 30),
                        tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                      ),
                      child: const Text('Lihat Semua', style: TextStyle(fontSize: 12)),
                    ),
                ],
              ),
              const SizedBox(height: 12),
              if (_isLoading && _recentTransactions.isEmpty)
                const Padding(
                  padding: EdgeInsets.symmetric(vertical: 24),
                  child: Center(child: CircularProgressIndicator()),
                )
              else if (_recentTransactions.isEmpty)
                Padding(
                  padding: const EdgeInsets.symmetric(vertical: 20),
                  child: Center(
                    child: Column(
                      children: [
                        Icon(
                          Icons.receipt_long_outlined,
                          size: 36,
                          color: AppColors.textMutedColor(context),
                        ),
                        const SizedBox(height: 8),
                        Text(
                          'Belum ada transaksi di RW ini',
                          style: TextStyle(
                            color: AppColors.textMutedColor(context),
                            fontSize: 13,
                          ),
                        ),
                      ],
                    ),
                  ),
                )
              else
                ListView.separated(
                  shrinkWrap: true,
                  physics: const NeverScrollableScrollPhysics(),
                  itemCount: _recentTransactions.length,
                  separatorBuilder: (ctx, i) => const Divider(height: 16),
                  itemBuilder: (ctx, i) {
                    final tx = _recentTransactions[i];
                    final isIncome = tx.type == 'income';
                    final color = isIncome ? AppColors.success : AppColors.danger;
                    final amt = (double.tryParse(tx.amount) ?? 0).toInt();
                    final amtStr = (isIncome ? '+ ' : '- ') + RupiahFormatter.format(amt);
                    final desc = tx.description.isNotEmpty
                        ? tx.description
                        : (isIncome ? 'KAS Masuk' : 'KAS Keluar');
                    final dateStr = '${tx.occurredAt.day} ${_monthShortNames[tx.occurredAt.month]} ${tx.occurredAt.year}';
                    final locationStr = tx.locationName != null && tx.locationName!.isNotEmpty
                        ? ' • ${tx.locationName}'
                        : '';

                    return InkWell(
                      onTap: () => context.push('/rw/${widget.rwId}/kas'),
                      borderRadius: BorderRadius.circular(AppRadius.sm),
                      child: Padding(
                        padding: const EdgeInsets.symmetric(vertical: 4),
                        child: Row(
                          children: [
                            Container(
                              width: 36,
                              height: 36,
                              decoration: BoxDecoration(
                                color: color.withValues(alpha: 0.1),
                                shape: BoxShape.circle,
                              ),
                              child: Icon(
                                isIncome ? Icons.arrow_downward : Icons.arrow_upward,
                                color: color,
                                size: 18,
                              ),
                            ),
                            const SizedBox(width: 12),
                            Expanded(
                              child: Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Text(
                                    desc,
                                    style: const TextStyle(
                                      fontSize: 13,
                                      fontWeight: FontWeight.w600,
                                    ),
                                    maxLines: 1,
                                    overflow: TextOverflow.ellipsis,
                                  ),
                                  const SizedBox(height: 2),
                                  Text(
                                    '$dateStr$locationStr',
                                    style: TextStyle(
                                      fontSize: 11,
                                      color: AppColors.textMutedColor(context),
                                    ),
                                    maxLines: 1,
                                    overflow: TextOverflow.ellipsis,
                                  ),
                                ],
                              ),
                            ),
                            const SizedBox(width: 8),
                            ConstrainedBox(
                              constraints: const BoxConstraints(maxWidth: 85),
                              child: FittedBox(
                                fit: BoxFit.scaleDown,
                                alignment: Alignment.centerRight,
                                child: Text(
                                  amtStr,
                                  style: TextStyle(
                                    fontSize: 13,
                                    fontWeight: FontWeight.w700,
                                    color: color,
                                  ),
                                ),
                              ),
                            ),
                          ],
                        ),
                      ),
                    );
                  },
                ),
            ],
          ),
        ),
      ),
    );
  }
}
