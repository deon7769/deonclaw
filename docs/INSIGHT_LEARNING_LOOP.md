# Insight and Learning Loop

## Purpose

This document defines how DeonClaw will learn from its own executions without silently mutating memory, skills, or policy.

The goal is to replace ad-hoc manual reflection with a stable, auditable loop similar in spirit to modern coding agents that periodically self-evaluate after commits, failures, and completed runs. DeonClaw should surface useful insights while preserving operator control.

## Design goals

- Capture actionable lessons from real runs, commits, CI failures, approvals, and corrections.
- Produce insights without storing hidden chain-of-thought.
- Create governed proposals for memory, skills, evals, rules, workflows, or follow-up tasks.
- Keep auto-evaluation on by default, but keep auto-application off by default.
- Allow Codex and OpenCode to review each other's work as evaluator agents.
- Make repeated failures and repeated corrections visible.
- Measure whether approved learning actually improves future runs.

## Non-goals

- No silent writes to canonical memory.
- No silent edits to `SKILL.md`.
- No model fine-tuning.
- No unrestricted self-modification.
- No provider call expansion beyond the already approved worker execution paths.
- No chain-of-thought persistence.

## Conceptual loop

```text
Run / session / commit / correction
        ↓
Evidence Bundle
        ↓
Trigger Policy
        ↓
Insight Evaluator
        ↓
Insight Report
        ↓
Learning Proposals
        ↓
Approval / Reject / Edit
        ↓
Memory, Skill, Eval, Rule, or Workflow update
        ↓
Next run loads approved learning
        ↓
Effectiveness measured
```

## Core entities

### EvidenceBundle

Evidence bundles are immutable references to what happened. They should be small enough to inspect, but rich enough to explain conclusions.

Fields:

```json
{
  "evidence_bundle_id": "evb_...",
  "created_at": "...",
  "scope": {
    "repository": "deonclaw",
    "agent_id": "backend-engineer",
    "session_id": "ses_...",
    "work_item_id": "work_..."
  },
  "trigger": "commit_created",
  "run_ids": ["run_..."],
  "event_ids": ["evt_..."],
  "artifact_ids": ["art_..."],
  "commit_shas": ["abc123"],
  "validation_results": [
    {"command": "go test ./...", "status": "ok"}
  ],
  "diff_summary": {
    "files_changed": 3,
    "insertions": 120,
    "deletions": 12
  },
  "user_feedback_refs": [],
  "policy_refs": [],
  "contains_text_excerpt": false
}
```

Evidence bundles must reference large artifacts by path/hash instead of copying large logs or payloads.

### InsightTrigger

Triggers decide whether an insight review should run.

Initial trigger types:

| Trigger | Purpose |
|---|---|
| `run_completed` | Learn from successful or failed worker runs |
| `commit_created` | Review coding progress after code changes |
| `validation_failed` | Capture failure pattern and recovery path |
| `user_correction` | Convert explicit correction into reusable guidance |
| `approval_denied` | Learn why a proposed action was unsafe or wrong |
| `repeated_error` | Escalate recurring failures |
| `cost_threshold_exceeded` | Detect expensive workflows |
| `manual_insight_request` | Operator asks for a review |
| `heartbeat_summary` | Future proactive runtime asks for periodic self-review |

Example policy:

```yaml
insight_policy:
  enabled: true
  evaluate_after_runs: 1
  evaluate_after_commits: 2
  evaluate_after_turns: 6
  always_on:
    - validation_failed
    - user_correction
    - approval_denied
    - repeated_error
  reviewer:
    preferred: codex
    fallback: opencode
  auto_propose: true
  auto_apply: false
```

### InsightReport

An insight report is the human-readable and machine-readable review output.

Required fields:

```json
{
  "insight_id": "ins_...",
  "status": "ok",
  "created_at": "...",
  "trigger": "commit_created",
  "scope": {
    "agent_id": "backend-engineer",
    "session_id": "ses_...",
    "repository": "deonclaw"
  },
  "evidence_bundle_sha256": "...",
  "observations": [
    "The CLI parser repeatedly omitted required path validation."
  ],
  "what_worked": [
    "A shared requireCLIPaths helper removed duplicated validation."
  ],
  "what_failed": [],
  "reusable_lessons": [
    "New multi-input CLI handlers should use the common required-path helper."
  ],
  "uncertainties": [],
  "risk_notes": [],
  "proposal_count": 2,
  "action_required": true,
  "contains_chain_of_thought": false
}
```

### LearningProposal

A proposal is a governed change request derived from an insight.

Types:

- `memory_add`
- `memory_replace`
- `skill_create`
- `skill_patch`
- `eval_case`
- `rule`
- `workflow`
- `routing_policy`
- `budget_policy`
- `documentation`
- `technical_debt`
- `follow_up_task`

Fields:

```json
{
  "proposal_id": "lp_...",
  "type": "skill_patch",
  "status": "pending",
  "target": "deonclaw-sprint-implementation",
  "insight_id": "ins_...",
  "evidence_bundle_sha256": "...",
  "reason": "Repeated CLI handlers forgot required flag checks.",
  "proposed_change_summary": "Add a rule requiring requireCLIPaths for multi-input handlers.",
  "patch_path": "artifacts/proposed-skill.patch",
  "confidence": 0.94,
  "auto_apply_allowed": false,
  "approval_required": true
}
```

## CLI commands

Initial command set:

```bash
deonctl insights policy validate --config configs/examples/insight-policy.yaml
deonctl insights evidence build --run <run-id> --output <evidence-bundle.json>
deonctl insights evaluate --evidence <evidence-bundle.json> --reviewer codex --output <insight-report.json>
deonctl insights proposals list --insight <insight-report.json>
deonctl insights proposals show <proposal-id>
deonctl insights proposals approve <proposal-id> --output <approval.json>
deonctl insights proposals apply <proposal-id> --approval <approval.json>
deonctl insights report --insight <insight-report.json>
deonctl insights timeline --agent <agent-id>
```

All mutating commands must support dry-run/report first.

## First vertical slice

The first implementation must prove a complete cycle:

```text
1. OpenCode executes a real task in a worktree.
2. DeonClaw captures events, diff, tests, and commit SHA.
3. Evidence bundle is created.
4. Codex evaluates the evidence as learning reviewer.
5. Insight report is produced.
6. A skill patch proposal is created.
7. Operator approves the proposal.
8. Skill patch is applied to `.agents/skills/.../SKILL.md`.
9. Next Codex/OpenCode session receives the skill snapshot.
10. An eval records whether the same failure pattern was avoided.
```

## Reviewer behavior

The learning reviewer should answer only with structured insight. It must not produce hidden reasoning logs.

Reviewer prompt shape:

```text
You are the DeonClaw learning reviewer.
Review only the provided evidence bundle and referenced summaries.
Do not infer secrets or unstated facts.
Produce observations, reusable lessons, uncertainties, and proposals.
Do not claim a proposal should be applied automatically unless policy allows it.
Do not include chain-of-thought.
```

## Approval model

Default policy:

| Proposal type | Auto-create | Auto-apply |
|---|---:|---:|
| memory_add | yes | no |
| memory_replace | yes | no |
| skill_create | yes | no |
| skill_patch | yes | no |
| eval_case | yes | no |
| documentation | yes | no |
| follow_up_task | yes | configurable |
| routing_policy | yes | no |
| budget_policy | yes | no |

Auto-apply may be introduced later per trusted low-risk class, but it must remain opt-in.

## Storage

Suggested package layout:

```text
internal/insights/
  evidence.go
  trigger_policy.go
  evaluator.go
  report.go
  proposal.go
  approval.go
  apply.go
  timeline.go
```

Suggested store tables:

```sql
insight_evidence_bundles(id, scope_json, trigger, created_at, artifact_path, sha256)
insight_reports(id, evidence_bundle_id, status, created_at, report_path, sha256)
learning_proposals(id, insight_id, type, target, status, proposal_path, sha256, created_at)
learning_proposal_approvals(id, proposal_id, approved_by, approved_at, approval_path, sha256)
learning_effectiveness(id, proposal_id, run_id, metric, value, observed_at)
```

## Metrics

Track:

- insights created;
- proposals created;
- proposals approved/rejected;
- repeated failure count before/after approval;
- time-to-fix after learning;
- validation pass rate before/after;
- cost delta after workflow improvement;
- skill usage count;
- stale or unused learning proposals.

## Safety and privacy

- Never store secrets.
- Never store hidden chain-of-thought.
- Avoid raw full stdout when summaries and hashes are enough.
- Mark whether evidence includes user-provided sensitive text.
- Learning proposals must cite evidence IDs and artifacts.
- Applying proposals must create reversible Git changes.
- Failed insight evaluator calls must not block the original run.

## Tests

Minimum tests:

- trigger policy validate OK;
- trigger policy rejects auto_apply for high-risk proposal classes;
- evidence bundle does not include raw secret-like values;
- insight report rejects chain-of-thought fields;
- proposal requires evidence hash;
- approval rejects stale proposal hash;
- skill patch proposal applies only after approval;
- next-session skill snapshot includes approved skill revision;
- effectiveness metric can link proposal to later run.

## Documentation updates for implementation

When implementing this epic, update:

- `README.md` implemented/future status;
- `docs/EPIC_23_ROADMAP.md` sequencing if changed;
- `docs/UNIVERSAL_SKILLS.md` if proposal application touches skill registry;
- `docs/MEMORY_INDEX.md` if learning proposals affect memory workflows;
- fixture README with a full insight loop example;
- `AGENTS.md` if repository workflow rules change.