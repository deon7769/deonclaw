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

Not implemented yet:

- real fallback execution
- Codex Docker worker runtime
- MCP execution/manager for automatic real external MCP tool orchestration
- memory index retrieval/LanceDB search in runner
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
