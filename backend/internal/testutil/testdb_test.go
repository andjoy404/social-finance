package testutil

import (
	"fmt"
	"os"
	"testing"
)

type mockTB struct {
	failed bool
	msg    string
}

func (m *mockTB) Helper() {}

func (m *mockTB) Fatalf(format string, args ...any) {
	m.failed = true
	m.msg = fmt.Sprintf(format, args...)
}

func TestEnsureTestDBName_RejectsDevelopmentDB(t *testing.T) {
	mock := &mockTB{}
	EnsureTestDBName(mock, "social_finance")
	if !mock.failed {
		t.Fatalf("expected EnsureTestDBName to fail for 'social_finance'")
	}
}

func TestEnsureTestDBName_RejectsNonTestDB(t *testing.T) {
	mock := &mockTB{}
	EnsureTestDBName(mock, "production_db")
	if !mock.failed {
		t.Fatalf("expected EnsureTestDBName to fail for 'production_db'")
	}
}

func TestEnsureTestDBName_AcceptsTestDB(t *testing.T) {
	mock := &mockTB{}
	EnsureTestDBName(mock, "social_finance_test")
	if mock.failed {
		t.Fatalf("expected EnsureTestDBName to pass for 'social_finance_test'")
	}

	EnsureTestDBName(mock, "custom_test_db")
	if mock.failed {
		t.Fatalf("expected EnsureTestDBName to pass for 'custom_test_db'")
	}
}

func TestGetTestDBName_Default(t *testing.T) {
	os.Unsetenv("TEST_DB_NAME")
	os.Unsetenv("DB_NAME")
	name := GetTestDBName(t)
	if name != DefaultTestDBName {
		t.Errorf("GetTestDBName = %q, want %q", name, DefaultTestDBName)
	}
}

func TestGetTestDBName_IgnoresSocialFinanceEnv(t *testing.T) {
	os.Unsetenv("TEST_DB_NAME")
	os.Setenv("DB_NAME", "social_finance")
	defer os.Unsetenv("DB_NAME")

	name := GetTestDBName(t)
	if name != DefaultTestDBName {
		t.Errorf("GetTestDBName = %q, want %q", name, DefaultTestDBName)
	}
}

