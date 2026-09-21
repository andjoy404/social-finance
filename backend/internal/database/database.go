package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"social-finance/internal/config"
)

// Pool wraps a *sql.DB and provides a readiness check.
type Pool struct {
	db *sql.DB
}

// NewPool creates a connection pool from the given configuration. It connects
// once immediately so that the application fails fast if the database is
// unreachable, then returns the pool.
func NewPool(ctx context.Context, cfg config.Config) (*Pool, error) {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBName,
		cfg.DBSSLMode,
	)

	// Use stdlib adapter so we get *sql.DB / *sql.Tx which the
	// repository layer expects. The stdlib registers as "pgx" driver.
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// Set connection pool parameters.
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(2)
	sqlDB.SetConnMaxLifetime(10 * time.Minute)

	// Wait for the pool to become ready.
	if err := sqlDB.PingContext(ctx); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return &Pool{db: sqlDB}, nil
}

// Ping checks whether the database is reachable by sending a lightweight
// query.
func (p *Pool) Ping(ctx context.Context) error {
	return p.db.PingContext(ctx)
}

// Acquire returns a connection from the pool.
func (p *Pool) Acquire(ctx context.Context) (*sql.Conn, error) {
	return p.db.Conn(ctx)
}

// BeginTx begins a transaction with default settings.
func (p *Pool) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return p.db.BeginTx(ctx, nil)
}

// Close releases all resources and waits for in-flight connections to finish.
func (p *Pool) Close() {
	p.db.Close()
}

// Raw returns the underlying *sql.DB for advanced usage.
func (p *Pool) Raw() *sql.DB {
	return p.db
}
