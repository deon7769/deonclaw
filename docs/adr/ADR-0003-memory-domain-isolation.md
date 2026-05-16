# ADR-0003 — Memory Domain Isolation

## Status

Accepted for MVP planning.

## Decision

DeonClaw will treat memory as separated domains.

Initial domains:

- general: `mysecondbrain`
- escalasoft: `escalasoft_brain`

`mysecondbrain` remains the general memory base.

`escalasoft_brain` remains isolated and is not part of default general recall.

## Rationale

Escalasoft contains heavy operational knowledge, SDs, WMS/ERP/TMS material, SQL, clients, months, themes and cases.

This content is valuable but should not contaminate general memory.

## Rules

General agents do not load `escalasoft_brain` by default.

Escalasoft agents may read `escalasoft_brain`.

Escalasoft details must not be promoted into `MEMORY.md`.

Only bridge/governance files may exist in general memory.

## Consequences

Context pack building must be domain-aware.

Memory search must apply domain filters before retrieval.

Workers must receive only scoped context.
