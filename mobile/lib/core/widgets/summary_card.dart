import 'package:flutter/material.dart';

import 'package:social_finance/core/theme/app_radius.dart';

/// Reusable metric / summary card.
///
/// Displays a title, a large value, and an optional icon or
/// semantic accent color for quick financial scanning.
class SummaryCard extends StatelessWidget {
  final String title;
  final String value;
  final IconData? icon;
  final Color? accentColor;
  final TextStyle? titleStyle;
  final TextStyle? valueStyle;

  const SummaryCard({
    super.key,
    required this.title,
    required this.value,
    this.icon,
    this.accentColor,
    this.titleStyle,
    this.valueStyle,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final textColor = accentColor ?? theme.colorScheme.primary;

    return DecoratedBox(
      decoration: BoxDecoration(
        color:
            theme.cardTheme.color ??
            (theme.brightness == Brightness.dark
                ? const Color(0xFF22282B)
                : Colors.white),
        border: Border.all(
          color: theme.cardTheme.shape is RoundedRectangleBorder
              ? (theme.cardTheme.shape as RoundedRectangleBorder).side.color
              : const Color(0xFFE8ECED),
        ),
        borderRadius: BorderRadius.circular(AppRadius.base),
      ),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                if (icon != null) ...[
                  Icon(icon, size: 20, color: textColor),
                  const SizedBox(width: 8),
                ],
                Flexible(
                  child: Text(
                    title,
                    style: titleStyle ?? theme.textTheme.bodySmall,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 8),
            Text(
              value,
              style:
                  valueStyle ??
                  theme.textTheme.titleLarge?.copyWith(
                    color: textColor,
                    fontWeight: FontWeight.bold,
                  ),
            ),
          ],
        ),
      ),
    );
  }
}
