package doctor

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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
	if strings.Contains(text, "worker kimi:") {
		t.Fatalf("doctor text = %q, want no kimi worker by default", text)
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
	fake := writeDoctorFakeCommand(t, tempDir, "fake-opencode")
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

func TestWorkersDoctorReportsProviderModel(t *testing.T) {
	tempDir := t.TempDir()
	writeDoctorFakeCommand(t, tempDir, "fake-opencode")
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	configPath := filepath.Join(tempDir, "workers.yaml")
	if err := os.WriteFile(configPath, []byte(`workers:
  opencode:
    command: fake-opencode
    provider: z_ai_glm
    model: glm-5.1
    env:
      ZAI_API_KEY: required
`), 0o600); err != nil {
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
	if worker.Provider != "z_ai_glm" || worker.Model != "glm-5.1" {
		t.Fatalf("worker check = %#v, want provider/model", worker)
	}

	var jsonOut bytes.Buffer
	if err := Write(report, OutputJSON, &jsonOut); err != nil {
		t.Fatalf("Write(json) error = %v", err)
	}
	if !strings.Contains(jsonOut.String(), `"provider": "z_ai_glm"`) || !strings.Contains(jsonOut.String(), `"model": "glm-5.1"`) {
		t.Fatalf("json output = %q, want provider/model", jsonOut.String())
	}

	var textOut bytes.Buffer
	if err := Write(report, OutputText, &textOut); err != nil {
		t.Fatalf("Write(text) error = %v", err)
	}
	if !strings.Contains(textOut.String(), "provider=z_ai_glm") || !strings.Contains(textOut.String(), "model=glm-5.1") {
		t.Fatalf("text output = %q, want provider/model", textOut.String())
	}
}

func TestWorkersDoctorReportsMissingRequiredEnv(t *testing.T) {
	unsetEnvForTest(t, "ZAI_API_KEY")
	configPath := writeDoctorWorkersConfig(t, `workers:
  opencode:
    command: opencode
    env:
      ZAI_API_KEY: required
`)

	report, err := Build(Options{Worker: "opencode", WorkersConfigPath: configPath})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	worker := report.Workers[0]
	if worker.EnvRequiredOK {
		t.Fatalf("EnvRequiredOK = true, want false for missing required env")
	}
	if len(worker.EnvRequirements) != 1 || worker.EnvRequirements[0].Name != "ZAI_API_KEY" || worker.EnvRequirements[0].Requirement != "required" || worker.EnvRequirements[0].State != "missing" {
		t.Fatalf("env requirements = %#v, want missing ZAI_API_KEY", worker.EnvRequirements)
	}

	var out bytes.Buffer
	if err := Write(report, OutputText, &out); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if !strings.Contains(out.String(), "env_required_ok=false") || !strings.Contains(out.String(), "worker opencode env ZAI_API_KEY: requirement=required state=missing") {
		t.Fatalf("text output = %q, want missing env indication", out.String())
	}
}

func TestWorkersDoctorReportsSetMaskedRequiredEnv(t *testing.T) {
	t.Setenv("ZAI_API_KEY", "zai-secret-value")
	configPath := writeDoctorWorkersConfig(t, `workers:
  opencode:
    command: opencode
    env:
      ZAI_API_KEY: required
`)

	report, err := Build(Options{Worker: "opencode", WorkersConfigPath: configPath})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	worker := report.Workers[0]
	if !worker.EnvRequiredOK {
		t.Fatalf("EnvRequiredOK = false, want true for set required env")
	}
	if len(worker.EnvRequirements) != 1 || worker.EnvRequirements[0].State != "set_masked" {
		t.Fatalf("env requirements = %#v, want set_masked", worker.EnvRequirements)
	}

	var out bytes.Buffer
	if err := Write(report, OutputJSON, &out); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	text := out.String()
	if !strings.Contains(text, `"state": "set_masked"`) {
		t.Fatalf("json output = %q, want set_masked", text)
	}
	if strings.Contains(text, "zai-secret-value") {
		t.Fatalf("json output leaked secret: %q", text)
	}
}

func TestWorkersDoctorListsModelProfilesWhenRequested(t *testing.T) {
	unsetEnvForTest(t, "ZAI_API_KEY")
	configPath := writeDoctorWorkersConfig(t, `workers:
  opencode:
    command: opencode
model_profiles:
  opencode-zai-glm-5-1:
    worker: opencode
    provider: z-ai
    model: glm-5.1
    model_arg: z-ai/glm-5.1
    env:
      ZAI_API_KEY: required
    tags:
      - coding
      - general
`)

	report, err := Build(Options{Worker: "opencode", WorkersConfigPath: configPath, IncludeProfiles: true})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(report.ModelProfiles) != 1 {
		t.Fatalf("model_profiles = %#v, want one profile", report.ModelProfiles)
	}
	profile := report.ModelProfiles[0]
	if profile.Name != "opencode-zai-glm-5-1" || profile.Worker != "opencode" || profile.Provider != "z-ai" || profile.Model != "glm-5.1" || profile.ModelArg != "z-ai/glm-5.1" {
		t.Fatalf("profile = %#v, want opencode z-ai glm profile", profile)
	}
	if profile.EnvRequiredOK || len(profile.EnvRequirements) != 1 || profile.EnvRequirements[0].State != "missing" {
		t.Fatalf("profile env = %#v env_ok=%t, want missing", profile.EnvRequirements, profile.EnvRequiredOK)
	}

	var textOut bytes.Buffer
	if err := Write(report, OutputText, &textOut); err != nil {
		t.Fatalf("Write(text) error = %v", err)
	}
	text := textOut.String()
	if !strings.Contains(text, "model_profile opencode-zai-glm-5-1: worker=opencode env_required_ok=false provider=z-ai model=glm-5.1 model_arg=z-ai/glm-5.1 tags=coding,general") {
		t.Fatalf("text output = %q, want model profile details", text)
	}
	if !strings.Contains(text, "model_profile opencode-zai-glm-5-1 env ZAI_API_KEY: requirement=required state=missing") {
		t.Fatalf("text output = %q, want model profile env requirement", text)
	}
}

func TestWorkersDoctorDoesNotListModelProfilesByDefault(t *testing.T) {
	configPath := writeDoctorWorkersConfig(t, `workers:
  opencode:
    command: opencode
model_profiles:
  opencode-zai-glm-5-1:
    worker: opencode
    provider: z-ai
    model: glm-5.1
`)

	report, err := Build(Options{Worker: "opencode", WorkersConfigPath: configPath})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(report.ModelProfiles) != 0 {
		t.Fatalf("model_profiles = %#v, want none by default", report.ModelProfiles)
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

func TestWorkersDoctorRejectsKimiAsInspirationOnly(t *testing.T) {
	report, err := Build(Options{Worker: "kimi"})
	if err == nil {
		t.Fatalf("Build() workers = %#v, want kimi rejected", report.Workers)
	}
	if !strings.Contains(err.Error(), `unknown worker "kimi"`) {
		t.Fatalf("error = %v, want kimi unknown worker", err)
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

func writeDoctorWorkersConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "workers.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}
	return path
}

func writeDoctorFakeCommand(t *testing.T, dir string, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	content := "#!/bin/sh\nexit 0\n"
	if runtime.GOOS == "windows" {
		path += ".bat"
		content = "@echo off\r\nexit /b 0\r\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatalf("WriteFile(fake) error = %v", err)
	}
	return path
}

func unsetEnvForTest(t *testing.T, name string) {
	t.Helper()
	oldValue, hadOldValue := os.LookupEnv(name)
	if err := os.Unsetenv(name); err != nil {
		t.Fatalf("Unsetenv(%s) error = %v", name, err)
	}
	t.Cleanup(func() {
		if hadOldValue {
			_ = os.Setenv(name, oldValue)
		} else {
			_ = os.Unsetenv(name)
		}
	})
}
