# Memory Workflow Smoke

This is a minimal end-to-end smoke workflow for memory proposals.

It uses the examples in `examples/memory/`:

- `memory-proposal-create.json`
- `memory-proposal-append.json`
- `memory-proposal-update.json`
- `memory-proposal-archive.json`

Each example contains `{{SMOKE_ROOT}}`. Copy one example to a temporary working directory and replace `{{SMOKE_ROOT}}` with an absolute temporary path before running the commands.

For `append`, `update`, and `archive`, create the target file before `backup-plan`:

- append target content: `Existing smoke memory.`
- update target content: `Old smoke memory.`
- archive target content: `Archive smoke memory.`

For `create`, the target file must not exist.

The workflow writes artifacts to a temporary artifact directory. Keep these artifacts outside target paths, backup paths, `backup_root`, `mysecondbrain`, and `escalasoft_brain`.

## Sequence

Set these paths for the copied proposal:

```bash
POLICY=configs/examples/memory-policy.yaml
PROPOSAL=/tmp/deonclaw-memory-smoke/artifacts/memory-proposal.json
APPROVAL=/tmp/deonclaw-memory-smoke/artifacts/memory-approval.json
APPLY_PREVIEW=/tmp/deonclaw-memory-smoke/artifacts/apply-preview.json
PREFLIGHT=/tmp/deonclaw-memory-smoke/artifacts/apply-preflight.json
BACKUP_PLAN=/tmp/deonclaw-memory-smoke/artifacts/backup-plan.json
BACKUP_RESULT=/tmp/deonclaw-memory-smoke/artifacts/backup-result.json
APPLY_RESULT=/tmp/deonclaw-memory-smoke/artifacts/apply-result.json
RESTORE_PREVIEW=/tmp/deonclaw-memory-smoke/artifacts/restore-preview.json
RESTORE_RESULT=/tmp/deonclaw-memory-smoke/artifacts/restore-result.json
```

1. Proposal lint

```bash
deonctl memory proposal lint \
  --proposal "$PROPOSAL" \
  --policy "$POLICY"
```

2. Apply dry-run

```bash
deonctl memory proposal apply \
  --proposal "$PROPOSAL" \
  --policy "$POLICY" \
  --dry-run \
  --output "$APPLY_PREVIEW"
```

3. Approve

```bash
deonctl memory proposal approve \
  --proposal "$PROPOSAL" \
  --policy "$POLICY" \
  --reviewer "Smoke Reviewer" \
  --decision approved \
  --reason "Smoke workflow approval." \
  --output "$APPROVAL"
```

4. Apply preflight

```bash
deonctl memory proposal apply-preflight \
  --proposal "$PROPOSAL" \
  --approval "$APPROVAL" \
  --policy "$POLICY" \
  --output "$PREFLIGHT"
```

5. Backup plan

```bash
deonctl memory proposal backup-plan \
  --proposal "$PROPOSAL" \
  --approval "$APPROVAL" \
  --policy "$POLICY" \
  --output "$BACKUP_PLAN"
```

6. Backup materialize

```bash
deonctl memory proposal backup-materialize \
  --backup-plan "$BACKUP_PLAN" \
  --output "$BACKUP_RESULT"
```

7. Apply execute

`apply-execute` requires `--confirm-apply`.

```bash
deonctl memory proposal apply-execute \
  --proposal "$PROPOSAL" \
  --approval "$APPROVAL" \
  --policy "$POLICY" \
  --backup-plan "$BACKUP_PLAN" \
  --backup-result "$BACKUP_RESULT" \
  --output "$APPLY_RESULT" \
  --confirm-apply
```

8. Restore dry-run

```bash
deonctl memory proposal restore \
  --backup-plan "$BACKUP_PLAN" \
  --backup-result "$BACKUP_RESULT" \
  --dry-run \
  --output "$RESTORE_PREVIEW"
```

9. Restore execute

`restore-execute` requires `--confirm-restore`.

```bash
deonctl memory proposal restore-execute \
  --backup-plan "$BACKUP_PLAN" \
  --backup-result "$BACKUP_RESULT" \
  --restore-preview "$RESTORE_PREVIEW" \
  --output "$RESTORE_RESULT" \
  --confirm-restore
```

## Notes

No step creates a Git commit automatically.

`apply-execute` only runs after approval, preflight, backup planning, and backup materialization.

`restore-execute` uses the backup plan, backup result, and restore preview as the manual recovery chain.
