# Memory Proposals

Memory proposals are review artifacts for candidate memory changes.

They do not apply changes to memory.

They do not write to `mysecondbrain`.

They do not write to `escalasoft_brain`.

## Artifacts

Workers may produce:

- `memory-proposal.json`
- `memory-proposal.md`

DeonClaw preserves these files as run artifacts when a worker returns them.

The proposal artifact is evidence for later review. It is not an approval and it is not an apply step.

## Codex Run Lint

`deonctl worker codex run` can lint a preserved proposal when `--memory-policy` is provided:

```bash
deonctl worker codex run <task-path> \
  --store <path> \
  --artifacts-dir <path> \
  --memory-policy configs/examples/memory-policy.yaml
```

When lint is enabled, DeonClaw looks for `memory-proposal.json`, validates it structurally, checks it against the memory policy, and writes `memory-proposal-lint.json` as a run artifact.

Lint does not apply the proposal.

Lint failure does not change the run status yet.

## Apply Dry Run

`deonctl memory proposal apply` supports dry-run previews:

```bash
deonctl memory proposal apply \
  --proposal memory-proposal.json \
  --policy configs/examples/memory-policy.yaml \
  --dry-run
```

`--dry-run` is required.

Real apply is a separate `apply-execute` step after approval, preflight, backup planning, and backup materialization.

The command does not apply the proposal and does not write memory files. It does not write to `mysecondbrain`, `escalasoft_brain`, the proposal `target_path`, or any patch target path.

It builds an apply preview that reports what a later approved apply-execute step would do.

The proposal lint must pass before the preview can reach `dry_run_ok`. If proposal parsing, structural validation, or policy lint fails, the preview status is `failed`.

Each `MemoryPatch` is also validated individually against the same memory policy before `dry_run_ok` is allowed.

Patch fallback rules:

- empty patch `target_path` uses the proposal `target_path`
- empty patch `operation` uses the proposal `operation`

Patch policy rules:

- invalid patch `operation` fails the dry-run
- patch `target_path` matching `protected` fails the dry-run
- patch `target_path` matching isolated-domain `forbidden_global_write` fails the dry-run
- patch `target_path` matching `allowed_global_bridge` passes with a warning
- if any patch fails, the apply preview status is `failed`
- patch violations are included in the apply preview

The optional `--output` flag writes only the preview JSON, commonly named `apply-preview.json`:

```bash
deonctl memory proposal apply \
  --proposal memory-proposal.json \
  --policy configs/examples/memory-policy.yaml \
  --dry-run \
  --output apply-preview.json
```

The output file must not be the proposal `target_path` or any effective patch `target_path`. DeonClaw refuses `--output` when it would write the preview to a memory target.

## Approval Artifact

`deonctl memory proposal approve` creates a review artifact for a memory proposal:

```bash
deonctl memory proposal approve \
  --proposal memory-proposal.json \
  --policy configs/examples/memory-policy.yaml \
  --reviewer "Davi" \
  --decision approved \
  --reason "Reviewed and accepted." \
  --output memory-approval.json
```

The command writes `memory-approval.json`.

It does not apply memory.

It does not write to `mysecondbrain`.

It does not write to `escalasoft_brain`.

The approval artifact records the proposal identity, target, reviewer, decision, reason, lint result and apply dry-run result, including:

- `proposal_sha256`
- `apply_preview_sha256`
- `lint_status`
- `lint_warnings`
- `lint_violations`
- `apply_status`
- `patch_count`
- `patch_warnings`
- `patch_violations`

Approval decisions are gated:

- `approved` requires `lint_status: ok`
- `approved` requires `apply_status: dry_run_ok`
- `approved` requires empty `patch_violations`
- `rejected` can be generated even when lint failed, apply failed, or patch violations exist

The approval output path is also guarded:

- it must not be the proposal `target_path`
- it must not be any effective patch `target_path`
- it must not be inside a memory domain such as `mysecondbrain` or `escalasoft_brain`

The approval artifact is evidence for a later apply workflow.

It is not an apply step.

It is not a Git commit.

## Apply Preflight

`deonctl memory proposal apply-preflight` checks that an approved proposal is still safe to apply and still bound to its approval before any future apply step can exist:

```bash
deonctl memory proposal apply-preflight \
  --proposal memory-proposal.json \
  --approval memory-approval.json \
  --policy configs/examples/memory-policy.yaml \
  --output apply-preflight.json
```

The command loads the current proposal, approval artifact, and memory policy.

It re-runs proposal lint against the current proposal and policy.

It re-runs the current apply dry-run preview against the current proposal and policy.

Preflight succeeds only when the approval artifact still describes an approved, clean review:

- approval `decision` must be `approved`
- approval `lint_status` must be `ok`
- approval `apply_status` must be `dry_run_ok`
- approval `patch_violations` must be empty

The approval artifact must also remain bound to the exact reviewed content:

- approved `proposal_sha256` must match the current proposal `proposal_sha256`
- approved `apply_preview_sha256` must match the current apply preview `apply_preview_sha256`
- current lint must still be `ok`
- current apply dry-run must still be `dry_run_ok`
- current patch violations must still be empty

If the proposal or any patch changes after approval, apply preflight fails. This includes patch content changes, patch target changes, and any change that alters the rebuilt apply preview.

The optional `--output` flag writes the preflight artifact JSON, not memory.

Apply preflight does not apply memory.

Apply preflight does not write to `mysecondbrain`.

Apply preflight does not write to `escalasoft_brain`.

Apply preflight does not write to the proposal `target_path` or any patch target path.

Apply preflight does not make a Git commit.

## Backup Plan

`deonctl memory proposal backup-plan` creates a backup and restore plan for the effective patch targets:

```bash
deonctl memory proposal backup-plan \
  --proposal memory-proposal.json \
  --approval memory-approval.json \
  --policy configs/examples/memory-policy.yaml \
  --output backup-plan.json
```

The command loads the proposal, approval artifact, and memory policy.

It runs apply preflight first.

Backup plan generation succeeds only when preflight status is `preflight_ok`.

The command identifies every effective patch target. Empty patch `target_path` values use the proposal `target_path`, matching apply dry-run behavior.

Repeated effective targets are deduplicated.

Each backup item records:

- `target_path`
- `exists`
- `operation`
- `size_bytes`, when the target exists
- `sha256`, when the target exists
- suggested `backup_path`

The backup plan embeds a restore plan with matching restore items.

Backup plan generation does not copy files.

Backup plan generation does not apply memory.

Backup plan generation does not write to `mysecondbrain`.

Backup plan generation does not write to `escalasoft_brain`.

Backup plan generation does not write to the proposal `target_path` or any patch target path.

The `--output` path must not be the proposal `target_path` or any effective patch target path.

The `--output` path must not be inside a memory domain such as `mysecondbrain` or `escalasoft_brain`.

Backup plan generation does not make a Git commit.

## Backup Materialization

`deonctl memory proposal backup-materialize` copies existing targets described by a backup plan into their planned backup paths:

```bash
deonctl memory proposal backup-materialize \
  --backup-plan backup-plan.json \
  --output backup-result.json
```

The command loads `backup-plan.json`.

For each backup item:

- `exists: false` is not copied and becomes `skipped_missing`
- `exists: true` requires the current `target_path` SHA-256 to match the plan `sha256`
- when the hash matches, the current `target_path` is copied to `backup_path`
- missing backup directories are created as needed
- after copying, DeonClaw records the copied backup file SHA-256 as `backup_sha256`

The generated `backup-result.json` records each item status and hash evidence.

Backup materialization does not write to any `target_path`.

Backup materialization does not apply memory.

Backup materialization does not write to `mysecondbrain`.

Backup materialization does not write to `escalasoft_brain`.

Backup materialization does not make a Git commit.

Safety checks:

- `backup_path` must stay inside `backup_root`
- `target_path` and `backup_path` must not contain path traversal
- `backup_path` must not equal `target_path`
- if a target changed after backup-plan generation, materialization fails before copying that item
- `--output` must not be any backup item `target_path`
- `--output` must not be any backup item `backup_path`
- `--output` must not be inside `backup_root`
- `--output` must not be inside a memory domain such as `mysecondbrain` or `escalasoft_brain`

## Apply Execute

`deonctl memory proposal apply-execute` performs the real memory write for the limited create/append MVP:

```bash
deonctl memory proposal apply-execute \
  --proposal memory-proposal.json \
  --approval memory-approval.json \
  --policy configs/examples/memory-policy.yaml \
  --backup-plan backup-plan.json \
  --backup-result backup-result.json \
  --output apply-result.json \
  --confirm-apply
```

`--confirm-apply` is required.

The command loads the current proposal, approval artifact, memory policy, backup plan, and backup result.

Before writing any target path, it runs apply preflight again. Apply execution is blocked unless preflight status is `preflight_ok`.

It also validates the backup chain before writing:

- backup plan `proposal_id` and `approval_id` must match the loaded proposal and approval
- backup result `proposal_id` and `approval_id` must match the loaded proposal and approval
- backup result must cover every backup plan item
- existing targets in the backup plan must have been copied in the backup result
- copied backup files must still hash to the recorded `backup_sha256`
- current target SHA-256 must still match the backup plan SHA-256 for targets that existed when the plan was created
- targets that were missing during backup planning must still be missing before apply

If any proposal, approval, policy, target, backup plan, or backup result check fails, apply execution fails before writing target files.

Supported operations:

- `create` creates a new file and fails if the target already exists
- `append` appends to an existing file, or creates the file when the approved append target is still missing

Unsupported operations:

- `update` fails with `operation not implemented`
- `archive` fails with `operation not implemented`

Apply execution creates target directories when needed.

The generated `apply-result.json` records each applied target, operation, status, bytes written, and final SHA-256.

Apply execution does not make a Git commit.

Apply execution does not implement restore.

Safety checks:

- `--output` must not be any proposal or patch `target_path`
- `--output` must not be any backup item `target_path`
- `--output` must not be any backup item `backup_path`
- `--output` must not be inside `backup_root`
- `--output` must not be inside a memory domain such as `mysecondbrain` or `escalasoft_brain`
- apply execution only writes effective patch target paths

## Summary Status

Run summaries report one of these memory proposal states:

- `missing`: `--memory-policy` was provided, but no `memory-proposal.json` was found.
- `not_checked`: no `--memory-policy` was provided, so any preserved proposal was not linted.
- `ok`: proposal lint passed.
- `failed`: proposal parsing, structural validation, or policy lint failed.

The summary also includes:

- proposal id, when a proposal is available
- violation count
- warning count
- detailed violations or warnings when the list is small

## Policy Boundary

Memory proposal lint enforces policy boundaries before approval and apply execution.

For example:

- protected paths reject direct proposals unless future policy explicitly allows them
- Escalasoft proposals must stay inside allowed Escalasoft targets or approved bridge files
- `allowed_global_bridge` targets are controlled and produce a warning

The current MVP supports artifact preservation, lint reporting, apply dry-run previews, approval artifacts, apply preflight, backup/restore plans, backup materialization, and real apply for `create`/`append`. It still does not implement real `update`/`archive` apply or commit memory changes.
