# Coding Agent Guide

## Read first

Before changing code, read:

1. AGENTS.md
2. docs/ARCHITECTURE.md
3. docs/ROADMAP.md
4. docs/MEMORY_PROPOSALS.md for memory tasks
5. docs/CONTEXT_PACKS.md for context tasks
6. docs/ARTIFACTS.md for artifact/store tasks

## Current rule

For current sequencing, follow AGENTS.md Next implementation order first.

Do not implement fallback execution, Codex Docker worker execution, MCP execution/manager, memory index, or UI out of order. The current fallback surface is schema/policy only. The current Docker surface is runtime config validation, docker-plan, simple `runtime docker-exec`, Docker-backed validation commands, allowlisted env passthrough, fake/test worker runtime scaffold, MCP fake/test smoke, fake read-only tool-smoke, real read-only discovery, real read-only call-smoke, MCP call approval workflow, and OpenCode Docker smoke only; Codex still runs locally. The current MCP surface is registry validation/list/plan/doctor/risk/docker-plan plus local/Docker fake/test stdio smoke, fake read-only tool-smoke, policy-gated real read-only discovery, policy-gated one-call real read-only call-smoke, and explicit proposal/preflight/approval execution for that same call path only; no automatic MCP tool orchestration, external worker integration, or fallback integration.

Memory apply and restore already exist. Do not alter their behavior unless a task explicitly targets the memory workflow.

The current safe apply chain is:

proposal -> lint -> apply dry-run -> approval -> preflight -> backup plan -> backup materialization -> apply

The current safe restore chain is:

backup plan -> backup materialization -> restore dry-run -> restore execute

## Package boundaries

- cmd/deonctl: CLI parsing and output only
- internal/runner: run orchestration
- internal/workers: external worker adapters
- internal/workerconfig: workers.yaml, model_profiles, model_strategy resolution, fallback policy schema validation, and env requirement metadata
- internal/runtime: workspace lifecycle
- internal/runtimeconfig: runtime.yaml loading, validation, Docker mount/env passthrough policy, docker-plan generation, simple docker-exec planning, validation command Docker planning, and fake/test worker Docker planning
- internal/mcpconfig: mcp.yaml loading, validation, registry listing, diagnostics, risk reporting, and static command/env planning only
- internal/mcpsmoke: controlled fake/test MCP stdio smoke, fake read-only tool-smoke, real read-only discovery smoke, real read-only call-smoke, policy scaffolds, and transcript artifacts only
- internal/mcpapproval: MCP read-only tool call proposal, lint, preflight, approval, hash binding, and explicit execute workflow only
- internal/policy: path policy
- internal/memory: memory proposal/lint/apply-preview/approval/preflight/backup-plan/backup-materialize/restore-preview/restore-execute/apply-execute
- internal/contextpack: scoped context generation
- internal/domains: domain config
- internal/store: persistence
- internal/artifacts: artifact retention/prune
- internal/doctor: local diagnostics for workers, env requirements and profiles
- internal/runreport: read-only reporting over persisted runs and execution traces

Do not move business logic back into cmd/deonctl.

## Task sizing

Small Codex task:

- one package or one documentation surface
- no cross-cutting behavior change
- validation can be `gofmt`, `git diff --check`, and targeted or full `go test ./...`

Medium Codex task:

- touches CLI plus one internal package
- adds or changes tests
- needs artifact or trace review when runner behavior changes

Larger Codex task:

- changes worker execution, memory apply/restore, fallback policy, Docker runtime, MCP manager, or persistence contracts
- requires a narrow implementation plan, contract review, full tests, and explicit non-goals before coding

## Test expectations

Before running a real worker command, run:

~~~bash
deonctl doctor
~~~

Use worker-scoped diagnostics when command config or PATH is part of the change:

~~~bash
deonctl workers doctor --worker codex
deonctl workers doctor --worker opencode
~~~

Run:

~~~bash
gofmt -w .
git diff --check
go test ./...
~~~

Add tests for every new safety rule.
