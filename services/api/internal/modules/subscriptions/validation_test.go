package subscriptions

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func TestSubscriptionDestinationBoundaries(t *testing.T) {
	for _, tc := range []struct {
		url string
		ok  bool
	}{{"https://fcm.googleapis.com/path", true}, {"https://updates.push.services.mozilla.com/path", true}, {"https://web.push.apple.com/path", true}, {"http://127.0.0.1/private", false}, {"https://fcm.googleapis.com.evil.test/path", false}, {"https://fcm.googleapis.com:8443/path", false}, {"https://user@fcm.googleapis.com/path", false}, {"https://example.test/path", false}} {
		if validPushEndpoint(tc.url) != tc.ok {
			t.Errorf("incorrect endpoint decision: %s", tc.url)
		}
	}
}
func TestSubscriptionRequiresRealCurveKeys(t *testing.T) {
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth := base64.RawURLEncoding.EncodeToString(make([]byte, 16))
	public := base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes())
	if !validPushKeys(auth, public) {
		t.Fatal("valid key rejected")
	}
	if validPushKeys("invalid", public) || validPushKeys(auth, base64.RawURLEncoding.EncodeToString(make([]byte, 65))) {
		t.Fatal("malformed key accepted")
	}
}
