# Memory LanceDB

Task 22.4 adds LanceDB write policy validation and a plan-only dry run over existing embedding vector artifacts. Task 22.4.1 hardens the embedding report gate and adds fake-write artifact smoke. Neither task imports LanceDB, writes a real database, runs retrieval, or integrates with the runner.

## Scope

Implemented now:

- `lancedb-policy.yaml` loading and validation
- modes: `plan_only` (contract validation) and `fake_write` (artifact smoke only)
- `validate`, `plan`, and `fake-write` CLI commands
- embedding manifest and vector artifact loading
- hardened `embeddingpolicy.Report` (provider/model consistency)
- `embeddingpolicy.Report` consistency gate when `chunks_path` is configured
- row/dimension limit checks against policy
- auditable plan output with `would_write_lancedb: false`
- fake-write manifest (`lancedb-fake-write-manifest.json`) and rows (`lancedb-fake-rows.jsonl`) under `artifacts-dir`

Not implemented yet:

- LanceDB SDK import or real database writes
- retrieval CLI or runner integration
- real embedding provider calls
- MCP integration
- UI/dashboard

## Modes

| Mode | Purpose |
|------|---------|
| `plan_only` | Validate policy and report a dry-run write plan; no artifacts beyond plan output |
| `fake_write` | Run plan, then emit fake row/manifest artifacts for table-shape smoke; not a database |

`fake_write` never imports LanceDB, never creates `database.path`, and never sets `lancedb_written: true`.

## Prerequisites

Before planning or fake-writing LanceDB rows:

1. Build memory index chunks (`memory index build`)
2. Generate embedding vectors (`memory embedding build-fake` for smoke, or a future real embedding task)
3. Pass embedding vector QA (`memory embedding report`), including provider/model consistency checks

`memory lancedb plan` and `memory lancedb fake-write` fail when the embedding report status is `failed`, when `vector_count` exceeds `max_vectors`, or when manifest dimensions differ from `expected_dimensions`.

## Config

Plan-only example:

~~~bash
configs/examples/lancedb-policy.yaml
~~~

Fake-write smoke example:

~~~bash
configs/examples/lancedb-policy-fake.yaml
~~~

Shape:

~~~yaml
lancedb_policy:
  input:
    embedding_manifest: artifacts/memory-embedding-manifest.json
    vectors_path: artifacts/memory-index-vectors.jsonl
    chunks_path: artifacts/memory-index-chunks.jsonl
  database:
    path: artifacts/lancedb-fake
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

- `mode` must be `plan_only` or `fake_write`
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

Fake-write artifact smoke (no LanceDB import, no `database.path` creation):

~~~bash
deonctl memory lancedb fake-write \
  --policy configs/examples/lancedb-policy-fake.yaml \
  --artifacts-dir artifacts \
  --confirm-fake-write
~~~

### Command roles

| Command | Purpose |
|---------|---------|
| `validate` | Policy schema, paths, limits, and identifier safety |
| `plan` | Dry-run LanceDB write plan over validated embedding artifacts |
| `fake-write` | Run plan, then write fake row/manifest artifacts for table-shape smoke |

`plan` reports:

- `database_path`, `table`
- `vector_count`, `expected_dimensions`, `manifest_dimensions`
- `provider`, `model`, `fake_vectors`, `lancedb_written`
- `would_write_lancedb: false`
- `plan_only: true` when `mode: plan_only`
- `estimated_rows`
- `embedding_report_status`
- `warnings`

`fake-write` requires:

- `mode: fake_write`
- `--confirm-fake-write`
- passing `memory lancedb plan` first (`status: ok`)

It writes:

- `artifacts/lancedb-fake-write-manifest.json`
- `artifacts/lancedb-fake-rows.jsonl`

Each fake row includes metadata fields and `vector_sha256` only — not full vectors or chunk text.

## Planned row shape

Future LanceDB writes will map vector JSONL records into a table with:

- `vector` column (float array)
- `chunk_id` text reference column
- metadata columns: `domain`, `source_path`, `source_sha256`, `text_sha256`, `embedding_model`, `provider`

`fake_write` validates the metadata/table contract via JSONL artifacts only.

## Safety

- no LanceDB import or real writes
- no retrieval
- no network calls
- no runner integration
- memory apply/restore unchanged
- `plan` and `fake-write` do not create `database.path`
- fake rows/manifests never include full vectors or chunk text

## Boundary

Tasks 22.0–22.1 own chunk build and QA. Tasks 22.2–22.3.1 own embedding policy, fake vectors, and vector report. Tasks 22.4–22.4.1 own LanceDB write planning and fake-write artifact smoke. Real LanceDB writes and retrieval remain future work. See also docs/MEMORY_EMBEDDINGS.md and docs/MEMORY_INDEX.md.
