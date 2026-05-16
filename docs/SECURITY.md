# Security Notes

## Core rule

Least privilege first.

Every worker run should have the smallest permission set that can complete the task.

## Auth

MVP uses harness-managed auth.

Do not copy OAuth tokens.

Do not commit:

- `.env`
- auth files
- `~/.codex/auth.json`
- API keys
- local provider configs
- memory vault contents

## Runtime

Initial runtime:

- local workspaces

Later runtime:

- Docker containers

Workers should not mount memory as writable.

## Memory safety

Memory vaults are read-only by default.

Canonical writes require:

- proposal
- policy validation
- approval

## Domain safety

General agents do not load isolated domain memories by default.

Escalasoft is isolated.

Escalasoft details do not enter general long-term memory.

## Dangerous execution

Do not use broad sandbox modes unless running inside a controlled container/workspace.

Any task with destructive commands must require approval.

## Path policy

A task must define allowed and forbidden paths.

If a diff touches forbidden paths, the run fails policy validation.
