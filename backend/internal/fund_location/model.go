package fundlocation

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrNotFound                   = errors.New("fund location not found")
	ErrDuplicateName              = errors.New("location with this name already exists in RT")
	ErrCannotDeactivateWithBalance = errors.New("cannot deactivate location with positive balance")
	ErrSameLocationTransfer       = errors.New("source and destination location must be different")
	ErrInsufficientFunds          = errors.New("insufficient funds in source location")
	ErrInsufficientReversal       = errors.New("insufficient balance in recipient location for reversal")
	ErrAlreadyReversed            = errors.New("transfer has already been reversed")
	ErrCannotReverseReversal      = errors.New("cannot reverse a reversal transfer")
	ErrInactiveLocation           = errors.New("location is inactive")
	ErrInvalidCategory            = errors.New("invalid category: must be rekening, brankas, or coh")
	ErrMissingBankName            = errors.New("bank name is required for category rekening")
	ErrInvalidAmount              = errors.New("amount must be greater than zero")
	ErrInsufficientUnallocated    = errors.New("opening balance cannot exceed unallocated cash balance")
	ErrInvalidMethodCategoryMatch = errors.New("payment method does not match fund location category")
	ErrSuperAdminWriteBlocked     = errors.New("super_admin cannot perform write mutations on fund locations")
	ErrWargaBlocked               = errors.New("warga cannot access fund locations")
)

type Category string

const (
	CategoryRekening Category = "rekening"
	CategoryBrankas Category = "brankas"
	CategoryCOH      Category = "coh"
)

type FundLocation struct {
	ID             string    `json:"id"`
	RTID           string    `json:"rt_id"`
	Name           string    `json:"name"`
	Category       Category  `json:"category"`
	BankName       *string   `json:"bank_name,omitempty"`
	AccountNumber  *string   `json:"account_number,omitempty"`
	AccountHolder  *string   `json:"account_holder,omitempty"`
	IsActive       bool      `json:"is_active"`
	CurrentBalance string    `json:"current_balance"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type CreateFundLocationInput struct {
	Name          string   `json:"name"`
	Category      Category `json:"category"`
	BankName      *string  `json:"bank_name,omitempty"`
	AccountNumber *string  `json:"account_number,omitempty"`
	AccountHolder *string  `json:"account_holder,omitempty"`
}

func (in *CreateFundLocationInput) Validate() error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return errors.New("name is required")
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

type UpdateFundLocationInput struct {
	Name          *string `json:"name,omitempty"`
	BankName      *string `json:"bank_name,omitempty"`
	AccountNumber *string `json:"account_number,omitempty"`
	AccountHolder *string `json:"account_holder,omitempty"`
}

type FundTransfer struct {
	ID                 string    `json:"id"`
	RTID               string    `json:"rt_id"`
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

type CreateFundTransferInput struct {
	FromLocationID string  `json:"from_location_id"`
	ToLocationID   string  `json:"to_location_id"`
	Amount         string  `json:"amount"`
	TransferDate   *string `json:"transfer_date,omitempty"`
	Notes          *string `json:"notes,omitempty"`
}

type ReverseFundTransferInput struct {
	Reason string `json:"reason"`
}

type SetOpeningBalanceInput struct {
	Amount string  `json:"amount"`
	Notes  *string `json:"notes,omitempty"`
}

type LocationBalanceChange struct {
	ID            string    `json:"id"`
	RTID          string    `json:"rt_id"`
	LocationID    string    `json:"location_id"`
	TransactionID *string   `json:"transaction_id,omitempty"`
	TransferID    *string   `json:"transfer_id,omitempty"`
	ChangeType    string    `json:"change_type"`
	AmountDelta   string    `json:"amount_delta"`
	Notes         *string   `json:"notes,omitempty"`
	CreatedBy     *string   `json:"created_by,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type CategorySummary struct {
	Category       string `json:"category"`
	TotalBalance   string `json:"total_balance"`
	LocationsCount int    `json:"locations_count"`
}

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
