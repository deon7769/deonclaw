package runs

import "testing"

func TestRunStatusIsTerminal(t *testing.T) {
	tests := []struct {
		status   RunStatus
		terminal bool
	}{
		{StatusPending, false},
		{StatusRunning, false},
		{StatusSucceeded, true},
		{StatusFailed, true},
		{StatusPolicyFailed, true},
		{StatusCancelled, true},
	}

	for _, tt := range tests {
		if got := tt.status.IsTerminal(); got != tt.terminal {
			t.Errorf("%s.IsTerminal() = %v, want %v", tt.status, got, tt.terminal)
		}
	}
}
