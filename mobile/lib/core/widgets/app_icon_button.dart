import 'package:flutter/material.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';

/// Preset button sizes for [AppIconButton].
enum AppIconButtonSize {
  medium,
  small,
}

/// Standard reusable Icon Button for Social Finance Android.
///
/// Features:
/// - Consistent touch target padding (min 44x44 or 48x48)
/// - Predictable icon sizing and colors
/// - Built-in tooltip / semantics
/// - Loading state support
class AppIconButton extends StatelessWidget {
  final IconData icon;
  final VoidCallback? onPressed;
  final String? tooltip;
  final Color? color;
  final Color? backgroundColor;
  final AppIconButtonSize size;
  final bool isLoading;

  const AppIconButton({
    super.key,
    required this.icon,
    this.onPressed,
    this.tooltip,
    this.color,
    this.backgroundColor,
    this.size = AppIconButtonSize.medium,
    this.isLoading = false,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final defaultColor = color ?? (isDark ? AppColors.darkText : AppColors.lightText);
    final iconSize = size == AppIconButtonSize.small ? 18.0 : 22.0;

    Widget child = isLoading
        ? SizedBox(
            width: iconSize,
            height: iconSize,
            child: CircularProgressIndicator(
              strokeWidth: 2,
              valueColor: AlwaysStoppedAnimation<Color>(defaultColor),
            ),
          )
        : Icon(icon, size: iconSize, color: defaultColor);

    Widget button = Material(
      color: backgroundColor ?? Colors.transparent,
      borderRadius: BorderRadius.circular(AppRadius.base),
      child: InkWell(
        onTap: isLoading ? null : onPressed,
        borderRadius: BorderRadius.circular(AppRadius.base),
        child: Padding(
          padding: EdgeInsets.all(size == AppIconButtonSize.small ? 6.0 : 10.0),
          child: child,
        ),
      ),
    );

    if (tooltip != null) {
      button = Tooltip(message: tooltip!, child: button);
    }

    return button;
  }
}
