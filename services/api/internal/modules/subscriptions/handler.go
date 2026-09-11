// Package subscriptions handles Web Push alert subscription registration and revocation.
//
// Rules:
//   - Explicit consent is required (consent_confirmed_at must be recorded).
//   - VAPID Web Push endpoints (browser PushSubscription) are stored, NOT FCM tokens.
//   - Subscriptions are isolated from child intake sessions (no linkage).
//   - TEST allowlist max: 20 consenting adult test devices.
package subscriptions

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/balsuraksha/api/internal/config"
	"github.com/balsuraksha/api/internal/db"
)

// Handler handles subscription routes.
type Handler struct {
	pool *db.Pool
	cfg  *config.Config
}

// NewHandler creates a subscription Handler.
func NewHandler(pool *db.Pool, cfg *config.Config) *Handler {
	return &Handler{pool: pool, cfg: cfg}
}

// Routes registers public subscription endpoints.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Post("/", h.HandleCreate)
	r.Delete("/{subscriptionID}", h.HandleRevoke)
	return r
}

// HandleCreate registers a new push alert subscription with explicit consent.
func (h *Handler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AreaName         string `json:"area_name"`
		Endpoint         string `json:"endpoint"`           // Browser PushSubscription.endpoint URL
		Auth             string `json:"auth"`               // PushSubscription.getKey("auth") base64url
		P256DH           string `json:"p256dh"`             // PushSubscription.getKey("p256dh") base64url
		Language         string `json:"language"`
		ConsentConfirmed bool   `json:"consent_confirmed"`
		IsTest           bool   `json:"is_test"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON")
		return
	}

	if req.AreaName == "" || req.Endpoint == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "area_name and endpoint are required")
		return
	}
	if req.Auth == "" || req.P256DH == "" {
		writeError(w, http.StatusBadRequest, "missing_push_keys", "auth and p256dh are required for encrypted push delivery")
		return
	}

	if !req.ConsentConfirmed {
		writeError(w, http.StatusBadRequest, "consent_required",
			"Explicit consent confirmation is required to receive alert notifications.")
		return
	}

	if req.Language == "" {
		req.Language = "en"
	}

	subID := uuid.New()
	now := time.Now()

	// Store area subscription with consent confirmation and Web Push endpoint
	_, err := h.pool.Exec(r.Context(), `
		INSERT INTO area_subscriptions (id, area_name, endpoint, auth, p256dh, language, consent_confirmed_at, consent_purpose, is_test, subscribed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'Receive verified missing-child alerts for the chosen area.', $8, $7)
	`, subID, req.AreaName, req.Endpoint, req.Auth, req.P256DH, req.Language, now, req.IsTest)

	if err != nil {
		log.Error().Err(err).Msg("failed to create area subscription")
		writeError(w, http.StatusInternalServerError, "db_error", "Failed to register subscription")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"subscription_id": subID.String(),
		"area_name":       req.AreaName,
		"language":        req.Language,
		"is_test":         req.IsTest,
		"subscribed_at":   now,
	})
}

// HandleRevoke cancels an active subscription.
func (h *Handler) HandleRevoke(w http.ResponseWriter, r *http.Request) {
	subIDStr := chi.URLParam(r, "subscriptionID")
	subID, err := uuid.Parse(subIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "Invalid subscription ID")
		return
	}

	result, err := h.pool.Exec(r.Context(), `
		UPDATE area_subscriptions SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL
	`, subID)

	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "Failed to revoke subscription")
		return
	}

	if result.RowsAffected() == 0 {
		// Idempotent revocation check
		var exists bool
		_ = h.pool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM area_subscriptions WHERE id = $1)`, subID).Scan(&exists)
		if !exists {
			writeError(w, http.StatusNotFound, "not_found", "Subscription not found")
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg, "code": code})
}
