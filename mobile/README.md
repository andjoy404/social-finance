# Social Finance - Flutter Client

## Flutter Environment

- **Flutter version:** 3.44.1
- **Dart version:** 3.12.1
- **Channel:** stable

## Architecture

Feature-oriented Flutter structure:

```
lib/
├── main.dart                 # App entry point
├── app/
│   ├── app.dart              # Root MaterialApp + go_router config
│   ├── navigation_shell.dart # Bottom navigation shell
│   └── theme/
│       └── app_theme.dart    # Material 3 theme, colors, typography
├── core/
│   ├── constants/
│   │   └── app_constants.dart
│   ├── errors/
│   │   └── app_errors.dart
│   ├── models/
│   │   ├── role.dart
│   │   └── tenant_info.dart
│   └── widgets/
│       ├── app_card.dart
│       ├── app_text_field.dart
│       ├── empty_state.dart
│       ├── error_state.dart
│       ├── loading_indicator.dart
│       └── primary_button.dart
└── features/
    ├── auth/
    │   └── data/
    │       └── mock_auth_repository.dart
    ├── cash/
    │   └── presentation/
    │       └── screens/
    ├── dashboard/
    │   ├── data/
    │   │   └── mock_dashboard_data.dart
    │   └── presentation/
    │       └── screens/
    ├── dues/
    ├── reports/
    └── profile/
```

## State Management

**Chosen library:** flutter_riverpod 2.6.1

Riverpod was chosen for its simplicity, type safety, and compile-time guarantees. It integrates naturally with Flutter's widget lifecycle and provides a clean API for both simple state (e.g., login state) and more complex state (e.g., dashboard data).

The `authRepositoryProvider` is a `StateNotifierProvider` that manages login state as `AsyncValue<Map<String, dynamic>??>`.

## Routing

**Chosen library:** go_router 14.8.1

Route structure:
- `/login` — Login screen (shown when not authenticated)
- `/home` — Main dashboard (shown when authenticated)
  - `/home/cash` — Cash management (placeholder)
  - `/home/dues` — Dues management (placeholder)
  - `/home/reports` — Reports (placeholder)
  - `/home/profile` — Profile settings

The `ShellRoute` wraps authenticated routes with the bottom navigation shell.

## Repository Strategy

All data access goes through repository providers. The current implementation uses **mock repositories** for Phase A development:

```
UI → Provider → MockRepository (current)
UI → Provider → ApiRepository → Dio → Backend (future)
```

### AuthRepository (`features/auth/data/mock_auth_repository.dart`)

Mock authentication for UI development. Accepts any non-empty email/password.
NOT real authentication.

### DashboardRepository (`features/dashboard/data/mock_dashboard_data.dart`)

Mock financial data for dashboard development.

## How to Run

```bash
cd mobile/
flutter pub get
flutter run -d <device-id>
```

## How to Test

```bash
cd mobile/
flutter analyze
flutter test
```

## Backend Integration Status

**NOT STARTED.** This is Phase A — mock data only.

- Dio is added as a dependency for future API integration
- NO real HTTP requests are made
- NO `localhost` or backend URLs are hardcoded
- Repository layer is designed for easy swap between mock and API implementations

### Next Phase for API Integration

1. Replace `MockRepository` with `ApiRepository` in each provider
2. Configure Dio client with base URL
3. Add interceptors for auth tokens
4. Keep provider interfaces unchanged — UI code won't need modification

## Role Model

```dart
enum AppRole { superAdmin, pengurus, bendahara, warga }
```

Indonesian display labels:
- `superAdmin` → Super Admin
- `pengurus` → Pengurus
- `bendahara` → Bendahara
- `warga` → Warga

> **IMPORTANT:** Role visibility on the client is a UX-only concern. All authorization
> decisions must be enforced server-side. Do NOT rely on client-side role checks for security.

## Active Tenant Model

Minimal client-side representation:

```dart
class TenantInfo {
  final String id;
  final String name;   // e.g., "RT 002 / RW 016"
  final String rt;
  final String rw;
}
```

For now, the active tenant context comes from mock data. In Phase B, the real active tenant
will come from `GET /api/v1/me` or the login response.
