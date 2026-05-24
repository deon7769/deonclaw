# DeonClaw Architecture

## System overview

```text
          deonctl / API / adapters
                    |
                    v
              DeonClaw Core
                    |
    +---------------+----------------+
    |               |                |
 tasks/runs     policy          context packs
    |               |                |
    v               v                v
 workspace       memory guard     domain router
    |
    v
 workers: Codex / OpenCode / Claude Code / OpenClaw
    |
    v
 events / logs / diffs / artifacts / proposals
```

## Core principle

DeonClaw does not try to be smarter than the worker.

DeonClaw controls the execution boundary.

The worker performs the task.

## Major packages

### `internal/tasks`

Task definitions, YAML parsing and validation.

### `internal/runs`

Run lifecycle:

- pending
- running
- succeeded
- failed
- policy_failed
- cancelled

### `internal/events`

Event model and event storage.

### `internal/workers`

Worker adapters.

Initial adapters:

- Codex CLI
- OpenCode

### `internal/runtime`

Workspace and runtime execution.

Initial runtime:

- local workspace directories
- isolated per-run workspace, initially via Git worktree
- workspace cleanup policy: remove succeeded runs, keep failed and policy_failed runs
- copy-based workspace remains a future implementation option

Later runtime:

- Docker containers

### `internal/policy`

Path policy, memory policy, worker policy and approval requirements.

### `internal/memory`

Memory proposals, memory lint and lifecycle state.

### `internal/memoryindex`

Derived retrieval/index adapters.

This package must never become the canonical memory store.

### `internal/domains`

Domain definitions and routing.

Examples:

- general
- escalasoft
- infra
- nutri

### `internal/contextpack`

Builds scoped context from domains, policies and task requirements.

### `internal/skills`

Skill inventory, renewal proposals and validation.

## Data model

```text
Task
 -> Run
 -> Events
 -> Artifacts
 -> Diff
 -> Summary
 -> ContextPack
 -> Optional MemoryProposal
 -> MemoryProposalLint
 -> ApplyPreview
 -> MemoryApproval
 -> ApplyPreflight
 -> BackupPlan
 -> BackupResult
 -> Future ApplyResult
```

## Storage model

MVP:

- SQLite for tasks, runs and events
- filesystem for artifacts

Later:

- Postgres if needed
- object storage if artifacts grow

## Memory model

Markdown/Git remains canonical.

Indexes are derived.

Workers receive context packs, not whole memory vaults.

## Memory safety pipeline

Canonical memory is never modified by workers.

The safe memory workflow is:

1. proposal artifact
2. policy lint
3. apply dry-run
4. patch-level validation
5. approval artifact
6. preflight with content-hash binding
7. backup plan
8. backup materialization
9. real apply, later

Real apply is not implemented yet.

## Worker model

Workers are external tools.

DeonClaw invokes them and captures their output.

Workers do not own:

- domain policy
- memory policy
- approvals
- task store

## Security model

- workers run in scoped workspaces
- memory mounts are read-only by default
- secrets are never committed
- auth is harness-managed in MVP
- destructive commands require policy and approval
