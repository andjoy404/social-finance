import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

/// Theme selection enum for Social Finance theme selector.
enum AppThemeMode {
  /// Follow the device's system theme.
  system,

  /// Always use light mode.
  light,

  /// Always use dark mode.
  dark,
}

/// Theme-mode provider that stores the user's current theme preference.
///
/// Starts with [AppThemeMode.system] in-memory. In production, you would
/// persist this value to shared preferences or a similar mechanism.
class ThemeNotifier extends StateNotifier<AppThemeMode> {
  ThemeNotifier() : super(AppThemeMode.dark);

  void setMode(AppThemeMode mode) {
    state = mode;
  }
}

final themeProvider = StateNotifierProvider<ThemeNotifier, AppThemeMode>((ref) {
  return ThemeNotifier();
});

/// Extension to convert [AppThemeMode] to Flutter's [ThemeMode].
extension AppThemeModeX on AppThemeMode {
  ThemeMode get themeMode {
    return switch (this) {
      AppThemeMode.system => ThemeMode.system,
      AppThemeMode.light => ThemeMode.light,
      AppThemeMode.dark => ThemeMode.dark,
    };
  }
}
