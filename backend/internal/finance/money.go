package finance

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	ErrInvalidMoneyFormat = errors.New("invalid money format: must be a positive decimal number with up to 2 decimal places")
	ErrNegativeMoney      = errors.New("monetary amount must be positive")
	moneyRegex            = regexp.MustCompile(`^[0-9]+(\.[0-9]{1,2})?$`)
)

// ParseMoney parses a numeric string representation of money (e.g. "100000", "50000.5", "25000.00")
// into an exact integer representing cents (Rupiah * 100), and a canonical normalized string ("100000.00").
// Floating point types (float32, float64) are NEVER used.
func ParseMoney(s string) (int64, string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, "", ErrInvalidMoneyFormat
	}
	if !moneyRegex.MatchString(s) {
		return 0, "", ErrInvalidMoneyFormat
	}

	parts := strings.Split(s, ".")
	wholePart, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("invalid whole money amount: %w", err)
	}
	if wholePart < 0 {
		return 0, "", ErrNegativeMoney
	}

	var fractionalPart int64
	if len(parts) == 2 {
		fracStr := parts[1]
		if len(fracStr) == 1 {
			fracStr += "0"
		}
		fractionalPart, err = strconv.ParseInt(fracStr, 10, 64)
		if err != nil {
			return 0, "", fmt.Errorf("invalid fractional money amount: %w", err)
		}
	}

	totalCents := wholePart*100 + fractionalPart
	canonical := fmt.Sprintf("%d.%02d", wholePart, fractionalPart)
	return totalCents, canonical, nil
}

// FormatMoney formats cents (Rupiah * 100) into a canonical "%.2f" string without floating point operations.
func FormatMoney(cents int64) string {
	if cents < 0 {
		abs := -cents
		return fmt.Sprintf("-%d.%02d", abs/100, abs%100)
	}
	return fmt.Sprintf("%d.%02d", cents/100, cents%100)
}
