# Social Finance - UI Design System

## Design Principles

- **Modern & Clean** — Minimal, professional financial application
- **Trustworthy** — Restrained color palette, clear financial hierarchy
- **Approachable** — Generous spacing, accessible touch targets
- **Community-focused** — Designed for non-technical neighborhood residents
- **No visual noise** — No excessive gradients, glassmorphism, gaming-style UI, or crypto/trading app appearance

## Colors (PROVISIONAL)

> This color palette is temporary until a formal brand review.
> The primary color can be replaced globally in `lib/app/theme/app_theme.dart`.

```dart
static const _primaryColor = Color(0xFF2E7D6F); // Teal-green — trust, community
// Material 3 derives secondary, surface, onPrimary shades automatically
```

### Derived Colors

Derived from the primary via Material 3's `ColorScheme.fromSeed()`:
- **Primary:** Teal-green (`#2E7D6F`)
- **Secondary:** Warm amber (`#F4A261`)
- **Success:** Green (`#4CAF50`)
- **Error:** Red (`#E53935`)
- **Background:** White / light gray (`#F8F8F8`)
- **Surface:** White (`#FFFFFF`)

## Typography

Material 3 text theme with Indonesian language priority:

| Style | Size | Weight | Color | Use |
|-------|------|--------|-------|-----|
| `headlineLarge` | 22 | w700 | black87 | Page titles |
| `headlineMedium` | 20 | w600 | black87 | Section headers |
| `headlineSmall` | 18 | w600 | black87 | Card titles |
| `titleLarge` | 16 | w600 | black87 | Transaction titles |
| `titleMedium` | 16 | w500 | dark gray | Menu items |
| `titleSmall` | 14 | w500 | medium gray | Labels |
| `bodyLarge` | 16 | normal | dark gray | Body text |
| `bodyMedium` | 14 | normal | medium gray | Secondary text |
| `bodySmall` | 12 | normal | light gray | Hints, subtitles |

### Minimum Font Size

All readable text is at least **12sp**. Interactive text is at least **14sp**.

## Spacing

Standard Flutter spacing tokens (8px grid):
- `8` — Tight grouping (between related elements)
- `12` — Component separation
- `16` — Section gaps
- `20` — Screen edge padding
- `24` — Card internal padding
- `32` — Major section separation

## Radius

- `12` — Standard cards, buttons, text fields
- `8` — Badge, icon container
- `6` — Tight containers, chips
- `20` — Pill labels (role badges)
- `16` — Card with primary background (saldo kas)

## Components

### PrimaryButton
Full-width elevated button. Blue primary color, rounded corners (r=12).
Loading state shows circular progress indicator.

### AppCard
Elevation-0 card with subtle border (`#F0F0F0`). Padding defaults to 16.

### AppTextField
Filled text field with rounded corners. Support for label, hint, validation, suffix icon.

### EmptyState
Centered icon + title + description + optional action button.

### ErrorState
Error icon + message + optional retry button.

### LoadingIndicator
Centered circular progress + optional message.

## Navigation

### Bottom Navigation
5 tabs with Material icons:

| Tab | Label | Icon (inactive) | Icon (active) | Route |
|-----|-------|-----------------|---------------|-------|
| 1 | Beranda | `home_outlined` | `home` | `/home` |
| 2 | Kas | `account_balance_wallet_outlined` | `account_balance_wallet` | `/home/cash` |
| 3 | Iuran | `receipt_long_outlined` | `receipt_long` | `/home/dues` |
| 4 | Laporan | `bar_chart_outlined` | `bar_chart` | `/home/reports` |
| 5 | Profil | `person_outline` | `person` | `/home/profile` |

State is preserved — navigating between tabs maintains each tab's scroll position.

## Indonesian Terminology

| English | Bahasa Indonesia |
|---------|-----------------|
| Home | Beranda |
| Cash | Kas |
| Dues | Iuran |
| Reports | Laporan |
| Profile | Profil |
| Enter | Masuk |
| Welcome | Selamat datang |
| Balance | Saldo |
| Income | Pemasukan |
| Expense | Pengeluaran |
| Current month | Bulan Ini |
| Last transactions | Transaksi Terbaru |
| Password | Kata Sandi |
| Email | Email |
| Logout | Keluar |
| Try Again | Coba Lagi |
| Loading | Memproses... |
| Error | Error / Terjadi kesalahan |

## Accessibility Guidelines

- **Readable contrast:** All text meets WCAG AA minimum (4.5:1 for body, 3:1 for large text)
- **Touch targets:** Minimum 48x48px for all interactive elements
- **Labels:** All form fields have text labels visible above the input
- **Color independence:** Status is communicated via icons and text, not just color
- **System fonts:** Material fonts are universally readable
- **Screen reader:** All icons have semantic meaning (Flutter's `Semantics` is leveraged by Material components)

## Dark Mode

Dark theme is NOT implemented in this phase. The theme structure (via `ThemeData`) supports
easy addition of a dark theme later via `ThemeMode.dark` in the `MaterialApp`.
