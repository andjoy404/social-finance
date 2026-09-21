package finance

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrNotFound             = errors.New("resource not found")
	ErrDuplicateCategory    = errors.New("category with this name already exists in RT")
	ErrDuplicateBill        = errors.New("bill already exists for this occupancy, due, and period")
	ErrBillAlreadyPaid      = errors.New("bill is already paid")
	ErrBillCancelled        = errors.New("bill is cancelled")
	ErrInvalidStatus        = errors.New("invalid status transition")
	ErrTransactionPosted    = errors.New("transaction is already posted and immutable")
	ErrAlreadyReversed      = errors.New("transaction is already reversed")
	ErrCannotReverseNonPost = errors.New("cannot reverse a non-posted transaction")
	ErrInvalidDuePeriod     = errors.New("invalid period_type: must be monthly, yearly, or one_time")
	ErrInvalidCategoryType  = errors.New("invalid category type: must be income or expense")
	ErrInvalidAmount        = errors.New("amount must be greater than zero")
)

// CategoryType represents income or expense.
type CategoryType string

const (
	CategoryTypeIncome  CategoryType = "income"
	CategoryTypeExpense CategoryType = "expense"
)

// FinancialCategory represents a category within an RT.
type FinancialCategory struct {
	ID        string       `json:"id"`
	RTID      string       `json:"rt_id"`
	Name      string       `json:"name"`
	Type      CategoryType `json:"type"`
	IsActive  bool         `json:"is_active"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

type CreateCategoryInput struct {
	Name string       `json:"name"`
	Type CategoryType `json:"type"`
}

type UpdateCategoryInput struct {
	Name     *string `json:"name"`
	IsActive *bool   `json:"is_active"`
}

// DuePeriodType represents the billing frequency of dues.
type DuePeriodType string

const (
	DuePeriodMonthly DuePeriodType = "monthly"
	DuePeriodYearly  DuePeriodType = "yearly"
	DuePeriodOneTime DuePeriodType = "one_time"
)

// Due represents RT dues configuration.
type Due struct {
	ID         string        `json:"id"`
	RTID       string        `json:"rt_id"`
	Name       string        `json:"name"`
	Amount     string        `json:"amount"` // String decimal NUMERIC(15,2)
	PeriodType DuePeriodType `json:"period_type"`
	IsActive   bool          `json:"is_active"`
	CreatedAt  time.Time     `json:"created_at"`
	UpdatedAt  time.Time     `json:"updated_at"`
}

type CreateDueInput struct {
	Name       string        `json:"name"`
	Amount     string        `json:"amount"`
	PeriodType DuePeriodType `json:"period_type"`
}

type UpdateDueInput struct {
	Name       *string        `json:"name"`
	Amount     *string        `json:"amount"`
	PeriodType *DuePeriodType `json:"period_type"`
	IsActive   *bool          `json:"is_active"`
}

// BillStatus represents current status of a bill.
type BillStatus string

const (
	BillStatusUnpaid    BillStatus = "unpaid"
	BillStatusPaid      BillStatus = "paid"
	BillStatusCancelled BillStatus = "cancelled"
)

// Bill represents a bill issued to a household occupancy.
type Bill struct {
	ID                   string     `json:"id"`
	RTID                 string     `json:"rt_id"`
	HouseholdOccupancyID string     `json:"household_occupancy_id"`
	DueID                string     `json:"due_id"`
	Amount               string     `json:"amount"` // String decimal
	Period               string     `json:"period"` // e.g. "2026-09"
	DueDate              string     `json:"due_date"`
	Status               BillStatus `json:"status"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`

	// Projected fields
	DueName     *string `json:"due_name,omitempty"`
	HouseNumber *string `json:"house_number,omitempty"`
	HeadName    *string `json:"head_name,omitempty"`
}

type CreateBillInput struct {
	HouseholdOccupancyID string `json:"household_occupancy_id"`
	DueID                string `json:"due_id"`
	Amount               string `json:"amount"`
	Period               string `json:"period"`
	DueDate              string `json:"due_date"` // YYYY-MM-DD
}

type GenerateBillsInput struct {
	DueID   string `json:"due_id"`
	Period  string `json:"period"`
	DueDate string `json:"due_date"` // YYYY-MM-DD
}

// PaymentMethod represents payment channel.
type PaymentMethod string

const (
	PaymentMethodCash     PaymentMethod = "CASH"
	PaymentMethodTransfer PaymentMethod = "TRANSFER"
)

// PaymentOrigin represents who initiated the payment recording.
type PaymentOrigin string

const (
	PaymentOriginSelfSubmitted PaymentOrigin = "SELF_SUBMITTED"
	PaymentOriginStaffRecorded PaymentOrigin = "STAFF_RECORDED"
)

// PaymentStatus represents verification state.
type PaymentStatus string

const (
	PaymentStatusPending  PaymentStatus = "PENDING"
	PaymentStatusApproved PaymentStatus = "APPROVED"
	PaymentStatusRejected PaymentStatus = "REJECTED"
)

// Payment represents a recorded payment for a bill.
type Payment struct {
	ID              string        `json:"id"`
	RTID            string        `json:"rt_id"`
	BillID          string        `json:"bill_id"`
	Amount          string        `json:"amount"` // String decimal
	Method          PaymentMethod `json:"method"`
	Origin          PaymentOrigin `json:"origin"`
	Status          PaymentStatus `json:"status"`
	ProofPath       *string       `json:"proof_path,omitempty"`
	PaidAt          time.Time     `json:"paid_at"`
	VerifiedBy      *string       `json:"verified_by,omitempty"`
	VerifiedAt      *time.Time    `json:"verified_at,omitempty"`
	RejectionReason *string       `json:"rejection_reason,omitempty"`
	Notes           *string       `json:"notes,omitempty"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}

type CreatePaymentInput struct {
	BillID    string        `json:"bill_id"`
	Amount    string        `json:"amount"`
	Method    PaymentMethod `json:"method"`
	ProofPath *string       `json:"proof_path,omitempty"`
	Notes     *string       `json:"notes,omitempty"`
}

type VerifyPaymentInput struct {
	Action          string  `json:"action"` // "approve" or "reject"
	RejectionReason *string `json:"rejection_reason,omitempty"`
}

// TransactionType represents ledger entry type.
type TransactionType string

const (
	TransactionTypeIncome  TransactionType = "income"
	TransactionTypeExpense TransactionType = "expense"
)

// TransactionStatus represents immutable ledger transaction status.
type TransactionStatus string

const (
	TransactionStatusDraft     TransactionStatus = "draft"
	TransactionStatusPosted    TransactionStatus = "posted"
	TransactionStatusCancelled TransactionStatus = "cancelled"
)

// Transaction represents an immutable financial ledger entry.
type Transaction struct {
	ID                    string            `json:"id"`
	RTID                  string            `json:"rt_id"`
	CategoryID            string            `json:"category_id"`
	Amount                string            `json:"amount"` // String decimal, strictly positive
	Type                  TransactionType   `json:"type"`
	Status                TransactionStatus `json:"status"`
	ReversesTransactionID *string           `json:"reverses_transaction_id,omitempty"`
	PaymentID             *string           `json:"payment_id,omitempty"`
	Description           string            `json:"description"`
	OccurredAt            time.Time         `json:"occurred_at"`
	CreatedAt             time.Time         `json:"created_at"`
	UpdatedAt             time.Time         `json:"updated_at"`

	CategoryName *string `json:"category_name,omitempty"`
}

type CreateTransactionInput struct {
	CategoryID  string          `json:"category_id"`
	Amount      string          `json:"amount"`
	Type        TransactionType `json:"type"`
	Description string          `json:"description"`
	OccurredAt  *string         `json:"occurred_at,omitempty"` // RFC3339 or YYYY-MM-DD
}

type ReverseTransactionInput struct {
	Reason string `json:"reason"`
}

// BalanceSummary represents calculated net financial balance from posted ledger entries.
type BalanceSummary struct {
	TotalIncome      string `json:"total_income"`
	TotalExpense     string `json:"total_expense"`
	NetBalance       string `json:"net_balance"`
	TransactionCount int    `json:"transaction_count"`
}

// AuditLog represents an immutable record in audit_logs.
type AuditLog struct {
	ID         string          `json:"id"`
	RTID       string          `json:"rt_id"`
	UserID     *string         `json:"user_id,omitempty"`
	Action     string          `json:"action"`
	EntityType string          `json:"entity_type"`
	EntityID   string          `json:"entity_id"`
	OldValues  json.RawMessage `json:"old_values,omitempty"`
	NewValues  json.RawMessage `json:"new_values,omitempty"`
	IPAddress  *string         `json:"ip_address,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}

func (c *CreateCategoryInput) Validate() error {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return errors.New("category name is required")
	}
	if c.Type != CategoryTypeIncome && c.Type != CategoryTypeExpense {
		return ErrInvalidCategoryType
	}
	return nil
}

func (d *CreateDueInput) Validate() error {
	d.Name = strings.TrimSpace(d.Name)
	if d.Name == "" {
		return errors.New("dues name is required")
	}
	cents, canon, err := ParseMoney(d.Amount)
	if err != nil || cents <= 0 {
		return ErrInvalidAmount
	}
	d.Amount = canon
	if d.PeriodType != DuePeriodMonthly && d.PeriodType != DuePeriodYearly && d.PeriodType != DuePeriodOneTime {
		return ErrInvalidDuePeriod
	}
	return nil
}

func (b *CreateBillInput) Validate() error {
	if strings.TrimSpace(b.HouseholdOccupancyID) == "" {
		return errors.New("household_occupancy_id is required")
	}
	if strings.TrimSpace(b.DueID) == "" {
		return errors.New("due_id is required")
	}
	if strings.TrimSpace(b.Period) == "" {
		return errors.New("period is required")
	}
	if strings.TrimSpace(b.DueDate) == "" {
		return errors.New("due_date is required")
	}
	if _, err := time.Parse("2006-01-02", b.DueDate); err != nil {
		return fmt.Errorf("due_date must be YYYY-MM-DD: %w", err)
	}
	cents, canon, err := ParseMoney(b.Amount)
	if err != nil || cents <= 0 {
		return ErrInvalidAmount
	}
	b.Amount = canon
	return nil
}

func (p *CreatePaymentInput) Validate() error {
	if strings.TrimSpace(p.BillID) == "" {
		return errors.New("bill_id is required")
	}
	if p.Method != PaymentMethodCash && p.Method != PaymentMethodTransfer {
		return errors.New("invalid payment method: must be CASH or TRANSFER")
	}
	cents, canon, err := ParseMoney(p.Amount)
	if err != nil || cents <= 0 {
		return ErrInvalidAmount
	}
	p.Amount = canon
	return nil
}

func (t *CreateTransactionInput) Validate() error {
	if strings.TrimSpace(t.CategoryID) == "" {
		return errors.New("category_id is required")
	}
	t.Description = strings.TrimSpace(t.Description)
	if t.Description == "" {
		return errors.New("description is required")
	}
	if t.Type != TransactionTypeIncome && t.Type != TransactionTypeExpense {
		return ErrInvalidCategoryType
	}
	cents, canon, err := ParseMoney(t.Amount)
	if err != nil || cents <= 0 {
		return ErrInvalidAmount
	}
	t.Amount = canon
	return nil
}
