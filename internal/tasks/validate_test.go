package tasks

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateValidFixture(t *testing.T) {
	task, err := LoadFromFile(filepath.Join("testdata", "valid.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(task); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateMissingID(t *testing.T) {
	task, err := LoadFromFile(filepath.Join("testdata", "missing-id.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	err = Validate(task)
	if err == nil {
		t.Fatal("Validate() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "id is required") {
		t.Fatalf("error = %v, want id is required", err)
	}
}

func TestValidateBadMode(t *testing.T) {
	task, err := LoadFromFile(filepath.Join("testdata", "bad-mode.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	err = Validate(task)
	if err == nil {
		t.Fatal("Validate() expected error, got nil")
	}
	if !strings.Contains(err.Error(), `mode "invalid_mode" is not supported`) {
		t.Fatalf("error = %v, want unsupported mode", err)
	}
}
