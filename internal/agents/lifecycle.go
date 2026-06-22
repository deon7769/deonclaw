package agents

import (
	"errors"
	"fmt"
)

func CanReceiveWork(agent Agent) error {
	switch agent.Status {
	case StatusActive:
		return nil
	case StatusPaused:
		return errors.New("agent is paused")
	case StatusDraining:
		return errors.New("agent is draining and cannot receive new work")
	case StatusTerminated:
		return errors.New("agent is terminated")
	case StatusQuarantined:
		return errors.New("agent is quarantined")
	default:
		return fmt.Errorf("agent status %q cannot receive work", agent.Status)
	}
}

func CanStartRun(agent Agent) error {
	switch agent.Status {
	case StatusActive, StatusDraining:
		return nil
	case StatusPaused:
		return errors.New("agent is paused")
	case StatusTerminated:
		return errors.New("agent is terminated")
	case StatusQuarantined:
		return errors.New("agent is quarantined")
	default:
		return fmt.Errorf("agent status %q cannot start runs", agent.Status)
	}
}

func PauseTargetStatus(current string) (string, error) {
	switch current {
	case StatusActive, StatusDraining:
		return StatusPaused, nil
	case StatusPaused:
		return "", errors.New("agent is already paused")
	case StatusTerminated:
		return "", errors.New("terminated agent cannot be paused")
	case StatusQuarantined:
		return "", errors.New("quarantined agent cannot be paused")
	default:
		return "", fmt.Errorf("agent status %q cannot transition to paused", current)
	}
}

func ResumeTargetStatus(current string) (string, error) {
	switch current {
	case StatusPaused:
		return StatusActive, nil
	case StatusActive:
		return "", errors.New("agent is already active")
	case StatusTerminated:
		return "", errors.New("terminated agent cannot be resumed")
	case StatusQuarantined:
		return "", errors.New("quarantined agent cannot be resumed")
	default:
		return "", fmt.Errorf("agent status %q cannot transition to active", current)
	}
}

func TerminateTargetStatus(current string) (string, error) {
	switch current {
	case StatusTerminated:
		return "", errors.New("agent is already terminated")
	case StatusQuarantined:
		return StatusTerminated, nil
	case StatusActive, StatusPaused, StatusDraining:
		return StatusTerminated, nil
	default:
		return "", fmt.Errorf("agent status %q cannot transition to terminated", current)
	}
}
