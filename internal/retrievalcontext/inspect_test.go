package retrievalcontext_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

func TestInspectOKForValidArtifact(t *testing.T) {
	artifact := validRetrievalArtifactJSON(t)
	result, err := retrievalcontext.InspectArtifactBytes(artifact)
	if err != nil {
		t.Fatalf("InspectArtifactBytes() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK || result.HitCount != 1 || result.AttachmentCount != 1 {
		t.Fatalf("result = %#v, want ok with one hit", result)
	}
	if len(result.UniqueChunkIDs) != 1 || result.UniqueChunkIDs[0] != "chunk-a" {
		t.Fatalf("unique_chunk_ids = %#v", result.UniqueChunkIDs)
	}
}

func TestInspectFailsWhenHitCountDiverges(t *testing.T) {
	artifact := validRetrievalArtifactJSON(t)
	var payload map[string]any
	if err := json.Unmarshal(artifact, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	payload["hit_count"] = 99
	artifact, _ = json.Marshal(payload)

	result, err := retrievalcontext.InspectArtifactBytes(artifact)
	if err != nil {
		t.Fatalf("InspectArtifactBytes() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
}

func TestInspectFailsWhenForbiddenFieldPresent(t *testing.T) {
	artifact := validRetrievalArtifactJSON(t)
	var payload map[string]any
	if err := json.Unmarshal(artifact, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	attachments := payload["attachments"].([]any)
	attachment := attachments[0].(map[string]any)
	hits := attachment["hits"].([]any)
	hit := hits[0].(map[string]any)
	hit["vector"] = []float64{0.1, 0.2}
	hit["text"] = "SECRET-CHUNK-TEXT"
	artifact, _ = json.Marshal(payload)

	result, err := retrievalcontext.InspectArtifactBytes(artifact)
	if err != nil {
		t.Fatalf("InspectArtifactBytes() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
}

func TestInspectFailsWhenAttachmentKindInvalid(t *testing.T) {
	artifact := validRetrievalArtifactJSON(t)
	var payload map[string]any
	if err := json.Unmarshal(artifact, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	attachments := payload["attachments"].([]any)
	attachment := attachments[0].(map[string]any)
	attachment["kind"] = "unsupported_kind"
	artifact, _ = json.Marshal(payload)

	result, err := retrievalcontext.InspectArtifactBytes(artifact)
	if err != nil {
		t.Fatalf("InspectArtifactBytes() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
}

func TestInspectTextOutputDoesNotLeakForbiddenPayload(t *testing.T) {
	marker := strings.Repeat("SECRET-CHUNK-TEXT-", 20)
	artifact := validRetrievalArtifactJSON(t)
	var payload map[string]any
	if err := json.Unmarshal(artifact, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	attachments := payload["attachments"].([]any)
	attachment := attachments[0].(map[string]any)
	hits := attachment["hits"].([]any)
	hit := hits[0].(map[string]any)
	hit["domain"] = marker
	artifact, _ = json.Marshal(payload)

	result, err := retrievalcontext.InspectArtifactBytes(artifact)
	if err != nil {
		t.Fatalf("InspectArtifactBytes() error = %v", err)
	}
	var buf bytes.Buffer
	if err := retrievalcontext.WriteInspectText(result, &buf); err != nil {
		t.Fatalf("WriteInspectText() error = %v", err)
	}
	output := buf.String()
	if strings.Contains(output, marker) {
		t.Fatal("inspect text leaked marker from hit payload")
	}
	for _, forbidden := range []string{`"vector":`, `"text":`, `"chunk_text":`, `"content":`, `"embedding":`} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("inspect text leaked forbidden field %q", forbidden)
		}
	}
}

func validRetrievalArtifactJSON(t *testing.T) []byte {
	t.Helper()
	payload := map[string]any{
		"status":                     "ok",
		"retrieval_context_attached": true,
		"hit_count":                  1,
		"attachments": []map[string]any{{
			"name":       "lancedb-search-1",
			"kind":       retrievalcontext.KindLanceDBSearchReport,
			"query_mode": "vector",
			"hit_count":  1,
			"hits": []map[string]any{{
				"rank":            1,
				"chunk_id":        "chunk-a",
				"distance":        0.12,
				"domain":          "general",
				"source_path":     "notes/a.md",
				"source_sha256":   "sha-source",
				"text_sha256":     "sha-text",
				"embedding_model": "deterministic-hash-v1",
				"provider":        "local",
			}},
		}},
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	return append(data, '\n')
}
