// Package messages handles case messaging between reporters and responders.
//
// Authorization rules:
//   - Session-authenticated reporters can only read/send on their own case (non-staff-note only).
//   - Staff can read all messages (including staff notes) for cases in their organization.
//   - Staff notes (is_staff_note=true) are NEVER exposed through the reporter-facing endpoints.
//   - Cross-org access is denied and audited.
package messages

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

// ── Staff handler ─────────────────────────────────────────────────────────────

// Handler handles staff-authenticated message routes.
// Mounted at /api/v1/staff/messages.
type Handler struct {
	pool *db.Pool
	cfg  *config.Config
}

// NewHandler creates a staff message Handler.
func NewHandler(pool *db.Pool, cfg *config.Config) *Handler {
	return &Handler{pool: pool, cfg: cfg}
}

// Routes registers staff message routes.
// Staff can list all messages (including staff notes) and send responder/system messages.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	// GET  /staff/messages?case_id=<uuid>   — list all messages (including staff notes)
	// POST /staff/messages                  — send a responder message or staff note
	r.Get("/", h.HandleList)
	r.Post("/", h.HandleSend)
	return r
}

// messageRow is the shared message representation returned to staff.
type messageRow struct {
	ID          string    `json:"id"`
	CaseID      string    `json:"case_id"`
	SenderType  string    `json:"sender_type"`
	SenderID    *string   `json:"sender_id,omitempty"`
	Body        string    `json:"body"`
	IsStaffNote bool      `json:"is_staff_note"`
	SentAt      time.Time `json:"sent_at"`
}

// HandleList returns all messages for a given case, scoped to the staff member's org.
// Staff notes are included. Returns 400 if case_id query param is missing.
// Returns 403 if the case belongs to a different organization (and audits the attempt).
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
	if _, err := uuid.Parse(caseIDStr); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_case_id", "case_id must be a valid UUID")
		return
	}

	// Enforce org-scope before returning any message content
	if denied := h.enforceCaseOrgScope(w, r, caseIDStr, claims); denied {
		return
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT id, case_id, sender_type, sender_id, body, is_staff_note, sent_at
		FROM case_messages
		WHERE case_id = $1
		ORDER BY sent_at ASC
	`, caseIDStr)
	if err != nil {
		log.Error().Err(err).Str("case_id", caseIDStr).Msg("failed to query messages")
		writeError(w, http.StatusInternalServerError, "db_error", "Could not retrieve messages")
		return
	}
	defer rows.Close()

	var msgs []messageRow
	for rows.Next() {
		var m messageRow
		if err := rows.Scan(
			&m.ID, &m.CaseID, &m.SenderType, &m.SenderID,
			&m.Body, &m.IsStaffNote, &m.SentAt,
		); err != nil {
			log.Error().Err(err).Msg("failed to scan message row")
			continue
		}
		msgs = append(msgs, m)
	}
	if msgs == nil {
		msgs = []messageRow{}
	}

	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"case_id":  caseIDStr,
		"messages": msgs,
		"total":    len(msgs),
	})
}

// HandleSend sends a message on a case (responder message or staff note).
// Only the assigned/accepting staff member's org may send.
// The sender_id is always set to the authenticated staff member — callers cannot spoof it.
func (h *Handler) HandleSend(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16000)
	claims := authMW.GetStaffClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Staff authorization required")
		return
	}

	var req struct {
		CaseID      string `json:"case_id"`
		Body        string `json:"body"`
		IsStaffNote bool   `json:"is_staff_note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}
	if req.CaseID == "" {
		writeError(w, http.StatusBadRequest, "missing_case_id", "case_id is required")
		return
	}
	if _, err := uuid.Parse(req.CaseID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_case_id", "case_id must be a valid UUID")
		return
	}
	if req.Body == "" {
		writeError(w, http.StatusBadRequest, "missing_body", "body is required")
		return
	}
	if len(req.Body) > 10000 {
		writeError(w, http.StatusBadRequest, "body_too_long", "body must be 10,000 characters or fewer")
		return
	}

	// Enforce org-scope before inserting
	if denied := h.enforceCaseOrgScope(w, r, req.CaseID, claims); denied {
		return
	}

	msgID := uuid.New()
	senderID := claims.StaffID

	result, err := h.pool.Exec(r.Context(), `
		INSERT INTO case_messages (id, case_id, sender_type, sender_id, body, is_staff_note)
		SELECT $1, c.id, 'responder', $3, $4, $5 FROM cases c WHERE c.id=$2 AND c.status<>'CLOSED' AND ($5 OR EXISTS(SELECT 1 FROM contact_preferences cp WHERE cp.case_id=c.id AND cp.preferred_channel='message_in_app' AND (cp.safe_hours_start=cp.safe_hours_end OR (cp.safe_hours_start<cp.safe_hours_end AND (NOW() AT TIME ZONE cp.time_zone)::time >= cp.safe_hours_start AND (NOW() AT TIME ZONE cp.time_zone)::time < cp.safe_hours_end) OR (cp.safe_hours_start>cp.safe_hours_end AND ((NOW() AT TIME ZONE cp.time_zone)::time >= cp.safe_hours_start OR (NOW() AT TIME ZONE cp.time_zone)::time < cp.safe_hours_end)))))
	`, msgID, req.CaseID, senderID, req.Body, req.IsStaffNote)
	if err != nil {
		log.Error().Err(err).Str("case_id", req.CaseID).Msg("failed to insert message")
		writeError(w, http.StatusInternalServerError, "db_error", "Could not send message")
		return
	}

	// Advance case status to IN_PROGRESS on first non-note responder message,
	if result.RowsAffected() != 1 {
		writeError(w, 409, "contact_not_allowed", "Message not sent: the report is closed, contact is disabled, or it is outside safe contact hours")
		return
	}
	// if it was in ACCEPTED state. (Best-effort; does not fail the send.)
	if !req.IsStaffNote {
		_, _ = h.pool.Exec(r.Context(), `
			UPDATE cases
			SET status = 'IN_PROGRESS', updated_at = NOW(), version = version + 1
			WHERE id = $1 AND status = 'ACCEPTED'
		`, req.CaseID)
	}

	log.Info().
		Str("message_id", msgID.String()).
		Str("case_id", req.CaseID).
		Str("staff_id", senderID).
		Bool("is_staff_note", req.IsStaffNote).
		Msg("message sent")

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message_id":    msgID.String(),
		"case_id":       req.CaseID,
		"is_staff_note": req.IsStaffNote,
		"sent_at":       time.Now(),
	})
}

// ── Session handler (reporter-facing) ─────────────────────────────────────────

// SessionHandler handles reporter-facing message routes.
// Mounted alongside the session-authenticated cases routes.
type SessionHandler struct {
	pool *db.Pool
	cfg  *config.Config
}

// NewSessionHandler creates a reporter-facing message SessionHandler.
func NewSessionHandler(pool *db.Pool, cfg *config.Config) *SessionHandler {
	return &SessionHandler{pool: pool, cfg: cfg}
}

// Routes registers session-authenticated message routes.
func (h *SessionHandler) Routes() http.Handler {
	r := chi.NewRouter()
	// POST /cases/{caseID}/messages   — reporter sends a message on their case
	// GET  /cases/{caseID}/messages   — reporter reads non-staff-note messages on their case
	r.Post("/{caseID}/messages", h.HandleSend)
	r.Get("/{caseID}/messages", h.HandleList)
	return r
}

// HandleSend allows a reporter (session-auth) to send a message on their own case.
// Staff notes are never created through this endpoint. sender_type is always 'reporter'.
func (h *SessionHandler) HandleSend(w http.ResponseWriter, r *http.Request) {
	claims := authMW.GetSessionClaims(r.Context())
	if claims == nil || claims.CaseID == "" {
		writeError(w, http.StatusForbidden, "no_case_access",
			"No case access. Use your return code to access your case.")
		return
	}

	caseID := chi.URLParam(r, "caseID")
	if caseID != claims.CaseID {
		writeError(w, http.StatusForbidden, "forbidden", "Not authorized for this case")
		return
	}

	var req struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Body == "" {
		writeError(w, http.StatusBadRequest, "missing_body", "body is required")
		return
	}
	if len(req.Body) > 5000 {
		writeError(w, http.StatusBadRequest, "body_too_long", "body must be 5,000 characters or fewer")
		return
	}

	// Confirm the case is still open (not CLOSED)
	var caseStatus string
	err := h.pool.QueryRow(r.Context(), `SELECT status FROM cases WHERE id = $1`, caseID).
		Scan(&caseStatus)
	if err == pgx.ErrNoRows {
		writeError(w, http.StatusNotFound, "not_found", "Case not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "")
		return
	}
	if caseStatus == "CLOSED" {
		writeError(w, http.StatusConflict, "case_closed",
			"This case is closed. If you need more help, please reach out through a new report.")
		return
	}

	msgID := uuid.New()
	_, err = h.pool.Exec(r.Context(), `
		INSERT INTO case_messages (id, case_id, sender_type, sender_id, body, is_staff_note)
		VALUES ($1, $2, 'reporter', NULL, $3, FALSE)
	`, msgID, caseID, req.Body)
	if err != nil {
		log.Error().Err(err).Str("case_id", caseID).Msg("failed to insert reporter message")
		writeError(w, http.StatusInternalServerError, "db_error", "Could not send message")
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message_id": msgID.String(),
		"sent_at":    time.Now(),
	})
}

// HandleList returns non-staff-note messages for the reporter's own case.
// Staff notes (is_staff_note=true) are never included.
func (h *SessionHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	claims := authMW.GetSessionClaims(r.Context())
	if claims == nil || claims.CaseID == "" {
		writeError(w, http.StatusForbidden, "no_case_access",
			"No case access. Use your return code to access your case.")
		return
	}

	caseID := chi.URLParam(r, "caseID")
	if caseID != claims.CaseID {
		writeError(w, http.StatusForbidden, "forbidden", "Not authorized for this case")
		return
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT id, body, sent_at, sender_type
		FROM case_messages
		WHERE case_id = $1 AND is_staff_note = FALSE
		ORDER BY sent_at ASC
	`, caseID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "")
		return
	}
	defer rows.Close()

	type safeMsg struct {
		ID         string    `json:"id"`
		Body       string    `json:"body"`
		SentAt     time.Time `json:"sent_at"`
		SenderRole string    `json:"sender_role"`
	}
	var msgs []safeMsg
	for rows.Next() {
		var m safeMsg
		var senderType string
		if err := rows.Scan(&m.ID, &m.Body, &m.SentAt, &senderType); err != nil {
			continue
		}
		// Map internal sender_type to safe display role
		switch senderType {
		case "responder":
			m.SenderRole = "responder"
		default:
			m.SenderRole = "reporter"
		}
		msgs = append(msgs, m)
	}
	if msgs == nil {
		msgs = []safeMsg{}
	}

	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"case_id":  caseID,
		"messages": msgs,
		"total":    len(msgs),
	})
}

// ── Shared helpers ────────────────────────────────────────────────────────────

// enforceCaseOrgScope checks that caseID belongs to the staff member's organization.
// If the check fails it writes the error response and returns true (denied).
// Caller must return immediately when denied == true.
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
		// Audit the denied access attempt without leaking case content
		_, _ = h.pool.Exec(r.Context(), `
			INSERT INTO audit_events (event_type, actor_type, actor_id, organization_id, target_type, target_id, metadata)
			VALUES ('cross_org_access_denied', 'staff', $1, $2, 'case_messages', $3, '{"reason":"cross_org_access_denied"}')
		`, claims.StaffID, claims.OrganizationID, caseID)

		writeError(w, http.StatusForbidden, "forbidden",
			"Not authorized to access this case. This access attempt has been logged.")
		return true
	}
	if !authMW.CanReadCase(r.Context(), h.pool, claims, caseID) {
		writeError(w, 403, "case_access_required", "Only assigned staff or a supervisor can open this report")
		return true
	}
	return false
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
