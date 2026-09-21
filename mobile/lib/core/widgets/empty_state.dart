import 'package:flutter/material.dart';

/// Centered empty-state widget.
///
/// Displays an icon, title, optional description, and an optional
/// action button for placeholder screens (Kas, Iuran, Laporan).
class EmptyState extends StatelessWidget {
  final IconData icon;
  final String title;
  final String description;
  final String? actionText;
  final VoidCallback? onAction;

  const EmptyState({
    super.key,
    required this.icon,
    required this.title,
    required this.description,
    this.actionText,
    this.onAction,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(
          icon,
          size: 64,
          color: theme.colorScheme.primary.withValues(alpha: 0.35),
        ),
        const SizedBox(height: 16),
        Text(title, style: theme.textTheme.titleLarge),
        const SizedBox(height: 8),
        Text(
          description,
          textAlign: TextAlign.center,
          style: theme.textTheme.bodyMedium,
        ),
        if (actionText != null && onAction != null) ...[
          const SizedBox(height: 16),
          ElevatedButton(onPressed: onAction, child: Text(actionText!)),
        ],
      ],
    );
  }
}
