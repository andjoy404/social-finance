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
	ErrBillCannotCancel     = errors.New("bill cannot be cancelled: has approved payments")
	ErrInvalidStatus        = errors.New("invalid status transition")
	ErrTransactionPosted    = errors.New("transaction is already posted and immutable")
	ErrAlreadyReversed      = errors.New("transaction is already reversed")
	ErrCannotReverseNonPost = errors.New("cannot reverse a non-posted transaction")
	ErrInvalidDuePeriod     = errors.New("invalid period_type: must be monthly, yearly, or one_time")
	ErrInvalidCategoryType  = errors.New("invalid category type: must be income or expense")
	ErrInvalidAmount        = errors.New("amount must be greater than zero")
	ErrOverpayment          = errors.New("payment amount exceeds remaining balance")
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

// PaymentRule represents how payment amounts are handled for a due type.
type PaymentRule string

const (
	PaymentRuleFull    PaymentRule = "full"    // warga must pay the full remaining balance
	PaymentRulePartial PaymentRule = "partial" // warga may pay any positive amount ≤ remaining balance
)

// Due represents RT dues configuration.
type Due struct {
	ID          string        `json:"id"`
	RTID        string        `json:"rt_id"`
	RTCode      string        `json:"rt_code"`
	Name        string        `json:"name"`
	Amount      string        `json:"amount"` // String decimal NUMERIC(15,2)
	PeriodType  DuePeriodType `json:"period_type"`
	PaymentRule PaymentRule   `json:"payment_rule"`
	IsVoluntary bool          `json:"is_voluntary"`
	IsActive    bool          `json:"is_active"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

type CreateDueInput struct {
	Name        string        `json:"name"`
	Amount      string        `json:"amount"`
	PeriodType  DuePeriodType `json:"period_type"`
	PaymentRule PaymentRule   `json:"payment_rule"` // defaults to "full"
	IsVoluntary bool          `json:"is_voluntary"`
}

type UpdateDueInput struct {
	Name        *string        `json:"name"`
	Amount      *string        `json:"amount"`
	PeriodType  *DuePeriodType `json:"period_type"`
	PaymentRule *PaymentRule   `json:"payment_rule,omitempty"`
	IsVoluntary *bool          `json:"is_voluntary,omitempty"`
	IsActive    *bool          `json:"is_active"`
}

// BillStatus represents current status of a bill.
type BillStatus string

const (
	BillStatusUnpaid    BillStatus = "unpaid"
	BillStatusPartial   BillStatus = "partial"
	BillStatusPaid      BillStatus = "paid"
	BillStatusCancelled BillStatus = "cancelled"
)

// Bill represents a bill issued to a household occupancy.
type Bill struct {
	ID                   string     `json:"id"`
	RTID                 string     `json:"rt_id"`
	RTCode               string     `json:"rt_code"`
	HouseholdOccupancyID string     `json:"household_occupancy_id"`
	DueID                string     `json:"due_id"`
	Amount               string     `json:"amount"` // String decimal
	Period               string     `json:"period"` // e.g. "2026-09"
	DueDate              string     `json:"due_date"`
	Status               BillStatus `json:"status"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`

	// Enriched fields from JOIN queries
	DueName       *string `json:"due_name,omitempty"`
	HouseNumber   *string `json:"house_number,omitempty"`
	HeadName      *string `json:"head_name,omitempty"`
	IsVoluntary   bool    `json:"is_voluntary"`
	PaidAmount    *string `json:"paid_amount,omitempty"`
	PendingAmount *string `json:"pending_amount,omitempty"`
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

// Payment represents a recorded payment for a bill or a voluntary donation.
// For bill payments, BillID is set and DueID is NULL.
// For donations, DueID is set and BillID is NULL.
type Payment struct {
	ID                string        `json:"id"`
	RTID              string        `json:"rt_id"`
	BillID            *string       `json:"bill_id"`
	DueID             *string       `json:"due_id,omitempty"`
	SubmittedBy       *string       `json:"submitted_by,omitempty"`
	SubmitterName     *string       `json:"submitter_name,omitempty"`
	Amount            string        `json:"amount"` // String decimal
	Method            PaymentMethod `json:"method"`
	Origin            PaymentOrigin `json:"origin"`
	Status            PaymentStatus `json:"status"`
	ProofPath         *string       `json:"proof_path,omitempty"`
	PaidAt            time.Time     `json:"paid_at"`
	VerifiedBy        *string       `json:"verified_by,omitempty"`
	VerifiedAt        *time.Time    `json:"verified_at,omitempty"`
	VerifiedByName    *string       `json:"verified_by_name,omitempty"`
	RejectionReason   *string       `json:"rejection_reason,omitempty"`
	Notes             *string       `json:"notes,omitempty"`
	OfficerNotes      *string       `json:"officer_notes,omitempty"`
	PaymentNo         *string       `json:"payment_no,omitempty"`
	CreatedAt         time.Time     `json:"created_at"`
	UpdatedAt         time.Time     `json:"updated_at"`
	ProofOriginalName *string       `json:"proof_original_name,omitempty"`
	ProofMimeType     *string       `json:"proof_mime_type,omitempty"`
	ProofSizeBytes    *int64        `json:"proof_size_bytes,omitempty"`
	ProofUploadedAt   *time.Time    `json:"proof_uploaded_at,omitempty"`
	ProofUploadedBy   *string       `json:"proof_uploaded_by,omitempty"`
}

type CreatePaymentInput struct {
	BillID            *string       `json:"bill_id,omitempty"`
	DueID             *string       `json:"due_id,omitempty"`
	SubmittedBy       *string       `json:"-"`
	Amount            string        `json:"amount"`
	Method            PaymentMethod `json:"method"`
	ProofPath         *string       `json:"proof_path,omitempty"`
	Notes             *string       `json:"notes,omitempty"`
	ProofOriginalName *string       `json:"proof_original_name,omitempty"`
	ProofMimeType     *string       `json:"proof_mime_type,omitempty"`
	ProofSizeBytes    *int64        `json:"proof_size_bytes,omitempty"`
	ProofUploadedBy   *string       `json:"proof_uploaded_by,omitempty"`
}

type VerifyPaymentInput struct {
	Action          string  `json:"action"` // "approve" or "reject"
	RejectionReason *string `json:"rejection_reason,omitempty"`
	OfficerNotes    *string `json:"officer_notes,omitempty"`
}

// VerifyPaymentResult carries the outcome of VerifyPayment.
// OverpaymentRejected is true when the bendahara tried to approve
// a payment that would exceed the remaining balance.  The payment
// row is already updated to REJECTED; the handler must COMMIT the
// transaction (not roll back) and return HTTP 400.
type VerifyPaymentResult struct {
	Payment             *Payment
	OverpaymentRejected bool
}

// PaymentAllocation links a payment to the specific bill portion it covers.
type PaymentAllocation struct {
	ID        string    `json:"id"`
	PaymentID string    `json:"payment_id"`
	BillID    string    `json:"bill_id"`
	Amount    string    `json:"amount"` // String decimal
	CreatedAt time.Time `json:"created_at"`
}

// CreateMultiPeriodPaymentInput describes a multi-period payment spanning multiple bills.
type CreateMultiPeriodPaymentInput struct {
	BillIDs           []string          `json:"bill_ids"`
	Allocations       []AllocationEntry `json:"allocations"`
	Method            PaymentMethod     `json:"method"`
	ProofPath         *string           `json:"proof_path,omitempty"`
	Notes             *string           `json:"notes,omitempty"`
	ProofOriginalName *string           `json:"proof_original_name,omitempty"`
	ProofMimeType     *string           `json:"proof_mime_type,omitempty"`
	ProofSizeBytes    *int64            `json:"proof_size_bytes,omitempty"`
	ProofUploadedBy   *string           `json:"proof_uploaded_by,omitempty"`
}

// AllocationEntry maps a bill to its allocated amount within a multi-period payment.
type AllocationEntry struct {
	BillID string `json:"bill_id"`
	Amount string `json:"amount"`
}

// MultiPeriodPaymentResult carries the outcome of CreateMultiPeriodPayment.
type MultiPeriodPaymentResult struct {
	Payment     *Payment             `json:"payment"`
	Allocations []*PaymentAllocation `json:"allocations"`
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

	CategoryName  *string `json:"category_name,omitempty"`
	PaymentNo     *string `json:"payment_no,omitempty"`
	ReferenceNo   *string `json:"reference_no,omitempty"`
	PaymentMethod *string `json:"payment_method,omitempty"`
	CreatedBy     *string `json:"created_by,omitempty"`
	OfficerName   *string `json:"officer_name,omitempty"`
	DueName       *string `json:"due_name,omitempty"`
	CitizenName   *string `json:"citizen_name,omitempty"`
	HouseNumber   *string `json:"house_number,omitempty"`
}

type CreateTransactionInput struct {
	CategoryID    string          `json:"category_id,omitempty"`
	Amount        string          `json:"amount"`
	Type          TransactionType `json:"type"`
	Description   string          `json:"description"`
	OccurredAt    *string         `json:"occurred_at,omitempty"` // RFC3339 or YYYY-MM-DD
	PaymentMethod *string         `json:"payment_method,omitempty"`
}

type UpdateTransactionInput struct {
	CategoryID    *string          `json:"category_id,omitempty"`
	Amount        *string          `json:"amount,omitempty"`
	Type          *TransactionType `json:"type,omitempty"`
	Description   *string          `json:"description,omitempty"`
	OccurredAt    *string          `json:"occurred_at,omitempty"`
	PaymentMethod *string          `json:"payment_method,omitempty"`
}

type DashboardPaymentMethodBreakdown struct {
	CashAmount     string `json:"cash_amount"`
	TransferAmount string `json:"transfer_amount"`
	CashCount      int    `json:"cash_count"`
	TransferCount  int    `json:"transfer_count"`
}

type DashboardSummary struct {
	Balance                string                          `json:"balance"`
	MonthlyIncome          string                          `json:"monthly_income"`
	MonthlyExpense         string                          `json:"monthly_expense"`
	NetMovement            string                          `json:"net_movement"`
	DuesProgress           DashboardDuesProgress           `json:"dues_progress"`
	PaymentMethodBreakdown DashboardPaymentMethodBreakdown `json:"payment_method_breakdown"`
	Chart                  []DashboardChartPoint           `json:"chart"`
	RecentTransactions     []*Transaction                  `json:"recent_transactions"`
}

type DashboardDuesProgress struct {
	PaidHouseholds  int `json:"paid_households"`
	TotalHouseholds int `json:"total_households"`
	Percentage      int `json:"percentage"`
}

type DashboardChartPoint struct {
	Month        string `json:"month"`
	Period       string `json:"period"`
	Income       int64  `json:"income"`
	Expense      int64  `json:"expense"`
	IncomeCount  int    `json:"income_count"`
	ExpenseCount int    `json:"expense_count"`
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

// PeriodBalanceSummary represents monthly/period balance sheet.
type PeriodBalanceSummary struct {
	OpeningBalance   string `json:"opening_balance"`
	TotalIncome      string `json:"total_income"`
	TotalExpense     string `json:"total_expense"`
	NetMovement      string `json:"net_movement"`
	ClosingBalance   string `json:"closing_balance"`
	TransactionCount int    `json:"transaction_count"`
}

// UpdateOpeningBalanceInput represents request payload for opening balance correction.
type UpdateOpeningBalanceInput struct {
	OpeningBalance string `json:"opening_balance"`
	Reason         string `json:"reason"`
}

// PeriodOpeningBalanceResponse represents response for opening balance correction.
type PeriodOpeningBalanceResponse struct {
	Period               string `json:"period"`
	OpeningBalanceBefore string `json:"opening_balance_before"`
	OpeningBalanceAfter  string `json:"opening_balance_after"`
	Source               string `json:"source"`
	CorrectedBy          string `json:"corrected_by"`
	CorrectedAt          string `json:"corrected_at"`
	Reason               string `json:"reason"`
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
	if err != nil {
		return ErrInvalidAmount
	}
	if !d.IsVoluntary && cents <= 0 {
		return ErrInvalidAmount
	}
	if d.IsVoluntary && cents < 0 {
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
	hasBill := p.BillID != nil && strings.TrimSpace(*p.BillID) != ""
	hasDue := p.DueID != nil && strings.TrimSpace(*p.DueID) != ""
	if hasBill == hasDue {
		return errors.New("exactly one of bill_id or due_id is required")
	}
	if p.BillID != nil {
		value := strings.TrimSpace(*p.BillID)
		if value == "" {
			p.BillID = nil
		} else {
			p.BillID = &value
		}
	}
	if p.DueID != nil {
		value := strings.TrimSpace(*p.DueID)
		if value == "" {
			p.DueID = nil
		} else {
			p.DueID = &value
		}
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
	normType := strings.ToLower(strings.TrimSpace(string(t.Type)))
	if normType == "income" || normType == "masuk" {
		t.Type = TransactionTypeIncome
	} else if normType == "expense" || normType == "keluar" {
		t.Type = TransactionTypeExpense
	} else {
		return ErrInvalidCategoryType
	}
	cents, canon, err := ParseMoney(t.Amount)
	if err != nil || cents <= 0 {
		return ErrInvalidAmount
	}
	t.Amount = canon
	return nil
}
