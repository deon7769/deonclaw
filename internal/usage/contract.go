package usage

import (
	"fmt"
	"strings"
	"time"
)

const (
	SourceWorkerMetadata   = "worker_metadata"
	SourceProviderReport   = "provider_report"
	SourceEstimated        = "estimated"
	SourceManualAdjustment = "manual_adjustment"
	SourceFakeFixture      = "fake_fixture"

	ConfidenceActual     = "actual"
	ConfidenceNormalized = "normalized"
	ConfidenceEstimated  = "estimated"
)

type Event struct {
	ID                    string `json:"id"`
	RunID                 string `json:"run_id"`
	WorkItemID            string `json:"work_item_id,omitempty"`
	AgentID               string `json:"agent_id,omitempty"`
	SessionID             string `json:"session_id,omitempty"`
	Worker                string `json:"worker"`
	Provider              string `json:"provider,omitempty"`
	Model                 string `json:"model,omitempty"`
	ModelProfile          string `json:"model_profile,omitempty"`
	InputTokens           int64  `json:"input_tokens"`
	OutputTokens          int64  `json:"output_tokens"`
	CachedInputTokens     int64  `json:"cached_input_tokens"`
	ToolCallCount         int64  `json:"tool_call_count"`
	DurationMS            int64  `json:"duration_ms"`
	EstimatedCostMicroUSD int64  `json:"estimated_cost_microusd"`
	ActualCostMicroUSD    int64  `json:"actual_cost_microusd"`
	Source                string `json:"source"`
	Confidence            string `json:"confidence"`
	TokensAvailable       bool   `json:"tokens_available"`
	CreatedAt             string `json:"created_at"`
}

func ValidateEvent(event Event) error {
	if strings.TrimSpace(event.ID) == "" {
		return fmt.Errorf("usage event id is required")
	}
	if strings.TrimSpace(event.RunID) == "" {
		return fmt.Errorf("usage event run_id is required")
	}
	if strings.TrimSpace(event.Worker) == "" {
		return fmt.Errorf("usage event worker is required")
	}
	if strings.TrimSpace(event.Source) == "" {
		return fmt.Errorf("usage event source is required")
	}
	if strings.TrimSpace(event.Confidence) == "" {
		return fmt.Errorf("usage event confidence is required")
	}
	if event.InputTokens < 0 || event.OutputTokens < 0 || event.CachedInputTokens < 0 {
		return fmt.Errorf("token counts must be non-negative")
	}
	if event.EstimatedCostMicroUSD < 0 || event.ActualCostMicroUSD < 0 {
		return fmt.Errorf("cost amounts must be non-negative")
	}
	return nil
}

func NewEventID(runID string, now time.Time) string {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return fmt.Sprintf("usage_%s_%d", runID, now.UnixNano())
}
