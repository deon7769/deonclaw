package worktemplates

import (
	"fmt"
	"strings"
)

type Template struct {
	ID              string `yaml:"-" json:"id"`
	TaskPath        string `yaml:"task_path" json:"task_path"`
	AssignedAgentID string `yaml:"assigned_agent_id" json:"assigned_agent_id"`
	Priority        int    `yaml:"priority" json:"priority"`
	MaxAttempts     int    `yaml:"max_attempts" json:"max_attempts"`
	BudgetPolicy    string `yaml:"budget_policy" json:"budget_policy"`
	AssignmentMode  string `yaml:"assignment_mode" json:"assignment_mode"`
	SessionKind     string `yaml:"session_kind" json:"session_kind"`
}

func ValidateTemplate(t Template) error {
	if strings.TrimSpace(t.ID) == "" {
		return fmt.Errorf("work template id is required")
	}
	if strings.TrimSpace(t.TaskPath) == "" {
		return fmt.Errorf("work template %q task_path is required", t.ID)
	}
	if strings.TrimSpace(t.AssignedAgentID) == "" {
		return fmt.Errorf("work template %q assigned_agent_id is required", t.ID)
	}
	if t.Priority == 0 {
		return fmt.Errorf("work template %q priority is required", t.ID)
	}
	if t.MaxAttempts == 0 {
		return fmt.Errorf("work template %q max_attempts is required", t.ID)
	}
	if strings.TrimSpace(t.BudgetPolicy) == "" {
		return fmt.Errorf("work template %q budget_policy is required", t.ID)
	}
	mode := strings.TrimSpace(t.AssignmentMode)
	if mode != "manual" && mode != "auto_accept" {
		return fmt.Errorf("work template %q assignment_mode %q is not supported", t.ID, t.AssignmentMode)
	}
	return nil
}
