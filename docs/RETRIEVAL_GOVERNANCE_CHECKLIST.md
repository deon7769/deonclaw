# Retrieval governance release checklist

Task 22.13 — operability and release checklist for the retrieval-context governance pipeline.

This document does **not** change runner behavior, task schema, memory apply/restore, or injection policy. It records how to validate the pipeline before shipping documentation or governance CLI changes.

## 1. Pipeline overview

End-to-end chain (metadata and governed artifacts only):

~~~text
retrieval-context
  -> materialize
  -> materialized-report
  -> bundle
  -> approval request
  -> approval
  -> injection-plan
  -> governance-report
~~~

Supporting references:

| Stage | CLI / artifact | Purpose |
|-------|----------------|---------|
| retrieval-context | `retrieval context inspect` | Passive metadata-only run artifact from worker attachment (Task 22.8) |
| materialize | `retrieval context materialize` | Governed chunk text excerpts from memory-index chunks JSONL (Task 22.9) |
| materialized-report | `retrieval context materialized-report` | QA over materialized artifact shape and counts (Task 22.9.1) |
| bundle | `retrieval context bundle` | Audit bundle linking retrieval + materialized hashes, no text (Task 22.10) |
| approval request | `retrieval context approval new` | Pending approval request bound to bundle hash (Task 22.11) |
| approval | `retrieval context approval approve` / `inspect` | Explicit governed-use approval, `manual_review_only` (Task 22.11) |
| injection-plan | `retrieval context injection-plan` | Plan-only future runner integration; no execution (Task 22.12) |
| governance-report | `retrieval context governance-report` | Consolidated chain audit and troubleshooting (Task 22.12.1) |
| fixture e2e | `configs/examples/retrieval-context-fixture/` | Reproducible local walkthrough and automated tests (Task 22.12.2) |

Detailed behavior: [RETRIEVAL_CONTEXT.md](RETRIEVAL_CONTEXT.md). Fixture commands: [configs/examples/retrieval-context-fixture/README.md](../configs/examples/retrieval-context-fixture/README.md). Future injection design: [ADR_RETRIEVAL_RUNNER_INJECTION.md](ADR_RETRIEVAL_RUNNER_INJECTION.md).

## 2. Local validation commands

Run from repository root before merging governance-related changes:

~~~bash
gofmt -w .
git diff --check
go test ./...
go test ./internal/retrievalcontext -run GovernanceFixture
go test ./cmd/deonctl -run RetrievalContextGovernanceFixture
deonctl retrieval context injection-policy validate --policy configs/examples/retrieval-injection-policy.yaml
~~~

Optional manual fixture walkthrough (from fixture directory):

~~~bash
cd configs/examples/retrieval-context-fixture
# follow README.md steps 1–9
~~~

Or use project ship gate when ready to commit:

~~~bash
make ship MSG="short commit message"
~~~

`make ship` runs formatting, `git diff --check`, and `go test ./...` (includes fixture e2e tests).

## 3. Safety invariants

These must remain true for any release touching retrieval governance:

- **Runner does not inject materialized text** — worker prompts receive passive metadata only (`Retrieved context metadata only`); chunk text stays in governed artifacts.
- **`can_inject_now: false`** — injection-plan and governance-report require injection disabled.
- **`runner_injection_allowed: false`** — approvals stay `manual_review_only`; injection is not supported yet.
- **Only materialized artifacts contain `text_excerpt`** — bundle, approval, injection-plan, governance-report, and retrieval-context artifacts must not include chunk text fields.
- **Governance-report never prints `text_excerpt`** — text and JSON output are metadata-only.
- **No LanceDB search** in governance CLI paths or fixture workflow.
- **No embedding provider API** calls in governance CLI paths or fixture workflow.
- **No source Markdown reads during materialize** — text comes only from validated memory-index chunks JSONL.
- **Memory apply/restore unchanged** — canonical memory writes still require proposal → lint → approval → backup → apply; governance pipeline is separate.

## 4. Troubleshooting

### Hash mismatch

Symptoms: `bundle_sha256 mismatch`, `materialized_sha256 mismatch`, `request_sha256 mismatch`, or `approval request_sha256 mismatch`.

Actions:

1. Re-run the upstream step that produced the artifact (do not hand-edit hashes).
2. Confirm files were not modified after the binding step.
3. Run `deonctl retrieval context governance-report` with all six inputs to see which link failed.

### `source_retrieval_context_sha256` mismatch

Symptoms: materialize or bundle fails; governance-report lists `materialized source_retrieval_context_sha256 mismatch`.

Actions:

1. Re-materialize from the current `retrieval-context.json`.
2. Confirm `retrieval-context.json` is the same file referenced when materialize ran.

### Approval request mismatch

Symptoms: `approval inspect status "failed"` or `request_sha256 mismatch`.

Actions:

1. Regenerate approval request from the current bundle: `retrieval context approval new`.
2. Re-approve with `--confirm-approve-materialized-context` if the request changed.

### `materialized-report` failed

Symptoms: `materialized report status "failed"` with consistency failures.

Actions:

1. Run `deonctl retrieval context materialized-report --artifact <path> --output-format json`.
2. Read `failures` (chunk counts, `included_chars`, forbidden fields).
3. Re-run `materialize` with valid chunks JSONL and limits.

### `governance-report` failed

Symptoms: exit code 1; `status: failed` in output.

Actions:

1. Run with `--output-format json` and read `failures` and per-stage `stages`.
2. Fix the earliest failing stage (inspect → materialized → bundle → approval → injection-plan).
3. Confirm `injection_plan.can_inject_now` is `false` and `reason` is `runner_injection_allowed_false`.

### Path hardening errors

Symptoms: `path must be relative`, blocked prefix (`/tmp`, `..`, `secrets/`, `.env/`).

Actions:

1. Use relative safe paths under the workspace or artifact tree.
2. Do not pass absolute paths or parent traversal to governance commands.

More context: [RETRIEVAL_CONTEXT.md — Debugging `status: failed`](RETRIEVAL_CONTEXT.md#debugging-status-failed).

## 5. Release checklist

Before merging or tagging retrieval-governance work:

- [ ] Fixture e2e tests pass (`GovernanceFixture` / `RetrievalContextGovernanceFixture`).
- [ ] `go test ./...` green locally (or CI equivalent).
- [ ] `gofmt -w .` and `git diff --check` clean.
- [ ] [fixture README](../configs/examples/retrieval-context-fixture/README.md) commands match current `deonctl retrieval context` CLI flags.
- [ ] [RETRIEVAL_CONTEXT.md](RETRIEVAL_CONTEXT.md) updated if CLI or chain semantics changed.
- [ ] No `text_excerpt` leakage outside materialized artifacts (bundle, approval, injection-plan, governance-report outputs).
- [ ] No runner prompt or `retrieval_context` task schema changes unless explicitly scoped to a future injection task.
- [ ] No memory apply/restore chain changes.
- [ ] [ADR_RETRIEVAL_RUNNER_INJECTION.md](ADR_RETRIEVAL_RUNNER_INJECTION.md) still reflects current non-injection decision if governance semantics changed.

- [ ] CI green on `main` after push.

## 6. Boundary

Task 22.13 is documentation and operability only. Runner materialized text injection, active runner search, natural-language retrieval, provider APIs, MCP integration, fallback execution, and UI remain out of scope.

Tasks 22.22–22.24 add the runner materialized injection execution gate, materialized prompt assembly dry-run, and materialized prompt assembly report. They are **metadata-only and dry-run** artifacts:

- worker execution with materialized text is still not implemented
- normal runner prompt injection is still not implemented
- task `enabled: true` is still blocked
- the materialized prompt assembly is never sent to Codex or OpenCode
- all metadata-only artifacts (gate, assembly dry-run JSON/text, assembly report JSON/text) must block `text_excerpt`, alpha text, prompt-output content, and assembled-output content

Boundary

Task 22.13 is documentation and operability only. Runner materialized text injection, active runner search, natural-language retrieval, provider APIs, MCP integration, fallback execution, and UI remain out of scope.

See also: [MEMORY_INDEX.md](MEMORY_INDEX.md) (Task 22.x table), [MEMORY_LANCEDB.md](MEMORY_LANCEDB.md) (upstream search smoke), [ADR_RETRIEVAL_RUNNER_INJECTION.md](ADR_RETRIEVAL_RUNNER_INJECTION.md) (future injection design).
