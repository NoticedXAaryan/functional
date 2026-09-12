package integration

import (
	"context"
	"github.com/google/uuid"
	"net/http"
	"testing"
)

func TestAlertTenantAndActivationBoundaries(t *testing.T) {
	t.Setenv("NOTIFICATION_MODE", "TEST_ALLOWLIST")
	env := newTestEnv(t)
	org, other := seedTwoOrgs(t, env)
	preparer, approver, outsider := uuid.NewString(), uuid.NewString(), uuid.NewString()
	seedStaffMember(t, env, preparer, org, "preparer")
	seedStaffMember(t, env, approver, org, "approver")
	seedStaffMember(t, env, outsider, other, "approver")
	id, revision := seedAlertRevision(t, env, org, preparer)
	headers := func(staff, organization, role string) map[string]string {
		return map[string]string{"Authorization": "Bearer " + makeStaffToken(t, env.cfg, staff, organization, role)}
	}
	own := headers(approver, org, "alert_approver")
	foreign := headers(outsider, other, "alert_approver")
	expect := func(response *http.Response, want int) {
		t.Helper()
		body := parseBody(t, response)
		if response.StatusCode != want {
			t.Fatalf("got %d want %d: %v", response.StatusCode, want, body)
		}
	}
	expect(getJSON(t, env.server.URL+"/api/v1/staff/alert-drafts/"+revision, foreign), 404)
	decision := map[string]interface{}{"revision_id": revision, "decision": "approve"}
	expect(postJSON(t, env.server.URL+"/api/v1/staff/alert-approvals", decision, foreign), 404)
	expect(postJSON(t, env.server.URL+"/api/v1/staff/alert-approvals", decision, own), 200)
	activation := map[string]interface{}{"approved_revision_id": revision}
	path := env.server.URL + "/api/v1/staff/alerts/" + id
	expect(postJSON(t, path+"/activate", activation, headers(approver, org, "responder")), 403)
	expect(postJSON(t, path+"/activate", activation, foreign), 404)
	expect(postJSON(t, path+"/activate", activation, own), 200)
	expect(postJSON(t, path+"/activate", activation, own), 200)
	assertDBCount(t, env.pool, `SELECT count(*) FROM outbox_events WHERE alert_id=$1`, []interface{}{id}, 1)
	reason := map[string]interface{}{"reason": "Synthetic test finished"}
	expect(postJSON(t, path+"/withdraw", reason, foreign), 404)
	var status string
	if err := env.pool.QueryRow(context.Background(), `SELECT status FROM alerts WHERE id=$1`, id).Scan(&status); err != nil || status != "ACTIVE" {
		t.Fatal("foreign withdrawal changed alert")
	}
	expect(postJSON(t, path+"/withdraw", reason, own), 200)
	expect(postJSON(t, path+"/activate", activation, own), 409)
	response := getJSON(t, env.server.URL+"/api/v1/public/alerts/"+id, nil)
	body := parseBody(t, response)
	if body["description_text"] != "" || body["tip_route_available"] != false || body["verification_reference"] != nil {
		t.Fatal("terminal public response exposed content or allowed tips")
	}
}
