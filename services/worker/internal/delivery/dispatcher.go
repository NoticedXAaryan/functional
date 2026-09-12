package delivery

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type Subscription struct{ Endpoint, Auth, P256DH string }
type NotificationPayload struct {
	Title      string    `json:"title"`
	Body       string    `json:"body"`
	IsTest     bool      `json:"is_test"`
	AlertID    string    `json:"alert_id"`
	RevisionID string    `json:"revision_id"`
	StatusURL  string    `json:"status_url"`
	ExpiresAt  time.Time `json:"expires_at"`
}
type Provider interface {
	Send(context.Context, Subscription, NotificationPayload) (string, bool, error)
	Name() string
}
type Dispatcher struct {
	pool     *pgxpool.Pool
	provider Provider
	isTest   bool
}

func NewDispatcher(pool *pgxpool.Pool, provider Provider, isTest bool) *Dispatcher {
	return &Dispatcher{pool, provider, isTest}
}

type job struct {
	ID, AlertID, RevisionID, SubscriptionID, AttemptID string
	Attempt                                            int
}

func (d *Dispatcher) ProcessJobs(ctx context.Context) error {
	if d.provider == nil {
		return errors.New("push provider required")
	}
	// A crashed sender may already have reached the provider. Never blindly replay it.
	if _, err := d.pool.Exec(ctx, `UPDATE delivery_jobs SET status='AMBIGUOUS' WHERE status='PROCESSING' AND processing_started_at<NOW()-INTERVAL '2 minutes'`); err != nil {
		return err
	}
	for i := 0; i < 20; i++ {
		j, err := d.claim(ctx)
		if err == pgx.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		if err = d.dispatch(ctx, j); err != nil {
			return err
		}
	}
	return nil
}
func (d *Dispatcher) claim(ctx context.Context) (job, error) {
	var j job
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return j, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `SELECT id,alert_id,revision_id,subscription_id,(SELECT count(*) FROM delivery_attempts WHERE job_id=dj.id) FROM delivery_jobs dj WHERE status='QUEUED' AND scheduled_at<=NOW() AND is_test=$1 ORDER BY scheduled_at LIMIT 1 FOR UPDATE SKIP LOCKED`, d.isTest).Scan(&j.ID, &j.AlertID, &j.RevisionID, &j.SubscriptionID, &j.Attempt)
	if err != nil {
		return j, err
	}
	j.Attempt++
	if j.Attempt > 3 {
		_, err = tx.Exec(ctx, `UPDATE delivery_jobs SET status='FAILED' WHERE id=$1`, j.ID)
		if err != nil {
			return j, err
		}
		return j, tx.Commit(ctx)
	}
	_, err = tx.Exec(ctx, `UPDATE delivery_jobs SET status='PROCESSING',processing_started_at=NOW() WHERE id=$1`, j.ID)
	if err == nil {
		err = tx.QueryRow(ctx, `INSERT INTO delivery_attempts(job_id,attempt_number) VALUES($1,$2) RETURNING id`, j.ID, j.Attempt).Scan(&j.AttemptID)
	}
	if err != nil {
		return j, err
	}
	return j, tx.Commit(ctx)
}
func (d *Dispatcher) dispatch(ctx context.Context, j job) error {
	if j.Attempt > 3 {
		return nil
	}
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Lock order: alert, job, subscription. Withdrawal takes the same alert lock.
	// The bounded provider call holds these locks: an already in-flight send can finish
	// before withdrawal returns, but no new send starts after withdrawal commits.
	var alertStatus, activeRevision, areaID, revStatus string
	var alertTest, approved, areaActive bool
	var expires time.Time
	err = tx.QueryRow(ctx, `SELECT a.status,COALESCE(a.active_revision_id::text,''),a.is_test,ar.expiry_at,COALESCE(ar.area_id,''),ar.status,EXISTS(SELECT 1 FROM alert_approvals WHERE revision_id=ar.id AND decision='approve'),COALESCE(area.active,FALSE) FROM alerts a JOIN alert_revisions ar ON ar.id=$2 AND ar.alert_id=a.id LEFT JOIN alert_areas area ON area.id=ar.area_id WHERE a.id=$1 FOR UPDATE OF a`, j.AlertID, j.RevisionID).Scan(&alertStatus, &activeRevision, &alertTest, &expires, &areaID, &revStatus, &approved, &areaActive)
	if err != nil {
		return err
	}
	var jobStatus string
	if err = tx.QueryRow(ctx, `SELECT status FROM delivery_jobs WHERE id=$1 FOR UPDATE`, j.ID).Scan(&jobStatus); err != nil {
		return err
	}
	if jobStatus != "PROCESSING" {
		return nil
	}
	var sub Subscription
	var subArea string
	var subTest, revoked bool
	err = tx.QueryRow(ctx, `SELECT endpoint,COALESCE(auth,''),COALESCE(p256dh,''),COALESCE(area_id,''),is_test,revoked_at IS NOT NULL FROM area_subscriptions WHERE id=$1 FOR UPDATE`, j.SubscriptionID).Scan(&sub.Endpoint, &sub.Auth, &sub.P256DH, &subArea, &subTest, &revoked)
	if err != nil {
		return err
	}
	status, accepted := "CANCELLED", false
	if alertStatus == "ACTIVE" && activeRevision == j.RevisionID && revStatus == "APPROVED" && approved && areaActive && areaID != "" && areaID == subArea && expires.After(time.Now()) && !revoked && subTest == d.isTest && alertTest == d.isTest {
		payload := NotificationPayload{Title: "Savera Alert", Body: "An approved missing-child alert is active in your chosen area. Open for current status.", IsTest: d.isTest, AlertID: j.AlertID, RevisionID: j.RevisionID, StatusURL: "/alerts/" + j.AlertID, ExpiresAt: expires}
		sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		status, accepted, err = d.provider.Send(sendCtx, sub, payload)
		cancel()
		if err != nil {
			status = "AMBIGUOUS"
			accepted = false
		}
	}
	final := outcome(status, accepted, j.Attempt)
	_, err = tx.Exec(ctx, `UPDATE delivery_attempts SET finished_at=NOW(),provider_status=$1,is_provider_accepted=$2 WHERE id=$3`, status, accepted, j.AttemptID)
	if err == nil {
		_, err = tx.Exec(ctx, `UPDATE delivery_jobs SET status=$1,scheduled_at=NOW()+($2 * INTERVAL '30 seconds') WHERE id=$3`, final, j.Attempt*j.Attempt, j.ID)
	}
	if err == nil && (status == "HTTP_404" || status == "HTTP_410") {
		_, err = tx.Exec(ctx, `UPDATE area_subscriptions SET revoked_at=COALESCE(revoked_at,NOW()) WHERE id=$1`, j.SubscriptionID)
	}
	if err != nil {
		return fmt.Errorf("record push outcome: %w", err)
	}
	return tx.Commit(ctx)
}
func outcome(status string, accepted bool, attempt int) string {
	if accepted {
		return "SENT"
	}
	if status == "AMBIGUOUS" {
		return "AMBIGUOUS"
	}
	if status == "CANCELLED" || status == "EXPIRED" {
		return status
	}
	if attempt < 3 && (status == "HTTP_429" || status == "HTTP_500" || status == "HTTP_502" || status == "HTTP_503" || status == "HTTP_504") {
		return "QUEUED"
	}
	return "FAILED"
}
