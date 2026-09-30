import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../theme/app_colors.dart';

/// Shows a floating, horizontally & vertically centered SnackBar notification
/// confirming the exit action, styled with the application's theme colors.
void showExitAppSnackBar(BuildContext context) {
  final theme = Theme.of(context);
  final isDark = theme.brightness == Brightness.dark;

  final bgColor = isDark
      ? const Color(0xFF1E1E22)
      : const Color(0xFF26262B);
  final borderColor = isDark
      ? AppColors.darkBorder
      : const Color(0xFF3E3E44);

  ScaffoldMessenger.of(context).removeCurrentSnackBar();
  ScaffoldMessenger.of(context).showSnackBar(
    SnackBar(
      behavior: SnackBarBehavior.floating,
      elevation: 6,
      margin: const EdgeInsets.symmetric(horizontal: 52, vertical: 20),
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: BorderSide(
          color: borderColor,
          width: 0.8,
        ),
      ),
      backgroundColor: bgColor,
      duration: const Duration(seconds: 2),
      content: const Center(
        heightFactor: 1.0,
        child: Text(
          'Tekan sekali lagi untuk keluar dari aplikasi',
          textAlign: TextAlign.center,
          style: TextStyle(
            color: Colors.white,
            fontSize: 13,
            fontWeight: FontWeight.w500,
            letterSpacing: 0.1,
            height: 1.3,
          ),
        ),
      ),
    ),
  );
}

/// A wrapper widget that provides consistent Android back button handling for
/// all main menu tabs.
///
/// Behavior:
/// 1. First back press: does NOT pop/close the screen, does not change routes.
///    Displays a centered floating SnackBar ("Tekan sekali lagi untuk keluar dari aplikasi").
/// 2. Second back press within 2 seconds: exits the app via [SystemNavigator.pop].
/// 3. Back press after > 2 seconds: treated as first back press again (re-shows SnackBar).
class DoubleBackExitScope extends StatefulWidget {
  final Widget child;

  const DoubleBackExitScope({super.key, required this.child});

  @override
  State<DoubleBackExitScope> createState() => _DoubleBackExitScopeState();
}

class _DoubleBackExitScopeState extends State<DoubleBackExitScope> {
  DateTime? _lastBackPressTime;

  @override
  Widget build(BuildContext context) {
    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, result) {
        if (didPop) return;
        final now = DateTime.now();
        if (_lastBackPressTime == null ||
            now.difference(_lastBackPressTime!) > const Duration(seconds: 2)) {
          _lastBackPressTime = now;
          showExitAppSnackBar(context);
        } else {
          SystemNavigator.pop();
        }
      },
      child: widget.child,
    );
  }
}
