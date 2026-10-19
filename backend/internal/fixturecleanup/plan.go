package fixturecleanup

import (
	"fmt"
	"strings"
)

// CleanupPlan describes the exact, ownership-scoped cleanup targets for E2E fixtures.
// It contains no patterns or wildcards.
type CleanupPlan struct {
	UserEmails []string
	RTNames    []string
	Tables     []string
}

// DefaultFixtureCleanupPlan returns the explicit fixture-owned targets.
func DefaultFixtureCleanupPlan() CleanupPlan {
	return CleanupPlan{
		UserEmails: []string{
			"e2e.user-a@social-finance-test.internal",
			"e2e.pengurus-a@social-finance-test.internal",
			"e2e.user-b@social-finance-test.internal",
			"e2e.user-c@social-finance-test.internal",
			"e2e.super-admin@social-finance-test.internal",
		},
		RTNames: []string{
			"E2E Test RT A",
			"E2E Test RT B",
			"E2E Test RT C",
		},
		Tables: []string{
			"user_rt_memberships",
			"users",
			"rts",
		},
	}
}

// Validate ensures the plan is explicit and non-empty.
func (p CleanupPlan) Validate() error {
	if len(p.UserEmails) == 0 || len(p.RTNames) == 0 {
		return fmt.Errorf("cleanup plan must not be empty")
	}
	for _, v := range append(append([]string{}, p.UserEmails...), p.RTNames...) {
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("cleanup plan contains empty target")
		}
	}
	if len(p.Tables) == 0 {
		return fmt.Errorf("cleanup plan must declare ordered tables")
	}
	return nil
}
