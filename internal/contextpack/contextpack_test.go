package contextpack

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestBuildGeneralContextPackExcludesIsolatedDomain(t *testing.T) {
	tempDir := t.TempDir()
	bridgePath := filepath.Join(tempDir, "escalasoft-bridge.md")
	if err := os.WriteFile(bridgePath, []byte("Escalasoft isolated details\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	taskPath := writeContextPackTask(t, tempDir, "general")
	domainsPath := writeContextPackDomains(t, tempDir, bridgePath)

	pack, err := Builder{}.Build(context.Background(), BuildOptions{
		TaskPath:    taskPath,
		DomainsPath: domainsPath,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	markdown := string(pack.Markdown())
	if strings.Contains(markdown, "Escalasoft isolated details") {
		t.Fatalf("markdown contains isolated bridge content for general task: %s", markdown)
	}
	if strings.Contains(markdown, "default_agent: escalasoft-agent") {
		t.Fatalf("markdown contains isolated domain metadata for general task: %s", markdown)
	}
	if len(pack.Warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", pack.Warnings)
	}
	if got := countSourcesByKind(pack.Sources, "bridge_file"); got != 0 {
		t.Fatalf("bridge source count = %d, want 0", got)
	}
}

func TestBuildEscalasoftContextPackIncludesBridgeFiles(t *testing.T) {
	tempDir := t.TempDir()
	bridgePath := filepath.Join(tempDir, "escalasoft-bridge.md")
	bridgeContent := "Escalasoft bridge context\n"
	if err := os.WriteFile(bridgePath, []byte(bridgeContent), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	taskPath := writeContextPackTask(t, tempDir, "escalasoft")
	domainsPath := writeContextPackDomains(t, tempDir, bridgePath)

	pack, err := Builder{}.Build(context.Background(), BuildOptions{
		TaskPath:    taskPath,
		DomainsPath: domainsPath,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if pack.Domain.Name != "escalasoft" || !pack.Domain.IsIsolated() {
		t.Fatalf("domain = %#v, want isolated escalasoft domain", pack.Domain)
	}
	if got := countSourcesByKind(pack.Sources, "bridge_file"); got != 1 {
		t.Fatalf("bridge source count = %d, want 1", got)
	}
	source := findSourceByKind(t, pack.Sources, "bridge_file")
	if !source.Exists {
		t.Fatalf("bridge source exists = false, want true")
	}

	markdown := string(pack.Markdown())
	for _, want := range []string{
		"domain: escalasoft",
		"type: isolated_domain",
		"isolated: true",
		"default_agent: escalasoft-agent",
		"historical_sqlite: /data/escalasoft.db",
		bridgeContent,
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("markdown = %q, want %q", markdown, want)
		}
	}
}

func TestBuildLargeBridgeFileTruncatesWithMetadata(t *testing.T) {
	tempDir := t.TempDir()
	bridgePath := filepath.Join(tempDir, "large-bridge.md")
	bridgeContent := strings.Repeat("x", defaultMaxSourceBytes) + "TAIL"
	if err := os.WriteFile(bridgePath, []byte(bridgeContent), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	taskPath := writeContextPackTask(t, tempDir, "escalasoft")
	domainsPath := writeContextPackDomains(t, tempDir, bridgePath)

	pack, err := Builder{}.Build(context.Background(), BuildOptions{
		TaskPath:    taskPath,
		DomainsPath: domainsPath,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	source := findSourceByKind(t, pack.Sources, "bridge_file")
	if !source.Exists {
		t.Fatalf("source exists = false, want true")
	}
	if !source.Truncated {
		t.Fatalf("source truncated = false, want true")
	}
	if source.SizeBytes != int64(len(bridgeContent)) {
		t.Fatalf("source size_bytes = %d, want %d", source.SizeBytes, len(bridgeContent))
	}
	if source.SHA256 != sha256Hex([]byte(bridgeContent)) {
		t.Fatalf("source sha256 = %q, want %q", source.SHA256, sha256Hex([]byte(bridgeContent)))
	}
	if len(source.Content) != defaultMaxSourceBytes {
		t.Fatalf("source content len = %d, want %d", len(source.Content), defaultMaxSourceBytes)
	}
	if strings.Contains(source.Content, "TAIL") {
		t.Fatalf("source content includes truncated tail")
	}
	if len(pack.Warnings) != 1 || !strings.Contains(pack.Warnings[0], "truncated") {
		t.Fatalf("warnings = %#v, want truncation warning", pack.Warnings)
	}

	markdown := string(pack.Markdown())
	for _, want := range []string{
		"exists: true",
		"size_bytes: " + strconv.Itoa(len(bridgeContent)),
		"sha256: " + sha256Hex([]byte(bridgeContent)),
		"truncated: true",
		"truncated to 262144 bytes",
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("markdown = %q, want %q", markdown, want)
		}
	}
	if strings.Contains(markdown, "TAIL") {
		t.Fatalf("markdown includes truncated tail")
	}
}

func TestBridgeFileMetadataAppearsInMarkdown(t *testing.T) {
	tempDir := t.TempDir()
	bridgePath := filepath.Join(tempDir, "bridge.md")
	bridgeContent := "metadata bridge\n"
	if err := os.WriteFile(bridgePath, []byte(bridgeContent), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	taskPath := writeContextPackTask(t, tempDir, "escalasoft")
	domainsPath := writeContextPackDomains(t, tempDir, bridgePath)

	pack, err := Builder{}.Build(context.Background(), BuildOptions{
		TaskPath:    taskPath,
		DomainsPath: domainsPath,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	source := findSourceByKind(t, pack.Sources, "bridge_file")
	if source.SizeBytes != int64(len(bridgeContent)) {
		t.Fatalf("source size_bytes = %d, want %d", source.SizeBytes, len(bridgeContent))
	}
	if source.SHA256 != sha256Hex([]byte(bridgeContent)) {
		t.Fatalf("source sha256 = %q, want %q", source.SHA256, sha256Hex([]byte(bridgeContent)))
	}

	markdown := string(pack.Markdown())
	for _, want := range []string{
		"exists: true",
		"size_bytes: " + strconv.Itoa(len(bridgeContent)),
		"sha256: " + sha256Hex([]byte(bridgeContent)),
		"truncated: false",
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("markdown = %q, want %q", markdown, want)
		}
	}
}

func TestBuildMissingBridgeFileAddsWarning(t *testing.T) {
	tempDir := t.TempDir()
	missingBridgePath := filepath.Join(tempDir, "missing-bridge.md")
	taskPath := writeContextPackTask(t, tempDir, "escalasoft")
	domainsPath := writeContextPackDomains(t, tempDir, missingBridgePath)

	pack, err := Builder{}.Build(context.Background(), BuildOptions{
		TaskPath:    taskPath,
		DomainsPath: domainsPath,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if len(pack.Warnings) != 1 {
		t.Fatalf("warnings = %#v, want one warning", pack.Warnings)
	}
	if !strings.Contains(pack.Warnings[0], filepath.ToSlash(missingBridgePath)) {
		t.Fatalf("warning = %q, want missing bridge path", pack.Warnings[0])
	}
	source := findSourceByKind(t, pack.Sources, "bridge_file")
	if source.Exists {
		t.Fatalf("missing bridge source exists = true, want false")
	}
	markdown := string(pack.Markdown())
	for _, want := range []string{"Warnings", "exists: false", "size_bytes: 0", "truncated: false"} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("markdown = %q, want %q", markdown, want)
		}
	}
}

func TestMarkdownContainsTaskAndDomainMetadata(t *testing.T) {
	tempDir := t.TempDir()
	taskPath := writeContextPackTask(t, tempDir, "general")
	domainsPath := writeContextPackDomains(t, tempDir, filepath.Join(tempDir, "bridge.md"))

	pack, err := Builder{}.Build(context.Background(), BuildOptions{
		TaskPath:    taskPath,
		DomainsPath: domainsPath,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	markdown := string(pack.Markdown())
	for _, want := range []string{
		"id: context-task-general",
		"title: Context Pack general",
		"domain: general",
		"goal: Build a context pack for general",
		"mode: read_only",
		"name: general",
		"type: canonical_memory",
		"root: /vault/mysecondbrain",
		"default: true",
		"allowed_paths",
		"forbidden_paths",
		"validation commands",
		"context sources",
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("markdown = %q, want %q", markdown, want)
		}
	}
}

func countSourcesByKind(sources []ContextSource, kind string) int {
	count := 0
	for _, source := range sources {
		if source.Kind == kind {
			count++
		}
	}
	return count
}

func findSourceByKind(t *testing.T, sources []ContextSource, kind string) ContextSource {
	t.Helper()
	for _, source := range sources {
		if source.Kind == kind {
			return source
		}
	}
	t.Fatalf("source kind %q not found in %#v", kind, sources)
	return ContextSource{}
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func writeContextPackTask(t *testing.T, dir string, domain string) string {
	t.Helper()

	content := `id: context-task-` + domain + `
title: Context Pack ` + domain + `
domain: ` + domain + `
worker: codex
goal: Build a context pack for ` + domain + `
mode: read_only
workspace:
  strategy: local_repo
  path: .
memory:
  scope: domain
validation:
  commands:
    - name: go-test
      command: go
      args:
        - test
        - ./...
allowed_paths: []
forbidden_paths:
  - secrets/**
expected_outputs:
  - artifacts/context.md
definition_of_done:
  - context pack exists
`
	path := filepath.Join(dir, "task-"+domain+".yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write task file: %v", err)
	}
	return path
}

func writeContextPackDomains(t *testing.T, dir string, bridgePath string) string {
	t.Helper()

	content := `domains:
  general:
    type: canonical_memory
    root: /vault/mysecondbrain
    default: true

  escalasoft:
    type: isolated_domain
    root: /domains/escalasoft_brain
    default: false
    bridge_files:
      - ` + filepath.ToSlash(bridgePath) + `
    structured_data:
      historical_sqlite: /data/escalasoft.db
    staging:
      - /tmp/escalasoft
    default_agent: escalasoft-agent
`
	path := filepath.Join(dir, "domains.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write domains file: %v", err)
	}
	return path
}
