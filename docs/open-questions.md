# Social Finance - Open Questions

## 1. Confirmed Technology Decisions

All major technology decisions have been resolved:

| # | Question | Decision | Status |
|---|----------|----------|--------|
| 1 | Backend language | Go | CONFIRMED |
| 2 | HTTP router | Chi | CONFIRMED |
| 3 | Database | PostgreSQL | CONFIRMED |
| 4 | Android framework | Flutter | CONFIRMED |
| 5 | Local backend runtime | Docker container | CONFIRMED |
| 6 | Local database runtime | Docker container | CONFIRMED |
| 7 | Container orchestration (dev) | Docker Compose | CONFIRMED |
| 8 | Database connection (dev) | Docker service hostname (NOT localhost) | CONFIRMED |
| 9 | Hot reload (dev) | Air | CONFIRMED |
| 10 | Hot reload (prod) | Not applicable — dev only | CONFIRMED |
| 11 | Production build | Multi-stage Docker | CONFIRMED |
| 12 | Android HTTP client | Dio | CONFIRMED |
| 13 | Secrets management | `.env` (git-ignored) | CONFIRMED |

---

## 2. Documented Business Decisions

These questions have been resolved during this architecture phase. See the referenced document for the decision and justification.

| # | Question | Decision | Referenced In |
|---|----------|----------|---------------|
| 1 | Dues charged per household or per resident? | Per household | `business-rules.md` §2.1 |
| 2 | How are incorrect transactions corrected? | Reversal transaction (append-only) | `business-rules.md` §3.1 |
| 3 | Can financial transactions be deleted? | Only drafts. Posted transactions are immutable. | `business-rules.md` §3.1 |
| 4 | Can residents make partial payments? | Yes | `business-rules.md` §2.3 |
| 5 | Can residents overpay? | Yes; overpayment creates household credit. | `business-rules.md` §2.3 |
| 6 | Can a treasurer edit a payment after recording? | Yes (Bendahara can edit their own). Only Pengurus can cancel. | `business-rules.md` §2.2 |
| 7 | Can financial transactions be deleted? | Only `draft` status. Once `posted`, no. | `business-rules.md` §3.1 |
| 8 | Receipts mandatory or optional? | Optional in MVP | `business-rules.md` §4 |
| 9 | Who can see detailed financial info? | WARGA sees own bills/payments only. Others see all. | `business-rules.md` §5 |
| 10 | Billing is per household or per resident? | Per household | `business-rules.md` §2.1 |
| 11 | Multiple active dues configurations? | One active per RT (MVP) | `business-rules.md` §2.1, `database.md` §3.6 |
| 12 | Transaction lifecycle states | Draft → Posted → Cancelled (reversal) | `business-rules.md` §3.1 |
| 13 | Money precision | NUMERIC(15,2) PostgreSQL | `database.md` §1.3 |
| 14 | NIK required for MVP | TBD (see §3 below) | TBD |

---

## 2. Open Questions

These questions require decisions before implementation. Grouped by topic.

### 2.1 SUPER_ADMIN User Creation

**Q:** How are the initial SUPER_ADMIN users created? Is there a signup flow, or are they seeded/provisioned?

**Decision: RESOLVED.** No public signup. SUPER_ADMIN users are created via the bootstrap CLI:
```bash
docker compose run --rm backend server --bootstrap
```
Environment variables: `BOOTSTRAP_EMAIL`, `BOOTSTRAP_PASSWORD`, `BOOTSTRAP_NAME`.
Idempotent: if a super admin already exists, the command is a no-op.

See `cmd/api/main.go:runBootstrap()`.

---

### 2.2 Multiple Active Dues Configurations

**Q:** Can an RT have multiple active dues types simultaneously (e.g., monthly maintenance, security, cleanliness)?

**Current design assumption:** One active dues configuration per RT (simplified MVP).

**Decision pending:** If multiple dues are needed, the `dues` and `bills` tables need modification to allow multiple bill lines per billing period per household.

---

### 2.3 Credit for Overpayment

**Q:** When a resident overpays, can the credit be:
a) Applied automatically to the next bill, or
b) Only manually applied by an administrator?

**Current design assumption:** Automatic to the next bill of the same household.

**Decision pending:** Confirm approach. Option (b) gives administrators more control but adds complexity.

---

### 2.4 Bill Cancellation

**Q:** Can bills be cancelled after they are active? When might this be needed?

**Current design assumption:** Bills can be deleted only in `draft` status. Once `active`, cancelling requires a financial reversal transaction instead (the payment already posted becomes a negative balance).

**Decision pending:** Should there be a dedicated "cancel bill" action for `active` bills (would also need to cancel all associated payments)?

---

### 2.5 Password Requirements

**Q:** What password complexity requirements should be enforced?

**Current design assumption:** Minimum 8 characters, no special requirements.

**Decision pending:** Define requirements:
- Minimum length
- Maximum length
- Required character types (uppercase, lowercase, number, special)
- Password history (prevent reuse of last N passwords)
- Password expiration policy (rarely needed for this scale)

---

### 2.6 Session Inactivity Timeout

**Q:** Should sessions auto-expire after inactivity, or only at the access token expiration (15 minutes)?

**Current design assumption:** 15-minute access token + 7-day refresh token covers the auth lifecycle. No inactivity timeout.

**Decision pending:** Is a stricter inactivity timeout needed for audit or security reasons?

---

### 2.7 Beginning Balance / Opening Balance

**Q:** How should existing RT finances be entered when onboarding an RT?

**Current design assumption:** An administrator can create manual income and expense transactions with historical dates to establish a starting point. The ledger will reflect the correct balance.

**Decision pending:** Should there be a dedicated "opening balance" transaction type or wizard to simplify this migration step?

---

### 2.8 Export Formats

**Q:** What export formats should financial reports support?

**Current design assumption:** Not in MVP. Export is deferred to v2.

**Decision pending:** For v2, should we support:
a) CSV only
b) CSV + PDF
c) CSV + Excel (XLSX)

---

### 2.9 Household Capacity

**Q:** What is the practical limit of residents per household?

**Current design assumption:** Unlimited in the database. No enforcement beyond reasonableness.

**Decision pending:** Consider adding a practical ceiling (e.g., 20 residents per household) for UI layout and data entry validation purposes.

---

### 2.10 Audit Log Retention

**Q:** How long should audit logs be retained?

**Current design assumption:** Indefinite retention (no archival or deletion policy in MVP).

**Decision pending:** For a financial system, indefinite retention may be legally appropriate. Confirm with stakeholders.

---

### 2.11 Duplicate Transaction Prevention

**Q:** Should there be additional duplicate detection beyond idempotency keys?

**Current design assumption:** Idempotency keys handle the client retry case. No further duplicate detection in MVP.

**Decision pending:** Should the system detect and warn about potentially duplicate transactions manually (e.g., same category, same amount, same date)?

---

### 2.12 Phone Number Format

**Q:** What format should phone numbers be stored and displayed in?

**Current design assumption:** Stored as free text. Displayed as-is.

**Decision pending:** Enforce a standard format (e.g., `+62xxxxxxxxxx` or `0xxxxxxxxxx`) for consistency.

---

### 2.13 Multi-RW Support

**Q:** Can one RT span multiple RWs?

**Current design assumption:** Each RT has one RW number. A "larger RT" is simply a different RT entity in the system.

**Decision pending:** In some Indonesian neighborhoods, administrative boundaries may not align neatly. Confirm if this edge case is relevant.

---

### 2.14 Role Name Localization

**Q:** Should user roles have localized display names?

**Current design assumption:** Indonesian role names only (Pengurus, Bendahara, Warga) since the system is for Indonesian RTs only in MVP.

**Decision pending:** If v2 adds English interface, roles need display name mapping.

---

### 2.15 File Upload Storage Strategy

**Q:** Where should receipt/attachment files be stored?

**Current design assumption:** Deferred to v2. MVP has no attachments on transactions.

**Decision pending for v2 planning:**
a) VPS local filesystem (simpler, limited scalability)
b) Cloud object storage (S3-compatible, more complex but scalable)
c) Database `BYTEA` column (only for very small receipts, not recommended)

---

## 3. Questions to Stakeholders

These require input from the RT administrators or community:

1. **Opening balance:** Do you have existing financial records that need to be migrated? (Q2.7)
2. **Password policy:** Do you have any password requirements from organizational policy? (Q2.5)
3. **Overpayment credit:** Should overpayment be auto-applied or administrator-controlled? (Q2.3)
4. **Bill cancellation:** How often do bills need to be cancelled after being sent? (Q2.4)
5. **Audit retention:** Is there a legal or organizational policy on how long financial records must be retained? (Q2.10)

---

## 4. Billing, Arrears & Operations — Open Decisions

These decisions are relevant to the approved future roadmap (Finance.1, Billing.1, Billing.2, Arrears.1, Service.1, Service.2, Web.1, Web.2, Ops.1, Ops.2). All items below are **planned / not implemented**.

### 4.1 Partial Payment and Overdue-Month Counting

**Q:** How should a `PARTIAL` payment affect overdue-month counting?

**Working assumption:** Any bill that is not fully paid and past its due date counts as an overdue month.

**Decision pending:** Should partial payments reduce the overdue-month count, or only `PAID` bills?

---

### 4.2 Payment Allocation Order

**Q:** When allocating a payment to outstanding bills, which bill receives credit first?

Options under consideration:
- Oldest outstanding bill first (FIFO)
- User selects which bill during payment
- Automatic allocation by an agreed rule

**Decision pending.**

---

### 4.3 Overdue Threshold — Consecutive vs. Any

**Q:** Does the garbage-collection suspension threshold of 3 months mean:
- Any 3 overdue monthly bills (non-consecutive allowed), or
- 3 consecutive overdue monthly bills?

**Working assumption:** 3 monthly bills that have passed their due date and are not fully paid, regardless of consecutiveness.

**Final policy remains pending confirmation.**

---

### 4.4 Vacant Houses and Billing

**Q:** Should vacant (unoccupied) houses receive monthly bills?

**Decision pending.** Possible approaches:
- Bill regardless of occupancy
- No bill for vacant houses
- Reduced rate for vacant houses

---

### 4.5 Temporary Service Exemption

**Q:** Can Pengurus grant a temporary exemption from garbage collection suspension for a household with >= 3 overdue months?

**Decision pending.** If yes, should the exemption have:
- No expiry (permanent until manually revoked)
- An expiry date (temporary dispensation)

---

### 4.6 Exemption Expiry Dates

**Q:** If temporary exemptions are allowed, should expired exemptions automatically revert the household to SUSPENDED status?

**Decision pending.**

---

### 4.7 Historical Tariff Changes

**Q:** How should historical billing represented when the monthly fee amount changes during the year?

**Options under consideration:**
- Dues table with effective dates (historical amounts preserved)
- Separate dues records per period
- Other

**Decision pending.**

---

### 4.8 Overpayments and Credits

**Q:** How are overpayments/credits handled in the billing system?

- Applied automatically to the next bill?
- Stored as a household credit balance?
- Manual administrator action required?

**Decision pending.**

---

### 4.9 Advance Payments for Future Months

**Q:** Can a household pay for multiple future months in advance?

**Decision pending.** If yes, should these advance payments appear as "PAID" on future bills, or as a separate credit line?

---

### 4.10 Operator Data Visibility

**Q:** What financial and resident information, if any, may Operator accounts see?

**Working principle (least privilege):**
- Operator may see house number and head name for operational identification
- Operator should see service eligibility status (ANGKUT / JANGAN ANGKUT)
- Operator should NOT see: RT cash balance, unrelated financial transactions, NIK, sensitive household information

**Decision pending.** A formal RBAC design for the `operator` role is required.

---

### 4.11 Tenant-Configurable Transaction Categories

**Q:** Should financial categories (income/expense) eventually be tenant-configurable?

**Current assumption:** Categories are pre-defined for MVP. Future RTs may want custom categories.

**Decision pending.** Consider this for later phases but not required for initial implementation.

---

### 4.12 Event / Activity Accounting

**Q:** Should activity/event accounting (grouping income/expenses by event like "HUT RI 2027") eventually become a separate module?

**Working assumption:** General transaction categorization must not prevent this future capability. Full event accounting is NOT in scope for this roadmap phase.

**Decision pending.** Marked as a future possibility only.
