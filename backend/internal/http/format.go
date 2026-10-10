package http

import (
	"fmt"
	"strconv"
	"strings"
)

// FormatNumericCode normalizes a numeric RT/RW code value to a 3-digit zero-padded string.
//
// Accepted inputs and results:
//
//	3    → "003"
//	"03" → "003"
//	16   → "016"
//	100  → "100"
//	""   → "" (empty — no fake code)
//	nil  → "" (empty — no fake code)
//	"abc","rt-1",UUID → "abc","rt-1",UUID (preserved as-is)
//	"C01" → "C01" (alphanumeric preserved)
//	-1 → "000" (negative rejected)
//
// IMPORTANT: Only pass actual RT/RW codes to this function. Do NOT pass UUIDs,
// NIKs, phone numbers, monetary amounts, or any other non-wilayah identifiers.
func FormatNumericCode(val interface{}) string {
	switch v := val.(type) {
	case int:
		if v < 0 {
			return "000"
		}
		return formatInt(int64(v))
	case int64:
		if v < 0 {
			return "000"
		}
		return formatInt(v)
	case int32:
		if v < 0 {
			return "000"
		}
		return formatInt(int64(v))
	case float64:
		if v < 0 {
			return "000"
		}
		return formatInt(int64(v))
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return ""
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			// Non-numeric string: preserve as-is (e.g., "C01")
			return v
		}
		if n < 0 {
			return "000"
		}
		return formatInt(int64(n))
	case nil:
		return ""
	default:
		return ""
	}
}

func formatInt(n int64) string {
	return fmt.Sprintf("%03d", n)
}
