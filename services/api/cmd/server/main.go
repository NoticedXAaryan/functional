// Bal Suraksha API server entry point.
// Loads configuration, connects to the database, registers routes, and starts serving.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

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

func main() {
	// Pretty logging in dev
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	// Load .env if present (for local dev). Ignore error — in Docker, env vars are injected.
	_ = godotenv.Load()

	// ── Configuration ──────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("configuration error — server refused to start")
	}

	log.Info().
		Str("app_mode", string(cfg.AppMode)).
		Str("notification_mode", string(cfg.NotificationMode)).
		Bool("ai_enabled", cfg.AIEnabled).
		Msg("bal suraksha api starting")

	// ── Database ───────────────────────────────────────────────────────────
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// MIGRATION_PATH can be set explicitly (e.g. /app/db/migrations in Docker).
	// Defaults to empty string which skips auto-migration (safe for already-migrated DBs).
	migrationPath := os.Getenv("MIGRATION_PATH")

	pool, err := db.Connect(ctx, cfg.DatabaseURL, migrationPath)
	if err != nil {
		log.Fatal().Err(err).Msg("database connection failed")
	}
	defer pool.Close()

	staffAuth := authMW.RequireStaffSession(cfg, pool)

	// ── Router ─────────────────────────────────────────────────────────────
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Referrer-Policy", "no-referrer")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			next.ServeHTTP(w, r)
		})
	})
	origins := []string{"http://localhost:5173", "http://localhost:5174", "http://127.0.0.1:5173", "http://127.0.0.1:5174"}
	if value := os.Getenv("WEB_ORIGINS"); value != "" {
		origins = strings.Split(value, ",")
	}
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "HEAD"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "Idempotency-Key", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// ── Health ─────────────────────────────────────────────────────────────
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		dbOK := "ok"
		if err := pool.Ping(r.Context()); err != nil {
			dbOK = "error"
		}
		w.Header().Set("Content-Type", "application/json")
		if dbOK != "ok" {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		fmt.Fprintf(w, `{"status":"ok","app_mode":"%s","notification_mode":"%s","database":"%s"}`,
			cfg.AppMode, cfg.NotificationMode, dbOK)
	})

	// ── API v1 routes ──────────────────────────────────────────────────────
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/config", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"mode": cfg.AppMode, "invitation_required": cfg.AppMode != config.AppModeDemo, "intake_open": cfg.AppMode == config.AppModeDemo || (os.Getenv("BETA_INTAKE_ENABLED") == "true" && len(os.Getenv("BETA_INVITATION_CODE")) >= 24), "assessment_available": cfg.AppMode == config.AppModeDemo})
		})

		// Entry points (public, no auth)
		r.Mount("/entry-points", entrypoints.NewHandler(pool, cfg).Routes())

		// Sessions (public — creates JWT)
		r.Mount("/sessions", sessions.NewHandler(pool, cfg).Routes())
		r.Post("/session-access", sessions.NewHandler(pool, cfg).HandleReturnAccess)

		// Assessments (session auth)
		r.Group(func(r chi.Router) {
			r.Use(authMW.RequireSessionToken(cfg, pool))
			r.Mount("/assessments", assessments.NewHandler(pool, cfg).Routes())
		})

		// Cases — session-authenticated (child submits case)
		r.Group(func(r chi.Router) {
			r.Use(authMW.RequireSessionToken(cfg, pool))
			r.Mount("/cases", cases.NewSessionHandler(pool, cfg).Routes())
			// Register session message routes directly — chi does not allow two Mount("/cases")
			sh := messages.NewSessionHandler(pool, cfg)
			r.Get("/cases/{caseID}/messages", sh.HandleList)
			r.Post("/cases/{caseID}/messages", sh.HandleSend)
			// Session DELETE requires auth (HandleEnd reads session claims for ownership check)
			r.Delete("/sessions/{sessionID}", sessions.NewHandler(pool, cfg).HandleEnd)
		})

		// Staff — all require staff session cookie
		r.Route("/staff", func(r chi.Router) {
			r.Get("/auth/config", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Cache-Control", "no-store")
				_ = json.NewEncoder(w).Encode(map[string]string{"mode": "session"})
			})
			// Login and logout are public within /staff (no session required)
			authHandler := authMW.NewStaffAuthHandler(pool, cfg)
			r.Post("/auth/login", authHandler.HandleLogin)
			r.Post("/auth/logout", authHandler.HandleLogout)

			// Everything else requires staff session
			r.Group(func(r chi.Router) {
				r.Use(staffAuth)
				r.Get("/auth/me", authHandler.HandleMe)
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

		// Public alert (no auth — approved projection only)
		r.Mount("/public/alerts", alerts.NewPublicHandler(pool, cfg).Routes())

		// Subscriptions (public — explicit consent required)
		r.Mount("/subscriptions", subscriptions.NewHandler(pool, cfg).Routes())

		// Tips (public — opaque receipt)
		r.Post("/tips", tips.NewPublicHandler(pool, cfg).HandleCreate)
	})

	// ── Start server ───────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%s", cfg.APIHost, cfg.APIPort),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Info().Msg("shutdown signal received")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer shutdownCancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("graceful shutdown failed")
		}
	}()

	log.Info().Str("addr", srv.Addr).Msg("server listening")
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal().Err(err).Msg("server error")
	}
	log.Info().Msg("server stopped")
}
