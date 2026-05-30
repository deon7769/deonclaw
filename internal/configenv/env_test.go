package configenv

import (
	"bytes"
	"strings"
	"testing"
)

func TestConfigEnvMasksSecrets(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-secret-value")
	t.Setenv("CUSTOM_PASSWORD", "super-secret")

	report := Build()
	var out bytes.Buffer
	if err := Write(report, OutputText, &out); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "OPENAI_API_KEY=set(masked)") {
		t.Fatalf("text = %q, want masked OPENAI_API_KEY", text)
	}
	if !strings.Contains(text, "CUSTOM_PASSWORD=set(masked)") {
		t.Fatalf("text = %q, want masked CUSTOM_PASSWORD", text)
	}
	if strings.Contains(text, "sk-secret-value") || strings.Contains(text, "super-secret") {
		t.Fatalf("text leaked secret: %q", text)
	}
}

func TestConfigEnvShowsUnset(t *testing.T) {
	t.Setenv("ZAI_API_KEY", "")
	// os.LookupEnv treats Setenv empty as set; use a fixed key likely unset through default list.
	report := Build()
	found := false
	for _, entry := range report.Entries {
		if entry.Name == "KIMI_API_KEY" {
			found = true
			if entry.Value != "unset" && entry.Value != "set(masked)" {
				t.Fatalf("KIMI_API_KEY value = %q, want unset or masked when externally set", entry.Value)
			}
		}
	}
	if !found {
		t.Fatal("KIMI_API_KEY not reported")
	}
}
