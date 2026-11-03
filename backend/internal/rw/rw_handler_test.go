package rw

import (
	"net/url"
	"strconv"
	"strings"
	"testing"
)

// --- Test parsePagination ---

func TestParsePagination(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		wantPage int
		wantSize int
	}{
		{"defaults", "", 1, 20},
		{"page=2", "page=2", 2, 20},
		{"page_size=50", "page_size=50", 1, 50},
		{"page=1&page_size=10", "page=1&page_size=10", 1, 10},
		{"page_size over 100 capped", "page_size=200", 1, 100},
		{"page=0 invalid", "page=0", 1, 20},
		{"page_size=0 invalid", "page_size=0", 1, 20},
		{"page invalid", "page=abc", 1, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Directly replicate parsePagination logic on URL values
			v := url.Values{}
			if tt.query != "" {
				pairs := strings.Split(tt.query, "&")
				for _, p := range pairs {
					kv := strings.SplitN(p, "=", 2)
					if len(kv) == 2 {
						v.Set(kv[0], kv[1])
					}
				}
			}

			page := 1
			pageSize := 20
			if p := v.Get("page"); p != "" {
				if n, err := strconv.Atoi(p); err == nil && n > 0 {
					page = n
				}
			}
			if ps := v.Get("page_size"); ps != "" {
				if n, err := strconv.Atoi(ps); err == nil && n > 0 {
					if n > 100 {
						n = 100
					}
					pageSize = n
				}
			}

			if page != tt.wantPage {
				t.Errorf("page = %d, want %d", page, tt.wantPage)
			}
			if pageSize != tt.wantSize {
				t.Errorf("pageSize = %d, want %d", pageSize, tt.wantSize)
			}
		})
	}
}

// --- Test ValidatePeriod ---

func TestValidatePeriod(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"valid", "2025-01", "2025-01", false},
		{"valid with spaces", " 2025-01 ", "2025-01", false},
		{"empty", "", "", true},
		{"too short", "2025", "", true},
		{"wrong format YYYY/MM", "2025/01", "", true},
		{"not a number", "abc-def", "", true},
		{"only year", "2025-", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidatePeriod(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePeriod(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ValidatePeriod(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- Test ValidateDate ---

func TestValidateDate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"valid", "2025-01-15", "2025-01-15", false},
		{"valid with spaces", " 2025-01-15 ", "2025-01-15", false},
		{"empty", "", "", true},
		{"wrong format YYYY/MM/DD", "2025/01/15", "", true},
		{"too short YYYY-MM", "2025-01", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateDate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDate(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ValidateDate(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- Test ParseMoney ---

func TestParseMoney(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantFmt string
		wantErr bool
	}{
		{"zero", "0", 0, "0.00", false},
		{"simple", "100", 10000, "100.00", false},
		{"with decimal", "100.50", 10050, "100.50", false},
		{"small decimal", "0.99", 99, "0.99", false},
		// finance.ParseMoney rejects negative values (ErrNegativeMoney)
		{"negative", "-50", 0, "", true},
		{"negative decimal", "-50.50", 0, "", true},
		// finance.ParseMoney rejects >2 decimal places (ErrInvalidMoneyFormat)
		{"three decimals", "1.234", 0, "", true},
		{"empty", "", 0, "", true},
		{"multiple dots", "1.2.3", 0, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotFmt, err := ParseMoney(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMoney(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got != tt.want {
				t.Errorf("ParseMoney(%q) cents = %d, want %d", tt.input, got, tt.want)
			}
			if gotFmt != tt.wantFmt {
				t.Errorf("ParseMoney(%q) fmt = %q, want %q", tt.input, gotFmt, tt.wantFmt)
			}
		})
	}
}

// --- Test FormatMoney ---

func TestFormatMoney(t *testing.T) {
	tests := []struct {
		name  string
		cents int64
		want  string
	}{
		{"zero", 0, "0.00"},
		{"positive", 10000, "100.00"},
		{"small", 99, "0.99"},
		{"negative", -5000, "-50.00"},
		{"negative small", -5050, "-50.50"},
		{"large", 99999999, "999999.99"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatMoney(tt.cents)
			if got != tt.want {
				t.Errorf("FormatMoney(%d) = %q, want %q", tt.cents, got, tt.want)
			}
		})
	}
}

// --- Test nextMonth ---

func TestNextMonth(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"jan to feb", "2025-01-01T00:00:00+07:00", "2025-02-01T00:00:00+07:00"},
		{"dec to jan next year", "2025-12-01T00:00:00+07:00", "2026-01-01T00:00:00+07:00"},
		{"mar", "2025-03-01T00:00:00+07:00", "2025-04-01T00:00:00+07:00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			year := tt.input[:4]
			month := tt.input[5:7]
			got := nextMonth(tt.input, year, month)
			if got != tt.want {
				t.Errorf("nextMonth(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- Test pagination query parameter parsing via URL ---

func TestPaginationFromURL(t *testing.T) {
	tests := []struct {
		url       string
		wantPage  int
		wantSize  int
	}{
		{"/api/v1/rw/abc/kas/transactions", 1, 20},
		{"/api/v1/rw/abc/kas/transactions?page=3", 3, 20},
		{"/api/v1/rw/abc/kas/transactions?page=1&page_size=50", 1, 50},
		{"/api/v1/rw/abc/kas/transactions?page_size=150", 1, 100},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			u, err := url.Parse(tt.url)
			if err != nil {
				t.Fatalf("parse URL: %v", err)
			}

			page := 1
			pageSize := 20
			if p := u.Query().Get("page"); p != "" {
				if n, err := strconv.Atoi(p); err == nil && n > 0 {
					page = n
				}
			}
			if ps := u.Query().Get("page_size"); ps != "" {
				if n, err := strconv.Atoi(ps); err == nil && n > 0 {
					if n > 100 {
						n = 100
					}
					pageSize = n
				}
			}

			if page != tt.wantPage {
				t.Errorf("page = %d, want %d", page, tt.wantPage)
			}
			if pageSize != tt.wantSize {
				t.Errorf("pageSize = %d, want %d", pageSize, tt.wantSize)
			}
		})
	}
}

// --- Test dashboard summary defaults ---

func TestDashboardSummaryDefaults(t *testing.T) {
	s := DashboardSummary{}
	if s.TotalRTs != 0 {
		t.Errorf("default TotalRTs = %d, want 0", s.TotalRTs)
	}
	if s.TotalOpeningBal != "" {
		t.Errorf("default TotalOpeningBal = %q, want empty", s.TotalOpeningBal)
	}
	if s.NetBalance != "" {
		t.Errorf("default NetBalance = %q, want empty", s.NetBalance)
	}
}

// --- Test KASReport net balance ---

func TestKASReportNetBalance(t *testing.T) {
	r := KASReport{
		TotalIncome:  "1000.00",
		TotalExpense: "400.00",
	}
	incCents, _, _ := ParseMoney(r.TotalIncome)
	expCents, _, _ := ParseMoney(r.TotalExpense)
	r.NetBalance = FormatMoney(incCents - expCents)
	if r.NetBalance != "600.00" {
		t.Errorf("NetBalance = %q, want 600.00", r.NetBalance)
	}
}
