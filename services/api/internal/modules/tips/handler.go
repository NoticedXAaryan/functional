// Package tips handles private tip submissions from the public and staff review.
//
// Security rules (T-038):
//   - Public endpoint returns ONLY an opaque receipt — never echoes sighting content.
//   - Staff endpoint is org-scoped; only org reviewers of the linked alert see the tip.
//   - Claimant (reporter) has NO automatic access to tips.
//   - Idempotency-Key prevents duplicate tip submissions on retry.
package tips

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

// ── Public handler ─────────────────────────────────────────────────────────────

// PublicHandler handles anonymous tip submissions.
// POST /api/v1/tips — returns opaque receipt only.
type PublicHandler struct {
	pool *db.Pool
	cfg  *config.Config
}

func NewPublicHandler(pool *db.Pool, cfg *config.Config) *PublicHandler {
	return &PublicHandler{pool: pool, cfg: cfg}
}

// HandleCreate accepts a tip and returns an opaque receipt.
// The sighting content is never returned to the submitter.
func (h *PublicHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	// Idempotency-Key prevents duplicate submissions on retry
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
		AlertID             string `json:"alert_id"`
		SightingDescription string `json:"sighting_description"`
		ApproximateLocation string `json:"approximate_location,omitempty"`
		ContactPreference   string `json:"contact_preference,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON")
		return
	}
	if req.AlertID == "" {
		writeError(w, http.StatusBadRequest, "missing_alert_id", "alert_id is required")
		return
	}
	if req.SightingDescription == "" {
		writeError(w, http.StatusBadRequest, "missing_description", "sighting_description is required")
		return
	}
	if len(req.SightingDescription) > 5000 {
		writeError(w, http.StatusBadRequest, "description_too_long", "sighting_description must be 5,000 characters or fewer")
		return
	}
	alertID, err := uuid.Parse(req.AlertID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_alert_id", "alert_id must be a valid UUID")
		return
	}

	// Idempotency: check if this key already exists
	var existingReceiptID uuid.UUID
	err = h.pool.QueryRow(r.Context(),
		`SELECT id FROM tips WHERE idempotency_key = $1`, idempotencyKey,
	).Scan(&existingReceiptID)
	if err == nil {
		// Already submitted — return same receipt
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"receipt_id": existingReceiptID.String(),
			"status":     "received",
		})
		return
	}
	if err != pgx.ErrNoRows {
		log.Error().Err(err).Msg("tips: failed to check idempotency")
		writeError(w, http.StatusInternalServerError, "db_error", "")
		return
	}

	// Verify alert exists and is ACTIVE
	var alertStatus string
	err = h.pool.QueryRow(r.Context(),
		`SELECT status FROM alerts WHERE id = $1`, alertID,
	).Scan(&alertStatus)
	if err == pgx.ErrNoRows {
		writeError(w, http.StatusNotFound, "alert_not_found", "Alert not found")
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("tips: failed to look up alert")
		writeError(w, http.StatusInternalServerError, "db_error", "")
		return
	}
	if alertStatus != "ACTIVE" {
		writeError(w, http.StatusConflict, "alert_not_active",
			"Tips can only be submitted for an active alert. This alert is no longer active.")
		return
	}

	tipID := uuid.New()
	_, err = h.pool.Exec(r.Context(), `
		INSERT INTO tips (id, alert_id, idempotency_key, sighting_description, approximate_location, contact_preference, received_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, tipID, alertID, idempotencyKey, req.SightingDescription,
		nullableStr(req.ApproximateLocation), nullableStr(req.ContactPreference), time.Now())
	if err != nil {
		log.Error().Err(err).Str("alert_id", alertID.String()).Msg("tips: failed to insert tip")
		writeError(w, http.StatusInternalServerError, "db_error", "Could not record tip")
		return
	}

	log.Info().Str("tip_id", tipID.String()).Str("alert_id", alertID.String()).Msg("tip received")

	// Return ONLY the opaque receipt — never the sighting content
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"receipt_id": tipID.String(),
		"status":     "received",
	})
}

// ── Staff handler ─────────────────────────────────────────────────────────────

// StaffHandler handles staff-authenticated tip review.
// GET /api/v1/staff/tips?alert_id=<uuid> — lists tips for alerts in the staff member's org.
type StaffHandler struct {
	pool *db.Pool
	cfg  *config.Config
}

func NewStaffHandler(pool *db.Pool, cfg *config.Config) *StaffHandler {
	return &StaffHandler{pool: pool, cfg: cfg}
}

func (h *StaffHandler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/", h.HandleList)
	return r
}

// tipRow is the internal representation returned to authorized staff reviewers.
type tipRow struct {
	ID                  string     `json:"id"`
	AlertID             string     `json:"alert_id"`
	SightingDescription string     `json:"sighting_description"`
	ApproximateLocation *string    `json:"approximate_location,omitempty"`
	ContactPreference   *string    `json:"contact_preference,omitempty"`
	ReceivedAt          time.Time  `json:"received_at"`
}

// HandleList returns all tips for a given alert, scoped to the staff member's org.
// The alert must belong to the staff member's organization.
func (h *StaffHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	claims := authMW.GetStaffClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Staff authorization required")
		return
	}

	alertIDStr := r.URL.Query().Get("alert_id")
	if alertIDStr == "" {
		writeError(w, http.StatusBadRequest, "missing_alert_id", "alert_id query parameter is required")
		return
	}
	if _, err := uuid.Parse(alertIDStr); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_alert_id", "alert_id must be a valid UUID")
		return
	}

	// Enforce org-scope: verify this alert belongs to the staff member's organization
	var alertOrgID string
	err := h.pool.QueryRow(r.Context(),
		`SELECT organization_id FROM alerts WHERE id = $1`, alertIDStr,
	).Scan(&alertOrgID)
	if err == pgx.ErrNoRows {
		writeError(w, http.StatusNotFound, "alert_not_found", "Alert not found")
		return
	}
	if err != nil {
		log.Error().Err(err).Str("alert_id", alertIDStr).Msg("tips: failed to look up alert org")
		writeError(w, http.StatusInternalServerError, "db_error", "")
		return
	}
	if alertOrgID != claims.OrganizationID {
		// Audit the denial without leaking content
		_, _ = h.pool.Exec(r.Context(), `
			INSERT INTO audit_events (event_type, actor_type, actor_id, organization_id, target_type, target_id, metadata)
			VALUES ('cross_org_tips_access_denied', 'staff', $1, $2, 'alert', $3, '{"reason":"cross_org_access_denied"}')
		`, claims.StaffID, claims.OrganizationID, alertIDStr)
		writeError(w, http.StatusForbidden, "forbidden",
			"Not authorized to access tips for this alert. This access attempt has been logged.")
		return
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT id, alert_id, sighting_description, approximate_location, contact_preference, received_at
		FROM tips
		WHERE alert_id = $1
		ORDER BY received_at ASC
	`, alertIDStr)
	if err != nil {
		log.Error().Err(err).Str("alert_id", alertIDStr).Msg("tips: failed to query tips")
		writeError(w, http.StatusInternalServerError, "db_error", "")
		return
	}
	defer rows.Close()

	var tips []tipRow
	for rows.Next() {
		var t tipRow
		if err := rows.Scan(&t.ID, &t.AlertID, &t.SightingDescription,
			&t.ApproximateLocation, &t.ContactPreference, &t.ReceivedAt); err != nil {
			log.Error().Err(err).Msg("tips: failed to scan row")
			continue
		}
		tips = append(tips, t)
	}
	if tips == nil {
		tips = []tipRow{}
	}

	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"alert_id": alertIDStr,
		"tips":     tips,
		"total":    len(tips),
	})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func nullableStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
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
