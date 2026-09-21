package finance

import (
	"testing"
)

func TestParseMoney(t *testing.T) {
	tests := []struct {
		input       string
		wantCents   int64
		wantCanon   string
		expectError bool
	}{
		{"100000", 10000000, "100000.00", false},
		{"50000.5", 5000050, "50000.50", false},
		{"25000.00", 2500000, "25000.00", false},
		{"0.05", 5, "0.05", false},
		{"0", 0, "0.00", false},
		{"-100", 0, "", true},
		{"abc", 0, "", true},
		{"12.345", 0, "", true},
		{"", 0, "", true},
	}

	for _, tc := range tests {
		cents, canon, err := ParseMoney(tc.input)
		if tc.expectError {
			if err == nil {
				t.Errorf("ParseMoney(%q) expected error, got nil", tc.input)
			}
		} else {
			if err != nil {
				t.Errorf("ParseMoney(%q) unexpected error: %v", tc.input, err)
			}
			if cents != tc.wantCents {
				t.Errorf("ParseMoney(%q) cents = %d, want %d", tc.input, cents, tc.wantCents)
			}
			if canon != tc.wantCanon {
				t.Errorf("ParseMoney(%q) canon = %s, want %s", tc.input, canon, tc.wantCanon)
			}
		}
	}
}

func TestFormatMoney(t *testing.T) {
	if got := FormatMoney(10000000); got != "100000.00" {
		t.Errorf("FormatMoney(10000000) = %s, want 100000.00", got)
	}
	if got := FormatMoney(5); got != "0.05" {
		t.Errorf("FormatMoney(5) = %s, want 0.05", got)
	}
	if got := FormatMoney(-500000); got != "-5000.00" {
		t.Errorf("FormatMoney(-500000) = %s, want -5000.00", got)
	}
}
