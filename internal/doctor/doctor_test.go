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
	if report.StorePath == nil || !report.StorePath.Exists || !report.StorePath.Writable || report.StorePath.State != "exists_writable" {
		t.Fatalf("store check = %#v, want exists writable", report.StorePath)
	}
	if report.ArtifactsDir == nil || !report.ArtifactsDir.Exists || !report.ArtifactsDir.Writable || report.ArtifactsDir.State != "exists_writable" {
		t.Fatalf("artifacts check = %#v, want exists writable", report.ArtifactsDir)
	}
}

func TestDoctorStorePathExistingDirectoryInvalid(t *testing.T) {
	tempDir := t.TempDir()
	report, err := Build(Options{StorePath: tempDir})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if report.StorePath == nil {
		t.Fatal("StorePath = nil, want check")
	}
	if report.StorePath.State != "invalid_directory" || !report.StorePath.Exists || report.StorePath.Writable {
		t.Fatalf("store check = %#v, want invalid_directory", report.StorePath)
	}
}

func TestDoctorArtifactsPathExistingFileInvalid(t *testing.T) {
	tempDir := t.TempDir()
	artifactsPath := filepath.Join(tempDir, "artifacts-file")
	if err := os.WriteFile(artifactsPath, []byte("not a dir"), 0o600); err != nil {
		t.Fatalf("WriteFile(artifacts) error = %v", err)
	}
	report, err := Build(Options{ArtifactsDir: artifactsPath})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if report.ArtifactsDir == nil {
		t.Fatal("ArtifactsDir = nil, want check")
	}
	if report.ArtifactsDir.State != "invalid_file" || !report.ArtifactsDir.Exists || report.ArtifactsDir.Writable {
		t.Fatalf("artifacts check = %#v, want invalid_file", report.ArtifactsDir)
	}
}

func TestDoctorChecksMissingStoreParentWritable(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "missing.db")
	report, err := Build(Options{StorePath: storePath})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if report.StorePath == nil {
		t.Fatal("StorePath = nil, want check")
	}
	if report.StorePath.Exists {
		t.Fatalf("store check = %#v, want missing path", report.StorePath)
	}
	if report.StorePath.State != "missing_parent_writable" || !report.StorePath.Writable {
		t.Fatalf("store check = %#v, want missing_parent_writable", report.StorePath)
	}
}

func TestDoctorChecksMissingStoreParentNotWritable(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "missing-parent", "store.db")
	report, err := Build(Options{StorePath: storePath})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if report.StorePath == nil {
		t.Fatal("StorePath = nil, want check")
	}
	if report.StorePath.State != "missing_parent_not_writable" || report.StorePath.Writable {
		t.Fatalf("store check = %#v, want missing_parent_not_writable", report.StorePath)
	}
}

func TestDoctorChecksArtifactsMissingParentWritable(t *testing.T) {
	tempDir := t.TempDir()
	artifactsPath := filepath.Join(tempDir, "artifacts")
	report, err := Build(Options{ArtifactsDir: artifactsPath})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if report.ArtifactsDir == nil {
		t.Fatal("ArtifactsDir = nil, want check")
	}
	if report.ArtifactsDir.Exists {
		t.Fatalf("artifacts check = %#v, want missing path", report.ArtifactsDir)
	}
	if report.ArtifactsDir.State != "missing_parent_writable" || !report.ArtifactsDir.Writable {
		t.Fatalf("artifacts check = %#v, want missing_parent_writable", report.ArtifactsDir)
	}
	if _, err := os.Stat(artifactsPath); !os.IsNotExist(err) {
		t.Fatalf("artifacts dir was left behind: %v", err)
	}
}

func TestDoctorChecksArtifactsMissingParentNotWritable(t *testing.T) {
	tempDir := t.TempDir()
	artifactsPath := filepath.Join(tempDir, "missing-parent", "artifacts")
	report, err := Build(Options{ArtifactsDir: artifactsPath})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if report.ArtifactsDir == nil {
		t.Fatal("ArtifactsDir = nil, want check")
	}
	if report.ArtifactsDir.State != "missing_parent_not_writable" || report.ArtifactsDir.Writable {
		t.Fatalf("artifacts check = %#v, want missing_parent_not_writable", report.ArtifactsDir)
	}
}

func TestWorkersDoctorAllowsKimiAsFutureWorkerDiagnostic(t *testing.T) {
	report, err := Build(Options{Worker: "kimi"})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(report.Workers) != 1 || report.Workers[0].Name != "kimi" || report.Workers[0].ConfiguredCommand != "kimi" || report.Workers[0].ImplementationStatus != "future_worker" {
		t.Fatalf("workers = %#v, want kimi future_worker diagnostic", report.Workers)
	}
	var out bytes.Buffer
	if err := Write(report, OutputText, &out); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if !strings.Contains(out.String(), "status=future_worker") {
		t.Fatalf("text output = %q, want future_worker status", out.String())
	}
}

func TestWorkersDoctorRejectsUnknownWorker(t *testing.T) {
	_, err := Build(Options{Worker: "unknown"})
	if err == nil {
		t.Fatal("Build() expected error, got nil")
	}
	if !strings.Contains(err.Error(), `unknown worker "unknown"`) {
		t.Fatalf("error = %v, want unknown worker", err)
	}
}
