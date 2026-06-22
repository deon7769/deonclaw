package insights

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/artifacts"
	"github.com/deon7769/deonclaw/internal/events"
	"github.com/deon7769/deonclaw/internal/runs"
	"github.com/deon7769/deonclaw/internal/store"
	"github.com/deon7769/deonclaw/internal/tasks"
)

const LegacyManualAgentID = "legacy-manual"

type EvidenceScope struct {
	Repository string `json:"repository"`
	AgentID    string `json:"agent_id"`
	SessionID  string `json:"session_id,omitempty"`
	WorkItemID string `json:"work_item_id,omitempty"`
	TaskID     string `json:"task_id,omitempty"`
}

type ArtifactRef struct {
	ArtifactID string `json:"artifact_id"`
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	SHA256     string `json:"sha256"`
	Name       string `json:"name"`
}

type ValidationResultRef struct {
	Command string `json:"command"`
	Status  string `json:"status"`
	Name    string `json:"name,omitempty"`
}

type DiffSummary struct {
	FilesChanged int `json:"files_changed"`
	Insertions   int `json:"insertions"`
	Deletions    int `json:"deletions"`
}

type EvidenceBundle struct {
	EvidenceBundleID    string                `json:"evidence_bundle_id"`
	CreatedAt           string                `json:"created_at"`
	Scope               EvidenceScope         `json:"scope"`
	Trigger             string                `json:"trigger"`
	RunIDs              []string              `json:"run_ids"`
	EventIDs            []string              `json:"event_ids"`
	ArtifactRefs        []ArtifactRef         `json:"artifact_refs"`
	CommitSHAs          []string              `json:"commit_shas"`
	ValidationResults   []ValidationResultRef `json:"validation_results"`
	DiffSummary         DiffSummary           `json:"diff_summary"`
	UserFeedbackRefs    []string              `json:"user_feedback_refs"`
	PolicyRefs          []string              `json:"policy_refs"`
	ContainsTextExcerpt bool                  `json:"contains_text_excerpt"`
	SHA256              string                `json:"sha256"`
}

type BuildEvidenceOptions struct {
	RunID     string
	Trigger   string
	PolicyRef string
	Now       time.Time
}

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)api[_-]?key\s*[:=]\s*\S+`),
	regexp.MustCompile(`(?i)secret\s*[:=]\s*\S+`),
	regexp.MustCompile(`(?i)token\s*[:=]\s*\S+`),
	regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._\-]+`),
	regexp.MustCompile(`sk-[A-Za-z0-9]{16,}`),
}

func BuildEvidenceFromRun(ctx context.Context, db store.Store, opts BuildEvidenceOptions) (EvidenceBundle, error) {
	runID := strings.TrimSpace(opts.RunID)
	if runID == "" {
		return EvidenceBundle{}, fmt.Errorf("run id is required")
	}

	runRecord, err := db.Run(ctx, runID)
	if err != nil {
		return EvidenceBundle{}, err
	}
	if runRecord == nil {
		return EvidenceBundle{}, store.ErrNotFound
	}

	taskRecord, err := db.Task(ctx, runRecord.TaskID)
	if err != nil {
		return EvidenceBundle{}, err
	}

	runEvents, err := db.EventsByRun(ctx, runID)
	if err != nil {
		return EvidenceBundle{}, err
	}

	runArtifacts, err := db.ArtifactsByRun(ctx, runID)
	if err != nil {
		return EvidenceBundle{}, err
	}

	trigger := strings.TrimSpace(opts.Trigger)
	if trigger == "" {
		trigger = inferTrigger(runRecord, runArtifacts)
	}
	if err := validateTriggerType("trigger", trigger); err != nil {
		return EvidenceBundle{}, err
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	bundle := EvidenceBundle{
		EvidenceBundleID:    newEvidenceBundleID(runID, now),
		CreatedAt:           now.Format(time.RFC3339Nano),
		Trigger:             trigger,
		RunIDs:              []string{runID},
		EventIDs:            eventIDs(runEvents),
		ArtifactRefs:        artifactRefs(runArtifacts),
		CommitSHAs:          commitSHAsFromArtifacts(runArtifacts),
		ValidationResults:   validationRefsFromArtifacts(runArtifacts),
		DiffSummary:         diffSummaryFromArtifacts(runArtifacts),
		UserFeedbackRefs:    []string{},
		PolicyRefs:          policyRefs(opts.PolicyRef),
		ContainsTextExcerpt: false,
		Scope:               evidenceScope(taskRecord, runRecord),
	}

	hash, err := bundleHash(bundle)
	if err != nil {
		return EvidenceBundle{}, err
	}
	bundle.SHA256 = hash

	if err := scanBundleForSecrets(bundle); err != nil {
		return EvidenceBundle{}, err
	}

	return bundle, nil
}

func WriteEvidenceJSON(bundle EvidenceBundle, path string) error {
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal evidence bundle: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write evidence bundle %q: %w", path, err)
	}
	return nil
}

func ReadEvidenceJSON(path string) (EvidenceBundle, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return EvidenceBundle{}, fmt.Errorf("read evidence bundle %q: %w", path, err)
	}
	var bundle EvidenceBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return EvidenceBundle{}, fmt.Errorf("parse evidence bundle %q: %w", path, err)
	}
	return bundle, nil
}

func inferTrigger(runRecord *runs.Run, runArtifacts []artifacts.Artifact) string {
	if validationStatusFromArtifacts(runArtifacts) == "failed" {
		return TriggerValidationFailed
	}
	if runRecord.Status.IsTerminal() {
		return TriggerRunCompleted
	}
	return TriggerManualInsightRequest
}

func evidenceScope(taskRecord *tasks.Task, runRecord *runs.Run) EvidenceScope {
	scope := EvidenceScope{
		Repository: repositoryName(taskRecord, runRecord),
		AgentID:    LegacyManualAgentID,
	}
	if taskRecord != nil {
		scope.TaskID = taskRecord.ID
	}
	return scope
}

func repositoryName(taskRecord *tasks.Task, runRecord *runs.Run) string {
	if taskRecord != nil {
		if path := strings.TrimSpace(taskRecord.Workspace.Path); path != "" {
			return filepath.Base(filepath.Clean(path))
		}
		if domain := strings.TrimSpace(taskRecord.Domain); domain != "" {
			return domain
		}
	}
	if runRecord != nil {
		if path := strings.TrimSpace(runRecord.WorkspacePath); path != "" {
			return filepath.Base(filepath.Clean(path))
		}
	}
	return "unknown"
}

func newEvidenceBundleID(runID string, now time.Time) string {
	sum := sha256.Sum256([]byte(runID + "|" + now.Format(time.RFC3339Nano)))
	return "evb_" + hex.EncodeToString(sum[:8])
}

func eventIDs(runEvents []events.Event) []string {
	ids := make([]string, 0, len(runEvents))
	for _, event := range runEvents {
		if strings.TrimSpace(event.ID) == "" {
			continue
		}
		ids = append(ids, event.ID)
	}
	return ids
}

func artifactRefs(runArtifacts []artifacts.Artifact) []ArtifactRef {
	refs := make([]ArtifactRef, 0, len(runArtifacts))
	for _, artifact := range runArtifacts {
		refs = append(refs, ArtifactRef{
			ArtifactID: artifact.ID,
			Path:       artifact.Path,
			Kind:       string(artifact.Kind),
			SHA256:     artifact.SHA256,
			Name:       artifactBaseName(artifact.Path),
		})
	}
	return refs
}

func artifactBaseName(path string) string {
	name := filepath.Base(filepath.Clean(path))
	if name == "." || name == string(filepath.Separator) {
		return ""
	}
	return name
}

func validationRefsFromArtifacts(runArtifacts []artifacts.Artifact) []ValidationResultRef {
	content, ok := artifactContentByName(runArtifacts, "validation.json")
	if !ok {
		return []ValidationResultRef{}
	}
	var payload struct {
		Commands []struct {
			Name    string `json:"name"`
			Command string `json:"command"`
			Status  string `json:"status"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(content, &payload); err != nil {
		return []ValidationResultRef{}
	}
	refs := make([]ValidationResultRef, 0, len(payload.Commands))
	for _, command := range payload.Commands {
		commandText := strings.TrimSpace(command.Command)
		if commandText == "" {
			commandText = strings.TrimSpace(command.Name)
		}
		refs = append(refs, ValidationResultRef{
			Command: commandText,
			Status:  strings.TrimSpace(command.Status),
			Name:    strings.TrimSpace(command.Name),
		})
	}
	return refs
}

func validationStatusFromArtifacts(runArtifacts []artifacts.Artifact) string {
	content, ok := artifactContentByName(runArtifacts, "validation.json")
	if !ok {
		return ""
	}
	var payload struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(content, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.Status)
}

func diffSummaryFromArtifacts(runArtifacts []artifacts.Artifact) DiffSummary {
	summary := DiffSummary{}
	if content, ok := artifactContentByName(runArtifacts, "changed-files.json"); ok {
		var changedFiles []struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(content, &changedFiles); err == nil {
			summary.FilesChanged = len(changedFiles)
		}
	}
	if content, ok := artifactContentByName(runArtifacts, "diff.patch"); ok {
		insertions, deletions := countDiffLines(content)
		summary.Insertions = insertions
		summary.Deletions = deletions
	}
	return summary
}

func countDiffLines(patch []byte) (insertions int, deletions int) {
	for _, line := range strings.Split(string(patch), "\n") {
		if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
			continue
		}
		if strings.HasPrefix(line, "+") {
			insertions++
		}
		if strings.HasPrefix(line, "-") {
			deletions++
		}
	}
	return insertions, deletions
}

func commitSHAsFromArtifacts(runArtifacts []artifacts.Artifact) []string {
	content, ok := artifactContentByName(runArtifacts, "execution-trace.json")
	if !ok {
		return []string{}
	}
	var payload struct {
		CommitSHA string `json:"commit_sha"`
	}
	if err := json.Unmarshal(content, &payload); err != nil {
		return []string{}
	}
	commitSHA := strings.TrimSpace(payload.CommitSHA)
	if commitSHA == "" {
		return []string{}
	}
	return []string{commitSHA}
}

func artifactContentByName(runArtifacts []artifacts.Artifact, name string) ([]byte, bool) {
	for _, artifact := range runArtifacts {
		if artifactBaseName(artifact.Path) != name {
			continue
		}
		if len(artifact.Content) > 0 {
			return artifact.Content, true
		}
		content, err := os.ReadFile(artifact.Path)
		if err != nil {
			return nil, false
		}
		return content, true
	}
	return nil, false
}

func policyRefs(policyRef string) []string {
	policyRef = strings.TrimSpace(policyRef)
	if policyRef == "" {
		return []string{}
	}
	return []string{policyRef}
}

func bundleHash(bundle EvidenceBundle) (string, error) {
	copy := bundle
	copy.SHA256 = ""
	data, err := json.Marshal(copy)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func scanBundleForSecrets(bundle EvidenceBundle) error {
	return scanJSONForSecrets(bundle)
}

func scanJSONForSecrets(value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	text := string(data)
	for _, pattern := range secretPatterns {
		if pattern.MatchString(text) {
			return fmt.Errorf("payload contains secret-like value matching %q", pattern.String())
		}
	}
	return nil
}
