import 'package:flutter/material.dart';

/// Reusable section header used on Dashboard / Beranda.
class SectionHeader extends StatelessWidget {
  final String title;
  final Widget? trailing;

  const SectionHeader({super.key, required this.title, this.trailing});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Text(title, style: theme.textTheme.titleMedium),
        if (trailing != null) ...[const SizedBox(width: 8), trailing!],
      ],
    );
  }
}
