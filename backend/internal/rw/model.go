package rw

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Sentinel errors for RW authorization, validation, and domain logic.
var (
	ErrRWAccessDenied              = errors.New("rw access denied")
	ErrPermissionDenied            = errors.New("permission denied")
	ErrValidation                  = errors.New("validation error")
	ErrNotFound                    = errors.New("not found")
	ErrDuplicateName               = errors.New("location with this name already exists in RW")
	ErrCannotDeactivateWithBalance = errors.New("cannot deactivate location with positive balance")
	ErrSameLocationTransfer        = errors.New("source and destination location must be different")
	ErrInsufficientFunds           = errors.New("insufficient funds in source location")
	ErrInsufficientReversal        = errors.New("insufficient balance in recipient location for reversal")
	ErrAlreadyReversed             = errors.New("transfer has already been reversed")
	ErrCannotReverseReversal       = errors.New("cannot reverse a reversal transfer")
	ErrInactiveLocation            = errors.New("location is inactive")
	ErrInvalidCategory             = errors.New("invalid category: must be rekening, brankas, or coh")
	ErrMissingBankName             = errors.New("bank name is required for category rekening")
	ErrInvalidAmount               = errors.New("amount must be greater than zero")
	ErrCrossRWTransfer             = errors.New("cross-RW transfer is forbidden")
	ErrInvalidMethodCategoryMatch  = errors.New("payment method does not match fund location category")
)

type Category string

const (
	CategoryRekening Category = "rekening"
	CategoryBrankas  Category = "brankas"
	CategoryCOH      Category = "coh"
)

// ---- Models ----

// DashboardSummary aggregates RW-level financial overview.
// ALL balance/income/expense come from RW's own independent ledger:
//   - OpeningBalance from rw_opening_balances
//   - Income / Expense from rw_transactions
//   - NetBalance = TotalIncome - TotalExpense (Saldo Bersih)
//   - TotalBalance = TotalOpeningBal + TotalIncome - TotalExpense (Total Saldo)
//   - TotalLocations: active RW fund locations
type DashboardSummary struct {
	TotalRTs         int64  `json:"total_rts"`
	TotalLocations   int64  `json:"total_locations"`
	TotalOpeningBal  string `json:"total_opening_balance"`
	TotalIncome      string `json:"total_income"`
	TotalExpense     string `json:"total_expense"`
	NetBalance       string `json:"net_balance"`
	TotalBalance     string `json:"total_balance,omitempty"`
	TransactionCount int64  `json:"transaction_count"`
}

// LocationSummary represents an independent fund location of the RW.
type LocationSummary struct {
	ID             string    `json:"id"`
	RWID           string    `json:"rw_id,omitempty"`
	RTID           string    `json:"rt_id,omitempty"`
	RTName         string    `json:"rt_name,omitempty"`
	Name           string    `json:"name"`
	Category       Category  `json:"category"`
	BankName       *string   `json:"bank_name,omitempty"`
	AccountNumber  *string   `json:"account_number,omitempty"`
	AccountHolder  *string   `json:"account_holder,omitempty"`
	IsActive       bool      `json:"is_active"`
	CurrentBalance string    `json:"current_balance"`
	CreatedAt      string    `json:"created_at"`
	UpdatedAt      string    `json:"updated_at,omitempty"`
}

// CreateRWFundLocationInput defines input for creating a new RW fund location.
type CreateRWFundLocationInput struct {
	Name          string   `json:"name"`
	Category      Category `json:"category"`
	BankName      *string  `json:"bank_name,omitempty"`
	AccountNumber *string  `json:"account_number,omitempty"`
	AccountHolder *string  `json:"account_holder,omitempty"`
}

func (in *CreateRWFundLocationInput) Validate() error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return fmt.Errorf("%w: name is required", ErrValidation)
	}
	in.Category = Category(strings.ToLower(strings.TrimSpace(string(in.Category))))
	if in.Category != CategoryRekening && in.Category != CategoryBrankas && in.Category != CategoryCOH {
		return ErrInvalidCategory
	}
	if in.Category == CategoryRekening {
		if in.BankName == nil || strings.TrimSpace(*in.BankName) == "" {
			return ErrMissingBankName
		}
		trimmed := strings.TrimSpace(*in.BankName)
		in.BankName = &trimmed
	}
	if in.AccountNumber != nil {
		trimmed := strings.TrimSpace(*in.AccountNumber)
		in.AccountNumber = &trimmed
	}
	if in.AccountHolder != nil {
		trimmed := strings.TrimSpace(*in.AccountHolder)
		in.AccountHolder = &trimmed
	}
	return nil
}

// UpdateRWFundLocationInput defines input for updating an existing RW fund location.
type UpdateRWFundLocationInput struct {
	Name          *string `json:"name,omitempty"`
	BankName      *string `json:"bank_name,omitempty"`
	AccountNumber *string `json:"account_number,omitempty"`
	AccountHolder *string `json:"account_holder,omitempty"`
}

// FundTransfer represents a fund transfer between two locations within the same RW.
type FundTransfer struct {
	ID                 string    `json:"id"`
	RWID               string    `json:"rw_id"`
	FromLocationID     string    `json:"from_location_id"`
	ToLocationID       string    `json:"to_location_id"`
	Amount             string    `json:"amount"`
	TransferDate       time.Time `json:"transfer_date"`
	Notes              *string   `json:"notes,omitempty"`
	CreatedBy          *string   `json:"created_by,omitempty"`
	ReversesTransferID *string   `json:"reverses_transfer_id,omitempty"`
	CreatedAt          time.Time `json:"created_at"`

	FromLocationName   *string   `json:"from_location_name,omitempty"`
	ToLocationName     *string   `json:"to_location_name,omitempty"`
}

// CreateFundTransferInput defines input for creating a transfer.
type CreateFundTransferInput struct {
	FromLocationID string  `json:"from_location_id"`
	ToLocationID   string  `json:"to_location_id"`
	Amount         string  `json:"amount"`
	TransferDate   *string `json:"transfer_date,omitempty"`
	Notes          *string `json:"notes,omitempty"`
}

// ReverseFundTransferInput defines input for reversing a transfer.
type ReverseFundTransferInput struct {
	Reason string `json:"reason"`
}

// LocationBalanceChange represents an append-only mutation row in rw_location_balance_changes.
type LocationBalanceChange struct {
	ID            string    `json:"id"`
	RWID          string    `json:"rw_id"`
	LocationID    string    `json:"location_id"`
	TransactionID *string   `json:"transaction_id,omitempty"`
	TransferID    *string   `json:"transfer_id,omitempty"`
	ChangeType    string    `json:"change_type"`
	AmountDelta   string    `json:"amount_delta"`
	Notes         *string   `json:"notes,omitempty"`
	CreatedBy     *string   `json:"created_by,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// CategorySummary aggregates total balance and count per category.
type CategorySummary struct {
	Category       string `json:"category"`
	TotalBalance   string `json:"total_balance"`
	LocationsCount int    `json:"locations_count"`
}

// ReconciliationReport provides reconciliation status between KAS and fund locations.
type ReconciliationReport struct {
	KasBalance            string            `json:"kas_balance"`
	TotalAllocatedBalance string            `json:"total_allocated_balance"`
	UnallocatedBalance    string            `json:"unallocated_balance"`
	Discrepancy           string            `json:"discrepancy"`
	Status                string            `json:"status"`
	StatusLabel           string            `json:"status_label"`
	LocationsSummary      []CategorySummary `json:"locations_summary"`
	AnomaliesDetected     []string          `json:"anomalies_detected"`
}

// KASReport aggregates KAS data (income/expense) from RW's own independent ledger.
type KASReport struct {
	OpeningBalance   string `json:"opening_balance"`
	TotalIncome      string `json:"total_income"`
	TotalExpense     string `json:"total_expense"`
	NetBalance       string `json:"net_balance"`
	TotalBalance     string `json:"total_balance,omitempty"`
	TransactionCount int64  `json:"transaction_count"`
}

// TransactionEntry is a single posted transaction from RW's own independent ledger.
type TransactionEntry struct {
	ID              string  `json:"id"`
	RWID            string  `json:"rw_id"`
	LocationID      *string `json:"location_id,omitempty"`
	LocationName    *string `json:"location_name,omitempty"`
	SourceRTID      *string `json:"source_rt_id,omitempty"`
	SourceRTName    *string `json:"source_rt_name,omitempty"`
	Type            string  `json:"type"`
	Amount          string  `json:"amount"`
	Description     string  `json:"description"`
	CategoryID      *string `json:"category_id,omitempty"`
	CategoryName    *string `json:"category_name,omitempty"`
	Status          string  `json:"status"`
	PaymentID       *string `json:"payment_id,omitempty"`
	ReferenceNo     *string `json:"reference_no,omitempty"`
	PaymentMethod   *string `json:"payment_method,omitempty"`
	OccurredAt      string  `json:"occurred_at"`
	CreatedAt       string  `json:"created_at"`
	ProofPath       *string `json:"proof_path,omitempty"`
	ProofURL        *string `json:"proof_url,omitempty"`
	ExpenseCategory *string `json:"expense_category,omitempty"`
}

// CreateTransactionInput defines payload for creating a new KAS Masuk or KAS Keluar transaction.
type CreateTransactionInput struct {
	Type          string  `json:"type"` // "income" | "expense"
	Amount        string  `json:"amount"`
	LocationID    string  `json:"location_id"`
	SourceRTID    *string `json:"source_rt_id,omitempty"` // For penerimaan dana dari RT
	CategoryID    *string `json:"category_id,omitempty"`
	Description   string  `json:"description"`
	OccurredAt    *string `json:"occurred_at,omitempty"`
	PaymentMethod *string `json:"payment_method,omitempty"` // "CASH" | "TRANSFER"
	ExpenseCategory *string `json:"expense_category,omitempty"`
}

type UpdateOpeningBalanceInput struct {
	OpeningBalance string `json:"opening_balance"`
}

// PaginationInfo metadata returned alongside list responses.
type PaginationInfo struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// ---- Input Validation ----

// ValidatePeriod parses YYYY-MM format.
func ValidatePeriod(period string) (string, error) {
	p := strings.TrimSpace(period)
	if p == "" {
		return "", fmt.Errorf("period is required (format: YYYY-MM)")
	}
	if len(p) != 7 {
		return "", fmt.Errorf("invalid period format: expected YYYY-MM, got %q", p)
	}
	for i, c := range p {
		if i == 4 {
			if c != '-' {
				return "", fmt.Errorf("invalid period format: expected YYYY-MM, got %q", p)
			}
		} else {
			if c < '0' || c > '9' {
				return "", fmt.Errorf("invalid period format: expected YYYY-MM, got %q", p)
			}
		}
	}
	return p, nil
}

// ValidateDate parses YYYY-MM-DD format.
func ValidateDate(date string) (string, error) {
	d := strings.TrimSpace(date)
	if d == "" {
		return "", fmt.Errorf("date is required (format: YYYY-MM-DD)")
	}
	if len(d) != 10 || d[4] != '-' || d[7] != '-' {
		return "", fmt.Errorf("invalid date format: expected YYYY-MM-DD, got %q", d)
	}
	return d, nil
}
