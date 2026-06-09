# DeonClaw Roadmap

## Completed MVP milestones

- Phase 0 - Documentation and contracts
- Phase 1 - Core skeleton/store
- Phase 2 - Codex worker execution
- Phase 3 - Workspace/path policy
- Phase 4 - Domain config and context packs
- Phase 5A - Memory proposal/lint/dry-run/approval/preflight
- Phase 5B - Memory workflow through backup/materialize/apply/restore
- Phase 6 - OpenCode worker dry-run and run
- Task 18.13.1 - runs report filters and no-profile split
- Task 18.14 - model_strategy schema and validation
- Task 18.15 - model profile command wiring for OpenCode
- Task 18.15.1 - model_strategy dry-run planning alignment
- Task 18.16 - controlled model_strategy execution without fallback
- execution-trace.json for shared worker runs
- runs report by worker/status and by model_profile with filters

## Next milestones

### 18.17 - Fallback policy/design

- policy for retry/fallback behavior
- clear failure boundaries
- no hidden worker switching

### 20 - Docker runtime

- read-only memory mounts
- artifact volumes
- restricted secrets
- runtime cleanup

### 21 - MCP manager

- manage MCP server definitions
- inspect configured tools
- keep secrets outside project files

### 22 - Memory index/LanceDB

- derived retrieval/index adapter
- rebuildable index state
- source citation metadata

### Later - UI/dashboard

- task list
- run details
- event logs
- artifacts
- approvals
## Historical roadmap

The sections below are the original roadmap and are kept for context.

Use the "Next milestones" section above as the source of truth for current implementation order.

## Phase 0 — Documentation and contracts

Goal: make the project consistent before coding.

Deliverables:

- `AGENTS.md`
- `README.md`
- project definition
- architecture notes
- ADRs
- example task files
- example config files

## Phase 1 — Core skeleton

Goal: create a minimal Go project.

Deliverables:

- `go.mod`
- `cmd/deonctl`
- task parser
- run model
- event model
- artifact model
- basic SQLite store
- tests

## Phase 2 — Codex worker smoke test

Goal: call Codex CLI through DeonClaw without editing files.

Deliverables:

- CodexWorker dry-run
- CodexWorker execution
- JSONL capture
- stderr capture
- summary artifact
- run folder layout

## Phase 3 — Workspace and path policy

Goal: allow safe workspace-write execution.

Deliverables:

- workspace manager
- dirty baseline failure by default
- future explicit override flag: `--allow-dirty-baseline`
- workspace cleanup policy after run completion
- allowed/forbidden path checks
- diff capture
- policy failure state
- validation commands

## Phase 4 — Memory domains and context packs

Goal: build scoped context without exposing full memory.

Deliverables:

- domains config
- memory policy config
- context pack builder
- general domain
- escalasoft isolated domain
- memory proposal format

## Phase 5 — Memory and skill lifecycle

Goal: support capture, lapidation, canonization and renewal.

Deliverables:

- memory proposal commands
- memory lint
- stale memory report
- skill inventory
- skill update proposal
- lifecycle event types

## Phase 6 — Optional memory index

Goal: add retrieval/index support without replacing Markdown source of truth.

Deliverables:

- index adapter interface
- LanceDB experimental adapter
- chunk metadata schema
- index rebuild command
- search command
- source citation metadata

## Phase 7 — OpenCode worker

Goal: add provider-agnostic coding worker.

Deliverables:

- OpenCodeWorker
- Z.ai/GLM config example
- event capture
- policy validation

## Phase 8 — Docker runtime

Goal: isolate worker runs.

Deliverables:

- container workspace
- read-only memory mounts
- artifact volumes
- restricted secrets
- runtime cleanup

## Phase 9 — UI or dashboard

Goal: only after CLI/core works.

Deliverables:

- task list
- run details
- event logs
- artifacts
- approvals
- memory proposals
