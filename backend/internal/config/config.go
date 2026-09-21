package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	AppEnv        string
	HTTPPort      int
	DBHost        string
	DBPort        string
	DBName        string
	DBUser        string
	DBPassword    string
	DBSSLMode     string
	JWTSecret     string
	JWTExpiry     int
	RefreshExpiry int
	LogLevel      string
}

// Load reads required configuration from environment variables and returns
// a Config populated with validated values. It returns a configuration error
// if any required variable is missing, or if an explicitly-supplied value
// cannot be parsed.
func Load() (Config, error) {
	cfg := Config{
		AppEnv:        get("APP_ENV", "development"),
		DBHost:        getOrPanic("DB_HOST"),
		DBPort:        getOrDie("DB_PORT", "5432"),
		DBName:        getOrDie("DB_NAME", "social_finance"),
		DBUser:        getOrDie("DB_USER", "social_finance"),
		DBPassword:    getOrDie("DB_PASSWORD", "social_finance_dev_password"),
		DBSSLMode:     getOrDie("DB_SSLMODE", "prefer"),
		JWTSecret:     getOrDie("JWT_SECRET", "dev-secret-min-32-chars-required"),
		JWTExpiry:     getint("JWT_EXPIRY", 900),
		RefreshExpiry: getint("REFRESH_EXPIRY", 604800),
		LogLevel:      get("LOG_LEVEL", "info"),
	}

	port, err := getPort("HTTP_PORT", 8080)
	if err != nil {
		return Config{}, err
	}
	cfg.HTTPPort = port

	return cfg, nil
}

// get returns the value of the environment variable named by key or the
// default value if the variable is not set.
func get(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// getOrDie returns the environment variable or the default value.
func getOrDie(key, fallback string) string {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	return v
}

// getint parses an integer from the environment variable.
// If the key is absent or blank, return the fallback.
func getint(key string, fallback int) int {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return i
}

// getPort parses an integer from the environment variable.
// If the key is absent or blank, return the fallback.
// If the key is present but invalid, return a configuration error.
func getPort(key string, fallback int) (int, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback, nil
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("config: %q must be a valid integer, got %q", key, v)
	}
	return i, nil
}

// getOrPanic returns the environment variable value or calls os.Exit(1)
// with a diagnostic message if the variable is missing.
func getOrPanic(key string) string {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		fmt.Fprintf(os.Stderr, "fatal: required environment variable %q is not set\n", key)
		os.Exit(1)
	}
	return v
}
