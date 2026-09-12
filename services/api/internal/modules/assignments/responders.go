package assignments

import (
	authMW "github.com/balsuraksha/api/internal/middleware"
	"net/http"
)

func (h *Handler) HandleResponders(w http.ResponseWriter, r *http.Request) {
	c := authMW.GetStaffClaims(r.Context())
	if c == nil || (c.Role != "supervisor" && c.Role != "admin") {
		writeError(w, 403, "forbidden", "Supervisor access required")
		return
	}
	rows, err := h.pool.Query(r.Context(), `SELECT sm.id,sm.username FROM staff_members sm WHERE sm.active AND sm.organization_id=$1 AND EXISTS(SELECT 1 FROM role_grants sr WHERE sr.staff_id=sm.id AND sr.organization_id=sm.organization_id AND sr.revoked_at IS NULL AND sr.role='responder') ORDER BY sm.username`, c.OrganizationID)
	if err != nil {
		writeError(w, 503, "unavailable", "Could not load responders")
		return
	}
	defer rows.Close()
	result := []map[string]string{}
	for rows.Next() {
		var id, name string
		if rows.Scan(&id, &name) != nil {
			writeError(w, 503, "unavailable", "Could not load responders")
			return
		}
		result = append(result, map[string]string{"id": id, "name": name})
	}
	if rows.Err() != nil {
		writeError(w, 503, "unavailable", "Could not load responders")
		return
	}
	writeJSON(w, 200, map[string]interface{}{"responders": result})
}
