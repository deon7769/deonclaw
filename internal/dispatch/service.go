package dispatch

import (
	"context"
	"fmt"
	"path/filepath"
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
	steps := make([]string, 0, 28)
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	base := OnceResult{
		WorkItemID:    opts.WorkItemID,
		Mode:          opts.Mode,
		LeaseRequired: true,
		ProviderCall:  false,
		NetworkCall:   false,
	}
	if strings.TrimSpace(opts.WorkItemID) == "" {
		return base, fmt.Errorf("work item id is required")
	}
	mode := strings.TrimSpace(opts.Mode)
	if mode == "" {
		mode = ModeFake
	}
	opts.Mode = mode
	base.Mode = mode
	steps = append(steps, StepValidateOptions)

	if mode == ModeReal {
		steps = append(steps, StepExecuteWorker)
		result := NewBlockedResult(opts, steps, BlockedRealModeUnsupported, "real dispatch adapter not wired in this sprint")
		return result, fmt.Errorf("%s", BlockedRealModeUnsupported)
	}

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

	lease, leaseAcquired, err := AcquireLease(ctx, s.Repo, agent, workItem, opts.LeaseID, opts.LeaseTTL, now)
	if err != nil {
		return NewErrorResult(opts, steps, err), err
	}
	steps = append(steps, StepVerifyLease)
	if !leaseAcquired {
		result := NewNotStartedResult(opts, steps, lease.ID)
		result.LeaseAcquired = false
		return result, nil
	}
	base.LeaseID = lease.ID
	base.LeaseAcquired = true

	policyID := strings.TrimSpace(workItem.BudgetPolicy)
	if policyID == "" {
		policyID = strings.TrimSpace(agent.BudgetPolicy)
	}
	if policyID == "" {
		_ = ReleaseActiveLease(ctx, s.Repo, lease, "dispatch_failed:"+BlockedBudgetPolicyRequired, true, now)
		steps = append(steps, StepResolveBudgetPolicy)
		result := NewBlockedResult(opts, steps, BlockedBudgetPolicyRequired, "budget policy is required for dispatch")
		result.LeaseID = lease.ID
		result.LeaseAcquired = true
		result.LeaseReleased = true
		return result, nil
	}
	policy, err := s.Repo.BudgetPolicy(ctx, policyID)
	if err != nil {
		_ = ReleaseActiveLease(ctx, s.Repo, lease, "dispatch_failed:budget_policy_load", true, now)
		return NewErrorResult(opts, steps, err), err
	}
	steps = append(steps, StepResolveBudgetPolicy)

	window, err := s.Repo.GetOrCreateWindow(ctx, policy, now)
	if err != nil {
		_ = ReleaseActiveLease(ctx, s.Repo, lease, "dispatch_failed:window", true, now)
		return NewErrorResult(opts, steps, err), err
	}
	steps = append(steps, StepGetOrCreateWindow)
	estimate := policy.DefaultEstimateMicroUSD
	if err := budget.CanReserve(window, policy, estimate); err != nil {
		steps = append(steps, StepBudgetPreflight)
		_ = ReleaseActiveLease(ctx, s.Repo, lease, "dispatch_failed:budget_blocked", true, now)
		contract := BudgetBlockedContract{
			Status: "budget_blocked", BlockedReason: err.Error(), WorkItemID: workItem.ID,
			AgentID: agent.ID, PolicyID: policy.ID, EstimatedMicroUSD: estimate,
			ProviderCall: false, NetworkCall: false,
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
		result.LeaseID = lease.ID
		result.LeaseAcquired = true
		result.LeaseReleased = true
		return result, nil
	}
	steps = append(steps, StepBudgetPreflight)

	runID := fmt.Sprintf("run_dispatch_%s_%d", workItem.ID, now.UnixNano())
	reserveResult, err := s.Repo.ReserveBudget(ctx, budget.ReserveBudgetOptions{
		PolicyID: policy.ID, WorkItemID: workItem.ID, RunID: runID, AgentID: agent.ID,
		EstimatedMicroUSD: estimate, Now: now,
	})
	if err != nil {
		steps = append(steps, StepReserveBudget)
		_ = ReleaseActiveLease(ctx, s.Repo, lease, "dispatch_failed:budget_reserve", true, now)
		contract := BudgetBlockedContract{
			Status: "budget_blocked", BlockedReason: err.Error(), WorkItemID: workItem.ID,
			AgentID: agent.ID, PolicyID: policy.ID, EstimatedMicroUSD: estimate,
			ProviderCall: false, NetworkCall: false,
		}
		workItem.Status = agents.WorkItemStatusBlocked
		workItem.UpdatedAt = now.Format(time.RFC3339Nano)
		_ = s.Repo.SaveWorkItem(ctx, workItem)
		result := NewBudgetBlockedResult(opts, steps, contract)
		result.LeaseID = lease.ID
		result.LeaseAcquired = true
		result.LeaseReleased = true
		return result, nil
	}
	reservation := reserveResult.Reservation
	steps = append(steps, StepReserveBudget)

	skillApplied := false
	if strings.TrimSpace(opts.RegistryRoot) != "" {
		skillResult, skillErr := PrepareSkills(ctx, s.Repo, SkillPrepareOptions{
			Agent: agent, Workspace: task.Workspace.Path, RegistryRoot: opts.RegistryRoot,
			SkillPolicy: opts.SkillPolicy, Now: now,
		})
		if skillErr != nil {
			return s.failDispatch(ctx, opts, steps, nil, workItem, lease, reservation, false, now, skillErr)
		}
		skillApplied = skillResult.Materialized
		_ = skillResult
	}

	if err := s.Repo.SaveTask(ctx, &task); err != nil {
		return s.failDispatch(ctx, opts, steps, nil, workItem, lease, reservation, false, now, err)
	}

	run := &runs.Run{
		ID: runID, TaskID: task.ID, Status: runs.StatusRunning, Worker: agent.DefaultWorker,
		AgentID: agent.ID, WorkItemID: workItem.ID, CreatedAt: now, UpdatedAt: now,
		DispatchMeta: runs.DispatchMeta{
			Mode: mode, BudgetReservationID: reservation.ID, ProviderCall: false, NetworkCall: false,
		},
	}
	if err := s.Repo.SaveRun(ctx, run); err != nil {
		return s.failDispatch(ctx, opts, steps, run, workItem, lease, reservation, false, now, err)
	}
	steps = append(steps, StepCreateRun)

	lease.RunID = runID
	lease.UpdatedAt = now.Format(time.RFC3339Nano)
	if err := s.Repo.SaveLease(ctx, lease); err != nil {
		return s.failDispatch(ctx, opts, steps, run, workItem, lease, reservation, false, now, err)
	}
	steps = append(steps, StepBindLease)

	workItem.Status = agents.WorkItemStatusRunning
	workItem.UpdatedAt = now.Format(time.RFC3339Nano)
	if err := s.Repo.SaveWorkItem(ctx, workItem); err != nil {
		return s.failDispatch(ctx, opts, steps, run, workItem, lease, reservation, false, now, err)
	}
	_ = s.Repo.AppendWorkQueueEvent(ctx, workqueue.QueueEvent{
		ID: "wqe_running_" + workItem.ID, WorkItemID: workItem.ID, EventType: workqueue.EventRunning,
		Payload: fmt.Sprintf(`{"run_id":%q}`, runID), CreatedAt: workItem.UpdatedAt,
	})
	steps = append(steps, StepMarkWorkRunning)

	workerName, runner, err := SelectWorkerRunner(agent, opts.CodexRunner, opts.OpenCodeRunner)
	if err != nil {
		return s.failDispatch(ctx, opts, steps, run, workItem, lease, reservation, false, now, err)
	}
	if mode == ModeFake && runner == nil {
		runner = NewFakeWorkerRunner()
	}
	workerResult, err := runner.Run(ctx, WorkerRunOptions{
		TaskJSON: snapshot.TaskJSON, TaskID: task.ID, Worker: workerName,
		ModelProfile: agent.ModelProfile, RunID: runID, Mode: mode, ConfirmReal: opts.ConfirmWorkerDispatch,
	})
	if err != nil {
		return s.failDispatch(ctx, opts, steps, run, workItem, lease, reservation, true, now, err)
	}
	if workerResult.Status == runs.StatusFailed || workerResult.Status == runs.StatusPolicyFailed {
		msg := workerResult.ErrorMessage
		if strings.TrimSpace(msg) == "" {
			msg = "worker returned failed status"
		}
		return s.failDispatch(ctx, opts, steps, run, workItem, lease, reservation, true, now, fmt.Errorf("%s", msg))
	}
	steps = append(steps, StepExecuteWorker)

	var learningLoop *LearningLoopResult
	if workItem.Kind == agents.WorkItemKindInsightReview {
		materialized, err := MaterializeInsightReviewLearning(InsightReviewLearningOptions{
			WorkItem:             workItem,
			Agent:                agent,
			WorkspacePath:        task.Workspace.Path,
			RegistryRoot:         opts.RegistryRoot,
			SkillPolicy:          opts.SkillPolicy,
			Reviewer:             agent.DefaultWorker,
			ReviewerResponsePath: opts.ReviewerResponsePath,
			Policy:               opts.InsightPolicy,
			ApprovalDecision:     opts.LearningApprovalDecision,
			ApprovalReason:       opts.LearningApprovalReason,
			ApprovalReviewer:     opts.LearningApprovalReviewer,
			ConfirmApply:         opts.LearningConfirmApply,
			ArtifactsDir:         opts.ArtifactsDir,
			RunID:                runID,
			Now:                  now,
		})
		if err != nil {
			return s.failDispatch(ctx, opts, steps, run, workItem, lease, reservation, true, now, err)
		}
		if materialized.Materialized || materialized.Skipped {
			learningLoop = &materialized
		}
	}

	normalized, err := usage.NormalizeWorkerMetadata(usage.NormalizeInput{
		Metadata: workerResult.UsageMeta, Worker: workerName, ModelProfile: agent.ModelProfile,
	})
	if err != nil {
		return s.failDispatch(ctx, opts, steps, run, workItem, lease, reservation, true, now, err)
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
				return s.failDispatch(ctx, opts, steps, run, workItem, lease, reservation, true, now, err)
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

	budgetCommitted := false
	if reservation.ID != "" {
		if _, err := s.Repo.CommitReservationWithUsage(ctx, reservation.ID, usageEvent, actualMicroUSD, now); err != nil {
			return s.failDispatch(ctx, opts, steps, run, workItem, lease, reservation, true, now, err)
		}
		budgetCommitted = true
	}
	steps = append(steps, StepCommitBudget, StepPersistUsage)

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
	_ = ReleaseActiveLease(ctx, s.Repo, lease, "dispatch complete", false, finished)
	steps = append(steps, StepFinalizeRun)

	var evidence *insights.EvidenceBundle
	var evidencePath string
	if opts.EvidenceBuilder != nil && run.Status == runs.StatusSucceeded {
		bundle, err := opts.EvidenceBuilder(ctx, s.Repo, runID, now)
		if err == nil {
			evidence = &bundle
			if strings.TrimSpace(opts.ArtifactsDir) != "" {
				evidencePath = filepath.Join(opts.ArtifactsDir, "evidence", runID, "evidence-bundle.json")
			}
			run.DispatchMeta.EvidenceBundleID = bundle.EvidenceBundleID
			_ = s.Repo.SaveRun(ctx, run)
		}
	}
	steps = append(steps, StepBuildEvidence)

	var review *InsightReviewResult
	if run.Status == runs.StatusSucceeded && evidence != nil {
		reviewResult, err := EnqueueInsightReview(ctx, s.Repo, workItem, runID, *evidence, evidencePath, agent, now)
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
		SkillSnapshotApplied: skillApplied, BudgetReserved: true, UsageRecorded: true,
		BudgetCommitted: budgetCommitted, WorkStatus: workItem.Status,
		LeaseStatus: workqueue.LeaseStatusReleased, EvidenceBundleCreated: evidence != nil,
		ReviewWorkQueued: review != nil && review.Queued, LeaseReleased: true,
	})
	result.EvidenceBundle = evidence
	result.InsightReview = review
	result.LearningLoop = learningLoop
	result.DispatchStarted = true
	result.LeaseAcquired = true
	result.LeaseRequired = true
	return result, nil
}

func (s Service) failDispatch(ctx context.Context, opts OnceOptions, steps []string, run *runs.Run, workItem agents.WorkItem, lease workqueue.Lease, reservation budget.Reservation, workerStarted bool, now time.Time, err error) (OnceResult, error) {
	budgetReleased := false
	if reservation.ID != "" {
		_, _ = s.Repo.ReleaseReservation(ctx, reservation.ID, err.Error(), now)
		budgetReleased = true
	}
	leaseReleased := false
	if strings.TrimSpace(lease.ID) != "" {
		if releaseErr := ReleaseActiveLease(ctx, s.Repo, lease, "dispatch_failed:"+err.Error(), true, now); releaseErr == nil {
			leaseReleased = true
		}
	}
	if run != nil && strings.TrimSpace(run.ID) != "" {
		finished := now
		run.Status = runs.StatusFailed
		run.UpdatedAt = finished
		run.FinishedAt = &finished
		run.DispatchMeta.BlockedReason = err.Error()
		_ = s.Repo.SaveRun(ctx, run)
	}
	workItem.Status = agents.WorkItemStatusFailed
	workItem.UpdatedAt = now.Format(time.RFC3339Nano)
	_ = s.Repo.SaveWorkItem(ctx, workItem)
	result := NewErrorResult(opts, steps, err)
	result.LeaseID = lease.ID
	result.LeaseAcquired = lease.ID != ""
	result.WorkerStarted = workerStarted
	result.LeaseReleased = leaseReleased
	result.BudgetReleased = budgetReleased
	result.DispatchStarted = workerStarted
	return result, err
}

func DefaultPricingLoader(ctx context.Context, repo Repository, modelProfile string, at time.Time) (usage.ModelPrice, error) {
	_ = at
	return repo.ModelPrice(ctx, modelProfile)
}
