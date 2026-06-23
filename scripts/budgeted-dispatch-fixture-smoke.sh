#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

DB="$TMP/budgeted.db"
CLI="$TMP/deonctl"
FIXTURE="configs/examples/budgeted-dispatch-fixture"
TASK="$FIXTURE/task-fake-success.yaml"

echo "==> build deonctl"
go build -o "$CLI" ./cmd/deonctl

echo "==> validate configs"
"$CLI" pricing validate --config configs/examples/model-prices.yaml
"$CLI" budgets validate --config configs/examples/budgets.yaml
"$CLI" work-templates validate --config configs/examples/work-templates.yaml

echo "==> sync store"
"$CLI" agents sync --config configs/examples/agents.yaml --store "$DB" --workers-config configs/examples/workers.yaml
"$CLI" pricing sync --config configs/examples/model-prices.yaml --store "$DB"
"$CLI" budgets sync --config configs/examples/budgets.yaml --store "$DB"
"$CLI" work-templates sync --config configs/examples/work-templates.yaml --store "$DB"

echo "==> assign, accept, claim"
ASSIGN_OUT="$("$CLI" agents assign --agent backend-engineer --task "$TASK" --store "$DB" --created-by smoke)"
echo "$ASSIGN_OUT"
WORK_ITEM="$(echo "$ASSIGN_OUT" | sed -n 's/.*work_item=\([^ ]*\).*/\1/p')"
INBOX="$(echo "$ASSIGN_OUT" | sed -n 's/.*inbox=\([^ ]*\).*/\1/p')"
"$CLI" agents inbox accept "$INBOX" --store "$DB"
CLAIM_JSON="$("$CLI" work claim --store "$DB" --agent backend-engineer --work-item "$WORK_ITEM")"
echo "$CLAIM_JSON"
LEASE="$(echo "$CLAIM_JSON" | grep -o '"id": "[^"]*"' | head -1 | sed 's/.*"\([^"]*\)".*/\1/')"

echo "==> dispatch-once fake"
DISPATCH_JSON="$("$CLI" work dispatch-once --store "$DB" --work-item "$WORK_ITEM" --lease "$LEASE" --mode fake)"
echo "$DISPATCH_JSON"
echo "$DISPATCH_JSON" | grep -q '"status": "ok"' || { echo "dispatch status not ok" >&2; exit 1; }
echo "$DISPATCH_JSON" | grep -q '"worker_started": true' || { echo "worker not started" >&2; exit 1; }
echo "$DISPATCH_JSON" | grep -q '"budget_reserved": true' || { echo "budget not reserved" >&2; exit 1; }
echo "$DISPATCH_JSON" | grep -q '"provider_call": false' || { echo "provider_call not false" >&2; exit 1; }

echo "==> package tests"
go test ./internal/dispatch -run 'TestDispatchOnce(FakeSuccess|BudgetBlocked)' -count=1

echo "budgeted-dispatch-fixture-smoke: ok"
