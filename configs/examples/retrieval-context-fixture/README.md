# Retrieval context governance E2E fixture

Small reproducible fixture for the retrieval-context governance chain (Tasks 22.8–22.12.1).

This fixture does **not** run LanceDB search, call embedding providers, or inject materialized text into runner prompts.

## Contents

- `retrieval-context.json` — passive metadata-only retrieval artifact (one hit)
- `memory-index-chunks.jsonl` — governed chunk text source for materialize
- `retrieval-injection-policy.yaml` — plan-only injection policy over generated artifacts (Task 22.15.1)
- `materialized-injection-task.yaml` — schema-only materialized injection task declaration (Task 22.19)
- `base-worker-prompt-fixture.md` — dry-run base worker prompt fixture for materialized prompt assembly (Task 22.23)

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

## CI smoke (Task 22.37)

Run the provider-call chain fixture smoke from the repository root:

~~~bash
make provider-call-chain-smoke
# or
bash scripts/provider-call-chain-fixture-smoke.sh
~~~

This executes the chain in isolated temp dirs and asserts `provider-call-chain-audit` returns `chain_continuity_ready: true`, `producer_commands_active: true`, `loaders_reconciled: true`, and keeps `provider_call`, `network_call`, `worker_execution`, and `sent_to_provider` false.

See [docs/PROVIDER_CALL_CHAIN_GATE.md](../../../docs/PROVIDER_CALL_CHAIN_GATE.md) for the last-gate boundary before a future provider executor skeleton.

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
- `make provider-call-chain-smoke` passes library + CLI fixture e2e guards (Task 22.37)
- `can_inject_now: false` in injection-plan and governance-report
- `required_future_flag: --confirm-inject-materialized-context` in injection-plan
- only `retrieval-context-materialized.json` / `.md` contain chunk text excerpts; other artifacts must not include `text_excerpt`

## Safety

- metadata-only runner attachment remains separate from this fixture workflow
- the materialized prompt assembly is dry-run only and is never sent to Codex or OpenCode
- the materialized injection execution gate keeps `worker_execution_allowed: false`, `prompt_injection_allowed_now: false`, and `blocked_reason: implementation_not_enabled`
- no memory apply/restore changes
- no LanceDB search or provider API calls
