# Android Warga Module Inspection Report

**Purpose:** Audit of the Android (Flutter) Warga module to identify UI patterns that will be reused for the Iuran and KAS modules. The Iuran/KAS modules must visually match Warga exactly. No new design patterns should be created.

**Date:** 2026-10-02

---

## 1. Executive Summary

The Warga module is the most feature-complete Android feature in the Social Finance application. It demonstrates the following **reusable UI patterns**:

1. **List page architecture** — `WargaScreen` uses `ConsumerStatefulWidget` with Riverpod, client-side search via `StateProvider`, `ListView.separated`, summary count, loading/error/empty/list states, and a conditional FAB.
2. **Form page architecture** — `HouseholdCreateScreen`, `HouseholdEditScreen`, `SpecialResidentCreateScreen`, and `SpecialResidentEditScreen` all share identical layouts: `PopScope` + `Scaffold` + `AppBar(title)` + `ListView(padding: base)` + `Form` with vertically stacked fields, error display, validation error list, and horizontal Simpan/Batal buttons.
3. **Shared widget library** — 12+ core widgets (`AppTextField`, `AppCard`, `EmptyState`, `SegmentedButton`, `DataChoiceDialog`, etc.) are reused across all Warga screens and are designed to be reusable for Iuran/KAS.
4. **Theme system** — All colors, spacing, and radii are defined as semantic constants in `AppColors`, `AppSpacing`, and `AppRadius`. No hardcoded values exist in feature screens.
5. **State management** — Riverpod `AsyncNotifierProvider` for list data, `StateProvider` for search query, repository pattern with error mapping.
6. **Responsive patterns** — `kIsWeb` detection controls FAB styling, edit button position, and dialog layout.
7. **Navigation** — GoRouter `ShellRoute` → `MainNavigationShell` → bottom nav bar; nested `GoRoute` for list/detail flows.

**Key finding:** `CashScreen` and `DuesScreen` currently use the **exact same scaffold pattern** as `WargaScreen` (DoubleBackExitScope + AppBar with MenuAppBarTitle + body), but are only displaying `EmptyState`. They are structural placeholders ready to be replaced with real list+form implementations that mirror Warga's patterns.

---

## 2. File Paths Read (Complete List)

| # | File Path | Purpose |
|---|-----------|---------|
| 1 | `mobile/lib/main.dart` | Entry: `ProviderScope` → `App` |
| 2 | `mobile/lib/app/app.dart` | GoRouter config, MaterialApp, auth guard, all route definitions |
| 3 | `mobile/lib/app/navigation_shell.dart` | `MainNavigationShell`, bottom nav bar (`_NavRailItem`) |
| 4 | `mobile/lib/core/theme/app_colors.dart` | Semantic color tokens (60+ constants), helper methods |
| 5 | `mobile/lib/core/theme/app_spacing.dart` | 4px grid spacing constants |
| 6 | `mobile/lib/core/theme/app_radius.dart` | Corner radius constants |
| 7 | `mobile/lib/core/widgets/app_badge.dart` | `AppBadge`, `_NeonBadge` |
| 8 | `mobile/lib/core/widgets/app_card.dart` | `AppCard` |
| 9 | `mobile/lib/core/widgets/app_text_field.dart` | `AppTextField` |
| 10 | `mobile/lib/core/widgets/double_back_exit_scope.dart` | `DoubleBackExitScope` |
| 11 | `mobile/lib/core/widgets/empty_state.dart` | `EmptyState` |
| 12 | `mobile/lib/core/widgets/error_state.dart` | `ErrorState` |
| 13 | `mobile/lib/core/widgets/loading_indicator.dart` | `LoadingIndicator` |
| 14 | `mobile/lib/core/widgets/menu_app_bar_title.dart` | `MenuAppBarTitle` |
| 15 | `mobile/lib/core/widgets/one_shot_animated_gif.dart` | One-shot GIF |
| 16 | `mobile/lib/core/widgets/primary_button.dart` | `PrimaryButton` |
| 17 | `mobile/lib/core/widgets/section_header.dart` | `SectionHeader` |
| 18 | `mobile/lib/core/widgets/summary_card.dart` | `SummaryCard` |
| 19 | `mobile/lib/features/warga/presentation/screens/warga_screen.dart` | Main list page |
| 20 | `mobile/lib/features/warga/presentation/screens/household_create_screen.dart` | Create form |
| 21 | `mobile/lib/features/warga/presentation/screens/household_edit_screen.dart` | Edit form |
| 22 | `mobile/lib/features/warga/presentation/screens/special_resident_create_screen.dart` | Create petugas form |
| 23 | `mobile/lib/features/warga/presentation/screens/special_resident_edit_screen.dart` | Edit petugas form |
| 24 | `mobile/lib/features/warga/presentation/dialogs/data_choice_dialog.dart` | "Tambah Data Baru" dialog |
| 25 | `mobile/lib/features/warga/data/warga_repository.dart` | API repository + error mapping |
| 26 | `mobile/lib/features/warga/data/warga_providers.dart` | Riverpod providers |
| 27 | `mobile/lib/features/warga/data/api_warga_models.dart` | Data models |
| 28 | `mobile/lib/features/warga/data/mock_warga_data.dart` | Mock data |
| 29 | `mobile/lib/features/cash/presentation/screens/cash_screen.dart` | Placeholder |
| 30 | `mobile/lib/features/dues/presentation/screens/dues_screen.dart` | Placeholder |
| 31 | `mobile/lib/core/models/role.dart` | `AppRole` enum, `kWargaWriteRoles` |
| 32 | `mobile/lib/core/errors/app_errors.dart` | `ServerException`, error types |
| 33 | `mobile/lib/core/api/client.dart` | Dio `ApiClient` |
| 34 | `mobile/lib/core/api/config.dart` | `ApiConfig` |
| 35 | `mobile/lib/auth/data/mock_auth_repository.dart` | Auth state management |

---

## 3. Detailed Findings

### 3.1 WargaScreen — Main List Page

**File:** `mobile/lib/features/warga/presentation/screens/warga_screen.dart`

**Widget Tree:**
```
DoubleBackExitScope
 └─ Scaffold(
      appBar: AppBar(title: MenuAppBarTitle('Warga'))
      floatingActionButton: Conditional FAB
      body: Column(
        children: [
          Search Section (AppTextField)
          Summary Count (Text)
          Content Area (Expanded)
            ├─ AsyncLoading → CircularProgressIndicator
            ├─ AsyncError → error icon + message + retry button
            ├─ Empty (no search) → EmptyState(people_outline)
            ├─ Empty (search) → EmptyState(search_off) + "Hapus Pencarian"
            └─ ListView.separated → _ResidentCard items
        ]
      )
    )
```

**Search:**
- `AppTextField` with `prefixIcon: Icons.search`, conditional `suffixIcon: IconButton(Icons.clear)`
- Padding: `EdgeInsets.fromLTRB(base, sm, base, sm)` = `16, 8, 16, 8`
- `onChanged` → updates `wargaSearchQueryProvider` (StateProvider<String>) + `setState()`
- Client-side filtering via `filteredWargaListProvider` matching name, nik, houseNumber

**Summary Count:**
```dart
Text(
  'Menampilkan ${residents.length} warga',
  style: theme.textTheme.bodySmall?.copyWith(
    color: theme.colorScheme.onSurfaceVariant,
    fontWeight: FontWeight.w600,
  ),
)
```
Padding: `EdgeInsets.symmetric(horizontal: base, vertical: xs)` = `16, 4`

**ResidentCard (`_ResidentCard`):**
- Wraps in `AppCard(padding: base)` = padding 16
- Layout: `Row` → `CircleAvatar(radius: 20)` + name + badges + edit button
- Occupancy badge: violet (`accent`) for OWNER, blue (`info`) for TENANT
- Jabatan badge: `_buildJabatanBadge()` — uses `resident.jabatanColor` with alpha-tinted background
- Detail rows: house number (home_outlined), phone (phone_outlined), email (email_outlined) — icon size 16
- Edit button: mobile shows `IconButton` in card header; web shows "Ubah" button below card

**FAB Patterns:**

| Platform | Shape | Size | Icon | Label |
|----------|-------|------|------|-------|
| Android | Circular | 36×36 | `person_add` 16px | None |
| Web | Rounded pill (24) | 48×auto | `person_add` 20px | "Tambah" 13px |

Both variants: `Container` with `accentSoft` background, `accent` border (1px), `accent` shadow (alpha 0.15), `InkWell` + `Material`.

**Write Access:**
- `_hasWargaWriteAccess()` checks: super_admin → true, jabatan in `kWargaWriteJabatans` → true, role in `kWargaWriteRoles` → true
- `kWargaWriteJabatans = {ketua, wakil_ketua, sekretaris, bendahara}`
- `kWargaWriteRoles = {superAdmin, pengurus, bendahara, perangkat}`
- FAB is `null` (hidden) when canWrite is false

**Error State:**
- Error icon: `Icons.error_outline`, size 64, alpha 0.5
- Title: "Terjadi Kesalahan" (`titleLarge`)
- Message: error body, centered
- Retry button: 36px height, primary bg 12% alpha, primary fg, elevation 0, borderRadius 6, text "Coba Lagi" 12px

**Empty States:**
- No data: `EmptyState(icon: Icons.people_outline, title: 'Belum Ada Warga', description: 'Data kependudukan warga RT belum tersedia.')`
- Search empty: `EmptyState(icon: Icons.search_off, title: 'Warga Tidak Ditemukan', description: '...' actionText: 'Hapus Pencarian', onAction: _clearSearch)`

---

### 3.2 HouseholdCreateScreen — Create Form

**File:** `mobile/lib/features/warga/presentation/screens/household_create_screen.dart`

**Widget Tree:**
```
PopScope(canPop: false, onPopInvokedWithResult: _confirmCancel)
 └─ Scaffold(
      appBar: AppBar(title: 'Tambah Warga', leading: IconButton(arrow_back) + _confirmCancel)
      body: Form(
        key: _formKey,
        child: ListView(
          padding: base (16),
          children: [
            Error message (server error)
            Validation errors (client-side)
            RT selector (superadmin only)
            Row: Nomor Rumah (flex 2) + Status Hunian: SegmentedButton (flex 3)
            Nama Kepala Keluarga
            NIK (digitsOnly, 16 char limit)
            Nomor Telepon (phone keyboard)
            Email (email keyboard)
            Tanggal Mulai Hunian: InkWell → showDatePicker
            Alamat (optional)
            Status: SegmentedButton (Aktif/Tidak Aktif)
            Row: Simpan + Batal (Equal Expanded)
          ]
        )
      )
    )
```

**Field Spacing:** `SizedBox(height: lg)` = 20px between fields.

**RT Selector (superadmin only):**
- `DropdownButtonFormField` with `padding: md horizontal, sm vertical`
- Border: `OutlineInputBorder(borderRadius: base)`
- Items formatted as: `RT $rtNum / RW $rwNum — $name`

**Status Hunian:** `SegmentedButton<String>` with `OWNER` (Pemilik, home icon) and `TENANT` (Penyewa, hail icon). Style: `padding: (8, 4)`, `selectedForegroundColor: accent`, `selectedBackgroundColor: accentSoftColor(context)`.

**Date Picker:** `InkWell` wrapping `Container` with border, `calendar_today` icon (accent color), date text, `arrow_forward_ios` chevron.

**Validation Errors:**
```dart
Container(
  padding: md,
  color: danger.withValues(alpha: isDark ? 0.15 : 0.08),
  borderRadius: base,
  child: Column(
    crossAxisAlignment: start,
    children: [
      Text('Data belum lengkap:', fontWeight: w600, color: danger),
      ...errors.map((e) => Row(children: ['•', Expanded(e)])),
    ],
  ),
)
```

**Form Validation Rules:**
| Field | Required | Format |
|-------|----------|--------|
| Nomor Rumah | Yes | Non-empty |
| Nama Kepala Keluarga | Yes | Non-empty |
| NIK | Yes | `^[0-9]{16}$` |
| Nomor Telepon | Yes | Non-empty |
| Email | Yes | `^[^\s@]+@[^\s@]+\.[^\s@]+$` |
| Tanggal Mulai Hunian | Yes | Non-empty date |
| RT Tujuan | Yes (superadmin) | Non-null |

---

### 3.3 HouseholdEditScreen — Edit Form

**File:** `mobile/lib/features/warga/presentation/screens/household_edit_screen.dart`

**Differences from Create:**
1. **Data loading:** `_loadHousehold()` in `initState()` → calls `fetchResidentsWithHouseholds(page: 1, pageSize: 100)` + `firstWhere(householdId: widget.householdId)` + `fetchHouseholds()` for startDate
2. **Loading state:** Returns Scaffold with `Center(CircularProgressIndicator)` while loading
3. **Not-found state:** Returns Scaffold with error icon + "Data warga tidak ditemukan." + "Kembali" button
4. **Tanggal Mulai Hunian is READ-ONLY:** Container with muted text display, no date picker
5. **Simpan button text:** "Simpan Perubahan" (uses Flexible + FittedBox for truncation)
6. **Update API:** Calls `repo.updateHousehold(householdId, request: UpdateHouseholdRequest(...))` — all fields nullable (PATCH)
7. **No RT selector** — edit is bound to existing household

---

### 3.4 SpecialResidentCreate/Edit — Petugas Forms

**Files:**
- `mobile/lib/features/warga/presentation/screens/special_resident_create_screen.dart`
- `mobile/lib/features/warga/presentation/screens/special_resident_edit_screen.dart`

**Create fields:** Jenis Petugas (Dropdown: keamanan/kebersihan_pembangunan), Nama, NIK, Telepon, Email, Status, RT (superadmin)

**Edit differences from Create:**
- `_loadSpecialResident()` in `initState` → `fetchResidentById(residentId)` → validates jabatan is non-empty
- Not-found state: "Data petugas tidak ditemukan."
- Status field label: "Status Aktif/Tidak Aktif"
- Simpan button: "Simpan Perubahan" (Flexible + FittedBox)

**Both share identical patterns with Household forms:** same error display, same validation errors, same Simpan/Batal buttons, same _confirmCancel dialog.

---

### 3.5 DataChoiceDialog

**File:** `mobile/lib/features/warga/presentation/dialogs/data_choice_dialog.dart`

```dart
Dialog(
  shape: RoundedRectangleBorder(borderRadius: lg),
  child: Padding(lg, child: Column(
    Text('Tambah Data Baru') — headlineSmall + bold
    Text('Pilih jenis data...') — bodyMuted
    _ChoiceCard: Warga — person_add, accent, "Kepala keluarga atau anggota"
    _ChoiceCard: Petugas — work_outline, info, "Keamanan/kebersihan"
    OutlinedButton: Batal — danger border + text + shadow
  )),
)
```

**ChoiceCard (`_ChoiceCard`):**
- `InkWell` + `Container` with colored background (alpha 0.06 light / 0.10 dark), colored border (alpha 0.20 light / 0.30 dark)
- Icon circle: `padding: sm`, background alpha 0.15 light / 0.20 dark
- Title: `titleMedium + w600`, subtitle: `bodySmall + onSurfaceVariant`
- Trailing: `chevron_right` 20px

---

### 3.6 Write Access Control in UI

**File:** `mobile/lib/features/warga/presentation/screens/warga_screen.dart`

```dart
bool _hasWargaWriteAccess(WidgetRef ref) {
  final authState = ref.read(authRepositoryProvider);
  final systemRole = authState?.valueOrNull?['system_role'] as String?;
  
  // 1. SUPER_ADMIN always has write access
  if (systemRole == 'super_admin') return true;
  
  // 2. Check jabatan-based authorization first
  final jabatan = authState?.valueOrNull?['jabatan'] as String?;
  if (jabatan != null && jabatan.isNotEmpty) {
    return kWargaWriteJabatans.contains(jabatan);
  }
  
  // 3. Fallback: role-based authorization
  final role = authState?.valueOrNull?['role'] as AppRole?;
  return (role != null && kWargaWriteRoles.contains(role));
}
```

**Constants:**
```dart
const Set<String> kWargaWriteJabatans = {
  'ketua', 'wakil_ketua', 'sekretaris', 'bendahara',
};

const Set<AppRole> kWargaWriteRoles = {
  AppRole.superAdmin, AppRole.pengurus, AppRole.bendahara, AppRole.perangkat,
};
```

**FAB visibility:** `canWrite ? buildFAB() : null` — FAB is hidden entirely when user lacks write access.

---

### 3.7 PopScope + Cancel Confirmation Dialog

All form screens use an identical `_confirmCancel()` pattern:

```dart
Future<void> _confirmCancel() async {
  final shouldCancel = await showDialog<bool>(
    context: context,
    builder: (dialogContext) => AlertDialog(
      title: Text('Apakah Anda yakin ingin membatalkan?'),
      content: Text('Perubahan yang belum disimpan akan hilang.'),
      actions: [
        OutlinedButton — "Tidak, Tetap di Halaman" (accent border + soft bg)
        OutlinedButton — "Ya, Batalkan" (danger border + soft bg + shadow)
      ],
    ),
  );
  if (shouldCancel == true && mounted) context.pop();
}
```

Used by: `PopScope(onPopInvokedWithResult: _confirmCancel)`, AppBar back button, and Batal button.

---

### 3.8 Server Error Display

```dart
if (_error != null)
  Container(
    padding: md,
    decoration: BoxDecoration(
      color: danger.withValues(alpha: isDark ? 0.2 : 0.1),
      borderRadius: base,
      border: Border.all(color: danger.withValues(alpha: isDark ? 0.4 : 0.2)),
    ),
    child: Text(_error!, style: bodyMedium?.copyWith(color: danger)),
  ),
```

---

### 3.9 Simpan/Batal Button Pattern

Both buttons use identical styling:

```dart
Row(
  children: [
    Expanded(
      child: Container(
        decoration: BoxDecoration(boxShadow: [accent shadow alpha 0.15]),
        child: SizedBox(
          height: 48,
          child: OutlinedButton(
            onPressed: _saving ? null : _save,
            style: OutlinedButton.styleFrom(
              side: const BorderSide(color: AppColors.accent),
              foregroundColor: AppColors.accent,
              shape: RoundedRectangleBorder(borderRadius: base),
            ),
            child: _saving ? CircularProgressIndicator : Row(icon + text),
          ),
        ),
      ),
    ),
    SizedBox(width: md),
    Expanded(
      child: Container(
        decoration: BoxDecoration(boxShadow: [danger shadow alpha 0.15]),
        child: SizedBox(
          height: 48,
          child: OutlinedButton(
            onPressed: _saving ? null : _confirmCancel,
            style: OutlinedButton.styleFrom(
              side: BorderSide(color: AppColors.danger),
              foregroundColor: AppColors.danger,
              shape: RoundedRectangleBorder(borderRadius: base),
            ),
            child: Row(icon + text),
          ),
        ),
      ),
    ),
  ],
)
```

---

### 3.10 Placeholder Screens (Cash / Dues)

**CashScreen** (`mobile/lib/features/cash/presentation/screens/cash_screen.dart`):
```dart
DoubleBackExitScope(
  child: Scaffold(
    appBar: AppBar(centerTitle: false, title: MenuAppBarTitle('Kas')),
    body: Center(
      child: EmptyState(
        icon: Icons.account_balance_wallet_outlined,
        title: 'Kas',
        description: 'Pengelolaan kas akan tersedia pada tahap berikutnya.',
      ),
    ),
  ),
)
```

**DuesScreen** (`mobile/lib/features/dues/presentation/screens/dues_screen.dart`):
```dart
DoubleBackExitScope(
  child: Scaffold(
    appBar: AppBar(centerTitle: false, title: MenuAppBarTitle('Iuran')),
    body: Center(
      child: EmptyState(
        icon: Icons.receipt_long_outlined,
        title: 'Iuran',
        description: 'Pengelolaan iuran warga akan tersedia pada tahap berikutnya.',
      ),
    ),
  ),
)
```

Both follow the **exact same scaffold pattern** as WargaScreen but with `EmptyState` instead of a list. They use the same `MenuAppBarTitle` with the correct tab label.

---

## 4. Widget/Component Inventory

### 4.1 Core Widgets (Reusable for Iuran/KAS)

| Widget | File | Purpose | Params | Reuse |
|--------|------|---------|--------|-------|
| `AppTextField` | `core/widgets/app_text_field.dart` | TextFormField wrapper with label/hint/prefixIcon/suffixIcon/inputFormatters | controller, label, hintText, validator, keyboardType, obscureText, prefixIcon, suffixIcon, onChanged, inputFormatters | ✅ YES — generic input |
| `AppCard` | `core/widgets/app_card.dart` | Card with elevation 0, border, configurable padding | child, padding, useEmphasis, cardColor | ✅ YES — generic container |
| `EmptyState` | `core/widgets/empty_state.dart` | Centered icon + title + description + optional action | icon, title, description, actionText, onAction | ✅ YES — generic |
| `ErrorState` | `core/widgets/error_state.dart` | Error icon + message + retry | icon, message, onRetry | ✅ YES — generic |
| `LoadingIndicator` | `core/widgets/loading_indicator.dart` | Centered CircularProgressIndicator + optional message | message | ✅ YES — generic |
| `SummaryCard` | `core/widgets/summary_card.dart` | Metric card with title, large value, optional icon & accentColor | title, value, icon, accentColor, titleStyle, valueStyle | ✅ YES — generic |
| `SectionHeader` | `core/widgets/section_header.dart` | Row(title + trailing) | title, trailing | ✅ YES — generic |
| `AppBadge` | `core/widgets/app_badge.dart` | Status badge/pill with optional neon style | label, backgroundColor, textColor, isNeonStyle | ✅ YES — generic |
| `PrimaryButton` | `core/widgets/primary_button.dart` | ElevatedButton with accent bg, loading state, icon support | child, isLoading, icon, onPressed | ✅ YES — generic |
| `MenuAppBarTitle` | `core/widgets/menu_app_bar_title.dart` | AppBar title with RT/RW info | title | ✅ YES — generic |
| `DoubleBackExitScope` | `core/widgets/double_back_exit_scope.dart` | Android double-tap back to exit | child | ✅ YES — wrap all screens |
| `DataChoiceDialog` | `features/warga/presentation/dialogs/data_choice_dialog.dart` | Two-choice dialog | onChoice callback | ⚠️ Conditionally — may need new dialogs |

### 4.2 Warga-Specific Widgets (Not Directly Reusable)

| Widget | File | Why |
|--------|------|-----|
| `_ResidentCard` (private) | `warga_screen.dart` | Warga-specific data display |
| `_buildJabatanBadge` (private) | `warga_screen.dart` | Jabatan-specific badge rendering |

### 4.3 Form-Specific Patterns (Reusable Template)

| Pattern | Location | Reuse For |
|---------|----------|-----------|
| `PopScope` + `_confirmCancel` | All 4 form screens | Iuran/KAS create & edit forms |
| Simpan/Batal horizontal Row | All 4 form screens | Iuran/KAS forms |
| Server error Container | All 4 form screens | Iuran/KAS forms |
| Validation error list | All 4 form screens | Iuran/KAS forms |
| SegmentedButton (Status) | All 4 form screens | Iuran/KAS boolean fields |
| SegmentedButton (Occupancy) | Household forms | Iuran/KAS enum fields |
| Date picker (InkWell + showDatePicker) | Household forms | Iuran/KAS date fields |
| RT selector (DropdownButtonFormField) | All 4 form screens | Iuran/KAS (if superadmin) |
| NIK input (digitsOnly, 16 limit) | All 4 form screens | Iuran/KAS if NIK needed |

---

## 5. Color/Typography Constants

### 5.1 Semantic Colors (`AppColors`)

| Token | Hex | Light Alpha | Dark Alpha | Used For |
|-------|-----|-------------|------------|----------|
| `accent` | `#A970FF` | — | — | Primary buttons, links, active states |
| `success` | `#73BF69` | — | — | Success messages, income |
| `warning` | `#FF9830` | — | — | Warnings, alerts |
| `danger` | `#F2495C` | — | — | Errors, expenses, delete |
| `info` | `#5794F2` | — | — | Information |

### 5.2 Light Mode Colors

| Token | Hex | Used For |
|-------|-----|----------|
| `lightBg` | `#F6F6F7` | Scaffold background |
| `lightSurface` | `#FFFFFF` | Cards, panels |
| `lightSurfaceHover` | `#EAEAEA` | Hover states |
| `lightText` | `#1A1A1A` | Primary text |
| `lightTextMuted` | `#6B6B6B` | Secondary text |
| `lightBorder` | `#DEDEDE` | Borders, dividers |
| `lightBgSubtle` | `#EAEDED` | Subtle backgrounds |
| `lightSurfaceSubtle` | `#F2F2F4` | Subtle surfaces |
| `lightBorderSubtle` | `#E9E9E9` | Subtle borders |
| `lightAccentSoft` | `#EDE8F9` | Accent tinted background |
| `lightSuccessSoft` | `#F1F7ED` | Success tint |
| `lightWarningSoft` | `#FFF5E6` | Warning tint |
| `lightDangerSoft` | `#FEF0F2` | Danger tint |
| `lightInfoSoft` | `#EEF3FB` | Info tint |

### 5.3 Dark Mode Colors

| Token | Hex | Used For |
|-------|-----|----------|
| `darkBg` | `#0D0D0D` | Scaffold background |
| `darkSurface` | `#1A1A1A` | Cards, panels |
| `darkSurfaceHover` | `#222222` | Hover states |
| `darkText` | `#E6E6E6` | Primary text |
| `darkTextMuted` | `#888888` | Secondary text |
| `darkBorder` | `#2A2A2A` | Borders, dividers |
| `darkBgSubtle` | `#141414` | Subtle backgrounds |
| `darkSurfaceSubtle` | `#121212` | Subtle surfaces |
| `darkBorderSubtle` | `#1F1F1F` | Subtle borders |
| `darkAccentSoft` | `#241F32` | Accent tinted background |
| `darkSuccessSoft` | `#1D261A` | Success tint |
| `darkWarningSoft` | `#2A2218` | Warning tint |
| `darkDangerSoft` | `#291B1E` | Danger tint |
| `darkInfoSoft` | `#1B2335` | Info tint |

### 5.4 Neutral Badge Colors

| Token | Hex | Used For |
|-------|-----|----------|
| `lightBadgeNeutral` | `#6B7280` | Neutral badge text (light) |
| `darkBadgeNeutral` | `#C4C9CE` | Neutral badge text (dark) |
| `lightBadgeNeutralBg` | `#ECEDEE` | Neutral badge background |
| `lightBadgeNeutralBorder` | `#969BA5` | Neutral badge border |
| `darkBadgeNeutralBg` | `#3A3F44` | Neutral badge background (dark) |
| `darkBadgeNeutralBorder` | `#737A82` | Neutral badge border (dark) |

### 5.5 Helper Methods

All return `Color` based on theme brightness:

```dart
AppColors.textColor(context)         // lightText / darkText
AppColors.textMutedColor(context)    // lightTextMuted / darkTextMuted
AppColors.surfaceColor(context)      // lightSurface / darkSurface
AppColors.bgColor(context)           // lightBg / darkBg
AppColors.borderColor(context)       // lightBorder / darkBorder
AppColors.accentSoftColor(context)   // lightAccentSoft / darkAccentSoft
```

### 5.6 Spacing (`AppSpacing`)

| Token | Value | Used For |
|-------|-------|----------|
| `xxs` | 2 | Tight spacing |
| `xs` | 4 | Small gaps |
| `sm` | 8 | Field separators, list dividers |
| `md` | 12 | Card padding, dialog inner |
| `base` | 16 | Screen padding, card default padding |
| `lg` | 20 | Field spacing in forms |
| `xl` | 24 | Large spacing |
| `xxl` | 32 | Bottom form button margin |
| `xxxl` | 40 | Extra large spacing |

### 5.7 Radius (`AppRadius`)

| Token | Value | Used For |
|-------|-------|----------|
| `xs` | 4.0 | Tiny badges |
| `sm` | 8.0 | Badges, nav items |
| `base` | 12.0 | Cards, buttons, dialogs |
| `lg` | 16.0 | Emphasis cards |
| `xl` | 24.0 | FAB pills, dialog shape |

---

## 6. Routing Map

**Framework:** GoRouter with ShellRoute

```
GoRouter (MaterialApp.router)
 │
 ├─ GoRoute /login              → LoginScreen
 │
 └─ ShellRoute                  → MainNavigationShell (BottomAppBar)
      │
      └─ GoRoute /home          → DashboardScreen
           │
           ├─ GoRoute /home/cash          → CashScreen (placeholder)
           ├─ GoRoute /home/dues          → DuesScreen (placeholder)
           ├─ GoRoute /home/warga         → WargaScreen
           │    ├─ GoRoute /home/warga/baru          → HouseholdCreateScreen
           │    ├─ GoRoute /home/warga/edit/household/:householdId → HouseholdEditScreen
           │    ├─ GoRoute /home/warga/baru/petugas  → SpecialResidentCreateScreen
           │    └─ GoRoute /home/warga/edit/petugas/:residentId → SpecialResidentEditScreen
           ├─ GoRoute /home/reports       → ReportsScreen
           └─ GoRoute /home/profile       → ProfileScreen
```

**Navigation methods used:**
- `context.push('/home/warga/baru')` — navigate to new page (pushes onto stack)
- `context.pop()` — go back (pops stack)
- `context.go('/home')` — jump to route (clears stack)
- `context.push('/home/warga/edit/petugas/{id}')` — navigate with path parameter

**Auth guard:** Router redirect checks auth state:
- Not logged in + trying non-login route → redirect to `/login`
- Logged in + on `/login` → redirect to `/home`

**Tab index sync:** `MainNavigationShell` uses `GoRouterState.of(context).matchedLocation` + longest prefix match to sync bottom nav bar index.

---

## 7. Data Flow Diagram

### 7.1 List Page Data Flow

```
┌─────────────────────────────────────────────────────┐
│                   WargaScreen                        │
│  ConsumerStatefulWidget (uses Riverpod)              │
│                                                      │
│  ref.watch(wargaListProvider)        → AsyncValue    │
│  ref.watch(filteredWargaListProvider) → List<>       │
│  ref.watch(wargaSearchQueryProvider)  → String       │
│                                                      │
│  ┌──────────────────────────────────────┐           │
│  │ wargaSearchQueryProvider             │           │
│  │   StateProvider<String>              │           │
│  │   Updated by: _clearSearch / onCh    │           │
│  └──────────────┬───────────────────────┘           │
│                 │                                    │
│                 ▼                                    │
│  ┌──────────────────────────────────────┐           │
│  │ filteredWargaListProvider            │           │
│  │   Provider<List<MappedResident>>     │           │
│  │   Client-side filtering              │           │
│  │   Matches: name, nik, houseNumber    │           │
│  └──────────────┬───────────────────────┘           │
│                 │                                    │
│                 ▼                                    │
│  ┌──────────────────────────────────────┐           │
│  │ wargaListProvider                    │           │
│  │   AsyncNotifierProvider              │           │
│  │   Auto-dispose                       │           │
│  │   Calls: WargaRepository             │           │
│  │          fetchResidentsWithHouseholds │           │
│  └──────────────┬───────────────────────┘           │
│                 │                                    │
│                 ▼                                    │
│  ┌──────────────────────────────────────┐           │
│  │ WargaRepository                      │           │
│  │   fetchResidentsWithHouseholds()      │           │
│  │     → fetchResidents() → Paginated<> │           │
│  │     → fetchHouseholds() → Map<>      │           │
│  │     → MappedResident.fromBackend()   │           │
│  └──────────────┬───────────────────────┘           │
│                 │                                    │
│                 ▼                                    │
│  ┌──────────────────────────────────────┐           │
│  │ ApiClient (Dio)                      │           │
│  │   GET /api/v1/residents              │           │
│  │   GET /api/v1/households             │           │
│  └──────────────────────────────────────┘           │
└─────────────────────────────────────────────────────┘
```

### 7.2 Form Page Data Flow

```
┌─────────────────────────────────────────────────────┐
│           HouseholdCreateScreen / EditScreen          │
│                                                      │
│  ┌─ Form Fields ──────────────────────────────────┐  │
│  │ TextEditingController[]                          │  │
│  │   _houseNumberCtrl, _headNameCtrl, _nikCtrl...  │  │
│  └────────────────────────────────────────────────┘  │
│                                                      │
│  ┌─ Validation ───────────────────────────────────┐  │
│  │ _validate(): checks required + regex           │  │
│  │ Sets _validationErrors state                   │  │
│  └────────────────────────────────────────────────┘  │
│                                                      │
│  ┌─ Save Action ──────────────────────────────────┐  │
│  │ _save(): _validate → repo.create/update        │  │
│  │ → ref.read(wargaListProvider.notifier).refresh │  │
│  │ → ScaffoldMessenger.success SnackBar           │  │
│  │ → context.pop()                                │  │
│  └────────────────────────────────────────────────┘  │
│                                                      │
│  ┌─ Error Handling ───────────────────────────────┐  │
│  │ catch (e) → ref.read(repo).mapDioError(e)      │  │
│  │ → setState(() => _error = error.message)       │  │
│  └────────────────────────────────────────────────┘  │
│                                                      │
│  ┌─ Cancel ───────────────────────────────────────┐  │
│  │ _confirmCancel(): showDialog → AlertDialog     │  │
│  │ → "Tidak, Tetap di Halaman" / "Ya, Batalkan"   │  │
│  │ → context.pop() only if confirmed              │  │
│  └────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────┘
```

### 7.3 State Management Summary

| Provider | Type | Purpose | Reset |
|----------|------|---------|-------|
| `wargaSearchQueryProvider` | `StateProvider<String>` | Current search text | Clear on logout |
| `wargaListProvider` | `AsyncNotifierProvider` (auto-dispose) | List data + loading/error | Auto on dispose, manual `.refresh()` |

**Not shown in code but imported:** `warga_providers.dart` defines the `AsyncNotifier` class that calls `WargaRepository.fetchResidentsWithHouseholds()`.

---

## 8. Reuse Plan — Iuran & KAS Modules

### 8.1 What to Reuse (Identical Pattern)

These patterns from Warga should be **copied directly** to Iuran and KAS screens:

| Pattern | Source | Target |
|---------|--------|--------|
| **Scaffold shell** | WargaScreen → CashScreen → DuesScreen | ✅ Both placeholders already match |
| **DoubleBackExitScope wrapper** | All Warga screens | ✅ Both placeholders already use it |
| **AppBar + MenuAppBarTitle** | WargaScreen → CashScreen → DuesScreen | ✅ Both placeholders already match |
| **Search bar** | WargaScreen | → Iuran: search by nama/pembayaran, → KAS: search by kategori/tanggal |
| **Summary count** | WargaScreen ("Menampilkan N data") | → "Menampilkan N transaksi", "Menampilkan N tagihan" |
| **ListView.separated + divider** | WargaScreen | → Iuran list, KAS list |
| **AsyncLoading / AsyncError / Empty / List states** | WargaScreen | → Iuran: 4 states, KAS: 4 states |
| **Error state with retry** | WargaScreen | → Iuran: retry button, KAS: retry button |
| **EmptyState widgets** | WargaScreen (people_outline / search_off) | → Iuran: receipt_long_outlined / search_off, KAS: account_balance_wallet_outlined / search_off |
| **PopScope + _confirmCancel** | All 4 form screens | → Iuran create/edit, KAS create/edit |
| **Server error Container** | All 4 form screens | → Iuran/KAS form error display |
| **Validation error list** | All 4 form screens | → Iuran/KAS form validation |
| **Simpan/Batal buttons** | All 4 form screens | → Iuran/KAS form buttons |
| **SegmentedButton (boolean enum)** | Status fields | → Iuran: status pembayaran, KAS: status transaksi |
| **Date picker** | Tanggal Mulai Hunian | → Iuran: tanggal pembayaran, KAS: tanggal transaksi |
| **RT selector (superadmin)** | All 4 form screens | → Iuran/KAS if superadmin routing needed |
| **AppTextField** | All form screens | → Iuran/KAS form inputs |
| **AppCard** | ResidentCard | → Iuran/KAS transaction cards |
| **SummaryCard** | Dashboard | → Iuran/KAS metric cards (total, count) |
| **DataChoiceDialog** | WargaScreen FAB | → Iuran/KAS FAB dialog (if two types needed) |

### 8.2 What Needs Adaptation (Warga-Specific)

| Warga Pattern | Iuran Adaptation | KAS Adaptation |
|---------------|------------------|----------------|
| `_ResidentCard` with name, NIK, houseNumber, occupancy, jabatan | Transaction card: tanggal, nama payer, nominal, kategori | Transaction card: tanggal, deskripsi, nominal (income/expense), kategori |
| Occupancy badge (OWNER/TENANT) | Status badge (LUNAS/BELUM LUNAS/DIBAYAR SEPARUH) | Status badge (DITERIMA/DITERIMA) |
| Jabatan badge | Kategori badge (Iuran Rutin, Iuran Pembangunan, dll) | Kategori badge (Pendapatan, Pengeluaran) |
| Edit IconButton (household vs petugas routing) | Edit payment status, edit payment details | Edit transaction, add new transaction |
| HouseholdCreateScreen fields (7 fields) | Iuran form: nama, bulan/tahun, nominal, metode pembayaran | KAS form: tanggal, kategori, deskripsi, nominal, tipe (income/expense) |
| `kWargaWriteJabatans` / `kWargaWriteRoles` | → `kIuranWriteJabatans` / `kKasWriteJabatans` (new constants) | → `kKasWriteJabatans` (new constants) |

### 8.3 New Components Needed

| Component | Purpose |
|-----------|---------|
| **TransactionCard** | Replaces `_ResidentCard` — displays amount, date, description, status |
| **StatusBadge** (generic) | Replaces occupancy+jabatan badges — shows "LUNAS", "BELUM LUNAS", "PENDAPATAN", "PENGELUARAN" |
| **Amount formatter** | Uses `rupiah_formatter.dart` — displays "Rp 150.000" format |
| **TypeSelector** | Replaces SegmentedButton for income/expense toggle (KAS) |
| **MonthSelector** | Dropdown for month/year selection (Iuran) |
| **CategorySelector** | Dropdown for transaction category (KAS) |
| **Iuran-specific DataChoiceDialog** | May need different options than Warga |

### 8.4 Implementation Order (Recommended)

1. **List pages first** — Iuran list page (mimics WargaScreen structure), KAS list page (mimics WargaScreen structure)
2. **Form pages second** — Iuran create/edit forms (mimics HouseholdCreateScreen structure), KAS create form (mimics HouseholdCreateScreen structure)
3. **DataChoiceDialog third** — New dialogs if FAB needs different choices
4. **SummaryCards fourth** — Metric cards for dashboards showing totals

### 8.5 Prohibited Actions

- **NEVER create new color tokens** — all colors already exist in `AppColors`. Use `accent`, `success`, `danger`, `info`, `warning`.
- **NEVER create new spacing constants** — all spacing exists in `AppSpacing`. Use `xxs` through `xxxl`.
- **NEVER create new radius constants** — all radii exist in `AppRadius`. Use `xs` through `xl`.
- **NEVER use hardcoded values** — no `Color(0xFF...)`, no `16`, no `8`, no `12` directly in feature code.
- **NEVER create new widget files for generic components** — use existing `AppTextField`, `AppCard`, `EmptyState`, etc.
- **NEVER change WargaScreen patterns** — Iuran and KAS must match Warga's visual style, not the other way around.
