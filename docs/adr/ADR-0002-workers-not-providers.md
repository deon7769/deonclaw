# ADR-0002 — Workers, Not Providers

## Status

Accepted for MVP planning.

## Decision

DeonClaw will initially integrate with harnesses/workers, not model providers directly.

Initial workers:

- Codex CLI
- OpenCode
- Claude Code later
- OpenClaw adapter later

## Rationale

Each harness already handles model-specific authentication, tool calling, prompt structure and execution semantics.

DeonClaw should focus on orchestration, policy and traceability.

## Auth

MVP uses `harness_managed` auth.

DeonClaw must not store OAuth tokens.

## Consequences

Provider-specific control will be limited at first.

This is acceptable because the MVP goal is to control execution, not replace every harness.
