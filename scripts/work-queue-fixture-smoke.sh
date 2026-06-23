#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

DB="$TMP/work-queue.db"
CLI="$TMP/deonctl"
TASK="configs/examples/budgeted-dispatch-fixture/task-fake-success.yaml"

go build -o "$CLI" ./cmd/deonctl
"$CLI" agents sync --config configs/examples/agents.yaml --store "$DB" --workers-config configs/examples/workers.yaml

ASSIGN_OUT="$("$CLI" agents assign --agent backend-engineer --task "$TASK" --store "$DB" --created-by smoke)"
WORK_ITEM="$(echo "$ASSIGN_OUT" | sed -n 's/.*work_item=\([^ ]*\).*/\1/p')"
INBOX="$(echo "$ASSIGN_OUT" | sed -n 's/.*inbox=\([^ ]*\).*/\1/p')"
"$CLI" agents inbox accept "$INBOX" --store "$DB"

CLAIM_JSON="$("$CLI" work claim --store "$DB" --agent backend-engineer --work-item "$WORK_ITEM")"
echo "$CLAIM_JSON" | grep -q '"claimed": true' || { echo "claim failed" >&2; exit 1; }

LEASE="$(echo "$CLAIM_JSON" | grep -o '"id": "[^"]*"' | head -1 | sed 's/.*"\([^"]*\)".*/\1/')"
"$CLI" work release --store "$DB" --lease "$LEASE" --reason smoke --requeue

go test ./internal/store -run TestClaimWorkItemIsAtomic -count=1

echo "work-queue-fixture-smoke: ok"
