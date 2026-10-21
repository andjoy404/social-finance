import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_spacing.dart';

/// Segmented tab bar placed directly below the "Iuran" AppBar title for Pengurus/Bendahara.
class IuranTabBar extends StatelessWidget implements PreferredSizeWidget {
  final int selectedIndex; // 0: Iuran Saya, 1: Iuran Warga, 2: Kelola Iuran

  const IuranTabBar({super.key, required this.selectedIndex});

  @override
  Size get preferredSize => const Size.fromHeight(44);

  @override
  Widget build(BuildContext context) {
    return Container(
      height: 44,
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.base, vertical: 4),
      alignment: Alignment.centerLeft,
      child: SingleChildScrollView(
        scrollDirection: Axis.horizontal,
        child: Row(
          children: [
            _IuranRouteChip(
              label: 'Iuran Saya',
              icon: Icons.pending_actions_outlined,
              selected: selectedIndex == 0,
              onTap: () {
                if (selectedIndex != 0) context.go('/home/iuran');
              },
            ),
            const SizedBox(width: AppSpacing.xs),
            _IuranRouteChip(
              label: 'Iuran Warga',
              icon: Icons.receipt_long_outlined,
              selected: selectedIndex == 1,
              onTap: () {
                if (selectedIndex != 1) context.go('/home/iuran/warga');
              },
            ),
            const SizedBox(width: AppSpacing.xs),
            _IuranRouteChip(
              label: 'Kelola Iuran',
              icon: Icons.settings_outlined,
              selected: selectedIndex == 2,
              onTap: () {
                if (selectedIndex != 2) context.go('/home/iuran/kelola');
              },
            ),
          ],
        ),
      ),
    );
  }
}

class _IuranRouteChip extends StatelessWidget {
  final String label;
  final IconData icon;
  final bool selected;
  final VoidCallback onTap;

  const _IuranRouteChip({
    required this.label,
    required this.icon,
    required this.selected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    if (selected) {
      return Container(
        height: 32,
        decoration: BoxDecoration(
          color: AppColors.accent.withValues(alpha: isDark ? 0.25 : 0.15),
          borderRadius: BorderRadius.circular(16),
          border: Border.all(color: AppColors.accent, width: 1),
        ),
        child: Material(
          color: Colors.transparent,
          child: InkWell(
            onTap: onTap,
            borderRadius: BorderRadius.circular(16),
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 12),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(icon, size: 16, color: AppColors.accent),
                  const SizedBox(width: 6),
                  Text(
                    label,
                    style: const TextStyle(
                      fontSize: 12,
                      fontWeight: FontWeight.w600,
                      color: AppColors.accent,
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      );
    }

    return Container(
      height: 32,
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkSurface : AppColors.lightSurface,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(
          color: isDark ? AppColors.darkBorder : AppColors.lightBorder,
          width: 1,
        ),
      ),
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: onTap,
          borderRadius: BorderRadius.circular(16),
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 12),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(
                  icon,
                  size: 16,
                  color: isDark ? AppColors.darkTextMuted : AppColors.lightTextMuted,
                ),
                const SizedBox(width: 6),
                Text(
                  label,
                  style: TextStyle(
                    fontSize: 12,
                    fontWeight: FontWeight.w500,
                    color: isDark ? AppColors.darkTextMuted : AppColors.lightTextMuted,
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
