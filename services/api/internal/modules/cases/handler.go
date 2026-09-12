// Package cases handles private case creation (child intake) and staff case management.
// Authorization rules:
//   - Session-authenticated users can only access their own case.
//   - Staff can only access cases belonging to their organization.
//   - Cross-org access is denied and audited.
package cases

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

// SessionHandler handles case routes accessible by session-authenticated users (child-facing).
type SessionHandler struct {
	pool *db.Pool
	cfg  *config.Config
}

// NewSessionHandler creates a SessionHandler.
func NewSessionHandler(pool *db.Pool, cfg *config.Config) *SessionHandler {
	return &SessionHandler{pool: pool, cfg: cfg}
}

// Routes registers session-authenticated case routes.
func (h *SessionHandler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Post("/", h.HandleCreate)
	r.Get("/{caseID}/safe-view", h.HandleSafeView)
	return r
}

// HandleCreate processes a case submission.
// Idempotency: the Idempotency-Key header prevents duplicate cases on retry.
// The case is persisted in a single transaction — the receipt is only returned
// after a successful database commit.
func (h *SessionHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	claims := authMW.GetSessionClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Session required")
		return
	}

	// Idempotency key is required
	idempotencyKeyStr := r.Header.Get("Idempotency-Key")
	if idempotencyKeyStr == "" {
		writeError(w, http.StatusBadRequest, "missing_idempotency_key",
			"Idempotency-Key header is required. Generate a UUID client-side and reuse it on retries.")
		return
	}
	idempotencyKey, err := uuid.Parse(idempotencyKeyStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_idempotency_key", "Idempotency-Key must be a valid UUID")
		return
	}

	var req struct {
		AccountText           string      `json:"account_text"`
		RouteType             string      `json:"route_type"`
		ConflictFlag          *string     `json:"conflict_flag"`
		SafeContactPreference interface{} `json:"safe_contact_preference"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}
	if req.AccountText == "" {
		writeError(w, http.StatusBadRequest, "missing_account_text", "account_text is required")
		return
	}
	validRoutes := map[string]bool{"ask_for_help": true, "worried_about_someone": true, "missing_child": true}
	if !validRoutes[req.RouteType] {
		writeError(w, http.StatusBadRequest, "invalid_route_type",
			"route_type must be ask_for_help, worried_about_someone, or missing_child")
		return
	}

	// Get the session to find the organization (via entry point)
	var orgID string
	err = h.pool.QueryRow(r.Context(), `
		SELECT ep.organization_id
		FROM private_sessions ps
		JOIN entry_points ep ON ep.id = ps.entry_point_id
		WHERE ps.id = $1 AND ps.revoked_at IS NULL AND ps.expires_at > NOW()
	`, claims.SessionID).Scan(&orgID)
	if err != nil {
		if h.cfg.AppMode != config.AppModeDemo || err != pgx.ErrNoRows {
			writeError(w, http.StatusServiceUnavailable, "route_unavailable", "A verified support route is required before submitting")
			return
		}
		// Only synthetic demos may use the fictional default organization.
		orgID = "00000000-0000-0000-0000-000000000001"
	}

	// Check for existing case with this idempotency key (idempotent retry)
	var existingCaseID, existingReceiptID, existingStatus string
	var existingCreatedAt time.Time
	err = h.pool.QueryRow(r.Context(), `
		SELECT c.id, c.receipt_id, c.status, c.created_at FROM cases c
		JOIN private_sessions ps ON ps.case_id = c.id
		WHERE c.idempotency_key = $1 AND ps.id = $2
	`, idempotencyKey, claims.SessionID).Scan(&existingCaseID, &existingReceiptID, &existingStatus, &existingCreatedAt)
	if err == nil {
		// Idempotent: return the existing receipt
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"case_id":     existingCaseID,
			"receipt_id":  existingReceiptID,
			"status":      existingStatus,
			"received_at": existingCreatedAt,
		})
		return
	}
	if err != pgx.ErrNoRows {
		log.Error().Err(err).Msg("error checking idempotency key")
		writeError(w, http.StatusInternalServerError, "db_error", "Database error")
		return
	}

	// Create case in a transaction — receipt only issued after commit
	caseID := uuid.New()
	receiptID := uuid.New()
	now := time.Now()

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "Could not start transaction")
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck

	_, err = tx.Exec(r.Context(), `
		INSERT INTO cases (id, organization_id, idempotency_key, status, route_type, account_text, conflict_flag, receipt_id)
		VALUES ($1, $2, $3, 'RECEIVED', $4, $5, $6, $7)
	`, caseID, orgID, idempotencyKey, req.RouteType, req.AccountText, req.ConflictFlag, receiptID)
	if err != nil {
		log.Error().Err(err).Msg("failed to insert case")
		writeError(w, http.StatusInternalServerError, "db_error", "Could not create case")
		return
	}

	// Link session to case
	_, err = tx.Exec(r.Context(), `
		UPDATE private_sessions SET case_id = $1 WHERE id = $2
	`, caseID, claims.SessionID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "db_error", "Report not confirmed; retry")
		return
	}

	// Write initial case event
	_, err = tx.Exec(r.Context(), `
		INSERT INTO case_events (case_id, from_status, to_status, actor_type, case_version)
		VALUES ($1, NULL, 'RECEIVED', 'system', 1)
	`, caseID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "db_error", "Report not confirmed; retry")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		log.Error().Err(err).Msg("failed to commit case transaction")
		writeError(w, http.StatusInternalServerError, "commit_failed",
			"Case could not be confirmed. Your report has NOT been received. Please retry.")
		return
	}

	log.Info().Str("case_id", caseID.String()).Str("route", req.RouteType).Msg("case created")

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"case_id":     caseID,
		"receipt_id":  receiptID,
		"status":      "RECEIVED",
		"received_at": now,
	})
}

// HandleSafeView returns the child-safe view of a case.
// Only safe progress fields are returned — never staff notes, other reporter data,
// or restricted contact/identity information.
func (h *SessionHandler) HandleSafeView(w http.ResponseWriter, r *http.Request) {
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

	var status, progressMsg string
	var lastUpdatedAt time.Time
	err := h.pool.QueryRow(r.Context(), `
		SELECT status, updated_at, COALESCE(
			(SELECT body FROM case_messages
			 WHERE case_id = $1 AND sender_type = 'responder' AND is_staff_note = FALSE
			 ORDER BY sent_at DESC LIMIT 1),
			''
		)
		FROM cases WHERE id = $1
	`, caseID).Scan(&status, &lastUpdatedAt, &progressMsg)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Case not found")
		return
	}

	// Fetch child-safe messages only (never staff notes)
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

	type msg struct {
		ID         string    `json:"id"`
		Body       string    `json:"body"`
		SentAt     time.Time `json:"sent_at"`
		SenderRole string    `json:"sender_role"`
	}
	var messages []msg
	for rows.Next() {
		var m msg
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
		messages = append(messages, m)
	}

	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"case_id":               caseID,
		"status":                status,
		"last_updated_at":       lastUpdatedAt,
		"safe_progress_message": safeProgressMessage(status),
		"messages":              messages,
	})
}

// safeProgressMessage returns a plain-language status update safe for a child to read.
// It never reveals investigation details, staff notes, or other reporter identities.
func safeProgressMessage(status string) string {
	switch status {
	case "RECEIVED":
		return "Your message has been received safely. Someone will look at it soon."
	case "TRIAGE":
		return "Your message is being reviewed by our team."
	case "ASSIGNED":
		return "A support person has been assigned to help you."
	case "ACCEPTED":
		return "Your support person has accepted your case and is ready to help."
	case "IN_PROGRESS":
		return "Your support person is actively working on your case."
	case "FOLLOW_UP":
		return "Your support person is following up to make sure you're okay."
	case "CLOSED":
		return "Your case has been closed. If you need more help, please reach out again."
	default:
		return "Your message has been received."
	}
}

// StaffHandler handles case routes for authenticated staff.
type StaffHandler struct {
	pool *db.Pool
	cfg  *config.Config
}

// NewStaffHandler creates a StaffHandler.
func NewStaffHandler(pool *db.Pool, cfg *config.Config) *StaffHandler {
	return &StaffHandler{pool: pool, cfg: cfg}
}

// Routes registers staff case routes.
func (h *StaffHandler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/", h.HandleList)
	r.Get("/{caseID}", h.HandleGet)
	r.Post("/{caseID}/assignments", h.HandleAssign)
	r.Post("/{caseID}/messages", h.HandleSendMessage)
	return r
}

// HandleList returns cases for the authenticated staff member's organization.
// Never returns cases from other organizations.
func (h *StaffHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	claims := authMW.GetStaffClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	statusFilter := r.URL.Query().Get("status")
	var rows pgx.Rows
	var err error

	if statusFilter != "" {
		rows, err = h.pool.Query(r.Context(), `
			SELECT id, status, route_type, conflict_flag, created_at, updated_at, version
			FROM cases
			WHERE organization_id = $1 AND status = $2::case_status
			ORDER BY created_at DESC LIMIT 50
		`, claims.OrganizationID, statusFilter)
	} else {
		rows, err = h.pool.Query(r.Context(), `
			SELECT id, status, route_type, conflict_flag, created_at, updated_at, version
			FROM cases
			WHERE organization_id = $1
			ORDER BY created_at DESC LIMIT 50
		`, claims.OrganizationID)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "")
		return
	}
	defer rows.Close()

	type caseRow struct {
		ID           string    `json:"case_id"`
		Status       string    `json:"status"`
		RouteType    string    `json:"route_type"`
		ConflictFlag *string   `json:"conflict_flag"`
		CreatedAt    time.Time `json:"created_at"`
		UpdatedAt    time.Time `json:"updated_at"`
		Version      int       `json:"version"`
	}
	var cases []caseRow
	for rows.Next() {
		var c caseRow
		if err := rows.Scan(&c.ID, &c.Status, &c.RouteType, &c.ConflictFlag,
			&c.CreatedAt, &c.UpdatedAt, &c.Version); err != nil {
			continue
		}
		cases = append(cases, c)
	}
	if cases == nil {
		cases = []caseRow{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"cases": cases, "total": len(cases)})
}

// HandleGet returns the full staff view of a case.
// Enforces org-scope: returns 403 if the case belongs to a different organization.
func (h *StaffHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	claims := authMW.GetStaffClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	caseID := chi.URLParam(r, "caseID")

	var orgID, status, routeType, accountText string
	var conflictFlag *string
	var createdAt, updatedAt time.Time
	var version int

	err := h.pool.QueryRow(r.Context(), `
		SELECT organization_id, status, route_type, account_text, conflict_flag, created_at, updated_at, version
		FROM cases WHERE id = $1
	`, caseID).Scan(&orgID, &status, &routeType, &accountText, &conflictFlag,
		&createdAt, &updatedAt, &version)
	if err == pgx.ErrNoRows {
		writeError(w, http.StatusNotFound, "not_found", "Case not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "")
		return
	}

	// Enforce org-scope authorization
	if orgID != claims.OrganizationID {
		// Audit the denied access attempt
		_, _ = h.pool.Exec(r.Context(), `
			INSERT INTO audit_events (event_type, actor_type, actor_id, organization_id, target_type, target_id, metadata)
			VALUES ('cross_org_access_denied', 'staff', $1, $2, 'case', $3, '{"reason":"cross_org_access_denied"}')
		`, claims.StaffID, claims.OrganizationID, caseID)

		writeError(w, http.StatusForbidden, "forbidden",
			"Not authorized to access this case. This access attempt has been logged.")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"case_id":       caseID,
		"status":        status,
		"route_type":    routeType,
		"account_text":  accountText,
		"conflict_flag": conflictFlag,
		"created_at":    createdAt,
		"updated_at":    updatedAt,
		"version":       version,
	})
}

// HandleAssign creates an assignment for a case.
func (h *StaffHandler) HandleAssign(w http.ResponseWriter, r *http.Request) {
	claims := authMW.GetStaffClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if claims.Role != "supervisor" && claims.Role != "admin" {
		writeError(w, http.StatusForbidden, "insufficient_role", "Only supervisors can assign cases")
		return
	}

	caseID := chi.URLParam(r, "caseID")
	var req struct {
		ResponderID string `json:"responder_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ResponderID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "responder_id required")
		return
	}

	// Verify case belongs to this org
	var orgID string
	if err := h.pool.QueryRow(r.Context(), `SELECT organization_id FROM cases WHERE id = $1`, caseID).
		Scan(&orgID); err != nil || orgID != claims.OrganizationID {
		writeError(w, http.StatusForbidden, "forbidden", "Not authorized for this case")
		return
	}

	assignmentID := uuid.New()
	escalationDue := time.Now().Add(60 * time.Minute)

	_, err := h.pool.Exec(r.Context(), `
		INSERT INTO assignments (id, case_id, responder_id, organization_id, assigned_by, escalation_due_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, assignmentID, caseID, req.ResponderID, claims.OrganizationID, claims.StaffID, escalationDue)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "Could not create assignment")
		return
	}

	// Update case status to ASSIGNED
	_, _ = h.pool.Exec(r.Context(), `
		UPDATE cases SET status = 'ASSIGNED', updated_at = NOW(), version = version + 1 WHERE id = $1
	`, caseID)

	writeJSON(w, http.StatusCreated, map[string]string{"assignment_id": assignmentID.String()})
}

// HandleSendMessage sends a message on a case (staff side).
func (h *StaffHandler) HandleSendMessage(w http.ResponseWriter, r *http.Request) {
	claims := authMW.GetStaffClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	caseID := chi.URLParam(r, "caseID")
	var req struct {
		Body        string `json:"body"`
		IsStaffNote bool   `json:"is_staff_note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Body == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "body required")
		return
	}

	// Verify org scope
	var orgID string
	if err := h.pool.QueryRow(r.Context(), `SELECT organization_id FROM cases WHERE id = $1`, caseID).
		Scan(&orgID); err != nil || orgID != claims.OrganizationID {
		writeError(w, http.StatusForbidden, "forbidden", "Not authorized for this case")
		return
	}

	msgID := uuid.New()
	_, err := h.pool.Exec(r.Context(), `
		INSERT INTO case_messages (id, case_id, sender_type, sender_id, body, is_staff_note)
		VALUES ($1, $2, 'responder', $3, $4, $5)
	`, msgID, caseID, claims.StaffID, req.Body, req.IsStaffNote)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "Could not send message")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"message_id": msgID.String()})
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
