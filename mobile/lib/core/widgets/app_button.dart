import 'package:flutter/material.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';

/// Button variants supported by [AppButton].
enum AppButtonVariant {
  /// Primary action (Simpan, Buat, Tambah, Kirim, Konfirmasi).
  /// Solid background: [AppColors.success] (green), text: white.
  primary,

  /// Danger / destructive / dismissive action (Hapus, Keluarkan, Batal, Tutup).
  /// Solid background: [AppColors.danger] (red), text: white.
  danger,

  /// Outline danger action (Batal on full-screen forms & destructive cancel).
  /// Standardized to solid background [AppColors.danger], text: white.
  dangerOutline,

  /// Secondary neutral action (Kembali, Lihat detail, netral).
  /// Muted surface background, subtle border, primary text.
  secondary,

  /// Neutral outline action (secondary with border, transparent bg).
  outline,

  /// Accent / Brand primary action (Violet - [AppColors.accent]).
  accent,

  /// Accent outline action.
  accentOutline,

  /// Soft-tinted accent action with clear visible border (e.g. Kamera / Galeri picker).
  /// Soft background: [AppColors.accentSoftColor], clear accent border, readable text & accent icon.
  accentSoft,
}

/// Button size presets for [AppButton].
enum AppButtonSize {
  /// Standard form action size (48px height, suitable for bottomNavigationBar).
  form,

  /// Standard compact button (40px height, suitable for dialogs & cards).
  medium,

  /// Small compact button (32px-36px height, suitable for dense lists & tables).
  small,
}

/// Standard reusable button for Social Finance Android.
///
/// Implements:
/// - Semantic coloring from [AppColors] (Success green for primary, Danger red for delete/cancel)
/// - Predictable sizing (48px for forms, 40px for dialogs, 34px for dense contexts)
/// - Loading state with spinner and prevention of double-submission
/// - Disabled state styling
/// - Semantic accessibility labels
class AppButton extends StatelessWidget {
  final String text;
  final VoidCallback? onPressed;
  final AppButtonVariant variant;
  final AppButtonSize size;
  final bool isLoading;
  final IconData? icon;
  final bool fullWidth;
  final double? width;
  final double? height;
  final String? semanticLabel;
  final Key? buttonKey;

  const AppButton({
    super.key,
    required this.text,
    this.onPressed,
    this.variant = AppButtonVariant.primary,
    this.size = AppButtonSize.medium,
    this.isLoading = false,
    this.icon,
    this.fullWidth = false,
    this.width,
    this.height,
    this.semanticLabel,
    this.buttonKey,
  });

  /// Factory for primary action (Simpan, Buat, Tambah, Kirim)
  const AppButton.primary({
    super.key,
    required this.text,
    this.onPressed,
    this.size = AppButtonSize.medium,
    this.isLoading = false,
    this.icon,
    this.fullWidth = false,
    this.width,
    this.height,
    this.semanticLabel,
    this.buttonKey,
  }) : variant = AppButtonVariant.primary;

  /// Factory for destructive action (Hapus, Keluarkan)
  const AppButton.danger({
    super.key,
    required this.text,
    this.onPressed,
    this.size = AppButtonSize.medium,
    this.isLoading = false,
    this.icon,
    this.fullWidth = false,
    this.width,
    this.height,
    this.semanticLabel,
    this.buttonKey,
  }) : variant = AppButtonVariant.danger;

  /// Factory for form Batal button (Outlined red)
  const AppButton.dangerOutline({
    super.key,
    required this.text,
    this.onPressed,
    this.size = AppButtonSize.medium,
    this.isLoading = false,
    this.icon,
    this.fullWidth = false,
    this.width,
    this.height,
    this.semanticLabel,
    this.buttonKey,
  }) : variant = AppButtonVariant.dangerOutline;

  /// Factory for secondary neutral action (Kembali, Lihat detail)
  const AppButton.secondary({
    super.key,
    required this.text,
    this.onPressed,
    this.size = AppButtonSize.medium,
    this.isLoading = false,
    this.icon,
    this.fullWidth = false,
    this.width,
    this.height,
    this.semanticLabel,
    this.buttonKey,
  }) : variant = AppButtonVariant.secondary;

  /// Factory for neutral outline action
  const AppButton.outline({
    super.key,
    required this.text,
    this.onPressed,
    this.size = AppButtonSize.medium,
    this.isLoading = false,
    this.icon,
    this.fullWidth = false,
    this.width,
    this.height,
    this.semanticLabel,
    this.buttonKey,
  }) : variant = AppButtonVariant.outline;

  /// Factory for brand accent action (Violet)
  const AppButton.accent({
    super.key,
    required this.text,
    this.onPressed,
    this.size = AppButtonSize.medium,
    this.isLoading = false,
    this.icon,
    this.fullWidth = false,
    this.width,
    this.height,
    this.semanticLabel,
    this.buttonKey,
  }) : variant = AppButtonVariant.accent;

  /// Factory for soft-tinted accent action with clear visible border
  /// (e.g. Kamera & Galeri media picker actions).
  const AppButton.accentSoft({
    super.key,
    required this.text,
    this.onPressed,
    this.size = AppButtonSize.medium,
    this.isLoading = false,
    this.icon,
    this.fullWidth = false,
    this.width,
    this.height,
    this.semanticLabel,
    this.buttonKey,
  }) : variant = AppButtonVariant.accentSoft;

  double get _targetHeight {
    if (height != null) return height!;
    switch (size) {
      case AppButtonSize.form:
        return 48.0;
      case AppButtonSize.medium:
        return 40.0;
      case AppButtonSize.small:
        return 34.0;
    }
  }

  double get _fontSize {
    switch (size) {
      case AppButtonSize.form:
        return 15.0;
      case AppButtonSize.medium:
        return 14.0;
      case AppButtonSize.small:
        return 12.5;
    }
  }

  double get _iconSize {
    switch (size) {
      case AppButtonSize.form:
        return 20.0;
      case AppButtonSize.medium:
        return 18.0;
      case AppButtonSize.small:
        return 16.0;
    }
  }

  EdgeInsetsGeometry get _padding {
    switch (size) {
      case AppButtonSize.form:
        return const EdgeInsets.symmetric(horizontal: 16, vertical: 0);
      case AppButtonSize.medium:
        return const EdgeInsets.symmetric(horizontal: 14, vertical: 0);
      case AppButtonSize.small:
        return const EdgeInsets.symmetric(horizontal: 10, vertical: 0);
    }
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final isEnabled = onPressed != null && !isLoading;
    final effectiveOnPressed = isEnabled ? onPressed : null;

    final targetH = _targetHeight;
    final radius = BorderRadius.circular(AppRadius.base);

    Widget buttonWidget;

    switch (variant) {
      case AppButtonVariant.primary:
        buttonWidget = ElevatedButton(
          key: buttonKey,
          onPressed: effectiveOnPressed,
          style: ElevatedButton.styleFrom(
            backgroundColor: AppColors.success,
            foregroundColor: Colors.white,
            disabledBackgroundColor: isDark
                ? AppColors.success.withValues(alpha: 0.3)
                : AppColors.success.withValues(alpha: 0.4),
            disabledForegroundColor: Colors.white60,
            elevation: 0,
            padding: _padding,
            minimumSize: Size(width ?? (fullWidth ? double.infinity : 0), targetH),
            fixedSize: Size(width ?? (fullWidth ? double.infinity : double.nan), targetH),
            visualDensity: VisualDensity.compact,
            shape: RoundedRectangleBorder(borderRadius: radius),
            textStyle: TextStyle(
              fontSize: _fontSize,
              fontWeight: FontWeight.w600,
            ),
          ),
          child: _buildChild(Colors.white),
        );
        break;

      case AppButtonVariant.danger:
        buttonWidget = ElevatedButton(
          key: buttonKey,
          onPressed: effectiveOnPressed,
          style: ElevatedButton.styleFrom(
            backgroundColor: AppColors.danger,
            foregroundColor: Colors.white,
            disabledBackgroundColor: isDark
                ? AppColors.danger.withValues(alpha: 0.3)
                : AppColors.danger.withValues(alpha: 0.4),
            disabledForegroundColor: Colors.white60,
            elevation: 0,
            padding: _padding,
            minimumSize: Size(width ?? (fullWidth ? double.infinity : 0), targetH),
            fixedSize: Size(width ?? (fullWidth ? double.infinity : double.nan), targetH),
            visualDensity: VisualDensity.compact,
            shape: RoundedRectangleBorder(borderRadius: radius),
            textStyle: TextStyle(
              fontSize: _fontSize,
              fontWeight: FontWeight.w600,
            ),
          ),
          child: _buildChild(Colors.white),
        );
        break;

      case AppButtonVariant.dangerOutline:
        // Standardized: Batal is solid red (filled/blocking) across the entire application.
        buttonWidget = ElevatedButton(
          key: buttonKey,
          onPressed: effectiveOnPressed,
          style: ElevatedButton.styleFrom(
            backgroundColor: AppColors.danger,
            foregroundColor: Colors.white,
            disabledBackgroundColor: isDark
                ? AppColors.danger.withValues(alpha: 0.3)
                : AppColors.danger.withValues(alpha: 0.4),
            disabledForegroundColor: Colors.white60,
            elevation: 0,
            padding: _padding,
            minimumSize: Size(width ?? (fullWidth ? double.infinity : 0), targetH),
            fixedSize: Size(width ?? (fullWidth ? double.infinity : double.nan), targetH),
            visualDensity: VisualDensity.compact,
            shape: RoundedRectangleBorder(borderRadius: radius),
            textStyle: TextStyle(
              fontSize: _fontSize,
              fontWeight: FontWeight.w600,
            ),
          ),
          child: _buildChild(Colors.white),
        );
        break;

      case AppButtonVariant.secondary:
        final bg = isDark ? AppColors.darkSurface : AppColors.lightSurface;
        final fg = isDark ? AppColors.darkText : AppColors.lightText;
        final border = isDark ? AppColors.darkBorder : AppColors.lightBorder;

        buttonWidget = OutlinedButton(
          key: buttonKey,
          onPressed: effectiveOnPressed,
          style: OutlinedButton.styleFrom(
            backgroundColor: bg,
            foregroundColor: fg,
            disabledForegroundColor: fg.withValues(alpha: 0.4),
            side: BorderSide(color: border, width: 1.0),
            elevation: 0,
            padding: _padding,
            minimumSize: Size(width ?? (fullWidth ? double.infinity : 0), targetH),
            fixedSize: Size(width ?? (fullWidth ? double.infinity : double.nan), targetH),
            visualDensity: VisualDensity.compact,
            shape: RoundedRectangleBorder(borderRadius: radius),
            textStyle: TextStyle(
              fontSize: _fontSize,
              fontWeight: FontWeight.w500,
            ),
          ),
          child: _buildChild(fg),
        );
        break;

      case AppButtonVariant.outline:
        final fg = isDark ? AppColors.darkText : AppColors.lightText;
        final border = isDark ? AppColors.darkBorder : AppColors.lightBorder;

        buttonWidget = OutlinedButton(
          key: buttonKey,
          onPressed: effectiveOnPressed,
          style: OutlinedButton.styleFrom(
            foregroundColor: fg,
            disabledForegroundColor: fg.withValues(alpha: 0.4),
            side: BorderSide(color: border, width: 1.0),
            elevation: 0,
            padding: _padding,
            minimumSize: Size(width ?? (fullWidth ? double.infinity : 0), targetH),
            fixedSize: Size(width ?? (fullWidth ? double.infinity : double.nan), targetH),
            visualDensity: VisualDensity.compact,
            shape: RoundedRectangleBorder(borderRadius: radius),
            textStyle: TextStyle(
              fontSize: _fontSize,
              fontWeight: FontWeight.w500,
            ),
          ),
          child: _buildChild(fg),
        );
        break;

      case AppButtonVariant.accent:
        buttonWidget = ElevatedButton(
          key: buttonKey,
          onPressed: effectiveOnPressed,
          style: ElevatedButton.styleFrom(
            backgroundColor: AppColors.accent,
            foregroundColor: Colors.white,
            disabledBackgroundColor: isDark
                ? AppColors.accent.withValues(alpha: 0.3)
                : AppColors.accent.withValues(alpha: 0.4),
            disabledForegroundColor: Colors.white60,
            elevation: 0,
            padding: _padding,
            minimumSize: Size(width ?? (fullWidth ? double.infinity : 0), targetH),
            fixedSize: Size(width ?? (fullWidth ? double.infinity : double.nan), targetH),
            visualDensity: VisualDensity.compact,
            shape: RoundedRectangleBorder(borderRadius: radius),
            textStyle: TextStyle(
              fontSize: _fontSize,
              fontWeight: FontWeight.w600,
            ),
          ),
          child: _buildChild(Colors.white),
        );
        break;

      case AppButtonVariant.accentOutline:
        buttonWidget = OutlinedButton(
          key: buttonKey,
          onPressed: effectiveOnPressed,
          style: OutlinedButton.styleFrom(
            foregroundColor: AppColors.accent,
            disabledForegroundColor: AppColors.accent.withValues(alpha: 0.4),
            side: const BorderSide(color: AppColors.accent, width: 1.5),
            elevation: 0,
            padding: _padding,
            minimumSize: Size(width ?? (fullWidth ? double.infinity : 0), targetH),
            fixedSize: Size(width ?? (fullWidth ? double.infinity : double.nan), targetH),
            visualDensity: VisualDensity.compact,
            shape: RoundedRectangleBorder(borderRadius: radius),
            textStyle: TextStyle(
              fontSize: _fontSize,
              fontWeight: FontWeight.w600,
            ),
          ),
          child: _buildChild(AppColors.accent),
        );
        break;

      case AppButtonVariant.accentSoft:
        final bg = AppColors.accentSoftColor(context);
        final fg = isDark ? AppColors.darkText : AppColors.lightText;
        final borderColor = isEnabled
            ? AppColors.accent.withValues(alpha: isDark ? 0.55 : 0.45)
            : AppColors.accent.withValues(alpha: 0.2);
        final iconTint = isEnabled
            ? AppColors.accent
            : AppColors.accent.withValues(alpha: 0.4);

        buttonWidget = OutlinedButton(
          key: buttonKey,
          onPressed: effectiveOnPressed,
          style: OutlinedButton.styleFrom(
            backgroundColor: bg,
            disabledBackgroundColor: bg.withValues(alpha: 0.5),
            foregroundColor: fg,
            disabledForegroundColor: fg.withValues(alpha: 0.4),
            side: BorderSide(color: borderColor, width: 1.25),
            elevation: 0,
            padding: EdgeInsets.symmetric(
              horizontal: size == AppButtonSize.small
                  ? AppSpacing.sm
                  : AppSpacing.md,
              vertical: 0,
            ),
            minimumSize: Size(width ?? (fullWidth ? double.infinity : 0), targetH),
            fixedSize: Size(width ?? (fullWidth ? double.infinity : double.nan), targetH),
            visualDensity: VisualDensity.compact,
            shape: RoundedRectangleBorder(borderRadius: radius),
            textStyle: TextStyle(
              fontSize: _fontSize,
              fontWeight: FontWeight.w600,
            ),
          ),
          child: _buildChild(fg, iconColor: iconTint),
        );
        break;
    }

    if (semanticLabel != null) {
      buttonWidget = Semantics(
        label: semanticLabel,
        button: true,
        enabled: isEnabled,
        child: buttonWidget,
      );
    }

    if (fullWidth) {
      return SizedBox(
        width: double.infinity,
        height: targetH,
        child: buttonWidget,
      );
    }

    if (width != null) {
      return SizedBox(
        width: width,
        height: targetH,
        child: buttonWidget,
      );
    }

    return buttonWidget;
  }

  Widget _buildChild(Color defaultForeground, {Color? iconColor}) {
    if (isLoading) {
      final spinnerSize = size == AppButtonSize.small ? 14.0 : 18.0;
      return SizedBox(
        width: spinnerSize,
        height: spinnerSize,
        child: CircularProgressIndicator(
          strokeWidth: 2,
          valueColor: AlwaysStoppedAnimation<Color>(
            variant == AppButtonVariant.dangerOutline ||
                    variant == AppButtonVariant.outline ||
                    variant == AppButtonVariant.secondary
                ? defaultForeground
                : variant == AppButtonVariant.accentSoft
                    ? AppColors.accent
                    : Colors.white,
          ),
        ),
      );
    }

    if (icon != null) {
      return Row(
        mainAxisSize: MainAxisSize.min,
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(icon, size: _iconSize, color: iconColor),
          const SizedBox(width: AppSpacing.sm),
          Flexible(
            child: Text(
              text,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
          ),
        ],
      );
    }

    return Text(
      text,
      maxLines: 1,
      overflow: TextOverflow.ellipsis,
    );
  }
}
