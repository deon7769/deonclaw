# Memory Embeddings

Task 22.2 adds embedding provider policy and dry-run diagnostics. It prepares a safe path toward vector generation without calling embedding APIs, writing vectors, or touching LanceDB.

## Scope

Implemented now:

- `embedding-policy.yaml` loading and validation
- `validate`, `doctor`, and `plan` CLI commands
- env requirement checks (`set_masked` / `missing` only; never prints values)
- input artifact existence and chunk counts
- batch estimation (`estimated_batches`)
- memory index consistency checks via `memoryindex.Report` during `plan`
- `dry_run` mode only

Not implemented yet:

- real embedding provider calls
- vector generation or `memory-index-vectors.jsonl` writes
- LanceDB writes or queries
- runner retrieval integration
- MCP integration
- UI/dashboard

## Config

Example:

~~~bash
configs/examples/embedding-policy.yaml
~~~

Shape:

~~~yaml
embedding_policy:
  provider: openai
  model: text-embedding-3-small
  dimensions: 1536
  input:
    chunks_path: artifacts/memory-index-chunks.jsonl
    manifest_path: artifacts/memory-index-manifest.json
  output:
    vectors_path: artifacts/memory-index-vectors.jsonl
    manifest_path: artifacts/memory-embedding-manifest.json
  env:
    required:
      - OPENAI_API_KEY
  limits:
    max_chunks: 10000
    max_chunk_chars: 8000
    batch_size: 64
  mode: dry_run
~~~

Validation rules:

- `provider` must be `openai`, `zai`, or `local`
- `model` is required
- `dimensions > 0`
- `mode` must be `dry_run` (only supported mode for now)
- `limits.max_chunks`, `limits.max_chunk_chars`, and `limits.batch_size` must be `> 0`
- input `chunks_path` and `manifest_path` are required relative paths
- output paths must be relative, under the artifacts dir inferred from input `chunks_path`, and must not contain `..`, `secrets`, or `.env`
- `env.required` entries must be environment variable names only (`OPENAI_API_KEY`), never `NAME=value`

## CLI

Validate policy schema and paths:

~~~bash
deonctl memory embedding validate --policy configs/examples/embedding-policy.yaml
~~~

Pre-flight diagnostics (no network, no embedding calls):

~~~bash
deonctl memory embedding doctor --policy configs/examples/embedding-policy.yaml
deonctl memory embedding doctor --policy configs/examples/embedding-policy.yaml --output-format json
~~~

Dry-run execution plan:

~~~bash
deonctl memory embedding plan --policy configs/examples/embedding-policy.yaml
deonctl memory embedding plan --policy configs/examples/embedding-policy.yaml --output-format json
~~~

### Command roles

| Command | Purpose |
|---------|---------|
| `validate` | Policy schema, paths, limits, and env name shape |
| `doctor` | Provider/model/env/input readiness before any embedding work |
| `plan` | Dry-run batch plan over validated index artifacts |

`doctor` reports provider, model, dimensions, env requirements (`set_masked` / `missing`), input file existence, `chunk_count`, `manifest_chunk_count`, `estimated_batches`, and warnings. It never prints environment values.

`plan` reads manifest/chunks, runs `memoryindex.Report` consistency checks when inputs exist, and reports:

- `chunk_count`, `max_chunks`, `batch_size`, `estimated_batches`
- output paths
- `would_call_provider: false`
- `dry_run_only: true`

If `chunk_count > max_chunks`, `plan` returns `status: failed` and a non-zero exit code. `doctor` reports the same condition as a warning.

## Safety

- no embedding API calls
- no vector generation
- no LanceDB writes
- no network calls
- env values are never printed
- memory apply/restore unchanged
- runner retrieval unchanged

## Boundary

Tasks 22.0 and 22.1 own chunk build and QA. Task 22.2 owns provider policy and dry-run only. Real embeddings and retrieval remain future work.
