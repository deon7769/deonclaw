# Context Packs

Context packs are scoped Markdown files generated from a task and a domains config.

They are an execution aid, not a memory source of truth.

## Scope

`deonctl context build` does not read a whole vault or scan directories recursively.

The builder reads only:

- the task YAML passed with `--task`
- the domains YAML passed with `--domains`
- bridge files explicitly listed in the selected domain config

## Domain Isolation

General/default context packs do not include isolated domains.

When a task targets the general domain, DeonClaw includes only the task metadata, the selected general domain metadata, policy paths and validation commands.

Isolated domains use explicit `bridge_files`.

For example, an Escalasoft task can include curated bridge files from the Escalasoft domain config, but DeonClaw still does not recursively ingest the Escalasoft vault and does not ingest SQLite data.

## Source Metadata

Each context source records:

- `exists`
- `size_bytes`
- `sha256`
- `truncated`

This keeps context packs auditable without turning them into hidden memory imports.

## Large Files

Bridge file content is limited to 262144 bytes by default.

If a bridge file is larger than the limit, the context pack stores only the truncated content, marks the source with `truncated: true`, and records a warning.

Missing bridge files also produce warnings, but do not fail context pack generation by default.
