package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/balsuraksha/api/internal/config"
	"github.com/golang-jwt/jwt/v5"
)

func TestTokenBoundaries(t *testing.T) {
	cfg := &config.Config{SessionSecret: "test-only-signing-secret-not-for-deployment"}
	expires := float64(time.Now().Add(time.Hour).Unix())
	tests := []struct {
		name   string
		staff  bool
		method jwt.SigningMethod
		claims jwt.MapClaims
		want   int
	}{
		{"session cannot become staff", true, jwt.SigningMethodHS256, jwt.MapClaims{"sid": "session-1", "exp": expires}, 401},
		{"staff cannot become session", false, jwt.SigningMethodHS256, jwt.MapClaims{"staff_id": "staff-1", "org_id": "org-1", "role": "responder", "exp": expires}, 401},
		{"staff requires organization", true, jwt.SigningMethodHS256, jwt.MapClaims{"staff_id": "staff-1", "role": "responder", "exp": expires}, 401},
		{"staff requires role", true, jwt.SigningMethodHS256, jwt.MapClaims{"staff_id": "staff-1", "org_id": "org-1", "exp": expires}, 401},
		{"session requires expiry", false, jwt.SigningMethodHS256, jwt.MapClaims{"sid": "session-1"}, 401},
		{"staff requires expiry", true, jwt.SigningMethodHS256, jwt.MapClaims{"staff_id": "staff-1", "org_id": "org-1", "role": "responder"}, 401},
		{"session rejects alternate HMAC", false, jwt.SigningMethodHS512, jwt.MapClaims{"sid": "session-1", "exp": expires}, 401},
		{"staff rejects alternate HMAC", true, jwt.SigningMethodHS512, jwt.MapClaims{"staff_id": "staff-1", "org_id": "org-1", "role": "responder", "exp": expires}, 401},
		{"valid session", false, jwt.SigningMethodHS256, jwt.MapClaims{"sid": "session-1", "exp": expires}, 204},
		{"valid staff", true, jwt.SigningMethodHS256, jwt.MapClaims{"staff_id": "staff-1", "org_id": "org-1", "role": "responder", "exp": expires}, 204},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			token, err := jwt.NewWithClaims(tc.method, tc.claims).SignedString([]byte(cfg.SessionSecret))
			if err != nil {
				t.Fatal(err)
			}
			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if tc.staff && GetStaffClaims(r.Context()) == nil {
					t.Fatal("staff claims missing from authorized request")
				}
				w.WriteHeader(http.StatusNoContent)
			})
			wrap := RequireSessionToken(cfg)
			if tc.staff {
				wrap = RequireStaffToken(cfg)
			}
			req := httptest.NewRequest(http.MethodGet, "/private", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			res := httptest.NewRecorder()
			wrap(next).ServeHTTP(res, req)
			if res.Code != tc.want || called != (tc.want == 204) {
				t.Fatalf("status=%d downstream=%v, want %d", res.Code, called, tc.want)
			}
			if res.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("private authentication responses must not be cached")
			}
		})
	}
}
