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

The current MVP stops at artifact preservation and lint reporting.
