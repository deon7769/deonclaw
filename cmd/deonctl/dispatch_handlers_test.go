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
