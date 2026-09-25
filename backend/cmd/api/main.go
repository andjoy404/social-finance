package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"social-finance/internal/auth"
	"social-finance/internal/config"
	"social-finance/internal/database"
	"social-finance/internal/finance"
	"social-finance/internal/health"
	"social-finance/internal/household"
	httpmw "social-finance/internal/http"
	httpx "social-finance/internal/http"
	"social-finance/internal/migration"
	rtpkg "social-finance/internal/rt"
)

func main() {
	// Check for --bootstrap flag first.
	if len(os.Args) > 1 && os.Args[1] == "--bootstrap" {
		if err := runBootstrap(); err != nil {
			slog.Error("bootstrap failed", "error", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Check for --preflight flag.
	if len(os.Args) > 1 && os.Args[1] == "--preflight" {
		if err := runPreFlight(); err != nil {
			slog.Error("preflight failed", "error", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration error", "error", err)
		os.Exit(1)
	}

	// Structured logging: print JSON in all environments.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slogLevel(cfg.AppEnv),
	})))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := database.NewPool(ctx, cfg)
	if err != nil {
		slog.Error("database failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	slog.Info("database connected", "host", cfg.DBHost, "database", cfg.DBName)

	// Initialize auth module.
	if cfg.JWTSecret != "" {
		auth.SigningSecret = []byte(cfg.JWTSecret)
	}
	if cfg.JWTExpiry > 0 {
		auth.DefaultAccessTokenLifetime = time.Duration(cfg.JWTExpiry) * time.Second
	}
	authSvc := auth.NewService(auth.ServiceOptions{
		RefreshLifetime: time.Duration(cfg.RefreshExpiry) * time.Second,
	})
	authHandler := auth.NewHandler(pool, authSvc)

	router := buildRouter(pool, authHandler)

	addr := fmt.Sprintf(":%d", cfg.HTTPPort)
	slog.Info("server starting", "port", cfg.HTTPPort, "env", cfg.AppEnv)

	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown on SIGINT / SIGTERM.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-quit
	slog.Info("shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped")
}

// runBootstrap creates a default SUPER_ADMIN user based on system_role.

// It reads configuration from environment variables:

//   - BOOTSTRAP_EMAIL: email for the super admin (required)

//   - BOOTSTRAP_PASSWORD: password for the super admin (required)

//   - BOOTSTRAP_NAME: full name for the super admin (default "Super Admin")

//

// This endpoint is NOT publicly exposed. It is run via:

//   docker compose run --rm backend server --bootstrap

func runBootstrap() error {

	cfg, err := config.Load()

	if err != nil {

		return fmt.Errorf("load config: %w", err)

	}

	// Initialize auth module for hashing.

	if cfg.JWTSecret != "" {

		auth.SigningSecret = []byte(cfg.JWTSecret)

	}

	if cfg.JWTExpiry > 0 {

		auth.DefaultAccessTokenLifetime = time.Duration(cfg.JWTExpiry) * time.Second

	}

	pool, err := database.NewPool(context.Background(), cfg)

	if err != nil {

		return fmt.Errorf("database connection: %w", err)

	}

	defer pool.Close()

	// Read and normalize bootstrap variables.

	email := strings.ToLower(strings.TrimSpace(os.Getenv("BOOTSTRAP_EMAIL")))

	password := os.Getenv("BOOTSTRAP_PASSWORD")

	name := os.Getenv("BOOTSTRAP_NAME")

	if name == "" {

		name = "Super Admin"

	}

	if email == "" {

		return fmt.Errorf("BOOTSTRAP_EMAIL is required")

	}

	if password == "" {

		return fmt.Errorf("BOOTSTRAP_PASSWORD is required")

	}

	// Check if a super_admin user already exists (users.system_role).

	checkQ := `SELECT COUNT(*) > 0 FROM users WHERE system_role = 'super_admin' LIMIT 1`

	var exists bool

	if err := pool.Raw().QueryRowContext(context.Background(), checkQ).Scan(&exists); err != nil {

		return fmt.Errorf("check existing admin: %w", err)

	}

	if exists {

		slog.Info("bootstrap: super admin already exists, skipping")

		return nil

	}

	// Hash the password.

	hashed, err := auth.Hash(password)

	if err != nil {

		return fmt.Errorf("hash password: %w", err)

	}

	// Create user with system_role=super_admin (no fake membership).

	tx, err := pool.BeginTx(context.Background())

	if err != nil {

		return fmt.Errorf("begin tx: %w", err)

	}

	defer tx.Rollback()

	phone := "" // no phone for bootstrap user

	userID, err := auth.CreateNewUser(context.Background(), tx, email, &phone, hashed, name, "super_admin")

	if err != nil {

		return fmt.Errorf("create user: %w", err)

	}

	if err := tx.Commit(); err != nil {

		return fmt.Errorf("commit bootstrap tx: %w", err)

	}

	slog.Info("bootstrap: super admin user created", "user_id", userID, "email", email)

	return nil

}

// runPreFlight runs the migration 008 preflight check.
func runPreFlight() error {
	cfg := config.Config{
		DBHost:     getEnvDefault("DB_HOST", "postgres"),
		DBPort:     getEnvDefault("DB_PORT", "5432"),
		DBName:     getEnvDefault("DB_NAME", "social_finance"),
		DBUser:     getEnvDefault("DB_USER", "social_finance"),
		DBPassword: getEnvDefault("DB_PASSWORD", "social_finance_dev_password"),
		DBSSLMode:  getEnvDefault("DB_SSLMODE", "disable"),
	}
	sqlDB, err := openDB(cfg)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer sqlDB.Close()

	result, err := migration.Run(context.Background(), sqlDB)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("### Migration 008 Preflight ###")
	if result.BlockingCount > 0 {
		fmt.Printf("MIGRATION 008 PREFLIGHT: FAIL\n")
		fmt.Printf("Blocking conflicts: %d\n\n", result.BlockingCount)
		for i, c := range result.Checks {
			if c.Severity == "blocking" {
				fmt.Printf("  [%d] %s: %s\n", i+1, c.Name, c.Message)
			}
		}
	} else {
		fmt.Println("MIGRATION 008 PREFLIGHT: PASS")
		fmt.Printf("Blocking conflicts: 0\n")
	}
	fmt.Printf("\n%s\n", string(mustMarshal(result)))

	return nil
}

// getEnvDefault returns the env var value or fallback.
func getEnvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// openDB opens a raw *sql.DB connection using config values.
func openDB(cfg config.Config) (*sql.DB, error) {
	// Use pgx driver like database.NewPool, but without Pool wrapper.
	sqlDB, err := sql.Open("pgx",
		fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
			cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName, cfg.DBSSLMode))
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	return sqlDB, nil
}

// mustMarshal is a small JSON encoder helper.
func mustMarshal(v any) []byte {
	b, _ := json.MarshalIndent(v, "", "  ")
	return b
}

// buildRouter wires up the HTTP router with middleware and endpoints.
func buildRouter(pool *database.Pool, authH *auth.Handler) http.Handler {
	r := chi.NewRouter()

	// Core middleware.
	r.Use(httpmw.RequestID)
	r.Use(httpmw.PanicRecovery)
	r.Use(httpmw.Logging)

	// Health endpoint (unauthenticated).
	r.Get("/health", health.Handler(pool))

	appEnv := os.Getenv("APP_ENV")
	if appEnv == "" {
		appEnv = "development"
	}
	loginLimiter := auth.LoginRateLimiter()
	if appEnv == "development" || appEnv == "test" {
		loginLimiter = auth.NewRateLimiter(1000, 5*time.Minute)
	}
	refreshLimiter := auth.RefreshRateLimiter()
	if appEnv == "development" || appEnv == "test" {
		refreshLimiter = auth.NewRateLimiter(1000, 15*time.Minute)
	}

	r.Post("/api/v1/auth/login", loginLimiter.LimitFunc(func(r *http.Request) string {
		return auth.ClientIP(r)
	})(authH.Login()).(http.HandlerFunc))
	r.Post("/api/v1/auth/refresh", refreshLimiter.LimitFunc(func(r *http.Request) string {
		return auth.ClientIP(r)
	})(authH.Refresh()).(http.HandlerFunc))

	// Protected auth routes.
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Post("/api/v1/auth/logout", authH.Logout().(http.HandlerFunc))
		r.Get("/api/v1/auth/me", authH.Me().(http.HandlerFunc))
	})

	// Test environment inspection endpoint used by E2E tests to ensure test DB isolation.
	r.Get("/__test__/info", func(w http.ResponseWriter, r *http.Request) {
		var dbName string
		_ = pool.Raw().QueryRowContext(r.Context(), "SELECT current_database()").Scan(&dbName)
		writeJSON(w, http.StatusOK, map[string]string{
			"status":   "ok",
			"app_env":  appEnv,
			"database": dbName,
		})
	})

	// ── Checkpoint C test-only middleware exercise endpoints ────────────
	// These handlers exist solely to verify middleware behavior via HTTP.
	// They are NOT Phase 3 business endpoints and carry no business logic.

	// 1. RequireRole(bendahara) — exercises tenant-role authorization
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireRole(auth.RoleBendahara))
		r.Get("/__test__require_role_bendahara", func(w http.ResponseWriter, r *http.Request) {
			ac := auth.GetAuthContext(r)
			if ac == nil {
				httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing auth")
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{
				"status":        "ok",
				"user_id":       ac.UserID,
				"membership_id": ac.MembershipID,
				"rt_id":         ac.RTID,
				"tenant_role":   string(ac.TenantRole),
				"system_role":   string(ac.SystemRole),
			})
		})
	})

	// 2. RequireSystemRole(super_admin) — exercises system-level authorization
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireSystemRole(auth.SystemRoleSuperAdmin))
		r.Get("/__test__require_system_admin", func(w http.ResponseWriter, r *http.Request) {
			ac := auth.GetAuthContext(r)
			writeJSON(w, http.StatusOK, map[string]string{
				"status":      "ok",
				"system_role": string(ac.SystemRole),
			})
		})
	})

	// 3. Test tenant override — verifies client-supplied rt_id is ignored
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireRole(auth.RoleBendahara))
		r.Get("/__test/override/query", func(w http.ResponseWriter, r *http.Request) {
			ac := auth.GetAuthContext(r)
			queryRTID := r.URL.Query().Get("rt_id")
			queryMID := r.URL.Query().Get("membership_id")
			writeJSON(w, http.StatusOK, map[string]string{
				"auth_rt_id":       ac.RTID,
				"auth_mid":         ac.MembershipID,
				"auth_role":        string(ac.TenantRole),
				"client_query_rt":  queryRTID,
				"client_query_mid": queryMID,
			})
		})
		// Chi v5 path-param variant — match /__test__/override/:fake_rt_id
		r.Get("/__test__/override/*", func(w http.ResponseWriter, r *http.Request) {
			ac := auth.GetAuthContext(r)
			fakePathVal := r.PathValue("*")
			writeJSON(w, http.StatusOK, map[string]string{
				"auth_rt_id":  ac.RTID,
				"auth_mid":    ac.MembershipID,
				"auth_role":   string(ac.TenantRole),
				"client_path": fakePathVal,
			})
		})
	})
	// ────────────────────────────────────────────────────────────────────

	// RT management routes — system-level, SUPER_ADMIN only.

	rtSvc := rtpkg.NewService()
	rtHandler := rtpkg.NewHandler(rtSvc, pool)

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireSystemRole(auth.SystemRoleSuperAdmin))
		r.Get("/api/v1/rts", rtHandler.ListAll)
		r.Post("/api/v1/rts", rtHandler.Create)
		r.Get("/api/v1/rts/{id}", rtHandler.GetByID)
		r.Patch("/api/v1/rts/{id}", rtHandler.Update)
		r.Patch("/api/v1/rts/{id}/deactivate", rtHandler.Deactivate)
	})

	// Household and resident routes (tenant-scoped, requires auth context).
	hh := household.NewHandler(pool)

	// Read access: warga, bendahara, pengurus.
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireRole(auth.RoleWarga, auth.RoleBendahara, auth.RolePengurus))
		r.Get("/api/v1/households", hh.ListHouseholds)
		r.Get("/api/v1/households/{id}", hh.GetHousehold)
		r.Get("/api/v1/residents", hh.ListResidents)
		r.Get("/api/v1/residents/{id}", hh.GetResident)
		r.Get("/api/v1/warga/export", household.HandleWargaExport(pool))
		r.Get("/api/v1/warga/export/xlsx", household.HandleWargaExportXLSX(pool))
		r.Get("/api/v1/warga/template", household.HandleWargaTemplateXLSX(pool))
	})

	// Write access: pengurus only. System-level SUPER_ADMIN can access and
	// will derive target RT from existing resources (not from auth context).
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireRole(auth.RolePengurus))
		r.Post("/api/v1/households", hh.CreateHousehold)
		r.Post("/api/v1/households/{id}/move", hh.MoveHousehold)
		r.Patch("/api/v1/households/{id}", hh.UpdateHousehold)
		r.Delete("/api/v1/households/{id}", hh.DeactivateHousehold)
		r.Post("/api/v1/residents", hh.CreateResident)
		r.Post("/api/v1/residents/{id}/move", hh.MoveResident)
		r.Patch("/api/v1/residents/{id}", hh.UpdateResident)
		r.Delete("/api/v1/residents/{id}", hh.DeactivateResident)
		r.Post("/api/v1/warga/import/preview", household.HandleWargaImportPreview(pool))
		r.Post("/api/v1/warga/import/commit", household.HandleWargaImportCommit(pool))
		r.Post("/api/v1/warga/import/preview/xlsx", household.HandleWargaImportPreviewXLSX(pool))
		r.Post("/api/v1/warga/import/commit/xlsx", household.HandleWargaImportCommitXLSX(pool))
	})

	// Finance module
	finSvc := finance.NewService(pool)
	finH := finance.NewHandler(pool, finSvc)

	// Financial read access & warga payments: warga, bendahara, pengurus
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireRole(auth.RoleWarga, auth.RoleBendahara, auth.RolePengurus))
		r.Get("/api/v1/categories", finH.ListCategories)
		r.Get("/api/v1/dues", finH.ListDues)
		r.Get("/api/v1/bills", finH.ListBills)
		r.Get("/api/v1/bills/{id}", finH.GetBill)
		r.Get("/api/v1/payments", finH.ListPayments)
		r.Get("/api/v1/payments/{id}", finH.GetPayment)
		r.Get("/api/v1/transactions", finH.ListTransactions)
		r.Get("/api/v1/transactions/{id}", finH.GetTransaction)
		r.Get("/api/v1/reports/balance", finH.GetBalance)

		// Self-submitted payments by warga or staff
		r.Post("/api/v1/payments", finH.CreatePayment)
	})

	// Master categories management: pengurus only (Bendahara cannot delete categories)
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireRole(auth.RolePengurus))
		r.Post("/api/v1/categories", finH.CreateCategory)
		r.Patch("/api/v1/categories/{id}", finH.UpdateCategory)
		r.Delete("/api/v1/categories/{id}", finH.DeactivateCategory)
	})

	// Financial operations: bendahara & pengurus
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireRole(auth.RoleBendahara, auth.RolePengurus))
		r.Post("/api/v1/dues", finH.CreateDue)
		r.Patch("/api/v1/dues/{id}", finH.UpdateDue)
		r.Delete("/api/v1/dues/{id}", finH.DeactivateDue)
		r.Post("/api/v1/bills", finH.CreateBill)
		r.Post("/api/v1/bills/generate", finH.GenerateBills)
		r.Post("/api/v1/payments/{id}/verify", finH.VerifyPayment)
		r.Post("/api/v1/transactions", finH.CreateTransaction)
		r.Post("/api/v1/transactions/{id}/reverse", finH.ReverseTransaction)
	})

	return r
}

// slogLevel translates APP_ENV into a slog Level.
func slogLevel(env string) slog.Level {
	switch env {
	case "development":
		return slog.LevelDebug
	default:
		return slog.LevelInfo
	}
}

// writeJSON writes a JSON response. Shared helper for test-only handlers.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
