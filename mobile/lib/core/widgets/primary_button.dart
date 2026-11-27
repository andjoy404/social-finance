import 'package:flutter/material.dart';

import 'package:social_finance/core/widgets/app_button.dart';

/// Legacy adapter for [PrimaryButton].
///
/// Keeps backwards-compatibility while delegating styling and states
/// to the standardized [AppButton].
class PrimaryButton extends StatelessWidget {
  final String text;
  final VoidCallback? onPressed;
  final bool isLoading;
  final IconData? icon;

  const PrimaryButton({
    super.key,
    required this.text,
    this.onPressed,
    this.isLoading = false,
    this.icon,
  });

  @override
  Widget build(BuildContext context) {
    return AppButton.primary(
      text: text,
      onPressed: onPressed,
      isLoading: isLoading,
      icon: icon,
      size: AppButtonSize.medium,
    );
  }
}
