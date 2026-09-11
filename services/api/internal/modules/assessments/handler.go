// Package assessments provides the bounded AI assessment feature ("Is this okay?").
// Design rules:
//   - AI is always optional. If unavailable, the human help path continues unimpaired.
//   - Input is treated as untrusted data — it cannot control the application or Gemini tools.
//   - Output is validated against a strict schema before serving.
//   - Demo mode uses a labeled stub; other modes expose an honest unavailable result.
//   - A Gemini key never enables child-facing inference.
//   - No guilt verdict, no numeric certainty score, no automatic actions.
package assessments

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	"github.com/balsuraksha/api/internal/config"
	"github.com/balsuraksha/api/internal/db"
)

// AssessmentResult is the structured output from the AI assessment.
// All fields are optional — the human_help_available field is always true.
type AssessmentResult struct {
	Status               string   `json:"status"`
	HumanHelpAvailable   bool     `json:"human_help_available"`
	ObservedBehaviors    []string `json:"observed_behaviors,omitempty"`
	UncertaintyStatement string   `json:"uncertainty_statement,omitempty"`
	ClarifyingQuestions  []string `json:"clarifying_questions,omitempty"`
	SuggestedOptions     []string `json:"suggested_options,omitempty"`
	Provider             string   `json:"provider"`
	IsSimulated          bool     `json:"is_simulated"`
}

// Handler handles assessment requests.
type Handler struct {
	pool    *db.Pool
	cfg     *config.Config
	adapter Adapter
}

// Adapter is the interface for AI providers.
type Adapter interface {
	Assess(ctx context.Context, selectedText, language string) (*AssessmentResult, error)
}

// NewHandler cannot select an external provider until child-use eligibility is established.
func NewHandler(pool *db.Pool, cfg *config.Config) *Handler {
	var adapter Adapter = unavailableAdapter{}
	if cfg.AppMode == config.AppModeDemo {
		adapter = &StubAdapter{}
	}
	return &Handler{pool: pool, cfg: cfg, adapter: adapter}
}

type unavailableAdapter struct{}

func (unavailableAdapter) Assess(context.Context, string, string) (*AssessmentResult, error) {
	return nil, errors.New("no approved assessment provider")
}

// Routes registers assessment routes.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Post("/", h.HandleCreate)
	return r
}

// HandleCreate processes an assessment request.
// Input text is sanitized and passed to the adapter as data — never as instructions.
func (h *Handler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	var req struct {
		SelectedText string `json:"selected_text"`
		Language     string `json:"language"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON")
		return
	}
	if req.SelectedText == "" {
		writeError(w, http.StatusBadRequest, "empty_text", "selected_text is required")
		return
	}
	if len(req.SelectedText) > 5000 {
		writeJSON(w, http.StatusOK, &AssessmentResult{
			Status:             "input_too_long",
			HumanHelpAvailable: true,
			Provider:           "none",
			IsSimulated:        false,
		})
		return
	}
	if req.Language == "" {
		req.Language = "en"
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(h.cfg.GeminiTimeoutSec)*time.Second)
	defer cancel()

	result, err := h.adapter.Assess(ctx, req.SelectedText, req.Language)
	if err != nil {
		// Provider errors can contain submitted text. Keep them out of ordinary logs.
		log.Warn().Msg("assessment unavailable; human-help route remains available")
		writeJSON(w, http.StatusOK, &AssessmentResult{
			Status:             "unavailable",
			HumanHelpAvailable: true,
			Provider:           "error",
			IsSimulated:        false,
		})
		return
	}

	// Safety validation: ensure human help is always available
	result.HumanHelpAvailable = true

	// Cap clarifying questions at 3
	if len(result.ClarifyingQuestions) > 3 {
		result.ClarifyingQuestions = result.ClarifyingQuestions[:3]
	}

	writeJSON(w, http.StatusOK, result)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg, "code": code})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
