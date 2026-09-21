import 'package:flutter/material.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';

class AppTheme {
  AppTheme._();

  // Dark surface palette for intentional dark-mode contrast.
  static const _darkSurfaceLighter = Color(0xFF2A2F32);
  static const _darkSurfaceDarker = Color(0xFF22282B);
  static const _darkBorder = Color(0xFF454B4F);

  // Brand violet with soft luminous quality for dark mode accents.
  static const _darkBrandAccent = Color(0xFF9B8FD6);

  // Brand violet with deeper contrast for light mode.
  static const _lightBrandAccent = Color(0xFF7C5FCE);

  // ── Light theme ──────────────────────────────────────────────

  static ThemeData get lightTheme {
    return ThemeData(
      useMaterial3: true,
      colorSchemeSeed: AppColors.seed,
      brightness: Brightness.light,

      // App bar
      appBarTheme: const AppBarTheme(
        centerTitle: true,
        elevation: 0,
        scrolledUnderElevation: 1,
      ),

      // Cards: subtle border, no large shadows
      cardTheme: CardThemeData(
        elevation: 0,
        shape: RoundedRectangleBorder(
          side: const BorderSide(color: Color(0xFFE8ECED), width: 1),
          borderRadius: BorderRadius.circular(AppRadius.base),
        ),
        clipBehavior: Clip.antiAlias,
      ),

      // Primary/raised button
      elevatedButtonTheme: ElevatedButtonThemeData(
        style: ElevatedButton.styleFrom(
          backgroundColor: AppColors.seed,
          foregroundColor: Colors.white,
          elevation: 0,
          padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 16),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(AppRadius.base),
          ),
          textStyle: const TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
        ),
      ),

      // Text button
      textButtonTheme: TextButtonThemeData(
        style: TextButton.styleFrom(foregroundColor: AppColors.seed),
      ),

      // Input fields
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: const Color(0xFFF8F9FA),
        contentPadding: const EdgeInsets.symmetric(
          horizontal: 16,
          vertical: 14,
        ),
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(AppRadius.base),
          borderSide: const BorderSide(color: Color(0xFFE0E0E0)),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(AppRadius.base),
          borderSide: const BorderSide(color: Color(0xFFE0E0E0)),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(AppRadius.base),
          borderSide: const BorderSide(color: AppColors.seed, width: 2),
        ),
        errorBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(AppRadius.base),
          borderSide: const BorderSide(color: AppColors.expense),
        ),
        focusedErrorBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(AppRadius.base),
          borderSide: const BorderSide(color: AppColors.expense, width: 2),
        ),
      ),

      // Bottom navigation — violet brand accent.
      bottomNavigationBarTheme: const BottomNavigationBarThemeData(
        type: BottomNavigationBarType.fixed,
        selectedItemColor: _lightBrandAccent,
        unselectedItemColor: Color(0xFF9E9E9E),
        elevation: 8,
        selectedLabelStyle: TextStyle(fontSize: 12),
        unselectedLabelStyle: TextStyle(fontSize: 12),
      ),

      // Text theme — size/weight, color via colorScheme for dark-mode safety
      textTheme: const TextTheme(
        displayLarge: TextStyle(
          fontSize: 32,
          fontWeight: FontWeight.bold,
          color: Color(0xFF1B1D1E),
        ),
        displayMedium: TextStyle(
          fontSize: 28,
          fontWeight: FontWeight.bold,
          color: Color(0xFF1B1D1E),
        ),
        displaySmall: TextStyle(
          fontSize: 24,
          fontWeight: FontWeight.bold,
          color: Color(0xFF1B1D1E),
        ),
        headlineLarge: TextStyle(
          fontSize: 22,
          fontWeight: FontWeight.w700,
          color: Color(0xFF1B1D1E),
        ),
        headlineMedium: TextStyle(
          fontSize: 20,
          fontWeight: FontWeight.w600,
          color: Color(0xFF1B1D1E),
        ),
        headlineSmall: TextStyle(
          fontSize: 18,
          fontWeight: FontWeight.w600,
          color: Color(0xFF1B1D1E),
        ),
        titleLarge: TextStyle(
          fontSize: 16,
          fontWeight: FontWeight.w600,
          color: Color(0xFF2E3031),
        ),
        titleMedium: TextStyle(
          fontSize: 16,
          fontWeight: FontWeight.w500,
          color: Color(0xFF424242),
        ),
        titleSmall: TextStyle(
          fontSize: 14,
          fontWeight: FontWeight.w500,
          color: Color(0xFF4E4E4E),
        ),
        bodyLarge: TextStyle(
          fontSize: 16,
          fontWeight: FontWeight.normal,
          color: Color(0xFF3C4043),
        ),
        bodyMedium: TextStyle(
          fontSize: 14,
          fontWeight: FontWeight.normal,
          color: Color(0xFF4E5458),
        ),
        bodySmall: TextStyle(
          fontSize: 12,
          fontWeight: FontWeight.normal,
          color: Color(0xFF6E777D),
        ),
      ),

      // Icon theme
      iconTheme: const IconThemeData(color: Color(0xFF6B7280), size: 24),
      dividerTheme: const DividerThemeData(
        color: Color(0xFFE8ECED),
        thickness: 1,
      ),
    );
  }

  // ── Dark theme — enhanced ────────────────────────────────────

  static ThemeData get darkTheme {
    // Precompute ColorScheme from violet seed to avoid conflict.
    final darkColorScheme =
        ColorScheme.fromSeed(
          seedColor: AppColors.seed,
          brightness: Brightness.dark,
        ).copyWith(
          surfaceContainerHighest: _darkSurfaceLighter,
          // Soft luminous violet for dark mode accents.
          primary: _darkBrandAccent,
        );

    return ThemeData(
      useMaterial3: true,
      brightness: Brightness.dark,
      // Scaffold / card surfaces — neutral, not pure-black.
      scaffoldBackgroundColor: _darkSurfaceDarker,
      colorScheme: darkColorScheme,

      // App bar
      appBarTheme: const AppBarTheme(
        centerTitle: true,
        elevation: 0,
        scrolledUnderElevation: 1,
        backgroundColor: Color(0xFF1C2226),
      ),

      // Cards: slightly lighter surface with subtle border.
      cardTheme: CardThemeData(
        elevation: 0,
        color: _darkSurfaceDarker,
        shape: RoundedRectangleBorder(
          side: const BorderSide(color: _darkBorder, width: 1),
          borderRadius: BorderRadius.circular(AppRadius.base),
        ),
        clipBehavior: Clip.antiAlias,
      ),

      // Primary/raised button
      elevatedButtonTheme: ElevatedButtonThemeData(
        style: ElevatedButton.styleFrom(
          backgroundColor: AppColors.seed,
          foregroundColor: Colors.white,
          elevation: 0,
          padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 16),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(AppRadius.base),
          ),
          textStyle: const TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
        ),
      ),

      // Text button
      textButtonTheme: TextButtonThemeData(
        style: TextButton.styleFrom(foregroundColor: _darkBrandAccent),
      ),

      // Input fields
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: const Color(0xFF2A2F32),
        contentPadding: const EdgeInsets.symmetric(
          horizontal: 16,
          vertical: 14,
        ),
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(AppRadius.base),
          borderSide: const BorderSide(color: _darkBorder),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(AppRadius.base),
          borderSide: const BorderSide(color: _darkBorder),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(AppRadius.base),
          borderSide: const BorderSide(color: _darkBrandAccent, width: 2),
        ),
        errorBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(AppRadius.base),
          borderSide: const BorderSide(color: AppColors.expense),
        ),
        focusedErrorBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(AppRadius.base),
          borderSide: const BorderSide(color: AppColors.expense, width: 2),
        ),
      ),

      // Bottom navigation — soft luminous violet.
      bottomNavigationBarTheme: BottomNavigationBarThemeData(
        type: BottomNavigationBarType.fixed,
        selectedItemColor: _darkBrandAccent,
        unselectedItemColor: const Color(0xFF637078),
        elevation: 0,
        backgroundColor: const Color(0xFF1C2226),
        selectedLabelStyle: const TextStyle(fontSize: 12),
        unselectedLabelStyle: const TextStyle(fontSize: 12),
      ),

      // Text theme
      textTheme: const TextTheme(
        displayLarge: TextStyle(
          fontSize: 32,
          fontWeight: FontWeight.bold,
          color: Color(0xFFECF0F3),
        ),
        displayMedium: TextStyle(
          fontSize: 28,
          fontWeight: FontWeight.bold,
          color: Color(0xFFECF0F3),
        ),
        displaySmall: TextStyle(
          fontSize: 24,
          fontWeight: FontWeight.bold,
          color: Color(0xFFECF0F3),
        ),
        headlineLarge: TextStyle(
          fontSize: 22,
          fontWeight: FontWeight.w700,
          color: Color(0xFFECF0F3),
        ),
        headlineMedium: TextStyle(
          fontSize: 20,
          fontWeight: FontWeight.w600,
          color: Color(0xFFECF0F3),
        ),
        headlineSmall: TextStyle(
          fontSize: 18,
          fontWeight: FontWeight.w600,
          color: Color(0xFFECF0F3),
        ),
        titleLarge: TextStyle(
          fontSize: 16,
          fontWeight: FontWeight.w600,
          color: Color(0xFFD8DDE0),
        ),
        titleMedium: TextStyle(
          fontSize: 16,
          fontWeight: FontWeight.w500,
          color: Color(0xFFBAC1C7),
        ),
        titleSmall: TextStyle(
          fontSize: 14,
          fontWeight: FontWeight.w500,
          color: Color(0xFF9DA6AF),
        ),
        bodyLarge: TextStyle(
          fontSize: 16,
          fontWeight: FontWeight.normal,
          color: Color(0xFFC6CDD3),
        ),
        bodyMedium: TextStyle(
          fontSize: 14,
          fontWeight: FontWeight.normal,
          color: Color(0xFFAEB8C0),
        ),
        bodySmall: TextStyle(
          fontSize: 12,
          fontWeight: FontWeight.normal,
          color: Color(0xFF7D8790),
        ),
      ),

      // Icon theme
      iconTheme: const IconThemeData(color: Color(0xFF9DA6AF), size: 24),
      dividerTheme: const DividerThemeData(color: _darkBorder, thickness: 1),
    );
  }
}
