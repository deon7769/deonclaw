package retrievalcontext

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

var forbiddenHitFields = []string{
	"vector",
	"text",
	"chunk_text",
	"content",
	"embedding",
}

var allowedAttachmentKinds = map[string]struct{}{
	KindLanceDBSearchReport: {},
}

type InspectResult struct {
	Status          string              `json:"status"`
	Attached        bool                `json:"attached"`
	HitCount        int                 `json:"hit_count"`
	AttachmentCount int                 `json:"attachment_count"`
	Attachments     []AttachmentSummary `json:"attachments"`
	UniqueChunkIDs  []string            `json:"unique_chunk_ids"`
	Warnings        []string            `json:"warnings"`
	Failures        []string            `json:"failures,omitempty"`
}

type AttachmentSummary struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	QueryMode string `json:"query_mode"`
	HitCount  int    `json:"hit_count"`
}

type artifactFile struct {
	Status      string           `json:"status"`
	Attached    bool             `json:"retrieval_context_attached"`
	HitCount    int              `json:"hit_count"`
	Attachments []attachmentFile `json:"attachments"`
}

type attachmentFile struct {
	Name      string            `json:"name"`
	Kind      string            `json:"kind"`
	QueryMode string            `json:"query_mode"`
	HitCount  int               `json:"hit_count"`
	Hits      []json.RawMessage `json:"hits"`
}

func InspectArtifact(path string) (InspectResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return InspectResult{}, fmt.Errorf("read retrieval context artifact %q: %w", path, err)
	}
	return InspectArtifactBytes(data)
}

func InspectArtifactBytes(data []byte) (InspectResult, error) {
	var file artifactFile
	if err := json.Unmarshal(data, &file); err != nil {
		return InspectResult{}, fmt.Errorf("parse retrieval context artifact: %w", err)
	}

	result := InspectResult{
		Status:          lancedbpolicy.StatusOK,
		Attached:        file.Attached,
		HitCount:        file.HitCount,
		AttachmentCount: len(file.Attachments),
	}

	var failures []string
	if strings.TrimSpace(file.Status) != lancedbpolicy.StatusOK {
		failures = append(failures, fmt.Sprintf("artifact status %q must be ok", file.Status))
	}

	attachmentHitSum := 0
	chunkSet := map[string]struct{}{}
	for i, attachment := range file.Attachments {
		label := fmt.Sprintf("attachments[%d]", i)
		if _, ok := allowedAttachmentKinds[strings.TrimSpace(attachment.Kind)]; !ok {
			failures = append(failures, fmt.Sprintf("%s.kind %q is not supported", label, attachment.Kind))
		}
		if attachment.HitCount != len(attachment.Hits) {
			failures = append(failures, fmt.Sprintf("%s.hit_count %d != hits length %d", label, attachment.HitCount, len(attachment.Hits)))
		}
		attachmentHitSum += len(attachment.Hits)
		result.Attachments = append(result.Attachments, AttachmentSummary{
			Name:      attachment.Name,
			Kind:      attachment.Kind,
			QueryMode: attachment.QueryMode,
			HitCount:  len(attachment.Hits),
		})
		for j, rawHit := range attachment.Hits {
			hitLabel := fmt.Sprintf("%s.hits[%d]", label, j)
			hitFailures, chunkID := validateArtifactHit(rawHit, hitLabel)
			failures = append(failures, hitFailures...)
			if chunkID != "" {
				chunkSet[chunkID] = struct{}{}
			}
		}
	}

	if file.HitCount != attachmentHitSum {
		failures = append(failures, fmt.Sprintf("hit_count %d != attachment hit sum %d", file.HitCount, attachmentHitSum))
	}
	if file.Attached && attachmentHitSum == 0 {
		failures = append(failures, "retrieval_context_attached is true but hit_count is 0")
	}
	if !file.Attached && (file.HitCount > 0 || attachmentHitSum > 0) {
		failures = append(failures, "retrieval_context_attached is false but hits are present")
	}

	for chunkID := range chunkSet {
		result.UniqueChunkIDs = append(result.UniqueChunkIDs, chunkID)
	}
	sort.Strings(result.UniqueChunkIDs)

	result.Failures = failures
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
	} else if len(result.Warnings) > 0 {
		result.Status = lancedbpolicy.StatusWarning
	}
	return result, nil
}

func validateArtifactHit(raw json.RawMessage, label string) (failures []string, chunkID string) {
	for _, field := range forbiddenHitFields {
		if hasJSONField(raw, field) {
			failures = append(failures, fmt.Sprintf("%s contains forbidden field %q", label, field))
		}
	}
	var hit SafeHit
	if err := json.Unmarshal(raw, &hit); err != nil {
		failures = append(failures, fmt.Sprintf("%s is not a valid hit object: %v", label, err))
		return failures, ""
	}
	if hit.Rank <= 0 {
		failures = append(failures, fmt.Sprintf("%s rank must be > 0", label))
	}
	if strings.TrimSpace(hit.ChunkID) == "" {
		failures = append(failures, fmt.Sprintf("%s chunk_id must not be empty", label))
	} else {
		chunkID = hit.ChunkID
	}
	if !hasJSONField(raw, "distance") {
		failures = append(failures, fmt.Sprintf("%s distance is required", label))
	} else if math.IsNaN(hit.Distance) || math.IsInf(hit.Distance, 0) {
		failures = append(failures, fmt.Sprintf("%s distance must be numeric", label))
	}
	return failures, chunkID
}

func hasJSONField(raw json.RawMessage, field string) bool {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return false
	}
	_, ok := object[field]
	return ok
}

func WriteInspectText(result InspectResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "retrieval_context_inspect:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "status: %s\n", result.Status); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "attached: %t\n", result.Attached); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "hit_count: %d\n", result.HitCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "attachment_count: %d\n", result.AttachmentCount); err != nil {
		return err
	}
	if len(result.Attachments) > 0 {
		if _, err := fmt.Fprintln(out, "\nattachments:"); err != nil {
			return err
		}
		table := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		if _, err := fmt.Fprintln(table, "name\tkind\tquery_mode\thit_count"); err != nil {
			return err
		}
		for _, attachment := range result.Attachments {
			if _, err := fmt.Fprintf(table, "%s\t%s\t%s\t%d\n", attachment.Name, attachment.Kind, attachment.QueryMode, attachment.HitCount); err != nil {
				return err
			}
		}
		if err := table.Flush(); err != nil {
			return err
		}
	}
	if len(result.UniqueChunkIDs) > 0 {
		if _, err := fmt.Fprintf(out, "unique_chunk_ids: %s\n", strings.Join(result.UniqueChunkIDs, ", ")); err != nil {
			return err
		}
	}
	if len(result.Failures) > 0 {
		if _, err := fmt.Fprintln(out, "\nfailures:"); err != nil {
			return err
		}
		for _, failure := range result.Failures {
			if _, err := fmt.Fprintf(out, "- %s\n", failure); err != nil {
				return err
			}
		}
	}
	if len(result.Warnings) > 0 {
		if _, err := fmt.Fprintln(out, "\nwarnings:"); err != nil {
			return err
		}
		for _, warning := range result.Warnings {
			if _, err := fmt.Fprintf(out, "- %s\n", warning); err != nil {
				return err
			}
		}
	}
	return nil
}

func WriteInspectJSON(result InspectResult, out io.Writer) error {
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}
