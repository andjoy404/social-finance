package http

import "testing"

func TestFormatNumericCode(t *testing.T) {
	tests := []struct {
		name   string
		input  interface{}
		expect string
	}{
		// Valid integers
		{"int 3", 3, "003"},
		{"int 16", 16, "016"},
		{"int 100", 100, "100"},
		{"int 0", 0, "000"},
		{"int64 3", int64(3), "003"},
		{"int32 3", int32(3), "003"},
		{"float64 3", float64(3), "003"},

		// Valid strings
		{"string '3'", "3", "003"},
		{"string '03'", "03", "003"},
		{"string '003'", "003", "003"},
		{"string '16'", "16", "016"},
		{"string '100'", "100", "100"},
		{"string ''", "", ""},
		{"string ' 3 '", " 3 ", "003"},

		// Invalid/empty — return "" or preserve original
		{"nil", nil, ""},
		{"string 'abc'", "abc", "abc"},
		{"string 'rt-1'", "rt-1", "rt-1"},
		{"string UUID", "550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-446655440000"},
		{"negative int", -1, "000"},
		{"string '-3'", "-3", "000"},
		{"empty string", "", ""},
		{"whitespace", "   ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatNumericCode(tt.input)
			if got != tt.expect {
				t.Errorf("FormatNumericCode(%v) = %q, want %q", tt.input, got, tt.expect)
			}
		})
	}
}
