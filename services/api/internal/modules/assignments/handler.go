// Package assignments handles case assignment, explicit acceptance, reassignment,
// and referrals between organizations.
//
// Critical rules (from delivery plan WP-004 and T-012/T-013/T-016):
//   - Assignment does NOT imply acceptance. accepted_at is only set by explicit responder action.
//   - Only one active (non-unassigned) assignment per case at a time.
//   - Concurrent acceptance attempts are resolved by optimistic locking on assignment.version.
//   - Unacknowledged referrals are NOT completed handoffs.
//   - Supervisors and admins can assign and reassign. Responders can only accept their own assignment.
package assignments

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"

	"github.com/balsuraksha/api/internal/config"
	"github.com/balsuraksha/api/internal/db"
	authMW "github.com/balsuraksha/api/internal/middleware"
)

// Handler handles staff-authenticated assignment and referral routes.
// Mounted at /api/v1/staff/assignments.
type Handler struct {
	pool *db.Pool
	cfg  *config.Config
}

// NewHandler creates an assignment Handler.
func NewHandler(pool *db.Pool, cfg *config.Config) *Handler {
	return &Handler{pool: pool, cfg: cfg}
}

// Routes registers assignment routes.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()

	// Assignments
	// GET  /assignments?case_id=<uuid>               — list assignments for a case
	// POST /assignments                               — create assignment (supervisor/admin only)
	// POST /assignments/{assignmentID}/accept         — explicit acceptance (assigned responder only)
	// POST /assignments/{assignmentID}/unassign       — close/reassign (supervisor/admin only)

	// Referrals
	// POST /assignments/referrals                     — create referral to another org (supervisor/admin)
	// POST /assignments/referrals/{referralID}/acknowledge — receiving-org acceptance (supervisor/admin)
	// GET  /assignments/referrals?case_id=<uuid>      — list referrals for a case

	r.Get("/", h.HandleList)
	r.Post("/", h.HandleCreate)
	r.Post("/{assignmentID}/accept", h.HandleAccept)
	r.Post("/{assignmentID}/unassign", h.HandleUnassign)

	r.Post("/referrals", h.HandleCreateReferral)
	r.Post("/referrals/{referralID}/acknowledge", h.HandleAcknowledgeReferral)
	r.Get("/referrals", h.HandleListReferrals)

	return r
}

// ── Assignment endpoints ──────────────────────────────────────────────────────

// HandleList returns all assignments for a given case, scoped to the staff member's org.
func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	claims := authMW.GetStaffClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Staff authorization required")
		return
	}

	caseIDStr := r.URL.Query().Get("case_id")
	if caseIDStr == "" {
		writeError(w, http.StatusBadRequest, "missing_case_id", "case_id query parameter is required")
		return
	}

	if denied := h.enforceCaseOrgScope(w, r, caseIDStr, claims); denied {
		return
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT id, case_id, responder_id, assigned_by, assigned_at,
		       accepted_at, escalation_due_at, unassigned_at, version
		FROM assignments
		WHERE case_id = $1
		ORDER BY assigned_at ASC
	`, caseIDStr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "")
		return
	}
	defer rows.Close()

	type assignmentRow struct {
		ID               string     `json:"id"`
		CaseID           string     `json:"case_id"`
		ResponderID      string     `json:"responder_id"`
		AssignedBy       string     `json:"assigned_by"`
		AssignedAt       time.Time  `json:"assigned_at"`
		AcceptedAt       *time.Time `json:"accepted_at,omitempty"`
		EscalationDueAt  *time.Time `json:"escalation_due_at,omitempty"`
		UnassignedAt     *time.Time `json:"unassigned_at,omitempty"`
		Version          int        `json:"version"`
	}

	var assignments []assignmentRow
	for rows.Next() {
		var a assignmentRow
		if err := rows.Scan(
			&a.ID, &a.CaseID, &a.ResponderID, &a.AssignedBy, &a.AssignedAt,
			&a.AcceptedAt, &a.EscalationDueAt, &a.UnassignedAt, &a.Version,
		); err != nil {
			log.Error().Err(err).Msg("failed to scan assignment row")
			continue
		}
		assignments = append(assignments, a)
	}
	if assignments == nil {
		assignments = []assignmentRow{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"case_id":     caseIDStr,
		"assignments": assignments,
		"total":       len(assignments),
	})
}

// HandleCreate assigns a case to a responder.
// Only supervisors and admins may assign. Assignment does NOT imply acceptance.
// If an active assignment already exists, it is closed (unassigned_at set) before the new one is created.
func (h *Handler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	claims := authMW.GetStaffClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Staff authorization required")
		return
	}
	if claims.Role != "supervisor" && claims.Role != "admin" {
		writeError(w, http.StatusForbidden, "insufficient_role", "Only supervisors and admins can assign cases")
		return
	}

	var req struct {
		CaseID      string `json:"case_id"`
		ResponderID string `json:"responder_id"`
		// EscalationMinutes defaults to 60 if not provided.
		EscalationMinutes *int `json:"escalation_minutes,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}
	if req.CaseID == "" {
		writeError(w, http.StatusBadRequest, "missing_case_id", "case_id is required")
		return
	}
	if req.ResponderID == "" {
		writeError(w, http.StatusBadRequest, "missing_responder_id", "responder_id is required")
		return
	}
	if _, err := uuid.Parse(req.CaseID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_case_id", "case_id must be a valid UUID")
		return
	}
	if _, err := uuid.Parse(req.ResponderID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_responder_id", "responder_id must be a valid UUID")
		return
	}

	// Enforce org-scope on the case
	if denied := h.enforceCaseOrgScope(w, r, req.CaseID, claims); denied {
		return
	}

	// Verify responder belongs to the same organization and is active
	var responderOrgID string
	err := h.pool.QueryRow(r.Context(), `
		SELECT organization_id FROM staff_members WHERE id = $1 AND active = TRUE
	`, req.ResponderID).Scan(&responderOrgID)
	if err == pgx.ErrNoRows {
		writeError(w, http.StatusBadRequest, "responder_not_found",
			"Responder not found or is not an active staff member")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "")
		return
	}
	if responderOrgID != claims.OrganizationID {
		writeError(w, http.StatusForbidden, "cross_org_responder",
			"Responder does not belong to your organization")
		return
	}

	escalationMinutes := 60
	if req.EscalationMinutes != nil && *req.EscalationMinutes > 0 {
		escalationMinutes = *req.EscalationMinutes
	}
	escalationDue := time.Now().Add(time.Duration(escalationMinutes) * time.Minute)
	assignmentID := uuid.New()

	// Use a transaction: close any existing active assignment, insert new one, update case status.
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "Could not start transaction")
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck

	// Close any existing active assignment for this case
	_, err = tx.Exec(r.Context(), `
		UPDATE assignments
		SET unassigned_at = NOW(), version = version + 1
		WHERE case_id = $1 AND unassigned_at IS NULL
	`, req.CaseID)
	if err != nil {
		log.Error().Err(err).Msg("failed to close existing assignment")
		writeError(w, http.StatusInternalServerError, "db_error", "Could not close existing assignment")
		return
	}

	// Insert new assignment
	_, err = tx.Exec(r.Context(), `
		INSERT INTO assignments (id, case_id, responder_id, organization_id, assigned_by, escalation_due_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, assignmentID, req.CaseID, req.ResponderID, claims.OrganizationID, claims.StaffID, escalationDue)
	if err != nil {
		log.Error().Err(err).Msg("failed to insert assignment")
		writeError(w, http.StatusInternalServerError, "db_error", "Could not create assignment")
		return
	}

	// Update case status to ASSIGNED and record the event
	_, err = tx.Exec(r.Context(), `
		UPDATE cases SET status = 'ASSIGNED', updated_at = NOW(), version = version + 1 WHERE id = $1
	`, req.CaseID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "Could not update case status")
		return
	}

	_, _ = tx.Exec(r.Context(), `
		INSERT INTO case_events (case_id, from_status, to_status, actor_type, actor_id, case_version)
		VALUES ($1, 'TRIAGE', 'ASSIGNED', 'staff', $2, (SELECT version FROM cases WHERE id = $1))
	`, req.CaseID, claims.StaffID)

	if err := tx.Commit(r.Context()); err != nil {
		log.Error().Err(err).Msg("failed to commit assignment transaction")
		writeError(w, http.StatusInternalServerError, "commit_failed", "Assignment could not be confirmed")
		return
	}

	log.Info().
		Str("assignment_id", assignmentID.String()).
		Str("case_id", req.CaseID).
		Str("responder_id", req.ResponderID).
		Str("assigned_by", claims.StaffID).
		Msg("case assigned")

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"assignment_id":    assignmentID.String(),
		"case_id":          req.CaseID,
		"responder_id":     req.ResponderID,
		"escalation_due_at": escalationDue,
		// Explicit reminder: assignment is not acceptance
		"note": "Assignment does not imply acceptance. The responder must explicitly accept.",
	})
}

// HandleAccept records explicit acceptance of an assignment.
// Only the assigned responder may accept their own assignment.
// Uses optimistic locking on assignment.version to prevent concurrent acceptance races.
func (h *Handler) HandleAccept(w http.ResponseWriter, r *http.Request) {
	claims := authMW.GetStaffClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Staff authorization required")
		return
	}

	assignmentIDStr := chi.URLParam(r, "assignmentID")
	assignmentID, err := uuid.Parse(assignmentIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_assignment_id", "Invalid assignment ID")
		return
	}

	// Load the assignment — must be active (not unassigned) and not already accepted
	var caseID, responderID, orgID string
	var currentVersion int
	var acceptedAt *time.Time
	var unassignedAt *time.Time

	err = h.pool.QueryRow(r.Context(), `
		SELECT case_id, responder_id, organization_id, version, accepted_at, unassigned_at
		FROM assignments WHERE id = $1
	`, assignmentID).Scan(&caseID, &responderID, &orgID, &currentVersion, &acceptedAt, &unassignedAt)
	if err == pgx.ErrNoRows {
		writeError(w, http.StatusNotFound, "not_found", "Assignment not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "")
		return
	}

	// Only the assigned responder may accept
	if responderID != claims.StaffID {
		writeError(w, http.StatusForbidden, "not_your_assignment",
			"Only the assigned responder can accept this assignment")
		return
	}
	// Org-scope check
	if orgID != claims.OrganizationID {
		writeError(w, http.StatusForbidden, "forbidden", "Not authorized for this assignment")
		return
	}
	// Already accepted
	if acceptedAt != nil {
		writeError(w, http.StatusConflict, "already_accepted",
			"This assignment has already been accepted")
		return
	}
	// Already unassigned/reassigned
	if unassignedAt != nil {
		writeError(w, http.StatusConflict, "assignment_closed",
			"This assignment has been closed or reassigned. Check for a new active assignment.")
		return
	}

	now := time.Now()

	// Optimistic lock: only update if version hasn't changed since we read it.
	// This prevents a race where two acceptance attempts land simultaneously.
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE assignments
		SET accepted_at = $1, version = version + 1
		WHERE id = $2 AND version = $3 AND accepted_at IS NULL AND unassigned_at IS NULL
	`, now, assignmentID, currentVersion)
	if err != nil {
		log.Error().Err(err).Msg("failed to accept assignment")
		writeError(w, http.StatusInternalServerError, "db_error", "Could not accept assignment")
		return
	}
	if tag.RowsAffected() == 0 {
		// Another concurrent request accepted or modified this assignment first
		writeError(w, http.StatusConflict, "concurrent_modification",
			"Assignment was modified concurrently. Please refresh and try again.")
		return
	}

	// Advance case status to ACCEPTED
	_, _ = h.pool.Exec(r.Context(), `
		UPDATE cases SET status = 'ACCEPTED', updated_at = NOW(), version = version + 1
		WHERE id = $1 AND status = 'ASSIGNED'
	`, caseID)

	_, _ = h.pool.Exec(r.Context(), `
		INSERT INTO case_events (case_id, from_status, to_status, actor_type, actor_id, case_version)
		VALUES ($1, 'ASSIGNED', 'ACCEPTED', 'staff', $2, (SELECT version FROM cases WHERE id = $1))
	`, caseID, claims.StaffID)

	log.Info().
		Str("assignment_id", assignmentIDStr).
		Str("case_id", caseID).
		Str("responder_id", claims.StaffID).
		Msg("assignment accepted")

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"assignment_id": assignmentIDStr,
		"case_id":       caseID,
		"accepted_at":   now,
	})
}

// HandleUnassign closes an active assignment (supervisor/admin only).
// Used for reassignment, escalation handoff, or supervisor override.
// A reason is required to maintain the audit trail.
func (h *Handler) HandleUnassign(w http.ResponseWriter, r *http.Request) {
	claims := authMW.GetStaffClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Staff authorization required")
		return
	}
	if claims.Role != "supervisor" && claims.Role != "admin" {
		writeError(w, http.StatusForbidden, "insufficient_role",
			"Only supervisors and admins can unassign cases")
		return
	}

	assignmentIDStr := chi.URLParam(r, "assignmentID")
	if _, err := uuid.Parse(assignmentIDStr); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_assignment_id", "Invalid assignment ID")
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Reason == "" {
		writeError(w, http.StatusBadRequest, "missing_reason",
			"reason is required to close or reassign an assignment")
		return
	}

	// Load assignment to check org scope
	var caseID, orgID string
	var unassignedAt *time.Time
	err := h.pool.QueryRow(r.Context(), `
		SELECT case_id, organization_id, unassigned_at FROM assignments WHERE id = $1
	`, assignmentIDStr).Scan(&caseID, &orgID, &unassignedAt)
	if err == pgx.ErrNoRows {
		writeError(w, http.StatusNotFound, "not_found", "Assignment not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "")
		return
	}
	if orgID != claims.OrganizationID {
		writeError(w, http.StatusForbidden, "forbidden", "Not authorized for this assignment")
		return
	}
	if unassignedAt != nil {
		writeError(w, http.StatusConflict, "already_unassigned",
			"This assignment is already closed")
		return
	}

	_, err = h.pool.Exec(r.Context(), `
		UPDATE assignments SET unassigned_at = NOW(), version = version + 1 WHERE id = $1
	`, assignmentIDStr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "Could not close assignment")
		return
	}

	// Log the unassign action for audit
	_, _ = h.pool.Exec(r.Context(), `
		INSERT INTO audit_events (event_type, actor_type, actor_id, organization_id, target_type, target_id, metadata)
		VALUES ('assignment_unassigned', 'staff', $1, $2, 'assignment', $3, $4)
	`, claims.StaffID, claims.OrganizationID, assignmentIDStr,
		`{"reason":"`+sanitizeReason(req.Reason)+`"}`)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"assignment_id": assignmentIDStr,
		"case_id":       caseID,
		"unassigned_at": time.Now(),
	})
}

// ── Referral endpoints ────────────────────────────────────────────────────────

// HandleCreateReferral creates a referral from the current org to another organization.
// An unacknowledged referral is NOT a completed handoff (T-016).
func (h *Handler) HandleCreateReferral(w http.ResponseWriter, r *http.Request) {
	claims := authMW.GetStaffClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Staff authorization required")
		return
	}
	if claims.Role != "supervisor" && claims.Role != "admin" {
		writeError(w, http.StatusForbidden, "insufficient_role",
			"Only supervisors and admins can create referrals")
		return
	}

	var req struct {
		CaseID         string `json:"case_id"`
		ToOrganization string `json:"to_organization_id"`
		Notes          string `json:"notes,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}
	if req.CaseID == "" {
		writeError(w, http.StatusBadRequest, "missing_case_id", "case_id is required")
		return
	}
	if req.ToOrganization == "" {
		writeError(w, http.StatusBadRequest, "missing_to_organization_id", "to_organization_id is required")
		return
	}
	if req.ToOrganization == claims.OrganizationID {
		writeError(w, http.StatusBadRequest, "same_org_referral",
			"Cannot refer a case to your own organization")
		return
	}

	if denied := h.enforceCaseOrgScope(w, r, req.CaseID, claims); denied {
		return
	}

	referralID := uuid.New()
	_, err := h.pool.Exec(r.Context(), `
		INSERT INTO referrals (id, case_id, from_organization, to_organization, referred_by, notes)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, referralID, req.CaseID, claims.OrganizationID, req.ToOrganization, claims.StaffID, req.Notes)
	if err != nil {
		log.Error().Err(err).Msg("failed to insert referral")
		writeError(w, http.StatusInternalServerError, "db_error", "Could not create referral")
		return
	}

	log.Info().
		Str("referral_id", referralID.String()).
		Str("case_id", req.CaseID).
		Str("to_org", req.ToOrganization).
		Msg("referral created")

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"referral_id":      referralID.String(),
		"case_id":          req.CaseID,
		"to_organization":  req.ToOrganization,
		"acknowledged_at":  nil,
		// Explicit reminder: referral is not complete until acknowledged
		"note": "Referral is not a completed handoff until explicitly acknowledged by the receiving organization.",
	})
}

// HandleAcknowledgeReferral allows the receiving organization's supervisor to accept a referral.
// Only the to_organization may acknowledge.
func (h *Handler) HandleAcknowledgeReferral(w http.ResponseWriter, r *http.Request) {
	claims := authMW.GetStaffClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Staff authorization required")
		return
	}
	if claims.Role != "supervisor" && claims.Role != "admin" {
		writeError(w, http.StatusForbidden, "insufficient_role",
			"Only supervisors and admins can acknowledge referrals")
		return
	}

	referralIDStr := chi.URLParam(r, "referralID")
	if _, err := uuid.Parse(referralIDStr); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_referral_id", "Invalid referral ID")
		return
	}

	var toOrgID string
	var acknowledgedAt *time.Time
	err := h.pool.QueryRow(r.Context(), `
		SELECT to_organization, acknowledged_at FROM referrals WHERE id = $1
	`, referralIDStr).Scan(&toOrgID, &acknowledgedAt)
	if err == pgx.ErrNoRows {
		writeError(w, http.StatusNotFound, "not_found", "Referral not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "")
		return
	}
	// Only the receiving org may acknowledge
	if toOrgID != claims.OrganizationID {
		writeError(w, http.StatusForbidden, "not_receiving_org",
			"Only the receiving organization can acknowledge this referral")
		return
	}
	if acknowledgedAt != nil {
		writeError(w, http.StatusConflict, "already_acknowledged",
			"This referral has already been acknowledged")
		return
	}

	now := time.Now()
	_, err = h.pool.Exec(r.Context(), `
		UPDATE referrals SET acknowledged_at = $1, acknowledged_by = $2 WHERE id = $3
	`, now, claims.StaffID, referralIDStr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "Could not acknowledge referral")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"referral_id":     referralIDStr,
		"acknowledged_at": now,
		"acknowledged_by": claims.StaffID,
	})
}

// HandleListReferrals returns referrals for a case.
// Returns referrals sent from or received by the staff member's organization.
func (h *Handler) HandleListReferrals(w http.ResponseWriter, r *http.Request) {
	claims := authMW.GetStaffClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Staff authorization required")
		return
	}

	caseIDStr := r.URL.Query().Get("case_id")
	if caseIDStr == "" {
		writeError(w, http.StatusBadRequest, "missing_case_id", "case_id query parameter is required")
		return
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT id, case_id, from_organization, to_organization, referred_by,
		       referred_at, acknowledged_at, acknowledged_by, notes
		FROM referrals
		WHERE case_id = $1
		  AND (from_organization = $2 OR to_organization = $2)
		ORDER BY referred_at ASC
	`, caseIDStr, claims.OrganizationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "")
		return
	}
	defer rows.Close()

	type referralRow struct {
		ID               string     `json:"id"`
		CaseID           string     `json:"case_id"`
		FromOrganization string     `json:"from_organization_id"`
		ToOrganization   string     `json:"to_organization_id"`
		ReferredBy       string     `json:"referred_by"`
		ReferredAt       time.Time  `json:"referred_at"`
		AcknowledgedAt   *time.Time `json:"acknowledged_at,omitempty"`
		AcknowledgedBy   *string    `json:"acknowledged_by,omitempty"`
		Notes            *string    `json:"notes,omitempty"`
	}

	var referrals []referralRow
	for rows.Next() {
		var ref referralRow
		if err := rows.Scan(
			&ref.ID, &ref.CaseID, &ref.FromOrganization, &ref.ToOrganization,
			&ref.ReferredBy, &ref.ReferredAt, &ref.AcknowledgedAt,
			&ref.AcknowledgedBy, &ref.Notes,
		); err != nil {
			log.Error().Err(err).Msg("failed to scan referral row")
			continue
		}
		referrals = append(referrals, ref)
	}
	if referrals == nil {
		referrals = []referralRow{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"case_id":   caseIDStr,
		"referrals": referrals,
		"total":     len(referrals),
	})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// enforceCaseOrgScope checks that caseID belongs to the staff member's organization.
// Returns true (denied) and writes the error response if the check fails.
func (h *Handler) enforceCaseOrgScope(
	w http.ResponseWriter,
	r *http.Request,
	caseID string,
	claims *authMW.StaffClaims,
) (denied bool) {
	var orgID string
	err := h.pool.QueryRow(r.Context(),
		`SELECT organization_id FROM cases WHERE id = $1`, caseID).Scan(&orgID)
	if err == pgx.ErrNoRows {
		writeError(w, http.StatusNotFound, "not_found", "Case not found")
		return true
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "")
		return true
	}
	if orgID != claims.OrganizationID {
		_, _ = h.pool.Exec(r.Context(), `
			INSERT INTO audit_events (event_type, actor_type, actor_id, organization_id, target_type, target_id, metadata)
			VALUES ('cross_org_access_denied', 'staff', $1, $2, 'case', $3, '{"reason":"cross_org_access_denied"}')
		`, claims.StaffID, claims.OrganizationID, caseID)

		writeError(w, http.StatusForbidden, "forbidden",
			"Not authorized to access this case. This access attempt has been logged.")
		return true
	}
	return false
}

// sanitizeReason removes characters that could break the JSON literal in the audit log.
// In production this would use parameterized JSON; this is a demo-safe approximation.
func sanitizeReason(s string) string {
	out := make([]byte, 0, len(s))
	for _, b := range []byte(s) {
		if b == '"' || b == '\\' || b == '\n' || b == '\r' {
			out = append(out, ' ')
		} else {
			out = append(out, b)
		}
	}
	return string(out)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg, "code": code})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
