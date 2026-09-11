// Package delivery dispatches queued delivery jobs to push notification providers.
//
// Safety rules:
//   - ALWAYS re-check alert status immediately before send (withdrawal guard).
//   - A provider accepting a request is NOT proof of device delivery.
//   - Ambiguous outcomes (network timeout) are recorded as AMBIGUOUS — never silently retried.
//   - TEST jobs are ONLY sent to is_test=true subscriptions when NOTIFICATION_MODE=TEST_ALLOWLIST.
//   - Maximum 3 attempts per job, then FAILED.
package delivery

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

const maxAttempts = 3

// Provider is the interface for push notification backends.
type Provider interface {
	// Send dispatches a notification to the given endpoint.
	// Returns (providerStatus, isAccepted, error).
	// isAccepted means the provider acknowledged the request — NOT delivery confirmation.
	Send(ctx context.Context, endpoint string, payload NotificationPayload) (providerStatus string, isAccepted bool, err error)
	// Name returns the provider identifier for logging.
	Name() string
}

// NotificationPayload is the content of a push notification.
type NotificationPayload struct {
	Title       string `json:"title"`
	Body        string `json:"body"`
	IsTest      bool   `json:"is_test"`
	AlertID     string `json:"alert_id"`
	RevisionID  string `json:"revision_id"`
	StatusURL   string `json:"status_url"`
}

// Dispatcher processes QUEUED delivery jobs using a configured push provider.
type Dispatcher struct {
	pool     *pgxpool.Pool
	provider Provider
	isTest   bool
}

// NewDispatcher creates a Dispatcher.
// isTest must be true when NOTIFICATION_MODE=TEST_ALLOWLIST.
func NewDispatcher(pool *pgxpool.Pool, provider Provider, isTest bool) *Dispatcher {
	return &Dispatcher{pool: pool, provider: provider, isTest: isTest}
}

// deliveryJobRow represents a queued job row from the DB.
type deliveryJobRow struct {
	ID             string
	AlertID        string
	RevisionID     string
	SubscriptionID string
	Endpoint       string
	IsTest         bool
	AttemptCount   int
}

// ProcessJobs picks up QUEUED jobs and dispatches them.
// Called periodically by the worker's outbox goroutine.
func (d *Dispatcher) ProcessJobs(ctx context.Context) error {
	rows, err := d.pool.Query(ctx, `
		SELECT dj.id, dj.alert_id, dj.revision_id, dj.subscription_id,
		       sub.endpoint, dj.is_test,
		       (SELECT COUNT(*) FROM delivery_attempts da WHERE da.job_id = dj.id) AS attempt_count
		FROM delivery_jobs dj
		JOIN area_subscriptions sub ON sub.id = dj.subscription_id
		WHERE dj.status = 'QUEUED'
		  AND ($1 = FALSE OR dj.is_test = TRUE)
		  AND sub.revoked_at IS NULL
		ORDER BY dj.scheduled_at ASC
		LIMIT 20
		FOR UPDATE OF dj SKIP LOCKED
	`, d.isTest)
	if err != nil {
		return fmt.Errorf("delivery: query jobs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var job deliveryJobRow
		if err := rows.Scan(
			&job.ID, &job.AlertID, &job.RevisionID, &job.SubscriptionID,
			&job.Endpoint, &job.IsTest, &job.AttemptCount,
		); err != nil {
			log.Error().Err(err).Msg("delivery: scan job row")
			continue
		}

		if err := d.dispatchJob(ctx, job); err != nil {
			log.Error().Err(err).Str("job_id", job.ID).Msg("delivery: job dispatch failed")
		}
	}
	return nil
}

// dispatchJob handles a single delivery job with withdrawal guard and retry cap.
func (d *Dispatcher) dispatchJob(ctx context.Context, job deliveryJobRow) error {
	// ── Withdrawal guard: re-check alert is still ACTIVE right before send ──
	var alertStatus string
	var expiryAt time.Time
	err := d.pool.QueryRow(ctx,
		`SELECT a.status, ar.expiry_at
		 FROM alerts a JOIN alert_revisions ar ON ar.id = a.active_revision_id
		 WHERE a.id = $1 AND a.active_revision_id = $2`,
		job.AlertID, job.RevisionID,
	).Scan(&alertStatus, &expiryAt)
	if err != nil {
		log.Warn().Str("job_id", job.ID).Msg("delivery: could not verify alert status, cancelling job")
		_, _ = d.pool.Exec(ctx, `UPDATE delivery_jobs SET status='CANCELLED' WHERE id=$1`, job.ID)
		return nil
	}
	if alertStatus != "ACTIVE" || time.Now().After(expiryAt) {
		log.Info().Str("job_id", job.ID).Str("alert_status", alertStatus).
			Msg("delivery: alert no longer active, cancelling job")
		_, _ = d.pool.Exec(ctx, `UPDATE delivery_jobs SET status='CANCELLED' WHERE id=$1`, job.ID)
		return nil
	}

	// ── Retry cap ──
	if job.AttemptCount >= maxAttempts {
		log.Warn().Str("job_id", job.ID).Int("attempts", job.AttemptCount).
			Msg("delivery: max attempts reached, marking FAILED")
		_, _ = d.pool.Exec(ctx, `UPDATE delivery_jobs SET status='FAILED' WHERE id=$1`, job.ID)
		return nil
	}

	// ── Mark as PROCESSING ──
	_, _ = d.pool.Exec(ctx, `UPDATE delivery_jobs SET status='PROCESSING' WHERE id=$1`, job.ID)

	// ── Build payload ──
	testLabel := ""
	if job.IsTest {
		testLabel = "TEST — FICTIONAL: "
	}
	payload := NotificationPayload{
		Title:      fmt.Sprintf("%sAlert: Missing Child", testLabel),
		Body:       "A verified alert has been issued for your area. Tap to view current status.",
		IsTest:     job.IsTest,
		AlertID:    job.AlertID,
		RevisionID: job.RevisionID,
		StatusURL:  fmt.Sprintf("/alerts/%s/status", job.AlertID),
	}

	attemptNum := job.AttemptCount + 1
	attemptID := ""
	_ = d.pool.QueryRow(ctx, `
		INSERT INTO delivery_attempts (job_id, attempt_number, started_at)
		VALUES ($1, $2, NOW())
		RETURNING id
	`, job.ID, attemptNum).Scan(&attemptID)

	// ── Dispatch to provider ──
	providerStatus, isAccepted, sendErr := d.provider.Send(ctx, job.Endpoint, payload)

	finishedAt := time.Now()
	var finalJobStatus string
	var errMsg *string

	if sendErr != nil {
		// Network error or timeout = AMBIGUOUS — record but don't claim failure or success
		errStr := sendErr.Error()
		errMsg = &errStr
		providerStatus = "AMBIGUOUS"
		isAccepted = false
		finalJobStatus = "QUEUED" // allow retry up to cap
		log.Warn().Str("job_id", job.ID).Err(sendErr).Msg("delivery: send ambiguous — will retry if under cap")
	} else if isAccepted {
		finalJobStatus = "SENT"
		log.Info().Str("job_id", job.ID).Str("provider", d.provider.Name()).
			Str("status", providerStatus).
			Msg("delivery: provider accepted — NOT proof of device delivery")
	} else {
		finalJobStatus = "FAILED"
		log.Error().Str("job_id", job.ID).Str("provider_status", providerStatus).
			Msg("delivery: provider rejected")
	}

	// ── Update attempt record ──
	_, _ = d.pool.Exec(ctx, `
		UPDATE delivery_attempts
		SET finished_at=$1, provider_status=$2, is_provider_accepted=$3, error_message=$4
		WHERE id=$5
	`, finishedAt, providerStatus, isAccepted, errMsg, attemptID)

	// ── Update job status ──
	_, _ = d.pool.Exec(ctx, `UPDATE delivery_jobs SET status=$1 WHERE id=$2`, finalJobStatus, job.ID)

	return nil
}
