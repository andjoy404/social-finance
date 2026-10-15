# Laporan Audit Implementasi KAS — Deep Analysis & Period Balance

**Tanggal:** 2026-10-15  
**Status:** READ-ONLY AUDIT — Tidak ada perubahan kode atau database  
**Basis:** Penelusuran menyeluruh source code backend (Go), Web (React/TypeScript), dan Android (Flutter)

---

## A. Ringkasan Eksekutif

### Kondisi Saat Ini

Modul KAS (Kas Umum RT) backend memiliki fondasi ledger yang **berfungsi dengan baik** untuk pencatatan transaksi dan integrasi otomatis dari pembayaran Iuran. Namun, tampilan UI KAS di Web dan Android **masih seluruhnya menggunakan mock data** tanpa integrasi API. Perhitungan saldo saat ini bersifat **kumulatif keseluruhan** — tidak mendukung saldo awal/akhir per periode, yang merupakan fitur utama yang dibutuhkan untuk tampilan neraca kas.

### Temuan Utama

| Area | Status | Keterangan |
|------|--------|------------|
| Transactions table (ledger) | ✅ IMPLEMENTED | Migration 009, tabel siap digunakan |
| Balance calculation (cumulative) | ✅ IMPLEMENTED | `CalculateBalance` di repository.go:1078 |
| Iuran → KAS integration | ✅ IMPLEMENTED | Payment approval → auto create KAS transaction |
| Atomicity (payment + KAS) | ✅ IMPLEMENTED | Semua operasi dalam satu PostgreSQL transaction |
| Reversal mechanism | ✅ IMPLEMENTED | `ReverseTransaction` creates compensating entry |
| Idempotency | ✅ IMPLEMENTED | Advisory locks + `idempotency_keys` table |
| Tenant isolation | ✅ IMPLEMENTED | JWT-derived `rt_id`, never client-supplied |
| Manual transaction CRUD | ✅ IMPLEMENTED | Create, list, get, reverse via API |
| Balance per period | ❌ NOT_IMPLEMENTED | `CalculateBalance` only returns cumulative total |
| Opening balance auto-calc | ❌ NOT_IMPLEMENTED | No code to compute opening balance per period |
| Web KAS neraca table | ❌ MOCK_ONLY | 6 files, all hardcoded mock data |
| Android KAS UI | ❌ MOCK_ONLY | 7 files, all hardcoded mock data |
| Period balance API endpoint | ❌ NOT_IMPLEMENTED | Balance endpoint accepts no period parameters |
| Cross-period reversal tests | ❌ NOT_TESTED | Only same-period reversal tested |

### Gap Prioritas

1. **High:** Implement period-based balance calculation — fondasi untuk neraca kas
2. **High:** Integrate Web KAS UI with real backend API
3. **High:** Integrate Android KAS UI with real backend API
4. **Medium:** Period balance tests (opening/closing, cross-period reversal)
5. **Medium:** Seed default financial categories (income + expense)

---

## B. Bukti Source Code — Backend KAS

### 1. Schema Ledger: `transactions` Table

**File:** `backend/migrations/009_finance.up.sql` (lines 1-80)

```sql
CREATE TABLE transactions (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    rt_id uuid NOT NULL REFERENCES rts(id),
    category_id uuid NOT NULL REFERENCES financial_categories(id),
    amount NUMERIC(15,2) CHECK (amount > 0),
    type text CHECK (type IN ('income', 'expense')),
    status text CHECK (status IN ('draft', 'posted', 'cancelled')) DEFAULT 'posted',
    reverses_transaction_id uuid REFERENCES transactions(id),
    payment_id uuid REFERENCES payments(id),
    description text NOT NULL,
    occurred_at timestamptz,
    created_at timestamptz DEFAULT now(),
    updated_at timestamptz DEFAULT now()
);
```

**Konfirmasi:**
- ✅ `NUMERIC(15,2)` — fixed-precision, NO floating point
- ✅ `CHECK (amount > 0)` — amount always positive, type distinguishes income/expense
- ✅ `CHECK (status IN (...))` — 3-state lifecycle enforced at DB level
- ✅ `reverses_transaction_id FK` — reversal references original
- ✅ `payment_id FK` — traceable to payment source
- ✅ Indexes on `rt_id`, `category_id`, `type`, `status`, `occurred_at`
- ❌ **NO `source_type` or `source_id` columns** — blueprint-defined but never implemented
- ❌ **NO unique constraint** preventing duplicate payment → transaction (idempotency handled separately via `idempotency_keys` table)

**Model mapping** (`backend/internal/finance/model.go:268-284`):

```go
type Transaction struct {
    ID                    string            `json:"id"`
    RTID                  string            `json:"rt_id"`
    CategoryID            string            `json:"category_id"`
    Amount                string            `json:"amount"`
    Type                  TransactionType   `json:"type"`
    Status                TransactionStatus `json:"status"`
    ReversesTransactionID *string           `json:"reverses_transaction_id,omitempty"`
    PaymentID             *string           `json:"payment_id,omitempty"`
    Description           string            `json:"description"`
    OccurredAt            time.Time         `json:"occurred_at"`
    CreatedAt             time.Time         `json:"created_at"`
    UpdatedAt             time.Time         `json:"updated_at"`
    CategoryName          *string           `json:"category_name,omitempty"`
}
```

Tidak ada field `source_type` atau `source_id`.

---

### 2. Balance Calculation — Cumulative Only

**File:** `backend/internal/finance/repository.go` (lines 1078-1108)

```go
func CalculateBalance(ctx context.Context, tx *sql.Tx, rtID string) (*BalanceSummary, error) {
    var incomeStr, expenseStr string
    var count int

    err := tx.QueryRowContext(ctx,
        `SELECT COALESCE(SUM(CASE WHEN type = 'income' THEN amount ELSE 0 END), 0)::text as total_income,
                COALESCE(SUM(CASE WHEN type = 'expense' THEN amount ELSE 0 END), 0)::text as total_expense,
                COUNT(*) as tx_count
         FROM transactions
         WHERE rt_id = $1 AND status = 'posted'`,
        rtID,
    ).Scan(&incomeStr, &expenseStr, &count)
    // ... ParseMoney for fixed-precision
}
```

**Response model** (`model.go:309-315`):
```go
type BalanceSummary struct {
    TotalIncome      string `json:"total_income"`
    TotalExpense     string `json:"total_expense"`
    NetBalance       string `json:"net_balance"`
    TransactionCount int    `json:"transaction_count"`
}
```

**Yang ADA:**
- ✅ Per-`rt_id` filter — tenant isolation
- ✅ Only `status = 'posted'` — excludes draft, cancelled, and their reversals
- ✅ Fixed-precision via `ParseMoney()` / `FormatMoney()`
- ✅ Net balance = TotalIncome - TotalExpense

**Yang TIDAK ADA:**
- ❌ **No period parameters** — cannot filter by date range
- ❌ **No opening balance calculation** — cannot compute balance at period start
- ❌ **No closing balance** — only cumulative net, not period-specific
- ❌ **No running balance** — cumulative only, no per-transaction balance
- ❌ Balance endpoint accepts NO query parameters for period filtering

**Cross-RT version** (`repository.go:1705-1729`):
```go
func CalculateBalanceAll(ctx context.Context, tx *sql.Tx) (*BalanceSummary, error) {
    // Same query, WITHOUT rt_id filter — for SUPER_ADMIN
}
```

---

### 3. Integrasi Iuran → KAS — Verified Working

**File:** `backend/internal/finance/service.go`

Setiap pembayaran yang disetujui otomatis membuat transaksi KAS income dalam satu PostgreSQL transaction:

#### 3a. Staff Auto-Approve (CreatePayment)

**File:** `service.go` (lines 319-366)

```go
// When userRole != "warga" (pengurus/bendahara record payment manually)
if status == PaymentStatusApproved {
    // ... validation, PaymentCreate, BillUpdateStatus ...

    // Post income transaction into ledger
    catID, err := FindOrCreateDefaultIncomeCategory(ctx, tx, rtID, "Iuran Warga")
    if err != nil {
        return nil, err
    }
    desc := fmt.Sprintf("Payment for bill %s (Period: %s)", bill.ID, bill.Period)
    _, err = TransactionCreate(ctx, tx, rtID, CreateTransactionInput{
        CategoryID:  catID,
        Amount:      payment.Amount,
        Type:        TransactionTypeIncome,
        Description: desc,
    }, TransactionStatusPosted, nil, &payment.ID, now)
}
```

**Atomicity:** PaymentCreate + BillUpdateStatus + TransactionCreate dalam satu `tx *sql.Tx`. Commit atau rollback bersama-sama.

#### 3b. Warga Submit → Bendahara Approve (VerifyPayment → verifySinglePayment)

**File:** `service.go` (lines 568-602)

```go
func (s *Service) verifySinglePayment(...) (*VerifyPaymentResult, error) {
    // ... lock bill, validate overpayment ...
    
    verifiedPayment, err := PaymentVerify(ctx, tx, ...) // update to APPROVED
    
    // Automatically post income transaction into ledger
    catID, err := FindOrCreateDefaultIncomeCategory(ctx, tx, rtID, "Iuran Warga")
    _, err = TransactionCreate(ctx, tx, rtID, CreateTransactionInput{
        CategoryID:  catID,
        Amount:      verifiedPayment.Amount,
        Type:        TransactionTypeIncome,
        Description: desc,
    }, TransactionStatusPosted, nil, &verifiedPayment.ID, now)
}
```

#### 3c. Donation via `due_id` (CreatePayment)

**File:** `service.go` (lines 273-282)

```go
if in.DueID != nil {
    // ... validate voluntary due ...
    if status == PaymentStatusApproved {
        if err := postPaymentIncome(ctx, tx, rtID, payment, 
            fmt.Sprintf("Voluntary donation for due %s", due.Name)); err != nil {
            return nil, err
        }
    }
}
```

#### 3d. Donation Approval (VerifyPayment)

**File:** `service.go` (lines 516-530)

```go
if payment.BillID == nil {
    // Donation path
    verifiedPayment, err := PaymentVerify(ctx, tx, paymentID, rtID, verifierID, PaymentStatusApproved, "", officerNotes)
    if err := postPaymentIncome(ctx, tx, rtID, verifiedPayment, 
        fmt.Sprintf("Voluntary donation for due %s", due.Name)); err != nil {
        return nil, err
    }
}
```

#### 3e. Multi-Period Payment

**File:** `service.go` (lines 891-907, 1130-1147)

Satu transaksi KAS untuk seluruh payment (bukan per bill):
```go
desc := fmt.Sprintf("Multi-period payment: %s", strings.Join(periods, ", "))
_, err = TransactionCreate(ctx, tx, rtID, CreateTransactionInput{
    CategoryID:  catID,
    Amount:      payment.Amount,  // TOTAL amount, not per-bill
    Type:        TransactionTypeIncome,
    Description: desc,
}, TransactionStatusPosted, nil, &payment.ID, now)
```

#### 3f. Rejection — NO KAS Transaction

**File:** `service.go` (lines 538-565)

Rejected payments (overpayment or bendahara reject):
```go
if action == "reject" {
    verifiedPayment, err := PaymentVerify(ctx, tx, paymentID, rtID, verifierID, PaymentStatusRejected, reason, officerNotes)
    // NO TransactionCreate call — payment rejected = no KAS entry
    return &VerifyPaymentResult{Payment: verifiedPayment}, nil
}
```

**Atomicity verified:** Semua operasi dalam satu transaction yang sama. Jika TransactionCreate gagal, seluruh transaction di-roll-back — payment status kembali ke PENDING.

---

### 4. Manual Transaction

**File:** `service.go` (lines 922-958)

```go
func (s *Service) CreateManualTransaction(ctx context.Context, tx *sql.Tx, rtID, userID string, in CreateTransactionInput) (*Transaction, error) {
    if err := in.Validate(); err != nil {
        return nil, err
    }
    cat, err := CategoryGetByID(ctx, tx, in.CategoryID, rtID)
    if err != nil {
        return nil, fmt.Errorf("category not found: %w", err)
    }
    if !cat.IsActive {
        return nil, errors.New("cannot create transaction for inactive category")
    }
    trans, err := TransactionCreate(ctx, tx, rtID, in, TransactionStatusPosted, nil, nil, occurredAt)
    // ... audit log ...
    return trans, nil
}
```

Status selalu `posted`. Tidak ada draft workflow untuk manual transaction.

---

### 5. Reversal

**File:** `service.go` (lines 960-1012)

```go
func (s *Service) ReverseTransaction(ctx context.Context, tx *sql.Tx, rtID, userID, origID string, in ReverseTransactionInput) (*Transaction, error) {
    orig, err := TransactionGetByID(ctx, tx, origID, rtID)
    if orig.Status != TransactionStatusPosted {
        return nil, ErrCannotReverseNonPost
    }
    if orig.ReversesTransactionID != nil {
        return nil, errors.New("cannot reverse a reversal transaction")
    }
    // Check already reversed
    var alreadyReversedCount int
    tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM transactions WHERE reverses_transaction_id = $1 AND rt_id = $2`, orig.ID, rtID).Scan(&alreadyReversedCount)
    if alreadyReversedCount > 0 {
        return nil, ErrAlreadyReversed
    }

    // Create opposite-type transaction
    var revType TransactionType
    if orig.Type == TransactionTypeIncome {
        revType = TransactionTypeExpense
    } else {
        revType = TransactionTypeIncome
    }

    reversal, err := TransactionCreate(ctx, tx, rtID, CreateTransactionInput{
        CategoryID:  orig.CategoryID,
        Amount:      orig.Amount,       // Same amount
        Type:        revType,           // Opposite type
        Description: desc,
    }, TransactionStatusPosted, &orig.ID, orig.PaymentID, now)
}
```

**Semantik reversal:**
- ✅ Original transaction REMAINS `posted` — never deleted or modified
- ✅ Reversal creates opposite-type transaction (income → expense, expense → income)
- ✅ Same amount — net effect = zero
- ✅ `reverses_transaction_id` set to original's ID
- ✅ `payment_id` copied from original (traceability preserved)
- ✅ Cannot reverse already-reversed transaction
- ✅ Cannot reverse non-posted transaction
- ✅ Cannot reverse twice

---

### 6. Idempotency

**File:** `backend/internal/finance/idempotency.go`

Mechanism menggunakan PostgreSQL advisory lock:
1. Request dengan `Idempotency-Key` header
2. `pg_try_advisory_lock(lockID)` — serializes across API instances
3. Cek `idempotency_keys` table — jika key sudah ada dengan request body sama, return cached response
4. Jika baru, simpan response setelah operasi berhasil
5. Release lock

**Protected endpoints:**
- `POST /api/v1/transactions` — CreateTransaction
- `POST /api/v1/transactions/{id}/reverse` — ReverseTransaction
- `POST /api/v1/payments` — CreatePayment (multipart)
- `POST /api/v1/bills/generate` — GenerateBills

**Response validation:** `user_id`, `request_path`, `request_hash` — memastikan key tidak digunakan untuk request berbeda.

---

### 7. Tenant Isolation

**Handler middleware** (`handler.go`):

```go
func (h *Handler) ListTransactions(w http.ResponseWriter, r *http.Request) {
    ac, ok := getAuthContext(r)
    // ...
    // System-only SUPER_ADMIN: query across all RTs
    if ac.SystemRole == auth.SystemRoleSuperAdmin && ac.RTID == "" {
        // No rt_id filter — SUPER_ADMIN sees everything
    }
    // Tenant-scoped user: rt_id from JWT
    total, err := TransactionCount(r.Context(), tx, ac.RTID, ...)
    transactions, err := TransactionList(r.Context(), tx, ac.RTID, ...)
}
```

**Authorization:**
- ✅ `ac.RTID` derived from JWT claims, NOT from URL path or query parameter
- ✅ `requireTenantContext(ac, w)` prevents super_admin without RT membership from writing
- ✅ Tenant-scoped users: all queries filter by `rt_id`
- ✅ SUPER_ADMIN system-only: `ac.RTID == ""` route, cross-RT queries

---

### 8. API Endpoints

| Endpoint | Method | Handler | Auth | Status |
|----------|--------|---------|------|--------|
| `/api/v1/transactions` | GET | `ListTransactions` | JWT + tenant context | ✅ IMPLEMENTED |
| `/api/v1/transactions/{id}` | GET | `GetTransaction` | JWT + tenant context | ✅ IMPLEMENTED |
| `/api/v1/transactions` | POST | `CreateTransaction` | JWT + tenant context + idempotency | ✅ IMPLEMENTED |
| `/api/v1/transactions/{id}/reverse` | POST | `ReverseTransaction` | JWT + tenant context + idempotency | ✅ IMPLEMENTED |
| `/api/v1/reports/balance` | GET | `GetBalance` | JWT + tenant context | ✅ IMPLEMENTED (cumulative only) |
| `/api/v1/payments/{id}/verify` | POST | `VerifyPayment` | JWT + tenant context (pengurus only) | ✅ IMPLEMENTED |

**Parameter support:**
- Transaction list/filter: `category_id`, `type`, `status`, `start_date`, `end_date`, `page`, `per_page`
- Balance: **NO parameters** — returns cumulative total for entire history

---

### 9. Test Coverage

**File:** `backend/internal/finance/handler_test.go` (3026 lines)

| Test | File | Line | Coverage |
|------|------|------|----------|
| Payment approval → KAS transaction posted | handler_test.go | 543-556 | ✅ PASSED |
| Balance derived from ledger (100000) | handler_test.go | 560-573 | ✅ PASSED |
| Compensating reversal updates balance | handler_test.go | 576-613 | ✅ PASSED |
| Transaction idempotency | handler_test.go | 655-690 | ✅ PASSED |
| Overpayment rejection (REJECTED payment) | handler_test.go | ~1327 | ✅ PASSED |
| Bill cancellation guard | handler_test.go | 1576 | ✅ PASSED |
| Donation verification → ledger transaction | handler_test.go | 1550-1568 | ✅ PASSED |
| Multi-period exact one KAS transaction | multi_period_payment_test.go | 1117 | ✅ PASSED |
| Cross-RT tenant isolation with transactions | handler_test.go | 1970-2055 | ✅ PASSED |

**GAP — Tidak Ada Test:**
- ❌ `CreateManualTransaction` endpoint (handler test)
- ❌ Rejected payment → NO KAS transaction (verified behavior, not tested)
- ❌ Donation via `due_id` → KAS transaction
- ❌ Multi-period `verifyMultiPeriodPayment` flow
- ❌ Cross-period reversal (reversal on different month)
- ❌ Period balance calculation (feature not implemented)
- ❌ Opening balance auto-calculation
- ❌ Running balance per transaction

---

## C. Web KAS — Mock Data Only

**Files:** 6 files in `frontend/src/features/kas/`

| File | Purpose | API Integration | Status |
|------|---------|-----------------|--------|
| `KasList.tsx` | Transaction list | None | ❌ MOCK |
| `KasDetail.tsx` | Transaction detail modal | None | ❌ MOCK |
| `KasIncome.tsx` | Income form | None | ❌ MOCK |
| `KasExpense.tsx` | Expense form | None | ❌ MOCK |
| `KasReport.tsx` | Report view | None | ❌ MOCK |
| `kasRoutes.tsx` | Route definitions | None | N/A |
| `kasTypes.ts` | Type definitions | N/A | N/A |
| `kasMockData.ts` | Mock data (25 entries) | N/A | N/A |

**Grep verification:** Zero matches for `api.`, `axios`, `fetch(`, `apiClient`, `http.` — **no API calls anywhere.**

**No API client files:** No `*api*.ts` or `*api*.tsx` files in `frontend/src/features/kas/`.

**Mock data characteristics:**
- 25 transactions across 4 months (July-October 2026)
- Categories hardcoded: "Iuran Warga", "Iuran Paksa", "Perbaikan Fasilitas", "Konsumsi", "Kebersihan", "Keamanan", "Perlengkapan", "Lain-lain"
- Running balance calculated from Rp5,000,000 opening balance
- Helper functions: `formatRupiah`, `formatTanggal`, `getFilteredTransactions`
- `generateSaldo()` function computes running balance

**Data model inconsistency:** Web uses "Perbaikan Fasilitas", Android uses "Perbaikan" (shorter name). Categories should be synchronized.

---

## D. Android KAS — Mock Data Only

**Files:** 7 files in `mobile/lib/features/kas/`

| File | Purpose | API Integration | Status |
|------|---------|-----------------|--------|
| `kas_screen.dart` | Main list with search/filter | None | ❌ MOCK |
| `kas_detail_screen.dart` | Transaction detail modal | None | ❌ MOCK |
| `kas_masuk_screen.dart` | Income form | None | ❌ MOCK |
| `kas_keluar_screen.dart` | Expense form | None | ❌ MOCK |
| `kas_report_screen.dart` | Report view | None | ❌ MOCK |
| `kas_models.dart` | Data models (enum + class) | N/A | N/A |
| `kas_mock_data.dart` | Mock data (20 entries) | N/A | N/A |

**Grep verification:** Zero matches for `dio`, `api.`, `get(`, `post(` — **no API calls anywhere.**

**No API client files:** No `*api*.dart` files in `mobile/lib/features/kas/`.

**Data model:**
```dart
enum KasCategory { iuranWarga, iuranPaksa, perlengkapan, konsumsi, perbaikan, kebersihan, keamanan, lainLain }
enum KasJenis { masuk, keluar }

class KasTransaction {
  String id;
  DateTime tanggal;
  KasJenis jenis;
  KasCategory kategori;
  String keterangan;
  String referensi;
  BigInt nominal;
  BigInt saldo;  // Running balance
}
```

**Mock data:** 20 transactions (10 masuk, 10 keluar), running balance from Rp5,000,000.

---

## E. Analisis Saldo Per Periode

### 1. Saldo Awal Otomatis — NOT IMPLEMENTED

**Status:** `NOT_IMPLEMENTED`

Rumus yang diinginkan:
```
Opening(period) = SUM(net impact of all posted transactions before period start)
Closing(period) = Opening(period) + period income - period expense
```

**Implementasi saat ini (`CalculateBalance`):**
```sql
SELECT COALESCE(SUM(CASE WHEN type = 'income' THEN amount ELSE 0 END), 0)::text,
       COALESCE(SUM(CASE WHEN type = 'expense' THEN amount ELSE 0 END), 0)::text,
       COUNT(*)
FROM transactions
WHERE rt_id = $1 AND status = 'posted'
```

**Gap:** Query tidak memiliki filter `occurred_at` → menghasilkan total seluruh sejarah, bukan per periode.

**Yang perlu diimplementasikan:**
1. Endpoint balance dengan parameter `start_date` dan `end_date`
2. Query opening balance: `SUM(net impact) WHERE occurred_at < start_date`
3. Query period income: `SUM(income) WHERE occurred_at BETWEEN start_date AND end_date`
4. Query period expense: `SUM(expense) WHERE occurred_at BETWEEN start_date AND end_date`
5. Closing = Opening + Income - Expense

### 2. Saldo Awal Historis — BUSINESS_DECISION_REQUIRED

**Status:** `BUSINESS_DECISION_REQUIRED`

Otomatisasi dari ledger hanya bekerja jika:
- RT mulai menggunakan aplikasi dari nol (tanpa saldo kas sebelumnya)
- ATAU ada transaksi pembuka awal saat onboarding

Jika RT memiliki saldo kas RpXX.XXX.XXX sebelum menggunakan aplikasi, maka:
- `Opening(first_period)` akan menghitung Rp0 (karena tidak ada transaksi historis)
- Saldo aktual akan lebih tinggi dari yang ditampilkan
- Tidak ada mekanisme di kode atau dokumentasi untuk merepresentasikan saldo awal historis

**Rekomendasi:** Buat satu transaksi manual saat onboarding dengan kategori "Saldo Awal" dan occurred_at sebelum periode pertama. Ini bukan "input saldo bulanan" — ini sekali saja saat inisialisasi aplikasi.

### 3. Reversal dan Saldo Periode

**Semantik reversal:**
- Transaksi asal: `status = 'posted'`, type = 'income', amount = X
- Reversal: `status = 'posted'`, type = 'expense', amount = X, reverses_transaction_id = original_id
- Net impact: +X (income) - X (expense) = 0

**Perhitungan balance:**
```sql
WHERE status = 'posted'
```
Mencakup DUA transaksi: original + reversal. Dampak bersih = 0. **Ini benar.**

**Cross-period scenario:**
- Transaksi income di Bulan 1: +100,000
- Reversal di Bulan 3: -100,000

Perhitungan per periode:
- Bulan 1: Opening=0, Income=100,000, Expense=0, Closing=100,000 ✅
- Bulan 2: Opening=100,000, Income=0, Expense=0, Closing=100,000 ✅
- Bulan 3: Opening=100,000, Income=0, Expense=100,000, Closing=0 ✅

**Semantik reversal saat ini sudah benar untuk saldo per periode.** Tidak ada penyesuaian diperlukan karena reversal adalah transaksi `posted` yang independen dengan type berlawanan.

### 4. Zona Waktu dan Batas Periode

**Backend:** PostgreSQL `timestamptz` untuk `occurred_at`. Query menggunakan `>=` dan `<=` untuk filter.

**Interval:** Tidak ada eksplisit start-exclusive / end-exclusive dalam kode saat ini karena belum ada filter periode. Namun, menggunakan `>= start AND <= end` dengan `timestamptz` akan inklusif di kedua batas. Untuk periode bulanan, disarankan:
- `start`: `2026-10-01 00:00:00+07:00` (inclusive)
- `end`: `2026-10-31 23:59:59+07:00` (inclusive) ATAU
- `end`: `2026-11-01 00:00:00+07:00` (exclusive)

**Frontend:** TypeScript `Date` objects, timezone bergantung pada browser user. Perlu konsistensi dengan backend timezone.

---

## F. Rekomendasi Desain UI KAS Web

### 1. Apakah Sudah Ada Rencana Tabel Neraca Kas?

**Status:** `NOT_DOCUMENTED`

Tidak ada dokumentasi eksplisit tentang desain tabel neraca kas di dokumen apa pun yang diperiksa:
- `docs/iuran-kas-module.md` — blueprint umum, tidak ada rancangan neraca
- `docs/business-rules.md` — aturan bisnis, tidak ada desain UI
- `docs/api.md` — endpoint draft, tidak ada laporan periode
- `docs/development-plan.md` — Phase 6 & 9, tidak ada detail UI neraca

### 2. Rekomendasi Desain

#### Ringkasan Periode (Header Card)

```
┌─────────────────────────────────────────────────────────────┐
│  Periode: [Oktober 2026 ▼]                                   │
├─────────────────────────────────────────────────────────────┤
│  Saldo Awal:     Rp 1,200,000                                │
│  Pemasukan:      Rp   850,000                                │
│  Pengeluaran:   -Rp   320,000                                │
│  ─────────────────────────────────                           │
│  Mutasi Bersih:  Rp   530,000                                │
│  Saldo Akhir:    Rp 1,730,000                                │
└─────────────────────────────────────────────────────────────┘
```

#### Tabel Mutasi Kas

| No | Tanggal | Keterangan | Kategori | Referensi | Pemasukan | Pengeluaran | Saldo |
|----|---------|-----------|----------|-----------|----------:|-----------:|--------:|
| — | — | **Saldo Awal** | — | — | — | — | Rp 1,200,000 |
| 1 | 02 Okt | Pembayaran IPL Budi | Iuran Warga | PAY-20261002-001 | Rp 100,000 | — | Rp 1,300,000 |
| 2 | 05 Okt | Konsumsi Rapat | Konsumsi | — | — | Rp 150,000 | Rp 1,150,000 |
| 3 | 10 Okt | Pembayaran IPL Siti | Iuran Warga | PAY-20261010-002 | Rp 100,000 | — | Rp 1,250,000 |
| 4 | 15 Okt | Perbaikan Gapura | Perbaikan | Manual Entry | — | Rp 200,000 | Rp 1,050,000 |
| 5 | 20 Okt | Donasi Sukarela Ahmad | Iuran Warga | PAY-20261020-003 | Rp 200,000 | — | Rp 1,250,000 |

**Indikator reversal:** Baris dengan ikon ↻ atau keterangan "(reversal)"
**Format:** Pemasukan dan pengeluaran selalu positif. Saldo bisa negatif.

#### Filter Periode

- Dropdown pemilih bulan dan tahun
- Navigasi: [ < ] [ Oktober 2026 ] [ > ]
- Tombol "Bulan Ini" untuk kembali ke periode aktif
- Opsi "Semua Periode" untuk view kumulatif

#### Pagination

- Rincian transaksi: paginasi (20 per halaman)
- **Ringkasan periode TIDAK dipaginasikan** — dihitung dari seluruh data yang sesuai filter
- Pastikan ringkasan dan rincian menggunakan kriteria filter yang sama

#### Saldo Berjalan

- **Frontend** — lebih baik karena tidak ada risiko inkonsistensi antara request ringkasan dan rincian
- Backend hanya perlu mengembalikan transaksi per periode (sudah ada)
- Frontend menghitung running balance dari data yang diterima
- Untuk data yang dipaginasikan, saldo berjalan hanya akurat di halaman saat ini

---

## G. Rekomendasi Kontrak API

### Opsi A: Enhance Existing Endpoints (Direkomendasikan)

**Kekuatan:** Memanfaatkan endpoint yang sudah ada, konsistensi dengan pola proyek

**Perubahan minimal:**

1. **`GET /api/v1/reports/balance`** — Tambah parameter periode
   ```
   Query params: start_date (YYYY-MM-DD), end_date (YYYY-MM-DD), period (YYYY-MM)
   
   Response: {
     "data": {
       "opening_balance": "1200000.00",
       "total_income": "850000.00",
       "total_expense": "320000.00",
       "net_movement": "530000.00",
       "closing_balance": "1730000.00",
       "transaction_count": 25
     }
   }
   ```

2. **`GET /api/v1/transactions`** — Sudah mendukung filter `start_date`, `end_date`, `category_id`, `type`, `status`
   - ✅ Sudah bisa disuplai rincian mutasi per periode
   - ✅ Sudah ada pagination

**Yang perlu ditambahkan di backend:**
- Logic untuk menghitung opening balance: `SUM(net impact WHERE occurred_at < start_date)`
- Logic untuk menghitung per-period income/expense
- Parameter parsing untuk `period` (auto-expand ke start/end of month)

### Opsi B: New Period Report Endpoint

```
GET /api/v1/reports/period?period=2026-10

Response: {
  "period": "2026-10",
  "period_label": "Oktober 2026",
  "summary": {
    "opening_balance": "1200000.00",
    "total_income": "850000.00",
    "total_expense": "320000.00",
    "net_movement": "530000.00",
    "closing_balance": "1730000.00"
  },
  "transactions": [...paginated...],
  "pagination": {...}
}
```

**Kelemahan:** Endpoint baru — perlu routing, handler, service, test. Pola yang ada sudah mendukung Opsi A dengan perubahan minimal.

### Rekomendasi: Opsi A

Gunakan existing endpoints dengan penambahan parameter. Hemat waktu, konsisten dengan arsitektur saat ini.

### Traceability

Kolom yang ada sudah memenuhi kebutuhan traceability untuk scope sekarang:
- `payment_id` → link ke payment source untuk transaksi dari Iuran
- `reverses_transaction_id` → link ke original untuk reversal
- `description` → teks bebas untuk penjelasan tambahan
- `category_id` → kategori transaksi (income/expense)

**Tidak perlu** `source_type` / `source_id` selama scope terbatas pada pelacakan payment dan reversal.

---

## H. Urutan Implementasi yang Direkomendasikan

### Fase 1: Fondasi Periode (Backend) — 2-3 hari

| Langkah | Pekerjaan | Estimasi |
|---------|-----------|----------|
| 1.1 | Enhance `CalculateBalance` → parameter `start_date`, `end_date` | 0.5 hari |
| 1.2 | Add `CalculatePeriodOpeningBalance` repository function | 0.5 hari |
| 1.3 | Update `BalanceSummary` model: add `opening_balance`, `closing_balance` | 0.5 hari |
| 1.4 | Update `GetBalance` handler: parse period params | 0.5 hari |
| 1.5 | Unit tests: opening/closing balance, cross-period reversal | 1 hari |

### Fase 2: Integrasi Web KAS (API → UI) — 3-4 hari

| Langkah | Pekerjaan | Estimasi |
|---------|-----------|----------|
| 2.1 | Create API client module (`frontend/src/features/kas/kas-api.ts`) | 1 hari |
| 2.2 | Replace mock data in `KasList.tsx` with real API calls | 1 hari |
| 2.3 | Replace mock data in `KasReport.tsx` with real API + period filter | 1 hari |
| 2.4 | Implement period selector (dropdown) | 0.5 hari |
| 2.5 | Loading, error, empty states | 0.5 hari |

### Fase 3: Implementasi Tabel Neraca Web — 3-4 hari

| Langkah | Pekerjaan | Estimasi |
|---------|-----------|----------|
| 3.1 | Design balance summary card component | 0.5 hari |
| 3.2 | Implement mutasi kas table with running balance | 1.5 hari |
| 3.3 | Reversal indicator (↻ icon) | 0.5 hari |
| 3.4 | Period navigation (< > buttons) | 0.5 hari |
| 3.5 | Pagination integration | 0.5 hari |

### Fase 4: Integrasi Android KAS (API → UI) — 2-3 hari

| Langkah | Pekerjaan | Estimasi |
|---------|-----------|----------|
| 4.1 | Create API client module (`mobile/lib/features/kas/kas_api_client.dart`) | 1 hari |
| 4.2 | Update models to match backend `BalanceSummary` | 0.5 hari |
| 4.3 | Replace mock data in `kas_screen.dart` | 1 hari |
| 4.4 | Period filter (simple date picker) | 0.5 hari |

### Fase 5: Seed Data & Polish — 1-2 hari

| Langkah | Pekerjaan | Estimasi |
|---------|-----------|----------|
| 5.1 | Seed default expense categories | 0.5 hari |
| 5.2 | Sync category names between Web and Android | 0.5 hari |
| 5.3 | End-to-end verification | 1 hari |

### Total Estimasi: 11-16 hari kerja

### Dependensi

```
Fase 1 (Backend Period Balance)
    ↓
Fase 2 (Web API Integration) ────┐
    ↓                            ↓
Fase 3 (Web Neraca Table)   Fase 4 (Android API)
    ↓                            ↓
    └────→ Fase 5 (Seed & Polish)
```

---

## I. Daftar Keputusan Bisnis yang Diperlukan

| # | Keputusan | Konsekuensi Jika Tidak diputuskan | Priority |
|---|-----------|-----------------------------------|----------|
| 1 | **Saldo awal historis** — Apakah perlu input saldo awal saat onboarding RT? | Saldo pertama periode akan selalu Rp0 untuk RT baru | **HIGH** |
| 2 | **Draft workflow** — Apakah manual transaction perlu status `draft` sebelum diposting? | Semua manual transaction langsung `posted` | Medium |
| 3 | **Zona waktu** — Apakah menggunakan WIB (UTC+7) atau UTC? | Konsistensi filter periode jika user di timezone berbeda | Medium |
| 4 | **Category management** — Apakah pengurus bisa create/update category sendiri? | Saat ini: auto-create "Iuran Warga", expense category perlu manual seed | Medium |
| 5 | **Multi-period table view** — Apakah tabel neraca perlu view multi-periode (span beberapa bulan)? | Saat ini: satu bulan per request | Low |

---

## J. Status Semua Temuan

### Backend KAS

| Aspek | Status | Bukti |
|-------|--------|-------|
| Transactions table (ledger) | ✅ IMPLEMENTED | `migrations/009_finance.up.sql` |
| Balance calculation (cumulative) | ✅ IMPLEMENTED_AND_TESTED | `repository.go:1078`, `handler_test.go:560` |
| Balance per period | ❌ NOT_IMPLEMENTED | `CalculateBalance` has no period params |
| Opening balance auto-calc | ❌ NOT_IMPLEMENTED | No code exists |
| Closing balance per period | ❌ NOT_IMPLEMENTED | Derived from opening + mutasi |
| Iuran → KAS integration | ✅ IMPLEMENTED_AND_TESTED | `service.go:319-366, 568-602`, tested at `handler_test.go:543` |
| Atomicity (payment + KAS) | ✅ IMPLEMENTED_AND_TESTED | Single `tx *sql.Tx` in all flows |
| Reversal mechanism | ✅ IMPLEMENTED_AND_TESTED | `service.go:960-1012`, `handler_test.go:576` |
| Cross-period reversal | ✅ IMPLEMENTED_NOT_TESTED | Semantics correct, no cross-period test |
| Manual transaction | ✅ IMPLEMENTED_NOT_TESTED | Handler endpoint exists, no handler test |
| Idempotency | ✅ IMPLEMENTED_AND_TESTED | `idempotency.go`, `handler_test.go:655` |
| Tenant isolation | ✅ IMPLEMENTED_AND_TESTED | JWT-derived rt_id, `handler_test.go:1970` |
| Role-based access | ✅ IMPLEMENTED | `requireTenantContext` guard |
| Payment rejection → no KAS | ✅ IMPLEMENTED_NOT_TESTED | Code path exists, no test verifies it |
| Donation via due_id → KAS | ✅ IMPLEMENTED_NOT_TESTED | Code path exists, no dedicated test |
| Multi-period KAS transaction | ✅ IMPLEMENTED_AND_TESTED | `multi_period_payment_test.go:1117` |
| Fixed-precision (NUMERIC) | ✅ IMPLEMENTED | Schema + `ParseMoney`/`FormatMoney` |
| `source_type`/`source_id` | ❌ NOT_IMPLEMENTED | Blueprint-defined, never in schema |

### Web KAS

| Aspek | Status | Bukti |
|-------|--------|-------|
| KasList.tsx | ❌ MOCK_ONLY | No API calls (grep verified) |
| KasDetail.tsx | ❌ MOCK_ONLY | No API calls |
| KasIncome.tsx | ❌ MOCK_ONLY | No API calls |
| KasExpense.tsx | ❌ MOCK_ONLY | No API calls |
| KasReport.tsx | ❌ MOCK_ONLY | No API calls |
| API client | ❌ NOT_IMPLEMENTED | No `*api*.ts` files |
| Period filter | ❌ NOT_IMPLEMENTED | Mock only |
| Balance summary | ❌ MOCK_ONLY | Mock data only |
| Running balance | ❌ MOCK_ONLY | `generateSaldo()` in mock data |
| Period table design | ❌ NOT_DOCUMENTED | No explicit design docs |

### Android KAS

| Aspek | Status | Bukti |
|-------|--------|-------|
| kas_screen.dart | ❌ MOCK_ONLY | No API calls (grep verified) |
| kas_detail_screen.dart | ❌ MOCK_ONLY | No API calls |
| kas_masuk_screen.dart | ❌ MOCK_ONLY | No API calls |
| kas_keluar_screen.dart | ❌ MOCK_ONLY | No API calls |
| kas_report_screen.dart | ❌ MOCK_ONLY | No API calls |
| API client | ❌ NOT_IMPLEMENTED | No `*api*.dart` files |
| Model mapping | N/A | Models exist but unused |
| Error/loading states | ❌ NOT_IMPLEMENTED | Mock only |

### Test Coverage

| Test Scenario | Status |
|--------------|--------|
| Payment approval → KAS transaction | ✅ HANDLED |
| Balance calculation (cumulative) | ✅ HANDLED |
| Reversal (same period) | ✅ HANDLED |
| Transaction idempotency | ✅ HANDLED |
| Overpayment rejection | ✅ HANDLED |
| Cross-RT isolation | ✅ HANDLED |
| Opening balance auto-calc | ❌ NOT TESTED |
| Period balance (income/expense range) | ❌ NOT TESTED |
| Reversal (cross-period) | ❌ NOT TESTED |
| Rejected payment → NO KAS | ❌ NOT TESTED |
| Donation via due_id → KAS | ❌ NOT TESTED |
| Manual transaction endpoint | ❌ NOT TESTED |
| Multi-period verify flow | ❌ NOT TESTED |

---

## K. Ringkasan Kesimpulan

### Yang Sudah Berfungsi dengan Baik

1. **Ledger KAS** menggunakan tabel `transactions` dengan semantik yang benar: 3-state lifecycle, immutable posted transactions, reversal via compensating entry
2. **Balance calculation** menggunakan `NUMERIC(15,2)` dan `ParseMoney()` — fixed-precision, no floating point
3. **Iuran → KAS integration** berjalan otomatis pada setiap payment approval: payment update + bill status + KAS transaction dalam satu PostgreSQL transaction (atomic)
4. **Idempotency** melindungi CreateTransaction, ReverseTransaction, CreatePayment, dan GenerateBills menggunakan PostgreSQL advisory locks
5. **Tenant isolation** memastikan rt_id berasal dari JWT claims, bukan client-supplied
6. **Test coverage** untuk payment verification, balance calculation, reversal, dan idempotency sudah ada dan passing

### Yang Belum Terimplementasi

1. **Period-based balance** — `CalculateBalance` hanya mengembalikan total kumulatif seluruh sejarah, tidak ada filter per periode. Ini fondasi yang dibutuhkan untuk neraca kas.
2. **Opening balance per periode** — Tidak ada mekanisme untuk menghitung saldo di awal periode. Untuk RT yang memulai dari nol, ini bisa dihitung dari transaksi sebelum periode. Untuk RT dengan saldo historis, dibutuhkan keputusan bisnis tentang saldo pembuka awal.
3. **Running balance** — Tidak ada endpoint atau UI yang menunjukkan saldo berjalan per transaksi.
4. **Web + Android KAS UI** — Seluruh 13 file (6 Web + 7 Android) menggunakan mock data. Tidak ada API client yang terintegrasi.
5. **Test coverage** — Manual transaction endpoint, rejected payment → no KAS, cross-period reversal, period balance tidak di-test.

### Rekomendasi Langkah Pertama

**Implement Phase 1: Period Balance (Backend)** — ini fondasi yang dibutuhkan sebelum UI mana pun bisa menampilkan neraca kas.

1. Tambah parameter `start_date` dan `end_date` (atau `period`) pada `CalculateBalance`
2. Return `opening_balance` dan `closing_balance` di samping `total_income` dan `total_expense`
3. Tulis test untuk opening/closing balance dan cross-period reversal

Setelah backend siap, UI Web dan Android bisa diintegrasikan secara paralel.

---

## L. Glosarium

| Istilah | Keterangan |
|---------|-----------|
| **KAS** | Kas Umum RT — manajemen keuangan umum RT |
| **Ledger** | Tabel `transactions` — single source of truth untuk semua transaksi |
| **Posted** | Status transaksi yang sudah final, tidak bisa diedit atau dihapus |
| **Draft** | Status transaksi yang belum final, bisa diedit atau dihapus |
| **Cancelled** | Status transaksi yang dibatalkan (reversal) |
| **Reversal** | Transaksi kompensasi dengan type berlawanan dan amount sama |
| **Iuran** | Pembayaran tagihan RT (IPL, dll) oleh warga |
| **Donasi** | Sumbangan sukarela via `due_id` (bukan `bill_id`) |
| **Payment** | Catatan pembayaran dengan status PENDING/APPROVED/REJECTED |
| **Atomic** | Operasi yang berhasil semua atau gagal semua dalam satu transaction |
| **Idempotency** | Memastikan request yang sama tidak dieksekusi dua kali |
| **Tenant** | Satu RT — data terisolasi antar RT |
| **Fixed-precision** | NUMERIC(15,2) di PostgreSQL, string di Go, BigInt/String di Flutter |
| **Running balance** | Saldo setelah setiap transaksi, dihitung dari saldo awal + mutasi |
| **Opening balance** | Saldo di awal periode (dari saldo akhir periode sebelumnya) |
| **Closing balance** | Saldo di akhir periode = opening + income - expense |

---

**Laporan ini dibuat berdasarkan penelusuran source code secara READ-ONLY.**  
**Tidak ada perubahan kode, database, atau working tree yang dilakukan.**  
**Semua temuan diverifikasi dari file aktual di repository.**
