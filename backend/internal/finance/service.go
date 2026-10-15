package finance

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"social-finance/internal/database"
	"social-finance/internal/storage"
)

// Notifier defines the interface for emitting notifications from the finance domain.
type Notifier interface {
	NotifyBillCreated(ctx context.Context, rtID, billID string, recipientUserIDs []string)
	NotifyPaymentSubmitted(ctx context.Context, rtID, billID, paymentID string)
	NotifyPaymentVerified(ctx context.Context, rtID, billID, paymentID, submitterUserID, status, rejectionReason string)
}

// Service encapsulates business logic for the Finance domain.
type Service struct {
	pool     *database.Pool
	storage  storage.Service
	notifier Notifier
}

func NewService(pool *database.Pool, opts ...storage.Service) *Service {
	var store storage.Service
	if len(opts) > 0 && opts[0] != nil {
		store = opts[0]
	} else {
		store = storage.NewMockStorage()
	}
	return &Service{pool: pool, storage: store}
}

func (s *Service) SetNotifier(n Notifier) {
	s.notifier = n
}

func (s *Service) Notifier() Notifier {
	return s.notifier
}

func (s *Service) Storage() storage.Service {
	return s.storage
}

func (s *Service) AttachPaymentProof(ctx context.Context, tx *sql.Tx, rtID, userID, userRole, paymentID string, meta *storage.ObjectMetadata) (*Payment, error) {
	payment, err := PaymentGetByID(ctx, tx, paymentID, rtID)
	if err != nil {
		return nil, err
	}

	// Verify permissions
	if userRole == "warga" {
		if payment.BillID == nil {
			if payment.SubmittedBy == nil || *payment.SubmittedBy != userID {
				return nil, errors.New("cannot attach proof to another user's donation")
			}
		} else {
			bill, err := BillGetByID(ctx, tx, *payment.BillID, rtID)
			if err != nil {
				return nil, fmt.Errorf("bill not found: %w", err)
			}
			occID, err := GetOccupancyIDForUser(ctx, tx, rtID, userID)
			if err != nil {
				return nil, fmt.Errorf("error verifying warga occupancy: %w", err)
			}
			if occID == "" || bill.HouseholdOccupancyID != occID {
				return nil, errors.New("cannot attach proof to another household's payment")
			}
		}
	}

	if payment.Status == PaymentStatusRejected {
		return nil, errors.New("cannot attach proof to a rejected payment")
	}

	updatedPayment, err := PaymentAttachProof(ctx, tx, paymentID, rtID, meta.Key, meta.OriginalName, meta.ContentType, meta.Size, userID)
	if err != nil {
		return nil, err
	}

	auditPayload, _ := json.Marshal(map[string]interface{}{
		"payment_id":  paymentID,
		"proof_path":  meta.Key,
		"uploaded_by": userID,
	})
	_ = AuditLogInsert(ctx, tx, AuditLog{
		RTID:       rtID,
		UserID:     &userID,
		Action:     "upload_payment_proof",
		EntityType: "payment",
		EntityID:   updatedPayment.ID,
		NewValues:  auditPayload,
	})

	return updatedPayment, nil
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
	// Default payment_rule to "full" if not specified
	if in.PaymentRule == "" {
		in.PaymentRule = PaymentRuleFull
	}
	// Relax amount validation: when is_voluntary, amount = 0 is allowed
	if !in.IsVoluntary {
		cents, canon, err := ParseMoney(in.Amount)
		if err != nil || cents <= 0 {
			return nil, ErrInvalidAmount
		}
		in.Amount = canon
	} else {
		// Voluntary: amount must be >= 0
		cents, canon, err := ParseMoney(in.Amount)
		if err != nil || cents < 0 {
			return nil, ErrInvalidAmount
		}
		in.Amount = canon
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
		if err != nil || cents < 0 {
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
			return nil, err
		}
		if bill == nil {
			// ON CONFLICT DO NOTHING — already exists for this occupancy+due+period
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
	in.SubmittedBy = &userID

	if in.DueID != nil {
		due, err := DueGetByIDForUpdate(ctx, tx, *in.DueID, rtID)
		if err != nil {
			return nil, fmt.Errorf("due not found in this RT: %w", err)
		}
		if !due.IsActive {
			return nil, errors.New("cannot donate to an inactive due")
		}
		if !due.IsVoluntary {
			return nil, errors.New("donation target must be a voluntary due")
		}
		dueCents, _, err := ParseMoney(due.Amount)
		if err != nil || dueCents != 0 {
			return nil, errors.New("donation target must have a configured amount of Rp0")
		}
		if userRole == "warga" {
			occID, err := GetOccupancyIDForUser(ctx, tx, rtID, userID)
			if err != nil {
				return nil, fmt.Errorf("error verifying warga occupancy: %w", err)
			}
			if occID == "" {
				return nil, errors.New("warga must belong to an active household to donate")
			}
		}

		payment, err := PaymentCreate(ctx, tx, rtID, in, origin, status, verifiedBy, verifiedAt)
		if err != nil {
			return nil, fmt.Errorf("create donation: %w", err)
		}
		if status == PaymentStatusApproved {
			if err := postPaymentIncome(ctx, tx, rtID, payment, fmt.Sprintf("Voluntary donation for due %s", due.Name)); err != nil {
				return nil, err
			}
		}
		auditPayload, _ := json.Marshal(payment)
		_ = AuditLogInsert(ctx, tx, AuditLog{
			RTID: rtID, UserID: &userID, Action: "create_payment", EntityType: "payment",
			EntityID: payment.ID, NewValues: auditPayload,
		})
		return payment, nil
	}

	bill, err := BillGetByIDForUpdate(ctx, tx, *in.BillID, rtID)
	if err != nil {
		return nil, fmt.Errorf("bill not found in this RT: %w", err)
	}

	// Fetch the due to determine payment_rule and voluntary status for amount validation
	due, err := DueGetByID(ctx, tx, bill.DueID, rtID)
	if err != nil {
		return nil, fmt.Errorf("due not found: %w", err)
	}
	isVoluntary := due.IsVoluntary

	if !isVoluntary && bill.Status == BillStatusPaid {
		return nil, ErrBillAlreadyPaid
	}
	if bill.Status == BillStatusCancelled {
		return nil, ErrBillCancelled
	}

	if userRole == "warga" {
		occID, err := GetOccupancyIDForUser(ctx, tx, rtID, userID)
		if err != nil {
			return nil, fmt.Errorf("error verifying warga occupancy: %w", err)
		}
		if occID == "" || bill.HouseholdOccupancyID != occID {
			return nil, errors.New("cannot pay bill belonging to another household")
		}
	}

	// Compute approved total BEFORE branching — both APPROVED and PENDING need it
	approvedTotal, err := GetApprovedPaymentTotal(ctx, tx, bill.ID)
	if err != nil {
		return nil, fmt.Errorf("compute approved payment total: %w", err)
	}

	billCents, _, err := ParseMoney(bill.Amount)
	if err != nil {
		return nil, fmt.Errorf("parse bill amount: %w", err)
	}
	approvedCents, _, err := ParseMoney(approvedTotal)
	if err != nil {
		return nil, fmt.Errorf("parse approved total: %w", err)
	}

	// --- APPROVED (staff-recorded): validate BEFORE insert ---
	if status == PaymentStatusApproved {
		// Parse money strings
		paymentCents, _, err := ParseMoney(in.Amount)
		if err != nil {
			return nil, fmt.Errorf("parse payment amount: %w", err)
		}

		if !isVoluntary {
			remaining := billCents - approvedCents
			if paymentCents > remaining {
				return nil, fmt.Errorf("payment amount %s exceeds remaining balance %s", in.Amount, FormatMoney(remaining))
			}

			// Enforce payment_rule: "full" requires payment == remaining
			if due.PaymentRule == PaymentRuleFull {
				if paymentCents != remaining {
					return nil, fmt.Errorf("this iuran requires full payment: amount must be %s", FormatMoney(remaining))
				}
			}
		}

		// Insert payment NOW (validation passed)
		payment, err := PaymentCreate(ctx, tx, rtID, in, origin, status, verifiedBy, verifiedAt)
		if err != nil {
			return nil, fmt.Errorf("create payment: %w", err)
		}

		// Update bill status
		if isVoluntary {
			if err := BillUpdateStatus(ctx, tx, bill.ID, rtID, BillStatusPaid); err != nil {
				return nil, err
			}
		} else {
			newTotal := approvedCents + paymentCents
			if newTotal >= billCents {
				if err := BillUpdateStatus(ctx, tx, bill.ID, rtID, BillStatusPaid); err != nil {
					return nil, err
				}
			} else {
				if err := BillUpdateStatus(ctx, tx, bill.ID, rtID, BillStatusPartial); err != nil {
					return nil, err
				}
			}
		}

		// Post income transaction into ledger
		catID, err := FindOrCreateDefaultIncomeCategory(ctx, tx, rtID, "Iuran Warga")
		if err != nil {
			return nil, err
		}
		desc := fmt.Sprintf("Payment for bill %s (Period: %s)", bill.ID, bill.Period)
		if isVoluntary {
			desc = fmt.Sprintf("Voluntary contribution for due %s (Bill: %s)", due.Name, bill.ID)
		}
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

	// --- PENDING (warga-submitted): validate limits BEFORE insert ---

	// Compute PENDING total for this bill (to prevent spam/overpayment)
	pendingTotal, err := GetPendingPaymentTotal(ctx, tx, bill.ID)
	if err != nil {
		return nil, fmt.Errorf("compute pending payment total: %w", err)
	}

	// Parse money strings — billCents/paymentCents scoped to APPROVED block above
	paymentCents, _, err := ParseMoney(in.Amount)
	if err != nil {
		return nil, fmt.Errorf("parse payment amount: %w", err)
	}
	pendingCents, _, err := ParseMoney(pendingTotal)
	if err != nil {
		return nil, fmt.Errorf("parse pending total: %w", err)
	}

	if !isVoluntary {
		remaining := billCents - approvedCents - pendingCents
		if paymentCents > remaining {
			return nil, fmt.Errorf("payment amount %s exceeds remaining balance %s", in.Amount, FormatMoney(remaining))
		}

		// Enforce payment_rule: "full" requires payment == remaining
		if due.PaymentRule == PaymentRuleFull {
			if paymentCents != remaining {
				return nil, fmt.Errorf("this iuran requires full payment: amount must be %s", FormatMoney(remaining))
			}
		}
	}

	payment, err := PaymentCreate(ctx, tx, rtID, in, origin, status, verifiedBy, verifiedAt)
	if err != nil {
		return nil, err
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

func postPaymentIncome(ctx context.Context, tx *sql.Tx, rtID string, payment *Payment, description string) error {
	catID, err := FindOrCreateDefaultIncomeCategory(ctx, tx, rtID, "Iuran Warga")
	if err != nil {
		return err
	}
	_, err = TransactionCreate(ctx, tx, rtID, CreateTransactionInput{
		CategoryID: catID, Amount: payment.Amount, Type: TransactionTypeIncome, Description: description,
	}, TransactionStatusPosted, nil, &payment.ID, time.Now())
	if err != nil {
		return fmt.Errorf("post ledger transaction for payment: %w", err)
	}
	return nil
}

func (s *Service) VerifyPayment(ctx context.Context, tx *sql.Tx, rtID, verifierID, paymentID string, in VerifyPaymentInput) (*VerifyPaymentResult, error) {
	payment, err := PaymentGetByIDForUpdate(ctx, tx, paymentID, rtID)
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

	if action == "reject" {
		reason := ""
		if in.RejectionReason != nil && strings.TrimSpace(*in.RejectionReason) != "" {
			reason = strings.TrimSpace(*in.RejectionReason)
		}
		officerNotes := ""
		if in.OfficerNotes != nil && strings.TrimSpace(*in.OfficerNotes) != "" {
			officerNotes = strings.TrimSpace(*in.OfficerNotes)
		}

		verifiedPayment, err := PaymentVerify(ctx, tx, paymentID, rtID, verifierID, PaymentStatusRejected, reason, officerNotes)
		if err != nil {
			return nil, err
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

		return &VerifyPaymentResult{Payment: verifiedPayment}, nil
	}

	// action == "approve"
	if payment.BillID == nil {
		if payment.DueID == nil {
			return nil, errors.New("payment has no target")
		}
		due, err := DueGetByID(ctx, tx, *payment.DueID, rtID)
		if err != nil {
			return nil, fmt.Errorf("donation due not found: %w", err)
		}
		if !due.IsVoluntary {
			return nil, errors.New("donation target is no longer voluntary")
		}
		officerNotes := ""
		if in.OfficerNotes != nil {
			officerNotes = strings.TrimSpace(*in.OfficerNotes)
		}
		verifiedPayment, err := PaymentVerify(ctx, tx, paymentID, rtID, verifierID, PaymentStatusApproved, "", officerNotes)
		if err != nil {
			return nil, fmt.Errorf("approve donation: %w", err)
		}
		if err := postPaymentIncome(ctx, tx, rtID, verifiedPayment, fmt.Sprintf("Voluntary donation for due %s", due.Name)); err != nil {
			return nil, err
		}
		auditPayload, _ := json.Marshal(verifiedPayment)
		_ = AuditLogInsert(ctx, tx, AuditLog{
			RTID: rtID, UserID: &verifierID, Action: "verify_payment", EntityType: "payment",
			EntityID: verifiedPayment.ID, NewValues: auditPayload,
		})
		return &VerifyPaymentResult{Payment: verifiedPayment}, nil
	}

	// Detect whether this is a multi-period payment by querying allocations.
	allocs, err := PaymentAllocationsByPaymentID(ctx, tx, paymentID)
	if err != nil {
		return nil, fmt.Errorf("query payment allocations: %w", err)
	}

	if len(allocs) == 0 {
		// ── Single-period payment (legacy path) ────────────────────────
		return s.verifySinglePayment(ctx, tx, paymentID, rtID, verifierID, payment, in)
	}

	// ── Multi-period payment approval ────────────────────────────────
	return s.verifyMultiPeriodPayment(ctx, tx, paymentID, rtID, verifierID, payment, allocs, in)
}

// verifySinglePayment handles legacy single-bill payment approval.
func (s *Service) verifySinglePayment(ctx context.Context, tx *sql.Tx, paymentID, rtID, verifierID string, payment *Payment, in VerifyPaymentInput) (*VerifyPaymentResult, error) {
	if payment.BillID == nil {
		return nil, errors.New("bill payment has no bill target")
	}
	bill, err := BillGetByIDForUpdate(ctx, tx, *payment.BillID, rtID)
	if err != nil {
		return nil, fmt.Errorf("bill not found: %w", err)
	}
	due, err := DueGetByID(ctx, tx, bill.DueID, rtID)
	if err != nil {
		return nil, fmt.Errorf("due not found: %w", err)
	}

	isVoluntary := due.IsVoluntary

	if !isVoluntary && bill.Status == BillStatusPaid {
		return nil, ErrBillAlreadyPaid
	}
	if bill.Status == BillStatusCancelled {
		return nil, ErrBillCancelled
	}

	// Validate overpayment BEFORE approving payment.
	// approved_total_before does NOT include this payment yet (still PENDING).
	approvedTotal, err := GetApprovedPaymentTotal(ctx, tx, bill.ID)
	if err != nil {
		return nil, fmt.Errorf("compute approved payment total: %w", err)
	}

	billCents, _, err := ParseMoney(bill.Amount)
	if err != nil {
		return nil, fmt.Errorf("parse bill amount: %w", err)
	}
	paymentCents, _, err := ParseMoney(payment.Amount)
	if err != nil {
		return nil, fmt.Errorf("parse payment amount: %w", err)
	}
	approvedCents, _, err := ParseMoney(approvedTotal)
	if err != nil {
		return nil, fmt.Errorf("parse approved total: %w", err)
	}

	if !isVoluntary {
		remaining := billCents - approvedCents
		if paymentCents > remaining {
			// Overpayment detected — reject the payment atomically.
			var rejectionReason string
			if in.RejectionReason != nil && strings.TrimSpace(*in.RejectionReason) != "" {
				rejectionReason = strings.TrimSpace(*in.RejectionReason)
			} else {
				rejectionReason = "Payment amount exceeds remaining balance"
			}
			officerNotes := ""
			if in.OfficerNotes != nil && strings.TrimSpace(*in.OfficerNotes) != "" {
				officerNotes = strings.TrimSpace(*in.OfficerNotes)
			}
			verifiedPayment, err := PaymentVerify(ctx, tx, paymentID, rtID, verifierID, PaymentStatusRejected, rejectionReason, officerNotes)
			if err != nil {
				return nil, fmt.Errorf("reject overpayment: %w", err)
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

			return &VerifyPaymentResult{
				Payment:             verifiedPayment,
				OverpaymentRejected: true,
			}, nil
		}
	}

	// Overpayment check passed — approve the payment
	officerNotes := ""
	if in.OfficerNotes != nil && strings.TrimSpace(*in.OfficerNotes) != "" {
		officerNotes = strings.TrimSpace(*in.OfficerNotes)
	}
	verifiedPayment, err := PaymentVerify(ctx, tx, paymentID, rtID, verifierID, PaymentStatusApproved, "", officerNotes)
	if err != nil {
		return nil, fmt.Errorf("approve payment: %w", err)
	}

	// Update bill status based on new total (this payment + previously approved)
	if isVoluntary {
		if err := BillUpdateStatus(ctx, tx, bill.ID, rtID, BillStatusPaid); err != nil {
			return nil, err
		}
	} else {
		newTotal := approvedCents + paymentCents
		if newTotal >= billCents {
			if err := BillUpdateStatus(ctx, tx, bill.ID, rtID, BillStatusPaid); err != nil {
				return nil, err
			}
		} else {
			if err := BillUpdateStatus(ctx, tx, bill.ID, rtID, BillStatusPartial); err != nil {
				return nil, err
			}
		}
	}

	// Automatically post income transaction into ledger
	catID, err := FindOrCreateDefaultIncomeCategory(ctx, tx, rtID, "Iuran Warga")
	if err != nil {
		return nil, err
	}
	desc := fmt.Sprintf("Payment for bill %s (Period: %s)", bill.ID, bill.Period)
	if isVoluntary {
		desc = fmt.Sprintf("Voluntary contribution for due %s (Bill: %s)", due.Name, bill.ID)
	}
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

	auditPayload, _ := json.Marshal(verifiedPayment)
	_ = AuditLogInsert(ctx, tx, AuditLog{
		RTID:       rtID,
		UserID:     &verifierID,
		Action:     "verify_payment",
		EntityType: "payment",
		EntityID:   verifiedPayment.ID,
		NewValues:  auditPayload,
	})

	return &VerifyPaymentResult{Payment: verifiedPayment}, nil
}

// verifyMultiPeriodPayment handles approval for payments spanning multiple bills.
// All bills are validated before any state changes; ONE KAS transaction is posted.
func (s *Service) verifyMultiPeriodPayment(ctx context.Context, tx *sql.Tx, paymentID, rtID, verifierID string, payment *Payment, allocs []*PaymentAllocation, in VerifyPaymentInput) (*VerifyPaymentResult, error) {
	// Step 1: Lock and cache bill info for all allocated bills.
	// Sort bill IDs deterministically to prevent deadlocks when multiple
	// concurrent transactions try to lock overlapping sets of bills.
	type billInfo struct {
		bill          *Bill
		remaining     int64
		billCents     int64
		approvedCents int64
	}

	// Extract and sort bill IDs for consistent locking order.
	billIDs := make([]string, 0, len(allocs))
	allocByBill := make(map[string]*PaymentAllocation, len(allocs))
	for _, a := range allocs {
		billIDs = append(billIDs, a.BillID)
		allocByBill[a.BillID] = a
	}
	sort.Strings(billIDs)

	infos := make(map[string]*billInfo)
	for _, billID := range billIDs {
		a := allocByBill[billID]
		bill, err := BillGetByIDForUpdate(ctx, tx, billID, rtID)
		if err != nil {
			return nil, fmt.Errorf("bill %s not found: %w", billID, err)
		}
		if bill.Status == BillStatusPaid {
			return nil, fmt.Errorf("bill %s is already paid", billID)
		}
		if bill.Status == BillStatusCancelled {
			return nil, fmt.Errorf("bill %s is cancelled", billID)
		}

		billCents, _, err := ParseMoney(bill.Amount)
		if err != nil {
			return nil, fmt.Errorf("parse bill %s amount: %w", billID, err)
		}
		approvedTotal, err := GetApprovedPaymentTotal(ctx, tx, billID)
		if err != nil {
			return nil, fmt.Errorf("approved total for bill %s: %w", billID, err)
		}
		approvedCents, _, err := ParseMoney(approvedTotal)
		if err != nil {
			return nil, fmt.Errorf("parse approved total for bill %s: %w", billID, err)
		}

		allocCents, _, err := ParseMoney(a.Amount)
		if err != nil {
			return nil, fmt.Errorf("parse allocation amount for bill %s: %w", billID, err)
		}

		// Overpayment guard: allocated amount must not exceed remaining.
		remaining := billCents - approvedCents
		if allocCents > remaining {
			return nil, fmt.Errorf("allocation %s exceeds remaining balance %s for bill %s",
				a.Amount, FormatMoney(remaining), billID)
		}

		infos[billID] = &billInfo{
			bill:          bill,
			remaining:     remaining,
			billCents:     billCents,
			approvedCents: approvedCents,
		}
	}

	// Step 2: Approve the payment row.
	officerNotes := ""
	if in.OfficerNotes != nil && strings.TrimSpace(*in.OfficerNotes) != "" {
		officerNotes = strings.TrimSpace(*in.OfficerNotes)
	}
	verifiedPayment, err := PaymentVerify(ctx, tx, paymentID, rtID, verifierID, PaymentStatusApproved, "", officerNotes)
	if err != nil {
		return nil, fmt.Errorf("approve payment: %w", err)
	}

	// Step 3: Update all bill statuses + create ONE KAS transaction.
	// Iterate allocs in original order to preserve allocation mapping.
	for _, a := range allocs {
		info := infos[a.BillID]
		allocCents, _, _ := ParseMoney(a.Amount)

		newTotal := info.approvedCents + allocCents
		if newTotal >= info.billCents {
			if err := BillUpdateStatus(ctx, tx, a.BillID, rtID, BillStatusPaid); err != nil {
				return nil, fmt.Errorf("update bill %s status: %w", a.BillID, err)
			}
		} else {
			if err := BillUpdateStatus(ctx, tx, a.BillID, rtID, BillStatusPartial); err != nil {
				return nil, fmt.Errorf("update bill %s status: %w", a.BillID, err)
			}
		}
	}

	// Post ONE income transaction into ledger for the entire payment.
	catID, err := FindOrCreateDefaultIncomeCategory(ctx, tx, rtID, "Iuran Warga")
	if err != nil {
		return nil, err
	}
	periods := make([]string, 0, len(allocs))
	for _, a := range allocs {
		if info := infos[a.BillID]; info != nil {
			periods = append(periods, info.bill.Period)
		}
	}
	desc := fmt.Sprintf("Multi-period payment: %s", strings.Join(periods, ", "))
	now := time.Now()
	_, err = TransactionCreate(ctx, tx, rtID, CreateTransactionInput{
		CategoryID:  catID,
		Amount:      payment.Amount,
		Type:        TransactionTypeIncome,
		Description: desc,
	}, TransactionStatusPosted, nil, &payment.ID, now)
	if err != nil {
		return nil, fmt.Errorf("post ledger transaction for multi-period payment: %w", err)
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

	return &VerifyPaymentResult{Payment: verifiedPayment}, nil
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

// CancelBill cancels a bill only if it has no approved payments.
// Bills with approved payments (partial or paid status) cannot be cancelled.
func (s *Service) CancelBill(ctx context.Context, tx *sql.Tx, id, rtID, userID string) error {
	// Lock the bill row
	bill, err := BillGetByIDForUpdate(ctx, tx, id, rtID)
	if err != nil {
		return err
	}

	// Only unpaid bills without approved payments can be cancelled
	if bill.Status != BillStatusUnpaid {
		return ErrBillCannotCancel
	}

	// Check for approved payments
	approvedCount, err := GetApprovedPaymentCount(ctx, tx, bill.ID)
	if err != nil {
		return fmt.Errorf("check approved payments: %w", err)
	}
	if approvedCount > 0 {
		return ErrBillCannotCancel
	}

	if err := BillUpdateStatus(ctx, tx, bill.ID, rtID, BillStatusCancelled); err != nil {
		return fmt.Errorf("cancel bill: %w", err)
	}

	_ = AuditLogInsert(ctx, tx, AuditLog{
		RTID:       rtID,
		UserID:     &userID,
		Action:     "cancel_bill",
		EntityType: "bill",
		EntityID:   bill.ID,
	})

	return nil
}

// CreateMultiPeriodPayment creates a single payment that spans multiple bills (multi-period).
// One payment row, one KAS transaction, multiple allocations.
func (s *Service) CreateMultiPeriodPayment(ctx context.Context, tx *sql.Tx, rtID, userID, userRole string, in CreateMultiPeriodPaymentInput) (*MultiPeriodPaymentResult, error) {
	// ── 1. Validate input ──────────────────────────────────────────────
	if len(in.BillIDs) == 0 {
		return nil, errors.New("at least one bill must be selected")
	}
	if len(in.Allocations) == 0 {
		return nil, errors.New("at least one allocation must be specified")
	}
	if len(in.Allocations) != len(in.BillIDs) {
		return nil, errors.New("allocations count must match bill IDs count")
	}

	// Build allocation lookup
	allocMap := make(map[string]string) // billID → amount
	for _, a := range in.Allocations {
		if strings.TrimSpace(a.BillID) == "" {
			return nil, errors.New("allocation bill_id is required")
		}
		if strings.TrimSpace(a.Amount) == "" {
			return nil, fmt.Errorf("allocation amount for bill %s is required", a.BillID)
		}
		allocMap[a.BillID] = a.Amount
	}

	// Compute total allocation
	totalCents := int64(0)
	for _, amountStr := range allocMap {
		cents, _, err := ParseMoney(amountStr)
		if err != nil {
			return nil, fmt.Errorf("invalid allocation amount: %w", err)
		}
		totalCents += cents
	}
	totalCanon := FormatMoney(totalCents)

	// ── 2. Lock and validate all bills ─────────────────────────────────
	// Sort bill IDs deterministically to prevent deadlocks when multiple
	// concurrent transactions try to lock overlapping sets of bills.
	sortedBillIDs := make([]string, len(in.BillIDs))
	copy(sortedBillIDs, in.BillIDs)
	sort.Strings(sortedBillIDs)

	type billInfo struct {
		bill          *Bill
		due           *Due
		remaining     int64
		billCents     int64
		approvedCents int64
		pendingCents  int64 // pending reservation (warga only)
	}

	infos := make(map[string]*billInfo)
	for _, billID := range sortedBillIDs {
		bill, err := BillGetByIDForUpdate(ctx, tx, billID, rtID)
		if err != nil {
			return nil, fmt.Errorf("bill %s not found in this RT: %w", billID, err)
		}
		if bill.Status == BillStatusPaid {
			return nil, fmt.Errorf("bill %s is already paid", billID)
		}
		if bill.Status == BillStatusCancelled {
			return nil, fmt.Errorf("bill %s is cancelled", billID)
		}

		due, err := DueGetByID(ctx, tx, bill.DueID, rtID)
		if err != nil {
			return nil, fmt.Errorf("due not found for bill %s: %w", billID, err)
		}

		// Compute remaining for this bill.
		// For warga, pending reservations reduce available capacity so that
		// duplicate submissions cannot overpay a bill through PENDING entries.
		// Staff auto-approve skips this check because staff payments are
		// immediately APPROVED and validated against approved totals.
		billCents, _, err := ParseMoney(bill.Amount)
		if err != nil {
			return nil, fmt.Errorf("parse bill amount: %w", err)
		}
		approvedTotal, err := GetApprovedPaymentTotal(ctx, tx, billID)
		if err != nil {
			return nil, fmt.Errorf("compute approved total: %w", err)
		}
		approvedCents, _, err := ParseMoney(approvedTotal)
		if err != nil {
			return nil, fmt.Errorf("parse approved total: %w", err)
		}

		// Pending reservation for warga — prevents double-submission overpayment.
		// Matches single-period policy: warga cannot submit more than
		// (bill amount - approved total - pending total).
		var pendingCents int64
		if userRole == "warga" {
			pendingTotal, err := GetPendingPaymentTotal(ctx, tx, billID)
			if err != nil {
				return nil, fmt.Errorf("compute pending total: %w", err)
			}
			pendingCents, _, err = ParseMoney(pendingTotal)
			if err != nil {
				return nil, fmt.Errorf("parse pending total: %w", err)
			}
		}

		remaining := billCents - approvedCents - pendingCents
		infos[billID] = &billInfo{
			bill:          bill,
			due:           due,
			remaining:     remaining,
			billCents:     billCents,
			approvedCents: approvedCents,
			pendingCents:  pendingCents,
		}
	}

	// ── 3. Warga ownership check ───────────────────────────────────────
	if userRole == "warga" {
		occID, err := GetOccupancyIDForUser(ctx, tx, rtID, userID)
		if err != nil {
			return nil, fmt.Errorf("error verifying warga occupancy: %w", err)
		}
		for billID, info := range infos {
			if info.bill.HouseholdOccupancyID != occID {
				return nil, fmt.Errorf("cannot pay bill belonging to another household (bill %s)", billID)
			}
		}
	}

	// ── 4. Validate allocation amounts ─────────────────────────────────
	// For warga, `remaining` already excludes pending reservations (same
	// policy as single-period `CreatePayment`), preventing double-submission
	// overpayment. For staff, `remaining` equals `billCents - approvedCents`
	// because staff payments are immediately APPROVED and validated
	// against approved totals at verification time.
	for _, a := range in.Allocations {
		info := infos[a.BillID]
		if info == nil {
			return nil, fmt.Errorf("allocation references unknown bill %s", a.BillID)
		}

		allocCents, _, err := ParseMoney(a.Amount)
		if err != nil {
			return nil, fmt.Errorf("parse allocation amount: %w", err)
		}
		if allocCents <= 0 {
			return nil, fmt.Errorf("allocation amount must be positive for bill %s", a.BillID)
		}
		if allocCents > info.remaining {
			return nil, fmt.Errorf("allocation amount %s exceeds remaining balance %s for bill %s",
				a.Amount, FormatMoney(info.remaining), a.BillID)
		}

		// Enforce payment_rule per bill
		if info.due.PaymentRule == PaymentRuleFull {
			if allocCents != info.remaining {
				return nil, fmt.Errorf("bill %s requires full payment: amount must be %s",
					a.BillID, FormatMoney(info.remaining))
			}
		}
	}

	// ── 5. Determine origin/status ─────────────────────────────────────
	var origin PaymentOrigin
	var status PaymentStatus
	var verifiedBy *string
	var verifiedAt *time.Time

	if userRole == "warga" {
		origin = PaymentOriginSelfSubmitted
		status = PaymentStatusPending
	} else {
		origin = PaymentOriginStaffRecorded
		status = PaymentStatusApproved
		verifiedBy = &userID
		now := time.Now()
		verifiedAt = &now
	}

	// ── 6. Create payment row ──────────────────────────────────────────
	// Use direct INSERT (PaymentCreate validates single-bill remaining;
	// multi-period total exceeds any single bill's remaining).
	// All validation already passed in steps 1-5.
	var payment Payment
	var err error
	err = tx.QueryRowContext(ctx,
		`INSERT INTO payments (rt_id, bill_id, due_id, submitted_by, amount, method, origin, status,
		 proof_path, notes, proof_original_name, proof_mime_type, proof_size_bytes,
		 proof_uploaded_by, verified_by, verified_at)
		 VALUES ($1, $2, NULL, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		 RETURNING id, rt_id, bill_id, due_id, submitted_by, amount::text, method, origin, status, proof_path, paid_at,
		          verified_by, verified_at, verified_by_name, rejection_reason, notes, officer_notes,
		          created_at, updated_at, proof_original_name, proof_mime_type, proof_size_bytes,
		          proof_uploaded_at, proof_uploaded_by`,
		rtID, in.Allocations[0].BillID, userID, totalCanon, in.Method, origin, status,
		in.ProofPath, in.Notes,
		in.ProofOriginalName, in.ProofMimeType, in.ProofSizeBytes, in.ProofUploadedBy,
		verifiedBy, verifiedAt,
	).Scan(&payment.ID, &payment.RTID, &payment.BillID, &payment.DueID, &payment.SubmittedBy, &payment.Amount,
		&payment.Method, &payment.Origin, &payment.Status,
		&payment.ProofPath, &payment.PaidAt,
		&payment.VerifiedBy, &payment.VerifiedAt, &payment.VerifiedByName,
		&payment.RejectionReason, &payment.Notes, &payment.OfficerNotes,
		&payment.CreatedAt, &payment.UpdatedAt,
		&payment.ProofOriginalName, &payment.ProofMimeType, &payment.ProofSizeBytes,
		&payment.ProofUploadedAt, &payment.ProofUploadedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("create payment: %w", err)
	}

	// ── 7. Create allocations ──────────────────────────────────────────
	allocations := make([]*PaymentAllocation, 0, len(in.Allocations))
	for _, a := range in.Allocations {
		alloc, err := PaymentAllocationCreate(ctx, tx, payment.ID, a.BillID, a.Amount)
		if err != nil {
			return nil, fmt.Errorf("create allocation for bill %s: %w", a.BillID, err)
		}
		allocations = append(allocations, alloc)
	}

	// ── 8. If APPROVED (staff): update bills + KAS transaction ─────────
	if status == PaymentStatusApproved {
		for _, a := range allocations {
			info := infos[a.BillID]
			allocCents, _, _ := ParseMoney(a.Amount)

			// Use cached values — do NOT re-query GetApprovedPaymentTotal()
			// because the payment row is already INSERTED and its FK points
			// to the first bill, which would double-count the amount for
			// staff-recorded payments.
			newTotal := info.approvedCents + allocCents
			if newTotal >= info.billCents {
				_ = BillUpdateStatus(ctx, tx, a.BillID, rtID, BillStatusPaid)
			} else {
				_ = BillUpdateStatus(ctx, tx, a.BillID, rtID, BillStatusPartial)
			}
		}

		// Post ONE income transaction into ledger for the entire payment
		catID, err := FindOrCreateDefaultIncomeCategory(ctx, tx, rtID, "Iuran Warga")
		if err != nil {
			return nil, err
		}
		periods := make([]string, 0, len(allocations))
		for _, a := range allocations {
			if info := infos[a.BillID]; info != nil {
				periods = append(periods, info.bill.Period)
			}
		}
		desc := fmt.Sprintf("Multi-period payment: %s", strings.Join(periods, ", "))
		now := time.Now()
		_, err = TransactionCreate(ctx, tx, rtID, CreateTransactionInput{
			CategoryID:  catID,
			Amount:      payment.Amount,
			Type:        TransactionTypeIncome,
			Description: desc,
		}, TransactionStatusPosted, nil, &payment.ID, now)
		if err != nil {
			return nil, fmt.Errorf("post ledger transaction for multi-period payment: %w", err)
		}
	}

	// ── 9. Audit log ───────────────────────────────────────────────────
	auditData, _ := json.Marshal(map[string]interface{}{
		"payment_id":       payment.ID,
		"bill_ids":         in.BillIDs,
		"total":            totalCanon,
		"allocation_count": len(allocations),
	})
	_ = AuditLogInsert(ctx, tx, AuditLog{
		RTID:       rtID,
		UserID:     &userID,
		Action:     "create_multi_period_payment",
		EntityType: "payment",
		EntityID:   payment.ID,
		NewValues:  auditData,
	})

	return &MultiPeriodPaymentResult{
		Payment:     &payment,
		Allocations: allocations,
	}, nil
}
