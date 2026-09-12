// Package integration — P2 gate tests.
//
// Tests (T-017, T-018, T-019, T-020) verify:
//   - T-017: Bounded AI — human_help_available is always true, fallback operates gracefully
//   - T-018: No certainty score or guilt verdict in the actual HTTP response body
//   - T-019: Incognito session revocation (DELETE /sessions/{id}) — session revoked, case preserved
//   - T-020: Cache-Control: no-store on actual HTTP responses for private endpoints
package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// TestT017_BoundedAIOptional verifies that the real assessments endpoint
// always returns human_help_available=true and never panics or errors.
func TestT017_BoundedAIOptional(t *testing.T) {
	env := newTestEnv(t)

	sessionToken := setupTestSession(t, env)

	resp := postJSON(t, urlf(env.server.URL, "/api/v1/assessments"), map[string]interface{}{
		"selected_text": "Someone asked me to send pictures online and threatened to tell my school.",
		"language":      "en",
	}, map[string]string{
		"Authorization": "Bearer " + sessionToken,
	})

	if resp.StatusCode != http.StatusOK {
		body := parseBody(t, resp)
		t.Fatalf("T-017: expected 200, got %d — body: %v", resp.StatusCode, body)
	}

	body := parseBody(t, resp)

	// human_help_available must always be true — this is a hard invariant
	humanHelp, _ := body["human_help_available"].(bool)
	if !humanHelp {
		t.Errorf("T-017 VIOLATION: human_help_available is not true in response: %v", body)
	}

	// Must include a provider field indicating stub or gemini
	if _, ok := body["provider"]; !ok {
		t.Errorf("T-017: response missing 'provider' field — schema issue: %v", body)
	}

	t.Logf("T-017 PASS: AI assessment returned human_help_available=true, provider=%v", body["provider"])
}

// TestT018_NoCertaintyScore verifies the ACTUAL HTTP response from the assessments endpoint
// contains no forbidden fields (certainty_score, confidence, probability, guilt_verdict).
func TestT018_NoCertaintyScore(t *testing.T) {
	env := newTestEnv(t)

	sessionToken := setupTestSession(t, env)

	resp := postJSON(t, urlf(env.server.URL, "/api/v1/assessments"), map[string]interface{}{
		"selected_text": "My uncle keeps asking me to keep secrets and gets angry when I say no.",
		"language":      "en",
	}, map[string]string{
		"Authorization": "Bearer " + sessionToken,
	})

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("T-018: expected 200, got %d", resp.StatusCode)
	}

	body := parseBody(t, resp)

	// These fields are absolutely forbidden — they imply a guilt verdict or probabilistic scoring
	forbiddenFields := []string{
		"certainty_score", "confidence", "probability",
		"guilt_verdict", "likelihood", "risk_score",
	}
	for _, field := range forbiddenFields {
		if _, exists := body[field]; exists {
			t.Errorf("T-018 VIOLATION: forbidden field %q present in actual API response", field)
		}
	}

	t.Logf("T-018 PASS: actual response contains no forbidden certainty/guilt fields. Keys: %v",
		func() []string {
			keys := make([]string, 0, len(body))
			for k := range body {
				keys = append(keys, k)
			}
			return keys
		}())
}

// TestT019_IncognitoSessionRevocation verifies:
//  1. DELETE /sessions/{id} returns 204.
//  2. A subsequent GET using the old token returns 401.
//  3. The case record that was created during the session still exists in the DB.
func TestT019_IncognitoSessionRevocation(t *testing.T) {
	env := newTestEnv(t)

	// Create a session with a case
	sessionID := uuid.New().String()
	tokenJTI := uuid.New().String()
	orgID := "00000000-0000-0000-0000-000000000001" // demo org from seed
	caseID := uuid.New().String()

	_, err := env.pool.Exec(context.Background(), `
		INSERT INTO private_sessions (id, mode, language, token_jti, expires_at)
		VALUES ($1, 'one_time', 'en', $2, NOW() + INTERVAL '15 minutes')
	`, sessionID, tokenJTI)
	if err != nil {
		t.Fatalf("T-019: seed session failed: %v", err)
	}

	_, err = env.pool.Exec(context.Background(), `
		INSERT INTO cases (id, organization_id, status, route_type, account_text, idempotency_key, version)
		VALUES ($1, $2, 'RECEIVED', 'ask_for_help', '[SYNTHETIC TEST DATA]', $3, 1)
	`, caseID, orgID, uuid.New().String())
	if err != nil {
		t.Fatalf("T-019: seed case failed: %v", err)
	}

	sessionToken := makeSessionToken(t, env.cfg, sessionID, caseID)
	authH := map[string]string{"Authorization": "Bearer " + sessionToken}

	// DELETE the session — revoke it
	req, _ := newDeleteRequest(env.server.URL+"/api/v1/sessions/"+sessionID, authH)
	delResp, doErr := http.DefaultClient.Do(req)
	if doErr != nil {
		t.Fatalf("T-019: DELETE request failed: %v", doErr)
	}
	if delResp.StatusCode != http.StatusNoContent && delResp.StatusCode != http.StatusOK {
		t.Errorf("T-019: DELETE /sessions expected 204, got %d", delResp.StatusCode)
	}

	// GET with the old token must now 401
	afterResp := getJSON(t, urlf(env.server.URL, "/api/v1/cases/%s/messages", caseID), authH)
	if afterResp.StatusCode != http.StatusUnauthorized && afterResp.StatusCode != http.StatusForbidden {
		t.Errorf("T-019: expected 401 after session delete, got %d", afterResp.StatusCode)
	}

	// Case must still exist in DB (Incognito guarantee: data durability)
	assertDBCount(t, env.pool,
		`SELECT COUNT(*) FROM cases WHERE id = $1`, []interface{}{caseID}, 1)

	t.Logf("T-019 PASS: session %s revoked (got %d), old token rejected, case %s still in DB",
		sessionID, delResp.StatusCode, caseID)
}

// TestT020_CacheControlNoStore verifies that real HTTP responses from private endpoints
// carry Cache-Control: no-store — NOT a fake recorder assertion.
func TestT020_CacheControlNoStore(t *testing.T) {
	env := newTestEnv(t)

	sessionID := uuid.New().String()
	tokenJTI := uuid.New().String()
	orgID := "00000000-0000-0000-0000-000000000001" // demo org from seed
	caseID := uuid.New().String()

	_, err := env.pool.Exec(context.Background(), `
		INSERT INTO private_sessions (id, mode, language, token_jti, expires_at)
		VALUES ($1, 'one_time', 'en', $2, NOW() + INTERVAL '15 minutes')
	`, sessionID, tokenJTI)
	if err != nil {
		t.Fatalf("T-020: seed session failed: %v", err)
	}
	_, err = env.pool.Exec(context.Background(), `
		INSERT INTO cases (id, organization_id, status, route_type, account_text, idempotency_key, version)
		VALUES ($1, $2, 'RECEIVED', 'ask_for_help', '[SYNTHETIC TEST DATA]', $3, 1)
	`, caseID, orgID, uuid.New().String())
	if err != nil {
		t.Fatalf("T-020: seed case failed: %v", err)
	}

	sessionToken := makeSessionToken(t, env.cfg, sessionID, caseID)

	// T-020: These endpoints must return Cache-Control: no-store
	privateEndpoints := []string{
		urlf(env.server.URL, "/api/v1/cases/%s/messages", caseID),
	}

	for _, endpoint := range privateEndpoints {
		resp := getJSON(t, endpoint, map[string]string{"Authorization": "Bearer " + sessionToken})
		cc := resp.Header.Get("Cache-Control")
		if cc != "no-store" {
			t.Errorf("T-020: endpoint %s — expected Cache-Control: no-store, got %q", endpoint, cc)
		} else {
			t.Logf("T-020: %s — Cache-Control: no-store ✓", endpoint)
		}
		resp.Body.Close()
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func newDeleteRequest(url string, headers map[string]string) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req, nil
}
