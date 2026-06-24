package workqueue

import (
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/agents"
)

type DoctorInput struct {
	Items           []agents.WorkItem
	Leases          []Lease
	Inbox           []agents.InboxItem
	SnapshotWorkIDs map[string]struct{}
	Now             time.Time
}

func Diagnose(input DoctorInput) DoctorReport {
	queued := 0
	leased := 0
	activeLeases := 0
	findings := make([]DoctorFinding, 0)

	activeByWork := map[string]int{}
	activeLeaseByWork := map[string]string{}
	for _, lease := range input.Leases {
		if lease.Status != LeaseStatusActive {
			continue
		}
		activeLeases++
		activeByWork[lease.WorkItemID]++
		activeLeaseByWork[lease.WorkItemID] = lease.ID
		now := input.Now
		if now.IsZero() {
			now = time.Now().UTC()
		}
		expiresAt, err := time.Parse(time.RFC3339Nano, lease.ExpiresAt)
		if err == nil && now.After(expiresAt) {
			findings = append(findings, DoctorFinding{
				Severity:   "error",
				Code:       "active_lease_expired",
				Message:    "active lease exceeded ttl",
				WorkItemID: lease.WorkItemID,
				LeaseID:    lease.ID,
			})
		}
		if activeByWork[lease.WorkItemID] > 1 {
			findings = append(findings, DoctorFinding{
				Severity:   "error",
				Code:       "multiple_active_leases",
				Message:    "multiple active leases for work item",
				WorkItemID: lease.WorkItemID,
				LeaseID:    lease.ID,
			})
		}
	}

	inboxByWork := map[string]agents.InboxItem{}
	for _, inbox := range input.Inbox {
		inboxByWork[inbox.WorkItemID] = inbox
	}

	for _, item := range input.Items {
		switch item.Status {
		case agents.WorkItemStatusQueued, agents.WorkItemStatusAssigned:
			queued++
		case agents.WorkItemStatusLeased, agents.WorkItemStatusRunning:
			leased++
		}
		if item.Status == agents.WorkItemStatusQueued {
			if item.MaxAttempts > 0 && item.Attempt >= item.MaxAttempts {
				findings = append(findings, DoctorFinding{
					Severity:   "error",
					Code:       "queued_attempts_exhausted",
					Message:    "queued work item has exhausted attempts",
					WorkItemID: item.ID,
				})
			}
			if err := agents.DependenciesSatisfied(item, ItemsByID(input.Items)); err != nil {
				findings = append(findings, DoctorFinding{
					Severity:   "warning",
					Code:       "queued_dependency_incomplete",
					Message:    err.Error(),
					WorkItemID: item.ID,
				})
			}
			if _, ok := input.SnapshotWorkIDs[item.ID]; !ok && item.Kind == agents.WorkItemKindTask {
				findings = append(findings, DoctorFinding{
					Severity:   "error",
					Code:       "queued_missing_task_snapshot",
					Message:    "queued work item has no task snapshot",
					WorkItemID: item.ID,
				})
			}
			inbox, ok := inboxByWork[item.ID]
			if ok && inbox.Status == agents.InboxStatusAccepted && item.Status == agents.WorkItemStatusAssigned {
				findings = append(findings, DoctorFinding{
					Severity:   "error",
					Code:       "accepted_inbox_not_queued",
					Message:    "inbox accepted but work item is still assigned",
					WorkItemID: item.ID,
					InboxID:    inbox.ID,
				})
			}
		}
		if item.Status == agents.WorkItemStatusLeased || item.Status == agents.WorkItemStatusRunning {
			if activeByWork[item.ID] == 0 {
				findings = append(findings, DoctorFinding{
					Severity:   "error",
					Code:       "leased_without_active_lease",
					Message:    "work item is leased/running without active lease",
					WorkItemID: item.ID,
				})
			}
		}
		if (item.Status == agents.WorkItemStatusQueued || item.Status == agents.WorkItemStatusLeased) && strings.TrimSpace(item.AssignedAgentID) != "" {
			// agent lifecycle checks require agent list; store doctor will add those findings
		}
	}

	for workID, leaseID := range activeLeaseByWork {
		var item agents.WorkItem
		for _, candidate := range input.Items {
			if candidate.ID == workID {
				item = candidate
				break
			}
		}
		if item.ID == "" {
			continue
		}
		if item.Status != agents.WorkItemStatusLeased && item.Status != agents.WorkItemStatusRunning {
			findings = append(findings, DoctorFinding{
				Severity:   "error",
				Code:       "active_lease_work_not_leased",
				Message:    "active lease exists for work item not leased/running",
				WorkItemID: workID,
				LeaseID:    leaseID,
			})
		}
	}

	status := "ok"
	for _, finding := range findings {
		if finding.Severity == "error" {
			status = "failed"
			break
		}
		if finding.Severity == "warning" && status == "ok" {
			status = "warning"
		}
	}
	return DoctorReport{
		Status:       status,
		QueuedWork:   queued,
		LeasedWork:   leased,
		ActiveLeases: activeLeases,
		Findings:     findings,
	}
}
