package rw_test

import (
	"testing"

	"social-finance/internal/finance"
	"social-finance/internal/rw"
)

// TestParseMoneyParityWithFinancePackage ensures rw.ParseMoney
// delegates to finance.ParseMoney without behavioral drift.
func TestParseMoneyParityWithFinancePackage(t *testing.T) {
	tests := []string{
		"100000", "50000.5", "25000.00", "0.05", "0", "0.00",
		"-100", "abc", "12.345", "", "1.234", "1.2",
	}

	for _, tc := range tests {
		rwCents, rwCanon, rwErr := rw.ParseMoney(tc)
		finCents, finCanon, finErr := finance.ParseMoney(tc)

		if (rwErr != nil) != (finErr != nil) {
			t.Errorf("ParseMoney(%q): error mismatch — rwErr=%v, finErr=%v", tc, rwErr, finErr)
		}
		if rwErr == nil && finErr == nil {
			if rwCents != finCents {
				t.Errorf("ParseMoney(%q): cents mismatch — rw=%d, finance=%d", tc, rwCents, finCents)
			}
			if rwCanon != finCanon {
				t.Errorf("ParseMoney(%q): canon mismatch — rw=%q, finance=%q", tc, rwCanon, finCanon)
			}
		}
	}
}

// TestFormatMoneyParityWithFinancePackage ensures rw.FormatMoney
// delegates to finance.FormatMoney without behavioral drift.
func TestFormatMoneyParityWithFinancePackage(t *testing.T) {
	for _, c := range []int64{0, 5, 10000000, -500000, 99999999999, -99999999999} {
		rwFmt := rw.FormatMoney(c)
		finFmt := finance.FormatMoney(c)
		if rwFmt != finFmt {
			t.Errorf("FormatMoney(%d): rw=%q, finance=%q", c, rwFmt, finFmt)
		}
	}
}
