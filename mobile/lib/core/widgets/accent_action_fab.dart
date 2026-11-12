import 'package:flutter/material.dart';

import '../theme/app_colors.dart';

/// Pill-shaped accent action button — the Lokasi Dana style used by
/// "Tambah …" actions across the app.
///
/// Visual Language:
///   * soft accent fill (`AppColors.accentSoftColor`)
///   * 1px accent border, 24dp radius, subtle accent glow
///   * `add` icon + a context icon (18–20dp), accent tint
///   * bottom-right, Material InkWell ripple
class AccentActionFab extends StatelessWidget {
  const AccentActionFab({
    super.key,
    required this.tooltip,
    required this.icon,
    required this.onPressed,
  });

  final String tooltip;
  final IconData icon;
  final VoidCallback onPressed;

  @override
  Widget build(BuildContext context) {
    final accentSoft = AppColors.accentSoftColor(context);
    return Tooltip(
      message: tooltip,
      child: Container(
        decoration: BoxDecoration(
          color: accentSoft,
          borderRadius: BorderRadius.circular(24),
          border: Border.all(color: AppColors.accent, width: 1),
          boxShadow: [
            BoxShadow(
              color: AppColors.accent.withValues(alpha: 0.15),
              blurRadius: 8,
              offset: const Offset(0, 0),
            ),
          ],
        ),
        child: Material(
          color: Colors.transparent,
          clipBehavior: Clip.none,
          child: InkWell(
            onTap: onPressed,
            borderRadius: BorderRadius.circular(24),
            child: Padding(
              padding: const EdgeInsets.symmetric(
                horizontal: 16,
                vertical: 10,
              ),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const Icon(
                    Icons.add,
                    size: 20,
                    color: AppColors.accent,
                  ),
                  const SizedBox(width: 6),
                  Icon(
                    icon,
                    size: 18,
                    color: AppColors.accent,
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
