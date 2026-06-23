# Budgeted dispatch fixture

CI-only fake dispatch smoke for Epic 23E (`budgeted-dispatch-smoke`).

## What it validates

- pricing/budget/work-template config validate + sync
- work item materialization path inputs (task snapshot + queued work)
- fake `work dispatch-once` success without `provider_call` or `network_call`
- budget reservation/commit on successful fake run

## Commands

```bash
deonctl pricing validate --config ../model-prices.yaml
deonctl budgets validate --config ../budgets.yaml
deonctl work-templates validate --config ../work-templates.yaml

deonctl pricing sync --config ../model-prices.yaml --store <db>
deonctl budgets sync --config ../budgets.yaml --store <db>
deonctl work-templates sync --config ../work-templates.yaml --store <db>
deonctl agents sync --config ../agents.yaml --store <db> --workers-config ../workers.yaml

deonctl work dispatch-once --store <db> --work-item <id> --lease <lease-id> --mode fake
```

See `scripts/budgeted-dispatch-fixture-smoke.sh` for the executable path.
