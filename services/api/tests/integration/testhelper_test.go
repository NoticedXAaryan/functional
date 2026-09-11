// Package integration provides shared test infrastructure for Bal Suraksha integration tests.
//
// Database strategy: Docker Compose (postgres container must be running).
// Tests skip gracefully if DATABASE_URL is not set or DB is unreachable.
//
// Usage:
//   DATABASE_URL=postgres://balsuraksha:balsuraksha@localhost:5432/balsuraksha_test?sslmode=disable \
//   APP_MODE=demo NOTIFICATION_MODE=DISABLED SESSION_SECRET=test-secret-32bytes-minimum-length \
//   go test ./tests/integration/... -v
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/balsuraksha/api/internal/config"
	"github.com/balsuraksha/api/internal/db"
	authMW "github.com/balsuraksha/api/internal/middleware"
	"github.com/balsuraksha/api/internal/modules/alerts"
	"github.com/balsuraksha/api/internal/modules/assessments"
	"github.com/balsuraksha/api/internal/modules/assignments"
	"github.com/balsuraksha/api/internal/modules/cases"
	"github.com/balsuraksha/api/internal/modules/entrypoints"
	"github.com/balsuraksha/api/internal/modules/messages"
	"github.com/balsuraksha/api/internal/modules/routing"
	"github.com/balsuraksha/api/internal/modules/sessions"
	"github.com/balsuraksha/api/internal/modules/subscriptions"
	"github.com/balsuraksha/api/internal/modules/tips"
)

// migrationsDir returns the absolute path to db/migrations, resolved
// relative to this source file's location — not the test binary's cwd.
func migrationsDir() string {
	_, filename, _, _ := runtime.Caller(0)
	// filename = .../services/api/tests/integration/testhelper_test.go
	// navigate up to the repo root: ../../../../
	return filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "db", "migrations")
}

// testEnv holds the shared test server, pool, and config.
type testEnv struct {
	server *httptest.Server
	pool   *pgxpool.Pool
	cfg    *config.Config
}

// newTestEnv spins up the real chi router wired to a test DB.
// It skips the test if the DB is unavailable.
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}

	// Set required env for config loading
	if os.Getenv("SESSION_SECRET") == "" {
		os.Setenv("SESSION_SECRET", "test-secret-minimum-32-bytes-long-here")
	}
	if os.Getenv("APP_MODE") == "" {
		os.Setenv("APP_MODE", "demo")
	}
	if os.Getenv("NOTIFICATION_MODE") == "" {
		os.Setenv("NOTIFICATION_MODE", "DISABLED")
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Pass empty migration path — the Docker DB already has all migrations applied.
	// Running migrations in tests causes Windows file:// URL issues with golang-migrate.
	pool, err := db.Connect(ctx, dbURL, "")
	if err != nil {
		t.Skipf("DB unreachable: %v", err)
	}

	r := buildRouter(pool, cfg)
	srv := httptest.NewServer(r)
	t.Cleanup(func() {
		srv.Close()
		pool.Close()
	})

	return &testEnv{server: srv, pool: pool.Pool, cfg: cfg}
}

// buildRouter constructs the same chi router as main.go.
func buildRouter(pool *db.Pool, cfg *config.Config) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "Idempotency-Key"},
	}))

	r.Route("/api/v1", func(r chi.Router) {
		r.Mount("/entry-points", entrypoints.NewHandler(pool, cfg).Routes())
		r.Mount("/sessions", sessions.NewHandler(pool, cfg).Routes())
		r.Post("/session-access", sessions.NewHandler(pool, cfg).HandleReturnAccess)

		r.Group(func(r chi.Router) {
			r.Use(authMW.RequireSessionToken(cfg, pool))
			r.Mount("/assessments", assessments.NewHandler(pool, cfg).Routes())
			r.Mount("/cases", cases.NewSessionHandler(pool, cfg).Routes())
			// Register session message routes directly — chi does not allow two Mount("/cases")
			sh := messages.NewSessionHandler(pool, cfg)
			r.Get("/cases/{caseID}/messages", sh.HandleList)
			r.Post("/cases/{caseID}/messages", sh.HandleSend)
			// Session DELETE requires auth (HandleEnd reads session claims)
			r.Delete("/sessions/{sessionID}", sessions.NewHandler(pool, cfg).HandleEnd)
		})

		r.Route("/staff", func(r chi.Router) {
			r.Post("/auth/login", authMW.NewStaffAuthHandler(pool, cfg).HandleLogin)
			r.Group(func(r chi.Router) {
				r.Use(authMW.RequireStaffToken(cfg))
				r.Mount("/cases", cases.NewStaffHandler(pool, cfg).Routes())
				r.Mount("/assignments", assignments.NewHandler(pool, cfg).Routes())
				r.Mount("/messages", messages.NewHandler(pool, cfg).Routes())
				r.Mount("/routing", routing.NewHandler(pool, cfg).Routes())
				r.Mount("/alert-drafts", alerts.NewDraftHandler(pool, cfg).Routes())
				r.Mount("/alert-approvals", alerts.NewApprovalHandler(pool, cfg).Routes())
				r.Mount("/alerts", alerts.NewActivationHandler(pool, cfg).Routes())
				r.Mount("/tips", tips.NewStaffHandler(pool, cfg).Routes())
			})
		})

		r.Mount("/public/alerts", alerts.NewPublicHandler(pool, cfg).Routes())
		r.Mount("/subscriptions", subscriptions.NewHandler(pool, cfg).Routes())
		r.Post("/tips", tips.NewPublicHandler(pool, cfg).HandleCreate)
	})
	return r
}

// ── Helper: make a signed session JWT ─────────────────────────────────────────

func makeSessionToken(t *testing.T, cfg *config.Config, sessionID, caseID string) string {
	t.Helper()
	now := time.Now()
	claims := authMW.SessionClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
		},
		SessionID: sessionID,
		CaseID:    caseID,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(cfg.SessionSecret))
	if err != nil {
		t.Fatalf("makeSessionToken: %v", err)
	}
	return signed
}

// ── Helper: make a signed staff JWT ───────────────────────────────────────────

func makeStaffToken(t *testing.T, cfg *config.Config, staffID, orgID, role string) string {
	t.Helper()
	now := time.Now()
	claims := authMW.StaffClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(8 * time.Hour)),
		},
		StaffID:        staffID,
		OrganizationID: orgID,
		Role:           role,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(cfg.SessionSecret))
	if err != nil {
		t.Fatalf("makeStaffToken: %v", err)
	}
	return signed
}

// ── Helpers: HTTP request builders ───────────────────────────────────────────

func postJSON(t *testing.T, url string, body interface{}, headers map[string]string) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("postJSON marshal: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		t.Fatalf("postJSON new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("postJSON do: %v", err)
	}
	return resp
}

func getJSON(t *testing.T, url string, headers map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("getJSON new request: %v", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("getJSON do: %v", err)
	}
	return resp
}

func parseBody(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()
	defer resp.Body.Close()
	var m map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		t.Fatalf("parseBody decode: %v", err)
	}
	return m
}

// ── Helper: database assertion ────────────────────────────────────────────────

func assertDBCount(t *testing.T, pool *pgxpool.Pool, query string, args []interface{}, expected int) {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&count); err != nil {
		t.Fatalf("assertDBCount query failed: %v\nQuery: %s", err, query)
	}
	if count != expected {
		t.Errorf("assertDBCount: expected %d rows, got %d\nQuery: %s", expected, count, query)
	}
}

func urlf(base, path string, args ...interface{}) string {
	return base + fmt.Sprintf(path, args...)
}
