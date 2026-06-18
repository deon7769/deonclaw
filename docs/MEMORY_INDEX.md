# Memory Index

Task 22.0 adds the memory index foundation. It prepares auditable chunks from approved/materialized Markdown memory sources without changing the canonical memory apply/restore workflow.

## Scope

Implemented now:

- `memory-index.yaml` config loading and validation
- read-only scanning of configured memory roots
- `validate`, `plan`, `build`, `doctor`, and `report` CLI commands
- `memory-index-manifest.json`
- `memory-index-chunks.jsonl`

Not implemented yet:

- real embedding provider calls and vector generation (see docs/MEMORY_EMBEDDINGS.md for dry-run policy only)
- LanceDB writes or queries
- runner retrieval integration
- MCP integration
- UI/dashboard

Markdown + Git remain the source of truth. The index is derived state only.

## Config

Example:

~~~bash
configs/examples/memory-index.yaml
~~~

Shape:

~~~yaml
memory_index:
  domains:
    - mysecondbrain
    - escalasoft_brain
  sources:
    - domain: mysecondbrain
      root: memory/mysecondbrain
      include:
        - "**/*.md"
      exclude:
        - "**/.archive/**"
        - "**/secrets/**"
  chunking:
    max_chars: 2000
    overlap_chars: 200
  output:
    manifest: memory-index-manifest.json
    chunks: memory-index-chunks.jsonl
~~~

Validation rules:

- domains must be known (`mysecondbrain`, `escalasoft_brain`)
- each source domain must be listed in `memory_index.domains`
- roots must exist
- include/exclude patterns must be non-empty
- `max_chars > 0`
- `overlap_chars >= 0` and `< max_chars`
- secrets paths are blocked
- files outside configured roots are never indexed
- symlinks are never followed or indexed
- `memory_index.output.manifest` and `memory_index.output.chunks` must be relative paths (not absolute)
- output paths must not contain `..`, `secrets`, or `.env`
- build writes manifest/chunks only inside `--artifacts-dir`

## CLI

Command roles:

| Command | Purpose |
|---------|---------|
| `validate` | Schema and output path validation |
| `plan` | List candidate source files per domain |
| `build` | Generate manifest and chunks JSONL |
| `doctor` | Pre-build diagnostics (scan, estimates, warnings) |
| `report` | Post-build QA over manifest and chunks |

Validate config and filesystem roots:

~~~bash
deonctl memory index validate --config configs/examples/memory-index.yaml
~~~

Plan candidate files without generating chunks:

~~~bash
deonctl memory index plan --config configs/examples/memory-index.yaml
deonctl memory index plan --config configs/examples/memory-index.yaml --output-format json
~~~

Build auditable index artifacts:

~~~bash
deonctl memory index build --config configs/examples/memory-index.yaml --artifacts-dir artifacts
~~~

Pre-build diagnostics without writing chunks:

~~~bash
deonctl memory index doctor --config configs/examples/memory-index.yaml
deonctl memory index doctor --config configs/examples/memory-index.yaml --output-format json
~~~

Post-build QA over generated artifacts:

~~~bash
deonctl memory index report \
  --manifest artifacts/memory-index-manifest.json \
  --chunks artifacts/memory-index-chunks.jsonl
deonctl memory index report \
  --manifest artifacts/memory-index-manifest.json \
  --chunks artifacts/memory-index-chunks.jsonl \
  --output-format json
~~~

`plan` reports candidate file counts per domain and skipped paths (`skipped_count`, plus `skipped_symlinks`, `skipped_secret_paths`, and `skipped_excluded` when available).

`build` writes manifest and chunks under `--artifacts-dir` using the configured relative output paths:

- `memory-index-manifest.json` (default)
- `memory-index-chunks.jsonl` (default)

Relative paths may include subdirectories (for example `indexes/manifest.json`); absolute output paths are rejected.

`doctor` reports domains, roots, `root_exists`, source and skipped counts, estimated chunks per domain, largest files (top 10), and warnings. It scans read-only and never follows symlinks.

`report` validates manifest/chunks consistency (`chunk_count`, unique chunk IDs, `text_sha256`, `source_sha256`, domain membership) and prints aggregate stats only — not raw chunk text.

Chunks JSONL can also serve as the derived source for governed retrieval-context materialization (`deonctl retrieval context materialize`). That command reads chunk rows only; it does not open canonical Markdown sources.

## Chunk contract

Each JSONL chunk records:

- `id`
- `domain`
- `source_path`
- `source_sha256`
- `chunk_index`
- `text`
- `text_sha256`
- `char_start`
- `char_end`

The manifest records:

- `generated_at`
- `config_sha256`
- `domains`
- `source_count`
- `chunk_count`
- `skipped_count`
- `skipped` (`skipped_symlinks`, `skipped_secret_paths`, `skipped_excluded`)
- output paths

## Safety

The memory index is derived state. Markdown + Git remain canonical; rebuilds are safe to repeat and do not modify source memory files.

- read-only over memory files
- no memory apply/restore changes
- no embeddings
- no LanceDB
- no network calls
- no environment values in artifacts
- secrets paths are excluded
- symlinks are skipped (never followed or indexed)
- index artifacts are confined to `--artifacts-dir` by default

A future task may transform these chunks into vectors or wire retrieval into the runner. That is intentionally out of scope for Task 22.0.

## Roadmap boundary

| Task | Scope |
|------|-------|
| 22.0 | Chunk config, scan, build |
| 22.1 | Index doctor and post-build report |
| 22.2 | Embedding provider policy and dry-run (`memory embedding validate/doctor/plan`) |
| 22.3 | Deterministic local fake vectors (`memory embedding build-fake`), no real embeddings |
| 22.3.1 | Embedding vector report/QA (`memory embedding report`), no LanceDB |
| 22.4 | LanceDB write plan-only (`memory lancedb validate/plan`), no database writes |
| 22.4.1 | LanceDB fake-write artifact smoke (`memory lancedb fake-write`), no LanceDB SDK |
| 22.5 | LanceDB real local write smoke (`memory lancedb write-smoke`), no retrieval |
| 22.6 | LanceDB readback doctor/report (`memory lancedb doctor/report`), no search |
| 22.7 | LanceDB controlled vector search smoke (`memory lancedb search-smoke`), explicit vector/chunk_id only; no natural-language retrieval or runner integration |
| 22.7.1 | LanceDB search-smoke result QA/report (`memory lancedb search-report`), no new search or runner integration |
| 22.8 | Passive LanceDB retrieval context attachment in runner (`retrieval_context` task config), no search in runner |
| 22.8.1 | Retrieval context inspect + runs retrieval-report (`retrieval context inspect`, `runs retrieval-report`), no active search |
| 22.9 | Governed chunk text materialization (`retrieval context materialize`), no runner auto-injection |
| 22.9.1 | Materialized retrieval context report + materialize path hardening (`retrieval context materialized-report`), no runner auto-injection |

22.3.1 validates fake vector artifacts read-only. Task 22.4 plans LanceDB writes from validated embedding artifacts. Task 22.4.1 emits fake row/manifest artifacts for table-shape smoke. Task 22.5 writes a real local LanceDB table via a Python adapter. Task 22.6 validates write-smoke databases structurally via doctor/report. Task 22.7 runs controlled top-k vector search with an explicit query vector or existing chunk row after doctor passes. Task 22.7.1 validates search-smoke result artifacts. Task 22.8 attaches validated search metadata passively into worker prompts. Task 22.8.1 inspects retrieval-context artifacts and reports runs with passive attachments. Task 22.9 materializes governed chunk text excerpts from chunks JSONL with explicit confirmation. Task 22.9.1 validates materialized artifacts and hardens materialize input paths. See docs/MEMORY_EMBEDDINGS.md, docs/MEMORY_LANCEDB.md, and docs/RETRIEVAL_CONTEXT.md.

## Boundary

Memory apply and restore remain the only canonical write path.

The memory index foundation does not:

- retrieve context during worker runs
- call MCP tools
- execute fallback policies
- modify source Markdown
