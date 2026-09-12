package delivery

import (
	"context"
	"crypto/ecdh"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	webpush "github.com/SherClockHolmes/webpush-go"
	"net/http"
	"net/url"
	"time"
)

type VAPIDProvider struct {
	privateKey, publicKey, subject string
	httpClient                     webpush.HTTPClient
}

func NewVAPIDProvider(privateKey, subject string) (*VAPIDProvider, error) {
	raw, err := base64.RawURLEncoding.DecodeString(privateKey)
	if err != nil {
		return nil, errors.New("invalid VAPID key")
	}
	key, err := ecdh.P256().NewPrivateKey(raw)
	if err != nil {
		return nil, errors.New("invalid VAPID key")
	}
	u, err := url.Parse(subject)
	if err != nil || (u.Scheme != "mailto" && u.Scheme != "https") {
		return nil, errors.New("VAPID subject must be a contact URI")
	}
	return &VAPIDProvider{privateKey: privateKey, publicKey: base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()), subject: subject, httpClient: &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (v *VAPIDProvider) Name() string      { return "webpush" }
func (v *VAPIDProvider) PublicKey() string { return v.publicKey }
func (v *VAPIDProvider) Send(ctx context.Context, sub Subscription, payload NotificationPayload) (string, bool, error) {
	u, err := url.Parse(sub.Endpoint)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" || (u.Port() != "" && u.Port() != "443") {
		return "INVALID_ENDPOINT", false, nil
	}
	switch u.Hostname() {
	case "fcm.googleapis.com", "updates.push.services.mozilla.com", "web.push.apple.com":
	default:
		return "INVALID_ENDPOINT", false, nil
	}
	ttl := int(time.Until(payload.ExpiresAt).Seconds())
	if ttl <= 0 {
		return "EXPIRED", false, nil
	}
	if ttl > 86400 {
		ttl = 86400
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "INVALID_PAYLOAD", false, nil
	}
	response, err := webpush.SendNotificationWithContext(ctx, body, &webpush.Subscription{Endpoint: sub.Endpoint, Keys: webpush.Keys{Auth: sub.Auth, P256dh: sub.P256DH}}, &webpush.Options{Subscriber: v.subject, VAPIDPublicKey: v.publicKey, VAPIDPrivateKey: v.privateKey, TTL: ttl, HTTPClient: v.httpClient, Urgency: webpush.UrgencyHigh})
	if err != nil {
		return "AMBIGUOUS", false, errors.New("push request outcome is unknown")
	}
	defer response.Body.Close()
	return fmt.Sprintf("HTTP_%d", response.StatusCode), response.StatusCode == 201, nil
}
