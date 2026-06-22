# Proactive Runtime: Daemon, Cron, Heartbeat, Hooks

## Purpose

This document defines the runtime layer that makes DeonClaw proactive. The current CLI-driven harness can execute tasks on demand. The proactive runtime adds a durable daemon, schedules, heartbeat turns, wakeup queues, hooks, idempotency, concurrency policy, and recovery.

The goal is to let DeonClaw run as a stable personal system that can check, plan, delegate, and execute work without relying on an external reminder loop.

## Design goals

- Add `deond`, a long-running local daemon.
- Persist schedules, wakeups, and run history in SQLite.
- Support one-shot, fixed-interval, cron, webhook, and heartbeat triggers.
- Separate precise schedules from context-aware heartbeat.
- Ensure jobs survive restarts and execute at most once per due window.
- Defer or skip work when the assigned agent is busy.
- Add active hours, timezone handling, catch-up policy, concurrency limits, and no-op contracts.
- Make every proactive action auditable.

## Non-goals

- No UI scheduler in the first pass.
- No mobile push notifications in the first pass.
- No channel adapters in the first pass except metadata-only plans.
- No real provider dispatch beyond existing worker execution rules.
- No direct cron mutation by agents without policy.

## Runtime components

```text
deond
  ├── Schedule Store
  ├── Wakeup Queue
  ├── Heartbeat Engine
  ├── Hook Dispatcher
  ├── Work Queue bridge
  ├── Lease/Budget gate
  ├── Worker Dispatcher
  └── Recovery/Maintenance loop
```

## Schedule types

| Type | Use case | Precision | Creates work item? |
|---|---|---:|---:|
| `at` | one-shot reminder or task | exact | yes |
| `every` | fixed interval | approximate/exact config | yes |
| `cron` | wall-clock schedule | exact | yes |
| `webhook` | external trigger | event-driven | yes |
| `heartbeat` | periodic context-aware check | approximate | configurable |
| `hook` | lifecycle or event reaction | event-driven | configurable |

## Cron vs heartbeat

DeonClaw should preserve the distinction:

| Dimension | Cron / Scheduled task | Heartbeat |
|---|---|---|
| Timing | exact or configured recurrence | approximate cadence |
| Context | isolated or selected session | agent main/session context |
| Work record | creates work item and run | may create insight or notification; work item optional |
| Best for | reports, reminders, batch jobs | inbox checks, calendar awareness, status review |
| Output | artifact, notification, or silent | no-op or concise alert |

Heartbeat should not spam. If no action is needed, the run returns a no-op token such as `HEARTBEAT_OK` or a structured no-notify response.

## Schedule model

```json
{
  "id": "sched_...",
  "kind": "cron",
  "name": "Daily repository review",
  "status": "active",
  "agent_id": "engineering-manager",
  "work_template_id": "tmpl_...",
  "cron": "0 9 * * 1-5",
  "timezone": "America/Sao_Paulo",
  "active_hours": {"start": "08:00", "end": "22:00"},
  "concurrency_policy": "skip_if_running",
  "catch_up_policy": "next_only",
  "max_lateness_seconds": 900,
  "jitter_seconds": 60,
  "delivery_policy": "artifact_only",
  "created_at": "...",
  "updated_at": "..."
}
```

## Wakeup model

Wakeups are concrete due occurrences derived from schedules.

```json
{
  "id": "wakeup_...",
  "schedule_id": "sched_...",
  "agent_id": "engineering-manager",
  "due_at": "...",
  "status": "queued",
  "idempotency_key": "sched_...:2026-06-22T09:00:00-03:00",
  "attempt": 0,
  "created_at": "...",
  "claimed_at": null,
  "run_id": null
}
```

Wakeup statuses:

```text
queued
claimed
running
succeeded
failed
skipped
cancelled
expired
lost
```

## Heartbeat policy

```yaml
heartbeat_policies:
  backend-default:
    enabled: true
    every: 30m
    target: none
    active_hours:
      start: "08:00"
      end: "22:00"
      timezone: America/Sao_Paulo
    skip_when_busy: true
    isolated_session: false
    light_context: true
    timeout_seconds: 120
    no_op_token: HEARTBEAT_OK
    prompt: |
      Review due items, failed runs, pending approvals, and follow-ups.
      If nothing needs attention, return HEARTBEAT_OK.
```

## No-op contract

A heartbeat result is no-op when:

- structured result has `notify: false`; or
- text starts or ends with configured no-op token and remaining text is under `ack_max_chars`.

No-op heartbeat should still record a compact event and should not create noisy artifacts.

## Hook model

Initial hook events:

```text
daemon.started
daemon.stopping
schedule.due
run.completed
run.failed
approval.created
approval.denied
learning.proposal.created
skill.installed
memory.applied
```

Hook config:

```yaml
hooks:
  - id: post-run-insight
    event: run.completed
    action: insights.evidence.build
    enabled: true
    policy: internal-only
```

Hooks are policy-controlled. Shell hooks should be disabled by default until the action-policy system exists.

## Concurrency and idempotency

Concurrency policies:

| Policy | Meaning |
|---|---|
| `allow_parallel` | Create a new run even if prior one is active |
| `skip_if_running` | Mark due wakeup skipped when same schedule is active |
| `queue_after_running` | Keep due wakeup queued until active run finishes |
| `replace_pending` | Cancel old queued wakeup and keep the latest |
| `coalesce` | Merge multiple due wakeups into one work item |

Idempotency key must prevent duplicate execution after restart, crash, or double scheduler tick.

## Recovery

`deond` maintenance should detect:

- claimed wakeups without active run after TTL;
- runs marked running but no lease heartbeat;
- schedules whose next occurrence was not materialized;
- orphaned worktrees;
- stale locks;
- missed wakeups beyond catch-up policy;
- failed worker startup;
- timed-out heartbeat.

Recovery actions:

```text
mark lost
retry if policy allows
reschedule next occurrence
emit recovery event
create operator alert if needed
```

## Daemon CLI

```bash
deonctl daemon start
deonctl daemon stop
deonctl daemon status
deonctl daemon doctor
deonctl daemon run-once

deonctl schedules validate --config configs/examples/schedules.yaml
deonctl schedules create --config <schedule.yaml>
deonctl schedules list
deonctl schedules show <schedule-id>
deonctl schedules pause <schedule-id>
deonctl schedules resume <schedule-id>
deonctl schedules cancel <schedule-id>
deonctl schedules due --now
deonctl schedules runs --schedule <schedule-id>

deonctl heartbeat validate --config configs/examples/heartbeat.yaml
deonctl heartbeat run --agent <agent-id> --dry-run
deonctl heartbeat report --agent <agent-id>
```

## Store additions

```sql
schedules(id, kind, status, agent_id, config_json, next_due_at, created_at, updated_at)
wakeups(id, schedule_id, agent_id, due_at, status, idempotency_key, attempt, run_id, created_at, claimed_at, finished_at)
heartbeat_state(agent_id, policy_id, last_due_at, last_run_at, last_status, last_summary, updated_at)
execution_leases(id, agent_id, work_item_id, run_id, status, expires_at, heartbeat_at, created_at)
daemon_state(key, value_json, updated_at)
hook_definitions(id, event, action, status, config_json, created_at, updated_at)
hook_runs(id, hook_id, event_id, status, result_json, created_at, finished_at)
```

## Integration with agents

A schedule targets an agent, not a worker. The agent manager resolves:

```text
agent → session → skill snapshot → memory snapshot → worker binding → budget policy → run
```

If the agent is paused, terminated, over budget, or already busy according to policy, the wakeup is skipped or deferred.

## Integration with learning

Heartbeat and scheduled runs should create evidence bundles when they produce actionable output or failures.

Examples:

- heartbeat finds recurring failed validation → insight trigger;
- cron report fails twice → repeated_error trigger;
- schedule is skipped due to budget → budget insight;
- no-op heartbeat → compact event only.

## Tests

Minimum tests:

- schedule config validate;
- cron expression parse with timezone;
- one-shot schedule creates one wakeup;
- restart simulation does not duplicate wakeup;
- idempotency key prevents duplicate run;
- skip_if_running works;
- coalesce merges due wakeups;
- heartbeat disabled with `0m` or enabled false;
- active hours skip outside window;
- heartbeat no-op suppresses notification;
- claimed wakeup TTL recovery marks lost;
- daemon run-once processes one due wakeup;
- paused agent blocks schedule execution;
- policy failure records event and no worker run.

## Implementation sequence

### 23.12 — Daemon and schedule store

- `cmd/deonclawd` or `deonctl daemon run`;
- schedule schema;
- SQLite tables;
- one-shot and interval schedules;
- run-once processor;
- docs and examples.

### 23.13 — Wakeup queue and recovery

- wakeup materialization;
- idempotency;
- claim/finish lifecycle;
- recovery doctor/report;
- basic concurrency policy.

### 23.14 — Heartbeat runtime

- heartbeat policy;
- active hours;
- skipWhenBusy;
- no-op contract;
- evidence generation when actionable.

### 23.15 — Hooks and integration smoke

- lifecycle hooks;
- internal action hooks only;
- scheduled insight trigger;
- e2e fixture: schedule survives restart and heartbeat no-ops.

## Documentation updates for implementation

When implementing this epic, update:

- `README.md` status;
- `docs/EPIC_23_ROADMAP.md`;
- `docs/PERSISTENT_AGENTS.md` if agent/session references change;
- `docs/INSIGHT_LEARNING_LOOP.md` for heartbeat-triggered insights;
- examples under `configs/examples/schedules.yaml` and `configs/examples/heartbeat.yaml`;
- smoke script for proactive runtime fixture.