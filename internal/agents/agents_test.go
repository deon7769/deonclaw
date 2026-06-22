package agents

import (
	"testing"

	"github.com/deon7769/deonclaw/internal/tasks"
	"github.com/deon7769/deonclaw/internal/workerconfig"
)

func TestValidateConfigOK(t *testing.T) {
	cfg, err := ParseConfig([]byte(exampleAgentsYAML))
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	workers := workerconfig.Default()
	workers.ModelProfiles = map[string]workerconfig.ModelProfile{
		"opencode-zai-glm-5-1": {Worker: "opencode"},
	}
	if err := ValidateConfig(cfg, workers); err != nil {
		t.Fatalf("ValidateConfig() error = %v", err)
	}
}

func TestValidateConfigRejectsDuplicateIDs(t *testing.T) {
	cfg, err := ParseConfig([]byte(exampleAgentsYAML))
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	cfg.Agents.List = append(cfg.Agents.List, cfg.Agents.List[0])
	if err := ValidateConfig(cfg, workerconfig.Default()); err == nil {
		t.Fatal("ValidateConfig() expected duplicate id error")
	}
}

func TestValidateConfigRejectsUnknownSupervisor(t *testing.T) {
	cfg, err := ParseConfig([]byte(exampleAgentsYAML))
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	cfg.Agents.List[1].SupervisorID = "missing-manager"
	workers := workerconfig.Default()
	workers.ModelProfiles = map[string]workerconfig.ModelProfile{
		"opencode-zai-glm-5-1": {Worker: "opencode"},
	}
	if err := ValidateConfig(cfg, workers); err == nil {
		t.Fatal("ValidateConfig() expected unknown supervisor error")
	}
}

func TestPauseBlocksAssignment(t *testing.T) {
	agent := Agent{ID: "backend-engineer", Status: StatusPaused, DefaultWorker: "codex"}
	if err := CanReceiveWork(agent); err == nil {
		t.Fatal("CanReceiveWork() expected error for paused agent")
	}
}

func TestTerminatedAgentCannotRun(t *testing.T) {
	agent := Agent{ID: "backend-engineer", Status: StatusTerminated, DefaultWorker: "codex"}
	if err := CanStartRun(agent); err == nil {
		t.Fatal("CanStartRun() expected error for terminated agent")
	}
}

func TestDelegationDepthGuard(t *testing.T) {
	parent := Agent{ID: "parent", Status: StatusActive, Role: "engineer", DefaultWorker: "codex"}
	child := Agent{ID: "child", Status: StatusActive, Role: "engineer", DefaultWorker: "codex"}
	parentWork := WorkItem{
		ID:           "work_parent",
		AllowedPaths: []string{"internal/**", "docs/**"},
	}
	task := tasks.Task{
		ID:           "task-1",
		Title:        "Child task",
		AllowedPaths: []string{"internal/skills/**"},
	}
	_, err := BuildDelegationProposal(BuildDelegationProposalOptions{
		ParentAgent:    parent,
		ChildAgent:     child,
		ParentWorkItem: parentWork,
		Task:           task,
		Reason:         "Delegate skill work",
		Depth:          3,
		MaxDepth:       2,
	})
	if err == nil {
		t.Fatal("BuildDelegationProposal() expected depth error")
	}
}

func TestChildPathsMustBeSubset(t *testing.T) {
	parent := Agent{ID: "parent", Status: StatusActive, Role: "engineer", DefaultWorker: "codex"}
	child := Agent{ID: "child", Status: StatusActive, Role: "engineer", DefaultWorker: "codex"}
	parentWork := WorkItem{
		ID:           "work_parent",
		AllowedPaths: []string{"internal/skills/**"},
	}
	task := tasks.Task{
		ID:           "task-1",
		Title:        "Child task",
		AllowedPaths: []string{"memory/**"},
	}
	_, err := BuildDelegationProposal(BuildDelegationProposalOptions{
		ParentAgent:    parent,
		ChildAgent:     child,
		ParentWorkItem: parentWork,
		Task:           task,
		Reason:         "Delegate memory work",
		Depth:          1,
	})
	if err == nil {
		t.Fatal("BuildDelegationProposal() expected path subset error")
	}
}

func TestWorkItemFromTask(t *testing.T) {
	item, err := WorkItemFromTask(WorkItemFromTaskOptions{
		Task: tasks.Task{
			ID:               "task-abc",
			Title:            "Implement agents",
			AllowedPaths:     []string{"internal/agents/**"},
			ForbiddenPaths:   []string{"memory/**"},
			DefinitionOfDone: []string{"go test ./..."},
		},
		AssignedAgentID: "backend-engineer",
	})
	if err != nil {
		t.Fatalf("WorkItemFromTask() error = %v", err)
	}
	if item.TaskID != "task-abc" {
		t.Fatalf("task_id = %q", item.TaskID)
	}
	if item.Status != WorkItemStatusAssigned {
		t.Fatalf("status = %q", item.Status)
	}
	if item.CreatedAt == "" {
		t.Fatal("created_at is empty")
	}
}

func TestSessionSnapshotImmutable(t *testing.T) {
	existing := Session{ID: "ses_1", AgentID: "a", Kind: SessionKindMain, SkillSnapshotPath: "/tmp/snap.json"}
	updated := Session{ID: "ses_1", AgentID: "a", Kind: SessionKindMain, SkillSnapshotPath: "/tmp/other.json"}
	if err := ValidateSessionImmutable(existing, updated); err == nil {
		t.Fatal("ValidateSessionImmutable() expected error")
	}
}

const exampleAgentsYAML = `
agents:
  defaults:
    workspace_strategy: worktree-per-run
    memory_scope: mysecondbrain
  list:
    - id: engineering-manager
      display_name: Engineering Manager
      role: manager
      default_worker: codex
      model_profile: opencode-zai-glm-5-1
    - id: backend-engineer
      display_name: Backend Engineer
      role: engineer
      supervisor_id: engineering-manager
      default_worker: opencode
      model_profile: opencode-zai-glm-5-1
`
