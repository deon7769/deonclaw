# Persistent Agents and Sessions

## Purpose

This document defines how DeonClaw will move from one-shot task execution to persistent agents with identity, roles, sessions, skills, memory scopes, work queues, and lifecycle state.

Current DeonClaw tasks name a worker such as `codex` or `opencode`. Epic 23 separates agent identity from worker runtime. Codex and OpenCode become interchangeable execution backends for persistent DeonClaw agents.

## Design goals

- Introduce persistent agent identity independent from worker backend.
- Support roles, goals, capabilities, supervisors, and budgets.
- Bind agents to preferred workers and model profiles without making worker identity equal agent identity.
- Track durable sessions and immutable skill snapshots.
- Allow assignment, delegation, inboxes, and dependencies.
- Prepare heartbeat, cron, and proactive runtime ownership.
- Preserve DeonClaw governance, artifact, and approval semantics.

## Non-goals

- No full UI/org chart in the first implementation.
- No multi-company model yet.
- No unrestricted autonomous execution.
- No hidden memory writes.
- No replacement of existing `Task` execution in one breaking change.

## Core distinction

```text
Agent  = identity, role, memory, goals, budget, skills, schedule
Worker = runtime adapter such as Codex CLI, OpenCode, Claude Code, browser, or shell
Run    = one concrete attempt by an agent through a worker
Session = durable context and skill snapshot for an agent over time
```

## Agent model

```json
{
  "id": "backend-engineer",
  "display_name": "Backend Engineer",
  "role": "engineer",
  "description": "Implements Go CLI/core features with tests and documentation.",
  "status": "active",
  "supervisor_id": "engineering-manager",
  "default_worker": "opencode",
  "fallback_workers": ["codex"],
  "model_profile": "opencode-zai-glm-5-1",
  "workspace_policy": "worktree-per-run",
  "memory_scope": "mysecondbrain",
  "skill_policy": "backend-engineer",
  "budget_policy": "backend-monthly",
  "heartbeat_policy": "backend-default",
  "created_at": "...",
  "updated_at": "..."
}
```

## Agent lifecycle

States:

```text
active
paused
draining
terminated
quarantined
```

Meanings:

| State | Meaning |
|---|---|
| `active` | May receive assigned work and heartbeat wakeups |
| `paused` | Keeps state but does not start new work |
| `draining` | Finishes owned runs but receives no new work |
| `terminated` | Cannot run; retained for audit |
| `quarantined` | Blocked due to policy or repeated unsafe behavior |

State changes require events and, for destructive transitions, approval artifacts.

## Session model

A session persists context about an agent's current mode of work.

```json
{
  "id": "ses_backend_main",
  "agent_id": "backend-engineer",
  "kind": "main",
  "status": "active",
  "worker": "opencode",
  "model_profile": "opencode-zai-glm-5-1",
  "workspace_path": "...",
  "skill_snapshot_id": "snap_...",
  "memory_snapshot_id": "memsnap_...",
  "created_at": "...",
  "last_active_at": "..."
}
```

Session kinds:

- `main` — default persistent context;
- `heartbeat` — periodic check context;
- `cron:<schedule-id>` — isolated scheduled work;
- `work:<work-item-id>` — task-specific context;
- `review:<run-id>` — reviewer context;
- `scratch` — temporary operator-created session.

## Work item model

`Task` remains as an input format, but `WorkItem` becomes the operational unit.

```json
{
  "id": "work_...",
  "title": "Implement skill registry loader",
  "goal_id": "goal_epic_23",
  "status": "queued",
  "assigned_agent_id": "backend-engineer",
  "created_by": "operator",
  "priority": 50,
  "parent_work_item_id": null,
  "dependencies": ["work_..."],
  "blocked_by": [],
  "definition_of_done": ["go test ./...", "docs updated"],
  "allowed_paths": ["internal/skills/**", "docs/**"],
  "forbidden_paths": ["memory/**/secrets/**"],
  "budget_policy": "backend-monthly",
  "schedule_id": null
}
```

## Delegation

Agents may create child work proposals, but initial execution should remain governed.

Delegation flow:

```text
parent agent identifies subtask
  ↓
delegation proposal
  ↓
policy validates target agent, scope, budget, and paths
  ↓
operator or supervisor approval if required
  ↓
child WorkItem created
  ↓
child run executes with parent link
```

Delegation guardrails:

- maximum child tasks per run;
- maximum delegation depth;
- no widening paths without approval;
- no escalation to more privileged agent without approval;
- parent run records child refs;
- child results route back to parent/supervisor.

## Agent registry CLI

Initial commands:

```bash
deonctl agents validate --config configs/examples/agents.yaml
deonctl agents list
deonctl agents show <agent-id>
deonctl agents create --config <agent.yaml>
deonctl agents pause <agent-id>
deonctl agents resume <agent-id>
deonctl agents terminate <agent-id> --approval <approval.json>
deonctl agents sessions list --agent <agent-id>
deonctl agents sessions show <session-id>
deonctl agents assign --agent <agent-id> --task <task.yaml>
deonctl agents inbox list --agent <agent-id>
deonctl agents inbox accept <item-id>
deonctl agents inbox defer <item-id>
```

## Config example

```yaml
agents:
  defaults:
    workspace_strategy: worktree-per-run
    memory_scope: mysecondbrain
    skill_policy: default
    heartbeat_policy: disabled
  list:
    - id: engineering-manager
      display_name: Engineering Manager
      role: manager
      default_worker: codex
      model_profile: codex-default
      skills:
        - project-planning
        - code-review
    - id: backend-engineer
      display_name: Backend Engineer
      role: engineer
      supervisor_id: engineering-manager
      default_worker: opencode
      fallback_workers:
        - codex
      model_profile: opencode-zai-glm-5-1
      skills:
        - go-cli-development
        - testing
```

## Store additions

Suggested tables:

```sql
agents(id, display_name, role, status, supervisor_id, config_json, created_at, updated_at)
agent_capabilities(agent_id, capability, policy_json)
agent_sessions(id, agent_id, kind, status, worker, model_profile, workspace_path, skill_snapshot_id, memory_snapshot_id, created_at, last_active_at)
work_items(id, title, status, assigned_agent_id, parent_work_item_id, priority, config_json, created_at, updated_at)
work_dependencies(work_item_id, depends_on_work_item_id, kind)
agent_inbox(id, agent_id, work_item_id, status, created_at, updated_at)
agent_lifecycle_events(id, agent_id, event_type, actor, payload_json, created_at)
```

## Run integration

`Run` should gain or be joinable to:

- `agent_id`;
- `session_id`;
- `work_item_id`;
- `attempt`;
- `parent_run_id`;
- `lease_id`;
- `model_profile`;
- `skill_snapshot_id`;
- `budget_reservation_id`;
- `trigger_type`;
- `schedule_id`.

Existing runs should remain readable. Migration can backfill `agent_id = legacy-manual`.

## Worker binding

Binding fields:

```json
{
  "agent_id": "backend-engineer",
  "worker": "opencode",
  "model_profile": "opencode-zai-glm-5-1",
  "runtime": "host",
  "sandbox": "workspace-write",
  "capabilities": ["code_edit", "test_run", "git_diff"],
  "max_concurrent_runs": 1
}
```

## Lifecycle events

Events:

```text
agent.created
agent.updated
agent.paused
agent.resumed
agent.terminated
agent.quarantined
agent.assigned
agent.delegated
agent.session.created
agent.session.snapshot_applied
agent.heartbeat.due
agent.heartbeat.skipped
```

## Tests

Minimum tests:

- parse valid agents config;
- reject duplicate agent IDs;
- reject invalid supervisor ref;
- reject worker unknown to `workers.yaml`;
- agent pause blocks new assignment;
- draining allows existing lease but blocks new work;
- terminated agent cannot run;
- session snapshot is immutable;
- delegation depth guard works;
- child work inherits narrowed paths;
- legacy Task can be converted to WorkItem;
- run records agent/session/work item refs.

## Implementation sequence

### 23.8 — Agent registry foundation

- config schema;
- SQLite tables;
- list/show/validate;
- lifecycle events;
- docs and examples.

### 23.9 — Sessions and worker binding

- session table;
- skill snapshot reference;
- worker/model binding;
- legacy manual session;
- run integration.

### 23.10 — Work items and inbox

- WorkItem model;
- assignment;
- inbox;
- basic dependencies;
- manual run from assigned work.

### 23.11 — Delegation and supervisor loop

- delegation proposal;
- policy validation;
- parent/child linkage;
- supervisor review flow.

## Documentation updates for implementation

When implementing this epic, update:

- `README.md` status;
- `docs/EPIC_23_ROADMAP.md`;
- `docs/PROACTIVE_RUNTIME.md` if heartbeat references change;
- `docs/BUDGETS_AND_COSTS.md` if agent budget fields change;
- examples under `configs/examples/agents.yaml`;
- task fixture docs if Task-to-WorkItem conversion changes.