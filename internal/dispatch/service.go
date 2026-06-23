package dispatch

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/agents"
	"github.com/deon7769/deonclaw/internal/budget"
	"github.com/deon7769/deonclaw/internal/insights"
	"github.com/deon7769/deonclaw/internal/runs"
	"github.com/deon7769/deonclaw/internal/usage"
	"github.com/deon7769/deonclaw/internal/workqueue"
)

type Service struct {
	Repo Repository
}

func (s Service) DispatchOnce(ctx context.Context, opts OnceOptions) (OnceResult, error) {
	steps := make([]string, 0, 24)
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if strings.TrimSpace(opts.WorkItemID) == "" {
		return OnceResult{}, fmt.Errorf("work item id is required")
	}
	mode := strings.TrimSpace(opts.Mode)
	if mode == "" {
		mode = ModeFake
	}
	opts.Mode = mode
	steps = append(steps, StepValidateOptions)

	workItem, err := s.Repo.WorkItem(ctx, opts.WorkItemID)
	if err != nil {
		return NewErrorResult(opts, steps, err), err
	}
	steps = append(steps, StepLoadWorkItem)
	if err := ValidateDispatchableWorkItem(workItem, opts.LeaseID); err != nil {
		return NewErrorResult(opts, steps, err), err
	}
	steps = append(steps, StepValidateWorkItem)

	agent, err := s.Repo.Agent(ctx, workItem.AssignedAgentID)
	if err != nil {
		return NewErrorResult(opts, steps, err), err
	}
	steps = append(steps, StepLoadAgent)
	if err := agents.CanStartRun(agent); err != nil {
		return NewErrorResult(opts, steps, err), err
	}
	steps = append(steps, StepValidateAgent)

	snapshot, err := s.Repo.WorkItemTaskSnapshot(ctx, workItem.ID)
	if err != nil {
		return NewErrorResult(opts, steps, err), err
	}
	steps = append(steps, StepLoadTaskSnapshot)
	task, err := TaskFromSnapshot(snapshot)
	if err != nil {
		return NewErrorResult(opts, steps, err), err
	}
	steps = append(steps, StepValidateTaskSnapshot)

	policyID := strings.TrimSpace(workItem.BudgetPolicy)
	if policyID == "" {
		policyID = strings.TrimSpace(agent.BudgetPolicy)
	}
	var policy budget.Policy
	if policyID != "" {
		policy, err = s.Repo.BudgetPolicy(ctx, policyID)
		if err != nil {
			return NewErrorResult(opts, steps, err), err
		}
	}
	steps = append(steps, StepResolveBudgetPolicy)

	var window budget.Window
	if policy.ID != "" {
		window, err = s.Repo.GetOrCreateWindow(ctx, policy, now)
		if err != nil {
			return NewErrorResult(opts, steps, err), err
		}
		steps = append(steps, StepGetOrCreateWindow)
		estimate := policy.DefaultEstimateMicroUSD
		if err := budget.CanReserve(window, policy, estimate); err != nil {
			steps = append(steps, StepBudgetPreflight)
			contract := BudgetBlockedContract{
				Status:            "budget_blocked",
				BlockedReason:     err.Error(),
				WorkItemID:        workItem.ID,
				AgentID:           agent.ID,
				PolicyID:          policy.ID,
				EstimatedMicroUSD: estimate,
				ProviderCall:      false,
				NetworkCall:       false,
			}
			workItem.Status = agents.WorkItemStatusBlocked
			workItem.UpdatedAt = now.Format(time.RFC3339Nano)
			_ = s.Repo.SaveWorkItem(ctx, workItem)
			_ = s.Repo.AppendWorkQueueEvent(ctx, workqueue.QueueEvent{
				ID: "wqe_budget_" + workItem.ID, WorkItemID: workItem.ID, EventType: workqueue.EventBudgetBlocked,
				Payload: fmt.Sprintf(`{"policy_id":%q,"reason":%q}`, policy.ID, err.Error()), CreatedAt: workItem.UpdatedAt,
			})
			if policy.OnExhausted == budget.OnExhaustedPauseAgent {
				_, _, _ = s.Repo.ApplyBudgetExhausted(ctx, agent, policy, now)
			}
			result := NewBudgetBlockedResult(opts, steps, contract)
			return result, nil
		}
		steps = append(steps, StepBudgetPreflight)
	}

	runID := fmt.Sprintf("run_dispatch_%s_%d", workItem.ID, now.UnixNano())
	var reservation budget.Reservation
	if policy.ID != "" && policy.ReserveBeforeRun {
		reserveResult, err := s.Repo.ReserveBudget(ctx, budget.ReserveBudgetOptions{
			PolicyID:          policy.ID,
			WorkItemID:        workItem.ID,
			RunID:             runID,
			AgentID:           agent.ID,
			EstimatedMicroUSD: policy.DefaultEstimateMicroUSD,
			Now:               now,
		})
		if err != nil {
			steps = append(steps, StepReserveBudget)
			contract := BudgetBlockedContract{
				Status: "budget_blocked", BlockedReason: err.Error(), WorkItemID: workItem.ID,
				AgentID: agent.ID, PolicyID: policy.ID, EstimatedMicroUSD: policy.DefaultEstimateMicroUSD,
				ProviderCall: false, NetworkCall: false,
			}
			workItem.Status = agents.WorkItemStatusBlocked
			workItem.UpdatedAt = now.Format(time.RFC3339Nano)
			_ = s.Repo.SaveWorkItem(ctx, workItem)
			result := NewBudgetBlockedResult(opts, steps, contract)
			return result, nil
		}
		reservation = reserveResult.Reservation
	}
	steps = append(steps, StepReserveBudget)

	var lease workqueue.Lease
	if strings.TrimSpace(opts.LeaseID) != "" {
		lease, err = s.Repo.Lease(ctx, opts.LeaseID)
		if err != nil {
			if reservation.ID != "" {
				_, _ = s.Repo.ReleaseReservation(ctx, reservation.ID, "lease missing", now)
			}
			return NewErrorResult(opts, steps, err), err
		}
		if lease.WorkItemID != workItem.ID || lease.Status != workqueue.LeaseStatusActive {
			err = fmt.Errorf("lease %q is not active for work item %q", lease.ID, workItem.ID)
			if reservation.ID != "" {
				_, _ = s.Repo.ReleaseReservation(ctx, reservation.ID, err.Error(), now)
			}
			return NewErrorResult(opts, steps, err), err
		}
	}
	steps = append(steps, StepVerifyLease)

	if err := s.Repo.SaveTask(ctx, &task); err != nil {
		if reservation.ID != "" {
			_, _ = s.Repo.ReleaseReservation(ctx, reservation.ID, "task persist failed", now)
		}
		return NewErrorResult(opts, steps, err), err
	}

	run := &runs.Run{
		ID: runID, TaskID: task.ID, Status: runs.StatusRunning, Worker: agent.DefaultWorker,
		AgentID: agent.ID, WorkItemID: workItem.ID, CreatedAt: now, UpdatedAt: now,
		DispatchMeta: runs.DispatchMeta{
			Mode: mode, BudgetReservationID: reservation.ID, ProviderCall: false, NetworkCall: false,
		},
	}
	if err := s.Repo.SaveRun(ctx, run); err != nil {
		if reservation.ID != "" {
			_, _ = s.Repo.ReleaseReservation(ctx, reservation.ID, "run create failed", now)
		}
		return NewErrorResult(opts, steps, err), err
	}
	steps = append(steps, StepCreateRun)

	if lease.ID != "" {
		lease.RunID = runID
		lease.UpdatedAt = now.Format(time.RFC3339Nano)
		if err := s.Repo.SaveLease(ctx, lease); err != nil {
			return NewErrorResult(opts, steps, err), err
		}
	}
	steps = append(steps, StepBindLease)

	workItem.Status = agents.WorkItemStatusRunning
	workItem.UpdatedAt = now.Format(time.RFC3339Nano)
	if err := s.Repo.SaveWorkItem(ctx, workItem); err != nil {
		return NewErrorResult(opts, steps, err), err
	}
	_ = s.Repo.AppendWorkQueueEvent(ctx, workqueue.QueueEvent{
		ID: "wqe_running_" + workItem.ID, WorkItemID: workItem.ID, EventType: workqueue.EventRunning,
		Payload: fmt.Sprintf(`{"run_id":%q}`, runID), CreatedAt: workItem.UpdatedAt,
	})
	steps = append(steps, StepMarkWorkRunning)

	workerName, runner, err := SelectWorkerRunner(agent, opts.CodexRunner, opts.OpenCodeRunner)
	if err != nil {
		return s.failDispatch(ctx, opts, steps, run, workItem, reservation, now, err)
	}
	if mode == ModeReal && !opts.ConfirmWorkerDispatch {
		err = fmt.Errorf("real dispatch requires --confirm-worker-dispatch")
		return s.failDispatch(ctx, opts, steps, run, workItem, reservation, now, err)
	}
	if mode == ModeFake && runner == nil {
		runner = NewFakeWorkerRunner()
	}
	workerResult, err := runner.Run(ctx, WorkerRunOptions{
		TaskJSON: snapshot.TaskJSON, TaskID: task.ID, Worker: workerName,
		ModelProfile: agent.ModelProfile, RunID: runID, Mode: mode, ConfirmReal: opts.ConfirmWorkerDispatch,
	})
	if err != nil {
		return s.failDispatch(ctx, opts, steps, run, workItem, reservation, now, err)
	}
	steps = append(steps, StepExecuteWorker)

	normalized, err := usage.NormalizeWorkerMetadata(usage.NormalizeInput{
		Metadata: workerResult.UsageMeta, Worker: workerName, ModelProfile: agent.ModelProfile,
	})
	if err != nil {
		return s.failDispatch(ctx, opts, steps, run, workItem, reservation, now, err)
	}
	usageEvent := normalized.Event
	usageEvent.ID = usage.NewEventID(runID, now)
	usageEvent.RunID = runID
	usageEvent.WorkItemID = workItem.ID
	usageEvent.AgentID = agent.ID
	usageEvent.DurationMS = workerResult.DurationMS
	usageEvent.CreatedAt = now.Format(time.RFC3339Nano)
	steps = append(steps, StepNormalizeUsage)

	actualMicroUSD := int64(0)
	if opts.PricingLoader != nil {
		price, priceErr := opts.PricingLoader(ctx, s.Repo, agent.ModelProfile, now)
		if priceErr == nil {
			actualMicroUSD, err = usage.CalculateCostMicroUSD(price, usageEvent.InputTokens, usageEvent.OutputTokens, usageEvent.CachedInputTokens)
			if err != nil {
				return s.failDispatch(ctx, opts, steps, run, workItem, reservation, now, err)
			}
			usageEvent.ActualCostMicroUSD = actualMicroUSD
			usageEvent.EstimatedCostMicroUSD = actualMicroUSD
		}
	}
	if actualMicroUSD == 0 && policy.DefaultEstimateMicroUSD > 0 {
		usageEvent.EstimatedCostMicroUSD = policy.DefaultEstimateMicroUSD
		actualMicroUSD = policy.DefaultEstimateMicroUSD
	}
	steps = append(steps, StepCalculateCost)

	if reservation.ID != "" {
		if _, err := s.Repo.CommitReservation(ctx, reservation.ID, usageEvent, actualMicroUSD, now); err != nil {
			return s.failDispatch(ctx, opts, steps, run, workItem, reservation, now, err)
		}
	}
	steps = append(steps, StepCommitBudget)

	if err := usage.ValidateEvent(usageEvent); err == nil {
		_ = s.Repo.SaveUsageEvent(ctx, usageEvent)
	}
	steps = append(steps, StepPersistUsage)

	finished := now
	run.Status = workerResult.Status
	if run.Status == "" {
		if workerResult.ErrorMessage != "" {
			run.Status = runs.StatusFailed
		} else {
			run.Status = runs.StatusSucceeded
		}
	}
	run.UpdatedAt = finished
	run.FinishedAt = &finished
	run.DispatchMeta.StepsCompleted = append([]string(nil), steps...)
	if err := s.Repo.SaveRun(ctx, run); err != nil {
		return NewErrorResult(opts, steps, err), err
	}

	switch run.Status {
	case runs.StatusSucceeded:
		workItem.Status = agents.WorkItemStatusSucceeded
	case runs.StatusFailed, runs.StatusPolicyFailed:
		workItem.Status = agents.WorkItemStatusFailed
	default:
		workItem.Status = agents.WorkItemStatusFailed
	}
	workItem.UpdatedAt = finished.Format(time.RFC3339Nano)
	_ = s.Repo.SaveWorkItem(ctx, workItem)
	if lease.ID != "" {
		lease.Status = workqueue.LeaseStatusReleased
		lease.ReleaseReason = "dispatch complete"
		lease.UpdatedAt = finished.Format(time.RFC3339Nano)
		_ = s.Repo.SaveLease(ctx, lease)
	}
	steps = append(steps, StepFinalizeRun)

	var evidence *insights.EvidenceBundle
	if opts.EvidenceBuilder != nil && run.Status == runs.StatusSucceeded {
		bundle, err := opts.EvidenceBuilder(ctx, s.Repo, runID, now)
		if err == nil {
			evidence = &bundle
			run.DispatchMeta.EvidenceBundleID = bundle.EvidenceBundleID
			_ = s.Repo.SaveRun(ctx, run)
		}
	}
	steps = append(steps, StepBuildEvidence)

	var review *InsightReviewResult
	if run.Status == runs.StatusSucceeded {
		reviewResult, err := EnqueueInsightReview(ctx, s.Repo, workItem, runID, now)
		if err == nil && reviewResult.Queued {
			review = &reviewResult
			run.DispatchMeta.InsightReviewWorkItemID = reviewResult.WorkItemID
			run.DispatchMeta.StepsCompleted = append(run.DispatchMeta.StepsCompleted, StepEnqueueInsightReview)
			_ = s.Repo.SaveRun(ctx, run)
		}
	}
	steps = append(steps, StepEnqueueInsightReview, StepComplete)

	result := NewSuccessResult(opts, steps, runID, run.Status, SuccessMeta{
		LeaseID: lease.ID, ReservationID: reservation.ID, Worker: workerName,
		BudgetReserved: reservation.ID != "", UsageRecorded: true,
		BudgetCommitted: reservation.ID != "", WorkStatus: workItem.Status,
		LeaseStatus: workqueue.LeaseStatusReleased, EvidenceBundleCreated: evidence != nil,
		ReviewWorkQueued: review != nil && review.Queued,
	})
	result.EvidenceBundle = evidence
	result.InsightReview = review
	return result, nil
}

func (s Service) failDispatch(ctx context.Context, opts OnceOptions, steps []string, run *runs.Run, workItem agents.WorkItem, reservation budget.Reservation, now time.Time, err error) (OnceResult, error) {
	if reservation.ID != "" {
		_, _ = s.Repo.ReleaseReservation(ctx, reservation.ID, err.Error(), now)
	}
	finished := now
	run.Status = runs.StatusFailed
	run.UpdatedAt = finished
	run.FinishedAt = &finished
	run.DispatchMeta.BlockedReason = err.Error()
	_ = s.Repo.SaveRun(ctx, run)
	workItem.Status = agents.WorkItemStatusFailed
	workItem.UpdatedAt = finished.Format(time.RFC3339Nano)
	_ = s.Repo.SaveWorkItem(ctx, workItem)
	return NewErrorResult(opts, steps, err), err
}

func DefaultPricingLoader(ctx context.Context, repo Repository, modelProfile string, at time.Time) (usage.ModelPrice, error) {
	_ = at
	return repo.ModelPrice(ctx, modelProfile)
}
