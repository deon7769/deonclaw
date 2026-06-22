# Proactive runtime fixture (Epic 23D)

Minimal schedules config for `scripts/proactive-runtime-fixture-smoke.sh`.

Uses shared example configs:

- `configs/examples/agents.yaml` — agent registry sync
- `configs/examples/heartbeat.yaml` — heartbeat policy (`backend-default`, `HEARTBEAT_OK` no-op)
- `configs/examples/hooks.yaml` — internal lifecycle hooks (plan-only)

Smoke flow:

1. validate schedules, heartbeat, and hooks configs
2. sync agents and schedules into an isolated SQLite store
3. re-sync schedules (restart simulation; no duplicate wakeups)
4. `daemon run-once` materializes and processes a heartbeat wakeup (dry-run, no worker)
5. second `run-once` confirms idempotency (`materialized: 0`)
6. `heartbeat dry-run` and `daemon doctor`
