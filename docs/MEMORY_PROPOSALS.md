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

`deonctl memory proposal apply` only supports dry-run previews today:

```bash
deonctl memory proposal apply \
  --proposal memory-proposal.json \
  --policy configs/examples/memory-policy.yaml \
  --dry-run
```

`--dry-run` is required.

Real apply does not exist yet.

The command does not write memory files. It builds an apply preview that reports what would happen if a future approved apply step existed.

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

The optional `--output` flag writes the preview JSON, not memory:

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

Memory proposal lint enforces policy boundaries before any future approval/apply workflow exists.

For example:

- protected paths reject direct proposals unless future policy explicitly allows them
- Escalasoft proposals must stay inside allowed Escalasoft targets or approved bridge files
- `allowed_global_bridge` targets are controlled and produce a warning

The current MVP supports artifact preservation, lint reporting, and apply dry-run previews. It still does not perform real memory apply.
