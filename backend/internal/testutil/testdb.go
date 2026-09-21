package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"social-finance/internal/config"
	"social-finance/internal/database"
)

const (
	// DefaultTestDBName is the dedicated test database name.
	DefaultTestDBName = "social_finance_test"

	// ForbiddenDBName is the production / real development database that automated tests must never touch.
	ForbiddenDBName = "social_finance"
)

// TB is a minimal interface matching *testing.T.
type TB interface {
	Helper()
	Fatalf(format string, args ...any)
}

// EnsureTestDBName enforces the hard safety guard:
// tests are strictly forbidden from targeting the development database 'social_finance'.
// The target DB name must not be 'social_finance' and must contain 'test'.
func EnsureTestDBName(t TB, dbName string) {
	t.Helper()
	trimmed := strings.TrimSpace(strings.ToLower(dbName))
	if trimmed == ForbiddenDBName || !strings.Contains(trimmed, "test") {
		t.Fatalf("HARD SAFETY GUARD BLOCKED: automated tests cannot run against development database %q. Target database must be an isolated test database (e.g., %s).", dbName, DefaultTestDBName)
	}
}

// GetTestDBName returns the validated test database name, defaulting to social_finance_test.
// If DB_NAME or TEST_DB_NAME is set to social_finance, it rejects it immediately.
func GetTestDBName(t *testing.T) string {
	t.Helper()
	dbName := os.Getenv("TEST_DB_NAME")
	if dbName == "" {
		envDB := os.Getenv("DB_NAME")
		if envDB != "" && strings.Contains(strings.ToLower(envDB), "test") {
			dbName = envDB
		} else {
			dbName = DefaultTestDBName
		}
	}
	EnsureTestDBName(t, dbName)
	return dbName
}

// GetTestPool creates a connection pool connected exclusively to the test database,
// verifying via current_database() at runtime that it is connected to the test database.
func GetTestPool(t *testing.T) *database.Pool {
	t.Helper()
	dbName := GetTestDBName(t)

	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "social_finance"
	}
	dbPass := os.Getenv("DB_PASSWORD")
	if dbPass == "" {
		dbPass = "social_finance_dev_password"
	}
	dbSSLMode := os.Getenv("DB_SSLMODE")
	if dbSSLMode == "" {
		dbSSLMode = "prefer"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := database.NewPool(ctx, config.Config{
		DBHost:     dbHost,
		DBPort:     dbPort,
		DBName:     dbName,
		DBUser:     dbUser,
		DBPassword: dbPass,
		DBSSLMode:  dbSSLMode,
	})
	if err != nil {
		t.Fatalf("failed to create test DB pool for %s: %v", dbName, err)
	}

	// Runtime hard safety check: verify PostgreSQL current_database() directly from the server.
	var currentDB string
	err = pool.Raw().QueryRowContext(ctx, "SELECT current_database()").Scan(&currentDB)
	if err != nil {
		pool.Close()
		t.Fatalf("failed to query current_database(): %v", err)
	}
	EnsureTestDBName(t, currentDB)

	t.Cleanup(func() {
		pool.Close()
	})

	return pool
}

// GetTestSQLDB returns a standard *sql.DB connection to the test database.
func GetTestSQLDB(t *testing.T) *sql.DB {
	t.Helper()
	dbName := GetTestDBName(t)

	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "social_finance"
	}
	dbPass := os.Getenv("DB_PASSWORD")
	if dbPass == "" {
		dbPass = "social_finance_dev_password"
	}
	dbSSLMode := os.Getenv("DB_SSLMODE")
	if dbSSLMode == "" {
		dbSSLMode = "prefer"
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		dbUser, dbPass, dbHost, dbPort, dbName, dbSSLMode)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open test db %s: %v", dbName, err)
	}

	// Runtime hard safety check
	var currentDB string
	if err := db.QueryRow("SELECT current_database()").Scan(&currentDB); err != nil {
		db.Close()
		t.Fatalf("ping/query current_database() on test db: %v", err)
	}
	EnsureTestDBName(t, currentDB)

	t.Cleanup(func() {
		db.Close()
	})
	return db
}
