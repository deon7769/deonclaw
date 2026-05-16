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

Early planning/MVP.
