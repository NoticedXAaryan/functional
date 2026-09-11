package middleware

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/balsuraksha/api/internal/config"
	"github.com/balsuraksha/api/internal/db"
	"github.com/coreos/go-oidc/v3/oidc"
)

type oidcVerifier interface {
	Verify(context.Context, string) (*oidc.IDToken, error)
}

type staffLookup func(context.Context, string, string) (*StaffClaims, error)

// NewOIDCStaffMiddleware delegates discovery, signatures, issuer, audience and
// expiry verification to go-oidc. Identity-provider roles never grant app access.
func NewOIDCStaffMiddleware(ctx context.Context, cfg *config.Config, pool *db.Pool) (func(http.Handler) http.Handler, error) {
	if pool == nil {
		return nil, errors.New("OIDC staff lookup requires a database")
	}
	provider, err := oidc.NewProvider(oidc.ClientContext(ctx, &http.Client{Timeout: 10 * time.Second}), cfg.OIDCIssuer)
	if err != nil {
		return nil, err
	}
	verifier := provider.Verifier(&oidc.Config{ClientID: cfg.OIDCAudience, SupportedSigningAlgs: []string{"RS256"}})
	lookup := func(ctx context.Context, issuer, subject string) (*StaffClaims, error) {
		claims := &StaffClaims{}
		err := pool.QueryRow(ctx, `
			SELECT sm.id, sm.organization_id, rg.role
			FROM staff_identities si
			JOIN staff_members sm ON sm.id = si.staff_id AND sm.active = TRUE
			JOIN role_grants rg ON rg.staff_id = sm.id
			  AND rg.organization_id = sm.organization_id AND rg.revoked_at IS NULL
			WHERE si.issuer = $1 AND si.subject = $2 AND si.revoked_at IS NULL
			ORDER BY CASE rg.role WHEN 'admin' THEN 0 WHEN 'supervisor' THEN 1
			  WHEN 'alert_approver' THEN 2 WHEN 'alert_preparer' THEN 3 ELSE 4 END, rg.id
			LIMIT 1`, issuer, subject).Scan(&claims.StaffID, &claims.OrganizationID, &claims.Role)
		return claims, err
	}
	return requireOIDCStaff(verifier, cfg.OIDCIssuer, cfg.OIDCClientID, lookup), nil
}

func requireOIDCStaff(verifier oidcVerifier, issuer, clientID string, lookup staffLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			raw := extractBearer(r)
			if raw == "" {
				writeError(w, http.StatusUnauthorized, "missing_staff_token", "Organization sign-in required")
				return
			}
			token, err := verifier.Verify(r.Context(), raw)
			if err != nil || token == nil || token.Subject == "" {
				writeError(w, http.StatusUnauthorized, "invalid_staff_token", "Organization session is invalid or expired")
				return
			}
			// Keycloak's access-token profile distinguishes API access from ID tokens.
			var access struct {
				Type            string `json:"typ"`
				AuthorizedParty string `json:"azp"`
			}
			if token.Claims(&access) != nil || access.Type != "Bearer" || access.AuthorizedParty != clientID {
				writeError(w, http.StatusUnauthorized, "invalid_access_token", "An access token for this application is required")
				return
			}
			claims, err := lookup(r.Context(), issuer, token.Subject)
			if err != nil || claims == nil || claims.StaffID == "" || claims.OrganizationID == "" || claims.Role == "" {
				writeError(w, http.StatusForbidden, "staff_access_not_granted", "No active staff access is assigned")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), staffClaimsKey, claims)))
		})
	}
}
