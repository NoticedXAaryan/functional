// Package sessions implements private session creation, BIP-39 return secrets,
// explicit exit (Incognito guarantee), and return-secret-based case access.
package sessions

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"

	"github.com/balsuraksha/api/internal/config"
	"github.com/balsuraksha/api/internal/db"
	authMW "github.com/balsuraksha/api/internal/middleware"
)

// maxAccessAttempts is the brute-force limit for return-secret access.
const maxAccessAttempts = 10

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
	r.Delete("/{sessionID}", h.HandleEnd)
	return r
}

// HandleCreate creates a new private session.
// For with_return_access mode, generates a BIP-39 mnemonic and stores only the bcrypt hash.
func (h *Handler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EntryPointID string `json:"entry_point_id"`
		Mode         string `json:"mode"`
		Language     string `json:"language"`
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

	// Generate return secret for with_return_access mode
	var returnWords []string
	var returnSecretHash string

	if req.Mode == "with_return_access" {
		words, err := generateMnemonic(h.cfg.ReturnSecretWordCount)
		if err != nil {
			log.Error().Err(err).Msg("failed to generate return secret")
			writeError(w, http.StatusInternalServerError, "secret_generation_failed", "Could not generate return secret")
			return
		}
		returnWords = words

		// Hash the joined mnemonic phrase. Plaintext is discarded after response.
		phrase := strings.Join(words, " ")
		hashBytes, err := bcrypt.GenerateFromPassword([]byte(phrase), bcrypt.DefaultCost)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "hash_failed", "Could not hash return secret")
			return
		}
		returnSecretHash = string(hashBytes)
	}

	// Validate entry point if provided
	var entryPointID *string
	if req.EntryPointID != "" {
		var count int
		if err := h.pool.QueryRow(r.Context(),
			`SELECT COUNT(*) FROM entry_points WHERE id = $1 AND active = TRUE AND revoked_at IS NULL`,
			req.EntryPointID).Scan(&count); err != nil || count == 0 {
			writeError(w, http.StatusBadRequest, "invalid_entry_point", "Entry point not found or revoked")
			return
		}
		entryPointID = &req.EntryPointID
	}

	// Persist session
	_, err := h.pool.Exec(r.Context(), `
		INSERT INTO private_sessions (id, entry_point_id, mode, language, token_jti, return_secret_hash, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, sessionID, entryPointID, req.Mode, req.Language, jti, nullableString(returnSecretHash), expiresAt)
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
	if len(returnWords) > 0 {
		resp["return_secret_words"] = returnWords
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
		`UPDATE private_sessions SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`,
		sessionID)
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

// HandleReturnAccess validates BIP-39 words and returns a new session token for case access.
// Rate-limited: after maxAccessAttempts, the session is locked.
func (h *Handler) HandleReturnAccess(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Words []string `json:"words"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Words) != 4 {
		writeError(w, http.StatusBadRequest, "invalid_request", "Exactly 4 words required")
		return
	}

	phrase := strings.Join(req.Words, " ")

	// Find sessions with a return secret that haven't exceeded attempt limit
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, return_secret_hash, case_id, access_attempt_count
		FROM private_sessions
		WHERE mode = 'with_return_access'
		  AND return_secret_hash IS NOT NULL
		  AND revoked_at IS NULL
		  AND access_attempt_count < $1
	`, maxAccessAttempts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "")
		return
	}
	defer rows.Close()

	// Compare against all eligible sessions (constant-time per session)
	type candidate struct {
		id           string
		hash         string
		caseID       *string
		attemptCount int
	}
	var candidates []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.id, &c.hash, &c.caseID, &c.attemptCount); err != nil {
			continue
		}
		candidates = append(candidates, c)
	}

	var matched *candidate
	for i := range candidates {
		if err := bcrypt.CompareHashAndPassword([]byte(candidates[i].hash), []byte(phrase)); err == nil {
			matched = &candidates[i]
			break
		}
		// Increment attempt count for any checked candidate (mild brute-force signal)
		// Only increment for the one we're checking, but in practice we check all.
	}

	if matched == nil {
		// Increment attempt count on all candidates to make enumeration harder
		_, _ = h.pool.Exec(r.Context(),
			`UPDATE private_sessions SET access_attempt_count = access_attempt_count + 1
			 WHERE mode = 'with_return_access' AND revoked_at IS NULL AND access_attempt_count < $1`,
			maxAccessAttempts)
		writeError(w, http.StatusUnauthorized, "invalid_secret", "Invalid return words")
		return
	}

	if matched.caseID == nil {
		writeError(w, http.StatusNotFound, "no_case", "No submitted case found for this secret")
		return
	}

	// Issue a new short-lived session token for case access
	jti := uuid.New()
	expiresAt := time.Now().Add(sessionTTL)
	tokenStr, err := issueSessionJWTWithCase(h.cfg.SessionSecret, matched.id, jti.String(), *matched.caseID, expiresAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token_error", "Could not issue token")
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"session_token": tokenStr,
		"case_id":       *matched.caseID,
		"session_id":    matched.id,
		"expires_at":    expiresAt,
	})
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

// generateMnemonic generates a cryptographically random BIP-39 mnemonic of wordCount words.
func generateMnemonic(wordCount int) ([]string, error) {
	words := make([]string, wordCount)
	wordlistLen := big.NewInt(int64(len(bip39Wordlist)))

	for i := 0; i < wordCount; i++ {
		idx, err := rand.Int(rand.Reader, wordlistLen)
		if err != nil {
			return nil, fmt.Errorf("crypto/rand failed: %w", err)
		}
		words[i] = bip39Wordlist[idx.Int64()]
	}
	return words, nil
}

// validateMnemonicWords returns true if all words are in the BIP-39 wordlist.
func validateMnemonicWords(words []string) bool {
	set := make(map[string]struct{}, len(bip39Wordlist))
	for _, w := range bip39Wordlist {
		set[w] = struct{}{}
	}
	for _, w := range words {
		if _, ok := set[strings.ToLower(w)]; !ok {
			return false
		}
	}
	return true
}

// Context-safe request-id extraction
func requestID(ctx context.Context) string {
	// chi middleware.RequestID stores under "requestID" key
	if id, ok := ctx.Value("requestID").(string); ok {
		return id
	}
	return ""
}
