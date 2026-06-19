# Retrieval context governance E2E fixture

Small reproducible fixture for the retrieval-context governance chain (Tasks 22.8–22.12.1).

This fixture does **not** run LanceDB search, call embedding providers, or inject materialized text into runner prompts.

## Contents

- `retrieval-context.json` — passive metadata-only retrieval artifact (one hit)
- `memory-index-chunks.jsonl` — governed chunk text source for materialize
- `retrieval-injection-policy.yaml` — plan-only injection policy over generated artifacts (Task 22.15.1)
- `materialized-injection-task.yaml` — schema-only materialized injection task declaration (Task 22.19)
- `base-worker-prompt-fixture.md` — dry-run base worker prompt fixture for materialized prompt assembly (Task 22.23)
- `../provider-call-executor.yaml` — executor policy config for provider-call-executor validate/plan (Task 22.39)

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

## Optional: runner injection approval (Task 22.16)

These steps require saving the governance report JSON from step 9 first:

~~~bash
deonctl retrieval context governance-report \
  --retrieval-context retrieval-context.json \
  --materialized retrieval-context-materialized.json \
  --bundle retrieval-context-bundle.json \
  --request retrieval-context-approval-request.json \
  --approval retrieval-context-approval.json \
  --injection-plan retrieval-context-injection-plan.json \
  --output-format json > retrieval-context-governance-report.json
~~~

### 12. Injection approval request

~~~bash
deonctl retrieval context injection-approval new \
  --governance-report retrieval-context-governance-report.json \
  --policy retrieval-injection-policy.yaml \
  --output retrieval-context-injection-approval-request.json
~~~

### 13. Approve runner injection (artifact only)

~~~bash
deonctl retrieval context injection-approval approve \
  --request retrieval-context-injection-approval-request.json \
  --output retrieval-context-injection-approval.json \
  --confirm-allow-runner-injection
~~~

### 14. Inspect injection approval

~~~bash
deonctl retrieval context injection-approval inspect \
  --approval retrieval-context-injection-approval.json \
  --request retrieval-context-injection-approval-request.json \
  --output-format json
~~~

### 15. Injection execution plan (no worker execution)

~~~bash
deonctl retrieval context injection-execution-plan \
  --policy retrieval-injection-policy.yaml \
  --governance-report retrieval-context-governance-report.json \
  --injection-approval retrieval-context-injection-approval.json \
  --injection-approval-request retrieval-context-injection-approval-request.json \
  --materialized retrieval-context-materialized.json \
  --output retrieval-context-injection-execution-plan.json \
  --summary retrieval-context-injection-execution-plan.md
~~~

### 16. Prompt preview (no worker execution)

~~~bash
deonctl retrieval context prompt-preview \
  --execution-plan retrieval-context-injection-execution-plan.json \
  --policy retrieval-injection-policy.yaml \
  --materialized retrieval-context-materialized.json \
  --output retrieval-context-prompt-preview.md \
  --manifest retrieval-context-prompt-preview.json \
  --confirm-render-materialized-context
~~~

### 17. Prompt preview report (QA only)

~~~bash
deonctl retrieval context prompt-preview-report \
  --preview retrieval-context-prompt-preview.md \
  --manifest retrieval-context-prompt-preview.json \
  --execution-plan retrieval-context-injection-execution-plan.json \
  --output-format json
~~~

Save the report JSON from step 17 before running step 18:

~~~bash
deonctl retrieval context prompt-preview-report \
  --preview retrieval-context-prompt-preview.md \
  --manifest retrieval-context-prompt-preview.json \
  --execution-plan retrieval-context-injection-execution-plan.json \
  --output-format json > retrieval-context-prompt-preview-report.json
~~~

### 18. Injection governance release bundle (no worker execution)

~~~bash
deonctl retrieval context injection-governance-bundle \
  --governance-report retrieval-context-governance-report.json \
  --policy retrieval-injection-policy.yaml \
  --injection-approval-request retrieval-context-injection-approval-request.json \
  --injection-approval retrieval-context-injection-approval.json \
  --execution-plan retrieval-context-injection-execution-plan.json \
  --prompt-preview-manifest retrieval-context-prompt-preview.json \
  --prompt-preview-report retrieval-context-prompt-preview-report.json \
  --output retrieval-context-injection-governance-bundle.json \
  --summary retrieval-context-injection-governance-bundle.md
~~~

## Optional: task declaration (Task 22.19)

After step 18 produces `retrieval-context-injection-governance-bundle.json`, validate the example task declaration:

~~~bash
deonctl task validate materialized-injection-task.yaml
~~~

Expected output includes:

- `task retrieval-context-materialized-injection-001: valid`
- `materialized_injection_declared: true`
- `materialized_injection_enabled: false`
- `materialized_injection_supported_now: false`
- `governance_bundle_sha256: ...`

### 19. Materialized injection task report (QA only)

~~~bash
deonctl task materialized-injection-report \
  --task materialized-injection-task.yaml \
  --output-format json
~~~

### 20. Runner materialized injection preflight (no worker execution)

~~~bash
deonctl worker codex materialized-injection-preflight \
  --task materialized-injection-task.yaml \
  --prompt-preview-report retrieval-context-prompt-preview-report.json \
  --output retrieval-context-materialized-injection-preflight.json \
  --output-format json
~~~

### 21. Runner materialized injection dry-run (no worker execution)

~~~bash
deonctl worker codex materialized-injection-dry-run \
  --task materialized-injection-task.yaml \
  --preflight retrieval-context-materialized-injection-preflight.json \
  --prompt-preview retrieval-context-prompt-preview.md \
  --output retrieval-context-materialized-injection-dry-run.json \
  --prompt-output retrieval-context-materialized-injection-prompt-section.md \
  --confirm-inject-materialized-context \
  --output-format json
~~~

### 22. Runner materialized injection dry-run report (no worker execution)

~~~bash
deonctl worker codex materialized-injection-dry-run-report \
  --dry-run retrieval-context-materialized-injection-dry-run.json \
  --prompt-output retrieval-context-materialized-injection-prompt-section.md \
  --output-format json > retrieval-context-materialized-injection-dry-run-report.json
~~~

### 23. Runner materialized injection readiness report (no worker execution)

~~~bash
deonctl worker codex materialized-injection-readiness-report \
  --preflight retrieval-context-materialized-injection-preflight.json \
  --dry-run retrieval-context-materialized-injection-dry-run.json \
  --dry-run-report retrieval-context-materialized-injection-dry-run-report.json \
  --output-format json > retrieval-context-materialized-injection-readiness-report.json
~~~

### 24. Materialized injection execution gate

~~~bash
deonctl worker codex materialized-injection-execution-gate \
  --task materialized-injection-task.yaml \
  --readiness-report retrieval-context-materialized-injection-readiness-report.json \
  --prompt-output retrieval-context-materialized-injection-prompt-section.md \
  --confirm-inject-materialized-context \
  --output-format json > retrieval-context-materialized-injection-execution-gate.json
~~~

### 25. Materialized prompt assembly dry-run

~~~bash
deonctl worker codex materialized-prompt-assembly-dry-run \
  --execution-gate retrieval-context-materialized-injection-execution-gate.json \
  --base-prompt-fixture base-worker-prompt-fixture.md \
  --prompt-output retrieval-context-materialized-injection-prompt-section.md \
  --output retrieval-context-materialized-prompt-assembly-dry-run.json \
  --assembled-output retrieval-context-materialized-prompt-assembly.md \
  --confirm-inject-materialized-context \
  --output-format json
~~~

### 26. Materialized prompt assembly report

~~~bash
deonctl worker codex materialized-prompt-assembly-report \
  --assembly-dry-run retrieval-context-materialized-prompt-assembly-dry-run.json \
  --assembled-output retrieval-context-materialized-prompt-assembly.md \
  --output-format json > retrieval-context-materialized-prompt-assembly-report.json
~~~

### 27. Materialized injection runtime config validate

~~~bash
deonctl worker codex materialized-injection-runtime validate \
  --config ../materialized-injection-runtime.yaml \
  --output-format json
~~~

### 28. Materialized injection run planner (no worker execution)

~~~bash
deonctl worker codex materialized-injection-run-plan \
  --task materialized-injection-task.yaml \
  --runtime-config ../materialized-injection-runtime.yaml \
  --execution-gate retrieval-context-materialized-injection-execution-gate.json \
  --assembly-report retrieval-context-materialized-prompt-assembly-report.json \
  --assembled-output retrieval-context-materialized-prompt-assembly.md \
  --output retrieval-context-materialized-injection-run-plan.json \
  --confirm-inject-materialized-context \
  --output-format json
~~~

### 29. Materialized injection execution enablement policy validate

~~~bash
deonctl worker codex materialized-injection-execution-enable validate \
  --config ../materialized-injection-execution-enable.yaml \
  --output-format json
~~~

### 30. Materialized injection provider run-plan (no provider call)

~~~bash
deonctl worker codex materialized-injection-provider-run-plan \
  --task materialized-injection-task.yaml \
  --enable-config ../materialized-injection-execution-enable.yaml \
  --execution-gate retrieval-context-materialized-injection-execution-gate.json \
  --assembly-report retrieval-context-materialized-prompt-assembly-report.json \
  --assembled-output retrieval-context-materialized-prompt-assembly.md \
  --output retrieval-context-materialized-injection-provider-run-plan.json \
  --confirm-inject-materialized-context \
  --output-format json
~~~

### 31. Materialized provider dispatch validate

~~~bash
deonctl worker codex materialized-provider-dispatch validate \
  --config ../materialized-provider-dispatch.yaml \
  --output-format json
~~~

### 32. Materialized provider payload dry-run (no provider call)

~~~bash
deonctl worker codex materialized-provider-payload-dry-run \
  --dispatch-config ../materialized-provider-dispatch.yaml \
  --provider-run-plan retrieval-context-materialized-injection-provider-run-plan.json \
  --assembled-output retrieval-context-materialized-prompt-assembly.md \
  --output retrieval-context-materialized-provider-payload-dry-run.json \
  --payload-output retrieval-context-materialized-provider-payload.md \
  --confirm-inject-materialized-context \
  --output-format json
~~~

### 33. Materialized provider payload report

~~~bash
deonctl worker codex materialized-provider-payload-report \
  --payload-dry-run retrieval-context-materialized-provider-payload-dry-run.json \
  --payload-output retrieval-context-materialized-provider-payload.md \
  --output-format json > retrieval-context-materialized-provider-payload-report.json
~~~

### 34. Materialized provider call gate (no provider call)

~~~bash
deonctl worker codex materialized-provider-call-gate \
  --dispatch-config ../materialized-provider-dispatch.yaml \
  --payload-report retrieval-context-materialized-provider-payload-report.json \
  --payload-output retrieval-context-materialized-provider-payload.md \
  --confirm-inject-materialized-context \
  --output retrieval-context-materialized-provider-call-gate.json \
  --output-format json
~~~

### 35. Materialized provider call readiness report

~~~bash
deonctl worker codex materialized-provider-call-readiness-report \
  --provider-call-gate retrieval-context-materialized-provider-call-gate.json \
  --payload-report retrieval-context-materialized-provider-payload-report.json \
  --output-format json > retrieval-context-materialized-provider-call-readiness-report.json
~~~

### 36. Provider call approval (no provider call)

~~~bash
deonctl worker codex provider-call-approval new \
  --readiness-report retrieval-context-materialized-provider-call-readiness-report.json \
  --provider-call-gate retrieval-context-materialized-provider-call-gate.json \
  --payload-report retrieval-context-materialized-provider-payload-report.json \
  --output retrieval-context-provider-call-approval-request.json

deonctl worker codex provider-call-approval approve \
  --request retrieval-context-provider-call-approval-request.json \
  --output retrieval-context-provider-call-approval.json \
  --confirm-provider-payload-sha256 <sha256-from-payload-report>

deonctl worker codex provider-call-approval inspect \
  --approval retrieval-context-provider-call-approval.json \
  --request retrieval-context-provider-call-approval-request.json \
  --output-format json
~~~

### 37. Provider call execution bundle (no provider call)

~~~bash
deonctl worker codex provider-call-execution-bundle \
  --dispatch-config ../materialized-provider-dispatch.yaml \
  --payload-report retrieval-context-materialized-provider-payload-report.json \
  --provider-call-gate retrieval-context-materialized-provider-call-gate.json \
  --readiness-report retrieval-context-materialized-provider-call-readiness-report.json \
  --approval retrieval-context-provider-call-approval.json \
  --output retrieval-context-provider-call-execution-bundle.json \
  --output-format json
~~~

### 38. Provider call chain continuity audit

~~~bash
deonctl worker codex provider-call-chain-audit \
  --dispatch-config ../materialized-provider-dispatch.yaml \
  --provider-run-plan retrieval-context-materialized-injection-provider-run-plan.json \
  --payload-dry-run retrieval-context-materialized-provider-payload-dry-run.json \
  --payload-output retrieval-context-materialized-provider-payload.md \
  --payload-report retrieval-context-materialized-provider-payload-report.json \
  --provider-call-gate retrieval-context-materialized-provider-call-gate.json \
  --readiness-report retrieval-context-materialized-provider-call-readiness-report.json \
  --approval-request retrieval-context-provider-call-approval-request.json \
  --approval retrieval-context-provider-call-approval.json \
  --execution-bundle retrieval-context-provider-call-execution-bundle.json \
  --output-format json
~~~

### 39. Provider call executor policy config validate

~~~bash
deonctl worker codex provider-call-executor config validate \
  --config ../provider-call-executor.yaml \
  --output-format json
~~~

### 40. Provider call executor validate / plan (no provider call)

~~~bash
deonctl worker codex provider-call-executor validate \
  --executor-config ../provider-call-executor.yaml \
  --dispatch-config ../materialized-provider-dispatch.yaml \
  --provider-run-plan retrieval-context-materialized-injection-provider-run-plan.json \
  --payload-dry-run retrieval-context-materialized-provider-payload-dry-run.json \
  --payload-output retrieval-context-materialized-provider-payload.md \
  --payload-report retrieval-context-materialized-provider-payload-report.json \
  --provider-call-gate retrieval-context-materialized-provider-call-gate.json \
  --readiness-report retrieval-context-materialized-provider-call-readiness-report.json \
  --approval-request retrieval-context-provider-call-approval-request.json \
  --approval retrieval-context-provider-call-approval.json \
  --execution-bundle retrieval-context-provider-call-execution-bundle.json \
  --output-format json

deonctl worker codex provider-call-executor plan \
  --executor-config ../provider-call-executor.yaml \
  --dispatch-config ../materialized-provider-dispatch.yaml \
  --provider-run-plan retrieval-context-materialized-injection-provider-run-plan.json \
  --payload-dry-run retrieval-context-materialized-provider-payload-dry-run.json \
  --payload-output retrieval-context-materialized-provider-payload.md \
  --payload-report retrieval-context-materialized-provider-payload-report.json \
  --provider-call-gate retrieval-context-materialized-provider-call-gate.json \
  --readiness-report retrieval-context-materialized-provider-call-readiness-report.json \
  --approval-request retrieval-context-provider-call-approval-request.json \
  --approval retrieval-context-provider-call-approval.json \
  --execution-bundle retrieval-context-provider-call-execution-bundle.json \
  --output retrieval-context-provider-call-executor-plan.json \
  --output-format json
~~~

### 41. Provider call executor dry-run (no provider call)

~~~bash
deonctl worker codex provider-call-executor dry-run \
  --executor-config ../provider-call-executor.yaml \
  --dispatch-config ../materialized-provider-dispatch.yaml \
  --provider-run-plan retrieval-context-materialized-injection-provider-run-plan.json \
  --payload-dry-run retrieval-context-materialized-provider-payload-dry-run.json \
  --payload-report retrieval-context-materialized-provider-payload-report.json \
  --provider-call-gate retrieval-context-materialized-provider-call-gate.json \
  --readiness-report retrieval-context-materialized-provider-call-readiness-report.json \
  --approval-request retrieval-context-provider-call-approval-request.json \
  --approval retrieval-context-provider-call-approval.json \
  --execution-bundle retrieval-context-provider-call-execution-bundle.json \
  --output retrieval-context-provider-call-executor-dry-run.json \
  --output-format json
~~~

### 42. Provider call executor dry-run report

~~~bash
deonctl worker codex provider-call-executor dry-run-report \
  --dry-run retrieval-context-provider-call-executor-dry-run.json \
  --executor-config ../provider-call-executor.yaml \
  --output-format json
~~~

### 43. Provider call executor preflight (no provider call)

~~~bash
deonctl worker codex provider-call-executor preflight \
  --executor-config ../provider-call-executor.yaml \
  --dry-run retrieval-context-provider-call-executor-dry-run.json \
  --dry-run-report retrieval-context-provider-call-executor-dry-run-report.json \
  --execution-bundle retrieval-context-provider-call-execution-bundle.json \
  --approval retrieval-context-provider-call-approval.json \
  --output retrieval-context-provider-call-executor-preflight.json \
  --output-format json
~~~

Save dry-run report first (step 42 stdout → file) as `retrieval-context-provider-call-executor-dry-run-report.json`.

### 44. Provider executor dispatch approval request

~~~bash
deonctl worker codex provider-call-executor-dispatch-approval new \
  --executor-config ../provider-call-executor.yaml \
  --dry-run retrieval-context-provider-call-executor-dry-run.json \
  --dry-run-report retrieval-context-provider-call-executor-dry-run-report.json \
  --preflight retrieval-context-provider-call-executor-preflight.json \
  --execution-bundle retrieval-context-provider-call-execution-bundle.json \
  --approval retrieval-context-provider-call-approval.json \
  --output retrieval-context-provider-call-executor-dispatch-approval-request.json
~~~

### 45. Provider executor dispatch approval approve

~~~bash
deonctl worker codex provider-call-executor-dispatch-approval approve \
  --request retrieval-context-provider-call-executor-dispatch-approval-request.json \
  --output retrieval-context-provider-call-executor-dispatch-approval.json \
  --confirm-executor-config-sha256 <sha256> \
  --confirm-execution-bundle-sha256 <sha256> \
  --confirm-provider-payload-sha256 <sha256>
~~~

Use hashes from the preflight artifact (step 43).

### 46. Provider executor dispatch approval inspect

~~~bash
deonctl worker codex provider-call-executor-dispatch-approval inspect \
  --approval retrieval-context-provider-call-executor-dispatch-approval.json \
  --request retrieval-context-provider-call-executor-dispatch-approval-request.json \
  --output-format json
~~~

### 47. Provider transport plan (blocked)

~~~bash
deonctl worker codex provider-transport-plan \
  --executor-config ../provider-call-executor.yaml \
  --executor-preflight retrieval-context-provider-call-executor-preflight.json \
  --dispatch-approval retrieval-context-provider-call-executor-dispatch-approval.json \
  --output retrieval-context-provider-transport-plan.json \
  --output-format json
~~~

### 48. Provider executor release bundle

~~~bash
deonctl worker codex provider-executor-release-bundle \
  --executor-config ../provider-call-executor.yaml \
  --execution-bundle retrieval-context-provider-call-execution-bundle.json \
  --dry-run retrieval-context-provider-call-executor-dry-run.json \
  --dry-run-report retrieval-context-provider-call-executor-dry-run-report.json \
  --executor-preflight retrieval-context-provider-call-executor-preflight.json \
  --dispatch-approval retrieval-context-provider-call-executor-dispatch-approval.json \
  --transport-plan retrieval-context-provider-transport-plan.json \
  --output retrieval-context-provider-executor-release-bundle.json \
  --output-format json
~~~

### 49. Provider executor release gate

~~~bash
deonctl worker codex provider-executor-release-gate \
  --executor-config ../provider-call-executor.yaml \
  --execution-bundle retrieval-context-provider-call-execution-bundle.json \
  --dry-run retrieval-context-provider-call-executor-dry-run.json \
  --dry-run-report retrieval-context-provider-call-executor-dry-run-report.json \
  --executor-preflight retrieval-context-provider-call-executor-preflight.json \
  --dispatch-approval retrieval-context-provider-call-executor-dispatch-approval.json \
  --transport-plan retrieval-context-provider-transport-plan.json \
  --release-bundle retrieval-context-provider-executor-release-bundle.json \
  --output-format json
~~~

### 50. Smoke / audit final

~~~bash
make provider-call-chain-smoke
~~~

Re-runs library + CLI fixture e2e (steps 29–58 equivalent) and asserts simulation `execution_result_available: false` with all execution flags blocked.

### 51. Provider request envelope dry-run

~~~bash
deonctl worker codex provider-request-envelope dry-run \
  --executor-config ../provider-call-executor.yaml \
  --execution-bundle retrieval-context-provider-call-execution-bundle.json \
  --executor-preflight retrieval-context-provider-call-executor-preflight.json \
  --dispatch-approval retrieval-context-provider-call-executor-dispatch-approval.json \
  --transport-plan retrieval-context-provider-transport-plan.json \
  --release-bundle retrieval-context-provider-executor-release-bundle.json \
  --release-gate retrieval-context-provider-executor-release-gate.json \
  --output retrieval-context-provider-request-envelope.json \
  --output-format json
~~~

Save release gate stdout from step 49 as `retrieval-context-provider-executor-release-gate.json`.

### 52. Provider request envelope report

~~~bash
deonctl worker codex provider-request-envelope report \
  --request-envelope retrieval-context-provider-request-envelope.json \
  --executor-config ../provider-call-executor.yaml \
  --execution-bundle retrieval-context-provider-call-execution-bundle.json \
  --executor-preflight retrieval-context-provider-call-executor-preflight.json \
  --dispatch-approval retrieval-context-provider-call-executor-dispatch-approval.json \
  --transport-plan retrieval-context-provider-transport-plan.json \
  --release-bundle retrieval-context-provider-executor-release-bundle.json \
  --release-gate retrieval-context-provider-executor-release-gate.json \
  --output-format json
~~~

### 53. Provider adapter registry validate

~~~bash
deonctl worker codex provider-adapter-registry validate \
  --config ../provider-adapters.yaml \
  --output-format json
~~~

### 54. Provider adapter plan

~~~bash
deonctl worker codex provider-adapter-plan \
  --config ../provider-adapters.yaml \
  --request-envelope retrieval-context-provider-request-envelope.json \
  --output retrieval-context-provider-adapter-plan.json \
  --output-format json
~~~

### 55. Provider response fixture generate

~~~bash
deonctl worker codex provider-response-fixture generate \
  --request-envelope retrieval-context-provider-request-envelope.json \
  --adapter-plan retrieval-context-provider-adapter-plan.json \
  --output retrieval-context-provider-response-fixture.json \
  --output-format json
~~~

### 56. Provider response fixture inspect

~~~bash
deonctl worker codex provider-response-fixture inspect \
  --fixture retrieval-context-provider-response-fixture.json \
  --request-envelope retrieval-context-provider-request-envelope.json \
  --adapter-plan retrieval-context-provider-adapter-plan.json \
  --output-format json
~~~

### 57. Provider execution simulation bundle

~~~bash
deonctl worker codex provider-execution-simulation-bundle \
  --request-envelope retrieval-context-provider-request-envelope.json \
  --adapter-plan retrieval-context-provider-adapter-plan.json \
  --response-fixture retrieval-context-provider-response-fixture.json \
  --release-gate retrieval-context-provider-executor-release-gate.json \
  --executor-config ../provider-call-executor.yaml \
  --output retrieval-context-provider-execution-simulation-bundle.json \
  --output-format json
~~~

### 58. Provider execution simulation report

~~~bash
deonctl worker codex provider-execution-simulation-report \
  --simulation-bundle retrieval-context-provider-execution-simulation-bundle.json \
  --request-envelope retrieval-context-provider-request-envelope.json \
  --adapter-plan retrieval-context-provider-adapter-plan.json \
  --response-fixture retrieval-context-provider-response-fixture.json \
  --release-gate retrieval-context-provider-executor-release-gate.json \
  --executor-config ../provider-call-executor.yaml \
  --output-format json > retrieval-context-provider-execution-simulation-report.json
~~~

### 59. Provider credential policy validate

~~~bash
deonctl worker codex provider-credential-policy validate \
  --config ../provider-credential-policy.yaml \
  --output-format json
~~~

### 60. Provider credential policy plan

~~~bash
deonctl worker codex provider-credential-policy plan \
  --config ../provider-credential-policy.yaml \
  --output retrieval-context-provider-credential-policy-plan.json \
  --output-format json
~~~

### 61. Provider real-call proposal new

~~~bash
deonctl worker codex provider-real-call-proposal new \
  --execution-simulation-report retrieval-context-provider-execution-simulation-report.json \
  --request-envelope retrieval-context-provider-request-envelope.json \
  --adapter-plan retrieval-context-provider-adapter-plan.json \
  --credential-policy-plan retrieval-context-provider-credential-policy-plan.json \
  --release-gate retrieval-context-provider-executor-release-gate.json \
  --output retrieval-context-provider-real-call-proposal.json
~~~

### 62. Provider real-call proposal inspect

~~~bash
deonctl worker codex provider-real-call-proposal inspect \
  --proposal retrieval-context-provider-real-call-proposal.json \
  --execution-simulation-report retrieval-context-provider-execution-simulation-report.json \
  --request-envelope retrieval-context-provider-request-envelope.json \
  --adapter-plan retrieval-context-provider-adapter-plan.json \
  --credential-policy-plan retrieval-context-provider-credential-policy-plan.json \
  --release-gate retrieval-context-provider-executor-release-gate.json \
  --output-format json
~~~

### 63. Provider response change proposal

~~~bash
deonctl worker codex provider-response-change-proposal \
  --response-fixture retrieval-context-provider-response-fixture.json \
  --execution-simulation-report retrieval-context-provider-execution-simulation-report.json \
  --real-call-proposal retrieval-context-provider-real-call-proposal.json \
  --output retrieval-context-provider-response-change-proposal.json \
  --output-format json
~~~

### 64. Provider response change proposal report

~~~bash
deonctl worker codex provider-response-change-proposal-report \
  --change-proposal retrieval-context-provider-response-change-proposal.json \
  --response-fixture retrieval-context-provider-response-fixture.json \
  --execution-simulation-report retrieval-context-provider-execution-simulation-report.json \
  --real-call-proposal retrieval-context-provider-real-call-proposal.json \
  --output-format json
~~~

### 65. Provider activation readiness audit

~~~bash
deonctl worker codex provider-activation-readiness-audit \
  --credential-policy-plan retrieval-context-provider-credential-policy-plan.json \
  --real-call-proposal retrieval-context-provider-real-call-proposal.json \
  --change-proposal retrieval-context-provider-response-change-proposal.json \
  --execution-simulation-report retrieval-context-provider-execution-simulation-report.json \
  --release-gate retrieval-context-provider-executor-release-gate.json \
  --output-format json
~~~

### 66. Provider activation readiness report

~~~bash
deonctl worker codex provider-activation-readiness-report \
  --credential-policy-plan retrieval-context-provider-credential-policy-plan.json \
  --real-call-proposal retrieval-context-provider-real-call-proposal.json \
  --change-proposal retrieval-context-provider-response-change-proposal.json \
  --execution-simulation-report retrieval-context-provider-execution-simulation-report.json \
  --release-gate retrieval-context-provider-executor-release-gate.json \
  --output-format json
~~~

## CI smoke (Tasks 22.37–22.52)

Run the provider-call chain fixture smoke from the repository root:

~~~bash
make provider-call-chain-smoke
# or
bash scripts/provider-call-chain-fixture-smoke.sh
~~~

This executes the chain in isolated temp dirs and asserts final activation readiness report keeps `provider_call`, `network_call`, `worker_execution`, `sent_to_provider`, `transport_called`, `received_from_provider`, `secret_values_read`, and `workspace_modified` false. It exercises executor sprint (22.41–22.44), execution simulation layer (22.45–22.48), and activation readiness layer (22.49–22.52).

See [docs/PROVIDER_CALL_CHAIN_GATE.md](../../../docs/PROVIDER_CALL_CHAIN_GATE.md) for the last-gate boundary before any real provider dispatch.

## Expected outcome

- inspect, materialized-report, bundle, approval inspect, injection-plan, and governance-report return `status: ok`
- injection-policy validate returns ok after steps 1–9 generated the artifacts
- injection-policy plan returns `would_inject: false`, `reason: schema_only_no_execution`, and `status: warning` while approval still has `runner_injection_allowed: false`
- optional injection-approval inspect returns `runner_injection_allowed: true` and `allowed_use: runner_injection_policy_only` on the dedicated injection-approval artifact; Task 22.11 `retrieval-context-approval.json` stays `manual_review_only` with `runner_injection_allowed: false`
- optional injection-execution-plan returns `would_execute_runner: false`, `execution_supported_now: false`, `would_inject_materialized_context: true`, and `reason: execution_plan_only`
- optional prompt-preview writes markdown with `text_excerpt` and a manifest with `contains_text: true`, `preview_only: true`, `runner_execution: false` (manifest must not repeat chunk text)
- optional prompt-preview-report returns `status: ok` and does not print preview chunk text in report output
- optional injection-governance-bundle returns `injection_authorized_for_future: true`, `runner_execution: false`, `execution_supported_now: false`, and does not include preview markdown or chunk text
- optional `materialized-injection-task.yaml` declares `retrieval_context.materialized_injection` with `enabled: false` for schema validation only (requires step 18 bundle artifact)
- optional materialized-injection-report returns `status: ok`, `prompt_preview_read: false`, `runner_prompt_changed: false`, and does not print preview chunk text
- optional materialized-injection-preflight returns `worker_execution_allowed: false`, `prompt_injection_allowed_now: false`, and does not print preview chunk text
- optional materialized-injection-dry-run returns `worker_execution: false`, `prompt_changed_in_real_runner: false`, `prompt_section_rendered: true`, writes prompt-section markdown with dry-run notice and `text_excerpt` (stdout/json dry-run output must not contain chunk text)
- optional materialized-injection-dry-run-report returns `status: ok`, validates prompt-section hash and dry-run flags, and does not print preview chunk text in report output
- optional materialized-injection-readiness-report returns `governance_ready_for_future_execution: true`, `execution_allowed_now: false`, and does not print preview chunk text in report output
- optional materialized-injection-execution-gate returns `execution_gate_ready: true`, `implementation_allows_execution_now: false`, `worker_execution_allowed: false`, `prompt_injection_allowed_now: false`, `blocked_reason: implementation_not_enabled`, and does not print preview chunk text in report output
- optional materialized-prompt-assembly-dry-run returns `assembled_prompt_rendered: true`, `worker_execution: false`, `sent_to_worker: false`, `prompt_changed_in_real_runner: false`, writes assembled markdown with dry-run notice and `text_excerpt` (stdout/json assembly dry-run output must not contain chunk text)
- optional materialized-prompt-assembly-report returns `status: ok`, `assembled_prompt_validated: true`, `worker_execution: false`, `sent_to_worker: false`, `prompt_changed_in_real_runner: false`, and does not print preview chunk text in report output
- optional materialized-injection-runtime validate returns `status: ok`, `enabled: false`, `allow_worker_execution: false`, `allow_prompt_injection: false`, `blocked_reason: implementation_not_enabled`
- optional materialized-injection-run-plan returns `run_plan_ready: true`, `worker_execution_planned: false`, `prompt_injection_planned: false`, `implementation_allows_execution_now: false`, `blocked_reason: implementation_not_enabled`, and does not print preview chunk text in plan output
- optional materialized-injection-execution-enable validate returns `status: ok`, `enabled: false`, `allow_provider_call: false`, `allow_worker_execution: false`, `allow_prompt_injection: false`, `blocked_reason: implementation_not_enabled`
- optional materialized-injection-provider-run-plan returns `provider_run_plan_ready: true`, `would_use_assembled_prompt: true`, `sent_to_provider: false`, `provider_call_allowed_now: false`, `worker_execution_allowed_now: false`, `prompt_injection_allowed_now: false`, `blocked_reason: implementation_not_enabled`, and does not print preview chunk text in plan output
- optional materialized-provider-dispatch validate returns `status: ok`, `enabled: false`, `allow_provider_call: false`, `allow_network: false`, `blocked_reason: implementation_not_enabled`
- optional materialized-provider-payload-dry-run returns `provider_payload_rendered: true`, `provider_call: false`, `network_call: false`, `worker_execution: false`, `sent_to_provider: false`; only `--payload-output` may contain `text_excerpt`
- optional materialized-provider-payload-report returns `provider_payload_validated: true`, `provider_call: false`, `network_call: false`, `sent_to_provider: false`, and does not print payload content
- optional materialized-provider-call-gate returns `provider_call_gate_ready: true`, `provider_call_allowed_now: false`, `network_call_allowed_now: false`, `sent_to_provider: false`, and does not print payload content
- optional materialized-provider-call-readiness-report returns `provider_call_readiness_ready: true`, `provider_call_allowed_now: false`, `network_call_allowed_now: false`, `sent_to_provider: false`, and does not print payload content
- optional provider-call-approval inspect returns `provider_call_authorized_for_future: true`, `provider_call_allowed_now: false`, `sent_to_provider: false`, and does not print payload-output content
- optional provider-call-execution-bundle returns `provider_call_authorized_for_future: true`, `provider_call_allowed_now: false`, `sent_to_provider: false`, `provider_call: false`, `network_call: false`, `worker_execution: false`, `execution_supported_now: false`, and does not print payload-output content
- optional provider-call-chain-audit returns `chain_continuity_ready: true`, `loaders_reconciled: true`, `producer_commands_active: true`, `provider_call: false`, `network_call: false`, `worker_execution: false`, `sent_to_provider: false`, and does not print payload-output content
- optional provider-call-executor config validate returns `status: ok`, `enabled: false`, `allow_provider_call: false`, `allow_network: false`, `blocked_reason: implementation_not_enabled`
- optional provider-call-executor validate/plan returns `executor_policy_validated: true`, `executor_config_validated: true`, `execution_supported_now: false`, `provider_call_authorized_for_future: true`, `chain_continuity_ready: true`, all execution flags false, and does not print payload-output content
- optional provider-call-executor dry-run/dry-run-report returns `executor_dry_run_ready: true`, `transport_called: false`, `contains_text: false`, `preview_only: true`, all execution flags false, and does not read or print payload markdown
- optional provider-call-executor preflight returns `executor_preflight_ready: true`, `execution_allowed_now: false`, `required_future_confirm_flag: --confirm-provider-executor-dispatch`, all execution flags false
- optional provider-call-executor-dispatch-approval returns `dispatch_authorized_for_future: true`, `dispatch_allowed_now: false`, all execution flags false
- optional provider-transport-plan returns `transport_plan_ready: true`, `transport_enabled: false`, `transport_mode: BlockedProviderTransport`, all execution flags false
- optional provider-executor-release-bundle/gate returns `release_bundle_ready: true`, `activation_gate_ready: true`, `activation_allowed_now: false`, `execution_supported_now: false`, all execution flags false
- optional provider-request-envelope dry-run/report returns `provider_request_ready: true`, `request_contains_text: false`, `sent_to_provider: false`, all execution flags false
- optional provider-adapter-registry validate/plan returns `adapter_registry_validated: true`, `adapter_available_for_future: true`, `adapter_enabled_now: false`, `transport_enabled: false`, all execution flags false
- optional provider-response-fixture generate/inspect returns `response_fixture_ready: true`, `response_source: fixture`, `received_from_provider: false`, all execution flags false
- optional provider-execution-simulation-bundle/report returns `simulation_bundle_ready: true`, `simulated_response_ready: true`, `execution_result_available: false`, all execution flags false
- optional provider-credential-policy validate/plan returns `credential_policy_validated: true`, `credential_check_enabled: false`, `secret_values_read: false`, all execution flags false
- optional provider-real-call-proposal new/inspect returns `real_call_proposal_ready: true`, `would_call_provider_if_enabled: true`, `provider_call_allowed_now: false`, `secret_values_read: false`, all transport flags false
- optional provider-response-change-proposal/report returns `change_proposal_ready: true`, `response_source: fixture`, `workspace_modified: false`, `diff_applied: false`, `worker_execution: false`
- optional provider-activation-readiness-audit/report returns `activation_readiness_ready: true`, `activation_allowed_now: false`, `real_provider_call_supported_now: false`, `blocked_reason: implementation_not_enabled`, all execution flags false
- `make provider-call-chain-smoke` passes library + CLI fixture e2e guards (Tasks 22.37–22.52)
- `can_inject_now: false` in injection-plan and governance-report
- `required_future_flag: --confirm-inject-materialized-context` in injection-plan
- only `retrieval-context-materialized.json` / `.md` contain chunk text excerpts; other artifacts must not include `text_excerpt`

## Safety

- metadata-only runner attachment remains separate from this fixture workflow
- the materialized prompt assembly is dry-run only and is never sent to Codex or OpenCode
- the materialized injection execution gate keeps `worker_execution_allowed: false`, `prompt_injection_allowed_now: false`, and `blocked_reason: implementation_not_enabled`
- no memory apply/restore changes
- no LanceDB search or provider API calls
