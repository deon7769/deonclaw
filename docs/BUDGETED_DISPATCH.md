# Budgeted dispatch

Epic 23.16–23.19 connects work queue leases, atomic budget reservations, immutable task/session/skill snapshots, and fake worker dispatch. **23.19.1** hardened the integration path before merge.

## Command

```bash
deonctl work dispatch-once \
  --store <deonclaw.db> \
  --work-item <work-item-id> \
  --artifacts-dir <dir> \
  --registry-root <dir> \
  --skill-policy <skill-policy.yaml> \
  [--lease <lease-id>] \
  --mode fake|real \
  [--confirm-worker-dispatch] \
  [--timeout-seconds <seconds>] \
  [--lease-ttl-seconds <seconds>]
```

Default mode is `fake`. `--mode real` requires `--confirm-worker-dispatch`; without it dispatch returns `blocked` with `real_dispatch_requires_confirmation` and does not load/start a worker. CI and fixture smokes remain fake-only.

`--lease` is optional for queued work: dispatch auto-claims exactly one lease atomically. Already-leased work without `--lease` returns `not_started` without starting a worker. Explicit leases must still be active, match the work item and agent, and not be expired.

For real dispatch, `--lease-ttl-seconds` controls the initial lease TTL and renewal TTL, and `--timeout-seconds` cancels the worker context if it runs too long. Real dispatch renews the active lease while the worker is running, revalidates lease ownership before committing budget/usage, and runs configured local `validation.commands` after a successful worker return. Validation outputs are persisted as `validation/validation.json` and `validation/validation.log`; validation failure releases the budget reservation and lease before budget commit.

## Ordering (23.19.1)

1. Load queued work and validate agent/task snapshot
2. **Acquire or validate active lease** (required before worker)
3. **Require budget policy** (blocked with `budget_policy_required` if missing)
4. Budget preflight and atomic reservation
5. Resolve session and materialize skill snapshot into workspace
6. Create run, bind lease, mark work running
7. Start worker (fake by default; real only with explicit confirmation, never before lease + budget reservation)
8. Persist worker events/artifacts under the dispatch run when the worker returns them
9. In real mode, run configured local validation commands and persist validation artifacts
10. In real mode, verify the same active lease still owns the run before commit
11. **Atomic budget commit + usage event** in one transaction
12. Release lease, build evidence bundle, queue `insight_review` with task snapshot

See [ADR_BUDGETED_DISPATCH_ORDERING.md](ADR_BUDGETED_DISPATCH_ORDERING.md).

## Fake-only CI boundary

| Boundary | CI / default |
|---|---|
| `provider_call` | false |
| `network_call` | false |
| `secret_values_read` | false |
| `automatic_learning_apply` | false |
| `real_worker_execution` | manual only with `--mode real --confirm-worker-dispatch`; blocked in `CI`/`GITHUB_ACTIONS` with `real_dispatch_blocked_in_ci` |

CI runs `make budgeted-dispatch-smoke` with fake workers only.

## Micro-USD

Authoritative costs use `amount_microusd` integers. Budget commit records `overage_microusd` when actual exceeds estimate; window status may move to `warning` or `exhausted` without failing the commit of cost already incurred.

## Learning loop

Dispatch creates evidence under `--artifacts-dir`, then queues `insight_review` work with parent run/evidence refs and a read-only task snapshot. Recursive `insight_review` is blocked. Skill/memory apply never runs automatically.

## Fixture

```bash
make budgeted-dispatch-smoke
```

Config tree: `configs/examples/budgeted-dispatch-fixture/`.
