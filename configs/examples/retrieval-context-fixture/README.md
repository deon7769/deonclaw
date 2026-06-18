# Retrieval context governance E2E fixture

Small reproducible fixture for the retrieval-context governance chain (Tasks 22.8–22.12.1).

This fixture does **not** run LanceDB search, call embedding providers, or inject materialized text into runner prompts.

## Contents

- `retrieval-context.json` — passive metadata-only retrieval artifact (one hit)
- `memory-index-chunks.jsonl` — governed chunk text source for materialize
- `retrieval-injection-policy.yaml` — plan-only injection policy over generated artifacts (Task 22.15.1)

Generated artifacts (created by the steps below) stay in this directory when commands use `--output` paths here.

## Prerequisites

- Built `deonctl` on `PATH`, or run via `go run ./cmd/deonctl` from the repository root
- Run commands from this directory:

~~~bash
cd configs/examples/retrieval-context-fixture
~~~

## Step-by-step commands

### 1. Inspect retrieval context

~~~bash
deonctl retrieval context inspect \
  --artifact retrieval-context.json
~~~

### 2. Materialize governed chunk text

~~~bash
deonctl retrieval context materialize \
  --retrieval-context retrieval-context.json \
  --chunks memory-index-chunks.jsonl \
  --output retrieval-context-materialized.json \
  --summary retrieval-context-materialized.md \
  --max-chars-per-chunk 1200 \
  --max-total-chars 6000 \
  --confirm-include-chunk-text
~~~

### 3. Materialized report QA

~~~bash
deonctl retrieval context materialized-report \
  --artifact retrieval-context-materialized.json
~~~

### 4. Audit bundle

~~~bash
deonctl retrieval context bundle \
  --retrieval-context retrieval-context.json \
  --materialized retrieval-context-materialized.json \
  --output retrieval-context-bundle.json \
  --summary retrieval-context-bundle.md
~~~

### 5. Approval request

~~~bash
deonctl retrieval context approval new \
  --bundle retrieval-context-bundle.json \
  --output retrieval-context-approval-request.json
~~~

### 6. Approve materialized context

~~~bash
deonctl retrieval context approval approve \
  --request retrieval-context-approval-request.json \
  --output retrieval-context-approval.json \
  --confirm-approve-materialized-context
~~~

### 7. Inspect approval

~~~bash
deonctl retrieval context approval inspect \
  --approval retrieval-context-approval.json
~~~

### 8. Injection plan (no runner execution)

~~~bash
deonctl retrieval context injection-plan \
  --approval retrieval-context-approval.json \
  --request retrieval-context-approval-request.json \
  --bundle retrieval-context-bundle.json \
  --materialized retrieval-context-materialized.json \
  --output retrieval-context-injection-plan.json \
  --summary retrieval-context-injection-plan.md
~~~

### 9. Governance report

~~~bash
deonctl retrieval context governance-report \
  --retrieval-context retrieval-context.json \
  --materialized retrieval-context-materialized.json \
  --bundle retrieval-context-bundle.json \
  --request retrieval-context-approval-request.json \
  --approval retrieval-context-approval.json \
  --injection-plan retrieval-context-injection-plan.json
~~~

### 10. Injection policy validate (schema only)

~~~bash
deonctl retrieval context injection-policy validate \
  --policy retrieval-injection-policy.yaml
~~~

### 11. Injection policy plan (no execution)

~~~bash
deonctl retrieval context injection-policy plan \
  --policy retrieval-injection-policy.yaml \
  --output-format json
~~~

## Expected outcome

- inspect, materialized-report, bundle, approval inspect, injection-plan, and governance-report return `status: ok`
- injection-policy validate returns ok after steps 1–9 generated the artifacts
- injection-policy plan returns `would_inject: false`, `reason: schema_only_no_execution`, and `status: warning` while approval still has `runner_injection_allowed: false`
- `can_inject_now: false` in injection-plan and governance-report
- `required_future_flag: --confirm-inject-materialized-context` in injection-plan
- only `retrieval-context-materialized.json` / `.md` contain chunk text excerpts; other artifacts must not include `text_excerpt`

## Safety

- metadata-only runner attachment remains separate from this fixture workflow
- no memory apply/restore changes
- no LanceDB search or provider API calls
