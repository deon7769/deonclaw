package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/deon7769/deonclaw/internal/agents"
	"github.com/deon7769/deonclaw/internal/store"
	"github.com/deon7769/deonclaw/internal/tasks"
	"github.com/deon7769/deonclaw/internal/workerconfig"
)

type agentsValidateOptions struct {
	configPath        string
	workersConfigPath string
}

type agentsStoreOptions struct {
	storePath string
}

type agentsSyncOptions struct {
	configPath        string
	workersConfigPath string
	storePath         string
}

type agentsLifecycleOptions struct {
	agentID      string
	storePath    string
	approvalPath string
	actor        string
}

type agentsSessionCreateOptions struct {
	agentID           string
	storePath         string
	kind              string
	sessionID         string
	workspacePath     string
	skillSnapshotPath string
}

type agentsSessionShowOptions struct {
	sessionID    string
	storePath    string
	outputFormat string
}

type agentsAssignOptions struct {
	agentID   string
	taskPath  string
	storePath string
	createdBy string
}

type agentsInboxOptions struct {
	agentID      string
	itemID       string
	storePath    string
	outputFormat string
}

type agentsDelegateProposeOptions struct {
	parentAgentID    string
	childAgentID     string
	parentWorkItemID string
	taskPath         string
	reason           string
	outputPath       string
	storePath        string
}

func openAgentStore(path string) (*store.SQLiteStore, error) {
	return store.OpenSQLite(path)
}

func runAgentsValidate(opts agentsValidateOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := agents.LoadConfig(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "agents validate failed: %v\n", err)
		return 1
	}
	workers, err := loadWorkersConfig(opts.workersConfigPath)
	if err != nil {
		fmt.Fprintf(stderr, "agents validate failed: %v\n", err)
		return 1
	}
	if err := agents.ValidateConfig(cfg, workers); err != nil {
		fmt.Fprintf(stderr, "agents validate failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "agents validate: ok")
	return 0
}

func loadWorkersConfig(path string) (workerconfig.Config, error) {
	if strings.TrimSpace(path) == "" {
		return workerconfig.Default(), nil
	}
	return workerconfig.Load(path)
}

func runAgentsSync(opts agentsSyncOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := agents.LoadConfig(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "agents sync failed: %v\n", err)
		return 1
	}
	workers, err := loadWorkersConfig(opts.workersConfigPath)
	if err != nil {
		fmt.Fprintf(stderr, "agents sync failed: %v\n", err)
		return 1
	}
	if err := agents.ValidateConfig(cfg, workers); err != nil {
		fmt.Fprintf(stderr, "agents sync failed: %v\n", err)
		return 1
	}
	db, err := openAgentStore(opts.storePath)
	if err != nil {
		fmt.Fprintf(stderr, "agents sync failed: %v\n", err)
		return 1
	}
	defer db.Close()

	ctx := context.Background()
	now := time.Now().UTC()
	for _, agent := range agents.SyncFromConfig(cfg, now) {
		_, loadErr := db.Agent(ctx, agent.ID)
		isNew := loadErr != nil
		if !isNew {
			existing, err := db.Agent(ctx, agent.ID)
			if err != nil {
				fmt.Fprintf(stderr, "agents sync failed: %v\n", err)
				return 1
			}
			agent.Status = existing.Status
			agent.CreatedAt = existing.CreatedAt
		}
		if err := db.SaveAgent(ctx, agent); err != nil {
			fmt.Fprintf(stderr, "agents sync failed: %v\n", err)
			return 1
		}
		eventType := agents.EventAgentUpdated
		if isNew {
			eventType = agents.EventAgentCreated
		}
		if err := db.SaveLifecycleEvent(ctx, agents.NewLifecycleEvent(agent.ID, eventType, "deonctl", "sync from config", now)); err != nil {
			fmt.Fprintf(stderr, "agents sync failed: %v\n", err)
			return 1
		}
	}
	fmt.Fprintf(stdout, "agents sync: ok count=%d\n", len(cfg.Agents.List))
	return 0
}

func runAgentsList(opts agentsStoreOptions, stdout io.Writer, stderr io.Writer) int {
	db, err := openAgentStore(opts.storePath)
	if err != nil {
		fmt.Fprintf(stderr, "agents list failed: %v\n", err)
		return 1
	}
	defer db.Close()
	items, err := db.ListAgents(context.Background())
	if err != nil {
		fmt.Fprintf(stderr, "agents list failed: %v\n", err)
		return 1
	}
	writer := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "id\tdisplay_name\trole\tstatus\tworker")
	for _, agent := range items {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", agent.ID, agent.DisplayName, agent.Role, agent.Status, agent.DefaultWorker)
	}
	_ = writer.Flush()
	return 0
}

func runAgentsShow(agentID string, opts agentsStoreOptions, stdout io.Writer, stderr io.Writer) int {
	db, err := openAgentStore(opts.storePath)
	if err != nil {
		fmt.Fprintf(stderr, "agents show failed: %v\n", err)
		return 1
	}
	defer db.Close()
	agent, err := db.Agent(context.Background(), agentID)
	if err != nil {
		fmt.Fprintf(stderr, "agents show failed: %v\n", err)
		return 1
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(agent); err != nil {
		fmt.Fprintf(stderr, "agents show failed: %v\n", err)
		return 1
	}
	return 0
}

func runAgentsPause(agentID string, opts agentsLifecycleOptions, stdout io.Writer, stderr io.Writer) int {
	return runAgentsLifecycleTransition(agentID, opts, agents.PauseTargetStatus, agents.EventAgentPaused, "agents pause", stdout, stderr)
}

func runAgentsResume(agentID string, opts agentsLifecycleOptions, stdout io.Writer, stderr io.Writer) int {
	return runAgentsLifecycleTransition(agentID, opts, agents.ResumeTargetStatus, agents.EventAgentResumed, "agents resume", stdout, stderr)
}

func runAgentsLifecycleTransition(agentID string, opts agentsLifecycleOptions, transition func(string) (string, error), eventType string, label string, stdout io.Writer, stderr io.Writer) int {
	db, err := openAgentStore(opts.storePath)
	if err != nil {
		fmt.Fprintf(stderr, "%s failed: %v\n", label, err)
		return 1
	}
	defer db.Close()
	ctx := context.Background()
	agent, err := db.Agent(ctx, agentID)
	if err != nil {
		fmt.Fprintf(stderr, "%s failed: %v\n", label, err)
		return 1
	}
	next, err := transition(agent.Status)
	if err != nil {
		fmt.Fprintf(stderr, "%s failed: %v\n", label, err)
		return 1
	}
	now := time.Now().UTC()
	agent.Status = next
	agent.UpdatedAt = now.Format(time.RFC3339Nano)
	if err := db.SaveAgent(ctx, agent); err != nil {
		fmt.Fprintf(stderr, "%s failed: %v\n", label, err)
		return 1
	}
	actor := strings.TrimSpace(opts.actor)
	if actor == "" {
		actor = "operator"
	}
	if err := db.SaveLifecycleEvent(ctx, agents.NewLifecycleEvent(agentID, eventType, actor, next, now)); err != nil {
		fmt.Fprintf(stderr, "%s failed: %v\n", label, err)
		return 1
	}
	fmt.Fprintf(stdout, "%s: ok agent=%s status=%s\n", label, agentID, next)
	return 0
}

func runAgentsTerminate(agentID string, opts agentsLifecycleOptions, stdout io.Writer, stderr io.Writer) int {
	if strings.TrimSpace(opts.approvalPath) == "" {
		fmt.Fprintf(stderr, "agents terminate failed: missing --approval\n")
		return 1
	}
	approval, err := agents.ReadTerminationApprovalJSON(opts.approvalPath)
	if err != nil {
		fmt.Fprintf(stderr, "agents terminate failed: %v\n", err)
		return 1
	}
	if err := agents.ValidateTerminationApproval(approval, agentID); err != nil {
		fmt.Fprintf(stderr, "agents terminate failed: %v\n", err)
		return 1
	}
	return runAgentsLifecycleTransition(agentID, opts, agents.TerminateTargetStatus, agents.EventAgentTerminated, "agents terminate", stdout, stderr)
}

func runAgentsSessionCreate(opts agentsSessionCreateOptions, stdout io.Writer, stderr io.Writer) int {
	db, err := openAgentStore(opts.storePath)
	if err != nil {
		fmt.Fprintf(stderr, "agents sessions create failed: %v\n", err)
		return 1
	}
	defer db.Close()
	ctx := context.Background()
	agent, err := db.Agent(ctx, opts.agentID)
	if err != nil {
		fmt.Fprintf(stderr, "agents sessions create failed: %v\n", err)
		return 1
	}
	session, err := agents.CreateSession(agents.CreateSessionOptions{
		Agent:             agent,
		Kind:              opts.kind,
		SessionID:         opts.sessionID,
		WorkspacePath:     opts.workspacePath,
		SkillSnapshotPath: opts.skillSnapshotPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "agents sessions create failed: %v\n", err)
		return 1
	}
	if err := db.SaveSession(ctx, session); err != nil {
		fmt.Fprintf(stderr, "agents sessions create failed: %v\n", err)
		return 1
	}
	if err := db.SaveLifecycleEvent(ctx, agents.NewLifecycleEvent(agent.ID, agents.EventAgentSessionCreated, "deonctl", session.ID, time.Now().UTC())); err != nil {
		fmt.Fprintf(stderr, "agents sessions create failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "agents sessions create: ok session_id=%s\n", session.ID)
	return 0
}

func runAgentsSessionsList(agentID string, opts agentsStoreOptions, stdout io.Writer, stderr io.Writer) int {
	db, err := openAgentStore(opts.storePath)
	if err != nil {
		fmt.Fprintf(stderr, "agents sessions list failed: %v\n", err)
		return 1
	}
	defer db.Close()
	sessions, err := db.ListSessionsByAgent(context.Background(), agentID)
	if err != nil {
		fmt.Fprintf(stderr, "agents sessions list failed: %v\n", err)
		return 1
	}
	writer := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "id\tkind\tstatus\tworker\tcreated_at")
	for _, session := range sessions {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", session.ID, session.Kind, session.Status, session.Worker, session.CreatedAt)
	}
	_ = writer.Flush()
	return 0
}

func runAgentsSessionShow(opts agentsSessionShowOptions, stdout io.Writer, stderr io.Writer) int {
	db, err := openAgentStore(opts.storePath)
	if err != nil {
		fmt.Fprintf(stderr, "agents sessions show failed: %v\n", err)
		return 1
	}
	defer db.Close()
	session, err := db.Session(context.Background(), opts.sessionID)
	if err != nil {
		fmt.Fprintf(stderr, "agents sessions show failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(session); err != nil {
			fmt.Fprintf(stderr, "agents sessions show failed: %v\n", err)
			return 1
		}
	default:
		writer := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(writer, "id:\t%s\n", session.ID)
		fmt.Fprintf(writer, "agent_id:\t%s\n", session.AgentID)
		fmt.Fprintf(writer, "kind:\t%s\n", session.Kind)
		fmt.Fprintf(writer, "status:\t%s\n", session.Status)
		fmt.Fprintf(writer, "worker:\t%s\n", session.Worker)
		fmt.Fprintf(writer, "skill_snapshot_path:\t%s\n", session.SkillSnapshotPath)
		_ = writer.Flush()
	}
	return 0
}

func runAgentsAssign(opts agentsAssignOptions, stdout io.Writer, stderr io.Writer) int {
	task, err := tasks.LoadFromFile(opts.taskPath)
	if err != nil {
		fmt.Fprintf(stderr, "agents assign failed: %v\n", err)
		return 1
	}
	db, err := openAgentStore(opts.storePath)
	if err != nil {
		fmt.Fprintf(stderr, "agents assign failed: %v\n", err)
		return 1
	}
	defer db.Close()
	ctx := context.Background()
	agent, err := db.Agent(ctx, opts.agentID)
	if err != nil {
		fmt.Fprintf(stderr, "agents assign failed: %v\n", err)
		return 1
	}
	now := time.Now().UTC()
	result, _, err := db.AssignWorkItemWithSnapshot(ctx, store.AssignWorkItemWithSnapshotOptions{
		Agent:     agent,
		Task:      *task,
		CreatedBy: opts.createdBy,
		Now:       now,
	})
	if err != nil {
		fmt.Fprintf(stderr, "agents assign failed: %v\n", err)
		return 1
	}
	if err := db.SaveLifecycleEvent(ctx, agents.NewLifecycleEvent(agent.ID, agents.EventAgentAssigned, opts.createdBy, result.WorkItem.ID, now)); err != nil {
		fmt.Fprintf(stderr, "agents assign failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "agents assign: ok work_item=%s inbox=%s\n", result.WorkItem.ID, result.InboxItem.ID)
	return 0
}

func runAgentsInboxList(opts agentsInboxOptions, stdout io.Writer, stderr io.Writer) int {
	db, err := openAgentStore(opts.storePath)
	if err != nil {
		fmt.Fprintf(stderr, "agents inbox list failed: %v\n", err)
		return 1
	}
	defer db.Close()
	items, err := db.ListInboxByAgent(context.Background(), opts.agentID)
	if err != nil {
		fmt.Fprintf(stderr, "agents inbox list failed: %v\n", err)
		return 1
	}
	writer := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "id\twork_item_id\tstatus\tcreated_at")
	for _, item := range items {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", item.ID, item.WorkItemID, item.Status, item.CreatedAt)
	}
	_ = writer.Flush()
	return 0
}

func runAgentsInboxAccept(itemID string, opts agentsInboxOptions, stdout io.Writer, stderr io.Writer) int {
	db, err := openAgentStore(opts.storePath)
	if err != nil {
		fmt.Fprintf(stderr, "agents inbox accept failed: %v\n", err)
		return 1
	}
	defer db.Close()
	ctx := context.Background()
	now := time.Now().UTC()
	accepted, workItem, err := db.AcceptInboxAndQueueWork(ctx, itemID, now)
	if err != nil {
		fmt.Fprintf(stderr, "agents inbox accept failed: %v\n", err)
		return 1
	}
	if err := db.SaveLifecycleEvent(ctx, agents.NewLifecycleEvent(accepted.AgentID, agents.EventInboxAccepted, "operator", itemID, now)); err != nil {
		fmt.Fprintf(stderr, "agents inbox accept failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "agents inbox accept: ok item=%s work_item=%s status=%s\n", accepted.ID, workItem.ID, workItem.Status)
	return 0
}

func runAgentsInboxDefer(itemID string, opts agentsInboxOptions, stdout io.Writer, stderr io.Writer) int {
	return runAgentsInboxTransition(itemID, opts, agents.DeferInboxItem, agents.EventInboxDeferred, "agents inbox defer", stdout, stderr)
}

func runAgentsInboxTransition(itemID string, opts agentsInboxOptions, transition func(agents.InboxItem, time.Time) (agents.InboxItem, error), eventType string, label string, stdout io.Writer, stderr io.Writer) int {
	db, err := openAgentStore(opts.storePath)
	if err != nil {
		fmt.Fprintf(stderr, "%s failed: %v\n", label, err)
		return 1
	}
	defer db.Close()
	ctx := context.Background()
	item, err := db.InboxItem(ctx, itemID)
	if err != nil {
		fmt.Fprintf(stderr, "%s failed: %v\n", label, err)
		return 1
	}
	updated, err := transition(item, time.Now().UTC())
	if err != nil {
		fmt.Fprintf(stderr, "%s failed: %v\n", label, err)
		return 1
	}
	if err := db.UpdateInboxItem(ctx, updated); err != nil {
		fmt.Fprintf(stderr, "%s failed: %v\n", label, err)
		return 1
	}
	if err := db.SaveLifecycleEvent(ctx, agents.NewLifecycleEvent(item.AgentID, eventType, "operator", itemID, time.Now().UTC())); err != nil {
		fmt.Fprintf(stderr, "%s failed: %v\n", label, err)
		return 1
	}
	fmt.Fprintf(stdout, "%s: ok item=%s status=%s\n", label, itemID, updated.Status)
	return 0
}

func runAgentsDelegatePropose(opts agentsDelegateProposeOptions, stdout io.Writer, stderr io.Writer) int {
	task, err := tasks.LoadFromFile(opts.taskPath)
	if err != nil {
		fmt.Fprintf(stderr, "agents delegate propose failed: %v\n", err)
		return 1
	}
	db, err := openAgentStore(opts.storePath)
	if err != nil {
		fmt.Fprintf(stderr, "agents delegate propose failed: %v\n", err)
		return 1
	}
	defer db.Close()
	ctx := context.Background()
	parent, err := db.Agent(ctx, opts.parentAgentID)
	if err != nil {
		fmt.Fprintf(stderr, "agents delegate propose failed: %v\n", err)
		return 1
	}
	child, err := db.Agent(ctx, opts.childAgentID)
	if err != nil {
		fmt.Fprintf(stderr, "agents delegate propose failed: %v\n", err)
		return 1
	}
	parentWork, err := db.WorkItem(ctx, opts.parentWorkItemID)
	if err != nil {
		fmt.Fprintf(stderr, "agents delegate propose failed: %v\n", err)
		return 1
	}
	proposal, err := agents.BuildDelegationProposal(agents.BuildDelegationProposalOptions{
		ParentAgent:    parent,
		ChildAgent:     child,
		ParentWorkItem: parentWork,
		Task:           *task,
		Reason:         opts.reason,
	})
	if err != nil {
		fmt.Fprintf(stderr, "agents delegate propose failed: %v\n", err)
		return 1
	}
	if err := agents.WriteDelegationProposalJSON(proposal, opts.outputPath); err != nil {
		fmt.Fprintf(stderr, "agents delegate propose failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "agents delegate propose: ok proposal_id=%s\n", proposal.ProposalID)
	return 0
}
