// Package sessions implements short-lived private sessions and high-entropy return capabilities.
package sessions

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/balsuraksha/api/internal/config"
	"github.com/balsuraksha/api/internal/db"
	authMW "github.com/balsuraksha/api/internal/middleware"
)

// sessionTTL is the active session lifetime. Inactivity also causes expiry server-side.
const sessionTTL = 15 * time.Minute

// Handler handles all session-related routes.
type Handler struct {
	pool *db.Pool
	cfg  *config.Config
}

// NewHandler creates a session Handler.
func NewHandler(pool *db.Pool, cfg *config.Config) *Handler {
	return &Handler{pool: pool, cfg: cfg}
}

// Routes registers session routes.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Post("/", h.HandleCreate)
	// DELETE is mounted behind session authentication in the server router.
	return r
}

// HandleCreate creates a new private session.
// Return access uses a random 256-bit code; only its digest is stored.
func (h *Handler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var req struct {
		EntryPointID   string `json:"entry_point_id"`
		InvitationCode string `json:"invitation_code"`
		Mode           string `json:"mode"`
		Language       string `json:"language"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON")
		return
	}
	if req.Mode != "one_time" && req.Mode != "with_return_access" {
		writeError(w, http.StatusBadRequest, "invalid_mode", "mode must be one_time or with_return_access")
		return
	}
	if req.Language == "" {
		req.Language = "en"
	}

	sessionID := uuid.New()
	jti := uuid.New()
	expiresAt := time.Now().Add(sessionTTL)

	var returnCode, returnDigest string
	if req.Mode == "with_return_access" {
		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			writeError(w, 500, "random_failed", "Could not start; try again")
			return
		}
		returnCode = hex.EncodeToString(raw)
		returnDigest = digestCode(returnCode)
	}
	if h.cfg.AppMode != config.AppModeDemo {
		invitation := os.Getenv("BETA_INVITATION_CODE")
		if os.Getenv("BETA_INTAKE_ENABLED") != "true" || len(invitation) < 24 {
			writeError(w, 503, "intake_unavailable", "Private reporting is not open yet")
			return
		}
		if subtle.ConstantTimeCompare([]byte(req.InvitationCode), []byte(invitation)) != 1 {
			writeError(w, 403, "invitation_required", "Enter the invitation from your support organization")
			return
		}
		if req.EntryPointID == "" {
			req.EntryPointID = os.Getenv("BETA_ENTRY_POINT_ID")
		}
		if req.EntryPointID == "" {
			writeError(w, 503, "route_unavailable", "No support organization is configured")
			return
		}
	}

	// Validate entry point if provided
	var entryPointID *string
	if req.EntryPointID != "" {
		var count int
		if err := h.pool.QueryRow(r.Context(),
			`SELECT COUNT(*) FROM entry_points ep JOIN organizations o ON o.id=ep.organization_id WHERE ep.id = $1 AND ep.active = TRUE AND ep.revoked_at IS NULL AND o.active=TRUE`,
			req.EntryPointID).Scan(&count); err != nil || count == 0 {
			writeError(w, http.StatusBadRequest, "invalid_entry_point", "Entry point not found or revoked")
			return
		}
		entryPointID = &req.EntryPointID
	}

	// Persist session
	_, err := h.pool.Exec(r.Context(), `
		INSERT INTO private_sessions (id, entry_point_id, mode, language, token_jti, return_code_digest, expires_at, return_expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW() + INTERVAL '30 days')
	`, sessionID, entryPointID, req.Mode, req.Language, jti, nullableString(returnDigest), expiresAt)
	if err != nil {
		log.Error().Err(err).Msg("failed to persist session")
		writeError(w, http.StatusInternalServerError, "db_error", "Could not create session")
		return
	}

	// Issue JWT
	tokenStr, err := issueSessionJWT(h.cfg.SessionSecret, sessionID.String(), jti.String(), expiresAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token_error", "Could not issue token")
		return
	}

	resp := map[string]interface{}{
		"session_id":    sessionID.String(),
		"session_token": tokenStr,
		"expires_at":    expiresAt,
	}
	if returnCode != "" {
		resp["return_code"] = returnCode
		resp["return_expires_at"] = time.Now().Add(30 * 24 * time.Hour)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// HandleEnd explicitly ends a session (Incognito exit).
// Revokes the session credential. The submitted case (if any) is NOT deleted.
func (h *Handler) HandleEnd(w http.ResponseWriter, r *http.Request) {
	claims := authMW.GetSessionClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Valid session token required")
		return
	}

	sessionID := chi.URLParam(r, "sessionID")
	if sessionID != claims.SessionID {
		writeError(w, http.StatusForbidden, "forbidden", "Cannot end another session")
		return
	}

	result, err := h.pool.Exec(r.Context(),
		`UPDATE private_sessions SET revoked_at = NOW() WHERE id = $1 AND token_jti=$2 AND revoked_at IS NULL`,
		sessionID, claims.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "Could not end session")
		return
	}
	if result.RowsAffected() == 0 {
		// Already revoked — still return 204 (idempotent)
	}

	// Log the exit for audit
	log.Info().Str("session_id", sessionID).Msg("session explicitly ended by user")

	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

// HandleReturnAccess uses one indexed lookup; guesses never mutate other reports.
// Returning rotates the active token, restores its TTL and preserves a fixed return deadline.
func (h *Handler) HandleReturnAccess(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var req struct {
		Code string `json:"return_code"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeError(w, 400, "invalid_request", "Enter your return code")
		return
	}
	code := strings.ToLower(strings.Join(strings.Fields(req.Code), ""))
	code = strings.ReplaceAll(code, "-", "")
	raw, err := hex.DecodeString(code)
	if err != nil || len(raw) != 32 {
		writeError(w, 401, "invalid_code", "Return code is invalid or expired")
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		writeError(w, 503, "db_error", "Try again later")
		return
	}
	defer tx.Rollback(r.Context())
	var sessionID, caseID string
	err = tx.QueryRow(r.Context(), `SELECT id,case_id FROM private_sessions WHERE return_code_digest=$1 AND mode='with_return_access' AND case_id IS NOT NULL AND return_expires_at>NOW() FOR UPDATE`, digestCode(code)).Scan(&sessionID, &caseID)
	if err != nil {
		writeError(w, 401, "invalid_code", "Return code is invalid or expired")
		return
	}
	jti := uuid.New().String()
	expires := time.Now().Add(sessionTTL)
	token, err := issueSessionJWTWithCase(h.cfg.SessionSecret, sessionID, jti, caseID, expires)
	if err != nil {
		writeError(w, 503, "token_error", "Try again later")
		return
	}
	if _, err = tx.Exec(r.Context(), `UPDATE private_sessions SET token_jti=$2,expires_at=$3,revoked_at=NULL WHERE id=$1`, sessionID, jti, expires); err != nil {
		writeError(w, 503, "db_error", "Try again later")
		return
	}
	if tx.Commit(r.Context()) != nil {
		writeError(w, 503, "db_error", "Try again later")
		return
	}
	writeJSON(w, 200, map[string]interface{}{"session_token": token, "session_id": sessionID, "case_id": caseID, "expires_at": expires})
}
func digestCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

// ── Helpers ────────────────────────────────────────────────────────────────

func issueSessionJWT(secret, sessionID, jti string, expiresAt time.Time) (string, error) {
	claims := &authMW.SessionClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        jti,
		},
		SessionID: sessionID,
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString([]byte(secret))
}

func issueSessionJWTWithCase(secret, sessionID, jti, caseID string, expiresAt time.Time) (string, error) {
	claims := &authMW.SessionClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        jti,
		},
		SessionID: sessionID,
		CaseID:    caseID,
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString([]byte(secret))
}

func nullableString(s string) interface{} {
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
