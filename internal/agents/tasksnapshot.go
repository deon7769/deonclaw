package agents

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/deon7769/deonclaw/internal/tasks"
)

const (
	WorkItemKindTask          = "task"
	WorkItemKindInsightReview = "insight_review"

	DefaultWorkItemMaxAttempts = 2
	DefaultWorkItemPriority    = 50
)

type WorkItemTaskSnapshot struct {
	WorkItemID string `json:"work_item_id"`
	TaskID     string `json:"task_id"`
	TaskJSON   string `json:"task_json"`
	TaskSHA256 string `json:"task_sha256"`
	CreatedAt  string `json:"created_at"`
}

func CanonicalTaskJSON(task tasks.Task) (string, error) {
	data, err := json.Marshal(task)
	if err != nil {
		return "", fmt.Errorf("marshal task json: %w", err)
	}
	return string(data), nil
}

func TaskSHA256(taskJSON string) string {
	sum := sha256.Sum256([]byte(taskJSON))
	return hex.EncodeToString(sum[:])
}

func BuildWorkItemTaskSnapshot(workItemID string, task tasks.Task, createdAt string) (WorkItemTaskSnapshot, error) {
	taskJSON, err := CanonicalTaskJSON(task)
	if err != nil {
		return WorkItemTaskSnapshot{}, err
	}
	return WorkItemTaskSnapshot{
		WorkItemID: workItemID,
		TaskID:     task.ID,
		TaskJSON:   taskJSON,
		TaskSHA256: TaskSHA256(taskJSON),
		CreatedAt:  createdAt,
	}, nil
}

func ValidateWorkItemTaskSnapshot(snapshot WorkItemTaskSnapshot) error {
	if strings.TrimSpace(snapshot.WorkItemID) == "" {
		return fmt.Errorf("task snapshot work_item_id is required")
	}
	if strings.TrimSpace(snapshot.TaskJSON) == "" {
		return fmt.Errorf("task snapshot task_json is required")
	}
	if snapshot.TaskSHA256 != TaskSHA256(snapshot.TaskJSON) {
		return fmt.Errorf("task snapshot task_sha256 mismatch")
	}
	return nil
}

func ApplyWorkItemDefaults(item *WorkItem, agent Agent) {
	if item.Priority == 0 {
		item.Priority = DefaultWorkItemPriority
	}
	if item.MaxAttempts == 0 {
		item.MaxAttempts = DefaultWorkItemMaxAttempts
	}
	if strings.TrimSpace(item.Kind) == "" {
		item.Kind = WorkItemKindTask
	}
	if strings.TrimSpace(item.BudgetPolicy) == "" && strings.TrimSpace(agent.BudgetPolicy) != "" {
		item.BudgetPolicy = agent.BudgetPolicy
	}
}

func DependenciesSatisfied(item WorkItem, itemsByID map[string]WorkItem) error {
	if len(item.Dependencies) == 0 {
		return nil
	}
	for _, depID := range item.Dependencies {
		dep, ok := itemsByID[depID]
		if !ok {
			return fmt.Errorf("dependency %q not found", depID)
		}
		switch dep.Status {
		case WorkItemStatusSucceeded, WorkItemStatusCompleted:
			continue
		default:
			return fmt.Errorf("dependency %q status %q is not complete", depID, dep.Status)
		}
	}
	return nil
}

func SortedDependencyIDs(ids []string) []string {
	out := append([]string(nil), ids...)
	sort.Strings(out)
	return out
}
