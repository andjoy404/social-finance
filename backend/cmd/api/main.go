package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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
	"social-finance/internal/community"
	"social-finance/internal/config"
	"social-finance/internal/database"
	"social-finance/internal/devseed"
	"social-finance/internal/finance"
	"social-finance/internal/fund_location"
	"social-finance/internal/health"
	"social-finance/internal/household"
	httpmw "social-finance/internal/http"
	httpx "social-finance/internal/http"
	"social-finance/internal/migration"
	"social-finance/internal/notification"
	"social-finance/internal/profilephoto"
	rtpkg "social-finance/internal/rt"
	rw "social-finance/internal/rw"
	"social-finance/internal/security"
	"social-finance/internal/storage"
)

func main() {
	// Check for --bootstrap flag first.
	if len(os.Args) > 1 && os.Args[1] == "--bootstrap" {
		cfg, err := config.Load()
		if err != nil {
			slog.Error("configuration error", "error", err)
			os.Exit(1)
		}
		auth.SigningSecret = []byte(cfg.JWTSecret)
		if cfg.JWTExpiry > 0 {
			auth.DefaultAccessTokenLifetime = time.Duration(cfg.JWTExpiry) * time.Second
		}
		pool, err := database.NewPool(context.Background(), cfg)
		if err != nil {
			slog.Error("database connection", "error", err)
			os.Exit(1)
		}
		defer pool.Close()
		if err := runAutoProvision(context.Background(), pool); err != nil {
			slog.Error("auto-provision failed", "error", err)
			os.Exit(1)
		}
		if err := runBootstrapWithPool(context.Background(), pool); err != nil {
			slog.Error("bootstrap failed", "error", err)
			os.Exit(1)
		}
		slog.Info("bootstrap completed successfully")
		os.Exit(0)
	}

	// Check for --seed-dev flag.
	if len(os.Args) > 1 && os.Args[1] == "--seed-dev" {
		if err := runDevSeed(); err != nil {
			slog.Error("seed-dev failed", "error", err)
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

	// Check for --init-storage flag.
	if len(os.Args) > 1 && os.Args[1] == "--init-storage" {
		cfg, err := config.Load()
		if err != nil {
			slog.Error("configuration error", "error", err)
			os.Exit(1)
		}
		s3Storage, err := storage.NewS3Storage(storage.S3Config{
			Endpoint:        cfg.RustFSEndpoint,
			AccessKeyID:     cfg.RustFSAccessKey,
			SecretAccessKey: cfg.RustFSSecretKey,
			BucketName:      cfg.RustFSBucket,
			UseSSL:          cfg.RustFSUseSSL,
		})
		if err != nil {
			slog.Error("storage client initialization failed", "error", err)
			os.Exit(1)
		}
		if err := s3Storage.InitBucket(context.Background()); err != nil {
			slog.Error("storage bucket init failed", "error", err)
			os.Exit(1)
		}
		slog.Info("storage bucket initialized successfully", "bucket", cfg.RustFSBucket)
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
	// Wire database pool for RequireAuth identity validation.
	// Must be set after pool creation succeeds and before any HTTP requests.
	auth.AuthDBPool = pool
	authSvc := auth.NewService(auth.ServiceOptions{
		RefreshLifetime: time.Duration(cfg.RefreshExpiry) * time.Second,
	})
	authHandler := auth.NewHandler(pool, authSvc)

	// Run automatic provisioning (idempotent) before bootstrap.
	if err := runAutoProvision(ctx, pool); err != nil {
		slog.Error("auto-provision failed", "error", err)
		os.Exit(1)
	}

	// Run bootstrap after provisioning.
	if err := runBootstrapWithPool(ctx, pool); err != nil {
		slog.Error("bootstrap failed", "error", err)
		os.Exit(1)
	}

	s3Storage, err := storage.NewS3Storage(storage.S3Config{
		Endpoint:        cfg.RustFSEndpoint,
		AccessKeyID:     cfg.RustFSAccessKey,
		SecretAccessKey: cfg.RustFSSecretKey,
		BucketName:      cfg.RustFSBucket,
		UseSSL:          cfg.RustFSUseSSL,
	})
	var store storage.Service
	if err != nil {
		slog.Warn("failed to initialize s3 storage client, falling back to mock storage", "error", err)
		store = storage.NewMockStorage()
	} else {
		store = s3Storage
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := s3Storage.InitBucket(ctx); err != nil {
				slog.Warn("storage bucket init deferred or failed", "error", err)
			}
		}()
	}

	// Validate scoping configuration before wiring routes.
	scopingCfg, err := auth.LoadScoping()
	if err != nil {
		slog.Error("startup failed: invalid scoping configuration", "error", err)
		os.Exit(1)
	}

	router := buildRouter(pool, authHandler, &scopingCfg, store)

	// Expire voluntary dues once on startup (catch-up) and daily at 00:05 WIB.
	// Test environments do not start background jobs; expiry logic is covered
	// deterministically by tests that pass an explicit reference time.
	if cfg.AppEnv != "test" {
		finance.StartVoluntaryDueExpiryScheduler(ctx, pool)
		secSched := security.NewScheduler(security.NewService(security.NewRepository(pool)))
		secSched.Start()
		defer secSched.Stop()
	}

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

// resolveRT002 looks up the canonical RT 002 (Urena) by its physical tenant
// number (rw=16, rt='002'). Returns the RT UUID, or sql.ErrNoRows if absent.
func resolveRT002(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}) (string, error) {
	var rtID string
	err := q.QueryRowContext(ctx, `SELECT id FROM rts WHERE rt = '002' AND rw = 16 AND is_active = true LIMIT 1`).Scan(&rtID)
	if err != nil {
		return "", err
	}
	return rtID, nil
}

// runAutoProvision creates RW 016 and RT 002 (Urena) if they don't exist,
// and derives SCOPED_RT_IDS from the resulting UUID.
//
// Scope policy:
//   - If SCOPED_RT_IDS is explicitly configured (non-blank), it is treated as
//     authoritative and is left untouched on every restart — operator scope
//     never shrinks or expands silently.
//   - If SCOPED_RT_IDS is blank/unset (e.g. fresh install with docker default
//     SCOPED_RT_IDS=), it is auto-derived to the bootstrapped RT 002 UUID.
//     This keeps fresh installs self-contained while staying fail-closed once
//     LoadScoping validates the resulting value.
//
// Idempotent: an existing RW 016 + RT 002 are always reused; never replaced.
// On a fresh database, brand-new UUIDs are generated — we never hardcode
// legacy canonical UUIDs from a prior database.
func runAutoProvision(ctx context.Context, pool *database.Pool) error {
	explicitScope := strings.TrimSpace(os.Getenv("SCOPED_RT_IDS")) != ""
	// applyDerivedScope installs the auto-derived scope only when the operator
	// did not configure one explicitly.
	applyDerivedScope := func(rtID string) error {
		if explicitScope {
			slog.Info("auto-provision: keeping operator-configured SCOPED_RT_IDS unchanged", "rt_id", rtID)
			return nil
		}
		if err := os.Setenv("SCOPED_RT_IDS", rtID); err != nil {
			return fmt.Errorf("auto-provision: set SCOPED_RT_IDS: %w", err)
		}
		slog.Info("auto-provision: auto-derived SCOPED_RT_IDS from bootstrapped RT", "rt_id", rtID)
		return nil
	}

	if rtID, err := resolveRT002(ctx, pool.Raw()); err == nil {
		if err := applyDerivedScope(rtID); err != nil {
			return err
		}
		slog.Info("auto-provision: RT 002 already exists, reusing existing ID", "rt_id", rtID)
		return nil
	}

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("auto-provision: begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	// ---- Resolve or create RW 016 ----
	var rwID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM rws WHERE rw_number = 16 AND is_active = true LIMIT 1`).Scan(&rwID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// No RW present; create with a fresh UUID.
		err = tx.QueryRowContext(ctx, `
			INSERT INTO rws (rw_number, name, is_active, created_at, updated_at)
			VALUES (16, 'RW 016', true, now(), now())
			RETURNING id
		`).Scan(&rwID)
		if err != nil {
			slog.Error("auto-provision: insert RW 016 failed", "operation", "insert rws", "error", err)
			return fmt.Errorf("auto-provision: insert RW 016: %w", err)
		}
		slog.Info("auto-provision: created new RW 016", "rw_id", rwID)
	case err != nil:
		return fmt.Errorf("auto-provision: query RW 016: %w", err)
	default:
		slog.Info("auto-provision: reusing existing RW 016", "rw_id", rwID)
	}
	if rwID == "" {
		return fmt.Errorf("auto-provision: RW 016 UUID empty after resolution")
	}

	// ---- Resolve or create RT 002 ----
	rtID, err := resolveRT002(ctx, tx)
	if err == nil {
		// RT 002 already present inside the transaction.
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("auto-provision: commit provision tx: %w", err)
		}
		committed = true
		if err := applyDerivedScope(rtID); err != nil {
			return err
		}
		slog.Info("auto-provision: RT 002 already exists, reusing existing ID", "rt_id", rtID)
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("auto-provision: query RT 002 inside tx: %w", err)
	}

	// Fresh RT 002 with a fresh UUID (gen_random_uuid default).
	err = tx.QueryRowContext(ctx, `
		INSERT INTO rts (name, rw, rt, address, head_name, is_active, rw_id)
		VALUES ('Urena', 16, '002', 'GNI Urena', 'Atom', true, $1)
		RETURNING id
	`, rwID).Scan(&rtID)
	if err != nil {
		slog.Error("auto-provision: insert RT 002 failed", "operation", "insert rts", "rw_id", rwID, "error", err)
		return fmt.Errorf("auto-provision: insert RT 002 (rw %s): %w", rwID, err)
	}
	if err := tx.Commit(); err != nil {
		committed = true // commit succeeded; avoid double-rollback no-op
		return fmt.Errorf("auto-provision: commit provision tx: %w", err)
	}
	committed = true
	if err := applyDerivedScope(rtID); err != nil {
		return err
	}
	slog.Info("auto-provision: RW 016 and RT 002 provisioned", "rw_id", rwID, "rt_id", rtID)
	return nil
}

// runBootstrap creates a default SUPER_ADMIN user based on system_role.
// It reads configuration from environment variables:
//   - BOOTSTRAP_EMAIL: email for the super admin (required)
//   - BOOTSTRAP_PASSWORD: password for the super admin (required)
//   - BOOTSTRAP_NAME: full name for the super admin (default "Super Admin")
//
// This endpoint is NOT publicly exposed. It is run via:
//
//	docker compose run --rm backend /tmp/build/api --bootstrap
func runBootstrap() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
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
	return runBootstrapWithPool(context.Background(), pool)
}

func runBootstrapWithPool(ctx context.Context, pool *database.Pool) error {

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

	tx, err := pool.BeginTx(context.Background())
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Check if a super_admin user with this email already exists.
	var userID string
	err = tx.QueryRowContext(context.Background(),
		`SELECT id FROM users WHERE system_role = 'super_admin' AND email = $1 LIMIT 1`, email).Scan(&userID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check existing admin: %w", err)
	}

	if userID == "" {
		// Hash the password and create user.
		hashed, err := auth.Hash(password)
		if err != nil {
			return fmt.Errorf("hash password: %w", err)
		}
		phone := ""
		userID, err = auth.CreateNewUser(context.Background(), tx, email, &phone, hashed, name, "super_admin")
		if err != nil {
			return fmt.Errorf("create user: %w", err)
		}
		slog.Info("bootstrap: super admin user created", "user_id", userID, "email", email)
	} else {
		slog.Info("bootstrap: super admin already exists", "user_id", userID)
	}

	// Ensure membership in RT 002 (Urena) if it exists.
	rt002ID, errRT := resolveRT002(context.Background(), tx)
	if errRT != nil && !errors.Is(errRT, sql.ErrNoRows) {
		return fmt.Errorf("bootstrap: resolve RT 002 for super admin: %w", errRT)
	}
	if rt002ID != "" {
		_, errMem := tx.ExecContext(context.Background(), `
			INSERT INTO user_rt_memberships (id, user_id, rt_id, role, is_active)
			VALUES (gen_random_uuid(), $1, $2, 'pengurus', true)
			ON CONFLICT (user_id, rt_id) WHERE is_active = true DO NOTHING
		`, userID, rt002ID)
		if errMem != nil {
			return fmt.Errorf("link super admin to canonical rt 002: %w", errMem)
		}
		slog.Info("bootstrap: super admin linked to canonical RT 002", "user_id", userID, "rt_id", rt002ID)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit bootstrap tx: %w", err)
	}

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
func buildRouter(pool *database.Pool, authH *auth.Handler, scopingCfg *auth.ScopingConfig, opts ...storage.Service) http.Handler {
	if authH != nil && scopingCfg != nil {
		authH.SetScopingConfig(scopingCfg)
	}
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

	resetLimiter := auth.NewRateLimiter(5, 15*time.Minute)
	if appEnv == "development" || appEnv == "test" {
		resetLimiter = auth.NewRateLimiter(1000, 15*time.Minute)
	}

	r.Post("/api/v1/auth/login", loginLimiter.LimitFunc(func(r *http.Request) string {
		return auth.ClientIP(r)
	})(authH.Login()).(http.HandlerFunc))
	r.Post("/api/v1/auth/refresh", refreshLimiter.LimitFunc(func(r *http.Request) string {
		return auth.ClientIP(r)
	})(authH.Refresh()).(http.HandlerFunc))
	r.Post("/api/v1/auth/reset-password-with-code", resetLimiter.LimitFunc(func(r *http.Request) string {
		return auth.ClientIP(r)
	})(authH.ResetPasswordWithCode()).(http.HandlerFunc))

	// Protected auth routes.
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Post("/api/v1/auth/logout", authH.Logout().(http.HandlerFunc))
		r.Get("/api/v1/auth/me", authH.Me().(http.HandlerFunc))
		r.Put("/api/v1/auth/profile", authH.UpdateProfile().(http.HandlerFunc))
		r.Put("/api/v1/auth/password", authH.ChangePassword().(http.HandlerFunc))
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
	// (Handler is instantiated later, after notification service is available.)

	// Notification credentials must be loaded before RT handler so that
	// default notification template/schedule can be seeded for new RTs.
	fcmCreds := os.Getenv("FCM_CREDENTIALS_FILE")
	if fcmCreds == "" {
		fcmCreds = os.Getenv("FCM_CREDENTIALS")
	}
	fcmProject := os.Getenv("FCM_PROJECT_ID")
	if fcmProject == "" {
		fcmProject = "social-finance-andjoy"
	}

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireSystemRole(auth.SystemRoleSuperAdmin))
	})

	// Household and resident routes (tenant-scoped, requires auth context).
	hh := household.NewHandler(pool)

	// Read access: warga, bendahara, pengurus.
	// jabatan-based users evaluated via position_permissions;
	// role-based users still pass through RequireRole.
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireAllowedRT(pool, *scopingCfg))
		r.Use(auth.RequirePermission(pool, "warga.read"))
		r.Use(auth.RequireRole(auth.RoleWarga, auth.RoleBendahara, auth.RolePengurus))
		r.Get("/api/v1/households", hh.ListHouseholds)
		r.Get("/api/v1/households/{id}", hh.GetHousehold)
		r.Get("/api/v1/residents", hh.ListResidents)
		r.Get("/api/v1/residents/{id}", hh.GetResident)
		r.Get("/api/v1/warga/export", household.HandleWargaExport(pool))
	})

	// Management read access: bendahara, pengurus (warga denied). Super admin allowed.
	// Both role-based (Bendahara, Pengurus) and jabatan-based (Ketua) access allowed.
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireAllowedRT(pool, *scopingCfg))
		r.Use(auth.RequirePermission(pool, "warga.export"))
		r.Use(auth.RequireAccess(
			[]auth.Role{auth.RolePengurus, auth.RoleBendahara},
			[]auth.Jabatan{auth.JabatanKetua, auth.JabatanWakilKetua, auth.JabatanBendahara},
		))
		r.Get("/api/v1/warga/template", household.HandleWargaTemplateXLSX(pool))
		r.Get("/api/v1/warga/export/xlsx", household.HandleWargaExportXLSX(pool))
	})

	// Write access: warga creation (households & residents).
	// Both role-based (Pengurus, Bendahara) and jabatan-based (Ketua, Wakil Ketua, Sekretaris, Bendahara) access allowed.
	// Users with a jabatan must possess the warga.create permission in position_permissions.
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireAllowedRT(pool, *scopingCfg))
		r.Use(auth.RequirePermission(pool, "warga.create"))
		r.Use(auth.RequireAccess(
			[]auth.Role{auth.RolePengurus, auth.RoleBendahara},
			[]auth.Jabatan{auth.JabatanKetua, auth.JabatanWakilKetua, auth.JabatanSekretaris, auth.JabatanBendahara},
		))
		r.Post("/api/v1/households", hh.CreateHousehold)
		r.Post("/api/v1/residents", hh.CreateResident)
	})

	// Pembuatan petugas khusus (keamanan, kebersihan, operasional) tanpa household.
	// Tidak memerlukan warga.create — special resident adalah akun petugas, bukan warga.
	// Hanya pengurus (Ketua/WakilKetua/Sekretaris/Bendahara) atau super admin yang berwenang.
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireAllowedRT(pool, *scopingCfg))
		r.Use(auth.RequireAccess(
			[]auth.Role{auth.RolePengurus, auth.RoleBendahara},
			[]auth.Jabatan{auth.JabatanKetua, auth.JabatanWakilKetua, auth.JabatanSekretaris, auth.JabatanBendahara},
		))
		r.Post("/api/v1/residents/special", hh.CreateSpecialResident)
	})

	// Write access: warga mutation (update, move, deactivate for households & residents).
	// Both role-based (Pengurus, Bendahara) and jabatan-based (Ketua, Wakil Ketua, Sekretaris, Bendahara) access allowed.
	// Users with a jabatan must possess the warga.update permission in position_permissions.
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireAllowedRT(pool, *scopingCfg))
		r.Use(auth.RequirePermission(pool, "warga.update"))
		r.Use(auth.RequireAccess(
			[]auth.Role{auth.RolePengurus, auth.RoleBendahara},
			[]auth.Jabatan{auth.JabatanKetua, auth.JabatanWakilKetua, auth.JabatanSekretaris, auth.JabatanBendahara},
		))
		r.Post("/api/v1/households/{id}/move", hh.MoveHousehold)
		r.Patch("/api/v1/households/{id}", hh.UpdateHousehold)
		r.Delete("/api/v1/households/{id}", hh.DeactivateHousehold)
		r.Post("/api/v1/residents/{id}/move", hh.MoveResident)
		r.Patch("/api/v1/residents/{id}", hh.UpdateResident)
		r.Delete("/api/v1/residents/{id}", hh.DeactivateResident)
		r.Post("/api/v1/residents/{id}/reset-password-code", hh.GenerateResetPasswordCode)
		r.Patch("/api/v1/residents/special/{id}", hh.UpdateSpecialResident)
	})

	// Warga import requires additional warga.import permission (only ketua).
	// Both role-based (Pengurus, Bendahara) and jabatan-based (Ketua) access allowed.
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireAllowedRT(pool, *scopingCfg))
		r.Use(auth.RequirePermission(pool, "warga.create"))
		r.Use(auth.RequirePermission(pool, "warga.update"))
		r.Use(auth.RequirePermission(pool, "warga.import"))
		r.Use(auth.RequireAccess(
			[]auth.Role{auth.RolePengurus, auth.RoleBendahara},
			[]auth.Jabatan{auth.JabatanKetua, auth.JabatanWakilKetua, auth.JabatanBendahara},
		))
		r.Post("/api/v1/warga/import/preview", household.HandleWargaImportPreview(pool))
		r.Post("/api/v1/warga/import/commit", household.HandleWargaImportCommit(pool))
		r.Post("/api/v1/warga/import/preview/xlsx", household.HandleWargaImportPreviewXLSX(pool))
		r.Post("/api/v1/warga/import/commit/xlsx", household.HandleWargaImportCommitXLSX(pool))
	})

	// Finance module
	var store storage.Service
	if len(opts) > 0 && opts[0] != nil {
		store = opts[0]
	} else {
		store = storage.NewMockStorage()
	}

	// Profile photos: upload/replace/clear. Only users whose membership carries
	// the resident.profile_photo.manage permission (currently position 'ketua')
	// may change a resident's photo. Visibility of uploaded photos is enforced
	// by the community image reference check for residents.
	profileH := profilephoto.NewHandler(pool, store)
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireAllowedRT(pool, *scopingCfg))
		// RequirePermission evaluates jabatan-based users against
		// position_permissions (super_admin bypass). Role-based users
		// (jabatan empty) would sail through that gate, so pair it with
		// RequireAccess limited to position 'ketua' (+ super_admin bypass).
		r.Use(auth.RequirePermission(pool, "resident.profile_photo.manage"))
		r.Use(auth.RequireAccess(nil, []auth.Jabatan{auth.JabatanKetua}))
		r.Post("/api/v1/residents/{id}/photo", profileH.Upload)
		r.Delete("/api/v1/residents/{id}/photo", profileH.Delete)
	})
	finSvc := finance.NewService(pool, store)

	// Notification service must be initialised after finance service and before
	// RT handler so that default notification template/schedule can be seeded
	// for new RTs.
	notifRepo := notification.NewRepository(pool)
	notifProvider := notification.NewFCMProvider(fcmCreds, appEnv)
	notifProvider.SetProjectID(fcmProject)
	notifSvc := notification.NewService(notifRepo, notifProvider)
	notifH := notification.NewHandler(notifSvc, pool, store, appEnv)

	// Wire notification into finance service
	finSvc.SetNotifier(notifSvc)

	// Start the generic notification scheduler (auto reminder iuran, etc.).
	// It runs catch-up on startup then ticks every minute. Disabled in the test
	// environment so automated tests are not affected by background pushes.
	if appEnv != "test" {
		notifScheduler := notification.NewScheduler(pool, notifSvc, appEnv)
		notifScheduler.Start(context.Background())
	}

	rtSvc := rtpkg.NewService()
	rtHandler := rtpkg.NewHandler(rtSvc, pool, notifSvc)

	// RT management routes — system-level, SUPER_ADMIN only.
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireSystemRole(auth.SystemRoleSuperAdmin))
		r.Get("/api/v1/rws", rtHandler.ListAllRWs)
		r.Post("/api/v1/rws", rtHandler.CreateRW)
		r.Get("/api/v1/rws/{id}", rtHandler.GetRWByID)
		r.Patch("/api/v1/rws/{id}", rtHandler.UpdateRW)
		r.Patch("/api/v1/rws/{id}/deactivate", rtHandler.DeactivateRW)
		r.Get("/api/v1/rws/{id}/memberships", rtHandler.ListRWMemberships)
		r.Put("/api/v1/rws/{id}/memberships/{userId}/jabatan", rtHandler.SetRWJabatan)
		r.Delete("/api/v1/rws/{id}/memberships/{userId}/jabatan", rtHandler.RemoveRWJabatan)
		r.Get("/api/v1/rts", rtHandler.ListAll)
		r.Post("/api/v1/rts", rtHandler.Create)
		r.Get("/api/v1/rts/{id}", rtHandler.GetByID)
		r.Patch("/api/v1/rts/{id}", rtHandler.Update)
		r.Patch("/api/v1/rts/{id}/deactivate", rtHandler.Deactivate)
		r.Get("/api/v1/rts/{id}/memberships", rtHandler.ListMemberships)
		r.Put("/api/v1/rts/{id}/memberships/{userId}/jabatan", rtHandler.SetJabatan)
		r.Delete("/api/v1/rts/{id}/memberships/{userId}/jabatan", rtHandler.RemoveJabatan)
	})

	finH := finance.NewHandler(pool, finSvc, store)

	// Wire household occupancy bill synchronization hook
	household.SyncOccupancyBillsFn = func(ctx context.Context, tx *sql.Tx, rtID, occID string) error {
		_, err := finance.SyncHouseholdOccupancyBills(ctx, tx, rtID, occID)
		return err
	}

	// Financial read access & warga payments: warga, bendahara, pengurus
	// jabatan-based users evaluated via position_permissions for finance.read.
	// Self-submitted payments by warga/staff use finance.create permission.
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireAllowedRT(pool, *scopingCfg))
		r.Use(auth.RequirePermission(pool, "finance.read"))
		r.Use(auth.RequireRole(auth.RoleWarga, auth.RoleBendahara, auth.RolePengurus))
		r.Get("/api/v1/categories", finH.ListCategories)
		r.Get("/api/v1/dues", finH.ListDues)
		r.Get("/api/v1/dues/{id}/months", finH.GetDueMonths)
		r.Get("/api/v1/bills/overdue-summary", finH.GetOverdueWarningSummary)
		r.Get("/api/v1/bills", finH.ListBills)
		r.Get("/api/v1/bills/{id}", finH.GetBill)
		r.Get("/api/v1/payments", finH.ListPayments)
		r.Get("/api/v1/payments/{id}", finH.GetPayment)
		r.Get("/api/v1/payments/{id}/proof", finH.GetPaymentProof)
		r.Get("/api/v1/transactions", finH.ListTransactions)
		r.Get("/api/v1/transactions/{id}", finH.GetTransaction)
		r.Get("/api/v1/reports/balance", finH.GetBalance)
		r.Get("/api/v1/dashboard/summary", finH.GetDashboardSummary)

		// Self-submitted payments by warga or staff
		r.Post("/api/v1/payments", finH.CreatePayment)
		r.Post("/api/v1/payments/multi-period", finH.CreateMultiPeriodPayment)
		r.Post("/api/v1/payments/{id}/proof", finH.UploadPaymentProof)
	})

	// Master categories management: pengurus only (Bendahara cannot delete categories)
	//
	// NOTE: The seed data has NO category-specific permissions (no "category.manage").
	// Adding RequirePermission here would block ALL jabatan-based users
	// (including ketua) because the position_permissions table has no mapping for it.
	// Both role-based (Pengurus) and jabatan-based (Ketua) access allowed.
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireAllowedRT(pool, *scopingCfg))
		r.Use(auth.RequireAccess(
			[]auth.Role{auth.RolePengurus},
			[]auth.Jabatan{auth.JabatanKetua, auth.JabatanWakilKetua},
		))
		r.Post("/api/v1/categories", finH.CreateCategory)
		r.Patch("/api/v1/categories/{id}", finH.UpdateCategory)
		r.Delete("/api/v1/categories/{id}", finH.DeactivateCategory)
	})

	// Financial operations: bendahara & pengurus
	// jabatan-based users evaluated via position_permissions for finance.create, finance.update, finance.approve.
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireAllowedRT(pool, *scopingCfg))
		r.Use(auth.RequirePermission(pool, "finance.create"))
		r.Use(auth.RequirePermission(pool, "finance.update"))
		r.Use(auth.RequirePermission(pool, "finance.approve"))
		r.Use(auth.RequireAccess(
			[]auth.Role{auth.RoleBendahara, auth.RolePengurus},
			[]auth.Jabatan{auth.JabatanKetua, auth.JabatanWakilKetua, auth.JabatanBendahara},
		))
		r.Post("/api/v1/dues", finH.CreateDue)
		r.Patch("/api/v1/dues/{id}", finH.UpdateDue)
		r.Patch("/api/v1/dues/{id}/periods/{period}/status", finH.UpdateDuePeriodStatus)
		r.Delete("/api/v1/dues/{id}", finH.DeactivateDue)
		r.Post("/api/v1/bills", finH.CreateBill)
		r.Patch("/api/v1/bills/{id}/status", finH.UpdateBillStatus)
		r.Post("/api/v1/bills/generate", finH.GenerateBills)
		r.Delete("/api/v1/bills/{id}", finH.CancelBill)
		r.Post("/api/v1/payments/{id}/verify", finH.VerifyPayment)
		r.Post("/api/v1/payments/{id}/reverse", finH.ReversePayment)
		r.Post("/api/v1/transactions", finH.CreateTransaction)
		r.Put("/api/v1/transactions/{id}", finH.UpdateTransaction)
		r.Patch("/api/v1/transactions/{id}", finH.UpdateTransaction)
		r.Post("/api/v1/transactions/{id}/reverse", finH.ReverseTransaction)
		r.Put("/api/v1/reports/balance/opening/{period}", finH.UpdateOpeningBalance)
	})

	// Fund Location module (Rekening, Brankas, COH)
	locRepo := fundlocation.NewRepository(pool.Raw())
	locSvc := fundlocation.NewService(pool, locRepo)
	locH := fundlocation.NewHandler(pool, locSvc)

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireAllowedRT(pool, *scopingCfg))
		r.Get("/api/v1/fund-locations", locH.ListLocations)
		r.Get("/api/v1/fund-locations/rekening/public", locH.ListPublicRekening)
		r.Post("/api/v1/fund-locations", locH.CreateLocation)
		r.Get("/api/v1/fund-locations/{id}", locH.GetLocation)
		r.Patch("/api/v1/fund-locations/{id}", locH.UpdateLocation)
		r.Delete("/api/v1/fund-locations/{id}", locH.DeactivateLocation)
		r.Get("/api/v1/fund-locations/{id}/history", locH.GetLocationHistory)
		r.Put("/api/v1/fund-locations/{id}/opening-balance", locH.SetOpeningBalance)
		r.Get("/api/v1/fund-transfers", locH.ListTransfers)
		r.Post("/api/v1/fund-transfers", locH.CreateTransfer)
		r.Get("/api/v1/fund-transfers/{id}", locH.GetTransfer)
		r.Post("/api/v1/fund-transfers/{id}/reverse", locH.ReverseTransfer)
		r.Get("/api/v1/reports/fund-locations/reconciliation", locH.GetReconciliationReport)
	})

	rwRepo := rw.NewRepository(pool.Raw())
	rwSvc := rw.NewService(pool, rwRepo, scopingCfg)
	rwSvc.SetNotifier(notifSvc)
	rwHandler := rw.NewHandler(pool, rwSvc, notifSvc, store)

	// RW operational endpoints
	// Requires RW membership via RequireRWScope (from scoping.go)
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireRWScope(pool, *scopingCfg))
		r.Get("/api/v1/rw/{rwId}/dashboard", rwHandler.GetDashboard)
		r.Get("/api/v1/rw/{rwId}/notifications", rwHandler.ListNotifications)
		r.Post("/api/v1/rw/{rwId}/notifications/manual", rwHandler.SendManualPush)
		r.Get("/api/v1/rw/{rwId}/notifications/templates", rwHandler.ListRWTemplates)
		r.Get("/api/v1/rw/{rwId}/notifications/templates/{id}", rwHandler.GetRWTemplateByID)
		r.Get("/api/v1/rw/{rwId}/notifications/templates/{id}/preview", rwHandler.PreviewRWTemplate)
		r.Post("/api/v1/rw/{rwId}/notifications/templates/{id}/preview", rwHandler.PreviewRWTemplate)
		r.Post("/api/v1/rw/{rwId}/notifications/templates/preview", rwHandler.PreviewRWTemplateDirect)
		r.Post("/api/v1/rw/{rwId}/notifications/templates/{id}/send", rwHandler.SendRWTemplateNotification)
		r.Post("/api/v1/rw/{rwId}/notifications/templates", rwHandler.CreateRWTemplate)
		r.Put("/api/v1/rw/{rwId}/notifications/templates/{id}", rwHandler.UpdateRWTemplate)
		r.Patch("/api/v1/rw/{rwId}/notifications/templates/{id}/toggle", rwHandler.ToggleRWTemplateActive)
		r.Delete("/api/v1/rw/{rwId}/notifications/templates/{id}", rwHandler.DeleteRWTemplate)
		r.Get("/api/v1/rw/{rwId}/kas", rwHandler.GetKAS)
		r.Post("/api/v1/rw/{rwId}/kas", rwHandler.CreateKASTransaction)
		r.Get("/api/v1/rw/{rwId}/rts", rwHandler.GetRTRs)
		r.Post("/api/v1/rw/{rwId}/opening-balance", rwHandler.UpdateOpeningBalance)
		r.Post("/api/v1/rw/{rwId}/locations/{id}/opening-balance", rwHandler.AllocateOpeningBalance)
		r.Get("/api/v1/rw/{rwId}/kas/transactions", rwHandler.GetKASTransactions)
		r.Post("/api/v1/rw/{rwId}/kas/transactions", rwHandler.CreateKASTransaction)
		r.Get("/api/v1/rw/{rwId}/kas/transactions/{id}", rwHandler.GetKASTransactionByID)
		r.Post("/api/v1/rw/{rwId}/kas/transactions/{id}/proof", rwHandler.UploadProof)
		r.Get("/api/v1/rw/{rwId}/kas/transactions/{id}/proof", rwHandler.GetProof)

		r.Get("/api/v1/rw/{rwId}/locations", rwHandler.GetLocations)
		r.Post("/api/v1/rw/{rwId}/locations", rwHandler.CreateLocation)
		r.Get("/api/v1/rw/{rwId}/locations/reconcile", rwHandler.GetReconciliation)
		r.Get("/api/v1/rw/{rwId}/transfers", rwHandler.GetTransfers)
		r.Post("/api/v1/rw/{rwId}/transfers", rwHandler.CreateTransfer)
		r.Post("/api/v1/rw/{rwId}/transfers/{id}/reverse", rwHandler.ReverseTransfer)
		r.Get("/api/v1/rw/{rwId}/locations/transfers", rwHandler.GetTransfers)
		r.Post("/api/v1/rw/{rwId}/locations/transfers", rwHandler.CreateTransfer)
		r.Post("/api/v1/rw/{rwId}/locations/transfers/{id}/reverse", rwHandler.ReverseTransfer)
		r.Get("/api/v1/rw/{rwId}/locations/{id}", rwHandler.GetLocationByID)
		r.Put("/api/v1/rw/{rwId}/locations/{id}", rwHandler.UpdateLocation)
		r.Patch("/api/v1/rw/{rwId}/locations/{id}/status", rwHandler.ToggleLocationStatus)
		r.Get("/api/v1/rw/{rwId}/locations/{id}/mutations", rwHandler.GetLocationMutations)
	})

	// Public notification image streaming
	r.Get("/api/v1/notifications/images/*", notifH.GetImage)

	// Notification device token management and notification history (accessible to all authenticated RT members)
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireAllowedRT(pool, *scopingCfg))
		r.Post("/api/v1/notifications/devices", notifH.RegisterDevice)
		r.Delete("/api/v1/notifications/devices", notifH.UnregisterDevice)
		r.Post("/api/v1/devices", notifH.RegisterDevice)
		r.Delete("/api/v1/devices", notifH.UnregisterDevice)
		r.Get("/api/v1/notifications/detail/{id}", notifH.GetNotificationDetail)
		r.Get("/api/v1/notifications/logs", notifH.ListLogs)
		r.Get("/api/v1/notifications/unread-count", notifH.GetUnreadCount)
		r.Post("/api/v1/notifications/logs/{id}/read", notifH.MarkAsRead)
		r.Patch("/api/v1/notifications/logs/{id}/read", notifH.MarkAsRead)
		r.Post("/api/v1/notifications/read/{id}", notifH.MarkAsRead)
		r.Patch("/api/v1/notifications/read/{id}", notifH.MarkAsRead)
		r.Post("/api/v1/notifications/logs/read-all", notifH.MarkAllAsRead)
		r.Patch("/api/v1/notifications/logs/read-all", notifH.MarkAllAsRead)
		r.Post("/api/v1/notifications/read-all", notifH.MarkAllAsRead)
		r.Patch("/api/v1/notifications/read-all", notifH.MarkAllAsRead)
		if appEnv != "production" {
			r.Post("/api/v1/notifications/test-self", notifH.TestSelf)
		}
	})

	// Waste truck arrival and manual push: pengurus or security
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireAllowedRT(pool, *scopingCfg))
		r.Get("/api/v1/notifications/waste-truck/template", notifH.GetWasteTruckTemplate)
		r.Get("/api/v1/notifications/waste-truck/history", notifH.ListWasteTruckHistory)
		r.Post("/api/v1/notifications/waste-truck/upload-image", notifH.UploadImage)
		r.Post("/api/v1/notifications/waste-truck", notifH.TriggerWasteTruckArrival)
		r.Post("/api/v1/notifications/manual", notifH.TriggerManualPush)
	})

	// Notification Management: SUPER_ADMIN and Ketua RT only
	// Requires AllowedRT middleware so that Super Admins cannot access
	// non-whitelisted RTs (e.g. rt-999) via template CRUD or test-push.
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireAllowedRT(pool, *scopingCfg))
		r.Use(auth.RequireNotificationAdmin(pool))
		r.Get("/api/v1/notifications/system-types", notifH.GetSystemTypes)
		r.Get("/api/v1/notifications/management-history", notifH.ListManagementHistory)
		r.Get("/api/v1/notifications/templates", notifH.ListTemplates)
		r.Post("/api/v1/notifications/templates", notifH.CreateTemplate)
		r.Get("/api/v1/notifications/templates/{id}", notifH.GetTemplate)
		r.Put("/api/v1/notifications/templates/{id}", notifH.UpdateTemplate)
		r.Patch("/api/v1/notifications/templates/{id}/toggle-active", notifH.ToggleTemplateActive)
		r.Delete("/api/v1/notifications/templates/{id}", notifH.DeleteTemplate)
		r.Post("/api/v1/notifications/upload-image", notifH.UploadImage)
		r.Post("/api/v1/notifications/test-push", notifH.TestPush)

		// Notification schedules (generic scheduler). Access-controlled with the
		// same notification.manage permission (SUPER_ADMIN + Ketua RT).
		r.Get("/api/v1/notifications/schedules", notifH.ListSchedules)
		r.Post("/api/v1/notifications/schedules", notifH.CreateSchedule)
		r.Get("/api/v1/notifications/schedules/{id}", notifH.GetSchedule)
		r.Put("/api/v1/notifications/schedules/{id}", notifH.UpdateSchedule)
		r.Patch("/api/v1/notifications/schedules/{id}/toggle-active", notifH.ToggleScheduleActive)
		r.Delete("/api/v1/notifications/schedules/{id}", notifH.DeleteSchedule)

		// Scheduler execution history (aggregated per execution) —
		// accessible to notification.manage users (Ketua RT / Super Admin).
		r.Get("/api/v1/notifications/schedules/history", notifH.ListSchedulerHistory)
		r.Get("/api/v1/notifications/schedules/{schedule_id}/recipients", notifH.GetSchedulerRecipientDetail)
	})

	// RT-scoped list of households with 3+ month overdue
	// obligatory bills, field notes, and export. The path uses a legacy
	// /api/v1/rw/{rwId}/... shape for client compatibility, but the feature is
	// RT-scoped: handlers resolve tenant RT via authenticated context/header and
	// do not use {rwId} as authorization scope. Cross-RT access is prevented by
	// RequireAllowedRT + handler tenant resolution.
	//
	// Access: SUPER_ADMIN bypass; jabatan kebersihan, keamanan, or ketua with the
	// corresponding rw.monitoring.* position permission. RequireAccess blocks
	// role-based users without the allowed jabatan (including role-only users who
	// would otherwise pass RequirePermission's role-based pass-through on stacked routes).
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireAllowedRT(pool, *scopingCfg))
		r.Use(auth.RequireAccess(nil, []auth.Jabatan{auth.JabatanKebersihan, auth.JabatanKeamanan, auth.JabatanKetua}))
		r.Group(func(r chi.Router) {
			r.Use(auth.RequirePermission(pool, "rw.monitoring.read"))
			r.Get("/api/v1/rw/{rwId}/monitoring", notifH.ListMonitoringHouseholds)
			r.Get("/api/v1/rw/{rwId}/monitoring/notes/{household_occupancy_id}", notifH.ListMonitoringNotes)
		})
		r.With(auth.RequirePermission(pool, "rw.monitoring.read")).Get("/api/v1/rw/{rwId}/monitoring/read-only", notifH.ListMonitoringHouseholdsReadOnly)
		r.Group(func(r chi.Router) {
			r.Use(auth.RequirePermission(pool, "rw.monitoring.notes.write"))
			r.Post("/api/v1/rw/{rwId}/monitoring/notes/{household_occupancy_id}", notifH.CreateMonitoringNote)
			r.Patch("/api/v1/rw/{rwId}/monitoring/notes/{household_occupancy_id}", notifH.UpdateMonitoringNote)
		})
		r.Group(func(r chi.Router) {
			r.Use(auth.RequirePermission(pool, "rw.monitoring.export"))
			r.Get("/api/v1/rw/{rwId}/monitoring/export/excel", notifH.ExportMonitoringExcel)
			r.Get("/api/v1/rw/{rwId}/monitoring/export/pdf", notifH.ExportMonitoringPDF)
		})
	})

	// ── Community Module (Beranda, Kegiatan, Pengumuman, Banner) ─────────
	communityRepo := community.NewRepository(pool)
	communitySvc := community.NewService(communityRepo, store)
	communitySvc.WithActivityNotifier(activityNotifierAdapter{svc: notifSvc})
	securityRepo := security.NewRepository(pool)
	securitySvc := security.NewService(securityRepo)
	communitySvc.WithOnDutyReader(func(ctx context.Context, rtID string) ([]community.SecurityOfficer, error) {
		items, err := securitySvc.GetOnDutyOfficers(ctx, rtID)
		if err != nil || len(items) == 0 {
			return nil, err
		}
		out := make([]community.SecurityOfficer, 0, len(items))
		for _, item := range items {
			out = append(out, community.SecurityOfficer{
				UserID:      item.UserID,
				FullName:    item.FullName,
				ShiftName:   item.ShiftName,
				ShiftStart:  item.ShiftStart,
				ShiftEnd:    item.ShiftEnd,
				Status:      item.Status,
				PhoneNumber:     item.PhoneNumber,
				AvatarPath:      item.AvatarPath,
				AvatarThumbPath: item.AvatarThumbPath,
			})
		}
		return out, nil
	})
	communityH := community.NewHandlerWithScope(communitySvc, pool, store, *scopingCfg)

	// Public community image streaming. Authorization happens INSIDE the
	// handler (Bearer JWT for API clients, or short-lived HMAC-signed URLs for
	// <img> tags): both modes enforce RT prefix ownership and that the key is
	// referenced by content the caller may see.
	r.Get("/api/v1/community/images/*", communityH.GetImage)

	// Community read access: all authenticated members within their allowed RT.
	// Draft content is NOT exposed by these routes — handlers enforce draft
	// visibility via content-management permissions.
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireAllowedRT(pool, *scopingCfg))
		r.Get("/api/v1/home/dashboard", communityH.GetHomeDashboard)
		r.Get("/api/v1/activities", communityH.ListActivities)
		r.Get("/api/v1/activities/{id}", communityH.GetActivity)
		r.Get("/api/v1/announcements", communityH.ListAnnouncements)
		r.Get("/api/v1/announcements/{id}", communityH.GetAnnouncement)
		r.Get("/api/v1/banner-templates", communityH.ListBannerTemplates)
		r.Get("/api/v1/banner-templates/{id}", communityH.GetBannerTemplate)
		r.Get("/api/v1/banners", communityH.ListBanners)
		r.Get("/api/v1/organization-structure", communityH.GetOrganizationStructure)
		// Short-lived signed URLs for community images (for <img> tags and
		// clients that cannot send Authorization headers). Issues only after
		// the same content authorization as the Bearer mode of GetImage.
		r.Post("/api/v1/community/images/sign", communityH.SignImageURL)
	})

	// Community write endpoints. Authorization model:
	//   RequireAccess(nil, AllJabatans) — jabatan-PRESENCE gate only: blocks
	//   role-only users without any jabatan (e.g. pure warga). It is NOT a
	//   per-operation permission.
	//   RequirePermission(code) — per-operation check against
	//   position_permissions (single source of truth; super admin bypass).
	// Ketua RT (role=warga + jabatan=ketua) passes both; other jabatan holders
	// pass the presence gate and are then granted/denied per their seeded
	// position_permissions rows.
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireAllowedRT(pool, *scopingCfg))
		r.Use(auth.RequireAccess(nil, auth.AllJabatans))
		// Upload is gated per category inside the handler (activities.create /
		// announcements.create) so read-only users cannot upload posters/banners.
		r.Post("/api/v1/community/upload", communityH.UploadImage)
		r.With(auth.RequirePermission(pool, "activities.create")).Post("/api/v1/activities", communityH.CreateActivity)
		r.With(auth.RequirePermission(pool, "activities.update")).Put("/api/v1/activities/{id}", communityH.UpdateActivity)
		r.With(auth.RequirePermission(pool, "activities.publish")).Post("/api/v1/activities/{id}/publish", communityH.PublishActivity)
		r.With(auth.RequirePermission(pool, "activities.cancel")).Post("/api/v1/activities/{id}/cancel", communityH.CancelActivity)
		r.With(auth.RequirePermission(pool, "activities.delete")).Delete("/api/v1/activities/{id}", communityH.DeleteActivity)
		r.With(auth.RequirePermission(pool, "announcements.create")).Post("/api/v1/announcements", communityH.CreateAnnouncement)
		r.With(auth.RequirePermission(pool, "announcements.update")).Put("/api/v1/announcements/{id}", communityH.UpdateAnnouncement)
		r.With(auth.RequirePermission(pool, "announcements.publish")).Post("/api/v1/announcements/{id}/publish", communityH.PublishAnnouncement)
		r.With(auth.RequirePermission(pool, "announcements.archive")).Post("/api/v1/announcements/{id}/archive", communityH.ArchiveAnnouncement)
		r.With(auth.RequirePermission(pool, "announcements.delete")).Delete("/api/v1/announcements/{id}", communityH.DeleteAnnouncement)
	})

	// ── Security Attendance Module (/api/v1/security & /api/v1/security-device) ─
	securityH := security.NewHandler(securitySvc, pool, func(ctx context.Context, rtID string, evalDate time.Time) (any, error) {
		return notification.GetMonitoringHouseholds(ctx, pool, rtID, evalDate)
	}, store)

	posActivateLimiter := auth.NewRateLimiter(15, 5*time.Minute)
	qrGenerateLimiter := auth.NewRateLimiter(60, 1*time.Minute)
	qrScanLimiter := auth.NewRateLimiter(30, 1*time.Minute)
	if appEnv == "development" || appEnv == "test" {
		posActivateLimiter = auth.NewRateLimiter(1000, 5*time.Minute)
		qrGenerateLimiter = auth.NewRateLimiter(1000, 1*time.Minute)
		qrScanLimiter = auth.NewRateLimiter(1000, 1*time.Minute)
	}

	// Unauthenticated one-time Pos device pairing activation (rate-limited).
	r.Post("/api/v1/security/devices/activate", posActivateLimiter.LimitFunc(func(r *http.Request) string {
		return auth.ClientIP(r)
	})(http.HandlerFunc(securityH.ActivateDevice)).(http.HandlerFunc))

	// Authenticated User endpoints (Ketua RT, Petugas Keamanan, Super Admin).
	// Strict RT isolation & position-based RBAC enforced via RequireAllowedRT + RequireAccess + Handler permission checks.
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireAllowedRT(pool, *scopingCfg))
		r.Use(auth.RequireAccess(nil, []auth.Jabatan{auth.JabatanKetua, auth.JabatanKeamanan}))

		r.Get("/api/v1/security/dashboard", securityH.GetDashboard)
		r.Get("/api/v1/security/officers", securityH.ListOfficers)

		r.Get("/api/v1/security/shifts", securityH.ListShifts)
		r.Post("/api/v1/security/shifts", securityH.CreateShift)
		r.Put("/api/v1/security/shifts/{id}", securityH.UpdateShift)

		r.Get("/api/v1/security/schedules", securityH.ListSchedules)
		r.Get("/api/v1/security/schedules/{id}", securityH.GetSchedule)
		r.Post("/api/v1/security/schedules", securityH.CreateSchedule)
		r.Put("/api/v1/security/schedules/{id}", securityH.UpdateSchedule)
		r.Delete("/api/v1/security/schedules/{id}", securityH.CancelSchedule)

		r.Get("/api/v1/security/swaps", securityH.ListSwaps)
		r.Post("/api/v1/security/swaps", securityH.CreateSwap)
		r.Post("/api/v1/security/swaps/{id}/target-respond", securityH.TargetRespondSwap)
		r.Post("/api/v1/security/swaps/{id}/decide", securityH.DecideSwap)

		r.Get("/api/v1/security/attendance/me", securityH.GetMyAttendanceOverview)
		r.With(qrScanLimiter.LimitFunc(func(r *http.Request) string {
			if ac := auth.GetAuthContext(r); ac != nil && ac.UserID != "" {
				return ac.UserID
			}
			return auth.ClientIP(r)
		})).Post("/api/v1/security/attendance/scan", securityH.ScanAttendanceQR)
		r.Get("/api/v1/security/attendance", securityH.ListAttendance)
		r.Get("/api/v1/security/attendance/export", securityH.ExportAttendanceCSV)

		r.Get("/api/v1/security/corrections", securityH.ListCorrections)
		r.Post("/api/v1/security/corrections", securityH.CreateCorrection)
		r.Post("/api/v1/security/corrections/{id}/decide", securityH.DecideCorrection)

		r.Get("/api/v1/security/devices", securityH.ListDevices)
		r.Post("/api/v1/security/devices", securityH.CreateDevice)
		r.Post("/api/v1/security/devices/{id}/rotate-pairing", securityH.RotateDevicePairing)
		r.Post("/api/v1/security/devices/{id}/revoke", securityH.RevokeDevice)

		r.Get("/api/v1/security/guest-books", securityH.ListGuestBooksAdmin)
	})

	// Authenticated Security Pos Device endpoints (scoped to bound RT only, non-user identity).
	r.Group(func(r chi.Router) {
		r.Use(security.DeviceAuthMiddleware(securityRepo))

		r.Get("/api/v1/security-device/me", securityH.PosMe)
		r.With(qrGenerateLimiter.LimitFunc(func(r *http.Request) string {
			if dc := security.GetDeviceContext(r); dc != nil {
				return dc.DeviceID
			}
			return auth.ClientIP(r)
		})).Post("/api/v1/security-device/qr", securityH.PosGenerateQR)
		r.Get("/api/v1/security-device/attendance/today", securityH.PosTodayAttendance)

		r.Get("/api/v1/security-device/guest-books", securityH.PosListGuestBooks)
		r.Post("/api/v1/security-device/guest-books", securityH.PosCreateGuestBook)
		r.Post("/api/v1/security-device/guest-books/{id}/checkout", securityH.PosCheckoutGuestBook)

		r.Get("/api/v1/security-device/warga/households", securityH.PosListHouseholds)
		r.Get("/api/v1/security-device/warga/households/{id}", securityH.PosGetHouseholdDetail)
		r.Get("/api/v1/security-device/monitoring/sampah", securityH.PosMonitoringSampah)
		r.Patch("/api/v1/security-device/monitoring/sampah/{householdOccupancyID}/note", securityH.PosUpdateSampahNote)
		r.Get("/api/v1/security-device/images/*", securityH.PosImage)
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

// runDevSeed populates development mock data into the development database.
func runDevSeed() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	ctx := context.Background()
	pool, err := database.NewPool(ctx, cfg)
	if err != nil {
		return fmt.Errorf("database connection: %w", err)
	}
	defer pool.Close()

	summary, err := devseed.Run(ctx, pool, cfg.AppEnv)
	if err != nil {
		return fmt.Errorf("devseed: %w", err)
	}
	fmt.Printf("devseed completed successfully: %+v\n", summary)
	return nil
}

type activityNotifierAdapter struct {
	svc notification.Service
}

func (a activityNotifierAdapter) NotifyActivityEvent(ctx context.Context, p community.ActivityNotificationPayload, eventType string) error {
	payload := notification.ActivityPayload{
		ID:                p.ID,
		RTID:              p.RTID,
		Title:             p.Title,
		Description:       p.Description,
		Location:          p.Location,
		Organizer:         p.Organizer,
		StartsAt:          p.StartsAt,
		Status:            p.Status,
		PublicationStatus: p.PublicationStatus,
		CreatedAt:         p.CreatedAt,
		UpdatedAt:         p.UpdatedAt,
	}
	if p.EndsAt != nil {
		ends := *p.EndsAt
		payload.EndsAt = &ends
	}
	return a.svc.NotifyActivityEvent(ctx, payload, eventType)
}

func (a activityNotifierAdapter) CancelActivityPendingSchedules(ctx context.Context, rtID, activityID string) error {
	return a.svc.CancelActivityPendingSchedules(ctx, rtID, activityID)
}
