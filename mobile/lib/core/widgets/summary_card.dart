import 'package:flutter/material.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';

/// Reusable metric / summary card.
///
/// Displays a title, a large value, and an optional icon or semantic color.
/// Uses Web semantic colors from AppColors.
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
    final isDark = theme.brightness == Brightness.dark;
    final textColor = accentColor ?? AppColors.accent;

    // Use Web semantic colors for card styling
    final cardBg = isDark ? AppColors.darkSurface : AppColors.lightSurface;
    final borderColor = isDark ? AppColors.darkBorder : AppColors.lightBorder;

    final border = (accentColor != null)
        ? Border.all(
            color: isDark
                ? accentColor!.withValues(alpha: 0.35)
                : accentColor!.withValues(alpha: 0.25),
            width: 1,
          )
        : Border.all(color: borderColor, width: 1);

    final shadows = (accentColor != null && isDark)
        ? [
            BoxShadow(
              color: accentColor!.withValues(alpha: 0.12),
              blurRadius: 8,
              spreadRadius: 0,
            ),
          ]
        : null;

    return DecoratedBox(
      decoration: BoxDecoration(
        color: cardBg,
        border: border,
        borderRadius: BorderRadius.circular(AppRadius.base),
        boxShadow: shadows,
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
