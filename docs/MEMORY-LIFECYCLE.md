# Memory Lifecycle

## Purpose

This document defines how DeonClaw should handle memory creation, improvement, lapidation, indexing and renewal.

## Source of truth

Markdown + Git are the source of truth.

For general memory, the source is `mysecondbrain`.

For Escalasoft, the source is `escalasoft_brain`.

A retrieval/index database such as LanceDB is derived state.

It can help search, map and retrieve memory, but it must not become the canonical memory source.

## Lifecycle states

### 1. Capture

Raw material enters the system.

Examples:

- agent summary
- run output
- daily note
- task result
- log triage
- useful command sequence
- bug diagnosis
- recurring fix pattern

Destination:

- inbox
- scratch
- proposals
- task artifacts

Rules:

- raw capture can be noisy
- raw capture is not canonical
- raw capture should include source metadata

### 2. Lapidation

Raw material is cleaned and structured.

Actions:

- dedupe
- classify
- chunk
- link
- summarize
- identify domain
- separate fact from inference
- remove secrets
- identify target layer

Output:

- memory proposal
- context update proposal
- reference update proposal
- skill update proposal

### 3. Canonization

A proposal is promoted into long-lived memory.

Rules:

- canonical memory must be concise
- durable memory needs evidence
- domain memory must stay inside its domain
- general memory must not absorb domain details
- Escalasoft details must not enter `MEMORY.md`

### 4. Renewal

Existing memory is reviewed.

Triggers:

- stale date
- broken links
- repeated agent failure
- changed tool behavior
- changed project state
- conflicting memories
- duplicated content
- obsolete workflow

Output:

- update proposal
- archive proposal
- merge proposal
- skill renewal proposal
- domain index update

## Memory proposal format

The current proposal artifact is `memory-proposal.json`.

```json
{
  "proposal_id": "mem-20260523T120000Z",
  "run_id": "run-001",
  "task_id": "task-001",
  "domain": "general",
  "target_path": "memory/context/example.md",
  "operation": "append",
  "status": "proposed",
  "reason": "Capture a stable workflow discovered during the run.",
  "evidence": [
    {
      "type": "run_artifact",
      "path": "artifacts/run-001/summary.md",
      "run_id": "run-001",
      "description": "Worker summary that supports the proposed memory update."
    }
  ],
  "created_at": "2026-05-23T12:00:00Z",
  "patches": [
    {
      "target_path": "memory/context/example.md",
      "operation": "append",
      "content": "New durable context.\n"
    }
  ]
}
```

Supported operations are `create`, `update`, `append` and `archive`.

`status` is currently always `proposed`.

`patches` are validated during apply dry-run and patch-level policy checks.

## Domain boundaries

### General domain

Allowed:

- goals
- personal operating rules
- general infra
- product direction
- high-level work state

Forbidden:

- Escalasoft SD details
- raw SQL snapshots
- client-specific WMS analysis
- operational dumps
- long domain case studies

### Escalasoft domain

Allowed:

- SD/HD analysis
- WMS/ERP/TMS docs
- cases
- clients
- themes
- months
- SQL/SQLite-derived summaries
- live ServiceDesk-derived notes

Forbidden:

- personal memory
- Nutri memory
- unrelated infra memory
- direct writes to `mysecondbrain/MEMORY.md`

## Indexing with LanceDB or similar

Use index storage for:

- semantic search
- hybrid lookup
- chunk mapping
- stale memory detection
- duplicate detection
- nearest related notes
- context pack candidate selection

Do not use index storage for:

- source of truth
- silent memory rewriting
- bypassing domain policy
- replacing Git history
- storing secrets

## Suggested index schema

```yaml
id: stable_chunk_id
domain: general | escalasoft | infra | nutri
source_repo: mysecondbrain | escalasoft_brain
source_path: memory/context/example.md
source_sha: git_blob_or_commit_sha
chunk_hash: sha256
chunk_type: note | project | context | skill | decision | reference | case
title: string
tags: []
updated_at: timestamp
embedding_model: string
text: string
```

## Index rebuild rules

- Rebuild must be reproducible.
- Rebuild must not modify source Markdown.
- Every result must point back to source path and chunk ID.
- Domain filtering must happen before retrieval.
- Retrieval results must be explainable.

## Current memory commands

- `deonctl memory proposal new`
- `deonctl memory proposal lint`
- `deonctl memory proposal apply --dry-run`
- `deonctl memory proposal approve`
- `deonctl memory proposal apply-preflight`
- `deonctl memory proposal backup-plan`
- `deonctl memory proposal backup-materialize`

## Future memory commands

- `deonctl memory proposal apply`
- `deonctl memory restore`
- `deonctl memory scan`
- `deonctl memory renew`
- `deonctl memory index rebuild`

## MVP boundary

The MVP does not need a full memory index.

The MVP only needs:

- domain config
- context pack builder
- memory proposal format
- memory lint
- no direct canonical writes by workers
