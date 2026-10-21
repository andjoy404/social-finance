import 'package:flutter/material.dart';

import '../theme/app_colors.dart';

/// Shows a smooth popup overlay that previews the app logo in full size.
///
/// The popup is centered, ~80% of screen width, with fade + scale animation.
/// Closes on backdrop tap, Back button, or close icon.
void showLogoPreview(BuildContext context) {
  showGeneralDialog(
    context: context,
    barrierColor: Colors.black.withValues(alpha: 0.75),
    barrierDismissible: true,
    barrierLabel: 'Close logo preview',
    transitionDuration: const Duration(milliseconds: 300),
    pageBuilder: (buildContext, animation, secondaryAnimation) {
      return Center(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(24.0),
          child: FadeTransition(
            opacity: animation,
            child: ScaleTransition(
              scale: animation,
              child: Container(
                decoration: BoxDecoration(
                  color: AppColors.lightSurface,
                  borderRadius: BorderRadius.circular(24),
                  boxShadow: [
                    BoxShadow(
                      color: Colors.black.withValues(alpha: 0.3),
                      blurRadius: 32,
                      offset: const Offset(0, 8),
                    ),
                  ],
                ),
                constraints: const BoxConstraints(
                  maxWidth: 500,
                  maxHeight: 500,
                ),
                child: Stack(
                  children: [
                    Padding(
                      padding: const EdgeInsets.all(32),
                      child: Image.asset(
                        'assets/images/logo.png',
                        fit: BoxFit.contain,
                        errorBuilder: (context, error, stackTrace) => Icon(
                          Icons.account_balance_wallet,
                          size: 120,
                          color: AppColors.accent,
                        ),
                      ),
                    ),
                    Positioned(
                      top: 8,
                      right: 8,
                      child: IconButton(
                        onPressed: () => Navigator.pop(buildContext),
                        icon: const Icon(
                          Icons.close,
                          color: AppColors.lightTextMuted,
                        ),
                        tooltip: 'Tutup',
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      );
    },
  );
}
