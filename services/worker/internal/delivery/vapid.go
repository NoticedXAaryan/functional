// Package delivery — Web Push (VAPID) provider.
// RFC 8030 Web Push + RFC 8292 VAPID (Voluntary Application Server Identification).
//
// VAPID uses an EC key pair (P-256). The public key is shared with browsers
// for PushSubscription creation. The private key signs each request JWT.
// Neither key is stored in the database or included in logs.
//
// IMPORTANT: provider acceptance (HTTP 201) is NOT proof of device delivery.
// OS delivery, user interaction, and notification open are separate unconfirmed events.
package delivery

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// VAPIDProvider sends Web Push notifications using VAPID authentication.
type VAPIDProvider struct {
	privateKey *ecdsa.PrivateKey
	publicKey  string // base64url-encoded uncompressed P-256 public key (65 bytes)
	subject    string // VAPID subject — "mailto:contact@example.org" or "https://..."
	httpClient *http.Client
}

// NewVAPIDProvider creates a VAPIDProvider from a base64url-encoded raw P-256 private key.
// privateKeyB64: 32-byte P-256 private key scalar encoded as base64url (no padding).
// subject: VAPID contact URI, e.g. "mailto:alerts@balsuraksha.org".
func NewVAPIDProvider(privateKeyB64, subject string) (*VAPIDProvider, error) {
	privBytes, err := base64.RawURLEncoding.DecodeString(privateKeyB64)
	if err != nil {
		return nil, fmt.Errorf("vapid: decode private key: %w", err)
	}
	if len(privBytes) != 32 {
		return nil, fmt.Errorf("vapid: expected 32-byte P-256 private key, got %d bytes", len(privBytes))
	}

	curve := elliptic.P256()
	d := new(big.Int).SetBytes(privBytes)
	privKey := &ecdsa.PrivateKey{
		D:         d,
		PublicKey: ecdsa.PublicKey{Curve: curve},
	}
	privKey.PublicKey.X, privKey.PublicKey.Y = curve.ScalarBaseMult(privBytes)

	// Encode uncompressed public key: 0x04 || X(32) || Y(32)
	pubRaw := make([]byte, 65)
	pubRaw[0] = 0x04
	xb := privKey.PublicKey.X.Bytes()
	yb := privKey.PublicKey.Y.Bytes()
	copy(pubRaw[1+32-len(xb):33], xb)
	copy(pubRaw[33+32-len(yb):65], yb)

	return &VAPIDProvider{
		privateKey: privKey,
		publicKey:  base64.RawURLEncoding.EncodeToString(pubRaw),
		subject:    subject,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}, nil
}

// Name returns the provider identifier.
func (v *VAPIDProvider) Name() string { return "vapid-webpush" }

// PublicKey returns the VAPID public key in base64url format.
// This value must be passed to the browser's PushManager.subscribe({ applicationServerKey }).
func (v *VAPIDProvider) PublicKey() string { return v.publicKey }

// Send dispatches a Web Push notification to a browser subscription endpoint.
// endpoint is the pushSubscription.endpoint from the browser Push API.
// Returns (httpStatusStr, isProviderAccepted, error).
func (v *VAPIDProvider) Send(ctx context.Context, endpoint string, payload NotificationPayload) (string, bool, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", false, fmt.Errorf("vapid: marshal payload: %w", err)
	}

	authHeader, err := v.vapidAuthHeader(endpoint)
	if err != nil {
		return "", false, fmt.Errorf("vapid: auth header: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", false, fmt.Errorf("vapid: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader)
	// TTL: cap at 24h; the caller should further cap at remaining alert lifetime
	req.Header.Set("TTL", "86400")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		// Network error — outcome is ambiguous; caller records as AMBIGUOUS
		return "NETWORK_ERROR", false, fmt.Errorf("vapid: http send: %w", err)
	}
	defer resp.Body.Close()

	statusStr := fmt.Sprintf("HTTP_%d", resp.StatusCode)
	// RFC 8030 §8.2: 201 Created means the push service accepted the message
	isAccepted := resp.StatusCode == http.StatusCreated

	log.Info().
		Str("provider", "vapid-webpush").
		Str("endpoint_prefix", safePrefix(endpoint, 40)).
		Int("http_status", resp.StatusCode).
		Bool("is_provider_accepted", isAccepted).
		Bool("is_test", payload.IsTest).
		Msg("web push: provider response recorded — HTTP 201 is acceptance only, NOT device delivery proof")

	if !isAccepted {
		return statusStr, false, nil
	}
	return statusStr, true, nil
}

// vapidAuthHeader generates the VAPID `vapid t=<jwt>,k=<pubkey>` header.
// JWT is signed with ES256 (ECDSA P-256 + SHA-256).
func (v *VAPIDProvider) vapidAuthHeader(endpoint string) (string, error) {
	audience := extractVAPIDAudience(endpoint)
	exp := time.Now().Add(12 * time.Hour).Unix()

	headerJSON := `{"typ":"JWT","alg":"ES256"}`
	claimsJSON := fmt.Sprintf(`{"aud":%q,"exp":%d,"sub":%q}`, audience, exp, v.subject)

	headerB64 := base64.RawURLEncoding.EncodeToString([]byte(headerJSON))
	claimsB64 := base64.RawURLEncoding.EncodeToString([]byte(claimsJSON))
	signingInput := headerB64 + "." + claimsB64

	digest := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, v.privateKey, digest[:])
	if err != nil {
		return "", fmt.Errorf("vapid: sign: %w", err)
	}

	// Encode as fixed-width R || S (32 bytes each)
	sig := make([]byte, 64)
	rb, sb := r.Bytes(), s.Bytes()
	copy(sig[32-len(rb):32], rb)
	copy(sig[64-len(sb):64], sb)

	jwt := signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
	return fmt.Sprintf("vapid t=%s,k=%s", jwt, v.publicKey), nil
}

// extractVAPIDAudience extracts the scheme+host from a push endpoint URL.
// e.g. "https://fcm.googleapis.com/fcm/send/abc" → "https://fcm.googleapis.com"
func extractVAPIDAudience(endpoint string) string {
	idx := strings.Index(endpoint, "//")
	if idx < 0 {
		return endpoint
	}
	rest := endpoint[idx+2:]
	if slashIdx := strings.Index(rest, "/"); slashIdx >= 0 {
		return endpoint[:idx+2+slashIdx]
	}
	return endpoint
}
