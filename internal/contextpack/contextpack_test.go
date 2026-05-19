package contextpack

import (
	"context"
	"os"
	"path/filepath"
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
	if !strings.Contains(string(pack.Markdown()), "Warnings") {
		t.Fatalf("markdown = %q, want warnings section", string(pack.Markdown()))
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
