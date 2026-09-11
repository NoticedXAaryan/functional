package middleware

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/balsuraksha/api/internal/config"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
)

func TestOIDCStaffBoundary(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	const issuer = "https://identity.example.test/realms/test"
	verifier := oidc.NewVerifier(issuer, &oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{&key.PublicKey}}, &oidc.Config{ClientID: "api"})
	tests := []struct {
		name    string
		change  func(jwt.MapClaims)
		revoked bool
		want    int
	}{
		{"active mapped staff", nil, false, 204},
		{"unmapped or revoked identity", nil, true, 403},
		{"wrong issuer", func(c jwt.MapClaims) { c["iss"] = "https://other.example.test" }, false, 401},
		{"wrong audience", func(c jwt.MapClaims) { c["aud"] = "other-api" }, false, 401},
		{"wrong authorized client", func(c jwt.MapClaims) { c["azp"] = "other-client" }, false, 401},
		{"ID token cannot call API", func(c jwt.MapClaims) { c["typ"] = "ID" }, false, 401},
		{"missing subject", func(c jwt.MapClaims) { delete(c, "sub") }, false, 401},
		{"expired token", func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Hour).Unix() }, false, 401},
		{"missing expiration", func(c jwt.MapClaims) { delete(c, "exp") }, false, 401},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			claims := jwt.MapClaims{"iss": issuer, "sub": "subject-1", "aud": "api", "azp": "ops", "typ": "Bearer", "exp": time.Now().Add(time.Hour).Unix(), "role": "admin"}
			if tc.change != nil {
				tc.change(claims)
			}
			raw, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
			if err != nil {
				t.Fatal(err)
			}
			lookup := func(_ context.Context, gotIssuer, subject string) (*StaffClaims, error) {
				if gotIssuer != issuer || subject != "subject-1" {
					t.Fatal("wrong identity lookup")
				}
				if tc.revoked {
					return nil, errors.New("no active grant")
				}
				return &StaffClaims{StaffID: "staff-1", OrganizationID: "org-1", Role: "responder"}, nil
			}
			handler := requireOIDCStaff(verifier, issuer, "ops", lookup)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if GetStaffClaims(r.Context()).Role != "responder" {
					t.Fatal("provider role must not grant app permissions")
				}
				w.WriteHeader(204)
			}))
			req := httptest.NewRequest("GET", "/staff/cases", nil)
			req.Header.Set("Authorization", "Bearer "+raw)
			out := httptest.NewRecorder()
			handler.ServeHTTP(out, req)
			if out.Code != tc.want {
				t.Fatalf("got %d, want %d: %s", out.Code, tc.want, out.Body.String())
			}
			if out.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("staff response must not be cached")
			}
		})
	}
}

func TestLocalLoginIsDemoOnly(t *testing.T) {
	for _, cfg := range []*config.Config{
		{AppMode: config.AppModeBeta, StaffAuthMode: "oidc"},
		{AppMode: config.AppModeProduction, StaffAuthMode: "oidc"},
		{AppMode: config.AppModeDemo, StaffAuthMode: "oidc"},
	} {
		out := httptest.NewRecorder()
		NewStaffAuthHandler(nil, cfg).HandleLogin(out, httptest.NewRequest("POST", "/staff/auth/login", nil))
		if out.Code != http.StatusForbidden {
			t.Fatalf("local login accepted in %s/%s", cfg.AppMode, cfg.StaffAuthMode)
		}
	}
}
