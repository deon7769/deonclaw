# Budgeted dispatch fixture

CI-only fake dispatch smoke for Epic 23E (`budgeted-dispatch-smoke`), including **23.19.1** hardening checks and the **23.20-23.23** closed-learning-loop fixture legs.

## What it validates

- pricing/budget/work-template config validate + sync
- work assign + inbox accept
- fake `work dispatch-once` **auto-claims lease** (no `--lease` required)
- mandatory budget reservation/commit before and after worker
- skill snapshot materialization (empty registry OK)
- evidence bundle artifact + insight review queue
- fake `insight_review` dispatch materializes `insight-report.json`, `learning-proposals.json`, governed `learning-approval-*.json`, governed `learning-apply-preview-*.md` / `learning-apply-result-*.json`, `learning-effectiveness-*.json`, `learning-session-refresh.json`, and `learning-session-snapshot-*.json` artifacts from explicit reviewer/approval/apply fixtures
- `provider_call: false`, `network_call: false`

## Commands

```bash
deonctl pricing validate --config ../model-prices.yaml
deonctl budgets validate --config ../budgets.yaml
deonctl work-templates validate --config ../work-templates.yaml

deonctl pricing sync --config ../model-prices.yaml --store <db>
deonctl budgets sync --config ../budgets.yaml --store <db>
deonctl work-templates sync --config ../work-templates.yaml --store <db>
deonctl agents sync --config ../agents.yaml --store <db> --workers-config ../workers.yaml

deonctl work dispatch-once \
  --store <db> \
  --work-item <id> \
  --mode fake \
  --artifacts-dir <tmpdir>/artifacts \
  --registry-root <tmpdir>/skills-registry \
  --skill-policy ../skill-policy.yaml

# For the queued insight_review work item:
deonctl work dispatch-once \
  --store <db> \
  --work-item <review-work-id> \
  --mode fake \
  --artifacts-dir <tmpdir>/artifacts/review \
  --registry-root <tmpdir>/skills-registry \
  --skill-policy ../skill-policy.yaml \
  --insight-policy ../insight-policy.yaml \
  --reviewer-response ../insight-reviewer-response-fixture.json \
  --learning-approval-decision approved \
  --learning-approval-reason "Operator approved 23.23 fixture proposal." \
  --learning-approval-reviewer opencode \
  --learning-confirm-apply
```

`--mode real` is blocked in this sprint.

See `scripts/budgeted-dispatch-fixture-smoke.sh` for the executable path.
