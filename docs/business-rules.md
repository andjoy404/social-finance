# Social Finance - Business Rules

## 1. User and Household Relationships

### 1.1 Household Members and Application Users

**Rule:** One household can have multiple application users.

**Justification:** Multiple adult residents within a household may need independent login accounts (e.g., spouses, adult children living at home).

**Rule:** One user belongs to exactly one primary household.

**Justification:** Simplifies permission and billing assignment. A user can be a household member of only one household at a time.

**Rule:** A user who moves to a new household must be reassigned by a Pengurus or Bendahara; residents cannot self-transfer households from the UI.

### 1.2 Residents vs Application Users

**Rule:** Residents can exist without application user accounts.

**Justification:** Not all residents (children, elderly without smartphones) will have accounts. They are household members tracked for completeness but cannot log in.

**Rule:** To create an application user, a resident must be linked to a household.

## 1.3 Resident Movement

**Rule:** Moving into the RT: A Pengurus or Bendahara creates a new household and/or adds residents to an existing household.

**Rule:** Moving out of the RT: A Pengurus or Bendahara marks a resident as "out" (inactive) with a move-out date. The resident record is preserved for historical records. Moving a user to a different household is not supported; instead, deactivate the old user account and create a new one tied to the new household.

**Rule:** Deactivated residents (moved out) remain visible in audit logs and historical reports but are excluded from active bill generation.

---

## 2. Dues and Billing Rules

### 2.1 Dues Structure

**Rule:** Dues (Iuran) are charged per household, not per resident.

**Justification:** Simplifies billing and aligns with standard RT/RT community practice where maintenance fees are per household.

**Rule:** Each RT defines its own dues structure (monthly, periodic) with a base amount.

**Rule:** The base amount can be uniform across all households or customized per household by administrators (e.g., special arrangements).

**Rule:** Dues are defined at the RT level and apply to all households unless overridden.

### 2.2 Bill Generation

**Rule:** Bills are generated periodically (e.g., monthly) from the active dues configuration.

**Rule:** Each bill is tied to a single household for a specific billing period.

**Rule:** Bills have a generation date, due date, amount, and status (unpaid, partially paid, paid, overdue).

**Rule:** Bills are created as Drafts by the system (via a scheduled job or manual trigger) and become Active when finalized.

### 2.3 Partial Payments and Overpayment

**Rule:** Residents can make partial payments.

**Justification:** Some residents may not be able to pay the full amount at once.

**Rule:** Partial payments create a payment record with the paid amount and reduce the outstanding balance on the bill.

**Rule:** Residents can overpay (pay more than the bill amount).

**Rule:** Overpayment creates a credit for the household that can be applied to future bills by default, unless specified otherwise by the administrator.

**Rule:** A bill reaches "paid" status only when the total paid amount equals or exceeds the bill amount.

### 2.4 Outstanding Balance Calculation

**Rule:** Outstanding balance = Bill Amount - Total Payments Applied - Credit Applied.

**Rule:** The system calculates outstanding balances dynamically from payment records; no separate "balance" field is stored.

**Rule:** For household-level summaries, sum of all unpaired bill portions and un-reversed credit creates the household net position.

### 2.5 Late Payments

**Rule:** Bills marked as overdue after the due date.

**Rule:** Late fees are configurable per RT but are not implemented in MVP. A TODO/v2 item.

**Rule:** Overdue status is calculated based on the current date vs. due date; no permanent "late fee" state is stored.

---

## 3. Financial Transaction Rules

### 3.1 Transaction Lifecycle

**Decision: Simple 3-State Model with Reversal**

The system uses a simple transaction lifecycle:

| Status | Description |
|--------|-------------|
| `draft` | Created but not yet finalized. Can be edited or deleted freely. |
| `posted` | Finalized. Locked from editing or deletion. Appends to ledger. |
| `cancelled` | A separate transaction marked as cancelled (reversal). Does not delete the original. |

**Decision: Reversal Instead of Correction**

Once a transaction is posted:
- It **cannot be edited or deleted**.
- If an error is detected, a new correction transaction is created as a reversal with opposite sign.
- The reversal transaction references the original transaction.
- Both the original and reversal remain visible in the ledger and audit log.

**Justification:** This balances audit integrity with practical usability. For a small RT application, a full multi-stage approval workflow would add unnecessary complexity. The 3-state model with a reversal requirement preserves the complete audit trail while keeping operations simple.

**Decision: No Approval Workflow in MVP**

In MVP, the distinction between draft and posted is enforced by role:
- Only BENDAHARA, PENGURUS, and SUPER_ADMIN can post transactions.
- Draft transactions can be created and edited by any of these roles.
- Once posted by any authorized user, the transaction is locked.

### 3.2 Financial Integrity Principles

1. **No editable balance stored.** Balance is always calculated from posted transactions.
2. **Append-only ledger.** No DELETE on posted transactions.
3. **Signed amounts.** Income and expense are represented by a single `amount` column with a `type` field (`income` or `expense`).
4. **Fixed-precision only.** All monetary values use `NUMERIC(15, 2)` in PostgreSQL. Never `REAL`, `DOUBLE PRECISION`, or `FLOAT`.
5. **Immutability enforced at the database level.** Check constraints, triggers, or application logic prevent silent modification.
6. **Every posted transaction is audited.** See Section 8.

---

## 4. Receipts and Attachments

**Decision (MVP): Receipts are optional for financial transactions.**

- Users can optionally attach a receipt image when creating income or expense transactions.
- The attachment is not required to post a transaction.
- Receipt storage is deferred to v2 (cloud storage or blob column).

---

## 5. Data Visibility Rules

### 5.1 By Role

| Data Type | SUPER_ADMIN | PENGURUS | BENDAHARA | WARGA |
|-----------|-------------|----------|-----------|-------|
| All RT data (platform) | Yes | No | No | No |
| Own RT household data | Yes | Yes | Yes | Own household only |
| Own RT resident data | Yes | Yes | Yes | All residents (read-only) |
| Own RT financial data | Yes | Yes | Full access | Own bills and payments only |
| Audit logs | Yes | Yes | Yes | No |
| RT dues config | Yes | Yes | Yes | No |
| Financial reports | Yes | Yes | Yes | Summary only |

### 5.2 Sensitive Information

**Rule:** NIK (National ID number) is stored but:
- NOT displayed to WARGA role.
- Partially masked (e.g., `************1234`) to PENGURUS and BENDAHARA.
- Fully visible only to SUPER_ADMIN.
- NEVER logged, exported, or exposed in API error messages.

---

## 6. Permissions Matrix

### 6.1 Detailed Permissions

#### Authentication
| Action | SUPER_ADMIN | PENGURUS | BENDAHARA | WARGA |
|--------|-------------|----------|-----------|-------|
| Login | Yes | Yes | Yes | Yes |
| Logout | Yes | Yes | Yes | Yes |
| Reset own password | Yes | Yes | Yes | Yes |

#### RT Management
| Action | SUPER_ADMIN | PENGURUS | BENDAHARA | WARGA |
|--------|-------------|----------|-----------|-------|
| Create RT | Yes | No | No | No |
| View own RT | Yes | Yes | Yes | Yes |
| Edit own RT | Yes | Yes | No | No |
| Delete RT | Yes | No | No | No |
| View other RTs (platform) | Yes | No | No | No |

#### User Management (within RT)
| Action | SUPER_ADMIN | PENGURUS | BENDAHARA | WARGA |
|--------|-------------|----------|-----------|-------|
| View users | Yes | Yes | Yes | Own profile only |
| Create user | Yes | Yes | No | Self only (register) |
| Edit own profile | Yes | Yes | Yes | Yes |
| Edit other users | Yes | Yes | No | No |
| Delete/Deactivate users | Yes | Yes | No | No |
| Change roles | Yes | No | No | No |

#### Household Management
| Action | SUPER_ADMIN | PENGURUS | BENDAHARA | WARGA |
|--------|-------------|----------|-----------|-------|
| View households | Yes | Yes | Yes | Own household only |
| Create household | Yes | Yes | Yes | No |
| Edit household | Yes | Yes | Yes | No |
| Delete household | Yes | Yes | No | No |
| Assign head of household | Yes | Yes | Yes | No |

#### Resident Management
| Action | SUPER_ADMIN | PENGURUS | BENDAHARA | WARGA |
|--------|-------------|----------|-----------|-------|
| View residents | Yes | Yes | Yes | All (read-only) |
| Create resident | Yes | Yes | Yes | No |
| Edit resident | Yes | Yes | Yes | No |
| Delete resident | Yes | Yes | Yes | No |

#### Financial Categories
| Action | SUPER_ADMIN | PENGURUS | BENDAHARA | WARGA |
|--------|-------------|----------|-----------|-------|
| View categories | Yes | Yes | Yes | No |
| Create categories | Yes | Yes | Yes | No |
| Edit categories | Yes | Yes | Yes | No |
| Delete categories | Yes | Pengurus only (Bendahara cannot) | No | No |

#### RT Dues
| Action | SUPER_ADMIN | PENGURUS | BENDAHARA | WARGA |
|--------|-------------|----------|-----------|-------|
| View dues config | Yes | Yes | Yes | No |
| Create/Modify dues | Yes | Yes | Yes | No |
| Delete dues config | Yes | Pengurus only | No | No |
| Generate bills | Yes | Yes | Yes | No |

#### Bills
| Action | SUPER_ADMIN | PENGURUS | BENDAHARA | WARGA |
|--------|-------------|----------|-----------|-------|
| View bills | Yes | All bills | All bills | Own bills only |
| Create bills | Yes | Yes | Yes | No |
| Edit bills | Yes | Yes | Yes | No |
| Delete bills | Yes | Pengurus only | No | No |

#### Payments
| Action | SUPER_ADMIN | PENGURUS | BENDAHARA | WARGA |
|--------|-------------|----------|-----------|-------|
| View payments | Yes | All payments | All payments | Own payments only |
| Create payments | Yes | Yes | Yes | No |
| Edit payments | Yes | Pengurus only | Yes | No |
| Cancel payments | Yes | Pengurus only | Pengurus only | No |

#### Financial Transactions (Income / Expense)
| Action | SUPER_ADMIN | PENGURUS | BENDAHARA | WARGA |
|--------|-------------|----------|-----------|-------|
| View transactions | Yes | All transactions | All transactions | No access |
| Create transactions (draft) | Yes | Yes | Yes | No |
| Post transactions | Yes | Yes | Yes | No |
| Edit draft transactions | Yes | Yes | Yes | No |
| Create reversal | Yes | Yes | Yes | No |
| Delete draft transactions | Yes | Yes | Yes | No |

#### Dashboard
| Action | SUPER_ADMIN | PENGURUS | BENDAHARA | WARGA |
|--------|-------------|----------|-----------|-------|
| View dashboard | Yes | Own RT dashboard | Own RT dashboard | Limited (own account) |

#### Financial Reports
| Action | SUPER_ADMIN | PENGURUS | BENDAHARA | WARGA |
|--------|-------------|----------|-----------|-------|
| View full reports | Yes | Yes | Yes | Summary only (own) |
| Export reports | Yes | Yes | Yes | No |

#### Audit Logs
| Action | SUPER_ADMIN | PENGURUS | BENDAHARA | WARGA |
|--------|-------------|----------|-----------|-------|
| View audit logs | Yes | Yes | Yes | No |

#### User Profile
| Action | SUPER_ADMIN | PENGURUS | BENDAHARA | WARGA |
|--------|-------------|----------|-----------|-------|
| View own profile | Yes | Yes | Yes | Yes |
| Edit own profile | Yes | Yes | Yes | Yes |
| Change own password | Yes | Yes | Yes | Yes |
| Delete own account | Yes | Yes | Yes | Yes (with restrictions) |

---

## 7. Audit Trail Rules

### 7.1 Events Requiring Audit Records

Every event below creates an `audit_logs` record:

| Event | Description |
|-------|-------------|
| `user.login_success` | Successful user login |
| `user.login_failed` | Failed login attempt (reason masked, no sensitive data) |
| `user.created` | New application user created |
| `user.updated` | User profile or role changed |
| `user.deleted` | User deactivated or removed |
| `user.role_changed` | User role modified |
| `household.created` | Household created |
| `household.updated` | Household data changed |
| `household.deleted` | Household removed |
| `resident.created` | Resident added to household |
| `resident.updated` | Resident data changed |
| `resident.deleted` | Resident removed |
| `bill.created` | Bill generated (single household) |
| `bill.generated` | Bulk bill generation from dues config |
| `bill.updated` | Bill edited (draft state only) |
| `bill.deleted` | Bill deleted (draft state only) |
| `payment.created` | Payment recorded |
| `payment.updated` | Payment edited (restricted;Pengurus only) |
| `payment.cancelled` | Payment reversed |
| `transaction.created` | Financial transaction created (draft) |
| `transaction.posted` | Financial transaction posted |
| `transaction.edited` | Draft transaction edited |
| `transaction.deleted` | Draft transaction deleted |
| `transaction.reversed` | Correction reversal transaction created |
| `category.created` | Financial category created |
| `category.updated` | Financial category modified |
| `category.deleted` | Financial category removed |
| `dues.updated` | Dues configuration changed |
| `dues.generated` | Bulk bills generated from dues config |
| `rt.updated` | RT data modified |
| `idempotency_key_matched` | Cached idempotency key replayed on retry |

**Note on `bill.created`:** Also logged via the related `dues.updated` event type when bills are bulk-generated. Audit logs include both `bill.created` (per-bill) and `dues.generated` (bulk batch) for traceability.

### 7.2 Audit Record Contents

Each audit log entry captures:
- `id` (PK)
- `rt_id` (tenant scope)
- `user_id` (who performed the action)
- `event_type` (category of event)
- `resource_type` (e.g., `user`, `bill`, `transaction`)
- `resource_id` (ID of the affected record)
- `old_values` (JSONB snapshot before change, null for create events)
- `new_values` (JSONB snapshot after change, null for delete events)
- `ip_address` (client IP, stored as string)
- `user_agent` (client identifier, stored as string)
- `created_at` (timestamp)

### 7.3 What is NEVER Logged

- Passwords or password hashes
- Access tokens or refresh tokens
- Full NIK numbers (masked if included)
- Authentication credentials
- File attachment contents (only metadata)
