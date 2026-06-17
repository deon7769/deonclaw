# Memory LanceDB

Task 22.4 adds LanceDB write policy validation and a plan-only dry run over existing embedding vector artifacts. Task 22.4.1 hardens the embedding report gate and adds fake-write artifact smoke. Task 22.5 adds controlled real local LanceDB write smoke via a Python adapter. None of these tasks run retrieval/search or integrate with the runner.

## Scope

Implemented now:

- `lancedb-policy.yaml` loading and validation
- modes: `plan_only`, `fake_write`, and `write_smoke`
- `validate`, `plan`, `fake-write`, and `write-smoke` CLI commands
- embedding manifest and vector artifact loading
- hardened `embeddingpolicy.Report` (provider/model consistency)
- `embeddingpolicy.Report` consistency gate when `chunks_path` is configured
- row/dimension limit checks against policy
- auditable plan output with `would_write_lancedb: false`
- fake-write manifest/rows artifacts under `artifacts-dir`
- real local LanceDB table write smoke through `scripts/lancedb_write_smoke.py` (Python `lancedb` + `pyarrow`)
- write-smoke manifest, rows summary, and log artifacts

Not implemented yet:

- retrieval/search CLI or runner integration
- real embedding provider calls
- MCP integration
- UI/dashboard

## Modes

| Mode | Purpose |
|------|---------|
| `plan_only` | Validate policy and report a dry-run write plan; no artifacts beyond plan output |
| `fake_write` | Run plan, then emit fake row/manifest artifacts for table-shape smoke; not a database |
| `write_smoke` | Run plan + embedding report, preflight deps, then write a real local LanceDB table |

`fake_write` never imports LanceDB, never creates `database.path`, and never sets `lancedb_written: true`.

`write_smoke` creates a real LanceDB dataset under `database.path` (must be under `--artifacts-dir`), sets `lancedb_written: true` in the write-smoke manifest only, and never performs retrieval/search.

## Prerequisites

Before planning, fake-writing, or write-smoking LanceDB rows:

1. Build memory index chunks (`memory index build`)
2. Generate embedding vectors (`memory embedding build-fake` for smoke, or a future real embedding task)
3. Pass embedding vector QA (`memory embedding report`), including provider/model consistency checks

All LanceDB write paths fail when the embedding report status is `failed`, when `vector_count` exceeds `max_vectors`, or when manifest dimensions differ from `expected_dimensions`.

For `write_smoke`, install local Python dependencies manually (no auto-install in DeonClaw):

~~~bash
pip install lancedb pyarrow
~~~

`write-smoke` preflight checks `python3`, `import lancedb`, and `scripts/lancedb_write_smoke.py`. Override with `DEONCLAW_LANCEDB_PYTHON` or `DEONCLAW_LANCEDB_WRITE_SCRIPT` when needed.

## Config

Plan-only example:

~~~bash
configs/examples/lancedb-policy.yaml
~~~

Fake-write smoke example:

~~~bash
configs/examples/lancedb-policy-fake.yaml
~~~

Write-smoke example:

~~~bash
configs/examples/lancedb-policy-write-smoke.yaml
~~~

Shape:

~~~yaml
lancedb_policy:
  input:
    embedding_manifest: artifacts/memory-embedding-manifest.json
    vectors_path: artifacts/memory-index-vectors.jsonl
    chunks_path: artifacts/memory-index-chunks.jsonl
  database:
    path: artifacts/lancedb-smoke
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
  mode: write_smoke
~~~

Validation rules:

- `mode` must be `plan_only`, `fake_write`, or `write_smoke`
- `input.embedding_manifest` and `input.vectors_path` are required relative paths
- `input.chunks_path` is optional; when set it must be a relative path
- `database.path` must be relative and must not contain `..`, `secrets`, or `.env`
- for `write_smoke`, `database.path` must be under `--artifacts-dir`
- `database.table`, `schema.vector_column`, `schema.text_ref_column`, and metadata column names must be safe identifiers (`^[A-Za-z][A-Za-z0-9_]*$`)
- `limits.max_vectors` and `limits.expected_dimensions` must be `> 0`

## CLI

Validate policy schema and paths:

~~~bash
deonctl memory lancedb validate --policy configs/examples/lancedb-policy.yaml
~~~

Plan-only write dry run:

~~~bash
deonctl memory lancedb plan --policy configs/examples/lancedb-policy.yaml
deonctl memory lancedb plan --policy configs/examples/lancedb-policy.yaml --output-format json
~~~

Fake-write artifact smoke:

~~~bash
deonctl memory lancedb fake-write \
  --policy configs/examples/lancedb-policy-fake.yaml \
  --artifacts-dir artifacts \
  --confirm-fake-write
~~~

Real local LanceDB write smoke:

~~~bash
deonctl memory lancedb write-smoke \
  --policy configs/examples/lancedb-policy-write-smoke.yaml \
  --artifacts-dir artifacts \
  --confirm-lancedb-write
~~~

Optional overwrite when `database.path` already exists and is non-empty:

~~~bash
deonctl memory lancedb write-smoke \
  --policy configs/examples/lancedb-policy-write-smoke.yaml \
  --artifacts-dir artifacts \
  --confirm-lancedb-write \
  --allow-overwrite-smoke
~~~

### Command roles

| Command | Purpose |
|---------|---------|
| `validate` | Policy schema, paths, limits, and identifier safety |
| `plan` | Dry-run LanceDB write plan over validated embedding artifacts |
| `fake-write` | Run plan, then write fake row/manifest artifacts for table-shape smoke |
| `write-smoke` | Run plan + embedding report + preflight, then write a real local LanceDB table |

`write-smoke` requires:

- `mode: write_smoke`
- `--confirm-lancedb-write`
- passing `memory lancedb plan` (`status: ok`)
- passing `memory embedding report` with chunks when configured
- successful Python/LanceDB preflight
- `database.path` under `--artifacts-dir`
- empty `database.path` by default (or `--allow-overwrite-smoke`)

It writes:

- `artifacts/lancedb-write-smoke-manifest.json` (`lancedb_written: true`, `retrieval_performed: false`)
- `artifacts/lancedb-write-smoke-rows-summary.json` (aggregates + up to 5 `sample_chunk_ids`; no full vectors or chunk text)
- `artifacts/lancedb-write-smoke.log` (step trace without vectors)

Full vectors are stored only inside the LanceDB database directory, never in stdout or summary artifacts.

## Adapter

Go uses `internal/lancedbpolicy` adapter interface `LanceDBWriter`. Production path invokes `scripts/lancedb_write_smoke.py` via `python3`. There is no Go LanceDB SDK dependency in this task.

## Safety

- no retrieval/search
- no network calls from DeonClaw write-smoke path
- no runner integration
- memory apply/restore unchanged
- `plan` and `fake-write` do not create `database.path`
- stdout/summary/log never include full vectors or chunk text
- preflight fails clearly when Python `lancedb` or the helper script is missing
- no automatic dependency installation

## Boundary

Tasks 22.0–22.1 own chunk build and QA. Tasks 22.2–22.3.1 own embedding policy, fake vectors, and vector report. Tasks 22.4–22.4.1 own LanceDB write planning and fake-write artifact smoke. Task 22.5 owns controlled real local LanceDB write smoke only. Retrieval and runner integration remain future work. See also docs/MEMORY_EMBEDDINGS.md and docs/MEMORY_INDEX.md.
