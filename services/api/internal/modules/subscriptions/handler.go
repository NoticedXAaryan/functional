package subscriptions

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"github.com/balsuraksha/api/internal/config"
	"github.com/balsuraksha/api/internal/db"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"net/http"
	"os"
	"strings"
)

type Handler struct {
	pool *db.Pool
	cfg  *config.Config
}

func NewHandler(pool *db.Pool, cfg *config.Config) *Handler { return &Handler{pool, cfg} }
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/config", h.HandleConfig)
	r.Post("/", h.HandleCreate)
	r.Delete("/{subscriptionID}", h.HandleRevoke)
	return r
}
func (h *Handler) HandleConfig(w http.ResponseWriter, r *http.Request) {
	test := h.cfg.AppMode == config.AppModeDemo || h.cfg.IsTestMode()
	rows, err := h.pool.Query(r.Context(), `SELECT id,name FROM alert_areas WHERE active AND is_test=$1 ORDER BY name`, test)
	if err != nil {
		writeError(w, 503, "unavailable", "Area registration is unavailable")
		return
	}
	defer rows.Close()
	areas := []map[string]string{}
	for rows.Next() {
		var id, name string
		if rows.Scan(&id, &name) != nil {
			writeError(w, 503, "unavailable", "Area registration is unavailable")
			return
		}
		areas = append(areas, map[string]string{"id": id, "name": name})
	}
	if rows.Err() != nil {
		writeError(w, 503, "unavailable", "Area registration is unavailable")
		return
	}
	key := os.Getenv("VAPID_PUBLIC_KEY")
	ready := h.cfg.IsSendingAllowed() && key != "" && os.Getenv("VAPID_PRIVATE_KEY") != "" && (!test || os.Getenv("TEST_ENROLLMENT_CODE") != "")
	writeJSON(w, 200, map[string]interface{}{"brand": "Savera Alert", "mode": h.cfg.NotificationMode, "is_test": test, "enrollment_available": ready, "public_key": key, "areas": areas})
}
func (h *Handler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	r.Body = http.MaxBytesReader(w, r.Body, 16384)
	var req struct {
		AreaID         string `json:"area_id"`
		Endpoint       string `json:"endpoint"`
		Auth           string `json:"auth"`
		P256DH         string `json:"p256dh"`
		Consent        bool   `json:"consent_confirmed"`
		Secret         string `json:"management_secret"`
		EnrollmentCode string `json:"enrollment_code"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeError(w, 400, "invalid_request", "Invalid subscription")
		return
	}
	if !h.cfg.IsSendingAllowed() || os.Getenv("VAPID_PUBLIC_KEY") == "" || os.Getenv("VAPID_PRIVATE_KEY") == "" {
		writeError(w, 503, "enrollment_disabled", "Notifications are not configured yet")
		return
	}
	test := h.cfg.AppMode == config.AppModeDemo || h.cfg.IsTestMode()
	if test {
		expected := os.Getenv("TEST_ENROLLMENT_CODE")
		if expected == "" || subtle.ConstantTimeCompare([]byte(expected), []byte(req.EnrollmentCode)) != 1 {
			writeError(w, 403, "test_invitation_required", "Enter the test invitation supplied by the organizer")
			return
		}
	}
	secret, err := base64.RawURLEncoding.DecodeString(req.Secret)
	if err != nil || len(secret) != 32 || !req.Consent || !validPushEndpoint(req.Endpoint) || !validPushKeys(req.Auth, req.P256DH) {
		writeError(w, 400, "invalid_subscription", "Explicit consent and a valid browser subscription are required")
		return
	}
	digest := sha256.Sum256(secret)
	hash := hex.EncodeToString(digest[:])
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		writeError(w, 503, "unavailable", "Registration unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(7182401)`); err != nil {
		writeError(w, 503, "unavailable", "Registration unavailable")
		return
	}
	var name string
	if err = tx.QueryRow(r.Context(), `SELECT name FROM alert_areas WHERE id=$1 AND active AND is_test=$2`, req.AreaID, test).Scan(&name); err != nil {
		writeError(w, 400, "invalid_area", "Choose an available registered area")
		return
	}
	var id, existingHash string
	err = tx.QueryRow(r.Context(), `SELECT id,management_secret_hash FROM area_subscriptions WHERE endpoint=$1 AND revoked_at IS NULL AND management_secret_hash IS NOT NULL AND is_test=$2`, req.Endpoint, test).Scan(&id, &existingHash)
	if err == nil {
		if subtle.ConstantTimeCompare([]byte(hash), []byte(existingHash)) != 1 {
			writeError(w, 409, "subscription_exists", "This browser is already registered. Use its original subscription controls.")
			return
		}
		_, err = tx.Exec(r.Context(), `UPDATE area_subscriptions SET area_id=$1,area_name=$2,auth=$3,p256dh=$4,consent_confirmed_at=NOW() WHERE id=$5`, req.AreaID, name, req.Auth, req.P256DH, id)
	} else if err == pgx.ErrNoRows {
		if test {
			var count int
			if tx.QueryRow(r.Context(), `SELECT count(*) FROM area_subscriptions WHERE revoked_at IS NULL AND is_test AND management_secret_hash IS NOT NULL`).Scan(&count) != nil {
				writeError(w, 503, "unavailable", "Registration unavailable")
				return
			}
			if count >= h.cfg.TestAllowlistMax {
				writeError(w, 409, "test_capacity", "The test group is full")
				return
			}
		}
		id = uuid.NewString()
		_, err = tx.Exec(r.Context(), `INSERT INTO area_subscriptions(id,area_id,area_name,endpoint,auth,p256dh,consent_confirmed_at,is_test,management_secret_hash) VALUES($1,$2,$3,$4,$5,$6,NOW(),$7,$8)`, id, req.AreaID, name, req.Endpoint, req.Auth, req.P256DH, test, hash)
	}
	if err != nil {
		writeError(w, 503, "unavailable", "Could not register subscription")
		return
	}
	if tx.Commit(r.Context()) != nil {
		writeError(w, 503, "unavailable", "Registration was not confirmed; retry")
		return
	}
	writeJSON(w, 201, map[string]interface{}{"subscription_id": id, "area_id": req.AreaID, "area_name": name, "is_test": test})
}
func (h *Handler) HandleRevoke(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	id := chi.URLParam(r, "subscriptionID")
	if _, err := uuid.Parse(id); err != nil {
		writeError(w, 400, "invalid_id", "Invalid subscription")
		return
	}
	raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	secret, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(secret) != 32 {
		writeError(w, 401, "subscription_access_required", "Subscription access is required")
		return
	}
	digest := sha256.Sum256(secret)
	result, err := h.pool.Exec(r.Context(), `UPDATE area_subscriptions SET revoked_at=COALESCE(revoked_at,NOW()) WHERE id=$1 AND management_secret_hash=$2`, id, hex.EncodeToString(digest[:]))
	if err != nil {
		writeError(w, 503, "unavailable", "Could not stop notifications; retry")
		return
	}
	if result.RowsAffected() != 1 {
		writeError(w, 404, "not_found", "Subscription access not found")
		return
	}
	w.WriteHeader(204)
}
func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{"error": msg, "code": code})
}
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
