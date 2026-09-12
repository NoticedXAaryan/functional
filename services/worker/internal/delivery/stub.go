package delivery

import "context"

// StubProvider is available to tests only; runtime never falls back to it.
type StubProvider struct{}

func NewStubProvider() *StubProvider { return &StubProvider{} }
func (s *StubProvider) Name() string { return "stub" }
func (s *StubProvider) Send(context.Context, Subscription, NotificationPayload) (string, bool, error) {
	return "SIMULATED", false, nil
}
