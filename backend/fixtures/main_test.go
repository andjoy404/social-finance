package main

import "testing"

func TestRequireDisposableTestTarget(t *testing.T) {
	if err := requireDisposableTestTarget("rw8_test_disposable", true); err != nil {
		t.Errorf("opt-in disposable target should be allowed: %v", err)
	}
	for _, name := range []string{"social_finance", "social_finance_test", "production", ""} {
		if err := requireDisposableTestTarget(name, true); err == nil {
			t.Errorf("%q should be rejected", name)
		}
	}
	if err := requireDisposableTestTarget("rw8_test_disposable", false); err == nil {
		t.Errorf("missing opt-in should be rejected")
	}
}
