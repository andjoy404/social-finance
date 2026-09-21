import 'package:flutter/material.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/utils/rupiah_formatter.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import '../../data/mock_dashboard_data.dart';

/// Reusable transaction row widget.
///
/// Shows description, category, amount, and +/- indicator.
/// Income and expense are distinguished by icon, sign, and label —
/// not just color.
class TransactionRow extends StatelessWidget {
  final Transaction transaction;
  final ThemeData theme;

  const TransactionRow({
    super.key,
    required this.transaction,
    required this.theme,
  });

  @override
  Widget build(BuildContext context) {
    final isIncome = transaction.isIncome;
    final accentColor = isIncome ? AppColors.income : AppColors.expense;
    final icon = isIncome ? Icons.arrow_downward : Icons.arrow_upward;

    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: AppCard(
        padding: const EdgeInsets.all(14),
        child: Row(
          children: [
            // Icon circle
            Container(
              padding: const EdgeInsets.all(8),
              decoration: BoxDecoration(
                color: accentColor.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(AppRadius.sm),
              ),
              child: Icon(icon, size: 18, color: accentColor),
            ),
            const SizedBox(width: 12),
            // Description + category
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    transaction.description,
                    style: theme.textTheme.bodyMedium,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                  const SizedBox(height: 2),
                  Text(
                    '${transaction.date}  •  ${transaction.category}',
                    style: theme.textTheme.bodySmall,
                  ),
                ],
              ),
            ),
            // Amount + status text
            Column(
              crossAxisAlignment: CrossAxisAlignment.end,
              children: [
                Text(
                  '${isIncome ? "+" : "-"}${RupiahFormatter.format(transaction.amount)}',
                  style: theme.textTheme.titleSmall?.copyWith(
                    color: accentColor,
                    fontWeight: FontWeight.w600,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  isIncome ? 'masuk' : 'keluar',
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: accentColor,
                    fontSize: 10,
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
