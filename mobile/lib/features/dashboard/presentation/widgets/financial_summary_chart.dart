import 'package:fl_chart/fl_chart.dart';
import 'package:flutter/material.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import '../../data/mock_dashboard_data.dart';

/// Y-axis label formatting in compact Indonesian million notation.
String _formatYAxis(double value) {
  final millions = value ~/ 1000000;
  if (millions == 0) return '0';
  return '$millions jt';
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

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(20),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('Ringkasan Keuangan', style: widget.theme.textTheme.titleMedium),
            const SizedBox(height: 20),
            SizedBox(
              height: 200,
              child: BarChart(
                BarChartData(
                  minY: 0,
                  maxY: _calculateMaxY(),
                  alignment: BarChartAlignment.spaceAround,
                  barTouchData: BarTouchData(
                    enabled: true,
                    touchTooltipData: BarTouchTooltipData(),
                    touchCallback: (event, barTouchResponse) {
                      setState(() {
                        if (event is FlPointerHoverEvent ||
                            event is FlPointerEnterEvent) {
                          _hoveredIndex =
                              barTouchResponse?.spot?.touchedBarGroupIndex;
                        } else if (event is FlPointerExitEvent) {
                          _hoveredIndex = null;
                        }
                      });
                    },
                    handleBuiltInTouches: false,
                  ),
                  titlesData: FlTitlesData(
                    bottomTitles: AxisTitles(
                      sideTitles: SideTitles(
                        showTitles: true,
                        getTitlesWidget: (value, meta) {
                          final index = value.toInt();
                          if (index < 0 ||
                              index >= widget.summary.monthlyData.length) {
                            return const SizedBox.shrink();
                          }
                          return Padding(
                            padding: const EdgeInsets.only(top: 8),
                            child: Text(
                              widget.summary.monthlyData[index].month,
                              style: widget.theme.textTheme.bodySmall?.copyWith(
                                fontSize: 11,
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
                            style: widget.theme.textTheme.bodySmall?.copyWith(
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
                        color: widget.theme.dividerColor.withValues(alpha: 0.15),
                        strokeWidth: 1,
                      );
                    },
                  ),
                  borderData: FlBorderData(show: false),
                  barGroups: _buildBarGroups(),
                ),
              ),
            ),
            const SizedBox(height: 16),
            _buildLegend(),
          ],
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
    // Round up to nearest clean million for a tidy Y-axis scale.
    final roundedUp = (max / 1000000).ceil() * 1000000;
    return roundedUp.toDouble();
  }

  double _calcInterval() {
    // Five equal steps across the Y-axis (clean million-level steps).
    return _calculateMaxY() / 5;
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
