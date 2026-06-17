# Retrieval Context

Task 22.8 adds passive LanceDB retrieval context attachment for worker runs. The runner validates prior `search-smoke` and `search-report` artifacts and injects metadata-only summaries into the worker prompt. It does not execute LanceDB search, call embedding providers, or read memory source files.

## Scope

Implemented now:

- `retrieval_context.attachments` on task YAML
- kind `lancedb_search_report` only
- runner-side validation before worker execution
- `retrieval-context.md` and `retrieval-context.json` run artifacts
- passive prompt section `Retrieved context metadata only`
- execution trace fields: `retrieval_context_attached`, `retrieval_context_count`, `retrieval_context_sha256`, `retrieval_context_status`
- sanitized `RunSpec.Task` without attachment paths

Not implemented yet:

- natural-language retrieval or query embedding
- LanceDB search inside the runner
- chunk text injection or memory source reads
- MCP integration
- UI/dashboard

## Prerequisites

Before attaching retrieval context to a worker task:

1. Build index chunks and embedding vectors (Tasks 22.0–22.3.1)
2. Run `memory lancedb write-smoke` to create a local table
3. Run `memory lancedb search-smoke` with an explicit query vector or chunk row (Task 22.7)
4. Run `memory lancedb search-report` and confirm `status: ok` (Task 22.7.1)

The runner re-validates the search result with `lancedbpolicy.SearchReport` at run time. It does not perform a new search.

## Task schema

~~~yaml
retrieval_context:
  attachments:
    - kind: lancedb_search_report
      path: artifacts/lancedb-search-smoke-result.json
      report_path: artifacts/lancedb-search-report.json
      policy: configs/examples/lancedb-policy-write-smoke.yaml
      max_results: 5
~~~

Rules:

- `kind` must be `lancedb_search_report`
- `path`, `report_path`, and `policy` are required
- `max_results` must be `> 0` and `<= 20`
- optional `name` defaults to `lancedb-search-N`

## Runner behavior

Before the worker starts, the runner:

1. checks `path`, `report_path`, and `policy` exist
2. loads the stored `search-report` artifact (`status: ok`)
3. runs `lancedbpolicy.SearchReport(result, policy)` and fails if `status != ok`
4. extracts up to `max_results` hits with safe metadata only

Safe metadata per hit:

- `rank`, `chunk_id`, `distance`
- optional `domain`, `source_path`, `source_sha256`, `text_sha256`, `embedding_model`, `provider`

Never attached:

- `vector`, `text`, `chunk_text`, `content`, `embedding`
- memory source file contents

The worker receives a sanitized task copy without `retrieval_context` paths. The composed prompt includes a passive metadata section only.

## Artifacts

Per run (under `artifacts/<run-id>/`):

- `retrieval-context.md` — human-readable metadata summary
- `retrieval-context.json` — machine-readable safe payload
- `execution-trace.json` — attachment status and SHA256

## Safety

- passive attachment only; runner does not search LanceDB
- no embedding provider API calls
- no memory apply/restore changes
- no chunk text injection
- attachment paths stripped from worker `RunSpec.Task`

## Boundary

Tasks 22.7–22.7.1 own controlled vector search smoke and result QA. Task 22.8 owns passive runner attachment of validated metadata. Natural-language retrieval and active runner search remain future work. See docs/MEMORY_LANCEDB.md and docs/MEMORY_INDEX.md.
