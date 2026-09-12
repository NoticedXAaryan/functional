package main

import (
	"context"
	"github.com/balsuraksha/worker/internal/delivery"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"sync"
	"testing"
)

type observedProvider struct {
	mu     sync.Mutex
	target string
	calls  int
}

func (p *observedProvider) Name() string { return "test-observer" }
func (p *observedProvider) Send(_ context.Context, _ delivery.Subscription, n delivery.NotificationPayload) (string, bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if n.AlertID == p.target {
		p.calls++
	}
	return "HTTP_201", true, nil
}
func TestDatabaseOutboxDispatchAndCrashRecovery(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL absent: worker database test not run")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	var id, revision, caseID, subID, outboxID string
	if err = pool.QueryRow(ctx, `SELECT uuid_generate_v4(),uuid_generate_v4(),uuid_generate_v4(),uuid_generate_v4(),uuid_generate_v4()`).Scan(&id, &revision, &caseID, &subID, &outboxID); err != nil {
		t.Fatal(err)
	}
	area := "test-" + id
	exec(`INSERT INTO alert_areas(id,name,is_test) VALUES($1,'FICTIONAL worker test',TRUE)`, area)
	exec(`INSERT INTO cases(id,organization_id,idempotency_key,route_type,account_text,receipt_id) VALUES($1,'00000000-0000-0000-0000-000000000001',uuid_generate_v4(),'missing_child','FICTIONAL worker test',uuid_generate_v4())`, caseID)
	exec(`INSERT INTO alerts(id,case_id,organization_id,status,is_test) VALUES($1,$2,'00000000-0000-0000-0000-000000000001','APPROVED',TRUE)`, id, caseID)
	exec(`INSERT INTO alert_revisions(id,alert_id,revision_number,status,description_text,issuer_name,expiry_at,tip_route_email,prepared_by,area_id,verification_reference) VALUES($1,$2,1,'APPROVED','FICTIONAL test','FICTIONAL team',NOW()+INTERVAL '1 hour','private-in-app','00000000-0000-0000-0000-000000000022',$3,'test fixture')`, revision, id, area)
	exec(`INSERT INTO alert_approvals(alert_id,revision_id,approver_id,decision) VALUES($1,$2,'00000000-0000-0000-0000-000000000023','approve')`, id, revision)
	exec(`UPDATE alerts SET status='ACTIVE',active_revision_id=$1 WHERE id=$2`, revision, id)
	exec(`INSERT INTO area_subscriptions(id,area_name,area_id,endpoint,auth,p256dh,consent_confirmed_at,is_test,management_secret_hash) VALUES($1,'FICTIONAL test',$2,$3,'test','test',NOW(),TRUE,'test')`, subID, area, "https://fcm.googleapis.com/fixture-"+id)
	exec(`INSERT INTO area_subscriptions(area_name,area_id,endpoint,auth,p256dh,consent_confirmed_at,is_test,management_secret_hash) VALUES('FICTIONAL live control',$1,$2,'test','test',NOW(),FALSE,'test')`, area, "https://fcm.googleapis.com/live-control-"+id)
	exec(`INSERT INTO outbox_events(id,event_type,alert_id,revision_id) VALUES($1,'alert_activated',$2,$3)`, outboxID, id, revision)
	// Cleanup only test enrollment. Keep synthetic audit/delivery evidence inspectable.
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `UPDATE area_subscriptions SET revoked_at=NOW() WHERE area_id=$1`, area) })
	var wg sync.WaitGroup
	errors := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errors <- processOutbox(ctx, pool, "demo", "TEST_ALLOWLIST") }()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM delivery_jobs WHERE revision_id=$1`, revision).Scan(&count); err != nil || count != 1 {
		t.Fatalf("area/test isolation or dedupe failed: count=%d err=%v", count, err)
	}
	provider := &observedProvider{target: id}
	dispatcher := delivery.NewDispatcher(pool, provider, true)
	if err = dispatcher.ProcessJobs(ctx); err != nil {
		t.Fatal(err)
	}
	if err = dispatcher.ProcessJobs(ctx); err != nil {
		t.Fatal(err)
	}
	if provider.calls != 1 {
		t.Fatalf("logical notification sent %d times", provider.calls)
	}
	var status string
	if err = pool.QueryRow(ctx, `SELECT status FROM delivery_jobs WHERE revision_id=$1`, revision).Scan(&status); err != nil || status != "SENT" {
		t.Fatalf("provider acceptance not recorded: %s %v", status, err)
	}
	exec(`UPDATE delivery_jobs SET status='PROCESSING',processing_started_at=NOW()-INTERVAL '3 minutes' WHERE revision_id=$1`, revision)
	if err = dispatcher.ProcessJobs(ctx); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT status FROM delivery_jobs WHERE revision_id=$1`, revision).Scan(&status); err != nil || status != "AMBIGUOUS" || provider.calls != 1 {
		t.Fatalf("uncertain crash was replayed: %s, %d calls, %v", status, provider.calls, err)
	}
	exec(`UPDATE delivery_jobs SET status='QUEUED' WHERE revision_id=$1`, revision)
	exec(`UPDATE area_subscriptions SET revoked_at=NOW() WHERE id=$1`, subID)
	if err = dispatcher.ProcessJobs(ctx); err != nil {
		t.Fatal(err)
	}
	if provider.calls != 1 {
		t.Fatal("revoked subscriber was sent another notification")
	}
}
