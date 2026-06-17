package memoryindex

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateConfiguredOutputPathRejectsAbsolute(t *testing.T) {
	err := validateConfiguredOutputPath("manifest", "/tmp/manifest.json")
	if err == nil || !strings.Contains(err.Error(), "must be relative to artifacts dir") {
		t.Fatalf("error = %v, want relative-to-artifacts failure", err)
	}
}

func TestValidateConfiguredOutputPathRejectsParentTraversal(t *testing.T) {
	err := validateConfiguredOutputPath("chunks", "../escape.jsonl")
	if err == nil || !strings.Contains(err.Error(), "must not contain ..") {
		t.Fatalf("error = %v, want parent traversal failure", err)
	}
}

func TestValidateConfiguredOutputPathRejectsSecrets(t *testing.T) {
	err := validateConfiguredOutputPath("manifest", "secrets/manifest.json")
	if err == nil || !strings.Contains(err.Error(), "blocked path") {
		t.Fatalf("error = %v, want blocked path failure", err)
	}
}

func TestResolveOutputPathStaysInsideArtifactsDir(t *testing.T) {
	root := t.TempDir()
	artifactsDir := filepath.Join(root, "artifacts")
	resolved, err := resolveOutputPath(artifactsDir, "manifest", "indexes/manifest.json")
	if err != nil {
		t.Fatalf("resolveOutputPath() error = %v", err)
	}
	want := filepath.Join(artifactsDir, "indexes", "manifest.json")
	if resolved != want {
		t.Fatalf("resolved = %q, want %q", resolved, want)
	}
	if !pathWithinRoot(artifactsDir, resolved) {
		t.Fatalf("resolved path %q is outside artifacts dir %q", resolved, artifactsDir)
	}
}
