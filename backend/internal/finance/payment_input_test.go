package finance

import "testing"

func TestCreatePaymentInputValidateRequiresExactlyOneTarget(t *testing.T) {
	billID := "bill-id"
	dueID := "due-id"
	tests := []struct {
		name   string
		billID *string
		dueID  *string
		valid  bool
	}{
		{name: "bill only", billID: &billID, valid: true},
		{name: "due only", dueID: &dueID, valid: true},
		{name: "both targets", billID: &billID, dueID: &dueID},
		{name: "neither target"},
		{name: "empty bill target", billID: new(string)},
		{name: "empty due target", dueID: new(string)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := CreatePaymentInput{
				BillID: tt.billID,
				DueID:  tt.dueID,
				Amount: "1000",
				Method: PaymentMethodTransfer,
			}
			err := in.Validate()
			if tt.valid && err != nil {
				t.Fatalf("expected valid target set, got %v", err)
			}
			if !tt.valid && err == nil {
				t.Fatal("expected invalid target set")
			}
		})
	}
}

func TestCreatePaymentInputValidatePositiveAmountAndMethod(t *testing.T) {
	dueID := "due-id"
	tests := []struct {
		name   string
		amount string
		method PaymentMethod
		want   error
	}{
		{name: "zero", amount: "0", method: PaymentMethodCash, want: ErrInvalidAmount},
		{name: "negative", amount: "-1", method: PaymentMethodTransfer, want: ErrInvalidAmount},
		{name: "positive", amount: "12500.50", method: PaymentMethodTransfer},
		{name: "invalid method", amount: "1", method: "QRIS"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := CreatePaymentInput{DueID: &dueID, Amount: tt.amount, Method: tt.method}
			err := in.Validate()
			if tt.want != nil && err != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, err)
			}
			if tt.name == "positive" {
				if err != nil {
					t.Fatalf("expected valid donation, got %v", err)
				}
				if in.Amount != "12500.50" {
					t.Fatalf("expected canonical decimal amount, got %q", in.Amount)
				}
			}
			if tt.name == "invalid method" && err == nil {
				t.Fatal("expected invalid payment method error")
			}
		})
	}
}
