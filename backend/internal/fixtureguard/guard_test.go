package fixtureguard

import "testing"

func baseConfig() FixtureTargetConfig {
	return FixtureTargetConfig{
		AllowFixture:   true,
		ExpectedDBName: "social_finance_fixture",
		ExpectedDBUser: "social_finance_fixture_user",
		ExpectedMarker: "DISPOSABLE_FIXTURE_DB",
	}
}

func baseObserved() ObservedDBIdentity {
	return ObservedDBIdentity{
		Database:      "social_finance_fixture",
		User:          "social_finance_fixture_user",
		MarkerPresent: true,
		MarkerValue:   "DISPOSABLE_FIXTURE_DB",
	}
}

func TestValidateFixtureTarget(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*FixtureTargetConfig, *ObservedDBIdentity)
		wantErr bool
	}{
		{"opt-in missing", func(c *FixtureTargetConfig, o *ObservedDBIdentity) { c.AllowFixture = false }, true},
		{"expected db empty", func(c *FixtureTargetConfig, o *ObservedDBIdentity) { c.ExpectedDBName = "" }, true},
		{"persistent db social_finance", func(c *FixtureTargetConfig, o *ObservedDBIdentity) {
			c.ExpectedDBName = "social_finance"
			o.Database = "social_finance"
		}, true},
		{"persistent db social_finance_test", func(c *FixtureTargetConfig, o *ObservedDBIdentity) {
			c.ExpectedDBName = "social_finance_test"
			o.Database = "social_finance_test"
		}, true},
		{"actual db mismatch", func(c *FixtureTargetConfig, o *ObservedDBIdentity) { o.Database = "other_db" }, true},
		{"expected user empty", func(c *FixtureTargetConfig, o *ObservedDBIdentity) { c.ExpectedDBUser = "" }, true},
		{"actual user mismatch", func(c *FixtureTargetConfig, o *ObservedDBIdentity) { o.User = "postgres" }, true},
		{"marker missing", func(c *FixtureTargetConfig, o *ObservedDBIdentity) { o.MarkerPresent = false }, true},
		{"marker mismatch", func(c *FixtureTargetConfig, o *ObservedDBIdentity) { o.MarkerValue = "WRONG" }, true},
		{"expected marker empty", func(c *FixtureTargetConfig, o *ObservedDBIdentity) { c.ExpectedMarker = "" }, true},
		{"all valid", func(c *FixtureTargetConfig, o *ObservedDBIdentity) {}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := baseConfig()
			obs := baseObserved()
			tc.mutate(&cfg, &obs)
			err := ValidateFixtureTarget(cfg, obs)
			if (err != nil) != tc.wantErr {
				t.Fatalf("got err=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}
