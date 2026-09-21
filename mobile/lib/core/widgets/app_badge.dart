import 'package:flutter/material.dart';

import 'package:social_finance/core/theme/app_radius.dart';

/// Compact status badge / pill.
///
/// Used for role display (Bendahara, Warga, etc.), payment status, etc.
/// When [isNeonStyle] is true (dark mode), renders a soft violet-tinted
/// badge with a thin violet border and subtle outer glow inspired by
/// the AndJoy visual language.
class AppBadge extends StatelessWidget {
  final String label;
  final Color? backgroundColor;
  final Color? textColor;
  final bool isNeonStyle;

  const AppBadge({
    super.key,
    required this.label,
    this.backgroundColor,
    this.textColor,
    this.isNeonStyle = false,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    if (isNeonStyle) {
      return _NeonVioletBadge(
        label: label,
        textColor: textColor ?? const Color(0xFFD4CCF5),
      );
    }

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        color:
            backgroundColor ??
            theme.colorScheme.primary.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(AppRadius.sm),
        border: Border.all(
          color:
              backgroundColor ??
              theme.colorScheme.primary.withValues(alpha: 0.25),
          width: 0.5,
        ),
      ),
      child: Text(
        label,
        style: theme.textTheme.bodySmall?.copyWith(
          color: textColor ?? theme.colorScheme.primary,
          fontWeight: FontWeight.w600,
          fontSize: 11,
        ),
      ),
    );
  }
}

/// Soft violet neon badge for dark-mode surface.
///
/// Restrained — no cyberpunk glow, no animated bloom.
class _NeonVioletBadge extends StatelessWidget {
  final String label;
  final Color textColor;

  const _NeonVioletBadge({required this.label, required this.textColor});

  @override
  Widget build(BuildContext context) {
    return AnimatedContainer(
      duration: const Duration(milliseconds: 200),
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 5),
      decoration: BoxDecoration(
        color: const Color(0x3A2E1F4D),
        borderRadius: BorderRadius.circular(AppRadius.sm),
        border: Border.all(color: const Color(0xFF9B8FD6), width: 1),
        boxShadow: [
          BoxShadow(
            color: const Color(0xFF9B8FD6).withValues(alpha: 0.15),
            blurRadius: 8,
            offset: const Offset(0, 0),
          ),
        ],
      ),
      child: Text(
        label,
        style: TextStyle(
          color: textColor,
          fontWeight: FontWeight.w600,
          fontSize: 11,
          letterSpacing: 0.2,
        ),
      ),
    );
  }
}
