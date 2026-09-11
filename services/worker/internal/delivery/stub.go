// Package delivery — stub provider.
// Used when GOOGLE_APPLICATION_CREDENTIALS is absent or NOTIFICATION_MODE=DISABLED.
// Records the simulated send in delivery_attempts but never contacts a real push service.
// All output is clearly labeled SIMULATED — never presented as real delivery.
package delivery

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
)

// StubProvider is a deterministic push adapter for demo/test mode.
// It always succeeds, never contacts a real endpoint, and logs the simulated send.
type StubProvider struct{}

// NewStubProvider creates a StubProvider.
func NewStubProvider() *StubProvider { return &StubProvider{} }

// Name returns the provider identifier.
func (s *StubProvider) Name() string { return "stub" }

// Send simulates a successful push send. Never contacts a real provider.
func (s *StubProvider) Send(_ context.Context, endpoint string, payload NotificationPayload) (string, bool, error) {
	log.Info().
		Str("provider", "stub").
		Str("endpoint_prefix", safePrefix(endpoint, 20)).
		Str("alert_id", payload.AlertID).
		Bool("is_test", payload.IsTest).
		Msg("SIMULATED push send — not a real delivery. For demonstration only.")

	return "SIMULATED_OK", true, nil
}

func safePrefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return fmt.Sprintf("%s...[truncated]", s[:n])
}
