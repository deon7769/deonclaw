# ADR-0004 — Optional Memory Index with LanceDB

## Status

Proposed.

## Decision

DeonClaw may use LanceDB or a similar embedded/vector retrieval layer as a derived memory index.

The index is not canonical.

Markdown + Git remain the source of truth.

## Rationale

A memory index can help with:

- semantic search
- chunk discovery
- duplicate detection
- stale note mapping
- context pack candidate selection
- skill-memory linkage
- domain map generation

LanceDB is a good candidate because it supports local/open-source usage and vector/full-text/SQL-style retrieval capabilities.

## Rules

The index must:

- be rebuildable from source Markdown
- store source metadata
- respect domain boundaries
- never silently write canonical memory
- never store secrets intentionally
- cite source paths/chunks when used

## MVP boundary

The MVP does not need this adapter.

The MVP should define the interface and keep the implementation optional.

## Future commands

```bash
deonctl memory index rebuild --domain general
deonctl memory index search --domain escalasoft "WMS coletor tarefa"
deonctl memory renew --domain general --stale-days 90
```
