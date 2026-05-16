# DeonClaw Project Definition

## What DeonClaw is

DeonClaw is a personal control plane for orchestrating agent harnesses.

It exists to make agent work:

- scoped
- observable
- reproducible
- policy-checked
- memory-safe
- domain-aware

## What DeonClaw is not

DeonClaw is not:

- a model provider
- a replacement for Codex/OpenCode/Claude Code
- a fork of OpenClaw
- a full memory system in the MVP
- a UI-first project
- a general-purpose autonomous company of agents

## Why this project exists

Current agent tools are useful but often mix:

- chat
- memory
- coding
- tool use
- permissions
- task state
- domain context

DeonClaw separates these concerns.

The goal is to keep useful harnesses while adding a personal layer for:

- task ownership
- memory protection
- domain isolation
- execution logs
- artifacts
- approvals

## First success criteria

The first useful version succeeds when it can:

1. read a task file
2. create a run
3. create or select a workspace
4. call Codex CLI through an adapter
5. capture JSONL events
6. save logs and artifacts
7. validate path policy
8. produce a summary
9. avoid touching memory directly

## Later success criteria

Later versions should:

- support OpenCode and Claude Code
- build scoped context packs from memory
- propose memory changes
- maintain skill lifecycle
- use retrieval indexes safely
- support isolated domain agents
- run in containers on VPS
