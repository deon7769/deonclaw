package heartbeat

import "testing"

func TestValidateConfigOK(t *testing.T) {
	cfg, err := ParseConfig([]byte(exampleHeartbeatYAML))
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("ValidateConfig() error = %v", err)
	}
}

func TestDisabledHeartbeatEveryZero(t *testing.T) {
	cfg, err := ParseConfig([]byte(`
heartbeat_policies:
  off:
    enabled: false
    every: 0m
    no_op_token: HEARTBEAT_OK
`))
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("ValidateConfig() error = %v", err)
	}
}

func TestNoOpSuppressesNotification(t *testing.T) {
	policy := Policy{NoOpToken: "HEARTBEAT_OK", AckMaxChars: 32}
	result := EvaluateResult(policy, "HEARTBEAT_OK")
	if result.Notify || !result.NoOp {
		t.Fatalf("result = %+v", result)
	}
}

func TestSkipWhenBusyBlocksDryRun(t *testing.T) {
	plan := PlanDryRun("backend-engineer", "backend-default", Policy{Enabled: true, SkipWhenBusy: true, NoOpToken: "HEARTBEAT_OK"}, true)
	if plan.WouldRun {
		t.Fatal("WouldRun should be false when busy")
	}
}

const exampleHeartbeatYAML = `
heartbeat_policies:
  backend-default:
    enabled: true
    every: 30m
    no_op_token: HEARTBEAT_OK
`
