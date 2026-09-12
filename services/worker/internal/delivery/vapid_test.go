package delivery

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	webpush "github.com/SherClockHolmes/webpush-go"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

type clientFunc func(*http.Request) (*http.Response, error)

func (f clientFunc) Do(r *http.Request) (*http.Response, error) { return f(r) }
func TestEncryptedPushUsesKeysAndExpiry(t *testing.T) {
	private, _, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	provider, err := NewVAPIDProvider(private, "mailto:synthetic@example.test")
	if err != nil {
		t.Fatal(err)
	}
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	provider.httpClient = clientFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		body, _ := io.ReadAll(r.Body)
		if r.Header.Get("Content-Encoding") != "aes128gcm" || bytes.Contains(body, []byte("PRIVATE-SENTINEL")) {
			t.Fatal("payload not encrypted")
		}
		ttl, err := strconv.Atoi(r.Header.Get("TTL"))
		if err != nil || ttl < 1 || ttl > 60 {
			t.Fatal("TTL exceeds alert lifetime")
		}
		if r.Header.Get("Authorization") == "" {
			t.Fatal("VAPID authentication missing")
		}
		return &http.Response{StatusCode: 201, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	sub := Subscription{Endpoint: "https://fcm.googleapis.com/synthetic", Auth: base64.RawURLEncoding.EncodeToString(make([]byte, 16)), P256DH: base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes())}
	payload := NotificationPayload{Body: "PRIVATE-SENTINEL", ExpiresAt: time.Now().Add(time.Minute)}
	status, accepted, err := provider.Send(context.Background(), sub, payload)
	if err != nil || !accepted || status != "HTTP_201" || calls != 1 {
		t.Fatalf("unexpected outcome %s %v %v", status, accepted, err)
	}
	sub.Endpoint = "http://127.0.0.1/private"
	_, accepted, _ = provider.Send(context.Background(), sub, payload)
	if accepted || calls != 1 {
		t.Fatal("untrusted endpoint reached transport")
	}
	sub.Endpoint = "https://fcm.googleapis.com/synthetic"
	payload.ExpiresAt = time.Now().Add(-time.Second)
	_, accepted, _ = provider.Send(context.Background(), sub, payload)
	if accepted || calls != 1 {
		t.Fatal("expired alert reached transport")
	}
}
func TestDeliveryOutcomePolicy(t *testing.T) {
	for _, tc := range []struct {
		status   string
		accepted bool
		attempt  int
		want     string
	}{{"HTTP_201", true, 1, "SENT"}, {"AMBIGUOUS", false, 1, "AMBIGUOUS"}, {"HTTP_429", false, 1, "QUEUED"}, {"HTTP_503", false, 3, "FAILED"}, {"HTTP_410", false, 1, "FAILED"}, {"CANCELLED", false, 1, "CANCELLED"}, {"SIMULATED", false, 1, "FAILED"}} {
		if got := outcome(tc.status, tc.accepted, tc.attempt); got != tc.want {
			t.Errorf("%s: %s != %s", tc.status, got, tc.want)
		}
	}
}
