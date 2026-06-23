# Budgeted dispatch fixture

CI-only fake dispatch smoke for Epic 23E (`budgeted-dispatch-smoke`), including **23.19.1** hardening checks.

## What it validates

- pricing/budget/work-template config validate + sync
- work assign + inbox accept
- fake `work dispatch-once` **auto-claims lease** (no `--lease` required)
- mandatory budget reservation/commit before and after worker
- skill snapshot materialization (empty registry OK)
- evidence bundle artifact + insight review queue
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
```

`--mode real` is blocked in this sprint.

See `scripts/budgeted-dispatch-fixture-smoke.sh` for the executable path.
