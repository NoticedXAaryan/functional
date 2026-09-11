// Package alerts implements alert draft, approval, activation, withdrawal, and public views.
// Authorization: alert_preparer role can create drafts; alert_approver role can approve (cannot self-approve).
// Activation: requires an approved revision. Any field change creates a new revision.
// Outbox: activation and outbox event are written in a single transaction.
package alerts

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

// DraftHandler handles alert draft creation.
type DraftHandler struct{ pool *db.Pool; cfg *config.Config }

func NewDraftHandler(pool *db.Pool, cfg *config.Config) *DraftHandler {
	return &DraftHandler{pool: pool, cfg: cfg}
}
func (h *DraftHandler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Post("/", h.HandleCreate)
	r.Get("/{revisionID}", h.HandleGet)
	return r
}

func (h *DraftHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	claims := authMW.GetStaffClaims(r.Context())
	if claims == nil || claims.Role != "alert_preparer" {
		writeError(w, http.StatusForbidden, "insufficient_role", "alert_preparer role required")
		return
	}

	var req struct {
		CaseID          string  `json:"case_id"`
		DescriptionText string  `json:"description_text"`
		IssuerName      string  `json:"issuer_name"`
		ExpiryAt        string  `json:"expiry_at"`
		TipRouteEmail   string  `json:"tip_route_email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON")
		return
	}
	if req.DescriptionText == "" || req.IssuerName == "" || req.ExpiryAt == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "description_text, issuer_name, expiry_at required")
		return
	}

	expiry, err := time.Parse(time.RFC3339, req.ExpiryAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_expiry", "expiry_at must be RFC3339")
		return
	}

	alertID := uuid.New()
	revisionID := uuid.New()

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "")
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck

	// Create the alert container
	_, err = tx.Exec(r.Context(), `
		INSERT INTO alerts (id, case_id, organization_id, status)
		VALUES ($1, $2, $3, 'DRAFT')
	`, alertID, req.CaseID, claims.OrganizationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "Could not create alert")
		return
	}

	// Create the first revision
	_, err = tx.Exec(r.Context(), `
		INSERT INTO alert_revisions (id, alert_id, revision_number, status, description_text, issuer_name, expiry_at, tip_route_email, prepared_by)
		VALUES ($1, $2, 1, 'DRAFT', $3, $4, $5, $6, $7)
	`, revisionID, alertID, req.DescriptionText, req.IssuerName, expiry, req.TipRouteEmail, claims.StaffID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "Could not create revision")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "commit_failed", "")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"revision_id": revisionID, "alert_id": alertID, "status": "DRAFT",
		"created_at": time.Now(),
	})
}

func (h *DraftHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	claims := authMW.GetStaffClaims(r.Context())
	if claims == nil { writeError(w, http.StatusUnauthorized, "unauthorized", ""); return }

	revID := chi.URLParam(r, "revisionID")
	var alertID, status, desc, issuer string
	var expiry time.Time
	err := h.pool.QueryRow(r.Context(), `
		SELECT alert_id, status, description_text, issuer_name, expiry_at
		FROM alert_revisions WHERE id = $1
	`, revID).Scan(&alertID, &status, &desc, &issuer, &expiry)
	if err == pgx.ErrNoRows {
		writeError(w, http.StatusNotFound, "not_found", "Revision not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"revision_id": revID, "alert_id": alertID, "status": status,
		"description_text": desc, "issuer_name": issuer, "expiry_at": expiry,
	})
}

// ApprovalHandler handles alert approval and rejection.
type ApprovalHandler struct{ pool *db.Pool; cfg *config.Config }

func NewApprovalHandler(pool *db.Pool, cfg *config.Config) *ApprovalHandler {
	return &ApprovalHandler{pool: pool, cfg: cfg}
}
func (h *ApprovalHandler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/", h.HandleListDrafts)
	r.Post("/", h.HandleDecision)
	return r
}

func (h *ApprovalHandler) HandleListDrafts(w http.ResponseWriter, r *http.Request) {
	claims := authMW.GetStaffClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT ar.id, ar.alert_id, ar.status, ar.description_text, ar.issuer_name, ar.expiry_at, ar.prepared_by
		FROM alert_revisions ar
		JOIN alerts a ON a.id = ar.alert_id
		WHERE a.organization_id = $1 AND ar.status = 'DRAFT'
		ORDER BY ar.created_at DESC LIMIT 50
	`, claims.OrganizationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "Database error")
		return
	}
	defer rows.Close()

	type draftRow struct {
		ID              string    `json:"id"`
		AlertID         string    `json:"alert_id"`
		Status          string    `json:"status"`
		DescriptionText string    `json:"description_text"`
		IssuerName      string    `json:"issuer_name"`
		ExpiryAt        time.Time `json:"expiry_at"`
		PreparedBy      string    `json:"prepared_by"`
	}

	var drafts []draftRow
	for rows.Next() {
		var d draftRow
		if err := rows.Scan(&d.ID, &d.AlertID, &d.Status, &d.DescriptionText, &d.IssuerName, &d.ExpiryAt, &d.PreparedBy); err != nil {
			continue
		}
		drafts = append(drafts, d)
	}
	if drafts == nil {
		drafts = []draftRow{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"drafts": drafts})
}

func (h *ApprovalHandler) HandleDecision(w http.ResponseWriter, r *http.Request) {
	claims := authMW.GetStaffClaims(r.Context())
	if claims == nil || claims.Role != "alert_approver" {
		writeError(w, http.StatusForbidden, "insufficient_role", "alert_approver role required")
		return
	}

	var req struct {
		RevisionID string `json:"revision_id"`
		Decision   string `json:"decision"`
		Notes      string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON")
		return
	}
	if req.Decision != "approve" && req.Decision != "reject" {
		writeError(w, http.StatusBadRequest, "invalid_decision", "decision must be approve or reject")
		return
	}

	// Verify: approver cannot be the preparer (self-approval prevention)
	var preparedBy string
	if err := h.pool.QueryRow(r.Context(),
		`SELECT prepared_by FROM alert_revisions WHERE id = $1`, req.RevisionID).
		Scan(&preparedBy); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Revision not found")
		return
	}
	if preparedBy == claims.StaffID {
		writeError(w, http.StatusForbidden, "self_approval_rejected",
			"The approver cannot be the same person who prepared this revision. Two-person approval is required.")
		return
	}

	_, err := h.pool.Exec(r.Context(), `
		INSERT INTO alert_approvals (alert_id, revision_id, approver_id, decision, notes)
		SELECT alert_id, $1, $2, $3, $4 FROM alert_revisions WHERE id = $1
	`, req.RevisionID, claims.StaffID, req.Decision, req.Notes)
	if err != nil {
		writeError(w, http.StatusConflict, "already_decided", "This revision already has an approval decision")
		return
	}

	// Update revision status
	newStatus := "APPROVED"
	if req.Decision == "reject" { newStatus = "DRAFT" }
	_, _ = h.pool.Exec(r.Context(),
		`UPDATE alert_revisions SET status = $1 WHERE id = $2`, newStatus, req.RevisionID)

	log.Info().Str("revision_id", req.RevisionID).Str("decision", req.Decision).
		Str("approver", claims.StaffID).Msg("alert revision decision recorded")
	writeJSON(w, http.StatusOK, map[string]string{"status": newStatus})
}

// ActivationHandler handles alert activation and withdrawal.
type ActivationHandler struct{ pool *db.Pool; cfg *config.Config }

func NewActivationHandler(pool *db.Pool, cfg *config.Config) *ActivationHandler {
	return &ActivationHandler{pool: pool, cfg: cfg}
}
func (h *ActivationHandler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Post("/{alertID}/activate", h.HandleActivate)
	r.Post("/{alertID}/withdraw", h.HandleWithdraw)
	return r
}

func (h *ActivationHandler) HandleActivate(w http.ResponseWriter, r *http.Request) {
	claims := authMW.GetStaffClaims(r.Context())
	if claims == nil { writeError(w, http.StatusUnauthorized, "unauthorized", ""); return }

	alertID := chi.URLParam(r, "alertID")
	var req struct {
		ApprovedRevisionID string `json:"approved_revision_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "approved_revision_id required")
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil { writeError(w, http.StatusInternalServerError, "db_error", ""); return }
	defer tx.Rollback(r.Context()) //nolint:errcheck

	// Verify the revision is approved and belongs to this alert
	var revStatus string
	var expiry time.Time
	err = tx.QueryRow(r.Context(), `
		SELECT ar.status, ar.expiry_at FROM alert_revisions ar
		JOIN alert_approvals aa ON aa.revision_id = ar.id AND aa.decision = 'approve'
		WHERE ar.id = $1 AND ar.alert_id = $2
	`, req.ApprovedRevisionID, alertID).Scan(&revStatus, &expiry)
	if err == pgx.ErrNoRows {
		writeError(w, http.StatusConflict, "not_approved", "Revision is not approved or does not exist")
		return
	}
	if time.Now().After(expiry) {
		writeError(w, http.StatusConflict, "revision_expired", "This revision has expired")
		return
	}

	// Activate the alert and write outbox event — in one transaction
	_, err = tx.Exec(r.Context(), `
		UPDATE alerts SET status = 'ACTIVE', active_revision_id = $1, updated_at = NOW() WHERE id = $2
	`, req.ApprovedRevisionID, alertID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "Activation failed")
		return
	}
	_, err = tx.Exec(r.Context(), `
		INSERT INTO outbox_events (event_type, alert_id, revision_id, payload)
		VALUES ('alert_activated', $1, $2, '{}')
	`, alertID, req.ApprovedRevisionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "Outbox write failed")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "commit_failed", "")
		return
	}

	log.Info().Str("alert_id", alertID).Str("revision_id", req.ApprovedRevisionID).Msg("alert activated")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ACTIVE"})
}

func (h *ActivationHandler) HandleWithdraw(w http.ResponseWriter, r *http.Request) {
	claims := authMW.GetStaffClaims(r.Context())
	if claims == nil { writeError(w, http.StatusUnauthorized, "unauthorized", ""); return }

	alertID := chi.URLParam(r, "alertID")
	var req struct{ Reason string `json:"reason"` }
	json.NewDecoder(r.Body).Decode(&req) //nolint:errcheck

	_, err := h.pool.Exec(r.Context(),
		`UPDATE alerts SET status = 'WITHDRAWN', updated_at = NOW() WHERE id = $1 AND status = 'ACTIVE'`,
		alertID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "")
		return
	}

	// Cancel all queued delivery jobs for this alert
	_, _ = h.pool.Exec(r.Context(),
		`UPDATE delivery_jobs SET status = 'CANCELLED' WHERE alert_id = $1 AND status = 'QUEUED'`, alertID)

	log.Info().Str("alert_id", alertID).Str("reason", req.Reason).Msg("alert withdrawn, queued jobs cancelled")
	writeJSON(w, http.StatusOK, map[string]string{"status": "WITHDRAWN"})
}

// PublicHandler serves the approved alert projection (no private data).
type PublicHandler struct{ pool *db.Pool; cfg *config.Config }

func NewPublicHandler(pool *db.Pool, cfg *config.Config) *PublicHandler {
	return &PublicHandler{pool: pool, cfg: cfg}
}
func (h *PublicHandler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/{alertID}", h.HandleGet)
	return r
}

func (h *PublicHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	alertID := chi.URLParam(r, "alertID")
	// Only serve approved public projection — never case_id, private narrative, or contact fields
	var revisionID, status, desc, issuer string
	var expiry, issuedAt time.Time
	err := h.pool.QueryRow(r.Context(), `
		SELECT ar.id, a.status, ar.description_text, ar.issuer_name, ar.expiry_at, a.created_at
		FROM alerts a
		JOIN alert_revisions ar ON ar.id = a.active_revision_id
		WHERE a.id = $1 AND a.status IN ('ACTIVE', 'RESOLVED', 'WITHDRAWN', 'EXPIRED')
	`, alertID).Scan(&revisionID, &status, &desc, &issuer, &expiry, &issuedAt)
	if err == pgx.ErrNoRows {
		writeError(w, http.StatusNotFound, "not_found", "Alert not found or not publicly visible")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "")
		return
	}

	isTest := h.cfg.AppMode == "demo" || h.cfg.IsTestMode()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"alert_id":           alertID,
		"revision_id":        revisionID,
		"status":             status,
		"description_text":   desc,
		"issuer_name":        issuer,
		"expiry_at":          expiry,
		"issued_at":          issuedAt,
		"tip_route_available": true,
		"current_status_url": "/public/alerts/" + alertID,
		"is_test":            isTest,
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
