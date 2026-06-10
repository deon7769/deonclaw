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
- workers smoke --dry-run model_strategy planning
- execution trace artifact
- runs report by worker/status and by model_profile with filters

Not implemented yet:

- real fallback execution/retry
- real Codex/OpenCode Docker worker execution
- MCP manager
- memory index/LanceDB
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
2. memory index/LanceDB
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
