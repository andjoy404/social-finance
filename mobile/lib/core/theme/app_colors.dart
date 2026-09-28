import 'package:flutter/material.dart';

/// Semantic color tokens for Social Finance.
///
/// **SOURCE OF TRUTH:** Web /frontend/src/styles/tokens.css
///
/// These tokens are extracted from the Web design system and MUST match
/// Web colors exactly. All Android screens must use these tokens.
///
/// Do NOT create new colors. Do NOT deviate from Web values.
/// If a color doesn't match Web, it's a bug.
class AppColors {
  AppColors._();

  // ─────────────────────────────────────────────────────────────────────
  // SEMANTIC COLORS — Same across light & dark themes
  // Source: tokens.css (--sf-accent, --sf-success, --sf-warning, etc.)
  // ─────────────────────────────────────────────────────────────────────

  /// Primary brand accent — Violet
  /// Web: --sf-accent (#a970ff)
  /// Used for: primary buttons, links, active states, focus rings
  static const Color accent = Color(0xFFA970FF);

  /// Semantic success/income — Green
  /// Web: --sf-success (#73bf69)
  /// Used for: success messages, income, completed states
  static const Color success = Color(0xFF73BF69);

  /// Semantic warning — Orange
  /// Web: --sf-warning (#ff9830)
  /// Used for: warnings, alerts, low balance
  static const Color warning = Color(0xFFFF9830);

  /// Semantic danger/error/expense — Red
  /// Web: --sf-danger (#f2495c)
  /// Used for: errors, expenses, invalid states, delete actions
  static const Color danger = Color(0xFFF2495C);

  /// Semantic info — Blue
  /// Web: --sf-info (#5794f2)
  /// Used for: information, notifications
  static const Color info = Color(0xFF5794F2);

  // ─────────────────────────────────────────────────────────────────────
  // LIGHT MODE COLORS
  // Source: tokens.css [data-theme='light']
  // ─────────────────────────────────────────────────────────────────────

  /// Light mode: Primary background
  /// Web: --sf-bg (#f6f6f7)
  static const Color lightBg = Color(0xFFF6F6F7);

  /// Light mode: Surface (cards, panels)
  /// Web: --sf-surface (#ffffff)
  static const Color lightSurface = Color(0xFFFFFFFF);

  /// Light mode: Surface hover state
  /// Web: --sf-surface-hover (#eaeaea)
  static const Color lightSurfaceHover = Color(0xFFEAEAEA);

  /// Light mode: Primary text
  /// Web: --sf-text (#1a1a1a)
  static const Color lightText = Color(0xFF1A1A1A);

  /// Light mode: Muted/secondary text
  /// Web: --sf-text-muted (#6b6b6b)
  static const Color lightTextMuted = Color(0xFF6B6B6B);

  /// Light mode: Borders and dividers
  /// Web: --sf-border (#dedede)
  static const Color lightBorder = Color(0xFFDEDEDE);

  /// Light mode: Subtle background
  /// Web: --sf-bg-subtle (#eaeded)
  static const Color lightBgSubtle = Color(0xFFEAEDED);

  /// Light mode: Subtle surface
  /// Web: --sf-surface-subtle (#f2f2f4)
  static const Color lightSurfaceSubtle = Color(0xFFF2F2F4);

  /// Light mode: Subtle border
  /// Web: --sf-border-subtle (#e9e9e9)
  static const Color lightBorderSubtle = Color(0xFFE9E9E9);

  /// Light mode: Accent soft (tinted background)
  /// Web: rgba(169, 112, 255, 0.10) on light bg = ~#f6f6f7 tinted
  /// Approximate: #ede8f9 (10% violet on #f6f6f7)
  static const Color lightAccentSoft = Color(0xFFEDE8F9);

  /// Light mode: Success soft tint
  /// Web: rgba(115, 191, 105, 0.10)
  static const Color lightSuccessSoft = Color(0xFFF1F7ED);

  /// Light mode: Warning soft tint
  /// Web: rgba(255, 152, 48, 0.10)
  static const Color lightWarningSoft = Color(0xFFFFF5E6);

  /// Light mode: Danger soft tint
  /// Web: rgba(242, 73, 92, 0.10)
  static const Color lightDangerSoft = Color(0xFFFEF0F2);

  /// Light mode: Info soft tint
  /// Web: rgba(87, 148, 242, 0.10)
  static const Color lightInfoSoft = Color(0xFFEEF3FB);

  // ─────────────────────────────────────────────────────────────────────
  // DARK MODE COLORS
  // Source: tokens.css [data-theme='dark']
  // ─────────────────────────────────────────────────────────────────────

  /// Dark mode: Primary background
  /// Web: --sf-bg (#0d0d0d)
  static const Color darkBg = Color(0xFF0D0D0D);

  /// Dark mode: Surface (cards, panels)
  /// Web: --sf-surface (#1a1a1a)
  static const Color darkSurface = Color(0xFF1A1A1A);

  /// Dark mode: Surface hover state
  /// Web: --sf-surface-hover (#222222)
  static const Color darkSurfaceHover = Color(0xFF222222);

  /// Dark mode: Primary text
  /// Web: --sf-text (#e6e6e6)
  static const Color darkText = Color(0xFFE6E6E6);

  /// Dark mode: Muted/secondary text
  /// Web: --sf-text-muted (#888888)
  static const Color darkTextMuted = Color(0xFF888888);

  /// Dark mode: Borders and dividers
  /// Web: --sf-border (#2a2a2a)
  static const Color darkBorder = Color(0xFF2A2A2A);

  /// Dark mode: Subtle background
  /// Web: --sf-bg-subtle (#141414)
  static const Color darkBgSubtle = Color(0xFF141414);

  /// Dark mode: Subtle surface
  /// Web: --sf-surface-subtle (#121212)
  static const Color darkSurfaceSubtle = Color(0xFF121212);

  /// Dark mode: Subtle border
  /// Web: --sf-border-subtle (#1f1f1f)
  static const Color darkBorderSubtle = Color(0xFF1F1F1F);

  /// Dark mode: Accent soft (tinted background)
  /// Web: rgba(169, 112, 255, 0.12) on dark bg = ~#1a1a1a tinted
  /// Approximate: #241f32 (12% violet on #1a1a1a)
  static const Color darkAccentSoft = Color(0xFF241F32);

  /// Dark mode: Success soft tint
  /// Web: rgba(115, 191, 105, 0.15)
  static const Color darkSuccessSoft = Color(0xFF1D261A);

  /// Dark mode: Warning soft tint
  /// Web: rgba(255, 152, 48, 0.15)
  static const Color darkWarningSoft = Color(0xFF2A2218);

  /// Dark mode: Danger soft tint
  /// Web: rgba(242, 73, 92, 0.15)
  static const Color darkDangerSoft = Color(0xFF291B1E);

  /// Dark mode: Info soft tint
  /// Web: rgba(87, 148, 242, 0.15)
  static const Color darkInfoSoft = Color(0xFF1B2335);

  // ─────────────────────────────────────────────────────────────────────
  // NEUTRAL BADGE COLORS — Soft gray for Sosial / Anggota jabatan
  // ─────────────────────────────────────────────────────────────────────

  /// Light mode: Neutral badge text color
  static const Color lightBadgeNeutral = Color(0xFF6B7280);

  /// Dark mode: Neutral badge text color
  static const Color darkBadgeNeutral = Color(0xFFC4C9CE);

  /// Light mode: Neutral badge background
  static const Color lightBadgeNeutralBg = Color(0xFFECEDEE);

  /// Light mode: Neutral badge border
  static const Color lightBadgeNeutralBorder = Color(0xFF969BA5);

  /// Dark mode: Neutral badge background
  static const Color darkBadgeNeutralBg = Color(0xFF3A3F44);

  /// Dark mode: Neutral badge border
  static const Color darkBadgeNeutralBorder = Color(0xFF737A82);

  // ─────────────────────────────────────────────────────────────────────
  // LOGIN PAGE SPECIFIC COLORS
  // These are intentionally preserved — Login page design is fixed
  // ─────────────────────────────────────────────────────────────────────

  /// Login page background (dark)
  static const Color loginBgDark = Color(0xFF07060B);

  /// Login page secondary dark
  static const Color loginBgDarker = Color(0xFF0A0910);

  /// Login page tertiary dark
  static const Color loginBgDarkest = Color(0xFF0E0C15);

  /// Login card gradient top
  static const Color loginCardGradientTop = Color(0xE015111F);

  /// Login card gradient bottom
  static const Color loginCardGradientBottom = Color(0xF50D0A14);

  /// Login card border accent (violet)
  static const Color loginCardBorder = Color(0x1AA970FF);

  /// Login accent button/focus
  static const Color loginAccent = Color(0xFFA970FF);

  /// Login accent secondary
  static const Color loginAccentSecondary = Color(0xFF7C5AC7);

  /// Login text primary (light)
  static const Color loginTextPrimary = Color(0xFFF5F3FB);

  /// Login text secondary (muted)
  static const Color loginTextMuted = Color(0xFF9B93AD);

  /// Login label text
  static const Color loginLabel = Color(0xFFCFC9DE);

  /// Login input text
  static const Color loginInputText = Color(0xFFF2EFF9);

  /// Login input placeholder
  static const Color loginInputPlaceholder = Color(0xFF7A7292);

  /// Login input icon
  static const Color loginInputIcon = Color(0xFF8D82AB);

  /// Login input background
  static const Color loginInputBg = Color(0x0F09070F);

  /// Login input border
  static const Color loginInputBorder = Color(0x38A970FF);

  /// Login error background
  static const Color loginErrorBg = Color(0xFFE8453C);

  /// Login error border
  static const Color loginErrorBorder = Color(0x80F2495C);

  /// Login error text
  static const Color loginErrorText = Color(0xFFFF98A6);

  /// Login button disabled state
  static const Color loginButtonDisabled = Color(0x387C5AC7);

  /// Login button disabled text
  static const Color loginButtonDisabledText = Color(0x73C8C8EB);

  /// Login button text
  static const Color loginButtonText = Color(0xFFF8F5FD);

  // ─────────────────────────────────────────────────────────────────────
  // LEGACY ALIASES (mapped to Web ACTUAL colors)
  // Old code used these names; they now point to correct Web colors
  // ─────────────────────────────────────────────────────────────────────

  /// Legacy: seed → now maps to Web accent
  static const Color seed = accent;

  /// Legacy: income → now maps to Web success
  static const Color income = success;

  /// Legacy: expense → now maps to Web danger
  static const Color expense = danger;

  /// Legacy: hoverTint → now maps to accent with transparency
  static const Color hoverTint = Color(0x1AA970FF);

  /// Legacy: dashboard colors → now maps to Web accent
  static const Color dashboardSaldoKas = accent;
  static const Color dashboardSaldoKasDark = accent;
  static const Color dashboardIncome = success;
  static const Color dashboardExpense = danger;

  /// Legacy: warga colors → now maps to accent/success
  static const Color wargaHeadViolet = accent;
  static const Color wargaFamilyBlue = success;
  static const Color wargaRelativeIndigo = accent;
  static const Color wargaOwner = accent;
  static const Color wargaTenant = success;

  /// Legacy: lightTextPrimary → now maps to Web light text
  static const Color lightTextPrimary = lightText;

  /// Legacy: lightTextSecondary → now maps to Web light text muted
  static const Color lightTextSecondary = lightTextMuted;

  /// Legacy: darkTextPrimary → now maps to Web dark text
  static const Color darkTextPrimary = darkText;

  /// Legacy: darkSurfaceLighter → now maps to Web dark surface
  static const Color darkSurfaceLighter = darkSurface;

  /// Legacy: seedDark → now maps to Web accent
  static const Color seedDark = accent;

  /// Legacy: seedDeep → now maps to Web accent
  static const Color seedDeep = accent;

  // ─────────────────────────────────────────────────────────────────────

  /// Get text color based on theme brightness
  static Color textColor(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    return isDark ? darkText : lightText;
  }

  /// Get muted text color based on theme brightness
  static Color textMutedColor(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    return isDark ? darkTextMuted : lightTextMuted;
  }

  /// Get surface color based on theme brightness
  static Color surfaceColor(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    return isDark ? darkSurface : lightSurface;
  }

  /// Get background color based on theme brightness
  static Color bgColor(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    return isDark ? darkBg : lightBg;
  }

  /// Get border color based on theme brightness
  static Color borderColor(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    return isDark ? darkBorder : lightBorder;
  }

  /// Get accent soft tint based on theme brightness
  static Color accentSoftColor(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    return isDark ? darkAccentSoft : lightAccentSoft;
  }

  /// Legacy method: iconTint (now uses Web text muted)
  static Color iconTint(BuildContext context) {
    return textMutedColor(context);
  }

  /// Legacy method: occupancyColor (now uses Web accent for owner, success for tenant)
  static Color occupancyColor({required bool isOwner, required bool isDark}) {
    return isOwner ? accent : success;
  }
}
