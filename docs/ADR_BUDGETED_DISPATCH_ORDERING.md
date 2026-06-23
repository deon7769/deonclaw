# ADR: Budgeted dispatch ordering

## Status

Accepted — Epic 23.16–23.19

## Context

Work queue claims, budget reservations, immutable snapshots, and worker execution were implemented as separate foundations. Without a fixed order, workers could start before budget reservation or task YAML could drift after assignment.

## Decision

Dispatch follows this strict sequence:

```text
task snapshot validation
→ agent/session/skill validation and materialization
→ budget window + atomic reservation
→ lease bind (must be active)
→ run creation with dispatch metadata
→ worker execution
→ usage normalization + budget commit/release
→ work / lease / wakeup terminalization
→ evidence bundle + optional insight_review work item
```

No worker process may start before a successful budget reservation when `reserve_before_run` is true.

## Consequences

- Task YAML is never re-read at dispatch; `work_item_task_snapshots` is authoritative.
- Session skill snapshots remain immutable; new skill revisions require new sessions.
- Budget exhaustion blocks worker start, releases lease, emits `work.budget.blocked`.
- Fake CI path keeps `provider_call:false` and `network_call:false`.
- Real dispatch requires `--confirm-worker-dispatch` and is excluded from CI.

## Recovery

- Expired leases: `deonctl work recover` (mutating); `deonctl work doctor` is read-only.
- Budget reservations: idempotent commit/release; overage records actual cost and applies `on_exhausted`.
- Skill rollback: `deonctl skills rollback` with approval artifact (no automatic apply).
