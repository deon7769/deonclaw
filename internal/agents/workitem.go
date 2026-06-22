package agents

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/tasks"
)

type WorkItem struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	Status           string   `json:"status"`
	AssignedAgentID  string   `json:"assigned_agent_id"`
	ParentWorkItemID string   `json:"parent_work_item_id,omitempty"`
	TaskID           string   `json:"task_id,omitempty"`
	GoalID           string   `json:"goal_id,omitempty"`
	Priority         int      `json:"priority"`
	AllowedPaths     []string `json:"allowed_paths"`
	ForbiddenPaths   []string `json:"forbidden_paths"`
	DefinitionOfDone []string `json:"definition_of_done"`
	Dependencies     []string `json:"dependencies,omitempty"`
	CreatedBy        string   `json:"created_by"`
	CreatedAt        string   `json:"created_at"`
	UpdatedAt        string   `json:"updated_at"`
}

type WorkItemFromTaskOptions struct {
	Task             tasks.Task
	AssignedAgentID  string
	CreatedBy        string
	ParentWorkItemID string
	GoalID           string
	Priority         int
	Now              time.Time
}

func WorkItemFromTask(opts WorkItemFromTaskOptions) (WorkItem, error) {
	agentID := strings.TrimSpace(opts.AssignedAgentID)
	if agentID == "" {
		return WorkItem{}, fmt.Errorf("assigned agent id is required")
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	priority := opts.Priority
	if priority == 0 {
		priority = 50
	}
	createdBy := strings.TrimSpace(opts.CreatedBy)
	if createdBy == "" {
		createdBy = "operator"
	}
	workID := newWorkItemID(opts.Task.ID, agentID, now)
	return WorkItem{
		ID:               workID,
		Title:            opts.Task.Title,
		Status:           WorkItemStatusAssigned,
		AssignedAgentID:  agentID,
		ParentWorkItemID: strings.TrimSpace(opts.ParentWorkItemID),
		TaskID:           opts.Task.ID,
		GoalID:           strings.TrimSpace(opts.GoalID),
		Priority:         priority,
		AllowedPaths:     append([]string(nil), opts.Task.AllowedPaths...),
		ForbiddenPaths:   append([]string(nil), opts.Task.ForbiddenPaths...),
		DefinitionOfDone: append([]string(nil), opts.Task.DefinitionOfDone...),
		CreatedBy:        createdBy,
		CreatedAt:        now.Format(time.RFC3339Nano),
		UpdatedAt:        now.Format(time.RFC3339Nano),
	}, nil
}

func newWorkItemID(taskID string, agentID string, now time.Time) string {
	sum := sha256.Sum256([]byte(taskID + "|" + agentID + "|" + now.Format(time.RFC3339Nano)))
	return "work_" + hex.EncodeToString(sum[:8])
}

func PathsSubset(child []string, parent []string) bool {
	if len(child) == 0 {
		return true
	}
	for _, childPath := range child {
		if pathAllowedByParent(childPath, parent) {
			continue
		}
		return false
	}
	return true
}

func pathAllowedByParent(childPath string, parent []string) bool {
	childPath = strings.TrimSpace(childPath)
	for _, parentPath := range parent {
		parentPath = strings.TrimSpace(parentPath)
		if parentPath == "" {
			continue
		}
		if parentPath == childPath {
			return true
		}
		if strings.HasSuffix(parentPath, "/**") {
			prefix := strings.TrimSuffix(parentPath, "/**")
			if strings.HasPrefix(childPath, prefix) {
				return true
			}
		}
		if strings.HasSuffix(parentPath, "/*") {
			prefix := strings.TrimSuffix(parentPath, "/*")
			if strings.HasPrefix(childPath, prefix+"/") || childPath == prefix {
				return true
			}
		}
	}
	return false
}
