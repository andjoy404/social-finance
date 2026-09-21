# Social Finance - Android Screen Flow

## 1. Architecture

The Android application uses **Flutter** with a multi-layer architecture following the separation of concerns principles:

```
mobile/
├── lib/
│   ├── core/
│   │   ├── auth/              # Auth state, token manager (flutter_secure_storage)
│   │   ├── api/               # API client (Dio)
│   │   ├── error/             # Error mapping, exception classes
│   │   ├── network/           # Connectivity checker
│   │   └── utils/             # Helpers, formatters, constants
│   ├── data/
│   │   ├── models/            # API response/request data classes
│   │   ├── repositories/      # Repository implementations (API calls)
│   │   └── local/             # Local cache (if any — MVP may skip)
│   ├── domain/
│   │   ├── entities/          # Domain models
│   │   └── repositories/      # Repository interfaces (contract)
│   ├── presentation/
│   │   ├── auth/              # Login, register, password change screens
│   │   ├── dashboard/         # Dashboard screen
│   │   ├── household/         # Household list, detail, create/edit
│   │   ├── resident/          # Resident list, detail, create/edit
│   │   ├── finance/           #
│   │   │   ├── category/      # Categories list, create
│   │   │   ├── dues/          # Dues config (Pengurus/Bendahara only)
│   │   │   ├── bills/         # Bills list, detail
│   │   │   ├── payments/      # Payments list, create, detail
│   │   │   ├── transactions/  # Income/expense list, create, detail
│   │   │   └── ledger/        # Transaction ledger view
│   │   ├── report/            # Financial reports
│   │   ├── admin/             # User management, audit logs (Pengurus only)
│   │   └── profile/           # Profile, settings, logout
│   └── main.dart
├── test/
│   ├── unit/
│   ├── widget/
│   └── integration/
```

### 1.1 State Management

To be selected during implementation (BLoC, Riverpod, or Provider).
- **BLoC:** Event-driven, separates business logic from UI. Very popular in enterprise Flutter.
- **Riverpod:** Reactive, type-safe, no BuildContext required. Good for dependency injection.
- **Provider:** Simpler, good for smaller apps. May not scale as well.

**Recommendation:** BLoC for MVP because it's well-documented, separates concerns clearly, and is widely used in financial applications.

### 1.2 Navigation

Hierarchical navigation with bottom navigation bar for main sections. Modal dialogs for create/edit forms on mobile.
- **go_router:** Declarative routing, deep linking support, type-safe navigation. Recommended for Flutter.
- **Navigator 2.0+:** Built into Flutter, more flexible but more verbose.

### 1.3 HTTP Client

- **Dio:** Intercepting HTTP client with built-in support for interceptors, timeouts, retry logic, and response transformer.
- Dio is the industry standard for Flutter API clients and handles token refresh automatically.

---

## 2. Screen Flow by Role

### 2.1 WARGA (Resident)

#### 2.1.1 Authentication Flow

```
Splash Screen
  └→ Login Screen (if not authenticated)
      └→ Dashboard Screen (WARGA role)
```

**Splash Screen**
- Shows app logo and loading indicator.
- Checks secure storage for existing auth session.
- If token exists and valid → Dashboard.
- If token expired → attempt refresh → Dashboard or Login.
- If no token → Login Screen.

**Login Screen**
- Email and password fields.
- Login button.
- "Change password" link (leads to change-password screen).
- On success → Dashboard.
- On failure → Error message (generic).

**Profile Screen**
- Display name, email, phone.
- Buttons: "Change Password", "Logout".

#### 2.1.2 WARGA Dashboard

Shows limited information:
- Household name and house number.
- Current month's bill (status: unpaid/paid/partially paid).
- Outstanding balance.
- Recent payment history.
- Navigation to other screens.

#### 2.1.3 WARGA Bills

- List of own-household bills (all periods).
- Bill status indicators (unpaid, partially paid, paid, overdue).
- Tap a bill → Bill Detail screen.
- **No create/edit/delete capability.**

**Bill Detail (WARGA)**
- Bill amount, due date, period, status.
- Payment history for this bill.
- Outstanding amount.

#### 2.1.4 WARGA Payments

- List of own-household payments (all periods).
- Payment date, amount, method, status.
- **No create/edit/delete capability.**

#### 2.1.5 WARGA Residents

- View all residents in the household (read-only).
- **No create/edit/delete capability.**

#### 2.1.6 WARGA Navigation

**Bottom Navigation Bar:**
1. Dashboard
2. Bills
3. Payments
4. Residents (household)
5. Profile

---

### 2.2 PENGURUS & BENDAHARA (Administrator)

#### 2.2.1 Authentication Flow (Same as Warga)

```
Splash Screen
  └→ Login Screen
      └→ Dashboard Screen (Admin role)
```

#### 2.2.2 Admin Dashboard

Shows comprehensive RT information:
- Total households, residents.
- Monthly income, expense, balance.
- Active bills count, total outstanding.
- Recent transactions.
- Recent payments.
- Quick action buttons.

#### 2.2.3 Household Management

**Household List Screen**
- List of all households in the RT.
- Search by house number or occupant name.
- Filter: active/inactive.
- FAB (Floating Action Button): "New Household".
- Tap → Household Detail.

**Household Detail Screen**
- House number, occupant name, owner name, renter name.
- Address details.
- List of residents in this household.
- Buttons: "Edit Household", "View Bills", "Add Resident".
- PENGURUS only: "Deactivate Household".

**Create/Edit Household Screen**
- Form: house number, occupant name, owner name, renter name, address.
- Save/Cancel.

#### 2.2.4 Resident Management

**Resident List Screen**
- List of all residents (filterable by household).
- Display: name, relationship, gender.
- NIK partially masked.
- FAB: "New Resident".
- Tap → Resident Detail.

**Resident Detail Screen**
- Full name, NIK (masked), gender, birth date.
- Household info.
- Buttons: "Edit", "Deactivate".

**Create/Edit Resident Screen**
- Form: household, NIK, name, gender, birth date.

#### 2.2.5 Financial Categories

**Categories List Screen**
- List of all categories, grouped by type (income, expense).
- FAB: "New Category".
- Edit/Delete per permissions.

#### 2.2.6 Dues Configuration

**Dues Config Screen (Bendahara excluded in MVP)**
- Current dues name and amount.
- "Update" button.
- "Generate Bills" button → selects month/year → confirms → creates bills.

#### 2.2.7 Bills

**Bills List Screen**
- Filterable by: status, household, period.
- Summary: total unpaid, partially paid, paid.
- FAB may be absent (bills generated from Dues screen).

**Bill Detail Screen**
- Household, period, amount, due date, status.
- Payment history with amounts and dates.
- Outstanding balance.

#### 2.2.8 Payments

**Payments List Screen**
- Filterable by: household, payment method, date range.
- Summary: total collected this period.
- FAB: "New Payment".

**New Payment Screen**
- Select bill from household's unpaid bills.
- Outstanding amount displayed (auto-calculated).
- Payment amount (defaults to outstanding).
- Payment date (defaults today).
- Payment method (cash, transfer, other).
- Notes.
- Submit → creates payment + posts financial transaction.

**Payment Detail Screen**
- Full payment info.
- Buttons: "Cancel Payment" (if permissions allow).

#### 2.2.9 Financial Transactions

**Transactions List Screen**
- Filter by: type (income/expense), category, date range, status (draft/posted).
- FAB: "New Transaction".

**New/Edit Transaction Screen**
- Type: income or expense.
- Category (dropdown from configured categories).
- Amount.
- Description.
- Transaction date.
- Submit → creates as draft.
- "Post" button → transitions to posted status.

**Transaction Detail Screen**
- Full transaction info.
- If posted: no edit/cancel; only visible with "View Reversal" option.
- If draft: edit or delete.
- Button to create reversal.

#### 2.2.10 Ledger View

**Ledger Screen**
- Full chronological transaction ledger (all posted transactions).
- Filterable by type, category, date range.
- Running balance (calculated incrementally).
- Read-only.

#### 2.2.11 Reports

**Report Screens**
- Cashflow report (date range selector).
- Household balance report.
- Summary view with totals and breakdown by category.
- Export (deferred to v2).

#### 2.2.12 User Management (Pengurus only)

**Users List Screen**
- List all users in the RT.
- Filter by role.
- FAB: "New User".
- Tap → User Detail.

**User Detail Screen**
- Name, email, phone, role, active status.
- Buttons: "Edit", "Deactivate".
- PENGURUS can edit and create. Cannot change roles.
- BENDAHARA views only.

#### 2.2.13 Audit Logs (PENGURUS & BENDAHARA)

**Audit Log Screen**
- List of all audit events.
- Filterable by: event type, resource type, user, date range.
- Tap → Event Detail showing old/new values.
- Read-only.

#### 2.2.14 Admin Navigation

**Bottom Navigation Bar:**
1. Dashboard
2. Households
3. Residents
4. Finance (tabs: Bills, Payments, Transactions, Ledger)
5. More (Reports, Admin, Profile)

---

## 3. Common UI States

Every screen/section handles these states:

| State | Behavior |
|-------|----------|
| **Loading** | Show circular progress indicator |
| **Empty** | Show empty state with descriptive text and action (if applicable) |
| **Error** | Show error card with retry button; do not crash |
| **Network Offline** | Show offline banner (if internet connectivity checking is implemented) |
| **Auth Expired** | Auto-redirect to login with token refresh fallback |
| **No Permissions** | Show "Access denied" message with option to go back |

---

## 4. Offline Considerations (Deferred)

MVP does not require offline support. The UI should display appropriate network error states when the backend is unavailable. Future versions may add offline data caching.
