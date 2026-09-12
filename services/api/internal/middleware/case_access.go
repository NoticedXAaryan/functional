package middleware

import (
	"context"
	"github.com/balsuraksha/api/internal/db"
)

// Case content requires supervision, an active assignment, or an explicit grant.
// Alert roles alone do not expose the private narrative or message thread.
func CanReadCase(ctx context.Context, pool *db.Pool, c *StaffClaims, id string) bool {
	if c == nil {
		return false
	}
	var allowed bool
	err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM cases c WHERE c.id=$1 AND c.organization_id=$2 AND
 ($3 IN ('supervisor','admin') OR EXISTS(SELECT 1 FROM assignments a WHERE a.case_id=c.id AND a.responder_id=$4 AND a.unassigned_at IS NULL)
 OR EXISTS(SELECT 1 FROM case_access_grants g WHERE g.case_id=c.id AND g.staff_id=$4 AND g.revoked_at IS NULL)))`, id, c.OrganizationID, c.Role, c.StaffID).Scan(&allowed)
	return err == nil && allowed
}
