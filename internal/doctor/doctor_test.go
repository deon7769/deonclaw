package doctor

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorTextWorks(t *testing.T) {
	report, err := Build(Options{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	var out bytes.Buffer
	if err := Write(report, OutputText, &out); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "deonctl:") || !strings.Contains(text, "working_dir:") || !strings.Contains(text, "worker codex:") {
		t.Fatalf("doctor text = %q, want core fields", text)
	}
}

func TestDoctorJSONIsValid(t *testing.T) {
	report, err := Build(Options{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	var out bytes.Buffer
	if err := Write(report, OutputJSON, &out); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if !json.Valid(out.Bytes()) {
		t.Fatalf("invalid JSON: %s", out.String())
	}
}

func TestWorkersDoctorDetectsFakeCommandOnPath(t *testing.T) {
	tempDir := t.TempDir()
	fake := filepath.Join(tempDir, "fake-opencode")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatalf("WriteFile(fake) error = %v", err)
	}
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	configPath := filepath.Join(tempDir, "workers.yaml")
	if err := os.WriteFile(configPath, []byte("workers:\n  opencode:\n    command: fake-opencode\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}

	report, err := Build(Options{Worker: "opencode", WorkersConfigPath: configPath})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(report.Workers) != 1 {
		t.Fatalf("workers = %d, want 1", len(report.Workers))
	}
	worker := report.Workers[0]
	if worker.ConfiguredCommand != "fake-opencode" || !worker.Available || worker.Path != fake {
		t.Fatalf("worker check = %#v, want fake command available", worker)
	}
}

func TestDoctorChecksStoreAndArtifactsPaths(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "store.db")
	if err := os.WriteFile(storePath, []byte(""), 0o600); err != nil {
		t.Fatalf("WriteFile(store) error = %v", err)
	}
	report, err := Build(Options{StorePath: storePath, ArtifactsDir: tempDir})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if report.StorePath == nil || !report.StorePath.Exists || !report.StorePath.Writable {
		t.Fatalf("store check = %#v, want exists writable", report.StorePath)
	}
	if report.ArtifactsDir == nil || !report.ArtifactsDir.Exists || !report.ArtifactsDir.Writable {
		t.Fatalf("artifacts check = %#v, want exists writable", report.ArtifactsDir)
	}
}
