# Memory LanceDB

Task 22.4 adds LanceDB write policy validation and a plan-only dry run over existing embedding vector artifacts. It does not import LanceDB, write a database, run retrieval, or integrate with the runner.

## Scope

Implemented now:

- `lancedb-policy.yaml` loading and validation
- `validate` and `plan` CLI commands
- embedding manifest and vector artifact loading
- `embeddingpolicy.Report` consistency gate when `chunks_path` is configured
- row/dimension limit checks against policy
- auditable plan output with `would_write_lancedb: false` and `plan_only: true`

Not implemented yet:

- LanceDB SDK import or database writes
- retrieval CLI or runner integration
- real embedding provider calls
- MCP integration
- UI/dashboard

## Prerequisites

Before planning a LanceDB write:

1. Build memory index chunks (`memory index build`)
2. Generate embedding vectors (`memory embedding build-fake` for smoke, or a future real embedding task)
3. Pass embedding vector QA (`memory embedding report`)

`memory lancedb plan` fails when the embedding report status is `failed`, when `vector_count` exceeds `max_vectors`, or when manifest dimensions differ from `expected_dimensions`.

## Config

Example:

~~~bash
configs/examples/lancedb-policy.yaml
~~~

Shape:

~~~yaml
lancedb_policy:
  input:
    embedding_manifest: artifacts/memory-embedding-manifest.json
    vectors_path: artifacts/memory-index-vectors.jsonl
    chunks_path: artifacts/memory-index-chunks.jsonl
  database:
    path: artifacts/lancedb
    table: memory_vectors
  schema:
    vector_column: vector
    text_ref_column: chunk_id
    metadata_columns:
      - domain
      - source_path
      - source_sha256
      - text_sha256
      - embedding_model
      - provider
  limits:
    max_vectors: 10000
    expected_dimensions: 16
  mode: plan_only
~~~

Validation rules:

- `mode` must be `plan_only` (only supported mode in this task)
- `input.embedding_manifest` and `input.vectors_path` are required relative paths
- `input.chunks_path` is optional; when set it must be a relative path
- `database.path` must be relative and must not contain `..`, `secrets`, or `.env`
- `database.table`, `schema.vector_column`, `schema.text_ref_column`, and metadata column names must be safe identifiers (`^[A-Za-z][A-Za-z0-9_]*$`)
- `limits.max_vectors` and `limits.expected_dimensions` must be `> 0`

## CLI

Validate policy schema and paths:

~~~bash
deonctl memory lancedb validate --policy configs/examples/lancedb-policy.yaml
~~~

Plan-only write dry run (no LanceDB import, no directory creation):

~~~bash
deonctl memory lancedb plan --policy configs/examples/lancedb-policy.yaml
deonctl memory lancedb plan --policy configs/examples/lancedb-policy.yaml --output-format json
~~~

### Command roles

| Command | Purpose |
|---------|---------|
| `validate` | Policy schema, paths, limits, and identifier safety |
| `plan` | Dry-run LanceDB write plan over validated embedding artifacts |

`plan` reports:

- `database_path`, `table`
- `vector_count`, `expected_dimensions`, `manifest_dimensions`
- `provider`, `model`, `fake_vectors`, `lancedb_written`
- `would_write_lancedb: false`
- `plan_only: true`
- `estimated_rows`
- `embedding_report_status`
- `warnings`

If `vector_count > max_vectors` or dimensions mismatch, `plan` returns `status: failed` and a non-zero exit code. The plan step never creates the database directory.

## Planned row shape

Future LanceDB writes will map vector JSONL records into a table with:

- `vector` column (float array)
- `chunk_id` text reference column
- metadata columns: `domain`, `source_path`, `source_sha256`, `text_sha256`, `embedding_model`, `provider`

This task validates the contract only; no rows are written.

## Safety

- no LanceDB import or writes
- no retrieval
- no network calls
- no runner integration
- memory apply/restore unchanged
- plan does not create `database.path`

## Boundary

Tasks 22.0–22.1 own chunk build and QA. Tasks 22.2–22.3.1 own embedding policy, fake vectors, and vector report. Task 22.4 owns LanceDB write planning only. Real LanceDB writes and retrieval remain future work. See also docs/MEMORY_EMBEDDINGS.md and docs/MEMORY_INDEX.md.
