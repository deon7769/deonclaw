# Memory LanceDB

Task 22.4 adds LanceDB write policy validation and a plan-only dry run over existing embedding vector artifacts. Task 22.4.1 hardens the embedding report gate and adds fake-write artifact smoke. Task 22.5 adds controlled real local LanceDB write smoke via a Python adapter. Task 22.6 adds structural readback doctor/report over write-smoke databases. Task 22.7 adds controlled vector search smoke with an explicit query vector or existing chunk row. Task 22.7.1 adds search-smoke result QA/report — not natural-language retrieval and not runner integration.

## Scope

Implemented now:

- `lancedb-policy.yaml` loading and validation
- modes: `plan_only`, `fake_write`, and `write_smoke`
- `validate`, `plan`, `fake-write`, `write-smoke`, `doctor`, `report`, `search-smoke`, and `search-report` CLI commands
- embedding manifest and vector artifact loading
- hardened `embeddingpolicy.Report` (provider/model consistency)
- `embeddingpolicy.Report` consistency gate when `chunks_path` is configured
- row/dimension limit checks against policy
- auditable plan output with `would_write_lancedb: false`
- fake-write manifest/rows artifacts under `artifacts-dir`
- real local LanceDB table write smoke through `scripts/lancedb_write_smoke.py` (Python `lancedb` + `pyarrow`)
- write-smoke manifest, rows summary, and log artifacts
- structural readback doctor/report through `scripts/lancedb_readback.py` (no search)
- controlled vector search smoke through `scripts/lancedb_search_smoke.py` (explicit vector or chunk_id only; no natural-language retrieval)
- search-smoke result QA/report over `lancedb-search-smoke-result.json` (no new search)
- passive runner retrieval context attachment from validated search artifacts (see docs/RETRIEVAL_CONTEXT.md)

Not implemented yet:

- natural-language retrieval or active LanceDB search in runner
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

`write-smoke` preflight checks `python3`, `import lancedb`, and `scripts/lancedb_write_smoke.py`. `doctor` and `report` preflight checks `python3`, `import lancedb`, and `scripts/lancedb_readback.py`. `search-smoke` preflight checks `python3`, `import lancedb`, and `scripts/lancedb_search_smoke.py`. Override with `DEONCLAW_LANCEDB_PYTHON`, `DEONCLAW_LANCEDB_WRITE_SCRIPT`, `DEONCLAW_LANCEDB_READBACK_SCRIPT`, or `DEONCLAW_LANCEDB_SEARCH_SCRIPT` when needed.

`--artifacts-dir` for `fake-write`, `write-smoke`, and `search-smoke` must be a relative path under the policy input base (typically `artifacts`). Absolute paths, `..`, `secrets`, and `.env` segments are rejected. `database.path` must stay under `--artifacts-dir` for `write_smoke` and `search-smoke`.

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

Structural readback doctor:

~~~bash
deonctl memory lancedb doctor \
  --policy configs/examples/lancedb-policy-write-smoke.yaml
deonctl memory lancedb doctor \
  --policy configs/examples/lancedb-policy-write-smoke.yaml \
  --output-format json
~~~

Write-smoke manifest cross-check report:

~~~bash
deonctl memory lancedb report \
  --manifest artifacts/lancedb-write-smoke-manifest.json \
  --policy configs/examples/lancedb-policy-write-smoke.yaml
deonctl memory lancedb report \
  --manifest artifacts/lancedb-write-smoke-manifest.json \
  --policy configs/examples/lancedb-policy-write-smoke.yaml \
  --output-format json
~~~

Controlled vector search smoke (explicit query vector or existing chunk row; not natural-language retrieval):

~~~bash
deonctl memory lancedb search-smoke \
  --policy configs/examples/lancedb-policy-write-smoke.yaml \
  --query-vector '[0.01,0.02,0.03,0.04,0.05,0.06,0.07,0.08,0.09,0.10,0.11,0.12,0.13,0.14,0.15,0.16]' \
  --top-k 5 \
  --artifacts-dir artifacts \
  --confirm-search-smoke

deonctl memory lancedb search-smoke \
  --policy configs/examples/lancedb-policy-write-smoke.yaml \
  --query-chunk-id chunk-a \
  --top-k 5 \
  --artifacts-dir artifacts \
  --confirm-search-smoke
~~~

Search-smoke result QA/report:

~~~bash
deonctl memory lancedb search-report \
  --result artifacts/lancedb-search-smoke-result.json \
  --policy configs/examples/lancedb-policy-write-smoke.yaml
deonctl memory lancedb search-report \
  --result artifacts/lancedb-search-smoke-result.json \
  --policy configs/examples/lancedb-policy-write-smoke.yaml \
  --output-format json
~~~

### Command roles

| Command | Purpose |
|---------|---------|
| `validate` | Policy schema, paths, limits, and identifier safety |
| `plan` | Dry-run LanceDB write plan over validated embedding artifacts |
| `fake-write` | Run plan, then write fake row/manifest artifacts for table-shape smoke |
| `write-smoke` | Run plan + embedding report + preflight, then write a real local LanceDB table |
| `doctor` | Structural readback over an existing write-smoke database/table (no search) |
| `report` | Cross-check write-smoke manifest against policy + readback metadata |
| `search-smoke` | Controlled top-k vector search using `--query-vector` or `--query-chunk-id` after doctor passes |
| `search-report` | Validate `lancedb-search-smoke-result.json` contract before any future context attachment |

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

`doctor` reports `status`, `database_path`, `table`, `table_exists`, `row_count`, `expected_dimensions`, `inferred_dimensions`, `columns`, and `warnings`. It fails when the database or table is missing, `row_count == 0`, required columns are absent, or inferred dimensions diverge.

`report` validates write-smoke manifest flags (`lancedb_written: true`, `retrieval_performed: false`, `runner_integration: false`), manifest/policy path alignment, and manifest/readback `row_count` and dimensions. Output includes `sample_chunk_ids` only — no full vectors or chunk text.

`search-smoke` requires:

- `--confirm-search-smoke`
- exactly one of `--query-vector` (JSON array) or `--query-chunk-id`
- `--top-k` between 1 and 50 inclusive
- passing `memory lancedb doctor` (`status: ok`) before search runs
- `database.path` under `--artifacts-dir`
- `--query-vector` length must equal `limits.expected_dimensions`

It does **not** generate embeddings from natural language, call a real embedding provider, or integrate with the runner.

It writes:

- `artifacts/lancedb-search-smoke-result.json` (ranked hits with metadata only)
- `artifacts/lancedb-search-smoke-summary.md` (`retrieval_performed: true`, `runner_integration: false`)
- `artifacts/lancedb-search-smoke.log` (step trace without vectors)

Stdout and artifacts never include full vectors or chunk text.

`search-report` validates a prior `search-smoke` result artifact without running a new search. It checks summary flags (`retrieval_performed: true`, `runner_integration: false`), `top_k`/`result_count` bounds, `query_mode`, ranked hit shape, forbidden payload fields (`vector`, `text`, `chunk_text`, `content`, `embedding`), and optional policy cross-checks for `database_path`/`table`. Output includes `status`, distance bounds, `unique_chunk_ids`, `invalid_results`, and `warnings` — never full vectors or chunk text.

Task 22.8 uses a passing `search-report` as a prerequisite for passive runner attachment via `retrieval_context` on worker tasks. Task 22.8.1 adds `retrieval context inspect` and `runs retrieval-report` for auditing passive attachments without search. See docs/RETRIEVAL_CONTEXT.md.

## Adapters

Go uses `internal/lancedbpolicy` adapter interfaces:

- `LanceDBWriter` + `scripts/lancedb_write_smoke.py` for write-smoke
- `LanceDBReader` + `scripts/lancedb_readback.py` for doctor/report readback
- `LanceDBSearcher` + `scripts/lancedb_search_smoke.py` for search-smoke

There is no Go LanceDB SDK dependency.

## Safety

- search-smoke is controlled vector retrieval only (explicit vector or chunk row); not natural-language retrieval
- no embedding provider API calls from DeonClaw LanceDB paths
- no runner integration
- memory apply/restore unchanged
- `plan` and `fake-write` do not create `database.path`
- stdout/summary/log never include full vectors or chunk text
- preflight fails clearly when Python `lancedb` or the helper script is missing
- no automatic dependency installation
- no network calls from DeonClaw LanceDB paths

## Boundary

Tasks 22.0–22.1 own chunk build and QA. Tasks 22.2–22.3.1 own embedding policy, fake vectors, and vector report. Tasks 22.4–22.5 own LanceDB write planning, fake-write artifact smoke, and real local write smoke. Task 22.6 owns structural readback doctor/report. Task 22.7 owns controlled vector search smoke with explicit query vectors or chunk rows. Task 22.7.1 owns search-smoke result QA/report. Task 22.8 owns passive runner retrieval context attachment — not natural-language retrieval and not runner search. See also docs/MEMORY_EMBEDDINGS.md, docs/MEMORY_INDEX.md, and docs/RETRIEVAL_CONTEXT.md.
