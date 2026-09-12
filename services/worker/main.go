// Bal Suraksha background worker.
// Responsibilities:
//   - Outbox poller: expands approved alert outbox events into delivery jobs.
//   - Web Push dispatcher: encrypts messages for consented browser subscriptions.
//   - Escalation timer: fires escalation for unaccepted assignments past due.
//   - Expiry guard: cancels delivery jobs for expired/withdrawn alerts.
//
// The worker NEVER sends without checking: alert status == ACTIVE,
// revision == approved, expiry > now(), job status == QUEUED, is_test flag matches mode.
package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/balsuraksha/worker/internal/delivery"
	"github.com/jackc/pgx/v5"
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

	if err := validateModes(appMode, notifMode); err != nil {
		log.Fatal().Err(err).Msg("invalid worker configuration")
	}
	var provider delivery.Provider
	if notifMode != "DISABLED" {
		vapid, err := delivery.NewVAPIDProvider(os.Getenv("VAPID_PRIVATE_KEY"), os.Getenv("VAPID_SUBJECT"))
		if err != nil || vapid == nil {
			log.Fatal().Msg("valid VAPID configuration required; sending will not be simulated")
		}
		if vapid.PublicKey() != os.Getenv("VAPID_PUBLIC_KEY") {
			log.Fatal().Msg("VAPID public and private key mismatch")
		}
		provider = vapid
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatal().Err(err).Msg("worker: database connection failed")
	}
	defer pool.Close()
	dispatcher := delivery.NewDispatcher(pool, provider, notifMode == "TEST_ALLOWLIST")

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
				} else if notifMode != "DISABLED" {
					if err := dispatcher.ProcessJobs(ctx); err != nil {
						log.Error().Err(err).Msg("push dispatch error")
					}
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

func validateModes(appMode, notifMode string) error {
	if appMode != "demo" && appMode != "beta" && appMode != "production" {
		return errors.New("APP_MODE must be demo, beta or production")
	}
	if notifMode != "DISABLED" && notifMode != "TEST_ALLOWLIST" && notifMode != "LIVE" {
		return errors.New("NOTIFICATION_MODE must be explicit")
	}
	if appMode == "demo" && notifMode == "LIVE" {
		return errors.New("demo cannot send LIVE notifications")
	}
	return nil
}
func processOutbox(ctx context.Context, pool *pgxpool.Pool, appMode, notifMode string) error {
	if err := validateModes(appMode, notifMode); err != nil {
		return err
	}
	if notifMode == "DISABLED" {
		return nil
	}
	for i := 0; i < 10; i++ {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		err = expandNext(ctx, tx, notifMode == "TEST_ALLOWLIST")
		if err != nil {
			_ = tx.Rollback(ctx)
			if err == pgx.ErrNoRows {
				return nil
			}
			return err
		}
		if err = tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}
func expandNext(ctx context.Context, tx pgx.Tx, test bool) error {
	var ev, id, revision string
	err := tx.QueryRow(ctx, `SELECT id,alert_id,revision_id FROM outbox_events WHERE status='PENDING' AND event_type='alert_activated' ORDER BY created_at LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(&ev, &id, &revision)
	if err != nil {
		return err
	}
	// One transaction owns event expansion. A rollback leaves it PENDING.
	var eligible bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM alerts a JOIN alert_revisions ar ON ar.id=a.active_revision_id JOIN alert_areas area ON area.id=ar.area_id AND area.active AND area.is_test=a.is_test JOIN alert_approvals aa ON aa.revision_id=ar.id AND aa.decision='approve' WHERE a.id=$1 AND ar.id=$2 AND a.status='ACTIVE' AND ar.status='APPROVED' AND ar.expiry_at>NOW() AND a.is_test=$3)`, id, revision, test).Scan(&eligible)
	if err != nil {
		return err
	}
	if eligible {
		// Synthetic campaigns have a cumulative job budget. No silent truncation.
		if test {
			if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(7182402)`); err != nil {
				return err
			}
			var total, matching int
			if err = tx.QueryRow(ctx, `SELECT count(*) FROM delivery_jobs WHERE is_test`).Scan(&total); err != nil {
				return err
			}
			if err = tx.QueryRow(ctx, `SELECT count(*) FROM area_subscriptions s JOIN alert_revisions ar ON ar.area_id=s.area_id WHERE ar.id=$1 AND s.is_test AND s.revoked_at IS NULL AND s.management_secret_hash IS NOT NULL`, revision).Scan(&matching); err != nil {
				return err
			}
			if matching > 20 || total+matching > 60 {
				_, err = tx.Exec(ctx, `UPDATE outbox_events SET status='FAILED',error_message='TEST campaign recipient budget reached' WHERE id=$1`, ev)
				return err
			}
		}
		_, err = tx.Exec(ctx, `INSERT INTO delivery_jobs(outbox_event_id,alert_id,revision_id,subscription_id,is_test) SELECT $1,$2,$3,s.id,$4 FROM area_subscriptions s JOIN alert_revisions ar ON ar.area_id=s.area_id WHERE ar.id=$3 AND s.is_test=$4 AND s.revoked_at IS NULL AND s.management_secret_hash IS NOT NULL ON CONFLICT(revision_id,subscription_id) DO NOTHING`, ev, id, revision, test)
		if err != nil {
			return fmt.Errorf("expand recipients: %w", err)
		}
	}
	_, err = tx.Exec(ctx, `UPDATE outbox_events SET status='DONE',processed_at=NOW() WHERE id=$1`, ev)
	return err
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
