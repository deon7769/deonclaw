# Memory Index

Task 22.0 adds the memory index foundation. It prepares auditable chunks from approved/materialized Markdown memory sources without changing the canonical memory apply/restore workflow.

## Scope

Implemented now:

- `memory-index.yaml` config loading and validation
- read-only scanning of configured memory roots
- `validate`, `plan`, and `build` CLI commands
- `memory-index-manifest.json`
- `memory-index-chunks.jsonl`

Not implemented yet:

- embeddings
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

## CLI

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

`plan` reports candidate file counts per domain and skipped paths.

`build` writes:

- `memory-index-manifest.json`
- `memory-index-chunks.jsonl`

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
- output paths

## Safety

- read-only over memory files
- no memory apply/restore changes
- no embeddings
- no LanceDB
- no network calls
- no environment values in artifacts
- secrets paths are excluded

A future task may transform these chunks into vectors or wire retrieval into the runner. That is intentionally out of scope for Task 22.0.

## Boundary

Memory apply and restore remain the only canonical write path.

The memory index foundation does not:

- retrieve context during worker runs
- call MCP tools
- execute fallback policies
- modify source Markdown
