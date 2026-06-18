package retrievalcontext

import (
	"strings"
	"testing"
)

func TestValidateRelativeSafePathRejectsAbsolute(t *testing.T) {
	err := validateRelativeSafePath("output path", "/tmp/out.json")
	if err == nil || !strings.Contains(err.Error(), "must be a relative path") {
		t.Fatalf("error = %v, want absolute rejection", err)
	}
}

func TestValidateRelativeSafePathRejectsParentTraversal(t *testing.T) {
	err := validateRelativeSafePath("chunks path", "../chunks.jsonl")
	if err == nil || !strings.Contains(err.Error(), "must not contain ..") {
		t.Fatalf("error = %v, want parent traversal rejection", err)
	}
}

func TestValidateRelativeSafePathRejectsSecrets(t *testing.T) {
	err := validateRelativeSafePath("summary path", "secrets/summary.md")
	if err == nil || !strings.Contains(err.Error(), "blocked path") {
		t.Fatalf("error = %v, want blocked path rejection", err)
	}
}
