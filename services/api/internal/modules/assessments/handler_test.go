package assessments

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/balsuraksha/api/internal/config"
)

func TestNoExternalProviderSelectedByCredentials(t *testing.T) {
	for _, mode := range []config.AppMode{config.AppModeDemo, config.AppModeBeta, config.AppModeProduction} {
		t.Run(string(mode), func(t *testing.T) {
			// Even a directly constructed config must not bypass provider eligibility.
			cfg := &config.Config{AppMode: mode, AIEnabled: true, GeminiAPIKey: "not-a-real-key", GeminiTimeoutSec: 10}
			h := NewHandler(nil, cfg)
			req := httptest.NewRequest(http.MethodPost, "/assessments", strings.NewReader(`{"selected_text":"FICTIONAL: I am unsure about a message.","language":"en"}`))
			res := httptest.NewRecorder()
			h.HandleCreate(res, req)
			var result AssessmentResult
			if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if res.Code != http.StatusOK || !result.HumanHelpAvailable || result.Provider == "gemini" {
				t.Fatalf("unexpected result: status=%d result=%+v", res.Code, result)
			}
			if mode == config.AppModeDemo && (!result.IsSimulated || result.Provider != "stub") {
				t.Fatal("demo response must be clearly simulated")
			}
			if mode != config.AppModeDemo && (result.Status != "unavailable" || result.IsSimulated) {
				t.Fatal("real-use modes must expose unavailability, not a simulated assessment")
			}
			if res.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("assessment response must not be cached")
			}
		})
	}
}

func TestAssessmentBoundsRequestBody(t *testing.T) {
	h := NewHandler(nil, &config.Config{AppMode: config.AppModeDemo, GeminiTimeoutSec: 10})
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/assessments", strings.NewReader(`{"selected_text":"`+strings.Repeat("x", 40*1024)+`"}`))
	h.HandleCreate(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("oversized JSON must be rejected, got %d", res.Code)
	}
}
