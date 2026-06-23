# Budgeted dispatch

Epic 23.16–23.19 connects work queue leases, atomic budget reservations, immutable task/session/skill snapshots, and fake or explicit real worker dispatch.

## Command

```bash
deonctl work dispatch-once \
  --store <deonclaw.db> \
  --work-item <work-item-id> \
  --lease <lease-id> \
  --mode fake|real \
  [--confirm-worker-dispatch]
```

Default mode is `fake`. Real mode requires `--confirm-worker-dispatch` and reuses existing `CodexRunner` / `OpenCodeRunner` harnesses — no parallel provider SDK.

## Ordering (mandatory)

1. Load queued work and validate dependencies
2. Validate agent lifecycle and concurrency
3. Load and verify task snapshot SHA-256
4. Resolve session and skill snapshot
5. Materialize skills into workspace (immutable session snapshot)
6. Budget preflight and atomic reservation
7. Bind active lease
8. Create run record with dispatch metadata
9. Start worker (never before reservation)
10. Normalize usage and commit/release budget
11. Terminal work state, release lease, finalize wakeup
12. Evidence bundle and insight review work item when triggered

See [ADR_BUDGETED_DISPATCH_ORDERING.md](ADR_BUDGETED_DISPATCH_ORDERING.md).

## Fake vs real

| Boundary | CI / default | Real dispatch |
|---|---|---|
| `provider_call` | false | true (worker only) |
| `network_call` | false | worker-dependent |
| `secret_values_read` | false | worker env only |
| `real_worker_execution` | false | explicit flag |

CI runs `make budgeted-dispatch-smoke` with fake workers only.

## Micro-USD

Authoritative costs use `amount_microusd` integers. `amount_usd` is display-only. Hard budget stops never use `float64`.

## Learning loop

Dispatch may create `insight_review` work items and learning proposals. Skill/memory/rule apply never runs automatically — approval artifacts are required.

## Fixture

```bash
make budgeted-dispatch-smoke
```

Config tree: `configs/examples/budgeted-dispatch-fixture/`.
