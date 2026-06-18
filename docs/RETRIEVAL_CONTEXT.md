# Retrieval Context

Task 22.8 adds passive LanceDB retrieval context attachment for worker runs. Task 22.8.1 adds read-only inspection and run reporting over retrieval-context artifacts. Task 22.9 adds governed chunk text materialization from memory-index chunks JSONL. None of these paths execute LanceDB search, call embedding providers, or read memory source files directly.

## Scope

Implemented now:

- `retrieval_context.attachments` on task YAML
- kind `lancedb_search_report` only
- runner-side validation before worker execution
- `retrieval-context.md` and `retrieval-context.json` run artifacts
- passive prompt section `Retrieved context metadata only`
- execution trace fields: `retrieval_context_attached`, `retrieval_context_count`, `retrieval_context_sha256`, `retrieval_context_status`
- sanitized `RunSpec.Task` without attachment paths
- `deonctl retrieval context inspect` for `retrieval-context.json` QA
- `deonctl runs retrieval-report` for runs with `retrieval_context_attached` in execution traces
- `deonctl retrieval context materialize` for governed chunk text excerpts from chunks JSONL (explicit confirm flag)

Not implemented yet:

- natural-language retrieval or query embedding
- LanceDB search inside the runner
- automatic chunk text injection into worker prompts
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

## Inspection and reporting

Inspect a run artifact (no search, no provider calls):

~~~bash
deonctl retrieval context inspect \
  --artifact artifacts/<run-id>/retrieval-context.json
deonctl retrieval context inspect \
  --artifact artifacts/<run-id>/retrieval-context.json \
  --output-format json
~~~

`inspect` validates:

- `status: ok`
- `retrieval_context_attached` coherence with `hit_count`
- allowed attachment kinds
- hit shape (`rank`, `chunk_id`, `distance`)
- forbidden fields absent (`vector`, `text`, `chunk_text`, `content`, `embedding`)

Stdout never includes raw chunk text or full vectors.

List runs that attached retrieval context from the SQLite store:

~~~bash
deonctl runs retrieval-report --store deonclaw.db
deonctl runs retrieval-report --store deonclaw.db --run <run-id> --output-format json
~~~

`runs retrieval-report` reads execution traces and artifact paths only. It does not open memory source files and does not execute LanceDB search.

## Materialized chunk text (Task 22.9)

Runner attachment remains metadata-only. Materializing chunk text is a separate, explicit, confirmed step. The runner does not inject materialized text into worker prompts automatically.

~~~bash
deonctl retrieval context materialize \
  --retrieval-context artifacts/<run-id>/retrieval-context.json \
  --chunks artifacts/memory-index-chunks.jsonl \
  --output artifacts/<run-id>/retrieval-context-materialized.json \
  --summary artifacts/<run-id>/retrieval-context-materialized.md \
  --max-chars-per-chunk 1200 \
  --max-total-chars 6000 \
  --confirm-include-chunk-text
~~~

Rules:

- `--confirm-include-chunk-text` is required
- loads and inspects `retrieval-context.json`; fails when inspect status is not `ok`
- resolves chunk text only from `--chunks` JSONL (never opens memory source files)
- validates `text_sha256` for every hit; validates `source_sha256` when present on the hit
- `chunk_id` missing from chunks JSONL fails with a clear error (not a silent skip)
- applies `max_chars_per_chunk` and `max_total_chars`; truncates with explicit warnings and char counts
- output paths must be relative safe paths (no absolute, `..`, `secrets`, or `.env`)
- chunks path must be a relative safe path and must exist
- never includes vector, embedding, or environment values

Outputs:

- `retrieval-context-materialized.json` — governed excerpts with hashes and truncation metadata
- `retrieval-context-materialized.md` — human summary stating derived/non-canonical status

### Debugging `status: failed`

1. Run `deonctl retrieval context inspect --artifact ...` and read `failures`
2. Re-run `memory lancedb search-report` on the upstream `lancedb-search-smoke-result.json`
3. Confirm the worker run trace has `retrieval_context_status: ok` in `execution-trace.json`
4. Use `deonctl runs retrieval-report --store ... --run <run-id>` to locate `retrieval-context.json` / `.md` paths

## Safety

- passive attachment only; runner does not search LanceDB
- no embedding provider API calls
- no memory apply/restore changes
- metadata-only runner attachment; chunk text requires explicit `materialize` with confirm flag
- attachment paths stripped from worker `RunSpec.Task`

## Boundary

Tasks 22.7–22.7.1 own controlled vector search smoke and result QA. Task 22.8 owns passive runner attachment of validated metadata. Task 22.8.1 owns retrieval-context inspect and runs retrieval-report for passive attachment auditing. Task 22.9 owns governed chunk text materialization from chunks JSONL without runner auto-injection. Natural-language retrieval and active runner search remain future work. See docs/MEMORY_LANCEDB.md and docs/MEMORY_INDEX.md.
