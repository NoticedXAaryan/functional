// Package entrypoints resolves QR placement opaque IDs.
// A revoked, unknown, or replaced entry point returns a 404 with no private data.
package entrypoints

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/balsuraksha/api/internal/config"
	"github.com/balsuraksha/api/internal/db"
)

// Handler resolves QR entry points.
type Handler struct {
	pool *db.Pool
	cfg  *config.Config
}

// NewHandler creates an entry point Handler.
func NewHandler(pool *db.Pool, cfg *config.Config) *Handler {
	return &Handler{pool: pool, cfg: cfg}
}

// Routes registers entry point routes.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/{opaqueID}", h.HandleResolve)
	return r
}

// HandleResolve resolves an opaque QR identifier to venue and service information.
// Revoked, unknown, or replaced placements return 404 with no private data disclosed.
func (h *Handler) HandleResolve(w http.ResponseWriter, r *http.Request) {
	opaqueID := chi.URLParam(r, "opaqueID")

	var epID, venueName string
	var active bool
	var serviceHours *string
	var languages []string
	var routes []string

	err := h.pool.QueryRow(r.Context(), `
		SELECT ep.id, v.name, ep.active, ep.service_hours, ep.languages, ep.routes_available
		FROM entry_points ep
		JOIN venues v ON v.id = ep.venue_id
		WHERE ep.opaque_id = $1
	`, opaqueID).Scan(&epID, &venueName, &active, &serviceHours, &languages, &routes)

	if err == pgx.ErrNoRows {
		// Unknown or non-existent QR — safe alternative message, no data disclosure
		writeError(w, http.StatusNotFound, "not_found",
			"This QR code could not be found. If you need help, please contact 1098 (Child Helpline) or 112 (Emergency).")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "")
		return
	}

	if !active {
		// Revoked or replaced — still 404, no reason disclosed
		writeError(w, http.StatusNotFound, "not_available",
			"This QR code is no longer active. Please use a current poster or contact 1098 (Child Helpline) or 112 (Emergency).")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":         epID,
		"venue_name": venueName,
		"active":     active,
		"languages":  languages,
		"service_hours": serviceHours,
		"routes_available": routes,
		"emergency_numbers": map[string]string{
			"national_emergency": "112",
			"child_helpline":     "1098",
		},
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
