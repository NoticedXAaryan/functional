package assessments

import "context"

// StubAdapter returns a clearly labeled deterministic result.
// Used when AI_ENABLED=false or GEMINI_API_KEY is not configured.
// The response is honest about being simulated — never presented as real AI output.
type StubAdapter struct{}

// Assess returns a deterministic, clearly simulated assessment result.
func (s *StubAdapter) Assess(_ context.Context, _, _ string) (*AssessmentResult, error) {
	return &AssessmentResult{
		Status:             "ok",
		HumanHelpAvailable: true,
		ObservedBehaviors: []string{
			"[SIMULATED] The text you shared describes a situation worth talking through with someone.",
		},
		UncertaintyStatement: "[SIMULATED] This is a demonstration response. A real assessment would review your specific text.",
		ClarifyingQuestions:  nil,
		SuggestedOptions: []string{
			"Talk to a trusted adult about what you shared.",
			"Contact the Child Helpline at 1098 for confidential support.",
			"Use the 'Ask a person for help' option to speak with a trained responder.",
		},
		Provider:    "stub",
		IsSimulated: true,
	}, nil
}
