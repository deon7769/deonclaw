# DeonClaw

Personal agent orchestration and harness control plane.

DeonClaw coordinates external coding agents, memory domains, workspaces, policies, runs, logs, artifacts and approvals.

It is not a fork of Codex, OpenCode, OpenClaw, Hermes or Paperclip.

## Core idea

Agents do work.

DeonClaw controls:

- task scope
- workspace isolation
- memory boundaries
- domain routing
- worker execution
- policy validation
- event capture
- artifacts
- approvals

## MVP focus

The first MVP focuses on:

- Go CLI/core
- structured tasks
- Codex CLI and OpenCode workers
- JSONL, mixed, text and empty stdout capture
- isolated workspaces
- memory/domain policy
- path validation
- memory proposal/apply/restore workflow
- run observability through execution traces and reports

## Memory direction

`mysecondbrain` is the general memory base.

`escalasoft_brain` is an isolated domain memory base.

Indexes such as LanceDB may be used to improve retrieval and mapping, but they are not the source of truth.

Markdown + Git remain canonical.

## First workers

1. Codex CLI
2. OpenCode
3. Claude Code, later
4. OpenClaw adapter, later

Kimi is `inspiration_only`. It is not an operational worker and must not appear in `workers.yaml`, doctor output, smoke runs, or fallback lists.

## Status

Functional MVP in active development.

Implemented:

- Codex CLI worker dry-run and run
- OpenCode worker dry-run and run
- OpenCode real smoke validated on the VPS
- isolated Git worktree runs
- path policy and diff artifacts
- validation commands
- artifact metadata/list/prune
- domain config
- context packs
- `workers.yaml` command/provider/model/env metadata
- `model_profiles`
- `task.model_profile`
- OpenCode `--model` through selected `model_profile`
- `model_strategy` schema and validation
- `model_strategy` dry-run planning
- controlled `model_strategy.preferred[0]` selection for OpenCode real runs
- `workers smoke --dry-run` with `model_strategy` planning
- `execution-trace.json`
- `runs report`
- `runs report --by model_profile` and filters
- Docker runtime config validation, docker-plan, docker-exec, and Docker validation commands
- OpenCode Docker worker runtime supported experimental
- OpenCode Docker Z.AI smoke runtime
- MCP registry config validation, list and plan foundation
- MCP fake/test smoke, fake read-only tool-smoke, real read-only discovery, policy-gated real read-only call-smoke, explicit MCP call proposal approval workflow with stale-hash checks and execution bundle, passive MCP context attachments for worker runs, worker-generated MCP proposal lint/preflight artifacts without tool execution, and run-scoped MCP proposal review queue (`mcp proposals list/show/export`) without automatic execution
- fallback policy schema validation only; no fallback execution or retries
- memory proposal/lint/dry-run/approval/preflight/backup/materialize/apply/restore workflow
- real memory apply for create/append/update/archive
- real restore execution
- memory index foundation (`memory index validate/plan/build/doctor/report`) with auditable chunks, without runner retrieval or LanceDB writes
- embedding policy dry-run and deterministic fake vector smoke (`memory embedding validate/doctor/plan/build-fake/report`), without real provider APIs or LanceDB writes
- LanceDB write plan-only (`memory lancedb validate/plan`) over embedding artifacts, without LanceDB import or database writes
- LanceDB fake-write artifact smoke (`memory lancedb fake-write`), without LanceDB SDK or real database creation
- LanceDB real local write smoke (`memory lancedb write-smoke`) via Python adapter, without retrieval or runner integration
- LanceDB structural readback doctor/report (`memory lancedb doctor/report`) over write-smoke databases, without search or runner integration
- LanceDB controlled vector search smoke (`memory lancedb search-smoke`) with explicit query vector or chunk_id, without natural-language retrieval, provider APIs, or runner integration
- LanceDB search-smoke result QA/report (`memory lancedb search-report`) over search artifacts, without new search or runner integration
- passive LanceDB retrieval context attachment in runner (`retrieval_context` on tasks), without runner search or chunk text injection
- retrieval context inspect and runs retrieval-report for passive attachment auditing, without active search
- governed retrieval context chunk text materialize (`retrieval context materialize`), without runner auto-injection
- retrieval context materialized-report for chunk text artifact QA, without runner auto-injection
- retrieval context audit bundle (`retrieval context bundle`), without runner text injection
- retrieval context approval workflow (`retrieval context approval`), without runner text injection
- retrieval context injection plan (`retrieval context injection-plan`), without runner execution
- retrieval context governance report (`retrieval context governance-report`), without runner injection
- retrieval context governance e2e fixture (`configs/examples/retrieval-context-fixture/`)
- retrieval governance release checklist (`docs/RETRIEVAL_GOVERNANCE_CHECKLIST.md`)
- retrieval runner injection design ADR (`docs/ADR_RETRIEVAL_RUNNER_INJECTION.md`)
- retrieval context injection policy schema (`retrieval context injection-policy`), no execution
- retrieval injection policy e2e fixture (`configs/examples/retrieval-context-fixture/retrieval-injection-policy.yaml`), validate/plan only
- runner injection approval artifact (`retrieval context injection-approval`), no execution
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
- provider call approval (`worker codex provider-call-approval new/approve/inspect`), no provider call
- provider call execution bundle (`worker codex provider-call-execution-bundle`), no provider call

Not implemented yet:

- real fallback execution
- Codex Docker worker runtime
- MCP execution/manager for automatic real external MCP tool orchestration
- memory index retrieval/LanceDB natural-language search or active search in runner
- UI/dashboard

## Quick command map

```bash
deonctl doctor
deonctl workers doctor --worker opencode --workers-config configs/examples/workers.yaml --profiles
deonctl workers smoke --worker opencode --task examples/tasks/opencode-smoke.yaml --store deonclaw.db --artifacts-dir artifacts --workers-config configs/examples/workers.yaml --dry-run
deonctl runs report --store deonclaw.db --by model_profile
deonctl worker opencode run examples/tasks/opencode-smoke.yaml --store deonclaw.db --artifacts-dir artifacts --workers-config configs/examples/workers.yaml
deonctl worker opencode run examples/tasks/opencode-smoke.yaml --store deonclaw.db --artifacts-dir artifacts --workers-config configs/examples/workers.yaml --domains configs/examples/domains.yaml --memory-policy configs/examples/memory-policy.yaml
```
