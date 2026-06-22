#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<EOF
Usage: bash scripts/proactive-runtime-fixture-smoke.sh

Runs Epic 23D proactive runtime fixture smoke (schedules, wakeups, heartbeat, hooks).

Validates example configs, syncs agents/schedules into an isolated SQLite store,
runs daemon run-once twice (restart/idempotency), and checks heartbeat no-op path.
EOF
}

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi
if [[ $# -gt 0 ]]; then
  echo "unknown argument: $1" >&2
  usage >&2
  exit 2
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

DB="$TMP/proactive.db"
CLI="$TMP/deonctl"
AGENTS_CFG="configs/examples/agents.yaml"
WORKERS_CFG="configs/examples/workers.yaml"
SCHEDULES_CFG="configs/examples/proactive-runtime-fixture/schedules-smoke.yaml"
HEARTBEAT_CFG="configs/examples/heartbeat.yaml"
HOOKS_CFG="configs/examples/hooks.yaml"

echo "==> build deonctl"
go build -o "$CLI" ./cmd/deonctl

echo "==> validate proactive configs"
"$CLI" schedules validate --config "$SCHEDULES_CFG"
"$CLI" heartbeat validate --config "$HEARTBEAT_CFG"
"$CLI" hooks validate --config "$HOOKS_CFG"

echo "==> sync agents and schedules"
"$CLI" agents sync --config "$AGENTS_CFG" --store "$DB" --workers-config "$WORKERS_CFG"
"$CLI" schedules sync --config "$SCHEDULES_CFG" --store "$DB"

echo "==> restart simulation (re-sync schedules)"
"$CLI" schedules sync --config "$SCHEDULES_CFG" --store "$DB"

echo "==> hooks plan (internal-only)"
"$CLI" hooks plan --config "$HOOKS_CFG" --event schedule.due | grep -q schedule-due-audit || {
  echo "hooks plan missing expected hook" >&2
  exit 1
}

echo "==> heartbeat dry-run"
"$CLI" heartbeat dry-run \
  --agent backend-engineer \
  --policy backend-default \
  --config "$HEARTBEAT_CFG" | grep -q '"would_run": true' || {
  echo "heartbeat dry-run blocked unexpectedly" >&2
  exit 1
}

echo "==> daemon run-once (first pass)"
OUT1="$("$CLI" daemon run-once --store "$DB" --heartbeat-config "$HEARTBEAT_CFG" --hooks-config "$HOOKS_CFG")"
echo "$OUT1"
if ! echo "$OUT1" | grep -Eq '"materialized": [1-9]|"processed": [1-9]'; then
  echo "first run-once did not materialize or process a wakeup" >&2
  exit 1
fi

echo "==> daemon run-once (idempotency / restart)"
OUT2="$("$CLI" daemon run-once --store "$DB" --heartbeat-config "$HEARTBEAT_CFG" --hooks-config "$HOOKS_CFG")"
echo "$OUT2"
if ! echo "$OUT2" | grep -q '"materialized": 0'; then
  echo "second run-once should not duplicate wakeups" >&2
  exit 1
fi

echo "==> daemon doctor"
"$CLI" daemon doctor --store "$DB"

echo "proactive runtime fixture smoke: ok"
