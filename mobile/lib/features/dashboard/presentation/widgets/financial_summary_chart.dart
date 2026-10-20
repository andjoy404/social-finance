import 'package:fl_chart/fl_chart.dart';
import 'package:flutter/material.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/utils/rupiah_formatter.dart';
import '../../data/mock_dashboard_data.dart';

/// Y-axis label formatting in compact Indonesian million notation.
///
/// Ensures unique labels (no duplicate "0" or "1 jt") and avoids showing
/// every value as a bare integer by using ribuan/juta with proper decimals.
String _formatYAxis(double value) {
  if (value == 0) return '0';
  final millions = value / 1000000;
  if (millions < 1) {
    final ribuan = (value / 1000).round();
    return '$ribuan ribu';
  }
  final isWhole = millions == millions.roundToDouble();
  if (isWhole) {
    return '${millions.toInt()} jt';
  }
  return '${millions.toStringAsFixed(1)} jt'.replaceAll('.', ',');
}

/// Bar chart widget showing monthly income vs expense comparison.
///
/// Consumes [DashboardSummary] data from provider rather than embedding
/// values directly in rendering code.
class FinancialSummaryChart extends StatefulWidget {
  final DashboardSummary summary;
  final ThemeData theme;

  const FinancialSummaryChart({
    super.key,
    required this.summary,
    required this.theme,
  });

  @override
  State<FinancialSummaryChart> createState() => _FinancialSummaryChartState();
}

class _FinancialSummaryChartState extends State<FinancialSummaryChart> {
  int? _hoveredIndex;

  void _onMonthTapped(int index) {
    if (index < 0 || index >= widget.summary.monthlyData.length) return;
    setState(() {
      _hoveredIndex = index;
    });
  }

  @override
  Widget build(BuildContext context) {
    final activeTooltipGroup = _hoveredIndex;
    final hasTooltipGroup = activeTooltipGroup != null &&
        activeTooltipGroup >= 0 &&
        activeTooltipGroup < widget.summary.monthlyData.length;

    return Card(
      child: GestureDetector(
        behavior: HitTestBehavior.translucent,
        onTap: () {
          setState(() {
            _hoveredIndex = null;
          });
        },
        child: Padding(
        padding: const EdgeInsets.all(20),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              'Ringkasan Keuangan',
              style: widget.theme.textTheme.titleMedium,
            ),
            const SizedBox(height: 20),
            SizedBox(
              height: 200,
              child: LayoutBuilder(
                builder: (context, constraints) {
                  return Stack(
                    clipBehavior: Clip.none,
                    children: [
                      BarChart(
                        BarChartData(
                          minY: 0,
                          maxY: _calculateMaxY(),
                          alignment: BarChartAlignment.spaceAround,
                          barTouchData: BarTouchData(
                            enabled: true,
                            handleBuiltInTouches: false,
                            touchCallback: (event, barTouchResponse) {
                              final spot = barTouchResponse?.spot;
                              final touchedIdx = spot?.touchedBarGroupIndex;
                              if (touchedIdx != null &&
                                  (event is FlTapDownEvent ||
                                      event is FlTapUpEvent)) {
                                _onMonthTapped(touchedIdx);
                              } else {
                                setState(() {
                                  if (event is FlPointerHoverEvent ||
                                      event is FlPointerEnterEvent) {
                                    _hoveredIndex = touchedIdx;
                                  } else if (event is FlPointerExitEvent) {
                                    _hoveredIndex = null;
                                  }
                                });
                              }
                            },
                          ),
                          titlesData: FlTitlesData(
                            bottomTitles: AxisTitles(
                              sideTitles: SideTitles(
                                showTitles: true,
                                getTitlesWidget: (value, meta) {
                                  final index = value.toInt();
                                  if (index < 0 ||
                                      index >=
                                          widget.summary.monthlyData.length) {
                                    return const SizedBox.shrink();
                                  }
                                  final monthLabel =
                                      widget.summary.monthlyData[index].month;
                                  return GestureDetector(
                                    behavior: HitTestBehavior.opaque,
                                    onTap: () => _onMonthTapped(index),
                                    child: Padding(
                                      padding: const EdgeInsets.only(top: 8),
                                      child: Text(
                                        monthLabel,
                                        style: widget.theme.textTheme.bodySmall
                                            ?.copyWith(
                                          fontSize: 11,
                                        ),
                                      ),
                                    ),
                                  );
                                },
                                reservedSize: 30,
                              ),
                            ),
                            leftTitles: AxisTitles(
                              sideTitles: SideTitles(
                                showTitles: true,
                                reservedSize: 38,
                                getTitlesWidget: (value, meta) {
                                  return Text(
                                    _formatYAxis(value),
                                    style: widget.theme.textTheme.bodySmall
                                        ?.copyWith(
                                      fontSize: 10,
                                    ),
                                  );
                                },
                              ),
                            ),
                            topTitles: AxisTitles(
                              sideTitles: SideTitles(showTitles: false),
                            ),
                            rightTitles: AxisTitles(
                              sideTitles: SideTitles(showTitles: false),
                            ),
                          ),
                          gridData: FlGridData(
                            show: true,
                            drawVerticalLine: false,
                            horizontalInterval: _calcInterval(),
                            getDrawingHorizontalLine: (value) {
                              return FlLine(
                                color: widget.theme.dividerColor.withValues(
                                  alpha: 0.15,
                                ),
                                strokeWidth: 1,
                              );
                            },
                          ),
                          borderData: FlBorderData(show: false),
                          barGroups: _buildBarGroups(),
                        ),
                      ),
                      if (hasTooltipGroup)
                        _buildFloatingBarTooltip(
                          context,
                          activeTooltipGroup,
                          constraints.maxWidth,
                        ),
                    ],
                  );
                },
              ),
            ),
            const SizedBox(height: 16),
            _buildLegend(),
          ],
        ),
      ),
    ),
  );
  }

  Widget _buildFloatingBarTooltip(
    BuildContext context,
    int groupIndex,
    double chartWidth,
  ) {
    final point = widget.summary.monthlyData[groupIndex];
    final count = widget.summary.monthlyData.length;
    const leftAxisWidth = 38.0;
    const plotHeight = 170.0; // 200 total - 30 bottomTitles
    const tooltipWidth = 160.0;
    const tooltipHeight = 64.0;

    final plotWidth = (chartWidth - leftAxisWidth).clamp(1.0, double.infinity);
    final groupCenterX =
        leftAxisWidth + plotWidth * (groupIndex + 0.5) / (count > 0 ? count : 1);

    final maxY = _calculateMaxY();
    final tallerBar = point.income > point.expense ? point.income : point.expense;
    final ratio = maxY > 0 ? (tallerBar / maxY).clamp(0.0, 1.0) : 0.0;
    final barTopY = plotHeight * (1.0 - ratio);

    final maxLeft =
        (chartWidth - tooltipWidth - 2.0).clamp(2.0, double.infinity);
    final clampedLeft = (groupCenterX - tooltipWidth / 2).clamp(2.0, maxLeft);
    final clampedTop =
        (barTopY - tooltipHeight - 6.0).clamp(0.0, plotHeight - tooltipHeight);

    final isDark = widget.theme.brightness == Brightness.dark;
    final masukFormatted = RupiahFormatter.format(point.income);
    final keluarFormatted = RupiahFormatter.format(point.expense);

    return Positioned(
      left: clampedLeft,
      top: clampedTop,
      width: tooltipWidth,
      child: IgnorePointer(
        child: Container(
          key: const Key('chart-bar-tooltip'),
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
          decoration: BoxDecoration(
            color: isDark
                ? const Color(0xF2181B1F)
                : const Color(0xF5FFFFFF),
            borderRadius: BorderRadius.circular(AppRadius.sm),
            border: Border.all(
              color: isDark ? AppColors.darkBorder : AppColors.lightBorder,
            ),
            boxShadow: [
              BoxShadow(
                color: Colors.black.withValues(alpha: 0.18),
                blurRadius: 8,
                offset: const Offset(0, 3),
              ),
            ],
          ),
          child: Text.rich(
            TextSpan(
              children: [
                TextSpan(
                  text: '${point.month}\n',
                  style: widget.theme.textTheme.bodySmall?.copyWith(
                    fontWeight: FontWeight.w700,
                    fontSize: 11,
                  ),
                ),
                TextSpan(
                  text: 'Masuk  $masukFormatted\n',
                  style: widget.theme.textTheme.bodySmall?.copyWith(
                    color: AppColors.income,
                    fontWeight: FontWeight.w600,
                    fontSize: 10.5,
                  ),
                ),
                TextSpan(
                  text: 'Keluar  $keluarFormatted',
                  style: widget.theme.textTheme.bodySmall?.copyWith(
                    color: AppColors.expense,
                    fontWeight: FontWeight.w600,
                    fontSize: 10.5,
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  List<BarChartGroupData> _buildBarGroups() {
    return List.generate(widget.summary.monthlyData.length, (index) {
      final point = widget.summary.monthlyData[index];
      final isHovered = _hoveredIndex == index;
      return BarChartGroupData(
        x: index,
        barRods: [
          BarChartRodData(
            toY: point.income.toDouble(),
            color: isHovered
                ? AppColors.seed.withValues(alpha: 0.30)
                : AppColors.income,
            width: 12,
            borderRadius: const BorderRadius.vertical(top: Radius.circular(4)),
            backDrawRodData: BackgroundBarChartRodData(
              show: true,
              toY: _calculateMaxY(),
              color: isHovered
                  ? AppColors.seed.withValues(alpha: 0.08)
                  : AppColors.income.withValues(alpha: 0.06),
            ),
          ),
          BarChartRodData(
            toY: point.expense.toDouble(),
            color: isHovered
                ? AppColors.seed.withValues(alpha: 0.30)
                : AppColors.expense,
            width: 12,
            borderRadius: const BorderRadius.vertical(top: Radius.circular(4)),
            backDrawRodData: BackgroundBarChartRodData(
              show: true,
              toY: _calculateMaxY(),
              color: isHovered
                  ? AppColors.seed.withValues(alpha: 0.08)
                  : AppColors.expense.withValues(alpha: 0.06),
            ),
          ),
        ],
        barsSpace: 4,
      );
    });
  }

  Widget _buildLegend() {
    return Row(
      mainAxisAlignment: MainAxisAlignment.center,
      children: [
        _LegendItem(color: AppColors.income, label: 'Pemasukan'),
        const SizedBox(width: 24),
        _LegendItem(color: AppColors.expense, label: 'Pengeluaran'),
      ],
    );
  }

  double _calculateMaxY() {
    double max = 0;
    for (final point in widget.summary.monthlyData) {
      if (point.income > max) max = point.income.toDouble();
      if (point.expense > max) max = point.expense.toDouble();
    }
    if (max <= 0) return 5000000.0;
    // Round up to nearest clean million for a tidy Y-axis scale.
    final roundedUp = (max / 1000000).ceil() * 1000000;
    return roundedUp <= 0 ? 5000000.0 : roundedUp.toDouble();
  }

  double _calcInterval() {
    // Five equal steps across the Y-axis, rounded to clean 100k steps so
    // labels stay unique (no duplicate "0" or "1 jt").
    final max = _calculateMaxY();
    final rawInterval = max / 5;
    final rounded = ((rawInterval / 100000).ceil() * 100000).toDouble();
    return rounded > 0 ? rounded : 1000000.0;
  }
}

class _LegendItem extends StatelessWidget {
  final Color color;
  final String label;

  const _LegendItem({required this.color, required this.label});

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Container(
          width: 12,
          height: 12,
          decoration: BoxDecoration(
            color: color,
            borderRadius: BorderRadius.circular(3),
          ),
        ),
        const SizedBox(width: 6),
        Text(label, style: Theme.of(context).textTheme.bodySmall),
      ],
    );
  }
}
