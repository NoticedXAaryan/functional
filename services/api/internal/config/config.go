// Package config loads and validates all runtime configuration.
// It enforces the APP_MODE × NOTIFICATION_MODE safety matrix at startup.
// Missing or malformed configuration fails closed — the server refuses to start.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// AppMode defines which operational context the server runs in.
type AppMode string

const (
	AppModeDemo       AppMode = "demo"
	AppModeBeta       AppMode = "beta"
	AppModeProduction AppMode = "production"
)

// NotificationMode controls whether push notifications may be sent.
type NotificationMode string

const (
	NotificationDisabled     NotificationMode = "DISABLED"
	NotificationTestAllowlist NotificationMode = "TEST_ALLOWLIST"
	NotificationLive         NotificationMode = "LIVE"
)

// Config is the single authoritative configuration object for the API server.
// Never pass individual fields; pass the whole Config.
type Config struct {
	AppMode          AppMode
	NotificationMode NotificationMode

	// Database
	DatabaseURL string

	// Redis
	RedisURL string

	// Server
	APIPort string
	APIHost string

	// JWT
	SessionSecret      string
	StaffJWTExpiryHours int

	// AI
	GeminiAPIKey     string
	GeminiModel      string
	GeminiTimeoutSec int
	AIEnabled        bool

	// FCM
	GoogleCredentialsPath string
	FCMProjectID          string

	// TEST controls
	TestAllowlistMax            int
	TestCampaignRevisionCap     int
	TestCumulativeRecipientCap  int

	// Return secret
	ReturnSecretWordCount int
}

// Load reads configuration from environment variables.
// Returns an error if any required variable is missing or the mode combination is invalid.
func Load() (*Config, error) {
	c := &Config{}

	var errs []string
	requireEnv := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			errs = append(errs, fmt.Sprintf("required environment variable %q is not set", key))
		}
		return v
	}

	// ── Required fields ──────────────────────────────────────────────────
	c.DatabaseURL = requireEnv("DATABASE_URL")
	c.SessionSecret = requireEnv("SESSION_SECRET")

	// ── App mode ─────────────────────────────────────────────────────────
	modeRaw := strings.ToLower(strings.TrimSpace(os.Getenv("APP_MODE")))
	switch AppMode(modeRaw) {
	case AppModeDemo, AppModeBeta, AppModeProduction:
		c.AppMode = AppMode(modeRaw)
	case "":
		return nil, errors.New("APP_MODE is required (demo|beta|production). Missing configuration defaults to safe state but the server will not start.")
	default:
		return nil, fmt.Errorf("APP_MODE=%q is not a valid value. Must be demo|beta|production", modeRaw)
	}

	// ── Notification mode ─────────────────────────────────────────────────
	notifRaw := strings.ToUpper(strings.TrimSpace(os.Getenv("NOTIFICATION_MODE")))
	switch NotificationMode(notifRaw) {
	case NotificationDisabled, NotificationTestAllowlist, NotificationLive:
		c.NotificationMode = NotificationMode(notifRaw)
	case "":
		// Default to safest combination
		c.NotificationMode = NotificationDisabled
	default:
		return nil, fmt.Errorf("NOTIFICATION_MODE=%q is not valid. Must be DISABLED|TEST_ALLOWLIST|LIVE", notifRaw)
	}

	// ── SAFETY MATRIX ─────────────────────────────────────────────────────
	// demo + LIVE is explicitly invalid. Fail closed.
	if err := validateModeMatrix(c.AppMode, c.NotificationMode); err != nil {
		return nil, err
	}

	// ── Optional with defaults ─────────────────────────────────────────────
	c.RedisURL = envOr("REDIS_URL", "redis://localhost:6379")
	c.APIPort = envOr("API_PORT", "8080")
	c.APIHost = envOr("API_HOST", "0.0.0.0")
	c.GeminiModel = envOr("GEMINI_MODEL", "gemini-2.5-flash")
	c.GeminiAPIKey = os.Getenv("GEMINI_API_KEY")
	c.GoogleCredentialsPath = os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	c.FCMProjectID = os.Getenv("FCM_PROJECT_ID")

	// AI only enabled if explicitly set AND key is present
	aiEnabled, _ := strconv.ParseBool(os.Getenv("AI_ENABLED"))
	c.AIEnabled = aiEnabled && c.GeminiAPIKey != ""

	c.GeminiTimeoutSec = envIntOr("GEMINI_TIMEOUT_SECONDS", 10)
	c.StaffJWTExpiryHours = envIntOr("STAFF_JWT_EXPIRY_HOURS", 8)
	c.TestAllowlistMax = envIntOr("TEST_ALLOWLIST_MAX", 20)
	c.TestCampaignRevisionCap = envIntOr("TEST_CAMPAIGN_REVISION_CAP", 3)
	c.TestCumulativeRecipientCap = envIntOr("TEST_CUMULATIVE_RECIPIENT_CAP", 60)
	c.ReturnSecretWordCount = envIntOr("RETURN_SECRET_WORD_COUNT", 4)

	if len(errs) > 0 {
		return nil, fmt.Errorf("configuration errors:\n  - %s", strings.Join(errs, "\n  - "))
	}

	return c, nil
}

// IsSendingAllowed returns true only when the mode combination permits push sends.
// This must be checked at API activation, queue expansion, and immediately before each send.
func (c *Config) IsSendingAllowed() bool {
	switch {
	case c.NotificationMode == NotificationDisabled:
		return false
	case c.AppMode == AppModeDemo && c.NotificationMode == NotificationLive:
		// This should never happen — validateModeMatrix prevents startup.
		// Defensive double-check at runtime.
		return false
	case c.NotificationMode == NotificationTestAllowlist:
		return true
	case c.NotificationMode == NotificationLive && (c.AppMode == AppModeBeta || c.AppMode == AppModeProduction):
		return true
	default:
		return false
	}
}

// IsTestMode returns true when notifications must be restricted to the TEST allowlist.
func (c *Config) IsTestMode() bool {
	return c.NotificationMode == NotificationTestAllowlist
}

// validateModeMatrix enforces the safety rules defined in the delivery plan.
func validateModeMatrix(mode AppMode, notif NotificationMode) error {
	if mode == AppModeDemo && notif == NotificationLive {
		return errors.New(
			"INVALID CONFIGURATION: APP_MODE=demo with NOTIFICATION_MODE=LIVE is not permitted. " +
				"This combination would allow real push sends from a demo environment. " +
				"The server refuses to start. Set NOTIFICATION_MODE=DISABLED or TEST_ALLOWLIST for demo mode.",
		)
	}
	return nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envIntOr(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}
