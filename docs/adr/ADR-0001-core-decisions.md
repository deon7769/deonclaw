# ADR-0001 — Core MVP Decisions

## Status

Accepted for MVP planning.

## Decision

DeonClaw will start as a Go-based control plane.

It will not build a full agent from scratch.

It will wrap external workers and enforce task, memory and path policy.

## Rationale

The hard problem for this project is not generating code.

The hard problem is controlling:

- scope
- memory boundaries
- execution traceability
- worker selection
- workspace isolation
- approvals
- artifacts

Go is a good fit for the control plane because it works well for CLIs, daemons, process control, filesystem operations, SQLite and Docker orchestration.

## Consequences

The MVP must avoid UI complexity and model-provider complexity.

The first useful version should be CLI-driven.

Workers remain external tools.
