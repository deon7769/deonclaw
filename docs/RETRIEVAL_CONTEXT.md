# Retrieval Context

Task 22.8 adds passive LanceDB retrieval context attachment for worker runs. Task 22.8.1 adds read-only inspection and run reporting over retrieval-context artifacts. Task 22.9 adds governed chunk text materialization from memory-index chunks JSONL. None of these paths execute LanceDB search, call embedding providers, or read memory source files directly.

## Scope

Implemented now:

- `retrieval_context.attachments` on task YAML
- kind `lancedb_search_report` only
- runner-side validation before worker execution
- `retrieval-context.md` and `retrieval-context.json` run artifacts
- passive prompt section `Retrieved context metadata only`
- execution trace fields: `retrieval_context_attached`, `retrieval_context_count`, `retrieval_context_sha256`, `retrieval_context_status`
- sanitized `RunSpec.Task` without attachment paths
- `deonctl retrieval context inspect` for `retrieval-context.json` QA
- `deonctl runs retrieval-report` for runs with `retrieval_context_attached` in execution traces
- `deonctl retrieval context materialize` for governed chunk text excerpts from chunks JSONL (explicit confirm flag)
- `deonctl retrieval context materialized-report` for QA over materialized chunk text artifacts
- `deonctl retrieval context bundle` for audit bundle over retrieval + materialized artifacts
- `deonctl retrieval context approval new/approve/inspect` for governed materialized context approval
- `deonctl retrieval context injection-plan` for runner injection planning without execution
- `deonctl retrieval context governance-report` for consolidated chain audit and troubleshooting
- `configs/examples/retrieval-context-fixture/` end-to-end governance chain fixture
- [Retrieval governance release checklist](RETRIEVAL_GOVERNANCE_CHECKLIST.md) (Task 22.13)
- [Retrieval runner injection ADR](ADR_RETRIEVAL_RUNNER_INJECTION.md) (Task 22.14, design only)
- `deonctl retrieval context injection-policy validate/plan` (Task 22.15, schema only)
- `deonctl retrieval context injection-approval new/approve/inspect` (Task 22.16, approval artifact only)
- `deonctl retrieval context injection-execution-plan` (Task 22.17, execution plan only)
- `deonctl retrieval context prompt-preview` (Task 22.18, preview render only)
- `deonctl retrieval context prompt-preview-report` (Task 22.18.1, preview QA only)
- `deonctl retrieval context injection-governance-bundle` (Task 22.18.2, injection governance release bundle only)
- task YAML `retrieval_context.materialized_injection` (Task 22.19, schema declaration only)
- `deonctl task materialized-injection-report` (Task 22.19.1, task declaration QA only)
- `deonctl worker codex materialized-injection-preflight` (Task 22.20, runner preflight only)
- `deonctl worker codex materialized-injection-dry-run` (Task 22.21, dry-run prompt section artifact only)
- `deonctl worker codex materialized-injection-dry-run-report` (Task 22.21.1, dry-run report/QA only)
- `deonctl worker codex materialized-injection-readiness-report` (Task 22.21.2, readiness report only)
- `deonctl worker codex materialized-injection-execution-gate` (Task 22.22, execution gate only)
- `deonctl worker codex materialized-prompt-assembly-dry-run` (Task 22.23, prompt assembly dry-run only)
- `deonctl worker codex materialized-prompt-assembly-report` (Task 22.24, prompt assembly report only)
- `deonctl worker codex materialized-injection-runtime validate` (Task 22.25, runtime config schema only)
- `deonctl worker codex materialized-injection-run-plan` (Task 22.26, run planner only)
- `deonctl worker codex materialized-injection-execution-enable validate` (Task 22.27, execution enablement policy schema only)
- `deonctl worker codex materialized-injection-provider-run-plan` (Task 22.28, provider run-plan only)
- `deonctl worker codex materialized-provider-dispatch validate` (Task 22.29, provider dispatch contract schema only)
- `deonctl worker codex materialized-provider-payload-dry-run` (Task 22.30, provider payload dry-run only)
- `deonctl worker codex materialized-provider-payload-report` (Task 22.31, provider payload report only)
- `deonctl worker codex materialized-provider-call-gate` (Task 22.32, provider call gate only)
- `deonctl worker codex materialized-provider-call-readiness-report` (Task 22.33, provider call readiness report only)
- `deonctl worker codex provider-call-approval new/approve/inspect` (Task 22.34, provider call approval only)
- `deonctl worker codex provider-call-execution-bundle` (Task 22.35, provider call execution bundle only)
- `deonctl worker codex provider-call-chain-audit` (Task 22.36, provider call chain continuity audit only)
- `make provider-call-chain-smoke` / `scripts/provider-call-chain-fixture-smoke.sh` (Task 22.37, fixture smoke / CI guard only)
- `deonctl worker codex provider-call-executor config validate` (Task 22.39, executor policy schema only)
- `deonctl worker codex provider-call-executor validate/plan` (Tasks 22.38–22.39, executor skeleton only)
- `deonctl worker codex provider-call-executor dry-run` / `dry-run-report` (Task 22.40, executor dry-run contract only)
- `deonctl worker codex provider-call-executor preflight` (Task 22.41, executor preflight only)
- `deonctl worker codex provider-call-executor-dispatch-approval new/approve/inspect` (Task 22.42, dispatch approval only)
- `deonctl worker codex provider-transport-plan` (Task 22.43, blocked transport plan only)
- `deonctl worker codex provider-executor-release-bundle` / `release-gate` (Task 22.44, release activation gate only)
- `deonctl worker codex provider-request-envelope dry-run` / `report` (Task 22.45, request envelope only)
- `deonctl worker codex provider-adapter-registry validate` / `provider-adapter-plan` (Task 22.46, blocked adapter plan only)
- `deonctl worker codex provider-response-fixture generate` / `inspect` (Task 22.47, fake response fixture only)
- `deonctl worker codex provider-execution-simulation-bundle` / `report` (Task 22.48, execution simulation only)
- `deonctl worker codex provider-credential-policy` validate/plan (Task 22.49, credential policy blocked)
- `deonctl worker codex provider-real-call-proposal` new/inspect (Task 22.50, real-call proposal not executable)
- `deonctl worker codex provider-response-change-proposal` / `report` (Task 22.51, fixture-only change proposal)
- `deonctl worker codex provider-activation-readiness-audit` / `report` (Task 22.52, activation readiness audit)
- `deonctl worker codex provider-activation-policy` validate/plan (Task 22.53, activation policy blocked)
- `deonctl worker codex provider-activation-approval` new/approve/inspect (Task 22.54, future activation approval)
- `deonctl worker codex provider-activation-rehearsal` / `rehearsal-report` (Task 22.55, activation sequence rehearsal)
- `deonctl worker codex provider-activation-release-package` / `release-gate` (Task 22.56, activation release gate)
- `deonctl worker codex provider-activation-operator-review-bundle` / `report` (Task 22.58, operator review metadata)
- `deonctl worker codex provider-activation-kill-switch` validate/plan (Task 22.59, kill-switch manifest)
- `deonctl worker codex provider-activation-final-audit` / `ci-report` (Task 22.60, final audit / CI observability)
- `deonctl worker codex provider-secret-read-proposal` new/inspect (Task 22.61, future secret-read env var names only)
- `deonctl worker codex provider-real-transport-implementation-plan` / `report` (Task 22.62, future transport contract)
- `deonctl worker codex provider-real-dispatch-design` / `report` (Task 22.63, future dispatch command design)
- `deonctl worker codex provider-real-activation-design-review-package` / `gate` (Task 22.64, real dispatch design review gate)
- `deonctl worker codex provider-real-dispatch-external-approval` new/approve/inspect (Task 22.65, external approval blocked)
- `deonctl worker codex provider-real-dispatch-runbook` generate/report (Task 22.66, metadata-only runbook)
- `deonctl worker codex provider-real-dispatch-risk-register` / `report` (Task 22.67, risk register)
- `deonctl worker codex provider-real-dispatch-preimplementation-gate` / `report` (Task 22.68, preimplementation gate)
- `configs/examples/provider-activation-policy.yaml` example activation policy
- `configs/examples/provider-activation-kill-switch.yaml` example activation kill-switch

Not implemented yet:

- natural-language retrieval or query embedding
- LanceDB search inside the runner
- automatic chunk text injection into worker prompts
- MCP integration
- UI/dashboard

## Prerequisites

Before attaching retrieval context to a worker task:

1. Build index chunks and embedding vectors (Tasks 22.0–22.3.1)
2. Run `memory lancedb write-smoke` to create a local table
3. Run `memory lancedb search-smoke` with an explicit query vector or chunk row (Task 22.7)
4. Run `memory lancedb search-report` and confirm `status: ok` (Task 22.7.1)

The runner re-validates the search result with `lancedbpolicy.SearchReport` at run time. It does not perform a new search.

## Task schema

~~~yaml
retrieval_context:
  attachments:
    - kind: lancedb_search_report
      path: artifacts/lancedb-search-smoke-result.json
      report_path: artifacts/lancedb-search-report.json
      policy: configs/examples/lancedb-policy-write-smoke.yaml
      max_results: 5
~~~

Rules:

- `kind` must be `lancedb_search_report`
- `path`, `report_path`, and `policy` are required
- `max_results` must be `> 0` and `<= 20`
- optional `name` defaults to `lancedb-search-N`

## Runner behavior

Before the worker starts, the runner:

1. checks `path`, `report_path`, and `policy` exist
2. loads the stored `search-report` artifact (`status: ok`)
3. runs `lancedbpolicy.SearchReport(result, policy)` and fails if `status != ok`
4. extracts up to `max_results` hits with safe metadata only

Safe metadata per hit:

- `rank`, `chunk_id`, `distance`
- optional `domain`, `source_path`, `source_sha256`, `text_sha256`, `embedding_model`, `provider`

Never attached:

- `vector`, `text`, `chunk_text`, `content`, `embedding`
- memory source file contents

The worker receives a sanitized task copy without `retrieval_context` paths. The composed prompt includes a passive metadata section only.

## Artifacts

Per run (under `artifacts/<run-id>/`):

- `retrieval-context.md` — human-readable metadata summary
- `retrieval-context.json` — machine-readable safe payload
- `execution-trace.json` — attachment status and SHA256

## Inspection and reporting

Inspect a run artifact (no search, no provider calls):

~~~bash
deonctl retrieval context inspect \
  --artifact artifacts/<run-id>/retrieval-context.json
deonctl retrieval context inspect \
  --artifact artifacts/<run-id>/retrieval-context.json \
  --output-format json
~~~

`inspect` validates:

- `status: ok`
- `retrieval_context_attached` coherence with `hit_count`
- allowed attachment kinds
- hit shape (`rank`, `chunk_id`, `distance`)
- forbidden fields absent (`vector`, `text`, `chunk_text`, `content`, `embedding`)

Stdout never includes raw chunk text or full vectors.

List runs that attached retrieval context from the SQLite store:

~~~bash
deonctl runs retrieval-report --store deonclaw.db
deonctl runs retrieval-report --store deonclaw.db --run <run-id> --output-format json
~~~

`runs retrieval-report` reads execution traces and artifact paths only. It does not open memory source files and does not execute LanceDB search.

## Materialized chunk text (Task 22.9)

Runner attachment remains metadata-only. Materializing chunk text is a separate, explicit, confirmed step. The runner does not inject materialized text into worker prompts automatically.

~~~bash
deonctl retrieval context materialize \
  --retrieval-context artifacts/<run-id>/retrieval-context.json \
  --chunks artifacts/memory-index-chunks.jsonl \
  --output artifacts/<run-id>/retrieval-context-materialized.json \
  --summary artifacts/<run-id>/retrieval-context-materialized.md \
  --max-chars-per-chunk 1200 \
  --max-total-chars 6000 \
  --confirm-include-chunk-text
~~~

Rules:

- `--confirm-include-chunk-text` is required
- loads and inspects `retrieval-context.json`; fails when inspect status is not `ok`
- resolves chunk text only from `--chunks` JSONL (never opens memory source files)
- validates `text_sha256` for every hit; validates `source_sha256` when present on the hit
- `chunk_id` missing from chunks JSONL fails with a clear error (not a silent skip)
- applies `max_chars_per_chunk` and `max_total_chars`; truncates with explicit warnings and char counts
- `--retrieval-context`, `--chunks`, `--output`, and `--summary` must be relative safe paths (no absolute, `..`, `secrets`, or `.env`)
- never includes vector, embedding, or environment values

Outputs:

- `retrieval-context-materialized.json` — governed excerpts with hashes and truncation metadata
- `retrieval-context-materialized.md` — human summary stating derived/non-canonical status

Validate a materialized artifact before any human review or future runner integration:

~~~bash
deonctl retrieval context materialized-report \
  --artifact artifacts/<run-id>/retrieval-context-materialized.json
deonctl retrieval context materialized-report \
  --artifact artifacts/<run-id>/retrieval-context-materialized.json \
  --output-format json
~~~

`materialized-report` validates JSON shape, hash fields, count coherence, per-item char limits, and forbidden fields (`vector`, `embedding`, `raw_embedding`, `env`, `secret`). It accepts artifact status `ok` or `warning`. Stdout text output never prints `text_excerpt` values.

## Audit bundle (Task 22.10)

The bundle is the checkpoint before any governed use of materialized chunk text. It consolidates validated retrieval metadata and materialized artifact references only. The bundle does not contain materialized text — only paths, hashes, counts, IDs, and warnings.

~~~bash
deonctl retrieval context bundle \
  --retrieval-context artifacts/<run-id>/retrieval-context.json \
  --materialized artifacts/<run-id>/retrieval-context-materialized.json \
  --output artifacts/<run-id>/retrieval-context-bundle.json \
  --summary artifacts/<run-id>/retrieval-context-bundle.md
deonctl retrieval context bundle \
  --retrieval-context artifacts/<run-id>/retrieval-context.json \
  --materialized artifacts/<run-id>/retrieval-context-materialized.json \
  --output artifacts/<run-id>/retrieval-context-bundle.json \
  --summary artifacts/<run-id>/retrieval-context-bundle.md \
  --output-format json
~~~

Rules:

- runs `inspect` on retrieval context; fails when inspect status is not `ok`
- runs `materialized-report` on the materialized artifact; fails when report status is `failed`
- validates `source_retrieval_context_sha256` against the retrieval context file
- materialized `chunk_id` values must be a subset of retrieval context chunk IDs
- `included_chunk_count` must be `<=` retrieval `hit_count`
- bundle status is `warning` when materialized report is `warning` or `omitted_chunk_count > 0`
- `contains_text: false` in bundle JSON; no `text_excerpt` in bundle JSON, summary, or stdout text output
- all input/output paths must be relative safe paths

## Approval workflow (Task 22.11)

The audit bundle is the checkpoint before any governed use of materialized chunk text. Explicit human approval is a separate, auditable step. Approval does not enable runner text injection yet.

Create an approval request from a validated bundle:

~~~bash
deonctl retrieval context approval new \
  --bundle artifacts/<run-id>/retrieval-context-bundle.json \
  --output artifacts/<run-id>/retrieval-context-approval-request.json
~~~

Approve for manual review only:

~~~bash
deonctl retrieval context approval approve \
  --request artifacts/<run-id>/retrieval-context-approval-request.json \
  --output artifacts/<run-id>/retrieval-context-approval.json \
  --confirm-approve-materialized-context
~~~

Inspect an approval artifact:

~~~bash
deonctl retrieval context approval inspect \
  --approval artifacts/<run-id>/retrieval-context-approval.json
deonctl retrieval context approval inspect \
  --approval artifacts/<run-id>/retrieval-context-approval.json \
  --output-format json
~~~

Rules:

- `approval new` fails when bundle status is `failed`
- request and approval artifacts never include `text_excerpt`
- `contains_text: false` on requests; `runner_injection_allowed: false` on approvals
- `allowed_use: manual_review_only`
- `--confirm-approve-materialized-context` is required for approve
- inspect stdout never prints materialized text
- all paths must be relative safe paths

## Injection plan (Task 22.12)

Current approvals allow `manual_review_only` and keep `runner_injection_allowed: false`. The injection plan describes what a future runner prompt integration would require without injecting text or changing task schema.

~~~bash
deonctl retrieval context injection-plan \
  --approval artifacts/<run-id>/retrieval-context-approval.json \
  --request artifacts/<run-id>/retrieval-context-approval-request.json \
  --bundle artifacts/<run-id>/retrieval-context-bundle.json \
  --materialized artifacts/<run-id>/retrieval-context-materialized.json \
  --output artifacts/<run-id>/retrieval-context-injection-plan.json \
  --summary artifacts/<run-id>/retrieval-context-injection-plan.md
~~~

Rules:

- runs `approval inspect` with request binding; fails when inspect status is not `ok`
- requires `approved: true`, `allowed_use: manual_review_only`, and `runner_injection_allowed: false`
- fails when `runner_injection_allowed: true` (injection not supported in this task)
- validates approval/bundle/materialized SHA256 coherence
- runs `materialized-report`; fails when report status is `failed`
- plan JSON sets `can_inject_now: false` and `reason: runner_injection_allowed_false`
- records `required_future_flag: --confirm-inject-materialized-context` for a future explicit injection step
- plan and summary never include `text_excerpt` or materialized text
- does not alter runner prompts or task schema

A future task would need a separate approval artifact with `runner_injection_allowed: true` or another explicit gate before any prompt injection.

## Governance report (Task 22.12.1)

Consolidated audit over the full retrieval-context governance chain:

~~~text
retrieval-context -> materialized -> materialized-report -> bundle -> approval-request -> approval -> injection-plan
~~~

~~~bash
deonctl retrieval context governance-report \
  --retrieval-context artifacts/<run-id>/retrieval-context.json \
  --materialized artifacts/<run-id>/retrieval-context-materialized.json \
  --bundle artifacts/<run-id>/retrieval-context-bundle.json \
  --request artifacts/<run-id>/retrieval-context-approval-request.json \
  --approval artifacts/<run-id>/retrieval-context-approval.json \
  --injection-plan artifacts/<run-id>/retrieval-context-injection-plan.json
~~~

Rules:

- path hardening on all inputs
- runs inspect/materialized-report/bundle validation/approval inspect/injection-plan parse
- validates SHA256 coherence across the chain
- requires `injection_plan.can_inject_now: false` and `reason: runner_injection_allowed_false`
- text and JSON output never include `text_excerpt` or materialized chunk text
- does not inject runner prompts, search LanceDB, or alter task schema

Use this report for auditing and troubleshooting before any future injection task.

## End-to-end fixture (Task 22.12.2)

Reproducible fixture at `configs/examples/retrieval-context-fixture/`:

- `retrieval-context.json` — metadata-only retrieval artifact
- `memory-index-chunks.jsonl` — chunk text source for materialize
- `retrieval-injection-policy.yaml` — plan-only injection policy over generated artifacts (Task 22.15.1)
- `README.md` — step-by-step `deonctl` commands for the full chain

Chain:

~~~text
inspect -> materialize -> materialized-report -> bundle -> approval new -> approval approve -> approval inspect -> injection-plan -> governance-report -> injection-policy validate -> injection-policy plan
~~~

The fixture validates documentation and CLI behavior without LanceDB search, provider calls, or runner text injection. Only materialized artifacts may contain `text_excerpt`. Task 22.15.1 extends the fixture with `retrieval-injection-policy.yaml` so validate/plan run against real artifacts from the governed chain (`would_inject: false`, `reason: schema_only_no_execution`, warning while `runner_injection_allowed` remains false).

Before releasing governance changes, use [RETRIEVAL_GOVERNANCE_CHECKLIST.md](RETRIEVAL_GOVERNANCE_CHECKLIST.md). For future runner injection scope, see [ADR_RETRIEVAL_RUNNER_INJECTION.md](ADR_RETRIEVAL_RUNNER_INJECTION.md).

## Injection policy (Task 22.15)

Schema and plan-only validation for a future runner materialized-context injection path. Does not inject text or alter runner/task schema.

~~~bash
deonctl retrieval context injection-policy validate \
  --policy configs/examples/retrieval-injection-policy.yaml

deonctl retrieval context injection-policy plan \
  --policy configs/examples/retrieval-injection-policy.yaml \
  --output-format json
~~~

Rules:

- `mode` must be `plan_only`
- artifact paths must be relative safe paths (no absolute, `..`, `secrets/`, `.env/`)
- `approval.required` and `approval.require_runner_injection_allowed` must be `true`
- all `safety.*` flags must be `true`
- policy YAML must not contain `text_excerpt`
- plan sets `would_inject: false` and `reason: schema_only_no_execution`
- plan may probe approval/bundle/materialized-report when configured paths exist; never prints `text_excerpt`
- plan warns when probed approval still has `runner_injection_allowed: false`

The e2e fixture at `configs/examples/retrieval-context-fixture/` includes `retrieval-injection-policy.yaml` with paths to artifacts produced by the README chain; see steps 10–11 in the fixture README.

## Injection approval (Task 22.16)

Dedicated approval artifact for future runner materialized-context injection. Separate from Task 22.11 materialized-context approval (`manual_review_only`, `runner_injection_allowed: false`). Does not inject text or alter runner/task schema.

~~~bash
deonctl retrieval context governance-report ... --output-format json \
  > artifacts/<run-id>/retrieval-context-governance-report.json

deonctl retrieval context injection-approval new \
  --governance-report artifacts/<run-id>/retrieval-context-governance-report.json \
  --policy configs/examples/retrieval-injection-policy.yaml \
  --output artifacts/<run-id>/retrieval-context-injection-approval-request.json

deonctl retrieval context injection-approval approve \
  --request artifacts/<run-id>/retrieval-context-injection-approval-request.json \
  --output artifacts/<run-id>/retrieval-context-injection-approval.json \
  --confirm-allow-runner-injection

deonctl retrieval context injection-approval inspect \
  --approval artifacts/<run-id>/retrieval-context-injection-approval.json \
  --request artifacts/<run-id>/retrieval-context-injection-approval-request.json \
  --output-format json
~~~

Rules:

- `new` loads governance-report JSON and requires `status: ok` or `status: warning` (documented acceptance)
- `new` validates injection-policy schema and requires injection-policy plan status not `failed`
- request records governance/policy/materialized/bundle SHA256s and policy char caps; `contains_text: false`; no `text_excerpt`
- `approve` requires `--confirm-allow-runner-injection` and pending request
- approval sets `runner_injection_allowed: true`, `allowed_use: runner_injection_policy_only`, `confirm_allow_runner_injection: true`
- Task 22.11 approval artifacts remain `manual_review_only` with `runner_injection_allowed: false`
- inspect may bind `request_sha256` when `--request` is provided

Optional fixture steps 12–14 in `configs/examples/retrieval-context-fixture/README.md` exercise this path after the governance chain.

## Injection execution plan (Task 22.17)

Plan-only artifact for a future runner materialized-context injection execution path. Requires Task 22.16 injection-approval with `runner_injection_allowed: true`. Does not execute workers, inject text, or alter runner/task schema.

~~~bash
deonctl retrieval context injection-execution-plan \
  --policy configs/examples/retrieval-injection-policy.yaml \
  --governance-report artifacts/<run-id>/retrieval-context-governance-report.json \
  --injection-approval artifacts/<run-id>/retrieval-context-injection-approval.json \
  --injection-approval-request artifacts/<run-id>/retrieval-context-injection-approval-request.json \
  --materialized artifacts/<run-id>/retrieval-context-materialized.json \
  --output artifacts/<run-id>/retrieval-context-injection-execution-plan.json \
  --summary artifacts/<run-id>/retrieval-context-injection-execution-plan.md
~~~

Rules:

- validates injection-policy schema and requires injection-policy plan status not `failed`
- loads governance-report JSON with `status: ok` or documented `warning` acceptance
- inspects injection-approval with request binding; requires `runner_injection_allowed: true` and `allowed_use: runner_injection_policy_only`
- validates approval `policy_sha256`, `governance_report_sha256`, and `materialized_sha256` against loaded artifacts
- runs materialized-report and enforces policy char/chunk caps
- plan sets `would_execute_runner: false`, `would_inject_materialized_context: true`, `execution_supported_now: false`, `reason: execution_plan_only`
- plan and summary must not contain `text_excerpt`

Optional fixture step 15 exercises this path after injection-approval.

## Prompt preview (Task 22.18)

Dry-run render of the governed materialized prompt section that a future runner injection path could use. Requires a valid Task 22.17 execution-plan. Does not execute workers, alter runner prompts, or change task schema.

~~~bash
deonctl retrieval context prompt-preview \
  --execution-plan artifacts/<run-id>/retrieval-context-injection-execution-plan.json \
  --policy configs/examples/retrieval-injection-policy.yaml \
  --materialized artifacts/<run-id>/retrieval-context-materialized.json \
  --output artifacts/<run-id>/retrieval-context-prompt-preview.md \
  --manifest artifacts/<run-id>/retrieval-context-prompt-preview.json \
  --confirm-render-materialized-context
~~~

Rules:

- requires `--confirm-render-materialized-context`
- execution-plan must have `would_inject_materialized_context: true`, `would_execute_runner: false`, `execution_supported_now: false`, `reason: execution_plan_only`
- validates injection-policy safety flags and materialized-report status not `failed`
- validates execution-plan `materialized_sha256` and policy char/chunk caps
- markdown preview may contain `text_excerpt`; manifest records `contains_text: true`, `preview_only: true`, `runner_execution: false` without repeating chunk text
- all other governance artifacts remain text-free

Optional fixture step 16 exercises this path after injection-execution-plan.

## Prompt preview report (Task 22.18.1)

QA/report over Task 22.18 preview markdown and manifest. Reads preview for validation but does not print chunk text in report output.

~~~bash
deonctl retrieval context prompt-preview-report \
  --preview artifacts/<run-id>/retrieval-context-prompt-preview.md \
  --manifest artifacts/<run-id>/retrieval-context-prompt-preview.json \
  --execution-plan artifacts/<run-id>/retrieval-context-injection-execution-plan.json \
  --output-format json
~~~

Rules:

- validates manifest flags (`contains_text`, `preview_only`, `runner_execution`), hashes, and caps
- validates execution-plan remains plan-only (`would_execute_runner: false`, `reason: execution_plan_only`)
- validates preview contains section title, derived-context notice, and `text_excerpt`
- manifest must not contain `text_excerpt`; report text/json must not leak preview chunk text
- preview markdown must not contain `embedding` or `vector`

Optional fixture step 17 exercises this path after prompt-preview.

## Injection governance release bundle (Task 22.18.2)

Metadata-only release bundle over the full injection governance chain. Does not include preview markdown or materialized chunk text.

~~~bash
deonctl retrieval context injection-governance-bundle \
  --governance-report artifacts/<run-id>/retrieval-context-governance-report.json \
  --policy configs/examples/retrieval-injection-policy.yaml \
  --injection-approval-request artifacts/<run-id>/retrieval-context-injection-approval-request.json \
  --injection-approval artifacts/<run-id>/retrieval-context-injection-approval.json \
  --execution-plan artifacts/<run-id>/retrieval-context-injection-execution-plan.json \
  --prompt-preview-manifest artifacts/<run-id>/retrieval-context-prompt-preview.json \
  --prompt-preview-report artifacts/<run-id>/retrieval-context-prompt-preview-report.json \
  --output artifacts/<run-id>/retrieval-context-injection-governance-bundle.json \
  --summary artifacts/<run-id>/retrieval-context-injection-governance-bundle.md
~~~

Rules:

- path hardening on all inputs and outputs
- validates governance-report status `ok` or `warning`, injection-policy schema, injection-approval inspect with request binding, execution-plan plan-only flags, prompt-preview manifest flags, and prompt-preview-report status `ok`
- validates hash coherence across approval, policy, governance-report, execution-plan, manifest, and prompt-preview-report
- requires `approval.runner_injection_allowed: true`, `execution_plan.would_execute_runner: false`, `execution_plan.execution_supported_now: false`, `manifest.preview_only: true`, `manifest.runner_execution: false`
- bundle output sets `contains_text: false`, `runner_execution: false`, `injection_authorized_for_future: true`, `execution_supported_now: false`
- bundle json and summary must not contain `text_excerpt` or preview chunk text

Optional fixture step 18 exercises this path after prompt-preview-report.

## Task declaration, no injection (Task 22.19)

Declarative task YAML for future governed materialized injection. Validates schema and governance bundle metadata only; does not read `prompt_preview` or inject text into runner prompts.

~~~yaml
retrieval_context:
  materialized_injection:
    enabled: false
    governance_bundle: artifacts/<run-id>/retrieval-context-injection-governance-bundle.json
    prompt_preview: artifacts/<run-id>/retrieval-context-prompt-preview.md
    require_confirm_flag: true
    max_total_chars: 6000
~~~

Rules:

- `enabled` must be `false` in Task 22.19; `enabled: true` fails validation with `materialized injection not supported yet`
- `governance_bundle` and `prompt_preview` must be relative safe paths; `prompt_preview` is not read during validation
- `require_confirm_flag` must be `true`; `max_total_chars` must be `> 0`
- governance bundle must have `contains_text: false`, `runner_execution: false`, `injection_authorized_for_future: true`, `execution_supported_now: false`
- `deonctl task validate` reports `materialized_injection_declared`, `materialized_injection_enabled`, `materialized_injection_supported_now`, and `governance_bundle_sha256` when declared
- runner and execution trace remain unchanged; no worker prompt injection

Optional example task: `configs/examples/retrieval-context-fixture/materialized-injection-task.yaml` (requires governance bundle artifact from fixture step 18).

## Materialized injection task report (Task 22.19.1)

QA/report over Task 22.19 task YAML declaration. Validates task schema and governance bundle metadata without reading `prompt_preview`.

~~~bash
deonctl task materialized-injection-report \
  --task configs/examples/retrieval-context-fixture/materialized-injection-task.yaml \
  --output-format json
~~~

Rules:

- runs `tasks.Validate` and `ValidateTaskMaterializedInjection`
- loads governance bundle and validates metadata-only flags (`contains_text: false`, `runner_execution: false`, `injection_authorized_for_future: true`, `execution_supported_now: false`)
- validates safe relative paths for `governance_bundle` and `prompt_preview`; does not read `prompt_preview`
- report sets `prompt_preview_read: false` and `runner_prompt_changed: false`
- report text/json must not contain `text_excerpt` or preview chunk text

Optional fixture step 19 exercises this path after task validate.

## Runner materialized injection preflight (Task 22.20)

Explicit preflight for future governed materialized injection before runner execution. Validates task declaration, governance bundle, and prompt-preview-report metadata without reading `prompt_preview` markdown.

~~~bash
deonctl worker codex materialized-injection-preflight \
  --task configs/examples/retrieval-context-fixture/materialized-injection-task.yaml \
  --prompt-preview-report retrieval-context-prompt-preview-report.json \
  --output artifacts/<run-id>/materialized-injection-preflight.json \
  --output-format json
~~~

Rules:

- runs `tasks.Validate`, `MaterializedInjectionTaskReport`, governance bundle load, and prompt-preview-report load
- requires `materialized_injection.enabled: false`, task report status `ok` or `warning`, `prompt_preview_read: false`, `runner_prompt_changed: false`
- validates governance bundle metadata-only flags and `prompt_preview_report.hashes.materialized_sha256 == governance_bundle.materialized_sha256`
- preflight sets `worker_execution_allowed: false`, `prompt_injection_allowed_now: false`, `required_future_flag: --confirm-inject-materialized-context`
- writes `materialized-injection-preflight.json`; stdout/json must not contain `text_excerpt` or preview chunk text
- runner prompt and worker execution remain unchanged

Optional fixture step 20 exercises this path after materialized-injection-report.

## Runner materialized injection dry-run (Task 22.21)

Dry-run prompt-section artifact showing how governed materialized context would attach to the runner prompt. Gated on preflight 22.20; reads `prompt_preview` markdown only in this command and only with explicit confirmation.

~~~bash
deonctl worker codex materialized-injection-dry-run \
  --task configs/examples/retrieval-context-fixture/materialized-injection-task.yaml \
  --preflight retrieval-context-materialized-injection-preflight.json \
  --prompt-preview retrieval-context-prompt-preview.md \
  --output artifacts/<run-id>/materialized-injection-dry-run.json \
  --prompt-output artifacts/<run-id>/materialized-injection-prompt-section.md \
  --confirm-inject-materialized-context \
  --output-format json
~~~

Rules:

- requires `--confirm-inject-materialized-context`; path hardening on all inputs and outputs
- runs `tasks.Validate`; requires `materialized_injection.enabled: false` and task `prompt_preview` path match
- loads preflight JSON: status `ok` or `warning`, `worker_execution_allowed: false`, `prompt_injection_allowed_now: false`, `materialized_injection_declared: true`, `materialized_sha256` present, `required_future_flag: --confirm-inject-materialized-context`
- does not recalculate hashes from source files; trusts preflight/report chain
- writes `materialized-injection-prompt-section.md` with dry-run header plus rendered preview section (`text_excerpt` allowed only here)
- writes `materialized-injection-dry-run.json` with `worker_execution: false`, `prompt_changed_in_real_runner: false`, `prompt_section_rendered: true`, `preview_only: true`, `confirm_flag_used: true`
- stdout text/json must not contain `text_excerpt` or preview chunk text
- runner prompt and worker execution remain unchanged

Optional fixture step 21 exercises this path after materialized-injection-preflight.

## Runner materialized injection dry-run report (Task 22.21.1)

QA/report over Task 22.21 dry-run artifacts. Validates dry-run JSON metadata and prompt-section markdown structure without printing chunk text.

~~~bash
deonctl worker codex materialized-injection-dry-run-report \
  --dry-run artifacts/<run-id>/materialized-injection-dry-run.json \
  --prompt-output artifacts/<run-id>/materialized-injection-prompt-section.md \
  --output-format json
~~~

Rules:

- path hardening on `--dry-run` and `--prompt-output`
- loads dry-run JSON and prompt-section markdown; dry-run file must not contain `text_excerpt` or preview chunk text
- validates dry-run `status` is `ok` or `warning`, `worker_execution: false`, `prompt_changed_in_real_runner: false`, `prompt_section_rendered: true`, `contains_text: true`, `preview_only: true`, `confirm_flag_used: true`, and `prompt_output_sha256` matches prompt-section file hash
- validates prompt-section contains dry-run notice and `text_excerpt`
- report stdout/json must not contain `text_excerpt` or preview chunk text; exit code 1 on failures
- runner prompt and worker execution remain unchanged

Optional fixture step 22 exercises this path after materialized-injection-dry-run.

## Runner materialized injection readiness report (Task 22.21.2)

Consolidates preflight, dry-run, and dry-run-report gates before any future real execution. Declares governance readiness while keeping execution disabled.

~~~bash
deonctl worker codex materialized-injection-readiness-report \
  --preflight artifacts/<run-id>/materialized-injection-preflight.json \
  --dry-run artifacts/<run-id>/materialized-injection-dry-run.json \
  --dry-run-report artifacts/<run-id>/materialized-injection-dry-run-report.json \
  --output-format json
~~~

Rules:

- path hardening on all three input JSON paths
- loads preflight, dry-run, and dry-run-report artifacts; none may contain `text_excerpt` or preview chunk text
- validates preflight `status` ok/warning, `worker_execution_allowed: false`, `prompt_injection_allowed_now: false`
- validates dry-run `status` ok/warning, `worker_execution: false`, `prompt_changed_in_real_runner: false`, `prompt_section_rendered: true`, `confirm_flag_used: true`
- validates dry-run-report `status` ok/warning, `worker_execution: false`, `prompt_changed_in_real_runner: false`, `prompt_section_rendered: true`
- validates `materialized_sha256` coherence across artifacts when present and `prompt_output_sha256` match between dry-run and dry-run-report
- sets `governance_ready_for_future_execution: true` on ok/warning, `execution_allowed_now: false`, `prompt_injection_allowed_now: false`, `worker_execution: false`, `required_future_flag: --confirm-inject-materialized-context`
- report stdout/json must not contain `text_excerpt` or preview chunk text; exit code 1 on failures
- runner prompt and worker execution remain unchanged

Optional fixture step 23 exercises this path after materialized-injection-dry-run-report.

## Runner materialized injection execution gate (Task 22.22)

Final gate between governance readiness and any future execution path. Loads task YAML, readiness report, and prompt-output metadata, validates every execution/injection invariant, and produces a metadata-only gate artifact. **Does not** execute the worker, attach the prompt-output to a real runner, change `taskForWorker`, or permit `enabled: true`.

~~~bash
deonctl worker codex materialized-injection-execution-gate \
  --task configs/examples/retrieval-context-fixture/materialized-injection-task.yaml \
  --readiness-report artifacts/<run-id>/materialized-injection-readiness-report.json \
  --prompt-output artifacts/<run-id>/materialized-injection-prompt-section.md \
  --output artifacts/<run-id>/materialized-injection-execution-gate.json \
  --confirm-inject-materialized-context \
  --output-format json
~~~

Rules:

- requires `--confirm-inject-materialized-context`; path hardening on all inputs/outputs
- runs `tasks.Validate`; requires `materialized_injection.enabled: false` and task `prompt_preview` declaration
- loads readiness report JSON: `status` ok/warning, `governance_ready_for_future_execution: true`, `execution_allowed_now: false`, `prompt_injection_allowed_now: false`, `worker_execution: false`, `required_future_flag: --confirm-inject-materialized-context`, `materialized_sha256` and `prompt_output_sha256` present
- reads prompt-output file only to compute its SHA256; content is never echoed or persisted in the gate artifact
- gate JSON sets `execution_gate_ready: true` when all gates pass, `implementation_allows_execution_now: false`, `worker_execution_allowed: false`, `prompt_injection_allowed_now: false`, `confirm_flag_used: true`, `blocked_reason: implementation_not_enabled`
- gate text/json output must not contain `text_excerpt` or alpha text; exit code 1 on `status: failed`
- runner prompt and worker execution remain unchanged

Optional fixture step 24 exercises this path after materialized-injection-readiness-report.

## Materialized prompt assembly dry-run (Task 22.23)

Adapter/skeleton showing how a future runner prompt integration would compose a base prompt fixture with the materialized prompt-output. Produces only a preview assembled markdown and metadata JSON; does not change the normal runner prompt or execute any worker.

~~~bash
deonctl worker codex materialized-prompt-assembly-dry-run \
  --execution-gate artifacts/<run-id>/materialized-injection-execution-gate.json \
  --base-prompt-fixture configs/examples/retrieval-context-fixture/base-worker-prompt-fixture.md \
  --prompt-output artifacts/<run-id>/materialized-injection-prompt-section.md \
  --output artifacts/<run-id>/materialized-prompt-assembly-dry-run.json \
  --assembled-output artifacts/<run-id>/materialized-prompt-assembly.md \
  --confirm-inject-materialized-context \
  --output-format json
~~~

Rules:

- requires `--confirm-inject-materialized-context`; path hardening on all inputs/outputs
- loads execution gate JSON: `status` ok/warning, `execution_gate_ready: true`, `implementation_allows_execution_now: false`, `worker_execution_allowed: false`, `prompt_injection_allowed_now: false`, `confirm_flag_used: true`, `blocked_reason: implementation_not_enabled`, `prompt_output_sha256` present
- validates prompt-output SHA256 against execution gate
- assembled markdown header reads `Dry-run only — assembled prompt was not sent to any worker.`, followed by a base prompt fixture section, a clear separator, and the materialized prompt-output section
- assembly dry-run JSON sets `assembled_prompt_rendered: true`, `worker_execution: false`, `sent_to_worker: false`, `prompt_changed_in_real_runner: false`, `contains_text: true`, `preview_only: true`, `confirm_flag_used: true`, `blocked_reason: implementation_not_enabled`, `base_prompt_sha256`, `prompt_output_sha256`, `assembled_output_sha256`, `materialized_sha256`
- assembly dry-run text/json output must not contain `text_excerpt` or alpha text; only the assembled-output markdown may contain `text_excerpt`
- normal runner prompt and worker execution remain unchanged

Optional fixture step 25 exercises this path after materialized-injection-execution-gate.

## Materialized prompt assembly report (Task 22.24)

QA/report over Task 22.23 assembly dry-run JSON and assembled markdown. Reads assembled markdown only to validate hash, dry-run notice, and `text_excerpt`; report output must not print assembled-output content.

~~~bash
deonctl worker codex materialized-prompt-assembly-report \
  --assembly-dry-run artifacts/<run-id>/materialized-prompt-assembly-dry-run.json \
  --assembled-output artifacts/<run-id>/materialized-prompt-assembly.md \
  --output-format json
~~~

Rules:

- path hardening on both input paths
- rejects assembly dry-run JSON containing `text_excerpt` or alpha text
- validates assembly `status` ok/warning, `assembled_prompt_rendered: true`, `worker_execution: false`, `sent_to_worker: false`, `prompt_changed_in_real_runner: false`, `contains_text: true`, `preview_only: true`, `confirm_flag_used: true`, `blocked_reason: implementation_not_enabled`
- validates `assembled_output_sha256 == sha256(assembled-output)` and that assembled-output contains the dry-run notice and `text_excerpt`
- report sets `assembled_prompt_validated: true`, `worker_execution: false`, `sent_to_worker: false`, `prompt_changed_in_real_runner: false`, `contains_text: true`, `preview_only: true`, `assembled_output_sha256`, `materialized_sha256`
- report text/json output must not contain `text_excerpt` or assembled-output content; exit code 1 on `status: failed`
- normal runner prompt and worker execution remain unchanged

Optional fixture step 26 exercises this path after materialized-prompt-assembly-dry-run.

## Materialized injection runtime config (Task 22.25)

Schema validation for future materialized injection runtime gates. Config must keep `enabled: false`, `allow_worker_execution: false`, `allow_prompt_injection: false`, and `blocked_reason: implementation_not_enabled`. Config and validate output must not contain `text_excerpt` or alpha text.

Example: `configs/examples/materialized-injection-runtime.yaml`

~~~bash
deonctl worker codex materialized-injection-runtime validate \
  --config configs/examples/materialized-injection-runtime.yaml \
  --output-format json
~~~

Rules:

- path hardening on `--config`
- rejects config containing `text_excerpt` or alpha text
- requires `require_confirm_flag`, `require_execution_gate`, and `require_assembly_report` all `true`
- requires `max_total_chars > 0`
- validate output echoes schema fields; exit code 1 on `status: failed`
- normal runner prompt and worker execution remain unchanged

Optional fixture step 27 exercises this path.

## Materialized injection run planner (Task 22.26)

Plan-only artifact binding task `retrieval_context.materialized_injection`, runtime config, execution gate, assembly report, and assembled-output hash. Requires `--confirm-inject-materialized-context`. Writes run-plan JSON; does not dispatch Codex or change the normal runner prompt.

~~~bash
deonctl worker codex materialized-injection-run-plan \
  --task examples/tasks/materialized-injection-task.yaml \
  --runtime-config configs/examples/materialized-injection-runtime.yaml \
  --execution-gate artifacts/<run-id>/materialized-injection-execution-gate.json \
  --assembly-report artifacts/<run-id>/materialized-prompt-assembly-report.json \
  --assembled-output artifacts/<run-id>/materialized-prompt-assembly.md \
  --output artifacts/<run-id>/materialized-injection-run-plan.json \
  --confirm-inject-materialized-context \
  --output-format json
~~~

Rules:

- path hardening on all input/output paths
- rejects gate/report JSON containing `text_excerpt` or alpha text
- validates task `materialized_injection` declaration with `enabled: false`
- validates runtime config via Task 22.25 rules
- validates execution gate `execution_gate_ready: true`, `implementation_allows_execution_now: false`, `worker_execution_allowed: false`, `prompt_injection_allowed_now: false`, `blocked_reason: implementation_not_enabled`
- validates assembly report `assembled_prompt_validated: true`, `worker_execution: false`, `sent_to_worker: false`, `prompt_changed_in_real_runner: false`, and `assembled_output_sha256` match
- on success sets `run_plan_ready: true`, `worker_execution_planned: false`, `prompt_injection_planned: false`, `implementation_allows_execution_now: false`, `blocked_reason: implementation_not_enabled`, `required_future_flag: --confirm-inject-materialized-context`
- plan text/json output must not contain `text_excerpt` or assembled-output content; exit code 1 on `status: failed`
- normal runner prompt and worker execution remain unchanged

Optional fixture step 28 exercises this path after materialized-prompt-assembly-report.

## Materialized injection execution enablement policy (Task 22.27)

Schema validation for future materialized injection execution enablement. Config must keep `enabled: false`, `allow_provider_call: false`, `allow_worker_execution: false`, `allow_prompt_injection: false`, and `blocked_reason: implementation_not_enabled`. Config and validate output must not contain `text_excerpt` or alpha text.

Example: `configs/examples/materialized-injection-execution-enable.yaml`

~~~bash
deonctl worker codex materialized-injection-execution-enable validate \
  --config configs/examples/materialized-injection-execution-enable.yaml \
  --output-format json
~~~

Rules:

- path hardening on `--config`
- rejects config containing `text_excerpt` or alpha text
- requires `require_confirm_flag`, `require_readiness_report`, `require_execution_gate`, and `require_assembly_report` all `true`
- requires `max_total_chars > 0`
- validate output echoes `enabled`, `allow_provider_call`, `allow_worker_execution`, `allow_prompt_injection`, `blocked_reason`, `max_total_chars`; exit code 1 on `status: failed`
- normal runner prompt and worker execution remain unchanged

Optional fixture step 29 exercises this path.

## Materialized injection provider run-plan (Task 22.28)

Plan-only artifact binding task `retrieval_context.materialized_injection`, execution enablement policy, execution gate, assembly report, and assembled-output hash. Requires `--confirm-inject-materialized-context`. Writes provider run-plan JSON; does not call Codex/OpenCode or any provider API.

~~~bash
deonctl worker codex materialized-injection-provider-run-plan \
  --task configs/examples/retrieval-context-fixture/materialized-injection-task.yaml \
  --enable-config configs/examples/materialized-injection-execution-enable.yaml \
  --execution-gate artifacts/<run-id>/materialized-injection-execution-gate.json \
  --assembly-report artifacts/<run-id>/materialized-prompt-assembly-report.json \
  --assembled-output artifacts/<run-id>/materialized-prompt-assembly.md \
  --output artifacts/<run-id>/materialized-injection-provider-run-plan.json \
  --confirm-inject-materialized-context \
  --output-format json
~~~

Rules:

- path hardening on all input/output paths
- rejects gate/report JSON containing `text_excerpt` or alpha text
- validates task `materialized_injection` declaration with `enabled: false`
- validates enablement config via Task 22.27 rules (`enabled: false`, `allow_provider_call: false`, `allow_worker_execution: false`, `allow_prompt_injection: false`)
- validates execution gate `execution_gate_ready: true`, `implementation_allows_execution_now: false`, `worker_execution_allowed: false`, `prompt_injection_allowed_now: false`, `blocked_reason: implementation_not_enabled`
- validates assembly report `assembled_prompt_validated: true`, `worker_execution: false`, `sent_to_worker: false`, `prompt_changed_in_real_runner: false`, and `assembled_output_sha256` match
- on success sets `provider_run_plan_ready: true`, `would_use_assembled_prompt: true`, `assembled_prompt_validated: true`, `sent_to_provider: false`, `provider_call_allowed_now: false`, `worker_execution_allowed_now: false`, `prompt_injection_allowed_now: false`, `blocked_reason: implementation_not_enabled`
- plan text/json output must not contain `text_excerpt` or assembled-output content; exit code 1 on `status: failed`
- normal runner prompt, worker execution, and provider calls remain unchanged

Optional fixture step 30 exercises this path after execution enablement validate.

## Materialized provider dispatch contract (Task 22.29)

Schema validation for future provider dispatch gates. Config must keep `enabled: false`, `provider: codex`, `allow_provider_call: false`, `allow_network: false`, and `blocked_reason: implementation_not_enabled`. Config and validate output must not contain `text_excerpt` or alpha text.

Example: `configs/examples/materialized-provider-dispatch.yaml`

~~~bash
deonctl worker codex materialized-provider-dispatch validate \
  --config configs/examples/materialized-provider-dispatch.yaml \
  --output-format json
~~~

Rules:

- path hardening on `--config`
- rejects config containing `text_excerpt` or alpha text
- requires `require_confirm_flag`, `require_provider_run_plan`, and `require_assembled_prompt` all `true`
- requires `max_total_chars > 0` and `max_payload_bytes > 0`
- validate output echoes schema fields; exit code 1 on `status: failed`
- normal runner prompt, worker execution, and provider calls remain unchanged

Optional fixture step 31 exercises this path.

## Materialized provider payload dry-run (Task 22.30)

Dry-run provider payload artifact from validated provider run-plan and assembled-output. Requires `--confirm-inject-materialized-context`. Writes payload dry-run JSON and payload-output markdown; does not call Codex/OpenCode or any provider API.

~~~bash
deonctl worker codex materialized-provider-payload-dry-run \
  --dispatch-config configs/examples/materialized-provider-dispatch.yaml \
  --provider-run-plan artifacts/<run-id>/materialized-injection-provider-run-plan.json \
  --assembled-output artifacts/<run-id>/materialized-prompt-assembly.md \
  --output artifacts/<run-id>/materialized-provider-payload-dry-run.json \
  --payload-output artifacts/<run-id>/materialized-provider-payload.md \
  --confirm-inject-materialized-context \
  --output-format json
~~~

Rules:

- path hardening on all input/output paths
- validates dispatch config via Task 22.29 rules
- validates provider run-plan `provider_run_plan_ready: true`, `provider_call_allowed_now: false`, `sent_to_provider: false`, `would_use_assembled_prompt: true`, `assembled_prompt_validated: true`, `blocked_reason: implementation_not_enabled`
- validates `assembled_output_sha256` match and size caps (`max_total_chars`, `max_payload_bytes`)
- on success sets `provider_payload_rendered: true`, `provider_call: false`, `network_call: false`, `sent_to_provider: false`, `worker_execution: false`, `prompt_injection_real_runner: false`
- dry-run JSON/stdout must not contain payload content, `text_excerpt`, or alpha text; only `--payload-output` may contain `text_excerpt`
- normal runner prompt, worker execution, and provider calls remain unchanged

Optional fixture step 32 exercises this path after provider dispatch validate.

## Materialized provider payload report (Task 22.31)

QA report for provider payload dry-run JSON and payload-output hash. Does not print payload content.

~~~bash
deonctl worker codex materialized-provider-payload-report \
  --payload-dry-run artifacts/<run-id>/materialized-provider-payload-dry-run.json \
  --payload-output artifacts/<run-id>/materialized-provider-payload.md \
  --output-format json
~~~

Rules:

- path hardening on both input paths
- rejects payload dry-run JSON containing `text_excerpt` or alpha text
- validates payload `provider_payload_rendered: true`, `provider_call: false`, `network_call: false`, `sent_to_provider: false`, `worker_execution: false`, `prompt_injection_real_runner: false`, `blocked_reason: implementation_not_enabled`
- validates `provider_payload_sha256 == sha256(payload-output)` and payload-output contains dry-run notice and `text_excerpt`
- report sets `provider_payload_validated: true`; report text/json must not contain payload content; exit code 1 on `status: failed`
- normal runner prompt, worker execution, and provider calls remain unchanged

Optional fixture step 33 exercises this path after provider payload dry-run.

## Materialized provider call gate (Task 22.32)

Final metadata-only gate before any future provider call. Binds dispatch config, payload report, and payload-output hash. Requires `--confirm-inject-materialized-context`. Writes call gate JSON; does not call Codex/OpenCode or any provider API.

~~~bash
deonctl worker codex materialized-provider-call-gate \
  --dispatch-config configs/examples/materialized-provider-dispatch.yaml \
  --payload-report artifacts/<run-id>/materialized-provider-payload-report.json \
  --payload-output artifacts/<run-id>/materialized-provider-payload.md \
  --confirm-inject-materialized-context \
  --output artifacts/<run-id>/materialized-provider-call-gate.json \
  --output-format json
~~~

Rules:

- path hardening on all input/output paths
- validates dispatch config via Task 22.29 rules
- validates payload report `provider_payload_validated: true`, `provider_call: false`, `network_call: false`, `sent_to_provider: false`, `prompt_injection_real_runner: false`
- validates `provider_payload_sha256` match with payload-output (hash only; does not print payload content)
- on success sets `provider_call_gate_ready: true`, `provider_call_allowed_now: false`, `network_call_allowed_now: false`, `worker_execution_allowed_now: false`, `prompt_injection_allowed_now: false`, `payload_ready_for_future_call: true`, `sent_to_provider: false`, `blocked_reason: implementation_not_enabled`
- gate text/json output must not contain `text_excerpt` or payload content; exit code 1 on `status: failed`
- normal runner prompt, worker execution, and provider calls remain unchanged

Optional fixture step 34 exercises this path after provider payload report.

## Materialized provider call readiness report (Task 22.33)

Consolidates provider call gate and payload report into a readiness artifact. Does not call providers or print payload content.

~~~bash
deonctl worker codex materialized-provider-call-readiness-report \
  --provider-call-gate artifacts/<run-id>/materialized-provider-call-gate.json \
  --payload-report artifacts/<run-id>/materialized-provider-payload-report.json \
  --output-format json
~~~

Rules:

- path hardening on both input paths
- rejects gate/report JSON containing `text_excerpt` or alpha text
- validates gate `provider_call_gate_ready: true`, `provider_call_allowed_now: false`, `network_call_allowed_now: false`, `payload_ready_for_future_call: true`, `sent_to_provider: false`, `blocked_reason: implementation_not_enabled`
- validates payload report `provider_payload_validated: true`, `provider_call: false`, `network_call: false`, `sent_to_provider: false`
- validates `materialized_sha256` and `provider_payload_sha256` coherence between gate and report when present
- on success sets `provider_call_readiness_ready: true`, `provider_call_allowed_now: false`, `network_call_allowed_now: false`, `sent_to_provider: false`, `blocked_reason: implementation_not_enabled`
- report text/json must not contain `text_excerpt` or payload content; exit code 1 on `status: failed`
- normal runner prompt, worker execution, and provider calls remain unchanged

Optional fixture step 35 exercises this path after provider call gate.

## Provider call approval (Task 22.34)

Explicit approval workflow for future provider calls. Binds `provider-call-readiness-report`, `provider-call-gate`, and `payload-report` metadata. Requires `--confirm-provider-payload-sha256` on approve matching `provider_payload_sha256` from the payload report. Does **not** read or print `payload-output` content.

~~~bash
deonctl worker codex provider-call-approval new \
  --readiness-report artifacts/<run-id>/provider-call-readiness-report.json \
  --provider-call-gate artifacts/<run-id>/provider-call-gate.json \
  --payload-report artifacts/<run-id>/payload-report.json \
  --output artifacts/<run-id>/provider-call-approval-request.json

deonctl worker codex provider-call-approval approve \
  --request artifacts/<run-id>/provider-call-approval-request.json \
  --output artifacts/<run-id>/provider-call-approval.json \
  --confirm-provider-payload-sha256 <sha256-from-payload-report>

deonctl worker codex provider-call-approval inspect \
  --approval artifacts/<run-id>/provider-call-approval.json \
  --request artifacts/<run-id>/provider-call-approval-request.json \
  --output-format json
~~~

Rules:

- path hardening on all input/output paths
- rejects readiness/gate/payload-report JSON containing `text_excerpt` or alpha text
- validates readiness report `provider_call_readiness_ready: true`, `provider_call_allowed_now: false`, `sent_to_provider: false`
- validates gate `provider_call_gate_ready: true`, `provider_call_allowed_now: false`, `sent_to_provider: false`, `blocked_reason: implementation_not_enabled`
- validates payload report `provider_payload_validated: true`, `sent_to_provider: false`, `provider_payload_sha256` present
- cross-checks `materialized_sha256`, `payload_report_sha256`, `provider_payload_sha256`, and `provider_call_gate_sha256` across artifacts
- approval sets `provider_call_authorized_for_future: true` while keeping `provider_call_allowed_now: false` and `sent_to_provider: false`
- approval/inspect output must not contain `text_excerpt` or payload-output content
- no provider call, no network, no worker execution

## Provider call execution bundle (Task 22.35)

Final metadata-only consolidation before any future provider call. Binds `dispatch-config`, `payload-report`, `provider-call-gate`, `provider-call-readiness-report`, and `provider-call-approval`. Does not read `payload-output`, does not call providers, and does not dispatch workers.

Example dispatch config: `configs/examples/materialized-provider-dispatch.yaml`

~~~bash
deonctl worker codex provider-call-execution-bundle \
  --dispatch-config configs/examples/materialized-provider-dispatch.yaml \
  --payload-report artifacts/<run-id>/payload-report.json \
  --provider-call-gate artifacts/<run-id>/provider-call-gate.json \
  --readiness-report artifacts/<run-id>/provider-call-readiness-report.json \
  --approval artifacts/<run-id>/provider-call-approval.json \
  --output artifacts/<run-id>/provider-call-execution-bundle.json \
  --output-format json
~~~

Rules:

- path hardening on all input/output paths
- validates dispatch config `enabled: false`, `allow_provider_call: false`, `allow_network: false`, `allow_worker_execution: false`, `blocked_reason: implementation_not_enabled`
- validates payload report, gate, readiness report, and approval via Task 22.34 rules
- cross-checks hashes across dispatch config, reports, gate, readiness, and approval
- on success sets `provider_call_authorized_for_future: true`, `provider_call_allowed_now: false`, `sent_to_provider: false`, `provider_call: false`, `network_call: false`, `worker_execution: false`, `execution_supported_now: false`, `contains_text: false`, `blocked_reason: implementation_not_enabled`
- bundle text/json output must not contain `text_excerpt` or payload-output content; exit code 1 on `status: failed`
- no provider call, no network, no worker execution

Optional fixture step 37 exercises this path after provider call approval.

## Provider call chain continuity audit (Task 22.36)

Reconciles the full provider-call artifact chain from dispatch validate through execution bundle. Confirms producer commands (Tasks 22.29–22.33) are active and loaders match artifact schemas. Does not call providers, read payload-output content, or dispatch workers.

~~~bash
deonctl worker codex provider-call-chain-audit \
  --dispatch-config configs/examples/materialized-provider-dispatch.yaml \
  --provider-run-plan artifacts/<run-id>/materialized-injection-provider-run-plan.json \
  --payload-dry-run artifacts/<run-id>/materialized-provider-payload-dry-run.json \
  --payload-output artifacts/<run-id>/materialized-provider-payload.md \
  --payload-report artifacts/<run-id>/materialized-provider-payload-report.json \
  --provider-call-gate artifacts/<run-id>/materialized-provider-call-gate.json \
  --readiness-report artifacts/<run-id>/materialized-provider-call-readiness-report.json \
  --approval-request artifacts/<run-id>/provider-call-approval-request.json \
  --approval artifacts/<run-id>/provider-call-approval.json \
  --execution-bundle artifacts/<run-id>/provider-call-execution-bundle.json \
  --output-format json
~~~

Rules:

- path hardening on all input paths
- validates each chain step via Tasks 22.29–22.35 loaders and flags
- on success sets `chain_continuity_ready: true`, `loaders_reconciled: true`, `producer_commands_active: true`
- keeps `provider_call: false`, `network_call: false`, `worker_execution: false`, `sent_to_provider: false` across the chain
- audit text/json must not contain `text_excerpt` or payload content; exit code 1 on `status: failed`
- no provider call, no network, no worker execution

Optional fixture step 38 exercises this path after provider call execution bundle.

## Provider call executor skeleton (Tasks 22.38–22.39)

Re-validates execution bundle, chain audit, approval, and executor policy config. Does not call providers, open network connections, or dispatch workers.

### Executor policy config validate (Task 22.39)

~~~bash
deonctl worker codex provider-call-executor config validate \
  --config configs/examples/provider-call-executor.yaml \
  --output-format json
~~~

Rules:

- path hardening on config path
- validates `enabled: false`, all `allow_*: false`, required artifact flags true, `blocked_reason: implementation_not_enabled`
- config must not contain `text_excerpt` or payload content; exit code 1 on `status: failed`

### Executor validate / plan (Tasks 22.38–22.39)

~~~bash
deonctl worker codex provider-call-executor validate \
  --executor-config configs/examples/provider-call-executor.yaml \
  --dispatch-config configs/examples/materialized-provider-dispatch.yaml \
  --provider-run-plan artifacts/<run-id>/materialized-injection-provider-run-plan.json \
  --payload-dry-run artifacts/<run-id>/materialized-provider-payload-dry-run.json \
  --payload-output artifacts/<run-id>/materialized-provider-payload.md \
  --payload-report artifacts/<run-id>/materialized-provider-payload-report.json \
  --provider-call-gate artifacts/<run-id>/materialized-provider-call-gate.json \
  --readiness-report artifacts/<run-id>/materialized-provider-call-readiness-report.json \
  --approval-request artifacts/<run-id>/provider-call-approval-request.json \
  --approval artifacts/<run-id>/provider-call-approval.json \
  --execution-bundle artifacts/<run-id>/provider-call-execution-bundle.json \
  --output-format json

deonctl worker codex provider-call-executor plan \
  ...same flags... \
  --output artifacts/<run-id>/provider-call-executor-plan.json
~~~

Rules:

- requires validated executor policy config plus full chain artifact paths
- re-loads execution bundle, runs chain audit, inspects approval, reconciles hashes
- on success sets `executor_policy_validated: true`, `executor_config_validated: true`, `provider_call_authorized_for_future: true`, `chain_continuity_ready: true`
- keeps `execution_supported_now: false`, `provider_call_allowed_now: false`, `sent_to_provider: false`, all execution flags false, `blocked_reason: implementation_not_enabled`
- plan writes metadata-only JSON artifact; stdout/json must not contain `text_excerpt` or payload content; exit code 1 on `status: failed`
- no provider call, no network, no worker execution

Optional fixture steps 39–40 exercise config validate and executor validate/plan after chain audit.

### Executor dry-run / dry-run report (Task 22.40)

~~~bash
deonctl worker codex provider-call-executor dry-run \
  --executor-config configs/examples/provider-call-executor.yaml \
  --dispatch-config configs/examples/materialized-provider-dispatch.yaml \
  --provider-run-plan artifacts/<run-id>/materialized-injection-provider-run-plan.json \
  --payload-dry-run artifacts/<run-id>/materialized-provider-payload-dry-run.json \
  --payload-report artifacts/<run-id>/materialized-provider-payload-report.json \
  --provider-call-gate artifacts/<run-id>/materialized-provider-call-gate.json \
  --readiness-report artifacts/<run-id>/materialized-provider-call-readiness-report.json \
  --approval-request artifacts/<run-id>/provider-call-approval-request.json \
  --approval artifacts/<run-id>/provider-call-approval.json \
  --execution-bundle artifacts/<run-id>/provider-call-execution-bundle.json \
  --output artifacts/<run-id>/provider-call-executor-dry-run.json \
  --output-format json

deonctl worker codex provider-call-executor dry-run-report \
  --dry-run artifacts/<run-id>/provider-call-executor-dry-run.json \
  --executor-config configs/examples/provider-call-executor.yaml \
  --output-format json
~~~

Rules:

- does not read or print payload markdown; chain audit runs in metadata-only mode
- does not call `transport.Deliver`
- on success sets `executor_dry_run_ready: true`, `transport_called: false`, `contains_text: false`, `preview_only: true`
- keeps all execution flags false and `blocked_reason: implementation_not_enabled`
- dry-run-report validates hashes, policy, and blocked flags on the dry-run artifact
- stdout/json must not contain `text_excerpt` or payload content; exit code 1 on `status: failed`

Optional fixture steps 41–42 exercise dry-run and dry-run-report after executor validate/plan.

### Executor preflight (Task 22.41)

~~~bash
deonctl worker codex provider-call-executor preflight \
  --executor-config configs/examples/provider-call-executor.yaml \
  --dry-run artifacts/<run-id>/provider-call-executor-dry-run.json \
  --dry-run-report artifacts/<run-id>/provider-call-executor-dry-run-report.json \
  --execution-bundle artifacts/<run-id>/provider-call-execution-bundle.json \
  --approval artifacts/<run-id>/provider-call-approval.json \
  --output artifacts/<run-id>/provider-call-executor-preflight.json \
  --output-format json
~~~

Rules:

- validates executor-config, dry-run-report, execution-bundle, and approval; reconciles hashes
- does not read payload markdown or call transport
- on success sets `executor_preflight_ready: true`, `execution_allowed_now: false`, `required_future_confirm_flag: --confirm-provider-executor-dispatch`
- keeps all execution flags false and `blocked_reason: implementation_not_enabled`

### Dispatch approval (Task 22.42)

~~~bash
deonctl worker codex provider-call-executor-dispatch-approval new \
  --executor-config configs/examples/provider-call-executor.yaml \
  --dry-run artifacts/<run-id>/provider-call-executor-dry-run.json \
  --dry-run-report artifacts/<run-id>/provider-call-executor-dry-run-report.json \
  --preflight artifacts/<run-id>/provider-call-executor-preflight.json \
  --execution-bundle artifacts/<run-id>/provider-call-execution-bundle.json \
  --approval artifacts/<run-id>/provider-call-approval.json \
  --output artifacts/<run-id>/provider-call-executor-dispatch-approval-request.json

deonctl worker codex provider-call-executor-dispatch-approval approve \
  --request artifacts/<run-id>/provider-call-executor-dispatch-approval-request.json \
  --output artifacts/<run-id>/provider-call-executor-dispatch-approval.json \
  --confirm-executor-config-sha256 <sha256> \
  --confirm-execution-bundle-sha256 <sha256> \
  --confirm-provider-payload-sha256 <sha256>

deonctl worker codex provider-call-executor-dispatch-approval inspect \
  --approval artifacts/<run-id>/provider-call-executor-dispatch-approval.json \
  --request artifacts/<run-id>/provider-call-executor-dispatch-approval-request.json \
  --output-format json
~~~

Rules:

- separate from payload approval (`provider-call-approval`); authorize future dispatch only
- approve requires explicit confirm hashes; sets `dispatch_authorized_for_future: true`, `dispatch_allowed_now: false`
- no provider call, no network, no transport

### Transport plan (Task 22.43)

~~~bash
deonctl worker codex provider-transport-plan \
  --executor-config configs/examples/provider-call-executor.yaml \
  --executor-preflight artifacts/<run-id>/provider-call-executor-preflight.json \
  --dispatch-approval artifacts/<run-id>/provider-call-executor-dispatch-approval.json \
  --output artifacts/<run-id>/provider-transport-plan.json \
  --output-format json
~~~

Rules:

- metadata-only transport layer plan using `BlockedProviderTransport`
- does not instantiate SDK, open network, or call `Deliver`
- on success sets `transport_plan_ready: true`, `transport_enabled: false`, all execution flags false

### Release bundle / gate (Task 22.44)

~~~bash
deonctl worker codex provider-executor-release-bundle \
  --executor-config configs/examples/provider-call-executor.yaml \
  --execution-bundle artifacts/<run-id>/provider-call-execution-bundle.json \
  --dry-run artifacts/<run-id>/provider-call-executor-dry-run.json \
  --dry-run-report artifacts/<run-id>/provider-call-executor-dry-run-report.json \
  --executor-preflight artifacts/<run-id>/provider-call-executor-preflight.json \
  --dispatch-approval artifacts/<run-id>/provider-call-executor-dispatch-approval.json \
  --transport-plan artifacts/<run-id>/provider-transport-plan.json \
  --output artifacts/<run-id>/provider-executor-release-bundle.json \
  --output-format json

deonctl worker codex provider-executor-release-gate \
  --executor-config configs/examples/provider-call-executor.yaml \
  --execution-bundle artifacts/<run-id>/provider-call-execution-bundle.json \
  --dry-run artifacts/<run-id>/provider-call-executor-dry-run.json \
  --dry-run-report artifacts/<run-id>/provider-call-executor-dry-run-report.json \
  --executor-preflight artifacts/<run-id>/provider-call-executor-preflight.json \
  --dispatch-approval artifacts/<run-id>/provider-call-executor-dispatch-approval.json \
  --transport-plan artifacts/<run-id>/provider-transport-plan.json \
  --release-bundle artifacts/<run-id>/provider-executor-release-bundle.json \
  --output-format json
~~~

Rules:

- consolidates artifacts 22.38–22.43; revalidates hashes and blocked flags
- release gate sets `activation_gate_ready: true`, `activation_allowed_now: false`, `execution_supported_now: false`
- no provider call, no network, no worker execution, no transport

Optional fixture steps 43–50 exercise preflight through release gate and final smoke.

### Request envelope (Task 22.45)

~~~bash
deonctl worker codex provider-request-envelope dry-run \
  --executor-config configs/examples/provider-call-executor.yaml \
  --execution-bundle artifacts/<run-id>/provider-call-execution-bundle.json \
  --executor-preflight artifacts/<run-id>/provider-call-executor-preflight.json \
  --dispatch-approval artifacts/<run-id>/provider-call-executor-dispatch-approval.json \
  --transport-plan artifacts/<run-id>/provider-transport-plan.json \
  --release-bundle artifacts/<run-id>/provider-executor-release-bundle.json \
  --release-gate artifacts/<run-id>/provider-executor-release-gate.json \
  --output artifacts/<run-id>/provider-request-envelope.json \
  --output-format json

deonctl worker codex provider-request-envelope report \
  --request-envelope artifacts/<run-id>/provider-request-envelope.json \
  ...same chain paths... \
  --output-format json
~~~

Rules:

- metadata-only future request envelope; hash references only
- does not read payload markdown or include request text
- sets `provider_request_ready: true`, `request_contains_text: false`, `sent_to_provider: false`

### Adapter registry / plan (Task 22.46)

~~~bash
deonctl worker codex provider-adapter-registry validate \
  --config configs/examples/provider-adapters.yaml \
  --output-format json

deonctl worker codex provider-adapter-plan \
  --config configs/examples/provider-adapters.yaml \
  --request-envelope artifacts/<run-id>/provider-request-envelope.json \
  --output artifacts/<run-id>/provider-adapter-plan.json \
  --output-format json
~~~

Rules:

- schema-only adapter registry; codex adapter blocked for future use
- no SDK, no network, no secrets required
- sets `adapter_available_for_future: true`, `adapter_enabled_now: false`, `transport_enabled: false`

### Response fixture (Task 22.47)

~~~bash
deonctl worker codex provider-response-fixture generate \
  --request-envelope artifacts/<run-id>/provider-request-envelope.json \
  --adapter-plan artifacts/<run-id>/provider-adapter-plan.json \
  --output artifacts/<run-id>/provider-response-fixture.json \
  --output-format json

deonctl worker codex provider-response-fixture inspect \
  --fixture artifacts/<run-id>/provider-response-fixture.json \
  --request-envelope artifacts/<run-id>/provider-request-envelope.json \
  --adapter-plan artifacts/<run-id>/provider-adapter-plan.json \
  --output-format json
~~~

Rules:

- deterministic fake response from envelope + adapter plan hashes
- `response_source: fixture`, `received_from_provider: false`
- no diff application, no worker execution

### Execution simulation (Task 22.48)

~~~bash
deonctl worker codex provider-execution-simulation-bundle \
  --request-envelope artifacts/<run-id>/provider-request-envelope.json \
  --adapter-plan artifacts/<run-id>/provider-adapter-plan.json \
  --response-fixture artifacts/<run-id>/provider-response-fixture.json \
  --release-gate artifacts/<run-id>/provider-executor-release-gate.json \
  --executor-config configs/examples/provider-call-executor.yaml \
  --output artifacts/<run-id>/provider-execution-simulation-bundle.json \
  --output-format json

deonctl worker codex provider-execution-simulation-report \
  --simulation-bundle artifacts/<run-id>/provider-execution-simulation-bundle.json \
  ...same simulation inputs... \
  --output-format json
~~~

Rules:

- consolidates envelope + adapter plan + fixture response
- validates hashes and blocked transport flags
- sets `simulation_bundle_ready: true`, `execution_result_available: false`
- no workspace changes, no PR creation

Optional fixture steps 51–66 exercise request envelope through activation readiness report.

### Credential policy (Task 22.49)

~~~bash
deonctl worker codex provider-credential-policy validate \
  --config configs/examples/provider-credential-policy.yaml \
  --output-format json

deonctl worker codex provider-credential-policy plan \
  --config configs/examples/provider-credential-policy.yaml \
  --output artifacts/<run-id>/provider-credential-policy-plan.json \
  --output-format json
~~~

Rules:

- declares allowed env var names (e.g. `CODEX_API_KEY`, `OPENAI_API_KEY`) without reading secret values
- `credential_check_enabled: false`, `allow_secret_read: false`, `allow_network: false`
- metadata-only plan; `secret_values_read: false`

### Real-call proposal (Task 22.50)

~~~bash
deonctl worker codex provider-real-call-proposal new \
  --execution-simulation-report artifacts/<run-id>/provider-execution-simulation-report.json \
  --request-envelope artifacts/<run-id>/provider-request-envelope.json \
  --adapter-plan artifacts/<run-id>/provider-adapter-plan.json \
  --credential-policy-plan artifacts/<run-id>/provider-credential-policy-plan.json \
  --release-gate artifacts/<run-id>/provider-executor-release-gate.json \
  --output artifacts/<run-id>/provider-real-call-proposal.json

deonctl worker codex provider-real-call-proposal inspect \
  --proposal artifacts/<run-id>/provider-real-call-proposal.json \
  ...same inputs... \
  --output-format json
~~~

Rules:

- consolidates simulation + readiness hashes
- `would_call_provider_if_enabled: true`, `provider_call_allowed_now: false`
- no transport, no secrets, no execution confirm flag

### Response change proposal (Task 22.51)

~~~bash
deonctl worker codex provider-response-change-proposal \
  --response-fixture artifacts/<run-id>/provider-response-fixture.json \
  --execution-simulation-report artifacts/<run-id>/provider-execution-simulation-report.json \
  --real-call-proposal artifacts/<run-id>/provider-real-call-proposal.json \
  --output artifacts/<run-id>/provider-response-change-proposal.json \
  --output-format json

deonctl worker codex provider-response-change-proposal-report \
  --change-proposal artifacts/<run-id>/provider-response-change-proposal.json \
  ...same inputs... \
  --output-format json
~~~

Rules:

- fixture-only response source; no diff application, no workspace modification
- metadata-only file intent; `workspace_modified: false`, `diff_applied: false`

### Activation readiness audit (Task 22.52)

~~~bash
deonctl worker codex provider-activation-readiness-audit \
  --credential-policy-plan artifacts/<run-id>/provider-credential-policy-plan.json \
  --real-call-proposal artifacts/<run-id>/provider-real-call-proposal.json \
  --change-proposal artifacts/<run-id>/provider-response-change-proposal.json \
  --execution-simulation-report artifacts/<run-id>/provider-execution-simulation-report.json \
  --release-gate artifacts/<run-id>/provider-executor-release-gate.json \
  --output-format json

deonctl worker codex provider-activation-readiness-report \
  ...same inputs... \
  --output-format json
~~~

Rules:

- revalidates chain 22.29–22.51
- `activation_readiness_ready: true`, `activation_allowed_now: false`, `blocked_reason: implementation_not_enabled`
- no provider call, network, transport, secrets, or workspace changes

### Activation policy (Task 22.53)

~~~bash
deonctl worker codex provider-activation-policy validate \
  --config configs/examples/provider-activation-policy.yaml \
  --output-format json

deonctl worker codex provider-activation-policy plan \
  --config configs/examples/provider-activation-policy.yaml \
  --output artifacts/<run-id>/provider-activation-policy-plan.json \
  --output-format json
~~~

Rules:

- `enabled: false`; all `allow_*: false`
- requires readiness audit, real-call proposal, credential policy, simulation report, release gate, and manual approval flags true in schema
- metadata-only plan; `activation_allowed_now: false`

### Activation approval (Task 22.54)

~~~bash
deonctl worker codex provider-activation-approval new \
  --activation-policy-plan artifacts/<run-id>/provider-activation-policy-plan.json \
  --activation-readiness-audit artifacts/<run-id>/provider-activation-readiness-audit.json \
  --real-call-proposal artifacts/<run-id>/provider-real-call-proposal.json \
  --credential-policy-plan artifacts/<run-id>/provider-credential-policy-plan.json \
  --execution-simulation-report artifacts/<run-id>/provider-execution-simulation-report.json \
  --output artifacts/<run-id>/provider-activation-approval-request.json

deonctl worker codex provider-activation-approval approve \
  --request artifacts/<run-id>/provider-activation-approval-request.json \
  --output artifacts/<run-id>/provider-activation-approval.json \
  --confirm-activation-policy-sha256 <sha256> \
  --confirm-readiness-audit-sha256 <sha256> \
  --confirm-real-call-proposal-sha256 <sha256> \
  --confirm-provider-payload-sha256 <sha256>

deonctl worker codex provider-activation-approval inspect \
  --approval artifacts/<run-id>/provider-activation-approval.json \
  --request artifacts/<run-id>/provider-activation-approval-request.json \
  --output-format json
~~~

Rules:

- binds policy plan, readiness audit, real-call proposal, credential plan, and simulation report hashes
- approve requires four explicit confirm hashes; `activation_authorized_for_future: true`, `activation_allowed_now: false`
- `allowed_use: provider_activation_policy_only`

### Activation rehearsal (Task 22.55)

~~~bash
deonctl worker codex provider-activation-rehearsal \
  --activation-policy-plan artifacts/<run-id>/provider-activation-policy-plan.json \
  --activation-approval artifacts/<run-id>/provider-activation-approval.json \
  --activation-readiness-audit artifacts/<run-id>/provider-activation-readiness-audit.json \
  --real-call-proposal artifacts/<run-id>/provider-real-call-proposal.json \
  --credential-policy-plan artifacts/<run-id>/provider-credential-policy-plan.json \
  --execution-simulation-report artifacts/<run-id>/provider-execution-simulation-report.json \
  --output artifacts/<run-id>/provider-activation-rehearsal.json \
  --output-format json

deonctl worker codex provider-activation-rehearsal-report \
  --rehearsal artifacts/<run-id>/provider-activation-rehearsal.json \
  ...same inputs... \
  --output-format json
~~~

Rules:

- metadata-only sequence validation with `future_steps`; no provider call or transport
- `activation_rehearsal_ready: true`, `activation_allowed_now: false`

### Activation release package / gate (Task 22.56)

~~~bash
deonctl worker codex provider-activation-release-package \
  --activation-policy-plan artifacts/<run-id>/provider-activation-policy-plan.json \
  --activation-approval artifacts/<run-id>/provider-activation-approval.json \
  --activation-rehearsal artifacts/<run-id>/provider-activation-rehearsal.json \
  --activation-readiness-audit artifacts/<run-id>/provider-activation-readiness-audit.json \
  --real-call-proposal artifacts/<run-id>/provider-real-call-proposal.json \
  --execution-simulation-report artifacts/<run-id>/provider-execution-simulation-report.json \
  --credential-policy-plan artifacts/<run-id>/provider-credential-policy-plan.json \
  --response-change-proposal-report artifacts/<run-id>/provider-response-change-proposal-report.json \
  --output artifacts/<run-id>/provider-activation-release-package.json \
  --output-format json

deonctl worker codex provider-activation-release-gate \
  ...same inputs... \
  --activation-release-package artifacts/<run-id>/provider-activation-release-package.json \
  --output-format json
~~~

Rules:

- last gate before any real provider dispatch
- `activation_gate_ready: true`, `real_activation_supported_now: false`, `activation_allowed_now: false`
- consolidates activation policy, approval, rehearsal, readiness audit, simulation, credential, and change-proposal report hashes

### Operator review bundle / report (Task 22.58)

~~~bash
deonctl worker codex provider-activation-operator-review-bundle \
  --activation-release-package artifacts/<run-id>/provider-activation-release-package.json \
  --activation-release-gate artifacts/<run-id>/provider-activation-release-gate.json \
  ...release gate inputs... \
  --output artifacts/<run-id>/provider-activation-operator-review-bundle.json \
  --output-format json

deonctl worker codex provider-activation-operator-review-report \
  --operator-review-bundle artifacts/<run-id>/provider-activation-operator-review-bundle.json \
  ...same inputs... \
  --output-format json
~~~

Rules:

- metadata-only human review bundle; no payload text or secrets
- `operator_review_required: true`, `operator_approved_now: false`

### Kill-switch validate / plan (Task 22.59)

~~~bash
deonctl worker codex provider-activation-kill-switch validate \
  --config configs/examples/provider-activation-kill-switch.yaml \
  --output-format json

deonctl worker codex provider-activation-kill-switch plan \
  --config configs/examples/provider-activation-kill-switch.yaml \
  --output artifacts/<run-id>/provider-activation-kill-switch-plan.json \
  --output-format json
~~~

Rules:

- `global_disabled: true` and all `block_*: true` required
- kill-switch must remain active before any future real dispatch

### Final audit / CI report (Task 22.60)

~~~bash
deonctl worker codex provider-activation-final-audit \
  --activation-release-package artifacts/<run-id>/provider-activation-release-package.json \
  --activation-release-gate artifacts/<run-id>/provider-activation-release-gate.json \
  --operator-review-bundle artifacts/<run-id>/provider-activation-operator-review-bundle.json \
  --kill-switch-plan artifacts/<run-id>/provider-activation-kill-switch-plan.json \
  ...supporting artifacts... \
  --output-format json

deonctl worker codex provider-activation-ci-report \
  ...same inputs... \
  --output-format json
~~~

Rules:

- last gate before any real provider dispatch design
- `final_audit_ready: true`, `ci_observability_ready: true`, `kill_switch_active: true`
- CI report lists expected local commands and PR checklist (no GitHub Actions API dependency)

### Real dispatch design package (Tasks 22.61–22.64)

~~~bash
deonctl worker codex provider-secret-read-proposal new \
  --credential-policy-plan artifacts/<run-id>/provider-credential-policy-plan.json \
  --activation-final-audit artifacts/<run-id>/provider-activation-final-audit.json \
  --activation-ci-report artifacts/<run-id>/provider-activation-ci-report.json \
  --activation-release-gate artifacts/<run-id>/provider-activation-release-gate.json \
  --output artifacts/<run-id>/provider-secret-read-proposal.json

deonctl worker codex provider-real-transport-implementation-plan \
  --secret-read-proposal artifacts/<run-id>/provider-secret-read-proposal.json \
  --provider-adapter-plan artifacts/<run-id>/provider-adapter-plan.json \
  --activation-final-audit artifacts/<run-id>/provider-activation-final-audit.json \
  --operator-review-bundle artifacts/<run-id>/provider-activation-operator-review-bundle.json \
  --kill-switch-plan artifacts/<run-id>/provider-activation-kill-switch-plan.json \
  --output artifacts/<run-id>/provider-real-transport-implementation-plan.json

deonctl worker codex provider-real-dispatch-design \
  --real-transport-implementation-plan artifacts/<run-id>/provider-real-transport-implementation-plan.json \
  --secret-read-proposal artifacts/<run-id>/provider-secret-read-proposal.json \
  --activation-final-audit artifacts/<run-id>/provider-activation-final-audit.json \
  --activation-ci-report artifacts/<run-id>/provider-activation-ci-report.json \
  --provider-request-envelope artifacts/<run-id>/provider-request-envelope.json \
  --provider-real-call-proposal artifacts/<run-id>/provider-real-call-proposal.json \
  --output artifacts/<run-id>/provider-real-dispatch-design.json

deonctl worker codex provider-real-activation-design-review-package \
  ...design artifacts... \
  --output artifacts/<run-id>/provider-real-activation-design-review-package.json

deonctl worker codex provider-real-activation-design-review-gate \
  --design-review-package artifacts/<run-id>/provider-real-activation-design-review-package.json \
  ...revalidation inputs... \
  --output-format json
~~~

Rules:

- metadata-only design package; no secret reads, no provider call, no transport, no network
- `execute_subcommand_registered: false` — future `provider-real-dispatch execute` is design-only
- `real_activation_design_gate_ready: true` with `real_dispatch_supported_now: false`

### External approval package (Tasks 22.65–22.68)

~~~bash
deonctl worker codex provider-real-dispatch-external-approval new \
  --design-review-package artifacts/<run-id>/provider-real-activation-design-review-package.json \
  --design-review-gate artifacts/<run-id>/provider-real-activation-design-review-gate.json \
  ...chain inputs... \
  --output artifacts/<run-id>/provider-real-dispatch-external-approval-request.json

deonctl worker codex provider-real-dispatch-external-approval approve \
  --request artifacts/<run-id>/provider-real-dispatch-external-approval-request.json \
  --output artifacts/<run-id>/provider-real-dispatch-external-approval.json \
  --confirm-design-review-package-sha256 <sha256> \
  --confirm-design-review-gate-sha256 <sha256> \
  --confirm-secret-read-proposal-sha256 <sha256> \
  --confirm-real-dispatch-design-sha256 <sha256> \
  --confirm-provider-payload-sha256 <sha256>

deonctl worker codex provider-real-dispatch-preimplementation-gate \
  --external-approval artifacts/<run-id>/provider-real-dispatch-external-approval.json \
  --runbook artifacts/<run-id>/provider-real-dispatch-runbook.json \
  --risk-register artifacts/<run-id>/provider-real-dispatch-risk-register.json \
  ...revalidation inputs... \
  --output-format json
~~~

Rules:

- external manual authorization for future real dispatch only
- metadata-only runbook and risk register; no scripts, no execute commands
- `preimplementation_gate_ready: true`, `real_dispatch_supported_now: false`, `execute_subcommand_registered: false`
- no secret reads, no provider/network/transport, no workspace changes

Optional fixture steps 92–99 exercise external approval through preimplementation report.

## Provider call chain fixture smoke / CI guard (Task 22.37)

End-to-end fixture smoke in isolated temp dirs. Validates hash coherence, anti-leak rules, and final audit flags. Does not call providers or dispatch workers.

~~~bash
make provider-call-chain-smoke
~~~

See [PROVIDER_CALL_CHAIN_GATE.md](PROVIDER_CALL_CHAIN_GATE.md) for the last gate before a future provider executor skeleton.

### Debugging `status: failed`

1. Run `deonctl retrieval context inspect --artifact ...` and read `failures`
2. Re-run `memory lancedb search-report` on the upstream `lancedb-search-smoke-result.json`
3. Confirm the worker run trace has `retrieval_context_status: ok` in `execution-trace.json`
4. Use `deonctl runs retrieval-report --store ... --run <run-id>` to locate `retrieval-context.json` / `.md` paths

## Safety

- passive attachment only; runner does not search LanceDB
- no embedding provider API calls
- no memory apply/restore changes
- metadata-only runner attachment; chunk text requires explicit `materialize` with confirm flag
- attachment paths stripped from worker `RunSpec.Task`

## Boundary

Tasks 22.7–22.7.1 own controlled vector search smoke and result QA. Task 22.8 owns passive runner attachment of validated metadata. Task 22.8.1 owns retrieval-context inspect and runs retrieval-report for passive attachment auditing. Task 22.9 owns governed chunk text materialization from chunks JSONL without runner auto-injection. Task 22.9.1 owns materialized-report QA and materialize path hardening. Task 22.10 owns retrieval context audit bundle consolidation without runner text injection. Task 22.11 owns explicit approval workflow for materialized context without runner text injection. Task 22.12 owns injection planning without runner execution. Task 22.12.1 owns consolidated governance reporting without injection. Task 22.12.2 owns the end-to-end governance fixture and e2e validation. Task 22.13 owns the retrieval governance release checklist. Task 22.14 owns the retrieval runner injection design ADR without implementation. Task 22.15 owns injection policy schema validate/plan without execution. Task 22.15.1 extends the governance e2e fixture with injection-policy validate/plan over real chain artifacts. Task 22.16 owns runner injection approval artifacts without execution. Task 22.17 owns runner injection execution-plan artifacts without worker execution. Task 22.18 owns materialized prompt section preview render without worker execution. Task 22.18.1 owns prompt preview report/QA without worker execution. Task 22.18.2 owns injection governance release bundle consolidation without worker execution. Task 22.19 owns task YAML declaration for future materialized injection with governance bundle validation only. Task 22.19.1 owns materialized injection task declaration report/QA without runner execution. Task 22.20 owns runner materialized injection preflight without worker execution. Task 22.21 owns runner materialized injection dry-run prompt artifact without worker execution. Task 22.21.1 owns runner materialized injection dry-run report/QA without worker execution. Task 22.21.2 owns runner materialized injection readiness report without worker execution. Task 22.22 owns runner materialized injection execution gate without worker execution. Task 22.23 owns materialized prompt assembly dry-run adapter without worker execution and without normal runner prompt changes. Task 22.24 owns materialized prompt assembly report/QA without worker execution. Task 22.25 owns materialized injection runtime config schema validation without worker execution. Task 22.26 owns materialized injection run planner without worker execution and without normal runner prompt changes. Task 22.27 owns materialized injection execution enablement policy schema validation without worker execution. Task 22.28 owns materialized injection provider run-plan without provider call. Task 22.34 owns provider call approval without provider call. Task 22.35 owns provider call execution bundle without provider call, network, or worker execution. Natural-language retrieval and active runner search remain future work. See docs/MEMORY_LANCEDB.md and docs/MEMORY_INDEX.md.
