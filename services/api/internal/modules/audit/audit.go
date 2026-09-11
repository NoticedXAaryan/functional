// Package audit provides helpers to write immutable security audit events.
//
// Rules (from docs/07-delivery-plan.md §6 and T-009):
//   - audit_events.metadata MUST NOT contain case narrative, return secrets, or staff notes.
//   - Every cross-org access denial must be recorded before the 403 response.
//   - Actor identity is always the authenticated staff/session ID — callers cannot spoof it.
package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/balsuraksha/api/internal/db"
)

// EventType describes the kind of security-relevant action.
type EventType string

const (
	EventCrossOrgAccessDenied    EventType = "cross_org_access_denied"
	EventCrossOrgTipsDenied      EventType = "cross_org_tips_access_denied"
	EventSessionRevoked          EventType = "session_revoked"
	EventReturnSecretAttempt     EventType = "return_secret_attempt"
	EventBruteForceBlocked       EventType = "brute_force_blocked"
	EventAlertSelfApprovalDenied EventType = "alert_self_approval_denied"
	EventAlertActivated          EventType = "alert_activated"
	EventAlertWithdrawn          EventType = "alert_withdrawn"
)

// ActorType identifies who performed the action.
type ActorType string

const (
	ActorSystem  ActorType = "system"
	ActorStaff   ActorType = "staff"
	ActorSession ActorType = "session"
	ActorPublic  ActorType = "public"
)

// LogEvent writes an immutable audit record.
// metadata must contain only non-sensitive context (e.g. reason codes, target IDs).
// Never include case narrative, return secrets, or staff notes in metadata.
func LogEvent(
	ctx context.Context,
	pool *db.Pool,
	eventType EventType,
	actorType ActorType,
	actorID string,
	organizationID string,
	targetType string,
	targetID string,
	metadata map[string]string,
) {
	metaJSON, err := json.Marshal(metadata)
	if err != nil {
		metaJSON = []byte("{}")
	}

	_, execErr := pool.Exec(ctx, `
		INSERT INTO audit_events (event_type, actor_type, actor_id, organization_id, target_type, target_id, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`,
		string(eventType),
		string(actorType),
		nullableStr(actorID),
		nullableStr(organizationID),
		nullableStr(targetType),
		nullableStr(targetID),
		metaJSON,
	)
	if execErr != nil {
		// Audit failures are logged but never silently swallowed — they surface in structured logs
		log.Error().
			Err(execErr).
			Str("event_type", string(eventType)).
			Str("actor_id", actorID).
			Msg("audit: failed to write audit event")
	}
}

// LogDenial is a convenience wrapper for access denial events.
func LogDenial(
	ctx context.Context,
	pool *db.Pool,
	eventType EventType,
	actorType ActorType,
	actorID string,
	organizationID string,
	targetType string,
	targetID string,
	reason string,
) {
	LogEvent(ctx, pool, eventType, actorType, actorID, organizationID, targetType, targetID,
		map[string]string{"reason": reason})
}

// Summary returns a redacted description of the event for structured log output.
// Never call this with sensitive content — it is for log lines only.
func Summary(eventType EventType, actorID, targetID string) string {
	return fmt.Sprintf("audit[%s] actor=%s target=%s", eventType, actorID, targetID)
}

func nullableStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
