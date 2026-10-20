package fixtureguard

import (
	"fmt"
	"strings"
)

// FixtureTargetConfig describes the expected identity of a disposable fixture database.
// Values must be provided explicitly; no persistent database may be used as a default.
type FixtureTargetConfig struct {
	AllowFixture   bool
	ExpectedDBName string
	ExpectedDBUser string
	ExpectedMarker string
}

// ObservedDBIdentity is the identity and marker value read from an actual database connection.
// It is intended to be populated by a small, explicit database probe, not by fixture startup.
type ObservedDBIdentity struct {
	Database      string
	User          string
	MarkerPresent bool
	MarkerValue   string
}

// ValidateFixtureTarget is a pure, database-free validation of fixture target safety.
// It fails closed unless every identity and marker value matches explicitly.
func ValidateFixtureTarget(cfg FixtureTargetConfig, observed ObservedDBIdentity) error {
	if !cfg.AllowFixture {
		return fmt.Errorf("fixture execution requires explicit opt-in")
	}

	expectedDB := strings.ToLower(strings.TrimSpace(cfg.ExpectedDBName))
	if expectedDB == "" {
		return fmt.Errorf("expected database name is required")
	}
	if expectedDB == "social_finance" || expectedDB == "social_finance_test" {
		return fmt.Errorf("persistent database %q is not a valid fixture target", expectedDB)
	}
	if !strings.Contains(expectedDB, "test") &&
		!strings.Contains(expectedDB, "fixture") &&
		!strings.Contains(expectedDB, "disposable") {
		return fmt.Errorf("expected database name %q does not identify an isolated fixture database", expectedDB)
	}

	expectedUser := strings.TrimSpace(cfg.ExpectedDBUser)
	if expectedUser == "" {
		return fmt.Errorf("expected database user is required")
	}

	expectedMarker := strings.TrimSpace(cfg.ExpectedMarker)
	if expectedMarker == "" {
		return fmt.Errorf("expected disposable marker is required")
	}

	actualDB := strings.ToLower(strings.TrimSpace(observed.Database))
	if actualDB != expectedDB {
		return fmt.Errorf("database mismatch: expected %q, got %q", expectedDB, actualDB)
	}

	actualUser := strings.TrimSpace(observed.User)
	if actualUser != expectedUser {
		return fmt.Errorf("database user mismatch: expected %q, got %q", expectedUser, actualUser)
	}

	if !observed.MarkerPresent {
		return fmt.Errorf("disposable marker is missing")
	}
	if strings.TrimSpace(observed.MarkerValue) != expectedMarker {
		return fmt.Errorf("disposable marker mismatch: expected %q, got %q", expectedMarker, observed.MarkerValue)
	}

	return nil
}
