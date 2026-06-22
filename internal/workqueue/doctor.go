package workqueue

import (
	"github.com/deon7769/deonclaw/internal/agents"
)

func Doctor(items []agents.WorkItem, leases []Lease, nowRecovery RecoveryReport) DoctorReport {
	queued := 0
	leased := 0
	activeLeases := 0
	for _, item := range items {
		switch item.Status {
		case agents.WorkItemStatusQueued, agents.WorkItemStatusAssigned:
			queued++
		case agents.WorkItemStatusLeased:
			leased++
		}
	}
	for _, lease := range leases {
		if lease.Status == LeaseStatusActive {
			activeLeases++
		}
	}
	status := "ok"
	if nowRecovery.Status != "ok" {
		status = nowRecovery.Status
	}
	return DoctorReport{
		Status:        status,
		QueuedWork:    queued,
		LeasedWork:    leased,
		ActiveLeases:  activeLeases,
		LeaseRecovery: nowRecovery,
	}
}
