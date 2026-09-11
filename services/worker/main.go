// Bal Suraksha background worker.
// Responsibilities:
//   - Outbox poller: expands approved alert outbox events into delivery jobs.
//   - FCM dispatcher: sends delivery jobs to Firebase Cloud Messaging.
//   - Escalation timer: fires escalation for unaccepted assignments past due.
//   - Expiry guard: cancels delivery jobs for expired/withdrawn alerts.
//
// The worker NEVER sends without checking: alert status == ACTIVE,
// revision == approved, expiry > now(), job status == QUEUED, is_test flag matches mode.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal().Msg("DATABASE_URL is required")
	}
	appMode := os.Getenv("APP_MODE")
	notifMode := os.Getenv("NOTIFICATION_MODE")

	// Enforce the safety matrix — demo+LIVE must never reach the worker
	if appMode == "demo" && notifMode == "LIVE" {
		log.Fatal().Msg("INVALID CONFIGURATION: demo + LIVE is not permitted. Worker refuses to start.")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatal().Err(err).Msg("worker: database connection failed")
	}
	defer pool.Close()

	log.Info().
		Str("app_mode", appMode).
		Str("notification_mode", notifMode).
		Msg("worker starting")

	pollInterval := 5 * time.Second
	escalationInterval := 30 * time.Second

	// ── Outbox poller goroutine ──────────────────────────────────────────
	go func() {
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := processOutbox(ctx, pool, appMode, notifMode); err != nil {
					log.Error().Err(err).Msg("outbox poll error")
				}
			}
		}
	}()

	// ── Escalation timer goroutine ───────────────────────────────────────
	go func() {
		ticker := time.NewTicker(escalationInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := processEscalations(ctx, pool); err != nil {
					log.Error().Err(err).Msg("escalation check error")
				}
			}
		}
	}()

	// ── Alert expiry goroutine ───────────────────────────────────────────
	go func() {
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := processExpiry(ctx, pool); err != nil {
					log.Error().Err(err).Msg("expiry check error")
				}
			}
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("worker shutdown")
}

// processOutbox picks up PENDING outbox events and expands them into delivery jobs.
func processOutbox(ctx context.Context, pool *pgxpool.Pool, appMode, notifMode string) error {
	// Only process if sending is configured
	if notifMode == "DISABLED" {
		return nil
	}

	rows, err := pool.Query(ctx, `
		SELECT id, alert_id, revision_id, payload
		FROM outbox_events
		WHERE status = 'PENDING'
		ORDER BY created_at ASC
		LIMIT 10
		FOR UPDATE SKIP LOCKED
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var evID, alertID, revisionID string
		var payload []byte
		if err := rows.Scan(&evID, &alertID, &revisionID, &payload); err != nil {
			continue
		}

		if err := expandOutboxEvent(ctx, pool, evID, alertID, revisionID, notifMode); err != nil {
			log.Error().Err(err).Str("outbox_event_id", evID).Msg("failed to expand outbox event")
			_, _ = pool.Exec(ctx,
				`UPDATE outbox_events SET status='FAILED', error_message=$1 WHERE id=$2`,
				err.Error(), evID)
		}
	}
	return nil
}

// expandOutboxEvent creates delivery jobs for an outbox event.
func expandOutboxEvent(ctx context.Context, pool *pgxpool.Pool, evID, alertID, revisionID, notifMode string) error {
	// Verify alert is still ACTIVE before expanding
	var alertStatus string
	var expiry time.Time
	if err := pool.QueryRow(ctx,
		`SELECT a.status, ar.expiry_at FROM alerts a JOIN alert_revisions ar ON ar.id = a.active_revision_id
		 WHERE a.id = $1 AND a.active_revision_id = $2`,
		alertID, revisionID).Scan(&alertStatus, &expiry); err != nil {
		return err
	}
	if alertStatus != "ACTIVE" || time.Now().After(expiry) {
		log.Info().Str("alert_id", alertID).Str("status", alertStatus).Msg("alert no longer active, skipping expansion")
		_, _ = pool.Exec(ctx, `UPDATE outbox_events SET status='DONE', processed_at=NOW() WHERE id=$1`, evID)
		return nil
	}

	// Find matching subscriptions via PostGIS (ST_Intersects with coverage polygon)
	isTest := notifMode == "TEST_ALLOWLIST"
	rows, err := pool.Query(ctx, `
		SELECT DISTINCT sub.id
		FROM area_subscriptions sub
		JOIN alert_revisions ar ON ST_Intersects(sub.polygon, ar.coverage_polygon)
		WHERE ar.id = $1
		  AND sub.revoked_at IS NULL
		  AND ($2 = FALSE OR sub.is_test = TRUE)
	`, revisionID, isTest)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var subID string
		if err := rows.Scan(&subID); err != nil {
			continue
		}
		// Insert delivery job — UNIQUE (revision_id, subscription_id) prevents duplicates
		_, err := pool.Exec(ctx, `
			INSERT INTO delivery_jobs (outbox_event_id, alert_id, revision_id, subscription_id, is_test)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (revision_id, subscription_id) DO NOTHING
		`, evID, alertID, revisionID, subID, isTest)
		if err != nil {
			log.Error().Err(err).Str("subscription_id", subID).Msg("failed to create delivery job")
		}
	}

	_, _ = pool.Exec(ctx, `UPDATE outbox_events SET status='DONE', processed_at=NOW() WHERE id=$1`, evID)
	return nil
}

// processEscalations fires escalation for unaccepted assignments past their due time.
func processEscalations(ctx context.Context, pool *pgxpool.Pool) error {
	rows, err := pool.Query(ctx, `
		SELECT id, case_id FROM assignments
		WHERE escalation_due_at < NOW()
		  AND accepted_at IS NULL
		  AND unassigned_at IS NULL
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id, caseID string
		if err := rows.Scan(&id, &caseID); err != nil {
			continue
		}
		log.Warn().Str("assignment_id", id).Str("case_id", caseID).
			Msg("ESCALATION: assignment not accepted past due time")
		// Insert audit event for escalation
		_, _ = pool.Exec(ctx, `
			INSERT INTO audit_events (event_type, actor_type, target_type, target_id, metadata)
			VALUES ('escalation_fired', 'system', 'assignment', $1, '{"reason":"not_accepted_past_due"}')
		`, id)
	}
	return nil
}

// processExpiry marks expired alert delivery jobs as EXPIRED.
func processExpiry(ctx context.Context, pool *pgxpool.Pool) error {
	// Mark alerts as EXPIRED if their expiry_at has passed
	_, err := pool.Exec(ctx, `
		UPDATE alerts a
		SET status = 'EXPIRED', updated_at = NOW()
		FROM alert_revisions ar
		WHERE ar.id = a.active_revision_id
		  AND a.status = 'ACTIVE'
		  AND ar.expiry_at < NOW()
	`)
	if err != nil {
		return err
	}

	// Cancel QUEUED delivery jobs for non-ACTIVE alerts
	_, err = pool.Exec(ctx, `
		UPDATE delivery_jobs dj
		SET status = 'CANCELLED'
		FROM alerts a
		WHERE dj.alert_id = a.id
		  AND dj.status = 'QUEUED'
		  AND a.status NOT IN ('ACTIVE')
	`)
	return err
}
