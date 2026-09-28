import 'package:flutter/material.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';

/// Standard card container for Social Finance.
/// Uses Web semantic colors from AppColors.
class AppCard extends StatelessWidget {
  final Widget child;
  final EdgeInsetsGeometry? padding;
  final bool useEmphasis;
  final Color? cardColor;

  const AppCard({
    super.key,
    required this.child,
    this.padding,
    this.useEmphasis = false,
    this.cardColor,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final defaultColor = isDark
        ? AppColors.darkSurface
        : AppColors.lightSurface;
    final borderColor = isDark ? AppColors.darkBorder : AppColors.lightBorder;

    return Card(
      color: cardColor ?? (useEmphasis ? defaultColor : null),
      elevation: 0,
      shape: useEmphasis
          ? RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(AppRadius.lg),
            )
          : RoundedRectangleBorder(
              side: BorderSide(color: borderColor, width: 1),
              borderRadius: BorderRadius.circular(AppRadius.base),
            ),
      clipBehavior: Clip.antiAlias,
      child: Padding(
        padding: padding ?? const EdgeInsets.all(16),
        child: child,
      ),
    );
  }
}
