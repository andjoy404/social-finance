import 'package:flutter/material.dart';

/// Semantic color tokens for Social Finance.
///
/// These named colors are intended for use directly in widgets via [Theme]
/// so that both light and dark themes remain consistent.
class AppColors {
  AppColors._();

  // Primary brand accent — refined violet for financial application.
  static const Color seed = Color(0xFF7C5FCE);

  // Income / success — semantic green, NOT brand color.
  static const Color income = Color(0xFF2E7D32);

  // Expense / error — semantic red, NOT brand color.
  static const Color expense = Color(0xFFC62828);

  // Warning (e.g. low balance, overdue) — amber.
  static const Color warning = Color(0xFFF57F17);

  // Information — blue.
  static const Color info = Color(0xFF1565C0);

  /// Helper method to derive a theme-aware icon tint.
  static Color iconTint(BuildContext context) {
    return Theme.of(context).colorScheme.onSurfaceVariant;
  }
}
