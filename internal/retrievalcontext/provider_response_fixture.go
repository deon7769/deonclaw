package retrievalcontext

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

const ProviderResponseFixtureSource = "fixture"

type ProviderResponseFixtureGenerateOptions struct {
	RequestEnvelopePath string
	AdapterPlanPath     string
	OutputPath          string
}

type ProviderResponseFixtureResult struct {
	Status                string   `json:"status"`
	ResponseFixtureReady  bool     `json:"response_fixture_ready"`
	ResponseSource        string   `json:"response_source"`
	ProviderCall          bool     `json:"provider_call"`
	NetworkCall           bool     `json:"network_call"`
	TransportCalled       bool     `json:"transport_called"`
	SentToProvider        bool     `json:"sent_to_provider"`
	ReceivedFromProvider  bool     `json:"received_from_provider"`
	WorkerExecution       bool     `json:"worker_execution"`
	BlockedReason         string   `json:"blocked_reason"`
	Provider              string   `json:"provider"`
	RequestEnvelopeSHA256 string   `json:"request_envelope_sha256"`
	AdapterPlanSHA256     string   `json:"adapter_plan_sha256"`
	ResponseFixtureID     string   `json:"response_fixture_id"`
	ResponseFixtureSHA256 string   `json:"response_fixture_sha256,omitempty"`
	Warnings              []string `json:"warnings,omitempty"`
	Failures              []string `json:"failures,omitempty"`
}

type ProviderResponseFixtureInspectOptions struct {
	RequestEnvelopePath string
	AdapterPlanPath     string
}

type ProviderResponseFixtureInspectResult struct {
	Status               string   `json:"status"`
	ResponseFixtureReady bool     `json:"response_fixture_ready"`
	ResponseSource       string   `json:"response_source"`
	ProviderCall         bool     `json:"provider_call"`
	NetworkCall          bool     `json:"network_call"`
	TransportCalled      bool     `json:"transport_called"`
	SentToProvider       bool     `json:"sent_to_provider"`
	ReceivedFromProvider bool     `json:"received_from_provider"`
	WorkerExecution      bool     `json:"worker_execution"`
	ResponseFixtureID    string   `json:"response_fixture_id"`
	Warnings             []string `json:"warnings"`
	Failures             []string `json:"failures,omitempty"`
}

func ProviderResponseFixtureGenerate(opts ProviderResponseFixtureGenerateOptions) (ProviderResponseFixtureResult, error) {
	for _, check := range []struct {
		field string
		path  string
	}{
		{"request envelope path", opts.RequestEnvelopePath},
		{"adapter plan path", opts.AdapterPlanPath},
		{"output path", opts.OutputPath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return ProviderResponseFixtureResult{}, err
		}
	}

	envelope, envelopeData, err := LoadProviderRequestEnvelope(opts.RequestEnvelopePath)
	if err != nil {
		return ProviderResponseFixtureResult{}, err
	}
	adapterPlan, adapterData, err := LoadProviderAdapterPlan(opts.AdapterPlanPath)
	if err != nil {
		return ProviderResponseFixtureResult{}, err
	}

	var failures []string
	if !envelope.ProviderRequestReady {
		failures = append(failures, "request envelope provider_request_ready must be true")
	}
	if !adapterPlan.AdapterPlanReady {
		failures = append(failures, "adapter plan adapter_plan_ready must be true")
	}
	if adapterPlan.RequestEnvelopeSHA256 != sha256Hex(envelopeData) {
		failures = append(failures, "adapter plan request_envelope_sha256 mismatch")
	}

	fixtureID := deterministicResponseFixtureID(sha256Hex(envelopeData), sha256Hex(adapterData))
	result := ProviderResponseFixtureResult{
		Status:                lancedbpolicy.StatusOK,
		ResponseSource:        ProviderResponseFixtureSource,
		ProviderCall:          false,
		NetworkCall:           false,
		TransportCalled:       false,
		SentToProvider:        false,
		ReceivedFromProvider:  false,
		WorkerExecution:       false,
		BlockedReason:         ProviderCallExecutorBlockedReason,
		Provider:              adapterPlan.Provider,
		RequestEnvelopeSHA256: sha256Hex(envelopeData),
		AdapterPlanSHA256:     sha256Hex(adapterData),
		ResponseFixtureID:     fixtureID,
		Failures:              failures,
	}
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
		return result, nil
	}
	result.ResponseFixtureReady = true
	if err := writeProviderResponseFixtureJSON(opts.OutputPath, result); err != nil {
		return ProviderResponseFixtureResult{}, err
	}
	data, err := readArtifactBytesNoTextExcerpt("response fixture", opts.OutputPath)
	if err != nil {
		return ProviderResponseFixtureResult{}, err
	}
	result.ResponseFixtureSHA256 = sha256Hex(data)
	return result, nil
}

func ProviderResponseFixtureInspect(path string, opts ProviderResponseFixtureInspectOptions) (ProviderResponseFixtureInspectResult, error) {
	if err := validateRelativeSafePath("response fixture path", path); err != nil {
		return ProviderResponseFixtureInspectResult{}, err
	}
	data, err := readArtifactBytesNoTextExcerpt("response fixture", path)
	if err != nil {
		return ProviderResponseFixtureInspectResult{}, err
	}
	fixture, err := ParseProviderResponseFixtureJSON(data)
	if err != nil {
		return ProviderResponseFixtureInspectResult{}, err
	}

	result := ProviderResponseFixtureInspectResult{
		Status:               lancedbpolicy.StatusOK,
		ResponseFixtureReady: fixture.ResponseFixtureReady,
		ResponseSource:       fixture.ResponseSource,
		ProviderCall:         fixture.ProviderCall,
		NetworkCall:          fixture.NetworkCall,
		TransportCalled:      fixture.TransportCalled,
		SentToProvider:       fixture.SentToProvider,
		ReceivedFromProvider: fixture.ReceivedFromProvider,
		WorkerExecution:      fixture.WorkerExecution,
		ResponseFixtureID:    fixture.ResponseFixtureID,
		Warnings:             append([]string(nil), fixture.Warnings...),
	}

	var failures []string
	if !fixture.ResponseFixtureReady {
		failures = append(failures, "response_fixture_ready must be true")
	}
	if fixture.ResponseSource != ProviderResponseFixtureSource {
		failures = append(failures, fmt.Sprintf("response_source %q must be %q", fixture.ResponseSource, ProviderResponseFixtureSource))
	}
	if fixture.ProviderCall || fixture.NetworkCall || fixture.TransportCalled || fixture.SentToProvider || fixture.ReceivedFromProvider || fixture.WorkerExecution {
		failures = append(failures, "response fixture must keep execution flags blocked")
	}
	if opts.RequestEnvelopePath != "" {
		envelopeData, err := readArtifactBytesNoTextExcerpt("request envelope", opts.RequestEnvelopePath)
		if err != nil {
			return ProviderResponseFixtureInspectResult{}, err
		}
		if fixture.RequestEnvelopeSHA256 != sha256Hex(envelopeData) {
			failures = append(failures, "request_envelope_sha256 mismatch")
		}
	}
	if opts.AdapterPlanPath != "" {
		adapterData, err := readArtifactBytesNoTextExcerpt("adapter plan", opts.AdapterPlanPath)
		if err != nil {
			return ProviderResponseFixtureInspectResult{}, err
		}
		if fixture.AdapterPlanSHA256 != sha256Hex(adapterData) {
			failures = append(failures, "adapter_plan_sha256 mismatch")
		}
		wantID := deterministicResponseFixtureID(fixture.RequestEnvelopeSHA256, fixture.AdapterPlanSHA256)
		if fixture.ResponseFixtureID != wantID {
			failures = append(failures, "response_fixture_id mismatch with deterministic fixture id")
		}
	}
	result.Failures = failures
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
	}
	return result, nil
}

func LoadProviderResponseFixture(path string) (ProviderResponseFixtureResult, []byte, error) {
	data, err := readArtifactBytesNoTextExcerpt("response fixture", path)
	if err != nil {
		return ProviderResponseFixtureResult{}, nil, err
	}
	result, err := ParseProviderResponseFixtureJSON(data)
	if err != nil {
		return ProviderResponseFixtureResult{}, nil, err
	}
	return result, data, nil
}

func ParseProviderResponseFixtureJSON(data []byte) (ProviderResponseFixtureResult, error) {
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return ProviderResponseFixtureResult{}, fmt.Errorf("response fixture must not contain materialized preview text")
	}
	var result ProviderResponseFixtureResult
	if err := json.Unmarshal(data, &result); err != nil {
		return ProviderResponseFixtureResult{}, fmt.Errorf("parse response fixture json: %w", err)
	}
	return result, nil
}

func deterministicResponseFixtureID(requestEnvelopeSHA256, adapterPlanSHA256 string) string {
	sum := sha256.Sum256([]byte(requestEnvelopeSHA256 + ":" + adapterPlanSHA256 + ":fixture"))
	return "fixture-" + hex.EncodeToString(sum[:8])
}

func writeProviderResponseFixtureJSON(path string, result ProviderResponseFixtureResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create response fixture output dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal response fixture json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("response fixture must not contain materialized preview text")
	}
	return os.WriteFile(path, data, 0o644)
}

func WriteProviderResponseFixtureInspectJSON(result ProviderResponseFixtureInspectResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal response fixture inspect json: %w", err)
	}
	data = append(data, '\n')
	_, err = out.Write(data)
	return err
}

func WriteProviderResponseFixtureInspectText(result ProviderResponseFixtureInspectResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_response_fixture_inspect:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"response_fixture_ready", fmt.Sprintf("%t", result.ResponseFixtureReady)},
		{"response_source", result.ResponseSource},
		{"provider_call", fmt.Sprintf("%t", result.ProviderCall)},
		{"network_call", fmt.Sprintf("%t", result.NetworkCall)},
		{"transport_called", fmt.Sprintf("%t", result.TransportCalled)},
		{"sent_to_provider", fmt.Sprintf("%t", result.SentToProvider)},
		{"received_from_provider", fmt.Sprintf("%t", result.ReceivedFromProvider)},
		{"response_fixture_id", result.ResponseFixtureID},
	}
	for _, line := range lines {
		if _, err := fmt.Fprintf(out, "%s: %s\n", line.label, line.value); err != nil {
			return err
		}
	}
	if len(result.Failures) > 0 {
		if _, err := fmt.Fprintln(out, "\nfailures:"); err != nil {
			return err
		}
		for _, failure := range result.Failures {
			if _, err := fmt.Fprintf(out, "- %s\n", failure); err != nil {
				return err
			}
		}
	}
	return nil
}

func WriteProviderResponseFixtureGenerateJSON(result ProviderResponseFixtureResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal response fixture generate json: %w", err)
	}
	data = append(data, '\n')
	_, err = out.Write(data)
	return err
}

func WriteProviderResponseFixtureGenerateText(result ProviderResponseFixtureResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_response_fixture_generate:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"response_fixture_ready", fmt.Sprintf("%t", result.ResponseFixtureReady)},
		{"response_source", result.ResponseSource},
		{"provider_call", fmt.Sprintf("%t", result.ProviderCall)},
		{"received_from_provider", fmt.Sprintf("%t", result.ReceivedFromProvider)},
		{"response_fixture_id", result.ResponseFixtureID},
	}
	for _, line := range lines {
		if _, err := fmt.Fprintf(out, "%s: %s\n", line.label, line.value); err != nil {
			return err
		}
	}
	return nil
}
