package config_test

import (
	"os"
	"testing"

	"social-finance/internal/config"
)

func TestLoadUsesDefaultsWhenEnvIsMissing(t *testing.T) {
	os.Setenv("DB_HOST", "postgres")
	defer os.Unsetenv("DB_HOST")
	os.Unsetenv("HTTP_PORT")
	os.Unsetenv("DB_PORT")
	os.Unsetenv("DB_NAME")
	os.Unsetenv("DB_USER")
	os.Unsetenv("DB_PASSWORD")
	os.Unsetenv("DB_SSLMODE")
	os.Unsetenv("APP_ENV")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.AppEnv != "development" {
		t.Errorf("AppEnv = %q; want development", cfg.AppEnv)
	}
	if cfg.HTTPPort != 8080 {
		t.Errorf("HTTPPort = %d; want 8080", cfg.HTTPPort)
	}
	if cfg.DBPort != "5432" {
		t.Errorf("DBPort = %q; want 5432", cfg.DBPort)
	}
	if cfg.DBName != "social_finance" {
		t.Errorf("DBName = %q; want social_finance", cfg.DBName)
	}
	if cfg.DBSSLMode != "prefer" {
		t.Errorf("DBSSLMode = %q; want prefer", cfg.DBSSLMode)
	}
}

func TestLoadReadsEnvValues(t *testing.T) {
	setenv(t, "DB_HOST", "postgres")
	setenv(t, "HTTP_PORT", "9090")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DBHost != "postgres" {
		t.Errorf("DBHost = %q; want postgres", cfg.DBHost)
	}
	if cfg.HTTPPort != 9090 {
		t.Errorf("HTTPPort = %d; want 9090", cfg.HTTPPort)
	}
}

func TestLoadSetsAppEnv(t *testing.T) {
	setenv(t, "DB_HOST", "postgres")
	setenv(t, "APP_ENV", "production")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.AppEnv != "production" {
		t.Errorf("AppEnv = %q; want production", cfg.AppEnv)
	}
}

func TestLoadInvalidPortReturnsError(t *testing.T) {
	setenv(t, "DB_HOST", "postgres")
	setenv(t, "HTTP_PORT", "not-a-number")

	_, err := config.Load()

	if err == nil {
		t.Fatal("expected error for invalid HTTP_PORT; got nil")
	}
}

func TestLoadValidPortOverridden(t *testing.T) {
	setenv(t, "DB_HOST", "postgres")
	setenv(t, "HTTP_PORT", "4444")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HTTPPort != 4444 {
		t.Errorf("HTTPPort = %d; want 4444", cfg.HTTPPort)
	}
}

func setenv(t *testing.T, key, val string) {
	t.Helper()
	if err := os.Setenv(key, val); err != nil {
		t.Fatalf("failed to set %s: %v", key, err)
	}
}
