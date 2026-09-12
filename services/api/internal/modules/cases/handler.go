// Package cases handles private case creation (child intake) and staff case management.
// Authorization rules:
//   - Session-authenticated users can only access their own case.
//   - Staff can only access cases belonging to their organization.
//   - Cross-org access is denied and audited.
package cases

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
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
		AccountText           string            `json:"account_text"`
		RouteType             string            `json:"route_type"`
		ConflictFlag          *string           `json:"conflict_flag"`
		SafeContactPreference ContactPreference `json:"safe_contact_preference"`
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

	if req.ConflictFlag != nil && *req.ConflictFlag != "" {
		if *req.ConflictFlag != "school_implicated" && *req.ConflictFlag != "caregiver_implicated" && *req.ConflictFlag != "staff_implicated" {
			writeError(w, 400, "invalid_conflict", "Choose a listed conflict option")
			return
		}
	} else {
		req.ConflictFlag = nil
	}
	if strings.TrimSpace(req.AccountText) == "" || len(req.AccountText) > 16000 {
		writeError(w, 400, "invalid_text", "Describe what happened in 16,000 bytes or fewer")
		return
	}
	if err := req.SafeContactPreference.Validate(); err != nil {
		writeError(w, 400, "invalid_contact", err.Error())
		return
	}
	encoded, _ := json.Marshal(req)
	sum := sha256.Sum256(encoded)
	requestDigest := hex.EncodeToString(sum[:])
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		writeError(w, 503, "db_error", "Try again")
		return
	}
	defer tx.Rollback(r.Context())
	var existing *string
	var entryPoint *string
	err = tx.QueryRow(r.Context(), `SELECT case_id,entry_point_id FROM private_sessions WHERE id=$1 AND token_jti=$2 AND revoked_at IS NULL AND expires_at>NOW() FOR UPDATE`, claims.SessionID, claims.ID).Scan(&existing, &entryPoint)
	if err != nil {
		writeError(w, 401, "session_expired", "Open a new session")
		return
	}
	if existing != nil {
		var receiptID, status, key string
		var created time.Time
		var digest *string
		if err = tx.QueryRow(r.Context(), `SELECT receipt_id,status,idempotency_key,created_at,submission_digest FROM cases WHERE id=$1`, *existing).Scan(&receiptID, &status, &key, &created, &digest); err != nil {
			writeError(w, 503, "db_error", "Try again")
			return
		}
		if key != idempotencyKey.String() || digest == nil || *digest != requestDigest {
			writeError(w, 409, "already_submitted", "This session already submitted a report. Keep its receipt; start a new session for a different report.")
			return
		}
		writeJSON(w, 200, map[string]interface{}{"case_id": *existing, "receipt_id": receiptID, "status": status, "received_at": created})
		return
	}
	// Serialize global idempotency keys without exposing a different session's receipt.
	if _, err = tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(hashtextextended($1,31))`, idempotencyKey.String()); err != nil {
		writeError(w, 503, "db_error", "Try again")
		return
	}
	var keyUsed bool
	if err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM cases WHERE idempotency_key=$1)`, idempotencyKey).Scan(&keyUsed); err != nil {
		writeError(w, 503, "db_error", "Try again")
		return
	}
	if keyUsed {
		writeError(w, 409, "key_used", "Use a new submission key")
		return
	}
	var orgID string
	var independent *string
	if entryPoint != nil {
		err = tx.QueryRow(r.Context(), `SELECT ep.organization_id,ep.independent_organization_id FROM entry_points ep JOIN organizations o ON o.id=ep.organization_id WHERE ep.id=$1 AND ep.active AND ep.revoked_at IS NULL AND o.active`, *entryPoint).Scan(&orgID, &independent)
		if err != nil {
			writeError(w, 503, "route_unavailable", "This support route is unavailable")
			return
		}
	} else if h.cfg.AppMode == config.AppModeDemo {
		orgID = "00000000-0000-0000-0000-000000000001"
	} else {
		writeError(w, 503, "route_unavailable", "A support organization is required")
		return
	}
	if req.ConflictFlag != nil && independent != nil && *independent != orgID {
		orgID = *independent
	} else if req.ConflictFlag != nil && h.cfg.AppMode != config.AppModeDemo {
		writeError(w, 503, "independent_route_unavailable", "An independent support route is not available here. Your report has not been sent.")
		return
	}
	if h.cfg.AppMode != config.AppModeDemo {
		var ready bool
		err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM organizations o JOIN staff_members sm ON sm.organization_id=o.id JOIN role_grants sr ON sr.staff_id=sm.id AND sr.organization_id=o.id WHERE o.id=$1 AND o.active AND sm.active AND sr.revoked_at IS NULL AND sr.role IN ('supervisor','admin'))`, orgID).Scan(&ready)
		if err != nil || !ready {
			writeError(w, 503, "route_unavailable", "This organization is not accepting reports")
			return
		}
	}
	caseID := uuid.New()
	receiptID := uuid.New()
	now := time.Now()
	_, err = tx.Exec(r.Context(), `
		INSERT INTO cases (id, organization_id, idempotency_key, status, route_type, account_text, conflict_flag, receipt_id, submission_digest, entry_point_id)
		VALUES ($1, $2, $3, 'RECEIVED', $4, $5, $6, $7, $8, $9)
	`, caseID, orgID, idempotencyKey, req.RouteType, req.AccountText, req.ConflictFlag, receiptID, requestDigest, entryPoint)
	if err != nil {
		log.Error().Err(err).Msg("failed to insert case")
		writeError(w, http.StatusInternalServerError, "db_error", "Could not create case")
		return
	}

	if _, err = tx.Exec(r.Context(), `INSERT INTO contact_preferences(case_id,preferred_channel,safe_hours_start,safe_hours_end,time_zone) VALUES($1,$2,$3::time,$4::time,$5)`, caseID, req.SafeContactPreference.PreferredChannel, req.SafeContactPreference.SafeHoursStart, req.SafeContactPreference.SafeHoursEnd, req.SafeContactPreference.TimeZone); err != nil {
		writeError(w, 503, "db_error", "Report not confirmed; retry")
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
		return "Your report has been saved. A responder has not accepted it yet."
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
	// Assignment changes use the canonical /staff/assignments endpoint.
	// Messages use the canonical /staff/messages endpoint.
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
	rows, err := h.pool.Query(r.Context(), `SELECT c.id,c.status,c.route_type,c.conflict_flag,c.created_at,c.updated_at,c.version FROM cases c WHERE c.organization_id=$1 AND ($2='' OR c.status::text=$2) AND ($3 IN ('supervisor','admin','alert_preparer','alert_approver') OR EXISTS(SELECT 1 FROM assignments a WHERE a.case_id=c.id AND a.responder_id=$4 AND a.unassigned_at IS NULL) OR EXISTS(SELECT 1 FROM case_access_grants g WHERE g.case_id=c.id AND g.staff_id=$4 AND g.revoked_at IS NULL)) ORDER BY c.created_at DESC LIMIT 50`, claims.OrganizationID, statusFilter, claims.Role, claims.StaffID)
	if err != nil {
		writeError(w, 503, "unavailable", "Could not load reports")
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

	if !authMW.CanReadCase(r.Context(), h.pool, claims, caseID) {
		writeError(w, 403, "case_access_required", "Only assigned staff or a supervisor can open this report")
		return
	}
	var contact ContactPreference
	if err = h.pool.QueryRow(r.Context(), `SELECT COALESCE(preferred_channel,'no_contact'),COALESCE(safe_hours_start::text,'00:00'),COALESCE(safe_hours_end::text,'00:00'),time_zone FROM contact_preferences WHERE case_id=$1`, caseID).Scan(&contact.PreferredChannel, &contact.SafeHoursStart, &contact.SafeHoursEnd, &contact.TimeZone); err != nil {
		contact.PreferredChannel = "no_contact"
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"case_id":                 caseID,
		"status":                  status,
		"route_type":              routeType,
		"account_text":            accountText,
		"safe_contact_preference": contact,
		"conflict_flag":           conflictFlag,
		"created_at":              createdAt,
		"updated_at":              updatedAt,
		"version":                 version,
	})
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
