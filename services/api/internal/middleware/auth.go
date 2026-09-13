// Package middleware provides HTTP middleware for the Bal Suraksha API.
// It handles session authentication, staff authentication, mode guards, and rate limiting.
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/balsuraksha/api/internal/config"
	"github.com/balsuraksha/api/internal/db"
)

// contextKey is an unexported type for context keys in this package.
type contextKey string

const (
	sessionClaimsKey contextKey = "session_claims"
	staffClaimsKey   contextKey = "staff_claims"

	// Staff session cookies
	staffSessionCookie = "bs_staff_session"
	csrfCookieName     = "bs_csrf"
	csrfHeaderName     = "X-CSRF-Token"
	staffSessionMaxAge = 8 * time.Hour  // sliding window per request
	staffSessionAbsMax = 24 * time.Hour // hard limit from creation
)

// SessionClaims are the JWT claims for a private session.
type SessionClaims struct {
	jwt.RegisteredClaims
	SessionID string `json:"sid"`
	CaseID    string `json:"cid,omitempty"`
}

// StaffClaims carries authenticated staff identity through request context.
// Populated by RequireStaffSession (cookie auth) or RequireStaffToken (legacy JWT).
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

// StaffAuthHandler handles staff login, logout, and session identity.
type StaffAuthHandler struct {
	pool *db.Pool
	cfg  *config.Config
}

// NewStaffAuthHandler creates a StaffAuthHandler.
func NewStaffAuthHandler(pool *db.Pool, cfg *config.Config) *StaffAuthHandler {
	return &StaffAuthHandler{pool: pool, cfg: cfg}
}

// generateCSRFToken creates a cryptographically random 32-byte hex token.
func generateCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// staffCookieSecure returns true when cookies must carry the Secure flag.
func staffCookieSecure(cfg *config.Config) bool {
	return cfg.AppMode != config.AppModeDemo
}

// setStaffSessionCookies writes the HttpOnly session cookie and readable CSRF cookie.
func setStaffSessionCookies(w http.ResponseWriter, cfg *config.Config, sessionID, csrfToken string) {
	http.SetCookie(w, &http.Cookie{
		Name:     staffSessionCookie,
		Value:    sessionID,
		Path:     "/api/v1/staff",
		HttpOnly: true,
		Secure:   staffCookieSecure(cfg),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(staffSessionMaxAge.Seconds()),
	})
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    csrfToken,
		Path:     "/",
		HttpOnly: false, // JavaScript reads this to send as X-CSRF-Token header
		Secure:   staffCookieSecure(cfg),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(staffSessionMaxAge.Seconds()),
	})
}

// clearStaffSessionCookies expires both session cookies.
func clearStaffSessionCookies(w http.ResponseWriter, cfg *config.Config) {
	http.SetCookie(w, &http.Cookie{
		Name:     staffSessionCookie,
		Value:    "",
		Path:     "/api/v1/staff",
		HttpOnly: true,
		Secure:   staffCookieSecure(cfg),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: false,
		Secure:   staffCookieSecure(cfg),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// HandleLogin authenticates staff and creates a server-side session stored in an HttpOnly cookie.
// The session survives page refresh because the browser sends the cookie automatically.
func (h *StaffAuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
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

	// Look up staff member and their highest-priority active role.
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
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid username or password")
		return
	}

	if err := verifyPassword(passwordHash, req.Password); err != nil {
		log.Warn().Str("username", req.Username).Msg("failed staff login attempt")
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid username or password")
		return
	}

	// Create server-side session.
	sessionID := uuid.New().String()
	csrfToken, err := generateCSRFToken()
	if err != nil {
		log.Error().Err(err).Msg("failed to generate CSRF token")
		writeError(w, http.StatusInternalServerError, "session_error", "Could not create session")
		return
	}

	expiresAt := time.Now().Add(time.Duration(h.cfg.StaffJWTExpiryHours) * time.Hour)
	_, err = h.pool.Exec(r.Context(), `
		INSERT INTO staff_sessions (id, staff_id, organization_id, role, csrf_token, ip_address, user_agent, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		sessionID, staffID, orgID, role, csrfToken,
		r.RemoteAddr, truncateUA(r.UserAgent()), expiresAt,
	)
	if err != nil {
		log.Error().Err(err).Msg("failed to create staff session")
		writeError(w, http.StatusInternalServerError, "session_error", "Could not create session")
		return
	}

	setStaffSessionCookies(w, h.cfg, sessionID, csrfToken)
	log.Info().Str("username", req.Username).Str("staff_id", staffID).Msg("staff login")

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"staff_id":        staffID,
		"role":            role,
		"organization_id": orgID,
	})
}

// HandleLogout revokes the active session and clears cookies.
func (h *StaffAuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	cookie, err := r.Cookie(staffSessionCookie)
	if err == nil && cookie.Value != "" {
		_, _ = h.pool.Exec(r.Context(),
			`UPDATE staff_sessions SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`,
			cookie.Value)
	}
	clearStaffSessionCookies(w, h.cfg)
	writeJSON(w, http.StatusOK, map[string]string{"status": "signed_out"})
}

// HandleMe returns the authenticated staff identity from the session cookie.
// This is the endpoint the frontend calls on page load to check for an existing session.
func (h *StaffAuthHandler) HandleMe(w http.ResponseWriter, r *http.Request) {
	claims := GetStaffClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "no_session", "Not signed in")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"staff_id":        claims.StaffID,
		"organization_id": claims.OrganizationID,
		"role":            claims.Role,
	})
}

// RequireStaffSession validates the HttpOnly session cookie against the database.
// It populates StaffClaims in context, identical to RequireStaffToken, so all
// downstream handlers work without changes.
func RequireStaffSession(cfg *config.Config, pool *db.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")

			cookie, err := r.Cookie(staffSessionCookie)
			if err != nil || cookie.Value == "" {
				writeError(w, http.StatusUnauthorized, "no_session", "Sign in required")
				return
			}

			// Validate session against DB.
			var staffID, orgID, role, storedCSRF string
			var expiresAt, createdAt time.Time
			var revokedAt *time.Time
			err = pool.QueryRow(r.Context(), `
				SELECT staff_id, organization_id, role, csrf_token,
				       expires_at, created_at, revoked_at
				FROM staff_sessions WHERE id = $1`,
				cookie.Value,
			).Scan(&staffID, &orgID, &role, &storedCSRF, &expiresAt, &createdAt, &revokedAt)
			if err != nil {
				clearStaffSessionCookies(w, cfg)
				writeError(w, http.StatusUnauthorized, "invalid_session", "Session not found")
				return
			}
			if revokedAt != nil {
				clearStaffSessionCookies(w, cfg)
				writeError(w, http.StatusUnauthorized, "session_revoked", "Session has ended")
				return
			}
			if time.Now().After(expiresAt) {
				clearStaffSessionCookies(w, cfg)
				writeError(w, http.StatusUnauthorized, "session_expired", "Session has expired. Please sign in again.")
				return
			}

			// CSRF check on state-changing methods.
			if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
				csrfHeader := r.Header.Get(csrfHeaderName)
				if csrfHeader == "" || csrfHeader != storedCSRF {
					writeError(w, http.StatusForbidden, "csrf_invalid", "Invalid or missing CSRF token")
					return
				}
			}

			// Sliding session extension, capped at absolute max from creation.
			newExpiry := time.Now().Add(staffSessionMaxAge)
			absMax := createdAt.Add(staffSessionAbsMax)
			if newExpiry.After(absMax) {
				newExpiry = absMax
			}
			_, _ = pool.Exec(r.Context(),
				`UPDATE staff_sessions SET last_active_at = NOW(), expires_at = $1 WHERE id = $2`,
				newExpiry, cookie.Value)

			claims := &StaffClaims{
				StaffID:        staffID,
				OrganizationID: orgID,
				Role:           role,
			}
			ctx := context.WithValue(r.Context(), staffClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// truncateUA limits user-agent to 512 chars to prevent DB bloat.
func truncateUA(ua string) string {
	if len(ua) > 512 {
		return ua[:512]
	}
	return ua
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
