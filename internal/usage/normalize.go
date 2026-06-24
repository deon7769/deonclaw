package usage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

type NormalizeInput struct {
	Metadata     map[string]any
	Worker       string
	ModelProfile string
	Provider     string
	Model        string
}

type NormalizeResult struct {
	Event           Event
	RawMetadataHash string
}

func NormalizeWorkerMetadata(input NormalizeInput) (NormalizeResult, error) {
	meta := input.Metadata
	if meta == nil {
		meta = map[string]any{}
	}
	inputTokens := int64Field(meta, "input_tokens", "prompt_tokens")
	outputTokens := int64Field(meta, "output_tokens", "completion_tokens")
	cachedInput := int64Field(meta, "cached_input_tokens", "cached_tokens")
	toolCalls := int64Field(meta, "tool_call_count", "tool_calls")
	durationMS := int64Field(meta, "duration_ms", "duration_milliseconds")

	if inputTokens < 0 || outputTokens < 0 || cachedInput < 0 || toolCalls < 0 || durationMS < 0 {
		return NormalizeResult{}, fmt.Errorf("negative usage values are not allowed")
	}
	if inputTokens > math.MaxInt64/2 || outputTokens > math.MaxInt64/2 {
		return NormalizeResult{}, fmt.Errorf("usage token overflow")
	}

	provider := strings.TrimSpace(input.Provider)
	model := strings.TrimSpace(input.Model)
	if v := stringField(meta, "provider"); provider == "" {
		provider = v
	}
	if v := stringField(meta, "model"); model == "" {
		model = v
	}

	tokensAvailable := inputTokens > 0 || outputTokens > 0 || cachedInput > 0
	source := SourceEstimated
	confidence := ConfidenceEstimated
	if tokensAvailable {
		source = SourceWorkerMetadata
		confidence = ConfidenceNormalized
	}

	event := Event{
		Worker:            strings.TrimSpace(input.Worker),
		Provider:          provider,
		Model:             model,
		ModelProfile:      strings.TrimSpace(input.ModelProfile),
		InputTokens:       inputTokens,
		OutputTokens:      outputTokens,
		CachedInputTokens: cachedInput,
		ToolCallCount:     toolCalls,
		DurationMS:        durationMS,
		Source:            source,
		Confidence:        confidence,
		TokensAvailable:   tokensAvailable,
	}
	return NormalizeResult{Event: event, RawMetadataHash: metadataSHA256(meta)}, nil
}

func int64Field(meta map[string]any, keys ...string) int64 {
	for _, key := range keys {
		v, ok := meta[key]
		if !ok {
			continue
		}
		switch n := v.(type) {
		case int:
			return int64(n)
		case int64:
			return n
		case float64:
			return int64(n)
		case json.Number:
			i, _ := n.Int64()
			return i
		}
	}
	return 0
}

func stringField(meta map[string]any, key string) string {
	v, ok := meta[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func metadataSHA256(meta map[string]any) string {
	data, _ := json.Marshal(meta)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
