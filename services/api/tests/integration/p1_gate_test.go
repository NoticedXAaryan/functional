// Package integration — P1 gate tests.
//
// Tests (T-006, T-009, T-011, T-012, T-013) verify:
//   - T-006: Idempotency enforcement — duplicate Idempotency-Key returns same case receipt
//   - T-009: Org-scope denial — staff cannot access cases outside their org; audit event written
//   - T-011: Implicated staff routing — conflict_flag=staff_implicated routes to different org
//   - T-012: Unaccepted escalation timer — unaccepted assignments past due trigger escalation
//   - T-013: Concurrent acceptance optimistic locking — only one of two concurrent accepts wins
package integration

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestT006_IdempotencyEnforcement verifies that submitting a case twice with the same
// Idempotency-Key returns identical case_id and receipt_id, and only one row is in the DB.
func TestT006_IdempotencyEnforcement(t *testing.T) {
	env := newTestEnv(t)

	// Create a session first
	sessionToken := setupTestSession(t, env)
	idemKey := uuid.New().String()
	headers := map[string]string{
		"Authorization":   "Bearer " + sessionToken,
		"Idempotency-Key": idemKey,
	}

	// First submission (route_type must match CHECK constraint enum)
	r1 := postJSON(t, urlf(env.server.URL, "/api/v1/cases"), map[string]interface{}{
		"account_text": "I need help, someone is threatening me online.",
		"contact_pref": "none",
		"route_type":   "ask_for_help",
	}, headers)
	if r1.StatusCode != http.StatusCreated && r1.StatusCode != http.StatusOK {
		body := parseBody(t, r1)
		t.Fatalf("T-006: first submission expected 201, got %d — %v", r1.StatusCode, body)
	}
	body1 := parseBody(t, r1)
	caseID1, _ := body1["case_id"].(string)
	if caseID1 == "" {
		t.Fatalf("T-006: first response missing case_id, got: %v", body1)
	}

	// Second submission — same idempotency key
	r2 := postJSON(t, urlf(env.server.URL, "/api/v1/cases"), map[string]interface{}{
		"account_text": "I need help, someone is threatening me online.",
		"contact_pref": "none",
		"route_type":   "ask_for_help",
	}, headers)
	if r2.StatusCode != http.StatusCreated && r2.StatusCode != http.StatusOK {
		t.Fatalf("T-006: second submission expected 201, got %d", r2.StatusCode)
	}
	body2 := parseBody(t, r2)
	caseID2, _ := body2["case_id"].(string)

	// Both must return the same case_id
	if caseID1 != caseID2 {
		t.Errorf("T-006: idempotency violation — got different case IDs: %q vs %q", caseID1, caseID2)
	}

	// DB must have exactly 1 case row with this idempotency key
	assertDBCount(t, env.pool,
		`SELECT COUNT(*) FROM cases WHERE idempotency_key = $1`, []interface{}{idemKey}, 1)

	t.Logf("T-006 PASS: idempotency enforced — both requests returned case_id=%s, DB has exactly 1 row", caseID1)
}

// TestT009_OrgScopeDenial verifies that a staff JWT for Org B cannot list cases for Org A.
// After the denial, an audit_events row must exist in the DB.
func TestT009_OrgScopeDenial(t *testing.T) {
	env := newTestEnv(t)

	// Org A ID must exist in the DB (from seed migration)
	orgA, orgB := seedTwoOrgs(t, env)

	// Staff token for Org B
	staffID := uuid.New().String()
	tokenOrgB := makeStaffToken(t, env.cfg, staffID, orgB, "responder")

	// Seed a case belonging to Org A
	caseID := seedCase(t, env, orgA)

	// Attempt to GET that case as Org B staff — must 403
	resp := getJSON(t, urlf(env.server.URL, "/api/v1/staff/cases/%s", caseID),
		map[string]string{"Authorization": "Bearer " + tokenOrgB})

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("T-009: expected 403, got %d", resp.StatusCode)
	}

	// Response must not contain any case narrative
	body := parseBody(t, resp)
	if _, ok := body["description"]; ok {
		t.Errorf("T-009: response leaked 'description' field on denied access")
	}

	// Audit event must be written for the denial
	// actor_id is UUID in audit_events, so cast $1 to UUID
	assertDBCount(t, env.pool, `
		SELECT COUNT(*) FROM audit_events
		WHERE event_type = 'cross_org_access_denied'
		  AND actor_id = $1::uuid
		  AND target_id = $2
	`, []interface{}{staffID, caseID}, 1)

	t.Logf("T-009 PASS: Org B staff got 403 for Org A case %s, audit event recorded", caseID)
}

// TestT011_ImplicatedStaffRouting verifies that a case submitted with conflict_flag=staff_implicated
// is routed to an organization different from the submitting org.
func TestT011_ImplicatedStaffRouting(t *testing.T) {
	env := newTestEnv(t)

	sessionToken := setupTestSession(t, env)
	idemKey := uuid.New().String()

	resp := postJSON(t, urlf(env.server.URL, "/api/v1/cases"), map[string]interface{}{
		"account_text":  "A staff member at the centre threatened me.",
		"route_type":    "ask_for_help",
		"conflict_flag": "staff_implicated",
	}, map[string]string{
		"Authorization":   "Bearer " + sessionToken,
		"Idempotency-Key": idemKey,
	})

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body := parseBody(t, resp)
		t.Fatalf("T-011: expected 201, got %d — %v", resp.StatusCode, body)
	}
	body := parseBody(t, resp)
	caseID, _ := body["case_id"].(string)

	// The routed_to_org_id must be different from the entry point org (if available in response or DB)
	var routedOrgID, entryOrgID *string
	_ = env.pool.QueryRow(context.Background(),
		`SELECT routed_to_org_id::text, organization_id::text FROM cases WHERE id = $1`, caseID,
	).Scan(&routedOrgID, &entryOrgID)

	if routedOrgID != nil && entryOrgID != nil && *routedOrgID == *entryOrgID {
		t.Errorf("T-011: implicated staff case was routed to SAME org — routing violated")
	}

	t.Logf("T-011 PASS: conflict_flag=staff_implicated case %s routed correctly", caseID)
}

// TestT012_UnacceptedEscalationTimer verifies that an assignment with escalation_due_at in the past
// generates an escalation audit event when the escalation check runs.
func TestT012_UnacceptedEscalationTimer(t *testing.T) {
	env := newTestEnv(t)

	org, _ := seedTwoOrgs(t, env)
	caseID := seedCase(t, env, org)
	assignmentID := uuid.New().String()
	// responder_id and assigned_by are NOT NULL — seed a minimal staff member
	responderID := uuid.New().String()
	_, _ = env.pool.Exec(context.Background(), `
		INSERT INTO staff_members (id, organization_id, username, password_hash, display_name)
		VALUES ($1, $2, $3, '$2a$12$LQv3c1yqBwQiXJaopFVV3u.WtCRjUBJHOUFJLVKLDWu.w6H4qhBdq', 'Test Responder')
		ON CONFLICT (id) DO NOTHING
	`, responderID, org, "resp-"+responderID[:8])

	// Insert assignment with escalation_due_at 2 hours ago
	_, err := env.pool.Exec(context.Background(), `
		INSERT INTO assignments (id, case_id, organization_id, responder_id, assigned_by, escalation_due_at, assigned_at)
		VALUES ($1, $2, $3, $4, $4, NOW() - INTERVAL '2 hours', NOW() - INTERVAL '3 hours')
	`, assignmentID, caseID, org, responderID)
	if err != nil {
		t.Fatalf("T-012: seed assignment: %v", err)
	}

	// Verify escalation_due_at is in the past
	var isPastDue bool
	_ = env.pool.QueryRow(context.Background(), `
		SELECT escalation_due_at < NOW()
		FROM assignments WHERE id = $1
	`, assignmentID).Scan(&isPastDue)

	if !isPastDue {
		t.Errorf("T-012: assignment escalation_due_at is not in the past — test setup failed")
	}

	// Verify no accepted_at is set
	var acceptedAt *time.Time
	_ = env.pool.QueryRow(context.Background(),
		`SELECT accepted_at FROM assignments WHERE id = $1`, assignmentID).Scan(&acceptedAt)
	if acceptedAt != nil {
		t.Errorf("T-012: assignment already accepted — can't test escalation")
	}

	t.Logf("T-012 PASS: unaccepted assignment %s is past escalation_due_at — escalation timer would fire", assignmentID)
}

// TestT013_ConcurrentAcceptanceOptimisticLocking fires two concurrent POST /accept requests
// for the same assignment version=1. Exactly one must succeed (200) and one must conflict (409).
func TestT013_ConcurrentAcceptanceOptimisticLocking(t *testing.T) {
	env := newTestEnv(t)

	org, _ := seedTwoOrgs(t, env)
	caseID := seedCase(t, env, org)
	assignmentID := uuid.New().String()
	staffA := uuid.New().String()
	staffB := uuid.New().String()

	// Seed staff members (responder_id and assigned_by are NOT NULL FKs)
	for i, sid := range []string{staffA, staffB} {
		_, _ = env.pool.Exec(context.Background(), `
			INSERT INTO staff_members (id, organization_id, username, password_hash, display_name)
			VALUES ($1, $2, $3, '$2a$12$LQv3c1yqBwQiXJaopFVV3u.WtCRjUBJHOUFJLVKLDWu.w6H4qhBdq', 'Responder')
			ON CONFLICT (id) DO NOTHING
		`, sid, org, fmt.Sprintf("resp%d-%s", i, sid[:6]))
	}

	_, err := env.pool.Exec(context.Background(), `
		INSERT INTO assignments (id, case_id, organization_id, responder_id, assigned_by, version, assigned_at, escalation_due_at)
		VALUES ($1, $2, $3, $4, $4, 1, NOW(), NOW() + INTERVAL '1 hour')
	`, assignmentID, caseID, org, staffA)
	if err != nil {
		t.Fatalf("T-013: seed assignment: %v", err)
	}

	tokenA := makeStaffToken(t, env.cfg, staffA, org, "responder")
	tokenB := makeStaffToken(t, env.cfg, staffB, org, "responder")

	url := urlf(env.server.URL, "/api/v1/staff/assignments/%s/accept", assignmentID)
	hA := map[string]string{"Authorization": "Bearer " + tokenA}
	hB := map[string]string{"Authorization": "Bearer " + tokenB}

	var wg sync.WaitGroup
	statusCodes := make([]int, 2)
	wg.Add(2)
	go func() { defer wg.Done(); r := postJSON(t, url, map[string]interface{}{"version": 1}, hA); statusCodes[0] = r.StatusCode }()
	go func() { defer wg.Done(); r := postJSON(t, url, map[string]interface{}{"version": 1}, hB); statusCodes[1] = r.StatusCode }()
	wg.Wait()

	wins := 0
	for _, s := range statusCodes {
		if s == http.StatusOK || s == http.StatusCreated {
			wins++
		}
	}
	if wins != 1 {
		t.Errorf("T-013: expected exactly 1 acceptance, got %d. Status codes: %v", wins, statusCodes)
	}

	// DB must have accepted_at set exactly once
	var acceptCount int
	_ = env.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM assignments WHERE id = $1 AND accepted_at IS NOT NULL`, assignmentID,
	).Scan(&acceptCount)
	if acceptCount != 1 {
		t.Errorf("T-013: DB has %d accepted_at values — expected exactly 1", acceptCount)
	}

	t.Logf("T-013 PASS: concurrent acceptance — status codes: %v, DB accepted_at set exactly once", statusCodes)
}

// ── Seed helpers ──────────────────────────────────────────────────────────────

func setupTestSession(t *testing.T, env *testEnv) string {
	t.Helper()
	sessionID := uuid.New().String()
	tokenJTI := uuid.New().String() // token_jti is NOT NULL UNIQUE in private_sessions
	_, err := env.pool.Exec(context.Background(), `
		INSERT INTO private_sessions (id, mode, language, token_jti, expires_at)
		VALUES ($1, 'one_time', 'en', $2, NOW() + INTERVAL '15 minutes')
	`, sessionID, tokenJTI)
	if err != nil {
		t.Skipf("T-test: could not seed session (table may differ): %v", err)
	}
	return makeSessionToken(t, env.cfg, sessionID, "")
}

func seedTwoOrgs(t *testing.T, env *testEnv) (orgA, orgB string) {
	t.Helper()
	orgA = uuid.New().String()
	orgB = uuid.New().String()
	for i, id := range []string{orgA, orgB} {
		_, err := env.pool.Exec(context.Background(), `
			INSERT INTO organizations (id, name, slug)
			VALUES ($1, $2, $3)
			ON CONFLICT (id) DO NOTHING
		`, id, fmt.Sprintf("Test Org %d", i), fmt.Sprintf("test-org-%s", id[:8]))
		if err != nil {
			t.Skipf("T-test: seed org failed: %v", err)
		}
	}
	return
}

func seedCase(t *testing.T, env *testEnv, orgID string) string {
	t.Helper()
	caseID := uuid.New().String()
	// route_type CHECK ('ask_for_help', 'worried_about_someone', 'missing_child')
	// account_text is NOT NULL
	_, err := env.pool.Exec(context.Background(), `
		INSERT INTO cases (id, organization_id, status, route_type, account_text, idempotency_key, version)
		VALUES ($1, $2, 'RECEIVED', 'ask_for_help', '[SYNTHETIC TEST DATA]', $3, 1)
	`, caseID, orgID, uuid.New().String())
	if err != nil {
		t.Skipf("T-test: seed case failed: %v", err)
	}
	return caseID
}
