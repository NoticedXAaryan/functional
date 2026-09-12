package subscriptions

import (
	"crypto/ecdh"
	"encoding/base64"
	"net/url"
)

func validPushEndpoint(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" || (u.Port() != "" && u.Port() != "443") {
		return false
	}
	switch u.Hostname() {
	case "fcm.googleapis.com", "updates.push.services.mozilla.com", "web.push.apple.com":
		return u.Path != "" && len(raw) <= 4096
	default:
		return false
	}
}

func validPushKeys(auth, publicKey string) bool {
	secret, err := base64.RawURLEncoding.DecodeString(auth)
	if err != nil || len(secret) != 16 {
		return false
	}
	key, err := base64.RawURLEncoding.DecodeString(publicKey)
	if err != nil {
		return false
	}
	_, err = ecdh.P256().NewPublicKey(key)
	return err == nil
}
