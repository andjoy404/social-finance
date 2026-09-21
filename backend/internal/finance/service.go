package finance

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"social-finance/internal/database"
)

// Service encapsulates business logic for the Finance domain.
type Service struct {
	pool *database.Pool
}

func NewService(pool *database.Pool) *Service {
	return &Service{pool: pool}
}

// --- Categories ---

func (s *Service) CreateCategory(ctx context.Context, tx *sql.Tx, rtID, userID string, in CreateCategoryInput) (*FinancialCategory, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	cat, err := CategoryCreate(ctx, tx, rtID, in)
	if err != nil {
		return nil, err
	}
	auditPayload, _ := json.Marshal(cat)
	_ = AuditLogInsert(ctx, tx, AuditLog{
		RTID:       rtID,
		UserID:     &userID,
		Action:     "create_category",
		EntityType: "financial_category",
		EntityID:   cat.ID,
		NewValues:  auditPayload,
	})
	return cat, nil
}

func (s *Service) UpdateCategory(ctx context.Context, tx *sql.Tx, id, rtID, userID string, in UpdateCategoryInput) (*FinancialCategory, error) {
	cat, err := CategoryUpdate(ctx, tx, id, rtID, in)
	if err != nil {
		return nil, err
	}
	auditPayload, _ := json.Marshal(cat)
	_ = AuditLogInsert(ctx, tx, AuditLog{
		RTID:       rtID,
		UserID:     &userID,
		Action:     "update_category",
		EntityType: "financial_category",
		EntityID:   cat.ID,
		NewValues:  auditPayload,
	})
	return cat, nil
}

func (s *Service) DeactivateCategory(ctx context.Context, tx *sql.Tx, id, rtID, userID string) error {
	if err := CategoryDeactivate(ctx, tx, id, rtID); err != nil {
		return err
	}
	_ = AuditLogInsert(ctx, tx, AuditLog{
		RTID:       rtID,
		UserID:     &userID,
		Action:     "deactivate_category",
		EntityType: "financial_category",
		EntityID:   id,
	})
	return nil
}

// --- Dues ---

func (s *Service) CreateDue(ctx context.Context, tx *sql.Tx, rtID, userID string, in CreateDueInput) (*Due, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	due, err := DueCreate(ctx, tx, rtID, in)
	if err != nil {
		return nil, err
	}
	auditPayload, _ := json.Marshal(due)
	_ = AuditLogInsert(ctx, tx, AuditLog{
		RTID:       rtID,
		UserID:     &userID,
		Action:     "create_due",
		EntityType: "due",
		EntityID:   due.ID,
		NewValues:  auditPayload,
	})
	return due, nil
}

func (s *Service) UpdateDue(ctx context.Context, tx *sql.Tx, id, rtID, userID string, in UpdateDueInput) (*Due, error) {
	if in.Amount != nil {
		cents, canon, err := ParseMoney(*in.Amount)
		if err != nil || cents <= 0 {
			return nil, ErrInvalidAmount
		}
		in.Amount = &canon
	}
	due, err := DueUpdate(ctx, tx, id, rtID, in)
	if err != nil {
		return nil, err
	}
	auditPayload, _ := json.Marshal(due)
	_ = AuditLogInsert(ctx, tx, AuditLog{
		RTID:       rtID,
		UserID:     &userID,
		Action:     "update_due",
		EntityType: "due",
		EntityID:   due.ID,
		NewValues:  auditPayload,
	})
	return due, nil
}

func (s *Service) DeactivateDue(ctx context.Context, tx *sql.Tx, id, rtID, userID string) error {
	if err := DueDeactivate(ctx, tx, id, rtID); err != nil {
		return err
	}
	_ = AuditLogInsert(ctx, tx, AuditLog{
		RTID:       rtID,
		UserID:     &userID,
		Action:     "deactivate_due",
		EntityType: "due",
		EntityID:   id,
	})
	return nil
}

// --- Bills ---

func (s *Service) GenerateBills(ctx context.Context, tx *sql.Tx, rtID, userID string, in GenerateBillsInput) ([]*Bill, error) {
	due, err := DueGetByID(ctx, tx, in.DueID, rtID)
	if err != nil {
		return nil, fmt.Errorf("due not found: %w", err)
	}
	if !due.IsActive {
		return nil, errors.New("cannot generate bills for inactive due")
	}

	if strings.TrimSpace(in.Period) == "" {
		return nil, errors.New("period is required")
	}
	if strings.TrimSpace(in.DueDate) == "" {
		return nil, errors.New("due_date is required")
	}
	if _, err := time.Parse("2006-01-02", in.DueDate); err != nil {
		return nil, fmt.Errorf("invalid due_date format (YYYY-MM-DD): %w", err)
	}

	occupancyIDs, err := GetActiveCurrentOccupancyIDs(ctx, tx, rtID)
	if err != nil {
		return nil, err
	}

	var generated []*Bill
	for _, occID := range occupancyIDs {
		createInput := CreateBillInput{
			HouseholdOccupancyID: occID,
			DueID:                due.ID,
			Amount:               due.Amount,
			Period:               in.Period,
			DueDate:              in.DueDate,
		}
		bill, err := BillCreate(ctx, tx, rtID, createInput)
		if err != nil {
			// Skip duplicates if already billed
			continue
		}
		generated = append(generated, bill)
	}

	_ = AuditLogInsert(ctx, tx, AuditLog{
		RTID:       rtID,
		UserID:     &userID,
		Action:     "generate_bills",
		EntityType: "bill",
		EntityID:   in.Period,
		NewValues:  json.RawMessage(fmt.Sprintf(`{"due_id":"%s","period":"%s","count":%d}`, in.DueID, in.Period, len(generated))),
	})

	return generated, nil
}

func (s *Service) CreateBill(ctx context.Context, tx *sql.Tx, rtID, userID string, in CreateBillInput) (*Bill, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	bill, err := BillCreate(ctx, tx, rtID, in)
	if err != nil {
		return nil, err
	}
	auditPayload, _ := json.Marshal(bill)
	_ = AuditLogInsert(ctx, tx, AuditLog{
		RTID:       rtID,
		UserID:     &userID,
		Action:     "create_bill",
		EntityType: "bill",
		EntityID:   bill.ID,
		NewValues:  auditPayload,
	})
	return bill, nil
}

// --- Payments ---

func (s *Service) CreatePayment(ctx context.Context, tx *sql.Tx, rtID, userID, userRole string, in CreatePaymentInput) (*Payment, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}

	bill, err := BillGetByID(ctx, tx, in.BillID, rtID)
	if err != nil {
		return nil, fmt.Errorf("bill not found in this RT: %w", err)
	}
	if bill.Status == BillStatusPaid {
		return nil, ErrBillAlreadyPaid
	}
	if bill.Status == BillStatusCancelled {
		return nil, ErrBillCancelled
	}

	// If user is warga, verify ownership
	if userRole == "warga" {
		occID, err := GetOccupancyIDForUser(ctx, tx, rtID, userID)
		if err != nil {
			return nil, fmt.Errorf("error verifying warga occupancy: %w", err)
		}
		if occID != "" && bill.HouseholdOccupancyID != occID {
			return nil, errors.New("cannot pay bill belonging to another household")
		}
	}

	var origin PaymentOrigin
	var status PaymentStatus
	var verifiedBy *string
	var verifiedAt *time.Time

	if userRole == "warga" {
		origin = PaymentOriginSelfSubmitted
		status = PaymentStatusPending
	} else {
		// Staff recorded by pengurus or bendahara -> immediately approved
		origin = PaymentOriginStaffRecorded
		status = PaymentStatusApproved
		verifiedBy = &userID
		now := time.Now()
		verifiedAt = &now
	}

	payment, err := PaymentCreate(ctx, tx, rtID, in, origin, status, verifiedBy, verifiedAt)
	if err != nil {
		return nil, err
	}

	// If approved immediately (staff-recorded):
	if status == PaymentStatusApproved {
		if err := BillUpdateStatus(ctx, tx, bill.ID, rtID, BillStatusPaid); err != nil {
			return nil, err
		}

		// Automatically post income transaction into ledger
		catID, err := FindOrCreateDefaultIncomeCategory(ctx, tx, rtID, "Iuran Warga")
		if err != nil {
			return nil, err
		}
		desc := fmt.Sprintf("Payment for bill %s (Period: %s)", bill.ID, bill.Period)
		now := time.Now()
		_, err = TransactionCreate(ctx, tx, rtID, CreateTransactionInput{
			CategoryID:  catID,
			Amount:      payment.Amount,
			Type:        TransactionTypeIncome,
			Description: desc,
		}, TransactionStatusPosted, nil, &payment.ID, now)
		if err != nil {
			return nil, fmt.Errorf("post ledger transaction for payment: %w", err)
		}
	}

	auditPayload, _ := json.Marshal(payment)
	_ = AuditLogInsert(ctx, tx, AuditLog{
		RTID:       rtID,
		UserID:     &userID,
		Action:     "create_payment",
		EntityType: "payment",
		EntityID:   payment.ID,
		NewValues:  auditPayload,
	})

	return payment, nil
}

func (s *Service) VerifyPayment(ctx context.Context, tx *sql.Tx, rtID, verifierID, paymentID string, in VerifyPaymentInput) (*Payment, error) {
	payment, err := PaymentGetByID(ctx, tx, paymentID, rtID)
	if err != nil {
		return nil, err
	}
	if payment.Status != PaymentStatusPending {
		return nil, errors.New("only pending payments can be verified")
	}

	action := strings.ToLower(strings.TrimSpace(in.Action))
	if action != "approve" && action != "reject" {
		return nil, errors.New("action must be 'approve' or 'reject'")
	}

	bill, err := BillGetByID(ctx, tx, payment.BillID, rtID)
	if err != nil {
		return nil, fmt.Errorf("bill not found: %w", err)
	}

	var newStatus PaymentStatus
	var reason *string
	if action == "approve" {
		newStatus = PaymentStatusApproved
	} else {
		newStatus = PaymentStatusRejected
		if in.RejectionReason != nil && strings.TrimSpace(*in.RejectionReason) != "" {
			trimmed := strings.TrimSpace(*in.RejectionReason)
			reason = &trimmed
		}
	}

	verifiedPayment, err := PaymentVerify(ctx, tx, paymentID, rtID, newStatus, verifierID, reason)
	if err != nil {
		return nil, err
	}

	if newStatus == PaymentStatusApproved {
		// Update bill status to paid
		if err := BillUpdateStatus(ctx, tx, bill.ID, rtID, BillStatusPaid); err != nil {
			return nil, err
		}

		// Automatically post income transaction into ledger
		catID, err := FindOrCreateDefaultIncomeCategory(ctx, tx, rtID, "Iuran Warga")
		if err != nil {
			return nil, err
		}
		desc := fmt.Sprintf("Payment for bill %s (Period: %s)", bill.ID, bill.Period)
		now := time.Now()
		_, err = TransactionCreate(ctx, tx, rtID, CreateTransactionInput{
			CategoryID:  catID,
			Amount:      verifiedPayment.Amount,
			Type:        TransactionTypeIncome,
			Description: desc,
		}, TransactionStatusPosted, nil, &verifiedPayment.ID, now)
		if err != nil {
			return nil, fmt.Errorf("post ledger transaction for approved payment: %w", err)
		}
	}

	auditPayload, _ := json.Marshal(verifiedPayment)
	_ = AuditLogInsert(ctx, tx, AuditLog{
		RTID:       rtID,
		UserID:     &verifierID,
		Action:     "verify_payment",
		EntityType: "payment",
		EntityID:   verifiedPayment.ID,
		NewValues:  auditPayload,
	})

	return verifiedPayment, nil
}

// --- Transactions Ledger & Reversals ---

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

	occurredAt := time.Now()
	if in.OccurredAt != nil && *in.OccurredAt != "" {
		parsed, err := time.Parse(time.RFC3339, *in.OccurredAt)
		if err != nil {
			parsedDate, errDate := time.Parse("2006-01-02", *in.OccurredAt)
			if errDate != nil {
				return nil, errors.New("occurred_at must be RFC3339 or YYYY-MM-DD")
			}
			occurredAt = parsedDate
		} else {
			occurredAt = parsed
		}
	}

	trans, err := TransactionCreate(ctx, tx, rtID, in, TransactionStatusPosted, nil, nil, occurredAt)
	if err != nil {
		return nil, err
	}

	auditPayload, _ := json.Marshal(trans)
	_ = AuditLogInsert(ctx, tx, AuditLog{
		RTID:       rtID,
		UserID:     &userID,
		Action:     "create_transaction",
		EntityType: "transaction",
		EntityID:   trans.ID,
		NewValues:  auditPayload,
	})

	return trans, nil
}

func (s *Service) ReverseTransaction(ctx context.Context, tx *sql.Tx, rtID, userID, origID string, in ReverseTransactionInput) (*Transaction, error) {
	orig, err := TransactionGetByID(ctx, tx, origID, rtID)
	if err != nil {
		return nil, err
	}
	if orig.Status != TransactionStatusPosted {
		return nil, ErrCannotReverseNonPost
	}
	if orig.ReversesTransactionID != nil {
		return nil, errors.New("cannot reverse a reversal transaction")
	}

	// Check if already reversed
	var alreadyReversedCount int
	err = tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM transactions WHERE reverses_transaction_id = $1 AND rt_id = $2`,
		orig.ID, rtID,
	).Scan(&alreadyReversedCount)
	if err != nil {
		return nil, err
	}
	if alreadyReversedCount > 0 {
		return nil, ErrAlreadyReversed
	}

	// Determine opposite type
	var revType TransactionType
	if orig.Type == TransactionTypeIncome {
		revType = TransactionTypeExpense
	} else {
		revType = TransactionTypeIncome
	}

	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		reason = "Compensating reversal"
	}
	desc := fmt.Sprintf("Reversal of %s: %s", orig.ID, reason)

	// Create reversal transaction
	now := time.Now()
	revInput := CreateTransactionInput{
		CategoryID:  orig.CategoryID,
		Amount:      orig.Amount,
		Type:        revType,
		Description: desc,
	}
	reversal, err := TransactionCreate(ctx, tx, rtID, revInput, TransactionStatusPosted, &orig.ID, orig.PaymentID, now)
	if err != nil {
		return nil, fmt.Errorf("create reversal transaction: %w", err)
	}

	auditPayload, _ := json.Marshal(reversal)
	_ = AuditLogInsert(ctx, tx, AuditLog{
		RTID:       rtID,
		UserID:     &userID,
		Action:     "reverse_transaction",
		EntityType: "transaction",
		EntityID:   reversal.ID,
		NewValues:  auditPayload,
	})

	return reversal, nil
}
