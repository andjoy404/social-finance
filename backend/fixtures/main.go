//go:build ignore

package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/alexedwards/argon2id"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	fixtureEmailA         = "e2e.user-a@social-finance-test.internal"
	fixtureEmailPengurusA = "e2e.pengurus-a@social-finance-test.internal"
	fixtureEmailB         = "e2e.user-b@social-finance-test.internal"
	fixtureEmailC         = "e2e.user-c@social-finance-test.internal"
	fixtureEmailSA        = "e2e.super-admin@social-finance-test.internal"
	fixturePassword       = "E2E_TEST_PASSWORD"
	fixturePasswordSA     = "E2E_SUPER_ADMIN_PASSWORD"
)

// FixtureSetup creates isolated E2E test data for Checkpoint C.
// Usage: docker compose run --rm backend go run ./fixtures/
func main() {
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "postgres"
	}
	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "5432"
	}
	user := os.Getenv("DB_USER")
	if user == "" {
		user = "social_finance"
	}
	pwd := os.Getenv("DB_PASSWORD")
	if pwd == "" {
		pwd = "social_finance_dev_password"
	}
	name := os.Getenv("DB_NAME")
	if name == "" {
		name = "social_finance_test"
	}
	if name == "social_finance" {
		log.Fatal("HARD SAFETY GUARD BLOCKED: cannot seed fixtures into development database 'social_finance'!")
	}
	sslmode := os.Getenv("DB_SSLMODE")
	if sslmode == "" {
		sslmode = "prefer"
	}

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, pwd, name, sslmode)

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatal("open:", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatal("ping:", err)
	}

	pw := os.Getenv(fixturePassword)
	if pw == "" {
		pw = "TestUser123!"
	}
	hash, err := argon2id.CreateHash(pw, argon2id.DefaultParams)
	if err != nil {
		log.Fatal("hash:", err)
	}

	ctx := context.Background()

	_, err = db.ExecContext(ctx,
		`DELETE FROM user_rt_memberships WHERE rt_id IN (SELECT id FROM rts WHERE name LIKE 'E2E%%')`)
	if err != nil {
		log.Fatal("cleanup memberships:", err)
	}
	_, err = db.ExecContext(ctx,
		`DELETE FROM users WHERE email IN ($1, $2, $3, $4, $5)`, fixtureEmailA, fixtureEmailPengurusA, fixtureEmailB, fixtureEmailC, fixtureEmailSA)
	if err != nil {
		log.Fatal("cleanup users:", err)
	}
	_, err = db.ExecContext(ctx,
		`DELETE FROM rts WHERE name LIKE 'E2E %'`)
	if err != nil {
		log.Fatal("cleanup rts:", err)
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO rts (name, rw, rt, address, head_name, is_active)
		 VALUES ($1, 1, $2, $3, $4, true)`,
		"E2E Test RT A", "E2EA", "Jl. Synthetic A", "E2E Head A")
	if err != nil {
		log.Fatal("insert rta:", err)
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO rts (name, rw, rt, address, head_name, is_active)
		 VALUES ($1, 1, $2, $3, $4, true)`,
		"E2E Test RT B", "E2EB", "Jl. Synthetic B", "E2E Head B")
	if err != nil {
		log.Fatal("insert rtb:", err)
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO rts (name, rw, rt, address, head_name, is_active)
		 VALUES ($1, 1, $2, $3, $4, true)`,
		"E2E Test RT C", "E2EC", "Jl. Synthetic C", "E2E Head C")
	if err != nil {
		log.Fatal("insert rtc:", err)
	}

	// Hash password for super_admin (same password, different identity).
	// No separate membership for system-only super_admin.
	pwSA := os.Getenv(fixturePassword)
	if pwSA == "" {
		pwSA = "TestUser123!"
	}
	hashSA, err := argon2id.CreateHash(pwSA, argon2id.DefaultParams)
	if err != nil {
		log.Fatal("hash super_admin:", err)
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO users (email, phone, password_hash, full_name, is_active, system_role) VALUES
		 ($1, $2, $3, $4, true, 'super_admin')`,
		fixtureEmailSA, "", hashSA, "E2E Super Admin")
	if err != nil {
		log.Fatal("insert super_admin:", err)
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO users (email, phone, password_hash, full_name, is_active) VALUES
		 ($1, $2, $3, $4, true)`,
		fixtureEmailA, "+62811111111", hash, "E2E User A")
	if err != nil {
		log.Fatal("insert user a:", err)
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO users (email, phone, password_hash, full_name, is_active) VALUES
		 ($1, $2, $3, $4, true)`,
		fixtureEmailPengurusA, "+62811111112", hash, "E2E Pengurus A")
	if err != nil {
		log.Fatal("insert user pengurus a:", err)
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO users (email, phone, password_hash, full_name, is_active) VALUES
		 ($1, $2, $3, $4, true)`,
		fixtureEmailB, "+62822222222", hash, "E2E User B")
	if err != nil {
		log.Fatal("insert user b:", err)
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO users (email, phone, password_hash, full_name, is_active) VALUES
		 ($1, $2, $3, $4, true)`,
		fixtureEmailC, "+62833333333", hash, "E2E User C")
	if err != nil {
		log.Fatal("insert user c:", err)
	}

	var rtaID, rtbID, rtcID, uaID, uPengurusAID, ubID, ucID string
	err = db.QueryRowContext(ctx, `SELECT id FROM rts WHERE rt = 'E2EA' LIMIT 1`).Scan(&rtaID)
	if err != nil {
		log.Fatal("lookup rta:", err)
	}
	err = db.QueryRowContext(ctx, `SELECT id FROM rts WHERE rt = 'E2EB' LIMIT 1`).Scan(&rtbID)
	if err != nil {
		log.Fatal("lookup rtb:", err)
	}
	err = db.QueryRowContext(ctx, `SELECT id FROM rts WHERE rt = 'E2EC' LIMIT 1`).Scan(&rtcID)
	if err != nil {
		log.Fatal("lookup rtc:", err)
	}
	err = db.QueryRowContext(ctx, `SELECT id FROM users WHERE email = $1 LIMIT 1`,
		fixtureEmailA).Scan(&uaID)
	if err != nil {
		log.Fatal("lookup ua:", err)
	}
	err = db.QueryRowContext(ctx, `SELECT id FROM users WHERE email = $1 LIMIT 1`,
		fixtureEmailPengurusA).Scan(&uPengurusAID)
	if err != nil {
		log.Fatal("lookup uPengurusA:", err)
	}
	err = db.QueryRowContext(ctx, `SELECT id FROM users WHERE email = $1 LIMIT 1`,
		fixtureEmailB).Scan(&ubID)
	if err != nil {
		log.Fatal("lookup ub:", err)
	}
	err = db.QueryRowContext(ctx, `SELECT id FROM users WHERE email = $1 LIMIT 1`,
		fixtureEmailC).Scan(&ucID)
	if err != nil {
		log.Fatal("lookup uc:", err)
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO user_rt_memberships (user_id, rt_id, role, is_active) VALUES ($1, $2, $3, true)`,
		uaID, rtaID, "bendahara")
	if err != nil {
		log.Fatal("insert membership a:", err)
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO user_rt_memberships (user_id, rt_id, role, is_active) VALUES ($1, $2, $3, true)`,
		uPengurusAID, rtaID, "pengurus")
	if err != nil {
		log.Fatal("insert membership pengurus a:", err)
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO user_rt_memberships (user_id, rt_id, role, is_active) VALUES ($1, $2, $3, true)`,
		ubID, rtbID, "warga")
	if err != nil {
		log.Fatal("insert membership b:", err)
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO user_rt_memberships (user_id, rt_id, role, is_active) VALUES ($1, $2, $3, true)`,
		ucID, rtcID, "pengurus")
	if err != nil {
		log.Fatal("insert membership c:", err)
	}

	fmt.Println("E2E fixtures created: 5 users, 3 RTs, 4 memberships in test database.")
}
