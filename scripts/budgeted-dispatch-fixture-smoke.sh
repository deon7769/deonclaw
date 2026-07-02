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
ARTIFACTS="$TMP/artifacts"
REGISTRY="$TMP/skills-registry"
SKILL_POLICY="configs/examples/skill-policy.yaml"
INSIGHT_POLICY="configs/examples/insight-policy.yaml"
REVIEWER_RESPONSE="configs/examples/insight-reviewer-response-fixture.json"

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

echo "==> assign and accept (dispatch auto-claims lease)"
ASSIGN_OUT="$("$CLI" agents assign --agent backend-engineer --task "$TASK" --store "$DB" --created-by smoke)"
echo "$ASSIGN_OUT"
WORK_ITEM="$(echo "$ASSIGN_OUT" | sed -n 's/.*work_item=\([^ ]*\).*/\1/p')"
INBOX="$(echo "$ASSIGN_OUT" | sed -n 's/.*inbox=\([^ ]*\).*/\1/p')"
"$CLI" agents inbox accept "$INBOX" --store "$DB"

echo "==> dispatch-once fake (auto-claim, budget, evidence, review)"
DISPATCH_JSON="$("$CLI" work dispatch-once \
  --store "$DB" \
  --work-item "$WORK_ITEM" \
  --mode fake \
  --artifacts-dir "$ARTIFACTS" \
  --registry-root "$REGISTRY" \
  --skill-policy "$SKILL_POLICY")"
echo "$DISPATCH_JSON"
echo "$DISPATCH_JSON" | grep -q '"status": "ok"' || { echo "dispatch status not ok" >&2; exit 1; }
echo "$DISPATCH_JSON" | grep -q '"lease_acquired": true' || { echo "lease not acquired" >&2; exit 1; }
echo "$DISPATCH_JSON" | grep -q '"worker_started": true' || { echo "worker not started" >&2; exit 1; }
echo "$DISPATCH_JSON" | grep -q '"budget_reserved": true' || { echo "budget not reserved" >&2; exit 1; }
echo "$DISPATCH_JSON" | grep -q '"budget_committed": true' || { echo "budget not committed" >&2; exit 1; }
echo "$DISPATCH_JSON" | grep -q '"lease_released": true' || { echo "lease not released" >&2; exit 1; }
echo "$DISPATCH_JSON" | grep -q '"evidence_bundle_created": true' || { echo "evidence not created" >&2; exit 1; }
echo "$DISPATCH_JSON" | grep -q '"review_work_queued": true' || { echo "review not queued" >&2; exit 1; }
echo "$DISPATCH_JSON" | grep -q '"provider_call": false' || { echo "provider_call not false" >&2; exit 1; }

REVIEW_WORK="$(echo "$DISPATCH_JSON" | sed -n 's/.*"work_item_id": "\(work_insight_[^"]*\)".*/\1/p' | head -1)"
if [[ -z "$REVIEW_WORK" ]]; then
  echo "review work item id not found in dispatch output" >&2
  exit 1
fi

echo "==> dispatch insight review work (fake)"
REVIEW_JSON="$("$CLI" work dispatch-once \
  --store "$DB" \
  --work-item "$REVIEW_WORK" \
  --mode fake \
  --artifacts-dir "$ARTIFACTS/review" \
  --registry-root "$REGISTRY" \
  --skill-policy "$SKILL_POLICY" \
  --insight-policy "$INSIGHT_POLICY" \
  --reviewer-response "$REVIEWER_RESPONSE" \
  --learning-approval-decision approved \
  --learning-approval-reason "Operator approved 23.22 fixture proposal." \
  --learning-approval-reviewer opencode \
  --learning-confirm-apply)"
echo "$REVIEW_JSON"
echo "$REVIEW_JSON" | grep -q '"status": "ok"' || { echo "review dispatch status not ok" >&2; exit 1; }
echo "$REVIEW_JSON" | grep -q '"worker_started": true' || { echo "review worker not started" >&2; exit 1; }
echo "$REVIEW_JSON" | grep -q '"learning_loop": {' || { echo "learning loop result missing" >&2; exit 1; }
echo "$REVIEW_JSON" | grep -q '"materialized": true' || { echo "learning loop not materialized" >&2; exit 1; }
echo "$REVIEW_JSON" | grep -q '"proposal_count": 1' || { echo "learning proposal not materialized" >&2; exit 1; }
echo "$REVIEW_JSON" | grep -q '"approval_count": 1' || { echo "learning approval not materialized" >&2; exit 1; }
echo "$REVIEW_JSON" | grep -q '"apply_count": 1' || { echo "learning apply not materialized" >&2; exit 1; }
APPROVAL_REVIEWER="$(REVIEW_JSON="$REVIEW_JSON" python3 - <<'PY'
import json
import os
from pathlib import Path

doc = json.loads(os.environ["REVIEW_JSON"])
loop = doc.get("learning_loop", {})
approval_paths = loop.get("approval_paths", [])
apply_result_paths = loop.get("apply_result_paths", [])
apply_preview_paths = loop.get("apply_preview_paths", [])
if len(approval_paths) != 1:
    raise SystemExit("approval path missing")
if len(apply_result_paths) != 1 or len(apply_preview_paths) != 1:
    raise SystemExit("apply paths missing")
approval = json.loads(Path(approval_paths[0]).read_text())
apply_result = json.loads(Path(apply_result_paths[0]).read_text())
if not apply_result.get("executed"):
    raise SystemExit("apply result not executed")
if apply_result.get("applied_artifact") != apply_preview_paths[0]:
    raise SystemExit("apply preview path mismatch")
print(approval.get("reviewer", ""))
PY
)"
[[ "$APPROVAL_REVIEWER" == "opencode" ]] || { echo "learning approval reviewer not materialized" >&2; exit 1; }

echo "==> package tests"
go test ./internal/dispatch -run 'TestDispatchOnce' -count=1

echo "budgeted-dispatch-fixture-smoke: ok"
