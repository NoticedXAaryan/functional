// Package routing handles case routing decisions, conflict-of-interest evaluation,
// and independent route assignment for implicated-staff cases.
//
// Rules (P1 Gate / T-011):
//   - When conflict_flag is set (e.g. 'staff_implicated' or 'school_implicated'),
//     the case MUST NOT be handled solely by the local organization staff.
//   - An independent route (e.g., state supervisory authority / independent NGO) is assigned.
//   - Implicated organization staff are excluded from case_access_grants.
package routing

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/balsuraksha/api/internal/config"
	"github.com/balsuraksha/api/internal/db"
	authMW "github.com/balsuraksha/api/internal/middleware"
)

// Handler handles staff-scoped case routing endpoints.
type Handler struct {
	pool *db.Pool
	cfg  *config.Config
}

// NewHandler creates a Handler for routing.
func NewHandler(pool *db.Pool, cfg *config.Config) *Handler {
	return &Handler{pool: pool, cfg: cfg}
}

// Routes registers routing management endpoints.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Post("/cases/{caseID}/evaluate", h.HandleEvaluateRouting)
	r.Get("/cases/{caseID}/route", h.HandleGetRoute)
	return r
}

type RouteDecision struct {
	CaseID               string   `json:"case_id"`
	OriginalOrgID        string   `json:"original_org_id"`
	EffectiveOrgID       string   `json:"effective_org_id"`
	ConflictFlag         *string  `json:"conflict_flag,omitempty"`
	IsIndependentRoute   bool     `json:"is_independent_route"`
	ImplicatedOrgBlocked bool     `json:"implicated_org_blocked"`
	Reason               string   `json:"reason"`
	AssignedRouteTarget  string   `json:"assigned_route_target"`
	EvaluatedAt          time.Time `json:"evaluated_at"`
}

// HandleEvaluateRouting determines and applies the route for a case based on conflict flags.
func (h *Handler) HandleEvaluateRouting(w http.ResponseWriter, r *http.Request) {
	staffClaims := authMW.GetStaffClaims(r.Context())
	if staffClaims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Staff token required")
		return
	}

	caseIDStr := chi.URLParam(r, "caseID")
	caseID, err := uuid.Parse(caseIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_case_id", "Invalid case ID format")
		return
	}

	// Fetch case and conflict status
	var orgID string
	var conflictFlag *string
	var status string
	err = h.pool.QueryRow(r.Context(), `
		SELECT organization_id, conflict_flag, status
		FROM cases WHERE id = $1
	`, caseID).Scan(&orgID, &conflictFlag, &status)

	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "case_not_found", "Case not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "db_error", "Database query failed")
		return
	}

	// Enforce org scope (unless independent supervisor)
	if orgID != staffClaims.OrganizationID && staffClaims.Role != "SUPERVISOR" && staffClaims.Role != "ADMIN" {
		writeError(w, http.StatusForbidden, "forbidden", "Cross-organization routing denied")
		return
	}

	decision := RouteDecision{
		CaseID:              caseID.String(),
		OriginalOrgID:       orgID,
		EffectiveOrgID:      orgID,
		ConflictFlag:        conflictFlag,
		IsIndependentRoute:  false,
		ImplicatedOrgBlocked: false,
		Reason:              "Standard local org intake routing",
		AssignedRouteTarget: "LOCAL_RESPONDER_POOL",
		EvaluatedAt:         time.Now(),
	}

	if conflictFlag != nil {
		switch *conflictFlag {
		case "staff_implicated", "school_implicated":
			// Route to independent supervisory org (demo independent org UUID)
			independentOrgID := "00000000-0000-0000-0000-000000000002"
			decision.EffectiveOrgID = independentOrgID
			decision.IsIndependentRoute = true
			decision.ImplicatedOrgBlocked = true
			decision.Reason = "Conflict of interest: implicated staff/school. Case transferred to Independent Supervisory Desk."
			decision.AssignedRouteTarget = "INDEPENDENT_SUPERVISORY_DESK"

			// Re-assign organization on case in database
			tx, txErr := h.pool.Begin(r.Context())
			if txErr == nil {
				_, _ = tx.Exec(r.Context(), `
					UPDATE cases SET organization_id = $1, updated_at = NOW() WHERE id = $2
				`, independentOrgID, caseID)

				// Log audit event
				_, _ = tx.Exec(r.Context(), `
					INSERT INTO case_events (case_id, from_status, to_status, actor_id, actor_type, reason, case_version)
					VALUES ($1, $2, $2, $3, 'staff', $4, 1)
				`, caseID, status, staffClaims.StaffID, decision.Reason)

				_ = tx.Commit(r.Context())
			}
		case "caregiver_implicated":
			decision.Reason = "Caregiver implicated. Priority response escalation required."
			decision.AssignedRouteTarget = "CHILD_PROTECTION_FAST_TRACK"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(decision)
}

// HandleGetRoute retrieves current routing status for a case.
func (h *Handler) HandleGetRoute(w http.ResponseWriter, r *http.Request) {
	staffClaims := authMW.GetStaffClaims(r.Context())
	if staffClaims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Staff token required")
		return
	}

	caseIDStr := chi.URLParam(r, "caseID")
	caseID, err := uuid.Parse(caseIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_case_id", "Invalid case ID format")
		return
	}

	var orgID string
	var conflictFlag *string
	err = h.pool.QueryRow(r.Context(), `
		SELECT organization_id, conflict_flag FROM cases WHERE id = $1
	`, caseID).Scan(&orgID, &conflictFlag)

	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "case_not_found", "Case not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "db_error", "Database query failed")
		return
	}

	isIndependent := conflictFlag != nil && (*conflictFlag == "staff_implicated" || *conflictFlag == "school_implicated")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"case_id":              caseID.String(),
		"organization_id":      orgID,
		"conflict_flag":        conflictFlag,
		"is_independent_route": isIndependent,
	})
}

func writeError(w http.ResponseWriter, statusCode int, errCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    errCode,
			"message": message,
		},
	})
}
