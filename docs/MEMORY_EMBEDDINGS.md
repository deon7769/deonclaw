# Memory Embeddings

Task 22.2 adds embedding provider policy and dry-run diagnostics. Task 22.3 adds deterministic local fake vector generation for smoke testing. Task 22.3.1 adds post-build vector QA. Neither task calls real embedding APIs, writes LanceDB, or integrates retrieval into the runner.

## Scope

Implemented now:

- `embedding-policy.yaml` loading and validation
- `validate`, `doctor`, `plan`, `build-fake`, and `report` CLI commands
- env requirement checks (`set_masked` / `missing` only; never prints values)
- input artifact existence and chunk counts
- batch estimation (`estimated_batches`)
- memory index consistency checks via `memoryindex.Report` during `plan` and `build-fake`
- modes: `dry_run` (default example policy) and `fake_vectors` (local deterministic smoke)
- auditable `memory-index-vectors.jsonl` and `memory-embedding-manifest.json` for fake vectors only

Not implemented yet:

- real embedding provider calls (OpenAI, Z.ai, or local model runtime)
- semantic-quality embeddings
- LanceDB writes or queries
- runner retrieval integration
- MCP integration
- UI/dashboard

## Config

Dry-run example:

~~~bash
configs/examples/embedding-policy.yaml
~~~

Fake-vector smoke example:

~~~bash
configs/examples/embedding-policy-fake.yaml
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

Fake smoke policy uses:

~~~yaml
embedding_policy:
  provider: local
  model: deterministic-hash-v1
  dimensions: 16
  mode: fake_vectors
~~~

Validation rules:

- `provider` must be `openai`, `zai`, or `local`
- `model` is required
- `dimensions > 0`
- `mode` must be `dry_run` or `fake_vectors`
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

Deterministic local fake vectors (no provider API, requires explicit confirmation):

~~~bash
deonctl memory embedding build-fake \
  --policy configs/examples/embedding-policy-fake.yaml \
  --artifacts-dir artifacts \
  --confirm-fake-vectors
~~~

Post-build QA over generated vector artifacts (read-only, no network):

~~~bash
deonctl memory embedding report \
  --manifest artifacts/memory-embedding-manifest.json \
  --vectors artifacts/memory-index-vectors.jsonl
deonctl memory embedding report \
  --manifest artifacts/memory-embedding-manifest.json \
  --vectors artifacts/memory-index-vectors.jsonl \
  --chunks artifacts/memory-index-chunks.jsonl \
  --output-format json
~~~

### Command roles

| Command | Purpose |
|---------|---------|
| `validate` | Policy schema, paths, limits, and env name shape |
| `doctor` | Provider/model/env/input readiness before any embedding work |
| `plan` | Dry-run batch plan over validated index artifacts |
| `build-fake` | Deterministic local vector smoke from validated chunks |
| `report` | Post-build QA over embedding manifest and vectors JSONL |

`doctor` reports provider, model, dimensions, env requirements (`set_masked` / `missing`), input file existence, `chunk_count`, `manifest_chunk_count`, `estimated_batches`, and warnings. It never prints environment values. For `provider: local`, missing env vars are warnings only when `env.required` is configured but not required for fake smoke.

`plan` reads manifest/chunks, runs `memoryindex.Report` consistency checks when inputs exist, and reports:

- `chunk_count`, `max_chunks`, `batch_size`, `estimated_batches`
- output paths
- `would_call_provider: false`
- `dry_run_only: true` when `mode: dry_run`

If `chunk_count > max_chunks`, `plan` returns `status: failed` and a non-zero exit code. `doctor` reports the same condition as a warning.

`build-fake` requires:

- `provider: local`
- `mode: fake_vectors`
- `--confirm-fake-vectors`
- passing `memoryindex.Report` before generation
- `chunk_count <= max_chunks`

It writes vector JSONL lines with deterministic hashes derived from `text_sha256` and `chunk_id`. Fake vectors are for pipeline smoke only; they are not semantically meaningful embeddings.

`report` validates embedding manifest/vectors consistency (`vector_count`, unique vector IDs, `vector_sha256`, dimensions, `fake_vectors`, `lancedb_written: false`), per-vector `provider`/`embedding_model` match against the manifest, and optional cross-checks against chunks when `--chunks` is provided. Text output prints aggregate stats only — not full vectors or chunk text.

A passing `memory embedding report` is a prerequisite for `memory lancedb plan` (Task 22.4). LanceDB writes are not implemented yet; see docs/MEMORY_LANCEDB.md.

## Vector contract

Each JSONL vector records:

- `id`
- `chunk_id`
- `domain`
- `source_path`
- `source_sha256`
- `text_sha256`
- `embedding_model`
- `provider`
- `dimensions`
- `vector`
- `vector_sha256`

The embedding manifest records:

- `generated_at`
- `policy_sha256`
- `input_manifest_sha256`
- `input_chunks_sha256`
- `chunk_count`
- `vector_count`
- `dimensions`
- `provider`
- `model`
- `mode`
- `fake_vectors`
- `dry_run_only`
- `lancedb_written`
- output paths

## Safety

- no real embedding API calls in `dry_run` or `fake_vectors`
- fake vectors use deterministic local hashing only
- no LanceDB writes
- no network calls
- env values are never printed
- chunk text is never written to vector artifacts
- memory apply/restore unchanged
- runner retrieval unchanged
- source chunk files are read-only

## Boundary

Tasks 22.0 and 22.1 own chunk build and QA. Task 22.2 owns provider policy and dry-run. Task 22.3 owns deterministic fake vector smoke only. Task 22.3.1 owns vector report QA (including provider/model hardening). Task 22.4 owns LanceDB write planning only (`memory lancedb validate/plan`). Task 22.4.1 adds fake-write artifact smoke (`memory lancedb fake-write`). Real embeddings, LanceDB writes, and retrieval remain future work.
