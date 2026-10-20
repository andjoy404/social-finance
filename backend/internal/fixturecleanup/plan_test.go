package fixturecleanup

import "testing"

func TestDefaultPlanIsScoped(t *testing.T) {
	p := DefaultFixtureCleanupPlan()
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, email := range p.UserEmails {
		if email == "admin@local.com" {
			t.Fatal("must not target admin account")
		}
	}
	for _, name := range p.RTNames {
		if name == "E2E%" {
			t.Fatal("must not use wildcard cleanup")
		}
	}
}

func TestEmptyPlanRejected(t *testing.T) {
	p := CleanupPlan{}
	if err := p.Validate(); err == nil {
		t.Fatal("empty plan must be rejected")
	}
}

func TestPlanTablesOrdered(t *testing.T) {
	p := DefaultFixtureCleanupPlan()
	if p.Tables[0] != "user_rt_memberships" || p.Tables[1] != "users" || p.Tables[2] != "rts" {
		t.Fatalf("unexpected cleanup order: %v", p.Tables)
	}
}
