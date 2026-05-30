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
- Codex CLI worker
- JSONL event capture
- isolated workspaces
- memory/domain policy
- path validation
- memory proposal workflow

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

## Status

Functional MVP in active development.

Implemented:

- Codex CLI worker execution
- OpenCode worker dry-run/run
- isolated Git worktree runs
- path policy and diff artifacts
- validation commands
- artifact metadata/list/prune
- domain config
- context packs
- memory proposal/lint/dry-run/approval/preflight
- safe memory apply for create/append/update/archive with backup/restore chain

Not implemented yet:

- Docker runtime
- memory index
- UI

## Quick command map

```bash
deonctl doctor
deonctl workers doctor --output-format json
deonctl config env
deonctl task validate examples/tasks/codex-smoke.yaml
deonctl domains validate --config configs/examples/domains.yaml
deonctl context build --task examples/tasks/codex-smoke.yaml --domains configs/examples/domains.yaml --output /tmp/context-pack.md
deonctl worker codex dry-run examples/tasks/codex-smoke.yaml
deonctl worker opencode dry-run <opencode-task.yaml>
deonctl worker opencode run <opencode-task.yaml> --store deonclaw.db --artifacts-dir artifacts --domains configs/examples/domains.yaml --memory-policy configs/examples/memory-policy.yaml
deonctl artifacts list --store deonclaw.db
deonctl memory proposal lint --proposal memory-proposal.json --policy configs/examples/memory-policy.yaml
```

`deonctl workers doctor --worker kimi` reports configured command diagnostics only. Kimi is marked as `future_worker` until a worker adapter is implemented.
