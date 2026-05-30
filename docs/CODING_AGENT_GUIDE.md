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

Do not implement real memory apply unless explicitly asked.

For current sequencing, follow AGENTS.md Next implementation order first.

Do not implement apply/OpenCode out of order.

The current safe apply chain is:

proposal -> lint -> apply dry-run -> approval -> preflight -> backup plan -> backup materialization -> apply

The current safe restore chain is:

backup plan -> backup materialization -> restore dry-run -> restore execute

## Package boundaries

- cmd/deonctl: CLI parsing and output only
- internal/runner: run orchestration
- internal/workers: external worker adapters
- internal/runtime: workspace lifecycle
- internal/policy: path policy
- internal/memory: memory proposal/lint/apply-preview/approval/preflight/backup-plan/backup-materialize/restore-preview/restore-execute/apply-execute
- internal/contextpack: scoped context generation
- internal/domains: domain config
- internal/store: persistence
- internal/artifacts: artifact retention/prune

Do not move business logic back into cmd/deonctl.

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
go test ./...
gofmt -w .
~~~

Add tests for every new safety rule.
