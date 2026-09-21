import 'package:flutter/material.dart';

/// Compact spacing scale used throughout Social Finance.
///
/// Values follow a 4px baseline grid.  Prefer using the named
/// constants rather than arbitrary pixel values.
class AppSpacing {
  AppSpacing._();

  static const double xxs = 2;
  static const double xs = 4;
  static const double sm = 8;
  static const double md = 12;
  static const double base = 16;
  static const double lg = 20;
  static const double xl = 24;
  static const double xxl = 32;
  static const double xxxl = 40;

  static const EdgeInsets screenPadding = EdgeInsets.symmetric(horizontal: 20);

  static const EdgeInsets sectionPadding = EdgeInsets.symmetric(
    horizontal: 20,
    vertical: 8,
  );
}
