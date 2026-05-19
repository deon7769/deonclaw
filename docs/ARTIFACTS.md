# Artifacts

DeonClaw stores artifact files on the filesystem.

SQLite stores artifact metadata only:

- artifact id
- run id
- kind
- path
- size in bytes
- sha256
- keep flag
- creation timestamp

The artifact path recorded in SQLite points to the file written under the configured artifacts directory for a run.

## Listing

Use:

```bash
deonctl artifacts list --store <path>
deonctl artifacts list --store <path> --run <run-id>
deonctl artifacts list --store <path> --status <status>
```

The list command prints artifact id, run id, kind, path, size, sha256, keep and created_at.

## Retention

Prune is conservative by default.

`deonctl artifacts prune` only deletes artifacts belonging to `succeeded` runs. Artifacts from `failed` and `policy_failed` runs are preserved so debugging evidence remains available.

Artifacts with `keep=true` are also preserved.
