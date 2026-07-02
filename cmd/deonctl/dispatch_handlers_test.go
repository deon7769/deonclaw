package main

import "testing"

func TestDispatchRunningInCI(t *testing.T) {
	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "")
	if dispatchRunningInCI() {
		t.Fatal("expected CI disabled when both env vars are empty")
	}
	t.Setenv("CI", "false")
	if dispatchRunningInCI() {
		t.Fatal("expected CI=false to be disabled")
	}
	t.Setenv("CI", "1")
	if !dispatchRunningInCI() {
		t.Fatal("expected CI=1 to be enabled")
	}
	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "true")
	if !dispatchRunningInCI() {
		t.Fatal("expected GITHUB_ACTIONS=true to be enabled")
	}
}

func TestParseWorkDispatchOnceOptionsRuntimeControls(t *testing.T) {
	opts, err := parseWorkDispatchOnceOptions([]string{
		"--store", "deonclaw.db",
		"--work-item", "work_1",
		"--artifacts-dir", "artifacts",
		"--registry-root", "skills",
		"--skill-policy", "skill-policy.yaml",
		"--timeout-seconds", "45",
		"--lease-ttl-seconds", "120",
	})
	if err != nil {
		t.Fatalf("parseWorkDispatchOnceOptions() error = %v", err)
	}
	if opts.timeoutSeconds != 45 {
		t.Fatalf("timeoutSeconds = %d, want 45", opts.timeoutSeconds)
	}
	if opts.leaseTTLSeconds != 120 {
		t.Fatalf("leaseTTLSeconds = %d, want 120", opts.leaseTTLSeconds)
	}
}

func TestParseWorkDispatchOnceOptionsRejectsInvalidRuntimeControls(t *testing.T) {
	base := []string{
		"--store", "deonclaw.db",
		"--work-item", "work_1",
		"--artifacts-dir", "artifacts",
		"--registry-root", "skills",
		"--skill-policy", "skill-policy.yaml",
	}
	if _, err := parseWorkDispatchOnceOptions(append(append([]string{}, base...), "--timeout-seconds", "0")); err == nil {
		t.Fatal("expected invalid --timeout-seconds error")
	}
	if _, err := parseWorkDispatchOnceOptions(append(append([]string{}, base...), "--lease-ttl-seconds", "-1")); err == nil {
		t.Fatal("expected invalid --lease-ttl-seconds error")
	}
}
