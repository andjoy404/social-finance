# Social Finance - Requirements Document

## 1. Introduction

Social Finance is a financial management application for neighborhoods. The application provides transparent financial tracking, bill management, payment recording, and reporting capabilities for administrators and residents.

The system is designed as a SaaS platform capable of serving multiple RT organizations with full tenant isolation, though the initial deployment serves a single RT.

## 2. Target Users

| Role | Description |
|------|-------------|
| **SUPER_ADMIN** | Platform administrator managing multiple RT organizations and platform configuration |
| **PENGURUS** | RT executive managing daily administration and user accounts within an RT |
| **BENDAHARA** | RT treasurer managing financial data: transactions, bills, payments, and reports |
| **WARGA** | RT residents who can view their own household bills, payment history, and limited financial summaries |

## 3. Functional Requirements

### 3.1 MVP v1 Modules (In Scope)

| # | Module | Description |
|---|--------|-------------|
| 1 | Authentication | Login, signup, password management, session handling |
| 2 | RT Management | Create/edit RT entities (SUPER_ADMIN) |
| 3 | User Management | Create/edit users within an RT, assign roles (PENGURUS, BENDAHARA) |
| 4 | Household Management | CRUD households within an RT, manage head of household |
| 5 | Resident Management | CRUD resident records linked to households |
| 6 | Financial Categories | CRUD categories for income and expense classification |
| 7 | RT Dues / Iuran | Define monthly dues structures per RT |
| 8 | Bills | Generate and manage billing for RT dues |
| 9 | Payments | Record payments made by residents (with receipt upload) |
| 10 | Cash Income | Record income transactions not tied to bills |
| 11 | Cash Expenses | Record expense transactions |
| 12 | Financial Ledger | View comprehensive audit-oriented transaction ledger |
| 13 | Dashboard | Overview with summary stats and recent activity |
| 14 | Financial Reports | Summary reports (cashflow, balance summary) |
| 15 | User Profile | View and edit own profile and password |
| 16 | Audit Logs | View system activity log (restricted roles) |

### 3.2 Deferred Modules (v2+)

| # | Module | Reason |
|---|--------|--------|
| 1 | Transaction Attachments / Receipts | Not critical for MVP ledger integrity; deferred to v2 for storage complexity considerations |
| 2 | Advanced Reporting | Basic reports sufficient for MVP; v2+ adds exports, charts, and custom filters |
| 3 | Notification System | Not required for MVP; deferred to v2+ |
| 4 | Mobile Money Integration | Manual payment recording sufficient for MVP; v2+ may add QRIS/link methods |
| 5 | Bulk Bill Generation Optimizations | Manual bill management acceptable for small RT volumes in MVP |
| 6 | Multi-language Support | Indonesian-only for MVP |

## 4. Non-Functional Requirements

### 4.1 Performance

- API response time: < 500ms for 95% of requests
- Support up to 5,000 residents per RT (first deployment limit)
- Dashboard load time: < 2 seconds with up to 5 years of transaction history

### 4.2 Reliability

- Financial data must be immutable once finalized (append-only ledger)
- Database transactions must ensure consistency (ACID compliance)
- Graceful degradation: UI shows cache or error states when backend is unavailable

### 4.3 Security

- HTTPS required in production
- Passwords hashed with Argon2 or bcrypt
- Token-based authentication with short-lived access tokens and refresh tokens
- Role-based access control enforced server-side
- Tenant isolation enforced at every query level
- No secrets in source code or configuration files

### 4.4 Data Integrity

- All monetary values stored with fixed precision (no floating point)
- Financial ledger must preserve audit trail; no silent edits or deletes
- Balance is derived from ledger, never stored as a calculated shortcut

### 4.5 Maintainability

- Modular monolith backend with clear domain boundaries
- Code readable by a small development team
- Migration system for database schema changes
- No unnecessary abstractions or enterprise patterns

### 4.6 Deployment

- Docker-based containerized deployment
- PostgreSQL hosted on VPS
- Environment-based configuration (no hardcoded secrets)
- Automated database backups

## 5. Platform Requirements

### 5.1 Backend

- REST API at `/api/v1/`
- PostgreSQL database
- Deployment on VPS via Docker
- Modular Go backend (or equivalent)

### 5.2 Frontend (Android / Flutter)

- Flutter application (confirmed)
- Runs on Android initially; iOS support possible in v2+
- Secure token storage via flutter_secure_storage (Android Keystore / iOS Keychain)
- Repository/data layer pattern
- Offline-aware UI states
- Communicates exclusively with backend via REST API (JSON)

## 6. Multi-Tenancy Requirements

- Each RT is a separate tenant identified by `rt_id`
- All tenant-scoped data includes `rt_id` for isolation
- `rt_id` is never trusted from client input; derived from authenticated session
- SUPER_ADMIN operates above the tenant level (platform-wide)
