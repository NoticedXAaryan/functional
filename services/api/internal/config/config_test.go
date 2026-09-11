// Package config — test coverage for T-001: configuration matrix.
// Every valid mode combination must start cleanly.
// Invalid combinations (demo + LIVE) must fail closed before the server starts.
package config_test

import (
	"testing"

	"github.com/balsuraksha/api/internal/config"
)

func setEnv(t *testing.T, pairs ...string) {
	t.Helper()
	// Keep tests independent of the developer's shell configuration.
	t.Setenv("AI_ENABLED", "false")
	t.Setenv("STAFF_AUTH_MODE", "oidc")
	t.Setenv("OIDC_ISSUER", "https://identity.example.test/realms/test")
	t.Setenv("OIDC_AUDIENCE", "balsuraksha-api")
	t.Setenv("OIDC_CLIENT_ID", "balsuraksha-ops")
	for i := 0; i < len(pairs); i += 2 {
		t.Setenv(pairs[i], pairs[i+1])
	}
}

func TestT001_ValidModes(t *testing.T) {
	cases := []struct {
		name   string
		mode   string
		notif  string
		wantOK bool
	}{
		{"demo/DISABLED", "demo", "DISABLED", true},
		{"demo/TEST_ALLOWLIST", "demo", "TEST_ALLOWLIST", true},
		{"beta/DISABLED", "beta", "DISABLED", true},
		{"beta/TEST_ALLOWLIST", "beta", "TEST_ALLOWLIST", true},
		{"beta/LIVE", "beta", "LIVE", true},
		{"production/DISABLED", "production", "DISABLED", true},
		{"production/TEST_ALLOWLIST", "production", "TEST_ALLOWLIST", true},
		{"production/LIVE", "production", "LIVE", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setEnv(t,
				"APP_MODE", tc.mode,
				"NOTIFICATION_MODE", tc.notif,
				"DATABASE_URL", "postgres://x:x@localhost/test",
				"SESSION_SECRET", "test-secret-at-least-32-bytes-for-tests-only",
			)
			_, err := config.Load()
			if tc.wantOK && err != nil {
				t.Errorf("expected valid config, got error: %v", err)
			}
			if !tc.wantOK && err == nil {
				t.Error("expected error for invalid config, got nil")
			}
		})
	}
}

func TestT001_InvalidCombinations(t *testing.T) {
	t.Run("demo/LIVE must fail closed", func(t *testing.T) {
		setEnv(t,
			"APP_MODE", "demo",
			"NOTIFICATION_MODE", "LIVE",
			"DATABASE_URL", "postgres://x:x@localhost/test",
			"SESSION_SECRET", "test-secret-at-least-32-bytes-for-tests-only",
		)
		_, err := config.Load()
		if err == nil {
			t.Fatal("demo+LIVE must be rejected but config.Load() returned nil error")
		}
	})
}

func TestT001_MissingAppMode(t *testing.T) {
	// Clear APP_MODE — must fail, not silently default
	t.Setenv("APP_MODE", "")
	setEnv(t,
		"NOTIFICATION_MODE", "DISABLED",
		"DATABASE_URL", "postgres://x:x@localhost/test",
		"SESSION_SECRET", "test-secret-at-least-32-bytes-for-tests-only",
	)
	_, err := config.Load()
	if err == nil {
		t.Error("expected error for missing APP_MODE, but got nil")
	}
}

func TestT001_MalformedMode(t *testing.T) {
	setEnv(t,
		"APP_MODE", "staging", // not a valid value
		"NOTIFICATION_MODE", "DISABLED",
		"DATABASE_URL", "postgres://x:x@localhost/test",
		"SESSION_SECRET", "test-secret-at-least-32-bytes-for-tests-only",
	)
	_, err := config.Load()
	if err == nil {
		t.Fatal("malformed APP_MODE must return an error")
	}
}

func TestT001_IsSendingAllowed(t *testing.T) {
	cases := []struct {
		mode      config.AppMode
		notif     config.NotificationMode
		wantAllow bool
	}{
		{config.AppModeDemo, config.NotificationDisabled, false},
		{config.AppModeDemo, config.NotificationTestAllowlist, true},
		{config.AppModeBeta, config.NotificationDisabled, false},
		{config.AppModeBeta, config.NotificationTestAllowlist, true},
		{config.AppModeBeta, config.NotificationLive, true},
		{config.AppModeProduction, config.NotificationDisabled, false},
		{config.AppModeProduction, config.NotificationLive, true},
	}
	for _, tc := range cases {
		setEnv(t,
			"APP_MODE", string(tc.mode),
			"NOTIFICATION_MODE", string(tc.notif),
			"DATABASE_URL", "postgres://x:x@localhost/test",
			"SESSION_SECRET", "test-secret-at-least-32-bytes-for-tests-only",
		)
		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("valid mode must load: %v", err)
		}
		got := cfg.IsSendingAllowed()
		if got != tc.wantAllow {
			t.Errorf("%s/%s: IsSendingAllowed()=%v, want %v", tc.mode, tc.notif, got, tc.wantAllow)
		}
	}
}

func TestLiveAIConfigurationFailsClosed(t *testing.T) {
	for _, value := range []string{"true", "not-a-boolean"} {
		t.Run(value, func(t *testing.T) {
			setEnv(t, "APP_MODE", "demo", "NOTIFICATION_MODE", "DISABLED",
				"DATABASE_URL", "postgres://x:x@localhost/test",
				"SESSION_SECRET", "test-only-secret", "AI_ENABLED", value,
				"GEMINI_API_KEY", "not-a-real-key")
			if _, err := config.Load(); err == nil {
				t.Fatal("unapproved or malformed live-AI configuration must fail closed")
			}
		})
	}
}
