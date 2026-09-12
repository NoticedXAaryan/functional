// Package middleware provides HTTP middleware for the Bal Suraksha API.
// It handles session authentication, staff authentication, mode guards, and rate limiting.
package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog/log"

	"github.com/balsuraksha/api/internal/config"
	"github.com/balsuraksha/api/internal/db"
)

// contextKey is an unexported type for context keys in this package.
type contextKey string

const (
	sessionClaimsKey contextKey = "session_claims"
	staffClaimsKey   contextKey = "staff_claims"
)

// SessionClaims are the JWT claims for a private session.
type SessionClaims struct {
	jwt.RegisteredClaims
	SessionID string `json:"sid"`
	CaseID    string `json:"cid,omitempty"`
}

// StaffClaims are the JWT claims for an authenticated staff member.
type StaffClaims struct {
	jwt.RegisteredClaims
	StaffID        string `json:"staff_id"`
	OrganizationID string `json:"org_id"`
	Role           string `json:"role"`
}

// RequireSessionToken validates the Bearer JWT issued on session creation.
// Returns 401 if the token is missing, invalid, expired, or the session has been revoked.
// IMPORTANT: This checks revoked_at in the DB — a revoked token is rejected immediately,
// even if the JWT signature is still valid. This enforces the Incognito guarantee.
func RequireSessionToken(cfg *config.Config, pool ...*db.Pool) func(http.Handler) http.Handler {
	var dbPool *db.Pool
	if len(pool) > 0 {
		dbPool = pool[0]
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			tokenStr := extractBearer(r)
			if tokenStr == "" {
				writeError(w, http.StatusUnauthorized, "missing_session_token", "Authorization token required")
				return
			}

			claims := &SessionClaims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(cfg.SessionSecret), nil
			}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
			if err != nil || token == nil || !token.Valid || claims.SessionID == "" {
				writeError(w, http.StatusUnauthorized, "invalid_session_token", "Session token is invalid or expired")
				return
			}

			// Check DB revocation — the Incognito guarantee requires immediate revocation.
			// Only checked when a pool is provided (production always provides one).
			if dbPool != nil && claims.SessionID != "" {
				var revokedAt *time.Time
				var expiresAt time.Time
				var tokenJTI string
				err := dbPool.QueryRow(r.Context(),
					`SELECT revoked_at, expires_at, token_jti FROM private_sessions WHERE id = $1`,
					claims.SessionID,
				).Scan(&revokedAt, &expiresAt, &tokenJTI)
				if err != nil {
					log.Warn().Err(err).Str("session_id", claims.SessionID).Msg("session revocation check failed")
					writeError(w, http.StatusUnauthorized, "session_lookup_failed", "Could not verify session")
					return
				}
				if revokedAt != nil || claims.ID != tokenJTI {
					writeError(w, http.StatusUnauthorized, "session_revoked", "Session has been ended")
					return
				}
				if time.Now().After(expiresAt) {
					writeError(w, http.StatusUnauthorized, "session_expired", "Session has expired")
					return
				}
			}

			ctx := context.WithValue(r.Context(), sessionClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireStaffToken validates the Bearer JWT issued on staff login.
func RequireStaffToken(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			tokenStr := extractBearer(r)
			if tokenStr == "" {
				writeError(w, http.StatusUnauthorized, "missing_staff_token", "Staff authorization required")
				return
			}

			claims := &StaffClaims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(cfg.SessionSecret), nil
			}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
			// Session and staff tokens share the signing key today. A valid signature
			// alone must never promote a child's session into staff authorization.
			if err != nil || token == nil || !token.Valid || claims.StaffID == "" || claims.OrganizationID == "" || claims.Role == "" {
				writeError(w, http.StatusUnauthorized, "invalid_staff_token", "Staff token is invalid or expired")
				return
			}

			ctx := context.WithValue(r.Context(), staffClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetSessionClaims extracts session claims from context. Returns nil if not present.
func GetSessionClaims(ctx context.Context) *SessionClaims {
	c, _ := ctx.Value(sessionClaimsKey).(*SessionClaims)
	return c
}

// GetStaffClaims extracts staff claims from context. Returns nil if not present.
func GetStaffClaims(ctx context.Context) *StaffClaims {
	c, _ := ctx.Value(staffClaimsKey).(*StaffClaims)
	return c
}

// StaffAuthHandler handles the local-stub staff login endpoint.
type StaffAuthHandler struct {
	pool *db.Pool
	cfg  *config.Config
}

// NewStaffAuthHandler creates a StaffAuthHandler.
func NewStaffAuthHandler(pool *db.Pool, cfg *config.Config) *StaffAuthHandler {
	return &StaffAuthHandler{pool: pool, cfg: cfg}
}

// HandleLogin authenticates staff with username/password and returns a JWT.
// This is the local stub for demo mode. In production, replace with OIDC.
func (h *StaffAuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h.cfg.AppMode != config.AppModeDemo || h.cfg.StaffAuthMode == "oidc" {
		writeError(w, http.StatusForbidden, "use_organization_sign_in", "Sign in through your organization's identity provider")
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "missing_credentials", "username and password required")
		return
	}

	// Look up staff member
	var staffID, orgID, passwordHash, role string
	err := h.pool.QueryRow(r.Context(), `
		SELECT sm.id, sm.organization_id, sm.password_hash, rg.role
		FROM staff_members sm
		JOIN role_grants rg ON rg.staff_id = sm.id AND rg.organization_id = sm.organization_id AND rg.revoked_at IS NULL
		WHERE sm.username = $1 AND sm.active = TRUE
		ORDER BY CASE rg.role WHEN 'admin' THEN 0 WHEN 'supervisor' THEN 1 WHEN 'alert_approver' THEN 2 WHEN 'alert_preparer' THEN 3 ELSE 4 END, rg.id
		LIMIT 1
	`, req.Username).Scan(&staffID, &orgID, &passwordHash, &role)
	if err != nil {
		// Don't reveal whether username exists
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid username or password")
		return
	}

	// Verify bcrypt password
	if err := verifyPassword(passwordHash, req.Password); err != nil {
		log.Warn().Str("username", req.Username).Msg("failed login attempt")
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid username or password")
		return
	}

	expiresAt := time.Now().Add(time.Duration(h.cfg.StaffJWTExpiryHours) * time.Hour)
	claims := &StaffClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   staffID,
		},
		StaffID:        staffID,
		OrganizationID: orgID,
		Role:           role,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(h.cfg.SessionSecret))
	if err != nil {
		log.Error().Err(err).Msg("failed to sign staff JWT")
		writeError(w, http.StatusInternalServerError, "token_error", "Could not issue token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token":           tokenStr,
		"expires_at":      expiresAt,
		"staff_id":        staffID,
		"role":            role,
		"organization_id": orgID,
	})
}

// extractBearer extracts the Bearer token from the Authorization header.
func extractBearer(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(auth, "Bearer ")
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":      msg,
		"code":       code,
		"request_id": w.Header().Get("X-Request-Id"),
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
