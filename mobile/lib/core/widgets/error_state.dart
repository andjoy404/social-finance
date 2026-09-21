import 'package:flutter/material.dart';

class ErrorState extends StatelessWidget {
  final String message;
  final String? actionText;
  final VoidCallback? onRetry;

  const ErrorState({
    super.key,
    required this.message,
    this.actionText = 'Coba Lagi',
    this.onRetry,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.error_outline, size: 48, color: theme.colorScheme.error),
            const SizedBox(height: 16),
            Text(
              message,
              textAlign: TextAlign.center,
              style: theme.textTheme.bodyMedium,
            ),
            if (onRetry != null) ...[
              const SizedBox(height: 16),
              ElevatedButton(onPressed: onRetry, child: Text(actionText!)),
            ],
          ],
        ),
      ),
    );
  }
}
