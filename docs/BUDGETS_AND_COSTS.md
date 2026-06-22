# Budgets, Costs, Leases, and Work Queue

## Purpose

This document defines the operational accounting layer for DeonClaw. Before DeonClaw can run persistent agents, scheduled work, and multi-worker delegation, it needs atomic work checkout, execution leases, cost events, budget reservations, and hard stops.

The goal is to prevent runaway spend, duplicate work, stale locks, and uncontrolled autonomous execution.

## Design goals

- Track cost and usage per run, agent, worker, model, work item, domain, and time window.
- Reserve budget before execution and commit actual cost after execution.
- Enforce warning thresholds and hard stops.
- Prevent duplicate execution with atomic leases.
- Support retries and cancellation without double-spending.
- Provide reports that explain where cost went and why work was blocked.
- Keep budget enforcement runtime-owned, not worker-owned.

## Non-goals

- No billing integration with provider APIs in the first pass.
- No exact token pricing guarantee for every provider at first.
- No multi-company billing model yet.
- No autonomous budget increase.
- No silent retry beyond approved policy.

## Core model

```text
WorkItem
  ↓
Budget preflight
  ↓
Budget reservation
  ↓
Execution lease
  ↓
Worker run
  ↓
Usage/cost events
  ↓
Budget commit or release
  ↓
Run report and insight triggers
```

## Work queue

The work queue owns assignable work. Agents claim work through leases, not by directly starting workers.

Work statuses:

```text
queued
blocked
leased
running
review_required
succeeded
failed
cancelled
dead_letter
```

Work item fields:

```json
{
  "id": "work_...",
  "status": "queued",
  "assigned_agent_id": "backend-engineer",
  "priority": 50,
  "attempt": 0,
  "max_attempts": 2,
  "budget_policy": "backend-monthly",
  "estimated_cost_usd": 0.15,
  "created_at": "...",
  "updated_at": "..."
}
```

## Execution lease

A lease prevents duplicate execution and allows recovery.

```json
{
  "lease_id": "lease_...",
  "work_item_id": "work_...",
  "agent_id": "backend-engineer",
  "run_id": "run_...",
  "status": "active",
  "ttl_seconds": 900,
  "expires_at": "...",
  "heartbeat_at": "...",
  "created_at": "..."
}
```

Lease rules:

- only one active lease per work item unless policy explicitly allows parallel work;
- lease acquisition is atomic;
- workers must renew lease heartbeat during long runs;
- expired lease can be recovered and marked lost;
- a run cannot commit cost if it no longer owns the lease;
- cancellation records lease release reason.

## Budget policy

```yaml
budget_policies:
  backend-monthly:
    scope: agent
    agent_id: backend-engineer
    period: monthly
    hard_limit_usd: 50.00
    warning_thresholds:
      - 0.5
      - 0.8
      - 0.95
    reserve_before_run: true
    default_estimate_usd: 0.25
    max_single_run_usd: 3.00
    on_exhausted: pause_agent
    allow_operator_override: true
```

Scopes:

```text
agent
project
domain
work_item
worker
model_profile
global
```

## Budget reservation

```json
{
  "reservation_id": "budres_...",
  "budget_policy": "backend-monthly",
  "scope": "agent:backend-engineer",
  "work_item_id": "work_...",
  "run_id": "run_...",
  "estimated_cost_usd": 0.25,
  "reserved_at": "...",
  "status": "reserved"
}
```

Reservation lifecycle:

```text
reserved
committed
released
expired
cancelled
```

If execution fails before the provider/model call or worker call begins, the reservation may be released. If partial usage occurred, commit partial actual cost.

## Usage event

```json
{
  "usage_event_id": "usage_...",
  "run_id": "run_...",
  "agent_id": "backend-engineer",
  "worker": "opencode",
  "provider": "z-ai",
  "model": "glm-5.1",
  "model_profile": "opencode-zai-glm-5-1",
  "input_tokens": 12000,
  "output_tokens": 2300,
  "cached_input_tokens": 0,
  "tool_call_count": 3,
  "estimated_cost_usd": 0.18,
  "actual_cost_usd": null,
  "source": "worker_metadata",
  "created_at": "..."
}
```

Sources:

- `worker_metadata`;
- `provider_report`;
- `estimated`;
- `manual_adjustment`;
- `imported`.

## Price table

```yaml
model_prices:
  opencode-zai-glm-5-1:
    provider: z-ai
    model: glm-5.1
    currency: USD
    input_per_million: 0.20
    output_per_million: 0.80
    cached_input_per_million: 0.02
    effective_at: "2026-06-22"
```

Price tables are operator-maintained and versioned. Unknown model pricing should fail budget preflight unless policy allows estimated default.

## Hard stops

Hard stop rules:

- no reservation if budget is exhausted;
- no new scheduled work if budget is exhausted;
- paused agents cannot acquire leases;
- queued work may be cancelled or deferred on exhaustion;
- work already running may be allowed to finish unless `kill_on_exhaustion` is configured;
- operator override requires approval artifact.

## Reports

Commands:

```bash
deonctl budgets validate --config configs/examples/budgets.yaml
deonctl budgets plan --agent backend-engineer
deonctl budgets status --agent backend-engineer
deonctl budgets report --by agent
deonctl budgets report --by model_profile
deonctl budgets reserve --work-item <id> --estimate-usd 0.25
deonctl budgets commit --reservation <id> --usage <usage.json>
deonctl budgets release --reservation <id>

deonctl queue list
deonctl queue show <work-item-id>
deonctl queue claim --agent <agent-id> --work-item <id>
deonctl queue release --lease <lease-id>
deonctl queue recover
deonctl queue dead-letter list
```

## Store additions

```sql
budget_policies(id, scope, config_json, status, created_at, updated_at)
budget_windows(id, policy_id, period_start, period_end, hard_limit_usd, committed_usd, reserved_usd, status)
budget_reservations(id, policy_id, window_id, work_item_id, run_id, estimated_cost_usd, status, created_at, updated_at)
usage_events(id, run_id, agent_id, worker, provider, model, model_profile, usage_json, estimated_cost_usd, actual_cost_usd, source, created_at)
model_prices(id, provider, model, currency, price_json, effective_at, created_at)
execution_leases(id, work_item_id, agent_id, run_id, status, expires_at, heartbeat_at, created_at, updated_at)
work_queue_events(id, work_item_id, event_type, payload_json, created_at)
```

## Integration with workers

Workers should expose normalized metadata when possible:

```json
{
  "provider": "z-ai",
  "model": "glm-5.1",
  "input_tokens": 12000,
  "output_tokens": 2300,
  "tool_calls": 3,
  "duration_ms": 128000
}
```

If a worker does not provide usage, DeonClaw may estimate from prompt/output length and model profile.

## Integration with scheduler

Scheduled work checks budget before becoming runnable.

```text
schedule due
  ↓
work item created
  ↓
budget preflight
  ↓
queued or budget_blocked
```

When budget is exhausted:

- heartbeat reports the blocked work once;
- schedules remain active but produce skipped wakeups according to policy;
- operator can raise budget by approval.

## Integration with insights

Budget events can trigger learning:

- repeated high-cost workflow;
- unusually expensive model profile;
- many retries;
- low value output;
- scheduled task always no-ops but costs tokens.

Learning proposals may suggest:

- cheaper worker/model profile;
- isolated heartbeat context;
- schedule cadence reduction;
- skill improvement to reduce retries;
- tighter retrieval context limits.

## Tests

Minimum tests:

- budget policy validate;
- price table validate;
- reservation succeeds under limit;
- reservation fails over hard limit;
- commit updates committed and reserved amounts atomically;
- release frees reserved amount;
- duplicate lease claim fails;
- expired lease recovery marks lost;
- budget exhaustion pauses agent when configured;
- schedule due creates budget-blocked work when exhausted;
- usage event report by model profile;
- unknown price fails unless estimated default is allowed;
- operator override requires approval.

## Implementation sequence

### 23.16 — Work queue and leases

- WorkItem queue tables;
- atomic claim/release;
- lease TTL and recovery;
- CLI list/show/claim/recover.

### 23.17 — Usage events and price table

- normalized usage artifacts;
- model price config;
- estimated cost calculation;
- run report integration.

### 23.18 — Budget policies and reservations

- budget validation;
- reservation/commit/release;
- hard stop before worker dispatch;
- budget status/report.

### 23.19 — Scheduler/agent integration

- scheduled work budget preflight;
- agent pause on budget exhaustion;
- insights for cost anomalies;
- e2e: budget blocks queued scheduled work.

## Documentation updates for implementation

When implementing this epic, update:

- `README.md` status;
- `docs/EPIC_23_ROADMAP.md`;
- `docs/PERSISTENT_AGENTS.md` for agent budget fields;
- `docs/PROACTIVE_RUNTIME.md` for schedule budget behavior;
- examples under `configs/examples/budgets.yaml`;
- run report documentation.