# AGENTS.md — DeonClaw

## Project identity

DeonClaw is a personal orchestration and agent-harness control plane.

It coordinates external coding agents, memory domains, workspaces, policies, runs, logs, artifacts and approvals.

DeonClaw is not a fork of Codex, OpenCode, OpenClaw, Hermes or Paperclip.

DeonClaw learns from these tools, wraps them when useful, and keeps its own control plane, memory policy and execution traceability.

## Current MVP goal

Build a small, observable MVP that can:

1. create structured tasks
2. create isolated workspaces
3. build scoped context packs
4. run a worker, starting with Codex CLI
5. capture JSONL events, logs and artifacts
6. enforce path and memory policies
7. prevent uncontrolled writes to memory domains

## Current implemented state

Implemented:

- Go CLI/core
- SQLite store
- Codex worker dry-run and run
- OpenCode worker dry-run
- OpenCode worker run
- OpenCode real smoke validated on the VPS
- isolated Git worktree workspace
- dirty baseline protection
- workspace cleanup
- JSONL/stdout/stderr/diff/changed-files artifacts
- validation commands
- artifact manifest, list and prune
- domain config loader
- context pack builder
- context pack integration in Codex runs
- memory proposal format and CLI
- memory policy lint
- memory proposal lint in Codex runs
- apply dry-run
- patch-level apply policy
- approval artifact
- approval content binding
- apply preflight
- backup/restore plan
- backup materialization
- real memory apply for create/append/update/archive
- restore dry-run
- restore execute
- workers config command/provider/model/env diagnostics
- model profiles
- task model_profile
- OpenCode --model through model_profile
- model_strategy schema and validation
- model_strategy dry-run planning
- controlled model_strategy execution for OpenCode real runs
- fallback_policy schema and validation for model_strategy
- Docker runtime config validation and docker-plan dry-run
- Docker runtime simple command execution
- Docker runtime validation command execution
- Docker runtime env passthrough by explicit allowlist
- Docker worker execution scaffold for fake/test workers
- OpenCode Docker worker runtime supported experimental
- MCP registry config validation, list and plan foundation
- MCP fake/test stdio smoke with transcript artifacts, locally and through Docker
- MCP fake read-only tool-smoke with policy scaffold, locally and through Docker
- MCP real read-only discovery smoke with policy scaffold, Docker-gated for real servers
- MCP real read-only call-smoke with explicit policy, split fake/real policy examples, artifact leak scanning, and one Docker-gated tool call
- MCP read-only tool call proposal/preflight/approval workflow reusing call-smoke execution, with stale-hash checks and execution bundle
- MCP passive context attachments for runner prompts from audited discovery/call artifacts, without worker tool execution
- MCP worker-generated tool call proposal lint/preflight artifacts, without worker tool execution
- MCP proposal review queue from runs (`mcp proposals list/show/export`), without automatic execution
- memory index foundation (`memory index validate/plan/build/doctor/report`) with auditable chunks, without runner retrieval
- embedding policy dry-run and deterministic fake vector smoke (`memory embedding validate/doctor/plan/build-fake/report`), without real provider APIs or LanceDB writes
- LanceDB write plan-only (`memory lancedb validate/plan`) over embedding artifacts, without LanceDB import or database writes
- LanceDB fake-write artifact smoke (`memory lancedb fake-write`), without LanceDB SDK or real database creation
- LanceDB real local write smoke (`memory lancedb write-smoke`) via Python adapter, without retrieval or runner integration
- LanceDB structural readback doctor/report (`memory lancedb doctor/report`) over write-smoke databases, without search or runner integration
- LanceDB controlled vector search smoke (`memory lancedb search-smoke`) with explicit query vector or chunk_id, without natural-language retrieval, provider APIs, or runner integration
- LanceDB search-smoke result QA/report (`memory lancedb search-report`) over search artifacts, without new search or runner integration
- passive LanceDB retrieval context attachment in runner (`retrieval_context` on tasks), without runner search or chunk text injection
- retrieval context inspect and runs retrieval-report for passive attachment auditing, without active search
- governed retrieval context chunk text materialize from memory-index chunks JSONL (`retrieval context materialize`), without runner auto-injection or source file reads
- retrieval context materialized-report for governed chunk text artifact QA, without runner auto-injection
- retrieval context audit bundle (`retrieval context bundle`) over retrieval + materialized artifacts, without runner text injection
- retrieval context approval workflow (`retrieval context approval new/approve/inspect`) for governed materialized context use, without runner text injection
- retrieval context injection plan (`retrieval context injection-plan`) without runner execution or prompt changes
- retrieval context governance report (`retrieval context governance-report`) without runner injection
- retrieval context governance e2e fixture (`configs/examples/retrieval-context-fixture/`)
- retrieval governance release checklist (`docs/RETRIEVAL_GOVERNANCE_CHECKLIST.md`)
- retrieval runner injection design ADR (`docs/ADR_RETRIEVAL_RUNNER_INJECTION.md`)
- retrieval context injection policy schema (`retrieval context injection-policy validate/plan`), no execution
- retrieval injection policy e2e fixture (`configs/examples/retrieval-context-fixture/retrieval-injection-policy.yaml`), validate/plan only
- runner injection approval artifact (`retrieval context injection-approval new/approve/inspect`), no execution
- runner injection execution plan (`retrieval context injection-execution-plan`), no worker execution
- materialized prompt section preview (`retrieval context prompt-preview`), no worker execution
- prompt preview report/QA (`retrieval context prompt-preview-report`), no worker execution
- injection governance release bundle (`retrieval context injection-governance-bundle`), no worker execution
- task schema declaration for materialized injection (`retrieval_context.materialized_injection`), no runner execution
- materialized injection task declaration report (`task materialized-injection-report`), no runner execution
- runner materialized injection preflight (`worker codex materialized-injection-preflight`), no runner execution
- runner materialized injection dry-run prompt artifact (`worker codex materialized-injection-dry-run`), no worker execution
- runner materialized injection dry-run report (`worker codex materialized-injection-dry-run-report`), no worker execution
- runner materialized injection readiness report (`worker codex materialized-injection-readiness-report`), no worker execution
- runner materialized injection execution gate (`worker codex materialized-injection-execution-gate`), no worker execution
- materialized prompt assembly dry-run (`worker codex materialized-prompt-assembly-dry-run`), no worker execution
- materialized prompt assembly report (`worker codex materialized-prompt-assembly-report`), no worker execution
- materialized injection runtime config schema (`worker codex materialized-injection-runtime validate`), no worker execution
- materialized injection run planner (`worker codex materialized-injection-run-plan`), no worker execution
- materialized injection execution enablement policy (`worker codex materialized-injection-execution-enable validate`), no worker execution
- materialized injection provider run-plan (`worker codex materialized-injection-provider-run-plan`), no provider call
- materialized provider dispatch validate (`worker codex materialized-provider-dispatch validate`), no provider call
- materialized provider payload dry-run/report (`worker codex materialized-provider-payload-dry-run`, `materialized-provider-payload-report`), no provider call
- materialized provider call gate/readiness report (`worker codex materialized-provider-call-gate`, `materialized-provider-call-readiness-report`), no provider call
- provider call approval (`worker codex provider-call-approval new/approve/inspect`), no provider call
- provider call execution bundle (`worker codex provider-call-execution-bundle`), no provider call
- provider call chain continuity audit (`worker codex provider-call-chain-audit`), no provider call
- provider call chain fixture smoke / CI guard (`scripts/provider-call-chain-fixture-smoke.sh`, `make provider-call-chain-smoke`), no provider call
- provider call executor skeleton (`worker codex provider-call-executor config validate`, `validate`, `plan`, `dry-run`, `dry-run-report`, `preflight`), no provider call
- provider call executor dispatch approval (`worker codex provider-call-executor-dispatch-approval new/approve/inspect`), no provider call
- provider transport plan (`worker codex provider-transport-plan`), blocked, no provider call
- provider executor release bundle / gate (`worker codex provider-executor-release-bundle`, `release-gate`), no provider call
- provider execution simulation layer (`provider-request-envelope`, `provider-adapter-registry`, `provider-adapter-plan`, `provider-response-fixture`, `provider-execution-simulation-bundle`, `provider-execution-simulation-report`), no provider call
- provider activation readiness layer (`provider-credential-policy`, `provider-real-call-proposal`, `provider-response-change-proposal`, `provider-activation-readiness-audit`), no provider call
- provider activation control plane (`provider-activation-policy`, `provider-activation-approval`, `provider-activation-rehearsal`, `provider-activation-release-package`, `provider-activation-release-gate`), no provider call
- provider activation hardening (`provider-activation-operator-review-bundle`, `provider-activation-kill-switch`, `provider-activation-final-audit`, `provider-activation-ci-report`), no provider call
- provider activation policy config (`configs/examples/provider-activation-policy.yaml`), no provider call
- provider activation kill-switch config (`configs/examples/provider-activation-kill-switch.yaml`), no provider call
- provider credential policy config (`configs/examples/provider-credential-policy.yaml`), no provider call
- provider adapters config (`configs/examples/provider-adapters.yaml`), no provider call
- provider call executor policy config (`configs/examples/provider-call-executor.yaml`), no provider call
- workers smoke --dry-run model_strategy planning
- fallback policy schema validation only; no fallback execution or retries
- execution trace artifact
- runs report by worker/status and by model_profile with filters

Not implemented yet:

- real fallback execution/retry
- Codex Docker worker execution
- MCP execution/manager; current MCP support is registry config/list/plan/doctor/risk/docker-plan plus local/Docker fake/test smoke, fake read-only tool-smoke, real read-only discovery, one-call real read-only call-smoke, explicit proposal approval workflow with execution bundle, passive context attachments, worker proposal lint/preflight only, and run-scoped proposal review queue without execution
- memory index retrieval/LanceDB natural-language search or active search in runner
- UI/dashboard

## Non-goals for the MVP

- Do not build a full AI agent from scratch.
- Do not implement a model provider first.
- Do not store OAuth tokens inside DeonClaw.
- Do not fork Codex, OpenCode, OpenClaw or Hermes.
- Do not build a UI before CLI/core execution works.
- Do not write directly to `mysecondbrain` canonical memory.
- Do not mix Escalasoft domain memory into general memory.
- Do not create a giant `core` package that absorbs unrelated responsibilities.

## Architecture decisions

- Core language: Go.
- MVP database: SQLite.
- Runtime isolation: workspace directories first, Docker later.
- First worker: Codex CLI through `codex exec --json`.
- Second worker: OpenCode, especially for provider-agnostic execution and Z.ai/GLM usage.
- Later workers: Claude Code, OpenClaw adapter, Hermes-inspired memory adapter.
- Kimi status: inspiration_only. Kimi can inform future UX/MCP/ACP ideas, but it is not an operational worker.
- Memory base: `mysecondbrain` mounted read-only by default.
- Escalasoft memory: isolated domain, not part of general recall.
- Auth strategy: harness-managed in MVP.
- Memory retrieval/index strategy: optional retrieval layer over Markdown sources, not source of truth.

## Repository structure target

```text
cmd/
  deonctl/
  deonclawd/

internal/
  tasks/
  runs/
  events/
  workers/
  runtime/
  policy/
  memory/
  memoryindex/
  domains/
  contextpack/
  artifacts/
  approvals/
  skills/

configs/
  domains.yaml
  workers.yaml
  memory-policy.yaml
  skill-policy.yaml

docs/
  adr/
  architecture/
  experiments/

examples/
  tasks/
  context-packs/
```

## Worker rules

Workers are external tools controlled by DeonClaw.

Initial workers:

- Codex CLI
- OpenCode
- Claude Code, later
- OpenClaw adapter, later

Kimi is `inspiration_only`. Keep it out of operational worker config, doctor output, smoke runs, fallback lists, and run dispatch until a dedicated future task defines a worker contract.

Workers must not own memory policy.

Workers must not write to canonical memory directly.

Workers produce:

- event stream
- logs
- diff
- artifacts
- summary
- optional memory proposal

## Codex worker contract

Initial command shape:

```bash
codex exec --json --sandbox workspace-write --cd <workspace> -
```

The prompt is passed through stdin.

DeonClaw must capture stdout JSONL and stderr logs separately.

Do not use `danger-full-access` unless the run is inside a controlled container/workspace.

## Memory rules

General memory:

- `mysecondbrain` is the general memory base.
- Mount read-only for workers.
- Canonical writes require proposal, policy validation and approval.

Escalasoft memory:

- `escalasoft_brain` is an isolated domain.
- General agents must not load it by default.
- Escalasoft agents may read it when the task domain is `escalasoft`.
- Escalasoft content must not be promoted into `MEMORY.md`.

Memory index:

- A vector/index store such as LanceDB may be used to retrieve and map memory.
- The index is derived state.
- Markdown/Git remains the source of truth.
- Index rebuild must be safe and reproducible.
- Index results must cite source files and line/chunk IDs when used in prompts.

## Memory apply rules

Real memory apply is implemented for create/append/update/archive. Do not change the apply or restore chain unless the task explicitly targets memory workflow behavior.

For real memory apply, the required chain is:

```text
proposal
-> lint
-> apply dry-run
-> patch-level policy
-> approval artifact
-> preflight
-> backup plan
-> backup materialization
-> apply
```

Never skip approval, preflight or backup.

Restore execution is implemented as a separate safety path and requires a backup plan, backup result, restore dry-run preview and explicit restore confirmation.

## Memory lifecycle

Memory work has four different modes:

1. Capture
   - raw notes, agent outputs, logs, task summaries
   - goes to inbox/proposals/scratch

2. Lapidation
   - clean, structure, dedupe, classify, link, summarize
   - produces candidate memory patches

3. Canonization
   - promotion into long-lived domain memory
   - requires stronger policy and review

4. Renewal
   - periodic review of stale memory, broken links, outdated skills and weak instructions
   - creates update proposals, not silent rewrites

Do not collapse these modes into a single "write memory" action.

## Skills lifecycle

Skills are operational playbooks for agents.

A skill can be:

- draft
- active
- deprecated
- archived

Agents may propose skill updates, but direct skill promotion requires policy validation.

Skills should be renewed when:

- a recurring task changes
- a tool changes behavior
- an agent repeatedly fails
- a better workflow emerges
- a domain policy changes

Skill updates must include:

- reason
- source evidence
- changed behavior
- expected use case
- validation plan

## Path policy

Every task must define:

- allowed paths
- forbidden paths
- workspace path
- memory scope
- expected artifacts
- validation commands

If a worker changes files outside allowed paths, the run must fail policy validation.

## Next implementation order

Current next sequence:

1. MCP manager
2. memory index retrieval/LanceDB search
3. UI/dashboard

## Coding style

- Prefer small packages.
- Keep interfaces narrow.
- Avoid premature abstractions.
- Avoid a package named `internal/core`.
- Add tests for policy, task parsing, event parsing and lifecycle state transitions.
- Keep MVP simple and observable.
- Every feature should produce logs/events useful for debugging.

## Before committing

Run:

```bash
go test ./...
gofmt -w .
```

For this repository, after the full validation gate passes and the task is ready to ship, use the project-local automation:

```bash
make ship MSG="short commit message"
```

`make ship` runs formatting, `git diff --check`, `go test ./...`, then commits and pushes the current branch. It is intentionally scoped to the DeonClaw remote and must not be reused for other repositories.

Do not commit generated secrets, tokens, local auth files, memory vault contents or domain data.
